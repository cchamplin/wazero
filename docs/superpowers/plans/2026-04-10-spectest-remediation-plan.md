# Spectest Remediation Plan — Iterative Fix Loop

> **For agentic workers:** This plan uses a priority-ordered iterative fix loop. Launch a subagent for each failing test or batch of related failures. Verify fixes yourself before proceeding. Do not trust agent claims of completion without running the test.

**Goal:** All 60 .wast test files pass (except tags.wast skip for exception-handling). Zero failures. Every test either passes or has an async spec citation.

**Current baseline:** Pass: 295 | Fail: 836 | Skip: 1 (tags.wast exception-handling)

**Repo:** /home/cchamplin/development/wazero  
**Branch:** feat/wasip2-complete-implementation

---

## Approach: Priority-Ordered Iterative Fix Loop

Work through failures in priority order. Each priority level unblocks tests in the next level. Within each level, start with the simplest test file (fewest commands, simplest component) and work up.

### Priority 1 — Spectest Runner Infrastructure (unblocks ~300+ tests)

These are runner bugs, not implementation bugs. Fixing them reveals the real implementation gaps.

**1a. "no current instance" (~140 failures)**

The runner doesn't properly track the "current instance" across module→assert sequences. When a `module` command instantiates a component, subsequent `invoke`/`assert_return`/`assert_trap` commands should target that instance. The runner loses state. Fix state tracking in `internal/component/spectest/resources_test.go`.

**1b. Missing command types**

The runner doesn't handle `assert_unlinkable` and `assert_malformed` .wast command types. They fall through as unknowns and break subsequent state tracking. Implement handlers:
- `assert_unlinkable`: compile + instantiate, expect instantiation to fail
- `assert_malformed`: expect compilation to fail with a specific error

**1c. `register` command cross-component import wiring**

When a component is `register`ed under a name, later components that `import` it should resolve via the registered instance. The runner's `register` implementation likely doesn't wire into the ComponentLinker's definitions. Fix by adding registered instances as import definitions on the linker.

### Priority 2 — Binary Decoder Gaps (unblocks ~50+ tests)

These fail at CompileComponent — the component binary can't even be parsed.

**2a. Unknown core sort `0x11` (~26 failures)**

Core module sort in export/alias sections. The binary decoder at `internal/component/binary/` doesn't recognize sort `0x11`. Read `debug-vendored/component-model/design/mvp/Binary.md` for the core sort encoding.

**2b. Unknown sort `0x636f7265` (~8 failures)**

The `core` prefix (`0x636f7265` = ASCII "core") in alias sorts. The decoder doesn't handle the two-level sort encoding where `core` prefixes core-level sorts.

**2c. Unsupported core type opcodes `0x03`, `0x04` (~12 failures)**

Missing core type opcodes in module type decoding. Read the binary format spec for core type section encoding.

**2d. Other decoder errors**

Any remaining `decode component:` errors found after 2a-2c. Process each one by reading the binary format spec.

### Priority 3 — Instantiation Pipeline (unblocks ~70+ tests)

Components parse but can't instantiate.

**3a. Nested component instantiation with core modules (~70 failures)**

"requires CompiledComponent for nested components" — nested components that contain core modules can't be instantiated because the nested component isn't compiled as a full `CompiledComponent`. The nested instantiation path needs to compile and instantiate nested components' core modules.

**3b. Module re-instantiation (~20 failures)**

"module[m] has already been instantiated" — core modules can't be instantiated twice with the same name. The component model requires creating fresh instances of the same module. The runtime needs to support multiple instantiations of the same compiled module under different names or with instance isolation.

**3c. Import resolution for registered instances (~30+ failures)**

Components importing "host" or other registered instances can't resolve them. This connects to Priority 1c — the `register` command must wire into the linker so imports resolve.

### Priority 4 — ABI/Runtime Correctness (remaining ~200+ tests)

Components instantiate but function calls fail or produce wrong results.

**4a. String encoding (big-strings, string-transcode)**

Cross-encoding string round-trips. Read `definitions.py` string lift/lower sections.

**4b. Post-return semantics**

trap-in-post-return, error-context-trap-in-post-return tests.

**4c. Type and validation mismatches**

Type export restrictions, kebab naming, defined types validation.

**4d. Everything else**

Process remaining failures one by one.

---

## For Each Failing Test, Follow This Process

1. **Run the specific failing test** to get the exact error:
   ```bash
   go test ./internal/component/spectest/ -run TestName/subtest -v
   ```

2. **Read the failing test** — understand what the .wast command expects to happen

3. **Read the actual error, trace to production code:**
   - If decoder error → trace to `internal/component/binary/`
   - If instantiation error → trace to `internal/component/component_linker.go`
   - If runtime error → trace to `internal/component/abi/` or `instance.go`
   - If runner error → trace to `internal/component/spectest/resources_test.go`

4. **Read the spec:**
   - Binary format: `debug-vendored/component-model/design/mvp/Binary.md`
   - Canonical ABI: `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py`
   - Canonical ABI prose: `debug-vendored/component-model/design/mvp/CanonicalABI.md`
   - Component model: `debug-vendored/component-model/design/mvp/Explainer.md`
   - WIT: `debug-vendored/component-model/design/mvp/WIT.md`
   - Linking: `debug-vendored/component-model/design/mvp/Linking.md`

5. **Read wasmtime's implementation as reference:**
   - Binary decoder/validator: `debug-vendored/wasmtime/crates/wasmparser/`
   - Runtime component layer: `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/`
   - VM component layer: `debug-vendored/wasmtime/crates/wasmtime/src/runtime/vm/component/`
   - Environ/types: `debug-vendored/wasmtime/crates/environ/src/component/`
   - C API: `debug-vendored/wasmtime/crates/c-api/include/wasmtime/component/`
   - Wasmtime tests: `debug-vendored/wasmtime/tests/all/component_model/`

6. **Grep the codebase for existing implementations** before writing anything:
   ```bash
   grep -rn "pattern" internal/component/ --include="*.go"
   ```
   Check `binary/`, `runtime/`, `types/`, `abi/`, `component/` for types/functions/fields that already handle this case, even under different names.

7. **Fix at source**, reusing existing code paths. Do not create new types or functions if equivalent ones exist.

8. **Verify fix matches spec**, no duplicate types/functions/fields introduced.

9. **Run the specific test**, confirm it passes:
   ```bash
   go test ./internal/component/spectest/ -run TestName -v
   ```

10. **Run the full spectest suite**, confirm no regressions:
    ```bash
    go test ./internal/component/spectest/ -count=1 2>&1 | grep -c "FAIL"
    ```

11. **Commit** with a clear message citing the spec section.

---

## What Is NOT Allowed

- Changing what a test expects
- Adding skip/workaround logic in test code
- Adding hardcoded skip lists or error-string matching to bypass failures
- Making a test pass by doing something the spec doesn't call for
- Guessing at behavior without reading the vendored spec/wasmtime sources
- Creating duplicate types or functions that already exist
- Working around decoder/runtime bugs instead of fixing them
- Using `context.Background()` for caller contexts
- Claiming "done" without the orchestrator independently verifying

---

## What the Orchestrator Verifies After Each Agent Completes

- Run the specific test and see it pass
- Spot-check the production fix against the spec reference cited
- Run the full suite to confirm no regressions
- Count remaining failures and compare to previous count

---

## Starting Point

Start with the simplest passing-to-failing transition:

1. **TestSimpleWast** — 8 subtests: 6 pass, 2 fail. Fix those 2 first. They will reveal the simplest root cause.

2. Then work through **Priority 1** (runner infrastructure):
   - Fix state tracking
   - Add missing command types
   - Wire register command

3. After Priority 1, re-run full suite and re-categorize remaining failures.

4. Then **Priority 2** (decoder), re-run and re-categorize.

5. Then **Priority 3** (pipeline), re-run and re-categorize.

6. Then **Priority 4** (ABI/runtime), one test at a time.

After each fix, report:
```
Before: X failures
After:  Y failures (fixed Z)
Root cause: [description]
Spec ref: [citation]
Files changed: [list]
```

---

## Target

All 60 .wast test files pass (except tags.wast skip for exception-handling proposal).
Zero failures. Every test either passes or has an async spec citation.
Pass: 1131+ | Fail: 0 | Skip: 1
