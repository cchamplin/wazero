// internal/component/instance_resource_ops_test.go
//
// Session 1 Task E5: tests for Instance.ResourceNew, ResourceRep, ResourceDrop.
//
// Spec: definitions.py:2134-2138 canon_resource_new,
//       :2142-2165 canon_resource_drop,
//       :2169-2173 canon_resource_rep.
package component

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestInstanceResourceNewSpecSignature asserts ResourceNew's signature
// matches spec canon_resource_new(rt, thread, rep).
//
// Spec: definitions.py:2134-2138 canon_resource_new:
//
//	def canon_resource_new(rt, thread, rep):
//	  trap_if(not thread.task.inst.may_leave)
//	  i = thread.task.inst.table.add(ResourceHandle(rt, rep, own=True))
//	  return [i]
func TestInstanceResourceNewSpecSignature(t *testing.T) {
	inst := newInstance(&Component{}, 1, nil)
	rt := &runtime.ResourceType{Impl: inst.rt}
	inst.rt.ResourceTypes = append(inst.rt.ResourceTypes, rt)

	h, err := inst.ResourceNew(types.ResourceIdx(0), uint32(42))
	require.NoError(t, err)
	// The returned index should be usable for a subsequent ResourceRep call.
	rep, err := inst.ResourceRep(types.ResourceIdx(0), h)
	require.NoError(t, err)
	require.Equal(t, uint32(42), rep)
}

// TestInstanceResourceNewTrapMayLeave asserts the may_leave trap.
// Spec: definitions.py:2135 trap_if(not may_leave).
func TestInstanceResourceNewTrapMayLeave(t *testing.T) {
	inst := newInstance(&Component{}, 1, nil)
	rt := &runtime.ResourceType{Impl: inst.rt}
	inst.rt.ResourceTypes = append(inst.rt.ResourceTypes, rt)
	inst.rt.MayLeave = false

	_, err := inst.ResourceNew(types.ResourceIdx(0), uint32(42))
	require.Error(t, err)
}

// TestInstanceResourceRepSpecSignature asserts ResourceRep returns the
// rep as uint32 and validates the type.
//
// Spec: definitions.py:2169-2173 canon_resource_rep:
//
//	def canon_resource_rep(rt, thread, i):
//	  h = thread.task.inst.table.get(i)
//	  trap_if(not isinstance(h, ResourceHandle))
//	  trap_if(h.rt is not rt)
//	  return [h.rep]
func TestInstanceResourceRepSpecSignature(t *testing.T) {
	inst := newInstance(&Component{}, 1, nil)
	rt := &runtime.ResourceType{Impl: inst.rt}
	inst.rt.ResourceTypes = append(inst.rt.ResourceTypes, rt)

	h, err := inst.ResourceNew(types.ResourceIdx(0), uint32(42))
	require.NoError(t, err)

	rep, err := inst.ResourceRep(types.ResourceIdx(0), h)
	require.NoError(t, err)
	require.Equal(t, uint32(42), rep)
}

// TestInstanceResourceDropSpecSignature asserts ResourceDrop removes
// the handle, validates type, and dispatches to the destructor.
//
// Spec: definitions.py:2142-2165 canon_resource_drop.
func TestInstanceResourceDropSpecSignature(t *testing.T) {
	inst := newInstance(&Component{}, 1, nil)
	var destructorCalls int
	rt := &runtime.ResourceType{
		Impl: inst.rt,
		HostDestructor: func(rep uint32) error {
			destructorCalls++
			require.Equal(t, uint32(42), rep)
			return nil
		},
	}
	inst.rt.ResourceTypes = append(inst.rt.ResourceTypes, rt)

	h, err := inst.ResourceNew(types.ResourceIdx(0), uint32(42))
	require.NoError(t, err)

	err = inst.ResourceDrop(types.ResourceIdx(0), h)
	require.NoError(t, err)
	require.Equal(t, 1, destructorCalls)

	// After drop, ResourceRep should fail.
	_, err = inst.ResourceRep(types.ResourceIdx(0), h)
	require.Error(t, err)
}

// TestInstanceResourceDropTypeMismatch asserts the type mismatch trap.
// Spec: definitions.py:2147 trap_if(h.rt is not rt).
func TestInstanceResourceDropTypeMismatch(t *testing.T) {
	inst := newInstance(&Component{}, 1, nil)
	rtA := &runtime.ResourceType{Impl: inst.rt}
	rtB := &runtime.ResourceType{Impl: inst.rt}
	inst.rt.ResourceTypes = append(inst.rt.ResourceTypes, rtA, rtB)

	h, err := inst.ResourceNew(types.ResourceIdx(0), uint32(1))
	require.NoError(t, err)

	// Drop with rtB (index 1) but handle belongs to rtA (index 0).
	err = inst.ResourceDrop(types.ResourceIdx(1), h)
	require.Error(t, err)
}

// TestInstanceResourceDropLendsTrap asserts own handles with outstanding
// lends trap on drop.
// Spec: definitions.py:2148 trap_if(h.num_lends != 0).
func TestInstanceResourceDropLendsTrap(t *testing.T) {
	inst := newInstance(&Component{}, 1, nil)
	rt := &runtime.ResourceType{Impl: inst.rt}
	inst.rt.ResourceTypes = append(inst.rt.ResourceTypes, rt)

	h, err := inst.ResourceNew(types.ResourceIdx(0), uint32(1))
	require.NoError(t, err)

	// Get the full 64-bit Handle from the table index so we can increment lends.
	fullHandle, _, err := inst.rt.Table.GetByIndex(h)
	require.NoError(t, err)
	err = inst.rt.Table.IncrementLends(fullHandle)
	require.NoError(t, err)

	// Drop should fail because NumLends > 0.
	err = inst.ResourceDrop(types.ResourceIdx(0), h)
	require.Error(t, err)
}

// TestInstanceResourceDropBorrowBranch asserts the borrow-handle drop path.
//
// Spec: definitions.py:2163-2164 (canon_resource_drop borrow branch):
//
//	else:  # borrow handle (not h.own)
//	  h.borrow_scope.num_borrows -= 1
//
// When a borrow handle is dropped via canon.resource.drop, the spec takes
// the else branch. It must NOT invoke the destructor, and the borrow
// handle is removed from the table.
//
// Wasmtime parallel: resources.rs:243-250 resource_drop → RemovedResource::Borrow
// decrements borrow_count on the scope. Lender NumLends cleanup happens
// separately at exit_call (resources.rs:338-345), NOT at borrow-drop time.
//
// In wazero's Session 1 synchronous model, Instance.ResourceDrop removes
// the borrow handle from the table. The CallContext/BorrowScope manages
// the borrow counter and lender NumLends cleanup at scope exit.
func TestInstanceResourceDropBorrowBranch(t *testing.T) {
	inst := newInstance(&Component{}, 1, nil)
	var destructorCalls int
	rt := &runtime.ResourceType{
		Impl: inst.rt,
		HostDestructor: func(rep uint32) error {
			destructorCalls++
			return nil
		},
	}
	inst.rt.ResourceTypes = append(inst.rt.ResourceTypes, rt)

	// Step 1: create an own handle (the lender) via ResourceNew.
	ownIdx, err := inst.ResourceNew(types.ResourceIdx(0), uint32(77))
	require.NoError(t, err)

	ownFull, ownEntryIface, err := inst.rt.Table.GetByIndex(ownIdx)
	require.NoError(t, err)
	ownEntry := ownEntryIface.(*runtime.ResourceHandleEntry)

	// Step 2: mint a borrow handle of the same type inside a fresh scope.
	// Simulates what abi.liftBorrowHandle does when a cross-component call
	// lifts an own handle from the caller and materializes a borrow in the
	// callee's table.
	scope := runtime.NewBorrowScope(inst.rt.Table)
	borrowFull, err := inst.rt.Table.NewResourceHandle(uint32(77), false, rt)
	require.NoError(t, err)

	// Register the lender in the scope (increments lender's NumLends).
	err = scope.AddLender(ownFull)
	require.NoError(t, err)

	// Associate the borrow entry with the scope.
	borrowEntryIface, err := inst.rt.Table.Get(borrowFull)
	require.NoError(t, err)
	borrowEntry := borrowEntryIface.(*runtime.ResourceHandleEntry)
	borrowEntry.BorrowScope = scope

	// Increment the scope's borrow counter (simulates what lower_borrow does).
	scope.IncrementBorrows()

	lendsBefore := ownEntry.NumLends
	require.True(t, lendsBefore > 0, "lender should have outstanding lends")
	require.Equal(t, 1, scope.NumBorrows(), "scope should have 1 outstanding borrow before drop")

	// Step 3: drop the borrow handle via ResourceDrop.
	err = inst.ResourceDrop(types.ResourceIdx(0), borrowFull.Index())
	require.NoError(t, err)

	// Step 4: destructor was NOT called (borrow branch — spec :2163-2164).
	require.Equal(t, 0, destructorCalls)

	// Step 4b: scope's borrow counter was decremented (spec :2163-2164).
	require.Equal(t, 0, scope.NumBorrows(), "scope borrow counter should be decremented after drop")

	// Step 5: lender's NumLends is NOT decremented at borrow-drop time.
	// Per wasmtime (resources.rs:338-345), lender NumLends cleanup happens
	// at exit_call, not at resource.drop. Instance.ResourceDrop only removes
	// the borrow handle from the table.
	lendsAfter := ownEntry.NumLends
	require.Equal(t, lendsBefore, lendsAfter)

	// Step 6: the lender own handle is still in the table.
	_, err = inst.ResourceRep(types.ResourceIdx(0), ownIdx)
	require.NoError(t, err)

	// Step 7: the borrow handle is gone from the table.
	_, _, err = inst.rt.Table.GetByIndex(borrowFull.Index())
	require.Error(t, err)

	// Step 8: scope.Release() at exit-call time decrements lender NumLends.
	err = scope.Release()
	require.NoError(t, err)
	require.Equal(t, lendsBefore-1, ownEntry.NumLends)
}
