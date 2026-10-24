#!/usr/bin/env bash
# Canonical single-server deploy: Go worker + Nuxt web.
# Retired: the old Go control (cmd/control, control/, cmd/attackboxd) and the old
# Vue SPA (frontend/) are archived under backup/, not built or installed.
set -euo pipefail

REPO="${REPO:-/home/laji/attackbox}"
BRANCH="${BRANCH:-main}"
WEB_ROOT="/var/www/attackbox-web"
ENV_FILE="${ENV_FILE:-/etc/attackbox/web.env}"

log() { printf '\n==> %s\n' "$*"; }
die() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }

cd "$REPO"
[ -f go.mod ] || die "run from the repo root"
[ -f "$ENV_FILE" ] || die "missing $ENV_FILE (see deploy/web.env.example)"
[ -z "$(git status --porcelain)" ] || die "working tree is dirty; commit or reset before deploying"

# The legacy oneshot unit creates the same nft table as the worker and can wipe it.
if systemctl is-enabled --quiet attackbox-fw.service 2>/dev/null; then
  die "legacy attackbox-fw.service is still enabled; remove it before deploying"
fi

log "sync $BRANCH"
git fetch origin "$BRANCH"
git merge --ff-only "origin/$BRANCH"

log "build worker"
mkdir -p bin
go build -trimpath -o bin/node ./cmd/node

log "build web"
(cd web && npm ci --no-audit --no-fund && npm run build)

log "install systemd units + nginx"
sudo install -m 0644 deploy/systemd/attackbox-web.service /etc/systemd/system/attackbox-web.service
sudo install -m 0644 deploy/systemd/attackbox-worker.service /etc/systemd/system/attackbox-worker.service
sudo install -m 0644 deploy/nginx/attackbox.conf /etc/nginx/sites-available/attackbox
sudo ln -sfn /etc/nginx/sites-available/attackbox /etc/nginx/sites-enabled/attackbox
sudo nginx -t
sudo systemctl daemon-reload

log "publish web build (previous kept as $WEB_ROOT.prev)"
sudo rm -rf "$WEB_ROOT.new"
sudo mkdir -p "$WEB_ROOT.new"
sudo cp -a web/.output "$WEB_ROOT.new/"
sudo chown -R laji:laji "$WEB_ROOT.new"
sudo systemctl stop attackbox-web
sudo rm -rf "$WEB_ROOT.prev"
if [ -d "$WEB_ROOT" ]; then sudo mv "$WEB_ROOT" "$WEB_ROOT.prev"; fi
sudo mv "$WEB_ROOT.new" "$WEB_ROOT"

log "restart services (this destroys running sandboxes)"
sudo systemctl enable attackbox-web attackbox-worker >/dev/null
sudo systemctl restart attackbox-web
sudo systemctl restart attackbox-worker
sudo systemctl reload nginx

log "health checks"
for _ in $(seq 1 20); do
  if curl -fsS http://127.0.0.1:3000/ >/dev/null 2>&1; then break; fi
  sleep 1
done
curl -fsS http://127.0.0.1:3000/ >/dev/null || die "web is not answering on :3000"
curl -fsS http://127.0.0.1:8443/health >/dev/null || die "worker gateway is not answering on :8443"
sleep 2
sudo systemctl is-active --quiet attackbox-web || die "attackbox-web is not active"
sudo systemctl is-active --quiet attackbox-worker || die "attackbox-worker is not active"
worker_pid="$(sudo systemctl show -p MainPID --value attackbox-worker)"
sudo journalctl _PID="$worker_pid" --since "2 min ago" --no-pager | grep -q "Registration acknowledged" \
  || die "worker did not register with the hub"
curl -fsS -o /dev/null http://127.0.0.1/ || die "nginx is not serving the site"

log "deployed $(git rev-parse --short HEAD)"