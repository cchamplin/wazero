// Package conformance contains conformance tests for the Component Model implementation.
// Instance type handling tests verify that all result types work correctly through
// LowerFlat/LiftFlat roundtrips.
package conformance

import (
	"math"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/abi"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestInstanceResultTypes verifies that all result types work correctly
// (not just s32) through LowerFlat/LiftFlat roundtrips.
// This is Task 266: Instance Result Type Handling.
func TestInstanceResultTypes(t *testing.T) {
	t.Run("s64_result", func(t *testing.T) {
		tests := []struct {
			name  string
			value int64
		}{
			{"zero", 0},
			{"positive", 9000000000000},
			{"negative", -9000000000000},
			{"min", math.MinInt64},
			{"max", math.MaxInt64},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				val := types.ValS64(tc.value)
				flat, err := abi.LowerFlat(nil, types.S64{}, val)
				require.NoError(t, err)
				require.Equal(t, 1, len(flat))

				iter := abi.NewFlatIter(flat)
				lifted, err := abi.LiftFlat(nil, types.S64{}, iter)
				require.NoError(t, err)
				require.Equal(t, tc.value, lifted.S64())
			})
		}
	})

	t.Run("u64_result", func(t *testing.T) {
		tests := []struct {
			name  string
			value uint64
		}{
			{"zero", 0},
			{"mid", 0x8000000000000000},
			{"max", math.MaxUint64},
			{"pattern", 0xDEADBEEF12345678},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				val := types.ValU64(tc.value)
				flat, err := abi.LowerFlat(nil, types.U64{}, val)
				require.NoError(t, err)
				require.Equal(t, 1, len(flat))

				iter := abi.NewFlatIter(flat)
				lifted, err := abi.LiftFlat(nil, types.U64{}, iter)
				require.NoError(t, err)
				require.Equal(t, tc.value, lifted.U64())
			})
		}
	})

	t.Run("f32_result", func(t *testing.T) {
		tests := []struct {
			name  string
			value float32
		}{
			{"zero", 0.0},
			{"one", 1.0},
			{"pi", 3.14159},
			{"max", math.MaxFloat32},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				val := types.ValF32(tc.value)
				flat, err := abi.LowerFlat(nil, types.F32{}, val)
				require.NoError(t, err)
				require.Equal(t, 1, len(flat))

				iter := abi.NewFlatIter(flat)
				lifted, err := abi.LiftFlat(nil, types.F32{}, iter)
				require.NoError(t, err)
				require.Equal(t, math.Float32bits(tc.value), math.Float32bits(lifted.F32()))
			})
		}

		t.Run("infinity", func(t *testing.T) {
			val := types.ValF32(float32(math.Inf(1)))
			flat, err := abi.LowerFlat(nil, types.F32{}, val)
			require.NoError(t, err)

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, types.F32{}, iter)
			require.NoError(t, err)
			require.True(t, math.IsInf(float64(lifted.F32()), 1))
		})

		t.Run("nan", func(t *testing.T) {
			val := types.ValF32(float32(math.NaN()))
			flat, err := abi.LowerFlat(nil, types.F32{}, val)
			require.NoError(t, err)

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, types.F32{}, iter)
			require.NoError(t, err)
			require.True(t, math.IsNaN(float64(lifted.F32())))
		})
	})

	t.Run("f64_result", func(t *testing.T) {
		tests := []struct {
			name  string
			value float64
		}{
			{"zero", 0.0},
			{"one", 1.0},
			{"pi", 3.14159265358979323846},
			{"max", math.MaxFloat64},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				val := types.ValF64(tc.value)
				flat, err := abi.LowerFlat(nil, types.F64{}, val)
				require.NoError(t, err)
				require.Equal(t, 1, len(flat))

				iter := abi.NewFlatIter(flat)
				lifted, err := abi.LiftFlat(nil, types.F64{}, iter)
				require.NoError(t, err)
				require.Equal(t, math.Float64bits(tc.value), math.Float64bits(lifted.F64()))
			})
		}

		t.Run("infinity", func(t *testing.T) {
			val := types.ValF64(math.Inf(1))
			flat, err := abi.LowerFlat(nil, types.F64{}, val)
			require.NoError(t, err)

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, types.F64{}, iter)
			require.NoError(t, err)
			require.True(t, math.IsInf(lifted.F64(), 1))
		})

		t.Run("nan", func(t *testing.T) {
			val := types.ValF64(math.NaN())
			flat, err := abi.LowerFlat(nil, types.F64{}, val)
			require.NoError(t, err)

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, types.F64{}, iter)
			require.NoError(t, err)
			require.True(t, math.IsNaN(lifted.F64()))
		})
	})

	t.Run("bool_result", func(t *testing.T) {
		tests := []struct {
			name  string
			value bool
		}{
			{"true", true},
			{"false", false},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				val := types.ValBool(tc.value)
				flat, err := abi.LowerFlat(nil, types.Bool{}, val)
				require.NoError(t, err)
				require.Equal(t, 1, len(flat))

				iter := abi.NewFlatIter(flat)
				lifted, err := abi.LiftFlat(nil, types.Bool{}, iter)
				require.NoError(t, err)
				require.Equal(t, tc.value, lifted.Bool())
			})
		}
	})
}

// TestRecordFieldNames verifies that records work with arbitrary field names
// (not just "x" and "y"). This is Task 267.
func TestRecordFieldNames(t *testing.T) {
	t.Run("arbitrary_field_names", func(t *testing.T) {
		recordType := types.Record{
			Fields: []types.Field{
				{Name: "first-name", Type: types.S32{}},
				{Name: "lastName", Type: types.S32{}},
				{Name: "age_value", Type: types.S32{}},
			},
		}

		val := types.ValRecord(map[string]types.Val{
			"first-name": types.ValS32(100),
			"lastName":   types.ValS32(200),
			"age_value":  types.ValS32(30),
		})

		flat, err := abi.LowerFlat(nil, recordType, val)
		require.NoError(t, err)
		require.Equal(t, 3, len(flat))

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, recordType, iter)
		require.NoError(t, err)

		firstName, ok := lifted.RecordField("first-name")
		require.True(t, ok)
		require.Equal(t, int32(100), firstName.S32())

		lastName, ok := lifted.RecordField("lastName")
		require.True(t, ok)
		require.Equal(t, int32(200), lastName.S32())

		ageValue, ok := lifted.RecordField("age_value")
		require.True(t, ok)
		require.Equal(t, int32(30), ageValue.S32())
	})

	t.Run("wit_style_names", func(t *testing.T) {
		// WIT allows kebab-case names
		recordType := types.Record{
			Fields: []types.Field{
				{Name: "http-status", Type: types.U16{}},
				{Name: "content-type", Type: types.S32{}},
				{Name: "is-valid", Type: types.Bool{}},
			},
		}

		val := types.ValRecord(map[string]types.Val{
			"http-status":  types.ValU16(200),
			"content-type": types.ValS32(1),
			"is-valid":     types.ValBool(true),
		})

		flat, err := abi.LowerFlat(nil, recordType, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, recordType, iter)
		require.NoError(t, err)

		httpStatus, ok := lifted.RecordField("http-status")
		require.True(t, ok)
		require.Equal(t, uint16(200), httpStatus.U16())

		contentType, ok := lifted.RecordField("content-type")
		require.True(t, ok)
		require.Equal(t, int32(1), contentType.S32())

		isValid, ok := lifted.RecordField("is-valid")
		require.True(t, ok)
		require.True(t, isValid.Bool())
	})

	t.Run("single_letter_names", func(t *testing.T) {
		recordType := types.Record{
			Fields: []types.Field{
				{Name: "a", Type: types.S32{}},
				{Name: "b", Type: types.S32{}},
				{Name: "c", Type: types.S32{}},
			},
		}

		val := types.ValRecord(map[string]types.Val{
			"a": types.ValS32(1),
			"b": types.ValS32(2),
			"c": types.ValS32(3),
		})

		flat, err := abi.LowerFlat(nil, recordType, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, recordType, iter)
		require.NoError(t, err)

		aVal, _ := lifted.RecordField("a")
		require.Equal(t, int32(1), aVal.S32())
		bVal, _ := lifted.RecordField("b")
		require.Equal(t, int32(2), bVal.S32())
		cVal, _ := lifted.RecordField("c")
		require.Equal(t, int32(3), cVal.S32())
	})

	t.Run("long_field_names", func(t *testing.T) {
		recordType := types.Record{
			Fields: []types.Field{
				{Name: "this-is-a-very-long-field-name-for-testing", Type: types.S32{}},
				{Name: "another-extremely-long-field-name-value", Type: types.U64{}},
			},
		}

		val := types.ValRecord(map[string]types.Val{
			"this-is-a-very-long-field-name-for-testing": types.ValS32(42),
			"another-extremely-long-field-name-value":    types.ValU64(12345678901234),
		})

		flat, err := abi.LowerFlat(nil, recordType, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, recordType, iter)
		require.NoError(t, err)

		longField, ok := lifted.RecordField("this-is-a-very-long-field-name-for-testing")
		require.True(t, ok)
		require.Equal(t, int32(42), longField.S32())

		anotherLong, ok := lifted.RecordField("another-extremely-long-field-name-value")
		require.True(t, ok)
		require.Equal(t, uint64(12345678901234), anotherLong.U64())
	})
}

// TestListElementEdgeCases tests list elements of various types
// and edge cases like empty lists and single-element lists.
// This is Task 270.
func TestListElementEdgeCases(t *testing.T) {
	t.Run("empty_list", func(t *testing.T) {
		listType := types.List{Element: types.S32{}}
		val := types.ValList([]types.Val{})

		flat, err := abi.LowerFlat(nil, listType, val)
		require.NoError(t, err)
		require.Equal(t, 2, len(flat), "empty list should still have ptr+len")
		require.Equal(t, uint64(0), flat[0], "empty list ptr should be 0")
		require.Equal(t, uint64(0), flat[1], "empty list len should be 0")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, listType, iter)
		require.NoError(t, err)
		require.Equal(t, 0, len(lifted.List()))
	})

	t.Run("single_element_s32", func(t *testing.T) {
		listType := types.List{Element: types.S32{}}

		mem := newMockMemory(1024)
		allocPtr := uint32(256)
		ctx := &abi.LowerContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
			Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
				result := allocPtr
				allocPtr += newSize
				return result, nil
			},
		}

		val := types.ValList([]types.Val{
			types.ValS32(42),
		})

		flat, err := abi.LowerFlat(ctx, listType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(256), flat[0], "ptr")
		require.Equal(t, uint64(1), flat[1], "len")
	})

	t.Run("list_of_bools", func(t *testing.T) {
		listType := types.List{Element: types.Bool{}}

		mem := newMockMemory(1024)
		allocPtr := uint32(256)
		ctx := &abi.LowerContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
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

		flat, err := abi.LowerFlat(ctx, listType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(3), flat[1], "should have 3 elements")
	})

	t.Run("list_of_u64", func(t *testing.T) {
		listType := types.List{Element: types.U64{}}

		mem := newMockMemory(1024)
		allocPtr := uint32(256)
		ctx := &abi.LowerContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
			Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
				result := allocPtr
				allocPtr += newSize
				return result, nil
			},
		}

		val := types.ValList([]types.Val{
			types.ValU64(math.MaxUint64),
			types.ValU64(0),
		})

		flat, err := abi.LowerFlat(ctx, listType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(2), flat[1], "should have 2 elements")
	})

	t.Run("list_of_f32", func(t *testing.T) {
		listType := types.List{Element: types.F32{}}

		mem := newMockMemory(1024)
		allocPtr := uint32(256)
		ctx := &abi.LowerContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
			Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
				result := allocPtr
				allocPtr += newSize
				return result, nil
			},
		}

		val := types.ValList([]types.Val{
			types.ValF32(3.14159),
			types.ValF32(-2.71828),
		})

		flat, err := abi.LowerFlat(ctx, listType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(2), flat[1], "should have 2 elements")
	})

	t.Run("list_type_properties", func(t *testing.T) {
		// Verify list properties for various element types
		tests := []struct {
			name       string
			elemType   types.ValType
			elemSize   uint32
			elemAlign  uint32
			flattenCnt int
		}{
			{"list<bool>", types.Bool{}, 1, 1, 2},
			{"list<u8>", types.U8{}, 1, 1, 2},
			{"list<u16>", types.U16{}, 2, 2, 2},
			{"list<u32>", types.U32{}, 4, 4, 2},
			{"list<u64>", types.U64{}, 8, 8, 2},
			{"list<f32>", types.F32{}, 4, 4, 2},
			{"list<f64>", types.F64{}, 8, 8, 2},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				listType := types.List{Element: tc.elemType}
				require.Equal(t, tc.elemSize, listType.ElementSize())
				require.Equal(t, tc.elemAlign, listType.ElementAlign())
				require.Equal(t, tc.flattenCnt, listType.FlattenCount())
			})
		}
	})
}

// TestVariantPayloadTypes tests variants with different payload types.
// This is Task 272.
func TestVariantPayloadTypes(t *testing.T) {
	t.Run("variant_with_u64_payload", func(t *testing.T) {
		variantType := types.Variant{
			Cases: []types.Case{
				{Name: "none", Type: nil},
				{Name: "some-u64", Type: types.U64{}},
			},
		}

		t.Run("none_case", func(t *testing.T) {
			val := types.ValVariant("none", nil)
			flat, err := abi.LowerFlat(nil, variantType, val)
			require.NoError(t, err)
			require.Equal(t, uint64(0), flat[0], "discriminant should be 0")

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, variantType, iter)
			require.NoError(t, err)
			name, payload := lifted.Variant()
			require.Equal(t, "none", name)
			require.Nil(t, payload)
		})

		t.Run("some_case", func(t *testing.T) {
			payload := types.ValU64(0xDEADBEEF12345678)
			val := types.ValVariant("some-u64", &payload)

			flat, err := abi.LowerFlat(nil, variantType, val)
			require.NoError(t, err)
			require.Equal(t, uint64(1), flat[0], "discriminant should be 1")

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, variantType, iter)
			require.NoError(t, err)
			name, liftedPayload := lifted.Variant()
			require.Equal(t, "some-u64", name)
			require.NotNil(t, liftedPayload)
			require.Equal(t, uint64(0xDEADBEEF12345678), liftedPayload.U64())
		})
	})

	t.Run("variant_with_f64_payload", func(t *testing.T) {
		variantType := types.Variant{
			Cases: []types.Case{
				{Name: "int-val", Type: types.S64{}},
				{Name: "float-val", Type: types.F64{}},
			},
		}

		t.Run("float_case", func(t *testing.T) {
			payload := types.ValF64(3.14159265358979)
			val := types.ValVariant("float-val", &payload)

			flat, err := abi.LowerFlat(nil, variantType, val)
			require.NoError(t, err)
			require.Equal(t, uint64(1), flat[0], "discriminant should be 1")

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, variantType, iter)
			require.NoError(t, err)
			name, liftedPayload := lifted.Variant()
			require.Equal(t, "float-val", name)
			require.NotNil(t, liftedPayload)
			require.Equal(t, 3.14159265358979, liftedPayload.F64())
		})
	})

	t.Run("variant_with_record_payload", func(t *testing.T) {
		recordType := types.Record{
			Fields: []types.Field{
				{Name: "x", Type: types.S32{}},
				{Name: "y", Type: types.S32{}},
			},
		}
		variantType := types.Variant{
			Cases: []types.Case{
				{Name: "empty", Type: nil},
				{Name: "point", Type: recordType},
			},
		}

		recordPayload := types.ValRecord(map[string]types.Val{
			"x": types.ValS32(10),
			"y": types.ValS32(20),
		})
		val := types.ValVariant("point", &recordPayload)

		flat, err := abi.LowerFlat(nil, variantType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(1), flat[0], "discriminant should be 1")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, variantType, iter)
		require.NoError(t, err)
		name, liftedPayload := lifted.Variant()
		require.Equal(t, "point", name)
		require.NotNil(t, liftedPayload)

		xVal, _ := liftedPayload.RecordField("x")
		require.Equal(t, int32(10), xVal.S32())
		yVal, _ := liftedPayload.RecordField("y")
		require.Equal(t, int32(20), yVal.S32())
	})

	t.Run("variant_with_multiple_payload_sizes", func(t *testing.T) {
		// Variant where cases have different payload sizes
		variantType := types.Variant{
			Cases: []types.Case{
				{Name: "byte", Type: types.U8{}},
				{Name: "word", Type: types.U16{}},
				{Name: "dword", Type: types.U32{}},
				{Name: "qword", Type: types.U64{}},
			},
		}

		// Test smallest payload
		t.Run("byte_case", func(t *testing.T) {
			payload := types.ValU8(255)
			val := types.ValVariant("byte", &payload)

			flat, err := abi.LowerFlat(nil, variantType, val)
			require.NoError(t, err)

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, variantType, iter)
			require.NoError(t, err)
			name, liftedPayload := lifted.Variant()
			require.Equal(t, "byte", name)
			require.Equal(t, uint8(255), liftedPayload.U8())
		})

		// Test largest payload
		t.Run("qword_case", func(t *testing.T) {
			payload := types.ValU64(math.MaxUint64)
			val := types.ValVariant("qword", &payload)

			flat, err := abi.LowerFlat(nil, variantType, val)
			require.NoError(t, err)

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, variantType, iter)
			require.NoError(t, err)
			name, liftedPayload := lifted.Variant()
			require.Equal(t, "qword", name)
			require.Equal(t, uint64(math.MaxUint64), liftedPayload.U64())
		})
	})
}

// TestResultErrorTypes tests result<ok, error> with various error types.
// This is Task 273.
func TestResultErrorTypes(t *testing.T) {
	t.Run("result_with_string_error", func(t *testing.T) {
		// result<u32, string> - ok is simple, error is complex
		resultType := types.Result{
			Ok:    types.U32{},
			Error: types.String{},
		}

		t.Run("ok_case", func(t *testing.T) {
			okVal := types.ValU32(42)
			val := types.ValResultOk(&okVal)

			flat, err := abi.LowerFlat(nil, resultType, val)
			require.NoError(t, err)
			require.Equal(t, uint64(0), flat[0], "ok discriminant should be 0")

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, resultType, iter)
			require.NoError(t, err)
			isOk, liftedOk, _ := lifted.Result()
			require.True(t, isOk)
			require.NotNil(t, liftedOk)
			require.Equal(t, uint32(42), liftedOk.U32())
		})

		// Note: Error case with string requires memory/realloc context
	})

	t.Run("result_with_record_error", func(t *testing.T) {
		errorRecord := types.Record{
			Fields: []types.Field{
				{Name: "code", Type: types.S32{}},
				{Name: "line", Type: types.U32{}},
			},
		}
		resultType := types.Result{
			Ok:    types.Bool{},
			Error: errorRecord,
		}

		t.Run("ok_case", func(t *testing.T) {
			okVal := types.ValBool(true)
			val := types.ValResultOk(&okVal)

			flat, err := abi.LowerFlat(nil, resultType, val)
			require.NoError(t, err)

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, resultType, iter)
			require.NoError(t, err)
			isOk, liftedOk, _ := lifted.Result()
			require.True(t, isOk)
			require.True(t, liftedOk.Bool())
		})

		t.Run("error_case", func(t *testing.T) {
			errVal := types.ValRecord(map[string]types.Val{
				"code": types.ValS32(-1),
				"line": types.ValU32(100),
			})
			val := types.ValResultError(&errVal)

			flat, err := abi.LowerFlat(nil, resultType, val)
			require.NoError(t, err)
			require.Equal(t, uint64(1), flat[0], "error discriminant should be 1")

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, resultType, iter)
			require.NoError(t, err)
			isOk, _, liftedErr := lifted.Result()
			require.False(t, isOk)
			require.NotNil(t, liftedErr)

			code, _ := liftedErr.RecordField("code")
			require.Equal(t, int32(-1), code.S32())
			line, _ := liftedErr.RecordField("line")
			require.Equal(t, uint32(100), line.U32())
		})
	})

	t.Run("result_with_enum_error", func(t *testing.T) {
		enumError := types.Enum{Cases: []string{"not-found", "permission-denied", "timeout"}}
		resultType := types.Result{
			Ok:    types.U64{},
			Error: enumError,
		}

		t.Run("error_not_found", func(t *testing.T) {
			errVal := types.ValEnum("not-found")
			val := types.ValResultError(&errVal)

			flat, err := abi.LowerFlat(nil, resultType, val)
			require.NoError(t, err)

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, resultType, iter)
			require.NoError(t, err)
			isOk, _, liftedErr := lifted.Result()
			require.False(t, isOk)
			require.Equal(t, "not-found", liftedErr.Enum())
		})

		t.Run("error_timeout", func(t *testing.T) {
			errVal := types.ValEnum("timeout")
			val := types.ValResultError(&errVal)

			flat, err := abi.LowerFlat(nil, resultType, val)
			require.NoError(t, err)

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, resultType, iter)
			require.NoError(t, err)
			isOk, _, liftedErr := lifted.Result()
			require.False(t, isOk)
			require.Equal(t, "timeout", liftedErr.Enum())
		})
	})

	t.Run("result_unit_ok_with_error", func(t *testing.T) {
		// result<_, error-code> - success has no payload
		resultType := types.Result{
			Ok:    nil,
			Error: types.S32{},
		}

		t.Run("ok_case", func(t *testing.T) {
			val := types.ValResultOk(nil)

			flat, err := abi.LowerFlat(nil, resultType, val)
			require.NoError(t, err)

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, resultType, iter)
			require.NoError(t, err)
			isOk, liftedOk, _ := lifted.Result()
			require.True(t, isOk)
			require.Nil(t, liftedOk)
		})

		t.Run("error_case", func(t *testing.T) {
			errVal := types.ValS32(-42)
			val := types.ValResultError(&errVal)

			flat, err := abi.LowerFlat(nil, resultType, val)
			require.NoError(t, err)

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, resultType, iter)
			require.NoError(t, err)
			isOk, _, liftedErr := lifted.Result()
			require.False(t, isOk)
			require.Equal(t, int32(-42), liftedErr.S32())
		})
	})
}

// TestDeepTypeResolution tests deeply nested type references.
// This is Task 274.
func TestDeepTypeResolution(t *testing.T) {
	t.Run("deeply_nested_option", func(t *testing.T) {
		// option<option<option<s32>>>
		level1 := types.Option{Some: types.S32{}}
		level2 := types.Option{Some: level1}
		level3 := types.Option{Some: level2}

		t.Run("all_some", func(t *testing.T) {
			innerVal := types.ValS32(42)
			level1Val := types.ValOption(&innerVal)
			level2Val := types.ValOption(&level1Val)
			level3Val := types.ValOption(&level2Val)

			flat, err := abi.LowerFlat(nil, level3, level3Val)
			require.NoError(t, err)

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, level3, iter)
			require.NoError(t, err)

			// Unwrap 3 levels
			l3 := lifted.Option()
			require.NotNil(t, l3)
			l2 := l3.Option()
			require.NotNil(t, l2)
			l1 := l2.Option()
			require.NotNil(t, l1)
			require.Equal(t, int32(42), l1.S32())
		})

		t.Run("outer_none", func(t *testing.T) {
			val := types.ValOption(nil)

			flat, err := abi.LowerFlat(nil, level3, val)
			require.NoError(t, err)

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, level3, iter)
			require.NoError(t, err)
			require.Nil(t, lifted.Option())
		})
	})

	t.Run("deeply_nested_result", func(t *testing.T) {
		// result<result<s32, u32>, bool>
		innerResult := types.Result{Ok: types.S32{}, Error: types.U32{}}
		outerResult := types.Result{Ok: innerResult, Error: types.Bool{}}

		t.Run("outer_ok_inner_ok", func(t *testing.T) {
			innerOk := types.ValS32(100)
			innerResultVal := types.ValResultOk(&innerOk)
			outerVal := types.ValResultOk(&innerResultVal)

			flat, err := abi.LowerFlat(nil, outerResult, outerVal)
			require.NoError(t, err)

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, outerResult, iter)
			require.NoError(t, err)

			outerIsOk, outerOkVal, _ := lifted.Result()
			require.True(t, outerIsOk)
			innerIsOk, innerOkVal, _ := outerOkVal.Result()
			require.True(t, innerIsOk)
			require.Equal(t, int32(100), innerOkVal.S32())
		})

		t.Run("outer_ok_inner_error", func(t *testing.T) {
			innerErr := types.ValU32(404)
			innerResultVal := types.ValResultError(&innerErr)
			outerVal := types.ValResultOk(&innerResultVal)

			flat, err := abi.LowerFlat(nil, outerResult, outerVal)
			require.NoError(t, err)

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, outerResult, iter)
			require.NoError(t, err)

			outerIsOk, outerOkVal, _ := lifted.Result()
			require.True(t, outerIsOk)
			innerIsOk, _, innerErrVal := outerOkVal.Result()
			require.False(t, innerIsOk)
			require.Equal(t, uint32(404), innerErrVal.U32())
		})
	})

	t.Run("nested_variant_in_tuple", func(t *testing.T) {
		variantType := types.Variant{
			Cases: []types.Case{
				{Name: "a", Type: types.S32{}},
				{Name: "b", Type: types.U64{}},
			},
		}
		tupleType := types.Tuple{
			Types: []types.ValType{variantType, types.Bool{}},
		}

		payload := types.ValU64(12345)
		variantVal := types.ValVariant("b", &payload)
		tupleVal := types.ValTuple([]types.Val{variantVal, types.ValBool(true)})

		flat, err := abi.LowerFlat(nil, tupleType, tupleVal)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, tupleType, iter)
		require.NoError(t, err)

		elems := lifted.Tuple()
		require.Equal(t, 2, len(elems))

		name, varPayload := elems[0].Variant()
		require.Equal(t, "b", name)
		require.Equal(t, uint64(12345), varPayload.U64())

		require.True(t, elems[1].Bool())
	})

	t.Run("record_with_nested_types", func(t *testing.T) {
		optionType := types.Option{Some: types.S32{}}
		recordType := types.Record{
			Fields: []types.Field{
				{Name: "id", Type: types.U64{}},
				{Name: "value", Type: optionType},
				{Name: "active", Type: types.Bool{}},
			},
		}

		innerVal := types.ValS32(42)
		optionVal := types.ValOption(&innerVal)
		recordVal := types.ValRecord(map[string]types.Val{
			"id":     types.ValU64(123),
			"value":  optionVal,
			"active": types.ValBool(true),
		})

		flat, err := abi.LowerFlat(nil, recordType, recordVal)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, recordType, iter)
		require.NoError(t, err)

		idVal, _ := lifted.RecordField("id")
		require.Equal(t, uint64(123), idVal.U64())

		valueVal, _ := lifted.RecordField("value")
		optionInner := valueVal.Option()
		require.NotNil(t, optionInner)
		require.Equal(t, int32(42), optionInner.S32())

		activeVal, _ := lifted.RecordField("active")
		require.True(t, activeVal.Bool())
	})
}

// TestReallocIntegration tests that realloc is properly called for heap operations.
// This is Task 271 - additional tests beyond realloc_failure_test.go.
func TestReallocIntegration(t *testing.T) {
	t.Run("realloc_called_for_list", func(t *testing.T) {
		reallocCallCount := 0
		mem := newMockMemory(4096)
		allocPtr := uint32(256)

		ctx := &abi.LowerContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
			Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
				reallocCallCount++
				result := allocPtr
				allocPtr += newSize
				return result, nil
			},
		}

		listType := types.List{Element: types.S32{}}
		listVal := types.ValList([]types.Val{
			types.ValS32(1),
			types.ValS32(2),
			types.ValS32(3),
		})

		_, err := abi.LowerFlat(ctx, listType, listVal)
		require.NoError(t, err)
		require.Equal(t, 1, reallocCallCount, "realloc should be called once for list")
	})

	t.Run("realloc_not_called_for_primitives", func(t *testing.T) {
		reallocCalled := false
		ctx := &abi.LowerContext{
			Memory: newMockMemory(1024),
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
			Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
				reallocCalled = true
				return 0, nil
			},
		}

		// Primitives don't need realloc
		_, err := abi.LowerFlat(ctx, types.S32{}, types.ValS32(42))
		require.NoError(t, err)
		require.False(t, reallocCalled, "realloc should not be called for primitives")

		_, err = abi.LowerFlat(ctx, types.U64{}, types.ValU64(123))
		require.NoError(t, err)
		require.False(t, reallocCalled, "realloc should not be called for primitives")

		_, err = abi.LowerFlat(ctx, types.F64{}, types.ValF64(3.14))
		require.NoError(t, err)
		require.False(t, reallocCalled, "realloc should not be called for primitives")
	})

	t.Run("realloc_receives_correct_params_for_list", func(t *testing.T) {
		var capturedAlign, capturedSize uint32
		mem := newMockMemory(4096)

		ctx := &abi.LowerContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
			Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
				capturedAlign = align
				capturedSize = newSize
				return 256, nil
			},
		}

		listType := types.List{Element: types.U64{}}
		listVal := types.ValList([]types.Val{
			types.ValU64(1),
			types.ValU64(2),
		})

		_, err := abi.LowerFlat(ctx, listType, listVal)
		require.NoError(t, err)
		require.Equal(t, uint32(8), capturedAlign, "align should be 8 for u64")
		require.Equal(t, uint32(16), capturedSize, "size should be 16 for 2 u64s")
	})
}
