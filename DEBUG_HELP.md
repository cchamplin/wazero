# WebAssembly & Component Model Debugging Guide

This document describes the tooling and reference repositories available for debugging WebAssembly and Component Model issues in this project.

## Environment Setup

Before using the tools, ensure your environment is configured:

```bash
source "$HOME/.cargo/env"
export PATH="$HOME/.wasmtime/bin:$PATH"
```

## Installed Tools

### Rust & Cargo (v1.92.0)

WebAssembly compilation targets installed:
- `wasm32-unknown-unknown` - Core WebAssembly
- `wasm32-wasip1` - WASI Preview 1
- `wasm32-wasip2` - WASI Preview 2 (Component Model)

### wasm-tools (v1.244.0)

Swiss army knife for WebAssembly manipulation.

```bash
# Parse and validate a wasm module
wasm-tools validate module.wasm

# Print wasm in WAT format
wasm-tools print module.wasm

# Parse a component and show its structure
wasm-tools component wit component.wasm

# Dump raw wasm structure
wasm-tools dump module.wasm

# Convert WAT to wasm
wasm-tools parse module.wat -o module.wasm

# Component model operations
wasm-tools component new core.wasm -o component.wasm
wasm-tools component embed --wit world.wit core.wasm -o embedded.wasm

# Inspect component imports/exports
wasm-tools component wit component.wasm

# Demangle wasm names
wasm-tools demangle module.wasm

# Strip debug info
wasm-tools strip module.wasm -o stripped.wasm

# Show component model metadata
wasm-tools metadata show component.wasm
```

### cargo-component (v0.21.1)

Build Rust projects as WebAssembly components.

```bash
# Create a new component project
cargo component new my-component

# Build a component
cargo component build --release

# Build targeting wasip2
cargo component build --target wasm32-wasip2
```

### wit-bindgen (v0.51.0)

Generate language bindings from WIT definitions.

```bash
# Generate Rust bindings
wit-bindgen rust ./wit

# Generate C bindings
wit-bindgen c ./wit

# Generate Go bindings
wit-bindgen go ./wit

# Generate markdown documentation
wit-bindgen markdown ./wit
```

### wasmtime (v41.0.0)

WebAssembly runtime for running and debugging modules/components.

```bash
# Run a wasm module
wasmtime run module.wasm

# Run a component
wasmtime run component.wasm

# Run with WASI
wasmtime run --dir . module.wasm

# Compile ahead-of-time
wasmtime compile module.wasm -o module.cwasm

# Inspect a module
wasmtime explore module.wasm

# Run with fuel limit (for debugging infinite loops)
wasmtime run --fuel 10000 module.wasm

# Enable debug output
WASMTIME_LOG=debug wasmtime run module.wasm
```

## Reference Repositories (debug-vendored/)

Git submodules with source code for quick reference:

| Directory | Repository | Contents |
|-----------|------------|----------|
| `component-model/` | WebAssembly/component-model | Component model specification, canonical ABI docs |
| `wasmtime/` | bytecodealliance/wasmtime | Wasmtime runtime implementation |
| `wasm-tools/` | bytecodealliance/wasm-tools | wasmparser, wit-parser, wasm-encoder, wit-component |
| `WASI/` | WebAssembly/WASI | WASI specification (wasip1, wasip2) |
| `wit-bindgen/` | bytecodealliance/wit-bindgen | WIT bindings generator for multiple languages |
| `cargo-component/` | bytecodealliance/cargo-component | Cargo integration for components |
| `wac/` | bytecodealliance/wac | WebAssembly component composition tool |

### Updating Submodules

```bash
# Initialize submodules (first time)
git submodule update --init --recursive

# Update all submodules to latest
git submodule update --remote

# Update a specific submodule
git submodule update --remote debug-vendored/wasmtime
```

### Key Reference Files

**Component Model Spec:**
- `debug-vendored/component-model/design/mvp/Explainer.md` - Component model overview
- `debug-vendored/component-model/design/mvp/CanonicalABI.md` - Canonical ABI specification
- `debug-vendored/component-model/design/mvp/WIT.md` - WIT language spec

**WASI Specification:**
- `debug-vendored/WASI/wasip2/` - WASI Preview 2 interfaces
- `debug-vendored/WASI/wasip2/cli/` - CLI world definition
- `debug-vendored/WASI/wasip2/http/` - HTTP world definition

**wasmtime Implementation:**
- `debug-vendored/wasmtime/crates/wasmtime/` - Core runtime
- `debug-vendored/wasmtime/crates/wasi/` - WASI implementation
- `debug-vendored/wasmtime/crates/component-macro/` - Component macros

**wasm-tools Crates:**
- `debug-vendored/wasm-tools/crates/wasmparser/` - WebAssembly binary parser
- `debug-vendored/wasm-tools/crates/wit-parser/` - WIT file parser
- `debug-vendored/wasm-tools/crates/wit-component/` - Component encoding/decoding
- `debug-vendored/wasm-tools/crates/wasm-encoder/` - WebAssembly binary encoder

## Common Debugging Tasks

### Inspect Component Structure

```bash
# Show WIT interface
wasm-tools component wit component.wasm

# Show full component structure
wasm-tools print component.wasm | head -100

# Dump binary structure
wasm-tools dump component.wasm | less
```

### Validate Component

```bash
# Basic validation
wasm-tools validate component.wasm

# Validate with features
wasm-tools validate --features component-model component.wasm
```

### Compare Components

```bash
# Diff WIT interfaces
diff <(wasm-tools component wit a.wasm) <(wasm-tools component wit b.wasm)
```

### Debug Canonical ABI Issues

```bash
# Print component with canonical ABI details
wasm-tools print --print-offsets component.wasm

# Check component metadata
wasm-tools metadata show component.wasm
```

### Extract Core Module from Component

```bash
# List modules in component
wasm-tools print component.wasm | grep "core module"

# Use wasm-tools to inspect inner modules
wasm-tools dump component.wasm
```

### Run with Detailed Tracing

```bash
# Wasmtime with logging
WASMTIME_LOG=wasmtime_wasi=trace wasmtime run component.wasm

# Wasmtime with call tracing
wasmtime run --wasm call-tracing component.wasm
```

## Useful Environment Variables

```bash
# Enable wasmtime debug logging
export WASMTIME_LOG=debug

# Rust backtrace for panics
export RUST_BACKTRACE=1

# wasm-tools verbose output
export WASM_TOOLS_LOG=debug
```

## Building Test Components

### Minimal Rust Component (wasip2)

```bash
# Create new component
cargo component new test-component
cd test-component

# Edit src/lib.rs and wit/world.wit as needed

# Build
cargo component build --release

# Output will be in target/wasm32-wasip2/release/test_component.wasm
```

### Convert Core Module to Component

```bash
# Embed WIT into core module
wasm-tools component embed --wit world.wit core.wasm -o embedded.wasm

# Create component from embedded module
wasm-tools component new embedded.wasm -o component.wasm
```
