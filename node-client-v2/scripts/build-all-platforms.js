#!/usr/bin/env node

/**
 * Cross-platform build verification script
 * Used by GitHub Actions to validate prebuilt binaries
 */

const fs = require('fs');
const path = require('path');

const SUPPORTED_PLATFORMS = [
    'darwin-arm64',
    'darwin-x64',
    'linux-x64',
    'win32-x64'
];

const REQUIRED_FILES = {
    'darwin-arm64': ['rocketmq_addon.node', 'librocketmq_cgo.dylib'],
    'darwin-x64': ['rocketmq_addon.node', 'librocketmq_cgo.dylib'],
    'linux-x64': ['rocketmq_addon.node', 'librocketmq_cgo.so'],
    'win32-x64': ['rocketmq_addon.node', 'librocketmq_cgo.dll']
};

function checkPlatformFiles() {
    console.log('🔍 Checking prebuilt binaries...\n');

    let allValid = true;

    for (const platform of SUPPORTED_PLATFORMS) {
        const platformDir = path.join(__dirname, '..', 'prebuilds', platform);
        const requiredFiles = REQUIRED_FILES[platform];

        console.log(`📦 Platform: ${platform}`);

        if (!fs.existsSync(platformDir)) {
            console.log(`   ❌ Directory missing: ${platformDir}`);
            allValid = false;
            continue;
        }

        for (const file of requiredFiles) {
            const filePath = path.join(platformDir, file);
            if (fs.existsSync(filePath)) {
                const stats = fs.statSync(filePath);
                console.log(`   ✅ ${file} (${(stats.size / 1024).toFixed(2)} KB)`);
            } else {
                console.log(`   ❌ Missing: ${file}`);
                allValid = false;
            }
        }
        console.log();
    }

    return allValid;
}

function testLoadAddon() {
    console.log('🧪 Testing addon loading...\n');

    try {
        // Try to load the platform loader
        const { getPlatformInfo, loadNativeAddon } = require('../dist/platform-loader');

        console.log('📋 Platform info:', getPlatformInfo());

        // Try to load the addon
        const addon = loadNativeAddon();
        console.log('✅ Addon loaded successfully');
        console.log('📤 Exported classes:', Object.keys(addon));

        return true;
    } catch (error) {
        console.log('❌ Addon loading failed:', error.message);
        return false;
    }
}

function main() {
    const command = process.argv[2];

    console.log('🚀 RocketMQ Native SDK Build Verification\n');

    switch (command) {
        case 'check':
            const filesValid = checkPlatformFiles();
            const addonValid = testLoadAddon();

            console.log('\n📊 Summary:');
            console.log(`   Files check: ${filesValid ? '✅ PASS' : '❌ FAIL'}`);
            console.log(`   Addon test: ${addonValid ? '✅ PASS' : '❌ FAIL'}`);

            if (filesValid && addonValid) {
                console.log('\n🎉 All checks PASSED!');
                process.exit(0);
            } else {
                console.log('\n💥 Some checks FAILED!');
                process.exit(1);
            }
            break;

        case 'list':
            checkPlatformFiles();
            break;

        default:
            console.log('Usage: node build-all-platforms.js <command>');
            console.log('Commands:');
            console.log('  check  - Verify all platform files and test addon loading');
            console.log('  list   - List all platform files');
            process.exit(1);
    }
}

if (require.main === module) {
    main();
}

module.exports = {
    checkPlatformFiles,
    testLoadAddon,
    SUPPORTED_PLATFORMS,
    REQUIRED_FILES
}; 