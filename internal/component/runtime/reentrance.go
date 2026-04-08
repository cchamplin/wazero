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

// CallMightBeRecursive reports whether the given instance is currently
// on the active call stack (i.e. between an EnterInstance and a
// LeaveInstance).
//
// This method serves the task-level concurrency trap at
// definitions.py:3664-3667, NOT the canonical-ABI
// definitions.py:290-299 call_might_be_recursive spec check. The two
// checks are different:
//
//   - definitions.py:290-299 is a STRUCTURAL reflexive-ancestor overlap
//     check implemented by component.Instance.CallMightBeRecursive via a
//     parent-chain walk. It does not consult this tracker.
//
//   - definitions.py:3664-3667 is a RUNTIME-STACK membership check used
//     when a canon.lift/lower boundary detects a recursive re-entry into
//     the same instance. That is this tracker's role.
//
// Do not conflate the two checks. Reintroducing a call from
// component.Instance.CallMightBeRecursive to this method was the B3/B4
// bug that the B4 corrective at commit b74f5558 removed.
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
