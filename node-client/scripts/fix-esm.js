const fs = require('fs');
const path = require('path');

// 读取 ESM 文件
const esmPath = path.join(__dirname, '../dist/index.mjs');
let content = fs.readFileSync(esmPath, 'utf8');

// 替换 __dirname 为 ES 模块兼容的代码
const esmDirnameFix = `
import { fileURLToPath } from 'url';
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
`;

// 在文件开头添加 ES 模块的 __dirname 定义
content = content.replace(
    /import \* as path from 'path';/,
    `import * as path from 'path';
import { fileURLToPath } from 'url';
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);`
);

// 写回文件
fs.writeFileSync(esmPath, content);

console.log('✅ ESM module fixed for __dirname compatibility'); 