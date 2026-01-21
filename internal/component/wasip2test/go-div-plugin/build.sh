#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "Building Go division plugin with TinyGo (wasip2)..."

# Clean previous build artifacts
rm -f component.wasm

# Rebuild WIT package if needed
if [ ! -f "docs:calculator@0.1.0.wasm" ] || [ "wit/world.wit" -nt "docs:calculator@0.1.0.wasm" ]; then
    echo "  Building WIT package..."
    wkg wit build -o "docs:calculator@0.1.0.wasm"
fi

# Regenerate bindings if needed
if [ ! -d "gen" ] || [ "docs:calculator@0.1.0.wasm" -nt "gen" ]; then
    echo "  Generating Go bindings..."
    rm -rf gen
    wit-bindgen-go generate --world plugin --out gen "./docs:calculator@0.1.0.wasm"
fi

# Build with TinyGo
echo "  Compiling with TinyGo..."
GOTOOLCHAIN=local tinygo build \
    -target=wasip2 \
    -o component.wasm \
    --wit-package "docs:calculator@0.1.0.wasm" \
    --wit-world plugin \
    .

echo "Go division plugin built: component.wasm ($(stat -c%s component.wasm 2>/dev/null || stat -f%z component.wasm) bytes)"

# Copy to plugins directory
cp component.wasm ../plugins/div.wasm
echo "Copied to plugins/div.wasm"
