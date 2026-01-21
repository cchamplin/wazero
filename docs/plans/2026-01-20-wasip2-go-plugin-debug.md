# WASIP2 Go Plugin Debug Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Find and fix the root cause preventing the Go WASI Preview 2 component plugin from working while keeping existing C/Rust plugins and calculator tests green.

**Architecture:** Reproduce the failing Go plugin test, trace component instantiation through wazero’s component model/WASI P2 code paths, compare against working C/Rust plugin flows, and validate fixes with targeted tests. Maintain DEBUGGING-NOTES.md with discoveries.

**Tech Stack:** Go, wazero component model, WASI Preview 2, wasm-tools (wastime), Go test.

### Task 1: Establish baseline and reproduction

**Files:** DEBUGGING-NOTES.md (new)

**Step 1: Run targeted tests to reproduce failure**
```bash
CGO_ENABLED=0 go test ./internal/component/wasip2test -run TestCalculatorGoPlugin -v
```
Expected: Repro of failing Go plugin behavior.

**Step 2: Record observations**
Add error output and environment notes to `DEBUGGING-NOTES.md`.

### Task 2: Inspect plugin artifacts

**Files:** DEBUGGING-NOTES.md

**Step 1: Locate plugin wasm files (C, Rust, Go)**
```bash
rg --files -g "*calculator*.wasm" internal/component/wasip2test
```

**Step 2: Disassemble/inspect metadata**
```bash
wasm-tools component print <path> | head -n 200
wasm-tools component metadata <path>
```
Capture key differences (imports/exports/types) in notes.

### Task 3: Trace wazero instantiation paths

**Files:** DEBUGGING-NOTES.md

**Step 1: Identify code paths for component loading and WASI P2 adaptation**
```bash
rg "wasip2" internal -g"*.go"
rg "component" internal/component -g"*.go"
```

**Step 2: Map call flow for working plugins vs Go plugin test**
Write sequence diagrams/notes in `DEBUGGING-NOTES.md` (no code changes yet).

### Task 4: Compare runtime differences (C/Rust vs Go plugin)

**Files:** DEBUGGING-NOTES.md

**Step 1: Capture runtime logs for working plugins**
```bash
CGO_ENABLED=0 go test ./internal/component/wasip2test -run TestCalculatorCPlugin -v
CGO_ENABLED=0 go test ./internal/component/wasip2test -run TestCalculatorRustPlugin -v
```

**Step 2: Add temporary verbose tracing if needed**
Plan minimal log points; note prospective insertion sites in notes (no code yet).

### Task 5: Form hypotheses and create failing focused test (if needed)

**Files:** internal/component/wasip2test/calculator_test.go (modify), DEBUGGING-NOTES.md

**Step 1: Form single root-cause hypothesis in notes.**

**Step 2: Add/adjust minimal test to expose issue (if reproduction insufficient).**
Run: `CGO_ENABLED=0 go test ./internal/component/wasip2test -run TestCalculatorGoPlugin -v`

### Task 6: Implement minimal fix once root cause known

**Files:** (TBD based on investigation), DEBUGGING-NOTES.md

**Step 1: Implement smallest change addressing root cause (one change at a time).**

**Step 2: Run focused and full relevant tests**
```bash
CGO_ENABLED=0 go test ./internal/component/wasip2test -run TestCalculatorGoPlugin -v
CGO_ENABLED=0 go test ./internal/component/wasip2test -run TestCalculator(C|Rust)Plugin -v
CGO_ENABLED=0 go test ./internal/component/wasip2test -v
```

**Step 3: Document fix and evidence in `DEBUGGING-NOTES.md`.**

### Task 7: Clean up instrumentation and finalize

**Files:** Modified source files, DEBUGGING-NOTES.md

**Step 1: Remove temporary debugging logs/traces.**

**Step 2: Run verification suite**
```bash
CGO_ENABLED=0 go test ./internal/component/wasip2test -v
```

**Step 3: Summarize outcome and next steps in notes.**
