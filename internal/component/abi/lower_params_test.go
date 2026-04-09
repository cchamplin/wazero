package abi

import (
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
