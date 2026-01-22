#!/bin/bash
# build_test_components.sh - Build service and middleware test components for composition testing
#
# Prerequisites:
#   - cargo-component: cargo install cargo-component
#   - Rust wasm targets: rustup target add wasm32-wasip1
#
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
TESTCOMPONENTS_DIR="$SCRIPT_DIR/testcomponents"
TESTDATA_DIR="$SCRIPT_DIR/testdata"

# Create output directory
mkdir -p "$TESTDATA_DIR"

echo "=== Building test components for wasm-compose testing ==="
echo ""

# Check for cargo-component
if ! command -v cargo-component &> /dev/null; then
    echo "ERROR: cargo-component is not installed"
    echo "Install with: cargo install cargo-component"
    exit 1
fi

echo "Using cargo-component version: $(cargo component --version)"
echo ""

# Build service component
echo "Building service component..."
cd "$TESTCOMPONENTS_DIR/service"
if cargo component build --release; then
    # Find the output wasm file
    WASM_FILE=$(find target -name "service.wasm" -path "*/release/*" 2>/dev/null | head -1)
    if [ -z "$WASM_FILE" ]; then
        # Try alternative naming
        WASM_FILE=$(find target -name "*.wasm" -path "*/release/*" 2>/dev/null | head -1)
    fi
    if [ -n "$WASM_FILE" ]; then
        cp "$WASM_FILE" "$TESTDATA_DIR/service.wasm"
        echo "  -> Copied to testdata/service.wasm"
    else
        echo "ERROR: Could not find built service.wasm"
        exit 1
    fi
else
    echo "ERROR: Failed to build service component"
    exit 1
fi
echo ""

# Build middleware component
echo "Building middleware component..."
cd "$TESTCOMPONENTS_DIR/middleware"
if cargo component build --release; then
    # Find the output wasm file
    WASM_FILE=$(find target -name "middleware.wasm" -path "*/release/*" 2>/dev/null | head -1)
    if [ -z "$WASM_FILE" ]; then
        # Try alternative naming
        WASM_FILE=$(find target -name "*.wasm" -path "*/release/*" 2>/dev/null | head -1)
    fi
    if [ -n "$WASM_FILE" ]; then
        cp "$WASM_FILE" "$TESTDATA_DIR/middleware.wasm"
        echo "  -> Copied to testdata/middleware.wasm"
    else
        echo "ERROR: Could not find built middleware.wasm"
        exit 1
    fi
else
    echo "ERROR: Failed to build middleware component"
    exit 1
fi
echo ""

echo "=== Build complete ==="
echo ""
echo "Output files:"
ls -la "$TESTDATA_DIR"/*.wasm 2>/dev/null || echo "No wasm files found in testdata/"
