package runtime

import (
	"attackbox/node/labnetwork"
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"
)

func (m *RuntimeManager) CreateSandbox(ctx context.Context, scfg SandboxConfig) (result SandboxSnapshot, err error) {
	if !sandboxIDPattern.MatchString(scfg.ID) {
		return result, errors.New("invalid sandbox ID")
	}
	if m.diskGuard.IsTripped() {
		return result, ErrDiskFuseTripped
	}
	rec := &SandboxRecord{ID: scfg.ID, OwnerID: scfg.OwnerID, UserID: scfg.UserID, SlotIndex: scfg.SlotIndex, State: "creating", StartedAt: time.Now(), LastActiveAt: time.Now()}
	ttl := scfg.TTLSeconds
	if ttl <= 0 {
		ttl = m.cfg.SandboxTTLSec
	}
	idle := scfg.IdleSeconds
	if idle <= 0 {
		idle = m.cfg.SandboxIdleSec
	}
	rec.ExpiresAt = rec.StartedAt.Add(time.Duration(ttl) * time.Second)
	rec.idleLimit = time.Duration(idle) * time.Second
	rec.opMu.Lock()
	defer rec.opMu.Unlock()
	m.mu.Lock()
	if _, exists := m.sandboxes[scfg.ID]; exists {
		m.mu.Unlock()
		return result, ErrSandboxExists
	}
	m.revision++
	rec.Revision = m.revision
	m.sandboxes[scfg.ID] = rec
	m.mu.Unlock()
	defer func() {
		if err != nil {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			err = errors.Join(err, m.destroyRecord(cleanupCtx, rec))
		}
	}()
	limits := LimitsFromPolicy(scfg.Policy.Resources, m.rootDeviceNumber)
	cgFile, _, err := m.cgroups.Create(scfg.ID, limits)
	if err != nil {
		return SandboxSnapshot{}, fmt.Errorf("create cgroup: %w", err)
	}
	defer cgFile.Close()

	rootfsPath, err := m.btrfs.CreateSnapshot(ctx, scfg.ID)
	if err != nil {

		return SandboxSnapshot{}, fmt.Errorf("create btrfs snapshot: %w", err)
	}

	rPipe, wPipe, err := os.Pipe()
	if err != nil {

		return SandboxSnapshot{}, fmt.Errorf("create sync pipe: %w", err)
	}
	defer rPipe.Close()
	defer wPipe.Close()

	exePath, err := os.Executable()
	if err != nil {
		exePath = "/proc/self/exe"
	}

	policyBytes, err := json.Marshal(scfg.Policy)
	if err != nil {

		return SandboxSnapshot{}, fmt.Errorf("marshal policy json: %w", err)
	}

	cmd := exec.Command(exePath, "__init__", scfg.ID, rootfsPath)
	cmd.ExtraFiles = []*os.File{wPipe}
	cmd.Env = append(os.Environ(), "ATTACKBOX_POLICY="+string(policyBytes))
	configureSysProcAttr(cmd, cgFile)

	if err := cmd.Start(); err != nil {
		_ = wPipe.Close()

		return SandboxSnapshot{}, fmt.Errorf("start container init process: %w", err)
	}
	_ = wPipe.Close()
	m.mu.Lock()
	rec.cmd = cmd
	rec.PID = cmd.Process.Pid
	rec.waitDone = make(chan struct{})
	m.mu.Unlock()
	// Capture before starting Wait: an unreaped child PID cannot be reused.
	identity, captureErr := labnetwork.CaptureNamespace(cmd.Process.Pid)
	m.mu.Lock()
	rec.Namespace = identity
	m.mu.Unlock()
	go func() {
		_ = cmd.Wait()
		close(rec.waitDone)
		rec.opMu.Lock()
		defer rec.opMu.Unlock()
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.sandboxes[rec.ID] == rec && rec.State == "running" {
			rec.State = "cleanup_failed"
			rec.CleanupError = "container init exited"
			m.revision++
			rec.Revision = m.revision
		}
	}()

	readyChan := make(chan error, 1)
	go func() {
		reader := bufio.NewReader(rPipe)
		line, err := reader.ReadString('\n')
		if err != nil {
			readyChan <- fmt.Errorf("read init ready signal: %w", err)
			return
		}
		if line != "READY\n" {
			readyChan <- fmt.Errorf("unexpected ready signal: %q", line)
			return
		}
		readyChan <- nil
	}()

	select {
	case <-ctx.Done():

		return SandboxSnapshot{}, ctx.Err()
	case <-time.After(10 * time.Second):

		return SandboxSnapshot{}, ErrInitTimeout
	case err := <-readyChan:
		if err != nil {

			return SandboxSnapshot{}, fmt.Errorf("container init failed: %w", err)
		}
	}

	sandboxIP := scfg.IP
	if captureErr != nil {
		return SandboxSnapshot{}, fmt.Errorf("capture sandbox namespace: %w", captureErr)
	}
	if m.netSetup != nil && scfg.SlotIndex > 0 {
		ip, netErr := m.netSetup(ctx, scfg.SlotIndex, labnetwork.Sandbox{ID: rec.ID, OwnerID: rec.OwnerID, PID: rec.PID, Namespace: rec.Namespace})
		if netErr != nil {

			return SandboxSnapshot{}, fmt.Errorf("setup sandbox network: %w", netErr)
		}
		sandboxIP = ip
	}

	m.mu.Lock()
	rec.IP = sandboxIP
	rec.State = "running"
	m.revision++
	rec.Revision = m.revision
	result = snapshotOf(rec)
	m.mu.Unlock()
	// Attachment errors do not invalidate an otherwise running CLI sandbox.
	_ = m.notifyLabReady(ctx, rec)
	return result, nil
}
