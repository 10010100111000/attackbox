//go:build linux

package labnetwork

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

type localLinks struct{ host *netlink.Handle }

func newLocalLinks() (*localLinks, error) {
	h, err := netlink.NewHandle(unix.NETLINK_ROUTE)
	if err != nil {
		return nil, err
	}
	if err := h.SetSocketTimeout(5 * time.Second); err != nil {
		h.Close()
		return nil, err
	}
	return &localLinks{host: h}, nil
}
func (l *localLinks) Close() { l.host.Close() }

func missingLink(err error) bool {
	var missing netlink.LinkNotFoundError
	return errors.As(err, &missing)
}

func (l *localLinks) Occupied(ctx context.Context) ([]netip.Prefix, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Include policy-routing tables (e.g. VPN), not only the main table.
	routes, err := l.host.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{Table: unix.RT_TABLE_UNSPEC}, netlink.RT_FILTER_TABLE)
	if err != nil {
		return nil, err
	}
	var out []netip.Prefix
	for _, r := range routes {
		if r.Dst == nil {
			continue
		}
		p, err := netip.ParsePrefix(r.Dst.String())
		if err != nil {
			return nil, err
		}
		if p.Bits() > 0 {
			out = append(out, p)
		}
	}
	return out, nil
}

func (l *localLinks) Ensure(ctx context.Context, n Network, a Attachment, ns *PinnedNamespace) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	bridge, err := l.host.LinkByName(n.Bridge)
	if err != nil {
		return err
	}
	if bridge.Type() != "bridge" {
		return fmt.Errorf("lab interface is not a bridge")
	}
	// NewHandleAt pins its socket to this descriptor's namespace. No nsenter,
	// shell, later PID lookup, or change to the worker goroutine's namespace.
	guest, err := netlink.NewHandleAt(netns.NsHandle(ns.file.Fd()), unix.NETLINK_ROUTE)
	if err != nil {
		return err
	}
	defer guest.Close()
	if err := guest.SetSocketTimeout(5 * time.Second); err != nil {
		return err
	}
	host, err := l.host.LinkByName(a.HostName)
	if err != nil && !missingLink(err) {
		return err
	}
	if missingLink(err) {
		for _, name := range []string{a.GuestName, a.PeerName} {
			if _, err := guest.LinkByName(name); err == nil {
				return fmt.Errorf("sandbox interface name is already in use: %s", name)
			} else if !missingLink(err) {
				return err
			}
		}
		attrs := netlink.NewLinkAttrs()
		attrs.Name = a.HostName
		attrs.Alias = a.Alias
		attrs.MTU = bridge.Attrs().MTU
		v := &netlink.Veth{LinkAttrs: attrs, PeerName: a.PeerName, PeerNamespace: netlink.NsFd(ns.file.Fd()), PeerMTU: uint32(bridge.Attrs().MTU)}
		host, err = createOwnedVeth(l.host, guest, v, a)
		if err != nil {
			return err
		}
	}
	if host.Type() != "veth" || host.Attrs().Alias != a.Alias {
		return fmt.Errorf("host interface ownership mismatch")
	}
	if host.Attrs().MasterIndex != 0 && host.Attrs().MasterIndex != bridge.Attrs().Index {
		return fmt.Errorf("host interface is attached to another bridge")
	}
	peer, err := guest.LinkByName(a.GuestName)
	if missingLink(err) {
		peer, err = guest.LinkByName(a.PeerName)
	}
	if err != nil {
		return err
	}
	if peer.Type() != "veth" || peer.Attrs().ParentIndex != host.Attrs().Index || host.Attrs().ParentIndex != peer.Attrs().Index {
		return fmt.Errorf("veth peer identity mismatch")
	}
	if peer.Attrs().Alias != a.Alias && !(peer.Attrs().Name == a.PeerName && peer.Attrs().Alias == "") {
		return fmt.Errorf("sandbox interface ownership mismatch")
	}
	if err := guest.LinkSetAlias(peer, a.Alias); err != nil {
		return err
	}
	if peer.Attrs().Name != a.GuestName {
		if err := guest.LinkSetName(peer, a.GuestName); err != nil {
			return err
		}
	}
	if err := l.host.LinkSetMaster(host, bridge); err != nil {
		return err
	}
	// This is an experiment port, not an isolated port on br-attackbox.
	if err := l.host.LinkSetIsolated(host, false); err != nil {
		return err
	}
	addr, err := netlink.ParseAddr(a.Address.String())
	if err != nil {
		return err
	}
	if err := guest.AddrReplace(peer, addr); err != nil {
		return err
	}
	if err := guest.LinkSetUp(peer); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return l.host.LinkSetUp(host)
}

func (l *localLinks) Remove(ctx context.Context, a Attachment) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	host, err := l.host.LinkByName(a.HostName)
	if missingLink(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if host.Type() != "veth" || host.Attrs().Alias != a.Alias {
		return fmt.Errorf("refusing to delete unowned interface")
	}
	return l.host.LinkDel(host)
}

func (l *localLinks) Check(ctx context.Context, n Network, a Attachment, ns *PinnedNamespace) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	host, err := l.host.LinkByName(a.HostName)
	if missingLink(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if host.Type() != "veth" || host.Attrs().Alias != a.Alias {
		return false, fmt.Errorf("interface ownership mismatch")
	}
	bridge, err := l.host.LinkByName(n.Bridge)
	if err != nil {
		return false, err
	}
	if bridge.Type() != "bridge" || host.Attrs().MasterIndex != bridge.Attrs().Index {
		return false, fmt.Errorf("lab bridge attachment changed")
	}
	guest, err := netlink.NewHandleAt(netns.NsHandle(ns.file.Fd()), unix.NETLINK_ROUTE)
	if err != nil {
		return false, err
	}
	defer guest.Close()
	if err := guest.SetSocketTimeout(5 * time.Second); err != nil {
		return false, err
	}
	peer, err := guest.LinkByName(a.GuestName)
	if err != nil {
		return false, err
	}
	if peer.Type() != "veth" || peer.Attrs().Alias != a.Alias || peer.Attrs().ParentIndex != host.Attrs().Index || host.Attrs().ParentIndex != peer.Attrs().Index {
		return false, fmt.Errorf("sandbox peer changed")
	}
	addresses, err := guest.AddrList(peer, netlink.FAMILY_V4)
	if err != nil {
		return false, err
	}
	found := false
	for _, addr := range addresses {
		if addr.IPNet != nil && addr.IPNet.String() == a.Address.String() {
			found = true
		}
	}
	if !found {
		return false, fmt.Errorf("sandbox lab address changed")
	}
	return true, nil
}

func (l *localLinks) CheckBridgeEmpty(ctx context.Context, n Network) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	bridge, err := l.host.LinkByName(n.Bridge)
	if missingLink(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if bridge.Type() != "bridge" {
		return fmt.Errorf("lab bridge ownership conflict")
	}
	links, err := l.host.LinkList()
	if err != nil {
		return err
	}
	for _, v := range links {
		if v.Attrs().MasterIndex == bridge.Attrs().Index {
			return fmt.Errorf("lab bridge still has a port: %s", v.Attrs().Name)
		}
	}
	return nil
}

func (l *localLinks) Orphans(ctx context.Context, node string, known map[string]bool) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	links, err := l.host.LinkList()
	if err != nil {
		return nil, err
	}
	var out []string
	for _, v := range links {
		if strings.HasPrefix(v.Attrs().Alias, "attackbox:lab:"+node+":") && !known[v.Attrs().Alias] {
			out = append(out, v.Attrs().Name)
		}
	}
	return out, nil
}
