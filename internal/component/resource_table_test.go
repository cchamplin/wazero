// internal/component/resource_table_test.go

package component

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestResourceTable_NewAndGet(t *testing.T) {
	table := NewResourceTable()

	// Create a resource
	h := table.New("my-resource", true) // own=true

	// Verify handle parts
	require.Equal(t, uint32(0), h.Index())
	require.Equal(t, uint32(0), h.Generation())

	// Retrieve it
	entry, err := table.Get(h)
	require.NoError(t, err)
	require.Equal(t, "my-resource", entry.Rep)
	require.True(t, entry.Own)
}

func TestResourceTable_MultipleResources(t *testing.T) {
	table := NewResourceTable()

	h1 := table.New("first", true)
	h2 := table.New("second", true)
	h3 := table.New("third", true)

	require.Equal(t, uint32(0), h1.Index())
	require.Equal(t, uint32(1), h2.Index())
	require.Equal(t, uint32(2), h3.Index())

	e1, _ := table.Get(h1)
	e2, _ := table.Get(h2)
	e3, _ := table.Get(h3)

	require.Equal(t, "first", e1.Rep)
	require.Equal(t, "second", e2.Rep)
	require.Equal(t, "third", e3.Rep)
}

func TestHandle_MakeHandle(t *testing.T) {
	h := MakeHandle(42, 7)
	require.Equal(t, uint32(42), h.Index())
	require.Equal(t, uint32(7), h.Generation())
}

func TestResourceTable_Remove(t *testing.T) {
	table := NewResourceTable()
	h := table.New("my-resource", true)

	// Remove returns the entry
	entry, err := table.Remove(h)
	require.NoError(t, err)
	require.Equal(t, "my-resource", entry.Rep)
	require.True(t, entry.Own)

	// Subsequent Get fails
	_, err = table.Get(h)
	require.Error(t, err)
}

func TestResourceTable_UseAfterFree(t *testing.T) {
	table := NewResourceTable()

	// Create and remove a resource
	h1 := table.New("first", true)
	_, err := table.Remove(h1)
	require.NoError(t, err)

	// Create another resource (reuses index 0)
	h2 := table.New("second", true)
	require.Equal(t, uint32(0), h2.Index())
	require.Equal(t, uint32(1), h2.Generation()) // Generation incremented

	// Old handle should fail (generation mismatch)
	_, err = table.Get(h1)
	require.Error(t, err, "generation mismatch should prevent access")

	// New handle works
	entry, err := table.Get(h2)
	require.NoError(t, err)
	require.Equal(t, "second", entry.Rep)
}

func TestResourceTable_DoubleFree(t *testing.T) {
	table := NewResourceTable()
	h := table.New("resource", true)

	// First remove succeeds
	_, err := table.Remove(h)
	require.NoError(t, err)

	// Second remove fails (already freed)
	_, err = table.Remove(h)
	require.Error(t, err)
}

func TestResourceTable_RemoveWithActiveBorrows(t *testing.T) {
	table := NewResourceTable()
	h := table.New("resource", true)

	// Manually set NumLends to simulate active borrow
	entry, _ := table.Get(h)
	entry.NumLends = 1

	_, err := table.Remove(h)
	require.ErrorIs(t, err, ErrResourceInUse)
}
