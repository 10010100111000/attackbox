#!/usr/bin/env bash
# AttackBox Control (Web 控制端 + Nginx) 部署脚本
# 前置条件:
#   1. 已执行 ./scripts/build.sh control 生成 web/.output
#   2. 已执行 sudo ./scripts/setup-control.py 生成 /etc/attackbox/web.env
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="/etc/attackbox/web.env"
WEB_ROOT="/var/www/attackbox-web"
STAGE_ROOT="/var/www/.attackbox-web-stage.$$"
PREV_ROOT="/var/www/.attackbox-web-prev.$$"

cleanup_stage() {
    sudo rm -rf "${STAGE_ROOT}"
}
trap cleanup_stage EXIT

if ! sudo test -f "${ENV_FILE}"; then
    echo "错误: 缺少 ${ENV_FILE}，请先运行: sudo ./scripts/setup-control.py" >&2
    exit 1
fi

if [[ ! -f "${REPO_ROOT}/web/.output/server/index.mjs" ]]; then
    echo "错误: 缺少编译产物 web/.output/server/index.mjs，请先运行: ./scripts/build.sh control" >&2
    exit 1
fi

# 读取 HOST 与 PORT
WEB_HOST="$(sudo awk -F= '/^HOST=/{print $2}' "${ENV_FILE}" | tail -n1 | tr -d '[:space:]')"
WEB_PORT="$(sudo awk -F= '/^PORT=/{print $2}' "${ENV_FILE}" | tail -n1 | tr -d '[:space:]')"
WEB_HOST="${WEB_HOST:-127.0.0.1}"
WEB_PORT="${WEB_PORT:-3000}"
WEB_UPSTREAM="${WEB_HOST}:${WEB_PORT}"

echo "==> [Deploy Control] 准备运行账户与持久化数据目录..."
if ! id -u attackbox >/dev/null 2>&1; then
    sudo useradd --system --home-dir /var/lib/attackbox-web --create-home --shell /usr/sbin/nologin attackbox
fi
sudo install -d -m 0750 -o root -g attackbox /etc/attackbox
sudo chown root:attackbox "${ENV_FILE}"
sudo chmod 0640 "${ENV_FILE}"
sudo install -d -m 0700 -o attackbox -g attackbox \
    /var/lib/attackbox-web \
    /var/lib/attackbox-web/content \
    /var/lib/attackbox-web/labs

echo "==> [Deploy Control] 发布 web/.output -> ${WEB_ROOT}/.output..."
sudo rm -rf "${STAGE_ROOT}" "${PREV_ROOT}"
sudo mkdir -p "${STAGE_ROOT}"
sudo cp -a "${REPO_ROOT}/web/.output" "${STAGE_ROOT}/.output"
sudo chown -R root:root "${STAGE_ROOT}"

echo "==> [Deploy Control] 渲染 Nginx 与 systemd 单元..."
sed "s|@@WEB_UPSTREAM@@|${WEB_UPSTREAM}|g" "${REPO_ROOT}/deploy/control/nginx.conf.template" \
    | sudo tee /etc/nginx/sites-available/attackbox >/dev/null
sudo ln -sfn /etc/nginx/sites-available/attackbox /etc/nginx/sites-enabled/attackbox
sudo rm -f /etc/nginx/sites-enabled/default
sudo nginx -t

sudo cp "${REPO_ROOT}/deploy/control/attackbox-web.service" /etc/systemd/system/attackbox-web.service
sudo systemctl daemon-reload

if sudo test -d "${WEB_ROOT}"; then
    sudo mv "${WEB_ROOT}" "${PREV_ROOT}"
fi
sudo mv "${STAGE_ROOT}" "${WEB_ROOT}"
sudo rm -rf "${PREV_ROOT}"

echo "==> [Deploy Control] 启动 attackbox-web 与 nginx..."
sudo systemctl enable attackbox-web nginx >/dev/null
sudo systemctl restart attackbox-web
sudo systemctl reload nginx

echo "==> [Deploy Control] 执行健康检查 (http://${WEB_UPSTREAM}/api/health)..."
for _ in $(seq 1 30); do
    if curl --fail --silent --show-error --max-time 2 "http://${WEB_UPSTREAM}/api/health" >/dev/null; then
        echo "==> [Deploy Control] 部署成功，服务运行正常。"
        exit 0
    fi
    sleep 1
done

echo "错误: attackbox-web 健康检查超时，请查看: sudo journalctl -u attackbox-web -n 50" >&2
exit 1
