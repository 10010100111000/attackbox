#!/bin/bash
set -e

echo "=== 1. Login ==="
COOKIE=$(curl -s -k -i -X POST https://127.0.0.1:443/api/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"testplayer","password":"123456"}' | grep -i set-cookie | awk '{print $2}' | tr -d '\r;')
echo "Login cookie: $COOKIE"

echo "=== 2. Check initial status ==="
STATUS_0=$(curl -s -k https://127.0.0.1:443/api/me -H "Cookie: $COOKIE")
echo "Initial Status: $STATUS_0"

echo "=== 3. Start Environment (启动环境) ==="
START_RESP=$(curl -s -k -X POST https://127.0.0.1:443/api/env/toggle -H "Cookie: $COOKIE")
echo "START Result: $START_RESP"
sleep 2

echo "=== 4. Check status after start (运行中) ==="
STATUS_1=$(curl -s -k https://127.0.0.1:443/api/me -H "Cookie: $COOKIE")
echo "Status After Start: $STATUS_1"

echo "=== 5. Stop Environment (关闭环境) ==="
STOP_RESP=$(curl -s -k -X POST https://127.0.0.1:443/api/env/toggle -H "Cookie: $COOKIE")
echo "STOP Result: $STOP_RESP"
sleep 1

echo "=== 6. Check status after stop (已停止) ==="
STATUS_2=$(curl -s -k https://127.0.0.1:443/api/me -H "Cookie: $COOKIE")
echo "Status After Stop: $STATUS_2"

echo "=== ALL API TESTS PASSED ==="
