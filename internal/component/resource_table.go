// internal/component/resource_table.go

package component

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidHandle       = errors.New("invalid resource handle")
	ErrHandleNotOwned      = errors.New("handle is not owned")
	ErrResourceInUse       = errors.New("resource has active borrows")
	ErrNoBorrowsToDecrement = errors.New("no active borrows to decrement")
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
	RT          ResourceTypeID // Resource type this handle belongs to
	Rep         any            // The resource representation value
	Own         bool           // True if this is an owning handle
	NumLends    uint32         // Number of active borrows from this handle
	BorrowScope any            // The scope that created this borrow (for borrowed handles)
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
// Note: This creates a handle with an invalid ResourceTypeID.
// Use NewWithType for type-tracked handles.
func (t *ResourceTable) New(rep any, own bool) Handle {
	return t.NewWithType(rep, own, InvalidResourceTypeID())
}

// NewWithType creates a new resource handle with a specific resource type.
func (t *ResourceTable) NewWithType(rep any, own bool, rtID ResourceTypeID) Handle {
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
		entry.entry = HandleEntry{RT: rtID, Rep: rep, Own: own}
		entry.nextFree = -1
	} else {
		// Allocate new slot
		idx = uint32(len(t.entries))
		gen = 0
		t.entries = append(t.entries, tableEntry{
			state:      entryOccupied,
			generation: gen,
			entry:      HandleEntry{RT: rtID, Rep: rep, Own: own},
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

// Remove removes a handle from the table and returns its entry.
// Used for lift_own to transfer ownership out of the component.
// Traps if the handle has active borrows (NumLends > 0).
func (t *ResourceTable) Remove(h Handle) (*HandleEntry, error) {
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
	if entry.entry.NumLends > 0 {
		return nil, ErrResourceInUse
	}

	// Copy the entry before clearing
	result := entry.entry

	// Mark as free and add to free list
	entry.state = entryFree
	entry.entry = HandleEntry{}
	entry.nextFree = t.freeHead
	t.freeHead = int32(idx)

	return &result, nil
}

// IncrementLends increments the borrow count for a handle.
// Called during lift_borrow to track active borrows.
func (t *ResourceTable) IncrementLends(h Handle) error {
	idx := h.Index()
	if idx >= uint32(len(t.entries)) {
		return ErrInvalidHandle
	}

	entry := &t.entries[idx]
	if entry.generation != h.Generation() {
		return fmt.Errorf("%w: generation mismatch", ErrInvalidHandle)
	}
	if entry.state == entryFree {
		return ErrInvalidHandle
	}

	entry.entry.NumLends++
	return nil
}

// DecrementLends decrements the borrow count for a handle.
// Called when a borrow scope completes.
func (t *ResourceTable) DecrementLends(h Handle) error {
	idx := h.Index()
	if idx >= uint32(len(t.entries)) {
		return ErrInvalidHandle
	}

	entry := &t.entries[idx]
	if entry.generation != h.Generation() {
		return fmt.Errorf("%w: generation mismatch", ErrInvalidHandle)
	}
	if entry.state == entryFree {
		return ErrInvalidHandle
	}
	if entry.entry.NumLends == 0 {
		return ErrNoBorrowsToDecrement
	}

	entry.entry.NumLends--
	return nil
}

// Rep returns the representation value for a handle.
// This is the underlying uint32 value that identifies the resource.
// Returns an error if the handle is invalid.
func (t *ResourceTable) Rep(h Handle) (uint32, error) {
	entry, err := t.Get(h)
	if err != nil {
		return 0, err
	}
	// The rep is stored as any, but for canonical ABI purposes
	// it should be a uint32
	switch v := entry.Rep.(type) {
	case uint32:
		return v, nil
	case int:
		return uint32(v), nil
	default:
		return 0, fmt.Errorf("resource rep is not a uint32: %T", entry.Rep)
	}
}

// CreateResourceNewFunc creates a core function for resource.new
// that can be called from core modules to create new resource handles.
// The returned function accepts a rep (representation) value and returns
// a handle index that can be used to access the resource.
func (t *ResourceTable) CreateResourceNewFunc(resourceTypeIdx uint32) func(rep uint32) uint32 {
	return func(rep uint32) uint32 {
		handle := t.New(rep, true) // own=true for newly created resources
		return uint32(handle)
	}
}

// CreateResourceDropFunc creates a core function for resource.drop
// that can be called from core modules to drop resource handles.
// The destructor is called when the resource is dropped (if provided).
func (t *ResourceTable) CreateResourceDropFunc(resourceTypeIdx uint32, destructor func(rep uint32)) func(handle uint32) {
	return func(handle uint32) {
		entry, err := t.Remove(Handle(handle))
		if err != nil {
			return // Silently ignore invalid handles per spec
		}
		if destructor != nil && entry.Rep != nil {
			// Convert rep to uint32 and call destructor
			switch rep := entry.Rep.(type) {
			case uint32:
				destructor(rep)
			case int:
				destructor(uint32(rep))
			}
		}
	}
}

// CreateResourceRepFunc creates a core function for resource.rep
// that can be called from core modules to get the representation
// value of a resource handle.
func (t *ResourceTable) CreateResourceRepFunc(resourceTypeIdx uint32) func(handle uint32) uint32 {
	return func(handle uint32) uint32 {
		rep, err := t.Rep(Handle(handle))
		if err != nil {
			return 0 // Return 0 for invalid handles
		}
		return rep
	}
}
