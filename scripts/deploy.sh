#!/usr/bin/env bash
# Canonical single-server deploy: Go worker + Nuxt web.
set -euo pipefail

REPO="${REPO:-/home/laji/attackbox}"
BRANCH="${BRANCH:-main}"
WEB_ROOT="/var/www/attackbox-web"
ENV_FILE="/etc/attackbox/web.env"

log() { printf '\n==> %s\n' "$*"; }
die() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }

cd "$REPO"
[ -f go.mod ] || die "run from the repo root"
[ -f "$ENV_FILE" ] || die "missing $ENV_FILE (see deploy/web.env.example)"
[ -r configs/node.yaml ] || die "missing or unreadable configs/node.yaml (copy configs/node.yaml.example and configure it)"
[ -z "$(git status --porcelain)" ] || die "working tree is dirty; commit or reset before deploying"

log "sync $BRANCH"
git fetch origin "$BRANCH"
git merge --ff-only "origin/$BRANCH"

log "build worker"
mkdir -p bin
go build -trimpath -o bin/node ./cmd/node
go build -trimpath -o bin/deploy-config ./cmd/deploy-config

log "render deployment addresses"
deploy_config_dir="$(mktemp -d)"
trap 'rm -rf "$deploy_config_dir"' EXIT
bin/deploy-config -env "$ENV_FILE" -config configs/node.yaml -out "$deploy_config_dir"
web_health_url="$(cat "$deploy_config_dir/web-health")"
worker_health_url="$(cat "$deploy_config_dir/worker-health")"

log "build web"
(cd web && npm ci --no-audit --no-fund && npm run build)

log "install systemd units + nginx"
sudo install -m 0644 deploy/systemd/attackbox-web.service /etc/systemd/system/attackbox-web.service
sudo install -m 0644 deploy/systemd/attackbox-worker.service /etc/systemd/system/attackbox-worker.service
sudo install -m 0644 "$deploy_config_dir/nginx.conf" /etc/nginx/sites-available/attackbox
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
  if curl --connect-timeout 3 --max-time 5 -fsS "$web_health_url" >/dev/null 2>&1; then break; fi
  sleep 1
done
curl --connect-timeout 3 --max-time 5 -fsS "$web_health_url" >/dev/null || die "web health check failed"
curl --connect-timeout 3 --max-time 5 -fsS "$worker_health_url" >/dev/null || die "worker gateway health check failed"
sleep 2
sudo systemctl is-active --quiet attackbox-web || die "attackbox-web is not active"
sudo systemctl is-active --quiet attackbox-worker || die "attackbox-worker is not active"
worker_pid="$(sudo systemctl show -p MainPID --value attackbox-worker)"
sudo journalctl _PID="$worker_pid" --since "2 min ago" --no-pager | grep -q "Registration acknowledged" \
  || die "worker did not register with the hub"
sudo systemctl is-active --quiet nginx || die "nginx is not active"

log "deployed $(git rev-parse --short HEAD)"