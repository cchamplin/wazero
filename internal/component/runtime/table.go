// internal/component/runtime/table.go

package runtime

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
	// ErrMayNotLeave is returned when an operation requires may_leave but it's false.
	ErrMayNotLeave = errors.New("operation not allowed: instance may not leave")
	// ErrReentrance is returned when an operation would cause invalid reentrance.
	ErrReentrance = errors.New("operation would cause invalid recursive reentrance")
	// ErrTableFull is returned when the resource table has reached its maximum size.
	ErrTableFull = errors.New("resource table full: maximum length exceeded")
)

// MaxTableLength is the maximum number of entries in a resource table.
// From the spec, this is 2^28 - 1.
const MaxTableLength = uint32(1<<28 - 1)

// Destroyable is implemented by resources that need cleanup when deleted.
// When a resource implementing this interface is deleted from the Table
// via Delete(), its Destroy() method will be called automatically if the
// handle was owned (not borrowed).
type Destroyable interface {
	Destroy()
}

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

// TableEntry is the interface implemented by everything stored in a
// Table. The dynamic type is checked via type assertion at retrieval.
//
// Spec: definitions.py:303-315 (class Table). The unified table holds
// heterogeneous handle kinds — resources today; subtasks/streams/futures/
// error-contexts when async lands.
type TableEntry interface {
	tableEntry()
}

// ResourceHandleEntry is the Table entry type for resource handles.
// Replaces the old HandleEntry{RT ResourceTypeID, ...} struct. RT is
// now a *ResourceType with pointer identity, fixing the cross-instance
// type-index collision bug in the deleted ValidateType.
type ResourceHandleEntry struct {
	RT          *ResourceType // Resource type this handle belongs to (pointer identity)
	Rep         any           // The resource representation value
	Own         bool          // True if this is an owning handle
	NumLends    uint32        // Number of active borrows from this handle
	BorrowScope any           // The scope that created this borrow (for borrowed handles)
}

func (*ResourceHandleEntry) tableEntry() {}

type entryState uint8

const (
	entryFree entryState = iota
	entryOccupied
)

// tableSlot is a single slot in the Table's entries array. It owns the
// generation counter, the free-list link, and the type-erased entry
// payload. The payload is non-nil iff state == entryOccupied.
type tableSlot struct {
	state      entryState
	generation uint32
	entry      TableEntry
	nextFree   int32
}

// Table manages handles with generation counting. Implements the
// Component Model's handle table semantics. The unified table holds
// heterogeneous handle kinds — resource handles today, subtasks /
// streams / futures / error-contexts when async lands.
//
// Spec: definitions.py:303-315 (class Table).
type Table struct {
	entries  []tableSlot
	freeHead int32 // Head of free list, -1 if empty
}

// NewTable creates an empty Table.
func NewTable() *Table {
	return &Table{
		freeHead: -1,
	}
}

// Add inserts an entry into the table and returns its handle. This is
// the public unified-entry add path; symmetric with Get.
func (t *Table) Add(entry TableEntry) (Handle, error) {
	return t.add(entry)
}

// add is the internal append helper used by all New* paths. It reuses
// a free slot if available, otherwise grows the entries slice.
func (t *Table) add(entry TableEntry) (Handle, error) {
	var idx uint32
	var gen uint32

	if t.freeHead >= 0 {
		// Reuse a free slot
		idx = uint32(t.freeHead)
		slot := &t.entries[idx]
		t.freeHead = slot.nextFree
		gen = slot.generation + 1
		slot.state = entryOccupied
		slot.generation = gen
		slot.entry = entry
		slot.nextFree = -1
	} else {
		// Allocate new slot
		if uint32(len(t.entries)) >= MaxTableLength {
			return 0, ErrTableFull
		}
		idx = uint32(len(t.entries))
		gen = 0
		t.entries = append(t.entries, tableSlot{
			state:      entryOccupied,
			generation: gen,
			entry:      entry,
			nextFree:   -1,
		})
	}

	return MakeHandle(idx, gen), nil
}

// NewResourceHandle inserts a resource handle into the table and returns
// its index. The RT is a *ResourceType pointer for spec-correct identity
// comparisons.
func (t *Table) NewResourceHandle(rep any, own bool, rt *ResourceType) (Handle, error) {
	entry := &ResourceHandleEntry{RT: rt, Rep: rep, Own: own}
	return t.add(entry)
}

// NewWithMayLeaveCheck creates a new resource handle with may_leave validation.
// Returns ErrMayNotLeave if the instance state doesn't allow leaving.
//
// From spec (CanonicalABI.md:3604-3609):
//
//	def canon_resource_new(rt, thread, rep):
//	  trap_if(not thread.task.inst.may_leave)
//	  ...
func (t *Table) NewWithMayLeaveCheck(rep any, own bool, rt *ResourceType, inst *ComponentInstance) (Handle, error) {
	if inst != nil && !inst.IsMayLeave() {
		return 0, ErrMayNotLeave
	}
	return t.NewResourceHandle(rep, own, rt)
}

// Get retrieves the entry for a handle without removing it. The returned
// TableEntry must be type-asserted to the concrete handle kind (e.g.
// *ResourceHandleEntry) by the caller.
func (t *Table) Get(h Handle) (TableEntry, error) {
	idx := h.Index()
	if idx >= uint32(len(t.entries)) {
		return nil, ErrInvalidHandle
	}

	slot := &t.entries[idx]
	if slot.generation != h.Generation() {
		return nil, fmt.Errorf("%w: generation mismatch", ErrInvalidHandle)
	}
	if slot.state == entryFree {
		return nil, ErrInvalidHandle
	}

	return slot.entry, nil
}

// GetResourceHandle is a convenience wrapper around Get that asserts the
// stored entry is a *ResourceHandleEntry. Returns ErrInvalidHandle if
// the entry is some other handle kind.
func (t *Table) GetResourceHandle(h Handle) (*ResourceHandleEntry, error) {
	entry, err := t.Get(h)
	if err != nil {
		return nil, err
	}
	resEntry, ok := entry.(*ResourceHandleEntry)
	if !ok {
		return nil, ErrInvalidHandle
	}
	return resEntry, nil
}

// Remove removes a handle from the table and returns its entry as a
// *ResourceHandleEntry. Used for lift_own to transfer ownership out of
// the component. Traps if the handle has active borrows (NumLends > 0).
//
// For borrow handles (Own == false), the caller (resource.drop implementation)
// is responsible for decrementing the borrow count in the call context:
//
//	entry, err := table.Remove(h)
//	if err != nil { return err }
//	if !entry.Own {
//	    callCtx.DecrementBorrows()  // Caller must do this!
//	}
func (t *Table) Remove(h Handle) (*ResourceHandleEntry, error) {
	idx := h.Index()
	if idx >= uint32(len(t.entries)) {
		return nil, ErrInvalidHandle
	}

	slot := &t.entries[idx]
	if slot.generation != h.Generation() {
		return nil, fmt.Errorf("%w: generation mismatch", ErrInvalidHandle)
	}
	if slot.state == entryFree {
		return nil, ErrInvalidHandle
	}

	resEntry, ok := slot.entry.(*ResourceHandleEntry)
	if !ok {
		return nil, ErrInvalidHandle
	}
	if resEntry.NumLends > 0 {
		return nil, ErrResourceInUse
	}

	// Mark as free and add to free list
	slot.state = entryFree
	slot.entry = nil
	slot.nextFree = t.freeHead
	t.freeHead = int32(idx)

	return resEntry, nil
}

// Delete removes a handle from the table and calls Destroy() if applicable.
// This is the preferred method for dropping resources as it handles cleanup.
//
// For owned handles (entry.Own == true):
//   - Removes the handle from the table
//   - If the resource implements Destroyable, calls Destroy()
//
// For borrowed handles (entry.Own == false):
//   - Removes the handle from the table
//   - Does NOT call Destroy() (borrows don't own the resource)
//
// Returns ErrResourceInUse if the handle has active borrows (NumLends > 0).
func (t *Table) Delete(h Handle) error {
	entry, err := t.Remove(h)
	if err != nil {
		return err
	}

	// Only call Destroy for owned handles
	if entry.Own {
		if destroyable, ok := entry.Rep.(Destroyable); ok {
			destroyable.Destroy()
		}
	}

	return nil
}

// IncrementLends increments the borrow count for a handle.
// Called during lift_borrow to track active borrows.
func (t *Table) IncrementLends(h Handle) error {
	resEntry, err := t.GetResourceHandle(h)
	if err != nil {
		return err
	}
	resEntry.NumLends++
	return nil
}

// DecrementLends decrements the borrow count for a handle.
// Called when a borrow scope completes.
func (t *Table) DecrementLends(h Handle) error {
	resEntry, err := t.GetResourceHandle(h)
	if err != nil {
		return err
	}
	if resEntry.NumLends == 0 {
		return ErrNoBorrowsToDecrement
	}
	resEntry.NumLends--
	return nil
}

// Rep returns the representation value for a handle.
// This is the underlying uint32 value that identifies the resource.
// Returns an error if the handle is invalid.
func (t *Table) Rep(h Handle) (uint32, error) {
	entry, err := t.GetResourceHandle(h)
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

// CreateResourceNewFunc creates a core function for resource.new that
// can be called from core modules to create new resource handles of
// the given nominal *ResourceType. The returned function accepts a rep
// (representation) value and returns a handle index that can be used to
// access the resource.
func (t *Table) CreateResourceNewFunc(rt *ResourceType) func(rep uint32) uint32 {
	return func(rep uint32) uint32 {
		handle, err := t.NewResourceHandle(rep, true, rt) // own=true for newly created resources
		if err != nil {
			return 0
		}
		return uint32(handle)
	}
}

// CreateResourceDropFunc creates a core function for resource.drop
// that can be called from core modules to drop resource handles.
// The destructor is called when the resource is dropped (if provided).
func (t *Table) CreateResourceDropFunc(rt *ResourceType, destructor func(rep uint32)) func(handle uint32) {
	return func(handle uint32) {
		h := Handle(handle)
		// Validate type before removal (spec: trap_if(h.rt is not rt))
		if rt != nil {
			if err := t.ValidateType(h, rt); err != nil {
				return // Silently ignore invalid handles per spec
			}
		}
		entry, err := t.Remove(h)
		if err != nil {
			return // Silently ignore invalid handles per spec
		}
		// Per spec: borrows do not own the resource, so no destructor call
		if destructor != nil && entry.Own && entry.Rep != nil {
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
func (t *Table) CreateResourceRepFunc(rt *ResourceType) func(handle uint32) uint32 {
	return func(handle uint32) uint32 {
		h := Handle(handle)
		if rt != nil {
			if err := t.ValidateType(h, rt); err != nil {
				return 0
			}
		}
		rep, err := t.Rep(h)
		if err != nil {
			return 0 // Return 0 for invalid handles
		}
		return rep
	}
}

// CreateResourceRepFuncWithTrap creates a core function for resource.rep
// that calls the trap handler on errors instead of returning 0.
// This is the spec-compliant version that properly validates types.
func (t *Table) CreateResourceRepFuncWithTrap(rt *ResourceType, trap TrapHandler) func(handle uint32) uint32 {
	return func(handle uint32) uint32 {
		h := Handle(handle)

		// Validate type (spec: trap_if(h.rt is not rt))
		if rt != nil {
			if err := t.ValidateType(h, rt); err != nil {
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

// GetResourceType returns the nominal type of the resource handle h.
// Returns an error if h is not a resource handle.
func (t *Table) GetResourceType(h Handle) (*ResourceType, error) {
	entry, err := t.Get(h)
	if err != nil {
		return nil, err
	}
	resEntry, ok := entry.(*ResourceHandleEntry)
	if !ok {
		return nil, ErrInvalidHandle
	}
	return resEntry.RT, nil
}

// ValidateType verifies that the handle h refers to a resource entry
// whose runtime type is the same nominal type as expected. Comparison
// is POINTER equality on *ResourceType — the spec's `is` check at
// definitions.py:1345.
//
// Bug fix: the old ValidateType compared only ResourceTypeID, ignoring
// instance identity. This silently accepted cross-instance handles
// when both happened to share a type-section index.
func (t *Table) ValidateType(h Handle, expected *ResourceType) error {
	entry, err := t.Get(h)
	if err != nil {
		return err
	}
	resEntry, ok := entry.(*ResourceHandleEntry)
	if !ok {
		return ErrInvalidHandle
	}
	if resEntry.RT != expected {
		return fmt.Errorf("%w: wrong resource type", ErrResourceTypeMismatch)
	}
	return nil
}

// GetWithType retrieves an entry after validating its type.
// Returns ErrResourceTypeMismatch if types don't match.
func (t *Table) GetWithType(h Handle, expectedRT *ResourceType) (*ResourceHandleEntry, error) {
	resEntry, err := t.GetResourceHandle(h)
	if err != nil {
		return nil, err
	}

	if expectedRT != nil && resEntry.RT != expectedRT {
		return nil, fmt.Errorf("%w: wrong resource type", ErrResourceTypeMismatch)
	}

	return resEntry, nil
}

// RepWithType returns the representation value after validating the handle's type.
// Returns ErrResourceTypeMismatch if types don't match.
func (t *Table) RepWithType(h Handle, expectedRT *ResourceType) (uint32, error) {
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
func (t *Table) RemoveWithType(h Handle, expectedRT *ResourceType) (*ResourceHandleEntry, error) {
	// Validate type first (before removal)
	if expectedRT != nil {
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
func (t *Table) CreateResourceDropFuncWithTrap(rt *ResourceType, destructor func(rep uint32), trap TrapHandler) func(handle uint32) {
	return func(handle uint32) {
		h := Handle(handle)

		// Validate type before removal (spec: trap_if(h.rt is not rt))
		if rt != nil {
			if err := t.ValidateType(h, rt); err != nil {
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

// CreateResourceDropFuncWithContext creates a fully-featured resource.drop function
// that handles destructor invocation, borrow count tracking, and type validation.
//
// This is the spec-compliant implementation that:
//   - Validates the handle type matches rt (pointer identity)
//   - For owned handles: uses DropOwned for destructor handling
//   - For borrowed handles: removes and decrements borrow count in callCtx
//
// Parameters:
//   - rt: the nominal *ResourceType to validate the handle against
//   - dtorRegistry: registry to look up same-instance destructors
//   - currentInstanceID: the instance performing the drop
//   - definingInstanceID: the instance that defined the resource type
//   - callCtx: the call context for borrow tracking (may be nil if no borrow tracking needed)
//   - crossInstanceDtor: callback for cross-instance destructor invocation
//   - trap: handler called when an error occurs
func (t *Table) CreateResourceDropFuncWithContext(
	rt *ResourceType,
	dtorRegistry *DestructorRegistry,
	currentInstanceID uint32,
	definingInstanceID uint32,
	callCtx *CallContext,
	crossInstanceDtor CrossInstanceDestructor,
	trap TrapHandler,
) func(handle uint32) {
	return func(handle uint32) {
		h := Handle(handle)

		// Get entry first to check if it's a borrow
		entry, err := t.GetWithType(h, rt)
		if err != nil {
			trap(err)
			return
		}

		if entry.Own {
			// Owned handle: use DropOwned for destructor handling
			err = t.DropOwned(h, rt, dtorRegistry, currentInstanceID, definingInstanceID, crossInstanceDtor)
			if err != nil {
				trap(err)
			}
		} else {
			// Borrowed handle: just remove and decrement borrow count
			_, err = t.Remove(h)
			if err != nil {
				trap(err)
				return
			}
			// Spec: h.borrow_scope.num_borrows -= 1
			if callCtx != nil {
				callCtx.DecrementBorrows()
			}
		}
	}
}

// DropOwned drops an owned handle, invoking the destructor if appropriate.
// This implements the spec's resource.drop for owned handles.
//
// Parameters:
//   - h: the handle to drop
//   - expectedRT: the expected nominal *ResourceType (for validation)
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
func (t *Table) DropOwned(
	h Handle,
	expectedRT *ResourceType,
	dtorRegistry *DestructorRegistry,
	currentInstanceID uint32,
	definingInstanceID uint32,
	crossInstanceDtor CrossInstanceDestructor,
) error {
	// Validate type first
	if expectedRT != nil {
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

// DropOwnedWithReentranceCheck drops an owned handle with full reentrance checking.
// This implements the complete spec behavior for resource.drop.
//
// From spec (CanonicalABI.md:3664-3667):
//
//	else:
//	  trap_if(call_might_be_recursive(thread.task, rt.impl))
//
// The reentrance check only applies when there's no destructor.
func (t *Table) DropOwnedWithReentranceCheck(
	h Handle,
	expectedRT *ResourceType,
	dtorRegistry *DestructorRegistry,
	currentInstanceID uint32,
	definingInstanceID uint32,
	crossInstanceDtor CrossInstanceDestructor,
	tracker *ReentranceTracker,
) error {
	// Validate type first
	if expectedRT != nil {
		if err := t.ValidateType(h, expectedRT); err != nil {
			return err
		}
	}

	// Check if this is cross-instance without destructor
	if currentInstanceID != definingInstanceID {
		hasDestructor := dtorRegistry != nil && dtorRegistry.Has(expectedRT)
		if !hasDestructor && tracker != nil && tracker.CallMightBeRecursive(definingInstanceID) {
			return ErrReentrance
		}
	}

	// Proceed with normal DropOwned
	return t.DropOwned(h, expectedRT, dtorRegistry, currentInstanceID, definingInstanceID, crossInstanceDtor)
}
