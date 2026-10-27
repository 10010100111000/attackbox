#!/usr/bin/env bash
# AttackBox Worker (Go 容器沙箱节点) 部署脚本
# 前置条件:
#   1. 已执行 ./scripts/build.sh worker 生成 dist/attackbox-worker
#   2. 已执行 sudo ./scripts/setup-worker.py 生成 /etc/attackbox/worker.yaml
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORKER_CONFIG="/etc/attackbox/worker.yaml"
WORKER_BIN="/usr/local/bin/attackbox-worker"

if ! sudo test -f "${WORKER_CONFIG}"; then
    echo "错误: 缺少 ${WORKER_CONFIG}，请先运行: sudo ./scripts/setup-worker.py" >&2
    exit 1
fi

if [[ ! -f "${REPO_ROOT}/dist/attackbox-worker" ]]; then
    echo "错误: 缺少编译产物 dist/attackbox-worker，请先运行: ./scripts/build.sh worker" >&2
    exit 1
fi

echo "==> [Deploy Worker] 准备运行目录与安装二进制..."
sudo install -d -m 0755 /var/lib/attackbox /var/lib/attackbox/instances /var/lib/attackbox/compose-labs
sudo install -m 0755 -o root -g root "${REPO_ROOT}/dist/attackbox-worker" "${WORKER_BIN}"

echo "==> [Deploy Worker] 安装并启动 attackbox-worker.service..."
sudo cp "${REPO_ROOT}/deploy/worker/attackbox-worker.service" /etc/systemd/system/attackbox-worker.service
sudo systemctl daemon-reload
sudo systemctl enable attackbox-worker >/dev/null
sudo systemctl restart attackbox-worker

LISTEN_ADDR="$(sudo awk -F'"' '/listen_addr:/{print $2}' "${WORKER_CONFIG}" | tail -n1)"
LISTEN_PORT="${LISTEN_ADDR##*:}"
LISTEN_PORT="${LISTEN_PORT:-8443}"

echo "==> [Deploy Worker] 执行健康检查 (http://127.0.0.1:${LISTEN_PORT}/healthz)..."
for _ in $(seq 1 30); do
    if curl --fail --silent --show-error --max-time 2 "http://127.0.0.1:${LISTEN_PORT}/healthz" >/dev/null; then
        echo "==> [Deploy Worker] 部署成功，Worker 守护进程运行正常。"
        exit 0
    fi
    sleep 1
done

echo "错误: attackbox-worker 健康检查超时，请查看: sudo journalctl -u attackbox-worker -n 50" >&2
exit 1
