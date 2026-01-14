// imports/wasip2/io/streams.go

package io

import (
	"context"
	goio "io"

	"github.com/tetratelabs/wazero/internal/component"
)

// StreamError represents the stream-error variant type.
type StreamError struct {
	kind      streamErrorKind
	lastError *Error
}

type streamErrorKind uint8

const (
	streamErrorKindClosed streamErrorKind = iota
	streamErrorKindLastOperationFailed
)

// StreamErrorClosed creates a closed stream error.
func StreamErrorClosed() *StreamError {
	return &StreamError{kind: streamErrorKindClosed}
}

// StreamErrorLastOperationFailed creates a last-operation-failed error.
func StreamErrorLastOperationFailed(err *Error) *StreamError {
	return &StreamError{
		kind:      streamErrorKindLastOperationFailed,
		lastError: err,
	}
}

// IsClosed returns true if this is a closed stream error.
func (e *StreamError) IsClosed() bool {
	return e.kind == streamErrorKindClosed
}

// IsLastOperationFailed returns true if this error wraps an underlying error.
func (e *StreamError) IsLastOperationFailed() bool {
	return e.kind == streamErrorKindLastOperationFailed
}

// Error returns the underlying error (nil for closed errors).
func (e *StreamError) Error() *Error {
	return e.lastError
}

// InputStream wraps a Go io.Reader for wasi:io/streams.
type InputStream struct {
	reader goio.Reader
	closed bool
}

// NewInputStream creates an input stream from a Go reader.
func NewInputStream(r goio.Reader) *InputStream {
	return &InputStream{reader: r}
}

// Read reads up to len bytes from the stream.
func (s *InputStream) Read(maxLen uint64) ([]byte, *StreamError) {
	if s.closed {
		return nil, StreamErrorClosed()
	}
	buf := make([]byte, minUint64(maxLen, 64*1024)) // Cap at 64KB
	n, err := s.reader.Read(buf)
	if err == goio.EOF {
		return buf[:n], nil
	}
	if err != nil {
		return nil, StreamErrorLastOperationFailed(NewError(err))
	}
	return buf[:n], nil
}

// BlockingRead reads up to len bytes, blocking until data is available.
func (s *InputStream) BlockingRead(maxLen uint64) ([]byte, *StreamError) {
	// For simple readers, blocking-read behaves the same as read
	return s.Read(maxLen)
}

// Skip skips up to len bytes from the stream.
func (s *InputStream) Skip(length uint64) (uint64, *StreamError) {
	if s.closed {
		return 0, StreamErrorClosed()
	}
	// Read and discard bytes. This is the most portable approach for streams.
	buf := make([]byte, minUint64(length, 64*1024))
	total := uint64(0)
	for total < length {
		toRead := minUint64(length-total, uint64(len(buf)))
		n, err := s.reader.Read(buf[:toRead])
		total += uint64(n)
		if err == goio.EOF {
			break
		}
		if err != nil {
			return total, StreamErrorLastOperationFailed(NewError(err))
		}
	}
	return total, nil
}

// BlockingSkip skips up to len bytes, blocking until complete.
func (s *InputStream) BlockingSkip(length uint64) (uint64, *StreamError) {
	return s.Skip(length)
}

// Subscribe returns a pollable for this stream.
func (s *InputStream) Subscribe() *Pollable {
	return NewReadyPollable() // Simple readers are always ready
}

// Close marks the stream as closed.
func (s *InputStream) Close() {
	s.closed = true
	if closer, ok := s.reader.(goio.Closer); ok {
		closer.Close()
	}
}

// IsClosed returns true if the stream has been closed.
func (s *InputStream) IsClosed() bool {
	return s.closed
}

// OutputStream wraps a Go io.Writer for wasi:io/streams.
type OutputStream struct {
	writer goio.Writer
	closed bool
}

// NewOutputStream creates an output stream from a Go writer.
func NewOutputStream(w goio.Writer) *OutputStream {
	return &OutputStream{writer: w}
}

// CheckWrite returns the number of bytes that can be written.
func (s *OutputStream) CheckWrite() (uint64, *StreamError) {
	if s.closed {
		return 0, StreamErrorClosed()
	}
	return 64 * 1024, nil // Allow up to 64KB per write
}

// Write writes data to the stream.
func (s *OutputStream) Write(data []byte) *StreamError {
	if s.closed {
		return StreamErrorClosed()
	}
	_, err := s.writer.Write(data)
	if err != nil {
		return StreamErrorLastOperationFailed(NewError(err))
	}
	return nil
}

// BlockingWriteAndFlush writes data and flushes, blocking until complete.
func (s *OutputStream) BlockingWriteAndFlush(data []byte) *StreamError {
	if err := s.Write(data); err != nil {
		return err
	}
	return s.Flush()
}

// Flush flushes the stream.
func (s *OutputStream) Flush() *StreamError {
	if s.closed {
		return StreamErrorClosed()
	}
	if f, ok := s.writer.(interface{ Flush() error }); ok {
		if err := f.Flush(); err != nil {
			return StreamErrorLastOperationFailed(NewError(err))
		}
	}
	return nil
}

// BlockingFlush flushes the stream, blocking until complete.
func (s *OutputStream) BlockingFlush() *StreamError {
	return s.Flush()
}

// Subscribe returns a pollable for this stream.
func (s *OutputStream) Subscribe() *Pollable {
	return NewReadyPollable()
}

// WriteZeroes writes len zero bytes to the stream.
func (s *OutputStream) WriteZeroes(length uint64) *StreamError {
	if s.closed {
		return StreamErrorClosed()
	}
	zeros := make([]byte, minUint64(length, 64*1024))
	remaining := length
	for remaining > 0 {
		toWrite := minUint64(remaining, uint64(len(zeros)))
		_, err := s.writer.Write(zeros[:toWrite])
		if err != nil {
			return StreamErrorLastOperationFailed(NewError(err))
		}
		remaining -= toWrite
	}
	return nil
}

// BlockingWriteZeroesAndFlush writes zeros and flushes, blocking until complete.
func (s *OutputStream) BlockingWriteZeroesAndFlush(length uint64) *StreamError {
	if err := s.WriteZeroes(length); err != nil {
		return err
	}
	return s.Flush()
}

// Splice copies data from an input stream to this output stream.
func (s *OutputStream) Splice(src *InputStream, length uint64) (uint64, *StreamError) {
	if s.closed {
		return 0, StreamErrorClosed()
	}
	if src.closed {
		return 0, StreamErrorClosed()
	}
	data, err := src.Read(length)
	if err != nil {
		return 0, err
	}
	if writeErr := s.Write(data); writeErr != nil {
		return 0, writeErr
	}
	return uint64(len(data)), nil
}

// BlockingSplice copies data from an input stream, blocking until complete.
func (s *OutputStream) BlockingSplice(src *InputStream, length uint64) (uint64, *StreamError) {
	return s.Splice(src, length)
}

// Close marks the stream as closed.
func (s *OutputStream) Close() {
	s.closed = true
	if closer, ok := s.writer.(goio.Closer); ok {
		closer.Close()
	}
}

// IsClosed returns true if the stream has been closed.
func (s *OutputStream) IsClosed() bool {
	return s.closed
}

func minUint64(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

func instantiateStreams(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:io/streams@0.2.0")

	// input-stream resource
	inst.Resource("input-stream", func(rep uint32) {})
	inst.FuncNoType("[method]input-stream.read", inputStreamRead)
	inst.FuncNoType("[method]input-stream.blocking-read", inputStreamBlockingRead)
	inst.FuncNoType("[method]input-stream.skip", inputStreamSkip)
	inst.FuncNoType("[method]input-stream.blocking-skip", inputStreamBlockingSkip)
	inst.FuncNoType("[method]input-stream.subscribe", inputStreamSubscribe)

	// output-stream resource
	inst.Resource("output-stream", func(rep uint32) {})
	inst.FuncNoType("[method]output-stream.check-write", outputStreamCheckWrite)
	inst.FuncNoType("[method]output-stream.write", outputStreamWrite)
	inst.FuncNoType("[method]output-stream.blocking-write-and-flush", outputStreamBlockingWriteAndFlush)
	inst.FuncNoType("[method]output-stream.flush", outputStreamFlush)
	inst.FuncNoType("[method]output-stream.blocking-flush", outputStreamBlockingFlush)
	inst.FuncNoType("[method]output-stream.subscribe", outputStreamSubscribe)
	inst.FuncNoType("[method]output-stream.write-zeroes", outputStreamWriteZeroes)
	inst.FuncNoType("[method]output-stream.blocking-write-zeroes-and-flush", outputStreamBlockingWriteZeroesAndFlush)
	inst.FuncNoType("[method]output-stream.splice", outputStreamSplice)
	inst.FuncNoType("[method]output-stream.blocking-splice", outputStreamBlockingSplice)

	return inst.Build()
}

// Host function implementations - return success results as placeholders
// Full implementations will integrate with ResourceTable for handle lookup

func inputStreamRead(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Returns result<list<u8>, stream-error> - ok with empty list
	emptyList := component.ValList([]component.Val{})
	return []component.Val{component.ValResultOk(&emptyList)}, nil
}

func inputStreamBlockingRead(ctx context.Context, args []component.Val) ([]component.Val, error) {
	emptyList := component.ValList([]component.Val{})
	return []component.Val{component.ValResultOk(&emptyList)}, nil
}

func inputStreamSkip(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Returns result<u64, stream-error> - ok with 0
	zero := component.ValU64(0)
	return []component.Val{component.ValResultOk(&zero)}, nil
}

func inputStreamBlockingSkip(ctx context.Context, args []component.Val) ([]component.Val, error) {
	zero := component.ValU64(0)
	return []component.Val{component.ValResultOk(&zero)}, nil
}

func inputStreamSubscribe(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Returns own<pollable>
	return []component.Val{component.ValOwn(0)}, nil
}

func outputStreamCheckWrite(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Returns result<u64, stream-error> - ok with 64KB
	size := component.ValU64(64 * 1024)
	return []component.Val{component.ValResultOk(&size)}, nil
}

func outputStreamWrite(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Returns result<_, stream-error> - ok with unit
	return []component.Val{component.ValResultOk(nil)}, nil
}

func outputStreamBlockingWriteAndFlush(ctx context.Context, args []component.Val) ([]component.Val, error) {
	return []component.Val{component.ValResultOk(nil)}, nil
}

func outputStreamFlush(ctx context.Context, args []component.Val) ([]component.Val, error) {
	return []component.Val{component.ValResultOk(nil)}, nil
}

func outputStreamBlockingFlush(ctx context.Context, args []component.Val) ([]component.Val, error) {
	return []component.Val{component.ValResultOk(nil)}, nil
}

func outputStreamSubscribe(ctx context.Context, args []component.Val) ([]component.Val, error) {
	return []component.Val{component.ValOwn(0)}, nil
}

func outputStreamWriteZeroes(ctx context.Context, args []component.Val) ([]component.Val, error) {
	return []component.Val{component.ValResultOk(nil)}, nil
}

func outputStreamBlockingWriteZeroesAndFlush(ctx context.Context, args []component.Val) ([]component.Val, error) {
	return []component.Val{component.ValResultOk(nil)}, nil
}

func outputStreamSplice(ctx context.Context, args []component.Val) ([]component.Val, error) {
	zero := component.ValU64(0)
	return []component.Val{component.ValResultOk(&zero)}, nil
}

func outputStreamBlockingSplice(ctx context.Context, args []component.Val) ([]component.Val, error) {
	zero := component.ValU64(0)
	return []component.Val{component.ValResultOk(&zero)}, nil
}
