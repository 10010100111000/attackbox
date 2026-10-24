#!/usr/bin/env bash
# Apply Drizzle migrations. Run this only when a release adds migrations.
set -euo pipefail

REPO="$(cd "$(dirname "$0")/.." && pwd)"
ENV_FILE="${ENV_FILE:-/etc/attackbox/web.env}"

if [ -z "${DATABASE_URL:-}" ]; then
  DATABASE_URL="$(grep -E '^DATABASE_URL=' "$ENV_FILE" | cut -d= -f2-)"
fi
[ -n "$DATABASE_URL" ] || { echo "DATABASE_URL is required" >&2; exit 1; }

cd "$REPO/web"
DATABASE_URL="$DATABASE_URL" npm run db:migrate