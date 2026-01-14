// imports/wasip2/io/poll.go

package io

import (
	"context"

	"github.com/tetratelabs/wazero/internal/component"
)

// Pollable represents something that can be polled for readiness.
// Per wasi:io/poll@0.2.0 spec.
type Pollable struct {
	// readyFn returns true if the pollable is ready
	readyFn func() bool
	// blockFn blocks until ready (may be nil for immediately ready)
	blockFn func()
}

// NewPollable creates a pollable with ready and block functions.
func NewPollable(readyFn func() bool, blockFn func()) *Pollable {
	return &Pollable{readyFn: readyFn, blockFn: blockFn}
}

// NewReadyPollable creates a pollable that is immediately ready.
func NewReadyPollable() *Pollable {
	return &Pollable{readyFn: func() bool { return true }}
}

// Ready returns true if the pollable is ready.
func (p *Pollable) Ready() bool {
	if p.readyFn == nil {
		return true
	}
	return p.readyFn()
}

// Block waits until the pollable becomes ready.
func (p *Pollable) Block() {
	if p.blockFn != nil {
		p.blockFn()
	}
}

func instantiatePoll(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:io/poll@0.2.0")

	// Define pollable resource
	inst.Resource("pollable", func(rep uint32) {
		// Destructor - nothing to clean up
	})

	// [method]pollable.ready: func() -> bool
	inst.FuncNoType("[method]pollable.ready", pollableReady)

	// [method]pollable.block: func()
	inst.FuncNoType("[method]pollable.block", pollableBlock)

	// poll: func(in: list<borrow<pollable>>) -> list<u32>
	inst.FuncNoType("poll", pollPoll)

	return inst.Build()
}

func pollableReady(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// args[0] is borrow<pollable> - the handle
	// For now return true (ready) as placeholder
	// Full implementation will look up handle and call Ready()
	return []component.Val{component.ValBool(true)}, nil
}

func pollableBlock(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// args[0] is borrow<pollable> - the handle
	// For now do nothing as placeholder
	// Full implementation will look up handle and call Block()
	return nil, nil
}

func pollPoll(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// args[0] is list<borrow<pollable>>
	// For now return all indices as ready (placeholder)
	// Real implementation would check each pollable

	pollables := args[0].List()
	result := make([]component.Val, len(pollables))
	for i := range pollables {
		result[i] = component.ValU32(uint32(i))
	}
	return []component.Val{component.ValList(result)}, nil
}
