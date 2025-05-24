// 配置模板文件
// 复制此文件为 config.js 并填入你的实际配置信息

module.exports = {
    // RocketMQ 实例配置
    endpoint: 'http://your-rocketmq-instance.rocketmq.volces.com:9876',
    accessKeyId: 'your-access-key-id',
    accessKeySecret: 'your-access-key-secret',
    instanceId: 'your-instance-id',

    // 本地代理服务配置
    grpcEndpoint: 'localhost:50051',

    // 测试配置
    testTopic: 'upload',
    testConsumerGroup: 'GID_group'
};

// 使用方法：
// 1. 复制此文件为 config.js
// 2. 填入你的实际配置信息
// 3. 在示例文件中引入: const config = require('./config'); 