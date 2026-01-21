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
