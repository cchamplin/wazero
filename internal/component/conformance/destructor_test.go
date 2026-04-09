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
	// Case 6: CrossInstanceDestructorDeferred — own handle where the
	// dropping instance differs from rt.Impl → cross-instance path.
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
	// Cross-instance destructor dispatch requires canon_lift/canon_lower
	// wiring which is deferred to Session 2.
	t.Run("CrossInstanceDestructorDeferred", func(t *testing.T) {
		t.Skip("Session 2: cross-instance destructor dispatch (spec definitions.py:2154-2160)")
	})
}
