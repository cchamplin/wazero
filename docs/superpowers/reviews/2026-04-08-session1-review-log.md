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

