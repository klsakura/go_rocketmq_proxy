# RocketMQ Native SDK

High-performance Node.js client for Apache RocketMQ with native addon support.

## Installation

```bash
npm install @klsakura/rocketmq-native-sdk
```

## Requirements

- Node.js 16.0.0 or higher
- Supported platforms: macOS (ARM64/x64), Linux (x64), Windows (x64)

## Quick Start

### CommonJS

```javascript
const { MQClient, MessageProperties } = require('@klsakura/rocketmq-native-sdk');

// Create client
const config = {
    endpoint: 'your-rocketmq-endpoint',
    accessKeyId: 'your-access-key-id',
    accessKeySecret: 'your-access-key-secret',
    instanceId: 'your-instance-id'
};

const client = new MQClient(config);

// Create producer
const producer = await client.getProducer('your-instance-id', 'your-topic');

// Send message
const result = await producer.publishMessage('Hello RocketMQ!', 'demo-tag');
console.log('Message sent:', result.messageId);

// Create consumer
const consumer = await client.getConsumer('your-instance-id', 'your-topic', 'your-group');

// Set message handler
consumer.onMessage(async (message) => {
    console.log('Received:', message.body);
    await consumer.ackMessage(message.receiptHandle);
});

// Start consuming
consumer.startReceiving();
```

### ES Module

```javascript
import { MQClient, MessageProperties } from '@klsakura/rocketmq-native-sdk';

const config = {
    endpoint: 'your-rocketmq-endpoint',
    accessKeyId: 'your-access-key-id', 
    accessKeySecret: 'your-access-key-secret',
    instanceId: 'your-instance-id'
};

const client = new MQClient(config);
const producer = await client.getProducer('instance-id', 'topic');

// Send message with properties
const properties = new MessageProperties()
    .putProperty('source', 'demo')
    .messageKey('unique-key-001');

await producer.publishMessage(
    { message: 'Hello from ES Module!' },
    'demo-tag',
    properties
);
```

## Producer Examples

### 1. Normal Message (普通消息)

```javascript
const producer = await client.getProducer('instance-id', 'your-topic');

// Simple message
await producer.publishMessage('Hello World');

// Message with tag
await producer.publishMessage('Hello World', 'order-tag');

// Message with properties
const properties = new MessageProperties()
    .putProperty('userId', '12345')
    .putProperty('source', 'web')
    .messageKey('order-001');

const result = await producer.publishMessage({
    orderId: 'ORDER-001',
    amount: 99.99
}, 'order-tag', properties);

console.log('Normal message sent:', result.messageId);
```

### 2. Ordered Message (顺序消息)

```javascript
// Ordered messages with same sharding key will be delivered in order
const shardingKey = 'user-12345'; // Messages with same key are ordered

await producer.publishOrderedMessage(
    { step: 1, action: 'create_order' },
    'order-step',
    properties,
    shardingKey
);

await producer.publishOrderedMessage(
    { step: 2, action: 'pay_order' },
    'order-step', 
    properties,
    shardingKey
);

await producer.publishOrderedMessage(
    { step: 3, action: 'ship_order' },
    'order-step',
    properties,
    shardingKey
);

console.log('Ordered messages sent');
```

### 3. Delay Message (延迟消息)

```javascript
// Method 1: Using delayTimeLevel (1-18, each level represents different delay time)
const delayOptions1 = {
    delayTimeLevel: 3  // Level 3 = 10 seconds delay
};

await producer.publishDelayMessage(
    'This message will be delivered after 10 seconds',
    'delay-tag',
    null,
    delayOptions1
);

// Method 2: Using specific timestamp
const delayOptions2 = {
    startDeliverTime: Date.now() + 60000  // Deliver after 1 minute
};

await producer.publishDelayMessage(
    { reminder: 'Meeting in 1 minute' },
    'reminder-tag',
    properties,
    delayOptions2
);

// Method 3: Using MessageProperties for delay
const delayProperties = new MessageProperties()
    .putProperty('businessType', 'reminder')
    .startDeliverTime(Date.now() + 300000);  // 5 minutes delay

await producer.publishDelayMessage(
    'Delayed reminder message',
    'reminder-tag',
    delayProperties
);

console.log('Delay messages sent');
```

### 4. Advanced Message Properties

```javascript
const properties = new MessageProperties()
    .putProperty('userId', '12345')          // Custom property
    .putProperty('orderType', 'premium')     // Business property
    .messageKey('unique-business-key-001')   // Message key for deduplication
    .shardingKey('user-12345')              // Sharding key for ordered messages
    .startDeliverTime(Date.now() + 30000)   // Delay 30 seconds
    .transCheckImmunityTime(60);            // Transaction check immunity time

const result = await producer.publishMessage(
    {
        orderId: 'ORDER-001',
        userId: '12345',
        amount: 199.99,
        items: ['item1', 'item2']
    },
    'premium-order',
    properties
);
```

## API Reference

### MQClient

```javascript
const client = new MQClient(config);
```

**Config object:**
- `endpoint` - RocketMQ endpoint URL
- `accessKeyId` - Access key ID  
- `accessKeySecret` - Access key secret
- `instanceId` - Instance ID

**Methods:**
- `getProducer(instanceId, topic)` - Create producer
- `getConsumer(instanceId, topic, groupId, tagExpression)` - Create consumer
- `healthCheck()` - Health check

### Producer

```javascript
const producer = await client.getProducer(instanceId, topic);
```

**Methods:**
- `publishMessage(body, tag?, properties?)` - Send normal message
- `publishOrderedMessage(body, tag?, properties?, shardingKey?)` - Send ordered message  
- `publishDelayMessage(body, tag?, properties?, options?)` - Send delay message
- `shutdown()` - Close producer

**DelayOptions:**
- `delayTimeLevel` - Delay level (1-18), each level represents different delay time
- `startDeliverTime` - Specific timestamp for message delivery

### Consumer

```javascript
const consumer = await client.getConsumer(instanceId, topic, groupId);
```

**Methods:**
- `onMessage(handler)` - Set message handler
- `startReceiving(tagExpression?)` - Start consuming
- `ackMessage(receiptHandle)` - Acknowledge message
- `shutdown()` - Close consumer

### MessageProperties

```javascript
const properties = new MessageProperties()
    .putProperty('key', 'value')                    // Custom properties
    .messageKey('unique-key')                       // Message key
    .shardingKey('partition-key')                   // Sharding key for ordering
    .startDeliverTime(Date.now() + 60000)          // Delay delivery
    .transCheckImmunityTime(60);                   // Transaction immunity time
```

**Methods:**
- `putProperty(key, value)` - Add custom property
- `messageKey(key)` - Set message key for deduplication
- `shardingKey(key)` - Set sharding key for ordered messages
- `startDeliverTime(timestamp)` - Set delay delivery time
- `transCheckImmunityTime(seconds)` - Set transaction check immunity time

## License

MIT 