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

### 修改构建环境

将 Linux 构建环境从 `ubuntu-latest` 降级到 `ubuntu-20.04`：

```yaml
# 修改前
- platform: linux-x64
  os: ubuntu-latest  # Ubuntu 22.04+, GLIBC 2.35+

# 修改后  
- platform: linux-x64
  os: ubuntu-20.04   # Ubuntu 20.04, GLIBC 2.31
```

### 环境版本对比

| 环境 | Ubuntu 版本 | GLIBC 版本 | 兼容性 |
|------|------------|-----------|--------|
| 字节云 VCI | Ubuntu 20.04 | 2.31 | ✅ 目标环境 |
| ubuntu-20.04 | Ubuntu 20.04 | 2.31 | ✅ 兼容 |
| ubuntu-22.04 | Ubuntu 22.04 | 2.35 | ❌ 不兼容 |
| ubuntu-latest | Ubuntu 22.04+ | 2.35+ | ❌ 不兼容 |

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
   - Linux 构建环境：`ubuntu-latest` → `ubuntu-20.04`
   - Combine 任务环境：`ubuntu-latest` → `ubuntu-20.04`
   - 添加了 GLIBC 兼容性说明

2. **`scripts/verify-glibc-compatibility.sh`** (新增)
   - 验证预编译二进制文件的 GLIBC 兼容性

3. **`GLIBC_COMPATIBILITY.md`** (新增)
   - 详细说明兼容性问题和解决方案

## 最佳实践

1. **构建环境选择**
   - 使用目标部署环境的最低版本进行构建
   - Linux 生产环境通常使用较老版本的 GLIBC

2. **兼容性测试**
   - 每次构建后运行兼容性验证脚本
   - 在目标环境中测试二进制文件

3. **版本管理**
   - 明确标记支持的最低 GLIBC 版本
   - 在发布说明中包含兼容性信息

## 影响范围

这个修改只影响 Linux x64 平台的预编译二进制文件，不会影响：
- macOS 平台 (darwin-arm64, darwin-x64)
- Windows 平台 (win32-x64)
- 源码编译方式

## 未来考虑

如果需要支持更老的 Linux 发行版，可以考虑：
- 使用 Ubuntu 18.04 (GLIBC 2.27)
- 使用 CentOS 7 (GLIBC 2.17)
- 静态链接关键依赖 