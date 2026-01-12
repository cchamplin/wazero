// Package binary provides constants for WebAssembly Component Model binary format.
package binary

// Magic is the 4-byte preamble for all WebAssembly binaries (modules and components).
// See https://github.com/WebAssembly/component-model/blob/main/design/mvp/Binary.md
var Magic = [4]byte{0x00, 0x61, 0x73, 0x6d} // "\0asm"

// Version is the component model version (pre-standard).
// This will change to [4]byte{0x01, 0x00, 0x01, 0x00} at 1.0.
var Version = [2]byte{0x0d, 0x00}

// LayerComponent identifies a binary as a component (vs core module).
var LayerComponent = [2]byte{0x01, 0x00}

// LayerModule identifies a binary as a core module (vs component).
var LayerModule = [2]byte{0x00, 0x00}
