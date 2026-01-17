# Component Model/WASI P2 Implementation Gap Analysis

**Date:** 2026-01-17
**Status:** Analysis Complete
**Goal:** Identify gaps between current implementation and spec/wasmtime for complete component model support

## Executive Summary

The wazero component model implementation has made significant progress on:
- Binary parsing (sections, types, canonicals, aliases, imports, exports)
- Type system (primitives, composites, resources)
- Canonical ABI (lift/lower for most types)
- WASI P2 interface definitions

However, **critical gaps prevent instantiation of real-world WASI P2 components**. The primary blocker is that wazero rejects empty import module names in core modules, which are valid and common in the component model.

## Test Results Summary

| Area | Status | Notes |
|------|--------|-------|
| Binary Parser | PASS | `add.wasm` and `subtract.wasm` parse successfully |
| Type System | PASS | All type tests pass |
| Canonical ABI | PASS | Lift/lower conformance tests pass |
| WASI P2 Interfaces | PASS | All interface registration tests pass |
| Component Instantiation | **FAIL** | `import[0] has an empty module name` |

## Critical Gap #1: Empty Module Name Validation

### Problem

wazero explicitly rejects empty import module names in `internal/wasm/module.go:481-483`:

```go
if imp.Module == "" {
    return fmt.Errorf("import[%d] has an empty module name", i)
}
```

### Why This Matters

In the component model, core modules inside components commonly use empty strings as import module names. For example, in `add.wasm`, module 3 has:

```wat
(import "" "0" (func (;0;) (type 0)))
(import "" "1" (func (;1;) (type 1)))
...
(import "" "$imports" (table (;0;) 15 15 funcref))
```

These are **placeholders** resolved during component instantiation via:
```wat
(core instance (;16;) (instantiate 3
    (with "" (instance 15))
  )
)
```

### Wasmtime Approach

Wasmtime:
- Does NOT relax validation for empty module names
- Treats `""` as a valid string like any other
- Defers import resolution to instantiation time
- Uses `args[""][import.name()]` lookup pattern

### Required Fix

Option A (Recommended): Add a compilation flag to allow empty module names:
```go
// In DecodeModule or CompileModule
allowEmptyModuleName bool  // true for component-embedded modules
```

Option B: Remove the validation entirely (may affect other uses)

Option C: Compile modules lazily during instantiation

## Critical Gap #2: Core Instance Instantiation

### Problem

The current `CompileComponent` eagerly compiles all core modules upfront (`runtime.go:460-474`), but:

1. Modules can't be compiled in isolation when they have unsatisfied imports
2. Core instances in components form a dependency graph that must be instantiated in order
3. Imports come from other core instance exports OR lowered component functions

### Current Flow (Broken)
```
CompileComponent
  → For each CoreModuleData
    → CompileModule (FAILS if module has empty import module name)
```

### Required Flow
```
CompileComponent
  → Parse component structure
  → Store raw module bytes (don't compile yet)

Instantiate
  → Process core instances in order
  → For each CoreInstanceExprInstantiate:
    → Resolve imports from:
      - Previously instantiated core instances
      - Lowered component import functions
    → Compile module with imports available
    → Instantiate module
```

### Key Data Structures Needed

```go
// CoreInstance describes how to instantiate a core module
type CoreInstance struct {
    Kind      CoreInstanceKind
    ModuleIdx uint32           // Index into CoreModules
    Args      []InstantiateArg // Import satisfiers
}

type InstantiateArg struct {
    Name       string  // Import module name to satisfy (e.g., "")
    InstanceIdx uint32 // Source core instance
    // OR
    Exports    []InlineExport // For inline instances
}
```

## Critical Gap #3: Canon Lower for Component Imports

### Problem

Component imports must be "lowered" to core wasm functions before core modules can use them. The current linker doesn't implement `canon lower`.

### Example from add.wasm

```wat
(alias export 6 "get-stderr" (func (;0;)))
(core func (;20;) (canon lower (func 0)))  -- MISSING IMPLEMENTATION
(core instance (;10;)
  (export "get-stderr" (func 20))
)
```

### Required Implementation

```go
// In linker or instantiation
func (l *ComponentLinker) canonLower(
    componentFunc Definition,  // Host-provided or component-imported function
    options CanonOptions,      // memory, realloc, string-encoding
) api.Function {
    return &loweredFunc{
        callback: func(ctx context.Context, coreArgs []uint64) []uint64 {
            // 1. Lift core args to component vals
            vals := lift(options, coreArgs)
            // 2. Call component function
            results := componentFunc.Call(ctx, vals)
            // 3. Lower results back to core
            return lower(options, results)
        },
    }
}
```

## Critical Gap #4: Resource Handle Operations

### Problem

Components use `canon resource.drop`, `canon resource.new`, and `canon resource.rep` operations. These are parsed but not executed during instantiation.

### Example from add.wasm

```wat
(alias export 8 "descriptor" (type (;18;)))
(core func (;5;) (canon resource.drop 18))  -- Creates a drop function
```

### Required Implementation

Resource operations need to:
1. Create callable functions that interact with the resource table
2. Be wired into core instance imports
3. Track ownership (own vs borrow) correctly

## Gap #5: Type Aliasing and Index Spaces

### Problem

Components use complex aliasing to reference types from outer scopes:
```wat
(alias outer $add-plugin 3 (type (;1;)))
```

The current implementation parses aliases but doesn't fully resolve them during type checking.

### Impact

- Type validation may be incomplete
- Function signatures may not match expected types

## Gap #6: Component Instance Types

### Problem

When a component imports an instance (like `wasi:cli/environment@0.2.3`), the import has an instance type that describes expected exports:

```wat
(type (;0;)
  (instance
    (type (;0;) (tuple string string))
    (type (;1;) (list 0))
    (type (;2;) (func (result 1)))
    (export (;0;) "get-environment" (func (type 2)))
  )
)
(import "wasi:cli/environment@0.2.3" (instance (;0;) (type 0)))
```

The linker must validate that provided definitions match the expected instance type.

### Current State

The linker does basic name matching but doesn't validate:
- Function signatures match
- All required exports are present
- Types are compatible

## WASI P2 Implementation Status

| Interface | Registration | Implementation |
|-----------|--------------|----------------|
| wasi:cli/environment | ✅ | Partial |
| wasi:cli/exit | ✅ | ✅ |
| wasi:cli/stdin | ✅ | ✅ |
| wasi:cli/stdout | ✅ | ✅ |
| wasi:cli/stderr | ✅ | ✅ |
| wasi:io/error | ✅ | ✅ |
| wasi:io/streams | ✅ | Partial |
| wasi:io/poll | ✅ | Partial |
| wasi:clocks/monotonic | ✅ | ✅ |
| wasi:clocks/wall | ✅ | ✅ |
| wasi:random/random | ✅ | ✅ |
| wasi:random/insecure | ✅ | ✅ |
| wasi:filesystem/types | ✅ | Partial |
| wasi:filesystem/preopens | ✅ | Partial |
| wasi:sockets/* | ✅ | Partial |
| wasi:http/* | ✅ | Stub |

**Note:** "Partial" means basic functions exist but edge cases or full spec compliance is not verified.

## Remediation Plan

### Phase 1: Enable Core Module Compilation (CRITICAL)

**Priority: P0 - Blocker**

1. Add `allowEmptyModuleName` flag to module validation
2. Modify `CompileComponent` to use relaxed validation for embedded modules
3. Test: `subtract.wasm` should compile (simpler, fewer deps)

**Files to modify:**
- `internal/wasm/module.go` - Add validation flag
- `internal/wasm/binary/decoder.go` - Pass flag through
- `runtime.go` - Use flag for component modules

### Phase 2: Implement Deferred Core Module Instantiation

**Priority: P0 - Blocker**

1. Refactor `CompileComponent` to store raw module bytes
2. Implement ordered core instance instantiation in linker
3. Build import resolution from dependency graph
4. Test: `add.wasm` should instantiate with WASI stubs

**Files to modify:**
- `internal/component/component_linker.go`
- `internal/component/instantiate.go`
- `runtime.go`

### Phase 3: Implement Canon Lower

**Priority: P0 - Required for WASI**

1. Add `canonLower` to create core functions from component functions
2. Wire lowered functions into core instance imports
3. Handle canonical options (memory, realloc, string-encoding)

**Files to modify:**
- `internal/component/component_linker.go`
- `internal/component/abi/lower.go`

### Phase 4: Implement Resource Operations

**Priority: P1 - Required for most WASI interfaces**

1. Implement `resource.new`, `resource.drop`, `resource.rep`
2. Wire into core instance creation
3. Test with WASI streams (use input-stream/output-stream resources)

### Phase 5: Type Validation

**Priority: P2 - Correctness**

1. Implement instance type validation in linker
2. Add function signature matching
3. Validate alias resolution

### Phase 6: Complete WASI P2

**Priority: P2 - Full functionality**

1. Review each WASI interface against spec
2. Implement missing methods
3. Add conformance tests

## Test Plan

1. **Unit Tests**
   - Module validation with empty import names
   - Canon lower for each type
   - Resource table operations

2. **Integration Tests**
   - `subtract.wasm` (simple, no WASI deps in core)
   - `add.wasm` (full WASI P2 dependencies)

3. **Conformance Tests**
   - Port wasmtime component model tests
   - WASI test suite

## Appendix: Component Structure Analysis

### subtract.wasm (Simple)
- 1 core module (self-contained, no imports)
- No component imports needed
- Direct function exports

### add.wasm (Complex WASI P2)
- 4 core modules
- 10 component imports (WASI interfaces)
- 16+ core instances with complex wiring
- Uses canon lower, resource.drop
- Indirect function tables for WASI P1→P2 adaptation

## References

- [Component Model Binary Format](https://github.com/WebAssembly/component-model/blob/main/design/mvp/Binary.md)
- [Canonical ABI Specification](https://github.com/WebAssembly/component-model/blob/main/design/mvp/CanonicalABI.md)
- [Wasmtime Component Implementation](https://github.com/bytecodealliance/wasmtime/tree/main/crates/wasmtime/src/runtime/component)
- [WASI Preview 2 Specification](https://github.com/WebAssembly/WASI/tree/main/preview2)
