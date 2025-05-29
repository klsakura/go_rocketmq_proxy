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
		consumers: make(map[string]rocketmq.PushConsumer),
		handlers:  make(map[string]C.MessageHandler),
		mutex:     sync.RWMutex{},
	}
)

// ProducerManager 生产者管理器
type ProducerManager struct {
	producers map[string]rocketmq.Producer
	mutex     sync.RWMutex
}

// ConsumerManager 消费者管理器
type ConsumerManager struct {
	consumers map[string]rocketmq.PushConsumer
	handlers  map[string]C.MessageHandler
	mutex     sync.RWMutex
}

// Config 配置结构
type Config struct {
	Endpoint        string `json:"endpoint"`
	AccessKeyId     string `json:"accessKeyId"`
	AccessKeySecret string `json:"accessKeySecret"`
	InstanceId      string `json:"instanceId"`
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
	rlog.SetLogLevel("warn")

	result := map[string]interface{}{
		"success": true,
		"message": "RocketMQ initialized successfully",
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
	}

	c, err := rocketmq.NewPushConsumer(opts...)
	if err != nil {
		return C.CString(fmt.Sprintf(`{"success": false, "message": "Failed to create consumer: %s"}`, err.Error()))
	}

	consumerManager.mutex.Lock()
	consumerManager.consumers[consumerId] = c
	consumerManager.mutex.Unlock()

	result := map[string]interface{}{
		"success":    true,
		"consumerId": consumerId,
		"topic":      topicStr,
		"groupId":    groupIdStr,
		"message":    "Consumer created successfully",
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

			// 调用注册的处理函数
			consumerManager.mutex.RLock()
			handler, hasHandler := consumerManager.handlers[consumerIdStr]
			consumerManager.mutex.RUnlock()

			if hasHandler {
				cMessageJson := C.CString(string(messageJson))
				C.call_cpp_handler(handler, cMessageJson)
				C.free(unsafe.Pointer(cMessageJson))
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

	consumerManager.mutex.Lock()
	defer consumerManager.mutex.Unlock()

	if c, exists := consumerManager.consumers[consumerIdStr]; exists {
		c.Shutdown()
		delete(consumerManager.consumers, consumerIdStr)
		delete(consumerManager.handlers, consumerIdStr)
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
