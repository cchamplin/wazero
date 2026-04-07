package runtime

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestDestructorRegistry_Register(t *testing.T) {
	registry := NewDestructorRegistry()

	var calledWith uint32
	dtor := func(rep uint32) {
		calledWith = rep
	}

	// Register destructor for type 5
	registry.Register(NewResourceTypeID(5), dtor)

	// Get and call it
	got := registry.Get(NewResourceTypeID(5))
	require.NotNil(t, got)

	got(42)
	require.Equal(t, uint32(42), calledWith)
}

func TestDestructorRegistry_Get_NotFound(t *testing.T) {
	registry := NewDestructorRegistry()

	got := registry.Get(NewResourceTypeID(99))
	require.Nil(t, got)
}

func TestDestructorRegistry_Unregister(t *testing.T) {
	registry := NewDestructorRegistry()

	registry.Register(NewResourceTypeID(5), func(uint32) {})
	require.NotNil(t, registry.Get(NewResourceTypeID(5)))

	registry.Unregister(NewResourceTypeID(5))
	require.Nil(t, registry.Get(NewResourceTypeID(5)))
}

func TestDestructorRegistry_Has(t *testing.T) {
	registry := NewDestructorRegistry()

	// Initially should not have destructor
	require.False(t, registry.Has(NewResourceTypeID(5)))

	// After registration should have it
	registry.Register(NewResourceTypeID(5), func(uint32) {})
	require.True(t, registry.Has(NewResourceTypeID(5)))

	// After unregistration should not have it
	registry.Unregister(NewResourceTypeID(5))
	require.False(t, registry.Has(NewResourceTypeID(5)))
}
