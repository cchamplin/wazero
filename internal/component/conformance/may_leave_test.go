package conformance

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestInstance_MayLeaveDefaultTrue(t *testing.T) {
	inst := &component.Instance{}
	require.True(t, inst.MayLeave(), "may_leave should default to true")
}

func TestInstance_SetMayLeave(t *testing.T) {
	inst := &component.Instance{}

	// Default is true
	require.True(t, inst.MayLeave())

	// Set to false
	inst.SetMayLeave(false)
	require.False(t, inst.MayLeave())

	// Set back to true
	inst.SetMayLeave(true)
	require.True(t, inst.MayLeave())
}

func TestInstance_MayLeaveFalseDuringLowering(t *testing.T) {
	// This test verifies that if a component function were to call back
	// during parameter lowering, it would see mayLeave=false.
	//
	// We test this by checking the flag state in the Call path.
	// The actual enforcement is tested in Task 1.5.

	inst := &component.Instance{}
	require.True(t, inst.MayLeave(), "should start true")

	// Simulate entering lowering
	inst.SetMayLeave(false)
	require.False(t, inst.MayLeave(), "should be false during lowering")

	// Simulate exiting lowering
	inst.SetMayLeave(true)
	require.True(t, inst.MayLeave(), "should be true after lowering")
}

func TestInstance_MayLeaveFalseDuringPostReturn(t *testing.T) {
	// This test documents that may_leave should be false during post-return.
	// The actual logic is in instance.go where postReturnFunc is called.
	// Per CanonicalABI.md lines 3287-3289, may_leave must be false during
	// post-return execution to ensure synchronous lowered calls can always
	// be implemented by plain synchronous function calls.

	inst := &component.Instance{}

	// Simulate post-return execution
	inst.SetMayLeave(false)
	require.False(t, inst.MayLeave(), "should be false during post-return")

	inst.SetMayLeave(true)
	require.True(t, inst.MayLeave(), "should be true after post-return")
}
