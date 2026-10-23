#!/usr/bin/env bash
set -euo pipefail
# Run on the VPS with the application stopped. WEB_DIR is the source web/ directory.
: "${WEB_DIR:?Set WEB_DIR to the source web directory}"
: "${DATABASE_URL:?Set DATABASE_URL for the database to reset}"
: "${RESET_DATABASE:?Set RESET_DATABASE to the target database name; all data will be erased}"
cd "$WEB_DIR"
npm run db:reset
echo 'Database initialized. Run npm run admin:init -- <email> <password> before restarting the application.'
