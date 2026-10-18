#!/bin/bash
set -e
export PATH=/usr/local/go/bin:/usr/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/sbin:/bin

echo "=== [1] Extracting update_sync.tar.gz ==="
tar -xzf /home/laji/update_sync.tar.gz -C /home/laji/attackbox/

cd /home/laji/attackbox
rm -f frontend/src/components/AdminSettingsModal.vue frontend/src/views/AdminBroadcastView.vue

echo "=== [2] Compiling attackboxd ==="
go build -o bin/attackboxd ./cmd/attackboxd

echo "=== [3] Installing binary and web assets ==="
sudo install -m 755 bin/attackboxd /usr/local/bin/attackboxd
sudo mkdir -p /usr/local/share/attackbox/web
sudo cp -r web/* /usr/local/share/attackbox/web/

echo "=== [4] Restarting attackboxd.service ==="
sudo systemctl restart attackboxd.service
sleep 1
sudo systemctl status attackboxd.service --no-pager

echo "=== [5] Service Active Check ==="
systemctl is-active attackboxd
echo "=== Synchronization & Deployment Successful ==="
