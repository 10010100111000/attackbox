//go:build linux

package runtime

import (
	"syscall"
)

func getFileSystemFreeMB(path string) (int64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	// Available blocks for non-root users * block size
	freeBytes := uint64(stat.Bavail) * uint64(stat.Bsize)
	return int64(freeBytes / (1024 * 1024)), nil
}
