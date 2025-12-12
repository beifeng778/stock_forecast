#!/bin/bash

echo "启动 Python 数据服务..."

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR/data-service"

# 检查虚拟环境
if [ -d "venv" ]; then
    source venv/Scripts/activate 2>/dev/null || source venv/bin/activate 2>/dev/null
fi

python -m app.main
