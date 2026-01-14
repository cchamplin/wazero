// imports/wasip2/io/streams_test.go

package io

import (
	"bytes"
	"errors"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
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
	linker := component.NewLinker()
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
	linker := component.NewLinker()

	// First registration should succeed
	err := instantiateStreams(linker)
	require.NoError(t, err)

	// Second registration should fail
	err = instantiateStreams(linker)
	require.Error(t, err)
}

