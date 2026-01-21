package conformance

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestInstance_CallDepthTracking(t *testing.T) {
	inst := &component.Instance{}

	require.Equal(t, 0, inst.ActiveCallDepth(), "should start at 0")

	inst.EnterCall()
	require.Equal(t, 1, inst.ActiveCallDepth())

	inst.EnterCall()
	require.Equal(t, 2, inst.ActiveCallDepth())

	inst.ExitCall()
	require.Equal(t, 1, inst.ActiveCallDepth())

	inst.ExitCall()
	require.Equal(t, 0, inst.ActiveCallDepth())
}

func TestInstance_CallDepthNilSafety(t *testing.T) {
	var inst *component.Instance

	// Should not panic and return 0 for nil receiver
	require.Equal(t, 0, inst.ActiveCallDepth(), "nil instance should return 0")

	// Should not panic for nil receiver
	inst.EnterCall()
	inst.ExitCall()
}

func TestInstance_CallDepthNoUnderflow(t *testing.T) {
	inst := &component.Instance{}

	// Exit without enter - should stay at 0, not go negative
	inst.ExitCall()
	require.Equal(t, 0, inst.ActiveCallDepth(), "should not go negative")

	// Multiple exits should not go negative
	inst.ExitCall()
	inst.ExitCall()
	require.Equal(t, 0, inst.ActiveCallDepth(), "should still be 0")
}
