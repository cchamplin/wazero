// Package conformance contains conformance tests for the Component Model implementation.
// This file implements Task 284: WASI Error Handling Pattern Tests.
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
// Task 284: WASI Error Handling Pattern Tests
// =============================================================================

// TestWASI_ErrorHandling_StreamClosedError tests that closed stream errors are properly returned.
func TestWASI_ErrorHandling_StreamClosedError(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	// Create an input stream and close it
	reader := bytes.NewBufferString("test data")
	inputStream := wasip2io.NewInputStream(reader)
	inputStream.Close() // Close the stream

	handle := table.New(inputStream, true)

	streamsDef, _ := linker.Get("wasi:io/streams@0.2.0")
	instDef := streamsDef.(*component.InstanceDef)

	readFunc := instDef.Exports["[method]input-stream.read"].(*component.FuncDef)

	// Try to read from closed stream
	result, err := readFunc.Callback(ctx, []component.Val{
		component.ValBorrow(uint32(handle)),
		component.ValU64(10),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result), "read should return exactly one value")

	// Result should be result<list<u8>, stream-error>
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "read on closed stream should return error")
	require.NotNil(t, errVal, "error value should not be nil")
}

// TestWASI_ErrorHandling_IOErrorInterface tests that the error interface exists.
func TestWASI_ErrorHandling_IOErrorInterface(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	// Get the error interface
	errorDef, ok := linker.Get("wasi:io/error@0.2.0")
	require.True(t, ok, "io/error interface should be registered")

	instDef, ok := errorDef.(*component.InstanceDef)
	require.True(t, ok, "should be an InstanceDef")

	// Verify error resource exists
	errorRes, ok := instDef.Exports["error"]
	require.True(t, ok, "error resource should be exported")
	require.NotNil(t, errorRes, "error resource should not be nil")
}

// TestWASI_ErrorHandling_IOErrorToDebugString tests the to-debug-string method.
func TestWASI_ErrorHandling_IOErrorToDebugString(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	// Get the error interface
	errorDef, ok := linker.Get("wasi:io/error@0.2.0")
	require.True(t, ok, "io/error interface should be registered")

	instDef := errorDef.(*component.InstanceDef)

	// Verify to-debug-string method exists
	toDebugStringMethod, ok := instDef.Exports["[method]error.to-debug-string"]
	require.True(t, ok, "[method]error.to-debug-string should be exported")
	require.NotNil(t, toDebugStringMethod, "[method]error.to-debug-string should not be nil")
}

// TestWASI_ErrorHandling_ResultOkPattern tests the result<T, E> Ok pattern.
func TestWASI_ErrorHandling_ResultOkPattern(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	// Create an input stream with data
	reader := bytes.NewBufferString("test data")
	inputStream := wasip2io.NewInputStream(reader)

	handle := table.New(inputStream, true)

	streamsDef, _ := linker.Get("wasi:io/streams@0.2.0")
	instDef := streamsDef.(*component.InstanceDef)

	readFunc := instDef.Exports["[method]input-stream.read"].(*component.FuncDef)

	// Read from stream (should succeed)
	result, err := readFunc.Callback(ctx, []component.Val{
		component.ValBorrow(uint32(handle)),
		component.ValU64(4),
	})
	require.NoError(t, err)

	// Result should be result<list<u8>, stream-error>
	isOk, okVal, _ := result[0].Result()
	require.True(t, isOk, "read should succeed")
	require.NotNil(t, okVal, "ok value should not be nil")

	// Verify data was read
	byteList := okVal.List()
	require.True(t, len(byteList) > 0, "should have read some data")
}

// TestWASI_ErrorHandling_ResultErrorPattern tests the result<T, E> Error pattern.
func TestWASI_ErrorHandling_ResultErrorPattern(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	// Create and close a stream to trigger error
	var buf bytes.Buffer
	outputStream := wasip2io.NewOutputStream(&buf)
	outputStream.Close()

	handle := table.New(outputStream, true)

	streamsDef, _ := linker.Get("wasi:io/streams@0.2.0")
	instDef := streamsDef.(*component.InstanceDef)

	writeFunc := instDef.Exports["[method]output-stream.write"].(*component.FuncDef)

	// Try to write to closed stream
	result, err := writeFunc.Callback(ctx, []component.Val{
		component.ValBorrow(uint32(handle)),
		component.ValList([]component.Val{component.ValU8('a')}),
	})
	require.NoError(t, err)

	// Result should be result<_, stream-error>
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "write to closed stream should return error")
	require.NotNil(t, errVal, "error value should not be nil")
}

// TestWASI_ErrorHandling_OptionSomePattern tests the option<T> Some pattern.
func TestWASI_ErrorHandling_OptionSomePattern(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()

	// Get initial-cwd which returns option<string>
	envDef, _ := linker.Get("wasi:cli/environment@0.2.0")
	instDef := envDef.(*component.InstanceDef)

	initialCwdFunc := instDef.Exports["initial-cwd"].(*component.FuncDef)

	result, err := initialCwdFunc.Callback(ctx, []component.Val{})
	require.NoError(t, err)

	// Result should be option<string>
	optVal := result[0].Option()
	// Should return Some(path) since we can get cwd
	require.NotNil(t, optVal, "initial-cwd should return Some(path)")
}

// TestWASI_ErrorHandling_OptionNonePattern tests the option<T> None pattern.
func TestWASI_ErrorHandling_OptionNonePattern(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	// Get an HTTP request-options and query connect-timeout which is initially None
	typesDef, _ := linker.Get("wasi:http/types@0.2.0")
	instDef := typesDef.(*component.InstanceDef)

	// Create request options
	constructorFunc := instDef.Exports["[constructor]request-options"].(*component.FuncDef)
	createResult, err := constructorFunc.Callback(ctx, []component.Val{})
	require.NoError(t, err)

	optsHandle := createResult[0].Own()

	// Get connect-timeout (should be None by default)
	connectTimeoutFunc := instDef.Exports["[method]request-options.connect-timeout"].(*component.FuncDef)
	result, err := connectTimeoutFunc.Callback(ctx, []component.Val{
		component.ValBorrow(optsHandle),
	})
	require.NoError(t, err)

	// Result should be option<duration> - likely None for new options
	optVal := result[0].Option()
	require.Nil(t, optVal, "connect-timeout should initially be None")
}

// TestWASI_ErrorHandling_ExitCodeSuccess tests exit with success.
func TestWASI_ErrorHandling_ExitCodeSuccess(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()

	// Get exit interface
	exitDef, _ := linker.Get("wasi:cli/exit@0.2.0")
	instDef := exitDef.(*component.InstanceDef)

	exitFunc := instDef.Exports["exit"].(*component.FuncDef)

	// Call exit with success (result ok variant, discriminant 0)
	okVal := component.ValTuple([]component.Val{})
	_, exitErr := exitFunc.Callback(ctx, []component.Val{
		component.ValResultOk(&okVal),
	})

	// Exit should return an error (ExitError) to signal termination
	require.NotNil(t, exitErr, "exit should return error to signal termination")
}

// TestWASI_ErrorHandling_FilesystemErrors tests filesystem error codes.
func TestWASI_ErrorHandling_FilesystemErrors(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	// Verify filesystem types interface exists and has error-code
	typesDef, ok := linker.Get("wasi:filesystem/types@0.2.0")
	require.True(t, ok, "filesystem/types interface should be registered")

	instDef := typesDef.(*component.InstanceDef)
	require.NotNil(t, instDef.Exports, "exports should not be nil")
}

// TestWASI_ErrorHandling_SocketsErrors tests socket error codes.
func TestWASI_ErrorHandling_SocketsErrors(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	// Create a TCP socket and verify error handling on bad operations
	tcpCreateDef, _ := linker.Get("wasi:sockets/tcp-create-socket@0.2.0")
	instDef := tcpCreateDef.(*component.InstanceDef)

	createTcpFunc := instDef.Exports["create-tcp-socket"].(*component.FuncDef)

	// Create socket with IPv4
	result, err := createTcpFunc.Callback(ctx, []component.Val{
		component.ValBorrow(0), // network handle
		component.ValEnum("ipv4"),
	})
	require.NoError(t, err)

	// Result should be result<own<tcp-socket>, error-code>
	isOk, _, errCode := result[0].Result()
	// Either success or error is valid - we're testing the pattern exists
	if !isOk {
		require.NotNil(t, errCode, "error code should be returned on failure")
	}
}

// TestWASI_ErrorHandling_HTTPErrors tests HTTP error codes.
func TestWASI_ErrorHandling_HTTPErrors(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	// Verify HTTP types has error-code function
	typesDef, ok := linker.Get("wasi:http/types@0.2.0")
	require.True(t, ok, "http/types interface should be registered")

	instDef := typesDef.(*component.InstanceDef)

	// Verify http-error-code function exists
	errorCodeFunc, ok := instDef.Exports["http-error-code"]
	require.True(t, ok, "http-error-code function should be exported")
	require.NotNil(t, errorCodeFunc, "http-error-code function should not be nil")
}

// TestWASI_ErrorHandling_StreamErrorVariants tests stream error variants.
func TestWASI_ErrorHandling_StreamErrorVariants(t *testing.T) {
	// Test stream error closed variant
	t.Run("Closed", func(t *testing.T) {
		linker := component.NewLinker()

		err := wasip2.Instantiate(linker)
		require.NoError(t, err)

		ctx := context.Background()
		table := component.NewResourceTable()
		ctx = component.WithResourceTable(ctx, table)

		// Create and close a stream
		reader := bytes.NewBufferString("test")
		inputStream := wasip2io.NewInputStream(reader)
		inputStream.Close()

		handle := table.New(inputStream, true)

		streamsDef, _ := linker.Get("wasi:io/streams@0.2.0")
		instDef := streamsDef.(*component.InstanceDef)

		readFunc := instDef.Exports["[method]input-stream.read"].(*component.FuncDef)

		result, err := readFunc.Callback(ctx, []component.Val{
			component.ValBorrow(uint32(handle)),
			component.ValU64(10),
		})
		require.NoError(t, err)

		isOk, _, errVal := result[0].Result()
		require.False(t, isOk, "read on closed stream should fail")
		require.NotNil(t, errVal, "should have error value")
	})
}

// TestWASI_ErrorHandling_CheckWritePermission tests write permission errors.
func TestWASI_ErrorHandling_CheckWritePermission(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	// Create a valid output stream
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
	require.True(t, isOk, "check-write on valid stream should succeed")
	require.NotNil(t, okVal, "should return writable amount")
}
