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
// It owns a call context for tracking borrowed handles during the call.
type Subtask struct {
	callContext *CallContext
	state       SubtaskState
	result      []types.Val // Stored result after resolve
}

// NewSubtask creates a new Subtask with its own call context.
// The call context is used to track borrowed handles during the call.
func NewSubtask(table *Table) *Subtask {
	return &Subtask{
		callContext: NewCallContext(table),
		state:       SubtaskStatePending,
	}
}

// CallContext returns the call context for this subtask.
func (s *Subtask) CallContext() *CallContext {
	return s.callContext
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
// This completes the subtask and releases the call context's lends.
func (s *Subtask) Finish() error {
	if s.state == SubtaskStatePending {
		return fmt.Errorf("subtask: cannot finish before resolve")
	}
	if s.state == SubtaskStateDone {
		return fmt.Errorf("subtask: already done")
	}

	// Exit call context (validates borrows + releases lenders).
	if s.callContext != nil {
		if err := s.callContext.ExitCall(); err != nil {
			return fmt.Errorf("subtask: exit call: %w", err)
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
	if s.callContext == nil {
		return fmt.Errorf("subtask: no call context")
	}
	return s.callContext.AddLender(handle)
}

// LendCount returns the number of active lends in this subtask.
func (s *Subtask) LendCount() int {
	if s.callContext == nil {
		return 0
	}
	return s.callContext.LendCount()
}
