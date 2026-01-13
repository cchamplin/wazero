package abi

import (
	"encoding/binary"
	"fmt"
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

func TestLiftFlatVariant(t *testing.T) {
	// variant { none, some(s32) }
	// Flat for some(42): [i32(case=1), i32(payload=42)]
	iter := NewFlatIter([]uint64{1, 42})
	varType := types.Variant{
		Cases: []types.Case{
			{Name: "none", Type: nil},
			{Name: "some", Type: types.S32{}},
		},
	}

	val, err := LiftFlat(nil, varType, iter)
	require.NoError(t, err)

	caseName, payload := val.Variant()
	require.Equal(t, "some", caseName)
	require.NotNil(t, payload)
	require.Equal(t, int32(42), payload.S32())
}

func TestLiftFlatVariantNoPayload(t *testing.T) {
	// variant { none, some(s32) }
	// Flat for none: [i32(case=0), i32(padding=0)]
	iter := NewFlatIter([]uint64{0, 0})
	varType := types.Variant{
		Cases: []types.Case{
			{Name: "none", Type: nil},
			{Name: "some", Type: types.S32{}},
		},
	}

	val, err := LiftFlat(nil, varType, iter)
	require.NoError(t, err)

	caseName, payload := val.Variant()
	require.Equal(t, "none", caseName)
	require.Nil(t, payload)
}

func TestLiftFlatVariantInvalidDiscriminant(t *testing.T) {
	// variant { none, some(s32) }
	// Invalid discriminant: 5 (only 0 and 1 are valid)
	iter := NewFlatIter([]uint64{5, 0})
	varType := types.Variant{
		Cases: []types.Case{
			{Name: "none", Type: nil},
			{Name: "some", Type: types.S32{}},
		},
	}

	_, err := LiftFlat(nil, varType, iter)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid variant discriminant")
}

func TestLiftFlatVariantMultiValuePayload(t *testing.T) {
	// variant { none, pair(record { a: s32, b: s32 }) }
	// Flat for pair({10, 20}): [i32(case=1), i32(a=10), i32(b=20)]
	iter := NewFlatIter([]uint64{1, 10, 20})
	varType := types.Variant{
		Cases: []types.Case{
			{Name: "none", Type: nil},
			{Name: "pair", Type: types.Record{
				Fields: []types.Field{
					{Name: "a", Type: types.S32{}},
					{Name: "b", Type: types.S32{}},
				},
			}},
		},
	}

	val, err := LiftFlat(nil, varType, iter)
	require.NoError(t, err)

	caseName, payload := val.Variant()
	require.Equal(t, "pair", caseName)
	require.NotNil(t, payload)
	rec := payload.Record()
	require.Equal(t, int32(10), rec["a"].S32())
	require.Equal(t, int32(20), rec["b"].S32())
}

// --- Tuple Tests ---

func TestLiftFlatTuple(t *testing.T) {
	// tuple<s32, u64>
	iter := NewFlatIter([]uint64{42, 100})
	tupleType := types.Tuple{
		Types: []types.ValType{types.S32{}, types.U64{}},
	}
	val, err := LiftFlat(nil, tupleType, iter)
	require.NoError(t, err)
	require.Equal(t, component.ValKindTuple, val.Kind())
	elems := val.Tuple()
	require.Equal(t, 2, len(elems))
	require.Equal(t, int32(42), elems[0].S32())
	require.Equal(t, uint64(100), elems[1].U64())
}

func TestLiftFlatTupleEmpty(t *testing.T) {
	// tuple<>
	iter := NewFlatIter([]uint64{})
	tupleType := types.Tuple{
		Types: []types.ValType{},
	}
	val, err := LiftFlat(nil, tupleType, iter)
	require.NoError(t, err)
	require.Equal(t, component.ValKindTuple, val.Kind())
	elems := val.Tuple()
	require.Equal(t, 0, len(elems))
}

func TestLiftFlatTupleNested(t *testing.T) {
	// tuple<s32, tuple<u64, bool>>
	iter := NewFlatIter([]uint64{42, 100, 1})
	innerTuple := types.Tuple{Types: []types.ValType{types.U64{}, types.Bool{}}}
	outerTuple := types.Tuple{Types: []types.ValType{types.S32{}, innerTuple}}

	val, err := LiftFlat(nil, outerTuple, iter)
	require.NoError(t, err)
	require.Equal(t, component.ValKindTuple, val.Kind())
	elems := val.Tuple()
	require.Equal(t, 2, len(elems))
	require.Equal(t, int32(42), elems[0].S32())
	// Inner tuple
	innerElems := elems[1].Tuple()
	require.Equal(t, uint64(100), innerElems[0].U64())
	require.True(t, innerElems[1].Bool())
}

// --- Option Tests ---

func TestLiftFlatOptionSome(t *testing.T) {
	// option<s32> as Some(42)
	iter := NewFlatIter([]uint64{1, 42})
	optType := types.Option{Some: types.S32{}}
	val, err := LiftFlat(nil, optType, iter)
	require.NoError(t, err)
	require.Equal(t, component.ValKindOption, val.Kind())
	payload := val.Option()
	require.NotNil(t, payload)
	require.Equal(t, int32(42), payload.S32())
}

func TestLiftFlatOptionNone(t *testing.T) {
	// option<s32> as None
	iter := NewFlatIter([]uint64{0, 0}) // discriminant=0, padding
	optType := types.Option{Some: types.S32{}}
	val, err := LiftFlat(nil, optType, iter)
	require.NoError(t, err)
	require.Equal(t, component.ValKindOption, val.Kind())
	require.Nil(t, val.Option())
}

func TestLiftFlatOptionWithRecord(t *testing.T) {
	// option<record { a: s32, b: u64 }> as Some({10, 20})
	iter := NewFlatIter([]uint64{1, 10, 20})
	recType := types.Record{
		Fields: []types.Field{
			{Name: "a", Type: types.S32{}},
			{Name: "b", Type: types.U64{}},
		},
	}
	optType := types.Option{Some: recType}
	val, err := LiftFlat(nil, optType, iter)
	require.NoError(t, err)
	payload := val.Option()
	require.NotNil(t, payload)
	rec := payload.Record()
	require.Equal(t, int32(10), rec["a"].S32())
	require.Equal(t, uint64(20), rec["b"].U64())
}

func TestLiftFlatOptionUnitSome(t *testing.T) {
	// option<> (unit option) as Some
	iter := NewFlatIter([]uint64{1}) // discriminant=1, no payload
	optType := types.Option{Some: nil}
	val, err := LiftFlat(nil, optType, iter)
	require.NoError(t, err)
	payload := val.Option()
	require.NotNil(t, payload) // Some(unit) is not nil
}

// --- Result Tests ---

func TestLiftFlatResultOk(t *testing.T) {
	// result<s32, u32> as Ok(42)
	iter := NewFlatIter([]uint64{0, 42})
	resType := types.Result{Ok: types.S32{}, Error: types.U32{}}
	val, err := LiftFlat(nil, resType, iter)
	require.NoError(t, err)
	require.Equal(t, component.ValKindResult, val.Kind())
	isOk, ok, errVal := val.Result()
	require.True(t, isOk)
	require.NotNil(t, ok)
	require.Nil(t, errVal)
	require.Equal(t, int32(42), ok.S32())
}

func TestLiftFlatResultErr(t *testing.T) {
	// result<s32, u32> as Err(99)
	iter := NewFlatIter([]uint64{1, 99})
	resType := types.Result{Ok: types.S32{}, Error: types.U32{}}
	val, err := LiftFlat(nil, resType, iter)
	require.NoError(t, err)
	require.Equal(t, component.ValKindResult, val.Kind())
	isOk, ok, errVal := val.Result()
	require.False(t, isOk)
	require.Nil(t, ok)
	require.NotNil(t, errVal)
	require.Equal(t, uint32(99), errVal.U32())
}

func TestLiftFlatResultOkNoPayload(t *testing.T) {
	// result<_, u32> as Ok (no payload)
	iter := NewFlatIter([]uint64{0, 0}) // discriminant=0, padding for error type
	resType := types.Result{Ok: nil, Error: types.U32{}}
	val, err := LiftFlat(nil, resType, iter)
	require.NoError(t, err)
	isOk, ok, errVal := val.Result()
	require.True(t, isOk)
	require.Nil(t, ok)
	require.Nil(t, errVal)
}

func TestLiftFlatResultErrNoPayload(t *testing.T) {
	// result<s32, _> as Err (no payload)
	iter := NewFlatIter([]uint64{1, 0}) // discriminant=1, padding for ok type
	resType := types.Result{Ok: types.S32{}, Error: nil}
	val, err := LiftFlat(nil, resType, iter)
	require.NoError(t, err)
	isOk, ok, errVal := val.Result()
	require.False(t, isOk)
	require.Nil(t, ok)
	require.Nil(t, errVal)
}

func TestLiftFlatResultWithDifferentPayloadSizes(t *testing.T) {
	// result<u64, u32> - ok is larger than error
	// Ok(12345678901234)
	iter := NewFlatIter([]uint64{0, 12345678901234})
	resType := types.Result{Ok: types.U64{}, Error: types.U32{}}
	val, err := LiftFlat(nil, resType, iter)
	require.NoError(t, err)
	isOk, ok, _ := val.Result()
	require.True(t, isOk)
	require.Equal(t, uint64(12345678901234), ok.U64())
}

// --- Enum Tests ---

func TestLiftFlatEnum(t *testing.T) {
	// enum { a, b, c }
	iter := NewFlatIter([]uint64{1})
	enumType := types.Enum{Cases: []string{"a", "b", "c"}}
	val, err := LiftFlat(nil, enumType, iter)
	require.NoError(t, err)
	require.Equal(t, component.ValKindEnum, val.Kind())
	require.Equal(t, "b", val.Enum())
}

func TestLiftFlatEnumFirstCase(t *testing.T) {
	// enum { first, second, third } - case 0
	iter := NewFlatIter([]uint64{0})
	enumType := types.Enum{Cases: []string{"first", "second", "third"}}
	val, err := LiftFlat(nil, enumType, iter)
	require.NoError(t, err)
	require.Equal(t, "first", val.Enum())
}

func TestLiftFlatEnumLastCase(t *testing.T) {
	// enum { a, b, c } - case 2
	iter := NewFlatIter([]uint64{2})
	enumType := types.Enum{Cases: []string{"a", "b", "c"}}
	val, err := LiftFlat(nil, enumType, iter)
	require.NoError(t, err)
	require.Equal(t, "c", val.Enum())
}

func TestLiftFlatEnumInvalidDiscriminant(t *testing.T) {
	// enum { a, b, c } - invalid discriminant 5
	iter := NewFlatIter([]uint64{5})
	enumType := types.Enum{Cases: []string{"a", "b", "c"}}
	_, err := LiftFlat(nil, enumType, iter)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid enum discriminant")
}

// --- Flags Tests ---

func TestLiftFlatFlags(t *testing.T) {
	// flags { read, write, execute } with read|execute set (bits 0 and 2)
	iter := NewFlatIter([]uint64{0b101})
	flagsType := types.Flags{Names: []string{"read", "write", "execute"}}
	val, err := LiftFlat(nil, flagsType, iter)
	require.NoError(t, err)
	require.Equal(t, component.ValKindFlags, val.Kind())
	flags := val.Flags()
	require.True(t, flags["read"])
	require.False(t, flags["write"])
	require.True(t, flags["execute"])
}

func TestLiftFlatFlagsAllSet(t *testing.T) {
	// flags { a, b, c } with all set (bits 0, 1, 2)
	iter := NewFlatIter([]uint64{0b111})
	flagsType := types.Flags{Names: []string{"a", "b", "c"}}
	val, err := LiftFlat(nil, flagsType, iter)
	require.NoError(t, err)
	flags := val.Flags()
	require.True(t, flags["a"])
	require.True(t, flags["b"])
	require.True(t, flags["c"])
}

func TestLiftFlatFlagsNoneSet(t *testing.T) {
	// flags { a, b, c } with none set
	iter := NewFlatIter([]uint64{0})
	flagsType := types.Flags{Names: []string{"a", "b", "c"}}
	val, err := LiftFlat(nil, flagsType, iter)
	require.NoError(t, err)
	flags := val.Flags()
	require.False(t, flags["a"])
	require.False(t, flags["b"])
	require.False(t, flags["c"])
}

func TestLiftFlatFlagsEmpty(t *testing.T) {
	// flags {} - no flags defined
	iter := NewFlatIter([]uint64{})
	flagsType := types.Flags{Names: []string{}}
	val, err := LiftFlat(nil, flagsType, iter)
	require.NoError(t, err)
	flags := val.Flags()
	require.Equal(t, 0, len(flags))
}

func TestLiftFlatFlagsMany(t *testing.T) {
	// flags with 32 flags (uses single i32)
	names := make([]string, 32)
	for i := 0; i < 32; i++ {
		names[i] = fmt.Sprintf("flag%d", i)
	}
	// Set bits 0, 15, 31
	iter := NewFlatIter([]uint64{(1 << 0) | (1 << 15) | (1 << 31)})
	flagsType := types.Flags{Names: names}
	val, err := LiftFlat(nil, flagsType, iter)
	require.NoError(t, err)
	flags := val.Flags()
	require.True(t, flags["flag0"])
	require.False(t, flags["flag1"])
	require.True(t, flags["flag15"])
	require.False(t, flags["flag16"])
	require.True(t, flags["flag31"])
}

func TestLiftFlatFlagsMoreThan32(t *testing.T) {
	// flags with 64 flags (uses two i32s)
	names := make([]string, 64)
	for i := 0; i < 64; i++ {
		names[i] = fmt.Sprintf("flag%d", i)
	}
	// Set bits 0, 31 in first i32 and bit 0, 31 in second i32 (flags 32, 63)
	iter := NewFlatIter([]uint64{(1 << 0) | (1 << 31), (1 << 0) | (1 << 31)})
	flagsType := types.Flags{Names: names}
	val, err := LiftFlat(nil, flagsType, iter)
	require.NoError(t, err)
	flags := val.Flags()
	require.True(t, flags["flag0"])
	require.True(t, flags["flag31"])
	require.True(t, flags["flag32"])
	require.True(t, flags["flag63"])
	require.False(t, flags["flag1"])
	require.False(t, flags["flag33"])
}

// --- List Tests ---

func TestLiftFlatList(t *testing.T) {
	// list<s32> with ptr=100, len=5
	iter := NewFlatIter([]uint64{100, 5})
	listType := types.List{Element: types.S32{}}
	val, err := LiftFlat(nil, listType, iter)
	require.NoError(t, err)
	// For flat lift, we return an empty list placeholder
	// The actual elements need heap access (to be implemented later)
	require.Equal(t, component.ValKindList, val.Kind())
}

func TestLiftFlatListPtrAndLen(t *testing.T) {
	// list<u64> with ptr=0x1000, len=10
	iter := NewFlatIter([]uint64{0x1000, 10})
	listType := types.List{Element: types.U64{}}
	val, err := LiftFlat(nil, listType, iter)
	require.NoError(t, err)
	require.Equal(t, component.ValKindList, val.Kind())
	// Empty list for now (elements deferred to heap lift)
	list := val.List()
	require.Equal(t, 0, len(list))
}

// --- LiftHeap Primitive Tests ---

func TestLiftHeapBool(t *testing.T) {
	data := make([]byte, 16)
	data[4] = 1 // true at offset 4
	ctx := &LiftContext{Memory: &mockMemory{data: data}}
	val, err := LiftHeap(ctx, types.Bool{}, 4)
	require.NoError(t, err)
	require.True(t, val.Bool())
}

func TestLiftHeapBoolFalse(t *testing.T) {
	data := make([]byte, 16)
	data[8] = 0 // false at offset 8
	ctx := &LiftContext{Memory: &mockMemory{data: data}}
	val, err := LiftHeap(ctx, types.Bool{}, 8)
	require.NoError(t, err)
	require.False(t, val.Bool())
}

func TestLiftHeapU8(t *testing.T) {
	data := make([]byte, 16)
	data[3] = 0x42
	ctx := &LiftContext{Memory: &mockMemory{data: data}}
	val, err := LiftHeap(ctx, types.U8{}, 3)
	require.NoError(t, err)
	require.Equal(t, uint8(0x42), val.U8())
}

func TestLiftHeapS8(t *testing.T) {
	data := make([]byte, 16)
	data[5] = 0x80 // -128
	ctx := &LiftContext{Memory: &mockMemory{data: data}}
	val, err := LiftHeap(ctx, types.S8{}, 5)
	require.NoError(t, err)
	require.Equal(t, int8(-128), val.S8())
}

func TestLiftHeapU16(t *testing.T) {
	data := make([]byte, 16)
	binary.LittleEndian.PutUint16(data[2:], 0x1234)
	ctx := &LiftContext{Memory: &mockMemory{data: data}}
	val, err := LiftHeap(ctx, types.U16{}, 2)
	require.NoError(t, err)
	require.Equal(t, uint16(0x1234), val.U16())
}

func TestLiftHeapS16(t *testing.T) {
	data := make([]byte, 16)
	binary.LittleEndian.PutUint16(data[6:], 0x8000) // -32768
	ctx := &LiftContext{Memory: &mockMemory{data: data}}
	val, err := LiftHeap(ctx, types.S16{}, 6)
	require.NoError(t, err)
	require.Equal(t, int16(-32768), val.S16())
}

func TestLiftHeapU32(t *testing.T) {
	data := make([]byte, 16)
	binary.LittleEndian.PutUint32(data[4:], 0xDEADBEEF)
	ctx := &LiftContext{Memory: &mockMemory{data: data}}
	val, err := LiftHeap(ctx, types.U32{}, 4)
	require.NoError(t, err)
	require.Equal(t, uint32(0xDEADBEEF), val.U32())
}

func TestLiftHeapS32(t *testing.T) {
	data := make([]byte, 16)
	binary.LittleEndian.PutUint32(data[8:], 0xFFFFFFFF) // -1
	ctx := &LiftContext{Memory: &mockMemory{data: data}}
	val, err := LiftHeap(ctx, types.S32{}, 8)
	require.NoError(t, err)
	require.Equal(t, int32(-1), val.S32())
}

func TestLiftHeapU64(t *testing.T) {
	data := make([]byte, 32)
	binary.LittleEndian.PutUint64(data[8:], 0x123456789ABCDEF0)
	ctx := &LiftContext{Memory: &mockMemory{data: data}}
	val, err := LiftHeap(ctx, types.U64{}, 8)
	require.NoError(t, err)
	require.Equal(t, uint64(0x123456789ABCDEF0), val.U64())
}

func TestLiftHeapS64(t *testing.T) {
	data := make([]byte, 32)
	binary.LittleEndian.PutUint64(data[16:], 0xFFFFFFFFFFFFFFFF) // -1
	ctx := &LiftContext{Memory: &mockMemory{data: data}}
	val, err := LiftHeap(ctx, types.S64{}, 16)
	require.NoError(t, err)
	require.Equal(t, int64(-1), val.S64())
}

func TestLiftHeapF32(t *testing.T) {
	data := make([]byte, 16)
	binary.LittleEndian.PutUint32(data[4:], math.Float32bits(3.14))
	ctx := &LiftContext{Memory: &mockMemory{data: data}}
	val, err := LiftHeap(ctx, types.F32{}, 4)
	require.NoError(t, err)
	require.Equal(t, float32(3.14), val.F32())
}

func TestLiftHeapF64(t *testing.T) {
	data := make([]byte, 32)
	binary.LittleEndian.PutUint64(data[8:], math.Float64bits(3.14159265359))
	ctx := &LiftContext{Memory: &mockMemory{data: data}}
	val, err := LiftHeap(ctx, types.F64{}, 8)
	require.NoError(t, err)
	require.Equal(t, 3.14159265359, val.F64())
}

func TestLiftHeapChar(t *testing.T) {
	data := make([]byte, 16)
	binary.LittleEndian.PutUint32(data[4:], 0x1F600) // Emoji code point
	ctx := &LiftContext{Memory: &mockMemory{data: data}}
	val, err := LiftHeap(ctx, types.Char{}, 4)
	require.NoError(t, err)
	require.Equal(t, rune(0x1F600), val.Char())
}

// --- LiftHeap Record Tests ---

func TestLiftHeapRecord(t *testing.T) {
	// Record { a: u8, b: u32, c: u16 } at offset 16
	// Layout: u8@0, padding@1-3, u32@4, u16@8
	data := make([]byte, 32)
	data[16] = 0x42                                  // a = 0x42
	binary.LittleEndian.PutUint32(data[20:], 0xDEADBEEF) // b at offset 16+4
	binary.LittleEndian.PutUint16(data[24:], 0x1234)     // c at offset 16+8

	ctx := &LiftContext{Memory: &mockMemory{data: data}}
	recType := types.Record{
		Fields: []types.Field{
			{Name: "a", Type: types.U8{}},
			{Name: "b", Type: types.U32{}},
			{Name: "c", Type: types.U16{}},
		},
	}

	val, err := LiftHeap(ctx, recType, 16)
	require.NoError(t, err)

	rec := val.Record()
	require.Equal(t, uint8(0x42), rec["a"].U8())
	require.Equal(t, uint32(0xDEADBEEF), rec["b"].U32())
	require.Equal(t, uint16(0x1234), rec["c"].U16())
}

func TestLiftHeapRecordEmpty(t *testing.T) {
	data := make([]byte, 16)
	ctx := &LiftContext{Memory: &mockMemory{data: data}}
	recType := types.Record{Fields: []types.Field{}}

	val, err := LiftHeap(ctx, recType, 0)
	require.NoError(t, err)
	require.Equal(t, component.ValKindRecord, val.Kind())
	require.Equal(t, 0, len(val.Record()))
}

func TestLiftHeapRecordNested(t *testing.T) {
	// Record { inner: Record { x: u32, y: u32 }, z: u16 }
	// Layout: inner.x@0, inner.y@4, z@8
	data := make([]byte, 32)
	binary.LittleEndian.PutUint32(data[0:], 10)  // inner.x
	binary.LittleEndian.PutUint32(data[4:], 20)  // inner.y
	binary.LittleEndian.PutUint16(data[8:], 100) // z

	ctx := &LiftContext{Memory: &mockMemory{data: data}}
	innerType := types.Record{
		Fields: []types.Field{
			{Name: "x", Type: types.U32{}},
			{Name: "y", Type: types.U32{}},
		},
	}
	outerType := types.Record{
		Fields: []types.Field{
			{Name: "inner", Type: innerType},
			{Name: "z", Type: types.U16{}},
		},
	}

	val, err := LiftHeap(ctx, outerType, 0)
	require.NoError(t, err)

	rec := val.Record()
	inner := rec["inner"].Record()
	require.Equal(t, uint32(10), inner["x"].U32())
	require.Equal(t, uint32(20), inner["y"].U32())
	require.Equal(t, uint16(100), rec["z"].U16())
}

func TestLiftHeapRecordAllPrimitives(t *testing.T) {
	// Record with various primitive types to verify alignment
	// { a: bool, b: u8, c: u16, d: u32, e: u64 }
	// Layout: bool@0, u8@1, u16@2, u32@4, u64@8
	data := make([]byte, 32)
	data[0] = 1                                           // bool true
	data[1] = 0xAB                                        // u8
	binary.LittleEndian.PutUint16(data[2:], 0x1234)       // u16
	binary.LittleEndian.PutUint32(data[4:], 0xDEADBEEF)   // u32
	binary.LittleEndian.PutUint64(data[8:], 0x123456789ABCDEF0) // u64

	ctx := &LiftContext{Memory: &mockMemory{data: data}}
	recType := types.Record{
		Fields: []types.Field{
			{Name: "a", Type: types.Bool{}},
			{Name: "b", Type: types.U8{}},
			{Name: "c", Type: types.U16{}},
			{Name: "d", Type: types.U32{}},
			{Name: "e", Type: types.U64{}},
		},
	}

	val, err := LiftHeap(ctx, recType, 0)
	require.NoError(t, err)

	rec := val.Record()
	require.True(t, rec["a"].Bool())
	require.Equal(t, uint8(0xAB), rec["b"].U8())
	require.Equal(t, uint16(0x1234), rec["c"].U16())
	require.Equal(t, uint32(0xDEADBEEF), rec["d"].U32())
	require.Equal(t, uint64(0x123456789ABCDEF0), rec["e"].U64())
}

func TestLiftHeapRecordWithPadding(t *testing.T) {
	// Record { a: u8, b: u64 } to test large alignment gap
	// Layout within record: u8@0, padding@1-7, u64@8
	// Record starts at offset 4, so:
	// - a is at 4+0 = 4
	// - b is at 4+8 = 12 (aligned to 8 bytes within the record)
	data := make([]byte, 24)
	data[4] = 0x42                                              // a at offset 4+0=4
	binary.LittleEndian.PutUint64(data[12:], 0xDEADBEEFCAFEBABE) // b at offset 4+8=12

	ctx := &LiftContext{Memory: &mockMemory{data: data}}
	recType := types.Record{
		Fields: []types.Field{
			{Name: "a", Type: types.U8{}},
			{Name: "b", Type: types.U64{}},
		},
	}

	val, err := LiftHeap(ctx, recType, 4)
	require.NoError(t, err)

	rec := val.Record()
	require.Equal(t, uint8(0x42), rec["a"].U8())
	require.Equal(t, uint64(0xDEADBEEFCAFEBABE), rec["b"].U64())
}

func TestLiftHeapUnsupportedType(t *testing.T) {
	data := make([]byte, 16)
	ctx := &LiftContext{Memory: &mockMemory{data: data}}
	// Use String type which isn't supported in heap lift yet
	_, err := LiftHeap(ctx, types.String{}, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported heap lift")
}
