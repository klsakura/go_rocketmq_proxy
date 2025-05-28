import * as path from 'path';

// 导入平台加载器
const { loadNativeAddon } = require('./platform-loader');

// 接口定义
export interface MQConfig {
    endpoint: string;
    accessKeyId: string;
    accessKeySecret: string;
    instanceId: string;
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

export interface DelayOptions {
    delayTimeLevel?: number;
    startDeliverTime?: number;
}

// MessageProperties类
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

    toJSON(): any {
        return {
            properties: this.properties,
            messageKey: this.message_key,
            shardingKey: this.sharding_key,
            startDeliverTime: this.start_deliver_time,
            transCheckImmunityTime: this.trans_check_immunity_time
        };
    }
}

// Native客户端实现
class NativeClient {
    private nativeClient: any;
    private config: MQConfig;

    constructor(config: MQConfig) {
        this.config = config;
        this.initNativeClient();
    }

    private initNativeClient() {
        try {
            // 使用平台加载器加载Native Addon
            const addon = loadNativeAddon();
            this.nativeClient = new addon.RocketMQClient();

            // 初始化RocketMQ
            const configJson = {
                endpoint: this.config.endpoint,
                accessKeyId: this.config.accessKeyId,
                accessKeySecret: this.config.accessKeySecret,
                instanceId: this.config.instanceId
            };

            const result = this.nativeClient.initRocketMQ(configJson);
            if (!result.success) {
                throw new Error(`Failed to initialize RocketMQ: ${result.message}`);
            }

            console.log('🚀 RocketMQ Native Client initialized successfully');
        } catch (error: any) {
            throw new Error(`Failed to load Native Addon: ${error.message}`);
        }
    }

    async getProducer(instanceId: string, topic: string): Promise<Producer> {
        const producer = this.nativeClient.createProducer(instanceId, topic);
        if (producer.success === false) {
            throw new Error(`Failed to create producer: ${producer.message}`);
        }
        return new NativeProducer(producer);
    }

    async getConsumer(instanceId: string, topic: string, groupId: string, tagExpression: string = '*'): Promise<Consumer> {
        const consumer = this.nativeClient.createConsumer(instanceId, topic, groupId, tagExpression);
        if (consumer.success === false) {
            throw new Error(`Failed to create consumer: ${consumer.message}`);
        }
        return new NativeConsumer(consumer);
    }

    async healthCheck(): Promise<any> {
        return {
            status: 'healthy',
            mode: 'native',
            timestamp: Date.now()
        };
    }
}

// 生产者抽象类
export abstract class Producer {
    abstract publishMessage(messageBody: any, tag?: string, properties?: MessageProperties | null): Promise<SendResult>;
    abstract publishOrderedMessage(messageBody: any, tag?: string, properties?: MessageProperties | null, shardingKey?: string): Promise<SendResult>;
    abstract publishDelayMessage(messageBody: any, tag?: string, properties?: MessageProperties | null, options?: DelayOptions): Promise<SendResult>;
    abstract shutdown(): Promise<any>;
}

// Native生产者实现
class NativeProducer extends Producer {
    constructor(private nativeProducer: any) {
        super();
    }

    async publishMessage(messageBody: any, tag: string = '', properties: MessageProperties | null = null): Promise<SendResult> {
        const body = typeof messageBody === 'object' ? JSON.stringify(messageBody) : String(messageBody);
        const props = properties ? properties.toJSON() : {};

        const result = this.nativeProducer.publishMessage(body, tag, props);
        if (!result.success) {
            throw new Error(`Failed to send message: ${result.message}`);
        }

        return {
            messageId: result.messageId,
            receiptHandle: result.receiptHandle
        };
    }

    async publishOrderedMessage(messageBody: any, tag: string = '', properties: MessageProperties | null = null, shardingKey?: string): Promise<SendResult> {
        const body = typeof messageBody === 'object' ? JSON.stringify(messageBody) : String(messageBody);
        const props = properties ? properties.toJSON() : {};

        const result = this.nativeProducer.publishOrderedMessage(body, tag, props, shardingKey || '');
        if (!result.success) {
            throw new Error(`Failed to send ordered message: ${result.message}`);
        }

        return {
            messageId: result.messageId,
            receiptHandle: result.receiptHandle
        };
    }

    async publishDelayMessage(messageBody: any, tag: string = '', properties: MessageProperties | null = null, options: DelayOptions = {}): Promise<SendResult> {
        // 对于延迟消息，将延迟时间设置到properties中
        const props: any = properties ? properties.toJSON() : {};
        if (options.startDeliverTime) {
            props.startDeliverTime = options.startDeliverTime;
        }
        if (options.delayTimeLevel) {
            props.delayTimeLevel = options.delayTimeLevel;
        }

        const body = typeof messageBody === 'object' ? JSON.stringify(messageBody) : String(messageBody);
        const result = this.nativeProducer.publishDelayMessage(body, tag, props);
        if (!result.success) {
            throw new Error(`Failed to send delay message: ${result.message}`);
        }

        return {
            messageId: result.messageId,
            receiptHandle: result.receiptHandle
        };
    }

    async shutdown(): Promise<any> {
        const result = this.nativeProducer.shutdown();
        if (!result.success) {
            throw new Error(`Failed to shutdown producer: ${result.message}`);
        }
        return result;
    }
}

// 消费者抽象类
export abstract class Consumer {
    protected messageHandlers: Array<(message: MessageData) => void | Promise<void>> = [];

    abstract onMessage(handler: (message: MessageData) => void | Promise<void>): void;
    abstract startReceiving(tagExpression?: string): any;
    abstract ackMessage(receiptHandle: string): Promise<any>;
    abstract shutdown(): Promise<any>;
}

// Native消费者实现
class NativeConsumer extends Consumer {
    constructor(private nativeConsumer: any) {
        super();
    }

    onMessage(handler: (message: MessageData) => void | Promise<void>): void {
        this.messageHandlers.push(handler);

        // 注册消息处理器到Native Addon
        this.nativeConsumer.onMessage(async (messageData: MessageData) => {
            for (const h of this.messageHandlers) {
                try {
                    await h(messageData);
                } catch (error) {
                    console.error('Error in message handler:', error);
                }
            }
        });
    }

    startReceiving(tagExpression: string = '*'): any {
        const result = this.nativeConsumer.startReceiving(tagExpression);
        if (!result.success) {
            throw new Error(`Failed to start receiving: ${result.message}`);
        }
        return result;
    }

    async ackMessage(receiptHandle: string): Promise<any> {
        const result = this.nativeConsumer.ackMessage(receiptHandle);
        if (!result.success) {
            throw new Error(`Failed to ack message: ${result.message}`);
        }
        return result;
    }

    async shutdown(): Promise<any> {
        const result = this.nativeConsumer.shutdown();
        if (!result.success) {
            throw new Error(`Failed to shutdown consumer: ${result.message}`);
        }
        return result;
    }
}

// 主客户端类
export class MQClient {
    private client: NativeClient;

    constructor(config: MQConfig) {
        // 验证必需的配置参数
        if (!config.endpoint) {
            throw new Error('endpoint is required');
        }
        if (!config.accessKeyId) {
            throw new Error('accessKeyId is required');
        }
        if (!config.accessKeySecret) {
            throw new Error('accessKeySecret is required');
        }
        if (!config.instanceId) {
            throw new Error('instanceId is required');
        }

        this.client = new NativeClient(config);
    }

    async getProducer(instanceId: string, topic: string): Promise<Producer> {
        if (!instanceId) {
            throw new Error('instanceId is required');
        }
        if (!topic) {
            throw new Error('topic is required');
        }
        return this.client.getProducer(instanceId, topic);
    }

    async getConsumer(instanceId: string, topic: string, groupId: string, tagExpression: string = '*'): Promise<Consumer> {
        if (!instanceId) {
            throw new Error('instanceId is required');
        }
        if (!topic) {
            throw new Error('topic is required');
        }
        if (!groupId) {
            throw new Error('groupId is required');
        }
        return this.client.getConsumer(instanceId, topic, groupId, tagExpression);
    }

    async healthCheck(): Promise<any> {
        return this.client.healthCheck();
    }

    // 获取当前使用的模式
    getMode(): string {
        return 'native';
    }
}

// 导出所有类型和类
export default MQClient; 