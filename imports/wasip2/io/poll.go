// imports/wasip2/io/poll.go

package io

import (
	"context"
	"fmt"
	"sync"

	"github.com/tetratelabs/wazero/internal/component"
)

// Pollable represents something that can be polled for readiness.
// Per wasi:io/poll@0.2.0 spec.
type Pollable struct {
	// readyFn returns true if the pollable is ready (callback-based approach)
	readyFn func() bool
	// blockFn blocks until ready (may be nil for immediately ready)
	blockFn func()

	// Channel-based ready state infrastructure
	ready   chan struct{} // Closed when ready (for channel-based pollables)
	isReady bool          // Cached ready state
	mu      sync.Mutex    // Protects isReady
	onReady func()        // Optional callback when becoming ready
}

// NewPollable creates a pollable with ready and block functions.
func NewPollable(readyFn func() bool, blockFn func()) *Pollable {
	return &Pollable{readyFn: readyFn, blockFn: blockFn}
}

// NewReadyPollable creates a pollable that is immediately ready.
func NewReadyPollable() *Pollable {
	return &Pollable{readyFn: func() bool { return true }}
}

// NewPollableWithChannel creates a pollable that supports channel-based multiplexing.
// The readyFn should check if the pollable is ready (non-blocking).
// The blockFn should block until the pollable becomes ready.
// This is an alias for NewPollable, kept for API clarity in tests.
func NewPollableWithChannel(readyFn func() bool, blockFn func()) *Pollable {
	return &Pollable{readyFn: readyFn, blockFn: blockFn}
}

// NewChannelPollable creates a pollable with channel-based ready state tracking.
// Use SetReady() to mark the pollable as ready, which closes the internal channel
// and unblocks any goroutines waiting on ReadyChan() or Block().
func NewChannelPollable() *Pollable {
	return &Pollable{
		ready: make(chan struct{}),
	}
}

// Ready returns true if the pollable is ready.
// Checks both the isReady flag (for channel-based pollables) and readyFn (for callback-based).
func (p *Pollable) Ready() bool {
	// Check channel-based ready state first
	if p.IsReady() {
		return true
	}
	// Fall back to callback-based check
	if p.readyFn == nil {
		return true
	}
	return p.readyFn()
}

// Block waits until the pollable becomes ready.
// Supports both callback-based (blockFn) and channel-based (ready channel) approaches.
func (p *Pollable) Block() {
	// If callback-based blocking is available, use it
	if p.blockFn != nil {
		p.blockFn()
		return
	}
	// Otherwise, use channel-based blocking if available
	if p.ready != nil {
		<-p.ready
	}
}

// IsReady returns true if the pollable has been marked ready via SetReady.
// Thread-safe check of the isReady flag.
func (p *Pollable) IsReady() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.isReady
}

// SetReady marks the pollable as ready, closes the ready channel (if present),
// and calls the onReady callback (if set). This method is idempotent - calling
// it multiple times is safe and only the first call has effect.
func (p *Pollable) SetReady() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isReady {
		return // Already ready, nothing to do
	}

	p.isReady = true

	// Close the channel to unblock any waiters
	if p.ready != nil {
		close(p.ready)
	}

	// Call the onReady callback if set
	if p.onReady != nil {
		p.onReady()
	}
}

// ReadyChan returns a channel that will be closed when the pollable becomes ready.
// This allows using select statements to wait on multiple pollables.
// Returns nil if the pollable was not created with NewChannelPollable.
func (p *Pollable) ReadyChan() <-chan struct{} {
	return p.ready
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

	// First pass: check which pollables are already ready
	var readyIndices []component.Val
	for i, p := range pollables {
		if p.Ready() {
			readyIndices = append(readyIndices, component.ValU32(uint32(i)))
		}
	}

	// If at least one is ready, return the ready indices immediately
	if len(readyIndices) > 0 {
		return []component.Val{component.ValList(readyIndices)}, nil
	}

	// None are ready - use goroutines and channels for true multiplexing
	// Per WASI spec: "If none of the pollables are ready, the function blocks
	// until at least one pollable becomes ready."

	// Check if any pollable has a block function
	hasBlockFn := false
	for _, p := range pollables {
		if p.blockFn != nil {
			hasBlockFn = true
			break
		}
	}

	// If no pollable has a block function but none are ready,
	// return an empty list (this shouldn't happen in correct usage).
	if !hasBlockFn {
		return []component.Val{component.ValList(nil)}, nil
	}

	// Use channel-based multiplexing: spawn a goroutine for each pollable
	// that has a block function, and wait for any one to signal readiness.
	readyCh := make(chan int, len(pollables))

	for i, p := range pollables {
		if p.blockFn != nil {
			go func(idx int, pollable *Pollable) {
				pollable.Block()
				// Signal that this pollable is now ready
				// Use non-blocking send in case we've already returned
				select {
				case readyCh <- idx:
				default:
				}
			}(i, p)
		}
	}

	// Wait for the first pollable to become ready
	<-readyCh

	// Now check all pollables again to collect all that are ready
	// (there may be multiple ready by now)
	readyIndices = nil
	for i, p := range pollables {
		if p.Ready() {
			readyIndices = append(readyIndices, component.ValU32(uint32(i)))
		}
	}

	// At least the one that signaled should be ready
	if len(readyIndices) == 0 {
		// Fallback: if somehow nothing is ready, return the one that signaled
		// This shouldn't happen in correct usage
		readyIndices = append(readyIndices, component.ValU32(0))
	}

	return []component.Val{component.ValList(readyIndices)}, nil
}
