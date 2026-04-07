// imports/wasip2/clocks/monotonic.go

package clocks

import (
	"context"
	"time"

	wasip2io "github.com/tetratelabs/wazero/imports/wasip2/io"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/types"
)

// monotonicBase is the starting point for monotonic time measurement.
var monotonicBase = time.Now()

// MonotonicNow returns the current monotonic clock instant in nanoseconds.
func MonotonicNow() uint64 {
	return uint64(time.Since(monotonicBase).Nanoseconds())
}

// MonotonicResolution returns the resolution in nanoseconds.
// Go's time package has nanosecond precision.
func MonotonicResolution() uint64 {
	return 1
}

// SubscribeDuration creates a pollable that becomes ready after duration nanoseconds.
func SubscribeDuration(duration uint64) (readyFn func() bool, blockFn func()) {
	// For immediate readiness (duration 0), return ready pollable
	if duration == 0 {
		return func() bool { return true }, nil
	}

	// Create pollable that becomes ready after duration
	deadline := time.Now().Add(time.Duration(duration))
	return func() bool { return time.Now().After(deadline) },
		func() { time.Sleep(time.Until(deadline)) }
}

// SubscribeInstant creates a pollable that becomes ready at the given instant.
func SubscribeInstant(instant uint64) (readyFn func() bool, blockFn func()) {
	// Calculate when the instant occurs relative to our base
	targetTime := monotonicBase.Add(time.Duration(instant))

	// If already past, immediately ready
	if time.Now().After(targetTime) {
		return func() bool { return true }, nil
	}

	return func() bool { return time.Now().After(targetTime) },
		func() { time.Sleep(time.Until(targetTime)) }
}

func instantiateMonotonicClock(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:clocks/monotonic-clock@0.2.0")

	inst.FuncNoType("now", monotonicClockNow)
	inst.FuncNoType("resolution", monotonicClockResolution)
	inst.FuncNoType("subscribe-instant", monotonicClockSubscribeInstant)
	inst.FuncNoType("subscribe-duration", monotonicClockSubscribeDuration)

	return inst.SkipValidation().Build()
}

func monotonicClockNow(ctx context.Context, args []types.Val) ([]types.Val, error) {
	return []types.Val{types.ValU64(MonotonicNow())}, nil
}

func monotonicClockResolution(ctx context.Context, args []types.Val) ([]types.Val, error) {
	return []types.Val{types.ValU64(MonotonicResolution())}, nil
}

func monotonicClockSubscribeInstant(ctx context.Context, args []types.Val) ([]types.Val, error) {
	// args[0] is instant (u64)
	instant := args[0].U64()

	// Create ready and block functions for this instant
	readyFn, blockFn := SubscribeInstant(instant)

	// Create the pollable resource
	pollable := wasip2io.NewPollable(readyFn, blockFn)

	// Get the resource table from context
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		// Fallback - return placeholder handle if no resource table
		return []types.Val{types.ValOwn(0)}, nil
	}

	// Register the pollable in the resource table and return the handle
	handle := table.New(pollable, true)
	return []types.Val{types.ValOwn(uint32(handle))}, nil
}

func monotonicClockSubscribeDuration(ctx context.Context, args []types.Val) ([]types.Val, error) {
	// args[0] is duration (u64)
	duration := args[0].U64()

	// Create ready and block functions for this duration
	readyFn, blockFn := SubscribeDuration(duration)

	// Create the pollable resource
	pollable := wasip2io.NewPollable(readyFn, blockFn)

	// Get the resource table from context
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		// Fallback - return placeholder handle if no resource table
		return []types.Val{types.ValOwn(0)}, nil
	}

	// Register the pollable in the resource table and return the handle
	handle := table.New(pollable, true)
	return []types.Val{types.ValOwn(uint32(handle))}, nil
}
