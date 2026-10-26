package labcompose

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type buildingEngine struct {
	*fakeEngine
	builds int
}

func (e *buildingEngine) BuildImage(_ context.Context, _ string, _ string, spec BuildSpec) (string, error) {
	e.builds++
	return "sha256:" + buildKey(spec), nil
}
func TestLocalBuildParsingAndPreparedImageOnlyRuntime(t *testing.T) {
	b := packageFile(t, "services:\n  web:\n    image: upstream:do-not-overwrite\n    build: {context: ./app, args: {VALUE: '${VALUE:-fallback}', OMITTED: null}}\n  other:\n    build: {context: ./app, args: {VALUE: '${VALUE:-fallback}', OMITTED: null}}\n")
	os.Mkdir(filepath.Join(b.Directory, "app"), 0700)
	os.WriteFile(filepath.Join(b.Directory, "app", "Dockerfile"), []byte("FROM scratch\nARG VALUE\n"), 0600)
	os.WriteFile(filepath.Join(b.Directory, "app", ".dockerignore"), []byte("ignored\n"), 0600)
	b.Variables = map[string]string{"VALUE": "literal$private"}
	t.Setenv("OMITTED", "host-secret")
	parsed, err := Parse(context.Background(), b, Budget{})
	if err != nil {
		t.Fatal(err)
	}
	build := parsed.Builds["web"]
	if build.Context != "app" || build.Dockerfile != "app/Dockerfile" || build.Args["VALUE"] != "literal$private" || len(build.Args) != 1 || len(build.Files) != 2 {
		t.Fatal("build inputs were not isolated")
	}
	m, base, _, _ := fixtureManager(t)
	engine := &buildingEngine{fakeEngine: base}
	m.cfg.Engine = engine
	d, err := m.Prepare(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	if engine.builds != 1 || engine.prepareCount != 0 {
		t.Fatal("build not deduplicated or upstream image pulled")
	}
	for _, s := range object(d.Definition.Document["services"]) {
		if object(s)["build"] != nil || !imageIDPattern.MatchString(text(object(s)["image"])) {
			t.Fatal("runtime retained build inputs")
		}
	}
	entries, _ := os.ReadDir(m.cfg.StateDir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".build-") {
			t.Fatal("build workspace leaked")
		}
	}
}
func TestBuildRejectsEscapesRemoteContextsAndUnsupportedOptions(t *testing.T) {
	for _, build := range []string{"'https://example.com/repo.git'", "{context: ., ssh: [default]}", "{context: ., additional_contexts: {other: .}}", "{context: ../}", "{context: ., dockerfile: ../../Dockerfile}", "{context: '${CONTEXT}'}"} {
		b := packageFile(t, "services: {web: {build: "+build+"}}\n")
		os.WriteFile(filepath.Join(b.Directory, "Dockerfile"), []byte("FROM scratch\n"), 0600)
		if _, err := Parse(context.Background(), b, Budget{}); err == nil {
			t.Errorf("unsupported build accepted: %s", build)
		}
	}
	b := packageFile(t, "services: {web: {build: .}}\n")
	os.WriteFile(filepath.Join(b.Directory, "Dockerfile"), []byte("FROM scratch\n"), 0600)
	os.Symlink("/etc/passwd", filepath.Join(b.Directory, "leak"))
	if _, err := Parse(context.Background(), b, Budget{}); err == nil {
		t.Fatal("build followed package symlink")
	}
}

func TestRestartRemovesOnlyAbandonedBuildWorkspace(t *testing.T) {
	m, _, _, _ := fixtureManager(t)
	stage, err := os.MkdirTemp(m.cfg.StateDir, ".build-")
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(stage, "build.json"), []byte("private build data"), 0600)
	keep := filepath.Join(m.cfg.StateDir, "keep")
	os.WriteFile(keep, []byte("keep"), 0600)
	cfg := m.cfg
	m.Close()
	reopened, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, err := os.Stat(stage); !os.IsNotExist(err) {
		t.Fatal("abandoned build context retained")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatal("unrelated file removed")
	}
}
