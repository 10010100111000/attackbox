package labcompose

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type BuildSpec struct {
	Context    string            `json:"context"`
	Dockerfile string            `json:"dockerfile"` // Relative to package root, not context.
	Target     string            `json:"target,omitempty"`
	Args       map[string]string `json:"args,omitempty"`
	Files      map[string]string `json:"files"`
}

// Optional capability so image-only engines remain small and compatible.
type ImageBuilder interface {
	BuildImage(context.Context, string, string, BuildSpec, bool) (string, error)
}

func parseBuild(root string, value any) (BuildSpec, error) {
	var spec BuildSpec
	b := object(value)
	if b == nil {
		return spec, fmt.Errorf("invalid local build configuration")
	}
	if err := allowed(b, "context dockerfile target args"); err != nil {
		return spec, err
	}
	contextPath, err := safePath(root, text(b["context"]))
	if err != nil {
		return spec, err
	}
	info, err := os.Stat(contextPath)
	if err != nil || !info.IsDir() {
		return spec, fmt.Errorf("build context must be a package directory")
	}
	spec.Files = map[string]string{}
	spec.Context, _, err = snapshotPath(root, contextPath, spec.Files)
	if err != nil {
		return spec, err
	}
	dockerfile := text(b["dockerfile"])
	if dockerfile == "" {
		dockerfile = "Dockerfile"
	}
	if !filepath.IsAbs(dockerfile) {
		dockerfile = filepath.Join(contextPath, dockerfile)
	}
	var dir bool
	spec.Dockerfile, dir, err = snapshotPath(root, dockerfile, spec.Files)
	if err != nil {
		return spec, err
	}
	if dir {
		return spec, fmt.Errorf("Dockerfile must be a regular package file")
	}
	spec.Target = text(b["target"])
	if spec.Target != "" && !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`).MatchString(spec.Target) {
		return spec, fmt.Errorf("invalid build target")
	}
	spec.Args = map[string]string{}
	if len(object(b["args"])) > 64 {
		return spec, fmt.Errorf("too many build arguments")
	}
	for key, value := range object(b["args"]) {
		if !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(key) {
			return spec, fmt.Errorf("invalid build argument")
		}
		if value == nil {
			continue
		}
		v, ok := value.(string)
		if !ok || len(v) > 8192 {
			return spec, fmt.Errorf("invalid build argument")
		}
		spec.Args[key] = v
	}
	return spec, nil
}

func buildKey(spec BuildSpec) string { b, _ := json.Marshal(spec); return digest(b) }

func (c *CLI) BuildImage(ctx context.Context, root, stage string, spec BuildSpec, rebuild bool) (string, error) {
	key := buildKey(spec)
	tag := "attackbox-build:" + key
	if !rebuild {
		if id, err := c.image(ctx, tag); err == nil {
			return id, nil
		}
	}
	if _, err := c.run(ctx, "buildx", "version"); err != nil {
		return "", fmt.Errorf("Docker Buildx plugin required for local builds")
	}
	// Copy the exact validated snapshot. Never give Docker the original package or
	// any arbitrary directory, even when the context is the package root.
	for rel, hash := range spec.Files {
		path, err := safePath(root, rel)
		if err != nil {
			return "", err
		}
		data, err := readBounded(path, 4<<20)
		if err != nil {
			return "", err
		}
		if digest(data) != hash {
			return "", fmt.Errorf("package changed during build")
		}
		dst := filepath.Join(stage, "source", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
			return "", err
		}
		if err := os.WriteFile(dst, data, 0600); err != nil {
			return "", err
		}
	}
	if err := filepath.WalkDir(filepath.Join(stage, "source"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.Chmod(path, 0755)
		}
		return os.Chmod(path, 0644)
	}); err != nil {
		return "", err
	}
	contextPath := filepath.Join(stage, "source", filepath.FromSlash(spec.Context))
	if err := os.MkdirAll(contextPath, 0700); err != nil {
		return "", err
	}
	build := map[string]any{"context": contextPath, "dockerfile": filepath.Join(stage, "source", filepath.FromSlash(spec.Dockerfile)), "args": spec.Args, "labels": map[string]string{labelPrefix + "build": key}}
	if spec.Target != "" {
		build["target"] = spec.Target
	}
	doc := map[string]any{"services": map[string]any{"image": map[string]any{"image": tag, "build": build}}}
	data, err := json.Marshal(escapeDollars(cloneDocument(doc)))
	if err != nil {
		return "", err
	}
	file := filepath.Join(stage, "build.json")
	if err := os.WriteFile(file, data, 0600); err != nil {
		return "", err
	}
	args := []string{"compose", "--ansi", "never", "--env-file", "/dev/null", "--project-directory", stage, "--project-name", "ab_build_" + key[:24], "--file", file, "build", "--builder", "default", "--quiet"}
	if rebuild {
		args = append(args, "--pull", "--no-cache")
	}
	if _, err := c.run(ctx, args...); err != nil {
		return "", fmt.Errorf("local image build failed")
	}
	return c.image(ctx, tag)
}

func rawBuildAllowed(value any) error {
	if path, ok := value.(string); ok {
		if strings.ContainsAny(path, ":$") || filepath.IsAbs(path) {
			return fmt.Errorf("only package-local build contexts supported")
		}
		return nil
	}
	b := object(value)
	if b == nil {
		return fmt.Errorf("invalid build configuration")
	}
	if err := allowed(b, "context dockerfile args target"); err != nil {
		return err
	}
	for _, key := range []string{"context", "dockerfile"} {
		if path, ok := b[key].(string); ok && (strings.ContainsAny(path, ":$") || filepath.IsAbs(path)) {
			return fmt.Errorf("only package-local build paths supported")
		}
	}
	return nil
}
