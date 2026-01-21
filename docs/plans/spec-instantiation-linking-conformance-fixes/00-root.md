# Component Model Instantiation & Linking Conformance Fixes

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Bring wazero's component model instantiation and linking to full spec compliance, enabling multi-component scenarios, proper type validation, and start function support.

**Architecture:** Implement spec compliance in phases, with each phase building on the previous. Phase 1 (Type Checking) is foundational. Phases 2-4 can proceed in parallel after Phase 1. Phase 5 is independent.

**Tech Stack:** Go, WebAssembly Component Model, Canonical ABI

---

## Reference Materials

### Primary Specifications
- `debug-vendored/component-model/design/mvp/Explainer.md` - Instance definitions, Import/Export, Type matching rules
- `debug-vendored/component-model/design/mvp/Binary.md` - Instance sections encoding
- `debug-vendored/component-model/design/mvp/Linking.md` - Import resolution, semver matching

### Wasmtime Reference Implementation
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs` - Linker structure, import resolution
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/matching.rs` - Type matching/subtyping (KEY FILE)
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/instance.rs` - Instance state management
- `debug-vendored/wasmtime/crates/environ/src/component/translate/` - Component translation

### Current Implementation
- `internal/component/linker.go` - Basic linker
- `internal/component/component_linker.go` - Full runtime integration
- `internal/component/instance.go` - Instance structure

### Gap Analysis (This Plan's Source)
- `docs/plans/2026-01-20-instantiation-linking-gap-analysis.md`
- `docs/plans/2026-01-20-instantiation-linking-implementation-plan.md`
- `docs/plans/2026-01-20-instantiation-linking-test-scenarios.md`

---

## Regression Requirement

**CRITICAL:** After completing each PHASE (not each task), verify:

```bash
CGO_ENABLED=0 go test -v ./internal/component/wasip2test/... -run TestCalculatorPlugins/add
CGO_ENABLED=0 go test -v ./internal/component/wasip2test/... -run TestCalculatorPlugins/subtract
```

Both `add` and `subtract` tests MUST pass. These validate the core instantiation/linking functionality.

---

## Progress Tracker

| Phase | Document | Status | Regression |
|-------|----------|--------|------------|
| 1 | [Type Checking System](./01-phase1-type-checking.md) | [x] Complete | [x] Verified |
| 2 | [Start Function Support](./02-phase2-start-function.md) | [x] Complete | [x] Verified |
| 3 | [Nested Component Support](./03-phase3-nested-components.md) | [x] Complete | [x] Verified |
| 4 | [Export Instance API](./04-phase4-export-instance-api.md) | [x] Complete | [x] Verified |
| 5 | [Advanced Import Names](./05-phase5-advanced-imports.md) | [ ] Not Started | [ ] Verified |

---

## Phase Dependency Graph

```
                    ┌─────────────────────────────────┐
                    │  Phase 1: Type Checking         │
                    │  (Foundation - must be first)   │
                    └───────────────┬─────────────────┘
                                    │
        ┌───────────────────────────┼───────────────────────────┐
        │                           │                           │
        ▼                           ▼                           ▼
┌───────────────────┐   ┌───────────────────────┐   ┌───────────────────┐
│ Phase 2:          │   │ Phase 3:              │   │ Phase 5:          │
│ Start Function    │   │ Nested Components     │   │ Advanced Imports  │
│                   │   │                       │   │ (Independent)     │
└───────────────────┘   └──────────┬────────────┘   └───────────────────┘
                                   │
                                   ▼
                        ┌───────────────────────┐
                        │ Phase 4:              │
                        │ Export Instance API   │
                        └───────────────────────┘
```

---

## Summary of Gaps Being Fixed

From the [gap analysis](../2026-01-20-instantiation-linking-gap-analysis.md):

### Phase 1: Type Checking
- **Gap:** No type validation at instantiation time
- **Current:** `MatchImport` does name matching only, no type checking
- **Fix:** Create `TypeChecker` with full subtyping rules per spec

### Phase 2: Start Function
- **Gap:** `c.Start` parsed but never executed
- **Current:** Start definition exists in Component but ignored
- **Fix:** Execute start function, manage value index space

### Phase 3: Nested Components
- **Gap:** `c.ComponentInstances` tracked but never instantiated
- **Current:** Just counts component instances, never processes them
- **Fix:** Recursive component instantiation with scope resolution

### Phase 4: Export Instance API
- **Gap:** `ExportedInstance()` returns nil
- **Current:** Instance exports not tracked or exposed
- **Fix:** Track and expose exported instances via API

### Phase 5: Advanced Import Names
- **Gap:** Only interface imports supported
- **Current:** No locked-dep, unlocked-dep, version ranges
- **Fix:** Parse all import name variants per spec

---

## Execution Instructions

1. **Start with Phase 1** - It's the foundation for type safety
2. **After Phase 1**, Phases 2, 3, and 5 can proceed in any order
3. **Phase 4 requires Phase 3** - Needs nested instance support
4. **Run regression tests after each PHASE** (not each task)
5. **Commit after each task** for easy rollback

---

## Quick Start

To begin implementation:

1. Open the Phase 1 document: [01-phase1-type-checking.md](./01-phase1-type-checking.md)
2. Follow tasks in order
3. After all Phase 1 tasks complete, run regression tests
4. Move to next phase

**Recommended session command:**
```bash
cd /Users/cchamplin/go/src/github.com/cchamplin/wazero
# Read Phase 1 plan and begin
```
