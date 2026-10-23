#!/usr/bin/env bash
set -e

echo "=== AttackBox Web Application Status Verification ==="

# 1. 检查 systemd 服务状态
echo -n "[1] Checking systemd service: "
systemctl is-active attackbox-web.service

# 2. 检查公开 API
echo -n "[2] Checking /api/categories: "
CAT_RES=$(curl -s http://127.0.0.1/api/categories)
echo "$CAT_RES" | grep -q '"ok":true' && echo "OK (found categories)" || (echo "FAIL: $CAT_RES" && exit 1)

echo -n "[3] Checking /api/modules: "
MOD_RES=$(curl -s http://127.0.0.1/api/modules)
echo "$MOD_RES" | grep -q '"ok":true' && echo "OK (found modules)" || (echo "FAIL: $MOD_RES" && exit 1)

# 3. 登录并检查管理端 API
echo -n "[4] Testing admin login: "
cat << 'EOF' > /tmp/admin_login_req.json
{"email":"admin@attackbox.org","password":"AttackBoxAdmin2026!"}
EOF

LOGIN_RES=$(curl -s -c /tmp/admin_test_cookie.txt -X POST http://127.0.0.1/api/auth/login \
  -H "Content-Type: application/json" \
  -d @/tmp/admin_login_req.json)
rm -f /tmp/admin_login_req.json

if echo "$LOGIN_RES" | grep -q '"ok":true'; then
    echo "OK (Admin logged in)"
else
    echo "WARN: Admin login output: $LOGIN_RES"
fi

echo -n "[5] Testing /api/admin/modules: "
ADMIN_MODS=$(curl -s -b /tmp/admin_test_cookie.txt http://127.0.0.1/api/admin/modules)
rm -f /tmp/admin_test_cookie.txt
if echo "$ADMIN_MODS" | grep -q '"ok":true'; then
    echo "OK (Admin modules API working)"
else
    echo "WARN: $ADMIN_MODS"
fi

echo "=== All Remote App Verifications Completed ==="
