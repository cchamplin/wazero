// Package conformance contains conformance tests for the Component Model implementation.
// Composite type tests ported from wasmtime's tests/all/component_model/func.rs (tuples)
package conformance

import (
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
		val := component.ValRecord(map[string]component.Val{})

		flat, err := abi.LowerFlat(nil, emptyRecord, val)
		require.NoError(t, err)
		require.Equal(t, 0, len(flat), "empty record should lower to 0 flat values")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, emptyRecord, iter)
		require.NoError(t, err)
		require.Equal(t, component.ValKindRecord, lifted.Kind())
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
		val := component.ValTuple([]component.Val{})

		flat, err := abi.LowerFlat(nil, emptyTuple, val)
		require.NoError(t, err)
		require.Equal(t, 0, len(flat), "empty tuple should lower to 0 flat values")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, emptyTuple, iter)
		require.NoError(t, err)
		require.Equal(t, component.ValKindTuple, lifted.Kind())
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
		val := component.ValRecord(map[string]component.Val{
			"x": component.ValS32(42),
		})

		flat, err := abi.LowerFlat(nil, recordType, val)
		require.NoError(t, err)
		require.Equal(t, 1, len(flat))

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, recordType, iter)
		require.NoError(t, err)
		require.Equal(t, component.ValKindRecord, lifted.Kind())

		fieldVal, ok := lifted.RecordField("x")
		require.True(t, ok)
		require.Equal(t, int32(42), fieldVal.S32())
	})

	t.Run("roundtrip_negative", func(t *testing.T) {
		val := component.ValRecord(map[string]component.Val{
			"x": component.ValS32(-100),
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
			val := component.ValRecord(map[string]component.Val{
				"x": component.ValS32(v),
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
		val := component.ValRecord(map[string]component.Val{
			"a": component.ValU8(255),
			"b": component.ValU32(0xDEADBEEF),
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
		val := component.ValRecord(map[string]component.Val{
			"a": component.ValU8(0x11),
			"b": component.ValU16(0x2233),
			"c": component.ValU8(0x44),
			"d": component.ValU32(0x55667788),
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
		val := component.ValTuple([]component.Val{
			component.ValS32(100),
			component.ValS32(200),
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
		val := component.ValTuple([]component.Val{
			component.ValS32(-1),
			component.ValS32(math.MinInt32),
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
		innerVal := component.ValRecord(map[string]component.Val{
			"x": component.ValS32(42),
		})
		outerVal := component.ValRecord(map[string]component.Val{
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
		require.Equal(t, component.ValKindRecord, liftedInner.Kind())

		xVal, ok := liftedInner.RecordField("x")
		require.True(t, ok)
		require.Equal(t, int32(42), xVal.S32())
	})

	t.Run("roundtrip_boundary_values", func(t *testing.T) {
		tests := []int32{0, math.MaxInt32, math.MinInt32}
		for _, v := range tests {
			innerVal := component.ValRecord(map[string]component.Val{
				"x": component.ValS32(v),
			})
			outerVal := component.ValRecord(map[string]component.Val{
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
		l3Val := component.ValRecord(map[string]component.Val{
			"value": component.ValU64(0xDEADBEEF12345678),
		})
		l2Val := component.ValRecord(map[string]component.Val{
			"level3": l3Val,
		})
		l1Val := component.ValRecord(map[string]component.Val{
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
		val := component.ValTuple([]component.Val{
			component.ValBool(true),
			component.ValU8(42),
			component.ValU16(1000),
			component.ValU32(100000),
			component.ValU64(9000000000000),
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
		val := component.ValTuple([]component.Val{
			component.ValBool(false),
			component.ValU8(math.MaxUint8),
			component.ValU16(math.MaxUint16),
			component.ValU32(math.MaxUint32),
			component.ValU64(math.MaxUint64),
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
		val := component.ValTuple([]component.Val{
			component.ValBool(false),
			component.ValU8(0),
			component.ValU16(0),
			component.ValU32(0),
			component.ValU64(0),
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
		innerVal := component.ValTuple([]component.Val{
			component.ValS32(10),
			component.ValS32(20),
		})
		outerVal := component.ValTuple([]component.Val{
			innerVal,
			component.ValU8(99),
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
		val := component.ValRecord(map[string]component.Val{
			"flag":  component.ValBool(true),
			"count": component.ValU32(12345),
			"value": component.ValS64(-9876543210),
			"ratio": component.ValF64(3.14159265358979),
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
		val := component.ValTuple([]component.Val{
			component.ValBool(true),
			component.ValS8(-42),
			component.ValU8(255),
			component.ValS16(-1000),
			component.ValU16(65000),
			component.ValS32(-100000),
			component.ValU32(4000000000),
			component.ValS64(-9000000000000),
			component.ValU64(18000000000000000000),
			component.ValF32(3.14),
			component.ValF64(2.718281828),
			component.ValChar('A'),
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
		val := component.ValRecord(map[string]component.Val{
			"x": component.ValS32(42),
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
		val := component.ValTuple([]component.Val{
			component.ValS32(42),
			// Missing second element
		})

		_, err := abi.LowerFlat(nil, tupleType, val)
		require.Error(t, err)
		require.Contains(t, err.Error(), "tuple has 1 elements, expected 2")
	})

	t.Run("too_many_elements", func(t *testing.T) {
		val := component.ValTuple([]component.Val{
			component.ValS32(1),
			component.ValS32(2),
			component.ValS32(3), // Extra element
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
		startVal := component.ValRecord(map[string]component.Val{
			"x": component.ValF32(1.0),
			"y": component.ValF32(2.0),
		})
		endVal := component.ValRecord(map[string]component.Val{
			"x": component.ValF32(3.0),
			"y": component.ValF32(4.0),
		})
		lineVal := component.ValRecord(map[string]component.Val{
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
		pair1 := component.ValTuple([]component.Val{
			component.ValF64(1.5),
			component.ValF64(2.5),
		})
		pair2 := component.ValTuple([]component.Val{
			component.ValF64(3.5),
			component.ValF64(4.5),
		})
		val := component.ValTuple([]component.Val{pair1, pair2})

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
