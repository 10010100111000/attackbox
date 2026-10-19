#!/bin/bash
set -e

# 1. Comment out kernel.modules_disabled = 1 in 99-security-stage0.conf
sudo sed -i 's/^kernel.modules_disabled/# kernel.modules_disabled/' /etc/sysctl.d/99-security-stage0.conf

# 2. Add veth, bridge, and br_netfilter to /etc/modules-load.d/attackbox.conf
cat << 'EOF' | sudo tee /etc/modules-load.d/attackbox.conf
veth
bridge
br_netfilter
EOF

echo "=== /etc/sysctl.d/99-security-stage0.conf ==="
grep "modules_disabled" /etc/sysctl.d/99-security-stage0.conf

echo "=== /etc/modules-load.d/attackbox.conf ==="
cat /etc/modules-load.d/attackbox.conf
