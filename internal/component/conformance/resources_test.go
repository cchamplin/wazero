// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package conformance: resource lifecycle conformance tests ported from
// the canonical-abi test suite.
//
// Canonical test: run_tests.py::test_handles (line 441).
// Spec: definitions.py:1333-1347 lift_own/lift_borrow;
//
//	:2134-2173 canon_resource_new/drop/rep.
package conformance

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestResources ports the canonical-abi test_handles cases from
// run_tests.py (line 441) that exercise the synchronous resource
// lifecycle: resource.new, resource.rep, resource.drop, borrow scope
// tracking, and destructor invocation.
//
// Canonical test: run_tests.py::test_handles (line 441).
// Spec: definitions.py:2134-2173 canon_resource_new/rep/drop;
//
//	:1333-1347 lift_own/lift_borrow;
//	:337-349 ResourceHandle class.
func TestResources(t *testing.T) {

	// ------------------------------------------------------------------
	// Case 1: resource.new + resource.rep + resource.drop round-trip
	// ------------------------------------------------------------------
	// Ports: run_tests.py:479-481 (canon_resource_rep calls after
	//   resource.new via lift) and :499-503 (canon_resource_drop +
	//   canon_resource_new reuse cycle).
	// Spec: definitions.py:2134-2138 canon_resource_new;
	//       :2169-2173 canon_resource_rep;
	//       :2142-2165 canon_resource_drop.
	t.Run("NewRepDropRoundTrip", func(t *testing.T) {
		inst := component.NewInstance(&component.Component{}, 1, nil)
		rt := &runtime.ResourceType{Impl: inst.Runtime()}
		inst.Runtime().ResourceTypes = append(inst.Runtime().ResourceTypes, rt)

		// resource.new(42) -> h
		h, err := inst.ResourceNew(types.ResourceIdx(0), uint32(42))
		require.NoError(t, err)

		// resource.rep(h) == 42
		rep, err := inst.ResourceRep(types.ResourceIdx(0), h)
		require.NoError(t, err)
		require.Equal(t, uint32(42), rep)

		// resource.drop(h) — no destructor set, should succeed.
		err = inst.ResourceDrop(types.ResourceIdx(0), h)
		require.NoError(t, err)

		// After drop, rep must fail (handle removed from table).
		_, err = inst.ResourceRep(types.ResourceIdx(0), h)
		require.Error(t, err)
	})

	// ------------------------------------------------------------------
	// Case 2: resource.new traps when may_leave is false
	// ------------------------------------------------------------------
	// Ports: the spec precondition at definitions.py:2135 which
	//   run_tests.py exercises implicitly via the canon_lift/canon_lower
	//   context. The spec code is:
	//     def canon_resource_new(rt, thread, rep):
	//       trap_if(not thread.task.inst.may_leave)
	// Spec: definitions.py:2135 trap_if(not may_leave).
	t.Run("NewTrapMayLeave", func(t *testing.T) {
		inst := component.NewInstance(&component.Component{}, 1, nil)
		rt := &runtime.ResourceType{Impl: inst.Runtime()}
		inst.Runtime().ResourceTypes = append(inst.Runtime().ResourceTypes, rt)

		inst.SetMayLeave(false)

		_, err := inst.ResourceNew(types.ResourceIdx(0), uint32(42))
		require.Error(t, err)
	})

	// ------------------------------------------------------------------
	// Case 3: resource.drop traps when may_leave is false
	// ------------------------------------------------------------------
	// Spec: definitions.py:2143 trap_if(not may_leave).
	// The may_leave guard on resource.drop is symmetric with
	// resource.new (case 2).
	t.Run("DropTrapMayLeave", func(t *testing.T) {
		inst := component.NewInstance(&component.Component{}, 1, nil)
		rt := &runtime.ResourceType{Impl: inst.Runtime()}
		inst.Runtime().ResourceTypes = append(inst.Runtime().ResourceTypes, rt)

		h, err := inst.ResourceNew(types.ResourceIdx(0), uint32(42))
		require.NoError(t, err)

		inst.SetMayLeave(false)

		err = inst.ResourceDrop(types.ResourceIdx(0), h)
		require.Error(t, err)
	})

	// ------------------------------------------------------------------
	// Case 4: resource.drop traps on type mismatch
	// ------------------------------------------------------------------
	// Ports: run_tests.py does not have an explicit mismatch case, but
	//   the spec mandates the check at definitions.py:2147:
	//     trap_if(h.rt is not rt)
	// Spec: definitions.py:2147 trap_if(h.rt is not rt).
	t.Run("DropTrapTypeMismatch", func(t *testing.T) {
		inst := component.NewInstance(&component.Component{}, 1, nil)
		rtA := &runtime.ResourceType{Impl: inst.Runtime()}
		rtB := &runtime.ResourceType{Impl: inst.Runtime()}
		inst.Runtime().ResourceTypes = append(inst.Runtime().ResourceTypes, rtA, rtB)

		// Create handle with rtA (index 0).
		h, err := inst.ResourceNew(types.ResourceIdx(0), uint32(1))
		require.NoError(t, err)

		// Drop with rtB (index 1) — type mismatch.
		err = inst.ResourceDrop(types.ResourceIdx(1), h)
		require.Error(t, err)
	})

	// ------------------------------------------------------------------
	// Case 5: resource.drop traps when num_lends != 0
	// ------------------------------------------------------------------
	// Ports: the spec guard at definitions.py:2148:
	//     trap_if(h.num_lends != 0)
	// run_tests.py:500 (canon_resource_drop on h1 which has no lends)
	//   succeeds, while an attempt to drop a handle that is still lent
	//   would trap.
	// Spec: definitions.py:2148 trap_if(h.num_lends != 0).
	t.Run("DropTrapOutstandingLends", func(t *testing.T) {
		inst := component.NewInstance(&component.Component{}, 1, nil)
		rt := &runtime.ResourceType{Impl: inst.Runtime()}
		inst.Runtime().ResourceTypes = append(inst.Runtime().ResourceTypes, rt)

		h, err := inst.ResourceNew(types.ResourceIdx(0), uint32(1))
		require.NoError(t, err)

		// Increment lends on the handle (simulates what lift_borrow does
		// via CallContext.AddLender).
		fullHandle, _, err := inst.Table().GetByIndex(h)
		require.NoError(t, err)
		err = inst.Table().IncrementLends(fullHandle)
		require.NoError(t, err)

		// Drop must fail — num_lends > 0.
		err = inst.ResourceDrop(types.ResourceIdx(0), h)
		require.Error(t, err)
	})

	// ------------------------------------------------------------------
	// Case 6: resource.drop with destructor invocation
	// ------------------------------------------------------------------
	// Ports: run_tests.py:499-501
	//     dtor_value = None
	//     [] = canon_resource_drop(rt, thread, h1)
	//     assert(dtor_value == 42)
	// Spec: definitions.py:2149-2153 (own branch, inst is rt.impl,
	//   rt.dtor is set -> call destructor).
	t.Run("DropInvokesDestructor", func(t *testing.T) {
		inst := component.NewInstance(&component.Component{}, 1, nil)
		var dtorValue uint32
		var dtorCalled bool
		rt := &runtime.ResourceType{
			Impl: inst.Runtime(),
			HostDestructor: func(rep uint32) error {
				dtorCalled = true
				dtorValue = rep
				return nil
			},
		}
		inst.Runtime().ResourceTypes = append(inst.Runtime().ResourceTypes, rt)

		h, err := inst.ResourceNew(types.ResourceIdx(0), uint32(42))
		require.NoError(t, err)

		err = inst.ResourceDrop(types.ResourceIdx(0), h)
		require.NoError(t, err)
		require.True(t, dtorCalled, "destructor should have been called")
		require.Equal(t, uint32(42), dtorValue)
	})

	// ------------------------------------------------------------------
	// Case 7: resource.drop on borrow — destructor NOT called, scope
	//   borrow counter decremented
	// ------------------------------------------------------------------
	// Ports: run_tests.py:512-517
	//     dtor_value = None
	//     [] = canon_resource_drop(rt, thread, h3)
	//     assert(dtor_value is None)
	//   h3 is a borrow handle (BorrowType(rt)), so the spec takes the
	//   else branch at definitions.py:2163-2164:
	//     else: h.borrow_scope.num_borrows -= 1
	// Spec: definitions.py:2163-2164 (borrow branch of canon_resource_drop).
	// Wasmtime parallel: resources.rs:243-250 RemovedResource::Borrow.
	t.Run("DropBorrowBranch", func(t *testing.T) {
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

		// Step 1: create an own handle (the lender).
		ownIdx, err := inst.ResourceNew(types.ResourceIdx(0), uint32(77))
		require.NoError(t, err)

		ownFull, ownEntryIface, err := inst.Table().GetByIndex(ownIdx)
		require.NoError(t, err)
		ownEntry := ownEntryIface.(*runtime.ResourceHandleEntry)

		// Step 2: mint a borrow handle and register it in a call context.
		callCtx := runtime.NewCallContext(inst.Table())
		borrowFull, err := inst.Table().NewResourceHandle(uint32(77), false, rt)
		require.NoError(t, err)

		err = callCtx.AddLender(ownFull)
		require.NoError(t, err)

		borrowEntryIface, err := inst.Table().Get(borrowFull)
		require.NoError(t, err)
		borrowEntry := borrowEntryIface.(*runtime.ResourceHandleEntry)
		borrowEntry.CallContext = callCtx

		callCtx.IncrementBorrows()

		lendsBefore := ownEntry.NumLends
		require.True(t, lendsBefore > 0, "lender should have outstanding lends")
		require.Equal(t, 1, callCtx.NumBorrows())

		// Step 3: drop the borrow handle.
		err = inst.ResourceDrop(types.ResourceIdx(0), borrowFull.Index())
		require.NoError(t, err)

		// Destructor was NOT called (borrow branch).
		require.False(t, dtorCalled, "destructor must not be called for borrow drop")

		// Call context borrow counter was decremented.
		require.Equal(t, 0, callCtx.NumBorrows())

		// Lender's NumLends is NOT decremented at borrow-drop time.
		// Per wasmtime (resources.rs:338-345), lender NumLends cleanup
		// happens at exit_call, not at resource.drop.
		require.Equal(t, lendsBefore, ownEntry.NumLends)

		// Lender own handle still in table.
		_, err = inst.ResourceRep(types.ResourceIdx(0), ownIdx)
		require.NoError(t, err)

		// Borrow handle gone from table.
		_, _, err = inst.Table().GetByIndex(borrowFull.Index())
		require.Error(t, err)

		// Call context release at exit-call time decrements lender NumLends.
		err = callCtx.ExitCall()
		require.NoError(t, err)
		require.Equal(t, lendsBefore-1, ownEntry.NumLends)
	})

	// ------------------------------------------------------------------
	// Case 8: slot reuse after drop — free list recycles indices
	// ------------------------------------------------------------------
	// Ports: run_tests.py:506-510
	//     h = (canon_resource_new(rt, thread, 46))[0]
	//     assert(h == h1)
	//     assert(len(inst.table.array) == 6)
	//     assert(inst.table.array[h] is not None)
	//     assert(len(inst.table.free) == 0)
	// After dropping h1, creating a new handle reuses the same slot
	// index. The table length does not grow.
	// Spec: definitions.py:303-315 class Table (free-list reuse).
	t.Run("SlotReuseAfterDrop", func(t *testing.T) {
		inst := component.NewInstance(&component.Component{}, 1, nil)
		rt := &runtime.ResourceType{Impl: inst.Runtime()}
		inst.Runtime().ResourceTypes = append(inst.Runtime().ResourceTypes, rt)

		// Create three handles: indices 0, 1, 2.
		h0, err := inst.ResourceNew(types.ResourceIdx(0), uint32(10))
		require.NoError(t, err)
		h1, err := inst.ResourceNew(types.ResourceIdx(0), uint32(20))
		require.NoError(t, err)
		_, err = inst.ResourceNew(types.ResourceIdx(0), uint32(30))
		require.NoError(t, err)

		// Drop h0 — slot 0 goes to free list.
		err = inst.ResourceDrop(types.ResourceIdx(0), h0)
		require.NoError(t, err)

		// New handle should reuse slot index 0.
		hNew, err := inst.ResourceNew(types.ResourceIdx(0), uint32(46))
		require.NoError(t, err)
		require.Equal(t, h0, hNew, "new handle should reuse freed slot index")

		// The reused slot has the new rep.
		rep, err := inst.ResourceRep(types.ResourceIdx(0), hNew)
		require.NoError(t, err)
		require.Equal(t, uint32(46), rep)

		// Other handles are still valid.
		rep, err = inst.ResourceRep(types.ResourceIdx(0), h1)
		require.NoError(t, err)
		require.Equal(t, uint32(20), rep)
	})

	// ------------------------------------------------------------------
	// Case 9: resource.rep validates type identity (pointer equality)
	// ------------------------------------------------------------------
	// Spec: definitions.py:2172 trap_if(h.rt is not rt).
	// Two distinct ResourceType pointers for the same instance
	// must be treated as different types.
	t.Run("RepTrapTypeMismatch", func(t *testing.T) {
		inst := component.NewInstance(&component.Component{}, 1, nil)
		rtA := &runtime.ResourceType{Impl: inst.Runtime()}
		rtB := &runtime.ResourceType{Impl: inst.Runtime()}
		inst.Runtime().ResourceTypes = append(inst.Runtime().ResourceTypes, rtA, rtB)

		// Create with rtA.
		h, err := inst.ResourceNew(types.ResourceIdx(0), uint32(99))
		require.NoError(t, err)

		// Rep with rtB must fail.
		_, err = inst.ResourceRep(types.ResourceIdx(1), h)
		require.Error(t, err)
	})

	// ------------------------------------------------------------------
	// Case 10: multiple resources of different types in same instance
	// ------------------------------------------------------------------
	// Ports: run_tests.py:453-455 where rt and rt2 are distinct
	//   ResourceType objects in the same store:
	//     rt = ResourceType(ComponentInstance(store), dtor)
	//     rt2 = ResourceType(inst, dtor)
	//   The test passes rt2 handles alongside rt handles, verifying
	//   that type checking discriminates correctly.
	// Spec: definitions.py:351-361 ResourceType identity is pointer
	//   equality.
	t.Run("MultipleResourceTypes", func(t *testing.T) {
		inst := component.NewInstance(&component.Component{}, 1, nil)
		rtA := &runtime.ResourceType{Impl: inst.Runtime()}
		rtB := &runtime.ResourceType{Impl: inst.Runtime()}
		inst.Runtime().ResourceTypes = append(inst.Runtime().ResourceTypes, rtA, rtB)

		hA, err := inst.ResourceNew(types.ResourceIdx(0), uint32(100))
		require.NoError(t, err)
		hB, err := inst.ResourceNew(types.ResourceIdx(1), uint32(200))
		require.NoError(t, err)

		// Rep of hA with rtA succeeds.
		rep, err := inst.ResourceRep(types.ResourceIdx(0), hA)
		require.NoError(t, err)
		require.Equal(t, uint32(100), rep)

		// Rep of hB with rtB succeeds.
		rep, err = inst.ResourceRep(types.ResourceIdx(1), hB)
		require.NoError(t, err)
		require.Equal(t, uint32(200), rep)

		// Rep of hA with rtB fails (type mismatch).
		_, err = inst.ResourceRep(types.ResourceIdx(1), hA)
		require.Error(t, err)

		// Rep of hB with rtA fails (type mismatch).
		_, err = inst.ResourceRep(types.ResourceIdx(0), hB)
		require.Error(t, err)

		// Drop hA with rtA succeeds.
		err = inst.ResourceDrop(types.ResourceIdx(0), hA)
		require.NoError(t, err)

		// Drop hB with rtB succeeds.
		err = inst.ResourceDrop(types.ResourceIdx(1), hB)
		require.NoError(t, err)
	})

	// ------------------------------------------------------------------
	// Case 11: destructor NOT called when own handle belongs to same
	//   instance as rt.impl and no destructor is set
	// ------------------------------------------------------------------
	// Ports: run_tests.py:512-514
	//     dtor_value = None
	//     [] = canon_resource_drop(rt, thread, h3)
	//     assert(dtor_value is None)
	//   h3 is a borrow so the destructor is not called, but the same
	//   no-destructor path applies for own handles without a dtor.
	// Spec: definitions.py:2151-2153 — only calls dtor if rt.dtor
	//   is set.
	t.Run("DropOwnNoDestructor", func(t *testing.T) {
		inst := component.NewInstance(&component.Component{}, 1, nil)
		// No HostDestructor set.
		rt := &runtime.ResourceType{Impl: inst.Runtime()}
		inst.Runtime().ResourceTypes = append(inst.Runtime().ResourceTypes, rt)

		h, err := inst.ResourceNew(types.ResourceIdx(0), uint32(99))
		require.NoError(t, err)

		err = inst.ResourceDrop(types.ResourceIdx(0), h)
		require.NoError(t, err)

		// Verify handle is gone.
		_, err = inst.ResourceRep(types.ResourceIdx(0), h)
		require.Error(t, err)
	})

	// ------------------------------------------------------------------
	// Case 12: destructor receives correct rep value for each drop
	// ------------------------------------------------------------------
	// Ports: run_tests.py:499-501, :512-514
	//     The dtor callback records the rep value for each drop. The
	//     test verifies that each drop passes the correct rep.
	// Spec: definitions.py:2152-2153 rt.dtor(h.rep).
	t.Run("DestructorReceivesCorrectRep", func(t *testing.T) {
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

		h1, err := inst.ResourceNew(types.ResourceIdx(0), uint32(42))
		require.NoError(t, err)
		h2, err := inst.ResourceNew(types.ResourceIdx(0), uint32(43))
		require.NoError(t, err)
		h3, err := inst.ResourceNew(types.ResourceIdx(0), uint32(44))
		require.NoError(t, err)

		err = inst.ResourceDrop(types.ResourceIdx(0), h1)
		require.NoError(t, err)
		err = inst.ResourceDrop(types.ResourceIdx(0), h3)
		require.NoError(t, err)
		err = inst.ResourceDrop(types.ResourceIdx(0), h2)
		require.NoError(t, err)

		require.Equal(t, 3, len(dtorReps))
		require.Equal(t, uint32(42), dtorReps[0])
		require.Equal(t, uint32(44), dtorReps[1])
		require.Equal(t, uint32(43), dtorReps[2])
	})

	// ------------------------------------------------------------------
	// Case 13: CallContext.AddLender increments NumLends exactly once
	// ------------------------------------------------------------------
	// Ports: the lift_borrow spec path at definitions.py:1341-1347:
	//     def lift_borrow(cx, i, t):
	//       h = cx.inst.table.get(i)
	//       trap_if(not isinstance(h, ResourceHandle))
	//       trap_if(h.rt is not t.rt)
	//       cx.borrow_scope.add_lender(h)
	//       return h.rep
	//   AddLender increments h.num_lends and records h in the context.
	// Spec: definitions.py:736 self.lenders.append(h); h.num_lends += 1.
	t.Run("AddLenderIncrementsOnce", func(t *testing.T) {
		inst := component.NewInstance(&component.Component{}, 1, nil)
		rt := &runtime.ResourceType{Impl: inst.Runtime()}
		inst.Runtime().ResourceTypes = append(inst.Runtime().ResourceTypes, rt)

		h, err := inst.ResourceNew(types.ResourceIdx(0), uint32(55))
		require.NoError(t, err)

		fullHandle, entryIface, err := inst.Table().GetByIndex(h)
		require.NoError(t, err)
		entry := entryIface.(*runtime.ResourceHandleEntry)
		require.Equal(t, uint32(0), entry.NumLends)

		callCtx := runtime.NewCallContext(inst.Table())

		// AddLender once.
		err = callCtx.AddLender(fullHandle)
		require.NoError(t, err)
		require.Equal(t, uint32(1), entry.NumLends)
		require.Equal(t, 1, callCtx.LendCount())

		// AddLender again on the same handle (two borrows of same resource).
		err = callCtx.AddLender(fullHandle)
		require.NoError(t, err)
		require.Equal(t, uint32(2), entry.NumLends)
		require.Equal(t, 2, callCtx.LendCount())

		// Release decrements all.
		err = callCtx.ExitCall()
		require.NoError(t, err)
		require.Equal(t, uint32(0), entry.NumLends)
	})

	// ------------------------------------------------------------------
	// Case 14: full lifecycle mirrors test_handles core_wasm flow
	// ------------------------------------------------------------------
	// Ports: run_tests.py:466-519 (the core_wasm callback inside
	//   test_handles). This case exercises the entire sequence:
	//   1. Create 3 own handles (h1=42, h2=43, h3=44).
	//   2. resource.rep on all three.
	//   3. resource.drop h1 (calls destructor with rep=42).
	//   4. resource.new(46) reuses h1's slot.
	//   5. resource.drop h3 (no destructor for borrow; but here as
	//      own handle with destructor).
	//
	// Spec: definitions.py:2134-2173 full canon_resource_* suite.
	t.Run("FullLifecycleMirroringTestHandles", func(t *testing.T) {
		inst := component.NewInstance(&component.Component{}, 1, nil)
		var dtorValue *uint32
		rt := &runtime.ResourceType{
			Impl: inst.Runtime(),
			HostDestructor: func(rep uint32) error {
				v := rep
				dtorValue = &v
				return nil
			},
		}
		inst.Runtime().ResourceTypes = append(inst.Runtime().ResourceTypes, rt)

		// Step 1: create own handles for reps 42, 43, 44.
		h1, err := inst.ResourceNew(types.ResourceIdx(0), uint32(42))
		require.NoError(t, err)
		h2, err := inst.ResourceNew(types.ResourceIdx(0), uint32(43))
		require.NoError(t, err)
		h3, err := inst.ResourceNew(types.ResourceIdx(0), uint32(44))
		require.NoError(t, err)

		// Step 2: resource.rep on all three (run_tests.py:479-481).
		rep, err := inst.ResourceRep(types.ResourceIdx(0), h1)
		require.NoError(t, err)
		require.Equal(t, uint32(42), rep)

		rep, err = inst.ResourceRep(types.ResourceIdx(0), h2)
		require.NoError(t, err)
		require.Equal(t, uint32(43), rep)

		rep, err = inst.ResourceRep(types.ResourceIdx(0), h3)
		require.NoError(t, err)
		require.Equal(t, uint32(44), rep)

		// Step 3: drop h1 — destructor called with 42
		// (run_tests.py:499-501).
		dtorValue = nil
		err = inst.ResourceDrop(types.ResourceIdx(0), h1)
		require.NoError(t, err)
		require.NotNil(t, dtorValue)
		require.Equal(t, uint32(42), *dtorValue)

		// Step 4: resource.new(46) reuses h1's slot
		// (run_tests.py:506-510).
		hNew, err := inst.ResourceNew(types.ResourceIdx(0), uint32(46))
		require.NoError(t, err)
		require.Equal(t, h1, hNew, "should reuse freed slot index")

		rep, err = inst.ResourceRep(types.ResourceIdx(0), hNew)
		require.NoError(t, err)
		require.Equal(t, uint32(46), rep)

		// Step 5: drop h3 — destructor called with 44
		// (run_tests.py:512-514 is the borrow version; here we test
		// the own path since h3 is an own handle).
		dtorValue = nil
		err = inst.ResourceDrop(types.ResourceIdx(0), h3)
		require.NoError(t, err)
		require.NotNil(t, dtorValue)
		require.Equal(t, uint32(44), *dtorValue)

		// h2 and hNew still alive.
		rep, err = inst.ResourceRep(types.ResourceIdx(0), h2)
		require.NoError(t, err)
		require.Equal(t, uint32(43), rep)

		rep, err = inst.ResourceRep(types.ResourceIdx(0), hNew)
		require.NoError(t, err)
		require.Equal(t, uint32(46), rep)

		// Cleanup.
		err = inst.ResourceDrop(types.ResourceIdx(0), h2)
		require.NoError(t, err)
		err = inst.ResourceDrop(types.ResourceIdx(0), hNew)
		require.NoError(t, err)
	})

	// ------------------------------------------------------------------
	// Case 15: resource.rep on invalid/dropped handle traps
	// ------------------------------------------------------------------
	// Spec: definitions.py:2170-2171
	//     h = thread.task.inst.table.get(i)
	//     trap_if(not isinstance(h, ResourceHandle))
	// A freed slot is no longer a valid ResourceHandle.
	t.Run("RepInvalidHandle", func(t *testing.T) {
		inst := component.NewInstance(&component.Component{}, 1, nil)
		rt := &runtime.ResourceType{Impl: inst.Runtime()}
		inst.Runtime().ResourceTypes = append(inst.Runtime().ResourceTypes, rt)

		// Rep on a never-allocated index.
		_, err := inst.ResourceRep(types.ResourceIdx(0), uint32(999))
		require.Error(t, err)

		// Rep on a dropped handle.
		h, err := inst.ResourceNew(types.ResourceIdx(0), uint32(1))
		require.NoError(t, err)
		err = inst.ResourceDrop(types.ResourceIdx(0), h)
		require.NoError(t, err)
		_, err = inst.ResourceRep(types.ResourceIdx(0), h)
		require.Error(t, err)
	})

	// ------------------------------------------------------------------
	// Case 16: resource.drop on already-dropped handle traps
	// ------------------------------------------------------------------
	// Spec: definitions.py:2145 h = inst.table.remove(i) — remove
	//   on a freed slot traps.
	t.Run("DropAlreadyDropped", func(t *testing.T) {
		inst := component.NewInstance(&component.Component{}, 1, nil)
		rt := &runtime.ResourceType{Impl: inst.Runtime()}
		inst.Runtime().ResourceTypes = append(inst.Runtime().ResourceTypes, rt)

		h, err := inst.ResourceNew(types.ResourceIdx(0), uint32(1))
		require.NoError(t, err)
		err = inst.ResourceDrop(types.ResourceIdx(0), h)
		require.NoError(t, err)

		// Second drop must fail.
		err = inst.ResourceDrop(types.ResourceIdx(0), h)
		require.Error(t, err)
	})

	// ------------------------------------------------------------------
	// Case 17: CallContext tracks multiple lenders independently
	// ------------------------------------------------------------------
	// Ports: run_tests.py:489-492 where the core_wasm callback passes
	//   two borrow handles (h1, h3) to a host import:
	//     args = [h1, h3]
	//     results = canon_lower(opts, host_ft, host_import, thread, args)
	//   Each borrow-typed parameter goes through lift_borrow (spec
	//   definitions.py:1341-1347) which calls call_context.add_lender(h).
	// Spec: definitions.py:736 self.lenders.append(h); h.num_lends += 1.
	t.Run("CallContextMultipleLenders", func(t *testing.T) {
		inst := component.NewInstance(&component.Component{}, 1, nil)
		rt := &runtime.ResourceType{Impl: inst.Runtime()}
		inst.Runtime().ResourceTypes = append(inst.Runtime().ResourceTypes, rt)

		h1, err := inst.ResourceNew(types.ResourceIdx(0), uint32(42))
		require.NoError(t, err)
		h2, err := inst.ResourceNew(types.ResourceIdx(0), uint32(44))
		require.NoError(t, err)

		fullH1, entry1Iface, err := inst.Table().GetByIndex(h1)
		require.NoError(t, err)
		entry1 := entry1Iface.(*runtime.ResourceHandleEntry)

		fullH2, entry2Iface, err := inst.Table().GetByIndex(h2)
		require.NoError(t, err)
		entry2 := entry2Iface.(*runtime.ResourceHandleEntry)

		callCtx := runtime.NewCallContext(inst.Table())

		err = callCtx.AddLender(fullH1)
		require.NoError(t, err)
		err = callCtx.AddLender(fullH2)
		require.NoError(t, err)

		require.Equal(t, uint32(1), entry1.NumLends)
		require.Equal(t, uint32(1), entry2.NumLends)
		require.Equal(t, 2, callCtx.LendCount())

		// h1 and h2 cannot be dropped while lent.
		err = inst.ResourceDrop(types.ResourceIdx(0), h1)
		require.Error(t, err)
		err = inst.ResourceDrop(types.ResourceIdx(0), h2)
		require.Error(t, err)

		// Release call context — both lenders' NumLends decremented.
		err = callCtx.ExitCall()
		require.NoError(t, err)
		require.Equal(t, uint32(0), entry1.NumLends)
		require.Equal(t, uint32(0), entry2.NumLends)

		// Now both can be dropped.
		err = inst.ResourceDrop(types.ResourceIdx(0), h1)
		require.NoError(t, err)
		err = inst.ResourceDrop(types.ResourceIdx(0), h2)
		require.NoError(t, err)
	})

	// ------------------------------------------------------------------
	// Case 18: resource.new with invalid resource index traps
	// ------------------------------------------------------------------
	// Spec: the resource index must resolve to a concrete ResourceType
	//   in the instance's ResourceTypes pool. An out-of-range index is
	//   a trap (spec's definitions.py does not distinguish — the Python
	//   model would raise IndexError which is a trap).
	t.Run("NewInvalidResourceIdx", func(t *testing.T) {
		inst := component.NewInstance(&component.Component{}, 1, nil)
		// No resource types registered.

		_, err := inst.ResourceNew(types.ResourceIdx(0), uint32(42))
		require.Error(t, err)
	})
}
