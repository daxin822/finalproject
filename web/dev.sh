#!/usr/bin/env bash
# 绕过损坏的系统 npm，直接用 node 启动 Vite
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
NODE_BIN="${NODE_BIN:-node}"

if ! command -v "$NODE_BIN" >/dev/null 2>&1; then
  CURSOR_NODE="/home/zhangyingxin/.cursor-server/bin/linux-arm64/38a27120cfc7419a5efa38420665eaeeed1e7b30/node"
  if [[ -x "$CURSOR_NODE" ]]; then
    NODE_BIN="$CURSOR_NODE"
  else
    echo "未找到 node，请先 export PATH 或设置 NODE_BIN" >&2
    exit 1
  fi
fi

exec "$NODE_BIN" "$ROOT/node_modules/vite/bin/vite.js" "$@"
