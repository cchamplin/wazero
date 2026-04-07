// Package conformance contains conformance tests for the Component Model implementation.
// Composite type tests ported from wasmtime's tests/all/component_model/func.rs (tuples)
package conformance

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/abi"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestCompositeRecordEmpty tests empty record handling.
// Empty records have size 0, align 1, and FlattenCount 0.
func TestCompositeRecordEmpty(t *testing.T) {
	emptyRecord := types.Record{Fields: []types.Field{}}

	t.Run("type_properties", func(t *testing.T) {
		require.Equal(t, uint32(0), emptyRecord.Size(), "empty record should have size 0")
		require.Equal(t, uint32(1), emptyRecord.Align(), "empty record should have align 1")
		require.Equal(t, 0, emptyRecord.FlattenCount(), "empty record should have FlattenCount 0")
	})

	t.Run("roundtrip", func(t *testing.T) {
		val := types.ValRecord(map[string]types.Val{})

		flat, err := abi.LowerFlat(nil, emptyRecord, val)
		require.NoError(t, err)
		require.Equal(t, 0, len(flat), "empty record should lower to 0 flat values")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, emptyRecord, iter)
		require.NoError(t, err)
		require.Equal(t, types.ValKindRecord, lifted.Kind())
		require.Equal(t, 0, len(lifted.Record()))
	})
}

// TestCompositeTupleEmpty tests empty tuple handling.
// Empty tuples have size 0, align 1, and FlattenCount 0.
func TestCompositeTupleEmpty(t *testing.T) {
	emptyTuple := types.Tuple{Types: []types.ValType{}}

	t.Run("type_properties", func(t *testing.T) {
		require.Equal(t, uint32(0), emptyTuple.Size(), "empty tuple should have size 0")
		require.Equal(t, uint32(1), emptyTuple.Align(), "empty tuple should have align 1")
		require.Equal(t, 0, emptyTuple.FlattenCount(), "empty tuple should have FlattenCount 0")
	})

	t.Run("roundtrip", func(t *testing.T) {
		val := types.ValTuple([]types.Val{})

		flat, err := abi.LowerFlat(nil, emptyTuple, val)
		require.NoError(t, err)
		require.Equal(t, 0, len(flat), "empty tuple should lower to 0 flat values")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, emptyTuple, iter)
		require.NoError(t, err)
		require.Equal(t, types.ValKindTuple, lifted.Kind())
		require.Equal(t, 0, len(lifted.Tuple()))
	})
}

// TestCompositeRecordSingleField tests record with a single s32 field.
// record { x: s32 } has size 4, align 4, FlattenCount 1
func TestCompositeRecordSingleField(t *testing.T) {
	recordType := types.Record{
		Fields: []types.Field{
			{Name: "x", Type: types.S32{}},
		},
	}

	t.Run("type_properties", func(t *testing.T) {
		require.Equal(t, uint32(4), recordType.Size(), "single s32 record should have size 4")
		require.Equal(t, uint32(4), recordType.Align(), "single s32 record should have align 4")
		require.Equal(t, 1, recordType.FlattenCount(), "single s32 record should have FlattenCount 1")
	})

	t.Run("roundtrip_positive", func(t *testing.T) {
		val := types.ValRecord(map[string]types.Val{
			"x": types.ValS32(42),
		})

		flat, err := abi.LowerFlat(nil, recordType, val)
		require.NoError(t, err)
		require.Equal(t, 1, len(flat))

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, recordType, iter)
		require.NoError(t, err)
		require.Equal(t, types.ValKindRecord, lifted.Kind())

		fieldVal, ok := lifted.RecordField("x")
		require.True(t, ok)
		require.Equal(t, int32(42), fieldVal.S32())
	})

	t.Run("roundtrip_negative", func(t *testing.T) {
		val := types.ValRecord(map[string]types.Val{
			"x": types.ValS32(-100),
		})

		flat, err := abi.LowerFlat(nil, recordType, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, recordType, iter)
		require.NoError(t, err)

		fieldVal, ok := lifted.RecordField("x")
		require.True(t, ok)
		require.Equal(t, int32(-100), fieldVal.S32())
	})

	t.Run("roundtrip_boundary_values", func(t *testing.T) {
		tests := []int32{0, math.MaxInt32, math.MinInt32}
		for _, v := range tests {
			val := types.ValRecord(map[string]types.Val{
				"x": types.ValS32(v),
			})

			flat, err := abi.LowerFlat(nil, recordType, val)
			require.NoError(t, err)

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, recordType, iter)
			require.NoError(t, err)

			fieldVal, _ := lifted.RecordField("x")
			require.Equal(t, v, fieldVal.S32())
		}
	})
}

// TestCompositeRecordWithPadding tests record with padding due to alignment.
// record { a: u8, b: u32 } should have size 8 due to alignment padding.
func TestCompositeRecordWithPadding(t *testing.T) {
	recordType := types.Record{
		Fields: []types.Field{
			{Name: "a", Type: types.U8{}},
			{Name: "b", Type: types.U32{}},
		},
	}

	t.Run("type_properties", func(t *testing.T) {
		// a: u8 at offset 0, size 1
		// b: u32 needs 4-byte alignment, so padding of 3 bytes, then at offset 4, size 4
		// Total: 8 bytes
		require.Equal(t, uint32(8), recordType.Size(), "record {u8, u32} should have size 8")
		require.Equal(t, uint32(4), recordType.Align(), "record {u8, u32} should have align 4")
		require.Equal(t, 2, recordType.FlattenCount(), "record {u8, u32} should have FlattenCount 2")
	})

	t.Run("field_offsets", func(t *testing.T) {
		offsets := recordType.FieldOffsets()
		require.Equal(t, 2, len(offsets))
		require.Equal(t, uint32(0), offsets[0], "field 'a' should be at offset 0")
		require.Equal(t, uint32(4), offsets[1], "field 'b' should be at offset 4 (after padding)")
	})

	t.Run("roundtrip", func(t *testing.T) {
		val := types.ValRecord(map[string]types.Val{
			"a": types.ValU8(255),
			"b": types.ValU32(0xDEADBEEF),
		})

		flat, err := abi.LowerFlat(nil, recordType, val)
		require.NoError(t, err)
		require.Equal(t, 2, len(flat))

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, recordType, iter)
		require.NoError(t, err)

		aVal, ok := lifted.RecordField("a")
		require.True(t, ok)
		require.Equal(t, uint8(255), aVal.U8())

		bVal, ok := lifted.RecordField("b")
		require.True(t, ok)
		require.Equal(t, uint32(0xDEADBEEF), bVal.U32())
	})
}

// TestCompositeRecordComplexPadding tests more complex padding scenarios.
func TestCompositeRecordComplexPadding(t *testing.T) {
	// record { a: u8, b: u16, c: u8, d: u32 }
	// a: offset 0, size 1
	// b: needs 2-byte align, so offset 2, size 2
	// c: offset 4, size 1
	// d: needs 4-byte align, so offset 8, size 4
	// Total size: 12 bytes, aligned to 4 = 12
	recordType := types.Record{
		Fields: []types.Field{
			{Name: "a", Type: types.U8{}},
			{Name: "b", Type: types.U16{}},
			{Name: "c", Type: types.U8{}},
			{Name: "d", Type: types.U32{}},
		},
	}

	t.Run("type_properties", func(t *testing.T) {
		require.Equal(t, uint32(12), recordType.Size(), "record should have size 12")
		require.Equal(t, uint32(4), recordType.Align(), "record should have align 4")
		require.Equal(t, 4, recordType.FlattenCount())
	})

	t.Run("field_offsets", func(t *testing.T) {
		offsets := recordType.FieldOffsets()
		require.Equal(t, 4, len(offsets))
		require.Equal(t, uint32(0), offsets[0], "field 'a' at offset 0")
		require.Equal(t, uint32(2), offsets[1], "field 'b' at offset 2")
		require.Equal(t, uint32(4), offsets[2], "field 'c' at offset 4")
		require.Equal(t, uint32(8), offsets[3], "field 'd' at offset 8")
	})

	t.Run("roundtrip", func(t *testing.T) {
		val := types.ValRecord(map[string]types.Val{
			"a": types.ValU8(0x11),
			"b": types.ValU16(0x2233),
			"c": types.ValU8(0x44),
			"d": types.ValU32(0x55667788),
		})

		flat, err := abi.LowerFlat(nil, recordType, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, recordType, iter)
		require.NoError(t, err)

		aVal, _ := lifted.RecordField("a")
		require.Equal(t, uint8(0x11), aVal.U8())

		bVal, _ := lifted.RecordField("b")
		require.Equal(t, uint16(0x2233), bVal.U16())

		cVal, _ := lifted.RecordField("c")
		require.Equal(t, uint8(0x44), cVal.U8())

		dVal, _ := lifted.RecordField("d")
		require.Equal(t, uint32(0x55667788), dVal.U32())
	})
}

// TestCompositeTupleRoundtrip tests tuple<s32, s32> roundtrip.
// tuple<s32, s32> should have FlattenCount 2.
func TestCompositeTupleRoundtrip(t *testing.T) {
	tupleType := types.Tuple{
		Types: []types.ValType{types.S32{}, types.S32{}},
	}

	t.Run("type_properties", func(t *testing.T) {
		require.Equal(t, uint32(8), tupleType.Size(), "tuple<s32,s32> should have size 8")
		require.Equal(t, uint32(4), tupleType.Align(), "tuple<s32,s32> should have align 4")
		require.Equal(t, 2, tupleType.FlattenCount(), "tuple<s32,s32> should have FlattenCount 2")
	})

	t.Run("element_offsets", func(t *testing.T) {
		offsets := tupleType.ElementOffsets()
		require.Equal(t, 2, len(offsets))
		require.Equal(t, uint32(0), offsets[0])
		require.Equal(t, uint32(4), offsets[1])
	})

	t.Run("roundtrip_positive", func(t *testing.T) {
		val := types.ValTuple([]types.Val{
			types.ValS32(100),
			types.ValS32(200),
		})

		flat, err := abi.LowerFlat(nil, tupleType, val)
		require.NoError(t, err)
		require.Equal(t, 2, len(flat))

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, tupleType, iter)
		require.NoError(t, err)

		elems := lifted.Tuple()
		require.Equal(t, 2, len(elems))
		require.Equal(t, int32(100), elems[0].S32())
		require.Equal(t, int32(200), elems[1].S32())
	})

	t.Run("roundtrip_negative", func(t *testing.T) {
		val := types.ValTuple([]types.Val{
			types.ValS32(-1),
			types.ValS32(math.MinInt32),
		})

		flat, err := abi.LowerFlat(nil, tupleType, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, tupleType, iter)
		require.NoError(t, err)

		elems := lifted.Tuple()
		require.Equal(t, int32(-1), elems[0].S32())
		require.Equal(t, int32(math.MinInt32), elems[1].S32())
	})
}

// TestCompositeRecordNested tests nested record handling.
// record { inner: record { x: s32 } }
func TestCompositeRecordNested(t *testing.T) {
	innerRecord := types.Record{
		Fields: []types.Field{
			{Name: "x", Type: types.S32{}},
		},
	}
	outerRecord := types.Record{
		Fields: []types.Field{
			{Name: "inner", Type: innerRecord},
		},
	}

	t.Run("inner_type_properties", func(t *testing.T) {
		require.Equal(t, uint32(4), innerRecord.Size())
		require.Equal(t, uint32(4), innerRecord.Align())
		require.Equal(t, 1, innerRecord.FlattenCount())
	})

	t.Run("outer_type_properties", func(t *testing.T) {
		// Outer record inherits size/align from inner
		require.Equal(t, uint32(4), outerRecord.Size(), "outer record should have size 4")
		require.Equal(t, uint32(4), outerRecord.Align(), "outer record should have align 4")
		require.Equal(t, 1, outerRecord.FlattenCount(), "outer record should have FlattenCount 1")
	})

	t.Run("roundtrip", func(t *testing.T) {
		innerVal := types.ValRecord(map[string]types.Val{
			"x": types.ValS32(42),
		})
		outerVal := types.ValRecord(map[string]types.Val{
			"inner": innerVal,
		})

		flat, err := abi.LowerFlat(nil, outerRecord, outerVal)
		require.NoError(t, err)
		require.Equal(t, 1, len(flat), "nested record should flatten to 1 value")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, outerRecord, iter)
		require.NoError(t, err)

		liftedInner, ok := lifted.RecordField("inner")
		require.True(t, ok)
		require.Equal(t, types.ValKindRecord, liftedInner.Kind())

		xVal, ok := liftedInner.RecordField("x")
		require.True(t, ok)
		require.Equal(t, int32(42), xVal.S32())
	})

	t.Run("roundtrip_boundary_values", func(t *testing.T) {
		tests := []int32{0, math.MaxInt32, math.MinInt32}
		for _, v := range tests {
			innerVal := types.ValRecord(map[string]types.Val{
				"x": types.ValS32(v),
			})
			outerVal := types.ValRecord(map[string]types.Val{
				"inner": innerVal,
			})

			flat, err := abi.LowerFlat(nil, outerRecord, outerVal)
			require.NoError(t, err)

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, outerRecord, iter)
			require.NoError(t, err)

			liftedInner, _ := lifted.RecordField("inner")
			xVal, _ := liftedInner.RecordField("x")
			require.Equal(t, v, xVal.S32())
		}
	})
}

// TestCompositeRecordDeeplyNested tests deeply nested records.
func TestCompositeRecordDeeplyNested(t *testing.T) {
	level3 := types.Record{
		Fields: []types.Field{
			{Name: "value", Type: types.U64{}},
		},
	}
	level2 := types.Record{
		Fields: []types.Field{
			{Name: "level3", Type: level3},
		},
	}
	level1 := types.Record{
		Fields: []types.Field{
			{Name: "level2", Type: level2},
		},
	}

	t.Run("type_properties", func(t *testing.T) {
		require.Equal(t, uint32(8), level1.Size())
		require.Equal(t, uint32(8), level1.Align())
		require.Equal(t, 1, level1.FlattenCount())
	})

	t.Run("roundtrip", func(t *testing.T) {
		l3Val := types.ValRecord(map[string]types.Val{
			"value": types.ValU64(0xDEADBEEF12345678),
		})
		l2Val := types.ValRecord(map[string]types.Val{
			"level3": l3Val,
		})
		l1Val := types.ValRecord(map[string]types.Val{
			"level2": l2Val,
		})

		flat, err := abi.LowerFlat(nil, level1, l1Val)
		require.NoError(t, err)
		require.Equal(t, 1, len(flat))

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, level1, iter)
		require.NoError(t, err)

		liftedL2, _ := lifted.RecordField("level2")
		liftedL3, _ := liftedL2.RecordField("level3")
		valueVal, _ := liftedL3.RecordField("value")
		require.Equal(t, uint64(0xDEADBEEF12345678), valueVal.U64())
	})
}

// TestCompositeTupleMixed tests tuple<bool, u8, u16, u32, u64>.
// Verifies all values roundtrip correctly.
func TestCompositeTupleMixed(t *testing.T) {
	mixedTuple := types.Tuple{
		Types: []types.ValType{
			types.Bool{},
			types.U8{},
			types.U16{},
			types.U32{},
			types.U64{},
		},
	}

	t.Run("type_properties", func(t *testing.T) {
		// Layout:
		// bool: offset 0, size 1, align 1
		// u8: offset 1, size 1, align 1
		// u16: offset 2, size 2, align 2
		// u32: offset 4, size 4, align 4
		// u64: offset 8, size 8, align 8
		// Total: 16 bytes, align 8
		require.Equal(t, uint32(16), mixedTuple.Size())
		require.Equal(t, uint32(8), mixedTuple.Align())
		require.Equal(t, 5, mixedTuple.FlattenCount())
	})

	t.Run("element_offsets", func(t *testing.T) {
		offsets := mixedTuple.ElementOffsets()
		require.Equal(t, 5, len(offsets))
		require.Equal(t, uint32(0), offsets[0], "bool at offset 0")
		require.Equal(t, uint32(1), offsets[1], "u8 at offset 1")
		require.Equal(t, uint32(2), offsets[2], "u16 at offset 2")
		require.Equal(t, uint32(4), offsets[3], "u32 at offset 4")
		require.Equal(t, uint32(8), offsets[4], "u64 at offset 8")
	})

	t.Run("roundtrip_typical", func(t *testing.T) {
		val := types.ValTuple([]types.Val{
			types.ValBool(true),
			types.ValU8(42),
			types.ValU16(1000),
			types.ValU32(100000),
			types.ValU64(9000000000000),
		})

		flat, err := abi.LowerFlat(nil, mixedTuple, val)
		require.NoError(t, err)
		require.Equal(t, 5, len(flat))

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, mixedTuple, iter)
		require.NoError(t, err)

		elems := lifted.Tuple()
		require.Equal(t, 5, len(elems))
		require.True(t, elems[0].Bool())
		require.Equal(t, uint8(42), elems[1].U8())
		require.Equal(t, uint16(1000), elems[2].U16())
		require.Equal(t, uint32(100000), elems[3].U32())
		require.Equal(t, uint64(9000000000000), elems[4].U64())
	})

	t.Run("roundtrip_boundary_values", func(t *testing.T) {
		val := types.ValTuple([]types.Val{
			types.ValBool(false),
			types.ValU8(math.MaxUint8),
			types.ValU16(math.MaxUint16),
			types.ValU32(math.MaxUint32),
			types.ValU64(math.MaxUint64),
		})

		flat, err := abi.LowerFlat(nil, mixedTuple, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, mixedTuple, iter)
		require.NoError(t, err)

		elems := lifted.Tuple()
		require.False(t, elems[0].Bool())
		require.Equal(t, uint8(math.MaxUint8), elems[1].U8())
		require.Equal(t, uint16(math.MaxUint16), elems[2].U16())
		require.Equal(t, uint32(math.MaxUint32), elems[3].U32())
		require.Equal(t, uint64(math.MaxUint64), elems[4].U64())
	})

	t.Run("roundtrip_zeros", func(t *testing.T) {
		val := types.ValTuple([]types.Val{
			types.ValBool(false),
			types.ValU8(0),
			types.ValU16(0),
			types.ValU32(0),
			types.ValU64(0),
		})

		flat, err := abi.LowerFlat(nil, mixedTuple, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, mixedTuple, iter)
		require.NoError(t, err)

		elems := lifted.Tuple()
		require.False(t, elems[0].Bool())
		require.Equal(t, uint8(0), elems[1].U8())
		require.Equal(t, uint16(0), elems[2].U16())
		require.Equal(t, uint32(0), elems[3].U32())
		require.Equal(t, uint64(0), elems[4].U64())
	})
}

// TestCompositeTupleNested tests nested tuple handling.
func TestCompositeTupleNested(t *testing.T) {
	innerTuple := types.Tuple{
		Types: []types.ValType{types.S32{}, types.S32{}},
	}
	outerTuple := types.Tuple{
		Types: []types.ValType{innerTuple, types.U8{}},
	}

	t.Run("type_properties", func(t *testing.T) {
		// Inner: size 8, align 4, flattenCount 2
		// Outer: inner at offset 0, u8 at offset 8
		// Total: 9 bytes, rounded to align 4 = 12 bytes
		require.Equal(t, uint32(12), outerTuple.Size())
		require.Equal(t, uint32(4), outerTuple.Align())
		require.Equal(t, 3, outerTuple.FlattenCount())
	})

	t.Run("roundtrip", func(t *testing.T) {
		innerVal := types.ValTuple([]types.Val{
			types.ValS32(10),
			types.ValS32(20),
		})
		outerVal := types.ValTuple([]types.Val{
			innerVal,
			types.ValU8(99),
		})

		flat, err := abi.LowerFlat(nil, outerTuple, outerVal)
		require.NoError(t, err)
		require.Equal(t, 3, len(flat))

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, outerTuple, iter)
		require.NoError(t, err)

		elems := lifted.Tuple()
		require.Equal(t, 2, len(elems))

		innerElems := elems[0].Tuple()
		require.Equal(t, 2, len(innerElems))
		require.Equal(t, int32(10), innerElems[0].S32())
		require.Equal(t, int32(20), innerElems[1].S32())

		require.Equal(t, uint8(99), elems[1].U8())
	})
}

// TestCompositeRecordMultipleFields tests records with multiple fields of varying types.
func TestCompositeRecordMultipleFields(t *testing.T) {
	recordType := types.Record{
		Fields: []types.Field{
			{Name: "flag", Type: types.Bool{}},
			{Name: "count", Type: types.U32{}},
			{Name: "value", Type: types.S64{}},
			{Name: "ratio", Type: types.F64{}},
		},
	}

	t.Run("type_properties", func(t *testing.T) {
		// flag: offset 0, size 1, align 1
		// count: offset 4, size 4, align 4 (padding 3 bytes)
		// value: offset 8, size 8, align 8
		// ratio: offset 16, size 8, align 8
		// Total: 24 bytes, align 8
		require.Equal(t, uint32(24), recordType.Size())
		require.Equal(t, uint32(8), recordType.Align())
		require.Equal(t, 4, recordType.FlattenCount())
	})

	t.Run("roundtrip", func(t *testing.T) {
		val := types.ValRecord(map[string]types.Val{
			"flag":  types.ValBool(true),
			"count": types.ValU32(12345),
			"value": types.ValS64(-9876543210),
			"ratio": types.ValF64(3.14159265358979),
		})

		flat, err := abi.LowerFlat(nil, recordType, val)
		require.NoError(t, err)
		require.Equal(t, 4, len(flat))

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, recordType, iter)
		require.NoError(t, err)

		flagVal, _ := lifted.RecordField("flag")
		require.True(t, flagVal.Bool())

		countVal, _ := lifted.RecordField("count")
		require.Equal(t, uint32(12345), countVal.U32())

		valueVal, _ := lifted.RecordField("value")
		require.Equal(t, int64(-9876543210), valueVal.S64())

		ratioVal, _ := lifted.RecordField("ratio")
		require.Equal(t, math.Float64bits(3.14159265358979), math.Float64bits(ratioVal.F64()))
	})
}

// TestCompositeTupleAllPrimitives tests a tuple containing all primitive types.
func TestCompositeTupleAllPrimitives(t *testing.T) {
	allPrimitives := types.Tuple{
		Types: []types.ValType{
			types.Bool{},
			types.S8{},
			types.U8{},
			types.S16{},
			types.U16{},
			types.S32{},
			types.U32{},
			types.S64{},
			types.U64{},
			types.F32{},
			types.F64{},
			types.Char{},
		},
	}

	t.Run("flatten_count", func(t *testing.T) {
		require.Equal(t, 12, allPrimitives.FlattenCount())
	})

	t.Run("roundtrip", func(t *testing.T) {
		val := types.ValTuple([]types.Val{
			types.ValBool(true),
			types.ValS8(-42),
			types.ValU8(255),
			types.ValS16(-1000),
			types.ValU16(65000),
			types.ValS32(-100000),
			types.ValU32(4000000000),
			types.ValS64(-9000000000000),
			types.ValU64(18000000000000000000),
			types.ValF32(3.14),
			types.ValF64(2.718281828),
			types.ValChar('A'),
		})

		flat, err := abi.LowerFlat(nil, allPrimitives, val)
		require.NoError(t, err)
		require.Equal(t, 12, len(flat))

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, allPrimitives, iter)
		require.NoError(t, err)

		elems := lifted.Tuple()
		require.Equal(t, 12, len(elems))

		require.True(t, elems[0].Bool())
		require.Equal(t, int8(-42), elems[1].S8())
		require.Equal(t, uint8(255), elems[2].U8())
		require.Equal(t, int16(-1000), elems[3].S16())
		require.Equal(t, uint16(65000), elems[4].U16())
		require.Equal(t, int32(-100000), elems[5].S32())
		require.Equal(t, uint32(4000000000), elems[6].U32())
		require.Equal(t, int64(-9000000000000), elems[7].S64())
		require.Equal(t, uint64(18000000000000000000), elems[8].U64())
		require.Equal(t, math.Float32bits(3.14), math.Float32bits(elems[9].F32()))
		require.Equal(t, math.Float64bits(2.718281828), math.Float64bits(elems[10].F64()))
		require.Equal(t, 'A', elems[11].Char())
	})
}

// TestCompositeRecordMissingField tests error handling for missing fields.
func TestCompositeRecordMissingField(t *testing.T) {
	recordType := types.Record{
		Fields: []types.Field{
			{Name: "x", Type: types.S32{}},
			{Name: "y", Type: types.S32{}},
		},
	}

	t.Run("missing_field_error", func(t *testing.T) {
		// Only provide one field when two are required
		val := types.ValRecord(map[string]types.Val{
			"x": types.ValS32(42),
			// "y" is missing
		})

		_, err := abi.LowerFlat(nil, recordType, val)
		require.Error(t, err)
		require.Contains(t, err.Error(), "missing record field")
	})
}

// TestCompositeTupleWrongLength tests error handling for wrong tuple length.
func TestCompositeTupleWrongLength(t *testing.T) {
	tupleType := types.Tuple{
		Types: []types.ValType{types.S32{}, types.S32{}},
	}

	t.Run("too_few_elements", func(t *testing.T) {
		val := types.ValTuple([]types.Val{
			types.ValS32(42),
			// Missing second element
		})

		_, err := abi.LowerFlat(nil, tupleType, val)
		require.Error(t, err)
		require.Contains(t, err.Error(), "tuple has 1 elements, expected 2")
	})

	t.Run("too_many_elements", func(t *testing.T) {
		val := types.ValTuple([]types.Val{
			types.ValS32(1),
			types.ValS32(2),
			types.ValS32(3), // Extra element
		})

		_, err := abi.LowerFlat(nil, tupleType, val)
		require.Error(t, err)
		require.Contains(t, err.Error(), "tuple has 3 elements, expected 2")
	})
}

// TestCompositeTypeAlignmentTable verifies alignment calculation for various field combinations.
func TestCompositeTypeAlignmentTable(t *testing.T) {
	tests := []struct {
		name  string
		typ   types.ValType
		size  uint32
		align uint32
	}{
		// Single field records
		{"record_u8", types.Record{Fields: []types.Field{{Name: "a", Type: types.U8{}}}}, 1, 1},
		{"record_u16", types.Record{Fields: []types.Field{{Name: "a", Type: types.U16{}}}}, 2, 2},
		{"record_u32", types.Record{Fields: []types.Field{{Name: "a", Type: types.U32{}}}}, 4, 4},
		{"record_u64", types.Record{Fields: []types.Field{{Name: "a", Type: types.U64{}}}}, 8, 8},
		{"record_f32", types.Record{Fields: []types.Field{{Name: "a", Type: types.F32{}}}}, 4, 4},
		{"record_f64", types.Record{Fields: []types.Field{{Name: "a", Type: types.F64{}}}}, 8, 8},

		// Single element tuples
		{"tuple_u8", types.Tuple{Types: []types.ValType{types.U8{}}}, 1, 1},
		{"tuple_u16", types.Tuple{Types: []types.ValType{types.U16{}}}, 2, 2},
		{"tuple_u32", types.Tuple{Types: []types.ValType{types.U32{}}}, 4, 4},
		{"tuple_u64", types.Tuple{Types: []types.ValType{types.U64{}}}, 8, 8},

		// Multi-field records with padding
		{"record_u8_u16", types.Record{Fields: []types.Field{
			{Name: "a", Type: types.U8{}},
			{Name: "b", Type: types.U16{}},
		}}, 4, 2}, // u8 + pad + u16

		{"record_u8_u32", types.Record{Fields: []types.Field{
			{Name: "a", Type: types.U8{}},
			{Name: "b", Type: types.U32{}},
		}}, 8, 4}, // u8 + 3 pad + u32

		{"record_u8_u64", types.Record{Fields: []types.Field{
			{Name: "a", Type: types.U8{}},
			{Name: "b", Type: types.U64{}},
		}}, 16, 8}, // u8 + 7 pad + u64

		{"record_u16_u32", types.Record{Fields: []types.Field{
			{Name: "a", Type: types.U16{}},
			{Name: "b", Type: types.U32{}},
		}}, 8, 4}, // u16 + 2 pad + u32

		// Records with trailing padding
		{"record_u32_u8", types.Record{Fields: []types.Field{
			{Name: "a", Type: types.U32{}},
			{Name: "b", Type: types.U8{}},
		}}, 8, 4}, // u32 + u8 + 3 trailing pad
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.size, tc.typ.Size(), "size mismatch")
			require.Equal(t, tc.align, tc.typ.Align(), "align mismatch")
		})
	}
}

// TestCompositeRecordWithRecordField tests record containing another record field.
func TestCompositeRecordWithRecordField(t *testing.T) {
	point := types.Record{
		Fields: []types.Field{
			{Name: "x", Type: types.F32{}},
			{Name: "y", Type: types.F32{}},
		},
	}
	line := types.Record{
		Fields: []types.Field{
			{Name: "start", Type: point},
			{Name: "end", Type: point},
		},
	}

	t.Run("type_properties", func(t *testing.T) {
		require.Equal(t, uint32(8), point.Size())
		require.Equal(t, uint32(4), point.Align())
		require.Equal(t, 2, point.FlattenCount())

		require.Equal(t, uint32(16), line.Size())
		require.Equal(t, uint32(4), line.Align())
		require.Equal(t, 4, line.FlattenCount())
	})

	t.Run("roundtrip", func(t *testing.T) {
		startVal := types.ValRecord(map[string]types.Val{
			"x": types.ValF32(1.0),
			"y": types.ValF32(2.0),
		})
		endVal := types.ValRecord(map[string]types.Val{
			"x": types.ValF32(3.0),
			"y": types.ValF32(4.0),
		})
		lineVal := types.ValRecord(map[string]types.Val{
			"start": startVal,
			"end":   endVal,
		})

		flat, err := abi.LowerFlat(nil, line, lineVal)
		require.NoError(t, err)
		require.Equal(t, 4, len(flat))

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, line, iter)
		require.NoError(t, err)

		liftedStart, _ := lifted.RecordField("start")
		xVal, _ := liftedStart.RecordField("x")
		yVal, _ := liftedStart.RecordField("y")
		require.Equal(t, float32(1.0), xVal.F32())
		require.Equal(t, float32(2.0), yVal.F32())

		liftedEnd, _ := lifted.RecordField("end")
		xVal, _ = liftedEnd.RecordField("x")
		yVal, _ = liftedEnd.RecordField("y")
		require.Equal(t, float32(3.0), xVal.F32())
		require.Equal(t, float32(4.0), yVal.F32())
	})
}

// TestCompositeTupleWithTupleElement tests tuple containing another tuple.
func TestCompositeTupleWithTupleElement(t *testing.T) {
	pair := types.Tuple{Types: []types.ValType{types.F64{}, types.F64{}}}
	pairOfPairs := types.Tuple{Types: []types.ValType{pair, pair}}

	t.Run("type_properties", func(t *testing.T) {
		require.Equal(t, uint32(16), pair.Size())
		require.Equal(t, uint32(8), pair.Align())
		require.Equal(t, 2, pair.FlattenCount())

		require.Equal(t, uint32(32), pairOfPairs.Size())
		require.Equal(t, uint32(8), pairOfPairs.Align())
		require.Equal(t, 4, pairOfPairs.FlattenCount())
	})

	t.Run("roundtrip", func(t *testing.T) {
		pair1 := types.ValTuple([]types.Val{
			types.ValF64(1.5),
			types.ValF64(2.5),
		})
		pair2 := types.ValTuple([]types.Val{
			types.ValF64(3.5),
			types.ValF64(4.5),
		})
		val := types.ValTuple([]types.Val{pair1, pair2})

		flat, err := abi.LowerFlat(nil, pairOfPairs, val)
		require.NoError(t, err)
		require.Equal(t, 4, len(flat))

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, pairOfPairs, iter)
		require.NoError(t, err)

		elems := lifted.Tuple()
		require.Equal(t, 2, len(elems))

		innerElems1 := elems[0].Tuple()
		require.Equal(t, 1.5, innerElems1[0].F64())
		require.Equal(t, 2.5, innerElems1[1].F64())

		innerElems2 := elems[1].Tuple()
		require.Equal(t, 3.5, innerElems2[0].F64())
		require.Equal(t, 4.5, innerElems2[1].F64())
	})
}

// =============================================================================
// Variant/Option/Result Tests
// Ported from wasmtime's tests/all/component_model/func.rs
// =============================================================================

// TestOptionNone tests option<s32> with None value.
// None has discriminant 0. Tests LowerFlat/LiftFlat roundtrip.
func TestOptionNone(t *testing.T) {
	optionType := types.Option{Some: types.S32{}}

	t.Run("type_properties", func(t *testing.T) {
		// option<s32> is like variant { none, some(s32) }
		// discriminant: 1 byte (2 cases)
		// payload: 4 bytes (s32), align 4
		// size: align(1,4) + 4 = 4 + 4 = 8, aligned to 4 = 8
		require.Equal(t, uint32(8), optionType.Size(), "option<s32> should have size 8")
		require.Equal(t, uint32(4), optionType.Align(), "option<s32> should have align 4")
		require.Equal(t, 2, optionType.FlattenCount(), "option<s32> should have FlattenCount 2 (disc + payload)")
	})

	t.Run("roundtrip_none", func(t *testing.T) {
		// Create None value (nil payload)
		val := types.ValOption(nil)

		flat, err := abi.LowerFlat(nil, optionType, val)
		require.NoError(t, err)
		require.Equal(t, 2, len(flat), "option<s32> should lower to 2 flat values")
		require.Equal(t, uint64(0), flat[0], "None should have discriminant 0")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, optionType, iter)
		require.NoError(t, err)
		require.Equal(t, types.ValKindOption, lifted.Kind())

		// Verify IsNone: payload should be nil
		liftedPayload := lifted.Option()
		require.Nil(t, liftedPayload, "None option should have nil payload")
	})
}

// TestOptionSome tests option<s32> with Some(42) value.
// Some has discriminant 1. Tests that payload is preserved through roundtrip.
func TestOptionSome(t *testing.T) {
	optionType := types.Option{Some: types.S32{}}

	t.Run("roundtrip_some_positive", func(t *testing.T) {
		payload := types.ValS32(42)
		val := types.ValOption(&payload)

		flat, err := abi.LowerFlat(nil, optionType, val)
		require.NoError(t, err)
		require.Equal(t, 2, len(flat), "option<s32> should lower to 2 flat values")
		require.Equal(t, uint64(1), flat[0], "Some should have discriminant 1")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, optionType, iter)
		require.NoError(t, err)
		require.Equal(t, types.ValKindOption, lifted.Kind())

		// Verify Some: payload should not be nil and should have correct value
		liftedPayload := lifted.Option()
		require.NotNil(t, liftedPayload, "Some option should have non-nil payload")
		require.Equal(t, int32(42), liftedPayload.S32(), "payload value should be preserved")
	})

	t.Run("roundtrip_some_negative", func(t *testing.T) {
		payload := types.ValS32(-100)
		val := types.ValOption(&payload)

		flat, err := abi.LowerFlat(nil, optionType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(1), flat[0], "Some should have discriminant 1")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, optionType, iter)
		require.NoError(t, err)

		liftedPayload := lifted.Option()
		require.NotNil(t, liftedPayload)
		require.Equal(t, int32(-100), liftedPayload.S32())
	})

	t.Run("roundtrip_some_boundary_values", func(t *testing.T) {
		tests := []int32{0, math.MaxInt32, math.MinInt32}
		for _, v := range tests {
			payload := types.ValS32(v)
			val := types.ValOption(&payload)

			flat, err := abi.LowerFlat(nil, optionType, val)
			require.NoError(t, err)
			require.Equal(t, uint64(1), flat[0])

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, optionType, iter)
			require.NoError(t, err)

			liftedPayload := lifted.Option()
			require.NotNil(t, liftedPayload)
			require.Equal(t, v, liftedPayload.S32())
		}
	})
}

// TestOptionWithDifferentPayloadTypes tests option with various payload types.
func TestOptionWithDifferentPayloadTypes(t *testing.T) {
	t.Run("option_u8", func(t *testing.T) {
		optType := types.Option{Some: types.U8{}}
		// option<u8>: disc 1 byte + u8 1 byte = 2 bytes, align 1
		require.Equal(t, uint32(2), optType.Size())
		require.Equal(t, uint32(1), optType.Align())
		require.Equal(t, 2, optType.FlattenCount())

		payload := types.ValU8(255)
		val := types.ValOption(&payload)

		flat, err := abi.LowerFlat(nil, optType, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, optType, iter)
		require.NoError(t, err)

		liftedPayload := lifted.Option()
		require.NotNil(t, liftedPayload)
		require.Equal(t, uint8(255), liftedPayload.U8())
	})

	t.Run("option_u64", func(t *testing.T) {
		optType := types.Option{Some: types.U64{}}
		// option<u64>: disc 1 byte + padding to 8 + u64 8 bytes = 16 bytes, align 8
		require.Equal(t, uint32(16), optType.Size())
		require.Equal(t, uint32(8), optType.Align())
		require.Equal(t, 2, optType.FlattenCount())

		payload := types.ValU64(0xDEADBEEFCAFEBABE)
		val := types.ValOption(&payload)

		flat, err := abi.LowerFlat(nil, optType, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, optType, iter)
		require.NoError(t, err)

		liftedPayload := lifted.Option()
		require.NotNil(t, liftedPayload)
		require.Equal(t, uint64(0xDEADBEEFCAFEBABE), liftedPayload.U64())
	})

	t.Run("option_f64", func(t *testing.T) {
		optType := types.Option{Some: types.F64{}}

		payload := types.ValF64(3.14159265358979)
		val := types.ValOption(&payload)

		flat, err := abi.LowerFlat(nil, optType, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, optType, iter)
		require.NoError(t, err)

		liftedPayload := lifted.Option()
		require.NotNil(t, liftedPayload)
		require.Equal(t, math.Float64bits(3.14159265358979), math.Float64bits(liftedPayload.F64()))
	})
}

// TestResultOk tests result<s32, string> with Ok(100) value.
// Ok has discriminant 0. Tests LowerFlat/LiftFlat roundtrip.
func TestResultOk(t *testing.T) {
	// Note: String type not fully supported in flat representation, using s32 for error type too
	resultType := types.Result{Ok: types.S32{}, Error: types.S32{}}

	t.Run("type_properties", func(t *testing.T) {
		// result<s32, s32> is like variant { ok(s32), error(s32) }
		// Both payloads are s32, so max payload is 4 bytes
		// disc: 1 byte, payload align: 4, so offset 4, payload 4 bytes
		// Total: 8 bytes, align 4
		require.Equal(t, uint32(8), resultType.Size(), "result<s32,s32> should have size 8")
		require.Equal(t, uint32(4), resultType.Align(), "result<s32,s32> should have align 4")
		require.Equal(t, 2, resultType.FlattenCount(), "result<s32,s32> should have FlattenCount 2")
	})

	t.Run("roundtrip_ok", func(t *testing.T) {
		okPayload := types.ValS32(100)
		val := types.ValResultOk(&okPayload)

		flat, err := abi.LowerFlat(nil, resultType, val)
		require.NoError(t, err)
		require.Equal(t, 2, len(flat), "result should lower to 2 flat values")
		require.Equal(t, uint64(0), flat[0], "Ok should have discriminant 0")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, resultType, iter)
		require.NoError(t, err)
		require.Equal(t, types.ValKindResult, lifted.Kind())

		// Verify IsOk and Ok()
		isOk, okVal, errVal := lifted.Result()
		require.True(t, isOk, "result should be Ok")
		require.NotNil(t, okVal, "Ok value should not be nil")
		require.Nil(t, errVal, "Error value should be nil for Ok result")
		require.Equal(t, int32(100), okVal.S32(), "Ok payload should be preserved")
	})

	t.Run("roundtrip_ok_boundary_values", func(t *testing.T) {
		tests := []int32{0, math.MaxInt32, math.MinInt32, -1}
		for _, v := range tests {
			okPayload := types.ValS32(v)
			val := types.ValResultOk(&okPayload)

			flat, err := abi.LowerFlat(nil, resultType, val)
			require.NoError(t, err)
			require.Equal(t, uint64(0), flat[0])

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, resultType, iter)
			require.NoError(t, err)

			isOk, okVal, _ := lifted.Result()
			require.True(t, isOk)
			require.Equal(t, v, okVal.S32())
		}
	})
}

// TestResultError tests result<s32, s32> with Error(-1) value.
// Error has discriminant 1. Tests LowerFlat/LiftFlat roundtrip.
func TestResultError(t *testing.T) {
	resultType := types.Result{Ok: types.S32{}, Error: types.S32{}}

	t.Run("roundtrip_error", func(t *testing.T) {
		errPayload := types.ValS32(-1)
		val := types.ValResultError(&errPayload)

		flat, err := abi.LowerFlat(nil, resultType, val)
		require.NoError(t, err)
		require.Equal(t, 2, len(flat), "result should lower to 2 flat values")
		require.Equal(t, uint64(1), flat[0], "Error should have discriminant 1")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, resultType, iter)
		require.NoError(t, err)
		require.Equal(t, types.ValKindResult, lifted.Kind())

		// Verify IsOk() == false and Error()
		isOk, okVal, errVal := lifted.Result()
		require.False(t, isOk, "result should be Error")
		require.Nil(t, okVal, "Ok value should be nil for Error result")
		require.NotNil(t, errVal, "Error value should not be nil")
		require.Equal(t, int32(-1), errVal.S32(), "Error payload should be preserved")
	})

	t.Run("roundtrip_error_various_values", func(t *testing.T) {
		tests := []int32{0, 1, -100, math.MaxInt32, math.MinInt32}
		for _, v := range tests {
			errPayload := types.ValS32(v)
			val := types.ValResultError(&errPayload)

			flat, err := abi.LowerFlat(nil, resultType, val)
			require.NoError(t, err)
			require.Equal(t, uint64(1), flat[0])

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, resultType, iter)
			require.NoError(t, err)

			isOk, _, errVal := lifted.Result()
			require.False(t, isOk)
			require.Equal(t, v, errVal.S32())
		}
	})
}

// TestResultWithDifferentPayloadTypes tests result with different ok/error types.
func TestResultWithDifferentPayloadTypes(t *testing.T) {
	t.Run("result_u64_u8", func(t *testing.T) {
		// result<u64, u8> - different sizes for ok and error
		resultType := types.Result{Ok: types.U64{}, Error: types.U8{}}

		// Size should be based on max(u64, u8) = u64
		// disc 1 byte + padding to 8 + payload 8 = 16, align 8
		require.Equal(t, uint32(16), resultType.Size())
		require.Equal(t, uint32(8), resultType.Align())
		require.Equal(t, 2, resultType.FlattenCount())

		// Test Ok
		okPayload := types.ValU64(0xFFFFFFFFFFFFFFFF)
		okVal := types.ValResultOk(&okPayload)

		flat, err := abi.LowerFlat(nil, resultType, okVal)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, resultType, iter)
		require.NoError(t, err)

		isOk, liftedOk, _ := lifted.Result()
		require.True(t, isOk)
		require.Equal(t, uint64(0xFFFFFFFFFFFFFFFF), liftedOk.U64())

		// Test Error
		errPayload := types.ValU8(42)
		errVal := types.ValResultError(&errPayload)

		flat, err = abi.LowerFlat(nil, resultType, errVal)
		require.NoError(t, err)

		iter = abi.NewFlatIter(flat)
		lifted, err = abi.LiftFlat(nil, resultType, iter)
		require.NoError(t, err)

		isOk, _, liftedErr := lifted.Result()
		require.False(t, isOk)
		require.Equal(t, uint8(42), liftedErr.U8())
	})

	t.Run("result_unit_ok", func(t *testing.T) {
		// result<_, s32> - no ok payload
		resultType := types.Result{Ok: nil, Error: types.S32{}}

		// Test Ok with nil payload
		okVal := types.ValResultOk(nil)

		flat, err := abi.LowerFlat(nil, resultType, okVal)
		require.NoError(t, err)
		require.Equal(t, uint64(0), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, resultType, iter)
		require.NoError(t, err)

		isOk, liftedOk, _ := lifted.Result()
		require.True(t, isOk)
		require.Nil(t, liftedOk)
	})

	t.Run("result_unit_error", func(t *testing.T) {
		// result<s32, _> - no error payload
		resultType := types.Result{Ok: types.S32{}, Error: nil}

		// Test Error with nil payload
		errVal := types.ValResultError(nil)

		flat, err := abi.LowerFlat(nil, resultType, errVal)
		require.NoError(t, err)
		require.Equal(t, uint64(1), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, resultType, iter)
		require.NoError(t, err)

		isOk, _, liftedErr := lifted.Result()
		require.False(t, isOk)
		require.Nil(t, liftedErr)
	})
}

// TestVariantSingleCase tests a variant with a single case.
// variant { only-case } has minimal size (discriminant only when no payload).
func TestVariantSingleCase(t *testing.T) {
	// Single case with no payload
	variantType := types.Variant{
		Cases: []types.Case{
			{Name: "only-case", Type: nil},
		},
	}

	t.Run("type_properties", func(t *testing.T) {
		// Single case variant: discriminant only (1 byte for <= 256 cases)
		// No payload, so size is just discriminant
		require.Equal(t, uint32(1), variantType.DiscriminantSize(), "1 case should use u8 discriminant")
		require.Equal(t, uint32(1), variantType.Size(), "single case variant should have size 1")
		require.Equal(t, uint32(1), variantType.Align(), "single case variant should have align 1")
		require.Equal(t, 1, variantType.FlattenCount(), "single case variant should have FlattenCount 1")
	})

	t.Run("roundtrip", func(t *testing.T) {
		val := types.ValVariant("only-case", nil)

		flat, err := abi.LowerFlat(nil, variantType, val)
		require.NoError(t, err)
		require.Equal(t, 1, len(flat), "single case variant should lower to 1 flat value")
		require.Equal(t, uint64(0), flat[0], "only-case should have discriminant 0")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, variantType, iter)
		require.NoError(t, err)
		require.Equal(t, types.ValKindVariant, lifted.Kind())

		caseName, payload := lifted.Variant()
		require.Equal(t, "only-case", caseName)
		require.Nil(t, payload, "single case with no payload should have nil payload")
	})
}

// TestVariantSingleCaseWithPayload tests a variant with single case that has payload.
func TestVariantSingleCaseWithPayload(t *testing.T) {
	variantType := types.Variant{
		Cases: []types.Case{
			{Name: "only", Type: types.S32{}},
		},
	}

	t.Run("type_properties", func(t *testing.T) {
		require.Equal(t, uint32(1), variantType.DiscriminantSize())
		// disc 1 + padding to 4 + s32 4 = 8 bytes
		require.Equal(t, uint32(8), variantType.Size())
		require.Equal(t, uint32(4), variantType.Align())
		require.Equal(t, 2, variantType.FlattenCount())
	})

	t.Run("roundtrip", func(t *testing.T) {
		payload := types.ValS32(999)
		val := types.ValVariant("only", &payload)

		flat, err := abi.LowerFlat(nil, variantType, val)
		require.NoError(t, err)
		require.Equal(t, 2, len(flat))

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, variantType, iter)
		require.NoError(t, err)

		caseName, liftedPayload := lifted.Variant()
		require.Equal(t, "only", caseName)
		require.NotNil(t, liftedPayload)
		require.Equal(t, int32(999), liftedPayload.S32())
	})
}

// TestVariantDiscriminantSizeU8 tests variants with 1-255 cases use u8 discriminant.
func TestVariantDiscriminantSizeU8(t *testing.T) {
	t.Run("10_cases", func(t *testing.T) {
		cases := make([]types.Case, 10)
		for i := 0; i < 10; i++ {
			cases[i] = types.Case{Name: fmt.Sprintf("case%d", i), Type: nil}
		}
		variantType := types.Variant{Cases: cases}

		require.Equal(t, uint32(1), variantType.DiscriminantSize(), "10 cases should use u8 discriminant")
		require.Equal(t, uint32(1), variantType.Size(), "variant with 10 cases no payload should have size 1")
		require.Equal(t, uint32(1), variantType.Align())
		require.Equal(t, 1, variantType.FlattenCount())
	})

	t.Run("256_cases", func(t *testing.T) {
		cases := make([]types.Case, 256)
		for i := 0; i < 256; i++ {
			cases[i] = types.Case{Name: fmt.Sprintf("case%d", i), Type: nil}
		}
		variantType := types.Variant{Cases: cases}

		// 256 cases = 0x100, which is exactly at the boundary
		// Per spec: n <= 0x100 (256) uses 1 byte
		require.Equal(t, uint32(1), variantType.DiscriminantSize(), "256 cases should use u8 discriminant")
	})

	t.Run("roundtrip_case_5", func(t *testing.T) {
		cases := make([]types.Case, 10)
		for i := 0; i < 10; i++ {
			cases[i] = types.Case{Name: fmt.Sprintf("case%d", i), Type: nil}
		}
		variantType := types.Variant{Cases: cases}

		val := types.ValVariant("case5", nil)

		flat, err := abi.LowerFlat(nil, variantType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(5), flat[0], "case5 should have discriminant 5")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, variantType, iter)
		require.NoError(t, err)

		caseName, _ := lifted.Variant()
		require.Equal(t, "case5", caseName)
	})

	t.Run("roundtrip_last_case", func(t *testing.T) {
		cases := make([]types.Case, 10)
		for i := 0; i < 10; i++ {
			cases[i] = types.Case{Name: fmt.Sprintf("case%d", i), Type: nil}
		}
		variantType := types.Variant{Cases: cases}

		val := types.ValVariant("case9", nil)

		flat, err := abi.LowerFlat(nil, variantType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(9), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, variantType, iter)
		require.NoError(t, err)

		caseName, _ := lifted.Variant()
		require.Equal(t, "case9", caseName)
	})
}

// TestVariantDiscriminantSizeU16 tests variants with 257-65535 cases use u16 discriminant.
func TestVariantDiscriminantSizeU16(t *testing.T) {
	t.Run("300_cases", func(t *testing.T) {
		cases := make([]types.Case, 300)
		for i := 0; i < 300; i++ {
			cases[i] = types.Case{Name: fmt.Sprintf("case%d", i), Type: nil}
		}
		variantType := types.Variant{Cases: cases}

		// 300 > 256, so uses u16 discriminant (2 bytes)
		require.Equal(t, uint32(2), variantType.DiscriminantSize(), "300 cases should use u16 discriminant")
		require.Equal(t, uint32(2), variantType.Size(), "variant with 300 cases no payload should have size 2")
		require.Equal(t, uint32(2), variantType.Align(), "variant with 300 cases should have align 2")
		require.Equal(t, 1, variantType.FlattenCount())
	})

	t.Run("257_cases", func(t *testing.T) {
		cases := make([]types.Case, 257)
		for i := 0; i < 257; i++ {
			cases[i] = types.Case{Name: fmt.Sprintf("case%d", i), Type: nil}
		}
		variantType := types.Variant{Cases: cases}

		// 257 > 256, so uses u16 discriminant
		require.Equal(t, uint32(2), variantType.DiscriminantSize(), "257 cases should use u16 discriminant")
	})

	t.Run("roundtrip_case_299", func(t *testing.T) {
		cases := make([]types.Case, 300)
		for i := 0; i < 300; i++ {
			cases[i] = types.Case{Name: fmt.Sprintf("case%d", i), Type: nil}
		}
		variantType := types.Variant{Cases: cases}

		val := types.ValVariant("case299", nil)

		flat, err := abi.LowerFlat(nil, variantType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(299), flat[0], "case299 should have discriminant 299")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, variantType, iter)
		require.NoError(t, err)

		caseName, _ := lifted.Variant()
		require.Equal(t, "case299", caseName)
	})
}

// TestVariantMultipleCasesWithPayloads tests variant with multiple cases having different payloads.
func TestVariantMultipleCasesWithPayloads(t *testing.T) {
	variantType := types.Variant{
		Cases: []types.Case{
			{Name: "none", Type: nil},
			{Name: "some-u8", Type: types.U8{}},
			{Name: "some-s32", Type: types.S32{}},
			{Name: "some-u64", Type: types.U64{}},
		},
	}

	t.Run("type_properties", func(t *testing.T) {
		// 4 cases: u8 discriminant (1 byte)
		// Max payload: u64 (8 bytes, align 8)
		// disc 1 + padding to 8 + payload 8 = 16 bytes
		require.Equal(t, uint32(1), variantType.DiscriminantSize())
		require.Equal(t, uint32(16), variantType.Size())
		require.Equal(t, uint32(8), variantType.Align())
		// FlattenCount: 1 (disc) + 1 (max payload u64 flattens to 1) = 2
		require.Equal(t, 2, variantType.FlattenCount())
	})

	t.Run("roundtrip_none", func(t *testing.T) {
		val := types.ValVariant("none", nil)

		flat, err := abi.LowerFlat(nil, variantType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(0), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, variantType, iter)
		require.NoError(t, err)

		caseName, payload := lifted.Variant()
		require.Equal(t, "none", caseName)
		require.Nil(t, payload)
	})

	t.Run("roundtrip_some_u8", func(t *testing.T) {
		payload := types.ValU8(200)
		val := types.ValVariant("some-u8", &payload)

		flat, err := abi.LowerFlat(nil, variantType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(1), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, variantType, iter)
		require.NoError(t, err)

		caseName, liftedPayload := lifted.Variant()
		require.Equal(t, "some-u8", caseName)
		require.NotNil(t, liftedPayload)
		require.Equal(t, uint8(200), liftedPayload.U8())
	})

	t.Run("roundtrip_some_s32", func(t *testing.T) {
		payload := types.ValS32(-12345)
		val := types.ValVariant("some-s32", &payload)

		flat, err := abi.LowerFlat(nil, variantType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(2), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, variantType, iter)
		require.NoError(t, err)

		caseName, liftedPayload := lifted.Variant()
		require.Equal(t, "some-s32", caseName)
		require.NotNil(t, liftedPayload)
		require.Equal(t, int32(-12345), liftedPayload.S32())
	})

	t.Run("roundtrip_some_u64", func(t *testing.T) {
		payload := types.ValU64(0xCAFEBABEDEADBEEF)
		val := types.ValVariant("some-u64", &payload)

		flat, err := abi.LowerFlat(nil, variantType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(3), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, variantType, iter)
		require.NoError(t, err)

		caseName, liftedPayload := lifted.Variant()
		require.Equal(t, "some-u64", caseName)
		require.NotNil(t, liftedPayload)
		require.Equal(t, uint64(0xCAFEBABEDEADBEEF), liftedPayload.U64())
	})
}

// TestVariantPayloadOffset tests variant payload offset calculation.
func TestVariantPayloadOffset(t *testing.T) {
	tests := []struct {
		name           string
		variant        types.Variant
		expectedOffset uint32
	}{
		{
			name: "no_payload",
			variant: types.Variant{Cases: []types.Case{
				{Name: "a", Type: nil},
				{Name: "b", Type: nil},
			}},
			expectedOffset: 1, // disc 1, no payload so offset is at disc end
		},
		{
			name: "u8_payload",
			variant: types.Variant{Cases: []types.Case{
				{Name: "a", Type: types.U8{}},
			}},
			expectedOffset: 1, // disc 1, u8 align 1, offset 1
		},
		{
			name: "s32_payload",
			variant: types.Variant{Cases: []types.Case{
				{Name: "a", Type: types.S32{}},
			}},
			expectedOffset: 4, // disc 1, s32 align 4, padded to 4
		},
		{
			name: "u64_payload",
			variant: types.Variant{Cases: []types.Case{
				{Name: "a", Type: types.U64{}},
			}},
			expectedOffset: 8, // disc 1, u64 align 8, padded to 8
		},
		{
			name: "mixed_payloads_max_align",
			variant: types.Variant{Cases: []types.Case{
				{Name: "a", Type: types.U8{}},
				{Name: "b", Type: types.U64{}},
			}},
			expectedOffset: 8, // max align is 8 (from u64)
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expectedOffset, tc.variant.PayloadOffset())
		})
	}
}

// TestVariantInvalidDiscriminant tests error handling for invalid discriminant.
func TestVariantInvalidDiscriminant(t *testing.T) {
	variantType := types.Variant{
		Cases: []types.Case{
			{Name: "a", Type: nil},
			{Name: "b", Type: nil},
		},
	}

	t.Run("invalid_discriminant_error", func(t *testing.T) {
		// Create flat values with invalid discriminant (2 for a 2-case variant)
		flat := []uint64{2} // Invalid: only 0 and 1 are valid

		iter := abi.NewFlatIter(flat)
		_, err := abi.LiftFlat(nil, variantType, iter)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid variant discriminant")
	})
}

// TestVariantUnknownCaseName tests error handling for unknown case name in LowerFlat.
func TestVariantUnknownCaseName(t *testing.T) {
	variantType := types.Variant{
		Cases: []types.Case{
			{Name: "known", Type: nil},
		},
	}

	t.Run("unknown_case_error", func(t *testing.T) {
		val := types.ValVariant("unknown-case", nil)

		_, err := abi.LowerFlat(nil, variantType, val)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown variant case")
	})
}

// =============================================================================
// List Tests
// Tests for list<T> types using LowerFlat/LiftFlat and LowerHeap/LiftHeap
// =============================================================================

// listTestMemory implements abi.Memory for list tests.
type listTestMemory struct {
	data []byte
}

func (m *listTestMemory) Read(offset, size uint32) ([]byte, bool) {
	end := uint64(offset) + uint64(size)
	if end > uint64(len(m.data)) {
		return nil, false
	}
	return m.data[offset : offset+size], true
}

func (m *listTestMemory) Write(offset uint32, data []byte) bool {
	end := uint64(offset) + uint64(len(data))
	if end > uint64(len(m.data)) {
		return false
	}
	copy(m.data[offset:], data)
	return true
}

func (m *listTestMemory) Size() uint32 {
	return uint32(len(m.data))
}

func newListTestMemory(size int) *listTestMemory {
	return &listTestMemory{data: make([]byte, size)}
}

// TestListEmpty tests list<s32> with empty list.
// Empty list: ptr=0, len=0. No allocation should happen.
func TestListEmpty(t *testing.T) {
	listType := types.List{Element: types.S32{}}

	t.Run("type_properties", func(t *testing.T) {
		// List is represented as (ptr, len), both i32
		require.Equal(t, uint32(8), listType.Size(), "list should have size 8 (ptr + len)")
		require.Equal(t, uint32(4), listType.Align(), "list should have align 4")
		require.Equal(t, 2, listType.FlattenCount(), "list should flatten to 2 values")
		require.Equal(t, uint32(4), listType.ElementSize(), "s32 element should have size 4")
		require.Equal(t, uint32(4), listType.ElementAlign(), "s32 element should have align 4")
	})

	t.Run("lower_flat_empty", func(t *testing.T) {
		val := types.ValList([]types.Val{})

		// No realloc should be called for empty list
		reallocCalled := false
		mem := newListTestMemory(1024)
		ctx := &abi.LowerContext{
			Memory: mem,
			Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
				reallocCalled = true
				return 0, nil
			},
		}

		flat, err := abi.LowerFlat(ctx, listType, val)
		require.NoError(t, err)
		require.Equal(t, 2, len(flat), "empty list should lower to 2 flat values")
		require.Equal(t, uint64(0), flat[0], "empty list ptr should be 0")
		require.Equal(t, uint64(0), flat[1], "empty list len should be 0")
		require.False(t, reallocCalled, "realloc should not be called for empty list")
	})

	t.Run("lift_flat_empty", func(t *testing.T) {
		// Create flat values representing empty list
		flat := []uint64{0, 0}

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, listType, iter)
		require.NoError(t, err)
		require.Equal(t, types.ValKindList, lifted.Kind())

		elems := lifted.List()
		require.Equal(t, 0, len(elems), "empty list should have 0 elements")
	})

	t.Run("heap_roundtrip_empty", func(t *testing.T) {
		mem := newListTestMemory(1024)

		lowerCtx := &abi.LowerContext{
			Memory: mem,
			Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
				return 256, nil
			},
		}

		val := types.ValList([]types.Val{})
		err := abi.LowerHeap(lowerCtx, listType, val, 0)
		require.NoError(t, err)

		// Verify ptr=0, len=0 were written
		ptr := binary.LittleEndian.Uint32(mem.data[0:4])
		length := binary.LittleEndian.Uint32(mem.data[4:8])
		require.Equal(t, uint32(0), ptr)
		require.Equal(t, uint32(0), length)

		// Lift back
		liftCtx := &abi.LiftContext{Memory: mem}
		lifted, err := abi.LiftHeap(liftCtx, listType, 0)
		require.NoError(t, err)
		require.Equal(t, 0, len(lifted.List()))
	})
}

// TestListSingleElement tests list<s32> with one element [42].
func TestListSingleElement(t *testing.T) {
	listType := types.List{Element: types.S32{}}

	t.Run("flat_roundtrip", func(t *testing.T) {
		mem := newListTestMemory(1024)
		allocPtr := uint32(256)

		lowerCtx := &abi.LowerContext{
			Memory: mem,
			Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
				result := allocPtr
				allocPtr += newSize
				return result, nil
			},
		}

		val := types.ValList([]types.Val{types.ValS32(42)})
		flat, err := abi.LowerFlat(lowerCtx, listType, val)
		require.NoError(t, err)
		require.Equal(t, 2, len(flat))
		require.Equal(t, uint64(256), flat[0], "ptr should be 256")
		require.Equal(t, uint64(1), flat[1], "len should be 1")

		// Verify element was written to memory
		elemVal := int32(binary.LittleEndian.Uint32(mem.data[256:260]))
		require.Equal(t, int32(42), elemVal)

		// Lift back
		liftCtx := &abi.LiftContext{Memory: mem}
		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, listType, iter)
		require.NoError(t, err)

		elems := lifted.List()
		require.Equal(t, 1, len(elems))
		require.Equal(t, int32(42), elems[0].S32())
	})

	t.Run("heap_roundtrip", func(t *testing.T) {
		mem := newListTestMemory(1024)
		allocPtr := uint32(256)

		lowerCtx := &abi.LowerContext{
			Memory: mem,
			Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
				result := allocPtr
				allocPtr += newSize
				return result, nil
			},
		}

		val := types.ValList([]types.Val{types.ValS32(42)})
		err := abi.LowerHeap(lowerCtx, listType, val, 0)
		require.NoError(t, err)

		// Verify ptr/len header was written
		ptr := binary.LittleEndian.Uint32(mem.data[0:4])
		length := binary.LittleEndian.Uint32(mem.data[4:8])
		require.Equal(t, uint32(256), ptr)
		require.Equal(t, uint32(1), length)

		// Lift back
		liftCtx := &abi.LiftContext{Memory: mem}
		lifted, err := abi.LiftHeap(liftCtx, listType, 0)
		require.NoError(t, err)

		elems := lifted.List()
		require.Equal(t, 1, len(elems))
		require.Equal(t, int32(42), elems[0].S32())
	})

	t.Run("negative_value", func(t *testing.T) {
		mem := newListTestMemory(1024)
		allocPtr := uint32(256)

		lowerCtx := &abi.LowerContext{
			Memory: mem,
			Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
				result := allocPtr
				allocPtr += newSize
				return result, nil
			},
		}

		val := types.ValList([]types.Val{types.ValS32(-123)})
		flat, err := abi.LowerFlat(lowerCtx, listType, val)
		require.NoError(t, err)

		liftCtx := &abi.LiftContext{Memory: mem}
		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, listType, iter)
		require.NoError(t, err)

		elems := lifted.List()
		require.Equal(t, int32(-123), elems[0].S32())
	})
}

// TestListMultipleElements tests list<s32> with [1, 2, 3, 4, 5].
func TestListMultipleElements(t *testing.T) {
	listType := types.List{Element: types.S32{}}

	t.Run("flat_roundtrip", func(t *testing.T) {
		mem := newListTestMemory(1024)
		allocPtr := uint32(256)

		lowerCtx := &abi.LowerContext{
			Memory: mem,
			Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
				result := allocPtr
				allocPtr += newSize
				return result, nil
			},
		}

		expected := []int32{1, 2, 3, 4, 5}
		elements := make([]types.Val, len(expected))
		for i, v := range expected {
			elements[i] = types.ValS32(v)
		}
		val := types.ValList(elements)

		flat, err := abi.LowerFlat(lowerCtx, listType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(256), flat[0], "ptr should be 256")
		require.Equal(t, uint64(5), flat[1], "len should be 5")

		// Verify all elements in memory
		for i, exp := range expected {
			offset := 256 + uint32(i*4)
			actual := int32(binary.LittleEndian.Uint32(mem.data[offset : offset+4]))
			require.Equal(t, exp, actual, "element %d should match", i)
		}

		// Lift back
		liftCtx := &abi.LiftContext{Memory: mem}
		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, listType, iter)
		require.NoError(t, err)

		elems := lifted.List()
		require.Equal(t, len(expected), len(elems))
		for i, exp := range expected {
			require.Equal(t, exp, elems[i].S32(), "element %d should match after roundtrip", i)
		}
	})

	t.Run("iteration_over_lifted", func(t *testing.T) {
		mem := newListTestMemory(1024)
		allocPtr := uint32(256)

		lowerCtx := &abi.LowerContext{
			Memory: mem,
			Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
				result := allocPtr
				allocPtr += newSize
				return result, nil
			},
		}

		elements := []types.Val{
			types.ValS32(10),
			types.ValS32(20),
			types.ValS32(30),
		}
		val := types.ValList(elements)

		flat, err := abi.LowerFlat(lowerCtx, listType, val)
		require.NoError(t, err)

		liftCtx := &abi.LiftContext{Memory: mem}
		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, listType, iter)
		require.NoError(t, err)

		// Test iteration
		sum := int32(0)
		for _, elem := range lifted.List() {
			sum += elem.S32()
		}
		require.Equal(t, int32(60), sum)
	})
}

// TestListNested tests list<list<s32>> (nested lists).
func TestListNested(t *testing.T) {
	innerListType := types.List{Element: types.S32{}}
	outerListType := types.List{Element: innerListType}

	t.Run("type_properties", func(t *testing.T) {
		// Inner list element type is (ptr, len) = 8 bytes, align 4
		require.Equal(t, uint32(8), innerListType.Size())
		require.Equal(t, uint32(4), innerListType.Align())

		// Outer list of inner lists
		require.Equal(t, uint32(8), outerListType.Size())
		require.Equal(t, uint32(4), outerListType.Align())
		require.Equal(t, uint32(8), outerListType.ElementSize(), "element of outer list is inner list (8 bytes)")
		require.Equal(t, uint32(4), outerListType.ElementAlign())
	})

	t.Run("heap_roundtrip_nested", func(t *testing.T) {
		mem := newListTestMemory(4096)
		allocPtr := uint32(512)

		lowerCtx := &abi.LowerContext{
			Memory: mem,
			Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
				// Align the allocation
				if allocPtr%align != 0 {
					allocPtr += align - (allocPtr % align)
				}
				result := allocPtr
				allocPtr += newSize
				return result, nil
			},
		}

		// Create nested list: [[1, 2], [3, 4, 5]]
		inner1 := types.ValList([]types.Val{
			types.ValS32(1),
			types.ValS32(2),
		})
		inner2 := types.ValList([]types.Val{
			types.ValS32(3),
			types.ValS32(4),
			types.ValS32(5),
		})
		outer := types.ValList([]types.Val{inner1, inner2})

		err := abi.LowerHeap(lowerCtx, outerListType, outer, 0)
		require.NoError(t, err)

		// Lift back
		liftCtx := &abi.LiftContext{Memory: mem}
		lifted, err := abi.LiftHeap(liftCtx, outerListType, 0)
		require.NoError(t, err)

		outerElems := lifted.List()
		require.Equal(t, 2, len(outerElems), "outer list should have 2 elements")

		// Check first inner list
		inner1Elems := outerElems[0].List()
		require.Equal(t, 2, len(inner1Elems))
		require.Equal(t, int32(1), inner1Elems[0].S32())
		require.Equal(t, int32(2), inner1Elems[1].S32())

		// Check second inner list
		inner2Elems := outerElems[1].List()
		require.Equal(t, 3, len(inner2Elems))
		require.Equal(t, int32(3), inner2Elems[0].S32())
		require.Equal(t, int32(4), inner2Elems[1].S32())
		require.Equal(t, int32(5), inner2Elems[2].S32())
	})

	t.Run("empty_inner_lists", func(t *testing.T) {
		mem := newListTestMemory(4096)
		allocPtr := uint32(512)

		lowerCtx := &abi.LowerContext{
			Memory: mem,
			Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
				if allocPtr%align != 0 {
					allocPtr += align - (allocPtr % align)
				}
				result := allocPtr
				allocPtr += newSize
				return result, nil
			},
		}

		// Create nested list with empty inner list: [[], [1]]
		inner1 := types.ValList([]types.Val{})
		inner2 := types.ValList([]types.Val{types.ValS32(1)})
		outer := types.ValList([]types.Val{inner1, inner2})

		err := abi.LowerHeap(lowerCtx, outerListType, outer, 0)
		require.NoError(t, err)

		liftCtx := &abi.LiftContext{Memory: mem}
		lifted, err := abi.LiftHeap(liftCtx, outerListType, 0)
		require.NoError(t, err)

		outerElems := lifted.List()
		require.Equal(t, 2, len(outerElems))
		require.Equal(t, 0, len(outerElems[0].List()), "first inner list should be empty")
		require.Equal(t, 1, len(outerElems[1].List()), "second inner list should have 1 element")
		require.Equal(t, int32(1), outerElems[1].List()[0].S32())
	})
}

// TestListMaxLength tests with 1000 elements.
func TestListMaxLength(t *testing.T) {
	listType := types.List{Element: types.S32{}}

	t.Run("large_list_roundtrip", func(t *testing.T) {
		mem := newListTestMemory(64 * 1024) // 64KB
		allocPtr := uint32(1024)

		lowerCtx := &abi.LowerContext{
			Memory: mem,
			Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
				result := allocPtr
				allocPtr += newSize
				return result, nil
			},
		}

		// Create list with 1000 elements
		numElements := 1000
		elements := make([]types.Val, numElements)
		for i := 0; i < numElements; i++ {
			elements[i] = types.ValS32(int32(i * 2)) // 0, 2, 4, 6, ...
		}
		val := types.ValList(elements)

		flat, err := abi.LowerFlat(lowerCtx, listType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(numElements), flat[1], "len should be 1000")

		// Lift back
		liftCtx := &abi.LiftContext{Memory: mem}
		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, listType, iter)
		require.NoError(t, err)

		elems := lifted.List()
		require.Equal(t, numElements, len(elems))

		// Verify all elements
		for i := 0; i < numElements; i++ {
			require.Equal(t, int32(i*2), elems[i].S32(), "element %d should be %d", i, i*2)
		}
	})
}

// TestListDifferentElementTypes tests lists with various element types.
func TestListDifferentElementTypes(t *testing.T) {
	t.Run("list_u8", func(t *testing.T) {
		listType := types.List{Element: types.U8{}}
		require.Equal(t, uint32(1), listType.ElementSize())
		require.Equal(t, uint32(1), listType.ElementAlign())

		mem := newListTestMemory(1024)
		allocPtr := uint32(256)

		lowerCtx := &abi.LowerContext{
			Memory: mem,
			Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
				result := allocPtr
				allocPtr += newSize
				return result, nil
			},
		}

		val := types.ValList([]types.Val{
			types.ValU8(0),
			types.ValU8(127),
			types.ValU8(255),
		})

		flat, err := abi.LowerFlat(lowerCtx, listType, val)
		require.NoError(t, err)

		liftCtx := &abi.LiftContext{Memory: mem}
		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, listType, iter)
		require.NoError(t, err)

		elems := lifted.List()
		require.Equal(t, 3, len(elems))
		require.Equal(t, uint8(0), elems[0].U8())
		require.Equal(t, uint8(127), elems[1].U8())
		require.Equal(t, uint8(255), elems[2].U8())
	})

	t.Run("list_u64", func(t *testing.T) {
		listType := types.List{Element: types.U64{}}
		require.Equal(t, uint32(8), listType.ElementSize())
		require.Equal(t, uint32(8), listType.ElementAlign())

		mem := newListTestMemory(1024)
		allocPtr := uint32(256)

		lowerCtx := &abi.LowerContext{
			Memory: mem,
			Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
				if allocPtr%align != 0 {
					allocPtr += align - (allocPtr % align)
				}
				result := allocPtr
				allocPtr += newSize
				return result, nil
			},
		}

		val := types.ValList([]types.Val{
			types.ValU64(0),
			types.ValU64(math.MaxUint64),
			types.ValU64(0xDEADBEEFCAFEBABE),
		})

		flat, err := abi.LowerFlat(lowerCtx, listType, val)
		require.NoError(t, err)

		liftCtx := &abi.LiftContext{Memory: mem}
		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, listType, iter)
		require.NoError(t, err)

		elems := lifted.List()
		require.Equal(t, 3, len(elems))
		require.Equal(t, uint64(0), elems[0].U64())
		require.Equal(t, uint64(math.MaxUint64), elems[1].U64())
		require.Equal(t, uint64(0xDEADBEEFCAFEBABE), elems[2].U64())
	})

	t.Run("list_f32", func(t *testing.T) {
		listType := types.List{Element: types.F32{}}

		mem := newListTestMemory(1024)
		allocPtr := uint32(256)

		lowerCtx := &abi.LowerContext{
			Memory: mem,
			Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
				result := allocPtr
				allocPtr += newSize
				return result, nil
			},
		}

		val := types.ValList([]types.Val{
			types.ValF32(0.0),
			types.ValF32(3.14159),
			types.ValF32(-1.5),
		})

		flat, err := abi.LowerFlat(lowerCtx, listType, val)
		require.NoError(t, err)

		liftCtx := &abi.LiftContext{Memory: mem}
		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, listType, iter)
		require.NoError(t, err)

		elems := lifted.List()
		require.Equal(t, 3, len(elems))
		require.Equal(t, float32(0.0), elems[0].F32())
		require.Equal(t, math.Float32bits(3.14159), math.Float32bits(elems[1].F32()))
		require.Equal(t, float32(-1.5), elems[2].F32())
	})

	t.Run("list_bool", func(t *testing.T) {
		listType := types.List{Element: types.Bool{}}

		mem := newListTestMemory(1024)
		allocPtr := uint32(256)

		lowerCtx := &abi.LowerContext{
			Memory: mem,
			Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
				result := allocPtr
				allocPtr += newSize
				return result, nil
			},
		}

		val := types.ValList([]types.Val{
			types.ValBool(true),
			types.ValBool(false),
			types.ValBool(true),
		})

		flat, err := abi.LowerFlat(lowerCtx, listType, val)
		require.NoError(t, err)

		liftCtx := &abi.LiftContext{Memory: mem}
		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, listType, iter)
		require.NoError(t, err)

		elems := lifted.List()
		require.Equal(t, 3, len(elems))
		require.True(t, elems[0].Bool())
		require.False(t, elems[1].Bool())
		require.True(t, elems[2].Bool())
	})
}

// TestListOfRecords tests list<record { x: s32, y: s32 }>.
func TestListOfRecords(t *testing.T) {
	pointRecord := types.Record{
		Fields: []types.Field{
			{Name: "x", Type: types.S32{}},
			{Name: "y", Type: types.S32{}},
		},
	}
	listType := types.List{Element: pointRecord}

	t.Run("type_properties", func(t *testing.T) {
		require.Equal(t, uint32(8), listType.ElementSize(), "point record should be 8 bytes")
		require.Equal(t, uint32(4), listType.ElementAlign(), "point record should have align 4")
	})

	t.Run("roundtrip", func(t *testing.T) {
		mem := newListTestMemory(1024)
		allocPtr := uint32(256)

		lowerCtx := &abi.LowerContext{
			Memory: mem,
			Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
				result := allocPtr
				allocPtr += newSize
				return result, nil
			},
		}

		// Create list of points: [(1, 2), (3, 4), (5, 6)]
		points := []types.Val{
			types.ValRecord(map[string]types.Val{
				"x": types.ValS32(1),
				"y": types.ValS32(2),
			}),
			types.ValRecord(map[string]types.Val{
				"x": types.ValS32(3),
				"y": types.ValS32(4),
			}),
			types.ValRecord(map[string]types.Val{
				"x": types.ValS32(5),
				"y": types.ValS32(6),
			}),
		}
		val := types.ValList(points)

		err := abi.LowerHeap(lowerCtx, listType, val, 0)
		require.NoError(t, err)

		liftCtx := &abi.LiftContext{Memory: mem}
		lifted, err := abi.LiftHeap(liftCtx, listType, 0)
		require.NoError(t, err)

		elems := lifted.List()
		require.Equal(t, 3, len(elems))

		for i, elem := range elems {
			xVal, ok := elem.RecordField("x")
			require.True(t, ok)
			yVal, ok := elem.RecordField("y")
			require.True(t, ok)
			require.Equal(t, int32(i*2+1), xVal.S32())
			require.Equal(t, int32(i*2+2), yVal.S32())
		}
	})
}

// TestListBoundsChecking tests that out-of-bounds access is handled.
func TestListBoundsChecking(t *testing.T) {
	listType := types.List{Element: types.S32{}}

	t.Run("lift_out_of_bounds", func(t *testing.T) {
		mem := newListTestMemory(64)

		// Set up ptr/len that would go beyond memory
		binary.LittleEndian.PutUint32(mem.data[0:], 100) // ptr beyond 64-byte memory
		binary.LittleEndian.PutUint32(mem.data[4:], 10)  // 10 elements

		liftCtx := &abi.LiftContext{Memory: mem}
		_, err := abi.LiftHeap(liftCtx, listType, 0)
		require.Error(t, err)
		require.Contains(t, err.Error(), "exceeds memory bounds")
	})

	t.Run("lift_ptr_plus_size_overflow", func(t *testing.T) {
		mem := newListTestMemory(64)

		// Set up valid ptr but len would overflow when multiplied by element size
		binary.LittleEndian.PutUint32(mem.data[0:], 32)
		binary.LittleEndian.PutUint32(mem.data[4:], 20) // 20 * 4 = 80, 32 + 80 > 64

		liftCtx := &abi.LiftContext{Memory: mem}
		_, err := abi.LiftHeap(liftCtx, listType, 0)
		require.Error(t, err)
	})

	t.Run("lift_flat_no_memory_for_nonempty", func(t *testing.T) {
		// Non-empty list requires memory context
		flat := []uint64{256, 5} // ptr=256, len=5

		iter := abi.NewFlatIter(flat)
		_, err := abi.LiftFlat(nil, listType, iter)
		require.Error(t, err)
		require.Contains(t, err.Error(), "memory context required")
	})
}

// TestListNoReallocError tests that lowering non-empty lists without realloc fails.
func TestListNoReallocError(t *testing.T) {
	listType := types.List{Element: types.S32{}}

	t.Run("lower_flat_no_realloc", func(t *testing.T) {
		val := types.ValList([]types.Val{types.ValS32(42)})

		// No realloc function provided
		_, err := abi.LowerFlat(nil, listType, val)
		require.Error(t, err)
		require.Contains(t, err.Error(), "realloc function required")
	})

	t.Run("lower_heap_no_realloc", func(t *testing.T) {
		mem := newListTestMemory(1024)
		lowerCtx := &abi.LowerContext{
			Memory:  mem,
			Realloc: nil, // No realloc
		}

		val := types.ValList([]types.Val{types.ValS32(42)})
		err := abi.LowerHeap(lowerCtx, listType, val, 0)
		require.Error(t, err)
		require.Contains(t, err.Error(), "realloc function required")
	})
}

// =============================================================================
// Flags Tests
// =============================================================================

// TestFlagsEmpty tests empty flags handling.
// flags {} has size 0, align 1, FlattenCount 0.
func TestFlagsEmpty(t *testing.T) {
	emptyFlags := types.Flags{Names: []string{}}

	t.Run("type_properties", func(t *testing.T) {
		require.Equal(t, uint32(0), emptyFlags.Size(), "empty flags should have size 0")
		require.Equal(t, uint32(1), emptyFlags.Align(), "empty flags should have align 1")
		require.Equal(t, 0, emptyFlags.FlattenCount(), "empty flags should have FlattenCount 0")
	})

	t.Run("roundtrip_flat", func(t *testing.T) {
		val := types.ValFlags(map[string]bool{})

		flat, err := abi.LowerFlat(nil, emptyFlags, val)
		require.NoError(t, err)
		require.Equal(t, 0, len(flat), "empty flags should lower to 0 flat values")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, emptyFlags, iter)
		require.NoError(t, err)
		require.Equal(t, types.ValKindFlags, lifted.Kind())
		require.Equal(t, 0, len(lifted.Flags()))
	})

	t.Run("roundtrip_heap", func(t *testing.T) {
		mem := newListTestMemory(64)
		lowerCtx := &abi.LowerContext{Memory: mem}
		liftCtx := &abi.LiftContext{Memory: mem}

		val := types.ValFlags(map[string]bool{})

		err := abi.LowerHeap(lowerCtx, emptyFlags, val, 0)
		require.NoError(t, err)

		lifted, err := abi.LiftHeap(liftCtx, emptyFlags, 0)
		require.NoError(t, err)
		require.Equal(t, types.ValKindFlags, lifted.Kind())
		require.Equal(t, 0, len(lifted.Flags()))
	})
}

// TestFlagsSingleFlag tests flags with a single flag.
// flags { read } has size 1 (u8), align 1.
func TestFlagsSingleFlag(t *testing.T) {
	singleFlag := types.Flags{Names: []string{"read"}}

	t.Run("type_properties", func(t *testing.T) {
		require.Equal(t, uint32(1), singleFlag.Size(), "single flag should have size 1")
		require.Equal(t, uint32(1), singleFlag.Align(), "single flag should have align 1")
		require.Equal(t, 1, singleFlag.FlattenCount(), "single flag should have FlattenCount 1")
	})

	t.Run("flag_true", func(t *testing.T) {
		val := types.ValFlags(map[string]bool{"read": true})

		flat, err := abi.LowerFlat(nil, singleFlag, val)
		require.NoError(t, err)
		require.Equal(t, 1, len(flat))
		require.Equal(t, uint64(1), flat[0], "read=true should be bit 0 set")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, singleFlag, iter)
		require.NoError(t, err)
		require.True(t, lifted.Flags()["read"])
	})

	t.Run("flag_false", func(t *testing.T) {
		val := types.ValFlags(map[string]bool{"read": false})

		flat, err := abi.LowerFlat(nil, singleFlag, val)
		require.NoError(t, err)
		require.Equal(t, 1, len(flat))
		require.Equal(t, uint64(0), flat[0], "read=false should be 0")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, singleFlag, iter)
		require.NoError(t, err)
		require.False(t, lifted.Flags()["read"])
	})

	t.Run("roundtrip_heap", func(t *testing.T) {
		mem := newListTestMemory(64)
		lowerCtx := &abi.LowerContext{Memory: mem}
		liftCtx := &abi.LiftContext{Memory: mem}

		val := types.ValFlags(map[string]bool{"read": true})

		err := abi.LowerHeap(lowerCtx, singleFlag, val, 0)
		require.NoError(t, err)

		// Verify byte in memory
		require.Equal(t, uint8(1), mem.data[0], "memory should have bit 0 set")

		lifted, err := abi.LiftHeap(liftCtx, singleFlag, 0)
		require.NoError(t, err)
		require.True(t, lifted.Flags()["read"])
	})
}

// TestFlagsMultipleFlags tests flags with multiple flags.
// flags { read, write, execute } - 3 flags fits in u8.
func TestFlagsMultipleFlags(t *testing.T) {
	multiFlags := types.Flags{Names: []string{"read", "write", "execute"}}

	t.Run("type_properties", func(t *testing.T) {
		require.Equal(t, uint32(1), multiFlags.Size(), "3 flags should have size 1 (u8)")
		require.Equal(t, uint32(1), multiFlags.Align(), "3 flags should have align 1")
		require.Equal(t, 1, multiFlags.FlattenCount(), "3 flags should have FlattenCount 1")
	})

	t.Run("read_execute_combination", func(t *testing.T) {
		// read=true (bit 0), write=false (bit 1), execute=true (bit 2)
		// Expected: 0b101 = 5
		val := types.ValFlags(map[string]bool{
			"read":    true,
			"write":   false,
			"execute": true,
		})

		flat, err := abi.LowerFlat(nil, multiFlags, val)
		require.NoError(t, err)
		require.Equal(t, 1, len(flat))
		require.Equal(t, uint64(0b101), flat[0], "read+execute should be 0b101")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, multiFlags, iter)
		require.NoError(t, err)
		flags := lifted.Flags()
		require.True(t, flags["read"])
		require.False(t, flags["write"])
		require.True(t, flags["execute"])
	})

	t.Run("all_flags_set", func(t *testing.T) {
		val := types.ValFlags(map[string]bool{
			"read":    true,
			"write":   true,
			"execute": true,
		})

		flat, err := abi.LowerFlat(nil, multiFlags, val)
		require.NoError(t, err)
		require.Equal(t, uint64(0b111), flat[0], "all flags should be 0b111")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, multiFlags, iter)
		require.NoError(t, err)
		flags := lifted.Flags()
		require.True(t, flags["read"])
		require.True(t, flags["write"])
		require.True(t, flags["execute"])
	})

	t.Run("no_flags_set", func(t *testing.T) {
		val := types.ValFlags(map[string]bool{
			"read":    false,
			"write":   false,
			"execute": false,
		})

		flat, err := abi.LowerFlat(nil, multiFlags, val)
		require.NoError(t, err)
		require.Equal(t, uint64(0), flat[0], "no flags should be 0")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, multiFlags, iter)
		require.NoError(t, err)
		flags := lifted.Flags()
		require.False(t, flags["read"])
		require.False(t, flags["write"])
		require.False(t, flags["execute"])
	})

	t.Run("roundtrip_heap_preserves_bits", func(t *testing.T) {
		mem := newListTestMemory(64)
		lowerCtx := &abi.LowerContext{Memory: mem}
		liftCtx := &abi.LiftContext{Memory: mem}

		val := types.ValFlags(map[string]bool{
			"read":    true,
			"write":   false,
			"execute": true,
		})

		err := abi.LowerHeap(lowerCtx, multiFlags, val, 0)
		require.NoError(t, err)

		// Verify byte in memory
		require.Equal(t, uint8(0b101), mem.data[0], "memory should have bits 0 and 2 set")

		lifted, err := abi.LiftHeap(liftCtx, multiFlags, 0)
		require.NoError(t, err)
		flags := lifted.Flags()
		require.True(t, flags["read"])
		require.False(t, flags["write"])
		require.True(t, flags["execute"])
	})
}

// TestFlagsLarge tests flags with 9 flags (requires u16).
// 9 flags requires 2 bytes (u16), align 2.
func TestFlagsLarge(t *testing.T) {
	// Create 9 flag names
	names := make([]string, 9)
	for i := 0; i < 9; i++ {
		names[i] = fmt.Sprintf("flag%d", i)
	}
	largeFlags := types.Flags{Names: names}

	t.Run("type_properties", func(t *testing.T) {
		require.Equal(t, uint32(2), largeFlags.Size(), "9 flags should have size 2 (u16)")
		require.Equal(t, uint32(2), largeFlags.Align(), "9 flags should have align 2")
		require.Equal(t, 1, largeFlags.FlattenCount(), "9 flags should have FlattenCount 1")
	})

	t.Run("boundary_flags", func(t *testing.T) {
		// Set flag0 (bit 0) and flag8 (bit 8)
		flagMap := make(map[string]bool)
		for _, name := range names {
			flagMap[name] = false
		}
		flagMap["flag0"] = true
		flagMap["flag8"] = true

		val := types.ValFlags(flagMap)

		flat, err := abi.LowerFlat(nil, largeFlags, val)
		require.NoError(t, err)
		require.Equal(t, 1, len(flat))
		require.Equal(t, uint64((1<<0)|(1<<8)), flat[0], "should have bits 0 and 8 set")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, largeFlags, iter)
		require.NoError(t, err)
		flags := lifted.Flags()
		require.True(t, flags["flag0"])
		require.True(t, flags["flag8"])
		for i := 1; i < 8; i++ {
			require.False(t, flags[fmt.Sprintf("flag%d", i)])
		}
	})

	t.Run("roundtrip_heap", func(t *testing.T) {
		mem := newListTestMemory(64)
		lowerCtx := &abi.LowerContext{Memory: mem}
		liftCtx := &abi.LiftContext{Memory: mem}

		flagMap := make(map[string]bool)
		for _, name := range names {
			flagMap[name] = false
		}
		flagMap["flag0"] = true
		flagMap["flag8"] = true

		val := types.ValFlags(flagMap)

		// Ensure 2-byte alignment
		err := abi.LowerHeap(lowerCtx, largeFlags, val, 0)
		require.NoError(t, err)

		// Verify u16 in memory (little-endian)
		expected := uint16((1 << 0) | (1 << 8))
		actual := binary.LittleEndian.Uint16(mem.data[0:2])
		require.Equal(t, expected, actual, "memory should have u16 with bits 0 and 8 set")

		lifted, err := abi.LiftHeap(liftCtx, largeFlags, 0)
		require.NoError(t, err)
		flags := lifted.Flags()
		require.True(t, flags["flag0"])
		require.True(t, flags["flag8"])
	})
}

// TestFlagsVeryLarge tests flags with 17 flags (requires u32).
// 17 flags requires 4 bytes (u32), align 4.
func TestFlagsVeryLarge(t *testing.T) {
	// Create 17 flag names
	names := make([]string, 17)
	for i := 0; i < 17; i++ {
		names[i] = fmt.Sprintf("flag%d", i)
	}
	veryLargeFlags := types.Flags{Names: names}

	t.Run("type_properties", func(t *testing.T) {
		require.Equal(t, uint32(4), veryLargeFlags.Size(), "17 flags should have size 4 (u32)")
		require.Equal(t, uint32(4), veryLargeFlags.Align(), "17 flags should have align 4")
		require.Equal(t, 1, veryLargeFlags.FlattenCount(), "17 flags should have FlattenCount 1")
	})

	t.Run("boundary_flags", func(t *testing.T) {
		// Set flag0 (bit 0) and flag16 (bit 16)
		flagMap := make(map[string]bool)
		for _, name := range names {
			flagMap[name] = false
		}
		flagMap["flag0"] = true
		flagMap["flag16"] = true

		val := types.ValFlags(flagMap)

		flat, err := abi.LowerFlat(nil, veryLargeFlags, val)
		require.NoError(t, err)
		require.Equal(t, 1, len(flat))
		require.Equal(t, uint64((1<<0)|(1<<16)), flat[0], "should have bits 0 and 16 set")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, veryLargeFlags, iter)
		require.NoError(t, err)
		flags := lifted.Flags()
		require.True(t, flags["flag0"])
		require.True(t, flags["flag16"])
	})

	t.Run("all_flags_set", func(t *testing.T) {
		flagMap := make(map[string]bool)
		for _, name := range names {
			flagMap[name] = true
		}

		val := types.ValFlags(flagMap)

		flat, err := abi.LowerFlat(nil, veryLargeFlags, val)
		require.NoError(t, err)
		require.Equal(t, 1, len(flat))
		// All 17 bits set: (1 << 17) - 1 = 0x1FFFF
		require.Equal(t, uint64(0x1FFFF), flat[0], "all 17 flags should be set")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, veryLargeFlags, iter)
		require.NoError(t, err)
		flags := lifted.Flags()
		for _, name := range names {
			require.True(t, flags[name], "%s should be set", name)
		}
	})

	t.Run("roundtrip_heap", func(t *testing.T) {
		mem := newListTestMemory(64)
		lowerCtx := &abi.LowerContext{Memory: mem}
		liftCtx := &abi.LiftContext{Memory: mem}

		flagMap := make(map[string]bool)
		for _, name := range names {
			flagMap[name] = false
		}
		flagMap["flag0"] = true
		flagMap["flag16"] = true

		val := types.ValFlags(flagMap)

		err := abi.LowerHeap(lowerCtx, veryLargeFlags, val, 0)
		require.NoError(t, err)

		// Verify u32 in memory (little-endian)
		expected := uint32((1 << 0) | (1 << 16))
		actual := binary.LittleEndian.Uint32(mem.data[0:4])
		require.Equal(t, expected, actual, "memory should have u32 with bits 0 and 16 set")

		lifted, err := abi.LiftHeap(liftCtx, veryLargeFlags, 0)
		require.NoError(t, err)
		flags := lifted.Flags()
		require.True(t, flags["flag0"])
		require.True(t, flags["flag16"])
	})
}

// TestFlagsSizeThresholds tests size transitions at 8, 16 flag boundaries.
func TestFlagsSizeThresholds(t *testing.T) {
	testCases := []struct {
		numFlags      int
		expectedSize  uint32
		expectedAlign uint32
	}{
		{1, 1, 1},  // 1-8 flags: u8
		{8, 1, 1},  // exactly 8 flags: u8
		{9, 2, 2},  // 9-16 flags: u16
		{16, 2, 2}, // exactly 16 flags: u16
		{17, 4, 4}, // 17+ flags: u32
		{32, 4, 4}, // exactly 32 flags: single u32
		{33, 8, 4}, // 33 flags: 2 u32s
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%d_flags", tc.numFlags), func(t *testing.T) {
			names := make([]string, tc.numFlags)
			for i := 0; i < tc.numFlags; i++ {
				names[i] = fmt.Sprintf("f%d", i)
			}
			flags := types.Flags{Names: names}

			require.Equal(t, tc.expectedSize, flags.Size(), "%d flags should have size %d", tc.numFlags, tc.expectedSize)
			require.Equal(t, tc.expectedAlign, flags.Align(), "%d flags should have align %d", tc.numFlags, tc.expectedAlign)
		})
	}
}

// =============================================================================
// Enum Tests
// =============================================================================

// TestEnumSimple tests a simple enum with 3 cases.
// enum { red, green, blue } has size 1, align 1.
func TestEnumSimple(t *testing.T) {
	colorEnum := types.Enum{Cases: []string{"red", "green", "blue"}}

	t.Run("type_properties", func(t *testing.T) {
		require.Equal(t, uint32(1), colorEnum.Size(), "3-case enum should have size 1 (u8)")
		require.Equal(t, uint32(1), colorEnum.Align(), "3-case enum should have align 1")
		require.Equal(t, 1, colorEnum.FlattenCount(), "enum should have FlattenCount 1")
	})

	t.Run("select_first_case", func(t *testing.T) {
		val := types.ValEnum("red")

		flat, err := abi.LowerFlat(nil, colorEnum, val)
		require.NoError(t, err)
		require.Equal(t, 1, len(flat))
		require.Equal(t, uint64(0), flat[0], "red should be discriminant 0")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, colorEnum, iter)
		require.NoError(t, err)
		require.Equal(t, "red", lifted.Enum())
	})

	t.Run("select_middle_case", func(t *testing.T) {
		val := types.ValEnum("green")

		flat, err := abi.LowerFlat(nil, colorEnum, val)
		require.NoError(t, err)
		require.Equal(t, uint64(1), flat[0], "green should be discriminant 1")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, colorEnum, iter)
		require.NoError(t, err)
		require.Equal(t, "green", lifted.Enum())
	})

	t.Run("select_last_case", func(t *testing.T) {
		val := types.ValEnum("blue")

		flat, err := abi.LowerFlat(nil, colorEnum, val)
		require.NoError(t, err)
		require.Equal(t, uint64(2), flat[0], "blue should be discriminant 2")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, colorEnum, iter)
		require.NoError(t, err)
		require.Equal(t, "blue", lifted.Enum())
	})

	t.Run("roundtrip_heap", func(t *testing.T) {
		mem := newListTestMemory(64)
		lowerCtx := &abi.LowerContext{Memory: mem}
		liftCtx := &abi.LiftContext{Memory: mem}

		val := types.ValEnum("green")

		err := abi.LowerHeap(lowerCtx, colorEnum, val, 0)
		require.NoError(t, err)

		// Verify byte in memory
		require.Equal(t, uint8(1), mem.data[0], "memory should have discriminant 1")

		lifted, err := abi.LiftHeap(liftCtx, colorEnum, 0)
		require.NoError(t, err)
		require.Equal(t, "green", lifted.Enum())
	})

	t.Run("all_cases_roundtrip", func(t *testing.T) {
		cases := []string{"red", "green", "blue"}
		for i, caseName := range cases {
			val := types.ValEnum(caseName)

			flat, err := abi.LowerFlat(nil, colorEnum, val)
			require.NoError(t, err)
			require.Equal(t, uint64(i), flat[0], "%s should be discriminant %d", caseName, i)

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, colorEnum, iter)
			require.NoError(t, err)
			require.Equal(t, caseName, lifted.Enum())
		}
	})
}

// TestEnumLarge tests an enum with more than 256 cases (requires u16).
func TestEnumLarge(t *testing.T) {
	// Create 257 case names to force u16 discriminant
	cases := make([]string, 257)
	for i := 0; i < 257; i++ {
		cases[i] = fmt.Sprintf("case%d", i)
	}
	largeEnum := types.Enum{Cases: cases}

	t.Run("type_properties", func(t *testing.T) {
		require.Equal(t, uint32(2), largeEnum.Size(), "257-case enum should have size 2 (u16)")
		require.Equal(t, uint32(2), largeEnum.Align(), "257-case enum should have align 2")
		require.Equal(t, 1, largeEnum.FlattenCount(), "enum should have FlattenCount 1")
	})

	t.Run("select_first_case", func(t *testing.T) {
		val := types.ValEnum("case0")

		flat, err := abi.LowerFlat(nil, largeEnum, val)
		require.NoError(t, err)
		require.Equal(t, uint64(0), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, largeEnum, iter)
		require.NoError(t, err)
		require.Equal(t, "case0", lifted.Enum())
	})

	t.Run("select_case_256", func(t *testing.T) {
		// This case (index 256) is beyond u8 range
		val := types.ValEnum("case256")

		flat, err := abi.LowerFlat(nil, largeEnum, val)
		require.NoError(t, err)
		require.Equal(t, uint64(256), flat[0], "case256 should be discriminant 256")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, largeEnum, iter)
		require.NoError(t, err)
		require.Equal(t, "case256", lifted.Enum())
	})

	t.Run("roundtrip_heap", func(t *testing.T) {
		mem := newListTestMemory(64)
		lowerCtx := &abi.LowerContext{Memory: mem}
		liftCtx := &abi.LiftContext{Memory: mem}

		val := types.ValEnum("case256")

		err := abi.LowerHeap(lowerCtx, largeEnum, val, 0)
		require.NoError(t, err)

		// Verify u16 in memory (little-endian)
		actual := binary.LittleEndian.Uint16(mem.data[0:2])
		require.Equal(t, uint16(256), actual, "memory should have discriminant 256")

		lifted, err := abi.LiftHeap(liftCtx, largeEnum, 0)
		require.NoError(t, err)
		require.Equal(t, "case256", lifted.Enum())
	})
}

// TestEnumSizeThresholds tests size transitions at 256, 65536 case boundaries.
func TestEnumSizeThresholds(t *testing.T) {
	testCases := []struct {
		numCases      int
		expectedSize  uint32
		expectedAlign uint32
	}{
		{1, 1, 1},     // 1-256 cases: u8
		{256, 1, 1},   // exactly 256 cases: u8
		{257, 2, 2},   // 257-65536 cases: u16
		{65536, 2, 2}, // exactly 65536 cases: u16
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%d_cases", tc.numCases), func(t *testing.T) {
			cases := make([]string, tc.numCases)
			for i := 0; i < tc.numCases; i++ {
				cases[i] = fmt.Sprintf("c%d", i)
			}
			enum := types.Enum{Cases: cases}

			require.Equal(t, tc.expectedSize, enum.Size(), "%d cases should have size %d", tc.numCases, tc.expectedSize)
			require.Equal(t, tc.expectedAlign, enum.Align(), "%d cases should have align %d", tc.numCases, tc.expectedAlign)
		})
	}
}

// TestEnumInvalidCase tests that lowering an invalid enum case returns an error.
func TestEnumInvalidCase(t *testing.T) {
	colorEnum := types.Enum{Cases: []string{"red", "green", "blue"}}

	val := types.ValEnum("yellow") // Not a valid case

	_, err := abi.LowerFlat(nil, colorEnum, val)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown enum case")
}

// TestEnumInvalidDiscriminant tests that lifting an invalid discriminant returns an error.
func TestEnumInvalidDiscriminant(t *testing.T) {
	colorEnum := types.Enum{Cases: []string{"red", "green", "blue"}}

	flat := []uint64{5} // Discriminant 5 is out of range [0, 2]

	iter := abi.NewFlatIter(flat)
	_, err := abi.LiftFlat(nil, colorEnum, iter)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid enum discriminant")
}
