package abi

import (
	"math"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestLiftFlatS32(t *testing.T) {
	iter := &FlatIter{values: []uint64{42}}
	val, err := LiftFlat(nil, types.S32{}, iter)
	require.NoError(t, err)
	require.Equal(t, int32(42), val.S32())
}

func TestLiftFlatU64(t *testing.T) {
	iter := &FlatIter{values: []uint64{0xDEADBEEF12345678}}
	val, err := LiftFlat(nil, types.U64{}, iter)
	require.NoError(t, err)
	require.Equal(t, uint64(0xDEADBEEF12345678), val.U64())
}

func TestLiftFlatF32(t *testing.T) {
	bits := math.Float32bits(3.14)
	iter := &FlatIter{values: []uint64{uint64(bits)}}
	val, err := LiftFlat(nil, types.F32{}, iter)
	require.NoError(t, err)
	// Use exact bit comparison for float32
	require.Equal(t, math.Float32bits(3.14), math.Float32bits(val.F32()))
}

func TestLiftFlatBool(t *testing.T) {
	iter := &FlatIter{values: []uint64{1}}
	val, err := LiftFlat(nil, types.Bool{}, iter)
	require.NoError(t, err)
	require.True(t, val.Bool())
}

func TestLiftFlatBoolFalse(t *testing.T) {
	iter := &FlatIter{values: []uint64{0}}
	val, err := LiftFlat(nil, types.Bool{}, iter)
	require.NoError(t, err)
	require.False(t, val.Bool())
}

func TestLiftFlatS8(t *testing.T) {
	iter := &FlatIter{values: []uint64{0xFFFFFF80}} // -128 sign extended
	val, err := LiftFlat(nil, types.S8{}, iter)
	require.NoError(t, err)
	require.Equal(t, int8(-128), val.S8())
}

func TestLiftFlatU8(t *testing.T) {
	iter := &FlatIter{values: []uint64{255}}
	val, err := LiftFlat(nil, types.U8{}, iter)
	require.NoError(t, err)
	require.Equal(t, uint8(255), val.U8())
}

func TestLiftFlatS16(t *testing.T) {
	iter := &FlatIter{values: []uint64{0xFFFF8000}} // -32768 sign extended
	val, err := LiftFlat(nil, types.S16{}, iter)
	require.NoError(t, err)
	require.Equal(t, int16(-32768), val.S16())
}

func TestLiftFlatU16(t *testing.T) {
	iter := &FlatIter{values: []uint64{65535}}
	val, err := LiftFlat(nil, types.U16{}, iter)
	require.NoError(t, err)
	require.Equal(t, uint16(65535), val.U16())
}

func TestLiftFlatU32(t *testing.T) {
	iter := &FlatIter{values: []uint64{0xDEADBEEF}}
	val, err := LiftFlat(nil, types.U32{}, iter)
	require.NoError(t, err)
	require.Equal(t, uint32(0xDEADBEEF), val.U32())
}

func TestLiftFlatS64(t *testing.T) {
	iter := &FlatIter{values: []uint64{0xFFFFFFFFFFFFFFFF}} // -1
	val, err := LiftFlat(nil, types.S64{}, iter)
	require.NoError(t, err)
	require.Equal(t, int64(-1), val.S64())
}

func TestLiftFlatF64(t *testing.T) {
	bits := math.Float64bits(3.14159265359)
	iter := &FlatIter{values: []uint64{bits}}
	val, err := LiftFlat(nil, types.F64{}, iter)
	require.NoError(t, err)
	require.Equal(t, 3.14159265359, val.F64())
}

func TestLiftFlatChar(t *testing.T) {
	iter := &FlatIter{values: []uint64{0x1F600}} // Unicode smiley face
	val, err := LiftFlat(nil, types.Char{}, iter)
	require.NoError(t, err)
	require.Equal(t, rune(0x1F600), val.Char())
}

func TestNewFlatIter(t *testing.T) {
	values := []uint64{1, 2, 3}
	iter := NewFlatIter(values)
	require.Equal(t, uint32(1), iter.NextI32())
	require.Equal(t, uint32(2), iter.NextI32())
	require.Equal(t, uint32(3), iter.NextI32())
}

func TestFlatIterNextI64(t *testing.T) {
	iter := &FlatIter{values: []uint64{0x123456789ABCDEF0}}
	require.Equal(t, uint64(0x123456789ABCDEF0), iter.NextI64())
}

func TestFlatIterNextF32(t *testing.T) {
	bits := math.Float32bits(2.5)
	iter := &FlatIter{values: []uint64{uint64(bits)}}
	require.Equal(t, float32(2.5), iter.NextF32())
}

func TestFlatIterNextF64(t *testing.T) {
	bits := math.Float64bits(2.5)
	iter := &FlatIter{values: []uint64{bits}}
	require.Equal(t, float64(2.5), iter.NextF64())
}

func TestLiftFlatRecord(t *testing.T) {
	// Record { a: s32, b: u64 }
	// Flat: [i32, i64]
	iter := NewFlatIter([]uint64{42, 100})
	recType := types.Record{
		Fields: []types.Field{
			{Name: "a", Type: types.S32{}},
			{Name: "b", Type: types.U64{}},
		},
	}

	val, err := LiftFlat(nil, recType, iter)
	require.NoError(t, err)
	require.Equal(t, component.ValKindRecord, val.Kind())

	rec := val.Record()
	require.Equal(t, int32(42), rec["a"].S32())
	require.Equal(t, uint64(100), rec["b"].U64())
}

func TestLiftFlatRecordNested(t *testing.T) {
	// Record { outer: Record { inner: s32 }, value: u64 }
	// Flat: [i32, i64]
	iter := NewFlatIter([]uint64{42, 100})
	innerType := types.Record{
		Fields: []types.Field{
			{Name: "inner", Type: types.S32{}},
		},
	}
	outerType := types.Record{
		Fields: []types.Field{
			{Name: "outer", Type: innerType},
			{Name: "value", Type: types.U64{}},
		},
	}

	val, err := LiftFlat(nil, outerType, iter)
	require.NoError(t, err)
	require.Equal(t, component.ValKindRecord, val.Kind())

	rec := val.Record()
	// Check outer field is a record
	outerVal := rec["outer"]
	require.Equal(t, component.ValKindRecord, outerVal.Kind())
	innerRec := outerVal.Record()
	require.Equal(t, int32(42), innerRec["inner"].S32())
	// Check value field
	require.Equal(t, uint64(100), rec["value"].U64())
}

func TestLiftFlatRecordEmpty(t *testing.T) {
	// Record {} - no fields
	iter := NewFlatIter([]uint64{})
	recType := types.Record{
		Fields: []types.Field{},
	}

	val, err := LiftFlat(nil, recType, iter)
	require.NoError(t, err)
	require.Equal(t, component.ValKindRecord, val.Kind())

	rec := val.Record()
	require.Equal(t, 0, len(rec))
}

func TestLiftFlatRecordFieldError(t *testing.T) {
	// Record with an unsupported field type (String not yet supported for flat lift)
	iter := NewFlatIter([]uint64{0, 0})
	recType := types.Record{
		Fields: []types.Field{
			{Name: "unsupported", Type: types.String{}},
		},
	}
	_, err := LiftFlat(nil, recType, iter)
	require.Error(t, err)
	require.Contains(t, err.Error(), "lift record field unsupported")
}
