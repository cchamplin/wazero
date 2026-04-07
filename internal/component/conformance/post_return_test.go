// Package conformance contains conformance tests for the Component Model implementation.
// This file implements Task 255: Post-Return Hook Tests for borrow/resource handling at call boundaries.
package conformance

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// =============================================================================
// Task 255: Post-Return Hook Tests
// These tests verify the behavior of CallContext for borrow tracking at call boundaries.
// Per the Component Model spec, borrowed handles must be released before a function returns.
// =============================================================================

// TestPostReturn_BorrowsReleasedBeforeReturn tests that borrows must be released
// (numBorrows==0) before returning. This is a fundamental invariant of the Canonical ABI.
func TestPostReturn_BorrowsReleasedBeforeReturn(t *testing.T) {
	ctx := runtime.NewCallContext()

	// Initially, no borrows - can return
	require.True(t, ctx.CanReturn())
	require.NoError(t, ctx.ValidateReturn())

	// Simulate receiving a borrowed handle
	ctx.IncrementBorrows()

	// Cannot return with outstanding borrow
	require.False(t, ctx.CanReturn())
	require.Error(t, ctx.ValidateReturn())
	require.ErrorIs(t, ctx.ValidateReturn(), runtime.ErrOutstandingBorrows)

	// Release the borrow
	ctx.DecrementBorrows()

	// Now can return
	require.True(t, ctx.CanReturn())
	require.NoError(t, ctx.ValidateReturn())
}

// TestPostReturn_ValidateReturn tests the ValidateReturn() behavior with various states.
func TestPostReturn_ValidateReturn(t *testing.T) {
	t.Run("fresh context can return", func(t *testing.T) {
		ctx := runtime.NewCallContext()
		err := ctx.ValidateReturn()
		require.NoError(t, err)
	})

	t.Run("single borrow prevents return", func(t *testing.T) {
		ctx := runtime.NewCallContext()
		ctx.IncrementBorrows()

		err := ctx.ValidateReturn()
		require.Error(t, err)
		require.ErrorIs(t, err, runtime.ErrOutstandingBorrows)

		ctx.DecrementBorrows()
		err = ctx.ValidateReturn()
		require.NoError(t, err)
	})

	t.Run("multiple borrows all must be released", func(t *testing.T) {
		ctx := runtime.NewCallContext()
		ctx.IncrementBorrows()
		ctx.IncrementBorrows()
		ctx.IncrementBorrows()

		// All three borrows outstanding
		require.Equal(t, 3, ctx.NumBorrows())
		require.Error(t, ctx.ValidateReturn())

		// Release one - still cannot return
		ctx.DecrementBorrows()
		require.Equal(t, 2, ctx.NumBorrows())
		require.Error(t, ctx.ValidateReturn())

		// Release second - still cannot return
		ctx.DecrementBorrows()
		require.Equal(t, 1, ctx.NumBorrows())
		require.Error(t, ctx.ValidateReturn())

		// Release last - now can return
		ctx.DecrementBorrows()
		require.Equal(t, 0, ctx.NumBorrows())
		require.NoError(t, ctx.ValidateReturn())
	})
}

// TestPostReturn_CanReturn tests the CanReturn() method which is the boolean
// equivalent of ValidateReturn().
func TestPostReturn_CanReturn(t *testing.T) {
	ctx := runtime.NewCallContext()

	// Fresh context
	require.True(t, ctx.CanReturn())

	// Add borrow
	ctx.IncrementBorrows()
	require.False(t, ctx.CanReturn())

	// Add another
	ctx.IncrementBorrows()
	require.False(t, ctx.CanReturn())

	// Release one
	ctx.DecrementBorrows()
	require.False(t, ctx.CanReturn())

	// Release last
	ctx.DecrementBorrows()
	require.True(t, ctx.CanReturn())
}

// TestPostReturn_NumBorrowsTracking tests that NumBorrows() accurately tracks the count.
func TestPostReturn_NumBorrowsTracking(t *testing.T) {
	ctx := runtime.NewCallContext()

	require.Equal(t, 0, ctx.NumBorrows())

	// Increment several times
	for i := 1; i <= 5; i++ {
		ctx.IncrementBorrows()
		require.Equal(t, i, ctx.NumBorrows())
	}

	// Decrement back to zero
	for i := 4; i >= 0; i-- {
		ctx.DecrementBorrows()
		require.Equal(t, i, ctx.NumBorrows())
	}
}

// TestPostReturn_MultipleCallContextsIndependent tests that separate call contexts
// maintain independent borrow counts.
func TestPostReturn_MultipleCallContextsIndependent(t *testing.T) {
	ctx1 := runtime.NewCallContext()
	ctx2 := runtime.NewCallContext()
	ctx3 := runtime.NewCallContext()

	// Add borrows to ctx1 only
	ctx1.IncrementBorrows()
	ctx1.IncrementBorrows()

	// Add one borrow to ctx2
	ctx2.IncrementBorrows()

	// ctx3 has no borrows

	// Check they're independent
	require.Equal(t, 2, ctx1.NumBorrows())
	require.Equal(t, 1, ctx2.NumBorrows())
	require.Equal(t, 0, ctx3.NumBorrows())

	require.False(t, ctx1.CanReturn())
	require.False(t, ctx2.CanReturn())
	require.True(t, ctx3.CanReturn())
}

// TestPostReturn_SimulateHostCallWithBorrows simulates a host function receiving
// borrowed parameters and properly tracking them.
func TestPostReturn_SimulateHostCallWithBorrows(t *testing.T) {
	// This simulates what happens when a guest calls a host function
	// with borrow<T> parameters

	// Create call context for this call
	ctx := runtime.NewCallContext()

	// Simulate: func process(a: borrow<X>, b: borrow<Y>) -> s32
	// Two borrowed parameters means two borrows to track

	// lower_borrow for param 'a'
	ctx.IncrementBorrows()
	// lower_borrow for param 'b'
	ctx.IncrementBorrows()

	require.Equal(t, 2, ctx.NumBorrows())
	require.False(t, ctx.CanReturn())

	// Host function does its work...
	// ...

	// Before returning, host must drop the borrowed handles
	// resource.drop for borrow 'a'
	ctx.DecrementBorrows()
	// resource.drop for borrow 'b'
	ctx.DecrementBorrows()

	// Now can return
	require.True(t, ctx.CanReturn())
	require.NoError(t, ctx.ValidateReturn())
}

// TestPostReturn_ErrOutstandingBorrows tests the error type and message.
func TestPostReturn_ErrOutstandingBorrows(t *testing.T) {
	ctx := runtime.NewCallContext()
	ctx.IncrementBorrows()

	err := ctx.ValidateReturn()
	require.Error(t, err)
	require.ErrorIs(t, err, runtime.ErrOutstandingBorrows)

	// Check the error message is informative
	require.Contains(t, err.Error(), "borrow")
}

// TestPostReturn_IntegrationWithResourceTable tests CallContext integration with
// the ResourceTable for a complete borrow lifecycle.
func TestPostReturn_IntegrationWithResourceTable(t *testing.T) {
	// This test shows how CallContext works together with ResourceTable
	// in a realistic scenario

	table := runtime.NewResourceTable()
	ctx := runtime.NewCallContext()

	// Create an owned resource
	handle := table.New("my-resource-data", true)

	// Create a borrow scope for tracking lends
	scope := runtime.NewBorrowScope(table)

	// Simulate: guest passes borrow<T> to host
	// lift_borrow increments lends on the original resource
	err := scope.AddLender(handle)
	require.NoError(t, err)

	// Call context tracks the borrowed parameter
	ctx.IncrementBorrows()

	// Verify state: resource is borrowed, call cannot return
	entry, err := table.Get(handle)
	require.NoError(t, err)
	require.Equal(t, uint32(1), entry.NumLends)
	require.False(t, ctx.CanReturn())

	// Original resource cannot be dropped while borrowed
	_, err = table.Remove(handle)
	require.ErrorIs(t, err, runtime.ErrResourceInUse)

	// Host function finishes using the borrow
	ctx.DecrementBorrows()

	// Release the borrow scope
	err = scope.Release()
	require.NoError(t, err)

	// Now everything is clean
	require.True(t, ctx.CanReturn())
	require.NoError(t, ctx.ValidateReturn())

	// Resource can now be dropped
	entry, err = table.Remove(handle)
	require.NoError(t, err)
	require.Equal(t, "my-resource-data", entry.Rep)
}

// TestPostReturn_NestedCalls tests that nested calls have independent contexts.
func TestPostReturn_NestedCalls(t *testing.T) {
	// Outer call receives a borrow
	outerCtx := runtime.NewCallContext()
	outerCtx.IncrementBorrows()

	// Outer call makes a nested call that also receives a borrow
	innerCtx := runtime.NewCallContext()
	innerCtx.IncrementBorrows()

	// Inner call finishes and releases its borrow
	innerCtx.DecrementBorrows()
	require.True(t, innerCtx.CanReturn())
	require.NoError(t, innerCtx.ValidateReturn())

	// Outer call still has its borrow
	require.False(t, outerCtx.CanReturn())
	require.Error(t, outerCtx.ValidateReturn())

	// Outer call finishes and releases its borrow
	outerCtx.DecrementBorrows()
	require.True(t, outerCtx.CanReturn())
	require.NoError(t, outerCtx.ValidateReturn())
}

// TestPostReturn_ZeroBorrowsIdempotent tests that ValidateReturn is idempotent
// when there are no borrows.
func TestPostReturn_ZeroBorrowsIdempotent(t *testing.T) {
	ctx := runtime.NewCallContext()

	// Multiple calls to ValidateReturn should all succeed
	require.NoError(t, ctx.ValidateReturn())
	require.NoError(t, ctx.ValidateReturn())
	require.NoError(t, ctx.ValidateReturn())

	// State should be unchanged
	require.Equal(t, 0, ctx.NumBorrows())
	require.True(t, ctx.CanReturn())
}

// TestPostReturn_LargeBorrowCount tests handling a large number of borrows.
func TestPostReturn_LargeBorrowCount(t *testing.T) {
	ctx := runtime.NewCallContext()

	const numBorrows = 1000

	// Add many borrows
	for i := 0; i < numBorrows; i++ {
		ctx.IncrementBorrows()
	}

	require.Equal(t, numBorrows, ctx.NumBorrows())
	require.False(t, ctx.CanReturn())

	// Release all borrows
	for i := 0; i < numBorrows; i++ {
		ctx.DecrementBorrows()
	}

	require.Equal(t, 0, ctx.NumBorrows())
	require.True(t, ctx.CanReturn())
	require.NoError(t, ctx.ValidateReturn())
}

// TestPostReturn_ReusableContext tests that a context can be reused across
// multiple logical call cycles.
func TestPostReturn_ReusableContext(t *testing.T) {
	ctx := runtime.NewCallContext()

	// First call cycle
	ctx.IncrementBorrows()
	require.False(t, ctx.CanReturn())
	ctx.DecrementBorrows()
	require.True(t, ctx.CanReturn())
	require.NoError(t, ctx.ValidateReturn())

	// Second call cycle on same context
	ctx.IncrementBorrows()
	ctx.IncrementBorrows()
	require.Equal(t, 2, ctx.NumBorrows())
	require.False(t, ctx.CanReturn())
	ctx.DecrementBorrows()
	ctx.DecrementBorrows()
	require.True(t, ctx.CanReturn())
	require.NoError(t, ctx.ValidateReturn())

	// Third call cycle
	ctx.IncrementBorrows()
	require.Equal(t, 1, ctx.NumBorrows())
	ctx.DecrementBorrows()
	require.NoError(t, ctx.ValidateReturn())
}
