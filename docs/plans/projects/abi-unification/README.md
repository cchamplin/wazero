# Canonical ABI Unification — Implementation Project

**Design:** [`docs/plans/2026-04-07-canonical-abi-unification-design.md`](../../2026-04-07-canonical-abi-unification-design.md)

**Status:** In progress

## What this project does

Unifies wazero's component-model implementation around
`internal/component/abi/` as the single source of truth for canonical ABI
lift/lower. Eliminates four parallel lift/lower implementations, four
duplicate type converters, and 67 silent-default-on-bad-handle sites in the
wasi-p2 host imports. Validates the result against the upstream
component-model spec WAST suite and the wit-bindgen runtime test suite,
exclusively through wazero's public API.

Read the design document linked above for the full problem statement,
findings from research, and architectural decisions.

## How to work this project

Every session that touches this project starts by invoking
[`loop-driver.md`](loop-driver.md). The driver enforces the per-item lifecycle
(claim → implement → spec-review → code-review → commit) and ensures every
agent reads the spec authorities before writing code.

```
1. Read loop-driver.md
2. Read spec-authorities.md
3. Pick the active loop's tracking file (loop-1, loop-2, or loop-3)
4. Find the next pending item; follow its lifecycle to done
5. Resume next session from the tracking file
```

## File index

| File | Purpose |
|---|---|
| [`README.md`](README.md) | This file |
| [`spec-authorities.md`](spec-authorities.md) | Mandatory reading list for every agent + universal rules |
| [`loop-driver.md`](loop-driver.md) | Universal session-start prompt; runs the per-item lifecycle |
| [`loop-1-tracking.md`](loop-1-tracking.md) | Loop 1 backlog (~35 items): unify type representation, make `abi/` correct |
| [`loop-2-tracking.md`](loop-2-tracking.md) | Loop 2 backlog (~16 items): wire `abi/` into production, delete dead code |
| [`loop-3-tracking.md`](loop-3-tracking.md) | Loop 3 backlog (~18 items): real wasm via public API, migrate existing tests |
| [`templates/implement-task.md`](templates/implement-task.md) | Implementation subagent prompt template |
| [`templates/review-spec-compliance.md`](templates/review-spec-compliance.md) | Spec-compliance reviewer subagent prompt template |
| [`templates/review-code-quality.md`](templates/review-code-quality.md) | Code-quality reviewer subagent prompt template |
| [`templates/verify-loop-complete.md`](templates/verify-loop-complete.md) | Terminal sweep subagent prompt template |

## Loop status dashboard

| Loop | Items | Status | Notes |
|---|---|---|---|
| Loop 1 — Type unification + `abi/` correctness | ~35 | not started | Phase 1.A is a high-risk surgical change; expect `go test ./...` to be broken until Loop 2 wires things in |
| Loop 2 — Wire `abi/` into production | ~16 | blocked on Loop 1 | |
| Loop 3 — Real wasm via public API | ~18 | blocked on Loop 2 | |

Loops are strictly sequential. Update this table when a loop opens or closes.

## Termination criteria

Project is complete when all 10 conditions in the design's "Termination
criteria" section are met. The verifier subagent (run via
[`templates/verify-loop-complete.md`](templates/verify-loop-complete.md))
checks them mechanically at the end of each loop.

## Out of scope

Async, streams, futures, error-context, threads, subtasks, cancellation, and
async-flavored post-return are tracked for a separate follow-up project.
See the design's "Out of scope" section.
