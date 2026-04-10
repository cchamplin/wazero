// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package conformance: composite type (record/variant/list/tuple/
// option/result/flags/enum) conformance tests. Ported from
// canonical-abi run_tests.py::test_heap composite round-trips and
// wasmtime tests/all/component_model/func.rs tuple tests.
package conformance

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"

	"github.com/tetratelabs/wazero/experimental/wazerotest"
	"github.com/tetratelabs/wazero/internal/component/abi"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// compositeListCtx constructs a LowerContext + LiftContext pair
// sharing a wazerotest memory, a bump-pointer realloc, and the
// supplied *ComponentTypes — the shape used by every list/record/
// variant round-trip test below.
func compositeListCtx(t *testing.T, ct *types.ComponentTypes, startPtr uint32, memBytes int) (*abi.LowerContext, *abi.LiftContext) {
	t.Helper()
	mem := wazerotest.NewMemory(memBytes)
	alloc := startPtr
	lower := &abi.LowerContext{
		Types:  ct,
		Memory: mem,
		Opts:   &abi.Options{StringEncoding: types.StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			if align > 1 {
				alloc = (alloc + align - 1) &^ (align - 1)
			}
			result := alloc
			alloc += newSize
			return result, nil
		},
	}
	lift := &abi.LiftContext{
		Types:  ct,
		Memory: mem,
		Opts:   &abi.Options{StringEncoding: types.StringEncodingUTF8},
	}
	return lower, lift
}

// TestCompositeRecordEmpty asserts that an empty record has size 0,
// align 1, FlattenCount 0 and round-trips trivially.
//
// Spec: definitions.py:1087-1091 alignment_record, :1145-1151
// elem_size_record (empty record is a documented divergence from the
// spec's s > 0 assertion). Spec: definitions.py:1726-1730
// flatten_record (zero-sum over zero fields).
// Canonical test: run_tests.py::test_heap exercises record fixtures.
func TestCompositeRecordEmpty(t *testing.T) {
	b := newBuilder()
	emptyRecord := b.InternRecord([]types.RecordField{})
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}
	liftCtx := &abi.LiftContext{Types: ct}

	t.Run("type_properties", func(t *testing.T) {
		abiInfo := emptyRecord.ABI(ct)
		require.Equal(t, uint32(0), abiInfo.Size32)
		require.Equal(t, uint32(1), abiInfo.Align32)
		require.Equal(t, int32(0), abiInfo.FlattenCount)
	})

	t.Run("roundtrip", func(t *testing.T) {
		val := types.ValRecord(map[string]types.Val{})

		flat, err := abi.LowerFlat(lowerCtx, emptyRecord, val)
		require.NoError(t, err)
		require.Equal(t, 0, len(flat))

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, emptyRecord, iter)
		require.NoError(t, err)
		require.Equal(t, types.ValKindRecord, lifted.Kind())
		require.Equal(t, 0, len(lifted.Record()))
	})
}

// TestCompositeTupleEmpty asserts that an empty tuple has size 0,
// align 1, FlattenCount 0.
//
// Spec: definitions.py:126-127 TupleType is positional record;
// empty tuples follow the same empty-record ABI.
// Canonical test: run_tests.py::test_heap empty-tuple fixtures.
func TestCompositeTupleEmpty(t *testing.T) {
	b := newBuilder()
	emptyTuple := b.InternTuple([]types.ValType{})
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}
	liftCtx := &abi.LiftContext{Types: ct}

	t.Run("type_properties", func(t *testing.T) {
		abiInfo := emptyTuple.ABI(ct)
		require.Equal(t, uint32(0), abiInfo.Size32)
		require.Equal(t, uint32(1), abiInfo.Align32)
		require.Equal(t, int32(0), abiInfo.FlattenCount)
	})

	t.Run("roundtrip", func(t *testing.T) {
		val := types.ValTuple([]types.Val{})

		flat, err := abi.LowerFlat(lowerCtx, emptyTuple, val)
		require.NoError(t, err)
		require.Equal(t, 0, len(flat))

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, emptyTuple, iter)
		require.NoError(t, err)
		require.Equal(t, types.ValKindTuple, lifted.Kind())
		require.Equal(t, 0, len(lifted.Tuple()))
	})
}

// TestCompositeRecordSingleField asserts that a record with a single
// s32 field has size 4, align 4, FlattenCount 1 and round-trips
// positive, negative, and boundary values.
//
// Spec: definitions.py:1087-1091 alignment_record, :1145-1151
// elem_size_record, :1726-1730 flatten_record.
// Canonical test: run_tests.py::test_heap record round-trips.
func TestCompositeRecordSingleField(t *testing.T) {
	b := newBuilder()
	recordType := b.InternRecord([]types.RecordField{
		{Name: "x", Type: types.S32},
	})
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}
	liftCtx := &abi.LiftContext{Types: ct}

	t.Run("type_properties", func(t *testing.T) {
		abiInfo := recordType.ABI(ct)
		require.Equal(t, uint32(4), abiInfo.Size32)
		require.Equal(t, uint32(4), abiInfo.Align32)
		require.Equal(t, int32(1), abiInfo.FlattenCount)
	})

	t.Run("roundtrip_positive", func(t *testing.T) {
		val := types.ValRecord(map[string]types.Val{"x": types.ValS32(42)})

		flat, err := abi.LowerFlat(lowerCtx, recordType, val)
		require.NoError(t, err)
		require.Equal(t, 1, len(flat))

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, recordType, iter)
		require.NoError(t, err)
		fieldVal, ok := lifted.RecordField("x")
		require.True(t, ok)
		require.Equal(t, int32(42), fieldVal.S32())
	})

	t.Run("roundtrip_negative", func(t *testing.T) {
		val := types.ValRecord(map[string]types.Val{"x": types.ValS32(-100)})

		flat, err := abi.LowerFlat(lowerCtx, recordType, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, recordType, iter)
		require.NoError(t, err)
		fieldVal, _ := lifted.RecordField("x")
		require.Equal(t, int32(-100), fieldVal.S32())
	})

	t.Run("roundtrip_boundary_values", func(t *testing.T) {
		tests := []int32{0, math.MaxInt32, math.MinInt32}
		for _, v := range tests {
			val := types.ValRecord(map[string]types.Val{"x": types.ValS32(v)})

			flat, err := abi.LowerFlat(lowerCtx, recordType, val)
			require.NoError(t, err)

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(liftCtx, recordType, iter)
			require.NoError(t, err)
			fieldVal, _ := lifted.RecordField("x")
			require.Equal(t, v, fieldVal.S32())
		}
	})
}

// TestCompositeRecordWithPadding asserts that record{u8, u32} has
// size 8 (u8 + 3 pad + u32), align 4, FlattenCount 2.
//
// Spec: definitions.py:1087-1091 alignment_record aligns to the max
// field align; :1145-1151 elem_size_record inserts field-align
// padding. The FieldOffsets accessor from the pre-Session-0 API was
// removed; this test now asserts only size/align/flatten and the
// field round-trip (the offsets are indirectly verified by the
// round-trip succeeding).
//
// Dropped sub-test: "field_offsets" used the deleted FieldOffsets()
// accessor which is not part of the current TypeRecord public API.
func TestCompositeRecordWithPadding(t *testing.T) {
	b := newBuilder()
	recordType := b.InternRecord([]types.RecordField{
		{Name: "a", Type: types.U8},
		{Name: "b", Type: types.U32},
	})
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}
	liftCtx := &abi.LiftContext{Types: ct}

	t.Run("type_properties", func(t *testing.T) {
		abiInfo := recordType.ABI(ct)
		require.Equal(t, uint32(8), abiInfo.Size32)
		require.Equal(t, uint32(4), abiInfo.Align32)
		require.Equal(t, int32(2), abiInfo.FlattenCount)
	})

	t.Run("roundtrip", func(t *testing.T) {
		val := types.ValRecord(map[string]types.Val{
			"a": types.ValU8(255),
			"b": types.ValU32(0xDEADBEEF),
		})

		flat, err := abi.LowerFlat(lowerCtx, recordType, val)
		require.NoError(t, err)
		require.Equal(t, 2, len(flat))

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, recordType, iter)
		require.NoError(t, err)

		aVal, ok := lifted.RecordField("a")
		require.True(t, ok)
		require.Equal(t, uint8(255), aVal.U8())

		bVal, ok := lifted.RecordField("b")
		require.True(t, ok)
		require.Equal(t, uint32(0xDEADBEEF), bVal.U32())
	})
}

// TestCompositeRecordComplexPadding asserts that record{u8, u16, u8,
// u32} has size 12 (u8, pad, u16, u8, pad×3, u32), align 4,
// FlattenCount 4. Dropped: FieldOffsets sub-test (deleted API).
//
// Spec: definitions.py:1087-1091 alignment_record, :1145-1151
// elem_size_record walk-through example.
// Canonical test: run_tests.py::test_heap multi-field records.
func TestCompositeRecordComplexPadding(t *testing.T) {
	b := newBuilder()
	recordType := b.InternRecord([]types.RecordField{
		{Name: "a", Type: types.U8},
		{Name: "b", Type: types.U16},
		{Name: "c", Type: types.U8},
		{Name: "d", Type: types.U32},
	})
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}
	liftCtx := &abi.LiftContext{Types: ct}

	t.Run("type_properties", func(t *testing.T) {
		abiInfo := recordType.ABI(ct)
		require.Equal(t, uint32(12), abiInfo.Size32)
		require.Equal(t, uint32(4), abiInfo.Align32)
		require.Equal(t, int32(4), abiInfo.FlattenCount)
	})

	t.Run("roundtrip", func(t *testing.T) {
		val := types.ValRecord(map[string]types.Val{
			"a": types.ValU8(0x11),
			"b": types.ValU16(0x2233),
			"c": types.ValU8(0x44),
			"d": types.ValU32(0x55667788),
		})

		flat, err := abi.LowerFlat(lowerCtx, recordType, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, recordType, iter)
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

// TestCompositeTupleRoundtrip asserts tuple<s32, s32> has size 8,
// align 4, FlattenCount 2, and round-trips positive + negative
// values. Dropped: ElementOffsets sub-test (deleted API).
//
// Spec: definitions.py:126-127 TupleType, :1726-1730 flatten_record.
// Canonical test: run_tests.py::test_heap tuple fixtures.
func TestCompositeTupleRoundtrip(t *testing.T) {
	b := newBuilder()
	tupleType := b.InternTuple([]types.ValType{types.S32, types.S32})
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}
	liftCtx := &abi.LiftContext{Types: ct}

	t.Run("type_properties", func(t *testing.T) {
		abiInfo := tupleType.ABI(ct)
		require.Equal(t, uint32(8), abiInfo.Size32)
		require.Equal(t, uint32(4), abiInfo.Align32)
		require.Equal(t, int32(2), abiInfo.FlattenCount)
	})

	t.Run("roundtrip_positive", func(t *testing.T) {
		val := types.ValTuple([]types.Val{types.ValS32(100), types.ValS32(200)})

		flat, err := abi.LowerFlat(lowerCtx, tupleType, val)
		require.NoError(t, err)
		require.Equal(t, 2, len(flat))

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, tupleType, iter)
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

		flat, err := abi.LowerFlat(lowerCtx, tupleType, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, tupleType, iter)
		require.NoError(t, err)
		elems := lifted.Tuple()
		require.Equal(t, int32(-1), elems[0].S32())
		require.Equal(t, int32(math.MinInt32), elems[1].S32())
	})
}

// TestCompositeRecordNested asserts that nested records inherit
// size/align/flatten from their inner record unmodified.
//
// Spec: definitions.py:1087-1091 alignment_record is recursive over
// field types; nested records inherit the inner's align.
// Canonical test: run_tests.py::test_heap nested record fixtures.
func TestCompositeRecordNested(t *testing.T) {
	b := newBuilder()
	innerRecord := b.InternRecord([]types.RecordField{
		{Name: "x", Type: types.S32},
	})
	outerRecord := b.InternRecord([]types.RecordField{
		{Name: "inner", Type: innerRecord},
	})
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}
	liftCtx := &abi.LiftContext{Types: ct}

	t.Run("inner_type_properties", func(t *testing.T) {
		abiInfo := innerRecord.ABI(ct)
		require.Equal(t, uint32(4), abiInfo.Size32)
		require.Equal(t, uint32(4), abiInfo.Align32)
		require.Equal(t, int32(1), abiInfo.FlattenCount)
	})

	t.Run("outer_type_properties", func(t *testing.T) {
		abiInfo := outerRecord.ABI(ct)
		require.Equal(t, uint32(4), abiInfo.Size32)
		require.Equal(t, uint32(4), abiInfo.Align32)
		require.Equal(t, int32(1), abiInfo.FlattenCount)
	})

	t.Run("roundtrip", func(t *testing.T) {
		innerVal := types.ValRecord(map[string]types.Val{"x": types.ValS32(42)})
		outerVal := types.ValRecord(map[string]types.Val{"inner": innerVal})

		flat, err := abi.LowerFlat(lowerCtx, outerRecord, outerVal)
		require.NoError(t, err)
		require.Equal(t, 1, len(flat))

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, outerRecord, iter)
		require.NoError(t, err)

		liftedInner, _ := lifted.RecordField("inner")
		xVal, ok := liftedInner.RecordField("x")
		require.True(t, ok)
		require.Equal(t, int32(42), xVal.S32())
	})

	t.Run("roundtrip_boundary_values", func(t *testing.T) {
		tests := []int32{0, math.MaxInt32, math.MinInt32}
		for _, v := range tests {
			innerVal := types.ValRecord(map[string]types.Val{"x": types.ValS32(v)})
			outerVal := types.ValRecord(map[string]types.Val{"inner": innerVal})

			flat, err := abi.LowerFlat(lowerCtx, outerRecord, outerVal)
			require.NoError(t, err)

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(liftCtx, outerRecord, iter)
			require.NoError(t, err)

			liftedInner, _ := lifted.RecordField("inner")
			xVal, _ := liftedInner.RecordField("x")
			require.Equal(t, v, xVal.S32())
		}
	})
}

// TestCompositeRecordDeeplyNested asserts that a 3-level nested
// record inherits size/align from the innermost u64 field.
//
// Spec: definitions.py:1087-1091 alignment_record recurses through
// arbitrary nesting.
// Canonical test: run_tests.py::test_heap deeply-nested fixtures.
func TestCompositeRecordDeeplyNested(t *testing.T) {
	b := newBuilder()
	level3 := b.InternRecord([]types.RecordField{
		{Name: "value", Type: types.U64},
	})
	level2 := b.InternRecord([]types.RecordField{
		{Name: "level3", Type: level3},
	})
	level1 := b.InternRecord([]types.RecordField{
		{Name: "level2", Type: level2},
	})
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}
	liftCtx := &abi.LiftContext{Types: ct}

	t.Run("type_properties", func(t *testing.T) {
		abiInfo := level1.ABI(ct)
		require.Equal(t, uint32(8), abiInfo.Size32)
		require.Equal(t, uint32(8), abiInfo.Align32)
		require.Equal(t, int32(1), abiInfo.FlattenCount)
	})

	t.Run("roundtrip", func(t *testing.T) {
		l3Val := types.ValRecord(map[string]types.Val{
			"value": types.ValU64(0xDEADBEEF12345678),
		})
		l2Val := types.ValRecord(map[string]types.Val{"level3": l3Val})
		l1Val := types.ValRecord(map[string]types.Val{"level2": l2Val})

		flat, err := abi.LowerFlat(lowerCtx, level1, l1Val)
		require.NoError(t, err)
		require.Equal(t, 1, len(flat))

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, level1, iter)
		require.NoError(t, err)

		liftedL2, _ := lifted.RecordField("level2")
		liftedL3, _ := liftedL2.RecordField("level3")
		valueVal, _ := liftedL3.RecordField("value")
		require.Equal(t, uint64(0xDEADBEEF12345678), valueVal.U64())
	})
}

// TestCompositeTupleMixed asserts that tuple<bool, u8, u16, u32, u64>
// has size 16, align 8, FlattenCount 5 and round-trips typical /
// boundary / zero values. Dropped: ElementOffsets sub-test.
//
// Spec: definitions.py:126-127 TupleType ABI follows record rules.
// Canonical test: run_tests.py::test_heap mixed tuple fixtures.
func TestCompositeTupleMixed(t *testing.T) {
	b := newBuilder()
	mixedTuple := b.InternTuple([]types.ValType{
		types.Bool, types.U8, types.U16, types.U32, types.U64,
	})
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}
	liftCtx := &abi.LiftContext{Types: ct}

	t.Run("type_properties", func(t *testing.T) {
		abiInfo := mixedTuple.ABI(ct)
		require.Equal(t, uint32(16), abiInfo.Size32)
		require.Equal(t, uint32(8), abiInfo.Align32)
		require.Equal(t, int32(5), abiInfo.FlattenCount)
	})

	t.Run("roundtrip_typical", func(t *testing.T) {
		val := types.ValTuple([]types.Val{
			types.ValBool(true),
			types.ValU8(42),
			types.ValU16(1000),
			types.ValU32(100000),
			types.ValU64(9000000000000),
		})

		flat, err := abi.LowerFlat(lowerCtx, mixedTuple, val)
		require.NoError(t, err)
		require.Equal(t, 5, len(flat))

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, mixedTuple, iter)
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

		flat, err := abi.LowerFlat(lowerCtx, mixedTuple, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, mixedTuple, iter)
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

		flat, err := abi.LowerFlat(lowerCtx, mixedTuple, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, mixedTuple, iter)
		require.NoError(t, err)
		elems := lifted.Tuple()
		require.False(t, elems[0].Bool())
		require.Equal(t, uint8(0), elems[1].U8())
		require.Equal(t, uint16(0), elems[2].U16())
		require.Equal(t, uint32(0), elems[3].U32())
		require.Equal(t, uint64(0), elems[4].U64())
	})
}

// TestCompositeTupleNested asserts that a tuple containing another
// tuple has size 12 (inner 8 + u8 1 + pad 3), align 4, FlattenCount 3.
//
// Spec: definitions.py:126-127 TupleType, recursive ABI.
// Canonical test: run_tests.py::test_heap nested tuple fixtures.
func TestCompositeTupleNested(t *testing.T) {
	b := newBuilder()
	innerTuple := b.InternTuple([]types.ValType{types.S32, types.S32})
	outerTuple := b.InternTuple([]types.ValType{innerTuple, types.U8})
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}
	liftCtx := &abi.LiftContext{Types: ct}

	t.Run("type_properties", func(t *testing.T) {
		abiInfo := outerTuple.ABI(ct)
		require.Equal(t, uint32(12), abiInfo.Size32)
		require.Equal(t, uint32(4), abiInfo.Align32)
		require.Equal(t, int32(3), abiInfo.FlattenCount)
	})

	t.Run("roundtrip", func(t *testing.T) {
		innerVal := types.ValTuple([]types.Val{
			types.ValS32(10),
			types.ValS32(20),
		})
		outerVal := types.ValTuple([]types.Val{innerVal, types.ValU8(99)})

		flat, err := abi.LowerFlat(lowerCtx, outerTuple, outerVal)
		require.NoError(t, err)
		require.Equal(t, 3, len(flat))

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, outerTuple, iter)
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

// TestCompositeRecordMultipleFields asserts record{bool, u32, s64,
// f64} layout: bool at 0 (pad 3), u32 at 4, s64 at 8, f64 at 16 =
// size 24, align 8, FlattenCount 4.
//
// Spec: definitions.py:1087-1091 alignment_record. Spec:
// definitions.py:1145-1151 elem_size_record.
// Canonical test: run_tests.py::test_heap multi-field records.
func TestCompositeRecordMultipleFields(t *testing.T) {
	b := newBuilder()
	recordType := b.InternRecord([]types.RecordField{
		{Name: "flag", Type: types.Bool},
		{Name: "count", Type: types.U32},
		{Name: "value", Type: types.S64},
		{Name: "ratio", Type: types.F64},
	})
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}
	liftCtx := &abi.LiftContext{Types: ct}

	t.Run("type_properties", func(t *testing.T) {
		abiInfo := recordType.ABI(ct)
		require.Equal(t, uint32(24), abiInfo.Size32)
		require.Equal(t, uint32(8), abiInfo.Align32)
		require.Equal(t, int32(4), abiInfo.FlattenCount)
	})

	t.Run("roundtrip", func(t *testing.T) {
		val := types.ValRecord(map[string]types.Val{
			"flag":  types.ValBool(true),
			"count": types.ValU32(12345),
			"value": types.ValS64(-9876543210),
			"ratio": types.ValF64(3.14159265358979),
		})

		flat, err := abi.LowerFlat(lowerCtx, recordType, val)
		require.NoError(t, err)
		require.Equal(t, 4, len(flat))

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, recordType, iter)
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

// TestCompositeTupleAllPrimitives asserts tuple containing all 12
// primitive types round-trips every value class.
//
// Spec: definitions.py:1703-1720 flatten_type covers every primitive
// scalar.
// Canonical test: run_tests.py::test_heap primitives exercise.
func TestCompositeTupleAllPrimitives(t *testing.T) {
	b := newBuilder()
	allPrimitives := b.InternTuple([]types.ValType{
		types.Bool, types.S8, types.U8, types.S16, types.U16,
		types.S32, types.U32, types.S64, types.U64,
		types.F32, types.F64, types.Char,
	})
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}
	liftCtx := &abi.LiftContext{Types: ct}

	t.Run("flatten_count", func(t *testing.T) {
		require.Equal(t, int32(12), allPrimitives.ABI(ct).FlattenCount)
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

		flat, err := abi.LowerFlat(lowerCtx, allPrimitives, val)
		require.NoError(t, err)
		require.Equal(t, 12, len(flat))

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, allPrimitives, iter)
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

// TestCompositeRecordMissingField asserts LowerFlat errors with
// "missing record field" when a field name is absent from the Val.
//
// Spec: definitions.py:1355-1361 store_record iterates every field
// in order; the Python path would KeyError, wazero returns a wrapped
// error.
// Canonical test: no direct negative case — wazero surface check.
func TestCompositeRecordMissingField(t *testing.T) {
	b := newBuilder()
	recordType := b.InternRecord([]types.RecordField{
		{Name: "x", Type: types.S32},
		{Name: "y", Type: types.S32},
	})
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}

	t.Run("missing_field_error", func(t *testing.T) {
		val := types.ValRecord(map[string]types.Val{
			"x": types.ValS32(42),
		})

		_, err := abi.LowerFlat(lowerCtx, recordType, val)
		require.Error(t, err)
		require.Contains(t, err.Error(), "missing record field")
	})
}

// TestCompositeTupleWrongLength asserts LowerFlat errors when the
// tuple value has a different element count from the declared type.
//
// Spec: definitions.py:126-127 TupleType has a fixed element count;
// store_record-equivalent dispatch would fail on missing elements.
// Canonical test: no direct negative case — wazero surface check.
func TestCompositeTupleWrongLength(t *testing.T) {
	b := newBuilder()
	tupleType := b.InternTuple([]types.ValType{types.S32, types.S32})
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}

	t.Run("too_few_elements", func(t *testing.T) {
		val := types.ValTuple([]types.Val{types.ValS32(42)})

		_, err := abi.LowerFlat(lowerCtx, tupleType, val)
		require.Error(t, err)
		require.Contains(t, err.Error(), "tuple has 1 elements, expected 2")
	})

	t.Run("too_many_elements", func(t *testing.T) {
		val := types.ValTuple([]types.Val{
			types.ValS32(1), types.ValS32(2), types.ValS32(3),
		})

		_, err := abi.LowerFlat(lowerCtx, tupleType, val)
		require.Error(t, err)
		require.Contains(t, err.Error(), "tuple has 3 elements, expected 2")
	})
}

// TestCompositeTypeAlignmentTable asserts size and alignment for a
// broad table of record/tuple single- and dual-field shapes.
//
// Spec: definitions.py:1087-1091 alignment_record, :1145-1151
// elem_size_record. Same rules apply to TupleType via its sugar
// over record at :126-127.
// Canonical test: run_tests.py::test_heap exercises these shapes
// implicitly via random round-trips.
func TestCompositeTypeAlignmentTable(t *testing.T) {
	b := newBuilder()

	tests := []struct {
		name  string
		typ   types.ValType
		size  uint32
		align uint32
	}{
		{"record_u8", b.InternRecord([]types.RecordField{{Name: "a", Type: types.U8}}), 1, 1},
		{"record_u16", b.InternRecord([]types.RecordField{{Name: "a", Type: types.U16}}), 2, 2},
		{"record_u32", b.InternRecord([]types.RecordField{{Name: "a", Type: types.U32}}), 4, 4},
		{"record_u64", b.InternRecord([]types.RecordField{{Name: "a", Type: types.U64}}), 8, 8},
		{"record_f32", b.InternRecord([]types.RecordField{{Name: "a", Type: types.F32}}), 4, 4},
		{"record_f64", b.InternRecord([]types.RecordField{{Name: "a", Type: types.F64}}), 8, 8},

		{"tuple_u8", b.InternTuple([]types.ValType{types.U8}), 1, 1},
		{"tuple_u16", b.InternTuple([]types.ValType{types.U16}), 2, 2},
		{"tuple_u32", b.InternTuple([]types.ValType{types.U32}), 4, 4},
		{"tuple_u64", b.InternTuple([]types.ValType{types.U64}), 8, 8},

		{"record_u8_u16", b.InternRecord([]types.RecordField{
			{Name: "a", Type: types.U8}, {Name: "b", Type: types.U16},
		}), 4, 2},
		{"record_u8_u32", b.InternRecord([]types.RecordField{
			{Name: "a", Type: types.U8}, {Name: "b", Type: types.U32},
		}), 8, 4},
		{"record_u8_u64", b.InternRecord([]types.RecordField{
			{Name: "a", Type: types.U8}, {Name: "b", Type: types.U64},
		}), 16, 8},
		{"record_u16_u32", b.InternRecord([]types.RecordField{
			{Name: "a", Type: types.U16}, {Name: "b", Type: types.U32},
		}), 8, 4},
		{"record_u32_u8", b.InternRecord([]types.RecordField{
			{Name: "a", Type: types.U32}, {Name: "b", Type: types.U8},
		}), 8, 4},
	}
	ct := b.Finish()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			abiInfo := tc.typ.ABI(ct)
			require.Equal(t, tc.size, abiInfo.Size32)
			require.Equal(t, tc.align, abiInfo.Align32)
		})
	}
}

// TestCompositeRecordWithRecordField asserts that a record with a
// record-typed field (point-of-line) round-trips through the flat
// path. line.Size == 16 (2 × point 8); align 4; FlattenCount 4.
//
// Spec: definitions.py:1087-1091 alignment_record recursive.
// Canonical test: run_tests.py::test_heap record-of-record fixtures.
func TestCompositeRecordWithRecordField(t *testing.T) {
	b := newBuilder()
	point := b.InternRecord([]types.RecordField{
		{Name: "x", Type: types.F32},
		{Name: "y", Type: types.F32},
	})
	line := b.InternRecord([]types.RecordField{
		{Name: "start", Type: point},
		{Name: "end", Type: point},
	})
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}
	liftCtx := &abi.LiftContext{Types: ct}

	t.Run("type_properties", func(t *testing.T) {
		pointABI := point.ABI(ct)
		require.Equal(t, uint32(8), pointABI.Size32)
		require.Equal(t, uint32(4), pointABI.Align32)
		require.Equal(t, int32(2), pointABI.FlattenCount)

		lineABI := line.ABI(ct)
		require.Equal(t, uint32(16), lineABI.Size32)
		require.Equal(t, uint32(4), lineABI.Align32)
		require.Equal(t, int32(4), lineABI.FlattenCount)
	})

	t.Run("roundtrip", func(t *testing.T) {
		startVal := types.ValRecord(map[string]types.Val{
			"x": types.ValF32(1.0), "y": types.ValF32(2.0),
		})
		endVal := types.ValRecord(map[string]types.Val{
			"x": types.ValF32(3.0), "y": types.ValF32(4.0),
		})
		lineVal := types.ValRecord(map[string]types.Val{
			"start": startVal, "end": endVal,
		})

		flat, err := abi.LowerFlat(lowerCtx, line, lineVal)
		require.NoError(t, err)
		require.Equal(t, 4, len(flat))

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, line, iter)
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

// TestCompositeTupleWithTupleElement asserts that a tuple containing
// another tuple round-trips correctly.
//
// Spec: definitions.py:126-127 TupleType + recursive ABI.
// Canonical test: run_tests.py::test_heap tuple-of-tuple fixtures.
func TestCompositeTupleWithTupleElement(t *testing.T) {
	b := newBuilder()
	pair := b.InternTuple([]types.ValType{types.F64, types.F64})
	pairOfPairs := b.InternTuple([]types.ValType{pair, pair})
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}
	liftCtx := &abi.LiftContext{Types: ct}

	t.Run("type_properties", func(t *testing.T) {
		pairABI := pair.ABI(ct)
		require.Equal(t, uint32(16), pairABI.Size32)
		require.Equal(t, uint32(8), pairABI.Align32)
		require.Equal(t, int32(2), pairABI.FlattenCount)

		popABI := pairOfPairs.ABI(ct)
		require.Equal(t, uint32(32), popABI.Size32)
		require.Equal(t, uint32(8), popABI.Align32)
		require.Equal(t, int32(4), popABI.FlattenCount)
	})

	t.Run("roundtrip", func(t *testing.T) {
		pair1 := types.ValTuple([]types.Val{types.ValF64(1.5), types.ValF64(2.5)})
		pair2 := types.ValTuple([]types.Val{types.ValF64(3.5), types.ValF64(4.5)})
		val := types.ValTuple([]types.Val{pair1, pair2})

		flat, err := abi.LowerFlat(lowerCtx, pairOfPairs, val)
		require.NoError(t, err)
		require.Equal(t, 4, len(flat))

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, pairOfPairs, iter)
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
// =============================================================================

// TestOptionNone asserts option<s32> with None round-trips correctly.
//
// Spec: definitions.py:160-162 OptionType sugar for variant{none, some}.
// Canonical test: run_tests.py::test_heap option fixtures.
func TestOptionNone(t *testing.T) {
	b := newBuilder()
	optionType := b.InternOption(types.S32)
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}
	liftCtx := &abi.LiftContext{Types: ct}

	t.Run("type_properties", func(t *testing.T) {
		abiInfo := optionType.ABI(ct)
		require.Equal(t, uint32(8), abiInfo.Size32)
		require.Equal(t, uint32(4), abiInfo.Align32)
		require.Equal(t, int32(2), abiInfo.FlattenCount)
	})

	t.Run("roundtrip_none", func(t *testing.T) {
		val := types.ValOption(nil)

		flat, err := abi.LowerFlat(lowerCtx, optionType, val)
		require.NoError(t, err)
		require.Equal(t, 2, len(flat))
		require.Equal(t, uint64(0), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, optionType, iter)
		require.NoError(t, err)
		require.Equal(t, types.ValKindOption, lifted.Kind())
		require.Nil(t, lifted.Option())
	})
}

// TestOptionSome asserts option<s32> with Some(T) round-trips
// positive, negative, and boundary values.
//
// Spec: definitions.py:160-162 OptionType is variant{none, some(T)}.
// Canonical test: run_tests.py::test_heap option fixtures.
func TestOptionSome(t *testing.T) {
	b := newBuilder()
	optionType := b.InternOption(types.S32)
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}
	liftCtx := &abi.LiftContext{Types: ct}

	t.Run("roundtrip_some_positive", func(t *testing.T) {
		payload := types.ValS32(42)
		val := types.ValOption(&payload)

		flat, err := abi.LowerFlat(lowerCtx, optionType, val)
		require.NoError(t, err)
		require.Equal(t, 2, len(flat))
		require.Equal(t, uint64(1), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, optionType, iter)
		require.NoError(t, err)
		liftedPayload := lifted.Option()
		require.NotNil(t, liftedPayload)
		require.Equal(t, int32(42), liftedPayload.S32())
	})

	t.Run("roundtrip_some_negative", func(t *testing.T) {
		payload := types.ValS32(-100)
		val := types.ValOption(&payload)

		flat, err := abi.LowerFlat(lowerCtx, optionType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(1), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, optionType, iter)
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

			flat, err := abi.LowerFlat(lowerCtx, optionType, val)
			require.NoError(t, err)
			require.Equal(t, uint64(1), flat[0])

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(liftCtx, optionType, iter)
			require.NoError(t, err)
			liftedPayload := lifted.Option()
			require.NotNil(t, liftedPayload)
			require.Equal(t, v, liftedPayload.S32())
		}
	})
}

// TestOptionWithDifferentPayloadTypes asserts option<u8>, option<u64>,
// option<f64> all round-trip with their expected ABI.
//
// Spec: definitions.py:160-162 OptionType sugar; ABI follows the
// variant rule for the specific payload type.
// Canonical test: run_tests.py::test_heap option fixtures.
func TestOptionWithDifferentPayloadTypes(t *testing.T) {
	t.Run("option_u8", func(t *testing.T) {
		b := newBuilder()
		optType := b.InternOption(types.U8)
		ct := b.Finish()

		lowerCtx := &abi.LowerContext{Types: ct}
		liftCtx := &abi.LiftContext{Types: ct}

		abiInfo := optType.ABI(ct)
		require.Equal(t, uint32(2), abiInfo.Size32)
		require.Equal(t, uint32(1), abiInfo.Align32)
		require.Equal(t, int32(2), abiInfo.FlattenCount)

		payload := types.ValU8(255)
		val := types.ValOption(&payload)

		flat, err := abi.LowerFlat(lowerCtx, optType, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, optType, iter)
		require.NoError(t, err)
		liftedPayload := lifted.Option()
		require.NotNil(t, liftedPayload)
		require.Equal(t, uint8(255), liftedPayload.U8())
	})

	t.Run("option_u64", func(t *testing.T) {
		b := newBuilder()
		optType := b.InternOption(types.U64)
		ct := b.Finish()

		lowerCtx := &abi.LowerContext{Types: ct}
		liftCtx := &abi.LiftContext{Types: ct}

		abiInfo := optType.ABI(ct)
		require.Equal(t, uint32(16), abiInfo.Size32)
		require.Equal(t, uint32(8), abiInfo.Align32)
		require.Equal(t, int32(2), abiInfo.FlattenCount)

		payload := types.ValU64(0xDEADBEEFCAFEBABE)
		val := types.ValOption(&payload)

		flat, err := abi.LowerFlat(lowerCtx, optType, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, optType, iter)
		require.NoError(t, err)
		liftedPayload := lifted.Option()
		require.NotNil(t, liftedPayload)
		require.Equal(t, uint64(0xDEADBEEFCAFEBABE), liftedPayload.U64())
	})

	t.Run("option_f64", func(t *testing.T) {
		b := newBuilder()
		optType := b.InternOption(types.F64)
		ct := b.Finish()

		lowerCtx := &abi.LowerContext{Types: ct}
		liftCtx := &abi.LiftContext{Types: ct}

		payload := types.ValF64(3.14159265358979)
		val := types.ValOption(&payload)

		flat, err := abi.LowerFlat(lowerCtx, optType, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, optType, iter)
		require.NoError(t, err)
		liftedPayload := lifted.Option()
		require.NotNil(t, liftedPayload)
		require.Equal(t, math.Float64bits(3.14159265358979), math.Float64bits(liftedPayload.F64()))
	})
}

// TestResultOk asserts result<s32, s32> with Ok(v) round-trips with
// discriminant 0.
//
// Spec: definitions.py:155-159 ResultType sugar for variant{ok, err}.
// Canonical test: run_tests.py::test_heap result fixtures.
func TestResultOk(t *testing.T) {
	b := newBuilder()
	resultType := b.InternResult(types.S32, types.S32, true, true)
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}
	liftCtx := &abi.LiftContext{Types: ct}

	t.Run("type_properties", func(t *testing.T) {
		abiInfo := resultType.ABI(ct)
		require.Equal(t, uint32(8), abiInfo.Size32)
		require.Equal(t, uint32(4), abiInfo.Align32)
		require.Equal(t, int32(2), abiInfo.FlattenCount)
	})

	t.Run("roundtrip_ok", func(t *testing.T) {
		okPayload := types.ValS32(100)
		val := types.ValResultOk(&okPayload)

		flat, err := abi.LowerFlat(lowerCtx, resultType, val)
		require.NoError(t, err)
		require.Equal(t, 2, len(flat))
		require.Equal(t, uint64(0), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, resultType, iter)
		require.NoError(t, err)
		require.Equal(t, types.ValKindResult, lifted.Kind())

		isOk, okVal, errVal := lifted.Result()
		require.True(t, isOk)
		require.NotNil(t, okVal)
		require.Nil(t, errVal)
		require.Equal(t, int32(100), okVal.S32())
	})

	t.Run("roundtrip_ok_boundary_values", func(t *testing.T) {
		tests := []int32{0, math.MaxInt32, math.MinInt32, -1}
		for _, v := range tests {
			okPayload := types.ValS32(v)
			val := types.ValResultOk(&okPayload)

			flat, err := abi.LowerFlat(lowerCtx, resultType, val)
			require.NoError(t, err)
			require.Equal(t, uint64(0), flat[0])

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(liftCtx, resultType, iter)
			require.NoError(t, err)

			isOk, okVal, _ := lifted.Result()
			require.True(t, isOk)
			require.Equal(t, v, okVal.S32())
		}
	})
}

// TestResultError asserts result<s32, s32> with Error(v) round-trips
// with discriminant 1.
//
// Spec: definitions.py:155-159 ResultType. Canonical test:
// run_tests.py::test_heap result-Error fixtures.
func TestResultError(t *testing.T) {
	b := newBuilder()
	resultType := b.InternResult(types.S32, types.S32, true, true)
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}
	liftCtx := &abi.LiftContext{Types: ct}

	t.Run("roundtrip_error", func(t *testing.T) {
		errPayload := types.ValS32(-1)
		val := types.ValResultError(&errPayload)

		flat, err := abi.LowerFlat(lowerCtx, resultType, val)
		require.NoError(t, err)
		require.Equal(t, 2, len(flat))
		require.Equal(t, uint64(1), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, resultType, iter)
		require.NoError(t, err)

		isOk, okVal, errVal := lifted.Result()
		require.False(t, isOk)
		require.Nil(t, okVal)
		require.NotNil(t, errVal)
		require.Equal(t, int32(-1), errVal.S32())
	})

	t.Run("roundtrip_error_various_values", func(t *testing.T) {
		tests := []int32{0, 1, -100, math.MaxInt32, math.MinInt32}
		for _, v := range tests {
			errPayload := types.ValS32(v)
			val := types.ValResultError(&errPayload)

			flat, err := abi.LowerFlat(lowerCtx, resultType, val)
			require.NoError(t, err)
			require.Equal(t, uint64(1), flat[0])

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(liftCtx, resultType, iter)
			require.NoError(t, err)

			isOk, _, errVal := lifted.Result()
			require.False(t, isOk)
			require.Equal(t, v, errVal.S32())
		}
	})
}

// TestResultWithDifferentPayloadTypes asserts result variants with
// asymmetric ok/error payloads and unit payloads round-trip.
//
// Spec: definitions.py:155-159 ResultType with HasOk / HasErr flags
// for unit payloads.
// Canonical test: run_tests.py::test_heap unit-payload result
// fixtures.
func TestResultWithDifferentPayloadTypes(t *testing.T) {
	t.Run("result_u64_u8", func(t *testing.T) {
		b := newBuilder()
		resultType := b.InternResult(types.U64, types.U8, true, true)
		ct := b.Finish()

		lowerCtx := &abi.LowerContext{Types: ct}
		liftCtx := &abi.LiftContext{Types: ct}

		abiInfo := resultType.ABI(ct)
		require.Equal(t, uint32(16), abiInfo.Size32)
		require.Equal(t, uint32(8), abiInfo.Align32)
		require.Equal(t, int32(2), abiInfo.FlattenCount)

		okPayload := types.ValU64(0xFFFFFFFFFFFFFFFF)
		okVal := types.ValResultOk(&okPayload)

		flat, err := abi.LowerFlat(lowerCtx, resultType, okVal)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, resultType, iter)
		require.NoError(t, err)

		isOk, liftedOk, _ := lifted.Result()
		require.True(t, isOk)
		require.Equal(t, uint64(0xFFFFFFFFFFFFFFFF), liftedOk.U64())

		errPayload := types.ValU8(42)
		errVal := types.ValResultError(&errPayload)

		flat, err = abi.LowerFlat(lowerCtx, resultType, errVal)
		require.NoError(t, err)

		iter = abi.NewFlatIter(flat)
		lifted, err = abi.LiftFlat(liftCtx, resultType, iter)
		require.NoError(t, err)

		isOk, _, liftedErr := lifted.Result()
		require.False(t, isOk)
		require.Equal(t, uint8(42), liftedErr.U8())
	})

	t.Run("result_unit_ok", func(t *testing.T) {
		b := newBuilder()
		resultType := b.InternResult(types.ValType{}, types.S32, false, true)
		ct := b.Finish()

		lowerCtx := &abi.LowerContext{Types: ct}
		liftCtx := &abi.LiftContext{Types: ct}

		okVal := types.ValResultOk(nil)

		flat, err := abi.LowerFlat(lowerCtx, resultType, okVal)
		require.NoError(t, err)
		require.Equal(t, uint64(0), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, resultType, iter)
		require.NoError(t, err)

		isOk, liftedOk, _ := lifted.Result()
		require.True(t, isOk)
		require.Nil(t, liftedOk)
	})

	t.Run("result_unit_error", func(t *testing.T) {
		b := newBuilder()
		resultType := b.InternResult(types.S32, types.ValType{}, true, false)
		ct := b.Finish()

		lowerCtx := &abi.LowerContext{Types: ct}
		liftCtx := &abi.LiftContext{Types: ct}

		errVal := types.ValResultError(nil)

		flat, err := abi.LowerFlat(lowerCtx, resultType, errVal)
		require.NoError(t, err)
		require.Equal(t, uint64(1), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, resultType, iter)
		require.NoError(t, err)

		isOk, _, liftedErr := lifted.Result()
		require.False(t, isOk)
		require.Nil(t, liftedErr)
	})
}

// TestVariantSingleCase asserts a single-case no-payload variant has
// discriminant-only ABI: size 1, align 1, FlattenCount 1.
//
// Spec: definitions.py:1096-1103 discriminant_type, :1732-1741
// flatten_variant.
// Canonical test: run_tests.py::test_heap single-case variants.
func TestVariantSingleCase(t *testing.T) {
	b := newBuilder()
	variantType := b.InternVariant([]types.VariantCase{
		{Name: "only-case"},
	})
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}
	liftCtx := &abi.LiftContext{Types: ct}

	t.Run("type_properties", func(t *testing.T) {
		abiInfo := variantType.ABI(ct)
		require.Equal(t, uint32(1), abiInfo.Size32)
		require.Equal(t, uint32(1), abiInfo.Align32)
		require.Equal(t, int32(1), abiInfo.FlattenCount)

		disc := ct.Variants[variantType.Index].Disc
		require.Equal(t, uint8(1), disc.DiscSize)
	})

	t.Run("roundtrip", func(t *testing.T) {
		val := types.ValVariant("only-case", nil)

		flat, err := abi.LowerFlat(lowerCtx, variantType, val)
		require.NoError(t, err)
		require.Equal(t, 1, len(flat))
		require.Equal(t, uint64(0), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, variantType, iter)
		require.NoError(t, err)
		require.Equal(t, types.ValKindVariant, lifted.Kind())

		caseName, payload := lifted.Variant()
		require.Equal(t, "only-case", caseName)
		require.Nil(t, payload)
	})
}

// TestVariantSingleCaseWithPayload asserts a single-case variant
// with an s32 payload has size 8 (disc 1 + pad 3 + s32 4), align 4,
// FlattenCount 2.
//
// Spec: definitions.py:1156-1164 elem_size_variant includes
// discriminant + padded payload.
// Canonical test: run_tests.py::test_heap variant fixtures.
func TestVariantSingleCaseWithPayload(t *testing.T) {
	b := newBuilder()
	variantType := b.InternVariant([]types.VariantCase{
		{Name: "only", Payload: types.S32, HasPayload: true},
	})
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}
	liftCtx := &abi.LiftContext{Types: ct}

	t.Run("type_properties", func(t *testing.T) {
		abiInfo := variantType.ABI(ct)
		require.Equal(t, uint32(8), abiInfo.Size32)
		require.Equal(t, uint32(4), abiInfo.Align32)
		require.Equal(t, int32(2), abiInfo.FlattenCount)

		disc := ct.Variants[variantType.Index].Disc
		require.Equal(t, uint8(1), disc.DiscSize)
	})

	t.Run("roundtrip", func(t *testing.T) {
		payload := types.ValS32(999)
		val := types.ValVariant("only", &payload)

		flat, err := abi.LowerFlat(lowerCtx, variantType, val)
		require.NoError(t, err)
		require.Equal(t, 2, len(flat))

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, variantType, iter)
		require.NoError(t, err)

		caseName, liftedPayload := lifted.Variant()
		require.Equal(t, "only", caseName)
		require.NotNil(t, liftedPayload)
		require.Equal(t, int32(999), liftedPayload.S32())
	})
}

// TestVariantDiscriminantSizeU8 asserts variants with 10 and 256
// cases both use a u8 discriminant (1 byte) per the 0 < n <= 256 rule.
//
// Spec: definitions.py:1096-1103 discriminant_type: n <= 256 → u8.
// Canonical test: run_tests.py::test_heap exercises the discriminant
// boundary through max-case variants.
func TestVariantDiscriminantSizeU8(t *testing.T) {
	t.Run("10_cases", func(t *testing.T) {
		b := newBuilder()
		cases := make([]types.VariantCase, 10)
		for i := 0; i < 10; i++ {
			cases[i] = types.VariantCase{Name: fmt.Sprintf("case%d", i)}
		}
		variantType := b.InternVariant(cases)
		ct := b.Finish()

		abiInfo := variantType.ABI(ct)
		disc := ct.Variants[variantType.Index].Disc
		require.Equal(t, uint8(1), disc.DiscSize)
		require.Equal(t, uint32(1), abiInfo.Size32)
		require.Equal(t, uint32(1), abiInfo.Align32)
		require.Equal(t, int32(1), abiInfo.FlattenCount)
	})

	t.Run("256_cases", func(t *testing.T) {
		b := newBuilder()
		cases := make([]types.VariantCase, 256)
		for i := 0; i < 256; i++ {
			cases[i] = types.VariantCase{Name: fmt.Sprintf("case%d", i)}
		}
		variantType := b.InternVariant(cases)
		ct := b.Finish()

		disc := ct.Variants[variantType.Index].Disc
		require.Equal(t, uint8(1), disc.DiscSize)
	})

	t.Run("roundtrip_case_5", func(t *testing.T) {
		b := newBuilder()
		cases := make([]types.VariantCase, 10)
		for i := 0; i < 10; i++ {
			cases[i] = types.VariantCase{Name: fmt.Sprintf("case%d", i)}
		}
		variantType := b.InternVariant(cases)
		ct := b.Finish()

		lowerCtx := &abi.LowerContext{Types: ct}
		liftCtx := &abi.LiftContext{Types: ct}

		val := types.ValVariant("case5", nil)

		flat, err := abi.LowerFlat(lowerCtx, variantType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(5), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, variantType, iter)
		require.NoError(t, err)

		caseName, _ := lifted.Variant()
		require.Equal(t, "case5", caseName)
	})

	t.Run("roundtrip_last_case", func(t *testing.T) {
		b := newBuilder()
		cases := make([]types.VariantCase, 10)
		for i := 0; i < 10; i++ {
			cases[i] = types.VariantCase{Name: fmt.Sprintf("case%d", i)}
		}
		variantType := b.InternVariant(cases)
		ct := b.Finish()

		lowerCtx := &abi.LowerContext{Types: ct}
		liftCtx := &abi.LiftContext{Types: ct}

		val := types.ValVariant("case9", nil)

		flat, err := abi.LowerFlat(lowerCtx, variantType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(9), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, variantType, iter)
		require.NoError(t, err)

		caseName, _ := lifted.Variant()
		require.Equal(t, "case9", caseName)
	})
}

// TestVariantDiscriminantSizeU16 asserts that variants with 257+
// cases use a u16 discriminant per the n > 256 rule.
//
// Spec: definitions.py:1096-1103 discriminant_type: 256 < n <=
// 65536 → u16.
// Canonical test: run_tests.py::test_heap wide-variant fixtures.
func TestVariantDiscriminantSizeU16(t *testing.T) {
	t.Run("300_cases", func(t *testing.T) {
		b := newBuilder()
		cases := make([]types.VariantCase, 300)
		for i := 0; i < 300; i++ {
			cases[i] = types.VariantCase{Name: fmt.Sprintf("case%d", i)}
		}
		variantType := b.InternVariant(cases)
		ct := b.Finish()

		abiInfo := variantType.ABI(ct)
		disc := ct.Variants[variantType.Index].Disc
		require.Equal(t, uint8(2), disc.DiscSize)
		require.Equal(t, uint32(2), abiInfo.Size32)
		require.Equal(t, uint32(2), abiInfo.Align32)
		require.Equal(t, int32(1), abiInfo.FlattenCount)
	})

	t.Run("257_cases", func(t *testing.T) {
		b := newBuilder()
		cases := make([]types.VariantCase, 257)
		for i := 0; i < 257; i++ {
			cases[i] = types.VariantCase{Name: fmt.Sprintf("case%d", i)}
		}
		variantType := b.InternVariant(cases)
		ct := b.Finish()

		disc := ct.Variants[variantType.Index].Disc
		require.Equal(t, uint8(2), disc.DiscSize)
	})

	t.Run("roundtrip_case_299", func(t *testing.T) {
		b := newBuilder()
		cases := make([]types.VariantCase, 300)
		for i := 0; i < 300; i++ {
			cases[i] = types.VariantCase{Name: fmt.Sprintf("case%d", i)}
		}
		variantType := b.InternVariant(cases)
		ct := b.Finish()

		lowerCtx := &abi.LowerContext{Types: ct}
		liftCtx := &abi.LiftContext{Types: ct}

		val := types.ValVariant("case299", nil)

		flat, err := abi.LowerFlat(lowerCtx, variantType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(299), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, variantType, iter)
		require.NoError(t, err)

		caseName, _ := lifted.Variant()
		require.Equal(t, "case299", caseName)
	})
}

// TestVariantMultipleCasesWithPayloads asserts that a variant with
// mixed-payload cases {none, some-u8, some-s32, some-u64} has the
// joined-payload ABI: size 16 (disc + pad + u64), align 8,
// FlattenCount 2.
//
// Spec: definitions.py:1732-1741 flatten_variant uses the joined
// types (max per slot). Spec: CanonicalABI.md:2962-2989 join rules.
// Canonical test: run_tests.py::test_heap multi-payload variants.
func TestVariantMultipleCasesWithPayloads(t *testing.T) {
	b := newBuilder()
	variantType := b.InternVariant([]types.VariantCase{
		{Name: "none"},
		{Name: "some-u8", Payload: types.U8, HasPayload: true},
		{Name: "some-s32", Payload: types.S32, HasPayload: true},
		{Name: "some-u64", Payload: types.U64, HasPayload: true},
	})
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}
	liftCtx := &abi.LiftContext{Types: ct}

	t.Run("type_properties", func(t *testing.T) {
		abiInfo := variantType.ABI(ct)
		require.Equal(t, uint32(16), abiInfo.Size32)
		require.Equal(t, uint32(8), abiInfo.Align32)
		require.Equal(t, int32(2), abiInfo.FlattenCount)

		disc := ct.Variants[variantType.Index].Disc
		require.Equal(t, uint8(1), disc.DiscSize)
	})

	t.Run("roundtrip_none", func(t *testing.T) {
		val := types.ValVariant("none", nil)

		flat, err := abi.LowerFlat(lowerCtx, variantType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(0), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, variantType, iter)
		require.NoError(t, err)

		caseName, payload := lifted.Variant()
		require.Equal(t, "none", caseName)
		require.Nil(t, payload)
	})

	t.Run("roundtrip_some_u8", func(t *testing.T) {
		payload := types.ValU8(200)
		val := types.ValVariant("some-u8", &payload)

		flat, err := abi.LowerFlat(lowerCtx, variantType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(1), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, variantType, iter)
		require.NoError(t, err)

		caseName, liftedPayload := lifted.Variant()
		require.Equal(t, "some-u8", caseName)
		require.NotNil(t, liftedPayload)
		require.Equal(t, uint8(200), liftedPayload.U8())
	})

	t.Run("roundtrip_some_s32", func(t *testing.T) {
		payload := types.ValS32(-12345)
		val := types.ValVariant("some-s32", &payload)

		flat, err := abi.LowerFlat(lowerCtx, variantType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(2), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, variantType, iter)
		require.NoError(t, err)

		caseName, liftedPayload := lifted.Variant()
		require.Equal(t, "some-s32", caseName)
		require.NotNil(t, liftedPayload)
		require.Equal(t, int32(-12345), liftedPayload.S32())
	})

	t.Run("roundtrip_some_u64", func(t *testing.T) {
		payload := types.ValU64(0xCAFEBABEDEADBEEF)
		val := types.ValVariant("some-u64", &payload)

		flat, err := abi.LowerFlat(lowerCtx, variantType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(3), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, variantType, iter)
		require.NoError(t, err)

		caseName, liftedPayload := lifted.Variant()
		require.Equal(t, "some-u64", caseName)
		require.NotNil(t, liftedPayload)
		require.Equal(t, uint64(0xCAFEBABEDEADBEEF), liftedPayload.U64())
	})
}

// TestVariantPayloadOffset asserts the payload offset calculation
// (disc size aligned to max-payload align). Exposed via
// ct.Variants[idx].Disc.PayloadOffset.
//
// Spec: definitions.py:1105-1110 max_case_alignment, :1156-1164
// elem_size_variant.
// Canonical test: run_tests.py::test_heap exercises heap stores
// that implicitly validate the payload offset.
func TestVariantPayloadOffset(t *testing.T) {
	tests := []struct {
		name           string
		cases          []types.VariantCase
		expectedOffset uint32
	}{
		{
			name: "no_payload",
			cases: []types.VariantCase{
				{Name: "a"}, {Name: "b"},
			},
			// disc 1, no payload align bump, but alignTo(1, 1) = 1.
			expectedOffset: 1,
		},
		{
			name: "u8_payload",
			cases: []types.VariantCase{
				{Name: "a", Payload: types.U8, HasPayload: true},
			},
			expectedOffset: 1,
		},
		{
			name: "s32_payload",
			cases: []types.VariantCase{
				{Name: "a", Payload: types.S32, HasPayload: true},
			},
			expectedOffset: 4,
		},
		{
			name: "u64_payload",
			cases: []types.VariantCase{
				{Name: "a", Payload: types.U64, HasPayload: true},
			},
			expectedOffset: 8,
		},
		{
			name: "mixed_payloads_max_align",
			cases: []types.VariantCase{
				{Name: "a", Payload: types.U8, HasPayload: true},
				{Name: "b", Payload: types.U64, HasPayload: true},
			},
			expectedOffset: 8,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := newBuilder()
			variantType := b.InternVariant(tc.cases)
			ct := b.Finish()

			disc := ct.Variants[variantType.Index].Disc
			require.Equal(t, tc.expectedOffset, disc.PayloadOffset)
		})
	}
}

// TestVariantInvalidDiscriminant asserts that LiftFlat errors on a
// discriminant >= len(cases).
//
// Spec: definitions.py:1788-1790 lift_flat_variant would assert
// i < len(cases) in the Python model; wazero returns an error.
// Canonical test: no direct negative case — wazero surface check.
func TestVariantInvalidDiscriminant(t *testing.T) {
	b := newBuilder()
	variantType := b.InternVariant([]types.VariantCase{
		{Name: "a"}, {Name: "b"},
	})
	ct := b.Finish()

	liftCtx := &abi.LiftContext{Types: ct}

	t.Run("invalid_discriminant_error", func(t *testing.T) {
		flat := []uint64{2}
		iter := abi.NewFlatIter(flat)
		_, err := abi.LiftFlat(liftCtx, variantType, iter)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid variant discriminant")
	})
}

// TestVariantUnknownCaseName asserts LowerFlat errors when a Val
// carries a case name not in the variant's declared cases.
//
// Spec: definitions.py:1357-1361 store_variant would KeyError in
// Python on an unknown case; wazero returns an error.
// Canonical test: no direct negative case — wazero surface check.
func TestVariantUnknownCaseName(t *testing.T) {
	b := newBuilder()
	variantType := b.InternVariant([]types.VariantCase{
		{Name: "known"},
	})
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}

	t.Run("unknown_case_error", func(t *testing.T) {
		val := types.ValVariant("unknown-case", nil)

		_, err := abi.LowerFlat(lowerCtx, variantType, val)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown variant case")
	})
}

// =============================================================================
// List Tests
// =============================================================================

// TestListEmpty asserts that an empty list has ptr=0, len=0 and
// round-trips without allocating.
//
// Spec: definitions.py:1075 ListType flat layout (ptr, len), :1714
// flatten_list (dynamic list → ['i32','i32']).
// Canonical test: run_tests.py::test_heap list fixtures.
func TestListEmpty(t *testing.T) {
	b := newBuilder()
	listType := b.InternList(types.S32)
	ct := b.Finish()

	t.Run("type_properties", func(t *testing.T) {
		abiInfo := listType.ABI(ct)
		require.Equal(t, uint32(8), abiInfo.Size32)
		require.Equal(t, uint32(4), abiInfo.Align32)
		require.Equal(t, int32(2), abiInfo.FlattenCount)

		elemABI := ct.Lists[listType.Index].Element.ABI(ct)
		require.Equal(t, uint32(4), elemABI.Size32)
		require.Equal(t, uint32(4), elemABI.Align32)
	})

	t.Run("lower_flat_empty", func(t *testing.T) {
		val := types.ValList([]types.Val{})

		reallocCalled := false
		mem := wazerotest.NewMemory(wazerotest.PageSize)
		ctx := &abi.LowerContext{
			Types:  ct,
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: types.StringEncodingUTF8},
			Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
				reallocCalled = true
				return 0, nil
			},
		}

		flat, err := abi.LowerFlat(ctx, listType, val)
		require.NoError(t, err)
		require.Equal(t, 2, len(flat))
		// Per canonical ABI spec: realloc is always called, even for empty
		// lists. The realloc returns 0 here, which is valid.
		require.Equal(t, uint64(0), flat[0])
		require.Equal(t, uint64(0), flat[1])
		require.True(t, reallocCalled)
	})

	t.Run("lift_flat_empty", func(t *testing.T) {
		flat := []uint64{0, 0}
		liftCtx := &abi.LiftContext{
			Types:  ct,
			Memory: wazerotest.NewMemory(wazerotest.PageSize),
			Opts:   &abi.Options{StringEncoding: types.StringEncodingUTF8},
		}
		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, listType, iter)
		require.NoError(t, err)
		require.Equal(t, types.ValKindList, lifted.Kind())
		require.Equal(t, 0, len(lifted.List()))
	})

	t.Run("heap_roundtrip_empty", func(t *testing.T) {
		lowerCtx, liftCtx := compositeListCtx(t, ct, 256, wazerotest.PageSize)

		val := types.ValList([]types.Val{})
		err := abi.LowerHeap(lowerCtx, listType, val, 0)
		require.NoError(t, err)

		// Per spec (definitions.py:1594-1601 store_list_into_range):
		// realloc is called even for empty lists. The pointer is whatever
		// realloc returns (aligned, within memory bounds); length is 0.
		ptr := binary.LittleEndian.Uint32(lowerCtx.Memory.(*wazerotest.Memory).Bytes[0:4])
		length := binary.LittleEndian.Uint32(lowerCtx.Memory.(*wazerotest.Memory).Bytes[4:8])
		require.NotEqual(t, uint32(0), ptr, "realloc must be called even for empty lists")
		require.Equal(t, uint32(0), length)

		lifted, err := abi.LiftHeap(liftCtx, listType, 0)
		require.NoError(t, err)
		require.Equal(t, 0, len(lifted.List()))
	})
}

// TestListSingleElement asserts list<s32> with one element lowers
// (ptr, 1) and round-trips through flat and heap paths.
//
// Spec: definitions.py:1427-1435 store_list, :1285-1301 load_list.
// Canonical test: run_tests.py::test_heap list fixtures.
func TestListSingleElement(t *testing.T) {
	b := newBuilder()
	listType := b.InternList(types.S32)
	ct := b.Finish()

	t.Run("flat_roundtrip", func(t *testing.T) {
		lowerCtx, liftCtx := compositeListCtx(t, ct, 256, wazerotest.PageSize)

		val := types.ValList([]types.Val{types.ValS32(42)})
		flat, err := abi.LowerFlat(lowerCtx, listType, val)
		require.NoError(t, err)
		require.Equal(t, 2, len(flat))
		require.Equal(t, uint64(256), flat[0])
		require.Equal(t, uint64(1), flat[1])

		memBytes := lowerCtx.Memory.(*wazerotest.Memory).Bytes
		elemVal := int32(binary.LittleEndian.Uint32(memBytes[256:260]))
		require.Equal(t, int32(42), elemVal)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, listType, iter)
		require.NoError(t, err)
		elems := lifted.List()
		require.Equal(t, 1, len(elems))
		require.Equal(t, int32(42), elems[0].S32())
	})

	t.Run("heap_roundtrip", func(t *testing.T) {
		lowerCtx, liftCtx := compositeListCtx(t, ct, 256, wazerotest.PageSize)

		val := types.ValList([]types.Val{types.ValS32(42)})
		err := abi.LowerHeap(lowerCtx, listType, val, 0)
		require.NoError(t, err)

		memBytes := lowerCtx.Memory.(*wazerotest.Memory).Bytes
		ptr := binary.LittleEndian.Uint32(memBytes[0:4])
		length := binary.LittleEndian.Uint32(memBytes[4:8])
		require.Equal(t, uint32(256), ptr)
		require.Equal(t, uint32(1), length)

		lifted, err := abi.LiftHeap(liftCtx, listType, 0)
		require.NoError(t, err)
		elems := lifted.List()
		require.Equal(t, 1, len(elems))
		require.Equal(t, int32(42), elems[0].S32())
	})

	t.Run("negative_value", func(t *testing.T) {
		lowerCtx, liftCtx := compositeListCtx(t, ct, 256, wazerotest.PageSize)

		val := types.ValList([]types.Val{types.ValS32(-123)})
		flat, err := abi.LowerFlat(lowerCtx, listType, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, listType, iter)
		require.NoError(t, err)
		elems := lifted.List()
		require.Equal(t, int32(-123), elems[0].S32())
	})
}

// TestListMultipleElements asserts list<s32> with [1,2,3,4,5]
// round-trips and the iteration invariant holds.
//
// Spec: definitions.py:1285-1301 load_list / 1427-1435 store_list.
// Canonical test: run_tests.py::test_heap multi-element lists.
func TestListMultipleElements(t *testing.T) {
	b := newBuilder()
	listType := b.InternList(types.S32)
	ct := b.Finish()

	t.Run("flat_roundtrip", func(t *testing.T) {
		lowerCtx, liftCtx := compositeListCtx(t, ct, 256, wazerotest.PageSize)

		expected := []int32{1, 2, 3, 4, 5}
		elements := make([]types.Val, len(expected))
		for i, v := range expected {
			elements[i] = types.ValS32(v)
		}
		val := types.ValList(elements)

		flat, err := abi.LowerFlat(lowerCtx, listType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(256), flat[0])
		require.Equal(t, uint64(5), flat[1])

		memBytes := lowerCtx.Memory.(*wazerotest.Memory).Bytes
		for i, exp := range expected {
			offset := 256 + uint32(i*4)
			actual := int32(binary.LittleEndian.Uint32(memBytes[offset : offset+4]))
			require.Equal(t, exp, actual)
		}

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, listType, iter)
		require.NoError(t, err)
		elems := lifted.List()
		require.Equal(t, len(expected), len(elems))
		for i, exp := range expected {
			require.Equal(t, exp, elems[i].S32())
		}
	})

	t.Run("iteration_over_lifted", func(t *testing.T) {
		lowerCtx, liftCtx := compositeListCtx(t, ct, 256, wazerotest.PageSize)

		elements := []types.Val{
			types.ValS32(10), types.ValS32(20), types.ValS32(30),
		}
		val := types.ValList(elements)

		flat, err := abi.LowerFlat(lowerCtx, listType, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, listType, iter)
		require.NoError(t, err)

		sum := int32(0)
		for _, elem := range lifted.List() {
			sum += elem.S32()
		}
		require.Equal(t, int32(60), sum)
	})
}

// TestListNested asserts list<list<s32>> round-trips (including
// empty inner lists) through the heap path.
//
// Spec: definitions.py:1285-1301 load_list recursive, :1427-1435
// store_list recursive.
// Canonical test: run_tests.py::test_heap nested list fixtures.
func TestListNested(t *testing.T) {
	b := newBuilder()
	innerListType := b.InternList(types.S32)
	outerListType := b.InternList(innerListType)
	ct := b.Finish()

	t.Run("type_properties", func(t *testing.T) {
		innerABI := innerListType.ABI(ct)
		require.Equal(t, uint32(8), innerABI.Size32)
		require.Equal(t, uint32(4), innerABI.Align32)

		outerABI := outerListType.ABI(ct)
		require.Equal(t, uint32(8), outerABI.Size32)
		require.Equal(t, uint32(4), outerABI.Align32)

		elemABI := ct.Lists[outerListType.Index].Element.ABI(ct)
		require.Equal(t, uint32(8), elemABI.Size32)
		require.Equal(t, uint32(4), elemABI.Align32)
	})

	t.Run("heap_roundtrip_nested", func(t *testing.T) {
		lowerCtx, liftCtx := compositeListCtx(t, ct, 512, wazerotest.PageSize)

		inner1 := types.ValList([]types.Val{types.ValS32(1), types.ValS32(2)})
		inner2 := types.ValList([]types.Val{types.ValS32(3), types.ValS32(4), types.ValS32(5)})
		outer := types.ValList([]types.Val{inner1, inner2})

		err := abi.LowerHeap(lowerCtx, outerListType, outer, 0)
		require.NoError(t, err)

		lifted, err := abi.LiftHeap(liftCtx, outerListType, 0)
		require.NoError(t, err)

		outerElems := lifted.List()
		require.Equal(t, 2, len(outerElems))

		inner1Elems := outerElems[0].List()
		require.Equal(t, 2, len(inner1Elems))
		require.Equal(t, int32(1), inner1Elems[0].S32())
		require.Equal(t, int32(2), inner1Elems[1].S32())

		inner2Elems := outerElems[1].List()
		require.Equal(t, 3, len(inner2Elems))
		require.Equal(t, int32(3), inner2Elems[0].S32())
		require.Equal(t, int32(4), inner2Elems[1].S32())
		require.Equal(t, int32(5), inner2Elems[2].S32())
	})

	t.Run("empty_inner_lists", func(t *testing.T) {
		lowerCtx, liftCtx := compositeListCtx(t, ct, 512, wazerotest.PageSize)

		inner1 := types.ValList([]types.Val{})
		inner2 := types.ValList([]types.Val{types.ValS32(1)})
		outer := types.ValList([]types.Val{inner1, inner2})

		err := abi.LowerHeap(lowerCtx, outerListType, outer, 0)
		require.NoError(t, err)

		lifted, err := abi.LiftHeap(liftCtx, outerListType, 0)
		require.NoError(t, err)

		outerElems := lifted.List()
		require.Equal(t, 2, len(outerElems))
		require.Equal(t, 0, len(outerElems[0].List()))
		require.Equal(t, 1, len(outerElems[1].List()))
		require.Equal(t, int32(1), outerElems[1].List()[0].S32())
	})
}

// TestListMaxLength asserts a list with 1000 elements round-trips
// without size-class issues.
//
// Spec: definitions.py:1427-1435 store_list has no explicit upper
// bound beyond memory availability.
// Canonical test: run_tests.py::test_heap large-list fixtures.
func TestListMaxLength(t *testing.T) {
	b := newBuilder()
	listType := b.InternList(types.S32)
	ct := b.Finish()

	t.Run("large_list_roundtrip", func(t *testing.T) {
		lowerCtx, liftCtx := compositeListCtx(t, ct, 1024, wazerotest.PageSize)

		numElements := 1000
		elements := make([]types.Val, numElements)
		for i := 0; i < numElements; i++ {
			elements[i] = types.ValS32(int32(i * 2))
		}
		val := types.ValList(elements)

		flat, err := abi.LowerFlat(lowerCtx, listType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(numElements), flat[1])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, listType, iter)
		require.NoError(t, err)

		elems := lifted.List()
		require.Equal(t, numElements, len(elems))

		for i := 0; i < numElements; i++ {
			require.Equal(t, int32(i*2), elems[i].S32())
		}
	})
}

// TestListDifferentElementTypes asserts that lists with u8, u64, f32,
// and bool element types all round-trip correctly.
//
// Spec: definitions.py:1075,1133 ListType layout; element ABI varies.
// Canonical test: run_tests.py::test_heap element-variety fixtures.
func TestListDifferentElementTypes(t *testing.T) {
	t.Run("list_u8", func(t *testing.T) {
		b := newBuilder()
		listType := b.InternList(types.U8)
		ct := b.Finish()

		elemABI := ct.Lists[listType.Index].Element.ABI(ct)
		require.Equal(t, uint32(1), elemABI.Size32)
		require.Equal(t, uint32(1), elemABI.Align32)

		lowerCtx, liftCtx := compositeListCtx(t, ct, 256, wazerotest.PageSize)

		val := types.ValList([]types.Val{
			types.ValU8(0), types.ValU8(127), types.ValU8(255),
		})

		flat, err := abi.LowerFlat(lowerCtx, listType, val)
		require.NoError(t, err)

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
		b := newBuilder()
		listType := b.InternList(types.U64)
		ct := b.Finish()

		elemABI := ct.Lists[listType.Index].Element.ABI(ct)
		require.Equal(t, uint32(8), elemABI.Size32)
		require.Equal(t, uint32(8), elemABI.Align32)

		lowerCtx, liftCtx := compositeListCtx(t, ct, 256, wazerotest.PageSize)

		val := types.ValList([]types.Val{
			types.ValU64(0),
			types.ValU64(math.MaxUint64),
			types.ValU64(0xDEADBEEFCAFEBABE),
		})

		flat, err := abi.LowerFlat(lowerCtx, listType, val)
		require.NoError(t, err)

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
		b := newBuilder()
		listType := b.InternList(types.F32)
		ct := b.Finish()

		lowerCtx, liftCtx := compositeListCtx(t, ct, 256, wazerotest.PageSize)

		val := types.ValList([]types.Val{
			types.ValF32(0.0), types.ValF32(3.14159), types.ValF32(-1.5),
		})

		flat, err := abi.LowerFlat(lowerCtx, listType, val)
		require.NoError(t, err)

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
		b := newBuilder()
		listType := b.InternList(types.Bool)
		ct := b.Finish()

		lowerCtx, liftCtx := compositeListCtx(t, ct, 256, wazerotest.PageSize)

		val := types.ValList([]types.Val{
			types.ValBool(true), types.ValBool(false), types.ValBool(true),
		})

		flat, err := abi.LowerFlat(lowerCtx, listType, val)
		require.NoError(t, err)

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

// TestListOfRecords asserts list<record{x:s32,y:s32}> round-trips
// through the heap path.
//
// Spec: definitions.py:1285-1301 load_list with record elem_type,
// :1427-1435 store_list.
// Canonical test: run_tests.py::test_heap list-of-record fixtures.
func TestListOfRecords(t *testing.T) {
	b := newBuilder()
	pointRecord := b.InternRecord([]types.RecordField{
		{Name: "x", Type: types.S32},
		{Name: "y", Type: types.S32},
	})
	listType := b.InternList(pointRecord)
	ct := b.Finish()

	t.Run("type_properties", func(t *testing.T) {
		elemABI := ct.Lists[listType.Index].Element.ABI(ct)
		require.Equal(t, uint32(8), elemABI.Size32)
		require.Equal(t, uint32(4), elemABI.Align32)
	})

	t.Run("roundtrip", func(t *testing.T) {
		lowerCtx, liftCtx := compositeListCtx(t, ct, 256, wazerotest.PageSize)

		points := []types.Val{
			types.ValRecord(map[string]types.Val{
				"x": types.ValS32(1), "y": types.ValS32(2),
			}),
			types.ValRecord(map[string]types.Val{
				"x": types.ValS32(3), "y": types.ValS32(4),
			}),
			types.ValRecord(map[string]types.Val{
				"x": types.ValS32(5), "y": types.ValS32(6),
			}),
		}
		val := types.ValList(points)

		err := abi.LowerHeap(lowerCtx, listType, val, 0)
		require.NoError(t, err)

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

// TestListBoundsChecking asserts LiftHeap errors when the (ptr, len)
// header points beyond memory.
//
// Spec: definitions.py:1293-1294 trap_if(ptr + length * elem_size >
// len(memory)).
// Canonical test: run_tests.py exercises the trap via out-of-bounds
// fixtures indirectly.
func TestListBoundsChecking(t *testing.T) {
	b := newBuilder()
	listType := b.InternList(types.S32)
	ct := b.Finish()

	t.Run("lift_out_of_bounds", func(t *testing.T) {
		mem := wazerotest.NewMemory(wazerotest.PageSize)
		// (ptr beyondPage, len 10) — 10 × 4 > any memory slack.
		binary.LittleEndian.PutUint32(mem.Bytes[0:], wazerotest.PageSize+1000)
		binary.LittleEndian.PutUint32(mem.Bytes[4:], 10)

		liftCtx := &abi.LiftContext{Types: ct, Memory: mem, Opts: &abi.Options{}}
		_, err := abi.LiftHeap(liftCtx, listType, 0)
		require.Error(t, err)
	})

	t.Run("lift_ptr_plus_size_overflow", func(t *testing.T) {
		mem := wazerotest.NewMemory(wazerotest.PageSize)
		// (ptr page-4, len 20) — ptr + 20×4 > memory.
		binary.LittleEndian.PutUint32(mem.Bytes[0:], wazerotest.PageSize-4)
		binary.LittleEndian.PutUint32(mem.Bytes[4:], 20)

		liftCtx := &abi.LiftContext{Types: ct, Memory: mem, Opts: &abi.Options{}}
		_, err := abi.LiftHeap(liftCtx, listType, 0)
		require.Error(t, err)
	})

	t.Run("lift_flat_no_memory_for_nonempty", func(t *testing.T) {
		// Non-empty list requires a memory context.
		flat := []uint64{256, 5}
		liftCtx := &abi.LiftContext{Types: ct}
		iter := abi.NewFlatIter(flat)
		_, err := abi.LiftFlat(liftCtx, listType, iter)
		require.Error(t, err)
	})
}

// TestListNoReallocError asserts LowerFlat / LowerHeap error when
// realloc is absent for a non-empty list.
//
// Spec: definitions.py:1427-1435 store_list calls realloc for every
// non-zero-length list.
// Canonical test: no direct negative case — wazero surface check.
func TestListNoReallocError(t *testing.T) {
	b := newBuilder()
	listType := b.InternList(types.S32)
	ct := b.Finish()

	t.Run("lower_flat_no_realloc", func(t *testing.T) {
		val := types.ValList([]types.Val{types.ValS32(42)})
		lowerCtx := &abi.LowerContext{Types: ct}
		_, err := abi.LowerFlat(lowerCtx, listType, val)
		require.Error(t, err)
		require.Contains(t, err.Error(), "realloc")
	})

	t.Run("lower_heap_no_realloc", func(t *testing.T) {
		mem := wazerotest.NewMemory(wazerotest.PageSize)
		lowerCtx := &abi.LowerContext{
			Types:   ct,
			Memory:  mem,
			Opts:    &abi.Options{},
			Realloc: nil,
		}

		val := types.ValList([]types.Val{types.ValS32(42)})
		err := abi.LowerHeap(lowerCtx, listType, val, 0)
		require.Error(t, err)
		require.Contains(t, err.Error(), "realloc")
	})
}

// =============================================================================
// Flags Tests
// =============================================================================

// TestFlagsEmpty asserts empty flags have size 0, align 1,
// FlattenCount 0 and round-trip as no-ops.
//
// Spec: definitions.py:1112-1117 alignment_flags, :1166-1171
// elem_size_flags.
// Canonical test: run_tests.py covers flags via test_heap.
func TestFlagsEmpty(t *testing.T) {
	b := newBuilder()
	emptyFlags := b.InternFlags([]string{})
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct, Memory: wazerotest.NewMemory(wazerotest.PageSize)}
	liftCtx := &abi.LiftContext{Types: ct, Memory: lowerCtx.Memory}

	t.Run("type_properties", func(t *testing.T) {
		abiInfo := emptyFlags.ABI(ct)
		require.Equal(t, uint32(0), abiInfo.Size32)
		require.Equal(t, uint32(1), abiInfo.Align32)
		require.Equal(t, int32(0), abiInfo.FlattenCount)
	})

	t.Run("roundtrip_flat", func(t *testing.T) {
		val := types.ValFlags(map[string]bool{})

		flat, err := abi.LowerFlat(lowerCtx, emptyFlags, val)
		require.NoError(t, err)
		require.Equal(t, 0, len(flat))

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, emptyFlags, iter)
		require.NoError(t, err)
		require.Equal(t, types.ValKindFlags, lifted.Kind())
		require.Equal(t, 0, len(lifted.Flags()))
	})

	t.Run("roundtrip_heap", func(t *testing.T) {
		val := types.ValFlags(map[string]bool{})

		err := abi.LowerHeap(lowerCtx, emptyFlags, val, 0)
		require.NoError(t, err)

		lifted, err := abi.LiftHeap(liftCtx, emptyFlags, 0)
		require.NoError(t, err)
		require.Equal(t, types.ValKindFlags, lifted.Kind())
		require.Equal(t, 0, len(lifted.Flags()))
	})
}

// TestFlagsSingleFlag asserts a single-flag type has size 1, align 1,
// FlattenCount 1 and round-trips via flat and heap paths.
//
// Spec: definitions.py:1112-1117 alignment_flags (n <= 8 → 1).
// Canonical test: run_tests.py covers flags via test_heap.
func TestFlagsSingleFlag(t *testing.T) {
	b := newBuilder()
	singleFlag := b.InternFlags([]string{"read"})
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct, Memory: wazerotest.NewMemory(wazerotest.PageSize)}
	liftCtx := &abi.LiftContext{Types: ct, Memory: lowerCtx.Memory}

	t.Run("type_properties", func(t *testing.T) {
		abiInfo := singleFlag.ABI(ct)
		require.Equal(t, uint32(1), abiInfo.Size32)
		require.Equal(t, uint32(1), abiInfo.Align32)
		require.Equal(t, int32(1), abiInfo.FlattenCount)
	})

	t.Run("flag_true", func(t *testing.T) {
		val := types.ValFlags(map[string]bool{"read": true})

		flat, err := abi.LowerFlat(lowerCtx, singleFlag, val)
		require.NoError(t, err)
		require.Equal(t, 1, len(flat))
		require.Equal(t, uint64(1), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, singleFlag, iter)
		require.NoError(t, err)
		require.True(t, lifted.Flags()["read"])
	})

	t.Run("flag_false", func(t *testing.T) {
		val := types.ValFlags(map[string]bool{"read": false})

		flat, err := abi.LowerFlat(lowerCtx, singleFlag, val)
		require.NoError(t, err)
		require.Equal(t, 1, len(flat))
		require.Equal(t, uint64(0), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, singleFlag, iter)
		require.NoError(t, err)
		require.False(t, lifted.Flags()["read"])
	})

	t.Run("roundtrip_heap", func(t *testing.T) {
		val := types.ValFlags(map[string]bool{"read": true})

		err := abi.LowerHeap(lowerCtx, singleFlag, val, 0)
		require.NoError(t, err)

		memBytes := lowerCtx.Memory.(*wazerotest.Memory).Bytes
		require.Equal(t, uint8(1), memBytes[0])

		lifted, err := abi.LiftHeap(liftCtx, singleFlag, 0)
		require.NoError(t, err)
		require.True(t, lifted.Flags()["read"])
	})
}

// TestFlagsMultipleFlags asserts a 3-flag type packs bits 0..2 into a
// single u8 and round-trips through common combinations.
//
// Spec: definitions.py:1112-1117 alignment_flags, :1885
// lower_flat_flags packs bits.
// Canonical test: run_tests.py covers flags via test_heap.
func TestFlagsMultipleFlags(t *testing.T) {
	b := newBuilder()
	multiFlags := b.InternFlags([]string{"read", "write", "execute"})
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct, Memory: wazerotest.NewMemory(wazerotest.PageSize)}
	liftCtx := &abi.LiftContext{Types: ct, Memory: lowerCtx.Memory}

	t.Run("type_properties", func(t *testing.T) {
		abiInfo := multiFlags.ABI(ct)
		require.Equal(t, uint32(1), abiInfo.Size32)
		require.Equal(t, uint32(1), abiInfo.Align32)
		require.Equal(t, int32(1), abiInfo.FlattenCount)
	})

	t.Run("read_execute_combination", func(t *testing.T) {
		val := types.ValFlags(map[string]bool{
			"read": true, "write": false, "execute": true,
		})

		flat, err := abi.LowerFlat(lowerCtx, multiFlags, val)
		require.NoError(t, err)
		require.Equal(t, 1, len(flat))
		require.Equal(t, uint64(0b101), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, multiFlags, iter)
		require.NoError(t, err)
		flags := lifted.Flags()
		require.True(t, flags["read"])
		require.False(t, flags["write"])
		require.True(t, flags["execute"])
	})

	t.Run("all_flags_set", func(t *testing.T) {
		val := types.ValFlags(map[string]bool{
			"read": true, "write": true, "execute": true,
		})

		flat, err := abi.LowerFlat(lowerCtx, multiFlags, val)
		require.NoError(t, err)
		require.Equal(t, uint64(0b111), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, multiFlags, iter)
		require.NoError(t, err)
		flags := lifted.Flags()
		require.True(t, flags["read"])
		require.True(t, flags["write"])
		require.True(t, flags["execute"])
	})

	t.Run("no_flags_set", func(t *testing.T) {
		val := types.ValFlags(map[string]bool{
			"read": false, "write": false, "execute": false,
		})

		flat, err := abi.LowerFlat(lowerCtx, multiFlags, val)
		require.NoError(t, err)
		require.Equal(t, uint64(0), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, multiFlags, iter)
		require.NoError(t, err)
		flags := lifted.Flags()
		require.False(t, flags["read"])
		require.False(t, flags["write"])
		require.False(t, flags["execute"])
	})

	t.Run("roundtrip_heap_preserves_bits", func(t *testing.T) {
		val := types.ValFlags(map[string]bool{
			"read": true, "write": false, "execute": true,
		})

		err := abi.LowerHeap(lowerCtx, multiFlags, val, 0)
		require.NoError(t, err)

		memBytes := lowerCtx.Memory.(*wazerotest.Memory).Bytes
		require.Equal(t, uint8(0b101), memBytes[0])

		lifted, err := abi.LiftHeap(liftCtx, multiFlags, 0)
		require.NoError(t, err)
		flags := lifted.Flags()
		require.True(t, flags["read"])
		require.False(t, flags["write"])
		require.True(t, flags["execute"])
	})
}

// TestFlagsLarge asserts a 9-flag type uses u16 storage (size 2,
// align 2) and preserves bits across the byte boundary.
//
// Spec: definitions.py:1112-1117 alignment_flags: 8 < n <= 16 → 2.
// Canonical test: run_tests.py covers flags via test_heap.
func TestFlagsLarge(t *testing.T) {
	b := newBuilder()
	names := make([]string, 9)
	for i := 0; i < 9; i++ {
		names[i] = fmt.Sprintf("flag%d", i)
	}
	largeFlags := b.InternFlags(names)
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct, Memory: wazerotest.NewMemory(wazerotest.PageSize)}
	liftCtx := &abi.LiftContext{Types: ct, Memory: lowerCtx.Memory}

	t.Run("type_properties", func(t *testing.T) {
		abiInfo := largeFlags.ABI(ct)
		require.Equal(t, uint32(2), abiInfo.Size32)
		require.Equal(t, uint32(2), abiInfo.Align32)
		require.Equal(t, int32(1), abiInfo.FlattenCount)
	})

	t.Run("boundary_flags", func(t *testing.T) {
		flagMap := make(map[string]bool)
		for _, name := range names {
			flagMap[name] = false
		}
		flagMap["flag0"] = true
		flagMap["flag8"] = true

		val := types.ValFlags(flagMap)

		flat, err := abi.LowerFlat(lowerCtx, largeFlags, val)
		require.NoError(t, err)
		require.Equal(t, 1, len(flat))
		require.Equal(t, uint64((1<<0)|(1<<8)), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, largeFlags, iter)
		require.NoError(t, err)
		flags := lifted.Flags()
		require.True(t, flags["flag0"])
		require.True(t, flags["flag8"])
		for i := 1; i < 8; i++ {
			require.False(t, flags[fmt.Sprintf("flag%d", i)])
		}
	})

	t.Run("roundtrip_heap", func(t *testing.T) {
		flagMap := make(map[string]bool)
		for _, name := range names {
			flagMap[name] = false
		}
		flagMap["flag0"] = true
		flagMap["flag8"] = true

		val := types.ValFlags(flagMap)

		err := abi.LowerHeap(lowerCtx, largeFlags, val, 0)
		require.NoError(t, err)

		memBytes := lowerCtx.Memory.(*wazerotest.Memory).Bytes
		expected := uint16((1 << 0) | (1 << 8))
		actual := binary.LittleEndian.Uint16(memBytes[0:2])
		require.Equal(t, expected, actual)

		lifted, err := abi.LiftHeap(liftCtx, largeFlags, 0)
		require.NoError(t, err)
		flags := lifted.Flags()
		require.True(t, flags["flag0"])
		require.True(t, flags["flag8"])
	})
}

// TestFlagsVeryLarge asserts a 17-flag type uses u32 storage (size 4,
// align 4) and preserves bits 0..16.
//
// Spec: definitions.py:1112-1117 alignment_flags: 16 < n <= 32 → 4.
// Canonical test: run_tests.py covers flags via test_heap.
func TestFlagsVeryLarge(t *testing.T) {
	b := newBuilder()
	names := make([]string, 17)
	for i := 0; i < 17; i++ {
		names[i] = fmt.Sprintf("flag%d", i)
	}
	veryLargeFlags := b.InternFlags(names)
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct, Memory: wazerotest.NewMemory(wazerotest.PageSize)}
	liftCtx := &abi.LiftContext{Types: ct, Memory: lowerCtx.Memory}

	t.Run("type_properties", func(t *testing.T) {
		abiInfo := veryLargeFlags.ABI(ct)
		require.Equal(t, uint32(4), abiInfo.Size32)
		require.Equal(t, uint32(4), abiInfo.Align32)
		require.Equal(t, int32(1), abiInfo.FlattenCount)
	})

	t.Run("boundary_flags", func(t *testing.T) {
		flagMap := make(map[string]bool)
		for _, name := range names {
			flagMap[name] = false
		}
		flagMap["flag0"] = true
		flagMap["flag16"] = true

		val := types.ValFlags(flagMap)

		flat, err := abi.LowerFlat(lowerCtx, veryLargeFlags, val)
		require.NoError(t, err)
		require.Equal(t, 1, len(flat))
		require.Equal(t, uint64((1<<0)|(1<<16)), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, veryLargeFlags, iter)
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

		flat, err := abi.LowerFlat(lowerCtx, veryLargeFlags, val)
		require.NoError(t, err)
		require.Equal(t, 1, len(flat))
		require.Equal(t, uint64(0x1FFFF), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, veryLargeFlags, iter)
		require.NoError(t, err)
		flags := lifted.Flags()
		for _, name := range names {
			require.True(t, flags[name])
		}
	})

	t.Run("roundtrip_heap", func(t *testing.T) {
		flagMap := make(map[string]bool)
		for _, name := range names {
			flagMap[name] = false
		}
		flagMap["flag0"] = true
		flagMap["flag16"] = true

		val := types.ValFlags(flagMap)

		err := abi.LowerHeap(lowerCtx, veryLargeFlags, val, 0)
		require.NoError(t, err)

		memBytes := lowerCtx.Memory.(*wazerotest.Memory).Bytes
		expected := uint32((1 << 0) | (1 << 16))
		actual := binary.LittleEndian.Uint32(memBytes[0:4])
		require.Equal(t, expected, actual)

		lifted, err := abi.LiftHeap(liftCtx, veryLargeFlags, 0)
		require.NoError(t, err)
		flags := lifted.Flags()
		require.True(t, flags["flag0"])
		require.True(t, flags["flag16"])
	})
}

// TestFlagsSizeThresholds asserts the flag-count size boundary table
// at 8, 16, 32, 33 flag counts. This includes wazero's divergence
// (2) from the literal spec: 33+ flags use multi-i32 encoding.
//
// Spec: definitions.py:1112-1117 alignment_flags (literal spec caps
// at n <= 32). Divergence (2): wasmtime / wazero allow n > 32 via
// Size4Plus multi-word encoding.
// Canonical test: run_tests.py::test_flatten uses the 32-cap form.
func TestFlagsSizeThresholds(t *testing.T) {
	testCases := []struct {
		numFlags      int
		expectedSize  uint32
		expectedAlign uint32
	}{
		{1, 1, 1},
		{8, 1, 1},
		{9, 2, 2},
		{16, 2, 2},
		{17, 4, 4},
		{32, 4, 4},
		{33, 8, 4},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%d_flags", tc.numFlags), func(t *testing.T) {
			b := newBuilder()
			names := make([]string, tc.numFlags)
			for i := 0; i < tc.numFlags; i++ {
				names[i] = fmt.Sprintf("f%d", i)
			}
			flags := b.InternFlags(names)
			ct := b.Finish()

			abiInfo := flags.ABI(ct)
			require.Equal(t, tc.expectedSize, abiInfo.Size32)
			require.Equal(t, tc.expectedAlign, abiInfo.Align32)
		})
	}
}

// =============================================================================
// Enum Tests
// =============================================================================

// TestEnumSimple asserts a 3-case enum has size 1, align 1,
// FlattenCount 1 and round-trips every case.
//
// Spec: definitions.py:163-165 EnumType discriminant-only variant;
// :1096-1103 discriminant_type.
// Canonical test: run_tests.py covers enums via test_heap.
func TestEnumSimple(t *testing.T) {
	b := newBuilder()
	colorEnum := b.InternEnum([]string{"red", "green", "blue"})
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct, Memory: wazerotest.NewMemory(wazerotest.PageSize)}
	liftCtx := &abi.LiftContext{Types: ct, Memory: lowerCtx.Memory}

	t.Run("type_properties", func(t *testing.T) {
		abiInfo := colorEnum.ABI(ct)
		require.Equal(t, uint32(1), abiInfo.Size32)
		require.Equal(t, uint32(1), abiInfo.Align32)
		require.Equal(t, int32(1), abiInfo.FlattenCount)
	})

	t.Run("select_first_case", func(t *testing.T) {
		val := types.ValEnum("red")

		flat, err := abi.LowerFlat(lowerCtx, colorEnum, val)
		require.NoError(t, err)
		require.Equal(t, 1, len(flat))
		require.Equal(t, uint64(0), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, colorEnum, iter)
		require.NoError(t, err)
		require.Equal(t, "red", lifted.Enum())
	})

	t.Run("select_middle_case", func(t *testing.T) {
		val := types.ValEnum("green")

		flat, err := abi.LowerFlat(lowerCtx, colorEnum, val)
		require.NoError(t, err)
		require.Equal(t, uint64(1), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, colorEnum, iter)
		require.NoError(t, err)
		require.Equal(t, "green", lifted.Enum())
	})

	t.Run("select_last_case", func(t *testing.T) {
		val := types.ValEnum("blue")

		flat, err := abi.LowerFlat(lowerCtx, colorEnum, val)
		require.NoError(t, err)
		require.Equal(t, uint64(2), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, colorEnum, iter)
		require.NoError(t, err)
		require.Equal(t, "blue", lifted.Enum())
	})

	t.Run("roundtrip_heap", func(t *testing.T) {
		val := types.ValEnum("green")

		err := abi.LowerHeap(lowerCtx, colorEnum, val, 0)
		require.NoError(t, err)

		memBytes := lowerCtx.Memory.(*wazerotest.Memory).Bytes
		require.Equal(t, uint8(1), memBytes[0])

		lifted, err := abi.LiftHeap(liftCtx, colorEnum, 0)
		require.NoError(t, err)
		require.Equal(t, "green", lifted.Enum())
	})

	t.Run("all_cases_roundtrip", func(t *testing.T) {
		cases := []string{"red", "green", "blue"}
		for i, caseName := range cases {
			val := types.ValEnum(caseName)

			flat, err := abi.LowerFlat(lowerCtx, colorEnum, val)
			require.NoError(t, err)
			require.Equal(t, uint64(i), flat[0])

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(liftCtx, colorEnum, iter)
			require.NoError(t, err)
			require.Equal(t, caseName, lifted.Enum())
		}
	})
}

// TestEnumLarge asserts a 257-case enum uses u16 (size 2, align 2)
// and round-trips case 256 through the u16 path.
//
// Spec: definitions.py:1096-1103 discriminant_type n > 256 → u16.
// Canonical test: run_tests.py covers enums via test_heap.
func TestEnumLarge(t *testing.T) {
	b := newBuilder()
	cases := make([]string, 257)
	for i := 0; i < 257; i++ {
		cases[i] = fmt.Sprintf("case%d", i)
	}
	largeEnum := b.InternEnum(cases)
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct, Memory: wazerotest.NewMemory(wazerotest.PageSize)}
	liftCtx := &abi.LiftContext{Types: ct, Memory: lowerCtx.Memory}

	t.Run("type_properties", func(t *testing.T) {
		abiInfo := largeEnum.ABI(ct)
		require.Equal(t, uint32(2), abiInfo.Size32)
		require.Equal(t, uint32(2), abiInfo.Align32)
		require.Equal(t, int32(1), abiInfo.FlattenCount)
	})

	t.Run("select_first_case", func(t *testing.T) {
		val := types.ValEnum("case0")

		flat, err := abi.LowerFlat(lowerCtx, largeEnum, val)
		require.NoError(t, err)
		require.Equal(t, uint64(0), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, largeEnum, iter)
		require.NoError(t, err)
		require.Equal(t, "case0", lifted.Enum())
	})

	t.Run("select_case_256", func(t *testing.T) {
		val := types.ValEnum("case256")

		flat, err := abi.LowerFlat(lowerCtx, largeEnum, val)
		require.NoError(t, err)
		require.Equal(t, uint64(256), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, largeEnum, iter)
		require.NoError(t, err)
		require.Equal(t, "case256", lifted.Enum())
	})

	t.Run("roundtrip_heap", func(t *testing.T) {
		val := types.ValEnum("case256")

		err := abi.LowerHeap(lowerCtx, largeEnum, val, 0)
		require.NoError(t, err)

		memBytes := lowerCtx.Memory.(*wazerotest.Memory).Bytes
		actual := binary.LittleEndian.Uint16(memBytes[0:2])
		require.Equal(t, uint16(256), actual)

		lifted, err := abi.LiftHeap(liftCtx, largeEnum, 0)
		require.NoError(t, err)
		require.Equal(t, "case256", lifted.Enum())
	})
}

// TestEnumSizeThresholds asserts the enum discriminant-size table at
// 1, 256, 257, 65536 cases.
//
// Spec: definitions.py:1096-1103 discriminant_type boundaries.
// Canonical test: run_tests.py covers enums via test_heap.
func TestEnumSizeThresholds(t *testing.T) {
	testCases := []struct {
		numCases      int
		expectedSize  uint32
		expectedAlign uint32
	}{
		{1, 1, 1},
		{256, 1, 1},
		{257, 2, 2},
		{65536, 2, 2},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%d_cases", tc.numCases), func(t *testing.T) {
			b := newBuilder()
			cases := make([]string, tc.numCases)
			for i := 0; i < tc.numCases; i++ {
				cases[i] = fmt.Sprintf("c%d", i)
			}
			enum := b.InternEnum(cases)
			ct := b.Finish()

			abiInfo := enum.ABI(ct)
			require.Equal(t, tc.expectedSize, abiInfo.Size32)
			require.Equal(t, tc.expectedAlign, abiInfo.Align32)
		})
	}
}

// TestEnumInvalidCase asserts LowerFlat errors when a Val carries a
// case name not in the enum.
//
// Spec: definitions.py:1886 lower_flat_enum would KeyError; wazero
// returns an error.
// Canonical test: no direct negative case — wazero surface check.
func TestEnumInvalidCase(t *testing.T) {
	b := newBuilder()
	colorEnum := b.InternEnum([]string{"red", "green", "blue"})
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}

	val := types.ValEnum("yellow")

	_, err := abi.LowerFlat(lowerCtx, colorEnum, val)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown enum case")
}

// TestEnumInvalidDiscriminant asserts LiftFlat errors when the
// lifted discriminant is out of range.
//
// Spec: definitions.py:1789 lift_flat_variant asserts the lifted
// discriminant is in range; the enum equivalent does the same.
// Canonical test: no direct negative case — wazero surface check.
func TestEnumInvalidDiscriminant(t *testing.T) {
	b := newBuilder()
	colorEnum := b.InternEnum([]string{"red", "green", "blue"})
	ct := b.Finish()

	liftCtx := &abi.LiftContext{Types: ct}

	flat := []uint64{5}
	iter := abi.NewFlatIter(flat)
	_, err := abi.LiftFlat(liftCtx, colorEnum, iter)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid enum discriminant")
}
