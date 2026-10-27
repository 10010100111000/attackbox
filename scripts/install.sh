#!/usr/bin/env bash
# 一键全新编译、自动化配置与全量部署脚本
set -euo pipefail

REPO="${REPO:-/home/laji/attackbox}"
DATABASE_URL="${DATABASE_URL:-postgres://attackbox:attackbox_pwd_9a7b12@127.0.0.1:5432/attackbox_db}"
ORIGIN="${ORIGIN:-http://77.67.1.106}"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@attackbox.local}"
ADMIN_NAME="${ADMIN_NAME:-Administrator}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-AttackBox2026!Admin}"

export PATH="/home/laji/node22/bin:/usr/local/bin:/usr/bin:/bin"

log() { printf '\n==> %s\n' "$*"; }

cd "$REPO"

log "1. 编译构建阶段 (先让所有产物就绪)"
mkdir -p bin
go build -trimpath -o bin/node ./cmd/node
go build -trimpath -o bin/deploy-config ./cmd/deploy-config

(cd web && npm ci --no-audit --no-fund && npm run build)

log "2. 自动化配置阶段 (清理旧配置并安全生成全新密钥)"
sudo python3 scripts/setup.py \
  --database "$DATABASE_URL" \
  --origin "$ORIGIN" \
  --force

log "3. 数据库迁移与安全初始化管理员"
sudo python3 scripts/manage.py migrate
sudo python3 scripts/manage.py admin \
  --email "$ADMIN_EMAIL" \
  --name "$ADMIN_NAME" \
  --password "$ADMIN_PASSWORD"

log "4. 生产服务部署与启动"
./scripts/deploy.sh

log "全新安装与部署全部完成！"
