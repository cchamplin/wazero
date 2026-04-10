// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package conformance: post-return / borrow-lifecycle conformance
// tests. These exercise CallContext's num_borrows gate (the
// canonical-ABI invariant that a call must not return while any
// borrowed handle is still outstanding) and the CallContext lends
// tracking that implements Subtask.lenders from the spec.
package conformance

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestPostReturn_BorrowsReleasedBeforeReturn asserts the fundamental
// canonical-ABI invariant: a call must not return while any borrowed
// handle is still outstanding. The spec encodes this as
// trap_if(self.num_borrows > 0) at definitions.py:690 inside
// Task.return_.
//
// Spec: definitions.py:690 Task.return_ trap_if(self.num_borrows > 0).
// Spec: definitions.py:571 Task field `num_borrows: int`, initialised
// to 0 at definitions.py:581.
func TestPostReturn_BorrowsReleasedBeforeReturn(t *testing.T) {
	ctx := runtime.NewCallContext(nil)

	// Initially, no borrows - can return.
	require.True(t, ctx.CanReturn())
	require.NoError(t, ctx.ValidateReturn())

	// Simulate receiving a borrowed handle.
	ctx.IncrementBorrows()

	// Cannot return with outstanding borrow.
	require.False(t, ctx.CanReturn())
	require.Error(t, ctx.ValidateReturn())
	require.ErrorIs(t, ctx.ValidateReturn(), runtime.ErrOutstandingBorrows)

	// Release the borrow.
	ctx.DecrementBorrows()

	// Now can return.
	require.True(t, ctx.CanReturn())
	require.NoError(t, ctx.ValidateReturn())
}

// TestPostReturn_ValidateReturn exercises ValidateReturn across the
// three borrow-count states (zero, single, multiple) that the spec's
// num_borrows > 0 trap discriminates. Each sub-case mirrors one step
// of the Python model's assert(self.num_borrows == 0) invariant at
// definitions.py:593 (thread_stop) / :690 (Task.return_).
//
// Spec: definitions.py:593 assert(self.num_borrows == 0) on last
// thread_stop. Spec: definitions.py:690 Task.return_
// trap_if(self.num_borrows > 0).
func TestPostReturn_ValidateReturn(t *testing.T) {
	t.Run("fresh context can return", func(t *testing.T) {
		ctx := runtime.NewCallContext(nil)
		err := ctx.ValidateReturn()
		require.NoError(t, err)
	})

	t.Run("single borrow prevents return", func(t *testing.T) {
		ctx := runtime.NewCallContext(nil)
		ctx.IncrementBorrows()

		err := ctx.ValidateReturn()
		require.Error(t, err)
		require.ErrorIs(t, err, runtime.ErrOutstandingBorrows)

		ctx.DecrementBorrows()
		err = ctx.ValidateReturn()
		require.NoError(t, err)
	})

	t.Run("multiple borrows all must be released", func(t *testing.T) {
		ctx := runtime.NewCallContext(nil)
		ctx.IncrementBorrows()
		ctx.IncrementBorrows()
		ctx.IncrementBorrows()

		// All three borrows outstanding.
		require.Equal(t, 3, ctx.NumBorrows())
		require.Error(t, ctx.ValidateReturn())

		// Release one - still cannot return.
		ctx.DecrementBorrows()
		require.Equal(t, 2, ctx.NumBorrows())
		require.Error(t, ctx.ValidateReturn())

		// Release second - still cannot return.
		ctx.DecrementBorrows()
		require.Equal(t, 1, ctx.NumBorrows())
		require.Error(t, ctx.ValidateReturn())

		// Release last - now can return.
		ctx.DecrementBorrows()
		require.Equal(t, 0, ctx.NumBorrows())
		require.NoError(t, ctx.ValidateReturn())
	})
}

// TestPostReturn_CanReturn exercises CanReturn() — the boolean
// counterpart of ValidateReturn() — across the same num_borrows
// transitions as the spec's trap at return_ time.
//
// Spec: definitions.py:690 Task.return_ trap_if(self.num_borrows > 0).
// Spec: definitions.py:697 Task.cancel trap_if(self.num_borrows > 0)
// (the same gate on the cancel path).
func TestPostReturn_CanReturn(t *testing.T) {
	ctx := runtime.NewCallContext(nil)

	// Fresh context.
	require.True(t, ctx.CanReturn())

	// Add borrow.
	ctx.IncrementBorrows()
	require.False(t, ctx.CanReturn())

	// Add another.
	ctx.IncrementBorrows()
	require.False(t, ctx.CanReturn())

	// Release one.
	ctx.DecrementBorrows()
	require.False(t, ctx.CanReturn())

	// Release last.
	ctx.DecrementBorrows()
	require.True(t, ctx.CanReturn())
}

// TestPostReturn_NumBorrowsTracking asserts NumBorrows() accurately
// tracks increment / decrement. This is the instrumentation layer
// behind the return_ trap: every lift_borrow must eventually be
// matched by a drop_borrow.
//
// Spec: definitions.py:571 Task.num_borrows is a plain counter that
// increments on each lift_borrow and decrements on each drop_borrow
// (wired through lower_borrow / drop_handle in the spec).
func TestPostReturn_NumBorrowsTracking(t *testing.T) {
	ctx := runtime.NewCallContext(nil)

	require.Equal(t, 0, ctx.NumBorrows())

	// Increment several times.
	for i := 1; i <= 5; i++ {
		ctx.IncrementBorrows()
		require.Equal(t, i, ctx.NumBorrows())
	}

	// Decrement back to zero.
	for i := 4; i >= 0; i-- {
		ctx.DecrementBorrows()
		require.Equal(t, i, ctx.NumBorrows())
	}
}

// TestPostReturn_MultipleCallContextsIndependent asserts that
// separate CallContext instances maintain independent borrow counts.
// Each active task in the spec has its own Task.num_borrows counter
// (definitions.py:571, per-Task field) — two calls in flight must
// not share the same counter.
//
// Spec: definitions.py:571 Task.num_borrows is a per-Task field; the
// Python model constructs a fresh Task at each canon_lift entry
// (definitions.py:1980).
func TestPostReturn_MultipleCallContextsIndependent(t *testing.T) {
	ctx1 := runtime.NewCallContext(nil)
	ctx2 := runtime.NewCallContext(nil)
	ctx3 := runtime.NewCallContext(nil)

	ctx1.IncrementBorrows()
	ctx1.IncrementBorrows()

	ctx2.IncrementBorrows()

	require.Equal(t, 2, ctx1.NumBorrows())
	require.Equal(t, 1, ctx2.NumBorrows())
	require.Equal(t, 0, ctx3.NumBorrows())

	require.False(t, ctx1.CanReturn())
	require.False(t, ctx2.CanReturn())
	require.True(t, ctx3.CanReturn())
}

// TestPostReturn_SimulateHostCallWithBorrows simulates a host
// function receiving borrowed parameters and properly tracking them.
// Each borrow<T> parameter increments num_borrows at lift time; the
// host must drop (decrement) each before calling task.return_().
//
// Spec: definitions.py:1650 `h.borrow_scope.num_borrows += 1` (borrow
// scope bump inside lift_borrow) + definitions.py:2164
// `h.borrow_scope.num_borrows -= 1` (drop_borrow decrement).
// Canonical test: no direct test_heap case — this is the spec's
// lift_borrow / drop_handle lifecycle exercised end-to-end in
// run_tests.py tests that thread resources through canon.lift/lower.
func TestPostReturn_SimulateHostCallWithBorrows(t *testing.T) {
	// Simulates: func process(a: borrow<X>, b: borrow<Y>) -> s32
	ctx := runtime.NewCallContext(nil)

	// lower_borrow for param 'a'.
	ctx.IncrementBorrows()
	// lower_borrow for param 'b'.
	ctx.IncrementBorrows()

	require.Equal(t, 2, ctx.NumBorrows())
	require.False(t, ctx.CanReturn())

	// Host function does its work...
	//
	// Before returning, host must drop the borrowed handles:
	// resource.drop for borrow 'a'.
	ctx.DecrementBorrows()
	// resource.drop for borrow 'b'.
	ctx.DecrementBorrows()

	// Now can return.
	require.True(t, ctx.CanReturn())
	require.NoError(t, ctx.ValidateReturn())
}

// TestPostReturn_ErrOutstandingBorrows asserts that ValidateReturn
// returns ErrOutstandingBorrows (not some other error) when borrows
// are still outstanding, and that the error message is informative.
// This is the observable "trap" that the spec models as
// trap_if(self.num_borrows > 0) on the return_ path.
//
// Spec: definitions.py:690 Task.return_ trap_if(self.num_borrows > 0).
func TestPostReturn_ErrOutstandingBorrows(t *testing.T) {
	ctx := runtime.NewCallContext(nil)
	ctx.IncrementBorrows()

	err := ctx.ValidateReturn()
	require.Error(t, err)
	require.ErrorIs(t, err, runtime.ErrOutstandingBorrows)

	require.Contains(t, err.Error(), "borrow")
}

// TestPostReturn_IntegrationWithResourceTable exercises the full
// borrow lifecycle: create an owned resource, borrow-lend it via
// CallContext, bump CallContext.num_borrows, verify that Table.Remove
// traps with ErrResourceInUse while the borrow is live, then release
// everything and drop the resource.
//
// Spec: definitions.py:713 Subtask.lenders list (CallContext
// lenders). Spec: definitions.py:736 `self.lenders.append(...)`
// (AddLender). Spec: definitions.py:740-742 release loop that
// decrements lends on every tracked handle.
func TestPostReturn_IntegrationWithResourceTable(t *testing.T) {
	table := runtime.NewTable()
	ctx := runtime.NewCallContext(table)

	// Create an owned resource handle. Post-C3 API uses
	// NewResourceHandle(rep, own, rt) instead of the deleted
	// Table.New helper; rt can be nil when no concrete ResourceType
	// identity check is needed.
	handle, err := table.NewResourceHandle(uint32(77), true, nil)
	require.NoError(t, err)

	// lift_borrow increments lends on the original resource via AddLender.
	err = ctx.AddLender(handle)
	require.NoError(t, err)

	// Call context tracks the borrowed parameter.
	ctx.IncrementBorrows()

	// Verify state: resource is borrowed, call cannot return.
	entry, err := table.GetResourceHandle(handle)
	require.NoError(t, err)
	require.Equal(t, uint32(1), entry.NumLends)
	require.False(t, ctx.CanReturn())

	// Original resource cannot be dropped while borrowed.
	_, err = table.Remove(handle)
	require.ErrorIs(t, err, runtime.ErrResourceInUse)

	// Host function finishes using the borrow.
	ctx.DecrementBorrows()

	// ExitCall cleans up the call context — drops lends back to zero.
	err = ctx.ExitCall()
	require.NoError(t, err)

	// Now everything is clean.
	require.True(t, ctx.CanReturn())
	require.NoError(t, ctx.ValidateReturn())

	// Resource can now be dropped — need to re-create it since Release
	// cleared lends and we still need a handle to remove.
	// Actually, the handle is still in the table (Release only decrements
	// lends, it doesn't remove handles).
	entry, err = table.Remove(handle)
	require.NoError(t, err)
	require.Equal(t, uint32(77), entry.Rep)
}

// TestPostReturn_NestedCalls asserts that nested calls have
// independent CallContexts: an inner call's return gate must not be
// affected by the outer call's outstanding borrows. The spec models
// this with a per-Task num_borrows counter (definitions.py:571), so
// the inner Task's counter starts at 0 regardless of any enclosing
// Task state.
//
// Spec: definitions.py:571 Task.num_borrows is a per-Task field,
// constructed fresh at each canon_lift entry (definitions.py:1980).
func TestPostReturn_NestedCalls(t *testing.T) {
	outerCtx := runtime.NewCallContext(nil)
	outerCtx.IncrementBorrows()

	// Outer call makes a nested call that also receives a borrow.
	innerCtx := runtime.NewCallContext(nil)
	innerCtx.IncrementBorrows()

	// Inner call finishes and releases its borrow.
	innerCtx.DecrementBorrows()
	require.True(t, innerCtx.CanReturn())
	require.NoError(t, innerCtx.ValidateReturn())

	// Outer call still has its borrow.
	require.False(t, outerCtx.CanReturn())
	require.Error(t, outerCtx.ValidateReturn())

	// Outer call finishes and releases its borrow.
	outerCtx.DecrementBorrows()
	require.True(t, outerCtx.CanReturn())
	require.NoError(t, outerCtx.ValidateReturn())
}

// TestPostReturn_ZeroBorrowsIdempotent asserts ValidateReturn is
// idempotent when there are no borrows — each call observes the
// invariant and does not mutate state. The spec's trap_if check
// (definitions.py:690) is pure-read with no side effect on the
// num_borrows counter itself.
//
// Spec: definitions.py:690 Task.return_ trap_if(self.num_borrows > 0)
// is a side-effect-free check.
func TestPostReturn_ZeroBorrowsIdempotent(t *testing.T) {
	ctx := runtime.NewCallContext(nil)

	// Multiple calls to ValidateReturn should all succeed.
	require.NoError(t, ctx.ValidateReturn())
	require.NoError(t, ctx.ValidateReturn())
	require.NoError(t, ctx.ValidateReturn())

	// State should be unchanged.
	require.Equal(t, 0, ctx.NumBorrows())
	require.True(t, ctx.CanReturn())
}

// TestPostReturn_LargeBorrowCount stresses num_borrows under a large
// number of increments, asserting the counter is a plain int with no
// saturation behaviour. The spec uses an unbounded Python int, so
// wazero's implementation must not clamp or overflow for reasonable
// call widths.
//
// Spec: definitions.py:571 Task.num_borrows is `int` (Python
// arbitrary-precision), used as a plain counter.
func TestPostReturn_LargeBorrowCount(t *testing.T) {
	ctx := runtime.NewCallContext(nil)

	const numBorrows = 1000

	for i := 0; i < numBorrows; i++ {
		ctx.IncrementBorrows()
	}

	require.Equal(t, numBorrows, ctx.NumBorrows())
	require.False(t, ctx.CanReturn())

	for i := 0; i < numBorrows; i++ {
		ctx.DecrementBorrows()
	}

	require.Equal(t, 0, ctx.NumBorrows())
	require.True(t, ctx.CanReturn())
	require.NoError(t, ctx.ValidateReturn())
}

// TestPostReturn_ReusableContext asserts a CallContext can be reused
// across multiple logical call cycles: a fully-released counter is
// indistinguishable from a freshly-constructed one. This matches the
// spec's "fresh num_borrows counter per Task" model — the actual
// runtime CallContext is reusable as long as callers observe the
// pre-call invariant num_borrows == 0.
//
// Spec: definitions.py:581 self.num_borrows = 0 (Task.__init__
// establishes the pre-call precondition).
func TestPostReturn_ReusableContext(t *testing.T) {
	ctx := runtime.NewCallContext(nil)

	// First call cycle.
	ctx.IncrementBorrows()
	require.False(t, ctx.CanReturn())
	ctx.DecrementBorrows()
	require.True(t, ctx.CanReturn())
	require.NoError(t, ctx.ValidateReturn())

	// Second call cycle on same context.
	ctx.IncrementBorrows()
	ctx.IncrementBorrows()
	require.Equal(t, 2, ctx.NumBorrows())
	require.False(t, ctx.CanReturn())
	ctx.DecrementBorrows()
	ctx.DecrementBorrows()
	require.True(t, ctx.CanReturn())
	require.NoError(t, ctx.ValidateReturn())

	// Third call cycle.
	ctx.IncrementBorrows()
	require.Equal(t, 1, ctx.NumBorrows())
	ctx.DecrementBorrows()
	require.NoError(t, ctx.ValidateReturn())
}
