# GLIBC 兼容性说明

## 问题描述

在字节云 VCI 环境中部署时遇到了 GLIBC 版本兼容性问题：

```
rsync: /lib/x86_64-linux-gnu/libc.so.6: version `GLIBC_2.32' not found (required by /app/node_modules/@klsakura/rocketmq-native-sdk/prebuilds/linux-x64/rocketmq_addon.node)
```

**根本原因：**
- 字节云 VCI 环境使用 GLIBC 2.31
- 原构建环境 `ubuntu-latest` (Ubuntu 22.04+) 使用 GLIBC 2.35+
- 导致预编译的二进制文件不兼容

## 解决方案

### 容器化构建方案

由于 GitHub Actions 中的 `ubuntu-20.04` runner 即将在 2025-04-15 弃用，我们采用容器化构建方案：

```yaml
# 新方案：使用容器化构建
- platform: linux-x64
  os: ubuntu-latest        # 使用最新的 runner
  container: ubuntu:20.04  # 在容器中运行 Ubuntu 20.04 (GLIBC 2.31)
  arch: x64
  go_os: linux
  go_arch: amd64
  node-version: '16.19.0'
```

### 方案优势

✅ **避免 runner 弃用**：使用 `ubuntu-latest` runner，不受 Ubuntu 20.04 弃用影响  
✅ **保持 GLIBC 兼容性**：容器内运行 Ubuntu 20.04，确保 GLIBC 2.31 兼容性  
✅ **环境一致性**：容器环境完全可控，避免 runner 环境变化影响  
✅ **长期可维护**：可以根据需要切换到更老或更新的容器版本  

### 环境版本对比

| 构建环境 | Runner | 容器 | GLIBC 版本 | 与字节云兼容性 |
|----------|--------|------|-----------|---------------|
| 字节云 VCI | - | Ubuntu 20.04 | 2.31 | ✅ 目标环境 |
| 新方案 | ubuntu-latest | ubuntu:20.04 | 2.31 | ✅ **完全兼容** |
| 旧方案 | ubuntu-20.04 | - | 2.31 | ⚠️ 即将弃用 |
| 默认方案 | ubuntu-latest | - | 2.35+ | ❌ 不兼容 |

## 技术实现

### 容器环境设置

1. **基础环境**：在 Ubuntu 20.04 容器中安装必要依赖
2. **Node.js 16.19.0**：精确版本控制，确保 MODULE_VERSION 93
3. **Go 1.21**：手动安装，支持 CGO 编译
4. **Python 3 + distutils**：node-gyp 构建依赖

### 构建流程

```bash
# 1. 容器环境设置
apt-get update && apt-get install -y build-essential curl wget git

# 2. 安装 Node.js 16.19.0
curl -fsSL https://deb.nodesource.com/setup_16.x | bash -
apt-get install -y nodejs=16.19.0-1nodesource1

# 3. 安装 Go 1.21
wget https://go.dev/dl/go1.21.13.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.21.13.linux-amd64.tar.gz

# 4. 构建 Go 共享库和 Node.js addon
```

## 验证兼容性

使用提供的脚本验证生成的二进制文件：

```bash
./scripts/verify-glibc-compatibility.sh
```

该脚本会检查：
- `rocketmq_addon.node` 的 GLIBC 依赖
- `librocketmq_cgo.so` 的 GLIBC 依赖  
- 与字节云 VCI 环境的兼容性

## 修改的文件

1. **`.github/workflows/build-prebuilds.yml`**
   - 采用容器化构建：`ubuntu-latest` + `ubuntu:20.04` 容器
   - Linux 环境专用的设置步骤
   - 添加了容器环境验证和 GLIBC 版本检查

2. **`scripts/verify-glibc-compatibility.sh`**
   - 验证预编译二进制文件的 GLIBC 兼容性

3. **`GLIBC_COMPATIBILITY.md`**
   - 更新为容器化构建方案说明

## 最佳实践

1. **构建环境选择**
   - 使用容器化方式确保环境一致性
   - 选择目标部署环境兼容的容器版本

2. **兼容性测试**
   - 每次构建后运行兼容性验证脚本
   - 在实际部署环境中测试二进制文件

3. **版本管理**
   - 精确控制 Node.js 和 Go 版本
   - 明确标记支持的最低 GLIBC 版本

## 影响范围

这个修改只影响 Linux x64 平台的构建方式，不会影响：
- macOS 平台 (darwin-arm64, darwin-x64) - 继续使用原生 runner
- Windows 平台 (win32-x64) - 继续使用原生 runner  
- 源码编译方式 - 完全不受影响

## 故障排除

### 常见问题

1. **容器权限问题**
   - GitHub Actions 容器默认以 root 用户运行
   - 无需额外的权限配置

2. **网络访问问题**
   - 容器可以正常访问外部网络
   - 包管理器和下载操作正常工作

3. **文件系统问题**
   - GitHub Actions 自动挂载工作目录
   - 构建产物可以正常上传

### 调试方法

```bash
# 检查容器环境
echo "Container OS: $(cat /etc/os-release | grep PRETTY_NAME)"
echo "GLIBC version: $(ldd --version | head -1)"

# 检查工具版本
node --version
npm --version  
go version
python3 --version
```

## 未来考虑

1. **更老版本支持**
   - 可切换到 `ubuntu:18.04` 容器 (GLIBC 2.27)
   - 可使用 `centos:7` 容器 (GLIBC 2.17)

2. **静态链接**
   - 考虑静态链接关键系统库
   - 进一步提高兼容性

3. **多版本支持**
   - 可以同时构建多个 GLIBC 版本的二进制文件
   - 运行时根据环境选择合适版本 