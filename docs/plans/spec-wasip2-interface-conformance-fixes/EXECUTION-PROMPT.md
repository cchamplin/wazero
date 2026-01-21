# WASI P2 Interface Conformance Fixes - Execution Prompt

> **Copy this entire file content to start a new Claude Code session for subagent-driven development.**

---

## Session Initialization

I need to execute the WASI Preview 2 interface conformance fixes plan using subagent-driven development.

**REQUIRED:** Use the `superpowers:subagent-driven-development` skill to execute this plan.

---

## Project Context

**Repository:** wazero - A zero-dependency WebAssembly runtime for Go
**Branch:** wasip2
**Working Directory:** `/Users/cchamplin/go/src/github.com/cchamplin/wazero`

**Objective:** Fix gaps in the WASI Preview 2 implementation to achieve spec conformance.

---

## Plan Location

All plan documents are in: `docs/plans/spec-wasip2-interface-conformance-fixes/`

- **Root Tracker:** `README.md` - Overall progress tracking
- **Gap Analysis:** `docs/plans/wasip2-interface-gap-analysis.md` - Source analysis

### Phase Documents (execute in order):

1. `phase-1-io-poll.md` - Blocking I/O foundation (HIGH priority)
2. `phase-2-io-streams.md` - Stream blocking & splice operations (MEDIUM priority)
3. `phase-3-sockets-udp.md` - UDP socket implementation (HIGH priority)
4. `phase-4-ip-name-lookup.md` - DNS resolution (HIGH priority)
5. `phase-5-http-handler.md` - HTTP client functionality (HIGH priority)
6. `phase-6-resource-lifecycle.md` - Resource destructors & cleanup (MEDIUM priority)
7. `phase-7-error-handling.md` - Error debug strings & mapping (LOW priority)

---

## Critical Regression Requirement

**After completing each PHASE (not each task), run:**

```bash
go test -v ./internal/component/wasip2test/... -run "TestCalculator"
```

**This test MUST pass (excluding div and mult which might be broken for other reasons).** It validates that the `add` and `subtract` calculator plugins still work. These tests exercise:
- wasi:cli/environment (args/env)
- wasi:io/streams (stdin/stdout)
- Basic resource table operations

If tests fail after a phase, debug and fix before proceeding to the next phase.

---

## Reference Materials

### Specification Sources (WIT files)
- `debug-vendored/WASI/proposals/io/wit/` - streams.wit, poll.wit, error.wit
- `debug-vendored/WASI/proposals/clocks/wit/` - monotonic-clock.wit, wall-clock.wit
- `debug-vendored/WASI/proposals/random/wit/` - random.wit, insecure.wit
- `debug-vendored/WASI/proposals/filesystem/wit/` - types.wit, preopens.wit
- `debug-vendored/WASI/proposals/sockets/wit/` - network.wit, tcp.wit, udp.wit, ip-name-lookup.wit
- `debug-vendored/WASI/proposals/cli/wit/` - environment.wit, terminal.wit
- `debug-vendored/WASI/proposals/http/wit/` - types.wit, handler.wit

### Current Implementation (Go files to modify)
- `imports/wasip2/io/` - streams.go, poll.go, error.go
- `imports/wasip2/clocks/` - monotonic.go, wall.go
- `imports/wasip2/random/` - random.go, insecure.go
- `imports/wasip2/filesystem/` - filesystem.go, preopens.go, types.go
- `imports/wasip2/sockets/` - sockets.go, tcp.go (udp.go to create)
- `imports/wasip2/cli/` - cli.go
- `imports/wasip2/http/` - http.go
- `internal/component/` - resource_table.go, val.go

### Wasmtime Reference (for implementation patterns)
- `debug-vendored/wasmtime/crates/wasi/src/host/` - Reference implementations
- `debug-vendored/wasmtime/crates/wasi/src/preview2/` - Preview 2 specific code

---

## Execution Workflow

### For Each Phase:

1. **Read the phase document** to understand all tasks
2. **For each task in the phase:**
   - Read the task details (files, steps)
   - Write the failing test first (TDD)
   - Run test to verify it fails
   - Implement minimal code to pass
   - Run test to verify it passes
   - Commit with descriptive message
3. **After all tasks in phase complete:**
   - Run calculator regression tests
   - If pass: Update README.md to mark phase complete
   - If fail: Debug and fix before next phase

### Task Structure (from plan documents):

Each task follows this pattern:
```
### Task N.M: [Component Name]

**Files:**
- Create/Modify: exact file paths
- Test: test file path

**Step 1: Write the failing test**
[Test code provided]

**Step 2: Run test to verify it fails**
[Command and expected output]

**Step 3: Write minimal implementation**
[Implementation code provided]

**Step 4: Run test to verify it passes**
[Command and expected output]

**Step 5: Commit**
[Git command with message]
```

---

## Key Implementation Notes

### Phase 1 (Poll) - Foundation
- Pollable needs `ready` channel for blocking
- `poll` function must use `reflect.Select` for dynamic channel selection
- All subsequent phases depend on this

### Phase 2 (Streams) - Blocking I/O
- Go's io.Reader/io.Writer naturally block
- `splice` uses `io.CopyN`
- `check-write` should check for `Available()` interface

### Phase 3 (UDP) - New Implementation
- Mirror TcpSocket state machine pattern
- Create `udp.go` as new file
- IncomingDatagramStream/OutgoingDatagramStream are new resources

### Phase 4 (DNS) - New Implementation
- Use `net.Resolver.LookupIP` with context timeout
- ResolveAddressStream iterates over results
- Handle both IPv4 and IPv6

### Phase 5 (HTTP) - Connect Existing Types
- HTTP types already exist, just need to wire `handle` function
- Use `http.Client` for actual requests
- FutureIncomingResponse wraps async response

### Phase 6 (Resources) - Cleanup
- Add `Destroyable` interface to `internal/component`
- Implement `Destroy()` on all resources that need cleanup
- `Delete()` in ResourceTable calls destructor for owned resources

### Phase 7 (Errors) - Polish
- Enhanced `Error` type with source and stack trace
- Map `os` and `syscall` errors to WASI error codes
- Update all error returns to use mappers

---

## Git Commit Convention

All commits should follow this pattern:
```
feat(wasip2): <short description>

<longer description if needed>

Ref: docs/plans/wasip2-interface-gap-analysis.md Section X.Y
```

Co-author line will be added automatically.

---

## Starting Point

**Recommended:** Start with Phase 1, Task 1.1

```bash
# First, read the phase 1 document
cat docs/plans/spec-wasip2-interface-conformance-fixes/phase-1-io-poll.md

# Verify current test status before starting
go test -v ./internal/component/wasip2test/... -run "TestCalculator"
```

---

## Questions to Resolve During Execution

If you encounter any of these situations, pause and investigate:

1. **Import path issues** - The codebase uses `github.com/tetratelabs/wazero` imports
2. **Missing component.Val methods** - Check `internal/component/val.go` for available methods
3. **Resource table API** - Check `internal/component/resource_table.go` for current API
4. **Test failures unrelated to changes** - May indicate existing issues; document and continue

---

## Success Criteria

Phase completion requires:
- [ ] All tasks in phase implemented with tests passing
- [ ] Calculator regression tests pass
- [ ] Code committed with appropriate messages
- [ ] README.md updated to mark phase complete

Full plan completion requires:
- [ ] All 7 phases complete
- [ ] All tests pass: `go test -v ./...`
- [ ] Gap analysis HIGH priority items addressed
- [ ] No new regressions introduced

---

**Begin execution by invoking the `superpowers:subagent-driven-development` skill.**
