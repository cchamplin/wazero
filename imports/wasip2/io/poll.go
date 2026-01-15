// imports/wasip2/io/poll.go

package io

import (
	"context"
	"fmt"

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

// getPollable retrieves a Pollable from the ResourceTable using a borrow handle.
func getPollable(ctx context.Context, handle uint32) (*Pollable, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return nil, fmt.Errorf("no resource table in context")
	}
	entry, err := table.Get(component.Handle(handle))
	if err != nil {
		return nil, fmt.Errorf("invalid handle %d: %w", handle, err)
	}
	pollable, ok := entry.Rep.(*Pollable)
	if !ok {
		return nil, fmt.Errorf("handle %d is not a Pollable", handle)
	}
	return pollable, nil
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
	handle := args[0].Borrow()

	pollable, err := getPollable(ctx, handle)
	if err != nil {
		return nil, err
	}

	return []component.Val{component.ValBool(pollable.Ready())}, nil
}

func pollableBlock(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()

	pollable, err := getPollable(ctx, handle)
	if err != nil {
		return nil, err
	}

	pollable.Block()
	return nil, nil
}

func pollPoll(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// args[0] is list<borrow<pollable>>
	handles := args[0].List()

	if len(handles) == 0 {
		// Empty list - return empty result
		return []component.Val{component.ValList(nil)}, nil
	}

	// Resolve all pollables first
	pollables := make([]*Pollable, len(handles))
	for i, h := range handles {
		handle := h.Borrow()
		pollable, err := getPollable(ctx, handle)
		if err != nil {
			return nil, err
		}
		pollables[i] = pollable
	}

	// Check which pollables are ready
	for {
		var readyIndices []component.Val
		for i, p := range pollables {
			if p.Ready() {
				readyIndices = append(readyIndices, component.ValU32(uint32(i)))
			}
		}

		// If at least one is ready, return the ready indices
		if len(readyIndices) > 0 {
			return []component.Val{component.ValList(readyIndices)}, nil
		}

		// None are ready - block on the first pollable that has a block function
		// Per WASI spec: "If none of the pollables are ready, the function blocks
		// until at least one pollable becomes ready."
		//
		// In a real implementation, we'd use a more sophisticated approach
		// (select on channels, etc.) but for now we block on each pollable in turn
		blocked := false
		for _, p := range pollables {
			if p.blockFn != nil {
				p.Block()
				blocked = true
				break
			}
		}

		// If no pollable has a block function but none are ready,
		// we'd spin-wait. For safety, if we can't block and none are ready,
		// just return an empty list (this shouldn't happen in correct usage).
		if !blocked {
			return []component.Val{component.ValList(nil)}, nil
		}
	}
}
