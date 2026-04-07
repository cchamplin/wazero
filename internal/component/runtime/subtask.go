// internal/component/runtime/subtask.go

package runtime

import (
	"fmt"

	"github.com/tetratelabs/wazero/internal/component/types"
)

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
	result      []types.Val // Stored result after resolve
}

// NewSubtask creates a new Subtask with its own borrow scope.
// The borrow scope is used to track borrowed handles during the call.
func NewSubtask(table *Table) *Subtask {
	return &Subtask{
		borrowScope: NewBorrowScope(table),
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

// DeliverResolve transitions the subtask from pending to resolved with a result.
// This is called when the lowered function returns.
func (s *Subtask) DeliverResolve(result []types.Val) error {
	if s.state != SubtaskStatePending {
		return fmt.Errorf("subtask: cannot resolve in state %d", s.state)
	}
	s.result = result
	s.state = SubtaskStateResolved
	return nil
}

// StartFinish transitions from resolved to finishing.
// This begins the cleanup phase.
func (s *Subtask) StartFinish() error {
	if s.state != SubtaskStateResolved {
		return fmt.Errorf("subtask: cannot start finish in state %d", s.state)
	}
	s.state = SubtaskStateFinishing
	return nil
}

// Finish transitions from finishing to done.
// This completes the subtask and releases the borrow scope.
func (s *Subtask) Finish() error {
	if s.state == SubtaskStatePending {
		return fmt.Errorf("subtask: cannot finish before resolve")
	}
	if s.state == SubtaskStateDone {
		return fmt.Errorf("subtask: already done")
	}

	// Release borrows
	if s.borrowScope != nil {
		if err := s.borrowScope.Release(); err != nil {
			return fmt.Errorf("subtask: release borrows: %w", err)
		}
	}

	s.state = SubtaskStateDone
	return nil
}

// Result returns the stored result after resolution.
// Returns nil if not yet resolved.
func (s *Subtask) Result() []types.Val {
	return s.result
}

// TrackLend records that a handle has been lent (borrowed) during this subtask.
// The lend will be released when the subtask finishes.
func (s *Subtask) TrackLend(handle Handle) error {
	if s.borrowScope == nil {
		return fmt.Errorf("subtask: no borrow scope")
	}
	return s.borrowScope.AddLender(handle)
}

// LendCount returns the number of active lends in this subtask.
func (s *Subtask) LendCount() int {
	if s.borrowScope == nil {
		return 0
	}
	return s.borrowScope.LendCount()
}
