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
- `publishMessage(body, tag, properties)` - Send normal message
- `publishOrderedMessage(body, tag, properties, shardingKey)` - Send ordered message  
- `publishDelayMessage(body, tag, properties, options)` - Send delay message
- `shutdown()` - Close producer

### Consumer

```javascript
const consumer = await client.getConsumer(instanceId, topic, groupId);
```

**Methods:**
- `onMessage(handler)` - Set message handler
- `startReceiving(tagExpression)` - Start consuming
- `ackMessage(receiptHandle)` - Acknowledge message
- `shutdown()` - Close consumer

### MessageProperties

```javascript
const properties = new MessageProperties()
    .putProperty('key', 'value')
    .messageKey('unique-key')
    .shardingKey('partition-key')
    .startDeliverTime(Date.now() + 60000);
```

## License

MIT 