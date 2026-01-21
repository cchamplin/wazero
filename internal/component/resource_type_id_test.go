package component

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestResourceTypeID_Equality(t *testing.T) {
	id1 := NewResourceTypeID(1)
	id2 := NewResourceTypeID(1)
	id3 := NewResourceTypeID(2)

	require.True(t, id1 == id2, "same index should be equal")
	require.False(t, id1 == id3, "different index should not be equal")
}

func TestResourceTypeID_Index(t *testing.T) {
	id := NewResourceTypeID(42)
	require.Equal(t, uint32(42), id.Index())
}

func TestResourceTypeID_IsValid(t *testing.T) {
	valid := NewResourceTypeID(1)
	invalid := InvalidResourceTypeID()

	require.True(t, valid.IsValid())
	require.False(t, invalid.IsValid())
}
