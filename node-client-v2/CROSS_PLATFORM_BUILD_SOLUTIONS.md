# RocketMQ Native SDK - 跨平台构建解决方案

## 🔍 问题分析

### 为什么不能像Go一样简单交叉编译？

1. **Go CGO限制** ❌
   - 我们的Go代码使用了CGO (`import "C"`)
   - CGO需要目标平台的C编译器和系统库
   - `GOOS=linux GOARCH=amd64` 在macOS上失败，因为缺少Linux的系统调用

2. **Node.js Native Addon限制** ❌
   - node-gyp需要目标平台的Node.js头文件
   - 需要目标平台的Python和构建工具链
   - 二进制文件与平台强绑定

## 🚀 可行的解决方案

### 方案1: GitHub Actions (推荐) ⭐⭐⭐⭐⭐

**优势**:
- ✅ 完全自动化
- ✅ 支持所有主流平台
- ✅ 免费且可靠
- ✅ 已经实现

**使用方法**:
```bash
# 1. 推送代码触发构建
git push origin main

# 2. 或创建版本标签
git tag v2.0.1
git push origin v2.0.1

# 3. 下载构建产物
# 从GitHub Actions页面下载 rocketmq-native-sdk-prebuilds
```

**构建矩阵**:
- macOS (Intel + ARM64)
- Linux (x64 + ARM64) 
- Windows (x64)

### 方案2: Docker交叉编译 ⭐⭐⭐⭐

**优势**:
- ✅ 本地可控
- ✅ 支持Linux和Windows
- ✅ 一致的构建环境

**使用方法**:
```bash
# 构建Linux x64
npm run docker:build:linux

# 构建Linux ARM64  
npm run docker:build:linux-arm

# 构建所有Docker支持的平台
npm run docker:build:all
```

**限制**:
- 需要Docker Desktop
- macOS构建仍需在macOS上进行

### 方案3: 云构建服务 ⭐⭐⭐

**选项**:
- GitHub Actions (已实现)
- GitLab CI
- Azure DevOps
- CircleCI

### 方案4: 本地虚拟机 ⭐⭐

**适用场景**:
- 需要完全离线构建
- 特殊的构建需求

**设置**:
- VMware/VirtualBox运行Linux/Windows
- 在各虚拟机中分别构建

## 🎯 推荐的工作流程

### 开发阶段
```bash
# 1. 本地开发和测试 (当前平台)
npm run build:platforms        # 构建当前平台
npm run example:health         # 测试功能

# 2. 提交代码
git add .
git commit -m "feat: 新功能"
git push origin develop
```

### 发布阶段
```bash
# 1. 创建发布分支
git checkout -b release/v2.0.1

# 2. 更新版本号
npm version 2.0.1

# 3. 推送触发GitHub Actions
git push origin release/v2.0.1

# 4. 等待所有平台构建完成
# 5. 下载构建产物到 prebuilds/

# 6. 验证和发布
npm run release:prepare
npm publish
```

## 🔧 当前可用的构建方法

### 1. 当前平台构建 ✅
```bash
npm run build:platforms        # 构建当前平台 (darwin-arm64)
```

### 2. GitHub Actions ✅
- 自动触发：推送到main/develop分支
- 手动触发：GitHub网页操作
- 版本发布：创建git标签

### 3. Docker构建 ✅ (新增)
```bash
npm run docker:build:linux     # Linux x64
npm run docker:build:linux-arm # Linux ARM64
npm run docker:build:all       # 所有Docker支持的平台
```

## 📊 各方案对比

| 方案 | 自动化 | 成本 | 可靠性 | 设置复杂度 | 支持平台 |
|------|--------|------|--------|------------|----------|
| GitHub Actions | ⭐⭐⭐⭐⭐ | 免费 | ⭐⭐⭐⭐⭐ | ⭐⭐ | 5个平台 |
| Docker | ⭐⭐⭐⭐ | 免费 | ⭐⭐⭐⭐ | ⭐⭐⭐ | 3个平台 |
| 云服务 | ⭐⭐⭐⭐⭐ | 付费 | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | 5个平台 |
| 虚拟机 | ⭐⭐ | 免费 | ⭐⭐⭐ | ⭐⭐⭐⭐⭐ | 5个平台 |

## 🎉 最佳实践

### 1. 主要依赖GitHub Actions
- 用于正式发布
- 自动化程度最高
- 支持所有平台

### 2. Docker作为备选
- 本地测试Linux构建
- 快速验证修改

### 3. 本地构建当前平台
- 开发和调试
- 快速迭代

## 🔮 未来优化

### 1. 减少CGO依赖
- 考虑纯Go实现
- 或使用更轻量的C绑定

### 2. 预编译缓存
- 缓存构建产物
- 增量构建优化

### 3. 更多平台支持
- Alpine Linux (musl)
- FreeBSD
- ARM32

## 💡 立即可用的解决方案

**对于你的需求，最简单的方案是**:

1. **使用GitHub Actions** (推荐)
   ```bash
   git push origin main  # 触发自动构建
   # 等待构建完成，下载产物
   ```

2. **使用Docker构建Linux**
   ```bash
   npm run docker:build:linux
   ```

3. **继续使用当前平台**
   ```bash
   npm run build:platforms  # 构建macOS ARM64
   ```

这样你就可以获得多平台的预编译二进制文件，而不需要在每个平台上都有物理机器！🎉 