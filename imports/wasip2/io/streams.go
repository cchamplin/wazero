// imports/wasip2/io/streams.go

// WIT source of truth: debug-vendored/WASI/proposals/io/wit/streams.wit
// Package version: wasi:io@0.2.9 (wazero targets wasi:io@0.2.0)
//
package io

import (
	"context"
	"fmt"
	goio "io"
	"sync"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
)

// Thread-safe registry for host-side InputStream objects keyed by uint32 ID.
var (
	inputStreamRegistryMu sync.Mutex
	inputStreamRegistry   []*InputStream
	inputStreamFreelist   []uint32
)

func RegisterInputStream(s *InputStream) uint32 {
	inputStreamRegistryMu.Lock()
	defer inputStreamRegistryMu.Unlock()
	if n := len(inputStreamFreelist); n > 0 {
		id := inputStreamFreelist[n-1]
		inputStreamFreelist = inputStreamFreelist[:n-1]
		inputStreamRegistry[id] = s
		return id
	}
	id := uint32(len(inputStreamRegistry))
	inputStreamRegistry = append(inputStreamRegistry, s)
	return id
}

func GetInputStream(id uint32) *InputStream {
	inputStreamRegistryMu.Lock()
	defer inputStreamRegistryMu.Unlock()
	if int(id) >= len(inputStreamRegistry) {
		return nil
	}
	return inputStreamRegistry[id]
}

func UnregisterInputStream(id uint32) {
	inputStreamRegistryMu.Lock()
	defer inputStreamRegistryMu.Unlock()
	if int(id) < len(inputStreamRegistry) && inputStreamRegistry[id] != nil {
		inputStreamRegistry[id] = nil
		inputStreamFreelist = append(inputStreamFreelist, id)
	}
}

// Thread-safe registry for host-side OutputStream objects keyed by uint32 ID.
var (
	outputStreamRegistryMu sync.Mutex
	outputStreamRegistry   []*OutputStream
	outputStreamFreelist   []uint32
)

func RegisterOutputStream(s *OutputStream) uint32 {
	outputStreamRegistryMu.Lock()
	defer outputStreamRegistryMu.Unlock()
	if n := len(outputStreamFreelist); n > 0 {
		id := outputStreamFreelist[n-1]
		outputStreamFreelist = outputStreamFreelist[:n-1]
		outputStreamRegistry[id] = s
		return id
	}
	id := uint32(len(outputStreamRegistry))
	outputStreamRegistry = append(outputStreamRegistry, s)
	return id
}

func GetOutputStream(id uint32) *OutputStream {
	outputStreamRegistryMu.Lock()
	defer outputStreamRegistryMu.Unlock()
	if int(id) >= len(outputStreamRegistry) {
		return nil
	}
	return outputStreamRegistry[id]
}

func UnregisterOutputStream(id uint32) {
	outputStreamRegistryMu.Lock()
	defer outputStreamRegistryMu.Unlock()
	if int(id) < len(outputStreamRegistry) && outputStreamRegistry[id] != nil {
		outputStreamRegistry[id] = nil
		outputStreamFreelist = append(outputStreamFreelist, id)
	}
}

// Thread-safe registry for host-side Pollable objects keyed by uint32 ID.
var (
	pollableRegistryMu sync.Mutex
	pollableRegistry   []*Pollable
	pollableFreelist   []uint32
)

func RegisterPollable(p *Pollable) uint32 {
	pollableRegistryMu.Lock()
	defer pollableRegistryMu.Unlock()
	if n := len(pollableFreelist); n > 0 {
		id := pollableFreelist[n-1]
		pollableFreelist = pollableFreelist[:n-1]
		pollableRegistry[id] = p
		return id
	}
	id := uint32(len(pollableRegistry))
	pollableRegistry = append(pollableRegistry, p)
	return id
}

func GetPollable(id uint32) *Pollable {
	pollableRegistryMu.Lock()
	defer pollableRegistryMu.Unlock()
	if int(id) >= len(pollableRegistry) {
		return nil
	}
	return pollableRegistry[id]
}

func UnregisterPollable(id uint32) {
	pollableRegistryMu.Lock()
	defer pollableRegistryMu.Unlock()
	if int(id) < len(pollableRegistry) && pollableRegistry[id] != nil {
		pollableRegistry[id] = nil
		pollableFreelist = append(pollableFreelist, id)
	}
}

// Thread-safe registry for host-side Error objects keyed by uint32 ID.
var (
	errorRegistryMu sync.Mutex
	errorRegistry   []*Error
	errorFreelist   []uint32
)

func RegisterError(e *Error) uint32 {
	errorRegistryMu.Lock()
	defer errorRegistryMu.Unlock()
	if n := len(errorFreelist); n > 0 {
		id := errorFreelist[n-1]
		errorFreelist = errorFreelist[:n-1]
		errorRegistry[id] = e
		return id
	}
	id := uint32(len(errorRegistry))
	errorRegistry = append(errorRegistry, e)
	return id
}

func GetError(id uint32) *Error {
	errorRegistryMu.Lock()
	defer errorRegistryMu.Unlock()
	if int(id) >= len(errorRegistry) {
		return nil
	}
	return errorRegistry[id]
}

func UnregisterError(id uint32) {
	errorRegistryMu.Lock()
	defer errorRegistryMu.Unlock()
	if int(id) < len(errorRegistry) && errorRegistry[id] != nil {
		errorRegistry[id] = nil
		errorFreelist = append(errorFreelist, id)
	}
}

// Host-managed resource type singletons. One *ResourceType per host
// resource kind. Impl is nil because these resources are host-owned;
// destruction flows through ResourceType.HostDestructor.
var (
	inputStreamResourceType = &runtime.ResourceType{
		HostDestructor: func(rep uint32) error {
			if s := GetInputStream(rep); s != nil {
				s.Destroy()
			}
			UnregisterInputStream(rep)
			return nil
		},
	}
	outputStreamResourceType = &runtime.ResourceType{
		HostDestructor: func(rep uint32) error {
			if s := GetOutputStream(rep); s != nil {
				s.Destroy()
			}
			UnregisterOutputStream(rep)
			return nil
		},
	}
	pollableResourceType = &runtime.ResourceType{
		HostDestructor: func(rep uint32) error {
			UnregisterPollable(rep)
			return nil
		},
	}
	errorResourceType = &runtime.ResourceType{
		HostDestructor: func(rep uint32) error {
			if e := GetError(rep); e != nil {
				e.Destroy()
			}
			UnregisterError(rep)
			return nil
		},
	}
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
// This handles resource cleanup.
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
// This handles resource cleanup.
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

// bytesToListU8 converts a Go byte slice to a types.Val list<u8>.
func bytesToListU8(data []byte) types.Val {
	vals := make([]types.Val, len(data))
	for i, b := range data {
		vals[i] = types.ValU8(b)
	}
	return types.ValList(vals)
}

// listU8ToBytes converts a types.Val list<u8> to a Go byte slice.
func listU8ToBytes(listVal types.Val) []byte {
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
func streamErrorToResultVal(ctx context.Context, err *StreamError) types.Val {
	if err.IsClosed() {
		// closed variant - just the variant discriminant "closed" with no payload
		return types.ValResultError(&types.Val{})
	}
	// last-operation-failed - needs to create an Error resource handle
	// For now, we create a simple variant representation
	// In a full implementation, we would create an Error resource in the table
	// and include its handle in the variant payload
	table := component.ResourceTableFromContext(ctx)
	if table != nil && err.Error() != nil {
		id := RegisterError(err.Error())
		handle, hErr := table.NewResourceHandle(id, true, errorResourceType)
		if hErr != nil {
			UnregisterError(id)
			closedVariant := types.ValVariant("closed", nil)
			return types.ValResultError(&closedVariant)
		}
		handleVal := types.ValOwn(uint32(handle))
		errVariant := types.ValVariant("last-operation-failed", &handleVal)
		return types.ValResultError(&errVariant)
	}
	closedVariant := types.ValVariant("closed", nil)
	return types.ValResultError(&closedVariant)
}

// getInputStream retrieves an InputStream from the ResourceTable using a borrow handle.
func getInputStream(ctx context.Context, handle uint32) (*InputStream, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return nil, fmt.Errorf("no resource table in context")
	}
	entry, err := table.Get(runtime.Handle(handle))
	if err != nil {
		return nil, fmt.Errorf("invalid handle %d: %w", handle, err)
	}
	resEntry, ok := entry.(*runtime.ResourceHandleEntry)
	if !ok {
		return nil, fmt.Errorf("handle %d is not a resource handle", handle)
	}
	s := GetInputStream(resEntry.Rep)
	if s == nil {
		return nil, fmt.Errorf("handle %d: InputStream not found in registry (rep=%d)", handle, resEntry.Rep)
	}
	return s, nil
}

// getOutputStream retrieves an OutputStream from the ResourceTable using a borrow handle.
func getOutputStream(ctx context.Context, handle uint32) (*OutputStream, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return nil, fmt.Errorf("no resource table in context")
	}
	entry, err := table.Get(runtime.Handle(handle))
	if err != nil {
		return nil, fmt.Errorf("invalid handle %d: %w", handle, err)
	}
	resEntry, ok := entry.(*runtime.ResourceHandleEntry)
	if !ok {
		return nil, fmt.Errorf("handle %d is not a resource handle", handle)
	}
	s := GetOutputStream(resEntry.Rep)
	if s == nil {
		return nil, fmt.Errorf("handle %d: OutputStream not found in registry (rep=%d)", handle, resEntry.Rep)
	}
	return s, nil
}

// createPollableHandle creates a new Pollable resource in the table and returns its handle.
func createPollableHandle(ctx context.Context, pollable *Pollable) types.Val {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return types.ValOwn(0)
	}
	id := RegisterPollable(pollable)
	handle, err := table.NewResourceHandle(id, true, pollableResourceType)
	if err != nil {
		UnregisterPollable(id)
		return types.ValOwn(0)
	}
	return types.ValOwn(uint32(handle))
}

func instantiateStreams(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:io/streams@0.2.0")

	// input-stream resource
	inst.Resource("input-stream", func(rep uint32) {
		if s := GetInputStream(rep); s != nil {
			s.Destroy()
		}
		UnregisterInputStream(rep)
	})
	inst.Func("[method]input-stream.read", inputStreamRead)
	inst.Func("[method]input-stream.blocking-read", inputStreamBlockingRead)
	inst.Func("[method]input-stream.skip", inputStreamSkip)
	inst.Func("[method]input-stream.blocking-skip", inputStreamBlockingSkip)
	inst.Func("[method]input-stream.subscribe", inputStreamSubscribe)

	// output-stream resource
	inst.Resource("output-stream", func(rep uint32) {
		if s := GetOutputStream(rep); s != nil {
			s.Destroy()
		}
		UnregisterOutputStream(rep)
	})
	inst.Func("[method]output-stream.check-write", outputStreamCheckWrite)
	inst.Func("[method]output-stream.write", outputStreamWrite)
	inst.Func("[method]output-stream.blocking-write-and-flush", outputStreamBlockingWriteAndFlush)
	inst.Func("[method]output-stream.flush", outputStreamFlush)
	inst.Func("[method]output-stream.blocking-flush", outputStreamBlockingFlush)
	inst.Func("[method]output-stream.subscribe", outputStreamSubscribe)
	inst.Func("[method]output-stream.write-zeroes", outputStreamWriteZeroes)
	inst.Func("[method]output-stream.blocking-write-zeroes-and-flush", outputStreamBlockingWriteZeroesAndFlush)
	inst.Func("[method]output-stream.splice", outputStreamSplice)
	inst.Func("[method]output-stream.blocking-splice", outputStreamBlockingSplice)

	return inst.SkipValidation().Build()
}

// Host function implementations - use ResourceTable to look up stream handles

// inputStreamRead implements [method]input-stream.read
// Signature: func(self: borrow<input-stream>, len: u64) -> result<list<u8>, stream-error>
func inputStreamRead(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()
	maxLen := args[1].U64()

	stream, err := getInputStream(ctx, handle)
	if err != nil {
		return nil, err
	}

	data, streamErr := stream.Read(maxLen)
	if streamErr != nil {
		return []types.Val{streamErrorToResultVal(ctx, streamErr)}, nil
	}

	listVal := bytesToListU8(data)
	return []types.Val{types.ValResultOk(&listVal)}, nil
}

// inputStreamBlockingRead implements [method]input-stream.blocking-read
// Signature: func(self: borrow<input-stream>, len: u64) -> result<list<u8>, stream-error>
func inputStreamBlockingRead(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()
	maxLen := args[1].U64()

	stream, err := getInputStream(ctx, handle)
	if err != nil {
		return nil, err
	}

	data, streamErr := stream.BlockingRead(maxLen)
	if streamErr != nil {
		return []types.Val{streamErrorToResultVal(ctx, streamErr)}, nil
	}

	listVal := bytesToListU8(data)
	return []types.Val{types.ValResultOk(&listVal)}, nil
}

// inputStreamSkip implements [method]input-stream.skip
// Signature: func(self: borrow<input-stream>, len: u64) -> result<u64, stream-error>
func inputStreamSkip(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()
	length := args[1].U64()

	stream, err := getInputStream(ctx, handle)
	if err != nil {
		return nil, err
	}

	skipped, streamErr := stream.Skip(length)
	if streamErr != nil {
		return []types.Val{streamErrorToResultVal(ctx, streamErr)}, nil
	}

	result := types.ValU64(skipped)
	return []types.Val{types.ValResultOk(&result)}, nil
}

// inputStreamBlockingSkip implements [method]input-stream.blocking-skip
// Signature: func(self: borrow<input-stream>, len: u64) -> result<u64, stream-error>
func inputStreamBlockingSkip(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()
	length := args[1].U64()

	stream, err := getInputStream(ctx, handle)
	if err != nil {
		return nil, err
	}

	skipped, streamErr := stream.BlockingSkip(length)
	if streamErr != nil {
		return []types.Val{streamErrorToResultVal(ctx, streamErr)}, nil
	}

	result := types.ValU64(skipped)
	return []types.Val{types.ValResultOk(&result)}, nil
}

// inputStreamSubscribe implements [method]input-stream.subscribe
// Signature: func(self: borrow<input-stream>) -> own<pollable>
func inputStreamSubscribe(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()

	stream, err := getInputStream(ctx, handle)
	if err != nil {
		return nil, err
	}

	pollable := stream.Subscribe()
	return []types.Val{createPollableHandle(ctx, pollable)}, nil
}

// outputStreamCheckWrite implements [method]output-stream.check-write
// Signature: func(self: borrow<output-stream>) -> result<u64, stream-error>
func outputStreamCheckWrite(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()

	stream, err := getOutputStream(ctx, handle)
	if err != nil {
		return nil, err
	}

	size, streamErr := stream.CheckWrite()
	if streamErr != nil {
		return []types.Val{streamErrorToResultVal(ctx, streamErr)}, nil
	}

	result := types.ValU64(size)
	return []types.Val{types.ValResultOk(&result)}, nil
}

// outputStreamWrite implements [method]output-stream.write
// Signature: func(self: borrow<output-stream>, contents: list<u8>) -> result<_, stream-error>
func outputStreamWrite(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()
	contents := listU8ToBytes(args[1])

	stream, err := getOutputStream(ctx, handle)
	if err != nil {
		return nil, err
	}

	streamErr := stream.Write(contents)
	if streamErr != nil {
		return []types.Val{streamErrorToResultVal(ctx, streamErr)}, nil
	}

	return []types.Val{types.ValResultOk(nil)}, nil
}

// outputStreamBlockingWriteAndFlush implements [method]output-stream.blocking-write-and-flush
// Signature: func(self: borrow<output-stream>, contents: list<u8>) -> result<_, stream-error>
func outputStreamBlockingWriteAndFlush(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()
	contents := listU8ToBytes(args[1])

	stream, err := getOutputStream(ctx, handle)
	if err != nil {
		return nil, err
	}

	streamErr := stream.BlockingWriteAndFlush(contents)
	if streamErr != nil {
		return []types.Val{streamErrorToResultVal(ctx, streamErr)}, nil
	}

	return []types.Val{types.ValResultOk(nil)}, nil
}

// outputStreamFlush implements [method]output-stream.flush
// Signature: func(self: borrow<output-stream>) -> result<_, stream-error>
func outputStreamFlush(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()

	stream, err := getOutputStream(ctx, handle)
	if err != nil {
		return nil, err
	}

	streamErr := stream.Flush()
	if streamErr != nil {
		return []types.Val{streamErrorToResultVal(ctx, streamErr)}, nil
	}

	return []types.Val{types.ValResultOk(nil)}, nil
}

// outputStreamBlockingFlush implements [method]output-stream.blocking-flush
// Signature: func(self: borrow<output-stream>) -> result<_, stream-error>
func outputStreamBlockingFlush(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()

	stream, err := getOutputStream(ctx, handle)
	if err != nil {
		return nil, err
	}

	streamErr := stream.BlockingFlush()
	if streamErr != nil {
		return []types.Val{streamErrorToResultVal(ctx, streamErr)}, nil
	}

	return []types.Val{types.ValResultOk(nil)}, nil
}

// outputStreamSubscribe implements [method]output-stream.subscribe
// Signature: func(self: borrow<output-stream>) -> own<pollable>
func outputStreamSubscribe(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()

	stream, err := getOutputStream(ctx, handle)
	if err != nil {
		return nil, err
	}

	pollable := stream.Subscribe()
	return []types.Val{createPollableHandle(ctx, pollable)}, nil
}

// outputStreamWriteZeroes implements [method]output-stream.write-zeroes
// Signature: func(self: borrow<output-stream>, len: u64) -> result<_, stream-error>
func outputStreamWriteZeroes(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()
	length := args[1].U64()

	stream, err := getOutputStream(ctx, handle)
	if err != nil {
		return nil, err
	}

	streamErr := stream.WriteZeroes(length)
	if streamErr != nil {
		return []types.Val{streamErrorToResultVal(ctx, streamErr)}, nil
	}

	return []types.Val{types.ValResultOk(nil)}, nil
}

// outputStreamBlockingWriteZeroesAndFlush implements [method]output-stream.blocking-write-zeroes-and-flush
// Signature: func(self: borrow<output-stream>, len: u64) -> result<_, stream-error>
func outputStreamBlockingWriteZeroesAndFlush(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	handle := args[0].Borrow()
	length := args[1].U64()

	stream, err := getOutputStream(ctx, handle)
	if err != nil {
		return nil, err
	}

	streamErr := stream.BlockingWriteZeroesAndFlush(length)
	if streamErr != nil {
		return []types.Val{streamErrorToResultVal(ctx, streamErr)}, nil
	}

	return []types.Val{types.ValResultOk(nil)}, nil
}

// outputStreamSplice implements [method]output-stream.splice
// Signature: func(self: borrow<output-stream>, src: borrow<input-stream>, len: u64) -> result<u64, stream-error>
func outputStreamSplice(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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
		return []types.Val{streamErrorToResultVal(ctx, streamErr)}, nil
	}

	result := types.ValU64(copied)
	return []types.Val{types.ValResultOk(&result)}, nil
}

// outputStreamBlockingSplice implements [method]output-stream.blocking-splice
// Signature: func(self: borrow<output-stream>, src: borrow<input-stream>, len: u64) -> result<u64, stream-error>
func outputStreamBlockingSplice(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
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
		return []types.Val{streamErrorToResultVal(ctx, streamErr)}, nil
	}

	result := types.ValU64(copied)
	return []types.Val{types.ValResultOk(&result)}, nil
}
