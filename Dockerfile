# 构建阶段
FROM golang:1.23-alpine AS builder

# 安装必要的工具
RUN apk add --no-cache git protobuf protobuf-dev

# 设置工作目录
WORKDIR /app

# 复制go mod文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用（proto文件已经生成，直接构建）
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/rocketmq-proxy server/main.go

# 运行阶段
FROM alpine:latest

# 安装必要的包
RUN apk --no-cache add ca-certificates tzdata && \
    addgroup -g 1001 -S rocketmq && \
    adduser -u 1001 -D -S -G rocketmq rocketmq

# 设置时区
ENV TZ=Asia/Shanghai

WORKDIR /home/rocketmq

# 从构建阶段复制二进制文件和启动脚本
COPY --from=builder /app/bin/rocketmq-proxy .
COPY --chmod=755 docker-entrypoint.sh .

# 更改文件所有者
RUN chown rocketmq:rocketmq rocketmq-proxy docker-entrypoint.sh

# 切换到非root用户
USER rocketmq

# 暴露端口
EXPOSE 50051 8080

# 运行应用
CMD ["sh", "./docker-entrypoint.sh"] 