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
