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
const { Producer, Consumer } = require('@klsakura/rocketmq-native-sdk');

// Create producer
const producer = new Producer({
    nameServer: 'localhost:9876',
    groupName: 'test-producer'
});

// Send message
await producer.send({
    topic: 'test-topic',
    body: 'Hello RocketMQ'
});
```

### ES Module

```javascript
import { Producer, Consumer } from '@klsakura/rocketmq-native-sdk';

// Create consumer
const consumer = new Consumer({
    nameServer: 'localhost:9876',
    groupName: 'test-consumer',
    topics: ['test-topic']
});

// Start consuming
consumer.start();
```

## API

### Producer

- `new Producer(config)` - Create producer instance
- `producer.send(message)` - Send message
- `producer.close()` - Close producer

### Consumer

- `new Consumer(config)` - Create consumer instance
- `consumer.start()` - Start consuming messages
- `consumer.stop()` - Stop consumer

## License

MIT 