# Component Model Phase 5: WASI Preview 2

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Parent Plan:** [2026-01-12-component-model-implementation.md](./2026-01-12-component-model-implementation.md)
**Design Doc:** [2026-01-12-component-model-design.md](./2026-01-12-component-model-design.md)
**Previous Phase:** [Phase 4: Full Instantiation & Linking](./2026-01-12-component-model-phase4-linking.md)
**Status:** NOT STARTED
**Estimated Tasks:** 151-240

---

## Overview

This phase implements all WASI Preview 2 interfaces, providing the standard system interfaces for component model applications.

**Goal:** Complete WASI Preview 2 implementation that works out-of-box with pluggable configuration.

**Prerequisites:**
- Phase 1-4 complete (full component model runtime)

---

## Phase 5 Milestones

| Milestone | Description | Success Criteria |
|-----------|-------------|------------------|
| 5.1 | wasi:cli | Environment, exit, terminal interfaces |
| 5.2 | wasi:filesystem | File operations, preopens |
| 5.3 | wasi:io | Streams, poll, error |
| 5.4 | wasi:clocks | Monotonic and wall clocks |
| 5.5 | wasi:random | Random number generation |
| 5.6 | wasi:sockets | TCP, UDP, name lookup |
| 5.7 | wasi:http | HTTP client and server |

---

## WASI P2 Architecture

From the design doc:

```go
// imports/wasip2/wasip2.go

func Instantiate(ctx context.Context, linker wazero.ComponentLinker) error {
    if err := cli.Instantiate(ctx, linker); err != nil {
        return err
    }
    if err := filesystem.Instantiate(ctx, linker); err != nil {
        return err
    }
    if err := io.Instantiate(ctx, linker); err != nil {
        return err
    }
    if err := clocks.Instantiate(ctx, linker); err != nil {
        return err
    }
    if err := random.Instantiate(ctx, linker); err != nil {
        return err
    }
    if err := sockets.Instantiate(ctx, linker); err != nil {
        return err
    }
    if err := http.Instantiate(ctx, linker); err != nil {
        return err
    }
    return nil
}
```

---

## Package Structure

```
imports/wasip2/
├── wasip2.go           # Top-level Instantiate function
├── cli/
│   ├── cli.go          # wasi:cli implementation
│   ├── environment.go  # get-environment, get-arguments
│   ├── exit.go         # exit
│   ├── stdin.go        # get-stdin
│   ├── stdout.go       # get-stdout, get-stderr
│   └── terminal.go     # terminal I/O
├── filesystem/
│   ├── filesystem.go   # wasi:filesystem implementation
│   ├── types.go        # Descriptor, path types
│   └── preopens.go     # get-directories
├── io/
│   ├── io.go           # wasi:io implementation
│   ├── streams.go      # input-stream, output-stream
│   ├── poll.go         # pollable
│   └── error.go        # stream-error
├── clocks/
│   ├── clocks.go       # wasi:clocks implementation
│   ├── monotonic.go    # monotonic-clock
│   └── wall.go         # wall-clock
├── random/
│   ├── random.go       # wasi:random implementation
│   ├── random.go       # get-random-bytes
│   └── insecure.go     # insecure-random
├── sockets/
│   ├── sockets.go      # wasi:sockets implementation
│   ├── tcp.go          # tcp-socket
│   ├── udp.go          # udp-socket
│   └── resolve.go      # ip-name-lookup
└── http/
    ├── http.go         # wasi:http implementation
    ├── types.go        # request, response types
    ├── incoming.go     # incoming-handler
    └── outgoing.go     # outgoing-handler
```

---

## Tasks

### Task 151-160: wasi:io (Foundation)

The IO interfaces are foundational - other WASI interfaces depend on them.

**Files:**
- Create: `imports/wasip2/io/io.go`
- Create: `imports/wasip2/io/streams.go`
- Create: `imports/wasip2/io/poll.go`

**Task 151: Define stream resource types**

```go
// imports/wasip2/io/streams.go

package io

import (
	"context"
	"io"

	"github.com/tetratelabs/wazero/internal/component"
)

// InputStream represents a readable byte stream.
type InputStream struct {
	reader io.Reader
	closed bool
}

// OutputStream represents a writable byte stream.
type OutputStream struct {
	writer io.Writer
	closed bool
}

// Pollable represents something that can be polled.
type Pollable struct {
	ready bool
}

// Define the resource types for the linker
func Instantiate(ctx context.Context, linker component.Linker) error {
	inst := linker.DefineInstance("wasi:io/streams@0.2.0")

	// Resource definitions
	inst.Resource("input-stream", func(rep uint32) {
		// Destructor - clean up stream
	})
	inst.Resource("output-stream", func(rep uint32) {
		// Destructor - flush and close
	})

	// Methods
	inst.Func("[method]input-stream.read", inputStreamRead)
	inst.Func("[method]input-stream.blocking-read", inputStreamBlockingRead)
	inst.Func("[method]output-stream.write", outputStreamWrite)
	inst.Func("[method]output-stream.blocking-write-and-flush", outputStreamBlockingWriteAndFlush)

	return inst.Build()
}

func inputStreamRead(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// self: borrow<input-stream>
	// len: u64
	// returns: result<list<u8>, stream-error>
	// ...
}
```

**Task 152: Implement input-stream.read**

```go
func inputStreamRead(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	maxLen := args[1].U64()

	// Get the stream from resource table
	table := component.ResourceTableFromContext(ctx)
	streamData, err := table.Rep(handle)
	if err != nil {
		return resultError(streamErrorClosed), nil
	}

	stream := streamData.(*InputStream)
	if stream.closed {
		return resultError(streamErrorClosed), nil
	}

	// Read up to maxLen bytes
	buf := make([]byte, min(maxLen, 64*1024))
	n, err := stream.reader.Read(buf)
	if err == io.EOF {
		return resultOk(component.ValList(nil)), nil // Empty list signals EOF
	}
	if err != nil {
		return resultError(streamErrorLastOperationFailed), nil
	}

	// Convert to list<u8>
	vals := make([]component.Val, n)
	for i := 0; i < n; i++ {
		vals[i] = component.ValU8(buf[i])
	}
	return resultOk(component.ValList(vals)), nil
}
```

**Task 153-160: Complete io streams**

- `[method]input-stream.blocking-read`
- `[method]input-stream.skip`
- `[method]input-stream.subscribe` (returns pollable)
- `[method]output-stream.write`
- `[method]output-stream.blocking-write-and-flush`
- `[method]output-stream.flush`
- `[method]output-stream.subscribe`

---

### Task 161-170: wasi:cli

**Files:**
- Create: `imports/wasip2/cli/cli.go`
- Create: `imports/wasip2/cli/environment.go`

**Task 161: Environment interface**

```go
// imports/wasip2/cli/environment.go

package cli

import (
	"context"
	"os"

	"github.com/tetratelabs/wazero/internal/component"
)

func Instantiate(ctx context.Context, linker component.Linker) error {
	env := linker.DefineInstance("wasi:cli/environment@0.2.0")

	env.Func("get-environment", getEnvironment)
	env.Func("get-arguments", getArguments)

	return env.Build()
}

func getEnvironment(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Returns: list<tuple<string, string>>
	envVars := os.Environ()
	tuples := make([]component.Val, len(envVars))

	for i, env := range envVars {
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
}

func getArguments(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Returns: list<string>
	argList := make([]component.Val, len(os.Args))
	for i, arg := range os.Args {
		argList[i] = component.ValString(arg)
	}
	return []component.Val{component.ValList(argList)}, nil
}
```

**Task 162-165: stdin/stdout/stderr**

```go
// imports/wasip2/cli/stdin.go

func InstantiateStdin(ctx context.Context, linker component.Linker) error {
	inst := linker.DefineInstance("wasi:cli/stdin@0.2.0")

	inst.Func("get-stdin", func(ctx context.Context, args []component.Val) ([]component.Val, error) {
		// Returns: own<input-stream>
		table := component.ResourceTableFromContext(ctx)
		stream := &io.InputStream{reader: os.Stdin}
		handle := table.New(stream)
		return []component.Val{component.ValOwn(handle)}, nil
	})

	return inst.Build()
}
```

**Task 166-170: exit, terminal**

---

### Task 171-180: wasi:filesystem

**Files:**
- Create: `imports/wasip2/filesystem/filesystem.go`
- Create: `imports/wasip2/filesystem/types.go`
- Create: `imports/wasip2/filesystem/preopens.go`

**Key interfaces:**
- `wasi:filesystem/types` - Descriptor resource, stat, read, write, etc.
- `wasi:filesystem/preopens` - Pre-opened directories

```go
// imports/wasip2/filesystem/types.go

// Descriptor represents an open file or directory.
type Descriptor struct {
	file    *os.File
	isDir   bool
	path    string
}

// FileType represents the type of a file.
type FileType uint8
const (
	FileTypeUnknown FileType = iota
	FileTypeBlockDevice
	FileTypeCharacterDevice
	FileTypeDirectory
	FileTypeFIFO
	FileTypeSymbolicLink
	FileTypeRegularFile
	FileTypeSocket
)

func Instantiate(ctx context.Context, linker component.Linker) error {
	types := linker.DefineInstance("wasi:filesystem/types@0.2.0")

	types.Resource("descriptor", descriptorDestructor)

	// Methods
	types.Func("[method]descriptor.read", descriptorRead)
	types.Func("[method]descriptor.write", descriptorWrite)
	types.Func("[method]descriptor.stat", descriptorStat)
	types.Func("[method]descriptor.read-directory", descriptorReadDirectory)
	types.Func("[method]descriptor.open-at", descriptorOpenAt)
	// ... more methods

	return types.Build()
}
```

---

### Task 181-190: wasi:clocks

**Files:**
- Create: `imports/wasip2/clocks/clocks.go`
- Create: `imports/wasip2/clocks/monotonic.go`
- Create: `imports/wasip2/clocks/wall.go`

```go
// imports/wasip2/clocks/monotonic.go

func Instantiate(ctx context.Context, linker component.Linker) error {
	mono := linker.DefineInstance("wasi:clocks/monotonic-clock@0.2.0")

	mono.Func("now", func(ctx context.Context, args []component.Val) ([]component.Val, error) {
		// Returns: instant (u64 nanoseconds)
		now := time.Now().UnixNano()
		return []component.Val{component.ValU64(uint64(now))}, nil
	})

	mono.Func("resolution", func(ctx context.Context, args []component.Val) ([]component.Val, error) {
		// Returns: duration (u64 nanoseconds)
		return []component.Val{component.ValU64(1)}, nil // 1ns resolution
	})

	mono.Func("subscribe-instant", subscribeInstant)
	mono.Func("subscribe-duration", subscribeDuration)

	return mono.Build()
}
```

---

### Task 191-200: wasi:random

```go
// imports/wasip2/random/random.go

import "crypto/rand"

func Instantiate(ctx context.Context, linker component.Linker) error {
	random := linker.DefineInstance("wasi:random/random@0.2.0")

	random.Func("get-random-bytes", func(ctx context.Context, args []component.Val) ([]component.Val, error) {
		len := args[0].U64()
		buf := make([]byte, len)
		_, err := rand.Read(buf)
		if err != nil {
			return nil, err
		}

		vals := make([]component.Val, len)
		for i, b := range buf {
			vals[i] = component.ValU8(b)
		}
		return []component.Val{component.ValList(vals)}, nil
	})

	random.Func("get-random-u64", func(ctx context.Context, args []component.Val) ([]component.Val, error) {
		var buf [8]byte
		_, err := rand.Read(buf[:])
		if err != nil {
			return nil, err
		}
		n := binary.LittleEndian.Uint64(buf[:])
		return []component.Val{component.ValU64(n)}, nil
	})

	return random.Build()
}
```

---

### Task 201-220: wasi:sockets

Implement TCP/UDP socket interfaces:
- `wasi:sockets/tcp` - TCP socket resource and operations
- `wasi:sockets/udp` - UDP datagram socket
- `wasi:sockets/ip-name-lookup` - DNS resolution
- `wasi:sockets/network` - Network configuration

---

### Task 221-240: wasi:http

Implement HTTP interfaces:
- `wasi:http/types` - Request, response, headers, body types
- `wasi:http/incoming-handler` - Server-side request handling
- `wasi:http/outgoing-handler` - Client-side HTTP requests

```go
// imports/wasip2/http/types.go

// Request represents an HTTP request.
type Request struct {
	method  Method
	uri     string
	headers Headers
	body    *InputStream
}

// Response represents an HTTP response.
type Response struct {
	status  uint16
	headers Headers
	body    *OutputStream
}
```

---

## Configuration

WASI P2 should support pluggable configuration:

```go
// imports/wasip2/config.go

type Config struct {
	// Environment overrides os.Environ()
	Environ []string

	// Args overrides os.Args
	Args []string

	// Preopens maps guest paths to host paths
	Preopens map[string]string

	// Stdin/Stdout/Stderr override defaults
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	// AllowNetwork enables socket operations
	AllowNetwork bool

	// AllowHTTP enables HTTP operations
	AllowHTTP bool
}

func InstantiateWithConfig(ctx context.Context, linker component.Linker, config *Config) error {
	// Apply config to all subsystems
	// ...
}
```

---

## Integration Tests

Test components built with cargo-component:

```
internal/component/testdata/
├── wasip2/
│   ├── cli_args.wasm        # Read args and env
│   ├── fs_read.wasm         # Read a file
│   ├── fs_write.wasm        # Write a file
│   ├── http_fetch.wasm      # HTTP GET request
│   ├── random.wasm          # Generate random bytes
│   └── wasip2.wit
```

---

## Running Tests

```bash
# Run all WASI P2 tests
go test ./imports/wasip2/... -v

# Run specific interface tests
go test ./imports/wasip2/cli/... -v
go test ./imports/wasip2/filesystem/... -v
go test ./imports/wasip2/io/... -v

# Integration tests
go test ./internal/component/... -v -run TestWASIP2
```

---

## References

- [WASI Preview 2 Spec](https://github.com/WebAssembly/WASI/tree/main/preview2)
- [wasi-cli WIT](https://github.com/WebAssembly/wasi-cli/tree/main/wit)
- [wasi-filesystem WIT](https://github.com/WebAssembly/wasi-filesystem/tree/main/wit)
- [wasi-io WIT](https://github.com/WebAssembly/wasi-io/tree/main/wit)
- [wasi-http WIT](https://github.com/WebAssembly/wasi-http/tree/main/wit)
