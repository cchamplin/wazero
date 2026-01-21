# Canonical ABI Conformance Fixes - Execution Prompt

> **For Claude:** Use this prompt to start a sub-agent driven development session.

---

## Task

Execute the Canonical ABI conformance fixes implementation plan using the **superpowers:subagent-driven-development** skill. This plan brings wazero's Canonical ABI type lifting and lowering implementation into full conformance with the Component Model specification.

---

## Plan Location

All plan documents are in this directory:
- **Overview & Progress:** `docs/plans/spec-canonical-abi-conformance-fixes/00-overview.md`
- **Phase 1 (Critical):** `docs/plans/spec-canonical-abi-conformance-fixes/01-phase1-critical-fixes.md`
- **Phase 2 (Major):** `docs/plans/spec-canonical-abi-conformance-fixes/02-phase2-major-improvements.md`
- **Phase 3 (Async):** `docs/plans/spec-canonical-abi-conformance-fixes/03-phase3-async-support.md` (DEFERRED)

Start with Phase 1, then Phase 2. Phase 3 is deferred.

---

## Key Context

### Codebase Structure

| Component | File |
|-----------|------|
| Lift operations | `internal/component/abi/lift.go` |
| Lower operations | `internal/component/abi/lower.go` |
| Flattening | `internal/component/abi/flatten.go` |
| String handling | `internal/component/abi/strings.go` |
| Context types | `internal/component/abi/context.go` |
| Primitive types | `internal/component/types/types.go` |
| Composite types | `internal/component/types/composite.go` |
| Resource types | `internal/component/types/resource.go` |
| Tests | `internal/component/abi/*_test.go` |

### Reference Materials

| Resource | Path | Purpose |
|----------|------|---------|
| Primary Spec | `debug-vendored/component-model/design/mvp/CanonicalABI.md` | Authoritative specification |
| Python Reference | `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py` | Executable reference implementation |
| Wasmtime component-util | `debug-vendored/wasmtime/crates/component-util/src/lib.rs` | Alignment/size utilities |
| Wasmtime values.rs | `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/values.rs` | Value lifting/lowering reference |
| Gap Analysis | `docs/plans/canonical-abi-gap-analysis.md` | Detailed defect analysis |

---

## Critical Requirements

### Regression Tests

**MUST pass at the end of each phase:**

```bash
CGO_ENABLED=0 go test ./internal/component/wasip2test/... -run "TestCalculatorPlugins/(add|subtract)" -v
```

Both `add` and `subtract` tests MUST pass. If they fail, you've broken something - investigate and fix before continuing.

These tests exercise:
- **add plugin** (Rust): `evaluate(28, 3) = 31`, returns string "add"
- **subtract plugin** (C): `evaluate(28, 3) = 25`, returns string "subtract"
- **multi plugin** (Go): `evaluate(28, 3) = 84`, returns string "Simple-Go-Multi"
- **div plugin** (Go): `evaluate(28, 3) = 9`, returns string "Simple-Go-Div"

### Build Requirement

Always use `CGO_ENABLED=0` when running tests to avoid CGO compiler issues.

### TDD Workflow

Each task follows this pattern:
1. Write failing test
2. Verify it fails
3. Implement minimal fix
4. Verify test passes
5. Run full suite for that package
6. Commit

---

## Execution Instructions

1. **Invoke the skill:**
   ```
   Use superpowers:subagent-driven-development to execute the plan
   ```

2. **Start with Phase 1:** Read `01-phase1-critical-fixes.md` and execute tasks 1.1 through 1.7 in order.

3. **After Phase 1 completion:** Run regression tests, then proceed to Phase 2.

4. **Track progress:** Update checkboxes in `00-overview.md` as tasks complete.

5. **Commit frequently:** Each task should have its own commit with a descriptive message.

---

## Phase 1 Tasks Summary (7 tasks)

| Task | Description | Key Files |
|------|-------------|-----------|
| 1.1 | Float NaN Canonicalization | `lift.go`, `lower.go` |
| 1.2 | String Alignment Validation | `strings.go` |
| 1.3 | List Element Alignment Validation | `lift.go` |
| 1.4 | Variant Flatten Join Semantics | `flatten.go` |
| 1.5 | Variant Lift Type Coercion | `lift.go` |
| 1.6 | Variant Lower Type Coercion | `lower.go` |
| 1.7 | Resource Type Validation | `lift.go`, `lower.go` |

## Phase 2 Tasks Summary (5 tasks)

| Task | Description | Key Files |
|------|-------------|-----------|
| 2.1 | Fixed-Length List Type Support | `types/composite.go` |
| 2.2 | Fixed-Length List Lifting | `lift.go`, `flatten.go` |
| 2.3 | Fixed-Length List Lowering | `lower.go` |
| 2.4 | Empty Type Prohibition | `types/composite.go` |
| 2.5 | Borrow Optimization (TODO) | `lower.go`, `context.go` |

---

## Notes

- The plan documents contain complete code snippets - use them as guidance
- Spec line references are provided for each fix - consult the spec when unclear
- Some tasks (like 2.5 Borrow Optimization) are documented as TODOs for future work
- Phase 3 is deferred - do not implement unless explicitly requested
