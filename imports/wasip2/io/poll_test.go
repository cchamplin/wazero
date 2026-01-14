// imports/wasip2/io/poll_test.go

package io

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
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
