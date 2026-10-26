package labcompose

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDirectoryAndWritablePackageAssetsArePrivateAndNeverReset(t *testing.T) {
	ctx := context.Background()
	m, _, _, b := fixtureManager(t)
	os.WriteFile(filepath.Join(b.Directory, "compose.yml"), []byte("services: {web: {image: 'fixture:local', volumes: ['./config:/config:ro', './data:/data', './data:/readonly-data:ro', './seed.txt:/seed']}}"), 0600)
	for _, dir := range []string{"config", "data"} {
		os.Mkdir(filepath.Join(b.Directory, dir), 0700)
		os.WriteFile(filepath.Join(b.Directory, dir, "seed"), []byte("original"), 0600)
	}
	os.WriteFile(filepath.Join(b.Directory, "seed.txt"), []byte("file-seed"), 0600)
	d := prepare(t, m, b)
	a, z := req(d, "asset-alice", "alice"), req(d, "asset-bob", "bob")
	for _, r := range []Request{a, z} {
		if _, err := m.Start(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	mountPaths := func(r Request) map[string]string {
		var doc map[string]any
		if err := decodeFile(filepath.Join(m.cfg.StateDir, "instances", r.ID, "compose.json"), &doc); err != nil {
			t.Fatal(err)
		}
		out := map[string]string{}
		for _, raw := range slice(object(object(doc["services"])["web"])["volumes"]) {
			mount := object(raw)
			out[text(mount["target"])] = text(mount["source"])
		}
		return out
	}
	first, second := mountPaths(a), mountPaths(z)
	if first["/readonly-data"] != first["/data"] {
		t.Fatal("read-only service cannot observe shared initialization")
	}
	if first["/config"] != second["/config"] || first["/data"] == second["/data"] || first["/seed"] == second["/seed"] {
		t.Fatal("shared writable data or lost shared immutable source")
	}
	if err := os.WriteFile(filepath.Join(first["/data"], "seed"), []byte("alice"), 0666); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Start(ctx, a); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(first["/data"], "seed"))
	other, _ := os.ReadFile(filepath.Join(second["/data"], "seed"))
	if string(data) != "alice" || string(other) != "original" {
		t.Fatal("mutable data reset or shared")
	}
	// Cleanup must unlink a malicious symlink without following it outside the copy.
	outside := filepath.Join(t.TempDir(), "keep")
	os.WriteFile(outside, []byte("keep"), 0600)
	os.Symlink(outside, filepath.Join(first["/data"], "link"))
	temporary, err := os.MkdirTemp(filepath.Join(m.cfg.StateDir, "instances", a.ID), ".assets-")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Stop(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(temporary); !os.IsNotExist(err) {
		t.Fatal("abandoned asset staging directory retained")
	}
	if _, err := os.Stat(first["/data"]); !os.IsNotExist(err) {
		t.Fatal("writable copy was not removed")
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatal("cleanup followed symlink")
	}
	if _, err := os.Stat(second["/data"]); err != nil {
		t.Fatal("other instance affected")
	}
	if _, err := m.Stop(ctx, z.ID); err != nil {
		t.Fatal(err)
	}
}
func TestPackageRootWritableMount(t *testing.T) {
	m, _, _, b := fixtureManager(t)
	os.WriteFile(filepath.Join(b.Directory, "compose.yml"), []byte("services: {web: {image: 'fixture:local', volumes: ['.:/workspace']}}"), 0600)
	d := prepare(t, m, b)
	r := req(d, "root-assets", "alice")
	if _, err := m.Start(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(m.cfg.StateDir, "instances", r.ID, "assets", "files", "compose.yml")); err != nil {
		t.Fatal("package root not copied", err)
	}
}
