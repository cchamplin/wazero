// internal/component/runtime/borrow_scope.go

package runtime

import "fmt"

// BorrowScope tracks resource handles that have been lent during a call.
// When the call completes, all lends must be released.
// This implements the Canonical ABI's Subtask.lenders tracking.
type BorrowScope struct {
	table   *Table
	lenders []Handle // Handles that were borrowed from
}

// NewBorrowScope creates a new borrow scope for tracking lends.
func NewBorrowScope(table *Table) *BorrowScope {
	return &BorrowScope{
		table:   table,
		lenders: nil,
	}
}

// AddLender records that a handle has been borrowed from.
// Increments the handle's NumLends counter.
func (s *BorrowScope) AddLender(h Handle) error {
	if err := s.table.IncrementLends(h); err != nil {
		return err
	}
	s.lenders = append(s.lenders, h)
	return nil
}

// ReleaseBorrow releases a single borrow from the scope: it decrements
// the handle's NumLends counter (via DecrementLends) and removes the
// handle from the scope's lender set. This is the symmetric inverse of
// AddLender and is called by canon_resource_drop for borrow handles.
//
// Spec: definitions.py:2163-2164 canon_resource_drop borrow branch
//
//	h.borrow_scope.num_borrows -= 1
//
// Spec: definitions.py:738-742 deliver_resolve (scope closure).
func (s *BorrowScope) ReleaseBorrow(h Handle) error {
	idx := -1
	for i, lh := range s.lenders {
		if lh == h {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("ReleaseBorrow: handle %d not found in borrow scope lender set", h)
	}
	if err := s.table.DecrementLends(h); err != nil {
		return err
	}
	// Remove the handle from the lender set (order-preserving removal).
	s.lenders = append(s.lenders[:idx], s.lenders[idx+1:]...)
	return nil
}

// Release decrements NumLends for all tracked lenders.
// Called when the call scope completes.
func (s *BorrowScope) Release() error {
	for _, h := range s.lenders {
		if err := s.table.DecrementLends(h); err != nil {
			return err
		}
	}
	s.lenders = nil
	return nil
}

// HasOutstandingBorrows returns true if any handles are still borrowed.
func (s *BorrowScope) HasOutstandingBorrows() bool {
	return len(s.lenders) > 0
}

// LendCount returns the number of active lends in this scope.
func (s *BorrowScope) LendCount() int {
	return len(s.lenders)
}
