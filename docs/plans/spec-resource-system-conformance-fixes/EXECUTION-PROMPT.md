# Resource System Spec Conformance - Execution Prompt

**Copy everything below the line to start a subagent-driven development session.**

---

## Task: Implement Resource System Spec Conformance

I need you to execute a comprehensive implementation plan that brings the wazero Component Model resource system into full conformance with the official Component Model specification.

### Plan Location

The implementation plan is located at:
```
docs/plans/spec-resource-system-conformance-fixes/
```

**Files:**
- `00-root-plan.md` - Master tracking document (start here)
- `01-phase1-core-type-system.md` - Phase 1: Add ResourceTypeID tracking
- `02-phase2-trap-conditions.md` - Phase 2: Fix trap conditions
- `03-phase3-borrow-scope-integration.md` - Phase 3: Borrow scope integration
- `04-phase4-destructor-support.md` - Phase 4: Destructor support
- `05-phase5-advanced-features.md` - Phase 5: Advanced features

### Gap Analysis

The detailed gap analysis documenting all defects and missing features is at:
```
docs/plans/resource-system-gap-analysis.md
```

### Reference Materials

**Primary Specification:**
- `debug-vendored/component-model/design/mvp/CanonicalABI.md`
- Key sections: Lines 493-550 (Resource State), 2215-2240 (lift_own/lift_borrow), 2673-2683 (lower_own/lower_borrow), 3590-3688 (canonical builtins)

**Wasmtime Reference Implementation:**
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/vm/component/resources.rs`
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/resources/host_tables.rs`

### Critical Regression Requirement

**After completing EACH PHASE**, you MUST verify these tests pass:

```bash
CGO_ENABLED=0 go test -v ./internal/component/wasip2test/... -run "TestCalculatorPlugins/add"
CGO_ENABLED=0 go test -v ./internal/component/wasip2test/... -run "TestCalculatorPlugins/subtract"
```

Both `add` and `subtract` plugin tests MUST pass before proceeding to the next phase. Do NOT proceed if they fail.

### Execution Instructions

1. **Use the subagent-driven-development skill** to execute this plan
2. **Start with Phase 1** (`01-phase1-core-type-system.md`)
3. **Follow TDD**: Each task has steps for failing test → implementation → passing test → commit
4. **Run regression tests** after completing all tasks in a phase
5. **Update progress** in `00-root-plan.md` by checking off completed tasks
6. **Commit after each task** using the commit messages provided in the plan

### Files You Will Modify

**Core Implementation:**
- `internal/component/resource_table.go`
- `internal/component/borrow_scope.go`
- `internal/component/call_context.go`
- `internal/component/types/resource.go`

**New Files to Create:**
- `internal/component/resource_type_id.go`
- `internal/component/destructor.go`
- `internal/component/instance_state.go`
- `internal/component/reentrance.go`
- `internal/component/abi/resource_lower.go`

**Test Files:**
- Corresponding `*_test.go` files for each implementation file
- `internal/component/conformance/destructor_test.go`

### Current State

The current implementation has:
- Basic resource table with generation counting (working)
- Handle entry with ownership tracking (working)
- Borrow scope tracking (partial - missing Task integration)

The current implementation is missing:
- ResourceType tracking in handles (GAP 1.1 - Critical)
- Proper trap conditions (GAP 3.2.1, 3.3.1 - Defects)
- Same-instance optimization for lower_borrow (GAP 4.4.1)
- Destructor invocation (GAP 3.2.5)
- may_leave checks (GAP 3.1.1)
- Reentrance protection (GAP 3.2.6)

### Commit Convention

Use conventional commits:
```
feat(resource): description
fix(resource): description
test(resource): description
docs(resource): description
```

### Success Criteria

The plan is complete when:
1. All 25 tasks across 5 phases are implemented
2. All regression tests pass after each phase
3. All new tests pass
4. All checkboxes in `00-root-plan.md` are checked
5. Final milestone commit is made

---

**Begin by reading `00-root-plan.md` and then invoke the `superpowers:subagent-driven-development` skill to start execution.**
