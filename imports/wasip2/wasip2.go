// Package wasip2 contains Go-defined functions to access WASI Preview 2
// interfaces from WebAssembly components. These are accessible from
// WebAssembly-defined functions via the component model.
//
// e.g. Call Instantiate before instantiating any component that imports
// WASI Preview 2 interfaces. Otherwise, it will error due to missing imports.
//
// See https://github.com/WebAssembly/WASI
package wasip2

import (
	"github.com/tetratelabs/wazero/internal/component"
)

// Instantiate registers all WASI Preview 2 interfaces with the linker.
// Uses default configuration backed by os package.
func Instantiate(linker *component.Linker) error {
	return InstantiateWithConfig(linker, DefaultConfig())
}

// InstantiateWithConfig registers all WASI Preview 2 interfaces with custom configuration.
func InstantiateWithConfig(linker *component.Linker, config *Config) error {
	// Phase 5.2-5.10 will add each interface
	// For now, register a placeholder for io/error
	return linker.DefineInstance("wasi:io/error@0.2.0").Build()
}
