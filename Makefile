.PHONY: proto deps build run clean

# 生成protobuf代码
proto:
	protoc --go_out=. --go_opt=paths=source_relative \
	       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
	       proto/rocketmq.proto

# 下载依赖
deps:
	go mod tidy
	go mod download

# 构建
build: proto deps
	go build -o bin/rocketmq-proxy server/main.go

# 运行
run: build
	./bin/rocketmq-proxy

# 清理
clean:
	rm -rf bin/
	rm -f proto/*.pb.go

# 初始化 (第一次运行)
init: deps proto 