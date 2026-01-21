# Resource System Spec Conformance - Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Bring the wazero Component Model resource system into full conformance with the official Component Model specification.

**Architecture:** Incrementally add type tracking, fix trap conditions, implement borrow scope integration, and add destructor support while maintaining backward compatibility. Each phase builds on the previous and includes validation via the regression test suite.

**Tech Stack:** Go, wazero Component Model internals, WebAssembly Component Model specification

---

## Reference Materials

### Primary Specification
- **File:** `debug-vendored/component-model/design/mvp/CanonicalABI.md`
- **Key Sections:**
  - Lines 493-550: Resource State (ResourceHandle, ResourceType classes)
  - Lines 2215-2240: lift_own, lift_borrow functions
  - Lines 2673-2683: lower_own, lower_borrow functions
  - Lines 3590-3688: canon resource.new, resource.drop, resource.rep

### Wasmtime Reference Implementation
- **Resource Tables:** `debug-vendored/wasmtime/crates/wasmtime/src/runtime/vm/component/resources.rs`
- **Host Tables:** `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/resources/host_tables.rs`
- **Comprehensive Tests:** `debug-vendored/wasmtime/tests/all/component_model/resources.rs`

### Gap Analysis Document
- **File:** `docs/plans/resource-system-gap-analysis.md`
- Contains detailed comparison of spec vs current implementation
- Lists all identified gaps with priority ratings

---

## Regression Requirement

**CRITICAL:** After completing EACH PHASE, verify the following tests pass:

```bash
CGO_ENABLED=0 go test -v ./internal/component/wasip2test/... -run "TestCalculatorPlugins/add"
CGO_ENABLED=0 go test -v ./internal/component/wasip2test/... -run "TestCalculatorPlugins/subtract"
```

Both `add` and `subtract` plugin tests MUST pass before proceeding to the next phase.

---

## Phase Overview

| Phase | File | Description | Priority |
|-------|------|-------------|----------|
| 1 | [01-phase1-core-type-system.md](./01-phase1-core-type-system.md) | Add ResourceType tracking to HandleEntry | P0 - Critical |
| 2 | [02-phase2-trap-conditions.md](./02-phase2-trap-conditions.md) | Fix trap conditions in resource operations | P0 - Critical |
| 3 | [03-phase3-borrow-scope-integration.md](./03-phase3-borrow-scope-integration.md) | Integrate borrow scope with CallContext | P1 - High |
| 4 | [04-phase4-destructor-support.md](./04-phase4-destructor-support.md) | Implement destructor invocation | P1 - High |
| 5 | [05-phase5-advanced-features.md](./05-phase5-advanced-features.md) | may_leave checks, reentrance protection | P2 - Medium |

---

## Progress Tracking

### Phase 1: Core Type System
- [x] Task 1.1: Define ResourceTypeID type
- [x] Task 1.2: Add RT field to HandleEntry
- [x] Task 1.3: Update New() to accept ResourceTypeID
- [x] Task 1.4: Update CreateResourceNewFunc to store type
- [x] Task 1.5: Add type accessor methods
- [x] **REGRESSION CHECK**

### Phase 2: Trap Conditions
- [x] Task 2.1: Fix resource.drop to trap on invalid handle
- [x] Task 2.2: Fix resource.rep to trap on invalid handle
- [x] Task 2.3: Add type validation to Remove()
- [x] Task 2.4: Add type validation to Get()
- [x] Task 2.5: Add type validation to Rep()
- [x] **REGRESSION CHECK**

### Phase 3: Borrow Scope Integration
- [x] Task 3.1: Add lenders tracking to CallContext
- [x] Task 3.2: Implement exit_call with undo_lend
- [x] Task 3.3: Add Task reference to BorrowScope
- [x] Task 3.4: Implement lower_borrow same-instance optimization
- [x] Task 3.5: Add borrow count decrement on borrow drop
- [x] **REGRESSION CHECK**

### Phase 4: Destructor Support
- [x] Task 4.1: Expand ResourceType with destructor fields
- [x] Task 4.2: Add ComponentInstance reference to ResourceType
- [x] Task 4.3: Implement destructor invocation on owned drop
- [x] Task 4.4: Add same-instance destructor call path
- [x] Task 4.5: Add cross-instance destructor routing
- [x] **REGRESSION CHECK**

### Phase 5: Advanced Features
- [ ] Task 5.1: Add may_leave field to instance state
- [ ] Task 5.2: Add may_leave checks to resource operations
- [ ] Task 5.3: Implement call_might_be_recursive
- [ ] Task 5.4: Add reentrance trap to resource.drop
- [ ] Task 5.5: Add table MAX_LENGTH enforcement
- [ ] **REGRESSION CHECK**

---

## Files Modified by This Plan

### Core Implementation Files
- `internal/component/resource_table.go` - Primary resource table implementation
- `internal/component/borrow_scope.go` - Borrow scope tracking
- `internal/component/call_context.go` - Call context for borrow validation
- `internal/component/types/resource.go` - Resource type definitions

### Test Files
- `internal/component/resource_table_test.go` - Resource table unit tests
- `internal/component/borrow_scope_test.go` - Borrow scope unit tests
- `internal/component/call_context_test.go` - Call context unit tests
- `internal/component/conformance/resources_test.go` - Conformance tests
- `internal/component/conformance/resource_type_validation_test.go` - NEW: Type validation tests

### Regression Test Files
- `internal/component/wasip2test/calculator_test.go` - Calculator plugin tests (DO NOT MODIFY)

---

## Getting Started

1. Read the gap analysis: `docs/plans/resource-system-gap-analysis.md`
2. Start with Phase 1: `docs/plans/spec-resource-system-conformance-fixes/01-phase1-core-type-system.md`
3. After each phase, run regression tests
4. Commit after each phase passes

---

## Commit Convention

Use conventional commits with scope:

```
feat(resource): add ResourceTypeID tracking to HandleEntry
fix(resource): trap on invalid handle in resource.drop
test(resource): add type validation conformance tests
```
