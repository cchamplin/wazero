// imports/wasip2/cli/cli_test.go

package cli

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestInstantiate(t *testing.T) {
	linker := component.NewLinker()
	err := Instantiate(linker)
	require.NoError(t, err)

	// Verify all interfaces are registered
	interfaces := []string{
		"wasi:cli/environment@0.2.0",
		"wasi:cli/exit@0.2.0",
		"wasi:cli/stdin@0.2.0",
		"wasi:cli/stdout@0.2.0",
		"wasi:cli/stderr@0.2.0",
		"wasi:cli/terminal-input@0.2.0",
		"wasi:cli/terminal-output@0.2.0",
	}

	for _, iface := range interfaces {
		def, err := linker.MatchImport(iface)
		require.NoError(t, err, "interface %s should be registered", iface)
		_, ok := def.(*component.InstanceDef)
		require.True(t, ok, "expected InstanceDef for %s", iface)
	}
}

func TestInstantiate_Duplicate(t *testing.T) {
	linker := component.NewLinker()

	// First registration should succeed
	err := Instantiate(linker)
	require.NoError(t, err)

	// Second registration should fail
	err = Instantiate(linker)
	require.Error(t, err)
}

// Environment interface tests
func TestInstantiateEnvironment(t *testing.T) {
	linker := component.NewLinker()
	err := instantiateEnvironment(linker)
	require.NoError(t, err)

	def, err := linker.MatchImport("wasi:cli/environment@0.2.0")
	require.NoError(t, err)

	instDef, ok := def.(*component.InstanceDef)
	require.True(t, ok, "expected InstanceDef")

	// Verify exports exist
	_, hasGetEnvironment := instDef.Exports["get-environment"]
	require.True(t, hasGetEnvironment, "get-environment function should be defined")

	_, hasGetArguments := instDef.Exports["get-arguments"]
	require.True(t, hasGetArguments, "get-arguments function should be defined")

	_, hasInitialCwd := instDef.Exports["initial-cwd"]
	require.True(t, hasInitialCwd, "initial-cwd function should be defined")
}

func TestGetEnvironment(t *testing.T) {
	result, err := getEnvironment(context.Background(), []component.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindList, result[0].Kind())
	// Returns empty list as placeholder
	list := result[0].List()
	require.NotNil(t, list)
}

func TestGetArguments(t *testing.T) {
	result, err := getArguments(context.Background(), []component.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindList, result[0].Kind())
	// Returns empty list as placeholder
	list := result[0].List()
	require.NotNil(t, list)
}

func TestInitialCwd(t *testing.T) {
	result, err := initialCwd(context.Background(), []component.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindOption, result[0].Kind())
	// Returns None as placeholder
	opt := result[0].Option()
	require.Nil(t, opt, "initial-cwd returns None as placeholder")
}

// Exit interface tests
func TestInstantiateExit(t *testing.T) {
	linker := component.NewLinker()
	err := instantiateExit(linker)
	require.NoError(t, err)

	def, err := linker.MatchImport("wasi:cli/exit@0.2.0")
	require.NoError(t, err)

	instDef, ok := def.(*component.InstanceDef)
	require.True(t, ok, "expected InstanceDef")

	_, hasExit := instDef.Exports["exit"]
	require.True(t, hasExit, "exit function should be defined")
}

func TestExit(t *testing.T) {
	// With ok result - should not error
	okResult := component.ValResultOk(nil)
	result, err := exit(context.Background(), []component.Val{okResult})
	require.NoError(t, err)
	require.Equal(t, 0, len(result), "exit returns nothing")

	// With error result - for now also should not error (just a stub)
	errResult := component.ValResultError(nil)
	result, err = exit(context.Background(), []component.Val{errResult})
	require.NoError(t, err)
	require.Equal(t, 0, len(result), "exit returns nothing")
}

// Stdin interface tests
func TestInstantiateStdin(t *testing.T) {
	linker := component.NewLinker()
	err := instantiateStdin(linker)
	require.NoError(t, err)

	def, err := linker.MatchImport("wasi:cli/stdin@0.2.0")
	require.NoError(t, err)

	instDef, ok := def.(*component.InstanceDef)
	require.True(t, ok, "expected InstanceDef")

	_, hasGetStdin := instDef.Exports["get-stdin"]
	require.True(t, hasGetStdin, "get-stdin function should be defined")
}

func TestGetStdin(t *testing.T) {
	result, err := getStdin(context.Background(), []component.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindOwn, result[0].Kind())
	// Returns handle 0 as placeholder for stdin
	handle := result[0].Own()
	require.Equal(t, uint32(0), handle)
}

// Stdout interface tests
func TestInstantiateStdout(t *testing.T) {
	linker := component.NewLinker()
	err := instantiateStdout(linker)
	require.NoError(t, err)

	def, err := linker.MatchImport("wasi:cli/stdout@0.2.0")
	require.NoError(t, err)

	instDef, ok := def.(*component.InstanceDef)
	require.True(t, ok, "expected InstanceDef")

	_, hasGetStdout := instDef.Exports["get-stdout"]
	require.True(t, hasGetStdout, "get-stdout function should be defined")
}

func TestGetStdout(t *testing.T) {
	result, err := getStdout(context.Background(), []component.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindOwn, result[0].Kind())
	// Returns handle 1 as placeholder for stdout
	handle := result[0].Own()
	require.Equal(t, uint32(1), handle)
}

// Stderr interface tests
func TestInstantiateStderr(t *testing.T) {
	linker := component.NewLinker()
	err := instantiateStderr(linker)
	require.NoError(t, err)

	def, err := linker.MatchImport("wasi:cli/stderr@0.2.0")
	require.NoError(t, err)

	instDef, ok := def.(*component.InstanceDef)
	require.True(t, ok, "expected InstanceDef")

	_, hasGetStderr := instDef.Exports["get-stderr"]
	require.True(t, hasGetStderr, "get-stderr function should be defined")
}

func TestGetStderr(t *testing.T) {
	result, err := getStderr(context.Background(), []component.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindOwn, result[0].Kind())
	// Returns handle 2 as placeholder for stderr
	handle := result[0].Own()
	require.Equal(t, uint32(2), handle)
}

// Terminal input interface tests
func TestInstantiateTerminalInput(t *testing.T) {
	linker := component.NewLinker()
	err := instantiateTerminalInput(linker)
	require.NoError(t, err)

	def, err := linker.MatchImport("wasi:cli/terminal-input@0.2.0")
	require.NoError(t, err)

	instDef, ok := def.(*component.InstanceDef)
	require.True(t, ok, "expected InstanceDef")

	// Verify resource is defined
	_, hasResource := instDef.Exports["terminal-input"]
	require.True(t, hasResource, "terminal-input resource should be defined")
}

// Terminal output interface tests
func TestInstantiateTerminalOutput(t *testing.T) {
	linker := component.NewLinker()
	err := instantiateTerminalOutput(linker)
	require.NoError(t, err)

	def, err := linker.MatchImport("wasi:cli/terminal-output@0.2.0")
	require.NoError(t, err)

	instDef, ok := def.(*component.InstanceDef)
	require.True(t, ok, "expected InstanceDef")

	// Verify resource is defined
	_, hasResource := instDef.Exports["terminal-output"]
	require.True(t, hasResource, "terminal-output resource should be defined")
}
