// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package conformance: WASI clocks conformance tests verify that
// wazero's clocks host module returns sensible values from the
// monotonic and wall clock APIs.
package conformance

import (
	"testing"
	"time"

	"github.com/tetratelabs/wazero/imports/wasip2/clocks"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestWASIClocks exercises the wasi:clocks host module API surface.
//
// No counterpart (justified): WASI P2 host module conformance invariant.
func TestWASIClocks(t *testing.T) {

	// ------------------------------------------------------------------
	// Case 1: MonotonicNow returns a positive value.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. wasi:clocks/monotonic-clock@0.2.0 now() must return
	// the elapsed time since an arbitrary epoch.
	t.Run("MonotonicNow", func(t *testing.T) {
		now := clocks.MonotonicNow()
		require.True(t, now > 0, "monotonic clock should return positive value")
	})

	// ------------------------------------------------------------------
	// Case 2: MonotonicNow is monotonically increasing.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. Successive calls to now() must not decrease.
	t.Run("MonotonicNowIncreasing", func(t *testing.T) {
		first := clocks.MonotonicNow()
		time.Sleep(time.Millisecond)
		second := clocks.MonotonicNow()
		require.True(t, second > first,
			"monotonic clock should increase: first=%d, second=%d", first, second)
	})

	// ------------------------------------------------------------------
	// Case 3: MonotonicResolution returns 1 (nanosecond precision).
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. Go's time package has nanosecond precision, so the
	// resolution should be 1.
	t.Run("MonotonicResolution", func(t *testing.T) {
		res := clocks.MonotonicResolution()
		require.Equal(t, uint64(1), res, "resolution should be 1ns")
	})

	// ------------------------------------------------------------------
	// Case 4: WallClockNow returns a reasonable datetime.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. wasi:clocks/wall-clock@0.2.0 now() must return a
	// datetime record with seconds since Unix epoch.
	t.Run("WallClockNow", func(t *testing.T) {
		dt := clocks.WallClockNow()
		// Should be after 2020-01-01 (1577836800)
		require.True(t, dt.Seconds > 1577836800,
			"wall clock seconds should be after 2020: got %d", dt.Seconds)
		// Nanoseconds should be in valid range [0, 999999999]
		require.True(t, dt.Nanoseconds < 1_000_000_000,
			"nanoseconds should be < 1e9: got %d", dt.Nanoseconds)
	})

	// ------------------------------------------------------------------
	// Case 5: WallClockResolution returns nanosecond precision.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. Go's time.Time has nanosecond precision.
	t.Run("WallClockResolution", func(t *testing.T) {
		res := clocks.WallClockResolution()
		require.Equal(t, uint64(0), res.Seconds, "resolution seconds should be 0")
		require.Equal(t, uint32(1), res.Nanoseconds, "resolution nanoseconds should be 1")
	})

	// ------------------------------------------------------------------
	// Case 6: SubscribeDuration with zero returns immediately ready.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. subscribe-duration(0) must produce a pollable that is
	// immediately ready.
	t.Run("SubscribeDurationZero", func(t *testing.T) {
		readyFn, _ := clocks.SubscribeDuration(0)
		require.True(t, readyFn(), "zero-duration pollable should be immediately ready")
	})
}
