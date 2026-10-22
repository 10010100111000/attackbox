#!/usr/bin/env bash
set -e

REMOTE_BACKUP="/var/www/attackbox_legacy_backup"
REMOTE_ROOT="/var/www/attackbox"
ARCHIVE="/home/laji/web_dist_phase1.tar.gz"

if [ ! -f "$ARCHIVE" ]; then
    echo "[-] 归档文件不存在: $ARCHIVE"
    exit 1
fi

# 初始备份旧版前端
if [ ! -d "$REMOTE_BACKUP" ] && [ -d "$REMOTE_ROOT" ]; then
    echo ">>> 正在备份旧版前端到 $REMOTE_BACKUP..."
    sudo cp -r "$REMOTE_ROOT" "$REMOTE_BACKUP"
fi

# 解压到临时暂存目录
STAGING_DIR="/tmp/attackbox_web_staging_$$"
rm -rf "$STAGING_DIR"
mkdir -p "$STAGING_DIR"
tar -xzf "$ARCHIVE" -C "$STAGING_DIR"
rm -f "$ARCHIVE"

# 保留旧版 admin.html (如果存在且新包未包含)
if [ -f "$REMOTE_ROOT/admin.html" ] && [ ! -f "$STAGING_DIR/admin.html" ]; then
    cp "$REMOTE_ROOT/admin.html" "$STAGING_DIR/admin.html"
fi

# 同步文件到 Web 根目录
sudo mkdir -p "$REMOTE_ROOT"
sudo cp -r "$STAGING_DIR"/* "$REMOTE_ROOT"/
sudo rm -rf "$STAGING_DIR"

# 统一文件所有权与访问权限
sudo chown -R www-data:www-data "$REMOTE_ROOT"
sudo find "$REMOTE_ROOT" -type d -exec chmod 755 {} +
sudo find "$REMOTE_ROOT" -type f -exec chmod 644 {} +

# 验证 Nginx 配置并平滑重载
sudo nginx -t
sudo systemctl reload nginx || sudo nginx -s reload

echo ">>> 远端安装与 Nginx 重载完成！"
