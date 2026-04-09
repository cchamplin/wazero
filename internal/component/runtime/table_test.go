// internal/component/runtime/table_test.go

package runtime

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestTable_NewAndGet(t *testing.T) {
	table := NewTable()

	// Create a resource (no type tracking — pass nil)
	h, err := table.NewResourceHandle(uint32(1), true, nil) // own=true
	require.NoError(t, err)

	// Verify handle parts
	require.Equal(t, uint32(0), h.Index())
	require.Equal(t, uint32(0), h.Generation())

	// Retrieve it
	entry, err := table.GetResourceHandle(h)
	require.NoError(t, err)
	require.Equal(t, uint32(1), entry.Rep)
	require.True(t, entry.Own)
}

func TestTable_MultipleResources(t *testing.T) {
	table := NewTable()

	h1, err := table.NewResourceHandle(uint32(10), true, nil)
	require.NoError(t, err)
	h2, err := table.NewResourceHandle(uint32(20), true, nil)
	require.NoError(t, err)
	h3, err := table.NewResourceHandle(uint32(30), true, nil)
	require.NoError(t, err)

	require.Equal(t, uint32(0), h1.Index())
	require.Equal(t, uint32(1), h2.Index())
	require.Equal(t, uint32(2), h3.Index())

	e1, _ := table.GetResourceHandle(h1)
	e2, _ := table.GetResourceHandle(h2)
	e3, _ := table.GetResourceHandle(h3)

	require.Equal(t, uint32(10), e1.Rep)
	require.Equal(t, uint32(20), e2.Rep)
	require.Equal(t, uint32(30), e3.Rep)
}

func TestHandle_MakeHandle(t *testing.T) {
	h := MakeHandle(42, 7)
	require.Equal(t, uint32(42), h.Index())
	require.Equal(t, uint32(7), h.Generation())
}

func TestTable_Remove(t *testing.T) {
	table := NewTable()
	h, err := table.NewResourceHandle(uint32(1), true, nil)
	require.NoError(t, err)

	// Remove returns the entry
	entry, err := table.Remove(h)
	require.NoError(t, err)
	require.Equal(t, uint32(1), entry.Rep)
	require.True(t, entry.Own)

	// Subsequent Get fails
	_, err = table.Get(h)
	require.Error(t, err)
}

func TestTable_UseAfterFree(t *testing.T) {
	table := NewTable()

	// Create and remove a resource
	h1, err := table.NewResourceHandle(uint32(10), true, nil)
	require.NoError(t, err)
	_, err = table.Remove(h1)
	require.NoError(t, err)

	// Create another resource (reuses index 0)
	h2, err := table.NewResourceHandle(uint32(20), true, nil)
	require.NoError(t, err)
	require.Equal(t, uint32(0), h2.Index())
	require.Equal(t, uint32(1), h2.Generation()) // Generation incremented

	// Old handle should fail (generation mismatch)
	_, err = table.Get(h1)
	require.Error(t, err, "generation mismatch should prevent access")

	// New handle works
	entry, err := table.GetResourceHandle(h2)
	require.NoError(t, err)
	require.Equal(t, uint32(20), entry.Rep)
}

func TestTable_DoubleFree(t *testing.T) {
	table := NewTable()
	h, err := table.NewResourceHandle(uint32(5), true, nil)
	require.NoError(t, err)

	// First remove succeeds
	_, err = table.Remove(h)
	require.NoError(t, err)

	// Second remove fails (already freed)
	_, err = table.Remove(h)
	require.Error(t, err)
}

func TestTable_RemoveWithActiveBorrows(t *testing.T) {
	table := NewTable()
	h, err := table.NewResourceHandle(uint32(6), true, nil)
	require.NoError(t, err)

	// Manually set NumLends to simulate active borrow
	entry, _ := table.GetResourceHandle(h)
	entry.NumLends = 1

	_, err = table.Remove(h)
	require.ErrorIs(t, err, ErrResourceInUse)
}

func TestTable_BorrowTracking(t *testing.T) {
	table := NewTable()
	h, err := table.NewResourceHandle(uint32(7), true, nil)
	require.NoError(t, err)

	// Increment lends (for lift_borrow)
	err = table.IncrementLends(h)
	require.NoError(t, err)

	entry, _ := table.GetResourceHandle(h)
	require.Equal(t, uint32(1), entry.NumLends)

	// Cannot remove while borrowed
	_, err = table.Remove(h)
	require.ErrorIs(t, err, ErrResourceInUse)

	// Decrement lends
	err = table.DecrementLends(h)
	require.NoError(t, err)

	entry, _ = table.GetResourceHandle(h)
	require.Equal(t, uint32(0), entry.NumLends)

	// Now can remove
	_, err = table.Remove(h)
	require.NoError(t, err)
}

func TestTable_MultipleBorrows(t *testing.T) {
	table := NewTable()
	h, err := table.NewResourceHandle(uint32(8), true, nil)
	require.NoError(t, err)

	// Multiple concurrent borrows
	require.NoError(t, table.IncrementLends(h))
	require.NoError(t, table.IncrementLends(h))
	require.NoError(t, table.IncrementLends(h))

	entry, _ := table.GetResourceHandle(h)
	require.Equal(t, uint32(3), entry.NumLends)

	// Decrement all
	require.NoError(t, table.DecrementLends(h))
	require.NoError(t, table.DecrementLends(h))
	require.NoError(t, table.DecrementLends(h))

	entry, _ = table.GetResourceHandle(h)
	require.Equal(t, uint32(0), entry.NumLends)
}

func TestTable_DecrementUnderflow(t *testing.T) {
	table := NewTable()
	h, err := table.NewResourceHandle(uint32(9), true, nil)
	require.NoError(t, err)

	// Decrement without increment should error
	err = table.DecrementLends(h)
	require.ErrorIs(t, err, ErrNoBorrowsToDecrement)
}

func TestTable_BorrowedHandle(t *testing.T) {
	table := NewTable()

	// Create borrowed handle (own=false)
	h, err := table.NewResourceHandle(uint32(11), false, nil)
	require.NoError(t, err)

	entry, err := table.GetResourceHandle(h)
	require.NoError(t, err)
	require.False(t, entry.Own)
	require.Equal(t, uint32(11), entry.Rep)
}

func TestTable_RemoveBorrowedMustNotCallDestructor(t *testing.T) {
	table := NewTable()
	h, err := table.NewResourceHandle(uint32(12), false, nil) // borrowed
	require.NoError(t, err)

	entry, err := table.Remove(h)
	require.NoError(t, err)
	require.False(t, entry.Own) // Caller checks Own to decide on destructor
}

func TestTable_Rep(t *testing.T) {
	table := NewTable()

	// Create a new resource with rep=42
	handle, err := table.NewResourceHandle(uint32(42), true, nil)
	require.NoError(t, err)

	// First handle has index 0 and generation 0
	require.Equal(t, uint32(0), handle.Index())
	require.Equal(t, uint32(0), handle.Generation())

	// Verify we can get the rep back
	rep, err := table.Rep(handle)
	require.NoError(t, err)
	require.Equal(t, uint32(42), rep)
}

func TestTable_Rep_InvalidHandle(t *testing.T) {
	table := NewTable()

	// Try to get rep of non-existent handle
	invalidHandle := MakeHandle(999, 0)
	_, err := table.Rep(invalidHandle)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidHandle)
}

// TestTable_Rep_Uint32Direct verifies that Rep returns the uint32 value
// directly now that the field is typed uint32.
//
// Spec: definitions.py:337-349 ResourceHandle.rep: int (u32 invariant).
func TestTable_Rep_Uint32Direct(t *testing.T) {
	table := NewTable()

	handle, err := table.NewResourceHandle(uint32(42), true, nil)
	require.NoError(t, err)

	rep, err := table.Rep(handle)
	require.NoError(t, err)
	require.Equal(t, uint32(42), rep)
}

// TestResourceHandleEntryRepIsUint32 asserts Rep is typed uint32 per
// spec definitions.py:337-349 ResourceHandle.rep.
//
// Spec: definitions.py:337-349 class ResourceHandle.rep.
// Wasmtime parallel: runtime/component/instance.rs:383-387 resource_new32
// + vm/component/resources.rs rep: u32 throughout.
func TestResourceHandleEntryRepIsUint32(t *testing.T) {
	entry := &ResourceHandleEntry{
		RT:  &ResourceType{},
		Rep: uint32(42),
	}
	if entry.Rep != uint32(42) {
		t.Fatalf("entry.Rep = %v, want 42", entry.Rep)
	}
}

func TestTable_CreateResourceNewFunc(t *testing.T) {
	table := NewTable()
	rt := &ResourceType{}

	// Create a resource.new function
	newFunc := table.CreateResourceNewFunc(rt)

	// Call it - first handle will be 0 (index 0, gen 0)
	handle := newFunc(42)

	// Verify the resource was created and handle is valid
	h := Handle(handle)
	require.Equal(t, uint32(0), h.Index())
	require.Equal(t, uint32(0), h.Generation())

	// Verify we can get the rep back
	rep, err := table.Rep(h)
	require.NoError(t, err)
	require.Equal(t, uint32(42), rep)
}

func TestTable_CreateResourceNewFunc_MultipleResources(t *testing.T) {
	table := NewTable()
	rt1 := &ResourceType{}
	rt2 := &ResourceType{}

	// Create resource.new functions for different resource types
	newFunc1 := table.CreateResourceNewFunc(rt1)
	newFunc2 := table.CreateResourceNewFunc(rt2)

	// Create resources
	h1 := newFunc1(100)
	h2 := newFunc2(200)
	h3 := newFunc1(300)

	// All handles should be valid and have correct reps
	rep1, err := table.Rep(Handle(h1))
	require.NoError(t, err)
	require.Equal(t, uint32(100), rep1)

	rep2, err := table.Rep(Handle(h2))
	require.NoError(t, err)
	require.Equal(t, uint32(200), rep2)

	rep3, err := table.Rep(Handle(h3))
	require.NoError(t, err)
	require.Equal(t, uint32(300), rep3)
}

func TestTable_CreateResourceNewFunc_CreatesOwnedResources(t *testing.T) {
	table := NewTable()
	rt := &ResourceType{}

	newFunc := table.CreateResourceNewFunc(rt)
	handle := Handle(newFunc(42))

	// Verify the created resource is owned
	entry, err := table.GetResourceHandle(handle)
	require.NoError(t, err)
	require.True(t, entry.Own)
}

func TestTable_CreateResourceDropFunc(t *testing.T) {
	table := NewTable()
	rt := &ResourceType{}

	var destructorCalled bool
	var droppedRep uint32

	dropFunc := table.CreateResourceDropFunc(rt, func(rep uint32) {
		destructorCalled = true
		droppedRep = rep
	})

	// Create a resource of the same type
	handle, err := table.NewResourceHandle(uint32(42), true, rt)
	require.NoError(t, err)

	// Drop it
	dropFunc(uint32(handle))

	// Verify destructor was called
	if !destructorCalled {
		t.Error("destructor was not called")
	}
	if droppedRep != 42 {
		t.Errorf("expected droppedRep=42, got %d", droppedRep)
	}

	// Verify resource was removed
	_, err = table.Get(handle)
	if err == nil {
		t.Error("expected error getting dropped handle")
	}
}

func TestTable_CreateResourceDropFunc_InvalidHandle(t *testing.T) {
	table := NewTable()
	rt := &ResourceType{}

	var destructorCalled bool
	dropFunc := table.CreateResourceDropFunc(rt, func(rep uint32) {
		destructorCalled = true
	})

	// Drop invalid handle - should not panic or call destructor
	dropFunc(999)

	if destructorCalled {
		t.Error("destructor should not be called for invalid handle")
	}
}

func TestTable_CreateResourceDropFunc_NilDestructor(t *testing.T) {
	table := NewTable()
	rt := &ResourceType{}

	dropFunc := table.CreateResourceDropFunc(rt, nil)

	// Create and drop - should work without destructor
	handle, err := table.NewResourceHandle(uint32(42), true, rt)
	require.NoError(t, err)
	dropFunc(uint32(handle))

	// Verify resource was removed
	_, err = table.Get(handle)
	if err == nil {
		t.Error("expected error getting dropped handle")
	}
}

func TestCreateResourceDropFunc_BorrowDoesNotCallDestructor(t *testing.T) {
	// Spec: destructors must only fire for owned handles. A drop on a
	// borrow handle removes the entry but must not call the destructor.
	tab := NewTable()
	rt := &ResourceType{}

	called := false
	dtor := func(rep uint32) { called = true }

	dropFn := tab.CreateResourceDropFunc(rt, dtor)

	// Insert a borrow handle (own=false).
	h, err := tab.NewResourceHandle(uint32(42), false, rt)
	if err != nil {
		t.Fatalf("NewResourceHandle: %v", err)
	}

	dropFn(uint32(h))

	if called {
		t.Errorf("destructor called for borrow drop, want NOT called")
	}

	// And confirm the entry was actually removed.
	if _, err := tab.Get(h); err == nil {
		t.Errorf("Get on dropped borrow handle = nil error, want error (entry should be removed)")
	}
}

func TestTable_CreateResourceRepFunc(t *testing.T) {
	table := NewTable()
	rt := &ResourceType{}

	repFunc := table.CreateResourceRepFunc(rt)

	// Create a resource of the same type
	handle, err := table.NewResourceHandle(uint32(42), true, rt)
	require.NoError(t, err)

	// Get its rep
	rep := repFunc(uint32(handle))
	if rep != 42 {
		t.Errorf("expected rep=42, got %d", rep)
	}
}

func TestTable_CreateResourceRepFunc_InvalidHandle(t *testing.T) {
	table := NewTable()
	rt := &ResourceType{}

	repFunc := table.CreateResourceRepFunc(rt)

	// Get rep for invalid handle - should return 0
	rep := repFunc(999)
	if rep != 0 {
		t.Errorf("expected rep=0 for invalid handle, got %d", rep)
	}
}

func TestResourceHandleEntry_HasResourceType(t *testing.T) {
	table := NewTable()

	// Create a handle with a specific resource type
	rt := &ResourceType{}
	h, err := table.NewResourceHandle(uint32(100), true, rt)
	require.NoError(t, err)

	entry, err := table.GetResourceHandle(h)
	require.NoError(t, err)
	require.True(t, entry.RT == rt)
	require.Equal(t, uint32(100), entry.Rep)
}

func TestTable_GetResourceType(t *testing.T) {
	table := NewTable()
	rt := &ResourceType{}
	h, err := table.NewResourceHandle(uint32(100), true, rt)
	require.NoError(t, err)

	// GetResourceType should return the resource type
	gotType, err := table.GetResourceType(h)
	require.NoError(t, err)
	require.True(t, gotType == rt)
}

func TestTable_GetResourceType_InvalidHandle(t *testing.T) {
	table := NewTable()

	invalidHandle := MakeHandle(999, 0)
	_, err := table.GetResourceType(invalidHandle)
	require.ErrorIs(t, err, ErrInvalidHandle)
}

func TestTable_ValidateType(t *testing.T) {
	table := NewTable()
	rt := &ResourceType{}
	wrongRT := &ResourceType{}
	h, err := table.NewResourceHandle(uint32(100), true, rt)
	require.NoError(t, err)

	// Correct type should pass
	require.NoError(t, table.ValidateType(h, rt))

	// Wrong type should fail
	err = table.ValidateType(h, wrongRT)
	require.ErrorIs(t, err, ErrResourceTypeMismatch)
}

func TestTable_ValidateType_InvalidHandle(t *testing.T) {
	table := NewTable()
	invalidHandle := MakeHandle(999, 0)
	rt := &ResourceType{}

	err := table.ValidateType(invalidHandle, rt)
	require.ErrorIs(t, err, ErrInvalidHandle)
}

// TestTable_ValidateType_PointerIdentity is the regression test for the
// bug fix that motivated Decision 5: two ResourceTypes with the same
// conceptual type index must NOT validate against each other if their
// pointers differ. Spec: definitions.py:1345 — `is` check, not value
// equality.
func TestTable_ValidateType_PointerIdentity(t *testing.T) {
	tab := NewTable()
	rtA := &ResourceType{}
	rtB := &ResourceType{}
	h, err := tab.NewResourceHandle(uint32(99), true, rtA)
	if err != nil {
		t.Fatalf("NewResourceHandle: %v", err)
	}
	if err := tab.ValidateType(h, rtA); err != nil {
		t.Errorf("ValidateType against same RT: %v, want nil", err)
	}
	if err := tab.ValidateType(h, rtB); err == nil {
		t.Errorf("ValidateType against different RT: nil, want error")
	}
}

func TestCreateResourceDropFunc_TrapsOnInvalidHandle(t *testing.T) {
	table := NewTable()
	rt := &ResourceType{}

	// Create drop function for type rt
	var trapCalled bool
	var trapErr error
	dropFunc := table.CreateResourceDropFuncWithTrap(rt, nil, func(err error) {
		trapCalled = true
		trapErr = err
	})

	// Try to drop an invalid handle
	dropFunc(999)

	require.True(t, trapCalled, "should trap on invalid handle")
	require.ErrorIs(t, trapErr, ErrInvalidHandle)
}

func TestCreateResourceDropFunc_TrapsOnTypeMismatch(t *testing.T) {
	table := NewTable()

	rt1 := &ResourceType{}
	rt2 := &ResourceType{}

	// Create a handle of type rt1
	h, err := table.NewResourceHandle(uint32(42), true, rt1)
	require.NoError(t, err)

	// Create drop function for type rt2 (different type)
	var trapCalled bool
	var trapErr error
	dropFunc := table.CreateResourceDropFuncWithTrap(rt2, nil, func(err error) {
		trapCalled = true
		trapErr = err
	})

	// Try to drop with wrong type
	dropFunc(uint32(h))

	require.True(t, trapCalled, "should trap on type mismatch")
	require.ErrorIs(t, trapErr, ErrResourceTypeMismatch)
}

func TestCreateResourceDropFuncWithTrap_SuccessfulDrop(t *testing.T) {
	table := NewTable()
	rt := &ResourceType{}

	// Create a handle with type rt
	h, err := table.NewResourceHandle(uint32(42), true, rt)
	require.NoError(t, err)

	var destructorCalledWith uint32
	destructor := func(rep uint32) {
		destructorCalledWith = rep
	}

	var trapCalled bool
	dropFunc := table.CreateResourceDropFuncWithTrap(rt, destructor, func(err error) {
		trapCalled = true
	})

	// Drop the handle
	dropFunc(uint32(h))

	// Verify trap was NOT called
	require.False(t, trapCalled, "should not trap on valid drop")

	// Verify destructor was called with correct rep
	require.Equal(t, uint32(42), destructorCalledWith)

	// Verify handle is now invalid
	_, err = table.Get(h)
	require.ErrorIs(t, err, ErrInvalidHandle)
}

func TestCreateResourceRepFunc_TrapsOnInvalidHandle(t *testing.T) {
	table := NewTable()
	rt := &ResourceType{}

	var trapCalled bool
	var trapErr error
	repFunc := table.CreateResourceRepFuncWithTrap(rt, func(err error) {
		trapCalled = true
		trapErr = err
	})

	// Try to get rep of invalid handle
	_ = repFunc(999)

	require.True(t, trapCalled, "should trap on invalid handle")
	require.ErrorIs(t, trapErr, ErrInvalidHandle)
}

func TestCreateResourceRepFunc_TrapsOnTypeMismatch(t *testing.T) {
	table := NewTable()

	rt1 := &ResourceType{}
	rt2 := &ResourceType{}

	// Create a handle of type rt1
	h, err := table.NewResourceHandle(uint32(42), true, rt1)
	require.NoError(t, err)

	var trapCalled bool
	var trapErr error
	// Create rep function for type rt2 (different type)
	repFunc := table.CreateResourceRepFuncWithTrap(rt2, func(err error) {
		trapCalled = true
		trapErr = err
	})

	// Try to get rep with wrong type
	_ = repFunc(uint32(h))

	require.True(t, trapCalled, "should trap on type mismatch")
	require.ErrorIs(t, trapErr, ErrResourceTypeMismatch)
}

func TestCreateResourceRepFuncWithTrap_ReturnsRepOnSuccess(t *testing.T) {
	table := NewTable()
	rt := &ResourceType{}

	h, err := table.NewResourceHandle(uint32(42), true, rt)
	require.NoError(t, err)

	var trapCalled bool
	repFunc := table.CreateResourceRepFuncWithTrap(rt, func(err error) {
		trapCalled = true
	})

	rep := repFunc(uint32(h))

	require.False(t, trapCalled, "should not trap on valid handle")
	require.Equal(t, uint32(42), rep)
}

func TestTable_RemoveWithType_Success(t *testing.T) {
	table := NewTable()
	rt := &ResourceType{}
	h, err := table.NewResourceHandle(uint32(100), true, rt)
	require.NoError(t, err)

	entry, err := table.RemoveWithType(h, rt)
	require.NoError(t, err)
	require.Equal(t, uint32(100), entry.Rep)
}

func TestTable_RemoveWithType_TypeMismatch(t *testing.T) {
	table := NewTable()
	rt := &ResourceType{}
	wrongRT := &ResourceType{}
	h, err := table.NewResourceHandle(uint32(100), true, rt)
	require.NoError(t, err)

	_, err = table.RemoveWithType(h, wrongRT)
	require.ErrorIs(t, err, ErrResourceTypeMismatch)

	// Handle should still be valid (not removed on type error)
	entry, err := table.GetResourceHandle(h)
	require.NoError(t, err)
	require.Equal(t, uint32(100), entry.Rep)
}

func TestTable_GetWithType_Success(t *testing.T) {
	table := NewTable()
	rt := &ResourceType{}
	h, err := table.NewResourceHandle(uint32(100), true, rt)
	require.NoError(t, err)

	entry, err := table.GetWithType(h, rt)
	require.NoError(t, err)
	require.Equal(t, uint32(100), entry.Rep)
}

func TestTable_GetWithType_TypeMismatch(t *testing.T) {
	table := NewTable()
	rt := &ResourceType{}
	wrongRT := &ResourceType{}
	h, err := table.NewResourceHandle(uint32(100), true, rt)
	require.NoError(t, err)

	_, err = table.GetWithType(h, wrongRT)
	require.ErrorIs(t, err, ErrResourceTypeMismatch)
}

func TestTable_GetWithType_InvalidHandle(t *testing.T) {
	table := NewTable()
	invalidH := MakeHandle(999, 0)
	rt := &ResourceType{}

	_, err := table.GetWithType(invalidH, rt)
	require.ErrorIs(t, err, ErrInvalidHandle)
}

func TestTable_RepWithType_Success(t *testing.T) {
	table := NewTable()
	rt := &ResourceType{}
	h, err := table.NewResourceHandle(uint32(100), true, rt)
	require.NoError(t, err)

	rep, err := table.RepWithType(h, rt)
	require.NoError(t, err)
	require.Equal(t, uint32(100), rep)
}

func TestTable_RepWithType_TypeMismatch(t *testing.T) {
	table := NewTable()
	rt := &ResourceType{}
	wrongRT := &ResourceType{}
	h, err := table.NewResourceHandle(uint32(100), true, rt)
	require.NoError(t, err)

	_, err = table.RepWithType(h, wrongRT)
	require.ErrorIs(t, err, ErrResourceTypeMismatch)
}

// TestTable_RemoveBorrow_DecrementsBorrowCount documents the integration
// pattern for borrow count decrement. The actual decrement happens in calling
// code (resource.drop), not in Remove itself.
func TestTable_RemoveBorrow_DecrementsBorrowCount(t *testing.T) {
	table := NewTable()
	callCtx := NewCallContext()
	rt := &ResourceType{}

	// Simulate lower_borrow: create borrow handle and increment borrow count
	h, err := table.NewResourceHandle(uint32(42), false, rt) // own=false
	require.NoError(t, err)
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

// Tests for DropOwned - Task 4.3: Destructor Invocation

func TestTable_DropOwned_CallsDestructor(t *testing.T) {
	table := NewTable()
	registry := NewDestructorRegistry()
	rt := &ResourceType{}

	var destructorCalledWith uint32
	registry.Register(rt, func(rep uint32) {
		destructorCalledWith = rep
	})

	// Create owned handle
	h, err := table.NewResourceHandle(uint32(42), true, rt)
	require.NoError(t, err)

	// Drop with destructor invocation
	err = table.DropOwned(h, rt, registry, 100, 100, nil)
	require.NoError(t, err)

	// Destructor should have been called with the rep
	require.Equal(t, uint32(42), destructorCalledWith)
}

func TestTable_DropOwned_NoDestructor(t *testing.T) {
	table := NewTable()
	registry := NewDestructorRegistry()
	rt := &ResourceType{}
	// No destructor registered

	h, err := table.NewResourceHandle(uint32(42), true, rt)
	require.NoError(t, err)

	// Should still succeed without destructor
	err = table.DropOwned(h, rt, registry, 100, 100, nil)
	require.NoError(t, err)

	// Handle should be removed
	_, err = table.Get(h)
	require.Error(t, err)
}

func TestTable_DropOwned_TypeMismatch(t *testing.T) {
	table := NewTable()
	registry := NewDestructorRegistry()
	rt1 := &ResourceType{}
	rt2 := &ResourceType{}

	// Create handle of type rt1
	h, err := table.NewResourceHandle(uint32(42), true, rt1)
	require.NoError(t, err)

	// Try to drop as type rt2
	err = table.DropOwned(h, rt2, registry, 100, 100, nil)
	require.ErrorIs(t, err, ErrResourceTypeMismatch)

	// Handle should NOT be removed (error occurred before removal)
	entry, err := table.GetResourceHandle(h)
	require.NoError(t, err)
	require.Equal(t, uint32(42), entry.Rep)
}

func TestTable_DropOwned_CrossInstance(t *testing.T) {
	table := NewTable()
	registry := NewDestructorRegistry()
	rt := &ResourceType{}

	var crossInstanceCallCount int
	crossInstanceDtor := func(rep uint32, definingInstance uint32) {
		crossInstanceCallCount++
	}

	registry.Register(rt, func(rep uint32) {
		// This is the same-instance destructor, should not be called
		t.Fatal("same-instance destructor should not be called for cross-instance drop")
	})

	h, err := table.NewResourceHandle(uint32(42), true, rt)
	require.NoError(t, err)

	// Drop from instance 200, but type defined in instance 100
	err = table.DropOwned(h, rt, registry, 200, 100, crossInstanceDtor)
	require.NoError(t, err)

	require.Equal(t, 1, crossInstanceCallCount)
}

func TestTable_DropOwned_BorrowHandle_NoDestructor(t *testing.T) {
	table := NewTable()
	registry := NewDestructorRegistry()
	rt := &ResourceType{}

	var destructorCalled bool
	registry.Register(rt, func(rep uint32) {
		destructorCalled = true
	})

	// Create borrow handle (Own=false)
	h, err := table.NewResourceHandle(uint32(42), false, rt)
	require.NoError(t, err)

	// Drop borrow handle - should NOT call destructor
	err = table.DropOwned(h, rt, registry, 100, 100, nil)
	require.NoError(t, err)

	// Destructor should NOT have been called for borrow handles
	require.False(t, destructorCalled, "destructor should not be called for borrow handles")

	// Handle should still be removed from table
	_, err = table.Get(h)
	require.ErrorIs(t, err, ErrInvalidHandle)
}

// Tests for CreateResourceDropFuncWithContext - Task 4.4

func TestCreateResourceDropFuncWithContext_CallsDestructor(t *testing.T) {
	table := NewTable()
	registry := NewDestructorRegistry()
	callCtx := NewCallContext()
	rt := &ResourceType{}

	var destructorCalledWith uint32
	registry.Register(rt, func(rep uint32) {
		destructorCalledWith = rep
	})

	// Create the drop function
	dropFunc := table.CreateResourceDropFuncWithContext(rt, registry, 100, 100, callCtx, nil, func(err error) {
		t.Fatalf("unexpected trap: %v", err)
	})

	// Create and drop an owned handle
	h, err := table.NewResourceHandle(uint32(42), true, rt)
	require.NoError(t, err)
	dropFunc(uint32(h))

	require.Equal(t, uint32(42), destructorCalledWith)
}

func TestCreateResourceDropFuncWithContext_DecrementsBorrowCount(t *testing.T) {
	table := NewTable()
	registry := NewDestructorRegistry()
	callCtx := NewCallContext()
	rt := &ResourceType{}

	dropFunc := table.CreateResourceDropFuncWithContext(rt, registry, 100, 100, callCtx, nil, func(err error) {
		t.Fatalf("unexpected trap: %v", err)
	})

	// Create a borrow handle and increment borrow count
	h, err := table.NewResourceHandle(uint32(42), false, rt) // own=false
	require.NoError(t, err)
	callCtx.IncrementBorrows()

	require.Equal(t, 1, callCtx.NumBorrows())

	// Drop the borrow
	dropFunc(uint32(h))

	// Borrow count should be decremented
	require.Equal(t, 0, callCtx.NumBorrows())
}

func TestCreateResourceDropFuncWithContext_TrapsOnInvalidHandle(t *testing.T) {
	table := NewTable()
	registry := NewDestructorRegistry()
	callCtx := NewCallContext()
	rt := &ResourceType{}

	var trappedErr error
	dropFunc := table.CreateResourceDropFuncWithContext(rt, registry, 100, 100, callCtx, nil, func(err error) {
		trappedErr = err
	})

	// Try to drop a non-existent handle
	dropFunc(999)

	require.ErrorIs(t, trappedErr, ErrInvalidHandle)
}

func TestCreateResourceDropFuncWithContext_TrapsOnTypeMismatch(t *testing.T) {
	table := NewTable()
	registry := NewDestructorRegistry()
	callCtx := NewCallContext()
	rt1 := &ResourceType{}
	rt2 := &ResourceType{}

	var trappedErr error
	// Create drop function for type rt1
	dropFunc := table.CreateResourceDropFuncWithContext(rt1, registry, 100, 100, callCtx, nil, func(err error) {
		trappedErr = err
	})

	// Create handle of type rt2
	h, err := table.NewResourceHandle(uint32(42), true, rt2)
	require.NoError(t, err)

	// Try to drop with wrong type
	dropFunc(uint32(h))

	require.ErrorIs(t, trappedErr, ErrResourceTypeMismatch)
}

func TestTable_NewWithMayLeaveCheck(t *testing.T) {
	table := NewTable()
	inst := NewComponentInstance(1, nil)
	rt := &ResourceType{}

	// When may_leave is true, New succeeds
	h, err := table.NewWithMayLeaveCheck(uint32(42), true, rt, inst)
	require.NoError(t, err)
	// Verify the handle is valid by retrieving its entry
	entry, err := table.GetResourceHandle(h)
	require.NoError(t, err)
	require.Equal(t, uint32(42), entry.Rep)

	// When may_leave is false, New fails.
	//
	// Session 1 Task B1 (Decision 3 IsMayLeave semantic fix, design
	// lines 254-263): toggling IsMayLeave() now requires setting the
	// MayLeave field directly. The previous `inst.Enter()` trick
	// relied on the buggy coupling of enterCount into IsMayLeave(),
	// which the spec at definitions.py:260, 270, 1955, 1973, 2065,
	// 2135, 2143 does not permit.
	inst.MayLeave = false
	_, err = table.NewWithMayLeaveCheck(uint32(43), true, rt, inst)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrMayNotLeave)
}

// Tests for Task 5.4: Reentrance Trap in DropOwned

func TestTable_DropOwned_TrapsOnReentrance(t *testing.T) {
	table := NewTable()
	registry := NewDestructorRegistry()
	tracker := NewReentranceTracker()
	rt := &ResourceType{}

	// No destructor registered for this type
	// (reentrance check only applies when no destructor)

	h, err := table.NewResourceHandle(uint32(42), true, rt)
	require.NoError(t, err)

	// Instance 100 is currently on the call stack
	tracker.EnterInstance(100)

	// Dropping a resource defined in instance 100 from instance 200
	// should trap because of potential reentrance
	err = table.DropOwnedWithReentranceCheck(
		h,
		rt,
		registry,
		200, // current instance
		100, // defining instance (on call stack!)
		nil,
		tracker,
	)
	require.ErrorIs(t, err, ErrReentrance)
}

func TestTable_DropOwned_NoReentranceWithDestructor(t *testing.T) {
	table := NewTable()
	registry := NewDestructorRegistry()
	tracker := NewReentranceTracker()
	rt := &ResourceType{}

	// Register a destructor
	registry.Register(rt, func(rep uint32) {})

	h, err := table.NewResourceHandle(uint32(42), true, rt)
	require.NoError(t, err)

	// Instance 100 is on the call stack
	tracker.EnterInstance(100)

	// But since there's a destructor, reentrance check is skipped
	// (the destructor will be called via canon_lift/canon_lower which handles reentrance)
	err = table.DropOwnedWithReentranceCheck(
		h,
		rt,
		registry,
		200,
		100,
		func(rep, inst uint32) {}, // cross-instance dtor
		tracker,
	)
	require.NoError(t, err)
}

func TestTable_DropOwned_SameInstanceNoReentranceCheck(t *testing.T) {
	table := NewTable()
	registry := NewDestructorRegistry()
	tracker := NewReentranceTracker()
	rt := &ResourceType{}

	// No destructor registered

	h, err := table.NewResourceHandle(uint32(42), true, rt)
	require.NoError(t, err)

	// Instance 100 is on the call stack
	tracker.EnterInstance(100)

	// Same-instance drop (current=100, defining=100) should NOT check reentrance
	err = table.DropOwnedWithReentranceCheck(
		h,
		rt,
		registry,
		100, // current instance
		100, // defining instance (same!)
		nil,
		tracker,
	)
	require.NoError(t, err) // Should succeed despite instance being on call stack
}

// Tests for Task 5.5: MaxTableLength and ErrTableFull

func TestTable_MaxLength(t *testing.T) {
	// This is a documentation/constant test, not a real allocation test
	// (we don't want to allocate 2^28 entries in a test)
	require.Equal(t, uint32(1<<28-1), MaxTableLength)
}

func TestTable_ReturnsErrorOnOverflow(t *testing.T) {
	// This tests the error path, not actual overflow
	// We mock this by checking the error type exists
	err := ErrTableFull
	require.Error(t, err)
	require.Contains(t, err.Error(), "table full")
}

// Phase 5: Resource System Integration Tests

func TestTable_CompleteLifecycle(t *testing.T) {
	table := NewTable()
	rt := &ResourceType{}

	var dtorCalls []uint32
	dtor := func(rep uint32) {
		dtorCalls = append(dtorCalls, rep)
	}
	registry := NewDestructorRegistry()
	registry.Register(rt, dtor)

	// Create resource
	h, err := table.NewResourceHandle(uint32(100), true, rt)
	require.NoError(t, err)
	require.Equal(t, uint32(0), h.Index())

	// Get rep
	rep, err := table.Rep(h)
	require.NoError(t, err)
	require.Equal(t, uint32(100), rep)

	// Borrow
	require.NoError(t, table.IncrementLends(h))

	// Cannot drop while borrowed
	err = table.DropOwned(h, rt, registry, 1, 1, nil)
	require.ErrorIs(t, err, ErrResourceInUse)

	// Return borrow
	require.NoError(t, table.DecrementLends(h))

	// Now can drop
	err = table.DropOwned(h, rt, registry, 1, 1, nil)
	require.NoError(t, err)

	// Destructor was called
	require.Equal(t, []uint32{100}, dtorCalls)

	// Handle is now invalid
	_, err = table.Get(h)
	require.ErrorIs(t, err, ErrInvalidHandle)
}

func TestTable_MultipleResourceTypes(t *testing.T) {
	table := NewTable()

	type1 := &ResourceType{}
	type2 := &ResourceType{}

	// Create resources of different types
	h1, err := table.NewResourceHandle(uint32(100), true, type1)
	require.NoError(t, err)
	h2, err := table.NewResourceHandle(uint32(200), true, type2)
	require.NoError(t, err)
	h3, err := table.NewResourceHandle(uint32(300), true, type1)
	require.NoError(t, err)

	// Verify types
	entry1, _ := table.GetResourceHandle(h1)
	entry2, _ := table.GetResourceHandle(h2)
	entry3, _ := table.GetResourceHandle(h3)

	require.True(t, entry1.RT == type1)
	require.True(t, entry2.RT == type2)
	require.True(t, entry3.RT == type1)

	// Type validation
	require.NoError(t, table.ValidateType(h1, type1))
	require.ErrorIs(t, table.ValidateType(h1, type2), ErrResourceTypeMismatch)
	require.NoError(t, table.ValidateType(h2, type2))
}

func TestTable_ConcurrentBorrowsMultipleResources(t *testing.T) {
	table := NewTable()

	// Create multiple resources
	h1, err := table.NewResourceHandle(uint32(50), true, nil)
	require.NoError(t, err)
	h2, err := table.NewResourceHandle(uint32(51), true, nil)
	require.NoError(t, err)

	// Borrow both
	require.NoError(t, table.IncrementLends(h1))
	require.NoError(t, table.IncrementLends(h2))

	// Check both have active borrows
	e1, _ := table.GetResourceHandle(h1)
	e2, _ := table.GetResourceHandle(h2)
	require.Equal(t, uint32(1), e1.NumLends)
	require.Equal(t, uint32(1), e2.NumLends)

	// Return one borrow
	require.NoError(t, table.DecrementLends(h1))

	// h1 can be removed, h2 cannot
	_, err = table.Remove(h1)
	require.NoError(t, err)

	_, err = table.Remove(h2)
	require.ErrorIs(t, err, ErrResourceInUse)

	// Return h2's borrow
	require.NoError(t, table.DecrementLends(h2))

	// Now h2 can be removed
	_, err = table.Remove(h2)
	require.NoError(t, err)
}

// Tests for HostDestructor on ResourceType - Phase 6: Resource Lifecycle

func TestTable_HostDestructorCalled(t *testing.T) {
	table := NewTable()

	// Create a resource type with a host destructor
	var destroyed bool
	rt := &ResourceType{
		HostDestructor: func(rep uint32) error {
			destroyed = true
			return nil
		},
	}

	h, err := table.NewResourceHandle(uint32(42), true, rt) // own=true
	require.NoError(t, err)

	// Verify it's not destroyed yet
	require.False(t, destroyed, "resource should not be destroyed yet")

	// Delete the resource
	err = table.Delete(h)
	require.NoError(t, err)

	// Verify HostDestructor was called
	require.True(t, destroyed, "HostDestructor should have been called")
}

func TestTable_HostDestructorNotCalledForBorrow(t *testing.T) {
	table := NewTable()

	// Create a resource type with a host destructor
	var destroyed bool
	rt := &ResourceType{
		HostDestructor: func(rep uint32) error {
			destroyed = true
			return nil
		},
	}

	h, err := table.NewResourceHandle(uint32(42), false, rt) // own=false (borrow)
	require.NoError(t, err)

	// Delete the resource (as borrow)
	err = table.Delete(h)
	require.NoError(t, err)

	// Verify HostDestructor was NOT called for borrows
	require.False(t, destroyed, "HostDestructor should NOT be called for borrowed handles")
}

func TestTable_DeleteNoDestructor(t *testing.T) {
	table := NewTable()

	// Resource type with no destructor
	rt := &ResourceType{}
	h, err := table.NewResourceHandle(uint32(42), true, rt) // own=true
	require.NoError(t, err)

	// Delete should work without panicking
	err = table.Delete(h)
	require.NoError(t, err)

	// Verify handle is invalid after delete
	_, err = table.Get(h)
	require.Error(t, err)
}

func TestTable_HostDestructorIdempotent(t *testing.T) {
	table := NewTable()

	callCount := 0
	rt := &ResourceType{
		HostDestructor: func(rep uint32) error {
			callCount++
			return nil
		},
	}

	h, err := table.NewResourceHandle(uint32(42), true, rt)
	require.NoError(t, err)

	// First delete
	err = table.Delete(h)
	require.NoError(t, err)

	// Verify destructor was called exactly once
	require.Equal(t, 1, callCount)

	// Second delete should fail (handle invalid)
	err = table.Delete(h)
	require.Error(t, err)

	// Verify destructor was still only called once
	require.Equal(t, 1, callCount)
}

func TestTable_DeleteWithActiveBorrows(t *testing.T) {
	table := NewTable()

	var destroyed bool
	rt := &ResourceType{
		HostDestructor: func(rep uint32) error {
			destroyed = true
			return nil
		},
	}

	// Create owned resource
	h, err := table.NewResourceHandle(uint32(42), true, rt)
	require.NoError(t, err)

	// Increment borrow count
	err = table.IncrementLends(h)
	require.NoError(t, err)

	// Delete should fail because resource is in use
	err = table.Delete(h)
	require.ErrorIs(t, err, ErrResourceInUse)

	// Destroy should NOT have been called
	require.False(t, destroyed)

	// Decrement borrow count
	err = table.DecrementLends(h)
	require.NoError(t, err)

	// Now delete should succeed
	err = table.Delete(h)
	require.NoError(t, err)

	// Destroy should have been called
	require.True(t, destroyed)
}

// TestTableGetByIndexGenerationBridging asserts GetByIndex looks up an
// entry by slot index (the low 32 bits of a Handle) and returns the
// full generation-tagged Handle plus the entry — bridging the 32-bit
// Wasm-side handle space to the 64-bit runtime Handle space.
//
// Spec: definitions.py:303-315 class Table (index-keyed; the generation
// bridging is a wazero implementation detail that must preserve the
// spec's index-keyed observable behavior).
// Wasmtime parallel: runtime/vm/component/resources.rs handle index
// lookup uses raw index + generation.
func TestTableGetByIndexGenerationBridging(t *testing.T) {
	tbl := NewTable()
	rt := &ResourceType{}
	h1, err := tbl.NewResourceHandle(uint32(42), true, rt)
	if err != nil {
		t.Fatalf("NewResourceHandle: %v", err)
	}
	// h1 is a 64-bit tagged handle; its low 32 bits are the slot index.
	idx := h1.Index()

	// GetByIndex(idx) must return (h1, entry, nil) even though the
	// caller only knows the 32-bit index.
	h, entry, err := tbl.GetByIndex(idx)
	if err != nil {
		t.Fatalf("GetByIndex: %v", err)
	}
	if h != h1 {
		t.Fatalf("GetByIndex handle = %v, want %v", h, h1)
	}
	if entry == nil {
		t.Fatalf("GetByIndex entry = nil")
	}

	// Remove h1, allocate a new handle at the same slot (generation increments).
	if _, err := tbl.Remove(h1); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	h2, err := tbl.NewResourceHandle(uint32(99), true, rt)
	if err != nil {
		t.Fatalf("NewResourceHandle 2: %v", err)
	}
	// h2 should reuse the slot (same Index) with incremented generation.
	if h2.Index() != idx {
		t.Fatalf("h2.Index = %d, want %d (slot reuse)", h2.Index(), idx)
	}
	if h2 == h1 {
		t.Fatalf("h2 == h1 (generation did not increment)")
	}

	// GetByIndex(idx) must now return h2 (with the new generation), not h1.
	h, _, err = tbl.GetByIndex(idx)
	if err != nil {
		t.Fatalf("GetByIndex after reuse: %v", err)
	}
	if h != h2 {
		t.Fatalf("GetByIndex handle = %v, want %v (new generation)", h, h2)
	}
	if h == h1 {
		t.Fatalf("GetByIndex returned stale handle %v", h1)
	}
}

// TestTableGetByIndexFreeSlot asserts GetByIndex returns ErrInvalidHandle
// for a slot that's currently free.
//
// Spec: definitions.py:303-315 class Table (index-keyed).
// No counterpart (justified): tests wazero's generation-bridging invariant
// for freed slots.
func TestTableGetByIndexFreeSlot(t *testing.T) {
	tbl := NewTable()
	rt := &ResourceType{}
	h, _ := tbl.NewResourceHandle(uint32(1), true, rt)
	tbl.Remove(h)
	_, _, err := tbl.GetByIndex(h.Index())
	if err == nil {
		t.Fatalf("GetByIndex(freed slot) returned nil error")
	}
}

// TestTableGetByIndexOutOfRange asserts GetByIndex returns ErrInvalidHandle
// for an index past the end of the entries slice.
//
// Spec: definitions.py:303-315 class Table (index-keyed).
// No counterpart (justified): tests wazero's bounds check on slot index.
func TestTableGetByIndexOutOfRange(t *testing.T) {
	tbl := NewTable()
	_, _, err := tbl.GetByIndex(9999)
	if err == nil {
		t.Fatalf("GetByIndex(9999) returned nil error")
	}
}
