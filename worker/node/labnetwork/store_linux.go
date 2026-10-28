//go:build linux

package labnetwork

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)


type state struct {
	Version     int                   `json:"version"`
	NodeID      string                `json:"node_id"`
	Networks    map[string]Network    `json:"networks"`
	Attachments map[string]Attachment `json:"attachments"`
}
type store struct {
	dir  string
	lock *os.File
}

func openStore(dir, node string) (*store, state, error) {
	var empty state
	if !filepath.IsAbs(dir) {
		return nil, empty, fmt.Errorf("state directory must be absolute")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, empty, err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, empty, err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return nil, empty, fmt.Errorf("state directory must be private (0700) and not a symlink")
	}
	fd, err := unix.Open(filepath.Join(dir, "manager.lock"), unix.O_RDWR|unix.O_CREAT|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, empty, err
	}
	f := os.NewFile(uintptr(fd), "manager.lock")
	if err := unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		return nil, empty, fmt.Errorf("network state already in use: %w", err)
	}
	s := &store{dir: dir, lock: f}
	fd, err = unix.Open(filepath.Join(dir, "state.json"), unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if os.IsNotExist(err) {
		return s, state{Version: 1, NodeID: node, Networks: map[string]Network{}, Attachments: map[string]Attachment{}}, nil
	}
	if err != nil {
		s.close()
		return nil, empty, err
	}
	input := os.NewFile(uintptr(fd), "state.json")
	defer input.Close()
	info, err = input.Stat()
	if err != nil {
		s.close()
		return nil, empty, err
	}
	if !info.Mode().IsRegular() || info.Size() > 8<<20 {
		s.close()
		return nil, empty, fmt.Errorf("network state is not a bounded regular file")
	}
	b, err := io.ReadAll(io.LimitReader(input, 8<<20))
	if err != nil {
		s.close()
		return nil, empty, err
	}
	var loaded state
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&loaded); err != nil {
		s.close()
		return nil, empty, fmt.Errorf("invalid network state: %w", err)
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		s.close()
		return nil, empty, fmt.Errorf("invalid trailing network state")
	}
	if err := validateState(loaded, node); err != nil {
		s.close()
		return nil, empty, err
	}
	for id, n := range loaded.Networks {
		if n.Cleaned {
			delete(loaded.Networks, id)
		}
	}
	for id, a := range loaded.Attachments {
		if a.Cleaned {
			delete(loaded.Attachments, id)
		}
	}
	return s, loaded, nil
}


func validateState(s state, node string) error {
	if s.Version != 1 || s.NodeID != node || s.Networks == nil || s.Attachments == nil {
		return fmt.Errorf("network state version or node mismatch")
	}
	var reserved []Network
	for id, n := range s.Networks {
		if validateSpec(n.Spec) != nil || id != n.ID || id != networkID(n.Spec) {
			return fmt.Errorf("invalid network state identity")
		}
		p, err := Plan(n.Subnet, n.Spec, nil)
		if err != nil || p.Name != n.Name || p.Bridge != n.Bridge || p.Subnet != n.Subnet || p.Gateway != n.Gateway || p.SandboxIP != n.SandboxIP || n.Cleaned && n.Desired {
			return fmt.Errorf("invalid network state configuration")
		}
		if !n.Cleaned {
			for _, old := range reserved {
				if old.Subnet.Overlaps(n.Subnet) || old.Name == n.Name || old.Bridge == n.Bridge {
					return fmt.Errorf("conflicting persisted networks")
				}
			}
			reserved = append(reserved, n)
		}
	}
	usedNetwork, usedHost, usedSlot := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for id, a := range s.Attachments {
		n, ok := s.Networks[a.NetworkID]
		if !ok {
			return fmt.Errorf("attachment without network")
		}
		var slot int
		if _, err := fmt.Sscanf(a.GuestName, "lab%d", &slot); err != nil || slot < 0 || slot > 255 {
			return fmt.Errorf("invalid attachment slot")
		}
		expected := makeAttachment(node, n, a.Sandbox, slot)
		if id != a.ID || a.ID != expected.ID || a.Alias != expected.Alias || a.HostName != expected.HostName || a.PeerName != expected.PeerName || a.GuestName != expected.GuestName || a.Address != expected.Address || a.Sandbox.OwnerID != n.Spec.OwnerID || !identityPattern.MatchString(a.Sandbox.ID) || a.Sandbox.PID <= 0 || a.Sandbox.Namespace.Inode == 0 || a.Sandbox.Namespace.Device == 0 || a.Cleaned && a.Desired {
			return fmt.Errorf("invalid attachment state")
		}
		if !a.Cleaned {
			slotKey := fmt.Sprintf("%s/%d/%d/%s", a.Sandbox.ID, a.Sandbox.Namespace.Device, a.Sandbox.Namespace.Inode, a.GuestName)
			if n.Cleaned || usedNetwork[a.NetworkID] || usedHost[a.HostName] || usedSlot[slotKey] {
				return fmt.Errorf("conflicting persisted attachments")
			}
			usedNetwork[a.NetworkID] = true
			usedHost[a.HostName] = true
			usedSlot[slotKey] = true
		}
	}
	return nil
}

func (s *store) save(value state) error {
	toPersist := state{
		Version:     value.Version,
		NodeID:      value.NodeID,
		Networks:    make(map[string]Network),
		Attachments: make(map[string]Attachment),
	}
	for k, v := range value.Networks {
		if !v.Cleaned {
			toPersist.Networks[k] = v
		}
	}
	for k, v := range value.Attachments {
		if !v.Cleaned {
			toPersist.Attachments[k] = v
		}
	}
	b, err := json.Marshal(toPersist)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(s.dir, ".state-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
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
	if err := os.Rename(tmp, filepath.Join(s.dir, "state.json")); err != nil {
		return err
	}
	d, err := os.Open(s.dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
func (s *store) close() error { return s.lock.Close() }
