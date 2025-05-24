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

### 3. 安装Node.js客户端

#### NPM安装（推荐）

```bash
npm install @klsakura/rocketmq-grpc-sdk
```

#### 本地开发安装

```bash
cd node-client
npm install
npm run build
```

### 4. 使用Node.js客户端

#### 安全配置说明

**⚠️ 重要：请勿在代码中硬编码敏感信息！**

推荐使用环境变量：
```javascript
const config = {
    endpoint: process.env.ROCKETMQ_ENDPOINT,
    accessKeyId: process.env.ROCKETMQ_ACCESS_KEY_ID,
    accessKeySecret: process.env.ROCKETMQ_ACCESS_KEY_SECRET,
    instanceId: process.env.ROCKETMQ_INSTANCE_ID,
    grpcEndpoint: process.env.GRPC_ENDPOINT || 'localhost:50051'
};
```

#### 引入方式

**CommonJS (require)**
```javascript
const { MQClient, MessageProperties } = require('@klsakura/rocketmq-grpc-sdk');
```

**ES Modules (import)**
```javascript
import { MQClient, MessageProperties } from '@klsakura/rocketmq-grpc-sdk';
```

**TypeScript**
```typescript
import { MQClient, MessageProperties, MQConfig } from '@klsakura/rocketmq-grpc-sdk';
```

#### 生产者示例

```javascript
const { MQClient, MessageProperties } = require('@klsakura/rocketmq-grpc-sdk');

const config = {
    endpoint: 'http://rocketmq-xxx.rocketmq.volces.com:9876',
    accessKeyId: 'your-access-key-id',
    accessKeySecret: 'your-access-key-secret',
    instanceId: 'your-instance-id',
    grpcEndpoint: 'localhost:50051'
};

async function producerDemo() {
    const client = new MQClient(config);
    const topic = 'upload';
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

    // 任意精度延迟：60秒后投递
    const deliverTime = Date.now() + 60 * 1000;
    const result3 = await producer.publishDelayMessage(
        timeoutCheckData,
        'timeout-check',
        msgProps,
        { startDeliverTime: deliverTime }  // 精确时间戳（毫秒）
    );
    console.log('✅ 任意精度延迟消息发送成功:', result3.messageId);
    console.log(`   将在 ${new Date(deliverTime).toLocaleString()} 投递`);

    // 传统延迟等级（兼容方式）
    const result4 = await producer.publishDelayMessage(
        timeoutCheckData,
        'legacy-timeout',
        msgProps,
        { delayTimeLevel: 10 }  // 延迟等级10 = 10分钟
    );
    console.log('✅ 传统延迟消息发送成功:', result4.messageId);

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

    const result5 = await producer.publishTransactionMessage(
        transData,
        'transaction',
        msgProps,
        30  // 事务回查免疫时间30秒
    );
    console.log('✅ 事务消息发送成功:', result5.messageId);
    console.log('📋 事务ID:', result5.transactionId);
}
```

#### 消费者示例

```javascript
async function consumerDemo() {
    const client = new MQClient(config);
    const topic = 'upload';
    
    // 使用预定义的消费者组名（字节云中配置的固定组名）
    const consumerGroupId = 'GID_group';  // 修改为你在字节云中预定义的组名
    const consumer = await client.getConsumer(
        config.instanceId,
        topic,
        consumerGroupId,
        '*'  // 接收所有tag
    );

    console.log(`✅ 消费者创建成功，组名: ${consumerGroupId}`);
    console.log('🎯 开始监听消息...');

    // 优雅退出处理
    process.on('SIGINT', () => {
        console.log('\n🛑 收到退出信号，正在优雅关闭...');
        console.log('💡 消费者资源将自动清理，可以立即重启！');
        process.exit(0);
    });

    // 注册消息处理器
    consumer.onMessage(async (message) => {
        try {
            const messageData = JSON.parse(message.body);
            const tag = message.tag;

            console.log(`📬 收到消息 [${tag}]:`, {
                messageId: message.messageId,
                tag: tag,
                timestamp: new Date(message.bornTimestamp).toLocaleString()
            });

            // 根据不同的tag处理不同的业务逻辑
            switch (tag) {
                case 'order':
                    await handleOrderMessage(messageData, message.properties);
                    break;
                case 'order-update':
                    await handleOrderUpdateMessage(messageData, message.properties);
                    break;
                case 'timeout-check':
                    await handleTimeoutCheckMessage(messageData, message.properties);
                    break;
                default:
                    console.log('📋 处理通用消息:', messageData);
            }

            // 确认消息消费
            await consumer.ackMessage(message.receiptHandle);
            console.log('✅ 消息处理完成并已确认\n');

        } catch (error) {
            console.error('❌ 消息处理失败:', error);
            // 注意：失败的消息不调用ack，会重新投递
        }
    });

    // 开始接收消息
    consumer.startReceiving();
}

async function handleOrderMessage(orderData, properties) {
    console.log('💰 处理订单创建消息:', {
        orderId: orderData.orderId,
        userId: orderData.userId,
        amount: orderData.amount,
        properties: properties
    });
}

async function handleOrderUpdateMessage(updateData, properties) {
    console.log('📊 处理订单状态更新:', {
        orderId: updateData.orderId,
        status: updateData.status,
        properties: properties
    });
}

async function handleTimeoutCheckMessage(checkData, properties) {
    console.log('⏰ 处理超时检查:', {
        orderId: checkData.orderId,
        action: checkData.action,
        properties: properties
    });
}
```

#### 运行示例

```bash
# 安装依赖
npm install @klsakura/rocketmq-grpc-sdk

# 运行生产者示例
node examples/usage-demo.js producer

# 运行消费者示例
node examples/usage-demo.js consumer
```

## API参考

### MQConfig 接口

```typescript
interface MQConfig {
    endpoint: string;           // RocketMQ实例端点
    accessKeyId: string;        // 访问密钥ID
    accessKeySecret: string;    // 访问密钥Secret
    instanceId: string;         // 实例ID
    grpcEndpoint?: string;      // gRPC代理服务地址，默认'localhost:50051'
}
```

### MQClient

主客户端类，负责管理连接和创建生产者/消费者。

```javascript
const client = new MQClient({
    endpoint: 'http://rocketmq-xxx.rocketmq.volces.com:9876',
    accessKeyId: 'your-access-key-id',
    accessKeySecret: 'your-access-key-secret', 
    instanceId: 'your-instance-id',
    grpcEndpoint: 'localhost:50051'
});
```

#### 方法

- `getProducer(instanceId, topic)` - 创建生产者
  - 返回: `Promise<Producer>`

- `getConsumer(instanceId, topic, groupId, tagExpression)` - 创建消费者
  - `tagExpression`: 标签表达式，如 `'*'`（所有）或 `'tag1||tag2'`（指定标签）
  - 返回: `Promise<Consumer>`

- `healthCheck()` - 健康检查
  - 返回: `Promise<any>`

### MessageProperties

消息属性类，用于设置消息的各种属性。支持链式调用。

```javascript
const props = new MessageProperties();
props.putProperty(key, value);              // 自定义属性
props.messageKey(key);                      // 消息Key
props.shardingKey(key);                     // 分区键(顺序消息)
props.startDeliverTime(timestamp);          // 延时投递时间
props.transCheckImmunityTime(seconds);      // 事务回查免疫时间
```

#### 属性

- `properties: { [key: string]: string }` - 自定义属性字典
- `message_key: string` - 消息键
- `sharding_key: string` - 分区键
- `start_deliver_time: number` - 开始投递时间
- `trans_check_immunity_time: number` - 事务回查免疫时间

### Producer

生产者类，用于发送消息。

#### 方法

- `publishMessage(messageBody, tag, properties)` - 发送普通消息
  - `messageBody`: 消息内容(对象或字符串)
  - `tag`: 消息标签
  - `properties`: MessageProperties实例
  - 返回: `Promise<SendResult>`

- `publishOrderedMessage(messageBody, tag, properties, shardingKey)` - 发送顺序消息
  - `messageBody`: 消息内容(对象或字符串)
  - `tag`: 消息标签  
  - `properties`: MessageProperties实例
  - `shardingKey`: 分区键，相同分区键的消息保证顺序
  - 返回: `Promise<SendResult>`

- `publishDelayMessage(messageBody, tag, properties, options)` - 发送延迟消息
  - `messageBody`: 消息内容(对象或字符串)
  - `tag`: 消息标签
  - `properties`: MessageProperties实例
  - `options`: 延迟选项 `DelayOptions`
    - `startDeliverTime?: number` - 精确投递时间戳(毫秒)，字节云任意精度延迟
    - `delayTimeLevel?: number` - 延迟等级1-18，传统延迟方式
  - 返回: `Promise<SendResult>`

- `publishTransactionMessage(messageBody, tag, properties, transCheckImmunityTime)` - 发送事务消息
  - `messageBody`: 消息内容(对象或字符串)
  - `tag`: 消息标签
  - `properties`: MessageProperties实例
  - `transCheckImmunityTime`: 事务回查免疫时间(秒)
  - 返回: `Promise<TransactionResult>`

#### 返回值类型

```typescript
interface SendResult {
    messageId: string;
    receiptHandle: string;
}

interface TransactionResult extends SendResult {
    transactionId: string;
}

interface DelayOptions {
    delayTimeLevel?: number;    // 延迟等级1-18
    startDeliverTime?: number;  // 精确时间戳(毫秒)
}
```

### Consumer

消费者类，用于接收消息。

#### 方法

- `onMessage(handler)` - 注册消息处理器
  - `handler`: `(message: MessageData) => void | Promise<void>`
  
- `startReceiving()` - 开始接收消息
  - 返回: `any` (gRPC流对象)

- `ackMessage(receiptHandle)` - 确认消息
  - `receiptHandle`: 消息回执句柄
  - 返回: `Promise<any>`

#### 消息数据类型

```typescript
interface MessageData {
    messageId: string;              // 消息ID
    receiptHandle: string;          // 回执句柄
    body: string;                   // 消息体
    tag: string;                    // 消息标签
    properties: { [key: string]: string }; // 消息属性
    bornTimestamp: number;          // 消息产生时间戳
    reconsumeTimes: number;         // 重消费次数
}
```

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
├── node-client/            # Node.js客户端
│   ├── package.json       # NPM包配置
│   ├── src/
│   │   └── index.ts       # TypeScript源码
│   ├── dist/              # 编译输出
│   ├── examples/          # 使用示例
│   │   ├── usage-demo.js  # 完整使用演示
│   │   ├── performance-check.js  # 性能测试
│   │   ├── connection-check.js   # 连接测试
│   │   └── config.example.js     # 配置示例
│   ├── rocketmq.proto     # gRPC定义文件
│   ├── tsconfig.json      # TypeScript配置
│   └── README.md          # 客户端文档
├── scripts/               # 构建脚本
├── bin/                   # 编译输出
├── Dockerfile             # Docker构建文件
├── docker-entrypoint.sh   # Docker启动脚本
├── Makefile               # 构建配置
└── README.md              # 项目文档
```

### 构建命令

```bash
# Go服务端
make proto    # 生成protobuf代码
make deps     # 下载依赖
make build    # 构建项目
make run      # 运行服务
make clean    # 清理文件

# Node.js客户端
cd node-client
npm install   # 安装依赖
npm run build # 构建TypeScript
npm run dev   # 开发模式（监听文件变化）
```

### 测试

```bash
# 测试生产者
cd node-client
node examples/usage-demo.js producer

# 测试消费者  
node examples/usage-demo.js consumer

# 性能测试
node examples/performance-check.js

# 连接测试
node examples/connection-check.js
```

## 部署建议

### 生产环境部署

1. **Docker部署（推荐）**
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
   - 配置健康检查端点 `:8080/metrics`
   - 设置适当的资源限制
   - 配置日志级别和输出

3. **负载均衡**
   - 可以部署多个Go代理实例
   - 使用gRPC负载均衡
   - Node.js客户端自动轮询连接

4. **监控告警**
   - 监控gRPC连接数
   - 监控消息发送/接收速率
   - 设置错误率告警
   - 消费者组连接状态监控

### 配置管理

建议将RocketMQ连接信息通过环境变量或配置文件管理：

```bash
# 环境变量方式
export ROCKETMQ_ENDPOINT="http://rocketmq-xxx.rocketmq.volces.com:9876"
export ROCKETMQ_ACCESS_KEY_ID="your-access-key-id"
export ROCKETMQ_ACCESS_KEY_SECRET="your-access-key-secret"
export ROCKETMQ_INSTANCE_ID="your-instance-id"
export GRPC_ENDPOINT="localhost:50051"
```

## 常见问题

### Q: 为什么选择gRPC而不是HTTP?

A: gRPC提供了以下优势：
- **更好的性能** - HTTP/2协议，二进制传输
- **流式支持** - 消费者实时接收消息流
- **类型安全** - Protobuf强类型定义
- **更好的错误处理** - 丰富的状态码和错误信息
- **跨语言支持** - 标准化的接口定义

### Q: 如何处理连接断开？

A: 
- **生产者**: 自动重连，发送失败会抛出异常
- **消费者**: 流断开时触发error事件，支持自动重连
- **代理服务**: 30秒内自动清理断开的消费者连接
- **最佳实践**: 在客户端实现重试逻辑和优雅退出

### Q: 支持哪些消息类型？

A: 支持所有RocketMQ的消息类型：
- **普通消息** - 基础异步消息
- **延时/定时消息** - 字节云任意精度延迟和传统18级延迟
- **顺序消息** - 基于分区键的顺序保证
- **事务消息** - 支持事务一致性

### Q: 消费者组连接管理机制？

A: 
- **自动清理** - 断开连接30秒后自动清理资源
- **智能重连** - 相同组名可在旧连接断开后重新连接
- **心跳检测** - 10秒心跳间隔，1分钟超时清理
- **手动清理** - 提供gRPC API手动清理卡住的连接

### Q: 如何优化性能？

A:
- **批量发送** - 在业务逻辑中实现消息批量处理
- **连接池** - 复用MQClient实例，避免频繁创建连接
- **异步处理** - 充分利用async/await异步特性
- **消息压缩** - 对大消息体进行压缩
- **合理设置** - 根据业务需求调整超时时间和重试次数

## 许可证

MIT License