// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package conformance: flat-ABI boundary tests. These verify the
// MAX_FLAT_PARAMS (16) and MAX_FLAT_RESULTS (1) thresholds from
// definitions.py:1665-1667 and their effect on flatten_functype at
// :1669-1698. Types are constructed via the post-Session-0
// ComponentTypesBuilder interning API; scalar types use the named
// ValType constants.
package conformance

import (
	"fmt"
	"testing"

	"github.com/tetratelabs/wazero/internal/component/abi"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// flattenCount returns the canonical-ABI flatten count for a
// ValType. Thin wrapper around ValType.ABI(ct).FlattenCount for
// readability.
func flattenCount(ct *types.ComponentTypes, v types.ValType) int {
	return int(v.ABI(ct).FlattenCount)
}

// TestFlatABIConstants asserts the flat-ABI limit constants match the
// spec: MAX_FLAT_PARAMS = 16, MAX_FLAT_RESULTS = 1.
//
// Spec: definitions.py:1665 MAX_FLAT_PARAMS = 16. Spec:
// definitions.py:1667 MAX_FLAT_RESULTS = 1.
// Canonical test: run_tests.py::test_flatten exercises the boundary
// behaviour via flatten_functype on each sample function.
func TestFlatABIConstants(t *testing.T) {
	t.Run("MaxFlatParams_is_16", func(t *testing.T) {
		require.Equal(t, 16, abi.MaxFlatParams)
	})

	t.Run("MaxFlatResults_is_1", func(t *testing.T) {
		require.Equal(t, 1, abi.MaxFlatResults)
	})
}

// TestFlatABIExactlyMaxParams asserts that a tuple of exactly
// MaxFlatParams (16) s32 values can pass in registers: FlattenCount
// == 16 and is at the boundary (still flattenable for params).
//
// Spec: definitions.py:1673 if len(flat_params) > MAX_FLAT_PARAMS:
// flat_params = ['i32'] — strict > means count == 16 still flattens.
// Canonical test: run_tests.py::test_flatten with a sample function
// at exactly 16 flat params.
// Wasmtime parallel: tests/all/component_model/func.rs many_parameters
// test exercises the boundary at param-count 16.
func TestFlatABIExactlyMaxParams(t *testing.T) {
	b := newBuilder()
	elems := make([]types.ValType, 16)
	for i := range elems {
		elems[i] = types.S32
	}
	tuple16 := b.InternTuple(elems)
	ct := b.Finish()

	t.Run("FlattenCount_equals_16", func(t *testing.T) {
		require.Equal(t, 16, flattenCount(ct, tuple16))
	})

	t.Run("equals_MaxFlatParams", func(t *testing.T) {
		require.Equal(t, abi.MaxFlatParams, flattenCount(ct, tuple16))
	})

	t.Run("can_flatten_params", func(t *testing.T) {
		require.True(t, flattenCount(ct, tuple16) <= abi.MaxFlatParams)
	})

	t.Run("type_properties", func(t *testing.T) {
		abiInfo := tuple16.ABI(ct)
		require.Equal(t, uint32(64), abiInfo.Size32)
		require.Equal(t, uint32(4), abiInfo.Align32)
	})
}

// TestFlatABIExceedsMaxParams asserts that a tuple of 17 s32 values
// exceeds MaxFlatParams and must spill to memory (via the retptr
// path).
//
// Spec: definitions.py:1673 if len(flat_params) > MAX_FLAT_PARAMS:
// flat_params = ['i32'] — the spilled form is a single i32 pointer.
// Canonical test: run_tests.py::test_flatten covers the >16 spill.
// Wasmtime parallel: tests/all/component_model/func.rs many_parameters
// at param-count 17 and above.
func TestFlatABIExceedsMaxParams(t *testing.T) {
	b := newBuilder()
	elems := make([]types.ValType, 17)
	for i := range elems {
		elems[i] = types.S32
	}
	tuple17 := b.InternTuple(elems)
	ct := b.Finish()

	t.Run("FlattenCount_equals_17", func(t *testing.T) {
		require.Equal(t, 17, flattenCount(ct, tuple17))
	})

	t.Run("exceeds_MaxFlatParams", func(t *testing.T) {
		require.True(t, flattenCount(ct, tuple17) > abi.MaxFlatParams)
	})

	t.Run("cannot_flatten_params", func(t *testing.T) {
		require.False(t, flattenCount(ct, tuple17) <= abi.MaxFlatParams)
	})

	t.Run("type_properties", func(t *testing.T) {
		abiInfo := tuple17.ABI(ct)
		require.Equal(t, uint32(68), abiInfo.Size32)
		require.Equal(t, uint32(4), abiInfo.Align32)
	})
}

// TestFlatABIExactlyMaxResults asserts that a single s32 value is at
// the MaxFlatResults (1) boundary and can be returned in the flat
// register slot.
//
// Spec: definitions.py:1675 if len(flat_results) > MAX_FLAT_RESULTS:
// — strict > means count == 1 still flattens.
// Canonical test: run_tests.py::test_flatten exercises the 1-result
// boundary implicitly.
// Wasmtime parallel: tests/all/component_model/func.rs many_results
// test.
func TestFlatABIExactlyMaxResults(t *testing.T) {
	ct := newBuilder().Finish()
	singleS32 := types.S32

	t.Run("FlattenCount_equals_1", func(t *testing.T) {
		require.Equal(t, 1, flattenCount(ct, singleS32))
	})

	t.Run("equals_MaxFlatResults", func(t *testing.T) {
		require.Equal(t, abi.MaxFlatResults, flattenCount(ct, singleS32))
	})

	t.Run("can_flatten_results", func(t *testing.T) {
		require.True(t, flattenCount(ct, singleS32) <= abi.MaxFlatResults)
	})
}

// TestFlatABIExceedsMaxResults asserts that a tuple of 2 s32 values
// exceeds MaxFlatResults and must spill to memory.
//
// Spec: definitions.py:1675-1681 flatten_functype sync-lift branch
// with len(flat_results) > 1 → flat_results = ['i32'] (retptr).
// Canonical test: run_tests.py::test_flatten covers the >1 spill.
// Wasmtime parallel: tests/all/component_model/func.rs many_results
// at result-count 2 and above.
func TestFlatABIExceedsMaxResults(t *testing.T) {
	b := newBuilder()
	tuple2 := b.InternTuple([]types.ValType{types.S32, types.S32})
	ct := b.Finish()

	t.Run("FlattenCount_equals_2", func(t *testing.T) {
		require.Equal(t, 2, flattenCount(ct, tuple2))
	})

	t.Run("exceeds_MaxFlatResults", func(t *testing.T) {
		require.True(t, flattenCount(ct, tuple2) > abi.MaxFlatResults)
	})

	t.Run("cannot_flatten_results", func(t *testing.T) {
		require.False(t, flattenCount(ct, tuple2) <= abi.MaxFlatResults)
	})
}

// TestFlatABIStringAlwaysSpillsResults asserts that a string (2 flat
// values: ptr, len) always spills to memory when used as a result.
//
// Spec: definitions.py:1712 StringType flattens to ['i32', 'i32']
// (count 2) — strictly > MAX_FLAT_RESULTS (1), so spills.
// Canonical test: run_tests.py::test_flatten covers string result
// spilling.
func TestFlatABIStringAlwaysSpillsResults(t *testing.T) {
	ct := newBuilder().Finish()
	stringType := types.String_

	t.Run("FlattenCount_equals_2", func(t *testing.T) {
		require.Equal(t, 2, flattenCount(ct, stringType))
	})

	t.Run("exceeds_MaxFlatResults", func(t *testing.T) {
		require.True(t, flattenCount(ct, stringType) > abi.MaxFlatResults)
	})

	t.Run("cannot_flatten_results", func(t *testing.T) {
		require.False(t, flattenCount(ct, stringType) <= abi.MaxFlatResults)
	})

	t.Run("can_flatten_params", func(t *testing.T) {
		require.True(t, flattenCount(ct, stringType) <= abi.MaxFlatParams)
	})

	t.Run("type_properties", func(t *testing.T) {
		abiInfo := stringType.ABI(ct)
		require.Equal(t, uint32(8), abiInfo.Size32)
		require.Equal(t, uint32(4), abiInfo.Align32)
	})
}

// TestFlatABIListAlwaysSpillsResults asserts that a list (2 flat
// values: ptr, len) always spills to memory when used as a result.
//
// Spec: definitions.py:1714 ListType(t, l) with l == None flattens
// to ['i32', 'i32'] (count 2) per flatten_list at :1721-1724.
// Canonical test: run_tests.py::test_flatten list result spilling.
func TestFlatABIListAlwaysSpillsResults(t *testing.T) {
	b := newBuilder()
	listType := b.InternList(types.S32)
	ct := b.Finish()

	t.Run("FlattenCount_equals_2", func(t *testing.T) {
		require.Equal(t, 2, flattenCount(ct, listType))
	})

	t.Run("exceeds_MaxFlatResults", func(t *testing.T) {
		require.True(t, flattenCount(ct, listType) > abi.MaxFlatResults)
	})

	t.Run("cannot_flatten_results", func(t *testing.T) {
		require.False(t, flattenCount(ct, listType) <= abi.MaxFlatResults)
	})

	t.Run("can_flatten_params", func(t *testing.T) {
		require.True(t, flattenCount(ct, listType) <= abi.MaxFlatParams)
	})

	t.Run("type_properties", func(t *testing.T) {
		abiInfo := listType.ABI(ct)
		require.Equal(t, uint32(8), abiInfo.Size32)
		require.Equal(t, uint32(4), abiInfo.Align32)
	})
}

// TestFlatABIComplexRecordSpill asserts that a record with 20 s32
// fields spills both as params (>16) and as results (>1).
//
// Spec: definitions.py:1726-1730 flatten_record sums FlattenCounts
// over all fields. Spec: definitions.py:1673-1681 boundary checks.
// Canonical test: run_tests.py::test_flatten covers multi-field
// record spilling.
func TestFlatABIComplexRecordSpill(t *testing.T) {
	b := newBuilder()
	fields := make([]types.RecordField, 20)
	for i := range fields {
		fields[i] = types.RecordField{Name: fmt.Sprintf("f%d", i), Type: types.S32}
	}
	record20 := b.InternRecord(fields)
	ct := b.Finish()

	t.Run("FlattenCount_equals_20", func(t *testing.T) {
		require.Equal(t, 20, flattenCount(ct, record20))
	})

	t.Run("exceeds_MaxFlatParams", func(t *testing.T) {
		require.True(t, flattenCount(ct, record20) > abi.MaxFlatParams)
	})

	t.Run("exceeds_MaxFlatResults", func(t *testing.T) {
		require.True(t, flattenCount(ct, record20) > abi.MaxFlatResults)
	})

	t.Run("cannot_flatten_params", func(t *testing.T) {
		require.False(t, flattenCount(ct, record20) <= abi.MaxFlatParams)
	})

	t.Run("cannot_flatten_results", func(t *testing.T) {
		require.False(t, flattenCount(ct, record20) <= abi.MaxFlatResults)
	})

	t.Run("type_properties", func(t *testing.T) {
		abiInfo := record20.ABI(ct)
		require.Equal(t, uint32(80), abiInfo.Size32)
		require.Equal(t, uint32(4), abiInfo.Align32)
	})
}

// TestFlatABIVariousCounts exercises FlattenCount for a broad set of
// primitive and composite type combinations, asserting canFlattenParams
// / canFlattenResults against the two thresholds.
//
// Spec: definitions.py:1703-1720 flatten_type dispatch table. Spec:
// definitions.py:1721-1730 flatten_list / flatten_record. Spec:
// definitions.py:1732-1741 flatten_variant.
// Canonical test: run_tests.py::test_flatten aggregates the same
// boundary assertions over a sample grid of types.
//
// Divergence (2): FlagsType flattens to a single i32 per the spec
// at :1717, but wazero implements wasmtime's FlagsSize::Size4Plus(n)
// multi-word encoding (n > 32 → ceil(n/32) i32s). This test encodes
// the divergence (flags(33)→2, flags(65)→3) which tracks the current
// wasmtime parity rather than the literal spec.
func TestFlatABIVariousCounts(t *testing.T) {
	b := newBuilder()

	tuple0 := b.InternTuple([]types.ValType{})
	tuple1S32 := b.InternTuple([]types.ValType{types.S32})
	tuple2S32 := b.InternTuple([]types.ValType{types.S32, types.S32})
	tuple2S64 := b.InternTuple([]types.ValType{types.S64, types.S64})

	listS32 := b.InternList(types.S32)

	optS32 := b.InternOption(types.S32)
	optS64 := b.InternOption(types.S64)

	resultOkErr := b.InternResult(types.S32, types.S32, true, true)
	resultErrOnly := b.InternResult(types.ValType{}, types.S32, false, true)
	resultOkOnly := b.InternResult(types.S32, types.ValType{}, true, false)

	enum2 := b.InternEnum([]string{"a", "b"})
	enum10 := b.InternEnum([]string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"})

	flags0 := b.InternFlags([]string{})
	flags1 := b.InternFlags([]string{"a"})
	flags8 := b.InternFlags([]string{"a", "b", "c", "d", "e", "f", "g", "h"})
	flags32Names := make([]string, 32)
	for i := range flags32Names {
		flags32Names[i] = fmt.Sprintf("f%d", i)
	}
	flags32 := b.InternFlags(flags32Names)
	flags33Names := make([]string, 33)
	for i := range flags33Names {
		flags33Names[i] = fmt.Sprintf("f%d", i)
	}
	flags33 := b.InternFlags(flags33Names)
	flags64Names := make([]string, 64)
	for i := range flags64Names {
		flags64Names[i] = fmt.Sprintf("f%d", i)
	}
	flags64 := b.InternFlags(flags64Names)
	flags65Names := make([]string, 65)
	for i := range flags65Names {
		flags65Names[i] = fmt.Sprintf("f%d", i)
	}
	flags65 := b.InternFlags(flags65Names)

	record0 := b.InternRecord([]types.RecordField{})
	record1 := b.InternRecord([]types.RecordField{{Name: "x", Type: types.S32}})
	record2 := b.InternRecord([]types.RecordField{
		{Name: "x", Type: types.S32},
		{Name: "y", Type: types.S32},
	})

	variantNoPayload := b.InternVariant([]types.VariantCase{
		{Name: "a"},
		{Name: "b"},
	})
	variantOneS32 := b.InternVariant([]types.VariantCase{
		{Name: "a"},
		{Name: "b", Payload: types.S32, HasPayload: true},
	})
	variantS32S64 := b.InternVariant([]types.VariantCase{
		{Name: "a", Payload: types.S32, HasPayload: true},
		{Name: "b", Payload: types.S64, HasPayload: true},
	})

	ct := b.Finish()

	testCases := []struct {
		name         string
		typ          types.ValType
		expectedFlat int
		canFlatParam bool
		canFlatRes   bool
	}{
		{"bool", types.Bool, 1, true, true},
		{"s8", types.S8, 1, true, true},
		{"u8", types.U8, 1, true, true},
		{"s16", types.S16, 1, true, true},
		{"u16", types.U16, 1, true, true},
		{"s32", types.S32, 1, true, true},
		{"u32", types.U32, 1, true, true},
		{"s64", types.S64, 1, true, true},
		{"u64", types.U64, 1, true, true},
		{"f32", types.F32, 1, true, true},
		{"f64", types.F64, 1, true, true},
		{"char", types.Char, 1, true, true},

		{"string", types.String_, 2, true, false},
		{"list<s32>", listS32, 2, true, false},

		{"tuple<>", tuple0, 0, true, true},
		{"tuple<s32>", tuple1S32, 1, true, true},
		{"tuple<s32, s32>", tuple2S32, 2, true, false},
		{"tuple<s64, s64>", tuple2S64, 2, true, false},

		{"option<s32>", optS32, 2, true, false},
		{"option<s64>", optS64, 2, true, false},

		{"result<s32, s32>", resultOkErr, 2, true, false},
		{"result<_, s32>", resultErrOnly, 2, true, false},
		{"result<s32, _>", resultOkOnly, 2, true, false},

		{"enum(2)", enum2, 1, true, true},
		{"enum(10)", enum10, 1, true, true},

		{"flags(0)", flags0, 0, true, true},
		{"flags(1)", flags1, 1, true, true},
		{"flags(8)", flags8, 1, true, true},
		{"flags(32)", flags32, 1, true, true},
		{"flags(33)", flags33, 2, true, false},
		{"flags(64)", flags64, 2, true, false},
		{"flags(65)", flags65, 3, true, false},

		{"record{}", record0, 0, true, true},
		{"record{s32}", record1, 1, true, true},
		{"record{s32,s32}", record2, 2, true, false},

		{"variant{a,b}", variantNoPayload, 1, true, true},
		{"variant{a,b(s32)}", variantOneS32, 2, true, false},
		{"variant{a(s32),b(s64)}", variantS32S64, 2, true, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fc := flattenCount(ct, tc.typ)
			t.Run("FlattenCount", func(t *testing.T) {
				require.Equal(t, tc.expectedFlat, fc)
			})
			t.Run("canFlattenParams", func(t *testing.T) {
				require.Equal(t, tc.canFlatParam, fc <= abi.MaxFlatParams)
			})
			t.Run("canFlattenResults", func(t *testing.T) {
				require.Equal(t, tc.canFlatRes, fc <= abi.MaxFlatResults)
			})
		})
	}
}

// TestFlatABIBoundaryParams asserts parameter flattening at exact
// boundaries: 15 (under), 16 (exactly at), 17 (just over).
//
// Spec: definitions.py:1673 strict > means 16 flattens, 17 spills.
// Canonical test: run_tests.py::test_flatten checks the boundary
// transition point.
func TestFlatABIBoundaryParams(t *testing.T) {
	for _, count := range []int{15, 16, 17} {
		t.Run(fmt.Sprintf("tuple_%d_s32", count), func(t *testing.T) {
			b := newBuilder()
			fields := make([]types.ValType, count)
			for i := range fields {
				fields[i] = types.S32
			}
			tuple := b.InternTuple(fields)
			ct := b.Finish()

			require.Equal(t, count, flattenCount(ct, tuple))

			canFlatten := flattenCount(ct, tuple) <= abi.MaxFlatParams
			if count <= 16 {
				require.True(t, canFlatten)
			} else {
				require.False(t, canFlatten)
			}
		})
	}
}

// TestFlatABIBoundaryResults asserts result flattening at exact
// boundaries: 0 (empty), 1 (single scalar), 2 (string/list/tuple-of-2),
// 3 (just over).
//
// Spec: definitions.py:1675 strict > means 1 flattens, 2 spills.
// Canonical test: run_tests.py::test_flatten checks result-side
// spill boundary.
func TestFlatABIBoundaryResults(t *testing.T) {
	b := newBuilder()
	tuple0 := b.InternTuple([]types.ValType{})
	tuple1 := b.InternTuple([]types.ValType{types.S32})
	tuple2 := b.InternTuple([]types.ValType{types.S32, types.S32})
	tuple3 := b.InternTuple([]types.ValType{types.S32, types.S32, types.S32})
	listS32 := b.InternList(types.S32)
	ct := b.Finish()

	testCases := []struct {
		name       string
		typ        types.ValType
		flatCount  int
		canFlatten bool
	}{
		{"tuple_0", tuple0, 0, true},
		{"s32", types.S32, 1, true},
		{"tuple_1_s32", tuple1, 1, true},
		{"tuple_2_s32", tuple2, 2, false},
		{"string", types.String_, 2, false},
		{"list", listS32, 2, false},
		{"tuple_3_s32", tuple3, 3, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.flatCount, flattenCount(ct, tc.typ))
			require.Equal(t, tc.canFlatten, flattenCount(ct, tc.typ) <= abi.MaxFlatResults)
		})
	}
}

// TestFlatABINestedTypes asserts that nested composite types
// correctly accumulate FlattenCount from their inner types.
//
// Spec: definitions.py:1726-1730 flatten_record accumulation. Spec:
// definitions.py:1732-1741 flatten_variant (disc + max payload).
// Canonical test: run_tests.py::test_flatten nested-type samples.
func TestFlatABINestedTypes(t *testing.T) {
	t.Run("nested_tuple", func(t *testing.T) {
		b := newBuilder()
		inner := b.InternTuple([]types.ValType{types.S32, types.S32})
		outer := b.InternTuple([]types.ValType{inner, types.S32})
		ct := b.Finish()

		require.Equal(t, 3, flattenCount(ct, outer))
		require.True(t, flattenCount(ct, outer) <= abi.MaxFlatParams)
		require.False(t, flattenCount(ct, outer) <= abi.MaxFlatResults)
	})

	t.Run("nested_record", func(t *testing.T) {
		b := newBuilder()
		inner := b.InternRecord([]types.RecordField{
			{Name: "x", Type: types.S32},
			{Name: "y", Type: types.S32},
		})
		outer := b.InternRecord([]types.RecordField{
			{Name: "inner", Type: inner},
			{Name: "z", Type: types.S32},
		})
		ct := b.Finish()

		require.Equal(t, 3, flattenCount(ct, outer))
	})

	t.Run("deeply_nested", func(t *testing.T) {
		b := newBuilder()
		level0 := b.InternTuple([]types.ValType{types.S32, types.S32})
		level1 := b.InternTuple([]types.ValType{level0, types.S32})
		level2 := b.InternTuple([]types.ValType{level1, types.S32})
		ct := b.Finish()

		require.Equal(t, 4, flattenCount(ct, level2))
		require.True(t, flattenCount(ct, level2) <= abi.MaxFlatParams)
		require.False(t, flattenCount(ct, level2) <= abi.MaxFlatResults)
	})

	t.Run("option_with_nested", func(t *testing.T) {
		b := newBuilder()
		innerTuple := b.InternTuple([]types.ValType{types.S32, types.S32})
		optionType := b.InternOption(innerTuple)
		ct := b.Finish()

		// option<tuple<s32,s32>> = disc(1) + max(tuple payload 2) = 3.
		require.Equal(t, 3, flattenCount(ct, optionType))
	})

	t.Run("result_with_nested", func(t *testing.T) {
		b := newBuilder()
		okType := b.InternTuple([]types.ValType{types.S32, types.S32})
		resultType := b.InternResult(okType, types.S32, true, true)
		ct := b.Finish()

		// result<tuple<s32,s32>, s32> = disc(1) + max(2, 1) = 3.
		require.Equal(t, 3, flattenCount(ct, resultType))
	})

	t.Run("variant_with_complex_cases", func(t *testing.T) {
		b := newBuilder()
		caseA := b.InternTuple([]types.ValType{types.S32, types.S32, types.S32})
		variant := b.InternVariant([]types.VariantCase{
			{Name: "a", Payload: caseA, HasPayload: true},
			{Name: "b", Payload: types.S32, HasPayload: true},
		})
		ct := b.Finish()

		// variant{ a(tuple<s32,s32,s32>), b(s32) } = disc(1) + max(3, 1) = 4.
		require.Equal(t, 4, flattenCount(ct, variant))
	})
}

// TestFlatABIManyParameters sweeps the parameter-count axis around
// the boundary, asserting the spill decision follows the
// flatten_functype gate.
//
// Spec: definitions.py:1673 if len(flat_params) > MAX_FLAT_PARAMS.
// Canonical test: run_tests.py::test_flatten many_parameters sweep.
// Wasmtime parallel: tests/all/component_model/func.rs many_parameters
// exercises the same boundary sweep at counts 1, 8, 15, 16, 17, 20,
// 32, 64.
func TestFlatABIManyParameters(t *testing.T) {
	for _, count := range []int{1, 8, 15, 16, 17, 20, 32, 64} {
		t.Run(fmt.Sprintf("params_%d", count), func(t *testing.T) {
			b := newBuilder()
			fields := make([]types.ValType, count)
			for i := range fields {
				fields[i] = types.S32
			}
			params := b.InternTuple(fields)
			ct := b.Finish()

			fc := flattenCount(ct, params)
			require.Equal(t, count, fc)

			shouldSpill := fc > abi.MaxFlatParams
			if count > 16 {
				require.True(t, shouldSpill)
			} else {
				require.False(t, shouldSpill)
			}
		})
	}
}

// TestFlatABIManyResults sweeps the result-count axis around the
// boundary, asserting the spill decision follows flatten_functype.
//
// Spec: definitions.py:1675 if len(flat_results) > MAX_FLAT_RESULTS.
// Canonical test: run_tests.py::test_flatten many_results sweep.
// Wasmtime parallel: tests/all/component_model/func.rs many_results
// exercises the same boundary sweep at result counts 0, 1, 2, 3, 4,
// 8, 16.
func TestFlatABIManyResults(t *testing.T) {
	for _, count := range []int{0, 1, 2, 3, 4, 8, 16} {
		t.Run(fmt.Sprintf("results_%d", count), func(t *testing.T) {
			b := newBuilder()
			fields := make([]types.ValType, count)
			for i := range fields {
				fields[i] = types.S32
			}
			results := b.InternTuple(fields)
			ct := b.Finish()

			fc := flattenCount(ct, results)
			require.Equal(t, count, fc)

			shouldSpill := fc > abi.MaxFlatResults
			if count > 1 {
				require.True(t, shouldSpill)
			} else {
				require.False(t, shouldSpill)
			}
		})
	}
}

// TestFlatABIHelperFunctions exercises the canFlatten{Params,Results}
// helper-function patterns used throughout the component-model
// implementation for deciding spill vs. register-pass.
//
// Spec: definitions.py:1672-1681 flatten_functype boundary logic;
// canFlatten{Params,Results} are the per-type projections of the
// same inequality.
// Canonical test: no direct run_tests.py case — these are
// implementation-pattern tests.
func TestFlatABIHelperFunctions(t *testing.T) {
	b := newBuilder()

	listS32 := b.InternList(types.S32)

	tuple16Fields := make([]types.ValType, 16)
	for i := range tuple16Fields {
		tuple16Fields[i] = types.S32
	}
	tuple16 := b.InternTuple(tuple16Fields)
	tuple17Fields := make([]types.ValType, 17)
	for i := range tuple17Fields {
		tuple17Fields[i] = types.S32
	}
	tuple17 := b.InternTuple(tuple17Fields)

	ct := b.Finish()

	canFlattenParams := func(v types.ValType) bool {
		return flattenCount(ct, v) <= abi.MaxFlatParams
	}
	canFlattenResults := func(v types.ValType) bool {
		return flattenCount(ct, v) <= abi.MaxFlatResults
	}

	t.Run("primitive_types", func(t *testing.T) {
		primitives := []types.ValType{
			types.Bool, types.S8, types.U8, types.S16, types.U16,
			types.S32, types.U32, types.S64, types.U64,
			types.F32, types.F64, types.Char,
		}
		for _, p := range primitives {
			require.True(t, canFlattenParams(p))
			require.True(t, canFlattenResults(p))
		}
	})

	t.Run("string_type", func(t *testing.T) {
		require.True(t, canFlattenParams(types.String_))
		require.False(t, canFlattenResults(types.String_))
	})

	t.Run("list_type", func(t *testing.T) {
		require.True(t, canFlattenParams(listS32))
		require.False(t, canFlattenResults(listS32))
	})

	t.Run("large_tuple_params", func(t *testing.T) {
		require.True(t, canFlattenParams(tuple16))
		require.False(t, canFlattenParams(tuple17))
	})
}

// TestFlatABISpecCompliance is a sanity check that the constants and
// primary metadata (string/list flatten counts) match the
// Component Model specification.
//
// Spec: definitions.py:1665 MAX_FLAT_PARAMS = 16, :1667
// MAX_FLAT_RESULTS = 1, :1712 StringType → ['i32','i32'], :1721-1724
// flatten_list (dynamic list) → ['i32','i32'].
// Canonical test: run_tests.py::test_flatten is the reference
// cross-check for these constants.
func TestFlatABISpecCompliance(t *testing.T) {
	b := newBuilder()
	listS32 := b.InternList(types.S32)
	ct := b.Finish()

	t.Run("spec_MaxFlatParams_is_16", func(t *testing.T) {
		require.Equal(t, 16, abi.MaxFlatParams)
	})

	t.Run("spec_MaxFlatResults_is_1", func(t *testing.T) {
		require.Equal(t, 1, abi.MaxFlatResults)
	})

	t.Run("spec_string_is_two_flat_values", func(t *testing.T) {
		require.Equal(t, 2, flattenCount(ct, types.String_))
	})

	t.Run("spec_list_is_two_flat_values", func(t *testing.T) {
		require.Equal(t, 2, flattenCount(ct, listS32))
	})
}
