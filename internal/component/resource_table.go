// internal/component/resource_table.go

package component

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidHandle        = errors.New("invalid resource handle")
	ErrHandleNotOwned       = errors.New("handle is not owned")
	ErrResourceInUse        = errors.New("resource has active borrows")
	ErrNoBorrowsToDecrement = errors.New("no active borrows to decrement")
	ErrResourceTypeMismatch = errors.New("resource type mismatch")
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
//
// For borrow handles (Own == false), the caller (resource.drop implementation)
// is responsible for decrementing the borrow count in the call context:
//
//	entry, err := table.Remove(h)
//	if err != nil { return err }
//	if !entry.Own {
//	    callCtx.DecrementBorrows()  // Caller must do this!
//	}
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

// CreateResourceNewFuncWithType creates a core function for resource.new
// that stores the resource type ID with each created handle.
// The resourceTypeIdx is the index from the component's type section.
func (t *ResourceTable) CreateResourceNewFuncWithType(resourceTypeIdx uint32) func(rep uint32) uint32 {
	rtID := NewResourceTypeID(resourceTypeIdx)
	return func(rep uint32) uint32 {
		handle := t.NewWithType(rep, true, rtID)
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

// CreateResourceRepFuncWithTrap creates a core function for resource.rep
// that calls the trap handler on errors instead of returning 0.
// This is the spec-compliant version that properly validates types.
func (t *ResourceTable) CreateResourceRepFuncWithTrap(resourceTypeIdx uint32, trap TrapHandler) func(handle uint32) uint32 {
	expectedRT := NewResourceTypeID(resourceTypeIdx)
	return func(handle uint32) uint32 {
		h := Handle(handle)

		// Validate type (spec: trap_if(h.rt is not rt))
		if expectedRT.IsValid() {
			if err := t.ValidateType(h, expectedRT); err != nil {
				trap(err)
				return 0
			}
		}

		rep, err := t.Rep(h)
		if err != nil {
			// Spec: trap_if(not isinstance(h, ResourceHandle))
			trap(err)
			return 0
		}
		return rep
	}
}

// GetType returns the ResourceTypeID for a handle.
func (t *ResourceTable) GetType(h Handle) (ResourceTypeID, error) {
	entry, err := t.Get(h)
	if err != nil {
		return InvalidResourceTypeID(), err
	}
	return entry.RT, nil
}

// ValidateType checks that a handle has the expected resource type.
// Returns ErrResourceTypeMismatch if types don't match.
func (t *ResourceTable) ValidateType(h Handle, expected ResourceTypeID) error {
	actual, err := t.GetType(h)
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("%w: expected type %d, got %d", ErrResourceTypeMismatch, expected.Index(), actual.Index())
	}
	return nil
}

// GetWithType retrieves an entry after validating its type.
// Returns ErrResourceTypeMismatch if types don't match.
func (t *ResourceTable) GetWithType(h Handle, expectedRT ResourceTypeID) (*HandleEntry, error) {
	entry, err := t.Get(h)
	if err != nil {
		return nil, err
	}

	if expectedRT.IsValid() && entry.RT != expectedRT {
		return nil, fmt.Errorf("%w: expected type %d, got %d", ErrResourceTypeMismatch, expectedRT.Index(), entry.RT.Index())
	}

	return entry, nil
}

// RepWithType returns the representation value after validating the handle's type.
// Returns ErrResourceTypeMismatch if types don't match.
func (t *ResourceTable) RepWithType(h Handle, expectedRT ResourceTypeID) (uint32, error) {
	entry, err := t.GetWithType(h, expectedRT)
	if err != nil {
		return 0, err
	}

	switch v := entry.Rep.(type) {
	case uint32:
		return v, nil
	case int:
		return uint32(v), nil
	default:
		return 0, fmt.Errorf("resource rep is not a uint32: %T", entry.Rep)
	}
}

// RemoveWithType removes a handle from the table after validating its type.
// Returns ErrResourceTypeMismatch if types don't match.
// The handle is NOT removed if type validation fails.
func (t *ResourceTable) RemoveWithType(h Handle, expectedRT ResourceTypeID) (*HandleEntry, error) {
	// Validate type first (before removal)
	if expectedRT.IsValid() {
		if err := t.ValidateType(h, expectedRT); err != nil {
			return nil, err
		}
	}

	return t.Remove(h)
}

// TrapHandler is a function called when a resource operation should trap.
// In production, this typically panics or records the error for the runtime.
type TrapHandler func(err error)

// CreateResourceDropFuncWithTrap creates a core function for resource.drop
// that calls the trap handler on errors instead of silently ignoring them.
// This is the spec-compliant version that properly validates types.
func (t *ResourceTable) CreateResourceDropFuncWithTrap(resourceTypeIdx uint32, destructor func(rep uint32), trap TrapHandler) func(handle uint32) {
	expectedRT := NewResourceTypeID(resourceTypeIdx)
	return func(handle uint32) {
		h := Handle(handle)

		// Validate type before removal (spec: trap_if(h.rt is not rt))
		if expectedRT.IsValid() {
			if err := t.ValidateType(h, expectedRT); err != nil {
				trap(err)
				return
			}
		}

		entry, err := t.Remove(h)
		if err != nil {
			// Spec: trap_if(not isinstance(h, ResourceHandle))
			trap(err)
			return
		}

		// Call destructor for owned resources only
		// Per spec: borrows do not own the resource, so no destructor call
		if destructor != nil && entry.Own && entry.Rep != nil {
			switch rep := entry.Rep.(type) {
			case uint32:
				destructor(rep)
			case int:
				destructor(uint32(rep))
			}
		}
	}
}

// CrossInstanceDestructor is called when dropping a resource from an instance
// different from where the resource type was defined.
// The caller is responsible for routing this to the proper canonical ABI path.
type CrossInstanceDestructor func(rep uint32, definingInstanceID uint32)

// DropOwned drops an owned handle, invoking the destructor if appropriate.
// This implements the spec's resource.drop for owned handles.
//
// Parameters:
//   - h: the handle to drop
//   - expectedRT: the expected resource type (for validation)
//   - dtorRegistry: registry to look up same-instance destructors
//   - currentInstanceID: the instance performing the drop
//   - definingInstanceID: the instance that defined the resource type
//   - crossInstanceDtor: callback for cross-instance destructor invocation
//
// From spec (CanonicalABI.md:3634-3646):
//
//	if h.own:
//	  if inst is rt.impl:
//	    if rt.dtor: rt.dtor(h.rep)
//	  else:
//	    if rt.dtor: [route through canon_lift/canon_lower]
func (t *ResourceTable) DropOwned(
	h Handle,
	expectedRT ResourceTypeID,
	dtorRegistry *DestructorRegistry,
	currentInstanceID uint32,
	definingInstanceID uint32,
	crossInstanceDtor CrossInstanceDestructor,
) error {
	// Validate type first
	if expectedRT.IsValid() {
		if err := t.ValidateType(h, expectedRT); err != nil {
			return err
		}
	}

	// Remove the handle
	entry, err := t.Remove(h)
	if err != nil {
		return err
	}

	// Only call destructor for owned handles
	if !entry.Own {
		return nil
	}

	// Get the rep value
	var rep uint32
	switch v := entry.Rep.(type) {
	case uint32:
		rep = v
	case int:
		rep = uint32(v)
	default:
		// No valid rep, skip destructor
		return nil
	}

	// Same-instance vs cross-instance destructor call
	if currentInstanceID == definingInstanceID {
		// Same instance: call destructor directly
		if dtor := dtorRegistry.Get(expectedRT); dtor != nil {
			dtor(rep)
		}
	} else {
		// Cross-instance: use the cross-instance callback
		if crossInstanceDtor != nil {
			crossInstanceDtor(rep, definingInstanceID)
		}
	}

	return nil
}
