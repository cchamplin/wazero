// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package conformance: WASI poll conformance tests verify that
// wazero's poll host module correctly tracks pollable readiness
// and blocking behavior.
package conformance

import (
	"testing"

	wasip2io "github.com/tetratelabs/wazero/imports/wasip2/io"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestWASIPoll exercises the wasi:io/poll host module API surface.
//
// No counterpart (justified): WASI P2 host module conformance invariant.
func TestWASIPoll(t *testing.T) {

	// ------------------------------------------------------------------
	// Case 1: NewReadyPollable is immediately ready.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. A pollable created via NewReadyPollable must report
	// Ready() == true immediately.
	t.Run("ReadyPollableIsReady", func(t *testing.T) {
		p := wasip2io.NewReadyPollable()
		require.True(t, p.Ready(), "ready pollable should be immediately ready")
	})

	// ------------------------------------------------------------------
	// Case 2: Callback-based pollable reports readiness correctly.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. A pollable created with a readyFn that returns true
	// should report Ready() == true.
	t.Run("CallbackPollableReady", func(t *testing.T) {
		p := wasip2io.NewPollable(func() bool { return true }, nil)
		require.True(t, p.Ready(), "callback pollable with true readyFn should be ready")
	})

	// ------------------------------------------------------------------
	// Case 3: Callback-based pollable reports not ready when readyFn
	// returns false.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. A pollable whose readyFn returns false should report
	// Ready() == false.
	t.Run("CallbackPollableNotReady", func(t *testing.T) {
		p := wasip2io.NewPollable(func() bool { return false }, nil)
		require.False(t, p.Ready(), "callback pollable with false readyFn should not be ready")
	})

	// ------------------------------------------------------------------
	// Case 4: Channel-based pollable transitions via SetReady.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. NewChannelPollable starts not-ready; after SetReady()
	// it must report IsReady() == true.
	t.Run("ChannelPollableSetReady", func(t *testing.T) {
		p := wasip2io.NewChannelPollable()
		require.False(t, p.IsReady(), "channel pollable should start not ready")
		p.SetReady()
		require.True(t, p.IsReady(), "channel pollable should be ready after SetReady")
		require.True(t, p.Ready(), "Ready() should agree with IsReady()")
	})

	// ------------------------------------------------------------------
	// Case 5: SetReady is idempotent.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. Calling SetReady multiple times must not panic.
	t.Run("SetReadyIdempotent", func(t *testing.T) {
		p := wasip2io.NewChannelPollable()
		p.SetReady()
		p.SetReady() // second call should be a no-op
		require.True(t, p.IsReady())
	})

	// ------------------------------------------------------------------
	// Case 6: Block on a ready pollable returns immediately.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. Blocking on an already-ready pollable must return
	// without deadlocking.
	t.Run("BlockOnReady", func(t *testing.T) {
		p := wasip2io.NewReadyPollable()
		// This should return immediately without blocking
		p.Block()
	})
}
