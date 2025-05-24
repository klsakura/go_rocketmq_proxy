# @klsakura/rocketmq-grpc-sdk

[![npm version](https://badge.fury.io/js/%40klsakura%2Frocketmq-grpc-sdk.svg)](https://badge.fury.io/js/%40klsakura%2Frocketmq-grpc-sdk)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Node.js client SDK for RocketMQ with gRPC proxy support. 字节火山Golang RocketMQ代理的客户端，服务端见https://github.com/klsakura/go_rocketmq_proxy

## 特性

- 🚀 **简单易用** - 简洁的 API 设计，快速上手
- 📦 **完整功能** - 支持普通消息、顺序消息、延迟消息、事务消息
- ⚡ **高性能** - 基于 gRPC 协议，高效稳定
- 🛡️ **类型安全** - 完整的 TypeScript 支持，内置类型定义
- 🔥 **延迟消息** - 支持字节云任意精度延迟消息
- 📊 **生产就绪** - 经过生产环境验证
- 🔄 **双模块支持** - 同时支持 CommonJS (require) 和 ES Modules (import)

## 安装

```bash
npm install @klsakura/rocketmq-grpc-sdk
```

## ⚠️ 安全配置说明

**重要：请勿在代码中硬编码敏感信息！**

### 推荐的配置方式

1. **使用环境变量**（推荐）
```javascript
const config = {
    endpoint: process.env.ROCKETMQ_ENDPOINT,
    accessKeyId: process.env.ROCKETMQ_ACCESS_KEY_ID,
    accessKeySecret: process.env.ROCKETMQ_ACCESS_KEY_SECRET,
    instanceId: process.env.ROCKETMQ_INSTANCE_ID,
    grpcEndpoint: process.env.GRPC_ENDPOINT || 'localhost:50051'
};
```

2. **使用配置文件**
```javascript
// 复制 examples/config.example.js 为 config.js
const config = require('./config');
```

3. **使用 .env 文件**
```bash
# .env 文件
ROCKETMQ_ENDPOINT=http://your-instance.rocketmq.volces.com:9876
ROCKETMQ_ACCESS_KEY_ID=your-access-key-id
ROCKETMQ_ACCESS_KEY_SECRET=your-access-key-secret
ROCKETMQ_INSTANCE_ID=your-instance-id
GRPC_ENDPOINT=localhost:50051
```

### 示例文件使用说明

- `examples/` 目录中的示例文件已脱敏处理
- 复制 `examples/config.example.js` 为 `examples/config.js` 并填入真实配置
- `config.js` 文件已加入 `.gitignore`，不会被提交到版本控制

## 快速开始

### CommonJS 用法

```javascript
const { MQClient, MessageProperties } = require('@klsakura/rocketmq-grpc-sdk');

const config = {
    endpoint: 'http://rocketmq-xxx.rocketmq.volces.com:9876',
    accessKeyId: 'your-access-key-id',
    accessKeySecret: 'your-access-key-secret',
    instanceId: 'your-instance-id',
    grpcEndpoint: 'localhost:50051'  // gRPC代理服务地址
};
```

### ES Modules 用法

```javascript
import { MQClient, MessageProperties } from '@klsakura/rocketmq-grpc-sdk';
// 或者
import sdk from '@klsakura/rocketmq-grpc-sdk';
const { MQClient, MessageProperties } = sdk;

const config = {
    endpoint: 'http://rocketmq-xxx.rocketmq.volces.com:9876',
    accessKeyId: 'your-access-key-id',
    accessKeySecret: 'your-access-key-secret',
    instanceId: 'your-instance-id',
    grpcEndpoint: 'localhost:50051'
};
```

### TypeScript 用法

```typescript
import { MQClient, MessageProperties, MQConfig } from '@klsakura/rocketmq-grpc-sdk';

const config: MQConfig = {
    endpoint: 'http://rocketmq-xxx.rocketmq.volces.com:9876',
    accessKeyId: 'your-access-key-id',
    accessKeySecret: 'your-access-key-secret',
    instanceId: 'your-instance-id',
    grpcEndpoint: 'localhost:50051'
};

const client = new MQClient(config);
```

### 发送消息 (生产者)

```javascript
async function sendMessage() {
    const client = new MQClient(config);
    const producer = await client.getProducer(config.instanceId, 'your-topic');
    
    // 1. 普通消息
    const result = await producer.publishMessage(
        { orderId: 'ORDER_123', amount: 99.99 },
        'order',
        new MessageProperties().messageKey('ORDER_123')
    );
    console.log('消息发送成功:', result.messageId);
    
    // 2. 顺序消息 (同一分区键保证顺序)
    await producer.publishOrderedMessage(
        { orderId: 'ORDER_123', status: 'paid' },
        'order-update',
        new MessageProperties().messageKey('ORDER_123'),
        'user_123'  // 分区键
    );
    
    // 3. 延迟消息 (字节云任意精度延迟)
    const deliverTime = Date.now() + 60 * 1000; // 60秒后投递
    await producer.publishDelayMessage(
        { orderId: 'ORDER_123', action: 'timeout-check' },
        'timeout-check',
        new MessageProperties().messageKey('ORDER_123'),
        { startDeliverTime: deliverTime }
    );
}
```

### 接收消息 (消费者)

```javascript
async function receiveMessages() {
    const client = new MQClient(config);
    const consumer = await client.getConsumer(
        config.instanceId,
        'your-topic',
        'your-consumer-group',
        '*'  // 接收所有标签
    );
    
    // 注册消息处理器
    consumer.onMessage(async (message) => {
        try {
            const data = JSON.parse(message.body);
            console.log('收到消息:', {
                messageId: message.messageId,
                tag: message.tag,
                data: data
            });
            
            // 处理业务逻辑
            await handleMessage(data, message.tag);
            
            // 确认消息消费
            await consumer.ackMessage(message.receiptHandle);
            console.log('消息处理完成');
            
        } catch (error) {
            console.error('消息处理失败:', error);
        }
    });
    
    // 开始接收消息
    consumer.startReceiving();
    console.log('消费者已启动');
}

async function handleMessage(data, tag) {
    switch (tag) {
        case 'order':
            console.log('处理订单创建:', data.orderId);
            break;
        case 'order-update':
            console.log('处理订单更新:', data.orderId);
            break;
        case 'timeout-check':
            console.log('处理超时检查:', data.orderId);
            break;
    }
}
```

## API 文档

### MQClient

主要的客户端类，用于创建生产者和消费者。

```typescript
const client = new MQClient(config: MQConfig);
```

#### 方法

- `getProducer(instanceId: string, topic: string): Promise<Producer>` - 创建生产者
- `getConsumer(instanceId: string, topic: string, groupId: string, tagExpression?: string): Promise<Consumer>` - 创建消费者
- `healthCheck(): Promise<any>` - 健康检查

### Producer

消息生产者，用于发送各种类型的消息。

#### 方法

- `publishMessage(messageBody: any, tag?: string, properties?: MessageProperties): Promise<SendResult>` - 发送普通消息
- `publishOrderedMessage(messageBody: any, tag?: string, properties?: MessageProperties, shardingKey?: string): Promise<SendResult>` - 发送顺序消息
- `publishDelayMessage(messageBody: any, tag?: string, properties?: MessageProperties, options?: DelayOptions): Promise<SendResult>` - 发送延迟消息
- `publishTransactionMessage(messageBody: any, tag?: string, properties?: MessageProperties, transCheckImmunityTime?: number): Promise<TransactionResult>` - 发送事务消息

### Consumer

消息消费者，用于接收和处理消息。

#### 方法

- `onMessage(handler: (message: MessageData) => void | Promise<void>): void` - 注册消息处理器
- `startReceiving(): any` - 开始接收消息
- `ackMessage(receiptHandle: string): Promise<any>` - 确认消息消费

### MessageProperties

消息属性构建器，支持链式调用。

```javascript
const props = new MessageProperties()
    .messageKey('ORDER_123')
    .putProperty('userId', 'user_456')
    .putProperty('source', 'web');
```

#### 方法

- `messageKey(key: string): MessageProperties` - 设置消息键
- `putProperty(key: string, value: string | number): MessageProperties` - 添加自定义属性
- `shardingKey(key: string): MessageProperties` - 设置分片键
- `startDeliverTime(time: number): MessageProperties` - 设置投递时间
- `transCheckImmunityTime(time: number): MessageProperties` - 设置事务检查免疫时间

## TypeScript 接口

```typescript
interface MQConfig {
  endpoint: string;
  accessKeyId: string;
  accessKeySecret: string;
  instanceId: string;
  grpcEndpoint?: string;
}

interface MessageData {
  messageId: string;
  receiptHandle: string;
  body: string;
  tag: string;
  properties: { [key: string]: string };
  bornTimestamp: number;
  reconsumeTimes: number;
}

interface SendResult {
  messageId: string;
  receiptHandle: string;
}

interface TransactionResult extends SendResult {
  transactionId: string;
}

interface DelayOptions {
  delayTimeLevel?: number;
  startDeliverTime?: number;
}
```

## 延迟消息

支持字节跳动云 RocketMQ 的任意精度延迟消息特性：

```javascript
// 方式1: 任意精度延迟（推荐）
const deliverTime = Date.now() + 5 * 60 * 1000; // 5分钟后
await producer.publishDelayMessage(data, tag, props, {
    startDeliverTime: deliverTime
});

// 方式2: 传统延迟等级（兼容）
await producer.publishDelayMessage(data, tag, props, {
    delayTimeLevel: 10  // 等级10 = 10分钟
});
```

### 延迟等级对照表

| 等级 | 延迟时间 | 等级 | 延迟时间 |
|------|----------|------|----------|
| 1    | 1s       | 10   | 10m      |
| 2    | 5s       | 11   | 20m      |
| 3    | 10s      | 12   | 30m      |
| 4    | 30s      | 13   | 1h       |
| 5    | 1m       | 14   | 2h       |

## 示例

查看 `examples/` 目录获取完整示例：

- `examples/usage-demo.js` - 完整使用示例
- `examples/connection-check.js` - 连接验证工具
- `examples/performance-check.js` - 性能测试工具

运行示例：

```bash
# 消费者示例
npm run example:consumer

# 生产者示例
npm run example:producer

# 或直接运行
node examples/usage-demo.js consumer
node examples/usage-demo.js producer
```

## 模块格式支持

本包同时支持 CommonJS 和 ES Modules：

### CommonJS (Node.js 传统方式)
```javascript
const { MQClient, MessageProperties } = require('@klsakura/rocketmq-grpc-sdk');
```

### ES Modules (现代 JavaScript)
```javascript
import { MQClient, MessageProperties } from '@klsakura/rocketmq-grpc-sdk';
```

### TypeScript
```typescript
import { MQClient, MessageProperties, MQConfig } from '@klsakura/rocketmq-grpc-sdk';
```

## 注意事项

1. **Topic 创建**: 需要在字节跳动云控制台预先创建 Topic
2. **消费者组**: 消费者组也需要在控制台预先创建
3. **gRPC 代理**: 需要启动对应的 gRPC 代理服务
4. **网络连接**: 确保网络能够访问字节跳动云 RocketMQ 服务
5. **资源清理**: 建议在应用关闭时优雅地关闭连接

## 生产环境建议

- 使用连接池管理 gRPC 连接
- 实现消息重试机制
- 监控消息发送和消费的成功率
- 设置合适的超时时间
- 实现优雅关闭流程

## 故障排查

1. **连接失败**
   - 检查 gRPC 代理服务是否启动
   - 验证网络连接
   - 确认配置信息正确

2. **消息发送失败**
   - 检查 Topic 是否存在
   - 验证访问权限
   - 查看错误日志

3. **消费失败**
   - 确认消费者组配置
   - 检查消息处理逻辑
   - 验证 ACK 机制

## 许可证

MIT

## 贡献

欢迎提交 Issue 和 Pull Request！

## 支持

如遇问题，请：
1. 查看示例代码
2. 检查配置信息
3. 提交 Issue 