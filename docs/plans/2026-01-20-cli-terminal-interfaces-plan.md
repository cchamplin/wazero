# CLI Terminal Interfaces Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement missing WASI P2 CLI terminal interfaces (terminal-stdin, terminal-stdout, terminal-stderr) to enable Go-compiled WebAssembly components to run on wazero.

**Architecture:** Extend WASIConfig interface with TerminalMode (None/Auto/Custom) configuration. Implement three new interfaces that return option<terminal-input/output> based on the configured mode. Default to None (safe for sandboxed environments).

**Tech Stack:** Go, golang.org/x/term for TTY detection, wazero component model

---

## Task 1: Add golang.org/x/term dependency

**Files:**
- Modify: `go.mod`

**Step 1: Add the dependency**

Run:
```bash
go get golang.org/x/term
```

**Step 2: Verify it's added**

Run:
```bash
grep "golang.org/x/term" go.mod
```

Expected: Shows the term dependency line

**Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "deps: add golang.org/x/term for TTY detection"
```

---

## Task 2: Extend WASIConfig interface with TerminalMode

**Files:**
- Modify: `internal/component/context.go`

**Step 2.1: Add TerminalMode type and extend interface**

Add after line 8 (imports) and before line 10 (contextKey):

```go
// TerminalMode controls how terminal detection behaves for WASI CLI interfaces.
type TerminalMode int

const (
	// TerminalModeNone always reports no terminal connection (safe default).
	TerminalModeNone TerminalMode = iota
	// TerminalModeAuto detects real TTY by checking if streams are *os.File
	// with valid terminal file descriptors.
	TerminalModeAuto
	// TerminalModeCustom uses explicit host-provided values.
	TerminalModeCustom
)
```

Then update WASIConfig interface (around line 32-45) to add terminal methods:

```go
// WASIConfig is an interface for WASI configuration that can be stored in context.
// This interface is defined here to avoid import cycles between wasip2 and cli packages.
type WASIConfig interface {
	// Stdin returns the stdin reader.
	Stdin() io.Reader
	// Stdout returns the stdout writer.
	Stdout() io.Writer
	// Stderr returns the stderr writer.
	Stderr() io.Writer
	// Environ returns environment variables as "KEY=value" strings.
	Environ() []string
	// Args returns command-line arguments.
	Args() []string
	// TerminalMode returns the terminal detection mode.
	TerminalMode() TerminalMode
	// StdinIsTerminal returns true if stdin is a terminal (used with TerminalModeCustom).
	StdinIsTerminal() bool
	// StdoutIsTerminal returns true if stdout is a terminal (used with TerminalModeCustom).
	StdoutIsTerminal() bool
	// StderrIsTerminal returns true if stderr is a terminal (used with TerminalModeCustom).
	StderrIsTerminal() bool
}
```

**Step 2.2: Run existing tests to verify interface change is detected**

Run:
```bash
CGO_ENABLED=0 go build ./internal/component/...
```

Expected: Build failures because Config implementations don't have new methods yet

**Step 2.3: Commit interface change**

```bash
git add internal/component/context.go
git commit -m "feat(component): extend WASIConfig with TerminalMode support"
```

---

## Task 3: Update testConfig in cli_test.go to implement new interface

**Files:**
- Modify: `imports/wasip2/cli/cli_test.go`

**Step 3.1: Extend testConfig struct and methods**

After line 25 (end of testConfig struct), add new fields:

```go
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
```

After line 31 (Args method), add new methods:

```go
func (c *testConfig) TerminalMode() component.TerminalMode {
	return c.terminalMode
}
func (c *testConfig) StdinIsTerminal() bool  { return c.stdinIsTerminal }
func (c *testConfig) StdoutIsTerminal() bool { return c.stdoutIsTerminal }
func (c *testConfig) StderrIsTerminal() bool { return c.stderrIsTerminal }
```

**Step 3.2: Verify cli tests compile**

Run:
```bash
CGO_ENABLED=0 go build ./imports/wasip2/cli/...
```

Expected: Still fails because wasip2.Config doesn't implement the interface

**Step 3.3: Commit test config update**

```bash
git add imports/wasip2/cli/cli_test.go
git commit -m "test(cli): extend testConfig with terminal mode support"
```

---

## Task 4: Update wasip2.Config to implement new interface methods

**Files:**
- Modify: `imports/wasip2/config.go`

**Step 4.1: Add terminal fields to Config struct**

After line 43 (allowHTTP field), add:

```go
// Config configures WASI Preview 2 behavior.
// It implements the component.WASIConfig interface.
type Config struct {
	stdin   io.Reader
	stdout  io.Writer
	stderr  io.Writer
	environ func() []string
	args    func() []string
	preopens map[string]string

	// Feature flags
	allowNetwork bool
	allowHTTP    bool

	// Terminal configuration
	terminalMode     component.TerminalMode
	stdinIsTerminal  bool
	stdoutIsTerminal bool
	stderrIsTerminal bool
}
```

**Step 4.2: Add interface methods at end of file**

After line 147 (AllowHTTP method), add:

```go
// TerminalMode returns the terminal detection mode.
func (c *Config) TerminalMode() component.TerminalMode { return c.terminalMode }

// StdinIsTerminal returns whether stdin is configured as a terminal.
func (c *Config) StdinIsTerminal() bool { return c.stdinIsTerminal }

// StdoutIsTerminal returns whether stdout is configured as a terminal.
func (c *Config) StdoutIsTerminal() bool { return c.stdoutIsTerminal }

// StderrIsTerminal returns whether stderr is configured as a terminal.
func (c *Config) StderrIsTerminal() bool { return c.stderrIsTerminal }

// WithTerminalMode sets the terminal detection mode.
func (c *Config) WithTerminalMode(mode component.TerminalMode) *Config {
	c.terminalMode = mode
	return c
}

// WithStdinIsTerminal explicitly sets whether stdin is a terminal.
// Only used when TerminalMode is TerminalModeCustom.
func (c *Config) WithStdinIsTerminal(isTerminal bool) *Config {
	c.stdinIsTerminal = isTerminal
	return c
}

// WithStdoutIsTerminal explicitly sets whether stdout is a terminal.
// Only used when TerminalMode is TerminalModeCustom.
func (c *Config) WithStdoutIsTerminal(isTerminal bool) *Config {
	c.stdoutIsTerminal = isTerminal
	return c
}

// WithStderrIsTerminal explicitly sets whether stderr is a terminal.
// Only used when TerminalMode is TerminalModeCustom.
func (c *Config) WithStderrIsTerminal(isTerminal bool) *Config {
	c.stderrIsTerminal = isTerminal
	return c
}
```

**Step 4.3: Verify build succeeds**

Run:
```bash
CGO_ENABLED=0 go build ./imports/wasip2/...
```

Expected: PASS (build succeeds)

**Step 4.4: Commit config implementation**

```bash
git add imports/wasip2/config.go
git commit -m "feat(wasip2): implement terminal mode configuration"
```

---

## Task 5: Write failing tests for terminal-stdin interface registration

**Files:**
- Modify: `imports/wasip2/cli/cli_test.go`

**Step 5.1: Add test for interface registration**

Add at end of file:

```go
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
```

**Step 5.2: Run test to verify it fails**

Run:
```bash
CGO_ENABLED=0 go test ./imports/wasip2/cli/... -run TestInstantiateTerminalStdin -v
```

Expected: FAIL with "undefined: instantiateTerminalStdin"

**Step 5.3: Commit failing test**

```bash
git add imports/wasip2/cli/cli_test.go
git commit -m "test(cli): add failing test for terminal-stdin interface registration"
```

---

## Task 6: Write failing tests for terminal-stdout and terminal-stderr registration

**Files:**
- Modify: `imports/wasip2/cli/cli_test.go`

**Step 6.1: Add tests for stdout and stderr interface registration**

Add after TestInstantiateTerminalStdin:

```go
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
```

**Step 6.2: Commit failing tests**

```bash
git add imports/wasip2/cli/cli_test.go
git commit -m "test(cli): add failing tests for terminal-stdout/stderr interface registration"
```

---

## Task 7: Write failing tests for get-terminal-stdin behavior

**Files:**
- Modify: `imports/wasip2/cli/cli_test.go`

**Step 7.1: Add behavior tests for all terminal modes**

Add after registration tests:

```go
func TestGetTerminalStdin_NoConfig(t *testing.T) {
	// Without config, returns None
	result, err := getTerminalStdin(context.Background(), []component.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindOption, result[0].Kind())
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

	result, err := getTerminalStdin(ctx, []component.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindOption, result[0].Kind())
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

	result, err := getTerminalStdin(ctx, []component.Val{})
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
	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)
	ctx = component.WithWASIConfig(ctx, config)

	result, err := getTerminalStdin(ctx, []component.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindOption, result[0].Kind())
	opt := result[0].Option()
	require.NotNil(t, opt, "should return Some when stdinIsTerminal=true")
	require.Equal(t, component.ValKindOwn, opt.Kind())

	// Verify the resource was created in the table
	handle := opt.Own()
	entry, err := table.Get(component.Handle(handle))
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

	result, err := getTerminalStdin(ctx, []component.Val{})
	require.NoError(t, err)
	opt := result[0].Option()
	require.Nil(t, opt, "should return None when stdin is not an *os.File")
}
```

**Step 7.2: Run tests to verify they fail**

Run:
```bash
CGO_ENABLED=0 go test ./imports/wasip2/cli/... -run "TestGetTerminalStdin" -v
```

Expected: FAIL with "undefined: getTerminalStdin" and "undefined: TerminalInput"

**Step 7.3: Commit failing tests**

```bash
git add imports/wasip2/cli/cli_test.go
git commit -m "test(cli): add failing tests for get-terminal-stdin behavior"
```

---

## Task 8: Write failing tests for get-terminal-stdout behavior

**Files:**
- Modify: `imports/wasip2/cli/cli_test.go`

**Step 8.1: Add behavior tests**

Add after stdin tests:

```go
func TestGetTerminalStdout_NoConfig(t *testing.T) {
	result, err := getTerminalStdout(context.Background(), []component.Val{})
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

	result, err := getTerminalStdout(ctx, []component.Val{})
	require.NoError(t, err)
	opt := result[0].Option()
	require.Nil(t, opt, "should return None with TerminalModeNone")
}

func TestGetTerminalStdout_ModeCustom_True(t *testing.T) {
	config := &testConfig{
		stdout:            &bytes.Buffer{},
		terminalMode:      component.TerminalModeCustom,
		stdoutIsTerminal:  true,
	}
	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)
	ctx = component.WithWASIConfig(ctx, config)

	result, err := getTerminalStdout(ctx, []component.Val{})
	require.NoError(t, err)
	opt := result[0].Option()
	require.NotNil(t, opt, "should return Some when stdoutIsTerminal=true")

	handle := opt.Own()
	entry, err := table.Get(component.Handle(handle))
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

	result, err := getTerminalStdout(ctx, []component.Val{})
	require.NoError(t, err)
	opt := result[0].Option()
	require.Nil(t, opt, "should return None when stdout is not an *os.File")
}
```

**Step 8.2: Commit failing tests**

```bash
git add imports/wasip2/cli/cli_test.go
git commit -m "test(cli): add failing tests for get-terminal-stdout behavior"
```

---

## Task 9: Write failing tests for get-terminal-stderr behavior

**Files:**
- Modify: `imports/wasip2/cli/cli_test.go`

**Step 9.1: Add behavior tests**

Add after stdout tests:

```go
func TestGetTerminalStderr_NoConfig(t *testing.T) {
	result, err := getTerminalStderr(context.Background(), []component.Val{})
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

	result, err := getTerminalStderr(ctx, []component.Val{})
	require.NoError(t, err)
	opt := result[0].Option()
	require.Nil(t, opt, "should return None with TerminalModeNone")
}

func TestGetTerminalStderr_ModeCustom_True(t *testing.T) {
	config := &testConfig{
		stderr:            &bytes.Buffer{},
		terminalMode:      component.TerminalModeCustom,
		stderrIsTerminal:  true,
	}
	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)
	ctx = component.WithWASIConfig(ctx, config)

	result, err := getTerminalStderr(ctx, []component.Val{})
	require.NoError(t, err)
	opt := result[0].Option()
	require.NotNil(t, opt, "should return Some when stderrIsTerminal=true")

	handle := opt.Own()
	entry, err := table.Get(component.Handle(handle))
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

	result, err := getTerminalStderr(ctx, []component.Val{})
	require.NoError(t, err)
	opt := result[0].Option()
	require.Nil(t, opt, "should return None when stderr is not an *os.File")
}
```

**Step 9.2: Commit failing tests**

```bash
git add imports/wasip2/cli/cli_test.go
git commit -m "test(cli): add failing tests for get-terminal-stderr behavior"
```

---

## Task 10: Create terminal.go with types and detection helper

**Files:**
- Create: `imports/wasip2/cli/terminal.go`

**Step 10.1: Create the terminal.go file**

```go
// Package cli implements the wasi:cli interfaces for WASI Preview 2.
package cli

import (
	"os"

	"golang.org/x/term"
)

// TerminalInput is a marker resource for terminal input handles.
// Currently has no methods per WASI spec - future versions may add
// echo control, buffering settings, etc.
type TerminalInput struct{}

// TerminalOutput is a marker resource for terminal output handles.
// Currently has no methods per WASI spec - future versions may add
// terminal size queries, resize notifications, etc.
type TerminalOutput struct{}

// detectTerminal checks if the given reader/writer is connected to a TTY.
// Returns true only if it's an *os.File with a valid terminal fd.
func detectTerminal(stream interface{}) bool {
	// Try to get the underlying file descriptor
	var fd int

	switch s := stream.(type) {
	case *os.File:
		fd = int(s.Fd())
	case interface{ Fd() uintptr }:
		// Support wrappers that expose Fd()
		fd = int(s.Fd())
	default:
		return false
	}

	return term.IsTerminal(fd)
}
```

**Step 10.2: Verify it compiles**

Run:
```bash
CGO_ENABLED=0 go build ./imports/wasip2/cli/...
```

Expected: Still fails because tests reference undefined functions, but terminal.go compiles

**Step 10.3: Commit terminal.go**

```bash
git add imports/wasip2/cli/terminal.go
git commit -m "feat(cli): add terminal resource types and TTY detection helper"
```

---

## Task 11: Implement instantiateTerminalStdin

**Files:**
- Modify: `imports/wasip2/cli/cli.go`

**Step 11.1: Add the instantiate function**

Add after instantiateTerminalOutput function (around line 261):

```go
// instantiateTerminalStdin registers wasi:cli/terminal-stdin@0.2.0
func instantiateTerminalStdin(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:cli/terminal-stdin@0.2.0")

	// get-terminal-stdin: func() -> option<own<terminal-input>>
	inst.FuncNoType("get-terminal-stdin", getTerminalStdin)

	return inst.Build()
}

// getTerminalStdin returns Some(terminal-input) if stdin is a terminal, None otherwise
func getTerminalStdin(ctx context.Context, args []component.Val) ([]component.Val, error) {
	config := component.WASIConfigFromContext(ctx)
	if config == nil {
		return []component.Val{component.ValOption(nil)}, nil
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
		return []component.Val{component.ValOption(nil)}, nil
	}

	// Create terminal-input resource and return Some(handle)
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return []component.Val{component.ValOption(nil)}, nil
	}
	handle := table.New(&TerminalInput{}, true)
	val := component.ValOwn(uint32(handle))
	return []component.Val{component.ValOption(&val)}, nil
}
```

**Step 11.2: Run tests**

Run:
```bash
CGO_ENABLED=0 go test ./imports/wasip2/cli/... -run "TestInstantiateTerminalStdin|TestGetTerminalStdin" -v
```

Expected: PASS

**Step 11.3: Commit implementation**

```bash
git add imports/wasip2/cli/cli.go
git commit -m "feat(cli): implement terminal-stdin interface"
```

---

## Task 12: Implement instantiateTerminalStdout

**Files:**
- Modify: `imports/wasip2/cli/cli.go`

**Step 12.1: Add the instantiate function**

Add after getTerminalStdin:

```go
// instantiateTerminalStdout registers wasi:cli/terminal-stdout@0.2.0
func instantiateTerminalStdout(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:cli/terminal-stdout@0.2.0")

	// get-terminal-stdout: func() -> option<own<terminal-output>>
	inst.FuncNoType("get-terminal-stdout", getTerminalStdout)

	return inst.Build()
}

// getTerminalStdout returns Some(terminal-output) if stdout is a terminal, None otherwise
func getTerminalStdout(ctx context.Context, args []component.Val) ([]component.Val, error) {
	config := component.WASIConfigFromContext(ctx)
	if config == nil {
		return []component.Val{component.ValOption(nil)}, nil
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
		return []component.Val{component.ValOption(nil)}, nil
	}

	// Create terminal-output resource and return Some(handle)
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return []component.Val{component.ValOption(nil)}, nil
	}
	handle := table.New(&TerminalOutput{}, true)
	val := component.ValOwn(uint32(handle))
	return []component.Val{component.ValOption(&val)}, nil
}
```

**Step 12.2: Run tests**

Run:
```bash
CGO_ENABLED=0 go test ./imports/wasip2/cli/... -run "TestInstantiateTerminalStdout|TestGetTerminalStdout" -v
```

Expected: PASS

**Step 12.3: Commit implementation**

```bash
git add imports/wasip2/cli/cli.go
git commit -m "feat(cli): implement terminal-stdout interface"
```

---

## Task 13: Implement instantiateTerminalStderr

**Files:**
- Modify: `imports/wasip2/cli/cli.go`

**Step 13.1: Add the instantiate function**

Add after getTerminalStdout:

```go
// instantiateTerminalStderr registers wasi:cli/terminal-stderr@0.2.0
func instantiateTerminalStderr(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:cli/terminal-stderr@0.2.0")

	// get-terminal-stderr: func() -> option<own<terminal-output>>
	inst.FuncNoType("get-terminal-stderr", getTerminalStderr)

	return inst.Build()
}

// getTerminalStderr returns Some(terminal-output) if stderr is a terminal, None otherwise
func getTerminalStderr(ctx context.Context, args []component.Val) ([]component.Val, error) {
	config := component.WASIConfigFromContext(ctx)
	if config == nil {
		return []component.Val{component.ValOption(nil)}, nil
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
		return []component.Val{component.ValOption(nil)}, nil
	}

	// Create terminal-output resource and return Some(handle)
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return []component.Val{component.ValOption(nil)}, nil
	}
	handle := table.New(&TerminalOutput{}, true)
	val := component.ValOwn(uint32(handle))
	return []component.Val{component.ValOption(&val)}, nil
}
```

**Step 13.2: Run tests**

Run:
```bash
CGO_ENABLED=0 go test ./imports/wasip2/cli/... -run "TestInstantiateTerminalStderr|TestGetTerminalStderr" -v
```

Expected: PASS

**Step 13.3: Commit implementation**

```bash
git add imports/wasip2/cli/cli.go
git commit -m "feat(cli): implement terminal-stderr interface"
```

---

## Task 14: Update Instantiate() and TestInstantiate

**Files:**
- Modify: `imports/wasip2/cli/cli.go`
- Modify: `imports/wasip2/cli/cli_test.go`

**Step 14.1: Update Instantiate function**

Modify Instantiate() to add the new interfaces (around line 27-51):

```go
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
```

**Step 14.2: Update TestInstantiate interfaces list**

In TestInstantiate (around line 39-47), update the interfaces list:

```go
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
```

**Step 14.3: Run all CLI tests**

Run:
```bash
CGO_ENABLED=0 go test ./imports/wasip2/cli/... -v
```

Expected: All PASS

**Step 14.4: Commit**

```bash
git add imports/wasip2/cli/cli.go imports/wasip2/cli/cli_test.go
git commit -m "feat(cli): register all terminal interfaces in Instantiate"
```

---

## Task 15: Run Go plugin integration test

**Files:**
- Test: `internal/component/wasip2test/calculator_test.go`

**Step 15.1: Run the calculator test with all plugins**

Run:
```bash
CGO_ENABLED=0 go test ./internal/component/wasip2test/... -run TestCalculatorPlugins -v
```

Expected: All three plugins pass (add, subtract, multi)

**Step 15.2: If tests pass, commit any needed fixes**

If any adjustments were needed:
```bash
git add -A
git commit -m "fix(cli): adjust terminal interfaces for Go plugin compatibility"
```

---

## Task 16: Run full test suite to ensure no regressions

**Step 16.1: Run all wasip2 tests**

Run:
```bash
CGO_ENABLED=0 go test ./imports/wasip2/... -v
```

Expected: All PASS

**Step 16.2: Run component tests**

Run:
```bash
CGO_ENABLED=0 go test ./internal/component/... -v
```

Expected: All PASS

**Step 16.3: Final commit if needed**

```bash
git status
# If clean, no commit needed
```

---

## Summary

This plan implements:
1. **TerminalMode configuration** - None (default), Auto, Custom modes
2. **Three new WASI P2 interfaces** - terminal-stdin, terminal-stdout, terminal-stderr
3. **Full TDD approach** - All tests written before implementation
4. **Backwards compatible** - Default TerminalModeNone preserves existing behavior
5. **Dependency** - golang.org/x/term for cross-platform TTY detection
