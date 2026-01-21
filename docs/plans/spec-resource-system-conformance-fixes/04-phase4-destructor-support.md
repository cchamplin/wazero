# Phase 4: Destructor Support

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement proper destructor invocation on owned handle drop, including the distinction between same-instance and cross-instance destructor calls.

**Architecture:** Expand ResourceType to include destructor function references and instance information. Implement destructor invocation in the resource.drop path with proper routing for same-instance vs cross-instance cases.

**Tech Stack:** Go

---

## Prerequisites

- Complete Phases 1-3
- Read gap analysis: `docs/plans/resource-system-gap-analysis.md` (Section 2 and 3.2)
- Understand spec: `debug-vendored/component-model/design/mvp/CanonicalABI.md` lines 537-549, 3634-3646

## Reference: Spec Destructor Handling

From ResourceType (CanonicalABI.md:537-549):
```python
class ResourceType(Type):
  impl: ComponentInstance     # Component that defines this type
  dtor: Optional[Callable]    # Destructor function
  dtor_async: bool            # Whether destructor is async
  dtor_callback: Optional[Callable]
```

From resource.drop (CanonicalABI.md:3634-3646):
```python
if h.own:
  assert(h.borrow_scope is None)
  if inst is rt.impl:  # Same instance
    if rt.dtor:
      rt.dtor(h.rep)  # Direct call
  else:  # Cross-instance
    if rt.dtor:
      # Call via canon_lift/canon_lower
      callee = partial(canon_lift, callee_opts, rt.impl, ft, rt.dtor)
      [] = canon_lower(caller_opts, ft, callee, thread, [h.rep])
    else:
      trap_if(call_might_be_recursive(thread.task, rt.impl))
```

---

## Task 4.1: Expand ResourceType Definition

**Files:**
- Modify: `internal/component/types/resource.go`
- Test: `internal/component/types/resource_test.go`

**Step 1: Write the failing test**

Create/modify `internal/component/types/resource_test.go`:
```go
package types

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestResourceType_HasDestructor(t *testing.T) {
	// Resource with destructor
	rt := ResourceType{
		Destructor: ptrTo(uint32(5)),
	}
	require.True(t, rt.HasDestructor())

	// Resource without destructor
	rtNoDtor := ResourceType{}
	require.False(t, rtNoDtor.HasDestructor())
}

func TestResourceType_InstanceID(t *testing.T) {
	rt := ResourceType{
		InstanceID: 42,
	}
	require.Equal(t, uint32(42), rt.InstanceID)
}

func TestResourceType_AsyncDestructor(t *testing.T) {
	rt := ResourceType{
		Destructor:   ptrTo(uint32(5)),
		DtorAsync:    true,
		DtorCallback: ptrTo(uint32(6)),
	}
	require.True(t, rt.DtorAsync)
	require.NotNil(t, rt.DtorCallback)
}

func ptrTo(v uint32) *uint32 {
	return &v
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/types/... -run "TestResourceType" -v`
Expected: FAIL with "HasDestructor undefined" or "InstanceID undefined"

**Step 3: Write minimal implementation**

Modify `internal/component/types/resource.go`:
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

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

// ResourceType represents a resource type definition.
// Resources have an optional destructor that is called when the resource is dropped.
//
// From spec (CanonicalABI.md:537-549):
//
//	class ResourceType(Type):
//	  impl: ComponentInstance
//	  dtor: Optional[Callable]
//	  dtor_async: bool
//	  dtor_callback: Optional[Callable]
type ResourceType struct {
	// InstanceID identifies the component instance that defines this resource type.
	// This corresponds to the 'impl' field in the spec.
	InstanceID uint32

	// Destructor is the index of the destructor function (nil if no destructor).
	// This is the core function index in the defining instance.
	Destructor *uint32

	// DtorAsync indicates if the destructor is an async function.
	DtorAsync bool

	// DtorCallback is the callback function index for async destructors.
	DtorCallback *uint32
}

// HasDestructor returns true if this resource type has a destructor.
func (rt *ResourceType) HasDestructor() bool {
	return rt.Destructor != nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/types/... -run "TestResourceType" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/types/resource.go internal/component/types/resource_test.go
git commit -m "feat(resource): expand ResourceType with destructor and instance fields"
```

---

## Task 4.2: Create DestructorFunc Type and Registry

**Files:**
- Create: `internal/component/destructor.go`
- Test: `internal/component/destructor_test.go`

**Step 1: Write the failing test**

Create `internal/component/destructor_test.go`:
```go
package component

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestDestructorRegistry_Register(t *testing.T) {
	registry := NewDestructorRegistry()

	var calledWith uint32
	dtor := func(rep uint32) {
		calledWith = rep
	}

	// Register destructor for type 5
	registry.Register(NewResourceTypeID(5), dtor)

	// Get and call it
	got := registry.Get(NewResourceTypeID(5))
	require.NotNil(t, got)

	got(42)
	require.Equal(t, uint32(42), calledWith)
}

func TestDestructorRegistry_Get_NotFound(t *testing.T) {
	registry := NewDestructorRegistry()

	got := registry.Get(NewResourceTypeID(99))
	require.Nil(t, got)
}

func TestDestructorRegistry_Unregister(t *testing.T) {
	registry := NewDestructorRegistry()

	registry.Register(NewResourceTypeID(5), func(uint32) {})
	require.NotNil(t, registry.Get(NewResourceTypeID(5)))

	registry.Unregister(NewResourceTypeID(5))
	require.Nil(t, registry.Get(NewResourceTypeID(5)))
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -run "TestDestructorRegistry" -v`
Expected: FAIL with "NewDestructorRegistry undefined"

**Step 3: Write minimal implementation**

Create `internal/component/destructor.go`:
```go
// internal/component/destructor.go

package component

// DestructorFunc is a function that destroys a resource given its representation.
// This is called when an owned handle is dropped.
type DestructorFunc func(rep uint32)

// DestructorRegistry maps resource types to their destructor functions.
// Each component instance has its own registry.
type DestructorRegistry struct {
	destructors map[ResourceTypeID]DestructorFunc
}

// NewDestructorRegistry creates a new destructor registry.
func NewDestructorRegistry() *DestructorRegistry {
	return &DestructorRegistry{
		destructors: make(map[ResourceTypeID]DestructorFunc),
	}
}

// Register associates a destructor function with a resource type.
func (r *DestructorRegistry) Register(rtID ResourceTypeID, dtor DestructorFunc) {
	r.destructors[rtID] = dtor
}

// Unregister removes the destructor for a resource type.
func (r *DestructorRegistry) Unregister(rtID ResourceTypeID) {
	delete(r.destructors, rtID)
}

// Get returns the destructor for a resource type, or nil if none registered.
func (r *DestructorRegistry) Get(rtID ResourceTypeID) DestructorFunc {
	return r.destructors[rtID]
}

// Has returns true if a destructor is registered for the resource type.
func (r *DestructorRegistry) Has(rtID ResourceTypeID) bool {
	_, ok := r.destructors[rtID]
	return ok
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -run "TestDestructorRegistry" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/destructor.go internal/component/destructor_test.go
git commit -m "feat(resource): add DestructorRegistry for destructor function management"
```

---

## Task 4.3: Implement DropOwned with Destructor Invocation

**Files:**
- Modify: `internal/component/resource_table.go`
- Test: `internal/component/resource_table_test.go`

**Step 1: Write the failing test**

Add to `internal/component/resource_table_test.go`:
```go
func TestResourceTable_DropOwned_CallsDestructor(t *testing.T) {
	table := NewResourceTable()
	registry := NewDestructorRegistry()

	var destructorCalledWith uint32
	registry.Register(NewResourceTypeID(1), func(rep uint32) {
		destructorCalledWith = rep
	})

	// Create owned handle
	h := table.NewWithType(uint32(42), true, NewResourceTypeID(1))

	// Drop with destructor invocation
	err := table.DropOwned(h, NewResourceTypeID(1), registry, 100, 100, nil)
	require.NoError(t, err)

	// Destructor should have been called with the rep
	require.Equal(t, uint32(42), destructorCalledWith)
}

func TestResourceTable_DropOwned_NoDestructor(t *testing.T) {
	table := NewResourceTable()
	registry := NewDestructorRegistry()
	// No destructor registered

	h := table.NewWithType(uint32(42), true, NewResourceTypeID(1))

	// Should still succeed without destructor
	err := table.DropOwned(h, NewResourceTypeID(1), registry, 100, 100, nil)
	require.NoError(t, err)

	// Handle should be removed
	_, err = table.Get(h)
	require.Error(t, err)
}

func TestResourceTable_DropOwned_TypeMismatch(t *testing.T) {
	table := NewResourceTable()
	registry := NewDestructorRegistry()

	// Create handle of type 1
	h := table.NewWithType(uint32(42), true, NewResourceTypeID(1))

	// Try to drop as type 2
	err := table.DropOwned(h, NewResourceTypeID(2), registry, 100, 100, nil)
	require.ErrorIs(t, err, ErrResourceTypeMismatch)

	// Handle should NOT be removed (error occurred before removal)
	entry, err := table.Get(h)
	require.NoError(t, err)
	require.Equal(t, uint32(42), entry.Rep.(uint32))
}

func TestResourceTable_DropOwned_CrossInstance(t *testing.T) {
	table := NewResourceTable()
	registry := NewDestructorRegistry()

	var crossInstanceCallCount int
	crossInstanceDtor := func(rep uint32, definingInstance uint32) {
		crossInstanceCallCount++
	}

	registry.Register(NewResourceTypeID(1), func(rep uint32) {
		// This is the same-instance destructor, should not be called
		t.Fatal("same-instance destructor should not be called for cross-instance drop")
	})

	h := table.NewWithType(uint32(42), true, NewResourceTypeID(1))

	// Drop from instance 200, but type defined in instance 100
	err := table.DropOwned(h, NewResourceTypeID(1), registry, 200, 100, crossInstanceDtor)
	require.NoError(t, err)

	require.Equal(t, 1, crossInstanceCallCount)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -run "TestResourceTable_DropOwned" -v`
Expected: FAIL with "DropOwned undefined"

**Step 3: Write minimal implementation**

Add to `internal/component/resource_table.go`:
```go
// CrossInstanceDestructor is called when dropping a resource from an instance
// different from where the resource type was defined.
// The caller is responsible for routing this to the proper canonical ABI path.
type CrossInstanceDestructor func(rep uint32, definingInstanceID uint32)

// DropOwned drops an owned handle, invoking the destructor if appropriate.
// This implements the spec's resource.drop for owned handles.
//
// Parameters:
//   - h: the handle to drop
//   - expectedRT: the expected resource type (for validation)
//   - dtorRegistry: registry to look up same-instance destructors
//   - currentInstanceID: the instance performing the drop
//   - definingInstanceID: the instance that defined the resource type
//   - crossInstanceDtor: callback for cross-instance destructor invocation
//
// From spec (CanonicalABI.md:3634-3646):
//
//	if h.own:
//	  if inst is rt.impl:
//	    if rt.dtor: rt.dtor(h.rep)
//	  else:
//	    if rt.dtor: [route through canon_lift/canon_lower]
func (t *ResourceTable) DropOwned(
	h Handle,
	expectedRT ResourceTypeID,
	dtorRegistry *DestructorRegistry,
	currentInstanceID uint32,
	definingInstanceID uint32,
	crossInstanceDtor CrossInstanceDestructor,
) error {
	// Validate type first
	if expectedRT.IsValid() {
		if err := t.ValidateType(h, expectedRT); err != nil {
			return err
		}
	}

	// Remove the handle
	entry, err := t.Remove(h)
	if err != nil {
		return err
	}

	// Only call destructor for owned handles
	if !entry.Own {
		return nil
	}

	// Get the rep value
	var rep uint32
	switch v := entry.Rep.(type) {
	case uint32:
		rep = v
	case int:
		rep = uint32(v)
	default:
		// No valid rep, skip destructor
		return nil
	}

	// Same-instance vs cross-instance destructor call
	if currentInstanceID == definingInstanceID {
		// Same instance: call destructor directly
		if dtor := dtorRegistry.Get(expectedRT); dtor != nil {
			dtor(rep)
		}
	} else {
		// Cross-instance: use the cross-instance callback
		if crossInstanceDtor != nil {
			crossInstanceDtor(rep, definingInstanceID)
		}
	}

	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -run "TestResourceTable_DropOwned" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/resource_table.go internal/component/resource_table_test.go
git commit -m "feat(resource): implement DropOwned with destructor invocation"
```

---

## Task 4.4: Update CreateResourceDropFuncWithTrap to Use DropOwned

**Files:**
- Modify: `internal/component/resource_table.go`
- Test: `internal/component/resource_table_test.go`

**Step 1: Write the failing test**

Add to `internal/component/resource_table_test.go`:
```go
func TestCreateResourceDropFuncWithContext_CallsDestructor(t *testing.T) {
	table := NewResourceTable()
	registry := NewDestructorRegistry()
	callCtx := NewCallContext()

	var destructorCalledWith uint32
	registry.Register(NewResourceTypeID(1), func(rep uint32) {
		destructorCalledWith = rep
	})

	// Create the drop function
	dropFunc := table.CreateResourceDropFuncWithContext(1, registry, 100, 100, callCtx, nil, func(err error) {
		t.Fatalf("unexpected trap: %v", err)
	})

	// Create and drop an owned handle
	h := table.NewWithType(uint32(42), true, NewResourceTypeID(1))
	dropFunc(uint32(h))

	require.Equal(t, uint32(42), destructorCalledWith)
}

func TestCreateResourceDropFuncWithContext_DecrementsBorrowCount(t *testing.T) {
	table := NewResourceTable()
	registry := NewDestructorRegistry()
	callCtx := NewCallContext()

	dropFunc := table.CreateResourceDropFuncWithContext(1, registry, 100, 100, callCtx, nil, func(err error) {
		t.Fatalf("unexpected trap: %v", err)
	})

	// Create a borrow handle and increment borrow count
	h := table.NewWithType(uint32(42), false, NewResourceTypeID(1)) // own=false
	callCtx.IncrementBorrows()

	require.Equal(t, 1, callCtx.NumBorrows())

	// Drop the borrow
	dropFunc(uint32(h))

	// Borrow count should be decremented
	require.Equal(t, 0, callCtx.NumBorrows())
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -run "TestCreateResourceDropFuncWithContext" -v`
Expected: FAIL with "CreateResourceDropFuncWithContext undefined"

**Step 3: Write minimal implementation**

Add to `internal/component/resource_table.go`:
```go
// CreateResourceDropFuncWithContext creates a fully-featured resource.drop function
// that handles destructor invocation, borrow count tracking, and type validation.
func (t *ResourceTable) CreateResourceDropFuncWithContext(
	resourceTypeIdx uint32,
	dtorRegistry *DestructorRegistry,
	currentInstanceID uint32,
	definingInstanceID uint32,
	callCtx *CallContext,
	crossInstanceDtor CrossInstanceDestructor,
	trap TrapHandler,
) func(handle uint32) {
	expectedRT := NewResourceTypeID(resourceTypeIdx)

	return func(handle uint32) {
		h := Handle(handle)

		// Get entry first to check if it's a borrow
		entry, err := t.GetWithType(h, expectedRT)
		if err != nil {
			trap(err)
			return
		}

		if entry.Own {
			// Owned handle: use DropOwned for destructor handling
			err = t.DropOwned(h, expectedRT, dtorRegistry, currentInstanceID, definingInstanceID, crossInstanceDtor)
			if err != nil {
				trap(err)
			}
		} else {
			// Borrowed handle: just remove and decrement borrow count
			_, err = t.Remove(h)
			if err != nil {
				trap(err)
				return
			}
			// Spec: h.borrow_scope.num_borrows -= 1
			if callCtx != nil {
				callCtx.DecrementBorrows()
			}
		}
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -run "TestCreateResourceDropFuncWithContext" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/resource_table.go internal/component/resource_table_test.go
git commit -m "feat(resource): add CreateResourceDropFuncWithContext with full context"
```

---

## Task 4.5: Add Integration Test for Cross-Instance Destructor

**Files:**
- Test: `internal/component/conformance/destructor_test.go`

**Step 1: Write the integration test**

Create `internal/component/conformance/destructor_test.go`:
```go
package conformance

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestDestructor_SameInstance verifies destructor is called directly
// when dropping from the same instance that defined the resource.
func TestDestructor_SameInstance(t *testing.T) {
	table := component.NewResourceTable()
	registry := component.NewDestructorRegistry()

	var calls []uint32
	registry.Register(component.NewResourceTypeID(1), func(rep uint32) {
		calls = append(calls, rep)
	})

	// Create resources
	h1 := table.NewWithType(uint32(100), true, component.NewResourceTypeID(1))
	h2 := table.NewWithType(uint32(200), true, component.NewResourceTypeID(1))

	// Drop from same instance (100 == 100)
	require.NoError(t, table.DropOwned(h1, component.NewResourceTypeID(1), registry, 100, 100, nil))
	require.NoError(t, table.DropOwned(h2, component.NewResourceTypeID(1), registry, 100, 100, nil))

	require.Equal(t, []uint32{100, 200}, calls)
}

// TestDestructor_CrossInstance verifies cross-instance callback is used
// when dropping from a different instance.
func TestDestructor_CrossInstance(t *testing.T) {
	table := component.NewResourceTable()
	registry := component.NewDestructorRegistry()

	// Register same-instance destructor (should NOT be called)
	registry.Register(component.NewResourceTypeID(1), func(rep uint32) {
		t.Fatal("same-instance destructor should not be called")
	})

	var crossCalls []struct{ rep, inst uint32 }
	crossDtor := func(rep uint32, definingInst uint32) {
		crossCalls = append(crossCalls, struct{ rep, inst uint32 }{rep, definingInst})
	}

	h := table.NewWithType(uint32(42), true, component.NewResourceTypeID(1))

	// Drop from instance 200, type defined in instance 100
	require.NoError(t, table.DropOwned(h, component.NewResourceTypeID(1), registry, 200, 100, crossDtor))

	require.Len(t, crossCalls, 1)
	require.Equal(t, uint32(42), crossCalls[0].rep)
	require.Equal(t, uint32(100), crossCalls[0].inst)
}

// TestDestructor_NoDestructor verifies drop works when no destructor registered.
func TestDestructor_NoDestructor(t *testing.T) {
	table := component.NewResourceTable()
	registry := component.NewDestructorRegistry()
	// No destructor registered

	h := table.NewWithType(uint32(42), true, component.NewResourceTypeID(1))

	// Should succeed
	require.NoError(t, table.DropOwned(h, component.NewResourceTypeID(1), registry, 100, 100, nil))

	// Handle should be gone
	_, err := table.Get(h)
	require.Error(t, err)
}

// TestDestructor_BorrowDoesNotCallDestructor verifies borrows don't trigger destructor.
func TestDestructor_BorrowDoesNotCallDestructor(t *testing.T) {
	table := component.NewResourceTable()
	registry := component.NewDestructorRegistry()

	registry.Register(component.NewResourceTypeID(1), func(rep uint32) {
		t.Fatal("destructor should not be called for borrow")
	})

	// Create borrow (own=false)
	h := table.NewWithType(uint32(42), false, component.NewResourceTypeID(1))

	require.NoError(t, table.DropOwned(h, component.NewResourceTypeID(1), registry, 100, 100, nil))
}
```

**Step 2: Run test to verify it passes**

Run: `go test ./internal/component/conformance/... -run "TestDestructor" -v`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/component/conformance/destructor_test.go
git commit -m "test(resource): add destructor integration tests"
```

---

## Phase 4 Completion: Regression Check

**CRITICAL: Run regression tests before proceeding to Phase 5**

```bash
CGO_ENABLED=0 go test -v ./internal/component/wasip2test/... -run "TestCalculatorPlugins/add"
CGO_ENABLED=0 go test -v ./internal/component/wasip2test/... -run "TestCalculatorPlugins/subtract"
```

**Expected:** Both tests PASS

Also run full tests:
```bash
CGO_ENABLED=0 go test -v ./internal/component/... -run "Test" -v
CGO_ENABLED=0 go test -v ./internal/component/types/... -run "Test" -v
CGO_ENABLED=0 go test -v ./internal/component/conformance/... -run "Test" -v
```

**Expected:** All tests PASS

**If all tests pass, commit the phase completion:**
```bash
git commit --allow-empty -m "milestone: complete phase 4 - destructor support"
```

---

## Summary of Changes in Phase 4

| File | Change |
|------|--------|
| `internal/component/types/resource.go` | ADD: InstanceID, DtorAsync, DtorCallback, HasDestructor |
| `internal/component/types/resource_test.go` | ADD: ResourceType tests |
| `internal/component/destructor.go` | NEW: DestructorRegistry |
| `internal/component/destructor_test.go` | NEW: DestructorRegistry tests |
| `internal/component/resource_table.go` | ADD: DropOwned method |
| `internal/component/resource_table.go` | ADD: CreateResourceDropFuncWithContext |
| `internal/component/conformance/destructor_test.go` | NEW: Destructor integration tests |

---

## Next Phase

Proceed to: [Phase 5: Advanced Features](./05-phase5-advanced-features.md)
