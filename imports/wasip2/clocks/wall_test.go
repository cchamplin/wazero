// imports/wasip2/clocks/wall_test.go

package clocks

import (
	"context"
	"testing"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestWallClock_Now(t *testing.T) {
	before := time.Now()
	dt := WallClockNow()
	after := time.Now()

	require.True(t, dt.Seconds >= uint64(before.Unix()))
	require.True(t, dt.Seconds <= uint64(after.Unix()))
}

func TestWallClock_Resolution(t *testing.T) {
	dt := WallClockResolution()
	require.Equal(t, uint64(0), dt.Seconds)
	require.Equal(t, uint32(1), dt.Nanoseconds)
}

func TestInstantiateWallClock(t *testing.T) {
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)
	err := instantiateWallClock(linker)
	require.NoError(t, err)

	// Verify interface is registered
	def, err := linker.MatchImport("wasi:clocks/wall-clock@0.2.0")
	require.NoError(t, err)

	// Verify it's an instance definition
	instDef, ok := def.(*component.InstanceDef)
	require.True(t, ok, "expected InstanceDef")

	// Verify exports exist
	_, hasNow := instDef.Exports["now"]
	require.True(t, hasNow, "now function should be defined")

	_, hasResolution := instDef.Exports["resolution"]
	require.True(t, hasResolution, "resolution function should be defined")
}

func TestInstantiateWallClock_Duplicate(t *testing.T) {
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)

	// First registration should succeed
	err := instantiateWallClock(linker)
	require.NoError(t, err)

	// Second registration should fail
	err = instantiateWallClock(linker)
	require.Error(t, err)
}
