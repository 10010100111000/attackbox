//go:build linux

package labnetwork

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"syscall"
)

var ErrNamespaceChanged = errors.New("sandbox namespace identity changed")

// PinnedNamespace owns an open nsfs descriptor. The backend uses this descriptor
// for both peer migration and a namespace-bound netlink socket, never PID again.
type PinnedNamespace struct {
	file     *os.File
	Identity NamespaceIdentity
}

func (p *PinnedNamespace) Close() error { return p.file.Close() }
func (p *PinnedNamespace) FD() uintptr  { return p.file.Fd() }

func fileIdentity(f *os.File) (NamespaceIdentity, error) {
	info, err := f.Stat()
	if err != nil {
		return NamespaceIdentity{}, err
	}
	s, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return NamespaceIdentity{}, fmt.Errorf("namespace stat unavailable")
	}
	return NamespaceIdentity{Device: uint64(s.Dev), Inode: s.Ino}, nil
}

// CaptureNamespace is called at sandbox creation under its lifecycle lock.
func CaptureNamespace(pid int) (NamespaceIdentity, error) {
	if pid <= 0 {
		return NamespaceIdentity{}, fmt.Errorf("invalid sandbox PID")
	}
	f, err := os.Open("/proc/" + strconv.Itoa(pid) + "/ns/net")
	if err != nil {
		return NamespaceIdentity{}, err
	}
	defer f.Close()
	return fileIdentity(f)
}

func PinNamespace(s Sandbox) (*PinnedNamespace, error) {
	if !identityPattern.MatchString(s.ID) || !identityPattern.MatchString(s.OwnerID) {
		return nil, fmt.Errorf("invalid sandbox identity")
	}
	return PinNetworkNamespace(s.PID, s.Namespace)
}

// PinNetworkNamespace is shared by primary eth0 and lab adapters. Validate the
// captured device/inode once, then use only FD() for all netlink operations.
func PinNetworkNamespace(pid int, expected NamespaceIdentity) (*PinnedNamespace, error) {
	if pid <= 0 || expected.Inode == 0 || expected.Device == 0 {
		return nil, fmt.Errorf("invalid sandbox namespace identity")
	}
	f, err := os.Open("/proc/" + strconv.Itoa(pid) + "/ns/net")
	if err != nil {
		return nil, fmt.Errorf("open sandbox namespace: %w", err)
	}
	fail := func(err error) (*PinnedNamespace, error) { f.Close(); return nil, err }
	id, err := fileIdentity(f)
	if err != nil {
		return fail(err)
	}
	if id != expected {
		return fail(ErrNamespaceChanged)
	}
	for _, path := range []string{"/proc/self/ns/net", "/proc/1/ns/net"} {
		host, err := os.Open(path)
		if err != nil {
			return fail(err)
		}
		hostID, err := fileIdentity(host)
		host.Close()
		if err != nil {
			return fail(err)
		}
		if id == hostID {
			return fail(fmt.Errorf("refusing host/worker network namespace"))
		}
	}
	return &PinnedNamespace{file: f, Identity: id}, nil
}
