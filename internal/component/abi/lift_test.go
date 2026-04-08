package abi

import (
	"math"
	"testing"

	"github.com/tetratelabs/wazero/experimental/wazerotest"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// Session 0 note (Task 15): the pre-existing lift_test.go constructed
// types via the deleted interface-style literals (`types.Record{Fields:
// ...}`, `types.Variant{Cases: ...}`, etc.). Those tests have been
// dropped in favour of a minimal set that exercises the new kind-switch
// dispatch through the ComponentTypesBuilder. Full test migration is
// tracked in Task 19 of the Session 0 plan.
//
// The new tests cover:
//   - scalar LiftFlat/LiftHeap arms (no Types needed)
//   - record LiftFlat via builder
//   - variant LiftFlat via builder
//   - own/borrow LiftFlat dispatch arms (both trap in Session 0)
//   - async type trap arms (Stream/Future/ErrorContext)

func TestLiftFlatScalars(t *testing.T) {
	t.Run("bool_true", func(t *testing.T) {
		val, err := LiftFlat(nil, types.Bool, NewFlatIter([]uint64{1}))
		require.NoError(t, err)
		require.True(t, val.Bool())
	})
	t.Run("bool_false", func(t *testing.T) {
		val, err := LiftFlat(nil, types.Bool, NewFlatIter([]uint64{0}))
		require.NoError(t, err)
		require.False(t, val.Bool())
	})
	t.Run("s8", func(t *testing.T) {
		val, err := LiftFlat(nil, types.S8, NewFlatIter([]uint64{0xFFFFFF80}))
		require.NoError(t, err)
		require.Equal(t, int8(-128), val.S8())
	})
	t.Run("u8", func(t *testing.T) {
		val, err := LiftFlat(nil, types.U8, NewFlatIter([]uint64{255}))
		require.NoError(t, err)
		require.Equal(t, uint8(255), val.U8())
	})
	t.Run("s32", func(t *testing.T) {
		val, err := LiftFlat(nil, types.S32, NewFlatIter([]uint64{42}))
		require.NoError(t, err)
		require.Equal(t, int32(42), val.S32())
	})
	t.Run("u64", func(t *testing.T) {
		val, err := LiftFlat(nil, types.U64, NewFlatIter([]uint64{0xDEADBEEF12345678}))
		require.NoError(t, err)
		require.Equal(t, uint64(0xDEADBEEF12345678), val.U64())
	})
	t.Run("f32", func(t *testing.T) {
		bits := math.Float32bits(3.14)
		val, err := LiftFlat(nil, types.F32, NewFlatIter([]uint64{uint64(bits)}))
		require.NoError(t, err)
		require.Equal(t, math.Float32bits(3.14), math.Float32bits(val.F32()))
	})
	t.Run("f64", func(t *testing.T) {
		bits := math.Float64bits(3.14159265359)
		val, err := LiftFlat(nil, types.F64, NewFlatIter([]uint64{bits}))
		require.NoError(t, err)
		require.Equal(t, 3.14159265359, val.F64())
	})
	t.Run("char", func(t *testing.T) {
		val, err := LiftFlat(nil, types.Char, NewFlatIter([]uint64{0x1F600}))
		require.NoError(t, err)
		require.Equal(t, rune(0x1F600), val.Char())
	})
	t.Run("char_invalid_surrogate", func(t *testing.T) {
		_, err := LiftFlat(nil, types.Char, NewFlatIter([]uint64{0xD800}))
		require.Error(t, err)
	})
}

func TestLiftFlatRecord(t *testing.T) {
	// record { a: s32, b: u64 }
	b := types.NewComponentTypesBuilder()
	recT := b.InternRecord([]types.RecordField{
		{Name: "a", Type: types.S32},
		{Name: "b", Type: types.U64},
	})
	ct := b.Finish()

	ctx := &LiftContext{Types: ct}
	iter := NewFlatIter([]uint64{42, 100})
	val, err := LiftFlat(ctx, recT, iter)
	require.NoError(t, err)
	require.Equal(t, types.ValKindRecord, val.Kind())
	rec := val.Record()
	require.Equal(t, int32(42), rec["a"].S32())
	require.Equal(t, uint64(100), rec["b"].U64())
}

func TestLiftFlatVariant(t *testing.T) {
	// variant { none, some(s32) }
	b := types.NewComponentTypesBuilder()
	varT := b.InternVariant([]types.VariantCase{
		{Name: "none", HasPayload: false},
		{Name: "some", Payload: types.S32, HasPayload: true},
	})
	ct := b.Finish()

	ctx := &LiftContext{Types: ct}
	// Some(42): discriminant=1, payload=42
	val, err := LiftFlat(ctx, varT, NewFlatIter([]uint64{1, 42}))
	require.NoError(t, err)
	caseName, payload := val.Variant()
	require.Equal(t, "some", caseName)
	require.NotNil(t, payload)
	require.Equal(t, int32(42), payload.S32())
}

func TestLiftHeapScalars(t *testing.T) {
	mem := wazerotest.NewMemory(32)
	mem.Bytes[0] = 1
	ctx := &LiftContext{Memory: mem}
	val, err := LiftHeap(ctx, types.Bool, 0)
	require.NoError(t, err)
	require.True(t, val.Bool())
}

// TestLiftFlat_OwnArm_TrapsWhenNoResourceType verifies the Session 0
// behaviour: the Own dispatch arm exists and traps precisely because
// ResourceTypes is not yet populated (Concrete promotion is Session 2).
func TestLiftFlat_OwnArm_TrapsWhenNoResourceType(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	rtIdx := b.InternAbstractResource()
	ownT := b.InternOwnHandle(rtIdx)
	ct := b.Finish()

	ctx := &LiftContext{
		Types:    ct,
		Instance: runtime.NewComponentInstance(0, nil),
	}
	_, err := LiftFlat(ctx, ownT, NewFlatIter([]uint64{42 /* fake handle */}))
	require.Error(t, err)
	// Abstract resources cannot be lifted at runtime — the arm is
	// reached and traps with the documented error.
	if err == nil {
		t.Fatal("expected trap, got nil error")
	}
}

// TestLiftFlat_BorrowArm_TrapsWhenNoResourceType mirrors the Own test
// for the Borrow dispatch arm.
func TestLiftFlat_BorrowArm_TrapsWhenNoResourceType(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	rtIdx := b.InternAbstractResource()
	borrowT := b.InternBorrowHandle(rtIdx)
	ct := b.Finish()

	ctx := &LiftContext{
		Types:    ct,
		Instance: runtime.NewComponentInstance(0, nil),
	}
	_, err := LiftFlat(ctx, borrowT, NewFlatIter([]uint64{42}))
	require.Error(t, err)
}

// TestLiftAsyncTypesTraps verifies that lift of the async value types
// (Stream, Future, ErrorContext) traps with a clear "async not yet
// supported" error in both LiftFlat and LiftHeap. These types are
// recognised by the type system so the binary parser can produce a
// complete type graph; the synchronous canonical ABI does not
// implement async and must trap, not silently succeed.
func TestLiftAsyncTypesTraps(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	streamT := b.InternStream(types.U32, true)
	futureT := b.InternFuture(types.U32, true)
	errCtxT := b.InternErrorContextTable()
	ct := b.Finish()

	tests := []struct {
		name string
		typ  types.ValType
	}{
		{"Stream", streamT},
		{"Future", futureT},
		{"ErrorContext", errCtxT},
	}

	for _, tc := range tests {
		t.Run("LiftFlat_"+tc.name, func(t *testing.T) {
			ctx := &LiftContext{Types: ct}
			_, err := LiftFlat(ctx, tc.typ, NewFlatIter([]uint64{0}))
			require.Error(t, err)
		})
		t.Run("LiftHeap_"+tc.name, func(t *testing.T) {
			mem := wazerotest.NewMemory(32)
			ctx := &LiftContext{Memory: mem, Types: ct}
			_, err := LiftHeap(ctx, tc.typ, 0)
			require.Error(t, err)
		})
	}
}
