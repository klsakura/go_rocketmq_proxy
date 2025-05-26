const { MQClient } = require('./node-client/dist/index');

// 配置信息 - 请替换为你的实际配置
const config = {
    endpoint: 'http://your-rocketmq-instance.rocketmq.volces.com:9876',
    accessKeyId: 'your-access-key-id',
    accessKeySecret: 'your-access-key-secret',
    instanceId: 'your-instance-id',
    grpcEndpoint: 'localhost:50051'
};

async function testConsumerReuse() {
    console.log('🧪 测试消费者复用机制');
    console.log('='.repeat(40));

    try {
        const client = new MQClient(config);
        const topic = 'DelayMsg';
        const groupId = 'GID_Mall';

        console.log('📋 测试配置:');
        console.log(`   Topic: ${topic}`);
        console.log(`   Group: ${groupId}`);
        console.log(`   gRPC: ${config.grpcEndpoint}`);
        console.log('');

        // 创建第一个消费者
        console.log('🔄 创建第一个消费者...');
        const consumer1 = await client.getConsumer(
            config.instanceId,
            topic,
            groupId,
            '*'
        );
        console.log('✅ 第一个消费者创建成功');

        // 等待一秒
        await new Promise(resolve => setTimeout(resolve, 1000));

        // 创建第二个消费者（应该复用第一个）
        console.log('🔄 创建第二个消费者（应该复用）...');
        const consumer2 = await client.getConsumer(
            config.instanceId,
            topic,
            groupId,
            '*'
        );
        console.log('✅ 第二个消费者创建成功');

        // 等待一秒
        await new Promise(resolve => setTimeout(resolve, 1000));

        // 创建第三个消费者（应该复用第一个）
        console.log('🔄 创建第三个消费者（应该复用）...');
        const consumer3 = await client.getConsumer(
            config.instanceId,
            topic,
            groupId,
            '*'
        );
        console.log('✅ 第三个消费者创建成功');

        console.log('');
        console.log('🎉 所有消费者创建成功！');
        console.log('💡 如果没有看到 "consumer group exist already" 错误，说明复用机制工作正常');
        console.log('');
        console.log('📊 现在可以查看Go服务日志，应该看到:');
        console.log('   - 第一个消费者: "New consumer created"');
        console.log('   - 第二个消费者: "Reusing consumer" (RefCount=2)');
        console.log('   - 第三个消费者: "Reusing consumer" (RefCount=3)');

        // 启动消息接收（可选）
        console.log('');
        console.log('🎯 启动消息接收（按 Ctrl+C 退出）...');

        let messageCount = 0;

        // 为每个消费者注册消息处理器
        [consumer1, consumer2, consumer3].forEach((consumer, index) => {
            consumer.onMessage(async (message) => {
                messageCount++;
                console.log(`📬 消费者${index + 1} 收到消息 #${messageCount}:`, {
                    messageId: message.messageId,
                    tag: message.tag,
                    body: message.body.substring(0, 100) + (message.body.length > 100 ? '...' : '')
                });
            });

            consumer.startReceiving();
        });

        // 优雅退出处理
        process.on('SIGINT', () => {
            console.log('\n🛑 收到退出信号，正在关闭...');
            console.log(`📊 总共处理了 ${messageCount} 条消息`);
            console.log('💡 消费者将自动清理，引用计数会递减');
            process.exit(0);
        });

        // 定期输出统计信息
        setInterval(() => {
            if (messageCount > 0) {
                console.log(`📊 统计: 已处理 ${messageCount} 条消息`);
            }
        }, 30000);

    } catch (error) {
        console.error('❌ 测试失败:', error.message);

        if (error.message.includes('consumer group exist already')) {
            console.log('');
            console.log('🔧 这个错误说明消费者复用机制还没有完全解决问题');
            console.log('   请检查:');
            console.log('   1. Go服务是否已重新编译和重启');
            console.log('   2. 是否有旧的消费者实例还在运行');
            console.log('   3. 查看Go服务日志获取更多信息');
        } else {
            console.log('');
            console.log('🔧 排查建议:');
            console.log('   1. 检查Go代理服务是否启动: ./bin/rocketmq-proxy');
            console.log('   2. 检查gRPC连接配置');
            console.log('   3. 查看Go服务日志');
        }

        process.exit(1);
    }
}

// 运行测试
if (require.main === module) {
    testConsumerReuse().catch(error => {
        console.error('程序运行失败:', error);
        process.exit(1);
    });
}

module.exports = { testConsumerReuse }; 