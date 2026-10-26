//go:build linux

package labnetwork

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
)

type helperRequest struct{ Action, Protocol, Address string }
type helperResponse struct {
	OK              bool
	Error           string
	Source          string
	Names, Defaults []string
}

// A process in a real network namespace stands in for the Btrfs sandbox.
// Control uses inherited pipes, not a network port or a second PID lookup.
func TestLabNamespaceHelper(t *testing.T) {
	if os.Getenv("ATTACKBOX_LAB_NS_HELPER") != "1" {
		t.Skip("acceptance subprocess only")
	}
	h, err := netlink.NewHandle(syscall.NETLINK_ROUTE)
	if err != nil {
		panic(err)
	}
	lo, err := h.LinkByName("lo")
	if err != nil {
		panic(err)
	}
	if err = h.LinkSetUp(lo); err != nil {
		panic(err)
	}
	attrs := netlink.NewLinkAttrs()
	attrs.Name = "eth0"
	primary := &netlink.Dummy{LinkAttrs: attrs}
	if err = h.LinkAdd(primary); err != nil {
		panic(err)
	}
	addr, _ := netlink.ParseAddr("192.0.2.2/24")
	if err = h.AddrAdd(primary, addr); err != nil {
		panic(err)
	}
	if err = h.LinkSetUp(primary); err != nil {
		panic(err)
	}
	if err = h.RouteAdd(&netlink.Route{LinkIndex: primary.Attrs().Index, Gw: net.ParseIP("192.0.2.1")}); err != nil {
		panic(err)
	}
	tcp, err := net.Listen("tcp4", ":18080")
	if err != nil {
		panic(err)
	}
	go func() {
		for {
			c, err := tcp.Accept()
			if err != nil {
				return
			}
			remote, _, _ := net.SplitHostPort(c.RemoteAddr().String())
			c.SetWriteDeadline(time.Now().Add(time.Second))
			fmt.Fprintln(c, remote)
			c.Close()
		}
	}()
	udp, err := net.ListenPacket("udp4", ":18081")
	if err != nil {
		panic(err)
	}
	go func() {
		b := make([]byte, 16)
		for {
			n, a, err := udp.ReadFrom(b)
			if err != nil {
				return
			}
			udp.WriteTo(b[:n], a)
		}
	}()
	dec, enc := json.NewDecoder(os.Stdin), json.NewEncoder(os.Stdout)
	enc.Encode(helperResponse{OK: true})
	for {
		var req helperRequest
		if err := dec.Decode(&req); err != nil {
			break
		}
		res := helperResponse{OK: true}
		switch req.Action {
		case "http":
			client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{Proxy: nil}}
			resp, err := client.Get(req.Address)
			if err != nil {
				res.OK = false
				res.Error = err.Error()
			} else {
				var who struct {
					Source string `json:"source"`
				}
				err = json.NewDecoder(resp.Body).Decode(&who)
				resp.Body.Close()
				res.OK = err == nil
				res.Source = who.Source
			}
		case "snapshot":
			links, err := h.LinkList()
			if err != nil {
				res.OK = false
				res.Error = err.Error()
			}
			for _, l := range links {
				res.Names = append(res.Names, l.Attrs().Name)
			}
			routes, err := h.RouteList(nil, netlink.FAMILY_V4)
			if err != nil {
				res.OK = false
				res.Error = err.Error()
			}
			for _, r := range routes {
				if r.Dst == nil {
					res.Defaults = append(res.Defaults, fmt.Sprintf("%s/%d", r.Gw, r.LinkIndex))
				}
			}
			sort.Strings(res.Names)
			sort.Strings(res.Defaults)
		case "probe":
			c, err := net.DialTimeout(req.Protocol, req.Address, time.Second)
			if err != nil {
				res.OK = false
				res.Error = err.Error()
			} else {
				c.SetDeadline(time.Now().Add(time.Second))
				if req.Protocol == "udp" {
					_, err = c.Write([]byte("ping"))
					if err == nil {
						b := make([]byte, 4)
						_, err = io.ReadFull(c, b)
					}
					res.OK = err == nil
				}
				c.Close()
			}
		default:
			res.OK = false
			res.Error = "unknown helper action"
		}
		enc.Encode(res)
	}
	h.Close()
	os.Exit(0)
}

func TestLocalAttachmentAcceptance(t *testing.T) {
	if os.Getenv("ATTACKBOX_TEST_LAB_NETWORK") != "1" {
		t.Skip("opt in on a disposable host with NET_ADMIN, SYS_ADMIN and nft")
	}
	if _, err := exec.LookPath("nft"); err != nil {
		t.Fatal("nft is required; enabled acceptance must not silently skip")
	}
	socket, image, poolText := os.Getenv("ATTACKBOX_TEST_DOCKER_SOCKET"), os.Getenv("ATTACKBOX_TEST_LAB_IMAGE"), os.Getenv("ATTACKBOX_TEST_LAB_POOL")
	if socket == "" || image == "" || poolText == "" {
		t.Fatal("set explicit local socket, offline-built fixture image and unused test pool")
	}
	pool, err := netip.ParsePrefix(poolText)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestLabNamespaceHelper$")
	cmd.Env = append(os.Environ(), "ATTACKBOX_LAB_NS_HELPER=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{Cloneflags: syscall.CLONE_NEWNET}
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("cannot create fixture namespace (NET_ADMIN and SYS_ADMIN required): %v", err)
	}
	t.Cleanup(func() { in.Close(); cmd.Process.Kill(); cmd.Wait() })
	dec, enc := json.NewDecoder(out), json.NewEncoder(in)
	var ready helperResponse
	if err := dec.Decode(&ready); err != nil || !ready.OK {
		t.Fatal("namespace helper startup", err)
	}
	request := func(req helperRequest) helperResponse {
		t.Helper()
		if err := enc.Encode(req); err != nil {
			t.Fatal(err)
		}
		var res helperResponse
		if err := dec.Decode(&res); err != nil {
			t.Fatal(err)
		}
		return res
	}
	before := request(helperRequest{Action: "snapshot"})
	id, err := CaptureNamespace(cmd.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	s := Sandbox{ID: "fixture-sandbox", OwnerID: "fixture-user", PID: cmd.Process.Pid, Namespace: id}
	suffix := shortHash(time.Now().String(), 10)
	node := "fixture-" + suffix
	m, err := New(Config{NodeID: node, StateDir: t.TempDir(), DockerSocket: socket, Pool: pool})
	if err != nil {
		t.Fatal(err)
	}
	d := m.docker.(*localDocker)
	var containers []string
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		for _, id := range containers {
			if err := d.request(cleanupCtx, "DELETE", "/containers/"+id+"?force=1&v=1", nil, nil); err != nil {
				t.Error(err)
			}
		}
		nets, _ := m.Snapshot()
		clean := true
		for _, n := range nets {
			if err := m.ReleaseNetwork(cleanupCtx, n.ID); err != nil {
				clean = false
				t.Error("cleanup pending", n.Name, err)
			}
		}
		if clean {
			guard := nftProtection{node: node}
			if owned, err := guard.ownedTable(cleanupCtx); err != nil {
				t.Error(err)
			} else if owned {
				if _, err := nft(cleanupCtx, "", "delete", "table", "inet", "attackbox_labs_"+shortHash(node, 8)); err != nil {
					t.Error(err)
				}
			}
		}
		m.Close()
	})
	n, err := m.CreateNetwork(ctx, NetworkSpec{TargetID: "fixture_" + suffix, OwnerID: s.OwnerID, Key: "front"})
	if err != nil {
		t.Fatal(err)
	}
	createContainer := func(n Network, alias string) string {
		t.Helper()
		var c struct {
			ID string `json:"Id"`
		}
		payload := map[string]any{"Image": image, "Labels": map[string]string{"io.attackbox.fixture": suffix}, "HostConfig": map[string]any{"NetworkMode": n.Name, "Memory": int64(64 << 20), "PidsLimit": int64(32), "RestartPolicy": map[string]string{"Name": "no"}}, "NetworkingConfig": map[string]any{"EndpointsConfig": map[string]any{n.Name: map[string]any{"Aliases": []string{alias}}}}}
		if err := d.request(ctx, "POST", "/containers/create", payload, &c); err != nil {
			t.Fatal(err)
		}
		containers = append(containers, c.ID)
		if err := d.request(ctx, "POST", "/containers/"+c.ID+"/start", nil, nil); err != nil {
			t.Fatal(err)
		}
		var detail struct {
			NetworkSettings struct {
				Networks map[string]struct{ IPAddress string }
			}
		}
		if err := d.request(ctx, "GET", "/containers/"+c.ID+"/json", nil, &detail); err != nil {
			t.Fatal(err)
		}
		ip := detail.NetworkSettings.Networks[n.Name].IPAddress
		if ip == "" || ip == n.SandboxIP.String() {
			t.Fatal("Docker allocated reserved sandbox address", ip)
		}
		return ip
	}
	ip := createContainer(n, "web")
	createContainer(n, "peer")
	a, err := m.Attach(ctx, n.ID, s)
	if err != nil {
		t.Fatal(err)
	}
	if again, err := m.Attach(ctx, n.ID, s); err != nil || again != a {
		t.Fatal("attachment not idempotent", again, err)
	}
	after := request(helperRequest{Action: "snapshot"})
	if strings.Join(before.Defaults, ",") != strings.Join(after.Defaults, ",") || !contains(after.Names, "eth0") || !contains(after.Names, a.GuestName) {
		t.Fatal("primary route changed or lab interface missing", before, after)
	}
	for _, p := range []struct{ proto, port string }{{"tcp", "8080"}, {"udp", "8081"}} {
		var res helperResponse
		for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
			res = request(helperRequest{Action: "probe", Protocol: p.proto, Address: net.JoinHostPort(ip, p.port)})
			if res.OK {
				break
			}
			time.Sleep(30 * time.Millisecond)
		}
		if !res.OK {
			t.Fatal("sandbox to lab failed", p, res)
		}
	}
	client := &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{Proxy: nil}}
	who := request(helperRequest{Action: "http", Address: "http://" + ip + ":8080/whoami"})
	if !who.OK || who.Source != n.SandboxIP.String() {
		t.Fatal("sandbox source IP was translated", who)
	}
	probe := func(address, proto string) bool {
		t.Helper()
		resp, err := client.Get("http://" + ip + ":8080/probe?network=" + proto + "&address=" + url.QueryEscape(address))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var result struct {
			OK bool `json:"ok"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatal(err)
		}
		return result.OK
	}
	if !probe(net.JoinHostPort(n.SandboxIP.String(), "18080"), "tcp") || !probe(net.JoinHostPort(n.SandboxIP.String(), "18081"), "udp") {
		t.Fatal("lab callback failed")
	}
	callback, err := client.Get("http://" + ip + ":8080/probe?network=tcp&source=1&address=" + url.QueryEscape(net.JoinHostPort(n.SandboxIP.String(), "18080")))
	if err != nil {
		t.Fatal(err)
	}
	var observed struct {
		OK     bool   `json:"ok"`
		Source string `json:"source"`
	}
	err = json.NewDecoder(callback.Body).Decode(&observed)
	callback.Body.Close()
	if err != nil || !observed.OK || observed.Source != ip {
		t.Fatal("target source IP was translated", observed, err)
	}
	if !probe("peer:8080", "tcp") {
		t.Fatal("Docker service DNS broken")
	}
	trap, err := net.Listen("tcp4", net.JoinHostPort(n.Gateway.String(), "0"))
	if err != nil {
		t.Fatal(err)
	}
	defer trap.Close()
	if probe(trap.Addr().String(), "tcp") || request(helperRequest{Action: "probe", Protocol: "tcp", Address: trap.Addr().String()}).OK {
		t.Fatal("host service reachable")
	}
	n2, err := m.CreateNetwork(ctx, NetworkSpec{TargetID: "fixture_other_" + suffix, OwnerID: "other-user", Key: "front"})
	if err != nil {
		t.Fatal(err)
	}
	ip2 := createContainer(n2, "other")
	if probe(net.JoinHostPort(ip2, "8080"), "tcp") {
		t.Fatal("cross-user network reachable")
	}
	if probe("198.51.100.1:443", "tcp") {
		t.Fatal("unexpected external connection")
	}
	bad := s
	bad.Namespace.Inode++
	if _, err := m.Attach(ctx, n.ID, bad); err != ErrNamespaceChanged {
		t.Fatal("namespace mismatch accepted", err)
	}
	if err := m.Detach(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if contains(request(helperRequest{Action: "snapshot"}).Names, a.GuestName) {
		t.Fatal("guest interface survived detach")
	}
	// A colliding interface must survive both failed attach and its rollback.
	l := m.links.(*localLinks)
	attrs := netlink.NewLinkAttrs()
	attrs.Name = a.HostName
	foreign := &netlink.Dummy{LinkAttrs: attrs}
	if err := l.host.LinkAdd(foreign); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Attach(ctx, n.ID, s); err == nil {
		t.Fatal("foreign interface adopted")
	}
	if _, err := l.host.LinkByName(a.HostName); err != nil {
		t.Fatal("foreign interface deleted", err)
	}
	if err := l.host.LinkDel(foreign); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Attach(ctx, n.ID, s); err != nil {
		t.Fatal(err)
	}
	// Simulate Worker restart while preserving the real namespace and ledger.
	cfg := m.cfg
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	m = reopened
	d = m.docker.(*localDocker)
	if report, err := m.Reconcile(ctx); err != nil || len(report.Pending) != 0 {
		t.Fatal(report, err)
	}
	// Destroying the namespace must clean only its attachment, not the lab.
	pinned, err := PinNamespace(s)
	if err != nil {
		t.Fatal(err)
	}
	cmd.Process.Kill()
	cmd.Wait()
	if identity, err := fileIdentity(pinned.file); err != nil || identity != s.Namespace {
		t.Fatal("namespace descriptor changed after process exit", identity, err)
	}
	pinned.Close()
	if report, err := m.Reconcile(ctx); err != nil || len(report.Detached) != 1 {
		t.Fatal(report, err)
	}
	if err := d.Verify(ctx, n, node); err != nil {
		t.Fatal("sandbox exit deleted lab", err)
	}
}

func contains(items []string, item string) bool {
	for _, v := range items {
		if v == item {
			return true
		}
	}
	return false
}
