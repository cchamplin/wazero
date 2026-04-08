package conformance

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestInstance_CallDepthTracking(t *testing.T) {
	inst := component.NewInstance(&component.Component{}, 0, nil)

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
	inst := component.NewInstance(&component.Component{}, 0, nil)

	// Exit without enter - should stay at 0, not go negative
	inst.ExitCall()
	require.Equal(t, 0, inst.ActiveCallDepth(), "should not go negative")

	// Multiple exits should not go negative
	inst.ExitCall()
	inst.ExitCall()
	require.Equal(t, 0, inst.ActiveCallDepth(), "should still be 0")
}

func TestInstance_CallMightBeRecursive(t *testing.T) {
	// Session 1 Task B4: distinct IDs so the ReentranceTracker can
	// actually distinguish callee from caller (same ID would collapse).
	callee := component.NewInstance(&component.Component{}, 1, nil)
	caller := component.NewInstance(&component.Component{}, 2, nil)

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

func TestInstance_ValidateNotRecursive(t *testing.T) {
	inst := component.NewInstance(&component.Component{}, 1, nil)

	t.Run("no_active_call_passes", func(t *testing.T) {
		err := inst.ValidateNotRecursive(inst)
		require.NoError(t, err)
	})

	t.Run("active_call_from_same_instance_fails", func(t *testing.T) {
		inst.EnterCall()
		defer inst.ExitCall()

		err := inst.ValidateNotRecursive(inst)
		require.Error(t, err)
		require.Contains(t, err.Error(), "recursive")
	})

	t.Run("active_call_from_different_instance_passes", func(t *testing.T) {
		other := component.NewInstance(&component.Component{}, 2, nil)
		inst.EnterCall()
		defer inst.ExitCall()

		err := inst.ValidateNotRecursive(other)
		require.NoError(t, err)
	})

	t.Run("host_call_always_passes", func(t *testing.T) {
		inst.EnterCall()
		defer inst.ExitCall()

		err := inst.ValidateNotRecursive(nil)
		require.NoError(t, err)
	})
}

func TestReentrance_CallerInstanceContext(t *testing.T) {
	ctx := context.Background()

	t.Run("no_caller_returns_nil", func(t *testing.T) {
		caller := component.GetCallerInstance(ctx)
		require.Nil(t, caller)
	})

	t.Run("with_caller_returns_instance", func(t *testing.T) {
		inst := component.NewInstance(&component.Component{}, 0, nil)
		ctxWithCaller := component.WithCallerInstance(ctx, inst)

		caller := component.GetCallerInstance(ctxWithCaller)
		require.Same(t, inst, caller)
	})

	t.Run("nested_contexts", func(t *testing.T) {
		inst1 := component.NewInstance(&component.Component{}, 1, nil)
		inst2 := component.NewInstance(&component.Component{}, 2, nil)

		ctx1 := component.WithCallerInstance(ctx, inst1)
		ctx2 := component.WithCallerInstance(ctx1, inst2)

		// Most recent caller wins
		require.Same(t, inst2, component.GetCallerInstance(ctx2))
		// Original context still has inst1
		require.Same(t, inst1, component.GetCallerInstance(ctx1))
	})
}

func TestReentrance_DeepCallStack(t *testing.T) {
	inst := component.NewInstance(&component.Component{}, 0, nil)

	// Simulate deep call stack
	depth := 100
	for i := 0; i < depth; i++ {
		inst.EnterCall()
	}
	require.Equal(t, depth, inst.ActiveCallDepth())

	for i := 0; i < depth; i++ {
		inst.ExitCall()
	}
	require.Equal(t, 0, inst.ActiveCallDepth())
}

func TestReentrance_ExitCallNeverNegative(t *testing.T) {
	inst := component.NewInstance(&component.Component{}, 0, nil)

	// Exit without enter should not go negative
	inst.ExitCall()
	inst.ExitCall()
	inst.ExitCall()

	require.Equal(t, 0, inst.ActiveCallDepth(), "call depth should never go negative")
}
