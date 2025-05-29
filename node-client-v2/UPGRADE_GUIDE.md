# Upgrade Guide: Node.js 16+ Migration

## Overview

RocketMQ Native SDK v2.0+ now requires **Node.js 16.0.0 or higher**. This upgrade brings significant performance improvements, better platform support, and access to modern JavaScript features.

## 🚀 Why Upgrade?

### Performance Improvements
- **Up to 20% faster execution** thanks to V8 engine improvements
- **Better memory management** with enhanced garbage collection
- **Improved native addon performance** with Node-API optimizations

### Platform Support
- **Full Apple Silicon support** for M1/M2 Macs
- **Better Windows compatibility** with modern toolchain
- **Enhanced Linux support** with updated dependencies

### Modern Features
- **Native ESM support** for better module handling
- **Advanced error handling** with better stack traces
- **WebAssembly integration** for future enhancements

## 📋 Migration Checklist

### 1. Check Current Node.js Version
```bash
node --version
```

### 2. Upgrade Node.js

#### Option A: Using nvm (Recommended)
```bash
# Install latest LTS
nvm install --lts
nvm use --lts

# Or install specific version
nvm install 16
nvm use 16
```

#### Option B: Direct Download
- Visit [nodejs.org](https://nodejs.org/)
- Download Node.js 18 LTS (recommended)
- Follow installation instructions

### 3. Update Your Project

#### Update package.json
```json
{
  "engines": {
    "node": ">=16.0.0"
  }
}
```

#### Update CI/CD (if applicable)
```yaml
# GitHub Actions
- uses: actions/setup-node@v4
  with:
    node-version: '18'  # or '16', '20', '22'
```

### 4. Reinstall Dependencies
```bash
# Clear node_modules and package-lock.json
rm -rf node_modules package-lock.json

# Reinstall with new Node.js version
npm install
```

### 5. Test Your Application
```bash
# Run your tests
npm test

# Test RocketMQ functionality
npm run example:health
npm run example:producer
npm run example:consumer
```

## 🔧 Breaking Changes

### Minimum Node.js Version
- **Before**: Node.js 14.0.0+
- **After**: Node.js 16.0.0+

### Dependencies
- **TypeScript**: Updated to 5.0+ for better performance
- **@types/node**: Updated to 16.18+ for accurate type definitions

### Features
- Enhanced error messages with better context
- Improved platform detection for Apple Silicon
- Better ESM/CommonJS interoperability

## 🐛 Common Issues & Solutions

### Issue: "Unsupported Node.js version"
```bash
Error: Node.js version 14.x.x is not supported.
Minimum required version: 16.0.0
```

**Solution**: Upgrade Node.js to 16+ following the steps above.

### Issue: Native addon rebuild required
```bash
Error: Module version mismatch
```

**Solution**: Rebuild native modules
```bash
npm rebuild @klsakura/rocketmq-native-sdk
```

### Issue: Package installation fails
**Solution**: Clear cache and reinstall
```bash
npm cache clean --force
rm -rf node_modules package-lock.json
npm install
```

## ✅ Verification Steps

### 1. Version Check
```bash
node --version  # Should show 16.0.0 or higher
npm --version   # Should be compatible
```

### 2. SDK Installation Test
```bash
npm install @klsakura/rocketmq-native-sdk
```

### 3. Runtime Test
```javascript
const sdk = require('@klsakura/rocketmq-native-sdk');
console.log('✅ SDK loaded successfully');
```

### 4. Platform Detection Test
```bash
node -e "
const { getPlatformInfo } = require('@klsakura/rocketmq-native-sdk/dist/platform-loader');
console.log('Platform:', getPlatformInfo());
"
```

## 🔄 Rollback Plan

If you encounter issues, you can temporarily rollback:

### 1. Use Previous SDK Version
```bash
npm install @klsakura/rocketmq-native-sdk@1.x
```

### 2. Use Node.js 14 with Compatibility Mode
```bash
# Only for testing - not recommended for production
nvm use 14
npm install @klsakura/rocketmq-native-sdk@1.x
```

## 📞 Support

If you encounter issues during migration:

1. **Check Documentation**: Review updated README and compatibility guide
2. **Search Issues**: Look for similar problems in GitHub issues
3. **Create Issue**: Report new bugs with detailed information
4. **Community**: Ask questions in discussions

## 🎯 Benefits After Migration

### Performance
- **Faster startup times** due to V8 improvements
- **Better memory usage** with enhanced GC
- **Improved native addon performance**

### Development Experience
- **Better error messages** with clearer stack traces
- **Enhanced debugging** with improved tooling
- **Modern JavaScript features** available

### Platform Support
- **Apple Silicon native support** for M1/M2 Macs
- **Better Windows compatibility**
- **Enhanced cross-platform consistency**

### Future-Proofing
- **Long-term support** with active Node.js versions
- **Security updates** with regular patches
- **Ecosystem compatibility** with modern packages

---

**Need Help?** Contact our support team or create an issue on GitHub for assistance with your migration. 