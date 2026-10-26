//go:build linux

package labcompose

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

func privateDir(path string) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("runtime directory must be private and not a symlink")
	}
	return nil
}
func writeAtomic(path string, b []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".write-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	return syncDir(filepath.Dir(path))
}
func syncDir(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
func writeJSON(path string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return writeAtomic(path, b)
}
func decodeFile(path string, v any) error {
	b, err := readBounded(path, 8<<20)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func openLock(root string) (*os.File, error) {
	if !filepath.IsAbs(root) {
		return nil, fmt.Errorf("runtime directory must be absolute")
	}
	if err := privateDir(root); err != nil {
		return nil, err
	}
	fd, err := unix.Open(filepath.Join(root, "runtime.lock"), unix.O_RDWR|unix.O_CREAT|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), "runtime.lock")
	if err := unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("Compose runtime already open")
	}
	return f, nil
}

func (m *Manager) Prepare(ctx context.Context, b Bundle) (Deployment, error) {
	if err := m.enter(); err != nil {
		return Deployment{}, err
	}
	defer m.leave()
	work, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	stop := context.AfterFunc(m.lifetime, cancel)
	defer stop()
	ctx = work
	select {
	case m.prepare <- struct{}{}:
		defer func() { <-m.prepare }()
	case <-ctx.Done():
		return Deployment{}, ctx.Err()
	}
	def, err := Parse(ctx, b, m.cfg.Budget)
	if err != nil {
		return Deployment{}, err
	}
	prepared := map[string]string{}
	for _, name := range keys(object(def.Document["services"])) {
		s := object(object(def.Document["services"])[name])
		ref := text(s["image"])
		build, local := def.Builds[name]
		if local {
			ref = "build:" + buildKey(build)
		}
		id, ok := prepared[ref]
		var err error
		if !ok {
			if local {
				builder, supported := m.cfg.Engine.(ImageBuilder)
				if !supported {
					return Deployment{}, fmt.Errorf("engine does not support local builds")
				}
				stage, stageErr := os.MkdirTemp(m.cfg.StateDir, ".build-")
				if stageErr != nil {
					return Deployment{}, stageErr
				}
				id, err = builder.BuildImage(ctx, b.Directory, stage, build)
				os.RemoveAll(stage)
			} else {
				id, err = m.cfg.Engine.PrepareImage(ctx, ref)
			}
			prepared[ref] = id
		}
		if err != nil {
			return Deployment{}, err
		}
		if !imageIDPattern.MatchString(id) {
			return Deployment{}, fmt.Errorf("prepared image ID invalid")
		}
		s["image"] = id
	}
	encoded, err := json.Marshal(def)
	if err != nil {
		return Deployment{}, err
	}
	dep := Deployment{ID: "dep_" + digest(encoded), Definition: def}
	dir := filepath.Join(m.cfg.StateDir, "deployments", dep.ID)
	if _, err := os.Lstat(dir); err == nil {
		return m.loadDeployment(dep.ID)
	} else if !os.IsNotExist(err) {
		return Deployment{}, err
	}
	tmp, err := os.MkdirTemp(filepath.Join(m.cfg.StateDir, "deployments"), ".prepare-")
	if err != nil {
		return Deployment{}, err
	}
	defer os.RemoveAll(tmp)
	var total int
	for rel, hash := range def.Files {
		src, err := safePath(b.Directory, rel)
		if err != nil {
			return Deployment{}, err
		}
		data, err := readBounded(src, 4<<20)
		if err != nil {
			return Deployment{}, err
		}
		if digest(data) != hash {
			return Deployment{}, fmt.Errorf("package changed during preparation")
		}
		total += len(data)
		if total > 32<<20 {
			return Deployment{}, fmt.Errorf("package files exceed 32 MiB")
		}
		dst := filepath.Join(tmp, "files", filepath.FromSlash(rel))
		if err := privateDir(filepath.Dir(dst)); err != nil {
			return Deployment{}, err
		}
		if err := writeAtomic(dst, data); err != nil {
			return Deployment{}, err
		}
		// Parent directories remain private; a bind-mounted file must still be
		// readable by a non-root container UID. The mount itself is read-only.
		if err := os.Chmod(dst, 0444); err != nil {
			return Deployment{}, err
		}
	}
	if err := exposeAssetDirectories(filepath.Join(tmp, "files"), def.Directories); err != nil {
		return Deployment{}, err
	}
	if err := writeJSON(filepath.Join(tmp, "deployment.json"), dep); err != nil {
		return Deployment{}, err
	}
	if err := os.Rename(tmp, dir); err != nil {
		return Deployment{}, err
	}
	if err := syncDir(filepath.Dir(dir)); err != nil {
		return Deployment{}, err
	}
	return dep, nil
}

func (m *Manager) loadDeployment(id string) (Deployment, error) {
	var d Deployment
	if !deploymentPattern.MatchString(id) {
		return d, fmt.Errorf("invalid deployment ID")
	}
	root := filepath.Join(m.cfg.StateDir, "deployments", id)
	path, err := safePath(root, "deployment.json")
	if err != nil {
		return d, err
	}
	if err := decodeFile(path, &d); err != nil {
		return d, fmt.Errorf("invalid deployment metadata")
	}
	b, err := json.Marshal(d.Definition)
	if err != nil || d.ID != id || "dep_"+digest(b) != id {
		return d, fmt.Errorf("deployment digest mismatch")
	}
	for rel, hash := range d.Definition.Files {
		if strings.HasPrefix(rel, "/") {
			return d, fmt.Errorf("invalid asset reference")
		}
		p, err := safePath(filepath.Join(root, "files"), rel)
		if err != nil {
			return d, err
		}
		b, err := readBounded(p, 4<<20)
		if err != nil || digest(b) != hash {
			return d, fmt.Errorf("prepared file changed")
		}
	}
	for _, dir := range d.Definition.Directories {
		path, err := safePath(filepath.Join(root, "files"), dir)
		if err != nil {
			return d, err
		}
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			return d, fmt.Errorf("prepared directory changed")
		}
	}
	return d, nil
}

func (m *Manager) render(r Record, d Deployment) (string, error) {
	doc := cloneDocument(d.Definition.Document)
	doc["name"] = r.Project
	labels := ownedLabels(m.cfg.NodeID, r)
	writable := writableSources(d)
	for _, v := range object(doc["services"]) {
		s := object(v)
		s["labels"] = labels
		for _, raw := range slice(s["volumes"]) {
			mount := object(raw)
			if text(mount["type"]) == "bind" {
				if !privateAssetSource(text(mount["source"]), writable) {
					mount["source"] = filepath.Join(m.cfg.StateDir, "deployments", d.ID, text(mount["source"]))
				} else {
					mount["source"] = filepath.Join(m.cfg.StateDir, "instances", r.Request.ID, "assets", text(mount["source"]))
				}
			}
		}
	}
	for key := range object(doc["volumes"]) {
		object(doc["volumes"])[key] = map[string]any{"name": r.Project + "_" + key, "labels": labels}
	}
	for _, key := range d.Definition.Networks {
		n, ok := r.Networks[key]
		name := "unallocated_" + digest([]byte(r.Project + "/" + key))[:24]
		if ok {
			name = n.Name
		}
		object(doc["networks"])[key] = map[string]any{"external": true, "name": name}
	}
	path := filepath.Join(m.cfg.StateDir, "instances", r.Request.ID, "compose.json")
	b, err := json.Marshal(escapeDollars(doc))
	if err != nil {
		return "", err
	}
	if err := writeAtomic(path, b); err != nil {
		return "", err
	}
	return path, nil
}
func slice(v any) []any { a, _ := v.([]any); return a }
