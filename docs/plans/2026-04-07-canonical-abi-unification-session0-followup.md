# Canonical ABI Unification — Session 0 Followup Note

This note captures intentionally-broken or intentionally-deferred work
from Session 0. Each item lists the exact file:line and the scope of
the follow-up work.

## Session 1 — Wire abi/ into production

### Broken in-place at end of Session 0 (compile-only, logically wrong):

- `internal/component/instance.go:156` — `ExportedFunc.Call` — Session 1
  must delete this panic stub and replace the body with a call to
  `abi.LiftContext` (for lifting arguments from the caller) followed by
  `abi.LowerContext` (for lowering results to the caller). The rewritten
  abi/ package already supports this; the wiring just hasn't been done.

- `internal/component/instance.go:185` — `Instance.ResourceNew` — Session 1
  must delete this panic stub and replace it with
  `runtime.Table.NewResourceHandle` using the concrete `*runtime.ResourceType`
  identity minted during Concrete promotion (Session 2 prerequisite).

- `internal/component/instance.go:193` — `Instance.ResourceRep` — Session 1
  must delete this panic stub and replace it with a `runtime.Table.Get` call
  that returns the rep stored inside the handle entry.

- `internal/component/instance.go:202` — `Instance.ResourceDrop` — Session 1
  must delete this panic stub and replace it with a `runtime.Table.Remove`
  call that also invokes the registered destructor.

- `internal/component/component_linker.go:146` — `ComponentLinker.Instantiate`
  — Session 1 must delete this panic stub and restore full instantiation:
  resolve imports via `MatchImport`, build a `runtime.ComponentInstance`,
  create per-function `ExportedFunc` wrappers, and wire canon-lift/lower.

- `internal/component/component_linker.go:177` — `coreSignature` — Session 1
  must delete this panic stub and replace it with a direct call to
  `abi.Flatten` (already implemented in `internal/component/abi/flatten.go`)
  to compute the core wasm param/result signature for a lowered function.

- `internal/component/nested_component.go:171` — `resolveExportTypeAlias`
  — Session 1 or 2 must restore real cross-component type alias resolution.
  The previous body walked `c.Types` as a `[]TypeDef` slice via
  `TypeIdxToStoredIdx`; both shapes were reworked in Tasks 2, 12, and 13.
  The new path must walk `*types.ComponentTypes` (the canonical bag produced
  by Task 13's binary decoder) to find the exported type and return the
  matching `*TypeDef`.

### Test coverage lost via wholesale stubs (Session 1 must restore):

The conformance package (`internal/component/conformance/`) was wholesale-
stubbed by Task 18. Each of the following files was reduced to a single
`TestXxxDeferredToSession1(t)` function that immediately calls
`t.Skip(session1SkipReason)`. The stub files range from 14–25 lines;
the real suites they stand in for were intended to be real multi-case
integration tests against the full lift/lower + linker path. Session 1
must replace each stub with the actual test suite.

**conformance/ wholesale stubs (each 14–25 lines, single deferred function):**

- `internal/component/conformance/abi_edge_cases_test.go` —
  `TestABIEdgeCasesDeferredToSession1`: edge cases in flat ABI encoding
  (alignment padding, >16 flat params falling back to memory, etc.)

- `internal/component/conformance/composites_test.go` —
  `TestCompositesDeferredToSession1`: record, variant, tuple, flags,
  and option round-trip tests.

- `internal/component/conformance/concurrent_access_test.go` —
  `TestConcurrentAccessDeferredToSession1`: concurrent handle table access
  from multiple goroutines.

- `internal/component/conformance/destructor_test.go` —
  `TestDestructorDeferredToSession1`: resource destructor invocation on
  drop and scope exit.

- `internal/component/conformance/error_messages_test.go` —
  `TestErrorMessagesDeferredToSession1`: error message formatting for trap
  conditions (bad UTF-8, bounds violations, type mismatches).

- `internal/component/conformance/flat_abi_test.go` —
  `TestFlatABIDeferredToSession1`: flat-ABI threshold tests (params/results
  at 16-item boundary, overflow to indirect memory).

- `internal/component/conformance/instance_types_test.go` —
  `TestInstanceTypesDeferredToSession1`: instance type import/export
  validation including width subtyping.

- `internal/component/conformance/linker_test.go` —
  `TestLinkerDeferredToSession1`: full `ComponentLinker.Instantiate` round-
  trip against a synthetic component.

- `internal/component/conformance/memory_bounds_test.go` —
  `TestMemoryBoundsDeferredToSession1`: memory bounds enforcement for heap
  lift/lower (out-of-bounds string pointers, list lengths, etc.)

- `internal/component/conformance/nested_instantiation_test.go` —
  `TestNestedInstantiationDeferredToSession1`: nested component instantiation
  with argument forwarding from parent scope.

- `internal/component/conformance/nested_test.go` —
  `TestNestedDeferredToSession1`: deeply nested component hierarchies and
  instance space alignment.

- `internal/component/conformance/nesting_depth_test.go` —
  `TestNestingDepthDeferredToSession1`: nesting depth limits and stack
  overflow guards.

- `internal/component/conformance/post_return_test.go` —
  `TestPostReturnDeferredToSession1`: post-return function invocation after
  canon lift and borrow scope cleanup.

- `internal/component/conformance/realloc_failure_test.go` —
  `TestReallocFailureDeferredToSession1`: realloc returning -1 / 0 results
  in a trap rather than silent corruption.

- `internal/component/conformance/resource_generation_test.go` —
  `TestResourceGenerationDeferredToSession1`: generation-tagged handle
  table ensuring stale handles are rejected after drop.

- `internal/component/conformance/resources_test.go` —
  `TestResourcesDeferredToSession1`: full resource lifecycle (new, rep, borrow
  lend-counting, drop, destructor).

- `internal/component/conformance/strings_test.go` —
  `TestStringsDeferredToSession1`: end-to-end string lift/lower across the
  full linker path (not just the unit-level abi/ tests).

- `internal/component/conformance/subtask_test.go` —
  `TestSubtaskDeferredToSession1`: subtask creation, completion, and error
  propagation.

- `internal/component/conformance/type_edge_cases_test.go` —
  `TestTypeEdgeCasesDeferredToSession1`: edge cases in type checking (missing
  exports, resource equality, function signature mismatch).

- `internal/component/conformance/utf_validation_test.go` —
  `TestUTFValidationDeferredToSession1`: UTF-8/16/latin1+utf16 validation
  coverage at the integration level.

- `internal/component/conformance/wasi_cli_test.go` —
  `TestWASICLIDeferredToSession1`: WASI CLI world conformance
  (originally imported `wasip2` host bindings).

- `internal/component/conformance/wasi_clocks_test.go` —
  `TestWASIClocksDeferredToSession1`: WASI clocks world conformance.

- `internal/component/conformance/wasi_error_handling_test.go` —
  `TestWASIErrorHandlingDeferredToSession1`: WASI error-handling conformance.

- `internal/component/conformance/wasi_filesystem_test.go` —
  `TestWASIFilesystemDeferredToSession1`: WASI filesystem world conformance.

- `internal/component/conformance/wasi_http_test.go` —
  `TestWASIHTTPDeferredToSession1`: WASI HTTP world conformance.

- `internal/component/conformance/wasi_poll_test.go` —
  `TestWASIPollDeferredToSession1`: WASI poll world conformance.

- `internal/component/conformance/wasi_random_test.go` —
  `TestWASIRandomDeferredToSession1`: WASI random world conformance.

- `internal/component/conformance/wasi_resource_lifecycle_test.go` —
  `TestWASIResourceLifecycleDeferredToSession1`: WASI resource lifecycle
  conformance.

- `internal/component/conformance/wasi_sockets_test.go` —
  `TestWASISocketsDeferredToSession1`: WASI sockets world conformance.

- `internal/component/conformance/wasi_streams_test.go` —
  `TestWASIStreamsDeferredToSession1`: WASI streams world conformance.

**internal/component/abi/ test files (Task 15 rewrite):**

These test files were rewritten from scratch in Task 15 with minimal new
coverage that exercises the new kind-switch dispatch arms. The original
breadth was lost and Session 1 must restore it via the builder API:

- `internal/component/abi/lift_test.go` — was 2925 lines; now minimal
  coverage. Original tests covered: scalar round-trips, record/variant/
  list/tuple/option/result/flags/enum lift dispatch, surrogate pair
  handling in chars, fixed-length lists, variant join semantics at all
  widths, error path coverage.
- `internal/component/abi/lower_test.go` — was 1856 lines; now minimal
  coverage. Original tests covered: scalar lower round-trips, all
  composite lower dispatch, the same-instance borrow optimization,
  cross-instance borrow paths, error/trap coverage for invalid handles.
- `internal/component/abi/flatten_test.go` — was 349 lines; now minimal
  coverage. Original tests covered: per-kind flattening to core
  (api.ValueType) signatures, MAX_FLAT_PARAMS overflow, MAX_FLAT_RESULTS
  overflow, fixed-length list flattening.

Session 1 must rewrite these test cases against the new builder-based
API, exercising the kind-switch dispatch arms with the same coverage
breadth.

**abi/ unit tests with partial skips (wazerotest.NewMemory harness issue):**

- `internal/component/abi/context_test.go` — 7 tests skipped:
  `TestLiftContextReadU8BoundsCheck`, `TestLiftContextReadU16BoundsCheck`,
  `TestLiftContextReadU32BoundsCheck`, `TestLiftContextReadU64BoundsCheck`,
  `TestLiftContextReadF32BoundsCheck`, `TestLiftContextReadF64BoundsCheck`,
  `TestLiftContextReadBytesBoundsCheck`. The issue: `wazerotest.NewMemory`
  rounds to page size (64KiB), making it impossible to construct a memory
  smaller than the pointer being accessed. The bounds-check harness needs a
  direct `[]byte`-backed memory stub. Additionally 2 context-shape tests
  are skipped: `TestLowerContext_WithSubtask` and
  `TestLowerContext_BorrowScope_NilSubtask`.

- `internal/component/abi/strings_test.go` — 4 tests skipped:
  `TestLiftStringUTF8_BoundsCheck`, `TestLiftStringUTF16_BoundsCheck`,
  `TestLiftStringLatin1UTF16_Latin1BoundsCheck`, and
  `TestLowerStringUTF8` (same wazerotest page-size issue).

### Skipped tests (`t.Skip("session 1 work: ...")`):

The following files in `internal/component/` contain tests gated behind
`t.Skip` with a reference to this followup note. All must pass (without
the skip) before Session 1 can be declared done.

**Top-level `internal/component/` package:**

- `internal/component/component_test.go` — 1 test skipped
  (`TestNewTypeDefs`, which exercises the new `*types.ComponentTypes` bag
  exposed by the binary decoder after Task 13).

- `internal/component/edge_case_test.go` — 1 test skipped
  (`TestTypeIndexOutOfRange`, which requires a working `Instantiate` path).

- `internal/component/integration_test.go` — 1 test skipped
  (the entire file's `TestIntegration_*` suite is reduced to a single skip;
  14 integration tests covering linker/instantiate end-to-end).

- `internal/component/linker_api_test.go` — 1 test skipped
  (the entire linker API file is reduced to a single skip; 20 tests
  covering `ComponentLinker.Instantiate`, `MatchImport`, `DefineFunc`, etc.)

- `internal/component/linker_test.go` — 1 test skipped
  (the entire `Linker` test file is reduced to a single skip; 19 tests
  covering `Linker.DefineFunc`, `MatchImport`, semver matching, etc.)

- `internal/component/nested_component_test.go` — 1 test skipped
  (the entire nested-component test file is reduced to a single skip; 16
  tests covering `instantiateNestedComponent`, `resolveFromParentScope`,
  `buildTypeSpace`, and export aliasing).

- `internal/component/start_function_test.go` — 1 test skipped
  (the entire start-function test file is reduced to a single skip; 9 tests
  covering `ExecuteStartFunction` edge cases).

- `internal/component/type_checker_test.go` — 1 test skipped
  (the entire type-checker test file is reduced to a single skip; 13 tests
  covering `CheckDefinition`, `checkFuncType`, `checkInstance`, and
  `checkResource`).

- `internal/component/value_import_test.go` — 1 test skipped
  (`TestValueImport`, which exercises value imports through the linker).

**`internal/component/binary/` package:**

- `internal/component/binary/component_type_test.go` — 8 tests skipped:
  `TestDecodeComponentType`, `TestDecodeComponentTypeEmpty`,
  `TestDecodeComponentTypeWithExport`, `TestDecodeComponentTypeWithAlias`,
  `TestDecodeComponentTypeWithCoreType`, `TestDecodeComponentTypeWithNestedType`,
  `TestDecodeComponentTypeMultipleDeclarations`,
  `TestDecodeComponentTypeImportInstance`. These validate that the binary
  decoder produces correct `*types.ComponentTypes` bags.

- `internal/component/binary/instance_type_test.go` — 10 tests skipped:
  `TestDecodeInstanceType`, `TestDecodeInstanceTypeWithAlias`,
  `TestDecodeInstanceTypeEmpty`, `TestDecodeInstanceTypeMultipleExports`,
  `TestDecodeInstanceTypeWithCoreType`, `TestDecodeInstanceTypeWithNestedType`,
  `TestDecodeInstanceTypeCoreModuleExport`, `TestDecodeInstanceTypeInstanceExport`,
  `TestDecodeInstanceTypeComponentExport`, `TestDecodeInstanceTypeValueExport`.

**`internal/component/abi/` package:**

- `internal/component/abi/context_test.go` — 9 tests skipped total (7
  bounds-check + 2 context-shape; see details in previous section).

- `internal/component/abi/strings_test.go` — 4 tests skipped (see details
  in previous section).

### Session 1 acceptance criteria:

- Zero references to `instance.go`'s old lift/lower bodies (all 4 panic
  stubs gone, replaced with `abi.LiftFlat`/`LowerFlat` calls through the
  rewritten `ExportedFunc.Call` and resource-op methods).
- `canon_lower.go` stays deleted (was removed in Task 16; do not resurrect).
- `ComponentLinker.Instantiate` and `coreSignature` panic stubs gone,
  replaced with real instantiation wiring and `abi.Flatten` routing.
- `resolveExportTypeAlias` panic stub gone, replaced with a walk over
  `*types.ComponentTypes` (the canonical bag).
- All 40 tests currently marked `t.Skip("session 1 work")` pass without the skip.
- All 30 conformance stubs (`TestXxxDeferredToSession1`) replaced with real
  multi-case tests.
- All 11 abi/ bounds-check and context-shape tests pass (requires a
  `[]byte`-backed memory stub that does not round to page size).
- `type_checker.go`'s `checkFuncType` subtyping walk restored to proper
  structural subtyping over `*types.ComponentTypes` entries (currently
  uses identity-only index comparison on `TypeFunc.Params`/`TypeFunc.Results`).
- `type_checker.go`'s `checkFuncDefinition` and `checkInstanceDefinition`
  restored to compare expected type against the actual definition
  (currently ignores the expected side entirely).
- `nested_component.go::resolveExportTypeAlias` restored to real
  cross-component type alias resolution (currently panics).

---

## Session 2 — Resource Concrete promotion + cross-component type checking

### Deferred from Session 0:

- **Linker plumbing: `TypeResourceTable.Concrete = false` → `true` at
  instantiation time.** For each resource declaration in the component, the
  linker mints a fresh `*runtime.ResourceType` and stores it in
  `runtime.ComponentInstance.ResourceTypes[ResourceIdx]`. Location:
  `internal/component/component_linker.go` instantiation path (the
  `ComponentLinker.Instantiate` stub scheduled for Session 1 deletion
  must include this promotion step once Session 2 proceeds).

- **`typeChecker` struct and full structural subtyping walk for cross-
  component import type-matching.** A new or expanded file
  `internal/component/types/typecheck.go` should implement the proper
  walk over `*types.ComponentTypes` entries. The current
  `internal/component/type_checker.go` performs identity-only index
  comparison which is correct only when both sides share the same
  `ComponentTypes` bag (i.e., within a single component). Cross-component
  matching (where the host's type bag differs from the component's) requires
  structural comparison.

- **Cross-instance resource type resolution.** When lift/lower encounters
  an `own<T>` or `borrow<T>` of a resource defined by a different instance,
  the runtime must walk from `LiftContext.Instance` to find the defining
  instance and fetch its `*runtime.ResourceType`. Session 0's
  `LookupResourceType` in `runtime.ComponentInstance` handles only the
  single-instance case (it looks up by `(instanceIdx, resourceIdx)` in the
  local `ResourceTypes` map). Session 2 must thread the multi-instance
  registry through `LiftContext` / `LowerContext`.

### Latent correctness gaps unblocked when Concrete promotion lands:

These bugs exist today in `internal/component/abi/lift.go` but are
unreachable in Session 0 because `TypeResourceTable.Concrete` is always
`false` (no concrete `*runtime.ResourceType` is ever minted). Once Session 2
wires Concrete promotion, these will become live and must be fixed before
the resource tests can pass:

1. **`liftOwnHandle` does not validate `entry.Own == true` before
   `Table.Remove` (lift.go:665).**  
   The spec (`lift_own`, definitions.py:1336-1347) requires that the handle
   at `handleIdx` is an own-handle, not a borrow-handle. The current
   implementation calls `ctx.Instance.Table.Remove(h)` after only validating
   the resource type via `ValidateType`. A borrow handle whose resource type
   happens to match `expectedRT` would be silently removed, violating the
   spec. Session 2 must add an explicit `entry.Own == true` guard before the
   `Remove` call, or rely on a `runtime.Table` method that enforces this
   invariant.

2. **`runtime.Handle(handleIdx)` hard-codes generation=0 (lift.go:660 and
   lift.go:696).**  
   `wazero`'s `runtime.Table` uses 64-bit generation-tagged handles
   (the high 32 bits are the generation counter, the low 32 bits are the
   index). The Component Model passes a raw `u32` flat value from Wasm
   memory as the handle index, but `runtime.Handle(handleIdx)` constructs a
   handle with generation=0. This means a handle that has been recycled
   (generation > 0) will not be found. Session 2 must either add a
   `Table.GetByIndex(idx uint32) (Handle, Entry, bool)` method that returns
   the current generation-tagged handle for a given index, or store the full
   64-bit generation-tagged handle in the flat value (requiring a wider Wasm
   type, which is non-trivial).

3. **`types.ValOwn(handleIdx)` and `types.ValBorrow(handleIdx)` return the
   table index, not the resource rep (lift.go:668 and lift.go:708).**  
   The return values of `liftOwnHandle` / `liftBorrowHandle` store the raw
   Wasm-side handle index in the `Val`. Callers that need the actual resource
   representation (`entry.Rep`) must call `Table.Get` again with the index.
   This creates a semantic divergence from the spec, where the lifted value
   is expected to carry the rep. Session 2 must decide: either commit to the
   convention that "handle index is the wazero rep" (document it clearly and
   update all callers), or change `liftOwnHandle` / `liftBorrowHandle` to
   return `types.ValOwn(entry.Rep)` after the `Remove` / `IncrementLends`
   call, using a `Table.GetByIndex` lookup to obtain both `entry` and the
   generation-tagged handle.

### Session 2 acceptance criteria:

- Instantiating two copies of the same component produces two distinct
  `*runtime.ResourceType` pointers. A handle minted by one instance traps
  if presented to the other instance's function expecting the same-typed
  resource (`runtime.Table.ValidateType` rejects the foreign type pointer).
- Cross-component type-import matching works for at least one realistic WIT
  world (e.g., `wasi:clocks/wall-clock` imported by two sibling components).
- The 3 latent correctness gaps in `liftOwnHandle`/`liftBorrowHandle` are
  addressed: own/borrow guard, generation-tag bridging, and rep vs. index
  semantics.
- `runtime.ComponentInstance.ResourceTypes` is populated at instantiation
  time for every resource declaration in the component binary.

---

## Later — Async lift/lower (no session scheduled)

### Stub-and-trap sites that need real implementations:

- `internal/component/abi/lift.go:306` and `lift.go:626` — `case
  types.TypeKindStream, types.TypeKindFuture, types.TypeKindErrorContext`
  trap arms. Currently return:
  `"component-model async types not yet supported: kind=N"`.
  Both the flat-path dispatch (`LiftFlat`) and the heap-path dispatch
  (`LiftHeap`) have this trap arm.

- `internal/component/abi/lower.go:282` and `lower.go:597` — same pattern
  for `LowerFlat` and `LowerHeap`.

- `internal/component/types/composite.go:102-119` — `TypeStream`,
  `TypeFuture`, and `TypeErrorContextTable` are empty placeholder structs.
  They need per-instance table identity layering analogous to
  `TypeResourceTable` (with a `Concrete bool` field, an `Instance` index,
  and a `Stream`/`Future`/`ErrorContext` declaration index within that
  instance) so that lift/lower can validate ownership and mint typed handles.

- `internal/component/types/val.go` — constructors for stream, future, and
  error-context values (`ValStream`, `ValFuture`, `ValErrorContext`) need to
  be added alongside the existing `ValOwn` / `ValBorrow` constructors. These
  should follow the same pattern: store the handle index and set the
  appropriate `ValKind`.
