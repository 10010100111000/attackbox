#!/usr/bin/env bash
# Explicit database migration using the same configuration as the service.
set -euo pipefail
exec python3 "$(dirname "$0")/manage.py" migrate
