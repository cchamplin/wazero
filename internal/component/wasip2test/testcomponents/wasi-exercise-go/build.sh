#!/bin/bash
# Build the wasi-exercise-go component.
#
# Uses the standard Go toolchain with GOOS=wasip1 -buildmode=c-shared to
# produce a core wasm module, then embeds the WIT exercise world and
# converts to a component using the WASI P1 reactor adapter.
#
# Generated bindings (wit_exports.go and wasi_*/) come from `wit-bindgen go`
# and are produced on demand if missing.
#
# Outputs ../../testdata/wasi-exercise-go.wasm.
set -eu

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
TESTDATA_DIR="$SCRIPT_DIR/../../testdata"
ADAPTER="$TESTDATA_DIR/wasi_snapshot_preview1.reactor.wasm"

cd "$SCRIPT_DIR"

if [ ! -f "$ADAPTER" ]; then
    echo "ERROR: WASI Preview 1 reactor adapter not found at $ADAPTER" >&2
    exit 1
fi

# Regenerate bindings if missing or out of date.
if [ ! -f wit_exports.go ] || [ wit/exercise.wit -nt wit_exports.go ]; then
    echo "Generating Go bindings via wit-bindgen..."
    wit-bindgen go -w exercise wit/
fi

echo "Building Go core wasm (wasip1, c-shared)..."
rm -f core.wasm core-with-wit.wasm component.wasm
GOARCH=wasm GOOS=wasip1 go build -o core.wasm -buildmode=c-shared -ldflags=-checklinkname=0 .

echo "Embedding WIT exercise world..."
wasm-tools component embed -w exercise wit/ core.wasm -o core-with-wit.wasm

echo "Converting to component with WASI P1 adapter..."
wasm-tools component new --adapt "$ADAPTER" core-with-wit.wasm -o component.wasm

mv component.wasm "$TESTDATA_DIR/wasi-exercise-go.wasm"
rm -f core.wasm core-with-wit.wasm
echo "Built $TESTDATA_DIR/wasi-exercise-go.wasm"
