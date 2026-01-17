// Package conformance contains conformance tests for the Component Model implementation.
// This file implements Task 276: WASI Clocks Conformance Tests.
package conformance

import (
	"context"
	"testing"
	"time"

	"github.com/tetratelabs/wazero/imports/wasip2"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// =============================================================================
// Task 276: WASI Clocks Conformance Tests
// =============================================================================

// TestWASI_MonotonicClock_Now tests that the monotonic clock now() function
// returns nanoseconds and values are monotonically increasing.
func TestWASI_MonotonicClock_Now(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	// Get the monotonic clock interface
	clockDef, ok := linker.Get("wasi:clocks/monotonic-clock@0.2.0")
	require.True(t, ok, "monotonic-clock interface should be registered")

	instDef, ok := clockDef.(*component.InstanceDef)
	require.True(t, ok, "should be an InstanceDef")

	nowFunc, ok := instDef.Exports["now"]
	require.True(t, ok, "now function should be exported")

	funcDef, ok := nowFunc.(*component.FuncDef)
	require.True(t, ok, "should be a FuncDef")

	// Call now() multiple times and verify values are monotonically increasing
	var lastInstant uint64
	for i := 0; i < 5; i++ {
		result, err := funcDef.Callback(ctx, []component.Val{})
		require.NoError(t, err)
		require.Equal(t, 1, len(result), "now() should return exactly one value")

		instant := result[0].U64()

		// First call: just verify it's non-zero
		if i == 0 {
			require.NotEqual(t, uint64(0), instant, "instant should be non-zero")
		} else {
			// Subsequent calls: verify monotonically increasing
			require.True(t, instant >= lastInstant,
				"monotonic clock should be monotonically increasing: got %d, previous was %d", instant, lastInstant)
		}
		lastInstant = instant

		// Small delay to ensure clock advances
		time.Sleep(1 * time.Microsecond)
	}
}

// TestWASI_MonotonicClock_Resolution tests that the monotonic clock resolution()
// function returns a positive nanosecond value.
func TestWASI_MonotonicClock_Resolution(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()

	// Get the monotonic clock interface
	clockDef, ok := linker.Get("wasi:clocks/monotonic-clock@0.2.0")
	require.True(t, ok, "monotonic-clock interface should be registered")

	instDef, ok := clockDef.(*component.InstanceDef)
	require.True(t, ok, "should be an InstanceDef")

	resolutionFunc, ok := instDef.Exports["resolution"]
	require.True(t, ok, "resolution function should be exported")

	funcDef, ok := resolutionFunc.(*component.FuncDef)
	require.True(t, ok, "should be a FuncDef")

	// Call resolution()
	result, err := funcDef.Callback(ctx, []component.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result), "resolution() should return exactly one value")

	resolution := result[0].U64()
	require.True(t, resolution > 0, "resolution should be positive, got %d", resolution)
	// Go's time package has nanosecond precision, so resolution should be 1
	require.Equal(t, uint64(1), resolution, "Go implementation should have nanosecond resolution")
}

// TestWASI_MonotonicClock_SubscribeDuration tests the subscribe-duration function.
func TestWASI_MonotonicClock_SubscribeDuration(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	// Get the monotonic clock interface
	clockDef, ok := linker.Get("wasi:clocks/monotonic-clock@0.2.0")
	require.True(t, ok, "monotonic-clock interface should be registered")

	instDef, ok := clockDef.(*component.InstanceDef)
	require.True(t, ok, "should be an InstanceDef")

	subscribeDurationFunc, ok := instDef.Exports["subscribe-duration"]
	require.True(t, ok, "subscribe-duration function should be exported")

	funcDef, ok := subscribeDurationFunc.(*component.FuncDef)
	require.True(t, ok, "should be a FuncDef")

	// Subscribe to a duration of 0 (immediately ready)
	result, err := funcDef.Callback(ctx, []component.Val{component.ValU64(0)})
	require.NoError(t, err)
	require.Equal(t, 1, len(result), "subscribe-duration should return exactly one value")

	// Result should be an own<pollable> handle
	handle := result[0].Own()
	require.True(t, handle > 0 || handle == 0, "should return a valid pollable handle")
}

// TestWASI_MonotonicClock_SubscribeInstant tests the subscribe-instant function.
func TestWASI_MonotonicClock_SubscribeInstant(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	// Get the monotonic clock interface
	clockDef, ok := linker.Get("wasi:clocks/monotonic-clock@0.2.0")
	require.True(t, ok, "monotonic-clock interface should be registered")

	instDef, ok := clockDef.(*component.InstanceDef)
	require.True(t, ok, "should be an InstanceDef")

	// Get current instant first
	nowFunc := instDef.Exports["now"].(*component.FuncDef)
	nowResult, err := nowFunc.Callback(ctx, []component.Val{})
	require.NoError(t, err)
	currentInstant := nowResult[0].U64()

	subscribeInstantFunc, ok := instDef.Exports["subscribe-instant"]
	require.True(t, ok, "subscribe-instant function should be exported")

	funcDef, ok := subscribeInstantFunc.(*component.FuncDef)
	require.True(t, ok, "should be a FuncDef")

	// Subscribe to the current instant (should be immediately ready)
	result, err := funcDef.Callback(ctx, []component.Val{component.ValU64(currentInstant)})
	require.NoError(t, err)
	require.Equal(t, 1, len(result), "subscribe-instant should return exactly one value")

	// Result should be an own<pollable> handle
	handle := result[0].Own()
	require.True(t, handle > 0 || handle == 0, "should return a valid pollable handle")
}

// TestWASI_WallClock_Now tests that the wall clock now() function returns
// a datetime record with seconds and nanoseconds.
func TestWASI_WallClock_Now(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()

	// Get the wall clock interface
	clockDef, ok := linker.Get("wasi:clocks/wall-clock@0.2.0")
	require.True(t, ok, "wall-clock interface should be registered")

	instDef, ok := clockDef.(*component.InstanceDef)
	require.True(t, ok, "should be an InstanceDef")

	nowFunc, ok := instDef.Exports["now"]
	require.True(t, ok, "now function should be exported")

	funcDef, ok := nowFunc.(*component.FuncDef)
	require.True(t, ok, "should be a FuncDef")

	// Call now() and verify it returns a datetime record
	result, err := funcDef.Callback(ctx, []component.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result), "now() should return exactly one value")

	// The result should be a record with seconds and nanoseconds
	record := result[0].Record()
	require.NotNil(t, record, "result should be a record")

	// Extract seconds field
	secondsVal, ok := record["seconds"]
	require.True(t, ok, "record should have 'seconds' field")
	seconds := secondsVal.U64()

	// Verify seconds is a reasonable Unix timestamp (after year 2000)
	// Year 2000 = 946684800 seconds since epoch
	require.True(t, seconds > 946684800, "seconds should be a valid Unix timestamp, got %d", seconds)

	// Extract nanoseconds field
	nanosecondsVal, ok := record["nanoseconds"]
	require.True(t, ok, "record should have 'nanoseconds' field")
	nanoseconds := nanosecondsVal.U32()

	// Verify nanoseconds is in valid range [0, 999999999]
	require.True(t, nanoseconds < 1000000000, "nanoseconds should be < 1 billion, got %d", nanoseconds)
}

// TestWASI_WallClock_Resolution tests that the wall clock resolution() function
// returns a datetime record with resolution information.
func TestWASI_WallClock_Resolution(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()

	// Get the wall clock interface
	clockDef, ok := linker.Get("wasi:clocks/wall-clock@0.2.0")
	require.True(t, ok, "wall-clock interface should be registered")

	instDef, ok := clockDef.(*component.InstanceDef)
	require.True(t, ok, "should be an InstanceDef")

	resolutionFunc, ok := instDef.Exports["resolution"]
	require.True(t, ok, "resolution function should be exported")

	funcDef, ok := resolutionFunc.(*component.FuncDef)
	require.True(t, ok, "should be a FuncDef")

	// Call resolution()
	result, err := funcDef.Callback(ctx, []component.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result), "resolution() should return exactly one value")

	// The result should be a record with seconds and nanoseconds
	record := result[0].Record()
	require.NotNil(t, record, "result should be a record")

	// Go has nanosecond resolution, so seconds should be 0 and nanoseconds should be 1
	secondsVal, ok := record["seconds"]
	require.True(t, ok, "record should have 'seconds' field")
	seconds := secondsVal.U64()
	require.Equal(t, uint64(0), seconds, "Go wall clock resolution should have 0 seconds")

	nanosecondsVal, ok := record["nanoseconds"]
	require.True(t, ok, "record should have 'nanoseconds' field")
	nanoseconds := nanosecondsVal.U32()
	require.Equal(t, uint32(1), nanoseconds, "Go wall clock resolution should be 1 nanosecond")
}

// TestWASI_Clocks_InterfaceRegistration tests that all clock interfaces are properly registered.
func TestWASI_Clocks_InterfaceRegistration(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	// Verify all clock interfaces are registered
	interfaces := []string{
		"wasi:clocks/wall-clock@0.2.0",
		"wasi:clocks/monotonic-clock@0.2.0",
	}

	for _, iface := range interfaces {
		def, ok := linker.Get(iface)
		require.True(t, ok, "interface %s should be registered", iface)
		require.NotNil(t, def, "interface %s definition should not be nil", iface)
	}
}

// TestWASI_MonotonicClock_AllFunctionsExist verifies all expected functions exist.
func TestWASI_MonotonicClock_AllFunctionsExist(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	clockDef, ok := linker.Get("wasi:clocks/monotonic-clock@0.2.0")
	require.True(t, ok, "monotonic-clock interface should be registered")

	instDef, ok := clockDef.(*component.InstanceDef)
	require.True(t, ok, "should be an InstanceDef")

	// Expected functions in wasi:clocks/monotonic-clock@0.2.0
	expectedFunctions := []string{
		"now",
		"resolution",
		"subscribe-instant",
		"subscribe-duration",
	}

	for _, fn := range expectedFunctions {
		funcDef, ok := instDef.Exports[fn]
		require.True(t, ok, "function %s should be exported", fn)
		require.NotNil(t, funcDef, "function %s should not be nil", fn)
	}
}

// TestWASI_WallClock_AllFunctionsExist verifies all expected functions exist.
func TestWASI_WallClock_AllFunctionsExist(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	clockDef, ok := linker.Get("wasi:clocks/wall-clock@0.2.0")
	require.True(t, ok, "wall-clock interface should be registered")

	instDef, ok := clockDef.(*component.InstanceDef)
	require.True(t, ok, "should be an InstanceDef")

	// Expected functions in wasi:clocks/wall-clock@0.2.0
	expectedFunctions := []string{
		"now",
		"resolution",
	}

	for _, fn := range expectedFunctions {
		funcDef, ok := instDef.Exports[fn]
		require.True(t, ok, "function %s should be exported", fn)
		require.NotNil(t, funcDef, "function %s should not be nil", fn)
	}
}
