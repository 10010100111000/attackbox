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

# 检查并更新 Nginx 反向代理配置
echo "--> 更新 Nginx /api/auth/ 路由..."
if ! grep -q "location /api/auth/" /etc/nginx/sites-available/attackbox; then
    sudo python3 - << 'PYEOF'
with open("/etc/nginx/sites-available/attackbox", "r") as f:
    content = f.read()

auth_block = """    location /api/auth/ {
        proxy_pass http://127.0.0.1:3000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 60s;
        proxy_send_timeout 60s;
    }
"""

if "location /api/ {" in content:
    content = content.replace("location /api/ {", auth_block + "\n    location /api/ {", 1)
    with open("/etc/nginx/sites-available/attackbox", "w") as f:
        f.write(content)
print("Updated nginx configuration successfully.")
PYEOF
fi

sudo nginx -t
sudo systemctl reload nginx
echo "Nginx 路由重载完成。"

echo "--> 运行端到端测试套件 (E2E)..."
cd /var/www/attackbox-web
BASE_URL="http://127.0.0.1" node --test tests/auth-remote-e2e.mjs
echo "=========================================="
echo "Remote Deployment & Verification Complete"
echo "=========================================="
