// internal/component/reentrance.go

package component

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

// CallMightBeRecursive returns true if calling the given instance would be recursive.
// This implements the spec's call_might_be_recursive check.
//
// From spec: A call to an instance is recursive if that instance is already
// on the current call stack.
func (r *ReentranceTracker) CallMightBeRecursive(instanceID uint32) bool {
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
