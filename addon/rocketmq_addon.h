#ifndef ROCKETMQ_ADDON_H
#define ROCKETMQ_ADDON_H

// Windows API 冲突保护 - 必须在所有其他include之前
#ifdef _WIN32
#define WIN32_LEAN_AND_MEAN
#ifndef NOMINMAX
#define NOMINMAX
#endif
// 防止Windows.h中的SendMessage宏冲突
#ifdef SendMessage
#undef SendMessage
#endif
#endif

#include <napi.h>
#include <string>
#include <map>
#include <memory>
#include <functional>
#include <vector>
#include <iostream>

#ifdef _WIN32
#include <windows.h>
// 重新禁用可能的宏冲突
#ifdef SendMessage
#undef SendMessage
#endif
#else
#include <dlfcn.h>
#endif

// Go CGO库的C接口声明
extern "C"
{
    typedef void (*MessageHandler)(const char *messageJson);

    char *InitRocketMQ(const char *configJson);
    char *CreateProducer(const char *configJson, const char *topic);
    char *SendRocketMQMessage(const char *producerId, const char *messageBody, const char *tag, const char *propertiesJson);
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

    // 线程安全的消息数据结构
    struct MessageCallbackData
    {
        std::string messageJson;
        std::string consumerId;
    };

    // 消息处理器包装类 - 使用Node-API获得更好的ABI稳定性
    class MessageHandlerWrapper
    {
    public:
        MessageHandlerWrapper(Napi::Env env, Napi::Function callback);
        ~MessageHandlerWrapper();

        void HandleMessage(const char *messageJson);
        static void StaticHandleMessage(const char *messageJson);

        // 新增：支持直接指定消费者ID的静态处理函数
        static void StaticHandleMessageWithConsumerId(const char *consumerId, const char *messageJson);

        // 线程安全回调相关
        void SetupThreadSafeCallback(const std::string &consumerId);
        void CleanupThreadSafeCallback();

        static std::map<std::string, std::shared_ptr<MessageHandlerWrapper>> handlers_;

    private:
        Napi::Env env_;
        Napi::FunctionReference callback_;
        Napi::ThreadSafeFunction thread_safe_callback_;
        std::string consumer_id_;

        // 线程安全回调函数
        static void ThreadSafeCallback(Napi::Env env, Napi::Function jsCallback, MessageCallbackData *data);
    };

    // RocketMQ客户端包装类 - 继承自Napi::ObjectWrap以获得跨版本兼容性
    class RocketMQClient : public Napi::ObjectWrap<RocketMQClient>
    {
    public:
        static Napi::Object Init(Napi::Env env, Napi::Object exports);
        static Napi::Value NewInstance(const Napi::CallbackInfo &info);

        RocketMQClient(const Napi::CallbackInfo &info);
        ~RocketMQClient();

    private:
        static Napi::FunctionReference constructor;

        Napi::Value InitRocketMQ(const Napi::CallbackInfo &info);
        Napi::Value CreateProducer(const Napi::CallbackInfo &info);
        Napi::Value SendMessage(const Napi::CallbackInfo &info);
        Napi::Value SendOrderedMessage(const Napi::CallbackInfo &info);
        Napi::Value CreateConsumer(const Napi::CallbackInfo &info);
        Napi::Value StartConsumer(const Napi::CallbackInfo &info);
        Napi::Value RegisterMessageHandler(const Napi::CallbackInfo &info);
        Napi::Value AckMessage(const Napi::CallbackInfo &info);
        Napi::Value ShutdownProducer(const Napi::CallbackInfo &info);
        Napi::Value ShutdownConsumer(const Napi::CallbackInfo &info);

        std::string config_json_;
        std::map<std::string, std::string> producers_;
        std::map<std::string, std::string> consumers_;
        std::map<std::string, std::shared_ptr<MessageHandlerWrapper>> message_handlers_;
    };

    // 生产者包装类 - 使用Node-API确保跨Node.js版本的ABI兼容性
    class Producer : public Napi::ObjectWrap<Producer>
    {
    public:
        static Napi::Object Init(Napi::Env env, Napi::Object exports);
        static Napi::Value NewInstance(const Napi::CallbackInfo &info);
        static Napi::FunctionReference constructor;

        Producer(const Napi::CallbackInfo &info);
        ~Producer();

    private:
        Napi::Value PublishMessage(const Napi::CallbackInfo &info);
        Napi::Value PublishOrderedMessage(const Napi::CallbackInfo &info);
        Napi::Value PublishDelayMessage(const Napi::CallbackInfo &info);
        Napi::Value Shutdown(const Napi::CallbackInfo &info);

        std::string producer_id_;
        std::string topic_;
    };

    // 消费者包装类 - 基于Node-API以避免V8 ABI依赖问题
    class Consumer : public Napi::ObjectWrap<Consumer>
    {
    public:
        static Napi::Object Init(Napi::Env env, Napi::Object exports);
        static Napi::Value NewInstance(const Napi::CallbackInfo &info);
        static Napi::FunctionReference constructor;

        Consumer(const Napi::CallbackInfo &info);
        ~Consumer();

    private:
        Napi::Value OnMessage(const Napi::CallbackInfo &info);
        Napi::Value StartReceiving(const Napi::CallbackInfo &info);
        Napi::Value AckMessage(const Napi::CallbackInfo &info);
        Napi::Value Shutdown(const Napi::CallbackInfo &info);

        std::string consumer_id_;
        std::string topic_;
        std::string group_id_;
        std::shared_ptr<MessageHandlerWrapper> message_handler_;
    };

    // 工具函数 - 使用Node-API类型以保证跨版本稳定性
    std::string NapiStringToStdString(const Napi::String &napiStr);
    Napi::String StdStringToNapiString(Napi::Env env, const std::string &str);
    Napi::Object JsonStringToNapiObject(Napi::Env env, const std::string &json_str);
    std::string NapiObjectToJsonString(const Napi::Object &obj);

} // namespace rocketmq_addon

#endif // ROCKETMQ_ADDON_H