// Package conformance contains conformance tests for the Component Model implementation.
// Flat ABI limit tests ported from wasmtime's tests/all/component_model/func.rs
// (many_parameters, many_results tests).
package conformance

import (
	"fmt"
	"testing"

	"github.com/tetratelabs/wazero/internal/component/abi"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestFlatABIConstants verifies that the flat ABI constants are correctly defined.
func TestFlatABIConstants(t *testing.T) {
	t.Run("MaxFlatParams_is_16", func(t *testing.T) {
		require.Equal(t, 16, abi.MaxFlatParams, "MaxFlatParams should be 16")
	})

	t.Run("MaxFlatResults_is_1", func(t *testing.T) {
		require.Equal(t, 1, abi.MaxFlatResults, "MaxFlatResults should be 1")
	})
}

// TestFlatABIExactlyMaxParams tests that a tuple of exactly MaxFlatParams (16) s32 values
// can be passed via flat representation (registers).
// Reference: wasmtime tests/all/component_model/func.rs many_parameters
func TestFlatABIExactlyMaxParams(t *testing.T) {
	// Create a tuple of 16 s32 values - exactly MaxFlatParams
	fields := make([]types.ValType, 16)
	for i := range fields {
		fields[i] = types.S32{}
	}
	tuple16 := types.Tuple{Types: fields}

	t.Run("FlattenCount_equals_16", func(t *testing.T) {
		require.Equal(t, 16, tuple16.FlattenCount(), "tuple of 16 s32 should have FlattenCount 16")
	})

	t.Run("equals_MaxFlatParams", func(t *testing.T) {
		require.Equal(t, abi.MaxFlatParams, tuple16.FlattenCount(),
			"tuple of 16 s32 should exactly equal MaxFlatParams")
	})

	t.Run("can_flatten_params", func(t *testing.T) {
		// A type with FlattenCount <= MaxFlatParams can be passed flat
		canFlatten := tuple16.FlattenCount() <= abi.MaxFlatParams
		require.True(t, canFlatten, "tuple of 16 s32 should be flattenable for params")
	})

	t.Run("type_properties", func(t *testing.T) {
		// Verify size and alignment
		// 16 s32s = 16 * 4 bytes = 64 bytes
		require.Equal(t, uint32(64), tuple16.Size(), "tuple of 16 s32 should have size 64")
		require.Equal(t, uint32(4), tuple16.Align(), "tuple of 16 s32 should have align 4")
	})
}

// TestFlatABIExceedsMaxParams tests that a tuple exceeding MaxFlatParams (17 s32 values)
// must spill to memory.
// Reference: wasmtime tests/all/component_model/func.rs many_parameters
func TestFlatABIExceedsMaxParams(t *testing.T) {
	// Create a tuple of 17 s32 values - exceeds MaxFlatParams
	fields := make([]types.ValType, 17)
	for i := range fields {
		fields[i] = types.S32{}
	}
	tuple17 := types.Tuple{Types: fields}

	t.Run("FlattenCount_equals_17", func(t *testing.T) {
		require.Equal(t, 17, tuple17.FlattenCount(), "tuple of 17 s32 should have FlattenCount 17")
	})

	t.Run("exceeds_MaxFlatParams", func(t *testing.T) {
		require.True(t, tuple17.FlattenCount() > abi.MaxFlatParams,
			"tuple of 17 s32 should exceed MaxFlatParams")
	})

	t.Run("cannot_flatten_params", func(t *testing.T) {
		// A type with FlattenCount > MaxFlatParams must spill to memory
		canFlatten := tuple17.FlattenCount() <= abi.MaxFlatParams
		require.False(t, canFlatten, "tuple of 17 s32 should NOT be flattenable for params")
	})

	t.Run("type_properties", func(t *testing.T) {
		// 17 s32s = 17 * 4 bytes = 68 bytes
		require.Equal(t, uint32(68), tuple17.Size(), "tuple of 17 s32 should have size 68")
		require.Equal(t, uint32(4), tuple17.Align(), "tuple of 17 s32 should have align 4")
	})
}

// TestFlatABIExactlyMaxResults tests that a single s32 value (exactly 1 flat result)
// can be returned via flat representation.
// Reference: wasmtime tests/all/component_model/func.rs many_results
func TestFlatABIExactlyMaxResults(t *testing.T) {
	singleS32 := types.S32{}

	t.Run("FlattenCount_equals_1", func(t *testing.T) {
		require.Equal(t, 1, singleS32.FlattenCount(), "s32 should have FlattenCount 1")
	})

	t.Run("equals_MaxFlatResults", func(t *testing.T) {
		require.Equal(t, abi.MaxFlatResults, singleS32.FlattenCount(),
			"s32 should exactly equal MaxFlatResults")
	})

	t.Run("can_flatten_results", func(t *testing.T) {
		// A type with FlattenCount <= MaxFlatResults can be returned flat
		canFlatten := singleS32.FlattenCount() <= abi.MaxFlatResults
		require.True(t, canFlatten, "s32 should be flattenable for results")
	})
}

// TestFlatABIExceedsMaxResults tests that a tuple of 2 s32 values (2 flat results)
// must spill to memory.
// Reference: wasmtime tests/all/component_model/func.rs many_results
func TestFlatABIExceedsMaxResults(t *testing.T) {
	// Create a tuple of 2 s32 values - exceeds MaxFlatResults
	tuple2 := types.Tuple{Types: []types.ValType{types.S32{}, types.S32{}}}

	t.Run("FlattenCount_equals_2", func(t *testing.T) {
		require.Equal(t, 2, tuple2.FlattenCount(), "tuple of 2 s32 should have FlattenCount 2")
	})

	t.Run("exceeds_MaxFlatResults", func(t *testing.T) {
		require.True(t, tuple2.FlattenCount() > abi.MaxFlatResults,
			"tuple of 2 s32 should exceed MaxFlatResults")
	})

	t.Run("cannot_flatten_results", func(t *testing.T) {
		// A type with FlattenCount > MaxFlatResults must spill to memory
		canFlatten := tuple2.FlattenCount() <= abi.MaxFlatResults
		require.False(t, canFlatten, "tuple of 2 s32 should NOT be flattenable for results")
	})
}

// TestFlatABIStringAlwaysSpillsResults tests that string (2 flat values: ptr, len)
// always spills to memory when used as a result type.
// String is [ptr: i32, len: i32] = 2 values, which exceeds MaxFlatResults (1).
func TestFlatABIStringAlwaysSpillsResults(t *testing.T) {
	stringType := types.String{}

	t.Run("FlattenCount_equals_2", func(t *testing.T) {
		require.Equal(t, 2, stringType.FlattenCount(), "string should have FlattenCount 2 (ptr, len)")
	})

	t.Run("exceeds_MaxFlatResults", func(t *testing.T) {
		require.True(t, stringType.FlattenCount() > abi.MaxFlatResults,
			"string FlattenCount should exceed MaxFlatResults")
	})

	t.Run("cannot_flatten_results", func(t *testing.T) {
		canFlatten := stringType.FlattenCount() <= abi.MaxFlatResults
		require.False(t, canFlatten, "string should NOT be flattenable for results")
	})

	t.Run("can_flatten_params", func(t *testing.T) {
		// String CAN be passed as params (2 <= 16)
		canFlatten := stringType.FlattenCount() <= abi.MaxFlatParams
		require.True(t, canFlatten, "string should be flattenable for params")
	})

	t.Run("type_properties", func(t *testing.T) {
		require.Equal(t, uint32(8), stringType.Size(), "string should have size 8 (ptr + len)")
		require.Equal(t, uint32(4), stringType.Align(), "string should have align 4")
	})
}

// TestFlatABIListAlwaysSpillsResults tests that list (2 flat values: ptr, len)
// always spills to memory when used as a result type.
// List is [ptr: i32, len: i32] = 2 values, which exceeds MaxFlatResults (1).
func TestFlatABIListAlwaysSpillsResults(t *testing.T) {
	listType := types.List{Element: types.S32{}}

	t.Run("FlattenCount_equals_2", func(t *testing.T) {
		require.Equal(t, 2, listType.FlattenCount(), "list should have FlattenCount 2 (ptr, len)")
	})

	t.Run("exceeds_MaxFlatResults", func(t *testing.T) {
		require.True(t, listType.FlattenCount() > abi.MaxFlatResults,
			"list FlattenCount should exceed MaxFlatResults")
	})

	t.Run("cannot_flatten_results", func(t *testing.T) {
		canFlatten := listType.FlattenCount() <= abi.MaxFlatResults
		require.False(t, canFlatten, "list should NOT be flattenable for results")
	})

	t.Run("can_flatten_params", func(t *testing.T) {
		// List CAN be passed as params (2 <= 16)
		canFlatten := listType.FlattenCount() <= abi.MaxFlatParams
		require.True(t, canFlatten, "list should be flattenable for params")
	})

	t.Run("type_properties", func(t *testing.T) {
		require.Equal(t, uint32(8), listType.Size(), "list should have size 8 (ptr + len)")
		require.Equal(t, uint32(4), listType.Align(), "list should have align 4")
	})
}

// TestFlatABIComplexRecordSpill tests that a record with 20 fields
// should spill both params and results.
// Reference: wasmtime tests/all/component_model/func.rs complex spill scenarios
func TestFlatABIComplexRecordSpill(t *testing.T) {
	// Create a record with 20 s32 fields
	fields := make([]types.Field, 20)
	for i := range fields {
		fields[i] = types.Field{Name: fmt.Sprintf("f%d", i), Type: types.S32{}}
	}
	record20 := types.Record{Fields: fields}

	t.Run("FlattenCount_equals_20", func(t *testing.T) {
		require.Equal(t, 20, record20.FlattenCount(), "record with 20 s32 fields should have FlattenCount 20")
	})

	t.Run("exceeds_MaxFlatParams", func(t *testing.T) {
		require.True(t, record20.FlattenCount() > abi.MaxFlatParams,
			"record with 20 fields should exceed MaxFlatParams")
	})

	t.Run("exceeds_MaxFlatResults", func(t *testing.T) {
		require.True(t, record20.FlattenCount() > abi.MaxFlatResults,
			"record with 20 fields should exceed MaxFlatResults")
	})

	t.Run("cannot_flatten_params", func(t *testing.T) {
		canFlatten := record20.FlattenCount() <= abi.MaxFlatParams
		require.False(t, canFlatten, "record with 20 fields should NOT be flattenable for params")
	})

	t.Run("cannot_flatten_results", func(t *testing.T) {
		canFlatten := record20.FlattenCount() <= abi.MaxFlatResults
		require.False(t, canFlatten, "record with 20 fields should NOT be flattenable for results")
	})

	t.Run("type_properties", func(t *testing.T) {
		// 20 s32s = 20 * 4 bytes = 80 bytes
		require.Equal(t, uint32(80), record20.Size(), "record with 20 s32 should have size 80")
		require.Equal(t, uint32(4), record20.Align(), "record with 20 s32 should have align 4")
	})
}

// TestFlatABIVariousCounts tests FlattenCount for various type combinations
// to verify correct flat counting behavior.
func TestFlatABIVariousCounts(t *testing.T) {
	testCases := []struct {
		name         string
		typ          types.ValType
		expectedFlat int
		canFlatParam bool  // flatCount <= MaxFlatParams
		canFlatRes   bool  // flatCount <= MaxFlatResults
	}{
		// Primitives - all have FlattenCount 1
		{"bool", types.Bool{}, 1, true, true},
		{"s8", types.S8{}, 1, true, true},
		{"u8", types.U8{}, 1, true, true},
		{"s16", types.S16{}, 1, true, true},
		{"u16", types.U16{}, 1, true, true},
		{"s32", types.S32{}, 1, true, true},
		{"u32", types.U32{}, 1, true, true},
		{"s64", types.S64{}, 1, true, true},
		{"u64", types.U64{}, 1, true, true},
		{"f32", types.F32{}, 1, true, true},
		{"f64", types.F64{}, 1, true, true},
		{"char", types.Char{}, 1, true, true},

		// String and List - 2 values (ptr + len)
		{"string", types.String{}, 2, true, false},
		{"list<s32>", types.List{Element: types.S32{}}, 2, true, false},

		// Tuples
		{"tuple<>", types.Tuple{Types: []types.ValType{}}, 0, true, true},
		{"tuple<s32>", types.Tuple{Types: []types.ValType{types.S32{}}}, 1, true, true},
		{"tuple<s32, s32>", types.Tuple{Types: []types.ValType{types.S32{}, types.S32{}}}, 2, true, false},
		{"tuple<s64, s64>", types.Tuple{Types: []types.ValType{types.S64{}, types.S64{}}}, 2, true, false},

		// Option - discriminant (1) + max payload
		{"option<s32>", types.Option{Some: types.S32{}}, 2, true, false},
		{"option<s64>", types.Option{Some: types.S64{}}, 2, true, false},

		// Result - discriminant (1) + max(ok, error) payload
		{"result<s32, s32>", types.Result{Ok: types.S32{}, Error: types.S32{}}, 2, true, false},
		{"result<_, s32>", types.Result{Ok: nil, Error: types.S32{}}, 2, true, false},
		{"result<s32, _>", types.Result{Ok: types.S32{}, Error: nil}, 2, true, false},

		// Enum - just discriminant
		{"enum(2)", types.Enum{Cases: []string{"a", "b"}}, 1, true, true},
		{"enum(10)", types.Enum{Cases: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}}, 1, true, true},

		// Flags - depends on number of flags
		{"flags(0)", types.Flags{Names: []string{}}, 0, true, true},
		{"flags(1)", types.Flags{Names: []string{"a"}}, 1, true, true},
		{"flags(8)", types.Flags{Names: []string{"a", "b", "c", "d", "e", "f", "g", "h"}}, 1, true, true},
		{"flags(32)", types.Flags{Names: make([]string, 32)}, 1, true, true},
		{"flags(33)", types.Flags{Names: make([]string, 33)}, 2, true, false}, // 2 i32s needed
		{"flags(64)", types.Flags{Names: make([]string, 64)}, 2, true, false},
		{"flags(65)", types.Flags{Names: make([]string, 65)}, 3, true, false},

		// Records
		{"record{}", types.Record{Fields: []types.Field{}}, 0, true, true},
		{"record{s32}", types.Record{Fields: []types.Field{{Name: "x", Type: types.S32{}}}}, 1, true, true},
		{"record{s32,s32}", types.Record{Fields: []types.Field{
			{Name: "x", Type: types.S32{}},
			{Name: "y", Type: types.S32{}},
		}}, 2, true, false},

		// Variant - discriminant (1) + max payload
		{"variant{a,b}", types.Variant{Cases: []types.Case{
			{Name: "a", Type: nil},
			{Name: "b", Type: nil},
		}}, 1, true, true},
		{"variant{a,b(s32)}", types.Variant{Cases: []types.Case{
			{Name: "a", Type: nil},
			{Name: "b", Type: types.S32{}},
		}}, 2, true, false},
		{"variant{a(s32),b(s64)}", types.Variant{Cases: []types.Case{
			{Name: "a", Type: types.S32{}},
			{Name: "b", Type: types.S64{}},
		}}, 2, true, false}, // disc + max(s32=1, s64=1) = 2
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("FlattenCount", func(t *testing.T) {
				require.Equal(t, tc.expectedFlat, tc.typ.FlattenCount(),
					"%s should have FlattenCount %d", tc.name, tc.expectedFlat)
			})

			t.Run("canFlattenParams", func(t *testing.T) {
				canFlatten := tc.typ.FlattenCount() <= abi.MaxFlatParams
				require.Equal(t, tc.canFlatParam, canFlatten,
					"%s canFlattenParams should be %v", tc.name, tc.canFlatParam)
			})

			t.Run("canFlattenResults", func(t *testing.T) {
				canFlatten := tc.typ.FlattenCount() <= abi.MaxFlatResults
				require.Equal(t, tc.canFlatRes, canFlatten,
					"%s canFlattenResults should be %v", tc.name, tc.canFlatRes)
			})
		})
	}
}

// TestFlatABIBoundaryParams tests parameter flattening at exact boundaries.
func TestFlatABIBoundaryParams(t *testing.T) {
	// Test tuples at boundary: 15, 16, 17 s32 values
	for _, count := range []int{15, 16, 17} {
		t.Run(fmt.Sprintf("tuple_%d_s32", count), func(t *testing.T) {
			fields := make([]types.ValType, count)
			for i := range fields {
				fields[i] = types.S32{}
			}
			tuple := types.Tuple{Types: fields}

			require.Equal(t, count, tuple.FlattenCount())

			canFlatten := tuple.FlattenCount() <= abi.MaxFlatParams
			if count <= 16 {
				require.True(t, canFlatten, "tuple of %d s32 should be flattenable for params", count)
			} else {
				require.False(t, canFlatten, "tuple of %d s32 should NOT be flattenable for params", count)
			}
		})
	}
}

// TestFlatABIBoundaryResults tests result flattening at exact boundaries.
func TestFlatABIBoundaryResults(t *testing.T) {
	testCases := []struct {
		name       string
		typ        types.ValType
		flatCount  int
		canFlatten bool
	}{
		// Exactly 0 - flattenable
		{"tuple_0", types.Tuple{Types: []types.ValType{}}, 0, true},

		// Exactly 1 - flattenable
		{"s32", types.S32{}, 1, true},
		{"tuple_1_s32", types.Tuple{Types: []types.ValType{types.S32{}}}, 1, true},

		// Exactly 2 - NOT flattenable (exceeds MaxFlatResults = 1)
		{"tuple_2_s32", types.Tuple{Types: []types.ValType{types.S32{}, types.S32{}}}, 2, false},
		{"string", types.String{}, 2, false},
		{"list", types.List{Element: types.S32{}}, 2, false},

		// More than 2 - NOT flattenable
		{"tuple_3_s32", types.Tuple{Types: []types.ValType{types.S32{}, types.S32{}, types.S32{}}}, 3, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.flatCount, tc.typ.FlattenCount())
			canFlatten := tc.typ.FlattenCount() <= abi.MaxFlatResults
			require.Equal(t, tc.canFlatten, canFlatten,
				"type %s with FlattenCount %d should have canFlattenResults=%v",
				tc.name, tc.flatCount, tc.canFlatten)
		})
	}
}

// TestFlatABINestedTypes tests FlattenCount for nested composite types.
func TestFlatABINestedTypes(t *testing.T) {
	t.Run("nested_tuple", func(t *testing.T) {
		// tuple<tuple<s32, s32>, s32> = 3 flat values
		inner := types.Tuple{Types: []types.ValType{types.S32{}, types.S32{}}}
		outer := types.Tuple{Types: []types.ValType{inner, types.S32{}}}

		require.Equal(t, 3, outer.FlattenCount())
		require.True(t, outer.FlattenCount() <= abi.MaxFlatParams)
		require.False(t, outer.FlattenCount() <= abi.MaxFlatResults)
	})

	t.Run("nested_record", func(t *testing.T) {
		// record { inner: record { x: s32, y: s32 }, z: s32 } = 3 flat values
		inner := types.Record{Fields: []types.Field{
			{Name: "x", Type: types.S32{}},
			{Name: "y", Type: types.S32{}},
		}}
		outer := types.Record{Fields: []types.Field{
			{Name: "inner", Type: inner},
			{Name: "z", Type: types.S32{}},
		}}

		require.Equal(t, 3, outer.FlattenCount())
	})

	t.Run("deeply_nested", func(t *testing.T) {
		// Build a deeply nested structure: tuple<tuple<tuple<s32, s32>, s32>, s32> = 4 flat
		level0 := types.Tuple{Types: []types.ValType{types.S32{}, types.S32{}}} // 2
		level1 := types.Tuple{Types: []types.ValType{level0, types.S32{}}}      // 3
		level2 := types.Tuple{Types: []types.ValType{level1, types.S32{}}}      // 4

		require.Equal(t, 4, level2.FlattenCount())
		require.True(t, level2.FlattenCount() <= abi.MaxFlatParams)
		require.False(t, level2.FlattenCount() <= abi.MaxFlatResults)
	})

	t.Run("option_with_nested", func(t *testing.T) {
		// option<tuple<s32, s32>> = 1 (disc) + 2 (tuple) = 3 flat
		innerTuple := types.Tuple{Types: []types.ValType{types.S32{}, types.S32{}}}
		optionType := types.Option{Some: innerTuple}

		require.Equal(t, 3, optionType.FlattenCount())
	})

	t.Run("result_with_nested", func(t *testing.T) {
		// result<tuple<s32, s32>, s32> = 1 (disc) + max(2, 1) = 3 flat
		okType := types.Tuple{Types: []types.ValType{types.S32{}, types.S32{}}}
		resultType := types.Result{Ok: okType, Error: types.S32{}}

		require.Equal(t, 3, resultType.FlattenCount())
	})

	t.Run("variant_with_complex_cases", func(t *testing.T) {
		// variant { a(tuple<s32,s32,s32>), b(s32) } = 1 + max(3, 1) = 4
		caseA := types.Tuple{Types: []types.ValType{types.S32{}, types.S32{}, types.S32{}}}
		variant := types.Variant{Cases: []types.Case{
			{Name: "a", Type: caseA},
			{Name: "b", Type: types.S32{}},
		}}

		require.Equal(t, 4, variant.FlattenCount())
	})
}

// TestFlatABIManyParameters tests the many_parameters pattern from wasmtime.
// This verifies parameter spilling behavior when exceeding MaxFlatParams.
func TestFlatABIManyParameters(t *testing.T) {
	// Test various parameter counts around the boundary
	for _, count := range []int{1, 8, 15, 16, 17, 20, 32, 64} {
		t.Run(fmt.Sprintf("params_%d", count), func(t *testing.T) {
			fields := make([]types.ValType, count)
			for i := range fields {
				fields[i] = types.S32{}
			}
			params := types.Tuple{Types: fields}

			flatCount := params.FlattenCount()
			require.Equal(t, count, flatCount, "FlattenCount should equal number of s32 params")

			shouldSpill := flatCount > abi.MaxFlatParams
			if count > 16 {
				require.True(t, shouldSpill,
					"params with %d s32 should spill to memory", count)
			} else {
				require.False(t, shouldSpill,
					"params with %d s32 should NOT spill to memory", count)
			}
		})
	}
}

// TestFlatABIManyResults tests the many_results pattern from wasmtime.
// This verifies result spilling behavior when exceeding MaxFlatResults.
func TestFlatABIManyResults(t *testing.T) {
	// Test various result counts around the boundary
	for _, count := range []int{0, 1, 2, 3, 4, 8, 16} {
		t.Run(fmt.Sprintf("results_%d", count), func(t *testing.T) {
			fields := make([]types.ValType, count)
			for i := range fields {
				fields[i] = types.S32{}
			}
			results := types.Tuple{Types: fields}

			flatCount := results.FlattenCount()
			require.Equal(t, count, flatCount, "FlattenCount should equal number of s32 results")

			shouldSpill := flatCount > abi.MaxFlatResults
			if count > 1 {
				require.True(t, shouldSpill,
					"results with %d s32 should spill to memory", count)
			} else {
				require.False(t, shouldSpill,
					"results with %d s32 should NOT spill to memory", count)
			}
		})
	}
}

// TestFlatABIHelperFunctions tests helper function patterns for checking flattenability.
// These patterns are used throughout the component model implementation.
func TestFlatABIHelperFunctions(t *testing.T) {
	// Helper function pattern: CanFlattenParams
	canFlattenParams := func(typ types.ValType) bool {
		return typ.FlattenCount() <= abi.MaxFlatParams
	}

	// Helper function pattern: CanFlattenResults
	canFlattenResults := func(typ types.ValType) bool {
		return typ.FlattenCount() <= abi.MaxFlatResults
	}

	t.Run("primitive_types", func(t *testing.T) {
		// All primitives should flatten both ways
		primitives := []types.ValType{
			types.Bool{}, types.S8{}, types.U8{}, types.S16{}, types.U16{},
			types.S32{}, types.U32{}, types.S64{}, types.U64{},
			types.F32{}, types.F64{}, types.Char{},
		}
		for _, p := range primitives {
			require.True(t, canFlattenParams(p), "primitive should flatten for params")
			require.True(t, canFlattenResults(p), "primitive should flatten for results")
		}
	})

	t.Run("string_type", func(t *testing.T) {
		strType := types.String{}
		require.True(t, canFlattenParams(strType), "string should flatten for params")
		require.False(t, canFlattenResults(strType), "string should NOT flatten for results")
	})

	t.Run("list_type", func(t *testing.T) {
		listType := types.List{Element: types.S32{}}
		require.True(t, canFlattenParams(listType), "list should flatten for params")
		require.False(t, canFlattenResults(listType), "list should NOT flatten for results")
	})

	t.Run("large_tuple_params", func(t *testing.T) {
		// 16 params - should flatten
		tuple16 := types.Tuple{Types: make([]types.ValType, 16)}
		for i := range tuple16.Types {
			tuple16.Types[i] = types.S32{}
		}
		require.True(t, canFlattenParams(tuple16), "tuple<16 x s32> should flatten for params")

		// 17 params - should NOT flatten
		tuple17 := types.Tuple{Types: make([]types.ValType, 17)}
		for i := range tuple17.Types {
			tuple17.Types[i] = types.S32{}
		}
		require.False(t, canFlattenParams(tuple17), "tuple<17 x s32> should NOT flatten for params")
	})
}

// TestFlatABISpecCompliance verifies compliance with Component Model specification
// for flat ABI limits.
func TestFlatABISpecCompliance(t *testing.T) {
	t.Run("spec_MaxFlatParams_is_16", func(t *testing.T) {
		// Per Component Model spec: MAX_FLAT_PARAMS = 16
		require.Equal(t, 16, abi.MaxFlatParams,
			"MaxFlatParams must be 16 per Component Model spec")
	})

	t.Run("spec_MaxFlatResults_is_1", func(t *testing.T) {
		// Per Component Model spec: MAX_FLAT_RESULTS = 1 (for sync calls)
		require.Equal(t, 1, abi.MaxFlatResults,
			"MaxFlatResults must be 1 per Component Model spec (sync)")
	})

	t.Run("spec_string_is_two_flat_values", func(t *testing.T) {
		// Per spec: string flattens to (i32, i32) for ptr and byte length
		strType := types.String{}
		require.Equal(t, 2, strType.FlattenCount(),
			"string must flatten to 2 values (ptr, len) per spec")
	})

	t.Run("spec_list_is_two_flat_values", func(t *testing.T) {
		// Per spec: list<T> flattens to (i32, i32) for ptr and element count
		listType := types.List{Element: types.S32{}}
		require.Equal(t, 2, listType.FlattenCount(),
			"list must flatten to 2 values (ptr, len) per spec")
	})
}
