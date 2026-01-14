// internal/component/borrow_scope.go

package component

// BorrowScope tracks resource handles that have been lent during a call.
// When the call completes, all lends must be released.
// This implements the Canonical ABI's Subtask.lenders tracking.
type BorrowScope struct {
	table   *ResourceTable
	lenders []Handle // Handles that were borrowed from
}

// NewBorrowScope creates a new borrow scope for tracking lends.
func NewBorrowScope(table *ResourceTable) *BorrowScope {
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
