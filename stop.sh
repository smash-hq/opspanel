#!/bin/bash
# Server Plane 停止脚本
# 用法: ./stop.sh [端口]  (默认 18080)

PORT=${1:-18080}

EXISTING=$(lsof -ti:$PORT 2>/dev/null)
if [ -n "$EXISTING" ]; then
    echo "==> 停止服务 (端口 $PORT, PID: $EXISTING)"
    echo "$EXISTING" | xargs kill -9 2>/dev/null
    echo "==> 已停止"
else
    echo "==> 端口 $PORT 上没有运行的服务"
fi

# 也清理可能残留的编译产物
rm -f /tmp/smp-server 2>/dev/null
