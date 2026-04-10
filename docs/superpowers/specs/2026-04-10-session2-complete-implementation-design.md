# Session 2 — Complete Component Model & WASI P2 Implementation

**Date:** 2026-04-10
**Status:** Design approved, ready for implementation planning
**Scope:** Final session. Cross-instance resources, post-return protocol, pipeline completion, spectest runner, public API, all test skips resolved.
**Previous sessions:**
- `docs/superpowers/specs/2026-04-07-canonical-abi-type-unification-design.md` (Session 0)
- `docs/plans/2026-04-07-canonical-abi-unification-session0-followup.md` (Session 0 followup)
- `docs/superpowers/specs/2026-04-08-canonical-abi-session1-design.md` (Session 1)
- `docs/superpowers/plans/2026-04-08-canonical-abi-session1-plan.md` (Session 1 plan)

## Summary

Sessions 0 and 1 delivered the canonical type representation, the unified runtime model, the rewritten abi/ lift/lower package, and the initial Instantiate pipeline. This session completes the implementation: cross-instance resource resolution, the post-return two-phase protocol, full multi-module Instantiate pipeline, spectest runner, and a public API matching wasmtime's C API surface.

The implementation proceeds in four layers, bottom-up: Foundation (cross-instance resources, post-return, handle+list wiring), Pipeline (multi-module Instantiate, wireExports, canon.lower/lift closures), Tests (spectest runner, unskip all tests, real .wasm integration), and Public API (type introspection, resource surface, InstancePre, post-return exposure).

## Goals

- Cross-instance resource operations work end-to-end: handles minted by one instance are correctly lifted/lowered/dropped by another, including destructor invocation via canon_lift/canon_lower.
- Post-return is a public two-phase protocol matching the spec and wasmtime C API.
- `ComponentLinker.Instantiate` handles real multi-module components: canon.lower closures invoke guest code, canon.resource operations are wired, memory and realloc are shared.
- The spectest runner implements all `.wast` command types: `component`, `assert_invalid`, `module`, `invoke`, `assert_return`, `assert_trap`, `register`.
- Every `t.Skip` in the codebase is either removed (test passes) or justified with a spec citation for async deferral. No skip is left unaddressed.
- The public `api/component/` surface matches wasmtime's C API: type introspection, resource operations, post-return, InstancePre, export access.
- Any defects found in abi/, binary/, runtime/, or types/ are fixed at the source. No workarounds, no shims, no duplicate types or paths.

## Non-Goals (explicitly deferred)

- **Async lift/lower** (stream, future, error-context, subtask). The `TypeKindStream`, `TypeKindFuture`, `TypeKindErrorContext` trap arms stay. Tests that require async semantics remain skipped with "async: no session scheduled" and a spec citation.
- **Typed function call API** (`TypedFunc[P, R]` generics). The dynamic `Val`-based API is sufficient when all spec types can be exercised through it. Code generation (wit-bindgen-go) is a separate tool.
- **WIT-binding codegen**. Not on any session.

## Spec Authorities

All citations reference files vendored in the repo. These sources win over any contrary wazero comment, doc, or test.

- `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py` — the canonical-abi reference implementation.
  - `lift_own` at lines 1333-1339, `lift_borrow` at 1341-1347.
  - `lower_own` at 1641-1643, `lower_borrow` at 1645-1651.
  - `canon_resource_new` at 2134-2138, `canon_resource_rep` at 2169-2173.
  - `canon_resource_drop` at 2142-2165 (cross-instance destructor routing).
  - `canon_lift` at 1978-2040 (post-return at 1999-2002).
  - `canon_lower` at 2064-2130.
  - `lower_flat_values` at 1943-1975, `lift_flat_values` at 1977-1993.
  - `call_might_be_recursive` at 290-299.
  - `ComponentInstance` at 256-273.
- `debug-vendored/component-model/design/mvp/CanonicalABI.md` — spec prose.
- `debug-vendored/wasmtime/crates/c-api/include/wasmtime/component/` — wasmtime C API (the language-neutral public surface reference).
  - `func.h` — `func_call`, `func_post_return`, `func_type`.
  - `linker.h` — linker, instantiation, resource definition.
  - `val.h` — value types, resource operations.
  - `types/val.h` — full type introspection.
  - `types/func.h` — function type introspection.
  - `types/resource.h` — resource type equality.
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/` — wasmtime Rust runtime.
  - `instance.rs` — Instantiator, InstancePre.
  - `func.rs` — Func::call, post_return.
  - `resources/ty.rs` — ResourceType::guest.
- `debug-vendored/wasmtime/tests/misc_testsuite/component-model/` — wasmtime WAT test files.

## Approach

Bottom-up in four layers. Each layer builds on the previous:

1. **Foundation** — Fix cross-instance resources, add post-return protocol, wire handle+list params through abi
2. **Pipeline** — Complete Instantiate for real multi-module components, fix wireExports, wire canon.lower/lift/resource closures
3. **Tests** — Spectest runner, unskip all tests, add real .wasm integration tests
4. **Public API** — Type introspection, resource surface, InstancePre, post-return, export access

If any layer reveals defects in abi/, binary/, runtime/, or types/, the defect is fixed at the source in that layer. No workarounds.

---

## Layer 1: Foundation

### 1A. Cross-Instance Resource Resolution

**Problem:** Three paths in `instance.go` return errors instead of implementing cross-instance resource operations. Cross-instance resources are a core part of the component model — any component that imports another component's resource type exercises this path.

**Spec requirement** (`definitions.py:2142-2165`):
```python
def canon_resource_drop(rt, thread, i):
    trap_if(not thread.task.inst.may_leave)
    inst = thread.task.inst
    h = inst.table.remove(i)
    trap_if(not isinstance(h, ResourceHandle))
    trap_if(h.rt is not rt)
    trap_if(h.num_lends != 0)
    if h.own:
        assert(h.borrow_scope is None)
        if inst is rt.impl:
            if rt.dtor:
                rt.dtor(h.rep)
        else:
            if rt.dtor:
                callee_opts = CanonicalOptions(async_ = False)
                callee = partial(canon_lift, callee_opts, rt.impl, ft, rt.dtor)
                [] = canon_lower(caller_opts, ft, callee, thread, [h.rep])
            else:
                trap_if(call_might_be_recursive(thread.task, rt.impl))
    else:
        h.borrow_scope.num_borrows -= 1
```

**Changes:**

1. **`Instance.ResourceDrop` cross-instance destructor** (`instance.go:344-353`): Replace the error with a real `canon_lift`/`canon_lower` invocation:
   - Construct a destructor function type: `FuncType([U32], [])`
   - Call the destructor on `rt.Impl` (the defining instance) via the abi lift/lower path
   - Set `may_leave=false` around the call per spec

2. **Guest destructor resolution** (`instance.go:376-389`): `rt.Dtor` is a core function index. Resolution requires access to the defining instance's core modules. Fix: add a `DestructorFunc func(rep uint32) error` field on `runtime.ResourceType` (a Go closure, not a core function index). The closure is set at instantiation time by the `ComponentLinker`, which has access to both the `runtime.ResourceType` and the core module exports. This avoids an import cycle between `runtime/` and `component/` — the closure captures the core function reference at bind time. The `Dtor *uint32` field is kept for the binary decoder but `DestructorFunc` is the runtime resolution path.

3. **Reentrance check** (`instance.go:350-352`): When no destructor and cross-instance, call `CallMightBeRecursive` between the calling instance and the defining instance per spec `:2162`. The defining instance is resolved from the store-wide registry (see item 4). `CallMightBeRecursive` already exists — wire it in with the resolved defining instance.

4. **Store-wide resource type registry**: `LookupResourceType` currently walks the `Parent` chain, which fails for sibling instances. Add a store-level `map[resourceTypeKey]*ResourceType` registry on `runtime.ComponentInstance` (or a shared store object). Populated at instantiation time. `LookupResourceType` consults the registry when the parent-chain walk fails. Key: `(RuntimeComponentInstanceIdx, ResourceIdx)`.

5. **Fix any defects in abi/lift.go or runtime/table.go** encountered during cross-instance testing. The `liftOwnHandle`/`liftBorrowHandle` paths must correctly resolve resource types across instances via `LookupResourceType` or the store registry.

### 1B. Post-Return Two-Phase Protocol

**Problem:** wazero's `ExportedFunc.Call` invokes post-return internally, hiding the two-phase protocol from embedders. This violates the spec and prevents embedders from reading results before cleanup.

**Spec requirement** (`definitions.py:1999-2002`):
```python
if opts.post_return is not None:
    inst.may_leave = False
    [] = call_and_trap_on_throw(opts.post_return, thread, flat_results)
    inst.may_leave = True
```

**Wasmtime C API:** `wasmtime_component_func_call()` + `wasmtime_component_func_post_return()` — mandatory two-phase.

**Changes:**

1. **Split `ExportedFunc` into two-phase protocol:**
   - `Call(ctx, params...) ([]Val, error)` — invokes the function, lifts results, but does NOT run post-return. Stores flat results on the ExportedFunc for post-return.
   - `PostReturn(ctx) error` — runs the post-return function with `may_leave=false`, cleans up borrow scope.
   - `needsPostReturn bool` flag — panic if `Call` is invoked again before `PostReturn`, matching wasmtime's enforcement.

2. **Convenience method:** `CallAndPostReturn(ctx, params...) ([]Val, error)` — does both in one shot for embedders who don't need the window.

3. **Borrow scope lifecycle:** Each `Call` creates a `runtime.CallContext` with a `BorrowScope`, passes it to `LowerContext`. `PostReturn` verifies all borrows are dropped (spec `deliver_resolve` at `:738-742`), then invokes the post-return core function.

### 1C. ExportedFunc.Call Handle + List Wiring

**Problem:** ExportedFunc.Call delegates to a HostFunc closure that doesn't wire resource handle or list parameters through `abi.LiftParams`/`abi.LowerParams`.

**Changes:**

1. **`buildCanonLiftFunc` in `component_linker.go`:** The closure must construct a `LiftContext` with the instance's memory, realloc, types bag, and call context. Use `abi.LiftParams` for arguments (handling `MAX_FLAT_PARAMS` retptr spill) and `abi.LowerResults` for return values (handling `MAX_FLAT_RESULTS` spill).

2. **Memory + realloc resolution:** Resolve the core module's `memory` and `cabi_realloc` exports at closure-construction time (post-core-module-instantiation). Thread into `LiftContext.Memory` and `LiftContext.Realloc`.

3. **Handle params:** Own/borrow params require `LowerContext` with the instance's Table to call `lowerOwnHandle`/`lowerBorrowHandle` during argument lowering. The LowerContext must be constructed per-call with a fresh BorrowScope.

4. **List params:** Lists require `LowerContext.Realloc` for heap allocation. The realloc function is resolved from the core module's `cabi_realloc` export.

5. **Fix any abi/ defects** found during wiring. If `abi.LiftParams` or `abi.LowerResults` have incorrect behavior for specific type kinds, fix them in abi/ directly.

---

## Layer 2: Pipeline

### 2A. Complete Instantiate for Real Multi-Module Components

**Problem:** ~60 tests skip because `Instantiate` can't handle real compiled components with core modules, canon.lower closures, and shared memory.

**What a real component contains:**
- One or more core modules (the actual wasm code)
- `canon.lower` entries creating core-wasm-callable thunks for imported component functions
- `canon.lift` entries wrapping core-wasm exports as component exports
- `canon.resource.new/rep/drop` entries for resource operations
- Inline core instances bundling canonical entries as synthetic host modules
- Memory sharing between core modules and the canonical ABI layer

**Changes:**

1. **`buildCanonLowerFunc(inst, c, canonLowerInfo) api.GoModuleFunction`**: New method implementing the canon.lower spec. The returned Go function:
   - Takes core wasm params (i32/i64/f32/f64)
   - Constructs a `LiftContext` with the callee instance's memory + types
   - Calls `abi.LiftParams` to lift flat core values into `[]Val`
   - Invokes the component-level function from the func index space
   - Calls `abi.LowerResults` to lower `[]Val` results back to flat core values
   - Handles `MAX_FLAT_PARAMS`/`MAX_FLAT_RESULTS` retptr spill (spec `lower_flat_values` at `definitions.py:1943-1975`)
   - Manages `may_leave` toggling per spec `:1955, :1973`

2. **`buildCanonResourceFunc(inst, c, canonResourceInfo) api.GoModuleFunction`**: New method for resource operation host functions. Bridges core wasm i32 params to `Instance.ResourceNew/Rep/Drop` with the correct `ResourceIdx`.

3. **Post-instantiation memory capture:** After each core module instantiation, scan its exports for `memory` and `cabi_realloc`. Back-patch these into canonical closures that reference that module's memory. This matches wasmtime's `Instantiator::build` flow.

4. **`buildCanonLiftFunc` enhancement:** The canon.lift closure (used by `wireExports`) resolves memory + realloc from the now-instantiated core module, not at wireExports time.

5. **`buildCoreHostModule` dispatch:** For each export in an inline core instance:
   - If `canon.lower` reference: call `buildCanonLowerFunc`
   - If `canon.resource.*` reference: call `buildCanonResourceFunc`
   - If alias to another core instance's export: forward to already-instantiated module
   - If direct core function: forward to owning core module

### 2B. wireExports Composite Type Resolution

**Problem:** 6 tests skip because `wireExports` can't resolve core function indices for record/option/result-typed exports.

**Changes:**

1. **Post-Step-12 funcSpace population:** After `instantiateCoreModules`, walk each core instance's exports and add them to `funcSpace`. Core function indices become resolvable.

2. **Memory space population:** Same pattern for `memSpace` — core module memory exports added so canon.lift closures can find the memory.

3. **Alias-aware resolution:** Core function aliases (`Alias` entries) that reference core instance exports resolved through the now-populated core instance modules.

### 2C. Defect Fixes in Underlying Packages

Any defects found in abi/, binary/, runtime/, or types/ during Layer 2 implementation are fixed at the source:

- If `abi.CoreSignature` computes wrong flattened signatures → fix in `abi/flatten.go`
- If the binary decoder doesn't emit correct `CanonicalDef` metadata → fix in `binary/`
- If `runtime.Table` has incorrect generation-tag handling → fix in `runtime/table.go`
- If `types.ComponentTypes` is missing fields needed by the pipeline → add in `types/`

No workarounds. No duplicate types or fields. Pre-production code means we fix the source.

---

## Layer 3: Tests

### 3A. Spectest Runner

**Problem:** 65 ResourcesWast skips. The spectest runner parses `.wast` but only implements `component` commands.

**Changes — implement all `.wast` command types:**

1. **`assert_invalid`**: Call `CompileComponent` on WAT, expect error. Compare error text against expected message. No instantiation needed.

2. **`module` / `module-definition`**: Call `wazero.Runtime.CompileModule` on WAT bytes, store in module registry.

3. **`invoke`**: Instantiate current component (requires Layer 2), call named function with args converted to `[]Val`, return results.

4. **`assert_return`**: `invoke` + compare results against expected values.

5. **`assert_trap`**: `invoke` + assert error contains trap message.

6. **`register`**: Store instance under name for cross-component imports.

### 3B. Unskip All Tests

**Critical rule: the specific skip counts from discovery are estimates. The implementation must find and address every `t.Skip` call in the codebase, regardless of count.** The process is:

1. `grep -rn "t.Skip" internal/component/ --include="*_test.go"` — find ALL skips
2. For each skip: determine if the underlying gap is fixed by Layers 1-2
3. If fixed: remove the skip, verify the test passes
4. If not fixed: determine why. If it's an async feature → add a spec citation and leave the skip. If it's something else → it's a bug in our Layer 1-2 implementation, fix it.
5. **Zero unaddressed skips.** Every remaining skip must have a spec citation for a genuinely deferred async feature.

Do not target a specific number. Do not assume a skip is accounted for because it appeared in the discovery. Process every single one found by the grep.

### 3C. Real .wasm Integration Tests

**Port wasmtime sync WAT tests into the spectest runner:**
- `types.wast`, `enums.wast`, `nested.wast`, `linking.wast`, `import.wast`, `modules.wast`, `aliasing.wast`, `enum_discriminant.wast`, `fixed_length_lists.wast`

**Verify all existing wasip2test .wasm files pass** once the pipeline is complete. These exercise WASI HTTP, filesystem, clocks, sockets, random end-to-end with real compiled components.

**Verify conformance coverage** against `run_tests.py` sync test categories: scalars, composites, strings, heap, flattening, roundtrips, resources, reentrance. Identify and fill any remaining gaps.

### 3D. Defect-Driven Fixes

Tests are defect discovery, not just validation. If any test reveals a defect in abi/, binary/, runtime/, or types/, fix the defect at the source. Do not modify the test to work around the defect. Do not add shims or adapters. Fix the root cause.

---

## Layer 4: Public API

### 4A. Type Introspection

**Reference:** wasmtime C API `types/val.h`, `types/func.h`.

**Add to `api/component/`:**

1. **`FuncType` introspection:**
   - `Params() []Param` — named params (name + ValType)
   - `Results() []Param` — named results
   - `NumParams() int`, `NumResults() int`

2. **`ValType` public wrapper** (read-only view of internal `types.ValType`):
   - `Kind() ValKind`
   - `ListElement() ValType`
   - `RecordFields() []Field` — name + type
   - `TupleTypes() []ValType`
   - `VariantCases() []Case` — name + optional payload
   - `EnumCases() []string`
   - `OptionSome() ValType`
   - `ResultOk() *ValType`, `ResultErr() *ValType`
   - `FlagsNames() []string`
   - `OwnResourceType() ResourceTypeID`, `BorrowResourceType() ResourceTypeID`

3. **Supporting types:**
   ```
   Param { Name string; Type ValType }
   Field { Name string; Type ValType }
   Case  { Name string; Type *ValType }  // nil = no payload
   ```

### 4B. Resource API Surface

**Reference:** wasmtime C API `val.h`, `linker.h`.

1. **`ResourceType` public type** — wraps `*runtime.ResourceType`:
   - `Equal(other ResourceType) bool` — pointer identity per spec

2. **`Instance.GetResource(name string) (ResourceType, bool)`** — resolve exported resource type.

3. **Resource handle operations on public Instance:**
   ```
   ResourceNew(rt ResourceType, rep uint32) (uint32, error)
   ResourceRep(rt ResourceType, handle uint32) (uint32, error)
   ResourceDrop(rt ResourceType, handle uint32) error
   ```

4. **`ComponentLinker.DefineResource` returns `ResourceType`** so embedders can use it for cross-instance operations.

### 4C. Post-Return on Public API

**Reference:** wasmtime C API `func.h` — `func_call` + `func_post_return`.

1. **`ComponentFunc` interface:**
   ```
   Call(ctx context.Context, params ...Val) ([]Val, error)
   PostReturn(ctx context.Context) error
   CallAndPostReturn(ctx context.Context, params ...Val) ([]Val, error)
   Type() FuncType
   ```

2. **Panic guard:** `Call` before `PostReturn` panics, matching wasmtime.

### 4D. InstancePre

**Reference:** wasmtime `instance.rs` InstancePre.

1. **`InstancePre` struct:**
   - Holds compiled component + cached import resolution + type-check results
   - Created by `ComponentLinker.InstantiatePre(compiled) (*InstancePre, error)`
   - `Instantiate(ctx) (*Instance, error)` — uses cached resolution
   - `Component() *CompiledComponent` — accessor

2. **Implementation:** Factor out import-resolution + type-checking from `ComponentLinker.Instantiate` into a shared helper. `Instantiate` = resolve + instantiate. `InstantiatePre` = resolve (cached). `InstancePre.Instantiate` = instantiate (from cache).

### 4E. Instance Export Access

**Reference:** wasmtime C API `instance.h`.

1. `Instance.ExportedFunction(name string) ComponentFunc` — already exists internally, expose publicly.
2. `Instance.ExportedFunctions() map[string]ComponentFunc` — iterate all exports.
3. `Instance.ExportedInstance(name string) *Instance` — already exists internally, expose publicly.

---

## Acceptance Criteria

1. `go test ./...` passes with zero failures. Every remaining `t.Skip` has a spec citation for an async feature deferral.
2. Real .wasm integration tests exercise the full component model pipeline with compiled components from debug-vendored/.
3. The spectest runner passes all sync commands in `resources.wast` and additional ported WAT files.
4. Cross-instance resource handles work end-to-end: create in instance A, lift/lower/drop in instance B.
5. Post-return two-phase protocol is exposed and enforced.
6. The public `api/component/` API covers: type introspection, resource operations, post-return, InstancePre, export access.
7. No workarounds, shims, duplicate types, or non-conformant code paths remain in abi/, binary/, runtime/, types/, or component/.
8. WASI P2 interfaces (HTTP, filesystem, sockets, clocks, random, CLI, io) exercised via real .wasm components.
