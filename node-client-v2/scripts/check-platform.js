#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const os = require('os');

function getPlatformInfo() {
    const platform = os.platform();
    const arch = os.arch();
    const platformKey = `${platform}-${arch}`;
    return { platform, arch, platformKey };
}

function checkPlatformBinaries() {
    console.log('🎯 RocketMQ Native SDK - Platform Check');
    console.log('==================================================');

    const { platform, arch, platformKey } = getPlatformInfo();
    console.log(`🔍 Platform: ${platform} ${arch} (${platformKey})`);

    // Check prebuilt binaries first
    const prebuildsDir = path.join(__dirname, '..', 'prebuilds', platformKey);
    const goLibExt = platform === 'win32' ? '.dll' : platform === 'darwin' ? '.dylib' : '.so';
    const goLibPath = path.join(prebuildsDir, `librocketmq_cgo${goLibExt}`);
    const addonPath = path.join(prebuildsDir, 'rocketmq_addon.node');

    let hasPrebuilts = false;
    if (fs.existsSync(goLibPath) && fs.existsSync(addonPath)) {
        console.log('✅ Prebuilt binaries found');
        console.log(`   Go library: ${goLibPath}`);
        console.log(`   Node addon: ${addonPath}`);
        hasPrebuilts = true;
    }

    // Check local build
    const localGoLib = path.join(__dirname, '..', '..', 'cgo', `librocketmq_cgo${goLibExt}`);
    const localAddon = path.join(__dirname, '..', '..', 'addon', 'build', 'Release', 'rocketmq_addon.node');

    let hasLocalBuild = false;
    if (fs.existsSync(localGoLib) && fs.existsSync(localAddon)) {
        console.log('✅ Local build found');
        console.log(`   Go library: ${localGoLib}`);
        console.log(`   Node addon: ${localAddon}`);
        hasLocalBuild = true;
    }

    if (hasPrebuilts || hasLocalBuild) {
        console.log('🎉 Native components are available!');
        return true;
    }

    console.log('⚠️  No native components found');
    console.log('');
    console.log('🔧 Build Instructions:');
    console.log('==================================================');
    console.log('📋 System Requirements:');
    console.log('- Go 1.21+ (for CGO shared library)');
    console.log('- Node.js 12+ (for Native Addon)');
    console.log('- C++ compiler:');
    console.log('  • GCC/G++: sudo apt-get install build-essential (Ubuntu/Debian)');
    console.log('  •          sudo yum groupinstall "Development Tools" (CentOS/RHEL)');
    console.log('');
    console.log('🚀 Build Commands:');
    console.log('npm run build:all');
    console.log('');
    console.log('Or step by step:');
    console.log('npm run build:go     # Build Go shared library');
    console.log('npm run build:addon  # Build C++ Native Addon');
    console.log('npm run build        # Build TypeScript SDK');
    console.log('');
    console.log('✅ Verify Build:');
    console.log('npm run example:health');

    return false;
}

// Only run if called directly (not required)
if (require.main === module) {
    const hasNativeComponents = checkPlatformBinaries();
    // Don't exit with error code - just inform
    process.exit(0);
}

module.exports = { checkPlatformBinaries, getPlatformInfo }; 