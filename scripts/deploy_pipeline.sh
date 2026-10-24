#!/usr/bin/env bash
set -euo pipefail

export PATH="$HOME/.local/bin:$PATH"

echo "=========================================================="
echo "  [1/6] 检查 Git 仓库状态与提交"
echo "=========================================================="
cd /home/laji/attackbox
git log -1 --oneline

echo "=========================================================="
echo "  [2/6] 配置 node.yaml 并编译 Go Worker"
echo "=========================================================="
cat << 'EOF' > /home/laji/attackbox/configs/node.yaml
node:
  node_id: "node-primary"
  cluster_secret: "attackbox_cluster_secret_change_me_in_prod!"
  control_url: "ws://127.0.0.1:3000/cluster/connect"
  heartbeat_interval_sec: 3

gateway:
  listen_addr: "0.0.0.0:8443"
  # Must match SANDBOX_NODES_JSON below; ws:// is only valid for loopback and the
  # browser rewrites the host to the page host before dialing the worker.
  public_data_url: "ws://127.0.0.1:8443"
  tls_enabled: false

runtime:
  base_rootfs: "/var/lib/attackbox/base_rootfs"
  instances_dir: "/var/lib/attackbox/instances"
  cgroup_root: "/sys/fs/cgroup/attackbox"
  limits:
    pids_max: 256
    memory_max_mb: 1024
    memory_swap_max_mb: 0
    cpu_weight: 100
    io_write_bytes_sec: 20971520
    io_write_iops: 1000
  disk_guard:
    enabled: true
    check_interval_sec: 3
    min_free_space_mb: 5120
    warn_free_space_mb: 10240
  tmpfs:
    tmp_size: "512M"
    run_size: "256M"

network:
  bridge_name: "br-attackbox"
  gateway_ip: "10.200.0.1"
  subnet_cidr: "10.200.0.0/16"
  egress_interface: ""

  docker_targets:
    enabled: true
    socket_path: "/var/run/docker.sock"
    name: "attackbox-targets"
    bridge_name: "br-ab-targets"
    subnet_cidr: "10.201.0.0/16"
    gateway_ip: "10.201.0.1"
EOF

mkdir -p /home/laji/attackbox/bin
go build -v -o /home/laji/attackbox/bin/node ./cmd/node
echo "Worker 二进制编译成功: /home/laji/attackbox/bin/node"

echo "=========================================================="
echo "  [3/6] 应用数据库迁移 0009_target_instances.sql"
echo "=========================================================="
PGPASSWORD=attackbox_pwd_9a7b12 psql -h 127.0.0.1 -U attackbox -d attackbox_db -f /home/laji/attackbox/web/server/db/migrations/0009_target_instances.sql || true

echo "=========================================================="
echo "  [4/6] 构建 Nuxt 3 生产包并同步至 /var/www/"
echo "=========================================================="
cd /home/laji/attackbox/web
npm install --no-audit --no-fund
npm run build

sudo rm -rf /var/www/attackbox-web
sudo mkdir -p /var/www/attackbox-web
sudo cp -r /home/laji/attackbox/web/.output /var/www/attackbox-web/
sudo cp -r /home/laji/attackbox/web/server /var/www/attackbox-web/
sudo chown -R laji:laji /var/www/attackbox-web

sudo rm -rf /var/www/attackbox
sudo mkdir -p /var/www/attackbox
sudo cp -a /var/www/attackbox-web/.output/public/. /var/www/attackbox/
sudo chown -R www-data:www-data /var/www/attackbox

echo "=========================================================="
echo "  [5/6] 配置并重载 systemd 服务 (Web & Worker)"
echo "=========================================================="
cat << 'EOF' | sudo tee /etc/systemd/system/attackbox-web.service > /dev/null
[Unit]
Description=AttackBox Nitro Web Service
After=network.target postgresql.service

[Service]
Type=simple
User=laji
WorkingDirectory=/var/www/attackbox-web/.output
Environment=NODE_ENV=production
Environment=PORT=3000
Environment=HOST=127.0.0.1
Environment=DATABASE_URL=postgres://attackbox:attackbox_pwd_9a7b12@127.0.0.1:5432/attackbox_db
Environment=SANDBOX_CONTROL_ENABLED=true
Environment=SANDBOX_SCHEDULING_ENABLED=true
Environment=SANDBOX_CLUSTER_SECRET=attackbox_cluster_secret_change_me_in_prod!
Environment=SANDBOX_SIGNING_SEED_HEX=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
Environment=SANDBOX_NODES_JSON=[{"id":"node-primary","label":"Primary\x20Node","publicDataUrl":"ws://127.0.0.1:8443","maxSlots":16}]
ExecStart=/usr/bin/node /var/www/attackbox-web/.output/server/index.mjs
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF

cat << 'EOF' | sudo tee /etc/systemd/system/attackbox-worker.service > /dev/null
[Unit]
Description=AttackBox Go Worker Service
After=network.target docker.service
Wants=docker.service

[Service]
Type=simple
User=root
WorkingDirectory=/home/laji/attackbox
ExecStart=/home/laji/attackbox/bin/node -config /home/laji/attackbox/configs/node.yaml
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl restart attackbox-web.service
sudo systemctl enable --now attackbox-worker.service
sudo systemctl restart attackbox-worker.service

sleep 4

echo "=========================================================="
echo "  [6/6] 检查服务在线状态与网络"
echo "=========================================================="
sudo systemctl is-active attackbox-web.service
sudo systemctl is-active attackbox-worker.service
sudo iptables -S DOCKER-USER || true
sudo docker network ls | grep -E 'targets|attackbox' || true
echo "部署与重启完成！"
