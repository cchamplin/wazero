# Canonical ABI Unification — Design

**Date:** 2026-04-07
**Status:** Approved for planning
**Branch:** `feat/wasip2-complete-implementation`

## Problem statement

Wazero's component-model implementation has accumulated **four parallel
canonical-ABI implementations**, **two parallel type representations** with
**four duplicate type converters** between them, and **systematic error
suppression** in the wasi-p2 host imports. The single most-correct
implementation (`internal/component/abi/`) is **dead code at runtime** — it has
zero non-test importers. Production code uses three other lift/lower paths
(`instance.go`, `component_linker.go`, `canon_lower.go`), each with confirmed
spec violations:

- Variant flat-join produces `f32` where the spec says `i32`
  (`component_linker.go:3747-3797`)
- Hard-coded `0` discriminant for variants in `writeResultsToMemory`
  (`component_linker.go:3292`)
- `f32`/`f64` lifted via Go numeric conversion instead of bit reinterpretation
  (`component_linker.go:2569,2571`)
- `liftRecord` sorts fields alphabetically (`instance.go:765`) — the spec
  iterates declared order
- `writeRecordToMemory` does not apply field alignment between fields
- `writeValToMemory` s16/u16 case writes 4 bytes but advances offset by 2
  (`component_linker.go:3402-3407`) — clobbers neighbouring fields
- Resource type validation trap missing on every `lift_own`/`lift_borrow`
- 67 silent-default-on-bad-handle sites in
  `imports/wasip2/sockets/{tcp,udp}.go` and `imports/wasip2/http/http.go`
  swallow handle-lookup errors and return placeholder success values

The 6,500-line `internal/component/abi/` package is mostly correct for the
synchronous canonical ABI: all primitives, strings (UTF-8/UTF-16/Latin1+UTF16),
records, tuples, variants (with proper join), options, results, enums, flags,
lists, NaN canonicalization, and standalone resource helpers. Its tests
(`abi/*_test.go`, ~5,000 lines) cover the implemented features comprehensively.
But it cannot be wired in as-is because it has key gaps (no `canon_lift`
top-level entry, no retptr param spill, no post-return invocation,
`types.Own`/`types.Borrow` missing from main dispatch) AND it consumes
`internal/component/types.ValType` whose shape is itself wrong in places
(`Own{ResourceIdx}` instead of `Own{*ResourceType}`, no `FixedSizeList`, no
`Stream`/`Future`/`ErrorContext`).

## Findings from research

Three independent investigations established the picture:

**1. The four type converters all share systemic problems.**
`(*TypeResolver).resolveDefinedType` (`type_resolver.go:172`) is the only one
with proper error handling, caching, and a sane fallback chain — but it still
omits `FixedSizeList`, `Stream`, `Future`, `ErrorContext`, and the
spec-mandated `*ResourceType` pointer for `Own`/`Borrow`. The other three
(`resolveToValType`, `typeDefToValType`, `valTypeRefToValType` in
`component_linker.go`) have **zero external production callers** — they only
call each other recursively from `resolveToValType`'s 4 callsites in the same
file. `valTypeRefToValType` has an actively dangerous bug: it returns
`types.U32{}` on lookup failure, silently turning a record into an i32.

**2. Wasmtime uses ONE type representation, not two.**
`wasmtime_environ::component::ComponentTypes` is built once during parse, then
frozen as `Arc<ComponentTypes>` and referenced unchanged through every layer:
`Component`, `Func::call`, `LiftContext`/`LowerContext`, `Val::lift`. The
`InterfaceType` enum (8 bytes: tag + `u32` index) is what lift/lower dispatches
on directly, with no conversion. Composites store precomputed
`CanonicalAbiInfo {size, align, flat_count}` so layout is never recomputed.
Resources are the only thing that cannot be compile-time baked: per-instance
`PrimaryMap<ResourceIndex, ResourceType>` is joined to the type table via a
two-pointer view struct (`InstanceType`).

**3. The Go ecosystem also uses ONE type representation.**
`go.bytecodealliance.org/wit` decodes JSON directly into a single type graph
via allocate-then-fill (`crates/wit/codec.go`). No converter exists.
`internal/wasm/module.go` — wazero's own core-wasm implementation — does the
same: the binary decoder populates `Module.TypeSection` directly during decode.
**Wazero's `internal/component` is the only place in the wazero codebase, and
the only Go component-model project surveyed, that maintains two parallel type
hierarchies and a converter.**

**Conclusion:** the right move is not to pick one of the four converters and
delete the other three. The right move is to collapse to one type
representation (matching wasmtime, `go.bytecodealliance.org/wit`, and wazero's
own internal precedent), at which point the converters all disappear because
there is nothing to convert.

## Architectural decisions

1. **One type representation.** `internal/component/types.ValType` becomes the
   single canonical form. The `internal/component/binary/` decoder populates it
   directly via allocate-then-fill, mirroring `internal/wasm/module.go` and
   `go.bytecodealliance.org/wit/codec.go`. The four converter functions are
   deleted, not unified.

2. **`internal/component/abi/` is the single source of truth for lift/lower.**
   After Loop 2 completes, no file in `internal/component` outside `abi/` and
   `binary/` contains lift/lower logic. Production code calls
   `abi.CanonLift`/`abi.CanonLower` for every operation.

3. **`Own`/`Borrow` carry `*ResourceType`, not `ResourceIdx`.** Spec authority:
   `definitions.py:1641` `lower_own(cx, rep, t)` takes the full `OwnType(rt)`
   with `rt: ResourceType`. Wasmtime authority:
   `crates/environ/src/component/types.rs` `TypeResourceTable` joins the index
   to a real `ResourceType`. Existing TODOs in
   `internal/component/types/resource.go` lines 14-16 and 35-38 acknowledge
   this is wrong.

4. **`lift_own`/`lift_borrow` are inner cases of unified dispatch, not
   standalone helpers.** Spec authority: `definitions.py:1197-1198,1387-1388,
   1792-1793,1886-1887` show these dispatched from `load`/`store`/`lift_flat`/
   `lower_flat`. Wasmtime authority:
   `crates/wasmtime/src/runtime/component/values.rs:115` matches
   `InterfaceType::Own(_) | InterfaceType::Borrow(_)` inside the same `lift`
   function as every other type. Existing standalone `LiftOwn`/`LiftBorrow`/
   `LowerOwn`/`LowerBorrow` helpers in `abi/` are non-canonical and get folded
   into integrated dispatch then deleted.

5. **Async, streams, futures, error-context, threads, subtasks, cancellation,
   and any async-flavored interactions with post-return are explicitly out of
   scope.** They are tracked for a future spec under
   `docs/plans/abi-unification-async/`. The synchronous canonical ABI —
   including the synchronous form of `canon post-return` per
   `definitions.py:3197+` — is in scope and is large enough to be a coherent
   unit of work on its own. `Stream`/`Future`/`ErrorContext` value types are
   added as recognised cases in `types.ValType` that **trap on lift/lower**
   with "async not yet supported" — this documents the surface and keeps the
   parser producing complete output.

6. **`debug-vendored/` is read-only spec authority.** Nothing in
   `debug-vendored/` is symlinked into production or test paths. Agents
   consult it to verify correctness; copies (with `// SOURCE:` provenance
   headers and the upstream git SHA) are the only way data flows from it into
   the wazero source tree.

7. **No async runtime, no scheduler, no `RacyBool`, no `Subtask` machinery.**
   The 22 scenario-style Python tests in `run_tests.py` (async, streams,
   cancellation, futures, threads) are NOT ported in this project. They are
   listed as future work in `docs/plans/abi-unification-async/`.

## Three loops, strict sequential ordering

### Loop 1 — Unify type representation, then make `abi/` correct

**Goal:** wazero has ONE type representation. The binary parser populates
`internal/component/types.ValType` directly. The four converters are gone.
`abi/` correctly implements the synchronous canonical ABI as defined by
`debug-vendored/component-model/design/mvp/canonical-abi/definitions.py` and
`CanonicalABI.md`, with full Python test parity plus wazero supplemental tests.

**Six phases, ~35 items:**

- **1.A — Type unification (~11 items).** Add `FixedSizeList`, `Stream`,
  `Future`, `ErrorContext` to `types.ValType`. Change `Own`/`Borrow` to carry
  `*ResourceType`. Add `Refines` to `Variant.Case`. Add `CanonicalAbiInfo`
  cache. Refactor `binary/` parser to populate `types.ValType` directly via
  allocate-then-fill. Delete `binary.TypeDef` and the parallel
  `binary.{Record,Variant,List,...}TypeDef` structs. Delete the four
  converters. Reviewer subagent confirms the new `types.ValType` shape matches
  the spec for every type category.

- **1.B — Test infrastructure (~3 items).** Create
  `internal/component/conformance/canonical_abi/` with table-driven helpers
  matching `run_tests.py`'s `test`/`test_pairs`/`test_heap`/`test_flatten`/
  `test_nan*`/`test_string`/`test_roundtrip`/`test_reentrance`. Define spec
  constants (`MaxFlatParams=16`, `MaxFlatResults=1`, `CanonicalFloat32NaN`,
  `Utf16Tag`, etc.) with citations.

- **1.C — Port Python tests, expected to fail (~9 items).** Port ~272
  data-driven cases from `run_tests.py`: primitive coercion (~55),
  lift/lower roundtrip (23), heap layout (31), flatten signatures (8),
  NaN canonicalization (14), string encoding matrix (135), `test_roundtrips`
  (6), `test_handles` (~50 asserts), `test_reentrance` (~12).

- **1.D — Fill `abi/` gaps to make tests green (~8 items).** Add
  `types.Own`/`types.Borrow` to `LiftFlat`/`LowerFlat`/`LiftHeap`/`LowerHeap`
  dispatch with resource-type validation. Add `CanonLift`/`CanonLower` entry
  points with retptr param spill (>16 flat) and result spill. Add post-return
  invocation per spec 3197+. Add lower-side list size overflow check. Add NaN
  scrambling on store. Add canonical-options pre-flight validation. Add
  `FixedSizeList` dispatch.

- **1.E — Wazero supplemental tests (~2 items).** Float bit-exact equality,
  byte-slice aliasing across realloc, deeply-nested record alignment, retptr
  boundary at exactly 16/17 flat params, FixedSizeList edge cases. Each must
  cite the spec section it exercises and not contradict any Python-ported
  case. Reviewer subagent confirms.

- **1.F — Termination (~2 items).** Verifier confirms 100% of ported tests
  green, no skips. Spec-coverage verifier produces
  `loop-1-spec-coverage-report.md` listing every CanonicalABI.md section as
  implemented / deferred / N/A.

### Loop 2 — Wire `abi/` into production, delete dead code and dead tests

**Goal:** Production runtime calls `abi.CanonLift`/`abi.CanonLower` for every
lift/lower operation. The three parallel implementations are deleted along
with their tests. The 67 silent-default sites in wasip2 sockets/http trap or
return `result.err(...)` correctly.

**Six phases, ~16 items:**

- **2.A — Mapping (1 item).** Map every lift/lower call site in
  `instance.go`, `component_linker.go`, `canon_lower.go`, `linker.go`,
  `linker_api.go`, `value_import_test.go`. Output:
  `loop-2-call-site-map.md`. Reviewer verifies completeness.

- **2.B — Wire host-import path (3 items).** Replace `LoweredFunc.CallWithStack`
  body, `createCanonLowerFunc` body, and `writeResultsToMemory` with thin
  shims to `abi.CanonLift`/`abi.CanonLower`. The dependent helpers
  (`liftFromStack`, `flattenVariantType`, `isWiderValueType`, `writeValToMemory`,
  `writeRecordToMemory`) lose all callers and are deleted with their tests in
  the same commits.

- **2.C — Wire guest-export path (1 item).** Replace `instance.go`
  `ExportedFunc.Call` family lift/lower with `abi.CanonLift`/`abi.CanonLower`.
  Delete `liftRecord` (alphabetical sort), `liftResolvedType`, and the
  retptr-as-return-value heuristic at instance.go:305-322. Replace with
  spec-correct retptr handling from `abi.CanonLift`. Delete tests.

- **2.D — Resource handle cleanup (2 items).** Delete the standalone
  `LiftOwn`/`LiftBorrow`/`LowerOwn`/`LowerBorrow` helpers in `abi/` (folded
  into integrated dispatch in Loop 1.D). Delete
  `ResourceTable.CreateResourceDropFunc` and `CreateResourceRepFunc`
  (silent-ignore variants). Migrate callers to trap-emitting versions.

- **2.E — Fix wasip2 silent-default error suppression (4 items, ~67 sites).**
  Trap rule: if the WIT method's return type is `result<_, error-code>`,
  return `result.err(<correct error-code per the WIT enum>)`. If no error
  union, trap. Never preserve placeholder success. Apply to
  `imports/wasip2/sockets/tcp.go` (22 sites), `udp.go` (14 sites),
  `imports/wasip2/http/http.go` (31 sites), and audit
  `imports/wasip2/{filesystem,clocks,random,cli,io}/*.go` for the same
  pattern.

- **2.F — Termination & test cleanup (5 items).** Dead-code & dead-test sweep
  via `Grep` for every removed function name; zero references repo-wide
  including test files. Test rework verification: zero new `t.Skip`, zero new
  `// TODO`, `TestPublicAPIAddS32` no longer skipped. Full `go test ./...`
  green. Spec-compliance reviewer re-reads wired paths against `definitions.py`
  `canon_lift`/`canon_lower`. Code-quality reviewer re-reads every modified
  file.

### Loop 3 — Real wasm via public API, migrate existing tests

**Goal:** End-to-end validation through `github.com/tetratelabs/wazero` +
`api` + `api/component` + `imports/wasip2`. No test in the validation suite
imports `internal/component`. Real-world WIT-defined components from upstream
sources drive the canonical ABI through the public surface.

**Four phases, ~18 items:**

- **3.A — Spec WAST suite (3 items).** Verify the existing
  `internal/component/spectest/` runner consumes upstream `.wast` format.
  Copy 69 files from `debug-vendored/component-model/test/` into
  `internal/component/spectest/testdata/upstream/` (no symlinks; `// SOURCE:`
  provenance headers; preserve directory structure). Wire `spectest_test.go`
  to discover and run every upstream file. Async-touching files get
  documented `t.Skipf` per Loop 1's deferral; everything else must pass.

- **3.B — wit-bindgen runtime suite (3 items).** Copy 33 cases from
  `debug-vendored/wit-bindgen/tests/runtime/` into
  `internal/component/wasip2test/upstream/wit-bindgen/<case>/`. Each case:
  `test.wit` verbatim, built `component.wasm`, `BUILD.md` recording build
  commands and upstream SHA. Add `upstream_wit_bindgen_test.go` —
  public-API-only Go test using `wazero.NewRuntime`,
  `wazero.NewComponentLinker`, `wasip2.MergeInto`, `linker.DefineInstance`,
  `runtime.CompileComponent`, `runtime.InstantiateComponent`. **No
  `internal/component` imports.**

- **3.C — Migrate existing wasip2test files to public API (9 items).**
  `calculator_test.go`, `composition_test.go`, `converter_test.go`,
  `kv_store_test.go`, `large_record_test.go`, `linking_test.go`,
  `nested_types_test.go`, `variant_types_test.go`, `wasi_exercise_test.go`.
  Each: replace `internal/component` imports with `github.com/tetratelabs/wazero`
  + `api/component`. Delete `testutil` imports.

- **3.D — Test surface verification (3 items).** Public API audit subagent:
  `Grep` for `internal/component` in `_test.go` files; expected zero matches
  outside `abi/`, `conformance/`, `spectest/`. Coverage audit subagent:
  confirm at least one upstream `.wast` or wit-bindgen case touches each
  `abi/` entry point (lift/lower of every type, retptr spills, post-return,
  resource trap, realloc trap). Final verifier: `go test ./...` green,
  `TestPublicAPIAddS32` not skipped, no `t.Skip` outside the documented async
  deferral list.

## Item lifecycle (applies to every code-changing item in every loop)

```
pending → claimed → implementing → spec-review → code-review → done
                          ↑                ↓             ↓
                          └────────────────┴─────────────┘
                          (blockers from either reviewer
                           bounce back to implementing)
```

1. Driver claims the item (updates tracking file).
2. Implementation subagent reads `spec-authorities.md` + cited spec files
   BEFORE writing code. Performs red/green TDD where applicable.
3. **Two reviewer subagents in parallel, both with fresh context:**
   spec-compliance reviewer (reads `definitions.py`/`CanonicalABI.md`/wasmtime
   cross-references) and code-quality reviewer (reads existing wazero patterns).
4. If either posts BLOCKER: bounce back to implementation; loop until clean.
5. Commit code change AND tracking-file update (`claimed → done`) in ONE
   commit. Commit message cites the spec file:line the item addresses.
6. Whole-loop final sweep (Phase F's verifier subagents) is independent of
   per-item review and catches cross-cutting issues that per-item review can't
   see.

## Loop driver, templates, and tracking

Layout under `docs/plans/abi-unification/`:

```
docs/plans/abi-unification/
├── README.md                          # design summary, links to this file
├── spec-authorities.md                # mandatory reading list
├── loop-driver.md                     # universal session-start prompt
├── loop-1-tracking.md                 # ~35-item backlog
├── loop-2-tracking.md                 # ~16-item backlog
├── loop-3-tracking.md                 # ~18-item backlog
└── templates/
    ├── implement-task.md              # implementation subagent prompt
    ├── review-spec-compliance.md      # spec reviewer prompt
    ├── review-code-quality.md         # code-quality reviewer prompt
    └── verify-loop-complete.md        # terminal sweep prompt
```

`spec-authorities.md` lists the mandatory reading for every agent session:

- **Synchronous canonical ABI:**
  `debug-vendored/component-model/design/mvp/CanonicalABI.md`,
  `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py`,
  `debug-vendored/component-model/design/mvp/canonical-abi/run_tests.py`
- **Wasmtime cross-reference:**
  `debug-vendored/wasmtime/crates/environ/src/component/types.rs`,
  `crates/wasmtime/src/runtime/component/values.rs`,
  `crates/wasmtime/src/runtime/component/func/typed.rs`,
  `crates/wasmtime/src/runtime/component/func/options.rs`
- **Go ecosystem precedent:**
  `debug-vendored/go-modules/wit/codec.go`,
  `debug-vendored/go-modules/wit/typedef.go`,
  `internal/wasm/module.go`
- **WASI WIT schemas (for Loop 2 phase 2.E trap rule):**
  `debug-vendored/WASI/proposals/sockets/*.wit`,
  `debug-vendored/WASI/proposals/http/*.wit`,
  `debug-vendored/WASI/proposals/filesystem/*.wit`

**Rules every agent receives:**

1. **Spec wins.** If your training data, prior commit history, or local code
   disagrees with the spec text, the spec text is correct and the code is
   wrong.
2. **No symlinks.** Anything you need from `debug-vendored/` is read into your
   context or copied with provenance headers.
3. **Cite the file:line you are implementing** in the commit message and in
   code comments where non-obvious.
4. **If the spec is ambiguous on a question**, consult wasmtime AND check that
   the chosen interpretation is consistent. If still ambiguous, escalate to
   the user — do not invent.

## Tracking file format

Each item is one Markdown row with explicit lifecycle state:

```markdown
- [ ] **Item 24** — Add types.Own/types.Borrow dispatch to LiftFlat/LowerFlat
  - status: pending
  - claimed_by: -
  - spec_review: -
  - code_review: -
  - commit: -
  - notes: Spec authority: definitions.py:1197-1198, 1387-1388;
    wasmtime values.rs:115
```

Becomes when done:

```markdown
- [x] **Item 24** — Add types.Own/types.Borrow dispatch to LiftFlat/LowerFlat
  - status: done
  - claimed_by: 2026-04-08 session
  - spec_review: 2026-04-08 PASSED (cited definitions.py:1197 match)
  - code_review: 2026-04-08 PASSED (1 nit fixed)
  - commit: abc1234
  - notes: ResourceType validation trap added per spec 2218-2219
```

This is the source of truth for resumability across sessions.

## Termination criteria for the project

The project is complete when:

1. Every item in Loop 1, Loop 2, and Loop 3 tracking files has
   `status: done` with both reviewers `PASS`.
2. Each loop's `verify-loop-complete.md` final sweep has been run and the
   resulting `loop-N-completion-report.md` is committed.
3. `go test ./...` from the repo root is green.
4. No `internal/component` imports exist in `_test.go` files outside the
   allow-list (`abi/`, `conformance/`, `spectest/`).
5. No file in `internal/component` outside `abi/` and `binary/` contains
   lift/lower logic.
6. The four converter functions (`resolveToValType`, `typeDefToValType`,
   `valTypeRefToValType`, `(*TypeResolver).resolveDefinedType` and helpers)
   are gone.
7. The standalone `LiftOwn`/`LiftBorrow`/`LowerOwn`/`LowerBorrow` helpers in
   `abi/` are gone.
8. The 67 silent-default-on-bad-handle sites in
   `imports/wasip2/sockets/{tcp,udp}.go` and `imports/wasip2/http/http.go` are
   gone.
9. `internal/component/integration_public_api_test.go::TestPublicAPIAddS32`
   is no longer `t.Skipf`'d.
10. A `loop-3-coverage-matrix.md` exists confirming every `abi/` entry point
    is exercised by at least one test that uses only the public API.

## Out of scope

- Async, streams, futures, error-context, threads, subtasks, cancellation,
  task return, waitable sets, callbacks, backpressure
- The 22 scenario-style Python tests in `run_tests.py` that exercise the above
- WAST files in `debug-vendored/component-model/test/async/` (skipped with
  documentation in Loop 3.A)
- wit-bindgen runtime cases that exclusively exercise async features
- Memory64 component-model support (`FIXME(#4311)` in wasmtime)
- Component-model linker improvements beyond what's needed to call
  `abi.CanonLift`/`abi.CanonLower`

These become a follow-up project under
`docs/plans/abi-unification-async/` opened after this project closes.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Loop 1 phase 1.A leaves the build broken until Loop 2 wires things in | Documented in `loop-1-unification-status.md`; Loop 2 starts immediately after Loop 1 phase 1.F closes; expected failures are tracked, not silently accepted |
| Reviewer subagents accept incorrect implementations because they share the same wrong training-data prior | `spec-authorities.md` mandates reading the spec files first; reviewers cite file:line in their findings; if a reviewer cannot cite, the finding is invalid |
| The 33 wit-bindgen cases include async-only ones with no synchronous fallback | Document each excluded case in `loop-3-async-deferred.md` with the specific WIT feature that triggers exclusion |
| Type unification surfaces previously-hidden parser bugs in `internal/component/binary/` | Reviewer in Loop 1 phase 1.A item 11 explicitly verifies the parser-produced types against the spec for each category; bugs are fixed in the same item, not deferred |
| Per-item dual review doubles the agent count per item | Acceptable cost — correctness over throughput, per project ground rules |
| Loop 2 deletion of dead code accidentally removes a function still used by something the agent didn't grep for | `verify-loop-complete.md` runs a final repo-wide `Grep` for every removed name; CI's full test suite is the second backstop |

## What this design does NOT prescribe

- **Specific Go function signatures for `abi.CanonLift`/`abi.CanonLower`.** The
  implementation subagent in Loop 1 phase 1.D items 25-26 picks the signature
  by reading `definitions.py:3237` and `func/typed.rs::call_raw` and selecting
  the shape that fits idiomatic Go and existing wazero patterns.
- **Exact line counts for the deletion sweeps.** Phase 2.F item 12 produces
  `loop-2-deletion-report.md` with the actual numbers.
- **Whether `FixedSizeList` is a distinct `types.ValType` case or merges into
  the existing `List{Length *uint32}`.** The agent in Loop 1 phase 1.A item 1
  decides after reading both shapes.
- **The exact name and shape of `CanonicalAbiInfo`.** The agent in Loop 1
  phase 1.A item 5 picks one consistent with `abi/`'s existing API.

These are deliberate. The design fixes the architectural decisions and
prohibits drift, but lets implementation choices live in the implementation
phase where the spec text and reviewer feedback inform them.
