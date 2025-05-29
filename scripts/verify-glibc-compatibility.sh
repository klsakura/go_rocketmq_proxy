#!/bin/bash

# Verify GLIBC compatibility for RocketMQ Native SDK prebuilds
# This script checks the GLIBC requirements of the generated binaries

echo "=== GLIBC Compatibility Verification ==="
echo ""

PREBUILDS_DIR="node-client-v2/prebuilds"

if [ ! -d "$PREBUILDS_DIR" ]; then
    echo "❌ Prebuilds directory not found: $PREBUILDS_DIR"
    exit 1
fi

echo "🔍 Checking Linux x64 prebuilds..."

LINUX_PREBUILDS="$PREBUILDS_DIR/linux-x64"
if [ ! -d "$LINUX_PREBUILDS" ]; then
    echo "❌ Linux x64 prebuilds not found: $LINUX_PREBUILDS"
    exit 1
fi

# Check rocketmq_addon.node
ADDON_FILE="$LINUX_PREBUILDS/rocketmq_addon.node"
if [ -f "$ADDON_FILE" ]; then
    echo "📦 Checking rocketmq_addon.node GLIBC requirements:"
    objdump -T "$ADDON_FILE" 2>/dev/null | grep GLIBC | awk '{print $5}' | sort -V | uniq | while read version; do
        echo "  - $version"
    done
    
    # Check maximum GLIBC version required
    MAX_GLIBC=$(objdump -T "$ADDON_FILE" 2>/dev/null | grep GLIBC | awk '{print $5}' | grep -o 'GLIBC_[0-9]\+\.[0-9]\+' | sed 's/GLIBC_//' | sort -V | tail -1)
    if [ ! -z "$MAX_GLIBC" ]; then
        echo "  ✅ Maximum GLIBC version required: $MAX_GLIBC"
        if [[ $(echo "$MAX_GLIBC 2.31" | tr ' ' '\n' | sort -V | head -1) == "$MAX_GLIBC" ]]; then
            echo "  ✅ Compatible with 字节云 VCI (GLIBC 2.31)"
        else
            echo "  ❌ NOT compatible with 字节云 VCI (GLIBC 2.31)"
        fi
    fi
else
    echo "❌ rocketmq_addon.node not found: $ADDON_FILE"
fi

echo ""

# Check librocketmq_cgo.so
GO_LIB_FILE="$LINUX_PREBUILDS/librocketmq_cgo.so"
if [ -f "$GO_LIB_FILE" ]; then
    echo "📦 Checking librocketmq_cgo.so GLIBC requirements:"
    objdump -T "$GO_LIB_FILE" 2>/dev/null | grep GLIBC | awk '{print $5}' | sort -V | uniq | while read version; do
        echo "  - $version"
    done
    
    # Check maximum GLIBC version required
    MAX_GLIBC=$(objdump -T "$GO_LIB_FILE" 2>/dev/null | grep GLIBC | awk '{print $5}' | grep -o 'GLIBC_[0-9]\+\.[0-9]\+' | sed 's/GLIBC_//' | sort -V | tail -1)
    if [ ! -z "$MAX_GLIBC" ]; then
        echo "  ✅ Maximum GLIBC version required: $MAX_GLIBC"
        if [[ $(echo "$MAX_GLIBC 2.31" | tr ' ' '\n' | sort -V | head -1) == "$MAX_GLIBC" ]]; then
            echo "  ✅ Compatible with 字节云 VCI (GLIBC 2.31)"
        else
            echo "  ❌ NOT compatible with 字节云 VCI (GLIBC 2.31)"
        fi
    fi
else
    echo "❌ librocketmq_cgo.so not found: $GO_LIB_FILE"
fi

echo ""
echo "=== Build Environment Recommendations ==="
echo "✅ Use ubuntu-20.04 for Linux builds (GLIBC 2.31)"
echo "✅ Compatible with 字节云 VCI and most production environments"
echo "✅ Avoids GLIBC_2.32+ requirements from newer Ubuntu versions"
echo ""
echo "=== Verification Complete ===" 