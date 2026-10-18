package box

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// EnsureHostNetwork 负责宿主机基础网络与 nftables 隔离基线的初始化 (完全替代原 attackbox-net.sh)
func EnsureHostNetwork() error {
	// 1. 开启内核 IPv4 转发
	_ = os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1\n"), 0644)
	_ = exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1").Run()

	// 2. 幂等创建网桥 br0
	bridge := "br0"
	bridgeIP := "10.200.0.1/24"
	if err := exec.Command("ip", "link", "show", bridge).Run(); err != nil {
		_ = exec.Command("ip", "link", "add", bridge, "type", "bridge").Run()
		_ = exec.Command("ip", "addr", "add", bridgeIP, "dev", bridge).Run()
		_ = exec.Command("ip", "link", "set", bridge, "up").Run()
	}

	// 3. 动态探测外网出口网卡
	wanIF := "eth0"
	if out, err := exec.Command("ip", "route", "show", "default").Output(); err == nil {
		fields := strings.Fields(string(out))
		for i, f := range fields {
			if f == "dev" && i+1 < len(fields) {
				wanIF = fields[i+1]
				break
			}
		}
	}

	// 4. 应用 nftables 安全拦截规则 (原子化且 fail-closed)
	nftRules := fmt.Sprintf(`
table inet attackbox {
    chain forward {
        type filter hook forward priority 0; policy drop;
        ct state established,related accept

        # 内部/靶机流量完全放行 (全端口、全协议、不限速)
        iifname "br0" oifname "br0" accept
        iifname "br0" oifname "docker0" accept
        iifname "br0" oifname "podman*" accept
        iifname "br0" oifname "tun*" accept
        iifname "br0" oifname "wg*" accept

        # DNS 解析放行
        iifname "br0" ip daddr { 1.1.1.1, 8.8.8.8, 8.8.4.4, 9.9.9.9 } udp dport 53 accept
        iifname "br0" ip daddr { 1.1.1.1, 8.8.8.8, 8.8.4.4, 9.9.9.9 } tcp dport 53 accept

        # 外网出口（WAN，如 ens3/eth0）：仅放行 Web 端口 (80, 443)，其余外网全数阻断
        iifname "br0" oifname "%s" tcp dport { 80, 443 } accept
        iifname "br0" oifname "%s" reject
    }
    chain input {
        type filter hook input priority 0;
        iifname "br0" ct state established,related accept
        iifname "br0" icmp type echo-request accept
        # 阻断沙箱访问母机本地敏感服务（包括宿主 SSH 22 等）
        iifname "br0" reject
    }
}
`, wanIF, wanIF)

	_ = exec.Command("nft", "delete", "table", "inet", "attackbox").Run()
	cmd := exec.Command("nft", "-f", "-")
	cmd.Stdin = bytes.NewReader([]byte(nftRules))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("apply nftables: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	// 5. 确保外网出口 NAT MASQUERADE 规则 (幂等维护，允许沙箱 Web 端口正常出网下载)
	if err := exec.Command("iptables", "-t", "nat", "-C", "POSTROUTING", "-s", "10.200.0.0/24", "-o", wanIF, "-j", "MASQUERADE").Run(); err != nil {
		_ = exec.Command("iptables", "-t", "nat", "-A", "POSTROUTING", "-s", "10.200.0.0/24", "-o", wanIF, "-j", "MASQUERADE").Run()
	}

	return nil
}
