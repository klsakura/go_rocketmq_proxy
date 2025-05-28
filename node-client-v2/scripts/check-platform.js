#!/usr/bin/env node

const os = require('os');
const fs = require('fs');
const path = require('path');

function getPlatformInfo() {
    const platform = os.platform();
    const arch = os.arch();
    const platformKey = `${platform}-${arch}`;

    const platformNames = {
        'darwin-arm64': 'macOS Apple Silicon (M1/M2)',
        'darwin-x64': 'macOS Intel',
        'linux-x64': 'Linux x64',
        'linux-arm64': 'Linux ARM64',
        'win32-x64': 'Windows x64'
    };

    return {
        platform,
        arch,
        platformKey,
        name: platformNames[platformKey] || `${platform}-${arch}`
    };
}

function checkBinaries() {
    const platformInfo = getPlatformInfo();

    console.log(`🔍 Platform: ${platformInfo.name} (${platformInfo.platformKey})`);

    const goLib = path.join(__dirname, '../../cgo/librocketmq_cgo.so');
    const addon = path.join(__dirname, '../../addon/build/Release/rocketmq_addon.node');

    let allFound = true;

    if (!fs.existsSync(goLib)) {
        console.log('❌ Go shared library not found:', goLib);
        allFound = false;
    } else {
        console.log('✅ Go shared library found');
    }

    if (!fs.existsSync(addon)) {
        console.log('❌ Node.js addon not found:', addon);
        allFound = false;
    } else {
        console.log('✅ Node.js addon found');
    }

    return allFound;
}

function showBuildInstructions() {
    const platformInfo = getPlatformInfo();

    console.log('\n🔧 Build Instructions:');
    console.log('='.repeat(50));

    // 系统要求
    console.log('\n📋 System Requirements:');
    console.log('- Go 1.21+ (for CGO shared library)');
    console.log('- Node.js 12+ (for Native Addon)');
    console.log('- C++ compiler:');

    switch (platformInfo.platform) {
        case 'darwin':
            console.log('  • Xcode Command Line Tools: xcode-select --install');
            break;
        case 'linux':
            console.log('  • GCC/G++: sudo apt-get install build-essential (Ubuntu/Debian)');
            console.log('  •          sudo yum groupinstall "Development Tools" (CentOS/RHEL)');
            break;
        case 'win32':
            console.log('  • Visual Studio Build Tools or Visual Studio Community');
            break;
        default:
            console.log('  • Platform-specific C++ compiler');
    }

    // 构建命令
    console.log('\n🚀 Build Commands:');
    console.log('npm run build:all');
    console.log('\nOr step by step:');
    console.log('npm run build:go     # Build Go shared library');
    console.log('npm run build:addon  # Build C++ Native Addon');
    console.log('npm run build:ts     # Build TypeScript SDK');

    // 验证
    console.log('\n✅ Verify Build:');
    console.log('npm run example:health');
}

function main() {
    console.log('🎯 RocketMQ Native SDK - Platform Check');
    console.log('='.repeat(50));

    if (checkBinaries()) {
        console.log('\n🎉 All binaries found! Ready to use.');

        // 显示平台特定警告
        const platformInfo = getPlatformInfo();
        if (platformInfo.platformKey === 'darwin-arm64') {
            console.log('\n⚠️  Note: Binaries compiled for macOS ARM64 (M1/M2)');
            console.log('   These will NOT work on other platforms.');
            console.log('   Users on other platforms need to run: npm run build:all');
        }

        return true;
    } else {
        console.log('\n❌ Missing binaries detected!');
        showBuildInstructions();
        return false;
    }
}

// 如果直接运行此脚本
if (require.main === module) {
    const success = main();
    process.exit(success ? 0 : 1);
}

module.exports = {
    getPlatformInfo,
    checkBinaries,
    showBuildInstructions,
    main
}; 