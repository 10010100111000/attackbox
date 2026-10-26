package node

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"attackbox/node/labcompose"
	"attackbox/protocol"
)

func preparationRequest(n int) protocol.ComposeRequest {
	id := fmt.Sprintf("00000000-0000-4000-8000-%012d", n)
	return protocol.ComposeRequest{RequestedAt: time.Now(), PreparationID: id, TargetID: id, OwnerID: "template", BundleHash: "hash"}
}
func missingDeployment(context.Context, string) (labcompose.Deployment, error) {
	return labcompose.Deployment{}, fmt.Errorf("missing")
}
func waitPreparation(t *testing.T, p *composePreparations, r protocol.ComposeRequest, state string) composeJob {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		j := p.handle(r, protocol.MsgComposePrepareStatus)
		if j.State == state {
			return j
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("preparation did not reach %s", state)
	return composeJob{}
}
func TestPreparationCancelWinsCompletionAndRetryUsesNewAttempt(t *testing.T) {
	var calls atomic.Int32
	started := make(chan struct{})
	dep := labcompose.Deployment{ID: "prepared"}
	p := newComposePreparations(context.Background(), func(ctx context.Context, r protocol.ComposeRequest) (labcompose.Deployment, error) {
		if calls.Add(1) == 1 {
			close(started)
			<-ctx.Done()
		}
		return dep, nil
	}, missingDeployment)
	defer p.Close()
	r := preparationRequest(1)
	if p.handle(r, protocol.MsgComposePrepare).State != "preparing" {
		t.Fatal("not admitted")
	}
	<-started
	p.handle(r, protocol.MsgComposePrepare)
	if calls.Load() != 1 {
		t.Fatal("duplicate attempt started")
	}
	foreign := r
	foreign.OwnerID = "other"
	if p.handle(foreign, protocol.MsgComposePrepareCancel).Error == "" {
		t.Fatal("foreign owner cancelled task")
	}
	if state := p.handle(r, protocol.MsgComposePrepareCancel).State; state != "cancelling" && state != "cancelled" {
		t.Fatal(state)
	}
	cancelled := waitPreparation(t, p, r, "cancelled")
	if cancelled.DeploymentID != "" {
		t.Fatal("cancel published completed artifact")
	}
	if p.handle(r, protocol.MsgComposePrepare).State != "cancelled" {
		t.Fatal("cancelled attempt resurrected")
	}
	next := preparationRequest(2)
	next.Rebuild = true
	p.handle(next, protocol.MsgComposePrepare)
	if waitPreparation(t, p, next, "ready").DeploymentID != "prepared" || calls.Load() != 2 {
		t.Fatal("retry failed")
	}
}
func TestCancellationTombstonePrecedesDelayedPrepareAndRecoveryDoesNotInventCompletion(t *testing.T) {
	var calls atomic.Int32
	p := newComposePreparations(context.Background(), func(context.Context, protocol.ComposeRequest) (labcompose.Deployment, error) {
		calls.Add(1)
		return labcompose.Deployment{}, nil
	}, func(context.Context, string) (labcompose.Deployment, error) {
		return labcompose.Deployment{ID: "old-deployment"}, nil
	})
	defer p.Close()
	r := preparationRequest(1)
	p.handle(r, protocol.MsgComposePrepareCancel)
	if p.handle(r, protocol.MsgComposePrepare).State != "cancelled" || calls.Load() != 0 {
		t.Fatal("late prepare ran after cancel")
	}
	r = preparationRequest(2)
	r.DeploymentID = "old-deployment"
	recovered := p.handle(r, protocol.MsgComposePrepareStatus)
	if recovered.State != "unknown" || recovered.DeploymentID != "old-deployment" {
		t.Fatal("lost attempt incorrectly reported successful")
	}
}
func TestPreparationAdmissionExpiryAndBoundedTombstoneRetention(t *testing.T) {
	p := newComposePreparations(context.Background(), func(context.Context, protocol.ComposeRequest) (labcompose.Deployment, error) {
		return labcompose.Deployment{ID: "prepared"}, nil
	}, missingDeployment)
	defer p.Close()
	for i := 0; i < 256; i++ {
		r := preparationRequest(i)
		p.handle(r, protocol.MsgComposePrepareCancel)
	}
	r := preparationRequest(300)
	if p.handle(r, protocol.MsgComposePrepare).Error != "compose_prepare_busy" {
		t.Fatal("recent cancellation tombstone evicted")
	}
	old := preparationRequest(301)
	old.RequestedAt = time.Now().Add(-4 * time.Minute)
	if p.handle(old, protocol.MsgComposePrepare).Error != "preparation_request_expired" {
		t.Fatal("expired request admitted")
	}
	p.mu.Lock()
	for id, job := range p.jobs {
		job.retainedAt = time.Now().Add(-4 * time.Minute)
		p.jobs[id] = job
	}
	p.mu.Unlock()
	p.handle(r, protocol.MsgComposePrepare)
	waitPreparation(t, p, r, "ready")
	p.mu.Lock()
	size := len(p.jobs)
	p.mu.Unlock()
	if size > 256 {
		t.Fatal("unbounded preparation cache")
	}
}
