//go:build linux

package labnetwork

import (
	"errors"
	"fmt"

	"github.com/vishvananda/netlink"
)

type linkLookup interface {
	LinkByName(string) (netlink.Link, error)
}
type vethCreator interface {
	linkLookup
	LinkAdd(netlink.Link) error
	LinkSetAlias(netlink.Link, string) error
	LinkDel(netlink.Link) error
}

// Only a successful exclusive creation followed by reciprocal peer checks may
// establish ownership without an existing alias. Never use this on a reused
// interface: an empty alias is not permission to adopt an arbitrary link.
func createOwnedVeth(host vethCreator, guest linkLookup, v *netlink.Veth, a Attachment) (netlink.Link, error) {
	if err := host.LinkAdd(v); err != nil {
		return nil, err
	}
	h, err := host.LinkByName(a.HostName)
	if err != nil {
		return nil, err
	}
	p, err := guest.LinkByName(a.PeerName)
	if err != nil {
		return nil, err
	}
	if h.Type() != "veth" || p.Type() != "veth" || h.Attrs().Index <= 0 || p.Attrs().Index <= 0 || h.Attrs().ParentIndex != p.Attrs().Index || p.Attrs().ParentIndex != h.Attrs().Index || h.Attrs().MasterIndex != 0 || v.Attrs().Index > 0 && v.Attrs().Index != h.Attrs().Index || h.Attrs().Alias != "" && h.Attrs().Alias != a.Alias || p.Attrs().Alias != "" && p.Attrs().Alias != a.Alias {
		return nil, fmt.Errorf("new veth identity could not be verified")
	}
	rollback := func(cause error) (netlink.Link, error) { return nil, errors.Join(cause, host.LinkDel(h)) }
	// Do not rely solely on the creation-time alias. SETLINK establishes and
	// verifies the ownership marker explicitly before the interface is used.
	if err := host.LinkSetAlias(h, a.Alias); err != nil {
		return rollback(err)
	}
	confirmed, err := host.LinkByName(a.HostName)
	if err != nil {
		return rollback(err)
	}
	if confirmed.Type() != "veth" || confirmed.Attrs().Index != h.Attrs().Index || confirmed.Attrs().ParentIndex != p.Attrs().Index || confirmed.Attrs().Alias != a.Alias {
		return rollback(fmt.Errorf("new veth alias was not retained"))
	}
	return confirmed, nil
}
