# Phase 5: Async Canonicals (Gated Features)

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add parsing support for all async, threading, and error-context canonical builtins.

**Architecture:** Extend canonical decoder to handle opcodes 0x05-0x42 and create appropriate data structures to store the parsed canonical definitions.

**Tech Stack:** Go

**Gap Analysis Reference:** Section 6.2 - Missing Canonicals

---

## Context

These features are gated behind feature flags in the spec:
- 🔀 Async operations (streams, futures, tasks, waitables)
- 📝 Error context operations
- 🧵 Threading operations

While runtime support may not be immediately needed, parsing support allows components using these features to be loaded without error.

---

## Reference Files

- **Spec:** `debug-vendored/component-model/design/mvp/Binary.md` (canon section)
- **wasmparser:** `debug-vendored/wasm-tools/crates/wasmparser/src/readers/component/canonicals.rs:299-433`
- **Current impl:** `internal/component/binary/canonical.go`

---

## Task 5.1: Add Task Canonicals (0x05, 0x09)

**Files:**
- Modify: `internal/component/component.go`
- Modify: `internal/component/binary/canonical.go`

**Step 1: Add new CanonKind constants**

```go
const (
    CanonKindLift         CanonKind = iota
    CanonKindLower
    CanonKindResourceNew
    CanonKindResourceDrop
    CanonKindResourceRep
    // NEW: Task operations
    CanonKindTaskCancel     // 0x05
    CanonKindTaskReturn     // 0x09
)
```

**Step 2: Add TaskReturn fields to CanonicalDef**

```go
type CanonicalDef struct {
    Kind CanonKind
    // ... existing fields ...

    // For TaskReturn
    TaskReturnType *ValTypeRef // Optional result type
}
```

**Step 3: Handle 0x05 and 0x09 in decodeCanonicalSection**

```go
case 0x05: // task.cancel
    canon.Kind = component.CanonKindTaskCancel
    // No additional data

case 0x09: // task.return
    canon.Kind = component.CanonKindTaskReturn
    // Read optional result type
    hasResult, err := r.ReadByte()
    if err != nil {
        return fmt.Errorf("read task.return result flag: %w", err)
    }
    if hasResult == 0x00 {
        // Single result type
        valType, err := decodeValType(r)
        if err != nil {
            return fmt.Errorf("decode task.return result type: %w", err)
        }
        canon.TaskReturnType = &valType
    } else if hasResult == 0x01 {
        // Named results - read count (should be 0)
        count, _, err := leb128.DecodeUint32(r)
        if err != nil {
            return fmt.Errorf("read task.return result count: %w", err)
        }
        if count != 0 {
            return fmt.Errorf("task.return with named results not supported")
        }
    }
    // Read options
    optCount, _, err := leb128.DecodeUint32(r)
    if err != nil {
        return fmt.Errorf("read task.return option count: %w", err)
    }
    canon.Options, err = decodeCanonicalOptions(r, optCount)
    if err != nil {
        return fmt.Errorf("decode task.return options: %w", err)
    }
```

**Step 4: Run build to verify**

```bash
CGO_ENABLED=0 go build ./internal/component/...
```

---

## Task 5.2: Add Context Canonicals (0x0a, 0x0b)

**Files:**
- Modify: `internal/component/component.go`
- Modify: `internal/component/binary/canonical.go`

**Step 1: Add CanonKind constants**

```go
    CanonKindContextGet  // 0x0a
    CanonKindContextSet  // 0x0b
```

**Step 2: Add ContextSlot field**

```go
type CanonicalDef struct {
    // ...
    ContextSlot uint32 // For context.get/set - slot index
}
```

**Step 3: Handle 0x0a and 0x0b**

```go
case 0x0a: // context.get
    // Read type prefix (should be 0x7f for i32)
    prefix, err := r.ReadByte()
    if err != nil {
        return fmt.Errorf("read context.get type prefix: %w", err)
    }
    if prefix != 0x7f {
        return fmt.Errorf("expected 0x7f for context.get, got 0x%02x", prefix)
    }
    canon.Kind = component.CanonKindContextGet
    canon.ContextSlot, _, err = leb128.DecodeUint32(r)
    if err != nil {
        return fmt.Errorf("read context.get slot: %w", err)
    }

case 0x0b: // context.set
    prefix, err := r.ReadByte()
    if err != nil {
        return fmt.Errorf("read context.set type prefix: %w", err)
    }
    if prefix != 0x7f {
        return fmt.Errorf("expected 0x7f for context.set, got 0x%02x", prefix)
    }
    canon.Kind = component.CanonKindContextSet
    canon.ContextSlot, _, err = leb128.DecodeUint32(r)
    if err != nil {
        return fmt.Errorf("read context.set slot: %w", err)
    }
```

---

## Task 5.3: Add Subtask Canonicals (0x06, 0x0d)

**Files:**
- Modify: `internal/component/component.go`
- Modify: `internal/component/binary/canonical.go`

**Step 1: Add CanonKind constants**

```go
    CanonKindSubtaskCancel // 0x06
    CanonKindSubtaskDrop   // 0x0d
```

**Step 2: Add Async field for subtask.cancel**

```go
type CanonicalDef struct {
    // ...
    SubtaskAsync bool // For subtask.cancel - whether to block
}
```

**Step 3: Handle 0x06 and 0x0d**

```go
case 0x06: // subtask.cancel
    canon.Kind = component.CanonKindSubtaskCancel
    asyncFlag, err := r.ReadByte()
    if err != nil {
        return fmt.Errorf("read subtask.cancel async flag: %w", err)
    }
    canon.SubtaskAsync = asyncFlag != 0

case 0x0d: // subtask.drop
    canon.Kind = component.CanonKindSubtaskDrop
    // No additional data
```

---

## Task 5.4: Add Stream Canonicals (0x0e-0x14)

**Files:**
- Modify: `internal/component/component.go`
- Modify: `internal/component/binary/canonical.go`

**Step 1: Add CanonKind constants**

```go
    CanonKindStreamNew          // 0x0e
    CanonKindStreamRead         // 0x0f
    CanonKindStreamWrite        // 0x10
    CanonKindStreamCancelRead   // 0x11
    CanonKindStreamCancelWrite  // 0x12
    CanonKindStreamDropReadable // 0x13
    CanonKindStreamDropWritable // 0x14
```

**Step 2: Add stream-specific fields**

```go
type CanonicalDef struct {
    // ...
    StreamTypeIdx uint32 // For stream operations - stream type index
    StreamAsync   bool   // For stream.cancel-* - whether to block
}
```

**Step 3: Handle 0x0e-0x14**

```go
case 0x0e: // stream.new
    canon.Kind = component.CanonKindStreamNew
    canon.StreamTypeIdx, _, err = leb128.DecodeUint32(r)
    if err != nil {
        return fmt.Errorf("read stream.new type: %w", err)
    }

case 0x0f: // stream.read
    canon.Kind = component.CanonKindStreamRead
    canon.StreamTypeIdx, _, err = leb128.DecodeUint32(r)
    if err != nil {
        return fmt.Errorf("read stream.read type: %w", err)
    }
    optCount, _, err := leb128.DecodeUint32(r)
    if err != nil {
        return fmt.Errorf("read stream.read option count: %w", err)
    }
    canon.Options, err = decodeCanonicalOptions(r, optCount)

case 0x10: // stream.write
    canon.Kind = component.CanonKindStreamWrite
    canon.StreamTypeIdx, _, err = leb128.DecodeUint32(r)
    if err != nil {
        return fmt.Errorf("read stream.write type: %w", err)
    }
    optCount, _, err := leb128.DecodeUint32(r)
    if err != nil {
        return fmt.Errorf("read stream.write option count: %w", err)
    }
    canon.Options, err = decodeCanonicalOptions(r, optCount)

case 0x11: // stream.cancel-read
    canon.Kind = component.CanonKindStreamCancelRead
    canon.StreamTypeIdx, _, err = leb128.DecodeUint32(r)
    if err != nil {
        return fmt.Errorf("read stream.cancel-read type: %w", err)
    }
    asyncFlag, err := r.ReadByte()
    if err != nil {
        return fmt.Errorf("read stream.cancel-read async: %w", err)
    }
    canon.StreamAsync = asyncFlag != 0

case 0x12: // stream.cancel-write
    canon.Kind = component.CanonKindStreamCancelWrite
    canon.StreamTypeIdx, _, err = leb128.DecodeUint32(r)
    if err != nil {
        return fmt.Errorf("read stream.cancel-write type: %w", err)
    }
    asyncFlag, err := r.ReadByte()
    if err != nil {
        return fmt.Errorf("read stream.cancel-write async: %w", err)
    }
    canon.StreamAsync = asyncFlag != 0

case 0x13: // stream.drop-readable
    canon.Kind = component.CanonKindStreamDropReadable
    canon.StreamTypeIdx, _, err = leb128.DecodeUint32(r)
    if err != nil {
        return fmt.Errorf("read stream.drop-readable type: %w", err)
    }

case 0x14: // stream.drop-writable
    canon.Kind = component.CanonKindStreamDropWritable
    canon.StreamTypeIdx, _, err = leb128.DecodeUint32(r)
    if err != nil {
        return fmt.Errorf("read stream.drop-writable type: %w", err)
    }
```

---

## Task 5.5: Add Future Canonicals (0x15-0x1b)

**Files:**
- Modify: `internal/component/component.go`
- Modify: `internal/component/binary/canonical.go`

**Step 1: Add CanonKind constants**

```go
    CanonKindFutureNew          // 0x15
    CanonKindFutureRead         // 0x16
    CanonKindFutureWrite        // 0x17
    CanonKindFutureCancelRead   // 0x18
    CanonKindFutureCancelWrite  // 0x19
    CanonKindFutureDropReadable // 0x1a
    CanonKindFutureDropWritable // 0x1b
```

**Step 2: Add FutureTypeIdx field**

```go
type CanonicalDef struct {
    // ...
    FutureTypeIdx uint32 // For future operations
    FutureAsync   bool   // For future.cancel-*
}
```

**Step 3: Handle 0x15-0x1b** (similar pattern to streams)

---

## Task 5.6: Add Error-Context Canonicals (0x1c-0x1e)

**Files:**
- Modify: `internal/component/component.go`
- Modify: `internal/component/binary/canonical.go`

**Step 1: Add CanonKind constants**

```go
    CanonKindErrorContextNew          // 0x1c
    CanonKindErrorContextDebugMessage // 0x1d
    CanonKindErrorContextDrop         // 0x1e
```

**Step 2: Handle 0x1c-0x1e**

```go
case 0x1c: // error-context.new
    canon.Kind = component.CanonKindErrorContextNew
    optCount, _, err := leb128.DecodeUint32(r)
    if err != nil {
        return fmt.Errorf("read error-context.new option count: %w", err)
    }
    canon.Options, err = decodeCanonicalOptions(r, optCount)

case 0x1d: // error-context.debug-message
    canon.Kind = component.CanonKindErrorContextDebugMessage
    optCount, _, err := leb128.DecodeUint32(r)
    if err != nil {
        return fmt.Errorf("read error-context.debug-message option count: %w", err)
    }
    canon.Options, err = decodeCanonicalOptions(r, optCount)

case 0x1e: // error-context.drop
    canon.Kind = component.CanonKindErrorContextDrop
    // No additional data
```

---

## Task 5.7: Add Waitable-Set Canonicals (0x1f-0x23)

**Step 1: Add CanonKind constants**

```go
    CanonKindWaitableSetNew  // 0x1f
    CanonKindWaitableSetWait // 0x20
    CanonKindWaitableSetPoll // 0x21
    CanonKindWaitableSetDrop // 0x22
    CanonKindWaitableJoin    // 0x23
```

**Step 2: Add fields for waitable-set operations**

```go
type CanonicalDef struct {
    // ...
    WaitableCancellable bool   // For waitable-set.wait/poll
    WaitableMemoryIdx   uint32 // For waitable-set.wait/poll
}
```

**Step 3: Handle 0x1f-0x23**

```go
case 0x1f: // waitable-set.new
    canon.Kind = component.CanonKindWaitableSetNew

case 0x20: // waitable-set.wait
    canon.Kind = component.CanonKindWaitableSetWait
    cancellable, err := r.ReadByte()
    if err != nil {
        return fmt.Errorf("read waitable-set.wait cancellable: %w", err)
    }
    canon.WaitableCancellable = cancellable != 0
    canon.WaitableMemoryIdx, _, err = leb128.DecodeUint32(r)
    if err != nil {
        return fmt.Errorf("read waitable-set.wait memory: %w", err)
    }

case 0x21: // waitable-set.poll
    canon.Kind = component.CanonKindWaitableSetPoll
    cancellable, err := r.ReadByte()
    if err != nil {
        return fmt.Errorf("read waitable-set.poll cancellable: %w", err)
    }
    canon.WaitableCancellable = cancellable != 0
    canon.WaitableMemoryIdx, _, err = leb128.DecodeUint32(r)
    if err != nil {
        return fmt.Errorf("read waitable-set.poll memory: %w", err)
    }

case 0x22: // waitable-set.drop
    canon.Kind = component.CanonKindWaitableSetDrop

case 0x23: // waitable.join
    canon.Kind = component.CanonKindWaitableJoin
```

---

## Task 5.8: Add Backpressure Canonicals (0x24-0x25)

```go
    CanonKindBackpressureInc // 0x24
    CanonKindBackpressureDec // 0x25

case 0x24: // backpressure.inc
    canon.Kind = component.CanonKindBackpressureInc

case 0x25: // backpressure.dec
    canon.Kind = component.CanonKindBackpressureDec
```

---

## Task 5.9: Add Threading Canonicals (0x26-0x42)

**Step 1: Add all threading CanonKind constants**

```go
    CanonKindThreadIndex         // 0x26
    CanonKindThreadNewIndirect   // 0x27
    CanonKindThreadSwitchTo      // 0x28
    CanonKindThreadSuspend       // 0x29
    CanonKindThreadResumeLater   // 0x2a
    CanonKindThreadYieldTo       // 0x2b
    CanonKindThreadYield         // 0x0c (already exists)
    CanonKindResourceDropAsync   // 0x07
    CanonKindThreadSpawnRef      // 0x40
    CanonKindThreadSpawnIndirect // 0x41
    CanonKindThreadAvailableParallelism // 0x42
```

**Step 2: Add threading-specific fields**

```go
type CanonicalDef struct {
    // ...
    ThreadFuncTypeIdx  uint32 // For thread.spawn-*, thread.new-indirect
    ThreadTableIdx     uint32 // For thread.spawn-indirect, thread.new-indirect
    ThreadCancellable  bool   // For thread.yield, thread.switch-to, etc.
}
```

**Step 3: Handle all threading opcodes**

Reference wasmparser lines 335-430 for exact encoding of each opcode.

---

## Task 5.10: Add Async Canonical Tests

**Files:**
- Create: `internal/component/binary/canonical_async_test.go`

**Step 1: Create test file with sample encodings**

```go
package binary

import (
    "bytes"
    "testing"

    "github.com/tetratelabs/wazero/internal/component"
)

func TestDecodeAsyncCanonicals(t *testing.T) {
    t.Run("task.cancel", func(t *testing.T) {
        input := []byte{0x05}
        r := bytes.NewReader(input)
        c := &component.Component{}

        canon, err := decodeCanonical(r, c)
        if err != nil {
            t.Fatalf("unexpected error: %v", err)
        }
        if canon.Kind != component.CanonKindTaskCancel {
            t.Errorf("Kind: got %v, want CanonKindTaskCancel", canon.Kind)
        }
    })

    t.Run("stream.new", func(t *testing.T) {
        input := []byte{0x0e, 0x05} // stream.new, type index 5
        r := bytes.NewReader(input)
        c := &component.Component{}

        canon, err := decodeCanonical(r, c)
        if err != nil {
            t.Fatalf("unexpected error: %v", err)
        }
        if canon.Kind != component.CanonKindStreamNew {
            t.Errorf("Kind: got %v, want CanonKindStreamNew", canon.Kind)
        }
        if canon.StreamTypeIdx != 5 {
            t.Errorf("StreamTypeIdx: got %d, want 5", canon.StreamTypeIdx)
        }
    })
}
```

---

## Task 5.11: Run Regression Tests and Commit

**Step 1: Run calculator regression tests**

```bash
CGO_ENABLED=0 go test ./internal/component/wasip2test/... -run "TestCalculatorPlugins/(add|subtract)" -v
```
Expected: Both add and subtract pass

**Step 2: Commit changes**

```bash
git add internal/component/component.go internal/component/binary/canonical.go internal/component/binary/canonical_async_test.go
git commit -m "feat(component): add async/threading canonical parsing (40+ opcodes)

Add parsing support for gated canonical operations:
- Task: task.cancel (0x05), task.return (0x09)
- Context: context.get (0x0a), context.set (0x0b)
- Subtask: subtask.cancel (0x06), subtask.drop (0x0d)
- Stream: stream.new/read/write/cancel-*/drop-* (0x0e-0x14)
- Future: future.new/read/write/cancel-*/drop-* (0x15-0x1b)
- Error-context: new/debug-message/drop (0x1c-0x1e)
- Waitable-set: new/wait/poll/drop/join (0x1f-0x23)
- Backpressure: inc/dec (0x24-0x25)
- Threading: index/new-indirect/switch-to/suspend/etc (0x26-0x42)

Ref: docs/plans/component-model-binary-parser-gap-analysis.md Section 6.2
Ref: debug-vendored/wasm-tools/crates/wasmparser/src/readers/component/canonicals.rs

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Verification Checklist

- [ ] All 40+ canonical opcodes have CanonKind constants
- [ ] CanonicalDef has fields for all operation-specific data
- [ ] All opcodes parse without error
- [ ] Tests cover representative sample of opcodes
- [ ] Calculator add/subtract tests pass
- [ ] Changes committed
