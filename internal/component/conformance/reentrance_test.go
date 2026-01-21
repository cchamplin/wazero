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

func TestInstance_CallMightBeRecursive(t *testing.T) {
	callee := &component.Instance{}
	caller := &component.Instance{}

	t.Run("different_instances_no_reentrance", func(t *testing.T) {
		// Caller and callee are different - never recursive
		callee.EnterCall()
		recursive := callee.CallMightBeRecursive(caller)
		require.False(t, recursive, "different instances cannot be recursive")
		callee.ExitCall()
	})

	t.Run("same_instance_no_active_call", func(t *testing.T) {
		// Same instance but no active call - not recursive
		recursive := callee.CallMightBeRecursive(callee)
		require.False(t, recursive, "no active call means not recursive")
	})

	t.Run("same_instance_with_active_call", func(t *testing.T) {
		// Same instance with active call - RECURSIVE
		callee.EnterCall()
		recursive := callee.CallMightBeRecursive(callee)
		require.True(t, recursive, "same instance with active call is recursive")
		callee.ExitCall()
	})

	t.Run("nil_caller", func(t *testing.T) {
		// Nil caller (host call) - never recursive
		recursive := callee.CallMightBeRecursive(nil)
		require.False(t, recursive, "nil caller means host call, not recursive")
	})
}
