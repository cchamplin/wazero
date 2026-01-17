// Package conformance contains conformance tests for the Component Model implementation.
// Nesting depth tests verify that deeply nested structures are handled without stack overflow.
package conformance

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/abi"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestNesting_DeeplyNestedTuples tests deeply nested tuple types.
func TestNesting_DeeplyNestedTuples(t *testing.T) {
	const depth = 50

	// Build deeply nested tuple type: tuple<tuple<tuple<...<u32>...>>>
	var innerType types.ValType = types.U32{}
	for i := 0; i < depth; i++ {
		innerType = types.Tuple{Types: []types.ValType{innerType}}
	}

	// Build corresponding value
	innerVal := component.ValU32(12345)
	for i := 0; i < depth; i++ {
		innerVal = component.ValTuple([]component.Val{innerVal})
	}

	t.Run("type_properties", func(t *testing.T) {
		tupleType := innerType.(types.Tuple)
		// Should not stack overflow
		_ = tupleType.Size()
		_ = tupleType.Align()
		_ = tupleType.FlattenCount()
	})

	t.Run("lower_flat", func(t *testing.T) {
		flat, err := abi.LowerFlat(nil, innerType, innerVal)
		require.NoError(t, err)
		require.Equal(t, 1, len(flat))
		require.Equal(t, uint64(12345), flat[0])
	})

	t.Run("lift_flat", func(t *testing.T) {
		iter := abi.NewFlatIter([]uint64{12345})
		lifted, err := abi.LiftFlat(nil, innerType, iter)
		require.NoError(t, err)

		// Navigate to innermost value
		current := lifted
		for i := 0; i < depth; i++ {
			tuple := current.Tuple()
			require.Equal(t, 1, len(tuple))
			current = tuple[0]
		}
		require.Equal(t, uint32(12345), current.U32())
	})
}

// TestNesting_DeeplyNestedOptions tests deeply nested option types.
func TestNesting_DeeplyNestedOptions(t *testing.T) {
	const depth = 30 // Options add overhead, so use smaller depth

	// Build: option<option<option<...<u32>...>>>
	var innerType types.ValType = types.U32{}
	for i := 0; i < depth; i++ {
		innerType = types.Option{Some: innerType}
	}

	t.Run("type_properties", func(t *testing.T) {
		optType := innerType.(types.Option)
		// Should not stack overflow
		_ = optType.Size()
		_ = optType.Align()
		_ = optType.FlattenCount()
	})

	t.Run("lower_lift_roundtrip", func(t *testing.T) {
		// Build value: Some(Some(Some(...(42)...)))
		// Use a recursive function to properly build nested options
		var buildNestedOption func(depth int) component.Val
		buildNestedOption = func(d int) component.Val {
			if d == 0 {
				return component.ValU32(42)
			}
			inner := buildNestedOption(d - 1)
			return component.ValOption(&inner)
		}
		innerVal := buildNestedOption(depth)

		flat, err := abi.LowerFlat(nil, innerType, innerVal)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, innerType, iter)
		require.NoError(t, err)

		// Navigate to innermost value
		current := lifted
		for i := 0; i < depth; i++ {
			opt := current.Option()
			require.NotNil(t, opt)
			current = *opt
		}
		require.Equal(t, uint32(42), current.U32())
	})
}

// TestNesting_DeeplyNestedResults tests deeply nested result types.
func TestNesting_DeeplyNestedResults(t *testing.T) {
	const depth = 20

	// Build: result<result<...<u32, string>..., string>, string>
	var innerType types.ValType = types.U32{}
	for i := 0; i < depth; i++ {
		innerType = types.Result{Ok: innerType, Error: types.String{}}
	}

	t.Run("type_properties", func(t *testing.T) {
		resultType := innerType.(types.Result)
		_ = resultType.Size()
		_ = resultType.Align()
		_ = resultType.FlattenCount()
	})

	t.Run("ok_path_roundtrip", func(t *testing.T) {
		// Build value: Ok(Ok(Ok(...(42)...)))
		// Use a recursive function to properly build nested results
		var buildNestedResult func(depth int) component.Val
		buildNestedResult = func(d int) component.Val {
			if d == 0 {
				return component.ValU32(42)
			}
			inner := buildNestedResult(d - 1)
			return component.ValResultOk(&inner)
		}
		innerVal := buildNestedResult(depth)

		flat, err := abi.LowerFlat(nil, innerType, innerVal)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, innerType, iter)
		require.NoError(t, err)

		// Navigate to innermost value
		current := lifted
		for i := 0; i < depth; i++ {
			isOk, okVal, _ := current.Result()
			require.True(t, isOk)
			require.NotNil(t, okVal)
			current = *okVal
		}
		require.Equal(t, uint32(42), current.U32())
	})
}

// TestNesting_DeeplyNestedVariants tests deeply nested variant types.
func TestNesting_DeeplyNestedVariants(t *testing.T) {
	const depth = 20

	// Build variant with nested variant payload
	var innerType types.ValType = types.U32{}
	for i := 0; i < depth; i++ {
		innerType = types.Variant{
			Cases: []types.Case{
				{Name: "inner", Type: innerType},
				{Name: "other", Type: types.Bool{}},
			},
		}
	}

	t.Run("type_properties", func(t *testing.T) {
		varType := innerType.(types.Variant)
		_ = varType.Size()
		_ = varType.Align()
		_ = varType.FlattenCount()
	})

	t.Run("roundtrip", func(t *testing.T) {
		// Build value using recursive function to properly nest
		var buildNestedVariant func(depth int) component.Val
		buildNestedVariant = func(d int) component.Val {
			if d == 0 {
				return component.ValU32(999)
			}
			inner := buildNestedVariant(d - 1)
			return component.ValVariant("inner", &inner)
		}
		innerVal := buildNestedVariant(depth)

		flat, err := abi.LowerFlat(nil, innerType, innerVal)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, innerType, iter)
		require.NoError(t, err)

		// Navigate to innermost value
		current := lifted
		for i := 0; i < depth; i++ {
			caseName, payload := current.Variant()
			require.Equal(t, "inner", caseName)
			require.NotNil(t, payload)
			current = *payload
		}
		require.Equal(t, uint32(999), current.U32())
	})
}

// TestNesting_WideRecord tests records with many fields.
func TestNesting_WideRecord(t *testing.T) {
	const numFields = 100

	fields := make([]types.Field, numFields)
	fieldVals := make(map[string]component.Val)
	for i := 0; i < numFields; i++ {
		name := string(rune('a' + i%26)) + string(rune('0'+i/26))
		fields[i] = types.Field{Name: name, Type: types.U8{}}
		fieldVals[name] = component.ValU8(uint8(i))
	}

	recordType := types.Record{Fields: fields}
	val := component.ValRecord(fieldVals)

	t.Run("type_properties", func(t *testing.T) {
		size := recordType.Size()
		require.Equal(t, uint32(numFields), size, "100 u8 fields = 100 bytes")
		require.Equal(t, uint32(1), recordType.Align())
		require.Equal(t, numFields, recordType.FlattenCount())
	})

	t.Run("roundtrip", func(t *testing.T) {
		flat, err := abi.LowerFlat(nil, recordType, val)
		require.NoError(t, err)
		require.Equal(t, numFields, len(flat))

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, recordType, iter)
		require.NoError(t, err)

		// Verify all fields
		for i := 0; i < numFields; i++ {
			name := string(rune('a' + i%26)) + string(rune('0'+i/26))
			fieldVal, ok := lifted.RecordField(name)
			require.True(t, ok)
			require.Equal(t, uint8(i), fieldVal.U8())
		}
	})
}

// TestNesting_WideTuple tests tuples with many elements.
func TestNesting_WideTuple(t *testing.T) {
	const numElements = 100

	elementTypes := make([]types.ValType, numElements)
	elementVals := make([]component.Val, numElements)
	for i := 0; i < numElements; i++ {
		elementTypes[i] = types.U8{}
		elementVals[i] = component.ValU8(uint8(i))
	}

	tupleType := types.Tuple{Types: elementTypes}
	val := component.ValTuple(elementVals)

	t.Run("type_properties", func(t *testing.T) {
		require.Equal(t, uint32(numElements), tupleType.Size())
		require.Equal(t, numElements, tupleType.FlattenCount())
	})

	t.Run("roundtrip", func(t *testing.T) {
		flat, err := abi.LowerFlat(nil, tupleType, val)
		require.NoError(t, err)
		require.Equal(t, numElements, len(flat))

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, tupleType, iter)
		require.NoError(t, err)

		elems := lifted.Tuple()
		require.Equal(t, numElements, len(elems))
		for i, elem := range elems {
			require.Equal(t, uint8(i), elem.U8())
		}
	})
}

// TestNesting_LargeFlags tests flags with many flag names.
func TestNesting_LargeFlags(t *testing.T) {
	const numFlags = 128

	names := make([]string, numFlags)
	for i := 0; i < numFlags; i++ {
		names[i] = string(rune('a'+i%26)) + string(rune('0'+i%10)) + string(rune('0'+i/10%10))
	}

	flagsType := types.Flags{Names: names}

	t.Run("type_properties", func(t *testing.T) {
		// 128 flags = 4 u32s = 16 bytes
		require.Equal(t, uint32(16), flagsType.Size())
		require.Equal(t, uint32(4), flagsType.Align())
		require.Equal(t, 4, flagsType.FlattenCount())
	})

	t.Run("all_flags_set", func(t *testing.T) {
		flags := make(map[string]bool)
		for _, name := range names {
			flags[name] = true
		}
		val := component.ValFlags(flags)

		flat, err := abi.LowerFlat(nil, flagsType, val)
		require.NoError(t, err)
		require.Equal(t, 4, len(flat))

		// All bits should be set
		require.Equal(t, uint64(0xFFFFFFFF), flat[0])
		require.Equal(t, uint64(0xFFFFFFFF), flat[1])
		require.Equal(t, uint64(0xFFFFFFFF), flat[2])
		require.Equal(t, uint64(0xFFFFFFFF), flat[3])
	})

	t.Run("alternating_flags", func(t *testing.T) {
		flags := make(map[string]bool)
		for i, name := range names {
			flags[name] = (i % 2) == 0
		}
		val := component.ValFlags(flags)

		flat, err := abi.LowerFlat(nil, flagsType, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, flagsType, iter)
		require.NoError(t, err)

		liftedFlags := lifted.Flags()
		for i, name := range names {
			expected := (i % 2) == 0
			require.Equal(t, expected, liftedFlags[name], "flag %s at index %d", name, i)
		}
	})
}

// TestNesting_LargeEnum tests enums with many cases.
func TestNesting_LargeEnum(t *testing.T) {
	const numCases = 500

	cases := make([]string, numCases)
	for i := 0; i < numCases; i++ {
		cases[i] = string(rune('a'+i%26)) + string(rune('0'+i%10)) + string(rune('0'+i/10%10))
	}

	enumType := types.Enum{Cases: cases}

	t.Run("type_properties", func(t *testing.T) {
		// 500 cases needs u16 (2 bytes)
		require.Equal(t, uint32(2), enumType.Size())
		require.Equal(t, uint32(2), enumType.Align())
	})

	t.Run("first_case", func(t *testing.T) {
		val := component.ValEnum(cases[0])
		flat, err := abi.LowerFlat(nil, enumType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(0), flat[0])
	})

	t.Run("last_case", func(t *testing.T) {
		val := component.ValEnum(cases[numCases-1])
		flat, err := abi.LowerFlat(nil, enumType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(numCases-1), flat[0])
	})

	t.Run("roundtrip_mid_case", func(t *testing.T) {
		midIdx := numCases / 2
		val := component.ValEnum(cases[midIdx])

		flat, err := abi.LowerFlat(nil, enumType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(midIdx), flat[0])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, enumType, iter)
		require.NoError(t, err)
		require.Equal(t, cases[midIdx], lifted.Enum())
	})
}

// TestNesting_LargeVariant tests variants with many cases.
func TestNesting_LargeVariant(t *testing.T) {
	const numCases = 300

	cases := make([]types.Case, numCases)
	for i := 0; i < numCases; i++ {
		name := string(rune('a'+i%26)) + string(rune('0'+i%10)) + string(rune('0'+i/10%10))
		// Alternate between u8 and u32 payloads
		if i%2 == 0 {
			cases[i] = types.Case{Name: name, Type: types.U8{}}
		} else {
			cases[i] = types.Case{Name: name, Type: types.U32{}}
		}
	}

	variantType := types.Variant{Cases: cases}

	t.Run("type_properties", func(t *testing.T) {
		// 300 cases needs u16 discriminant
		require.Equal(t, uint32(2), variantType.DiscriminantSize())
	})

	t.Run("first_case", func(t *testing.T) {
		payload := component.ValU8(42)
		val := component.ValVariant(cases[0].Name, &payload)

		flat, err := abi.LowerFlat(nil, variantType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(0), flat[0])
	})

	t.Run("last_case", func(t *testing.T) {
		lastIdx := numCases - 1
		var payload component.Val
		if lastIdx%2 == 0 {
			payload = component.ValU8(42)
		} else {
			payload = component.ValU32(12345)
		}
		val := component.ValVariant(cases[lastIdx].Name, &payload)

		flat, err := abi.LowerFlat(nil, variantType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(lastIdx), flat[0])
	})
}

// TestNesting_MixedDeep tests a combination of deeply nested different types.
func TestNesting_MixedDeep(t *testing.T) {
	// record { opt: option<tuple<variant { a: u32, b: result<u8, bool> }>> }
	resultType := types.Result{Ok: types.U8{}, Error: types.Bool{}}
	variantType := types.Variant{
		Cases: []types.Case{
			{Name: "a", Type: types.U32{}},
			{Name: "b", Type: resultType},
		},
	}
	tupleType := types.Tuple{Types: []types.ValType{variantType}}
	optionType := types.Option{Some: tupleType}
	recordType := types.Record{
		Fields: []types.Field{
			{Name: "opt", Type: optionType},
		},
	}

	t.Run("type_properties", func(t *testing.T) {
		// Should compute without issues
		_ = recordType.Size()
		_ = recordType.Align()
		_ = recordType.FlattenCount()
	})

	t.Run("roundtrip_a_case", func(t *testing.T) {
		payload := component.ValU32(12345)
		variantVal := component.ValVariant("a", &payload)
		tupleVal := component.ValTuple([]component.Val{variantVal})
		optionVal := component.ValOption(&tupleVal)
		recordVal := component.ValRecord(map[string]component.Val{
			"opt": optionVal,
		})

		flat, err := abi.LowerFlat(nil, recordType, recordVal)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, recordType, iter)
		require.NoError(t, err)

		// Navigate to innermost value
		optField, _ := lifted.RecordField("opt")
		optPayload := optField.Option()
		require.NotNil(t, optPayload)
		tupleElems := optPayload.Tuple()
		require.Equal(t, 1, len(tupleElems))
		caseName, casePayload := tupleElems[0].Variant()
		require.Equal(t, "a", caseName)
		require.NotNil(t, casePayload)
		require.Equal(t, uint32(12345), casePayload.U32())
	})

	t.Run("roundtrip_b_case_ok", func(t *testing.T) {
		okVal := component.ValU8(42)
		resultVal := component.ValResultOk(&okVal)
		variantVal := component.ValVariant("b", &resultVal)
		tupleVal := component.ValTuple([]component.Val{variantVal})
		optionVal := component.ValOption(&tupleVal)
		recordVal := component.ValRecord(map[string]component.Val{
			"opt": optionVal,
		})

		flat, err := abi.LowerFlat(nil, recordType, recordVal)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, recordType, iter)
		require.NoError(t, err)

		optField, _ := lifted.RecordField("opt")
		optPayload := optField.Option()
		tupleElems := optPayload.Tuple()
		caseName, casePayload := tupleElems[0].Variant()
		require.Equal(t, "b", caseName)
		isOk, okPayload, _ := casePayload.Result()
		require.True(t, isOk)
		require.NotNil(t, okPayload)
		require.Equal(t, uint8(42), okPayload.U8())
	})
}
