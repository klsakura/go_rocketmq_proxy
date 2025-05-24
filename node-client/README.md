# RocketMQ Node.js Client 使用指南

## 📦 独立客户端包

这是一个**独立的**RocketMQ Node.js客户端包，包含了所有必要的文件：
- ✅ 完整的客户端SDK (`index.js`)
- ✅ gRPC协议定义 (`rocketmq.proto`)
- ✅ 使用示例和工具

可以**独立部署和使用**，无需依赖整个RocketMQ代理项目。

## 📁 文件说明

### 核心文件
- **`index.js`** - RocketMQ Node.js客户端SDK主文件
- **`rocketmq.proto`** - gRPC协议定义文件

### Demo示例
- **`connection-check.js`** - 连接验证工具
- **`performance-check.js`** - 性能测试工具  
- **`usage-demo.js`** - 实际使用示例

---

## 🔍 1. 连接验证 (`connection-check.js`)

**用途**: 验证RocketMQ连接和基本功能是否正常

**使用方法**:
```bash
node connection-check.js
```

**验证内容**:
- ✅ 代理服务连接状态
- ✅ 生产者创建
- ✅ 消息发送
- ✅ 消费者创建
- ✅ 配置信息验证

**适用场景**: 
- 首次部署时验证环境
- 排查连接问题
- 健康检查

---

## ⚡ 2. 性能验证 (`performance-check.js`)

**用途**: 测试RocketMQ在不同负载下的性能表现

**使用方法**:
```bash
node performance-check.js [consumers] [messages] [batchSize]

# 示例
node performance-check.js 2 1000 50  # 2个消费者，1000条消息，批量大小50
```

**测试指标**:
- 📊 发送QPS (每秒消息数)
- 📊 消费QPS 
- 📊 端到端延迟
- 📊 内存使用情况
- 📊 连接复用效果

**适用场景**:
- 容量规划
- 性能调优
- 压力测试

---

## 📚 3. 实际使用示例 (`usage-demo.js`)

**用途**: 展示真实业务场景下的完整使用方法

**使用方法**:
```bash
# 启动消费者
node usage-demo.js consumer

# 发送消息（另开终端）
node usage-demo.js producer
```

**业务场景**: 电商订单处理流程
- 📦 **订单创建** (普通消息)
- 🔄 **状态更新** (顺序消息，保证同用户订单顺序)
- ⏰ **超时检查** (延迟消息，30分钟后执行)

**消息类型展示**:
- ✅ 普通消息: `publishMessage()`
- ✅ 顺序消息: `publishOrderedMessage()`  
- ✅ 延迟消息: `publishDelayMessage()`
- ✅ 事务消息: `publishTransactionMessage()`

**适用场景**:
- 学习参考
- 代码模板
- 业务集成

---

## ⚙️ 配置说明

所有demo都使用统一的配置格式：

```javascript
const config = {
    endpoint: 'http://rocketmq-xxx.rocketmq.volces.com:9876',
    accessKeyId: 'your-access-key',
    accessKeySecret: 'your-secret-key', 
    instanceId: 'your-instance-id',
    grpcEndpoint: 'localhost:50051'  // Go代理服务地址
};
```

**重要提醒**:
- 确保Go代理服务已启动: `./bin/rocketmq-proxy`
- Topic需要在字节跳动云控制台预先创建
- instanceId用于资源隔离，不可省略

---

## 🚀 快速开始

1. **启动Go代理服务**:
```bash
cd .. && ./bin/rocketmq-proxy
```

2. **验证连接**:
```bash
node connection-check.js
```

3. **运行完整示例**:
```bash
# 终端1 - 启动消费者
node usage-demo.js consumer

# 终端2 - 发送消息
node usage-demo.js producer
```

4. **性能测试**:
```bash
node performance-check.js 3 500 25
```

---

## 📞 技术支持

如遇问题，请检查：
1. Go代理服务是否正常运行
2. RocketMQ配置信息是否正确
3. 网络连接是否畅通
4. Topic是否已在控制台创建

---

**Happy Messaging! 🚀** 