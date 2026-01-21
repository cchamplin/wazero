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
