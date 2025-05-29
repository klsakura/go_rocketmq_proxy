# Node.js Version Compatibility

## Minimum Requirements

**Minimum Node.js Version: 16.0.0**

This SDK is designed to support Node.js 16+ with modern JavaScript features and full platform compatibility.

## ✅ What's Compatible

### Runtime Requirements
- **Node.js 16.0.0+** - Minimum supported version
- **Node.js 18.x** - Recommended for best performance
- **Node.js 20.x** - Full support with latest features
- **Node.js 22.x** - Latest LTS support

### Features Used
- **ES2018** - TypeScript compilation target
- **CommonJS/ESM** - Dual module support
- **Native Addons** - Node-API/N-API based
- **Standard Node.js APIs** - `os`, `path`, `fs`, `require()`
- **Modern JavaScript** - Optional chaining, nullish coalescing, etc.

## 🔧 Technical Details

### TypeScript Compilation
```json
{
  "target": "es2018",    // Compatible with Node.js 16+
  "module": "commonjs"   // Primary module format
}
```

### Native Addon Compatibility
- Built with **Node-API (N-API)** for forward compatibility
- Precompiled binaries for major Node.js versions
- Automatic platform detection and loading

### Module System
- **CommonJS**: `require()` - Primary support
- **ESM**: `import` - Full support via `.mjs`

## 🚀 Why Node.js 16+?

### Performance Benefits
- **V8 Engine Improvements**: Better performance and memory usage
- **Worker Threads**: Better utilization for concurrent operations
- **WASM Support**: Enhanced WebAssembly integration

### Modern Features
- **Apple Silicon Support**: Full native support for M1/M2 Macs
- **Advanced Error Handling**: Better stack traces and debugging
- **ECMAScript Modules**: Native ESM support

### Security & Stability
- **Long-term Support**: Node.js 16 was LTS until September 2023
- **Security Updates**: Regular security patches
- **Ecosystem Compatibility**: Better package ecosystem support

## 📦 Installation

### For Node.js 16+
```bash
npm install @klsakura/rocketmq-native-sdk
```

### Usage Examples

#### CommonJS (Node.js 16+)
```javascript
const { Producer, Consumer } = require('@klsakura/rocketmq-native-sdk');
```

#### ESM (Node.js 16+)
```javascript
import { Producer, Consumer } from '@klsakura/rocketmq-native-sdk';
```

## 🔍 Version Checking

You can check compatibility at runtime:

```javascript
const nodeVersion = process.versions.node;
const majorVersion = parseInt(nodeVersion.split('.')[0]);

if (majorVersion < 16) {
    console.error('Node.js 16+ required');
    process.exit(1);
}

console.log(`✅ Node.js ${nodeVersion} is supported`);
```

## 🛠️ Development

### Building from Source
- **Node.js 16+**: Required for development
- **Python 3.x**: Required for native compilation
- **C++ Compiler**: Platform-specific requirements

### Testing Compatibility
```bash
# Test with different Node.js versions using nvm
nvm use 16
npm test

nvm use 18
npm test

nvm use 20
npm test
```

## 📊 Platform Support Matrix

| Platform       | Node.js 16 | Node.js 18 | Node.js 20 | Node.js 22 |
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

## 🔄 Migration from Older Versions

If you're upgrading from Node.js 14/15:

### Update Node.js
```bash
# Using nvm
nvm install 16
nvm use 16

# Or download from nodejs.org
```

### Benefits After Migration
- **Better Performance**: Up to 20% faster execution
- **Apple Silicon**: Native support for M1/M2 Macs
- **Modern Features**: Access to latest JavaScript features
- **Better Debugging**: Enhanced error handling and stack traces 