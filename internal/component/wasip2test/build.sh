#!/bin/bash
# build.sh - Build all calculator plugins
#
# Prerequisites:
#   - cargo-component: cargo install cargo-component
#   - wasi-sdk: Download from https://github.com/WebAssembly/wasi-sdk/releases
#   - wit-bindgen: cargo install wit-bindgen-cli
#
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PLUGINS_DIR="$SCRIPT_DIR/plugins"

mkdir -p "$PLUGINS_DIR"

echo "Building Rust add plugin..."
cd "$SCRIPT_DIR/rust-plugin"
cargo component build --release
cp target/wasm32-wasip1/release/add_plugin.wasm "$PLUGINS_DIR/add.wasm"

echo "Building C subtract plugin..."
cd "$SCRIPT_DIR/c-plugin"
# Generate C bindings if not present
if [ ! -f plugin.h ]; then
    wit-bindgen c ../wit --out-dir .
fi
make subtract.wasm
cp subtract.wasm "$PLUGINS_DIR/subtract.wasm"

echo "Done! Plugins built:"
ls -la "$PLUGINS_DIR"
