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
