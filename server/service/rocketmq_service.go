package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"go_rocketmq_sdk/proto"
	"go_rocketmq_sdk/server/config"
	"go_rocketmq_sdk/server/metrics"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/producer"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ProducerInfo 生产者信息
type ProducerInfo struct {
	Producer   rocketmq.Producer
	Topic      string
	Endpoint   string
	InstanceId string
	RefCount   int       // 引用计数，支持复用
	LastActive time.Time // 最后活跃时间，用于超时清理
	CreatedAt  time.Time // 创建时间
}

// ConsumerInfo 消费者信息
type ConsumerInfo struct {
	Consumer    rocketmq.PushConsumer
	Topics      map[string]bool // 改为支持多个Topic的映射
	GroupID     string
	Endpoint    string
	InstanceId  string
	MessageChan chan *proto.Message
	CancelFunc  context.CancelFunc // 用于取消消费者上下文
	LastActive  time.Time          // 最后活跃时间
	RefCount    int                // 引用计数，支持复用
	CreatedAt   time.Time          // 创建时间
}

// ConnectionKey 连接唯一标识
type ConnectionKey struct {
	Endpoint    string
	AccessKeyId string
	InstanceId  string
	Topic       string
}

// ConsumerKey 消费者连接唯一标识 - 支持不同RocketMQ实例
type ConsumerKey struct {
	Endpoint    string
	AccessKeyId string
	InstanceId  string
	GroupID     string // 按GroupID+Endpoint复用，支持不同RocketMQ实例
}

// RocketMQProxyService gRPC服务实现
type RocketMQProxyService struct {
	proto.UnimplementedRocketMQProxyServer
	producers       map[string]*ProducerInfo // producer_id -> ProducerInfo
	consumers       map[string]*ConsumerInfo // consumer_id -> ConsumerInfo
	sharedProducers map[ConnectionKey]string // 连接配置 -> producer_id (支持多实例)
	sharedConsumers map[ConsumerKey]string   // 消费者配置 -> consumer_id (支持复用)
	mu              sync.RWMutex
	config          *config.ServerConfig // 服务配置
}

// NewRocketMQProxyService 创建新的服务实例
func NewRocketMQProxyService(cfg *config.ServerConfig) *RocketMQProxyService {
	return &RocketMQProxyService{
		producers:       make(map[string]*ProducerInfo),
		consumers:       make(map[string]*ConsumerInfo),
		sharedProducers: make(map[ConnectionKey]string),
		sharedConsumers: make(map[ConsumerKey]string),
		config:          cfg,
	}
}

// CreateProducer 创建生产者
func (s *RocketMQProxyService) CreateProducer(ctx context.Context, req *proto.CreateProducerRequest) (*proto.CreateProducerResponse, error) {
	log.Printf("Creating producer for topic: %s, instance: %s, endpoint: %s", req.Topic, req.InstanceId, req.Endpoint)

	// 生成连接key，支持多个RocketMQ实例
	connKey := ConnectionKey{
		Endpoint:    req.Endpoint,
		AccessKeyId: req.AccessKeyId,
		InstanceId:  req.InstanceId,
		Topic:       req.Topic,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查是否已有相同配置的生产者可以复用
	if existingProducerID, exists := s.sharedProducers[connKey]; exists {
		if producerInfo, found := s.producers[existingProducerID]; found {
			// 生成新的ID但复用同一个生产者实例
			newProducerID := uuid.New().String()
			s.producers[newProducerID] = &ProducerInfo{
				Producer:   producerInfo.Producer,
				Topic:      producerInfo.Topic,
				Endpoint:   producerInfo.Endpoint,
				InstanceId: producerInfo.InstanceId,
				RefCount:   producerInfo.RefCount + 1,
				LastActive: producerInfo.LastActive,
				CreatedAt:  producerInfo.CreatedAt,
			}
			producerInfo.RefCount++

			log.Printf("✅ Reusing producer: ID=%s, Topic=%s, Instance=%s, RefCount=%d",
				newProducerID, req.Topic, req.InstanceId, producerInfo.RefCount)
			return &proto.CreateProducerResponse{
				Success:    true,
				Message:    fmt.Sprintf("Producer reused (ref: %d)", producerInfo.RefCount),
				ProducerId: newProducerID,
			}, nil
		}
	}

	// 创建新的生产者
	// 使用简单的生产者组名，不包含实例ID
	producerGroup := fmt.Sprintf("grpc_proxy_producer_%d", time.Now().UnixNano())

	// 创建生产者配置 - 按照官方示例
	opts := []producer.Option{
		// 使用官方推荐的NameServer配置方式
		producer.WithNsResolver(primitive.NewPassthroughResolver([]string{req.Endpoint})),
		producer.WithCredentials(primitive.Credentials{
			AccessKey: req.AccessKeyId,
			SecretKey: req.AccessKeySecret,
		}),
		producer.WithGroupName(producerGroup),
		producer.WithRetry(2),
	}

	// 创建生产者
	p, err := rocketmq.NewProducer(opts...)
	if err != nil {
		return &proto.CreateProducerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to create producer: %v", err),
		}, nil
	}

	// 启动生产者
	err = p.Start()
	if err != nil {
		// 启动失败时清理已创建的生产者资源
		if shutdownErr := p.Shutdown(); shutdownErr != nil {
			log.Printf("⚠️ Error shutting down failed producer during cleanup: %v", shutdownErr)
		}
		return &proto.CreateProducerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to start producer: %v", err),
		}, nil
	}

	// 生成唯一ID
	producerID := uuid.New().String()

	// 存储生产者信息
	s.producers[producerID] = &ProducerInfo{
		Producer:   p,
		Topic:      req.Topic,
		Endpoint:   req.Endpoint,
		InstanceId: req.InstanceId,
		RefCount:   1,
		LastActive: time.Now(),
		CreatedAt:  time.Now(),
	}

	// 记录复用映射
	s.sharedProducers[connKey] = producerID

	// 增加生产者计数
	metrics.GlobalMetrics.IncActiveProducers()

	log.Printf("🚀 New producer created: ID=%s, Group=%s, Topic=%s, Instance=%s",
		producerID, producerGroup, req.Topic, req.InstanceId)
	return &proto.CreateProducerResponse{
		Success:    true,
		Message:    "Producer created successfully",
		ProducerId: producerID,
	}, nil
}

// SendMessage 发送消息
func (s *RocketMQProxyService) SendMessage(ctx context.Context, req *proto.SendMessageRequest) (*proto.SendMessageResponse, error) {
	log.Printf("Sending message with producer: %s", req.ProducerId)

	// 获取生产者
	s.mu.RLock()
	producerInfo, exists := s.producers[req.ProducerId]
	s.mu.RUnlock()

	if !exists {
		return &proto.SendMessageResponse{
			Success: false,
			Message: "Producer not found",
		}, status.Error(codes.NotFound, "Producer not found")
	}

	// 更新生产者最后活跃时间
	s.mu.Lock()
	if info, ok := s.producers[req.ProducerId]; ok {
		info.LastActive = time.Now()
	}
	s.mu.Unlock()

	// 构建消息
	msg := &primitive.Message{
		Topic: producerInfo.Topic,
		Body:  []byte(req.MessageBody),
	}

	// 设置Tag
	if req.Tag != "" {
		msg.WithTag(req.Tag)
	}

	// 处理消息属性
	if req.Properties != nil {
		// 设置自定义属性
		for k, v := range req.Properties.Properties {
			msg.WithProperty(k, v)
		}

		// 设置消息Key
		if req.Properties.MessageKey != "" {
			msg.WithKeys([]string{req.Properties.MessageKey})
		}

		// 设置分区键(顺序消息)
		if req.Properties.ShardingKey != "" {
			msg.WithShardingKey(req.Properties.ShardingKey)
		}

		// 设置延时投递时间 - 支持字节云的任意精度延时消息
		if req.Properties.StartDeliverTime > 0 {
			// 使用字节云的 __STARTDELIVERTIME 属性实现任意精度延时
			msg.WithProperty("__STARTDELIVERTIME", fmt.Sprintf("%d", req.Properties.StartDeliverTime))
			log.Printf("Using arbitrary precision delay with __STARTDELIVERTIME: %d", req.Properties.StartDeliverTime)
		}
	}

	// 发送消息
	result, err := producerInfo.Producer.SendSync(ctx, msg)
	if err != nil {
		metrics.GlobalMetrics.IncErrorCount()
		return &proto.SendMessageResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to send message: %v", err),
		}, nil
	}

	// 增加发送消息计数
	metrics.GlobalMetrics.IncMessagesSent()

	return &proto.SendMessageResponse{
		Success:       true,
		Message:       "Message sent successfully",
		MessageId:     result.MsgID,
		ReceiptHandle: result.MsgID,
	}, nil
}

// SendOrderedMessage 发送顺序消息
func (s *RocketMQProxyService) SendOrderedMessage(ctx context.Context, req *proto.SendOrderedMessageRequest) (*proto.SendMessageResponse, error) {
	log.Printf("Sending ordered message with producer: %s, sharding key: %s", req.ProducerId, req.ShardingKey)

	// 获取生产者
	s.mu.RLock()
	producerInfo, exists := s.producers[req.ProducerId]
	s.mu.RUnlock()

	if !exists {
		return &proto.SendMessageResponse{
			Success: false,
			Message: "Producer not found",
		}, status.Error(codes.NotFound, "Producer not found")
	}

	// 构建顺序消息
	msg := &primitive.Message{
		Topic: producerInfo.Topic,
		Body:  []byte(req.MessageBody),
	}

	// 设置Tag
	if req.Tag != "" {
		msg.WithTag(req.Tag)
	}

	// 设置分区键（顺序消息的关键）
	if req.ShardingKey != "" {
		msg.WithShardingKey(req.ShardingKey)
	}

	// 处理消息属性
	if req.Properties != nil {
		for k, v := range req.Properties.Properties {
			msg.WithProperty(k, v)
		}
		if req.Properties.MessageKey != "" {
			msg.WithKeys([]string{req.Properties.MessageKey})
		}
	}

	// 发送顺序消息
	result, err := producerInfo.Producer.SendSync(ctx, msg)
	if err != nil {
		metrics.GlobalMetrics.IncErrorCount()
		return &proto.SendMessageResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to send ordered message: %v", err),
		}, nil
	}

	metrics.GlobalMetrics.IncMessagesSent()
	log.Printf("✅ Ordered message sent successfully: %s", result.MsgID)

	return &proto.SendMessageResponse{
		Success:       true,
		Message:       "Ordered message sent successfully",
		MessageId:     result.MsgID,
		ReceiptHandle: result.MsgID,
	}, nil
}

// SendDelayMessage 发送延迟消息
func (s *RocketMQProxyService) SendDelayMessage(ctx context.Context, req *proto.SendDelayMessageRequest) (*proto.SendMessageResponse, error) {
	log.Printf("Sending delay message with producer: %s, delay level: %d, deliver time: %d",
		req.ProducerId, req.DelayTimeLevel, req.StartDeliverTime)

	// 获取生产者
	s.mu.RLock()
	producerInfo, exists := s.producers[req.ProducerId]
	s.mu.RUnlock()

	if !exists {
		return &proto.SendMessageResponse{
			Success: false,
			Message: "Producer not found",
		}, status.Error(codes.NotFound, "Producer not found")
	}

	// 构建延迟消息
	msg := &primitive.Message{
		Topic: producerInfo.Topic,
		Body:  []byte(req.MessageBody),
	}

	// 设置Tag
	if req.Tag != "" {
		msg.WithTag(req.Tag)
	}

	// 设置延迟时间 - 支持字节云的任意精度延时消息
	if req.StartDeliverTime > 0 {
		// 使用字节云的 __STARTDELIVERTIME 属性实现任意精度延时
		msg.WithProperty("__STARTDELIVERTIME", fmt.Sprintf("%d", req.StartDeliverTime))
		log.Printf("Using arbitrary precision delay with __STARTDELIVERTIME: %d", req.StartDeliverTime)
	} else if req.DelayTimeLevel > 0 {
		// 兼容传统的延迟等级 (1-18)，使用RocketMQ原生延迟等级
		msg.WithDelayTimeLevel(int(req.DelayTimeLevel))
		log.Printf("Using traditional delay level: %d", req.DelayTimeLevel)
	}

	// 处理消息属性
	if req.Properties != nil {
		for k, v := range req.Properties.Properties {
			msg.WithProperty(k, v)
		}
		if req.Properties.MessageKey != "" {
			msg.WithKeys([]string{req.Properties.MessageKey})
		}
	}

	// 发送延迟消息
	result, err := producerInfo.Producer.SendSync(ctx, msg)
	if err != nil {
		metrics.GlobalMetrics.IncErrorCount()
		return &proto.SendMessageResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to send delay message: %v", err),
		}, nil
	}

	metrics.GlobalMetrics.IncMessagesSent()
	log.Printf("✅ Delay message sent successfully: %s", result.MsgID)

	return &proto.SendMessageResponse{
		Success:       true,
		Message:       "Delay message sent successfully",
		MessageId:     result.MsgID,
		ReceiptHandle: result.MsgID,
	}, nil
}

// SendTransactionMessage 发送事务消息
func (s *RocketMQProxyService) SendTransactionMessage(ctx context.Context, req *proto.SendTransactionMessageRequest) (*proto.SendTransactionMessageResponse, error) {
	log.Printf("Sending transaction message with producer: %s, immunity time: %d",
		req.ProducerId, req.TransCheckImmunityTime)

	// 获取生产者
	s.mu.RLock()
	producerInfo, exists := s.producers[req.ProducerId]
	s.mu.RUnlock()

	if !exists {
		return &proto.SendTransactionMessageResponse{
			Success: false,
			Message: "Producer not found",
		}, status.Error(codes.NotFound, "Producer not found")
	}

	// 构建事务消息
	msg := &primitive.Message{
		Topic: producerInfo.Topic,
		Body:  []byte(req.MessageBody),
	}

	// 设置Tag
	if req.Tag != "" {
		msg.WithTag(req.Tag)
	}

	// 设置事务回查免疫时间
	if req.TransCheckImmunityTime > 0 {
		// RocketMQ Go SDK中事务消息的处理方式
		msg.WithProperty("__STARTDELIVERTIME", fmt.Sprintf("%d", time.Now().Add(time.Duration(req.TransCheckImmunityTime)*time.Second).UnixMilli()))
	}

	// 处理消息属性
	if req.Properties != nil {
		for k, v := range req.Properties.Properties {
			msg.WithProperty(k, v)
		}
		if req.Properties.MessageKey != "" {
			msg.WithKeys([]string{req.Properties.MessageKey})
		}
	}

	// 发送事务消息（这里使用普通发送，实际事务逻辑需要更复杂的处理）
	result, err := producerInfo.Producer.SendSync(ctx, msg)
	if err != nil {
		metrics.GlobalMetrics.IncErrorCount()
		return &proto.SendTransactionMessageResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to send transaction message: %v", err),
		}, nil
	}

	metrics.GlobalMetrics.IncMessagesSent()
	log.Printf("✅ Transaction message sent successfully: %s", result.MsgID)

	return &proto.SendTransactionMessageResponse{
		Success:       true,
		Message:       "Transaction message sent successfully",
		MessageId:     result.MsgID,
		ReceiptHandle: result.MsgID,
		TransactionId: fmt.Sprintf("trans_%s_%d", result.MsgID, time.Now().UnixNano()),
	}, nil
}

// CreateConsumer 创建消费者 - 支持集群消费模式和消费者复用，支持同一GroupID订阅多个Topic
func (s *RocketMQProxyService) CreateConsumer(ctx context.Context, req *proto.CreateConsumerRequest) (*proto.CreateConsumerResponse, error) {
	log.Printf("Creating consumer for topic: %s, group: %s (cluster mode with consumer reuse)", req.Topic, req.GroupId)

	// 生成消费者连接key，按GroupID+Endpoint复用，支持多Topic订阅和不同RocketMQ实例
	consumerKey := ConsumerKey{
		Endpoint:    req.Endpoint,
		AccessKeyId: req.AccessKeyId,
		InstanceId:  req.InstanceId,
		GroupID:     req.GroupId,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查是否已有相同GroupID的消费者可以复用
	if existingConsumerID, exists := s.sharedConsumers[consumerKey]; exists {
		if consumerInfo, found := s.consumers[existingConsumerID]; found {
			// 检查现有消费者是否还活跃
			timeSinceLastActive := time.Since(consumerInfo.LastActive)

			if timeSinceLastActive < 5*time.Minute {
				// 检查是否已经订阅了这个Topic
				if !consumerInfo.Topics[req.Topic] {
					// 为现有消费者添加新Topic订阅
					selector := consumer.MessageSelector{
						Type:       consumer.TAG,
						Expression: req.TagExpression,
					}

					err := consumerInfo.Consumer.Subscribe(req.Topic, selector, func(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
						select {
						case <-ctx.Done():
							return consumer.ConsumeRetryLater, fmt.Errorf("consumer context cancelled")
						default:
						}

						for _, msg := range msgs {
							metrics.GlobalMetrics.IncMessagesReceived()

							protoMsg := &proto.Message{
								MessageId:      msg.MsgId,
								ReceiptHandle:  msg.MsgId,
								MessageBody:    string(msg.Body),
								Tag:            msg.GetTags(),
								Properties:     msg.GetProperties(),
								BornTimestamp:  msg.BornTimestamp,
								ReconsumeTimes: int32(msg.ReconsumeTimes),
							}

							select {
							case consumerInfo.MessageChan <- protoMsg:
								log.Printf("Message sent to channel: %s (topic: %s)", msg.MsgId, msg.Topic)
							case <-time.After(5 * time.Second):
								log.Printf("⚠️ Message channel send timeout, will retry later: %s", msg.MsgId)
								metrics.GlobalMetrics.IncChannelFullEvents()
								return consumer.ConsumeRetryLater, fmt.Errorf("message channel timeout - will retry")
							}
						}
						return consumer.ConsumeSuccess, nil
					})

					if err != nil {
						log.Printf("⚠️ Failed to add topic subscription %s to existing consumer: %v", req.Topic, err)
						// 如果添加订阅失败，继续创建新的消费者
					} else {
						// 成功添加Topic订阅
						consumerInfo.Topics[req.Topic] = true
						log.Printf("✅ Added topic %s to existing consumer group %s", req.Topic, req.GroupId)
					}
				}

				// 消费者仍然活跃，复用现有消费者
				newConsumerID := uuid.New().String()

				// 直接引用同一个ConsumerInfo实例，而不是创建新的
				s.consumers[newConsumerID] = consumerInfo
				consumerInfo.RefCount++
				consumerInfo.LastActive = time.Now()

				topicList := make([]string, 0, len(consumerInfo.Topics))
				for topic := range consumerInfo.Topics {
					topicList = append(topicList, topic)
				}

				log.Printf("✅ Reusing consumer: ID=%s, Group=%s, Topics=%v, RefCount=%d",
					newConsumerID, req.GroupId, topicList, consumerInfo.RefCount)
				return &proto.CreateConsumerResponse{
					Success:    true,
					Message:    fmt.Sprintf("Consumer reused (ref: %d) - cluster mode, topics: %v", consumerInfo.RefCount, topicList),
					ConsumerId: newConsumerID,
				}, nil
			} else {
				// 消费者不活跃，清理后重新创建
				log.Printf("🔄 Found inactive shared consumer, will recreate: %s (inactive for %v)", existingConsumerID, timeSinceLastActive)
				s.cleanupSharedConsumerInternal(consumerKey, existingConsumerID)
			}
		}
	}

	// 清理其他不活跃的消费者（但不影响当前创建）
	var inactiveConsumers []string
	for consumerID, existingConsumer := range s.consumers {
		if existingConsumer.GroupID == req.GroupId {
			timeSinceLastActive := time.Since(existingConsumer.LastActive)
			if timeSinceLastActive > 5*time.Minute {
				inactiveConsumers = append(inactiveConsumers, consumerID)
				log.Printf("🔄 Found long-inactive consumer, will clean up: %s (inactive for %v)", consumerID, timeSinceLastActive)
			}
		}
	}

	// 释放锁后清理不活跃的消费者
	s.mu.Unlock()
	for _, consumerID := range inactiveConsumers {
		if err := s.CleanupConsumerByID(consumerID); err != nil {
			log.Printf("⚠️ Error cleaning up inactive consumer: %v", err)
		}
	}
	s.mu.Lock()

	// 创建新的消费者
	consumerGroup := req.GroupId

	// 创建带取消功能的上下文
	consumerCtx, cancelFunc := context.WithCancel(context.Background())

	// 创建消费者配置 - 明确启用集群消费模式
	opts := []consumer.Option{
		consumer.WithNsResolver(primitive.NewPassthroughResolver([]string{req.Endpoint})),
		consumer.WithCredentials(primitive.Credentials{
			AccessKey: req.AccessKeyId,
			SecretKey: req.AccessKeySecret,
		}),
		consumer.WithGroupName(consumerGroup),
		consumer.WithConsumeFromWhere(consumer.ConsumeFromLastOffset),
		consumer.WithConsumerModel(consumer.Clustering),
		consumer.WithConsumerPullTimeout(s.config.PullTimeout),
		consumer.WithPullInterval(s.config.PullInterval),
		consumer.WithMaxReconsumeTimes(3),
		consumer.WithConsumeMessageBatchMaxSize(32),
	}

	// 创建消费者
	c, err := rocketmq.NewPushConsumer(opts...)
	if err != nil {
		cancelFunc()
		return &proto.CreateConsumerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to create consumer: %v", err),
		}, nil
	}

	// 生成唯一ID
	consumerID := uuid.New().String()

	// 创建消息通道
	messageChan := make(chan *proto.Message, s.config.MessageBufferSize)

	// 订阅主题
	selector := consumer.MessageSelector{
		Type:       consumer.TAG,
		Expression: req.TagExpression,
	}

	err = c.Subscribe(req.Topic, selector, func(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
		select {
		case <-consumerCtx.Done():
			return consumer.ConsumeRetryLater, fmt.Errorf("consumer context cancelled")
		default:
		}

		for _, msg := range msgs {
			metrics.GlobalMetrics.IncMessagesReceived()

			protoMsg := &proto.Message{
				MessageId:      msg.MsgId,
				ReceiptHandle:  msg.MsgId,
				MessageBody:    string(msg.Body),
				Tag:            msg.GetTags(),
				Properties:     msg.GetProperties(),
				BornTimestamp:  msg.BornTimestamp,
				ReconsumeTimes: int32(msg.ReconsumeTimes),
			}

			select {
			case messageChan <- protoMsg:
				log.Printf("Message sent to channel: %s (topic: %s)", msg.MsgId, msg.Topic)
			case <-consumerCtx.Done():
				log.Printf("Consumer context cancelled, dropping message: %s", msg.MsgId)
				return consumer.ConsumeRetryLater, fmt.Errorf("consumer context cancelled")
			case <-time.After(5 * time.Second):
				log.Printf("⚠️ Message channel send timeout, will retry later: %s", msg.MsgId)
				metrics.GlobalMetrics.IncChannelFullEvents()
				return consumer.ConsumeRetryLater, fmt.Errorf("message channel timeout - will retry")
			}
		}
		return consumer.ConsumeSuccess, nil
	})

	if err != nil {
		cancelFunc()
		return &proto.CreateConsumerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to subscribe topic: %v", err),
		}, nil
	}

	// 启动消费者
	err = c.Start()
	if err != nil {
		cancelFunc()
		return &proto.CreateConsumerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to start consumer: %v", err),
		}, nil
	}

	// 存储消费者信息
	consumerInfo := &ConsumerInfo{
		Consumer:    c,
		Topics:      map[string]bool{req.Topic: true}, // 初始化Topics映射
		GroupID:     consumerGroup,
		Endpoint:    req.Endpoint,
		InstanceId:  req.InstanceId,
		MessageChan: messageChan,
		CancelFunc:  cancelFunc,
		LastActive:  time.Now(),
		RefCount:    1,
		CreatedAt:   time.Now(),
	}

	s.consumers[consumerID] = consumerInfo

	// 记录复用映射
	s.sharedConsumers[consumerKey] = consumerID

	// 统计同组消费者数量
	sameGroupCount := 0
	for _, consumer := range s.consumers {
		if consumer.GroupID == consumerGroup {
			sameGroupCount++
		}
	}

	// 增加消费者计数
	metrics.GlobalMetrics.IncActiveConsumers()

	log.Printf("✅ New consumer created: ID=%s, Group=%s, Topic=%s (cluster mode: %d consumers in group)",
		consumerID, consumerGroup, req.Topic, sameGroupCount)
	return &proto.CreateConsumerResponse{
		Success:    true,
		Message:    fmt.Sprintf("Consumer created successfully for group: %s (cluster mode: %d consumers)", consumerGroup, sameGroupCount),
		ConsumerId: consumerID,
	}, nil
}

// ReceiveMessages 接收消息(流式)
func (s *RocketMQProxyService) ReceiveMessages(req *proto.ReceiveMessagesRequest, stream proto.RocketMQProxy_ReceiveMessagesServer) error {
	log.Printf("🎬 Starting message stream for consumer: %s", req.ConsumerId)

	// 获取消费者
	s.mu.RLock()
	consumerInfo, exists := s.consumers[req.ConsumerId]
	s.mu.RUnlock()

	if !exists {
		return status.Error(codes.NotFound, "Consumer not found")
	}

	// 更新最后活跃时间
	s.mu.Lock()
	if info, ok := s.consumers[req.ConsumerId]; ok {
		info.LastActive = time.Now()
	}
	s.mu.Unlock()

	// 创建受控的上下文
	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	// 缩短心跳间隔，更快检测断开连接
	heartbeatTicker := time.NewTicker(10 * time.Second)
	defer heartbeatTicker.Stop()

	// Stream状态监控 - 使用受控的goroutine
	streamDone := make(chan bool, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("⚠️ Stream monitor goroutine recovered from panic: %v", r)
			}
		}()
		select {
		case <-ctx.Done():
			return
		case <-stream.Context().Done():
			select {
			case streamDone <- true:
			default:
			}
			return
		}
	}()

	// 确保在函数退出时清理消费者 - 直接调用，不使用额外goroutine
	defer func() {
		log.Printf("🔚 Stream ended for consumer: %s, reason: client disconnection", req.ConsumerId)

		// 直接清理，避免goroutine泄漏
		if err := s.CleanupConsumerByID(req.ConsumerId); err != nil {
			log.Printf("❌ Error cleaning up consumer %s: %v", req.ConsumerId, err)
		} else {
			log.Printf("✅ Consumer %s cleaned up immediately after stream end", req.ConsumerId)
		}
	}()

	log.Printf("📡 Consumer %s ready to receive messages (heartbeat: 10s)", req.ConsumerId)

	// 从消息通道读取并发送消息
	for {
		select {
		case <-streamDone:
			log.Printf("📴 Client disconnected for consumer: %s", req.ConsumerId)
			return nil

		case <-ctx.Done():
			log.Printf("📴 Stream context done for consumer: %s", req.ConsumerId)
			return nil

		case msg, ok := <-consumerInfo.MessageChan:
			if !ok {
				// 通道已关闭
				log.Printf("📨 Message channel closed for consumer: %s", req.ConsumerId)
				return nil
			}

			if err := stream.Send(msg); err != nil {
				log.Printf("❌ Failed to send message to stream for consumer %s: %v", req.ConsumerId, err)
				return err
			}

			// 更新最后活跃时间
			s.mu.Lock()
			if info, ok := s.consumers[req.ConsumerId]; ok {
				info.LastActive = time.Now()
			}
			s.mu.Unlock()

		case <-heartbeatTicker.C:
			// 发送心跳，更新活跃时间
			s.mu.Lock()
			if info, ok := s.consumers[req.ConsumerId]; ok {
				info.LastActive = time.Now()
				log.Printf("💓 Heartbeat for consumer: %s (group: %s)", req.ConsumerId, info.GroupID)
			}
			s.mu.Unlock()
		}
	}
}

// AckMessage 确认消息
func (s *RocketMQProxyService) AckMessage(ctx context.Context, req *proto.AckMessageRequest) (*proto.AckMessageResponse, error) {
	log.Printf("Acking message for consumer: %s", req.ConsumerId)

	// 在实际实现中，这里应该确认具体的消息
	// RocketMQ的确认是在消费回调中处理的，这里可以记录确认状态

	return &proto.AckMessageResponse{
		Success: true,
		Message: "Message acknowledged",
	}, nil
}

// CleanupProducer 清理生产者 (gRPC API)
func (s *RocketMQProxyService) CleanupProducer(ctx context.Context, req *proto.CleanupProducerRequest) (*proto.CleanupProducerResponse, error) {
	log.Printf("🧹 Manual producer cleanup request - ProducerID: %s, Topic: %s, Endpoint: %s", req.ProducerId, req.Topic, req.Endpoint)

	cleanedCount := int32(0)
	var cleanupErrors []string

	s.mu.Lock()
	var toCleanup []string

	if req.ProducerId != "" {
		// 通过生产者ID清理
		if _, exists := s.producers[req.ProducerId]; exists {
			toCleanup = append(toCleanup, req.ProducerId)
		}
	} else if req.Topic != "" {
		// 通过主题清理（可选配合Endpoint）
		for producerID, producerInfo := range s.producers {
			if producerInfo.Topic == req.Topic {
				if req.Endpoint == "" || producerInfo.Endpoint == req.Endpoint {
					toCleanup = append(toCleanup, producerID)
				}
			}
		}
	}
	s.mu.Unlock()

	// 执行清理
	for _, producerID := range toCleanup {
		if err := s.cleanupProducerInternal(producerID); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Sprintf("Producer %s: %v", producerID, err))
		} else {
			cleanedCount++
			log.Printf("✅ Manually cleaned up producer: %s", producerID)
		}
	}

	// 构建响应消息
	var message string
	success := len(cleanupErrors) == 0

	if cleanedCount > 0 {
		message = fmt.Sprintf("Successfully cleaned up %d producer(s)", cleanedCount)
		if len(cleanupErrors) > 0 {
			message += fmt.Sprintf(", but %d failed: %v", len(cleanupErrors), cleanupErrors)
		}
	} else {
		if req.ProducerId != "" {
			message = "No producer found with the specified ID"
		} else {
			message = "No producers found matching the criteria"
		}
	}

	return &proto.CleanupProducerResponse{
		Success:      success,
		Message:      message,
		CleanedCount: cleanedCount,
	}, nil
}

// cleanupProducerInternal 内部生产者清理方法
func (s *RocketMQProxyService) cleanupProducerInternal(producerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	producerInfo, exists := s.producers[producerID]
	if !exists {
		return fmt.Errorf("producer not found: %s", producerID)
	}

	log.Printf("🧹 Cleaning up producer: ID=%s, Topic=%s, Endpoint=%s, RefCount=%d",
		producerID, producerInfo.Topic, producerInfo.Endpoint, producerInfo.RefCount)

	// 减少引用计数
	producerInfo.RefCount--

	// 如果引用计数大于0，只是移除当前ID，但保持生产者实例
	if producerInfo.RefCount > 0 {
		delete(s.producers, producerID)
		log.Printf("✅ Producer instance preserved with RefCount=%d", producerInfo.RefCount)
		return nil
	}

	// 引用计数为0，真正清理资源
	if err := producerInfo.Producer.Shutdown(); err != nil {
		log.Printf("⚠️ Error shutting down producer %s: %v", producerID, err)
	}

	// 从映射中删除
	delete(s.producers, producerID)

	// 清理共享映射 - 删除所有指向该producerID的映射
	keysToDelete := make([]ConnectionKey, 0)
	for key, id := range s.sharedProducers {
		if id == producerID {
			keysToDelete = append(keysToDelete, key)
		}
	}
	for _, key := range keysToDelete {
		delete(s.sharedProducers, key)
	}

	// 减少生产者计数
	metrics.GlobalMetrics.DecActiveProducers()

	log.Printf("✅ Producer cleanup completed: %s", producerID)
	return nil
}

// CleanupInactiveProducers 清理不活跃的生产者（定时任务）
func (s *RocketMQProxyService) CleanupInactiveProducers(timeout time.Duration) {
	var toCleanup []string

	// 只在读取时获取锁
	func() {
		s.mu.RLock()
		defer s.mu.RUnlock()

		now := time.Now()
		for producerID, producerInfo := range s.producers {
			if now.Sub(producerInfo.LastActive) > timeout {
				toCleanup = append(toCleanup, producerID)
			}
		}
	}()

	// 释放锁后清理
	for _, producerID := range toCleanup {
		log.Printf("🕒 Cleaning up inactive producer: %s", producerID)
		if err := s.cleanupProducerInternal(producerID); err != nil {
			log.Printf("❌ Error cleaning up inactive producer %s: %v", producerID, err)
		}
	}
}

// CleanupConsumerByID 清理指定的消费者（外部调用，包含锁管理）
func (s *RocketMQProxyService) CleanupConsumerByID(consumerID string) error {
	return s.cleanupConsumerInternal(consumerID)
}

// cleanupConsumerInternal 内部清理方法（支持引用计数）
func (s *RocketMQProxyService) cleanupConsumerInternal(consumerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	consumerInfo, exists := s.consumers[consumerID]
	if !exists {
		return fmt.Errorf("consumer not found: %s", consumerID)
	}

	log.Printf("🧹 Cleaning up consumer: ID=%s, Group=%s, Topics=%v, RefCount=%d",
		consumerID, consumerInfo.GroupID, getTopicList(consumerInfo.Topics), consumerInfo.RefCount)

	// 减少引用计数
	consumerInfo.RefCount--

	// 如果引用计数大于0，只是移除当前ID，但保持消费者实例
	if consumerInfo.RefCount > 0 {
		delete(s.consumers, consumerID)
		log.Printf("✅ Consumer instance preserved with RefCount=%d", consumerInfo.RefCount)
		return nil
	}

	// 引用计数为0，真正清理资源
	// 取消消费者上下文
	if consumerInfo.CancelFunc != nil {
		consumerInfo.CancelFunc()
	}

	// 停止消费者
	if err := consumerInfo.Consumer.Shutdown(); err != nil {
		log.Printf("⚠️ Error shutting down consumer %s: %v", consumerID, err)
	}

	// 关闭消息通道
	if consumerInfo.MessageChan != nil {
		close(consumerInfo.MessageChan)
	}

	// 找到所有指向同一个ConsumerInfo实例的ID并清理
	var idsToDelete []string
	for id, info := range s.consumers {
		if info == consumerInfo {
			idsToDelete = append(idsToDelete, id)
		}
	}

	// 删除所有相关的消费者ID
	for _, id := range idsToDelete {
		delete(s.consumers, id)
		log.Printf("🧹 Removed consumer ID: %s", id)
	}

	// 清理共享映射 - 删除所有指向该consumerID的映射
	keysToDelete := make([]ConsumerKey, 0)
	for key, id := range s.sharedConsumers {
		if id == consumerID {
			keysToDelete = append(keysToDelete, key)
		}
	}
	for _, key := range keysToDelete {
		delete(s.sharedConsumers, key)
	}

	// 减少消费者计数
	metrics.GlobalMetrics.DecActiveConsumers()

	log.Printf("✅ Consumer cleanup completed: %s (cleaned %d IDs)", consumerID, len(idsToDelete))
	return nil
}

// CleanupInactiveConsumers 清理不活跃的消费者（定时任务）
func (s *RocketMQProxyService) CleanupInactiveConsumers(timeout time.Duration) {
	var toCleanup []string

	// 只在读取时获取锁，避免双重解锁问题
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
		log.Printf("🕒 Cleaning up inactive consumer: %s", consumerID)
		if err := s.CleanupConsumerByID(consumerID); err != nil {
			log.Printf("❌ Error cleaning up inactive consumer %s: %v", consumerID, err)
		}
	}
}

// CleanupConsumer 清理消费者 (gRPC API)
func (s *RocketMQProxyService) CleanupConsumer(ctx context.Context, req *proto.CleanupConsumerRequest) (*proto.CleanupConsumerResponse, error) {
	log.Printf("🧹 Manual cleanup request - ConsumerID: %s, GroupID: %s, Topic: %s", req.ConsumerId, req.GroupId, req.Topic)

	cleanedCount := int32(0)
	var cleanupErrors []string

	s.mu.Lock()
	var toCleanup []string

	if req.ConsumerId != "" {
		// 通过消费者ID清理
		if _, exists := s.consumers[req.ConsumerId]; exists {
			toCleanup = append(toCleanup, req.ConsumerId)
		}
	} else if req.GroupId != "" {
		// 通过组名清理（可选配合Topic）
		for consumerID, consumerInfo := range s.consumers {
			if consumerInfo.GroupID == req.GroupId {
				if req.Topic == "" || consumerInfo.Topics[req.Topic] {
					toCleanup = append(toCleanup, consumerID)
				}
			}
		}
	}
	s.mu.Unlock()

	// 执行清理
	for _, consumerID := range toCleanup {
		if err := s.cleanupConsumerInternal(consumerID); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Sprintf("Consumer %s: %v", consumerID, err))
		} else {
			cleanedCount++
			log.Printf("✅ Manually cleaned up consumer: %s", consumerID)
		}
	}

	// 构建响应消息
	var message string
	success := len(cleanupErrors) == 0

	if cleanedCount > 0 {
		message = fmt.Sprintf("Successfully cleaned up %d consumer(s)", cleanedCount)
		if len(cleanupErrors) > 0 {
			message += fmt.Sprintf(", but %d failed: %v", len(cleanupErrors), cleanupErrors)
		}
	} else {
		if req.ConsumerId != "" {
			message = "No consumer found with the specified ID"
		} else {
			message = "No consumers found matching the criteria"
		}
	}

	return &proto.CleanupConsumerResponse{
		Success:      success,
		Message:      message,
		CleanedCount: cleanedCount,
	}, nil
}

// HealthCheck 健康检查
func (s *RocketMQProxyService) HealthCheck(ctx context.Context, req *proto.HealthCheckRequest) (*proto.HealthCheckResponse, error) {
	return &proto.HealthCheckResponse{
		Healthy: true,
		Message: "Service is healthy",
	}, nil
}

// calculateDelayLevel 计算延时等级（仅用于兼容性场景）
// 注意：字节云支持任意精度延时，优先使用 __STARTDELIVERTIME 属性
func calculateDelayLevel(deliverTime int64) int {
	now := time.Now().UnixMilli()
	delay := deliverTime - now

	// RocketMQ延时等级: 1s 5s 10s 30s 1m 2m 3m 4m 5m 6m 7m 8m 9m 10m 20m 30m 1h 2h
	delayLevels := []int64{
		1000, 5000, 10000, 30000, 60000, 120000, 180000, 240000,
		300000, 360000, 420000, 480000, 540000, 600000, 1200000,
		1800000, 3600000, 7200000,
	}

	for i, level := range delayLevels {
		if delay <= level {
			return i + 1
		}
	}

	return 18 // 最大延时等级
}

// ShutdownAllProducers 优雅关闭所有生产者 - 用于服务停止时
func (s *RocketMQProxyService) ShutdownAllProducers() {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Printf("🛑 Shutting down all producers...")

	// 记录唯一的生产者实例，避免重复关闭
	shutdownProducers := make(map[rocketmq.Producer]bool)

	for producerID, producerInfo := range s.producers {
		if !shutdownProducers[producerInfo.Producer] {
			log.Printf("🧹 Shutting down producer: ID=%s, Topic=%s, RefCount=%d",
				producerID, producerInfo.Topic, producerInfo.RefCount)

			if err := producerInfo.Producer.Shutdown(); err != nil {
				log.Printf("⚠️ Error shutting down producer %s: %v", producerID, err)
			}
			shutdownProducers[producerInfo.Producer] = true
		}
	}

	// 清空所有映射
	s.producers = make(map[string]*ProducerInfo)
	s.sharedProducers = make(map[ConnectionKey]string)

	// 重置计数器
	metrics.GlobalMetrics.ResetActiveProducers()

	log.Printf("✅ All producers shutdown completed")
}

// ShutdownAllConsumers 优雅关闭所有消费者 - 用于服务停止时
func (s *RocketMQProxyService) ShutdownAllConsumers() {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Printf("🛑 Shutting down all consumers...")

	// 记录唯一的消费者实例，避免重复关闭
	shutdownConsumers := make(map[rocketmq.PushConsumer]bool)

	for consumerID, consumerInfo := range s.consumers {
		if !shutdownConsumers[consumerInfo.Consumer] {
			log.Printf("🧹 Shutting down consumer: ID=%s, Group=%s, Topics=%v, RefCount=%d",
				consumerID, consumerInfo.GroupID, getTopicList(consumerInfo.Topics), consumerInfo.RefCount)

			// 取消消费者上下文
			if consumerInfo.CancelFunc != nil {
				consumerInfo.CancelFunc()
			}

			// 停止消费者
			if err := consumerInfo.Consumer.Shutdown(); err != nil {
				log.Printf("⚠️ Error shutting down consumer %s: %v", consumerID, err)
			}

			// 关闭消息通道
			if consumerInfo.MessageChan != nil {
				close(consumerInfo.MessageChan)
			}

			shutdownConsumers[consumerInfo.Consumer] = true
		}
	}

	// 清空所有映射
	s.consumers = make(map[string]*ConsumerInfo)
	s.sharedConsumers = make(map[ConsumerKey]string)

	// 重置计数器
	metrics.GlobalMetrics.ResetActiveConsumers()

	log.Printf("✅ All consumers shutdown completed")
}

// ValidateAndFixProducerRefCounts 验证并修复生产者引用计数不一致问题
func (s *RocketMQProxyService) ValidateAndFixProducerRefCounts() {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Printf("🔍 Validating producer reference counts...")

	// 统计每个生产者实例的实际引用数
	actualRefCounts := make(map[rocketmq.Producer]int)

	for _, producerInfo := range s.producers {
		actualRefCounts[producerInfo.Producer]++
	}

	// 检查并修复引用计数不一致
	fixedCount := 0
	for _, producerInfo := range s.producers {
		expectedCount := actualRefCounts[producerInfo.Producer]
		if producerInfo.RefCount != expectedCount {
			log.Printf("⚠️ Reference count mismatch detected: Producer has RefCount=%d, but actual references=%d. Fixing...",
				producerInfo.RefCount, expectedCount)
			producerInfo.RefCount = expectedCount
			fixedCount++
		}
	}

	if fixedCount > 0 {
		log.Printf("🔧 Fixed %d producer reference count mismatches", fixedCount)
	} else {
		log.Printf("✅ All producer reference counts are consistent")
	}
}

// cleanupSharedConsumerInternal 清理共享消费者映射
func (s *RocketMQProxyService) cleanupSharedConsumerInternal(consumerKey ConsumerKey, consumerID string) {
	// 从共享映射中删除
	delete(s.sharedConsumers, consumerKey)

	// 如果消费者还存在，也清理掉
	if consumerInfo, exists := s.consumers[consumerID]; exists {
		log.Printf("🧹 Cleaning up shared consumer: ID=%s, Group=%s, Topics=%v",
			consumerID, consumerInfo.GroupID, getTopicList(consumerInfo.Topics))

		// 取消消费者上下文
		if consumerInfo.CancelFunc != nil {
			consumerInfo.CancelFunc()
		}

		// 停止消费者
		if err := consumerInfo.Consumer.Shutdown(); err != nil {
			log.Printf("⚠️ Error shutting down shared consumer %s: %v", consumerID, err)
		}

		// 关闭消息通道
		if consumerInfo.MessageChan != nil {
			close(consumerInfo.MessageChan)
		}

		// 从map中移除
		delete(s.consumers, consumerID)

		// 减少消费者计数
		metrics.GlobalMetrics.DecActiveConsumers()
	}
}

// getTopicList 获取消费者订阅的Topic列表
func getTopicList(topics map[string]bool) string {
	topicList := make([]string, 0, len(topics))
	for topic := range topics {
		topicList = append(topicList, topic)
	}
	return fmt.Sprintf("%v", topicList)
}
