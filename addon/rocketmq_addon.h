#ifndef ROCKETMQ_ADDON_H
#define ROCKETMQ_ADDON_H

#include <node.h>
#include <node_object_wrap.h>
#include <v8.h>
#include <uv.h>
#include <string>
#include <map>
#include <memory>
#include <functional>

// Go CGO库的C接口声明
extern "C"
{
    typedef void (*MessageHandler)(const char *messageJson);

    char *InitRocketMQ(const char *configJson);
    char *CreateProducer(const char *configJson, const char *topic);
    char *SendMessage(const char *producerId, const char *messageBody, const char *tag, const char *propertiesJson);
    char *SendOrderedMessage(const char *producerId, const char *messageBody, const char *tag, const char *propertiesJson, const char *shardingKey);
    char *CreateConsumer(const char *configJson, const char *topic, const char *groupId, const char *tagExpression);
    char *StartConsumer(const char *consumerId, const char *topic, const char *tagExpression);
    char *RegisterMessageHandler(const char *consumerId, MessageHandler handler);
    char *AckMessage(const char *consumerId, const char *receiptHandle);
    char *ShutdownProducer(const char *producerId);
    char *ShutdownConsumer(const char *consumerId);
    void FreeString(char *str);
}

namespace rocketmq_addon
{

    // 消息处理器包装类
    class MessageHandlerWrapper
    {
    public:
        MessageHandlerWrapper(v8::Isolate *isolate, v8::Local<v8::Function> callback);
        ~MessageHandlerWrapper();

        void HandleMessage(const char *messageJson);
        static void StaticHandleMessage(const char *messageJson);

        static std::map<std::string, std::shared_ptr<MessageHandlerWrapper>> handlers_;
        static std::string current_consumer_id_;

    private:
        v8::Isolate *isolate_;
        v8::Persistent<v8::Function> callback_;
    };

    // RocketMQ客户端包装类
    class RocketMQClient : public node::ObjectWrap
    {
    public:
        static void Init(v8::Local<v8::Object> exports);
        static void NewInstance(const v8::FunctionCallbackInfo<v8::Value> &args);

    private:
        explicit RocketMQClient();
        ~RocketMQClient();

        static void New(const v8::FunctionCallbackInfo<v8::Value> &args);
        static void InitRocketMQ(const v8::FunctionCallbackInfo<v8::Value> &args);
        static void CreateProducer(const v8::FunctionCallbackInfo<v8::Value> &args);
        static void SendMessage(const v8::FunctionCallbackInfo<v8::Value> &args);
        static void SendOrderedMessage(const v8::FunctionCallbackInfo<v8::Value> &args);
        static void CreateConsumer(const v8::FunctionCallbackInfo<v8::Value> &args);
        static void StartConsumer(const v8::FunctionCallbackInfo<v8::Value> &args);
        static void RegisterMessageHandler(const v8::FunctionCallbackInfo<v8::Value> &args);
        static void AckMessage(const v8::FunctionCallbackInfo<v8::Value> &args);
        static void ShutdownProducer(const v8::FunctionCallbackInfo<v8::Value> &args);
        static void ShutdownConsumer(const v8::FunctionCallbackInfo<v8::Value> &args);

        static v8::Persistent<v8::Function> constructor;

        std::string config_json_;
        std::map<std::string, std::string> producers_;
        std::map<std::string, std::string> consumers_;
        std::map<std::string, std::shared_ptr<MessageHandlerWrapper>> message_handlers_;
    };

    // 生产者包装类
    class Producer : public node::ObjectWrap
    {
    public:
        static void Init(v8::Local<v8::Object> exports);
        static void NewInstance(const v8::FunctionCallbackInfo<v8::Value> &args);
        static v8::Persistent<v8::Function> constructor;

    private:
        explicit Producer(const std::string &producer_id, const std::string &topic);
        ~Producer();

        static void New(const v8::FunctionCallbackInfo<v8::Value> &args);
        static void PublishMessage(const v8::FunctionCallbackInfo<v8::Value> &args);
        static void PublishOrderedMessage(const v8::FunctionCallbackInfo<v8::Value> &args);
        static void PublishDelayMessage(const v8::FunctionCallbackInfo<v8::Value> &args);
        static void Shutdown(const v8::FunctionCallbackInfo<v8::Value> &args);

        std::string producer_id_;
        std::string topic_;
    };

    // 消费者包装类
    class Consumer : public node::ObjectWrap
    {
    public:
        static void Init(v8::Local<v8::Object> exports);
        static void NewInstance(const v8::FunctionCallbackInfo<v8::Value> &args);
        static v8::Persistent<v8::Function> constructor;

    private:
        explicit Consumer(const std::string &consumer_id, const std::string &topic, const std::string &group_id);
        ~Consumer();

        static void New(const v8::FunctionCallbackInfo<v8::Value> &args);
        static void OnMessage(const v8::FunctionCallbackInfo<v8::Value> &args);
        static void StartReceiving(const v8::FunctionCallbackInfo<v8::Value> &args);
        static void AckMessage(const v8::FunctionCallbackInfo<v8::Value> &args);
        static void Shutdown(const v8::FunctionCallbackInfo<v8::Value> &args);

        std::string consumer_id_;
        std::string topic_;
        std::string group_id_;
        std::shared_ptr<MessageHandlerWrapper> message_handler_;
    };

    // 工具函数
    std::string V8StringToStdString(v8::Isolate *isolate, v8::Local<v8::Value> value);
    v8::Local<v8::String> StdStringToV8String(v8::Isolate *isolate, const std::string &str);
    v8::Local<v8::Object> JsonStringToV8Object(v8::Isolate *isolate, const std::string &json_str);
    std::string V8ObjectToJsonString(v8::Isolate *isolate, v8::Local<v8::Object> obj);

} // namespace rocketmq_addon

#endif // ROCKETMQ_ADDON_H