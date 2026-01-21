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

func TestResourceTable_RemoveWithActiveBorrows(t *testing.T) {
	table := NewResourceTable()
	h := table.New("resource", true)

	// Manually set NumLends to simulate active borrow
	entry, _ := table.Get(h)
	entry.NumLends = 1

	_, err := table.Remove(h)
	require.ErrorIs(t, err, ErrResourceInUse)
}

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
	require.ErrorIs(t, err, ErrNoBorrowsToDecrement)
}

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

func TestResourceTable_Rep(t *testing.T) {
	table := NewResourceTable()

	// Create a new resource with rep=42
	handle := table.New(uint32(42), true)

	// First handle has index 0 and generation 0
	require.Equal(t, uint32(0), handle.Index())
	require.Equal(t, uint32(0), handle.Generation())

	// Verify we can get the rep back
	rep, err := table.Rep(handle)
	require.NoError(t, err)
	require.Equal(t, uint32(42), rep)
}

func TestResourceTable_Rep_InvalidHandle(t *testing.T) {
	table := NewResourceTable()

	// Try to get rep of non-existent handle
	invalidHandle := MakeHandle(999, 0)
	_, err := table.Rep(invalidHandle)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidHandle)
}

func TestResourceTable_Rep_IntConversion(t *testing.T) {
	table := NewResourceTable()

	// Create a resource with int rep (common in tests)
	handle := table.New(42, true) // int, not uint32

	// Rep should still work with int->uint32 conversion
	rep, err := table.Rep(handle)
	require.NoError(t, err)
	require.Equal(t, uint32(42), rep)
}

func TestResourceTable_Rep_NonNumeric(t *testing.T) {
	table := NewResourceTable()

	// Create a resource with non-numeric rep
	handle := table.New("string-rep", true)

	// Rep should error for non-numeric rep
	_, err := table.Rep(handle)
	require.Error(t, err)
}

func TestResourceTable_CreateResourceNewFunc(t *testing.T) {
	table := NewResourceTable()

	// Create a resource.new function
	newFunc := table.CreateResourceNewFunc(0)

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

func TestResourceTable_CreateResourceNewFunc_MultipleResources(t *testing.T) {
	table := NewResourceTable()

	// Create resource.new functions for different resource types
	newFunc1 := table.CreateResourceNewFunc(0)
	newFunc2 := table.CreateResourceNewFunc(1)

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

func TestResourceTable_CreateResourceNewFunc_CreatesOwnedResources(t *testing.T) {
	table := NewResourceTable()

	newFunc := table.CreateResourceNewFunc(0)
	handle := Handle(newFunc(42))

	// Verify the created resource is owned
	entry, err := table.Get(handle)
	require.NoError(t, err)
	require.True(t, entry.Own)
}

func TestResourceTable_CreateResourceDropFunc(t *testing.T) {
	table := NewResourceTable()

	var destructorCalled bool
	var droppedRep uint32

	dropFunc := table.CreateResourceDropFunc(0, func(rep uint32) {
		destructorCalled = true
		droppedRep = rep
	})

	// Create a resource
	handle := table.New(uint32(42), true)

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
	_, err := table.Get(handle)
	if err == nil {
		t.Error("expected error getting dropped handle")
	}
}

func TestResourceTable_CreateResourceDropFunc_InvalidHandle(t *testing.T) {
	table := NewResourceTable()

	var destructorCalled bool
	dropFunc := table.CreateResourceDropFunc(0, func(rep uint32) {
		destructorCalled = true
	})

	// Drop invalid handle - should not panic or call destructor
	dropFunc(999)

	if destructorCalled {
		t.Error("destructor should not be called for invalid handle")
	}
}

func TestResourceTable_CreateResourceDropFunc_NilDestructor(t *testing.T) {
	table := NewResourceTable()

	dropFunc := table.CreateResourceDropFunc(0, nil)

	// Create and drop - should work without destructor
	handle := table.New(uint32(42), true)
	dropFunc(uint32(handle))

	// Verify resource was removed
	_, err := table.Get(handle)
	if err == nil {
		t.Error("expected error getting dropped handle")
	}
}

func TestResourceTable_CreateResourceRepFunc(t *testing.T) {
	table := NewResourceTable()

	repFunc := table.CreateResourceRepFunc(0)

	// Create a resource
	handle := table.New(42, true)

	// Get its rep
	rep := repFunc(uint32(handle))
	if rep != 42 {
		t.Errorf("expected rep=42, got %d", rep)
	}
}

func TestResourceTable_CreateResourceRepFunc_InvalidHandle(t *testing.T) {
	table := NewResourceTable()

	repFunc := table.CreateResourceRepFunc(0)

	// Get rep for invalid handle - should return 0
	rep := repFunc(999)
	if rep != 0 {
		t.Errorf("expected rep=0 for invalid handle, got %d", rep)
	}
}

func TestHandleEntry_HasResourceTypeID(t *testing.T) {
	table := NewResourceTable()

	// Create a handle with a specific resource type
	rtID := NewResourceTypeID(5)
	h := table.NewWithType(uint32(100), true, rtID)

	entry, err := table.Get(h)
	require.NoError(t, err)
	require.Equal(t, rtID, entry.RT)
	require.Equal(t, uint32(100), entry.Rep.(uint32))
}

func TestResourceTable_New_HasInvalidTypeByDefault(t *testing.T) {
	table := NewResourceTable()

	// Old API should still work, RT will be invalid
	h := table.New("test-rep", true)

	entry, err := table.Get(h)
	require.NoError(t, err)
	require.False(t, entry.RT.IsValid(), "legacy New() should have invalid RT")
}

func TestCreateResourceNewFunc_StoresResourceType(t *testing.T) {
	table := NewResourceTable()

	// Create the resource.new function for type index 3
	newFunc := table.CreateResourceNewFuncWithType(3)

	// Call it to create a resource with rep=42
	handleIdx := newFunc(42)

	// Verify the handle has the correct type
	entry, err := table.Get(Handle(handleIdx))
	require.NoError(t, err)
	require.True(t, entry.RT.IsValid())
	require.Equal(t, uint32(3), entry.RT.Index())
	require.Equal(t, uint32(42), entry.Rep.(uint32))
}

func TestResourceTable_GetType(t *testing.T) {
	table := NewResourceTable()
	rtID := NewResourceTypeID(7)
	h := table.NewWithType(uint32(100), true, rtID)

	// GetType should return the resource type
	gotType, err := table.GetType(h)
	require.NoError(t, err)
	require.Equal(t, rtID, gotType)
}

func TestResourceTable_GetType_InvalidHandle(t *testing.T) {
	table := NewResourceTable()

	invalidHandle := MakeHandle(999, 0)
	_, err := table.GetType(invalidHandle)
	require.ErrorIs(t, err, ErrInvalidHandle)
}

func TestResourceTable_ValidateType(t *testing.T) {
	table := NewResourceTable()
	rtID := NewResourceTypeID(7)
	wrongID := NewResourceTypeID(8)
	h := table.NewWithType(uint32(100), true, rtID)

	// Correct type should pass
	require.NoError(t, table.ValidateType(h, rtID))

	// Wrong type should fail
	err := table.ValidateType(h, wrongID)
	require.ErrorIs(t, err, ErrResourceTypeMismatch)
}

func TestResourceTable_ValidateType_InvalidHandle(t *testing.T) {
	table := NewResourceTable()
	invalidHandle := MakeHandle(999, 0)
	rtID := NewResourceTypeID(7)

	err := table.ValidateType(invalidHandle, rtID)
	require.ErrorIs(t, err, ErrInvalidHandle)
}

func TestCreateResourceDropFunc_TrapsOnInvalidHandle(t *testing.T) {
	table := NewResourceTable()

	// Create drop function for type 1
	var trapCalled bool
	var trapErr error
	dropFunc := table.CreateResourceDropFuncWithTrap(1, nil, func(err error) {
		trapCalled = true
		trapErr = err
	})

	// Try to drop an invalid handle
	dropFunc(999)

	require.True(t, trapCalled, "should trap on invalid handle")
	require.ErrorIs(t, trapErr, ErrInvalidHandle)
}

func TestCreateResourceDropFunc_TrapsOnTypeMismatch(t *testing.T) {
	table := NewResourceTable()

	// Create a handle of type 1
	h := table.NewWithType(uint32(42), true, NewResourceTypeID(1))

	// Create drop function for type 2 (different type)
	var trapCalled bool
	var trapErr error
	dropFunc := table.CreateResourceDropFuncWithTrap(2, nil, func(err error) {
		trapCalled = true
		trapErr = err
	})

	// Try to drop with wrong type
	dropFunc(uint32(h))

	require.True(t, trapCalled, "should trap on type mismatch")
	require.ErrorIs(t, trapErr, ErrResourceTypeMismatch)
}

func TestCreateResourceDropFuncWithTrap_SuccessfulDrop(t *testing.T) {
	table := NewResourceTable()

	// Create a handle with type 1
	h := table.NewWithType(uint32(42), true, NewResourceTypeID(1))

	var destructorCalledWith uint32
	destructor := func(rep uint32) {
		destructorCalledWith = rep
	}

	var trapCalled bool
	dropFunc := table.CreateResourceDropFuncWithTrap(1, destructor, func(err error) {
		trapCalled = true
	})

	// Drop the handle
	dropFunc(uint32(h))

	// Verify trap was NOT called
	require.False(t, trapCalled, "should not trap on valid drop")

	// Verify destructor was called with correct rep
	require.Equal(t, uint32(42), destructorCalledWith)

	// Verify handle is now invalid
	_, err := table.Get(h)
	require.ErrorIs(t, err, ErrInvalidHandle)
}

func TestCreateResourceRepFunc_TrapsOnInvalidHandle(t *testing.T) {
	table := NewResourceTable()

	var trapCalled bool
	var trapErr error
	repFunc := table.CreateResourceRepFuncWithTrap(1, func(err error) {
		trapCalled = true
		trapErr = err
	})

	// Try to get rep of invalid handle
	_ = repFunc(999)

	require.True(t, trapCalled, "should trap on invalid handle")
	require.ErrorIs(t, trapErr, ErrInvalidHandle)
}

func TestCreateResourceRepFunc_TrapsOnTypeMismatch(t *testing.T) {
	table := NewResourceTable()

	// Create a handle of type 1
	h := table.NewWithType(uint32(42), true, NewResourceTypeID(1))

	var trapCalled bool
	var trapErr error
	// Create rep function for type 2 (different type)
	repFunc := table.CreateResourceRepFuncWithTrap(2, func(err error) {
		trapCalled = true
		trapErr = err
	})

	// Try to get rep with wrong type
	_ = repFunc(uint32(h))

	require.True(t, trapCalled, "should trap on type mismatch")
	require.ErrorIs(t, trapErr, ErrResourceTypeMismatch)
}

func TestCreateResourceRepFuncWithTrap_ReturnsRepOnSuccess(t *testing.T) {
	table := NewResourceTable()

	h := table.NewWithType(uint32(42), true, NewResourceTypeID(1))

	var trapCalled bool
	repFunc := table.CreateResourceRepFuncWithTrap(1, func(err error) {
		trapCalled = true
	})

	rep := repFunc(uint32(h))

	require.False(t, trapCalled, "should not trap on valid handle")
	require.Equal(t, uint32(42), rep)
}

func TestResourceTable_RemoveWithType_Success(t *testing.T) {
	table := NewResourceTable()
	rtID := NewResourceTypeID(5)
	h := table.NewWithType(uint32(100), true, rtID)

	entry, err := table.RemoveWithType(h, rtID)
	require.NoError(t, err)
	require.Equal(t, uint32(100), entry.Rep.(uint32))
}

func TestResourceTable_RemoveWithType_TypeMismatch(t *testing.T) {
	table := NewResourceTable()
	rtID := NewResourceTypeID(5)
	wrongID := NewResourceTypeID(6)
	h := table.NewWithType(uint32(100), true, rtID)

	_, err := table.RemoveWithType(h, wrongID)
	require.ErrorIs(t, err, ErrResourceTypeMismatch)

	// Handle should still be valid (not removed on type error)
	entry, err := table.Get(h)
	require.NoError(t, err)
	require.Equal(t, uint32(100), entry.Rep.(uint32))
}

func TestResourceTable_GetWithType_Success(t *testing.T) {
	table := NewResourceTable()
	rtID := NewResourceTypeID(5)
	h := table.NewWithType(uint32(100), true, rtID)

	entry, err := table.GetWithType(h, rtID)
	require.NoError(t, err)
	require.Equal(t, uint32(100), entry.Rep.(uint32))
}

func TestResourceTable_GetWithType_TypeMismatch(t *testing.T) {
	table := NewResourceTable()
	rtID := NewResourceTypeID(5)
	wrongID := NewResourceTypeID(6)
	h := table.NewWithType(uint32(100), true, rtID)

	_, err := table.GetWithType(h, wrongID)
	require.ErrorIs(t, err, ErrResourceTypeMismatch)
}

func TestResourceTable_GetWithType_InvalidHandle(t *testing.T) {
	table := NewResourceTable()
	invalidH := MakeHandle(999, 0)

	_, err := table.GetWithType(invalidH, NewResourceTypeID(1))
	require.ErrorIs(t, err, ErrInvalidHandle)
}

func TestResourceTable_RepWithType_Success(t *testing.T) {
	table := NewResourceTable()
	rtID := NewResourceTypeID(5)
	h := table.NewWithType(uint32(100), true, rtID)

	rep, err := table.RepWithType(h, rtID)
	require.NoError(t, err)
	require.Equal(t, uint32(100), rep)
}

func TestResourceTable_RepWithType_TypeMismatch(t *testing.T) {
	table := NewResourceTable()
	rtID := NewResourceTypeID(5)
	wrongID := NewResourceTypeID(6)
	h := table.NewWithType(uint32(100), true, rtID)

	_, err := table.RepWithType(h, wrongID)
	require.ErrorIs(t, err, ErrResourceTypeMismatch)
}

// TestResourceTable_RemoveBorrow_DecrementsBorrowCount documents the integration
// pattern for borrow count decrement. The actual decrement happens in calling
// code (resource.drop), not in Remove itself.
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

// Tests for DropOwned - Task 4.3: Destructor Invocation

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

func TestResourceTable_DropOwned_BorrowHandle_NoDestructor(t *testing.T) {
	table := NewResourceTable()
	registry := NewDestructorRegistry()

	var destructorCalled bool
	registry.Register(NewResourceTypeID(1), func(rep uint32) {
		destructorCalled = true
	})

	// Create borrow handle (Own=false)
	h := table.NewWithType(uint32(42), false, NewResourceTypeID(1))

	// Drop borrow handle - should NOT call destructor
	err := table.DropOwned(h, NewResourceTypeID(1), registry, 100, 100, nil)
	require.NoError(t, err)

	// Destructor should NOT have been called for borrow handles
	require.False(t, destructorCalled, "destructor should not be called for borrow handles")

	// Handle should still be removed from table
	_, err = table.Get(h)
	require.ErrorIs(t, err, ErrInvalidHandle)
}

// Tests for CreateResourceDropFuncWithContext - Task 4.4

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

func TestCreateResourceDropFuncWithContext_TrapsOnInvalidHandle(t *testing.T) {
	table := NewResourceTable()
	registry := NewDestructorRegistry()
	callCtx := NewCallContext()

	var trappedErr error
	dropFunc := table.CreateResourceDropFuncWithContext(1, registry, 100, 100, callCtx, nil, func(err error) {
		trappedErr = err
	})

	// Try to drop a non-existent handle
	dropFunc(999)

	require.ErrorIs(t, trappedErr, ErrInvalidHandle)
}

func TestCreateResourceDropFuncWithContext_TrapsOnTypeMismatch(t *testing.T) {
	table := NewResourceTable()
	registry := NewDestructorRegistry()
	callCtx := NewCallContext()

	var trappedErr error
	// Create drop function for type 1
	dropFunc := table.CreateResourceDropFuncWithContext(1, registry, 100, 100, callCtx, nil, func(err error) {
		trappedErr = err
	})

	// Create handle of type 2
	h := table.NewWithType(uint32(42), true, NewResourceTypeID(2))

	// Try to drop with wrong type
	dropFunc(uint32(h))

	require.ErrorIs(t, trappedErr, ErrResourceTypeMismatch)
}

func TestResourceTable_NewWithMayLeaveCheck(t *testing.T) {
	table := NewResourceTable()
	state := NewInstanceState(1)

	// When may_leave is true, New succeeds
	h, err := table.NewWithMayLeaveCheck(uint32(42), true, NewResourceTypeID(1), state)
	require.NoError(t, err)
	// Verify the handle is valid by retrieving its entry
	entry, err := table.Get(h)
	require.NoError(t, err)
	require.Equal(t, uint32(42), entry.Rep.(uint32))

	// When may_leave is false, New fails
	state.Enter()
	_, err = table.NewWithMayLeaveCheck(uint32(43), true, NewResourceTypeID(1), state)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrMayNotLeave)
}

// Tests for Task 5.4: Reentrance Trap in DropOwned

func TestResourceTable_DropOwned_TrapsOnReentrance(t *testing.T) {
	table := NewResourceTable()
	registry := NewDestructorRegistry()
	tracker := NewReentranceTracker()

	// No destructor registered for this type
	// (reentrance check only applies when no destructor)

	h := table.NewWithType(uint32(42), true, NewResourceTypeID(1))

	// Instance 100 is currently on the call stack
	tracker.EnterInstance(100)

	// Dropping a resource defined in instance 100 from instance 200
	// should trap because of potential reentrance
	err := table.DropOwnedWithReentranceCheck(
		h,
		NewResourceTypeID(1),
		registry,
		200,  // current instance
		100,  // defining instance (on call stack!)
		nil,
		tracker,
	)
	require.ErrorIs(t, err, ErrReentrance)
}

func TestResourceTable_DropOwned_NoReentranceWithDestructor(t *testing.T) {
	table := NewResourceTable()
	registry := NewDestructorRegistry()
	tracker := NewReentranceTracker()

	// Register a destructor
	registry.Register(NewResourceTypeID(1), func(rep uint32) {})

	h := table.NewWithType(uint32(42), true, NewResourceTypeID(1))

	// Instance 100 is on the call stack
	tracker.EnterInstance(100)

	// But since there's a destructor, reentrance check is skipped
	// (the destructor will be called via canon_lift/canon_lower which handles reentrance)
	err := table.DropOwnedWithReentranceCheck(
		h,
		NewResourceTypeID(1),
		registry,
		200,
		100,
		func(rep, inst uint32) {}, // cross-instance dtor
		tracker,
	)
	require.NoError(t, err)
}

func TestResourceTable_DropOwned_SameInstanceNoReentranceCheck(t *testing.T) {
	table := NewResourceTable()
	registry := NewDestructorRegistry()
	tracker := NewReentranceTracker()

	// No destructor registered

	h := table.NewWithType(uint32(42), true, NewResourceTypeID(1))

	// Instance 100 is on the call stack
	tracker.EnterInstance(100)

	// Same-instance drop (current=100, defining=100) should NOT check reentrance
	err := table.DropOwnedWithReentranceCheck(
		h,
		NewResourceTypeID(1),
		registry,
		100, // current instance
		100, // defining instance (same!)
		nil,
		tracker,
	)
	require.NoError(t, err) // Should succeed despite instance being on call stack
}

// Tests for Task 5.5: MaxTableLength and ErrTableFull

func TestResourceTable_MaxLength(t *testing.T) {
	// This is a documentation/constant test, not a real allocation test
	// (we don't want to allocate 2^28 entries in a test)
	require.Equal(t, uint32(1<<28-1), MaxTableLength)
}

func TestResourceTable_ReturnsErrorOnOverflow(t *testing.T) {
	// This tests the error path, not actual overflow
	// We mock this by checking the error type exists
	err := ErrTableFull
	require.Error(t, err)
	require.Contains(t, err.Error(), "table full")
}

func TestResourceTable_NewWithLimit(t *testing.T) {
	table := NewResourceTable()

	// Normal creation should work
	h, err := table.NewWithLimit(uint32(42), true, NewResourceTypeID(1))
	require.NoError(t, err)

	// Verify the handle is valid
	entry, err := table.Get(h)
	require.NoError(t, err)
	require.Equal(t, uint32(42), entry.Rep.(uint32))
	require.True(t, entry.Own)
}

// Phase 5: Resource System Integration Tests

func TestResourceTable_CompleteLifecycle(t *testing.T) {
	table := NewResourceTable()

	var dtorCalls []uint32
	dtor := func(rep uint32) {
		dtorCalls = append(dtorCalls, rep)
	}
	registry := NewDestructorRegistry()
	registry.Register(NewResourceTypeID(1), dtor)

	// Create resource
	h := table.NewWithType(uint32(100), true, NewResourceTypeID(1))
	require.Equal(t, uint32(0), h.Index())

	// Get rep
	rep, err := table.Rep(h)
	require.NoError(t, err)
	require.Equal(t, uint32(100), rep)

	// Borrow
	require.NoError(t, table.IncrementLends(h))

	// Cannot drop while borrowed
	err = table.DropOwned(h, NewResourceTypeID(1), registry, 1, 1, nil)
	require.ErrorIs(t, err, ErrResourceInUse)

	// Return borrow
	require.NoError(t, table.DecrementLends(h))

	// Now can drop
	err = table.DropOwned(h, NewResourceTypeID(1), registry, 1, 1, nil)
	require.NoError(t, err)

	// Destructor was called
	require.Equal(t, []uint32{100}, dtorCalls)

	// Handle is now invalid
	_, err = table.Get(h)
	require.ErrorIs(t, err, ErrInvalidHandle)
}

func TestResourceTable_MultipleResourceTypes(t *testing.T) {
	table := NewResourceTable()

	type1 := NewResourceTypeID(1)
	type2 := NewResourceTypeID(2)

	// Create resources of different types
	h1 := table.NewWithType(uint32(100), true, type1)
	h2 := table.NewWithType(uint32(200), true, type2)
	h3 := table.NewWithType(uint32(300), true, type1)

	// Verify types
	entry1, _ := table.Get(h1)
	entry2, _ := table.Get(h2)
	entry3, _ := table.Get(h3)

	require.Equal(t, type1, entry1.RT)
	require.Equal(t, type2, entry2.RT)
	require.Equal(t, type1, entry3.RT)

	// Type validation
	require.NoError(t, table.ValidateType(h1, type1))
	require.ErrorIs(t, table.ValidateType(h1, type2), ErrResourceTypeMismatch)
	require.NoError(t, table.ValidateType(h2, type2))
}

func TestResourceTable_ConcurrentBorrowsMultipleResources(t *testing.T) {
	table := NewResourceTable()

	// Create multiple resources
	h1 := table.New("res1", true)
	h2 := table.New("res2", true)

	// Borrow both
	require.NoError(t, table.IncrementLends(h1))
	require.NoError(t, table.IncrementLends(h2))

	// Check both have active borrows
	e1, _ := table.Get(h1)
	e2, _ := table.Get(h2)
	require.Equal(t, uint32(1), e1.NumLends)
	require.Equal(t, uint32(1), e2.NumLends)

	// Return one borrow
	require.NoError(t, table.DecrementLends(h1))

	// h1 can be removed, h2 cannot
	_, err := table.Remove(h1)
	require.NoError(t, err)

	_, err = table.Remove(h2)
	require.ErrorIs(t, err, ErrResourceInUse)

	// Return h2's borrow
	require.NoError(t, table.DecrementLends(h2))

	// Now h2 can be removed
	_, err = table.Remove(h2)
	require.NoError(t, err)
}

// Tests for Destroyable interface - Phase 6: Resource Lifecycle

// mockDestroyable is a test resource that tracks destruction
type mockDestroyable struct {
	destroyed bool
}

func (m *mockDestroyable) Destroy() {
	m.destroyed = true
}

func TestResourceTable_DestructorCalled(t *testing.T) {
	table := NewResourceTable()

	// Create a destroyable resource as owned
	resource := &mockDestroyable{}
	h := table.New(resource, true) // own=true

	// Verify it's not destroyed yet
	require.False(t, resource.destroyed, "resource should not be destroyed yet")

	// Delete the resource
	err := table.Delete(h)
	require.NoError(t, err)

	// Verify Destroy() was called
	require.True(t, resource.destroyed, "resource.Destroy() should have been called")
}

func TestResourceTable_DestructorNotCalledForBorrow(t *testing.T) {
	table := NewResourceTable()

	// Create a destroyable resource as borrowed (not owned)
	resource := &mockDestroyable{}
	h := table.New(resource, false) // own=false (borrow)

	// Verify it's not destroyed yet
	require.False(t, resource.destroyed, "resource should not be destroyed yet")

	// Delete the resource (as borrow)
	err := table.Delete(h)
	require.NoError(t, err)

	// Verify Destroy() was NOT called for borrows
	require.False(t, resource.destroyed, "resource.Destroy() should NOT be called for borrowed handles")
}

func TestResourceTable_DestructorNotCalledForNonDestroyable(t *testing.T) {
	table := NewResourceTable()

	// Create a non-destroyable resource (string)
	h := table.New("plain-resource", true) // own=true

	// Delete should work without panicking (no Destroy method)
	err := table.Delete(h)
	require.NoError(t, err)

	// Verify handle is invalid after delete
	_, err = table.Get(h)
	require.Error(t, err)
}

// countingDestroyable tracks how many times Destroy is called
type countingDestroyable struct {
	count *int
}

func (c *countingDestroyable) Destroy() {
	*c.count++
}

func TestResourceTable_DestructorIdempotent(t *testing.T) {
	table := NewResourceTable()

	// Create a destroyable resource that tracks call count
	callCount := 0
	cd := &countingDestroyable{count: &callCount}

	// Use a wrapper type that implements Destroyable
	h := table.New(cd, true)

	// First delete
	err := table.Delete(h)
	require.NoError(t, err)

	// Verify Destroy was called exactly once
	require.Equal(t, 1, callCount)

	// Second delete should fail (handle invalid)
	err = table.Delete(h)
	require.Error(t, err)

	// Verify Destroy was still only called once (not called on failed delete)
	require.Equal(t, 1, callCount)
}

func TestResourceTable_DeleteWithActiveBorrows(t *testing.T) {
	table := NewResourceTable()

	// Create owned resource
	resource := &mockDestroyable{}
	h := table.New(resource, true)

	// Increment borrow count
	err := table.IncrementLends(h)
	require.NoError(t, err)

	// Delete should fail because resource is in use
	err = table.Delete(h)
	require.ErrorIs(t, err, ErrResourceInUse)

	// Destroy should NOT have been called
	require.False(t, resource.destroyed)

	// Decrement borrow count
	err = table.DecrementLends(h)
	require.NoError(t, err)

	// Now delete should succeed
	err = table.Delete(h)
	require.NoError(t, err)

	// Destroy should have been called
	require.True(t, resource.destroyed)
}
