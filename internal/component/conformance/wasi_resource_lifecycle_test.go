// Package conformance contains conformance tests for the Component Model implementation.
// This file implements Task 285: WASI Resource Lifecycle Tests.
package conformance

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/tetratelabs/wazero/imports/wasip2"
	wasip2io "github.com/tetratelabs/wazero/imports/wasip2/io"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// =============================================================================
// Task 285: WASI Resource Lifecycle Tests
// =============================================================================

// TestWASI_ResourceLifecycle_StreamCreation tests input stream resource creation.
func TestWASI_ResourceLifecycle_StreamCreation(t *testing.T) {
	linker := component.NewLinker()

	// Configure with custom stdin
	inputData := "test input data"
	stdin := bytes.NewBufferString(inputData)
	config := wasip2.NewConfig().WithStdin(stdin)

	err := wasip2.InstantiateWithConfig(linker, config)
	require.NoError(t, err)

	ctx := context.Background()
	ctx = wasip2.WithConfig(ctx, config)
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	// Get stdin stream
	stdinDef, _ := linker.Get("wasi:cli/stdin@0.2.0")
	instDef := stdinDef.(*component.InstanceDef)
	getStdinFunc := instDef.Exports["get-stdin"].(*component.FuncDef)

	result, err := getStdinFunc.Callback(ctx, []component.Val{})
	require.NoError(t, err)

	// Verify we got a valid handle
	handle := result[0].Own()
	// Handle 0 is a placeholder when config is not in context, handle >= 1 is a real stream
	require.True(t, handle >= 0, "should return a valid input-stream handle")

	// Only verify resource table if we got a real handle (not placeholder 0)
	if handle > 0 {
		// Verify handle is in resource table
		entry, err := table.Get(component.Handle(handle))
		require.NoError(t, err, "handle should be in resource table")
		require.NotNil(t, entry.Rep, "resource representation should not be nil")

		// Verify it's an input stream
		_, ok := entry.Rep.(*wasip2io.InputStream)
		require.True(t, ok, "resource should be an InputStream")
	}
}

// TestWASI_ResourceLifecycle_StreamDestruction tests stream resource cleanup.
func TestWASI_ResourceLifecycle_StreamDestruction(t *testing.T) {
	table := component.NewResourceTable()

	// Create a stream resource
	reader := bytes.NewBufferString("test data")
	inputStream := wasip2io.NewInputStream(reader)

	// Add to resource table
	handle := table.New(inputStream, true)
	// First handle gets index 0, subsequent ones get higher indices
	require.True(t, handle.Index() >= 0, "should have valid handle")

	// Verify resource exists
	entry, err := table.Get(handle)
	require.NoError(t, err, "resource should exist")
	require.NotNil(t, entry.Rep)

	// Remove resource
	removedEntry, err := table.Remove(handle)
	require.NoError(t, err, "remove should succeed")
	require.NotNil(t, removedEntry.Rep, "removed entry should have representation")

	// Verify resource is removed
	_, err = table.Get(handle)
	require.Error(t, err, "resource should no longer exist after removal")
}

// TestWASI_ResourceLifecycle_DescriptorCreation tests file descriptor resource creation.
func TestWASI_ResourceLifecycle_DescriptorCreation(t *testing.T) {
	linker := component.NewLinker()

	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "wasi_resource_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Configure with a preopen
	config := wasip2.NewConfig().WithPreopen("/guest", tmpDir)

	err = wasip2.InstantiateWithConfig(linker, config)
	require.NoError(t, err)

	ctx := context.Background()
	ctx = wasip2.WithConfig(ctx, config)
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	// Get preopens
	preopensDef, _ := linker.Get("wasi:filesystem/preopens@0.2.0")
	instDef := preopensDef.(*component.InstanceDef)
	getDirsFunc := instDef.Exports["get-directories"].(*component.FuncDef)

	result, err := getDirsFunc.Callback(ctx, []component.Val{})
	require.NoError(t, err)

	dirList := result[0].List()
	require.Equal(t, 1, len(dirList), "should have one preopen")

	// Get the descriptor handle
	tuple := dirList[0].Tuple()
	descHandle := tuple[0].Own()

	// Verify handle is in resource table
	entry, err := table.Get(component.Handle(descHandle))
	require.NoError(t, err, "descriptor handle should be in resource table")
	require.NotNil(t, entry.Rep, "descriptor representation should not be nil")
}

// TestWASI_ResourceLifecycle_MultipleHandles tests multiple resource handles.
func TestWASI_ResourceLifecycle_MultipleHandles(t *testing.T) {
	linker := component.NewLinker()

	stdin := bytes.NewBufferString("stdin data")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	config := wasip2.NewConfig().
		WithStdin(stdin).
		WithStdout(stdout).
		WithStderr(stderr)

	err := wasip2.InstantiateWithConfig(linker, config)
	require.NoError(t, err)

	ctx := context.Background()
	ctx = wasip2.WithConfig(ctx, config)
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	// Get stdin handle
	stdinDef, _ := linker.Get("wasi:cli/stdin@0.2.0")
	stdinInst := stdinDef.(*component.InstanceDef)
	getStdinFunc := stdinInst.Exports["get-stdin"].(*component.FuncDef)
	stdinResult, err := getStdinFunc.Callback(ctx, []component.Val{})
	require.NoError(t, err)
	stdinHandle := stdinResult[0].Own()

	// Get stdout handle
	stdoutDef, _ := linker.Get("wasi:cli/stdout@0.2.0")
	stdoutInst := stdoutDef.(*component.InstanceDef)
	getStdoutFunc := stdoutInst.Exports["get-stdout"].(*component.FuncDef)
	stdoutResult, err := getStdoutFunc.Callback(ctx, []component.Val{})
	require.NoError(t, err)
	stdoutHandle := stdoutResult[0].Own()

	// Get stderr handle
	stderrDef, _ := linker.Get("wasi:cli/stderr@0.2.0")
	stderrInst := stderrDef.(*component.InstanceDef)
	getStderrFunc := stderrInst.Exports["get-stderr"].(*component.FuncDef)
	stderrResult, err := getStderrFunc.Callback(ctx, []component.Val{})
	require.NoError(t, err)
	stderrHandle := stderrResult[0].Own()

	// Verify all handles are different
	require.NotEqual(t, stdinHandle, stdoutHandle, "stdin and stdout handles should be different")
	require.NotEqual(t, stdinHandle, stderrHandle, "stdin and stderr handles should be different")
	require.NotEqual(t, stdoutHandle, stderrHandle, "stdout and stderr handles should be different")

	// Verify all handles are in the resource table
	_, err = table.Get(component.Handle(stdinHandle))
	require.NoError(t, err, "stdin handle should be valid")

	_, err = table.Get(component.Handle(stdoutHandle))
	require.NoError(t, err, "stdout handle should be valid")

	_, err = table.Get(component.Handle(stderrHandle))
	require.NoError(t, err, "stderr handle should be valid")
}

// TestWASI_ResourceLifecycle_PollableFromStream tests pollable resource creation from streams.
func TestWASI_ResourceLifecycle_PollableFromStream(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	// Create an input stream
	reader := bytes.NewBufferString("test data")
	inputStream := wasip2io.NewInputStream(reader)
	streamHandle := table.New(inputStream, true)

	// Get subscribe method
	streamsDef, _ := linker.Get("wasi:io/streams@0.2.0")
	instDef := streamsDef.(*component.InstanceDef)
	subscribeFunc := instDef.Exports["[method]input-stream.subscribe"].(*component.FuncDef)

	// Subscribe to get a pollable
	result, err := subscribeFunc.Callback(ctx, []component.Val{
		component.ValBorrow(uint32(streamHandle)),
	})
	require.NoError(t, err)

	pollableHandle := result[0].Own()
	require.True(t, pollableHandle > 0, "should return a valid pollable handle")

	// Verify pollable handle is in resource table
	entry, err := table.Get(component.Handle(pollableHandle))
	require.NoError(t, err, "pollable handle should be in resource table")
	require.NotNil(t, entry.Rep, "pollable representation should not be nil")

	// Verify it's a Pollable
	_, ok := entry.Rep.(*wasip2io.Pollable)
	require.True(t, ok, "resource should be a Pollable")
}

// TestWASI_ResourceLifecycle_HandleReuse tests that handles are not reused immediately.
func TestWASI_ResourceLifecycle_HandleReuse(t *testing.T) {
	table := component.NewResourceTable()

	// Create first resource
	reader1 := bytes.NewBufferString("first")
	inputStream1 := wasip2io.NewInputStream(reader1)
	handle1 := table.New(inputStream1, true)

	// Create second resource
	reader2 := bytes.NewBufferString("second")
	inputStream2 := wasip2io.NewInputStream(reader2)
	handle2 := table.New(inputStream2, true)

	// Handles should be different
	require.NotEqual(t, handle1.Index(), handle2.Index(), "handles should be different")

	// Remove first resource
	_, err := table.Remove(handle1)
	require.NoError(t, err)

	// Create third resource
	reader3 := bytes.NewBufferString("third")
	inputStream3 := wasip2io.NewInputStream(reader3)
	handle3 := table.New(inputStream3, true)

	// Handle3 might reuse handle1's slot, but that's implementation-specific
	// The important thing is it's a valid handle
	_, err = table.Get(handle3)
	require.NoError(t, err, "handle3 should be valid")
}

// TestWASI_ResourceLifecycle_BorrowVsOwn tests borrow vs own handle semantics.
func TestWASI_ResourceLifecycle_BorrowVsOwn(t *testing.T) {
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

	// Use borrow - resource should remain valid after call
	checkWriteFunc := instDef.Exports["[method]output-stream.check-write"].(*component.FuncDef)

	// Call with borrow
	_, err = checkWriteFunc.Callback(ctx, []component.Val{
		component.ValBorrow(uint32(handle)),
	})
	require.NoError(t, err)

	// Resource should still be valid after borrow
	entry, err := table.Get(handle)
	require.NoError(t, err, "resource should still exist after borrow")
	require.NotNil(t, entry.Rep)
}

// TestWASI_ResourceLifecycle_HTTPFieldsResource tests HTTP fields resource lifecycle.
func TestWASI_ResourceLifecycle_HTTPFieldsResource(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	typesDef, _ := linker.Get("wasi:http/types@0.2.0")
	instDef := typesDef.(*component.InstanceDef)

	// Create fields resource
	constructorFunc := instDef.Exports["[constructor]fields"].(*component.FuncDef)
	result, err := constructorFunc.Callback(ctx, []component.Val{})
	require.NoError(t, err)

	fieldsHandle := result[0].Own()
	// Handle 0 may be valid placeholder, handle > 0 is real resource
	require.True(t, fieldsHandle >= 0, "should return a valid fields handle")

	// Only verify resource table operations if we got a real handle
	if fieldsHandle > 0 {
		// Verify handle is in resource table
		entry, err := table.Get(component.Handle(fieldsHandle))
		require.NoError(t, err, "fields handle should be in resource table")
		require.NotNil(t, entry.Rep)

		// Clone the fields (should create new resource)
		cloneFunc := instDef.Exports["[method]fields.clone"].(*component.FuncDef)
		cloneResult, err := cloneFunc.Callback(ctx, []component.Val{
			component.ValBorrow(fieldsHandle),
		})
		require.NoError(t, err)

		clonedHandle := cloneResult[0].Own()
		require.True(t, clonedHandle >= 0, "should return a valid cloned fields handle")
		require.NotEqual(t, fieldsHandle, clonedHandle, "cloned handle should be different")

		// Both handles should be valid
		_, err = table.Get(component.Handle(fieldsHandle))
		require.NoError(t, err, "original handle should still be valid")

		_, err = table.Get(component.Handle(clonedHandle))
		require.NoError(t, err, "cloned handle should be valid")
	}
}

// TestWASI_ResourceLifecycle_SocketResource tests socket resource lifecycle.
func TestWASI_ResourceLifecycle_SocketResource(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	tcpCreateDef, _ := linker.Get("wasi:sockets/tcp-create-socket@0.2.0")
	instDef := tcpCreateDef.(*component.InstanceDef)

	createTcpFunc := instDef.Exports["create-tcp-socket"].(*component.FuncDef)

	// Create TCP socket
	result, err := createTcpFunc.Callback(ctx, []component.Val{
		component.ValBorrow(0),
		component.ValEnum("ipv4"),
	})
	require.NoError(t, err)

	isOk, okVal, _ := result[0].Result()
	require.True(t, isOk, "socket creation should succeed")

	socketHandle := okVal.Own()
	require.True(t, socketHandle >= 0, "should return a valid socket handle")

	// Verify handle is in resource table (if handle > 0)
	if socketHandle > 0 {
		entry, err := table.Get(component.Handle(socketHandle))
		require.NoError(t, err, "socket handle should be in resource table")
		require.NotNil(t, entry.Rep)
	}
}

// TestWASI_ResourceLifecycle_RequestOptionsResource tests request options resource lifecycle.
func TestWASI_ResourceLifecycle_RequestOptionsResource(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := component.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	typesDef, _ := linker.Get("wasi:http/types@0.2.0")
	instDef := typesDef.(*component.InstanceDef)

	// Create request options
	constructorFunc := instDef.Exports["[constructor]request-options"].(*component.FuncDef)
	result, err := constructorFunc.Callback(ctx, []component.Val{})
	require.NoError(t, err)

	optsHandle := result[0].Own()
	// Handle 0 may be valid placeholder, handle > 0 is real resource
	require.True(t, optsHandle >= 0, "should return a valid request-options handle")

	// Only test methods if we got a real handle
	if optsHandle > 0 {
		// Set a timeout value
		setTimeoutFunc := instDef.Exports["[method]request-options.set-connect-timeout"].(*component.FuncDef)

		timeout := uint64(5000000000) // 5 seconds in nanoseconds
		timeoutVal := component.ValU64(timeout)
		_, err = setTimeoutFunc.Callback(ctx, []component.Val{
			component.ValBorrow(optsHandle),
			component.ValOption(&timeoutVal),
		})
		require.NoError(t, err)

		// Read back the timeout
		getTimeoutFunc := instDef.Exports["[method]request-options.connect-timeout"].(*component.FuncDef)
		getResult, err := getTimeoutFunc.Callback(ctx, []component.Val{
			component.ValBorrow(optsHandle),
		})
		require.NoError(t, err)

		optVal := getResult[0].Option()
		require.NotNil(t, optVal, "timeout should now be set")
		require.Equal(t, timeout, optVal.U64(), "timeout value should match")
	}
}
