package node

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"attackbox/node/labcompose"
	"attackbox/node/labnetwork"
	"attackbox/protocol"
)

type dispatchEngine struct {
	started, down chan struct{}
	once          sync.Once
}

func (e *dispatchEngine) Check(context.Context) error { return nil }
func (e *dispatchEngine) PrepareImage(context.Context, string) (string, error) {
	return "sha256:" + strings.Repeat("a", 64), nil
}
func (e *dispatchEngine) ImagePresent(context.Context, string) error { return nil }
func (e *dispatchEngine) Up(ctx context.Context, _ labcompose.Record, _ string) error {
	close(e.started)
	<-ctx.Done()
	return ctx.Err()
}
func (e *dispatchEngine) Down(context.Context, labcompose.Record, string) error {
	e.once.Do(func() { close(e.down) })
	return nil
}
func (e *dispatchEngine) Inspect(context.Context, labcompose.Record) ([]labcompose.Service, error) {
	return nil, nil
}

type dispatchNetworks struct {
	mu      sync.Mutex
	network labnetwork.Network
}

func (n *dispatchNetworks) CreateNetwork(_ context.Context, s labnetwork.NetworkSpec) (labnetwork.Network, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.network = labnetwork.Network{Spec: s, ID: s.TargetID + "/" + s.Key, Name: "test-net", Desired: true}
	return n.network, nil
}
func (n *dispatchNetworks) ReleaseNetwork(context.Context, string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.network.Cleaned = true
	return nil
}
func (n *dispatchNetworks) Attach(context.Context, string, labnetwork.Sandbox) (labnetwork.Attachment, error) {
	return labnetwork.Attachment{}, nil
}
func (n *dispatchNetworks) Detach(context.Context, string) error { return nil }
func (n *dispatchNetworks) Snapshot() ([]labnetwork.Network, []labnetwork.Attachment) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return []labnetwork.Network{n.network}, nil
}

func TestComposeStopHasCapacityWhenStartDispatchSlotsAreFull(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "compose.yml"), []byte("services: {web: {image: 'fixture:local'}}"), 0600)
	engine := &dispatchEngine{started: make(chan struct{}), down: make(chan struct{})}
	rt, err := labcompose.New(ctx, labcompose.Config{NodeID: "test-node", StateDir: filepath.Join(dir, "state"), Engine: engine, Networks: &dispatchNetworks{}})
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	dep, err := rt.Prepare(ctx, labcompose.Bundle{Directory: dir, File: "compose.yml", EntryPort: 8080})
	if err != nil {
		t.Fatal(err)
	}
	req := labcompose.Request{ID: "tgt_" + strings.Repeat("a", 32), OwnerID: "alice", DeploymentID: dep.ID, ExpiresAt: time.Now().Add(30 * time.Minute)}
	done := make(chan struct{})
	go func() { rt.Start(ctx, req); close(done) }()
	select {
	case <-engine.started:
	case <-time.After(3 * time.Second):
		t.Fatal("start did not reach engine")
	}
	labs := &composeLabs{runtime: rt, ctx: ctx, commands: make(chan struct{}, 8), stops: make(chan struct{}, 4)}
	for i := 0; i < cap(labs.commands); i++ {
		labs.commands <- struct{}{}
	}
	c := &NodeControlClient{bootID: "boot", compose: labs, stopCh: make(chan struct{})}
	msg, _ := protocol.NewMessage(protocol.MsgComposeStop, protocol.ComposeRequest{BootID: "boot", TargetID: req.ID, OwnerID: req.OwnerID, DeploymentID: dep.ID, ExpiresAt: req.ExpiresAt})
	c.dispatchCompose(msg)
	select {
	case <-engine.down:
	case <-time.After(3 * time.Second):
		t.Fatal("stop was rejected behind saturated start slots")
	}
	<-done
	labs.wg.Wait()
}

func TestComposePreparationIdentityUsesContentNotSourceSnapshot(t *testing.T) {
	a := protocol.ComposeRequest{OwnerID: "template", BundleID: "old", BundleHash: strings.Repeat("a", 64), Variables: map[string]string{"PASSWORD": "one"}}
	b := a
	b.BundleID = "new"
	if composeRequestFingerprint(a) != composeRequestFingerprint(b) {
		t.Fatal("same content lost preparation on metadata update")
	}
	b.BundleHash = strings.Repeat("b", 64)
	if composeRequestFingerprint(a) == composeRequestFingerprint(b) {
		t.Fatal("different content reused preparation")
	}
	b = a
	b.Variables = map[string]string{"PASSWORD": "two"}
	if composeRequestFingerprint(a) == composeRequestFingerprint(b) {
		t.Fatal("variable change reused preparation")
	}
}
