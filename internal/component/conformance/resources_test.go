// Package conformance contains conformance tests for the Component Model implementation.
// This file implements Tasks 248-250: Resource Lifecycle, Borrow Scope, and Call Context Tests.
// Ported from wasmtime tests/all/component_model/resources.rs
package conformance

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// =============================================================================
// Task 248: Resource Lifecycle Tests
// =============================================================================

// TestResourceLifecycle_DropTwice tests that dropping a resource twice should error.
// This is the "drop host resource twice" test from wasmtime.
func TestResourceLifecycle_DropTwice(t *testing.T) {
	table := runtime.NewResourceTable()

	// Create an owned resource
	h := table.New("my-resource", true)

	// First drop succeeds
	entry, err := table.Remove(h)
	require.NoError(t, err)
	require.Equal(t, "my-resource", entry.Rep)
	require.True(t, entry.Own)

	// Second drop should error (resource already freed)
	_, err = table.Remove(h)
	require.Error(t, err)
	require.ErrorIs(t, err, runtime.ErrInvalidHandle)
}

// TestResourceLifecycle_DropGuestResourceTwice tests double drop for guest resources.
// Guest resources behave the same as host resources for drop semantics.
func TestResourceLifecycle_DropGuestResourceTwice(t *testing.T) {
	table := runtime.NewResourceTable()

	// Simulate a guest resource (owned)
	h := table.New(42, true) // Using an int as the rep

	// First drop succeeds
	_, err := table.Remove(h)
	require.NoError(t, err)

	// Second drop fails
	_, err = table.Remove(h)
	require.Error(t, err)
}

// TestResourceLifecycle_TypeIdentity tests that different resource types produce different handles.
// Even with the same rep value, different resources should have distinct handles.
func TestResourceLifecycle_TypeIdentity(t *testing.T) {
	// Each resource type has its own table in a real component
	table1 := runtime.NewResourceTable() // Resource type A
	table2 := runtime.NewResourceTable() // Resource type B

	// Create resources with the same representation in different tables
	h1 := table1.New("same-rep", true)
	h2 := table2.New("same-rep", true)

	// The handles are independent - same index but different tables
	require.Equal(t, h1.Index(), h2.Index()) // Both get index 0

	// Operations on one table don't affect the other
	_, err := table1.Remove(h1)
	require.NoError(t, err)

	// h2 in table2 is still valid
	entry, err := table2.Get(h2)
	require.NoError(t, err)
	require.Equal(t, "same-rep", entry.Rep)
}

// TestResourceLifecycle_GenerationWrap tests that generation counting works correctly
// when creating and dropping many resources. This verifies use-after-free protection.
func TestResourceLifecycle_GenerationWrap(t *testing.T) {
	table := runtime.NewResourceTable()

	// Create and drop many resources to cycle through generations
	const iterations = 100
	var lastHandle runtime.Handle

	for i := 0; i < iterations; i++ {
		h := table.New(i, true)

		// After first iteration, should reuse index 0
		if i > 0 {
			require.Equal(t, uint32(0), h.Index(), "should reuse slot 0")
			require.Equal(t, uint32(i), h.Generation(), "generation should increment")
		}

		lastHandle = h
		_, err := table.Remove(h)
		require.NoError(t, err)
	}

	// Old handles with stale generations should fail
	staleHandle := runtime.MakeHandle(0, 0)
	_, err := table.Get(staleHandle)
	require.Error(t, err, "stale generation should fail")

	// The last handle should also fail (it was removed)
	_, err = table.Get(lastHandle)
	require.Error(t, err, "removed handle should fail")
}

// TestResourceLifecycle_ActiveBorrowsPreventDrop tests that you cannot drop a resource
// while it has active borrows.
func TestResourceLifecycle_ActiveBorrowsPreventDrop(t *testing.T) {
	table := runtime.NewResourceTable()
	h := table.New("resource", true)

	// Create a borrow (increment lends)
	err := table.IncrementLends(h)
	require.NoError(t, err)

	// Cannot drop while borrowed
	_, err = table.Remove(h)
	require.ErrorIs(t, err, runtime.ErrResourceInUse)

	// Release the borrow
	err = table.DecrementLends(h)
	require.NoError(t, err)

	// Now can drop
	entry, err := table.Remove(h)
	require.NoError(t, err)
	require.Equal(t, "resource", entry.Rep)
}

// TestResourceLifecycle_MultipleBorrows tests tracking multiple concurrent borrows.
func TestResourceLifecycle_MultipleBorrows(t *testing.T) {
	table := runtime.NewResourceTable()
	h := table.New("resource", true)

	// Create multiple borrows
	for i := 0; i < 5; i++ {
		err := table.IncrementLends(h)
		require.NoError(t, err)
	}

	// Verify borrow count
	entry, err := table.Get(h)
	require.NoError(t, err)
	require.Equal(t, uint32(5), entry.NumLends)

	// Cannot drop with borrows
	_, err = table.Remove(h)
	require.ErrorIs(t, err, runtime.ErrResourceInUse)

	// Release all borrows one by one
	for i := 0; i < 5; i++ {
		err = table.DecrementLends(h)
		require.NoError(t, err)
	}

	// Now can drop
	_, err = table.Remove(h)
	require.NoError(t, err)
}

// TestResourceLifecycle_CannotUseBorrowForOwn tests that a borrowed handle
// is not an owned handle - they have different semantics.
func TestResourceLifecycle_CannotUseBorrowForOwn(t *testing.T) {
	table := runtime.NewResourceTable()

	// Create an owned handle
	ownedHandle := table.New("resource", true)

	// Create a borrowed handle (own=false)
	borrowedHandle := table.New("resource", false)

	// Verify ownership flags
	ownedEntry, err := table.Get(ownedHandle)
	require.NoError(t, err)
	require.True(t, ownedEntry.Own)

	borrowedEntry, err := table.Get(borrowedHandle)
	require.NoError(t, err)
	require.False(t, borrowedEntry.Own)

	// Both can be removed, but only owned should trigger destructor
	entry1, err := table.Remove(borrowedHandle)
	require.NoError(t, err)
	require.False(t, entry1.Own, "borrowed handle should not be owned")

	entry2, err := table.Remove(ownedHandle)
	require.NoError(t, err)
	require.True(t, entry2.Own, "owned handle should be owned")
}

// TestResourceLifecycle_CanUseOwnForBorrow tests that an owned handle can satisfy
// a borrow parameter. The owner keeps ownership but allows temporary borrowing.
func TestResourceLifecycle_CanUseOwnForBorrow(t *testing.T) {
	table := runtime.NewResourceTable()

	// Create an owned resource
	ownedHandle := table.New("resource", true)

	// Borrow from the owned handle (this is what lift_borrow does)
	err := table.IncrementLends(ownedHandle)
	require.NoError(t, err)

	// The owned handle is still valid and owned
	entry, err := table.Get(ownedHandle)
	require.NoError(t, err)
	require.True(t, entry.Own)
	require.Equal(t, uint32(1), entry.NumLends)

	// Cannot drop while borrowed
	_, err = table.Remove(ownedHandle)
	require.ErrorIs(t, err, runtime.ErrResourceInUse)

	// End the borrow
	err = table.DecrementLends(ownedHandle)
	require.NoError(t, err)

	// Now can use the owned handle (drop it)
	_, err = table.Remove(ownedHandle)
	require.NoError(t, err)
}

// TestResourceLifecycle_HandleComponents tests handle index and generation accessors.
func TestResourceLifecycle_HandleComponents(t *testing.T) {
	tests := []struct {
		idx uint32
		gen uint32
	}{
		{0, 0},
		{1, 0},
		{0, 1},
		{100, 50},
		{0xFFFFFFFF, 0xFFFFFFFF},
		{0x12345678, 0xABCDEF00},
	}

	for _, tc := range tests {
		h := runtime.MakeHandle(tc.idx, tc.gen)
		require.Equal(t, tc.idx, h.Index())
		require.Equal(t, tc.gen, h.Generation())
	}
}

// TestResourceLifecycle_SlotReuse tests that slots are properly reused in LIFO order.
func TestResourceLifecycle_SlotReuse(t *testing.T) {
	table := runtime.NewResourceTable()

	// Create three resources
	h0 := table.New("r0", true)
	h1 := table.New("r1", true)
	h2 := table.New("r2", true)

	require.Equal(t, uint32(0), h0.Index())
	require.Equal(t, uint32(1), h1.Index())
	require.Equal(t, uint32(2), h2.Index())

	// Drop in order: 1, 0, 2
	_, _ = table.Remove(h1)
	_, _ = table.Remove(h0)
	_, _ = table.Remove(h2)

	// Create new resources - should reuse in LIFO order (2, 0, 1)
	h3 := table.New("r3", true)
	h4 := table.New("r4", true)
	h5 := table.New("r5", true)

	require.Equal(t, uint32(2), h3.Index(), "should reuse slot 2 first")
	require.Equal(t, uint32(0), h4.Index(), "should reuse slot 0 second")
	require.Equal(t, uint32(1), h5.Index(), "should reuse slot 1 third")

	// Generations should be incremented
	require.Equal(t, uint32(1), h3.Generation())
	require.Equal(t, uint32(1), h4.Generation())
	require.Equal(t, uint32(1), h5.Generation())
}

// =============================================================================
// Task 249: Borrow Scope Tests
// =============================================================================

// TestBorrowScope_TrackLenders tests that a scope tracks which handles are borrowed.
func TestBorrowScope_TrackLenders(t *testing.T) {
	table := runtime.NewResourceTable()
	h1 := table.New("resource1", true)
	h2 := table.New("resource2", true)
	h3 := table.New("resource3", true)

	scope := runtime.NewBorrowScope(table)

	// Initially no outstanding borrows
	require.False(t, scope.HasOutstandingBorrows())

	// Add lenders
	require.NoError(t, scope.AddLender(h1))
	require.NoError(t, scope.AddLender(h2))
	require.NoError(t, scope.AddLender(h3))

	// Scope has outstanding borrows
	require.True(t, scope.HasOutstandingBorrows())

	// All handles have incremented lend counts
	e1, _ := table.Get(h1)
	e2, _ := table.Get(h2)
	e3, _ := table.Get(h3)
	require.Equal(t, uint32(1), e1.NumLends)
	require.Equal(t, uint32(1), e2.NumLends)
	require.Equal(t, uint32(1), e3.NumLends)
}

// TestBorrowScope_EndScopeReturnsTrackedBorrows tests that ending a scope releases all borrows.
func TestBorrowScope_EndScopeReturnsTrackedBorrows(t *testing.T) {
	table := runtime.NewResourceTable()
	h1 := table.New("resource1", true)
	h2 := table.New("resource2", true)

	scope := runtime.NewBorrowScope(table)

	// Borrow from both handles
	require.NoError(t, scope.AddLender(h1))
	require.NoError(t, scope.AddLender(h2))

	// Verify borrows are active
	e1, _ := table.Get(h1)
	e2, _ := table.Get(h2)
	require.Equal(t, uint32(1), e1.NumLends)
	require.Equal(t, uint32(1), e2.NumLends)

	// End the scope
	require.NoError(t, scope.Release())

	// All borrows should be released
	e1, _ = table.Get(h1)
	e2, _ = table.Get(h2)
	require.Equal(t, uint32(0), e1.NumLends)
	require.Equal(t, uint32(0), e2.NumLends)

	// Scope should be empty now
	require.False(t, scope.HasOutstandingBorrows())
}

// TestBorrowScope_EmptyScope tests that an empty scope behaves correctly.
func TestBorrowScope_EmptyScope(t *testing.T) {
	table := runtime.NewResourceTable()
	scope := runtime.NewBorrowScope(table)

	// Empty scope has no outstanding borrows
	require.False(t, scope.HasOutstandingBorrows())

	// Release on empty scope should succeed
	require.NoError(t, scope.Release())

	// Still no outstanding borrows
	require.False(t, scope.HasOutstandingBorrows())
}

// TestBorrowScope_NestedScopesWorkIndependently tests that nested scopes work independently.
func TestBorrowScope_NestedScopesWorkIndependently(t *testing.T) {
	table := runtime.NewResourceTable()
	h := table.New("resource", true)

	// Create outer scope
	outerScope := runtime.NewBorrowScope(table)
	require.NoError(t, outerScope.AddLender(h))

	// Create inner scope
	innerScope := runtime.NewBorrowScope(table)
	require.NoError(t, innerScope.AddLender(h))

	// Handle has 2 borrows now
	entry, _ := table.Get(h)
	require.Equal(t, uint32(2), entry.NumLends)

	// Release inner scope
	require.NoError(t, innerScope.Release())

	// Handle still has 1 borrow from outer scope
	entry, _ = table.Get(h)
	require.Equal(t, uint32(1), entry.NumLends)

	// Release outer scope
	require.NoError(t, outerScope.Release())

	// Handle is now free
	entry, _ = table.Get(h)
	require.Equal(t, uint32(0), entry.NumLends)
}

// TestBorrowScope_MultipleBorrowsSameHandle tests borrowing the same handle multiple times.
func TestBorrowScope_MultipleBorrowsSameHandle(t *testing.T) {
	table := runtime.NewResourceTable()
	h := table.New("resource", true)

	scope := runtime.NewBorrowScope(table)

	// Borrow the same handle multiple times within one scope
	require.NoError(t, scope.AddLender(h))
	require.NoError(t, scope.AddLender(h))
	require.NoError(t, scope.AddLender(h))

	// Handle should have 3 borrows
	entry, _ := table.Get(h)
	require.Equal(t, uint32(3), entry.NumLends)

	// Release scope - all borrows from this scope released
	require.NoError(t, scope.Release())

	// Handle is now free
	entry, _ = table.Get(h)
	require.Equal(t, uint32(0), entry.NumLends)
}

// TestBorrowScope_AddLenderInvalidHandle tests adding an invalid handle as a lender.
func TestBorrowScope_AddLenderInvalidHandle(t *testing.T) {
	table := runtime.NewResourceTable()
	scope := runtime.NewBorrowScope(table)

	// Try to add a non-existent handle
	invalidHandle := runtime.MakeHandle(999, 0)
	err := scope.AddLender(invalidHandle)
	require.Error(t, err)
	require.ErrorIs(t, err, runtime.ErrInvalidHandle)
}

// TestBorrowScope_AddLenderAfterRelease tests adding a lender after release.
func TestBorrowScope_AddLenderAfterRelease(t *testing.T) {
	table := runtime.NewResourceTable()
	h := table.New("resource", true)

	scope := runtime.NewBorrowScope(table)
	require.NoError(t, scope.AddLender(h))
	require.NoError(t, scope.Release())

	// Scope is now empty, can add new lenders
	require.NoError(t, scope.AddLender(h))
	require.True(t, scope.HasOutstandingBorrows())

	entry, _ := table.Get(h)
	require.Equal(t, uint32(1), entry.NumLends)

	// Clean up
	require.NoError(t, scope.Release())
}

// TestBorrowScope_InteractionWithRemove tests that borrowed handles cannot be removed.
func TestBorrowScope_InteractionWithRemove(t *testing.T) {
	table := runtime.NewResourceTable()
	h := table.New("resource", true)

	scope := runtime.NewBorrowScope(table)
	require.NoError(t, scope.AddLender(h))

	// Cannot remove while borrowed
	_, err := table.Remove(h)
	require.ErrorIs(t, err, runtime.ErrResourceInUse)

	// After release, can remove
	require.NoError(t, scope.Release())
	_, err = table.Remove(h)
	require.NoError(t, err)
}

// =============================================================================
// Task 250: Call Context Tests
// =============================================================================

// TestCallContext_EnterExitCallTracksBorrows tests that entering and exiting a call
// properly tracks borrowed handles.
func TestCallContext_EnterExitCallTracksBorrows(t *testing.T) {
	ctx := runtime.NewCallContext()

	// Initially no borrows
	require.Equal(t, 0, ctx.NumBorrows())
	require.True(t, ctx.CanReturn())

	// "Enter" call by receiving borrowed handles
	ctx.IncrementBorrows()
	require.Equal(t, 1, ctx.NumBorrows())
	require.False(t, ctx.CanReturn())

	ctx.IncrementBorrows()
	require.Equal(t, 2, ctx.NumBorrows())

	// "Exit" by releasing borrows
	ctx.DecrementBorrows()
	ctx.DecrementBorrows()
	require.Equal(t, 0, ctx.NumBorrows())
	require.True(t, ctx.CanReturn())
}

// TestCallContext_ExitReleasesAllBorrows simulates releasing all borrows on exit.
func TestCallContext_ExitReleasesAllBorrows(t *testing.T) {
	ctx := runtime.NewCallContext()

	// Receive multiple borrowed handles
	ctx.IncrementBorrows()
	ctx.IncrementBorrows()
	ctx.IncrementBorrows()
	ctx.IncrementBorrows()

	require.Equal(t, 4, ctx.NumBorrows())

	// Simulate dropping all borrowed handles before return
	for ctx.NumBorrows() > 0 {
		ctx.DecrementBorrows()
	}

	require.True(t, ctx.CanReturn())
}

// TestCallContext_MultipleBorrowsInSingleCall tests tracking multiple borrows in one call.
func TestCallContext_MultipleBorrowsInSingleCall(t *testing.T) {
	ctx := runtime.NewCallContext()

	// A function that receives multiple borrow parameters
	// e.g., func(a: borrow<X>, b: borrow<Y>, c: borrow<Z>)
	numParams := 5
	for i := 0; i < numParams; i++ {
		ctx.IncrementBorrows()
	}

	require.Equal(t, numParams, ctx.NumBorrows())
	require.False(t, ctx.CanReturn())

	// Must drop all before returning
	for i := 0; i < numParams; i++ {
		ctx.DecrementBorrows()
	}

	require.True(t, ctx.CanReturn())
	require.NoError(t, ctx.ValidateReturn())
}

// TestCallContext_ValidatesReturn_ErrorsIfBorrowsActive tests that ValidateReturn
// errors if borrows are still active.
func TestCallContext_ValidatesReturn_ErrorsIfBorrowsActive(t *testing.T) {
	ctx := runtime.NewCallContext()

	// Receive a borrowed handle
	ctx.IncrementBorrows()

	// ValidateReturn should error
	err := ctx.ValidateReturn()
	require.Error(t, err)
	require.ErrorIs(t, err, runtime.ErrOutstandingBorrows)

	// After releasing the borrow, ValidateReturn succeeds
	ctx.DecrementBorrows()
	err = ctx.ValidateReturn()
	require.NoError(t, err)
}

// TestCallContext_ValidatesReturn_Success tests successful return validation.
func TestCallContext_ValidatesReturn_Success(t *testing.T) {
	ctx := runtime.NewCallContext()

	// No borrows - can return
	require.NoError(t, ctx.ValidateReturn())

	// Add and remove borrows
	ctx.IncrementBorrows()
	ctx.IncrementBorrows()
	ctx.DecrementBorrows()
	ctx.DecrementBorrows()

	// Still can return
	require.NoError(t, ctx.ValidateReturn())
}

// TestCallContext_FreshContextCanReturn tests that a fresh context can return immediately.
func TestCallContext_FreshContextCanReturn(t *testing.T) {
	ctx := runtime.NewCallContext()
	require.True(t, ctx.CanReturn())
	require.NoError(t, ctx.ValidateReturn())
}

// TestCallContext_MultipleCallContexts tests that multiple call contexts are independent.
func TestCallContext_MultipleCallContexts(t *testing.T) {
	ctx1 := runtime.NewCallContext()
	ctx2 := runtime.NewCallContext()

	// Add borrows to ctx1
	ctx1.IncrementBorrows()
	ctx1.IncrementBorrows()

	// ctx2 is unaffected
	require.Equal(t, 2, ctx1.NumBorrows())
	require.Equal(t, 0, ctx2.NumBorrows())

	require.False(t, ctx1.CanReturn())
	require.True(t, ctx2.CanReturn())
}

// =============================================================================
// Integration Tests
// =============================================================================

// TestResourceIntegration_FullLifecycle tests a complete resource lifecycle.
func TestResourceIntegration_FullLifecycle(t *testing.T) {
	// Simulate a component with a resource table
	table := runtime.NewResourceTable()

	// Host creates a resource
	rep := "host-resource-data"
	handle := table.New(rep, true)

	// Pass to guest via borrow (create borrow scope)
	scope := runtime.NewBorrowScope(table)
	require.NoError(t, scope.AddLender(handle))

	// Guest has a call context tracking the borrowed handle
	ctx := runtime.NewCallContext()
	ctx.IncrementBorrows()

	// Guest cannot return until it drops the borrow
	require.False(t, ctx.CanReturn())

	// Host cannot drop the resource while borrowed
	_, err := table.Remove(handle)
	require.ErrorIs(t, err, runtime.ErrResourceInUse)

	// Guest finishes using the borrow
	ctx.DecrementBorrows()
	require.True(t, ctx.CanReturn())

	// End the borrow scope
	require.NoError(t, scope.Release())

	// Now host can drop the resource
	entry, err := table.Remove(handle)
	require.NoError(t, err)
	require.Equal(t, rep, entry.Rep)
	require.True(t, entry.Own)
}

// TestResourceIntegration_GuestToHostCall simulates a guest calling a host function.
func TestResourceIntegration_GuestToHostCall(t *testing.T) {
	// Guest's resource table
	guestTable := runtime.NewResourceTable()

	// Guest creates a resource
	guestHandle := guestTable.New("guest-resource", true)

	// Guest wants to call host function with a borrow parameter
	// lift_borrow increments lends
	require.NoError(t, guestTable.IncrementLends(guestHandle))

	// Host receives the borrow and tracks it in call context
	hostCtx := runtime.NewCallContext()
	hostCtx.IncrementBorrows()

	// Host does some work with the borrowed resource...
	entry, _ := guestTable.Get(guestHandle)
	require.Equal(t, "guest-resource", entry.Rep)

	// Host finishes and drops the borrow
	hostCtx.DecrementBorrows()
	require.NoError(t, hostCtx.ValidateReturn())

	// Decrement lends when call returns
	require.NoError(t, guestTable.DecrementLends(guestHandle))

	// Guest can now drop its owned resource
	_, err := guestTable.Remove(guestHandle)
	require.NoError(t, err)
}

// TestResourceIntegration_OwnershipTransfer tests transferring ownership of a resource.
func TestResourceIntegration_OwnershipTransfer(t *testing.T) {
	// Source component's table
	sourceTable := runtime.NewResourceTable()

	// Create resource in source
	sourceHandle := sourceTable.New("transferable-resource", true)

	// Transfer ownership: lift_own removes from source table
	entry, err := sourceTable.Remove(sourceHandle)
	require.NoError(t, err)
	require.True(t, entry.Own)

	// The representation is passed to the destination
	rep := entry.Rep

	// Destination component creates a new handle for the resource
	destTable := runtime.NewResourceTable()
	destHandle := destTable.New(rep, true)

	// Destination now owns the resource
	destEntry, err := destTable.Get(destHandle)
	require.NoError(t, err)
	require.Equal(t, "transferable-resource", destEntry.Rep)
	require.True(t, destEntry.Own)

	// Source can no longer access the resource
	_, err = sourceTable.Get(sourceHandle)
	require.Error(t, err)
}

// TestResourceIntegration_ConcurrentBorrows tests multiple concurrent borrows.
func TestResourceIntegration_ConcurrentBorrows(t *testing.T) {
	table := runtime.NewResourceTable()
	handle := table.New("shared-resource", true)

	// Multiple call scopes can borrow the same resource
	scope1 := runtime.NewBorrowScope(table)
	scope2 := runtime.NewBorrowScope(table)
	scope3 := runtime.NewBorrowScope(table)

	require.NoError(t, scope1.AddLender(handle))
	require.NoError(t, scope2.AddLender(handle))
	require.NoError(t, scope3.AddLender(handle))

	// Resource has 3 outstanding borrows
	entry, _ := table.Get(handle)
	require.Equal(t, uint32(3), entry.NumLends)

	// Cannot drop while any scope has it borrowed
	_, err := table.Remove(handle)
	require.ErrorIs(t, err, runtime.ErrResourceInUse)

	// Release scopes in any order
	require.NoError(t, scope2.Release())
	require.NoError(t, scope1.Release())
	require.NoError(t, scope3.Release())

	// Now can drop
	_, err = table.Remove(handle)
	require.NoError(t, err)
}
