//go:build linux

package node

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"syscall"
	"testing"

	"attackbox/node/labnetwork"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

func TestPrimaryNetworkRejectsHostNamespaceAndChangedIdentity(t *testing.T) {
	cfg := NetworkConfig{BridgeName: "br-primary", GatewayIP: "10.200.0.1", SubnetCIDR: "10.200.0.0/16"}
	identity, err := labnetwork.CaptureNamespace(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	sandbox := labnetwork.Sandbox{ID: "sbx_test", PID: os.Getpid(), Namespace: identity}
	if _, err := SetupSandboxNetwork(context.Background(), cfg, 1, sandbox); err == nil {
		t.Fatal("host namespace accepted")
	}
	sandbox.Namespace.Inode++
	if _, err := SetupSandboxNetwork(context.Background(), cfg, 1, sandbox); !errors.Is(err, labnetwork.ErrNamespaceChanged) {
		t.Fatal("namespace mismatch not rejected", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := SetupSandboxNetwork(ctx, cfg, 1, sandbox); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := SetupSandboxNetwork(context.Background(), cfg, 0, sandbox); err == nil {
		t.Fatal("invalid slot accepted")
	}
}

func TestSandboxNetworkNamespaceHelper(t *testing.T) {
	if os.Getenv("ATTACKBOX_PRIMARY_NS_HELPER") != "1" {
		return
	}
	fmt.Println("READY")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
	}
	os.Exit(0)
}

// Run only in a disposable host network namespace, e.g. a local Docker test
// container with --network none and NET_ADMIN/SYS_ADMIN. No Docker daemon or
// nftables mutations are needed inside that container.
func TestSandboxPrimaryNetworkAcceptance(t *testing.T) {
	if os.Getenv("ATTACKBOX_TEST_SANDBOX_NETWORK") != "1" {
		t.Skip("requires isolated NET_ADMIN/SYS_ADMIN namespace")
	}
	ctx := context.Background()
	cfg := NetworkConfig{BridgeName: "br-primary", GatewayIP: "10.200.0.1", SubnetCIDR: "10.200.0.0/16"}
	if err := ensureSandboxBridge(cfg, "10.200.0.1/16"); err != nil {
		t.Fatal(err)
	}
	if err := ensureSandboxBridge(cfg, "10.200.0.1/16"); err != nil {
		t.Fatal("bridge initialization is not idempotent", err)
	}
	if err := ensureSandboxBridge(cfg, "10.200.0.2/16"); err == nil {
		t.Fatal("conflicting bridge address accepted")
	}
	host, err := sandboxNetworkHandle()
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	bridge, _ := host.LinkByName(cfg.BridgeName)
	defer host.LinkDel(bridge)
	cmd := exec.Command(os.Args[0], "-test.run=^TestSandboxNetworkNamespaceHelper$")
	cmd.Env = append(os.Environ(), "ATTACKBOX_PRIMARY_NS_HELPER=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{Cloneflags: syscall.CLONE_NEWNET}
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { input.Close(); cmd.Process.Kill(); cmd.Wait() }()
	scanner := bufio.NewScanner(output)
	if !scanner.Scan() || scanner.Text() != "READY" {
		t.Fatal("namespace helper not ready", scanner.Err())
	}
	identity, err := labnetwork.CaptureNamespace(cmd.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	sandbox := labnetwork.Sandbox{ID: "sbx_owned123", PID: cmd.Process.Pid, Namespace: identity}
	network, err := SetupSandboxNetwork(ctx, cfg, 1, sandbox)
	if err != nil {
		t.Fatal(err)
	}
	if network.IP != "10.200.0.10" || network.Namespace != identity {
		t.Fatal(network)
	}
	pinned, err := labnetwork.PinNetworkNamespace(sandbox.PID, identity)
	if err != nil {
		t.Fatal(err)
	}
	defer pinned.Close()
	guest, err := netlink.NewHandleAt(netns.NsHandle(pinned.FD()), unix.NETLINK_ROUTE)
	if err != nil {
		t.Fatal(err)
	}
	defer guest.Close()
	peer, err := guest.LinkByName("eth0")
	if err != nil {
		t.Fatal(err)
	}
	if peer.Attrs().Alias != "attackbox:sandbox:"+sandbox.ID || peer.Attrs().Flags&net.FlagUp == 0 {
		t.Fatal("guest identity or state", peer)
	}
	addresses, err := guest.AddrList(peer, netlink.FAMILY_V4)
	if err != nil || len(addresses) != 1 || addresses[0].IPNet.String() != "10.200.0.10/16" {
		t.Fatal(addresses, err)
	}
	routes, err := guest.RouteList(peer, netlink.FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	defaultFound := false
	for _, route := range routes {
		if (route.Dst == nil || route.Dst.String() == "0.0.0.0/0") && route.Gw.Equal(net.ParseIP(cfg.GatewayIP)) {
			defaultFound = true
		}
	}
	if !defaultFound {
		t.Fatal("default route missing", routes)
	}
	hostPeer, err := host.LinkByName(network.VethHost)
	if err != nil {
		t.Fatal(err)
	}
	info, err := host.LinkGetProtinfo(hostPeer)
	if err != nil || !info.Isolated || hostPeer.Attrs().MasterIndex != bridge.Attrs().Index {
		t.Fatal("primary bridge isolation missing", info, err)
	}
	if _, err := SetupSandboxNetwork(ctx, cfg, 1, sandbox); err == nil {
		t.Fatal("live eth0 replaced")
	}
	if err := TeardownSandboxNetwork(ctx, 1, sandbox.ID); err != nil {
		t.Fatal(err)
	}
	if err := TeardownSandboxNetwork(ctx, 1, sandbox.ID); err != nil {
		t.Fatal("teardown not idempotent", err)
	}
	if _, err := guest.LinkByName("eth0"); !missingSandboxLink(err) {
		t.Fatal("guest peer survived teardown", err)
	}
	// Never adopt/delete an unrelated interface with a colliding deterministic name.
	collision, _ := VethNameForSandbox(1, sandbox.ID)
	dummy := &netlink.Veth{LinkAttrs: netlink.LinkAttrs{Name: collision}, PeerName: "foreignpeer"}
	if err := host.LinkAdd(dummy); err != nil {
		t.Fatal(err)
	}
	if err := TeardownSandboxNetwork(ctx, 1, sandbox.ID); err == nil {
		t.Fatal("foreign device deleted")
	}
	if _, err := SetupSandboxNetwork(ctx, cfg, 1, sandbox); err == nil {
		t.Fatal("collision adopted")
	}
	if _, err := host.LinkByName(collision); err != nil {
		t.Fatal("collision removed", err)
	}
	if err := CleanupSandboxInterfaces(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := host.LinkByName(collision); err != nil {
		t.Fatal("startup cleanup removed foreign device", err)
	}
	if err := host.LinkDel(dummy); err != nil {
		t.Fatal(err)
	}
	// A default-route conflict after pair creation must roll back the owned pair.
	lo, err := guest.LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}
	conflict := &netlink.Route{LinkIndex: lo.Attrs().Index, Dst: &net.IPNet{IP: net.IPv4zero, Mask: net.CIDRMask(0, 32)}}
	if err := guest.RouteAdd(conflict); err != nil {
		t.Fatal(err)
	}
	if _, err := SetupSandboxNetwork(ctx, cfg, 1, sandbox); err == nil {
		t.Fatal("default route conflict ignored")
	}
	if _, err := host.LinkByName(network.VethHost); !missingSandboxLink(err) {
		t.Fatal("failed setup leaked veth", err)
	}
	if _, err := guest.LinkByName("eth0"); !missingSandboxLink(err) {
		t.Fatal("failed setup leaked guest veth", err)
	}
}
