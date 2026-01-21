#!/usr/bin/env bash

wit-bindgen go -w plugin ../wit/
go get github.com/bytecodealliance/wit-bindgen
rm -f core.wasm core-with-wit.wasm component.wasm
echo "Building Go calculator plugin..."
GOARCH="wasm" GOOS="wasip1" go build -o core.wasm -buildmode=c-shared -ldflags=-checklinkname=0
echo "Embedding WIT interface into core.wasm..."
wasm-tools component embed -w plugin ../wit/calculator.wit core.wasm -o core-with-wit.wasm
echo "Adapting to component model with WASI Snapshot Preview 1 reactor interface..."
wasm-tools component new --adapt wasi_snapshot_preview1.reactor.wasm core-with-wit.wasm -o component.wasm
echo "Go calculator plugin built: component.wasm"
cp component.wasm ../plugins/multi.wasm
echo "Copying to plugins/multi.wasm"
