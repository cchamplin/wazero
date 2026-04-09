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
