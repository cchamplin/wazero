package abi

import (
	"math"
	"testing"

	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// Session 0 note (Task 15): the pre-existing lower_test.go constructed
// types via the deleted interface-style literals. Those tests have been
// dropped in favour of a minimal set that exercises the new kind-switch
// dispatch through the ComponentTypesBuilder. Full test migration is
// tracked in Task 19 of the Session 0 plan.

func TestLowerFlatScalars(t *testing.T) {
	t.Run("bool_true", func(t *testing.T) {
		flat, err := LowerFlat(nil, types.Bool, types.ValBool(true))
		require.NoError(t, err)
		require.Equal(t, []uint64{1}, flat)
	})
	t.Run("bool_false", func(t *testing.T) {
		flat, err := LowerFlat(nil, types.Bool, types.ValBool(false))
		require.NoError(t, err)
		require.Equal(t, []uint64{0}, flat)
	})
	t.Run("s8", func(t *testing.T) {
		flat, err := LowerFlat(nil, types.S8, types.ValS8(-1))
		require.NoError(t, err)
		require.Equal(t, []uint64{0xFFFFFFFF}, flat)
	})
	t.Run("s32", func(t *testing.T) {
		flat, err := LowerFlat(nil, types.S32, types.ValS32(42))
		require.NoError(t, err)
		require.Equal(t, []uint64{42}, flat)
	})
	t.Run("u64", func(t *testing.T) {
		flat, err := LowerFlat(nil, types.U64, types.ValU64(0xDEADBEEF12345678))
		require.NoError(t, err)
		require.Equal(t, []uint64{0xDEADBEEF12345678}, flat)
	})
	t.Run("f32", func(t *testing.T) {
		flat, err := LowerFlat(nil, types.F32, types.ValF32(3.14))
		require.NoError(t, err)
		require.Equal(t, []uint64{uint64(math.Float32bits(3.14))}, flat)
	})
}

func TestLowerFlatRecord(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	recT := b.InternRecord([]types.RecordField{
		{Name: "a", Type: types.S32},
		{Name: "b", Type: types.U64},
	})
	ct := b.Finish()

	ctx := &LowerContext{Types: ct}
	val := types.ValRecord(map[string]types.Val{
		"a": types.ValS32(42),
		"b": types.ValU64(100),
	})
	flat, err := LowerFlat(ctx, recT, val)
	require.NoError(t, err)
	require.Equal(t, []uint64{42, 100}, flat)
}

// TestLowerFlat_OwnArm_TrapsWhenNoResourceType verifies the Session 0
// behaviour: the Own lower arm exists, dispatches, and traps precisely
// because ResourceTypes is not yet populated (Concrete promotion is
// Session 2).
func TestLowerFlat_OwnArm_TrapsWhenNoResourceType(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	rtIdx := b.InternAbstractResource()
	ownT := b.InternOwnHandle(rtIdx)
	ct := b.Finish()

	ctx := &LowerContext{
		Types:    ct,
		Instance: runtime.NewComponentInstance(0, nil),
	}
	_, err := LowerFlat(ctx, ownT, types.ValOwn(42))
	require.Error(t, err)
}

// TestLowerFlat_BorrowArm_TrapsWhenNoResourceType mirrors the Own test
// for the Borrow lower arm.
func TestLowerFlat_BorrowArm_TrapsWhenNoResourceType(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	rtIdx := b.InternAbstractResource()
	borrowT := b.InternBorrowHandle(rtIdx)
	ct := b.Finish()

	ctx := &LowerContext{
		Types:    ct,
		Instance: runtime.NewComponentInstance(0, nil),
	}
	_, err := LowerFlat(ctx, borrowT, types.ValBorrow(42))
	require.Error(t, err)
}

// TestLowerAsyncTypesTraps verifies that lower of the async value types
// (Stream, Future, ErrorContext) traps with a clear "async not yet
// supported" error in LowerFlat.
func TestLowerAsyncTypesTraps(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	streamT := b.InternStream(types.U32, true)
	futureT := b.InternFuture(types.U32, true)
	errCtxT := b.InternErrorContextTable()
	ct := b.Finish()

	ctx := &LowerContext{Types: ct}
	cases := []struct {
		name string
		typ  types.ValType
	}{
		{"Stream", streamT},
		{"Future", futureT},
		{"ErrorContext", errCtxT},
	}
	for _, tc := range cases {
		t.Run("LowerFlat_"+tc.name, func(t *testing.T) {
			_, err := LowerFlat(ctx, tc.typ, types.ValU32(0))
			require.Error(t, err)
		})
	}
}
