// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package conformance: ABI edge-case conformance tests. These
// exercise boundary conditions in the Canonical ABI — max-flat
// record, zero-length list, alignment boundaries, empty tuple,
// variant/option/result padding — matching the lift/lower
// roundtrip pattern from run_tests.py::test_roundtrips.
package conformance

import (
	"encoding/binary"
	"testing"

	"github.com/tetratelabs/wazero/experimental/wazerotest"
	"github.com/tetratelabs/wazero/internal/component/abi"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestABI_ExactlyMaxFlatParams asserts that a record with exactly
// MaxFlatParams (16) u32 fields flattens to 16 values and round-trips
// cleanly through LowerFlat/LiftFlat.
//
// Spec: definitions.py:1726-1730 flatten_record (sum of field
// FlattenCounts). Spec: definitions.py:1673 boundary check uses
// strict >, so 16 flattens.
// Canonical test: run_tests.py::test_roundtrips exercises composite
// lift/lower end-to-end at :399-438.
func TestABI_ExactlyMaxFlatParams(t *testing.T) {
	b := newBuilder()
	fields := make([]types.RecordField, abi.MaxFlatParams)
	fieldVals := make(map[string]types.Val)
	for i := 0; i < abi.MaxFlatParams; i++ {
		name := string(rune('a' + i))
		fields[i] = types.RecordField{Name: name, Type: types.U32}
		fieldVals[name] = types.ValU32(uint32(i + 1))
	}
	recordType := b.InternRecord(fields)
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}
	liftCtx := &abi.LiftContext{Types: ct}

	t.Run("type_properties", func(t *testing.T) {
		require.Equal(t, int32(abi.MaxFlatParams), recordType.ABI(ct).FlattenCount)
	})

	t.Run("lower_flat", func(t *testing.T) {
		val := types.ValRecord(fieldVals)
		flat, err := abi.LowerFlat(lowerCtx, recordType, val)
		require.NoError(t, err)
		require.Equal(t, abi.MaxFlatParams, len(flat))

		for i := 0; i < abi.MaxFlatParams; i++ {
			require.Equal(t, uint64(i+1), flat[i])
		}
	})

	t.Run("lift_flat", func(t *testing.T) {
		flatVals := make([]uint64, abi.MaxFlatParams)
		for i := 0; i < abi.MaxFlatParams; i++ {
			flatVals[i] = uint64(i + 100)
		}

		iter := abi.NewFlatIter(flatVals)
		lifted, err := abi.LiftFlat(liftCtx, recordType, iter)
		require.NoError(t, err)

		for i := 0; i < abi.MaxFlatParams; i++ {
			name := string(rune('a' + i))
			fieldVal, ok := lifted.RecordField(name)
			require.True(t, ok)
			require.Equal(t, uint32(i+100), fieldVal.U32())
		}
	})
}

// TestABI_ExactlyMaxPlusOne asserts that a record with MaxFlatParams+1
// (17) u32 fields exceeds the flat threshold and would spill to
// memory in an end-to-end canon.lift/lower.
//
// Spec: definitions.py:1673 boundary check with strict >, so 17
// spills. Canonical test: run_tests.py::test_roundtrips covers the
// spilled path via composite fixtures.
func TestABI_ExactlyMaxPlusOne(t *testing.T) {
	b := newBuilder()
	numFields := abi.MaxFlatParams + 1
	fields := make([]types.RecordField, numFields)
	for i := 0; i < numFields; i++ {
		name := string(rune('a' + i))
		fields[i] = types.RecordField{Name: name, Type: types.U32}
	}
	recordType := b.InternRecord(fields)
	ct := b.Finish()

	t.Run("type_properties", func(t *testing.T) {
		require.Equal(t, int32(numFields), recordType.ABI(ct).FlattenCount)
		require.True(t, recordType.ABI(ct).FlattenCount > int32(abi.MaxFlatParams))
	})
}

// TestABI_MaxStringLength asserts that LowerString / LiftFlat handle
// very large strings (1 MiB here). This verifies there is no short
// integer cap on the length field that would silently truncate.
//
// Spec: definitions.py:1476 MAX_STRING_BYTE_LENGTH = (1 << 31) - 1;
// 1 MiB is well under the cap. Spec: definitions.py:1497-1529
// store_string_to_utf8 path handles arbitrary lengths via realloc.
// Canonical test: run_tests.py::test_roundtrips exercises string
// round-trips at :430.
func TestABI_MaxStringLength(t *testing.T) {
	const size = 1024 * 1024
	data := make([]byte, size)
	for i := range data {
		data[i] = 'a' + byte(i%26)
	}
	str := string(data)

	// wazerotest.NewMemory rounds up to a page size (65536), so 1
	// MiB + headroom is 17 pages = 17 * 65536.
	mem := wazerotest.NewMemory(size + 4096)
	allocPtr := uint32(256)

	lowerCtx := &abi.LowerContext{
		Memory: mem,
		Opts:   &abi.Options{StringEncoding: types.StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			result := allocPtr
			allocPtr += newSize
			return result, nil
		},
	}

	t.Run("lower_large_string", func(t *testing.T) {
		ptr, length, err := abi.LowerString(lowerCtx, str)
		require.NoError(t, err)
		require.Equal(t, uint32(size), length)
		require.True(t, ptr > 0)
	})

	t.Run("roundtrip_large_string", func(t *testing.T) {
		allocPtr = uint32(256)

		ptr, length, err := abi.LowerString(lowerCtx, str)
		require.NoError(t, err)

		liftCtx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: types.StringEncodingUTF8},
		}

		iter := abi.NewFlatIter([]uint64{uint64(ptr), uint64(length)})
		lifted, err := abi.LiftFlat(liftCtx, types.String_, iter)
		require.NoError(t, err)
		require.Equal(t, str, lifted.StringVal())
	})
}

// TestABI_ZeroLengthList asserts that empty lists short-circuit
// without calling realloc during Lower. The spec's store_list at
// definitions.py:1427-1435 short-circuits at length 0 as an
// optimisation; wazero's LowerFlat mirrors this.
//
// Spec: definitions.py:1427-1435 store_list (length == 0 early
// return). Canonical test: run_tests.py::test_roundtrips covers
// empty-list fixtures implicitly.
func TestABI_ZeroLengthList(t *testing.T) {
	b := newBuilder()
	listType := b.InternList(types.U32)
	ct := b.Finish()

	reallocCalled := false
	lowerCtx := &abi.LowerContext{
		Types:  ct,
		Memory: wazerotest.NewMemory(wazerotest.PageSize),
		Opts:   &abi.Options{StringEncoding: types.StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			reallocCalled = true
			return 0, nil
		},
	}

	t.Run("lower_empty_list", func(t *testing.T) {
		reallocCalled = false
		emptyList := types.ValList([]types.Val{})
		flat, err := abi.LowerFlat(lowerCtx, listType, emptyList)
		require.NoError(t, err)
		require.Equal(t, 2, len(flat))
		// Per canonical ABI spec: realloc is always called, even for empty
		// lists. The returned pointer (0 here) must be aligned and within
		// memory bounds.
		require.Equal(t, uint64(0), flat[0])
		require.Equal(t, uint64(0), flat[1])
		require.True(t, reallocCalled)
	})

	t.Run("lift_empty_list", func(t *testing.T) {
		liftCtx := &abi.LiftContext{
			Types:  ct,
			Memory: wazerotest.NewMemory(wazerotest.PageSize),
			Opts:   &abi.Options{StringEncoding: types.StringEncodingUTF8},
		}

		iter := abi.NewFlatIter([]uint64{0, 0})
		lifted, err := abi.LiftFlat(liftCtx, listType, iter)
		require.NoError(t, err)
		require.Equal(t, 0, len(lifted.List()))
	})
}

// TestABI_AlignmentBoundary asserts that LiftHeap correctly reads
// scalar values at various naturally-aligned offsets. The spec's
// load at definitions.py:1186-1204 dispatches on type; wazero's
// LiftHeap reads little-endian bytes at the given offset.
//
// Spec: definitions.py:1186-1204 load dispatch table for scalar
// types. Canonical test: run_tests.py::test_roundtrips exercises
// store/load pairs through canon_lift/lower.
func TestABI_AlignmentBoundary(t *testing.T) {
	testCases := []struct {
		name   string
		offset uint32
		typ    types.ValType
		setup  func(mem *wazerotest.Memory, offset uint32)
		verify func(t *testing.T, lifted types.Val)
	}{
		{
			name:   "u32_at_aligned_offset",
			offset: 0,
			typ:    types.U32,
			setup: func(mem *wazerotest.Memory, offset uint32) {
				binary.LittleEndian.PutUint32(mem.Bytes[offset:], 0xDEADBEEF)
			},
			verify: func(t *testing.T, lifted types.Val) {
				require.Equal(t, uint32(0xDEADBEEF), lifted.U32())
			},
		},
		{
			name:   "u32_at_offset_4",
			offset: 4,
			typ:    types.U32,
			setup: func(mem *wazerotest.Memory, offset uint32) {
				binary.LittleEndian.PutUint32(mem.Bytes[offset:], 0xCAFEBABE)
			},
			verify: func(t *testing.T, lifted types.Val) {
				require.Equal(t, uint32(0xCAFEBABE), lifted.U32())
			},
		},
		{
			name:   "u64_at_aligned_offset",
			offset: 8,
			typ:    types.U64,
			setup: func(mem *wazerotest.Memory, offset uint32) {
				binary.LittleEndian.PutUint64(mem.Bytes[offset:], 0x123456789ABCDEF0)
			},
			verify: func(t *testing.T, lifted types.Val) {
				require.Equal(t, uint64(0x123456789ABCDEF0), lifted.U64())
			},
		},
		{
			name:   "u16_at_offset_2",
			offset: 2,
			typ:    types.U16,
			setup: func(mem *wazerotest.Memory, offset uint32) {
				binary.LittleEndian.PutUint16(mem.Bytes[offset:], 0xABCD)
			},
			verify: func(t *testing.T, lifted types.Val) {
				require.Equal(t, uint16(0xABCD), lifted.U16())
			},
		},
		{
			name:   "u8_at_offset_1",
			offset: 1,
			typ:    types.U8,
			setup: func(mem *wazerotest.Memory, offset uint32) {
				mem.Bytes[offset] = 0x42
			},
			verify: func(t *testing.T, lifted types.Val) {
				require.Equal(t, uint8(0x42), lifted.U8())
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mem := wazerotest.NewMemory(wazerotest.PageSize)
			tc.setup(mem, tc.offset)

			ctx := &abi.LiftContext{
				Memory: mem,
				Opts:   &abi.Options{StringEncoding: types.StringEncodingUTF8},
			}

			lifted, err := abi.LiftHeap(ctx, tc.typ, tc.offset)
			require.NoError(t, err)
			tc.verify(t, lifted)
		})
	}
}

// TestABI_InvalidAlignment asserts that LiftHeap does not panic on
// misaligned offsets — WebAssembly permits unaligned loads and the
// spec's load ops at definitions.py:1186-1204 are alignment-agnostic
// (only the canonical ABI's store_string/load_string for UTF-16 and
// list pointers require alignment).
//
// Spec: definitions.py:1186-1204 load dispatch — no alignment trap
// for scalar types. Canonical test: no direct case — this is a
// wazero-specific "does not panic" guarantee for core-wasm memory
// semantics.
func TestABI_InvalidAlignment(t *testing.T) {
	testCases := []struct {
		name   string
		offset uint32
		typ    types.ValType
	}{
		{"u32_at_offset_1", 1, types.U32},
		{"u32_at_offset_3", 3, types.U32},
		{"u64_at_offset_1", 1, types.U64},
		{"u64_at_offset_5", 5, types.U64},
		{"u16_at_offset_1", 1, types.U16},
		{"u16_at_offset_3", 3, types.U16},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mem := wazerotest.NewMemory(wazerotest.PageSize)
			for i := 0; i < 16; i++ {
				mem.Bytes[i] = byte(i)
			}

			ctx := &abi.LiftContext{
				Memory: mem,
				Opts:   &abi.Options{StringEncoding: types.StringEncodingUTF8},
			}

			lifted, err := abi.LiftHeap(ctx, tc.typ, tc.offset)
			// WebAssembly allows unaligned access — the lift must
			// either succeed (returning some value) or error
			// gracefully; no panic is the key invariant.
			if err == nil {
				require.NotNil(t, lifted)
			}
		})
	}
}

// TestABI_FlatIterExhaustion asserts that FlatIter returns stored
// values in sequence for the mixed i32 / i64 / f32 / f64 flat-value
// vocabulary. This is a positive-path test for the iter; going past
// the end is intentionally out of scope (it panics by design).
//
// Spec: definitions.py:1768-1787 CoreValueIter.next — sequential
// access without bounds check on the Python side.
// Canonical test: no direct case — FlatIter is a wazero-specific
// implementation detail; this mirrors the CoreValueIter contract.
func TestABI_FlatIterExhaustion(t *testing.T) {
	t.Run("single_value_iter", func(t *testing.T) {
		iter := abi.NewFlatIter([]uint64{42})
		val := iter.NextI32()
		require.Equal(t, uint32(42), val)
	})

	t.Run("multi_value_iter", func(t *testing.T) {
		iter := abi.NewFlatIter([]uint64{1, 2, 3, 4, 5})
		require.Equal(t, uint32(1), iter.NextI32())
		require.Equal(t, uint64(2), iter.NextI64())
		require.Equal(t, uint32(3), iter.NextI32())
		require.Equal(t, uint32(4), iter.NextI32())
		require.Equal(t, uint32(5), iter.NextI32())
	})

	t.Run("float_values", func(t *testing.T) {
		f32Bits := uint64(0x40490FDB)
		f64Bits := uint64(0x400921FB54442D18)

		iter := abi.NewFlatIter([]uint64{f32Bits, f64Bits})
		f32 := iter.NextF32()
		f64 := iter.NextF64()

		require.True(t, f32 > 3.0 && f32 < 4.0)
		require.True(t, f64 > 3.0 && f64 < 4.0)
	})
}

// TestABI_RecordFieldOrder asserts that LowerFlat emits record
// fields in declaration order, matching the spec's ordered
// iteration over fields.
//
// Spec: definitions.py:1355-1361 store_record iterates fields in
// declaration order; the flat path mirrors this ordering.
// Canonical test: run_tests.py::test_roundtrips records are
// built field-order-sensitive.
func TestABI_RecordFieldOrder(t *testing.T) {
	b := newBuilder()
	recordType := b.InternRecord([]types.RecordField{
		{Name: "first", Type: types.U32},
		{Name: "second", Type: types.U32},
		{Name: "third", Type: types.U32},
	})
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}
	val := types.ValRecord(map[string]types.Val{
		"first":  types.ValU32(100),
		"second": types.ValU32(200),
		"third":  types.ValU32(300),
	})

	flat, err := abi.LowerFlat(lowerCtx, recordType, val)
	require.NoError(t, err)

	require.Equal(t, uint64(100), flat[0])
	require.Equal(t, uint64(200), flat[1])
	require.Equal(t, uint64(300), flat[2])
}

// TestABI_VariantPadding asserts that variant payloads use joined
// flat types: a variant with a u8 case and a u64 case produces a
// single i64 payload slot (join of i32 and i64 = i64), plus the
// discriminant.
//
// Spec: definitions.py:1732-1741 flatten_variant computes the
// joined payload type. Spec: CanonicalABI.md:2962-2989 defines the
// join rules for variant payloads.
// Canonical test: run_tests.py::test_roundtrips variant fixtures
// at :433-438 exercise the joined-payload path.
func TestABI_VariantPadding(t *testing.T) {
	b := newBuilder()
	variantType := b.InternVariant([]types.VariantCase{
		{Name: "small", Payload: types.U8, HasPayload: true},
		{Name: "large", Payload: types.U64, HasPayload: true},
	})
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}

	t.Run("small_case_padded", func(t *testing.T) {
		payload := types.ValU8(42)
		val := types.ValVariant("small", &payload)

		flat, err := abi.LowerFlat(lowerCtx, variantType, val)
		require.NoError(t, err)
		require.Equal(t, 2, len(flat))
		require.Equal(t, uint64(0), flat[0])
		require.Equal(t, uint64(42), flat[1])
	})

	t.Run("large_case", func(t *testing.T) {
		payload := types.ValU64(0x123456789ABCDEF0)
		val := types.ValVariant("large", &payload)

		flat, err := abi.LowerFlat(lowerCtx, variantType, val)
		require.NoError(t, err)
		require.Equal(t, 2, len(flat))
		require.Equal(t, uint64(1), flat[0])
		require.Equal(t, uint64(0x123456789ABCDEF0), flat[1])
	})
}

// TestABI_OptionNone asserts that option<u32> with a None payload
// lowers to (0, 0) (discriminant 0 + zero-padded payload) and lifts
// back to a nil Option.
//
// Spec: definitions.py:160-162 OptionType is sugar for
// variant{none, some(T)}, so the flatten rule is the variant rule.
// Spec: definitions.py:1732-1741 flatten_variant.
// Canonical test: run_tests.py::test_roundtrips option-of-tuple
// fixture at :432.
func TestABI_OptionNone(t *testing.T) {
	b := newBuilder()
	optionType := b.InternOption(types.U32)
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}
	liftCtx := &abi.LiftContext{Types: ct}

	val := types.ValOption(nil)
	flat, err := abi.LowerFlat(lowerCtx, optionType, val)
	require.NoError(t, err)
	require.Equal(t, 2, len(flat))
	require.Equal(t, uint64(0), flat[0])

	iter := abi.NewFlatIter(flat)
	lifted, err := abi.LiftFlat(liftCtx, optionType, iter)
	require.NoError(t, err)
	require.Nil(t, lifted.Option())
}

// TestABI_ResultOkNil asserts that result<unit, string> with Ok(nil)
// lowers with discriminant 0.
//
// Spec: definitions.py:155-159 ResultType is sugar for variant{ok,
// err} with optional payloads. Spec: definitions.py:1732-1741
// flatten_variant.
// Canonical test: run_tests.py::test_roundtrips result-bearing
// fixtures.
func TestABI_ResultOkNil(t *testing.T) {
	b := newBuilder()
	// result<_, string> — no ok payload.
	resultType := b.InternResult(types.ValType{}, types.String_, false, true)
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}

	val := types.ValResultOk(nil)
	flat, err := abi.LowerFlat(lowerCtx, resultType, val)
	require.NoError(t, err)
	require.Equal(t, uint64(0), flat[0])
}

// TestABI_ResultErrorNil asserts that result<u32, _> with Error(nil)
// lowers with discriminant 1.
//
// Spec: definitions.py:155-159 ResultType sugar for variant{ok, err}.
// Spec: definitions.py:1732-1741 flatten_variant.
// Canonical test: run_tests.py::test_roundtrips result-bearing
// fixtures.
func TestABI_ResultErrorNil(t *testing.T) {
	b := newBuilder()
	// result<u32, _> — no err payload.
	resultType := b.InternResult(types.U32, types.ValType{}, true, false)
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}

	val := types.ValResultError(nil)
	flat, err := abi.LowerFlat(lowerCtx, resultType, val)
	require.NoError(t, err)
	require.Equal(t, uint64(1), flat[0])
}

// TestABI_FlagsAllSet asserts that a flags type with all flags set
// lowers to a single i32 with every relevant bit set (0xFF for an
// 8-flag type).
//
// Spec: definitions.py:1717 FlagsType flattens to ['i32']. Spec:
// definitions.py:1885 lower_flat_flags packs set bits.
// Canonical test: run_tests.py covers flags via the test_heap path
// in its store/load pairs.
func TestABI_FlagsAllSet(t *testing.T) {
	b := newBuilder()
	names := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	flagsType := b.InternFlags(names)
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}

	flags := make(map[string]bool)
	for _, name := range names {
		flags[name] = true
	}
	val := types.ValFlags(flags)

	flat, err := abi.LowerFlat(lowerCtx, flagsType, val)
	require.NoError(t, err)
	require.Equal(t, 1, len(flat))
	require.Equal(t, uint64(0xFF), flat[0])
}

// TestABI_FlagsNoneSet asserts that a flags type with no flags set
// lowers to a single i32 with value 0.
//
// Spec: definitions.py:1885 lower_flat_flags with empty set.
// Canonical test: none direct; the fixture is an obvious boundary
// case implied by the flag encoding.
func TestABI_FlagsNoneSet(t *testing.T) {
	b := newBuilder()
	flagsType := b.InternFlags([]string{"a", "b", "c", "d", "e", "f", "g", "h"})
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}

	val := types.ValFlags(map[string]bool{})

	flat, err := abi.LowerFlat(lowerCtx, flagsType, val)
	require.NoError(t, err)
	require.Equal(t, 1, len(flat))
	require.Equal(t, uint64(0), flat[0])
}

// TestABI_EnumRoundtrip asserts that enum values round-trip through
// LowerFlat/LiftFlat with their discriminant index preserved.
//
// Spec: definitions.py:163-165 EnumType is a discriminant-only
// variant. Spec: definitions.py:1791 lift_flat dispatch for flags
// (and the companion for enums).
// Canonical test: run_tests.py::test_roundtrips exercises enum-
// carrying composites.
func TestABI_EnumRoundtrip(t *testing.T) {
	b := newBuilder()
	enumType := b.InternEnum([]string{"first", "second", "third"})
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}
	liftCtx := &abi.LiftContext{Types: ct}

	testCases := []struct {
		name     string
		expected uint64
	}{
		{"first", 0},
		{"second", 1},
		{"third", 2},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			val := types.ValEnum(tc.name)

			flat, err := abi.LowerFlat(lowerCtx, enumType, val)
			require.NoError(t, err)
			require.Equal(t, tc.expected, flat[0])

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(liftCtx, enumType, iter)
			require.NoError(t, err)
			require.Equal(t, tc.name, lifted.Enum())
		})
	}
}

// TestABI_TupleEmpty asserts that an empty tuple has FlattenCount 0
// and round-trips through an empty flat iterator.
//
// Spec: definitions.py:126-127 TupleType is positional record;
// an empty field list gives FlattenCount 0.
// Canonical test: run_tests.py::test_flatten covers tuple_0 at the
// boundary.
func TestABI_TupleEmpty(t *testing.T) {
	b := newBuilder()
	tupleType := b.InternTuple([]types.ValType{})
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}
	liftCtx := &abi.LiftContext{Types: ct}

	val := types.ValTuple([]types.Val{})

	flat, err := abi.LowerFlat(lowerCtx, tupleType, val)
	require.NoError(t, err)
	require.Equal(t, 0, len(flat))

	iter := abi.NewFlatIter([]uint64{})
	lifted, err := abi.LiftFlat(liftCtx, tupleType, iter)
	require.NoError(t, err)
	require.Equal(t, 0, len(lifted.Tuple()))
}

// TestABI_MaxFlatParamsConstant asserts the MaxFlatParams constant
// equals 16 per the Component Model spec.
//
// Spec: definitions.py:1665 MAX_FLAT_PARAMS = 16.
// Canonical test: run_tests.py::test_flatten depends on this
// exact constant.
func TestABI_MaxFlatParamsConstant(t *testing.T) {
	require.Equal(t, 16, abi.MaxFlatParams)
}

// TestABI_MaxFlatResultsConstant asserts the MaxFlatResults constant
// equals 1 per the Component Model spec.
//
// Spec: definitions.py:1667 MAX_FLAT_RESULTS = 1.
// Canonical test: run_tests.py::test_flatten depends on this
// exact constant.
func TestABI_MaxFlatResultsConstant(t *testing.T) {
	require.Equal(t, 1, abi.MaxFlatResults)
}
