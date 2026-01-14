// internal/component/resource_table.go

package component

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidHandle  = errors.New("invalid resource handle")
	ErrHandleNotOwned = errors.New("handle is not owned")
	ErrResourceInUse  = errors.New("resource has active borrows")
)

// Handle is a 64-bit resource handle: upper 32 bits = generation, lower 32 = index.
// Generation counting prevents use-after-free when slots are reused.
type Handle uint64

// Index returns the table index portion of the handle.
func (h Handle) Index() uint32 { return uint32(h) }

// Generation returns the generation counter portion of the handle.
func (h Handle) Generation() uint32 { return uint32(h >> 32) }

// MakeHandle constructs a handle from an index and generation.
func MakeHandle(idx, gen uint32) Handle {
	return Handle(uint64(gen)<<32 | uint64(idx))
}

// HandleEntry represents an active resource in the table.
type HandleEntry struct {
	Rep         any    // The resource representation value
	Own         bool   // True if this is an owning handle
	NumLends    uint32 // Number of active borrows from this handle
	BorrowScope any    // The scope that created this borrow (for borrowed handles)
}

type entryState uint8

const (
	entryFree entryState = iota
	entryOccupied
)

type tableEntry struct {
	state      entryState
	generation uint32
	entry      HandleEntry
	nextFree   int32
}

// ResourceTable manages resource handles with generation counting.
// Implements the Component Model's handle table semantics.
type ResourceTable struct {
	entries  []tableEntry
	freeHead int32 // Head of free list, -1 if empty
}

// NewResourceTable creates an empty resource table.
func NewResourceTable() *ResourceTable {
	return &ResourceTable{
		freeHead: -1,
	}
}

// New creates a new resource handle and returns it.
func (t *ResourceTable) New(rep any, own bool) Handle {
	var idx uint32
	var gen uint32

	if t.freeHead >= 0 {
		// Reuse a free slot
		idx = uint32(t.freeHead)
		entry := &t.entries[idx]
		t.freeHead = entry.nextFree
		gen = entry.generation + 1
		entry.state = entryOccupied
		entry.generation = gen
		entry.entry = HandleEntry{Rep: rep, Own: own}
		entry.nextFree = -1
	} else {
		// Allocate new slot
		idx = uint32(len(t.entries))
		gen = 0
		t.entries = append(t.entries, tableEntry{
			state:      entryOccupied,
			generation: gen,
			entry:      HandleEntry{Rep: rep, Own: own},
			nextFree:   -1,
		})
	}

	return MakeHandle(idx, gen)
}

// Get retrieves the entry for a handle without removing it.
func (t *ResourceTable) Get(h Handle) (*HandleEntry, error) {
	idx := h.Index()
	if idx >= uint32(len(t.entries)) {
		return nil, ErrInvalidHandle
	}

	entry := &t.entries[idx]
	if entry.generation != h.Generation() {
		return nil, fmt.Errorf("%w: generation mismatch", ErrInvalidHandle)
	}
	if entry.state == entryFree {
		return nil, ErrInvalidHandle
	}

	return &entry.entry, nil
}
