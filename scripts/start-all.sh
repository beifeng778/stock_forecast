#!/bin/bash

echo "========================================"
echo "   股票预测系统 - 启动脚本"
echo "========================================"
echo ""

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# 启动 Python 数据服务
echo "[1/3] 启动 Python 数据服务..."
cd "$SCRIPT_DIR/data-service"
python -m app.main &
PYTHON_PID=$!
echo "Python 数据服务 PID: $PYTHON_PID"

sleep 3

# 启动 Golang 后端服务
echo "[2/3] 启动 Golang 后端服务..."
cd "$SCRIPT_DIR/backend"
go run cmd/server/main.go &
GO_PID=$!
echo "Golang 后端 PID: $GO_PID"

sleep 2

# 启动前端开发服务器
echo "[3/3] 启动前端开发服务器..."
cd "$SCRIPT_DIR/frontend"
npm run dev &
FRONTEND_PID=$!
echo "前端服务 PID: $FRONTEND_PID"

echo ""
echo "========================================"
echo "所有服务已启动！"
echo ""
echo "- Python 数据服务: http://localhost:5000"
echo "- Golang 后端:     http://localhost:8080"
echo "- 前端界面:        http://localhost:5173"
echo "========================================"
echo ""
echo "按 Ctrl+C 停止所有服务"

# 保存 PID 到文件
echo "$PYTHON_PID" > "$SCRIPT_DIR/.pids"
echo "$GO_PID" >> "$SCRIPT_DIR/.pids"
echo "$FRONTEND_PID" >> "$SCRIPT_DIR/.pids"

# 等待任意子进程结束
wait
