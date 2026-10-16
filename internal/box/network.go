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

	// 4. 应用 nftables 安全拦截规则
	nftRules := fmt.Sprintf(`
table inet attackbox {
    chain forward {
        type filter hook forward priority 0; policy drop;
        ct state established,related accept
        iifname "br0" oifname "br0" accept
        iifname "br0" oifname "docker0" accept
        iifname "br0" oifname "tun*" accept
        iifname "br0" oifname "wg*" accept
        iifname "br0" ip daddr 1.1.1.1 udp dport 53 accept
        iifname "br0" oifname "%s" reject
    }
    chain input {
        type filter hook input priority 0;
        iifname "br0" tcp dport { 22, 443 } reject
    }
}
`, wanIF)

	_ = exec.Command("nft", "delete", "table", "inet", "attackbox").Run()
	cmd := exec.Command("nft", "-f", "-")
	cmd.Stdin = bytes.NewReader([]byte(nftRules))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("apply nftables: %w", err)
	}

	return nil
}
