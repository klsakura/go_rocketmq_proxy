const grpc = require('@grpc/grpc-js');
const protoLoader = require('@grpc/proto-loader');
const path = require('path');

// 加载proto文件 - 使用本地的proto文件
const PROTO_PATH = path.join(__dirname, 'rocketmq.proto');
const packageDefinition = protoLoader.loadSync(PROTO_PATH, {
    keepCase: true,
    longs: String,
    enums: String,
    defaults: true,
    oneofs: true
});

const rocketmqProto = grpc.loadPackageDefinition(packageDefinition).rocketmq;

// MessageProperties类，兼容你现有的API
class MessageProperties {
    constructor() {
        this.properties = {};
        this.message_key = '';
        this.sharding_key = '';
        this.start_deliver_time = 0;
        this.trans_check_immunity_time = 0;
    }

    putProperty(key, value) {
        this.properties[key] = String(value);
        return this;
    }

    messageKey(key) {
        this.message_key = String(key);
        return this;
    }

    shardingKey(key) {
        this.sharding_key = String(key);
        return this;
    }

    startDeliverTime(time) {
        this.start_deliver_time = Number(time);
        return this;
    }

    transCheckImmunityTime(time) {
        this.trans_check_immunity_time = Number(time);
        return this;
    }
}

// Producer类
class Producer {
    constructor(client, producerId, topic) {
        this.client = client;
        this.producerId = producerId;
        this.topic = topic;
    }

    // 发送普通消息
    async publishMessage(messageBody, tag = '', properties = null) {
        return new Promise((resolve, reject) => {
            const request = {
                producer_id: this.producerId,
                message_body: typeof messageBody === 'object' ? JSON.stringify(messageBody) : messageBody,
                tag: tag,
                properties: properties
            };

            this.client.SendMessage(request, (error, response) => {
                if (error) {
                    reject(new Error(`Failed to send message: ${error.message}`));
                } else if (!response.success) {
                    reject(new Error(response.message));
                } else {
                    resolve({
                        messageId: response.message_id,
                        receiptHandle: response.receipt_handle
                    });
                }
            });
        });
    }

    // 发送顺序消息
    async publishOrderedMessage(messageBody, tag = '', properties = null, shardingKey) {
        return new Promise((resolve, reject) => {
            const request = {
                producer_id: this.producerId,
                message_body: typeof messageBody === 'object' ? JSON.stringify(messageBody) : messageBody,
                tag: tag,
                properties: properties,
                sharding_key: shardingKey
            };

            this.client.SendOrderedMessage(request, (error, response) => {
                if (error) {
                    reject(new Error(`Failed to send ordered message: ${error.message}`));
                } else if (!response.success) {
                    reject(new Error(response.message));
                } else {
                    resolve({
                        messageId: response.message_id,
                        receiptHandle: response.receipt_handle
                    });
                }
            });
        });
    }

    // 发送延迟消息
    async publishDelayMessage(messageBody, tag = '', properties = null, options = {}) {
        return new Promise((resolve, reject) => {
            const request = {
                producer_id: this.producerId,
                message_body: typeof messageBody === 'object' ? JSON.stringify(messageBody) : messageBody,
                tag: tag,
                properties: properties,
                delay_time_level: options.delayTimeLevel || 0,
                start_deliver_time: options.startDeliverTime || 0
            };

            this.client.SendDelayMessage(request, (error, response) => {
                if (error) {
                    reject(new Error(`Failed to send delay message: ${error.message}`));
                } else if (!response.success) {
                    reject(new Error(response.message));
                } else {
                    resolve({
                        messageId: response.message_id,
                        receiptHandle: response.receipt_handle
                    });
                }
            });
        });
    }

    // 发送事务消息
    async publishTransactionMessage(messageBody, tag = '', properties = null, transCheckImmunityTime = 5) {
        return new Promise((resolve, reject) => {
            const request = {
                producer_id: this.producerId,
                message_body: typeof messageBody === 'object' ? JSON.stringify(messageBody) : messageBody,
                tag: tag,
                properties: properties,
                trans_check_immunity_time: transCheckImmunityTime
            };

            this.client.SendTransactionMessage(request, (error, response) => {
                if (error) {
                    reject(new Error(`Failed to send transaction message: ${error.message}`));
                } else if (!response.success) {
                    reject(new Error(response.message));
                } else {
                    resolve({
                        messageId: response.message_id,
                        receiptHandle: response.receipt_handle,
                        transactionId: response.transaction_id
                    });
                }
            });
        });
    }
}

// Consumer类
class Consumer {
    constructor(client, consumerId, topic, groupId) {
        this.client = client;
        this.consumerId = consumerId;
        this.topic = topic;
        this.groupId = groupId;
        this.messageHandlers = [];
    }

    onMessage(handler) {
        this.messageHandlers.push(handler);
    }

    startReceiving() {
        const request = { consumer_id: this.consumerId };
        const stream = this.client.ReceiveMessages(request);

        stream.on('data', (message) => {
            const processedMessage = {
                messageId: message.message_id,
                receiptHandle: message.receipt_handle,
                body: message.message_body,
                tag: message.tag,
                properties: message.properties,
                bornTimestamp: message.born_timestamp,
                reconsumeTimes: message.reconsume_times
            };

            // 调用所有注册的消息处理器
            this.messageHandlers.forEach(handler => {
                try {
                    handler(processedMessage);
                } catch (error) {
                    console.error('Error in message handler:', error);
                }
            });
        });

        stream.on('error', (error) => {
            console.error('Consumer stream error:', error);
        });

        stream.on('end', () => {
            console.log('Consumer stream ended');
        });

        return stream;
    }

    async ackMessage(receiptHandle) {
        return new Promise((resolve, reject) => {
            const request = {
                consumer_id: this.consumerId,
                receipt_handle: receiptHandle
            };

            this.client.AckMessage(request, (error, response) => {
                if (error) {
                    reject(error);
                } else if (!response.success) {
                    reject(new Error(response.message));
                } else {
                    resolve(response);
                }
            });
        });
    }
}

// 主客户端类
class MQClient {
    constructor(config) {
        this.config = {
            endpoint: config.endpoint,
            accessKeyId: config.accessKeyId,
            accessKeySecret: config.accessKeySecret,
            instanceId: config.instanceId,
            grpcEndpoint: config.grpcEndpoint || 'localhost:50051'
        };

        // 创建gRPC客户端
        this.client = new rocketmqProto.RocketMQProxy(
            this.config.grpcEndpoint,
            grpc.credentials.createInsecure()
        );
    }

    async getProducer(instanceId, topic) {
        return new Promise((resolve, reject) => {
            const request = {
                endpoint: this.config.endpoint,
                access_key_id: this.config.accessKeyId,
                access_key_secret: this.config.accessKeySecret,
                instance_id: instanceId,
                topic: topic
            };

            this.client.CreateProducer(request, (error, response) => {
                if (error) {
                    reject(error);
                } else if (!response.success) {
                    reject(new Error(response.message));
                } else {
                    resolve(new Producer(this.client, response.producer_id, topic));
                }
            });
        });
    }

    async getConsumer(instanceId, topic, groupId, tagExpression = '*') {
        return new Promise((resolve, reject) => {
            const request = {
                endpoint: this.config.endpoint,
                access_key_id: this.config.accessKeyId,
                access_key_secret: this.config.accessKeySecret,
                instance_id: instanceId,
                topic: topic,
                group_id: groupId,
                tag_expression: tagExpression
            };

            this.client.CreateConsumer(request, (error, response) => {
                if (error) {
                    reject(error);
                } else if (!response.success) {
                    reject(new Error(response.message));
                } else {
                    resolve(new Consumer(this.client, response.consumer_id, topic, groupId));
                }
            });
        });
    }

    async healthCheck() {
        return new Promise((resolve, reject) => {
            this.client.HealthCheck({}, (error, response) => {
                if (error) {
                    reject(error);
                } else {
                    resolve(response);
                }
            });
        });
    }
}

module.exports = {
    MQClient,
    MessageProperties,
    Producer,
    Consumer
}; 