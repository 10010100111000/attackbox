#!/bin/bash
# scripts/attackbox-net.sh - 宿主网桥与 nftables 出网安全基线
set -euo pipefail

BRIDGE="br0"
BRIDGE_IP="10.200.0.1/24"
DNS_ALLOWED="1.1.1.1"

# 动态探测宿主机外网出口网卡
WAN_IF=$(ip route show default | awk '{print $5}' | head -n1 || true)
if [ -z "$WAN_IF" ]; then
    WAN_IF="eth0"
fi

# 1. 开启内核 IPv4 转发
sysctl -w net.ipv4.ip_forward=1 >/dev/null

# 2. 幂等创建原生网桥 br0
if ! ip link show "$BRIDGE" >/dev/null 2>&1; then
    ip link add "$BRIDGE" type bridge
    ip addr add "$BRIDGE_IP" dev "$BRIDGE"
    ip link set "$BRIDGE" up
fi

# 3. 配置 nftables 拦截规则
nft delete table inet attackbox 2>/dev/null || true
nft add table inet attackbox

# 仅允许指定 DNS，其余流向公网外网网卡（如 eth0 / ens3）的流量全部强制阻断
nft add chain inet attackbox forward '{ type filter hook forward priority 0; }'
nft add rule inet attackbox forward iifname "$BRIDGE" oifname "$WAN_IF" ip daddr "$DNS_ALLOWED" udp dport 53 accept
nft add rule inet attackbox forward iifname "$BRIDGE" oifname "$WAN_IF" reject

# 阻断沙箱访问宿主自身关键管理端口
nft add chain inet attackbox input '{ type filter hook input priority 0; }'
nft add rule inet attackbox input iifname "$BRIDGE" tcp dport { 22, 443 } reject

echo "attackbox-net: br0 ready, egress firewall applied (WAN: $WAN_IF)"
