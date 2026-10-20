package runtime

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// GoldenImageBuilder manages automated snapshotting and deep sanitization of the golden base rootfs.
type GoldenImageBuilder struct {
	targetDir string
}

// NewGoldenImageBuilder constructs a builder for the golden rootfs image.
func NewGoldenImageBuilder(targetDir string) *GoldenImageBuilder {
	return &GoldenImageBuilder{
		targetDir: targetDir,
	}
}

// EnsureGoldenImage validates that the golden base exists and is locked read-only.
// If it does not exist, it executes the full derive-sanitize-lock pipeline from host '/'.
func (b *GoldenImageBuilder) EnsureGoldenImage() error {
	if _, err := os.Stat(b.targetDir); err == nil {
		// Image already exists; ensure read-only property is enforced
		_ = b.lockImageReadOnly()
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(b.targetDir), 0755); err != nil {
		return fmt.Errorf("create golden image parent dir: %w", err)
	}

	// 1. Take snapshot of host root '/'
	if err := b.SnapshotHostRoot(); err != nil {
		return fmt.Errorf("snapshot host root: %w", err)
	}

	// 2. Unlock as writable temporarily for in-place sanitization
	if err := b.unlockImageWritable(); err != nil {
		_ = b.cleanupFailedImage()
		return fmt.Errorf("unlock golden image: %w", err)
	}

	// 3. Deep sanitize rootfs (shadow, passwd, ssh, logs, sudoers, default user)
	if err := b.SanitizeRootfs(); err != nil {
		_ = b.cleanupFailedImage()
		return fmt.Errorf("sanitize golden image: %w", err)
	}

	// 4. Lock image as physical read-only for tamper resistance
	if err := b.lockImageReadOnly(); err != nil {
		fmt.Printf("[WARN] Failed to lock golden image read-only: %v\n", err)
	}

	return nil
}

// SnapshotHostRoot derives an initial btrfs subvolume snapshot from the host '/'.
func (b *GoldenImageBuilder) SnapshotHostRoot() error {
	cmd := exec.Command("btrfs", "subvolume", "snapshot", "/", b.targetDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("btrfs snapshot / to %s failed: %s (%w)", b.targetDir, strings.TrimSpace(string(out)), err)
	}
	return nil
}

// SanitizeRootfs executes all data scrubbing steps on the target rootfs.
func (b *GoldenImageBuilder) SanitizeRootfs() error {
	validUsers, err := b.sanitizePasswdFile()
	if err != nil {
		return fmt.Errorf("sanitize passwd: %w", err)
	}

	if err := b.sanitizeShadowFile(validUsers); err != nil {
		return fmt.Errorf("sanitize shadow: %w", err)
	}

	if err := b.configureGroupFile(); err != nil {
		return fmt.Errorf("configure groups: %w", err)
	}

	if err := b.configureSudoersRule(); err != nil {
		return fmt.Errorf("configure sudoers: %w", err)
	}

	if err := b.purgeHostCredentials(); err != nil {
		return fmt.Errorf("purge credentials: %w", err)
	}

	if err := b.purgeHistoryAndLogs(); err != nil {
		return fmt.Errorf("purge history and logs: %w", err)
	}

	b.ensureEssentialDirectories()

	return nil
}

// sanitizePasswdFile removes host administrative users and ensures 'box' exists.
// Returns the set of valid usernames retained in the passwd file.
func (b *GoldenImageBuilder) sanitizePasswdFile() (map[string]bool, error) {
	passwdPath := filepath.Join(b.targetDir, "etc/passwd")
	f, err := os.Open(passwdPath)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{"root": true, "box": true}, nil
		}
		return nil, err
	}
	defer f.Close()

	validUsers := make(map[string]bool)
	var sanitizedLines []string
	hasBox := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			sanitizedLines = append(sanitizedLines, line)
			continue
		}

		parts := strings.Split(line, ":")
		if len(parts) >= 7 {
			username := parts[0]
			uid, _ := strconv.Atoi(parts[2])

			if username == "box" {
				hasBox = true
				validUsers["box"] = true
				sanitizedLines = append(sanitizedLines, "box:x:1001:1013:AttackBox User:/home/box:/bin/bash")
				continue
			}

			// Strip host physical users (UID >= 1000 except nobody 65534)
			if uid >= 1000 && uid != 65534 {
				// Drop host-specific user from container image
				continue
			}

			validUsers[username] = true
			sanitizedLines = append(sanitizedLines, line)
		} else {
			sanitizedLines = append(sanitizedLines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if !hasBox {
		validUsers["box"] = true
		sanitizedLines = append(sanitizedLines, "box:x:1001:1013:AttackBox User:/home/box:/bin/bash")
	}

	output := strings.Join(sanitizedLines, "\n") + "\n"
	return validUsers, os.WriteFile(passwdPath, []byte(output), 0644)
}

// sanitizeShadowFile zeroes out all password hashes to '!' and strips removed host users.
func (b *GoldenImageBuilder) sanitizeShadowFile(validUsers map[string]bool) error {
	shadowPath := filepath.Join(b.targetDir, "etc/shadow")
	f, err := os.Open(shadowPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	var sanitizedLines []string
	hasBox := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			sanitizedLines = append(sanitizedLines, line)
			continue
		}

		parts := strings.Split(line, ":")
		if len(parts) >= 2 {
			username := parts[0]
			// Drop lines belonging to deleted host users
			if validUsers != nil && !validUsers[username] && username != "box" {
				continue
			}

			if username == "box" {
				hasBox = true
			}
			// Force wipe password hash to '!' (locked / no password)
			parts[1] = "!"
			sanitizedLines = append(sanitizedLines, strings.Join(parts, ":"))
		} else {
			sanitizedLines = append(sanitizedLines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	if !hasBox {
		sanitizedLines = append(sanitizedLines, "box:!:20740:0:99999:7:::")
	}

	output := strings.Join(sanitizedLines, "\n") + "\n"
	return os.WriteFile(shadowPath, []byte(output), 0640)
}

// configureGroupFile aligns sudo and box groups for non-root privileges.
func (b *GoldenImageBuilder) configureGroupFile() error {
	groupPath := filepath.Join(b.targetDir, "etc/group")
	f, err := os.Open(groupPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	var lines []string
	hasBoxGroup := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ":")
		if len(parts) >= 4 {
			groupName := parts[0]
			if groupName == "sudo" {
				// Sandbox sudo group strictly binds 'box'
				line = "sudo:x:27:box"
			} else if groupName == "box" {
				hasBoxGroup = true
				line = "box:x:1013:"
			}
		}
		lines = append(lines, line)
	}

	if !hasBoxGroup {
		lines = append(lines, "box:x:1013:")
	}

	output := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(groupPath, []byte(output), 0644)
}

// configureSudoersRule drops passwordless sudo permission for 'box' into /etc/sudoers.d/.
func (b *GoldenImageBuilder) configureSudoersRule() error {
	sudoersDir := filepath.Join(b.targetDir, "etc/sudoers.d")
	if err := os.MkdirAll(sudoersDir, 0755); err != nil {
		return err
	}

	ruleFile := filepath.Join(sudoersDir, "attackbox")
	ruleContent := "box ALL=(ALL) NOPASSWD: ALL\n"
	// Sudo strictly enforces 0440 permission on sudoers.d files
	return os.WriteFile(ruleFile, []byte(ruleContent), 0440)
}

// purgeHostCredentials wipes host ssh host keys, user keys, and VPN credentials.
func (b *GoldenImageBuilder) purgeHostCredentials() error {
	purgePatterns := []string{
		filepath.Join(b.targetDir, "etc/ssh/ssh_host_*"),
		filepath.Join(b.targetDir, "root/.ssh/*"),
		filepath.Join(b.targetDir, "home/*/.ssh/*"),
		filepath.Join(b.targetDir, "etc/wireguard/*"),
		filepath.Join(b.targetDir, "etc/openvpn/*"),
	}

	for _, pattern := range purgePatterns {
		matches, err := filepath.Glob(pattern)
		if err == nil {
			for _, m := range matches {
				_ = os.RemoveAll(m)
			}
		}
	}
	return nil
}

// purgeHistoryAndLogs clears bash history files and logs.
func (b *GoldenImageBuilder) purgeHistoryAndLogs() error {
	purgePatterns := []string{
		filepath.Join(b.targetDir, "root/.bash_history"),
		filepath.Join(b.targetDir, "home/*/.bash_history"),
		filepath.Join(b.targetDir, "var/log/*"),
		filepath.Join(b.targetDir, "tmp/*"),
		filepath.Join(b.targetDir, "var/tmp/*"),
		filepath.Join(b.targetDir, "var/run/*"),
	}

	for _, pattern := range purgePatterns {
		matches, err := filepath.Glob(pattern)
		if err == nil {
			for _, m := range matches {
				_ = os.RemoveAll(m)
			}
		}
	}
	return nil
}

// ensureEssentialDirectories guarantees minimal Linux root structure and correct permissions.
func (b *GoldenImageBuilder) ensureEssentialDirectories() {
	dirs := []string{"proc", "sys", "dev", "tmp", "run", "etc", "home/box"}
	for _, d := range dirs {
		p := filepath.Join(b.targetDir, d)
		_ = os.MkdirAll(p, 0755)
	}
	_ = os.Chmod(filepath.Join(b.targetDir, "tmp"), 01777)

	// Set /home/box ownership to UID 1001, GID 1013
	homeBox := filepath.Join(b.targetDir, "home/box")
	_ = os.Chown(homeBox, 1001, 1013)
}



func (b *GoldenImageBuilder) unlockImageWritable() error {
	cmd := exec.Command("btrfs", "property", "set", b.targetDir, "ro", "false")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("btrfs set ro false: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (b *GoldenImageBuilder) lockImageReadOnly() error {
	cmd := exec.Command("btrfs", "property", "set", b.targetDir, "ro", "true")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("btrfs set ro true: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (b *GoldenImageBuilder) cleanupFailedImage() error {
	cmd := exec.Command("btrfs", "subvolume", "delete", b.targetDir)
	_ = cmd.Run()
	return os.RemoveAll(b.targetDir)
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	_ = os.MkdirAll(filepath.Dir(dst), 0755)
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return os.Chmod(dst, perm)
}
