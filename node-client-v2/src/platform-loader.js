const os = require('os');
const path = require('path');
const fs = require('fs');

/**
 * 检查Node.js版本兼容性
 */
function checkNodeVersionCompatibility() {
    const nodeVersion = process.versions.node;
    const majorVersion = parseInt(nodeVersion.split('.')[0]);
    const minorVersion = parseInt(nodeVersion.split('.')[1]);

    // 检查最低版本要求：Node.js 16.0.0
    if (majorVersion < 16) {
        throw new Error(
            `Node.js version ${nodeVersion} is not supported.\n` +
            `Minimum required version: 16.0.0\n` +
            `Please upgrade Node.js: https://nodejs.org/`
        );
    }

    // 显示兼容性信息
    console.log(`✅ Node.js ${nodeVersion} is compatible with RocketMQ Native SDK`);

    return {
        version: nodeVersion,
        majorVersion,
        minorVersion,
        isSupported: true
    };
}

/**
 * 获取当前平台信息
 */
function getPlatformInfo() {
    const platform = os.platform();
    const arch = os.arch();

    // 标准化架构名称
    const normalizedArch = arch === 'x64' ? 'x64' : arch;

    return {
        platform,
        arch: normalizedArch,
        platformKey: `${platform}-${normalizedArch}`
    };
}

/**
 * 获取平台特定的二进制文件路径
 */
function getPlatformPaths(platformKey) {
    const prebuildsDir = path.join(__dirname, '../prebuilds', platformKey);

    return {
        goLib: path.join(prebuildsDir, getGoLibName(platformKey)),
        addon: path.join(prebuildsDir, 'rocketmq_addon.node')
    };
}

/**
 * 获取Go共享库的文件名（不同平台扩展名不同）
 */
function getGoLibName(platformKey) {
    if (platformKey.startsWith('win32')) {
        return 'librocketmq_cgo.dll';
    } else if (platformKey.startsWith('darwin')) {
        return 'librocketmq_cgo.dylib';
    } else {
        return 'librocketmq_cgo.so';
    }
}

/**
 * 检查平台是否支持
 */
function isSupportedPlatform(platformKey) {
    const supportedPlatforms = [
        'darwin-arm64',  // macOS Apple Silicon
        'darwin-x64',    // macOS Intel
        'linux-x64',     // Linux x64
        'linux-arm64',   // Linux ARM64
        'win32-x64'      // Windows x64
    ];

    return supportedPlatforms.includes(platformKey);
}

/**
 * 加载Native Addon
 */
function loadNativeAddon() {
    // 首先检查Node.js版本兼容性
    checkNodeVersionCompatibility();

    const platformInfo = getPlatformInfo();
    const { platformKey } = platformInfo;

    // 检查平台支持
    if (!isSupportedPlatform(platformKey)) {
        throw new Error(
            `Unsupported platform: ${platformKey}\n` +
            `Supported platforms: darwin-arm64, darwin-x64, linux-x64, linux-arm64, win32-x64\n` +
            `Please build from source using: npm run build:all`
        );
    }

    const paths = getPlatformPaths(platformKey);

    // 检查文件是否存在
    if (!fs.existsSync(paths.addon)) {
        throw new Error(
            `Native addon not found for platform ${platformKey}\n` +
            `Expected: ${paths.addon}\n` +
            `Please run: npm run build:all`
        );
    }

    if (!fs.existsSync(paths.goLib)) {
        throw new Error(
            `Go shared library not found for platform ${platformKey}\n` +
            `Expected: ${paths.goLib}\n` +
            `Please run: npm run build:all`
        );
    }

    try {
        // 加载Native Addon
        const addon = require(paths.addon);

        console.log(`✅ Loaded RocketMQ Native Addon for ${platformKey}`);
        return addon;
    } catch (error) {
        throw new Error(
            `Failed to load native addon for ${platformKey}: ${error.message}\n` +
            `Addon path: ${paths.addon}\n` +
            `Go library path: ${paths.goLib}\n` +
            `Try rebuilding: npm run build:all`
        );
    }
}

/**
 * 获取所有平台的构建信息
 */
function getAllPlatformInfo() {
    const platforms = [
        'darwin-arm64',
        'darwin-x64',
        'linux-x64',
        'linux-arm64',
        'win32-x64'
    ];

    return platforms.map(platformKey => {
        const paths = getPlatformPaths(platformKey);
        return {
            platformKey,
            paths,
            exists: {
                addon: fs.existsSync(paths.addon),
                goLib: fs.existsSync(paths.goLib)
            }
        };
    });
}

module.exports = {
    checkNodeVersionCompatibility,
    getPlatformInfo,
    getPlatformPaths,
    getGoLibName,
    isSupportedPlatform,
    loadNativeAddon,
    getAllPlatformInfo
}; 