// internal/component/runtime/call_context_test.go

package runtime

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestCallContext_NumBorrows(t *testing.T) {
	ctx := NewCallContext()

	require.Equal(t, 0, ctx.NumBorrows())

	ctx.IncrementBorrows()
	ctx.IncrementBorrows()
	require.Equal(t, 2, ctx.NumBorrows())

	ctx.DecrementBorrows()
	require.Equal(t, 1, ctx.NumBorrows())
}

func TestCallContext_CanReturn(t *testing.T) {
	ctx := NewCallContext()

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
	ctx := NewCallContext()
	require.NoError(t, ctx.ValidateReturn())
}

func TestCallContext_ValidateReturn_WithBorrows(t *testing.T) {
	ctx := NewCallContext()
	ctx.IncrementBorrows()

	err := ctx.ValidateReturn()
	require.ErrorIs(t, err, ErrOutstandingBorrows)
}

func TestCallContext_TrackLenders(t *testing.T) {
	ctx := NewCallContext()

	// Add some lender handles
	h1 := MakeHandle(1, 0)
	h2 := MakeHandle(2, 0)
	h3 := MakeHandle(3, 0)

	ctx.AddLender(h1)
	ctx.AddLender(h2)
	ctx.AddLender(h3)

	lenders := ctx.Lenders()
	require.Equal(t, 3, len(lenders))
	require.Equal(t, h1, lenders[0])
	require.Equal(t, h2, lenders[1])
	require.Equal(t, h3, lenders[2])
}

func TestCallContext_ClearLenders(t *testing.T) {
	ctx := NewCallContext()

	ctx.AddLender(MakeHandle(1, 0))
	ctx.AddLender(MakeHandle(2, 0))

	require.Equal(t, 2, len(ctx.Lenders()))

	ctx.ClearLenders()
	require.Equal(t, 0, len(ctx.Lenders()))
}

func TestCallContext_ExitCall_UndoesLends(t *testing.T) {
	table := NewTable()

	// Create a resource
	h, err := table.NewResourceHandle(uint32(10), true, nil)
	require.NoError(t, err)

	// Simulate lift_borrow: increment lends on the source handle
	require.NoError(t, table.IncrementLends(h))
	require.NoError(t, table.IncrementLends(h))

	// Verify lends are incremented
	entry, _ := table.GetResourceHandle(h)
	require.Equal(t, uint32(2), entry.NumLends)

	// Create call context and track lenders
	ctx := NewCallContext()
	ctx.AddLender(h)
	ctx.AddLender(h) // Same handle borrowed twice

	// Exit call should undo all lends
	err = ctx.ExitCall(table)
	require.NoError(t, err)

	// Verify lends are decremented
	entry, _ = table.GetResourceHandle(h)
	require.Equal(t, uint32(0), entry.NumLends)

	// Lenders should be cleared
	require.Equal(t, 0, len(ctx.Lenders()))
}

func TestCallContext_ExitCall_FailsWithOutstandingBorrows(t *testing.T) {
	table := NewTable()

	ctx := NewCallContext()
	ctx.IncrementBorrows() // Simulate unreleased borrow

	err := ctx.ExitCall(table)
	require.ErrorIs(t, err, ErrOutstandingBorrows)
}
