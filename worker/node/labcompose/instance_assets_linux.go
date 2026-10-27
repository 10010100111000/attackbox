//go:build linux

package labcompose

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func writableSources(d Deployment) []string {
	seen := map[string]bool{}
	for _, s := range object(d.Definition.Document["services"]) {
		for _, raw := range slice(object(s)["volumes"]) {
			mount := object(raw)
			if text(mount["type"]) == "bind" && mount["read_only"] != true {
				seen[text(mount["source"])] = true
			}
		}
	}
	out := []string{}
	for source := range seen {
		out = append(out, source)
	}
	return out
}

// Read-only consumers of the same/overlapping source must see initialization
// done by writable services. Preserve Compose sharing within one instance.
func privateAssetSource(source string, writable []string) bool {
	for _, root := range writable {
		if underPath(source, root) || underPath(root, source) {
			return true
		}
	}
	return false
}
func privateAssetSources(d Deployment, writable []string) []string {
	sources := []string{}
	for _, s := range object(d.Definition.Document["services"]) {
		for _, raw := range slice(object(s)["volumes"]) {
			mount := object(raw)
			source := text(mount["source"])
			if text(mount["type"]) == "bind" && privateAssetSource(source, writable) {
				sources = append(sources, source)
			}
		}
	}
	return sortedUnique(sources)
}

// Called once before first Up, never during render/inspect/down. Repeated starts
// must not reset a learner's files. Mutable data is never hashed as deployment data.
func (m *Manager) prepareInstanceAssets(r Record, d Deployment) error {
	writable := writableSources(d)
	sources := privateAssetSources(d, writable)
	if len(sources) == 0 {
		return nil
	}
	root := filepath.Join(m.cfg.StateDir, "instances", r.Request.ID)
	marker := filepath.Join(root, "assets.json")
	if _, err := os.Lstat(marker); err == nil {
		var saved struct {
			Deployment string `json:"deployment"`
		}
		if err := decodeFile(marker, &saved); err != nil || saved.Deployment != d.ID {
			return fmt.Errorf("instance asset identity mismatch")
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	destination := filepath.Join(root, "assets")
	if _, err := os.Lstat(destination); err == nil {
		return fmt.Errorf("instance asset initialization incomplete")
	} else if !os.IsNotExist(err) {
		return err
	}
	tmp, err := os.MkdirTemp(root, ".assets-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	for _, dir := range d.Definition.Directories {
		if err := os.MkdirAll(filepath.Join(tmp, "files", filepath.FromSlash(dir)), 0700); err != nil {
			return err
		}
	}
	for rel, hash := range d.Definition.Files {
		needed := false
		for _, source := range sources {
			if underPath("files/"+rel, source) {
				needed = true
				break
			}
		}
		if !needed {
			continue
		}
		src, err := safePath(filepath.Join(m.cfg.StateDir, "deployments", d.ID, "files"), rel)
		if err != nil {
			return err
		}
		data, err := readBounded(src, 4<<20)
		if err != nil {
			return err
		}
		if digest(data) != hash {
			return fmt.Errorf("prepared file changed")
		}
		dst := filepath.Join(tmp, "files", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
			return err
		}
		if err := writeAtomic(dst, data); err != nil {
			return err
		}
		if err := os.Chmod(dst, 0666); err != nil {
			return err
		}
	}
	if err := filepath.WalkDir(tmp, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.Chmod(path, 0755)
		}
		return nil
	}); err != nil {
		return err
	}
	// Private instance parent protects the tree on the host. Writable mount roots
	// allow images with different unprivileged UIDs to edit their own copy.
	for _, source := range writable {
		path := filepath.Join(tmp, filepath.FromSlash(source))
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			if err := filepath.WalkDir(path, func(p string, e os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if e.IsDir() {
					return os.Chmod(p, 0777)
				}
				return nil
			}); err != nil {
				return err
			}
		}
	}
	if err := os.Rename(tmp, destination); err != nil {
		return err
	}
	if err := writeJSON(marker, map[string]string{"deployment": d.ID}); err != nil {
		return err
	}
	return syncDir(root)
}
func (m *Manager) removeInstanceAssets(r Record) error {
	root := filepath.Join(m.cfg.StateDir, "instances", r.Request.ID)
	// RemoveAll does not follow symlinks left inside a writable mount. The parent
	// is owned by this instance's verified ledger and is never itself mounted.
	if err := os.RemoveAll(filepath.Join(root, "assets")); err != nil {
		return err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".assets-") {
			if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
				return err
			}
		}
	}
	if err := os.Remove(filepath.Join(root, "assets.json")); err != nil && !os.IsNotExist(err) {
		return err
	}
	return syncDir(root)
}

func exposeAssetDirectories(root string, dirs []string) error {
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(dir)), 0700); err != nil {
			return err
		}
	}
	for _, dir := range dirs {
		path := filepath.Join(root, filepath.FromSlash(dir))
		if err := filepath.WalkDir(path, func(p string, e os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if e.IsDir() {
				return os.Chmod(p, 0755)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}
