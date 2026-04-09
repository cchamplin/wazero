// internal/component/runtime/borrow_scope_test.go

package runtime

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestBorrowScope_TrackLenders(t *testing.T) {
	table := NewTable()
	h1, err := table.NewResourceHandle(uint32(1), true, nil)
	require.NoError(t, err)
	h2, err := table.NewResourceHandle(uint32(2), true, nil)
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

// TestBorrowScopeReleaseBorrow asserts the scope's release operation is
// the symmetric inverse of AddLender: it decrements NumLends on the
// handle's entry and removes the handle from the lender set.
//
// Spec: definitions.py:2163-2164 canon_resource_drop borrow branch.
// Spec: definitions.py:738-742 deliver_resolve (scope closure).
func TestBorrowScopeReleaseBorrow(t *testing.T) {
	tbl := NewTable()
	rt := &ResourceType{}
	h, err := tbl.NewResourceHandle(uint32(1), true, rt)
	require.NoError(t, err)

	scope := NewBorrowScope(tbl)
	// AddLender internally calls IncrementLends.
	require.NoError(t, scope.AddLender(h))

	if !scope.HasOutstandingBorrows() {
		t.Fatalf("HasOutstandingBorrows = false, want true")
	}

	require.NoError(t, scope.ReleaseBorrow(h))

	if scope.HasOutstandingBorrows() {
		t.Fatalf("HasOutstandingBorrows after release = true, want false")
	}

	entry, err := tbl.GetResourceHandle(h)
	require.NoError(t, err)
	if entry.NumLends != 0 {
		t.Fatalf("entry.NumLends = %d, want 0", entry.NumLends)
	}
}

// TestBorrowScopeReleaseBorrowNotFound verifies ReleaseBorrow returns an
// error when the handle is not in the lender set.
func TestBorrowScopeReleaseBorrowNotFound(t *testing.T) {
	tbl := NewTable()
	rt := &ResourceType{}
	h, err := tbl.NewResourceHandle(uint32(1), true, rt)
	require.NoError(t, err)

	scope := NewBorrowScope(tbl)
	// Never added — should fail.
	err = scope.ReleaseBorrow(h)
	if err == nil {
		t.Fatalf("ReleaseBorrow on handle not in scope should return error")
	}
}

func TestBorrowScope_SameLenderMultipleTimes(t *testing.T) {
	table := NewTable()
	h, err := table.NewResourceHandle(uint32(3), true, nil)
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
