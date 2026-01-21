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

func TestSubtask_FullLifecycle(t *testing.T) {
	rt := component.NewResourceTable()

	t.Run("simple_call", func(t *testing.T) {
		subtask := component.NewSubtask(rt)

		// Pending state
		require.Equal(t, component.SubtaskStatePending, subtask.State())

		// Call completes
		result := []component.Val{component.ValU32(42)}
		err := subtask.DeliverResolve(result)
		require.NoError(t, err)
		require.Equal(t, component.SubtaskStateResolved, subtask.State())
		require.Equal(t, result, subtask.Result())

		// Cleanup
		err = subtask.StartFinish()
		require.NoError(t, err)
		require.Equal(t, component.SubtaskStateFinishing, subtask.State())

		err = subtask.Finish()
		require.NoError(t, err)
		require.Equal(t, component.SubtaskStateDone, subtask.State())
	})

	t.Run("call_with_borrows", func(t *testing.T) {
		subtask := component.NewSubtask(rt)

		// Create resources and track lends
		h1 := rt.New("resource-1", true)
		h2 := rt.New("resource-2", true)

		err := subtask.TrackLend(h1)
		require.NoError(t, err)
		err = subtask.TrackLend(h2)
		require.NoError(t, err)

		require.Equal(t, 2, subtask.LendCount())

		// Complete call
		subtask.DeliverResolve(nil)
		subtask.StartFinish()
		err = subtask.Finish()
		require.NoError(t, err)

		// Lends should be released
		require.Equal(t, component.SubtaskStateDone, subtask.State())
	})
}

func TestSubtask_NilResult(t *testing.T) {
	rt := component.NewResourceTable()
	subtask := component.NewSubtask(rt)

	err := subtask.DeliverResolve(nil)
	require.NoError(t, err)
	require.Nil(t, subtask.Result())
}

func TestSubtask_EmptyResult(t *testing.T) {
	rt := component.NewResourceTable()
	subtask := component.NewSubtask(rt)

	err := subtask.DeliverResolve([]component.Val{})
	require.NoError(t, err)
	require.Equal(t, 0, len(subtask.Result()))
}

func TestSubtask_MultipleResults(t *testing.T) {
	rt := component.NewResourceTable()
	subtask := component.NewSubtask(rt)

	results := []component.Val{
		component.ValU32(1),
		component.ValU32(2),
		component.ValU32(3),
	}
	err := subtask.DeliverResolve(results)
	require.NoError(t, err)
	require.Equal(t, 3, len(subtask.Result()))
}

func TestSubtask_StateErrors(t *testing.T) {
	rt := component.NewResourceTable()

	t.Run("double_resolve", func(t *testing.T) {
		subtask := component.NewSubtask(rt)
		_ = subtask.DeliverResolve(nil)

		err := subtask.DeliverResolve(nil)
		require.Error(t, err)
	})

	t.Run("finish_without_resolve", func(t *testing.T) {
		subtask := component.NewSubtask(rt)

		err := subtask.Finish()
		require.Error(t, err)
	})

	t.Run("start_finish_twice", func(t *testing.T) {
		subtask := component.NewSubtask(rt)
		_ = subtask.DeliverResolve(nil)
		_ = subtask.StartFinish()

		err := subtask.StartFinish()
		require.Error(t, err)
	})

	t.Run("double_finish", func(t *testing.T) {
		subtask := component.NewSubtask(rt)
		_ = subtask.DeliverResolve(nil)
		_ = subtask.StartFinish()
		_ = subtask.Finish()

		err := subtask.Finish()
		require.Error(t, err)
	})
}

func TestSubtask_NilResourceTable(t *testing.T) {
	// Subtask with nil resource table should still work
	subtask := component.NewSubtask(nil)
	require.NotNil(t, subtask)

	// BorrowScope is still created, just with nil resource table
	// This is acceptable for calls that don't involve resources
	require.NotNil(t, subtask.BorrowScope())

	// Can still go through lifecycle
	err := subtask.DeliverResolve(nil)
	require.NoError(t, err)
	err = subtask.StartFinish()
	require.NoError(t, err)
	err = subtask.Finish()
	require.NoError(t, err)
}
