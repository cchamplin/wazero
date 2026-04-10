package abi

import (
	"math"
	"testing"

	"github.com/tetratelabs/wazero/experimental/wazerotest"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestLowerParamsFlatPath asserts LowerParams takes the flat path when
// the flattened parameter count is <= MaxFlatParams. The may_leave
// toggle is applied by the caller (buildCanonLiftFunc), not by
// LowerParams itself — the helper is pure with respect to instance
// state so test harnesses can share it.
//
// Spec: definitions.py:1954-1974 lower_flat_values.
// Canonical test: run_tests.py test_flatten cases exercising small
// parameter signatures.
func TestLowerParamsFlatPath(t *testing.T) {
	ct := types.NewComponentTypesBuilder().Finish()
	inst := runtime.NewComponentInstance(0, nil)
	mem := wazerotest.NewMemory(wazerotest.PageSize)
	ctx := &LowerContext{
		Memory:   mem,
		Types:    ct,
		Instance: inst,
	}
	// Two i32 params → 2 flat values → under MaxFlatParams=16.
	paramTypes := []types.ValType{types.S32, types.S32}
	args := []types.Val{types.ValS32(42), types.ValS32(-7)}
	flat, err := LowerParams(ctx, paramTypes, args, MaxFlatParams)
	require.NoError(t, err)
	require.Equal(t, 2, len(flat))
	require.Equal(t, int32(42), int32(flat[0]))
	require.Equal(t, int32(-7), int32(flat[1]))
}

// TestLowerParamsRetptrPath exercises the aggregate-boundary retptr
// path: 3 S32 params with maxFlat=2 forces `len(flatTypes) > maxFlat`,
// so LowerParams must realloc a tuple buffer, store each arg via
// LowerHeap at ptr + i*4, and return a single-element flat slice
// containing the pointer.
//
// Spec: definitions.py:1961-1968 lower_flat_values retptr branch
// (realloc + store + return single ptr).
// Canonical test: run_tests.py test_flatten 17-param case at :395.
func TestLowerParamsRetptrPath(t *testing.T) {
	bag := types.NewComponentTypesBuilder().Finish()
	inst := runtime.NewComponentInstance(0, nil)
	mem := wazerotest.NewMemory(wazerotest.PageSize)
	// Wire a fake realloc that returns a fixed pointer and tracks calls.
	var reallocCalls int
	const fixedPtr uint32 = 1024
	realloc := func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
		reallocCalls++
		return fixedPtr, nil
	}
	ctx := &LowerContext{
		Memory:   mem,
		Realloc:  realloc,
		Types:    bag,
		Instance: inst,
	}
	// 3 i32 params with maxFlat=2 → retptr path.
	paramTypes := []types.ValType{types.S32, types.S32, types.S32}
	args := []types.Val{types.ValS32(1), types.ValS32(2), types.ValS32(3)}
	flat, err := LowerParams(ctx, paramTypes, args, 2)
	require.NoError(t, err)
	require.Equal(t, 1, len(flat))
	require.Equal(t, fixedPtr, uint32(flat[0]))
	require.Equal(t, 1, reallocCalls)
	// Verify each arg was written to memory at ptr + i*4.
	for i := 0; i < 3; i++ {
		v, ok := mem.ReadUint32Le(fixedPtr + uint32(i*4))
		require.True(t, ok)
		require.Equal(t, int32(i+1), int32(v))
	}
}

// TestLowerParamsFlatPathAtBoundary addresses C1 reviewer's M3: verify
// that the flat path is taken when the flattened width is EXACTLY
// MaxFlatParams (16). Realloc is nil so any accidental retptr spill
// would surface as a clear error rather than silently passing.
//
// Spec: definitions.py:1957 `if len(flat_types) > max_flat` — the
// boundary is strict >, so 16 == MaxFlatParams stays flat.
func TestLowerParamsFlatPathAtBoundary(t *testing.T) {
	ct := types.NewComponentTypesBuilder().Finish()
	inst := runtime.NewComponentInstance(0, nil)
	mem := wazerotest.NewMemory(wazerotest.PageSize)
	ctx := &LowerContext{
		Memory:   mem,
		Types:    ct,
		Instance: inst,
		// Realloc intentionally nil — the retptr path would trip on it.
	}
	paramTypes := make([]types.ValType, MaxFlatParams)
	args := make([]types.Val, MaxFlatParams)
	for i := range paramTypes {
		paramTypes[i] = types.S32
		args[i] = types.ValS32(int32(i))
	}
	flat, err := LowerParams(ctx, paramTypes, args, MaxFlatParams)
	require.NoError(t, err)
	require.Equal(t, MaxFlatParams, len(flat))
	for i, v := range flat {
		require.Equal(t, int32(i), int32(v))
	}
}

// TestParamsTupleLayout covers the shared tuple-of-params layout
// helper used by the retptr paths of LowerParams / LiftParams /
// LiftResults / LowerResults. The algorithm mirrors computeRecordABI
// at internal/component/types/abi_info.go:107-133 and implements the
// spec's alignment(tuple_type) / elem_size(tuple_type) on the synthetic
// TupleType(ts). In particular, the `S32 + u8` case exercises the
// trailing alignTo(size, tupleAlign) tail path (3 bytes of trailing
// padding to round 5 → 8), which was previously untested.
//
// Spec: definitions.py alignment(tuple_type) /
// elem_size(tuple_type); algorithm mirror of
// internal/component/types/abi_info.go:107-133 computeRecordABI.
func TestParamsTupleLayout(t *testing.T) {
	ct := types.NewComponentTypesBuilder().Finish()
	tests := []struct {
		name       string
		elems      []types.ValType
		wantSize   uint32
		wantAlign  uint32
		wantOffset []uint32
	}{
		{
			name:       "empty",
			elems:      nil,
			wantSize:   0,
			wantAlign:  1,
			wantOffset: []uint32{},
		},
		{
			name:       "single u8",
			elems:      []types.ValType{types.U8},
			wantSize:   1,
			wantAlign:  1,
			wantOffset: []uint32{0},
		},
		{
			name:       "two S32",
			elems:      []types.ValType{types.S32, types.S32},
			wantSize:   8,
			wantAlign:  4,
			wantOffset: []uint32{0, 4},
		},
		{
			name:       "mixed u8+S32 (3 bytes leading padding)",
			elems:      []types.ValType{types.U8, types.S32},
			wantSize:   8,
			wantAlign:  4,
			wantOffset: []uint32{0, 4},
		},
		{
			name:       "mixed S32+u8 (3 bytes trailing padding)",
			elems:      []types.ValType{types.S32, types.U8},
			wantSize:   8,
			wantAlign:  4,
			wantOffset: []uint32{0, 4},
		},
		{
			name:       "three S64",
			elems:      []types.ValType{types.S64, types.S64, types.S64},
			wantSize:   24,
			wantAlign:  8,
			wantOffset: []uint32{0, 8, 16},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			size, align, offsets := paramsTupleLayout(ct, tc.elems)
			require.Equal(t, tc.wantSize, size)
			require.Equal(t, tc.wantAlign, align)
			require.Equal(t, len(tc.wantOffset), len(offsets))
			for i, want := range tc.wantOffset {
				require.Equal(t, want, offsets[i])
			}
		})
	}
}

// TestLowerParamsFlatPathMixedKinds verifies the flat lowering path
// correctly concatenates differently-shaped flat representations from
// mixed primitive kinds: S32 occupies one i32 slot, F64 occupies one
// i64 slot (as Float64bits), and U8 occupies one i32 slot. All three
// should appear in order in the flat output.
//
// Spec: definitions.py:1954-1974 lower_flat_values flat branch,
// lower_flat for each primitive at definitions.py:1811-1820.
func TestLowerParamsFlatPathMixedKinds(t *testing.T) {
	ct := types.NewComponentTypesBuilder().Finish()
	inst := runtime.NewComponentInstance(0, nil)
	mem := wazerotest.NewMemory(wazerotest.PageSize)
	ctx := &LowerContext{
		Memory:   mem,
		Types:    ct,
		Instance: inst,
	}
	paramTypes := []types.ValType{types.S32, types.F64, types.U8}
	args := []types.Val{
		types.ValS32(-42),
		types.ValF64(3.14159),
		types.ValU8(0xAB),
	}
	flat, err := LowerParams(ctx, paramTypes, args, MaxFlatParams)
	require.NoError(t, err)
	require.Equal(t, 3, len(flat))
	require.Equal(t, int32(-42), int32(flat[0]))
	require.Equal(t, math.Float64bits(3.14159), flat[1])
	require.Equal(t, uint32(0xAB), uint32(flat[2]))
}

// TestLiftParamsFlatPathMixedKinds verifies the flat lifting path
// round-trips the mixed-kind encoding produced by
// TestLowerParamsFlatPathMixedKinds: S32 + F64 + U8 as three flat
// slots consumed in order by LiftFlat.
//
// Spec: definitions.py:1943-1952 lift_flat_values flat branch.
func TestLiftParamsFlatPathMixedKinds(t *testing.T) {
	ct := types.NewComponentTypesBuilder().Finish()
	inst := runtime.NewComponentInstance(0, nil)
	mem := wazerotest.NewMemory(wazerotest.PageSize)
	ctx := &LiftContext{
		Memory:      mem,
		Types:       ct,
		Instance:    inst,
		CallContext: runtime.NewCallContext(inst.Table),
	}
	paramTypes := []types.ValType{types.S32, types.F64, types.U8}
	neg42 := int32(-42)
	flat := []uint64{
		uint64(uint32(neg42)),
		math.Float64bits(3.14159),
		0xAB,
	}
	vals, err := LiftParams(ctx, paramTypes, flat, MaxFlatParams)
	require.NoError(t, err)
	require.Equal(t, 3, len(vals))
	require.Equal(t, int32(-42), vals[0].S32())
	require.Equal(t, 3.14159, vals[1].F64())
	require.Equal(t, uint8(0xAB), vals[2].U8())
}

// TestLowerParamsEmpty and TestLiftParamsEmpty cover the degenerate
// zero-param case. Both must return empty slices without error and
// without touching realloc or memory.
//
// Spec: definitions.py:1943-1974 lift_flat_values / lower_flat_values
// with an empty type list — flatten returns [] and the flat branch
// trivially produces an empty result.
func TestLowerParamsEmpty(t *testing.T) {
	ct := types.NewComponentTypesBuilder().Finish()
	inst := runtime.NewComponentInstance(0, nil)
	mem := wazerotest.NewMemory(wazerotest.PageSize)
	ctx := &LowerContext{
		Memory:   mem,
		Types:    ct,
		Instance: inst,
	}
	flat, err := LowerParams(ctx, nil, nil, MaxFlatParams)
	require.NoError(t, err)
	require.Equal(t, 0, len(flat))
}

// TestLiftParamsEmpty covers the zero-param lift case.
//
// Spec: definitions.py:1943-1952 lift_flat_values with empty ts.
func TestLiftParamsEmpty(t *testing.T) {
	ct := types.NewComponentTypesBuilder().Finish()
	inst := runtime.NewComponentInstance(0, nil)
	mem := wazerotest.NewMemory(wazerotest.PageSize)
	ctx := &LiftContext{
		Memory:      mem,
		Types:       ct,
		Instance:    inst,
		CallContext: runtime.NewCallContext(inst.Table),
	}
	vals, err := LiftParams(ctx, nil, nil, MaxFlatParams)
	require.NoError(t, err)
	require.Equal(t, 0, len(vals))
}

// TestLowerResultsLengthMismatch asserts LowerResults reports an error
// when the number of result values does not match the number of
// result types.
//
// Spec: definitions.py:2104-2113 canon_lower return path — the caller
// contract is that results and resultTypes are same-length (the host
// Go function produced exactly len(resultTypes) values).
func TestLowerResultsLengthMismatch(t *testing.T) {
	ct := types.NewComponentTypesBuilder().Finish()
	inst := runtime.NewComponentInstance(0, nil)
	mem := wazerotest.NewMemory(wazerotest.PageSize)
	ctx := &LowerContext{
		Memory:   mem,
		Types:    ct,
		Instance: inst,
	}
	resultTypes := []types.ValType{types.S32, types.S32}
	results := []types.Val{types.ValS32(1)} // only 1 value for 2 types
	stack := make([]uint64, 2)
	err := LowerResults(ctx, resultTypes, results, stack, false, MaxFlatResults)
	require.Error(t, err)
}
