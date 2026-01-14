# Component Model Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement full WebAssembly Component Model and WASI Preview 2 support in wazero.

**Architecture:** Parallel `internal/component/` package structure with single-pass streaming binary parser, hybrid lift/lower (dynamic Val + interfaces), generation-counted resource handles, and engine-agnostic component orchestration layer.

**Tech Stack:** Go (zero external dependencies), wazero core wasm runtime, cargo-component/wasm-tools for test fixture generation.

**Design Doc:** [2026-01-12-component-model-design.md](./2026-01-12-component-model-design.md)

---

## Implementation Status

| Phase | Name | Status | Tasks | Plan File |
|-------|------|--------|-------|-----------|
| 1 | Binary Parser & Primitives | **COMPLETE** | 1-25 | [phase1-binary-parser.md](./2026-01-12-component-model-phase1-binary-parser.md) |
| 2 | Complete Type System | **COMPLETE** | 31-70 | [phase2-type-system.md](./2026-01-12-component-model-phase2-type-system.md) |
| 3 | Resources | **COMPLETE** | 71-100 | [phase3-resources.md](./2026-01-12-component-model-phase3-resources.md) |
| 4 | Full Instantiation & Linking | **COMPLETE** | 101-150 | [phase4-linking.md](./2026-01-12-component-model-phase4-linking.md) |
| 5 | WASI Preview 2 | NOT STARTED | 151-240 | [phase5-wasip2.md](./2026-01-12-component-model-phase5-wasip2.md) |
| 6 | Polish & Conformance | NOT STARTED | 241-280 | [phase6-polish.md](./2026-01-12-component-model-phase6-polish.md) |

---

## Phase Summaries

### Phase 1: Binary Parser & Primitives (COMPLETE)

Establishes the foundation: detecting component binaries, parsing sections, and executing a simple `add(s32, s32) -> s32` function.

**Completed:**
- Component package structure (`internal/component/`)
- Binary format parser with preamble validation
- Section header parsing and core module extraction
- Primitive value types and dynamic Val type
- Type section parsing (function types)
- Canonical section parsing (lift/lower)
- Export section parsing
- Basic component instantiation
- Working `add(s32, s32) -> s32` test

**Recent commits:**
- `00c3ae05` test(component): add edge case tests for add_s32 function
- `f2fa275c` feat(component): implement basic component instantiation
- `7dc99d93` feat(component): add Instance and ExportedFunc structures
- `e1f83096` feat(component): add add_s32 test component fixture
- `a462f223` feat(component): integrate export section parsing into decoder

---

### Phase 2: Complete Type System (COMPLETE)

Implements all WIT composite types and their Canonical ABI lift/lower operations.

**Completed:**
- Composite type definitions (record, variant, list, option, result, flags, enum, tuple)
- Memory layout calculations (`Size()`, `Align()`, `FlattenCount()`)
- Flat ABI lift/lower for all types
- Heap ABI lift/lower for all types
- String encoding (UTF-8, UTF-16, Latin1+UTF16)
- Integration test components (echo_record, option_roundtrip, list_sum, result_divide)
- 324 passing tests

**Key files created:**
- `internal/component/types/composite.go` - All composite type definitions
- `internal/component/abi/lift.go` - LiftFlat and LiftHeap implementations
- `internal/component/abi/lower.go` - LowerFlat and LowerHeap implementations
- `internal/component/abi/strings.go` - String encoding/decoding
- `internal/component/abi/context.go` - LiftContext and LowerContext
- `internal/component/testdata/gen/` - Test component generators

---

### Phase 3: Resources (COMPLETE)

Implements the Component Model's resource system with generation-counted handle tables.

**Completed:**
- Resource type definitions (Own, Borrow types implementing ValType)
- Generation-counted ResourceTable with use-after-free protection
- BorrowScope and CallContext for call-scoped borrow tracking
- Binary parsing for resource (0x3f), own (0x69), borrow (0x68) types
- Canonical resource operations (resource.new, resource.rep, resource.drop)
- LiftOwn, LowerOwn, LiftBorrow, LowerBorrow ABI operations
- Destructor invocation for owned handles
- ExportedFunc.Call integration with own/borrow parameter/result handling

**Key files created:**
- `internal/component/types/resource.go`
- `internal/component/resource_table.go`
- `internal/component/borrow_scope.go`
- `internal/component/call_context.go`

---

### Phase 4: Full Instantiation & Linking

Complete component instantiation and linking with semver-compatible import resolution.

**Milestones:**
- Alias section parsing
- Import section parsing
- Linker with semver matching
- Nested component support
- Instance exports

**Key files to create:**
- `internal/component/linker.go`

---

### Phase 5: WASI Preview 2

Implements all WASI Preview 2 interfaces.

**Interfaces:**
- `wasi:cli` - environment, exit, terminal
- `wasi:filesystem` - file operations, preopens
- `wasi:io` - streams, poll
- `wasi:clocks` - monotonic, wall
- `wasi:random` - random bytes
- `wasi:sockets` - TCP, UDP, DNS
- `wasi:http` - client and server

**Key files to create:**
- `imports/wasip2/` package hierarchy

---

### Phase 6: Polish & Conformance

Production-ready implementation with conformance testing and documentation.

**Milestones:**
- Wasmtime test suite port
- Edge case coverage
- Performance optimization
- API documentation
- Examples

---

## Running Tests

```bash
# Run all component tests
go test ./internal/component/... -v

# Run specific phase tests
go test ./internal/component/binary/... -v
go test ./internal/component/types/... -v

# Run with race detector
go test ./internal/component/... -race -v

# Run benchmarks
go test ./internal/component/... -bench=. -benchmem
```

---

## References

- [Component Model Binary Format](https://github.com/WebAssembly/component-model/blob/main/design/mvp/Binary.md)
- [Canonical ABI Specification](https://github.com/WebAssembly/component-model/blob/main/design/mvp/CanonicalABI.md)
- [Component Model Explainer](https://github.com/WebAssembly/component-model/blob/main/design/mvp/Explainer.md)
- [WIT Reference](https://component-model.bytecodealliance.org/design/wit.html)
- [WASI Preview 2](https://github.com/WebAssembly/WASI/tree/main/preview2)
- [Wasmtime Component Model](https://github.com/bytecodealliance/wasmtime/tree/main/crates/wasmtime/src/runtime/component)
