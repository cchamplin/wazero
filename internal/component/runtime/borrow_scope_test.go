// internal/component/runtime/borrow_scope_test.go

package runtime

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestBorrowScope_TrackLenders(t *testing.T) {
	table := NewTable()
	h1, err := table.NewResourceHandle("resource1", true, nil)
	require.NoError(t, err)
	h2, err := table.NewResourceHandle("resource2", true, nil)
	require.NoError(t, err)

	scope := NewBorrowScope(table)

	// Track borrows from two handles
	require.NoError(t, scope.AddLender(h1))
	require.NoError(t, scope.AddLender(h2))

	// Both handles should have NumLends incremented
	e1, _ := table.GetResourceHandle(h1)
	e2, _ := table.GetResourceHandle(h2)
	require.Equal(t, uint32(1), e1.NumLends)
	require.Equal(t, uint32(1), e2.NumLends)

	// End scope releases all lends
	require.NoError(t, scope.Release())

	e1, _ = table.GetResourceHandle(h1)
	e2, _ = table.GetResourceHandle(h2)
	require.Equal(t, uint32(0), e1.NumLends)
	require.Equal(t, uint32(0), e2.NumLends)
}

func TestBorrowScope_SameLenderMultipleTimes(t *testing.T) {
	table := NewTable()
	h, err := table.NewResourceHandle("resource", true, nil)
	require.NoError(t, err)

	scope := NewBorrowScope(table)

	// Same handle borrowed multiple times
	require.NoError(t, scope.AddLender(h))
	require.NoError(t, scope.AddLender(h))

	entry, _ := table.GetResourceHandle(h)
	require.Equal(t, uint32(2), entry.NumLends)

	require.NoError(t, scope.Release())

	entry, _ = table.GetResourceHandle(h)
	require.Equal(t, uint32(0), entry.NumLends)
}
