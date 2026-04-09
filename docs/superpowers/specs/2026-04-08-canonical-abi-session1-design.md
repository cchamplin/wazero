# Canonical-ABI Session 1 — Design

**Date:** 2026-04-08
**Status:** Design approved, ready for implementation planning
**Scope:** Session 1 only (wire `abi/` into production, rebuild `Instantiate`, local-only Concrete promotion, restore 223 tests + 29 conformance stubs). Session 2+ documented as followups.
**Previous session:** `docs/superpowers/specs/2026-04-07-canonical-abi-type-unification-design.md` (Session 0) + `docs/plans/2026-04-07-canonical-abi-unification-session0-followup.md` (Session 0 followup note).

## Summary

Session 0 delivered the new type representation (`types.ValType` + `types.ComponentTypes` + builder), the unified `runtime.Table`, the single-layer `runtime.ComponentInstance`, the pointer-identity `*runtime.ResourceType`, and a rewritten `abi/` lift/lower package that consumes the canonical bag. It left seven panic stubs at known file:line locations, 223 skipped tests behind `t.Skip("session 1 work")`, and 29 conformance test files reduced to single-deferred shells.

Session 1 wires `abi/` into the production call path, rebuilds the full `ComponentLinker.Instantiate` pipeline (including nested components, inline host instances, canon.lift/lower/resource host module exports, start function execution, and memory sharing) against the new types, binds component-local resource declarations to runtime `*ResourceType` identities at instantiation time (the wasmtime-parallel `Instantiator::resource` pattern), fixes four latent correctness gaps in `abi/lift.go` (missing `h.own` check, missing `h.num_lends` check, generation-tag bridging via a new `Table.GetByIndex`, and `ResourceHandleEntry.Rep` type change from `any` to `uint32` per spec), exposes the decoder's per-type-section-slot mapping on `Component.TypeDefs`, changes `ComponentLinker.DefineFunc` to require a typed `*types.TypeFunc` (matching wasmtime's strict host-function registration), and restores all 223 skipped tests + 29 conformance stubs from pre-Session-0 git history with per-test validation against `definitions.py` / `run_tests.py` / wasmtime test counterparts. Three already-passing conformance files (primitives, may_leave, reentrance) get audit tasks to add missing upstream citations.

Session 2 (cross-instance resource resolution + cross-component `typeChecker` structural walk) and Later (async lift/lower for stream/future/error-context/subtask) are explicitly deferred.

## Goals

- Zero panic stubs in `internal/component/instance.go`, `internal/component/component_linker.go`, `internal/component/nested_component.go`. Every method body either does real work or traps with a precise error pointing at Session 2 / Later.
- `ComponentLinker.Instantiate` rebuilt end-to-end against `*types.ComponentTypes` + `*runtime.ComponentInstance` + `abi.LiftContext`/`LowerContext` + `abi.FlattenParams`/`FlattenResults`/`CoreSignature`. No direct dependency on the deleted `resolveToValType` / `typeDefToValType` / `valTypeRefToValType` / `TypeResolver` helpers.
- `component.Instance` embeds `*runtime.ComponentInstance`; all spec-level runtime state (Table, MayLeave, EnterCount, ResourceTypes, Destructors, Reentrance, Parent) delegates into the embedded runtime struct. Every duplicate field on `component.Instance` is deleted.
- `Instance.ResourceNew` / `ResourceRep` / `ResourceDrop` signatures match the spec's `canon_resource_new(rt, thread, rep)` / `canon_resource_rep(rt, thread, i)` / `canon_resource_drop(rt, thread, i)` at `definitions.py:2134, 2142, 2169`. Pointer-identity `*runtime.ResourceType` enforces the spec's `h.rt is t.rt` check at `definitions.py:1345, 2147, 2172`.
- Local-only Concrete promotion lands: every resource declaration in the component being instantiated mints a fresh `*runtime.ResourceType` stored in `rt.ResourceTypes[ResourceIdx]`. Same-instance own/borrow + resource.new/rep/drop works end-to-end. Cross-instance resource handles trap with a precise "session 2 wiring" error.
- The four latent correctness gaps in `abi/lift.go::liftOwnHandle`/`liftBorrowHandle` are fixed per the canonical-abi reference: `trap_if(not h.own)` added (`definitions.py:1338`), `trap_if(h.num_lends != 0)` added (`definitions.py:1337`), `Table.GetByIndex(idx uint32)` added for Wasm-side-32-bit → runtime-64-bit generation bridging, and `ResourceHandleEntry.Rep` changed from `any` to `uint32` so `liftOwnHandle`/`liftBorrowHandle` return `entry.Rep` directly per spec `definitions.py:1339, 1347`.
- `Component.TypeDefs []TypeDef` is populated by the binary decoder at each type-section slot. Every caller using `CanonicalDef.TypeIdx` / `ImportExternDesc.TypeIdx` / `InstanceExport.TypeIdx` / `Export.TypeIdx` resolves via `c.TypeDefs[idx]`. The currently-private `decodeContext.funcTypeIdx` + `decodeContext.resourceDefs` maps are deleted — `Component.TypeDefs` is the single source of truth.
- `type_checker.go` `checkFuncDefinition` and `checkInstanceDefinition` compare expected vs actual via same-bag identity (works for all within-single-component matching, which is the only case the host-supplied definitions Session 1 handles). Cross-bag structural walk remains Session 2.
- All 223 tests currently marked `t.Skip("session 1 work")` have real bodies and pass without the skip.
- All 29 conformance stubs (`TestXxxDeferredToSession1`) are replaced with real multi-case test suites.
- All 11 `abi/context_test.go` + `abi/strings_test.go` bounds-check / context-shape tests pass, via a new direct `[]byte`-backed memory stub that does not round up to page size.
- Every restored test cites a matching `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py` behavior or wasmtime test counterpart. Tests asserting behavior that contradicts the spec/canonical/wasmtime reference are reworked or deleted.
- Per-task spec-compliance + code-quality review discipline is carried from Session 0, with an amended spec-review checklist: every restored test task must record upstream comparison citations in its review.

## Non-Goals (explicitly deferred)

- **Cross-instance resource type resolution** (Session 2). When lift/lower encounters an own/borrow of a resource defined by a different instance than `ctx.Instance`, the current `LookupResourceType` walks only the `Parent` chain. Session 2 extends this via a store-wide or linker-maintained cross-instance registry. Session 1 traps with: `"no resource type for instance N declaration M (cross-instance resolution: session 2 wiring)"`.
- **`typeChecker` cross-component structural walk** (Session 2). `checkFuncType` identity-on-index comparison is correct only when both sides share the same `*types.ComponentTypes` bag. Cross-component import matching (where the host's expected type bag differs from the component's) requires a structural walk over `ComponentTypes` entries. Session 1 handles same-bag only.
- **Async lift/lower** (Later — no session scheduled). The `TypeKindStream`, `TypeKindFuture`, `TypeKindErrorContext` trap arms in `abi/lift.go:306, 626` and `abi/lower.go:282, 597` stay as trap arms. `conformance/subtask_test.go` stays deferred-to-Later.
- **WIT-binding codegen typed path** (no session). Dynamic `Val`-based API only.
- **Public `api/component` surface redesign** beyond what the Session 0 renames forced.
- **Refactoring `types/`, `runtime/`, `abi/`, or `binary/` internals** beyond what Session 1 explicitly needs. The packages are frozen as of end-of-Session-0. Session 1 consumes them.
- **Re-litigating the 9 Session 0 Design Decisions.** They are final.

## Spec Authorities

All citations reference files vendored in the repo. These sources win over any contrary wazero comment, doc, or test.

- `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py` — the canonical-abi reference implementation.
  - `class ComponentInstance` at lines 256–273 (spec instance shape).
  - `class Table` at lines 303–315 (unified handle table).
  - `class ResourceType` at lines 351–361 (nominal identity).
  - `lift_own` at lines 1333–1339 (including `trap_if(not h.own)` at 1338 and `return h.rep` at 1339).
  - `lift_borrow` at lines 1341–1347.
  - `canon_lift` at lines 1978–2040 (full exported call flow).
  - `canon_lower` at lines 2064–2130.
  - `canon_resource_new` at lines 2134–2138.
  - `canon_resource_drop` at lines 2142–2165.
  - `canon_resource_rep` at lines 2169–2173.
  - `may_leave` toggling during post-return at 2000–2002.
  - `deliver_resolve` / borrow scope cleanup at 738–742.
- `debug-vendored/component-model/design/mvp/CanonicalABI.md` — spec prose.
  - Resource identity at 531–549.
  - Same-instance borrow optimization at 2677–2683.
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/func.rs` — wasmtime's `Func::call` / `call_impl` / `call_raw` top-level exported call flow (lines 232–706, post-return at 737–837).
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/func/options.rs` — `LiftContext` / `LowerContext` analog.
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/instance.rs` — `Instantiator` at line 710 + `Instantiator::new` at 743. `extract_resource` at 920–930 (resource type minting).
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/vm/component.rs` — `ComponentInstance` at 93–159 (the aggregating-map approach wazero is NOT adopting — cited for cross-reference only; Session 0 committed to the spec's single-layer model per Decision 6).
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/vm/component/resources.rs` — `resource_lift_own` at 275–279, `resource_lift_borrow` at 291–297, `resource_new` at 218–221, `enter_call` / `exit_call` borrow scope at 324–346.
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/resources/ty.rs` — `ResourceType::guest` at 68–79.
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/vm/component/concurrent_disabled.rs:159` — `may_enter` check.
- `debug-vendored/wasmtime/tests/all/component_model/` — wasmtime's component-model integration tests; consulted during test restoration to validate behavior assertions.

## Background: Current State at HEAD `c5d023d6`

### Panic stubs from Session 0

Confirmed against current code (line numbers as of HEAD `c5d023d6`):

1. `internal/component/instance.go:156` — `ExportedFunc.Call` panics.
2. `internal/component/instance.go:185` — `Instance.ResourceNew(rep any) (uint32, error)` panics.
3. `internal/component/instance.go:193` — `Instance.ResourceRep(handleIdx uint32) (any, error)` panics.
4. `internal/component/instance.go:202` — `Instance.ResourceDrop(handleIdx uint32, resourceTypeIdx uint32) error` panics.
5. `internal/component/component_linker.go:146` — `ComponentLinker.Instantiate(ctx, compiled) (*Instance, error)` panics.
6. `internal/component/component_linker.go:177` — `coreSignature(paramTypes, resultTypes) (params, results, needsRetptr)` panics (stub; real implementation lives in `abi.CoreSignature`).
7. `internal/component/nested_component.go:167` — `resolveExportTypeAlias(parent, c, alias) *TypeDef` panics.

### Spec-divergent field shapes on `component.Instance`

`component.Instance` currently carries duplicated runtime state that Session 0's `runtime.ComponentInstance` also owns (with spec-correct semantics):

- `table *runtime.Table` — duplicates `runtime.ComponentInstance.Table`.
- `destructors map[uint32]func(any)` — duplicates `runtime.ComponentInstance.Destructors` (which is a `*DestructorRegistry`, spec-shaped).
- `mayLeaveDisabled bool` — inverse of `runtime.ComponentInstance.MayLeave` (spec field at `definitions.py:260`).
- `activeCallDepth int32` — duplicates `runtime.ComponentInstance.enterCount`.
- `callContext *runtime.CallContext` — standalone; per-call state, not per-instance; its lifetime ownership needs a clearer home.
- `parent *Instance` + `children []*Instance` — `runtime.ComponentInstance` has its own `Parent *ComponentInstance` via spec field at `definitions.py:258`. The wrapper layer's `parent`/`children` track the `component.Instance` wrapper back-pointers; they need to be kept in sync with `rt.Parent` or collapsed into a single back-pointer with wrapper lookup.

### Session 0 followup note — contradictions flagged during discovery

Two internal contradictions in the followup note surfaced during Session 1 discovery. Session 1 resolves both:

1. **Resource Concrete promotion.** The followup note places `Instance.ResourceNew` in Session 1 scope while flagging that it depends on the concrete `*runtime.ResourceType` identity "minted during Concrete promotion (Session 2 prerequisite)." This is self-contradictory: Session 1 cannot wire `ResourceNew` to the spec without minting at least a local `*ResourceType`. **Resolution:** Session 1 pulls local-only Concrete promotion (Decision 2 below). Cross-instance resolution remains Session 2.

2. **`type_checker.go` structural subtyping.** The followup note's Session 1 acceptance criteria say *"`type_checker.go`'s `checkFuncType` subtyping walk restored to proper structural subtyping over `*types.ComponentTypes` entries."* The same note's Session 2 scope says *"`typeChecker` struct and full structural subtyping walk for cross-component import type-matching."* These are the same work described twice. **Resolution:** Session 1 restores `checkFuncDefinition` / `checkInstanceDefinition` to compare expected vs actual via same-bag identity (the within-single-component case). Session 2 adds the cross-bag structural walk. Decision 6 below.

### Decoder→linker indirection gap

`decodeContext.funcTypeIdx map[uint32]types.FuncTypeIdx` and `decodeContext.resourceDefs map[uint32]*ResourceTypeDef` are populated during decoding but are private to the decoder. Post-decode, there is no path from `CanonicalDef.TypeIdx` / `ImportExternDesc.TypeIdx` / `Export.TypeIdx` to the canonical-bag entry for that type-section slot. Every caller that previously used `c.Types[canon.TypeIdx]` (the old `[]TypeDef` slice) is now broken. The binary decoder tests flag this at `internal/component/binary/component_type_test.go` and `instance_type_test.go`. **Resolution:** Decision 5 below — add `Component.TypeDefs []TypeDef` populated by the decoder at each type-section slot; delete the private decoder maps.

### Four latent correctness gaps in `abi/lift.go`

Verified against HEAD `c5d023d6` lift.go:

1. **`liftOwnHandle` does not validate `entry.Own == true`** (lift.go:665). Spec `definitions.py:1338` asserts `trap_if(not h.own)` — a borrow handle whose `rt` happens to match the expected type would currently be silently removed, triggering a spec-incorrect ownership transfer.

2. **`liftOwnHandle` does not validate `entry.NumLends == 0`** (lift.go:665). Spec `definitions.py:1337` asserts `trap_if(h.num_lends != 0)` — an own handle with outstanding borrows cannot be lifted (the borrow scope still needs the handle). The current body calls `ValidateType` and then `Remove` with no lends check.

3. **`runtime.Handle(handleIdx)` hard-codes generation=0** (lift.go:660 and lift.go:696). Wazero's `runtime.Table` uses 64-bit generation-tagged handles (upper 32 bits = generation counter, lower 32 bits = slot index). The component-model flat ABI passes a raw 32-bit `u32` handle index from Wasm memory. `runtime.Handle(handleIdx)` constructs a handle with generation = 0, so any table slot with a non-zero current generation (i.e., any slot that has ever been recycled) will not be found. **Fix:** add `Table.GetByIndex(idx uint32) (Handle, TableEntry, error)` that looks up by slot index and returns the current generation-tagged handle plus the entry. `liftOwnHandle` / `liftBorrowHandle` + every other abi-side Wasm-handle-to-runtime-handle crossing call this instead of constructing a Handle from the raw index.

4. **`ValOwn(handleIdx)` semantics — the rep must be `uint32` per spec `definitions.py:337-349` and `:1339`** (lift.go:668 and lift.go:708). Wazero's `ResourceHandleEntry.Rep` is currently typed `any`, which does not match the spec's unambiguous `ResourceHandle.rep: int` definition (always a u32 in spec context, matching the flat ABI i32 handle encoding). Wasmtime also uses `rep: u32` throughout (`instance.rs:383-387` `resource_new32`, `vm/component/resources.rs` and `host_tables.rs`). **Fix:** change `runtime.ResourceHandleEntry.Rep` from `any` to `uint32`. Host modules (`imports/wasip2/io/streams.go` etc.) that currently store Go pointers in `Rep` refactor to maintain host state in a per-module registry (a slice of `*InputStream` indexed by the u32 id that the module hands to `NewResourceHandle`). `liftOwnHandle` returns `types.ValOwn(entry.Rep)` after validation + remove; `liftBorrowHandle` returns `types.ValBorrow(entry.Rep)` after validation + increment-lends.

### Test state from Session 0

- **223 skipped tests** in `internal/component/` — each file contains `t.Skip(session1SkipReason)` calls with no bodies. The original pre-Session-0 bodies are available in git at commit `98b3bbc3` (the rename commit before the compile-fix stub commit `36a29b13`).
- **29 conformance files wholesale-stubbed** in `internal/component/conformance/` — each reduced to a single `TestXxxDeferredToSession1(t)` function. Original multi-case bodies are in git history.
- **11 abi/ bounds-check + context-shape tests skipped** in `internal/component/abi/context_test.go` + `strings_test.go`. The skip reason is the `wazerotest.NewMemory` harness rounding up to page size (64 KiB), which makes it impossible to construct a memory smaller than the pointer being bounds-checked.
- **Decoder tests skipped** in `internal/component/binary/component_type_test.go` (8 tests) and `instance_type_test.go` (10 tests). These validate the decoder's `*types.ComponentTypes` production.

## Design Decisions

### Decision 1: Full rebuild of `Instantiate` against new types, no salvage

The pre-Session-0 `component_linker.go` (3810 lines) is a reference, not a source to copy from. Session 1 rebuilds `Instantiate` and its helpers (`buildComponentFuncs`, `instantiateCoreModule`, `wireExportedFunc`, `wireNestedComponentExports`, `createInlineInstanceModule`, `createCanonLowerExport`, `createResourceOpExport`, `createAliasExport`, `wireMemorySharing`, `resolveMemorySource`, `executeStartFunction`, `resolveExportTypeAlias`, `buildTypeSpace`) against the new types:

- Per-type dispatch goes through `abi.LiftFlat` / `abi.LiftHeap` / `abi.LowerFlat` / `abi.LowerHeap` for individual values, and through **`abi.LowerParams` / `abi.LiftParams` / `abi.LowerResults` / `abi.LiftResults` for aggregate-boundary-aware whole-signature handling** (spec `lower_flat_values` at `definitions.py:1943-1975` and `lift_flat_values` at `:1977-1993`). The aggregate entry points own the `MAX_FLAT_PARAMS` / `MAX_FLAT_RESULTS` (16 / 1) decision and the `may_leave` toggle; the per-value entry points are only used by dispatch internals (e.g., `LowerParams` calls `LowerFlat` per arg, but the canon.lift/lower closures never call `LowerFlat` directly for top-level args).
- Core signature computation goes through `abi.CoreSignature`, `abi.FlattenParams`, `abi.FlattenResults`. The `component_linker.go:177 coreSignature` panic stub is deleted and its call sites route directly to `abi.CoreSignature`.
- Type resolution goes through `c.TypeDefs[canon.TypeIdx]` (Decision 5). No re-added `resolveToValType` / `typeDefToValType` / `valTypeRefToValType` / `TypeResolver` helpers.
- `*types.ComponentTypes` is threaded through `LiftContext.Types` / `LowerContext.Types` via the per-call construction site. `*runtime.ComponentInstance` is threaded via `LiftContext.Instance` / `LowerContext.Instance`.
- Host module creation for `canon.lower` uses a closure that constructs a `LiftContext` + `LowerContext` per call, calls `abi.LiftParams` to lift arguments (handling the MAX_FLAT_PARAMS retptr-or-flat decision), invokes the component function, then calls `abi.LowerResults` to lower results (handling the MAX_FLAT_RESULTS retptr-or-flat decision plus the `may_leave` toggle around realloc). Closes the borrow scope at the end via `deliver_resolve`-equivalent. This replaces the old `createCanonLowerFunc` body.
- Host module creation for `canon.resource.new` / `canon.resource.drop` / `canon.resource.rep` binds the resource's `*runtime.ResourceType` at host-module-creation time (from `c.TypeDefs[canon.TypeIdx].Resource` + the instance's ResourceTypes pool). Calls `inst.rt.Table.NewResourceHandle(rep, true, rt)` / `Table.RemoveResourceHandle` / per-entry `RT`-pointer-identity check.
- The canon.lift closure mirrors the canon.lower closure in structure and adds: (a) `trap_if(call_might_be_recursive(caller, inst))` at entry per spec `:1979` using the `runtime.ReentranceTracker` (Decision 3 amendment); (b) `inst.rt.MayLeave = false` around `abi.LowerParams` per spec `:1955` and `inst.rt.MayLeave = true` after per `:1973`; (c) `may_leave = false` around post-return invocation per spec `:2000-2002`; (d) `BorrowScope.Close()` at the very end to cleanup lends per spec `:738-742` `deliver_resolve`.

### Decision 2: Local-only Concrete promotion lands in Session 1

Spec authority: `definitions.py:351–361` (ResourceType class) + `:1345, 2147, 2172` (`is` checks). Wasmtime parallel: `runtime/component/instance.rs:920–930` (`extract_resource`) which calls `ResourceType::guest(store_id, instance, resource.index)` at instantiation time.

At `ComponentLinker.Instantiate` entry, before any host module creation or core module instantiation, the linker walks `compiled.Internal().Types.ResourceTables` and mints one fresh `*runtime.ResourceType` per concrete resource declaration owned by this component. Each fresh `*ResourceType` is stored at `inst.rt.ResourceTypes[ResourceIdx]` where `ResourceIdx` is the decoder-assigned declaration index on `TypeResourceTable.Resource`.

Promotion promotes Abstract → Concrete: the `TypeResourceTable.Concrete` field flips true, `TypeResourceTable.Instance` is set to the runtime instance id (the instance being constructed). **Wrinkle:** the `*types.ComponentTypes` bag is shared across multiple instantiations of the same component. Session 1 cannot mutate the shared bag in place — it would break the second instantiation. **Resolution:** Session 1 introduces a per-instance clone of the resource table slice:

```go
// inside rt construction at Instantiate time
rt.LocalResourceTables = append([]types.TypeResourceTable(nil), compiled.Internal().Types.ResourceTables...)
for rtIdx, ttable := range rt.LocalResourceTables {
    if ttable.Concrete {
        // already concrete (cross-component owner) — leave as-is
        continue
    }
    // mint a *ResourceType for this declaration
    dtor, dtorAsync, dtorCallback := resolveResourceDestructor(inst, c, rtIdx)
    rt.ResourceTypes = append(rt.ResourceTypes, &runtime.ResourceType{
        Impl:         rt,
        Dtor:         dtor,
        DtorAsync:    dtorAsync,
        DtorCallback: dtorCallback,
    })
    // promote the local table entry to Concrete, pointing at the freshly-allocated *ResourceType's
    // slot in rt.ResourceTypes (index = len(ResourceTypes)-1 before the append).
    rt.LocalResourceTables[rtIdx] = types.TypeResourceTable{
        Concrete: true,
        Resource: types.ResourceIdx(len(rt.ResourceTypes) - 1),
        Instance: types.RuntimeComponentInstanceIdx(rt.ID),
    }
}
```

And the `LiftContext` / `LowerContext` constructed by this instance's calls carries `Types: &componentTypesView{base: compiled.Types, resourceTables: rt.LocalResourceTables}` — a tiny view struct that overrides `ResourceTables` lookup to consult the per-instance promoted slice while every other `*ComponentTypes` field reads from the shared bag.

Alternatively — and this is the simpler path, adopted in this design — the lift/lower dispatch in `abi/lift.go:liftOwnHandle` / `liftBorrowHandle` is already threaded through `ctx.Instance.LookupResourceType(rt.Instance, rt.Resource)` which resolves a `(instanceIdx, resourceIdx)` pair to a `*ResourceType` from the instance's `ResourceTypes` slice. That API does not depend on mutating `TypeResourceTable.Concrete`. **The actual Session 1 fix is:** leave `TypeResourceTable.Concrete = false` (the shared bag stays untouched), and populate `rt.ResourceTypes` directly. Then modify the `liftOwnHandle` / `liftBorrowHandle` check: instead of trapping when `!rt.Concrete`, check whether `ctx.Instance.LookupResourceType(rt.Instance, rt.Resource)` returns nil. The `TypeResourceTable.Concrete` flag becomes a Session 2 concern (cross-component resource-type matching).

Chosen approach: **the simpler path.** The `TypeResourceTable.Concrete` bit is not used by Session 1's lift/lower path. The `*ComponentTypes` bag stays immutable and shared. `rt.ResourceTypes` is populated at Instantiate time; `LookupResourceType` returns non-nil; lift/lower proceeds. The `TypeResourceTable.Concrete` check in the dispatch arms is removed — replaced by a nil check on `LookupResourceType`'s return.

**Populated field:** `TypeResourceTable.Resource` holds the declaration index within this component's type section (assigned by `InternAbstractResource`). This index is the same index used by the runtime to look up `rt.ResourceTypes[resourceIdx]`. The bijection between "declaration index in binary" and "index in runtime ResourceTypes slice" is maintained by Session 1's Instantiate walking `compiled.Types.ResourceTables` in declaration order.

Cross-instance resource handles (where a handle's `expectedRT.Impl != ctx.Instance` and the target instance is not in `ctx.Instance`'s `Parent` chain) still trap with a precise error: `"no resource type for instance N declaration M (cross-instance resolution: session 2 wiring)"`. This is unchanged from Session 0's behavior at that code path; the change is that same-instance resolution now succeeds.

### Decision 3: `component.Instance` embeds `*runtime.ComponentInstance`

```go
package component

import (
    "github.com/tetratelabs/wazero/api"
    "github.com/tetratelabs/wazero/internal/component/runtime"
    "github.com/tetratelabs/wazero/internal/component/types"
)

// Instance is the linker/compile-time wrapper around a running component
// instantiation. The per-instance runtime state that matches the
// canonical-abi spec's ComponentInstance (definitions.py:256-273) lives on
// the embedded *runtime.ComponentInstance. Wrapper-level state (core
// module instances, component-level exports, linker-time index spaces) is
// kept on this struct because runtime/ cannot import component/ without
// introducing an import cycle.
type Instance struct {
    // rt is the per-instance runtime state. One-to-one with this Instance
    // and non-nil after NewInstance.
    rt *runtime.ComponentInstance

    // Linker-time state.
    component      *Component
    coreInstances  []api.Module
    exports        map[string]*ExportedFunc
    componentFuncs map[uint32]ComponentFunc

    // Value index space for start function support.
    values         []types.Val
    valuesConsumed []bool

    // Wrapper-layer instance tree. rt.Parent holds the runtime-layer
    // back-pointer; children / instanceSpace / etc. hold *component.Instance
    // wrapper pointers so linker code can navigate without going through rt.
    children          []*Instance
    instanceSpace     []*Instance
    typeSpace         []*TypeDef
    componentSpace    []*Component
    exportedInstances map[string]*Instance
}
```

**Deleted fields** (moved to rt): `table *runtime.Table`, `destructors map[uint32]func(any)`, `mayLeaveDisabled bool`, `activeCallDepth int32`, `callContext *runtime.CallContext`.

**Kept fields**: `parent *Instance` stays as the wrapper-layer back-pointer (see "Wrapper-layer parent pointer" below). It is paired with `rt.Parent` at construction time; the wrapper pointer is a linker-time convenience, not duplicated runtime state.

**Deleted methods** (delegated to rt): `MayLeave() bool`, `SetMayLeave(bool)`, `ActiveCallDepth() int`, `EnterCall()`, `ExitCall()`, `CallMightBeRecursive(caller *Instance) bool`, `ValidateMayLeave() error`, `ValidateNotRecursive(caller *Instance) error`, `CallContext() *runtime.CallContext`, `SetCallContext(*runtime.CallContext)`, `SetDestructor(resourceTypeIdx uint32, dtor func(any))`, `Parent() *Instance`, `GetAncestor(depth uint32) *Instance`.

**New methods on `component.Instance`:**

- `(i *Instance) Runtime() *runtime.ComponentInstance { return i.rt }` — accessor.
- `(i *Instance) Parent() *Instance { ... walks via rt.Parent + wrapper-back-pointer map maintained by the linker; see below ... }`.
- `(i *Instance) MayLeave() bool { return i.rt.IsMayLeave() }`.
- `(i *Instance) ActiveCallDepth() int { return i.rt.EnterCount() }`.
- `(i *Instance) EnterCall() { i.rt.Enter() }`.
- `(i *Instance) ExitCall() { i.rt.Leave() }`.
- `(i *Instance) CallMightBeRecursive(caller *Instance) bool { ... uses rt.Reentrance or rt identity ... }`.
- `(i *Instance) ValidateMayLeave() error { if !i.rt.IsMayLeave() { return errMayNotLeave }; return nil }`.
- `(i *Instance) CallContext() *runtime.CallContext { ... }`.

**Wrapper-layer parent pointer:** because `runtime.ComponentInstance.Parent` is `*runtime.ComponentInstance` (not `*component.Instance`), the wrapper-layer `Parent() *Instance` method cannot simply `return i.rt.Parent.<something>`. Two options:

1. Linker maintains a `componentLinker.instanceWrappers map[*runtime.ComponentInstance]*component.Instance` for wrapper lookup. `Parent() *Instance` looks up `l.instanceWrappers[i.rt.Parent]`.
2. `component.Instance` keeps a `parent *Instance` wrapper back-pointer alongside `rt.Parent`. Both are set at construction (`parent.rt` is set to `rt.Parent`). The wrapper back-pointer is a linker-time convenience, not spec state.

**Chosen: option 2.** The wrapper back-pointer is cheap (one pointer per instance), avoids introducing a map lookup at every `Parent()` call, and has zero risk of the map getting out of sync. The "no parallel paths" constraint targets *duplicated runtime state*, not convenience back-pointers for wrapper navigation.

**`IsMayLeave` semantic fix.** Wazero's current `runtime.ComponentInstance.IsMayLeave() bool` returns `c.MayLeave && c.enterCount == 0`. Spec `definitions.py:260, 270, 1955, 1973, 2065, 2135, 2143` defines `may_leave` as a standalone boolean field — there is no coupling with call depth. The `enter_count` concept is wazero-specific and serves a different purpose (reentrance detection, used by `CallMightBeRecursive`, separate from `may_leave` gating). Session 1 fixes this:

```go
// runtime/component_instance.go — Session 1 fix
func (c *ComponentInstance) IsMayLeave() bool {
    return c.MayLeave   // spec definitions.py:260 — standalone boolean
}
```

The `enterCount` field stays (used by `Enter()`/`Leave()`/`EnterCount()` for reentrance-related bookkeeping), but it no longer gates `IsMayLeave`.

**`CallMightBeRecursive` transitive ancestor check.** Spec `definitions.py:290-299` `call_might_be_recursive(caller, callee_inst)` uses `reflexive_ancestors()` overlap, not direct caller equality. The wazero delegator must not just compare `caller == i`; it must check whether `caller` is in any of `i`'s ancestor chain OR `i` is in any of `caller`'s ancestor chain.

**B4 corrective (commit `b74f5558`, 2026-04-08):** The implementation below reflects the spec-correct structural walk. An earlier version of this design delegated to `runtime.ReentranceTracker.CallMightBeRecursive(callerID)`, but the tracker models runtime-stack membership (which is what `definitions.py:3664-3667`'s concurrency trap consults), NOT structural ancestry. They are different checks and must not be conflated. The structural walk below matches `definitions.py:290-299` exactly; the `ReentranceTracker` continues to exist for its separate concurrency-trap role and must not be reintroduced as a substitute for this check.

```go
// internal/component/instance.go
//
// component.Instance delegator — structural reflexive-ancestor overlap.
func (i *Instance) CallMightBeRecursive(caller *Instance) bool {
    if i == nil || caller == nil {
        return false
    }
    // Spec definitions.py:290-299: check whether caller.inst is a reflexive
    // ancestor of callee_inst, or vice versa. A nil caller models a host
    // call with no supertask chain and reduces to returning false.
    return isReflexiveAncestor(caller, i) || isReflexiveAncestor(i, caller)
}

// isReflexiveAncestor reports whether ancestor appears in descendant's
// parent chain (reflexive: descendant qualifies as its own ancestor).
// Spec: definitions.py ComponentInstance.reflexive_ancestors().
func isReflexiveAncestor(ancestor, descendant *Instance) bool {
    for cur := descendant; cur != nil; cur = cur.parent {
        if cur == ancestor {
            return true
        }
    }
    return false
}
```

The `ReentranceTracker` is populated during canon.lift / canon.lower closures via `EnterInstance`/`LeaveInstance` at call entry/exit for the separate task-level concurrency trap at `definitions.py:3664-3667`. It is NOT consulted by `CallMightBeRecursive`.

### Decision 4: Four latent `lift.go` gaps fixed per spec

All four fixes land in `abi/lift.go`, `runtime/table.go`, and `imports/wasip2/io/*`.

**Gap 1 — `trap_if(not h.own)` in `liftOwnHandle`.** Spec `definitions.py:1338`. Current `abi/lift.go:665` calls `ctx.Instance.Table.Remove(h)` after only `ValidateType`; a borrow handle with a matching `RT` would be silently removed. **Fix:** after retrieving the entry, assert `entry.Own == true` before calling `Remove`. Trap otherwise.

**Gap 2 — `trap_if(h.num_lends != 0)` in `liftOwnHandle`.** Spec `definitions.py:1337`. The current body calls `ValidateType` and then `Remove` with no lends check — an own handle with outstanding borrows must trap per spec. **Fix:** assert `entry.NumLends == 0` before `Remove`. Trap otherwise.

**Gap 3 — Generation-tag bridging via `Table.GetByIndex`.** Wazero's `runtime.Table` uses 64-bit generation-tagged handles. Wasm memory carries only the low 32 bits (the slot index). Current `abi/lift.go:660` constructs `runtime.Handle(handleIdx)` which is a 64-bit handle with generation = 0, so any recycled slot (generation > 0) is unreachable. **Fix:** add a new method on `runtime.Table`:

```go
// runtime/table.go
//
// GetByIndex looks up an entry by slot index (the low 32 bits of a Handle),
// returning the current generation-tagged Handle and the entry. Used by the
// canonical ABI lift path to resolve a Wasm-side u32 handle index to the
// runtime's generation-tagged handle.
//
// Returns ErrInvalidHandle if the slot index is out of range or the slot is
// currently free.
func (t *Table) GetByIndex(idx uint32) (Handle, TableEntry, error) {
    if int(idx) >= len(t.entries) {
        return 0, nil, ErrInvalidHandle
    }
    slot := &t.entries[idx]
    if slot.entry == nil {
        return 0, nil, ErrInvalidHandle
    }
    h := makeHandle(idx, slot.generation)  // reconstructs the 64-bit tagged handle
    return h, slot.entry, nil
}
```

Every `abi/lift.go`, `abi/lower.go`, and `instance.go` call site that currently constructs `runtime.Handle(handleIdx)` from a Wasm-side u32 migrates to `Table.GetByIndex(handleIdx)`. This includes `liftOwnHandle`, `liftBorrowHandle`, `lowerOwnHandleFlat`, `lowerBorrowHandleFlat`, and the new spec-correct `Instance.ResourceNew` / `ResourceRep` / `ResourceDrop` method bodies (Decision 7).

**Gap 4 — `ResourceHandleEntry.Rep` must be `uint32` per spec.** Spec `definitions.py:337-349` defines `ResourceHandle.rep: int` (a Python int — semantically u32, matches the flat ABI i32 handle encoding). Wasmtime uses `rep: u32` throughout (`runtime/component/instance.rs:383-387` `resource_new32`, `runtime/vm/component/resources.rs` and `host_tables.rs`). Wazero's current `ResourceHandleEntry.Rep any` is a wazero-specific divergence that breaks both the `.(uint32)` type assertion in any spec-correct `canon.resource.rep` body and the "`h.rep` is a u32" invariant that lift/lower rely on.

**Fix:** change `runtime.ResourceHandleEntry.Rep` from `any` to `uint32`. After this change, `liftOwnHandle` returns `types.ValOwn(entry.Rep)` directly (spec `definitions.py:1339` — `return h.rep`), `liftBorrowHandle` returns `types.ValBorrow(entry.Rep)` (spec `:1347`), and `ResourceRep` returns `entry.Rep` without a type assertion.

**Host-managed resources migration.** Host modules (`imports/wasip2/io/streams.go` etc.) currently store Go pointers (`*InputStream`, `*OutputStream`, `*Pollable`, `*Error`) in `Rep`. They migrate to the wasmtime pattern: maintain a per-module registry of host state, use a u32 id as the rep, and look up the Go object via the id at host-side access time:

```go
// imports/wasip2/io/streams.go — new host state pattern
package io

import "sync"

// inputStreamRegistry holds the host-side InputStream state indexed by a u32 id.
// The id is what goes into the runtime Table as the Rep, so it round-trips
// through canon.resource.new / canon.resource.rep as a spec-conformant u32.
var (
    inputStreamRegistryMu sync.Mutex
    inputStreamRegistry   []*InputStream // index = id; nil slot = freed
    inputStreamFreelist   []uint32
)

func registerInputStream(s *InputStream) uint32 {
    inputStreamRegistryMu.Lock()
    defer inputStreamRegistryMu.Unlock()
    if n := len(inputStreamFreelist); n > 0 {
        id := inputStreamFreelist[n-1]
        inputStreamFreelist = inputStreamFreelist[:n-1]
        inputStreamRegistry[id] = s
        return id
    }
    id := uint32(len(inputStreamRegistry))
    inputStreamRegistry = append(inputStreamRegistry, s)
    return id
}

func getInputStream(id uint32) *InputStream {
    inputStreamRegistryMu.Lock()
    defer inputStreamRegistryMu.Unlock()
    if int(id) >= len(inputStreamRegistry) {
        return nil
    }
    return inputStreamRegistry[id]
}

func unregisterInputStream(id uint32) {
    inputStreamRegistryMu.Lock()
    defer inputStreamRegistryMu.Unlock()
    if int(id) < len(inputStreamRegistry) {
        inputStreamRegistry[id] = nil
        inputStreamFreelist = append(inputStreamFreelist, id)
    }
}
```

Minting a handle: `id := registerInputStream(stream); handle := table.NewResourceHandle(id, true, inputStreamResourceType)`. Reading: `entry := table.Get(h); stream := getInputStream(entry.(*ResourceHandleEntry).Rep)`. Destroying: the existing `Destroyable` interface on `*InputStream` handles cleanup via `Rep.Destroy()` — but since `Rep` is now a u32, the `Destroyable` integration moves: the `Table.Remove` path for host resources looks up the host object via `getInputStream(entry.Rep)`, calls its `Destroy()`, and then calls `unregisterInputStream(entry.Rep)`. Alternative: the `runtime.ResourceType` gains a Go-side `HostDestructor func(rep uint32)` field that the table invokes on remove; `inputStreamResourceType` sets it to a closure that looks up and destroys the stream.

**Chosen:** add a `HostDestructor func(rep uint32)` field to `runtime.ResourceType`. For guest resources (minted from the guest side via canon.resource.new), `HostDestructor` is nil and destruction goes through `Dtor *uint32` (the core function). For host resources, `HostDestructor` is set to `func(rep uint32) { if s := getInputStream(rep); s != nil { s.Destroy(); unregisterInputStream(rep) } }`. `runtime.Table.Remove` dispatches on which is non-nil. The `Destroyable` interface on `*InputStream` is preserved (the `Destroy()` method still exists on the type), but the invocation path shifts from "Table.Remove reads Rep as Destroyable" to "Table.Remove calls HostDestructor with the u32 rep, which looks up the Destroyable in the per-module registry and invokes it."

This migration is Session 1 scope: it is mechanically required to make `Rep: uint32` and the spec-correct lift/lower bodies compile together. The imports/wasip2 files need touching anyway because of the Decision 3 `component.Instance` embedding. Session 1 does not treat this as "out of scope" — the user's "I don't care about preserving existing wazero code" constraint applies.

### Decision 5: `Component.TypeDefs []TypeDef` restores the per-slot index

Add one field to `component.Component`:

```go
// internal/component/component.go
type Component struct {
    // ... existing fields ...

    // TypeDefs is one entry per slot in the component's type index
    // space, in declaration order. Densely aligned with
    // Component.NextTypeIdx — every slot that bumps NextTypeIdx has
    // exactly one corresponding entry here, including outer and
    // export type aliases.
    //
    // Aliases are stored with Kind == TypeDefKindAlias and a
    // populated Alias *AliasTarget field carrying the unresolved
    // target metadata. Callers that need to resolve a raw typeidx to
    // a concrete TypeDef MUST go through Component.ResolveTypeDef,
    // which walks the alias chain — mirror of wasmparser's
    // Validator.component_any_type_at(typeidx).
    //
    // Every caller that previously used CanonicalDef.TypeIdx /
    // ImportExternDesc.TypeIdx / Export.TypeIdx / InstanceExport.TypeIdx
    // resolves the raw type-section index through this slice:
    //
    //   slot, _, err := c.ResolveTypeDef(canon.TypeIdx)
    //   if err != nil { ... }
    //   switch slot.Kind {
    //   case TypeDefKindFunc:     use slot.Func
    //   case TypeDefKindResource: use slot.Resource (ResourceTableIdx)
    //   case TypeDefKindDefined:  use slot.ValType
    //   ...
    //   }
    TypeDefs []TypeDef
}
```

The alias variant is carried on a new `AliasTarget` struct that records
the unresolved target metadata for a type alias. Exactly one of the
two target shapes is populated per instance (outer vs export alias):

```go
// Spec: Binary.md:118-126 aliastarget grammar.
type AliasTarget struct {
    // IsExport selects between the two variants: true = export-alias
    // (InstanceIdx + ExportName), false = outer-alias (OuterCount +
    // OuterIndex).
    IsExport bool

    // Outer alias fields (when IsExport == false).
    OuterCount uint32
    OuterIndex uint32

    // Export alias fields (when IsExport == true).
    InstanceIdx uint32
    ExportName  string
}
```

`TypeDefKind` gains one new variant — `TypeDefKindAlias` — alongside
the existing Func/Component/Instance/Resource/Defined variants. A
slot with `Kind == TypeDefKindAlias` has `Alias != nil` and every
other kind-specific field zeroed.

**Densification invariant.** `len(c.TypeDefs) == c.NextTypeIdx` after
`DecodeComponent` returns. This mirrors wasmparser's flat typeidx-
indexed table in `Validator`, where every type-section entry AND every
outer/export type alias consumes a slot in the component's flat type
index space. Wizer's standalone counter-based parser makes this
explicit: `crates/wizer/src/component/parse.rs:110-185` calls
`inc_types()` for every type-section entry AND every outer/export
type alias. Wasmtime's translator never indexes a flat typeidx array
directly — it calls `validator.types(0).component_any_type_at(idx)`
which transparently walks alias chains
(`crates/environ/src/component/translate.rs:796-801`). Wazero's
`Component.ResolveTypeDef` is the Go equivalent.

The `TypeDef` struct already exists at `component.go:126–144` in the
post-Session-0 shape. Session 1 populates it during decoding:

- Every type-section slot appends exactly one `TypeDef` to
  `dc.c.TypeDefs`.
- Every outer or export type alias section also appends exactly one
  `TypeDef` (with `Kind == TypeDefKindAlias`) to preserve the flat
  typeidx index space.
- `decodeTypeSection` (at `binary/decoder.go:216`) gains a one-liner
  per case:

```go
case TypeOpFuncSync, TypeOpFuncAsync:
    // ... existing decode ...
    dc.funcTypeIdx[slot] = ftIdx   // DELETED
    dc.scope.appendOther()
    dc.c.TypeDefs = append(dc.c.TypeDefs, component.TypeDef{
        Kind: component.TypeDefKindFunc,
        Func: &dc.c.Types.Funcs[ftIdx],   // pointer into the canonical bag
    })

case TypeOpResourceSync:
    // ... existing decode ...
    dc.resourceDefs[slot] = resourceDef   // DELETED
    dc.scope.appendResource(resourceDef.ResourceTableIdx)
    dc.c.TypeDefs = append(dc.c.TypeDefs, component.TypeDef{
        Kind:     component.TypeDefKindResource,
        Resource: resourceDef.ResourceTableIdx,
    })

// (same for TypeOpResourceAsync, TypeOpInstance, TypeOpComponent, and every ValTypeOpcode* case)
```

For type aliases, `binary/alias.go`'s `SortType` branches (both outer
and export) append a `TypeDefKindAlias` entry alongside the
`NextTypeIdx++` bump:

```go
case component.SortType:
    alias.Idx = c.NextTypeIdx
    c.NextTypeIdx++
    c.TypeDefs = append(c.TypeDefs, component.TypeDef{
        Kind: component.TypeDefKindAlias,
        Alias: &component.AliasTarget{
            IsExport:   false,       // or true for export aliases
            OuterCount: alias.OuterCount,
            OuterIndex: alias.OuterIndex,
        },
    })
```

`DecodeComponent` ends with an explicit invariant check
(`len(dc.c.TypeDefs) == dc.c.NextTypeIdx`) that surfaces any
densification regression as a typed error rather than a silent
sparsity bug.

**Gotcha: the `*types.TypeFunc` pointer problem.** The canonical bag's `Funcs []TypeFunc` is a slice; appending to it invalidates pointers to earlier entries. The decoder populates `TypeDefs` incrementally during decoding (before the builder is finished), so any `&dc.c.Types.Funcs[ftIdx]` captured now will dangle after subsequent appends. **Resolution:** store `types.FuncTypeIdx` on `TypeDef.Func` as an index, not a pointer. But the existing `TypeDef.Func` is typed `*types.TypeFunc`. Options:

- **A:** change `TypeDef.Func` to `types.FuncTypeIdx` (a uint32 alias). Callers that want the `*TypeFunc` do `&c.Types.Funcs[td.Func]` after the bag is finished.
- **B:** defer population of `TypeDef.Func` until `builder.Finish()` has been called (the bag is frozen and pointers are stable). Store `types.FuncTypeIdx` in a temporary per-slot slice during decoding, then rewrite `TypeDefs` with `*TypeFunc` pointers after `Finish`.

**Chosen: option A.** `TypeDef.Func` becomes `types.FuncTypeIdx`. This removes the dangling-pointer footgun entirely and is more consistent with the other fields (`TypeDef.Resource` is already `types.ResourceTableIdx`, an index). All callers of `td.Func` update mechanically to `&c.Types.Funcs[td.Func]` (or a helper on `TypeDef`: `(td *TypeDef) FuncType(c *Component) *types.TypeFunc`).

**Deleted:** `decodeContext.funcTypeIdx` and `decodeContext.resourceDefs` maps on `binary/decoder.go`. Their callers inside the `binary/` package (if any) migrate to `c.TypeDefs[slot]`.

### Decision 6: Type checker + dynamic-host-function model (revised 2026-04-09)

**Revision history:** This Decision was rewritten on 2026-04-09 after a deep audit of wasmtime's component-model linker/host APIs. The original Decision 6 prescribed a "host declares `*types.TypeFunc` at registration time" model with per-module `ComponentTypesBuilder`s. That model was incompatible with three things at once: (a) wasmtime's actual dynamic host-function path, which never asks the host to declare a type; (b) Decision 5's `c.TypeDefs[idx]` integer-index comparison which requires both sides to live in the same bag; and (c) the cross-subtree resource identity problem (wasi-clocks's `pollable` and wasi-io's `pollable` must be the SAME type, but per-subtree builders mint disjoint indices). The audit findings are recorded in `docs/superpowers/reviews/2026-04-08-session1-review-log.md` under the "Gap 1 + Gap 2 wasmtime audit" entry.

#### The wasmtime model

Wasmtime offers two host-function registration paths. Both live on `LinkerInstance<'a, T>` in `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs`:

1. **`func_wrap` (statically typed)** at `linker.rs:426-434`:
   ```rust
   pub fn func_wrap<F, Params, Return>(&mut self, name: &str, func: F) -> Result<()>
   where F: Fn(StoreContextMut<T>, Params) -> Result<Return> + ...,
         Params: ComponentNamedList + Lift + 'static,
         Return: ComponentNamedList + Lower + 'static,
   ```
   The function's type is **derived from `F`'s signature** via Rust generic monomorphization. No explicit `FuncType` argument. Used by `wit-bindgen` generated host stubs.

2. **`func_new` (dynamically typed)** at `linker.rs:665-675`:
   ```rust
   pub fn func_new(
       &mut self,
       name: &str,
       func: impl Fn(StoreContextMut<'_, T>, types::ComponentFunc, &[Val], &mut [Val]) -> Result<()>,
   ) -> Result<()>
   ```
   **No `FuncType` argument is ever passed by the caller.** The `types::ComponentFunc` type handle is supplied by the runtime to the callback at call time, looked up from the component's import declaration. Used by hand-written embedders.

Both paths share a `HostFunc` (`func/host.rs:35-53`) carrying a `typecheck: fn(TypeFuncIndex, &InstanceType<'_>) -> Result<()>` function pointer that runs at `instantiate_pre` time against the component's bag (`linker.rs:163-181`, `matching.rs:168-174`). For the dynamic path (`DynamicHostFn::typecheck` at `host.rs:619-626`), this function only validates the async bit:

```rust
fn typecheck(ty: TypeFuncIndex, types: &InstanceType<'_>) -> Result<()> {
    let ty = &types.types[ty];
    if ASYNC != ty.async_ { bail!("type mismatch with async"); }
    Ok(())
}
```

Param/result types are validated **at lift/lower time** against the component-declared type via `cx.types[ty]` (`host.rs:640-694`). The host accepts ANY type the component declares; the runtime lifts the params via the component's import type and hands the host a `&[Val]`.

**The point.** Wasmtime never asks the host to declare a type for the dynamic path because the component's import declaration IS the source of truth. Requiring the host to redeclare it adds 200 sites of duplicated work, creates cross-subtree identity bugs (each builder mints its own resource indices), and is more strict than wasmtime without buying any safety.

#### Cross-subtree resource identity in wasmtime

Wasmtime never has cross-subtree identity bugs because **there is no host-side type bag at all**. `Linker<T>` holds `NameMap<usize, Definition>` plus an `Engine` (`linker.rs:61-68`). Resource types are `ResourceType::host::<T>() = TypeId::of::<T>()` (`resources/ty.rs:44-48`). When `wasi-http` exports a function taking `Resource<InputStream>`, the type identity is `TypeId::of::<InputStream>()` which is the same `TypeId` no matter which crate registers it. wit-bindgen's `with: { "wasi:io": wasmtime_wasi::p2::bindings::io }` directive (`wasi-http/src/bindings.rs:14-30`) re-exports the SAME Rust types across crates via `pub type Request = super::http_types::Request` aliases (`component-macro/tests/expanded/share-types.rs:257`). Cross-crate type identity flows via Rust module paths, not via interned numeric indices.

The Go equivalent of "Rust type identity flows via the type system" is "the component's import declaration is the canonical type, and the host accepts any value the runtime lifts against that type." The host function gets a `*types.TypeFunc` parameter at call time and a `[]types.Val` carrying the lifted args.

#### Wazero's adopted model

**`ComponentLinker.DefineFunc` / `ComponentInstanceBuilder.Func` / `Linker.DefineFunc` / `InstanceBuilder.Func` take NO `*types.TypeFunc` parameter at registration time.** The host registers a typed `HostFunc`:

```go
// internal/component/linker.go
//
// HostFunc is the canonical host-function callback shape, modeled on
// wasmtime's `func_new` dynamic host path (linker.rs:665-675,
// host.rs:619-626). The fnType parameter is supplied by the runtime
// at call time, looked up from the component's import declaration
// (the canonical source of truth — there is no host-declared type).
//
// Spec: definitions.py:1997 (canon_lift), :2089 (canon_lower) lift
// the args against the component's import type and pass them as
// []types.Val to the host. The host returns []types.Val which the
// runtime lowers back per the same import type.
type HostFunc func(
    ctx context.Context,
    fnType *types.TypeFunc,
    args []types.Val,
) ([]types.Val, error)
```

At the public API surface:

```go
// api/component.go
type ComponentInstanceBuilder interface {
    // Func adds a host function export. fn is the canonical typed
    // HostFunc. The component-declared import type is supplied to fn
    // as its second argument at call time; the host has no type to
    // declare. Mirrors wasmtime LinkerInstance::func_new.
    Func(name string, fn component.HostFunc) ComponentInstanceBuilder

    Resource(name string, dtor func(rep uint32)) ComponentInstanceBuilder
    SkipValidation() ComponentInstanceBuilder
    Build() error

    internalapi.WazeroOnly
}
```

`InstanceBuilder.FuncNoType` is **deleted** — the typed `Func` IS the canonical entry point because there is no untyped variant under the new model.

#### Type checker — what changes

The type checker still does three things, but bullet (2) is **dropped entirely**:

1. `checkFuncDefinition` / `checkInstanceDefinition` currently ignore the `expected` side entirely (`type_checker.go:192, 206`). Session 1 reads `expected.TypeIdx` via `c.ResolveTypeDef(expected.TypeIdx)` (Decision 5) and resolves to a concrete `*types.TypeFunc` / `*InstanceTypeDef`. For host functions, the type checker stores the resolved `*types.TypeFunc` on the resolved `FuncDef` so that lift/lower at call time can pass it to the host's `HostFunc` callback. This mirrors wasmtime's `cx.types[ty]` pattern at `host.rs:640-694`.

2. ~~Host functions without a declared type currently slip through silently. Session 1 **requires** every host `FuncDef` to carry a non-nil `*types.TypeFunc`...~~ **DROPPED.** Wasmtime's `DynamicHostFn::typecheck` at `host.rs:619-626` proves this is over-strict. The host accepts the component's import type by definition; the type checker's job is to look up the import's `*types.TypeFunc` and bind it to the host callback for use at lift/lower time.

3. `checkInstance` currently only checks that each required export exists + has the right Go-level kind (`type_checker.go:76-97`). Session 1 extends it to **recursively** type-check each declared export against the corresponding actual export (func → recurse into `checkFuncType`, instance → recurse into `checkInstanceDefinition`). Matches wasmtime `matching.rs:162`. For host-function actuals, this is a no-op (the host accepts any type) — only the component's declared signature is structurally validated against itself + against any nested imports.

`checkFuncType` uses identity on `(Async, Params, Results)` only — **no ParamNames comparison**. Spec `definitions.py:88-101` `FuncType.param_types()` / `result_type()` return ValTypes with names stripped; names are metadata, not part of the type equation. Wasmtime's `matching.rs` also does not compare names. This rule still applies for the cross-component / nested-component cases where both sides are component-declared.

Full function bodies for the rewritten `checkFuncType`, `checkFuncDefinition`, and `checkInstanceDefinition` are in the "Type Checker Scope — Session 1" section later in this document.

#### Lift/lower wiring

When the component invokes a host import, the canon.lower (or canon.lift, depending on direction) site:

1. Resolves the import's `*types.TypeFunc` from the component's bag via the type checker's recorded mapping.
2. Lifts the core wasm flat values into a `[]types.Val` using `abi.LiftParams(ctx, ft.Params, flat, MaxFlatParams)`.
3. Invokes `hostFunc(ctx, ft, vals)` — handing the host the canonical type AND the lifted values.
4. Lowers the returned `[]types.Val` back into core wasm flat values using `abi.LowerResults(ctx, ft.Results, results, stack, needsRetptr, MaxFlatResults)`.

The host function never constructs a `*types.TypeFunc`. It only inspects the one supplied by the runtime (typically ignoring it and reading from `args` directly).

#### Cross-bag structural walk — still Session 2

For the case where two different components have different `*types.ComponentTypes` bags and need to satisfy imports across the boundary, Session 1 retains the same-bag integer comparison (Decision 5's `c.TypeDefs[idx]` resolution after `ResolveTypeDef` walks aliases) and defers structural cross-bag walking to Session 2. The host-side dynamic-typing model removes one source of cross-bag traffic (host modules no longer mint types in a separate bag); the remaining cross-bag case is purely component-vs-component.

#### Authoritative WIT schema for wasip2 host modules

When wazero's `imports/wasip2/<subtree>/` host modules dispatch on the component-declared import type at call time, they need to KNOW which fields/cases the type contains so they can correctly read `args` and construct results. The authoritative source for every wasip2 type is the WIT schema vendored at `debug-vendored/WASI/proposals/<subtree>/wit/`:

| wazero subtree | WIT source |
|---|---|
| `imports/wasip2/cli/` | `debug-vendored/WASI/proposals/cli/wit/{environment,exit,stdio,terminal,imports}.wit` |
| `imports/wasip2/clocks/` | `debug-vendored/WASI/proposals/clocks/wit/{monotonic-clock,wall-clock}.wit` |
| `imports/wasip2/filesystem/` | `debug-vendored/WASI/proposals/filesystem/wit/{types,preopens}.wit` |
| `imports/wasip2/http/` | `debug-vendored/WASI/proposals/http/wit/{types,handler,proxy}.wit` |
| `imports/wasip2/io/` | `debug-vendored/WASI/proposals/io/wit/{error,poll,streams}.wit` |
| `imports/wasip2/random/` | `debug-vendored/WASI/proposals/random/wit/{random,insecure,insecure-seed}.wit` |
| `imports/wasip2/sockets/` | `debug-vendored/WASI/proposals/sockets/wit/{network,tcp,tcp-create-socket,udp,udp-create-socket,ip-name-lookup,instance-network}.wit` |

Package version: `@0.2.9` (wazero pins import strings at `@0.2.0`, the first stable release of the 0.2.x series; types are forward-compatible within 0.2.x). Every migrated `imports/wasip2/<subtree>/*.go` file MUST carry a top-of-file comment of the form:

```go
// WIT source of truth: debug-vendored/WASI/proposals/<subtree>/wit/<file>.wit
// Package version: wasi:<name>@0.2.9 (wazero targets @0.2.0)
```

so that reviewers can diff the host's case-handling against the canonical schema. Variant case order MUST match the WIT source exactly — encoding `wasi:http/types.error-code` (40 cases), `wasi:filesystem/types.error-code` (37 cases), or `wasi:sockets/network.error-code` (22 cases) from memory rather than from the file is the most dangerous pattern in this migration.

DO NOT use `debug-vendored/wasmtime/crates/wasi/src/p2/wit/deps/*.wit` (package version `@0.2.6`) — although it is functionally compatible, the `debug-vendored/WASI/proposals/` tree is the upstream source. The wasmtime copy is for cross-reference only.

### Decision 7: Spec-correct `Instance.ResourceNew` / `ResourceRep` / `ResourceDrop` signatures

Current signatures (all panic):

```go
func (i *Instance) ResourceNew(rep any) (uint32, error)
func (i *Instance) ResourceRep(handleIdx uint32) (any, error)
func (i *Instance) ResourceDrop(handleIdx uint32, resourceTypeIdx uint32) error
```

New signatures (spec-matching `definitions.py:2134, 2142, 2169`):

```go
// ResourceNew is canon.resource.new — spec definitions.py:2134-2138.
// Mints a new own-handle of the given resource type and returns the
// Wasm-side handle index. Traps if !may_leave. resourceIdx is the
// declaration index in the component's type section; it resolves via
// i.rt.ResourceTypes[resourceIdx] to the pointer-identity *ResourceType.
func (i *Instance) ResourceNew(resourceIdx types.ResourceIdx, rep uint32) (uint32, error)

// ResourceRep is canon.resource.rep — spec definitions.py:2169-2173.
// Returns the stored rep for a resource handle, after validating the
// handle's RT matches the expected resource declaration. Traps on type
// mismatch.
func (i *Instance) ResourceRep(resourceIdx types.ResourceIdx, handleIdx uint32) (uint32, error)

// ResourceDrop is canon.resource.drop — spec definitions.py:2142-2165.
// Removes the handle from the table, validates its RT, checks no
// outstanding lends, and invokes the destructor if the handle is an
// owned handle.
func (i *Instance) ResourceDrop(resourceIdx types.ResourceIdx, handleIdx uint32) error
```

Bodies:

```go
func (i *Instance) ResourceNew(resourceIdx types.ResourceIdx, rep uint32) (uint32, error) {
    if !i.rt.IsMayLeave() {
        return 0, errMayNotLeave
    }
    if int(resourceIdx) >= len(i.rt.ResourceTypes) {
        return 0, fmt.Errorf("resource.new: resource declaration %d not defined", resourceIdx)
    }
    rt := i.rt.ResourceTypes[resourceIdx]
    if rt == nil {
        return 0, fmt.Errorf("resource.new: resource type %d not concrete", resourceIdx)
    }
    h, err := i.rt.Table.NewResourceHandle(rep, true /* own */, rt)
    if err != nil {
        return 0, err
    }
    return h.Index(), nil   // Handle.Index() returns the low 32 bits of the tagged handle
}

func (i *Instance) ResourceRep(resourceIdx types.ResourceIdx, handleIdx uint32) (uint32, error) {
    if int(resourceIdx) >= len(i.rt.ResourceTypes) {
        return 0, fmt.Errorf("resource.rep: resource declaration %d not defined", resourceIdx)
    }
    rt := i.rt.ResourceTypes[resourceIdx]
    if rt == nil {
        return 0, fmt.Errorf("resource.rep: resource type %d not concrete", resourceIdx)
    }
    h, entry, err := i.rt.Table.GetByIndex(handleIdx)
    if err != nil {
        return 0, err
    }
    _ = h
    resEntry, ok := entry.(*runtime.ResourceHandleEntry)
    if !ok {
        return 0, runtime.ErrInvalidHandle
    }
    // Spec definitions.py:2172 — trap_if(h.rt is not rt).
    if resEntry.RT != rt {
        return 0, fmt.Errorf("resource.rep: type mismatch")
    }
    // Spec definitions.py:2173 — return h.rep. Rep is uint32 (Decision 4 Gap 4).
    return resEntry.Rep, nil
}

func (i *Instance) ResourceDrop(resourceIdx types.ResourceIdx, handleIdx uint32) error {
    // Spec definitions.py:2142-2165 canon_resource_drop.
    if !i.rt.IsMayLeave() {
        return errMayNotLeave
    }
    if int(resourceIdx) >= len(i.rt.ResourceTypes) {
        return fmt.Errorf("resource.drop: resource declaration %d not defined", resourceIdx)
    }
    rt := i.rt.ResourceTypes[resourceIdx]
    if rt == nil {
        return fmt.Errorf("resource.drop: resource type %d not concrete", resourceIdx)
    }
    h, entry, err := i.rt.Table.GetByIndex(handleIdx)
    if err != nil {
        return err
    }
    resEntry, ok := entry.(*runtime.ResourceHandleEntry)
    if !ok {
        return runtime.ErrInvalidHandle
    }
    // Spec :2147 — trap_if(h.rt is not rt).
    if resEntry.RT != rt {
        return fmt.Errorf("resource.drop: type mismatch")
    }
    // Spec :2148 — trap_if(h.num_lends != 0). Note: the spec assertion is
    // on the handle about to be dropped; outstanding lends on an OWN handle
    // are a trap; borrow handles don't have num_lends.
    if resEntry.Own && resEntry.NumLends != 0 {
        return fmt.Errorf("resource.drop: own handle has %d outstanding lends", resEntry.NumLends)
    }
    // Now remove from table. Table.Remove handles the unified slot-freelist
    // bookkeeping regardless of own/borrow kind.
    if _, err := i.rt.Table.Remove(h); err != nil {
        return err
    }
    if resEntry.Own {
        // Spec :2149-2161 — own branch: invoke destructor.
        if rt.HasDestructor() {
            if rt.Impl != i.rt {
                // Spec :2154-2160 — cross-instance destructor dispatch via
                // canon_lift → canon_lower. Session 2 work.
                return fmt.Errorf("resource.drop: cross-instance destructor invocation (session 2 wiring)")
            }
            // Spec :2151 — local destructor: rt.dtor(h.rep).
            if err := invokeLocalDestructor(i, rt, resEntry.Rep); err != nil {
                return fmt.Errorf("resource.drop: destructor: %w", err)
            }
        }
    } else {
        // Spec :2163-2164 — borrow branch: decrement h.borrow_scope.num_borrows.
        // In wazero, the borrow scope is tracked via the lender-set on the
        // borrow scope at lift time; dropping a borrow handle while the
        // borrow scope is still active is unusual (normally borrow handles
        // are cleaned up by the borrow scope's deliver_resolve at return,
        // not by canon.resource.drop). Session 1 matches the spec: if the
        // borrow scope is still open, decrement its lender counter.
        if resEntry.BorrowScope != nil {
            if err := resEntry.BorrowScope.ReleaseBorrow(h); err != nil {
                return fmt.Errorf("resource.drop: borrow release: %w", err)
            }
        }
    }
    return nil
}
```

The `invokeLocalDestructor` helper reads `rt.Dtor` (a `*uint32` core function index) from the defining instance's core module and calls it with `rep` as the single i32 argument, per spec `definitions.py:2151`. It returns any error the destructor produces. For host-managed resources where `HostDestructor` (Decision 4 Gap 4) is set, `invokeLocalDestructor` calls that instead of looking up a guest core function. The `Table.Remove` call itself is unified — it handles slot freelist bookkeeping regardless of own/borrow kind and does not invoke destructors; all destructor dispatch lives in `ResourceDrop`.

The borrow-branch `resEntry.BorrowScope.ReleaseBorrow(h)` is a new method on `runtime.BorrowScope` that decrements the scope's outstanding lend counter and removes `h` from the lender set. Current `runtime.BorrowScope.AddLender(h)` already tracks lenders; Session 1 adds the symmetric release operation.

The `canon.resource.new` / `canon.resource.drop` / `canon.resource.rep` host module exports (built by `createResourceOpExport` during Instantiate) close over the resolved `types.ResourceIdx` at host-module-creation time. The core wasm signature for each is unchanged from the pre-Session-0 shape: `(i32 rep) -> i32 handle`, `(i32 handle) -> ()`, `(i32 handle) -> i32 rep`. Only the Go body changes:

```go
// createResourceOpExport — Session 1 shape
func (l *ComponentLinker) createResourceOpExport(
    inst *Instance,
    name string,
    info canonResourceInfo,
) *HostModuleExport {
    // Resolve TypeIdx → ResourceIdx via Component.TypeDefs.
    td := inst.component.TypeDefs[info.typeIdx]
    if td.Kind != TypeDefKindResource {
        return nil
    }
    resourceTableIdx := td.Resource
    // ResourceTableIdx → ResourceIdx (for ResourceTypes slice) via
    // compiled.Types.ResourceTables (Session 1 maps them 1:1; Session 2
    // differentiates Abstract vs Concrete).
    resourceIdx := types.ResourceIdx(inst.component.Types.ResourceTables[resourceTableIdx].Resource)

    switch info.kind {
    case CanonKindResourceNew:
        return &HostModuleExport{
            Name:        name,
            ParamTypes:  []api.ValueType{api.ValueTypeI32},
            ResultTypes: []api.ValueType{api.ValueTypeI32},
            Func: api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
                rep := uint32(stack[0])
                h, err := inst.ResourceNew(resourceIdx, rep)
                if err != nil {
                    // Spec: traps are Wasm-level traps; wazero surfaces
                    // via panic(wasmruntime.Trap...) pattern consistent
                    // with other canon ops.
                    panic(err)
                }
                stack[0] = uint64(h)
            }),
        }
    // ... ResourceDrop, ResourceRep analogous ...
    }
}
```

### Decision 8: Test restoration methodology — git + upstream validation per test

For each `t.Skip("session 1 work")` test and each `TestXxxDeferredToSession1` stub:

1. **Locate the pre-Session-0 body** in git history. The compile-fix stub commit is `36a29b13`; the prior commit (`98b3bbc3` — "Rename ComponentInstance") has the last working body for most files.

   ```bash
   git show 98b3bbc3:internal/component/instance_test.go
   git show 98b3bbc3:internal/component/conformance/resources_test.go
   # ... etc.
   ```

2. **Rewrite the body** against the new types (`*types.ComponentTypes`, `abi.LiftContext`, `abi.LowerContext`, `*runtime.ComponentInstance`, `*runtime.ResourceType`, `runtime.Table`). Mechanical for most cases:
   - `types.S32{}` literals → `types.S32` (named scalar constant).
   - `types.Record{Fields: [...]}` literals → `builder.InternRecord([]types.RecordField{...})`.
   - `ResourceTable` → `Table`.
   - `NewResourceTable()` → `runtime.NewTable()`.
   - `table.New(rep, true)` → `table.NewResourceHandle(rep, true, resourceTypeSingleton)`.
   - `entry.Rep` → `entry.(*runtime.ResourceHandleEntry).Rep`.
   - `FuncType{Params: [...]}` → `types.TypeFunc{ParamNames: [...], Params: builder.InternTuple([...]), Results: builder.InternTuple([...])}`.
   - `liftResolvedType`/`liftFromStack`/`liftFieldFromMemory` → `abi.LiftFlat` / `abi.LiftHeap`.

3. **Validate behavior against upstream.** For each assertion in the restored test, cite one of:
   - A matching `definitions.py` line or function that the assertion verifies.
   - A matching wasmtime test (`debug-vendored/wasmtime/tests/all/component_model/*.rs`) that makes the equivalent assertion.
   - A matching wasmtime source behavior (with file:line citation).
   - A note that the behavior is a wazero-specific engineering invariant not covered by spec/wasmtime (e.g., "value index space out-of-bounds returns a typed error" — wazero invariant, no spec counterpart).

   The validation record is stored inline in the test body as comments:
   ```go
   func TestExportedFuncCall_OwnArgument(t *testing.T) {
       // Spec: definitions.py:1333-1339 (lift_own).
       // Wasmtime parallel: resources.rs:275-279 (resource_lift_own).
       // Upstream test: debug-vendored/wasmtime/tests/all/component_model/resources.rs (no direct counterpart — asserted via InstancePre+component integration tests in resources.rs).
       // ... test body ...
   }
   ```

4. **Rework or delete tests that contradict upstream.** If a restored test asserts behavior that `definitions.py` or wasmtime explicitly does not support (e.g., a test that expects a non-spec error message, or a test that relies on wazero-specific shortcut behavior that the spec forbids), the test is either reworked to match the spec or deleted with a commit message explaining why.

5. **Per-task spec-review subagent checklist amendment.** The Session 0 plan dispatched a spec-compliance review subagent after each task. Session 1 extends the spec reviewer's checklist with:
   - "Does every restored test in this task have a cited `definitions.py` / wasmtime / upstream reference?"
   - "Are any assertions in restored tests inconsistent with spec behavior?"
   - "Were any tests reworked or deleted during restoration? If so, is the reasoning documented?"

### Decision 9: Per-task spec + code-quality review discipline

Session 1 uses the `superpowers:subagent-driven-development` pattern from Session 0. Every numbered task in the implementation plan dispatches both:

1. **`superpowers:code-reviewer`** after implementation — checks plan adherence, code quality, test discipline.
2. **A spec-compliance review subagent** with an amended checklist for Session 1:
   - Does every new function / modified call site cite the spec or wasmtime reference line it implements?
   - Does every restored test cite its upstream counterpart (`definitions.py` line, wasmtime test file:line, or "no counterpart" rationale)?
   - Are any behavior deviations from spec explicitly flagged with a Session 2 / Later trap?

Session 0 had eight tasks that needed correctives based on review findings. Session 1's task count is 2–3× larger, so expect similar or greater corrective volume. The plan reserves time for correctives between checkpoints.

## Instance Layering — Concrete Shape

### Before (current HEAD)

```go
type Instance struct {
    component      *Component
    coreInstances  []api.Module
    exports        map[string]*ExportedFunc
    componentFuncs map[uint32]ComponentFunc

    // Duplicated runtime state (deleted in Session 1):
    table            *runtime.Table
    destructors      map[uint32]func(any)
    callContext      *runtime.CallContext
    mayLeaveDisabled bool
    activeCallDepth  int32

    values         []types.Val
    valuesConsumed []bool

    parent   *Instance          // duplicated tree; Session 1 keeps wrapper back-pointer
    children []*Instance        // wrapper-layer tree

    instanceSpace  []*Instance
    typeSpace      []*TypeDef
    componentSpace []*Component

    exportedInstances map[string]*Instance
}
```

### After (Session 1)

```go
type Instance struct {
    // rt is the per-instance runtime state matching the canonical-abi
    // spec's ComponentInstance (definitions.py:256-273). Holds Table,
    // MayLeave, enterCount, ResourceTypes, Destructors, Reentrance,
    // Parent (a *runtime.ComponentInstance back-pointer).
    rt *runtime.ComponentInstance

    // Linker-time state. Cannot live on runtime.ComponentInstance without
    // creating a runtime → component import cycle.
    component      *Component
    coreInstances  []api.Module
    exports        map[string]*ExportedFunc
    componentFuncs map[uint32]ComponentFunc

    values         []types.Val
    valuesConsumed []bool

    // Wrapper-layer instance tree. rt.Parent holds the runtime back-pointer;
    // parent / children hold *component.Instance wrapper pointers so linker
    // code can navigate the wrapper tree without a runtime→component map.
    parent            *Instance
    children          []*Instance
    instanceSpace     []*Instance
    typeSpace         []*TypeDef
    componentSpace    []*Component
    exportedInstances map[string]*Instance
}
```

### Construction

```go
func newInstance(c *Component, id uint32, parent *Instance) *Instance {
    var parentRT *runtime.ComponentInstance
    if parent != nil {
        parentRT = parent.rt
    }
    inst := &Instance{
        component:      c,
        rt:             runtime.NewComponentInstance(id, parentRT),
        coreInstances:  make([]api.Module, 0),
        exports:        make(map[string]*ExportedFunc),
        componentFuncs: make(map[uint32]ComponentFunc),
        parent:         parent,
    }
    return inst
}
```

### Delegated methods

All methods that previously read/wrote the deleted fields are rewritten as one-liner delegators. For example:

```go
func (i *Instance) MayLeave() bool             { return i.rt.IsMayLeave() }
func (i *Instance) SetMayLeave(allowed bool)    { i.rt.MayLeave = allowed }
func (i *Instance) ActiveCallDepth() int        { return i.rt.EnterCount() }
func (i *Instance) EnterCall() {
    i.rt.Enter()
    i.rt.Reentrance.EnterInstance(i.rt.ID)
}
func (i *Instance) ExitCall() {
    i.rt.Reentrance.LeaveInstance(i.rt.ID)
    i.rt.Leave()
}
func (i *Instance) Table() *runtime.Table       { return i.rt.Table }
func (i *Instance) Parent() *Instance           { return i.parent }
// CallMightBeRecursive — see Decision 3's "CallMightBeRecursive transitive
// ancestor check" above. This is a structural reflexive-ancestor walk via
// isReflexiveAncestor; it must NOT consult ReentranceTracker (that tracker
// serves the separate concurrency trap at definitions.py:3664-3667, a
// different spec check). B4 corrective (commit b74f5558) removed an
// earlier draft that delegated to the tracker.
func (i *Instance) CallMightBeRecursive(caller *Instance) bool {
    if i == nil || caller == nil {
        return false
    }
    return isReflexiveAncestor(caller, i) || isReflexiveAncestor(i, caller)
}
```

`ValidateMayLeave` and `ValidateNotRecursive` keep their existing shape, delegating through the new accessors. The `EnterCall`/`ExitCall` bookkeeping of `Reentrance.EnterInstance`/`LeaveInstance` tracks runtime-stack membership for the Session 2 concurrency trap at `definitions.py:3664-3667`; `CallMightBeRecursive`'s structural walk is a separate spec path (`definitions.py:290-299`).

## Instantiate Pipeline — Top-Level Shape

`ComponentLinker.Instantiate` is rebuilt end-to-end. The top-level shape (abbreviated; full plumbing in the implementation plan):

```go
func (l *ComponentLinker) Instantiate(ctx context.Context, compiled *CompiledComponent) (*Instance, error) {
    c := compiled.Internal()
    compiledModules := compiled.CompiledModules()

    // 1. Allocate instance + runtime.ComponentInstance.
    inst := newInstance(c, l.nextInstanceID(), nil)

    // 2. Bind resource type declarations to runtime identities. Matches
    //    wasmtime Instantiator::resource at instance.rs:912-931 — mint
    //    *runtime.ResourceType per declared resource and populate
    //    inst.rt.ResourceTypes. Decision 2.
    if err := l.bindResourceTypes(inst, c); err != nil {
        return nil, fmt.Errorf("bind resource types: %w", err)
    }

    // 3. Build index spaces from aliases (funcSpace, memSpace) — unchanged
    //    logic from pre-Session-0, retargeted to new types.
    funcSpace := NewCoreFuncIndexSpace()
    memSpace := NewCoreMemoryIndexSpace()
    l.buildCoreIndexSpaces(c, funcSpace, memSpace)

    // 4. Validate imports + type check + build resolvedImports + instanceToImport map.
    tc := NewTypeChecker(c)
    resolvedImports := make(map[string]Definition)
    instanceToImport := make(map[uint32]string)
    if err := l.resolveAndCheckImports(c, tc, resolvedImports, instanceToImport); err != nil {
        return nil, err
    }

    // 5. Populate value index space from value imports.
    l.populateValueImports(inst, c, resolvedImports)

    // 6. Align instance index space with instance imports (placeholder slots).
    l.alignInstanceImports(inst, c)

    // 7. Build component function index space (canon lift + aliases +
    //    resolved imports). Uses c.TypeDefs to resolve canon.TypeIdx.
    l.buildComponentFuncs(inst, c, resolvedImports, instanceToImport)

    // 8. Build type index space for nested instantiation argument resolution.
    l.buildTypeSpace(inst, c)

    // 9. Process nested component instances — instantiateNestedComponent,
    //    componentInstDefs bookkeeping.
    componentInstDefs, err := l.processNestedInstances(ctx, inst, c)
    if err != nil {
        return nil, err
    }

    // 10. Build canon lower / canon resource info maps from CanonicalDef
    //     entries (keyed by the assigned core function index).
    canonLowers, canonResources := l.buildCanonMaps(c)

    // 11. Build function alias map for inline instance resolution.
    funcAliases := l.buildFuncAliases(c)

    // 12. Instantiate core modules. Wires canon.lower + canon.resource.*
    //     host module exports from the inline-instance shortcut path.
    if err := l.instantiateCoreModules(ctx, inst, c, compiledModules,
        resolvedImports, canonLowers, canonResources, funcAliases); err != nil {
        return nil, err
    }

    // 13. Execute start function if defined.
    if err := l.executeStartFunction(ctx, inst, c); err != nil {
        return nil, fmt.Errorf("start function: %w", err)
    }

    // 14. Wire exports — one ExportedFunc per exported function, one
    //     Instance per exported instance.
    if err := l.wireExports(inst, c, componentInstDefs, funcSpace, memSpace); err != nil {
        return nil, err
    }

    return inst, nil
}
```

Each numbered sub-helper gets its own task in the implementation plan. The pipeline structure mirrors the pre-Session-0 `Instantiate` but each helper is rewritten against the new types.

### canon.lift wiring

`canon.lift` creates a component function from a core wasm function. The closure below implements every spec-required step of `canon_lift` at `definitions.py:1978-2040` that applies to the synchronous sub-session (Task/Thread machinery is a no-op for sync calls; async Task/Thread is a Later-bucket concern).

```go
// buildCanonLiftFunc creates the closure that implements a canon.lift
// component function. Matches spec canon_lift at definitions.py:1978-2040
// for synchronous calls.
//
// Step-by-step spec mapping:
//   :1979  trap_if(call_might_be_recursive(caller, inst))
//   :1989  args = on_start()
//   :1990  flat_args = lower_flat_values(cx, MAX_FLAT_PARAMS, args, ft.param_types())
//            (lower_flat_values toggles may_leave False/True internally at :1955/:1973)
//   :1995  flat_results = call_and_trap_on_throw(callee, thread, flat_args)
//   :1997  result = lift_flat_values(cx, MAX_FLAT_RESULTS, iter(flat_results), ft.result_type())
//   :1998  task.return_(result)
//   :2000  inst.may_leave = False
//   :2001  [] = call_and_trap_on_throw(opts.post_return, thread, flat_results)
//   :2002  inst.may_leave = True
//          deliver_resolve() -- borrow scope cleanup
func (l *ComponentLinker) buildCanonLiftFunc(
    inst *Instance,
    canon *CanonicalDef,
    coreFunc api.Function,
    funcType *types.TypeFunc,
    memory api.Memory,
    realloc api.Function,
) func(goCtx context.Context, caller *Instance, args []types.Val) ([]types.Val, error) {
    return func(goCtx context.Context, caller *Instance, args []types.Val) ([]types.Val, error) {
        // Spec :1979 — trap_if(call_might_be_recursive(caller, inst)).
        if caller != nil && inst.CallMightBeRecursive(caller) {
            return nil, errReentrance
        }
        // Mark the instance as on the call stack for transitive reentrance detection.
        inst.rt.Reentrance.EnterInstance(inst.rt.ID)
        defer inst.rt.Reentrance.LeaveInstance(inst.rt.ID)

        // CallContext is per-call state (the canon_lower/canon_lift subtask
        // equivalent). Allocated fresh per call, not stored on Instance.
        callCtx := runtime.NewCallContext()
        opts := buildOptions(canon, memory, realloc)
        lowerCtx := &abi.LowerContext{
            Memory:      memory,
            Opts:        opts,
            Realloc:     reallocAdapter(realloc),
            Types:       inst.component.Types,
            Instance:    inst.rt,
            CallContext: callCtx,
        }
        liftCtx := &abi.LiftContext{
            Memory:      memory,
            Opts:        opts,
            Types:       inst.component.Types,
            Instance:    inst.rt,
            BorrowScope: runtime.NewBorrowScope(inst.rt.Table),
        }

        paramTypes := unpackTupleElems(inst.component.Types, funcType.Params)
        resultTypes := unpackTupleElems(inst.component.Types, funcType.Results)

        // Spec :1990 — lower_flat_values(cx, MAX_FLAT_PARAMS, args, ft.param_types()).
        // The spec function toggles may_leave = False at :1955 before any
        // realloc call, restores at :1973. We mirror that via a single
        // LowerParams entry point on abi/ that encapsulates the aggregate
        // boundary decision (flat vs indirect) AND the may_leave toggle.
        inst.rt.MayLeave = false
        flatArgs, err := abi.LowerParams(lowerCtx, paramTypes, args, abi.MaxFlatParams)
        inst.rt.MayLeave = true
        if err != nil {
            return nil, fmt.Errorf("canon.lift: lower params: %w", err)
        }

        // Spec :1995 — call the core wasm callee with the flat args.
        flatResults, err := coreFunc.Call(goCtx, flatArgs...)
        if err != nil {
            return nil, fmt.Errorf("canon.lift: core call: %w", err)
        }

        // Spec :1997 — lift_flat_values(cx, MAX_FLAT_RESULTS, iter(flat_results), ft.result_type()).
        // Aggregate boundary: if the flattened result width > MAX_FLAT_RESULTS,
        // the core function actually returned a retptr (i32 pointer into memory)
        // as its single return value. abi.LiftResults encapsulates this.
        results, err := abi.LiftResults(liftCtx, resultTypes, flatResults, abi.MaxFlatResults)
        if err != nil {
            return nil, fmt.Errorf("canon.lift: lift results: %w", err)
        }

        // Spec :2000-2002 — may_leave = False around post_return; may_leave = True after.
        if postReturn := canonPostReturn(inst, canon); postReturn != nil {
            inst.rt.MayLeave = false
            _, perr := postReturn.Call(goCtx, flatResults...)
            inst.rt.MayLeave = true
            if perr != nil {
                return nil, fmt.Errorf("canon.lift: post_return: %w", perr)
            }
        }

        // Spec deliver_resolve at :738-742 — close the borrow scope, undoing
        // every outstanding lend and trapping if a borrow is still held.
        if err := liftCtx.BorrowScope.Close(); err != nil {
            return nil, fmt.Errorf("canon.lift: borrow scope: %w", err)
        }

        return results, nil
    }
}
```

`abi.LowerParams(ctx, paramTypes, args, maxFlat)` is a new entry point on `abi/` that implements spec `lower_flat_values` at `definitions.py:1943-1975` including the aggregate `MAX_FLAT_PARAMS` check:

1. Compute `flat_types := FlattenParams(ctx.Types, paramTypes)`.
2. If `len(flat_types) > maxFlat`: allocate a memory buffer via `ctx.Realloc`, write each arg to memory via `LowerHeap`, return `[]uint64{uint64(bufferPtr)}`.
3. Otherwise: for each arg, call `LowerFlat` and collect into a single `[]uint64`.

`abi.LiftResults(ctx, resultTypes, flatResults, maxFlat)` is the mirror: if the core function returned more than `maxFlat` values, `flatResults[0]` is a retptr and the rest are empty; lift from memory. Otherwise, lift each result from the flat iterator.

Both helpers are introduced in checkpoint C and subsume the per-param / per-result loops of the earlier design draft. They replace the boundary-unsafe iteration that the chunk-9 reviewer flagged.

`unpackTupleElems(c.Types, tupleValType)` resolves a `ValType{Kind: TypeKindTuple, Index: N}` to the `[]ValType` of the tuple's element types via `c.Tuples[N].Types`. Returns empty slice for nil / non-tuple inputs.

`buildOptions(canon, memory, realloc)` constructs an `abi.Options` struct from `canon.Options`. `canonPostReturn` looks up the post-return function if `canon.Options.PostReturnIdx != nil`.

`errReentrance` is the sentinel for spec `:1979` `trap_if(call_might_be_recursive(...))`. It matches the existing wazero reentrance error but now fires from the spec-correct transitive check (Decision 3 amendment using `runtime.ReentranceTracker`).

### canon.lower wiring

`canon.lower` creates a core wasm function from a component function. Matches spec `canon_lower` at `definitions.py:2064-2130`.

```go
// createCanonLowerFunc produces the api.GoModuleFunc that implements a
// canon.lower core function. Matches spec canon_lower at definitions.py:2064-2130.
//
// Step-by-step spec mapping:
//   :2065  trap_if(not caller_task.inst.may_leave)
//   :2068  subtask = Subtask()
//   :2070  cx = LiftLowerContext(opts, caller_task.inst, borrow_scope=subtask)
//   :2089  args = on_start() -- for canon_lower from core wasm, on_start()
//          lifts args from flat iter.
//   :2095  result = callee(args)  -- invoke the component function
//   :2113  deliver_resolve()  -- close borrow scope, undo lends
func (l *ComponentLinker) createCanonLowerFunc(
    ctx context.Context,
    inst *Instance,
    c *Component,
    info canonLowerInfo,
    compFunc ComponentFunc,
    paramTypes []types.ValType,
    resultTypes []types.ValType,
    needsRetptr bool,
) api.GoModuleFunc {
    return api.GoModuleFunc(func(goCtx context.Context, mod api.Module, stack []uint64) {
        memory := mod.Memory()
        realloc := resolveRealloc(mod, info.options)

        // Spec :2065 — trap_if(not caller_task.inst.may_leave).
        if !inst.rt.IsMayLeave() {
            panic(errMayNotLeave)
        }

        // Spec :2068-2070 — create the Subtask-equivalent (a fresh
        // BorrowScope owning the lender set for this call) and the
        // LiftLowerContext.
        opts := buildOptions(info.options, memory, realloc)
        borrowScope := runtime.NewBorrowScope(inst.rt.Table)
        liftCtx := &abi.LiftContext{
            Memory:      memory,
            Opts:        opts,
            Types:       c.Types,
            Instance:    inst.rt,
            BorrowScope: borrowScope,
        }
        lowerCtx := &abi.LowerContext{
            Memory:      memory,
            Opts:        opts,
            Realloc:     reallocAdapter(realloc),
            Types:       c.Types,
            Instance:    inst.rt,
            CallContext: runtime.NewCallContext(),
        }

        // Spec :2089 — lift args from the flat iterator. abi.LiftParams
        // encapsulates the MAX_FLAT_PARAMS aggregate boundary decision:
        // if len(flat_types) > MAX_FLAT_PARAMS the first flat value is a
        // retptr into memory and remaining lift operations read from that
        // memory region. Otherwise the flat iterator is consumed directly.
        args, err := abi.LiftParams(liftCtx, paramTypes, stack, abi.MaxFlatParams)
        if err != nil {
            panic(fmt.Errorf("canon.lower: lift params: %w", err))
        }

        // Spec :2095 — invoke the callee (the component function).
        results, err := compFunc.Impl(goCtx, args)
        if err != nil {
            panic(fmt.Errorf("canon.lower: callee: %w", err))
        }

        // Lower results back into the stack or memory, toggling may_leave
        // per spec :1955/:1973 (the lowering path runs inside an implicit
        // lower_flat_values; the toggle guards realloc calls).
        inst.rt.MayLeave = false
        err = abi.LowerResults(lowerCtx, resultTypes, results, stack, needsRetptr, abi.MaxFlatResults)
        inst.rt.MayLeave = true
        if err != nil {
            panic(fmt.Errorf("canon.lower: lower results: %w", err))
        }

        // Spec :2113 — deliver_resolve: close the borrow scope, decrementing
        // every outstanding lend and trapping if a borrow is still held.
        if err := borrowScope.Close(); err != nil {
            panic(fmt.Errorf("canon.lower: borrow scope: %w", err))
        }
    })
}
```

`abi.LiftParams(ctx, paramTypes, stack, maxFlat)` is the mirror of `LowerParams`: it computes `FlattenParams(ctx.Types, paramTypes)` and either reads from a retptr in `stack[0]` (if the flat width exceeded `maxFlat`) or iterates `stack` directly. `abi.LowerResults(ctx, resultTypes, results, stack, needsRetptr, maxFlat)` writes either to memory via `stack[len(paramTypes)]` (retptr) or into `stack[0..n]` for the flat path.

Both helpers subsume the per-param / per-result iteration and make the aggregate boundary decision exactly once per call, matching spec `definitions.py:1943-1975` `lower_flat_values` and `:1977-1993` `lift_flat_values`.

### Nested component instantiation

`instantiateNestedComponent` is restored to rebuild a nested `*Instance` with its own `*runtime.ComponentInstance` (parent pointing at the outer instance's `rt`). Argument resolution via `resolveFromParentScope` walks the parent's component function / instance / type / component spaces, unchanged in structure from the pre-Session-0 body but retargeted to the new types. `resolveExportTypeAlias` (the Session 0 panic stub) is rebuilt to walk `parent.component.TypeDefs` + the alias's source instance's exports, returning a `*TypeDef` that points into the canonical bag.

## Resource Identity — Binding Declarations to Runtime Types

This section's previous title was "Local Concrete Promotion" tied to Session 0's `TypeResourceTable.Concrete` state model. Session 1's approach (Decision 2) does not mutate `Concrete` — it binds declarations directly to `*runtime.ResourceType` pointers in the instance's `ResourceTypes` pool. The renamed `bindResourceTypes` function mirrors wasmtime's `Instantiator::resource` at `instance.rs:912-931` which creates `ResourceType::guest(store_id, instance, resource.index)` and pushes it into `instance_resource_types`. No "promotion" — direct binding per declaration.

Session 1 populates `inst.rt.ResourceTypes` at `Instantiate` time. The walk is driven by `compiled.Types.ResourceTables`:

```go
func (l *ComponentLinker) bindResourceTypes(inst *Instance, c *Component) error {
    for rtIdx, table := range c.Types.ResourceTables {
        if table.Concrete {
            // Already concrete (e.g., ResourceTable describes an imported
            // resource from a host module). Session 1 does not overwrite.
            continue
        }

        // Locate the resource's destructor info. The decoder stored
        // destructor metadata in the TypeDef entry for the slot that
        // declared the resource. Walk c.TypeDefs to find the one whose
        // Resource field equals rtIdx.
        var dtor *uint32
        var dtorAsync bool
        var dtorCallback *uint32
        for _, td := range c.TypeDefs {
            if td.Kind == TypeDefKindResource && td.Resource == types.ResourceTableIdx(rtIdx) {
                // The ResourceTypeDef (stored in the decoder) has Dtor /
                // DtorAsync / DtorCallback. Session 1 extends TypeDef to
                // carry them directly, since the decoder already has
                // them per slot.
                dtor = td.ResourceDtor
                dtorAsync = td.ResourceDtorAsync
                dtorCallback = td.ResourceDtorCallback
                break
            }
        }

        rt := &runtime.ResourceType{
            Impl:         inst.rt,
            Dtor:         dtor,
            DtorAsync:    dtorAsync,
            DtorCallback: dtorCallback,
            // HostDestructor is nil for guest-declared resources. Host
            // resources (wasip2 io streams etc.) set HostDestructor when
            // they construct their own *ResourceType singletons in
            // imports/wasip2/*/...; bindResourceTypes does not touch those.
            HostDestructor: nil,
        }
        inst.rt.ResourceTypes = append(inst.rt.ResourceTypes, rt)
    }
    return nil
}
```

Matches wasmtime's `Instantiator::resource` at `runtime/component/instance.rs:912-931`: walk per-resource declarations, create a `ResourceType::guest(store_id, instance, resource.index)`-equivalent (wazero's pointer-identity `*runtime.ResourceType{Impl: inst.rt, Dtor: ...}`), push into `instance_resource_types` (wazero's `inst.rt.ResourceTypes`). No Abstract/Concrete state machine — direct binding per declaration.

`TypeDef` gains three new fields for resource declarations plus the
`Alias *AliasTarget` variant (Decision 5 densification):

```go
type TypeDef struct {
    Kind     TypeDefKind
    Func     types.FuncTypeIdx   // Decision 5 — changed from *types.TypeFunc
    Resource types.ResourceTableIdx
    ValType  types.ValType
    Instance  *InstanceTypeDef
    Component *ComponentTypeDef

    // Alias carries the unresolved target of a type alias when
    // Kind == TypeDefKindAlias. Populated by binary/alias.go at
    // decode time; resolved at use time via Component.ResolveTypeDef.
    // Spec: Binary.md:118-126 aliastarget grammar.
    Alias *AliasTarget

    // Resource destructor metadata (populated for Kind==TypeDefKindResource).
    // Session 1 adds these so bindResourceTypes can wire Dtor without
    // a second pass over the decoder state.
    ResourceDtor         *uint32
    ResourceDtorAsync    bool
    ResourceDtorCallback *uint32
}
```

### Mapping TypeResourceTable.Resource to ResourceTypes slice index

In the canonical bag, `TypeResourceTable` has two modes (Concrete vs Abstract — design doc Decision 5 in Session 0). Session 1 uses only the Abstract mode at decode time and maps it to a concrete instance-owned `*ResourceType` via a simple convention:

**Convention:** `TypeResourceTable.Resource` in the Abstract entry is the resource's declaration-order index within the component. `inst.rt.ResourceTypes` is populated in the same declaration order by `bindResourceTypes`, so `inst.rt.ResourceTypes[TypeResourceTable.Resource]` is the matching `*ResourceType`.

**Verification:** the canonical bag's `ResourceTables` slice is populated by `builder.InternAbstractResource()` (one call per resource declaration in scope order), and `bindResourceTypes` walks the same slice in the same order. Order parity is maintained by construction.

### Lift dispatch adjustment

`abi/lift.go:liftOwnHandle` / `liftBorrowHandle` stop trapping on `!rt.Concrete`. Instead they rely on `ctx.Instance.LookupResourceType(rt.Instance, rt.Resource)` returning non-nil. In Session 1:

- `rt.Instance` is `types.RuntimeComponentInstanceIdx(ctx.Instance.ID)` for the same-instance case — the `Abstract` entry's `Instance` field is populated when `bindResourceTypes` runs (or left zero; the `LookupResourceType` call walks by ID, and for a same-instance call the ID matches).
- For cross-instance resource handles (declared in a different instance), `LookupResourceType` returns nil (Session 1 does not maintain a cross-instance registry) and the lift path traps with the precise error.

The `!rt.Concrete` trap check is removed from `liftOwnHandle` and `liftBorrowHandle`. The trap branch moves into the `expectedRT == nil` check which now also catches the cross-instance case.

## Decoder → Linker Indirection — `Component.TypeDefs`

Full decoder change: every `decodeTypeSection` case that previously populated `dc.funcTypeIdx` or `dc.resourceDefs` instead (or also) appends a `TypeDef` to `dc.c.TypeDefs`. The private maps are deleted. Additionally, every outer/export type alias in `binary/alias.go` appends a `TypeDefKindAlias` entry alongside its `NextTypeIdx++` bump — the densification guarantee that `len(c.TypeDefs) == c.NextTypeIdx` across the full decode.

For resource declarations, `decodeResourceDecl` returns a `ResourceTypeDef` with destructor metadata; the decoder extracts `Dtor`, `DtorAsync`, `DtorCallback` and stores them on the `TypeDef` (Decision 2 adjustment).

For function declarations, the decoder stores `types.FuncTypeIdx` directly (not a pointer into the bag; Decision 5 option A).

For value types (records, variants, lists, etc.), the decoder stores the `types.ValType` returned by the matching `decode<Kind>` helper.

For instance / component type declarations, the decoder stores a `*InstanceTypeDef` / `*ComponentTypeDef` pointer (the existing shape from pre-Session-0).

For type aliases, the decoder stores a `TypeDefKindAlias` entry with a populated `Alias *AliasTarget` carrying the unresolved outer (`OuterCount` + `OuterIndex`) or export (`InstanceIdx` + `ExportName`) target. Resolution happens at use time via `Component.ResolveTypeDef`, which walks alias chains until it hits a concrete kind — mirror of wasmparser's `Validator.component_any_type_at(typeidx)` (`crates/environ/src/component/translate.rs:796-801`).

All callers:

- `component_linker.go::Instantiate` and its helpers — resolve `canon.TypeIdx` via `slot, _, err := c.ResolveTypeDef(canon.TypeIdx)` for canon operations. Direct `c.TypeDefs[canon.TypeIdx]` indexing is only valid when the caller has already proven the slot is not an alias.
- `type_checker.go::checkFuncDefinition` / `checkInstanceDefinition` — use `c.ResolveTypeDef(expected.TypeIdx)` for expected-type lookup.
- `instance.go::ResourceNew`/`ResourceRep`/`ResourceDrop` — take `types.ResourceIdx` directly; resolution via `c.ResolveTypeDef` happens in the wiring layer (`createResourceOpExport`).
- `nested_component.go::resolveExportTypeAlias` — walks `parent.component.TypeDefs` to resolve a type alias export. Cross-scope outer aliases (`OuterCount > 0`) and export aliases are deferred to this wiring-layer helper rather than resolved by `ResolveTypeDef`.
- Test code in `component_linker_test.go`, `linker_test.go`, `nested_component_test.go`, etc. — test fixtures build small Components with hand-populated `TypeDefs` slices.

## Canon Resource Ops — Host Module Export Shapes

`createResourceNewExport`, `createResourceDropExport`, `createResourceRepExport` are restored with the Session 1 signatures (Decision 7). Core wasm signatures (unchanged from the spec):

- `canon resource.new $T` : `(i32 rep) -> i32 handle`
- `canon resource.drop $T` : `(i32 handle) -> ()`
- `canon resource.rep $T` : `(i32 handle) -> i32 rep`

Each host module export is keyed by the resource's declaration index, resolved once at host-module creation time from `c.TypeDefs[canon.TypeIdx].Resource`. The closure captures the resolved `types.ResourceIdx` so the runtime call is index-free.

Error handling: traps at the Wasm level. The Go body panics with a typed error that the core-wasm host-function adapter converts to a trap. This matches the pre-Session-0 pattern for other canon ops and wazero's `wasmruntime.Trap*` conventions.

## `abi/lift.go` Gap Fixes — Concrete Patch Sites

All four gaps are fixed by rewriting `liftOwnHandle` and `liftBorrowHandle` in place and adding `Table.GetByIndex`. The order of operations follows wasmtime's `resource_lift_own` at `runtime/vm/component/resources.rs:275-279` (check-then-remove), which is observationally equivalent to the spec's `remove-then-check` at `definitions.py:1334-1339` post-trap but avoids consuming a handle on a failed lift.

**`liftOwnHandle` full rewrite:**

```go
// liftOwnHandle implements TypeKindOwn lift per definitions.py:1333-1339.
//
// Spec:
//   def lift_own(cx, i, t):
//     h = cx.inst.table.remove(i)         # :1334
//     trap_if(not isinstance(h, ResourceHandle))
//     trap_if(h.rt is not t.rt)           # :1336
//     trap_if(h.num_lends != 0)           # :1337
//     trap_if(not h.own)                  # :1338
//     return h.rep                        # :1339
//
// Wasmtime parallel: resources.rs:275-279 resource_lift_own.
func liftOwnHandle(ctx *LiftContext, typ types.ValType, handleIdx uint32) (types.Val, error) {
    if ctx == nil || ctx.Instance == nil {
        return types.Val{}, fmt.Errorf("lift own: no component instance available")
    }
    if ctx.Types == nil {
        return types.Val{}, fmt.Errorf("lift own: no component types available")
    }
    if int(typ.Index) >= len(ctx.Types.ResourceTables) {
        return types.Val{}, fmt.Errorf("lift own: resource table index %d out of range", typ.Index)
    }
    rt := ctx.Types.ResourceTables[typ.Index]
    expectedRT := ctx.Instance.LookupResourceType(rt.Instance, rt.Resource)
    if expectedRT == nil {
        return types.Val{}, fmt.Errorf(
            "lift own: no resource type for instance %d declaration %d "+
                "(cross-instance resolution: session 2 wiring)",
            rt.Instance, rt.Resource)
    }

    // Gap 3 fix: GetByIndex bridges Wasm-side u32 to runtime 64-bit generation-tagged Handle.
    h, entry, err := ctx.Instance.Table.GetByIndex(handleIdx)
    if err != nil {
        return types.Val{}, fmt.Errorf("lift own: %w", err)
    }
    resEntry, ok := entry.(*runtime.ResourceHandleEntry)
    if !ok {
        return types.Val{}, fmt.Errorf("lift own: handle %d is not a resource handle", handleIdx)
    }
    // Spec :1336 — trap_if(h.rt is not t.rt). Pointer identity.
    if resEntry.RT != expectedRT {
        return types.Val{}, fmt.Errorf("lift own: resource type mismatch")
    }
    // Spec :1337 — trap_if(h.num_lends != 0). Gap 2 fix.
    if resEntry.NumLends != 0 {
        return types.Val{}, fmt.Errorf("lift own: handle has %d outstanding lends", resEntry.NumLends)
    }
    // Spec :1338 — trap_if(not h.own). Gap 1 fix.
    if !resEntry.Own {
        return types.Val{}, fmt.Errorf("lift own: handle %d is a borrow, not an own", handleIdx)
    }
    // All checks passed — remove and return rep.
    if _, err := ctx.Instance.Table.Remove(h); err != nil {
        return types.Val{}, fmt.Errorf("lift own: %w", err)
    }
    // Spec :1339 — return h.rep. Gap 4 fix: Rep is now uint32 (Decision 4).
    return types.ValOwn(resEntry.Rep), nil
}
```

**`liftBorrowHandle` full rewrite:**

```go
// liftBorrowHandle implements TypeKindBorrow lift per definitions.py:1341-1347.
//
// Spec:
//   def lift_borrow(cx, i, t):
//     assert(isinstance(cx.borrow_scope, Subtask))   # :1342
//     h = cx.inst.table.get(i)                       # :1343
//     trap_if(not isinstance(h, ResourceHandle))
//     trap_if(h.rt is not t.rt)                      # :1345
//     cx.borrow_scope.add_lender(h)                  # :1346
//     return h.rep                                   # :1347
//
// Wasmtime parallel: resources.rs:291-297 resource_lift_borrow + scope.lenders.push.
func liftBorrowHandle(ctx *LiftContext, typ types.ValType, handleIdx uint32) (types.Val, error) {
    if ctx == nil || ctx.Instance == nil {
        return types.Val{}, fmt.Errorf("lift borrow: no component instance available")
    }
    if ctx.Types == nil {
        return types.Val{}, fmt.Errorf("lift borrow: no component types available")
    }
    // Spec :1342 — assert(isinstance(cx.borrow_scope, Subtask)). wazero
    // equivalent: BorrowScope must be non-nil during any borrow lift. This
    // is a precondition of the call; a nil BorrowScope here is a bug in
    // the caller (canon.lift / canon.lower must construct one).
    if ctx.BorrowScope == nil {
        return types.Val{}, fmt.Errorf("lift borrow: no borrow scope active (bug: lift closure must construct one)")
    }
    if int(typ.Index) >= len(ctx.Types.ResourceTables) {
        return types.Val{}, fmt.Errorf("lift borrow: resource table index %d out of range", typ.Index)
    }
    rt := ctx.Types.ResourceTables[typ.Index]
    expectedRT := ctx.Instance.LookupResourceType(rt.Instance, rt.Resource)
    if expectedRT == nil {
        return types.Val{}, fmt.Errorf(
            "lift borrow: no resource type for instance %d declaration %d "+
                "(cross-instance resolution: session 2 wiring)",
            rt.Instance, rt.Resource)
    }

    // Gap 3 fix: GetByIndex for generation bridging.
    h, entry, err := ctx.Instance.Table.GetByIndex(handleIdx)
    if err != nil {
        return types.Val{}, fmt.Errorf("lift borrow: %w", err)
    }
    resEntry, ok := entry.(*runtime.ResourceHandleEntry)
    if !ok {
        return types.Val{}, fmt.Errorf("lift borrow: handle %d is not a resource handle", handleIdx)
    }
    // Spec :1345 — trap_if(h.rt is not t.rt).
    if resEntry.RT != expectedRT {
        return types.Val{}, fmt.Errorf("lift borrow: resource type mismatch")
    }
    // Spec :1346 — cx.borrow_scope.add_lender(h). Increments h.num_lends
    // and records the lender in the borrow scope for later cleanup
    // (deliver_resolve at spec :738-742).
    if err := ctx.Instance.Table.IncrementLends(h); err != nil {
        return types.Val{}, fmt.Errorf("lift borrow: %w", err)
    }
    if err := ctx.BorrowScope.AddLender(h); err != nil {
        return types.Val{}, fmt.Errorf("lift borrow: %w", err)
    }
    // Spec :1347 — return h.rep. Gap 4 fix: Rep is now uint32.
    return types.ValBorrow(resEntry.Rep), nil
}
```

**`Table.GetByIndex` new method on `runtime/table.go`:**

```go
// GetByIndex looks up a table entry by slot index (the low 32 bits of a
// generation-tagged Handle). Returns the current generation-tagged
// Handle alongside the entry, so the caller can perform further
// operations (Remove, IncrementLends) using the full Handle.
//
// Used by abi/lift.go and abi/lower.go to bridge the 32-bit Wasm-side
// handle index to the runtime's 64-bit generation-tagged handle space.
// Every call site that receives a raw u32 handle from Wasm memory
// (liftOwnHandle, liftBorrowHandle, lowerOwnHandleFlat, lowerBorrowHandleFlat,
// Instance.ResourceNew/Rep/Drop) calls GetByIndex.
//
// Returns ErrInvalidHandle if the slot is out of range or currently free.
func (t *Table) GetByIndex(idx uint32) (Handle, TableEntry, error) {
    if int(idx) >= len(t.entries) {
        return 0, nil, ErrInvalidHandle
    }
    slot := &t.entries[idx]
    if slot.free || slot.entry == nil {
        return 0, nil, ErrInvalidHandle
    }
    h := makeHandle(idx, slot.generation)
    return h, slot.entry, nil
}
```

(Exact field names depend on `runtime/table.go`'s current slot layout; the contract is "idx (Wasm-side u32) → (full Handle, entry, nil) | (0, nil, ErrInvalidHandle)".)

**`types.ValOwn` / `types.ValBorrow` — no signature change.** Both constructors already accept a `uint32` and store it as the payload. The payload's semantics change from "Wasm-side handle index by convention" to "rep per spec definitions.py:1339/:1347, which equals the u32 stored in `ResourceHandleEntry.Rep` under Gap 4 fix." A comment block is added to each constructor citing the spec line and Gap 4.

## Type Checker Scope — Session 1

`checkFuncType(expected, actual *types.TypeFunc)` at `type_checker.go:42–58` compares via identity on the three spec-relevant fields only:

```go
// Spec: definitions.py:88-101 FuncType's param_types() and result_type()
// strip parameter NAMES and return only ValTypes. Parameter names are
// metadata, not part of type equivalence. Wasmtime's matching.rs also
// does not compare param names.
func (tc *TypeChecker) checkFuncType(expected, actual *types.TypeFunc) error {
    if expected == nil || actual == nil {
        if expected != actual {
            return fmt.Errorf("function type mismatch: one side is nil")
        }
        return nil
    }
    if expected.Async != actual.Async {
        return fmt.Errorf("function async mismatch: expected %v, got %v", expected.Async, actual.Async)
    }
    // Same-bag identity on the interned tuple indices. Both sides share
    // tc.component.Types (cross-bag structural walk is Session 2).
    if expected.Params != actual.Params {
        return fmt.Errorf("function params mismatch")
    }
    if expected.Results != actual.Results {
        return fmt.Errorf("function results mismatch")
    }
    return nil
}
```

**No ParamNames comparison.** The original design doc draft added `ParamNames` element-wise comparison; the chunk-6 reviewer flagged this as over-specification. Spec `definitions.py:88-101` has `param_types()` that returns only ValTypes (names stripped). Wasmtime's `matching.rs` typecheck uses `InterfaceType::Tuple(ty.params)` / `InterfaceType::Tuple(ty.results)` with no name inspection. Session 1 matches both.

`checkFuncDefinition(expected *ImportExternDesc, actual Definition) error` at `type_checker.go:187–194` is rewritten to require a typed host `FuncDef`. Session 1 does NOT trust untyped host functions:

```go
// Session 1 contract: every FuncDef passed through the linker MUST carry
// a non-nil Type. The ComponentLinker.DefineFunc signature is changed
// below to require *types.TypeFunc at call time. This matches wasmtime's
// matching.rs:51 which bails with "function implementation is missing"
// when actual is None.
func (tc *TypeChecker) checkFuncDefinition(expected *ImportExternDesc, actual Definition) error {
    fd, ok := actual.(*FuncDef)
    if !ok {
        return fmt.Errorf("import: expected function, got %T", actual)
    }
    if fd.Type == nil {
        return fmt.Errorf("import: host function has no type (DefineFunc must be called with a *types.TypeFunc)")
    }
    // Resolve the expected type via c.TypeDefs (Decision 5).
    if int(expected.TypeIdx) >= len(tc.component.TypeDefs) {
        return fmt.Errorf("import: type index %d out of range", expected.TypeIdx)
    }
    expectedTd := &tc.component.TypeDefs[expected.TypeIdx]
    if expectedTd.Kind != TypeDefKindFunc {
        return fmt.Errorf("import: type %d is not a function type", expected.TypeIdx)
    }
    expectedFT := &tc.component.Types.Funcs[expectedTd.Func]
    return tc.checkFuncType(expectedFT, fd.Type)
}
```

`ComponentLinker.DefineFunc` and related helpers get an updated signature taking `*types.TypeFunc`:

```go
// BEFORE (Session 0):
// func (l *ComponentLinker) DefineFunc(namespace, name string, fn HostFunc) error

// AFTER (Session 1): every host function must declare its type at registration.
func (l *ComponentLinker) DefineFunc(namespace, name string, typ *types.TypeFunc, fn HostFunc) error
```

Every `DefineFunc` call site in `imports/wasip2/...` updates to pass a `*types.TypeFunc` constructed via a package-level `ComponentTypesBuilder`. This is the same migration that the Decision 4 Rep-as-u32 refactor applies to the wasip2 io/filesystem/sockets/http/cli/clocks modules — both touch the same files.

`checkInstanceDefinition(expected *ImportExternDesc, actual Definition) error` at `type_checker.go:201–208` is rewritten to recursively type-check each declared export of the expected instance type against the corresponding actual export:

```go
// Session 1 contract: instance subtyping per Explainer.md is width-
// covariant (actual may have more exports than expected) and each
// matching export must recursively type-check. Spec: Explainer.md
// :920-982. Wasmtime parallel: matching.rs:162 self.definition(expected, actual).
func (tc *TypeChecker) checkInstanceDefinition(expected *ImportExternDesc, actual Definition) error {
    id, ok := actual.(*InstanceDef)
    if !ok {
        return fmt.Errorf("import: expected instance, got %T", actual)
    }
    if id.SkipValidation {
        return nil
    }
    if int(expected.TypeIdx) >= len(tc.component.TypeDefs) {
        return fmt.Errorf("import: instance type index %d out of range", expected.TypeIdx)
    }
    expectedTd := &tc.component.TypeDefs[expected.TypeIdx]
    if expectedTd.Kind != TypeDefKindInstance || expectedTd.Instance == nil {
        return fmt.Errorf("import: type %d is not an instance type", expected.TypeIdx)
    }
    // Walk each required export and recursively type-check.
    for _, decl := range expectedTd.Instance.Declarations {
        if decl.Kind != InstanceDeclKindExport || decl.Export == nil {
            continue
        }
        name := decl.Export.Name
        actualExport, ok := id.Exports[name]
        if !ok {
            return fmt.Errorf("import: missing required export %q", name)
        }
        // Recurse into the export's type kind.
        switch decl.Export.Kind {
        case ExportKindFunc:
            fd, ok := actualExport.(*FuncDef)
            if !ok || fd.Type == nil {
                return fmt.Errorf("import: export %q expected typed FuncDef, got %T", name, actualExport)
            }
            if decl.Export.TypeIdx == nil {
                return fmt.Errorf("import: export %q has no declared type", name)
            }
            exportTd := &tc.component.TypeDefs[*decl.Export.TypeIdx]
            if exportTd.Kind != TypeDefKindFunc {
                return fmt.Errorf("import: export %q type is not a function", name)
            }
            exportFT := &tc.component.Types.Funcs[exportTd.Func]
            if err := tc.checkFuncType(exportFT, fd.Type); err != nil {
                return fmt.Errorf("import: export %q: %w", name, err)
            }
        case ExportKindInstance:
            // Recurse into nested instance type.
            if _, ok := actualExport.(*InstanceDef); !ok {
                return fmt.Errorf("import: export %q expected instance, got %T", name, actualExport)
            }
            // Full recursive check via synthesized ImportExternDesc.
            if decl.Export.TypeIdx == nil {
                return fmt.Errorf("import: export %q has no declared type", name)
            }
            nested := &ImportExternDesc{Kind: ImportExternDescInstance, TypeIdx: *decl.Export.TypeIdx}
            if err := tc.checkInstanceDefinition(nested, actualExport); err != nil {
                return fmt.Errorf("import: export %q: %w", name, err)
            }
        case ExportKindType:
            // Type exports are metadata; nominal resource matching handled elsewhere.
        case ExportKindComponent:
            if _, ok := actualExport.(*ComponentDef); !ok {
                return fmt.Errorf("import: export %q expected component, got %T", name, actualExport)
            }
        }
    }
    return nil
}
```

Session 2 adds cross-bag structural walk (for components where the host and guest have different `*types.ComponentTypes` bags). Session 1 handles same-bag only — the case where the host constructs `*types.TypeFunc` values from a builder that shares the component's bag (typically via the linker's lazy-construction helpers in `imports/wasip2/`).

## Test Restoration Methodology

The user's hard constraint: **"each test/aspect must be compared against upstream spec/canonical implementation (and its tests)/and wasmtime (and its tests), every step of the way. If there are tests or code that shouldn't exist because they break conformance with the spec or canonical upstreams then they must be reworked or removed."** Session 1's restoration methodology enforces this at both the restorer and reviewer layers.

### Methodology steps (per test file)

For each `t.Skip("session 1 work")` test and each `TestXxxDeferredToSession1` stub the restorer follows these steps in order:

**Step 1 — Deduplication check (MANDATORY before writing anything).**

Before creating or restoring any test, grep the current repo for tests covering the same behavior. Goal: avoid "dozens of copies of the same test cases created by different agents with slightly different names."

```bash
# Example: before restoring TestExportedFuncCall_OwnArgument in instance_test.go,
# check whether any other test file already covers own-handle argument lifting.
grep -rn "own.*argument\|TestExportedFuncCall_Own\|TestLift.*Own\|own handle" internal/component/ api/ imports/
```

If an existing test already covers the behavior, the restorer extends the existing test (adding table-driven cases) rather than creating a new top-level function. If the existing test lives in the wrong package, the restorer moves it rather than duplicating it.

**Step 2 — Locate the pre-Session-0 body in git.**

```bash
git show 98b3bbc3:internal/component/instance_test.go > /tmp/old_instance_test.go
```

Read the old body as a **reference** for test intent, not a template to copy. Per the user's "I don't care about preserving existing wazero code" constraint, the old body is discardable if it doesn't match spec/canonical/wasmtime.

**Step 3 — Identify the upstream source and READ it.**

For every test function the restorer intends to write, identify the governing spec/canonical/wasmtime source and **read it in full**. Upstream sources in precedence order:

1. **`debug-vendored/component-model/design/mvp/canonical-abi/run_tests.py`** — the canonical-abi reference test harness. Authoritative for observable behavior of every canon_* function. Every Session 1 test must have a run_tests.py counterpart OR an explicit justified reason why it has none.
2. **`debug-vendored/component-model/design/mvp/canonical-abi/definitions.py`** — the reference implementation. Authoritative for step-by-step semantics of lift/lower/canon operations.
3. **`debug-vendored/component-model/design/mvp/CanonicalABI.md`** — the spec prose. Authoritative for rationale and invariants.
4. **`debug-vendored/wasmtime/tests/all/component_model/*.rs`** — wasmtime's production integration tests. Secondary to canonical sources but valuable for real-world scenarios.
5. **`debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/**/*.rs`** — wasmtime's production implementation. Useful when canonical sources are ambiguous.

**Precedence rule:** if `run_tests.py` and wasmtime tests exercise the same behavior differently (wasmtime may skip edge cases), `run_tests.py` wins. If `run_tests.py` asserts behavior that wasmtime does not test at all, Session 1's test must match `run_tests.py`.

**Step 4 — Port canonical cases.**

If `run_tests.py` has test cases covering the behavior being restored, **port them verbatim** (translated to Go, using wazero's types). Example: `run_tests.py::test_handles` has ~30 resource lifecycle cases covering new/rep/drop/own/borrow edge cases — the restored `conformance/resources_test.go` must include Go translations of each case. Every ported case has a comment line citing the run_tests.py line number of its origin.

**Step 5 — Write the test body with citation comment block.**

Every restored `func TestX(t *testing.T)` must be preceded by a comment block in this exact format:

```go
// TestExportedFuncCall_OwnArgument asserts lift_own semantics when an own
// handle is passed as a function argument.
//
// Spec: definitions.py:1333-1339 (lift_own).
// Canonical test: run_tests.py:482-501 (test_handles own-argument case).
// Wasmtime parallel: resources.rs:275-279 (resource_lift_own).
// Wasmtime test: wasmtime/tests/all/component_model/resources.rs:120-145.
func TestExportedFuncCall_OwnArgument(t *testing.T) {
    // ... body ...
}
```

Citation lines use one of these prefixes verbatim: `Spec:`, `Canonical test:`, `Wasmtime parallel:`, `Wasmtime test:`, or (only when no counterpart exists after exhaustive search) `No counterpart (justified):` followed by a one-sentence rationale. V4 verifies the citation block regex.

**Step 6 — Validate assertions against upstream.**

For every assertion in the restored test (`require.Equal`, `require.Error`, etc.), the restorer walks the cited upstream source and confirms the assertion matches observable behavior. If any assertion would contradict the upstream, the restorer reworks the assertion (preferred) or deletes the test entirely with a commit message explaining why.

**Step 7 — Run the test.**

```bash
go test ./internal/component/... -run TestX -count=1
```

Failures trigger either a bug in the restored test (reread the upstream) or a bug in the production code (fix the production code). The restorer iterates until the test passes against a spec-correct production code path.

**Step 8 — Commit with a citation-heavy message.**

The commit message names every upstream source the restorer consulted. The per-task spec-reviewer subagent re-reads the same upstream sources and cross-checks the assertions.

### Spec-reviewer checklist (Session 1 amendment)

After every test-restoration task, the spec-compliance review subagent performs these steps in order:

1. **Citation coverage**: run the V4 grep script on the files touched by the task. Fail if any test function lacks a citation block.
2. **Citation verification**: for each citation in each restored test, the reviewer **opens the cited file at the cited line number and reads the surrounding context**. The reviewer writes a one-sentence confirmation in the review report: "Confirmed: definitions.py:1338 contains `trap_if(not h.own)` as cited in TestExportedFuncCall_OwnArgument."
3. **Assertion cross-check**: for each `require.*` call in each restored test, the reviewer cross-references the assertion against the cited upstream and records whether it matches observable behavior.
4. **Deduplication check**: the reviewer searches the repo for any other test asserting the same behavior; if found, flags it as a duplicate for consolidation.
5. **`run_tests.py` coverage audit**: for the category of behavior the test restoration task targets (e.g., "resource lifecycle"), the reviewer lists every `run_tests.py` test case in that category and cross-checks whether the restoration covers each case; flags any `run_tests.py` case that Session 1's tests do not address.

Session 0 had eight tasks that needed correctives from review. Session 1's test-restoration tasks will see heavier review cost — plan for 2-3 review iterations per task on average.

### `run_tests.py` test enumeration (non-exhaustive starter list for Session 1 scope)

The following `run_tests.py` top-level test functions are in Session 1 scope and must have corresponding wazero tests:

- `test_pairs` — primitive type round-trips (48 cases). Ported in checkpoint C into `conformance/primitives_test.go` (already exists post-Session-0, but each case must be cross-referenced to its run_tests.py origin; restoration task for checkpoint C adds missing citation comments).
- `test_heap` — composite type heap serialization (32 cases). Checkpoint C → `conformance/composites_test.go`.
- `test_flatten` — flat ABI parameter/result flattening (7 cases). Checkpoint C → `conformance/flat_abi_test.go`.
- `test_roundtrips` — end-to-end lift/lower round-trips across primitive + composite types. Checkpoint C → `conformance/abi_edge_cases_test.go`.
- `test_handles` — resource lifecycle (new/rep/drop/own/borrow). ~30 cases. Checkpoint E → `conformance/resources_test.go`.
- `test_nan32` / `test_nan64` — NaN canonicalization (16 cases total). Checkpoint C → `conformance/primitives_test.go` or `abi_edge_cases_test.go`.
- `test_reentrance` — reentrance semantics. Checkpoint D → `conformance/reentrance_test.go` (already exists post-Session-0; restoration adds citation comments and extends with missing cases).

These are deferred to Later (async lift/lower):
- `test_wasm_to_wasm_stream`, `test_eager_stream_completion`, `test_async_stream_ops` — stream primitives.
- `test_futures`, `test_cancel_subtask`, `test_cancel_copy` — futures + cancellation.
- `test_threads`, `test_thread_cancel_callback` — threading.
- `test_async_to_async`, `test_async_callback` — async function calls.

### Bounds-check test harness ([]byte-backed memory)

The 11 bounds-check tests in `abi/context_test.go` + `abi/strings_test.go` fail because `wazerotest.NewMemory` rounds to page size (64 KiB). Session 1 adds a minimal `byteMemory` test helper in `abi/memory_test_helper.go` implementing the full `api.Memory` interface:

```go
package abi

import (
    "encoding/binary"
    "math"

    "github.com/tetratelabs/wazero/api"
)

// byteMemory is a direct []byte-backed api.Memory implementation for
// bounds-check tests that need to construct memories smaller than a
// Wasm page (64 KiB). Every method implements the wazero api.Memory
// contract: methods that read or write out of range return (zero, false)
// or (false) per the interface's "or returns false if out of range" rule.
//
// Does NOT implement Definition() or Grow() with meaningful semantics;
// both return zero values. Tests that exercise Definition or Grow must
// use wazerotest.NewMemory or a fuller stub; byteMemory is explicitly
// scoped to the 11 bounds-check tests.
//
// No counterpart (justified): this is a wazero test-harness invariant
// (api.Memory bounds semantics) not covered by canonical-abi spec.
type byteMemory struct {
    data []byte
}

func newByteMemory(size uint32) *byteMemory {
    return &byteMemory{data: make([]byte, size)}
}

func (m *byteMemory) Definition() api.MemoryDefinition { return nil }
func (m *byteMemory) Size() uint32                    { return uint32(len(m.data)) }
func (m *byteMemory) Grow(deltaPages uint32) (previousPages uint32, ok bool) {
    return 0, false
}

func (m *byteMemory) inRange(offset, length uint32) bool {
    return uint64(offset)+uint64(length) <= uint64(len(m.data))
}

func (m *byteMemory) ReadByteAt(offset uint32) (byte, bool) {
    if !m.inRange(offset, 1) {
        return 0, false
    }
    return m.data[offset], true
}

func (m *byteMemory) ReadUint16Le(offset uint32) (uint16, bool) {
    if !m.inRange(offset, 2) {
        return 0, false
    }
    return binary.LittleEndian.Uint16(m.data[offset:]), true
}

func (m *byteMemory) ReadUint32Le(offset uint32) (uint32, bool) {
    if !m.inRange(offset, 4) {
        return 0, false
    }
    return binary.LittleEndian.Uint32(m.data[offset:]), true
}

func (m *byteMemory) ReadFloat32Le(offset uint32) (float32, bool) {
    v, ok := m.ReadUint32Le(offset)
    if !ok {
        return 0, false
    }
    return math.Float32frombits(v), true
}

func (m *byteMemory) ReadUint64Le(offset uint32) (uint64, bool) {
    if !m.inRange(offset, 8) {
        return 0, false
    }
    return binary.LittleEndian.Uint64(m.data[offset:]), true
}

func (m *byteMemory) ReadFloat64Le(offset uint32) (float64, bool) {
    v, ok := m.ReadUint64Le(offset)
    if !ok {
        return 0, false
    }
    return math.Float64frombits(v), true
}

func (m *byteMemory) Read(offset, byteCount uint32) ([]byte, bool) {
    if !m.inRange(offset, byteCount) {
        return nil, false
    }
    return m.data[offset : offset+byteCount], true
}

func (m *byteMemory) WriteByte(offset uint32, v byte) bool {
    if !m.inRange(offset, 1) {
        return false
    }
    m.data[offset] = v
    return true
}

func (m *byteMemory) WriteUint16Le(offset uint32, v uint16) bool {
    if !m.inRange(offset, 2) {
        return false
    }
    binary.LittleEndian.PutUint16(m.data[offset:], v)
    return true
}

func (m *byteMemory) WriteUint32Le(offset uint32, v uint32) bool {
    if !m.inRange(offset, 4) {
        return false
    }
    binary.LittleEndian.PutUint32(m.data[offset:], v)
    return true
}

func (m *byteMemory) WriteFloat32Le(offset uint32, v float32) bool {
    return m.WriteUint32Le(offset, math.Float32bits(v))
}

func (m *byteMemory) WriteUint64Le(offset uint32, v uint64) bool {
    if !m.inRange(offset, 8) {
        return false
    }
    binary.LittleEndian.PutUint64(m.data[offset:], v)
    return true
}

func (m *byteMemory) WriteFloat64Le(offset uint32, v float64) bool {
    return m.WriteUint64Le(offset, math.Float64bits(v))
}

func (m *byteMemory) Write(offset uint32, v []byte) bool {
    if !m.inRange(offset, uint32(len(v))) {
        return false
    }
    copy(m.data[offset:], v)
    return true
}

func (m *byteMemory) WriteString(offset uint32, v string) bool {
    if !m.inRange(offset, uint32(len(v))) {
        return false
    }
    copy(m.data[offset:], v)
    return true
}
```

The 11 tests use `newByteMemory(3)` (or similar sub-page sizes) to construct memories that trigger bounds errors on 4/8-byte reads.

The file is placed in `internal/component/abi/memory_test_helper.go` with a build tag constraining it to test builds (`//go:build test` or naming it `*_test.go`). Naming as `memory_test.go` (appended to an existing file) is also acceptable.

## Work Order + Checkpoint Gates

The implementation plan (separate doc at `docs/superpowers/plans/2026-04-08-canonical-abi-session1-plan.md`) numbers tasks linearly. This section names the checkpoint gates and the task groups that feed each gate.

| Checkpoint | Scope | Success criterion |
|---|---|---|
| **A — `Component.TypeDefs` + decoder exposure** | Add `TypeDef` with `ResourceDtor*` fields + `Component.TypeDefs []TypeDef`. Populate in decoder. Delete `decodeContext.funcTypeIdx` / `resourceDefs`. Restore 18 decoder tests in `binary/component_type_test.go` + `binary/instance_type_test.go`. | `go build ./internal/component/binary/... ./internal/component/...` green. `go test ./internal/component/binary/...` green including the 18 restored tests. |
| **B — `component.Instance` embeds `*runtime.ComponentInstance`** | Delete duplicated fields. Rewrite delegators. Update every `i.table`/`i.mayLeaveDisabled`/etc. call site. Compile-fix every caller in `internal/component/`, `imports/wasip2/`, test files. | `go build ./internal/component/... ./imports/wasip2/...` green. Accessor tests in `instance_test.go` pass. |
| **C — `Instantiate` top-level + canon.lift/lower/resource wiring + primitive conformance** | Delete Instantiate/coreSignature stubs. Rebuild Instantiate (step 1-12 of pipeline). Rebuild buildComponentFuncs, instantiateCoreModule, createCanonLowerExport, createResourceNewExport/DropExport/RepExport, createAliasExport, wireExportedFunc. Restore primitive + composite + string + abi_edge_cases conformance tests. Restore linker_test.go + linker_api_test.go. | `go test ./internal/component/conformance/ -run 'Primitive|Composite|String|ABIEdge'` green. `go test ./internal/component/ -run 'Linker'` green. |
| **D — Nested components + resolveExportTypeAlias + integration tests** | Delete nested_component.go panic stub. Rebuild resolveExportTypeAlias + instantiateNestedComponent + wireNestedComponentExports + createInlineInstanceModule. Restore nested_component_test.go (21 tests), integration_test.go (19 tests), start_function_test.go (9 tests), component_linker_test.go (8 tests). | `go test ./internal/component/ -run 'Nested|Integration|StartFunction|ComponentLinker'` green. |
| **E — Resource type binding + 4 lift.go fixes + resource conformance** | Implement `bindResourceTypes`. Add `Table.GetByIndex`. Change `ResourceHandleEntry.Rep` from `any` to `uint32` + migrate `imports/wasip2/io/*` to per-module host state registries with u32 ids. Add `runtime.ResourceType.HostDestructor` field. Fix 4 lift.go gaps in `liftOwnHandle` / `liftBorrowHandle`. Rewrite `Instance.ResourceNew` / `ResourceRep` / `ResourceDrop` to spec-correct signatures. Restore conformance/resources_test.go + destructor_test.go (local-instance cases only; cross-instance destructor cases get per-case session-2 skips) + resource_generation_test.go. Restore instance_test.go's resource-related tests. Restore 11 abi/ bounds-check tests via byteMemory helper. | `go test ./internal/component/conformance/ -run 'Resources\|Destructor\|ResourceGeneration'` green. `go test ./internal/component/ -run 'CanonResource'` green. `go test ./internal/component/abi/ -run 'BoundsCheck'` green. |
| **F — All 223 tests + 29 conformance stubs green, no skips remaining** | Restore remaining instance_test.go lift/lower tests (TestLiftEnum, TestLiftFlags, TestLiftVariant, TestExportedFunc_Call_*, etc., 33 tests). Restore conformance/*test.go stubs not covered in C-E (concurrent_access, error_messages, flat_abi, instance_types, memory_bounds, nesting_depth, post_return, realloc_failure, type_edge_cases, utf_validation, WASI world tests). Restore type_checker_test.go (17 tests), edge_case_test.go (1), component_test.go (1), value_import_test.go (1), composite_test.go (5), instantiate_test.go (2), integration_public_api_test.go (7), integration_records_test.go (2). Fix type_checker.go checkFuncDefinition/checkInstanceDefinition. | `go test ./... -run '.*' -count=1` green with zero `t.Skip("session 1 work")` remaining. `grep -rn "session 1 work" internal/ api/ imports/` returns empty. |
| **Final** | Run full suite, regenerate followup note for Session 2 + Later. | `go vet ./...` green. `go test ./... -count=1` green. `docs/plans/2026-04-08-canonical-abi-session1-followup.md` written (Session 2 scope + Later async scope, deltas from Session 0 note). |

### Between-checkpoint reviews

After each checkpoint A–F, the plan runs:
1. `superpowers:code-reviewer` over the checkpoint's task group.
2. Spec-compliance review subagent with the Session 1 amended checklist.

Correctives identified by either reviewer land as follow-up tasks before the next checkpoint begins.

## File Manifest

### Stubs to delete (replace with real bodies)

- `internal/component/instance.go:156` — `ExportedFunc.Call` panic → real body.
- `internal/component/instance.go:185` — `Instance.ResourceNew` panic → new signature + real body.
- `internal/component/instance.go:193` — `Instance.ResourceRep` panic → new signature + real body.
- `internal/component/instance.go:202` — `Instance.ResourceDrop` panic → new signature + real body.
- `internal/component/component_linker.go:146` — `ComponentLinker.Instantiate` panic → full rebuild.
- `internal/component/component_linker.go:177` — `coreSignature` panic → **deleted entirely**. Callers route to `abi.CoreSignature` directly.
- `internal/component/nested_component.go:167` — `resolveExportTypeAlias` panic → real body walking `c.TypeDefs`.

### New files

- `internal/component/abi/memory_test_helper.go` — `byteMemory` direct `[]byte`-backed `api.Memory` implementation for bounds-check tests.
- `internal/component/component_linker_helpers.go` — extracted helpers for `Instantiate` (if the rebuilt `component_linker.go` grows too large). Optional split; may stay in `component_linker.go`.

### Deleted fields / methods

**`internal/component/instance.go` `Instance` struct — deleted fields:**
- `table *runtime.Table` — moved to `rt.Table`.
- `destructors map[uint32]func(any)` — moved to `rt.Destructors`.
- `callContext *runtime.CallContext` — per-call state, allocated fresh in each lift/lower closure; no longer stored on `Instance`.
- `mayLeaveDisabled bool` — inverse of `rt.MayLeave`; delegated.
- `activeCallDepth int32` — delegated to `rt.EnterCount()`.

**`internal/component/instance.go` `Instance` struct — kept fields:**
- `parent *Instance` — wrapper-layer back-pointer, paired with `rt.Parent` at construction. Not a spec field; linker-time navigation convenience.

**`internal/component/instance.go` methods** (rewritten as delegators, body removed):
- `SetDestructor(uint32, func(any))` → delegates to `rt.Destructors.Register`.
- `SetCallContext(*runtime.CallContext)` → **deleted**; `CallContext` is per-call state allocated fresh in each canon.lift / canon.lower closure, not stored on `Instance`.
- `CallContext() *runtime.CallContext` → **deleted**; callers that need one allocate via `runtime.NewCallContext()`.
- `MayLeave()`, `SetMayLeave()`, `ActiveCallDepth()`, `EnterCall()`, `ExitCall()`, `CallMightBeRecursive()`, `ValidateMayLeave()`, `ValidateNotRecursive()` — all delegate to `rt`.

**`internal/component/binary/decoder.go`:**
- `decodeContext.funcTypeIdx map[uint32]types.FuncTypeIdx` — deleted. Info stored on `c.TypeDefs` during `decodeTypeSection`.
- `decodeContext.resourceDefs map[uint32]*ResourceTypeDef` — deleted. Info stored on `c.TypeDefs` during `decodeTypeSection`.

### New fields / types

**`internal/component/component.go`:**
- `Component.TypeDefs []TypeDef` — per-type-section-slot index.
- `TypeDef.ResourceDtor *uint32`, `TypeDef.ResourceDtorAsync bool`, `TypeDef.ResourceDtorCallback *uint32` — resource declaration destructor metadata.
- `TypeDef.Func` changes from `*types.TypeFunc` to `types.FuncTypeIdx` (Decision 5).

**`internal/component/runtime/table.go`:**
- `Table.GetByIndex(idx uint32) (Handle, TableEntry, error)` — new method for lift-gap 3 (generation-tag bridging).
- `ResourceHandleEntry.Rep` type changes from `any` to `uint32` (Decision 4 Gap 4 / spec `definitions.py:337-349` `ResourceHandle.rep`).

**`internal/component/runtime/resource_type.go`:**
- `ResourceType.HostDestructor func(rep uint32) error` — new field; nil for guest-declared resources, set by `imports/wasip2/*` modules on their per-kind `*ResourceType` singletons. `Table.Remove` invokes `HostDestructor(entry.Rep)` when set (otherwise it uses the guest-side `Dtor *uint32` via `invokeLocalDestructor`).

**`internal/component/runtime/borrow_scope.go`:**
- `BorrowScope.ReleaseBorrow(h Handle) error` — new method; symmetric to `AddLender`. Decrements the scope's outstanding lend counter and removes `h` from the lender set. Called by `Instance.ResourceDrop` borrow branch (spec `:2163-2164`).

**`internal/component/abi/*.go`:**
- `abi.LowerParams(ctx *LowerContext, paramTypes []types.ValType, args []types.Val, maxFlat int) ([]uint64, error)` — aggregate-boundary-aware param lowering per spec `lower_flat_values` at `definitions.py:1943-1975` including the `may_leave` toggle.
- `abi.LiftParams(ctx *LiftContext, paramTypes []types.ValType, flat []uint64, maxFlat int) ([]types.Val, error)` — mirror of `LowerParams`.
- `abi.LiftResults(ctx *LiftContext, resultTypes []types.ValType, flat []uint64, maxFlat int) ([]types.Val, error)` — result lifting with retptr support.
- `abi.LowerResults(ctx *LowerContext, resultTypes []types.ValType, results []types.Val, stack []uint64, needsRetptr bool, maxFlat int) error` — result lowering with retptr support.

**`internal/component/component_linker.go`:**
- `ComponentLinker.DefineFunc(namespace, name string, typ *types.TypeFunc, fn HostFunc) error` — new signature with required `*types.TypeFunc` parameter (Decision 6).
- `Linker.DefineFunc` / `InstanceBuilder.Func` / `ComponentInstanceBuilder.Func` — analogous signature change.

### Test files to restore (from git commit `98b3bbc3`)

| File | Skipped tests | Restore strategy |
|---|---|---|
| `internal/component/instance_test.go` | 57 | Resource tests in checkpoint E, lift/lower tests in F, accessor tests in B. |
| `internal/component/linker_api_test.go` | 8 | Checkpoint C. |
| `internal/component/linker_test.go` | 34 | Checkpoint C. |
| `internal/component/nested_component_test.go` | 21 | Checkpoint D. |
| `internal/component/integration_test.go` | 19 | Checkpoint D. |
| `internal/component/start_function_test.go` | 9 | Checkpoint D. |
| `internal/component/component_linker_test.go` | 8 | Checkpoint D. |
| `internal/component/integration_public_api_test.go` | 7 | Checkpoint F. |
| `internal/component/composite_test.go` | 5 | Checkpoint F. |
| `internal/component/instantiate_test.go` | 2 | Checkpoint F. |
| `internal/component/integration_records_test.go` | 2 | Checkpoint F. |
| `internal/component/type_checker_test.go` | 17 | Checkpoint F. |
| `internal/component/edge_case_test.go` | 1 | Checkpoint F. |
| `internal/component/component_test.go` | 1 | Checkpoint A (decoder-produced ComponentTypes test). |
| `internal/component/value_import_test.go` | 1 | Checkpoint C. |
| `internal/component/binary/component_type_test.go` | 8 | Checkpoint A. |
| `internal/component/binary/instance_type_test.go` | 10 | Checkpoint A. |
| `internal/component/abi/context_test.go` | 9 | Checkpoint E (bounds-check helpers). |
| `internal/component/abi/strings_test.go` | 4 | Checkpoint E. |

| Conformance file | Session 1 checkpoint |
|---|---|
| `abi_edge_cases_test.go` | C |
| `composites_test.go` | C |
| `strings_test.go` | C |
| `flat_abi_test.go` | C |
| `concurrent_access_test.go` | E |
| `destructor_test.go` | E |
| `error_messages_test.go` | F |
| `instance_types_test.go` | F |
| `linker_test.go` | C |
| `memory_bounds_test.go` | F |
| `nested_instantiation_test.go` | D |
| `nested_test.go` | D |
| `nesting_depth_test.go` | D |
| `post_return_test.go` | C |
| `realloc_failure_test.go` | F |
| `resource_generation_test.go` | E |
| `resources_test.go` | E |
| `type_edge_cases_test.go` | F |
| `utf_validation_test.go` | F |
| `wasi_cli_test.go` | F |
| `wasi_clocks_test.go` | F |
| `wasi_error_handling_test.go` | F |
| `wasi_filesystem_test.go` | F |
| `wasi_http_test.go` | F |
| `wasi_poll_test.go` | F |
| `wasi_random_test.go` | F |
| `wasi_resource_lifecycle_test.go` | F |
| `wasi_sockets_test.go` | F |
| `wasi_streams_test.go` | F |

**Already-complete conformance test files (audit-only, no restoration needed):**

Three conformance test files already have real test bodies at HEAD `c5d023d6` and are currently passing. They are not deferred stubs and are NOT in the "restore from git" workflow. Session 1 treats each as an **audit-only** target: a dedicated task reads the file, adds the required citation comment blocks per the Test Restoration Methodology (Step 5), cross-references every assertion against `run_tests.py` / definitions.py / wasmtime tests, extends the file with any missing `run_tests.py` cases identified during the audit, and fails the V4 grep check if citations are incomplete.

| Conformance file | Lines at HEAD | Session 1 task | Checkpoint |
|---|---|---|---|
| `primitives_test.go` | 897 | audit + add citations + extend with missing `test_pairs` / `test_nan32` / `test_nan64` cases | C |
| `may_leave_test.go` | 142 | audit + add citations from `definitions.py:1955, 1973, 2065, 2135, 2143` may_leave sites | C |
| `reentrance_test.go` | 176 | audit + add citations from `definitions.py:290-299, 1979` + `test_reentrance` cases | D |

The audit tasks for these three files are counted separately from the "restore X tests from git" tasks. They may surface assertions that contradict upstream behavior (user's rework-or-delete constraint applies). Total new scope: three audit tasks added to checkpoints C (2) and D (1).

`conformance/subtask_test.go` stays deferred-to-Later.

### `abi/lift_test.go`, `abi/lower_test.go`, `abi/flatten_test.go` — breadth restoration

These were rewritten from scratch in Task 15 of Session 0 with minimal new coverage. The original files were 2925, 1856, and 349 lines respectively. Session 1 reads the pre-Session-0 bodies from git (`git show 98b3bbc3:internal/component/abi/lift_test.go`) and restores the breadth — scalar round-trips, record/variant/list/tuple/option/result/flags/enum dispatch, surrogate-pair char handling, fixed-length list, variant join semantics at all widths, error-path coverage — rewritten against the new builder API. This work is split across checkpoints C (primitives + composites), D (nested types with variants), and F (error paths + edge cases).

## Build State at End of Session 1

| Package | Build | Tests |
|---|---|---|
| `internal/component/types/` | green | green |
| `internal/component/runtime/` | green | green (+ `Table.GetByIndex` test coverage) |
| `internal/component/binary/` | green | green (18 decoder tests restored) |
| `internal/component/abi/` | green | green (11 bounds-check tests passing, lift/lower/flatten breadth restored) |
| `internal/component/conformance/` | green | green (29 stubs replaced; `subtask_test.go` stays deferred-to-Later) |
| `internal/component/` (top-level) | **green** | **green** (223 tests restored, zero `t.Skip("session 1 work")` remaining) |
| `api/component/` | green | green |
| `imports/wasip2/...` | green | green |
| Repo-wide `go build ./...` | green | — |
| Repo-wide `go test ./...` | — | green except `conformance/subtask_test.go` (`t.Skip("later work: async lift/lower")`) |

## Followup Note — Session 2 Scope

At the end of Session 1, `docs/plans/2026-04-08-canonical-abi-session1-followup.md` is written with:

### Session 2 — Cross-instance resource resolution + cross-component type checking

- **Cross-instance resource type resolution.** Current `LookupResourceType` walks only the `Parent` chain. Session 2 extends via a linker-maintained or store-wide registry: given a `(RuntimeComponentInstanceIdx, ResourceIdx)` pair from a different instance's type table, find that instance's `*runtime.ResourceType`. Location: `runtime.ComponentInstance.LookupResourceType` + a new `runtime.InstanceRegistry` or equivalent.

- **`typeChecker` cross-component structural walk.** Current `checkFuncType` does same-bag identity. Cross-component matching (host's type bag differs from component's) requires structural comparison. New file: `internal/component/types/typecheck.go` or extension of existing `type_checker.go`. Walks `*types.ComponentTypes` entries recursively, comparing Record fields, Variant cases, etc.

- **`TypeResourceTable.Concrete` promotion bit.** Session 1 does not mutate `TypeResourceTable.Concrete` — it relies on `ResourceTypes` population + `LookupResourceType`. Session 2 may either wire the `Concrete` bit for cross-component matching, or document that `Concrete` stays in the Abstract/deferred state for all local-only instantiation.

- **Cross-instance destructor invocation.** Current `ResourceDrop` traps when `rt.Impl != i.rt` (spec `definitions.py:2154-2160` case). Session 2 wires the `canon_lift` → `canon_lower` cross-call that invokes a destructor in a foreign instance.

### Later — Async lift/lower

- Unchanged from Session 0 followup note. Stream / future / error-context / subtask support.
- `conformance/subtask_test.go` stays deferred.

## Verification Checklist

### V1 — All panic stubs deleted

```bash
grep -n 'panic("compile-fix stub' internal/component/
```

Returns empty.

### V2 — No `t.Skip("session 1 work")` remaining

```bash
grep -rn 'session 1 work' internal/ api/ imports/
```

Returns empty.

### V3 — No `TestXxxDeferredToSession1` functions remaining

```bash
grep -rn 'DeferredToSession1' internal/component/conformance/
```

Returns empty.

### V4 — All restored tests cite upstream (operationalized)

Every restored test function must have an upstream-citation comment block immediately above the `func Test...` declaration. The comment block must contain one or more lines matching the regex `(Spec:|definitions\.py:|run_tests\.py|Wasmtime:|wasmtime tests/|No counterpart \(justified\):)`. Verified by grep:

```bash
# List every test function in the restored files and confirm each is preceded
# by an upstream-citation comment block within 10 lines above the declaration.
#
# The per-task spec-review subagent runs this script on the files touched by
# the task and fails the review if any test function lacks a citation.
python3 -c '
import os, re, sys

def check_file(path):
    with open(path) as f:
        lines = f.readlines()
    bad = []
    pattern = re.compile(r"^func (Test\w+)\(t \*testing\.T\)")
    cite = re.compile(r"(Spec:|definitions\.py:|run_tests\.py|Wasmtime:|wasmtime tests/|No counterpart \(justified\):)")
    for i, line in enumerate(lines):
        m = pattern.match(line)
        if not m: continue
        # walk back up to 15 non-blank comment lines looking for a citation
        found = False
        for j in range(i-1, max(i-16, -1), -1):
            if lines[j].strip().startswith("//") and cite.search(lines[j]):
                found = True
                break
            if not lines[j].strip().startswith("//") and lines[j].strip() != "":
                break
        if not found:
            bad.append(f"{path}:{i+1}: {m.group(1)}")
    return bad

bad = []
for root, _, files in os.walk("internal/component"):
    for f in files:
        if f.endswith("_test.go"):
            bad.extend(check_file(os.path.join(root, f)))
if bad:
    print("\n".join(bad))
    sys.exit(1)
'
```

Returns empty (exit code 0) iff every restored test function has a citation block. Every test the spec-review subagent clears must pass this grep. The `No counterpart (justified):` form is the only acceptable escape hatch and must be followed by a one-sentence rationale explaining why this behavior has no upstream counterpart.

### V5 — `c.TypeDefs[idx]` is the canonical resolver

```bash
grep -rn 'funcTypeIdx\|resourceDefs' internal/component/binary/
```

Returns empty (private decoder maps deleted).

### V6 — `component.Instance` embeds `*runtime.ComponentInstance`

```bash
grep -n 'table.*runtime\.Table\|mayLeaveDisabled\|activeCallDepth' internal/component/instance.go
```

Returns empty (duplicated fields deleted).

### V7 — Four latent lift.go gaps closed

- `grep -n 'resEntry.Own' internal/component/abi/lift.go` → `liftOwnHandle` has the `trap_if !Own` check (Gap 1).
- `grep -n 'resEntry.NumLends' internal/component/abi/lift.go` → `liftOwnHandle` has the `NumLends != 0` check (Gap 2).
- `grep -n 'GetByIndex' internal/component/abi/lift.go` → `liftOwnHandle` AND `liftBorrowHandle` use `Table.GetByIndex` (Gap 3).
- `grep -n 'func .Table. GetByIndex' internal/component/runtime/table.go` → confirms the method exists (Gap 3).
- `grep -n 'Rep uint32\|Rep   uint32' internal/component/runtime/table.go` → `ResourceHandleEntry.Rep` is typed `uint32` (Gap 4).
- `grep -n '\.Rep\.(uint32)' internal/component/` → returns empty (no lingering type assertions on a type-`any` Rep).
- `grep -rn 'HostDestructor' internal/component/runtime/ imports/wasip2/` → confirms `runtime.ResourceType.HostDestructor` field exists and is set by wasip2 modules.

### V8 — Instance resource ops match spec signatures

- `grep -n 'func (i \*Instance) ResourceNew' internal/component/instance.go` → `ResourceNew(resourceIdx types.ResourceIdx, rep uint32) (uint32, error)`.
- `grep -n 'func (i \*Instance) ResourceRep' internal/component/instance.go` → `ResourceRep(resourceIdx types.ResourceIdx, handleIdx uint32) (uint32, error)`.
- `grep -n 'func (i \*Instance) ResourceDrop' internal/component/instance.go` → `ResourceDrop(resourceIdx types.ResourceIdx, handleIdx uint32) error`.

### V9 — `c.TypeDefs` exists and is populated

```bash
grep -n 'TypeDefs \[\]TypeDef' internal/component/component.go
```

Returns the field definition.

```bash
grep -n 'c\.TypeDefs = append' internal/component/binary/decoder.go
```

Returns the decoder population sites.

### V10 — Resource type binding runs during Instantiate

```bash
grep -n 'bindResourceTypes' internal/component/component_linker.go
```

Returns the function definition + call site.

### V11 — type_checker.go same-bag identity fix

```bash
grep -n '_ = expected' internal/component/type_checker.go
```

Returns empty (ignored-expected bug gone).

### V12 — Every restored test cites its upstream

Sample audit (5 files):

```bash
for f in internal/component/conformance/resources_test.go \
         internal/component/conformance/strings_test.go \
         internal/component/instance_test.go \
         internal/component/linker_test.go \
         internal/component/abi/lift_test.go; do
    echo "=== $f ==="
    grep -c 'Spec:\|Wasmtime\|definitions\.py' "$f" || echo "MISSING CITATIONS"
done
```

Returns non-zero citation counts per file.

## Out of Scope

- Cross-instance resource type resolution (Session 2).
- `typeChecker` cross-component structural walk (Session 2).
- Async lift/lower (Later — no session scheduled).
- WIT-binding codegen typed path (no session).
- Any change to `api/component` public surface beyond what's forced by Session 1's internal type changes.
- Re-litigation of Session 0 Design Decisions 1–9.
