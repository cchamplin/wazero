# Component Model Phase 3: Resources

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Parent Plan:** [2026-01-12-component-model-implementation.md](./2026-01-12-component-model-implementation.md)
**Design Doc:** [2026-01-12-component-model-design.md](./2026-01-12-component-model-design.md)
**Previous Phase:** [Phase 2: Complete Type System](./2026-01-12-component-model-phase2-type-system.md)
**Status:** NOT STARTED
**Estimated Tasks:** 71-100

---

## Overview

This phase implements the Component Model's resource system, including generation-counted handle tables, own/borrow semantics, and destructor invocation.

**Goal:** Full resource lifecycle management with proper ownership tracking and use-after-free protection.

**Prerequisites:**
- Phase 1 complete (binary parser, primitive types)
- Phase 2 complete (all composite types, lift/lower)

---

## Phase 3 Milestones

| Milestone | Description | Success Criteria |
|-----------|-------------|------------------|
| 3.1 | Resource type parsing | Resource types parsed from binary |
| 3.2 | Handle table implementation | Generation-counted handles prevent use-after-free |
| 3.3 | Own semantics | `own<T>` handles transfer ownership |
| 3.4 | Borrow semantics | `borrow<T>` handles track active borrows |
| 3.5 | Destructor invocation | Dropped resources trigger destructors |
| 3.6 | Call scope tracking | Borrows invalidated at call boundaries |

---

## Resource Handle Architecture

From the design doc:

```go
// internal/component/resource_table.go

// Handle is a 64-bit value: upper 32 = generation, lower 32 = index
type Handle uint64

func (h Handle) Index() uint32      { return uint32(h) }
func (h Handle) Generation() uint32 { return uint32(h >> 32) }
func MakeHandle(idx, gen uint32) Handle {
    return Handle(uint64(gen)<<32 | uint64(idx))
}

type ResourceTable struct {
    entries    []resourceEntry
    freeHead   int32  // Head of free list, -1 if empty
    generation uint32 // Monotonically increasing
}

type resourceEntry struct {
    state       entryState
    generation  uint32
    data        any           // The actual resource value
    nextFree    int32         // -1 if end of free list
    borrowCount uint32        // Active borrows (must be 0 to drop)
}

func (t *ResourceTable) New(data any) Handle
func (t *ResourceTable) Rep(h Handle) (any, error)
func (t *ResourceTable) Drop(h Handle) (any, error)
func (t *ResourceTable) Borrow(h Handle) (Handle, error)
func (t *ResourceTable) EndBorrow(h Handle) error

// Borrow tracking per call
func (t *ResourceTable) EnterCall()
func (t *ResourceTable) ExitCall()
```

---

## Tasks

### Task 71: Define Resource Type Structures

**Files:**
- Create: `internal/component/types/resource.go`
- Create: `internal/component/types/resource_test.go`

**Step 1: Write failing test**

```go
// internal/component/types/resource_test.go

package types

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestOwnType(t *testing.T) {
	// own<file> has size 4 (handle index), align 4
	o := Own{ResourceIdx: 0}
	require.Equal(t, uint32(4), o.Size())
	require.Equal(t, uint32(4), o.Align())
	require.Equal(t, 1, o.FlattenCount())
}

func TestBorrowType(t *testing.T) {
	// borrow<file> same layout as own
	b := Borrow{ResourceIdx: 0}
	require.Equal(t, uint32(4), b.Size())
	require.Equal(t, uint32(4), b.Align())
	require.Equal(t, 1, b.FlattenCount())
}
```

**Step 2: Implement**

```go
// internal/component/types/resource.go

package types

// ResourceType represents a resource type definition.
type ResourceType struct {
	// Destructor is the function index of the destructor (may be nil).
	Destructor *uint32
}

// Own represents an owning handle to a resource.
type Own struct {
	ResourceIdx uint32 // Index of the resource type
}

func (Own) valType() {}
func (Own) Size() uint32     { return 4 } // Handle is i32
func (Own) Align() uint32    { return 4 }
func (Own) FlattenCount() int { return 1 }

// Borrow represents a borrowed handle to a resource.
type Borrow struct {
	ResourceIdx uint32 // Index of the resource type
}

func (Borrow) valType() {}
func (Borrow) Size() uint32     { return 4 }
func (Borrow) Align() uint32    { return 4 }
func (Borrow) FlattenCount() int { return 1 }
```

---

### Task 72: Implement ResourceTable New/Rep/Drop

**Files:**
- Create: `internal/component/resource_table.go`
- Create: `internal/component/resource_table_test.go`

**Step 1: Write failing tests**

```go
// internal/component/resource_table_test.go

package component

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestResourceTable_NewAndRep(t *testing.T) {
	table := NewResourceTable()

	// Create a resource
	h := table.New("my-resource")

	// Retrieve it
	data, err := table.Rep(h)
	require.NoError(t, err)
	require.Equal(t, "my-resource", data)
}

func TestResourceTable_Drop(t *testing.T) {
	table := NewResourceTable()
	h := table.New("my-resource")

	// Drop returns the data
	data, err := table.Drop(h)
	require.NoError(t, err)
	require.Equal(t, "my-resource", data)

	// Subsequent access fails
	_, err = table.Rep(h)
	require.Error(t, err)
}

func TestResourceTable_UseAfterFree(t *testing.T) {
	table := NewResourceTable()

	// Create and drop a resource
	h1 := table.New("first")
	_, err := table.Drop(h1)
	require.NoError(t, err)

	// Create another resource (reuses index)
	h2 := table.New("second")

	// Old handle should fail even though index matches
	_, err = table.Rep(h1)
	require.Error(t, err, "generation mismatch should prevent access")

	// New handle works
	data, err := table.Rep(h2)
	require.NoError(t, err)
	require.Equal(t, "second", data)
}
```

**Step 2: Implement**

```go
// internal/component/resource_table.go

package component

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidHandle  = errors.New("invalid resource handle")
	ErrHandleNotOwned = errors.New("handle is not owned")
	ErrResourceInUse  = errors.New("resource has active borrows")
)

// Handle is a 64-bit resource handle: upper 32 bits = generation, lower 32 = index.
type Handle uint64

func (h Handle) Index() uint32      { return uint32(h) }
func (h Handle) Generation() uint32 { return uint32(h >> 32) }

func MakeHandle(idx, gen uint32) Handle {
	return Handle(uint64(gen)<<32 | uint64(idx))
}

type entryState uint8

const (
	entryFree entryState = iota
	entryOwned
	entryBorrowed
)

type resourceEntry struct {
	state       entryState
	generation  uint32
	data        any
	nextFree    int32
	borrowCount uint32
}

// ResourceTable manages resource handles with generation counting.
type ResourceTable struct {
	entries  []resourceEntry
	freeHead int32
}

// NewResourceTable creates an empty resource table.
func NewResourceTable() *ResourceTable {
	return &ResourceTable{
		freeHead: -1,
	}
}

// New creates a new resource and returns its handle.
func (t *ResourceTable) New(data any) Handle {
	var idx uint32
	var gen uint32

	if t.freeHead >= 0 {
		// Reuse a free slot
		idx = uint32(t.freeHead)
		entry := &t.entries[idx]
		t.freeHead = entry.nextFree
		gen = entry.generation + 1
		entry.state = entryOwned
		entry.generation = gen
		entry.data = data
		entry.nextFree = -1
		entry.borrowCount = 0
	} else {
		// Allocate new slot
		idx = uint32(len(t.entries))
		gen = 0
		t.entries = append(t.entries, resourceEntry{
			state:      entryOwned,
			generation: gen,
			data:       data,
			nextFree:   -1,
		})
	}

	return MakeHandle(idx, gen)
}

// Rep returns the data for a handle.
func (t *ResourceTable) Rep(h Handle) (any, error) {
	idx := h.Index()
	if idx >= uint32(len(t.entries)) {
		return nil, ErrInvalidHandle
	}

	entry := &t.entries[idx]
	if entry.generation != h.Generation() {
		return nil, fmt.Errorf("%w: generation mismatch", ErrInvalidHandle)
	}
	if entry.state == entryFree {
		return nil, ErrInvalidHandle
	}

	return entry.data, nil
}

// Drop destroys a resource and returns its data.
func (t *ResourceTable) Drop(h Handle) (any, error) {
	idx := h.Index()
	if idx >= uint32(len(t.entries)) {
		return nil, ErrInvalidHandle
	}

	entry := &t.entries[idx]
	if entry.generation != h.Generation() {
		return nil, fmt.Errorf("%w: generation mismatch", ErrInvalidHandle)
	}
	if entry.state != entryOwned {
		return nil, ErrHandleNotOwned
	}
	if entry.borrowCount > 0 {
		return nil, ErrResourceInUse
	}

	data := entry.data
	entry.state = entryFree
	entry.data = nil
	entry.nextFree = t.freeHead
	t.freeHead = int32(idx)

	return data, nil
}
```

---

### Task 73: Implement Borrow/EndBorrow

**Step 1: Write failing tests**

```go
func TestResourceTable_Borrow(t *testing.T) {
	table := NewResourceTable()
	h := table.New("resource")

	// Borrow the resource
	bh, err := table.Borrow(h)
	require.NoError(t, err)
	require.NotEqual(t, h, bh, "borrow handle should differ")

	// Can still access via borrow handle
	data, err := table.Rep(bh)
	require.NoError(t, err)
	require.Equal(t, "resource", data)

	// Cannot drop while borrowed
	_, err = table.Drop(h)
	require.ErrorIs(t, err, ErrResourceInUse)

	// End borrow
	err = table.EndBorrow(bh)
	require.NoError(t, err)

	// Now can drop
	_, err = table.Drop(h)
	require.NoError(t, err)
}
```

**Step 2: Implement**

```go
// Borrow creates a borrowed handle from an owned handle.
func (t *ResourceTable) Borrow(h Handle) (Handle, error) {
	idx := h.Index()
	if idx >= uint32(len(t.entries)) {
		return 0, ErrInvalidHandle
	}

	entry := &t.entries[idx]
	if entry.generation != h.Generation() {
		return 0, fmt.Errorf("%w: generation mismatch", ErrInvalidHandle)
	}
	if entry.state == entryFree {
		return 0, ErrInvalidHandle
	}

	entry.borrowCount++

	// Return handle with special marker (high bit of generation)
	borrowGen := entry.generation | 0x80000000
	return MakeHandle(idx, borrowGen), nil
}

// EndBorrow releases a borrowed handle.
func (t *ResourceTable) EndBorrow(h Handle) error {
	idx := h.Index()
	if idx >= uint32(len(t.entries)) {
		return ErrInvalidHandle
	}

	entry := &t.entries[idx]
	ownerGen := h.Generation() &^ 0x80000000
	if entry.generation != ownerGen {
		return fmt.Errorf("%w: generation mismatch", ErrInvalidHandle)
	}
	if entry.borrowCount == 0 {
		return errors.New("no active borrows")
	}

	entry.borrowCount--
	return nil
}
```

---

### Task 74-76: Call Scope Tracking

Implement `EnterCall()` and `ExitCall()` for tracking which borrows are active within a call scope.

---

### Task 77-80: Parse Resource Types from Binary

Parse resource type definitions from the component type section:
- `0x3f` - resource type
- `0x3e` - own handle
- `0x3d` - borrow handle

---

### Task 81-85: Canonical Resource Operations

Implement canonical built-in operations:
- `resource.new` - Create a new resource
- `resource.drop` - Destroy a resource
- `resource.rep` - Get the representation of a resource handle

---

### Task 86-90: Destructor Invocation

When a resource with a destructor is dropped, automatically invoke the destructor function.

---

### Task 91-100: Integration Tests

Test components to build:

```
internal/component/testdata/
├── resources/
│   ├── resource_own.wasm      # Create, use, drop resource
│   ├── resource_borrow.wasm   # Borrow semantics test
│   ├── resource_drop.wasm     # Destructor invocation test
│   └── resources.wit
```

---

## Running Tests

```bash
# Run resource table tests
go test ./internal/component/... -v -run TestResourceTable

# Run resource type tests
go test ./internal/component/types/... -v -run TestOwn
go test ./internal/component/types/... -v -run TestBorrow

# Run with race detector
go test ./internal/component/... -race -v
```

---

## References

- [Canonical ABI - Resource Handles](https://github.com/WebAssembly/component-model/blob/main/design/mvp/CanonicalABI.md#resource-handles)
- [Component Model Explainer - Resources](https://github.com/WebAssembly/component-model/blob/main/design/mvp/Explainer.md#resources)
