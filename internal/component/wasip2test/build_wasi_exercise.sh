#!/bin/bash
# build_wasi_exercise.sh - Build the wasi-exercise integration test components.
#
# This script builds Rust and Go WASM components that exercise wazero's
# WASI Preview 2 host implementations end-to-end. The output .wasm files are
# placed in testdata/ for use by wasi_exercise_test.go.
#
# Prerequisites for the Rust component:
#   - rustc with wasm32-wasip1 target: rustup target add wasm32-wasip1
#   - wasm-tools: cargo install wasm-tools (or download a release)
#
# Prerequisites for the Go component:
#   - tinygo
#   - wit-bindgen-go (go install github.com/bytecodealliance/wit-bindgen-go/cmd/wit-bindgen-go@latest)
#
# Note: cargo-component is NOT required. Components are produced by building
# a core wasm cdylib with cargo, embedding the WIT metadata via
# `wasm-tools component embed`, then converting to a component via
# `wasm-tools component new`.
#
# This script attempts to build each component independently and reports any
# missing tools without failing the whole script (so partial builds work).

set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
TESTDATA_DIR="$SCRIPT_DIR/testdata"
mkdir -p "$TESTDATA_DIR"

build_rust=0
build_go=0

# ----- Rust component -----
RUST_DIR="$SCRIPT_DIR/testcomponents/wasi-exercise-rust"
ADAPTER="$TESTDATA_DIR/wasi_snapshot_preview1.reactor.wasm"
if command -v cargo >/dev/null 2>&1 && command -v wasm-tools >/dev/null 2>&1; then
    echo "==> Building wasi-exercise-rust"
    if [ ! -f "$ADAPTER" ]; then
        echo "ERROR: WASI Preview 1 adapter not found at $ADAPTER"
    elif (cd "$RUST_DIR" && cargo build --release --target wasm32-wasip1); then
        CORE_WASM="$RUST_DIR/target/wasm32-wasip1/release/wasi_exercise_rust.wasm"
        if [ ! -f "$CORE_WASM" ]; then
            echo "ERROR: expected core wasm at $CORE_WASM not found"
        else
            EMBEDDED="$RUST_DIR/target/embedded.wasm"
            COMPONENT="$TESTDATA_DIR/wasi-exercise-rust.wasm"
            # Embed component metadata into the core module, then convert to a
            # component, supplying the WASI P1 adapter so that Rust stdlib's
            # wasi_snapshot_preview1 imports are translated into P2 calls.
            if wasm-tools component embed "$RUST_DIR/wit" "$CORE_WASM" -o "$EMBEDDED" \
               && wasm-tools component new "$EMBEDDED" --adapt "$ADAPTER" -o "$COMPONENT"; then
                echo "    built $COMPONENT"
                build_rust=1
            else
                echo "ERROR: wasm-tools embed/new failed for Rust component"
            fi
        fi
    else
        echo "ERROR: cargo build failed for Rust component"
    fi
else
    echo "==> Skipping wasi-exercise-rust (cargo and/or wasm-tools missing)"
fi

# ----- Go component (TinyGo) -----
GO_DIR="$SCRIPT_DIR/testcomponents/wasi-exercise-go"
if [ -d "$GO_DIR" ] && command -v tinygo >/dev/null 2>&1 && command -v wit-bindgen-go >/dev/null 2>&1; then
    echo "==> Building wasi-exercise-go"
    if (cd "$GO_DIR" && bash build.sh); then
        if [ -f "$TESTDATA_DIR/wasi-exercise-go.wasm" ]; then
            echo "    built $TESTDATA_DIR/wasi-exercise-go.wasm"
            build_go=1
        fi
    else
        echo "ERROR: build.sh failed for Go component"
    fi
else
    echo "==> Skipping wasi-exercise-go (tinygo, wit-bindgen-go, or directory missing)"
fi

echo ""
echo "==> Build summary: rust=$build_rust go=$build_go"
exit 0
