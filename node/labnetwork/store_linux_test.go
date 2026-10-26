//go:build linux

package labnetwork

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStateChildIsPrivateAndBroadExistingDirectoryRejected(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	s, _, err := openStore(dir, "node-a")
	if err != nil {
		t.Fatal(err)
	}
	s.close()
	info, err := os.Stat(dir)
	if err != nil || info.Mode().Perm() != 0700 {
		t.Fatal("private directory not created", info, err)
	}
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if s, _, err := openStore(dir, "node-a"); err == nil {
		s.close()
		t.Fatal("broad existing state directory accepted")
	}
}
