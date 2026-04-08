// internal/component/runtime/reentrance.go

package runtime

// ReentranceTracker tracks which component instances are currently on the call stack.
// This is used to implement the call_might_be_recursive check from the spec.
//
// From spec (CanonicalABI.md:3664-3667):
//
//	else:
//	  trap_if(call_might_be_recursive(thread.task, rt.impl))
type ReentranceTracker struct {
	// Maps instance ID to the number of times it's currently on the call stack
	activeCalls map[uint32]int
}

// NewReentranceTracker creates a new reentrance tracker.
func NewReentranceTracker() *ReentranceTracker {
	return &ReentranceTracker{
		activeCalls: make(map[uint32]int),
	}
}

// EnterInstance records that we're entering a call to the given instance.
func (r *ReentranceTracker) EnterInstance(instanceID uint32) {
	r.activeCalls[instanceID]++
}

// LeaveInstance records that we're leaving a call to the given instance.
func (r *ReentranceTracker) LeaveInstance(instanceID uint32) {
	if count := r.activeCalls[instanceID]; count > 0 {
		r.activeCalls[instanceID] = count - 1
		if r.activeCalls[instanceID] == 0 {
			delete(r.activeCalls, instanceID)
		}
	}
}

// CallMightBeRecursive reports whether calling the given instance would be
// recursive given the currently-active instance stack. An instance is
// considered active between EnterInstance(id) and LeaveInstance(id). A call
// is recursive if the callee is already on the active stack.
//
// Spec: definitions.py:290-299 call_might_be_recursive(caller, callee_inst)
// uses reflexive_ancestors() overlap between caller and callee; wazero's
// tracker models this by maintaining the per-instance active set on the
// shared tracker used by all instances on a call stack. Populating the
// tracker with every ancestor instance on Enter/Leave (to be wired by
// the runtime delegator in component_instance.go in a later Session 1
// task) makes a plain membership query equivalent to the spec's
// reflexive-ancestor overlap check.
func (r *ReentranceTracker) CallMightBeRecursive(instanceID uint32) bool {
	if r == nil {
		return false
	}
	return r.isActive(instanceID)
}

// isActive reports whether the given instance ID is currently on the
// active call stack (i.e. has more EnterInstance calls than LeaveInstance
// calls outstanding). Internal helper for CallMightBeRecursive.
func (r *ReentranceTracker) isActive(instanceID uint32) bool {
	return r.activeCalls[instanceID] > 0
}

// ActiveInstances returns a copy of the active instance IDs (for debugging).
func (r *ReentranceTracker) ActiveInstances() []uint32 {
	result := make([]uint32, 0, len(r.activeCalls))
	for id := range r.activeCalls {
		result = append(result, id)
	}
	return result
}
