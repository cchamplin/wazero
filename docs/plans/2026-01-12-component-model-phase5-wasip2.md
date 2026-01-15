# Component Model Phase 5: WASI Preview 2

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Parent Plan:** [2026-01-12-component-model-implementation.md](./2026-01-12-component-model-implementation.md)
**Design Doc:** [2026-01-12-component-model-design.md](./2026-01-12-component-model-design.md)
**Previous Phase:** [Phase 4: Full Instantiation & Linking](./2026-01-12-component-model-phase4-linking.md)
**Status:** COMPLETE
**Tasks:** 151-240

---

## Implementation Summary (2026-01-15)

Phase 5 has been fully implemented with the following components:

### Completed Packages

| Package | Status | Description |
|---------|--------|-------------|
| imports/wasip2 | ✅ | Top-level Instantiate and Config |
| imports/wasip2/io | ✅ | Streams, Poll, Error with ResourceTable integration |
| imports/wasip2/clocks | ✅ | Wall clock and monotonic clock with real time data |
| imports/wasip2/random | ✅ | Secure and insecure random with real crypto/rand |
| imports/wasip2/cli | ✅ | Environment, args, stdin/stdout/stderr, exit |
| imports/wasip2/filesystem | ✅ | Full descriptor operations with real file I/O |
| imports/wasip2/sockets | ✅ | TCP/UDP with real net package operations |
| imports/wasip2/http | ✅ | HTTP client with real net/http operations |

### Key Implementation Details

1. **Host functions use ResourceTable** - All resources (streams, sockets, descriptors) are stored in ResourceTable and looked up by handle in host functions.

2. **Config passed via context** - WASIConfig interface allows Config to be stored in context and retrieved by sub-packages without import cycles.

3. **Real operations** - All functions perform real I/O operations using Go's standard library (os, net, net/http, crypto/rand).

4. **Comprehensive tests** - 400+ unit tests covering all host functions plus integration tests.

### Test Coverage

```
ok      github.com/tetratelabs/wazero/imports/wasip2             (integration tests)
ok      github.com/tetratelabs/wazero/imports/wasip2/cli         (CLI tests)
ok      github.com/tetratelabs/wazero/imports/wasip2/clocks      (clock tests)
ok      github.com/tetratelabs/wazero/imports/wasip2/filesystem  (filesystem tests)
ok      github.com/tetratelabs/wazero/imports/wasip2/http        (HTTP tests)
ok      github.com/tetratelabs/wazero/imports/wasip2/io          (streams/poll tests)
ok      github.com/tetratelabs/wazero/imports/wasip2/random      (random tests)
ok      github.com/tetratelabs/wazero/imports/wasip2/sockets     (socket tests)
```

---

## Overview

This phase implements all WASI Preview 2 interfaces, providing the standard system interfaces for component model applications. WASI P2 (version 0.2.0, released January 2024) defines a complete set of interfaces using the WebAssembly Interface Type (WIT) format.

**Goal:** Complete WASI Preview 2 implementation that works out-of-box with pluggable configuration.

**Architecture:** Host functions registered via the component Linker infrastructure with resources backed by Go's standard library.

**Prerequisites:**
- Phase 1-4 complete (full component model runtime with linking)
- Linker with DefineInstance, DefineFunc, DefineResource
- Resource table with own/borrow semantics
- Val type system for all WIT types

---

## WASI P2 Interface Dependencies

```
wasi:io/error        (standalone)
wasi:io/poll         (uses error)
wasi:io/streams      (uses error, poll)
wasi:clocks/wall     (standalone)
wasi:clocks/mono     (uses poll)
wasi:random          (standalone)
wasi:cli             (uses io/streams)
wasi:filesystem      (uses io/streams, clocks/wall)
wasi:sockets         (uses io/streams, poll, clocks/mono)
wasi:http            (uses io/streams, poll)
```

Implementation order: io → clocks → random → cli → filesystem → sockets → http

---

## Phase 5 Milestones

| Milestone | Description | Tasks | Wasmtime Tests |
|-----------|-------------|-------|----------------|
| 5.1 | Package structure & config | 151-154 | - |
| 5.2 | wasi:io/error | 155-157 | p2_stream_pollable_correct |
| 5.3 | wasi:io/poll | 158-162 | p2_stream_pollable_correct |
| 5.4 | wasi:io/streams | 163-175 | p2_stream_pollable_correct |
| 5.5 | wasi:clocks | 176-185 | p2_sleep |
| 5.6 | wasi:random | 186-192 | p2_random |
| 5.7 | wasi:cli | 193-210 | p2_cli_args, p2_cli_env, p2_cli_exit_success |
| 5.8 | wasi:filesystem | 211-225 | p1_file_read_write, p2_cli_file_read |
| 5.9 | wasi:sockets | 226-235 | p2_tcp_bind, p2_tcp_connect, p2_udp_bind |
| 5.10 | wasi:http | 236-240 | p2_http_outbound_request_get |

---

## Package Structure

```
imports/wasip2/
├── wasip2.go              # Top-level Instantiate and InstantiateWithConfig
├── config.go              # WASIConfig for customization
├── io/
│   ├── io.go              # Package registration
│   ├── error.go           # wasi:io/error@0.2.0
│   ├── poll.go            # wasi:io/poll@0.2.0
│   └── streams.go         # wasi:io/streams@0.2.0
├── clocks/
│   ├── clocks.go          # Package registration
│   ├── monotonic.go       # wasi:clocks/monotonic-clock@0.2.0
│   └── wall.go            # wasi:clocks/wall-clock@0.2.0
├── random/
│   ├── random.go          # wasi:random/random@0.2.0
│   ├── insecure.go        # wasi:random/insecure@0.2.0
│   └── insecure_seed.go   # wasi:random/insecure-seed@0.2.0
├── cli/
│   ├── cli.go             # Package registration
│   ├── environment.go     # wasi:cli/environment@0.2.0
│   ├── exit.go            # wasi:cli/exit@0.2.0
│   ├── stdin.go           # wasi:cli/stdin@0.2.0
│   ├── stdout.go          # wasi:cli/stdout@0.2.0
│   ├── stderr.go          # wasi:cli/stderr@0.2.0
│   └── terminal.go        # terminal-input, terminal-output, etc.
├── filesystem/
│   ├── filesystem.go      # Package registration
│   ├── types.go           # wasi:filesystem/types@0.2.0
│   └── preopens.go        # wasi:filesystem/preopens@0.2.0
├── sockets/
│   ├── sockets.go         # Package registration
│   ├── network.go         # wasi:sockets/network@0.2.0
│   ├── tcp.go             # wasi:sockets/tcp@0.2.0
│   ├── tcp_create.go      # wasi:sockets/tcp-create-socket@0.2.0
│   ├── udp.go             # wasi:sockets/udp@0.2.0
│   ├── udp_create.go      # wasi:sockets/udp-create-socket@0.2.0
│   └── ip_name_lookup.go  # wasi:sockets/ip-name-lookup@0.2.0
└── http/
    ├── http.go            # Package registration
    ├── types.go           # wasi:http/types@0.2.0
    ├── incoming.go        # wasi:http/incoming-handler@0.2.0
    └── outgoing.go        # wasi:http/outgoing-handler@0.2.0
```

---

## Tasks

### Task 151: Create Package Structure and Top-Level Entry Point

**Files:**
- Create: `imports/wasip2/wasip2.go`
- Test: `imports/wasip2/wasip2_test.go`

**Step 1: Write failing test**

```go
// imports/wasip2/wasip2_test.go

package wasip2

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestInstantiate(t *testing.T) {
	linker := component.NewLinker()
	err := Instantiate(linker)
	require.NoError(t, err)

	// Verify io interfaces are registered
	_, err = linker.MatchImport("wasi:io/error@0.2.0")
	require.NoError(t, err)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./imports/wasip2/... -v -run TestInstantiate`
Expected: FAIL with "undefined: Instantiate"

**Step 3: Implement minimal Instantiate**

```go
// imports/wasip2/wasip2.go

package wasip2

import (
	"github.com/tetratelabs/wazero/internal/component"
)

// Instantiate registers all WASI Preview 2 interfaces with the linker.
// Uses default configuration backed by os package.
func Instantiate(linker *component.Linker) error {
	return InstantiateWithConfig(linker, DefaultConfig())
}

// InstantiateWithConfig registers all WASI Preview 2 interfaces with custom configuration.
func InstantiateWithConfig(linker *component.Linker, config *Config) error {
	// Phase 5.2-5.10 will add each interface
	// For now, register a placeholder for io/error
	return linker.DefineInstance("wasi:io/error@0.2.0").Build()
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./imports/wasip2/... -v -run TestInstantiate`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/wasip2.go imports/wasip2/wasip2_test.go
git commit -m "$(cat <<'EOF'
feat(wasip2): create package structure and Instantiate entry point

Initial WASI Preview 2 package with Instantiate and
InstantiateWithConfig functions.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 152: Define WASIConfig Structure

**Files:**
- Create: `imports/wasip2/config.go`
- Create: `imports/wasip2/config_test.go`

**Step 1: Write failing test**

```go
// imports/wasip2/config_test.go

package wasip2

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	require.NotNil(t, config)
	require.NotNil(t, config.Stdin)
	require.NotNil(t, config.Stdout)
	require.NotNil(t, config.Stderr)
	require.NotNil(t, config.Environ)
	require.NotNil(t, config.Args)
}

func TestConfigWithStdio(t *testing.T) {
	stdin := bytes.NewReader([]byte("hello"))
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	config := NewConfig().
		WithStdin(stdin).
		WithStdout(stdout).
		WithStderr(stderr)

	require.Equal(t, stdin, config.Stdin)
	require.Equal(t, stdout, config.Stdout)
	require.Equal(t, stderr, config.Stderr)
}

func TestConfigWithEnviron(t *testing.T) {
	config := NewConfig().
		WithEnviron([]string{"FOO=bar", "BAZ=qux"})

	require.Equal(t, []string{"FOO=bar", "BAZ=qux"}, config.Environ())
}

func TestConfigWithArgs(t *testing.T) {
	config := NewConfig().
		WithArgs([]string{"prog", "arg1", "arg2"})

	require.Equal(t, []string{"prog", "arg1", "arg2"}, config.Args())
}

func TestConfigWithPreopens(t *testing.T) {
	config := NewConfig().
		WithPreopen("/guest", "/host/path").
		WithPreopen("/tmp", "/var/tmp")

	preopens := config.Preopens()
	require.Len(t, preopens, 2)
	require.Equal(t, "/host/path", preopens["/guest"])
	require.Equal(t, "/var/tmp", preopens["/tmp"])
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./imports/wasip2/... -v -run TestDefaultConfig`
Expected: FAIL with "undefined: DefaultConfig"

**Step 3: Implement Config**

```go
// imports/wasip2/config.go

package wasip2

import (
	"io"
	"os"
)

// Config configures WASI Preview 2 behavior.
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
}

// NewConfig creates a new config with no defaults set.
func NewConfig() *Config {
	return &Config{
		preopens: make(map[string]string),
	}
}

// DefaultConfig returns config backed by os package defaults.
func DefaultConfig() *Config {
	return &Config{
		stdin:   os.Stdin,
		stdout:  os.Stdout,
		stderr:  os.Stderr,
		environ: os.Environ,
		args:    func() []string { return os.Args },
		preopens: make(map[string]string),
		allowNetwork: true,
		allowHTTP:    true,
	}
}

// WithStdin sets the reader for stdin.
func (c *Config) WithStdin(r io.Reader) *Config {
	c.stdin = r
	return c
}

// WithStdout sets the writer for stdout.
func (c *Config) WithStdout(w io.Writer) *Config {
	c.stdout = w
	return c
}

// WithStderr sets the writer for stderr.
func (c *Config) WithStderr(w io.Writer) *Config {
	c.stderr = w
	return c
}

// WithEnviron sets the environment variables as "KEY=value" strings.
func (c *Config) WithEnviron(environ []string) *Config {
	c.environ = func() []string { return environ }
	return c
}

// WithArgs sets the command-line arguments.
func (c *Config) WithArgs(args []string) *Config {
	c.args = func() []string { return args }
	return c
}

// WithPreopen maps a guest path to a host path for filesystem access.
func (c *Config) WithPreopen(guestPath, hostPath string) *Config {
	c.preopens[guestPath] = hostPath
	return c
}

// WithNetwork enables or disables network access.
func (c *Config) WithNetwork(allow bool) *Config {
	c.allowNetwork = allow
	return c
}

// WithHTTP enables or disables HTTP access.
func (c *Config) WithHTTP(allow bool) *Config {
	c.allowHTTP = allow
	return c
}

// Accessors

func (c *Config) Stdin() io.Reader { return c.stdin }
func (c *Config) Stdout() io.Writer { return c.stdout }
func (c *Config) Stderr() io.Writer { return c.stderr }
func (c *Config) Environ() []string {
	if c.environ == nil {
		return nil
	}
	return c.environ()
}
func (c *Config) Args() []string {
	if c.args == nil {
		return nil
	}
	return c.args()
}
func (c *Config) Preopens() map[string]string { return c.preopens }
func (c *Config) AllowNetwork() bool { return c.allowNetwork }
func (c *Config) AllowHTTP() bool { return c.allowHTTP }
```

**Step 4: Run test to verify it passes**

Run: `go test ./imports/wasip2/... -v -run TestDefaultConfig`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/config.go imports/wasip2/config_test.go
git commit -m "$(cat <<'EOF'
feat(wasip2): implement Config for WASI customization

Config allows customizing stdio, environment, args, preopens,
and feature flags for WASI P2 instantiation.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 153: Create IO Package Structure

**Files:**
- Create: `imports/wasip2/io/io.go`
- Create: `imports/wasip2/io/io_test.go`

**Step 1: Write failing test**

```go
// imports/wasip2/io/io_test.go

package io

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestInstantiate(t *testing.T) {
	linker := component.NewLinker()
	err := Instantiate(linker)
	require.NoError(t, err)

	// Verify all io interfaces are registered
	_, err = linker.MatchImport("wasi:io/error@0.2.0")
	require.NoError(t, err)
	_, err = linker.MatchImport("wasi:io/poll@0.2.0")
	require.NoError(t, err)
	_, err = linker.MatchImport("wasi:io/streams@0.2.0")
	require.NoError(t, err)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./imports/wasip2/io/... -v -run TestInstantiate`
Expected: FAIL with "undefined: Instantiate"

**Step 3: Implement IO package**

```go
// imports/wasip2/io/io.go

package io

import (
	"github.com/tetratelabs/wazero/internal/component"
)

// Instantiate registers all wasi:io interfaces with the linker.
func Instantiate(linker *component.Linker) error {
	if err := instantiateError(linker); err != nil {
		return err
	}
	if err := instantiatePoll(linker); err != nil {
		return err
	}
	if err := instantiateStreams(linker); err != nil {
		return err
	}
	return nil
}

// Placeholder implementations - will be replaced in subsequent tasks
func instantiateError(linker *component.Linker) error {
	return linker.DefineInstance("wasi:io/error@0.2.0").Build()
}

func instantiatePoll(linker *component.Linker) error {
	return linker.DefineInstance("wasi:io/poll@0.2.0").Build()
}

func instantiateStreams(linker *component.Linker) error {
	return linker.DefineInstance("wasi:io/streams@0.2.0").Build()
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./imports/wasip2/io/... -v -run TestInstantiate`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/io/io.go imports/wasip2/io/io_test.go
git commit -m "$(cat <<'EOF'
feat(wasip2): create io package structure

Registers placeholder instances for wasi:io/error, poll, and streams.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 154: Integrate IO Package into Top-Level Instantiate

**Files:**
- Modify: `imports/wasip2/wasip2.go`
- Modify: `imports/wasip2/wasip2_test.go`

**Step 1: Write failing test**

```go
// Add to imports/wasip2/wasip2_test.go

func TestInstantiate_IOInterfaces(t *testing.T) {
	linker := component.NewLinker()
	err := Instantiate(linker)
	require.NoError(t, err)

	// Verify io interfaces
	_, err = linker.MatchImport("wasi:io/error@0.2.0")
	require.NoError(t, err)
	_, err = linker.MatchImport("wasi:io/poll@0.2.0")
	require.NoError(t, err)
	_, err = linker.MatchImport("wasi:io/streams@0.2.0")
	require.NoError(t, err)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./imports/wasip2/... -v -run TestInstantiate_IOInterfaces`
Expected: FAIL (interfaces not registered via io package)

**Step 3: Integrate io package**

```go
// Modify imports/wasip2/wasip2.go

package wasip2

import (
	"github.com/tetratelabs/wazero/imports/wasip2/io"
	"github.com/tetratelabs/wazero/internal/component"
)

func Instantiate(linker *component.Linker) error {
	return InstantiateWithConfig(linker, DefaultConfig())
}

func InstantiateWithConfig(linker *component.Linker, config *Config) error {
	if err := io.Instantiate(linker); err != nil {
		return err
	}
	// clocks, random, cli, filesystem, sockets, http will be added
	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./imports/wasip2/... -v -run TestInstantiate_IOInterfaces`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/wasip2.go imports/wasip2/wasip2_test.go
git commit -m "$(cat <<'EOF'
feat(wasip2): integrate io package into top-level Instantiate

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 155: Implement wasi:io/error Resource

**Files:**
- Create: `imports/wasip2/io/error.go`
- Create: `imports/wasip2/io/error_test.go`

**Step 1: Write failing test**

```go
// imports/wasip2/io/error_test.go

package io

import (
	"context"
	"errors"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestError_ToDebugString(t *testing.T) {
	linker := component.NewLinker()
	err := instantiateError(linker)
	require.NoError(t, err)

	// Create an error resource
	table := component.NewResourceTable()
	goErr := errors.New("test error message")
	handle := table.New(&Error{err: goErr}, true)

	// The to-debug-string method should return the error message
	entry, err := table.Get(handle)
	require.NoError(t, err)
	errResource := entry.Rep.(*Error)
	require.Equal(t, "test error message", errResource.ToDebugString())
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./imports/wasip2/io/... -v -run TestError_ToDebugString`
Expected: FAIL with "undefined: Error"

**Step 3: Implement error resource**

```go
// imports/wasip2/io/error.go

package io

import (
	"context"

	"github.com/tetratelabs/wazero/internal/component"
)

// Error wraps a Go error for the wasi:io/error resource.
type Error struct {
	err error
}

// NewError creates a new Error resource wrapping a Go error.
func NewError(err error) *Error {
	return &Error{err: err}
}

// ToDebugString returns a human-readable debug string.
func (e *Error) ToDebugString() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

// Unwrap returns the underlying Go error.
func (e *Error) Unwrap() error {
	return e.err
}

func instantiateError(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:io/error@0.2.0")

	// Define the error resource type
	inst.Resource("error", func(rep uint32) {
		// Destructor - nothing to clean up for errors
	})

	// [method]error.to-debug-string: func() -> string
	inst.Func("[method]error.to-debug-string", errorToDebugString)

	return inst.Build()
}

func errorToDebugString(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// args[0] is borrow<error>
	handle := args[0].Borrow()

	table := component.ResourceTableFromContext(ctx)
	entry, err := table.Get(handle)
	if err != nil {
		return []component.Val{component.ValString("")}, nil
	}

	errResource, ok := entry.Rep.(*Error)
	if !ok {
		return []component.Val{component.ValString("")}, nil
	}

	return []component.Val{component.ValString(errResource.ToDebugString())}, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./imports/wasip2/io/... -v -run TestError_ToDebugString`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/io/error.go imports/wasip2/io/error_test.go
git commit -m "$(cat <<'EOF'
feat(wasip2): implement wasi:io/error resource

Error wraps Go errors with to-debug-string method per WIT spec.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 156-157: Error resource edge cases and destructor

Tasks 156-157 follow the same TDD pattern for error edge cases.

---

### Task 158: Implement wasi:io/poll Pollable Resource

**Files:**
- Create: `imports/wasip2/io/poll.go`
- Create: `imports/wasip2/io/poll_test.go`

**Step 1: Write failing test**

```go
// imports/wasip2/io/poll_test.go

package io

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestPollable_Ready_Immediate(t *testing.T) {
	// A pollable that is already ready
	p := &Pollable{readyFn: func() bool { return true }}
	require.True(t, p.Ready())
}

func TestPollable_Ready_NotReady(t *testing.T) {
	p := &Pollable{readyFn: func() bool { return false }}
	require.False(t, p.Ready())
}

func TestPollable_Block(t *testing.T) {
	blocked := false
	p := &Pollable{
		readyFn: func() bool { return blocked },
		blockFn: func() { blocked = true },
	}
	require.False(t, p.Ready())
	p.Block()
	require.True(t, p.Ready())
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./imports/wasip2/io/... -v -run TestPollable`
Expected: FAIL with "undefined: Pollable"

**Step 3: Implement Pollable**

```go
// imports/wasip2/io/poll.go

package io

import (
	"context"

	"github.com/tetratelabs/wazero/internal/component"
)

// Pollable represents something that can be polled for readiness.
// Per wasi:io/poll@0.2.0 spec.
type Pollable struct {
	// readyFn returns true if the pollable is ready
	readyFn func() bool
	// blockFn blocks until ready (may be nil for immediately ready)
	blockFn func()
}

// NewPollable creates a pollable with ready and block functions.
func NewPollable(readyFn func() bool, blockFn func()) *Pollable {
	return &Pollable{readyFn: readyFn, blockFn: blockFn}
}

// NewReadyPollable creates a pollable that is immediately ready.
func NewReadyPollable() *Pollable {
	return &Pollable{readyFn: func() bool { return true }}
}

// Ready returns true if the pollable is ready.
func (p *Pollable) Ready() bool {
	if p.readyFn == nil {
		return true
	}
	return p.readyFn()
}

// Block waits until the pollable becomes ready.
func (p *Pollable) Block() {
	if p.blockFn != nil {
		p.blockFn()
	}
}

func instantiatePoll(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:io/poll@0.2.0")

	// Define pollable resource
	inst.Resource("pollable", func(rep uint32) {
		// Destructor - nothing to clean up
	})

	// [method]pollable.ready: func() -> bool
	inst.Func("[method]pollable.ready", pollableReady)

	// [method]pollable.block: func()
	inst.Func("[method]pollable.block", pollableBlock)

	// poll: func(in: list<borrow<pollable>>) -> list<u32>
	inst.Func("poll", pollPoll)

	return inst.Build()
}

func pollableReady(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()

	table := component.ResourceTableFromContext(ctx)
	entry, err := table.Get(handle)
	if err != nil {
		return []component.Val{component.ValBool(false)}, nil
	}

	p, ok := entry.Rep.(*Pollable)
	if !ok {
		return []component.Val{component.ValBool(false)}, nil
	}

	return []component.Val{component.ValBool(p.Ready())}, nil
}

func pollableBlock(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()

	table := component.ResourceTableFromContext(ctx)
	entry, err := table.Get(handle)
	if err != nil {
		return nil, nil
	}

	p, ok := entry.Rep.(*Pollable)
	if !ok {
		return nil, nil
	}

	p.Block()
	return nil, nil
}

func pollPoll(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// args[0] is list<borrow<pollable>>
	pollables := args[0].List()

	table := component.ResourceTableFromContext(ctx)
	var readyIndices []component.Val

	// Check each pollable, return indices of ready ones
	for i, pVal := range pollables {
		handle := pVal.Borrow()
		entry, err := table.Get(handle)
		if err != nil {
			continue
		}

		p, ok := entry.Rep.(*Pollable)
		if !ok {
			continue
		}

		if p.Ready() {
			readyIndices = append(readyIndices, component.ValU32(uint32(i)))
		}
	}

	// If none ready, block on first one (simplified)
	if len(readyIndices) == 0 && len(pollables) > 0 {
		handle := pollables[0].Borrow()
		entry, _ := table.Get(handle)
		if p, ok := entry.Rep.(*Pollable); ok {
			p.Block()
			readyIndices = append(readyIndices, component.ValU32(0))
		}
	}

	return []component.Val{component.ValList(readyIndices)}, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./imports/wasip2/io/... -v -run TestPollable`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/io/poll.go imports/wasip2/io/poll_test.go
git commit -m "$(cat <<'EOF'
feat(wasip2): implement wasi:io/poll pollable resource

Pollable with ready(), block() methods and poll() function
per wasi:io/poll@0.2.0 spec.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 159-162: Poll edge cases and multi-pollable support

Tasks 159-162 follow the same TDD pattern for poll edge cases.

---

### Task 163: Define Stream Error Variant

**Files:**
- Modify: `imports/wasip2/io/streams.go`
- Create: `imports/wasip2/io/streams_test.go`

**Step 1: Write failing test**

```go
// imports/wasip2/io/streams_test.go

package io

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestStreamError_Closed(t *testing.T) {
	err := StreamErrorClosed()
	require.True(t, err.IsClosed())
	require.False(t, err.IsLastOperationFailed())
}

func TestStreamError_LastOperationFailed(t *testing.T) {
	ioErr := NewError(errors.New("test"))
	err := StreamErrorLastOperationFailed(ioErr)
	require.False(t, err.IsClosed())
	require.True(t, err.IsLastOperationFailed())
	require.Equal(t, ioErr, err.Error())
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./imports/wasip2/io/... -v -run TestStreamError`
Expected: FAIL with "undefined: StreamErrorClosed"

**Step 3: Implement StreamError**

```go
// imports/wasip2/io/streams.go

package io

import (
	"context"
	goio "io"

	"github.com/tetratelabs/wazero/internal/component"
)

// StreamError represents the stream-error variant type.
// variant stream-error {
//   last-operation-failed(error),
//   closed,
// }
type StreamError struct {
	kind      streamErrorKind
	lastError *Error
}

type streamErrorKind uint8

const (
	streamErrorKindClosed streamErrorKind = iota
	streamErrorKindLastOperationFailed
)

// StreamErrorClosed creates a closed stream error.
func StreamErrorClosed() *StreamError {
	return &StreamError{kind: streamErrorKindClosed}
}

// StreamErrorLastOperationFailed creates a last-operation-failed error.
func StreamErrorLastOperationFailed(err *Error) *StreamError {
	return &StreamError{
		kind:      streamErrorKindLastOperationFailed,
		lastError: err,
	}
}

func (e *StreamError) IsClosed() bool {
	return e.kind == streamErrorKindClosed
}

func (e *StreamError) IsLastOperationFailed() bool {
	return e.kind == streamErrorKindLastOperationFailed
}

func (e *StreamError) Error() *Error {
	return e.lastError
}

// ToVal converts StreamError to a component Val (variant).
func (e *StreamError) ToVal() component.Val {
	switch e.kind {
	case streamErrorKindClosed:
		return component.ValVariant(1, nil) // closed = discriminant 1
	case streamErrorKindLastOperationFailed:
		// Payload would be the error handle
		return component.ValVariant(0, nil) // last-operation-failed = discriminant 0
	default:
		return component.ValVariant(1, nil)
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./imports/wasip2/io/... -v -run TestStreamError`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/io/streams.go imports/wasip2/io/streams_test.go
git commit -m "$(cat <<'EOF'
feat(wasip2): define stream-error variant type

StreamError implements the stream-error variant with closed
and last-operation-failed cases per wasi:io/streams spec.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 164: Implement InputStream Resource

**Files:**
- Modify: `imports/wasip2/io/streams.go`
- Modify: `imports/wasip2/io/streams_test.go`

**Step 1: Write failing test**

```go
// Add to imports/wasip2/io/streams_test.go

func TestInputStream_Read(t *testing.T) {
	reader := bytes.NewReader([]byte("hello world"))
	stream := NewInputStream(reader)

	data, err := stream.Read(5)
	require.NoError(t, err)
	require.Equal(t, []byte("hello"), data)
}

func TestInputStream_Read_EOF(t *testing.T) {
	reader := bytes.NewReader([]byte("hi"))
	stream := NewInputStream(reader)

	// Read all data
	data, err := stream.Read(10)
	require.NoError(t, err)
	require.Equal(t, []byte("hi"), data)

	// Next read returns empty (EOF)
	data, err = stream.Read(10)
	require.NoError(t, err)
	require.Empty(t, data)
}

func TestInputStream_Subscribe(t *testing.T) {
	reader := bytes.NewReader([]byte("test"))
	stream := NewInputStream(reader)

	pollable := stream.Subscribe()
	require.NotNil(t, pollable)
	require.True(t, pollable.Ready()) // bytes.Reader is always ready
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./imports/wasip2/io/... -v -run TestInputStream`
Expected: FAIL with "undefined: NewInputStream"

**Step 3: Implement InputStream**

```go
// Add to imports/wasip2/io/streams.go

// InputStream wraps a Go io.Reader for wasi:io/streams.
type InputStream struct {
	reader goio.Reader
	closed bool
}

// NewInputStream creates an input stream from a Go reader.
func NewInputStream(r goio.Reader) *InputStream {
	return &InputStream{reader: r}
}

// Read reads up to len bytes from the stream.
// Returns the data read or an error.
func (s *InputStream) Read(maxLen uint64) ([]byte, error) {
	if s.closed {
		return nil, StreamErrorClosed()
	}

	buf := make([]byte, min(maxLen, 64*1024)) // Cap at 64KB
	n, err := s.reader.Read(buf)
	if err == goio.EOF {
		return buf[:n], nil // Return what we got, no error
	}
	if err != nil {
		return nil, StreamErrorLastOperationFailed(NewError(err))
	}
	return buf[:n], nil
}

// BlockingRead reads exactly len bytes, blocking until available.
func (s *InputStream) BlockingRead(maxLen uint64) ([]byte, error) {
	if s.closed {
		return nil, StreamErrorClosed()
	}

	buf := make([]byte, min(maxLen, 64*1024))
	n, err := goio.ReadFull(s.reader, buf)
	if err == goio.EOF || err == goio.ErrUnexpectedEOF {
		return buf[:n], nil
	}
	if err != nil {
		return nil, StreamErrorLastOperationFailed(NewError(err))
	}
	return buf[:n], nil
}

// Skip skips len bytes from the stream.
func (s *InputStream) Skip(len uint64) (uint64, error) {
	if s.closed {
		return 0, StreamErrorClosed()
	}

	// Use io.Discard as target
	n, err := goio.CopyN(goio.Discard, s.reader, int64(len))
	if err == goio.EOF {
		return uint64(n), nil
	}
	if err != nil {
		return uint64(n), StreamErrorLastOperationFailed(NewError(err))
	}
	return uint64(n), nil
}

// Subscribe returns a pollable for this stream.
func (s *InputStream) Subscribe() *Pollable {
	// For simple readers, always ready
	// More sophisticated implementation would check underlying fd
	return NewReadyPollable()
}

// Close marks the stream as closed.
func (s *InputStream) Close() {
	s.closed = true
	if closer, ok := s.reader.(goio.Closer); ok {
		closer.Close()
	}
}

func min(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./imports/wasip2/io/... -v -run TestInputStream`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/io/streams.go imports/wasip2/io/streams_test.go
git commit -m "$(cat <<'EOF'
feat(wasip2): implement input-stream resource

InputStream wraps io.Reader with read, blocking-read, skip,
and subscribe methods per wasi:io/streams spec.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 165-170: OutputStream and stream host functions

Tasks 165-170 follow the same TDD pattern for OutputStream and registering host functions.

---

### Task 171-175: Stream integration tests

**Reference Test: wasmtime p2_stream_pollable_correct**

```go
// imports/wasip2/io/streams_integration_test.go

func TestStreams_PollableCorrect(t *testing.T) {
	// This test mirrors wasmtime's p2_stream_pollable_correct test
	// 1. Get stdin stream
	// 2. Subscribe to get pollable
	// 3. Verify pollable.ready() behavior
	// 4. Test write with check_write() capacity
}
```

---

### Task 176: Implement wasi:clocks/wall-clock

**Files:**
- Create: `imports/wasip2/clocks/clocks.go`
- Create: `imports/wasip2/clocks/wall.go`
- Create: `imports/wasip2/clocks/wall_test.go`

**Step 1: Write failing test**

```go
// imports/wasip2/clocks/wall_test.go

package clocks

import (
	"testing"
	"time"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestWallClock_Now(t *testing.T) {
	before := time.Now()
	dt := WallClockNow()
	after := time.Now()

	// Verify datetime is within the time window
	require.True(t, dt.Seconds >= uint64(before.Unix()))
	require.True(t, dt.Seconds <= uint64(after.Unix()))
}

func TestWallClock_Resolution(t *testing.T) {
	dt := WallClockResolution()
	// Resolution should be at least nanosecond precision
	require.True(t, dt.Seconds == 0 && dt.Nanoseconds > 0)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./imports/wasip2/clocks/... -v -run TestWallClock`
Expected: FAIL with "undefined: WallClockNow"

**Step 3: Implement wall-clock**

```go
// imports/wasip2/clocks/wall.go

package clocks

import (
	"context"
	"time"

	"github.com/tetratelabs/wazero/internal/component"
)

// Datetime represents the wall-clock datetime record.
// record datetime {
//   seconds: u64,
//   nanoseconds: u32,
// }
type Datetime struct {
	Seconds     uint64
	Nanoseconds uint32
}

// WallClockNow returns the current wall clock time.
func WallClockNow() Datetime {
	now := time.Now()
	return Datetime{
		Seconds:     uint64(now.Unix()),
		Nanoseconds: uint32(now.Nanosecond()),
	}
}

// WallClockResolution returns the resolution of the wall clock.
func WallClockResolution() Datetime {
	// Go's time.Time has nanosecond precision
	return Datetime{
		Seconds:     0,
		Nanoseconds: 1,
	}
}

func instantiateWallClock(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:clocks/wall-clock@0.2.0")

	// now: func() -> datetime
	inst.Func("now", wallClockNow)

	// resolution: func() -> datetime
	inst.Func("resolution", wallClockResolution)

	return inst.Build()
}

func wallClockNow(ctx context.Context, args []component.Val) ([]component.Val, error) {
	dt := WallClockNow()
	// Return as record with seconds and nanoseconds
	return []component.Val{
		component.ValRecord(map[string]component.Val{
			"seconds":     component.ValU64(dt.Seconds),
			"nanoseconds": component.ValU32(dt.Nanoseconds),
		}),
	}, nil
}

func wallClockResolution(ctx context.Context, args []component.Val) ([]component.Val, error) {
	dt := WallClockResolution()
	return []component.Val{
		component.ValRecord(map[string]component.Val{
			"seconds":     component.ValU64(dt.Seconds),
			"nanoseconds": component.ValU32(dt.Nanoseconds),
		}),
	}, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./imports/wasip2/clocks/... -v -run TestWallClock`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/clocks/wall.go imports/wasip2/clocks/wall_test.go imports/wasip2/clocks/clocks.go
git commit -m "$(cat <<'EOF'
feat(wasip2): implement wasi:clocks/wall-clock

Datetime record with now() and resolution() functions backed by
time.Now() with nanosecond precision.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 177-180: Implement wasi:clocks/monotonic-clock

**Files:**
- Create: `imports/wasip2/clocks/monotonic.go`
- Create: `imports/wasip2/clocks/monotonic_test.go`

**Reference Test: wasmtime p2_sleep**

The p2_sleep test validates:
1. `monotonic_clock::subscribe_duration(duration)` returns a pollable
2. `pollable.block()` waits for duration to elapse
3. Zero-duration timers are immediately ready
4. Past instants are immediately ready

```go
// imports/wasip2/clocks/monotonic_test.go

func TestMonotonicClock_SubscribeDuration(t *testing.T) {
	// 10ms sleep
	duration := uint64(10 * time.Millisecond)
	pollable := SubscribeDuration(duration)

	start := time.Now()
	pollable.Block()
	elapsed := time.Since(start)

	require.True(t, elapsed >= 10*time.Millisecond)
}

func TestMonotonicClock_SubscribeDuration_Zero(t *testing.T) {
	// Zero duration should be immediately ready
	pollable := SubscribeDuration(0)
	require.True(t, pollable.Ready())
}
```

---

### Task 181-185: Clock integration tests

Following the wasmtime p2_sleep pattern:
- Test duration subscriptions
- Test instant subscriptions
- Test zero-duration behavior
- Test past-instant behavior

---

### Task 186-192: Implement wasi:random

**Files:**
- Create: `imports/wasip2/random/random.go`
- Create: `imports/wasip2/random/insecure.go`
- Create: `imports/wasip2/random/insecure_seed.go`
- Create: `imports/wasip2/random/random_test.go`

**Reference Test: wasmtime p2_random**

```go
// imports/wasip2/random/random_test.go

func TestRandom_GetRandomBytes(t *testing.T) {
	bytes := GetRandomBytes(100)
	require.Len(t, bytes, 100)
	// At least some bytes should be non-zero
	hasNonZero := false
	for _, b := range bytes {
		if b != 0 {
			hasNonZero = true
			break
		}
	}
	require.True(t, hasNonZero)
}

func TestRandom_GetRandomU64(t *testing.T) {
	// Generate several u64 values to verify randomness
	values := make(map[uint64]bool)
	for i := 0; i < 10; i++ {
		v := GetRandomU64()
		values[v] = true
	}
	// Should have multiple unique values
	require.True(t, len(values) > 1)
}

func TestInsecureSeed_Deterministic(t *testing.T) {
	// Per spec: insecure_seed returns same value within instance
	seed1a, seed1b := InsecureSeed()
	seed2a, seed2b := InsecureSeed()
	require.Equal(t, seed1a, seed2a)
	require.Equal(t, seed1b, seed2b)
}
```

---

### Task 193-210: Implement wasi:cli

**Reference Tests: wasmtime p2_cli_args, p2_cli_env, p2_cli_exit_success**

#### Task 193: wasi:cli/environment

```go
// imports/wasip2/cli/environment.go

func instantiateEnvironment(linker *component.Linker, config *Config) error {
	inst := linker.DefineInstance("wasi:cli/environment@0.2.0")

	// get-environment: func() -> list<tuple<string, string>>
	inst.Func("get-environment", func(ctx context.Context, args []component.Val) ([]component.Val, error) {
		environ := config.Environ()
		tuples := make([]component.Val, len(environ))
		for i, env := range environ {
			parts := strings.SplitN(env, "=", 2)
			key := parts[0]
			value := ""
			if len(parts) > 1 {
				value = parts[1]
			}
			tuples[i] = component.ValTuple(
				component.ValString(key),
				component.ValString(value),
			)
		}
		return []component.Val{component.ValList(tuples)}, nil
	})

	// get-arguments: func() -> list<string>
	inst.Func("get-arguments", func(ctx context.Context, args []component.Val) ([]component.Val, error) {
		arguments := config.Args()
		vals := make([]component.Val, len(arguments))
		for i, arg := range arguments {
			vals[i] = component.ValString(arg)
		}
		return []component.Val{component.ValList(vals)}, nil
	})

	// initial-cwd: func() -> option<string>
	inst.Func("initial-cwd", func(ctx context.Context, args []component.Val) ([]component.Val, error) {
		cwd, err := os.Getwd()
		if err != nil {
			return []component.Val{component.ValOption(nil)}, nil
		}
		s := component.ValString(cwd)
		return []component.Val{component.ValOption(&s)}, nil
	})

	return inst.Build()
}
```

**Test matching wasmtime p2_cli_env:**

```go
func TestEnvironment_GetEnvironment(t *testing.T) {
	config := NewConfig().WithEnviron([]string{
		"frabjous=day",
		"callooh=callay",
	})
	// ... test that get-environment returns expected values
}

func TestEnvironment_GetArguments(t *testing.T) {
	config := NewConfig().WithArgs([]string{
		"hello", "this", "", "is an argument", "with emoji",
	})
	// ... test that get-arguments returns expected values
}
```

#### Task 197: wasi:cli/exit

```go
// imports/wasip2/cli/exit.go

func instantiateExit(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:cli/exit@0.2.0")

	// exit: func(status: result)
	inst.Func("exit", func(ctx context.Context, args []component.Val) ([]component.Val, error) {
		// status is result<_, _>
		// discriminant 0 = ok (success), 1 = error (failure)
		status := args[0].Result()
		if status.IsOk() {
			os.Exit(0)
		} else {
			os.Exit(1)
		}
		return nil, nil // Never reached
	})

	return inst.Build()
}
```

#### Task 200-210: stdin, stdout, stderr, terminal interfaces

Following the pattern, implement each stdio interface connecting to config streams.

---

### Task 211-225: Implement wasi:filesystem

**Reference Tests: wasmtime p1_file_read_write, p2_cli_file_read**

#### Key Types

```go
// imports/wasip2/filesystem/types.go

// Descriptor represents an open file or directory.
type Descriptor struct {
	file      *os.File
	isDir     bool
	path      string
	flags     DescriptorFlags
}

// DescriptorFlags matches wasi:filesystem/types flags
type DescriptorFlags uint8

const (
	DescriptorFlagRead              DescriptorFlags = 1 << 0
	DescriptorFlagWrite             DescriptorFlags = 1 << 1
	DescriptorFlagFileIntegritySync DescriptorFlags = 1 << 2
	DescriptorFlagDataIntegritySync DescriptorFlags = 1 << 3
	DescriptorFlagRequestedWriteSync DescriptorFlags = 1 << 4
	DescriptorFlagMutateDirectory   DescriptorFlags = 1 << 5
)

// ErrorCode matches wasi:filesystem/types error-code enum
type ErrorCode uint8

const (
	ErrorCodeAccess ErrorCode = iota
	ErrorCodeWouldBlock
	ErrorCodeAlready
	ErrorCodeBadDescriptor
	// ... all 30+ error codes from spec
)
```

#### Task 214: Descriptor.read

```go
func (d *Descriptor) Read(length, offset uint64) ([]byte, bool, error) {
	if d.file == nil {
		return nil, false, ErrorCodeBadDescriptor
	}

	buf := make([]byte, length)
	n, err := d.file.ReadAt(buf, int64(offset))
	if err == io.EOF {
		return buf[:n], true, nil // EOF flag
	}
	if err != nil {
		return nil, false, mapOSError(err)
	}
	return buf[:n], false, nil
}
```

#### Task 220: Preopens

```go
// imports/wasip2/filesystem/preopens.go

func instantiatePreopens(linker *component.Linker, config *Config) error {
	inst := linker.DefineInstance("wasi:filesystem/preopens@0.2.0")

	// get-directories: func() -> list<tuple<descriptor, string>>
	inst.Func("get-directories", func(ctx context.Context, args []component.Val) ([]component.Val, error) {
		table := component.ResourceTableFromContext(ctx)
		preopens := config.Preopens()

		result := make([]component.Val, 0, len(preopens))
		for guestPath, hostPath := range preopens {
			file, err := os.Open(hostPath)
			if err != nil {
				continue
			}

			desc := &Descriptor{file: file, isDir: true, path: hostPath}
			handle := table.New(desc, true)

			result = append(result, component.ValTuple(
				component.ValOwn(handle),
				component.ValString(guestPath),
			))
		}

		return []component.Val{component.ValList(result)}, nil
	})

	return inst.Build()
}
```

---

### Task 226-235: Implement wasi:sockets

**Reference Tests: wasmtime p2_tcp_bind, p2_tcp_connect, p2_udp_bind**

#### Key TCP Socket Operations

```go
// imports/wasip2/sockets/tcp.go

// TcpSocket represents a TCP socket resource.
type TcpSocket struct {
	listener *net.TCPListener
	conn     *net.TCPConn
	family   IpAddressFamily
	state    tcpState
}

type tcpState uint8

const (
	tcpStateUnbound tcpState = iota
	tcpStateBound
	tcpStateListening
	tcpStateConnecting
	tcpStateConnected
)

func (s *TcpSocket) StartBind(network *Network, localAddr IpSocketAddress) error {
	if s.state != tcpStateUnbound {
		return ErrorCodeInvalidState
	}
	// Validate address family matches socket
	// Begin non-blocking bind
	return nil
}

func (s *TcpSocket) FinishBind() error {
	if s.state != tcpStateUnbound {
		return ErrorCodeInvalidState
	}
	// Complete the bind
	s.state = tcpStateBound
	return nil
}
```

**Test matching wasmtime p2_tcp_bind:**

```go
func TestTcpSocket_BindEphemeral(t *testing.T) {
	// Bind to port 0, verify assigned port
	sock := NewTcpSocket(IpAddressFamilyIpv4)
	network := NewNetwork()

	err := sock.StartBind(network, IpSocketAddress{
		Port: 0,
		Address: IpAddressIpv4{127, 0, 0, 1},
	})
	require.NoError(t, err)

	err = sock.FinishBind()
	require.NoError(t, err)

	addr, err := sock.LocalAddress()
	require.NoError(t, err)
	require.NotEqual(t, uint16(0), addr.Port)
}

func TestTcpSocket_BindAddressInUse(t *testing.T) {
	// Bind same port twice without SO_REUSEADDR
	sock1 := NewTcpSocket(IpAddressFamilyIpv4)
	sock2 := NewTcpSocket(IpAddressFamilyIpv4)
	network := NewNetwork()

	// First bind succeeds
	sock1.StartBind(network, IpSocketAddress{Port: 0})
	sock1.FinishBind()
	addr1, _ := sock1.LocalAddress()

	// Second bind to same port fails
	err := sock2.StartBind(network, IpSocketAddress{Port: addr1.Port})
	require.NoError(t, err)
	err = sock2.FinishBind()
	require.Equal(t, ErrorCodeAddressInUse, err)
}
```

---

### Task 236-240: Implement wasi:http

**Reference Test: wasmtime p2_http_outbound_request_get**

#### Key Types

```go
// imports/wasip2/http/types.go

// Method represents HTTP methods
type Method uint8

const (
	MethodGet Method = iota
	MethodHead
	MethodPost
	MethodPut
	MethodDelete
	MethodConnect
	MethodOptions
	MethodTrace
	MethodPatch
	MethodOther
)

// OutgoingRequest represents an HTTP request to be sent.
type OutgoingRequest struct {
	method        Method
	scheme        *Scheme
	authority     *string
	pathWithQuery *string
	headers       *Fields
	body          *OutgoingBody
}

// Fields represents HTTP header/trailer fields.
type Fields struct {
	entries map[string][][]byte
}
```

#### Task 238: Outgoing Handler

```go
// imports/wasip2/http/outgoing.go

func instantiateOutgoingHandler(linker *component.Linker, config *Config) error {
	inst := linker.DefineInstance("wasi:http/outgoing-handler@0.2.0")

	// handle: func(request: outgoing-request, options: option<request-options>)
	//         -> result<future-incoming-response, error-code>
	inst.Func("handle", func(ctx context.Context, args []component.Val) ([]component.Val, error) {
		if !config.AllowHTTP() {
			return resultError(ErrorCodeInternalError), nil
		}

		table := component.ResourceTableFromContext(ctx)
		reqHandle := args[0].Own()
		entry, err := table.Remove(reqHandle)
		if err != nil {
			return resultError(ErrorCodeInternalError), nil
		}

		req := entry.Rep.(*OutgoingRequest)

		// Build Go http.Request
		httpReq, err := req.ToHTTPRequest(ctx)
		if err != nil {
			return resultError(ErrorCodeHTTPRequestURIInvalid), nil
		}

		// Execute request asynchronously
		future := &FutureIncomingResponse{
			doneCh: make(chan struct{}),
		}
		go func() {
			defer close(future.doneCh)
			resp, err := http.DefaultClient.Do(httpReq)
			if err != nil {
				future.err = err
				return
			}
			future.response = NewIncomingResponse(resp)
		}()

		futureHandle := table.New(future, true)
		return resultOk(component.ValOwn(futureHandle)), nil
	})

	return inst.Build()
}
```

**Test matching wasmtime p2_http_outbound_request_get:**

```go
func TestOutgoingHandler_Get(t *testing.T) {
	// Start a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-wasmtime-test-method", r.Method)
		w.Header().Set("x-wasmtime-test-uri", r.RequestURI)
		w.WriteHeader(200)
	}))
	defer server.Close()

	// Create outgoing request
	req := NewOutgoingRequest(NewFields())
	req.SetMethod(MethodGet)
	req.SetScheme(&SchemeHTTP)
	req.SetAuthority(server.Listener.Addr().String())
	req.SetPathWithQuery("/get?some=arg")

	// Execute via handler
	// ... verify response status and headers
}
```

---

## Integration Tests

### Test Component Generation

Create test components using cargo-component:

```
internal/component/testdata/wasip2/
├── cli_args.wasm          # Tests get-arguments
├── cli_env.wasm           # Tests get-environment
├── cli_exit.wasm          # Tests exit
├── clocks_sleep.wasm      # Tests monotonic-clock subscribe
├── random.wasm            # Tests random bytes generation
├── fs_read.wasm           # Tests file read
├── http_get.wasm          # Tests HTTP GET
└── tcp_connect.wasm       # Tests TCP client
```

### Integration Test Pattern

```go
// internal/component/wasip2_integration_test.go

func TestWASIP2_CLI_Args(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	binary, _ := testdata.ReadFile("testdata/wasip2/cli_args.wasm")
	compiled, err := rt.CompileComponent(ctx, binary)
	require.NoError(t, err)

	linker := rt.NewComponentLinker()
	config := wasip2.NewConfig().
		WithArgs([]string{"hello", "this", "", "is an argument", "with emoji"})
	err = wasip2.InstantiateWithConfig(linker, config)
	require.NoError(t, err)

	instance, err := linker.Instantiate(ctx, compiled)
	require.NoError(t, err)

	// The test component validates args and exits 0 on success
	run := instance.ExportedFunction("wasi:cli/run@0.2.0#run")
	results, err := run.Call(ctx)
	require.NoError(t, err)
	require.True(t, results[0].Result().IsOk())
}
```

---

## Running Tests

```bash
# Run all WASI P2 tests
go test ./imports/wasip2/... -v

# Run specific interface tests
go test ./imports/wasip2/io/... -v
go test ./imports/wasip2/cli/... -v
go test ./imports/wasip2/clocks/... -v
go test ./imports/wasip2/random/... -v
go test ./imports/wasip2/filesystem/... -v
go test ./imports/wasip2/sockets/... -v
go test ./imports/wasip2/http/... -v

# Integration tests
go test ./internal/component/... -v -run TestWASIP2

# With race detector
go test ./imports/wasip2/... -race -v
```

---

## Phase 5 Completion Checklist

- [ ] Package structure created (`imports/wasip2/`)
- [ ] Config with builder pattern for customization
- [ ] wasi:io/error with to-debug-string method
- [ ] wasi:io/poll with pollable resource and poll function
- [ ] wasi:io/streams with input-stream and output-stream resources
- [ ] wasi:clocks/wall-clock with now() and resolution()
- [ ] wasi:clocks/monotonic-clock with subscribe-duration/instant
- [ ] wasi:random/random with get-random-bytes and get-random-u64
- [ ] wasi:random/insecure with insecure random
- [ ] wasi:random/insecure-seed with deterministic seed
- [ ] wasi:cli/environment with get-environment and get-arguments
- [ ] wasi:cli/exit with exit function
- [ ] wasi:cli/stdin, stdout, stderr with stream getters
- [ ] wasi:cli/terminal interfaces
- [ ] wasi:filesystem/types with descriptor resource
- [ ] wasi:filesystem/preopens with get-directories
- [ ] wasi:sockets/network with error codes and addresses
- [ ] wasi:sockets/tcp with tcp-socket resource
- [ ] wasi:sockets/udp with udp-socket resource
- [ ] wasi:sockets/ip-name-lookup with DNS resolution
- [ ] wasi:http/types with request/response resources
- [ ] wasi:http/outgoing-handler for HTTP clients
- [ ] Integration tests with cargo-component built binaries
- [ ] Wasmtime test scenarios validated:
  - [ ] p2_stream_pollable_correct
  - [ ] p2_sleep
  - [ ] p2_random
  - [ ] p2_cli_args
  - [ ] p2_cli_env
  - [ ] p2_cli_exit_success
  - [ ] p2_tcp_bind
  - [ ] p2_tcp_connect
  - [ ] p2_udp_bind
  - [ ] p2_http_outbound_request_get

---

## References

- [WASI Preview 2 Specification](https://github.com/WebAssembly/WASI/tree/main/preview2)
- [wasi:io WIT](https://github.com/WebAssembly/wasi-io/tree/main/wit)
- [wasi:cli WIT](https://github.com/WebAssembly/wasi-cli/tree/main/wit)
- [wasi:clocks WIT](https://github.com/WebAssembly/wasi-clocks/tree/main/wit)
- [wasi:random WIT](https://github.com/WebAssembly/wasi-random/tree/main/wit)
- [wasi:filesystem WIT](https://github.com/WebAssembly/wasi-filesystem/tree/main/wit)
- [wasi:sockets WIT](https://github.com/WebAssembly/wasi-sockets/tree/main/wit)
- [wasi:http WIT](https://github.com/WebAssembly/wasi-http/tree/main/wit)
- [Wasmtime WASI Tests](https://github.com/bytecodealliance/wasmtime/tree/main/crates/test-programs/src/bin)
- [Component Model Bytecode Alliance](https://component-model.bytecodealliance.org/)
