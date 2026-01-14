// Package cli implements the wasi:cli interfaces for WASI Preview 2.
// It provides environment, arguments, standard streams, and terminal access.
package cli

import (
	"context"

	"github.com/tetratelabs/wazero/internal/component"
)

// Instantiate registers all wasi:cli interfaces with the linker.
func Instantiate(linker *component.Linker) error {
	if err := instantiateEnvironment(linker); err != nil {
		return err
	}
	if err := instantiateExit(linker); err != nil {
		return err
	}
	if err := instantiateStdin(linker); err != nil {
		return err
	}
	if err := instantiateStdout(linker); err != nil {
		return err
	}
	if err := instantiateStderr(linker); err != nil {
		return err
	}
	if err := instantiateTerminalInput(linker); err != nil {
		return err
	}
	if err := instantiateTerminalOutput(linker); err != nil {
		return err
	}
	return nil
}

// instantiateEnvironment registers wasi:cli/environment@0.2.0
func instantiateEnvironment(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:cli/environment@0.2.0")

	inst.FuncNoType("get-environment", getEnvironment)
	inst.FuncNoType("get-arguments", getArguments)
	inst.FuncNoType("initial-cwd", initialCwd)

	return inst.Build()
}

// getEnvironment returns the environment variables as list<tuple<string, string>>
func getEnvironment(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return empty list as placeholder
	// Full implementation will get environment from config
	return []component.Val{component.ValList([]component.Val{})}, nil
}

// getArguments returns command line arguments as list<string>
func getArguments(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return empty list as placeholder
	// Full implementation will get arguments from config
	return []component.Val{component.ValList([]component.Val{})}, nil
}

// initialCwd returns the initial working directory as option<string>
func initialCwd(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return None as placeholder
	// Full implementation will get cwd from config or os
	return []component.Val{component.ValOption(nil)}, nil
}

// instantiateExit registers wasi:cli/exit@0.2.0
func instantiateExit(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:cli/exit@0.2.0")

	inst.FuncNoType("exit", exit)

	return inst.Build()
}

// exit handles program termination with result<_, _>
func exit(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// For now, this is a no-op stub
	// Full implementation may panic or signal the runtime
	// The argument is result<_, _> where ok means success, error means failure
	return []component.Val{}, nil
}

// instantiateStdin registers wasi:cli/stdin@0.2.0
func instantiateStdin(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:cli/stdin@0.2.0")

	inst.FuncNoType("get-stdin", getStdin)

	return inst.Build()
}

// getStdin returns stdin as own<input-stream>
func getStdin(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return handle 0 as placeholder for stdin
	return []component.Val{component.ValOwn(0)}, nil
}

// instantiateStdout registers wasi:cli/stdout@0.2.0
func instantiateStdout(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:cli/stdout@0.2.0")

	inst.FuncNoType("get-stdout", getStdout)

	return inst.Build()
}

// getStdout returns stdout as own<output-stream>
func getStdout(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return handle 1 as placeholder for stdout
	return []component.Val{component.ValOwn(1)}, nil
}

// instantiateStderr registers wasi:cli/stderr@0.2.0
func instantiateStderr(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:cli/stderr@0.2.0")

	inst.FuncNoType("get-stderr", getStderr)

	return inst.Build()
}

// getStderr returns stderr as own<output-stream>
func getStderr(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return handle 2 as placeholder for stderr
	return []component.Val{component.ValOwn(2)}, nil
}

// instantiateTerminalInput registers wasi:cli/terminal-input@0.2.0
func instantiateTerminalInput(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:cli/terminal-input@0.2.0")

	// Define terminal-input resource with no-op destructor
	inst.Resource("terminal-input", func(rep uint32) {})

	return inst.Build()
}

// instantiateTerminalOutput registers wasi:cli/terminal-output@0.2.0
func instantiateTerminalOutput(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:cli/terminal-output@0.2.0")

	// Define terminal-output resource with no-op destructor
	inst.Resource("terminal-output", func(rep uint32) {})

	return inst.Build()
}
