# Loop 2 — Wire `abi/` into production, delete dead code

> **Status:** blocked on Loop 1
>
> **Goal:** Production runtime calls `abi.LiftValues`/`abi.LowerValues`
> (Loop 1 item 25) for every lift/lower operation. The three parallel
> implementations are deleted along with their tests. The ~85 silent-
> default sites in wasip2 sockets/http trap or return
> `result.err(...)` correctly. After this loop,
> `internal/component/{instance.go,component_linker.go,canon_lower.go,
> linker.go}` contain only ORCHESTRATION (subtask, borrow scope,
> may_leave, post_return, reentrance, enter_call/exit_call); the
> lift/lower MATH lives exclusively in `abi/`. This matches wasmtime's
> three-layer separation: `vm/component/resources.rs` (low-level state)
> + `runtime/component/values.rs` (Val + lift/lower math, no lifecycle)
> + `runtime/component/func.rs` (orchestration wrapper).
>
> **Total items:** 16 across 6 phases
>
> Items must be worked in numerical order within a phase. Phase 2.A must
> complete before any item in 2.B–2.E starts. Phase 2.F items 12–16 are
> the terminal sweep and run last.

---

## Phase 2.A — Mapping (1 item)

### Item 1: Map every lift/lower call site and dependent test file

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** -

**Files:**
- Create: `docs/plans/projects/abi-unification/loop-2-call-site-map.md`
- Read (no modification): `internal/component/instance.go`,
  `internal/component/component_linker.go`,
  `internal/component/canon_lower.go`, `internal/component/linker.go`,
  `internal/component/linker_api.go`,
  `internal/component/value_import_test.go`,
  `internal/component/type_resolver.go`

**Spec authorities:**
- N/A — this is a research/mapping item, not a code change item

**Description:**
This item produces no production code. It produces a single Markdown
document, `loop-2-call-site-map.md`, that lists every lift/lower call
site in the production component code AND every test file that exercises
any of those functions. The document is the input for items 2-7 (which
each replace one or more of these call sites) and item 12 (which uses
the test-file list to know which tests must be deleted along with their
subjects).

**Pre-verified function list (all line numbers verified by Grep):**

| File | Line | Function | Operation |
|---|---|---|---|
| component_linker.go | 2430 | `(l *ComponentLinker) createCanonLowerFunc` | host import lower closure |
| component_linker.go | 2547 | `liftFromStack` | core stack → Val for record/option/variant |
| component_linker.go | 2681 | `liftRecordFromStack` | recursive helper |
| component_linker.go | 2694 | `liftOptionFromStack` | recursive helper |
| component_linker.go | 2709 | `liftVariantFromStack` | recursive helper |
| component_linker.go | 2726 | `liftValFromMemory` | heap lift helper |
| component_linker.go | 2797 | `liftRecordFromMemory` | recursive heap helper |
| component_linker.go | 2815 | `liftOptionFromMemory` | recursive heap helper |
| component_linker.go | 2991 | `liftListFromMemory` | recursive heap helper |
| component_linker.go | 3020 | `flatSlotCount` | layout helper |
| component_linker.go | 3074 | `lowerToStack` | Val → core stack for non-retptr results |
| component_linker.go | 3157 | `writeResultsToMemory` | retptr result writer (recursive) |
| component_linker.go | 3369 | `writeRecordToMemory` | recursive heap writer |
| component_linker.go | 3387 | `writeValToMemory` | recursive heap writer |
| component_linker.go | 3625-3797 | `flattenValType`, `flattenRecordType`, `flattenTupleType`, `flattenOptionType`, `flattenResultType`, `flattenFlagsType`, `flattenVariantType`, `valueTypeWidth`, `isWiderValueType`, `componentTypeToCoreTypes` | flatten family |
| canon_lower.go | LoweredFunc.CallWithStack | dead host import path |
| canon_lower.go | various | private helpers (already deleted in Loop 1 item 9.5) |
| instance.go | 305-322 | retptr-as-PARAM detection block in ExportedFunc.Call |
| instance.go | 335-338 | retptr-as-RESULT synthesis (separate from 305-322 — DO NOT delete in Loop 2; this is part of the orchestration that stays) |
| instance.go | 442 | `f.liftResolvedType(typeRef, ...)` call site in ExportedFunc.Call |
| instance.go | 501 | `rec := f.liftRecord(...)` call site in legacy fallback |
| instance.go | 757 | `liftRecord` (alphabetical sort at 765) |
| instance.go | 794 | `liftResolvedType` |

**Pre-verified test files that test the deleted functions** (item 12
will delete these along with their subjects):

| Test file | Tests |
|---|---|
| instance_test.go | `TestLiftResolvedType_RecordRetptr` (2149), `TestLiftResolvedType_RecordFlat` (2177), `TestLiftResolvedType_LargeRecordRetptr` (2198), and any `TestLiftRecord*` |
| component_linker_test.go | every test referencing `liftFromStack`, `lowerToStack`, `writeResultsToMemory`, `writeValToMemory`, `writeRecordToMemory`, `flattenVariantType`, `componentTypeToCoreTypes` |
| canon_lower_test.go | already deleted in Loop 1 item 9.5 |
| resource_table_test.go | `TestResourceTable_CreateResourceDropFunc`, `TestResourceTable_CreateResourceDropFunc_InvalidHandle`, `TestResourceTable_CreateResourceDropFunc_NilDestructor`, `TestResourceTable_CreateResourceRepFunc`, `TestResourceTable_CreateResourceRepFunc_InvalidHandle` |
| wasip2test/kv_store_test.go | line 208 calls `CreateResourceDropFunc` (silent variant) — must be migrated, not deleted |

This item's role is to **verify the above tables match current source**
(line numbers shift as commits land) and produce a final
`loop-2-call-site-map.md` that items 2-12 consume.

The map format is:

```markdown
| File | Line | Function | Operation | To be replaced by |
|---|---|---|---|---|
| component_linker.go | 2430 | createCanonLowerFunc body | host import lower | abi.LiftValues + abi.LowerValues |
| component_linker.go | 2547 | liftFromStack | lift core stack | abi.LiftValues |
| ... |
```

And a second table with one row per test file that depends on a function
that will be deleted:

```markdown
| Test file | Tests deleted/migrated function | Action |
|---|---|---|
| component_linker_test.go | TestLiftFromStack* | delete in item 6 |
| ... |
```

**Definition of done:**
- `loop-2-call-site-map.md` exists in
  `docs/plans/projects/abi-unification/`
- Both tables are filled with at least the file:line:function granularity
- Every entry in the call-site table cross-references the Loop 2 item
  that will replace it (item 2, 3, 4, 5, 6, 7, 8, or 9)
- Every entry in the test-file table cross-references the Loop 2 item
  that will delete or migrate it
- Reviewer subagent verifies completeness against fresh `Grep` output:
  for each file in the Files Read list, run Grep for `func .*lift|func
  .*lower|writeRecordToMemory|writeValToMemory|writeResultsToMemory|
  liftRecord|liftFromStack|liftResolvedType|flattenVariantType|
  isWiderValueType|createCanonLowerFunc|LoweredFunc.*CallWithStack` and
  confirm every match is in the document

**Reviewer focus areas:**
- Spec compliance: N/A (no spec change)
- Code quality: completeness — the reviewer must run their own Grep and
  cross-check every match exists in the document. Missing entries are
  BLOCKERs because items 2-9 will produce broken work if their input
  list is incomplete.

---

## Phase 2.B — Wire host-import path (3 items)

### Item 2: Rewrite `LoweredFunc.CallWithStack` body as a lifecycle wrapper around abi.LiftValues / abi.LowerValues

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Depends on Loop 1 items 9.5 (canon_lower.go cleanup), 9.7 (package boundary), 24 (Own/Borrow dispatch), 25 (LiftValues/LowerValues). The body becomes a may_leave-aware lifecycle wrapper, not a one-line shim.

**Files:**
- Modify: `internal/component/canon_lower.go` — replace the body of
  `(f *LoweredFunc) CallWithStack(ctx context.Context, stack []uint64) ([]uint64, error)`
  (note the actual signature returns `([]uint64, error)`, not `error`)
- Modify: `internal/component/canon_lower_test.go` — adjust tests to
  exercise the new shim path; delete tests that asserted intermediate
  helpers' behavior

**Spec authorities:**
- `definitions.py:1978-2063` — `canon_lift` (for the host's
  return-value lifting)
- `definitions.py:2064-...` — `canon_lower` (for the host's
  parameter lowering)
- `crates/wasmtime/src/runtime/component/func/host.rs` — wasmtime's
  host import wrapper (`HostFn::cabi_entrypoint`, `call_sync_lower`,
  `lower_result_and_exit_call`). This is the architectural model.

**Description:**
`LoweredFunc.CallWithStack` is the dead host-import path. The
existing body has per-type case logic for lift/lower. The new body is
a **lifecycle wrapper** around `abi.LiftValues` (for params) and
`abi.LowerValues` (for results), matching wasmtime's
`call_sync_lower` shape.

```go
// LoweredFunc.CallWithStack matches the actual current signature:
//   (f *LoweredFunc) CallWithStack(ctx context.Context, stack []uint64) ([]uint64, error)
//
// This is the host-import side of the canonical ABI: a guest is
// invoking a host function. The wasm core stack contains the params
// (already in flat form). We lift them, call the host, then lower
// the host's results back.
func (f *LoweredFunc) CallWithStack(ctx context.Context, stack []uint64) ([]uint64, error) {
    // Lifecycle: check may_leave (the spec requires that host imports
    // may not be invoked while the guest is in post-return etc.).
    // This is the wasmtime func/host.rs:292 may_leave check.
    if !f.callerInstance.MayLeave() {
        return nil, fmt.Errorf("cannot leave component during post-return")
    }

    // Build the LiftContext (memory, options, resource tables — no
    // lifecycle).
    lcx := abi.NewLiftContext(f.memory, f.options, f.resourceTable)

    // Push a borrow scope before lifting params (wasmtime
    // host.rs:464). The scope ensures borrows are tracked across
    // the host call.
    lcx.EnterCall()

    // Lift params via the pure-math entry point (Loop 1 item 25).
    args, err := abi.LiftValues(lcx, abi.MaxFlatParams, stack, f.funcType.Params)
    if err != nil {
        lcx.ExitCall()
        return nil, err
    }

    // Toggle may_leave to false while invoking the host (the host can
    // call back into the runtime, but not into the same guest
    // instance — wasmtime func.rs:957).
    f.callerInstance.SetMayLeave(false)
    results, err := f.host(ctx, args)
    f.callerInstance.SetMayLeave(true)
    if err != nil {
        lcx.ExitCall()
        return nil, err
    }

    // Lower the host's results into the result-side of the stack.
    lwx := abi.NewLowerContext(f.memory, f.options, f.realloc, f.resourceTable)
    outStack, err := abi.LowerValues(lwx, abi.MaxFlatResults, results, f.funcType.Results, nil)
    if err != nil {
        lcx.ExitCall()
        return nil, err
    }

    // Pop the borrow scope (wasmtime host.rs:505 lower.exit_call()).
    if err := lcx.ExitCall(); err != nil {
        return nil, err
    }
    return outStack, nil
}
```

(The exact field names on `LoweredFunc` — `callerInstance`, `memory`,
`options`, `realloc`, `resourceTable`, `host`, `funcType` — must be
read from the actual struct before writing. The body above is the
shape; the field accesses are illustrative. The lifecycle methods
`MayLeave`/`SetMayLeave`/`EnterCall`/`ExitCall` must exist on the
relevant types post-item-9.7. If they don't, escalate.)

After the replacement, every helper function in `canon_lower.go` that
was only called by the old body has zero callers. Delete each of them in
this same commit. Use Grep to find any other callers before deleting.

Tests in `canon_lower_test.go` that asserted the per-type case logic
(for example, "test that variant lowering picks the right discriminant
size") become redundant because that logic now lives in `abi/` and is
tested there. Delete those tests.

**Definition of done:**
- `LoweredFunc.CallWithStack` body is replaced with the shim above (or
  equivalent matching the actual struct fields)
- Every helper function deleted has zero references repo-wide (verify
  with `Grep` before deleting)
- All tests in `canon_lower_test.go` either:
  (a) test the new shim path and pass, or
  (b) are deleted because their subject was a deleted helper
- `go test ./internal/component/...` passes (or shows only the
  expected pre-existing failures from Loop 1 phase 1.A item 10)

**Reviewer focus areas:**
- Spec compliance: confirm the shim matches `definitions.py:3453`
  `canon_lower` shape — specifically that argument lifting happens
  before the host call, not the other way around
- Code quality: confirm no helpers were missed in deletion; confirm
  no `// fallback` or error suppression introduced; confirm tests use
  the same patterns as adjacent abi/ tests

---

### Item 3: Rewrite `createCanonLowerFunc` body as a lifecycle wrapper

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Depends on item 2 (same wrapper pattern). createCanonLowerFunc is at component_linker.go:2430 (verified, NOT 3157). Same lifecycle responsibilities as item 2.

**Files:**
- Modify: `internal/component/component_linker.go` — replace the body
  of `(l *ComponentLinker) createCanonLowerFunc(...)` at line 2430
  (verified)
- Modify: `internal/component/component_linker_test.go` — adjust tests;
  delete tests of intermediate helpers
- Delete: the inner helpers `liftFromStack` (2547),
  `liftRecordFromStack` (2681), `liftOptionFromStack` (2694),
  `liftVariantFromStack` (2709), `liftValFromMemory` (2726),
  `liftRecordFromMemory` (2797), `liftOptionFromMemory` (2815),
  `liftListFromMemory` (2991), `flatSlotCount` (3020), `lowerToStack`
  (3074) — all become orphans after the body rewrite (verified by
  the Loop 1 audit)

**Spec authorities:**
- `definitions.py:1978-2063` — `canon_lift` (verified)
- `definitions.py:2064` — `canon_lower` (verified)
- `definitions.py:1943` — `lift_flat_values`
- `definitions.py:1954` — `lower_flat_values`
- `crates/wasmtime/src/runtime/component/func/host.rs::HostFn::call_sync_lower`
  — wasmtime's host import wrapper

**Description:**
`createCanonLowerFunc` (component_linker.go:2430) builds a closure
that performs canon-lowering for host imports inside inline component
instances. The closure's body currently:
1. Resolves memory from canonical options + memory index (lines 2442-2465)
2. Resolves realloc from canonical options + func index (lines 2466-2480)
3. Reads retptr from end of stack if `needsRetptr` (lines 2487-2492)
4. Lifts core args via `liftFromStack` (line 2502)
5. Calls host with panic on error (lines 2509-2519)
6. If `needsRetptr`: writes results via `writeResultsToMemory` (2522-2529)
7. Else: writes results via `lowerToStack` for each result (2532-2541)

The new body does the same orchestration but delegates the lift/lower
math to `abi.LiftValues`/`abi.LowerValues` (Loop 1 item 25):

```go
func (l *ComponentLinker) createCanonLowerFunc(
    ctx context.Context, inst *ComponentInstance, c *CanonicalOptions,
    info *FuncInfo, compFunc *NamedValType, needsRetptr bool,
) GoFunc {
    return func(ctx context.Context, stack []uint64) ([]uint64, error) {
        // Resolve memory and realloc as before (steps 1-2 above stay)
        memory := /* lookup via c.MemoryIdx */
        realloc := /* lookup via c.ReallocIdx */

        // Lifecycle: may_leave check (wasmtime host.rs:292)
        if !inst.MayLeave() {
            return nil, fmt.Errorf("cannot leave component during post-return")
        }

        // Build contexts and push borrow scope
        lcx := abi.NewLiftContext(memory, c, inst.ResourceTable())
        lwx := abi.NewLowerContext(memory, c, realloc, inst.ResourceTable())
        lcx.EnterCall()

        // Lift params via abi (replaces liftFromStack and family)
        args, err := abi.LiftValues(lcx, abi.MaxFlatParams, stack, compFunc.FuncType.Params)
        if err != nil {
            lcx.ExitCall()
            return nil, err
        }

        // Invoke host with may_leave toggle
        inst.SetMayLeave(false)
        results, err := info.HostCallable(ctx, args)
        inst.SetMayLeave(true)
        if err != nil {
            lcx.ExitCall()
            return nil, err
        }

        // Lower results via abi (replaces writeResultsToMemory and lowerToStack)
        outStack, err := abi.LowerValues(lwx, abi.MaxFlatResults, results, compFunc.FuncType.Results, nil)
        if err != nil {
            lcx.ExitCall()
            return nil, err
        }
        if err := lcx.ExitCall(); err != nil {
            return nil, err
        }
        return outStack, nil
    }
}
```

(Field names like `info.HostCallable`, `compFunc.FuncType` are
illustrative; read the actual structs first.)

After the rewrite, every helper function listed in **Files: Delete**
above has zero callers. Delete each with `Grep` confirmation.

**Definition of done:**
- `createCanonLowerFunc` body is a single call to `abi.CanonLower`
- Every helper deleted has zero references
- Tests pass or are deleted with their subject
- `go test ./internal/component/...` passes (or shows expected
  pre-existing failures only)

**Reviewer focus areas:**
- Spec compliance: confirm the closure does not perform any
  pre-processing or post-processing that the spec does not authorize
- Code quality: confirm no orphaned helpers; confirm closure capture
  list is minimal

---

### Item 4: Delete `writeResultsToMemory`, `writeRecordToMemory`, `writeValToMemory` and the flatten family

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Closes the Fix #11 cycle. Depends on item 3 (which removes the only callers). All function lines verified.

**Files:**
- Modify: `internal/component/component_linker.go` — delete:
  - `writeResultsToMemory` (line 3157, verified)
  - `writeRecordToMemory` (line 3369, verified) — has the missing
    `alignTo` between fields bug
  - `writeValToMemory` (line 3387, verified) — has the s16/u16 4-byte
    write / 2-byte advance bug at the case label around 3402, plus the
    innerSize fallback at 3443
  - The flatten family (lines 3625-3797): `flattenValType`,
    `flattenRecordType`, `flattenTupleType`, `flattenOptionType`,
    `flattenResultType`, `flattenFlagsType`, `flattenVariantType`,
    `valueTypeWidth`, `isWiderValueType`, `componentTypeToCoreTypes`
    (the duplicate flatten implementation; abi/flatten.go is the
    canonical one used by Loop 1 item 25)
- Modify: `internal/component/component_linker_test.go` — delete every
  test referencing the deleted functions

**Spec authorities:**
- `definitions.py:1365` — `store(cx, v, t, ptr)` (the unified store
  dispatcher)
- `definitions.py:1607` — `store_record` (confirms iterate-declared-order
  with alignment between fields, the bug at writeRecordToMemory)
- `definitions.py:1613` — `store_variant` (confirms discriminant size
  via `discriminant_type`, the bug at writeResultsToMemory:3292)
- `crates/wasmtime/src/runtime/component/values.rs::Val::store`
  — wasmtime equivalent
- `crates/wasmtime/src/runtime/component/func/typed.rs` `Lower` trait

**Description:**
The flatten and write families in `component_linker.go` have multiple
confirmed bugs:
- Line 3292 (in `writeResultsToMemory`): hard-coded `0` discriminant
  for variants (`memory.WriteUint32Le(offset, 0) // Placeholder discriminant`)
- Line 3402-3407 (in `writeValToMemory` s16/u16 case): writes 4 bytes
  via `WriteUint32Le`, then advances offset by only 2
- Line 3370-3382 (in `writeRecordToMemory`): iterates `recordDef.Fields`
  in declared order but never calls `alignTo` between fields
- Line 3443 (in `writeValToMemory`): `innerSize := fieldSizeForType(ValTypeRef{}, localTypes)`
  — empty `ValTypeRef{}` fallback that misreads union types as i32
- Lines 3747-3797 (`flattenVariantType` + `isWiderValueType`):
  variant flat join produces `f32` where spec says `i32`

All of these are buggy duplicates of logic that already exists,
correctly, in `internal/component/abi/`. This item DELETES them
entirely. The correct replacements are:
- `abi.LowerValues` (Loop 1 item 25) for the multi-value write path
- `abi.LowerHeap` (already in `abi/lower.go`) for individual values
- `abi.FlattenParams`/`abi.FlattenResults`/`abi.CoreSignature`
  (already in `abi/flatten.go`) for signature flattening

Item 3's rewrite of `createCanonLowerFunc` is the only caller of these
functions; after item 3 lands, every function in the **Files: Delete**
list above is dead. This item just confirms they have zero callers and
deletes them.

Tests in `component_linker_test.go` that asserted these functions'
behavior are deleted. The same coverage exists in `abi/lower_test.go`
and `conformance/canonical_abi/` (Loop 1 phase 1.C).

**Definition of done:**
- `writeResultsToMemory`, `writeValToMemory`, `writeRecordToMemory`
  deleted from `component_linker.go`
- Entire flatten family deleted: `flattenValType`, `flattenRecordType`,
  `flattenTupleType`, `flattenOptionType`, `flattenResultType`,
  `flattenFlagsType`, `flattenVariantType`, `valueTypeWidth`,
  `isWiderValueType`, `componentTypeToCoreTypes`
- Bug sites at lines 3292, 3402, 3370-3382, 3443 (and 3747-3797 for
  variant flatten) all demonstrably gone (Grep for the comment text
  `Placeholder discriminant` or function names returns zero)
- All tests of the deleted helpers are deleted
- `go test ./internal/component/abi/...` and
  `go test ./internal/component/conformance/canonical_abi/...` pass
- `go test ./internal/component/...` passes (or shows expected
  pre-existing failures only)

**Reviewer focus areas:**
- Spec compliance: confirm record store now applies alignment between
  fields (cite `definitions.py:1607` `store_record`); confirm variant
  store uses the spec-correct discriminant size (cite `definitions.py`
  `discriminant_type`); confirm s16/u16 stores write 2 bytes and advance
  2 bytes
- Code quality: confirm all three bug sites are demonstrably gone;
  confirm no resurrected `if err != nil { return defaultValue }` paths

---

## Phase 2.C — Wire guest-export path (1 item)

### Item 5: Rewrite `instance.go::ExportedFunc.Call` as a lifecycle shim around abi.LiftValues / abi.LowerValues

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Largest single-file change in Loop 2. Lifecycle (subtask, borrow scope, may_leave, post_return, reentrance, enter/exit, validateReturn) STAYS in instance.go per the wasmtime layering. abi/ stays pure math. Depends on items 4 and Loop 1 items 9.7, 24, 25.

**Files:**
- Modify: `internal/component/instance.go` — DELETE everything that
  is per-value lift/lower logic; KEEP the lifecycle orchestration
  (numbered list below). After this item, `ExportedFunc.Call` is a
  pure lifecycle wrapper that calls `abi.LowerValues` for params and
  `abi.LiftValues` for results.

  **Delete** (none of these are workarounds; they are accumulated
  buggy parallel implementations):
  - The retptr-as-PARAM allocation block at instance.go:305-322 —
    spec-compliant in intent but lives in the wrong place per the
    wasmtime layering. Retptr allocation moves INTO `abi.LowerValues`
    (Loop 1 item 25), where the spec function `lower_flat_values`
    at `definitions.py:1954` puts it via the `out_param` argument.
  - The stale "Some toolchains (Go): core function returns retptr as
    i32, no extra param" comment at instance.go:308 — this comment
    describes an abandoned approach the code never actually
    implements. Go and TinyGo components produce spec-compliant
    retptr signatures; there is no Go-specific retptr branch.
  - The synthesis hack at instance.go:335-338
    (`if usedRetptr && len(coreResults) == 0 { coreResults = []uint64{retptrVal} }`)
    — this exists ONLY because the buggy `liftRecord`/`liftResolvedType`
    family can't read memory directly; it stuffs the retptr pointer
    into `coreResults` so the lifter knows where to look. After
    `abi.LiftValues` (which has a `LiftContext` with direct memory
    access) replaces the lifter, the hack is unnecessary and goes.
  - `liftRecord` at instance.go:757 (with the alphabetical sort at
    line 765 — the spec violation)
  - `liftResolvedType` at instance.go:794 (the dispatcher used by
    `liftRecord`)
  - **The entire "legacy fallback" block at instance.go:450+** —
    this is a fifth parallel lift implementation with its own
    hardcoded per-type logic (`if typeDef.Option != nil`,
    `if typeDef.Record != nil`, `if typeDef.Result != nil`, etc.).
    It assumes specific flat layouts ("Option type: first result is
    discriminant, second is payload"), does not handle nested types,
    and was bolted on next to the TypeResolver path as a "fallback".
    Both paths get replaced by `abi.LiftValues` which handles every
    case correctly.
  - The TypeResolver path at instance.go:440-447 — was the "newer"
    approach bolted on top of the legacy fallback; deleted because
    `TypeResolver` itself goes away in Loop 1 item 9.
- Modify: `internal/component/instance_test.go` — delete the three
  `TestLiftResolvedType_*` tests at lines 2149, 2177, 2198 (verified
  by audit). Delete any `TestLiftRecord*` tests. Delete any test
  that asserts the legacy fallback's per-type behavior (option-
  hardcoded, result-hardcoded). Keep tests of the orchestration
  (subtask, borrow scope, may_leave, post_return) — those still apply.

**Spec authorities:**
- `definitions.py:1978-2063` — `canon_lift` (verified line). Note:
  Python's `canon_lift` combines lift+invoke+post_return+exit; wazero
  splits per the wasmtime layering — math goes to abi/, lifecycle
  stays in instance.go.
- `definitions.py:1303` — `load_record`, confirms iterate-declared-order
  (NOT alphabetical, contra `instance.go:765`)
- `crates/wasmtime/src/runtime/component/func.rs::Func::call_raw`
  (lines 603-707) — wasmtime's lifecycle wrapper. The model wazero
  follows.
- `crates/wasmtime/src/runtime/component/func.rs::Func::post_return_impl`
  (lines 765-837) — wasmtime's post-return path. The model for the
  post-return invocation that lives in instance.go (NOT abi/).

**Description:**
`ExportedFunc.Call` is the guest-export path. It's the wazero
analogue of wasmtime's `Func::call_raw`. Per the wasmtime layering
research, it owns the **lifecycle**. The lifecycle steps below are
the ones that STAY in instance.go (the per-value lift/lower internals
they currently include get DELETED):

1. Reentrance check (`may_enter`)
2. Subtask creation
3. Borrow scope creation from subtask
4. Set instance.callContext
5. EnterCall/ExitCall tracking
6. Toggle `mayLeave` around lowering
7. Call core function (the core wasm `Function.Call`)
8. Toggle `mayLeave` around result handling
9. Call post-return function
10. Validate `callCtx.ValidateReturn()`
11. Resolve subtask

What gets DELETED (per the Files section above): the entire per-type
lift/lower logic, the retptr allocation (which moves into
`abi.LowerValues`), the synthesis hack at 335-338, the legacy
fallback at 450+, `liftRecord`, `liftResolvedType`, the TypeResolver
path. Result lifting flows through `abi.LiftValues` which reads
directly from memory via `LiftContext`.

```go
func (f *ExportedFunc) Call(ctx context.Context, args ...runtime.Val) ([]runtime.Val, error) {
    // Steps 1-5: orchestration setup (stays unchanged)
    callCtx := newCallContext(...)
    subtask := newSubtask(...)
    scope := newBorrowScope(subtask)
    f.instance.callContext = callCtx
    if err := f.instance.reentrance.ValidateNotRecursive(...); err != nil { return nil, err }
    f.instance.EnterCall()
    defer f.instance.ExitCall()

    // Step 6: lower params via abi (replaces ~75 lines of per-type
    // lowering at instance.go:207-281). LowerValues handles param
    // spill AND result-retptr allocation internally per
    // definitions.py:1954 lower_flat_values (the out_param argument).
    f.instance.SetMayLeave(false)
    lwx := abi.NewLowerContext(f.memory, f.options, f.realloc, f.instance.ResourceTable())
    coreArgs, err := abi.LowerValues(lwx, abi.MaxFlatParams, args, f.funcType.Params, nil)
    if err != nil {
        f.instance.SetMayLeave(true)
        return nil, err
    }
    f.instance.SetMayLeave(true)

    // Step 7: Invoke core wasm.
    _, err = f.coreFunc.Call(ctx, coreArgs...)
    if err != nil { return nil, err }

    // Step 8: Lift results via abi. LiftValues reads directly from
    // memory via the LiftContext for retptr-spilled results — no
    // synthesis hack needed because the lifter has memory access.
    f.instance.SetMayLeave(false)
    lcx := abi.NewLiftContext(f.memory, f.options, f.instance.ResourceTable())
    results, err := abi.LiftValues(lcx, abi.MaxFlatResults, coreArgs, f.funcType.Results)
    f.instance.SetMayLeave(true)
    if err != nil { return nil, err }

    // Step 9: post-return invocation (lifecycle, stays here, NOT in abi/).
    if f.options.PostReturnIdx != nil {
        if err := f.invokePostReturn(ctx, coreArgs); err != nil { return nil, err }
    }

    // Step 10: validate return
    if err := callCtx.ValidateReturn(); err != nil { return nil, err }

    // Step 11: Resolve subtask
    subtask.Resolve()

    return results, nil
}
```

(Field accesses are illustrative. Read the actual structs first.
The key point: NO synthesis hacks, NO Go/TinyGo workarounds, NO
legacy fallbacks. Per-type lift/lower is delegated entirely to
`abi.LiftValues`/`abi.LowerValues`. The retptr allocation that
currently lives at instance.go:305-322 moves into `abi.LowerValues`
where the `out_param` argument of `lower_flat_values` belongs per
spec.)

**On the deleted "Go/TinyGo retptr workaround":** Both Go and TinyGo
components produce spec-compliant canonical ABI binaries — the same
as Rust. The stale comment at instance.go:308 describes an abandoned
approach the code never implemented. The synthesis hack at 335-338
exists ONLY because the buggy `liftRecord`/`liftResolvedType` family
can't read memory directly; once `abi.LiftValues` (which has direct
memory access via `LiftContext`) replaces them, the hack is
unnecessary. There is no toolchain-specific divergence.

**Definition of done:**
- `ExportedFunc.Call` body is the lifecycle wrapper above, calling
  `abi.LowerValues` and `abi.LiftValues`
- `liftRecord` (757), `liftResolvedType` (794), the retptr allocation
  at 305-322, the synthesis hack at 335-338, the entire legacy
  fallback at 450+, the TypeResolver path at 440-447, and the stale
  "Some toolchains (Go)" comment at 308 are ALL deleted
- Tests asserting the alphabetical sort, the synthesis hack, or the
  legacy fallback's per-type behavior are deleted (they were testing
  wrong behavior)
- The previously-failing test `TestCalculatorPlugins/multi` now passes
  (or, if it still fails, it must be for a documented reason traced
  to a different item — escalate if so)
- `go test ./internal/component/wasip2test/...` shows the previously-
  broken tests passing

**Reviewer focus areas:**
- Spec compliance: confirm `load_record` field iteration is now in
  declared order, NOT alphabetical (cite `definitions.py:1303`); confirm
  retptr handling matches `definitions.py:3237` `canon_lift` and
  wasmtime's `call_raw`
- Code quality: confirm `liftRecord`, `liftResolvedType`, and the
  heuristic are demonstrably gone (Grep returns zero); confirm the new
  shim is minimal

---

## Phase 2.D — Resource handle cleanup (2 items)

### Item 6: Un-export the standalone `LiftOwn`/`LiftBorrow`/`LowerOwn`/`LowerBorrow` helpers (they remain as internal dispatch implementations)

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Loop 1 item 24 EXTENDED these helpers (added *ResourceType param) and made them callable from the integrated dispatch. They are no longer standalone entry points but are still the dispatch's implementation. This item un-exports them (lowercase) and confirms the only callers are inside abi/.

**Files:**
- Modify: `internal/component/abi/lift.go` — delete `LiftOwn`,
  `LiftBorrow` exported functions
- Modify: `internal/component/abi/lower.go` — delete `LowerOwn`,
  `LowerBorrow` exported functions
- Modify: `internal/component/abi/resource_lower.go` — delete
  `LowerOwnWithType`, `LowerBorrowWithType` if they are now unused
- Modify: `internal/component/abi/lift_test.go`,
  `internal/component/abi/lower_test.go`,
  `internal/component/abi/resource_lower_test.go` — delete tests of the
  deleted standalones

**Spec authorities:**
- `definitions.py:1197-1198` — `load(cx, ptr, t)` dispatching
  `OwnType()`/`BorrowType()` to `lift_own`/`lift_borrow` inside the
  unified switch
- `definitions.py:1387-1388` — `store(cx, v, t, ptr)` doing the
  symmetric dispatch
- `definitions.py:1792-1793` — `lift_flat()` Own/Borrow case
- `definitions.py:1886-1887` — `lower_flat()` Own/Borrow case
- `crates/wasmtime/src/runtime/component/values.rs:115` —
  `InterfaceType::Own(_) | InterfaceType::Borrow(_)` matched inside the
  unified `lift` function

**Description:**
Loop 1 item 24 extended the existing `LiftOwn`/`LiftBorrow`/`LowerOwn`/
`LowerBorrow` helpers in `abi/` to take `*ResourceType` and folded
them into the four dispatch functions. After Loop 2 items 2-5 wired
production code through `abi.LiftValues`/`abi.LowerValues`, the only
callers of `LiftOwn` etc. are inside `abi/` itself (the dispatch
cases call them as the implementation).

Per spec authority (`definitions.py:1197/1792`) and wasmtime
(`values.rs:115`), there is no need to expose these as separate
public entry points. **Un-export them** (lowercase first letter).
This guarantees no external code re-introduces a parallel handle
path in the future. Their tests can remain since they're internal
unit tests.

Before un-exporting, run Grep for each function name across the
entire repo. If any non-test reference outside `abi/` remains, items
2-5 missed something — escalate.

**Definition of done:**
- `LiftOwn`, `LiftBorrow`, `LowerOwn`, `LowerBorrow` are renamed to
  `liftOwn`, `liftBorrow`, `lowerOwn`, `lowerBorrow` (un-exported)
- `LowerOwnWithType`, `LowerBorrowWithType` (in
  `abi/resource_lower.go:21,52`) are also un-exported if they have no
  external callers; OR deleted if Loop 1 item 24 inlined their logic
- All callers (only inside `abi/`) updated to use the un-exported names
- Grep for `\babi\.LiftOwn\b|\babi\.LiftBorrow\b|\babi\.LowerOwn\b|\babi\.LowerBorrow\b`
  across the entire repo returns zero
- `go test ./internal/component/abi/...` passes

**Reviewer focus areas:**
- Spec compliance: confirm the integrated dispatch (Loop 1 item 24)
  correctly handles every case the standalones handled
- Code quality: confirm un-exporting (not deletion) if the helpers
  are still the dispatch's implementation; confirm no external code
  uses the exported names

---

### Item 7: Delete `ResourceTable.CreateResourceDropFunc` and `CreateResourceRepFunc` (silent variants)

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** -

**Files:**
- Modify: `internal/component/resource_table.go` — delete the silent
  variants `CreateResourceDropFunc` and `CreateResourceRepFunc`; keep
  the trap-emitting variants
- Modify: `internal/component/resource_table_test.go` — delete the
  following tests (verified by audit):
  `TestResourceTable_CreateResourceDropFunc` (line 302),
  `TestResourceTable_CreateResourceDropFunc_InvalidHandle` (334),
  `TestResourceTable_CreateResourceDropFunc_NilDestructor` (350),
  `TestResourceTable_CreateResourceRepFunc` (366),
  `TestResourceTable_CreateResourceRepFunc_InvalidHandle` (381).
  Add equivalent tests for the trap-emitting variants if not already
  present.
- Modify: `internal/component/wasip2test/kv_store_test.go:208` — this
  caller uses `CreateResourceDropFunc` (the silent variant). Migrate
  to the trap-emitting variant. Note: kv_store_test.go's
  `TestResourceLifecycle_LinkerDefinition` is the white-box test that
  Loop 3 explicitly leaves in the allow-list — this fix happens here
  in Loop 2 because it's a sockets-style trap-rule fix, not a public-
  API migration.

**Spec authorities:**
- `definitions.py:1641` `lower_own(cx, rep, t)` — confirms drop is a
  trapping operation when called on an invalid handle
- `definitions.py:1645` `lower_borrow(cx, rep, t)`
- `CanonicalABI.md` "Resources" section

**Description:**
The audit found that `ResourceTable.CreateResourceDropFunc` and
`CreateResourceRepFunc` exist in two variants: a silent-ignore version
and a trap-emitting version, both exported and wired into different
code paths. The silent-ignore versions are non-canonical (the spec
requires drop on an invalid handle to trap).

Delete the silent variants. Migrate any caller to the trap-emitting
versions. Update or delete tests that asserted the silent behavior.

**Definition of done:**
- The silent-ignore variants are deleted
- Every caller now uses the trap-emitting variant
- Tests assert trap behavior, not silent ignore
- `go test ./internal/component/...` passes

**Reviewer focus areas:**
- Spec compliance: confirm trap behavior matches the spec definition
  for `lower_own`/`lower_borrow` and for `drop` operations
- Code quality: confirm no caller was missed; confirm no test asserts
  the old silent behavior

---

## Phase 2.E — Fix wasip2 silent-default error suppression (~67 sites, 4 items)

> **Trap rule (universal for items 8-11):**
>
> For each silent-default site, the agent must read the WIT method
> definition vendored under `debug-vendored/WASI/proposals/<area>/wit/`
> (note the `/wit/` segment — verified path; the original plan
> incorrectly omitted it). Then:
>
> 1. If the WIT method's return type is `result<_, error-code>`,
>    replace the silent-default with `result.err(<correct error-code
>    per the WIT enum>)`. The `error-code` enum is in the same `wit/`
>    directory: `network.wit` for sockets, `types.wit` for HTTP,
>    `types.wit` for filesystem. The error-code must be the most
>    accurate one for the failure (e.g., `invalid-state` for a wrong
>    handle type, `not-permitted` for an authorization failure,
>    `bad-descriptor` for an invalid descriptor handle).
> 2. If the WIT method's return type does NOT have an error union,
>    replace the silent-default with a trap (return an error from the
>    Go function; the wazero machinery turns errors into traps).
> 3. **Never preserve the placeholder success.**
> 4. **Delete the misleading `// Fallback for tests without resource
>    table` comment** in the same change.
> 5. The pattern includes both `if err != nil { return placeholder }`
>    and `if table == nil { return placeholder }` — both are silent
>    defaults; both must be fixed.

### Item 8: Fix `imports/wasip2/sockets/tcp.go` silent-default sites (~28 sites)

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Audit found ~28 sites (not 22 as originally claimed). Includes both `if err != nil` and `if table == nil` patterns. Confirm count via Grep before starting.

**Files:**
- Modify: `imports/wasip2/sockets/tcp.go` — convert all silent-default
  sites per the trap rule above (~28 sites; verify count via Grep)
- Modify: `imports/wasip2/sockets/tcp_test.go` — add tests for each
  error path that asserts the spec-correct trap or `result.err`

**Spec authorities:**
- `debug-vendored/WASI/proposals/sockets/wit/tcp.wit` — the WIT
  definitions (verified path with `wit/` segment)
- `debug-vendored/WASI/proposals/sockets/wit/network.wit` — for the
  `error-code` enum
- `debug-vendored/wasmtime/crates/wasi/src/p2/host/tcp.rs` — wasmtime's
  implementation, for ambiguity resolution

**Description:**
22 sites in `tcp.go` currently follow the pattern:

```go
sock, err := getTcpSocket(ctx, handle)
if err != nil {
    // Fallback for tests without resource table
    return ValBool(false), nil  // or similar placeholder success
}
```

This is wrong on two counts: there is no "fallback for tests"
(production tests should set up resource tables; the comment is a lie),
and the spec requires either a trap or a `result.err`.

For each site:

1. Read the corresponding WIT method definition. The Go function name
   maps to a WIT method name; find it in `tcp.wit`.
2. Read the WIT method's return type. If it's `result<_,
   error-code>`, the fix is `return ValResultErr(ValEnum("invalid-state"))`
   (or whatever error-code best matches; consult the WIT enum
   definition). If it's not a result type, the fix is to return an
   error from the Go function: `return nil, fmt.Errorf("invalid TCP
   socket handle: %w", err)`.
3. Add a test that creates the import with no resource table (or with
   a wrong handle), invokes the method, and asserts the trap or the
   `result.err`.

**Definition of done:**
- All 22 silent-default sites converted (verify with Grep for the
  pattern `// Fallback` and `if err != nil .*\n.*return Val`)
- Each site has a corresponding test that exercises the error path
- `go test ./imports/wasip2/sockets/...` passes

**Reviewer focus areas:**
- Spec compliance: for each site, confirm the WIT return type was
  consulted (cite `tcp.wit` line) and the chosen error code matches
  the failure mode
- Code quality: confirm the "Fallback for tests" comments are gone;
  confirm no `// TODO` introduced; confirm tests assert specific error
  codes, not just "any error"

---

### Item 9: Fix `imports/wasip2/sockets/udp.go` silent-default sites (~20 sites)

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Same trap rule as item 8. Audit found ~20 sites (not 14). udp_test.go does NOT exist today — must be CREATED.

**Files:**
- Modify: `imports/wasip2/sockets/udp.go` — convert all silent-default
  sites (~20; verify count via Grep)
- **Create:** `imports/wasip2/sockets/udp_test.go` (does not exist
  today — verified by audit) — add tests for each error path

**Spec authorities:**
- `debug-vendored/WASI/proposals/sockets/wit/udp.wit` (verified path)
- `debug-vendored/WASI/proposals/sockets/wit/network.wit`
- `debug-vendored/wasmtime/crates/wasi/src/p2/host/udp.rs`

**Description:**
Same as item 8 but for UDP. 14 sites total.

**Definition of done:**
Same as item 8.

**Reviewer focus areas:**
Same as item 8.

---

### Item 10: Fix `imports/wasip2/http/http.go` silent-default sites (~38 sites)

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Largest of the silent-default cleanups. Same trap rule as item 8. Audit found 31 `if err != nil` sites + ~7 `if table == nil` sites = ~38 total.

**Files:**
- Modify: `imports/wasip2/http/http.go` — convert all silent-default
  sites (~38; verify count via Grep)
- Modify: `imports/wasip2/http/http_test.go` — add tests

**Spec authorities:**
- `debug-vendored/WASI/proposals/http/wit/types.wit` (verified path)
- `debug-vendored/WASI/proposals/http/wit/handler.wit`
- `debug-vendored/wasmtime/crates/wasi-http/src/`

**Description:**
Same as item 8 but for HTTP. 31 sites total. Note that HTTP has more
varied error types (`incoming-request`, `outgoing-request`, `fields`,
`incoming-response`, etc.) — the agent must consult `types.wit` for
each handle type to determine the correct error code.

**Definition of done:**
Same as item 8.

**Reviewer focus areas:**
Same as item 8 plus: confirm the agent did not collapse multiple
distinct error types into a single generic error code. Each handle
type should map to its own most-specific error.

---

### Item 11: Audit and fix `imports/wasip2/{filesystem,clocks,random,cli,io}/*.go` for the same pattern

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** The audit only counted sockets/http; this item verifies the rest are clean or fixes them.

**Files:**
- Read: `imports/wasip2/filesystem/*.go`, `imports/wasip2/clocks/*.go`,
  `imports/wasip2/random/*.go`, `imports/wasip2/cli/*.go`,
  `imports/wasip2/io/*.go`
- Modify: any of the above that contain the silent-default pattern,
  per the trap rule
- Create: `docs/plans/projects/abi-unification/loop-2-wasip2-audit-report.md`
  recording what was found and fixed in each subdirectory

**Spec authorities:**
- `debug-vendored/WASI/proposals/filesystem/types.wit`
- `debug-vendored/WASI/proposals/clocks/`
- `debug-vendored/WASI/proposals/random/random.wit`
- `debug-vendored/WASI/proposals/cli/`
- `debug-vendored/WASI/proposals/io/streams.wit`,
  `debug-vendored/WASI/proposals/io/poll.wit`
- `debug-vendored/wasmtime/crates/wasi/src/p2/host/` for cross-reference

**Description:**
Run a Grep for the silent-default pattern across each subdirectory:

```
Grep pattern: if err != nil .*\n.*return Val(Bool|U16|Own|Result|Err)
```

(or equivalent — the implementer should also Grep for `// Fallback`
comments). For each match found, apply the trap rule from items 8-10.

After processing each subdirectory, write a paragraph in
`loop-2-wasip2-audit-report.md` recording:
- How many sites were found
- How they were fixed
- Whether any subdirectory was already clean

**Definition of done:**
- `loop-2-wasip2-audit-report.md` exists with a section per
  subdirectory
- Every silent-default site is fixed (zero matches for the pattern in
  any of the listed directories)
- Each fix has a corresponding test
- `go test ./imports/wasip2/...` passes

**Reviewer focus areas:**
- Spec compliance: confirm the agent actually read the WIT files for
  each subdirectory (each fix should cite a WIT line)
- Code quality: confirm the audit report is honest — if a subdirectory
  was clean, the report says so; the report does not claim work that
  wasn't done

---

## Phase 2.F — Termination & test cleanup (5 items)

### Item 12: Dead-code & dead-test sweep

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Verifies items 2-11 left no orphans

**Files:**
- Create: `docs/plans/projects/abi-unification/loop-2-deletion-report.md`
- Read (no modification, just verification): every file modified by
  items 2-11

**Spec authorities:**
- N/A — this is a cleanup verification item

**Description:**
For every function name removed in items 2-11 (refer to the
`loop-2-call-site-map.md` from item 1 and the implementation summaries
of items 2-11), run Grep across the entire repo and confirm zero
references — including in test files, table-driven test cases, fixture
builders, helper functions, and comments.

For each function with zero references: confirm it is actually deleted
from its source file. If a function has zero references but still
exists in the source, delete it as part of this item.

For each test file that exclusively tested a deleted function: if the
test file is now empty or contains only setup helpers with no test
functions, delete the test file.

For each helper function in `internal/component/` (outside `abi/` and
`binary/`) that has zero callers after items 2-11: this is dead code
created indirectly. Delete it.

Write `loop-2-deletion-report.md` listing every file deleted and every
function name removed, with line counts:

```markdown
# Loop 2 Deletion Report

## Functions removed
| Function | File (before) | Removed in item | Lines |
|---|---|---|---|
| liftFromStack | component_linker.go | item 4 | 47 |
| ... |

## Files deleted
| File | Removed in item | Lines |
|---|---|---|
| canon_lower_per_type.go | item 2 | 312 |
| ... |

## Cleanup performed in this item
| Function | File | Lines |
|---|---|---|
| <orphan> | <file> | <count> |
```

**Definition of done:**
- `loop-2-deletion-report.md` exists with all three tables filled
- Grep for every removed function name returns zero matches repo-wide
- No test file in `internal/component/` references a deleted function
- `go test ./...` passes (or shows only the documented Loop 1 phase
  1.A pre-existing failures, which Loop 2 should have resolved by
  this point)

**Reviewer focus areas:**
- Spec compliance: N/A
- Code quality: confirm the deletion report is accurate by running
  fresh Greps for a sample of the removed names; confirm no orphan
  helpers remain

---

### Item 13: Test rework verification

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** -

**Files:**
- Read (no modification): every `_test.go` file under
  `internal/component/`, `internal/component/abi/`,
  `internal/component/wasip2test/`, `imports/wasip2/`
- Modify: `internal/component/integration_public_api_test.go` — remove
  the `t.Skipf` from `TestPublicAPIAddS32` and confirm it now passes

**Spec authorities:**
- N/A — verification item

**Description:**
After items 2-12, verify the test surface is clean:

1. Run `Grep` for `t.Skip` introduced by Loop 2 commits. Expected:
   zero. If any `t.Skip` was added by Loop 2 (not pre-existing from
   Loop 1's documented allow-list), it's a violation. Either the test
   is now valid (delete the skip) or it's invalid (delete the test).

2. Run `Grep` for `// TODO`, `// FIXME`, `// fallback`, `// hack`
   added by Loop 2 commits. Expected: zero.

3. Run `Grep` for new mocks in any test file. (Look for
   `mockMemory{}`, `fakeRuntime{}`, etc. that weren't there before
   Loop 2.) Expected: zero.

4. Run `Grep` for new helpers in `_test.go` files that have only one
   caller (their own test). Expected: zero — single-use test helpers
   should be inlined.

5. Read `integration_public_api_test.go::TestPublicAPIAddS32`.
   It contains FOUR `t.Skipf` calls (verified by audit at lines 70,
   89, 110, 121):
   - Line 70: `t.Skipf("test component not available: %v", err)` —
     legitimate guard for missing fixture; KEEP unless the fixture is
     committed in this item
   - Line 89: `t.Skipf("instantiation not fully wired yet: %v", err)` —
     REMOVE (item 5 wired this)
   - Line 110: `t.Skipf("function call not wired yet (recovered from
     panic): %v", r)` — REMOVE
   - Line 121: `t.Skipf("function call not wired yet: %v", err)` —
     REMOVE
   Also remove the `defer recover()` panic guard at lines 105-115
   that catches the broken-call panic. After removal, run the test.
   It should pass through `add_s32.wasm` end-to-end.

**Definition of done:**
- Zero new `t.Skip` introduced by Loop 2
- Zero new `// TODO`/`// FIXME`/`// fallback`/`// hack` introduced by
  Loop 2
- Zero new mocks
- Zero new single-use test helpers
- `TestPublicAPIAddS32` is no longer skipped and passes
- `go test ./internal/component/...` passes

**Reviewer focus areas:**
- Spec compliance: N/A
- Code quality: this IS the code-quality verification item; the
  reviewer should run their own Greps and cross-check

---

### Item 14: Run full test suite and verify previously-broken tests now pass

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** -

**Files:**
- Modify: any test that asserts wrong-spec behavior — rewrite to assert
  correct behavior with a comment citing the spec line that says it
  was wrong
- Create: `docs/plans/projects/abi-unification/loop-2-test-report.md`

**Spec authorities:**
- The cited spec lines for any rewritten tests

**Description:**
Run `go test ./...` from the repo root. Expected: all green.

The previously-broken tests that should now pass:
- `TestCalculatorPlugins` (especially `multi`)
- `TestHostImport_*`
- `TestPublicAPI_*`
- `TestProperty_*`
- `TestWasiExercise_*`

Any test that still fails after items 2-13: investigate the failure.
There are three possible causes:
1. **A bug in the implementation of items 2-12** — file a regression
   note and fix it (this becomes a sub-item; bounce the original item's
   review).
2. **The test was asserting wrong-spec behavior** (e.g., it expected
   alphabetical record order). Rewrite the test to assert correct
   behavior. Add a code comment citing the spec line.
3. **The test was testing a deleted function** — it should have been
   deleted in items 2-12. Delete it.

Write `loop-2-test-report.md` recording:
- Which previously-failing tests now pass
- Which tests were rewritten (with reason and spec citation)
- Which tests were deleted (with reason)
- Which tests remain failing (with reason — escalation to user if any)

**Definition of done:**
- `go test ./...` is green
- `loop-2-test-report.md` exists
- No test was silently weakened (e.g., `assertEqual` changed to
  `assertNotNil` to make it pass)

**Reviewer focus areas:**
- Spec compliance: for each rewritten test, confirm the new assertion
  matches the spec
- Code quality: confirm no test was weakened; confirm the test report
  is honest

---

### Item 15: Spec-compliance reviewer subagent — final sweep against `definitions.py`

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** This is run via the `verify-loop-complete.md` template's spec-specific path

**Files:**
- Read (no modification): every file modified by items 2-14
- Read: `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py`,
  `debug-vendored/component-model/design/mvp/CanonicalABI.md`

**Spec authorities:**
- All of the canonical ABI spec sections

**Description:**
Dispatch a fresh subagent (using the
`templates/review-spec-compliance.md` template) with the scope set to
"the cumulative diff of Loop 2". The subagent re-reads `canon_lift`,
`canon_lower`, `load`, `store`, `lift_flat`, `lower_flat` in
`definitions.py` and confirms the wired production code matches.

This is independent of the per-item spec reviews that happened during
items 2-11. Per-item review catches per-item mistakes; this catches
cross-cutting mistakes (e.g. "items 2 and 3 each match the spec
individually, but together they leak a borrow that neither caught").

**Definition of done:**
- Subagent dispatched with `verify-loop-complete.md`-style scope
- Subagent's findings recorded in `loop-2-spec-compliance-final.md`
- Verdict is `PASS` (any `BLOCKER` becomes a sub-item; bounce the
  loop)

**Reviewer focus areas:**
- This IS the spec-compliance review; no further review needed

---

### Item 16: Code-quality reviewer subagent — final sweep against wazero patterns

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** -

**Files:**
- Read (no modification): every file modified by items 2-14

**Spec authorities:**
- N/A

**Description:**
Dispatch a fresh subagent (using the
`templates/review-code-quality.md` template) with the scope set to
"the cumulative diff of Loop 2". The subagent re-reads every modified
file and confirms:
- No `// TODO`, `// FIXME`, `// fallback`, `// hack`
- No error suppression
- No orphaned helpers
- No skipped tests
- No new `internal/component` imports in test files outside the
  allow-list
- No dead exports
- Idiomatic Go consistent with adjacent code

**Definition of done:**
- Subagent dispatched
- Findings recorded in `loop-2-code-quality-final.md`
- Verdict is `PASS`

**Reviewer focus areas:**
- This IS the code-quality review

---

## Loop 2 termination

When all 16 items are `status: done`, the driver runs
`templates/verify-loop-complete.md` with `{LOOP_NUMBER}=2`. The
verifier produces `loop-2-completion-report.md`. If verdict is
`COMPLETE`, the loop closes and Loop 3 opens. If `INCOMPLETE`, the
verifier's failing checks become new items at the end of this backlog
and the loop continues.
