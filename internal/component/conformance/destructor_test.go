// Package conformance contains conformance tests for the Component Model implementation.
// This file implements Task 4.5: Destructor Integration Tests.
// Tests validate the destructor system for same-instance, cross-instance, and no-destructor scenarios.
package conformance

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// =============================================================================
// Task 4.5: Destructor Integration Tests
// =============================================================================

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

	require.Equal(t, 1, len(crossCalls))
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

// =============================================================================
// Additional Destructor Edge Case Tests
// =============================================================================

// TestDestructor_MultipleResourceTypes verifies destructors are called for the correct type.
func TestDestructor_MultipleResourceTypes(t *testing.T) {
	table := component.NewResourceTable()
	registry := component.NewDestructorRegistry()

	var type1Calls []uint32
	var type2Calls []uint32

	registry.Register(component.NewResourceTypeID(1), func(rep uint32) {
		type1Calls = append(type1Calls, rep)
	})
	registry.Register(component.NewResourceTypeID(2), func(rep uint32) {
		type2Calls = append(type2Calls, rep)
	})

	// Create resources of different types
	h1a := table.NewWithType(uint32(10), true, component.NewResourceTypeID(1))
	h2a := table.NewWithType(uint32(20), true, component.NewResourceTypeID(2))
	h1b := table.NewWithType(uint32(11), true, component.NewResourceTypeID(1))
	h2b := table.NewWithType(uint32(21), true, component.NewResourceTypeID(2))

	// Drop in mixed order
	require.NoError(t, table.DropOwned(h2a, component.NewResourceTypeID(2), registry, 100, 100, nil))
	require.NoError(t, table.DropOwned(h1a, component.NewResourceTypeID(1), registry, 100, 100, nil))
	require.NoError(t, table.DropOwned(h1b, component.NewResourceTypeID(1), registry, 100, 100, nil))
	require.NoError(t, table.DropOwned(h2b, component.NewResourceTypeID(2), registry, 100, 100, nil))

	// Verify each destructor was called with correct reps
	require.Equal(t, []uint32{10, 11}, type1Calls)
	require.Equal(t, []uint32{20, 21}, type2Calls)
}

// TestDestructor_CrossInstanceNoCallback verifies behavior when cross-instance
// destructor is nil (no callback provided).
func TestDestructor_CrossInstanceNoCallback(t *testing.T) {
	table := component.NewResourceTable()
	registry := component.NewDestructorRegistry()

	registry.Register(component.NewResourceTypeID(1), func(rep uint32) {
		t.Fatal("same-instance destructor should not be called for cross-instance drop")
	})

	h := table.NewWithType(uint32(42), true, component.NewResourceTypeID(1))

	// Drop from instance 200, type defined in instance 100, but no cross-instance callback
	// This should succeed but not call the same-instance destructor
	require.NoError(t, table.DropOwned(h, component.NewResourceTypeID(1), registry, 200, 100, nil))

	// Handle should be gone
	_, err := table.Get(h)
	require.Error(t, err)
}

// TestDestructor_DropOwnedWithActiveBorrows verifies drop fails if borrows are active.
func TestDestructor_DropOwnedWithActiveBorrows(t *testing.T) {
	table := component.NewResourceTable()
	registry := component.NewDestructorRegistry()

	destructorCalled := false
	registry.Register(component.NewResourceTypeID(1), func(rep uint32) {
		destructorCalled = true
	})

	h := table.NewWithType(uint32(42), true, component.NewResourceTypeID(1))

	// Create an active borrow
	require.NoError(t, table.IncrementLends(h))

	// Drop should fail due to active borrows
	err := table.DropOwned(h, component.NewResourceTypeID(1), registry, 100, 100, nil)
	require.ErrorIs(t, err, component.ErrResourceInUse)

	// Destructor should not have been called
	require.False(t, destructorCalled)

	// Handle should still be valid
	entry, err := table.Get(h)
	require.NoError(t, err)
	require.Equal(t, uint32(1), entry.NumLends)

	// Release borrow and drop again
	require.NoError(t, table.DecrementLends(h))
	require.NoError(t, table.DropOwned(h, component.NewResourceTypeID(1), registry, 100, 100, nil))

	// Now destructor should have been called
	require.True(t, destructorCalled)
}

// TestDestructor_TypeMismatch verifies drop fails on type mismatch.
func TestDestructor_TypeMismatch(t *testing.T) {
	table := component.NewResourceTable()
	registry := component.NewDestructorRegistry()

	destructorCalled := false
	registry.Register(component.NewResourceTypeID(1), func(rep uint32) {
		destructorCalled = true
	})

	// Create resource with type 1
	h := table.NewWithType(uint32(42), true, component.NewResourceTypeID(1))

	// Try to drop with type 2 - should fail
	err := table.DropOwned(h, component.NewResourceTypeID(2), registry, 100, 100, nil)
	require.ErrorIs(t, err, component.ErrResourceTypeMismatch)

	// Destructor should not have been called
	require.False(t, destructorCalled)

	// Handle should still be valid
	_, err = table.Get(h)
	require.NoError(t, err)
}

// TestDestructor_DoubleDropFails verifies double drop is properly rejected.
func TestDestructor_DoubleDropFails(t *testing.T) {
	table := component.NewResourceTable()
	registry := component.NewDestructorRegistry()

	callCount := 0
	registry.Register(component.NewResourceTypeID(1), func(rep uint32) {
		callCount++
	})

	h := table.NewWithType(uint32(42), true, component.NewResourceTypeID(1))

	// First drop succeeds
	require.NoError(t, table.DropOwned(h, component.NewResourceTypeID(1), registry, 100, 100, nil))
	require.Equal(t, 1, callCount)

	// Second drop fails
	err := table.DropOwned(h, component.NewResourceTypeID(1), registry, 100, 100, nil)
	require.ErrorIs(t, err, component.ErrInvalidHandle)

	// Destructor called only once
	require.Equal(t, 1, callCount)
}

// TestDestructor_InvalidHandle verifies drop with invalid handle returns error.
func TestDestructor_InvalidHandle(t *testing.T) {
	table := component.NewResourceTable()
	registry := component.NewDestructorRegistry()

	registry.Register(component.NewResourceTypeID(1), func(rep uint32) {
		t.Fatal("destructor should not be called for invalid handle")
	})

	// Try to drop a handle that was never created
	invalidHandle := component.MakeHandle(999, 0)
	err := table.DropOwned(invalidHandle, component.NewResourceTypeID(1), registry, 100, 100, nil)
	require.ErrorIs(t, err, component.ErrInvalidHandle)
}

// TestDestructor_SameInstanceZeroIDs verifies same-instance check with zero instance IDs.
func TestDestructor_SameInstanceZeroIDs(t *testing.T) {
	table := component.NewResourceTable()
	registry := component.NewDestructorRegistry()

	var called bool
	registry.Register(component.NewResourceTypeID(1), func(rep uint32) {
		called = true
	})

	h := table.NewWithType(uint32(42), true, component.NewResourceTypeID(1))

	// Both instance IDs are 0 - should be same-instance
	require.NoError(t, table.DropOwned(h, component.NewResourceTypeID(1), registry, 0, 0, nil))
	require.True(t, called)
}
