# Component Model Phase 3: Resources

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Parent Plan:** [2026-01-12-component-model-implementation.md](./2026-01-12-component-model-implementation.md)
**Design Doc:** [2026-01-12-component-model-design.md](./2026-01-12-component-model-design.md)
**Previous Phase:** [Phase 2: Complete Type System](./2026-01-12-component-model-phase2-type-system.md)
**Status:** COMPLETE
**Tasks:** 71-100

**Completed:**
- Resource type definitions (Own, Borrow types with ValType interface)
- Generation-counted ResourceTable preventing use-after-free
- BorrowScope and CallContext for call-scoped borrow tracking
- Binary parsing for resource (0x3f), own (0x69), borrow (0x68) types
- Canonical resource operations (resource.new, resource.rep, resource.drop)
- LiftOwn, LowerOwn, LiftBorrow, LowerBorrow ABI operations
- Destructor invocation for owned handles
- ExportedFunc.Call integration with own/borrow parameter/result handling
- Comprehensive unit tests (60+ resource-related tests)

---

## Overview

This phase implements the Component Model's resource system, including generation-counted handle tables, own/borrow semantics, and destructor invocation.

**Goal:** Full resource lifecycle management with proper ownership tracking and use-after-free protection.

**Prerequisites:**
- Phase 1 complete (binary parser, primitive types)
- Phase 2 complete (all composite types, lift/lower)

---

## Phase 3 Milestones

| Milestone | Description | Tasks | Success Criteria |
|-----------|-------------|-------|------------------|
| 3.1 | Resource type definitions | 71-73 | Own, Borrow types implement ValType with correct Size/Align/FlattenCount |
| 3.2 | ResourceTable implementation | 74-77 | Generation-counted handles prevent use-after-free |
| 3.3 | Call scope tracking | 78-80 | Borrow scope tracks active borrows, traps on return with borrows |
| 3.4 | Binary parsing for resources | 81-84 | Decoder reads resource type 0x3f, own 0x69, borrow 0x68 |
| 3.5 | Canonical resource operations | 85-90 | resource.new, resource.drop, resource.rep work correctly |
| 3.6 | Lift/lower own and borrow | 91-94 | lift_own, lower_own, lift_borrow, lower_borrow implemented |
| 3.7 | Destructor invocation | 95-97 | Dropped resources trigger destructors |
| 3.8 | Integration tests | 98-100 | Round-trip resources through real components |

---

## Reference: Wasmtime Test Scenarios to Port

Based on wasmtime's `tests/all/component_model/resources.rs`:

| Test Pattern | Description | Error Expected |
|--------------|-------------|----------------|
| `drop_guest_twice` | Double-drop of guest resource | "unknown handle index" |
| `drop_host_twice` | Double-drop of host resource | "host resource already consumed" |
| `active_borrows_at_end_of_call` | Return with active borrows | "borrow handles still remain" |
| `cannot_use_borrow_for_own` | Borrow where own expected | type error |
| `can_use_own_for_borrow` | Own implicitly converts to borrow | success |
| `pass_moved_resource` | Same own handle used twice | "unknown handle index" |
| `mismatch_resource_types` | Wrong resource type | type mismatch trap |

---

## Canonical ABI Reference

### ResourceHandle Structure (from spec)

```python
class ResourceHandle:
    rt: ResourceType        # Runtime resource type identity
    rep: int                # Opaque i32 representation
    own: bool               # True for owned, False for borrowed
    borrow_scope: Task      # Task that lowered this borrow (if borrowed)
    num_lends: int          # Count of active lends from this handle
```

### Binary Format

| Opcode | Encoding | Meaning |
|--------|----------|---------|
| `0x3f 0x7f f?` | Sync resource | `(resource (rep i32) (dtor f)?)` |
| `0x69 i:<typeidx>` | Own handle | `(own i)` |
| `0x68 i:<typeidx>` | Borrow handle | `(borrow i)` |

### Canonical Operations

| Opcode | Encoding | Operation |
|--------|----------|-----------|
| `0x02 rt:<typeidx>` | resource.new | Creates owning handle from representation |
| `0x03 rt:<typeidx>` | resource.drop | Drops handle, calls destructor if owned |
| `0x04 rt:<typeidx>` | resource.rep | Extracts representation from handle |

### Lift/Lower Rules

- **lift_own**: Remove owned handle from table, return rep, trap if has active lends
- **lower_own**: Create owned handle in table, return index
- **lift_borrow**: Get handle from table (don't remove), track lend, return rep
- **lower_borrow**: If same component as resource impl, return rep directly; else create borrowed handle

---

## Tasks

### Task 71: Define Own and Borrow Types

**Files:**
- Create: `internal/component/types/resource.go`
- Create: `internal/component/types/resource_test.go`

**Step 1: Write failing test for Own type**

```go
// internal/component/types/resource_test.go

package types

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestOwnType(t *testing.T) {
	// own<T> is represented as i32 handle index
	o := Own{ResourceIdx: 0}
	require.Equal(t, uint32(4), o.Size())
	require.Equal(t, uint32(4), o.Align())
	require.Equal(t, 1, o.FlattenCount())
}

func TestBorrowType(t *testing.T) {
	// borrow<T> same layout as own
	b := Borrow{ResourceIdx: 0}
	require.Equal(t, uint32(4), b.Size())
	require.Equal(t, uint32(4), b.Align())
	require.Equal(t, 1, b.FlattenCount())
}

func TestOwnAndBorrowDistinct(t *testing.T) {
	o := Own{ResourceIdx: 5}
	b := Borrow{ResourceIdx: 5}

	// They reference the same resource type but are different handle types
	require.Equal(t, o.ResourceIdx, b.ResourceIdx)

	// Type assertion should work
	var _ ValType = o
	var _ ValType = b
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/types/... -v -run TestOwn`
Expected: FAIL with "undefined: Own"

**Step 3: Implement Own and Borrow types**

```go
// internal/component/types/resource.go

package types

// Own represents an owning handle to a resource.
// When an own<T> is dropped, the resource's destructor is called.
type Own struct {
	ResourceIdx uint32 // Index of the resource type in component's type section
}

func (Own) valType() {}

// Size returns 4 because handles are i32 indices.
func (Own) Size() uint32 { return 4 }

// Align returns 4 for i32 alignment.
func (Own) Align() uint32 { return 4 }

// FlattenCount returns 1 because a handle is a single i32.
func (Own) FlattenCount() int { return 1 }

// Borrow represents a borrowed handle to a resource.
// Borrows do not own the resource and must not outlive the call scope.
type Borrow struct {
	ResourceIdx uint32 // Index of the resource type in component's type section
}

func (Borrow) valType() {}

// Size returns 4 because handles are i32 indices.
func (Borrow) Size() uint32 { return 4 }

// Align returns 4 for i32 alignment.
func (Borrow) Align() uint32 { return 4 }

// FlattenCount returns 1 because a handle is a single i32.
func (Borrow) FlattenCount() int { return 1 }
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/types/... -v -run TestOwn`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/types/resource.go internal/component/types/resource_test.go
git commit -m "$(cat <<'EOF'
feat(component): add Own and Borrow handle types

Implements ValType for own<T> and borrow<T> handle types.
Both are represented as i32 indices into a handle table.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 72: Define ResourceType Structure

**Files:**
- Modify: `internal/component/types/resource.go`
- Modify: `internal/component/types/resource_test.go`

**Step 1: Write failing test for ResourceType**

```go
// Add to internal/component/types/resource_test.go

func TestResourceType(t *testing.T) {
	// Resource with destructor
	dtorIdx := uint32(42)
	r := ResourceType{
		Destructor: &dtorIdx,
	}
	require.NotNil(t, r.Destructor)
	require.Equal(t, uint32(42), *r.Destructor)

	// Resource without destructor
	r2 := ResourceType{
		Destructor: nil,
	}
	require.Nil(t, r2.Destructor)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/types/... -v -run TestResourceType`
Expected: FAIL with "undefined: ResourceType"

**Step 3: Implement ResourceType**

```go
// Add to internal/component/types/resource.go

// ResourceType represents a resource type definition.
// Resources have an optional destructor that is called when the resource is dropped.
type ResourceType struct {
	// Destructor is the index of the destructor function (nil if no destructor).
	Destructor *uint32
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/types/... -v -run TestResourceType`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/types/resource.go internal/component/types/resource_test.go
git commit -m "$(cat <<'EOF'
feat(component): add ResourceType definition

ResourceType holds the optional destructor function index
for resource cleanup when handles are dropped.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 73: Add Val constructors for Own and Borrow

**Files:**
- Modify: `internal/component/val.go`
- Modify: `internal/component/val_test.go`

**Step 1: Write failing test for Val constructors**

```go
// Add to internal/component/val_test.go

func TestValOwn(t *testing.T) {
	v := ValOwn(42)
	require.Equal(t, ValKindOwn, v.Kind())
	require.Equal(t, uint32(42), v.Own())
}

func TestValBorrow(t *testing.T) {
	v := ValBorrow(99)
	require.Equal(t, ValKindBorrow, v.Kind())
	require.Equal(t, uint32(99), v.Borrow())
}

func TestValOwnWrongKind(t *testing.T) {
	v := ValS32(5)
	err := require.CapturePanic(func() { v.Own() })
	require.Error(t, err)
}

func TestValBorrowWrongKind(t *testing.T) {
	v := ValS32(5)
	err := require.CapturePanic(func() { v.Borrow() })
	require.Error(t, err)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -v -run TestValOwn`
Expected: FAIL with "undefined: ValOwn"

**Step 3: Implement Val constructors and accessors**

```go
// Add to internal/component/val.go

// ValOwn creates a Val containing an owning handle.
func ValOwn(handle uint32) Val {
	return Val{kind: ValKindOwn, v: handle}
}

// ValBorrow creates a Val containing a borrowed handle.
func ValBorrow(handle uint32) Val {
	return Val{kind: ValKindBorrow, v: handle}
}

// Own returns the handle index if this Val is an own handle.
func (v Val) Own() (uint32, error) {
	if v.kind != ValKindOwn {
		return 0, fmt.Errorf("expected own, got %v", v.kind)
	}
	return v.v.(uint32), nil
}

// Borrow returns the handle index if this Val is a borrowed handle.
func (v Val) Borrow() (uint32, error) {
	if v.kind != ValKindBorrow {
		return 0, fmt.Errorf("expected borrow, got %v", v.kind)
	}
	return v.v.(uint32), nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -v -run TestValOwn`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/val.go internal/component/val_test.go
git commit -m "$(cat <<'EOF'
feat(component): add Val constructors for own and borrow handles

Adds ValOwn() and ValBorrow() constructors plus Own() and Borrow()
accessor methods to the Val type.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 74: Create ResourceTable with New and Get

**Files:**
- Create: `internal/component/resource_table.go`
- Create: `internal/component/resource_table_test.go`

**Step 1: Write failing test for ResourceTable.New and Get**

```go
// internal/component/resource_table_test.go

package component

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestResourceTable_NewAndGet(t *testing.T) {
	table := NewResourceTable()

	// Create a resource
	h := table.New("my-resource", true) // own=true

	// Verify handle parts
	require.Equal(t, uint32(0), h.Index())
	require.Equal(t, uint32(0), h.Generation())

	// Retrieve it
	entry, err := table.Get(h)
	require.NoError(t, err)
	require.Equal(t, "my-resource", entry.Rep)
	require.True(t, entry.Own)
}

func TestResourceTable_MultipleResources(t *testing.T) {
	table := NewResourceTable()

	h1 := table.New("first", true)
	h2 := table.New("second", true)
	h3 := table.New("third", true)

	require.Equal(t, uint32(0), h1.Index())
	require.Equal(t, uint32(1), h2.Index())
	require.Equal(t, uint32(2), h3.Index())

	e1, _ := table.Get(h1)
	e2, _ := table.Get(h2)
	e3, _ := table.Get(h3)

	require.Equal(t, "first", e1.Rep)
	require.Equal(t, "second", e2.Rep)
	require.Equal(t, "third", e3.Rep)
}

func TestHandle_MakeHandle(t *testing.T) {
	h := MakeHandle(42, 7)
	require.Equal(t, uint32(42), h.Index())
	require.Equal(t, uint32(7), h.Generation())
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -v -run TestResourceTable_New`
Expected: FAIL with "undefined: NewResourceTable"

**Step 3: Implement ResourceTable with New and Get**

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
// Generation counting prevents use-after-free when slots are reused.
type Handle uint64

// Index returns the table index portion of the handle.
func (h Handle) Index() uint32 { return uint32(h) }

// Generation returns the generation counter portion of the handle.
func (h Handle) Generation() uint32 { return uint32(h >> 32) }

// MakeHandle constructs a handle from an index and generation.
func MakeHandle(idx, gen uint32) Handle {
	return Handle(uint64(gen)<<32 | uint64(idx))
}

// HandleEntry represents an active resource in the table.
type HandleEntry struct {
	Rep         any    // The resource representation value
	Own         bool   // True if this is an owning handle
	NumLends    uint32 // Number of active borrows from this handle
	BorrowScope any    // The scope that created this borrow (for borrowed handles)
}

type entryState uint8

const (
	entryFree entryState = iota
	entryOccupied
)

type tableEntry struct {
	state      entryState
	generation uint32
	entry      HandleEntry
	nextFree   int32
}

// ResourceTable manages resource handles with generation counting.
// Implements the Component Model's handle table semantics.
type ResourceTable struct {
	entries  []tableEntry
	freeHead int32 // Head of free list, -1 if empty
}

// NewResourceTable creates an empty resource table.
func NewResourceTable() *ResourceTable {
	return &ResourceTable{
		freeHead: -1,
	}
}

// New creates a new resource handle and returns it.
func (t *ResourceTable) New(rep any, own bool) Handle {
	var idx uint32
	var gen uint32

	if t.freeHead >= 0 {
		// Reuse a free slot
		idx = uint32(t.freeHead)
		entry := &t.entries[idx]
		t.freeHead = entry.nextFree
		gen = entry.generation + 1
		entry.state = entryOccupied
		entry.generation = gen
		entry.entry = HandleEntry{Rep: rep, Own: own}
		entry.nextFree = -1
	} else {
		// Allocate new slot
		idx = uint32(len(t.entries))
		gen = 0
		t.entries = append(t.entries, tableEntry{
			state:      entryOccupied,
			generation: gen,
			entry:      HandleEntry{Rep: rep, Own: own},
			nextFree:   -1,
		})
	}

	return MakeHandle(idx, gen)
}

// Get retrieves the entry for a handle without removing it.
func (t *ResourceTable) Get(h Handle) (*HandleEntry, error) {
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

	return &entry.entry, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -v -run TestResourceTable_New`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/resource_table.go internal/component/resource_table_test.go
git commit -m "$(cat <<'EOF'
feat(component): add ResourceTable with New and Get operations

Implements generation-counted handle table for resource management.
Handles are 64-bit values with 32-bit generation + 32-bit index.
Generation counting prevents use-after-free when slots are reused.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 75: Add ResourceTable.Remove (transfer ownership out)

**Files:**
- Modify: `internal/component/resource_table.go`
- Modify: `internal/component/resource_table_test.go`

**Step 1: Write failing test for Remove**

```go
// Add to internal/component/resource_table_test.go

func TestResourceTable_Remove(t *testing.T) {
	table := NewResourceTable()
	h := table.New("my-resource", true)

	// Remove returns the entry
	entry, err := table.Remove(h)
	require.NoError(t, err)
	require.Equal(t, "my-resource", entry.Rep)
	require.True(t, entry.Own)

	// Subsequent Get fails
	_, err = table.Get(h)
	require.Error(t, err)
}

func TestResourceTable_UseAfterFree(t *testing.T) {
	table := NewResourceTable()

	// Create and remove a resource
	h1 := table.New("first", true)
	_, err := table.Remove(h1)
	require.NoError(t, err)

	// Create another resource (reuses index 0)
	h2 := table.New("second", true)
	require.Equal(t, uint32(0), h2.Index())
	require.Equal(t, uint32(1), h2.Generation()) // Generation incremented

	// Old handle should fail (generation mismatch)
	_, err = table.Get(h1)
	require.Error(t, err, "generation mismatch should prevent access")

	// New handle works
	entry, err := table.Get(h2)
	require.NoError(t, err)
	require.Equal(t, "second", entry.Rep)
}

func TestResourceTable_DoubleFree(t *testing.T) {
	table := NewResourceTable()
	h := table.New("resource", true)

	// First remove succeeds
	_, err := table.Remove(h)
	require.NoError(t, err)

	// Second remove fails (already freed)
	_, err = table.Remove(h)
	require.Error(t, err)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -v -run TestResourceTable_Remove`
Expected: FAIL with "undefined: Remove" or method not found

**Step 3: Implement Remove**

```go
// Add to internal/component/resource_table.go

// Remove removes a handle from the table and returns its entry.
// Used for lift_own to transfer ownership out of the component.
// Traps if the handle has active borrows (NumLends > 0).
func (t *ResourceTable) Remove(h Handle) (*HandleEntry, error) {
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
	if entry.entry.NumLends > 0 {
		return nil, ErrResourceInUse
	}

	// Copy the entry before clearing
	result := entry.entry

	// Mark as free and add to free list
	entry.state = entryFree
	entry.entry = HandleEntry{}
	entry.nextFree = t.freeHead
	t.freeHead = int32(idx)

	return &result, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -v -run TestResourceTable_Remove`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/resource_table.go internal/component/resource_table_test.go
git commit -m "$(cat <<'EOF'
feat(component): add ResourceTable.Remove for ownership transfer

Remove extracts a handle from the table, returning its entry.
Uses free list for efficient slot reuse. Generation is incremented
on reuse to prevent use-after-free via stale handles.

Traps if handle has active borrows (NumLends > 0).

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 76: Add borrow tracking (IncrementLends/DecrementLends)

**Files:**
- Modify: `internal/component/resource_table.go`
- Modify: `internal/component/resource_table_test.go`

**Step 1: Write failing test for borrow tracking**

```go
// Add to internal/component/resource_table_test.go

func TestResourceTable_BorrowTracking(t *testing.T) {
	table := NewResourceTable()
	h := table.New("resource", true)

	// Increment lends (for lift_borrow)
	err := table.IncrementLends(h)
	require.NoError(t, err)

	entry, _ := table.Get(h)
	require.Equal(t, uint32(1), entry.NumLends)

	// Cannot remove while borrowed
	_, err = table.Remove(h)
	require.ErrorIs(t, err, ErrResourceInUse)

	// Decrement lends
	err = table.DecrementLends(h)
	require.NoError(t, err)

	entry, _ = table.Get(h)
	require.Equal(t, uint32(0), entry.NumLends)

	// Now can remove
	_, err = table.Remove(h)
	require.NoError(t, err)
}

func TestResourceTable_MultipleBorrows(t *testing.T) {
	table := NewResourceTable()
	h := table.New("resource", true)

	// Multiple concurrent borrows
	require.NoError(t, table.IncrementLends(h))
	require.NoError(t, table.IncrementLends(h))
	require.NoError(t, table.IncrementLends(h))

	entry, _ := table.Get(h)
	require.Equal(t, uint32(3), entry.NumLends)

	// Decrement all
	require.NoError(t, table.DecrementLends(h))
	require.NoError(t, table.DecrementLends(h))
	require.NoError(t, table.DecrementLends(h))

	entry, _ = table.Get(h)
	require.Equal(t, uint32(0), entry.NumLends)
}

func TestResourceTable_DecrementUnderflow(t *testing.T) {
	table := NewResourceTable()
	h := table.New("resource", true)

	// Decrement without increment should error
	err := table.DecrementLends(h)
	require.Error(t, err)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -v -run TestResourceTable_Borrow`
Expected: FAIL with method not found

**Step 3: Implement IncrementLends and DecrementLends**

```go
// Add to internal/component/resource_table.go

// IncrementLends increments the borrow count for a handle.
// Called during lift_borrow to track active borrows.
func (t *ResourceTable) IncrementLends(h Handle) error {
	idx := h.Index()
	if idx >= uint32(len(t.entries)) {
		return ErrInvalidHandle
	}

	entry := &t.entries[idx]
	if entry.generation != h.Generation() {
		return fmt.Errorf("%w: generation mismatch", ErrInvalidHandle)
	}
	if entry.state == entryFree {
		return ErrInvalidHandle
	}

	entry.entry.NumLends++
	return nil
}

// DecrementLends decrements the borrow count for a handle.
// Called when a borrow scope completes.
func (t *ResourceTable) DecrementLends(h Handle) error {
	idx := h.Index()
	if idx >= uint32(len(t.entries)) {
		return ErrInvalidHandle
	}

	entry := &t.entries[idx]
	if entry.generation != h.Generation() {
		return fmt.Errorf("%w: generation mismatch", ErrInvalidHandle)
	}
	if entry.state == entryFree {
		return ErrInvalidHandle
	}
	if entry.entry.NumLends == 0 {
		return errors.New("no active borrows to decrement")
	}

	entry.entry.NumLends--
	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -v -run TestResourceTable_Borrow`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/resource_table.go internal/component/resource_table_test.go
git commit -m "$(cat <<'EOF'
feat(component): add borrow tracking to ResourceTable

IncrementLends/DecrementLends track active borrows from owned handles.
A handle cannot be removed while it has outstanding borrows (NumLends > 0).

This implements the spec's lend counting mechanism for borrow scope safety.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 77: Add borrowed handle support

**Files:**
- Modify: `internal/component/resource_table.go`
- Modify: `internal/component/resource_table_test.go`

**Step 1: Write failing test for borrowed handles**

```go
// Add to internal/component/resource_table_test.go

func TestResourceTable_BorrowedHandle(t *testing.T) {
	table := NewResourceTable()

	// Create borrowed handle (own=false)
	h := table.New("resource", false)

	entry, err := table.Get(h)
	require.NoError(t, err)
	require.False(t, entry.Own)
	require.Equal(t, "resource", entry.Rep)
}

func TestResourceTable_RemoveBorrowedMustNotCallDestructor(t *testing.T) {
	table := NewResourceTable()
	h := table.New("resource", false) // borrowed

	entry, err := table.Remove(h)
	require.NoError(t, err)
	require.False(t, entry.Own) // Caller checks Own to decide on destructor
}
```

**Step 2: Run test to verify it passes**

Run: `go test ./internal/component/... -v -run TestResourceTable_Borrowed`
Expected: PASS (existing implementation supports this)

**Step 3: Commit (if tests pass with existing code)**

```bash
git add internal/component/resource_table_test.go
git commit -m "$(cat <<'EOF'
test(component): add tests for borrowed handle entries

Borrowed handles (own=false) are created by lower_borrow for
cross-component resource passing. They track the borrow scope
and don't invoke destructors when removed.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 78: Create BorrowScope for call-scoped tracking

**Files:**
- Create: `internal/component/borrow_scope.go`
- Create: `internal/component/borrow_scope_test.go`

**Step 1: Write failing test for BorrowScope**

```go
// internal/component/borrow_scope_test.go

package component

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestBorrowScope_TrackLenders(t *testing.T) {
	table := NewResourceTable()
	h1 := table.New("resource1", true)
	h2 := table.New("resource2", true)

	scope := NewBorrowScope(table)

	// Track borrows from two handles
	require.NoError(t, scope.AddLender(h1))
	require.NoError(t, scope.AddLender(h2))

	// Both handles should have NumLends incremented
	e1, _ := table.Get(h1)
	e2, _ := table.Get(h2)
	require.Equal(t, uint32(1), e1.NumLends)
	require.Equal(t, uint32(1), e2.NumLends)

	// End scope releases all lends
	require.NoError(t, scope.Release())

	e1, _ = table.Get(h1)
	e2, _ = table.Get(h2)
	require.Equal(t, uint32(0), e1.NumLends)
	require.Equal(t, uint32(0), e2.NumLends)
}

func TestBorrowScope_SameLenderMultipleTimes(t *testing.T) {
	table := NewResourceTable()
	h := table.New("resource", true)

	scope := NewBorrowScope(table)

	// Same handle borrowed multiple times
	require.NoError(t, scope.AddLender(h))
	require.NoError(t, scope.AddLender(h))

	entry, _ := table.Get(h)
	require.Equal(t, uint32(2), entry.NumLends)

	require.NoError(t, scope.Release())

	entry, _ = table.Get(h)
	require.Equal(t, uint32(0), entry.NumLends)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -v -run TestBorrowScope`
Expected: FAIL with "undefined: NewBorrowScope"

**Step 3: Implement BorrowScope**

```go
// internal/component/borrow_scope.go

package component

// BorrowScope tracks resource handles that have been lent during a call.
// When the call completes, all lends must be released.
// This implements the Canonical ABI's Subtask.lenders tracking.
type BorrowScope struct {
	table   *ResourceTable
	lenders []Handle // Handles that were borrowed from
}

// NewBorrowScope creates a new borrow scope for tracking lends.
func NewBorrowScope(table *ResourceTable) *BorrowScope {
	return &BorrowScope{
		table:   table,
		lenders: nil,
	}
}

// AddLender records that a handle has been borrowed from.
// Increments the handle's NumLends counter.
func (s *BorrowScope) AddLender(h Handle) error {
	if err := s.table.IncrementLends(h); err != nil {
		return err
	}
	s.lenders = append(s.lenders, h)
	return nil
}

// Release decrements NumLends for all tracked lenders.
// Called when the call scope completes.
func (s *BorrowScope) Release() error {
	for _, h := range s.lenders {
		if err := s.table.DecrementLends(h); err != nil {
			return err
		}
	}
	s.lenders = nil
	return nil
}

// HasOutstandingBorrows returns true if any handles are still borrowed.
func (s *BorrowScope) HasOutstandingBorrows() bool {
	return len(s.lenders) > 0
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -v -run TestBorrowScope`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/borrow_scope.go internal/component/borrow_scope_test.go
git commit -m "$(cat <<'EOF'
feat(component): add BorrowScope for call-scoped borrow tracking

BorrowScope tracks handles borrowed during a function call.
When the call completes, Release() decrements NumLends for all
tracked handles, enforcing borrow lifetime constraints.

Implements the spec's Subtask.lenders mechanism.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 79: Add CallContext with borrow enforcement

**Files:**
- Create: `internal/component/call_context.go`
- Create: `internal/component/call_context_test.go`

**Step 1: Write failing test for CallContext**

```go
// internal/component/call_context_test.go

package component

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestCallContext_NumBorrows(t *testing.T) {
	ctx := NewCallContext()

	require.Equal(t, 0, ctx.NumBorrows())

	ctx.IncrementBorrows()
	ctx.IncrementBorrows()
	require.Equal(t, 2, ctx.NumBorrows())

	ctx.DecrementBorrows()
	require.Equal(t, 1, ctx.NumBorrows())
}

func TestCallContext_CanReturn(t *testing.T) {
	ctx := NewCallContext()

	// Can return when no outstanding borrows
	require.True(t, ctx.CanReturn())

	// Cannot return with outstanding borrows
	ctx.IncrementBorrows()
	require.False(t, ctx.CanReturn())

	// Can return after borrows released
	ctx.DecrementBorrows()
	require.True(t, ctx.CanReturn())
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -v -run TestCallContext`
Expected: FAIL with "undefined: NewCallContext"

**Step 3: Implement CallContext**

```go
// internal/component/call_context.go

package component

// CallContext tracks state for a single component function call.
// Implements the Canonical ABI's Task tracking for borrow validation.
type CallContext struct {
	numBorrows int // Number of borrowed handles received by this call
}

// NewCallContext creates a new call context.
func NewCallContext() *CallContext {
	return &CallContext{}
}

// IncrementBorrows records receiving a borrowed handle.
// Called by lower_borrow when a borrowed handle is created.
func (c *CallContext) IncrementBorrows() {
	c.numBorrows++
}

// DecrementBorrows records dropping a borrowed handle.
// Called by resource.drop for borrowed handles.
func (c *CallContext) DecrementBorrows() {
	c.numBorrows--
}

// NumBorrows returns the current count of outstanding borrowed handles.
func (c *CallContext) NumBorrows() int {
	return c.numBorrows
}

// CanReturn returns true if the call can return (no outstanding borrows).
// Per spec, returning with outstanding borrows is a trap.
func (c *CallContext) CanReturn() bool {
	return c.numBorrows == 0
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -v -run TestCallContext`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/call_context.go internal/component/call_context_test.go
git commit -m "$(cat <<'EOF'
feat(component): add CallContext for borrow count enforcement

CallContext tracks borrowed handles received during a function call.
CanReturn() returns false if any borrows are outstanding, enforcing
the spec requirement that all borrows must be dropped before return.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 80: Integrate CallContext trap on return with borrows

**Files:**
- Modify: `internal/component/call_context.go`
- Modify: `internal/component/call_context_test.go`

**Step 1: Write failing test for return trap**

```go
// Add to internal/component/call_context_test.go

func TestCallContext_ValidateReturn_Success(t *testing.T) {
	ctx := NewCallContext()
	require.NoError(t, ctx.ValidateReturn())
}

func TestCallContext_ValidateReturn_WithBorrows(t *testing.T) {
	ctx := NewCallContext()
	ctx.IncrementBorrows()

	err := ctx.ValidateReturn()
	require.Error(t, err)
	require.Contains(t, err.Error(), "borrow")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -v -run TestCallContext_ValidateReturn`
Expected: FAIL with method not found

**Step 3: Implement ValidateReturn**

```go
// Add to internal/component/call_context.go

import "errors"

var ErrOutstandingBorrows = errors.New("cannot return: borrow handles still remain")

// ValidateReturn checks if returning is allowed.
// Returns an error if there are outstanding borrowed handles.
func (c *CallContext) ValidateReturn() error {
	if c.numBorrows > 0 {
		return ErrOutstandingBorrows
	}
	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -v -run TestCallContext_ValidateReturn`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/call_context.go internal/component/call_context_test.go
git commit -m "$(cat <<'EOF'
feat(component): add ValidateReturn for borrow enforcement

ValidateReturn returns ErrOutstandingBorrows if the call has
unreleased borrowed handles, implementing the spec's trap
on return with active borrows.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 81: Parse resource type from binary (0x3f)

**Files:**
- Modify: `internal/component/binary/types.go`
- Modify: `internal/component/binary/types_test.go`

**Step 1: Write failing test for resource type parsing**

```go
// Add to internal/component/binary/types_test.go

func TestDecodeResourceType(t *testing.T) {
	// 0x3f 0x7f = resource with rep i32, no destructor
	data := []byte{0x3f, 0x7f}
	r := bytes.NewReader(data)

	typeDef, err := decodeResourceTypeDef(r)
	require.NoError(t, err)
	require.Nil(t, typeDef.Destructor)
}

func TestDecodeResourceType_WithDestructor(t *testing.T) {
	// 0x3f 0x7f 0x05 = resource with rep i32, destructor at func index 5
	data := []byte{0x3f, 0x7f, 0x05}
	r := bytes.NewReader(data)

	typeDef, err := decodeResourceTypeDef(r)
	require.NoError(t, err)
	require.NotNil(t, typeDef.Destructor)
	require.Equal(t, uint32(5), *typeDef.Destructor)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/binary/... -v -run TestDecodeResourceType`
Expected: FAIL with "undefined: decodeResourceTypeDef"

**Step 3: Implement resource type parsing**

```go
// Add to internal/component/binary/types.go

// TypeOpResource is the opcode for resource type definitions.
const TypeOpResource = 0x3f

// ResourceTypeDef represents a parsed resource type definition.
type ResourceTypeDef struct {
	Destructor *uint32 // Function index of destructor, nil if none
}

// decodeResourceTypeDef decodes a resource type definition.
// Format: 0x3f 0x7f [destructor_idx]
// The 0x7f indicates i32 representation (always required).
func decodeResourceTypeDef(r *bytes.Reader) (*ResourceTypeDef, error) {
	// Read the opcode (already consumed by caller typically)
	// Read rep type (must be 0x7f for i32)
	repType, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("reading resource rep type: %w", err)
	}
	if repType != 0x7f {
		return nil, fmt.Errorf("unsupported resource rep type: 0x%02x (expected 0x7f)", repType)
	}

	// Check if there's a destructor index
	var destructor *uint32
	if r.Len() > 0 {
		// Peek to see if there's more data
		b, _ := r.ReadByte()
		r.UnreadByte()
		if b != 0 { // 0 would start next type, non-zero is destructor idx
			idx, err := leb128.DecodeUint32(r)
			if err == nil {
				destructor = &idx
			}
		}
	}

	return &ResourceTypeDef{
		Destructor: destructor,
	}, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/binary/... -v -run TestDecodeResourceType`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/binary/types.go internal/component/binary/types_test.go
git commit -m "$(cat <<'EOF'
feat(component): parse resource type definitions (0x3f)

Decodes resource type definitions with optional destructor function.
Format: 0x3f 0x7f [destructor_idx]
The 0x7f byte indicates i32 representation (always required).

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 82: Parse own handle type (0x69)

**Files:**
- Modify: `internal/component/binary/valtype.go`
- Modify: `internal/component/binary/valtype_test.go`

**Step 1: Write failing test for own type parsing**

```go
// Add to internal/component/binary/valtype_test.go

func TestDecodeOwnType(t *testing.T) {
	// 0x69 0x03 = own<resource_type_3>
	data := []byte{0x69, 0x03}
	r := bytes.NewReader(data)

	valType, err := decodeValType(r)
	require.NoError(t, err)
	require.Equal(t, ValTypeKindOwn, valType.Kind)
	require.Equal(t, uint32(3), valType.TypeIdx)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/binary/... -v -run TestDecodeOwnType`
Expected: FAIL

**Step 3: Implement own type parsing**

```go
// Add constants to internal/component/binary/valtype.go

const (
	ValTypeOpOwn    = 0x69
	ValTypeOpBorrow = 0x68
)

// Add to ValTypeKind enum
const (
	// ... existing kinds ...
	ValTypeKindOwn
	ValTypeKindBorrow
)

// Update decodeValType to handle 0x69
// In the switch statement:
case ValTypeOpOwn:
	idx, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("reading own type index: %w", err)
	}
	return &ValTypeRef{Kind: ValTypeKindOwn, TypeIdx: idx}, nil
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/binary/... -v -run TestDecodeOwnType`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/binary/valtype.go internal/component/binary/valtype_test.go
git commit -m "$(cat <<'EOF'
feat(component): parse own handle type (0x69)

Decodes own<T> handle types from binary format.
Format: 0x69 <typeidx>

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 83: Parse borrow handle type (0x68)

**Files:**
- Modify: `internal/component/binary/valtype.go`
- Modify: `internal/component/binary/valtype_test.go`

**Step 1: Write failing test for borrow type parsing**

```go
// Add to internal/component/binary/valtype_test.go

func TestDecodeBorrowType(t *testing.T) {
	// 0x68 0x07 = borrow<resource_type_7>
	data := []byte{0x68, 0x07}
	r := bytes.NewReader(data)

	valType, err := decodeValType(r)
	require.NoError(t, err)
	require.Equal(t, ValTypeKindBorrow, valType.Kind)
	require.Equal(t, uint32(7), valType.TypeIdx)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/binary/... -v -run TestDecodeBorrowType`
Expected: FAIL

**Step 3: Implement borrow type parsing**

```go
// Add to decodeValType switch in internal/component/binary/valtype.go

case ValTypeOpBorrow:
	idx, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("reading borrow type index: %w", err)
	}
	return &ValTypeRef{Kind: ValTypeKindBorrow, TypeIdx: idx}, nil
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/binary/... -v -run TestDecodeBorrowType`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/binary/valtype.go internal/component/binary/valtype_test.go
git commit -m "$(cat <<'EOF'
feat(component): parse borrow handle type (0x68)

Decodes borrow<T> handle types from binary format.
Format: 0x68 <typeidx>

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 84: Integrate resource types into type section decoder

**Files:**
- Modify: `internal/component/binary/decoder.go`
- Modify: `internal/component/binary/decoder_test.go`

**Step 1: Write failing test**

```go
// Add to internal/component/binary/decoder_test.go

func TestDecodeTypeSection_WithResource(t *testing.T) {
	// Type section with one resource type
	// Count=1, then 0x3f 0x7f (resource, i32 rep, no destructor)
	data := []byte{
		0x01,       // count = 1
		0x3f, 0x7f, // resource type
	}

	types, err := decodeTypeSection(bytes.NewReader(data))
	require.NoError(t, err)
	require.Len(t, types, 1)
	require.Equal(t, TypeDefKindResource, types[0].Kind)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/binary/... -v -run TestDecodeTypeSection_WithResource`
Expected: FAIL (resource case not handled)

**Step 3: Add resource case to type section decoder**

```go
// Add to decodeTypeSection switch in internal/component/binary/decoder.go

case TypeOpResource:
	resourceDef, err := decodeResourceTypeDef(r)
	if err != nil {
		return nil, fmt.Errorf("decoding resource type: %w", err)
	}
	types = append(types, TypeDef{
		Kind:     TypeDefKindResource,
		Resource: resourceDef,
	})
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/binary/... -v -run TestDecodeTypeSection_WithResource`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/binary/decoder.go internal/component/binary/decoder_test.go
git commit -m "$(cat <<'EOF'
feat(component): integrate resource types into type section decoder

The type section decoder now handles 0x3f resource type definitions
alongside function and composite types.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 85: Implement LiftOwn in ABI

**Files:**
- Modify: `internal/component/abi/lift.go`
- Modify: `internal/component/abi/lift_test.go`

**Step 1: Write failing test**

```go
// Add to internal/component/abi/lift_test.go

func TestLiftOwn(t *testing.T) {
	table := component.NewResourceTable()
	h := table.New("my-resource", true)

	ctx := &LiftContext{
		ResourceTable: table,
	}

	// Lift the handle (transfers ownership out)
	rep, err := LiftOwn(ctx, uint32(h.Index()))
	require.NoError(t, err)
	require.Equal(t, "my-resource", rep)

	// Handle should be removed from table
	_, err = table.Get(h)
	require.Error(t, err)
}

func TestLiftOwn_WithActiveBorrows(t *testing.T) {
	table := component.NewResourceTable()
	h := table.New("my-resource", true)
	table.IncrementLends(h) // Active borrow

	ctx := &LiftContext{
		ResourceTable: table,
	}

	// Should trap because handle has active borrows
	_, err := LiftOwn(ctx, uint32(h.Index()))
	require.Error(t, err)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/abi/... -v -run TestLiftOwn`
Expected: FAIL with "undefined: LiftOwn"

**Step 3: Implement LiftOwn**

```go
// Add to internal/component/abi/lift.go

// LiftOwn transfers ownership of a resource out of the component.
// Removes the handle from the table and returns the representation.
// Traps if the handle has active borrows (NumLends > 0).
func LiftOwn(ctx *LiftContext, handleIdx uint32) (any, error) {
	h := component.MakeHandle(handleIdx, 0) // Generation checked in Remove

	// For proper generation checking, we need to track it
	// This is simplified - full impl needs to preserve generation
	entry, err := ctx.ResourceTable.Get(component.Handle(handleIdx))
	if err != nil {
		return nil, fmt.Errorf("lift_own: %w", err)
	}
	if !entry.Own {
		return nil, errors.New("lift_own: handle is not owned")
	}

	// Remove from table (checks NumLends > 0)
	removed, err := ctx.ResourceTable.Remove(component.Handle(handleIdx))
	if err != nil {
		return nil, fmt.Errorf("lift_own: %w", err)
	}

	return removed.Rep, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/abi/... -v -run TestLiftOwn`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/abi/lift.go internal/component/abi/lift_test.go
git commit -m "$(cat <<'EOF'
feat(component): implement LiftOwn for ownership transfer

LiftOwn removes an owned handle from the table and returns its
representation. Traps if handle has active borrows.

Implements canon_lift for own<T> types.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 86: Implement LowerOwn in ABI

**Files:**
- Modify: `internal/component/abi/lower.go`
- Modify: `internal/component/abi/lower_test.go`

**Step 1: Write failing test**

```go
// Add to internal/component/abi/lower_test.go

func TestLowerOwn(t *testing.T) {
	table := component.NewResourceTable()

	ctx := &LowerContext{
		ResourceTable: table,
	}

	// Lower creates a new handle in the table
	handleIdx, err := LowerOwn(ctx, "my-resource")
	require.NoError(t, err)

	// Should be in table now
	h := component.MakeHandle(handleIdx, 0)
	entry, err := table.Get(h)
	require.NoError(t, err)
	require.Equal(t, "my-resource", entry.Rep)
	require.True(t, entry.Own)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/abi/... -v -run TestLowerOwn`
Expected: FAIL with "undefined: LowerOwn"

**Step 3: Implement LowerOwn**

```go
// Add to internal/component/abi/lower.go

// LowerOwn receives ownership of a resource into the component.
// Creates a new owned handle in the table and returns its index.
func LowerOwn(ctx *LowerContext, rep any) (uint32, error) {
	h := ctx.ResourceTable.New(rep, true)
	return h.Index(), nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/abi/... -v -run TestLowerOwn`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/abi/lower.go internal/component/abi/lower_test.go
git commit -m "$(cat <<'EOF'
feat(component): implement LowerOwn for ownership receipt

LowerOwn creates a new owned handle in the table for an incoming
resource representation.

Implements canon_lower for own<T> types.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 87: Implement LiftBorrow in ABI

**Files:**
- Modify: `internal/component/abi/lift.go`
- Modify: `internal/component/abi/lift_test.go`

**Step 1: Write failing test**

```go
// Add to internal/component/abi/lift_test.go

func TestLiftBorrow(t *testing.T) {
	table := component.NewResourceTable()
	h := table.New("my-resource", true)
	scope := component.NewBorrowScope(table)

	ctx := &LiftContext{
		ResourceTable: table,
		BorrowScope:   scope,
	}

	// Lift borrow (reads but doesn't remove)
	rep, err := LiftBorrow(ctx, uint32(h.Index()))
	require.NoError(t, err)
	require.Equal(t, "my-resource", rep)

	// Handle should still be in table
	entry, err := table.Get(h)
	require.NoError(t, err)
	require.Equal(t, "my-resource", entry.Rep)

	// NumLends should be incremented
	require.Equal(t, uint32(1), entry.NumLends)

	// Release scope
	scope.Release()

	entry, _ = table.Get(h)
	require.Equal(t, uint32(0), entry.NumLends)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/abi/... -v -run TestLiftBorrow`
Expected: FAIL with "undefined: LiftBorrow"

**Step 3: Implement LiftBorrow**

```go
// Add to internal/component/abi/lift.go

// LiftBorrow reads a resource representation for borrowing.
// Does not remove from table, but tracks the lend in the borrow scope.
func LiftBorrow(ctx *LiftContext, handleIdx uint32) (any, error) {
	h := component.Handle(handleIdx)

	entry, err := ctx.ResourceTable.Get(h)
	if err != nil {
		return nil, fmt.Errorf("lift_borrow: %w", err)
	}

	// Track the lend in the borrow scope
	if ctx.BorrowScope != nil {
		if err := ctx.BorrowScope.AddLender(h); err != nil {
			return nil, fmt.Errorf("lift_borrow: tracking lend: %w", err)
		}
	}

	return entry.Rep, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/abi/... -v -run TestLiftBorrow`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/abi/lift.go internal/component/abi/lift_test.go
git commit -m "$(cat <<'EOF'
feat(component): implement LiftBorrow for resource lending

LiftBorrow reads a resource representation without removing from table.
Tracks the lend in BorrowScope to prevent ownership transfer while
borrowed.

Implements canon_lift for borrow<T> types.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 88: Implement LowerBorrow in ABI

**Files:**
- Modify: `internal/component/abi/lower.go`
- Modify: `internal/component/abi/lower_test.go`

**Step 1: Write failing test**

```go
// Add to internal/component/abi/lower_test.go

func TestLowerBorrow(t *testing.T) {
	table := component.NewResourceTable()
	callCtx := component.NewCallContext()

	ctx := &LowerContext{
		ResourceTable: table,
		CallContext:   callCtx,
	}

	// Lower borrow creates a borrowed handle
	handleIdx, err := LowerBorrow(ctx, "my-resource")
	require.NoError(t, err)

	// Should be in table as borrowed
	h := component.MakeHandle(handleIdx, 0)
	entry, err := table.Get(h)
	require.NoError(t, err)
	require.Equal(t, "my-resource", entry.Rep)
	require.False(t, entry.Own) // Borrowed, not owned

	// CallContext should track the borrow
	require.Equal(t, 1, callCtx.NumBorrows())
}

func TestLowerBorrow_SameInstance(t *testing.T) {
	// When lowering to same instance as resource implementer,
	// we can return rep directly without table entry
	// (This is an optimization - test the basic case first)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/abi/... -v -run TestLowerBorrow`
Expected: FAIL with "undefined: LowerBorrow"

**Step 3: Implement LowerBorrow**

```go
// Add to internal/component/abi/lower.go

// LowerBorrow receives a borrowed resource into the component.
// Creates a borrowed handle in the table and tracks it in CallContext.
func LowerBorrow(ctx *LowerContext, rep any) (uint32, error) {
	h := ctx.ResourceTable.New(rep, false) // own=false for borrowed

	// Track borrow in call context for return validation
	if ctx.CallContext != nil {
		ctx.CallContext.IncrementBorrows()
	}

	return h.Index(), nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/abi/... -v -run TestLowerBorrow`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/abi/lower.go internal/component/abi/lower_test.go
git commit -m "$(cat <<'EOF'
feat(component): implement LowerBorrow for receiving borrowed resources

LowerBorrow creates a borrowed (non-owning) handle in the table.
Tracks the borrow in CallContext to enforce drop-before-return.

Implements canon_lower for borrow<T> types.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 89: Implement canon resource.new

**Files:**
- Modify: `internal/component/instance.go`
- Modify: `internal/component/instance_test.go`

**Step 1: Write failing test**

```go
// Add to internal/component/instance_test.go

func TestCanonResourceNew(t *testing.T) {
	inst := &Instance{
		resourceTable: component.NewResourceTable(),
	}

	// resource.new creates an owned handle from rep
	handleIdx, err := inst.ResourceNew(42) // rep = 42
	require.NoError(t, err)

	h := component.MakeHandle(handleIdx, 0)
	entry, err := inst.resourceTable.Get(h)
	require.NoError(t, err)
	require.Equal(t, 42, entry.Rep)
	require.True(t, entry.Own)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -v -run TestCanonResourceNew`
Expected: FAIL with method not found

**Step 3: Implement ResourceNew**

```go
// Add to internal/component/instance.go

// ResourceNew implements canon resource.new.
// Creates an owned handle from a representation value.
func (i *Instance) ResourceNew(rep any) (uint32, error) {
	if i.resourceTable == nil {
		i.resourceTable = NewResourceTable()
	}
	h := i.resourceTable.New(rep, true)
	return h.Index(), nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -v -run TestCanonResourceNew`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/instance.go internal/component/instance_test.go
git commit -m "$(cat <<'EOF'
feat(component): implement canon resource.new

ResourceNew creates an owned handle from a representation value.
Implements the canonical resource.new intrinsic.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 90: Implement canon resource.rep

**Files:**
- Modify: `internal/component/instance.go`
- Modify: `internal/component/instance_test.go`

**Step 1: Write failing test**

```go
// Add to internal/component/instance_test.go

func TestCanonResourceRep(t *testing.T) {
	inst := &Instance{
		resourceTable: component.NewResourceTable(),
	}

	handleIdx, _ := inst.ResourceNew(42)

	// resource.rep extracts the representation
	rep, err := inst.ResourceRep(handleIdx)
	require.NoError(t, err)
	require.Equal(t, 42, rep)

	// Handle is still valid after rep (unlike drop)
	_, err = inst.resourceTable.Get(component.MakeHandle(handleIdx, 0))
	require.NoError(t, err)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -v -run TestCanonResourceRep`
Expected: FAIL with method not found

**Step 3: Implement ResourceRep**

```go
// Add to internal/component/instance.go

// ResourceRep implements canon resource.rep.
// Extracts the representation from a handle without removing it.
// Only the resource's defining component can call this.
func (i *Instance) ResourceRep(handleIdx uint32) (any, error) {
	h := Handle(handleIdx)
	entry, err := i.resourceTable.Get(h)
	if err != nil {
		return nil, fmt.Errorf("resource.rep: %w", err)
	}
	return entry.Rep, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -v -run TestCanonResourceRep`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/instance.go internal/component/instance_test.go
git commit -m "$(cat <<'EOF'
feat(component): implement canon resource.rep

ResourceRep extracts representation from handle without removing.
Implements the canonical resource.rep intrinsic.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 91: Implement canon resource.drop for owned handles

**Files:**
- Modify: `internal/component/instance.go`
- Modify: `internal/component/instance_test.go`

**Step 1: Write failing test**

```go
// Add to internal/component/instance_test.go

func TestCanonResourceDrop_Owned(t *testing.T) {
	dtorCalled := false
	inst := &Instance{
		resourceTable: NewResourceTable(),
		destructors: map[uint32]func(any){
			0: func(rep any) { dtorCalled = true },
		},
	}

	handleIdx, _ := inst.ResourceNew(42)

	// resource.drop removes and calls destructor
	err := inst.ResourceDrop(handleIdx, 0) // resourceTypeIdx = 0
	require.NoError(t, err)
	require.True(t, dtorCalled)

	// Handle is no longer valid
	_, err = inst.resourceTable.Get(MakeHandle(handleIdx, 0))
	require.Error(t, err)
}

func TestCanonResourceDrop_NoDestructor(t *testing.T) {
	inst := &Instance{
		resourceTable: NewResourceTable(),
	}

	handleIdx, _ := inst.ResourceNew(42)

	// resource.drop with no destructor just removes
	err := inst.ResourceDrop(handleIdx, 0)
	require.NoError(t, err)

	// Handle is gone
	_, err = inst.resourceTable.Get(MakeHandle(handleIdx, 0))
	require.Error(t, err)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -v -run TestCanonResourceDrop`
Expected: FAIL with method not found

**Step 3: Implement ResourceDrop**

```go
// Add to internal/component/instance.go

// ResourceDrop implements canon resource.drop.
// Removes the handle and invokes destructor if owned.
func (i *Instance) ResourceDrop(handleIdx uint32, resourceTypeIdx uint32) error {
	h := Handle(handleIdx)
	entry, err := i.resourceTable.Remove(h)
	if err != nil {
		return fmt.Errorf("resource.drop: %w", err)
	}

	// Call destructor for owned handles
	if entry.Own {
		if i.destructors != nil {
			if dtor, ok := i.destructors[resourceTypeIdx]; ok {
				dtor(entry.Rep)
			}
		}
	}

	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -v -run TestCanonResourceDrop`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/instance.go internal/component/instance_test.go
git commit -m "$(cat <<'EOF'
feat(component): implement canon resource.drop for owned handles

ResourceDrop removes handle from table and invokes destructor
for owned handles. Implements the canonical resource.drop intrinsic.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 92: Implement resource.drop for borrowed handles

**Files:**
- Modify: `internal/component/instance.go`
- Modify: `internal/component/instance_test.go`

**Step 1: Write failing test**

```go
// Add to internal/component/instance_test.go

func TestCanonResourceDrop_Borrowed(t *testing.T) {
	inst := &Instance{
		resourceTable: NewResourceTable(),
		callContext:   NewCallContext(),
	}

	// Create borrowed handle (simulating lower_borrow)
	h := inst.resourceTable.New("resource", false) // own=false
	inst.callContext.IncrementBorrows()

	// resource.drop on borrowed decrements call context
	err := inst.ResourceDropBorrowed(h.Index())
	require.NoError(t, err)

	// CallContext borrow count decremented
	require.Equal(t, 0, inst.callContext.NumBorrows())

	// Handle is removed
	_, err = inst.resourceTable.Get(h)
	require.Error(t, err)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -v -run TestCanonResourceDrop_Borrowed`
Expected: FAIL

**Step 3: Update ResourceDrop to handle borrowed**

```go
// Update ResourceDrop in internal/component/instance.go

func (i *Instance) ResourceDrop(handleIdx uint32, resourceTypeIdx uint32) error {
	h := Handle(handleIdx)
	entry, err := i.resourceTable.Remove(h)
	if err != nil {
		return fmt.Errorf("resource.drop: %w", err)
	}

	if entry.Own {
		// Call destructor for owned handles
		if i.destructors != nil {
			if dtor, ok := i.destructors[resourceTypeIdx]; ok {
				dtor(entry.Rep)
			}
		}
	} else {
		// Decrement borrow count for borrowed handles
		if i.callContext != nil {
			i.callContext.DecrementBorrows()
		}
	}

	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -v -run TestCanonResourceDrop_Borrowed`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/instance.go internal/component/instance_test.go
git commit -m "$(cat <<'EOF'
feat(component): handle borrowed handles in resource.drop

For borrowed handles, resource.drop decrements the call context's
borrow count instead of invoking a destructor. This enforces
drop-before-return semantics.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 93: Add resource table to Instance struct

**Files:**
- Modify: `internal/component/instance.go`
- Modify: `internal/component/instance_test.go`

**Step 1: Write failing test**

```go
// Add to internal/component/instance_test.go

func TestInstance_ResourceTableInit(t *testing.T) {
	inst := NewInstance(nil, nil) // component, coreInstances
	require.NotNil(t, inst.ResourceTable())
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -v -run TestInstance_ResourceTableInit`
Expected: FAIL (depending on current implementation)

**Step 3: Ensure Instance has ResourceTable accessor**

```go
// Add to internal/component/instance.go

// ResourceTable returns the instance's resource table.
func (i *Instance) ResourceTable() *ResourceTable {
	if i.resourceTable == nil {
		i.resourceTable = NewResourceTable()
	}
	return i.resourceTable
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -v -run TestInstance_ResourceTableInit`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/instance.go internal/component/instance_test.go
git commit -m "$(cat <<'EOF'
feat(component): add ResourceTable accessor to Instance

Instance lazily initializes a ResourceTable for managing
resource handles during execution.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 94: Integrate lift/lower for own/borrow in function calls

**Files:**
- Modify: `internal/component/instance.go`
- Modify: `internal/component/instance_test.go`

**Step 1: Write failing test**

```go
// Add to internal/component/instance_test.go

func TestExportedFunc_CallWithOwn(t *testing.T) {
	// This test will require a component that accepts own<T>
	// For now, test the infrastructure
	ctx := context.Background()
	table := NewResourceTable()
	h := table.New("test-resource", true)

	liftCtx := &abi.LiftContext{
		ResourceTable: table,
	}

	// Lift own should work
	rep, err := abi.LiftOwn(liftCtx, h.Index())
	require.NoError(t, err)
	require.Equal(t, "test-resource", rep)
}
```

**Step 2-5: Implementation and commit**

(Follow similar pattern - integrate lift/lower calls in ExportedFunc.Call)

---

### Task 95: Create test component generator for resources

**Files:**
- Create: `internal/component/testdata/gen/resource_own.go`
- Create: `internal/component/testdata/resources/resource_own.wasm`

**Step 1: Write the generator**

```go
// internal/component/testdata/gen/resource_own.go

package main

// This generator creates a component that:
// - Defines a resource type
// - Exports a function that creates and returns an owned resource
// - Exports a function that consumes an owned resource

// WIT:
// package test:resources;
//
// interface resources {
//     resource counter {
//         constructor(initial: s32);
//         increment: func();
//         get: func() -> s32;
//     }
// }
//
// world test-resources {
//     export resources;
// }
```

**Step 2-5: Build and commit**

```bash
cd internal/component/testdata/gen
cargo component build --release
cp target/wasm32-wasip1/release/resource_own.wasm ../resources/
git add internal/component/testdata/gen/resource_own.go internal/component/testdata/resources/resource_own.wasm
git commit -m "$(cat <<'EOF'
test(component): add resource_own test component

Creates a test component with resource type definition
for integration testing of own<T> handles.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 96: Integration test for resource creation and drop

**Files:**
- Create: `internal/component/resource_integration_test.go`

**Step 1: Write failing test**

```go
// internal/component/resource_integration_test.go

package component_test

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/internal/component/testdata"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestResourceOwn_CreateAndDrop(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	binary, err := testdata.ReadFile("testdata/resources/resource_own.wasm")
	require.NoError(t, err)

	compiled, err := rt.CompileComponent(ctx, binary)
	require.NoError(t, err)

	linker := rt.NewComponentLinker()
	instance, err := linker.Instantiate(ctx, compiled)
	require.NoError(t, err)

	// Create a resource
	createFn := instance.ExportedFunction("create")
	results, err := createFn.Call(ctx, ValS32(10))
	require.NoError(t, err)

	// Should get an owned handle
	handle := results[0].Own()

	// Drop the resource
	dropFn := instance.ExportedFunction("drop")
	_, err := dropFn.Call(ctx, ValOwn(handle))
	require.NoError(t, err)
}
```

**Step 2-5: Implement and commit**

---

### Task 97: Integration test for borrow semantics

**Files:**
- Modify: `internal/component/resource_integration_test.go`

**Step 1: Write failing test**

```go
// Add to internal/component/resource_integration_test.go

func TestResourceBorrow_UseWithoutTransfer(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	binary, _ := testdata.ReadFile("testdata/resources/resource_borrow.wasm")
	compiled, _ := rt.CompileComponent(ctx, binary)
	linker := rt.NewComponentLinker()
	instance, _ := linker.Instantiate(ctx, compiled)

	// Create resource
	createFn := instance.ExportedFunction("create")
	results, _ := createFn.Call(ctx, ValS32(42))
	handle := results[0].Own()

	// Borrow the resource (doesn't transfer ownership)
	readFn := instance.ExportedFunction("read") // takes borrow<T>
	results, err := readFn.Call(ctx, ValOwn(handle)) // own implicitly borrows
	require.NoError(t, err)
	require.Equal(t, int32(42), results[0].S32())

	// Handle still valid (wasn't transferred)
	readFn.Call(ctx, ValOwn(handle)) // can borrow again
}
```

**Step 2-5: Implement and commit**

---

### Task 98: Test double-drop error (wasmtime: drop_guest_twice)

**Files:**
- Modify: `internal/component/resource_integration_test.go`

**Step 1: Write failing test**

```go
// Add to internal/component/resource_integration_test.go

func TestResourceOwn_DoubleDrop(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	binary, _ := testdata.ReadFile("testdata/resources/resource_own.wasm")
	compiled, _ := rt.CompileComponent(ctx, binary)
	linker := rt.NewComponentLinker()
	instance, _ := linker.Instantiate(ctx, compiled)

	// Create and drop once
	createFn := instance.ExportedFunction("create")
	results, _ := createFn.Call(ctx, ValS32(10))
	handle := results[0].Own()

	dropFn := instance.ExportedFunction("drop")
	_, err := dropFn.Call(ctx, ValOwn(handle))
	require.NoError(t, err)

	// Second drop should fail: "unknown handle index"
	_, err = dropFn.Call(ctx, ValOwn(handle))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid") // or "unknown handle"
}
```

**Step 2-5: Verify and commit**

---

### Task 99: Test active borrows at return (wasmtime: active_borrows_at_end_of_call)

**Files:**
- Modify: `internal/component/resource_integration_test.go`

**Step 1: Write failing test**

```go
// Add to internal/component/resource_integration_test.go

func TestResourceBorrow_ActiveAtReturn(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	// Component that receives borrow but doesn't drop before return
	binary, _ := testdata.ReadFile("testdata/resources/resource_borrow_leak.wasm")
	compiled, _ := rt.CompileComponent(ctx, binary)
	linker := rt.NewComponentLinker()
	instance, _ := linker.Instantiate(ctx, compiled)

	createFn := instance.ExportedFunction("create")
	results, _ := createFn.Call(ctx)
	handle := results[0].Own()

	// Call function that takes borrow but doesn't drop
	leakFn := instance.ExportedFunction("leak_borrow")
	_, err := leakFn.Call(ctx, ValOwn(handle))

	// Should trap: "borrow handles still remain"
	require.Error(t, err)
	require.Contains(t, err.Error(), "borrow")
}
```

**Step 2-5: Verify and commit**

---

### Task 100: Test cannot use borrow for own (wasmtime: cannot_use_borrow_for_own)

**Files:**
- Modify: `internal/component/resource_integration_test.go`

**Step 1: Write failing test**

```go
// Add to internal/component/resource_integration_test.go

func TestResource_CannotUseBorrowForOwn(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	binary, _ := testdata.ReadFile("testdata/resources/resource_own.wasm")
	compiled, _ := rt.CompileComponent(ctx, binary)
	linker := rt.NewComponentLinker()
	instance, _ := linker.Instantiate(ctx, compiled)

	// Try to pass a borrow where own is expected
	// This is a type error that should be caught during validation or at call time
	consumeFn := instance.ExportedFunction("consume") // takes own<T>

	// Creating a borrowed handle directly and passing it should fail
	_, err := consumeFn.Call(ctx, ValBorrow(42)) // borrow instead of own
	require.Error(t, err)
}
```

**Step 2-5: Verify and commit**

---

## Running Tests

```bash
# Run all resource tests
go test ./internal/component/... -v -run Resource

# Run resource table tests
go test ./internal/component/... -v -run TestResourceTable

# Run borrow scope tests
go test ./internal/component/... -v -run TestBorrowScope

# Run call context tests
go test ./internal/component/... -v -run TestCallContext

# Run integration tests
go test ./internal/component/... -v -run TestResourceOwn
go test ./internal/component/... -v -run TestResourceBorrow

# Run with race detector
go test ./internal/component/... -race -v

# Run all component tests
go test ./internal/component/... -v
```

---

## Phase 3 Completion Checklist

- [ ] Own and Borrow types implement ValType
- [ ] ResourceTable with generation-counted handles
- [ ] BorrowScope for call-scoped tracking
- [ ] CallContext enforces drop-before-return
- [ ] Binary parser handles 0x3f (resource), 0x69 (own), 0x68 (borrow)
- [ ] LiftOwn, LowerOwn, LiftBorrow, LowerBorrow implemented
- [ ] canon resource.new, resource.drop, resource.rep work
- [ ] Destructors invoked on owned handle drop
- [ ] Double-drop returns error
- [ ] Active borrows at return traps
- [ ] Integration tests pass with real components

---

## References

- [Canonical ABI - Resources](https://github.com/WebAssembly/component-model/blob/main/design/mvp/CanonicalABI.md)
- [Component Model Explainer - Resources](https://github.com/WebAssembly/component-model/blob/main/design/mvp/Explainer.md)
- [Binary Format - Resource Types](https://github.com/WebAssembly/component-model/blob/main/design/mvp/Binary.md)
- [Wasmtime tests/all/component_model/resources.rs](https://github.com/bytecodealliance/wasmtime/blob/main/tests/all/component_model/resources.rs)
