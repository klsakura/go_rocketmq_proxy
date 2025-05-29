const fs = require('fs');
const path = require('path');

// Fix ESM exports
const esmFile = path.join(__dirname, '../dist/index.mjs');
if (fs.existsSync(esmFile)) {
    let content = fs.readFileSync(esmFile, 'utf8');

    // 修复 platform-loader 导入路径，添加 .js 扩展名
    content = content.replace(
        /const { loadNativeAddon } = await import\('\.\/platform-loader'\);/g,
        "const { loadNativeAddon } = await import('./platform-loader.js');"
    );

    // 修复其他可能的导入路径
    content = content.replace(
        /await import\('([^']+)'\)/g,
        (match, importPath) => {
            // 如果是相对路径且没有扩展名，添加 .js
            if (importPath.startsWith('./') && !importPath.includes('.')) {
                return `await import('${importPath}.js')`;
            }
            return match;
        }
    );

    fs.writeFileSync(esmFile, content);
    console.log('✅ Fixed ESM exports in index.mjs');
}

// Copy platform-loader.js
const platformLoader = path.join(__dirname, '../src/platform-loader.js');
const distPlatformLoader = path.join(__dirname, '../dist/platform-loader.js');
if (fs.existsSync(platformLoader)) {
    fs.copyFileSync(platformLoader, distPlatformLoader);
    console.log('✅ Copied platform-loader.js to dist/');
} 