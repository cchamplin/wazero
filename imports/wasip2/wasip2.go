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
	"fmt"

	"github.com/tetratelabs/wazero/api"
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

// MergeInto registers all WASI Preview 2 interfaces onto an api.ComponentLinker.
// It creates a temporary basic Linker, registers WASI P2 on it, then merges
// the definitions into the ComponentLinkerWrapper. Uses default configuration.
//
// The linker parameter must be a *component.ComponentLinkerWrapper (as returned
// by wazero.Runtime.NewComponentLinker()). Returns an error if the type assertion fails.
func MergeInto(linker api.ComponentLinker) error {
	return MergeIntoWithConfig(linker, DefaultConfig())
}

// MergeIntoWithConfig registers all WASI Preview 2 interfaces onto an api.ComponentLinker
// with custom configuration. See MergeInto for details.
func MergeIntoWithConfig(linker api.ComponentLinker, config *Config) error {
	wrapper, ok := linker.(*component.ComponentLinkerWrapper)
	if !ok {
		return fmt.Errorf("wasip2: linker must be *component.ComponentLinkerWrapper, got %T", linker)
	}
	basicLinker := component.NewLinker()
	if err := InstantiateWithConfig(basicLinker, config); err != nil {
		return fmt.Errorf("wasip2: failed to register WASI P2 interfaces: %w", err)
	}
	wrapper.MergeFrom(basicLinker)
	return nil
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
