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
    properties: {
        [key: string]: string;
    };
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
export declare class MessageProperties {
    properties: {
        [key: string]: string;
    };
    message_key: string;
    sharding_key: string;
    start_deliver_time: number;
    trans_check_immunity_time: number;
    putProperty(key: string, value: string | number): MessageProperties;
    messageKey(key: string): MessageProperties;
    shardingKey(key: string): MessageProperties;
    startDeliverTime(time: number): MessageProperties;
    transCheckImmunityTime(time: number): MessageProperties;
}
export declare class Producer {
    private client;
    private producerId;
    private topic;
    constructor(client: any, producerId: string, topic: string);
    publishMessage(messageBody: any, tag?: string, properties?: MessageProperties | null): Promise<SendResult>;
    publishOrderedMessage(messageBody: any, tag?: string, properties?: MessageProperties | null, shardingKey?: string): Promise<SendResult>;
    publishDelayMessage(messageBody: any, tag?: string, properties?: MessageProperties | null, options?: DelayOptions): Promise<SendResult>;
    publishTransactionMessage(messageBody: any, tag?: string, properties?: MessageProperties | null, transCheckImmunityTime?: number): Promise<TransactionResult>;
}
export declare class Consumer {
    private client;
    private consumerId;
    private topic;
    private groupId;
    private messageHandlers;
    constructor(client: any, consumerId: string, topic: string, groupId: string);
    onMessage(handler: (message: MessageData) => void | Promise<void>): void;
    startReceiving(): any;
    ackMessage(receiptHandle: string): Promise<any>;
}
export declare class MQClient {
    private config;
    private client;
    constructor(config: MQConfig);
    getProducer(instanceId: string, topic: string): Promise<Producer>;
    getConsumer(instanceId: string, topic: string, groupId: string, tagExpression?: string): Promise<Consumer>;
    healthCheck(): Promise<any>;
}
declare const _default: {
    MQClient: typeof MQClient;
    MessageProperties: typeof MessageProperties;
    Producer: typeof Producer;
    Consumer: typeof Consumer;
};
export default _default;
