// Package cli implements the wasi:cli interfaces for WASI Preview 2.
// It provides environment, arguments, standard streams, and terminal access.
package cli

import (
	"context"
	"os"
	"strings"

	wasip2io "github.com/tetratelabs/wazero/imports/wasip2/io"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
)

// Host-managed resource type singletons. One *ResourceType per host
// resource kind. Impl is nil because these resources are host-owned.
var (
	cliInputStreamResourceType    = &runtime.ResourceType{}
	cliOutputStreamResourceType   = &runtime.ResourceType{}
	cliTerminalInputResourceType  = &runtime.ResourceType{}
	cliTerminalOutputResourceType = &runtime.ResourceType{}
)

// ExitError is returned when a WASI program calls exit with an error status.
type ExitError struct {
	// Code is the exit code. 0 means success, non-zero means failure.
	Code int
}

func (e *ExitError) Error() string {
	if e.Code == 0 {
		return "exit: success"
	}
	return "exit: failure"
}

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
	if err := instantiateTerminalStdin(linker); err != nil {
		return err
	}
	if err := instantiateTerminalStdout(linker); err != nil {
		return err
	}
	if err := instantiateTerminalStderr(linker); err != nil {
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

	return inst.SkipValidation().Build()
}

// getEnvironment returns the environment variables as list<tuple<string, string>>
func getEnvironment(ctx context.Context, args []types.Val) ([]types.Val, error) {
	config := component.WASIConfigFromContext(ctx)
	if config == nil {
		// No config, return empty list
		return []types.Val{types.ValList([]types.Val{})}, nil
	}

	environ := config.Environ()
	tuples := make([]types.Val, 0, len(environ))
	for _, env := range environ {
		// Split "KEY=value" into (key, value) tuple
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			tuple := types.ValTuple([]types.Val{
				types.ValString(parts[0]),
				types.ValString(parts[1]),
			})
			tuples = append(tuples, tuple)
		}
	}

	return []types.Val{types.ValList(tuples)}, nil
}

// getArguments returns command line arguments as list<string>
func getArguments(ctx context.Context, args []types.Val) ([]types.Val, error) {
	config := component.WASIConfigFromContext(ctx)
	if config == nil {
		// No config, return empty list
		return []types.Val{types.ValList([]types.Val{})}, nil
	}

	configArgs := config.Args()
	stringVals := make([]types.Val, len(configArgs))
	for i, arg := range configArgs {
		stringVals[i] = types.ValString(arg)
	}

	return []types.Val{types.ValList(stringVals)}, nil
}

// initialCwd returns the initial working directory as option<string>
func initialCwd(ctx context.Context, args []types.Val) ([]types.Val, error) {
	cwd, err := os.Getwd()
	if err != nil {
		// Unable to get cwd, return None
		return []types.Val{types.ValOption(nil)}, nil
	}

	cwdVal := types.ValString(cwd)
	return []types.Val{types.ValOption(&cwdVal)}, nil
}

// instantiateExit registers wasi:cli/exit@0.2.0
func instantiateExit(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:cli/exit@0.2.0")

	inst.FuncNoType("exit", exit)

	return inst.SkipValidation().Build()
}

// exit handles program termination with result<_, _>
func exit(ctx context.Context, args []types.Val) ([]types.Val, error) {
	// The argument is result<_, _> where ok (discriminant 0) means success, error (discriminant 1) means failure
	if len(args) > 0 {
		isOk, _, _ := args[0].Result()
		if !isOk {
			// Exit with error - return an error that the runtime can catch
			return nil, &ExitError{Code: 1}
		}
	}
	// Exit with success - also signal via error for consistency
	return nil, &ExitError{Code: 0}
}

// instantiateStdin registers wasi:cli/stdin@0.2.0
func instantiateStdin(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:cli/stdin@0.2.0")

	// get-stdin: func() -> own<input-stream>
	inst.FuncNoType("get-stdin", getStdin)

	return inst.SkipValidation().Build()
}

// getStdin returns stdin as own<input-stream>
func getStdin(ctx context.Context, args []types.Val) ([]types.Val, error) {
	table := component.ResourceTableFromContext(ctx)
	config := component.WASIConfigFromContext(ctx)

	if table == nil || config == nil || config.Stdin() == nil {
		// No table or config, return placeholder handle 0
		return []types.Val{types.ValOwn(0)}, nil
	}

	// Create an InputStream from the config's stdin reader
	stream := wasip2io.NewInputStream(config.Stdin())

	// Register in resource table and get handle
	handle, hErr := table.NewResourceHandle(stream, true, cliInputStreamResourceType)
	if hErr != nil {
		return []types.Val{types.ValOwn(0)}, nil
	}
	return []types.Val{types.ValOwn(uint32(handle))}, nil
}

// instantiateStdout registers wasi:cli/stdout@0.2.0
func instantiateStdout(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:cli/stdout@0.2.0")

	// get-stdout: func() -> own<output-stream>
	inst.FuncNoType("get-stdout", getStdout)

	return inst.SkipValidation().Build()
}

// getStdout returns stdout as own<output-stream>
func getStdout(ctx context.Context, args []types.Val) ([]types.Val, error) {
	table := component.ResourceTableFromContext(ctx)
	config := component.WASIConfigFromContext(ctx)

	if table == nil || config == nil || config.Stdout() == nil {
		// No table or config, return placeholder handle 1
		return []types.Val{types.ValOwn(1)}, nil
	}

	// Create an OutputStream from the config's stdout writer
	stream := wasip2io.NewOutputStream(config.Stdout())

	// Register in resource table and get handle
	handle, hErr := table.NewResourceHandle(stream, true, cliOutputStreamResourceType)
	if hErr != nil {
		return []types.Val{types.ValOwn(1)}, nil
	}
	return []types.Val{types.ValOwn(uint32(handle))}, nil
}

// instantiateStderr registers wasi:cli/stderr@0.2.0
func instantiateStderr(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:cli/stderr@0.2.0")

	// get-stderr: func() -> own<output-stream>
	inst.FuncNoType("get-stderr", getStderr)

	return inst.SkipValidation().Build()
}

// getStderr returns stderr as own<output-stream>
func getStderr(ctx context.Context, args []types.Val) ([]types.Val, error) {
	table := component.ResourceTableFromContext(ctx)
	config := component.WASIConfigFromContext(ctx)

	if table == nil || config == nil || config.Stderr() == nil {
		// No table or config, return placeholder handle 2
		return []types.Val{types.ValOwn(2)}, nil
	}

	// Create an OutputStream from the config's stderr writer
	stream := wasip2io.NewOutputStream(config.Stderr())

	// Register in resource table and get handle
	handle, hErr := table.NewResourceHandle(stream, true, cliOutputStreamResourceType)
	if hErr != nil {
		return []types.Val{types.ValOwn(2)}, nil
	}
	return []types.Val{types.ValOwn(uint32(handle))}, nil
}

// instantiateTerminalInput registers wasi:cli/terminal-input@0.2.0
func instantiateTerminalInput(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:cli/terminal-input@0.2.0")

	// Define terminal-input resource with no-op destructor
	inst.Resource("terminal-input", func(rep uint32) {})

	return inst.SkipValidation().Build()
}

// instantiateTerminalOutput registers wasi:cli/terminal-output@0.2.0
func instantiateTerminalOutput(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:cli/terminal-output@0.2.0")

	// Define terminal-output resource with no-op destructor
	inst.Resource("terminal-output", func(rep uint32) {})

	return inst.SkipValidation().Build()
}

// instantiateTerminalStdin registers wasi:cli/terminal-stdin@0.2.0
func instantiateTerminalStdin(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:cli/terminal-stdin@0.2.0")

	// get-terminal-stdin: func() -> option<own<terminal-input>>
	inst.FuncNoType("get-terminal-stdin", getTerminalStdin)

	return inst.SkipValidation().Build()
}

// getTerminalStdin returns Some(terminal-input) if stdin is a terminal, None otherwise
func getTerminalStdin(ctx context.Context, args []types.Val) ([]types.Val, error) {
	config := component.WASIConfigFromContext(ctx)
	if config == nil {
		return []types.Val{types.ValOption(nil)}, nil
	}

	isTerminal := false
	switch config.TerminalMode() {
	case component.TerminalModeNone:
		isTerminal = false
	case component.TerminalModeAuto:
		isTerminal = detectTerminal(config.Stdin())
	case component.TerminalModeCustom:
		isTerminal = config.StdinIsTerminal()
	}

	if !isTerminal {
		return []types.Val{types.ValOption(nil)}, nil
	}

	// Create terminal-input resource and return Some(handle)
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return []types.Val{types.ValOption(nil)}, nil
	}
	handle, hErr := table.NewResourceHandle(&TerminalInput{}, true, cliTerminalInputResourceType)
	if hErr != nil {
		return []types.Val{types.ValOption(nil)}, nil
	}
	val := types.ValOwn(uint32(handle))
	return []types.Val{types.ValOption(&val)}, nil
}

// instantiateTerminalStdout registers wasi:cli/terminal-stdout@0.2.0
func instantiateTerminalStdout(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:cli/terminal-stdout@0.2.0")

	// get-terminal-stdout: func() -> option<own<terminal-output>>
	inst.FuncNoType("get-terminal-stdout", getTerminalStdout)

	return inst.SkipValidation().Build()
}

// getTerminalStdout returns Some(terminal-output) if stdout is a terminal, None otherwise
func getTerminalStdout(ctx context.Context, args []types.Val) ([]types.Val, error) {
	config := component.WASIConfigFromContext(ctx)
	if config == nil {
		return []types.Val{types.ValOption(nil)}, nil
	}

	isTerminal := false
	switch config.TerminalMode() {
	case component.TerminalModeNone:
		isTerminal = false
	case component.TerminalModeAuto:
		isTerminal = detectTerminal(config.Stdout())
	case component.TerminalModeCustom:
		isTerminal = config.StdoutIsTerminal()
	}

	if !isTerminal {
		return []types.Val{types.ValOption(nil)}, nil
	}

	// Create terminal-output resource and return Some(handle)
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return []types.Val{types.ValOption(nil)}, nil
	}
	handle, hErr := table.NewResourceHandle(&TerminalOutput{}, true, cliTerminalOutputResourceType)
	if hErr != nil {
		return []types.Val{types.ValOption(nil)}, nil
	}
	val := types.ValOwn(uint32(handle))
	return []types.Val{types.ValOption(&val)}, nil
}

// instantiateTerminalStderr registers wasi:cli/terminal-stderr@0.2.0
func instantiateTerminalStderr(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:cli/terminal-stderr@0.2.0")

	// get-terminal-stderr: func() -> option<own<terminal-output>>
	inst.FuncNoType("get-terminal-stderr", getTerminalStderr)

	return inst.SkipValidation().Build()
}

// getTerminalStderr returns Some(terminal-output) if stderr is a terminal, None otherwise
func getTerminalStderr(ctx context.Context, args []types.Val) ([]types.Val, error) {
	config := component.WASIConfigFromContext(ctx)
	if config == nil {
		return []types.Val{types.ValOption(nil)}, nil
	}

	isTerminal := false
	switch config.TerminalMode() {
	case component.TerminalModeNone:
		isTerminal = false
	case component.TerminalModeAuto:
		isTerminal = detectTerminal(config.Stderr())
	case component.TerminalModeCustom:
		isTerminal = config.StderrIsTerminal()
	}

	if !isTerminal {
		return []types.Val{types.ValOption(nil)}, nil
	}

	// Create terminal-output resource and return Some(handle)
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return []types.Val{types.ValOption(nil)}, nil
	}
	handle, hErr := table.NewResourceHandle(&TerminalOutput{}, true, cliTerminalOutputResourceType)
	if hErr != nil {
		return []types.Val{types.ValOption(nil)}, nil
	}
	val := types.ValOwn(uint32(handle))
	return []types.Val{types.ValOption(&val)}, nil
}
