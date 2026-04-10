// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package conformance: destructor integration tests covering the destructor
// dispatch branch of canon_resource_drop.
//
// Spec: definitions.py:2149-2161 (destructor dispatch in canon_resource_drop).
package conformance

import (
	"errors"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestDestructors exercises the destructor dispatch paths in
// canon_resource_drop (spec definitions.py:2149-2161). Local-instance
// cases are fully tested; cross-instance cases are deferred to Session 2.
func TestDestructors(t *testing.T) {

	// ------------------------------------------------------------------
	// Case 1: LocalDestructorInvoked — own handle dropped on the same
	// instance that defines the resource → HostDestructor called with
	// the correct rep.
	// ------------------------------------------------------------------
	//
	// Spec: definitions.py:2149-2153
	//   if h.own:
	//     assert(h.borrow_scope is None)
	//     if inst is rt.impl:
	//       if rt.dtor:
	//         rt.dtor(h.rep)
	//
	// When the dropping instance (`inst`) IS the defining instance
	// (`rt.impl`) and a destructor is declared, the destructor is
	// invoked with `h.rep`.
	t.Run("LocalDestructorInvoked", func(t *testing.T) {
		inst := component.NewInstance(&component.Component{}, 1, nil)
		var dtorCalled bool
		var dtorRep uint32
		rt := &runtime.ResourceType{
			Impl: inst.Runtime(),
			HostDestructor: func(rep uint32) error {
				dtorCalled = true
				dtorRep = rep
				return nil
			},
		}
		inst.Runtime().ResourceTypes = append(inst.Runtime().ResourceTypes, rt)

		h, err := inst.ResourceNew(types.ResourceIdx(0), uint32(42))
		require.NoError(t, err)

		err = inst.ResourceDrop(types.ResourceIdx(0), h)
		require.NoError(t, err)
		require.True(t, dtorCalled, "destructor should have been invoked")
		require.Equal(t, uint32(42), dtorRep)
	})

	// ------------------------------------------------------------------
	// Case 2: LocalDestructorNotCalledOnBorrow — dropping a borrow
	// handle must NOT invoke the destructor; it takes the else branch.
	// ------------------------------------------------------------------
	//
	// Spec: definitions.py:2163-2164
	//   else:  # not h.own
	//     h.borrow_scope.num_borrows -= 1
	//
	// The destructor dispatch at :2149-2153 is only reached for own
	// handles. Borrow handles take the else branch and merely
	// decrement the scope borrow counter.
	t.Run("LocalDestructorNotCalledOnBorrow", func(t *testing.T) {
		inst := component.NewInstance(&component.Component{}, 1, nil)
		var dtorCalled bool
		rt := &runtime.ResourceType{
			Impl: inst.Runtime(),
			HostDestructor: func(rep uint32) error {
				dtorCalled = true
				return nil
			},
		}
		inst.Runtime().ResourceTypes = append(inst.Runtime().ResourceTypes, rt)

		// Create an own handle as the lender.
		ownIdx, err := inst.ResourceNew(types.ResourceIdx(0), uint32(77))
		require.NoError(t, err)

		ownFull, _, err := inst.Table().GetByIndex(ownIdx)
		require.NoError(t, err)

		// Mint a borrow handle in a scope.
		scope := runtime.NewBorrowScope(inst.Table())
		borrowFull, err := inst.Table().NewResourceHandle(uint32(77), false, rt)
		require.NoError(t, err)

		err = scope.AddLender(ownFull)
		require.NoError(t, err)

		borrowEntryIface, err := inst.Table().Get(borrowFull)
		require.NoError(t, err)
		borrowEntry := borrowEntryIface.(*runtime.ResourceHandleEntry)
		borrowEntry.BorrowScope = scope

		scope.IncrementBorrows()

		// Drop the borrow handle.
		err = inst.ResourceDrop(types.ResourceIdx(0), borrowFull.Index())
		require.NoError(t, err)

		// Destructor must NOT have been called.
		require.False(t, dtorCalled, "destructor must not be called for borrow handle drop")

		// Cleanup: release scope and drop the own handle.
		err = scope.Release()
		require.NoError(t, err)
		err = inst.ResourceDrop(types.ResourceIdx(0), ownIdx)
		require.NoError(t, err)
	})

	// ------------------------------------------------------------------
	// Case 3: NoDestructorDeclared — own handle with nil Dtor and nil
	// HostDestructor → HasDestructor() is false, drop is a no-op on
	// the destructor path.
	// ------------------------------------------------------------------
	//
	// Spec: definitions.py:2151-2153
	//   if inst is rt.impl:
	//     if rt.dtor:
	//       rt.dtor(h.rep)
	//
	// When `rt.dtor` is falsy (None/nil), the destructor branch is
	// skipped entirely and the handle is simply removed from the table.
	t.Run("NoDestructorDeclared", func(t *testing.T) {
		inst := component.NewInstance(&component.Component{}, 1, nil)
		// No destructor set — both Dtor and HostDestructor are nil.
		rt := &runtime.ResourceType{Impl: inst.Runtime()}
		inst.Runtime().ResourceTypes = append(inst.Runtime().ResourceTypes, rt)

		h, err := inst.ResourceNew(types.ResourceIdx(0), uint32(99))
		require.NoError(t, err)

		// Drop should succeed silently — no destructor to invoke.
		err = inst.ResourceDrop(types.ResourceIdx(0), h)
		require.NoError(t, err)

		// Handle is removed from the table.
		_, err = inst.ResourceRep(types.ResourceIdx(0), h)
		require.Error(t, err)
	})

	// ------------------------------------------------------------------
	// Case 4: DestructorErrorPropagated — HostDestructor returns an
	// error → ResourceDrop propagates the error to the caller.
	// ------------------------------------------------------------------
	//
	// Spec: definitions.py:2152-2153
	//   if rt.dtor:
	//     rt.dtor(h.rep)
	//
	// In the spec, the destructor is a core function that can trap.
	// In our host-destructor model, the Go callback returns an error
	// which ResourceDrop wraps and returns.
	t.Run("DestructorErrorPropagated", func(t *testing.T) {
		inst := component.NewInstance(&component.Component{}, 1, nil)
		dtorErr := errors.New("destructor failed: simulated error")
		rt := &runtime.ResourceType{
			Impl: inst.Runtime(),
			HostDestructor: func(rep uint32) error {
				return dtorErr
			},
		}
		inst.Runtime().ResourceTypes = append(inst.Runtime().ResourceTypes, rt)

		h, err := inst.ResourceNew(types.ResourceIdx(0), uint32(42))
		require.NoError(t, err)

		err = inst.ResourceDrop(types.ResourceIdx(0), h)
		require.Error(t, err)
	})

	// ------------------------------------------------------------------
	// Case 5: MultipleDropsSameType — drop several own handles of the
	// same type, verify each destructor call receives the correct rep.
	// ------------------------------------------------------------------
	//
	// Spec: definitions.py:2152-2153
	//   if rt.dtor:
	//     rt.dtor(h.rep)
	//
	// Each canon_resource_drop invocation is independent; each passes
	// the dropped handle's rep to the destructor.
	t.Run("MultipleDropsSameType", func(t *testing.T) {
		inst := component.NewInstance(&component.Component{}, 1, nil)
		var dtorReps []uint32
		rt := &runtime.ResourceType{
			Impl: inst.Runtime(),
			HostDestructor: func(rep uint32) error {
				dtorReps = append(dtorReps, rep)
				return nil
			},
		}
		inst.Runtime().ResourceTypes = append(inst.Runtime().ResourceTypes, rt)

		reps := []uint32{10, 20, 30, 40, 50}
		handles := make([]uint32, len(reps))
		for i, rep := range reps {
			h, err := inst.ResourceNew(types.ResourceIdx(0), rep)
			require.NoError(t, err)
			handles[i] = h
		}

		// Drop in reverse order.
		for i := len(handles) - 1; i >= 0; i-- {
			err := inst.ResourceDrop(types.ResourceIdx(0), handles[i])
			require.NoError(t, err)
		}

		require.Equal(t, len(reps), len(dtorReps))
		// Verify each destructor call received the correct rep (reverse order).
		for i, rep := range dtorReps {
			expected := reps[len(reps)-1-i]
			require.Equal(t, expected, rep)
		}
	})

	// ------------------------------------------------------------------
	// Case 6: CrossInstanceDestructor — own handle where the dropping
	// instance differs from rt.Impl → cross-instance destructor via
	// HostDestructor closure.
	// ------------------------------------------------------------------
	//
	// Spec: definitions.py:2154-2160
	//   else:  # inst is not rt.impl
	//     if rt.dtor:
	//       caller_opts = CanonicalOptions(async_ = False)
	//       callee_opts = CanonicalOptions(...)
	//       ft = FuncType([U32Type()],[], async_ = False)
	//       callee = partial(canon_lift, callee_opts, rt.impl, ft, rt.dtor)
	//       [] = canon_lower(caller_opts, ft, callee, thread, [h.rep])
	//
	// The HostDestructor closure bridges the cross-instance gap until
	// canon_lift/canon_lower pipeline lands (Task 7/9).
	t.Run("CrossInstanceDestructor", func(t *testing.T) {
		// Two instances sharing a store — instA defines the resource,
		// instB drops it.
		store := runtime.NewResourceStore()
		instA := component.NewInstance(&component.Component{}, 1, nil)
		instA.Runtime().Store = store
		instB := component.NewInstance(&component.Component{}, 2, nil)
		instB.Runtime().Store = store

		var destructorCalled bool
		var destructorRep uint32
		rt := &runtime.ResourceType{
			Impl: instA.Runtime(),
			HostDestructor: func(rep uint32) error {
				destructorCalled = true
				destructorRep = rep
				return nil
			},
		}
		instA.Runtime().ResourceTypes = append(instA.Runtime().ResourceTypes, rt)
		store.Register(1, 0, rt)
		store.RegisterInstance(1, instA)
		store.RegisterInstance(2, instB)

		// Register the resource type on instB so ResourceDrop can look it up.
		instB.Runtime().ResourceTypes = append(instB.Runtime().ResourceTypes, rt)

		// Create an own handle in instB's table for this resource type.
		h, err := instB.Runtime().Table.NewResourceHandle(uint32(55), true, rt)
		require.NoError(t, err)

		// Drop from instB — should invoke destructor via cross-instance path
		// because instB.rt != rt.Impl (which is instA.rt).
		err = instB.ResourceDrop(types.ResourceIdx(0), h.Index())
		require.NoError(t, err)
		require.True(t, destructorCalled, "cross-instance destructor should have been invoked")
		require.Equal(t, uint32(55), destructorRep)
	})

	// ------------------------------------------------------------------
	// Case 7: CrossInstanceNoDestructorReentranceCheck — cross-instance
	// drop with no destructor checks call_might_be_recursive.
	// ------------------------------------------------------------------
	//
	// Spec: definitions.py:2162
	//   else:
	//     trap_if(call_might_be_recursive(thread.task, rt.impl))
	//
	// When there's no destructor and the drop is cross-instance, the
	// spec requires a reentrance check. If the defining instance is
	// already on the call stack, the drop must trap.
	t.Run("CrossInstanceNoDestructorReentranceCheck", func(t *testing.T) {
		store := runtime.NewResourceStore()
		// instA defines the resource (no destructor).
		instA := component.NewInstance(&component.Component{}, 1, nil)
		instA.Runtime().Store = store
		// instB drops it.
		instB := component.NewInstance(&component.Component{}, 2, nil)
		instB.Runtime().Store = store

		rt := &runtime.ResourceType{
			Impl: instA.Runtime(),
			// No destructor — HostDestructor and Dtor are both nil.
		}
		instA.Runtime().ResourceTypes = append(instA.Runtime().ResourceTypes, rt)
		instB.Runtime().ResourceTypes = append(instB.Runtime().ResourceTypes, rt)
		store.Register(1, 0, rt)
		store.RegisterInstance(1, instA)
		store.RegisterInstance(2, instB)

		// Create an own handle in instB's table.
		h, err := instB.Runtime().Table.NewResourceHandle(uint32(66), true, rt)
		require.NoError(t, err)

		// Without reentrance: drop should succeed.
		err = instB.ResourceDrop(types.ResourceIdx(0), h.Index())
		require.NoError(t, err)

		// Now simulate instA being on the call stack by making instB a
		// child of instA (so isReflexiveAncestor(instB, instA) is true,
		// meaning CallMightBeRecursive returns true).
		instA2 := component.NewInstance(&component.Component{}, 1, nil)
		instA2.Runtime().Store = store
		instB2 := component.NewInstance(&component.Component{}, 2, instA2)
		instB2.Runtime().Store = store
		instA2.AddChild(instB2)

		rt2 := &runtime.ResourceType{
			Impl: instA2.Runtime(),
		}
		instA2.Runtime().ResourceTypes = append(instA2.Runtime().ResourceTypes, rt2)
		instB2.Runtime().ResourceTypes = append(instB2.Runtime().ResourceTypes, rt2)
		store.Register(1, 0, rt2)
		store.RegisterInstance(1, instA2)
		store.RegisterInstance(2, instB2)

		h2, err := instB2.Runtime().Table.NewResourceHandle(uint32(77), true, rt2)
		require.NoError(t, err)

		// instB2 is a child of instA2, so CallMightBeRecursive(instB2)
		// on instA2 returns true → drop should trap.
		err = instB2.ResourceDrop(types.ResourceIdx(0), h2.Index())
		require.Error(t, err)
		require.Contains(t, err.Error(), "recursive")
	})

	// ------------------------------------------------------------------
	// Case 8: CrossInstanceDestructorError — cross-instance destructor
	// returns an error → ResourceDrop propagates it.
	// ------------------------------------------------------------------
	t.Run("CrossInstanceDestructorError", func(t *testing.T) {
		store := runtime.NewResourceStore()
		instA := component.NewInstance(&component.Component{}, 1, nil)
		instA.Runtime().Store = store
		instB := component.NewInstance(&component.Component{}, 2, nil)
		instB.Runtime().Store = store

		dtorErr := errors.New("cross-instance destructor failed")
		rt := &runtime.ResourceType{
			Impl: instA.Runtime(),
			HostDestructor: func(rep uint32) error {
				return dtorErr
			},
		}
		instA.Runtime().ResourceTypes = append(instA.Runtime().ResourceTypes, rt)
		instB.Runtime().ResourceTypes = append(instB.Runtime().ResourceTypes, rt)
		store.Register(1, 0, rt)
		store.RegisterInstance(1, instA)
		store.RegisterInstance(2, instB)

		h, err := instB.Runtime().Table.NewResourceHandle(uint32(88), true, rt)
		require.NoError(t, err)

		err = instB.ResourceDrop(types.ResourceIdx(0), h.Index())
		require.Error(t, err)
		require.Contains(t, err.Error(), "cross-instance destructor")
	})
}
