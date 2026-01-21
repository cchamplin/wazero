# Canon Lift/Lower Conformance Fixes - Master Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan phase-by-phase.

**Goal:** Bring wazero's Component Model canon lift/lower implementation into full compliance with the official Component Model specification.

**Architecture:** This plan implements the missing runtime safety mechanisms (may_leave flag, reentrance guards), completes the calling convention (parameter spilling), and adds proper call tracking (Subtask management). Each phase is independent and can be validated against the calculator test suite.

**Tech Stack:** Go 1.21+, wazero internal APIs, Component Model binary format

---

## Reference Materials

### Specifications
- **Primary Spec:** `debug-vendored/component-model/design/mvp/CanonicalABI.md`
- **Reference Python:** `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py`

### Wasmtime Reference Implementation
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/func/host.rs` - Host function handling
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/func/typed.rs` - Typed function wrappers
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/func/options.rs` - Canonical options
- `debug-vendored/wasmtime/crates/environ/src/component/dfg.rs` - Data flow graph for canonicals
- `debug-vendored/wasmtime/tests/all/component_model/func.rs` - Function call tests
- `debug-vendored/wasmtime/tests/all/component_model/post_return.rs` - Post-return tests

### Gap Analysis
- **Source Analysis:** `docs/plans/spec-canon-lift-lower-gap-analysis.md`

---

## Regression Requirement

**CRITICAL:** After completing each phase (not each task), verify the calculator tests pass:

```bash
go test -v ./internal/component/wasip2test/... -run "TestAdd|TestSubtract"
```

Both `add` and `subtract` plugins must continue working throughout all phases.

---

## Phase Overview

| Phase | Document | Description | Priority | Status |
|-------|----------|-------------|----------|--------|
| 1 | [01-may-leave-flag.md](./01-may-leave-flag.md) | Implement may_leave flag and enforcement | P0 | ⬜ TODO |
| 2 | [02-reentrance-guard.md](./02-reentrance-guard.md) | Implement call_might_be_recursive guard | P0 | ⬜ TODO |
| 3 | [03-subtask-management.md](./03-subtask-management.md) | Implement Subtask for borrow scope tracking | P1 | ⬜ TODO |
| 4 | [04-parameter-spilling.md](./04-parameter-spilling.md) | Implement MAX_FLAT_PARAMS spilling | P1 | ⬜ TODO |
| 5 | [05-alignment-validation.md](./05-alignment-validation.md) | Add alignment assertion on load/store | P3 | ⬜ TODO |

---

## Gap-to-Phase Mapping

| Gap ID | Gap Description | Addressed In |
|--------|----------------|--------------|
| GAP-LEAVE-1 | may_leave flag missing | Phase 1 |
| GAP-LIFT-2 | may_leave not set during post-return | Phase 1 |
| GAP-LOWER-1 | may_leave not checked in canon_lower | Phase 1 |
| GAP-LIFT-1 | No reentrance guard | Phase 2 |
| GAP-STACK-1 | No call stack tracking | Phase 2 |
| GAP-LOWER-2 | No Subtask tracking | Phase 3 |
| GAP-CTX-2 | Borrow scope not in context | Phase 3 |
| GAP-CALL-1 | Parameter spilling not implemented | Phase 4 |
| GAP-LIFT-3 | MAX_FLAT_PARAMS not enforced | Phase 4 |
| GAP-BOUNDS-1 | Alignment not asserted | Phase 5 |

---

## Execution Order

Phases must be executed in order due to dependencies:

```
Phase 1 (may_leave) ─┐
                     ├──► Phase 3 (Subtask) ──► Phase 4 (Param Spilling)
Phase 2 (reentrance) ┘                              │
                                                    ▼
                                              Phase 5 (Alignment)
```

- **Phases 1 & 2:** Independent, can be done in parallel
- **Phase 3:** Depends on Phase 1 (uses may_leave in context)
- **Phase 4:** Depends on Phase 3 (uses Subtask for allocation)
- **Phase 5:** Independent, can be done anytime

---

## Progress Tracking

### Phase 1: may_leave Flag
- [ ] Task 1.1: Add mayLeave field to Instance
- [ ] Task 1.2: Implement setMayLeave helper
- [ ] Task 1.3: Set mayLeave=false during parameter lowering
- [ ] Task 1.4: Set mayLeave=false during post-return
- [ ] Task 1.5: Check mayLeave in lowered call path
- [ ] Task 1.6: Add conformance tests
- [ ] **REGRESSION CHECK**

### Phase 2: Reentrance Guard
- [ ] Task 2.1: Add call tracking fields to Instance
- [ ] Task 2.2: Implement callMightBeRecursive check
- [ ] Task 2.3: Add guard at canon_lift entry
- [ ] Task 2.4: Track caller across calls
- [ ] Task 2.5: Add conformance tests
- [ ] **REGRESSION CHECK**

### Phase 3: Subtask Management
- [ ] Task 3.1: Define Subtask struct
- [ ] Task 3.2: Create Subtask at canon_lower
- [ ] Task 3.3: Add borrow scope to Subtask
- [ ] Task 3.4: Track lends in Subtask
- [ ] Task 3.5: Deliver resolve and cleanup
- [ ] Task 3.6: Add conformance tests
- [ ] **REGRESSION CHECK**

### Phase 4: Parameter Spilling
- [ ] Task 4.1: Detect >MAX_FLAT_PARAMS condition
- [ ] Task 4.2: Implement param memory layout
- [ ] Task 4.3: Allocate param buffer via realloc
- [ ] Task 4.4: Store params to memory
- [ ] Task 4.5: Pass pointer instead of flat params
- [ ] Task 4.6: Add conformance tests
- [ ] **REGRESSION CHECK**

### Phase 5: Alignment Validation
- [ ] Task 5.1: Add alignment check to LiftContext reads
- [ ] Task 5.2: Add alignment check to LowerContext writes
- [ ] Task 5.3: Add conformance tests
- [ ] **REGRESSION CHECK**

---

## Files Modified Summary

| File | Phases | Changes |
|------|--------|---------|
| `internal/component/instance.go` | 1, 2, 4 | mayLeave, call tracking, param spilling |
| `internal/component/component_linker.go` | 2 | Reentrance check at call entry |
| `internal/component/abi/context.go` | 3, 5 | Subtask field, alignment checks |
| `internal/component/abi/lower.go` | 1, 4 | mayLeave set, param spilling |
| `internal/component/abi/lift.go` | 5 | Alignment validation |
| `internal/component/subtask.go` | 3 | New file |
| `internal/component/conformance/may_leave_test.go` | 1 | New file |
| `internal/component/conformance/reentrance_test.go` | 2 | New file |
| `internal/component/conformance/subtask_test.go` | 3 | New file |
| `internal/component/conformance/param_spilling_test.go` | 4 | New file |
| `internal/component/conformance/alignment_test.go` | 5 | New file |

---

## Quick Start

To begin implementation:

1. Read the gap analysis: `docs/plans/spec-canon-lift-lower-gap-analysis.md`
2. Start with Phase 1: `docs/plans/spec-canon-lift-lower-conformance-fixes/01-may-leave-flag.md`
3. After each phase, run regression test
4. Update progress tracking above

```bash
# Before starting any phase
git checkout -b fix/canon-conformance-phase-N

# After completing each phase
go test -v ./internal/component/wasip2test/... -run "TestAdd|TestSubtract"
git add -A && git commit -m "fix(component): phase N - description"
```
