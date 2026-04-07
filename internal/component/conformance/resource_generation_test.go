// Package conformance contains conformance tests for the Component Model implementation.
// Resource generation counter edge case tests verify handle generation wraparound behavior.
package conformance

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestResourceGeneration_Basic tests basic handle creation and generation.
func TestResourceGeneration_Basic(t *testing.T) {
	table := runtime.NewResourceTable()

	t.Run("first_handle_generation_zero", func(t *testing.T) {
		h := table.New("resource1", true)
		require.Equal(t, uint32(0), h.Index())
		require.Equal(t, uint32(0), h.Generation())
	})

	t.Run("second_handle_different_index", func(t *testing.T) {
		h := table.New("resource2", true)
		require.Equal(t, uint32(1), h.Index())
		require.Equal(t, uint32(0), h.Generation())
	})
}

// TestResourceGeneration_Reuse tests generation increment on slot reuse.
func TestResourceGeneration_Reuse(t *testing.T) {
	table := runtime.NewResourceTable()

	// Create and remove a handle
	h1 := table.New("resource1", true)
	require.Equal(t, uint32(0), h1.Index())
	require.Equal(t, uint32(0), h1.Generation())

	_, err := table.Remove(h1)
	require.NoError(t, err)

	// Reuse the slot - generation should increment
	h2 := table.New("resource2", true)
	require.Equal(t, uint32(0), h2.Index(), "should reuse same index")
	require.Equal(t, uint32(1), h2.Generation(), "generation should increment")

	// Remove and reuse again
	_, err = table.Remove(h2)
	require.NoError(t, err)

	h3 := table.New("resource3", true)
	require.Equal(t, uint32(0), h3.Index())
	require.Equal(t, uint32(2), h3.Generation())
}

// TestResourceGeneration_MultipleSlots tests generation tracking across multiple slots.
func TestResourceGeneration_MultipleSlots(t *testing.T) {
	table := runtime.NewResourceTable()

	// Create multiple handles
	h0 := table.New("r0", true)
	h1 := table.New("r1", true)
	h2 := table.New("r2", true)

	require.Equal(t, uint32(0), h0.Index())
	require.Equal(t, uint32(1), h1.Index())
	require.Equal(t, uint32(2), h2.Index())

	// Remove middle slot
	_, err := table.Remove(h1)
	require.NoError(t, err)

	// New handle should reuse slot 1 with incremented generation
	h1New := table.New("r1_new", true)
	require.Equal(t, uint32(1), h1New.Index())
	require.Equal(t, uint32(1), h1New.Generation())

	// Original h0 and h2 should still work
	entry0, err := table.Get(h0)
	require.NoError(t, err)
	require.Equal(t, "r0", entry0.Rep)

	entry2, err := table.Get(h2)
	require.NoError(t, err)
	require.Equal(t, "r2", entry2.Rep)
}

// TestResourceGeneration_StaleHandleRejection tests that stale handles are rejected.
func TestResourceGeneration_StaleHandleRejection(t *testing.T) {
	table := runtime.NewResourceTable()

	// Create handle
	h1 := table.New("resource1", true)
	require.Equal(t, uint32(0), h1.Generation())

	// Remove and recreate
	_, err := table.Remove(h1)
	require.NoError(t, err)

	h2 := table.New("resource2", true)
	require.Equal(t, uint32(1), h2.Generation())

	// Try to use old handle - should fail with generation mismatch
	_, err = table.Get(h1)
	require.Error(t, err, "stale handle should be rejected")
	require.Contains(t, err.Error(), "generation", "error should mention generation")

	// New handle should work
	entry, err := table.Get(h2)
	require.NoError(t, err)
	require.Equal(t, "resource2", entry.Rep)
}

// TestResourceGeneration_HandleComponents tests Handle index/generation extraction.
func TestResourceGeneration_HandleComponents(t *testing.T) {
	testCases := []struct {
		name       string
		index      uint32
		generation uint32
	}{
		{"zero_zero", 0, 0},
		{"max_index", 0xFFFFFFFF, 0},
		{"max_generation", 0, 0xFFFFFFFF},
		{"both_max", 0xFFFFFFFF, 0xFFFFFFFF},
		{"typical", 42, 7},
		{"large_values", 0x12345678, 0x9ABCDEF0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h := runtime.MakeHandle(tc.index, tc.generation)
			require.Equal(t, tc.index, h.Index())
			require.Equal(t, tc.generation, h.Generation())
		})
	}
}

// TestResourceGeneration_BorrowTracking tests borrow count tracking.
func TestResourceGeneration_BorrowTracking(t *testing.T) {
	table := runtime.NewResourceTable()

	h := table.New("resource", true)

	t.Run("increment_lends", func(t *testing.T) {
		err := table.IncrementLends(h)
		require.NoError(t, err)

		entry, err := table.Get(h)
		require.NoError(t, err)
		require.Equal(t, uint32(1), entry.NumLends)
	})

	t.Run("cannot_remove_with_active_borrows", func(t *testing.T) {
		_, err := table.Remove(h)
		require.Error(t, err)
		require.Contains(t, err.Error(), "borrow", "should indicate active borrows")
	})

	t.Run("decrement_lends", func(t *testing.T) {
		err := table.DecrementLends(h)
		require.NoError(t, err)

		entry, err := table.Get(h)
		require.NoError(t, err)
		require.Equal(t, uint32(0), entry.NumLends)
	})

	t.Run("can_remove_after_borrow_released", func(t *testing.T) {
		_, err := table.Remove(h)
		require.NoError(t, err)
	})
}

// TestResourceGeneration_BorrowUnderflow tests decrementing borrows below zero.
func TestResourceGeneration_BorrowUnderflow(t *testing.T) {
	table := runtime.NewResourceTable()

	h := table.New("resource", true)

	// Try to decrement without any active borrows
	err := table.DecrementLends(h)
	require.Error(t, err, "should error on borrow underflow")
}

// TestResourceGeneration_InvalidHandle tests operations on invalid handles.
func TestResourceGeneration_InvalidHandle(t *testing.T) {
	table := runtime.NewResourceTable()

	t.Run("get_nonexistent", func(t *testing.T) {
		h := runtime.MakeHandle(100, 0)
		_, err := table.Get(h)
		require.Error(t, err)
	})

	t.Run("remove_nonexistent", func(t *testing.T) {
		h := runtime.MakeHandle(100, 0)
		_, err := table.Remove(h)
		require.Error(t, err)
	})

	t.Run("increment_lends_nonexistent", func(t *testing.T) {
		h := runtime.MakeHandle(100, 0)
		err := table.IncrementLends(h)
		require.Error(t, err)
	})

	t.Run("decrement_lends_nonexistent", func(t *testing.T) {
		h := runtime.MakeHandle(100, 0)
		err := table.DecrementLends(h)
		require.Error(t, err)
	})
}

// TestResourceGeneration_FreeListBehavior tests free list ordering.
func TestResourceGeneration_FreeListBehavior(t *testing.T) {
	table := runtime.NewResourceTable()

	// Create 5 handles
	handles := make([]runtime.Handle, 5)
	for i := 0; i < 5; i++ {
		handles[i] = table.New(i, true)
	}

	// Remove in order: 1, 3, 0
	table.Remove(handles[1])
	table.Remove(handles[3])
	table.Remove(handles[0])

	// New handles should be allocated from free list (LIFO order)
	// Free list: 0 -> 3 -> 1 (most recently freed at head)
	h1 := table.New("new1", true)
	require.Equal(t, uint32(0), h1.Index(), "should reuse index 0 first (LIFO)")

	h2 := table.New("new2", true)
	require.Equal(t, uint32(3), h2.Index(), "should reuse index 3 second")

	h3 := table.New("new3", true)
	require.Equal(t, uint32(1), h3.Index(), "should reuse index 1 third")

	// Next allocation should extend the array
	h4 := table.New("new4", true)
	require.Equal(t, uint32(5), h4.Index(), "should allocate new index 5")
}

// TestResourceGeneration_OwnVsBorrow tests ownership flag tracking.
func TestResourceGeneration_OwnVsBorrow(t *testing.T) {
	table := runtime.NewResourceTable()

	t.Run("owned_handle", func(t *testing.T) {
		h := table.New("owned", true)
		entry, err := table.Get(h)
		require.NoError(t, err)
		require.True(t, entry.Own)
	})

	t.Run("borrowed_handle", func(t *testing.T) {
		h := table.New("borrowed", false)
		entry, err := table.Get(h)
		require.NoError(t, err)
		require.False(t, entry.Own)
	})
}

// TestResourceGeneration_RepresentationValue tests storing different rep values.
func TestResourceGeneration_RepresentationValue(t *testing.T) {
	table := runtime.NewResourceTable()

	testCases := []struct {
		name string
		rep  any
	}{
		{"string", "hello"},
		{"int", 42},
		{"struct", struct{ X int }{X: 100}},
		{"slice", []byte{1, 2, 3}},
		{"nil", nil},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h := table.New(tc.rep, true)
			entry, err := table.Get(h)
			require.NoError(t, err)
			require.Equal(t, tc.rep, entry.Rep)
		})
	}
}

// TestResourceGeneration_RemoveReturnsEntry tests that Remove returns the entry.
func TestResourceGeneration_RemoveReturnsEntry(t *testing.T) {
	table := runtime.NewResourceTable()

	h := table.New("my_resource", true)

	entry, err := table.Remove(h)
	require.NoError(t, err)
	require.NotNil(t, entry)
	require.Equal(t, "my_resource", entry.Rep)
	require.True(t, entry.Own)
	require.Equal(t, uint32(0), entry.NumLends)
}

// TestResourceGeneration_DoubleRemove tests that removing twice fails.
func TestResourceGeneration_DoubleRemove(t *testing.T) {
	table := runtime.NewResourceTable()

	h := table.New("resource", true)

	// First remove succeeds
	_, err := table.Remove(h)
	require.NoError(t, err)

	// Second remove fails
	_, err = table.Remove(h)
	require.Error(t, err)
}

// TestResourceGeneration_ManyOperations tests many create/remove cycles.
func TestResourceGeneration_ManyOperations(t *testing.T) {
	table := runtime.NewResourceTable()
	iterations := 1000

	for i := 0; i < iterations; i++ {
		h := table.New(i, true)

		// Verify we can get it
		entry, err := table.Get(h)
		require.NoError(t, err)
		require.Equal(t, i, entry.Rep)

		// Remove it
		_, err = table.Remove(h)
		require.NoError(t, err)
	}
}

// TestResourceGeneration_ManyActiveHandles tests having many active handles.
func TestResourceGeneration_ManyActiveHandles(t *testing.T) {
	table := runtime.NewResourceTable()
	count := 1000

	handles := make([]runtime.Handle, count)
	for i := 0; i < count; i++ {
		handles[i] = table.New(i, true)
	}

	// Verify all handles are accessible
	for i, h := range handles {
		entry, err := table.Get(h)
		require.NoError(t, err)
		require.Equal(t, i, entry.Rep)
	}

	// Remove all handles
	for _, h := range handles {
		_, err := table.Remove(h)
		require.NoError(t, err)
	}
}
