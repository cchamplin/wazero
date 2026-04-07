# Loop 2 — Wire `abi/` into production, delete dead code

> **Status:** blocked on Loop 1
>
> **Goal:** Production runtime calls `abi.CanonLift`/`abi.CanonLower` for
> every lift/lower operation. The three parallel implementations are
> deleted along with their tests. The 67 silent-default sites in wasip2
> sockets/http trap or return `result.err(...)` correctly. After this
> loop, `internal/component/{instance.go,component_linker.go,canon_lower.go,
> linker.go}` contain only orchestration; lift/lower live exclusively in
> `abi/`.
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

The map must be a Markdown table with one row per call site:

```markdown
| File | Line | Function | Operation | To be replaced by |
|---|---|---|---|---|
| component_linker.go | 2547 | liftFromStack | lift core stack values to Val | abi.CanonLift |
| component_linker.go | 3157 | createCanonLowerFunc body | host import lower | abi.CanonLower |
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

### Item 2: Replace `LoweredFunc.CallWithStack` body with thin shim to abi.CanonLift/CanonLower

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Depends on Loop 1 items 25-26 (canon_lift/canon_lower entry points)

**Files:**
- Modify: `internal/component/canon_lower.go` — replace
  `LoweredFunc.CallWithStack` body
- Modify: `internal/component/canon_lower_test.go` — adjust tests to
  exercise the new shim path; delete tests that asserted intermediate
  helpers' behavior
- Delete: any private helper functions in `canon_lower.go` that have no
  callers after the body replacement

**Spec authorities:**
- `definitions.py:3453` `canon_lower` definition
- `definitions.py:3237` `canon_lift` definition (used for the host's
  return values)
- `crates/wasmtime/src/runtime/component/func/typed.rs` `Lower::lower`
  trait — wasmtime's equivalent dispatch shape

**Description:**
`LoweredFunc.CallWithStack` is one of the three production lift/lower
paths. It currently contains per-type case logic for converting host
return values into the wasm core stack. Replace the entire body with:

```go
func (l *LoweredFunc) CallWithStack(ctx context.Context, mod api.Module, stack []uint64) error {
    // Lift the wasm-side parameters into host Vals
    args, err := abi.CanonLift(l.options, l.funcType, stack, l.callee)
    if err != nil {
        return err
    }

    // Invoke the host function
    results, err := l.host(ctx, mod, args)
    if err != nil {
        return err
    }

    // Lower the host results back into the wasm core stack
    return abi.CanonLower(l.options, l.funcType, results, stack)
}
```

(Exact signature: read the actual `LoweredFunc` struct in the current
file to get the field names right. Read `abi.CanonLift` and
`abi.CanonLower` signatures from Loop 1 items 25-26.)

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

### Item 3: Replace `createCanonLowerFunc` body with thin shim to abi.CanonLower

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Depends on item 2 (same shim pattern)

**Files:**
- Modify: `internal/component/component_linker.go` — replace
  `createCanonLowerFunc` body (currently around line 3157; verify with
  `Grep` since line numbers shift)
- Modify: `internal/component/component_linker_test.go` — adjust tests;
  delete tests of intermediate helpers
- Delete: any private helper functions only used by the old body

**Spec authorities:**
- `definitions.py:3453` `canon_lower`
- `definitions.py:3132` `lower_flat_values`

**Description:**
`createCanonLowerFunc` builds a closure that performs canon-lowering for
host imports inside inline component instances. Replace the closure body
with a single call to `abi.CanonLower`. The closure's captured variables
(options, function type, host callable) become parameters to the
`abi.CanonLower` call.

After the replacement, the helper functions referenced in
`loop-2-call-site-map.md` for this code path lose their callers. Delete
each that has zero references repo-wide (use `Grep`). This may include:
- Inner functions building the per-type lowering case logic
- Helpers for retptr handling that duplicated abi/ behavior

Test files that asserted intermediate behavior get the same treatment
as item 2: migrate or delete.

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

### Item 4: Replace `writeResultsToMemory` and its bug sites with abi.CanonLift heap writes

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Closes the Fix #11 cycle. Depends on item 3.

**Files:**
- Modify: `internal/component/component_linker.go` — replace
  `writeResultsToMemory`, delete `writeValToMemory`, delete
  `writeRecordToMemory`. Verify line numbers with Grep — current cited
  bug sites are at 3292 (variant disc), 3402 (s16/u16 4-byte/2-byte
  bug), 3369 (record alignment), 3443 (innerSize fallback).
- Modify: `internal/component/component_linker_test.go` — delete tests
  of `writeValToMemory`, `writeRecordToMemory`, `writeResultsToMemory`
  internal behavior

**Spec authorities:**
- `definitions.py:1365` `store(cx, v, t, ptr)` — the unified store
  dispatcher
- `definitions.py:1607` `store_record` — confirms iterate-declared-order
  with alignment between fields
- `definitions.py:1613` `store_variant` — confirms discriminant size
  via `discriminant_type`
- `CanonicalABI.md` "Storing" section
- `crates/wasmtime/src/runtime/component/func/typed.rs` `Lower::store`
  trait

**Description:**
`writeResultsToMemory` is the heap-write counterpart to `lowerByKind`.
It currently has multiple confirmed bugs:
- `component_linker.go:3292`: hard-coded `0` discriminant for variants
- `component_linker.go:3402`: s16/u16 case writes 4 bytes, advances 2
- `component_linker.go:3369`: record fields written without alignment
- `component_linker.go:3443`: innerSize fallback to empty `ValTypeRef{}`

All of these get deleted. The replacement is a single call to the
heap-store path provided by `abi/` (the agent reads `abi/lower.go`'s
`LowerHeap` and confirms its signature; if it needs an entry-point
wrapper for the "write a slice of results to a retptr" use case, that
wrapper goes in `abi/lower.go` as part of this item — NOT in
`component_linker.go`).

After this item, `writeResultsToMemory`, `writeValToMemory`,
`writeRecordToMemory` are deleted. Tests in `component_linker_test.go`
that asserted these functions' behavior are deleted (the same coverage
exists in `abi/lower_test.go` and `conformance/canonical_abi/`).

**Definition of done:**
- `writeResultsToMemory` is either replaced with a thin wrapper around
  `abi/` heap-store, or deleted entirely if its callers can call
  `abi.CanonLift`/`abi.CanonLower` directly
- `writeValToMemory` deleted
- `writeRecordToMemory` deleted
- Bug sites at lines 3292, 3402, 3369, 3443 all gone (verify with Grep
  for the comment text or function name)
- All tests of the deleted helpers are deleted
- `go test ./internal/component/abi/...` and
  `go test ./internal/component/conformance/canonical_abi/...` pass —
  the abi/ heap-store path now carries the load that
  `writeResultsToMemory` used to carry, so its tests must cover the
  same cases
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

### Item 5: Replace `instance.go` `ExportedFunc.Call` family lift/lower with abi.CanonLift/abi.CanonLower

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Largest deletion in Loop 2. Depends on item 4 (so all the host-side helpers are gone first).

**Files:**
- Modify: `internal/component/instance.go` — replace `ExportedFunc.Call`
  body, delete `liftRecord` (around line 757), delete `liftResolvedType`
  (around 794), delete the retptr-as-return-value heuristic at
  instance.go:305-322
- Modify: `internal/component/instance_test.go` — delete tests of
  `liftRecord` (the alphabetical-sort tests confirm wrong behavior),
  `liftResolvedType`, and the retptr heuristic

**Spec authorities:**
- `definitions.py:3237` `canon_lift`
- `definitions.py:3113` `lift_flat_values`
- `definitions.py:1175` `load(cx, ptr, t)` (used for retptr result
  reading)
- `definitions.py:1303` `load_record` — confirms iterate-declared-order
  (NOT alphabetical, contra `instance.go:765`)
- `crates/wasmtime/src/runtime/component/func/typed.rs::call_raw` —
  retptr handling reference

**Description:**
`ExportedFunc.Call` is the third production lift/lower path. It calls a
guest export function: parameters are lowered into the wasm core stack,
the export is invoked, and results are lifted from the stack (or from a
caller-supplied retptr if results overflow flat-results).

Replace the body with a thin wrapper that calls `abi.CanonLower` for
parameters and `abi.CanonLift` for results. Both calls use the new
canonical entry points from Loop 1 items 25-26 which handle the retptr
spill logic correctly.

`liftRecord` (lines 757-790) sorts field names alphabetically before
reading — `sort.Strings(fieldNames)` at line 765 with the comment "the
component model spec requires alphabetical order". This is wrong: the
spec at `definitions.py:1303` `load_record` iterates `fields` (the
declared list). Delete `liftRecord` entirely.

`liftResolvedType` (around 794) is a separate dispatcher used by
`liftRecord`. Once `liftRecord` is gone, check `liftResolvedType` for
other callers. If none, delete it. If it has other callers, migrate
them to `abi.CanonLift`-style calls and then delete.

The retptr-as-return-value heuristic at instance.go:305-322 is a custom
wazero invention. Delete it. The replacement is the spec-correct retptr
handling that `abi.CanonLift` already implements (it reads the retptr
out of the wasm stack, then calls `LiftHeap` for each result via the
retptr).

**Definition of done:**
- `ExportedFunc.Call` body is a thin shim calling `abi.CanonLower` /
  `abi.CanonLift`
- `liftRecord`, `liftResolvedType`, and the retptr-as-return-value
  heuristic at lines 305-322 are deleted
- Tests asserting the alphabetical sort or the retptr heuristic are
  deleted (they were testing wrong behavior)
- The previously-failing test `TestCalculatorPlugins/multi` now passes
  (or, if it still fails, it must be for a documented reason traced to
  a different item — escalate if so)
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

### Item 6: Delete standalone `LiftOwn`/`LiftBorrow`/`LowerOwn`/`LowerBorrow` helpers from abi/

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Depends on Loop 1 item 24 (integrated dispatch). Depends on items 2, 3, 4, 5 (so production paths use integrated dispatch).

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
The standalone `LiftOwn`/`LiftBorrow`/`LowerOwn`/`LowerBorrow` exports
in `abi/` are non-canonical. The spec dispatches own/borrow inside the
unified `load`/`store`/`lift_flat`/`lower_flat` functions, NOT through
separate entry points. Wasmtime confirms this pattern.

Loop 1 item 24 added the integrated dispatch. After items 2-5, all
production code uses `abi.CanonLift`/`abi.CanonLower`, which dispatches
own/borrow internally. The standalone helpers now have:
- Zero production callers
- Tests in abi/lift_test.go, abi/lower_test.go, abi/resource_lower_test.go

Delete the standalones AND their tests in this commit.

Before deleting, run Grep for each function name across the entire
repo. If any non-test reference remains, escalate — items 2-5 missed
something.

**Definition of done:**
- `LiftOwn`, `LiftBorrow`, `LowerOwn`, `LowerBorrow` (and any
  `*WithType` variants only used by tests) are deleted from `abi/`
- All tests of the deleted standalones are deleted
- Grep across the entire repo for each deleted name returns zero
- `go test ./internal/component/abi/...` passes (the integrated
  dispatch tests added in Loop 1 item 24 carry the coverage)

**Reviewer focus areas:**
- Spec compliance: confirm the integrated dispatch correctly handles
  every case the standalones handled (resource type validation, borrow
  scope tracking, generation matching)
- Code quality: confirm no test was left calling a deleted name;
  confirm no production code was missed by items 2-5

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
  variants; keep the trap-emitting variants (the audit cited both
  variants existing in parallel)
- Modify: `internal/component/resource_table_test.go` — delete tests of
  the silent behavior; replace with tests of trap behavior if missing
- Modify: any caller of the silent variants — migrate to the
  trap-emitting versions

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
> definition (vendored under `internal/component/wasip2test/.../wit/deps/`
> or `debug-vendored/WASI/proposals/`) and consult the spec. Then:
>
> 1. If the WIT method's return type is `result<_, error-code>`,
>    replace the silent-default with `result.err(<correct error-code
>    per the WIT enum>)`. The error-code must be the most accurate one
>    for the failure (e.g., `invalid-state` for a wrong handle type,
>    `not-permitted` for an authorization failure, `bad-descriptor` for
>    an invalid descriptor handle).
> 2. If the WIT method's return type does NOT have an error union,
>    replace the silent-default with a trap (return an error from the
>    Go function; the wazero machinery turns errors into traps).
> 3. **Never preserve the placeholder success.**

### Item 8: Fix `imports/wasip2/sockets/tcp.go` silent-default sites (22 sites)

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** -

**Files:**
- Modify: `imports/wasip2/sockets/tcp.go` — convert all 22 silent-default
  sites per the trap rule above
- Modify: `imports/wasip2/sockets/tcp_test.go` (if it exists; if it
  doesn't, create it) — add tests for each error path that asserts the
  spec-correct trap or `result.err`

**Spec authorities:**
- `debug-vendored/WASI/proposals/sockets/tcp.wit` — the WIT definitions
- `debug-vendored/WASI/proposals/sockets/network.wit` — for the
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

### Item 9: Fix `imports/wasip2/sockets/udp.go` silent-default sites (14 sites)

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Same trap rule as item 8

**Files:**
- Modify: `imports/wasip2/sockets/udp.go` — convert all 14 silent-default
  sites
- Modify: `imports/wasip2/sockets/udp_test.go` — add tests

**Spec authorities:**
- `debug-vendored/WASI/proposals/sockets/udp.wit`
- `debug-vendored/WASI/proposals/sockets/network.wit`
- `debug-vendored/wasmtime/crates/wasi/src/p2/host/udp.rs`

**Description:**
Same as item 8 but for UDP. 14 sites total.

**Definition of done:**
Same as item 8.

**Reviewer focus areas:**
Same as item 8.

---

### Item 10: Fix `imports/wasip2/http/http.go` silent-default sites (31 sites)

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Largest of the silent-default cleanups. Same trap rule as item 8.

**Files:**
- Modify: `imports/wasip2/http/http.go` — convert all 31 silent-default
  sites
- Modify: `imports/wasip2/http/http_test.go` — add tests

**Spec authorities:**
- `debug-vendored/WASI/proposals/http/types.wit`
- `debug-vendored/WASI/proposals/http/handler.wit`
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

5. Read `integration_public_api_test.go::TestPublicAPIAddS32`. Confirm
   it currently has a `t.Skipf("not fully wired yet")` (or similar).
   Remove the skip. Run the test. It should pass now that items 2-5
   wired the public API path.

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
