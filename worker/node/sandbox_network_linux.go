//go:build linux

package node

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"attackbox/node/labnetwork"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

func sandboxNetworkHandle() (*netlink.Handle, error) {
	h, err := netlink.NewHandle(unix.NETLINK_ROUTE)
	if err != nil {
		return nil, err
	}
	if err := h.SetSocketTimeout(5 * time.Second); err != nil {
		h.Close()
		return nil, err
	}
	return h, nil
}

func missingSandboxLink(err error) bool {
	var missing netlink.LinkNotFoundError
	return errors.As(err, &missing)
}

func ensureSandboxBridge(cfg NetworkConfig, address string) error {
	h, err := sandboxNetworkHandle()
	if err != nil {
		return err
	}
	defer h.Close()
	bridge, err := h.LinkByName(cfg.BridgeName)
	if missingSandboxLink(err) {
		b := &netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Name: cfg.BridgeName}}
		if err := h.LinkAdd(b); err != nil {
			return fmt.Errorf("create sandbox bridge: %w", err)
		}
		addr, err := netlink.ParseAddr(address)
		if err == nil {
			err = h.AddrAdd(b, addr)
		}
		if err != nil {
			return errors.Join(err, h.LinkDel(b))
		}
		bridge = b
	} else if err != nil {
		return err
	} else {
		if bridge.Type() != "bridge" {
			return fmt.Errorf("sandbox interface %s is not a bridge", cfg.BridgeName)
		}
		addresses, err := h.AddrList(bridge, netlink.FAMILY_V4)
		if err != nil {
			return err
		}
		found := false
		for _, addr := range addresses {
			if addr.IPNet != nil && addr.IPNet.String() == address {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("existing sandbox bridge %s does not have configured address %s", cfg.BridgeName, address)
		}
	}
	return h.LinkSetUp(bridge)
}

func detectDefaultWANInterface() (string, error) {
	h, err := sandboxNetworkHandle()
	if err != nil {
		return "", err
	}
	defer h.Close()
	routes, err := h.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		return "", err
	}
	var best *netlink.Route
	for i := range routes {
		r := &routes[i]
		if (r.Dst == nil || r.Dst.String() == "0.0.0.0/0") && r.LinkIndex > 0 && (best == nil || r.Priority < best.Priority) {
			best = r
		}
	}
	if best == nil {
		return "", fmt.Errorf("no IPv4 default route; configure network.egress_interface")
	}
	link, err := h.LinkByIndex(best.LinkIndex)
	if err != nil {
		return "", err
	}
	return link.Attrs().Name, nil
}

// SetupSandboxNetwork initializes primary eth0. Both network adapters pin and
// verify the creation-time namespace identity, create the peer directly in that
// namespace, and configure it through a namespace-bound netlink socket.
func SetupSandboxNetwork(ctx context.Context, cfg NetworkConfig, slotIndex int, sandbox labnetwork.Sandbox) (_ *SandboxNetwork, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !sandboxIDForNetwork.MatchString(sandbox.ID) {
		return nil, fmt.Errorf("invalid sandbox ID")
	}
	if err := validateSandboxNetwork(cfg); err != nil {
		return nil, err
	}
	address, err := sandboxAddress(cfg, slotIndex)
	if err != nil {
		return nil, err
	}
	ns, err := labnetwork.PinNetworkNamespace(sandbox.PID, sandbox.Namespace)
	if err != nil {
		return nil, err
	}
	defer ns.Close()
	host, err := sandboxNetworkHandle()
	if err != nil {
		return nil, err
	}
	defer host.Close()
	guest, err := netlink.NewHandleAt(netns.NsHandle(ns.FD()), unix.NETLINK_ROUTE)
	if err != nil {
		return nil, err
	}
	defer guest.Close()
	if err := guest.SetSocketTimeout(5 * time.Second); err != nil {
		return nil, err
	}
	bridge, err := host.LinkByName(cfg.BridgeName)
	if err != nil {
		return nil, err
	}
	if bridge.Type() != "bridge" {
		return nil, fmt.Errorf("sandbox interface is not a bridge")
	}
	if _, err := guest.LinkByName("eth0"); err == nil {
		return nil, fmt.Errorf("sandbox eth0 already exists")
	} else if !missingSandboxLink(err) {
		return nil, err
	}
	hostName, peerName := VethNameForSandbox(slotIndex, sandbox.ID)
	alias := "attackbox:sandbox:" + sandbox.ID
	veth := &netlink.Veth{LinkAttrs: netlink.LinkAttrs{Name: hostName, MTU: bridge.Attrs().MTU}, PeerName: peerName, PeerNamespace: netlink.NsFd(ns.FD()), PeerMTU: uint32(bridge.Attrs().MTU)}
	owned, err := labnetwork.CreateOwnedVeth(host, guest, veth, alias)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, host.LinkDel(owned))
		}
	}()
	peer, err := guest.LinkByName(peerName)
	if err != nil {
		return nil, err
	}
	if err = guest.LinkSetAlias(peer, alias); err != nil {
		return nil, err
	}
	if err = guest.LinkSetName(peer, "eth0"); err != nil {
		return nil, err
	}
	if err = host.LinkSetMaster(owned, bridge); err != nil {
		return nil, err
	}
	// eth0 ports remain isolated; only dedicated lab* ports permit lab L2 traffic.
	if err = host.LinkSetIsolated(owned, true); err != nil {
		return nil, err
	}
	addr, err := netlink.ParseAddr(address)
	if err != nil {
		return nil, err
	}
	if err = guest.AddrAdd(peer, addr); err != nil {
		return nil, err
	}
	if err = guest.LinkSetUp(peer); err != nil {
		return nil, err
	}
	loopback, err := guest.LinkByName("lo")
	if err != nil {
		return nil, err
	}
	if err = guest.LinkSetUp(loopback); err != nil {
		return nil, err
	}
	if err = guest.RouteAdd(&netlink.Route{LinkIndex: peer.Attrs().Index, Gw: net.ParseIP(cfg.GatewayIP)}); err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if err = host.LinkSetUp(owned); err != nil {
		return nil, err
	}
	return &SandboxNetwork{SlotIndex: slotIndex, IP: addr.IP.String(), VethHost: hostName, VethGuest: "eth0", Namespace: ns.Identity}, nil
}

func TeardownSandboxNetwork(ctx context.Context, slotIndex int, sandboxID string) error {
	if !sandboxIDForNetwork.MatchString(sandboxID) {
		return fmt.Errorf("invalid sandbox ID")
	}
	if slotIndex <= 0 {
		return cleanupSandboxInterfaces(ctx, sandboxID)
	}
	h, err := sandboxNetworkHandle()
	if err != nil {
		return err
	}
	defer h.Close()
	name, _ := VethNameForSandbox(slotIndex, sandboxID)
	return deleteSandboxInterface(ctx, h, name, "attackbox:sandbox:"+sandboxID)
}

func deleteSandboxInterface(ctx context.Context, h *netlink.Handle, name, alias string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	link, err := h.LinkByName(name)
	if missingSandboxLink(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if link.Type() != "veth" || link.Attrs().Alias != alias {
		return fmt.Errorf("refusing to delete unowned sandbox interface %s", name)
	}
	return h.LinkDel(link)
}

// CleanupSandboxInterfaces runs before accepting controller/gateway work. Only
// exact name + ownership markers qualify; arbitrary vh* devices are not ours.
func CleanupSandboxInterfaces(ctx context.Context) error { return cleanupSandboxInterfaces(ctx, "") }
func cleanupSandboxInterfaces(ctx context.Context, sandboxID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	h, err := sandboxNetworkHandle()
	if err != nil {
		return err
	}
	defer h.Close()
	links, err := h.LinkList()
	if err != nil {
		return err
	}
	var errs []error
	for _, link := range links {
		alias := link.Attrs().Alias
		if !strings.HasPrefix(alias, "attackbox:sandbox:") {
			continue
		}
		id := strings.TrimPrefix(alias, "attackbox:sandbox:")
		if !sandboxIDForNetwork.MatchString(id) || sandboxID != "" && id != sandboxID {
			continue
		}
		parts := strings.SplitN(strings.TrimPrefix(link.Attrs().Name, "vh"), "_", 2)
		slot, err := strconv.Atoi(parts[0])
		if err != nil || slot < 1 || slot > 250 {
			continue
		}
		expected, _ := VethNameForSandbox(slot, id)
		if link.Attrs().Name != expected {
			continue
		}
		if err := deleteSandboxInterface(ctx, h, expected, alias); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
