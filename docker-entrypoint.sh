#!/bin/sh

set -e

# 打印启动信息
echo "🚀 Starting RocketMQ Proxy..."
echo "Time: $(date)"

# 检查二进制文件是否存在
if [ ! -f "./rocketmq-proxy" ]; then
    echo "❌ Error: rocketmq-proxy binary not found!"
    exit 1
fi

# 启动应用
echo "🎯 Starting RocketMQ Proxy Server..."
exec ./rocketmq-proxy 