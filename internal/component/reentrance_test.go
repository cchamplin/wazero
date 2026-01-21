package component

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
