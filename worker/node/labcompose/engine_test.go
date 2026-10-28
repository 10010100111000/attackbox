package labcompose

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func scriptCLI(t *testing.T, body string) *CLI {
	t.Helper()
	path := filepath.Join(t.TempDir(), "docker-fixture")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0700); err != nil {
		t.Fatal(err)
	}
	cli, err := NewCLI(CLIConfig{DockerBinary: path, DockerSocket: "/var/run/docker.sock", DockerConfig: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	return cli
}
func TestCLIEnvironmentIsNotInheritedAndErrorsDoNotLeak(t *testing.T) {
	t.Setenv("HOST_PASSWORD", "super-secret")
	t.Setenv("DOCKER_HOST", "tcp://remote:2375")
	t.Setenv("COMPOSE_FILE", "/etc/private.yml")
	c := scriptCLI(t, "/usr/bin/env\n")
	b, err := c.run(context.Background(), "anything")
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"super-secret", "DOCKER_HOST=", "COMPOSE_FILE="} {
		if strings.Contains(string(b), bad) {
			t.Fatal("environment leaked", bad)
		}
	}
	c = scriptCLI(t, "echo super-secret >&2\nexit 1\n")
	if _, err := c.run(context.Background(), "anything"); err == nil || strings.Contains(err.Error(), "super-secret") {
		t.Fatal("Docker error leaked", err)
	}
}
func TestUnlabelledVolumeCollisionIsRejectedBeforeCompose(t *testing.T) {
	r := Record{NodeID: "node-a", Project: "ab_project", Request: Request{ID: "one", OwnerID: "alice", DeploymentID: "dep_test"}, VolumeNames: []string{"ab_project_data"}}
	c := scriptCLI(t, `case "$2 $3" in
"container ls") exit 0;;
"volume ls") echo ab_project_data;;
"volume inspect") echo '[{"Labels":{}}]';;
*) echo 'unexpected mutation' >&2; exit 9;;
esac
`)
	if err := c.Up(context.Background(), r, "/unused"); err == nil || !strings.Contains(err.Error(), "ownership") {
		t.Fatal("unlabelled volume adopted", err)
	}
}
func TestNodeLabelMustMatchExactly(t *testing.T) {
	r := Record{NodeID: "node-a", Project: "ab_project", Request: Request{ID: "one", OwnerID: "alice", DeploymentID: "dep_test"}}
	labels := map[string]string{}
	for k, v := range ownedLabels("node-b", r) {
		labels[k] = v.(string)
	}
	labels["com.docker.compose.project"] = r.Project
	if err := checkLabels(labels, r); err == nil {
		t.Fatal("foreign node accepted")
	}
}

func TestComposeVersionCompatibility(t *testing.T) {
	for _, version := range []string{"2.40.3", "v2.40.4", "2.41.0", "2.99.0", "2.40.3+vendor.1\n"} {
		if err := checkComposeVersion(version); err != nil {
			t.Errorf("%q rejected: %v", version, err)
		}
	}
	for _, version := range []string{"2.40.2", "2.39.9", "1.29.2", "3.0.0", "2.41.0-rc.1", "garbage", "2.41", "2.x.1"} {
		if err := checkComposeVersion(version); err == nil {
			t.Errorf("%q accepted", version)
		}
	}
}

func TestBuildArgumentsStayInPrivateFileAndBuilderUsesLocalDefault(t *testing.T) {
	b := packageFile(t, "services: {web: {build: {context: ., args: {TOKEN: '${TOKEN}'}}}}")
	os.WriteFile(filepath.Join(b.Directory, "Dockerfile"), []byte("FROM scratch\n"), 0600)
	b.Variables = map[string]string{"TOKEN": "literal$private-build-value"}
	d, err := Parse(context.Background(), b, Budget{})
	if err != nil {
		t.Fatal(err)
	}
	stage := t.TempDir()
	log := filepath.Join(stage, "arguments")
	marker := filepath.Join(stage, "built")
	script := fmt.Sprintf(`case "$2 $3" in
"image inspect") if [ -f %q ]; then echo sha256:%s; else exit 1; fi;;
"buildx version") exit 0;;
"compose --ansi") printf '%%s\n' "$@" > %q; touch %q;;
*) exit 9;;
esac
`, marker, strings.Repeat("a", 64), log, marker)
	cli := scriptCLI(t, script)
	if _, err := cli.BuildImage(context.Background(), b.Directory, stage, d.Builds["web"], false); err != nil {
		t.Fatal(err)
	}
	arguments, _ := os.ReadFile(log)
	if strings.Contains(string(arguments), "private-build-value") || !strings.Contains(string(arguments), "--builder\ndefault") {
		t.Fatal("build arguments leaked or remote builder selected")
	}
	encoded, _ := os.ReadFile(filepath.Join(stage, "build.json"))
	if !strings.Contains(string(encoded), "literal$$private-build-value") {
		t.Fatal("literal dollar changed in second Compose parse")
	}
	if _, err := cli.BuildImage(context.Background(), b.Directory, stage, d.Builds["web"], true); err != nil {
		t.Fatal(err)
	}
	forced, _ := os.ReadFile(log)
	if !strings.Contains(string(forced), "--pull\n--no-cache") {
		t.Fatal("force reused cached image or omitted refresh flags")
	}
	info, _ := os.Stat(filepath.Join(stage, "build.json"))
	if info.Mode().Perm()&0077 != 0 {
		t.Fatal("build definition is not private")
	}
}

func TestImageRefreshPullsEvenWhenTagAlreadyExists(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "pulled")
	cli := scriptCLI(t, fmt.Sprintf(`case "$2 $3" in
"image inspect") echo sha256:%s;;
"image pull") touch %q;;
*) exit 9;;
esac
`, strings.Repeat("a", 64), marker))
	if _, err := cli.PrepareImage(context.Background(), "fixture:local"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("ordinary prepare pulled cached image")
	}
	if _, err := cli.RefreshImage(context.Background(), "fixture:local"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("force refresh did not pull")
	}
}

func TestInventoryUsesUnixSocketWithoutSpawningSubprocess(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "d.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Skipf("unix socket listen unavailable: %v", err)
	}
	defer ln.Close()

	r := Record{
		NodeID:  "node-a",
		Project: "ab_project",
		Request: Request{ID: "one", OwnerID: "alice", DeploymentID: "dep_test"},
	}
	cid := strings.Repeat("a", 64)
	mux := http.NewServeMux()
	mux.HandleFunc("/containers/json", func(w http.ResponseWriter, req *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{{"Id": cid}})
	})
	mux.HandleFunc("/containers/"+cid+"/json", func(w http.ResponseWriter, req *http.Request) {
		labels := map[string]string{"com.docker.compose.project": r.Project, "com.docker.compose.service": "web"}
		for k, v := range ownedLabels(r.NodeID, r) {
			labels[k] = v.(string)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Id":     cid,
			"Config": map[string]any{"Labels": labels},
			"State":  map[string]any{"Status": "running", "ExitCode": 0, "Health": map[string]any{"Status": "healthy"}},
			"NetworkSettings": map[string]any{
				"Networks": map[string]any{"net_1": map[string]any{"IPAddress": "10.201.0.2"}},
			},
		})
	})
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	// Use a binary path named "docker" that exits 1 if executed, proving zero subprocesses are spawned.
	binPath := filepath.Join(dir, "docker")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	cli, err := NewCLI(CLIConfig{DockerBinary: binPath, DockerSocket: sockPath, DockerConfig: dir})
	if err != nil {
		t.Fatal(err)
	}
	services, err := cli.Inspect(context.Background(), r)
	if err != nil {
		t.Fatalf("Inspect over unix socket failed: %v", err)
	}
	if len(services) != 1 || services[0].Name != "web" || services[0].State != "running" || services[0].Health != "healthy" || services[0].IPs["net_1"] != "10.201.0.2" {
		t.Fatalf("unexpected services: %+v", services)
	}
}
