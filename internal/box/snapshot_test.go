package box

import (
	"path/filepath"
	"testing"
)

func TestSnapshotPath(t *testing.T) {
	sm := NewSnapshotManager("/var/lib/attackbox/boxes")
	p := sm.PathFor("box1")
	expected := filepath.Join("/var/lib/attackbox/boxes", "box1", "rootfs")
	if p != expected {
		t.Fatalf("expected %s, got %s", expected, p)
	}
}
