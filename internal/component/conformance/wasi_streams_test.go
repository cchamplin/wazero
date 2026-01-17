// Package conformance contains conformance tests for the Component Model implementation.
// This file implements Task 280: WASI Streams Conformance Tests.
package conformance

import (
	"bytes"
	"context"
	"testing"

	"github.com/tetratelabs/wazero/imports/wasip2"
	wasip2io "github.com/tetratelabs/wazero/imports/wasip2/io"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// =============================================================================
// Task 280: WASI Streams Conformance Tests
// =============================================================================

// TestWASI_Streams_InputStreamRead tests the input-stream read operation.
func TestWASI_Streams_InputStreamRead(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	// Create an input stream with test data
	testData := "Hello, WASI Streams!"
	reader := bytes.NewBufferString(testData)
	inputStream := wasip2io.NewInputStream(reader)

	// Register in resource table
	handle := table.New(inputStream, true)

	// Get the streams interface
	streamsDef, ok := linker.Get("wasi:io/streams@0.2.0")
	require.True(t, ok, "streams interface should be registered")

	instDef := streamsDef.(*component.InstanceDef)

	// Get the read method
	readFunc, ok := instDef.Exports["[method]input-stream.read"]
	require.True(t, ok, "read method should be exported")

	funcDef := readFunc.(*component.FuncDef)

	// Call read
	result, err := funcDef.Callback(ctx, []component.Val{
		component.ValBorrow(uint32(handle)),
		component.ValU64(uint64(len(testData))),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result), "read should return exactly one value")

	// Result should be result<list<u8>, stream-error>
	isOk, okVal, _ := result[0].Result()
	require.True(t, isOk, "read should succeed")

	// okVal should be a list<u8>
	byteList := okVal.List()
	require.Equal(t, len(testData), len(byteList), "should read all bytes")

	// Convert to string and verify
	readBytes := make([]byte, len(byteList))
	for i, v := range byteList {
		readBytes[i] = v.U8()
	}
	require.Equal(t, testData, string(readBytes), "read data should match")
}

// TestWASI_Streams_InputStreamBlockingRead tests the input-stream blocking-read operation.
func TestWASI_Streams_InputStreamBlockingRead(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	// Create an input stream with test data
	testData := "Blocking read test"
	reader := bytes.NewBufferString(testData)
	inputStream := wasip2io.NewInputStream(reader)

	handle := table.New(inputStream, true)

	streamsDef, _ := linker.Get("wasi:io/streams@0.2.0")
	instDef := streamsDef.(*component.InstanceDef)

	blockingReadFunc := instDef.Exports["[method]input-stream.blocking-read"].(*component.FuncDef)

	result, err := blockingReadFunc.Callback(ctx, []component.Val{
		component.ValBorrow(uint32(handle)),
		component.ValU64(uint64(len(testData))),
	})
	require.NoError(t, err)

	isOk, okVal, _ := result[0].Result()
	require.True(t, isOk, "blocking-read should succeed")

	byteList := okVal.List()
	require.Equal(t, len(testData), len(byteList), "should read all bytes")
}

// TestWASI_Streams_OutputStreamWrite tests the output-stream write operation.
func TestWASI_Streams_OutputStreamWrite(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	// Create an output stream
	var buf bytes.Buffer
	outputStream := wasip2io.NewOutputStream(&buf)

	handle := table.New(outputStream, true)

	streamsDef, _ := linker.Get("wasi:io/streams@0.2.0")
	instDef := streamsDef.(*component.InstanceDef)

	writeFunc := instDef.Exports["[method]output-stream.write"].(*component.FuncDef)

	// Prepare data to write as list<u8>
	testData := "Hello, Output Stream!"
	dataVals := make([]component.Val, len(testData))
	for i, b := range []byte(testData) {
		dataVals[i] = component.ValU8(b)
	}

	result, err := writeFunc.Callback(ctx, []component.Val{
		component.ValBorrow(uint32(handle)),
		component.ValList(dataVals),
	})
	require.NoError(t, err)

	// Result should be result<_, stream-error>
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "write should succeed")

	// Verify data was written
	require.Equal(t, testData, buf.String(), "written data should match")
}

// TestWASI_Streams_OutputStreamCheckWrite tests the check-write operation.
func TestWASI_Streams_OutputStreamCheckWrite(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	var buf bytes.Buffer
	outputStream := wasip2io.NewOutputStream(&buf)

	handle := table.New(outputStream, true)

	streamsDef, _ := linker.Get("wasi:io/streams@0.2.0")
	instDef := streamsDef.(*component.InstanceDef)

	checkWriteFunc := instDef.Exports["[method]output-stream.check-write"].(*component.FuncDef)

	result, err := checkWriteFunc.Callback(ctx, []component.Val{
		component.ValBorrow(uint32(handle)),
	})
	require.NoError(t, err)

	// Result should be result<u64, stream-error>
	isOk, okVal, _ := result[0].Result()
	require.True(t, isOk, "check-write should succeed")

	capacity := okVal.U64()
	require.True(t, capacity > 0, "writable capacity should be positive")
}

// TestWASI_Streams_OutputStreamFlush tests the flush operation.
func TestWASI_Streams_OutputStreamFlush(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	var buf bytes.Buffer
	outputStream := wasip2io.NewOutputStream(&buf)

	handle := table.New(outputStream, true)

	streamsDef, _ := linker.Get("wasi:io/streams@0.2.0")
	instDef := streamsDef.(*component.InstanceDef)

	flushFunc := instDef.Exports["[method]output-stream.flush"].(*component.FuncDef)

	result, err := flushFunc.Callback(ctx, []component.Val{
		component.ValBorrow(uint32(handle)),
	})
	require.NoError(t, err)

	// Result should be result<_, stream-error>
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "flush should succeed")
}

// TestWASI_Streams_InputStreamSkip tests the skip operation.
func TestWASI_Streams_InputStreamSkip(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	testData := "Skip these bytes and read this"
	reader := bytes.NewBufferString(testData)
	inputStream := wasip2io.NewInputStream(reader)

	handle := table.New(inputStream, true)

	streamsDef, _ := linker.Get("wasi:io/streams@0.2.0")
	instDef := streamsDef.(*component.InstanceDef)

	skipFunc := instDef.Exports["[method]input-stream.skip"].(*component.FuncDef)

	result, err := skipFunc.Callback(ctx, []component.Val{
		component.ValBorrow(uint32(handle)),
		component.ValU64(10), // Skip 10 bytes
	})
	require.NoError(t, err)

	isOk, okVal, _ := result[0].Result()
	require.True(t, isOk, "skip should succeed")

	skipped := okVal.U64()
	require.Equal(t, uint64(10), skipped, "should skip 10 bytes")
}

// TestWASI_Streams_InputStreamSubscribe tests the subscribe operation.
func TestWASI_Streams_InputStreamSubscribe(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	reader := bytes.NewBufferString("test data")
	inputStream := wasip2io.NewInputStream(reader)

	handle := table.New(inputStream, true)

	streamsDef, _ := linker.Get("wasi:io/streams@0.2.0")
	instDef := streamsDef.(*component.InstanceDef)

	subscribeFunc := instDef.Exports["[method]input-stream.subscribe"].(*component.FuncDef)

	result, err := subscribeFunc.Callback(ctx, []component.Val{
		component.ValBorrow(uint32(handle)),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result), "subscribe should return exactly one value")

	// Result should be own<pollable>
	pollableHandle := result[0].Own()
	require.True(t, pollableHandle > 0, "should return a valid pollable handle")
}

// TestWASI_Streams_OutputStreamSubscribe tests the subscribe operation for output streams.
func TestWASI_Streams_OutputStreamSubscribe(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	var buf bytes.Buffer
	outputStream := wasip2io.NewOutputStream(&buf)

	handle := table.New(outputStream, true)

	streamsDef, _ := linker.Get("wasi:io/streams@0.2.0")
	instDef := streamsDef.(*component.InstanceDef)

	subscribeFunc := instDef.Exports["[method]output-stream.subscribe"].(*component.FuncDef)

	result, err := subscribeFunc.Callback(ctx, []component.Val{
		component.ValBorrow(uint32(handle)),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result), "subscribe should return exactly one value")

	pollableHandle := result[0].Own()
	require.True(t, pollableHandle > 0, "should return a valid pollable handle")
}

// TestWASI_Streams_InterfaceRegistration tests that streams interfaces are properly registered.
func TestWASI_Streams_InterfaceRegistration(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	// Verify streams interface is registered
	interfaces := []string{
		"wasi:io/streams@0.2.0",
	}

	for _, iface := range interfaces {
		def, ok := linker.Get(iface)
		require.True(t, ok, "interface %s should be registered", iface)
		require.NotNil(t, def, "interface %s definition should not be nil", iface)
	}
}

// TestWASI_Streams_AllMethodsExist tests that all expected stream methods exist.
func TestWASI_Streams_AllMethodsExist(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	streamsDef, ok := linker.Get("wasi:io/streams@0.2.0")
	require.True(t, ok, "streams interface should be registered")

	instDef := streamsDef.(*component.InstanceDef)

	// Expected input-stream methods
	inputMethods := []string{
		"[method]input-stream.read",
		"[method]input-stream.blocking-read",
		"[method]input-stream.skip",
		"[method]input-stream.blocking-skip",
		"[method]input-stream.subscribe",
	}

	for _, method := range inputMethods {
		methodDef, ok := instDef.Exports[method]
		require.True(t, ok, "method %s should be exported", method)
		require.NotNil(t, methodDef, "method %s should not be nil", method)
	}

	// Expected output-stream methods
	outputMethods := []string{
		"[method]output-stream.check-write",
		"[method]output-stream.write",
		"[method]output-stream.blocking-write-and-flush",
		"[method]output-stream.flush",
		"[method]output-stream.blocking-flush",
		"[method]output-stream.subscribe",
		"[method]output-stream.write-zeroes",
		"[method]output-stream.blocking-write-zeroes-and-flush",
		"[method]output-stream.splice",
		"[method]output-stream.blocking-splice",
	}

	for _, method := range outputMethods {
		methodDef, ok := instDef.Exports[method]
		require.True(t, ok, "method %s should be exported", method)
		require.NotNil(t, methodDef, "method %s should not be nil", method)
	}

	// Verify resources exist
	inputStreamRes, ok := instDef.Exports["input-stream"]
	require.True(t, ok, "input-stream resource should be exported")
	require.NotNil(t, inputStreamRes)

	outputStreamRes, ok := instDef.Exports["output-stream"]
	require.True(t, ok, "output-stream resource should be exported")
	require.NotNil(t, outputStreamRes)
}

// TestWASI_Streams_WriteZeroes tests the write-zeroes operation.
func TestWASI_Streams_WriteZeroes(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	var buf bytes.Buffer
	outputStream := wasip2io.NewOutputStream(&buf)

	handle := table.New(outputStream, true)

	streamsDef, _ := linker.Get("wasi:io/streams@0.2.0")
	instDef := streamsDef.(*component.InstanceDef)

	writeZeroesFunc := instDef.Exports["[method]output-stream.write-zeroes"].(*component.FuncDef)

	result, err := writeZeroesFunc.Callback(ctx, []component.Val{
		component.ValBorrow(uint32(handle)),
		component.ValU64(10), // Write 10 zero bytes
	})
	require.NoError(t, err)

	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "write-zeroes should succeed")

	// Verify bytes were written
	require.Equal(t, 10, buf.Len(), "should have written 10 bytes")

	// Verify they are all zeros
	for _, b := range buf.Bytes() {
		require.Equal(t, byte(0), b, "all bytes should be zero")
	}
}

// TestWASI_Streams_BlockingWriteAndFlush tests blocking-write-and-flush operation.
func TestWASI_Streams_BlockingWriteAndFlush(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	var buf bytes.Buffer
	outputStream := wasip2io.NewOutputStream(&buf)

	handle := table.New(outputStream, true)

	streamsDef, _ := linker.Get("wasi:io/streams@0.2.0")
	instDef := streamsDef.(*component.InstanceDef)

	blockingWriteFunc := instDef.Exports["[method]output-stream.blocking-write-and-flush"].(*component.FuncDef)

	testData := "Blocking write test"
	dataVals := make([]component.Val, len(testData))
	for i, b := range []byte(testData) {
		dataVals[i] = component.ValU8(b)
	}

	result, err := blockingWriteFunc.Callback(ctx, []component.Val{
		component.ValBorrow(uint32(handle)),
		component.ValList(dataVals),
	})
	require.NoError(t, err)

	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "blocking-write-and-flush should succeed")

	require.Equal(t, testData, buf.String(), "written data should match")
}
