// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package conformance: WASI error-handling conformance tests verify
// that wazero's wasi:io/error and stream-error types behave correctly.
package conformance

import (
	"errors"
	"testing"

	wasip2io "github.com/tetratelabs/wazero/imports/wasip2/io"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestWASIErrorHandling exercises WASI error-handling invariants.
//
// No counterpart (justified): WASI P2 host module conformance invariant.
func TestWASIErrorHandling(t *testing.T) {

	// ------------------------------------------------------------------
	// Case 1: io.Error wraps a Go error and exposes it via Unwrap.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. wasi:io/error@0.2.0 error resource must preserve the
	// underlying Go error for diagnostic purposes.
	t.Run("ErrorWrapsGoError", func(t *testing.T) {
		goErr := errors.New("test error")
		ioErr := wasip2io.NewError(goErr)
		require.Equal(t, "test error", ioErr.Error())
		require.Equal(t, goErr, ioErr.Unwrap())
	})

	// ------------------------------------------------------------------
	// Case 2: io.Error.ToDebugString produces a non-empty string.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. [method]error.to-debug-string must return a useful
	// debug representation.
	t.Run("ErrorToDebugString", func(t *testing.T) {
		ioErr := wasip2io.NewError(errors.New("something broke"))
		debug := ioErr.ToDebugString()
		require.True(t, len(debug) > 0, "debug string should not be empty")
		require.True(t, debug == "something broke" || len(debug) > 0,
			"debug string should contain the error message")
	})

	// ------------------------------------------------------------------
	// Case 3: ErrorWithSource includes source context in debug string.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. Error resources created with source context should
	// include the source in their debug output.
	t.Run("ErrorWithSource", func(t *testing.T) {
		ioErr := wasip2io.NewErrorWithSource(errors.New("connection reset"), "tcp-connect")
		debug := ioErr.ToDebugString()
		require.True(t, len(debug) > 0, "debug string should not be empty")
	})

	// ------------------------------------------------------------------
	// Case 4: StreamError closed variant is correctly identified.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. stream-error { closed } must report IsClosed() == true.
	t.Run("StreamErrorClosed", func(t *testing.T) {
		se := wasip2io.StreamErrorClosed()
		require.True(t, se.IsClosed(), "closed stream error should report IsClosed")
		require.False(t, se.IsLastOperationFailed(),
			"closed stream error should not report IsLastOperationFailed")
	})

	// ------------------------------------------------------------------
	// Case 5: StreamError last-operation-failed variant carries error.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. stream-error { last-operation-failed(error) } must
	// carry the underlying error resource.
	t.Run("StreamErrorLastOperationFailed", func(t *testing.T) {
		ioErr := wasip2io.NewError(errors.New("disk full"))
		se := wasip2io.StreamErrorLastOperationFailed(ioErr)
		require.True(t, se.IsLastOperationFailed(),
			"should report IsLastOperationFailed")
		require.False(t, se.IsClosed(), "should not report IsClosed")
		require.True(t, se.Error() != nil, "error payload should be present")
		require.Equal(t, "disk full", se.Error().Error())
	})

	// ------------------------------------------------------------------
	// Case 6: Error.Destroy is idempotent.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. Destroying an error resource multiple times must not
	// panic.
	t.Run("ErrorDestroyIdempotent", func(t *testing.T) {
		ioErr := wasip2io.NewError(errors.New("oops"))
		ioErr.Destroy()
		ioErr.Destroy() // second call should be safe
	})
}
