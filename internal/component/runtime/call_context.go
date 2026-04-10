// internal/component/runtime/call_context.go

package runtime

import (
	"errors"
	"fmt"
)

var ErrOutstandingBorrows = errors.New("cannot return: borrow handles still remain")

// CallContext tracks state for a single component function call.
// Implements the Canonical ABI's Task tracking for borrow validation
// and the Subtask.lenders tracking (previously in BorrowScope).
//
// Spec: definitions.py:571 Task fields (num_borrows, lenders).
// Wasmtime parallel: runtime/component/resources.rs:189-192 CallContext.
type CallContext struct {
	table      *Table   // The resource table for lend tracking (may be nil for borrow-count-only contexts)
	numBorrows int      // Number of borrowed handles received by this call
	lenders    []Handle // Handles that were borrowed FROM during this call
}

// NewCallContext creates a new call context bound to the given table.
// The table is used for IncrementLends/DecrementLends on lender handles.
// Pass nil when only borrow-count tracking is needed (no lend management).
func NewCallContext(table *Table) *CallContext {
	return &CallContext{
		table:   table,
		lenders: make([]Handle, 0),
	}
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

// AddLender records a handle that was borrowed FROM during this call.
// Increments the handle's NumLends counter in the table and appends it
// to the lender set. This is used by lift_borrow to track which handles
// need their num_lends decremented when the call exits.
//
// Spec: definitions.py:736 self.lenders.append(h); h.num_lends += 1.
func (c *CallContext) AddLender(h Handle) error {
	if c.table != nil {
		if err := c.table.IncrementLends(h); err != nil {
			return err
		}
	}
	c.lenders = append(c.lenders, h)
	return nil
}

// Lenders returns the list of handles that were borrowed from.
func (c *CallContext) Lenders() []Handle {
	return c.lenders
}


// ReleaseBorrow releases a single borrow from the scope: it decrements
// the handle's NumLends counter (via DecrementLends) and removes the
// handle from the lender set. This is the symmetric inverse of
// AddLender and is called by canon_resource_drop for borrow handles.
//
// Spec: definitions.py:2163-2164 canon_resource_drop borrow branch
//
//	h.borrow_scope.num_borrows -= 1
//
// Spec: definitions.py:738-742 deliver_resolve (scope closure).
func (c *CallContext) ReleaseBorrow(h Handle) error {
	idx := -1
	for i, lh := range c.lenders {
		if lh == h {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("ReleaseBorrow: handle %d not found in call context lender set", h)
	}
	if c.table != nil {
		if err := c.table.DecrementLends(h); err != nil {
			return err
		}
	}
	// Remove the handle from the lender set (order-preserving removal).
	c.lenders = append(c.lenders[:idx], c.lenders[idx+1:]...)
	return nil
}

// HasOutstandingBorrows returns true if any handles are still borrowed.
func (c *CallContext) HasOutstandingBorrows() bool {
	return len(c.lenders) > 0
}

// LendCount returns the number of active lends in this scope.
func (c *CallContext) LendCount() int {
	return len(c.lenders)
}

// ExitCall validates that the call can return and undoes all lend
// operations. This is the single cleanup path for a call scope.
// Returns an error if there are outstanding borrows (handles not dropped)
// or if any lend decrement fails.
//
// Spec: definitions.py exit_call — trap if borrow_count > 0, then
// decrement num_lends for all lenders.
func (c *CallContext) ExitCall() error {
	// Spec: trap if borrow_count > 0
	if err := c.ValidateReturn(); err != nil {
		return err
	}

	// Undo all lend operations (decrement num_lends on source handles).
	if c.table != nil {
		for _, h := range c.lenders {
			if err := c.table.DecrementLends(h); err != nil {
				return err
			}
		}
	}

	c.lenders = c.lenders[:0]
	return nil
}
