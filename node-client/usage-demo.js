const { MQClient, MessageProperties } = require('./index');

// 配置信息
const config = {
    endpoint: 'http://rocketmq-cnaie069c174d7a1.rocketmq.volces.com:9876',
    accessKeyId: 'I4q3P1ZGEqzocp7Ar4e7VUTM',
    accessKeySecret: 'k5KKAYupmSNcOJdgqLW2wBd8',
    instanceId: 'rocketmq-cnaie069c174d7a1',
    grpcEndpoint: 'localhost:50051'
};

// 生产者示例
async function producerDemo() {
    console.log('🚀 RocketMQ生产者使用示例');
    console.log('='.repeat(40));

    try {
        const client = new MQClient(config);
        const topic = 'upload';
        const producer = await client.getProducer(config.instanceId, topic);

        console.log('✅ 生产者创建成功\n');

        // 1. 发送普通消息
        console.log('📤 发送普通消息...');
        const orderData = {
            orderId: 'ORDER_' + Date.now(),
            userId: 'user123',
            productId: 'prod456',
            amount: 99.99,
            status: 'created'
        };

        let msgProps = new MessageProperties()
            .putProperty("orderId", orderData.orderId)
            .putProperty("userId", orderData.userId)
            .messageKey(orderData.orderId);

        const result1 = await producer.publishMessage(orderData, 'order', msgProps);
        console.log(`   ✅ 订单消息发送成功: ${result1.messageId}\n`);

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
        console.log(`   ✅ 订单更新消息发送成功: ${result2.messageId}\n`);

        // 3. 发送延迟消息演示（字节云任意精度延迟）
        console.log('⏰ 发送延迟消息演示...');

        // 3.1 字节云任意精度延迟消息（推荐方式）
        console.log('   📌 方式1: 任意精度延迟（字节云2023年2月21日后实例）');
        const timeoutCheckData = {
            orderId: orderData.orderId,
            action: 'timeout-check',
            createTime: Date.now()
        };

        msgProps = new MessageProperties()
            .putProperty("orderId", timeoutCheckData.orderId)
            .putProperty("action", "timeoutCheck")
            .putProperty("delayType", "arbitrary-precision")
            .messageKey(`timeout_${timeoutCheckData.orderId}`);

        // 使用任意精度延迟：60秒后投递
        const deliverTime = Date.now() + 60 * 1000; // 60秒后
        const result3 = await producer.publishDelayMessage(
            timeoutCheckData,
            'timeout-check',
            msgProps,
            { startDeliverTime: deliverTime }  // 使用精确时间戳（毫秒）
        );
        console.log(`   ✅ 任意精度延迟消息发送成功: ${result3.messageId}`);
        console.log(`      将在 ${new Date(deliverTime).toLocaleString()} 投递\n`);

        // 3.2 传统18级延迟消息（兼容方式）
        console.log('   📌 方式2: 传统延迟等级（兼容老版本）');
        const legacyTimeoutData = {
            orderId: orderData.orderId,
            action: 'legacy-timeout-check',
            createTime: Date.now()
        };

        msgProps = new MessageProperties()
            .putProperty("orderId", legacyTimeoutData.orderId)
            .putProperty("action", "legacyTimeoutCheck")
            .putProperty("delayType", "level-based")
            .messageKey(`legacy_timeout_${legacyTimeoutData.orderId}`);

        // 使用延迟等级：级别10 = 10分钟
        const result4 = await producer.publishDelayMessage(
            legacyTimeoutData,
            'legacy-timeout',
            msgProps,
            { delayTimeLevel: 10 }  // 延迟等级10 = 10分钟
        );
        console.log(`   ✅ 传统延迟消息发送成功: ${result4.messageId}`);
        console.log(`      延迟等级10 = 10分钟后投递\n`);

        // 3.3 定时消息演示（指定具体时间）
        console.log('   📌 方式3: 定时消息（指定具体投递时间）');
        const scheduledData = {
            orderId: orderData.orderId,
            action: 'scheduled-check',
            createTime: Date.now()
        };

        msgProps = new MessageProperties()
            .putProperty("orderId", scheduledData.orderId)
            .putProperty("action", "scheduledCheck")
            .putProperty("delayType", "scheduled")
            .messageKey(`scheduled_${scheduledData.orderId}`);

        // 定时到今天晚上23:59投递
        const tonight = new Date();
        tonight.setHours(23, 59, 0, 0);

        const result5 = await producer.publishDelayMessage(
            scheduledData,
            'scheduled-check',
            msgProps,
            { startDeliverTime: tonight.getTime() }
        );
        console.log(`   ✅ 定时消息发送成功: ${result5.messageId}`);
        console.log(`      将在 ${tonight.toLocaleString()} 投递\n`);

        console.log('🎉 所有消息发送完成！');

    } catch (error) {
        console.error('❌ 生产者示例失败:', error.message);
    }
}

// 消费者示例
async function consumerDemo() {
    console.log('👂 RocketMQ消费者使用示例');
    console.log('='.repeat(40));

    try {
        const client = new MQClient(config);
        const topic = 'upload';

        // 使用预定义的消费者组名（字节云中配置的固定组名）
        const consumerGroupId = 'GID_group';  // 修改为你在字节云中预定义的组名
        const consumer = await client.getConsumer(
            config.instanceId,
            topic,
            consumerGroupId,  // 使用预定义的消费者组名
            '*'  // 接收所有tag
        );

        console.log(`✅ 消费者创建成功，组名: ${consumerGroupId}`);
        console.log('🎯 开始监听消息...');
        console.log('💡 提示: 现在支持Ctrl+C后重新启动，无需等待!');
        console.log('');

        // 优雅退出处理
        let isShuttingDown = false;

        process.on('SIGINT', () => {
            if (!isShuttingDown) {
                isShuttingDown = true;
                console.log('\n🛑 收到退出信号，正在优雅关闭...');
                console.log('💡 消费者资源将自动清理，可以立即重启！');
                process.exit(0);
            }
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
                    case 'legacy-timeout':
                        await handleLegacyTimeoutMessage(messageData, message.properties);
                        break;
                    case 'scheduled-check':
                        await handleScheduledCheckMessage(messageData, message.properties);
                        break;
                    default:
                        console.log(`   ⚠️  未知消息类型: ${tag}`);
                }

                // 确认消息处理完成
                await consumer.ackMessage(message.receiptHandle);
                console.log(`   ✅ 消息处理完成\n`);

            } catch (error) {
                console.error(`   ❌ 消息处理失败: ${error.message}\n`);
                // 在实际应用中，这里可能需要重试或者发送到死信队列
            }
        });

        const stream = consumer.startReceiving();

        console.log('💡 消费者已启动，按 Ctrl+C 停止\n');

    } catch (error) {
        console.error('❌ 消费者示例失败:', error.message);
    }
}

// 订单消息处理
async function handleOrderMessage(orderData, properties) {
    console.log(`   🛒 处理订单创建: ${orderData.orderId}`);
    console.log(`      用户: ${orderData.userId}`);
    console.log(`      金额: ¥${orderData.amount}`);

    // 这里是实际的业务逻辑，比如：
    // 1. 保存订单到数据库
    // 2. 扣减库存
    // 3. 发送通知等

    console.log(`   📝 订单入库完成`);
}

// 订单更新消息处理
async function handleOrderUpdateMessage(updateData, properties) {
    console.log(`   🔄 处理订单状态更新: ${updateData.orderId}`);
    console.log(`      新状态: ${updateData.status}`);

    // 实际业务逻辑：
    // 1. 更新订单状态
    // 2. 触发后续流程
    // 3. 发送用户通知等

    console.log(`   💫 订单状态更新完成`);
}

// 超时检查消息处理（任意精度延迟）
async function handleTimeoutCheckMessage(checkData, properties) {
    console.log(`   ⏱️  处理订单超时检查: ${checkData.orderId}`);
    console.log(`      延迟类型: ${properties.delayType || '未知'}`);
    console.log(`      检查动作: ${checkData.action}`);

    // 实际业务逻辑：
    // 1. 查询订单当前状态
    // 2. 如果还是未支付状态，则取消订单
    // 3. 释放库存和优惠券等

    console.log(`   🔍 任意精度延迟检查完成`);
}

// 传统延迟检查消息处理
async function handleLegacyTimeoutMessage(checkData, properties) {
    console.log(`   ⏰ 处理传统延迟检查: ${checkData.orderId}`);
    console.log(`      延迟类型: ${properties.delayType || '未知'}`);
    console.log(`      检查动作: ${checkData.action}`);

    // 传统延迟等级的业务逻辑处理
    console.log(`   📊 传统延迟等级检查完成`);
}

// 定时检查消息处理
async function handleScheduledCheckMessage(checkData, properties) {
    console.log(`   📅 处理定时检查: ${checkData.orderId}`);
    console.log(`      延迟类型: ${properties.delayType || '未知'}`);
    console.log(`      检查动作: ${checkData.action}`);
    console.log(`      当前时间: ${new Date().toLocaleString()}`);

    // 定时消息的业务逻辑处理
    console.log(`   ⏰ 定时检查完成`);
}

// 主函数
async function main() {
    const mode = process.argv[2];

    if (mode === 'producer') {
        await producerDemo();
    } else if (mode === 'consumer') {
        await consumerDemo();
    } else {
        console.log('📚 RocketMQ使用示例');
        console.log('');
        console.log('用法:');
        console.log('  node usage-demo.js producer   # 运行生产者示例');
        console.log('  node usage-demo.js consumer   # 运行消费者示例');
        console.log('');
        console.log('建议使用顺序:');
        console.log('  1. 先启动消费者: node usage-demo.js consumer');
        console.log('  2. 再运行生产者: node usage-demo.js producer');
        console.log('');
        console.log('💡 这个示例展示了电商订单处理的完整流程：');
        console.log('   📦 订单创建 → 状态更新 → 延迟/定时检查');
        console.log('');
        console.log('🕐 延迟消息类型说明：');
        console.log('   1. 任意精度延迟 - 字节云2023年2月21日后实例（推荐）');
        console.log('   2. 传统18级延迟 - 兼容老版本RocketMQ');
        console.log('   3. 定时消息 - 指定具体投递时间');
        console.log('');
        console.log('📋 延迟等级参考（传统方式）：');
        console.log('   1s 5s 10s 30s 1m 2m 3m 4m 5m 6m 7m 8m 9m 10m 20m 30m 1h 2h');
    }
}

main().catch(console.error); 