//go:build linux

package labcompose

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDockerCancellationTerminatesPluginChildren(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "child.pid")
	cli := scriptCLI(t, fmt.Sprintf("sleep 30 &\necho $! > %q\nwait\n", marker))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := cli.run(ctx, "compose", "build"); done <- err }()
	var pid string
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		if b, err := os.ReadFile(marker); err == nil {
			pid = strings.TrimSpace(string(b))
			if pid != "" {
				break
			}
		}
		time.Sleep(time.Millisecond)
	}
	if pid == "" {
		t.Fatal("plugin child did not start")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled CLI reported success")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancel waited for inherited plugin pipe")
	}
	// An orphan may briefly be a zombie awaiting the environment's init reaper,
	// but it must no longer be executing or holding the build connection open.
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		b, err := os.ReadFile("/proc/" + pid + "/stat")
		if os.IsNotExist(err) {
			return
		}
		if err == nil {
			stat := string(b)
			fields := strings.Fields(stat[strings.LastIndex(stat, ")")+1:])
			if len(fields) > 0 && fields[0] == "Z" {
				return
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("plugin child still running")
}
