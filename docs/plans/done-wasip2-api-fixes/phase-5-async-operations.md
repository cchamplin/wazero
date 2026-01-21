# Phase 5: Async Operations (Future Work)

These tasks are for longer-term async/streaming support.

---

## Task 5.1: Parse Async Canonical Operations

**Status:** PENDING

**Files:**
- Modify: `internal/component/binary/canonical.go`
- Test: `internal/component/binary/canonical_test.go`

**Add support for parsing:**
- task.return, task.cancel, task.poll, task.wait, task.yield
- subtask.cancel, subtask.drop
- stream.new, stream.read, stream.write, stream.cancel-read, stream.cancel-write, stream.close-readable, stream.close-writable
- future.new, future.read, future.write, future.cancel-read, future.cancel-write, future.close-readable, future.close-writable
- error-context.new, error-context.debug-message, error-context.drop
- waitable-set.new, waitable-set.wait, waitable-set.poll, waitable-set.drop, waitable-set.subscribe

---

## Task 5.2: Implement Stream Types Runtime

**Status:** PENDING

**Files:**
- Create: `internal/component/stream.go`
- Test: `internal/component/stream_test.go`

---

## Task 5.3: Implement Future Types Runtime

**Status:** PENDING

**Files:**
- Create: `internal/component/future.go`
- Test: `internal/component/future_test.go`
