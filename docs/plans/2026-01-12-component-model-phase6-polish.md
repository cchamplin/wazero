# Component Model Phase 6: Polish & Conformance

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Parent Plan:** [2026-01-12-component-model-implementation.md](./2026-01-12-component-model-implementation.md)
**Design Doc:** [2026-01-12-component-model-design.md](./2026-01-12-component-model-design.md)
**Previous Phase:** [Phase 5: WASI Preview 2](./2026-01-12-component-model-phase5-wasip2.md)
**Status:** NOT STARTED
**Estimated Tasks:** 241-280

---

## Overview

This phase focuses on conformance testing against wasmtime's test suite, performance optimization, and comprehensive documentation.

**Goal:** Production-ready component model implementation with verified conformance and good performance.

**Prerequisites:**
- Phases 1-5 complete (full component model and WASI P2)

---

## Phase 6 Milestones

| Milestone | Description | Success Criteria |
|-----------|-------------|------------------|
| 6.1 | Wasmtime conformance | Port and pass wasmtime test suite |
| 6.2 | Edge case testing | All spec edge cases covered |
| 6.3 | Performance optimization | Benchmarks within 2x of wasmtime |
| 6.4 | API documentation | Complete godoc coverage |
| 6.5 | Examples | Working examples for common use cases |
| 6.6 | Release preparation | CI/CD, versioning, changelog |

---

## Tasks

### Task 241-250: Wasmtime Test Suite Port

Port relevant tests from wasmtime's component model test suite.

**Source:** https://github.com/bytecodealliance/wasmtime/tree/main/tests/all/component_model

**Categories to port:**
- Binary format parsing tests
- Type system tests
- Canonical ABI tests
- Resource tests
- Instance/linking tests
- WASI P2 tests

**Step 1: Set up test infrastructure**

```go
// internal/component/conformance/conformance_test.go

package conformance

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/internal/component/binary"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestWasmtimeConformance(t *testing.T) {
	// Walk test fixtures directory
	testDir := "testdata/wasmtime"
	entries, err := os.ReadDir(testDir)
	require.NoError(t, err)

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".wasm" {
			name := entry.Name()
			t.Run(name, func(t *testing.T) {
				data, err := os.ReadFile(filepath.Join(testDir, name))
				require.NoError(t, err)

				// Parse and instantiate
				ctx := context.Background()
				rt := wazero.NewRuntime(ctx)
				defer rt.Close(ctx)

				c, err := binary.DecodeComponent(data)
				require.NoError(t, err, "parse should succeed for %s", name)

				// Additional validation based on test type...
			})
		}
	}
}
```

**Step 2: Port specific test categories**

```
internal/component/conformance/testdata/wasmtime/
├── parse/           # Binary parsing tests
├── types/           # Type system tests
├── abi/             # Canonical ABI tests
├── resources/       # Resource lifecycle tests
├── linking/         # Import/export tests
└── wasi/            # WASI P2 tests
```

---

### Task 251-260: Edge Case Testing

Cover edge cases from the Component Model spec:

**Binary format edge cases:**
- Maximum section sizes
- Empty components
- Maximum nesting depth
- Invalid section order
- Truncated sections

**Type system edge cases:**
- Empty records/tuples
- Single-case variants
- Zero-length flags
- Deeply nested types
- Maximum type index

**ABI edge cases:**
- Exactly 16 flat params
- Exactly 17 flat params (spills to memory)
- Maximum string length
- Empty lists
- Maximum list length

**Resource edge cases:**
- Maximum borrow depth
- Concurrent borrows
- Drop during active borrow
- Generation wrap-around

```go
// internal/component/edge_cases_test.go

func TestEdgeCases_EmptyRecord(t *testing.T) {
	// record {} has size 0, align 1
	r := types.Record{Fields: nil}
	require.Equal(t, uint32(0), r.Size())
	require.Equal(t, uint32(1), r.Align())
}

func TestEdgeCases_MaxFlatParams(t *testing.T) {
	// Function with exactly 16 i32 params should use flat ABI
	// Function with 17 i32 params should use heap ABI

	// Build test components...
}

func TestEdgeCases_ResourceGenerationWrap(t *testing.T) {
	table := component.NewResourceTable()

	// Rapidly create/drop to cycle through generations
	for i := uint64(0); i < (1 << 32); i++ {
		h := table.New(i)
		_, _ = table.Drop(h)
	}

	// Verify handles still work correctly after wrap
	h := table.New("after-wrap")
	data, err := table.Rep(h)
	require.NoError(t, err)
	require.Equal(t, "after-wrap", data)
}
```

---

### Task 261-270: Performance Optimization

Identify and optimize hot paths:

**Benchmarks to create:**

```go
// internal/component/bench_test.go

func BenchmarkDecodeComponent(b *testing.B) {
	data := loadTestComponent("bench/simple.wasm")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = binary.DecodeComponent(data)
	}
}

func BenchmarkInstantiateSimple(b *testing.B) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	data := loadTestComponent("bench/simple.wasm")
	c, _ := binary.DecodeComponent(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		inst, _ := component.Instantiate(ctx, rt, c)
		_ = inst
	}
}

func BenchmarkCallPrimitive(b *testing.B) {
	// Setup...
	add := inst.ExportedFunction("add")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = add.Call(ctx, component.ValS32(2), component.ValS32(3))
	}
}

func BenchmarkLiftLowerRecord(b *testing.B) {
	// Benchmark record serialization/deserialization
}

func BenchmarkResourceTableOps(b *testing.B) {
	table := component.NewResourceTable()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h := table.New(i)
		_, _ = table.Drop(h)
	}
}
```

**Optimization targets:**
1. Decoder memory allocation (pooling)
2. Lift/lower hot paths (avoid reflection)
3. Resource table free list
4. Type lookup caching
5. String encoding buffers

---

### Task 271-275: API Documentation

Complete godoc documentation for public API:

```go
// api/component.go

// CompiledComponent is a pre-validated component ready for instantiation.
//
// Unlike CompiledModule, a CompiledComponent may contain multiple nested
// modules and components, with rich type definitions and canonical ABI
// lift/lower operations.
//
// # Example
//
//	ctx := context.Background()
//	rt := wazero.NewRuntime(ctx)
//	defer rt.Close(ctx)
//
//	compiled, err := rt.CompileComponent(ctx, wasmBytes)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	linker := rt.NewComponentLinker()
//	instance, err := linker.Instantiate(ctx, compiled)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	add := instance.ExportedFunction("add")
//	results, _ := add.Call(ctx, component.ValS32(2), component.ValS32(3))
//	fmt.Println(results[0].S32()) // 5
type CompiledComponent interface {
	// Imports returns the component's imports.
	// Each import specifies a name and expected type.
	Imports() []ComponentImport

	// Exports returns the component's exports.
	// Each export specifies a name and the kind of item exported.
	Exports() []ComponentExport

	// Close releases resources associated with this compiled component.
	// After calling Close, the component cannot be instantiated.
	Close(context.Context) error
}
```

---

### Task 276-278: Examples

Create runnable examples:

```
examples/
├── component_hello/        # Simple hello world component
│   ├── main.go
│   ├── hello.wasm
│   └── README.md
├── component_calculator/   # Calculator with multiple functions
│   ├── main.go
│   ├── calc.wasm
│   └── README.md
├── component_wasi/         # WASI P2 file I/O example
│   ├── main.go
│   ├── fileio.wasm
│   └── README.md
└── component_compose/      # Component composition example
    ├── main.go
    ├── lib.wasm
    ├── app.wasm
    └── README.md
```

**Example: Hello World**

```go
// examples/component_hello/main.go

package main

import (
	"context"
	"fmt"
	"log"
	_ "embed"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/internal/component"
)

//go:embed hello.wasm
var helloWasm []byte

func main() {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	// Compile the component
	compiled, err := rt.CompileComponent(ctx, helloWasm)
	if err != nil {
		log.Fatal(err)
	}

	// Create linker and instantiate
	linker := rt.NewComponentLinker()
	instance, err := linker.Instantiate(ctx, compiled)
	if err != nil {
		log.Fatal(err)
	}

	// Call the greet function
	greet := instance.ExportedFunction("greet")
	results, err := greet.Call(ctx, component.ValString("World"))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(results[0].String()) // "Hello, World!"
}
```

---

### Task 279-280: Release Preparation

**CI/CD updates:**
- Add component tests to CI matrix
- Benchmark tracking
- Component fixture generation

**Version and changelog:**
```markdown
## [0.x.0] - YYYY-MM-DD

### Added
- WebAssembly Component Model support
- WASI Preview 2 implementation
- `Runtime.CompileComponent()` and `Runtime.NewComponentLinker()` APIs
- Component-specific `Val` type for dynamic values
- Resource handle management with generation counting

### Component Model Features
- Full binary format parsing (Component Model MVP)
- All WIT types: primitives, records, variants, lists, options, results, flags, enums, tuples
- Canonical ABI lift/lower for all types
- Resource ownership and borrowing semantics
- Component composition and linking
- Semver-compatible import resolution

### WASI Preview 2
- wasi:cli (environment, exit, stdin/stdout/stderr, terminal)
- wasi:filesystem (types, preopens)
- wasi:io (streams, poll, error)
- wasi:clocks (monotonic, wall)
- wasi:random (random, insecure)
- wasi:sockets (tcp, udp, ip-name-lookup)
- wasi:http (types, incoming-handler, outgoing-handler)
```

---

## Running Tests

```bash
# Run all conformance tests
go test ./internal/component/conformance/... -v

# Run benchmarks
go test ./internal/component/... -bench=. -benchmem

# Generate coverage report
go test ./internal/component/... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Run examples
go run ./examples/component_hello/
```

---

## References

- [Wasmtime Component Tests](https://github.com/bytecodealliance/wasmtime/tree/main/tests/all/component_model)
- [Component Model Spec](https://github.com/WebAssembly/component-model)
- [wazero Documentation Guidelines](https://github.com/tetratelabs/wazero/blob/main/CONTRIBUTING.md)
