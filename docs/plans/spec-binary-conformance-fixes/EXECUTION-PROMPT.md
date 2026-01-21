# Component Model Binary Parser Conformance - Execution Prompt

**Copy everything below the line to start a subagent-driven development session.**

---

## Task

Implement the Component Model binary parser spec conformance fixes for wazero, following the phased implementation plan.

## Plan Location

All plan documents are in `docs/plans/spec-binary-conformance-fixes/`:

- `00-plan-overview.md` - Master tracking document (update checkboxes as you complete tasks)
- `01-phase-index-space-management.md` - Phase 1: Track all 12 index spaces
- `02-phase-canonical-options.md` - Phase 2: Add canonical options 0x06-0x09
- `03-phase-type-definitions.md` - Phase 3: Core type prefix and async resource
- `04-phase-value-section.md` - Phase 4: Complete value section decoder
- `05-phase-async-canonicals.md` - Phase 5: 40+ async/threading canonicals
- `06-phase-validation.md` - Phase 6: Validation rules

## Gap Analysis

The detailed gap analysis is at `docs/plans/component-model-binary-parser-gap-analysis.md`. Reference this for understanding WHY each change is needed.

## Reference Materials (USE THESE)

When implementing, cross-reference these authoritative sources:

| Resource | Path | Use For |
|----------|------|---------|
| **Binary Spec** | `debug-vendored/component-model/design/mvp/Binary.md` | Definitive binary format |
| **Index Spaces** | `debug-vendored/component-model/design/mvp/Explainer.md` | Index space semantics |
| **wasmparser** | `debug-vendored/wasm-tools/crates/wasmparser/src/readers/component/` | Reference Rust implementation |
| **wasmtime** | `debug-vendored/wasmtime/crates/environ/src/component/` | How wasmtime uses parsed data |

Key wasmparser files:
- `canonicals.rs` - Canonical function parsing (40+ opcodes)
- `types.rs` - Type definitions and ComponentValType
- `aliases.rs` - Alias parsing with sort handling
- `imports.rs` / `exports.rs` - Import/export parsing

## Implementation Location

All changes go in `internal/component/`:
- `component.go` - Data structures (Component, TypeDef, CanonicalDef, etc.)
- `binary/` - Binary format decoders

## Critical Regression Requirement

**AFTER EVERY PHASE**, run this command and ensure it passes:

```bash
CGO_ENABLED=0 go test ./internal/component/wasip2test/... -run "TestCalculatorPlugins/(add|subtract)" -v
```

Both `add` and `subtract` tests MUST pass. If they fail, you've broken something - investigate and fix before continuing.

## Execution Instructions

1. **Use the subagent-driven-development skill** to work through each phase
2. **Read each phase document thoroughly** before starting that phase
3. **Follow tasks in order** - they build on each other
4. **Run tests after each task** as specified in the plan
5. **Commit after each phase** with the provided commit message format
6. **Update `00-plan-overview.md`** checkboxes as you complete tasks

## Phase Order

Execute phases in order (each depends on previous):

1. **Phase 1: Index Space Management** - Foundation for correct parsing
2. **Phase 2: Canonical Options** - Quick win, enables async features
3. **Phase 3: Type Definitions** - Fixes edge cases in type parsing
4. **Phase 4: Value Section** - Completes value encoding support
5. **Phase 5: Async Canonicals** - Large phase, 40+ opcodes (can defer if not needed)
6. **Phase 6: Validation** - Polish, catches malformed components

## Code Style

- Follow existing patterns in `internal/component/binary/`
- Use `leb128.DecodeUint32` for unsigned integers
- Use `bytes.Reader` for input
- Return descriptive errors with context: `fmt.Errorf("decode X: %w", err)`
- Add constants for new opcodes near related constants

## Testing Approach

Each phase includes test tasks. For tests:
- Create `*_test.go` files in `internal/component/binary/`
- Use `bytes.NewReader()` to create test input
- Test both success and error cases
- Use table-driven tests where appropriate

## Starting Point

Begin by:
1. Reading `00-plan-overview.md` to understand the full scope
2. Reading `01-phase-index-space-management.md` for first phase details
3. Running the regression test to establish baseline:
   ```bash
   CGO_ENABLED=0 go test ./internal/component/wasip2test/... -run "TestCalculatorPlugins/(add|subtract)" -v
   ```
4. Using the subagent-driven-development skill to execute Phase 1

## Success Criteria

- All phases completed (or explicitly deferred with justification)
- All regression tests pass
- Each phase committed with descriptive message
- `00-plan-overview.md` shows all completed checkboxes
