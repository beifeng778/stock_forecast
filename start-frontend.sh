#!/bin/bash

echo "启动前端开发服务器..."

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR/frontend"

npm run dev
