#!/bin/bash
# Server Plane 启动脚本
# 用法: ./start.sh [端口]  (默认 18080)

PORT=${1:-18080}
cd "$(dirname "$0")"

echo "==> Server Plane"

# 清理残留进程
EXISTING=$(lsof -ti:$PORT 2>/dev/null)
if [ -n "$EXISTING" ]; then
    echo "==> 清理端口 $PORT 残留进程"
    echo "$EXISTING" | xargs kill -9 2>/dev/null
    sleep 1
fi

# 编译
echo "==> 编译..."
go build -o /tmp/smp-server ./cmd/server/ || { echo "编译失败"; exit 1; }

# 启动（通过环境变量设置端口）
echo "==> 启动服务: http://localhost:$PORT"
SMP_SERVER_PORT=$PORT /tmp/smp-server
