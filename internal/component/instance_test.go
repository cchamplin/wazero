// internal/component/instance_test.go
//
// Session 1 Task E12: restore resource-related tests from Session 0 stubs.
// Session 1 Task F3: restore remaining non-resource lift/lower + instance
// structure tests.
//
// Spec: definitions.py:2134-2173 canon_resource_new/drop/rep.
// Spec: definitions.py:256-273 class ComponentInstance (instance structure).
// Spec: definitions.py:1461-1469 lift_flat (NaN canonicalization).
//
// Session 1 Task B3 adds the TestInstance{EmbedsRuntimeComponentInstance,
// MayLeaveDelegatesToRuntime, CallMightBeRecursiveUsesReentranceTracker}
// tests at the bottom of this file.
package component

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"testing"

	"github.com/tetratelabs/wazero/experimental/wazerotest"
	"github.com/tetratelabs/wazero/internal/component/abi"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

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
	callCtx := runtime.NewCallContext(inst.rt.Table)
	borrowFull, err := inst.rt.Table.NewResourceHandle(uint32(88), false, rt)
	require.NoError(t, err)

	// Associate the borrow with the call context.
	borrowEntry, err := inst.rt.Table.GetResourceHandle(borrowFull)
	require.NoError(t, err)
	borrowEntry.CallContext = callCtx
	callCtx.IncrementBorrows()

	require.Equal(t, 1, callCtx.NumBorrows())

	// Drop the borrow handle.
	err = inst.ResourceDrop(types.ResourceIdx(0), borrowFull.Index())
	require.NoError(t, err)

	// Destructor was NOT called (borrow branch).
	require.Equal(t, 0, dtorCalls)
	// Call context borrow counter was decremented.
	require.Equal(t, 0, callCtx.NumBorrows())
}

// TestCanonResourceDrop_BorrowedNoCallContext asserts ResourceDrop for a
// borrow handle with a nil CallContext does not panic.
//
// Spec: definitions.py:2163-2164 canon_resource_drop borrow branch.
func TestCanonResourceDrop_BorrowedNoCallContext(t *testing.T) {
	inst := newInstance(&Component{}, 1, nil)
	rt := &runtime.ResourceType{Impl: inst.rt}
	inst.rt.ResourceTypes = append(inst.rt.ResourceTypes, rt)

	// Create a borrow handle with no call context attached.
	borrowFull, err := inst.rt.Table.NewResourceHandle(uint32(33), false, rt)
	require.NoError(t, err)

	// Drop should succeed without panicking even with nil CallContext.
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

// --- ExportedFunc resource-param tests ----------------------------------------
//
// These tests exercise ExportedFunc.Call with Own/Borrow handle arguments
// through the abi.LowerParams/LiftResults pipeline. Each test creates an
// ExportedFunc whose impl simulates what buildCanonLiftFunc does: constructs
// LowerContext/LiftContext, lowers the args, invokes a mock core function,
// lifts the results, and manages the CallContext lifecycle.

// makeResourceTestTypes creates ComponentTypes with one concrete resource
// table entry (instance 0, resource 0), plus a runtime ComponentInstance
// with the corresponding ResourceType.
func makeResourceTestTypes() (*types.ComponentTypes, *runtime.ComponentInstance, *runtime.ResourceType) {
	rt := &runtime.ResourceType{}
	inst := runtime.NewComponentInstance(0, nil)
	inst.ResourceTypes = []*runtime.ResourceType{rt}

	ct := &types.ComponentTypes{
		ResourceTables: []types.TypeResourceTable{
			{Concrete: true, Instance: 0, Resource: 0},
		},
	}
	return ct, inst, rt
}

func TestExportedFuncCall_OwnArgument(t *testing.T) {
	ct, rtInst, rt := makeResourceTestTypes()
	mem := wazerotest.NewMemory(wazerotest.PageSize)

	// Pre-populate an own handle in the table for the argument to reference.
	ownH, err := rtInst.Table.NewResourceHandle(uint32(42), true, rt)
	require.NoError(t, err)

	ownType := types.ValType{Kind: types.TypeKindOwn, Index: 0}
	paramTypes := []types.ValType{ownType}

	ef := &ExportedFunc{
		name:     "accept-own",
		funcType: &types.TypeFunc{},
		impl: func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			callCtx := runtime.NewCallContext(rtInst.Table)
			lowerCtx := &abi.LowerContext{
				Memory:      mem,
				Types:       ct,
				Instance:    rtInst,
				CallContext: callCtx,
			}
			flat, err := abi.LowerParams(lowerCtx, paramTypes, args, abi.MaxFlatParams)
			if err != nil {
				return nil, err
			}
			// Verify the handle index was lowered.
			require.Equal(t, 1, len(flat))
			// The lowered own handle creates a NEW entry. Verify by lifting
			// it back: lift_own removes the handle and returns rep.
			liftCtx := &abi.LiftContext{
				Memory:      mem,
				Types:       ct,
				Instance:    rtInst,
				CallContext: callCtx,
			}
			lifted, err := abi.LiftParams(liftCtx, paramTypes, flat, abi.MaxFlatParams)
			if err != nil {
				return nil, err
			}
			// The lifted own value should carry the original rep (42).
			require.Equal(t, uint32(42), lifted[0].Own())
			return nil, callCtx.Release()
		},
	}

	// Call with an Own handle carrying rep=42. The val carries the rep value
	// that lowerOwnHandleFlat will use to create a new handle in the table.
	// First we need to remove the original handle to avoid it being stale.
	// Actually lowerOwnHandleFlat expects val.Own() to return the rep, and
	// it creates a new handle from that. So we pass ValOwn(42).
	// But lift_own removes the handle. The original handle ownH was created
	// for the purpose of having something in the table to reference. Lower
	// creates a new handle from the rep. So the test passes rep=42 through
	// lower->core->lift roundtrip.
	_, err = rtInst.Table.Remove(ownH) // clean up the pre-populated handle
	require.NoError(t, err)
	results, err := ef.Call(context.Background(), types.ValOwn(42))
	require.NoError(t, err)
	require.Nil(t, results)
}

func TestExportedFuncCall_BorrowArgument(t *testing.T) {
	ct, rtInst, rt := makeResourceTestTypes()
	mem := wazerotest.NewMemory(wazerotest.PageSize)

	// For borrow lowering, when the caller instance IS the defining instance,
	// the same-instance optimization returns rep directly (no table entry).
	// Set rt.Impl = rtInst to trigger this optimization.
	rt.Impl = rtInst

	borrowType := types.ValType{Kind: types.TypeKindBorrow, Index: 0}
	paramTypes := []types.ValType{borrowType}

	ef := &ExportedFunc{
		name:     "accept-borrow",
		funcType: &types.TypeFunc{},
		impl: func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			callCtx := runtime.NewCallContext(rtInst.Table)
			lowerCtx := &abi.LowerContext{
				Memory:      mem,
				Types:       ct,
				Instance:    rtInst,
				CallContext: callCtx,
			}
			flat, err := abi.LowerParams(lowerCtx, paramTypes, args, abi.MaxFlatParams)
			if err != nil {
				return nil, err
			}
			// Same-instance optimization: flat[0] is the rep directly (99).
			require.Equal(t, uint64(99), flat[0])
			return nil, callCtx.Release()
		},
	}

	results, err := ef.Call(context.Background(), types.ValBorrow(99))
	require.NoError(t, err)
	require.Nil(t, results)
}

func TestExportedFuncCall_OwnResult(t *testing.T) {
	ct, rtInst, _ := makeResourceTestTypes()
	mem := wazerotest.NewMemory(wazerotest.PageSize)

	ownType := types.ValType{Kind: types.TypeKindOwn, Index: 0}
	resultTypes := []types.ValType{ownType}

	ef := &ExportedFunc{
		name:     "produce-own",
		funcType: &types.TypeFunc{},
		impl: func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			callCtx := runtime.NewCallContext(rtInst.Table)
			// Simulate a core function that returns a handle index.
			// First create an own handle in the table.
			_, err := rtInst.Table.NewResourceHandle(uint32(77), true, rtInst.ResourceTypes[0])
			if err != nil {
				return nil, err
			}
			// The core function returns the handle index (0).
			flatResults := []uint64{0}

			liftCtx := &abi.LiftContext{
				Memory:      mem,
				Types:       ct,
				Instance:    rtInst,
				CallContext: callCtx,
			}
			lifted, err := abi.LiftResults(liftCtx, resultTypes, flatResults, abi.MaxFlatResults)
			if err != nil {
				_ = callCtx.Release()
				return nil, err
			}
			_ = callCtx.Release()
			return lifted, nil
		},
	}

	results, err := ef.Call(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, uint32(77), results[0].Own())
}

func TestExportedFuncCall_OutstandingBorrowTrap(t *testing.T) {
	ct, rtInst, rt := makeResourceTestTypes()
	mem := wazerotest.NewMemory(wazerotest.PageSize)

	// Pre-populate an own handle in the table for cross-instance borrow.
	_, err := rtInst.Table.NewResourceHandle(uint32(55), true, rt)
	require.NoError(t, err)

	borrowType := types.ValType{Kind: types.TypeKindBorrow, Index: 0}
	paramTypes := []types.ValType{borrowType}

	ef := &ExportedFunc{
		name:     "borrow-no-drop",
		funcType: &types.TypeFunc{},
		impl: func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			callCtx := runtime.NewCallContext(rtInst.Table)

			// Use a second instance (child of rtInst) so the same-instance
			// optimization is NOT triggered — this forces a real borrow
			// handle allocation. The parent chain allows LookupResourceType
			// to find instance 0's ResourceTypes.
			borrowerInst := runtime.NewComponentInstance(1, rtInst)

			lowerCtx := &abi.LowerContext{
				Memory:      mem,
				Types:       ct,
				Instance:    borrowerInst,
				CallContext: callCtx,
			}
			_, err := abi.LowerParams(lowerCtx, paramTypes, args, abi.MaxFlatParams)
			if err != nil {
				return nil, err
			}

			// The borrow was created but NOT dropped — callCtx has an
			// outstanding borrow. Attempting to validate return should fail.
			err = callCtx.ValidateReturn()
			require.Error(t, err)
			require.Contains(t, err.Error(), "borrow")

			// Clean up for the test (release the borrow scope).
			_ = callCtx.Release()
			return nil, nil
		},
	}

	_, err = ef.Call(context.Background(), types.ValBorrow(55))
	require.NoError(t, err)
}

func TestExportedFuncCall_BorrowDroppedBeforeReturn(t *testing.T) {
	ct, rtInst, rt := makeResourceTestTypes()
	mem := wazerotest.NewMemory(wazerotest.PageSize)

	// Pre-populate an own handle for borrow source.
	_, err := rtInst.Table.NewResourceHandle(uint32(66), true, rt)
	require.NoError(t, err)

	borrowType := types.ValType{Kind: types.TypeKindBorrow, Index: 0}
	paramTypes := []types.ValType{borrowType}

	ef := &ExportedFunc{
		name:     "borrow-then-drop",
		funcType: &types.TypeFunc{},
		impl: func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			callCtx := runtime.NewCallContext(rtInst.Table)

			// Second instance (child of rtInst) to trigger real borrow
			// handle allocation. Parent chain enables LookupResourceType.
			borrowerInst := runtime.NewComponentInstance(1, rtInst)

			lowerCtx := &abi.LowerContext{
				Memory:      mem,
				Types:       ct,
				Instance:    borrowerInst,
				CallContext: callCtx,
			}
			flat, err := abi.LowerParams(lowerCtx, paramTypes, args, abi.MaxFlatParams)
			if err != nil {
				return nil, err
			}

			// The borrow handle index is flat[0].
			borrowIdx := uint32(flat[0])

			// Drop the borrow handle to clear the borrow count.
			borrowH, entry, gerr := borrowerInst.Table.GetByIndex(borrowIdx)
			require.NoError(t, gerr)
			resEntry, ok := entry.(*runtime.ResourceHandleEntry)
			require.True(t, ok)
			require.NotNil(t, resEntry.CallContext)
			resEntry.CallContext.DecrementBorrows()

			// Remove from table.
			_, rerr := borrowerInst.Table.Remove(borrowH)
			require.NoError(t, rerr)

			// Now return should be allowed.
			err = callCtx.ValidateReturn()
			require.NoError(t, err)

			_ = callCtx.Release()
			return nil, nil
		},
	}

	_, err = ef.Call(context.Background(), types.ValBorrow(66))
	require.NoError(t, err)
}

func TestExportedFuncCall_MultipleOwnBorrowParams(t *testing.T) {
	ct, rtInst, rt := makeResourceTestTypes()
	mem := wazerotest.NewMemory(wazerotest.PageSize)

	// Same-instance optimization for borrows.
	rt.Impl = rtInst

	ownType := types.ValType{Kind: types.TypeKindOwn, Index: 0}
	borrowType := types.ValType{Kind: types.TypeKindBorrow, Index: 0}
	paramTypes := []types.ValType{ownType, borrowType}

	ef := &ExportedFunc{
		name:     "mixed-handles",
		funcType: &types.TypeFunc{},
		impl: func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			callCtx := runtime.NewCallContext(rtInst.Table)
			lowerCtx := &abi.LowerContext{
				Memory:      mem,
				Types:       ct,
				Instance:    rtInst,
				CallContext: callCtx,
			}
			flat, err := abi.LowerParams(lowerCtx, paramTypes, args, abi.MaxFlatParams)
			if err != nil {
				return nil, err
			}
			// Own param creates a handle (index in flat[0]).
			// Borrow param with same-instance optimization returns rep in flat[1].
			require.Equal(t, 2, len(flat))
			// Borrow rep should be 200 (same-instance returns rep directly).
			require.Equal(t, uint64(200), flat[1])

			// Lift the own handle back.
			liftCtx := &abi.LiftContext{
				Memory:      mem,
				Types:       ct,
				Instance:    rtInst,
				CallContext: callCtx,
			}
			ownLifted, err := abi.LiftFlat(liftCtx, ownType, abi.NewFlatIter(flat[:1]))
			require.NoError(t, err)
			require.Equal(t, uint32(100), ownLifted.Own())

			_ = callCtx.Release()
			return nil, nil
		},
	}

	_, err := ef.Call(context.Background(), types.ValOwn(100), types.ValBorrow(200))
	require.NoError(t, err)
}

func TestExportedFuncCall_CallContextRestored(t *testing.T) {
	ct, rtInst, rt := makeResourceTestTypes()
	mem := wazerotest.NewMemory(wazerotest.PageSize)
	rt.Impl = rtInst

	borrowType := types.ValType{Kind: types.TypeKindBorrow, Index: 0}
	paramTypes := []types.ValType{borrowType}

	callCount := 0
	ef := &ExportedFunc{
		name:     "nested-calls",
		funcType: &types.TypeFunc{},
		impl: func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			callCount++
			// Each call gets its own CallContext.
			callCtx := runtime.NewCallContext(rtInst.Table)
			lowerCtx := &abi.LowerContext{
				Memory:      mem,
				Types:       ct,
				Instance:    rtInst,
				CallContext: callCtx,
			}
			flat, err := abi.LowerParams(lowerCtx, paramTypes, args, abi.MaxFlatParams)
			if err != nil {
				return nil, err
			}
			// Verify the borrow was lowered (same-instance optimization).
			require.Equal(t, uint64(42), flat[0])

			// Verify each call's CallContext is independent.
			require.Equal(t, 0, callCtx.NumBorrows())

			_ = callCtx.Release()
			return []types.Val{types.ValU32(uint32(callCount))}, nil
		},
	}

	// First call.
	results, err := ef.Call(context.Background(), types.ValBorrow(42))
	require.NoError(t, err)
	require.Equal(t, uint32(1), results[0].U32())

	// Second call — verify call context was not leaked from first call.
	results, err = ef.Call(context.Background(), types.ValBorrow(42))
	require.NoError(t, err)
	require.Equal(t, uint32(2), results[0].U32())
}

// TestLiftPrimitiveVal_F32_BitPattern asserts that lifting an f32 from
// flat representation canonicalizes NaN bit patterns per the spec.
//
// Spec: definitions.py:1461-1469 lift_flat, case F32 applies
// canonicalize_nan32. All NaN payloads collapse to 0x7fc00000.
func TestLiftPrimitiveVal_F32_BitPattern(t *testing.T) {
	// Normal value round-trips unchanged.
	bits := math.Float32bits(3.14)
	val, err := abi.LiftFlat(nil, types.F32, abi.NewFlatIter([]uint64{uint64(bits)}))
	require.NoError(t, err)
	require.Equal(t, bits, math.Float32bits(val.F32()))

	// Signalling NaN is canonicalized.
	sNaN := uint32(0x7f800001) // signalling NaN
	val, err = abi.LiftFlat(nil, types.F32, abi.NewFlatIter([]uint64{uint64(sNaN)}))
	require.NoError(t, err)
	require.Equal(t, abi.CanonicalFloat32NaN, math.Float32bits(val.F32()))

	// Quiet NaN with non-canonical payload is canonicalized.
	qNaN := uint32(0x7fc00001) // quiet NaN, non-canonical payload
	val, err = abi.LiftFlat(nil, types.F32, abi.NewFlatIter([]uint64{uint64(qNaN)}))
	require.NoError(t, err)
	require.Equal(t, abi.CanonicalFloat32NaN, math.Float32bits(val.F32()))
}

// TestLiftPrimitiveVal_F64_BitPattern asserts that lifting an f64 from
// flat representation canonicalizes NaN bit patterns per the spec.
//
// Spec: definitions.py:1461-1469 lift_flat, case F64 applies
// canonicalize_nan64. All NaN payloads collapse to 0x7ff8000000000000.
func TestLiftPrimitiveVal_F64_BitPattern(t *testing.T) {
	// Normal value round-trips unchanged.
	bits := math.Float64bits(2.718281828)
	val, err := abi.LiftFlat(nil, types.F64, abi.NewFlatIter([]uint64{bits}))
	require.NoError(t, err)
	require.Equal(t, bits, math.Float64bits(val.F64()))

	// Signalling NaN is canonicalized.
	sNaN := uint64(0x7ff0000000000001)
	val, err = abi.LiftFlat(nil, types.F64, abi.NewFlatIter([]uint64{sNaN}))
	require.NoError(t, err)
	require.Equal(t, abi.CanonicalFloat64NaN, math.Float64bits(val.F64()))

	// Quiet NaN with non-canonical payload is canonicalized.
	qNaN := uint64(0x7ff8000000000001)
	val, err = abi.LiftFlat(nil, types.F64, abi.NewFlatIter([]uint64{qNaN}))
	require.NoError(t, err)
	require.Equal(t, abi.CanonicalFloat64NaN, math.Float64bits(val.F64()))
}

// TestLiftResolvedPrimitiveVal_F32_BitPattern asserts that lifting an f32
// from heap memory canonicalizes NaN bit patterns per the spec.
//
// Spec: definitions.py:1326-1333 load, case F32 applies canonicalize_nan32.
func TestLiftResolvedPrimitiveVal_F32_BitPattern(t *testing.T) {
	mem := wazerotest.NewMemory(wazerotest.PageSize)

	// Normal value round-trips unchanged.
	binary.LittleEndian.PutUint32(mem.Bytes[0:], math.Float32bits(1.5))
	ctx := &abi.LiftContext{Memory: mem}
	val, err := abi.LiftHeap(ctx, types.F32, 0)
	require.NoError(t, err)
	require.Equal(t, math.Float32bits(float32(1.5)), math.Float32bits(val.F32()))

	// Signalling NaN is canonicalized.
	binary.LittleEndian.PutUint32(mem.Bytes[0:], 0x7f800001)
	val, err = abi.LiftHeap(ctx, types.F32, 0)
	require.NoError(t, err)
	require.Equal(t, abi.CanonicalFloat32NaN, math.Float32bits(val.F32()))
}

// TestLiftResolvedPrimitiveVal_F64_BitPattern asserts that lifting an f64
// from heap memory canonicalizes NaN bit patterns per the spec.
//
// Spec: definitions.py:1326-1333 load, case F64 applies canonicalize_nan64.
func TestLiftResolvedPrimitiveVal_F64_BitPattern(t *testing.T) {
	mem := wazerotest.NewMemory(wazerotest.PageSize)

	// Normal value round-trips unchanged.
	binary.LittleEndian.PutUint64(mem.Bytes[0:], math.Float64bits(2.5))
	ctx := &abi.LiftContext{Memory: mem}
	val, err := abi.LiftHeap(ctx, types.F64, 0)
	require.NoError(t, err)
	require.Equal(t, math.Float64bits(2.5), math.Float64bits(val.F64()))

	// Signalling NaN is canonicalized.
	binary.LittleEndian.PutUint64(mem.Bytes[0:], 0x7ff0000000000001)
	val, err = abi.LiftHeap(ctx, types.F64, 0)
	require.NoError(t, err)
	require.Equal(t, abi.CanonicalFloat64NaN, math.Float64bits(val.F64()))
}

// --- ExportedFunc.Call list-param tests ----------------------------------------
//
// These tests exercise ExportedFunc.Call with list-typed parameters through
// the abi.LowerParams/LiftResults pipeline. Each test creates an ExportedFunc
// whose impl simulates what buildCanonLiftFunc does: constructs LowerContext
// with realloc and memory, lowers args, then lifts them back via LiftParams
// to verify the round-trip.

// makeListTestHelper creates the common test infrastructure for list param
// round-trip tests. Returns an ExportedFunc whose impl lowers the args into
// memory via LowerParams, then lifts them back via LiftParams and returns
// the lifted results for verification.
func makeListTestExportedFunc(t *testing.T, ct *types.ComponentTypes, paramTypes []types.ValType) *ExportedFunc {
	t.Helper()
	mem := wazerotest.NewMemory(wazerotest.PageSize)
	inst := runtime.NewComponentInstance(0, nil)

	// Track the next free offset for realloc.
	nextAlloc := uint32(256) // start allocations at offset 256
	realloc := func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
		ptr := types.AlignTo(nextAlloc, align)
		nextAlloc = ptr + newSize
		return ptr, nil
	}

	return &ExportedFunc{
		name:     "list-roundtrip",
		funcType: &types.TypeFunc{},
		impl: func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			callCtx := runtime.NewCallContext(inst.Table)
			lowerCtx := &abi.LowerContext{
				Memory:      mem,
				Realloc:     realloc,
				Types:       ct,
				Instance:    inst,
				CallContext: callCtx,
			}
			flat, err := abi.LowerParams(lowerCtx, paramTypes, args, abi.MaxFlatParams)
			if err != nil {
				return nil, err
			}

			// Lift the params back from the flat representation.
			liftCtx := &abi.LiftContext{
				Memory:      mem,
				Types:       ct,
				Instance:    inst,
				CallContext: callCtx,
			}
			lifted, err := abi.LiftParams(liftCtx, paramTypes, flat, abi.MaxFlatParams)
			if err != nil {
				return nil, err
			}

			_ = callCtx.Release()
			return lifted, nil
		},
	}
}

func TestExportedFunc_Call_ListWithRealloc(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	listT := b.InternList(types.S32)
	ct := b.Finish()
	paramTypes := []types.ValType{listT}

	ef := makeListTestExportedFunc(t, ct, paramTypes)

	results, err := ef.Call(context.Background(), types.ValList([]types.Val{
		types.ValS32(10), types.ValS32(20), types.ValS32(30),
	}))
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	elems := results[0].List()
	require.Equal(t, 3, len(elems))
	require.Equal(t, int32(10), elems[0].S32())
	require.Equal(t, int32(20), elems[1].S32())
	require.Equal(t, int32(30), elems[2].S32())
}

func TestExportedFunc_Call_EmptyListNeedsRealloc(t *testing.T) {
	// Per canonical ABI spec: realloc is always called, even for empty lists.
	// The returned pointer must be aligned and within memory bounds.
	b := types.NewComponentTypesBuilder()
	listT := b.InternList(types.U32)
	ct := b.Finish()
	paramTypes := []types.ValType{listT}

	mem := wazerotest.NewMemory(wazerotest.PageSize)
	inst := runtime.NewComponentInstance(0, nil)

	ef := &ExportedFunc{
		name:     "empty-list-with-realloc",
		funcType: &types.TypeFunc{},
		impl: func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			callCtx := runtime.NewCallContext(inst.Table)
			lowerCtx := &abi.LowerContext{
				Memory:      mem,
				Types:       ct,
				Instance:    inst,
				CallContext: callCtx,
				Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
					return 0, nil // return valid aligned pointer
				},
			}
			flat, err := abi.LowerParams(lowerCtx, paramTypes, args, abi.MaxFlatParams)
			if err != nil {
				return nil, err
			}
			// flat should be [0, 0] (ptr=0, len=0).
			require.Equal(t, 2, len(flat))
			require.Equal(t, uint64(0), flat[0])
			require.Equal(t, uint64(0), flat[1])

			liftCtx := &abi.LiftContext{
				Memory:      mem,
				Types:       ct,
				Instance:    inst,
				CallContext: callCtx,
			}
			lifted, err := abi.LiftParams(liftCtx, paramTypes, flat, abi.MaxFlatParams)
			if err != nil {
				return nil, err
			}
			_ = callCtx.Release()
			return lifted, nil
		},
	}

	results, err := ef.Call(context.Background(), types.ValList([]types.Val{}))
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, 0, len(results[0].List()))
}

func TestExportedFunc_Call_EmptyList(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	listT := b.InternList(types.S32)
	ct := b.Finish()
	paramTypes := []types.ValType{listT}

	ef := makeListTestExportedFunc(t, ct, paramTypes)

	results, err := ef.Call(context.Background(), types.ValList([]types.Val{}))
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, 0, len(results[0].List()))
}

func TestExportedFunc_Call_ListOfS64(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	listT := b.InternList(types.S64)
	ct := b.Finish()
	paramTypes := []types.ValType{listT}

	ef := makeListTestExportedFunc(t, ct, paramTypes)

	results, err := ef.Call(context.Background(), types.ValList([]types.Val{
		types.ValS64(-100), types.ValS64(9876543210),
	}))
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	elems := results[0].List()
	require.Equal(t, 2, len(elems))
	require.Equal(t, int64(-100), elems[0].S64())
	require.Equal(t, int64(9876543210), elems[1].S64())
}

func TestExportedFunc_Call_ListOfF32(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	listT := b.InternList(types.F32)
	ct := b.Finish()
	paramTypes := []types.ValType{listT}

	ef := makeListTestExportedFunc(t, ct, paramTypes)

	results, err := ef.Call(context.Background(), types.ValList([]types.Val{
		types.ValF32(1.5), types.ValF32(-2.75),
	}))
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	elems := results[0].List()
	require.Equal(t, 2, len(elems))
	require.Equal(t, float32(1.5), elems[0].F32())
	require.Equal(t, float32(-2.75), elems[1].F32())
}

func TestExportedFunc_Call_ListOfF64(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	listT := b.InternList(types.F64)
	ct := b.Finish()
	paramTypes := []types.ValType{listT}

	ef := makeListTestExportedFunc(t, ct, paramTypes)

	results, err := ef.Call(context.Background(), types.ValList([]types.Val{
		types.ValF64(3.14159), types.ValF64(-2.71828),
	}))
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	elems := results[0].List()
	require.Equal(t, 2, len(elems))
	require.Equal(t, 3.14159, elems[0].F64())
	require.Equal(t, -2.71828, elems[1].F64())
}

func TestExportedFunc_Call_ListOfU8(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	listT := b.InternList(types.U8)
	ct := b.Finish()
	paramTypes := []types.ValType{listT}

	ef := makeListTestExportedFunc(t, ct, paramTypes)

	results, err := ef.Call(context.Background(), types.ValList([]types.Val{
		types.ValU8(0), types.ValU8(127), types.ValU8(255),
	}))
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	elems := results[0].List()
	require.Equal(t, 3, len(elems))
	require.Equal(t, uint8(0), elems[0].U8())
	require.Equal(t, uint8(127), elems[1].U8())
	require.Equal(t, uint8(255), elems[2].U8())
}

func TestExportedFunc_Call_ListOfS8(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	listT := b.InternList(types.S8)
	ct := b.Finish()
	paramTypes := []types.ValType{listT}

	ef := makeListTestExportedFunc(t, ct, paramTypes)

	results, err := ef.Call(context.Background(), types.ValList([]types.Val{
		types.ValS8(-128), types.ValS8(0), types.ValS8(127),
	}))
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	elems := results[0].List()
	require.Equal(t, 3, len(elems))
	require.Equal(t, int8(-128), elems[0].S8())
	require.Equal(t, int8(0), elems[1].S8())
	require.Equal(t, int8(127), elems[2].S8())
}

func TestExportedFunc_Call_ListOfU16(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	listT := b.InternList(types.U16)
	ct := b.Finish()
	paramTypes := []types.ValType{listT}

	ef := makeListTestExportedFunc(t, ct, paramTypes)

	results, err := ef.Call(context.Background(), types.ValList([]types.Val{
		types.ValU16(0), types.ValU16(1000), types.ValU16(65535),
	}))
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	elems := results[0].List()
	require.Equal(t, 3, len(elems))
	require.Equal(t, uint16(0), elems[0].U16())
	require.Equal(t, uint16(1000), elems[1].U16())
	require.Equal(t, uint16(65535), elems[2].U16())
}

func TestExportedFunc_Call_ListOfS16(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	listT := b.InternList(types.S16)
	ct := b.Finish()
	paramTypes := []types.ValType{listT}

	ef := makeListTestExportedFunc(t, ct, paramTypes)

	results, err := ef.Call(context.Background(), types.ValList([]types.Val{
		types.ValS16(-32768), types.ValS16(0), types.ValS16(32767),
	}))
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	elems := results[0].List()
	require.Equal(t, 3, len(elems))
	require.Equal(t, int16(-32768), elems[0].S16())
	require.Equal(t, int16(0), elems[1].S16())
	require.Equal(t, int16(32767), elems[2].S16())
}

func TestExportedFunc_Call_ListOfU64(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	listT := b.InternList(types.U64)
	ct := b.Finish()
	paramTypes := []types.ValType{listT}

	ef := makeListTestExportedFunc(t, ct, paramTypes)

	results, err := ef.Call(context.Background(), types.ValList([]types.Val{
		types.ValU64(0), types.ValU64(0xDEADBEEFCAFEBABE),
	}))
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	elems := results[0].List()
	require.Equal(t, 2, len(elems))
	require.Equal(t, uint64(0), elems[0].U64())
	require.Equal(t, uint64(0xDEADBEEFCAFEBABE), elems[1].U64())
}

func TestExportedFunc_Call_ListOfBool(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	listT := b.InternList(types.Bool)
	ct := b.Finish()
	paramTypes := []types.ValType{listT}

	ef := makeListTestExportedFunc(t, ct, paramTypes)

	results, err := ef.Call(context.Background(), types.ValList([]types.Val{
		types.ValBool(true), types.ValBool(false), types.ValBool(true),
	}))
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	elems := results[0].List()
	require.Equal(t, 3, len(elems))
	require.True(t, elems[0].Bool())
	require.False(t, elems[1].Bool())
	require.True(t, elems[2].Bool())
}

// TestLiftEnum asserts that LiftFlat correctly decodes an enum discriminant
// to the corresponding case name.
//
// Spec: definitions.py:163-165 EnumType; lift_flat at :1506-1510.
func TestLiftEnum(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	enumT := b.InternEnum([]string{"red", "green", "blue"})
	ct := b.Finish()

	ctx := &abi.LiftContext{Types: ct}

	// Discriminant 0 -> "red"
	val, err := abi.LiftFlat(ctx, enumT, abi.NewFlatIter([]uint64{0}))
	require.NoError(t, err)
	require.Equal(t, "red", val.Enum())

	// Discriminant 2 -> "blue"
	val, err = abi.LiftFlat(ctx, enumT, abi.NewFlatIter([]uint64{2}))
	require.NoError(t, err)
	require.Equal(t, "blue", val.Enum())
}

// TestLiftEnum_InvalidDiscriminant asserts that LiftFlat traps when an
// enum discriminant is out of range.
//
// Spec: definitions.py:1506-1510 lift_flat enum — trap on invalid.
func TestLiftEnum_InvalidDiscriminant(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	enumT := b.InternEnum([]string{"red", "green", "blue"})
	ct := b.Finish()

	ctx := &abi.LiftContext{Types: ct}
	_, err := abi.LiftFlat(ctx, enumT, abi.NewFlatIter([]uint64{3}))
	require.Error(t, err)
}

// TestLiftFlags asserts that LiftFlat correctly decodes flags from i32
// bit words into a map of flag names to booleans.
//
// Spec: definitions.py:166-168 FlagsType; lift_flat at :1512-1524.
func TestLiftFlags(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	flagsT := b.InternFlags([]string{"read", "write", "execute"})
	ct := b.Finish()

	ctx := &abi.LiftContext{Types: ct}
	// Bit 0 (read) and bit 2 (execute) set -> 0b101 = 5
	val, err := abi.LiftFlat(ctx, flagsT, abi.NewFlatIter([]uint64{5}))
	require.NoError(t, err)
	flags := val.Flags()
	require.True(t, flags["read"])
	require.False(t, flags["write"])
	require.True(t, flags["execute"])

	// No flags set.
	val, err = abi.LiftFlat(ctx, flagsT, abi.NewFlatIter([]uint64{0}))
	require.NoError(t, err)
	flags = val.Flags()
	require.False(t, flags["read"])
	require.False(t, flags["write"])
	require.False(t, flags["execute"])
}

// TestLiftVariant asserts that LiftFlat correctly decodes a variant with
// discriminant and optional payload.
//
// Spec: definitions.py:128-132 VariantType; lift_flat at :1478-1504.
func TestLiftVariant(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	varT := b.InternVariant([]types.VariantCase{
		{Name: "none", HasPayload: false},
		{Name: "some", Payload: types.S32, HasPayload: true},
	})
	ct := b.Finish()

	ctx := &abi.LiftContext{Types: ct}

	// Case "none" (discriminant 0), payload padding consumed.
	val, err := abi.LiftFlat(ctx, varT, abi.NewFlatIter([]uint64{0, 0}))
	require.NoError(t, err)
	caseName, payload := val.Variant()
	require.Equal(t, "none", caseName)
	require.Nil(t, payload)

	// Case "some" (discriminant 1) with payload 42.
	val, err = abi.LiftFlat(ctx, varT, abi.NewFlatIter([]uint64{1, 42}))
	require.NoError(t, err)
	caseName, payload = val.Variant()
	require.Equal(t, "some", caseName)
	require.NotNil(t, payload)
	require.Equal(t, int32(42), payload.S32())
}

// TestLiftVariant_MultiplePayloadTypes asserts that LiftFlat correctly
// handles a variant with multiple cases carrying different payload types.
//
// Spec: definitions.py:128-132 VariantType; lift_flat at :1478-1504.
// The joined flat layout accommodates the largest payload.
func TestLiftVariant_MultiplePayloadTypes(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	varT := b.InternVariant([]types.VariantCase{
		{Name: "int-val", Payload: types.S32, HasPayload: true},
		{Name: "float-val", Payload: types.F64, HasPayload: true},
		{Name: "empty", HasPayload: false},
	})
	ct := b.Finish()

	ctx := &abi.LiftContext{Types: ct}

	// Case "int-val" (discriminant 0) with s32 payload -7.
	// The joined layout is i64 (widened from s32 and f64).
	neg7 := int32(-7)
	val, err := abi.LiftFlat(ctx, varT, abi.NewFlatIter([]uint64{0, uint64(uint32(neg7))}))
	require.NoError(t, err)
	caseName, payload := val.Variant()
	require.Equal(t, "int-val", caseName)
	require.NotNil(t, payload)
	require.Equal(t, int32(-7), payload.S32())

	// Case "float-val" (discriminant 1) with f64 payload.
	f64bits := math.Float64bits(3.14)
	val, err = abi.LiftFlat(ctx, varT, abi.NewFlatIter([]uint64{1, f64bits}))
	require.NoError(t, err)
	caseName, payload = val.Variant()
	require.Equal(t, "float-val", caseName)
	require.NotNil(t, payload)
	require.Equal(t, 3.14, payload.F64())

	// Case "empty" (discriminant 2), no payload.
	val, err = abi.LiftFlat(ctx, varT, abi.NewFlatIter([]uint64{2, 0}))
	require.NoError(t, err)
	caseName, payload = val.Variant()
	require.Equal(t, "empty", caseName)
	require.Nil(t, payload)
}

func TestExportedFunc_Call_ListOfChar(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	listT := b.InternList(types.Char)
	ct := b.Finish()
	paramTypes := []types.ValType{listT}

	ef := makeListTestExportedFunc(t, ct, paramTypes)

	results, err := ef.Call(context.Background(), types.ValList([]types.Val{
		types.ValChar('A'), types.ValChar(0x1F600), types.ValChar('Z'),
	}))
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	elems := results[0].List()
	require.Equal(t, 3, len(elems))
	require.Equal(t, rune('A'), elems[0].Char())
	require.Equal(t, rune(0x1F600), elems[1].Char())
	require.Equal(t, rune('Z'), elems[2].Char())
}

// TestInstance_ValueIndexSpace asserts that AddValue populates the value
// index space and GetValue retrieves by index, including out-of-bounds.
//
// Spec: definitions.py:256-273 class ComponentInstance — start function
// may produce values that later instantiation steps consume by index.
func TestInstance_ValueIndexSpace(t *testing.T) {
	inst := newInstance(&Component{}, 1, nil)

	// Empty initially.
	_, err := inst.GetValue(0)
	require.Error(t, err)

	// Add two values.
	idx0 := inst.AddValue(types.ValU32(100))
	idx1 := inst.AddValue(types.ValString("hello"))
	require.Equal(t, uint32(0), idx0)
	require.Equal(t, uint32(1), idx1)

	// Retrieve them.
	v0, err := inst.GetValue(0)
	require.NoError(t, err)
	require.Equal(t, uint32(100), v0.U32())

	v1, err := inst.GetValue(1)
	require.NoError(t, err)
	require.Equal(t, "hello", v1.StringVal())

	// Out-of-bounds.
	_, err = inst.GetValue(2)
	require.Error(t, err)
}

// TestInstance_ConsumeValue asserts that ConsumeValue retrieves a value
// and marks it consumed, preventing double-consumption.
//
// Spec: definitions.py:256-273 — values from start functions are
// single-use; double consumption is an error.
func TestInstance_ConsumeValue(t *testing.T) {
	inst := newInstance(&Component{}, 1, nil)
	inst.AddValue(types.ValS32(42))

	// First consume succeeds.
	val, err := inst.ConsumeValue(0)
	require.NoError(t, err)
	require.Equal(t, int32(42), val.S32())
	require.True(t, inst.IsValueConsumed(0))

	// Second consume fails.
	_, err = inst.ConsumeValue(0)
	require.Error(t, err)

	// Out-of-bounds consume fails.
	_, err = inst.ConsumeValue(99)
	require.Error(t, err)
}

// TestInstance_ParentChild asserts that AddChild wires the parent/children
// relationship and Parent() returns the correct instance.
//
// Spec: definitions.py:256-273 class ComponentInstance — nested
// component instantiation produces parent/child relationships.
func TestInstance_ParentChild(t *testing.T) {
	parent := newInstance(&Component{}, 1, nil)
	child := newInstance(&Component{}, 2, nil)

	require.Nil(t, child.Parent())
	require.Equal(t, 0, len(parent.Children()))

	parent.AddChild(child)

	require.Equal(t, parent, child.Parent())
	require.Equal(t, 1, len(parent.Children()))
	require.Equal(t, child, parent.Children()[0])
}

// TestInstance_ParentChain asserts that a multi-level parent chain is
// navigable via Parent().
//
// Spec: definitions.py:290-299 call_might_be_recursive uses reflexive
// ancestor traversal, which depends on parent chain integrity.
func TestInstance_ParentChain(t *testing.T) {
	root := newInstance(&Component{}, 1, nil)
	mid := newInstance(&Component{}, 2, nil)
	leaf := newInstance(&Component{}, 3, nil)

	root.AddChild(mid)
	mid.AddChild(leaf)

	require.Equal(t, mid, leaf.Parent())
	require.Equal(t, root, mid.Parent())
	require.Nil(t, root.Parent())
	require.Equal(t, root, leaf.Parent().Parent())
}

// TestInstance_GetAncestor asserts that GetAncestor walks the parent
// chain by the given depth and returns nil for excessive depth.
//
// Spec: definitions.py:290-299 — reflexive_ancestors walks the parent
// chain; GetAncestor is the helper for indexed access.
func TestInstance_GetAncestor(t *testing.T) {
	root := newInstance(&Component{}, 1, nil)
	mid := newInstance(&Component{}, 2, nil)
	leaf := newInstance(&Component{}, 3, nil)

	root.AddChild(mid)
	mid.AddChild(leaf)

	// Depth 0 = self.
	require.Equal(t, leaf, leaf.GetAncestor(0))
	// Depth 1 = parent.
	require.Equal(t, mid, leaf.GetAncestor(1))
	// Depth 2 = grandparent.
	require.Equal(t, root, leaf.GetAncestor(2))
	// Depth 3 = past root -> nil.
	require.Nil(t, leaf.GetAncestor(3))
}

// TestInstance_IndexSpaces asserts that AddInstanceToSpace /
// GetInstanceFromSpace and AddTypeToSpace / GetTypeFromSpace maintain
// their respective index spaces with sequential indexing.
//
// Spec: definitions.py:256-273 — ComponentInstance maintains index
// spaces for instances and types during nested component support.
func TestInstance_IndexSpaces(t *testing.T) {
	inst := newInstance(&Component{}, 1, nil)
	child1 := newInstance(&Component{}, 2, nil)
	child2 := newInstance(&Component{}, 3, nil)

	// Instance index space.
	idx0 := inst.AddInstanceToSpace(child1)
	idx1 := inst.AddInstanceToSpace(child2)
	require.Equal(t, uint32(0), idx0)
	require.Equal(t, uint32(1), idx1)
	require.Equal(t, child1, inst.GetInstanceFromSpace(0))
	require.Equal(t, child2, inst.GetInstanceFromSpace(1))
	require.Nil(t, inst.GetInstanceFromSpace(2))

	// Type index space.
	td := &TypeDef{Kind: TypeDefKindFunc}
	tidx := inst.AddTypeToSpace(td)
	require.Equal(t, uint32(0), tidx)
	require.Equal(t, td, inst.GetTypeFromSpace(0))
	require.Nil(t, inst.GetTypeFromSpace(1))
}

// TestInstance_IndexSpaces_Component asserts that AddComponentToSpace /
// GetComponentFromSpace maintain the component index space.
//
// Spec: nested component support — the instance keeps a component index
// space for sub-components encountered during instantiation.
func TestInstance_IndexSpaces_Component(t *testing.T) {
	inst := newInstance(&Component{}, 1, nil)
	comp := &Component{}

	idx := inst.AddComponentToSpace(comp)
	require.Equal(t, uint32(0), idx)
	require.Equal(t, comp, inst.GetComponentFromSpace(0))
	require.Nil(t, inst.GetComponentFromSpace(1))
}

// TestInstance_IndexSpaces_OutOfBounds asserts that Get*FromSpace returns
// nil for out-of-bounds indices rather than panicking.
//
// Spec: index-space lookups must validate bounds.
func TestInstance_IndexSpaces_OutOfBounds(t *testing.T) {
	inst := newInstance(&Component{}, 1, nil)

	require.Nil(t, inst.GetInstanceFromSpace(0))
	require.Nil(t, inst.GetTypeFromSpace(0))
	require.Nil(t, inst.GetComponentFromSpace(0))
}

// TestInstance_ExportedInstances asserts that AddExportedInstance /
// GetExportedInstance maintain the name-keyed exported instances map.
//
// Spec: component-model export resolution — exported instances are
// accessed by name for API consumers.
func TestInstance_ExportedInstances(t *testing.T) {
	parent := newInstance(&Component{}, 1, nil)
	child := newInstance(&Component{}, 2, nil)

	// Not found before adding.
	require.Nil(t, parent.GetExportedInstance("child"))

	parent.AddExportedInstance("child", child)
	require.Equal(t, child, parent.GetExportedInstance("child"))

	// Different name returns nil.
	require.Nil(t, parent.GetExportedInstance("other"))
}

// TestLiftResolvedType_RecordRetptr asserts that LiftHeap correctly reads
// a record from linear memory with aligned field layout.
//
// Spec: definitions.py:1326-1333 load for record — fields are read
// sequentially at aligned offsets within the record.
func TestLiftResolvedType_RecordRetptr(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	recT := b.InternRecord([]types.RecordField{
		{Name: "x", Type: types.S32},
		{Name: "y", Type: types.S32},
	})
	ct := b.Finish()

	mem := wazerotest.NewMemory(wazerotest.PageSize)
	// record {x: s32, y: s32} at offset 0: x at +0, y at +4.
	binary.LittleEndian.PutUint32(mem.Bytes[0:], uint32(int32(10)))
	binary.LittleEndian.PutUint32(mem.Bytes[4:], uint32(int32(20)))

	ctx := &abi.LiftContext{Memory: mem, Types: ct}
	val, err := abi.LiftHeap(ctx, recT, 0)
	require.NoError(t, err)
	rec := val.Record()
	require.Equal(t, int32(10), rec["x"].S32())
	require.Equal(t, int32(20), rec["y"].S32())
}

// TestLiftResolvedType_RecordFlat asserts that LiftFlat correctly reads
// a record from the flat value iterator.
//
// Spec: definitions.py:1461-1469 lift_flat for record — each field is
// lifted sequentially from the flat iterator.
func TestLiftResolvedType_RecordFlat(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	recT := b.InternRecord([]types.RecordField{
		{Name: "a", Type: types.U8},
		{Name: "b", Type: types.U64},
	})
	ct := b.Finish()

	ctx := &abi.LiftContext{Types: ct}
	iter := abi.NewFlatIter([]uint64{255, 0xDEADBEEF12345678})
	val, err := abi.LiftFlat(ctx, recT, iter)
	require.NoError(t, err)
	rec := val.Record()
	require.Equal(t, uint8(255), rec["a"].U8())
	require.Equal(t, uint64(0xDEADBEEF12345678), rec["b"].U64())
}

// TestLiftResolvedType_LargeRecordRetptr asserts that LiftHeap handles
// a record with many fields (forcing alignment padding between them).
//
// Spec: definitions.py:1326-1333 load for record — field alignment
// must be respected for mixed-size fields.
func TestLiftResolvedType_LargeRecordRetptr(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	recT := b.InternRecord([]types.RecordField{
		{Name: "flag", Type: types.Bool},   // offset 0, size 1
		{Name: "count", Type: types.U32},   // align 4 -> offset 4, size 4
		{Name: "value", Type: types.F64},   // align 8 -> offset 8, size 8
		{Name: "tag", Type: types.U8},      // offset 16, size 1
		{Name: "amount", Type: types.S64},  // align 8 -> offset 24, size 8
	})
	ct := b.Finish()

	mem := wazerotest.NewMemory(wazerotest.PageSize)
	// flag at offset 0
	mem.Bytes[0] = 1
	// count at offset 4
	binary.LittleEndian.PutUint32(mem.Bytes[4:], 42)
	// value at offset 8
	binary.LittleEndian.PutUint64(mem.Bytes[8:], math.Float64bits(3.14))
	// tag at offset 16
	mem.Bytes[16] = 7
	// amount at offset 24
	amountVal := int64(-999)
	binary.LittleEndian.PutUint64(mem.Bytes[24:], uint64(amountVal))

	ctx := &abi.LiftContext{Memory: mem, Types: ct}
	val, err := abi.LiftHeap(ctx, recT, 0)
	require.NoError(t, err)
	rec := val.Record()
	require.True(t, rec["flag"].Bool())
	require.Equal(t, uint32(42), rec["count"].U32())
	require.Equal(t, 3.14, rec["value"].F64())
	require.Equal(t, uint8(7), rec["tag"].U8())
	require.Equal(t, int64(-999), rec["amount"].S64())
}

// TestLiftFieldFromMemory_AllPrimitiveTypes asserts that LiftHeap can
// read every primitive type from linear memory. This exercises the
// per-kind heap-read arms in abi.LiftHeap.
//
// Spec: definitions.py:1326-1333 load — each primitive is loaded at
// its natural size and alignment.
func TestLiftFieldFromMemory_AllPrimitiveTypes(t *testing.T) {
	mem := wazerotest.NewMemory(wazerotest.PageSize)
	ctx := &abi.LiftContext{Memory: mem}

	tests := []struct {
		name   string
		typ    types.ValType
		write  func(offset uint32)
		check  func(t *testing.T, v types.Val)
		size   uint32
	}{
		{
			name: "bool_true",
			typ:  types.Bool,
			write: func(o uint32) { mem.Bytes[o] = 1 },
			check: func(t *testing.T, v types.Val) { require.True(t, v.Bool()) },
			size:  1,
		},
		{
			name: "bool_false",
			typ:  types.Bool,
			write: func(o uint32) { mem.Bytes[o] = 0 },
			check: func(t *testing.T, v types.Val) { require.False(t, v.Bool()) },
			size:  1,
		},
		{
			name: "u8",
			typ:  types.U8,
			write: func(o uint32) { mem.Bytes[o] = 200 },
			check: func(t *testing.T, v types.Val) { require.Equal(t, uint8(200), v.U8()) },
			size:  1,
		},
		{
			name:  "s8",
			typ:   types.S8,
			write: func(o uint32) { s := int8(-50); mem.Bytes[o] = byte(s) },
			check: func(t *testing.T, v types.Val) { require.Equal(t, int8(-50), v.S8()) },
			size:  1,
		},
		{
			name: "u16",
			typ:  types.U16,
			write: func(o uint32) { binary.LittleEndian.PutUint16(mem.Bytes[o:], 60000) },
			check: func(t *testing.T, v types.Val) { require.Equal(t, uint16(60000), v.U16()) },
			size:  2,
		},
		{
			name:  "s16",
			typ:   types.S16,
			write: func(o uint32) { s := int16(-1000); binary.LittleEndian.PutUint16(mem.Bytes[o:], uint16(s)) },
			check: func(t *testing.T, v types.Val) { require.Equal(t, int16(-1000), v.S16()) },
			size:  2,
		},
		{
			name: "u32",
			typ:  types.U32,
			write: func(o uint32) { binary.LittleEndian.PutUint32(mem.Bytes[o:], 3000000) },
			check: func(t *testing.T, v types.Val) { require.Equal(t, uint32(3000000), v.U32()) },
			size:  4,
		},
		{
			name:  "s32",
			typ:   types.S32,
			write: func(o uint32) { s := int32(-12345); binary.LittleEndian.PutUint32(mem.Bytes[o:], uint32(s)) },
			check: func(t *testing.T, v types.Val) { require.Equal(t, int32(-12345), v.S32()) },
			size:  4,
		},
		{
			name: "u64",
			typ:  types.U64,
			write: func(o uint32) { binary.LittleEndian.PutUint64(mem.Bytes[o:], 0xDEADBEEF) },
			check: func(t *testing.T, v types.Val) { require.Equal(t, uint64(0xDEADBEEF), v.U64()) },
			size:  8,
		},
		{
			name:  "s64",
			typ:   types.S64,
			write: func(o uint32) { s := int64(-9876543210); binary.LittleEndian.PutUint64(mem.Bytes[o:], uint64(s)) },
			check: func(t *testing.T, v types.Val) { require.Equal(t, int64(-9876543210), v.S64()) },
			size:  8,
		},
		{
			name: "f32",
			typ:  types.F32,
			write: func(o uint32) { binary.LittleEndian.PutUint32(mem.Bytes[o:], math.Float32bits(2.5)) },
			check: func(t *testing.T, v types.Val) { require.Equal(t, float32(2.5), v.F32()) },
			size:  4,
		},
		{
			name: "f64",
			typ:  types.F64,
			write: func(o uint32) { binary.LittleEndian.PutUint64(mem.Bytes[o:], math.Float64bits(1.23456789)) },
			check: func(t *testing.T, v types.Val) { require.Equal(t, 1.23456789, v.F64()) },
			size:  8,
		},
		{
			name: "char",
			typ:  types.Char,
			write: func(o uint32) { binary.LittleEndian.PutUint32(mem.Bytes[o:], 0x1F600) },
			check: func(t *testing.T, v types.Val) { require.Equal(t, rune(0x1F600), v.Char()) },
			size:  4,
		},
	}

	offset := uint32(0)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Align offset to the type's natural alignment.
			if tc.size > 1 {
				offset = types.AlignTo(offset, tc.size)
			}
			tc.write(offset)
			val, err := abi.LiftHeap(ctx, tc.typ, offset)
			require.NoError(t, err)
			tc.check(t, val)
			offset += tc.size
		})
	}
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

// --- Task 5: Two-phase post-return protocol tests ---------------------------
//
// These test the ExportedFunc.Call / PostReturn / CallAndPostReturn surface
// matching the spec's two-phase canon_lift protocol (definitions.py:1999-2002)
// and the wasmtime C API (func_call + func_post_return).

// TestExportedFunc_TwoPhase_BasicProtocol asserts that for a canon.lift
// export with a post-return function, Call returns results without running
// post-return, and PostReturn completes the cleanup.
//
// Spec: definitions.py:1999-2002 — post_return invocation after results
// are available.
func TestExportedFunc_TwoPhase_BasicProtocol(t *testing.T) {
	postReturnCalled := false
	prRef := &postReturnState{}
	ef := &ExportedFunc{
		name:     "test-func",
		funcType: &types.TypeFunc{},
		prRef:    prRef,
		impl: func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			// Simulate what buildCanonLiftFunc does: set the post-return
			// closure on the shared ref and mark needsPostReturn.
			prRef.needsPostReturn = true
			prRef.fn = func(ctx context.Context) error {
				postReturnCalled = true
				return nil
			}
			return []types.Val{types.ValU32(42)}, nil
		},
	}

	ctx := context.Background()

	// Phase 1: Call returns results.
	results, err := ef.Call(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, uint32(42), results[0].U32())

	// Post-return has NOT been called yet.
	require.False(t, postReturnCalled)
	require.True(t, ef.NeedsPostReturn())

	// Phase 2: PostReturn runs the deferred cleanup.
	err = ef.PostReturn(ctx)
	require.NoError(t, err)
	require.True(t, postReturnCalled)
	require.False(t, ef.NeedsPostReturn())
}

// TestExportedFunc_CallAndPostReturn asserts that CallAndPostReturn is
// equivalent to Call + PostReturn in sequence.
//
// Spec: convenience API matching wasmtime's combined call path.
func TestExportedFunc_CallAndPostReturn(t *testing.T) {
	postReturnCalled := false
	prRef := &postReturnState{}
	ef := &ExportedFunc{
		name:     "test-func",
		funcType: &types.TypeFunc{},
		prRef:    prRef,
		impl: func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			prRef.needsPostReturn = true
			prRef.fn = func(ctx context.Context) error {
				postReturnCalled = true
				return nil
			}
			return []types.Val{types.ValS32(7)}, nil
		},
	}

	ctx := context.Background()
	results, err := ef.CallAndPostReturn(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, int32(7), results[0].S32())
	require.True(t, postReturnCalled)
	require.False(t, ef.NeedsPostReturn())
}

// TestExportedFunc_TwoPhase_PanicOnReentry asserts that calling Call
// while PostReturn is pending panics.
//
// Spec: definitions.py:1999-2002 — cannot re-enter until post-return
// completes.
func TestExportedFunc_TwoPhase_PanicOnReentry(t *testing.T) {
	prRef := &postReturnState{}
	ef := &ExportedFunc{
		name:     "test-func",
		funcType: &types.TypeFunc{},
		prRef:    prRef,
		impl: func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			prRef.needsPostReturn = true
			prRef.fn = func(ctx context.Context) error { return nil }
			return nil, nil
		},
	}

	ctx := context.Background()

	// First call succeeds.
	_, err := ef.Call(ctx)
	require.NoError(t, err)

	// Second call without PostReturn panics.
	defer func() {
		r := recover()
		require.NotNil(t, r)
		msg, ok := r.(string)
		require.True(t, ok)
		require.Contains(t, msg, "PostReturn must be called before calling again")
	}()
	_, _ = ef.Call(ctx)
	t.Fatal("should have panicked")
}

// TestExportedFunc_PostReturn_NilReceiver asserts that PostReturn on a
// nil ExportedFunc returns an error rather than panicking.
func TestExportedFunc_PostReturn_NilReceiver(t *testing.T) {
	var ef *ExportedFunc
	err := ef.PostReturn(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "nil receiver")
}

// TestExportedFunc_PostReturn_NoPrRef asserts that PostReturn is a no-op
// for non-canon.lift exports (prRef is nil).
func TestExportedFunc_PostReturn_NoPrRef(t *testing.T) {
	ef := &ExportedFunc{
		name:     "imported-func",
		funcType: &types.TypeFunc{},
		impl: func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			return nil, nil
		},
	}

	ctx := context.Background()

	// Call succeeds.
	_, err := ef.Call(ctx)
	require.NoError(t, err)

	// PostReturn is a no-op (prRef is nil).
	err = ef.PostReturn(ctx)
	require.NoError(t, err)
	require.False(t, ef.NeedsPostReturn())
}

// TestExportedFunc_PostReturn_Idempotent asserts that calling PostReturn
// twice is safe — the second call is a no-op.
func TestExportedFunc_PostReturn_Idempotent(t *testing.T) {
	callCount := 0
	prRef := &postReturnState{}
	ef := &ExportedFunc{
		name:     "test-func",
		funcType: &types.TypeFunc{},
		prRef:    prRef,
		impl: func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			prRef.needsPostReturn = true
			prRef.fn = func(ctx context.Context) error {
				callCount++
				return nil
			}
			return nil, nil
		},
	}

	ctx := context.Background()
	_, err := ef.Call(ctx)
	require.NoError(t, err)

	// First PostReturn runs the closure.
	err = ef.PostReturn(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, callCount)

	// Second PostReturn is a no-op.
	err = ef.PostReturn(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, callCount)
}

// TestExportedFunc_NeedsPostReturn_NilReceiver asserts NeedsPostReturn
// returns false for nil receiver.
func TestExportedFunc_NeedsPostReturn_NilReceiver(t *testing.T) {
	var ef *ExportedFunc
	require.False(t, ef.NeedsPostReturn())
}

// TestExportedFunc_TwoPhase_MultipleCallCycles asserts that Call /
// PostReturn can be invoked multiple times in sequence.
func TestExportedFunc_TwoPhase_MultipleCallCycles(t *testing.T) {
	callCount := 0
	postReturnCount := 0
	prRef := &postReturnState{}
	ef := &ExportedFunc{
		name:     "test-func",
		funcType: &types.TypeFunc{},
		prRef:    prRef,
		impl: func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			callCount++
			prRef.needsPostReturn = true
			prRef.fn = func(ctx context.Context) error {
				postReturnCount++
				return nil
			}
			return []types.Val{types.ValU32(uint32(callCount))}, nil
		},
	}

	ctx := context.Background()

	for i := 1; i <= 3; i++ {
		results, err := ef.Call(ctx)
		require.NoError(t, err)
		require.Equal(t, uint32(i), results[0].U32())

		err = ef.PostReturn(ctx)
		require.NoError(t, err)
	}

	require.Equal(t, 3, callCount)
	require.Equal(t, 3, postReturnCount)
}

// TestExportedFunc_CallAndPostReturn_CallError asserts that when Call
// fails, CallAndPostReturn still runs PostReturn for cleanup.
func TestExportedFunc_CallAndPostReturn_CallError(t *testing.T) {
	postReturnCalled := false
	prRef := &postReturnState{}
	ef := &ExportedFunc{
		name:     "test-func",
		funcType: &types.TypeFunc{},
		prRef:    prRef,
		impl: func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			// Simulate: set needsPostReturn then error.
			prRef.needsPostReturn = true
			prRef.fn = func(ctx context.Context) error {
				postReturnCalled = true
				return nil
			}
			return nil, fmt.Errorf("core call failed")
		},
	}

	ctx := context.Background()
	_, err := ef.CallAndPostReturn(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "core call failed")
	// PostReturn should have been called for cleanup.
	require.True(t, postReturnCalled)
	require.False(t, ef.NeedsPostReturn())
}

// TestExportedFunc_CallAndPostReturn_PostReturnError asserts that when
// PostReturn fails, the error is returned along with the results.
func TestExportedFunc_CallAndPostReturn_PostReturnError(t *testing.T) {
	prRef := &postReturnState{}
	ef := &ExportedFunc{
		name:     "test-func",
		funcType: &types.TypeFunc{},
		prRef:    prRef,
		impl: func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			prRef.needsPostReturn = true
			prRef.fn = func(ctx context.Context) error {
				return fmt.Errorf("post-return failed")
			}
			return []types.Val{types.ValU32(99)}, nil
		},
	}

	ctx := context.Background()
	results, err := ef.CallAndPostReturn(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "post-return failed")
	// Results are still returned even when post-return fails.
	require.Equal(t, 1, len(results))
	require.Equal(t, uint32(99), results[0].U32())
}
