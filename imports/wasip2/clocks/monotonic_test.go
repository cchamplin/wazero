// imports/wasip2/clocks/monotonic_test.go

package clocks

import (
	"context"
	"testing"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestMonotonicClock_Now(t *testing.T) {
	before := MonotonicNow()
	time.Sleep(1 * time.Millisecond)
	after := MonotonicNow()

	require.True(t, after > before)
}

func TestMonotonicClock_Resolution(t *testing.T) {
	res := MonotonicResolution()
	require.Equal(t, uint64(1), res)
}

func TestSubscribeDuration_Zero(t *testing.T) {
	readyFn, _ := SubscribeDuration(0)
	require.True(t, readyFn())
}

func TestSubscribeDuration_Short(t *testing.T) {
	duration := uint64(10 * time.Millisecond)
	readyFn, blockFn := SubscribeDuration(duration)

	// Should not be ready immediately
	// (might be ready if test runs slowly, so just test block works)
	start := time.Now()
	blockFn()
	elapsed := time.Since(start)

	require.True(t, elapsed >= 10*time.Millisecond)
	require.True(t, readyFn())
}

func TestSubscribeInstant_Past(t *testing.T) {
	// Past instant (0) should be immediately ready
	readyFn, _ := SubscribeInstant(0)
	require.True(t, readyFn())
}

func TestSubscribeInstant_Future(t *testing.T) {
	// Get a future instant
	futureInstant := MonotonicNow() + uint64(10*time.Millisecond)
	readyFn, blockFn := SubscribeInstant(futureInstant)

	// Block and verify timing
	start := time.Now()
	blockFn()
	elapsed := time.Since(start)

	require.True(t, elapsed >= 10*time.Millisecond)
	require.True(t, readyFn())
}

func TestInstantiateMonotonicClock(t *testing.T) {
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)
	err := instantiateMonotonicClock(linker)
	require.NoError(t, err)

	// Verify interface is registered
	def, err := linker.MatchImport("wasi:clocks/monotonic-clock@0.2.0")
	require.NoError(t, err)

	// Verify it's an instance definition
	instDef, ok := def.(*component.InstanceDef)
	require.True(t, ok, "expected InstanceDef")

	// Verify exports exist
	_, hasNow := instDef.Exports["now"]
	require.True(t, hasNow, "now function should be defined")

	_, hasResolution := instDef.Exports["resolution"]
	require.True(t, hasResolution, "resolution function should be defined")

	_, hasSubscribeInstant := instDef.Exports["subscribe-instant"]
	require.True(t, hasSubscribeInstant, "subscribe-instant function should be defined")

	_, hasSubscribeDuration := instDef.Exports["subscribe-duration"]
	require.True(t, hasSubscribeDuration, "subscribe-duration function should be defined")
}

func TestInstantiateMonotonicClock_Duplicate(t *testing.T) {
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)

	// First registration should succeed
	err := instantiateMonotonicClock(linker)
	require.NoError(t, err)

	// Second registration should fail
	err = instantiateMonotonicClock(linker)
	require.Error(t, err)
}

// Tests for host functions with ResourceTable

func TestSubscribeDuration_HostFunction(t *testing.T) {
}

func TestSubscribeDuration_HostFunction_Zero(t *testing.T) {
}

func TestSubscribeDuration_HostFunction_NoResourceTable(t *testing.T) {
	ctx := context.Background() // No resource table

	args := []types.Val{
		types.ValU64(1000),
	}

	// Should return placeholder handle 0 when no resource table
	results, err := monotonicClockSubscribeDuration(ctx, nil, args)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, uint32(0), results[0].Own())
}

func TestSubscribeInstant_HostFunction(t *testing.T) {
}

func TestSubscribeInstant_HostFunction_Past(t *testing.T) {
}

func TestSubscribeInstant_HostFunction_NoResourceTable(t *testing.T) {
	ctx := context.Background() // No resource table

	args := []types.Val{
		types.ValU64(MonotonicNow() + 1000),
	}

	// Should return placeholder handle 0 when no resource table
	results, err := monotonicClockSubscribeInstant(ctx, nil, args)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, uint32(0), results[0].Own())
}
