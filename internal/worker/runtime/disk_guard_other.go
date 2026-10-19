//go:build !linux

package runtime

func getFileSystemFreeMB(path string) (int64, error) {
	// Stub for local non-Linux builds / linting
	return 50000, nil
}
