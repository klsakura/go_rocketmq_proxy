package config

import (
	"os"
	"strconv"
	"time"
)

// ServerConfig 服务配置
type ServerConfig struct {
	// gRPC服务配置
	Port          string
	MaxConcurrent int // 最大并发gRPC连接数
	ReadTimeout   time.Duration
	WriteTimeout  time.Duration

	// RocketMQ配置
	ProducerPoolSize  int    // 生产者连接池大小
	ConsumerPoolSize  int    // 消费者连接池大小
	MessageBufferSize int    // 消息缓冲区大小
	LogLevel          string // RocketMQ SDK日志级别

	// RocketMQ 超时配置
	PullTimeout  time.Duration // 消息拉取超时时间
	PullInterval time.Duration // 拉取间隔

	// 性能配置
	WorkerPoolSize int           // 工作协程池大小
	BatchSize      int           // 批量处理大小
	FlushInterval  time.Duration // 刷新间隔

	// 清理配置
	ProducerCleanupTimeout time.Duration // 生产者清理超时时间
	ConsumerCleanupTimeout time.Duration // 消费者清理超时时间

	// 监控配置
	EnableMetrics bool
	MetricsPort   string
}

// DefaultConfig 默认配置
func DefaultConfig() *ServerConfig {
	return &ServerConfig{
		Port:          getEnv("GRPC_PORT", "50051"),
		MaxConcurrent: getEnvInt("MAX_CONCURRENT", 1000),
		ReadTimeout:   getEnvDuration("READ_TIMEOUT", "30s"),
		WriteTimeout:  getEnvDuration("WRITE_TIMEOUT", "30s"),

		ProducerPoolSize:  getEnvInt("PRODUCER_POOL_SIZE", 10),
		ConsumerPoolSize:  getEnvInt("CONSUMER_POOL_SIZE", 20),
		MessageBufferSize: getEnvInt("MESSAGE_BUFFER_SIZE", 1000),
		LogLevel:          getEnv("ROCKETMQ_LOG_LEVEL", "warn"),

		WorkerPoolSize: getEnvInt("WORKER_POOL_SIZE", 100),
		BatchSize:      getEnvInt("BATCH_SIZE", 100),
		FlushInterval:  getEnvDuration("FLUSH_INTERVAL", "100ms"),

		ProducerCleanupTimeout: getEnvDuration("PRODUCER_CLEANUP_TIMEOUT", "30s"),
		ConsumerCleanupTimeout: getEnvDuration("CONSUMER_CLEANUP_TIMEOUT", "30s"),

		EnableMetrics: getEnvBool("ENABLE_METRICS", true),
		MetricsPort:   getEnv("METRICS_PORT", "8080"),

		PullTimeout:  getEnvDuration("PULL_TIMEOUT", "5s"),
		PullInterval: getEnvDuration("PULL_INTERVAL", "1s"),
	}
}

// 辅助函数
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue string) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	d, _ := time.ParseDuration(defaultValue)
	return d
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}
