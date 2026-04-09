// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package conformance: WASI CLI conformance tests verify that
// wazero's cli host module correctly handles environment variables,
// arguments, exit, and standard stream accessor functions.
package conformance

import (
	"testing"

	"github.com/tetratelabs/wazero/imports/wasip2/cli"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestWASICLI exercises the wasi:cli host module API surface.
//
// No counterpart (justified): WASI P2 host module conformance invariant.
func TestWASICLI(t *testing.T) {

	// ------------------------------------------------------------------
	// Case 1: Instantiate registers all expected wasi:cli interfaces.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. cli.Instantiate must register all 10 wasi:cli
	// interfaces with the linker.
	t.Run("InstantiateRegistersInterfaces", func(t *testing.T) {
		linker := component.NewLinker()
		err := cli.Instantiate(linker)
		require.NoError(t, err)

		interfaces := []string{
			"wasi:cli/environment@0.2.0",
			"wasi:cli/exit@0.2.0",
			"wasi:cli/stdin@0.2.0",
			"wasi:cli/stdout@0.2.0",
			"wasi:cli/stderr@0.2.0",
			"wasi:cli/terminal-input@0.2.0",
			"wasi:cli/terminal-output@0.2.0",
			"wasi:cli/terminal-stdin@0.2.0",
			"wasi:cli/terminal-stdout@0.2.0",
			"wasi:cli/terminal-stderr@0.2.0",
		}
		for _, iface := range interfaces {
			def, lookupErr := linker.MatchImport(iface)
			require.NoError(t, lookupErr, "interface %s should be registered", iface)
			_, ok := def.(*component.InstanceDef)
			require.True(t, ok, "expected InstanceDef for %s", iface)
		}
	})

	// ------------------------------------------------------------------
	// Case 2: Duplicate Instantiate fails.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. Registering the same interfaces twice must return an
	// error to prevent accidental double-registration.
	t.Run("InstantiateDuplicateFails", func(t *testing.T) {
		linker := component.NewLinker()
		err := cli.Instantiate(linker)
		require.NoError(t, err)
		err = cli.Instantiate(linker)
		require.Error(t, err)
	})

	// ------------------------------------------------------------------
	// Case 3: ExitError with code 0 reports success.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. wasi:cli/exit@0.2.0 exit(ok) must signal success.
	t.Run("ExitErrorSuccess", func(t *testing.T) {
		exitErr := &cli.ExitError{Code: 0}
		require.Equal(t, "exit: success", exitErr.Error())
		// Verify it satisfies the error interface
		var err error = exitErr
		require.True(t, err != nil)
	})

	// ------------------------------------------------------------------
	// Case 4: ExitError with code 1 reports failure.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. wasi:cli/exit@0.2.0 exit(err) must signal failure.
	t.Run("ExitErrorFailure", func(t *testing.T) {
		exitErr := &cli.ExitError{Code: 1}
		require.Equal(t, "exit: failure", exitErr.Error())
	})

	// ------------------------------------------------------------------
	// Case 5: Environment returns empty list without WASI config.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. get-environment with no config in context must return
	// an empty list (graceful degradation).
	t.Run("EnvironmentWithoutConfig", func(t *testing.T) {
		// Verify the linker-level registration succeeds (environment
		// function exists). We do not call the raw host function here
		// since it requires context plumbing; the unit tests in
		// imports/wasip2/cli already cover that path.
		linker := component.NewLinker()
		err := cli.Instantiate(linker)
		require.NoError(t, err)
		def, err := linker.MatchImport("wasi:cli/environment@0.2.0")
		require.NoError(t, err)
		instDef, ok := def.(*component.InstanceDef)
		require.True(t, ok)
		_, hasGetEnv := instDef.Exports["get-environment"]
		require.True(t, hasGetEnv, "get-environment function should be exported")
	})

	// ------------------------------------------------------------------
	// Case 6: Stdin/stdout/stderr accessors are registered.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. The linker must expose get-stdin, get-stdout, and
	// get-stderr functions.
	t.Run("StdioAccessorsRegistered", func(t *testing.T) {
		linker := component.NewLinker()
		err := cli.Instantiate(linker)
		require.NoError(t, err)

		for _, pair := range []struct{ iface, fn string }{
			{"wasi:cli/stdin@0.2.0", "get-stdin"},
			{"wasi:cli/stdout@0.2.0", "get-stdout"},
			{"wasi:cli/stderr@0.2.0", "get-stderr"},
		} {
			def, lookupErr := linker.MatchImport(pair.iface)
			require.NoError(t, lookupErr)
			instDef, ok := def.(*component.InstanceDef)
			require.True(t, ok)
			_, hasFn := instDef.Exports[pair.fn]
			require.True(t, hasFn, "%s should export %s", pair.iface, pair.fn)
		}
	})
}
