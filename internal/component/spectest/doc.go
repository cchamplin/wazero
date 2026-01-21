// Package spectest provides infrastructure for running WebAssembly Component Model
// specification tests (.wast files) using wasm-tools CLI.
//
// The package parses .wast files by converting them to JSON via wasm-tools,
// then unmarshaling into Go structs that can be used by test runners.
package spectest
