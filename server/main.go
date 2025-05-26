package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"

	"go_rocketmq_sdk/proto"
	"go_rocketmq_sdk/server/config"
	"go_rocketmq_sdk/server/metrics"
	"go_rocketmq_sdk/server/service"

	"github.com/apache/rocketmq-client-go/v2/rlog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	// 加载配置
	cfg := config.DefaultConfig()

	// 设置RocketMQ SDK日志级别，减少offset更新等INFO级别的噪音日志
	// 可选值: "debug", "info", "warn", "error", "fatal"
	rlog.SetLogLevel(cfg.LogLevel)

	log.Printf("🚀 Starting RocketMQ Proxy Server with config:")
	log.Printf("   - Port: %s", cfg.Port)
	log.Printf("   - Max Concurrent: %d", cfg.MaxConcurrent)
	log.Printf("   - Producer Pool: %d", cfg.ProducerPoolSize)
	log.Printf("   - Consumer Pool: %d", cfg.ConsumerPoolSize)
	log.Printf("   - Message Buffer: %d", cfg.MessageBufferSize)
	log.Printf("   - Worker Pool: %d", cfg.WorkerPoolSize)
	log.Printf("   - RocketMQ Log Level: %s (to reduce offset update noise)", cfg.LogLevel)

	// 启动性能监控
	metrics.GlobalMetrics.StartPeriodicUpdate()

	// 启动指标HTTP服务器
	if cfg.EnableMetrics {
		go startMetricsServer(cfg.MetricsPort)
	}

	// 启动资源监控
	go startResourceMonitor()

	// 创建服务实例
	rocketmqService := service.NewRocketMQProxyService(cfg)

	// 启动消费者清理定时任务
	go startConsumerCleanupTask(rocketmqService)

	// 启动生产者清理定时任务
	go startProducerCleanupTask(rocketmqService)

	// 监听端口
	lis, err := net.Listen("tcp", ":"+cfg.Port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// 创建gRPC服务器配置
	opts := []grpc.ServerOption{
		grpc.MaxConcurrentStreams(uint32(cfg.MaxConcurrent)),
		grpc.ConnectionTimeout(cfg.ReadTimeout),
		grpc.MaxRecvMsgSize(4 * 1024 * 1024), // 4MB
		grpc.MaxSendMsgSize(4 * 1024 * 1024), // 4MB
		grpc.UnaryInterceptor(metricsInterceptor),
	}

	// 创建gRPC服务器
	s := grpc.NewServer(opts...)

	// 注册服务
	proto.RegisterRocketMQProxyServer(s, rocketmqService)

	// 启用反射服务（用于调试）
	reflection.Register(s)

	log.Printf("🎯 RocketMQ Proxy Server starting on :%s", cfg.Port)
	log.Printf("📊 Metrics available at http://localhost:%s/metrics", cfg.MetricsPort)
	log.Printf("🔧 Performance tuning parameters:")
	log.Printf("   - Set MAX_CONCURRENT=%d for max concurrent connections", cfg.MaxConcurrent)
	log.Printf("   - Set WORKER_POOL_SIZE=%d for worker goroutines", cfg.WorkerPoolSize)
	log.Printf("   - Set MESSAGE_BUFFER_SIZE=%d for message buffering", cfg.MessageBufferSize)
	log.Printf("🧹 Consumer cleanup enabled: inactive consumers will be cleaned up after 10 minutes")
	log.Printf("🔄 Supports predefined consumer groups from 字节云 with auto-reconnection")

	// 添加信号处理机制，在服务停止时优雅关闭所有生产者和消费者
	go func() {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
		<-signals

		log.Println("🛑 RocketMQ Proxy Server is shutting down...")
		log.Println("🧹 Cleaning up all RocketMQ resources...")

		// 清理所有生产者
		rocketmqService.ShutdownAllProducers()

		// 清理所有消费者 - 需要先获取所有消费者ID
		rocketmqService.ShutdownAllConsumers()

		log.Println("✅ All RocketMQ resources cleaned up")
		log.Println("🔚 Stopping gRPC server...")

		s.GracefulStop()
	}()

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

// metricsInterceptor gRPC拦截器，用于收集指标
func metricsInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	start := time.Now()

	// 增加活跃连接数
	metrics.GlobalMetrics.IncActiveConnections()
	defer metrics.GlobalMetrics.DecActiveConnections()

	// 处理请求
	resp, err := handler(ctx, req)

	// 更新响应时间
	duration := time.Since(start)
	metrics.GlobalMetrics.UpdateResponseTime(duration)

	// 记录错误
	if err != nil {
		metrics.GlobalMetrics.IncErrorCount()
	}

	return resp, err
}

// startMetricsServer 启动指标HTTP服务器
func startMetricsServer(port string) {
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		stats := metrics.GlobalMetrics.GetStats()
		json.NewEncoder(w).Encode(stats)
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "healthy",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	log.Printf("📊 Metrics server starting on :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Printf("Metrics server error: %v", err)
	}
}

// startResourceMonitor 启动资源监控
func startResourceMonitor() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// 更新协程数量
		atomic.StoreInt64(&metrics.GlobalMetrics.GoroutineCount, int64(runtime.NumGoroutine()))

		// 更新内存使用
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		atomic.StoreInt64(&metrics.GlobalMetrics.MemoryUsage, int64(m.Alloc))
	}
}

// startConsumerCleanupTask 启动消费者清理定时任务
func startConsumerCleanupTask(rocketmqService *service.RocketMQProxyService) {
	ticker := time.NewTicker(2 * time.Minute) // 清理检查间隔：2分钟
	defer ticker.Stop()

	log.Printf("🧹 Starting consumer cleanup task (check interval: 2 minutes, timeout: 10 minutes) - supports cluster mode")

	for range ticker.C {
		// 清理超过10分钟未活跃的消费者（支持集群消费模式，更长的超时时间）
		rocketmqService.CleanupInactiveConsumers(10 * time.Minute)
	}
}

// startProducerCleanupTask 启动生产者清理定时任务
func startProducerCleanupTask(rocketmqService *service.RocketMQProxyService) {
	ticker := time.NewTicker(30 * time.Second) // 更频繁的清理检查：30秒
	defer ticker.Stop()

	// 引用计数验证计时器 - 每5分钟检查一次
	refCountCheckTicker := time.NewTicker(5 * time.Minute)
	defer refCountCheckTicker.Stop()

	log.Printf("🧹 Starting producer cleanup task (check interval: 30s, timeout: 1 minute)")
	log.Printf("🔍 Producer reference count validation enabled (check interval: 5 minutes)")

	for {
		select {
		case <-ticker.C:
			// 清理超过1分钟未活跃的生产者（更快的清理）
			rocketmqService.CleanupInactiveProducers(1 * time.Minute)
		case <-refCountCheckTicker.C:
			// 验证并修复引用计数不一致问题
			rocketmqService.ValidateAndFixProducerRefCounts()
		}
	}
}
