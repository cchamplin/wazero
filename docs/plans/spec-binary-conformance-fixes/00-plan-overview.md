# Component Model Binary Parser Spec Conformance

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Bring the wazero Component Model binary format parser into full conformance with the official specification.

**Architecture:** The parser in `internal/component/binary/` decodes Component Model binaries into Go structures in `internal/component/`. This plan fixes gaps identified in the gap analysis, prioritizing features needed for WASI P2 compatibility while deferring gated async/threading features.

**Tech Stack:** Go, WebAssembly Component Model, LEB128 encoding

---

## Reference Materials

**CRITICAL: Use these references when implementing:**

| Resource | Path | Purpose |
|----------|------|---------|
| **Primary Spec** | `debug-vendored/component-model/design/mvp/Binary.md` | Authoritative binary format |
| **Index Spaces** | `debug-vendored/component-model/design/mvp/Explainer.md` | Index space semantics |
| **wasmparser** | `debug-vendored/wasm-tools/crates/wasmparser/src/readers/component/` | Reference parsing impl |
| **wasmtime** | `debug-vendored/wasmtime/crates/environ/src/component/` | Component translation |
| **Test WATs** | `debug-vendored/wasmtime/tests/misc_testsuite/component-model/` | Expected parsing behavior |

**Gap Analysis:** `docs/plans/component-model-binary-parser-gap-analysis.md`

---

## Regression Requirement

**CRITICAL:** All changes MUST ensure these tests continue to pass:

```bash
CGO_ENABLED=0 go test ./internal/component/wasip2test/... -run "TestCalculatorPlugins/(add|subtract)" -v
```

Run this command after each phase to verify no regressions.

---

## Phase Overview

| Phase | Name | Status | Document |
|-------|------|--------|----------|
| 1 | Index Space Management | ✅ Complete | [01-phase-index-space-management.md](./01-phase-index-space-management.md) |
| 2 | Canonical Options | ✅ Complete | [02-phase-canonical-options.md](./02-phase-canonical-options.md) |
| 3 | Type Definition Fixes | ✅ Complete | [03-phase-type-definitions.md](./03-phase-type-definitions.md) |
| 4 | Value Section Completion | ✅ Complete | [04-phase-value-section.md](./04-phase-value-section.md) |
| 5 | Async Canonicals (Gated) | ⏸️ Deferred | [05-phase-async-canonicals.md](./05-phase-async-canonicals.md) |
| 6 | Validation Layer | ✅ Complete | [06-phase-validation.md](./06-phase-validation.md) |

**Status Legend:**
- ⬜ Not Started
- 🔄 In Progress
- ✅ Complete
- ⏸️ Blocked

---

## Progress Tracking

### Phase 1: Index Space Management ✅
- [x] Task 1.1: Add missing index counters to Component struct
- [x] Task 1.2: Update alias.go to increment all index spaces
- [x] Task 1.3: Update decoder.go for import index effects
- [x] Task 1.4: Add index space tracking tests
- [x] Task 1.5: Run regression tests and commit

### Phase 2: Canonical Options ✅
- [x] Task 2.1: Add async option (0x06) parsing
- [x] Task 2.2: Add callback option (0x07) parsing
- [x] Task 2.3: Add core-type option (0x08) parsing
- [x] Task 2.4: Add gc option (0x09) parsing
- [x] Task 2.5: Update CanonicalOptions struct
- [x] Task 2.6: Add canonical options tests
- [x] Task 2.7: Run regression tests and commit

### Phase 3: Type Definition Fixes ✅
- [x] Task 3.1: Handle 0x00 0x50 core type prefix
- [x] Task 3.2: Add async resource destructor (0x3e)
- [x] Task 3.3: Add type definition tests
- [x] Task 3.4: Run regression tests and commit

### Phase 4: Value Section Completion ✅
- [x] Task 4.1: Add float value decoding (f32, f64)
- [x] Task 4.2: Add char value decoding
- [x] Task 4.3: Add string value decoding
- [x] Task 4.4: Add composite value decoding
- [x] Task 4.5: Add value section tests
- [x] Task 4.6: Run regression tests and commit

### Phase 5: Async Canonicals (Gated) ⏸️ DEFERRED
> Deferred: 40+ gated async/threading opcodes not needed for basic WASI P2 support.
> These can be implemented when async component model features are needed.

- [ ] Task 5.1: Add task canonicals (0x05, 0x09)
- [ ] Task 5.2: Add context canonicals (0x0a, 0x0b)
- [ ] Task 5.3: Add subtask canonicals (0x06, 0x0d)
- [ ] Task 5.4: Add stream canonicals (0x0e-0x14)
- [ ] Task 5.5: Add future canonicals (0x15-0x1b)
- [ ] Task 5.6: Add error-context canonicals (0x1c-0x1e)
- [ ] Task 5.7: Add waitable-set canonicals (0x1f-0x23)
- [ ] Task 5.8: Add backpressure canonicals (0x24-0x25)
- [ ] Task 5.9: Add threading canonicals (0x26-0x42)
- [ ] Task 5.10: Add async canonical tests
- [ ] Task 5.11: Run regression tests and commit

### Phase 6: Validation Layer ✅
- [x] Task 6.1: Add type element count validation
- [x] Task 6.2: Add borrow-in-results validation
- [x] Task 6.3: Add outer alias sort validation
- [x] Task 6.4: Add unique name validation
- [x] Task 6.5: Add validation tests
- [x] Task 6.6: Run regression tests and commit

---

## Completion Criteria

1. All phases marked ✅ Complete
2. All regression tests pass
3. Gap analysis items addressed or documented as deferred
4. Code reviewed and merged to main branch
