# RocketMQ gRPC Proxy

这是一个Go语言实现的RocketMQ代理服务，通过gRPC协议为Node.js应用提供RocketMQ访问能力。

## 架构设计

```
Node.js App -> gRPC Client -> Go Proxy Server -> RocketMQ (TCP)
```

- **Go Proxy Server**: 使用官方`github.com/apache/rocketmq-client-go/v2` SDK
- **gRPC协议**: 提供高性能、类型安全的通信
- **Node.js Client**: 兼容现有API的客户端SDK

## 功能特性

✅ **生产者功能**
- 普通消息发送
- 延时/定时消息
- 顺序消息（分区键）
- 事务消息支持
- 自定义消息属性

✅ **消费者功能**
- 推送模式消费
- 标签过滤 (tag1||tag2)
- 流式消息接收
- 消息确认机制

✅ **其他特性**
- 健康检查
- 连接管理
- 错误处理

## 快速开始

### 1. 启动Go代理服务

```bash
# 构建并运行
make build
./bin/rocketmq-proxy

# 或者直接运行
make run
```

服务将在 `:50051` 端口启动。

### 2. 消费者组连接管理 🔄

**新增功能：支持预定义消费者组的自动重连**

针对字节云RocketMQ中预定义的消费者组，本代理服务提供了以下特性：

- ✅ **自动清理断开的连接** - 当Node.js客户端断开时，消费者连接会在30秒内自动清理
- ✅ **智能重连检测** - 允许相同组名的消费者在旧连接断开后重新连接  
- ✅ **快速故障检测** - 10秒心跳间隔，1分钟超时清理
- ✅ **手动清理API** - 提供gRPC API手动清理卡住的连接

**常见场景**:
```bash
# 场景1: Node.js客户端Ctrl+C关闭后重启
# 🎯 修复前: 报错 "Consumer group already exists"
# ✅ 修复后: 自动清理旧连接，允许重新连接

# 场景2: 查看消费者状态
curl http://localhost:8080/metrics

# 场景3: 手动清理指定消费者组（如果需要）
# grpcurl -plaintext localhost:50051 rocketmq.RocketMQProxy.CleanupConsumer \
#   -d '{"group_id": "your_group_name", "topic": "your_topic"}'
```

**重要说明**:
- 消费者组名请使用字节云中预定义的组名，不要添加时间戳
- 当客户端断开连接时，Go代理会自动检测并清理资源
- 支持同一组名在旧连接断开后重新连接

### 3. 安装Node.js客户端依赖

```bash
cd nodejs-client
npm install
```

### 4. 使用Node.js客户端

#### 生产者示例

```javascript
const { MQClient, MessageProperties } = require('./index');

const config = {
    endpoint: 'your-rocketmq-endpoint',
    accessKeyId: 'your-access-key-id', 
    accessKeySecret: 'your-access-key-secret',
    instanceId: 'your-instance-id',
    grpcEndpoint: 'localhost:50051'
};

async function producerDemo() {
    const client = new MQClient(config);
    const topic = 'test';
    const producer = await client.getProducer(config.instanceId, topic);

    // 1. 发送普通消息
    console.log('📤 发送普通消息...');
    const orderData = {
        orderId: 'ORDER_' + Date.now(),
        userId: 'user123',
        amount: 99.99,
        status: 'created'
    };

    let msgProps = new MessageProperties()
        .putProperty("orderId", orderData.orderId)
        .putProperty("userId", orderData.userId)
        .messageKey(orderData.orderId);

    const result1 = await producer.publishMessage(orderData, 'order', msgProps);
    console.log('✅ 普通消息发送成功:', result1.messageId);

    // 2. 发送顺序消息（同一用户的订单保证顺序）
    console.log('📊 发送顺序消息...');
    const orderUpdateData = {
        orderId: orderData.orderId,
        userId: orderData.userId,
        status: 'paid',
        timestamp: Date.now()
    };

    msgProps = new MessageProperties()
        .putProperty("orderId", orderUpdateData.orderId)
        .putProperty("action", "statusUpdate")
        .messageKey(orderUpdateData.orderId);

    const result2 = await producer.publishOrderedMessage(
        orderUpdateData,
        'order-update',
        msgProps,
        orderData.userId  // 使用userId作为分区键保证同一用户订单顺序
    );
    console.log('✅ 顺序消息发送成功:', result2.messageId);

    // 3. 发送延迟消息（字节云任意精度延迟）
    console.log('⏰ 发送延迟消息...');
    const timeoutCheckData = {
        orderId: orderData.orderId,
        action: 'timeout-check',
        createTime: Date.now()
    };

    msgProps = new MessageProperties()
        .putProperty("orderId", timeoutCheckData.orderId)
        .putProperty("action", "timeoutCheck")
        .messageKey(`timeout_${timeoutCheckData.orderId}`);

    // 60秒后投递
    const deliverTime = Date.now() + 60 * 1000;
    const result3 = await producer.publishDelayMessage(
        timeoutCheckData,
        'timeout-check',
        msgProps,
        { startDeliverTime: deliverTime }  // 精确时间戳（毫秒）
    );
    console.log('✅ 延迟消息发送成功:', result3.messageId);

    // 4. 发送事务消息
    console.log('🔄 发送事务消息...');
    const transData = {
        orderId: orderData.orderId,
        action: 'payment',
        amount: orderData.amount
    };

    msgProps = new MessageProperties()
        .putProperty("orderId", transData.orderId)
        .putProperty("transactionType", "payment")
        .messageKey(`trans_${transData.orderId}`);

    const result4 = await producer.publishTransactionMessage(
        transData,
        'transaction',
        msgProps,
        30  // 事务回查免疫时间30秒
    );
    console.log('✅ 事务消息发送成功:', result4.messageId);
    console.log('📋 事务ID:', result4.transactionId);
}
```

#### 消费者示例

```javascript
async function consumeMessage() {
    const client = new MQClient(config);
    
    // 获取消费者
    const topic = 'test';
    const groupId = 'test';
    const consumer = await client.getConsumer(config.instanceId, topic, groupId, 'tag1||tag2');
    
    // 设置消息处理器
    consumer.onMessage(async (message) => {
        console.log('收到消息:', message.body);
        
        // 确认消息
        await consumer.ackMessage(message.receiptHandle);
    });
    
    // 开始接收消息
    consumer.startReceiving();
}
```

## API参考

### MQClient

主客户端类，负责管理连接和创建生产者/消费者。

```javascript
const client = new MQClient({
    endpoint: 'rocketmq-endpoint',
    accessKeyId: 'access-key-id',
    accessKeySecret: 'access-key-secret', 
    instanceId: 'instance-id',
    grpcEndpoint: 'localhost:50051'
});
```

#### 方法

- `getProducer(instanceId, topic)` - 创建生产者
- `getConsumer(instanceId, topic, groupId, tagExpression)` - 创建消费者
- `healthCheck()` - 健康检查

### MessageProperties

消息属性类，用于设置消息的各种属性。

```javascript
const props = new MessageProperties();
props.putProperty(key, value);      // 自定义属性
props.messageKey(key);              // 消息Key
props.shardingKey(key);             // 分区键(顺序消息)
props.startDeliverTime(timestamp);  // 延时投递时间
props.transCheckImmunityTime(seconds); // 事务回查免疫时间
```

### Producer

生产者类，用于发送消息。

#### 方法

- `publishMessage(message, tag, properties)` - 发送普通消息
  - `message`: 消息内容(对象或字符串)
  - `tag`: 消息标签
  - `properties`: MessageProperties实例

- `publishOrderedMessage(message, tag, properties, shardingKey)` - 发送顺序消息
  - `message`: 消息内容(对象或字符串)
  - `tag`: 消息标签  
  - `properties`: MessageProperties实例
  - `shardingKey`: 分区键，相同分区键的消息保证顺序

- `publishDelayMessage(message, tag, properties, options)` - 发送延迟消息
  - `message`: 消息内容(对象或字符串)
  - `tag`: 消息标签
  - `properties`: MessageProperties实例
  - `options`: 延迟选项
    - `startDeliverTime`: 精确投递时间戳(毫秒)，字节云任意精度延迟
    - `delayTimeLevel`: 延迟等级1-18，传统延迟方式

- `publishTransactionMessage(message, tag, properties, transCheckImmunityTime)` - 发送事务消息
  - `message`: 消息内容(对象或字符串)
  - `tag`: 消息标签
  - `properties`: MessageProperties实例
  - `transCheckImmunityTime`: 事务回查免疫时间(秒)

### Consumer

消费者类，用于接收消息。

#### 方法

- `onMessage(handler)` - 注册消息处理器
- `startReceiving()` - 开始接收消息
- `ackMessage(receiptHandle)` - 确认消息

## 开发指南

### 项目结构

```
├── proto/                  # Protobuf定义
│   ├── rocketmq.proto     # gRPC服务定义
│   ├── rocketmq.pb.go     # 生成的Go代码
│   └── rocketmq_grpc.pb.go
├── server/                 # Go服务端
│   ├── main.go            # 主程序
│   └── service/           # 服务实现
│       └── rocketmq_service.go
├── nodejs-client/          # Node.js客户端
│   ├── package.json
│   ├── index.js           # 客户端SDK
│   └── test.js            # 测试示例
├── Makefile               # 构建脚本
└── README.md
```

### 构建命令

```bash
make proto    # 生成protobuf代码
make deps     # 下载依赖
make build    # 构建项目
make run      # 运行服务
make clean    # 清理文件
```

### 测试

```bash
# 测试生产者
cd nodejs-client
node test.js producer

# 测试消费者  
node test.js consumer
```

## 部署建议

### 生产环境部署

1. **Docker部署**
   ```bash
   # 构建Docker镜像
   docker build --platform linux/amd64 -t go-rocketmq-grpc-proxy:1.0.0 .
   
   # 运行容器
   docker run -d \
     --name rocketmq-proxy \
     -p 50051:50051 \
     -p 8080:8080 \
     -e ROCKETMQ_LOG_LEVEL=warn \
     go-rocketmq-grpc-proxy:1.0.0
   ```

2. **Go服务部署**
   - 使用Docker容器化部署
   - 配置健康检查端点
   - 设置适当的资源限制

3. **负载均衡**
   - 可以部署多个Go代理实例
   - 使用gRPC负载均衡

4. **监控告警**
   - 监控gRPC连接数
   - 监控消息发送/接收速率
   - 设置错误率告警

### 配置管理

建议将RocketMQ连接信息通过环境变量或配置文件管理：

```bash
export ROCKETMQ_ENDPOINT="your-endpoint"
export ROCKETMQ_ACCESS_KEY_ID="your-access-key"
export ROCKETMQ_ACCESS_KEY_SECRET="your-secret"
```

## 常见问题

### Q: 为什么选择gRPC而不是HTTP?

A: gRPC提供了以下优势：
- 更好的性能（HTTP/2）
- 流式支持（消费者实时接收）
- 类型安全（protobuf）
- 更好的错误处理

### Q: 如何处理连接断开？

A: 客户端会自动重连，消费者流会在断开时触发error事件，可以重新启动。

### Q: 支持哪些消息类型？

A: 支持所有RocketMQ 4.x的消息类型：
- 普通消息
- 延时/定时消息  
- 顺序消息
- 事务消息

## 许可证

MIT License 