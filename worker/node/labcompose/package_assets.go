package labcompose

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Package snapshots use the same bounds as uploaded/Git packages. Every file is
// checked again when copied; neither builders nor running targets read sources.
func snapshotPath(root, path string, files map[string]string) (string, bool, error) {
	path, err := safePath(root, path)
	if err != nil {
		return "", false, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", false, err
	}
	total := int64(0)
	for key := range files {
		p, err := safePath(root, key)
		if err != nil {
			return "", false, err
		}
		i, err := os.Stat(p)
		if err != nil {
			return "", false, err
		}
		total += i.Size()
	}
	rel, _ := filepath.Rel(root, path)
	rel = filepath.ToSlash(rel)
	add := func(p string, entry fs.DirEntry) error {
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("package links and special files unsupported")
		}
		p, err := safePath(root, p)
		if err != nil {
			return err
		}
		data, err := readBounded(p, 4<<20)
		if err != nil {
			return err
		}
		key, _ := filepath.Rel(root, p)
		key = filepath.ToSlash(key)
		if len(key) > 240 {
			return fmt.Errorf("package path too long")
		}
		if _, exists := files[key]; !exists {
			total += int64(len(data))
		}
		if total > 32<<20 {
			return fmt.Errorf("package files exceed 32 MiB")
		}
		files[key] = digest(data)
		if len(files) > 512 {
			return fmt.Errorf("too many package files")
		}
		return nil
	}
	if info.IsDir() {
		err = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("package symlink not supported")
			}
			return add(p, d)
		})
	} else {
		err = add(path, fs.FileInfoToDirEntry(info))
	}
	return rel, info.IsDir(), err
}
func underPath(file, root string) bool {
	file = filepath.ToSlash(filepath.Clean(file))
	root = filepath.ToSlash(filepath.Clean(root))
	return root == "." || file == root || strings.HasPrefix(file, root+"/")
}
func sortedUnique(dirs []string) []string {
	seen := map[string]bool{}
	for _, dir := range dirs {
		seen[dir] = true
	}
	out := make([]string, 0, len(seen))
	for dir := range seen {
		out = append(out, dir)
	}
	sort.Strings(out)
	return out
}
