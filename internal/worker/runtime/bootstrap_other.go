//go:build !linux

package runtime

import "errors"

// RunContainerInit is a stub for non-Linux platforms.
func RunContainerInit(sandboxID, rootfsPath, tmpSize, runSize string) error {
	return errors.New("container init is only supported on Linux")
}
