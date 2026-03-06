#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Regenerate bindings
wit-bindgen go -w test-plugin "./test:repro.wasm"
go get github.com/bytecodealliance/wit-bindgen

# Clean
rm -f core.wasm embedded.wasm component.wasm

# Step 1: Compile to core WASM
echo "Compiling core module..."
GOARCH=wasm GOOS=wasip1 go build -o core.wasm -buildmode=c-shared -ldflags=-checklinkname=0

# Step 2: Embed WIT metadata
echo "Embedding WIT..."
wasm-tools component embed -w test-plugin ./wit core.wasm -o embedded.wasm

# Step 3: Convert to component
echo "Converting to component..."
wasm-tools component new embedded.wasm -o component.wasm --adapt ../testdata/wasi_snapshot_preview1.reactor.wasm

rm -f embedded.wasm
echo "Built: component.wasm ($(stat -c%s component.wasm 2>/dev/null || stat -f%z component.wasm) bytes)"
