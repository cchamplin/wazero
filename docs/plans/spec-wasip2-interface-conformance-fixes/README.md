# WASI Preview 2 Interface Conformance Fixes

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement each phase.

**Goal:** Bring wazero's WASI Preview 2 implementation into full conformance with the official WASI 0.2.x specification.

**Architecture:** Fix gaps identified in the gap analysis by implementing missing functionality in priority order. Each phase focuses on a logical grouping of related fixes. Phases are designed to be independent where possible, allowing parallel work.

**Tech Stack:** Go 1.21+, wazero component model, WASI Preview 2 WIT specifications

---

## Reference Materials

### Specification Sources
- **WASI Proposals:** `debug-vendored/WASI/proposals/`
- **WIT Definitions:** Each proposal's `wit/` subdirectory
- **Current Implementation:** `imports/wasip2/`

### Wasmtime Reference (for accelerating development)
- **Wasmtime Host:** `debug-vendored/wasmtime/crates/wasi/src/host/`
- **Wasmtime Preview2:** `debug-vendored/wasmtime/crates/wasi/src/preview2/`

### Gap Analysis
- **Source Document:** [`docs/plans/wasip2-interface-gap-analysis.md`](../wasip2-interface-gap-analysis.md)

---

## Regression Requirement

**CRITICAL:** After completing each phase, verify that the calculator tests still pass:

```bash
go test -v ./internal/component/wasip2test/... -run "TestCalculator"
```

**Expected:** Tests for `add` and `subtract` plugins must pass. These tests exercise:
- wasi:cli/environment (args/env)
- wasi:io/streams (stdin/stdout)
- Basic resource table operations

---

## Phase Overview

| Phase | Focus Area | Priority | Status | Document |
|-------|------------|----------|--------|----------|
| 1 | wasi:io/poll - Blocking I/O Foundation | HIGH | [ ] Not Started | [phase-1-io-poll.md](./phase-1-io-poll.md) |
| 2 | wasi:io/streams - Blocking & Splice | MEDIUM | [ ] Not Started | [phase-2-io-streams.md](./phase-2-io-streams.md) |
| 3 | wasi:sockets/udp - UDP Socket Implementation | HIGH | [ ] Not Started | [phase-3-sockets-udp.md](./phase-3-sockets-udp.md) |
| 4 | wasi:sockets/ip-name-lookup - DNS Resolution | HIGH | [ ] Not Started | [phase-4-ip-name-lookup.md](./phase-4-ip-name-lookup.md) |
| 5 | wasi:http/outgoing-handler - HTTP Client | HIGH | [ ] Not Started | [phase-5-http-handler.md](./phase-5-http-handler.md) |
| 6 | Resource Lifecycle - Destructors & Cleanup | MEDIUM | [ ] Not Started | [phase-6-resource-lifecycle.md](./phase-6-resource-lifecycle.md) |
| 7 | Error Handling - Debug Strings & Mapping | LOW | [ ] Not Started | [phase-7-error-handling.md](./phase-7-error-handling.md) |

---

## Progress Tracking

### Phase 1: wasi:io/poll
- [ ] Task 1.1: Implement proper `pollable.block` method
- [ ] Task 1.2: Implement correct `poll` function for multiple pollables
- [ ] Task 1.3: Add pollable ready state tracking
- [ ] **Phase 1 Checkpoint: Run calculator tests**

### Phase 2: wasi:io/streams
- [ ] Task 2.1: Implement `blocking-read` with actual blocking
- [ ] Task 2.2: Implement `blocking-skip` with actual blocking
- [ ] Task 2.3: Implement `blocking-write-and-flush` with actual blocking
- [ ] Task 2.4: Implement `splice` operation
- [ ] Task 2.5: Implement `blocking-splice` operation
- [ ] Task 2.6: Fix `check-write` to return actual buffer capacity
- [ ] **Phase 2 Checkpoint: Run calculator tests**

### Phase 3: wasi:sockets/udp
- [ ] Task 3.1: Implement UdpSocket struct with state management
- [ ] Task 3.2: Implement start-bind/finish-bind for UDP
- [ ] Task 3.3: Implement stream method for UDP
- [ ] Task 3.4: Implement incoming-datagram-stream resource
- [ ] Task 3.5: Implement outgoing-datagram-stream resource
- [ ] Task 3.6: Implement socket options (hop-limit, buffer sizes)
- [ ] Task 3.7: Implement udp-create-socket properly
- [ ] **Phase 3 Checkpoint: Run calculator tests**

### Phase 4: wasi:sockets/ip-name-lookup
- [ ] Task 4.1: Implement resolve-address-stream resource
- [ ] Task 4.2: Implement resolve-addresses function
- [ ] Task 4.3: Add async DNS resolution support
- [ ] **Phase 4 Checkpoint: Run calculator tests**

### Phase 5: wasi:http/outgoing-handler
- [ ] Task 5.1: Connect outgoing-handler to Go's http.Client
- [ ] Task 5.2: Implement request body streaming
- [ ] Task 5.3: Implement response body streaming
- [ ] Task 5.4: Add timeout and cancellation support
- [ ] **Phase 5 Checkpoint: Run calculator tests**

### Phase 6: Resource Lifecycle
- [ ] Task 6.1: Add destructors for HTTP fields resource
- [ ] Task 6.2: Add destructors for HTTP body resources
- [ ] Task 6.3: Implement automatic cleanup on scope exit
- [ ] Task 6.4: Add type-safe resource table wrapper
- [ ] **Phase 6 Checkpoint: Run calculator tests**

### Phase 7: Error Handling
- [ ] Task 7.1: Improve error.to-debug-string with context
- [ ] Task 7.2: Add stack trace preservation
- [ ] Task 7.3: Map internal errors to specific WASI error codes
- [ ] **Phase 7 Checkpoint: Run calculator tests**

---

## Execution Options

After reviewing the plans, choose an execution approach:

### Option 1: Subagent-Driven (Recommended for complex phases)
- Execute in current session
- Fresh subagent per task with code review between tasks
- Use `superpowers:subagent-driven-development` skill

### Option 2: Parallel Session
- Open new session in worktree for each phase
- Use `superpowers:executing-plans` skill
- Batch execution with phase checkpoints

### Option 3: Phase-by-Phase Manual
- Work through one phase at a time
- Manual review and testing between phases
- Most control, slower iteration

---

## Dependencies Between Phases

```
Phase 1 (poll) ─────────────────────────────────────┐
                                                    │
Phase 2 (streams) ──────────────────────────────────┼──► Foundation for all I/O
                                                    │
Phase 3 (UDP) ──────► Phase 4 (DNS) ────────────────┘
                          │
                          ▼
                    Phase 5 (HTTP) ──► Needs DNS for hostname resolution
                          │
                          ▼
                    Phase 6 (Resources) ──► Cleanup for HTTP resources
                          │
                          ▼
                    Phase 7 (Errors) ──► Polish pass

Recommended Order: 1 → 2 → 3 → 4 → 5 → 6 → 7
Parallel Option: (1, 2) in parallel, then (3, 4) in parallel, then 5 → 6 → 7
```

---

## Quick Start

1. Read the gap analysis: `docs/plans/wasip2-interface-gap-analysis.md`
2. Choose a phase to start with (Phase 1 recommended)
3. Open the phase document and follow tasks in order
4. After completing all tasks in a phase, run the calculator tests
5. If tests pass, mark phase complete and continue to next phase
6. If tests fail, debug and fix before proceeding
