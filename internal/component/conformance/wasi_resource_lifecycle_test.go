// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package conformance: WASI resource-lifecycle conformance tests verify
// that wazero's WASI host module resources support correct creation,
// use, and destruction patterns.
package conformance

import (
	"bytes"
	"errors"
	"testing"

	wasip2io "github.com/tetratelabs/wazero/imports/wasip2/io"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestWASIResourceLifecycle exercises WASI resource lifecycle invariants.
//
// No counterpart (justified): WASI P2 host module conformance invariant.
func TestWASIResourceLifecycle(t *testing.T) {

	// ------------------------------------------------------------------
	// Case 1: InputStream create-use-destroy lifecycle.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. An input-stream resource must support create, read,
	// and destroy without errors.
	t.Run("InputStreamLifecycle", func(t *testing.T) {
		r := bytes.NewReader([]byte("lifecycle"))
		stream := wasip2io.NewInputStream(r)
		require.False(t, stream.IsClosed(), "new stream should be open")

		data, streamErr := stream.Read(1024)
		require.True(t, streamErr == nil, "read should succeed")
		require.Equal(t, "lifecycle", string(data))

		stream.Destroy()
		require.True(t, stream.IsClosed(), "destroyed stream should be closed")
	})

	// ------------------------------------------------------------------
	// Case 2: OutputStream create-use-destroy lifecycle.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. An output-stream resource must support create, write,
	// and destroy without errors.
	t.Run("OutputStreamLifecycle", func(t *testing.T) {
		var buf bytes.Buffer
		stream := wasip2io.NewOutputStream(&buf)
		require.False(t, stream.IsClosed(), "new stream should be open")

		streamErr := stream.Write([]byte("hello"))
		require.True(t, streamErr == nil, "write should succeed")
		require.Equal(t, "hello", buf.String())

		stream.Destroy()
		require.True(t, stream.IsClosed(), "destroyed stream should be closed")
	})

	// ------------------------------------------------------------------
	// Case 3: Destroy is idempotent for InputStream.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. Calling destroy multiple times on an input-stream must
	// not panic.
	t.Run("InputStreamDestroyIdempotent", func(t *testing.T) {
		stream := wasip2io.NewInputStream(bytes.NewReader([]byte("data")))
		stream.Destroy()
		stream.Destroy() // second call should be safe
		require.True(t, stream.IsClosed())
	})

	// ------------------------------------------------------------------
	// Case 4: Destroy is idempotent for OutputStream.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. Calling destroy multiple times on an output-stream
	// must not panic.
	t.Run("OutputStreamDestroyIdempotent", func(t *testing.T) {
		stream := wasip2io.NewOutputStream(&bytes.Buffer{})
		stream.Destroy()
		stream.Destroy() // second call should be safe
		require.True(t, stream.IsClosed())
	})

	// ------------------------------------------------------------------
	// Case 5: Pollable register-unregister lifecycle.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. Registered pollables must be retrievable by ID;
	// after unregistration, retrieval must return nil.
	t.Run("PollableRegistryLifecycle", func(t *testing.T) {
		p := wasip2io.NewReadyPollable()
		id := wasip2io.RegisterPollable(p)
		retrieved := wasip2io.GetPollable(id)
		require.True(t, retrieved != nil, "registered pollable should be retrievable")
		require.True(t, retrieved.Ready(), "retrieved pollable should be ready")

		wasip2io.UnregisterPollable(id)
		afterUnreg := wasip2io.GetPollable(id)
		require.True(t, afterUnreg == nil, "unregistered pollable should return nil")
	})

	// ------------------------------------------------------------------
	// Case 6: Error register-unregister lifecycle.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. Registered error resources must be retrievable by ID;
	// after unregistration, retrieval must return nil.
	t.Run("ErrorRegistryLifecycle", func(t *testing.T) {
		ioErr := wasip2io.NewError(errors.New("test"))
		id := wasip2io.RegisterError(ioErr)
		retrieved := wasip2io.GetError(id)
		require.True(t, retrieved != nil, "registered error should be retrievable")
		require.Equal(t, "test", retrieved.Error())

		wasip2io.UnregisterError(id)
		afterUnreg := wasip2io.GetError(id)
		require.True(t, afterUnreg == nil, "unregistered error should return nil")
	})
}
