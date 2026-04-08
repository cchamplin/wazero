# Canonical-ABI Session 1 — Design

**Date:** 2026-04-08
**Status:** Design approved, ready for implementation planning
**Scope:** Session 1 only (wire `abi/` into production, rebuild `Instantiate`, local-only Concrete promotion, restore 223 tests + 29 conformance stubs). Session 2+ documented as followups.
**Previous session:** `docs/superpowers/specs/2026-04-07-canonical-abi-type-unification-design.md` (Session 0) + `docs/plans/2026-04-07-canonical-abi-unification-session0-followup.md` (Session 0 followup note).

## Summary

Session 0 delivered the new type representation (`types.ValType` + `types.ComponentTypes` + builder), the unified `runtime.Table`, the single-layer `runtime.ComponentInstance`, the pointer-identity `*runtime.ResourceType`, and a rewritten `abi/` lift/lower package that consumes the canonical bag. It left seven panic stubs at known file:line locations, 223 skipped tests behind `t.Skip("session 1 work")`, and 29 conformance test files reduced to single-deferred shells.

Session 1 wires `abi/` into the production call path, rebuilds the full `ComponentLinker.Instantiate` pipeline (including nested components, inline host instances, canon.lift/lower/resource host module exports, start function execution, and memory sharing) against the new types, pulls the local subset of resource Concrete promotion forward from Session 2 (so `Instance.ResourceNew/Rep/Drop` and same-instance own/borrow work end-to-end), fixes three latent correctness gaps in `abi/lift.go`, exposes the decoder's per-type-section-slot mapping on `Component.TypeDefs`, and restores all 223 skipped tests + 29 conformance stubs from pre-Session-0 git history with per-test validation against `definitions.py` / wasmtime / canonical-abi reference test counterparts.

Session 2 (cross-instance resource resolution + cross-component `typeChecker` structural walk) and Later (async lift/lower for stream/future/error-context/subtask) are explicitly deferred.

## Goals

- Zero panic stubs in `internal/component/instance.go`, `internal/component/component_linker.go`, `internal/component/nested_component.go`. Every method body either does real work or traps with a precise error pointing at Session 2 / Later.
- `ComponentLinker.Instantiate` rebuilt end-to-end against `*types.ComponentTypes` + `*runtime.ComponentInstance` + `abi.LiftContext`/`LowerContext` + `abi.FlattenParams`/`FlattenResults`/`CoreSignature`. No direct dependency on the deleted `resolveToValType` / `typeDefToValType` / `valTypeRefToValType` / `TypeResolver` helpers.
- `component.Instance` embeds `*runtime.ComponentInstance`; all spec-level runtime state (Table, MayLeave, EnterCount, ResourceTypes, Destructors, Reentrance, Parent) delegates into the embedded runtime struct. Every duplicate field on `component.Instance` is deleted.
- `Instance.ResourceNew` / `ResourceRep` / `ResourceDrop` signatures match the spec's `canon_resource_new(rt, thread, rep)` / `canon_resource_rep(rt, thread, i)` / `canon_resource_drop(rt, thread, i)` at `definitions.py:2134, 2142, 2169`. Pointer-identity `*runtime.ResourceType` enforces the spec's `h.rt is t.rt` check at `definitions.py:1345, 2147, 2172`.
- Local-only Concrete promotion lands: every resource declaration in the component being instantiated mints a fresh `*runtime.ResourceType` stored in `rt.ResourceTypes[ResourceIdx]`. Same-instance own/borrow + resource.new/rep/drop works end-to-end. Cross-instance resource handles trap with a precise "session 2 wiring" error.
- The three latent correctness gaps in `abi/lift.go::liftOwnHandle`/`liftBorrowHandle` are fixed per the canonical-abi reference: `trap_if(not h.own)` added (`definitions.py:1338`), `Table.GetByIndex(idx uint32)` added for Wasm-side-32-bit → runtime-64-bit generation bridging, and `liftOwnHandle`/`liftBorrowHandle` return `entry.Rep` instead of the raw handle index (`definitions.py:1339, 1347`).
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
  - `canon_lower` at lines 2065–2120.
  - `canon_resource_new` at lines 2134–2138.
  - `canon_resource_drop` at lines 2142–2165.
  - `canon_resource_rep` at lines 2169–2173.
  - `may_leave` toggling during post-return at 2000–2002.
  - `deliver_resolve` / borrow scope cleanup at 738–742.
- `debug-vendored/component-model/design/mvp/CanonicalABI.md` — spec prose.
  - Resource identity at 531–549.
  - Same-instance borrow optimization at 2677–2683.
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/func.rs` — wasmtime's `Func::call` / `call_impl` / `call_raw` top-level exported call flow (lines 232–706, post-return at 737–834).
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

### Three latent correctness gaps in `abi/lift.go`

Verified against HEAD `c5d023d6` lift.go:

1. **`liftOwnHandle` does not validate `entry.Own == true` before `Remove`** (lift.go:665). The canonical-abi reference `lift_own` at `definitions.py:1333–1339` asserts `trap_if(not h.own)` at line 1338 — a borrow handle whose `rt` happens to match the expected type would currently be silently removed. Triggers a spec-incorrect ownership transfer. Unreachable in Session 0 because `LookupResourceType` returns nil before the gap is hit, but live once Session 1 lands Concrete promotion.

2. **`runtime.Handle(handleIdx)` hard-codes generation=0** (lift.go:660 and lift.go:696). Wazero's `runtime.Table` uses 64-bit generation-tagged handles (upper 32 bits = generation counter, lower 32 bits = slot index). The component-model flat ABI passes a raw 32-bit `u32` handle index from Wasm memory. `runtime.Handle(handleIdx)` constructs a handle with generation = 0, so any table slot with a non-zero current generation (i.e., any slot that has ever been recycled) will not be found. **Fix:** add `Table.GetByIndex(idx uint32) (Handle, TableEntry, bool)` that looks up by slot index and returns the current generation-tagged handle plus the entry. `liftOwnHandle` / `liftBorrowHandle` call this instead of constructing a Handle from the raw index.

3. **`ValOwn(handleIdx)` stores the Wasm-side handle index, not the rep** (lift.go:668 and lift.go:708). The canonical-abi reference `lift_own` at `definitions.py:1339` returns `h.rep`. Callers that need the representation must read `entry.Rep` — the current `types.ValOwn(handleIdx)` encoding forces a re-lookup. **Fix:** `liftOwnHandle` returns `types.ValOwn(uint32(entry.Rep))` (after `Remove`), and `liftBorrowHandle` returns `types.ValBorrow(uint32(entry.Rep))` (after `IncrementLends`). Caller-side callers that previously read the index and re-fetched the rep adapt to use the rep directly.

### Test state from Session 0

- **223 skipped tests** in `internal/component/` — each file contains `t.Skip(session1SkipReason)` calls with no bodies. The original pre-Session-0 bodies are available in git at commit `98b3bbc3` (the rename commit before the compile-fix stub commit `36a29b13`).
- **29 conformance files wholesale-stubbed** in `internal/component/conformance/` — each reduced to a single `TestXxxDeferredToSession1(t)` function. Original multi-case bodies are in git history.
- **11 abi/ bounds-check + context-shape tests skipped** in `internal/component/abi/context_test.go` + `strings_test.go`. The skip reason is the `wazerotest.NewMemory` harness rounding up to page size (64 KiB), which makes it impossible to construct a memory smaller than the pointer being bounds-checked.
- **Decoder tests skipped** in `internal/component/binary/component_type_test.go` (8 tests) and `instance_type_test.go` (10 tests). These validate the decoder's `*types.ComponentTypes` production.

## Design Decisions

### Decision 1: Full rebuild of `Instantiate` against new types, no salvage

The pre-Session-0 `component_linker.go` (3810 lines) is a reference, not a source to copy from. Session 1 rebuilds `Instantiate` and its helpers (`buildComponentFuncs`, `instantiateCoreModule`, `wireExportedFunc`, `wireNestedComponentExports`, `createInlineInstanceModule`, `createCanonLowerExport`, `createResourceOpExport`, `createAliasExport`, `wireMemorySharing`, `resolveMemorySource`, `executeStartFunction`, `resolveExportTypeAlias`, `buildTypeSpace`) against the new types:

- Per-type dispatch goes through `abi.LiftFlat` / `abi.LiftHeap` / `abi.LowerFlat` / `abi.LowerHeap`. No lift/lower helpers re-added to `component_linker.go`.
- Core signature computation goes through `abi.CoreSignature`, `abi.FlattenParams`, `abi.FlattenResults`. The `component_linker.go:177 coreSignature` panic stub is deleted and its call sites route directly to `abi.CoreSignature`.
- Type resolution goes through `c.TypeDefs[canon.TypeIdx]` (Decision 5). No re-added `resolveToValType` / `typeDefToValType` / `valTypeRefToValType` / `TypeResolver` helpers.
- `*types.ComponentTypes` is threaded through `LiftContext.Types` / `LowerContext.Types` via the per-call construction site. `*runtime.ComponentInstance` is threaded via `LiftContext.Instance` / `LowerContext.Instance`.
- Host module creation for `canon.lower` uses a closure that constructs a `LowerContext` per call, runs `LowerFlat` / `LowerHeap` per parameter, stashes outgoing values, invokes the core func, then constructs a `LiftContext` to lift incoming results. This replaces the old `createCanonLowerFunc` body.
- Host module creation for `canon.resource.new` / `canon.resource.drop` / `canon.resource.rep` binds the resource's `*runtime.ResourceType` at host-module-creation time (from `c.TypeDefs[canon.TypeIdx].Resource` + the instance's ResourceTypes pool). Calls `inst.rt.Table.NewResourceHandle(rep, true, rt)` / `Table.RemoveResourceHandle` / per-entry `RT`-pointer-identity check.

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

### Decision 4: Three latent `lift.go` gaps fixed per spec

All three fixes land in `abi/lift.go` and `runtime/table.go`.

**Gap 1 — `trap_if(not h.own)` in liftOwnHandle.** Spec `definitions.py:1338`. Current `abi/lift.go:665` calls `ctx.Instance.Table.Remove(h)` after only `ValidateType`; a borrow handle with a matching `RT` would be silently removed. Fix: after retrieving the entry (via `GetByIndex`, Gap 2), assert `entry.Own == true` before calling `Remove`. On failure, return `fmt.Errorf("lift own: handle %d is a borrow, not an own", handleIdx)`.

**Gap 2 — Generation-tag bridging via `Table.GetByIndex`.** Wazero's `runtime.Table` uses 64-bit generation-tagged handles. Wasm memory carries only the low 32 bits (the slot index). Current `abi/lift.go:660` constructs `runtime.Handle(handleIdx)` which is a 64-bit handle with generation = 0, so any recycled slot (generation > 0) is unreachable. Fix: add a new method on `runtime.Table`:

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

Exact implementation depends on the current `runtime/table.go` slot shape; the contract is "take a 32-bit index from Wasm, return the current generation-tagged handle + entry." Every `abi/lift.go` and `abi/lower.go` call site that currently constructs `runtime.Handle(handleIdx)` migrates to `Table.GetByIndex(handleIdx)`.

**Gap 3 — `ValOwn(handleIdx)` → `ValOwn(entry.Rep)`.** Spec `definitions.py:1339` (`return h.rep`). Current `abi/lift.go:668` returns `types.ValOwn(handleIdx)` — the Wasm-side index, not the rep. Fix: after `GetByIndex` + `Own` check + `Remove` (for own) or `IncrementLends` (for borrow), read `entry.Rep`, coerce to `uint32`, return `types.ValOwn(uint32(entry.Rep.(uint32)))` (or `ValBorrow`).

Wrinkle: `entry.Rep` is typed `any`. In spec terms, the rep is a u32. In wazero practice, host-managed resources store a Go pointer / any as the rep, not a u32. A `Val` must carry a 32-bit value (matching the Wasm flat ABI i32). Two sub-options:

- **Option A:** store the Wasm-side handle index in `ValOwn`/`ValBorrow` (matching the current broken behavior) but reinterpret the semantics: the `Val` carries the Wasm-side handle, and callers that need the rep re-query the table. This is a spec divergence.
- **Option B:** reinterpret `Rep` as `uint32`-or-pointer in a variant shape: host-managed resources store their pointer as an opaque uint32 (via a separate rep registry), and guest-managed resources store the guest's u32 rep directly. `ValOwn`/`ValBorrow` carry the guest-visible u32 rep. This matches the spec at `definitions.py:1339`.

The right choice: **Option B at the spec level, with a twist.** The spec's `h.rep` is always a `u32`, because the component-model resource rep IS a u32 passed from the guest. Host-managed resources have a separate code path — their `Rep` field is a Go pointer that represents host state, but the Wasm-side visible "rep" is the handle index itself (the core wasm function that created the handle returned its own index as the rep). In practice: for every own/borrow handle in the runtime table, the stored `entry.Rep` either IS a uint32 (guest-managed) OR the handle was minted by a wasip2 host module where the "rep" in the spec sense is the handle index.

**Chosen:** `liftOwnHandle` and `liftBorrowHandle` return `types.ValOwn(uint32(handleIdx))` / `types.ValBorrow(uint32(handleIdx))` with a documentation commitment that "for the component model call path, the rep carried by ValOwn/ValBorrow is the Wasm-side handle index; lift retrieves the entry via `Table.GetByIndex` and validates ownership / type, but the Val payload carries the index for round-trip by reference." This matches how real host-managed resources work in practice (wasip2's InputStream handle index IS the rep the guest sees) and makes cross-language serialization of a `Val` tractable. The spec's `h.rep` as a literal u32 only holds under the Python reference's in-memory model where Rep IS the integer; wazero's `any`-typed Rep needs the index level.

**But**: the missing validation (Gap 1) and generation bridging (Gap 2) still land verbatim per spec. Only Gap 3's choice deviates in the narrow sense that `ValOwn` carries the handle index by documented convention. A `// Spec: definitions.py:1339 — wazero convention: ValOwn/ValBorrow carry the Wasm-side handle index as the rep` comment is added to `types/val.go`'s `ValOwn` / `ValBorrow` constructors, and the wasmtime parallel (wasmtime's `ValRaw` also carries the handle index, not an unrelated rep) is cited.

### Decision 5: `Component.TypeDefs []TypeDef` restores the per-slot index

Add one field to `component.Component`:

```go
// internal/component/component.go
type Component struct {
    // ... existing fields ...

    // TypeDefs is one entry per type-section slot in the binary, in the
    // order the slots were decoded. Each entry's Kind discriminates what
    // kind of declaration lived at that slot; the kind-specific field
    // (Func / Resource / ValType / Instance / Component) points into
    // Component.Types (*ComponentTypes) via an interned index or is a
    // pointer to an InstanceTypeDef / ComponentTypeDef declaration.
    //
    // Every caller that previously used CanonicalDef.TypeIdx /
    // ImportExternDesc.TypeIdx / Export.TypeIdx / InstanceExport.TypeIdx
    // resolves the raw type-section index through this slice:
    //
    //   slot := c.TypeDefs[canon.TypeIdx]
    //   switch slot.Kind {
    //   case TypeDefKindFunc:     use slot.Func
    //   case TypeDefKindResource: use slot.Resource (ResourceTableIdx)
    //   case TypeDefKindDefined:  use slot.ValType
    //   ...
    //   }
    TypeDefs []TypeDef
}
```

The `TypeDef` struct already exists at `component.go:126–144` in the post-Session-0 shape. Session 1 populates it during decoding:

- Every type-section slot appends exactly one `TypeDef` to `dc.c.TypeDefs`.
- `decodeTypeSection` (at `binary/decoder.go:216`) gains a one-liner per case:

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

**Gotcha: the `*types.TypeFunc` pointer problem.** The canonical bag's `Funcs []TypeFunc` is a slice; appending to it invalidates pointers to earlier entries. The decoder populates `TypeDefs` incrementally during decoding (before the builder is finished), so any `&dc.c.Types.Funcs[ftIdx]` captured now will dangle after subsequent appends. **Resolution:** store `types.FuncTypeIdx` on `TypeDef.Func` as an index, not a pointer. But the existing `TypeDef.Func` is typed `*types.TypeFunc`. Options:

- **A:** change `TypeDef.Func` to `types.FuncTypeIdx` (a uint32 alias). Callers that want the `*TypeFunc` do `&c.Types.Funcs[td.Func]` after the bag is finished.
- **B:** defer population of `TypeDef.Func` until `builder.Finish()` has been called (the bag is frozen and pointers are stable). Store `types.FuncTypeIdx` in a temporary per-slot slice during decoding, then rewrite `TypeDefs` with `*TypeFunc` pointers after `Finish`.

**Chosen: option A.** `TypeDef.Func` becomes `types.FuncTypeIdx`. This removes the dangling-pointer footgun entirely and is more consistent with the other fields (`TypeDef.Resource` is already `types.ResourceTableIdx`, an index). All callers of `td.Func` update mechanically to `&c.Types.Funcs[td.Func]` (or a helper on `TypeDef`: `(td *TypeDef) FuncType(c *Component) *types.TypeFunc`).

**Deleted:** `decodeContext.funcTypeIdx` and `decodeContext.resourceDefs` maps on `binary/decoder.go`. Their callers inside the `binary/` package (if any) migrate to `c.TypeDefs[slot]`.

### Decision 6: `type_checker.go` — same-bag identity checks for Session 1

Session 1 fixes the two bugs where `checkFuncDefinition` / `checkInstanceDefinition` ignore the `expected` side entirely (`type_checker.go:192, 206`), replacing them with same-bag identity checks:

```go
func (tc *TypeChecker) checkFuncDefinition(expected *ImportExternDesc, actual Definition) error {
    fd, ok := actual.(*FuncDef)
    if !ok {
        return fmt.Errorf("expected function, got %T", actual)
    }
    // If the host did not provide a typed FuncDef, trust the host
    // (same-bag matching is not possible without a typed actual).
    if fd.Type == nil {
        return nil
    }
    // Resolve the expected type via c.TypeDefs (Decision 5).
    expectedTd := tc.component.TypeDefs[expected.TypeIdx]
    if expectedTd.Kind != TypeDefKindFunc {
        return fmt.Errorf("import type %d is not a function type", expected.TypeIdx)
    }
    expectedFT := &tc.component.Types.Funcs[expectedTd.Func]
    if !funcTypesMatchSameBag(expectedFT, fd.Type) {
        return fmt.Errorf("function type mismatch")
    }
    return nil
}

// funcTypesMatchSameBag compares two *types.TypeFunc that both live in
// the same *types.ComponentTypes bag via identity on Params / Results
// (both are interned tuple ValTypes). Works only when both sides share
// the same bag; cross-bag structural comparison is Session 2 work.
func funcTypesMatchSameBag(a, b *types.TypeFunc) bool {
    if a == nil || b == nil {
        return a == b
    }
    if a.Async != b.Async {
        return false
    }
    if a.Params != b.Params || a.Results != b.Results {
        return false
    }
    if len(a.ParamNames) != len(b.ParamNames) {
        return false
    }
    for i := range a.ParamNames {
        if a.ParamNames[i] != b.ParamNames[i] {
            return false
        }
    }
    return true
}
```

The existing `checkFuncType(expected, actual *types.TypeFunc)` (at `type_checker.go:42–58`) already does same-bag identity on `Params`/`Results`. Session 1 extends it only to handle the new `ParamNames` comparison; the body otherwise stays.

`checkInstanceDefinition` gets an analogous same-bag identity check that walks `InstanceTypeDef.Declarations` and ensures each required export exists in the host-provided `*InstanceDef`. The current Session 0 stub's loop (`type_checker.go:76–97`) is already close; Session 1 ensures the `expected` side is read from `c.TypeDefs[expected.TypeIdx]`.

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
    if resEntry.RT != rt {
        return 0, fmt.Errorf("resource.rep: type mismatch")   // spec :2172
    }
    return resEntry.Rep.(uint32), nil
}

func (i *Instance) ResourceDrop(resourceIdx types.ResourceIdx, handleIdx uint32) error {
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
    if resEntry.RT != rt {
        return fmt.Errorf("resource.drop: type mismatch")   // spec :2147
    }
    if resEntry.NumLends != 0 {
        return fmt.Errorf("resource.drop: handle has %d outstanding lends", resEntry.NumLends)   // spec :2148
    }
    // Remove from table and invoke destructor per spec :2149-2161.
    if _, err := i.rt.Table.Remove(h); err != nil {
        return err
    }
    if resEntry.Own && rt.HasDestructor() {
        // Local destructor — spec :2151. Cross-instance destructor
        // invocation via canon_lift (spec :2154-2160) is Session 2
        // work and traps.
        if rt.Impl != i.rt {
            return fmt.Errorf("resource.drop: cross-instance destructor invocation is session 2 work")
        }
        invokeLocalDestructor(i, rt, resEntry.Rep)
    }
    return nil
}
```

The `invokeLocalDestructor` helper reads `rt.Dtor` (a `*uint32` core function index) from the defining instance's core module and calls it with the rep. For host-managed resources (where `rt.Dtor == nil` and destruction flows through the `runtime.Destroyable` interface), the `Table.Remove` call already invokes `Destroy()` if the entry's `Rep` implements `Destroyable` — so `invokeLocalDestructor` is a no-op for host-managed resources, and the `rt.HasDestructor()` check gates only guest-side destructors.

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
func (i *Instance) EnterCall()                  { i.rt.Enter() }
func (i *Instance) ExitCall()                   { i.rt.Leave() }
func (i *Instance) Table() *runtime.Table       { return i.rt.Table }
func (i *Instance) Parent() *Instance           { return i.parent }
func (i *Instance) CallMightBeRecursive(caller *Instance) bool {
    return caller == i && i.rt.EnterCount() > 0
}
```

`ValidateMayLeave` and `ValidateNotRecursive` keep their existing shape, delegating through the new accessors.

## Instantiate Pipeline — Top-Level Shape

`ComponentLinker.Instantiate` is rebuilt end-to-end. The top-level shape (abbreviated; full plumbing in the implementation plan):

```go
func (l *ComponentLinker) Instantiate(ctx context.Context, compiled *CompiledComponent) (*Instance, error) {
    c := compiled.Internal()
    compiledModules := compiled.CompiledModules()

    // 1. Allocate instance + runtime.ComponentInstance.
    inst := newInstance(c, l.nextInstanceID(), nil)

    // 2. Local Concrete promotion: mint *runtime.ResourceType per declared
    //    resource, populate inst.rt.ResourceTypes. Decision 2.
    if err := l.promoteLocalResources(inst, c); err != nil {
        return nil, fmt.Errorf("resource promotion: %w", err)
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

`canon.lift` creates a component function from a core wasm function. The component function's body lifts arguments, calls the core func, and lowers results. Session 1's implementation:

```go
// buildCanonLiftFunc creates the closure that implements a canon.lift
// component function. It constructs LiftContext / LowerContext per call
// using inst.rt and the calling instance's types bag.
func (l *ComponentLinker) buildCanonLiftFunc(
    inst *Instance,
    canon *CanonicalDef,
    coreFunc api.Function,
    funcType *types.TypeFunc,
    memory api.Memory,
    realloc api.Function,
) func(ctx context.Context, args []types.Val) ([]types.Val, error) {
    return func(ctx context.Context, args []types.Val) ([]types.Val, error) {
        // CallContext is per-call state: allocated fresh here, lives for the
        // duration of this canon.lift call, closed at the end. Not stored on
        // Instance (Decision 3 deleted Instance.CallContext / SetCallContext).
        callCtx := runtime.NewCallContext()
        lowerCtx := &abi.LowerContext{
            Memory:      memory,
            Opts:        buildOptions(canon, memory, realloc),
            Realloc:     reallocAdapter(realloc),
            Types:       inst.component.Types,
            Instance:    inst.rt,
            CallContext: callCtx,
        }
        liftCtx := &abi.LiftContext{
            Memory:      memory,
            Opts:        lowerCtx.Opts,
            Types:       inst.component.Types,
            Instance:    inst.rt,
            BorrowScope: runtime.NewBorrowScope(inst.rt.Table),
        }

        // Lower args to flat core values.
        paramTypes := unpackTupleParams(inst.component.Types, funcType.Params)
        var flatArgs []uint64
        for idx, arg := range args {
            flat, err := abi.LowerFlat(lowerCtx, paramTypes[idx], arg)
            if err != nil { return nil, err }
            flatArgs = append(flatArgs, flat...)
        }

        // Call core wasm.
        flatResults, err := coreFunc.Call(ctx, flatArgs...)
        if err != nil { return nil, err }

        // Lift results.
        resultTypes := unpackTupleParams(inst.component.Types, funcType.Results)
        iter := abi.NewFlatIter(flatResults)
        var results []types.Val
        for _, rt := range resultTypes {
            v, err := abi.LiftFlat(liftCtx, rt, iter)
            if err != nil { return nil, err }
            results = append(results, v)
        }

        // Post-return (spec definitions.py:1999-2002).
        if postReturn := canonPostReturn(inst, canon); postReturn != nil {
            inst.rt.MayLeave = false
            _, err := postReturn.Call(ctx, flatResults...)
            inst.rt.MayLeave = true
            if err != nil { return nil, err }
        }

        // Borrow scope cleanup — trap if any borrow outstanding.
        if err := liftCtx.BorrowScope.Close(); err != nil {
            return nil, fmt.Errorf("outstanding borrows at return: %w", err)
        }

        return results, nil
    }
}
```

`unpackTupleParams` resolves `funcType.Params` (which is a `ValType` of kind `TypeKindTuple`) to a flat `[]types.ValType` for the tuple's element types. `buildOptions` constructs an `abi.Options` struct from the `CanonicalDef.Options`. `canonPostReturn` looks up the post-return function if `canon.Options.PostReturnIdx != nil`.

The order of operations — `may_leave = false` before post-return, post-return invoked, `may_leave = true` restored, then borrow scope closed — matches spec at `definitions.py:1999–2003` and wasmtime at `func.rs:808–834` (verified via the Session 1 discovery agent report).

### canon.lower wiring

`canon.lower` creates a core wasm function from a component function. The core function's body lifts arguments from flat core values, calls the component function, and lowers results back to core values. Symmetric to canon.lift but the `LiftContext` / `LowerContext` roles flip.

```go
// createCanonLowerFunc produces the api.GoModuleFunc that implements a
// canon.lower core function.
func (l *ComponentLinker) createCanonLowerFunc(
    ctx context.Context,
    inst *Instance,
    c *Component,
    info canonLowerInfo,
    compFunc ComponentFunc,
    needsRetptr bool,
) api.GoModuleFunc {
    return api.GoModuleFunc(func(goCtx context.Context, mod api.Module, stack []uint64) {
        memory := mod.Memory()
        realloc := resolveRealloc(mod, info.options)

        // Lift core values → component Vals.
        paramTypes := unpackTupleParams(c.Types, compFunc.Type.Params)
        iter := abi.NewFlatIter(stack)
        liftCtx := &abi.LiftContext{
            Memory:      memory,
            Opts:        buildOptions(info.options, memory, realloc),
            Types:       c.Types,
            Instance:    inst.rt,
            BorrowScope: runtime.NewBorrowScope(inst.rt.Table),
        }
        var args []types.Val
        for _, pt := range paramTypes {
            v, err := abi.LiftFlat(liftCtx, pt, iter)
            if err != nil { panic(err) }
            args = append(args, v)
        }

        // Call the component function.
        results, err := compFunc.Impl(goCtx, args)
        if err != nil { panic(err) }

        // Lower results → core values.
        lowerCtx := &abi.LowerContext{
            Memory:      memory,
            Opts:        liftCtx.Opts,
            Realloc:     reallocAdapter(realloc),
            Types:       c.Types,
            Instance:    inst.rt,
            CallContext: runtime.NewCallContext(),
        }
        resultTypes := unpackTupleParams(c.Types, compFunc.Type.Results)
        if needsRetptr {
            // Results go to memory via the retptr param.
            retptr := uint32(stack[len(paramTypes)])
            for idx, rt := range resultTypes {
                if err := abi.LowerHeap(lowerCtx, rt, results[idx], retptr); err != nil { panic(err) }
                retptr += rt.ABI(c.Types).Size32
            }
        } else {
            // Results go in stack positions starting at 0.
            pos := 0
            for idx, rt := range resultTypes {
                flat, err := abi.LowerFlat(lowerCtx, rt, results[idx])
                if err != nil { panic(err) }
                for _, v := range flat {
                    stack[pos] = v
                    pos++
                }
            }
        }

        // Close borrow scope (lift-side scope). Borrows created during
        // lifting are resolved when this call returns; any outstanding
        // lends are a trap.
        if err := liftCtx.BorrowScope.Close(); err != nil { panic(err) }
    })
}
```

### Nested component instantiation

`instantiateNestedComponent` is restored to rebuild a nested `*Instance` with its own `*runtime.ComponentInstance` (parent pointing at the outer instance's `rt`). Argument resolution via `resolveFromParentScope` walks the parent's component function / instance / type / component spaces, unchanged in structure from the pre-Session-0 body but retargeted to the new types. `resolveExportTypeAlias` (the Session 0 panic stub) is rebuilt to walk `parent.component.TypeDefs` + the alias's source instance's exports, returning a `*TypeDef` that points into the canonical bag.

## Resource Identity — Local Concrete Promotion

Session 1 populates `inst.rt.ResourceTypes` at `Instantiate` time. The walk is driven by `compiled.Types.ResourceTables`:

```go
func (l *ComponentLinker) promoteLocalResources(inst *Instance, c *Component) error {
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
        }
        inst.rt.ResourceTypes = append(inst.rt.ResourceTypes, rt)
    }
    return nil
}
```

`TypeDef` gains three new fields for resource declarations:

```go
type TypeDef struct {
    Kind     TypeDefKind
    Func     types.FuncTypeIdx   // Decision 5 — changed from *types.TypeFunc
    Resource types.ResourceTableIdx
    ValType  types.ValType
    Instance  *InstanceTypeDef
    Component *ComponentTypeDef

    // Resource destructor metadata (populated for Kind==TypeDefKindResource).
    // Session 1 adds these so promoteLocalResources can wire Dtor without
    // a second pass over the decoder state.
    ResourceDtor         *uint32
    ResourceDtorAsync    bool
    ResourceDtorCallback *uint32
}
```

### Mapping TypeResourceTable.Resource to ResourceTypes slice index

In the canonical bag, `TypeResourceTable` has two modes (Concrete vs Abstract — design doc Decision 5 in Session 0). Session 1 uses only the Abstract mode at decode time and maps it to a concrete instance-owned `*ResourceType` via a simple convention:

**Convention:** `TypeResourceTable.Resource` in the Abstract entry is the resource's declaration-order index within the component. `inst.rt.ResourceTypes` is populated in the same declaration order by `promoteLocalResources`, so `inst.rt.ResourceTypes[TypeResourceTable.Resource]` is the matching `*ResourceType`.

**Verification:** the canonical bag's `ResourceTables` slice is populated by `builder.InternAbstractResource()` (one call per resource declaration in scope order), and `promoteLocalResources` walks the same slice in the same order. Order parity is maintained by construction.

### Lift dispatch adjustment

`abi/lift.go:liftOwnHandle` / `liftBorrowHandle` stop trapping on `!rt.Concrete`. Instead they rely on `ctx.Instance.LookupResourceType(rt.Instance, rt.Resource)` returning non-nil. In Session 1:

- `rt.Instance` is `types.RuntimeComponentInstanceIdx(ctx.Instance.ID)` for the same-instance case — the `Abstract` entry's `Instance` field is populated when `promoteLocalResources` runs (or left zero; the `LookupResourceType` call walks by ID, and for a same-instance call the ID matches).
- For cross-instance resource handles (declared in a different instance), `LookupResourceType` returns nil (Session 1 does not maintain a cross-instance registry) and the lift path traps with the precise error.

The `!rt.Concrete` trap check is removed from `liftOwnHandle` and `liftBorrowHandle`. The trap branch moves into the `expectedRT == nil` check which now also catches the cross-instance case.

## Decoder → Linker Indirection — `Component.TypeDefs`

Full decoder change: every `decodeTypeSection` case that previously populated `dc.funcTypeIdx` or `dc.resourceDefs` instead (or also) appends a `TypeDef` to `dc.c.TypeDefs`. The private maps are deleted.

For resource declarations, `decodeResourceDecl` returns a `ResourceTypeDef` with destructor metadata; the decoder extracts `Dtor`, `DtorAsync`, `DtorCallback` and stores them on the `TypeDef` (Decision 2 adjustment).

For function declarations, the decoder stores `types.FuncTypeIdx` directly (not a pointer into the bag; Decision 5 option A).

For value types (records, variants, lists, etc.), the decoder stores the `types.ValType` returned by the matching `decode<Kind>` helper.

For instance / component type declarations, the decoder stores a `*InstanceTypeDef` / `*ComponentTypeDef` pointer (the existing shape from pre-Session-0).

All callers:

- `component_linker.go::Instantiate` and its helpers — use `c.TypeDefs[canon.TypeIdx]` for canon operations.
- `type_checker.go::checkFuncDefinition` / `checkInstanceDefinition` — use `c.TypeDefs[expected.TypeIdx]` for expected-type lookup.
- `instance.go::ResourceNew`/`ResourceRep`/`ResourceDrop` — take `types.ResourceIdx` directly; resolution via `c.TypeDefs` happens in the wiring layer (`createResourceOpExport`).
- `nested_component.go::resolveExportTypeAlias` — walks `parent.component.TypeDefs` to resolve a type alias export.
- Test code in `component_linker_test.go`, `linker_test.go`, `nested_component_test.go`, etc. — test fixtures build small Components with hand-populated `TypeDefs` slices.

## Canon Resource Ops — Host Module Export Shapes

`createResourceNewExport`, `createResourceDropExport`, `createResourceRepExport` are restored with the Session 1 signatures (Decision 7). Core wasm signatures (unchanged from the spec):

- `canon resource.new $T` : `(i32 rep) -> i32 handle`
- `canon resource.drop $T` : `(i32 handle) -> ()`
- `canon resource.rep $T` : `(i32 handle) -> i32 rep`

Each host module export is keyed by the resource's declaration index, resolved once at host-module creation time from `c.TypeDefs[canon.TypeIdx].Resource`. The closure captures the resolved `types.ResourceIdx` so the runtime call is index-free.

Error handling: traps at the Wasm level. The Go body panics with a typed error that the core-wasm host-function adapter converts to a trap. This matches the pre-Session-0 pattern for other canon ops and wazero's `wasmruntime.Trap*` conventions.

## `abi/lift.go` Gap Fixes — Concrete Patch Sites

**Gap 1 — `liftOwnHandle` missing `entry.Own` check.** Current lift.go:638–669. New body:

```go
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

    // Spec definitions.py:1334 — cx.inst.table.remove(i).
    // Spec definitions.py:1345 — trap_if(h.rt is not t.rt).
    // Spec definitions.py:1337 — trap_if(h.num_lends != 0).
    // Spec definitions.py:1338 — trap_if(not h.own).
    h, entry, err := ctx.Instance.Table.GetByIndex(handleIdx)   // Gap 2 fix
    if err != nil {
        return types.Val{}, fmt.Errorf("lift own: %w", err)
    }
    resEntry, ok := entry.(*runtime.ResourceHandleEntry)
    if !ok {
        return types.Val{}, fmt.Errorf("lift own: handle %d is not a resource handle", handleIdx)
    }
    if resEntry.RT != expectedRT {
        return types.Val{}, fmt.Errorf("lift own: resource type mismatch")   // spec :1345
    }
    if resEntry.NumLends != 0 {
        return types.Val{}, fmt.Errorf("lift own: handle has %d outstanding lends", resEntry.NumLends)   // spec :1337
    }
    if !resEntry.Own {
        return types.Val{}, fmt.Errorf("lift own: handle %d is a borrow, not an own", handleIdx)   // spec :1338 — Gap 1 fix
    }
    if _, err := ctx.Instance.Table.Remove(h); err != nil {
        return types.Val{}, fmt.Errorf("lift own: %w", err)
    }
    // Spec :1339 — return h.rep. Wazero convention: the Wasm-side handle
    // index is the rep for the component-call path. Documented in
    // types/val.go ValOwn constructor.
    return types.ValOwn(handleIdx), nil   // Gap 3 decision — see Decision 4
}
```

**Gap 2 — `Table.GetByIndex` added.** New method on `runtime/table.go`:

```go
// GetByIndex looks up a table entry by slot index (the low 32 bits of a
// generation-tagged Handle). Returns the current generation-tagged
// Handle alongside the entry, so the caller can perform further
// operations (Remove, IncrementLends) using the full Handle.
//
// Used by abi/lift.go to bridge the 32-bit Wasm-side handle index to
// the runtime's 64-bit generation-tagged handle space.
//
// Returns ErrInvalidHandle if the slot is out of range or currently free.
func (t *Table) GetByIndex(idx uint32) (Handle, TableEntry, error) {
    // Implementation depends on the current Table slot layout; the
    // contract is "idx (Wasm-side u32) -> (full Handle, entry, nil) |
    // (0, nil, ErrInvalidHandle)".
}
```

**Gap 3 — `ValOwn` convention documented, behavior preserved.** No change to `types.ValOwn` / `types.ValBorrow` bodies — they continue to accept a `uint32` and store it as the payload. A comment block on each constructor cites the spec line (`definitions.py:1339, 1347`) and explicitly documents the wazero convention: the payload IS the Wasm-side handle index.

## Type Checker Scope — Session 1

`checkFuncType` (current `type_checker.go:42–58`) is extended to compare `ParamNames` in addition to the existing `Async`/`Params`/`Results` identity check.

`checkFuncDefinition` (current `type_checker.go:187–194`) is rewritten to:
1. Type-assert `actual` to `*FuncDef`.
2. If `actual.(*FuncDef).Type == nil`, trust the host.
3. Otherwise, resolve `expected.TypeIdx` via `tc.component.TypeDefs[expected.TypeIdx]`, assert it's a function type, pull the `*types.TypeFunc` via `&tc.component.Types.Funcs[td.Func]`, and call `checkFuncType(expectedFT, actual.Type)`.

`checkInstanceDefinition` (current `type_checker.go:201–208`) is rewritten to:
1. Type-assert `actual` to `*InstanceDef`.
2. If `actual.(*InstanceDef).SkipValidation`, return nil.
3. Otherwise, resolve `expected.TypeIdx` via `tc.component.TypeDefs[expected.TypeIdx]`, assert it's an instance type, pull the `*InstanceTypeDef`, and call the existing `checkInstance` body (which already walks declarations against exports).

Session 2 (structural walk for cross-bag matching) is unchanged.

## Test Restoration Methodology

Each checkpoint in Section "Work Order" below has a corresponding batch of tests to restore. The pattern per test file:

1. Identify the file's pre-Session-0 body:

   ```bash
   git show 98b3bbc3:internal/component/instance_test.go > /tmp/old_instance_test.go
   ```

2. Read the old body and catalog its test functions + assertions.

3. For each test function:
   - **Rewrite the body** against new types/abi/runtime.
   - **Cite upstream** (`definitions.py` line, wasmtime test, or "no counterpart" rationale) as a comment at the top of the test.
   - **Validate assertions**: if any assertion would contradict the spec or wasmtime's observable behavior, rework the assertion or mark the test for deletion.
   - **Delete skip-placeholder**.

4. Run the test file: `go test ./internal/component/ -run TestX`. Failures trigger either a bug in the restored test or a bug in the production code; fix one, then re-run.

5. Commit the test file restoration as part of the matching checkpoint task.

The per-task spec-compliance review subagent verifies citations and reasoning for deleted/reworked tests.

### Bounds-check test harness ([]byte-backed memory)

The 11 bounds-check tests in `abi/context_test.go` + `abi/strings_test.go` fail because `wazerotest.NewMemory` rounds to page size. Session 1 adds a minimal `byteMemory` test helper in `abi/memory_test_helper.go`:

```go
// byteMemory is a direct []byte-backed api.Memory implementation for
// bounds-check tests that need to construct memories smaller than a
// page. Does not support Grow or any non-read/write op.
type byteMemory struct {
    data []byte
}

func (m *byteMemory) Size() uint32 { return uint32(len(m.data)) }
func (m *byteMemory) Read(offset, length uint32) ([]byte, bool) {
    if uint64(offset)+uint64(length) > uint64(len(m.data)) {
        return nil, false
    }
    return m.data[offset : offset+length], true
}
func (m *byteMemory) Write(offset uint32, data []byte) bool {
    if uint64(offset)+uint64(len(data)) > uint64(len(m.data)) {
        return false
    }
    copy(m.data[offset:], data)
    return true
}
// ... ReadByte, WriteByte, ReadUint32Le, WriteUint32Le, etc. as api.Memory requires
```

The 11 tests use `&byteMemory{data: make([]byte, 3)}` (or similar short sizes) to construct memories that trigger bounds errors on 4-byte reads.

## Work Order + Checkpoint Gates

The implementation plan (separate doc at `docs/superpowers/plans/2026-04-08-canonical-abi-session1-plan.md`) numbers tasks linearly. This section names the checkpoint gates and the task groups that feed each gate.

| Checkpoint | Scope | Success criterion |
|---|---|---|
| **A — `Component.TypeDefs` + decoder exposure** | Add `TypeDef` with `ResourceDtor*` fields + `Component.TypeDefs []TypeDef`. Populate in decoder. Delete `decodeContext.funcTypeIdx` / `resourceDefs`. Restore 18 decoder tests in `binary/component_type_test.go` + `binary/instance_type_test.go`. | `go build ./internal/component/binary/... ./internal/component/...` green. `go test ./internal/component/binary/...` green including the 18 restored tests. |
| **B — `component.Instance` embeds `*runtime.ComponentInstance`** | Delete duplicated fields. Rewrite delegators. Update every `i.table`/`i.mayLeaveDisabled`/etc. call site. Compile-fix every caller in `internal/component/`, `imports/wasip2/`, test files. | `go build ./internal/component/... ./imports/wasip2/...` green. Accessor tests in `instance_test.go` pass. |
| **C — `Instantiate` top-level + canon.lift/lower/resource wiring + primitive conformance** | Delete Instantiate/coreSignature stubs. Rebuild Instantiate (step 1-12 of pipeline). Rebuild buildComponentFuncs, instantiateCoreModule, createCanonLowerExport, createResourceNewExport/DropExport/RepExport, createAliasExport, wireExportedFunc. Restore primitive + composite + string + abi_edge_cases conformance tests. Restore linker_test.go + linker_api_test.go. | `go test ./internal/component/conformance/ -run 'Primitive|Composite|String|ABIEdge'` green. `go test ./internal/component/ -run 'Linker'` green. |
| **D — Nested components + resolveExportTypeAlias + integration tests** | Delete nested_component.go panic stub. Rebuild resolveExportTypeAlias + instantiateNestedComponent + wireNestedComponentExports + createInlineInstanceModule. Restore nested_component_test.go (21 tests), integration_test.go (19 tests), start_function_test.go (9 tests), component_linker_test.go (8 tests). | `go test ./internal/component/ -run 'Nested|Integration|StartFunction|ComponentLinker'` green. |
| **E — Local Concrete promotion + 3 lift.go fixes + resource conformance** | Implement `promoteLocalResources`. Add `Table.GetByIndex`. Fix 3 lift.go gaps in `liftOwnHandle` / `liftBorrowHandle`. Rewrite `Instance.ResourceNew`/`ResourceRep`/`ResourceDrop` to new signatures. Restore conformance/resources_test.go + destructor_test.go + resource_generation_test.go. Restore instance_test.go's resource-related tests (12 tests). Restore 11 abi/ bounds-check tests via byteMemory helper. | `go test ./internal/component/conformance/ -run 'Resources|Destructor|ResourceGeneration'` green. `go test ./internal/component/ -run 'CanonResource'` green. `go test ./internal/component/abi/ -run 'BoundsCheck'` green. |
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
- `TypeDef.Func` changes from `*types.TypeFunc` to `types.FuncTypeIdx` (Decision 5 option A).

**`internal/component/runtime/table.go`:**
- `Table.GetByIndex(idx uint32) (Handle, TableEntry, error)` — new method for Gap 2.

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

### V4 — All restored tests cite upstream

Per-task spec-compliance reviewer verifies every restored test has a comment block citing `definitions.py` line / wasmtime test file:line / wazero-invariant rationale.

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

### V7 — Three latent lift.go gaps closed

- `grep -n 'resEntry.Own' internal/component/abi/lift.go` → shows the `trap_if !Own` check in `liftOwnHandle`.
- `grep -n 'GetByIndex' internal/component/abi/lift.go` → shows `Table.GetByIndex` usage.
- `grep -n 'Table.GetByIndex\|func .Table. GetByIndex' internal/component/runtime/table.go` → confirms the method exists.

### V8 — Instance resource ops match spec signatures

- `ResourceNew(resourceIdx types.ResourceIdx, rep uint32) (uint32, error)` — grep confirms.
- `ResourceRep(resourceIdx types.ResourceIdx, handleIdx uint32) (uint32, error)` — grep confirms.
- `ResourceDrop(resourceIdx types.ResourceIdx, handleIdx uint32) error` — grep confirms.

### V9 — `c.TypeDefs` exists and is populated

```bash
grep -n 'TypeDefs \[\]TypeDef' internal/component/component.go
```

Returns the field definition.

```bash
grep -n 'c\.TypeDefs = append' internal/component/binary/decoder.go
```

Returns the decoder population sites.

### V10 — Local Concrete promotion runs during Instantiate

```bash
grep -n 'promoteLocalResources' internal/component/component_linker.go
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
