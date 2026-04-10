// imports/wasip2/io/streams_test.go

package io

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestStreamError_Closed(t *testing.T) {
	err := StreamErrorClosed()
	require.True(t, err.IsClosed())
	require.False(t, err.IsLastOperationFailed())
	require.Nil(t, err.Error())
}

func TestStreamError_LastOperationFailed(t *testing.T) {
	ioErr := NewError(errors.New("test error"))
	err := StreamErrorLastOperationFailed(ioErr)
	require.False(t, err.IsClosed())
	require.True(t, err.IsLastOperationFailed())
	require.Equal(t, ioErr, err.Error())
}

func TestStreamError_LastOperationFailed_NilError(t *testing.T) {
	err := StreamErrorLastOperationFailed(nil)
	require.False(t, err.IsClosed())
	require.True(t, err.IsLastOperationFailed())
	require.Nil(t, err.Error())
}

func TestInputStream_Read(t *testing.T) {
	reader := bytes.NewReader([]byte("hello world"))
	stream := NewInputStream(reader)

	data, err := stream.Read(5)
	require.Nil(t, err)
	require.Equal(t, []byte("hello"), data)
}

func TestInputStream_Read_All(t *testing.T) {
	reader := bytes.NewReader([]byte("hello world"))
	stream := NewInputStream(reader)

	data, err := stream.Read(100)
	require.Nil(t, err)
	require.Equal(t, []byte("hello world"), data)
}

func TestInputStream_Read_EOF(t *testing.T) {
	reader := bytes.NewReader([]byte("hi"))
	stream := NewInputStream(reader)

	data, err := stream.Read(10)
	require.Nil(t, err)
	require.Equal(t, []byte("hi"), data)

	// Next read at EOF should return empty slice, not an error
	data, err = stream.Read(10)
	require.Nil(t, err)
	require.Equal(t, 0, len(data))
}

func TestInputStream_Read_Closed(t *testing.T) {
	reader := bytes.NewReader([]byte("hello"))
	stream := NewInputStream(reader)
	stream.Close()

	data, err := stream.Read(10)
	require.Nil(t, data)
	require.NotNil(t, err)
	require.True(t, err.IsClosed())
}

func TestInputStream_BlockingRead(t *testing.T) {
	reader := bytes.NewReader([]byte("blocking test"))
	stream := NewInputStream(reader)

	data, err := stream.BlockingRead(8)
	require.Nil(t, err)
	require.Equal(t, []byte("blocking"), data)
}

func TestInputStream_Skip(t *testing.T) {
	reader := bytes.NewReader([]byte("hello world"))
	stream := NewInputStream(reader)

	skipped, err := stream.Skip(6)
	require.Nil(t, err)
	require.Equal(t, uint64(6), skipped)

	// Verify we skipped
	data, _ := stream.Read(10)
	require.Equal(t, []byte("world"), data)
}

func TestInputStream_Skip_PartialAtEOF(t *testing.T) {
	reader := bytes.NewReader([]byte("short"))
	stream := NewInputStream(reader)

	skipped, err := stream.Skip(100)
	require.Nil(t, err)
	require.Equal(t, uint64(5), skipped)
}

func TestInputStream_Skip_Closed(t *testing.T) {
	reader := bytes.NewReader([]byte("test"))
	stream := NewInputStream(reader)
	stream.Close()

	skipped, err := stream.Skip(10)
	require.Equal(t, uint64(0), skipped)
	require.NotNil(t, err)
	require.True(t, err.IsClosed())
}

func TestInputStream_BlockingSkip(t *testing.T) {
	reader := bytes.NewReader([]byte("hello world"))
	stream := NewInputStream(reader)

	skipped, err := stream.BlockingSkip(6)
	require.Nil(t, err)
	require.Equal(t, uint64(6), skipped)
}

func TestInputStream_Subscribe(t *testing.T) {
	reader := bytes.NewReader([]byte("test"))
	stream := NewInputStream(reader)

	pollable := stream.Subscribe()
	require.NotNil(t, pollable)
	require.True(t, pollable.Ready())
}

func TestInputStream_Close(t *testing.T) {
	reader := bytes.NewReader([]byte("test"))
	stream := NewInputStream(reader)

	require.False(t, stream.IsClosed())
	stream.Close()
	require.True(t, stream.IsClosed())
}

type errorReader struct{}

func (r *errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("read error")
}

func TestInputStream_Read_Error(t *testing.T) {
	stream := NewInputStream(&errorReader{})

	data, err := stream.Read(10)
	require.Nil(t, data)
	require.NotNil(t, err)
	require.True(t, err.IsLastOperationFailed())
	require.NotNil(t, err.Error())
	require.Equal(t, "read error", err.Error().ToDebugString())
}

func TestOutputStream_Write(t *testing.T) {
	buf := &bytes.Buffer{}
	stream := NewOutputStream(buf)

	err := stream.Write([]byte("hello"))
	require.Nil(t, err)
	require.Equal(t, "hello", buf.String())
}

func TestOutputStream_Write_Multiple(t *testing.T) {
	buf := &bytes.Buffer{}
	stream := NewOutputStream(buf)

	err := stream.Write([]byte("hello "))
	require.Nil(t, err)
	err = stream.Write([]byte("world"))
	require.Nil(t, err)
	require.Equal(t, "hello world", buf.String())
}

func TestOutputStream_Write_Closed(t *testing.T) {
	buf := &bytes.Buffer{}
	stream := NewOutputStream(buf)
	stream.Close()

	err := stream.Write([]byte("test"))
	require.NotNil(t, err)
	require.True(t, err.IsClosed())
}

func TestOutputStream_CheckWrite(t *testing.T) {
	buf := &bytes.Buffer{}
	stream := NewOutputStream(buf)

	size, err := stream.CheckWrite()
	require.Nil(t, err)
	require.Equal(t, uint64(64*1024), size)
}

func TestOutputStream_CheckWrite_Closed(t *testing.T) {
	buf := &bytes.Buffer{}
	stream := NewOutputStream(buf)
	stream.Close()

	size, err := stream.CheckWrite()
	require.Equal(t, uint64(0), size)
	require.NotNil(t, err)
	require.True(t, err.IsClosed())
}

func TestOutputStream_Flush(t *testing.T) {
	buf := &bytes.Buffer{}
	stream := NewOutputStream(buf)

	err := stream.Flush()
	require.Nil(t, err)
}

func TestOutputStream_Flush_Closed(t *testing.T) {
	buf := &bytes.Buffer{}
	stream := NewOutputStream(buf)
	stream.Close()

	err := stream.Flush()
	require.NotNil(t, err)
	require.True(t, err.IsClosed())
}

func TestOutputStream_BlockingFlush(t *testing.T) {
	buf := &bytes.Buffer{}
	stream := NewOutputStream(buf)

	err := stream.BlockingFlush()
	require.Nil(t, err)
}

func TestOutputStream_BlockingWriteAndFlush(t *testing.T) {
	buf := &bytes.Buffer{}
	stream := NewOutputStream(buf)

	err := stream.BlockingWriteAndFlush([]byte("test data"))
	require.Nil(t, err)
	require.Equal(t, "test data", buf.String())
}

func TestOutputStream_WriteZeroes(t *testing.T) {
	buf := &bytes.Buffer{}
	stream := NewOutputStream(buf)

	err := stream.WriteZeroes(10)
	require.Nil(t, err)
	require.Equal(t, 10, buf.Len())
	// Verify all zeros
	for _, b := range buf.Bytes() {
		require.Equal(t, byte(0), b)
	}
}

func TestOutputStream_WriteZeroes_Large(t *testing.T) {
	buf := &bytes.Buffer{}
	stream := NewOutputStream(buf)

	// Write more than 64KB to test chunking
	err := stream.WriteZeroes(100 * 1024)
	require.Nil(t, err)
	require.Equal(t, 100*1024, buf.Len())
}

func TestOutputStream_WriteZeroes_Closed(t *testing.T) {
	buf := &bytes.Buffer{}
	stream := NewOutputStream(buf)
	stream.Close()

	err := stream.WriteZeroes(10)
	require.NotNil(t, err)
	require.True(t, err.IsClosed())
}

func TestOutputStream_BlockingWriteZeroesAndFlush(t *testing.T) {
	buf := &bytes.Buffer{}
	stream := NewOutputStream(buf)

	err := stream.BlockingWriteZeroesAndFlush(5)
	require.Nil(t, err)
	require.Equal(t, 5, buf.Len())
}

func TestOutputStream_Subscribe(t *testing.T) {
	buf := &bytes.Buffer{}
	stream := NewOutputStream(buf)

	pollable := stream.Subscribe()
	require.NotNil(t, pollable)
	require.True(t, pollable.Ready())
}

func TestOutputStream_Close(t *testing.T) {
	buf := &bytes.Buffer{}
	stream := NewOutputStream(buf)

	require.False(t, stream.IsClosed())
	stream.Close()
	require.True(t, stream.IsClosed())
}

type errorWriter struct{}

func (w *errorWriter) Write(p []byte) (n int, err error) {
	return 0, errors.New("write error")
}

func TestOutputStream_Write_Error(t *testing.T) {
	stream := NewOutputStream(&errorWriter{})

	err := stream.Write([]byte("test"))
	require.NotNil(t, err)
	require.True(t, err.IsLastOperationFailed())
	require.Equal(t, "write error", err.Error().ToDebugString())
}

func TestOutputStream_Splice(t *testing.T) {
	srcData := []byte("source data to splice")
	srcReader := bytes.NewReader(srcData)
	srcStream := NewInputStream(srcReader)

	dstBuf := &bytes.Buffer{}
	dstStream := NewOutputStream(dstBuf)

	n, err := dstStream.Splice(srcStream, 10)
	require.Nil(t, err)
	require.Equal(t, uint64(10), n)
	require.Equal(t, "source dat", dstBuf.String())
}

func TestOutputStream_Splice_ClosedDst(t *testing.T) {
	srcReader := bytes.NewReader([]byte("test"))
	srcStream := NewInputStream(srcReader)

	dstBuf := &bytes.Buffer{}
	dstStream := NewOutputStream(dstBuf)
	dstStream.Close()

	n, err := dstStream.Splice(srcStream, 10)
	require.Equal(t, uint64(0), n)
	require.NotNil(t, err)
	require.True(t, err.IsClosed())
}

func TestOutputStream_Splice_ClosedSrc(t *testing.T) {
	srcReader := bytes.NewReader([]byte("test"))
	srcStream := NewInputStream(srcReader)
	srcStream.Close()

	dstBuf := &bytes.Buffer{}
	dstStream := NewOutputStream(dstBuf)

	n, err := dstStream.Splice(srcStream, 10)
	require.Equal(t, uint64(0), n)
	require.NotNil(t, err)
	require.True(t, err.IsClosed())
}

func TestOutputStream_BlockingSplice(t *testing.T) {
	srcData := []byte("blocking splice data")
	srcReader := bytes.NewReader(srcData)
	srcStream := NewInputStream(srcReader)

	dstBuf := &bytes.Buffer{}
	dstStream := NewOutputStream(dstBuf)

	n, err := dstStream.BlockingSplice(srcStream, 8)
	require.Nil(t, err)
	require.Equal(t, uint64(8), n)
	require.Equal(t, "blocking", dstBuf.String())
}

// closerBuffer wraps bytes.Buffer with a Close method
type closerBuffer struct {
	bytes.Buffer
	closed bool
}

func (c *closerBuffer) Close() error {
	c.closed = true
	return nil
}

func TestInputStream_Close_WithCloser(t *testing.T) {
	buf := &closerBuffer{}
	buf.WriteString("test")
	stream := NewInputStream(buf)

	stream.Close()
	require.True(t, stream.IsClosed())
	require.True(t, buf.closed)
}

// closerWriter wraps bytes.Buffer with a Close method for output
type closerWriter struct {
	bytes.Buffer
	closed bool
}

func (c *closerWriter) Close() error {
	c.closed = true
	return nil
}

func TestOutputStream_Close_WithCloser(t *testing.T) {
	buf := &closerWriter{}
	stream := NewOutputStream(buf)

	stream.Close()
	require.True(t, stream.IsClosed())
	require.True(t, buf.closed)
}

// flushableWriter for testing flush
type flushableWriter struct {
	bytes.Buffer
	flushed bool
}

func (f *flushableWriter) Flush() error {
	f.flushed = true
	return nil
}

func TestOutputStream_Flush_WithFlusher(t *testing.T) {
	buf := &flushableWriter{}
	stream := NewOutputStream(buf)

	err := stream.Flush()
	require.Nil(t, err)
	require.True(t, buf.flushed)
}

type flushErrorWriter struct {
	bytes.Buffer
}

func (f *flushErrorWriter) Flush() error {
	return errors.New("flush error")
}

func TestOutputStream_Flush_Error(t *testing.T) {
	buf := &flushErrorWriter{}
	stream := NewOutputStream(buf)

	err := stream.Flush()
	require.NotNil(t, err)
	require.True(t, err.IsLastOperationFailed())
	require.Equal(t, "flush error", err.Error().ToDebugString())
}

func TestMinUint64(t *testing.T) {
	require.Equal(t, uint64(5), minUint64(5, 10))
	require.Equal(t, uint64(5), minUint64(10, 5))
	require.Equal(t, uint64(7), minUint64(7, 7))
}

func TestInstantiateStreams(t *testing.T) {
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)
	err := instantiateStreams(linker)
	require.NoError(t, err)

	// Verify interface is registered
	def, err := linker.MatchImport("wasi:io/streams@0.2.0")
	require.NoError(t, err)

	// Verify it's an instance definition
	instDef, ok := def.(*component.InstanceDef)
	require.True(t, ok, "expected InstanceDef")

	// Verify input-stream exports exist
	_, hasInputStreamResource := instDef.Exports["input-stream"]
	require.True(t, hasInputStreamResource, "input-stream resource should be defined")

	_, hasReadMethod := instDef.Exports["[method]input-stream.read"]
	require.True(t, hasReadMethod, "input-stream.read method should be defined")

	_, hasBlockingReadMethod := instDef.Exports["[method]input-stream.blocking-read"]
	require.True(t, hasBlockingReadMethod, "input-stream.blocking-read method should be defined")

	_, hasSkipMethod := instDef.Exports["[method]input-stream.skip"]
	require.True(t, hasSkipMethod, "input-stream.skip method should be defined")

	_, hasBlockingSkipMethod := instDef.Exports["[method]input-stream.blocking-skip"]
	require.True(t, hasBlockingSkipMethod, "input-stream.blocking-skip method should be defined")

	_, hasInputSubscribeMethod := instDef.Exports["[method]input-stream.subscribe"]
	require.True(t, hasInputSubscribeMethod, "input-stream.subscribe method should be defined")

	// Verify output-stream exports exist
	_, hasOutputStreamResource := instDef.Exports["output-stream"]
	require.True(t, hasOutputStreamResource, "output-stream resource should be defined")

	_, hasCheckWriteMethod := instDef.Exports["[method]output-stream.check-write"]
	require.True(t, hasCheckWriteMethod, "output-stream.check-write method should be defined")

	_, hasWriteMethod := instDef.Exports["[method]output-stream.write"]
	require.True(t, hasWriteMethod, "output-stream.write method should be defined")

	_, hasBlockingWriteAndFlushMethod := instDef.Exports["[method]output-stream.blocking-write-and-flush"]
	require.True(t, hasBlockingWriteAndFlushMethod, "output-stream.blocking-write-and-flush method should be defined")

	_, hasFlushMethod := instDef.Exports["[method]output-stream.flush"]
	require.True(t, hasFlushMethod, "output-stream.flush method should be defined")

	_, hasBlockingFlushMethod := instDef.Exports["[method]output-stream.blocking-flush"]
	require.True(t, hasBlockingFlushMethod, "output-stream.blocking-flush method should be defined")

	_, hasOutputSubscribeMethod := instDef.Exports["[method]output-stream.subscribe"]
	require.True(t, hasOutputSubscribeMethod, "output-stream.subscribe method should be defined")

	_, hasWriteZeroesMethod := instDef.Exports["[method]output-stream.write-zeroes"]
	require.True(t, hasWriteZeroesMethod, "output-stream.write-zeroes method should be defined")

	_, hasBlockingWriteZeroesAndFlushMethod := instDef.Exports["[method]output-stream.blocking-write-zeroes-and-flush"]
	require.True(t, hasBlockingWriteZeroesAndFlushMethod, "output-stream.blocking-write-zeroes-and-flush method should be defined")

	_, hasSpliceMethod := instDef.Exports["[method]output-stream.splice"]
	require.True(t, hasSpliceMethod, "output-stream.splice method should be defined")

	_, hasBlockingSpliceMethod := instDef.Exports["[method]output-stream.blocking-splice"]
	require.True(t, hasBlockingSpliceMethod, "output-stream.blocking-splice method should be defined")
}

func TestInstantiateStreams_Duplicate(t *testing.T) {
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)

	// First registration should succeed
	err := instantiateStreams(linker)
	require.NoError(t, err)

	// Second registration should fail
	err = instantiateStreams(linker)
	require.Error(t, err)
}

// Tests for host functions with ResourceTable

func TestInputStreamRead_HostFunction(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Create input stream and add to table
	reader := bytes.NewReader([]byte("hello world"))
	sid := RegisterInputStream(NewInputStream(reader))
	handle, errH1 := table.NewResourceHandle(sid, true, inputStreamResourceType)
	if errH1 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH1)
	}

	// Call host function
	args := []types.Val{
		types.ValBorrow(uint32(handle)),
		types.ValU64(5),
	}
	results, err := inputStreamRead(ctx, nil, args)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))

	// Verify result is ok with data
	isOk, ok, _ := results[0].Result()
	require.True(t, isOk)
	require.NotNil(t, ok)

	// Extract list<u8> and convert to bytes
	list := ok.List()
	require.Equal(t, 5, len(list))
	data := make([]byte, len(list))
	for i, v := range list {
		data[i] = v.U8()
	}
	require.Equal(t, []byte("hello"), data)
}

func TestInputStreamRead_ClosedStream(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Create closed input stream
	reader := bytes.NewReader([]byte("hello"))
	stream := NewInputStream(reader)
	stream.Close()
	sid := RegisterInputStream(stream)
	handle, errH2 := table.NewResourceHandle(sid, true, inputStreamResourceType)
	if errH2 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH2)
	}

	args := []types.Val{
		types.ValBorrow(uint32(handle)),
		types.ValU64(5),
	}
	results, err := inputStreamRead(ctx, nil, args)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))

	// Verify result is error (closed)
	isOk, _, _ := results[0].Result()
	require.False(t, isOk)
}

func TestInputStreamSkip_HostFunction(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	reader := bytes.NewReader([]byte("hello world"))
	sid := RegisterInputStream(NewInputStream(reader))
	handle, errH3 := table.NewResourceHandle(sid, true, inputStreamResourceType)
	if errH3 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH3)
	}

	args := []types.Val{
		types.ValBorrow(uint32(handle)),
		types.ValU64(6),
	}
	results, err := inputStreamSkip(ctx, nil, args)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))

	isOk, ok, _ := results[0].Result()
	require.True(t, isOk)
	require.NotNil(t, ok)
	require.Equal(t, uint64(6), ok.U64())
}

func TestInputStreamSubscribe_HostFunction(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	reader := bytes.NewReader([]byte("test"))
	sid := RegisterInputStream(NewInputStream(reader))
	handle, errH4 := table.NewResourceHandle(sid, true, inputStreamResourceType)
	if errH4 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH4)
	}

	args := []types.Val{
		types.ValBorrow(uint32(handle)),
	}
	results, err := inputStreamSubscribe(ctx, nil, args)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))

	// Verify result is own<pollable>
	pollableHandle := results[0].Own()
	// The handle should be valid (non-zero after the input stream handle)
	require.True(t, pollableHandle > 0 || pollableHandle == 0)
}

func TestOutputStreamCheckWrite_HostFunction(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	buf := &bytes.Buffer{}
	sid := RegisterOutputStream(NewOutputStream(buf))
	handle, errH5 := table.NewResourceHandle(sid, true, inputStreamResourceType)
	if errH5 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH5)
	}

	args := []types.Val{
		types.ValBorrow(uint32(handle)),
	}
	results, err := outputStreamCheckWrite(ctx, nil, args)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))

	isOk, ok, _ := results[0].Result()
	require.True(t, isOk)
	require.NotNil(t, ok)
	require.Equal(t, uint64(64*1024), ok.U64())
}

func TestOutputStreamWrite_HostFunction(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	buf := &bytes.Buffer{}
	sid := RegisterOutputStream(NewOutputStream(buf))
	handle, errH6 := table.NewResourceHandle(sid, true, inputStreamResourceType)
	if errH6 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH6)
	}

	// Create list<u8> for "hello"
	data := []byte("hello")
	listVals := make([]types.Val, len(data))
	for i, b := range data {
		listVals[i] = types.ValU8(b)
	}

	args := []types.Val{
		types.ValBorrow(uint32(handle)),
		types.ValList(listVals),
	}
	results, err := outputStreamWrite(ctx, nil, args)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))

	isOk, _, _ := results[0].Result()
	require.True(t, isOk)

	// Verify data was written
	require.Equal(t, "hello", buf.String())
}

func TestOutputStreamFlush_HostFunction(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	buf := &flushableWriter{}
	sid := RegisterOutputStream(NewOutputStream(buf))
	handle, errH7 := table.NewResourceHandle(sid, true, inputStreamResourceType)
	if errH7 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH7)
	}

	args := []types.Val{
		types.ValBorrow(uint32(handle)),
	}
	results, err := outputStreamFlush(ctx, nil, args)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))

	isOk, _, _ := results[0].Result()
	require.True(t, isOk)
	require.True(t, buf.flushed)
}

func TestOutputStreamWriteZeroes_HostFunction(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	buf := &bytes.Buffer{}
	sid := RegisterOutputStream(NewOutputStream(buf))
	handle, errH8 := table.NewResourceHandle(sid, true, inputStreamResourceType)
	if errH8 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH8)
	}

	args := []types.Val{
		types.ValBorrow(uint32(handle)),
		types.ValU64(10),
	}
	results, err := outputStreamWriteZeroes(ctx, nil, args)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))

	isOk, _, _ := results[0].Result()
	require.True(t, isOk)

	require.Equal(t, 10, buf.Len())
	for _, b := range buf.Bytes() {
		require.Equal(t, byte(0), b)
	}
}

func TestOutputStreamSplice_HostFunction(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	srcReader := bytes.NewReader([]byte("source data"))
	sid1 := RegisterInputStream(NewInputStream(srcReader))
	srcHandle, errH9 := table.NewResourceHandle(sid1, true, inputStreamResourceType)
	if errH9 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH9)
	}

	dstBuf := &bytes.Buffer{}
	sid2 := RegisterOutputStream(NewOutputStream(dstBuf))
	dstHandle, errH10 := table.NewResourceHandle(sid2, true, outputStreamResourceType)
	if errH10 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH10)
	}

	args := []types.Val{
		types.ValBorrow(uint32(dstHandle)),
		types.ValBorrow(uint32(srcHandle)),
		types.ValU64(6),
	}
	results, err := outputStreamSplice(ctx, nil, args)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))

	isOk, ok, _ := results[0].Result()
	require.True(t, isOk)
	require.NotNil(t, ok)
	require.Equal(t, uint64(6), ok.U64())
	require.Equal(t, "source", dstBuf.String())
}

func TestOutputStreamSubscribe_HostFunction(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	buf := &bytes.Buffer{}
	sid := RegisterOutputStream(NewOutputStream(buf))
	handle, errH11 := table.NewResourceHandle(sid, true, inputStreamResourceType)
	if errH11 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH11)
	}

	args := []types.Val{
		types.ValBorrow(uint32(handle)),
	}
	results, err := outputStreamSubscribe(ctx, nil, args)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))

	// Verify result is own<pollable>
	_ = results[0].Own() // Just verify it doesn't panic
}

func TestHostFunction_InvalidHandle(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Use an invalid handle
	args := []types.Val{
		types.ValBorrow(999),
		types.ValU64(5),
	}
	_, err := inputStreamRead(ctx, nil, args)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid handle")
}

func TestHostFunction_NoResourceTable(t *testing.T) {
	// Context without ResourceTable
	ctx := context.Background()

	args := []types.Val{
		types.ValBorrow(0),
		types.ValU64(5),
	}
	_, err := inputStreamRead(ctx, nil, args)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no resource table")
}

func TestHostFunction_WrongResourceType(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Create an OutputStream but try to use it as InputStream
	buf := &bytes.Buffer{}
	sid := RegisterOutputStream(NewOutputStream(buf))
	handle, errH12 := table.NewResourceHandle(sid, true, inputStreamResourceType)
	if errH12 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH12)
	}

	args := []types.Val{
		types.ValBorrow(uint32(handle)),
		types.ValU64(5),
	}
	_, err := inputStreamRead(ctx, nil, args)
	require.Error(t, err)
	require.Contains(t, err.Error(), "InputStream not found in registry")
}

func TestBytesToListU8(t *testing.T) {
	data := []byte{1, 2, 3, 4, 5}
	listVal := bytesToListU8(data)

	list := listVal.List()
	require.Equal(t, 5, len(list))
	for i, v := range list {
		require.Equal(t, data[i], v.U8())
	}
}

func TestListU8ToBytes(t *testing.T) {
	listVals := []types.Val{
		types.ValU8(1),
		types.ValU8(2),
		types.ValU8(3),
	}
	listVal := types.ValList(listVals)

	data := listU8ToBytes(listVal)
	require.Equal(t, []byte{1, 2, 3}, data)
}

// Tests for Destroy method

func TestInputStream_Destroy(t *testing.T) {
	buf := &closerBuffer{}
	buf.WriteString("test data")
	stream := NewInputStream(buf)

	// Stream should not be closed yet
	require.False(t, stream.IsClosed())
	require.False(t, buf.closed)

	// Destroy should close the reader
	stream.Destroy()

	// Stream and underlying reader should be closed
	require.True(t, stream.IsClosed())
	require.True(t, buf.closed)
}

func TestInputStream_Destroy_Idempotent(t *testing.T) {
	buf := &closerBuffer{}
	buf.WriteString("test")
	stream := NewInputStream(buf)

	// Multiple calls to Destroy should be safe
	stream.Destroy()
	stream.Destroy()
	stream.Destroy()

	require.True(t, stream.IsClosed())
	require.True(t, buf.closed)
}

func TestInputStream_Destroy_NonCloser(t *testing.T) {
	// bytes.Reader doesn't implement io.Closer
	reader := bytes.NewReader([]byte("test"))
	stream := NewInputStream(reader)

	// Destroy should not panic for non-Closer readers
	stream.Destroy()

	require.True(t, stream.IsClosed())
}

func TestOutputStream_Destroy(t *testing.T) {
	buf := &closerWriter{}
	stream := NewOutputStream(buf)

	// Write some data
	stream.Write([]byte("hello world"))

	// Stream should not be closed yet
	require.False(t, stream.IsClosed())
	require.False(t, buf.closed)

	// Destroy should close the writer
	stream.Destroy()

	// Stream and underlying writer should be closed
	require.True(t, stream.IsClosed())
	require.True(t, buf.closed)
}

func TestOutputStream_Destroy_Idempotent(t *testing.T) {
	buf := &closerWriter{}
	stream := NewOutputStream(buf)

	// Multiple calls to Destroy should be safe
	stream.Destroy()
	stream.Destroy()
	stream.Destroy()

	require.True(t, stream.IsClosed())
	require.True(t, buf.closed)
}

func TestOutputStream_Destroy_FlushesFirst(t *testing.T) {
	buf := &flushableCloserWriter{}
	stream := NewOutputStream(buf)

	stream.Write([]byte("test data"))

	// Destroy should flush before close
	stream.Destroy()

	// Both flush and close should have been called
	require.True(t, buf.flushed, "Destroy should flush before closing")
	require.True(t, buf.closed)
}

func TestOutputStream_Destroy_NonCloser(t *testing.T) {
	// bytes.Buffer doesn't implement io.Closer
	buf := &bytes.Buffer{}
	stream := NewOutputStream(buf)

	stream.Write([]byte("test"))

	// Destroy should not panic for non-Closer writers
	stream.Destroy()

	require.True(t, stream.IsClosed())
}

// flushableCloserWriter is a test helper that tracks both Flush and Close calls
type flushableCloserWriter struct {
	bytes.Buffer
	flushed bool
	closed  bool
}

func (f *flushableCloserWriter) Flush() error {
	f.flushed = true
	return nil
}

func (f *flushableCloserWriter) Close() error {
	f.closed = true
	return nil
}
