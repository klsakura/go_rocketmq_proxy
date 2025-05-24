#!/bin/bash

echo "Starting RocketMQ gRPC Proxy..."

# 启用Go模块
export GO111MODULE=on

# 获取Go环境信息
GOPATH=$(go env GOPATH)
GOROOT=$(go env GOROOT)

# 确保PATH包含Go bin目录
export PATH=$GOROOT/bin:$GOPATH/bin:$PATH

echo "Go PATH: $GOPATH"
echo "Go MODULE: $(go env GO111MODULE)"

# 检查protoc是否可用
if ! command -v protoc &> /dev/null; then
    echo "protoc not found! Please install protobuf compiler:"
    echo "  macOS: brew install protobuf"
    echo "  Ubuntu: sudo apt install protobuf-compiler"
    exit 1
fi

echo "protoc version: $(protoc --version)"

# 检查并安装protoc插件
echo "Checking protoc-gen-go..."
if ! command -v protoc-gen-go &> /dev/null; then
    echo "Installing protoc-gen-go..."
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
    if [ $? -ne 0 ]; then
        echo "Failed to install protoc-gen-go"
        exit 1
    fi
fi

echo "Checking protoc-gen-go-grpc..."
if ! command -v protoc-gen-go-grpc &> /dev/null; then
    echo "Installing protoc-gen-go-grpc..."
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
    if [ $? -ne 0 ]; then
        echo "Failed to install protoc-gen-go-grpc"
        exit 1
    fi
fi

# 再次检查插件是否可用
if command -v protoc-gen-go &> /dev/null; then
    echo "protoc-gen-go version: $(protoc-gen-go --version)"
else
    echo "protoc-gen-go still not found after installation"
    exit 1
fi

if command -v protoc-gen-go-grpc &> /dev/null; then
    echo "protoc-gen-go-grpc: OK"
else
    echo "protoc-gen-go-grpc still not found after installation"
    exit 1
fi

# 构建项目
echo "Building project..."
make build

if [ $? -ne 0 ]; then
    echo "Build failed!"
    exit 1
fi

# 启动服务
echo "Starting server on :50051..."
./bin/rocketmq-proxy 