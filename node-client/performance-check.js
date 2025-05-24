const { MQClient, MessageProperties } = require('./index');

// 测试配置
const testConfig = {
    endpoint: 'test-endpoint',
    accessKeyId: 'test-key',
    accessKeySecret: 'test-secret',
    instanceId: 'perf-test-instance',
    grpcEndpoint: 'localhost:50051'
};

// 性能测试参数
const CONCURRENT_CLIENTS = 50;      // 并发客户端数
const MESSAGES_PER_CLIENT = 100;    // 每个客户端发送的消息数
const TEST_DURATION_MS = 30000;     // 测试持续时间（毫秒）

async function performanceTest() {
    console.log('🚀 RocketMQ代理服务性能测试');
    console.log('='.repeat(50));
    console.log(`📊 测试参数:`);
    console.log(`   - 并发客户端: ${CONCURRENT_CLIENTS}`);
    console.log(`   - 每客户端消息数: ${MESSAGES_PER_CLIENT}`);
    console.log(`   - 总消息数: ${CONCURRENT_CLIENTS * MESSAGES_PER_CLIENT}`);
    console.log(`   - 测试时长: ${TEST_DURATION_MS / 1000}秒\n`);

    let totalSent = 0;
    let totalErrors = 0;
    const startTime = Date.now();

    // 创建多个并发客户端
    const clients = [];
    for (let i = 0; i < CONCURRENT_CLIENTS; i++) {
        clients.push(new MQClient(testConfig));
    }

    console.log('📡 检查服务健康状态...');
    try {
        const health = await clients[0].healthCheck();
        console.log(`✅ 服务状态: ${health.healthy ? '健康' : '异常'}\n`);
    } catch (error) {
        console.log(`❌ 无法连接到代理服务: ${error.message}`);
        console.log('💡 请确保Go代理服务正在运行: ./start.sh\n');
        return;
    }

    console.log('🔥 开始并发测试...\n');

    // 并发发送消息
    const tasks = clients.map(async (client, clientIndex) => {
        let clientSent = 0;
        let clientErrors = 0;

        try {
            const producer = await client.getProducer(testConfig.instanceId, 'perf-test-topic');

            for (let msgIndex = 0; msgIndex < MESSAGES_PER_CLIENT; msgIndex++) {
                try {
                    const msgProps = new MessageProperties();
                    msgProps.putProperty('clientId', clientIndex.toString())
                        .putProperty('messageIndex', msgIndex.toString())
                        .messageKey(`client-${clientIndex}-msg-${msgIndex}`);

                    const messageBody = JSON.stringify({
                        clientId: clientIndex,
                        messageIndex: msgIndex,
                        timestamp: Date.now(),
                        testData: 'A'.repeat(100) // 100字节测试数据
                    });

                    await producer.publishMessage(messageBody, 'perf-test', msgProps);
                    clientSent++;

                    // 避免过快发送，给系统一点喘息时间
                    if (msgIndex % 10 === 0) {
                        await new Promise(resolve => setTimeout(resolve, 1));
                    }

                } catch (error) {
                    clientErrors++;
                    if (clientErrors <= 3) { // 只记录前3个错误避免刷屏
                        console.log(`❌ Client ${clientIndex} 消息 ${msgIndex} 失败: ${error.message}`);
                    }
                }
            }
        } catch (error) {
            console.log(`❌ Client ${clientIndex} 初始化失败: ${error.message}`);
            clientErrors = MESSAGES_PER_CLIENT;
        }

        return { clientSent, clientErrors };
    });

    // 等待所有任务完成
    const results = await Promise.all(tasks);
    const endTime = Date.now();
    const duration = endTime - startTime;

    // 统计结果
    results.forEach(result => {
        totalSent += result.clientSent;
        totalErrors += result.clientErrors;
    });

    // 获取服务指标
    console.log('\n📊 获取服务性能指标...');
    try {
        const response = await fetch('http://localhost:8080/metrics');
        const metrics = await response.json();

        console.log('\n🎯 测试结果:');
        console.log('='.repeat(50));
        console.log(`⏱️  测试耗时: ${duration}ms (${(duration / 1000).toFixed(2)}秒)`);
        console.log(`✅ 成功发送: ${totalSent} 条消息`);
        console.log(`❌ 发送失败: ${totalErrors} 条消息`);
        console.log(`🎯 成功率: ${((totalSent / (totalSent + totalErrors)) * 100).toFixed(2)}%`);
        console.log(`🚀 平均QPS: ${(totalSent / (duration / 1000)).toFixed(2)} 消息/秒`);
        console.log(`⚡ 平均延迟: ${(duration / totalSent).toFixed(2)}ms/消息`);

        console.log('\n📈 服务指标:');
        console.log(`🔗 活跃连接: ${metrics.active_connections}`);
        console.log(`📤 总发送消息: ${metrics.messages_sent}`);
        console.log(`📊 服务QPS: ${metrics.messages_per_second}`);
        console.log(`⏱️  平均响应时间: ${metrics.avg_response_time_ms}ms`);
        console.log(`🧵 协程数量: ${metrics.goroutine_count}`);
        console.log(`💾 内存使用: ${metrics.memory_usage_mb}MB`);
        console.log(`❌ 错误计数: ${metrics.error_count}`);

    } catch (error) {
        console.log(`⚠️ 无法获取指标: ${error.message}`);

        console.log('\n🎯 测试结果:');
        console.log('='.repeat(50));
        console.log(`⏱️  测试耗时: ${duration}ms (${(duration / 1000).toFixed(2)}秒)`);
        console.log(`✅ 成功发送: ${totalSent} 条消息`);
        console.log(`❌ 发送失败: ${totalErrors} 条消息`);
        console.log(`🎯 成功率: ${((totalSent / (totalSent + totalErrors)) * 100).toFixed(2)}%`);
        console.log(`🚀 平均QPS: ${(totalSent / (duration / 1000)).toFixed(2)} 消息/秒`);
        console.log(`⚡ 平均延迟: ${(duration / totalSent).toFixed(2)}ms/消息`);
    }

    console.log('\n🔧 性能调优建议:');
    if (totalErrors > 0) {
        console.log('⚠️ 检测到错误，建议：');
        console.log('   - 增加 MESSAGE_BUFFER_SIZE (当前: 1000)');
        console.log('   - 增加 MAX_CONCURRENT (当前: 1000)');
        console.log('   - 检查网络和RocketMQ服务状态');
    }

    const qps = totalSent / (duration / 1000);
    if (qps < 100) {
        console.log('📈 QPS较低，建议：');
        console.log('   - 减少 FLUSH_INTERVAL (当前: 100ms)');
        console.log('   - 增加 WORKER_POOL_SIZE (当前: 100)');
        console.log('   - 检查代理服务资源使用情况');
    } else if (qps > 1000) {
        console.log('🚀 QPS良好！可以考虑：');
        console.log('   - 进一步增加并发数测试极限');
        console.log('   - 优化消息大小和批处理');
    }

    console.log('\n✨ 测试完成！');
}

// 运行性能测试
performanceTest().catch(console.error); 