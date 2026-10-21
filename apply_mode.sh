#!/usr/bin/env bash
set -e

cd /home/laji/attackbox
tar -xzf /home/laji/mode_update.tar.gz
rm -f /home/laji/mode_update.tar.gz

echo "=== Building Control ==="
go build -o bin/control ./cmd/control

echo "=== Restarting Control Service ==="
sudo pkill -f "bin/control" || true
sleep 1
sudo nohup /home/laji/attackbox/bin/control -config /home/laji/attackbox/configs/control.yaml >/tmp/control.log 2>&1 &
sleep 2

ps aux | grep bin/control | grep -v grep
echo "=== Success ==="
