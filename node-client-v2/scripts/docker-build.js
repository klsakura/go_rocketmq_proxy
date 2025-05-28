#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

// Docker镜像配置
const DOCKER_IMAGES = {
    'linux-x64': {
        image: 'node:16-bullseye',
        platform: 'linux/amd64',
        goOS: 'linux',
        goArch: 'amd64'
    },
    'linux-arm64': {
        image: 'node:16-bullseye',
        platform: 'linux/arm64',
        goOS: 'linux',
        goArch: 'arm64'
    },
    'win32-x64': {
        image: 'node:16-windowsservercore',
        platform: 'windows/amd64',
        goOS: 'windows',
        goArch: 'amd64'
    }
};

function log(message) {
    console.log(`[DOCKER] ${message}`);
}

function error(message) {
    console.error(`[ERROR] ${message}`);
}

function execCommand(command, options = {}) {
    log(`Executing: ${command}`);
    try {
        return execSync(command, {
            stdio: 'inherit',
            encoding: 'utf8',
            ...options
        });
    } catch (err) {
        error(`Command failed: ${command}`);
        throw err;
    }
}

function createDockerfile(platform) {
    const config = DOCKER_IMAGES[platform];
    if (!config) {
        throw new Error(`Unsupported platform: ${platform}`);
    }

    const dockerfile = `
FROM ${config.image}

# 安装Go
RUN apt-get update && apt-get install -y wget
RUN wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
RUN tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
ENV PATH="/usr/local/go/bin:$PATH"

# 安装构建工具
RUN apt-get install -y build-essential python3 python3-pip

# 设置工作目录
WORKDIR /workspace

# 复制源代码
COPY . .

# 构建脚本
RUN chmod +x /workspace/scripts/docker-build-inside.sh
CMD ["/workspace/scripts/docker-build-inside.sh", "${platform}"]
`;

    const dockerfilePath = path.join(__dirname, `../Dockerfile.${platform}`);
    fs.writeFileSync(dockerfilePath, dockerfile.trim());
    return dockerfilePath;
}

function createBuildScript() {
    const script = `#!/bin/bash
set -e

PLATFORM=$1
echo "Building for platform: $PLATFORM"

# 构建Go库
cd /workspace/cgo
case $PLATFORM in
    "linux-x64")
        GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -buildmode=c-shared -o ../node-client-v2/prebuilds/linux-x64/librocketmq_cgo.so rocketmq_cgo.go
        ;;
    "linux-arm64")
        GOOS=linux GOARCH=arm64 CGO_ENABLED=1 go build -buildmode=c-shared -o ../node-client-v2/prebuilds/linux-arm64/librocketmq_cgo.so rocketmq_cgo.go
        ;;
    "win32-x64")
        GOOS=windows GOARCH=amd64 CGO_ENABLED=1 go build -buildmode=c-shared -o ../node-client-v2/prebuilds/win32-x64/librocketmq_cgo.dll rocketmq_cgo.go
        ;;
esac

# 构建Node.js addon
cd /workspace/addon
npm ci
npm run build

# 复制addon到prebuilds
mkdir -p /workspace/node-client-v2/prebuilds/$PLATFORM
cp build/Release/rocketmq_addon.node /workspace/node-client-v2/prebuilds/$PLATFORM/

echo "Build completed for $PLATFORM"
`;

    const scriptPath = path.join(__dirname, '../scripts/docker-build-inside.sh');
    fs.writeFileSync(scriptPath, script);
    fs.chmodSync(scriptPath, '755');
    return scriptPath;
}

function buildWithDocker(platform) {
    log(`Building ${platform} with Docker...`);

    const config = DOCKER_IMAGES[platform];
    if (!config) {
        error(`Unsupported platform: ${platform}`);
        return false;
    }

    try {
        // 创建Dockerfile和构建脚本
        const dockerfilePath = createDockerfile(platform);
        createBuildScript();

        // 确保输出目录存在
        const outputDir = path.join(__dirname, '../prebuilds', platform);
        if (!fs.existsSync(outputDir)) {
            fs.mkdirSync(outputDir, { recursive: true });
        }

        // 构建Docker镜像
        const imageName = `rocketmq-builder-${platform}`;
        execCommand(`docker build --platform ${config.platform} -f ${dockerfilePath} -t ${imageName} .`, {
            cwd: path.join(__dirname, '../..')
        });

        // 运行构建
        execCommand(`docker run --rm --platform ${config.platform} -v "${path.join(__dirname, '../prebuilds')}:/workspace/node-client-v2/prebuilds" ${imageName}`, {
            cwd: path.join(__dirname, '../..')
        });

        log(`✅ ${platform} build completed successfully`);
        return true;
    } catch (err) {
        error(`❌ ${platform} build failed: ${err.message}`);
        return false;
    }
}

function buildAllWithDocker() {
    log('Building all platforms with Docker...');

    const platforms = Object.keys(DOCKER_IMAGES);
    let successCount = 0;

    for (const platform of platforms) {
        if (buildWithDocker(platform)) {
            successCount++;
        }
    }

    log(`Docker build summary: ${successCount}/${platforms.length} platforms successful`);
    return successCount === platforms.length;
}

function main() {
    const args = process.argv.slice(2);
    const command = args[0];
    const platform = args[1];

    console.log('🐳 RocketMQ Native SDK - Docker Cross-Platform Builder');
    console.log('='.repeat(60));

    switch (command) {
        case 'build':
            if (!platform) {
                error('Please specify platform: linux-x64, linux-arm64, win32-x64');
                return false;
            }
            return buildWithDocker(platform);

        case 'all':
            return buildAllWithDocker();

        default:
            console.log('Usage:');
            console.log('  node docker-build.js build <platform>');
            console.log('  node docker-build.js all');
            console.log('');
            console.log('Platforms:');
            console.log('  linux-x64, linux-arm64, win32-x64');
            return false;
    }
}

if (require.main === module) {
    const success = main();
    process.exit(success ? 0 : 1);
}

module.exports = {
    buildWithDocker,
    buildAllWithDocker,
    DOCKER_IMAGES
}; 