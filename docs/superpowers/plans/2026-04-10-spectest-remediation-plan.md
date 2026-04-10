# Spectest Remediation Plan — Iterative Fix Loop

> **For agentic workers:** This plan is executed by an **orchestrator agent** that dispatches **worker subagents** for each fix and **reviewer subagents** for each verification. The orchestrator does NOT do the implementation work itself — it dispatches, validates, and tracks progress.

**Goal:** All 60 .wast test files pass (except tags.wast skip for the exception-handling proposal, which is a core wasm engine limitation, not a component model gap). Zero failures. Every test either passes or has an async spec citation.

**Current baseline:** Pass: 295 | Fail: 836 | Skip: 1 (tags.wast exception-handling)

**Repo:** /home/cchamplin/development/wazero  
**Branch:** feat/wasip2-complete-implementation

---

## Non-Negotiable Requirements (Apply to ALL Agents)

Every agent — orchestrator, worker, reviewer — must understand and follow these:

1. **The canonical-ABI spec and wasmtime are the authorities.** Every behavioral decision must be verified against the vendored spec files and wasmtime source. No guessing from training data.

2. **Only async is deferred.** Stream, future, error-context, subtask, and any other features that are explicitly part of the component model async proposal — these are the ONLY features that may remain unimplemented. Everything else is in scope and must work. If a test exercises a non-async feature and fails, it must be fixed. When uncertain whether a feature is async, read the spec — if it involves `Task`, `Subtask`, `Stream`, `Future`, `ErrorContext`, `Waitable`, `WaitableSet`, async lifting/lowering, or thread/callback machinery, it is async. Everything else is sync and must be implemented.

3. **No known failures, no known defects.** Do not insert TODOs. Do not add skip logic. Do not add hardcoded error-string matching to bypass failures. Do not stub or mock. Do not add "known limitation" comments. Do not defer work to a future session. Fix it or explain why it's async.

4. **No duplicate types, functions, or fields.** The codebase has `binary/`, `runtime/`, `types/`, `abi/`, `component/` subpackages. A concept may already exist under a different name in a different package. Before creating anything new, perform the existence verification process (see below). If something already exists, use it. If two things exist that should be one, unify them.

5. **Fix at source.** Bugs in `abi/` are fixed in `abi/`. Bugs in `binary/` are fixed in `binary/`. Do not add workarounds in `component/` or test code.

6. **Do not change what tests expect.** The .wast files are upstream test cases. They define correct behavior. If a test fails, the implementation is wrong, not the test.

---

## Existence Verification Process

Before creating ANY new type, function, field, or constant, follow this process:

### Step 1: Search by exact name
```bash
grep -rn "ExactName" internal/component/ --include="*.go" | grep -v _test.go
```

### Step 2: Search by concept/purpose
Think about what the thing DOES, not what it's called. Search for related terms:
```bash
# Example: looking for a "resource store" concept
grep -rn "store\|registry\|lookup.*resource\|resource.*map\|resource.*pool" internal/component/ --include="*.go" | grep -v _test.go
```

### Step 3: Search by type signature
If adding a function, search for functions with similar parameter/return types:
```bash
# Example: looking for something that takes (uint32, *ResourceType) and returns Handle
grep -rn "func.*uint32.*ResourceType\|func.*ResourceType.*uint32" internal/component/ --include="*.go" | grep -v _test.go
```

### Step 4: Read the candidate file
If you find something that MIGHT be the same thing, read the full function/type definition and its doc comment. Determine:
- Is this the same concept? (Same purpose, same semantics)
- Is this a different concept with a similar name? (e.g., `lift` in abi/lift.go vs `lift` in a canon_lift closure — different layers)
- Is this a complementary concept? (e.g., `LiftContext` and `LowerContext` are related but distinct — do NOT unify them)

### Step 5: Check package boundaries
If the existing thing is in a different package, check if you can import it without creating a cycle:
```bash
grep -rn "\"github.com/tetratelabs/wazero/internal/component/runtime\"" internal/component/abi/ --include="*.go"
```

### When in doubt: READ the existing code. Do NOT create a parallel.

---

## Priority 1 — Spectest Runner Infrastructure

### How to Fix the Runner

The spectest runner at `internal/component/spectest/` parses `.wast` files (via `wasm-tools json-from-wast`) and executes commands. The current implementation is incomplete.

**Before making changes, study how upstream runtimes implement .wast test runners:**

1. **Read wasmtime's .wast runner:**
   - `debug-vendored/wasmtime/crates/wast/src/wast.rs` — wasmtime's full .wast test runner
   - Understand how it tracks state (current instance, registered instances, module definitions)
   - Understand how it handles ALL command types (module, assert_return, assert_trap, assert_invalid, assert_malformed, assert_unlinkable, register, invoke)
   - Understand how it converts between .wast JSON values and runtime values

2. **Read wazero's existing core wasm spectest runner:**
   - `internal/integration_test/spectest/` — wazero already has a .wast runner for core wasm spec tests
   - Study its patterns: how it tracks state, handles assertions, manages instances
   - The component spectest runner should follow the same idioms

3. **Read the .wast format documentation:**
   - The output of `wasm-tools json-from-wast` produces a JSON structure with command types
   - Run `wasm-tools json-from-wast internal/component/spectest/testdata/wasmtime/simple.wast` to see the actual JSON structure
   - Ensure EVERY command type in the JSON output has a handler

**The runner must handle ALL command types that `wasm-tools json-from-wast` can emit.** The following are known command types, but this list may not be exhaustive — if the runner encounters an unknown command type during execution, it must be implemented, not skipped:

| Command | Purpose | Current State |
|---------|---------|---------------|
| `module` | Compile + instantiate a component, set as current | Partially implemented |
| `module-definition` | Compile a component without instantiating | Implemented |
| `assert_return` | Invoke function, compare results | Implemented |
| `assert_trap` | Invoke function, expect trap | Implemented |
| `assert_invalid` | Compile component, expect validation error | Implemented |
| `assert_malformed` | Parse component, expect malformation error | Missing |
| `assert_unlinkable` | Compile + instantiate, expect link error | Missing |
| `assert_uninstantiable` | Compile + instantiate, expect instantiation error | Missing |
| `register` | Register current instance under a name for imports | Partially implemented |
| `invoke` | Call a function on current instance (no assertion) | Implemented |

Any command type not listed here that appears in the `.wast` JSON output must also be implemented. Run `wasm-tools json-from-wast` on each `.wast` file and check for command types not in this table. Wasmtime's `wast.rs` runner is the definitive reference for what commands exist and how they behave.

**State tracking requirements (minimum — study wasmtime's wast.rs for the complete set):**
- The runner must maintain a "current instance" that persists across commands
- `module` commands set the current instance
- `register` commands register the current instance under a name AND wire it into the ComponentLinker so future components can import it
- `assert_return`/`assert_trap`/`invoke` commands operate on the current instance
- If a `module` command fails, subsequent assert commands that depend on it should report "prior module failed" (not "no current instance")
- Any additional state management patterns found in wasmtime's wast.rs must also be implemented — do not assume this list is complete

### Priority 1 Verification

After fixing the runner, re-run the full suite:
```bash
go test ./internal/component/spectest/ -count=1 2>&1 | grep -c "FAIL"
```
The "no current instance" failures (~140) should be eliminated. New failures may appear as previously-unreachable code paths are now exercised.

---

## Priority 2 — Binary Decoder Gaps

These fail at CompileComponent — the component binary can't even be parsed.

**For each decoder error:**
1. Read the exact error message to identify the missing opcode/sort/section
2. Read `debug-vendored/component-model/design/mvp/Binary.md` for the binary format spec
3. Read wasmtime's decoder: `debug-vendored/wasmtime/crates/wasmparser/` (search for the opcode)
4. Read wazero's decoder: `internal/component/binary/` — find where the error is raised
5. Implement the missing decoding, following the spec and wasmtime's patterns
6. Verify the decoding matches the spec exactly

**Known decoder gaps (from initial failure analysis — this list is NOT exhaustive):**
- Core sort `0x11` — core module sort in export/alias sections
- Sort prefix `0x636f7265` — the ASCII "core" prefix for two-level sort encoding
- Core type opcodes `0x03`, `0x04` — missing core type variants in module type decoding

Additional decoder gaps will be revealed as earlier fixes unblock more tests. Every `decode component:` error in the test output represents a decoder gap that must be fixed. Do not maintain a static list — dynamically discover and fix all decoder errors encountered during the remediation loop.

---

## Priority 3 — Instantiation Pipeline

Components parse but can't instantiate.

**For each instantiation error:**
1. Read the exact error message
2. Trace to the failing code in `internal/component/component_linker.go` or `internal/component/nested_component.go`
3. Read the spec section for the failing operation
4. Read wasmtime's instantiator: `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/instance.rs`
5. Fix the pipeline code

**Known pipeline gaps (from initial failure analysis — this list is NOT exhaustive):**
- Nested component instantiation — nested components with core modules need full compilation
- Module re-instantiation — the runtime must support multiple instances of the same compiled module
- Import resolution for registered instances — `register` command must wire into the linker

Additional pipeline gaps will be revealed as decoder fixes unblock more tests. Every `Instantiate` or `resolve imports` error represents a pipeline gap that must be fixed. Process all instantiation errors dynamically, not just the ones listed here.

---

## Priority 4 — ABI/Runtime Correctness

Components instantiate but function calls fail or produce wrong results. This is a broad category — any failure that isn't a runner bug, decoder gap, or pipeline issue falls here. Examples include but are not limited to:

- String encoding and transcoding errors
- Post-return semantics violations
- Type and validation mismatches
- Incorrect lift/lower behavior for any value type
- Resource handle lifecycle errors
- Reentrance/may_leave guard violations
- Incorrect flat ABI encoding/decoding

**For each ABI/runtime error:**
1. Read the exact error and expected-vs-actual values
2. Trace to the failing code — it may be in `internal/component/abi/`, `internal/component/instance.go`, `internal/component/component_linker.go`, `internal/component/runtime/`, or `internal/component/types/`
3. Read `definitions.py` for the specific operation
4. Read wasmtime's implementation of the same operation
5. Fix the code at source
6. Verify with the spec's reference implementation

Do not assume the error categories listed above are exhaustive. Any non-async test failure that reaches this priority level must be investigated and fixed regardless of its category.

---

## Orchestrator Agent Instructions

You are the orchestrator. You do NOT implement fixes yourself. You dispatch worker subagents and reviewer subagents.

### Loop Structure

```
while failures > 0:
    1. Run the full spectest suite, capture results
    2. Identify the highest-priority failing test (per priority ordering)
    3. Dispatch a WORKER subagent to fix it
    4. When worker completes, dispatch TWO REVIEWER subagents:
       a. Spec conformance reviewer — verifies fix matches the spec
       b. Code quality reviewer — verifies no duplicates, no workarounds, no TODOs
    5. Run the specific test yourself to verify it passes
    6. Run the full suite to verify no regressions
    7. If reviewers or tests find issues, dispatch a corrective worker subagent
    8. Once clean: commit, log progress, continue loop
```

### Dispatching Worker Subagents

Each worker subagent receives a prompt containing:

1. **The specific test failure** — exact test name, exact error message, exact line in .wast file
2. **The root cause category** — decoder gap, runner bug, pipeline issue, or ABI error
3. **Which spec section to read** — exact file path and line range
4. **Which wasmtime source to read** — exact file path
5. **The existence verification process** (copied from above)
6. **The non-negotiable requirements** (copied from above)
7. **The specific instruction:** "Fix this failure. Do not skip it. Do not work around it. Read the spec. Fix at source."

Example worker prompt:
```
FIX THIS TEST FAILURE:
  Test: TestSimpleWast/assert-invalid_line25_idx4
  Error: "Component should have failed to compile but succeeded"
  .wast line 25: (assert_invalid (component ...) "error message")

ROOT CAUSE: The wazero binary decoder does not validate [specific thing].

READ THESE FILES:
  Spec: debug-vendored/component-model/design/mvp/Binary.md (search for [topic])
  Wasmtime: debug-vendored/wasmtime/crates/wasmparser/src/... (search for [topic])
  Wazero decoder: internal/component/binary/... (find where validation should occur)

BEFORE WRITING CODE:
  Follow the existence verification process — search for existing validation
  code that might handle this case under a different name.

FIX AT SOURCE: Add the validation to the decoder. Do not add skip logic.

NON-NEGOTIABLE: [paste requirements section]

After fixing, run:
  go test ./internal/component/spectest/ -run TestSimpleWast -v
```

### Dispatching Reviewer Subagents

After each worker completes, dispatch TWO reviewers in parallel:

**Spec Conformance Reviewer prompt:**
```
REVIEW this change for spec conformance.

The worker fixed [test name] by [description of change].

Files changed: [list]

Your job:
1. Read the ACTUAL spec at [spec file path]
2. Read the changed code
3. Verify the change matches the spec EXACTLY
4. Check for any spec requirements the change MISSED
5. Check that no non-spec behavior was introduced

Report: CONFORMANT or NON-CONFORMANT with specific citations.
```

**Code Quality Reviewer prompt:**
```
REVIEW this change for code quality and codebase consistency.

The worker fixed [test name] by [description of change].

Files changed: [list]

Your job:
1. Check for duplicate types/functions/fields — search the codebase
2. Check for TODOs, skip logic, workarounds, mocks, stubs
3. Check that fixes are at source (not in test code or wrapper layers)
4. Check for import cycle risks
5. Check that existing code was reused where possible
6. Check that no regressions were introduced in other test files

Report: CLEAN or ISSUES with specific findings.
```

### Progress Tracking

After each fix cycle, log:
```
Fix #N:
  Test: [name]
  Before: X failures
  After:  Y failures (fixed Z, regressed W)
  Root cause: [description]
  Spec ref: [citation]
  Files changed: [list]
  Spec review: CONFORMANT / NON-CONFORMANT
  Code review: CLEAN / ISSUES
  Commit: [hash]
```

### Batching Related Failures

When multiple tests fail for the SAME root cause (e.g., 26 tests fail because of missing core sort 0x11), dispatch ONE worker to fix the root cause, then verify ALL affected tests pass. Do not dispatch 26 separate workers for the same bug.

To identify batches:
```bash
# Group failures by error message
go test ./internal/component/spectest/ -v -count=1 2>&1 | grep "FAIL:" -A3 | grep "resources_test.go" | sed 's/.*: //' | sort | uniq -c | sort -rn | head -20
```

### When to Stop

Stop when:
```bash
go test ./internal/component/spectest/ -count=1 2>&1 | grep -c "FAIL"
```
returns `0` (the top-level FAIL for the package doesn't count if all subtests pass).

And:
```bash
go test ./internal/component/spectest/ -v -count=1 2>&1 | grep "SKIP" | grep -v "async\|exception-handling"
```
returns no results (the only skips are async or exception-handling).

---

## Spec Authorities (for all agents)

**Canonical ABI (the rulebook):**
- `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py` — reference implementation
- `debug-vendored/component-model/design/mvp/CanonicalABI.md` — spec prose

**Binary Format:**
- `debug-vendored/component-model/design/mvp/Binary.md` — binary encoding spec

**Component Model:**
- `debug-vendored/component-model/design/mvp/Explainer.md` — overview and semantics
- `debug-vendored/component-model/design/mvp/WIT.md` — WIT interface types
- `debug-vendored/component-model/design/mvp/Linking.md` — linking semantics

**Wasmtime (reference runtime):**
- `debug-vendored/wasmtime/crates/wasmparser/` — binary decoder/validator
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/` — runtime
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/vm/component/` — VM layer
- `debug-vendored/wasmtime/crates/environ/src/component/` — type machinery
- `debug-vendored/wasmtime/crates/c-api/include/wasmtime/component/` — C API
- `debug-vendored/wasmtime/crates/wast/src/wast.rs` — .wast test runner
- `debug-vendored/wasmtime/tests/all/component_model/` — integration tests

**Wazero (our codebase):**
- `internal/component/binary/` — binary decoder
- `internal/component/runtime/` — runtime state (ComponentInstance, Table, ResourceType)
- `internal/component/types/` — type definitions (ValType, ComponentTypes)
- `internal/component/abi/` — lift/lower implementation
- `internal/component/` — linker, instance, exported functions
- `internal/component/spectest/` — .wast test runner
- `internal/integration_test/spectest/` — existing core wasm .wast runner (study for patterns)

---

## Starting Point

1. Run the full suite to establish baseline
2. Start with **TestSimpleWast** — 6 pass, 2 fail. The simplest test file with failures.
3. After fixing TestSimpleWast, move to Priority 1 (runner infrastructure)
4. After each priority level, re-run full suite and re-categorize remaining failures
5. Continue until zero failures

---

## Target

```
Pass: 1131+ | Fail: 0 | Skip: 1 (tags.wast exception-handling only)
```

All 60 .wast test files execute. All non-async, non-exception-handling subtests pass.
