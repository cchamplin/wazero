---
title: Canonical ABI Unification — Design Spec
date: 2026-04-07
status: approved-for-planning
project_dir: docs/superpowers/projects/2026-04-07-canonical-abi-unification/
plan_doc: docs/superpowers/plans/2026-04-07-canonical-abi-unification.md (to be written by writing-plans skill)
---

# Canonical ABI Unification — Design Spec

## 0. Executive summary

The wazero fork currently has **four parallel implementations** of canonical-ABI lifting and lowering. The most spec-compliant of the four — `internal/component/abi/` — is **dead code at runtime**: it is imported only by tests under `internal/component/conformance/`, never by production. The three implementations that production actually uses — `internal/component/instance.go` (`ExportedFunc.Call` family), `internal/component/component_linker.go` (`createCanonLowerFunc` + `writeResultsToMemory` family), and `internal/component/canon_lower.go` (`LoweredFunc.CallWithStack`) — collectively contain at least 14 verified correctness bugs against the canonical ABI spec, and `imports/wasip2/sockets/{tcp,udp}.go` and `imports/wasip2/http/http.go` contain ~77 silent-default-on-bad-handle sites that should trap per spec.

This project unifies all canonical-ABI work onto `internal/component/abi/`, reconciles every function in that package against the canonical reference implementation `definitions.py` from `debug-vendored/component-model/design/mvp/canonical-abi/`, builds a Go port of the synchronous test functions from `run_tests.py` as the correctness oracle, migrates all production runtime call sites to call into `abi/`, deletes every parallel implementation, and converts the wasip2 silent-error sites to trap-emitting sites. The synchronous canonical ABI is fully implemented; asynchronous primitives (streams, futures, error-context, threads) are out of scope and stubbed with explicit-spec-citation error messages.

The work is structured as three sequential, fully-isolated loops with hard rules preventing context contamination between layers. Every gate is mechanical. Every code change goes through a mandatory two-stage review chain by fresh subagents (spec compliance, then code quality). Every agent and subagent reads the spec text before doing anything. The end state is a wazero tree with a single canonical-ABI engine, zero parallel implementations, zero silent error swallows, zero `panic("not implemented")` in canonical-ABI or wasip2 paths, and a fully green `go test ./...`.

The user has explicitly accepted that this work is expensive in time and compute. Correctness and comprehensiveness are the only optimization targets. Speed, parallelism, batching, simplicity, and "saving tokens" are not.

## 1. Background and verified problem statement

A four-agent research pass on 2026-04-06 produced the audit reports under `docs/research/canonical-abi-audit-2026-04-06/` (uncommitted). The findings most relevant to this design were independently verified by a fresh exploration pass on 2026-04-07:

**1.1 The dead-code package.** `internal/component/abi/` exports 19 public functions (`LiftFlat`, `LowerFlat`, `LiftHeap`, `LowerHeap`, `LiftString`, `LowerString`, `LiftOwn`, `LiftBorrow`, `LowerOwn`, `LowerBorrow`, `LowerOwnWithType`, `LowerBorrowWithType`, `FlattenParams`, `FlattenResults`, `CoreSignature`, `NewFlatIter`, plus context types). Tree-wide grep for callers of these symbols outside the `abi` package itself returns hits only in `internal/component/conformance/*_test.go` (13 test files). Zero production callers.

**1.2 Three parallel runtime implementations.** Production canonical-ABI work happens in:

- `internal/component/instance.go` — `ExportedFunc.Call` (entry: line 133), with helpers `liftResolvedType` (794), `liftFieldFromMemory` (1242), `liftResultFromMemory` (1406), `liftRecord` (757), `lowerParam` (1518), `lowerTyped` (1601), `lowerByKind` (1856), `lowerStringParam` (1964), `liftOwn` (2312), `liftBorrow` (2353), `elementSizeForKind` (2769), `alignmentForKind` (2785), `sizeOfVal` (2802), and a retptr-as-return-value heuristic at 305-322 that has no spec basis.

- `internal/component/component_linker.go` — `createCanonLowerFunc` (entry: line 2430), with helpers `liftFromStack` (2545), `liftRecordFromStack` (2680), `liftOptionFromStack` (2693), `liftVariantFromStack` (2990), `lowerToStack` (3072), `writeResultsToMemory` (3157), `writeRecordToMemory` (3369), `writeValToMemory` (3387), `flattenVariantType` (3745), `elemSizeFromTypeRef` (2851), `elemAlignFromTypeRef` (2878), `recordSize` (2916), `optionSize` (2940), `variantSize` (2969).

- `internal/component/canon_lower.go` — `LoweredFunc.CallWithStack` (entry: line 201), with helpers `liftArguments`, `liftArgumentsTyped`, `liftValFromFlat`, `liftString`, `lowerResults`, `lowerResultsTyped`, `lowerValToFlatTyped`, `lowerString`. Handles primitives + string only; falls through to runtime trap on composite types.

**1.3 Verified bug sites.** The audit catalogued, with line citations, at minimum:

- `linker.go:3747-3797` — `flattenVariantType` flat-join semantics produce f32 where spec says i32.
- `linker.go:3292` — hard-coded variant discriminant 0 in `writeResultsToMemory`.
- `linker.go:2569,2571` — f32/f64 lifted via Go numeric cast (`float32(stack[0])`) instead of `math.Float32frombits`/`Float64frombits`. For any non-zero bit pattern this produces a different value.
- `instance.go:1335-1354` — `liftFieldFromMemory` reads 4 bytes for Enum/Flags regardless of count (spec mandates 1/2/4 by case count).
- `instance.go:1355-1365` — `liftFieldFromMemory` reads 4-byte Option discriminant (spec: 1 byte).
- `instance.go:1385-1393` — `liftFieldFromMemory` Record branch missing field alignment.
- `instance.go:757-790` — `liftRecord` sorts field names alphabetically before reading (spec: declared order).
- `instance.go:1940-1957` — `lowerByKind` sorts record fields alphabetically before lowering (spec: declared order).
- `instance.go:1084-1113` — flat variant lift uses `coreResults[1]` directly without join coercion.
- `instance.go:1757-1799` — `lowerTyped` variant missing coerce to joined type.
- `instance.go:807-814` — flat record lift assumes one core value per field (broken for fields containing string/list/composite).
- `linker.go:3402-3407` — `writeValToMemory` ValKindS16/U16 case writes 4 bytes via `WriteUint32Le` but advances offset by 2, clobbering the next field.
- `linker.go:3369-3384` — `writeRecordToMemory` does not apply field alignment between fields.
- `linker.go:3332-3346` — `writeResultsToMemory` ValKindFlags always writes 4 bytes regardless of flag count.
- `abi/lift.go:702-707` and `instance.go:2312-2388` — `lift_own`/`lift_borrow` missing the `trap_if(h.rt is not t.rt)` resource type check.
- All runtime string paths hardcode UTF-8; UTF-16 and Latin1+UTF16 are unhandled.
- `internal/component/resource_table.go` exports both silent-ignore (`CreateResourceDropFunc`, `CreateResourceRepFunc`) and trap-emitting (`CreateResourceDropFuncWithTrap`, `CreateResourceRepFuncWithTrap`) variants of resource handle ops.

**1.4 Wasip2 silent-error sites.** ~77 sites in `imports/wasip2/sockets/tcp.go` (~30), `imports/wasip2/sockets/udp.go` (~12), and `imports/wasip2/http/http.go` (~35) follow the pattern: call `getTcpSocket(ctx, handle)` (or `getUdpSocket`, `getFields`, etc.) → on error, return a placeholder success value (`ValResultOk(nil)`, `ValBool(false)`, `ValOption(nil)`, `ValOwn(0)`) instead of returning `(nil, error)` which would trap via `createCanonLowerFunc`'s panic-on-error path. The trap pattern is established in recent commits (eb632848 for UDP send, d71ffbf3 for HTTP response-outparam.set, 3f91ed37 preserving panic-on-error in writeResultsToMemory).

**1.5 Reference materials available in `debug-vendored/`.**

- `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py` — 2,609 lines, the official canonical ABI reference implementation. The single source of truth for canonical ABI semantics that this project reconciles against.
- `debug-vendored/component-model/design/mvp/canonical-abi/run_tests.py` — 2,831 lines, 31 test functions, the official spec test suite. The synchronous subset (`test_pairs`, `test_nan32`, `test_nan64`, `test_string`, `test_heap`, `test_flatten`, `test_roundtrips`, `test_handles`, `test_self_copy`, `test_reentrance`) is in scope.
- `debug-vendored/component-model/design/mvp/CanonicalABI.md` — the canonical ABI specification text. Cited in every doc comment, every reconciliation report, every review.
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/` — Rust reference implementation. Used as a tertiary reference when spec text is ambiguous.
- `debug-vendored/component-model/test/wasmtime/`, `debug-vendored/wit-bindgen/tests/runtime/`, `debug-vendored/wasmtime/tests/disas/component-model/` — vendored .wasm and .wat test fixtures usable for differential testing against the installed `wasmtime` CLI.

## 2. Decisions made during brainstorming

These were resolved through structured Q&A with the user. They constrain everything that follows.

**2.1 Architectural target.** Reconcile `internal/component/abi/` line-by-line against `definitions.py` and use it as the single canonical-ABI engine. (Option B in the brainstorm — keep abi/ as the unification target but verify every function against the reference before wiring.)

**2.2 Phase 1 scope.** Synchronous canonical ABI only. Asynchronous primitives (canon stream.*, canon future.*, canon error-context.*, canon thread.*, canon waitable-set.*, canon task.return/cancel, canon subtask.cancel/drop, canon context.get/set, callbacks) are out of scope. Every async entry point in `abi/` is a stub that returns an explicit error of the form `fmt.Errorf("canonical ABI: <op> requires <spec-primitive>, defined in CanonicalABI.md §<section>, not implemented in wazero")`. No panics. No silent no-ops. No placeholder values. (Option A.)

**2.3 Test corpus structure.** One Go test function per Python test function in `run_tests.py`, with table-driven `t.Run(name, ...)` subtests inside. Header comment cites the Python source line range for mechanical diffability if `run_tests.py` is updated upstream. Async test functions are scaffolded as `t.Skip(...)` placeholders with explicit messages naming the missing spec primitive and the spec section it is defined in. Skip messages do not say "phase 2" or "TODO" — they cite the missing primitive and the spec section. (Option C.)

**2.4 Loop structure.** Three sequential loops with hard isolation:
- **Loop 1 — abi/ correctness (spec-only).** Verifies abi/ exclusively against `definitions.py`, the Go port of `run_tests.py`, the wasmtime CLI on standalone fixtures, and `CanonicalABI.md`. Cannot read or load any production runtime path.
- **Loop 2 — runtime migration (migration-only).** Replaces every parallel canonical-ABI helper in `instance.go` / `component_linker.go` / `canon_lower.go` with calls into `abi/`. Cannot edit `abi/`.
- **Loop 3 — wasip2 cleanup.** Converts every silent-error site in `imports/wasip2/` to `(nil, error)` trap. Cannot edit `internal/component/`.

Hard isolation prevents subagents from rationalizing changes to `abi/` to match buggy runtime expectations and prevents Loop 2/3 from contaminating the verified canonical engine. (Option A in the brainstorm; Options B and C were explicitly rejected because they allowed contamination.)

**2.5 Verification gates.** Six gates govern Loop 1; their definitions are in §5. The dropped candidates from the brainstorm: property/fuzz testing (Gate 5, dropped), 100% line/branch coverage threshold (Gate 8, dropped), building wasmtime from source (replaced with installed CLI). Gate 7 (spec-text traceability) is kept in lightweight form.

**2.6 Deliverable shape.** Prompts, scripts, templates, and orchestration artifacts live under `docs/superpowers/projects/2026-04-07-canonical-abi-unification/` (uncommitted). The spec (this document) and the plan (writing-plans output) live under `docs/superpowers/specs/` and `docs/superpowers/plans/` (committed). Real Go test files produced during Loop 1 are committed to `internal/component/conformance/` per iteration. The Python differential driver `harness/spec_diff_driver.py` is real Python code that lives uncommitted under the project dir.

**2.7 Mandatory review chain.** Every code change goes through R1 (spec compliance review by a fresh subagent) → R3 (code quality review by a different fresh subagent), with revisions performed by yet other fresh subagents. No self-review. No batching. No grouping. No skipping. No deferring. After any revision, the chain restarts at R1. Details in §6.

**2.8 Spec-overrides-local-instructions rule.** Every agent and subagent reads the relevant spec section and `definitions.py` function before acting. If any local instruction (prompt, template, in-tree comment, prior commit, status file) conflicts with the spec or `definitions.py`, the spec wins. Verbatim text in §7 — embedded in every template, every loop prompt, and every iteration script.

## 3. Architecture

### 3.1 End state

A single canonical-ABI engine in `internal/component/abi/` is the only place in the codebase that performs canonical-ABI lifting, lowering, flattening, sizing, alignment, store/load, resource handle ops, and string-encoding conversion. Every production runtime path that needs canonical-ABI work calls into `abi/`. No parallel implementation of any canonical-ABI operation exists anywhere else in the tree.

### 3.2 The canonical engine — `internal/component/abi/`

The package keeps its current Go package name and its current public type names where they don't conflict with `definitions.py`. Where the existing names diverge from the spec/Python names without spec justification, they are renamed to match. Goal: any reader can put `abi/lift.go` and `definitions.py` side-by-side and read them as the same algorithm in two languages.

For each public function in `abi/`, before it backs any production runtime path:

1. The Go signature is reconciled function-by-function against the corresponding `definitions.py` function (Gate 1).
2. The function body is reconciled branch-by-branch.
3. Differences are either fixed or documented in a per-function reconciliation report (markdown, lives uncommitted in the project dir under `status/reconciliation/<func>.md`) with explicit citation to either `CanonicalABI.md` or `definitions.py` justifying any intentional deviation. "Wazero handles this differently because of <reason>" is not acceptable without a spec citation.
4. Spec citation comments are added (Gate 7) in the form `// Spec: CanonicalABI.md §<section>` and `// Reference: definitions.py:<func> (lines <start>-<end>)` and `// Reconciled <YYYY-MM-DD>`.

The synchronous portion of `definitions.py` is fully ported. The asynchronous portion is **not** ported. Every async entry point is a stub returning an explicit error per §2.2.

### 3.3 Production runtime paths that call into the engine

After Loop 2:

- **`internal/component/canon_lower.go`** — `LoweredFunc.CallWithStack` shrinks to a thin wrapper that constructs an `abi.LiftContext` / `abi.LowerContext` and calls `abi.LiftFlat` / `abi.LowerFlat`. All in-file lift/lower helpers deleted.
- **`internal/component/component_linker.go`** — `createCanonLowerFunc` becomes glue that prepares `abi/` contexts and dispatches into `abi/`. All in-file lift/lower/size/align helpers deleted, including `liftFromStack`, `liftRecordFromStack`, `liftOptionFromStack`, `liftVariantFromStack`, `lowerToStack`, `writeResultsToMemory`, `writeRecordToMemory`, `writeValToMemory`, `flattenVariantType`, `elemSizeFromTypeRef`, `elemAlignFromTypeRef`, `recordSize`, `optionSize`, `variantSize`.
- **`internal/component/instance.go`** — `ExportedFunc.Call` becomes glue that prepares `abi/` contexts from a wazero `Instance`, invokes the core function, and unpacks results via `abi/`. All in-file lift/lower/size/align helpers deleted, including `liftResolvedType`, `liftFieldFromMemory`, `liftResultFromMemory`, `liftRecord`, `lowerParam`, `lowerTyped`, `lowerByKind`, `lowerStringParam`, `liftOwn`, `liftBorrow`, `elementSizeForKind`, `alignmentForKind`, `sizeOfVal`. The `instance.go:305-322` retptr heuristic is deleted; whatever it was working around is replaced by the spec-correct retptr handling in `abi/`.

### 3.4 Resource handle path

`internal/component/resource_table.go` retains a single set of trap-emitting handle operations. The silent-ignore `CreateResourceDropFunc` and `CreateResourceRepFunc` are deleted. Every test currently using the silent variants is updated to use the trap variants and to assert the expected trap. Resource handle creation, drop, and rep operations are exposed only through `abi.LiftOwn` / `abi.LiftBorrow` / `abi.LowerOwn` / `abi.LowerBorrow` / `abi.LowerOwnWithType` / `abi.LowerBorrowWithType`.

### 3.5 Wasip2 cleanup (Loop 3 scope)

The ~77 silent-default sites in `imports/wasip2/sockets/{tcp,udp}.go` and `imports/wasip2/http/http.go` are mechanically converted to return `(nil, error)` so that `createCanonLowerFunc`'s panic-on-error path traps. Each site gets a wiring test asserting the trap is delivered to the guest correctly.

### 3.6 Conformance test package handling

The Go port of `run_tests.py` lives in the existing `internal/component/conformance/` package alongside the existing topic-organized tests. New files use a `spec_` filename prefix (`spec_pairs_test.go`, `spec_string_test.go`, etc.) to mark them as direct ports of `definitions.py`/`run_tests.py`.

The existing topic-organized tests in `conformance/` are themselves audited as part of Loop 1, before any function-reconciliation work begins. Each existing `*_test.go` file is checked assertion-by-assertion against `definitions.py`'s output for the same input. Tests that audit clean stay (with a header comment `// Audited <date> against definitions.py — assertions match canonical reference.`). Tests that don't audit clean are fixed, because a test that locks in a wazero bug as expected behavior is worse than no test at all.

### 3.7 Final tree state

After Loops 1, 2, and 3:

- `internal/component/abi/` — full synchronous canonical-ABI engine, every function reconciled to `definitions.py`, every async stub trapping with explicit messages.
- `internal/component/canon_lower.go` / `component_linker.go` / `instance.go` — thin glue, no in-file lift/lower code.
- `internal/component/resource_table.go` — single trap-emitting API.
- `internal/component/conformance/` — existing tests audited; new `spec_*_test.go` files cover every sync `run_tests.py` test function.
- `imports/wasip2/sockets/{tcp,udp}.go` and `imports/wasip2/http/http.go` — every error path traps via `(nil, error)` return.
- Zero parallel canonical-ABI implementations.
- Zero silent-default-on-bad-handle sites in wasip2.
- Zero `panic("not implemented")` in any canonical-ABI or wasip2 code path.

## 4. Two-loop separation with hard isolation

### 4.1 Loop 1 — abi/ correctness (spec-only)

**Goal.** Every sync function in `internal/component/abi/` is reconciled against `definitions.py`, every sync test from `run_tests.py` is ported to `internal/component/conformance/spec_*_test.go` and passing, every gate (1, 2, 3, 4, 6, 7) passes for every sync function, every existing topic-organized test in `conformance/` is audited to confirm it does not encode a wazero bug, and every async entry point in `abi/` traps with an explicit-spec-citation error.

**Hard isolation rules — Loop 1:**

- **Rule L1-A.** No file under `internal/component/` outside `internal/component/abi/`, `internal/component/conformance/`, `internal/component/types/`, and `internal/component/val.go` (or whatever the `Val` types file is) may be read, edited, or even loaded into the agent's context. The parent agent enforces this by failing fast if a subagent's tool calls touch any other path.
- **Rule L1-B.** No test under `internal/component/wasip2test/`, `imports/wasip2/`, or any other directory that exercises the runtime call paths is run in Loop 1. If a Loop 1 agent finds itself wanting to know "does this fix make wasip2test pass?" — that's the signal that it's doing Loop 2 work in a Loop 1 session, and it must stop immediately.
- **Rule L1-C.** The only correctness oracles Loop 1 may consult are: `definitions.py` (Gate 1, Gate 3), `run_tests.py` (Gate 2), the `CanonicalABI.md` spec text (every gate), the installed `wasmtime` CLI on standalone `.wasm` files that don't go through wazero's runtime paths (Gate 4), and direct construction-then-call-into-`abi/` Go code (Gate 6 — runs through the public `wazero.NewRuntime()` API but exercises `abi/` directly via a constructed minimal component, not via a wasip2test fixture).
- **Rule L1-D.** If a Loop 1 agent encounters a discrepancy between `abi/` and `definitions.py`, the resolution is **always** "fix `abi/` to match `definitions.py`" unless the agent can produce a spec citation showing `definitions.py` itself diverges from `CanonicalABI.md`. In the second case, the agent halts Loop 1 and surfaces a `BLOCKER:` entry naming the spec/reference disagreement; a human resolves it. No agent makes a unilateral "definitions.py is wrong here" call.
- **Rule L1-E.** Every Loop 1 commit lands a verifiable, atomic improvement: either a function reconciled, a spec test ported, an existing test audited, or an async stub installed. Mixed commits are prohibited.

**Loop 1 termination condition.** See §8.

### 4.2 Loop 2 — runtime migration (migration-only)

**Goal.** Every call site in `canon_lower.go`, `component_linker.go`, and `instance.go` that performs canonical-ABI work directly is replaced with a call into `abi/`. Every dead helper left behind by a migration is deleted. Every existing test that is currently failing because of a known runtime bug goes green as a side effect of migration — never as a side effect of an `abi/` change.

**Hard isolation rules — Loop 2:**

- **Rule L2-A.** No file under `internal/component/abi/` may be edited in a Loop 2 session. Read-only access is allowed (the agent needs to know the abi/ API to call it). Any subagent that tries to write to `abi/` halts immediately and surfaces a `BLOCKER:` naming the call site that demanded the change. A human reopens Loop 1 to address it under the full Gate 1–7 protocol, then Loop 2 resumes.
- **Rule L2-B.** Loop 2 commits replace one runtime call site at a time. After each replacement, the entire `go test ./...` suite is run. The failing-test count is read from `status/loop2-baseline.json`. The new failing-test count is compared:
  - **Strictly less** → expected; the migration fixed a runtime bug. Commit.
  - **Equal** → expected; the call site was already producing correct behavior despite the broken implementation, or the test doesn't yet cover it. Commit.
  - **Strictly greater** → regression. The replacement is wrong. Revert. Investigate via `templates/diagnose-loop2-regression.md`. Do not commit. Do not edit `abi/`.
  - **New failure that was passing before, even if total count is unchanged** → regression. Same handling.
- **Rule L2-C.** Dead-helper deletion is its own commit, separate from the migration commit that orphaned the helper. The deletion commit verifies via grep that the symbol has zero references in the entire tree before deleting.
- **Rule L2-D.** Loop 2 may add **wiring tests** — tests that exercise a migrated call site through the public wazero API using a real component fixture (vendored .wasm or freshly built fixture under `internal/component/testdata/`). Wiring tests assert observable wiring outcomes (return values, traps, memory bytes, resource handles) and live alongside the migrated file or in `internal/component/wasip2test/`. Loop 2 may also audit-and-fix existing wiring-layer tests under `wasip2test/`, `canon_lower_test.go`, `component_linker_test.go`, `instance_test.go` that encode a wazero bug as expected behavior.
  - Loop 2 may **not** add or modify any file under `internal/component/conformance/` (Loop 1's domain, sealed at Loop 2 start).
  - Loop 2 may **not** add tests that assert canonical-ABI behavior directly (those belong in conformance/).
  - Wiring tests may **not** import `internal/component/abi` directly. They use only the public wazero API.

**Loop 2 termination condition.** See §8.

### 4.3 Loop 3 — wasip2 cleanup

Runs after Loop 2. Same isolation principle: Loop 3 may not edit any file under `internal/component/`. Loop 3 converts every site in `imports/wasip2/sockets/{tcp,udp}.go` and `imports/wasip2/http/http.go` from silent-default to `(nil, error)` trap, with a wiring test per site. Has its own status file, baseline, and termination criteria (see §8).

## 5. Gates in detail

### 5.0 Universal rule: code changes inside any gate trigger the review chain

Within any gate, any subagent dispatch that produces a file edit is followed immediately by the mandatory R1 → R3 → R5 review chain defined in §6. The gate cannot be marked pass until every code change produced inside it has run the full chain to `APPROVED` from both R1 and R3 reviewers, and the parent has committed the change.

The chain runs *inside* the gate, before the gate's pass criterion is evaluated. The gate's stated pass criterion is necessary but not sufficient — the gate also requires every code change in it to have completed its review chain.

No code change is exempt. One-line fixes, doc comment additions, deletions, and test ports all run the chain. No batching across gates: a code change produced in Gate 2 cannot be reviewed alongside a code change produced in Gate 6.

Audit artifacts (reconciliation reports, audit reports, status file updates, BLOCKER entries) are not code changes and do not trigger the chain. They are verified by the next gate's correctness check.

### 5.1 Gate 1 — Line-by-line reconciliation against `definitions.py`

**Precondition.** The function in `internal/component/abi/<file>.go` exists. The corresponding function in `definitions.py` is identified by name.

**Procedure.** Subagent template `templates/reconcile-function.md` is dispatched with placeholders `{go_file}`, `{go_func}`, `{py_file}`, `{py_func}`, `{py_line_range}`, `{spec_section}`. The subagent:

1. Reads the Go function in full.
2. Reads the Python function in full, plus every helper Python function the target calls (transitively, until reaching primitives or already-reconciled functions).
3. Reads the corresponding `CanonicalABI.md` section in full.
4. Produces a side-by-side reconciliation table: every Python statement (or contiguous group forming one logical step) maps to either (a) the Go statement that implements it identically, (b) the Go statement that implements it differently, or (c) "missing in Go".
5. Classifies every (b)/(c) row as: `bug-in-go`, `bug-in-python` (must cite spec line contradicting Python), or `intentional-deviation` (must cite spec line justifying it).
6. Does not write code in this gate. Produces report only.

**Pass criterion.** Every row classified. Zero unresolved `bug-in-go` rows. Zero `bug-in-python` without spec quote. Zero `intentional-deviation` without spec quote.

**Failure mode.** Unresolved `bug-in-go` → parent dispatches `templates/fix-reconciliation-finding.md`. **The fix template produces a code change in `abi/`, which triggers the full R1 → R3 → R5 review chain per §6 before the gate proceeds.** After the chain completes and the change is committed, Gate 1 re-runs from scratch with a fresh reconciliation subagent. `bug-in-python` without spec quote → halt loop, file `BLOCKER:`.

**Artifact.** `status/reconciliation/<go_func>.md` (uncommitted), ending with `RECONCILED <date> — function matches definitions.py and CanonicalABI.md.`

### 5.2 Gate 2 — Spec test passing

**Precondition.** Gate 1 passes. The function is the implementation target of one or more tests in the Go port of `run_tests.py`.

**Procedure.**

1. If the Go port file does not yet exist for the relevant Python test function, dispatch `templates/port-spec-test.md`. The subagent ports the Python test function to a Go test using table-driven `t.Run(name, ...)` subtests, mirroring the Python data tables row-for-row, with a header comment citing the upstream Python line range. **The ported test file is a code change and triggers the full R1 → R3 → R5 review chain per §6 before Gate 2 can proceed.**
2. Run `go test -v -run TestSpec<Name> ./internal/component/conformance/`.
3. If any subtest fails, the failure name and inputs are recorded.

**Pass criterion.** Every subtest in the relevant `TestSpec*` function passes. Only allowed skips are async-stub tests with explicit-spec-primitive skip messages.

**Failure mode.** Subtest failure → parent dispatches `templates/fix-reconciliation-finding.md` against the abi/ function with the failing input as a regression case. **The fix is a code change and triggers the full review chain.** Loop on Gate 1 + Gate 2 (each with its own review chains for any code each produces) until both pass.

**Artifact.** The Go test file, committed to `internal/component/conformance/spec_*_test.go`.

### 5.3 Gate 3 — Differential testing against `definitions.py` via Python subprocess

**Precondition.** Gate 2 passes. The function is a candidate for differential testing (functions whose inputs are exhaustively tabulated by Gate 2 are exempt; functions taking memory state or dynamic inputs are not).

**Procedure.**

1. The Python differential driver `docs/superpowers/projects/2026-04-07-canonical-abi-unification/harness/spec_diff_driver.py` exposes a JSON-IO protocol over stdin/stdout, importing `definitions.py` directly and forwarding calls. **The driver, when first created or any time it is modified, is a code change and triggers the full R1 → R3 → R5 review chain.** R1 verifies the JSON-IO protocol forwards inputs without altering them and serializes outputs without lossy conversion. R3 verifies Python best practices.
2. A Go differential test (`internal/component/conformance/spec_diff_test.go`, committed) reads inputs from the same data tables as Gate 2, sends each to the Python subprocess, sends the same to the Go abi/ function, and asserts byte-for-byte equality (memory writes hashed and compared; component values structurally compared with string-encoding tolerance only when input encoding differs). **The Go differential test, when first created or modified, is a code change and triggers the review chain.**
3. Run `go test -v -run TestSpecDiff ./internal/component/conformance/`.

**Pass criterion.** Every Gate 2 input also produces matching outputs from `abi/` and `definitions.py`. Zero divergences.

**Failure mode.** Divergence → record both outputs, classify per Gate 1 rules, reopen Gate 1. **Reopening Gate 1 means a fresh reconcile-function subagent runs, and any `fix-reconciliation-finding` it triggers runs through the full review chain.**

**Artifact.** `internal/component/conformance/spec_diff_test.go` (committed). `harness/spec_diff_driver.py` (uncommitted).

**Operational note.** The Python driver runs once per test session as a long-lived subprocess. Test setup launches it, communicates over stdin/stdout, terminates in TestMain teardown.

### 5.4 Gate 4 — Differential testing against installed `wasmtime` CLI

**Precondition.** Gate 3 passes. The function is exercised by at least one standalone `.wasm` file in the vendored corpus that wasmtime can run directly without going through any wazero runtime path.

**Procedure.**

1. A Go test (`internal/component/conformance/spec_wasmtime_diff_test.go`, committed) enumerates `.wasm` and `.wat` files under `debug-vendored/component-model/test/`, `debug-vendored/wit-bindgen/tests/runtime/*/`, and `debug-vendored/wasmtime/tests/disas/component-model/*.wat`. **The test file, when first created or modified, triggers the review chain.**
2. For each fixture:
   - Detect the installed `wasmtime` binary via `exec.LookPath("wasmtime")`. If absent, the entire Gate 4 test calls `t.Skip("wasmtime CLI not installed; install via curl https://wasmtime.dev/install.sh -sSf | bash")`. The skip message is the install command.
   - Run the fixture under wasmtime, capture stdout/stderr/exit code/diagnostics.
   - Run the same fixture under wazero via the public API, capture the same observable surface.
   - Assert byte-for-byte equality. Permitted normalization for system-injected nondeterminism only when documented in the fixture metadata.
3. Each fixture runs as its own subtest named after its path.

**Pass criterion.** Every fixture: equivalent observable behavior. Zero divergences. Allowed skips: (a) wasmtime not installed, (b) async/stream/future/thread/error-context fixtures (skip message names the missing primitive and spec section), (c) Loop 3 wasip2 territory (skip message names the wasip2 op).

**Failure mode.** Divergence → parent dispatches `templates/diagnose-wasmtime-divergence.md`, which reads the spec, decides which side is wrong, and either reopens Gate 1 or files a `BLOCKER:` for wasmtime version mismatch. **`diagnose-wasmtime-divergence.md` produces only an audit artifact, not a code change, so it does not directly trigger the review chain. But if its conclusion reopens Gate 1, the resulting `fix-reconciliation-finding` is a code change that runs the full chain.**

**Artifact.** `internal/component/conformance/spec_wasmtime_diff_test.go` (committed). Per-fixture divergence records (uncommitted).

### 5.5 Gate 6 — Public-API exercise via `wazero.NewRuntime()` end-to-end

**Precondition.** Gate 3 passes. The function is reachable through the wazero public API surface.

**Procedure.**

1. A Go test (`internal/component/conformance/spec_publicapi_test.go`, committed) enumerates every public abi/ function via a function-name → fixture-path map maintained by the parent agent in `status/loop1-publicapi-coverage.json`. **The test file, when first created or modified, triggers the review chain.**
2. For each function:
   - Construct a `wazero.Runtime` via the public API.
   - Load a real .wasm fixture: existing `internal/component/testdata/*.wasm`, vendored fixture from `debug-vendored/wit-bindgen/tests/runtime/`, or newly built fixture committed to `internal/component/testdata/` with source under `internal/component/testdata/gen/`. **No synthetic in-memory wasm bytes constructed by the test itself are allowed.**
   - Instantiate the component via the public component API.
   - Invoke an exported function via the public `ComponentFunc.Call(...)` API.
   - Assert the return value matches the expected canonical-ABI behavior.
3. Test uses **only** the wazero public API. May not import `internal/component/abi` directly. May not import any internal package.

**Pass criterion.** Every public abi/ function has at least one passing public-API exercise test using a real .wasm fixture and the public wazero API. No exceptions.

**Failure mode.** Function with no passing public-API test → not eligible for Gate 6 pass. Parent dispatches `templates/build-public-api-exercise.md` to construct the missing fixture and test. **The output is a multi-file code change** (Go test, fixture source, built `.wasm`, possibly `Cargo.toml`). **All produced files together count as one code change for the review chain and go through one R1 → R3 → R5 chain as a unit.** R1 verifies the fixture exercises the spec primitive correctly; R3 verifies build hygiene and reproducibility.

**Artifact.** `internal/component/conformance/spec_publicapi_test.go` (committed). New fixtures under `internal/component/testdata/` and generators under `internal/component/testdata/gen/` (committed).

### 5.6 Gate 7 — Spec-text traceability (lightweight)

**Precondition.** None. Runs after Gates 1–4 and 6 all pass for a function.

**Procedure.**

1. The Go function gets a doc comment of the exact form:
   ```go
   // <FuncName> implements canon <spec_op> from CanonicalABI.md §<section>.
   //
   // Reference implementation: definitions.py:<func_name> (lines <start>-<end>).
   // Reconciled <YYYY-MM-DD> — see status/reconciliation/<func>.md.
   ```
2. Inside the function body, every nontrivial branch (any `if`/`switch` case whose condition is not a simple type-switch) gets a comment of the form `// Spec: §<section>` or `// Spec: definitions.py:<line>`. Trivial branches (type switches over `ValType` interface variants) do not require per-case comments — the type switch itself is the citation.
3. **The doc comment edit produced by `templates/add-spec-citation.md` is a code change and triggers the full R1 → R3 → R5 review chain.** R1 verifies the cited spec section actually exists in `CanonicalABI.md` and contains relevant text; R3 verifies Go convention and accuracy of in-body branch comments.

**Pass criterion.** Doc comment in required form. Reconciliation report supports it.

**Failure mode.** Missing/malformed → dispatch `templates/add-spec-citation.md`.

**Artifact.** Doc comment in `internal/component/abi/<file>.go`.

### 5.7 Gates and the iteration loop

A single Loop 1 iteration covers exactly one abi/ function and runs Gates 1 → 2 → 3 → 4 → 6 → 7 in order. A gate can only run if all prior gates passed for the same function. Failure of any gate reopens the earliest applicable gate. The iteration commits when all gates pass and all review chains for code produced inside it have completed.

There is no Gate 5 (property/fuzz, dropped). There is no Gate 8 (coverage threshold, dropped).

**Iteration accounting with the review chain.** A single Loop 1 iteration may produce multiple code changes across its six gates. Each code change runs its own complete R1 → R3 → R5 review chain inside the gate that produced it. Two code changes from two different gates never share a review chain. The iteration's commit log records every chain that ran, in order, with the writing template, the R1 reviewer's approval, and the R3 reviewer's approval listed per chain.

A common iteration shape:

1. Gate 1 reconciles → audit artifact only → no review chain → finds two `bug-in-go` rows.
2. Fix #1 → code change → review chain runs → committed.
3. Gate 1 re-reconciles → finds one row remaining.
4. Fix #2 → code change → review chain runs → committed.
5. Gate 1 re-reconciles → clean.
6. Gate 2 ports test → code change → review chain runs → committed.
7. Gate 2 runs test → passes.
8. Gate 3 differential → no code change → passes.
9. Gate 4 wasmtime → no code change → passes.
10. Gate 6 builds public-API exercise → code change → review chain runs → committed.
11. Gate 7 adds doc comment → code change → review chain runs → committed.
12. Iteration done. Status file updated. Loop continues.

Five review chains in one iteration. None batched, none skipped, none combined. The iteration is long. The user has explicitly accepted that as the cost of correctness.

## 6. Subagent dispatch model and mandatory review chain

### 6.1 Parent / subagent split

**Parent agent.** A single Claude session running one of the loop prompts. The parent's job: read the relevant status file, find the next work item, dispatch the appropriate template-filled subagent via the Agent tool, wait for the subagent to return, integrate the result (update status file, commit changes if the subagent wrote code, write artifacts to disk), independently verify the integration by re-running the gate the subagent claimed to satisfy, move to the next work item.

The parent never writes code itself. The parent never reads `definitions.py` itself. The parent never reads the spec text itself. The parent's only reading is the status files, the loop prompt, the templates, and the subagent's return message.

**Subagent.** A fresh Claude session spawned via the Agent tool with one filled template as its prompt. Receives no conversation history. Reads only the files the template authorizes it to read. Performs exactly one task. Produces a structured return. Halts immediately on any `BLOCKER:` condition.

Subagents are dispatched serially within an iteration. The parent does not parallelize subagents within an iteration.

### 6.2 Templates

Every template is a markdown file under `templates/` with the following sections, in order:

1. **`# <Template Name>`** — title.
2. **`## Spec-overrides-instructions warning`** — the literal text of the rule from §7, verbatim.
3. **`## First action`** — read the relevant `CanonicalABI.md` section and the relevant `definitions.py` function (or wasmtime reference) and acknowledge in the return message that this has been done with verbatim quotes the parent can grep-verify.
4. **`## Inputs`** — placeholders the parent fills in.
5. **`## Allowed reads`** — exact list of files (or glob patterns) the subagent may Read or Grep. Reading anything else is a protocol violation.
6. **`## Allowed writes`** — exact list of files the subagent may Edit or Write. Writing anything else is a protocol violation.
7. **`## Procedure`** — numbered steps the subagent must follow.
8. **`## Halt conditions`** — explicit conditions that force the subagent to return a `BLOCKER:` instead of continuing.
9. **`## Return format`** — exact structure the parent expects.
10. **`## Self-check`** — questions the subagent must answer truthfully in its return message before declaring success.

Code-producing templates additionally end with a section `## After this template runs` that names the next required step (R1 spec compliance review) and forbids the parent from skipping it.

### 6.3 Template inventory

**Loop 1 — abi/ correctness:**

- `templates/reconcile-function.md` — Gate 1: produces reconciliation report.
- `templates/fix-reconciliation-finding.md` — fixes one specific Gate 1 finding by editing abi/.
- `templates/port-spec-test.md` — Gate 2: ports one Python test function to Go.
- `templates/run-python-differential.md` — Gate 3: drives the Python subprocess and reports divergences.
- `templates/run-wasmtime-conformance.md` — Gate 4: runs one wasmtime fixture, reports divergence.
- `templates/diagnose-wasmtime-divergence.md` — investigates a Gate 4 divergence and decides which side is wrong.
- `templates/build-public-api-exercise.md` — Gate 6: builds a new fixture and Go test exercising one abi/ function via public API.
- `templates/add-spec-citation.md` — Gate 7: writes the doc comment.
- `templates/audit-existing-conformance-test.md` — audits one existing `internal/component/conformance/*_test.go` file.
- `templates/install-async-stub.md` — replaces an async entry point with the explicit-error stub.
- `templates/enumerate-functions.md` — runs at Loop 1 start: enumerates the sync function list from `definitions.py`.
- `templates/enumerate-wasmtime-fixtures.md` — runs at Loop 1 start: enumerates the wasmtime fixture corpus.

**Loop 2 — runtime migration:**

- `templates/enumerate-callsites.md` — runs at Loop 2 start: produces the full list of runtime call sites that perform canonical-ABI work.
- `templates/capture-loop2-baseline.md` — runs at Loop 2 start: captures `go test ./...` failure baseline.
- `templates/migrate-callsite.md` — replaces one runtime call site with an abi/ call.
- `templates/add-wiring-test.md` — adds a wiring test for a migrated call site.
- `templates/audit-existing-wiring-test.md` — audits one existing wiring-layer test file.
- `templates/diagnose-loop2-regression.md` — investigates a regression after a migration commit.
- `templates/delete-dead-helper.md` — verifies a parallel-impl symbol has zero references and deletes it.

**Loop 3 — wasip2 cleanup:**

- `templates/enumerate-wasip2-suppression-sites.md`
- `templates/fix-error-suppression.md`
- `templates/add-wasip2-trap-test.md`

**Review chain (used by all loops):**

- `templates/review-spec-compliance.md` — R1.
- `templates/review-code-quality.md` — R3.
- `templates/revise-after-review.md` — R2 and R4.

**Shared support:**

- `templates/file-blocker.md` — writes a `BLOCKER:` entry to `status/blockers.json`.
- `templates/verify-grep-zero.md` — verifies a symbol has zero references in the tree before deletion.

### 6.4 Mandatory review chain after every code change

A code change is any edit to a `.go`, `.py`, `.rs`, `.wat`, `.wasm`, `.toml` file or any other actual source file produced by the iteration. Doc comments and spec citations are code changes. Deletions of files or symbols are code changes. Adds of new test files are code changes.

Updates to status files, reconciliation reports, audit reports, and BLOCKER entries are **not** code changes — they are audit artifacts and they are verified by the next gate that runs.

Every code-producing subagent dispatch is followed by:

1. **Step R1 — Spec compliance review.** Parent dispatches a fresh subagent using `templates/review-spec-compliance.md`. Inputs: the diff produced by the writing subagent, the abi/ function the change affects (if applicable), the relevant `CanonicalABI.md` section, the relevant `definitions.py` reference. The reviewer reads the diff line-by-line against the spec and against `definitions.py`, produces a structured findings list, and returns either `APPROVED` (with verbatim spec quotes) or `FINDINGS` (with each finding linked to a spec quote).

2. **Step R2 — Revision after R1.** If R1 returned `FINDINGS`, the parent dispatches a **fresh** revision subagent using `templates/revise-after-review.md` with the findings as input. The revision subagent is **not** the original writer, **not** the R1 reviewer, and has no prior context other than the findings and the file paths. It produces a new diff. Then the chain restarts at R1 with yet another fresh subagent. Loop until R1 returns `APPROVED`.

3. **Step R3 — Code quality review.** Once R1 is `APPROVED`, the parent dispatches a fresh subagent using `templates/review-code-quality.md`. Inputs: the same diff, the wazero codebase conventions. The reviewer checks naming, error handling, idiomatic Go usage, comment clarity, test naming, imports, and adherence to the patterns used in the rest of `internal/component/`. Returns `APPROVED` or `FINDINGS`. The reviewer is forbidden from making spec-correctness judgments — those are R1's job.

4. **Step R4 — Revision after R3.** If R3 returned `FINDINGS`, the parent dispatches a fresh revision subagent (not the original writer, not the R1 reviewer, not the R3 reviewer, not any prior revision subagent in this iteration) with the findings. After revision, **the chain restarts at R1**, not at R3 — because a quality revision can affect spec compliance. Loop until both R1 and R3 return `APPROVED` consecutively without intervening revisions.

5. **Step R5 — Parent integration.** Only after both R1 and R3 have returned `APPROVED` for the same diff state does the parent commit the change. The commit message names the writing subagent template, the R1 reviewer's approval, the R3 reviewer's approval, and the iteration count if revisions occurred.

### 6.5 Hard constraints on the review chain

- **No self-review.** The R1 reviewer is never the same agent that wrote the code. The R3 reviewer is never the writer and never the R1 reviewer for the same diff. Revision subagents are never reviewers for the same diff.
- **No batching.** Each code change gets its own R1 → (revisions) → R3 → (revisions) chain. Two code changes from two different gates do not share a review pass.
- **No grouping.** R1 and R3 are not combined into a single review.
- **No skipping.** Every code change runs the full chain. There is no "trivial change exception."
- **No deferring.** R1 runs immediately after the writer returns. R3 runs immediately after R1 returns `APPROVED`. The parent does not move to the next gate, does not commit, does not update the status file with success state, until the chain completes for the current change.
- **Restart-on-revision is total.** Any revision in either the R1 or R3 phase restarts the chain at R1, never at R3.

### 6.6 Subagent identity tracking

The parent maintains an in-memory list of subagent IDs used in the current iteration, tagged by role (writer, R1-reviewer, R2-reviser, R3-reviewer, R4-reviser, …). The list is logged in the iteration's commit message footer.

### 6.7 Result integration

When a code-producing subagent returns:

1. Parent reads the structured return.
2. Parent independently verifies the artifacts on disk support the claim. The parent does not trust the subagent's "I did it" — it trusts only the artifacts on disk.
3. Parent dispatches R1.
4. Chain runs through R5.
5. Only after R5 does the parent update the status file and move to the next gate.

When a non-code-producing subagent returns (Gate 1 reconciliation report, Gate 3 differential run, Gate 4 wasmtime run):

1. Parent reads the structured return.
2. Parent independently verifies the artifacts on disk.
3. Next gate runs immediately. No review chain.
4. If the next gate fails, the prior gate's audit was wrong; the prior gate is reopened with a fresh subagent.

### 6.8 The "no reasoning shortcut" rule

A subagent's return message must include a verbatim quote of the relevant `CanonicalABI.md` paragraph (or the relevant `definitions.py` function) that justifies its conclusion. A subagent that returns a conclusion without supporting quotes has its return rejected and the iteration restarts with a new subagent. The parent does not parse the quotes for correctness — the parent only checks (a) that quotes are present in the expected sections and (b) that the quoted text appears verbatim in the file the subagent claims to have read (parent does its own grep on the source file to confirm the quote string exists).

## 7. The spec-overrides-local-instructions rule

### 7.1 The rule, stated

> **Spec-Overrides-Local-Instructions Rule.**
>
> Before performing any action — reading, writing, dispatching, reviewing, deciding — every agent and subagent must read the relevant section of `debug-vendored/component-model/design/mvp/CanonicalABI.md` and the relevant function in `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py` (or the relevant portion of the wasmtime reference for cases the spec text alone is ambiguous on).
>
> If the prompt, template, plan, status file, prior commit, in-tree comment, or any other local instruction conflicts with what the spec or reference implementation says, **the spec/reference wins**. The agent must fix the local instruction (file a `BLOCKER:` if the fix is structural) before continuing the task.
>
> The agent does not assume the local instruction is right because it is local. The agent does not assume `definitions.py` is wrong because the local code is established. The agent does not "split the difference" or "preserve existing behavior for compatibility" — there is no compatibility constraint stronger than spec correctness.
>
> An agent that cannot find a spec citation for what it is about to do halts and files a `BLOCKER:` rather than guessing.

### 7.2 Where the rule is enforced

**In every loop prompt** (`prompts/loop1-abi-correctness.md`, `prompts/loop2-runtime-migration.md`, `prompts/loop3-wasip2-cleanup.md`):

The first section of every loop prompt is the literal text of the rule, verbatim. The second section is a step-by-step protocol the parent agent must follow at the start of every iteration:

1. Read `debug-vendored/component-model/design/mvp/CanonicalABI.md` table of contents.
2. Read the section relevant to the work item.
3. Read the corresponding `definitions.py` function in full.
4. State, in the parent's session context, the spec quote and the `definitions.py` reference that govern the work item.
5. Only then dispatch a subagent.

The parent does not skip steps 1–4 even if it has done them in a previous iteration.

**In every template under `templates/`:**

Section `## Spec-overrides-instructions warning` of every template contains the literal text of the rule, verbatim, as the second section after the title. There is no abbreviated version. There is no "see prompt for details" reference — the full text is repeated in every file.

Section `## First action` of every template requires the subagent to perform the spec/reference read **before** doing anything else, including reading the inputs.

The first-action read is mandatory for every subagent regardless of role: writing, reviewing, revising, auditing, enumerating. The R1 spec compliance reviewer reads the spec before reading the diff. The R3 code quality reviewer reads the spec before reading the diff (because quality cannot be judged without knowing what the code is supposed to do). The reviser reads the spec before applying findings. No exceptions.

**In every iteration script** (`scripts/loop1-iteration.md`, `scripts/loop2-iteration.md`, `scripts/loop3-iteration.md`):

Step 0 of every iteration script is "Re-read the spec section and `definitions.py` reference for the current work item, even if you read them in a prior iteration." Step 0 is not skippable.

**In every commit message produced by the parent agent:**

The commit message footer contains a `Spec:` line citing the `CanonicalABI.md` section and a `Reference:` line citing the `definitions.py` line range. A commit without these lines is not a valid project commit and is rejected by the iteration script's pre-commit check.

**In every doc comment added by Gate 7:**

The doc comment is the spec citation made permanent in the code. Future contributors editing the function read the citation in the doc comment and re-derive the spec context.

### 7.3 Failure modes the rule blocks

1. An agent trusting an in-tree comment that encodes a wazero bug.
2. An agent reading a previously-committed function in `abi/` and assuming it is correct without re-verifying.
3. An agent rationalizing a divergence as "wazero's idiomatic Go style."
4. An agent skipping the spec read because the work item "looks obvious."
5. An agent assuming `definitions.py` agrees with `CanonicalABI.md` without checking.
6. An agent assuming a prior agent's reconciliation report is correct without re-deriving it.

### 7.4 Failure to follow the rule

If the parent agent observes that a subagent's return message does not include the required spec/Python quotes (or quotes that fail the parent's verbatim grep check), the subagent's return is **rejected**. The iteration restarts with a fresh subagent. The rejected subagent's identity is logged in `status/iteration-log.json` for forensics. Repeated rejections of the same template at the same work item (three or more in one iteration) trigger a `BLOCKER:`.

### 7.5 Verbatim text embedded in every prompt and template

This is the canonical version. Any deviation in any file is a bug.

> **SPEC-OVERRIDES-LOCAL-INSTRUCTIONS RULE.**
>
> Before doing anything in this task, read the relevant section of `debug-vendored/component-model/design/mvp/CanonicalABI.md` and the relevant function in `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py`. If anything in this prompt, in the existing wazero code, in any in-tree comment, in any prior commit, or in any status file conflicts with the spec or with definitions.py, **the spec and definitions.py win**. Fix the local instruction (or file a BLOCKER if you cannot) before continuing.
>
> Do not assume any prior wazero code is correct. Do not assume any prior agent's reconciliation report is correct. Do not assume any in-tree comment is correct. Do not "preserve existing behavior for compatibility" — there is no compatibility constraint stronger than spec correctness.
>
> If you cannot find a spec citation for what you are about to do, halt and file a BLOCKER. Do not guess. Do not interpolate. Do not "use your judgment" about canonical ABI semantics — the spec is the only judgment that counts.
>
> Your return message must include verbatim quotes from the spec section and the definitions.py function you read. The parent agent will grep the cited files to verify the quotes exist. A return without quotes, or with quotes that do not appear verbatim in the cited files, will be rejected and the work will restart with a fresh subagent.

## 8. Status & resume

### 8.1 Principles

- All long-lived state lives in JSON files under `docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/`.
- Status files are written atomically (temp + rename).
- Status files are human-readable JSON; the user may inspect with `jq` and override by hand between sessions.
- Status files are single-writer: only the parent agent for a given loop writes its status files.
- Status files are versioned with `schema_version`.
- Reads-before-writes are validated: the parent re-reads before updating and halts on unexpected state changes.

### 8.2 Status file inventory

- `status/loop1-functions.json` — per-function gate progress for Loop 1, including `review_chains` per code change.
- `status/loop1-existing-conformance-audit.json` — audit progress for existing tests.
- `status/loop1-async-stubs.json` — installation progress for explicit-error stubs.
- `status/loop1-publicapi-coverage.json` — function-name → fixture-path map for Gate 6.
- `status/loop2-callsites.json` — per-call-site migration progress.
- `status/loop2-baseline.json` — `go test ./...` failure baseline at Loop 2 start.
- `status/loop2-existing-wiring-audit.json` — audit progress for existing wiring-layer tests.
- `status/loop2-wiring-tests.json` — per-migration wiring test mapping.
- `status/loop3-suppressed-errors.json` — per-site progress for wasip2 cleanup.
- `status/loop3-baseline.json` — baseline at Loop 3 start.
- `status/blockers.json` — every `BLOCKER:` filed by any subagent in any loop. The human's mailbox.
- `status/iteration-log.json` — append-only forensic log.
- `status/wasmtime-fixture-corpus.json` — captured at Loop 1 start, used by Gate 4 and project termination.
- `status/project-state.json` — top-level loop completion flags.
- `status/reconciliation/<func>.md` — per-function reconciliation reports (markdown, not JSON).
- `status/audit/<file>.md` — per-file audit reports (markdown).

The full schemas are specified in the implementation plan (writing-plans output). The brainstorming-level commitment is: every status file has `schema_version`, the parent verifies it on read, mismatch halts with `BLOCKER:`.

### 8.3 Resume protocol

When a fresh session starts by feeding `prompts/loop1-abi-correctness.md` to a new Claude:

1. Parent reads every status file and runs a self-consistency check. Confirms parseability, schema versions, no unresolved blockers in current loop's scope.
2. Parent runs `git status` to confirm working tree is clean. Uncommitted changes from a previous session halt with a report.
3. Parent runs `git log --oneline -20` and cross-checks against `iteration-log.json`. Disagreement → `BLOCKER:`.
4. Parent finds the next pending work item by scanning the relevant status file.
5. Parent dispatches the appropriate template for the next gate.

When a session ends mid-iteration:

- Mid-review-chain → next session sees the most recent committed state and the in-progress entry; re-runs the gate from the beginning. Partial review chains are inherently untrusted because the writer's diff has not been fully reviewed.
- Between gates → next session resumes at the next pending gate.
- Between iterations → next session picks the next pending work item.

### 8.4 Manual override

The user may at any time edit any status file by hand. The parent agent reads whatever state it finds, as long as it parses and matches the schema version.

### 8.5 What is not in status files

- Conversation transcripts. Sessions communicate only via committed code, audit artifacts on disk, and JSON status files.
- Subagent return messages. Once integrated, subagent returns are discarded.
- Anything that can be re-derived from the code or `git log`.

## 9. Termination criteria

### 9.1 Loop 1 termination

Loop 1 is complete when **every one of the following assertions is true**, mechanically verified:

- **L1-T1.** Every existing `internal/component/conformance/*_test.go` file has `audit_status == "pass"` in `loop1-existing-conformance-audit.json` and carries the `// Audited <date> against definitions.py — assertions match canonical reference.` header. Verified by `grep -L`.
- **L1-T2.** Every function in `loop1-functions.json` has `iteration_status == "complete"` and every gate (gate1, gate2, gate3, gate4, gate6, gate7 — there is no gate5; see §5.7) in `pass`. Verified by reading the file. Cross-checked against `python3 harness/spec_diff_driver.py --list-sync-functions`.
- **L1-T3.** Every async entry point in `loop1-async-stubs.json` has `stub_status == "installed"`. Verified by `grep -rn "requires.*not implemented in wazero" internal/component/abi/` matching the count, plus `grep -rn "panic.*async\|panic.*not.implemented\|panic.*TODO" internal/component/abi/` returning empty.
- **L1-T4.** Every function in `loop1-functions.json` is mapped to a non-null fixture and test name in `loop1-publicapi-coverage.json`. `go test -v -run TestPublicAPI_ ./internal/component/conformance/` passes.
- **L1-T5.** Every Loop 1 commit has `Spec:` and `Reference:` footer lines. Verified by `git log` parsing.
- **L1-T6.** `go test -v ./internal/component/abi/... ./internal/component/conformance/...` shows zero failures, zero unexplained skips. Allowed skips: only async-stub tests with explicit-spec-citation messages.
- **L1-T7.** `status/blockers.json` has zero open entries.
- **L1-T8.** Every committed file in `internal/component/abi/` has a corresponding reconciliation report under `status/reconciliation/` ending with `RECONCILED <date>`.
- **L1-T9.** Review chain accounting: for every Loop 1 commit with code changes, the recorded review chain in `loop1-functions.json` (or in the matching `loop1-existing-conformance-audit.json[file].review_chains` or `loop1-async-stubs.json[stub].review_chains`) contains at least one R1 entry ending in `APPROVED` and at least one R3 entry ending in `APPROVED`, with the R3 `APPROVED` occurring after the R1 `APPROVED` and with no revisions occurring after the final R3 `APPROVED`. Commits with no recorded review chain (or with a recorded chain whose final state is not `APPROVED` from both R1 and R3) are protocol violations and the iteration must be reopened.

When all nine pass: parent writes `loop1_complete: true` to `status/project-state.json`, stops accepting Loop 1 iterations. Loop 2 may start.

### 9.2 Loop 2 termination

- **L2-T1.** Every callsite in `loop2-callsites.json` has `migration_status == "complete"` and `wiring_test != null`. Wiring tests pass.
- **L2-T2.** Every helper symbol in the deletion-target list returns zero hits from a tree-wide grep.
- **L2-T3.** `internal/component/abi/` is unchanged since Loop 1 termination. Verified by `git diff <loop1-completion-sha> -- internal/component/abi/`.
- **L2-T4.** `go test ./...` failure count is strictly less than baseline; every removed failure is attributable to a specific Loop 2 commit.
- **L2-T5.** No test that was passing in baseline is now failing.
- **L2-T6.** Every wiring-layer test file in `loop2-existing-wiring-audit.json` has `audit_status == "pass"`.
- **L2-T7.** Every Loop 2 commit has `Spec:`, `Reference:`, and `Migration:` footer lines.
- **L2-T8.** Zero open Loop-2-filed blockers.
- **L2-T9.** Review chain accounting per L1-T9 form, applied to Loop 2 commits and Loop 2 status files.

### 9.3 Loop 3 termination

- **L3-T1.** Every site in `loop3-suppressed-errors.json` has `fix_status == "complete"` and `trap_test != null`. Trap tests pass.
- **L3-T2.** Tree-wide grep for the silent-default pattern in `imports/wasip2/` returns zero hits.
- **L3-T3.** No incidental edits outside touched suppression sites.
- **L3-T4.** `go test ./...` failure count strictly less than `loop3-baseline.json`; no passing-test regressions.
- **L3-T5.** Every Loop 3 commit has `Spec:`, `Reference:`, and `SuppressionSite:` footer lines.
- **L3-T6.** Review chain accounting per L1-T9 form, applied to Loop 3 commits and Loop 3 status files.
- **L3-T7.** Zero open Loop-3-filed blockers.

### 9.4 Project termination

- **P-T1.** `loop1_complete && loop2_complete && loop3_complete`.
- **P-T2.** Tree-wide grep for **every** canonical-ABI duplication symbol enumerated by `templates/enumerate-callsites.md` at Loop 2 start and persisted to `status/loop2-callsites.json[deletion_targets]` returns zero hits in production code outside `internal/component/abi/`. The enumeration is the authoritative source for the deletion target list. As a non-exhaustive sanity check, the symbols known at design time include: `liftFromStack`, `liftRecordFromStack`, `liftOptionFromStack`, `liftVariantFromStack`, `lowerToStack`, `writeResultsToMemory`, `writeRecordToMemory`, `writeValToMemory`, `flattenVariantType`, `elemSizeFromTypeRef`, `elemAlignFromTypeRef`, `recordSize`, `optionSize`, `variantSize`, `liftFieldFromMemory`, `liftResultFromMemory`, `liftRecord`, `lowerByKind`, `lowerTyped`, `lowerStringParam`, `elementSizeForKind`, `alignmentForKind`, `sizeOfVal`. If `enumerate-callsites.md` finds additional symbols, they are added to `loop2-callsites.json[deletion_targets]` and verified by P-T2 along with the design-time list. If `enumerate-callsites.md` finds **fewer** symbols than the design-time list, the parent agent halts with a `BLOCKER:` because that suggests the enumeration template is incomplete.
- **P-T3.** `canon_lower.go`, `component_linker.go`, `instance.go` are reduced to glue (deletion proxy: substantial LOC reduction reported in project-state).
- **P-T4.** `resource_table.go` exposes only trap-emitting handle ops; silent-ignore variants have zero references.
- **P-T5.** `go test ./...` runs with zero failures. **Zero**, not "fewer than baseline."
- **P-T6.** Every public function in `internal/component/abi/` has a Gate 7 doc comment with `Spec:`, `Reference:`, and `Reconciled:` lines.
- **P-T7.** Every fixture in `wasmtime-fixture-corpus.json`, when run through wazero via the public API, produces observable behavior bit-equivalent to the installed `wasmtime` CLI. Skipped fixtures must carry explicit-spec-primitive skip messages.
- **P-T8.** Zero open blockers across all loops.
- **P-T9.** Manual user confirmation. Parent surfaces project-state.json contents to the user and waits. Parent does not declare project completion unilaterally.

### 9.5 What "done" looks like

A wazero tree with:

- `internal/component/abi/` as the only canonical-ABI engine, every function reconciled against `definitions.py`, every function carrying spec citations in doc comments, every function exercised by a real-wasm public-API end-to-end test, the synchronous canonical ABI passing every relevant test in the Go port of `run_tests.py`, every async entry point trapping with an explicit missing-primitive message.
- Three runtime files (`canon_lower.go`, `component_linker.go`, `instance.go`) reduced to glue. Every parallel canonical-ABI helper deleted.
- `resource_table.go` with a single trap-emitting handle API.
- `imports/wasip2/sockets/{tcp,udp}.go` and `imports/wasip2/http/http.go` with every error path returning `(nil, error)` to trap. Zero silent-default sites.
- Every existing test in `conformance/` audited.
- Every wiring test exercising migrated call sites via real .wasm fixtures and the public wazero API.
- A `go test ./...` run that is fully green.
- Per-fixture wasmtime-conformance equivalence demonstrated for the captured corpus.
- A complete forensic record under `docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/` showing every iteration, every review chain, every commit, every blocker, every reconciliation report.
- Zero `TODO`, zero `panic("not implemented")`, zero silent error swallows, zero parallel implementations of any canonical-ABI operation.

## 10. Deliverable inventory

### 10.1 Committed deliverables

- This spec: `docs/superpowers/specs/2026-04-07-canonical-abi-unification-design.md`
- The plan (writing-plans output): `docs/superpowers/plans/2026-04-07-canonical-abi-unification.md`
- Go test files produced during Loop 1: `internal/component/conformance/spec_*_test.go`, `spec_diff_test.go`, `spec_wasmtime_diff_test.go`, `spec_publicapi_test.go`
- New `.wasm` test fixtures and their generators: `internal/component/testdata/*.wasm`, `internal/component/testdata/gen/*.go` (or `*.rs` + `Cargo.toml`)
- abi/ source code edits: `internal/component/abi/*.go`
- Runtime glue edits: `internal/component/canon_lower.go`, `component_linker.go`, `instance.go`, `resource_table.go`
- Wasip2 trap-conversion edits: `imports/wasip2/sockets/{tcp,udp}.go`, `imports/wasip2/http/http.go`
- Wiring tests added during Loop 2: `internal/component/{canon_lower,component_linker,instance}_test.go`, `internal/component/wasip2test/*`
- Trap tests added during Loop 3: under `imports/wasip2/`

### 10.2 Uncommitted deliverables (project dir)

Layout of `docs/superpowers/projects/2026-04-07-canonical-abi-unification/`:

```
docs/superpowers/projects/2026-04-07-canonical-abi-unification/
├── README.md                          # entry point: how to start a session, links back to spec and plan
├── prompts/
│   ├── loop1-abi-correctness.md
│   ├── loop2-runtime-migration.md
│   └── loop3-wasip2-cleanup.md
├── scripts/
│   ├── loop1-iteration.md
│   ├── loop2-iteration.md
│   └── loop3-iteration.md
├── templates/
│   ├── reconcile-function.md
│   ├── fix-reconciliation-finding.md
│   ├── port-spec-test.md
│   ├── run-python-differential.md
│   ├── run-wasmtime-conformance.md
│   ├── diagnose-wasmtime-divergence.md
│   ├── build-public-api-exercise.md
│   ├── add-spec-citation.md
│   ├── audit-existing-conformance-test.md
│   ├── install-async-stub.md
│   ├── enumerate-functions.md
│   ├── enumerate-wasmtime-fixtures.md
│   ├── enumerate-callsites.md
│   ├── capture-loop2-baseline.md
│   ├── migrate-callsite.md
│   ├── add-wiring-test.md
│   ├── audit-existing-wiring-test.md
│   ├── diagnose-loop2-regression.md
│   ├── delete-dead-helper.md
│   ├── enumerate-wasip2-suppression-sites.md
│   ├── fix-error-suppression.md
│   ├── add-wasip2-trap-test.md
│   ├── review-spec-compliance.md
│   ├── review-code-quality.md
│   ├── revise-after-review.md
│   ├── file-blocker.md
│   └── verify-grep-zero.md
├── status/
│   ├── project-state.json
│   ├── loop1-functions.json
│   ├── loop1-existing-conformance-audit.json
│   ├── loop1-async-stubs.json
│   ├── loop1-publicapi-coverage.json
│   ├── loop2-callsites.json
│   ├── loop2-baseline.json
│   ├── loop2-existing-wiring-audit.json
│   ├── loop2-wiring-tests.json
│   ├── loop3-suppressed-errors.json
│   ├── loop3-baseline.json
│   ├── blockers.json
│   ├── iteration-log.json
│   ├── wasmtime-fixture-corpus.json
│   ├── reconciliation/
│   │   └── <func>.md (one per reconciled function)
│   └── audit/
│       └── <file>.md (one per audited file)
└── harness/
    └── spec_diff_driver.py            # the only real source file in the project dir; JSON-IO wrapper around definitions.py
```

The whole `projects/` dir is untracked (never `git add`'d).

## 11. Out of scope

- **Asynchronous canonical ABI primitives.** Streams, futures, error-context, threads, waitable-set, callbacks, subtasks, async tasks. Stubbed with explicit-error returns. A future project may port them; that project will need its own brainstorming and design cycle to fit wazero's concurrency model.
- **Property-based or fuzz testing.** Considered (Gate 5 in the brainstorm); dropped.
- **100% line/branch coverage threshold.** Considered (Gate 8); dropped.
- **Building wasmtime from source.** The installed CLI is used. If unavailable, Gate 4 skips with the install command in the skip message.
- **Performance optimization of `abi/`.** The reconciled engine is allowed to be slower than the parallel implementations. Performance is not a Loop 1 or Loop 2 concern. Any optimization happens in a future project after correctness is established.
- **Refactoring outside the canonical-ABI surface.** Loops 1, 2, and 3 do not touch unrelated code. Each loop's isolation rules forbid incidental edits.
- **WIT-bindgen replacement or component-tooling work.** Out of scope.
- **API surface changes.** The wazero public API does not change in this project. If a Loop 2 migration appears to require an API change, the migration is halted with a `BLOCKER:` and the user resolves it (likely by reopening Loop 1 to redesign the abi/ entry point so the existing public API can continue to call into it).

## 12. Open risks

- **`abi/` may have its own gaps that Loop 1 reconciliation does not catch.** Mitigation: Gates 2, 3, 4, and 6 are independent of the reconciliation report; they catch behavioral divergences the report missed.
- **`definitions.py` may itself diverge from `CanonicalABI.md` in places.** Mitigation: every reconciliation classifies findings into `bug-in-go`/`bug-in-python`/`intentional-deviation`; `bug-in-python` requires a spec quote and halts with a `BLOCKER:` if one cannot be produced.
- **The installed `wasmtime` CLI version may not match the vendored spec/`definitions.py` version.** Mitigation: Gate 4 records the installed version in its first run and `BLOCKER:`s on a known-incompatible version. The user resolves by upgrading or downgrading wasmtime.
- **Loop 2 may surface bugs in `abi/` that escaped all six Loop 1 gates.** Mitigation: Rule L2-A halts immediately and reopens Loop 1 under the full Gate 1–7 protocol. There is no fast-path for Loop 2 to fix abi/ directly.
- **The wasip2 cleanup (Loop 3) may break existing test fixtures that depended on the silent-default behavior.** Mitigation: Loop 3's audit-and-fix protocol mirrors Loop 2's, treating any test that asserts a silent-default as expected behavior as a test that locks in a wazero bug.
- **A future spec update upstream may invalidate large parts of the Loop 1 work.** Mitigation: every doc comment cites both the spec section and the vendored `definitions.py` line range, so a future spec-update audit can mechanically diff. The reconciliation reports under `status/reconciliation/` provide a forensic record of which sections were verified against which version of the spec.

---

*End of design spec. Plan to be written next by the writing-plans skill at `docs/superpowers/plans/2026-04-07-canonical-abi-unification.md`.*
