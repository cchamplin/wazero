// Package conformance contains conformance tests for the Component Model implementation.
// This file implements Task 281: WASI Poll Conformance Tests.
package conformance

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/imports/wasip2"
	wasip2io "github.com/tetratelabs/wazero/imports/wasip2/io"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// =============================================================================
// Task 281: WASI Poll Conformance Tests
// =============================================================================

// TestWASI_Poll_PollFunctionExists tests that the poll function exists.
func TestWASI_Poll_PollFunctionExists(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	// Get the poll interface
	pollDef, ok := linker.Get("wasi:io/poll@0.2.0")
	require.True(t, ok, "poll interface should be registered")

	instDef, ok := pollDef.(*component.InstanceDef)
	require.True(t, ok, "should be an InstanceDef")

	// Verify poll function exists
	pollFunc, ok := instDef.Exports["poll"]
	require.True(t, ok, "poll function should be exported")
	require.NotNil(t, pollFunc, "poll function should not be nil")
}

// TestWASI_Poll_PollableResource tests that the pollable resource exists.
func TestWASI_Poll_PollableResource(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	pollDef, ok := linker.Get("wasi:io/poll@0.2.0")
	require.True(t, ok, "poll interface should be registered")

	instDef := pollDef.(*component.InstanceDef)

	// Verify pollable resource exists
	pollableRes, ok := instDef.Exports["pollable"]
	require.True(t, ok, "pollable resource should be exported")
	require.NotNil(t, pollableRes, "pollable resource should not be nil")
}

// TestWASI_Poll_PollableReady tests the pollable.ready method.
func TestWASI_Poll_PollableReady(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := runtime.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	// Create a ready pollable
	pollable := wasip2io.NewReadyPollable()
	handle := table.New(pollable, true)

	pollDef, _ := linker.Get("wasi:io/poll@0.2.0")
	instDef := pollDef.(*component.InstanceDef)

	readyFunc, ok := instDef.Exports["[method]pollable.ready"]
	require.True(t, ok, "pollable.ready method should be exported")

	funcDef := readyFunc.(*component.FuncDef)

	result, err := funcDef.Callback(ctx, []types.Val{
		types.ValBorrow(uint32(handle)),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result), "ready should return exactly one value")

	isReady := result[0].Bool()
	require.True(t, isReady, "ready pollable should return true")
}

// TestWASI_Poll_PollableBlock tests the pollable.block method.
func TestWASI_Poll_PollableBlock(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := runtime.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	// Create a ready pollable (block should return immediately)
	pollable := wasip2io.NewReadyPollable()
	handle := table.New(pollable, true)

	pollDef, _ := linker.Get("wasi:io/poll@0.2.0")
	instDef := pollDef.(*component.InstanceDef)

	blockFunc, ok := instDef.Exports["[method]pollable.block"]
	require.True(t, ok, "pollable.block method should be exported")

	funcDef := blockFunc.(*component.FuncDef)

	// Block should return without hanging for a ready pollable
	result, err := funcDef.Callback(ctx, []types.Val{
		types.ValBorrow(uint32(handle)),
	})
	require.NoError(t, err)
	require.Equal(t, 0, len(result), "block should return no values")
}

// TestWASI_Poll_PollList tests the poll function with a list of pollables.
func TestWASI_Poll_PollList(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := runtime.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	// Create multiple ready pollables
	pollable1 := wasip2io.NewReadyPollable()
	pollable2 := wasip2io.NewReadyPollable()
	pollable3 := wasip2io.NewReadyPollable()

	handle1 := table.New(pollable1, true)
	handle2 := table.New(pollable2, true)
	handle3 := table.New(pollable3, true)

	pollDef, _ := linker.Get("wasi:io/poll@0.2.0")
	instDef := pollDef.(*component.InstanceDef)

	pollFunc := instDef.Exports["poll"].(*component.FuncDef)

	// Create list of borrow<pollable>
	pollableList := []types.Val{
		types.ValBorrow(uint32(handle1)),
		types.ValBorrow(uint32(handle2)),
		types.ValBorrow(uint32(handle3)),
	}

	result, err := pollFunc.Callback(ctx, []types.Val{
		types.ValList(pollableList),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result), "poll should return exactly one value")

	// Result should be a list of indices of ready pollables
	readyIndices := result[0].List()
	require.True(t, len(readyIndices) > 0, "should have at least one ready pollable")

	// All pollables are ready, so all indices should be returned
	require.Equal(t, 3, len(readyIndices), "all 3 pollables should be ready")
}

// TestWASI_Poll_PollEmptyList tests poll with an empty list.
func TestWASI_Poll_PollEmptyList(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := runtime.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	pollDef, _ := linker.Get("wasi:io/poll@0.2.0")
	instDef := pollDef.(*component.InstanceDef)

	pollFunc := instDef.Exports["poll"].(*component.FuncDef)

	// Call poll with empty list
	result, err := pollFunc.Callback(ctx, []types.Val{
		types.ValList([]types.Val{}),
	})
	require.NoError(t, err)

	// Should return empty list
	readyIndices := result[0].List()
	require.Equal(t, 0, len(readyIndices), "poll of empty list should return empty list")
}

// TestWASI_Poll_PollSinglePollable tests poll with a single pollable.
func TestWASI_Poll_PollSinglePollable(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := runtime.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	// Create a single ready pollable
	pollable := wasip2io.NewReadyPollable()
	handle := table.New(pollable, true)

	pollDef, _ := linker.Get("wasi:io/poll@0.2.0")
	instDef := pollDef.(*component.InstanceDef)

	pollFunc := instDef.Exports["poll"].(*component.FuncDef)

	pollableList := []types.Val{
		types.ValBorrow(uint32(handle)),
	}

	result, err := pollFunc.Callback(ctx, []types.Val{
		types.ValList(pollableList),
	})
	require.NoError(t, err)

	readyIndices := result[0].List()
	require.Equal(t, 1, len(readyIndices), "should have one ready pollable")
	require.Equal(t, uint32(0), readyIndices[0].U32(), "index should be 0")
}

// TestWASI_Poll_InterfaceRegistration tests that the poll interface is properly registered.
func TestWASI_Poll_InterfaceRegistration(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	// Verify poll interface is registered
	def, ok := linker.Get("wasi:io/poll@0.2.0")
	require.True(t, ok, "wasi:io/poll@0.2.0 should be registered")
	require.NotNil(t, def, "poll interface definition should not be nil")
}

// TestWASI_Poll_AllExportsExist tests that all expected exports exist.
func TestWASI_Poll_AllExportsExist(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	pollDef, ok := linker.Get("wasi:io/poll@0.2.0")
	require.True(t, ok, "poll interface should be registered")

	instDef := pollDef.(*component.InstanceDef)

	// Expected exports
	expectedExports := []string{
		"pollable",               // resource
		"[method]pollable.ready", // method
		"[method]pollable.block", // method
		"poll",                   // function
	}

	for _, export := range expectedExports {
		exportDef, ok := instDef.Exports[export]
		require.True(t, ok, "export %s should exist", export)
		require.NotNil(t, exportDef, "export %s should not be nil", export)
	}
}

// TestWASI_Poll_PollableWithCustomReadyFunction tests pollable with custom ready logic.
func TestWASI_Poll_PollableWithCustomReadyFunction(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := runtime.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	// Create a pollable with custom ready function that returns false initially
	callCount := 0
	pollable := wasip2io.NewPollable(
		func() bool {
			callCount++
			return callCount > 1 // Ready after first check
		},
		nil,
	)
	handle := table.New(pollable, true)

	pollDef, _ := linker.Get("wasi:io/poll@0.2.0")
	instDef := pollDef.(*component.InstanceDef)

	readyFunc := instDef.Exports["[method]pollable.ready"].(*component.FuncDef)

	// First call - not ready
	result1, err := readyFunc.Callback(ctx, []types.Val{
		types.ValBorrow(uint32(handle)),
	})
	require.NoError(t, err)
	require.False(t, result1[0].Bool(), "first check should return false")

	// Second call - ready
	result2, err := readyFunc.Callback(ctx, []types.Val{
		types.ValBorrow(uint32(handle)),
	})
	require.NoError(t, err)
	require.True(t, result2[0].Bool(), "second check should return true")
}

// TestWASI_Poll_MixedReadyStates tests poll with pollables in different states.
func TestWASI_Poll_MixedReadyStates(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()
	table := runtime.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	// Create pollables with different ready states
	pollableReady := wasip2io.NewReadyPollable()
	pollableNotReady := wasip2io.NewPollable(func() bool { return false }, nil)
	pollableReady2 := wasip2io.NewReadyPollable()

	handle1 := table.New(pollableReady, true)    // index 0, ready
	handle2 := table.New(pollableNotReady, true) // index 1, not ready
	handle3 := table.New(pollableReady2, true)   // index 2, ready

	pollDef, _ := linker.Get("wasi:io/poll@0.2.0")
	instDef := pollDef.(*component.InstanceDef)

	pollFunc := instDef.Exports["poll"].(*component.FuncDef)

	pollableList := []types.Val{
		types.ValBorrow(uint32(handle1)),
		types.ValBorrow(uint32(handle2)),
		types.ValBorrow(uint32(handle3)),
	}

	result, err := pollFunc.Callback(ctx, []types.Val{
		types.ValList(pollableList),
	})
	require.NoError(t, err)

	readyIndices := result[0].List()
	require.Equal(t, 2, len(readyIndices), "should have 2 ready pollables")

	// Collect ready indices
	indices := make(map[uint32]bool)
	for _, idx := range readyIndices {
		indices[idx.U32()] = true
	}

	require.True(t, indices[0], "pollable at index 0 should be ready")
	require.False(t, indices[1], "pollable at index 1 should not be ready")
	require.True(t, indices[2], "pollable at index 2 should be ready")
}
