#!/bin/bash
# scripts/test_net.sh - 验证 br0 与 nft 规则
set -e
bash scripts/attackbox-net.sh
ip link show br0 | grep -q "UP"
nft list table inet attackbox | grep -q "reject"
echo "PASS: Network base initialized successfully"
