package labcompose

import (
	"attackbox/node/labnetwork"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoppedInstanceSurvivesRestart(t *testing.T) {
	m, e, _, b := fixtureManager(t)
	d := prepare(t, m, b)
	r := req(d, "review-replay", "alice")
	ctx := context.Background()
	if _, err := m.Start(ctx, r); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Stop(ctx, r.ID); err != nil {
		t.Fatal(err)
	}
	before := e.upCount
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(ctx, m.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	s, err := reopened.Start(ctx, r)
	if err != nil || s.State != "stopped" || e.upCount != before {
		t.Fatalf("stopped request revived after restart: state=%s, Up calls %d -> %d", s.State, before, e.upCount)
	}
}
func TestInterruptedCleanupResumesAfterRestart(t *testing.T) {
	m, _, _, b := fixtureManager(t)
	d := prepare(t, m, b)
	r := req(d, "review-cleanup", "alice")
	ctx := context.Background()
	if _, err := m.Start(ctx, r); err != nil {
		t.Fatal(err)
	}
	// Make the asset marker removal fail, after all networks have been released.
	marker := filepath.Join(m.cfg.StateDir, "instances", r.ID, "assets.json")
	if err := os.Remove(marker); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.Mkdir(marker, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(marker, "block"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Stop(ctx, r.ID); err == nil {
		t.Fatal("expected interrupted cleanup")
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	// Reopen with the network ledger retained until parent acknowledgement.
	if err := os.RemoveAll(marker); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(ctx, m.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	s, err := reopened.Stop(ctx, r.ID)
	if err != nil || s.State != "stopped" {
		t.Fatalf("cleanup cannot resume: state=%s, err=%v", s.State, err)
	}
}

func TestStoppedAcknowledgementRetriesAfterRestart(t *testing.T) {
	m, e, n, b := fixtureManager(t)
	ctx := context.Background()
	d := prepare(t, m, b)
	r := req(d, "ack-retry", "alice")
	if _, err := m.Start(ctx, r); err != nil {
		t.Fatal(err)
	}
	n.forgetErr = fmt.Errorf("injected acknowledgement failure")
	if _, err := m.Stop(ctx, r.ID); err == nil {
		t.Fatal("acknowledgement failure hidden")
	}
	if !m.entries[r.ID].snapshot().Cleaned {
		t.Fatal("terminal state must precede acknowledgement")
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	n.forgetErr = nil
	reopened, err := New(ctx, m.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.Reap(ctx); err != nil {
		t.Fatal(err)
	}
	saved := reopened.entries[r.ID].snapshot()
	if !saved.Cleaned || len(saved.Networks) != 0 || len(saved.VolumeNames) != 0 || saved.StartIssued {
		t.Fatal("stopped record not compacted", saved)
	}
	if e.downCount != 1 {
		t.Fatal("completed cleanup reran", e.downCount)
	}
	files, err := os.ReadDir(filepath.Join(m.cfg.StateDir, "instances", r.ID))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Name() != "record.json" {
		t.Fatal("stopped instance retains runtime assets", files)
	}
	reopened.clock = func() time.Time { return r.ExpiresAt.Add(time.Second) }
	if err := reopened.Reap(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(m.cfg.StateDir, "instances", r.ID)); !os.IsNotExist(err) {
		t.Fatal("expired tombstone retained", err)
	}
	if _, err := reopened.Start(ctx, r); err == nil {
		t.Fatal("expired request accepted")
	}
}

func TestTerminalCommitFailureDoesNotPublishStopped(t *testing.T) {
	m, e, n, b := fixtureManager(t)
	ctx := context.Background()
	d := prepare(t, m, b)
	r := req(d, "commit-failure", "alice")
	if _, err := m.Start(ctx, r); err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(m.cfg.StateDir, "instances", r.ID, "record.json")
	backup := filepath.Join(t.TempDir(), "record.json")
	e.beforeDown = func() {
		if err := os.Rename(record, backup); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(record, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := m.Stop(ctx, r.ID); err == nil {
		t.Fatal("terminal write failure hidden")
	}
	if m.entries[r.ID].snapshot().Cleaned {
		t.Fatal("uncommitted stop was published")
	}
	if len(n.nets) == 0 {
		t.Fatal("network recovery records acknowledged before commit")
	}
	e.beforeDown = nil
	if err := os.Remove(record); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(backup, record); err != nil {
		t.Fatal(err)
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(ctx, m.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	s, err := reopened.Stop(ctx, r.ID)
	if err != nil || s.State != "stopped" {
		t.Fatal("repaired ledger could not recover", s, err)
	}
}

func TestCleanupAcknowledgesNetworksDiscoveredOutsideInstanceRecord(t *testing.T) {
	m, _, n, b := fixtureManager(t)
	ctx := context.Background()
	d := prepare(t, m, b)
	r := req(d, "discovered-network", "alice")
	if _, err := m.Start(ctx, r); err != nil {
		t.Fatal(err)
	}
	// Network creation can succeed before its instance-ledger write is committed.
	if _, err := n.CreateNetwork(ctx, labnetwork.NetworkSpec{TargetID: r.ID, OwnerID: r.OwnerID, Key: "orphan"}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Stop(ctx, r.ID); err != nil {
		t.Fatal(err)
	}
	if len(n.nets) != 0 {
		t.Fatal("discovered network record was not acknowledged", n.nets)
	}
}
