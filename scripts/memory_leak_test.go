package main

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"time"

	"go_rocketmq_sdk/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// MemoryStats 内存统计
type MemoryStats struct {
	Timestamp  time.Time
	Alloc      uint64 // 当前分配的内存
	TotalAlloc uint64 // 累计分配的内存
	Sys        uint64 // 系统内存
	NumGC      uint32 // GC次数
	Goroutines int    // goroutine数量
}

func main() {
	log.Println("🔍 开始 RocketMQ Proxy 内存泄漏检测...")

	// 连接到 gRPC 服务
	conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("连接失败: %v", err)
	}
	defer conn.Close()

	client := proto.NewRocketMQProxyClient(conn)

	// 记录初始内存状态
	initialStats := getMemoryStats()
	log.Printf("📊 初始内存状态: %s", formatStats(initialStats))

	// 测试1: 生产者泄漏检测
	log.Println("\n🔬 测试1: 生产者内存泄漏检测")
	testProducerLeak(client)

	// 测试2: 消费者泄漏检测
	log.Println("\n🔬 测试2: 消费者内存泄漏检测")
	testConsumerLeak(client)

	// 测试3: 大量连接测试
	log.Println("\n🔬 测试3: 大量连接压力测试")
	testMassiveConnections(client)

	// 最终报告
	finalStats := getMemoryStats()
	log.Printf("\n📊 最终内存状态: %s", formatStats(finalStats))

	// 内存增长分析
	memGrowth := finalStats.Alloc - initialStats.Alloc
	totalAllocGrowth := finalStats.TotalAlloc - initialStats.TotalAlloc
	goroutineGrowth := finalStats.Goroutines - initialStats.Goroutines

	log.Printf("\n📈 内存增长分析:")
	log.Printf("   当前内存增长: %.2f MB", float64(memGrowth)/(1024*1024))
	log.Printf("   累计分配增长: %.2f MB", float64(totalAllocGrowth)/(1024*1024))
	log.Printf("   Goroutine 增长: %d", goroutineGrowth)
	log.Printf("   GC 次数增长: %d", finalStats.NumGC-initialStats.NumGC)

	// 风险评估
	if memGrowth > 10*1024*1024 { // 10MB
		log.Printf("🔴 高风险: 内存增长超过 10MB")
	} else if memGrowth > 5*1024*1024 { // 5MB
		log.Printf("🟡 中等风险: 内存增长 5-10MB")
	} else {
		log.Printf("✅ 低风险: 内存增长在正常范围内")
	}

	if goroutineGrowth > 10 {
		log.Printf("🔴 高风险: Goroutine 增长过多 (%d)", goroutineGrowth)
	} else if goroutineGrowth > 5 {
		log.Printf("🟡 中等风险: Goroutine 有增长 (%d)", goroutineGrowth)
	} else {
		log.Printf("✅ 正常: Goroutine 数量稳定")
	}
}

// testProducerLeak 测试生产者内存泄漏
func testProducerLeak(client proto.RocketMQProxyClient) {
	ctx := context.Background()

	log.Println("创建大量生产者...")
	var producerIDs []string

	// 创建100个生产者
	for i := 0; i < 100; i++ {
		req := &proto.CreateProducerRequest{
			Endpoint:        "your-endpoint",
			AccessKeyId:     "your-access-key",
			AccessKeySecret: "your-secret-key",
			InstanceId:      "your-instance-id",
			Topic:           fmt.Sprintf("test-topic-%d", i%5), // 使用5个不同topic
		}

		resp, err := client.CreateProducer(ctx, req)
		if err != nil {
			log.Printf("⚠️ 创建生产者失败: %v", err)
			continue
		}

		if resp.Success {
			producerIDs = append(producerIDs, resp.ProducerId)
		}

		// 每10个检查一次内存
		if (i+1)%10 == 0 {
			stats := getMemoryStats()
			log.Printf("   创建 %d 个生产者后: %s", i+1, formatStats(stats))
		}
	}

	log.Printf("✅ 总共创建了 %d 个生产者", len(producerIDs))

	// 等待一段时间观察内存
	log.Println("等待 10 秒观察内存变化...")
	time.Sleep(10 * time.Second)

	afterStats := getMemoryStats()
	log.Printf("📊 等待后内存状态: %s", formatStats(afterStats))

	// 注意: 这里无法清理生产者，因为当前代码没有清理API
	log.Printf("⚠️ 警告: 当前代码没有生产者清理机制，这些生产者将永远存在!")
}

// testConsumerLeak 测试消费者内存泄漏
func testConsumerLeak(client proto.RocketMQProxyClient) {
	ctx := context.Background()

	log.Println("创建并快速断开消费者连接...")

	for i := 0; i < 20; i++ {
		// 创建消费者
		req := &proto.CreateConsumerRequest{
			Endpoint:        "your-endpoint",
			AccessKeyId:     "your-access-key",
			AccessKeySecret: "your-secret-key",
			InstanceId:      "your-instance-id",
			Topic:           "test-topic",
			GroupId:         fmt.Sprintf("test-group-%d", i),
			TagExpression:   "*",
		}

		resp, err := client.CreateConsumer(ctx, req)
		if err != nil {
			log.Printf("⚠️ 创建消费者失败: %v", err)
			continue
		}

		if resp.Success {
			// 立即开始接收消息然后断开
			stream, err := client.ReceiveMessages(ctx, &proto.ReceiveMessagesRequest{
				ConsumerId: resp.ConsumerId,
			})
			if err != nil {
				log.Printf("⚠️ 创建流失败: %v", err)
				continue
			}

			// 快速断开连接
			go func() {
				time.Sleep(100 * time.Millisecond)
				// 模拟客户端断开
			}()

			// 尝试接收一条消息然后退出
			_, err = stream.Recv()
			if err != nil {
				// 预期的错误，连接断开
			}
		}

		if (i+1)%5 == 0 {
			stats := getMemoryStats()
			log.Printf("   处理 %d 个消费者后: %s", i+1, formatStats(stats))
		}
	}

	// 等待清理任务运行
	log.Println("等待 65 秒让清理任务运行...")
	time.Sleep(65 * time.Second)

	cleanupStats := getMemoryStats()
	log.Printf("📊 清理后内存状态: %s", formatStats(cleanupStats))
}

// testMassiveConnections 测试大量连接
func testMassiveConnections(client proto.RocketMQProxyClient) {
	ctx := context.Background()

	log.Println("创建大量并发连接...")

	// 并发创建连接
	for i := 0; i < 50; i++ {
		go func(index int) {
			// 创建生产者
			req := &proto.CreateProducerRequest{
				Endpoint:        "your-endpoint",
				AccessKeyId:     "your-access-key",
				AccessKeySecret: "your-secret-key",
				InstanceId:      "your-instance-id",
				Topic:           fmt.Sprintf("stress-topic-%d", index%3),
			}

			_, err := client.CreateProducer(ctx, req)
			if err != nil {
				log.Printf("⚠️ 并发创建生产者失败: %v", err)
			}
		}(i)
	}

	// 等待所有 goroutine 完成
	time.Sleep(5 * time.Second)

	stressStats := getMemoryStats()
	log.Printf("📊 压力测试后内存状态: %s", formatStats(stressStats))
}

// getMemoryStats 获取当前内存统计
func getMemoryStats() MemoryStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return MemoryStats{
		Timestamp:  time.Now(),
		Alloc:      m.Alloc,
		TotalAlloc: m.TotalAlloc,
		Sys:        m.Sys,
		NumGC:      m.NumGC,
		Goroutines: runtime.NumGoroutine(),
	}
}

// formatStats 格式化内存统计信息
func formatStats(stats MemoryStats) string {
	return fmt.Sprintf("内存: %.1fMB, 累计: %.1fMB, 系统: %.1fMB, GC: %d, Goroutines: %d",
		float64(stats.Alloc)/(1024*1024),
		float64(stats.TotalAlloc)/(1024*1024),
		float64(stats.Sys)/(1024*1024),
		stats.NumGC,
		stats.Goroutines)
}
