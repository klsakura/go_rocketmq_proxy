const fs = require('fs');
const path = require('path');

// 修复ESM导出
function fixESMExports() {
    const mjsPath = path.join(__dirname, '../dist/index.mjs');

    if (fs.existsSync(mjsPath)) {
        let content = fs.readFileSync(mjsPath, 'utf8');

        // 替换require为import
        content = content.replace(/const { loadNativeAddon } = require\('\.\/platform-loader'\);/g,
            "import { loadNativeAddon } from './platform-loader.js';");

        fs.writeFileSync(mjsPath, content);
        console.log('✅ Fixed ESM exports in index.mjs');
    }
}

// 复制platform-loader到dist目录
function copyPlatformLoader() {
    const srcPath = path.join(__dirname, '../src/platform-loader.js');
    const distPath = path.join(__dirname, '../dist/platform-loader.js');

    if (fs.existsSync(srcPath)) {
        fs.copyFileSync(srcPath, distPath);
        console.log('✅ Copied platform-loader.js to dist/');
    }
}

// 主函数
function main() {
    fixESMExports();
    copyPlatformLoader();
}

if (require.main === module) {
    main();
}

module.exports = { fixESMExports, copyPlatformLoader };