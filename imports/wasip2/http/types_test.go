// imports/wasip2/http/types_test.go

package http

import (
	"bytes"
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

// Tests for Destroy method

func TestFields_Destroy(t *testing.T) {
	fields := NewFields()
	fields.Set("Content-Type", [][]byte{[]byte("application/json")})
	fields.Set("Accept", [][]byte{[]byte("text/html")})

	// Verify fields are set
	require.Equal(t, 1, len(fields.Get("Content-Type")))

	// Destroy should clear the entries
	fields.Destroy()

	// Entries should be cleared
	require.Equal(t, 0, len(fields.Get("Content-Type")))
	require.Equal(t, 0, len(fields.Get("Accept")))
}

func TestFields_Destroy_Idempotent(t *testing.T) {
	fields := NewFields()
	fields.Set("X-Test", [][]byte{[]byte("value")})

	// Multiple calls to Destroy should be safe
	fields.Destroy()
	fields.Destroy()
	fields.Destroy()

	// Should still be destroyed
	require.Equal(t, 0, len(fields.Get("X-Test")))
}

func TestIncomingBody_Destroy(t *testing.T) {
	// Create a mock reader that tracks close
	reader := &mockReadCloser{data: []byte("test body")}
	body := NewIncomingBodyFromReader(reader)

	// Body should not be closed yet
	require.False(t, reader.closed)

	// Destroy should close the reader
	body.Destroy()

	// Reader should be closed
	require.True(t, reader.closed)
}

func TestIncomingBody_Destroy_NilReader(t *testing.T) {
	body := NewIncomingBody()

	// Destroy should not panic with nil reader
	body.Destroy()
}

func TestIncomingBody_Destroy_Idempotent(t *testing.T) {
	reader := &mockReadCloser{data: []byte("test")}
	body := NewIncomingBodyFromReader(reader)

	// Multiple calls to Destroy should be safe
	body.Destroy()
	body.Destroy()
	body.Destroy()

	require.True(t, reader.closed)
}

func TestOutgoingBody_Destroy(t *testing.T) {
	body := NewOutgoingBody()

	// Write some data
	stream, err := body.Write()
	require.NoError(t, err)
	stream.Write([]byte("hello world"))

	// Verify data was written
	require.True(t, len(body.Bytes()) > 0)

	// Destroy should clear the buffer
	body.Destroy()

	// Buffer should be cleared
	require.Nil(t, body.buffer)
}

func TestOutgoingBody_Destroy_Idempotent(t *testing.T) {
	body := NewOutgoingBody()
	body.Write() // Get the stream

	// Multiple calls to Destroy should be safe
	body.Destroy()
	body.Destroy()
	body.Destroy()

	require.Nil(t, body.buffer)
}

func TestOutgoingRequest_Destroy(t *testing.T) {
	headers := NewFields()
	headers.Set("Content-Type", [][]byte{[]byte("text/plain")})
	req := NewOutgoingRequest(headers)

	// Get the body
	body, err := req.Body()
	require.NoError(t, err)
	require.NotNil(t, body)

	// Destroy should clean up headers and body
	req.Destroy()

	// Headers should be cleared
	require.Equal(t, 0, len(headers.Get("Content-Type")))

	// Body buffer should be nil
	require.Nil(t, body.buffer)
}

func TestOutgoingRequest_Destroy_Idempotent(t *testing.T) {
	headers := NewFields()
	req := NewOutgoingRequest(headers)

	// Multiple calls to Destroy should be safe
	req.Destroy()
	req.Destroy()
	req.Destroy()
}

func TestOutgoingRequest_Destroy_NilFields(t *testing.T) {
	// Create request with nil headers (edge case)
	req := &OutgoingRequest{}

	// Destroy should not panic
	req.Destroy()
}

func TestIncomingResponse_Destroy(t *testing.T) {
	headers := NewFields()
	headers.Set("Server", [][]byte{[]byte("test-server")})

	// Create response with headers
	resp := NewIncomingResponse(200, headers)

	// Consume the body
	body, err := resp.Consume()
	require.NoError(t, err)
	require.NotNil(t, body)

	// Destroy should clean up headers and body
	resp.Destroy()

	// Headers should be cleared
	require.Equal(t, 0, len(headers.Get("Server")))
}

func TestIncomingResponse_Destroy_Idempotent(t *testing.T) {
	headers := NewFields()
	resp := NewIncomingResponse(200, headers)

	// Multiple calls to Destroy should be safe
	resp.Destroy()
	resp.Destroy()
	resp.Destroy()
}

func TestIncomingResponse_Destroy_NilFields(t *testing.T) {
	// Create response with nil headers (edge case)
	resp := &IncomingResponse{statusCode: 200}

	// Destroy should not panic
	resp.Destroy()
}

func TestFutureIncomingResponse_Destroy(t *testing.T) {
	future := NewFutureIncomingResponse()

	// Set a response
	headers := NewFields()
	headers.Set("X-Test", [][]byte{[]byte("value")})
	resp := NewIncomingResponse(200, headers)
	future.SetResponse(resp)

	// Destroy should mark consumed and destroy response
	future.Destroy()

	// Should be marked as consumed
	require.True(t, future.retrieved)

	// Response headers should be cleared
	require.Equal(t, 0, len(headers.Get("X-Test")))
}

func TestFutureIncomingResponse_Destroy_NoResponse(t *testing.T) {
	future := NewFutureIncomingResponse()

	// Destroy without setting response should not panic
	future.Destroy()

	require.True(t, future.retrieved)
}

func TestFutureIncomingResponse_Destroy_Idempotent(t *testing.T) {
	future := NewFutureIncomingResponse()
	resp := NewIncomingResponse(200, NewFields())
	future.SetResponse(resp)

	// Multiple calls to Destroy should be safe
	future.Destroy()
	future.Destroy()
	future.Destroy()
}

// mockReadCloser is a test helper that tracks Close calls
type mockReadCloser struct {
	data   []byte
	offset int
	closed bool
}

func (m *mockReadCloser) Read(p []byte) (n int, err error) {
	if m.offset >= len(m.data) {
		return 0, bytes.ErrTooLarge
	}
	n = copy(p, m.data[m.offset:])
	m.offset += n
	return n, nil
}

func (m *mockReadCloser) Close() error {
	m.closed = true
	return nil
}
