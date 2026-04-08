package runtime

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestCallMightBeRecursive_NoActiveCall(t *testing.T) {
	tracker := NewReentranceTracker()

	// No active calls to instance 100
	require.False(t, tracker.CallMightBeRecursive(100))
}

func TestCallMightBeRecursive_ActiveCall(t *testing.T) {
	tracker := NewReentranceTracker()

	// Start a call to instance 100
	tracker.EnterInstance(100)

	// A call to instance 100 would be recursive
	require.True(t, tracker.CallMightBeRecursive(100))

	// A call to instance 200 would not be recursive
	require.False(t, tracker.CallMightBeRecursive(200))
}

func TestCallMightBeRecursive_NestedCalls(t *testing.T) {
	tracker := NewReentranceTracker()

	// 100 calls 200
	tracker.EnterInstance(100)
	tracker.EnterInstance(200)

	// Both would be recursive
	require.True(t, tracker.CallMightBeRecursive(100))
	require.True(t, tracker.CallMightBeRecursive(200))

	// 300 would not be recursive
	require.False(t, tracker.CallMightBeRecursive(300))

	// Leave 200
	tracker.LeaveInstance(200)
	require.False(t, tracker.CallMightBeRecursive(200))
	require.True(t, tracker.CallMightBeRecursive(100))
}

// TestReentranceTrackerCallMightBeRecursive asserts the tracker's
// runtime-stack membership query for a single-thread active stack.
//
// This tracker serves the task-level concurrency trap at
// definitions.py:3664-3667, NOT the canonical-ABI
// definitions.py:290-299 call_might_be_recursive spec check. The
// latter is a STRUCTURAL reflexive-ancestor overlap implemented by
// component.Instance.CallMightBeRecursive via a parent-chain walk
// (see internal/component/instance.go:isReflexiveAncestor).
//
// The two checks are intentionally distinct: reintroducing a call
// from component.Instance.CallMightBeRecursive to this tracker was
// the B3/B4 bug that commit b74f5558 removed.
func TestReentranceTrackerCallMightBeRecursive(t *testing.T) {
	rt := NewReentranceTracker()

	// No active calls: no recursion possible.
	require.False(t, rt.CallMightBeRecursive(5))

	// Enter instance 5. Calling 5 is now recursive.
	rt.EnterInstance(5)
	require.True(t, rt.CallMightBeRecursive(5))
	// Calling a different instance (7) from inside 5 is not recursive.
	require.False(t, rt.CallMightBeRecursive(7))

	// Enter nested instance 7. Calling either is now recursive.
	rt.EnterInstance(7)
	require.True(t, rt.CallMightBeRecursive(5))
	require.True(t, rt.CallMightBeRecursive(7))

	// Leave 7. Back to just 5 on the stack.
	rt.LeaveInstance(7)
	require.True(t, rt.CallMightBeRecursive(5))
	require.False(t, rt.CallMightBeRecursive(7))

	// Leave 5. Stack empty.
	rt.LeaveInstance(5)
	require.False(t, rt.CallMightBeRecursive(5))
}
