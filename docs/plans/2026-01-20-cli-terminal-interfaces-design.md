# WASI P2 CLI Terminal Interfaces Design

## Overview

Implement the missing `wasi:cli` terminal interfaces to achieve full WASI P2 CLI conformance. This enables Go-compiled WebAssembly components (which use the full WASI adapter) to run on wazero.

## Problem

The Go plugin (`multi.wasm`) fails to instantiate because it requires three interfaces not currently implemented:

- `wasi:cli/terminal-stdin@0.2.0`
- `wasi:cli/terminal-stdout@0.2.0`
- `wasi:cli/terminal-stderr@0.2.0`

Error: `import "wasi:cli/terminal-stdin@0.2.6": no compatible definition`

## WASI P2 Specification

Per the official WIT definitions (`wasi:cli@0.2.8`):

```wit
interface terminal-stdin {
    use terminal-input.{terminal-input};
    get-terminal-stdin: func() -> option<terminal-input>;
}

interface terminal-stdout {
    use terminal-output.{terminal-output};
    get-terminal-stdout: func() -> option<terminal-output>;
}

interface terminal-stderr {
    use terminal-output.{terminal-output};
    get-terminal-stderr: func() -> option<terminal-output>;
}
```

These functions return `Some(handle)` if the stream is connected to a terminal (TTY), or `None` otherwise.

## Design

### Terminal Mode Configuration

Add configurable terminal detection via `WASIConfig`:

```go
type TerminalMode int

const (
    TerminalModeNone   TerminalMode = iota  // Always return None (safe default)
    TerminalModeAuto                         // Detect real TTY via os.File fd
    TerminalModeCustom                       // Use explicit host-provided values
)

// WASIConfig additions:
TerminalMode() TerminalMode
StdinIsTerminal() bool   // Used when TerminalModeCustom
StdoutIsTerminal() bool
StderrIsTerminal() bool
```

### Terminal Resource Types

```go
// Marker resources per WASI spec (no methods currently)
type TerminalInput struct{}
type TerminalOutput struct{}
```

### Auto-Detection

For `TerminalModeAuto`, use `golang.org/x/term.IsTerminal(fd)`:

```go
func detectTerminal(stream interface{}) bool {
    switch s := stream.(type) {
    case *os.File:
        return term.IsTerminal(int(s.Fd()))
    case interface{ Fd() uintptr }:
        return term.IsTerminal(int(s.Fd()))
    default:
        return false
    }
}
```

### Interface Implementation

```go
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

    table := component.ResourceTableFromContext(ctx)
    handle := table.New(&TerminalInput{}, true)
    val := component.ValOwn(uint32(handle))
    return []component.Val{component.ValOption(&val)}, nil
}
```

## Files to Modify

1. `internal/component/wasi_config.go` - Add TerminalMode type and interface methods
2. `imports/wasip2/cli/terminal.go` - New file for terminal types and detection
3. `imports/wasip2/cli/cli.go` - Add three new interface registrations
4. `imports/wasip2/cli/cli_test.go` - Add tests for new interfaces
5. `imports/wasip2/config.go` - Update default config implementation

## Test Strategy

TDD approach:
1. Write failing tests for interface registration
2. Write failing tests for each TerminalMode behavior
3. Implement to make tests pass
4. Verify Go plugin test passes

## Dependencies

- `golang.org/x/term` - Standard Go extended library for terminal detection
