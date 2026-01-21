package conformance

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestSubtask_Creation(t *testing.T) {
	rt := component.NewResourceTable()
	subtask := component.NewSubtask(rt)

	require.NotNil(t, subtask)
	require.NotNil(t, subtask.BorrowScope())
	require.Equal(t, component.SubtaskStatePending, subtask.State())
}
