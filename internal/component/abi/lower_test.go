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

// --- lower_borrow tests ---
//
// These test lowerBorrowHandleFlat against definitions.py:1645-1651.

// makeLowerCtx creates a LowerContext whose Instance (ID instID) has a single
// *runtime.ResourceType at ResourceTypes[0] with Impl set to implInst.
// The returned ComponentTypes has one Concrete ResourceTable entry at index 0
// pointing to Instance instID / Resource 0. If withCallContext is true,
// a CallContext is attached.
func makeLowerCtx(instID uint32, implInst *runtime.ComponentInstance, withCallContext bool) (*LowerContext, *runtime.ResourceType) {
	rt := &runtime.ResourceType{Impl: implInst}
	inst := runtime.NewComponentInstance(instID, nil)
	inst.ResourceTypes = []*runtime.ResourceType{rt}

	ct := &types.ComponentTypes{
		ResourceTables: []types.TypeResourceTable{
			{
				Concrete: true,
				Instance: types.RuntimeComponentInstanceIdx(instID),
				Resource: 0,
			},
		},
	}

	ctx := &LowerContext{
		Types:    ct,
		Instance: inst,
	}
	if withCallContext {
		ctx.CallContext = runtime.NewCallContext(inst.Table)
	}
	return ctx, rt
}

// lowerBorrowType returns a ValType for borrow<resource-0>.
func lowerBorrowType() types.ValType {
	return types.ValType{Kind: types.TypeKindBorrow, Index: 0}
}

// TestLowerBorrowSameInstance verifies the same-instance optimization from
// definitions.py:1647: if cx.inst is t.rt.impl, return rep directly.
// No handle should be allocated in the table and no borrow count changes.
func TestLowerBorrowSameInstance(t *testing.T) {
	// The instance IS the resource type's Impl — same-instance path.
	ctx, _ := makeLowerCtx(0, nil, true) // implInst will be set below

	// Set Impl to ctx.Instance itself so the pointer-identity check passes.
	ctx.Instance.ResourceTypes[0].Impl = ctx.Instance

	rep := uint32(42)
	flat, err := lowerBorrowHandleFlat(ctx, lowerBorrowType(), types.ValBorrow(rep))
	require.NoError(t, err)

	// Same-instance: returns rep directly.
	require.Equal(t, []uint64{uint64(rep)}, flat)

	// No handle was allocated in the table — table should be empty.
	// Try to get index 0; it should fail since nothing was added.
	_, _, gerr := ctx.Instance.Table.GetByIndex(0)
	require.Error(t, gerr)

	// Borrow count unchanged (still 0).
	require.Equal(t, 0, ctx.CallContext.NumBorrows())
}

// TestLowerBorrowCrossInstance verifies the cross-instance path from
// definitions.py:1648-1651: a borrow handle is allocated in the caller's
// table, CallContext is set on the handle entry, and borrow count is
// incremented.
func TestLowerBorrowCrossInstance(t *testing.T) {
	// Create the "other" instance that defines the resource.
	otherInst := runtime.NewComponentInstance(99, nil)

	// Create the calling instance (ID 0) with Impl pointing to otherInst.
	ctx, _ := makeLowerCtx(0, otherInst, true)

	rep := uint32(7)
	flat, err := lowerBorrowHandleFlat(ctx, lowerBorrowType(), types.ValBorrow(rep))
	require.NoError(t, err)

	// Cross-instance: returns a handle index (not the raw rep).
	require.Equal(t, 1, len(flat))
	handleIdx := uint32(flat[0])

	// The handle should be in the table.
	_, entry, gerr := ctx.Instance.Table.GetByIndex(handleIdx)
	require.NoError(t, gerr)

	resEntry, ok := entry.(*runtime.ResourceHandleEntry)
	require.True(t, ok)

	// Verify handle fields.
	require.Equal(t, rep, resEntry.Rep)
	require.False(t, resEntry.Own) // borrow, not own
	require.Equal(t, ctx.CallContext, resEntry.CallContext) // CallContext wired

	// Borrow count was incremented.
	require.Equal(t, 1, ctx.CallContext.NumBorrows())
}

// TestLowerBorrowCrossInstanceNoCallContext verifies that the cross-instance
// path traps when CallContext is nil, matching the spec assertion
// assert(isinstance(cx.borrow_scope, Task)) at definitions.py:1645.
func TestLowerBorrowCrossInstanceNoCallContext(t *testing.T) {
	otherInst := runtime.NewComponentInstance(99, nil)
	ctx, _ := makeLowerCtx(0, otherInst, false /* no CallContext */)

	_, err := lowerBorrowHandleFlat(ctx, lowerBorrowType(), types.ValBorrow(7))
	require.Error(t, err)
	require.Contains(t, err.Error(), "CallContext")
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
