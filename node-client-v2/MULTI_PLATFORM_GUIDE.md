# RocketMQ Native SDK - 多平台构建和发布指南

## 🎯 概述

本指南介绍如何为RocketMQ Native SDK构建多平台预编译二进制文件并发布到npm。

## 🏗️ 架构说明

### 预编译二进制文件结构
```
node-client-v2/
├── prebuilds/                    # 预编译二进制文件目录
│   ├── darwin-arm64/             # macOS Apple Silicon
│   │   ├── librocketmq_cgo.dylib # Go共享库
│   │   └── rocketmq_addon.node   # Node.js Native Addon
│   ├── darwin-x64/               # macOS Intel
│   │   ├── librocketmq_cgo.dylib
│   │   └── rocketmq_addon.node
│   ├── linux-x64/                # Linux x64
│   │   ├── librocketmq_cgo.so
│   │   └── rocketmq_addon.node
│   ├── linux-arm64/              # Linux ARM64
│   │   ├── librocketmq_cgo.so
│   │   └── rocketmq_addon.node
│   └── win32-x64/                # Windows x64
│       ├── librocketmq_cgo.dll
│       └── rocketmq_addon.node
├── src/
│   ├── platform-loader.js        # 平台检测和加载器
│   └── index.ts                  # 主入口文件
└── dist/                         # 编译输出
    ├── platform-loader.js        # 编译后的平台加载器
    └── ...
```

### 平台加载机制

1. **自动检测**: 运行时自动检测当前平台 (`os.platform()` + `os.arch()`)
2. **路径映射**: 根据平台映射到对应的预编译文件路径
3. **动态加载**: 加载对应平台的Native Addon
4. **错误处理**: 提供清晰的错误信息和构建指导

## 🔧 本地构建

### 当前平台构建
```bash
# 构建当前平台的二进制文件
npm run build:platforms

# 或者分步构建
npm run build:platforms:go    # 只构建Go库
npm run build:addon          # 只构建Node.js addon
npm run build               # 只构建TypeScript
```

### 检查构建状态
```bash
# 检查所有平台的构建状态
npm run build:platforms:check

# 检查当前平台兼容性
npm run check:platform
```

### 测试构建结果
```bash
# 健康检查
npm run example:health

# 测试生产者
npm run example:producer

# 测试消费者
npm run example:consumer
```

## 🚀 GitHub Actions 自动构建

### 工作流配置

GitHub Actions工作流 (`.github/workflows/build-prebuilds.yml`) 自动为所有支持的平台构建：

- **触发条件**: 
  - Push到main/develop分支
  - 创建版本标签 (v*)
  - 手动触发

- **构建矩阵**:
  - macOS (Intel & Apple Silicon)
  - Linux (x64 & ARM64)
  - Windows (x64)

### 使用方法

1. **推送代码触发**:
   ```bash
   git push origin main
   ```

2. **手动触发**:
   - 访问GitHub Actions页面
   - 选择"Build Prebuilt Binaries"工作流
   - 点击"Run workflow"

3. **版本发布触发**:
   ```bash
   git tag v2.0.0
   git push origin v2.0.0
   ```

### 下载构建产物

构建完成后，可以从GitHub Actions页面下载：
- `rocketmq-native-sdk-prebuilds` - 包含所有平台的预编译文件
- `prebuilds-{platform}` - 单个平台的构建文件

## 📦 发布流程

### 1. 准备发布

```bash
# 完整的发布准备检查
npm run release:prepare

# 或者分步检查
npm run release:check     # 检查发布条件
npm run release:test      # 运行测试
npm run release:notes     # 生成发布说明
```

### 2. 获取预编译文件

**方法一: 从GitHub Actions下载**
1. 等待GitHub Actions构建完成
2. 下载`rocketmq-native-sdk-prebuilds`
3. 解压到`node-client-v2/prebuilds/`目录

**方法二: 本地构建（仅当前平台）**
```bash
npm run build:platforms
```

### 3. 验证发布包

```bash
# 检查所有平台的二进制文件
npm run build:platforms:check

# 验证发布条件
npm run release:check
```

### 4. 发布到npm

```bash
# 发布（会自动运行prepublishOnly检查）
npm publish

# 或者发布到特定tag
npm publish --tag beta
```

## 🔍 故障排除

### 常见问题

#### 1. 缺少预编译文件
```
❌ Missing binaries for: linux-x64, win32-x64
```

**解决方案**:
- 等待GitHub Actions构建完成
- 或在对应平台上手动构建
- 或使用Docker进行交叉编译

#### 2. 平台不支持
```
Unsupported platform: linux-arm
```

**解决方案**:
- 检查`src/platform-loader.js`中的支持平台列表
- 添加新平台支持或从源码构建

#### 3. Native Addon加载失败
```
Failed to load native addon: dlopen error
```

**解决方案**:
- 确认二进制文件是为正确平台编译的
- 检查依赖库是否安装
- 重新构建当前平台的二进制文件

### 调试命令

```bash
# 查看平台信息
node -e "console.log(require('./src/platform-loader').getPlatformInfo())"

# 查看所有平台状态
node -e "console.log(require('./src/platform-loader').getAllPlatformInfo())"

# 测试加载
node -e "require('./src/platform-loader').loadNativeAddon()"
```

## 📋 支持的平台

| 平台 | 架构 | Go目标 | 状态 | 说明 |
|------|------|--------|------|------|
| macOS | ARM64 | darwin/arm64 | ✅ | Apple Silicon (M1/M2) |
| macOS | x64 | darwin/amd64 | ✅ | Intel Mac |
| Linux | x64 | linux/amd64 | ✅ | 标准Linux发行版 |
| Linux | ARM64 | linux/arm64 | ✅ | ARM64 Linux |
| Windows | x64 | windows/amd64 | ✅ | Windows 10/11 |

## 🔄 CI/CD 最佳实践

### 版本管理
1. 使用语义化版本 (semver)
2. 为每个版本创建Git标签
3. 自动触发构建和发布

### 质量保证
1. 每次构建都运行测试
2. 验证所有平台的二进制文件
3. 自动生成发布说明

### 发布策略
1. 开发版本发布到`beta`标签
2. 稳定版本发布到`latest`标签
3. 保留历史版本的预编译文件

## 📚 相关文档

- [README.md](./README.md) - 项目主文档
- [package.json](./package.json) - npm包配置
- [.github/workflows/build-prebuilds.yml](../.github/workflows/build-prebuilds.yml) - GitHub Actions配置
- [scripts/](./scripts/) - 构建和发布脚本

## 🤝 贡献指南

1. Fork项目
2. 创建功能分支
3. 确保所有平台构建通过
4. 提交Pull Request
5. 等待CI/CD验证

---

**注意**: 这是一个纯Native实现，无需gRPC服务器！🎉 