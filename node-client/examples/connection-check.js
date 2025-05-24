const { MQClient, MessageProperties } = require('../dist/index');

// 配置信息 - 请替换为你的实际配置
const config = {
    endpoint: 'http://your-rocketmq-instance.rocketmq.volces.com:9876',
    accessKeyId: 'your-access-key-id',
    accessKeySecret: 'your-access-key-secret',
    instanceId: 'your-instance-id',
    grpcEndpoint: 'localhost:50051'
};

async function checkConnection() {
    console.log('🔍 RocketMQ连接验证');
    console.log('='.repeat(50));

    try {
        const client = new MQClient(config);

        // 1. 检查代理服务健康状态
        console.log('📡 1. 检查代理服务连接...');
        const health = await client.healthCheck();
        console.log(`   ✅ 代理服务状态: ${health.healthy ? '正常' : '异常'}`);
        console.log(`   📝 消息: ${health.message}`);

        // 2. 测试创建生产者
        console.log('\n🚀 2. 测试创建生产者...');
        const topic = 'upload';
        const producer = await client.getProducer(config.instanceId, topic);
        console.log(`   ✅ 生产者创建成功`);
        console.log(`   📍 Topic: ${config.instanceId}%${topic}`);

        // 3. 测试发送消息
        console.log('\n📤 3. 测试发送消息...');
        const testMessage = {
            type: 'connection-test',
            content: '连接验证测试消息',
            timestamp: Date.now()
        };

        const msgProps = new MessageProperties()
            .putProperty("test", "connection")
            .messageKey(`conn_test_${Date.now()}`);

        const result = await producer.publishMessage(testMessage, 'test', msgProps);
        console.log(`   ✅ 消息发送成功`);
        console.log(`   🆔 消息ID: ${result.messageId}`);

        // 4. 测试创建消费者
        console.log('\n👂 4. 测试创建消费者...');
        const consumer = await client.getConsumer(
            config.instanceId,
            topic,
            'connection-test-group',
            '*'
        );
        console.log(`   ✅ 消费者创建成功`);
        console.log(`   🔗 消费组: connection-test-group`);

        console.log('\n🎉 所有连接测试通过！');
        console.log('\n📋 连接信息摘要:');
        console.log(`   🌐 RocketMQ端点: ${config.endpoint}`);
        console.log(`   🏠 实例ID: ${config.instanceId}`);
        console.log(`   🔌 代理服务: ${config.grpcEndpoint}`);
        console.log(`   📊 Topic: ${topic}`);

    } catch (error) {
        console.error('\n❌ 连接测试失败:', error.message);
        console.log('\n🔧 排查建议:');
        console.log('   1. 检查Go代理服务是否启动: ./bin/rocketmq-proxy');
        console.log('   2. 检查RocketMQ配置信息是否正确');
        console.log('   3. 检查网络连接和防火墙设置');
        console.log('   4. 确认Topic已在控制台创建');

        process.exit(1);
    }
}

if (require.main === module) {
    checkConnection().catch(console.error);
}

module.exports = { checkConnection }; 