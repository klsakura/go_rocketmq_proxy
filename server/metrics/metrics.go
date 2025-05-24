package metrics

import (
	"sync/atomic"
	"time"
)

// Metrics 性能指标
type Metrics struct {
	// 并发指标
	ActiveConnections int64 // 当前活跃连接数
	TotalConnections  int64 // 总连接数
	ActiveProducers   int64 // 当前活跃生产者数
	ActiveConsumers   int64 // 当前活跃消费者数

	// 消息指标
	MessagesSent      int64 // 发送的消息总数
	MessagesReceived  int64 // 接收的消息总数
	MessagesPerSecond int64 // 每秒处理消息数

	// 性能指标
	AvgResponseTime int64 // 平均响应时间(纳秒)
	ErrorCount      int64 // 错误计数

	// 资源指标
	GoroutineCount int64 // 协程数量
	MemoryUsage    int64 // 内存使用量

	// 新增 - 内存泄漏监控指标
	ProducerLeakDetections int64 // 生产者泄漏检测次数
	ConsumerLeakDetections int64 // 消费者泄漏检测次数
	ChannelFullEvents      int64 // Channel 满事件次数
	GoroutineLeaks         int64 // Goroutine 泄漏检测次数

	lastUpdateTime     time.Time
	messageCountWindow []int64 // 滑动窗口计算QPS
}

// GlobalMetrics 全局指标实例
var GlobalMetrics = &Metrics{
	lastUpdateTime:     time.Now(),
	messageCountWindow: make([]int64, 60), // 60秒窗口
}

// IncActiveConnections 增加活跃连接数
func (m *Metrics) IncActiveConnections() {
	atomic.AddInt64(&m.ActiveConnections, 1)
	atomic.AddInt64(&m.TotalConnections, 1)
}

// DecActiveConnections 减少活跃连接数
func (m *Metrics) DecActiveConnections() {
	atomic.AddInt64(&m.ActiveConnections, -1)
}

// IncActiveProducers 增加活跃生产者数
func (m *Metrics) IncActiveProducers() {
	atomic.AddInt64(&m.ActiveProducers, 1)
}

// DecActiveProducers 减少活跃生产者数
func (m *Metrics) DecActiveProducers() {
	atomic.AddInt64(&m.ActiveProducers, -1)
}

// ResetActiveProducers 重置活跃生产者数为0 - 用于服务关闭时
func (m *Metrics) ResetActiveProducers() {
	atomic.StoreInt64(&m.ActiveProducers, 0)
}

// IncActiveConsumers 增加活跃消费者数
func (m *Metrics) IncActiveConsumers() {
	atomic.AddInt64(&m.ActiveConsumers, 1)
}

// DecActiveConsumers 减少活跃消费者数
func (m *Metrics) DecActiveConsumers() {
	atomic.AddInt64(&m.ActiveConsumers, -1)
}

// ResetActiveConsumers 重置活跃消费者数为0 - 用于服务关闭时
func (m *Metrics) ResetActiveConsumers() {
	atomic.StoreInt64(&m.ActiveConsumers, 0)
}

// IncMessagesSent 增加发送消息计数
func (m *Metrics) IncMessagesSent() {
	atomic.AddInt64(&m.MessagesSent, 1)
}

// IncMessagesReceived 增加接收消息计数
func (m *Metrics) IncMessagesReceived() {
	atomic.AddInt64(&m.MessagesReceived, 1)
}

// IncErrorCount 增加错误计数
func (m *Metrics) IncErrorCount() {
	atomic.AddInt64(&m.ErrorCount, 1)
}

// IncChannelFullEvents 增加Channel满事件计数
func (m *Metrics) IncChannelFullEvents() {
	atomic.AddInt64(&m.ChannelFullEvents, 1)
}

// IncProducerLeakDetections 增加生产者泄漏检测次数
func (m *Metrics) IncProducerLeakDetections() {
	atomic.AddInt64(&m.ProducerLeakDetections, 1)
}

// IncConsumerLeakDetections 增加消费者泄漏检测次数
func (m *Metrics) IncConsumerLeakDetections() {
	atomic.AddInt64(&m.ConsumerLeakDetections, 1)
}

// IncGoroutineLeaks 增加Goroutine泄漏检测次数
func (m *Metrics) IncGoroutineLeaks() {
	atomic.AddInt64(&m.GoroutineLeaks, 1)
}

// UpdateResponseTime 更新响应时间
func (m *Metrics) UpdateResponseTime(duration time.Duration) {
	atomic.StoreInt64(&m.AvgResponseTime, int64(duration))
}

// GetStats 获取当前统计信息
func (m *Metrics) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"active_connections":       atomic.LoadInt64(&m.ActiveConnections),
		"total_connections":        atomic.LoadInt64(&m.TotalConnections),
		"active_producers":         atomic.LoadInt64(&m.ActiveProducers),
		"active_consumers":         atomic.LoadInt64(&m.ActiveConsumers),
		"messages_sent":            atomic.LoadInt64(&m.MessagesSent),
		"messages_received":        atomic.LoadInt64(&m.MessagesReceived),
		"messages_per_second":      atomic.LoadInt64(&m.MessagesPerSecond),
		"avg_response_time_ms":     atomic.LoadInt64(&m.AvgResponseTime) / int64(time.Millisecond),
		"error_count":              atomic.LoadInt64(&m.ErrorCount),
		"goroutine_count":          atomic.LoadInt64(&m.GoroutineCount),
		"memory_usage_mb":          atomic.LoadInt64(&m.MemoryUsage) / 1024 / 1024,
		"producer_leak_detections": atomic.LoadInt64(&m.ProducerLeakDetections),
		"consumer_leak_detections": atomic.LoadInt64(&m.ConsumerLeakDetections),
		"channel_full_events":      atomic.LoadInt64(&m.ChannelFullEvents),
		"goroutine_leaks":          atomic.LoadInt64(&m.GoroutineLeaks),
	}
}

// StartPeriodicUpdate 启动定期更新
func (m *Metrics) StartPeriodicUpdate() {
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		var lastSent, lastReceived int64

		for range ticker.C {
			currentSent := atomic.LoadInt64(&m.MessagesSent)
			currentReceived := atomic.LoadInt64(&m.MessagesReceived)

			// 计算每秒消息数
			qps := (currentSent + currentReceived) - (lastSent + lastReceived)
			atomic.StoreInt64(&m.MessagesPerSecond, qps)

			lastSent = currentSent
			lastReceived = currentReceived
		}
	}()
}
