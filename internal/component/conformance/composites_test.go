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
		val := component.ValOption(nil)

		flat, err := abi.LowerFlat(nil, optionType, val)
		require.NoError(t, err)
		require.Equal(t, 2, len(flat), "option<s32> should lower to 2 flat values")
		require.Equal(t, uint64(0), flat[0], "None should have discriminant 0")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, optionType, iter)
		require.NoError(t, err)
		require.Equal(t, component.ValKindOption, lifted.Kind())

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
		payload := component.ValS32(42)
		val := component.ValOption(&payload)

		flat, err := abi.LowerFlat(nil, optionType, val)
		require.NoError(t, err)
		require.Equal(t, 2, len(flat), "option<s32> should lower to 2 flat values")
		require.Equal(t, uint64(1), flat[0], "Some should have discriminant 1")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, optionType, iter)
		require.NoError(t, err)
		require.Equal(t, component.ValKindOption, lifted.Kind())

		// Verify Some: payload should not be nil and should have correct value
		liftedPayload := lifted.Option()
		require.NotNil(t, liftedPayload, "Some option should have non-nil payload")
		require.Equal(t, int32(42), liftedPayload.S32(), "payload value should be preserved")
	})

	t.Run("roundtrip_some_negative", func(t *testing.T) {
		payload := component.ValS32(-100)
		val := component.ValOption(&payload)

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
			payload := component.ValS32(v)
			val := component.ValOption(&payload)

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

		payload := component.ValU8(255)
		val := component.ValOption(&payload)

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

		payload := component.ValU64(0xDEADBEEFCAFEBABE)
		val := component.ValOption(&payload)

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

		payload := component.ValF64(3.14159265358979)
		val := component.ValOption(&payload)

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
		okPayload := component.ValS32(100)
		val := component.ValResultOk(&okPayload)

		flat, err := abi.LowerFlat(nil, resultType, val)
		require.NoError(t, err)
		require.Equal(t, 2, len(flat), "result should lower to 2 flat values")
		require.Equal(t, uint64(0), flat[0], "Ok should have discriminant 0")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, resultType, iter)
		require.NoError(t, err)
		require.Equal(t, component.ValKindResult, lifted.Kind())

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
			okPayload := component.ValS32(v)
			val := component.ValResultOk(&okPayload)

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
		errPayload := component.ValS32(-1)
		val := component.ValResultError(&errPayload)

		flat, err := abi.LowerFlat(nil, resultType, val)
		require.NoError(t, err)
		require.Equal(t, 2, len(flat), "result should lower to 2 flat values")
		require.Equal(t, uint64(1), flat[0], "Error should have discriminant 1")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, resultType, iter)
		require.NoError(t, err)
		require.Equal(t, component.ValKindResult, lifted.Kind())

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
			errPayload := component.ValS32(v)
			val := component.ValResultError(&errPayload)

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
		okPayload := component.ValU64(0xFFFFFFFFFFFFFFFF)
		okVal := component.ValResultOk(&okPayload)

		flat, err := abi.LowerFlat(nil, resultType, okVal)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, resultType, iter)
		require.NoError(t, err)

		isOk, liftedOk, _ := lifted.Result()
		require.True(t, isOk)
		require.Equal(t, uint64(0xFFFFFFFFFFFFFFFF), liftedOk.U64())

		// Test Error
		errPayload := component.ValU8(42)
		errVal := component.ValResultError(&errPayload)

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
		okVal := component.ValResultOk(nil)

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
		errVal := component.ValResultError(nil)

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
		val := component.ValVariant("only-case", nil)

		flat, err := abi.LowerFlat(nil, variantType, val)
		require.NoError(t, err)
		require.Equal(t, 1, len(flat), "single case variant should lower to 1 flat value")
		require.Equal(t, uint64(0), flat[0], "only-case should have discriminant 0")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, variantType, iter)
		require.NoError(t, err)
		require.Equal(t, component.ValKindVariant, lifted.Kind())

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
		payload := component.ValS32(999)
		val := component.ValVariant("only", &payload)

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

		val := component.ValVariant("case5", nil)

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

		val := component.ValVariant("case9", nil)

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

		val := component.ValVariant("case299", nil)

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
		val := component.ValVariant("none", nil)

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
		payload := component.ValU8(200)
		val := component.ValVariant("some-u8", &payload)

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
		payload := component.ValS32(-12345)
		val := component.ValVariant("some-s32", &payload)

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
		payload := component.ValU64(0xCAFEBABEDEADBEEF)
		val := component.ValVariant("some-u64", &payload)

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
		val := component.ValVariant("unknown-case", nil)

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
		val := component.ValList([]component.Val{})

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
		require.Equal(t, component.ValKindList, lifted.Kind())

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

		val := component.ValList([]component.Val{})
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

		val := component.ValList([]component.Val{component.ValS32(42)})
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

		val := component.ValList([]component.Val{component.ValS32(42)})
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

		val := component.ValList([]component.Val{component.ValS32(-123)})
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
		elements := make([]component.Val, len(expected))
		for i, v := range expected {
			elements[i] = component.ValS32(v)
		}
		val := component.ValList(elements)

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

		elements := []component.Val{
			component.ValS32(10),
			component.ValS32(20),
			component.ValS32(30),
		}
		val := component.ValList(elements)

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
		inner1 := component.ValList([]component.Val{
			component.ValS32(1),
			component.ValS32(2),
		})
		inner2 := component.ValList([]component.Val{
			component.ValS32(3),
			component.ValS32(4),
			component.ValS32(5),
		})
		outer := component.ValList([]component.Val{inner1, inner2})

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
		inner1 := component.ValList([]component.Val{})
		inner2 := component.ValList([]component.Val{component.ValS32(1)})
		outer := component.ValList([]component.Val{inner1, inner2})

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
		elements := make([]component.Val, numElements)
		for i := 0; i < numElements; i++ {
			elements[i] = component.ValS32(int32(i * 2)) // 0, 2, 4, 6, ...
		}
		val := component.ValList(elements)

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

		val := component.ValList([]component.Val{
			component.ValU8(0),
			component.ValU8(127),
			component.ValU8(255),
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

		val := component.ValList([]component.Val{
			component.ValU64(0),
			component.ValU64(math.MaxUint64),
			component.ValU64(0xDEADBEEFCAFEBABE),
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

		val := component.ValList([]component.Val{
			component.ValF32(0.0),
			component.ValF32(3.14159),
			component.ValF32(-1.5),
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

		val := component.ValList([]component.Val{
			component.ValBool(true),
			component.ValBool(false),
			component.ValBool(true),
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
		points := []component.Val{
			component.ValRecord(map[string]component.Val{
				"x": component.ValS32(1),
				"y": component.ValS32(2),
			}),
			component.ValRecord(map[string]component.Val{
				"x": component.ValS32(3),
				"y": component.ValS32(4),
			}),
			component.ValRecord(map[string]component.Val{
				"x": component.ValS32(5),
				"y": component.ValS32(6),
			}),
		}
		val := component.ValList(points)

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
		val := component.ValList([]component.Val{component.ValS32(42)})

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

		val := component.ValList([]component.Val{component.ValS32(42)})
		err := abi.LowerHeap(lowerCtx, listType, val, 0)
		require.Error(t, err)
		require.Contains(t, err.Error(), "realloc function required")
	})
}
