package node

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestComposeBundleDownloadIdentityDigestAndPaths(t *testing.T) {
	id := "00000000-0000-4000-8000-000000000001"
	file := []byte("services: {}\n")
	hash := bundleDigest([]byte("nested/compose.yml\x00" + bundleDigest(file) + "\n"))
	badPath := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/worker/lab-bundles/"+id || r.Header.Get("Authorization") != "Bearer test-cluster" || r.Header.Get("X-Node-ID") != "test-node" {
			t.Error("download escaped configured controller or lost authentication")
		}
		name := "nested/compose.yml"
		if badPath {
			name = "../escape"
		}
		json.NewEncoder(w).Encode(map[string]any{"id": id, "hash": hash, "files": map[string]string{name: base64.StdEncoding.EncodeToString(file)}})
	}))
	defer srv.Close()
	cfg := &NodeConfig{Node: NodeInfoConfig{NodeID: "test-node", ClusterSecret: "test-cluster", ControlURL: strings.Replace(srv.URL, "http:", "ws:", 1) + "/cluster/connect"}, ComposeLabs: ComposeConfig{StateDir: filepath.Join(t.TempDir(), "compose")}}
	dir, err := downloadComposeBundle(context.Background(), cfg, id, hash)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	if b, err := os.ReadFile(filepath.Join(dir, "nested", "compose.yml")); err != nil || string(b) != string(file) {
		t.Fatal(err)
	}
	badPath = true
	if _, err := downloadComposeBundle(context.Background(), cfg, id, hash); err == nil {
		t.Fatal("traversal accepted")
	}
	badPath = false
	if _, err := downloadComposeBundle(context.Background(), cfg, id, strings.Repeat("a", 64)); err == nil {
		t.Fatal("digest mismatch accepted")
	}
	cfg.Node.ControlURL = "ws://remote.invalid/cluster/connect"
	if _, err := downloadComposeBundle(context.Background(), cfg, id, hash); err == nil {
		t.Fatal("remote plaintext download accepted")
	}
}
func TestComposeBundleNeverFollowsRedirects(t *testing.T) {
	visited := false
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { visited = true }))
	defer other.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, other.URL, 302) }))
	defer srv.Close()
	cfg := &NodeConfig{Node: NodeInfoConfig{ControlURL: strings.Replace(srv.URL, "http:", "ws:", 1)}, ComposeLabs: ComposeConfig{StateDir: t.TempDir()}}
	_, err := downloadComposeBundle(context.Background(), cfg, "00000000-0000-4000-8000-000000000001", strings.Repeat("a", 64))
	if err == nil || visited {
		t.Fatal("redirect followed")
	}
}
func TestComposeRequiresExplicitNetworkAcceptance(t *testing.T) {
	if _, err := newComposeLabs(&NodeConfig{ComposeLabs: ComposeConfig{Enabled: true}}, nil); err == nil || err.Error() != "compose_network_not_verified" {
		t.Fatal("unverified network activated", err)
	}
}
