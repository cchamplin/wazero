package abi

import (
	"testing"

	"github.com/tetratelabs/wazero/experimental/wazerotest"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestLiftParamsFlatPath asserts LiftParams consumes its flat value
// iterator per-param when the flattened parameter count is within
// maxFlat. This mirrors LowerParams's flat branch in the lift
// direction.
//
// Spec: definitions.py:1943-1952 lift_flat_values flat branch
// (return [lift_flat(cx, vi, t) for t in ts]).
// Canonical test: run_tests.py test_flatten small-signature cases.
func TestLiftParamsFlatPath(t *testing.T) {
	bag := types.NewComponentTypesBuilder().Finish()
	inst := runtime.NewComponentInstance(0, nil)
	mem := wazerotest.NewMemory(wazerotest.PageSize)
	ctx := &LiftContext{
		Memory:      mem,
		Types:       bag,
		Instance:    inst,
		BorrowScope: runtime.NewBorrowScope(inst.Table),
	}
	paramTypes := []types.ValType{types.S32, types.S32}
	flat := []uint64{42, 0xFFFFFFF9} // 42, -7
	vals, err := LiftParams(ctx, paramTypes, flat, MaxFlatParams)
	require.NoError(t, err)
	require.Equal(t, 2, len(vals))
	require.Equal(t, int32(42), vals[0].S32())
	require.Equal(t, int32(-7), vals[1].S32())
}

// TestLiftResultsFlatPath asserts LiftResults takes the flat branch
// when the flattened result width is within MaxFlatResults. A single
// S32 result fits in the 1-slot cap, so the retptr path is not taken.
//
// Spec: definitions.py:1943-1952 lift_flat_values applied to the
// canon.lift return path at definitions.py:1997.
func TestLiftResultsFlatPath(t *testing.T) {
	bag := types.NewComponentTypesBuilder().Finish()
	inst := runtime.NewComponentInstance(0, nil)
	mem := wazerotest.NewMemory(wazerotest.PageSize)
	ctx := &LiftContext{
		Memory:      mem,
		Types:       bag,
		Instance:    inst,
		BorrowScope: runtime.NewBorrowScope(inst.Table),
	}
	// Single i32 result — fits in MaxFlatResults = 1.
	resultTypes := []types.ValType{types.S32}
	flat := []uint64{1234}
	vals, err := LiftResults(ctx, resultTypes, flat, MaxFlatResults)
	require.NoError(t, err)
	require.Equal(t, 1, len(vals))
	require.Equal(t, int32(1234), vals[0].S32())
}

// TestLowerResultsFlatPath asserts LowerResults writes lowered values
// directly into stack[0..flatResultWidth] when needsRetptr is false.
// This is the symmetric lower for canon.lower's return path when the
// result width fits in MAX_FLAT_RESULTS.
//
// Spec: definitions.py:1954-1974 lower_flat_values applied to the
// canon.lower return path at definitions.py:2104 (the `out_param`
// None-equivalent flat branch).
func TestLowerResultsFlatPath(t *testing.T) {
	bag := types.NewComponentTypesBuilder().Finish()
	inst := runtime.NewComponentInstance(0, nil)
	mem := wazerotest.NewMemory(wazerotest.PageSize)
	ctx := &LowerContext{
		Memory:   mem,
		Types:    bag,
		Instance: inst,
	}
	resultTypes := []types.ValType{types.S32}
	results := []types.Val{types.ValS32(999)}
	stack := make([]uint64, 1)
	err := LowerResults(ctx, resultTypes, results, stack, false, MaxFlatResults)
	require.NoError(t, err)
	require.Equal(t, int32(999), int32(stack[0]))
}

// TestLiftParamsRetptrPath exercises the retptr branch of LiftParams:
// 3 S32 params with maxFlat=2 forces `len(flatTypes) > maxFlat`, so
// LiftParams must read flat[0] as a pointer, compute the tuple layout
// via paramsTupleLayout, and LiftHeap each element from ptr+offsets[i].
//
// Spec: definitions.py:1943-1952 lift_flat_values retptr branch
// (ptr = vi.next('i32'); ...; [load(cx, ptr + offsets[i], t) for i, t
// in enumerate(ts)]).
func TestLiftParamsRetptrPath(t *testing.T) {
	bag := types.NewComponentTypesBuilder().Finish()
	inst := runtime.NewComponentInstance(0, nil)
	mem := wazerotest.NewMemory(wazerotest.PageSize)
	const ptr uint32 = 1024
	// Pre-populate memory with three i32 values at ptr, ptr+4, ptr+8.
	for i := 0; i < 3; i++ {
		require.True(t, mem.WriteUint32Le(ptr+uint32(i*4), uint32(i+1)))
	}
	ctx := &LiftContext{
		Memory:      mem,
		Types:       bag,
		Instance:    inst,
		BorrowScope: runtime.NewBorrowScope(inst.Table),
	}
	paramTypes := []types.ValType{types.S32, types.S32, types.S32}
	vals, err := LiftParams(ctx, paramTypes, []uint64{uint64(ptr)}, 2)
	require.NoError(t, err)
	require.Equal(t, 3, len(vals))
	for i := 0; i < 3; i++ {
		require.Equal(t, int32(i+1), vals[i].S32())
	}
}

// TestLiftResultsRetptrPath exercises the retptr branch of LiftResults:
// 2 S32 results with maxFlat=0 forces the retptr path, so flat[0] is
// read as a pointer and the results are loaded from memory at
// ptr+offsets[i] via paramsTupleLayout.
//
// Spec: definitions.py:1943-1952 lift_flat_values retptr branch,
// applied to the canon.lift return path at definitions.py:1997.
func TestLiftResultsRetptrPath(t *testing.T) {
	bag := types.NewComponentTypesBuilder().Finish()
	inst := runtime.NewComponentInstance(0, nil)
	mem := wazerotest.NewMemory(wazerotest.PageSize)
	const ptr uint32 = 2048
	require.True(t, mem.WriteUint32Le(ptr+0, 100))
	require.True(t, mem.WriteUint32Le(ptr+4, 200))
	ctx := &LiftContext{
		Memory:      mem,
		Types:       bag,
		Instance:    inst,
		BorrowScope: runtime.NewBorrowScope(inst.Table),
	}
	resultTypes := []types.ValType{types.S32, types.S32}
	vals, err := LiftResults(ctx, resultTypes, []uint64{uint64(ptr)}, 0)
	require.NoError(t, err)
	require.Equal(t, 2, len(vals))
	require.Equal(t, int32(100), vals[0].S32())
	require.Equal(t, int32(200), vals[1].S32())
}

// TestLowerResultsRetptrPath is the load-bearing regression test for
// the stack[len(stack)-1] retptr convention. By the CoreSignature
// convention at internal/component/abi/flatten.go:41-51, a lowered
// function with needsRetptr=true has the retptr appended as the LAST
// core-wasm parameter, so LowerResults must read `stack[len(stack)-1]`
// to recover the caller-provided pointer. The leading stack slots
// (0x1111, 0x2222, 0x3333) are distinct opaque values so any
// regression that read from stack[0] instead of stack[len-1] would
// dereference 0x1111 (or similar) and the subsequent memory writes
// would either land at the wrong address or fail bounds-check.
//
// Spec: definitions.py:2104-2113 canon_lower calls lower_flat_values
// with flat_args as out_param; flatten.go:41-51 CoreSignature emits
// retptr as trailing i32 param.
func TestLowerResultsRetptrPath(t *testing.T) {
	bag := types.NewComponentTypesBuilder().Finish()
	inst := runtime.NewComponentInstance(0, nil)
	mem := wazerotest.NewMemory(wazerotest.PageSize)
	ctx := &LowerContext{
		Memory:   mem,
		Types:    bag,
		Instance: inst,
	}
	const ptr uint32 = 1024
	resultTypes := []types.ValType{types.S32, types.S32, types.S32}
	results := []types.Val{
		types.ValS32(10),
		types.ValS32(20),
		types.ValS32(30),
	}
	// Distinct opaque leading slots + trailing retptr. A regression
	// to stack[0] would read 0x1111 and corrupt memory at that addr.
	stack := []uint64{0x1111, 0x2222, 0x3333, uint64(ptr)}
	err := LowerResults(ctx, resultTypes, results, stack, true, 0)
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		v, ok := mem.ReadUint32Le(ptr + uint32(i*4))
		require.True(t, ok)
		require.Equal(t, uint32((i+1)*10), v)
	}
}
