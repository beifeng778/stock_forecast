#!/bin/bash

echo "正在停止所有服务..."

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# 从 PID 文件读取并终止进程
if [ -f "$SCRIPT_DIR/.pids" ]; then
    while read pid; do
        if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
            echo "停止进程 $pid"
            kill "$pid" 2>/dev/null
        fi
    done < "$SCRIPT_DIR/.pids"
    rm -f "$SCRIPT_DIR/.pids"
fi

# 额外清理：按端口杀进程
echo "清理残留进程..."

# 杀掉占用 8080 端口的进程 (Go)
lsof -ti:8080 | xargs -r kill 2>/dev/null

# 杀掉占用 5173 端口的进程 (Vite)
lsof -ti:5173 | xargs -r kill 2>/dev/null

echo "所有服务已停止。"
