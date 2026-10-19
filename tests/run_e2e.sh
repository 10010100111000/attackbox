#!/usr/bin/env bash
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$DIR"

echo "[0/4] Cleaning any stale test processes..."
sudo -n fuser -k 18080/tcp 18443/tcp 2>/dev/null || true
sudo -n pkill -9 -f /tmp/attackbox 2>/dev/null || true

echo "[1/4] Building latest binaries..."
/usr/bin/go build -o /tmp/attackbox-control ./cmd/control
/usr/bin/go build -o /tmp/attackbox-node ./cmd/node

echo "[2/4] Generating temporary test configurations..."
cat <<EOF > /tmp/test_control.yaml
server:
  listen_addr: "127.0.0.1:18080"
  cors_allowed_origins: ["*"]
database:
  path: "/tmp/test_attackbox_e2e.db"
cluster:
  cluster_secret: "security_cluster_secret_min_32_bytes_long!"
  heartbeat_timeout_sec: 15
  rpc_timeout_sec: 30
auth:
  jwt_expire_hours: 24
  ticket_ttl_sec: 60
sandbox:
  resources:
    memory_max_mb: 2048
    memory_swap_max_mb: 0
    pids_max: 512
    cpu_weight: 100
    io_write_bytes_sec: 20971520
    io_write_iops: 500
  storage:
    tmp_size: "512M"
    run_size: "256M"
  network:
    subnet_cidr: "10.200.0.0/16"
    gateway_ip: "10.200.0.1"
    default_egress_mode: "restricted"
    allowed_egress_ports: [53, 80, 443]
EOF

cat <<EOF > /tmp/test_node.yaml
node:
  node_id: "worker-node-e2e-01"
  control_url: "ws://127.0.0.1:18080/api/agent/connect"
  cluster_secret: "security_cluster_secret_min_32_bytes_long!"
  heartbeat_interval_sec: 2
gateway:
  listen_addr: "127.0.0.1:18443"
  tls_enabled: false
runtime:
  base_rootfs: "/var/lib/attackbox/base_rootfs"
  instances_dir: "/var/lib/attackbox/instances"
  cgroup_root: "/sys/fs/cgroup/attackbox"
  disk_guard:
    enabled: true
    check_interval_sec: 3
    min_free_space_mb: 5120
    warn_free_space_mb: 10240
EOF

rm -f /tmp/test_attackbox_e2e.db*

cleanup() {
    echo "Cleaning up test background services..."
    sudo pkill -9 -f /tmp/attackbox-node 2>/dev/null || true
    kill -9 "$CTRL_PID" 2>/dev/null || true
    pkill -9 -f /tmp/attackbox-control 2>/dev/null || true
    rm -f /tmp/test_attackbox_e2e.db* /tmp/test_control.yaml /tmp/test_node.yaml
}
trap cleanup EXIT INT TERM

echo "[3/4] Launching Control Hub & Node Daemon..."
/tmp/attackbox-control -config /tmp/test_control.yaml > /tmp/test_control.log 2>&1 &
CTRL_PID=$!

sleep 1

sudo /tmp/attackbox-node -config /tmp/test_node.yaml > /tmp/test_node.log 2>&1 &
NODE_PID=$!

sleep 2

echo "[4/4] Executing Python E2E Security & P1-3 Test Suite..."
python3 tests/test_attackbox_e2e.py

echo "SUCCESS! All end-to-end tests completed."
