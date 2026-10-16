package box

import (
	"path/filepath"
	"testing"
)

func TestSnapshotPath(t *testing.T) {
	sm := NewSnapshotManager("/var/lib/attackbox/boxes", "/var/lib/attackbox/base_rootfs")
	p := sm.PathFor("box1")
	expected := filepath.Join("/var/lib/attackbox/boxes", "box1", "rootfs")
	if p != expected {
		t.Fatalf("expected %s, got %s", expected, p)
	}
	if sm.BaseRootfs != "/var/lib/attackbox/base_rootfs" {
		t.Fatalf("unexpected BaseRootfs: %s", sm.BaseRootfs)
	}
}
