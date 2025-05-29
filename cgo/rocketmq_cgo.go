package main

/*
#include <stdlib.h>
#include <string.h>

// 定义C++回调函数类型
typedef void (*MessageHandler)(const char* messageJson);

// 调用C++回调函数的包装函数
static void call_cpp_handler(MessageHandler handler, const char* messageJson) {
    if (handler != NULL) {
        handler(messageJson);
    }
}
*/
import "C"

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/producer"
	"github.com/apache/rocketmq-client-go/v2/rlog"
)

// 全局管理器
var (
	producerManager = &ProducerManager{
		producers: make(map[string]rocketmq.Producer),
		mutex:     sync.RWMutex{},
	}
	consumerManager = &ConsumerManager{
		consumers:       make(map[string]rocketmq.PushConsumer),
		handlers:        make(map[string]C.MessageHandler),
		goroutineCounts: make(map[string]int),
		semaphores:      make(map[string]chan struct{}),
		mutex:           sync.RWMutex{},
	}
	globalConfig = &Config{} // 全局配置
	configMutex  = sync.RWMutex{}
)

// calculateDefaultGoroutines 根据系统资源计算默认协程数
func calculateDefaultGoroutines() int {
	cpuCount := runtime.NumCPU()

	// 基于 CPU 核心数的推荐算法：
	// - 对于 I/O 密集型任务（如消息处理），建议是 CPU 核心数的 2-4 倍
	// - 考虑内存使用情况，每个协程大约占用 2KB 栈空间

	var recommended int
	switch {
	case cpuCount <= 2:
		// 单核或双核：8 个协程
		recommended = 8
	case cpuCount <= 4:
		// 四核：CPU 核心数 * 3
		recommended = cpuCount * 3
	case cpuCount <= 8:
		// 八核：CPU 核心数 * 2.5
		recommended = int(float64(cpuCount) * 2.5)
	case cpuCount <= 16:
		// 16核：CPU 核心数 * 2
		recommended = cpuCount * 2
	default:
		// 高核心数：CPU 核心数 * 1.5，但不超过 64
		recommended = int(float64(cpuCount) * 1.5)
		if recommended > 64 {
			recommended = 64
		}
	}

	// 确保最小值为 4，最大值为 64
	if recommended < 4 {
		recommended = 4
	}
	if recommended > 64 {
		recommended = 64
	}

	return recommended
}

// ProducerManager 生产者管理器
type ProducerManager struct {
	producers map[string]rocketmq.Producer
	mutex     sync.RWMutex
}

// ConsumerManager 消费者管理器
type ConsumerManager struct {
	consumers       map[string]rocketmq.PushConsumer
	handlers        map[string]C.MessageHandler
	goroutineCounts map[string]int           // 每个消费者的协程数配置
	semaphores      map[string]chan struct{} // 协程池信号量
	mutex           sync.RWMutex
}

// Config 配置结构
type Config struct {
	Endpoint        string `json:"endpoint"`
	AccessKeyId     string `json:"accessKeyId"`
	AccessKeySecret string `json:"accessKeySecret"`
	InstanceId      string `json:"instanceId"`
	LogLevel        string `json:"logLevel,omitempty"` // 可选：日志级别 (debug, info, warn, error, fatal)
	Thread          int    `json:"thread,omitempty"`   // 可选：消费者并发协程数，0表示使用系统推荐值
}

// MessageProperties 消息属性
type MessageProperties struct {
	Properties             map[string]string `json:"properties"`
	MessageKey             string            `json:"messageKey"`
	ShardingKey            string            `json:"shardingKey"`
	StartDeliverTime       int64             `json:"startDeliverTime"`
	TransCheckImmunityTime int64             `json:"transCheckImmunityTime"`
}

// SendResult 发送结果
type SendResult struct {
	MessageId     string `json:"messageId"`
	ReceiptHandle string `json:"receiptHandle"`
	Success       bool   `json:"success"`
	Message       string `json:"message"`
}

// MessageData 消息数据
type MessageData struct {
	MessageId      string            `json:"messageId"`
	ReceiptHandle  string            `json:"receiptHandle"`
	Body           string            `json:"body"`
	Tag            string            `json:"tag"`
	Properties     map[string]string `json:"properties"`
	BornTimestamp  int64             `json:"bornTimestamp"`
	ReconsumeTimes int32             `json:"reconsumeTimes"`
}

//export InitRocketMQ
func InitRocketMQ(configJson *C.char) *C.char {
	configStr := C.GoString(configJson)
	var config Config
	if err := json.Unmarshal([]byte(configStr), &config); err != nil {
		return C.CString(fmt.Sprintf(`{"success": false, "message": "Invalid config: %s"}`, err.Error()))
	}

	// 设置日志级别
	logLevel := "warn" // 默认日志级别
	if config.LogLevel != "" {
		// 验证日志级别是否有效
		validLevels := map[string]bool{
			"debug": true,
			"info":  true,
			"warn":  true,
			"error": true,
			"fatal": true,
		}
		if validLevels[strings.ToLower(config.LogLevel)] {
			logLevel = strings.ToLower(config.LogLevel)
		} else {
			return C.CString(fmt.Sprintf(`{"success": false, "message": "Invalid log level: %s. Valid levels: debug, info, warn, error, fatal"}`, config.LogLevel))
		}
	}
	rlog.SetLogLevel(logLevel)

	// 保存全局配置
	configMutex.Lock()
	*globalConfig = config
	globalConfig.LogLevel = logLevel // 确保使用验证后的日志级别
	configMutex.Unlock()

	// 计算推荐的协程数
	recommendedGoroutines := calculateDefaultGoroutines()
	cpuCount := runtime.NumCPU()

	result := map[string]interface{}{
		"success":               true,
		"message":               "RocketMQ initialized successfully",
		"logLevel":              logLevel,
		"thread":                config.Thread,
		"recommendedGoroutines": recommendedGoroutines,
		"cpuCores":              cpuCount,
		"systemInfo": map[string]interface{}{
			"cpuCores":  cpuCount,
			"goVersion": runtime.Version(),
			"arch":      runtime.GOARCH,
		},
	}

	resultJson, _ := json.Marshal(result)
	return C.CString(string(resultJson))
}

//export CreateProducer
func CreateProducer(configJson *C.char, topic *C.char) *C.char {
	configStr := C.GoString(configJson)
	topicStr := C.GoString(topic)

	var config Config
	if err := json.Unmarshal([]byte(configStr), &config); err != nil {
		return C.CString(fmt.Sprintf(`{"success": false, "message": "Invalid config: %s"}`, err.Error()))
	}

	producerId := fmt.Sprintf("%s_%s_%d", config.InstanceId, topicStr, time.Now().UnixNano())

	// 创建生产者配置
	opts := []producer.Option{
		producer.WithNameServer([]string{config.Endpoint}),
		producer.WithCredentials(primitive.Credentials{
			AccessKey: config.AccessKeyId,
			SecretKey: config.AccessKeySecret,
		}),
		producer.WithInstanceName(config.InstanceId),
		producer.WithRetry(3),
		producer.WithQueueSelector(producer.NewRoundRobinQueueSelector()),
	}

	p, err := rocketmq.NewProducer(opts...)
	if err != nil {
		return C.CString(fmt.Sprintf(`{"success": false, "message": "Failed to create producer: %s"}`, err.Error()))
	}

	if err := p.Start(); err != nil {
		return C.CString(fmt.Sprintf(`{"success": false, "message": "Failed to start producer: %s"}`, err.Error()))
	}

	producerManager.mutex.Lock()
	producerManager.producers[producerId] = p
	producerManager.mutex.Unlock()

	result := map[string]interface{}{
		"success":    true,
		"producerId": producerId,
		"message":    "Producer created successfully",
	}

	resultJson, _ := json.Marshal(result)
	return C.CString(string(resultJson))
}

//export SendMessage
func SendMessage(producerId *C.char, messageBody *C.char, tag *C.char, propertiesJson *C.char) *C.char {
	producerIdStr := C.GoString(producerId)
	messageBodyStr := C.GoString(messageBody)
	tagStr := C.GoString(tag)
	propertiesStr := C.GoString(propertiesJson)

	producerManager.mutex.RLock()
	p, exists := producerManager.producers[producerIdStr]
	producerManager.mutex.RUnlock()

	if !exists {
		return C.CString(`{"success": false, "message": "Producer not found"}`)
	}

	var props MessageProperties
	if propertiesStr != "" {
		if err := json.Unmarshal([]byte(propertiesStr), &props); err != nil {
			return C.CString(fmt.Sprintf(`{"success": false, "message": "Invalid properties: %s"}`, err.Error()))
		}
	}

	// 创建消息
	msg := &primitive.Message{
		Topic: extractTopicFromProducerId(producerIdStr),
		Body:  []byte(messageBodyStr),
	}

	if tagStr != "" {
		msg.WithTag(tagStr)
	}

	if props.MessageKey != "" {
		msg.WithKeys([]string{props.MessageKey})
	}

	if props.ShardingKey != "" {
		msg.WithShardingKey(props.ShardingKey)
	}

	// 设置属性
	for k, v := range props.Properties {
		msg.WithProperty(k, v)
	}

	// 设置延迟投递时间
	if props.StartDeliverTime > 0 {
		msg.WithDelayTimeLevel(int(props.StartDeliverTime))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := p.SendSync(ctx, msg)
	if err != nil {
		return C.CString(fmt.Sprintf(`{"success": false, "message": "Failed to send message: %s"}`, err.Error()))
	}

	sendResult := SendResult{
		MessageId:     result.MsgID,
		ReceiptHandle: result.MsgID, // 使用MessageId作为ReceiptHandle
		Success:       true,
		Message:       "Message sent successfully",
	}

	resultJson, _ := json.Marshal(sendResult)
	return C.CString(string(resultJson))
}

//export SendOrderedMessage
func SendOrderedMessage(producerId *C.char, messageBody *C.char, tag *C.char, propertiesJson *C.char, shardingKey *C.char) *C.char {
	producerIdStr := C.GoString(producerId)
	messageBodyStr := C.GoString(messageBody)
	tagStr := C.GoString(tag)
	propertiesStr := C.GoString(propertiesJson)
	shardingKeyStr := C.GoString(shardingKey)

	producerManager.mutex.RLock()
	p, exists := producerManager.producers[producerIdStr]
	producerManager.mutex.RUnlock()

	if !exists {
		return C.CString(`{"success": false, "message": "Producer not found"}`)
	}

	var props MessageProperties
	if propertiesStr != "" {
		if err := json.Unmarshal([]byte(propertiesStr), &props); err != nil {
			return C.CString(fmt.Sprintf(`{"success": false, "message": "Invalid properties: %s"}`, err.Error()))
		}
	}

	// 创建消息
	msg := &primitive.Message{
		Topic: extractTopicFromProducerId(producerIdStr),
		Body:  []byte(messageBodyStr),
	}

	if tagStr != "" {
		msg.WithTag(tagStr)
	}

	if props.MessageKey != "" {
		msg.WithKeys([]string{props.MessageKey})
	}

	if shardingKeyStr != "" {
		msg.WithShardingKey(shardingKeyStr)
	}

	// 设置属性
	for k, v := range props.Properties {
		msg.WithProperty(k, v)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 发送顺序消息
	result, err := p.SendSync(ctx, msg)
	if err != nil {
		return C.CString(fmt.Sprintf(`{"success": false, "message": "Failed to send ordered message: %s"}`, err.Error()))
	}

	sendResult := SendResult{
		MessageId:     result.MsgID,
		ReceiptHandle: result.MsgID,
		Success:       true,
		Message:       "Ordered message sent successfully",
	}

	resultJson, _ := json.Marshal(sendResult)
	return C.CString(string(resultJson))
}

//export CreateConsumer
func CreateConsumer(configJson *C.char, topic *C.char, groupId *C.char, tagExpression *C.char) *C.char {
	configStr := C.GoString(configJson)
	topicStr := C.GoString(topic)
	groupIdStr := C.GoString(groupId)
	_ = C.GoString(tagExpression) // 暂时不使用，避免编译错误

	var config Config
	if err := json.Unmarshal([]byte(configStr), &config); err != nil {
		return C.CString(fmt.Sprintf(`{"success": false, "message": "Invalid config: %s"}`, err.Error()))
	}

	consumerId := fmt.Sprintf("%s_%s_%s_%d", config.InstanceId, topicStr, groupIdStr, time.Now().UnixNano())

	// 获取全局配置中的协程数
	configMutex.RLock()
	goroutineCount := globalConfig.Thread
	configMutex.RUnlock()

	// 设置默认协程数：如果用户没有设置或设置为0，则使用系统推荐值
	if goroutineCount <= 0 {
		goroutineCount = calculateDefaultGoroutines()
	}

	// 创建消费者配置
	opts := []consumer.Option{
		consumer.WithNameServer([]string{config.Endpoint}),
		consumer.WithCredentials(primitive.Credentials{
			AccessKey: config.AccessKeyId,
			SecretKey: config.AccessKeySecret,
		}),
		consumer.WithGroupName(groupIdStr),
		consumer.WithConsumeFromWhere(consumer.ConsumeFromLastOffset),
		consumer.WithConsumerModel(consumer.Clustering),
		consumer.WithConsumeMessageBatchMaxSize(1), // 每次消费一条消息
		consumer.WithMaxReconsumeTimes(3),          // 最大重试次数
	}

	c, err := rocketmq.NewPushConsumer(opts...)
	if err != nil {
		return C.CString(fmt.Sprintf(`{"success": false, "message": "Failed to create consumer: %s"}`, err.Error()))
	}

	consumerManager.mutex.Lock()
	consumerManager.consumers[consumerId] = c
	consumerManager.goroutineCounts[consumerId] = goroutineCount
	consumerManager.semaphores[consumerId] = make(chan struct{}, goroutineCount) // 创建协程池信号量
	consumerManager.mutex.Unlock()

	result := map[string]interface{}{
		"success":     true,
		"consumerId":  consumerId,
		"topic":       topicStr,
		"groupId":     groupIdStr,
		"threadCount": goroutineCount,
		"message":     "Consumer created successfully",
	}

	resultJson, _ := json.Marshal(result)
	return C.CString(string(resultJson))
}

//export StartConsumer
func StartConsumer(consumerId *C.char, topic *C.char, tagExpression *C.char) *C.char {
	consumerIdStr := C.GoString(consumerId)
	topicStr := C.GoString(topic)
	tagExpressionStr := C.GoString(tagExpression)

	if tagExpressionStr == "" {
		tagExpressionStr = "*"
	}

	consumerManager.mutex.RLock()
	c, exists := consumerManager.consumers[consumerIdStr]
	consumerManager.mutex.RUnlock()

	if !exists {
		return C.CString(`{"success": false, "message": "Consumer not found"}`)
	}

	// 订阅主题
	err := c.Subscribe(topicStr, consumer.MessageSelector{
		Type:       consumer.TAG,
		Expression: tagExpressionStr,
	}, func(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
		for _, msg := range msgs {
			messageData := MessageData{
				MessageId:      msg.MsgId,
				ReceiptHandle:  msg.MsgId,
				Body:           string(msg.Body),
				Tag:            msg.GetTags(),
				Properties:     msg.GetProperties(),
				BornTimestamp:  msg.BornTimestamp,
				ReconsumeTimes: msg.ReconsumeTimes,
			}

			messageJson, _ := json.Marshal(messageData)

			// 获取协程池信号量和处理函数
			consumerManager.mutex.RLock()
			handler, hasHandler := consumerManager.handlers[consumerIdStr]
			semaphore, hasSemaphore := consumerManager.semaphores[consumerIdStr]
			consumerManager.mutex.RUnlock()

			if hasHandler && hasSemaphore {
				// 使用协程池处理消息
				go func(msgJson []byte, msgHandler C.MessageHandler, sem chan struct{}) {
					// 获取协程池令牌
					sem <- struct{}{}
					defer func() { <-sem }() // 释放令牌

					cMessageJson := C.CString(string(msgJson))
					C.call_cpp_handler(msgHandler, cMessageJson)
					C.free(unsafe.Pointer(cMessageJson))
				}(messageJson, handler, semaphore)
			}
		}
		return consumer.ConsumeSuccess, nil
	})

	if err != nil {
		return C.CString(fmt.Sprintf(`{"success": false, "message": "Failed to subscribe: %s"}`, err.Error()))
	}

	if err := c.Start(); err != nil {
		return C.CString(fmt.Sprintf(`{"success": false, "message": "Failed to start consumer: %s"}`, err.Error()))
	}

	result := map[string]interface{}{
		"success": true,
		"message": "Consumer started successfully",
	}

	resultJson, _ := json.Marshal(result)
	return C.CString(string(resultJson))
}

//export RegisterMessageHandler
func RegisterMessageHandler(consumerId *C.char, handler C.MessageHandler) *C.char {
	consumerIdStr := C.GoString(consumerId)

	consumerManager.mutex.Lock()
	consumerManager.handlers[consumerIdStr] = handler
	consumerManager.mutex.Unlock()

	result := map[string]interface{}{
		"success": true,
		"message": "Message handler registered successfully",
	}

	resultJson, _ := json.Marshal(result)
	return C.CString(string(resultJson))
}

//export AckMessage
func AckMessage(consumerId *C.char, receiptHandle *C.char) *C.char {
	// RocketMQ的推送模式下，消息确认是自动的
	// 这里只是为了保持API兼容性
	result := map[string]interface{}{
		"success": true,
		"message": "Message acknowledged successfully",
	}

	resultJson, _ := json.Marshal(result)
	return C.CString(string(resultJson))
}

//export ShutdownProducer
func ShutdownProducer(producerId *C.char) *C.char {
	producerIdStr := C.GoString(producerId)

	producerManager.mutex.Lock()
	defer producerManager.mutex.Unlock()

	if p, exists := producerManager.producers[producerIdStr]; exists {
		p.Shutdown()
		delete(producerManager.producers, producerIdStr)
	}

	result := map[string]interface{}{
		"success": true,
		"message": "Producer shutdown successfully",
	}

	resultJson, _ := json.Marshal(result)
	return C.CString(string(resultJson))
}

//export ShutdownConsumer
func ShutdownConsumer(consumerId *C.char) *C.char {
	consumerIdStr := C.GoString(consumerId)

	// 添加 panic 恢复机制
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in ShutdownConsumer: %v", r)
		}
	}()

	consumerManager.mutex.Lock()
	defer consumerManager.mutex.Unlock()

	if c, exists := consumerManager.consumers[consumerIdStr]; exists {
		// 安全关闭，捕获可能的 panic
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Recovered from panic during consumer shutdown: %v", r)
				}
			}()
			c.Shutdown()
		}()

		delete(consumerManager.consumers, consumerIdStr)
		delete(consumerManager.handlers, consumerIdStr)
		delete(consumerManager.goroutineCounts, consumerIdStr)
		delete(consumerManager.semaphores, consumerIdStr)
	}

	result := map[string]interface{}{
		"success": true,
		"message": "Consumer shutdown successfully",
	}

	resultJson, _ := json.Marshal(result)
	return C.CString(string(resultJson))
}

//export FreeString
func FreeString(str *C.char) {
	C.free(unsafe.Pointer(str))
}

// 辅助函数
func extractTopicFromProducerId(producerId string) string {
	// 从producerId中提取topic: instanceId_topic_timestamp
	parts := strings.Split(producerId, "_")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

func main() {
	// 这个main函数在编译为共享库时不会被调用
	log.Println("RocketMQ CGO library loaded")
}
