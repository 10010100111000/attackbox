#!/bin/bash
set -e

CONF_FILE="/etc/nginx/sites-available/attackbox"

if grep -q "location /api/modules/" "$CONF_FILE"; then
    echo "location /api/modules/ already exists in $CONF_FILE"
else
    cat << 'EOF' > /tmp/modules_block.txt
    location /api/modules/ {
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

EOF
    # 在 location /api/ { 之前插入
    sed -i '/location \/api\/ {/e cat /tmp/modules_block.txt' "$CONF_FILE"
    echo "Added location /api/modules/ to $CONF_FILE"
fi

nginx -t
systemctl reload nginx
