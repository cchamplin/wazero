# Canon Lift/Lower Conformance Fixes - Execution Prompt

Use this prompt to start a subagent-driven development session for implementing the conformance fixes.

---

## Prompt

```
I need to implement conformance fixes for the wazero Component Model canon lift/lower implementation. There is a comprehensive plan ready for execution.

## Plan Location
All plan documents are in: `docs/plans/spec-canon-lift-lower-conformance-fixes/`

- `00-root-plan.md` - Master plan with progress tracking
- `01-may-leave-flag.md` - Phase 1: may_leave flag implementation
- `02-reentrance-guard.md` - Phase 2: Reentrance guard implementation
- `03-subtask-management.md` - Phase 3: Subtask management
- `04-parameter-spilling.md` - Phase 4: Parameter spilling (MAX_FLAT_PARAMS)
- `05-alignment-validation.md` - Phase 5: Alignment validation

## Gap Analysis
The source analysis is at: `docs/plans/spec-canon-lift-lower-gap-analysis.md`

## Reference Materials
These are available for understanding spec requirements:

**Specifications:**
- `debug-vendored/component-model/design/mvp/CanonicalABI.md` - Primary spec
- `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py` - Reference Python

**Wasmtime Reference:**
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/func/` - Canon lift/lower impl
- `debug-vendored/wasmtime/tests/all/component_model/func.rs` - Function tests

## Key Implementation Files
- `internal/component/instance.go` - Instance struct, Call method
- `internal/component/component_linker.go` - Instantiation, wiring
- `internal/component/abi/context.go` - LiftContext, LowerContext
- `internal/component/abi/lift.go` - Lifting operations
- `internal/component/abi/lower.go` - Lowering operations
- `internal/component/abi/flatten.go` - Type flattening
- `internal/component/resources.go` - Resource table, BorrowScope

## CRITICAL Regression Requirement
After completing EACH PHASE (not each task), run:
```bash
go test -v ./internal/component/wasip2test/... -run "TestAdd|TestSubtract"
```
Both tests MUST pass before proceeding to the next phase.

## Execution Instructions
1. Use the superpowers:subagent-driven-development skill
2. Start with Phase 1 (01-may-leave-flag.md)
3. Execute tasks in order within each phase
4. Each task follows TDD: failing test → implement → passing test → commit
5. After all tasks in a phase, run regression check
6. Update progress in 00-root-plan.md after each phase
7. Proceed to next phase only after regression passes

## Phase Priority
- Phase 1 & 2: P0 CRITICAL (must fix)
- Phase 3 & 4: P1 HIGH (should fix)
- Phase 5: P3 LOW (nice to have)

Start by reading 00-root-plan.md to understand the overall structure, then begin executing Phase 1.
```

---

## Quick Start Commands

```bash
# Navigate to the repo
cd /Users/cchamplin/go/src/github.com/cchamplin/wazero

# Read the master plan
cat docs/plans/spec-canon-lift-lower-conformance-fixes/00-root-plan.md

# Read Phase 1
cat docs/plans/spec-canon-lift-lower-conformance-fixes/01-may-leave-flag.md

# Run regression tests (do this after each phase)
go test -v ./internal/component/wasip2test/... -run "TestAdd|TestSubtract"

# Run all component tests
go test -v ./internal/component/... -short
```

---

## Context Summary

### What We're Fixing

The wazero Component Model implementation has these critical gaps:

1. **may_leave flag missing** - Components can call out during lowering/post-return (should trap)
2. **No reentrance guard** - Recursive calls into same component allowed (should trap)
3. **No Subtask tracking** - Borrow lifetimes not properly scoped to calls
4. **No param spilling** - Functions with >16 flat params will fail
5. **No alignment validation** - Misaligned memory access not detected

### What's Already Working

- Canonical options parsing (string-encoding, memory, realloc, post-return)
- Post-return invocation
- String encoding (UTF-8, UTF-16, Latin1+UTF16)
- Memory bounds checking
- Result spilling (MAX_FLAT_RESULTS = 1)
- Realloc parameter passing
- Resource handle basics
- Type flattening

### Files That Will Be Modified

| Phase | Files |
|-------|-------|
| 1 | `instance.go`, `conformance/may_leave_test.go` (new) |
| 2 | `instance.go`, `conformance/reentrance_test.go` (new) |
| 3 | `subtask.go` (new), `resources.go`, `abi/context.go`, `instance.go`, `conformance/subtask_test.go` (new) |
| 4 | `abi/flatten.go`, `abi/lower.go`, `instance.go`, `conformance/param_spilling_test.go` (new) |
| 5 | `abi/context.go`, `conformance/alignment_test.go` (new) |

---

## Skill Invocation

When starting the session, invoke:

```
/skill superpowers:subagent-driven-development
```

Or use the Skill tool:

```json
{
  "skill": "superpowers:subagent-driven-development"
}
```

Then provide the prompt above.
