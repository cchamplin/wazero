// internal/component/call_context_test.go

package component

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
