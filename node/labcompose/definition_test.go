package labcompose

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func packageFile(t *testing.T, content string) Bundle {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "compose.yml"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return Bundle{Directory: dir, File: "compose.yml", EntryService: "web", EntryPort: 8080}
}

const twoServices = `services:
  web:
    image: fixture:local
    depends_on:
      init:
        condition: service_completed_successfully
    volumes: [data:/data]
  init:
    image: fixture:local
    command: ["write", "/data/seed", "seed"]
    volumes: [data:/data]
volumes:
  data: {}
`

func TestParseRetainsDependenciesAndAppliesWholeProjectBudget(t *testing.T) {
	d, err := Parse(context.Background(), packageFile(t, twoServices), Budget{})
	if err != nil {
		t.Fatal(err)
	}
	if !d.OneShot["init"] || len(d.Networks) != 1 || d.EntryNetwork != "default" {
		t.Fatal(d)
	}
	var mem, cpu, pids float64
	for _, s := range object(d.Document["services"]) {
		svc := object(s)
		mem += svc["mem_limit"].(float64)
		cpu += svc["cpus"].(float64)
		pids += svc["pids_limit"].(float64)
		if svc["pull_policy"] != "never" {
			t.Fatal(svc)
		}
	}
	if mem != 1<<30 || cpu != 2 || pids != 256 {
		t.Fatal("per-service budget duplicated", mem, cpu, pids)
	}
}

func TestParserRejectsRemoteExecutionAndUnsafeFeatures(t *testing.T) {
	for _, content := range []string{
		"include: [https://example.com/private.yml]\nservices: {}",
		"services: {web: {image: fixture:local, build: .}}",
		"services: {web: {image: fixture:local, privileged: true}}",
		"services: {web: {image: fixture:local, ports: ['8080:80']}}",
		"services: {web: {image: fixture:local, env_file: /etc/passwd}}",
		"services: {web: {image: fixture:local, networks: [front]}}\nnetworks: {front: {external: true}}",
		"services: {web: {image: fixture:local, volumes: ['/etc/passwd:/leak:ro']}}",
		"services: {web: {image: fixture:local, volumes: ['./missing:/file:ro']}}",
		"services: {web: {image: fixture:local, profiles: [test]}}",
		"services: {web: {image: fixture:local, image: overwritten:tag}}",
	} {
		t.Run(content, func(t *testing.T) {
			if _, err := Parse(context.Background(), packageFile(t, content), Budget{}); err == nil {
				t.Fatal("unsafe/unsupported input accepted")
			}
		})
	}
}

func TestPackageReadOnlyFileAndLiteralVariableIsolation(t *testing.T) {
	b := packageFile(t, "services:\n  web:\n    image: fixture:local\n    environment: {TOKEN: '${TOKEN:?required}', HOST_LEAK: '${HOST_LEAK:-empty}'}\n    volumes: ['./init.sql:/init.sql:ro']\n")
	b.Variables = map[string]string{"TOKEN": "literal$secret"}
	t.Setenv("HOST_LEAK", "host-secret")
	if err := os.WriteFile(filepath.Join(b.Directory, "init.sql"), []byte("select 1;"), 0600); err != nil {
		t.Fatal(err)
	}
	d, err := Parse(context.Background(), b, Budget{})
	if err != nil {
		t.Fatal(err)
	}
	env := object(object(object(d.Document["services"])["web"])["environment"])
	if env["TOKEN"] != "literal$secret" || env["HOST_LEAK"] != "empty" || len(d.Files) != 1 {
		t.Fatal("environment inherited or file omitted", env, d.Files)
	}
	out, _ := json.Marshal(escapeDollars(cloneDocument(d.Document)))
	if !strings.Contains(string(out), "literal$$secret") {
		t.Fatal("second interpolation not escaped")
	}
	if err := os.Remove(filepath.Join(b.Directory, "init.sql")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/etc/passwd", filepath.Join(b.Directory, "init.sql")); err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(context.Background(), b, Budget{}); err == nil {
		t.Fatal("symlink accepted")
	}
}
