// Copyright 2024 Tetrate
// SPDX-License-Identifier: Apache-2.0

package wasip2

import (
	"bytes"
	"context"
	"testing"

	wasip2io "github.com/tetratelabs/wazero/imports/wasip2/io"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestInstantiateWithDefaultConfig verifies that all WASI Preview 2 interfaces
// are properly registered with the linker when using the default configuration.
func TestInstantiateWithDefaultConfig(t *testing.T) {
	linker := component.NewLinker()

	err := Instantiate(linker)
	require.NoError(t, err)

	// All major interfaces that should be registered
	interfaces := []string{
		// wasi:io interfaces
		"wasi:io/streams@0.2.0",
		"wasi:io/poll@0.2.0",
		"wasi:io/error@0.2.0",
		// wasi:clocks interfaces
		"wasi:clocks/wall-clock@0.2.0",
		"wasi:clocks/monotonic-clock@0.2.0",
		// wasi:random interfaces
		"wasi:random/random@0.2.0",
		"wasi:random/insecure@0.2.0",
		"wasi:random/insecure-seed@0.2.0",
		// wasi:cli interfaces
		"wasi:cli/environment@0.2.0",
		"wasi:cli/stdin@0.2.0",
		"wasi:cli/stdout@0.2.0",
		"wasi:cli/stderr@0.2.0",
		"wasi:cli/exit@0.2.0",
		"wasi:cli/terminal-input@0.2.0",
		"wasi:cli/terminal-output@0.2.0",
		// wasi:filesystem interfaces
		"wasi:filesystem/types@0.2.0",
		"wasi:filesystem/preopens@0.2.0",
		// wasi:sockets interfaces
		"wasi:sockets/network@0.2.0",
		"wasi:sockets/instance-network@0.2.0",
		"wasi:sockets/ip-name-lookup@0.2.0",
		"wasi:sockets/tcp@0.2.0",
		"wasi:sockets/tcp-create-socket@0.2.0",
		"wasi:sockets/udp@0.2.0",
		"wasi:sockets/udp-create-socket@0.2.0",
		// wasi:http interfaces
		"wasi:http/types@0.2.0",
		"wasi:http/outgoing-handler@0.2.0",
		"wasi:http/incoming-handler@0.2.0",
	}

	for _, iface := range interfaces {
		def, ok := linker.Get(iface)
		require.True(t, ok, "interface %s should be registered", iface)
		require.NotNil(t, def, "interface %s definition should not be nil", iface)
	}
}

// TestInstantiateWithCustomConfig verifies that custom configuration is properly
// propagated when using InstantiateWithConfig.
func TestInstantiateWithCustomConfig(t *testing.T) {
	linker := component.NewLinker()

	stdin := bytes.NewBufferString("custom input")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	config := NewConfig().
		WithStdin(stdin).
		WithStdout(stdout).
		WithStderr(stderr).
		WithEnviron([]string{"FOO=bar", "BAZ=qux"}).
		WithArgs([]string{"prog", "arg1", "arg2"}).
		WithPreopen("/guest", "/host/path").
		WithNetwork(true).
		WithHTTP(true)

	err := InstantiateWithConfig(linker, config)
	require.NoError(t, err)

	// Verify the config is set correctly
	require.Equal(t, stdin, config.Stdin())
	require.Equal(t, stdout, config.Stdout())
	require.Equal(t, stderr, config.Stderr())
	require.Equal(t, []string{"FOO=bar", "BAZ=qux"}, config.Environ())
	require.Equal(t, []string{"prog", "arg1", "arg2"}, config.Args())
	require.Equal(t, "/host/path", config.Preopens()["/guest"])
	require.True(t, config.AllowNetwork())
	require.True(t, config.AllowHTTP())

	// Verify key interfaces are still registered
	def, ok := linker.Get("wasi:cli/environment@0.2.0")
	require.True(t, ok, "cli/environment interface should be registered")
	require.NotNil(t, def)

	def, ok = linker.Get("wasi:io/streams@0.2.0")
	require.True(t, ok, "io/streams interface should be registered")
	require.NotNil(t, def)
}

// TestFullWorkflow tests a realistic scenario where the config is used to
// configure stdin/stdout/stderr and the CLI functions are invoked through
// the linker to verify they return the correct values.
func TestFullWorkflow(t *testing.T) {
	// Setup custom streams
	inputData := "test input data"
	stdin := bytes.NewBufferString(inputData)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	// Create config with custom values
	config := NewConfig().
		WithStdin(stdin).
		WithStdout(stdout).
		WithStderr(stderr).
		WithEnviron([]string{"HOME=/home/test", "PATH=/usr/bin"}).
		WithArgs([]string{"myapp", "--verbose", "file.txt"})

	linker := component.NewLinker()
	table := component.NewResourceTable()

	// Create context with config and resource table
	ctx := WithConfig(context.Background(), config)
	ctx = component.WithResourceTable(ctx, table)

	// Instantiate all WASI interfaces
	err := InstantiateWithConfig(linker, config)
	require.NoError(t, err)

	// Test: Invoke get-environment through the linker
	t.Run("GetEnvironment", func(t *testing.T) {
		envDef, ok := linker.Get("wasi:cli/environment@0.2.0")
		require.True(t, ok)

		instDef, ok := envDef.(*component.InstanceDef)
		require.True(t, ok)

		getEnvFunc, ok := instDef.Exports["get-environment"]
		require.True(t, ok)

		funcDef, ok := getEnvFunc.(*component.FuncDef)
		require.True(t, ok)

		result, err := funcDef.Callback(ctx, []component.Val{})
		require.NoError(t, err)
		require.Equal(t, 1, len(result))

		// The result is a list of tuples
		envList := result[0].List()
		require.Equal(t, 2, len(envList))

		// Check first env var: HOME=/home/test
		tuple0 := envList[0].Tuple()
		require.Equal(t, "HOME", tuple0[0].StringVal())
		require.Equal(t, "/home/test", tuple0[1].StringVal())

		// Check second env var: PATH=/usr/bin
		tuple1 := envList[1].Tuple()
		require.Equal(t, "PATH", tuple1[0].StringVal())
		require.Equal(t, "/usr/bin", tuple1[1].StringVal())
	})

	// Test: Invoke get-arguments through the linker
	t.Run("GetArguments", func(t *testing.T) {
		envDef, ok := linker.Get("wasi:cli/environment@0.2.0")
		require.True(t, ok)

		instDef, ok := envDef.(*component.InstanceDef)
		require.True(t, ok)

		getArgsFunc, ok := instDef.Exports["get-arguments"]
		require.True(t, ok)

		funcDef, ok := getArgsFunc.(*component.FuncDef)
		require.True(t, ok)

		result, err := funcDef.Callback(ctx, []component.Val{})
		require.NoError(t, err)
		require.Equal(t, 1, len(result))

		// The result is a list of strings
		argList := result[0].List()
		require.Equal(t, 3, len(argList))
		require.Equal(t, "myapp", argList[0].StringVal())
		require.Equal(t, "--verbose", argList[1].StringVal())
		require.Equal(t, "file.txt", argList[2].StringVal())
	})

	// Test: Get stdin stream and read from it
	t.Run("StdinStream", func(t *testing.T) {
		stdinDef, ok := linker.Get("wasi:cli/stdin@0.2.0")
		require.True(t, ok)

		instDef, ok := stdinDef.(*component.InstanceDef)
		require.True(t, ok)

		getStdinFunc, ok := instDef.Exports["get-stdin"]
		require.True(t, ok)

		funcDef, ok := getStdinFunc.(*component.FuncDef)
		require.True(t, ok)

		result, err := funcDef.Callback(ctx, []component.Val{})
		require.NoError(t, err)
		require.Equal(t, 1, len(result))

		// The result is an own<input-stream> handle
		handle := result[0].Own()

		// Get the stream from the resource table and verify we can read from it
		entry, err := table.Get(component.Handle(handle))
		require.NoError(t, err)

		stream, ok := entry.Rep.(*wasip2io.InputStream)
		require.True(t, ok)

		data, streamErr := stream.Read(uint64(len(inputData)))
		require.Nil(t, streamErr)
		require.Equal(t, inputData, string(data))
	})

	// Test: Get stdout stream and write to it
	t.Run("StdoutStream", func(t *testing.T) {
		stdoutDef, ok := linker.Get("wasi:cli/stdout@0.2.0")
		require.True(t, ok)

		instDef, ok := stdoutDef.(*component.InstanceDef)
		require.True(t, ok)

		getStdoutFunc, ok := instDef.Exports["get-stdout"]
		require.True(t, ok)

		funcDef, ok := getStdoutFunc.(*component.FuncDef)
		require.True(t, ok)

		result, err := funcDef.Callback(ctx, []component.Val{})
		require.NoError(t, err)
		require.Equal(t, 1, len(result))

		// The result is an own<output-stream> handle
		handle := result[0].Own()

		// Get the stream from the resource table and verify we can write to it
		entry, err := table.Get(component.Handle(handle))
		require.NoError(t, err)

		stream, ok := entry.Rep.(*wasip2io.OutputStream)
		require.True(t, ok)

		testOutput := "hello output"
		streamErr := stream.Write([]byte(testOutput))
		require.Nil(t, streamErr)
		require.Equal(t, testOutput, stdout.String())
	})

	// Test: Get stderr stream and write to it
	t.Run("StderrStream", func(t *testing.T) {
		stderrDef, ok := linker.Get("wasi:cli/stderr@0.2.0")
		require.True(t, ok)

		instDef, ok := stderrDef.(*component.InstanceDef)
		require.True(t, ok)

		getStderrFunc, ok := instDef.Exports["get-stderr"]
		require.True(t, ok)

		funcDef, ok := getStderrFunc.(*component.FuncDef)
		require.True(t, ok)

		result, err := funcDef.Callback(ctx, []component.Val{})
		require.NoError(t, err)
		require.Equal(t, 1, len(result))

		// The result is an own<output-stream> handle
		handle := result[0].Own()

		// Get the stream from the resource table and verify we can write to it
		entry, err := table.Get(component.Handle(handle))
		require.NoError(t, err)

		stream, ok := entry.Rep.(*wasip2io.OutputStream)
		require.True(t, ok)

		testError := "error message"
		streamErr := stream.Write([]byte(testError))
		require.Nil(t, streamErr)
		require.Equal(t, testError, stderr.String())
	})
}

// TestConfigWithoutDefaults verifies that NewConfig() creates a config
// without any default values set.
func TestConfigWithoutDefaults(t *testing.T) {
	config := NewConfig()

	require.Nil(t, config.Stdin())
	require.Nil(t, config.Stdout())
	require.Nil(t, config.Stderr())
	require.Nil(t, config.Environ())
	require.Nil(t, config.Args())
	require.NotNil(t, config.Preopens()) // preopens is an empty map, not nil
	require.Equal(t, 0, len(config.Preopens()))
	require.False(t, config.AllowNetwork())
	require.False(t, config.AllowHTTP())
}

// TestDefaultConfigWithOSDefaults verifies that DefaultConfig() creates
// a config with OS-backed defaults.
func TestDefaultConfigWithOSDefaults(t *testing.T) {
	config := DefaultConfig()

	// These should be non-nil and backed by os package
	require.NotNil(t, config.Stdin())
	require.NotNil(t, config.Stdout())
	require.NotNil(t, config.Stderr())
	require.NotNil(t, config.Environ())
	require.NotNil(t, config.Args())
	require.NotNil(t, config.Preopens())
	require.True(t, config.AllowNetwork())
	require.True(t, config.AllowHTTP())
}

// TestInstantiateIdempotent verifies that attempting to instantiate twice
// returns an error for duplicate definitions.
func TestInstantiateIdempotent(t *testing.T) {
	linker := component.NewLinker()

	// First instantiation should succeed
	err := Instantiate(linker)
	require.NoError(t, err)

	// Second instantiation should fail because definitions already exist
	err = Instantiate(linker)
	require.Error(t, err)
}

// TestContextPropagation verifies that the config set in context is properly
// retrieved by the host functions.
func TestContextPropagation(t *testing.T) {
	config := NewConfig().
		WithEnviron([]string{"CONTEXT_TEST=passed"}).
		WithArgs([]string{"context_prog"})

	ctx := context.Background()

	// Initially no config
	retrieved := ConfigFromContext(ctx)
	require.Nil(t, retrieved)

	// Set config in context
	ctx = WithConfig(ctx, config)

	// Now config should be retrievable
	retrieved = ConfigFromContext(ctx)
	require.NotNil(t, retrieved)
	require.Equal(t, []string{"CONTEXT_TEST=passed"}, retrieved.Environ())
	require.Equal(t, []string{"context_prog"}, retrieved.Args())
}

// TestResourceTableIntegration verifies that resource handles work correctly
// across multiple operations.
func TestResourceTableIntegration(t *testing.T) {
	stdin := bytes.NewBufferString("multi-read test")
	stdout := &bytes.Buffer{}

	config := NewConfig().
		WithStdin(stdin).
		WithStdout(stdout)

	linker := component.NewLinker()
	table := component.NewResourceTable()

	ctx := WithConfig(context.Background(), config)
	ctx = component.WithResourceTable(ctx, table)

	err := InstantiateWithConfig(linker, config)
	require.NoError(t, err)

	// Get stdin handle
	stdinDef, ok := linker.Get("wasi:cli/stdin@0.2.0")
	require.True(t, ok)

	instDef := stdinDef.(*component.InstanceDef)
	getStdinFunc := instDef.Exports["get-stdin"].(*component.FuncDef)

	result, err := getStdinFunc.Callback(ctx, []component.Val{})
	require.NoError(t, err)

	stdinHandle := result[0].Own()

	// Get stdout handle
	stdoutDef, _ := linker.Get("wasi:cli/stdout@0.2.0")
	instDef = stdoutDef.(*component.InstanceDef)
	getStdoutFunc := instDef.Exports["get-stdout"].(*component.FuncDef)

	result, err = getStdoutFunc.Callback(ctx, []component.Val{})
	require.NoError(t, err)

	stdoutHandle := result[0].Own()

	// Verify handles are different
	require.NotEqual(t, stdinHandle, stdoutHandle)

	// Verify we can retrieve both from the table
	stdinEntry, err := table.Get(component.Handle(stdinHandle))
	require.NoError(t, err)
	require.NotNil(t, stdinEntry.Rep)

	stdoutEntry, err := table.Get(component.Handle(stdoutHandle))
	require.NoError(t, err)
	require.NotNil(t, stdoutEntry.Rep)

	// Verify they are different types
	_, isInputStream := stdinEntry.Rep.(*wasip2io.InputStream)
	require.True(t, isInputStream)

	_, isOutputStream := stdoutEntry.Rep.(*wasip2io.OutputStream)
	require.True(t, isOutputStream)
}

// TestClocksIntegration verifies that clock interfaces are properly accessible.
func TestClocksIntegration(t *testing.T) {
	linker := component.NewLinker()

	err := Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	// Test wall-clock now function
	t.Run("WallClockNow", func(t *testing.T) {
		clockDef, ok := linker.Get("wasi:clocks/wall-clock@0.2.0")
		require.True(t, ok)

		instDef, ok := clockDef.(*component.InstanceDef)
		require.True(t, ok)

		nowFunc, ok := instDef.Exports["now"]
		require.True(t, ok)

		funcDef, ok := nowFunc.(*component.FuncDef)
		require.True(t, ok)

		result, err := funcDef.Callback(ctx, []component.Val{})
		require.NoError(t, err)
		require.Equal(t, 1, len(result))

		// Wall clock returns a record with seconds and nanoseconds
		record := result[0].Record()
		require.NotNil(t, record)
	})

	// Test monotonic-clock now function
	t.Run("MonotonicClockNow", func(t *testing.T) {
		clockDef, ok := linker.Get("wasi:clocks/monotonic-clock@0.2.0")
		require.True(t, ok)

		instDef, ok := clockDef.(*component.InstanceDef)
		require.True(t, ok)

		nowFunc, ok := instDef.Exports["now"]
		require.True(t, ok)

		funcDef, ok := nowFunc.(*component.FuncDef)
		require.True(t, ok)

		result, err := funcDef.Callback(ctx, []component.Val{})
		require.NoError(t, err)
		require.Equal(t, 1, len(result))

		// Monotonic clock returns u64 (instant)
		instant := result[0].U64()
		require.NotEqual(t, uint64(0), instant)
	})
}

// TestRandomIntegration verifies that random interfaces produce values.
func TestRandomIntegration(t *testing.T) {
	linker := component.NewLinker()

	err := Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()

	// Test random get-random-bytes function
	t.Run("GetRandomBytes", func(t *testing.T) {
		randomDef, ok := linker.Get("wasi:random/random@0.2.0")
		require.True(t, ok)

		instDef, ok := randomDef.(*component.InstanceDef)
		require.True(t, ok)

		getRandomFunc, ok := instDef.Exports["get-random-bytes"]
		require.True(t, ok)

		funcDef, ok := getRandomFunc.(*component.FuncDef)
		require.True(t, ok)

		// Request 16 random bytes
		result, err := funcDef.Callback(ctx, []component.Val{component.ValU64(16)})
		require.NoError(t, err)
		require.Equal(t, 1, len(result))

		// Should return a list of 16 bytes
		byteList := result[0].List()
		require.Equal(t, 16, len(byteList))
	})

	// Test random get-random-u64 function
	t.Run("GetRandomU64", func(t *testing.T) {
		randomDef, ok := linker.Get("wasi:random/random@0.2.0")
		require.True(t, ok)

		instDef, ok := randomDef.(*component.InstanceDef)
		require.True(t, ok)

		getRandomU64Func, ok := instDef.Exports["get-random-u64"]
		require.True(t, ok)

		funcDef, ok := getRandomU64Func.(*component.FuncDef)
		require.True(t, ok)

		// Get two random u64s and verify they are different (statistically unlikely to be same)
		result1, err := funcDef.Callback(ctx, []component.Val{})
		require.NoError(t, err)
		require.Equal(t, 1, len(result1))

		result2, err := funcDef.Callback(ctx, []component.Val{})
		require.NoError(t, err)
		require.Equal(t, 1, len(result2))

		// Extremely unlikely for two random u64s to be equal
		val1 := result1[0].U64()
		val2 := result2[0].U64()
		require.NotEqual(t, val1, val2)
	})
}

// TestFilesystemPreopensIntegration verifies filesystem preopens are accessible.
func TestFilesystemPreopensIntegration(t *testing.T) {
	linker := component.NewLinker()

	err := Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	// Test get-directories function
	preopensDef, ok := linker.Get("wasi:filesystem/preopens@0.2.0")
	require.True(t, ok)

	instDef, ok := preopensDef.(*component.InstanceDef)
	require.True(t, ok)

	getDirsFunc, ok := instDef.Exports["get-directories"]
	require.True(t, ok)

	funcDef, ok := getDirsFunc.(*component.FuncDef)
	require.True(t, ok)

	result, err := funcDef.Callback(ctx, []component.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))

	// Result should be a list (possibly empty if no preopens configured)
	dirList := result[0].List()
	require.NotNil(t, dirList)
}
