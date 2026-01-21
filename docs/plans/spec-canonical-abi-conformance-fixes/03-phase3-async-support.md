# Phase 3: Async Support (Deferred)

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement async-related types and operations for full Component Model async support.

**Architecture:** New types in the type system, lifting/lowering operations, and async function flattening.

**Tech Stack:** Go, Component Model async primitives

**Status:** DEFERRED - These features are part of the async Component Model extensions and are lower priority than core functionality.

---

## Prerequisites

- Complete all Phase 1 and Phase 2 tasks
- Full understanding of Component Model async semantics
- Decision on async runtime implementation strategy

---

## Reference

- **Gap Analysis:** `docs/plans/canonical-abi-gap-analysis.md` (Sections 8-9, 12, 16)
- **Spec:** `debug-vendored/component-model/design/mvp/CanonicalABI.md`
- **Async Spec Sections:** Lines 3693-4850+ (async canon operations)

---

## Task 3.1: ErrorContext Type

**Problem:** ErrorContextType is missing from the type system. Used for passing error context information.

**Spec Reference:** CanonicalABI.md lines 1859, 1945, 2011, 2295

**Files:**
- Create: `internal/component/types/async.go`
- Modify: `internal/component/abi/lift.go`
- Modify: `internal/component/abi/lower.go`
- Test: `internal/component/types/async_test.go`

### Step 1: Create async types file

Create `internal/component/types/async.go`:

```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

// ErrorContext represents an error context value.
// Error contexts provide additional diagnostic information for errors.
// In memory: stored as i32 index into component instance's table.
type ErrorContext struct{}

func (ErrorContext) valType()          {}
func (ErrorContext) Size() uint32      { return 4 }
func (ErrorContext) Align() uint32     { return 4 }
func (ErrorContext) FlattenCount() int { return 1 }
```

### Step 2: Write tests

Create `internal/component/types/async_test.go`:

```go
package types

import "testing"

func TestErrorContextType(t *testing.T) {
	ec := ErrorContext{}

	if got := ec.Size(); got != 4 {
		t.Errorf("Size() = %d, want 4", got)
	}
	if got := ec.Align(); got != 4 {
		t.Errorf("Align() = %d, want 4", got)
	}
	if got := ec.FlattenCount(); got != 1 {
		t.Errorf("FlattenCount() = %d, want 1", got)
	}
}
```

### Step 3: Add lift/lower stubs

Add to `internal/component/abi/lift.go`:

```go
	case types.ErrorContext:
		// TODO: Implement error context lifting
		// Per spec line 2011: lift_error_context(cx, load_int(cx, ptr, 4))
		return component.Val{}, fmt.Errorf("ErrorContext lifting not yet implemented")
```

Add to `internal/component/abi/lower.go`:

```go
	case types.ErrorContext:
		// TODO: Implement error context lowering
		// Per spec line 2295: store_int(cx, lower_error_context(cx, v), ptr, 4)
		return fmt.Errorf("ErrorContext lowering not yet implemented")
```

### Step 4: Commit

```bash
git add internal/component/types/async.go internal/component/types/async_test.go \
        internal/component/abi/lift.go internal/component/abi/lower.go
git commit -m "feat(types): add ErrorContext type stub

Per Canonical ABI spec, ErrorContext has:
- Size: 4 bytes
- Align: 4 bytes
- Flatten: 1 i32 value

Lift/lower operations are stubbed pending full async implementation."
```

---

## Task 3.2: Stream Type

**Problem:** StreamType is missing from the type system. Used for async streaming data.

**Spec Reference:** CanonicalABI.md lines 175-180, 1865, 1951, 2018, 2302, 2796

**Files:**
- Modify: `internal/component/types/async.go`
- Modify: `internal/component/abi/lift.go`
- Modify: `internal/component/abi/lower.go`
- Test: `internal/component/types/async_test.go`

### Step 1: Add Stream type

Add to `internal/component/types/async.go`:

```go
// Stream represents a stream type for async data transfer.
// Streams allow incremental transfer of sequences of values.
// The element type T specifies what values flow through the stream.
// In memory: stored as i32 index (readable/writable end).
type Stream struct {
	Element ValType // The type of values in the stream (may be nil)
}

func (Stream) valType()          {}
func (Stream) Size() uint32      { return 4 }
func (Stream) Align() uint32     { return 4 }
func (Stream) FlattenCount() int { return 1 }
```

### Step 2: Add tests

Add to `internal/component/types/async_test.go`:

```go
func TestStreamType(t *testing.T) {
	// Stream of u32 values
	s := Stream{Element: U32{}}

	if got := s.Size(); got != 4 {
		t.Errorf("Size() = %d, want 4", got)
	}
	if got := s.Align(); got != 4 {
		t.Errorf("Align() = %d, want 4", got)
	}
	if got := s.FlattenCount(); got != 1 {
		t.Errorf("FlattenCount() = %d, want 1", got)
	}
}

func TestStreamTypeNoElement(t *testing.T) {
	// Stream with no element type (byte stream)
	s := Stream{Element: nil}

	if got := s.Size(); got != 4 {
		t.Errorf("Size() = %d, want 4", got)
	}
}
```

### Step 3: Add lift/lower stubs

Add to `internal/component/abi/lift.go`:

```go
	case types.Stream:
		// TODO: Implement stream lifting
		// Per spec line 2018: lift_stream(cx, load_int(cx, ptr, 4), t)
		return component.Val{}, fmt.Errorf("Stream lifting not yet implemented")
```

Add to `internal/component/abi/lower.go`:

```go
	case types.Stream:
		// TODO: Implement stream lowering
		// Per spec line 2302: store_int(cx, lower_stream(cx, v, t), ptr, 4)
		return fmt.Errorf("Stream lowering not yet implemented")
```

### Step 4: Commit

```bash
git add internal/component/types/async.go internal/component/types/async_test.go \
        internal/component/abi/lift.go internal/component/abi/lower.go
git commit -m "feat(types): add Stream type stub

Per Canonical ABI spec, Stream<T> has:
- Size: 4 bytes
- Align: 4 bytes
- Flatten: 1 i32 value (readable/writable end index)

Lift/lower operations are stubbed pending full async implementation."
```

---

## Task 3.3: Future Type

**Problem:** FutureType is missing from the type system. Used for async single-value promises.

**Spec Reference:** CanonicalABI.md lines 175-180, 1865, 1951, 2019, 2303, 2796

**Files:**
- Modify: `internal/component/types/async.go`
- Modify: `internal/component/abi/lift.go`
- Modify: `internal/component/abi/lower.go`
- Test: `internal/component/types/async_test.go`

### Step 1: Add Future type

Add to `internal/component/types/async.go`:

```go
// Future represents a future type for async single-value transfer.
// Futures represent a value that will be available at some point.
// The element type T specifies the eventual value type.
// In memory: stored as i32 index (readable/writable end).
type Future struct {
	Element ValType // The type of the eventual value (may be nil)
}

func (Future) valType()          {}
func (Future) Size() uint32      { return 4 }
func (Future) Align() uint32     { return 4 }
func (Future) FlattenCount() int { return 1 }
```

### Step 2: Add tests

Add to `internal/component/types/async_test.go`:

```go
func TestFutureType(t *testing.T) {
	// Future of string value
	f := Future{Element: String{}}

	if got := f.Size(); got != 4 {
		t.Errorf("Size() = %d, want 4", got)
	}
	if got := f.Align(); got != 4 {
		t.Errorf("Align() = %d, want 4", got)
	}
	if got := f.FlattenCount(); got != 1 {
		t.Errorf("FlattenCount() = %d, want 1", got)
	}
}

func TestFutureTypeNoElement(t *testing.T) {
	// Future with no element (unit future)
	f := Future{Element: nil}

	if got := f.Size(); got != 4 {
		t.Errorf("Size() = %d, want 4", got)
	}
}
```

### Step 3: Add lift/lower stubs

Add to `internal/component/abi/lift.go`:

```go
	case types.Future:
		// TODO: Implement future lifting
		// Per spec line 2019: lift_future(cx, load_int(cx, ptr, 4), t)
		return component.Val{}, fmt.Errorf("Future lifting not yet implemented")
```

Add to `internal/component/abi/lower.go`:

```go
	case types.Future:
		// TODO: Implement future lowering
		// Per spec line 2303: store_int(cx, lower_future(cx, v, t), ptr, 4)
		return fmt.Errorf("Future lowering not yet implemented")
```

### Step 4: Commit

```bash
git add internal/component/types/async.go internal/component/types/async_test.go \
        internal/component/abi/lift.go internal/component/abi/lower.go
git commit -m "feat(types): add Future type stub

Per Canonical ABI spec, Future<T> has:
- Size: 4 bytes
- Align: 4 bytes
- Flatten: 1 i32 value (readable/writable end index)

Lift/lower operations are stubbed pending full async implementation."
```

---

## Task 3.4: Async Function Flattening (Future Work)

**Problem:** Async functions have different flattening rules with MAX_FLAT_ASYNC_PARAMS = 4.

**Spec Reference:** CanonicalABI.md lines 2736-2768

**Status:** Deferred - requires full async infrastructure

### Design Notes

```go
// Constants for async flattening
const (
	MaxFlatParams      = 16
	MaxFlatAsyncParams = 4  // Lower limit for async functions
	MaxFlatResults     = 1
)

// FlattenFuncType would need to take async flag:
// func FlattenFuncType(params, results []ValType, async bool) CoreFuncType
```

### When to Implement

Implement when:
1. Full async runtime is designed
2. Task/Subtask infrastructure is in place
3. Waitable set primitives are implemented

---

## Phase 3 Completion Checklist

This phase is DEFERRED. When implementing:

1. Run full type tests:
```bash
go test -v ./internal/component/types/...
```

2. Run full ABI tests:
```bash
go test -v ./internal/component/abi/...
```

3. Run regression tests:
```bash
CGO_ENABLED=0 go test -v ./internal/component/wasip2test/... -run "TestCalculatorPlugins"
```

4. Add async-specific integration tests

5. Update progress in `00-overview.md`

---

## Additional Async Canon Operations (Reference)

For complete async support, these canon operations also need implementation:

| Operation | Spec Lines | Purpose |
|-----------|------------|---------|
| `canon context.get` | 3693-3713 | Get async context value |
| `canon context.set` | 3714-3735 | Set async context value |
| `canon backpressure.set` | 3736-3759 | Control backpressure |
| `canon task.return` | 3788-3838 | Return from async task |
| `canon task.cancel` | 3839-3867 | Cancel async task |
| `canon waitable-set.*` | 3868-3988 | Waitable set operations |
| `canon subtask.*` | 4020-4109 | Subtask management |
| `canon stream.*` | 4110-4423 | Stream read/write/cancel |
| `canon future.*` | 4257-4463 | Future read/write/cancel |
| `canon thread.*` | 4464-4679 | Thread primitives |
| `canon error-context.*` | 4683-4773 | Error context operations |

These are extensive and require careful design of the async runtime before implementation.
