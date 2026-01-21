package component

// SubtaskState represents the state of a subtask.
type SubtaskState int

const (
	// SubtaskStatePending is the initial state before the call completes.
	SubtaskStatePending SubtaskState = iota
	// SubtaskStateResolved is set when the call has returned a value.
	SubtaskStateResolved
	// SubtaskStateFinishing is set during cleanup.
	SubtaskStateFinishing
	// SubtaskStateDone is set when the subtask is fully complete.
	SubtaskStateDone
)

// Subtask tracks the lifetime of a single canon_lower call.
// It owns a borrow scope for tracking borrowed handles during the call.
type Subtask struct {
	borrowScope *BorrowScope
	state       SubtaskState
	result      []Val // Stored result after resolve
}

// NewSubtask creates a new Subtask with its own borrow scope.
// The borrow scope is used to track borrowed handles during the call.
func NewSubtask(resourceTable *ResourceTable) *Subtask {
	return &Subtask{
		borrowScope: NewBorrowScope(resourceTable),
		state:       SubtaskStatePending,
	}
}

// BorrowScope returns the borrow scope for this subtask.
func (s *Subtask) BorrowScope() *BorrowScope {
	return s.borrowScope
}

// State returns the current state of this subtask.
func (s *Subtask) State() SubtaskState {
	return s.state
}
