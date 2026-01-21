# Subagent-Driven Development Session Prompt

**Copy everything below the line to start a new session:**

---

## Task: Implement Component Model Instantiation & Linking Conformance Fixes

I need you to implement the component model instantiation and linking conformance fixes for wazero using the subagent-driven development approach.

### Context

The wazero WebAssembly runtime has a component model implementation that works for basic cases (calculator plugins pass), but has gaps compared to the official Component Model specification. A comprehensive gap analysis has been completed and a detailed implementation plan created.

### Plan Location

All plan documents are in:
```
docs/plans/spec-instantiation-linking-conformance-fixes/
```

**Start by reading the root document:**
```
docs/plans/spec-instantiation-linking-conformance-fixes/00-root.md
```

**Phase documents (in dependency order):**
1. `01-phase1-type-checking.md` - MUST be done first (foundation)
2. `02-phase2-start-function.md` - After Phase 1
3. `03-phase3-nested-components.md` - After Phase 1
4. `04-phase4-export-instance-api.md` - After Phase 3
5. `05-phase5-advanced-imports.md` - Independent, can be done anytime

### Reference Materials (USE THESE)

**Specifications:**
- `debug-vendored/component-model/design/mvp/Explainer.md` - Instance definitions, type matching rules
- `debug-vendored/component-model/design/mvp/Binary.md` - Binary encoding
- `debug-vendored/component-model/design/mvp/Linking.md` - Import resolution

**Wasmtime Reference Implementation:**
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/matching.rs` - Type matching (KEY FILE)
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs` - Linker implementation
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/instance.rs` - Instance management

**Gap Analysis:**
- `docs/plans/2026-01-20-instantiation-linking-gap-analysis.md`
- `docs/plans/2026-01-20-instantiation-linking-implementation-plan.md`
- `docs/plans/2026-01-20-instantiation-linking-test-scenarios.md`

### Current Implementation Files

- `internal/component/linker.go` - Basic linker
- `internal/component/component_linker.go` - Full runtime integration
- `internal/component/instance.go` - Instance structure
- `internal/component/type_checker.go` - Will be created in Phase 1

### CRITICAL: Regression Requirement

**After completing each PHASE (not each task), verify these tests pass:**

```bash
CGO_ENABLED=0 go test -v ./internal/component/wasip2test/... -run TestCalculatorPlugins/add
CGO_ENABLED=0 go test -v ./internal/component/wasip2test/... -run TestCalculatorPlugins/subtract
```

Both `add` and `subtract` tests MUST pass. If they break, fix before proceeding.

### Execution Instructions

1. **Use the `superpowers:subagent-driven-development` skill** to execute this plan
2. **Read the root plan document first** (`00-root.md`)
3. **Start with Phase 1** - It's the foundation for type safety
4. **Follow tasks in order within each phase** - Each task has 5 steps
5. **Commit after each task** - Small, atomic commits
6. **Run regression tests after each PHASE** - Not after each task
7. **Update the progress tracker** in `00-root.md` after each phase

### What Each Phase Implements

| Phase | Gap Being Fixed | Key Files Created/Modified |
|-------|-----------------|---------------------------|
| 1 | No type validation at instantiation | `type_checker.go` (new) |
| 2 | Start function never executed | `instance.go`, `component_linker.go` |
| 3 | Nested components not instantiated | `nested_component.go` (new), `outer_alias.go` (new) |
| 4 | ExportedInstance returns nil | `linker_api.go` |
| 5 | Only interface imports supported | `import_name.go` (new), `semver.go` |

### Success Criteria

- All phase tests pass
- Calculator `add` and `subtract` tests pass after each phase
- Progress tracker in `00-root.md` shows all phases complete
- Code follows existing patterns in the codebase

### Starting Point

Begin by invoking the `superpowers:subagent-driven-development` skill and reading the root plan document. The skill will guide you through dispatching subagents for each task.

```
Use skill: superpowers:subagent-driven-development
Plan file: docs/plans/spec-instantiation-linking-conformance-fixes/00-root.md
```
