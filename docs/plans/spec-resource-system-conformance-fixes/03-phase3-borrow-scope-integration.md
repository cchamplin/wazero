# Phase 3: Borrow Scope Integration

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Integrate BorrowScope with CallContext for proper borrow tracking and implement the lower_borrow same-instance optimization.

**Architecture:** Add lenders tracking to CallContext, implement exit_call with undo_lend, add the same-instance optimization for lower_borrow, and ensure borrow drops decrement the borrow_scope.num_borrows counter.

**Tech Stack:** Go

---

## Prerequisites

- Complete Phase 1 (Core Type System) and Phase 2 (Trap Conditions)
- Read gap analysis: `docs/plans/resource-system-gap-analysis.md` (Sections 4 and 5)
- Understand spec: `debug-vendored/component-model/design/mvp/CanonicalABI.md` lines 2234-2240, 2677-2683

## Reference: Spec Borrow Operations

From lift_borrow (CanonicalABI.md:2234-2240):
```python
def lift_borrow(cx, i, t):
  assert(isinstance(cx.borrow_scope, Subtask))
  h = cx.inst.table.get(i)
  trap_if(not isinstance(h, ResourceHandle))
  trap_if(h.rt is not t.rt)
  cx.borrow_scope.add_lender(h)  # <-- Track lender
  return h.rep
```

From lower_borrow (CanonicalABI.md:2677-2683):
```python
def lower_borrow(cx, rep, t):
  assert(isinstance(cx.borrow_scope, Task))
  if cx.inst is t.rt.impl:
    return rep  # <-- Same-instance optimization!
  h = ResourceHandle(t.rt, rep, own = False, borrow_scope = cx.borrow_scope)
  h.borrow_scope.num_borrows += 1  # <-- Track borrow count
  return cx.inst.table.add(h)
```

From wasmtime CallContext (resources.rs:187-192):
```rust
pub struct CallContext {
    lenders: Vec<TypedResourceIndex>,  // <-- We need this
    borrow_count: u32,
}
```

---

## Task 3.1: Add Lenders Tracking to CallContext

**Files:**
- Modify: `internal/component/call_context.go`
- Test: `internal/component/call_context_test.go`

**Step 1: Write the failing test**

Add to `internal/component/call_context_test.go`:
```go
func TestCallContext_TrackLenders(t *testing.T) {
	ctx := NewCallContext()

	// Add some lender handles
	h1 := MakeHandle(1, 0)
	h2 := MakeHandle(2, 0)
	h3 := MakeHandle(3, 0)

	ctx.AddLender(h1)
	ctx.AddLender(h2)
	ctx.AddLender(h3)

	lenders := ctx.Lenders()
	require.Len(t, lenders, 3)
	require.Contains(t, lenders, h1)
	require.Contains(t, lenders, h2)
	require.Contains(t, lenders, h3)
}

func TestCallContext_ClearLenders(t *testing.T) {
	ctx := NewCallContext()

	ctx.AddLender(MakeHandle(1, 0))
	ctx.AddLender(MakeHandle(2, 0))

	require.Len(t, ctx.Lenders(), 2)

	ctx.ClearLenders()
	require.Len(t, ctx.Lenders(), 0)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -run "TestCallContext_TrackLenders|TestCallContext_ClearLenders" -v`
Expected: FAIL with "AddLender undefined" or "Lenders undefined"

**Step 3: Write minimal implementation**

Modify `internal/component/call_context.go`:
```go
// internal/component/call_context.go

package component

import "errors"

var ErrOutstandingBorrows = errors.New("cannot return: borrow handles still remain")

// CallContext tracks state for a single component function call.
// Implements the Canonical ABI's Task tracking for borrow validation.
type CallContext struct {
	numBorrows int      // Number of borrowed handles received by this call
	lenders    []Handle // Handles that were borrowed FROM during this call (for undo_lend)
}

// NewCallContext creates a new call context.
func NewCallContext() *CallContext {
	return &CallContext{
		lenders: make([]Handle, 0),
	}
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

// ValidateReturn checks if returning is allowed.
// Returns an error if there are outstanding borrowed handles.
func (c *CallContext) ValidateReturn() error {
	if c.numBorrows > 0 {
		return ErrOutstandingBorrows
	}
	return nil
}

// AddLender records a handle that was borrowed FROM during this call.
// This is used by lift_borrow to track which handles need their num_lends
// decremented when the call exits.
func (c *CallContext) AddLender(h Handle) {
	c.lenders = append(c.lenders, h)
}

// Lenders returns the list of handles that were borrowed from.
func (c *CallContext) Lenders() []Handle {
	return c.lenders
}

// ClearLenders clears the lenders list (called after exit_call processes them).
func (c *CallContext) ClearLenders() {
	c.lenders = c.lenders[:0]
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -run "TestCallContext_TrackLenders|TestCallContext_ClearLenders" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/call_context.go internal/component/call_context_test.go
git commit -m "feat(resource): add lenders tracking to CallContext"
```

---

## Task 3.2: Implement ExitCall with Undo Lend

**Files:**
- Modify: `internal/component/call_context.go`
- Test: `internal/component/call_context_test.go`

**Step 1: Write the failing test**

Add to `internal/component/call_context_test.go`:
```go
func TestCallContext_ExitCall_UndoesLends(t *testing.T) {
	table := NewResourceTable()

	// Create a resource
	h := table.New("resource", true)

	// Simulate lift_borrow: increment lends on the source handle
	require.NoError(t, table.IncrementLends(h))
	require.NoError(t, table.IncrementLends(h))

	// Verify lends are incremented
	entry, _ := table.Get(h)
	require.Equal(t, uint32(2), entry.NumLends)

	// Create call context and track lenders
	ctx := NewCallContext()
	ctx.AddLender(h)
	ctx.AddLender(h) // Same handle borrowed twice

	// Exit call should undo all lends
	err := ctx.ExitCall(table)
	require.NoError(t, err)

	// Verify lends are decremented
	entry, _ = table.Get(h)
	require.Equal(t, uint32(0), entry.NumLends)

	// Lenders should be cleared
	require.Len(t, ctx.Lenders(), 0)
}

func TestCallContext_ExitCall_FailsWithOutstandingBorrows(t *testing.T) {
	table := NewResourceTable()

	ctx := NewCallContext()
	ctx.IncrementBorrows() // Simulate unreleased borrow

	err := ctx.ExitCall(table)
	require.ErrorIs(t, err, ErrOutstandingBorrows)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -run "TestCallContext_ExitCall" -v`
Expected: FAIL with "ExitCall undefined"

**Step 3: Write minimal implementation**

Add to `internal/component/call_context.go`:
```go
// ExitCall validates that the call can return and undoes all lend operations.
// This is called when a call scope completes.
// Returns an error if there are outstanding borrows (handles not dropped).
func (c *CallContext) ExitCall(table *ResourceTable) error {
	// Spec: trap if borrow_count > 0
	if err := c.ValidateReturn(); err != nil {
		return err
	}

	// Undo all lend operations (decrement num_lends on source handles)
	for _, h := range c.lenders {
		// Ignore errors from already-removed handles (can happen if
		// the source handle was transferred during the call)
		_ = table.DecrementLends(h)
	}

	c.ClearLenders()
	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -run "TestCallContext_ExitCall" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/call_context.go internal/component/call_context_test.go
git commit -m "feat(resource): implement ExitCall with undo_lend for borrow tracking"
```

---

## Task 3.3: Add ComponentInstanceID to ResourceType for Same-Instance Detection

**Files:**
- Modify: `internal/component/resource_type_id.go`
- Test: `internal/component/resource_type_id_test.go`

**Step 1: Write the failing test**

Add to `internal/component/resource_type_id_test.go`:
```go
func TestResourceTypeInfo_SameInstance(t *testing.T) {
	info1 := NewResourceTypeInfo(1, 100) // type 1 in instance 100
	info2 := NewResourceTypeInfo(2, 100) // type 2 in instance 100
	info3 := NewResourceTypeInfo(1, 200) // type 1 in instance 200

	require.True(t, info1.SameInstance(info2), "same instance should match")
	require.False(t, info1.SameInstance(info3), "different instance should not match")
}

func TestResourceTypeInfo_InstanceID(t *testing.T) {
	info := NewResourceTypeInfo(5, 42)
	require.Equal(t, uint32(42), info.InstanceID())
	require.Equal(t, uint32(5), info.TypeIndex())
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -run "TestResourceTypeInfo" -v`
Expected: FAIL with "NewResourceTypeInfo undefined"

**Step 3: Write minimal implementation**

Add to `internal/component/resource_type_id.go`:
```go
// ResourceTypeInfo contains extended information about a resource type,
// including which component instance defines it.
// This is needed for the lower_borrow same-instance optimization.
type ResourceTypeInfo struct {
	typeID     ResourceTypeID
	instanceID uint32 // ID of the component instance that defines this type
}

// NewResourceTypeInfo creates a ResourceTypeInfo from a type index and instance ID.
func NewResourceTypeInfo(typeIndex uint32, instanceID uint32) ResourceTypeInfo {
	return ResourceTypeInfo{
		typeID:     NewResourceTypeID(typeIndex),
		instanceID: instanceID,
	}
}

// TypeID returns the ResourceTypeID.
func (r ResourceTypeInfo) TypeID() ResourceTypeID {
	return r.typeID
}

// TypeIndex returns the type index.
func (r ResourceTypeInfo) TypeIndex() uint32 {
	return r.typeID.Index()
}

// InstanceID returns the defining component instance ID.
func (r ResourceTypeInfo) InstanceID() uint32 {
	return r.instanceID
}

// SameInstance returns true if this type is defined in the same instance as other.
func (r ResourceTypeInfo) SameInstance(other ResourceTypeInfo) bool {
	return r.instanceID == other.instanceID
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -run "TestResourceTypeInfo" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/resource_type_id.go internal/component/resource_type_id_test.go
git commit -m "feat(resource): add ResourceTypeInfo for same-instance detection"
```

---

## Task 3.4: Implement lower_borrow Same-Instance Optimization

**Files:**
- Create: `internal/component/abi/resource_lower.go`
- Test: `internal/component/abi/resource_lower_test.go`

**Step 1: Write the failing test**

Create `internal/component/abi/resource_lower_test.go`:
```go
package abi

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestLowerBorrow_SameInstance_ReturnsRep(t *testing.T) {
	table := component.NewResourceTable()
	callCtx := component.NewCallContext()

	// Resource type defined in instance 100
	resourceTypeInfo := component.NewResourceTypeInfo(1, 100)

	// Lower borrow FROM instance 100 (same as defining instance)
	currentInstanceID := uint32(100)

	result, err := LowerBorrow(table, callCtx, 42, resourceTypeInfo, currentInstanceID)
	require.NoError(t, err)

	// Should return rep directly (same-instance optimization)
	require.Equal(t, uint32(42), result)

	// No handle should be created in the table
	// (can verify by checking no borrow was incremented)
	require.Equal(t, 0, callCtx.NumBorrows())
}

func TestLowerBorrow_DifferentInstance_CreatesHandle(t *testing.T) {
	table := component.NewResourceTable()
	callCtx := component.NewCallContext()

	// Resource type defined in instance 100
	resourceTypeInfo := component.NewResourceTypeInfo(1, 100)

	// Lower borrow FROM instance 200 (different from defining instance)
	currentInstanceID := uint32(200)

	result, err := LowerBorrow(table, callCtx, 42, resourceTypeInfo, currentInstanceID)
	require.NoError(t, err)

	// Should return a handle index (not the rep directly)
	require.NotEqual(t, uint32(42), result)

	// A borrow should be tracked
	require.Equal(t, 1, callCtx.NumBorrows())

	// Handle should exist in the table
	entry, err := table.Get(component.Handle(result))
	require.NoError(t, err)
	require.False(t, entry.Own, "should be a borrow, not own")
	require.Equal(t, uint32(42), entry.Rep.(uint32))
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/abi/... -run "TestLowerBorrow" -v`
Expected: FAIL with "LowerBorrow undefined"

**Step 3: Write minimal implementation**

Create `internal/component/abi/resource_lower.go`:
```go
// internal/component/abi/resource_lower.go

package abi

import (
	"github.com/tetratelabs/wazero/internal/component"
)

// LowerBorrow implements the Canonical ABI lower_borrow function.
// It has a special optimization: when lowering a borrow to the same component
// instance that defined the resource type, it returns the rep directly.
//
// From spec (CanonicalABI.md:2677-2683):
//
//	def lower_borrow(cx, rep, t):
//	  if cx.inst is t.rt.impl:
//	    return rep  # Same-instance optimization
//	  h = ResourceHandle(t.rt, rep, own = False, borrow_scope = cx.borrow_scope)
//	  h.borrow_scope.num_borrows += 1
//	  return cx.inst.table.add(h)
func LowerBorrow(
	table *component.ResourceTable,
	callCtx *component.CallContext,
	rep uint32,
	resourceType component.ResourceTypeInfo,
	currentInstanceID uint32,
) (uint32, error) {
	// Same-instance optimization: return rep directly
	if currentInstanceID == resourceType.InstanceID() {
		return rep, nil
	}

	// Different instance: create a borrow handle
	handle := table.NewWithType(rep, false, resourceType.TypeID())

	// Track the borrow in the call context
	callCtx.IncrementBorrows()

	return uint32(handle), nil
}

// LowerOwn implements the Canonical ABI lower_own function.
// Creates an owning handle in the table.
//
// From spec (CanonicalABI.md:2673-2675):
//
//	def lower_own(cx, rep, t):
//	  h = ResourceHandle(t.rt, rep, own = True)
//	  return cx.inst.table.add(h)
func LowerOwn(
	table *component.ResourceTable,
	rep uint32,
	resourceType component.ResourceTypeInfo,
) (uint32, error) {
	handle := table.NewWithType(rep, true, resourceType.TypeID())
	return uint32(handle), nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/abi/... -run "TestLowerBorrow" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/abi/resource_lower.go internal/component/abi/resource_lower_test.go
git commit -m "feat(resource): implement lower_borrow with same-instance optimization"
```

---

## Task 3.5: Add Borrow Count Decrement on Borrow Handle Drop

**Files:**
- Modify: `internal/component/resource_table.go`
- Test: `internal/component/resource_table_test.go`

**Step 1: Write the failing test**

Add to `internal/component/resource_table_test.go`:
```go
func TestResourceTable_RemoveBorrow_DecrementsBorrowCount(t *testing.T) {
	table := NewResourceTable()
	callCtx := NewCallContext()

	// Simulate lower_borrow: create borrow handle and increment borrow count
	h := table.NewWithType(uint32(42), false, NewResourceTypeID(1)) // own=false
	callCtx.IncrementBorrows()

	require.Equal(t, 1, callCtx.NumBorrows())

	// Remove the borrow handle (simulates resource.drop on a borrow)
	entry, err := table.Remove(h)
	require.NoError(t, err)
	require.False(t, entry.Own)

	// If this was a borrow, we need to decrement the borrow count
	if !entry.Own {
		callCtx.DecrementBorrows()
	}

	require.Equal(t, 0, callCtx.NumBorrows())
}
```

Note: This test documents the expected integration pattern. The actual decrement
happens in the calling code (resource.drop implementation), not in Remove itself.

**Step 2: Run test to verify it passes**

Run: `go test ./internal/component/... -run "TestResourceTable_RemoveBorrow_DecrementsBorrowCount" -v`
Expected: PASS (this is documenting existing behavior)

**Step 3: Document the integration pattern**

Add documentation comment to `internal/component/resource_table.go` at the Remove method:
```go
// Remove removes a handle from the table and returns its entry.
// Used for lift_own to transfer ownership out of the component.
// Traps if the handle has active borrows (NumLends > 0).
//
// For borrow handles (Own == false), the caller (resource.drop implementation)
// is responsible for decrementing the borrow count in the call context:
//
//	entry, err := table.Remove(h)
//	if err != nil { return err }
//	if !entry.Own {
//	    callCtx.DecrementBorrows()  // Caller must do this!
//	}
func (t *ResourceTable) Remove(h Handle) (*HandleEntry, error) {
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -run "TestResourceTable_RemoveBorrow" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/resource_table.go internal/component/resource_table_test.go
git commit -m "docs(resource): document borrow count decrement on borrow handle drop"
```

---

## Phase 3 Completion: Regression Check

**CRITICAL: Run regression tests before proceeding to Phase 4**

```bash
CGO_ENABLED=0 go test -v ./internal/component/wasip2test/... -run "TestCalculatorPlugins/add"
CGO_ENABLED=0 go test -v ./internal/component/wasip2test/... -run "TestCalculatorPlugins/subtract"
```

**Expected:** Both tests PASS

Also run full resource tests:
```bash
CGO_ENABLED=0 go test -v ./internal/component/... -run "TestResource|TestCallContext|TestBorrowScope" -v
CGO_ENABLED=0 go test -v ./internal/component/abi/... -run "TestLower" -v
CGO_ENABLED=0 go test -v ./internal/component/conformance/... -run "Resource"
```

**Expected:** All tests PASS

**If all tests pass, commit the phase completion:**
```bash
git commit --allow-empty -m "milestone: complete phase 3 - borrow scope integration"
```

---

## Summary of Changes in Phase 3

| File | Change |
|------|--------|
| `internal/component/call_context.go` | ADD: lenders field and tracking methods |
| `internal/component/call_context.go` | ADD: ExitCall with undo_lend |
| `internal/component/call_context_test.go` | ADD: Lenders and ExitCall tests |
| `internal/component/resource_type_id.go` | ADD: ResourceTypeInfo struct |
| `internal/component/resource_type_id_test.go` | ADD: ResourceTypeInfo tests |
| `internal/component/abi/resource_lower.go` | NEW: LowerBorrow and LowerOwn |
| `internal/component/abi/resource_lower_test.go` | NEW: Lower function tests |
| `internal/component/resource_table.go` | ADD: Documentation for borrow drop |

---

## Next Phase

Proceed to: [Phase 4: Destructor Support](./04-phase4-destructor-support.md)
