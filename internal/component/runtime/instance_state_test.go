package runtime

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestInstanceState_MayLeave(t *testing.T) {
	state := NewInstanceState(1)

	// Initially may_leave is true
	require.True(t, state.MayLeave())

	// During certain operations, may_leave is false
	state.SetMayLeave(false)
	require.False(t, state.MayLeave())

	// Can be restored
	state.SetMayLeave(true)
	require.True(t, state.MayLeave())
}

func TestInstanceState_ID(t *testing.T) {
	state := NewInstanceState(42)
	require.Equal(t, uint32(42), state.ID())
}

func TestInstanceState_EnterLeave(t *testing.T) {
	state := NewInstanceState(1)

	// Enter disables may_leave
	state.Enter()
	require.False(t, state.MayLeave())

	// Leave enables may_leave
	state.Leave()
	require.True(t, state.MayLeave())
}

func TestInstanceState_NestedEnter(t *testing.T) {
	state := NewInstanceState(1)

	// Multiple enters
	state.Enter()
	state.Enter()
	state.Enter()

	// Still can't leave
	require.False(t, state.MayLeave())

	// Need to leave same number of times
	state.Leave()
	require.False(t, state.MayLeave())

	state.Leave()
	require.False(t, state.MayLeave())

	state.Leave()
	require.True(t, state.MayLeave())
}
