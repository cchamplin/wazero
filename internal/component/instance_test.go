// internal/component/instance_test.go
//
// Session 1 Task E12: restore resource-related tests from Session 0 stubs.
//
// Spec: definitions.py:2134-2173 canon_resource_new/drop/rep.
//
// Session 1 Task B3 adds the TestInstance{EmbedsRuntimeComponentInstance,
// MayLeaveDelegatesToRuntime, CallMightBeRecursiveUsesReentranceTracker}
// tests at the bottom of this file.
package component

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

const instanceTestSkipMsg = "session 1 work: see docs/plans/2026-04-07-canonical-abi-unification-session0-followup.md"

// TestInstanceStructure asserts that newInstance produces a well-formed
// Instance with its embedded runtime.ComponentInstance, component
// back-pointer, and initialized maps/slices.
//
// Spec: definitions.py:256-273 class ComponentInstance.
func TestInstanceStructure(t *testing.T) {
	c := &Component{}
	inst := newInstance(c, 42, nil)

	require.NotNil(t, inst)
	require.NotNil(t, inst.rt)
	require.Equal(t, c, inst.Component())
	require.NotNil(t, inst.Runtime())
	require.NotNil(t, inst.Table())
	require.True(t, inst.MayLeave())
	require.Equal(t, 0, inst.ActiveCallDepth())
	require.Nil(t, inst.Parent())
}

// TestCanonResourceNew asserts ResourceNew creates an own handle and
// returns a valid table index.
//
// Spec: definitions.py:2134-2138 canon_resource_new.
func TestCanonResourceNew(t *testing.T) {
	inst := newInstance(&Component{}, 1, nil)
	rt := &runtime.ResourceType{Impl: inst.rt}
	inst.rt.ResourceTypes = append(inst.rt.ResourceTypes, rt)

	idx, err := inst.ResourceNew(types.ResourceIdx(0), uint32(99))
	require.NoError(t, err)

	// The returned index should be valid for ResourceRep.
	rep, err := inst.ResourceRep(types.ResourceIdx(0), idx)
	require.NoError(t, err)
	require.Equal(t, uint32(99), rep)
}

// TestCanonResourceNew_MultipleResources asserts ResourceNew works with
// multiple distinct resource types on the same instance.
//
// Spec: definitions.py:2134-2138 canon_resource_new.
func TestCanonResourceNew_MultipleResources(t *testing.T) {
	inst := newInstance(&Component{}, 1, nil)
	rtA := &runtime.ResourceType{Impl: inst.rt}
	rtB := &runtime.ResourceType{Impl: inst.rt}
	inst.rt.ResourceTypes = append(inst.rt.ResourceTypes, rtA, rtB)

	idxA, err := inst.ResourceNew(types.ResourceIdx(0), uint32(10))
	require.NoError(t, err)
	idxB, err := inst.ResourceNew(types.ResourceIdx(1), uint32(20))
	require.NoError(t, err)

	// Each handle returns its own rep.
	repA, err := inst.ResourceRep(types.ResourceIdx(0), idxA)
	require.NoError(t, err)
	require.Equal(t, uint32(10), repA)

	repB, err := inst.ResourceRep(types.ResourceIdx(1), idxB)
	require.NoError(t, err)
	require.Equal(t, uint32(20), repB)

	// Cross-type rep lookup must fail (type mismatch).
	_, err = inst.ResourceRep(types.ResourceIdx(1), idxA)
	require.Error(t, err)
}

// TestCanonResourceRep asserts ResourceRep returns the correct rep for a
// valid handle.
//
// Spec: definitions.py:2169-2173 canon_resource_rep.
func TestCanonResourceRep(t *testing.T) {
	inst := newInstance(&Component{}, 1, nil)
	rt := &runtime.ResourceType{Impl: inst.rt}
	inst.rt.ResourceTypes = append(inst.rt.ResourceTypes, rt)

	idx, err := inst.ResourceNew(types.ResourceIdx(0), uint32(42))
	require.NoError(t, err)

	rep, err := inst.ResourceRep(types.ResourceIdx(0), idx)
	require.NoError(t, err)
	require.Equal(t, uint32(42), rep)
}

// TestCanonResourceRep_InvalidHandle asserts ResourceRep traps on an
// invalid (out-of-range) handle index.
//
// Spec: definitions.py:2170 trap_if(not isinstance(h, ResourceHandle)).
func TestCanonResourceRep_InvalidHandle(t *testing.T) {
	inst := newInstance(&Component{}, 1, nil)
	rt := &runtime.ResourceType{Impl: inst.rt}
	inst.rt.ResourceTypes = append(inst.rt.ResourceTypes, rt)

	_, err := inst.ResourceRep(types.ResourceIdx(0), 9999)
	require.Error(t, err)
}

// TestCanonResourceDrop_Owned asserts ResourceDrop removes an owned
// handle and invokes the destructor.
//
// Spec: definitions.py:2142-2165 canon_resource_drop (own branch).
func TestCanonResourceDrop_Owned(t *testing.T) {
	inst := newInstance(&Component{}, 1, nil)
	var dtorCalls int
	rt := &runtime.ResourceType{
		Impl: inst.rt,
		HostDestructor: func(rep uint32) error {
			dtorCalls++
			require.Equal(t, uint32(55), rep)
			return nil
		},
	}
	inst.rt.ResourceTypes = append(inst.rt.ResourceTypes, rt)

	idx, err := inst.ResourceNew(types.ResourceIdx(0), uint32(55))
	require.NoError(t, err)

	err = inst.ResourceDrop(types.ResourceIdx(0), idx)
	require.NoError(t, err)
	require.Equal(t, 1, dtorCalls)

	// After drop, ResourceRep should fail.
	_, err = inst.ResourceRep(types.ResourceIdx(0), idx)
	require.Error(t, err)
}

// TestCanonResourceDrop_NoDestructor asserts ResourceDrop succeeds for an
// owned handle whose resource type has no destructor.
//
// Spec: definitions.py:2151-2153 if rt.dtor: ... (no dtor → no-op).
func TestCanonResourceDrop_NoDestructor(t *testing.T) {
	inst := newInstance(&Component{}, 1, nil)
	rt := &runtime.ResourceType{Impl: inst.rt}
	inst.rt.ResourceTypes = append(inst.rt.ResourceTypes, rt)

	idx, err := inst.ResourceNew(types.ResourceIdx(0), uint32(77))
	require.NoError(t, err)

	err = inst.ResourceDrop(types.ResourceIdx(0), idx)
	require.NoError(t, err)

	// Handle is gone.
	_, err = inst.ResourceRep(types.ResourceIdx(0), idx)
	require.Error(t, err)
}

// TestCanonResourceDrop_DifferentResourceTypes asserts ResourceDrop traps
// when the handle's resource type doesn't match the requested type.
//
// Spec: definitions.py:2147 trap_if(h.rt is not rt).
func TestCanonResourceDrop_DifferentResourceTypes(t *testing.T) {
	inst := newInstance(&Component{}, 1, nil)
	rtA := &runtime.ResourceType{Impl: inst.rt}
	rtB := &runtime.ResourceType{Impl: inst.rt}
	inst.rt.ResourceTypes = append(inst.rt.ResourceTypes, rtA, rtB)

	idx, err := inst.ResourceNew(types.ResourceIdx(0), uint32(1))
	require.NoError(t, err)

	// Drop with rtB (index 1) but handle belongs to rtA (index 0).
	err = inst.ResourceDrop(types.ResourceIdx(1), idx)
	require.Error(t, err)
}

// TestCanonResourceDrop_Borrowed asserts ResourceDrop for a borrow handle
// does NOT invoke the destructor and decrements the borrow scope counter.
//
// Spec: definitions.py:2163-2164 canon_resource_drop borrow branch:
//
//	else:
//	  h.borrow_scope.num_borrows -= 1
func TestCanonResourceDrop_Borrowed(t *testing.T) {
	inst := newInstance(&Component{}, 1, nil)
	var dtorCalls int
	rt := &runtime.ResourceType{
		Impl: inst.rt,
		HostDestructor: func(rep uint32) error {
			dtorCalls++
			return nil
		},
	}
	inst.rt.ResourceTypes = append(inst.rt.ResourceTypes, rt)

	// Create a borrow handle directly in the table (simulating lift_borrow).
	scope := runtime.NewBorrowScope(inst.rt.Table)
	borrowFull, err := inst.rt.Table.NewResourceHandle(uint32(88), false, rt)
	require.NoError(t, err)

	// Associate the borrow with the scope.
	borrowEntry, err := inst.rt.Table.GetResourceHandle(borrowFull)
	require.NoError(t, err)
	borrowEntry.BorrowScope = scope
	scope.IncrementBorrows()

	require.Equal(t, 1, scope.NumBorrows())

	// Drop the borrow handle.
	err = inst.ResourceDrop(types.ResourceIdx(0), borrowFull.Index())
	require.NoError(t, err)

	// Destructor was NOT called (borrow branch).
	require.Equal(t, 0, dtorCalls)
	// Scope borrow counter was decremented.
	require.Equal(t, 0, scope.NumBorrows())
}

// TestCanonResourceDrop_BorrowedNoCallContext asserts ResourceDrop for a
// borrow handle with a nil BorrowScope does not panic.
//
// Spec: definitions.py:2163-2164 canon_resource_drop borrow branch.
func TestCanonResourceDrop_BorrowedNoCallContext(t *testing.T) {
	inst := newInstance(&Component{}, 1, nil)
	rt := &runtime.ResourceType{Impl: inst.rt}
	inst.rt.ResourceTypes = append(inst.rt.ResourceTypes, rt)

	// Create a borrow handle with no scope attached.
	borrowFull, err := inst.rt.Table.NewResourceHandle(uint32(33), false, rt)
	require.NoError(t, err)

	// Drop should succeed without panicking even with nil BorrowScope.
	err = inst.ResourceDrop(types.ResourceIdx(0), borrowFull.Index())
	require.NoError(t, err)
}

// TestCanonResourceDrop_InvalidHandle asserts ResourceDrop traps on an
// invalid (out-of-range) handle index.
//
// Spec: definitions.py:2145 h = inst.table.remove(i) — traps if invalid.
func TestCanonResourceDrop_InvalidHandle(t *testing.T) {
	inst := newInstance(&Component{}, 1, nil)
	rt := &runtime.ResourceType{Impl: inst.rt}
	inst.rt.ResourceTypes = append(inst.rt.ResourceTypes, rt)

	err := inst.ResourceDrop(types.ResourceIdx(0), 9999)
	require.Error(t, err)
}

// TestCanonResourceDrop_DoubleDrop asserts ResourceDrop traps when the
// same handle is dropped twice (use-after-free).
//
// Spec: definitions.py:2145 h = inst.table.remove(i) — second remove traps.
func TestCanonResourceDrop_DoubleDrop(t *testing.T) {
	inst := newInstance(&Component{}, 1, nil)
	rt := &runtime.ResourceType{Impl: inst.rt}
	inst.rt.ResourceTypes = append(inst.rt.ResourceTypes, rt)

	idx, err := inst.ResourceNew(types.ResourceIdx(0), uint32(11))
	require.NoError(t, err)

	err = inst.ResourceDrop(types.ResourceIdx(0), idx)
	require.NoError(t, err)

	// Second drop must fail.
	err = inst.ResourceDrop(types.ResourceIdx(0), idx)
	require.Error(t, err)
}

// TestInstance_SetCallContext is skipped because CallContext is not wired
// as Instance-level state. CallContext lives in the abi.LiftContext/
// LowerContext layer and is per-call, not per-instance.
func TestInstance_SetCallContext(t *testing.T) {
	t.Skip("Instance does not expose SetCallContext; CallContext is per-call state in abi.LiftContext/LowerContext")
}

// TestCanonResourceNew_IntRepresentation asserts that ResourceNew
// correctly round-trips a uint32 rep value. (Rep was changed from
// interface{} to uint32 in Task E2 per spec definitions.py:337-349.)
//
// Spec: definitions.py:2134-2138 canon_resource_new.
func TestCanonResourceNew_IntRepresentation(t *testing.T) {
	inst := newInstance(&Component{}, 1, nil)
	rt := &runtime.ResourceType{Impl: inst.rt}
	inst.rt.ResourceTypes = append(inst.rt.ResourceTypes, rt)

	// Test various uint32 values round-trip correctly.
	for _, rep := range []uint32{0, 1, 42, 0xFFFFFFFF} {
		idx, err := inst.ResourceNew(types.ResourceIdx(0), rep)
		require.NoError(t, err)

		got, err := inst.ResourceRep(types.ResourceIdx(0), idx)
		require.NoError(t, err)
		require.Equal(t, rep, got)
	}
}

// TestCanonResourceNew_StructRepresentation asserts that rep is uint32
// and does NOT hold arbitrary Go types. The old test expected Go structs
// to be stored as rep; per spec (definitions.py:337-349 ResourceHandle.rep)
// rep is always a u32. This test verifies the u32 contract by storing
// a "struct index" pattern where the host maps uint32 -> Go struct
// externally.
//
// Spec: definitions.py:2134-2138 canon_resource_new.
func TestCanonResourceNew_StructRepresentation(t *testing.T) {
	inst := newInstance(&Component{}, 1, nil)
	rt := &runtime.ResourceType{Impl: inst.rt}
	inst.rt.ResourceTypes = append(inst.rt.ResourceTypes, rt)

	// Simulate the host-side pattern: allocate a Go struct, assign it an
	// index (uint32), and use that index as the rep. The host maintains
	// the mapping externally.
	type myResource struct {
		Name  string
		Value int
	}
	registry := map[uint32]*myResource{
		0: {Name: "first", Value: 100},
		1: {Name: "second", Value: 200},
	}

	idx0, err := inst.ResourceNew(types.ResourceIdx(0), uint32(0))
	require.NoError(t, err)
	idx1, err := inst.ResourceNew(types.ResourceIdx(0), uint32(1))
	require.NoError(t, err)

	rep0, err := inst.ResourceRep(types.ResourceIdx(0), idx0)
	require.NoError(t, err)
	require.Equal(t, "first", registry[rep0].Name)

	rep1, err := inst.ResourceRep(types.ResourceIdx(0), idx1)
	require.NoError(t, err)
	require.Equal(t, "second", registry[rep1].Name)
}

// --- ExportedFunc resource-param tests (skipped: pipeline not wired) --------
//
// These tests exercise ExportedFunc.Call with Own/Borrow handle arguments.
// The full lift/lower pipeline for resource params through ExportedFunc is
// not wired yet — ExportedFunc.Call delegates to a HostFunc closure which
// requires a fully instantiated canon.lift/canon.lower pipeline.

func TestExportedFuncCall_OwnArgument(t *testing.T) {
	t.Skip("ExportedFunc.Call does not yet support Own handle params through the lift/lower pipeline")
}

func TestExportedFuncCall_BorrowArgument(t *testing.T) {
	t.Skip("ExportedFunc.Call does not yet support Borrow handle params through the lift/lower pipeline")
}

func TestExportedFuncCall_OwnResult(t *testing.T) {
	t.Skip("ExportedFunc.Call does not yet support Own handle results through the lift/lower pipeline")
}

func TestExportedFuncCall_OutstandingBorrowTrap(t *testing.T) {
	t.Skip("ExportedFunc.Call does not yet wire CallContext borrow tracking for return-gate validation")
}

func TestExportedFuncCall_BorrowDroppedBeforeReturn(t *testing.T) {
	t.Skip("ExportedFunc.Call does not yet wire CallContext borrow tracking for return-gate validation")
}

func TestExportedFuncCall_MultipleOwnBorrowParams(t *testing.T) {
	t.Skip("ExportedFunc.Call does not yet support mixed Own/Borrow handle params through the lift/lower pipeline")
}

func TestExportedFuncCall_CallContextRestored(t *testing.T) {
	t.Skip("ExportedFunc.Call does not yet wire per-call CallContext save/restore across nested calls")
}

func TestLiftPrimitiveVal_F32_BitPattern(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

func TestLiftPrimitiveVal_F64_BitPattern(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

func TestLiftResolvedPrimitiveVal_F32_BitPattern(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

func TestLiftResolvedPrimitiveVal_F64_BitPattern(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

func TestExportedFunc_Call_ListWithRealloc(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

func TestExportedFunc_Call_ListWithoutRealloc(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

func TestExportedFunc_Call_EmptyList(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

func TestExportedFunc_Call_ListOfS64(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

func TestExportedFunc_Call_ListOfF32(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

func TestExportedFunc_Call_ListOfF64(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

func TestExportedFunc_Call_ListOfU8(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

func TestExportedFunc_Call_ListOfS8(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

func TestExportedFunc_Call_ListOfU16(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

func TestExportedFunc_Call_ListOfS16(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

func TestExportedFunc_Call_ListOfU64(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

func TestExportedFunc_Call_ListOfBool(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

func TestLiftEnum(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

func TestLiftEnum_InvalidDiscriminant(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

func TestLiftFlags(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

func TestLiftVariant(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

func TestLiftVariant_MultiplePayloadTypes(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

func TestExportedFunc_Call_ListOfChar(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

func TestInstance_ValueIndexSpace(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

func TestInstance_ConsumeValue(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

func TestInstance_ParentChild(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

func TestInstance_ParentChain(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

func TestInstance_GetAncestor(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

func TestInstance_IndexSpaces(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

func TestInstance_IndexSpaces_Component(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

func TestInstance_IndexSpaces_OutOfBounds(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

func TestInstance_ExportedInstances(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

func TestLiftResolvedType_RecordRetptr(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

func TestLiftResolvedType_RecordFlat(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

func TestLiftResolvedType_LargeRecordRetptr(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

func TestLiftFieldFromMemory_AllPrimitiveTypes(t *testing.T) {
	t.Skip(instanceTestSkipMsg)
}

// --- Session 1 Task B3 tests ---------------------------------------------
//
// These exercise the new shape in which Instance embeds a
// *runtime.ComponentInstance and delegates spec-level state to it.

// TestInstanceEmbedsRuntimeComponentInstance asserts Instance carries
// a *runtime.ComponentInstance and delegates spec-level state.
//
// Spec: definitions.py:256-273 class ComponentInstance.
// Wasmtime parallel: runtime/component/instance.rs:710-743 (Instantiator).
func TestInstanceEmbedsRuntimeComponentInstance(t *testing.T) {
	c := &Component{}
	inst := newInstance(c, 0, nil)
	require.NotNil(t, inst.Runtime())
	// Spec: definitions.py:260 may_leave defaults true.
	require.True(t, inst.MayLeave())
	require.Equal(t, 0, inst.ActiveCallDepth())
}

// TestInstanceMayLeaveDelegatesToRuntime asserts MayLeave/SetMayLeave
// read/write runtime state directly, not a duplicate wrapper field.
//
// Spec: definitions.py:260 may_leave field.
func TestInstanceMayLeaveDelegatesToRuntime(t *testing.T) {
	inst := newInstance(&Component{}, 0, nil)
	inst.SetMayLeave(false)
	require.False(t, inst.Runtime().MayLeave)
	inst.SetMayLeave(true)
	require.True(t, inst.Runtime().MayLeave)
}

// TestInstanceCallMightBeRecursiveUsesReentranceTracker asserts the
// wrapper's CallMightBeRecursive implements the spec's reflexive-ancestor
// overlap check via a structural parent-chain walk.
//
// (Name retained from B3 for plan traceability; the B4 corrective
// replaced the tracker-consultation implementation with a structural
// walk because the tracker models runtime-stack membership rather than
// structural ancestry. Spec: definitions.py:290-299 call_might_be_recursive.)
func TestInstanceCallMightBeRecursiveUsesReentranceTracker(t *testing.T) {
	a := newInstance(&Component{}, 1, nil)
	b := newInstance(&Component{}, 2, nil)

	// Sibling instances with no parent relationship have disjoint
	// reflexive_ancestors sets — not recursive regardless of active calls.
	require.False(t, a.CallMightBeRecursive(b))

	// Same instance is its own reflexive ancestor, so calling a from a
	// is trivially recursive. Active-call state is irrelevant.
	a.EnterCall()
	defer a.ExitCall()
	require.True(t, a.CallMightBeRecursive(a))
}
