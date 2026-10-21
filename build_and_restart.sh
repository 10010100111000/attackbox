#!/usr/bin/env bash
set -e

cd /home/laji/attackbox
tar -xzf /home/laji/control_patch.tar.gz
rm -f /home/laji/control_patch.tar.gz

echo "=== Testing Go compilation ==="
go build -o bin/control ./cmd/control

echo "=== Restarting control service ==="
sudo pkill -f "bin/control" || true
sleep 1
sudo nohup /home/laji/attackbox/bin/control -config /home/laji/attackbox/configs/control.yaml >/tmp/control.log 2>&1 &
sleep 2

ps aux | grep bin/control | grep -v grep
echo "=== Success ==="
