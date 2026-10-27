package runtime

import (
	"context"
	"testing"
	"time"
)

func TestDesktopRejectsStoppedSandboxAndDoesNotLockOtherUsers(t *testing.T) {
	m := testRuntime(t)
	addTestRecord(m, "alice")
	addTestRecord(m, "bob")
	m.sandboxes["bob"].State = "stopping"
	m.sandboxes["bob"].PID = 123
	// Alice's long-running operation must not hold a global GUI lock.
	m.sandboxes["alice"].opMu.Lock()
	defer m.sandboxes["alice"].opMu.Unlock()
	done := make(chan error, 1)
	go func() { done <- m.EnsureGUI(context.Background(), "bob") }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("started desktop in stopping sandbox")
		}
	case <-time.After(time.Second):
		t.Fatal("another user's operation blocked GUI request")
	}
	if _, err := m.GetGUISocketPath("bob"); err == nil {
		t.Fatal("issued path for stopping sandbox")
	}
	if err := m.EnsureGUI(context.Background(), "missing"); err == nil {
		t.Fatal("created a desktop without a sandbox")
	}
}
