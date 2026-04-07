// internal/component/runtime/borrow_scope_test.go

package runtime

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestBorrowScope_TrackLenders(t *testing.T) {
	table := NewResourceTable()
	h1 := table.New("resource1", true)
	h2 := table.New("resource2", true)

	scope := NewBorrowScope(table)

	// Track borrows from two handles
	require.NoError(t, scope.AddLender(h1))
	require.NoError(t, scope.AddLender(h2))

	// Both handles should have NumLends incremented
	e1, _ := table.Get(h1)
	e2, _ := table.Get(h2)
	require.Equal(t, uint32(1), e1.NumLends)
	require.Equal(t, uint32(1), e2.NumLends)

	// End scope releases all lends
	require.NoError(t, scope.Release())

	e1, _ = table.Get(h1)
	e2, _ = table.Get(h2)
	require.Equal(t, uint32(0), e1.NumLends)
	require.Equal(t, uint32(0), e2.NumLends)
}

func TestBorrowScope_SameLenderMultipleTimes(t *testing.T) {
	table := NewResourceTable()
	h := table.New("resource", true)

	scope := NewBorrowScope(table)

	// Same handle borrowed multiple times
	require.NoError(t, scope.AddLender(h))
	require.NoError(t, scope.AddLender(h))

	entry, _ := table.Get(h)
	require.Equal(t, uint32(2), entry.NumLends)

	require.NoError(t, scope.Release())

	entry, _ = table.Get(h)
	require.Equal(t, uint32(0), entry.NumLends)
}
