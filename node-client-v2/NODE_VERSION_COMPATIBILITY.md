# Node.js Version Compatibility

## Minimum Requirements

**Minimum Node.js Version: 14.0.0**

This SDK is designed to support Node.js 14+ with the following compatibility considerations:

## ✅ What's Compatible

### Runtime Requirements
- **Node.js 14.0.0+** - Minimum supported version
- **Node.js 16.x** - Full support  
- **Node.js 18.x** - Full support
- **Node.js 20.x** - Full support

### Features Used
- **ES2018** - TypeScript compilation target
- **CommonJS/ESM** - Dual module support
- **Native Addons** - Node-API/N-API based
- **Standard Node.js APIs** - `os`, `path`, `fs`, `require()`

## 🔧 Technical Details

### TypeScript Compilation
```json
{
  "target": "es2018",    // Compatible with Node.js 14+
  "module": "commonjs"   // Primary module format
}
```

### Native Addon Compatibility
- Built with **Node-API (N-API)** for forward compatibility
- Precompiled binaries for major Node.js versions
- Automatic platform detection and loading

### Module System
- **CommonJS**: `require()` - Primary support
- **ESM**: `import` - Modern support via `.mjs`

## 🚨 Known Limitations

### Node.js 12.x Specific
1. **Native Module Version**: May need rebuild for very old Node.js 12 versions
2. **ESM Support**: Limited in Node.js 12, prefer CommonJS

### Recommendations
- **Node.js 14+**: Minimum requirement
- **Node.js 16+**: Best experience with full ESM support

## 📦 Installation

### For Node.js 14+
```bash
npm install @klsakura/rocketmq-native-sdk
```

### Usage Examples

#### CommonJS (Node.js 14+)
```javascript
const { Producer, Consumer } = require('@klsakura/rocketmq-native-sdk');
```

#### ESM (Node.js 14+)
```javascript
import { Producer, Consumer } from '@klsakura/rocketmq-native-sdk';
```

## 🔍 Version Checking

You can check compatibility at runtime:

```javascript
const nodeVersion = process.versions.node;
const majorVersion = parseInt(nodeVersion.split('.')[0]);

if (majorVersion < 14) {
    console.error('Node.js 14+ required');
    process.exit(1);
}

console.log(`✅ Node.js ${nodeVersion} is supported`);
```

## 🛠️ Development

### Building from Source
- **Node.js 14+**: Required for development
- **Python 3.x**: Required for native compilation
- **C++ Compiler**: Platform-specific requirements

### Testing Compatibility
```bash
# Test with different Node.js versions using nvm
nvm use 14
npm test

nvm use 16
npm test

nvm use 18
npm test
```

## 📊 Platform Support Matrix

| Platform       | Node.js 14 | Node.js 16 | Node.js 18 | Node.js 20 |
|---------------|------------|------------|------------|------------|
| macOS arm64   | ✅         | ✅         | ✅         | ✅         |
| macOS x64     | ✅         | ✅         | ✅         | ✅         |
| Linux x64     | ✅         | ✅         | ✅         | ✅         |
| Windows x64   | ✅         | ✅         | ✅         | ✅         |

## 🐛 Troubleshooting

### "Module version mismatch" Error
```bash
# Rebuild for your Node.js version
npm rebuild @klsakura/rocketmq-native-sdk
```

### Platform Detection Issues
```javascript
const { getPlatformInfo } = require('@klsakura/rocketmq-native-sdk/dist/platform-loader');
console.log('Platform:', getPlatformInfo());
``` 