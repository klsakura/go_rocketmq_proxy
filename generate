#!/bin/bash

# 确保proto目录存在
mkdir -p proto

# 生成Go代码
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       proto/rocketmq.proto

echo "Protobuf code generated successfully!" 