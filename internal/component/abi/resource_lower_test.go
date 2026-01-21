package abi

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestLowerBorrowWithType_SameInstance_ReturnsRep(t *testing.T) {
	table := component.NewResourceTable()
	callCtx := component.NewCallContext()

	// Resource type defined in instance 100
	resourceTypeInfo := component.NewResourceTypeInfo(1, 100)

	// Lower borrow FROM instance 100 (same as defining instance)
	currentInstanceID := uint32(100)

	result, err := LowerBorrowWithType(table, callCtx, 42, resourceTypeInfo, currentInstanceID)
	require.NoError(t, err)

	// Should return rep directly (same-instance optimization)
	require.Equal(t, uint32(42), result)

	// No handle should be created in the table
	// (can verify by checking no borrow was incremented)
	require.Equal(t, 0, callCtx.NumBorrows())
}

func TestLowerBorrowWithType_DifferentInstance_CreatesHandle(t *testing.T) {
	table := component.NewResourceTable()
	callCtx := component.NewCallContext()

	// Resource type defined in instance 100
	resourceTypeInfo := component.NewResourceTypeInfo(1, 100)

	// Lower borrow FROM instance 200 (different from defining instance)
	currentInstanceID := uint32(200)

	result, err := LowerBorrowWithType(table, callCtx, 42, resourceTypeInfo, currentInstanceID)
	require.NoError(t, err)

	// Should return a handle index (not the rep directly)
	require.NotEqual(t, uint32(42), result)

	// A borrow should be tracked
	require.Equal(t, 1, callCtx.NumBorrows())

	// Handle should exist in the table
	entry, err := table.Get(component.Handle(result))
	require.NoError(t, err)
	require.False(t, entry.Own, "should be a borrow, not own")

	// Rep should be the original value
	rep, ok := entry.Rep.(uint32)
	require.True(t, ok, "Rep should be uint32")
	require.Equal(t, uint32(42), rep)
}

func TestLowerBorrowWithType_NilCallContext(t *testing.T) {
	table := component.NewResourceTable()

	// Resource type defined in instance 100
	resourceTypeInfo := component.NewResourceTypeInfo(1, 100)

	// Lower borrow FROM instance 200 with nil CallContext
	currentInstanceID := uint32(200)

	result, err := LowerBorrowWithType(table, nil, 42, resourceTypeInfo, currentInstanceID)
	require.NoError(t, err)

	// Should still create handle even without call context
	entry, err := table.Get(component.Handle(result))
	require.NoError(t, err)
	require.False(t, entry.Own)
}

func TestLowerBorrowWithType_MultipleBorrowsSameInstance(t *testing.T) {
	table := component.NewResourceTable()
	callCtx := component.NewCallContext()

	// Resource type defined in instance 100
	resourceTypeInfo := component.NewResourceTypeInfo(1, 100)
	currentInstanceID := uint32(100) // Same instance

	// Lower multiple borrows - all should return rep directly
	for i := uint32(0); i < 5; i++ {
		rep := i * 10
		result, err := LowerBorrowWithType(table, callCtx, rep, resourceTypeInfo, currentInstanceID)
		require.NoError(t, err)
		require.Equal(t, rep, result)
	}

	// No borrows should be tracked (same-instance optimization)
	require.Equal(t, 0, callCtx.NumBorrows())
}

func TestLowerBorrowWithType_MultipleBorrowsDifferentInstance(t *testing.T) {
	table := component.NewResourceTable()
	callCtx := component.NewCallContext()

	// Resource type defined in instance 100
	resourceTypeInfo := component.NewResourceTypeInfo(1, 100)
	currentInstanceID := uint32(200) // Different instance

	// Lower multiple borrows - all should create handles
	// Use rep values that won't coincidentally match handle indices
	reps := []uint32{1000, 2000, 3000, 4000, 5000}
	for _, rep := range reps {
		result, err := LowerBorrowWithType(table, callCtx, rep, resourceTypeInfo, currentInstanceID)
		require.NoError(t, err)
		require.NotEqual(t, rep, result, "should not return rep directly")

		// Verify the handle exists and has correct rep
		entry, err := table.Get(component.Handle(result))
		require.NoError(t, err)
		require.Equal(t, rep, entry.Rep.(uint32))
	}

	// All borrows should be tracked
	require.Equal(t, 5, callCtx.NumBorrows())
}

func TestLowerOwnWithType(t *testing.T) {
	table := component.NewResourceTable()

	// Resource type defined in instance 100
	resourceTypeInfo := component.NewResourceTypeInfo(1, 100)

	result, err := LowerOwnWithType(table, 42, resourceTypeInfo)
	require.NoError(t, err)

	// Handle should exist in the table
	entry, err := table.Get(component.Handle(result))
	require.NoError(t, err)
	require.True(t, entry.Own, "should be owned")

	// Rep should be the original value
	rep, ok := entry.Rep.(uint32)
	require.True(t, ok, "Rep should be uint32")
	require.Equal(t, uint32(42), rep)

	// Type should be set correctly
	require.Equal(t, resourceTypeInfo.TypeID(), entry.RT)
}

func TestLowerOwnWithType_MultipleResources(t *testing.T) {
	table := component.NewResourceTable()
	resourceTypeInfo := component.NewResourceTypeInfo(1, 100)

	// Lower multiple resources
	handles := make([]uint32, 3)
	for i := uint32(0); i < 3; i++ {
		h, err := LowerOwnWithType(table, i*10, resourceTypeInfo)
		require.NoError(t, err)
		handles[i] = h
	}

	// All should be distinct handles
	require.NotEqual(t, handles[0], handles[1])
	require.NotEqual(t, handles[1], handles[2])
	require.NotEqual(t, handles[0], handles[2])

	// All should be retrievable with correct values
	for i := uint32(0); i < 3; i++ {
		entry, err := table.Get(component.Handle(handles[i]))
		require.NoError(t, err)
		require.True(t, entry.Own)
		rep, ok := entry.Rep.(uint32)
		require.True(t, ok)
		require.Equal(t, i*10, rep)
	}
}

func TestLowerBorrowWithType_DifferentResourceTypes(t *testing.T) {
	table := component.NewResourceTable()
	callCtx := component.NewCallContext()

	// Two different resource types, both defined in instance 100
	resourceType1 := component.NewResourceTypeInfo(1, 100)
	resourceType2 := component.NewResourceTypeInfo(2, 100)

	currentInstanceID := uint32(200) // Different instance

	// Lower borrows of different types
	result1, err := LowerBorrowWithType(table, callCtx, 42, resourceType1, currentInstanceID)
	require.NoError(t, err)

	result2, err := LowerBorrowWithType(table, callCtx, 42, resourceType2, currentInstanceID)
	require.NoError(t, err)

	// Should be different handles even with same rep
	require.NotEqual(t, result1, result2)

	// Both should be borrows
	entry1, err := table.Get(component.Handle(result1))
	require.NoError(t, err)
	require.False(t, entry1.Own)
	require.Equal(t, resourceType1.TypeID(), entry1.RT)

	entry2, err := table.Get(component.Handle(result2))
	require.NoError(t, err)
	require.False(t, entry2.Own)
	require.Equal(t, resourceType2.TypeID(), entry2.RT)
}
