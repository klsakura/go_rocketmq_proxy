#include "rocketmq_addon.h"
#include <dlfcn.h>
#include <iostream>
#include <sstream>

namespace rocketmq_addon
{

    // 静态成员初始化
    std::map<std::string, std::shared_ptr<MessageHandlerWrapper>> MessageHandlerWrapper::handlers_;
    std::string MessageHandlerWrapper::current_consumer_id_;

    v8::Persistent<v8::Function> RocketMQClient::constructor;
    v8::Persistent<v8::Function> Producer::constructor;
    v8::Persistent<v8::Function> Consumer::constructor;

    // 动态库句柄
    static void *go_lib_handle = nullptr;

    // 动态加载Go库的函数指针
    static char *(*go_InitRocketMQ)(const char *) = nullptr;
    static char *(*go_CreateProducer)(const char *, const char *) = nullptr;
    static char *(*go_SendMessage)(const char *, const char *, const char *, const char *) = nullptr;
    static char *(*go_SendOrderedMessage)(const char *, const char *, const char *, const char *, const char *) = nullptr;
    static char *(*go_CreateConsumer)(const char *, const char *, const char *, const char *) = nullptr;
    static char *(*go_StartConsumer)(const char *, const char *, const char *) = nullptr;
    static char *(*go_RegisterMessageHandler)(const char *, MessageHandler) = nullptr;
    static char *(*go_AckMessage)(const char *, const char *) = nullptr;
    static char *(*go_ShutdownProducer)(const char *) = nullptr;
    static char *(*go_ShutdownConsumer)(const char *) = nullptr;
    static void (*go_FreeString)(char *) = nullptr;

    // 加载Go动态库
    bool LoadGoLibrary()
    {
        if (go_lib_handle != nullptr)
        {
            return true; // 已经加载
        }

        // 尝试加载动态库
        const char *lib_paths[] = {
            "./librocketmq_cgo.so",
            "../cgo/librocketmq_cgo.so",
            "/usr/local/lib/librocketmq_cgo.so",
            nullptr};

        for (int i = 0; lib_paths[i] != nullptr; i++)
        {
            go_lib_handle = dlopen(lib_paths[i], RTLD_LAZY);
            if (go_lib_handle != nullptr)
            {
                break;
            }
        }

        if (go_lib_handle == nullptr)
        {
            std::cerr << "Failed to load Go library: " << dlerror() << std::endl;
            return false;
        }

        // 加载函数符号
        go_InitRocketMQ = (char *(*)(const char *))dlsym(go_lib_handle, "InitRocketMQ");
        go_CreateProducer = (char *(*)(const char *, const char *))dlsym(go_lib_handle, "CreateProducer");
        go_SendMessage = (char *(*)(const char *, const char *, const char *, const char *))dlsym(go_lib_handle, "SendMessage");
        go_SendOrderedMessage = (char *(*)(const char *, const char *, const char *, const char *, const char *))dlsym(go_lib_handle, "SendOrderedMessage");
        go_CreateConsumer = (char *(*)(const char *, const char *, const char *, const char *))dlsym(go_lib_handle, "CreateConsumer");
        go_StartConsumer = (char *(*)(const char *, const char *, const char *))dlsym(go_lib_handle, "StartConsumer");
        go_RegisterMessageHandler = (char *(*)(const char *, MessageHandler))dlsym(go_lib_handle, "RegisterMessageHandler");
        go_AckMessage = (char *(*)(const char *, const char *))dlsym(go_lib_handle, "AckMessage");
        go_ShutdownProducer = (char *(*)(const char *))dlsym(go_lib_handle, "ShutdownProducer");
        go_ShutdownConsumer = (char *(*)(const char *))dlsym(go_lib_handle, "ShutdownConsumer");
        go_FreeString = (void (*)(char *))dlsym(go_lib_handle, "FreeString");

        if (!go_InitRocketMQ || !go_CreateProducer || !go_SendMessage ||
            !go_SendOrderedMessage || !go_CreateConsumer || !go_StartConsumer ||
            !go_RegisterMessageHandler || !go_AckMessage || !go_ShutdownProducer ||
            !go_ShutdownConsumer || !go_FreeString)
        {
            std::cerr << "Failed to load Go library functions" << std::endl;
            dlclose(go_lib_handle);
            go_lib_handle = nullptr;
            return false;
        }

        return true;
    }

    // MessageHandlerWrapper 实现
    MessageHandlerWrapper::MessageHandlerWrapper(v8::Isolate *isolate, v8::Local<v8::Function> callback)
        : isolate_(isolate)
    {
        callback_.Reset(isolate, callback);
    }

    MessageHandlerWrapper::~MessageHandlerWrapper()
    {
        callback_.Reset();
    }

    void MessageHandlerWrapper::HandleMessage(const char *messageJson)
    {
        v8::HandleScope handle_scope(isolate_);
        v8::Local<v8::Context> context = isolate_->GetCurrentContext();

        v8::Local<v8::Function> callback = v8::Local<v8::Function>::New(isolate_, callback_);

        // 解析JSON消息
        v8::Local<v8::Object> messageObj = JsonStringToV8Object(isolate_, std::string(messageJson));

        v8::Local<v8::Value> argv[] = {messageObj};

        v8::TryCatch try_catch(isolate_);
        callback->Call(context, v8::Null(isolate_), 1, argv).ToLocalChecked();

        if (try_catch.HasCaught())
        {
            v8::Local<v8::Message> message = try_catch.Message();
            v8::String::Utf8Value error(isolate_, message->Get());
            std::cerr << "Error in message handler: " << *error << std::endl;
        }
    }

    void MessageHandlerWrapper::StaticHandleMessage(const char *messageJson)
    {
        if (!current_consumer_id_.empty())
        {
            auto it = handlers_.find(current_consumer_id_);
            if (it != handlers_.end())
            {
                it->second->HandleMessage(messageJson);
            }
        }
    }

    // RocketMQClient 实现
    RocketMQClient::RocketMQClient() {}

    RocketMQClient::~RocketMQClient() {}

    void RocketMQClient::Init(v8::Local<v8::Object> exports)
    {
        v8::Isolate *isolate = exports->GetIsolate();
        v8::Local<v8::Context> context = isolate->GetCurrentContext();

        // 准备构造函数模板
        v8::Local<v8::FunctionTemplate> tpl = v8::FunctionTemplate::New(isolate, New);
        tpl->SetClassName(v8::String::NewFromUtf8(isolate, "RocketMQClient").ToLocalChecked());
        tpl->InstanceTemplate()->SetInternalFieldCount(1);

        // 原型方法
        NODE_SET_PROTOTYPE_METHOD(tpl, "initRocketMQ", InitRocketMQ);
        NODE_SET_PROTOTYPE_METHOD(tpl, "createProducer", CreateProducer);
        NODE_SET_PROTOTYPE_METHOD(tpl, "sendMessage", SendMessage);
        NODE_SET_PROTOTYPE_METHOD(tpl, "sendOrderedMessage", SendOrderedMessage);
        NODE_SET_PROTOTYPE_METHOD(tpl, "createConsumer", CreateConsumer);
        NODE_SET_PROTOTYPE_METHOD(tpl, "startConsumer", StartConsumer);
        NODE_SET_PROTOTYPE_METHOD(tpl, "registerMessageHandler", RegisterMessageHandler);
        NODE_SET_PROTOTYPE_METHOD(tpl, "ackMessage", AckMessage);
        NODE_SET_PROTOTYPE_METHOD(tpl, "shutdownProducer", ShutdownProducer);
        NODE_SET_PROTOTYPE_METHOD(tpl, "shutdownConsumer", ShutdownConsumer);

        v8::Local<v8::Function> constructor_func = tpl->GetFunction(context).ToLocalChecked();
        constructor.Reset(isolate, constructor_func);
        exports->Set(context, v8::String::NewFromUtf8(isolate, "RocketMQClient").ToLocalChecked(), constructor_func).FromJust();
    }

    void RocketMQClient::New(const v8::FunctionCallbackInfo<v8::Value> &args)
    {
        v8::Isolate *isolate = args.GetIsolate();
        v8::Local<v8::Context> context = isolate->GetCurrentContext();

        if (args.IsConstructCall())
        {
            // 加载Go库
            if (!LoadGoLibrary())
            {
                isolate->ThrowException(v8::Exception::Error(
                    v8::String::NewFromUtf8(isolate, "Failed to load RocketMQ Go library").ToLocalChecked()));
                return;
            }

            RocketMQClient *obj = new RocketMQClient();
            obj->Wrap(args.This());
            args.GetReturnValue().Set(args.This());
        }
        else
        {
            const int argc = 1;
            v8::Local<v8::Value> argv[argc] = {args[0]};
            v8::Local<v8::Function> cons = v8::Local<v8::Function>::New(isolate, constructor);
            v8::Local<v8::Object> result = cons->NewInstance(context, argc, argv).ToLocalChecked();
            args.GetReturnValue().Set(result);
        }
    }

    void RocketMQClient::InitRocketMQ(const v8::FunctionCallbackInfo<v8::Value> &args)
    {
        v8::Isolate *isolate = args.GetIsolate();
        v8::Local<v8::Context> context = isolate->GetCurrentContext();

        if (args.Length() < 1)
        {
            isolate->ThrowException(v8::Exception::TypeError(
                v8::String::NewFromUtf8(isolate, "Wrong number of arguments").ToLocalChecked()));
            return;
        }

        RocketMQClient *obj = ObjectWrap::Unwrap<RocketMQClient>(args.Holder());

        std::string configJson = V8ObjectToJsonString(isolate, args[0]->ToObject(context).ToLocalChecked());
        obj->config_json_ = configJson;

        char *result = go_InitRocketMQ(configJson.c_str());
        v8::Local<v8::Object> resultObj = JsonStringToV8Object(isolate, std::string(result));
        go_FreeString(result);

        args.GetReturnValue().Set(resultObj);
    }

    void RocketMQClient::CreateProducer(const v8::FunctionCallbackInfo<v8::Value> &args)
    {
        v8::Isolate *isolate = args.GetIsolate();
        v8::Local<v8::Context> context = isolate->GetCurrentContext();

        if (args.Length() < 2)
        {
            isolate->ThrowException(v8::Exception::TypeError(
                v8::String::NewFromUtf8(isolate, "Wrong number of arguments").ToLocalChecked()));
            return;
        }

        RocketMQClient *obj = ObjectWrap::Unwrap<RocketMQClient>(args.Holder());

        std::string instanceId = V8StringToStdString(isolate, args[0]);
        std::string topic = V8StringToStdString(isolate, args[1]);

        char *result = go_CreateProducer(obj->config_json_.c_str(), topic.c_str());
        v8::Local<v8::Object> resultObj = JsonStringToV8Object(isolate, std::string(result));
        go_FreeString(result);

        // 如果成功，创建Producer对象
        v8::Local<v8::Value> success = resultObj->Get(context, v8::String::NewFromUtf8(isolate, "success").ToLocalChecked()).ToLocalChecked();
        if (success->BooleanValue(isolate))
        {
            v8::Local<v8::Value> producerId = resultObj->Get(context, v8::String::NewFromUtf8(isolate, "producerId").ToLocalChecked()).ToLocalChecked();
            std::string producerIdStr = V8StringToStdString(isolate, producerId);

            obj->producers_[topic] = producerIdStr;

            // 创建Producer实例
            const int argc = 2;
            v8::Local<v8::Value> argv[argc] = {
                v8::String::NewFromUtf8(isolate, producerIdStr.c_str()).ToLocalChecked(),
                v8::String::NewFromUtf8(isolate, topic.c_str()).ToLocalChecked()};
            v8::Local<v8::Function> cons = v8::Local<v8::Function>::New(isolate, Producer::constructor);
            v8::Local<v8::Object> producerObj = cons->NewInstance(context, argc, argv).ToLocalChecked();

            args.GetReturnValue().Set(producerObj);
        }
        else
        {
            args.GetReturnValue().Set(resultObj);
        }
    }

    void RocketMQClient::SendMessage(const v8::FunctionCallbackInfo<v8::Value> &args)
    {
        v8::Isolate *isolate = args.GetIsolate();

        if (args.Length() < 4)
        {
            isolate->ThrowException(v8::Exception::TypeError(
                v8::String::NewFromUtf8(isolate, "Wrong number of arguments").ToLocalChecked()));
            return;
        }

        std::string producerId = V8StringToStdString(isolate, args[0]);
        std::string messageBody = V8StringToStdString(isolate, args[1]);
        std::string tag = V8StringToStdString(isolate, args[2]);
        std::string propertiesJson = V8ObjectToJsonString(isolate, args[3]->ToObject(isolate->GetCurrentContext()).ToLocalChecked());

        char *result = go_SendMessage(producerId.c_str(), messageBody.c_str(), tag.c_str(), propertiesJson.c_str());
        v8::Local<v8::Object> resultObj = JsonStringToV8Object(isolate, std::string(result));
        go_FreeString(result);

        args.GetReturnValue().Set(resultObj);
    }

    void RocketMQClient::SendOrderedMessage(const v8::FunctionCallbackInfo<v8::Value> &args)
    {
        v8::Isolate *isolate = args.GetIsolate();

        if (args.Length() < 5)
        {
            isolate->ThrowException(v8::Exception::TypeError(
                v8::String::NewFromUtf8(isolate, "Wrong number of arguments").ToLocalChecked()));
            return;
        }

        std::string producerId = V8StringToStdString(isolate, args[0]);
        std::string messageBody = V8StringToStdString(isolate, args[1]);
        std::string tag = V8StringToStdString(isolate, args[2]);
        std::string propertiesJson = V8ObjectToJsonString(isolate, args[3]->ToObject(isolate->GetCurrentContext()).ToLocalChecked());
        std::string shardingKey = V8StringToStdString(isolate, args[4]);

        char *result = go_SendOrderedMessage(producerId.c_str(), messageBody.c_str(), tag.c_str(), propertiesJson.c_str(), shardingKey.c_str());
        v8::Local<v8::Object> resultObj = JsonStringToV8Object(isolate, std::string(result));
        go_FreeString(result);

        args.GetReturnValue().Set(resultObj);
    }

    void RocketMQClient::CreateConsumer(const v8::FunctionCallbackInfo<v8::Value> &args)
    {
        v8::Isolate *isolate = args.GetIsolate();
        v8::Local<v8::Context> context = isolate->GetCurrentContext();

        if (args.Length() < 4)
        {
            isolate->ThrowException(v8::Exception::TypeError(
                v8::String::NewFromUtf8(isolate, "Wrong number of arguments").ToLocalChecked()));
            return;
        }

        RocketMQClient *obj = ObjectWrap::Unwrap<RocketMQClient>(args.Holder());

        std::string instanceId = V8StringToStdString(isolate, args[0]);
        std::string topic = V8StringToStdString(isolate, args[1]);
        std::string groupId = V8StringToStdString(isolate, args[2]);
        std::string tagExpression = V8StringToStdString(isolate, args[3]);

        char *result = go_CreateConsumer(obj->config_json_.c_str(), topic.c_str(), groupId.c_str(), tagExpression.c_str());
        v8::Local<v8::Object> resultObj = JsonStringToV8Object(isolate, std::string(result));
        go_FreeString(result);

        // 如果成功，创建Consumer对象
        v8::Local<v8::Value> success = resultObj->Get(context, v8::String::NewFromUtf8(isolate, "success").ToLocalChecked()).ToLocalChecked();
        if (success->BooleanValue(isolate))
        {
            v8::Local<v8::Value> consumerId = resultObj->Get(context, v8::String::NewFromUtf8(isolate, "consumerId").ToLocalChecked()).ToLocalChecked();
            std::string consumerIdStr = V8StringToStdString(isolate, consumerId);

            std::string key = topic + "_" + groupId;
            obj->consumers_[key] = consumerIdStr;

            // 创建Consumer实例
            const int argc = 3;
            v8::Local<v8::Value> argv[argc] = {
                v8::String::NewFromUtf8(isolate, consumerIdStr.c_str()).ToLocalChecked(),
                v8::String::NewFromUtf8(isolate, topic.c_str()).ToLocalChecked(),
                v8::String::NewFromUtf8(isolate, groupId.c_str()).ToLocalChecked()};
            v8::Local<v8::Function> cons = v8::Local<v8::Function>::New(isolate, Consumer::constructor);
            v8::Local<v8::Object> consumerObj = cons->NewInstance(context, argc, argv).ToLocalChecked();

            args.GetReturnValue().Set(consumerObj);
        }
        else
        {
            args.GetReturnValue().Set(resultObj);
        }
    }

    void RocketMQClient::StartConsumer(const v8::FunctionCallbackInfo<v8::Value> &args)
    {
        v8::Isolate *isolate = args.GetIsolate();

        if (args.Length() < 3)
        {
            isolate->ThrowException(v8::Exception::TypeError(
                v8::String::NewFromUtf8(isolate, "Wrong number of arguments").ToLocalChecked()));
            return;
        }

        std::string consumerId = V8StringToStdString(isolate, args[0]);
        std::string topic = V8StringToStdString(isolate, args[1]);
        std::string tagExpression = V8StringToStdString(isolate, args[2]);

        char *result = go_StartConsumer(consumerId.c_str(), topic.c_str(), tagExpression.c_str());
        v8::Local<v8::Object> resultObj = JsonStringToV8Object(isolate, std::string(result));
        go_FreeString(result);

        args.GetReturnValue().Set(resultObj);
    }

    void RocketMQClient::RegisterMessageHandler(const v8::FunctionCallbackInfo<v8::Value> &args)
    {
        v8::Isolate *isolate = args.GetIsolate();

        if (args.Length() < 2)
        {
            isolate->ThrowException(v8::Exception::TypeError(
                v8::String::NewFromUtf8(isolate, "Wrong number of arguments").ToLocalChecked()));
            return;
        }

        RocketMQClient *obj = ObjectWrap::Unwrap<RocketMQClient>(args.Holder());

        std::string consumerId = V8StringToStdString(isolate, args[0]);
        v8::Local<v8::Function> callback = v8::Local<v8::Function>::Cast(args[1]);

        // 创建消息处理器包装
        auto handler = std::make_shared<MessageHandlerWrapper>(isolate, callback);
        obj->message_handlers_[consumerId] = handler;
        MessageHandlerWrapper::handlers_[consumerId] = handler;
        MessageHandlerWrapper::current_consumer_id_ = consumerId;

        char *result = go_RegisterMessageHandler(consumerId.c_str(), MessageHandlerWrapper::StaticHandleMessage);
        v8::Local<v8::Object> resultObj = JsonStringToV8Object(isolate, std::string(result));
        go_FreeString(result);

        args.GetReturnValue().Set(resultObj);
    }

    void RocketMQClient::AckMessage(const v8::FunctionCallbackInfo<v8::Value> &args)
    {
        v8::Isolate *isolate = args.GetIsolate();

        if (args.Length() < 2)
        {
            isolate->ThrowException(v8::Exception::TypeError(
                v8::String::NewFromUtf8(isolate, "Wrong number of arguments").ToLocalChecked()));
            return;
        }

        std::string consumerId = V8StringToStdString(isolate, args[0]);
        std::string receiptHandle = V8StringToStdString(isolate, args[1]);

        char *result = go_AckMessage(consumerId.c_str(), receiptHandle.c_str());
        v8::Local<v8::Object> resultObj = JsonStringToV8Object(isolate, std::string(result));
        go_FreeString(result);

        args.GetReturnValue().Set(resultObj);
    }

    void RocketMQClient::ShutdownProducer(const v8::FunctionCallbackInfo<v8::Value> &args)
    {
        v8::Isolate *isolate = args.GetIsolate();

        if (args.Length() < 1)
        {
            isolate->ThrowException(v8::Exception::TypeError(
                v8::String::NewFromUtf8(isolate, "Wrong number of arguments").ToLocalChecked()));
            return;
        }

        std::string producerId = V8StringToStdString(isolate, args[0]);

        char *result = go_ShutdownProducer(producerId.c_str());
        v8::Local<v8::Object> resultObj = JsonStringToV8Object(isolate, std::string(result));
        go_FreeString(result);

        args.GetReturnValue().Set(resultObj);
    }

    void RocketMQClient::ShutdownConsumer(const v8::FunctionCallbackInfo<v8::Value> &args)
    {
        v8::Isolate *isolate = args.GetIsolate();

        if (args.Length() < 1)
        {
            isolate->ThrowException(v8::Exception::TypeError(
                v8::String::NewFromUtf8(isolate, "Wrong number of arguments").ToLocalChecked()));
            return;
        }

        RocketMQClient *obj = ObjectWrap::Unwrap<RocketMQClient>(args.Holder());
        std::string consumerId = V8StringToStdString(isolate, args[0]);

        char *result = go_ShutdownConsumer(consumerId.c_str());
        v8::Local<v8::Object> resultObj = JsonStringToV8Object(isolate, std::string(result));
        go_FreeString(result);

        // 清理消息处理器
        obj->message_handlers_.erase(consumerId);
        MessageHandlerWrapper::handlers_.erase(consumerId);

        args.GetReturnValue().Set(resultObj);
    }

    // Producer 实现
    Producer::Producer(const std::string &producer_id, const std::string &topic)
        : producer_id_(producer_id), topic_(topic) {}

    Producer::~Producer() {}

    void Producer::Init(v8::Local<v8::Object> exports)
    {
        v8::Isolate *isolate = exports->GetIsolate();
        v8::Local<v8::Context> context = isolate->GetCurrentContext();

        v8::Local<v8::FunctionTemplate> tpl = v8::FunctionTemplate::New(isolate, New);
        tpl->SetClassName(v8::String::NewFromUtf8(isolate, "Producer").ToLocalChecked());
        tpl->InstanceTemplate()->SetInternalFieldCount(1);

        NODE_SET_PROTOTYPE_METHOD(tpl, "publishMessage", PublishMessage);
        NODE_SET_PROTOTYPE_METHOD(tpl, "publishOrderedMessage", PublishOrderedMessage);
        NODE_SET_PROTOTYPE_METHOD(tpl, "publishDelayMessage", PublishDelayMessage);
        NODE_SET_PROTOTYPE_METHOD(tpl, "shutdown", Shutdown);

        v8::Local<v8::Function> constructor_func = tpl->GetFunction(context).ToLocalChecked();
        constructor.Reset(isolate, constructor_func);
        exports->Set(context, v8::String::NewFromUtf8(isolate, "Producer").ToLocalChecked(), constructor_func).FromJust();
    }

    void Producer::New(const v8::FunctionCallbackInfo<v8::Value> &args)
    {
        v8::Isolate *isolate = args.GetIsolate();
        v8::Local<v8::Context> context = isolate->GetCurrentContext();

        if (args.IsConstructCall())
        {
            std::string producer_id = V8StringToStdString(isolate, args[0]);
            std::string topic = V8StringToStdString(isolate, args[1]);

            Producer *obj = new Producer(producer_id, topic);
            obj->Wrap(args.This());
            args.GetReturnValue().Set(args.This());
        }
        else
        {
            const int argc = 2;
            v8::Local<v8::Value> argv[argc] = {args[0], args[1]};
            v8::Local<v8::Function> cons = v8::Local<v8::Function>::New(isolate, constructor);
            v8::Local<v8::Object> result = cons->NewInstance(context, argc, argv).ToLocalChecked();
            args.GetReturnValue().Set(result);
        }
    }

    void Producer::PublishMessage(const v8::FunctionCallbackInfo<v8::Value> &args)
    {
        v8::Isolate *isolate = args.GetIsolate();

        if (args.Length() < 3)
        {
            isolate->ThrowException(v8::Exception::TypeError(
                v8::String::NewFromUtf8(isolate, "Wrong number of arguments").ToLocalChecked()));
            return;
        }

        Producer *obj = ObjectWrap::Unwrap<Producer>(args.Holder());

        std::string messageBody = V8StringToStdString(isolate, args[0]);
        std::string tag = V8StringToStdString(isolate, args[1]);
        std::string propertiesJson = "{}";

        if (args.Length() > 2 && !args[2]->IsNull() && !args[2]->IsUndefined())
        {
            propertiesJson = V8ObjectToJsonString(isolate, args[2]->ToObject(isolate->GetCurrentContext()).ToLocalChecked());
        }

        char *result = go_SendMessage(obj->producer_id_.c_str(), messageBody.c_str(), tag.c_str(), propertiesJson.c_str());
        v8::Local<v8::Object> resultObj = JsonStringToV8Object(isolate, std::string(result));
        go_FreeString(result);

        args.GetReturnValue().Set(resultObj);
    }

    void Producer::PublishOrderedMessage(const v8::FunctionCallbackInfo<v8::Value> &args)
    {
        v8::Isolate *isolate = args.GetIsolate();

        if (args.Length() < 4)
        {
            isolate->ThrowException(v8::Exception::TypeError(
                v8::String::NewFromUtf8(isolate, "Wrong number of arguments").ToLocalChecked()));
            return;
        }

        Producer *obj = ObjectWrap::Unwrap<Producer>(args.Holder());

        std::string messageBody = V8StringToStdString(isolate, args[0]);
        std::string tag = V8StringToStdString(isolate, args[1]);
        std::string propertiesJson = "{}";
        std::string shardingKey = V8StringToStdString(isolate, args[3]);

        if (args.Length() > 2 && !args[2]->IsNull() && !args[2]->IsUndefined())
        {
            propertiesJson = V8ObjectToJsonString(isolate, args[2]->ToObject(isolate->GetCurrentContext()).ToLocalChecked());
        }

        char *result = go_SendOrderedMessage(obj->producer_id_.c_str(), messageBody.c_str(), tag.c_str(), propertiesJson.c_str(), shardingKey.c_str());
        v8::Local<v8::Object> resultObj = JsonStringToV8Object(isolate, std::string(result));
        go_FreeString(result);

        args.GetReturnValue().Set(resultObj);
    }

    void Producer::PublishDelayMessage(const v8::FunctionCallbackInfo<v8::Value> &args)
    {
        // 延迟消息可以通过在properties中设置startDeliverTime来实现
        PublishMessage(args);
    }

    void Producer::Shutdown(const v8::FunctionCallbackInfo<v8::Value> &args)
    {
        v8::Isolate *isolate = args.GetIsolate();
        Producer *obj = ObjectWrap::Unwrap<Producer>(args.Holder());

        char *result = go_ShutdownProducer(obj->producer_id_.c_str());
        v8::Local<v8::Object> resultObj = JsonStringToV8Object(isolate, std::string(result));
        go_FreeString(result);

        args.GetReturnValue().Set(resultObj);
    }

    // Consumer 实现
    Consumer::Consumer(const std::string &consumer_id, const std::string &topic, const std::string &group_id)
        : consumer_id_(consumer_id), topic_(topic), group_id_(group_id) {}

    Consumer::~Consumer() {}

    void Consumer::Init(v8::Local<v8::Object> exports)
    {
        v8::Isolate *isolate = exports->GetIsolate();
        v8::Local<v8::Context> context = isolate->GetCurrentContext();

        v8::Local<v8::FunctionTemplate> tpl = v8::FunctionTemplate::New(isolate, New);
        tpl->SetClassName(v8::String::NewFromUtf8(isolate, "Consumer").ToLocalChecked());
        tpl->InstanceTemplate()->SetInternalFieldCount(1);

        NODE_SET_PROTOTYPE_METHOD(tpl, "onMessage", OnMessage);
        NODE_SET_PROTOTYPE_METHOD(tpl, "startReceiving", StartReceiving);
        NODE_SET_PROTOTYPE_METHOD(tpl, "ackMessage", AckMessage);
        NODE_SET_PROTOTYPE_METHOD(tpl, "shutdown", Shutdown);

        v8::Local<v8::Function> constructor_func = tpl->GetFunction(context).ToLocalChecked();
        constructor.Reset(isolate, constructor_func);
        exports->Set(context, v8::String::NewFromUtf8(isolate, "Consumer").ToLocalChecked(), constructor_func).FromJust();
    }

    void Consumer::New(const v8::FunctionCallbackInfo<v8::Value> &args)
    {
        v8::Isolate *isolate = args.GetIsolate();
        v8::Local<v8::Context> context = isolate->GetCurrentContext();

        if (args.IsConstructCall())
        {
            std::string consumer_id = V8StringToStdString(isolate, args[0]);
            std::string topic = V8StringToStdString(isolate, args[1]);
            std::string group_id = V8StringToStdString(isolate, args[2]);

            Consumer *obj = new Consumer(consumer_id, topic, group_id);
            obj->Wrap(args.This());
            args.GetReturnValue().Set(args.This());
        }
        else
        {
            const int argc = 3;
            v8::Local<v8::Value> argv[argc] = {args[0], args[1], args[2]};
            v8::Local<v8::Function> cons = v8::Local<v8::Function>::New(isolate, constructor);
            v8::Local<v8::Object> result = cons->NewInstance(context, argc, argv).ToLocalChecked();
            args.GetReturnValue().Set(result);
        }
    }

    void Consumer::OnMessage(const v8::FunctionCallbackInfo<v8::Value> &args)
    {
        v8::Isolate *isolate = args.GetIsolate();

        if (args.Length() < 1)
        {
            isolate->ThrowException(v8::Exception::TypeError(
                v8::String::NewFromUtf8(isolate, "Wrong number of arguments").ToLocalChecked()));
            return;
        }

        Consumer *obj = ObjectWrap::Unwrap<Consumer>(args.Holder());
        v8::Local<v8::Function> callback = v8::Local<v8::Function>::Cast(args[0]);

        // 创建消息处理器包装
        auto handler = std::make_shared<MessageHandlerWrapper>(isolate, callback);
        obj->message_handler_ = handler;
        MessageHandlerWrapper::handlers_[obj->consumer_id_] = handler;
        MessageHandlerWrapper::current_consumer_id_ = obj->consumer_id_;

        char *result = go_RegisterMessageHandler(obj->consumer_id_.c_str(), MessageHandlerWrapper::StaticHandleMessage);
        v8::Local<v8::Object> resultObj = JsonStringToV8Object(isolate, std::string(result));
        go_FreeString(result);

        args.GetReturnValue().Set(resultObj);
    }

    void Consumer::StartReceiving(const v8::FunctionCallbackInfo<v8::Value> &args)
    {
        v8::Isolate *isolate = args.GetIsolate();
        Consumer *obj = ObjectWrap::Unwrap<Consumer>(args.Holder());

        std::string tagExpression = "*";
        if (args.Length() > 0)
        {
            tagExpression = V8StringToStdString(isolate, args[0]);
        }

        char *result = go_StartConsumer(obj->consumer_id_.c_str(), obj->topic_.c_str(), tagExpression.c_str());
        v8::Local<v8::Object> resultObj = JsonStringToV8Object(isolate, std::string(result));
        go_FreeString(result);

        args.GetReturnValue().Set(resultObj);
    }

    void Consumer::AckMessage(const v8::FunctionCallbackInfo<v8::Value> &args)
    {
        v8::Isolate *isolate = args.GetIsolate();

        if (args.Length() < 1)
        {
            isolate->ThrowException(v8::Exception::TypeError(
                v8::String::NewFromUtf8(isolate, "Wrong number of arguments").ToLocalChecked()));
            return;
        }

        Consumer *obj = ObjectWrap::Unwrap<Consumer>(args.Holder());
        std::string receiptHandle = V8StringToStdString(isolate, args[0]);

        char *result = go_AckMessage(obj->consumer_id_.c_str(), receiptHandle.c_str());
        v8::Local<v8::Object> resultObj = JsonStringToV8Object(isolate, std::string(result));
        go_FreeString(result);

        args.GetReturnValue().Set(resultObj);
    }

    void Consumer::Shutdown(const v8::FunctionCallbackInfo<v8::Value> &args)
    {
        v8::Isolate *isolate = args.GetIsolate();
        Consumer *obj = ObjectWrap::Unwrap<Consumer>(args.Holder());

        char *result = go_ShutdownConsumer(obj->consumer_id_.c_str());
        v8::Local<v8::Object> resultObj = JsonStringToV8Object(isolate, std::string(result));
        go_FreeString(result);

        // 清理消息处理器
        MessageHandlerWrapper::handlers_.erase(obj->consumer_id_);
        obj->message_handler_.reset();

        args.GetReturnValue().Set(resultObj);
    }

    // 工具函数实现
    std::string V8StringToStdString(v8::Isolate *isolate, v8::Local<v8::Value> value)
    {
        v8::String::Utf8Value utf8_value(isolate, value);
        return std::string(*utf8_value, utf8_value.length());
    }

    v8::Local<v8::String> StdStringToV8String(v8::Isolate *isolate, const std::string &str)
    {
        return v8::String::NewFromUtf8(isolate, str.c_str()).ToLocalChecked();
    }

    v8::Local<v8::Object> JsonStringToV8Object(v8::Isolate *isolate, const std::string &json_str)
    {
        v8::Local<v8::Context> context = isolate->GetCurrentContext();
        v8::Local<v8::String> json_string = StdStringToV8String(isolate, json_str);

        v8::MaybeLocal<v8::Value> maybe_value = v8::JSON::Parse(context, json_string);
        if (maybe_value.IsEmpty())
        {
            return v8::Object::New(isolate);
        }

        v8::Local<v8::Value> value = maybe_value.ToLocalChecked();
        if (value->IsObject())
        {
            return value->ToObject(context).ToLocalChecked();
        }

        return v8::Object::New(isolate);
    }

    std::string V8ObjectToJsonString(v8::Isolate *isolate, v8::Local<v8::Object> obj)
    {
        v8::Local<v8::Context> context = isolate->GetCurrentContext();

        v8::MaybeLocal<v8::String> maybe_json = v8::JSON::Stringify(context, obj);
        if (maybe_json.IsEmpty())
        {
            return "{}";
        }

        v8::Local<v8::String> json_string = maybe_json.ToLocalChecked();
        return V8StringToStdString(isolate, json_string);
    }

    // 模块初始化
    void InitModule(v8::Local<v8::Object> exports)
    {
        RocketMQClient::Init(exports);
        Producer::Init(exports);
        Consumer::Init(exports);
    }

    NODE_MODULE(NODE_GYP_MODULE_NAME, InitModule)

} // namespace rocketmq_addon