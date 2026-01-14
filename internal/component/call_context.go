// internal/component/call_context.go

package component

import "errors"

var ErrOutstandingBorrows = errors.New("cannot return: borrow handles still remain")

// CallContext tracks state for a single component function call.
// Implements the Canonical ABI's Task tracking for borrow validation.
type CallContext struct {
	numBorrows int // Number of borrowed handles received by this call
}

// NewCallContext creates a new call context.
func NewCallContext() *CallContext {
	return &CallContext{}
}

// IncrementBorrows records receiving a borrowed handle.
// Called by lower_borrow when a borrowed handle is created.
func (c *CallContext) IncrementBorrows() {
	c.numBorrows++
}

// DecrementBorrows records dropping a borrowed handle.
// Called by resource.drop for borrowed handles.
func (c *CallContext) DecrementBorrows() {
	c.numBorrows--
}

// NumBorrows returns the current count of outstanding borrowed handles.
func (c *CallContext) NumBorrows() int {
	return c.numBorrows
}

// CanReturn returns true if the call can return (no outstanding borrows).
// Per spec, returning with outstanding borrows is a trap.
func (c *CallContext) CanReturn() bool {
	return c.numBorrows == 0
}

// ValidateReturn checks if returning is allowed.
// Returns an error if there are outstanding borrowed handles.
func (c *CallContext) ValidateReturn() error {
	if c.numBorrows > 0 {
		return ErrOutstandingBorrows
	}
	return nil
}
