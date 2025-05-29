const fs = require('fs');
const path = require('path');

// Fix ESM exports
const esmFile = path.join(__dirname, '../dist/index.mjs');
if (fs.existsSync(esmFile)) {
    let content = fs.readFileSync(esmFile, 'utf8');
    content = content.replace(/require\(/g, 'await import(').replace(/\);$/gm, ');');
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