import * as grpc from '@grpc/grpc-js';
import * as protoLoader from '@grpc/proto-loader';
import * as path from 'path';

// 接口定义
export interface MQConfig {
    endpoint: string;
    accessKeyId: string;
    accessKeySecret: string;
    instanceId: string;
    grpcEndpoint?: string;
}

export interface MessageData {
    messageId: string;
    receiptHandle: string;
    body: string;
    tag: string;
    properties: { [key: string]: string };
    bornTimestamp: number;
    reconsumeTimes: number;
}

export interface SendResult {
    messageId: string;
    receiptHandle: string;
}

export interface TransactionResult extends SendResult {
    transactionId: string;
}

export interface DelayOptions {
    delayTimeLevel?: number;
    startDeliverTime?: number;
}

// 加载proto文件 - 使用同目录下的proto文件
const PROTO_PATH = path.join(__dirname, 'rocketmq.proto');
const packageDefinition = protoLoader.loadSync(PROTO_PATH, {
    keepCase: true,
    longs: String,
    enums: String,
    defaults: true,
    oneofs: true
});

const rocketmqProto = grpc.loadPackageDefinition(packageDefinition).rocketmq as any;

// MessageProperties类，兼容你现有的API
export class MessageProperties {
    public properties: { [key: string]: string } = {};
    public message_key: string = '';
    public sharding_key: string = '';
    public start_deliver_time: number = 0;
    public trans_check_immunity_time: number = 0;

    putProperty(key: string, value: string | number): MessageProperties {
        this.properties[key] = String(value);
        return this;
    }

    messageKey(key: string): MessageProperties {
        this.message_key = String(key);
        return this;
    }

    shardingKey(key: string): MessageProperties {
        this.sharding_key = String(key);
        return this;
    }

    startDeliverTime(time: number): MessageProperties {
        this.start_deliver_time = Number(time);
        return this;
    }

    transCheckImmunityTime(time: number): MessageProperties {
        this.trans_check_immunity_time = Number(time);
        return this;
    }
}

// Producer类
export class Producer {
    constructor(
        private client: any,
        private producerId: string,
        private topic: string
    ) { }

    // 发送普通消息
    async publishMessage(
        messageBody: any,
        tag: string = '',
        properties: MessageProperties | null = null
    ): Promise<SendResult> {
        return new Promise((resolve, reject) => {
            const request = {
                producer_id: this.producerId,
                message_body: typeof messageBody === 'object' ? JSON.stringify(messageBody) : messageBody,
                tag: tag,
                properties: properties
            };

            this.client.SendMessage(request, (error: any, response: any) => {
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
    async publishOrderedMessage(
        messageBody: any,
        tag: string = '',
        properties: MessageProperties | null = null,
        shardingKey?: string
    ): Promise<SendResult> {
        return new Promise((resolve, reject) => {
            const request = {
                producer_id: this.producerId,
                message_body: typeof messageBody === 'object' ? JSON.stringify(messageBody) : messageBody,
                tag: tag,
                properties: properties,
                sharding_key: shardingKey
            };

            this.client.SendOrderedMessage(request, (error: any, response: any) => {
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
    async publishDelayMessage(
        messageBody: any,
        tag: string = '',
        properties: MessageProperties | null = null,
        options: DelayOptions = {}
    ): Promise<SendResult> {
        return new Promise((resolve, reject) => {
            const request = {
                producer_id: this.producerId,
                message_body: typeof messageBody === 'object' ? JSON.stringify(messageBody) : messageBody,
                tag: tag,
                properties: properties,
                delay_time_level: options.delayTimeLevel || 0,
                start_deliver_time: options.startDeliverTime || 0
            };

            this.client.SendDelayMessage(request, (error: any, response: any) => {
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
    async publishTransactionMessage(
        messageBody: any,
        tag: string = '',
        properties: MessageProperties | null = null,
        transCheckImmunityTime: number = 5
    ): Promise<TransactionResult> {
        return new Promise((resolve, reject) => {
            const request = {
                producer_id: this.producerId,
                message_body: typeof messageBody === 'object' ? JSON.stringify(messageBody) : messageBody,
                tag: tag,
                properties: properties,
                trans_check_immunity_time: transCheckImmunityTime
            };

            this.client.SendTransactionMessage(request, (error: any, response: any) => {
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
export class Consumer {
    private messageHandlers: Array<(message: MessageData) => void | Promise<void>> = [];

    constructor(
        private client: any,
        private consumerId: string,
        private topic: string,
        private groupId: string
    ) { }

    onMessage(handler: (message: MessageData) => void | Promise<void>): void {
        this.messageHandlers.push(handler);
    }

    startReceiving(): any {
        const request = { consumer_id: this.consumerId };
        const stream = this.client.ReceiveMessages(request);

        stream.on('data', (message: any) => {
            const processedMessage: MessageData = {
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

        stream.on('error', (error: any) => {
            console.error('Consumer stream error:', error);
        });

        stream.on('end', () => {
            console.log('Consumer stream ended');
        });

        return stream;
    }

    async ackMessage(receiptHandle: string): Promise<any> {
        return new Promise((resolve, reject) => {
            const request = {
                consumer_id: this.consumerId,
                receipt_handle: receiptHandle
            };

            this.client.AckMessage(request, (error: any, response: any) => {
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
export class MQClient {
    private config: Required<MQConfig>;
    private client: any;

    constructor(config: MQConfig) {
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

    async getProducer(instanceId: string, topic: string): Promise<Producer> {
        return new Promise((resolve, reject) => {
            const request = {
                endpoint: this.config.endpoint,
                access_key_id: this.config.accessKeyId,
                access_key_secret: this.config.accessKeySecret,
                instance_id: instanceId,
                topic: topic
            };

            this.client.CreateProducer(request, (error: any, response: any) => {
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

    async getConsumer(
        instanceId: string,
        topic: string,
        groupId: string,
        tagExpression: string = '*'
    ): Promise<Consumer> {
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

            this.client.CreateConsumer(request, (error: any, response: any) => {
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

    async healthCheck(): Promise<any> {
        return new Promise((resolve, reject) => {
            this.client.HealthCheck({}, (error: any, response: any) => {
                if (error) {
                    reject(error);
                } else {
                    resolve(response);
                }
            });
        });
    }
}

// 默认导出
export default {
    MQClient,
    MessageProperties,
    Producer,
    Consumer
}; 