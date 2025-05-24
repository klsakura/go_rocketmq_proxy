#!/bin/bash

echo "🔍 RocketMQ Proxy 内存泄漏检测工具"
echo "=================================="

# 检查服务是否运行
echo "📡 检查服务状态..."
if ! curl -s http://localhost:8080/health > /dev/null; then
    echo "❌ 服务未运行，请先启动 RocketMQ Proxy 服务"
    echo "启动命令: cd server && go run main.go"
    exit 1
fi

echo "✅ 服务正在运行"

# 检查初始状态
echo "📊 获取初始 metrics..."
curl -s http://localhost:8080/metrics | jq '.' || echo "⚠️ 无法获取 metrics，请确保服务正常运行"

echo ""
echo "🚀 开始内存泄漏测试..."
echo "注意: 测试大约需要 2-3 分钟完成"
echo ""

# 运行测试
cd "$(dirname "$0")/.."
go run scripts/memory_leak_test.go

# 获取最终状态
echo ""
echo "📊 获取最终 metrics..."
curl -s http://localhost:8080/metrics | jq '.' || echo "⚠️ 无法获取最终 metrics"

echo ""
echo "✅ 测试完成！请查看详细分析报告: MEMORY_LEAK_ANALYSIS.md" 