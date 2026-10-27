package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoldenSanitizationDoesNotFollowAbsoluteRunLink(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "var"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "run"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "var/run")); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(outside, "host-state")
	inside := filepath.Join(root, "run", "image-state")
	if err := os.WriteFile(victim, []byte("host fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inside, []byte("image fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := NewGoldenImageBuilder(root).SanitizeRootfs(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(victim); err != nil {
		t.Fatal("sanitizer touched path outside image", err)
	}
	if _, err := os.Stat(inside); !os.IsNotExist(err) {
		t.Fatal("image run directory not cleaned")
	}
}

func TestGoldenSanitizationRejectsEscapingLogDirectory(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "var"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "var/log")); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(outside, "host-log")
	if err := os.WriteFile(victim, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := NewGoldenImageBuilder(root).SanitizeRootfs(root); err == nil {
		t.Fatal("escaping path was accepted")
	}
	if _, err := os.Stat(victim); err != nil {
		t.Fatal("host log was removed", err)
	}
}

func TestGoldenImageBuilder_DataSanitization(t *testing.T) {
	tempDir := t.TempDir()
	builder := NewGoldenImageBuilder(tempDir)

	etcDir := filepath.Join(tempDir, "etc")
	_ = os.MkdirAll(etcDir, 0755)

	// 1. Prepare raw /etc/shadow with host hash
	rawShadow := `root:$6$rounds=50000$randomsalt$hostroothash123:19000:0:99999:7:::
daemon:*:19000:0:99999:7:::
laji:$y$j9T$8BMlyIAvQ4DECdteeQtvP1$LfuXxdDZExBhJDyWExGoHhTXVn0oAg4Yughahjz2hI/:20740:0:99999:7:::
nobody:*:19000:0:99999:7:::
`
	shadowFile := filepath.Join(etcDir, "shadow")
	if err := os.WriteFile(shadowFile, []byte(rawShadow), 0640); err != nil {
		t.Fatalf("write raw shadow: %v", err)
	}

	// 2. Prepare raw /etc/passwd with host users
	rawPasswd := `root:x:0:0:root:/root:/bin/bash
daemon:x:1:1:daemon:/usr/sbin:/usr/sbin/nologin
laji:x:1000:1000:Laji Host User,,,:/home/laji:/bin/bash
admin:x:1002:1002:Admin User:/home/admin:/bin/bash
nobody:x:65534:65534:nobody:/nonexistent:/usr/sbin/nologin
`
	passwdFile := filepath.Join(etcDir, "passwd")
	if err := os.WriteFile(passwdFile, []byte(rawPasswd), 0644); err != nil {
		t.Fatalf("write raw passwd: %v", err)
	}

	// 3. Prepare raw /etc/group
	rawGroup := `root:x:0:
daemon:x:1:
sudo:x:27:laji
users:x:100:
`
	groupFile := filepath.Join(etcDir, "group")
	if err := os.WriteFile(groupFile, []byte(rawGroup), 0644); err != nil {
		t.Fatalf("write raw group: %v", err)
	}

	// 4. Prepare fake SSH credentials to purge
	sshDir := filepath.Join(etcDir, "ssh")
	_ = os.MkdirAll(sshDir, 0755)
	_ = os.WriteFile(filepath.Join(sshDir, "ssh_host_rsa_key"), []byte("FAKE_HOST_KEY"), 0600)
	_ = os.WriteFile(filepath.Join(sshDir, "ssh_host_rsa_key.pub"), []byte("FAKE_HOST_PUB"), 0644)

	rootSSHDir := filepath.Join(tempDir, "root/.ssh")
	_ = os.MkdirAll(rootSSHDir, 0700)
	_ = os.WriteFile(filepath.Join(rootSSHDir, "id_rsa"), []byte("FAKE_ROOT_KEY"), 0600)

	// Execute Sanitization Pipeline
	if err := builder.SanitizeRootfs(tempDir); err != nil {
		t.Fatalf("SanitizeRootfs failed: %v", err)
	}

	// Verify /etc/shadow
	cleanedShadow, err := os.ReadFile(shadowFile)
	if err != nil {
		t.Fatalf("read cleaned shadow: %v", err)
	}
	shadowContent := string(cleanedShadow)
	t.Logf("Cleaned Shadow:\n%s", shadowContent)

	if strings.Contains(shadowContent, "$6$rounds=50000") || strings.Contains(shadowContent, "$y$j9T$8BMlyIAv") {
		t.Fatalf("Shadow still contains host password hashes! Output: %s", shadowContent)
	}
	if strings.Contains(shadowContent, "laji:") {
		t.Fatalf("Shadow still contains removed host user 'laji'! Output: %s", shadowContent)
	}
	if !strings.Contains(shadowContent, "root:!:") {
		t.Fatalf("Root password was not locked with '!': %s", shadowContent)
	}
	if !strings.Contains(shadowContent, "box:!:") {
		t.Fatalf("Box user was not present with '!' in shadow: %s", shadowContent)
	}

	// Verify /etc/passwd
	cleanedPasswd, err := os.ReadFile(passwdFile)
	if err != nil {
		t.Fatalf("read cleaned passwd: %v", err)
	}
	passwdContent := string(cleanedPasswd)
	t.Logf("Cleaned Passwd:\n%s", passwdContent)

	if strings.Contains(passwdContent, "laji:") || strings.Contains(passwdContent, "admin:") {
		t.Fatalf("Passwd still contains host users (laji/admin)! Output: %s", passwdContent)
	}
	if !strings.Contains(passwdContent, "box:x:1001:1013:AttackBox User:/home/box:/bin/bash") {
		t.Fatalf("Passwd does not contain clean box user: %s", passwdContent)
	}

	// Verify /etc/group
	cleanedGroup, _ := os.ReadFile(groupFile)
	groupContent := string(cleanedGroup)
	t.Logf("Cleaned Group:\n%s", groupContent)
	if !strings.Contains(groupContent, "sudo:x:27:box") {
		t.Fatalf("Sudo group does not contain box user: %s", groupContent)
	}

	// Verify /etc/sudoers.d/attackbox
	sudoersRule, err := os.ReadFile(filepath.Join(etcDir, "sudoers.d/attackbox"))
	if err != nil {
		t.Fatalf("sudoers rule missing: %v", err)
	}
	if !strings.Contains(string(sudoersRule), "box ALL=(ALL) NOPASSWD: ALL") {
		t.Fatalf("Invalid sudoers rule: %s", string(sudoersRule))
	}

	// Verify SSH credentials purged
	if _, err := os.Stat(filepath.Join(sshDir, "ssh_host_rsa_key")); !os.IsNotExist(err) {
		t.Fatalf("Host SSH key was not purged!")
	}
	if _, err := os.Stat(filepath.Join(rootSSHDir, "id_rsa")); !os.IsNotExist(err) {
		t.Fatalf("Root SSH key was not purged!")
	}

	t.Log("[PASS] Golden image deep sanitization pipeline verified successfully!")
}
