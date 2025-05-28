#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const { getPlatformInfo, getAllPlatformInfo } = require('../src/platform-loader');

// 支持的平台配置
const PLATFORMS = [
    {
        key: 'darwin-arm64',
        name: 'macOS Apple Silicon (M1/M2)',
        goOS: 'darwin',
        goArch: 'arm64',
        cgoEnabled: '1'
    },
    {
        key: 'darwin-x64',
        name: 'macOS Intel',
        goOS: 'darwin',
        goArch: 'amd64',
        cgoEnabled: '1'
    },
    {
        key: 'linux-x64',
        name: 'Linux x64',
        goOS: 'linux',
        goArch: 'amd64',
        cgoEnabled: '1'
    },
    {
        key: 'linux-arm64',
        name: 'Linux ARM64',
        goOS: 'linux',
        goArch: 'arm64',
        cgoEnabled: '1'
    },
    {
        key: 'win32-x64',
        name: 'Windows x64',
        goOS: 'windows',
        goArch: 'amd64',
        cgoEnabled: '1'
    }
];

function log(message) {
    console.log(`[BUILD] ${message}`);
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

function ensureDirectory(dir) {
    if (!fs.existsSync(dir)) {
        fs.mkdirSync(dir, { recursive: true });
        log(`Created directory: ${dir}`);
    }
}

function getGoLibExtension(platform) {
    if (platform.startsWith('win32')) return '.dll';
    if (platform.startsWith('darwin')) return '.dylib';
    return '.so';
}

function buildGoLibrary(platform) {
    log(`Building Go library for ${platform.name}...`);

    const cgoDir = path.join(__dirname, '../../cgo');
    const outputDir = path.join(__dirname, '../prebuilds', platform.key);
    const extension = getGoLibExtension(platform.key);
    const outputFile = path.join(outputDir, `librocketmq_cgo${extension}`);

    ensureDirectory(outputDir);

    // 设置环境变量
    const env = {
        ...process.env,
        GOOS: platform.goOS,
        GOARCH: platform.goArch,
        CGO_ENABLED: platform.cgoEnabled
    };

    // 对于交叉编译，需要特殊处理
    if (platform.key !== getPlatformInfo().platformKey) {
        log(`⚠️  Cross-compiling for ${platform.name} - this may require additional setup`);

        // 对于Linux目标，尝试设置交叉编译工具链
        if (platform.goOS === 'linux' && process.platform === 'darwin') {
            // macOS -> Linux 交叉编译通常需要额外的工具
            log(`⚠️  macOS -> Linux cross-compilation may fail without proper toolchain`);
        }

        // 对于Windows目标
        if (platform.goOS === 'windows' && process.platform !== 'win32') {
            log(`⚠️  Cross-compiling to Windows may require mingw-w64`);
        }
    }

    // 构建命令
    const buildCmd = `go build -buildmode=c-shared -o "${outputFile}" rocketmq_cgo.go`;

    try {
        execCommand(buildCmd, {
            cwd: cgoDir,
            env: env
        });
        log(`✅ Go library built: ${outputFile}`);
        return true;
    } catch (err) {
        error(`❌ Failed to build Go library for ${platform.name}: ${err.message}`);

        // 提供具体的解决建议
        if (platform.key !== getPlatformInfo().platformKey) {
            log(`💡 Cross-compilation failed. Consider:`);
            log(`   1. Use Docker: npm run docker:build ${platform.key}`);
            log(`   2. Use GitHub Actions for automatic builds`);
            log(`   3. Build on the target platform directly`);
        }

        return false;
    }
}

function buildNodeAddon(platform) {
    log(`Building Node.js addon for ${platform.name}...`);

    const addonDir = path.join(__dirname, '../../addon');
    const outputDir = path.join(__dirname, '../prebuilds', platform.key);
    const sourceFile = path.join(addonDir, 'build/Release/rocketmq_addon.node');
    const targetFile = path.join(outputDir, 'rocketmq_addon.node');

    ensureDirectory(outputDir);

    try {
        // 注意：Node.js addon需要在目标平台上构建
        // 这里我们假设当前平台可以构建，或者使用交叉编译
        if (platform.key === getPlatformInfo().platformKey) {
            // 当前平台，直接构建
            execCommand('npm run build', { cwd: addonDir });

            // 复制到prebuilds目录
            if (fs.existsSync(sourceFile)) {
                fs.copyFileSync(sourceFile, targetFile);
                log(`✅ Node.js addon built: ${targetFile}`);
                return true;
            } else {
                error(`❌ Addon build output not found: ${sourceFile}`);
                return false;
            }
        } else {
            // 跨平台构建 - 需要特殊处理
            log(`⚠️  Cross-platform build for ${platform.name} requires target platform or Docker`);
            return false;
        }
    } catch (err) {
        error(`❌ Failed to build Node.js addon for ${platform.name}: ${err.message}`);
        return false;
    }
}

function buildCurrentPlatform() {
    const currentPlatform = getPlatformInfo();
    const platform = PLATFORMS.find(p => p.key === currentPlatform.platformKey);

    if (!platform) {
        error(`Unsupported platform: ${currentPlatform.platformKey}`);
        return false;
    }

    log(`Building for current platform: ${platform.name}`);

    const goSuccess = buildGoLibrary(platform);
    const addonSuccess = buildNodeAddon(platform);

    return goSuccess && addonSuccess;
}

function buildAllPlatforms() {
    log('Building for all platforms...');

    let successCount = 0;
    let totalCount = PLATFORMS.length;

    for (const platform of PLATFORMS) {
        log(`\n${'='.repeat(60)}`);
        log(`Building ${platform.name} (${platform.key})`);
        log(`${'='.repeat(60)}`);

        const goSuccess = buildGoLibrary(platform);
        const addonSuccess = buildNodeAddon(platform);

        if (goSuccess && addonSuccess) {
            successCount++;
            log(`✅ ${platform.name} build completed successfully`);
        } else {
            error(`❌ ${platform.name} build failed`);
        }
    }

    log(`\n${'='.repeat(60)}`);
    log(`Build Summary: ${successCount}/${totalCount} platforms successful`);
    log(`${'='.repeat(60)}`);

    return successCount === totalCount;
}

function checkBuilds() {
    log('Checking existing builds...');

    const platformInfo = getAllPlatformInfo();

    console.log('\nPlatform Build Status:');
    console.log('='.repeat(80));

    for (const info of platformInfo) {
        const goStatus = info.exists.goLib ? '✅' : '❌';
        const addonStatus = info.exists.addon ? '✅' : '❌';

        console.log(`${info.platformKey.padEnd(15)} | Go: ${goStatus} | Addon: ${addonStatus}`);
    }

    console.log('='.repeat(80));
}

function main() {
    const args = process.argv.slice(2);
    const command = args[0] || 'current';

    console.log('🚀 RocketMQ Native SDK - Multi-Platform Builder');
    console.log('='.repeat(60));

    switch (command) {
        case 'current':
            return buildCurrentPlatform();

        case 'all':
            return buildAllPlatforms();

        case 'check':
            checkBuilds();
            return true;

        case 'go':
            const currentPlatform = getPlatformInfo();
            const platform = PLATFORMS.find(p => p.key === currentPlatform.platformKey);
            return buildGoLibrary(platform);

        default:
            console.log('Usage:');
            console.log('  node build-all-platforms.js [command]');
            console.log('');
            console.log('Commands:');
            console.log('  current  - Build for current platform (default)');
            console.log('  all      - Build for all platforms');
            console.log('  check    - Check existing builds');
            console.log('  go       - Build only Go library for current platform');
            return false;
    }
}

if (require.main === module) {
    const success = main();
    process.exit(success ? 0 : 1);
}

module.exports = {
    buildCurrentPlatform,
    buildAllPlatforms,
    checkBuilds,
    PLATFORMS
}; 