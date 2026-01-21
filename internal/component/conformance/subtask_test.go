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

func TestSubtask_StateTransitions(t *testing.T) {
	rt := component.NewResourceTable()
	subtask := component.NewSubtask(rt)

	t.Run("pending_to_resolved", func(t *testing.T) {
		result := []component.Val{component.ValU32(42)}
		err := subtask.DeliverResolve(result)
		require.NoError(t, err)
		require.Equal(t, component.SubtaskStateResolved, subtask.State())
	})

	t.Run("resolved_to_finishing", func(t *testing.T) {
		err := subtask.StartFinish()
		require.NoError(t, err)
		require.Equal(t, component.SubtaskStateFinishing, subtask.State())
	})

	t.Run("finishing_to_done", func(t *testing.T) {
		err := subtask.Finish()
		require.NoError(t, err)
		require.Equal(t, component.SubtaskStateDone, subtask.State())
	})
}

func TestSubtask_InvalidTransitions(t *testing.T) {
	rt := component.NewResourceTable()

	t.Run("cannot_resolve_twice", func(t *testing.T) {
		subtask := component.NewSubtask(rt)
		_ = subtask.DeliverResolve(nil)

		err := subtask.DeliverResolve(nil)
		require.Error(t, err, "should not resolve twice")
	})

	t.Run("cannot_finish_before_resolve", func(t *testing.T) {
		subtask := component.NewSubtask(rt)

		err := subtask.Finish()
		require.Error(t, err, "should not finish before resolve")
	})
}

func TestSubtask_LendTracking(t *testing.T) {
	rt := component.NewResourceTable()

	// Create a resource to borrow
	handle := rt.New("test-resource", true)

	subtask := component.NewSubtask(rt)

	t.Run("track_lend", func(t *testing.T) {
		err := subtask.TrackLend(handle)
		require.NoError(t, err)
		require.Equal(t, 1, subtask.LendCount())
	})

	t.Run("multiple_lends", func(t *testing.T) {
		handle2 := rt.New("test-resource-2", true)
		err := subtask.TrackLend(handle2)
		require.NoError(t, err)
		require.Equal(t, 2, subtask.LendCount())
	})

	t.Run("finish_releases_lends", func(t *testing.T) {
		subtask.DeliverResolve(nil)
		subtask.StartFinish()
		err := subtask.Finish()
		require.NoError(t, err)
		require.Equal(t, component.SubtaskStateDone, subtask.State())
	})
}
