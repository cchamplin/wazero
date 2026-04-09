// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package conformance: WASI streams conformance tests verify that
// wazero's input/output stream implementations correctly read, write,
// and handle closed-stream errors.
package conformance

import (
	"bytes"
	"strings"
	"testing"

	wasip2io "github.com/tetratelabs/wazero/imports/wasip2/io"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestWASIStreams exercises the wasi:io/streams host module API surface.
//
// No counterpart (justified): WASI P2 host module conformance invariant.
func TestWASIStreams(t *testing.T) {

	// ------------------------------------------------------------------
	// Case 1: InputStream.Read returns data from the underlying reader.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. [method]input-stream.read must return data from the
	// wrapped Go io.Reader.
	t.Run("InputStreamRead", func(t *testing.T) {
		r := strings.NewReader("hello world")
		stream := wasip2io.NewInputStream(r)
		data, streamErr := stream.Read(1024)
		require.True(t, streamErr == nil, "read should succeed")
		require.Equal(t, "hello world", string(data))
	})

	// ------------------------------------------------------------------
	// Case 2: InputStream.Read on a closed stream returns closed error.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. Reading from a closed input stream must return the
	// stream-error closed variant.
	t.Run("InputStreamReadClosed", func(t *testing.T) {
		r := strings.NewReader("data")
		stream := wasip2io.NewInputStream(r)
		stream.Close()
		require.True(t, stream.IsClosed(), "stream should be closed")
		_, streamErr := stream.Read(1024)
		require.True(t, streamErr != nil, "read on closed stream should error")
		require.True(t, streamErr.IsClosed(), "error should be closed variant")
	})

	// ------------------------------------------------------------------
	// Case 3: OutputStream.Write writes data to the underlying writer.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. [method]output-stream.write must forward bytes to the
	// wrapped Go io.Writer.
	t.Run("OutputStreamWrite", func(t *testing.T) {
		var buf bytes.Buffer
		stream := wasip2io.NewOutputStream(&buf)
		streamErr := stream.Write([]byte("hello"))
		require.True(t, streamErr == nil, "write should succeed")
		require.Equal(t, "hello", buf.String())
	})

	// ------------------------------------------------------------------
	// Case 4: OutputStream.CheckWrite on open stream returns capacity.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. check-write on an open stream must return a positive
	// value indicating available write capacity.
	t.Run("OutputStreamCheckWrite", func(t *testing.T) {
		var buf bytes.Buffer
		stream := wasip2io.NewOutputStream(&buf)
		capacity, streamErr := stream.CheckWrite()
		require.True(t, streamErr == nil, "check-write should succeed")
		require.True(t, capacity > 0, "capacity should be positive: got %d", capacity)
	})

	// ------------------------------------------------------------------
	// Case 5: OutputStream.Write on a closed stream returns closed error.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. Writing to a closed output stream must return the
	// stream-error closed variant.
	t.Run("OutputStreamWriteClosed", func(t *testing.T) {
		var buf bytes.Buffer
		stream := wasip2io.NewOutputStream(&buf)
		stream.Close()
		require.True(t, stream.IsClosed(), "stream should be closed")
		streamErr := stream.Write([]byte("data"))
		require.True(t, streamErr != nil, "write on closed stream should error")
		require.True(t, streamErr.IsClosed(), "error should be closed variant")
	})

	// ------------------------------------------------------------------
	// Case 6: InputStream.Subscribe returns a ready pollable.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. subscribe on a simple reader-backed input stream must
	// return an immediately-ready pollable (simple readers are always
	// ready).
	t.Run("InputStreamSubscribe", func(t *testing.T) {
		r := strings.NewReader("data")
		stream := wasip2io.NewInputStream(r)
		pollable := stream.Subscribe()
		require.True(t, pollable.Ready(), "input stream pollable should be ready")
	})
}
