// Package wasip2 provides WASI Preview 2 host implementations for
// WebAssembly components.
//
// WASI Preview 2 (WASI P2) defines standardized interfaces that allow
// components to interact with the host environment: file I/O, networking,
// clocks, random number generation, and more. This package implements all
// major WASI P2 interfaces:
//
//   - wasi:cli — environment variables, arguments, stdio streams
//   - wasi:clocks — wall clock and monotonic clock
//   - wasi:random — cryptographic and insecure random
//   - wasi:filesystem — file and directory operations with preopens
//   - wasi:sockets — TCP/UDP networking and DNS resolution
//   - wasi:http — outgoing HTTP requests and incoming handler support
//   - wasi:io — streams and pollable I/O
//
// # Usage with ComponentLinker
//
// Most WASI P2 interfaces use pre-1.0 versions (0.2.x), so
// [api.ComponentLinker.SetRelaxedSemverMatching] should be enabled.
//
// # Configuration
//
// Use [NewConfig] to customize WASI behavior (stdio, environment, args):
//
//	config := wasip2.NewConfig().
//		WithStdout(os.Stdout).
//		WithArgs([]string{"my-app", "--verbose"}).
//		WithEnviron([]string{"HOME=/home/user"})
//	wasip2.InitializeWithConfig(linker, config)
//	ctx = wasip2.WithConfig(ctx, config) // pass config in context
//
// See https://github.com/WebAssembly/WASI
package wasip2

import (
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasip2/cli"
	"github.com/tetratelabs/wazero/imports/wasip2/clocks"
	"github.com/tetratelabs/wazero/imports/wasip2/filesystem"
	wasip2http "github.com/tetratelabs/wazero/imports/wasip2/http"
	wasip2io "github.com/tetratelabs/wazero/imports/wasip2/io"
	"github.com/tetratelabs/wazero/imports/wasip2/random"
	"github.com/tetratelabs/wazero/imports/wasip2/sockets"
)

// Instantiate registers all WASI Preview 2 interfaces with the linker.
// Uses default configuration backed by os package.
func Instantiate(linker api.ComponentLinker) error {
	return InstantiateWithConfig(linker, DefaultConfig())
}

// InstantiateWithConfig registers all WASI Preview 2 interfaces with custom configuration.
func InstantiateWithConfig(linker api.ComponentLinker, config *Config) error {
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
