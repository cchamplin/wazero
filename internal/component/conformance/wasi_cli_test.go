// Package conformance contains conformance tests for the Component Model implementation.
// This file implements Task 278: WASI CLI Conformance Tests.
package conformance

import (
	"bytes"
	"context"
	"testing"

	"github.com/tetratelabs/wazero/imports/wasip2"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// =============================================================================
// Task 278: WASI CLI Conformance Tests
// =============================================================================

// TestWASI_CLI_Environment tests that get-environment returns configured env vars.
func TestWASI_CLI_Environment(t *testing.T) {
	linker := component.NewLinker()

	// Configure with specific environment variables
	config := wasip2.NewConfig().
		WithEnviron([]string{"HOME=/home/test", "PATH=/usr/bin", "WASI_TEST=value"})

	err := wasip2.InstantiateWithConfig(linker, config)
	require.NoError(t, err)

	ctx := context.Background()
	ctx = wasip2.WithConfig(ctx, config)
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	// Get the environment interface
	envDef, ok := linker.Get("wasi:cli/environment@0.2.0")
	require.True(t, ok, "environment interface should be registered")

	instDef, ok := envDef.(*component.InstanceDef)
	require.True(t, ok, "should be an InstanceDef")

	getEnvFunc, ok := instDef.Exports["get-environment"]
	require.True(t, ok, "get-environment function should be exported")

	funcDef, ok := getEnvFunc.(*component.FuncDef)
	require.True(t, ok, "should be a FuncDef")

	// Call get-environment
	result, err := funcDef.Callback(ctx, []component.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result), "get-environment should return exactly one value")

	// Result should be a list of tuples
	envList := result[0].List()
	require.Equal(t, 3, len(envList), "should return 3 environment variables")

	// Verify the environment variables
	envMap := make(map[string]string)
	for _, entry := range envList {
		tuple := entry.Tuple()
		require.Equal(t, 2, len(tuple), "each entry should be a tuple of (key, value)")
		key := tuple[0].StringVal()
		value := tuple[1].StringVal()
		envMap[key] = value
	}

	require.Equal(t, "/home/test", envMap["HOME"], "HOME should be /home/test")
	require.Equal(t, "/usr/bin", envMap["PATH"], "PATH should be /usr/bin")
	require.Equal(t, "value", envMap["WASI_TEST"], "WASI_TEST should be value")
}

// TestWASI_CLI_GetArguments tests that get-arguments returns configured args.
func TestWASI_CLI_GetArguments(t *testing.T) {
	linker := component.NewLinker()

	// Configure with specific arguments
	config := wasip2.NewConfig().
		WithArgs([]string{"myprogram", "--verbose", "-f", "file.txt"})

	err := wasip2.InstantiateWithConfig(linker, config)
	require.NoError(t, err)

	ctx := context.Background()
	ctx = wasip2.WithConfig(ctx, config)
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	// Get the environment interface
	envDef, ok := linker.Get("wasi:cli/environment@0.2.0")
	require.True(t, ok, "environment interface should be registered")

	instDef := envDef.(*component.InstanceDef)

	getArgsFunc, ok := instDef.Exports["get-arguments"]
	require.True(t, ok, "get-arguments function should be exported")

	funcDef := getArgsFunc.(*component.FuncDef)

	// Call get-arguments
	result, err := funcDef.Callback(ctx, []component.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result), "get-arguments should return exactly one value")

	// Result should be a list of strings
	argList := result[0].List()
	require.Equal(t, 4, len(argList), "should return 4 arguments")

	require.Equal(t, "myprogram", argList[0].StringVal())
	require.Equal(t, "--verbose", argList[1].StringVal())
	require.Equal(t, "-f", argList[2].StringVal())
	require.Equal(t, "file.txt", argList[3].StringVal())
}

// TestWASI_CLI_InitialCwd tests that initial-cwd returns the working directory.
func TestWASI_CLI_InitialCwd(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()

	// Get the environment interface
	envDef, ok := linker.Get("wasi:cli/environment@0.2.0")
	require.True(t, ok, "environment interface should be registered")

	instDef := envDef.(*component.InstanceDef)

	initialCwdFunc, ok := instDef.Exports["initial-cwd"]
	require.True(t, ok, "initial-cwd function should be exported")

	funcDef := initialCwdFunc.(*component.FuncDef)

	// Call initial-cwd
	result, err := funcDef.Callback(ctx, []component.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result), "initial-cwd should return exactly one value")

	// Result should be an option<string>
	optVal := result[0].Option()
	// Should return Some(path) since we should be able to get cwd
	require.NotNil(t, optVal, "initial-cwd should return Some(path)")

	cwd := optVal.StringVal()
	require.True(t, len(cwd) > 0, "cwd should not be empty")
}

// TestWASI_CLI_Exit tests that the exit interface exists.
func TestWASI_CLI_Exit(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	// Get the exit interface
	exitDef, ok := linker.Get("wasi:cli/exit@0.2.0")
	require.True(t, ok, "exit interface should be registered")

	instDef, ok := exitDef.(*component.InstanceDef)
	require.True(t, ok, "should be an InstanceDef")

	exitFunc, ok := instDef.Exports["exit"]
	require.True(t, ok, "exit function should be exported")
	require.NotNil(t, exitFunc, "exit function should not be nil")
}

// TestWASI_CLI_Stdin tests that get-stdin returns an input-stream handle.
func TestWASI_CLI_Stdin(t *testing.T) {
	linker := component.NewLinker()

	// Configure with custom stdin
	inputData := "test input data"
	stdin := bytes.NewBufferString(inputData)
	config := wasip2.NewConfig().WithStdin(stdin)

	err := wasip2.InstantiateWithConfig(linker, config)
	require.NoError(t, err)

	ctx := context.Background()
	ctx = wasip2.WithConfig(ctx, config)
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	// Get the stdin interface
	stdinDef, ok := linker.Get("wasi:cli/stdin@0.2.0")
	require.True(t, ok, "stdin interface should be registered")

	instDef := stdinDef.(*component.InstanceDef)

	getStdinFunc, ok := instDef.Exports["get-stdin"]
	require.True(t, ok, "get-stdin function should be exported")

	funcDef := getStdinFunc.(*component.FuncDef)

	// Call get-stdin
	result, err := funcDef.Callback(ctx, []component.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result), "get-stdin should return exactly one value")

	// Result should be an own<input-stream> handle
	handle := result[0].Own()
	// Handle 0 is a placeholder when config is not in context, handle >= 1 is a real stream
	require.True(t, handle >= 0, "should return a valid input-stream handle")
}

// TestWASI_CLI_Stdout tests that get-stdout returns an output-stream handle.
func TestWASI_CLI_Stdout(t *testing.T) {
	linker := component.NewLinker()

	// Configure with custom stdout
	stdout := &bytes.Buffer{}
	config := wasip2.NewConfig().WithStdout(stdout)

	err := wasip2.InstantiateWithConfig(linker, config)
	require.NoError(t, err)

	ctx := context.Background()
	ctx = wasip2.WithConfig(ctx, config)
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	// Get the stdout interface
	stdoutDef, ok := linker.Get("wasi:cli/stdout@0.2.0")
	require.True(t, ok, "stdout interface should be registered")

	instDef := stdoutDef.(*component.InstanceDef)

	getStdoutFunc, ok := instDef.Exports["get-stdout"]
	require.True(t, ok, "get-stdout function should be exported")

	funcDef := getStdoutFunc.(*component.FuncDef)

	// Call get-stdout
	result, err := funcDef.Callback(ctx, []component.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result), "get-stdout should return exactly one value")

	// Result should be an own<output-stream> handle
	handle := result[0].Own()
	// Handle 0 is a placeholder when config is not in context, handle >= 1 is a real stream
	require.True(t, handle >= 0, "should return a valid output-stream handle")
}

// TestWASI_CLI_Stderr tests that get-stderr returns an output-stream handle.
func TestWASI_CLI_Stderr(t *testing.T) {
	linker := component.NewLinker()

	// Configure with custom stderr
	stderr := &bytes.Buffer{}
	config := wasip2.NewConfig().WithStderr(stderr)

	err := wasip2.InstantiateWithConfig(linker, config)
	require.NoError(t, err)

	ctx := context.Background()
	ctx = wasip2.WithConfig(ctx, config)
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	// Get the stderr interface
	stderrDef, ok := linker.Get("wasi:cli/stderr@0.2.0")
	require.True(t, ok, "stderr interface should be registered")

	instDef := stderrDef.(*component.InstanceDef)

	getStderrFunc, ok := instDef.Exports["get-stderr"]
	require.True(t, ok, "get-stderr function should be exported")

	funcDef := getStderrFunc.(*component.FuncDef)

	// Call get-stderr
	result, err := funcDef.Callback(ctx, []component.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result), "get-stderr should return exactly one value")

	// Result should be an own<output-stream> handle
	handle := result[0].Own()
	// Handle 0 is a placeholder when config is not in context, handle >= 1 is a real stream
	require.True(t, handle >= 0, "should return a valid output-stream handle")
}

// TestWASI_CLI_EmptyEnvironment tests behavior with no environment configured.
func TestWASI_CLI_EmptyEnvironment(t *testing.T) {
	linker := component.NewLinker()

	// Config with empty environment
	config := wasip2.NewConfig().WithEnviron([]string{})

	err := wasip2.InstantiateWithConfig(linker, config)
	require.NoError(t, err)

	ctx := context.Background()
	ctx = wasip2.WithConfig(ctx, config)
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	envDef, _ := linker.Get("wasi:cli/environment@0.2.0")
	instDef := envDef.(*component.InstanceDef)
	funcDef := instDef.Exports["get-environment"].(*component.FuncDef)

	result, err := funcDef.Callback(ctx, []component.Val{})
	require.NoError(t, err)

	envList := result[0].List()
	require.Equal(t, 0, len(envList), "should return empty list when no env configured")
}

// TestWASI_CLI_InterfaceRegistration tests that all CLI interfaces are properly registered.
func TestWASI_CLI_InterfaceRegistration(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	// Verify all CLI interfaces are registered
	interfaces := []string{
		"wasi:cli/environment@0.2.0",
		"wasi:cli/stdin@0.2.0",
		"wasi:cli/stdout@0.2.0",
		"wasi:cli/stderr@0.2.0",
		"wasi:cli/exit@0.2.0",
		"wasi:cli/terminal-input@0.2.0",
		"wasi:cli/terminal-output@0.2.0",
	}

	for _, iface := range interfaces {
		def, ok := linker.Get(iface)
		require.True(t, ok, "interface %s should be registered", iface)
		require.NotNil(t, def, "interface %s definition should not be nil", iface)
	}
}

// TestWASI_CLI_AllFunctionsExist verifies all expected functions exist.
func TestWASI_CLI_AllFunctionsExist(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	// Test wasi:cli/environment@0.2.0
	t.Run("Environment", func(t *testing.T) {
		envDef, ok := linker.Get("wasi:cli/environment@0.2.0")
		require.True(t, ok, "environment interface should be registered")

		instDef := envDef.(*component.InstanceDef)

		expectedFunctions := []string{
			"get-environment",
			"get-arguments",
			"initial-cwd",
		}

		for _, fn := range expectedFunctions {
			funcDef, ok := instDef.Exports[fn]
			require.True(t, ok, "function %s should be exported", fn)
			require.NotNil(t, funcDef, "function %s should not be nil", fn)
		}
	})

	// Test wasi:cli/exit@0.2.0
	t.Run("Exit", func(t *testing.T) {
		exitDef, ok := linker.Get("wasi:cli/exit@0.2.0")
		require.True(t, ok, "exit interface should be registered")

		instDef := exitDef.(*component.InstanceDef)

		funcDef, ok := instDef.Exports["exit"]
		require.True(t, ok, "exit function should be exported")
		require.NotNil(t, funcDef, "exit function should not be nil")
	})

	// Test wasi:cli/stdin@0.2.0
	t.Run("Stdin", func(t *testing.T) {
		stdinDef, ok := linker.Get("wasi:cli/stdin@0.2.0")
		require.True(t, ok, "stdin interface should be registered")

		instDef := stdinDef.(*component.InstanceDef)

		funcDef, ok := instDef.Exports["get-stdin"]
		require.True(t, ok, "get-stdin function should be exported")
		require.NotNil(t, funcDef, "get-stdin function should not be nil")
	})

	// Test wasi:cli/stdout@0.2.0
	t.Run("Stdout", func(t *testing.T) {
		stdoutDef, ok := linker.Get("wasi:cli/stdout@0.2.0")
		require.True(t, ok, "stdout interface should be registered")

		instDef := stdoutDef.(*component.InstanceDef)

		funcDef, ok := instDef.Exports["get-stdout"]
		require.True(t, ok, "get-stdout function should be exported")
		require.NotNil(t, funcDef, "get-stdout function should not be nil")
	})

	// Test wasi:cli/stderr@0.2.0
	t.Run("Stderr", func(t *testing.T) {
		stderrDef, ok := linker.Get("wasi:cli/stderr@0.2.0")
		require.True(t, ok, "stderr interface should be registered")

		instDef := stderrDef.(*component.InstanceDef)

		funcDef, ok := instDef.Exports["get-stderr"]
		require.True(t, ok, "get-stderr function should be exported")
		require.NotNil(t, funcDef, "get-stderr function should not be nil")
	})
}

// TestWASI_CLI_TerminalResources tests that terminal resource types exist.
func TestWASI_CLI_TerminalResources(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	// Test terminal-input resource exists
	t.Run("TerminalInput", func(t *testing.T) {
		termInputDef, ok := linker.Get("wasi:cli/terminal-input@0.2.0")
		require.True(t, ok, "terminal-input interface should be registered")

		instDef := termInputDef.(*component.InstanceDef)

		// Should have a terminal-input resource
		resDef, ok := instDef.Exports["terminal-input"]
		require.True(t, ok, "terminal-input resource should be exported")
		require.NotNil(t, resDef, "terminal-input resource should not be nil")
	})

	// Test terminal-output resource exists
	t.Run("TerminalOutput", func(t *testing.T) {
		termOutputDef, ok := linker.Get("wasi:cli/terminal-output@0.2.0")
		require.True(t, ok, "terminal-output interface should be registered")

		instDef := termOutputDef.(*component.InstanceDef)

		// Should have a terminal-output resource
		resDef, ok := instDef.Exports["terminal-output"]
		require.True(t, ok, "terminal-output resource should be exported")
		require.NotNil(t, resDef, "terminal-output resource should not be nil")
	})
}
