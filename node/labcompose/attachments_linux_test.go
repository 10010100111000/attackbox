//go:build linux

package labcompose

import (
	"context"
	"sync"
	"testing"
	"time"

	"attackbox/node/labnetwork"
)

func TestBusyInstanceRetainsLifecycleEventWithoutInspectRepair(t *testing.T) {
	m, _, n, bundle := fixtureManager(t)
	ctx := context.Background()
	d := prepare(t, m, bundle)
	var lifecycle sync.Mutex
	var current *labnetwork.Sandbox
	m.cfg.SandboxAccess = func(ctx context.Context, owner string, fn func(context.Context, labnetwork.Sandbox) error) error {
		lifecycle.Lock()
		defer lifecycle.Unlock()
		if current != nil {
			return fn(ctx, *current)
		}
		return nil
	}
	r := req(d, "one", "alice")
	if _, err := m.Start(ctx, r); err != nil {
		t.Fatal(err)
	}
	e, _ := m.get(r.ID)
	// Represents a slow Compose operation holding op, while sandbox creation
	// invokes the lifecycle hook under its own lock. The hook must not wait.
	e.op.Lock()
	locked := true
	defer func() {
		if locked {
			e.op.Unlock()
		}
	}()
	s := labnetwork.Sandbox{ID: "old", OwnerID: "alice", PID: 123, Namespace: labnetwork.NamespaceIdentity{Device: 1, Inode: 2}}
	lifecycle.Lock()
	current = &s
	cancelled, cancel := context.WithCancel(ctx)
	cancel() // Lifecycle facts outlive the sandbox creator's request context.
	if err := m.SandboxAvailable(cancelled, s); err != nil {
		t.Fatal(err)
	}
	lifecycle.Unlock()
	for i := 0; i < 3; i++ {
		status, err := m.Inspect(ctx, r.ID)
		if err != nil || status.Attachment == "connected" {
			t.Fatal("Inspect changed networking", status, err)
		}
	}
	n.mu.Lock()
	calls := n.attachCalls
	n.mu.Unlock()
	if calls != 0 {
		t.Fatal("busy instance unexpectedly mutated", calls)
	}
	// Replace the sandbox before the queued work can run. The worker must use
	// the current lifecycle-locked identity, not a stale event PID.
	lifecycle.Lock()
	current = nil
	if err := m.SandboxGone(ctx, s.ID); err != nil {
		t.Fatal(err)
	}
	s2 := s
	s2.ID = "new"
	s2.Namespace.Inode++
	current = &s2
	if err := m.SandboxAvailable(ctx, s2); err != nil {
		t.Fatal(err)
	}
	lifecycle.Unlock()
	e.op.Unlock()
	locked = false
	waitAttachment(t, n, s2.ID, true)
	_, attachments := n.Snapshot()
	for _, a := range attachments {
		if a.Sandbox.ID == s.ID {
			t.Fatal("queued event attached stale namespace", a)
		}
	}
	e.op.Lock()
	locked = true
	n.mu.Lock()
	calls = n.attachCalls
	n.mu.Unlock()
	if _, err := m.Inspect(ctx, r.ID); err != nil {
		t.Fatal(err)
	}
	n.mu.Lock()
	after := n.attachCalls
	n.mu.Unlock()
	if calls != after {
		t.Fatal("Inspect performed an attachment")
	}
	e.op.Unlock()
	locked = false
	lifecycle.Lock()
	current = nil
	if err := m.SandboxGone(ctx, s2.ID); err != nil {
		t.Fatal(err)
	}
	lifecycle.Unlock()
	waitAttachment(t, n, s2.ID, false)
}

func TestQueuedAttachmentCannotReviveStoppedInstance(t *testing.T) {
	m, engine, n, bundle := fixtureManager(t)
	ctx := context.Background()
	d := prepare(t, m, bundle)
	r := req(d, "one", "alice")
	if _, err := m.Start(ctx, r); err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	engine.beforeDown = func() { close(entered); <-release }
	stopped := make(chan error, 1)
	go func() { _, err := m.Stop(ctx, r.ID); stopped <- err }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("stop did not begin")
	}
	s := labnetwork.Sandbox{ID: "sandbox", OwnerID: "alice", PID: 123, Namespace: labnetwork.NamespaceIdentity{Device: 1, Inode: 2}}
	if err := m.SandboxAvailable(ctx, s); err != nil {
		t.Fatal(err)
	}
	once.Do(func() { close(release) })
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("stop did not finish")
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	_, attachments := n.Snapshot()
	if len(attachments) != 0 {
		t.Fatal("stopped instance gained an attachment")
	}
}
