#!/usr/bin/env bash
set -e

echo "=========================================="
echo "Remote Deploying AttackBox Nitro Service"
echo "=========================================="

echo "--> 解包 Nitro 服务产物..."
sudo systemctl stop attackbox-web.service || true
sudo mkdir -p /var/www/attackbox-web
sudo rm -rf /var/www/attackbox-web/*
sudo tar -xzf /home/laji/web_nitro_dist.tar.gz -C /var/www/attackbox-web/
rm -f /home/laji/web_nitro_dist.tar.gz

# 拷贝静态资源到 Nginx 根目录
sudo cp -r /var/www/attackbox-web/.output/public/* /var/www/attackbox/
sudo chown -R www-data:www-data /var/www/attackbox

# 配置 systemd 服务单元
echo "--> 配置 systemd attackbox-web.service..."
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
ExecStart=/usr/bin/node /var/www/attackbox-web/.output/server/index.mjs
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable --now attackbox-web.service
sleep 2

if ! sudo systemctl is-active --quiet attackbox-web.service; then
    echo "ERROR: attackbox-web.service 启动失败！日志如下："
    sudo journalctl -u attackbox-web.service --no-pager -n 20
    exit 1
fi
echo "attackbox-web.service 正在运行中"

# 验证 Nginx 配置并平滑重载
sudo /usr/sbin/nginx -t
sudo systemctl reload nginx
echo "Nginx 路由检测与重载完成。"

# 运行端到端测试套件 (E2E)
if [ -f /var/www/attackbox-web/tests/auth-remote-e2e.mjs ]; then
    echo "--> 运行端到端认证测试套件 (auth E2E)..."
    cd /var/www/attackbox-web
    BASE_URL="http://127.0.0.1" node --test tests/auth-remote-e2e.mjs
fi

echo "=========================================="
echo "Remote Deployment & Verification Complete"
echo "=========================================="
