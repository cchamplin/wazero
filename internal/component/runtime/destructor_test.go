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

	// Register destructor for a resource type
	rt := &ResourceType{}
	registry.Register(rt, dtor)

	// Get and call it
	got := registry.Get(rt)
	require.NotNil(t, got)

	got(42)
	require.Equal(t, uint32(42), calledWith)
}

func TestDestructorRegistry_Get_NotFound(t *testing.T) {
	registry := NewDestructorRegistry()

	rt := &ResourceType{}
	got := registry.Get(rt)
	require.Nil(t, got)
}

func TestDestructorRegistry_Unregister(t *testing.T) {
	registry := NewDestructorRegistry()

	rt := &ResourceType{}
	registry.Register(rt, func(uint32) {})
	require.NotNil(t, registry.Get(rt))

	registry.Unregister(rt)
	require.Nil(t, registry.Get(rt))
}

func TestDestructorRegistry_Has(t *testing.T) {
	registry := NewDestructorRegistry()

	rt := &ResourceType{}
	// Initially should not have destructor
	require.False(t, registry.Has(rt))

	// After registration should have it
	registry.Register(rt, func(uint32) {})
	require.True(t, registry.Has(rt))

	// After unregistration should not have it
	registry.Unregister(rt)
	require.False(t, registry.Has(rt))
}

func TestDestructorRegistry_PointerIdentity(t *testing.T) {
	// Two distinct *ResourceType pointers are separate keys, even if
	// their field contents are identical. Spec: definitions.py:1345.
	registry := NewDestructorRegistry()

	rA := &ResourceType{}
	rB := &ResourceType{}

	registry.Register(rA, func(uint32) {})
	require.True(t, registry.Has(rA))
	require.False(t, registry.Has(rB))
}
