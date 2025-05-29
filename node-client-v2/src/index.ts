import * as path from 'path';

// 动态导入平台加载器（支持ESM和CommonJS）
let loadNativeAddon: () => any;
try {
    // 尝试使用require (CommonJS)
    loadNativeAddon = require('./platform-loader').loadNativeAddon;
} catch (e) {
    // 如果require失败，将在运行时使用动态import (ESM)
    loadNativeAddon = null as any;
}

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
    }

    public async initNativeClient() {
        try {
            // 如果是ESM环境，使用动态导入
            if (!loadNativeAddon) {
                // @ts-ignore - 动态导入平台加载器
                const module = await import('./platform-loader.js');
                loadNativeAddon = module.loadNativeAddon;
            }

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
        const configJson = JSON.stringify(this.config);
        const producer = this.nativeClient.createProducer(configJson, topic);
        if (producer.success === false) {
            throw new Error(`Failed to create producer: ${producer.message}`);
        }
        return new NativeProducer(producer, this.nativeClient);
    }

    async getConsumer(instanceId: string, topic: string, groupId: string, tagExpression: string = '*'): Promise<Consumer> {
        const configJson = JSON.stringify(this.config);
        const consumer = this.nativeClient.createConsumer(configJson, topic, groupId, tagExpression);
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
    constructor(private producerInfo: any, private nativeClient: any) {
        super();
    }

    async publishMessage(messageBody: any, tag: string = '', properties: MessageProperties | null = null): Promise<SendResult> {
        const body = typeof messageBody === 'object' ? JSON.stringify(messageBody) : String(messageBody);
        const props = properties ? JSON.stringify(properties.toJSON()) : '{}';

        const result = this.nativeClient.sendMessage(this.producerInfo.producerId, body, tag, props);
        if (!result || typeof result === 'string') {
            // 如果返回字符串，假设是成功的消息ID
            return {
                messageId: result || 'unknown',
                receiptHandle: result || 'unknown'
            };
        }

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
        const props = properties ? JSON.stringify(properties.toJSON()) : '{}';

        const result = this.nativeClient.sendOrderedMessage(this.producerInfo.producerId, body, tag, props, shardingKey || '');
        if (!result || typeof result === 'string') {
            // 如果返回字符串，假设是成功的消息ID
            return {
                messageId: result || 'unknown',
                receiptHandle: result || 'unknown'
            };
        }

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
        const result = this.nativeClient.sendMessage(this.producerInfo.producerId, body, tag, JSON.stringify(props));

        if (!result || typeof result === 'string') {
            // 如果返回字符串，假设是成功的消息ID
            return {
                messageId: result || 'unknown',
                receiptHandle: result || 'unknown'
            };
        }

        if (!result.success) {
            throw new Error(`Failed to send delay message: ${result.message}`);
        }

        return {
            messageId: result.messageId,
            receiptHandle: result.receiptHandle
        };
    }

    async shutdown(): Promise<any> {
        const result = this.nativeClient.shutdownProducer(this.producerInfo.producerId);
        if (!result || typeof result === 'string') {
            return { success: true, message: 'Producer shutdown' };
        }

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
    private initialized: boolean = false;

    constructor(config: MQConfig) {
        this.client = new NativeClient(config);
    }

    private async ensureInitialized() {
        if (!this.initialized) {
            await this.client.initNativeClient();
            this.initialized = true;
        }
    }

    /**
     * 创建生产者
     * @param instanceId 实例ID
     * @param topic 主题
     * @returns 生产者实例
     */
    async getProducer(instanceId: string, topic: string): Promise<Producer> {
        await this.ensureInitialized();
        return this.client.getProducer(instanceId, topic);
    }

    /**
     * 创建消费者
     * @param instanceId 实例ID
     * @param topic 主题
     * @param groupId 消费者组ID
     * @param tagExpression 标签表达式 (默认为'*')
     * @returns 消费者实例
     */
    async getConsumer(instanceId: string, topic: string, groupId: string, tagExpression: string = '*'): Promise<Consumer> {
        await this.ensureInitialized();
        return this.client.getConsumer(instanceId, topic, groupId, tagExpression);
    }

    /**
     * 健康检查
     */
    async healthCheck(): Promise<any> {
        await this.ensureInitialized();
        return this.client.healthCheck();
    }

    /**
     * 获取客户端模式
     */
    getMode(): string {
        return 'native';
    }
}

// 导出所有类型和类
export default MQClient; 