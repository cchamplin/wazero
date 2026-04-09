package conformance

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// Spec: definitions.py:290-299 (call_might_be_recursive and the
// reflexive-ancestor overlap check that gates reentrance).
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

// No counterpart (justified): nil-receiver safety is a Go-specific
// defensive guard; the spec assumes non-nil instances.
func TestInstance_CallDepthNilSafety(t *testing.T) {
	var inst *component.Instance

	// Should not panic and return 0 for nil receiver
	require.Equal(t, 0, inst.ActiveCallDepth(), "nil instance should return 0")

	// Should not panic for nil receiver
	inst.EnterCall()
	inst.ExitCall()
}

// No counterpart (justified): underflow guard is a Go-specific
// defensive check; the spec assumes balanced enter/exit pairs.
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

// Spec: definitions.py:290-299 (call_might_be_recursive — reflexive-ancestor
// overlap check for reentrance detection).
func TestInstance_CallMightBeRecursive(t *testing.T) {
	// Session 1 Task B4 corrective: CallMightBeRecursive implements the
	// spec's structural reflexive-ancestor overlap check per
	// definitions.py:290-299, not an active-call-tracker consultation.
	// Distinct IDs keep the instances separable for debugging.
	callee := component.NewInstance(&component.Component{}, 1, nil)
	caller := component.NewInstance(&component.Component{}, 2, nil)

	t.Run("different_instances_no_reentrance", func(t *testing.T) {
		// Spec: definitions.py:290-299. Siblings with no parent
		// relationship have disjoint reflexive_ancestors sets, so the
		// call is not structurally recursive — regardless of whether
		// the callee is currently executing on its own call stack.
		callee.EnterCall()
		recursive := callee.CallMightBeRecursive(caller)
		require.False(t, recursive, "different instances cannot be recursive")
		callee.ExitCall()
	})

	t.Run("same_instance_no_active_call", func(t *testing.T) {
		// Spec: definitions.py:290-299 call_might_be_recursive.
		// Per spec, same-instance calls are ALWAYS potentially recursive
		// because an instance is its own reflexive ancestor — so
		// caller.inst.is_reflexive_ancestor_of(callee_inst) is trivially
		// true when caller == callee. The old wazero implementation gated
		// this on active call depth, which was a wazero-specific
		// over-strict simplification the spec does not permit.
		recursive := callee.CallMightBeRecursive(callee)
		require.True(t, recursive, "same instance is always reflexively its own ancestor")
	})

	t.Run("same_instance_with_active_call", func(t *testing.T) {
		// Same instance is reflexively recursive regardless of active
		// call depth. Spec: definitions.py:290-299.
		callee.EnterCall()
		recursive := callee.CallMightBeRecursive(callee)
		require.True(t, recursive, "same instance with active call is recursive")
		callee.ExitCall()
	})

	t.Run("nil_caller", func(t *testing.T) {
		// Spec: definitions.py:290-299. A nil caller models a host call
		// with no supertask chain; wazero's Session 1 local-only model
		// has no supertask tree above the host, so the spec's host
		// branch reduces to returning false.
		recursive := callee.CallMightBeRecursive(nil)
		require.False(t, recursive, "nil caller means host call, not recursive")
	})
}

// Spec: definitions.py:290-299 (call_might_be_recursive — ValidateNotRecursive
// wraps the reflexive-ancestor overlap check).
func TestInstance_ValidateNotRecursive(t *testing.T) {
	inst := component.NewInstance(&component.Component{}, 1, nil)

	t.Run("no_active_call_still_recursive", func(t *testing.T) {
		// Spec: definitions.py:290-299 call_might_be_recursive.
		// ValidateNotRecursive delegates to CallMightBeRecursive, which
		// per spec checks reflexive ancestor overlap. An instance is
		// trivially its own reflexive ancestor, so a same-instance call
		// is ALWAYS potentially recursive — active call depth is
		// irrelevant. The old wazero assertion gated this on active
		// call state, which was a wazero-specific over-strict
		// simplification the spec does not permit. Symmetric with the
		// `same_instance_no_active_call` flip in TestInstance_CallMightBeRecursive.
		err := inst.ValidateNotRecursive(inst)
		require.Error(t, err)
		require.Contains(t, err.Error(), "recursive")
	})

	t.Run("active_call_from_same_instance_fails", func(t *testing.T) {
		// Also recursive per spec — same instance is its own reflexive
		// ancestor regardless of active-call state. This subtest's
		// assertion direction matches the spec; only its rationale
		// changes (reflexive ancestry, not active-call depth).
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

// Spec: definitions.py:290-299 (call_might_be_recursive requires knowing
// the caller instance; these tests verify context-based caller tracking).
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

// Spec: definitions.py:290-299 (deep call stacks exercise the reentrance
// tracking infrastructure without exceeding stack limits).
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

// No counterpart (justified): negative-depth guard is a Go-specific
// defensive check; the spec assumes balanced enter/exit pairs.
func TestReentrance_ExitCallNeverNegative(t *testing.T) {
	inst := component.NewInstance(&component.Component{}, 0, nil)

	// Exit without enter should not go negative
	inst.ExitCall()
	inst.ExitCall()
	inst.ExitCall()

	require.Equal(t, 0, inst.ActiveCallDepth(), "call depth should never go negative")
}
