// imports/wasip2/io/streams.go

package io

import (
	"context"
	"fmt"
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
	streamErrorKindLastOperationFailed streamErrorKind = iota // 0: last-operation-failed(error)
	streamErrorKindClosed                                     // 1: closed
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

// Destroy closes the reader if it implements io.Closer.
// This implements the Destroyable interface for resource cleanup.
// Safe to call multiple times (idempotent).
func (s *InputStream) Destroy() {
	s.Close()
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

// Destroy flushes (if Flusher), then closes (if io.Closer) the writer.
// This implements the Destroyable interface for resource cleanup.
// Safe to call multiple times (idempotent).
func (s *OutputStream) Destroy() {
	// Flush first if possible (ignore errors during cleanup)
	if f, ok := s.writer.(interface{ Flush() error }); ok && !s.closed {
		f.Flush()
	}
	s.Close()
}

func minUint64(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

// bytesToListU8 converts a Go byte slice to a component.Val list<u8>.
func bytesToListU8(data []byte) component.Val {
	vals := make([]component.Val, len(data))
	for i, b := range data {
		vals[i] = component.ValU8(b)
	}
	return component.ValList(vals)
}

// listU8ToBytes converts a component.Val list<u8> to a Go byte slice.
func listU8ToBytes(listVal component.Val) []byte {
	list := listVal.List()
	data := make([]byte, len(list))
	for i, v := range list {
		data[i] = v.U8()
	}
	return data
}

// streamErrorToResultVal converts a StreamError to a result error Val.
// The stream-error variant has two cases (per wasi:io/streams@0.2.0):
//   - last-operation-failed(error): discriminant 0, with Error resource handle
//   - closed: discriminant 1, no payload
func streamErrorToResultVal(ctx context.Context, err *StreamError) component.Val {
	if err.IsClosed() {
		// closed variant - just the variant discriminant "closed" with no payload
		return component.ValResultError(&component.Val{})
	}
	// last-operation-failed - needs to create an Error resource handle
	// For now, we create a simple variant representation
	// In a full implementation, we would create an Error resource in the table
	// and include its handle in the variant payload
	table := component.ResourceTableFromContext(ctx)
	if table != nil && err.Error() != nil {
		// Create an Error resource in the table
		handle := table.New(err.Error(), true)
		handleVal := component.ValOwn(uint32(handle))
		errVariant := component.ValVariant("last-operation-failed", &handleVal)
		return component.ValResultError(&errVariant)
	}
	// No table or no error, return closed
	closedVariant := component.ValVariant("closed", nil)
	return component.ValResultError(&closedVariant)
}

// getInputStream retrieves an InputStream from the ResourceTable using a borrow handle.
func getInputStream(ctx context.Context, handle uint32) (*InputStream, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return nil, fmt.Errorf("no resource table in context")
	}
	entry, err := table.Get(component.Handle(handle))
	if err != nil {
		return nil, fmt.Errorf("invalid handle %d: %w", handle, err)
	}
	stream, ok := entry.Rep.(*InputStream)
	if !ok {
		return nil, fmt.Errorf("handle %d is not an InputStream", handle)
	}
	return stream, nil
}

// getOutputStream retrieves an OutputStream from the ResourceTable using a borrow handle.
func getOutputStream(ctx context.Context, handle uint32) (*OutputStream, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return nil, fmt.Errorf("no resource table in context")
	}
	entry, err := table.Get(component.Handle(handle))
	if err != nil {
		return nil, fmt.Errorf("invalid handle %d: %w", handle, err)
	}
	stream, ok := entry.Rep.(*OutputStream)
	if !ok {
		return nil, fmt.Errorf("handle %d is not an OutputStream", handle)
	}
	return stream, nil
}

// createPollableHandle creates a new Pollable resource in the table and returns its handle.
func createPollableHandle(ctx context.Context, pollable *Pollable) component.Val {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		// No table, return placeholder handle 0
		return component.ValOwn(0)
	}
	handle := table.New(pollable, true)
	return component.ValOwn(uint32(handle))
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

// Host function implementations - use ResourceTable to look up stream handles

// inputStreamRead implements [method]input-stream.read
// Signature: func(self: borrow<input-stream>, len: u64) -> result<list<u8>, stream-error>
func inputStreamRead(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	maxLen := args[1].U64()

	stream, err := getInputStream(ctx, handle)
	if err != nil {
		return nil, err
	}

	data, streamErr := stream.Read(maxLen)
	if streamErr != nil {
		return []component.Val{streamErrorToResultVal(ctx, streamErr)}, nil
	}

	listVal := bytesToListU8(data)
	return []component.Val{component.ValResultOk(&listVal)}, nil
}

// inputStreamBlockingRead implements [method]input-stream.blocking-read
// Signature: func(self: borrow<input-stream>, len: u64) -> result<list<u8>, stream-error>
func inputStreamBlockingRead(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	maxLen := args[1].U64()

	stream, err := getInputStream(ctx, handle)
	if err != nil {
		return nil, err
	}

	data, streamErr := stream.BlockingRead(maxLen)
	if streamErr != nil {
		return []component.Val{streamErrorToResultVal(ctx, streamErr)}, nil
	}

	listVal := bytesToListU8(data)
	return []component.Val{component.ValResultOk(&listVal)}, nil
}

// inputStreamSkip implements [method]input-stream.skip
// Signature: func(self: borrow<input-stream>, len: u64) -> result<u64, stream-error>
func inputStreamSkip(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	length := args[1].U64()

	stream, err := getInputStream(ctx, handle)
	if err != nil {
		return nil, err
	}

	skipped, streamErr := stream.Skip(length)
	if streamErr != nil {
		return []component.Val{streamErrorToResultVal(ctx, streamErr)}, nil
	}

	result := component.ValU64(skipped)
	return []component.Val{component.ValResultOk(&result)}, nil
}

// inputStreamBlockingSkip implements [method]input-stream.blocking-skip
// Signature: func(self: borrow<input-stream>, len: u64) -> result<u64, stream-error>
func inputStreamBlockingSkip(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	length := args[1].U64()

	stream, err := getInputStream(ctx, handle)
	if err != nil {
		return nil, err
	}

	skipped, streamErr := stream.BlockingSkip(length)
	if streamErr != nil {
		return []component.Val{streamErrorToResultVal(ctx, streamErr)}, nil
	}

	result := component.ValU64(skipped)
	return []component.Val{component.ValResultOk(&result)}, nil
}

// inputStreamSubscribe implements [method]input-stream.subscribe
// Signature: func(self: borrow<input-stream>) -> own<pollable>
func inputStreamSubscribe(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()

	stream, err := getInputStream(ctx, handle)
	if err != nil {
		return nil, err
	}

	pollable := stream.Subscribe()
	return []component.Val{createPollableHandle(ctx, pollable)}, nil
}

// outputStreamCheckWrite implements [method]output-stream.check-write
// Signature: func(self: borrow<output-stream>) -> result<u64, stream-error>
func outputStreamCheckWrite(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()

	stream, err := getOutputStream(ctx, handle)
	if err != nil {
		return nil, err
	}

	size, streamErr := stream.CheckWrite()
	if streamErr != nil {
		return []component.Val{streamErrorToResultVal(ctx, streamErr)}, nil
	}

	result := component.ValU64(size)
	return []component.Val{component.ValResultOk(&result)}, nil
}

// outputStreamWrite implements [method]output-stream.write
// Signature: func(self: borrow<output-stream>, contents: list<u8>) -> result<_, stream-error>
func outputStreamWrite(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	contents := listU8ToBytes(args[1])

	stream, err := getOutputStream(ctx, handle)
	if err != nil {
		return nil, err
	}

	streamErr := stream.Write(contents)
	if streamErr != nil {
		return []component.Val{streamErrorToResultVal(ctx, streamErr)}, nil
	}

	return []component.Val{component.ValResultOk(nil)}, nil
}

// outputStreamBlockingWriteAndFlush implements [method]output-stream.blocking-write-and-flush
// Signature: func(self: borrow<output-stream>, contents: list<u8>) -> result<_, stream-error>
func outputStreamBlockingWriteAndFlush(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	contents := listU8ToBytes(args[1])

	stream, err := getOutputStream(ctx, handle)
	if err != nil {
		return nil, err
	}

	streamErr := stream.BlockingWriteAndFlush(contents)
	if streamErr != nil {
		return []component.Val{streamErrorToResultVal(ctx, streamErr)}, nil
	}

	return []component.Val{component.ValResultOk(nil)}, nil
}

// outputStreamFlush implements [method]output-stream.flush
// Signature: func(self: borrow<output-stream>) -> result<_, stream-error>
func outputStreamFlush(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()

	stream, err := getOutputStream(ctx, handle)
	if err != nil {
		return nil, err
	}

	streamErr := stream.Flush()
	if streamErr != nil {
		return []component.Val{streamErrorToResultVal(ctx, streamErr)}, nil
	}

	return []component.Val{component.ValResultOk(nil)}, nil
}

// outputStreamBlockingFlush implements [method]output-stream.blocking-flush
// Signature: func(self: borrow<output-stream>) -> result<_, stream-error>
func outputStreamBlockingFlush(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()

	stream, err := getOutputStream(ctx, handle)
	if err != nil {
		return nil, err
	}

	streamErr := stream.BlockingFlush()
	if streamErr != nil {
		return []component.Val{streamErrorToResultVal(ctx, streamErr)}, nil
	}

	return []component.Val{component.ValResultOk(nil)}, nil
}

// outputStreamSubscribe implements [method]output-stream.subscribe
// Signature: func(self: borrow<output-stream>) -> own<pollable>
func outputStreamSubscribe(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()

	stream, err := getOutputStream(ctx, handle)
	if err != nil {
		return nil, err
	}

	pollable := stream.Subscribe()
	return []component.Val{createPollableHandle(ctx, pollable)}, nil
}

// outputStreamWriteZeroes implements [method]output-stream.write-zeroes
// Signature: func(self: borrow<output-stream>, len: u64) -> result<_, stream-error>
func outputStreamWriteZeroes(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	length := args[1].U64()

	stream, err := getOutputStream(ctx, handle)
	if err != nil {
		return nil, err
	}

	streamErr := stream.WriteZeroes(length)
	if streamErr != nil {
		return []component.Val{streamErrorToResultVal(ctx, streamErr)}, nil
	}

	return []component.Val{component.ValResultOk(nil)}, nil
}

// outputStreamBlockingWriteZeroesAndFlush implements [method]output-stream.blocking-write-zeroes-and-flush
// Signature: func(self: borrow<output-stream>, len: u64) -> result<_, stream-error>
func outputStreamBlockingWriteZeroesAndFlush(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	length := args[1].U64()

	stream, err := getOutputStream(ctx, handle)
	if err != nil {
		return nil, err
	}

	streamErr := stream.BlockingWriteZeroesAndFlush(length)
	if streamErr != nil {
		return []component.Val{streamErrorToResultVal(ctx, streamErr)}, nil
	}

	return []component.Val{component.ValResultOk(nil)}, nil
}

// outputStreamSplice implements [method]output-stream.splice
// Signature: func(self: borrow<output-stream>, src: borrow<input-stream>, len: u64) -> result<u64, stream-error>
func outputStreamSplice(ctx context.Context, args []component.Val) ([]component.Val, error) {
	dstHandle := args[0].Borrow()
	srcHandle := args[1].Borrow()
	length := args[2].U64()

	dstStream, err := getOutputStream(ctx, dstHandle)
	if err != nil {
		return nil, err
	}

	srcStream, err := getInputStream(ctx, srcHandle)
	if err != nil {
		return nil, err
	}

	copied, streamErr := dstStream.Splice(srcStream, length)
	if streamErr != nil {
		return []component.Val{streamErrorToResultVal(ctx, streamErr)}, nil
	}

	result := component.ValU64(copied)
	return []component.Val{component.ValResultOk(&result)}, nil
}

// outputStreamBlockingSplice implements [method]output-stream.blocking-splice
// Signature: func(self: borrow<output-stream>, src: borrow<input-stream>, len: u64) -> result<u64, stream-error>
func outputStreamBlockingSplice(ctx context.Context, args []component.Val) ([]component.Val, error) {
	dstHandle := args[0].Borrow()
	srcHandle := args[1].Borrow()
	length := args[2].U64()

	dstStream, err := getOutputStream(ctx, dstHandle)
	if err != nil {
		return nil, err
	}

	srcStream, err := getInputStream(ctx, srcHandle)
	if err != nil {
		return nil, err
	}

	copied, streamErr := dstStream.BlockingSplice(srcStream, length)
	if streamErr != nil {
		return []component.Val{streamErrorToResultVal(ctx, streamErr)}, nil
	}

	result := component.ValU64(copied)
	return []component.Val{component.ValResultOk(&result)}, nil
}
