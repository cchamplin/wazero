// internal/component/abi/resource_lower.go

package abi

import (
	"github.com/tetratelabs/wazero/internal/component"
)

// LowerBorrowWithType implements the Canonical ABI lower_borrow function with
// same-instance optimization. When lowering a borrow to the same component
// instance that defined the resource type, it returns the rep directly.
//
// From spec (CanonicalABI.md:2677-2683):
//
//	def lower_borrow(cx, rep, t):
//	  if cx.inst is t.rt.impl:
//	    return rep  # Same-instance optimization
//	  h = ResourceHandle(t.rt, rep, own = False, borrow_scope = cx.borrow_scope)
//	  h.borrow_scope.num_borrows += 1
//	  return cx.inst.table.add(h)
func LowerBorrowWithType(
	table *component.ResourceTable,
	callCtx *component.CallContext,
	rep uint32,
	resourceType component.ResourceTypeInfo,
	currentInstanceID uint32,
) (uint32, error) {
	// Same-instance optimization: return rep directly
	if currentInstanceID == resourceType.InstanceID() {
		return rep, nil
	}

	// Different instance: create a borrow handle
	handle := table.NewWithType(rep, false, resourceType.TypeID())

	// Track the borrow in the call context
	if callCtx != nil {
		callCtx.IncrementBorrows()
	}

	return uint32(handle), nil
}

// LowerOwnWithType implements the Canonical ABI lower_own function.
// Creates an owning handle in the table with type information.
//
// From spec (CanonicalABI.md:2673-2675):
//
//	def lower_own(cx, rep, t):
//	  h = ResourceHandle(t.rt, rep, own = True)
//	  return cx.inst.table.add(h)
func LowerOwnWithType(
	table *component.ResourceTable,
	rep uint32,
	resourceType component.ResourceTypeInfo,
) (uint32, error) {
	handle := table.NewWithType(rep, true, resourceType.TypeID())
	return uint32(handle), nil
}
