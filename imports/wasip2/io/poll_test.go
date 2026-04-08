// imports/wasip2/io/poll_test.go

package io

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestPollable_Ready_Immediate(t *testing.T) {
	// A pollable that is already ready
	p := &Pollable{readyFn: func() bool { return true }}
	require.True(t, p.Ready())
}

func TestPollable_Ready_NotReady(t *testing.T) {
	p := &Pollable{readyFn: func() bool { return false }}
	require.False(t, p.Ready())
}

func TestPollable_Ready_NilFn(t *testing.T) {
	// Nil readyFn should default to ready
	p := &Pollable{}
	require.True(t, p.Ready())
}

func TestPollable_Block(t *testing.T) {
	blocked := false
	p := &Pollable{
		readyFn: func() bool { return blocked },
		blockFn: func() { blocked = true },
	}
	require.False(t, p.Ready())
	p.Block()
	require.True(t, p.Ready())
}

func TestPollable_Block_NilFn(t *testing.T) {
	// Block with nil blockFn should not panic
	p := &Pollable{}
	p.Block() // Should not panic
}

func TestNewPollable(t *testing.T) {
	ready := false
	p := NewPollable(
		func() bool { return ready },
		func() { ready = true },
	)
	require.False(t, p.Ready())
	p.Block()
	require.True(t, p.Ready())
}

func TestNewReadyPollable(t *testing.T) {
	p := NewReadyPollable()
	require.True(t, p.Ready())
}

func TestInstantiatePoll(t *testing.T) {
	linker := component.NewLinker()
	err := instantiatePoll(linker)
	require.NoError(t, err)

	// Verify interface is registered
	def, err := linker.MatchImport("wasi:io/poll@0.2.0")
	require.NoError(t, err)

	// Verify it's an instance definition
	instDef, ok := def.(*component.InstanceDef)
	require.True(t, ok, "expected InstanceDef")

	// Verify exports exist
	_, hasResource := instDef.Exports["pollable"]
	require.True(t, hasResource, "pollable resource should be defined")

	_, hasReadyMethod := instDef.Exports["[method]pollable.ready"]
	require.True(t, hasReadyMethod, "ready method should be defined")

	_, hasBlockMethod := instDef.Exports["[method]pollable.block"]
	require.True(t, hasBlockMethod, "block method should be defined")

	_, hasPollFunc := instDef.Exports["poll"]
	require.True(t, hasPollFunc, "poll function should be defined")
}

func TestInstantiatePoll_Duplicate(t *testing.T) {
	linker := component.NewLinker()

	// First registration should succeed
	err := instantiatePoll(linker)
	require.NoError(t, err)

	// Second registration should fail
	err = instantiatePoll(linker)
	require.Error(t, err)
}

// Tests for host functions with ResourceTable

func TestPollableReady_HostFunction(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Create a pollable that is ready
	pollable := NewReadyPollable()
	handle, errHandle113 := table.NewResourceHandle(pollable, true, pollableResourceType)
	if errHandle113 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errHandle113)
	}

	args := []types.Val{
		types.ValBorrow(uint32(handle)),
	}
	results, err := pollableReady(ctx, args)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.True(t, results[0].Bool())
}

func TestPollableReady_HostFunction_NotReady(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Create a pollable that is not ready
	pollable := NewPollable(func() bool { return false }, nil)
	handle, errHandle130 := table.NewResourceHandle(pollable, true, pollableResourceType)
	if errHandle130 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errHandle130)
	}

	args := []types.Val{
		types.ValBorrow(uint32(handle)),
	}
	results, err := pollableReady(ctx, args)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.False(t, results[0].Bool())
}

func TestPollableReady_HostFunction_InvalidHandle(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	args := []types.Val{
		types.ValBorrow(999), // Invalid handle
	}
	_, err := pollableReady(ctx, args)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid handle")
}

func TestPollableReady_HostFunction_NoResourceTable(t *testing.T) {
	ctx := context.Background() // No resource table

	args := []types.Val{
		types.ValBorrow(0),
	}
	_, err := pollableReady(ctx, args)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no resource table")
}

func TestPollableReady_HostFunction_WrongType(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Add something that's not a Pollable
	handle, errHandle := table.NewResourceHandle("not a pollable", true, pollableResourceType)
	if errHandle != nil {
		t.Fatalf("NewResourceHandle failed: %v", errHandle)
	}

	args := []types.Val{
		types.ValBorrow(uint32(handle)),
	}
	_, err := pollableReady(ctx, args)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not a Pollable")
}

func TestPollableBlock_HostFunction(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Create a pollable that blocks and then becomes ready
	blocked := false
	pollable := NewPollable(
		func() bool { return blocked },
		func() { blocked = true },
	)
	handle, errHandle189 := table.NewResourceHandle(pollable, true, pollableResourceType)
	if errHandle189 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errHandle189)
	}

	// Verify not ready before block
	require.False(t, pollable.Ready())

	args := []types.Val{
		types.ValBorrow(uint32(handle)),
	}
	results, err := pollableBlock(ctx, args)
	require.NoError(t, err)
	require.Nil(t, results) // block returns no values

	// Verify ready after block
	require.True(t, pollable.Ready())
}

func TestPollableBlock_HostFunction_NilBlockFn(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Create a pollable with no block function
	pollable := NewReadyPollable()
	handle, errHandle211 := table.NewResourceHandle(pollable, true, pollableResourceType)
	if errHandle211 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errHandle211)
	}

	args := []types.Val{
		types.ValBorrow(uint32(handle)),
	}
	// Should not panic even with nil blockFn
	results, err := pollableBlock(ctx, args)
	require.NoError(t, err)
	require.Nil(t, results)
}

func TestPollableBlock_HostFunction_InvalidHandle(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	args := []types.Val{
		types.ValBorrow(999),
	}
	_, err := pollableBlock(ctx, args)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid handle")
}

func TestPoll_AllReady(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Create three ready pollables
	p1 := NewReadyPollable()
	p2 := NewReadyPollable()
	p3 := NewReadyPollable()
	h1, errHandle242 := table.NewResourceHandle(p1, true, pollableResourceType)
	if errHandle242 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errHandle242)
	}
	h2, errHandle243 := table.NewResourceHandle(p2, true, pollableResourceType)
	if errHandle243 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errHandle243)
	}
	h3, errHandle244 := table.NewResourceHandle(p3, true, pollableResourceType)
	if errHandle244 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errHandle244)
	}

	args := []types.Val{
		types.ValList([]types.Val{
			types.ValBorrow(uint32(h1)),
			types.ValBorrow(uint32(h2)),
			types.ValBorrow(uint32(h3)),
		}),
	}
	results, err := pollPoll(ctx, args)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))

	// All indices should be returned
	indices := results[0].List()
	require.Equal(t, 3, len(indices))
	require.Equal(t, uint32(0), indices[0].U32())
	require.Equal(t, uint32(1), indices[1].U32())
	require.Equal(t, uint32(2), indices[2].U32())
}

func TestPoll_SomeReady(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Create pollables: first ready, second not ready, third ready
	p1 := NewReadyPollable()
	p2 := NewPollable(func() bool { return false }, nil)
	p3 := NewReadyPollable()
	h1, errHandle273 := table.NewResourceHandle(p1, true, pollableResourceType)
	if errHandle273 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errHandle273)
	}
	h2, errHandle274 := table.NewResourceHandle(p2, true, pollableResourceType)
	if errHandle274 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errHandle274)
	}
	h3, errHandle275 := table.NewResourceHandle(p3, true, pollableResourceType)
	if errHandle275 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errHandle275)
	}

	args := []types.Val{
		types.ValList([]types.Val{
			types.ValBorrow(uint32(h1)),
			types.ValBorrow(uint32(h2)),
			types.ValBorrow(uint32(h3)),
		}),
	}
	results, err := pollPoll(ctx, args)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))

	// Only indices 0 and 2 should be returned
	indices := results[0].List()
	require.Equal(t, 2, len(indices))
	require.Equal(t, uint32(0), indices[0].U32())
	require.Equal(t, uint32(2), indices[1].U32())
}

func TestPoll_NoneReady_WithBlock(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Create pollables that are initially not ready but become ready after block
	ready := false
	p1 := NewPollable(
		func() bool { return ready },
		func() { ready = true },
	)
	h1, errHandle305 := table.NewResourceHandle(p1, true, pollableResourceType)
	if errHandle305 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errHandle305)
	}

	args := []types.Val{
		types.ValList([]types.Val{
			types.ValBorrow(uint32(h1)),
		}),
	}
	results, err := pollPoll(ctx, args)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))

	// After blocking, index 0 should be returned
	indices := results[0].List()
	require.Equal(t, 1, len(indices))
	require.Equal(t, uint32(0), indices[0].U32())
}

func TestPoll_NoneReady_NoBlockFn(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Create pollable that is not ready and has no block function
	// This is an edge case - in practice, pollables should either be ready
	// or have a way to become ready via blocking
	p1 := NewPollable(func() bool { return false }, nil)
	h1, errHandle330 := table.NewResourceHandle(p1, true, pollableResourceType)
	if errHandle330 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errHandle330)
	}

	args := []types.Val{
		types.ValList([]types.Val{
			types.ValBorrow(uint32(h1)),
		}),
	}
	results, err := pollPoll(ctx, args)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))

	// Should return empty list since no pollable can become ready
	indices := results[0].List()
	require.Equal(t, 0, len(indices))
}

func TestPoll_EmptyList(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	args := []types.Val{
		types.ValList([]types.Val{}),
	}
	results, err := pollPoll(ctx, args)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))

	// Should return empty list
	indices := results[0].List()
	require.Equal(t, 0, len(indices))
}

func TestPoll_InvalidHandle(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	args := []types.Val{
		types.ValList([]types.Val{
			types.ValBorrow(999), // Invalid handle
		}),
	}
	_, err := pollPoll(ctx, args)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid handle")
}

func TestPoll_BlockConcurrent(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Create a pollable that becomes ready from another goroutine
	var mu sync.Mutex
	ready := false
	p1 := NewPollable(
		func() bool {
			mu.Lock()
			defer mu.Unlock()
			return ready
		},
		func() {
			// This simulates waiting for an external event
			time.Sleep(10 * time.Millisecond)
			mu.Lock()
			ready = true
			mu.Unlock()
		},
	)
	h1, errHandle397 := table.NewResourceHandle(p1, true, pollableResourceType)
	if errHandle397 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errHandle397)
	}

	args := []types.Val{
		types.ValList([]types.Val{
			types.ValBorrow(uint32(h1)),
		}),
	}

	start := time.Now()
	results, err := pollPoll(ctx, args)
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.Equal(t, 1, len(results))

	// Should have blocked for at least a short time
	require.True(t, elapsed >= 5*time.Millisecond, "poll should have blocked")

	indices := results[0].List()
	require.Equal(t, 1, len(indices))
	require.Equal(t, uint32(0), indices[0].U32())
}

func TestGetPollable(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	pollable := NewReadyPollable()
	handle, errHandle425 := table.NewResourceHandle(pollable, true, pollableResourceType)
	if errHandle425 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errHandle425)
	}

	retrieved, err := getPollable(ctx, uint32(handle))
	require.NoError(t, err)
	require.Equal(t, pollable, retrieved)
}

func TestGetPollable_NoTable(t *testing.T) {
	ctx := context.Background()

	_, err := getPollable(ctx, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no resource table")
}

func TestGetPollable_InvalidHandle(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	_, err := getPollable(ctx, 999)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid handle")
}

func TestGetPollable_WrongType(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	handle, errHandle := table.NewResourceHandle("not a pollable", true, pollableResourceType)
	if errHandle != nil {
		t.Fatalf("NewResourceHandle failed: %v", errHandle)
	}

	_, err := getPollable(ctx, uint32(handle))
	require.Error(t, err)
	require.Contains(t, err.Error(), "not a Pollable")
}

// Tests for poll multiplexing - Task 3.4

func TestPoll_MultiplePollables_FastOneReturnsFirst(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Create three pollables with different ready times:
	// - p1: becomes ready after 100ms (slow)
	// - p2: becomes ready after 10ms (fast)
	// - p3: becomes ready after 50ms (medium)
	// The poll function should return when p2 becomes ready (~10ms)
	// NOT wait for all of them

	var mu1, mu2, mu3 sync.Mutex
	ready1, ready2, ready3 := false, false, false

	p1 := NewPollableWithChannel(
		func() bool {
			mu1.Lock()
			defer mu1.Unlock()
			return ready1
		},
		func() {
			time.Sleep(100 * time.Millisecond)
			mu1.Lock()
			ready1 = true
			mu1.Unlock()
		},
	)

	p2 := NewPollableWithChannel(
		func() bool {
			mu2.Lock()
			defer mu2.Unlock()
			return ready2
		},
		func() {
			time.Sleep(10 * time.Millisecond)
			mu2.Lock()
			ready2 = true
			mu2.Unlock()
		},
	)

	p3 := NewPollableWithChannel(
		func() bool {
			mu3.Lock()
			defer mu3.Unlock()
			return ready3
		},
		func() {
			time.Sleep(50 * time.Millisecond)
			mu3.Lock()
			ready3 = true
			mu3.Unlock()
		},
	)

	h1, errHandle518 := table.NewResourceHandle(p1, true, pollableResourceType)
	if errHandle518 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errHandle518)
	}
	h2, errHandle519 := table.NewResourceHandle(p2, true, pollableResourceType)
	if errHandle519 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errHandle519)
	}
	h3, errHandle520 := table.NewResourceHandle(p3, true, pollableResourceType)
	if errHandle520 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errHandle520)
	}

	args := []types.Val{
		types.ValList([]types.Val{
			types.ValBorrow(uint32(h1)),
			types.ValBorrow(uint32(h2)),
			types.ValBorrow(uint32(h3)),
		}),
	}

	start := time.Now()
	results, err := pollPoll(ctx, args)
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.Equal(t, 1, len(results))

	// Should have returned around 10ms (when p2 became ready), not 100ms
	// Allow some tolerance for scheduling
	require.True(t, elapsed >= 5*time.Millisecond, "poll should have blocked for some time")
	require.True(t, elapsed < 80*time.Millisecond, "poll should not have waited for slowest pollable, elapsed: %v", elapsed)

	// The fast pollable (index 1) should be in the ready list
	indices := results[0].List()
	require.True(t, len(indices) >= 1, "at least one pollable should be ready")

	// Check that index 1 (p2, the fast one) is in the ready list
	foundFast := false
	for _, idx := range indices {
		if idx.U32() == 1 {
			foundFast = true
			break
		}
	}
	require.True(t, foundFast, "the fast pollable (index 1) should be in the ready list")
}

func TestPoll_MultiplePollables_AllReadyAtOnce(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Create pollables that all become ready at about the same time
	var mu sync.Mutex
	allReady := false

	makePollable := func() *Pollable {
		return NewPollableWithChannel(
			func() bool {
				mu.Lock()
				defer mu.Unlock()
				return allReady
			},
			func() {
				time.Sleep(10 * time.Millisecond)
				mu.Lock()
				allReady = true
				mu.Unlock()
			},
		)
	}

	p1 := makePollable()
	p2 := makePollable()
	p3 := makePollable()

	h1, errHandle585 := table.NewResourceHandle(p1, true, pollableResourceType)
	if errHandle585 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errHandle585)
	}
	h2, errHandle586 := table.NewResourceHandle(p2, true, pollableResourceType)
	if errHandle586 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errHandle586)
	}
	h3, errHandle587 := table.NewResourceHandle(p3, true, pollableResourceType)
	if errHandle587 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errHandle587)
	}

	args := []types.Val{
		types.ValList([]types.Val{
			types.ValBorrow(uint32(h1)),
			types.ValBorrow(uint32(h2)),
			types.ValBorrow(uint32(h3)),
		}),
	}

	results, err := pollPoll(ctx, args)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))

	// All should be ready since they share the same state
	indices := results[0].List()
	require.Equal(t, 3, len(indices), "all pollables should be ready")
}

func TestPoll_ChannelBasedPollable(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Create a pollable with a channel for signaling readiness
	readyCh := make(chan struct{})

	p := NewPollableWithChannel(
		func() bool {
			select {
			case <-readyCh:
				return true
			default:
				return false
			}
		},
		func() {
			<-readyCh // Block until channel is closed
		},
	)

	h, errHandle627 := table.NewResourceHandle(p, true, pollableResourceType)
	if errHandle627 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errHandle627)
	}

	// Signal readiness after a short delay
	go func() {
		time.Sleep(15 * time.Millisecond)
		close(readyCh)
	}()

	args := []types.Val{
		types.ValList([]types.Val{
			types.ValBorrow(uint32(h)),
		}),
	}

	start := time.Now()
	results, err := pollPoll(ctx, args)
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.True(t, elapsed >= 10*time.Millisecond, "should have waited for channel")
	require.True(t, elapsed < 100*time.Millisecond, "should not have taken too long")

	indices := results[0].List()
	require.Equal(t, 1, len(indices))
	require.Equal(t, uint32(0), indices[0].U32())
}

// Tests for Task 1.1: Channel-based ready state infrastructure

func TestPollable_IsReady_InitiallyFalse(t *testing.T) {
	p := NewChannelPollable()
	if p.IsReady() {
		t.Error("new channel pollable should not be ready initially")
	}
}

func TestPollable_SetReady(t *testing.T) {
	p := NewChannelPollable()
	p.SetReady()
	if !p.IsReady() {
		t.Error("pollable should be ready after SetReady")
	}
}

func TestPollable_ReadyChan(t *testing.T) {
	p := NewChannelPollable()

	done := make(chan struct{})
	go func() {
		<-p.ReadyChan()
		close(done)
	}()

	// Should not be done yet
	select {
	case <-done:
		t.Error("ReadyChan should block before SetReady")
	case <-time.After(50 * time.Millisecond):
		// Good
	}

	p.SetReady()

	select {
	case <-done:
		// Good
	case <-time.After(100 * time.Millisecond):
		t.Error("ReadyChan should unblock after SetReady")
	}
}

func TestPollable_Block_WithChannel(t *testing.T) {
	p := NewChannelPollable()

	done := make(chan struct{})
	go func() {
		p.Block()
		close(done)
	}()

	select {
	case <-done:
		t.Error("Block should not return before SetReady")
	case <-time.After(50 * time.Millisecond):
		// Good
	}

	p.SetReady()

	select {
	case <-done:
		// Good
	case <-time.After(100 * time.Millisecond):
		t.Error("Block should return after SetReady")
	}
}

func TestPollable_BackwardCompatibility(t *testing.T) {
	// Existing callback-based pollables should still work
	ready := false
	p := NewPollable(
		func() bool { return ready },
		func() { ready = true },
	)

	if p.Ready() {
		t.Error("should not be ready initially")
	}

	p.Block()

	if !p.Ready() {
		t.Error("should be ready after block")
	}
}

func TestPollable_SetReady_Idempotent(t *testing.T) {
	// SetReady should be safe to call multiple times
	p := NewChannelPollable()
	p.SetReady()
	p.SetReady() // Should not panic
	if !p.IsReady() {
		t.Error("pollable should still be ready after multiple SetReady calls")
	}
}

func TestPollable_OnReadyCallback(t *testing.T) {
	callbackCalled := false
	p := NewChannelPollableWithCallback(func() {
		callbackCalled = true
	})

	p.SetReady()

	if !callbackCalled {
		t.Error("onReady callback should have been called")
	}
}

func TestPollable_Ready_ChecksBothIsReadyAndReadyFn(t *testing.T) {
	// Test that Ready() checks isReady flag even when readyFn returns false
	p := NewChannelPollable()
	p.readyFn = func() bool { return false }

	if p.Ready() {
		t.Error("should not be ready initially")
	}

	p.SetReady()

	if !p.Ready() {
		t.Error("should be ready after SetReady even if readyFn returns false")
	}
}

func TestPollable_IsReady_ThreadSafe(t *testing.T) {
	p := NewChannelPollable()

	var wg sync.WaitGroup
	numGoroutines := 100

	// Multiple goroutines checking and setting ready
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			if n%2 == 0 {
				p.SetReady()
			} else {
				_ = p.IsReady()
			}
		}(i)
	}

	wg.Wait()

	// After all goroutines complete, pollable should be ready
	if !p.IsReady() {
		t.Error("pollable should be ready after SetReady was called")
	}
}
