const { MQClient, MessageProperties } = require('../dist/index.js');

// 配置
const config = {
    endpoint: 'rmq-cn-7mz2ub6uo0a.cn-hangzhou.rmq.aliyuncs.com:8080',
    accessKeyId: 'your-access-key-id',
    accessKeySecret: 'your-access-key-secret',
    instanceId: 'MQ_INST_1234567890_test'
};

async function testProducer() {
    console.log('🚀 Testing Producer...');

    try {
        const client = new MQClient(config);
        console.log(`📡 Using ${client.getMode()} mode`);

        const producer = await client.getProducer(config.instanceId, 'test-topic');

        // 发送普通消息
        const properties = new MessageProperties()
            .putProperty('source', 'demo')
            .messageKey('demo-key-001');

        const result = await producer.publishMessage(
            { message: 'Hello RocketMQ!', timestamp: Date.now() },
            'demo-tag',
            properties
        );

        console.log('✅ Message sent:', result);

        // 发送顺序消息
        const orderedResult = await producer.publishOrderedMessage(
            { message: 'Ordered message', order: 1 },
            'ordered-tag',
            properties,
            'order-key-001'
        );

        console.log('✅ Ordered message sent:', orderedResult);

        // 发送延迟消息
        const delayResult = await producer.publishDelayMessage(
            { message: 'Delayed message', delay: true },
            'delay-tag',
            properties,
            { startDeliverTime: Date.now() + 60000 } // 1分钟后投递
        );

        console.log('✅ Delay message sent:', delayResult);

        // 关闭生产者
        await producer.shutdown();
        console.log('✅ Producer shutdown');

    } catch (error) {
        console.error('❌ Producer error:', error.message);
    }
}

async function testConsumer() {
    console.log('🚀 Testing Consumer...');

    try {
        const client = new MQClient(config);
        console.log(`📡 Using ${client.getMode()} mode`);

        const consumer = await client.getConsumer(
            config.instanceId,
            'test-topic',
            'test-group',
            '*'
        );

        consumer.onMessage(async (message) => {
            console.log('📨 Received message:', {
                messageId: message.messageId,
                body: message.body,
                tag: message.tag,
                properties: message.properties
            });

            // 确认消息
            try {
                await consumer.ackMessage(message.receiptHandle);
                console.log('✅ Message acknowledged');
            } catch (ackError) {
                console.error('❌ Failed to ack message:', ackError.message);
            }
        });

        const startResult = consumer.startReceiving();
        console.log('✅ Consumer started:', startResult);

        // 保持运行
        console.log('🔄 Waiting for messages... (Press Ctrl+C to exit)');

        // 优雅退出处理
        process.on('SIGINT', async () => {
            console.log('\n👋 Shutting down consumer...');
            try {
                await consumer.shutdown();
                console.log('✅ Consumer shutdown successfully');
                process.exit(0);
            } catch (error) {
                console.error('❌ Error during shutdown:', error.message);
                process.exit(1);
            }
        });

    } catch (error) {
        console.error('❌ Consumer error:', error.message);
    }
}

async function testHealthCheck() {
    console.log('🚀 Testing Health Check...');

    try {
        const client = new MQClient(config);
        const health = await client.healthCheck();
        console.log('✅ Health check result:', health);
    } catch (error) {
        console.error('❌ Health check error:', error.message);
    }
}

// 主函数
async function main() {
    const mode = process.argv[2];

    console.log('🎯 RocketMQ Native SDK Demo');
    console.log('📋 Available modes:');
    console.log('   - producer: Test message production');
    console.log('   - consumer: Test message consumption');
    console.log('   - health: Test health check');
    console.log('');

    switch (mode) {
        case 'producer':
            await testProducer();
            break;
        case 'consumer':
            await testConsumer();
            break;
        case 'health':
            await testHealthCheck();
            break;
        default:
            console.log('❓ Usage: node usage-demo.js [producer|consumer|health]');
            console.log('');
            console.log('Examples:');
            console.log('  node usage-demo.js producer   # Test producer');
            console.log('  node usage-demo.js consumer   # Test consumer');
            console.log('  node usage-demo.js health     # Test health check');
            break;
    }
}

// 处理未捕获的异常
process.on('uncaughtException', (error) => {
    console.error('💥 Uncaught Exception:', error);
    process.exit(1);
});

process.on('unhandledRejection', (reason, promise) => {
    console.error('💥 Unhandled Rejection at:', promise, 'reason:', reason);
    process.exit(1);
});

main().catch(console.error); 