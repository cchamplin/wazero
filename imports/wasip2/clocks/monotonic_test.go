// imports/wasip2/clocks/monotonic_test.go

package clocks

import (
	"context"
	"testing"
	"time"

	wasip2io "github.com/tetratelabs/wazero/imports/wasip2/io"
	"github.com/tetratelabs/wazero/internal/component"
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
	linker := component.NewLinker()
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
	linker := component.NewLinker()

	// First registration should succeed
	err := instantiateMonotonicClock(linker)
	require.NoError(t, err)

	// Second registration should fail
	err = instantiateMonotonicClock(linker)
	require.Error(t, err)
}

// Tests for host functions with ResourceTable

func TestSubscribeDuration_HostFunction(t *testing.T) {
	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Subscribe for a 10ms duration
	duration := uint64(10 * time.Millisecond)
	args := []component.Val{
		component.ValU64(duration),
	}

	results, err := monotonicClockSubscribeDuration(ctx, args)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))

	// Get the handle from the result
	handle := component.Handle(results[0].Own())

	// Verify the pollable was registered correctly by retrieving it
	entry, err := table.Get(handle)
	require.NoError(t, err)
	require.True(t, entry.Own, "expected owned handle")

	// Verify we can cast to *Pollable
	pollable, ok := entry.Rep.(*wasip2io.Pollable)
	require.True(t, ok, "expected *Pollable type")

	// Block and verify the pollable becomes ready
	start := time.Now()
	pollable.Block()
	elapsed := time.Since(start)

	require.True(t, elapsed >= 10*time.Millisecond, "expected block for at least 10ms")
	require.True(t, pollable.Ready(), "expected pollable to be ready after block")
}

func TestSubscribeDuration_HostFunction_Zero(t *testing.T) {
	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Zero duration should be immediately ready
	args := []component.Val{
		component.ValU64(0),
	}

	results, err := monotonicClockSubscribeDuration(ctx, args)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))

	handle := component.Handle(results[0].Own())
	entry, err := table.Get(handle)
	require.NoError(t, err)

	pollable, ok := entry.Rep.(*wasip2io.Pollable)
	require.True(t, ok, "expected *Pollable type")

	// Should be immediately ready
	require.True(t, pollable.Ready(), "zero duration pollable should be immediately ready")
}

func TestSubscribeDuration_HostFunction_NoResourceTable(t *testing.T) {
	ctx := context.Background() // No resource table

	args := []component.Val{
		component.ValU64(1000),
	}

	// Should return placeholder handle 0 when no resource table
	results, err := monotonicClockSubscribeDuration(ctx, args)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, uint32(0), results[0].Own())
}

func TestSubscribeInstant_HostFunction(t *testing.T) {
	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Subscribe for an instant 10ms in the future
	futureInstant := MonotonicNow() + uint64(10*time.Millisecond)
	args := []component.Val{
		component.ValU64(futureInstant),
	}

	results, err := monotonicClockSubscribeInstant(ctx, args)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))

	// Get the handle from the result
	handle := component.Handle(results[0].Own())

	// Verify the pollable was registered correctly by retrieving it
	entry, err := table.Get(handle)
	require.NoError(t, err)
	require.True(t, entry.Own, "expected owned handle")

	// Verify we can cast to *Pollable
	pollable, ok := entry.Rep.(*wasip2io.Pollable)
	require.True(t, ok, "expected *Pollable type")

	// Block and verify the pollable becomes ready
	start := time.Now()
	pollable.Block()
	elapsed := time.Since(start)

	require.True(t, elapsed >= 10*time.Millisecond, "expected block for at least 10ms")
	require.True(t, pollable.Ready(), "expected pollable to be ready after block")
}

func TestSubscribeInstant_HostFunction_Past(t *testing.T) {
	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Past instant (0) should be immediately ready
	args := []component.Val{
		component.ValU64(0),
	}

	results, err := monotonicClockSubscribeInstant(ctx, args)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))

	handle := component.Handle(results[0].Own())
	entry, err := table.Get(handle)
	require.NoError(t, err)

	pollable, ok := entry.Rep.(*wasip2io.Pollable)
	require.True(t, ok, "expected *Pollable type")

	// Past instant should be immediately ready
	require.True(t, pollable.Ready(), "past instant pollable should be immediately ready")
}

func TestSubscribeInstant_HostFunction_NoResourceTable(t *testing.T) {
	ctx := context.Background() // No resource table

	args := []component.Val{
		component.ValU64(MonotonicNow() + 1000),
	}

	// Should return placeholder handle 0 when no resource table
	results, err := monotonicClockSubscribeInstant(ctx, args)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, uint32(0), results[0].Own())
}
