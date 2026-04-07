// internal/component/runtime/instance_state.go

package runtime

// InstanceState tracks the execution state of a component instance.
// This includes the may_leave flag required by the Canonical ABI.
//
// From spec (CanonicalABI.md:3604-3609):
//
//	def canon_resource_new(rt, thread, rep):
//	  trap_if(not thread.task.inst.may_leave)
//	  ...
type InstanceState struct {
	id         uint32
	enterCount int  // Number of active Enter() calls
	mayLeave   bool // Can the instance perform operations that "leave" it?
}

// NewInstanceState creates a new instance state with the given ID.
func NewInstanceState(id uint32) *InstanceState {
	return &InstanceState{
		id:       id,
		mayLeave: true,
	}
}

// ID returns the instance ID.
func (s *InstanceState) ID() uint32 {
	return s.id
}

// MayLeave returns whether the instance may perform "leave" operations.
// Resource operations like resource.new and resource.drop require this to be true.
func (s *InstanceState) MayLeave() bool {
	return s.mayLeave && s.enterCount == 0
}

// SetMayLeave sets the may_leave flag directly.
func (s *InstanceState) SetMayLeave(may bool) {
	s.mayLeave = may
}

// Enter marks entry into a region where may_leave should be false.
// This is called when entering code that shouldn't be reentered.
func (s *InstanceState) Enter() {
	s.enterCount++
}

// Leave marks exit from a region where may_leave was false.
// Must be paired with Enter.
func (s *InstanceState) Leave() {
	if s.enterCount > 0 {
		s.enterCount--
	}
}

// EnterCount returns the current nesting depth of Enter calls.
func (s *InstanceState) EnterCount() int {
	return s.enterCount
}
