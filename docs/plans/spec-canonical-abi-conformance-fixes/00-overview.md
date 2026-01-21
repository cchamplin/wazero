# Canonical ABI Conformance Fixes - Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Bring wazero's Canonical ABI type lifting and lowering implementation into full conformance with the Component Model specification.

**Architecture:** Incremental fixes to `internal/component/abi/` and `internal/component/types/` packages, following TDD approach with each fix validated against spec behavior and existing regression tests.

**Tech Stack:** Go, WebAssembly Component Model, Canonical ABI

---

## Gap Analysis Reference

This implementation plan is derived from the comprehensive gap analysis at:
- **`docs/plans/canonical-abi-gap-analysis.md`**

## Reference Materials

These documents should be consulted during implementation:

| Resource | Path | Purpose |
|----------|------|---------|
| Primary Spec | `debug-vendored/component-model/design/mvp/CanonicalABI.md` | Authoritative specification |
| Python Reference | `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py` | Executable reference implementation |
| Wasmtime component-util | `debug-vendored/wasmtime/crates/component-util/src/lib.rs` | Alignment/size utilities |
| Wasmtime values.rs | `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/values.rs` | Value lifting/lowering reference |
| Wasmtime tests | `debug-vendored/wasmtime/tests/all/component_model/` | Runtime test cases |

## Regression Requirements

**CRITICAL:** All complete phases MUST ensure the following tests continue to pass:

```bash
# Run after EVERY phase
CGO_ENABLED=0 go test -v ./internal/component/wasip2test/... -run "TestCalculatorPlugins"
```

The calculator tests exercise:
- **add plugin** (Rust): `evaluate(28, 3) = 31`, returns string "add"
- **subtract plugin** (C): `evaluate(28, 3) = 25`, returns string "subtract"
- **multi plugin** (Go): `evaluate(28, 3) = 84`, returns string "Simple-Go-Multi"
- **div plugin** (Go): `evaluate(28, 3) = 9`, returns string "Simple-Go-Div"

These use `s32` and `string` types, so most fixes won't affect them, but verification is mandatory.

---

## Phase Overview

| Phase | Focus | Document | Priority |
|-------|-------|----------|----------|
| 1 | Critical Fixes | [01-phase1-critical-fixes.md](./01-phase1-critical-fixes.md) | **HIGH** |
| 2 | Major Improvements | [02-phase2-major-improvements.md](./02-phase2-major-improvements.md) | MEDIUM |
| 3 | Async Support | [03-phase3-async-support.md](./03-phase3-async-support.md) | LOW (Deferred) |

---

## Progress Tracking

### Phase 1: Critical Fixes (7 tasks)

- [x] **Task 1.1:** Float NaN Canonicalization
- [x] **Task 1.2:** String Alignment Validation
- [x] **Task 1.3:** List Element Alignment Validation
- [x] **Task 1.4:** Variant Flatten Join Semantics
- [x] **Task 1.5:** Variant Lift Type Coercion
- [x] **Task 1.6:** Variant Lower Type Coercion
- [x] **Task 1.7:** Resource Type Validation

### Phase 2: Major Improvements (5 tasks)

- [x] **Task 2.1:** Fixed-Length List Type Support
- [x] **Task 2.2:** Fixed-Length List Lifting
- [x] **Task 2.3:** Fixed-Length List Lowering
- [x] **Task 2.4:** Empty Type Prohibition
- [x] **Task 2.5:** Borrow Optimization for Same Instance (documented as TODO)

### Phase 3: Async Support (3 tasks - Deferred)

- [ ] **Task 3.1:** ErrorContext Type
- [ ] **Task 3.2:** Stream Type
- [ ] **Task 3.3:** Future Type

---

## Implementation Order

Execute phases and tasks in order. Each task follows TDD:

1. Write failing test
2. Verify it fails
3. Implement minimal fix
4. Verify test passes
5. Run regression tests
6. Commit

---

## Spec Line References

Quick reference to key spec sections in `CanonicalABI.md`:

| Topic | Lines |
|-------|-------|
| Despecialization | 1790-1810 |
| Alignment | 1842-1921 |
| Element Size | 1924-1985 |
| Loading | 1987-2270 |
| Storing | 2272-2704 |
| Flattening | 2707-2841 |
| Flat Lifting | 2843-2998 |
| Flat Lowering | 3000-3105 |

---

## Key Code Locations

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
