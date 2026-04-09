# Canonical-ABI Session 1 — Per-Task Review Log

Append-only log of every task's dual-reviewer dispatch, findings, and
correctives. One entry per task in plan order.

**Plan:** docs/superpowers/plans/2026-04-08-canonical-abi-session1-plan.md
**Design:** docs/superpowers/specs/2026-04-08-canonical-abi-session1-design.md
**Branch:** feat/wasip2-complete-implementation

---

## Task A1 — Add TypeDef resource destructor fields + change TypeDef.Func to FuncTypeIdx

**Code commit:** `5885f93e`
**Base:** `9e98ea9f`
**Files changed:**
- `internal/component/component.go` — TypeDef struct: Func → FuncTypeIdx; added ResourceDtor/ResourceDtorAsync/ResourceDtorCallback; added FuncType(c) helper.
- `internal/component/component_typedef_test.go` — new, 2 tests with citation blocks.

### Spec-compliance reviewer

**Verdict:** ✅ SPEC COMPLIANT.

Checklist results:
1. Spec citation on TypeDef struct cites `definitions.py:351-361` and Decision 5 — confirmed by direct read of upstream.
2. Both new tests have citation blocks within 15 lines of declaration (lines 12, 30 per declarations at 16, 31).
3. Citation verification: `definitions.py:88-101` is FuncType dataclass; `definitions.py:351-361` is ResourceType with `{dtor, dtor_async, dtor_callback}`. Both opened and confirmed.
4. Assertions match spec-observable behavior; Optional-via-pointer encoding correct.
5. Dedup grep: two test names appear only in the new file.
6. No TODO/FIXME/XXX introduced.
7. No panic/session-1-work/DeferredToSession1 markers.
8. Parallel-paths audit: only one `type TypeDef struct` in internal/component/.
9. Call-site migration audit: no production call site read/wrote `TypeDef.Func`; all `.Func` hits classified as unrelated (CoreTypeDef.Func, builder method, helper comment).
10. Build + tests green. Non-regression confirmed.

### Code quality reviewer (superpowers:code-reviewer)

**Verdict:** APPROVED_WITH_MINOR.

- No CRITICAL or IMPORTANT findings.
- Strengths: struct doc clarified ("per-slot in order; non-Kind fields zero"); per-field kind-specific docs added; FuncType helper receiver is `*TypeDef` (avoids copy); additive change with zero regression risk.
- MINOR findings (not blocking):
  - M1: Tests use raw `t.Fatalf` instead of package-conventional `require.*` helpers.
  - M2: `FuncType` helper has no positive-path test; caller discipline documented but not enforced at runtime.
  - M3: No zero-value assertion for optional dtor fields in the test.
  - M4: Magic constants in test body.

### Correctives

None. APPROVED_WITH_MINOR is sufficient per the user's execution discipline
("correctives required only for CRITICAL findings"). M1-M4 are tracked here
for a future cleanup pass but do not block Task A2.

### Task status

✅ Complete. Proceeding to Task A2.

---

## Task A2 — Add Component.TypeDefs []TypeDef field

**Code commit:** `d7b4a41a`
**Base:** `5885f93e`
**Files changed:**
- `internal/component/component.go` — added `TypeDefs []TypeDef` field with doc comment citing Decision 5.
- `internal/component/component_typedef_test.go` — appended `TestComponentTypeDefsField`.

### Spec-compliance reviewer

**Verdict:** ✅ SPEC COMPLIANT.

- Citation: "No counterpart (justified):" block within 15 lines of declaration.
- Struct doc comment cites Decision 5 and describes caller pattern.
- Dedup: exactly one field hit.
- No TODO/stub/panic markers.
- No parallel paths.
- Build + tests green.
- Design alignment verified against lines 382-411 and 1229-1248.

### Code quality reviewer

**Verdict:** APPROVED_WITH_MINOR.

- Strengths: field placement adjacent to `Types`; load-bearing doc comment (caller contract, legacy map callout, design citation); minimal test scope.
- MINOR: test uses raw `t.Fatalf` instead of package `require.*` helpers (same finding as A1, file-internally consistent). Tracked for a Session 2 cleanup sweep.

### Correctives

None. Proceeding to Task A3.

### Task status

✅ Complete.

---

## Task A3 — Populate Component.TypeDefs in binary decoder

**Code commit:** `137fdaa1`
**Base:** `d7b4a41a`
**Files changed:**
- `internal/component/binary/decoder.go` — every case in `decodeTypeSection` now appends one `TypeDef`; added in-loop invariant using `beforeTypeDefs` baseline to tolerate alias sections.
- `internal/component/binary/decoder_test.go` — new end-to-end `TestDecoderPopulatesTypeDefs` exercising `DecodeComponent` on a hand-assembled binary with 5 type slots.
- `internal/component/component_test.go` — rewrote previously-skipped `TestNewTypeDefs` as a compile-time shape canary (old body in `98b3bbc3` used deleted types).

**Implementer status:** DONE_WITH_CONCERNS. Both concerns verified as legitimate escalations:
1. `98b3bbc3` old body referenced deleted types → fresh body per plan's escalation clause.
2. `component_test.go` in `package component` cannot import `internal/component/binary` (cycle) → split: shape test in `component_test.go`, real decoder test in `binary/decoder_test.go`.

### Spec-compliance reviewer

**Verdict:** ✅ SPEC COMPLIANT.

- Opcode coverage: every case (TypeOpFuncSync/Async, TypeOpResourceSync/Async, TypeOpInstance, TypeOpComponent, 11 ValType opcodes, default primitive) appends one TypeDef.
- Per-slot invariant in-loop and stricter than plan (avoids alias-section false positive).
- Resource destructor fields propagated.
- Citation blocks on both restored tests.
- No new parallel data structures (funcTypeIdx/resourceDefs still coexist; A4 deletes them).
- Build + tests green within scope.

### Code quality reviewer

**Verdict:** APPROVED_WITH_MINOR.

- Strengths: tight plan alignment; invariant avoids alias false positive; real end-to-end test; style-clean.
- MINOR (all non-blocking):
  - M1: TestNewTypeDefs docstring should explicitly call it a compile-time canary.
  - M2: 12 ValType* opcode cases share identical append tail (defer consolidation until A4 or later).
  - M3: Test only covers happy path; two-section test would exercise baseline-delta invariant directly.
  - M4: Invariant error message could include opcode byte.

### Correctives

None. Proceeding to Task A4.

### Task status

✅ Complete.

---

## Task A4 — Delete funcTypeIdx + resourceDefs private maps

**Code commit:** `7bbf9a06`
**Base:** `137fdaa1`
**Files changed:**
- `internal/component/binary/decoder.go` — deleted both struct fields, their `make()` initializers, and 3 write sites (the maps were write-only; no readers existed).

Audit findings:
- `decoder.go:248`: `dc.funcTypeIdx[slot] = ftIdx` → deleted; TypeDef.Func already carries this.
- `decoder.go:260`: `dc.resourceDefs[slot] = resourceDef` → deleted.
- `decoder.go:275`: `dc.resourceDefs[slot] = resourceDef` (async) → deleted.

No Kind assertions needed at migration — no readers existed.

V5 verification: `grep -rn 'funcTypeIdx\|resourceDefs' internal/component/binary/` → empty.

### Spec-compliance reviewer

**Verdict:** ✅ SPEC COMPLIANT.

- Write-only claim verified: all 3 pre-A4 usages of the two maps were writes (decoder.go:248, 260, 275 in 137fdaa1). No reads in binary/ or codebase-wide.
- V5 grep empty.
- No parallel structure introduced.
- Struct fields + `make()` initializers + docstrings all cleanly removed.
- Build + 9 component packages test green.
- Commit message cites Decision 5 and V5.

### Code quality reviewer

**Verdict:** APPROVED_WITH_MINOR (with one IMPORTANT docstring overpromise — see below).

Strengths: pure deletion, no drift; atomic change; write-only verification; V5 honored; tests green.

**IMPORTANT finding (I1): `decodeContext` + `component.TypeDefs` docstrings overpromise caller semantics.**
Both docstrings imply callers can do `c.TypeDefs[canon.TypeIdx]` / `c.TypeDefs[slot]`, but `internal/component/binary/alias.go:144-162` bumps `Component.NextTypeIdx` on every outer/export type alias WITHOUT appending to `TypeDefs`. So `len(TypeDefs) < NextTypeIdx` when aliases are present, and direct indexing by `canon.TypeIdx` misreads or panics.

The reviewer noted this is a pre-existing condition from A2/A3 that A4 inherits; no live non-test caller uses this path at HEAD. But the design doc's Decoder → Linker Indirection section (design lines 1241-1247) explicitly lists `component_linker.go::Instantiate` and `type_checker.go::checkFuncDefinition` as callers that resolve via `c.TypeDefs[canon.TypeIdx]` — so Checkpoint C WILL exercise this and needs alias-aware resolution.

**Checkpoint C prerequisite — must be raised with the user before C1:**
The design's expected caller contract (`c.TypeDefs[canon.TypeIdx]`) requires either (a) densifying TypeDefs so aliases produce entries pointing at the aliased target, OR (b) an alias-aware resolver helper on Component that walks through aliases. The plan and design do not specify which — this is a design-gap that blocks Checkpoint C and needs user decision.

### Correctives applied

Commit `<pending>`: docstring clarification in both `component.go` (TypeDefs field) and `binary/decoder.go` (decodeContext) to:
1. Honestly state that TypeDefs is NOT densely aligned with NextTypeIdx due to alias-sparsity.
2. Tell callers to go through an alias-aware resolver (to be added in Checkpoint C) rather than indexing directly.
3. Flag the design gap as a Checkpoint C prerequisite.

Build + 9 component packages test still green after the docstring change.

MINOR observations from the code-quality review (not blocking, not fixed):
- M1: docstring phrasing "lives on" vs "is recorded on" inconsistency.
- M2: line-numbered design references in commit messages are brittle.
- M3: post-condition assertion on `TypeDef.Func == builder-assigned ftIdx` is not required but would catch mismatches.

### Task status

✅ Complete. Proceeding to Task A5.

⚠️ **Checkpoint C prerequisite** raised: alias-aware TypeDefs resolver needs a
design decision before C1 starts. Must flag to user.

---

## Task A5 — Restore 8 component_type_test.go tests

**Code commit:** `0c395a96`
**Base:** `06b95970`
**Files changed:**
- `internal/component/binary/component_type_test.go` — 8 restored tests with citation blocks, migrated to `c.TypeDefs[i]` + `require.*` helpers, explicit byte-builder style.

Tests restored: TestDecodeComponentType, TestDecodeComponentTypeEmpty, TestDecodeComponentTypeWithExport, TestDecodeComponentTypeWithAlias, TestDecodeComponentTypeWithCoreType, TestDecodeComponentTypeWithNestedType, TestDecodeComponentTypeMultipleDeclarations, TestDecodeComponentTypeImportInstance.

One non-mechanical rewrite: `TestDecodeComponentTypeWithNestedType` — old body asserted `decl.Type.Record != nil`; `TypeDef` no longer carries Record. Reworked to assert `Kind == TypeDefKindDefined` with a Session 0 caveat comment pointing at Session 2 for structural assertion restoration.

### Spec-compliance reviewer

**Verdict:** ✅ SPEC COMPLIANT.

- 8 functions present, zero session-1-work skips.
- Citation blocks on each (all within 15 lines of declaration).
- Nested-type rework verified: `decodeNestedTypeDef` at `instance_type.go:288-295` sets `Kind = TypeDefKindDefined` without populating Record — assertion matches decoder.
- Cited `Binary.md` line ranges confirmed (component-type 0x41, externdesc 0x01/0x04/0x05, alias sort encoding).
- No new TODO/FIXME/XXX markers.
- No duplicates.
- Build + 8/8 tests green.

### Code quality reviewer

**Verdict:** APPROVED.

- Strengths: faithful plan execution; stronger assertions than originals (added Import.ExternDesc.Kind checks); uniform citation format; byte-builder style preserved per convention.
- No CRITICAL / IMPORTANT / MINOR blockers.
- Soft suggestions for later polish: add alias OuterIdx assertion; add TODO-placed marker near nested-type caveat to be visible at assertion-site.

### Correctives

None.

### Task status

✅ Complete. Proceeding to Task A6.

---

## Task A6 — Restore 10 instance_type_test.go tests

**Code commit:** `55c8081e`
**Base:** `0c395a96`
**Files changed:**
- `internal/component/binary/instance_type_test.go` — 10 restored tests with citation blocks.

Tests restored: TestDecodeInstanceType, TestDecodeInstanceTypeWithAlias, TestDecodeInstanceTypeEmpty, TestDecodeInstanceTypeMultipleExports, TestDecodeInstanceTypeWithCoreType, TestDecodeInstanceTypeWithNestedType, TestDecodeInstanceTypeCoreModuleExport, TestDecodeInstanceTypeInstanceExport, TestDecodeInstanceTypeComponentExport, TestDecodeInstanceTypeValueExport.

Three non-mechanical rewrites:
1. `TestDecodeInstanceTypeMultipleExports`: eq-bound encoding 0x01 → 0x00 (current `decodeInstanceExportDecl` at instance_type.go:153-164 uses 0x00=eq with typeidx, 0x01=sub-resource without index — spec-correct per Binary.md:239).
2. `TestDecodeInstanceTypeWithNestedType`: old `decl.Type.Record != nil` weakened to `Kind == TypeDefKindDefined`. Decoder doesn't populate Record yet (Session 2 scope).
3. `TestDecodeInstanceTypeCoreModuleExport`: deliberately does not assert `Export.Kind` because the decoder currently shims externdesc 0x00 → ExportKindFunc as a temporary placeholder.

### Spec-compliance reviewer

**Verdict:** ✅ SPEC COMPLIANT.

- 10 functions, zero skips.
- Citation blocks within 15 lines of every declaration.
- All three non-mechanical rewrites verified against current decoder source and Binary.md spec.
- Every encoded byte sequence in every test verified against Binary.md grammar (lines 224-242).
- No TODO/FIXME/XXX markers.
- No duplicates.
- Build + 10/10 tests green.
- Soft finding: pre-existing valuebound discriminator shim at `instance_type.go:138-143` and `import.go:74-82` — the decoder reads a bare valtype after externdesc 0x02 instead of the spec's `valuebound` discriminator. Out of A6 scope; worth a Session 2 follow-up.

### Code quality reviewer

**Verdict:** APPROVED_WITH_MINOR.

- Strengths: exact plan alignment; strictly stronger assertions than originals; uniform style with component_type_test.go; 366-line file remains readable; `require.*` throughout.
- MINOR 1: `TestDecodeInstanceTypeCoreModuleExport` docstring should explain why Export.Kind is not asserted (consistency with nested-type caveat).
- MINOR 2: `TestDecodeInstanceTypeValueExport` docstring phrasing "In Session 0" is misleading; should reference the `valuebound` discriminator shim explicitly.
- MINOR 3: Coverage gaps worth flagging for Session 2 follow-up (nested-record structural content, valuebound payload, core-module Kind, alias Kind assertion in TestDecodeInstanceTypeWithAlias).
- **MINOR 4 / IMPORTANT flag: pre-existing `import.go:83-102` typebound inversion bug** — comment and code say `0x00=sub, 0x01=eq` but spec (Binary.md:239-240) and `instance_type.go:145-164` correctly say `0x00=eq, 0x01=sub resource`. The import-side decoder has swapped tags AND unconditionally reads a typeidx even for the sub-resource case (which has no following index per spec). Existing `import_test.go:195-236` locks in the wrong behavior. This is a real spec bug, not an A6 regression, and must be raised with the user before Checkpoint A is signed off.

### Correctives applied in A6 corrective commit

1. Added Session 0 caveat to `TestDecodeInstanceTypeCoreModuleExport` docstring explaining the ExportKindFunc shim.
2. Rewrote `TestDecodeInstanceTypeValueExport` docstring to reference the valuebound discriminator shim and Binary.md:241-242.

### Deferred for user decision (Checkpoint A close)

- ⚠️ **import.go typebound inversion (real spec bug)**: Must be raised with user. Options are (a) fix now as a new corrective task before Checkpoint A closes, (b) file as Session 2 followup.
- ⚠️ **Checkpoint C prerequisite (from Task A4 corrective)**: alias-aware TypeDefs resolver.

### Task status

✅ Complete. Proceeding to Task A7.

---

## Task A7 — Checkpoint A Verification

**No code commit.** This is a checkpoint-gate task that runs the exit-criterion bash script, executes V5/V9 grep verifications, and dispatches checkpoint-level reviewers over the A1-A6 scope.

### Automated exit criteria

| Check | Command | Result |
|---|---|---|
| A7.1 Build | `go build ./internal/component/binary/... ./internal/component/...` | ✅ empty |
| A7.2 Tests | `go test ./internal/component/binary/... -count=1` | ✅ `ok ...binary 0.005s` |
| A7.3 V5 | `grep -rn 'funcTypeIdx\|resourceDefs' internal/component/binary/` | ✅ empty |
| A7.4 V9 (a) | `grep -n 'TypeDefs \[\]TypeDef' internal/component/component.go` | ✅ `52: TypeDefs []TypeDef` |
| A7.4 V9 (b) | `grep -n 'c\.TypeDefs = append' internal/component/binary/decoder.go` | ✅ 17 hits across all opcodes |
| A7.5 Working tree | `git status --porcelain` | ✅ only pre-existing `.env`/`.envrc` |

### Checkpoint-level spec-compliance reviewer

**Verdict:** ✅ CHECKPOINT A SPEC COMPLIANT.

V4 citation audit: 23/23 modified-or-added test functions have valid citation blocks within 15 lines of their declarations (3 in component_typedef_test.go, 1 in component_test.go, 1 in decoder_test.go, 8 in component_type_test.go, 10 in instance_type_test.go).

Decoder walk: every `decodeTypeSection` case appends exactly one `TypeDef` with the correct Kind + kind-specific field. In-loop invariant at decoder.go:461-464 tolerates alias-sparsity correctly.

Restored tests walk: all 18 tests exercise the full `DecodeComponent` path, no stubs. Three non-mechanical rewrites in A6 all cite the actual decoder behavior.

Both pre-flagged issues confirmed:
- Alias-sparsity docstrings describe current state honestly; captured as Checkpoint C prerequisite.
- import.go typebound inversion bug is real (lines 83-102: 0x00→TypeBoundSub, 0x01→TypeBoundEq, swapped from spec; unconditional typeidx read even for sub-resource). Sibling instance_type.go:145-164 is spec-correct. Test lock-in in import_test.go:195-236.

### Checkpoint-level code-quality reviewer

**Verdict:** APPROVED_WITH_MINOR (close Checkpoint A without correctives).

Strengths: tight plan alignment; no partial-state commits; Decision 5 option A executed correctly; in-loop invariant is stricter than the plan asked; A4 pure deletion with zero drift; A5/A6 restorations strictly stronger than originals; uniform citation discipline; honest documentation of the alias-sparsity hazard.

IMPORTANT I1: **import.go:83-102 typebound inversion** — pre-existing spec bug, not introduced by Checkpoint A. Reviewer recommends promoting from "Session 2 vague followup" to named Checkpoint C prerequisite with concrete test cases.

MINOR (all non-blocking, deferred cleanup pass): docstring wording drift; 12 ValType* opcode cases share append tail; raw t.Fatalf in component_typedef_test.go; TestNewTypeDefs isn't explicitly called a "canary"; no positive-path test on FuncType helper; "Session 2" label hardcoded in a few comments.

Integration: A1→A2→A3→A4 composes cleanly. At no commit is there a partial state. No production caller indexes `TypeDefs[canon.TypeIdx]` at Checkpoint A HEAD — all hazard-prone consumers land in Checkpoint C.

### Correctives

**None required to close Checkpoint A.** Both flagged issues are next-checkpoint concerns.

### Deferred for user decision at Checkpoint B → C transition

1. **Alias-sparsity resolution strategy** (from Task A4). Densify `TypeDefs` on alias (append an entry referencing the aliased target) OR add an alias-aware resolver helper on Component (walks aliases at lookup time). Design doc lines 1241-1247 list `component_linker.Instantiate` and `type_checker.checkFuncDefinition` as the first two callers that force this choice.

2. **import.go typebound inversion** (from Task A6). File: `internal/component/binary/import.go:83-102`. Spec: Binary.md:239-240. Fix is bounded: flip the tag-to-name mapping, gate typeidx read on tag, rewrite locked-in tests at `import_test.go:195-236`, possibly flip enum constants at `component.go:693-695`. Recommendation: land a corrective before Checkpoint C starts, with new tests for both `(import "r" (type (eq T)))` and `(import "r" (type (sub resource)))`.

### Task status

✅ Complete. Checkpoint A closed.

---

## Checkpoint A Summary

**Commits:** 9 (7 task commits + 2 correctives)
**Files touched:** 7 (component.go, component_typedef_test.go, component_test.go, binary/decoder.go, binary/decoder_test.go, binary/component_type_test.go, binary/instance_type_test.go)
**Tests added/restored:** 23 (3 new in component_typedef_test.go, 1 rewrite in component_test.go, 1 new in decoder_test.go, 8 restored in component_type_test.go, 10 restored in instance_type_test.go)
**Build status:** green on `./internal/component/...`
**Test status:** 9 component packages green

**Per-task statuses:**
| Task | Commit | Spec | Quality | Corrective |
|---|---|---|---|---|
| A1 | 5885f93e | ✅ | APPROVED_WITH_MINOR | — |
| A2 | d7b4a41a | ✅ | APPROVED_WITH_MINOR | — |
| A3 | 137fdaa1 | ✅ | APPROVED_WITH_MINOR | — |
| A4 | 7bbf9a06 + 06b95970 | ✅ | APPROVED_WITH_MINOR → docstring corrective | ✅ landed |
| A5 | 0c395a96 | ✅ | APPROVED | — |
| A6 | 55c8081e + 0c8f42a2 | ✅ | APPROVED_WITH_MINOR → docstring corrective + flagged pre-existing import.go bug | ✅ landed |
| A7 | (no commit; verification only) | ✅ | APPROVED_WITH_MINOR | — |

**Two items requiring user decision before starting Checkpoint C:**
1. Alias-aware TypeDefs resolver strategy.
2. import.go typebound inversion bug fix approach.

Checkpoint B (runtime instance embedding) does NOT depend on either of these and can proceed immediately.

---

# Checkpoint B — `component.Instance` embeds `*runtime.ComponentInstance`

## Task B1 — Decouple IsMayLeave from enterCount

**Code commit:** `f831c4ee`
**Base:** `daf31065`
**Files changed:**
- `internal/component/runtime/component_instance.go` — `IsMayLeave()` returns `c.MayLeave` directly; updated doc cites definitions.py:260, 270, 1955, 1973, 2065, 2135, 2143 and the Session 1 fix rationale.
- `internal/component/runtime/component_instance_may_leave_test.go` — new file with `TestIsMayLeaveIsStandaloneBoolean`.
- `internal/component/runtime/component_instance_test.go` — `TestComponentInstance_IsMayLeave` assertion flipped with citation.
- `internal/component/runtime/table_test.go` — `TestTable_NewWithMayLeaveCheck` updated (replaced `inst.Enter()` indirection with direct `inst.MayLeave = false`) with citation.

### Spec-compliance reviewer

**Verdict:** ✅ SPEC COMPLIANT.

- Confirmed all 7 cited spec lines by reading `definitions.py` directly.
- Citation block present on new test and both pre-existing test rewrites.
- Wasmtime parallel verified (note: design's `concurrent_disabled.rs:159` citation is imprecise — true may_leave parallel lives at `vm/component.rs:1000-1128` where flags are distinct bits. This reinforces Decision 3's orthogonality thesis).
- No cross-package callers of `runtime.ComponentInstance.IsMayLeave`. Parallel path `component.Instance.MayLeave()` at `instance.go:223-226` reads separate `mayLeaveDisabled` field — correctly out of B1 scope (target for Checkpoint B embedding task).
- Build + full runtime package green.

### Code quality reviewer

**Verdict:** APPROVED.

- Strengths: minimal 1-line production change; root cause explained in code + commit; test exhausts 2×2 state matrix; pre-existing tests rewritten without weakening unrelated assertions.
- MINOR (non-blocking): test coverage between new file and `TestComponentInstance_IsMayLeave` partially overlaps; file placement (standalone vs append) is defensible but not strictly justified; new test file header could link Session 1 Decision 3 directly.

### Correctives

None.

### Task status

✅ Complete. Proceeding to Task B2.

---

## Task B2 — ReentranceTracker.CallMightBeRecursive (isActive helper + doc)

**Code commit:** `c0a745c9` + corrective `cb15ffc4`
**Base:** `f831c4ee`
**Files changed:**
- `internal/component/runtime/reentrance.go` — extracted `isActive` helper, added nil-receiver guard, expanded docstring.
- `internal/component/runtime/reentrance_test.go` — new `TestReentranceTrackerCallMightBeRecursive` scenario test.

Implementer discovered the method already existed with correct behavior; refactored per plan's "do NOT replace" constraint.

**Spec review:** ✅ SPEC COMPLIANT (2 docs-only nits: forward-looking "done by runtime delegator" hedge and inaccurate Python paraphrase, both addressed in corrective `cb15ffc4`).

**Code quality review:** APPROVED.

### Task status

✅ Complete. Proceeding to Task B3.

---

## Task B3 — Rewrite component.Instance to embed *runtime.ComponentInstance

**Code commit:** `4da3d03d`
**Base:** `cb15ffc4`
**Files changed:**
- `internal/component/instance.go` — struct rewrite; `rt *runtime.ComponentInstance` as first field; 5 duplicated fields deleted; 10 delegator methods added/rewritten; 3 methods deleted (SetCallContext, CallContext, SetDestructor old signature); errMayNotLeave sentinel added.
- `internal/component/instance_test.go` — 3 new tests with citation blocks.

**Status: DONE_WITH_CONCERNS** (intentional per plan — build broken at `nested_component.go:49` for B4 to fix; 3 new tests deferred to B5).

### Spec-compliance reviewer

**Verdict:** ✅ SPEC COMPLIANT.

- Struct shape matches Decision 3 exactly; rt is first field.
- All runtime.ComponentInstance dependencies (ID, MayLeave, IsMayLeave, Enter/Leave, EnterCount, Reentrance, Table, Destructors, Parent) present.
- Delegators semantically correct against definitions.py:256-273.
- No silent field drops (12 linker-time fields preserved; 5 deleted per plan).
- SetDestructor deletion legitimate: zero live call sites; old `(uint32, func(any))` signature incompatible with spec-correct `(*ResourceType, DestructorFunc)`.
- Build broken only at expected single site (nested_component.go:49).

### Code quality reviewer

**Verdict:** APPROVED.

Strengths: faithful plan execution; rt field placement is ideal (first field); thin delegators; spec line citations; IsMayLeave divergence visibly fixed.

Suggestions (non-blocking): rt field name rationale; errMayNotLeave could hoist to runtime package; no FuncType positive-path test.

### Task status

✅ Complete. Proceeding to Task B4.

---

## Task B4 — Migrate call sites to embedded shape + precise error stubs

**Code commit:** `5c800896` + corrective `b74f5558` + polish `90f43a29` + checkpoint-close `bca037da`
**Base:** `4da3d03d`
**Files changed:**
- `internal/component/instance.go` — panic stubs replaced with precise "rebuild in progress" errors citing Task C5 (ExportedFunc.Call) and Task E5 (Resource*). NewInstance exported helper added. Nil-safety guards restored on ActiveCallDepth/EnterCall/ExitCall.
- `internal/component/nested_component.go` — struct literal migrated to newInstance.
- `internal/component/instantiate.go`, `internal/component/linker.go` — struct literals migrated.
- `internal/component/edge_case_test.go`, `internal/component/outer_alias_test.go` — test fixtures migrated.
- `internal/component/conformance/may_leave_test.go`, `internal/component/conformance/reentrance_test.go` — test fixtures migrated + error-substring assertions updated.

### Spec-compliance reviewer (B4 initial)

**Verdict:** ✅ SPEC COMPLIANT. Build + B3 accessor tests green.

**Test failures flagged** (not a B4 regression): 5 conformance subtests failing because they encoded old wazero-specific `caller == i && depth > 0` semantics. Tests predate B3; implementation gap is the tracker-based CallMightBeRecursive not implementing the spec's reflexive-ancestor overlap.

### B4 corrective (`b74f5558`)

Rewrote `Instance.CallMightBeRecursive` as a structural parent-chain walk via new `isReflexiveAncestor` helper. Matches `definitions.py:290-299` exactly. Rewrote 2 spec-wrong subtests (`same_instance_no_active_call`, `no_active_call_passes` → renamed `no_active_call_still_recursive`). All previously-failing conformance subtests now pass.

### B4 polish (`90f43a29`)

Documentation-only: Decision 3 in design doc updated to show structural walk + "tracker must not be reintroduced" warning; instance.go docstring hedge removed; reentrance.go docstring rewritten to cite definitions.py:3664-3667 (its actual purpose, task-level concurrency trap) instead of :290-299.

### Code quality reviewer (full B4)

**Verdict:** APPROVED_WITH_MINOR.

3 IMPORTANT docs-only findings (all addressed in `bca037da` Checkpoint B close commit):
- I1: Design doc line 833-835 stale CallMightBeRecursive sketch → rewritten to structural walk with warning.
- I2: instance.go docstring hedge "may be supplemented by tracker" → firmed up.
- I3: reentrance.go docstring cited :290-299 → rewritten to cite :3664-3667.

Strengths: commit hygiene exemplary; nil-safety uniform; test fixture migration complete; precise error stubs match Decision 7 signatures; B4 corrective untangled two conflated spec checks with defense-in-depth warnings in 3 locations.

### Task status

✅ Complete. Proceeding to Task B5.

---

## Task B5 — Checkpoint B verification

**Code commit:** `bca037da` (doc fixes from checkpoint-level review findings)

Automated exit criteria (B5.1-B5.5): all pass.

| Check | Command | Result |
|---|---|---|
| B5.1 Build | `go build ./internal/component/... ./imports/wasip2/...` | ✅ empty |
| B5.2 Runtime tests | `go test ./internal/component/runtime/... -count=1` | ✅ green |
| B5.3 Accessor tests | `go test ./internal/component/ -run 'TestInstance(Embeds|MayLeave|CallMight)'` | ✅ green |
| B5.4 V6 | `grep 'table.*runtime\.Table\|mayLeaveDisabled\|activeCallDepth' internal/component/instance.go` | ✅ empty |
| B5.5 Working tree | `git status --porcelain` | ✅ only .env/.envrc |

### Checkpoint-level spec-compliance reviewer

**Verdict:** APPROVED_WITH_MINOR → APPROVED after applying I1 fix (stale design doc sketch at lines 833-835).

### Checkpoint-level code quality reviewer

**Verdict:** APPROVED_WITH_MINOR (non-blocking).

Minor items (deferred to Checkpoint C / E where those files will be touched):
- I1: `reentrance_test.go:49-60` stale citation → **addressed in `bca037da`**.
- I2: Test name `TestInstanceCallMightBeRecursiveUsesReentranceTracker` is now a lie. Kept for plan traceability (plan line 1063); can rename later.

### Task status

✅ Complete. Checkpoint B closed.

---

## Checkpoint B Summary

**Commits:** 10 (5 task commits + 5 correctives/polish)
- `f831c4ee` B1: decouple IsMayLeave from enterCount
- `c0a745c9` B2: ReentranceTracker.CallMightBeRecursive refactor
- `cb15ffc4` B2 corrective: docstring softening
- `4da3d03d` B3: Instance embeds *runtime.ComponentInstance (intentional broken build)
- `5c800896` B4: call-site migration
- `b74f5558` B4 corrective: structural CallMightBeRecursive
- `90f43a29` B4 polish: Decision 3 + docstring hygiene
- `bca037da` B5 close: stale sketch + test citation

**Files touched:** 11
- `internal/component/instance.go`
- `internal/component/instance_test.go`
- `internal/component/runtime/component_instance.go`
- `internal/component/runtime/component_instance_may_leave_test.go` (new)
- `internal/component/runtime/component_instance_test.go`
- `internal/component/runtime/reentrance.go`
- `internal/component/runtime/reentrance_test.go`
- `internal/component/runtime/table_test.go`
- `internal/component/nested_component.go`
- `internal/component/instantiate.go`
- `internal/component/linker.go`
- `internal/component/conformance/may_leave_test.go`
- `internal/component/conformance/reentrance_test.go`
- `internal/component/edge_case_test.go`
- `internal/component/outer_alias_test.go`
- `docs/superpowers/specs/2026-04-08-canonical-abi-session1-design.md`

**Key architectural delivery:**
- `IsMayLeave` decoupled from `enterCount` (spec-orthogonal).
- `component.Instance` embeds `*runtime.ComponentInstance` via `rt` field.
- 5 duplicated fields deleted, 10 delegator methods added.
- `CallMightBeRecursive` correctly implements `definitions.py:290-299` structural reflexive-ancestor walk.
- `ReentranceTracker` retained for its separate purpose (`definitions.py:3664-3667` task-level concurrency trap).
- Precise "rebuild in progress" error stubs replace panics on `ExportedFunc.Call` and `Resource*` with citations to the Tasks that will fill them in (C5, E5).
- Build green on `./internal/component/...` and `./imports/wasip2/...`.

**Both Checkpoint C prerequisites RESOLVED via deep spec + wasmtime research** (research session 2026-04-09):

### Research-driven corrective 1: import.go typebound inversion

**Commit:** `15447fdc`

Research confirmed `binary/instance_type.go:145-164` was spec-correct and `binary/import.go:83-102` was wrong. Per `Binary.md:239-240`:
- `typebound ::= 0x00 i:<typeidx> => (eq i)` (reads typeidx)
- `typebound ::= 0x01 => (sub resource)` (no payload)

Fix landed via Red/Green TDD:
- Flipped constants: `TypeBoundEq = 0x00`, renamed `TypeBoundSub → TypeBoundSubResource = 0x01`.
- Rewrote `import.go:83-107` to switch on tag and only read typeidx when tag == 0x00.
- Deleted 4 spec-wrong tests (`TestDecodeExternDesc_Type`, `TestDecodeExternDesc_TypeEq`, `TestDecodeImportWithTypeBound`, `TestDecodeImportWithTypeBoundEq`) that locked in the bug.
- Added 3 spec-correct tests (`TestDecodeImportTypeSubResource`, `TestDecodeImportTypeEq`, `TestDecodeImportTypeBothBoundsSideBySide`) with citation blocks pointing at Binary.md + wasmtime fixtures at `tests/all/component_model/resources.rs:14` and `tests/misc_testsuite/component-model/types.wast:327`.

**Ground truth citations:**
- `Binary.md:231-243` — shared externdesc production for import and export.
- `wac/crates/wac-graph/src/encoding.rs:673,761,813,827` — `TypeBounds::Eq(u32)` / `TypeBounds::SubResource`.
- `wasmtime/tests/all/component_model/resources.rs:14-15` — `(import "t" (type $t (sub resource)))`.
- `wasmtime/tests/misc_testsuite/component-model/instance.wast:288-325` — both forms side by side.

### Research-driven corrective 2: Component.TypeDefs alias densification

**Commits:** `f821c0fb` (main fix) + `621ada86` (reviewer-nit polish)

Research definitively established:
- Spec (Explainer.md:326-338): "the `id` of the alias is bound to the **new index added by the alias**" — aliases consume slots in the component's type index space.
- Wasmtime (`crates/environ/src/component/translate.rs:796-801`): delegates typeidx resolution to `wasmparser::Validator.component_any_type_at(typeidx)` which transparently walks alias chains at use sites. Translator's outer-Type-alias branch at `:1499-1501` is EMPTY because the validator handles it internally.
- Wizer (`crates/wizer/src/component/parse.rs:118-137`): explicitly calls `inc_types()` on every type-section entry AND every outer/export type alias — direct counter-based proof that aliases consume slots.
- Wac encoder (`crates/wac-graph/src/encoding.rs:163-182`): when emitting an outer alias, uses `self.current.encodable.type_count()` as the resulting index.

Fix landed via Red/Green TDD:
1. New `TypeDefKindAlias` enum variant with Binary.md citation.
2. New `AliasTarget` struct with `IsExport` discriminator, outer-alias fields (`OuterCount`, `OuterIndex`) and export-alias fields (`InstanceIdx`, `ExportName`).
3. New `TypeDef.Alias *AliasTarget` field.
4. New `Component.ResolveTypeDef(typeidx) (*TypeDef, uint32, error)` helper — mirror of `wasmparser::Validator.component_any_type_at`. Walks alias chains via `OuterIndex` with cycle detection. Cross-scope (`OuterCount > 0`) and export (`IsExport == true`) alias resolution is deferred to the wiring layer with precise spec-line-cited errors.
5. `binary/alias.go` `SortType` branches (both `AliasKindExport` and `AliasKindOuter`) now append a densified `TypeDef{Kind: TypeDefKindAlias, Alias: ...}` entry alongside `NextTypeIdx++`.
6. Whole-component decoder invariant added to `DecodeComponent`: `len(TypeDefs) == NextTypeIdx` — build-time insurance against future sparsity regressions. Pre-existing per-call `decodeTypeSection` delta check preserved.
7. 3 Red/Green tests grounded in wasmtime + wasm-tools corpus:
   - `TestDecoderOuterTypeAliasConsumesSlot` — mirrors `wasm-tools/tests/cli/component-model/resources.wast:779-796` outer-0 self-alias pattern.
   - `TestDecoderExportTypeAliasConsumesSlot` — mirrors `wasmtime/tests/all/component_model/bindgen.rs:424` instance-export-alias pattern.
   - `TestComponentResolveTypeDefWalksAlias` — unit test of the chain-walking helper.
8. Design doc Decision 5 + Decoder → Linker Indirection section updated to reflect the densified invariant, new `TypeDef` shape, `AliasTarget` struct, and `ResolveTypeDef` caller contract. wasmparser parallel cited.

### Consolidated spec-compliance reviewer

**Verdict:** ✅ BOTH COMMITS SPEC COMPLIANT.

17/17 checklist items pass. Two LOW-severity doc nits flagged (both addressed in polish commit `621ada86`):
- `AliasTarget` doc comment cited `Binary.md:118` with tag `0x01` but outer alias is tag `0x02` per `Binary.md:121`.
- `TestComponentResolveTypeDefWalksAlias` cited `Binary.md:263-265` (typebound prose) instead of `Binary.md:118-122` (aliastarget grammar).

### Consolidated code-quality reviewer

**Verdict:** APPROVED_WITH_MINOR → APPROVED after polish commit.

Blocking finding (addressed in `621ada86`): gofmt violation at `component.go:278-279` (double blank line after `ResolveTypeDef`).

Low-priority suggestions tracked for Session 2 followup:
- Add error-path coverage for `ResolveTypeDef` (out-of-range, cycle, nil-target, export-deferred, cross-scope-deferred) — currently only happy path tested.
- Add `unknown typebound tag` negative test for `import.go`.
- Move whole-component densification invariant from `DecodeComponent` into `decodeComponentInto` so nested-component decode paths inherit it.
- Consider unifying `AliasTarget` and `component.Alias` (duplicated payload fields) via embedding when nested components land in Checkpoint D.
- Consider stack-sized visited buffer (`var visited [MaxAliasDepth]bool`) instead of `make(map[uint32]bool)` in `ResolveTypeDef` for hot-path canon callers.

### Checkpoint C unblocked

Both prerequisites resolved. Proceeding to Task C1.

---

(Review log entries for Tasks C1–C8 were not recorded in this file during
their original execution; the commits `d398fa09..d9fb7516` and their
per-task dual reviews landed on 2026-04-09 alongside the Decision 6
revision. Entries resume at C9 below.)

---

## Task C9 — Restore `linker_test.go` (34 tests)

**Base commit:** `d9fb7516`
**Task commits (4):**
- `33c622ef` — 5 tests: `TestNewLinker` + `TestLinker_Define{Func,Func_Duplicate,Instance,Resource}`
- `50ce6c96` — 10 tests: `TestLinker_DefineResource_Duplicate` + `Get_{Direct,NotFound,Instance}` + `MatchImport_{OldImportNewItem,NewImportOldItem,SelectsMax,DirectMatch}` + `Instantiate_{Basic,WithImports}`
- `0c62cfab` — 10 tests: `Instantiate_MissingImport` + `GetExportedFunc{,_NotFound,_ExportOldGetNew,_ExportNewGetOld,_SelectsMax}` + `RelaxedSemverMatching_{FuncImport,InstanceImport,DifferentMinor,Post1_0}`
- `226d92f6` — 9 tests: `MatchLockedDep{,_NotFound}` + `MatchUnlockedDep{,_MatchAll,_NoMatch}` + `MatchURLImport` + `MatchHashImport` + `MatchPlainImport` + `MatchInterfaceImport_Unchanged`; plus deletion of dead `linkerTestSkipMsg` const and Session 0 stub header.

**Files changed:** `internal/component/linker_test.go` only (+895/-75). Pre-Session-0 body pulled from `98b3bbc3:internal/component/linker_test.go` via `/tmp/old_linker_test.go`.

**Mechanical adaptations applied to every test:**
- Dropped the registration-time `*FuncType` parameter from `DefineFunc(ns, name, funcType, fn)` → `DefineFunc(ns, name, fn)` per the wasmtime func_new dynamic-host model (Decision 6).
- Dropped `Component.Types: []TypeDef{{Kind: TypeDefKindFunc, Func: funcType}}` literals; `Component.Types` is now `*types.ComponentTypes` pointer and the legacy `Linker.Instantiate` path doesn't read Types (verified against `linker.go:483-489, :492-496`).
- Updated `FuncDef.Callback(ctx, nil)` → `FuncDef.Callback(ctx, nil, nil)` to match the new 3-arg `HostFunc` signature (`func(ctx, fnType, args)`).
- `InstanceBuilder.FuncNoType` → `InstanceBuilder.Func` (the post-Session-0 `Func` IS the no-type variant; `FuncNoType` does not exist in the current API).

### Spec-compliance reviewer

**Verdict:** ✅ SPEC COMPLIANT.

All 15 Session 1 amended-checklist items pass:
1. Every test cites spec / wasmtime ref.
2. Every citation block is within 15 lines of its `func Test...` declaration.
3. Random spot-check of 5 citations (plus ~12 additional) — all resolved against actual wasmtime files at cited line ranges:
   - `TestNewLinker` → `linker.rs:61-68` (struct Linker with NameMap<usize, Definition> field) ✓
   - `TestLinker_DefineFunc` → `linker.rs:665-675` (LinkerInstance::func_new dynamic host path) ✓
   - `TestLinker_MatchImport_SelectsMax` → `tests/all/component_model/linker.rs:81` (missing_import_selects_max) ✓
   - `TestInstance_GetExportedFunc_ExportOldGetNew` → `tests/all/component_model/instance.rs:66` (export_old_get_new) ✓
   - `TestLinker_MatchImport_DirectMatch` → `environ/src/component/names.rs:105-117` (NameMap::get) ✓
4. Assertions cross-checked against current wazero linker behavior (`linker.go:104-498`).
5. No definitions.py / wasmtime contradictions.
6. Zero TODO/FIXME/panic/return-default-on-error.
7. No parallel paths to `internal/component/abi/`.
8. Dedup: 34 uniquely-named tests.
9. No Session 2 deferrals in this file.
10. Honored "no parallel paths" / "spec over preservation" (edited in place).
11. V4 grep: `PASS: all Test functions have citation blocks`.
12. Tests pass: `go test ./internal/component/ -run TestLinker` ok; `-run TestInstance_GetExportedFunc` ok; full package green.
13. `linkerTestSkipMsg` const fully deleted.
14. Session 0 stub header deleted.
15. `grep -c '^func Test' linker_test.go` = 34.

### Code quality reviewer (superpowers:code-reviewer)

**Verdict:** APPROVED. No CRITICAL or IMPORTANT findings.

**Strengths:**
1. File is pure test body — 34 top-level functions, 0 helpers, 0 stub artifacts.
2. Citation discipline uniform: narrative + `Wasmtime parallel:` with precise line ranges + `No counterpart (justified):` tail. Wazero-specific extensions (`RelaxedSemverMatching`, `matchLockedDep`, `matchUnlockedDep`) explicitly identify the divergence and name the production source.
3. Selection assertions use callback invocation (`TestLinker_MatchImport_SelectsMax` asserts `results[0].S32() == 102`; `TestInstance_GetExportedFunc_SelectsMax` cross-checks `fn.name`).
4. Strict-then-relaxed pattern in `TestLinker_RelaxedSemverMatching_FuncImport` exemplary: same linker instance verifies strict rejection AND relaxed acceptance.
5. Test isolation perfect: every test calls `NewLinker()` at top; no state leaks.
6. Loop-closure safety: Go 1.24 per-iteration loop-var scoping; no capture bugs.
7. Correct pinning to legacy `Linker.Instantiate` path via inline comments citing `linker.go:492-496`.

**Minor findings (all non-blocking, none require corrective):**
- M1: `TestLinker_MatchUnlockedDep` / `_MatchAll` use `require.NotNil(t, def)` only, don't independently verify which version was selected. **Pre-existing weakness** carried forward from `98b3bbc3`. Restoration fidelity preserved. Defensible as-is.
- M2: `TestInstance_GetExportedFunc_SelectsMax` reads unexported `fn.name` field. Public `Name()` exists at `instance.go:175`; stylistic preference.
- M3: `TestLinker_Instantiate_WithImports` only asserts `NotNil(inst)` — the production path (`linker.go:488`) doesn't actually store resolved imports yet; test is correctly pinned to what the legacy path can prove. Flag-for-later when `linker.go:488` is filled in.
- M4: `RelaxedSemverMatching_DifferentMinor` / `_Post1_0` lack strict-mode baseline that `_FuncImport` has. Stylistic.

### Correctives

None. No CRITICAL or IMPORTANT findings from either reviewer. Per the project's execution discipline (correctives only for CRITICAL/IMPORTANT), M1-M4 are tracked here for a future cleanup pass but do not block Task C10.

### Verification at task close

```
go test ./internal/component/ -run TestLinker -count=1            ✅ ok
go test ./internal/component/ -run TestInstance_GetExportedFunc   ✅ ok
go test ./internal/component/... -count=1                         ✅ green
go build ./internal/component/...                                 ✅ clean
go vet ./internal/component/...                                   ✅ clean
git status --porcelain                                            ✅ only .env/.envrc
grep -c 't\.Skip' internal/component/linker_test.go               ✅ 0
grep -c '^func Test' internal/component/linker_test.go            ✅ 34
```

### Task status

✅ Complete. Proceeding to Task C10.

---

## Task C10 — Restore `linker_api_test.go` (8 tests)

**Base commit:** `098658c2`
**Task commit:** `43c6ca53` (single commit, "component: Task C10 — restore 8 linker_api_test.go tests")
**Files changed:** `internal/component/linker_api_test.go` only (+276/-16).

**Tests restored:**
1. `TestComponentInstanceWrapper` — api-adapter `ExportedFunction` lookup (hit + miss).
2. `TestComponentInstanceWrapper_ExportedInstance` — nested-instance lookup via `GetExportedInstance`.
3. `TestComponentInstanceWrapper_NilInstance` — nil-safety on both `ExportedFunction` and `ExportedInstance`.
4. `TestComponentWrapper_Close_NilInstance` — nil-instance fast path.
5. `TestComponentWrapper_Close_EmptyCoreInstances` — empty coreInstances loop + nil-out.
6. `TestComponentWrapper_Close_ClosesCoreModules` — closes every mock module + nil-out.
7. `TestComponentWrapper_Close_ReturnsFirstError` — first-error-wins; all modules still closed; nil-out.
8. `TestComponentWrapper_Close_DoubleClose` — idempotent double-close.

**Test helper:** `closeMockModule` copied verbatim from pre-Session-0 snapshot; matches current `api.Module` interface at `api/wasm.go:143-217` via `internalapi.WazeroOnlyType` embedding.

**Instance construction choice:** `NewInstance(nil, 0, nil)` (Session 1 Decision 3 / Task B4) for the 6 tests that need a non-nil instance; bare `&ComponentInstanceWrapper{instance: nil}` / `&ComponentWrapper{instance: nil}` literals for the 2 nil-safety tests where the nil pointer IS the test subject. `newInstance` at `instance.go:89-102` is safe with `component == nil` and `parent == nil`; pre-allocates `exports` / `coreInstances` / `componentFuncs` (but NOT `exportedInstances`, which the one affected test assigns itself).

### Spec-compliance reviewer

**Verdict:** ✅ SPEC COMPLIANT.

All 16 Session 1 amended-checklist items pass. Random spot-checks of all 3 distinct wasmtime citations opened and verified:
- `instance.rs:156` — `pub fn get_func` ✓
- `instance.rs:290` — `pub fn get_export` ✓
- `linker.rs:61` — `pub struct Linker<T: 'static>` ✓

V4 grep passes. `linkerAPITestSkipMsg` const fully deleted. Session 0 stub header deleted. Dedup clean (8 unique test names). All 8 tests pass. `closeMockModule` implements every method of `api.Module` (`String`, `Name`, `Memory`, `ExportedFunction`, `ExportedFunctionDefinitions`, `ExportedMemory`, `ExportedMemoryDefinitions`, `ExportedGlobal`, `CloseWithExitCode`, `Close`, `IsClosed` + `WazeroOnly` sentinel). `go vet` clean. No findings.

### Code quality reviewer (superpowers:code-reviewer)

**Verdict:** APPROVED. No CRITICAL or IMPORTANT findings.

**Strengths:**
1. Clean dead-code removal; no vestigial symbols.
2. `closeMockModule` minimal and correct; `Close`/`CloseWithExitCode`/`IsClosed` internally consistent.
3. Citation uniformity excellent: identical 3-line schema across all 8 tests.
4. Per-test justifications specific (not copy-pasted).
5. `NewInstance` vs bare-literal choice defensible and documented via inline comments.
6. Production-path fidelity cross-verified against `linker_api.go:177-193, :230-251`.
7. Test isolation solid: no shared state, no loop-var capture, no `t.Parallel`.
8. All 8 tests pass; `go vet` clean.

**Minor findings (non-blocking, no corrective required):**
- M1: `TestComponentWrapper_Close_EmptyCoreInstances` reassigns `inst.coreInstances = []api.Module{}` which is a no-op since `NewInstance` already pre-allocates. The inline comment acknowledges this as a defensive pin; stylistic.
- M2: `TestComponentWrapper_Close_ReturnsFirstError` uses `err != expectedErr` identity compare. Works today because `Close` doesn't wrap, but `errors.Is` would be more future-proof.
- M3: `closeMockModule.String()` / `Name()` are interface-satisfaction-only and unexercised. Acceptable.
- M4: No context-cancellation test — out of scope for C10.

**Assertion strength table:** reviewer walked each test against its production branch in `linker_api.go`; all 8 tests verify what their names and comments claim.

### Correctives

None. No CRITICAL or IMPORTANT findings from either reviewer. M1-M4 tracked here for future cleanup; do not block Task C11.

### Verification at task close

```
go test ./internal/component/ -run TestComponent -count=1           ✅ ok
go test ./internal/component/... -count=1                           ✅ green
go build ./internal/component/...                                   ✅ clean
go vet ./internal/component/...                                     ✅ clean
git status --porcelain                                              ✅ only .env/.envrc
grep -c 't\.Skip' internal/component/linker_api_test.go             ✅ 0
grep -c '^func Test' internal/component/linker_api_test.go          ✅ 8
grep -c 'linkerAPITestSkipMsg\|SESSION 0' internal/component/linker_api_test.go  ✅ 0
```

### Task status

✅ Complete. Proceeding to Task C11.

---

## Task C11 — Restore `conformance/primitives_test.go` citations (audit-only)

**Base commit:** `466431a6`
**Task commit:** `29abc0d7` (single commit, "component: Task C11 — conformance/primitives_test.go citations + missing test_pairs/test_nan cases")
**Files changed:** `internal/component/conformance/primitives_test.go` only (+406/-15, 897 → 1288 lines).

**Citation coverage (7/7 top-level tests):**
- `TestPrimitivesIntegers` → canonical (`run_tests.py:185-196` + `definitions.py:1706-1708, 1797-1808`)
- `TestPrimitivesFloats` → canonical (`run_tests.py:197-198, 231-244` + `definitions.py:1210-1211, 1213-1223, 1395-1411, 1783-1784`)
- `TestPrimitivesBools` → canonical (`run_tests.py:184` + `definitions.py:1205-1207, 1774`)
- `TestPrimitivesChars` → canonical (`run_tests.py:199-200` + `definitions.py:1237-1241, 1785`)
- `TestPrimitivesSignExtension` → canonical (`run_tests.py:187-196` + `definitions.py:1802-1808, 1891-1894`)
- `TestPrimitivesTypeProperties` → "No counterpart (justified): wazero-specific ABI accessor test"
- `TestPrimitivesAllTypesRoundtrip` → "No counterpart (justified): dispatch smoke test"

**New `spec_test_*` subtests (13 total):**
- `spec_test_pairs_bool` (`run_tests.py:184`): (0,false),(1,true),(2,true),(4294967295,true).
- `spec_test_pairs_char` (`run_tests.py:199-200`): 0/65/0xD7FF/0xE000/0x10FFFF valid; 0xD800/0xDFFF/0x110000/0xFFFFFFFF reject. The 0xFFFFFFFF case was previously uncovered.
- `spec_test_pairs_{u8,s8,u16,s16,u32,s32,u64,s64}` (`run_tests.py:185-196`): 8 subtests × 3-7 canonical truncation/sign-fold pairs each.
- `spec_test_nan32` (`run_tests.py:231-237`): 5 canonicalize patterns → `0x7fc00000`; +Inf preserved; 1.5 preserved.
- `spec_test_nan64` (`run_tests.py:238-244`): analogous 64-bit patterns canonicalizing to `0x7ff8000000000000`.

**Cases judged already-covered** (cited via comments, not duplicated):
- Char valid boundary values (0xD7FF, 0xE000, 0x10FFFF) — already in `TestPrimitivesChars/boundary_values`.
- Char surrogate rejection 0xD800-0xDFFF — already in `TestPrimitivesChars/surrogate_rejection`.
- Float infinity preservation — pinned by new `spec_test_nan32/64` cases.
- Sign-extension per-type — already in `TestPrimitivesSignExtension`.

**Case skipped with justification:**
- `EnumType(['a','b'])` at `run_tests.py:201` — enums are composite (despecialized to `VariantType` per `definitions.py:1029`); out-of-scope for primitives_test.go. Deferred to `composites_test.go` task.

**NaN canonicalization status:**
Wazero implements DETERMINISTIC_PROFILE lift-path canonicalization. `internal/component/abi/context.go:25-47` defines `CanonicalFloat32NaN = 0x7fc00000` and `CanonicalFloat64NaN = 0x7ff8000000000000`, matching `definitions.py:1210-1211` exactly. Both `LiftFlat` (`lift.go:74-76`) and `LiftHeap` (`lift.go:376, 382`) call `canonicalizeNaN32/64` unconditionally. New tests assert bit-exact canonicalization on the lift path only.

**Deferred-out-of-scope concern (flagged for Session 2 followup):**
`internal/component/abi/lower.go:37-40` does NOT canonicalize NaN on the lower path, diverging from `definitions.py:1877-1878 + :1395-1411` under the deterministic profile. Audit-only C11 correctly deferred this to a future production-code task. Do NOT land a TODO — track it in the Session 2 followup note instead.

### Spec-compliance reviewer

**Verdict:** ✅ SPEC COMPLIANT.

All 17 Session 1 amended-checklist items pass. Nine citation spot-checks opened and verified within zero-line tolerance:
- `run_tests.py:184` (BoolType), `:185-196` (integer tuples), `:199-200` (char tuples), `:231-244` (nan32/64).
- `definitions.py:1205-1241` (bool/char/float classes + canonicalize_nan32/64), `:1395-1411` (maybe_scramble_nan32/64), `:1706-1711` (flatten_type), `:1774-1808` (lift_flat + signed/unsigned), `:1866-1894` (lower_flat).

Five canonical arithmetic spot-checks traced pair-by-pair:
- `U8Type (4294967168, 128)`: `0xFFFFFF80 mod 256 = 128` ✓
- `S8Type (128, -128)`: `128 mod 256 = 128; 128 >= 128 → 128 - 256 = -128` ✓
- `S16Type (4294934528, -32768)`: `0xFFFF8000 mod 65536 = 32768; 32768 >= 32768 → -32768` ✓
- `S32Type ((1<<32)-1, -1)`: `0xFFFFFFFF >= 0x80000000 → -1` ✓
- `S64Type (1<<63, MinInt64)`: `1<<63 >= 1<<63 → -(1<<63) = MinInt64` ✓

V4 grep PASS. NaN lift canonicalization verified at `lift.go:74-76`. LowerFlat NaN concern flagged LOW (out-of-scope for C11 audit-only).

### Code quality reviewer (superpowers:code-reviewer)

**Verdict:** APPROVE for merge. No CRITICAL or IMPORTANT findings.

**Strengths:**
1. Honest overlap disclosure — `spec_test_pairs_bool`/`spec_test_pairs_char` explicitly document overlap with pre-existing subtests and justify retention ("pins the full spec pair list in one place").
2. Uniform citation blocks: "Canonical test:" line + "Spec:" line, consistent across all 7 + 13 subtests.
3. Subtest design type-appropriate: flat `for _, c := range cases` for integer tables (compact), nested `t.Run(c.name, ...)` for NaN/char (semantic names worth surfacing).
4. Typed `expect` field in each integer table: compiler enforces sign-fold correctness at build time.
5. Magic numbers documented inline; large constants use Go expression syntax (`1<<63`, `(1<<32)-1`) matching `run_tests.py` expressions.
6. NaN assertions use `math.Float32bits`/`Float64bits` bit-compare (can't use `==` on NaN).
7. Loop-var capture safe under Go 1.22+ per-iteration semantics (project on go1.25).
8. No helper-function proliferation: 8 integer tables kept self-contained (extracting a generic would obscure the type contract).
9. No production code touched (verified via diff stat).

**Minor findings (non-blocking, no corrective required):**
- M1: Comment at lines 753-756 doesn't explicitly justify retaining `lift_nonzero_as_true` alongside `spec_test_pairs_bool`. One-line clarification would help; stylistic.
- M2: `TestPrimitivesTypeProperties` cites `definitions.py:1706-1711` twice; the second reference was likely intended to point at per-type `elem_size`/alignment formulas elsewhere. Cosmetic citation bug; spec-substance already verified.
- M3: Integer `spec_test_pairs_*` subtests could use per-case `t.Run(c.name, ...)` for nicer `-v` output (NaN/char tests already do). Stylistic inconsistency; defensible given integer table density.
- M4: `require.Equal(..., "... expected %d", c.expect)` duplicates expected-value rendering that the framework already emits. Minor verbosity.

**File growth math verified:** 897 → 1288 lines (+391 net). ~80 lines for expanded citations, ~145 for 8 integer subtests, ~56 for 2 NaN subtests, ~22 for bool, ~42 for char, ~45 for mid-function comments/spacing. Every line justified.

### Correctives

None. No CRITICAL or IMPORTANT findings from either reviewer. M1-M4 tracked here for a future cleanup pass; do not block Task C12.

**LOW finding tracked for Session 2:** `lower.go:37-40` NaN canonicalization divergence from `definitions.py:1877-1878 + :1395-1411` under deterministic profile.

### Verification at task close

```
go test ./internal/component/conformance/ -run TestPrimitives -count=1   ✅ ok
go test ./internal/component/... -count=1                                ✅ green
go build ./internal/component/...                                        ✅ clean
go vet ./internal/component/...                                          ✅ clean
git status --porcelain                                                   ✅ only .env/.envrc
V4 grep                                                                  ✅ PASS (7/7)
git diff --stat 466431a6..29abc0d7                                       ✅ 1 file: primitives_test.go
```

### Task status

✅ Complete. Proceeding to Task C12.

---

## Task C12 — Restore `conformance/may_leave_test.go` citations (audit-only)

**Base commit:** `46f1f549`
**Task commit:** `7ba764db` (single, "component: Task C12 — may_leave_test.go citation audit")
**Files changed:** `internal/component/conformance/may_leave_test.go` only (+90/-2).

**Citation coverage (9/9 top-level tests):**
- `TestInstance_MayLeaveDefaultTrue` → `definitions.py:256-273 + :270`
- `TestInstance_SetMayLeave` → `definitions.py:1955/:1973` (lower_flat_values toggle)
- `TestInstance_MayLeaveFalseDuringLowering` → `definitions.py:1955/:1973`
- `TestInstance_MayLeaveFalseDuringPostReturn` → `definitions.py:1999-2002` + `CanonicalABI.md:3286-3297` (post-return block + rationale; extended from the pre-audit `:3287-3289` range to include the rationale prose)
- `TestInstance_ValidateMayLeave` → `definitions.py:2065, :2135, :2143` (three trap_if sites) + Session 1 B3 sentinel rename
- `TestInstance_ValidateMayLeaveNilInstance` → `Spec: definitions.py:2065` + "No counterpart (justified): wazero nil-receiver concern"
- `TestMayLeave_MultipleSetCycles` → `Spec: definitions.py:1955/:1973` + "No counterpart (justified): wazero accessor robustness"
- `TestMayLeave_ValidationDuringLowering` → `definitions.py:2065` + Session 1 B3 sentinel rename
- `TestMayLeave_ConcurrentAccess` → `Spec: definitions.py:256-273` single-threaded model + "No counterpart (justified): wazero accessor concurrency safety"

**Test semantic rewrites:** Zero. All 9 tests already use Session 1 Task B1 spec-correct semantics (standalone boolean, decoupled from `enterCount`). No pre-Session-0 buggy behavior assertions present.

**Production cross-check (verified):**
- `instance.go:246` → `MayLeave()`
- `instance.go:250` → `SetMayLeave()`
- `instance.go:324-332` → `ValidateMayLeave()` (nil-guarded)
- `instance.go:345` → `errMayNotLeave = "component instance cannot leave (may_leave=false)"` (matches `require.Contains` assertions)

### Spec-compliance reviewer

**Verdict:** ✅ SPEC COMPLIANT.

All 13 Session 1 amended-checklist items pass. Nine citation spot-checks opened and verified character-accurate:
- `definitions.py:260` → `  may_leave: bool` field inside `class ComponentInstance` ✓
- `definitions.py:270` → `    self.may_leave = True` default in __init__ ✓
- `definitions.py:1955` → `  cx.inst.may_leave = False` at entry of `lower_flat_values` ✓
- `definitions.py:1973` → `  cx.inst.may_leave = True` at exit of `lower_flat_values` ✓
- `definitions.py:1999-2002` → post-return `inst.may_leave = False / True` toggle in `canon_lift` ✓
- `definitions.py:2065` → `  trap_if(not thread.task.inst.may_leave)` in `canon_lower` ✓
- `definitions.py:2135` → same trap in `canon_resource_new` ✓
- `definitions.py:2143` → same trap in `canon_resource_drop` ✓
- `CanonicalABI.md:3286-3297` → post-return `may_leave` toggle block (3286-3291) + rationale prose (3293-3297) ✓

No test asserts pre-B1 buggy semantics (all `enterCount` mentions are in citation prose documenting the decoupling). No production code modified. V4 grep PASS. Build + vet clean.

### Code quality reviewer (superpowers:code-reviewer)

**Verdict:** APPROVE. No CRITICAL or IMPORTANT findings.

**Strengths:**
1. Consistent 3-part citation-block template across all 9 tests.
2. Line-accurate narrow citations (e.g., `:1955` for entry toggle, `:1973` for exit toggle) rather than whole-function citations.
3. `CanonicalABI.md` reference pinned to specific HEAD (`verified on HEAD 46f1f549`) — strongest form of citation discipline.
4. Fixed a stale in-body `CanonicalABI.md:3287-3289` reference to match the new header-block `:3286-3297` range — header and body in sync.
5. "No counterpart (justified) + Spec:" hybrid blocks correctly anchor to nearest spec concept BEFORE the justification.
6. No production-code churn, no parallel paths, no TODOs.
7. File growth (+90/-2 for 9 tests, ~10 lines per test) is proportionate.

**Minor suggestions (non-blocking, no corrective required):**
- S1: Minor inconsistency in `Spec:` line-number separator — slash (`:1955/:1973`) vs comma (`:1955, :1973`). Plan-level convention worth documenting.
- S2: `TestInstance_ValidateMayLeave` and `TestMayLeave_ValidationDuringLowering` each duplicate the "Session 1 B3 renamed the sentinel" comment in both header and body. Pure style.
- S3: Em-dash vs ASCII hyphen consistency in citation prose. Optional.

### Correctives

None. No CRITICAL or IMPORTANT findings. S1-S3 are stylistic polish, tracked for a future convention pass across C10-C18.

### Verification at task close

```
go test ./internal/component/conformance/ -run MayLeave -count=1   ✅ ok
go test ./internal/component/... -count=1                          ✅ green
go build ./internal/component/...                                  ✅ clean
go vet ./internal/component/...                                    ✅ clean
git status --porcelain                                             ✅ only .env/.envrc
V4 grep                                                            ✅ PASS (9/9)
git diff --stat 46f1f549..7ba764db                                 ✅ 1 file: may_leave_test.go
```

### Task status

✅ Complete. Proceeding to Task C13.

