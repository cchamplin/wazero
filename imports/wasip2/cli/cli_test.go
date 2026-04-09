// imports/wasip2/cli/cli_test.go

package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	wasip2io "github.com/tetratelabs/wazero/imports/wasip2/io"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// testConfig implements component.WASIConfig for testing
type testConfig struct {
	stdin            io.Reader
	stdout           io.Writer
	stderr           io.Writer
	environ          []string
	args             []string
	terminalMode     component.TerminalMode
	stdinIsTerminal  bool
	stdoutIsTerminal bool
	stderrIsTerminal bool
}

func (c *testConfig) Stdin() io.Reader  { return c.stdin }
func (c *testConfig) Stdout() io.Writer { return c.stdout }
func (c *testConfig) Stderr() io.Writer { return c.stderr }
func (c *testConfig) Environ() []string { return c.environ }
func (c *testConfig) Args() []string    { return c.args }
func (c *testConfig) TerminalMode() component.TerminalMode {
	return c.terminalMode
}
func (c *testConfig) StdinIsTerminal() bool  { return c.stdinIsTerminal }
func (c *testConfig) StdoutIsTerminal() bool { return c.stdoutIsTerminal }
func (c *testConfig) StderrIsTerminal() bool { return c.stderrIsTerminal }

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
		"wasi:cli/terminal-stdin@0.2.0",
		"wasi:cli/terminal-stdout@0.2.0",
		"wasi:cli/terminal-stderr@0.2.0",
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

func TestGetEnvironment_NoConfig(t *testing.T) {
	// Without config, returns empty list
	result, err := getEnvironment(context.Background(), nil, []types.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindList, result[0].Kind())
	list := result[0].List()
	require.NotNil(t, list)
	require.Equal(t, 0, len(list))
}

func TestGetEnvironment_WithConfig(t *testing.T) {
	// Create config with environment variables
	config := &testConfig{
		environ: []string{"FOO=bar", "BAZ=qux", "EMPTY="},
	}

	ctx := component.WithWASIConfig(context.Background(), config)

	result, err := getEnvironment(ctx, nil, []types.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindList, result[0].Kind())

	list := result[0].List()
	require.Equal(t, 3, len(list))

	// Check first tuple (FOO, bar)
	tuple0 := list[0].Tuple()
	require.Equal(t, 2, len(tuple0))
	require.Equal(t, "FOO", tuple0[0].StringVal())
	require.Equal(t, "bar", tuple0[1].StringVal())

	// Check second tuple (BAZ, qux)
	tuple1 := list[1].Tuple()
	require.Equal(t, 2, len(tuple1))
	require.Equal(t, "BAZ", tuple1[0].StringVal())
	require.Equal(t, "qux", tuple1[1].StringVal())

	// Check third tuple (EMPTY, "")
	tuple2 := list[2].Tuple()
	require.Equal(t, 2, len(tuple2))
	require.Equal(t, "EMPTY", tuple2[0].StringVal())
	require.Equal(t, "", tuple2[1].StringVal())
}

func TestGetArguments_NoConfig(t *testing.T) {
	// Without config, returns empty list
	result, err := getArguments(context.Background(), nil, []types.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindList, result[0].Kind())
	list := result[0].List()
	require.NotNil(t, list)
	require.Equal(t, 0, len(list))
}

func TestGetArguments_WithConfig(t *testing.T) {
	// Create config with arguments
	config := &testConfig{
		args: []string{"prog", "--flag", "value"},
	}

	ctx := component.WithWASIConfig(context.Background(), config)

	result, err := getArguments(ctx, nil, []types.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindList, result[0].Kind())

	list := result[0].List()
	require.Equal(t, 3, len(list))
	require.Equal(t, "prog", list[0].StringVal())
	require.Equal(t, "--flag", list[1].StringVal())
	require.Equal(t, "value", list[2].StringVal())
}

func TestInitialCwd(t *testing.T) {
	// initialCwd uses os.Getwd(), so it should return a real directory
	result, err := initialCwd(context.Background(), nil, []types.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOption, result[0].Kind())

	opt := result[0].Option()
	require.NotNil(t, opt, "initial-cwd should return Some with current directory")

	cwd := opt.StringVal()
	expectedCwd, err := os.Getwd()
	require.NoError(t, err)
	require.Equal(t, expectedCwd, cwd)
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

func TestExit_Success(t *testing.T) {
	// With ok result - should return ExitError with code 0
	okResult := types.ValResultOk(nil)
	result, err := exit(context.Background(), nil, []types.Val{okResult})
	require.Nil(t, result)

	exitErr, ok := err.(*ExitError)
	require.True(t, ok, "expected ExitError")
	require.Equal(t, 0, exitErr.Code)
	require.Equal(t, "exit: success", exitErr.Error())
}

func TestExit_Failure(t *testing.T) {
	// With error result - should return ExitError with code 1
	errResult := types.ValResultError(nil)
	result, err := exit(context.Background(), nil, []types.Val{errResult})
	require.Nil(t, result)

	exitErr, ok := err.(*ExitError)
	require.True(t, ok, "expected ExitError")
	require.Equal(t, 1, exitErr.Code)
	require.Equal(t, "exit: failure", exitErr.Error())
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

func TestGetStdin_NoConfig(t *testing.T) {
	// Without config or table, returns placeholder handle 0
	result, err := getStdin(context.Background(), nil, []types.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOwn, result[0].Kind())
	handle := result[0].Own()
	require.Equal(t, uint32(0), handle)
}

func TestGetStdin_WithConfig(t *testing.T) {
	// Create config with stdin
	stdinData := bytes.NewReader([]byte("hello world"))
	config := &testConfig{stdin: stdinData}

	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)
	ctx = component.WithWASIConfig(ctx, config)

	result, err := getStdin(ctx, nil, []types.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOwn, result[0].Kind())

	handle := result[0].Own()
	// Should have created a real handle (not placeholder 0)
	rawEntry1, err := table.Get(runtime.Handle(handle))
	entry, _ := rawEntry1.(*runtime.ResourceHandleEntry)
	require.NoError(t, err)
	require.NotNil(t, entry)

	// Verify it's an InputStream
	stream, ok := entry.Rep.(*wasip2io.InputStream)
	require.True(t, ok, "expected InputStream resource")
	require.NotNil(t, stream)

	// Read from the stream to verify it works
	data, streamErr := stream.Read(1024)
	require.Nil(t, streamErr)
	require.Equal(t, "hello world", string(data))
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

func TestGetStdout_NoConfig(t *testing.T) {
	// Without config or table, returns placeholder handle 1
	result, err := getStdout(context.Background(), nil, []types.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOwn, result[0].Kind())
	handle := result[0].Own()
	require.Equal(t, uint32(1), handle)
}

func TestGetStdout_WithConfig(t *testing.T) {
	// Create config with stdout
	var stdoutBuf bytes.Buffer
	config := &testConfig{stdout: &stdoutBuf}

	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)
	ctx = component.WithWASIConfig(ctx, config)

	result, err := getStdout(ctx, nil, []types.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOwn, result[0].Kind())

	handle := result[0].Own()
	// Should have created a real handle
	rawEntry2, err := table.Get(runtime.Handle(handle))
	entry, _ := rawEntry2.(*runtime.ResourceHandleEntry)
	require.NoError(t, err)
	require.NotNil(t, entry)

	// Verify it's an OutputStream
	stream, ok := entry.Rep.(*wasip2io.OutputStream)
	require.True(t, ok, "expected OutputStream resource")
	require.NotNil(t, stream)

	// Write to the stream to verify it works
	streamErr := stream.Write([]byte("hello stdout"))
	require.Nil(t, streamErr)
	require.Equal(t, "hello stdout", stdoutBuf.String())
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

func TestGetStderr_NoConfig(t *testing.T) {
	// Without config or table, returns placeholder handle 2
	result, err := getStderr(context.Background(), nil, []types.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOwn, result[0].Kind())
	handle := result[0].Own()
	require.Equal(t, uint32(2), handle)
}

func TestGetStderr_WithConfig(t *testing.T) {
	// Create config with stderr
	var stderrBuf bytes.Buffer
	config := &testConfig{stderr: &stderrBuf}

	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)
	ctx = component.WithWASIConfig(ctx, config)

	result, err := getStderr(ctx, nil, []types.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOwn, result[0].Kind())

	handle := result[0].Own()
	// Should have created a real handle
	rawEntry3, err := table.Get(runtime.Handle(handle))
	entry, _ := rawEntry3.(*runtime.ResourceHandleEntry)
	require.NoError(t, err)
	require.NotNil(t, entry)

	// Verify it's an OutputStream
	stream, ok := entry.Rep.(*wasip2io.OutputStream)
	require.True(t, ok, "expected OutputStream resource")
	require.NotNil(t, stream)

	// Write to the stream to verify it works
	streamErr := stream.Write([]byte("hello stderr"))
	require.Nil(t, streamErr)
	require.Equal(t, "hello stderr", stderrBuf.String())
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

// Test ExitError type
func TestExitError(t *testing.T) {
	successErr := &ExitError{Code: 0}
	require.Equal(t, "exit: success", successErr.Error())

	failureErr := &ExitError{Code: 1}
	require.Equal(t, "exit: failure", failureErr.Error())

	// Test that it implements error interface
	var err error = successErr
	require.NotNil(t, err)
	require.True(t, strings.HasPrefix(err.Error(), "exit:"))
}

// Terminal stdin interface tests
func TestInstantiateTerminalStdin(t *testing.T) {
	linker := component.NewLinker()
	err := instantiateTerminalStdin(linker)
	require.NoError(t, err)

	def, err := linker.MatchImport("wasi:cli/terminal-stdin@0.2.0")
	require.NoError(t, err)

	instDef, ok := def.(*component.InstanceDef)
	require.True(t, ok, "expected InstanceDef")

	_, hasGetTerminalStdin := instDef.Exports["get-terminal-stdin"]
	require.True(t, hasGetTerminalStdin, "get-terminal-stdin function should be defined")
}

// Terminal stdout interface tests
func TestInstantiateTerminalStdout(t *testing.T) {
	linker := component.NewLinker()
	err := instantiateTerminalStdout(linker)
	require.NoError(t, err)

	def, err := linker.MatchImport("wasi:cli/terminal-stdout@0.2.0")
	require.NoError(t, err)

	instDef, ok := def.(*component.InstanceDef)
	require.True(t, ok, "expected InstanceDef")

	_, hasGetTerminalStdout := instDef.Exports["get-terminal-stdout"]
	require.True(t, hasGetTerminalStdout, "get-terminal-stdout function should be defined")
}

// Terminal stderr interface tests
func TestInstantiateTerminalStderr(t *testing.T) {
	linker := component.NewLinker()
	err := instantiateTerminalStderr(linker)
	require.NoError(t, err)

	def, err := linker.MatchImport("wasi:cli/terminal-stderr@0.2.0")
	require.NoError(t, err)

	instDef, ok := def.(*component.InstanceDef)
	require.True(t, ok, "expected InstanceDef")

	_, hasGetTerminalStderr := instDef.Exports["get-terminal-stderr"]
	require.True(t, hasGetTerminalStderr, "get-terminal-stderr function should be defined")
}

func TestGetTerminalStdin_NoConfig(t *testing.T) {
	// Without config, returns None
	result, err := getTerminalStdin(context.Background(), nil, []types.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOption, result[0].Kind())
	opt := result[0].Option()
	require.Nil(t, opt, "should return None when no config")
}

func TestGetTerminalStdin_ModeNone(t *testing.T) {
	// With TerminalModeNone, always returns None
	config := &testConfig{
		stdin:        bytes.NewReader([]byte("test")),
		terminalMode: component.TerminalModeNone,
	}
	ctx := component.WithWASIConfig(context.Background(), config)

	result, err := getTerminalStdin(ctx, nil, []types.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOption, result[0].Kind())
	opt := result[0].Option()
	require.Nil(t, opt, "should return None with TerminalModeNone")
}

func TestGetTerminalStdin_ModeCustom_False(t *testing.T) {
	// With TerminalModeCustom and stdinIsTerminal=false, returns None
	config := &testConfig{
		stdin:           bytes.NewReader([]byte("test")),
		terminalMode:    component.TerminalModeCustom,
		stdinIsTerminal: false,
	}
	ctx := component.WithWASIConfig(context.Background(), config)

	result, err := getTerminalStdin(ctx, nil, []types.Val{})
	require.NoError(t, err)
	opt := result[0].Option()
	require.Nil(t, opt, "should return None when stdinIsTerminal=false")
}

func TestGetTerminalStdin_ModeCustom_True(t *testing.T) {
	// With TerminalModeCustom and stdinIsTerminal=true, returns Some(terminal-input)
	config := &testConfig{
		stdin:           bytes.NewReader([]byte("test")),
		terminalMode:    component.TerminalModeCustom,
		stdinIsTerminal: true,
	}
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)
	ctx = component.WithWASIConfig(ctx, config)

	result, err := getTerminalStdin(ctx, nil, []types.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOption, result[0].Kind())
	opt := result[0].Option()
	require.NotNil(t, opt, "should return Some when stdinIsTerminal=true")
	require.Equal(t, types.ValKindOwn, opt.Kind())

	// Verify the resource was created in the table
	handle := opt.Own()
	rawEntry4, err := table.Get(runtime.Handle(handle))
	entry, _ := rawEntry4.(*runtime.ResourceHandleEntry)
	require.NoError(t, err)
	require.NotNil(t, entry)
	_, ok := entry.Rep.(*TerminalInput)
	require.True(t, ok, "expected TerminalInput resource")
}

func TestGetTerminalStdin_ModeAuto_NotFile(t *testing.T) {
	// With TerminalModeAuto and non-file reader, returns None
	config := &testConfig{
		stdin:        bytes.NewReader([]byte("test")), // Not an *os.File
		terminalMode: component.TerminalModeAuto,
	}
	ctx := component.WithWASIConfig(context.Background(), config)

	result, err := getTerminalStdin(ctx, nil, []types.Val{})
	require.NoError(t, err)
	opt := result[0].Option()
	require.Nil(t, opt, "should return None when stdin is not an *os.File")
}

func TestGetTerminalStdout_NoConfig(t *testing.T) {
	result, err := getTerminalStdout(context.Background(), nil, []types.Val{})
	require.NoError(t, err)
	opt := result[0].Option()
	require.Nil(t, opt, "should return None when no config")
}

func TestGetTerminalStdout_ModeNone(t *testing.T) {
	config := &testConfig{
		stdout:       &bytes.Buffer{},
		terminalMode: component.TerminalModeNone,
	}
	ctx := component.WithWASIConfig(context.Background(), config)

	result, err := getTerminalStdout(ctx, nil, []types.Val{})
	require.NoError(t, err)
	opt := result[0].Option()
	require.Nil(t, opt, "should return None with TerminalModeNone")
}

func TestGetTerminalStdout_ModeCustom_True(t *testing.T) {
	config := &testConfig{
		stdout:           &bytes.Buffer{},
		terminalMode:     component.TerminalModeCustom,
		stdoutIsTerminal: true,
	}
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)
	ctx = component.WithWASIConfig(ctx, config)

	result, err := getTerminalStdout(ctx, nil, []types.Val{})
	require.NoError(t, err)
	opt := result[0].Option()
	require.NotNil(t, opt, "should return Some when stdoutIsTerminal=true")

	handle := opt.Own()
	rawEntry5, err := table.Get(runtime.Handle(handle))
	entry, _ := rawEntry5.(*runtime.ResourceHandleEntry)
	require.NoError(t, err)
	_, ok := entry.Rep.(*TerminalOutput)
	require.True(t, ok, "expected TerminalOutput resource")
}

func TestGetTerminalStdout_ModeAuto_NotFile(t *testing.T) {
	config := &testConfig{
		stdout:       &bytes.Buffer{}, // Not an *os.File
		terminalMode: component.TerminalModeAuto,
	}
	ctx := component.WithWASIConfig(context.Background(), config)

	result, err := getTerminalStdout(ctx, nil, []types.Val{})
	require.NoError(t, err)
	opt := result[0].Option()
	require.Nil(t, opt, "should return None when stdout is not an *os.File")
}

func TestGetTerminalStderr_NoConfig(t *testing.T) {
	result, err := getTerminalStderr(context.Background(), nil, []types.Val{})
	require.NoError(t, err)
	opt := result[0].Option()
	require.Nil(t, opt, "should return None when no config")
}

func TestGetTerminalStderr_ModeNone(t *testing.T) {
	config := &testConfig{
		stderr:       &bytes.Buffer{},
		terminalMode: component.TerminalModeNone,
	}
	ctx := component.WithWASIConfig(context.Background(), config)

	result, err := getTerminalStderr(ctx, nil, []types.Val{})
	require.NoError(t, err)
	opt := result[0].Option()
	require.Nil(t, opt, "should return None with TerminalModeNone")
}

func TestGetTerminalStderr_ModeCustom_True(t *testing.T) {
	config := &testConfig{
		stderr:           &bytes.Buffer{},
		terminalMode:     component.TerminalModeCustom,
		stderrIsTerminal: true,
	}
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)
	ctx = component.WithWASIConfig(ctx, config)

	result, err := getTerminalStderr(ctx, nil, []types.Val{})
	require.NoError(t, err)
	opt := result[0].Option()
	require.NotNil(t, opt, "should return Some when stderrIsTerminal=true")

	handle := opt.Own()
	rawEntry6, err := table.Get(runtime.Handle(handle))
	entry, _ := rawEntry6.(*runtime.ResourceHandleEntry)
	require.NoError(t, err)
	_, ok := entry.Rep.(*TerminalOutput)
	require.True(t, ok, "expected TerminalOutput resource")
}

func TestGetTerminalStderr_ModeAuto_NotFile(t *testing.T) {
	config := &testConfig{
		stderr:       &bytes.Buffer{}, // Not an *os.File
		terminalMode: component.TerminalModeAuto,
	}
	ctx := component.WithWASIConfig(context.Background(), config)

	result, err := getTerminalStderr(ctx, nil, []types.Val{})
	require.NoError(t, err)
	opt := result[0].Option()
	require.Nil(t, opt, "should return None when stderr is not an *os.File")
}
