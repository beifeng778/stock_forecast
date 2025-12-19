#!/bin/bash

echo "[INFO][sample-gen] 生成 LLM 蒸馏样本(llm_samples.db)..."

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_DIR/sample-gen" || exit 1

mkdir -p "$PROJECT_DIR/rag"

go run ./main.go
