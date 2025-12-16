#!/bin/bash

echo "启动 Golang 后端服务..."

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_DIR/backend"

# 加载环境变量
if [ -f ".env" ]; then
    export $(cat .env | grep -v '^#' | xargs)
fi

mkdir -p ../rag
LLM_SAMPLES_PATH=../rag go run cmd/server/main.go
