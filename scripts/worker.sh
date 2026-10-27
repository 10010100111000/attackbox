#!/usr/bin/env bash
# Stable role entrypoint; configuration and deployment are implemented in Python.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
if [ "$(id -u)" -eq 0 ]; then
    exec python3 "${REPO_ROOT}/scripts/deploy.py" worker "$@"
fi
exec sudo python3 "${REPO_ROOT}/scripts/deploy.py" worker "$@"
