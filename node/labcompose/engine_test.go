package labcompose

import (
	"context"
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
