#!/usr/bin/env bash
# AttackBox 独立编译脚本（编译与部署解耦）
# 用法: ./scripts/build.sh [all|control|worker]
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TARGET="${1:-all}"

build_control() {
    echo "==> [Build Control] 编译 Web 控制端 (web/ -> web/.output)..."
    cd "${REPO_ROOT}/web"
    npm ci --no-audit --no-fund
    npm run build
    test -f "${REPO_ROOT}/web/.output/server/index.mjs"
    echo "==> [Build Control] 完成: web/.output/server/index.mjs"
}

build_worker() {
    echo "==> [Build Worker] 编译 Go 工作节点守护进程 (worker/ -> dist/attackbox-worker)..."
    mkdir -p "${REPO_ROOT}/dist"
    cd "${REPO_ROOT}/worker"
    go build -trimpath -o "${REPO_ROOT}/dist/attackbox-worker" ./cmd/node
    chmod +x "${REPO_ROOT}/dist/attackbox-worker"
    echo "==> [Build Worker] 完成: dist/attackbox-worker"
}

case "${TARGET}" in
    all)
        build_control
        build_worker
        ;;
    control|web)
        build_control
        ;;
    worker|node)
        build_worker
        ;;
    *)
        echo "用法: $0 [all|control|worker]" >&2
        exit 1
        ;;
esac
