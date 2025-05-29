// Windows API 冲突保护 - 必须最先处理
#ifdef _WIN32
#define WIN32_LEAN_AND_MEAN
#ifndef NOMINMAX
#define NOMINMAX
#endif
// 在包含任何头文件之前禁用Windows宏
#ifdef SendMessage
#undef SendMessage
#endif
#endif

#include "rocketmq_addon.h"
#include <iostream>
#include <sstream>

// 在所有其他包含之后再次禁用可能的宏冲突
#ifdef _WIN32
#ifdef SendMessage
#undef SendMessage
#endif
#endif

namespace rocketmq_addon
{

    // 静态成员初始化
    std::map<std::string, std::shared_ptr<MessageHandlerWrapper>> MessageHandlerWrapper::handlers_;
    std::string MessageHandlerWrapper::current_consumer_id_;

    Napi::FunctionReference RocketMQClient::constructor;
    Napi::FunctionReference Producer::constructor;
    Napi::FunctionReference Consumer::constructor;

    // 动态库句柄
#ifdef _WIN32
    static HMODULE go_lib_handle = nullptr;
#else
    static void *go_lib_handle = nullptr;
#endif

    // 动态加载Go库的函数指针
    static char *(*go_InitRocketMQ)(const char *) = nullptr;
    static char *(*go_CreateProducer)(const char *, const char *) = nullptr;
    static char *(*go_SendRocketMQMessage)(const char *, const char *, const char *, const char *) = nullptr;
    static char *(*go_SendOrderedMessage)(const char *, const char *, const char *, const char *, const char *) = nullptr;
    static char *(*go_CreateConsumer)(const char *, const char *, const char *, const char *) = nullptr;
    static char *(*go_StartConsumer)(const char *, const char *, const char *) = nullptr;
    static char *(*go_RegisterMessageHandler)(const char *, MessageHandler) = nullptr;
    static char *(*go_AckMessage)(const char *, const char *) = nullptr;
    static char *(*go_ShutdownProducer)(const char *) = nullptr;
    static char *(*go_ShutdownConsumer)(const char *) = nullptr;
    static void (*go_FreeString)(char *) = nullptr;

    // 获取addon模块所在目录的函数
    std::string GetAddonDirectory()
    {
#ifdef _WIN32
        HMODULE hModule = NULL;
        if (GetModuleHandleEx(GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS |
                                  GET_MODULE_HANDLE_EX_FLAG_UNCHANGED_REFCOUNT,
                              (LPCTSTR)GetAddonDirectory, &hModule))
        {
            char path[MAX_PATH];
            if (GetModuleFileName(hModule, path, MAX_PATH))
            {
                std::string fullPath(path);
                size_t lastSlash = fullPath.find_last_of('\\');
                if (lastSlash != std::string::npos)
                {
                    return fullPath.substr(0, lastSlash);
                }
            }
        }
        return ".";
#else
        Dl_info info;
        if (dladdr((void *)GetAddonDirectory, &info) && info.dli_fname)
        {
            std::string fullPath(info.dli_fname);
            size_t lastSlash = fullPath.find_last_of('/');
            if (lastSlash != std::string::npos)
            {
                return fullPath.substr(0, lastSlash);
            }
        }
        return ".";
#endif
    }

    // 加载Go动态库
    bool LoadGoLibrary()
    {
        if (go_lib_handle != nullptr)
        {
            return true; // 已经加载
        }

        // 获取addon所在目录
        std::string addonDir = GetAddonDirectory();

#ifdef _WIN32
        // Windows路径 - 基于addon位置动态构建路径
        std::vector<std::string> lib_paths = {
            addonDir + "\\librocketmq_cgo.dll",                           // 同目录 (npm包)
            addonDir + "\\..\\prebuilds\\win32-x64\\librocketmq_cgo.dll", // npm包结构
            ".\\librocketmq_cgo.dll",                                     // 当前工作目录
            "..\\cgo\\librocketmq_cgo.dll",                               // 开发环境
            "librocketmq_cgo.dll"                                         // 系统PATH
        };

        for (const auto &lib_path : lib_paths)
        {
            go_lib_handle = LoadLibraryA(lib_path.c_str());
            if (go_lib_handle != nullptr)
            {
                std::cout << "✅ Loaded Go library: " << lib_path << std::endl;
                break;
            }
        }

        if (go_lib_handle == nullptr)
        {
            DWORD error = GetLastError();
            std::cerr << "Failed to load Go library. Error code: " << error << std::endl;
            return false;
        }

        // 加载函数符号 (Windows)
        go_InitRocketMQ = (char *(*)(const char *))GetProcAddress(go_lib_handle, "InitRocketMQ");
        go_CreateProducer = (char *(*)(const char *, const char *))GetProcAddress(go_lib_handle, "CreateProducer");
        go_SendRocketMQMessage = (char *(*)(const char *, const char *, const char *, const char *))GetProcAddress(go_lib_handle, "SendMessage");
        go_SendOrderedMessage = (char *(*)(const char *, const char *, const char *, const char *, const char *))GetProcAddress(go_lib_handle, "SendOrderedMessage");
        go_CreateConsumer = (char *(*)(const char *, const char *, const char *, const char *))GetProcAddress(go_lib_handle, "CreateConsumer");
        go_StartConsumer = (char *(*)(const char *, const char *, const char *))GetProcAddress(go_lib_handle, "StartConsumer");
        go_RegisterMessageHandler = (char *(*)(const char *, MessageHandler))GetProcAddress(go_lib_handle, "RegisterMessageHandler");
        go_AckMessage = (char *(*)(const char *, const char *))GetProcAddress(go_lib_handle, "AckMessage");
        go_ShutdownProducer = (char *(*)(const char *))GetProcAddress(go_lib_handle, "ShutdownProducer");
        go_ShutdownConsumer = (char *(*)(const char *))GetProcAddress(go_lib_handle, "ShutdownConsumer");
        go_FreeString = (void (*)(char *))GetProcAddress(go_lib_handle, "FreeString");

        if (!go_InitRocketMQ || !go_CreateProducer || !go_SendRocketMQMessage ||
            !go_SendOrderedMessage || !go_CreateConsumer || !go_StartConsumer ||
            !go_RegisterMessageHandler || !go_AckMessage || !go_ShutdownProducer ||
            !go_ShutdownConsumer || !go_FreeString)
        {
            std::cerr << "Failed to load Go library functions" << std::endl;
            FreeLibrary(go_lib_handle);
            go_lib_handle = nullptr;
            return false;
        }
#else
        // Unix路径 - 基于addon位置动态构建路径
        std::vector<std::string> lib_paths;

#ifdef __APPLE__
        // macOS动态库路径
        lib_paths = {
            addonDir + "/librocketmq_cgo.dylib",                           // 同目录 (npm包)
            addonDir + "/../prebuilds/darwin-arm64/librocketmq_cgo.dylib", // npm包结构 (ARM64)
            addonDir + "/../prebuilds/darwin-x64/librocketmq_cgo.dylib",   // npm包结构 (x64)
            "./librocketmq_cgo.dylib",                                     // 当前工作目录
            "./prebuilds/darwin-arm64/librocketmq_cgo.dylib",              // 开发环境
            "./prebuilds/darwin-x64/librocketmq_cgo.dylib",                // 开发环境
            "../prebuilds/darwin-arm64/librocketmq_cgo.dylib",             // 开发环境
            "../prebuilds/darwin-x64/librocketmq_cgo.dylib",               // 开发环境
            "../cgo/librocketmq_cgo.dylib",                                // 开发目录
            "/usr/local/lib/librocketmq_cgo.dylib"                         // 系统安装
        };
#else
        // Linux动态库路径
        lib_paths = {
            addonDir + "/librocketmq_cgo.so",                        // 同目录 (npm包)
            addonDir + "/../prebuilds/linux-x64/librocketmq_cgo.so", // npm包结构
            "./librocketmq_cgo.so",                                  // 当前工作目录
            "./prebuilds/linux-x64/librocketmq_cgo.so",              // 开发环境
            "../prebuilds/linux-x64/librocketmq_cgo.so",             // 开发环境
            "../cgo/librocketmq_cgo.so",                             // 开发目录
            "/usr/local/lib/librocketmq_cgo.so"                      // 系统安装
        };
#endif

        for (const auto &lib_path : lib_paths)
        {
            go_lib_handle = dlopen(lib_path.c_str(), RTLD_LAZY);
            if (go_lib_handle != nullptr)
            {
                std::cout << "✅ Loaded Go library: " << lib_path << std::endl;
                break;
            }
        }

        if (go_lib_handle == nullptr)
        {
            std::cerr << "Failed to load Go library: " << dlerror() << std::endl;
            return false;
        }

        // 加载函数符号 (Unix)
        go_InitRocketMQ = (char *(*)(const char *))dlsym(go_lib_handle, "InitRocketMQ");
        go_CreateProducer = (char *(*)(const char *, const char *))dlsym(go_lib_handle, "CreateProducer");
        go_SendRocketMQMessage = (char *(*)(const char *, const char *, const char *, const char *))dlsym(go_lib_handle, "SendMessage");
        go_SendOrderedMessage = (char *(*)(const char *, const char *, const char *, const char *, const char *))dlsym(go_lib_handle, "SendOrderedMessage");
        go_CreateConsumer = (char *(*)(const char *, const char *, const char *, const char *))dlsym(go_lib_handle, "CreateConsumer");
        go_StartConsumer = (char *(*)(const char *, const char *, const char *))dlsym(go_lib_handle, "StartConsumer");
        go_RegisterMessageHandler = (char *(*)(const char *, MessageHandler))dlsym(go_lib_handle, "RegisterMessageHandler");
        go_AckMessage = (char *(*)(const char *, const char *))dlsym(go_lib_handle, "AckMessage");
        go_ShutdownProducer = (char *(*)(const char *))dlsym(go_lib_handle, "ShutdownProducer");
        go_ShutdownConsumer = (char *(*)(const char *))dlsym(go_lib_handle, "ShutdownConsumer");
        go_FreeString = (void (*)(char *))dlsym(go_lib_handle, "FreeString");

        if (!go_InitRocketMQ || !go_CreateProducer || !go_SendRocketMQMessage ||
            !go_SendOrderedMessage || !go_CreateConsumer || !go_StartConsumer ||
            !go_RegisterMessageHandler || !go_AckMessage || !go_ShutdownProducer ||
            !go_ShutdownConsumer || !go_FreeString)
        {
            std::cerr << "Failed to load Go library functions" << std::endl;
            dlclose(go_lib_handle);
            go_lib_handle = nullptr;
            return false;
        }
#endif

        return true;
    }

    // MessageHandlerWrapper 实现 - 使用Node-API
    MessageHandlerWrapper::MessageHandlerWrapper(Napi::Env env, Napi::Function callback)
        : env_(env), callback_(Napi::Persistent(callback))
    {
    }

    MessageHandlerWrapper::~MessageHandlerWrapper()
    {
    }

    void MessageHandlerWrapper::HandleMessage(const char *messageJson)
    {
        try
        {
            // 解析JSON消息
            Napi::Object messageObj = JsonStringToNapiObject(env_, std::string(messageJson));

            // 调用JavaScript回调
            callback_.Call({messageObj});
        }
        catch (const std::exception &e)
        {
            std::cerr << "Error in message handler: " << e.what() << std::endl;
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

    // RocketMQClient 实现 - 使用Node-API
    RocketMQClient::RocketMQClient(const Napi::CallbackInfo &info) : Napi::ObjectWrap<RocketMQClient>(info)
    {
        Napi::Env env = info.Env();

        if (info.Length() > 0 && info[0].IsString())
        {
            config_json_ = info[0].As<Napi::String>().Utf8Value();
        }

        // 加载Go库
        if (!LoadGoLibrary())
        {
            Napi::Error::New(env, "Failed to load Go library").ThrowAsJavaScriptException();
            return;
        }
    }

    RocketMQClient::~RocketMQClient()
    {
    }

    Napi::Object RocketMQClient::Init(Napi::Env env, Napi::Object exports)
    {
        Napi::Function func = DefineClass(env, "RocketMQClient", {
                                                                     InstanceMethod("initRocketMQ", &RocketMQClient::InitRocketMQ),
                                                                     InstanceMethod("createProducer", &RocketMQClient::CreateProducer),
                                                                     InstanceMethod("sendMessage", &RocketMQClient::SendMessage),
                                                                     InstanceMethod("sendOrderedMessage", &RocketMQClient::SendOrderedMessage),
                                                                     InstanceMethod("createConsumer", &RocketMQClient::CreateConsumer),
                                                                     InstanceMethod("startConsumer", &RocketMQClient::StartConsumer),
                                                                     InstanceMethod("registerMessageHandler", &RocketMQClient::RegisterMessageHandler),
                                                                     InstanceMethod("ackMessage", &RocketMQClient::AckMessage),
                                                                     InstanceMethod("shutdownProducer", &RocketMQClient::ShutdownProducer),
                                                                     InstanceMethod("shutdownConsumer", &RocketMQClient::ShutdownConsumer),
                                                                 });

        constructor = Napi::Persistent(func);
        constructor.SuppressDestruct();

        exports.Set("RocketMQClient", func);
        return exports;
    }

    Napi::Value RocketMQClient::NewInstance(const Napi::CallbackInfo &info)
    {
        return constructor.New({});
    }

    Napi::Value RocketMQClient::InitRocketMQ(const Napi::CallbackInfo &info)
    {
        Napi::Env env = info.Env();

        if (info.Length() > 0 && info[0].IsString())
        {
            std::string config = info[0].As<Napi::String>().Utf8Value();
            char *result = go_InitRocketMQ(config.c_str());

            if (result)
            {
                std::string resultStr(result);
                go_FreeString(result);
                return Napi::String::New(env, resultStr);
            }
        }

        return env.Null();
    }

    Napi::Value RocketMQClient::CreateProducer(const Napi::CallbackInfo &info)
    {
        Napi::Env env = info.Env();

        if (info.Length() >= 2 && info[0].IsString() && info[1].IsString())
        {
            std::string config = info[0].As<Napi::String>().Utf8Value();
            std::string topic = info[1].As<Napi::String>().Utf8Value();

            char *result = go_CreateProducer(config.c_str(), topic.c_str());

            if (result)
            {
                std::string resultStr(result);
                go_FreeString(result);
                return Napi::String::New(env, resultStr);
            }
        }

        return env.Null();
    }

    Napi::Value RocketMQClient::SendMessage(const Napi::CallbackInfo &info)
    {
        Napi::Env env = info.Env();

        if (info.Length() >= 4)
        {
            std::string producerId = info[0].As<Napi::String>().Utf8Value();
            std::string messageBody = info[1].As<Napi::String>().Utf8Value();
            std::string tag = info[2].As<Napi::String>().Utf8Value();
            std::string properties = info[3].As<Napi::String>().Utf8Value();

            char *result = go_SendRocketMQMessage(producerId.c_str(), messageBody.c_str(), tag.c_str(), properties.c_str());

            if (result)
            {
                std::string resultStr(result);
                go_FreeString(result);
                return Napi::String::New(env, resultStr);
            }
        }

        return env.Null();
    }

    Napi::Value RocketMQClient::SendOrderedMessage(const Napi::CallbackInfo &info)
    {
        Napi::Env env = info.Env();

        if (info.Length() >= 5)
        {
            std::string producerId = info[0].As<Napi::String>().Utf8Value();
            std::string messageBody = info[1].As<Napi::String>().Utf8Value();
            std::string tag = info[2].As<Napi::String>().Utf8Value();
            std::string properties = info[3].As<Napi::String>().Utf8Value();
            std::string shardingKey = info[4].As<Napi::String>().Utf8Value();

            char *result = go_SendOrderedMessage(producerId.c_str(), messageBody.c_str(), tag.c_str(), properties.c_str(), shardingKey.c_str());

            if (result)
            {
                std::string resultStr(result);
                go_FreeString(result);
                return Napi::String::New(env, resultStr);
            }
        }

        return env.Null();
    }

    Napi::Value RocketMQClient::CreateConsumer(const Napi::CallbackInfo &info)
    {
        Napi::Env env = info.Env();

        if (info.Length() >= 4)
        {
            std::string config = info[0].As<Napi::String>().Utf8Value();
            std::string topic = info[1].As<Napi::String>().Utf8Value();
            std::string groupId = info[2].As<Napi::String>().Utf8Value();
            std::string tagExpression = info[3].As<Napi::String>().Utf8Value();

            char *result = go_CreateConsumer(config.c_str(), topic.c_str(), groupId.c_str(), tagExpression.c_str());

            if (result)
            {
                std::string resultStr(result);
                go_FreeString(result);
                return Napi::String::New(env, resultStr);
            }
        }

        return env.Null();
    }

    Napi::Value RocketMQClient::StartConsumer(const Napi::CallbackInfo &info)
    {
        Napi::Env env = info.Env();

        if (info.Length() >= 3)
        {
            std::string consumerId = info[0].As<Napi::String>().Utf8Value();
            std::string topic = info[1].As<Napi::String>().Utf8Value();
            std::string tagExpression = info[2].As<Napi::String>().Utf8Value();

            char *result = go_StartConsumer(consumerId.c_str(), topic.c_str(), tagExpression.c_str());

            if (result)
            {
                std::string resultStr(result);
                go_FreeString(result);
                return Napi::String::New(env, resultStr);
            }
        }

        return env.Null();
    }

    Napi::Value RocketMQClient::RegisterMessageHandler(const Napi::CallbackInfo &info)
    {
        Napi::Env env = info.Env();

        if (info.Length() >= 2 && info[0].IsString() && info[1].IsFunction())
        {
            std::string consumerId = info[0].As<Napi::String>().Utf8Value();
            Napi::Function callback = info[1].As<Napi::Function>();

            // 创建消息处理器
            auto handler = std::make_shared<MessageHandlerWrapper>(env, callback);
            message_handlers_[consumerId] = handler;
            MessageHandlerWrapper::handlers_[consumerId] = handler;

            // 设置当前消费者ID
            MessageHandlerWrapper::current_consumer_id_ = consumerId;

            char *result = go_RegisterMessageHandler(consumerId.c_str(), MessageHandlerWrapper::StaticHandleMessage);

            if (result)
            {
                std::string resultStr(result);
                go_FreeString(result);
                return Napi::String::New(env, resultStr);
            }
        }

        return env.Null();
    }

    Napi::Value RocketMQClient::AckMessage(const Napi::CallbackInfo &info)
    {
        Napi::Env env = info.Env();

        if (info.Length() >= 2)
        {
            std::string consumerId = info[0].As<Napi::String>().Utf8Value();
            std::string receiptHandle = info[1].As<Napi::String>().Utf8Value();

            char *result = go_AckMessage(consumerId.c_str(), receiptHandle.c_str());

            if (result)
            {
                std::string resultStr(result);
                go_FreeString(result);
                return Napi::String::New(env, resultStr);
            }
        }

        return env.Null();
    }

    Napi::Value RocketMQClient::ShutdownProducer(const Napi::CallbackInfo &info)
    {
        Napi::Env env = info.Env();

        if (info.Length() >= 1)
        {
            std::string producerId = info[0].As<Napi::String>().Utf8Value();

            char *result = go_ShutdownProducer(producerId.c_str());

            if (result)
            {
                std::string resultStr(result);
                go_FreeString(result);
                return Napi::String::New(env, resultStr);
            }
        }

        return env.Null();
    }

    Napi::Value RocketMQClient::ShutdownConsumer(const Napi::CallbackInfo &info)
    {
        Napi::Env env = info.Env();

        if (info.Length() >= 1)
        {
            std::string consumerId = info[0].As<Napi::String>().Utf8Value();

            char *result = go_ShutdownConsumer(consumerId.c_str());

            if (result)
            {
                std::string resultStr(result);
                go_FreeString(result);
                return Napi::String::New(env, resultStr);
            }
        }

        return env.Null();
    }

    // Producer 实现 - 使用Node-API
    Producer::Producer(const Napi::CallbackInfo &info) : Napi::ObjectWrap<Producer>(info)
    {
        Napi::Env env = info.Env();

        if (info.Length() >= 2)
        {
            producer_id_ = info[0].As<Napi::String>().Utf8Value();
            topic_ = info[1].As<Napi::String>().Utf8Value();
        }
    }

    Producer::~Producer()
    {
    }

    Napi::Object Producer::Init(Napi::Env env, Napi::Object exports)
    {
        Napi::Function func = DefineClass(env, "Producer", {
                                                               InstanceMethod("publishMessage", &Producer::PublishMessage),
                                                               InstanceMethod("publishOrderedMessage", &Producer::PublishOrderedMessage),
                                                               InstanceMethod("publishDelayMessage", &Producer::PublishDelayMessage),
                                                               InstanceMethod("shutdown", &Producer::Shutdown),
                                                           });

        constructor = Napi::Persistent(func);
        constructor.SuppressDestruct();

        exports.Set("Producer", func);
        return exports;
    }

    Napi::Value Producer::NewInstance(const Napi::CallbackInfo &info)
    {
        return constructor.New({});
    }

    Napi::Value Producer::PublishMessage(const Napi::CallbackInfo &info)
    {
        Napi::Env env = info.Env();
        return Napi::String::New(env, "Producer::PublishMessage - TODO: implement");
    }

    Napi::Value Producer::PublishOrderedMessage(const Napi::CallbackInfo &info)
    {
        Napi::Env env = info.Env();
        return Napi::String::New(env, "Producer::PublishOrderedMessage - TODO: implement");
    }

    Napi::Value Producer::PublishDelayMessage(const Napi::CallbackInfo &info)
    {
        Napi::Env env = info.Env();
        return Napi::String::New(env, "Producer::PublishDelayMessage - TODO: implement");
    }

    Napi::Value Producer::Shutdown(const Napi::CallbackInfo &info)
    {
        Napi::Env env = info.Env();
        return Napi::String::New(env, "Producer::Shutdown - TODO: implement");
    }

    // Consumer 实现 - 使用Node-API
    Consumer::Consumer(const Napi::CallbackInfo &info) : Napi::ObjectWrap<Consumer>(info)
    {
        Napi::Env env = info.Env();

        if (info.Length() >= 3)
        {
            consumer_id_ = info[0].As<Napi::String>().Utf8Value();
            topic_ = info[1].As<Napi::String>().Utf8Value();
            group_id_ = info[2].As<Napi::String>().Utf8Value();
        }
    }

    Consumer::~Consumer()
    {
    }

    Napi::Object Consumer::Init(Napi::Env env, Napi::Object exports)
    {
        Napi::Function func = DefineClass(env, "Consumer", {
                                                               InstanceMethod("onMessage", &Consumer::OnMessage),
                                                               InstanceMethod("startReceiving", &Consumer::StartReceiving),
                                                               InstanceMethod("ackMessage", &Consumer::AckMessage),
                                                               InstanceMethod("shutdown", &Consumer::Shutdown),
                                                           });

        constructor = Napi::Persistent(func);
        constructor.SuppressDestruct();

        exports.Set("Consumer", func);
        return exports;
    }

    Napi::Value Consumer::NewInstance(const Napi::CallbackInfo &info)
    {
        return constructor.New({});
    }

    Napi::Value Consumer::OnMessage(const Napi::CallbackInfo &info)
    {
        Napi::Env env = info.Env();
        return Napi::String::New(env, "Consumer::OnMessage - TODO: implement");
    }

    Napi::Value Consumer::StartReceiving(const Napi::CallbackInfo &info)
    {
        Napi::Env env = info.Env();
        return Napi::String::New(env, "Consumer::StartReceiving - TODO: implement");
    }

    Napi::Value Consumer::AckMessage(const Napi::CallbackInfo &info)
    {
        Napi::Env env = info.Env();
        return Napi::String::New(env, "Consumer::AckMessage - TODO: implement");
    }

    Napi::Value Consumer::Shutdown(const Napi::CallbackInfo &info)
    {
        Napi::Env env = info.Env();
        return Napi::String::New(env, "Consumer::Shutdown - TODO: implement");
    }

    // 工具函数 - 使用Node-API类型
    std::string NapiStringToStdString(const Napi::String &napiStr)
    {
        return napiStr.Utf8Value();
    }

    Napi::String StdStringToNapiString(Napi::Env env, const std::string &str)
    {
        return Napi::String::New(env, str);
    }

    Napi::Object JsonStringToNapiObject(Napi::Env env, const std::string &json_str)
    {
        // 简化实现：返回包含原始JSON字符串的对象
        Napi::Object obj = Napi::Object::New(env);
        obj.Set("raw", Napi::String::New(env, json_str));
        // TODO: 实现完整的JSON解析
        return obj;
    }

    std::string NapiObjectToJsonString(const Napi::Object &obj)
    {
        // 简化实现
        // TODO: 实现完整的JSON序列化
        return "{}";
    }

} // namespace rocketmq_addon

// 模块初始化 - 使用Node-API
Napi::Object InitModule(Napi::Env env, Napi::Object exports)
{
    rocketmq_addon::RocketMQClient::Init(env, exports);
    rocketmq_addon::Producer::Init(env, exports);
    rocketmq_addon::Consumer::Init(env, exports);
    return exports;
}

NODE_API_MODULE(rocketmq_addon, InitModule)