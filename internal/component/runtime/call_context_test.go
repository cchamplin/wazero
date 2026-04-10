// internal/component/runtime/call_context_test.go

package runtime

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestCallContext_NumBorrows(t *testing.T) {
	ctx := NewCallContext(nil)

	require.Equal(t, 0, ctx.NumBorrows())

	ctx.IncrementBorrows()
	ctx.IncrementBorrows()
	require.Equal(t, 2, ctx.NumBorrows())

	ctx.DecrementBorrows()
	require.Equal(t, 1, ctx.NumBorrows())
}

func TestCallContext_CanReturn(t *testing.T) {
	ctx := NewCallContext(nil)

	// Can return when no outstanding borrows
	require.True(t, ctx.CanReturn())

	// Cannot return with outstanding borrows
	ctx.IncrementBorrows()
	require.False(t, ctx.CanReturn())

	// Can return after borrows released
	ctx.DecrementBorrows()
	require.True(t, ctx.CanReturn())
}

func TestCallContext_ValidateReturn_Success(t *testing.T) {
	ctx := NewCallContext(nil)
	require.NoError(t, ctx.ValidateReturn())
}

func TestCallContext_ValidateReturn_WithBorrows(t *testing.T) {
	ctx := NewCallContext(nil)
	ctx.IncrementBorrows()

	err := ctx.ValidateReturn()
	require.ErrorIs(t, err, ErrOutstandingBorrows)
}

func TestCallContext_TrackLenders(t *testing.T) {
	table := NewTable()
	// Create resource handles so AddLender can IncrementLends
	h1, err := table.NewResourceHandle(uint32(1), true, nil)
	require.NoError(t, err)
	h2, err := table.NewResourceHandle(uint32(2), true, nil)
	require.NoError(t, err)
	h3, err := table.NewResourceHandle(uint32(3), true, nil)
	require.NoError(t, err)

	ctx := NewCallContext(table)

	require.NoError(t, ctx.AddLender(h1))
	require.NoError(t, ctx.AddLender(h2))
	require.NoError(t, ctx.AddLender(h3))

	lenders := ctx.Lenders()
	require.Equal(t, 3, len(lenders))
	require.Equal(t, h1, lenders[0])
	require.Equal(t, h2, lenders[1])
	require.Equal(t, h3, lenders[2])
}

func TestCallContext_ExitCall_UndoesLends(t *testing.T) {
	table := NewTable()

	// Create a resource
	h, err := table.NewResourceHandle(uint32(10), true, nil)
	require.NoError(t, err)

	// Create call context with table and track lenders
	// AddLender now calls IncrementLends internally.
	ctx := NewCallContext(table)
	require.NoError(t, ctx.AddLender(h))
	require.NoError(t, ctx.AddLender(h)) // Same handle borrowed twice

	// Verify lends are incremented
	entry, _ := table.GetResourceHandle(h)
	require.Equal(t, uint32(2), entry.NumLends)

	// Exit call should undo all lends
	err = ctx.ExitCall()
	require.NoError(t, err)

	// Verify lends are decremented
	entry, _ = table.GetResourceHandle(h)
	require.Equal(t, uint32(0), entry.NumLends)

	// Lenders should be cleared
	require.Equal(t, 0, len(ctx.Lenders()))
}

func TestCallContext_ExitCall_FailsWithOutstandingBorrows(t *testing.T) {
	table := NewTable()

	ctx := NewCallContext(table)
	ctx.IncrementBorrows() // Simulate unreleased borrow

	err := ctx.ExitCall()
	require.ErrorIs(t, err, ErrOutstandingBorrows)
}

// TestCallContext_TrackLenders_WithTable verifies that AddLender
// increments NumLends on the table entry when a table is provided.
// (Migrated from borrow_scope_test.go TestBorrowScope_TrackLenders)
func TestCallContext_TrackLenders_WithTable(t *testing.T) {
	table := NewTable()
	h1, err := table.NewResourceHandle(uint32(1), true, nil)
	require.NoError(t, err)
	h2, err := table.NewResourceHandle(uint32(2), true, nil)
	require.NoError(t, err)

	ctx := NewCallContext(table)

	// Track borrows from two handles
	require.NoError(t, ctx.AddLender(h1))
	require.NoError(t, ctx.AddLender(h2))

	// Both handles should have NumLends incremented
	e1, _ := table.GetResourceHandle(h1)
	e2, _ := table.GetResourceHandle(h2)
	require.Equal(t, uint32(1), e1.NumLends)
	require.Equal(t, uint32(1), e2.NumLends)

	// Release decrements all lends
	require.NoError(t, ctx.ExitCall())

	e1, _ = table.GetResourceHandle(h1)
	e2, _ = table.GetResourceHandle(h2)
	require.Equal(t, uint32(0), e1.NumLends)
	require.Equal(t, uint32(0), e2.NumLends)
}

// TestCallContext_ReleaseBorrow asserts ReleaseBorrow is
// the symmetric inverse of AddLender: it decrements NumLends on the
// handle's entry and removes the handle from the lender set.
// (Migrated from borrow_scope_test.go TestBorrowScopeReleaseBorrow)
func TestCallContext_ReleaseBorrow(t *testing.T) {
	tbl := NewTable()
	rt := &ResourceType{}
	h, err := tbl.NewResourceHandle(uint32(1), true, rt)
	require.NoError(t, err)

	ctx := NewCallContext(tbl)
	// AddLender internally calls IncrementLends.
	require.NoError(t, ctx.AddLender(h))

	if !ctx.HasOutstandingBorrows() {
		t.Fatalf("HasOutstandingBorrows = false, want true")
	}

	require.NoError(t, ctx.ReleaseBorrow(h))

	if ctx.HasOutstandingBorrows() {
		t.Fatalf("HasOutstandingBorrows after release = true, want false")
	}

	entry, err := tbl.GetResourceHandle(h)
	require.NoError(t, err)
	if entry.NumLends != 0 {
		t.Fatalf("entry.NumLends = %d, want 0", entry.NumLends)
	}
}

// TestCallContext_ReleaseBorrowNotFound verifies ReleaseBorrow returns an
// error when the handle is not in the lender set.
// (Migrated from borrow_scope_test.go TestBorrowScopeReleaseBorrowNotFound)
func TestCallContext_ReleaseBorrowNotFound(t *testing.T) {
	tbl := NewTable()
	rt := &ResourceType{}
	h, err := tbl.NewResourceHandle(uint32(1), true, rt)
	require.NoError(t, err)

	ctx := NewCallContext(tbl)
	// Never added — should fail.
	err = ctx.ReleaseBorrow(h)
	if err == nil {
		t.Fatalf("ReleaseBorrow on handle not in context should return error")
	}
}

// TestCallContext_SameLenderMultipleTimes verifies that the same handle
// can be lent multiple times and all lends are cleaned up on Release.
// (Migrated from borrow_scope_test.go TestBorrowScope_SameLenderMultipleTimes)
func TestCallContext_SameLenderMultipleTimes(t *testing.T) {
	table := NewTable()
	h, err := table.NewResourceHandle(uint32(3), true, nil)
	require.NoError(t, err)

	ctx := NewCallContext(table)

	// Same handle borrowed multiple times
	require.NoError(t, ctx.AddLender(h))
	require.NoError(t, ctx.AddLender(h))

	entry, _ := table.GetResourceHandle(h)
	require.Equal(t, uint32(2), entry.NumLends)

	require.NoError(t, ctx.ExitCall())

	entry, _ = table.GetResourceHandle(h)
	require.Equal(t, uint32(0), entry.NumLends)
}
