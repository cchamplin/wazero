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
	"github.com/tetratelabs/wazero/imports/wasip2/cli"
	"github.com/tetratelabs/wazero/imports/wasip2/clocks"
	"github.com/tetratelabs/wazero/imports/wasip2/filesystem"
	wasip2http "github.com/tetratelabs/wazero/imports/wasip2/http"
	wasip2io "github.com/tetratelabs/wazero/imports/wasip2/io"
	"github.com/tetratelabs/wazero/imports/wasip2/random"
	"github.com/tetratelabs/wazero/imports/wasip2/sockets"
	"github.com/tetratelabs/wazero/internal/component"
)

// Instantiate registers all WASI Preview 2 interfaces with the linker.
// Uses default configuration backed by os package.
func Instantiate(linker *component.Linker) error {
	return InstantiateWithConfig(linker, DefaultConfig())
}

// InstantiateWithConfig registers all WASI Preview 2 interfaces with custom configuration.
func InstantiateWithConfig(linker *component.Linker, config *Config) error {
	if err := wasip2io.Instantiate(linker); err != nil {
		return err
	}
	if err := clocks.Instantiate(linker); err != nil {
		return err
	}
	if err := random.Instantiate(linker); err != nil {
		return err
	}
	if err := cli.Instantiate(linker); err != nil {
		return err
	}
	if err := filesystem.Instantiate(linker); err != nil {
		return err
	}
	if err := sockets.Instantiate(linker); err != nil {
		return err
	}
	if err := wasip2http.Instantiate(linker); err != nil {
		return err
	}
	return nil
}
