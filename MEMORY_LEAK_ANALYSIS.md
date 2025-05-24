# Go RocketMQ代理服务内存泄漏风险分析报告

## 🔍 分析概述

本报告对 Go RocketMQ 代理服务进行了全面的内存泄漏风险评估，重点关注：
- Goroutine 泄漏风险
- 资源释放问题
- 内存分配模式
- 并发安全问题

## ⚠️ 发现的风险问题

### 1. 生产者资源泄漏风险 - 高风险 🔴

**问题描述：**
在 `server/service/rocketmq_service.go` 中，生产者只有创建逻辑，**没有任何清理和释放机制**。

**风险代码位置：**
```go
// 只有创建，没有清理！
func (s *RocketMQProxyService) CreateProducer(ctx context.Context, req *proto.CreateProducerRequest) (*proto.CreateProducerResponse, error) {
    // 创建生产者
    p, err := rocketmq.NewProducer(opts...)
    // 启动生产者
    err = p.Start()
    // 存储到 map 中，但没有释放机制
    s.producers[producerID] = &ProducerInfo{...}
}
```

**潜在影响：**
- 生产者连接会无限累积
- RocketMQ 连接资源泄漏
- 内存持续增长
- 最终导致服务崩溃

**修复建议：**
1. 添加生产者清理 API
2. 实现生产者超时清理机制
3. 在 gRPC 连接断开时自动清理相关生产者

### 2. Goroutine 泄漏风险 - 中等风险 🟡

**问题描述：**
在 `ReceiveMessages` 方法中存在未受控的 goroutine 创建。

**风险代码位置：**
```go:620-635:server/service/rocketmq_service.go
// Stream状态监控
streamDone := make(chan bool, 1)
go func() {
    <-stream.Context().Done()
    streamDone <- true
}()

// 确保在函数退出时清理消费者
defer func() {
    // 立即清理消费者，不等待定时任务
    go func() {  // 💀 潜在 goroutine 泄漏
        if err := s.CleanupConsumerByID(req.ConsumerId); err != nil {
            log.Printf("❌ Error cleaning up consumer %s: %v", req.ConsumerId, err)
        } else {
            log.Printf("✅ Consumer %s cleaned up immediately after stream end", req.ConsumerId)
        }
    }()
}()
```

**风险分析：**
- 每次流连接都会创建额外的 goroutine
- 虽然有清理机制，但如果清理失败，goroutine 可能永远不会结束

### 3. 锁竞争和死锁风险 - 中等风险 🟡

**问题代码：**
```go:740-751:server/service/rocketmq_service.go
func (s *RocketMQProxyService) CleanupInactiveConsumers(timeout time.Duration) {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    // ... 获取待清理列表 ...
    
    // 释放锁后清理，避免死锁
    s.mu.Unlock()
    for _, consumerID := range toCleanup {
        s.CleanupConsumerByID(consumerID)  // 这里会再次尝试获取锁
    }
    s.mu.Lock()  // 💀 双重解锁风险
}
```

**风险分析：**
- 双重解锁可能导致 panic
- 锁的释放和重新获取之间存在竞态条件

### 4. Channel 阻塞风险 - 中等风险 🟡

**风险代码：**
```go:507:server/service/rocketmq_service.go
messageChan := make(chan *proto.Message, s.config.MessageBufferSize)
```

**消息处理逻辑：**
```go:532-542:server/service/rocketmq_service.go
select {
case messageChan <- protoMsg:
    log.Printf("Message sent to channel: %s", msg.MsgId)
case <-consumerCtx.Done():
    return consumer.ConsumeRetryLater, fmt.Errorf("consumer context cancelled")
default:
    log.Printf("Message channel full, dropping message: %s", msg.MsgId)  // 💀 消息丢失
}
```

**风险分析：**
- 如果消费速度跟不上，channel 会满
- 满了之后消息会被直接丢弃
- 可能导致消息丢失

## ✅ 已经实现的良好实践

### 1. 消费者资源管理
- ✅ 实现了完整的消费者清理机制
- ✅ 定时清理不活跃的消费者（30秒检查，1分钟超时）
- ✅ 流断开时立即清理
- ✅ 正确关闭 channel 和 context

### 2. 内存监控
- ✅ 实现了内存使用监控
- ✅ 实时更新 goroutine 数量
- ✅ 提供 metrics 接口查看资源使用情况

### 3. 并发控制
- ✅ 使用 `sync.RWMutex` 保护共享资源
- ✅ 正确的读写锁使用模式

## 🔧 推荐的修复方案

### 1. 添加生产者清理机制

```go
// 添加生产者清理 API
func (s *RocketMQProxyService) CleanupProducer(ctx context.Context, req *proto.CleanupProducerRequest) (*proto.CleanupProducerResponse, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    producerInfo, exists := s.producers[req.ProducerId]
    if !exists {
        return &proto.CleanupProducerResponse{Success: false, Message: "Producer not found"}, nil
    }
    
    // 减少引用计数
    producerInfo.RefCount--
    
    // 如果引用计数为0，真正清理资源
    if producerInfo.RefCount <= 0 {
        if err := producerInfo.Producer.Shutdown(); err != nil {
            log.Printf("⚠️ Error shutting down producer: %v", err)
        }
        
        // 从映射中删除
        delete(s.producers, req.ProducerId)
        
        // 清理共享映射
        for key, id := range s.sharedProducers {
            if id == req.ProducerId {
                delete(s.sharedProducers, key)
                break
            }
        }
        
        metrics.GlobalMetrics.DecActiveProducers()
    }
    
    return &proto.CleanupProducerResponse{Success: true}, nil
}

// 添加定时清理任务
func (s *RocketMQProxyService) CleanupInactiveProducers(timeout time.Duration) {
    // 实现类似消费者的超时清理逻辑
}
```

### 2. 修复 Goroutine 泄漏

```go
// 使用 context 控制 goroutine 生命周期
func (s *RocketMQProxyService) ReceiveMessages(req *proto.ReceiveMessagesRequest, stream proto.RocketMQProxy_ReceiveMessagesServer) error {
    ctx, cancel := context.WithCancel(stream.Context())
    defer cancel()
    
    // 受控的 goroutine
    go func() {
        select {
        case <-ctx.Done():
            return
        case <-stream.Context().Done():
            return
        }
    }()
    
    // 直接清理，不使用额外的 goroutine
    defer func() {
        if err := s.CleanupConsumerByID(req.ConsumerId); err != nil {
            log.Printf("❌ Error cleaning up consumer %s: %v", req.ConsumerId, err)
        }
    }()
}
```

### 3. 修复锁使用问题

```go
func (s *RocketMQProxyService) CleanupInactiveConsumers(timeout time.Duration) {
    var toCleanup []string
    
    // 只在需要时获取锁
    func() {
        s.mu.RLock()
        defer s.mu.RUnlock()
        
        now := time.Now()
        for consumerID, consumerInfo := range s.consumers {
            if now.Sub(consumerInfo.LastActive) > timeout {
                toCleanup = append(toCleanup, consumerID)
            }
        }
    }()
    
    // 释放锁后清理
    for _, consumerID := range toCleanup {
        s.CleanupConsumerByID(consumerID)
    }
}
```

### 4. 增强 Channel 处理

```go
// 使用带超时的 channel 发送
select {
case messageChan <- protoMsg:
    log.Printf("Message sent to channel: %s", msg.MsgId)
case <-consumerCtx.Done():
    return consumer.ConsumeRetryLater, fmt.Errorf("consumer context cancelled")
case <-time.After(5 * time.Second):
    log.Printf("⚠️ Message channel send timeout, increasing buffer size may help: %s", msg.MsgId)
    return consumer.ConsumeRetryLater, fmt.Errorf("message channel timeout")
}
```

## 📊 建议的监控指标

添加以下监控指标：

```go
// 在 metrics 中添加
type Metrics struct {
    // 现有指标...
    
    // 新增指标
    ActiveProducerConnections int64 // 活跃生产者连接数
    ProducerLeakDetections    int64 // 生产者泄漏检测次数
    ChannelFullEvents         int64 // Channel 满事件次数
    GoroutineLeaks           int64 // Goroutine 泄漏检测次数
}
```

## 🎯 优先级建议

1. **立即修复（高优先级）**：
   - 添加生产者清理机制
   - 实现生产者超时清理

2. **近期修复（中优先级）**：
   - 修复锁使用问题
   - 优化 goroutine 管理

3. **持续优化（低优先级）**：
   - 增强监控指标
   - 添加更多配置选项

## 🔍 测试建议

1. **压力测试**：持续创建大量生产者和消费者，观察内存使用情况
2. **长期运行测试**：运行服务24小时以上，监控资源增长
3. **连接中断测试**：模拟网络中断，验证资源清理是否正确

## 📋 结论

您的服务在消费者资源管理方面做得很好，但在**生产者资源管理方面存在严重的内存泄漏风险**。建议优先解决生产者清理问题，这是最紧迫的内存泄漏风险源。 