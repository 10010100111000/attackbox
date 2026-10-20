package runtime

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// SanitizeRootfs executes all data scrubbing steps on the specified target rootfs directory.
func (b *GoldenImageBuilder) SanitizeRootfs(rootDir string) error {
	if rootDir == "" {
		rootDir = b.targetDir
	}

	// Reject writes through host-absolute links before touching account files.
	for _, rel := range []string{"etc/passwd", "etc/shadow", "etc/group", "etc/sudoers.d/attackbox", "home/box", "tmp"} {
		if err := checkImagePath(rootDir, filepath.Join(rootDir, rel), true); err != nil {
			return err
		}
	}
	validUsers, err := b.sanitizePasswdFile(rootDir)
	if err != nil {
		return fmt.Errorf("sanitize passwd: %w", err)
	}

	if err := b.sanitizeShadowFile(rootDir, validUsers); err != nil {
		return fmt.Errorf("sanitize shadow: %w", err)
	}

	if err := b.configureGroupFile(rootDir); err != nil {
		return fmt.Errorf("configure groups: %w", err)
	}

	if err := b.configureSudoersRule(rootDir); err != nil {
		return fmt.Errorf("configure sudoers: %w", err)
	}

	if err := b.purgeHostCredentials(rootDir); err != nil {
		return fmt.Errorf("purge credentials: %w", err)
	}

	if err := b.purgeHistoryAndLogs(rootDir); err != nil {
		return fmt.Errorf("purge history and logs: %w", err)
	}

	b.ensureEssentialDirectories(rootDir)

	return nil
}

// sanitizePasswdFile removes host administrative users and ensures 'box' exists.
// Returns the set of valid usernames retained in the passwd file.
func (b *GoldenImageBuilder) sanitizePasswdFile(rootDir string) (map[string]bool, error) {
	passwdPath := filepath.Join(rootDir, "etc/passwd")
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
func (b *GoldenImageBuilder) sanitizeShadowFile(rootDir string, validUsers map[string]bool) error {
	shadowPath := filepath.Join(rootDir, "etc/shadow")
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
func (b *GoldenImageBuilder) configureGroupFile(rootDir string) error {
	groupPath := filepath.Join(rootDir, "etc/group")
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
func (b *GoldenImageBuilder) configureSudoersRule(rootDir string) error {
	sudoersDir := filepath.Join(rootDir, "etc/sudoers.d")
	if err := os.MkdirAll(sudoersDir, 0755); err != nil {
		return err
	}

	ruleFile := filepath.Join(sudoersDir, "attackbox")
	ruleContent := "box ALL=(ALL) NOPASSWD: ALL\n"
	// Sudo strictly enforces 0440 permission on sudoers.d files
	return os.WriteFile(ruleFile, []byte(ruleContent), 0440)
}

// purgeHostCredentials wipes host configs, TLS private keys, SSH keys, VPN, cloud tokens, and cluster secrets.
func (b *GoldenImageBuilder) purgeHostCredentials(rootDir string) error {
	// 1. Explicit removal of AttackBox cluster secrets and data dirs
	attackboxDirs := []string{
		filepath.Join(rootDir, "etc/attackbox"),
		filepath.Join(rootDir, "var/lib/attackbox"),
		filepath.Join(rootDir, "etc/ssl/private"),
		filepath.Join(rootDir, "etc/tls"),
		filepath.Join(rootDir, "root/.ssh"),
		filepath.Join(rootDir, "root/.aws"),
		filepath.Join(rootDir, "root/.azure"),
		filepath.Join(rootDir, "root/.config/gcloud"),
		filepath.Join(rootDir, "root/.kube"),
		filepath.Join(rootDir, "root/.docker"),
		filepath.Join(rootDir, "run/secrets"),
	}
	for _, d := range attackboxDirs {
		if err := removeImagePath(rootDir, d); err != nil {
			return err
		}
	}

	// 2. Comprehensive pattern eradication for certificates, systemd units, tokens
	purgePatterns := []string{
		filepath.Join(rootDir, "etc/ssh/ssh_host_*"),
		filepath.Join(rootDir, "root/.ssh/*"),
		filepath.Join(rootDir, "home/*/.ssh"),
		filepath.Join(rootDir, "home/*/.ssh/*"),
		filepath.Join(rootDir, "home/*/.aws"),
		filepath.Join(rootDir, "home/*/.azure"),
		filepath.Join(rootDir, "home/*/.config/gcloud"),
		filepath.Join(rootDir, "home/*/.kube"),
		filepath.Join(rootDir, "home/*/.docker"),
		filepath.Join(rootDir, "home/*/.netrc"),
		filepath.Join(rootDir, "root/.netrc"),
		filepath.Join(rootDir, "etc/wireguard/*"),
		filepath.Join(rootDir, "etc/openvpn/*"),
		filepath.Join(rootDir, "etc/ipsec*"),
		filepath.Join(rootDir, "etc/strongswan*"),
		filepath.Join(rootDir, "etc/ssl/certs/*node*"),
		filepath.Join(rootDir, "etc/ssl/certs/*cluster*"),
		filepath.Join(rootDir, "etc/systemd/system/attackbox*"),
		filepath.Join(rootDir, "etc/default/attackbox*"),
		filepath.Join(rootDir, "etc/shadow-"),
		filepath.Join(rootDir, "etc/gshadow-"),
		filepath.Join(rootDir, "etc/passwd-"),
		filepath.Join(rootDir, "etc/group-"),
		filepath.Join(rootDir, "etc/subuid-"),
		filepath.Join(rootDir, "etc/subgid-"),
	}

	for _, pattern := range purgePatterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return err
		}
		for _, path := range matches {
			if err := removeImagePath(rootDir, path); err != nil {
				return err
			}
		}
	}
	return nil
}

// purgeHistoryAndLogs clears bash history files and logs.
func (b *GoldenImageBuilder) purgeHistoryAndLogs(rootDir string) error {
	purgePatterns := []string{
		filepath.Join(rootDir, "root/.bash_history"),
		filepath.Join(rootDir, "home/*/.bash_history"),
		filepath.Join(rootDir, "var/log/*"),
		filepath.Join(rootDir, "tmp/*"),
		filepath.Join(rootDir, "var/tmp/*"),
		filepath.Join(rootDir, "run/*"),
	}

	if info, err := os.Lstat(filepath.Join(rootDir, "var/run")); err == nil && info.Mode()&os.ModeSymlink == 0 {
		purgePatterns = append(purgePatterns, filepath.Join(rootDir, "var/run/*"))
	}

	for _, pattern := range purgePatterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return err
		}
		for _, path := range matches {
			if err := removeImagePath(rootDir, path); err != nil {
				return err
			}
		}
	}
	return nil
}

// ensureEssentialDirectories guarantees minimal Linux root structure and correct permissions.
func (b *GoldenImageBuilder) ensureEssentialDirectories(rootDir string) {
	dirs := []string{"proc", "sys", "dev", "tmp", "run", "etc", "home/box"}
	for _, d := range dirs {
		p := filepath.Join(rootDir, d)
		_ = os.MkdirAll(p, 0755)
	}
	_ = os.Chmod(filepath.Join(rootDir, "tmp"), 01777)

	// Set /home/box ownership to UID 1001, GID 1013
	homeBox := filepath.Join(rootDir, "home/box")
	_ = os.Chown(homeBox, 1001, 1013)
}

// The image is offline and root-owned while sanitized. Verify resolved ancestors
// before writes/removals; RemoveAll itself does not follow a final symlink.
func checkImagePath(root, path string, followFinal bool) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	if !followFinal {
		path = filepath.Dir(path)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return err
	}
	for {
		resolved, e := filepath.EvalSymlinks(path)
		if e == nil {
			rel, e := filepath.Rel(root, resolved)
			if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
				return fmt.Errorf("image path escapes root: %s", path)
			}
			return nil
		}
		if !os.IsNotExist(e) {
			return e
		}
		parent := filepath.Dir(path)
		if parent == path {
			return e
		}
		path = parent
	}
}

func removeImagePath(root, path string) error {
	if err := checkImagePath(root, path, false); err != nil {
		return err
	}
	return os.RemoveAll(path)
}
