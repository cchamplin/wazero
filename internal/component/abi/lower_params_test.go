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
