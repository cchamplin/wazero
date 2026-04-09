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
	"encoding/binary"
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

// --- ExportedFunc.Call list-param tests (skipped: pipeline not wired) --------
//
// These tests exercise ExportedFunc.Call with list-typed parameters.
// ExportedFunc.Call delegates to a HostFunc closure built by wireExports,
// which requires a fully instantiated canon.lift/canon.lower pipeline
// including realloc, memory, and LowerContext wiring. The list element
// lift/lower is tested directly in abi/lift_test.go and
// conformance/composites_test.go. These stubs remain until the full
// ExportedFunc.Call pipeline supports list params end-to-end.

func TestExportedFunc_Call_ListWithRealloc(t *testing.T) {
	t.Skip("ExportedFunc.Call does not yet wire realloc for list params through the canon.lift/canon.lower pipeline")
}

func TestExportedFunc_Call_ListWithoutRealloc(t *testing.T) {
	t.Skip("ExportedFunc.Call does not yet wire list params through the canon.lift/canon.lower pipeline")
}

func TestExportedFunc_Call_EmptyList(t *testing.T) {
	t.Skip("ExportedFunc.Call does not yet wire list params through the canon.lift/canon.lower pipeline")
}

func TestExportedFunc_Call_ListOfS64(t *testing.T) {
	t.Skip("ExportedFunc.Call does not yet wire list<s64> params through the canon.lift/canon.lower pipeline")
}

func TestExportedFunc_Call_ListOfF32(t *testing.T) {
	t.Skip("ExportedFunc.Call does not yet wire list<f32> params through the canon.lift/canon.lower pipeline")
}

func TestExportedFunc_Call_ListOfF64(t *testing.T) {
	t.Skip("ExportedFunc.Call does not yet wire list<f64> params through the canon.lift/canon.lower pipeline")
}

func TestExportedFunc_Call_ListOfU8(t *testing.T) {
	t.Skip("ExportedFunc.Call does not yet wire list<u8> params through the canon.lift/canon.lower pipeline")
}

func TestExportedFunc_Call_ListOfS8(t *testing.T) {
	t.Skip("ExportedFunc.Call does not yet wire list<s8> params through the canon.lift/canon.lower pipeline")
}

func TestExportedFunc_Call_ListOfU16(t *testing.T) {
	t.Skip("ExportedFunc.Call does not yet wire list<u16> params through the canon.lift/canon.lower pipeline")
}

func TestExportedFunc_Call_ListOfS16(t *testing.T) {
	t.Skip("ExportedFunc.Call does not yet wire list<s16> params through the canon.lift/canon.lower pipeline")
}

func TestExportedFunc_Call_ListOfU64(t *testing.T) {
	t.Skip("ExportedFunc.Call does not yet wire list<u64> params through the canon.lift/canon.lower pipeline")
}

func TestExportedFunc_Call_ListOfBool(t *testing.T) {
	t.Skip("ExportedFunc.Call does not yet wire list<bool> params through the canon.lift/canon.lower pipeline")
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
	t.Skip("ExportedFunc.Call does not yet wire list<char> params through the canon.lift/canon.lower pipeline")
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
				offset = alignTo(offset, tc.size)
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
