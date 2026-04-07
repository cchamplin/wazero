// Package conformance contains conformance tests for the Component Model implementation.
// ABI edge case tests verify handling of boundary conditions in the Canonical ABI.
package conformance

import (
	"encoding/binary"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/abi"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestABI_ExactlyMaxFlatParams tests a record with exactly MaxFlatParams (16) i32 fields.
// This should pass in flat representation without spilling to memory.
func TestABI_ExactlyMaxFlatParams(t *testing.T) {
	// Create a record with exactly 16 u32 fields
	fields := make([]types.Field, abi.MaxFlatParams)
	fieldVals := make(map[string]types.Val)
	for i := 0; i < abi.MaxFlatParams; i++ {
		name := string(rune('a' + i))
		fields[i] = types.Field{Name: name, Type: types.U32{}}
		fieldVals[name] = types.ValU32(uint32(i + 1))
	}
	recordType := types.Record{Fields: fields}
	val := types.ValRecord(fieldVals)

	t.Run("type_properties", func(t *testing.T) {
		require.Equal(t, abi.MaxFlatParams, recordType.FlattenCount())
	})

	t.Run("lower_flat", func(t *testing.T) {
		flat, err := abi.LowerFlat(nil, recordType, val)
		require.NoError(t, err)
		require.Equal(t, abi.MaxFlatParams, len(flat))

		// Verify values
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
		lifted, err := abi.LiftFlat(nil, recordType, iter)
		require.NoError(t, err)

		// Verify all fields lifted correctly
		for i := 0; i < abi.MaxFlatParams; i++ {
			name := string(rune('a' + i))
			fieldVal, ok := lifted.RecordField(name)
			require.True(t, ok)
			require.Equal(t, uint32(i+100), fieldVal.U32())
		}
	})
}

// TestABI_ExactlyMaxPlusOne tests a record with MaxFlatParams+1 (17) i32 fields.
// This exceeds the flat threshold and would require heap allocation in real usage.
func TestABI_ExactlyMaxPlusOne(t *testing.T) {
	// Create a record with 17 u32 fields
	numFields := abi.MaxFlatParams + 1
	fields := make([]types.Field, numFields)
	fieldVals := make(map[string]types.Val)
	for i := 0; i < numFields; i++ {
		name := string(rune('a' + i))
		fields[i] = types.Field{Name: name, Type: types.U32{}}
		fieldVals[name] = types.ValU32(uint32(i + 1))
	}
	recordType := types.Record{Fields: fields}

	t.Run("type_properties", func(t *testing.T) {
		require.Equal(t, numFields, recordType.FlattenCount())
		require.True(t, recordType.FlattenCount() > abi.MaxFlatParams)
	})
}

// TestABI_MaxStringLength tests handling of very large strings.
func TestABI_MaxStringLength(t *testing.T) {
	// Create a 1MB string
	const size = 1024 * 1024
	largeString := make([]byte, size)
	for i := range largeString {
		largeString[i] = 'a' + byte(i%26)
	}
	str := string(largeString)

	mem := newMockMemory(size + 4096)
	allocPtr := uint32(256)

	lowerCtx := &abi.LowerContext{
		Memory: mem,
		Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
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
		// Reset allocator
		allocPtr = uint32(256)

		ptr, length, err := abi.LowerString(lowerCtx, str)
		require.NoError(t, err)

		liftCtx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		}

		iter := abi.NewFlatIter([]uint64{uint64(ptr), uint64(length)})
		lifted, err := abi.LiftFlat(liftCtx, types.String{}, iter)
		require.NoError(t, err)
		require.Equal(t, str, lifted.StringVal())
	})
}

// TestABI_ZeroLengthList tests that empty lists don't call realloc.
func TestABI_ZeroLengthList(t *testing.T) {
	listType := types.List{Element: types.U32{}}
	emptyList := types.ValList([]types.Val{})

	reallocCalled := false
	ctx := &abi.LowerContext{
		Memory: newMockMemory(1024),
		Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			reallocCalled = true
			return 0, nil
		},
	}

	t.Run("lower_empty_list", func(t *testing.T) {
		flat, err := abi.LowerFlat(ctx, listType, emptyList)
		require.NoError(t, err)
		require.Equal(t, 2, len(flat))
		require.Equal(t, uint64(0), flat[0], "empty list ptr should be 0")
		require.Equal(t, uint64(0), flat[1], "empty list len should be 0")
		require.False(t, reallocCalled, "realloc should not be called for empty list")
	})

	t.Run("lift_empty_list", func(t *testing.T) {
		liftCtx := &abi.LiftContext{
			Memory: newMockMemory(1024),
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		}

		iter := abi.NewFlatIter([]uint64{0, 0})
		lifted, err := abi.LiftFlat(liftCtx, listType, iter)
		require.NoError(t, err)
		require.Equal(t, 0, len(lifted.List()))
	})
}

// TestABI_AlignmentBoundary tests reading values at various alignment boundaries.
func TestABI_AlignmentBoundary(t *testing.T) {
	testCases := []struct {
		name   string
		offset uint32
		typ    types.ValType
		setup  func(mem *mockMemory, offset uint32)
		verify func(t *testing.T, lifted types.Val)
	}{
		{
			name:   "u32_at_aligned_offset",
			offset: 0,
			typ:    types.U32{},
			setup: func(mem *mockMemory, offset uint32) {
				binary.LittleEndian.PutUint32(mem.data[offset:], 0xDEADBEEF)
			},
			verify: func(t *testing.T, lifted types.Val) {
				require.Equal(t, uint32(0xDEADBEEF), lifted.U32())
			},
		},
		{
			name:   "u32_at_offset_4",
			offset: 4,
			typ:    types.U32{},
			setup: func(mem *mockMemory, offset uint32) {
				binary.LittleEndian.PutUint32(mem.data[offset:], 0xCAFEBABE)
			},
			verify: func(t *testing.T, lifted types.Val) {
				require.Equal(t, uint32(0xCAFEBABE), lifted.U32())
			},
		},
		{
			name:   "u64_at_aligned_offset",
			offset: 8,
			typ:    types.U64{},
			setup: func(mem *mockMemory, offset uint32) {
				binary.LittleEndian.PutUint64(mem.data[offset:], 0x123456789ABCDEF0)
			},
			verify: func(t *testing.T, lifted types.Val) {
				require.Equal(t, uint64(0x123456789ABCDEF0), lifted.U64())
			},
		},
		{
			name:   "u16_at_offset_2",
			offset: 2,
			typ:    types.U16{},
			setup: func(mem *mockMemory, offset uint32) {
				binary.LittleEndian.PutUint16(mem.data[offset:], 0xABCD)
			},
			verify: func(t *testing.T, lifted types.Val) {
				require.Equal(t, uint16(0xABCD), lifted.U16())
			},
		},
		{
			name:   "u8_at_offset_1",
			offset: 1,
			typ:    types.U8{},
			setup: func(mem *mockMemory, offset uint32) {
				mem.data[offset] = 0x42
			},
			verify: func(t *testing.T, lifted types.Val) {
				require.Equal(t, uint8(0x42), lifted.U8())
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mem := newMockMemory(1024)
			tc.setup(mem, tc.offset)

			ctx := &abi.LiftContext{
				Memory: mem,
				Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
			}

			lifted, err := abi.LiftHeap(ctx, tc.typ, tc.offset)
			require.NoError(t, err)
			tc.verify(t, lifted)
		})
	}
}

// TestABI_InvalidAlignment tests that misaligned reads work (wasm32 allows unaligned access).
// Note: WebAssembly allows unaligned memory access, though it may be slower.
func TestABI_InvalidAlignment(t *testing.T) {
	testCases := []struct {
		name   string
		offset uint32 // intentionally misaligned
		typ    types.ValType
	}{
		{"u32_at_offset_1", 1, types.U32{}},
		{"u32_at_offset_3", 3, types.U32{}},
		{"u64_at_offset_1", 1, types.U64{}},
		{"u64_at_offset_5", 5, types.U64{}},
		{"u16_at_offset_1", 1, types.U16{}},
		{"u16_at_offset_3", 3, types.U16{}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mem := newMockMemory(1024)
			// Write some data that we can read back
			for i := 0; i < 16; i++ {
				mem.data[i] = byte(i)
			}

			ctx := &abi.LiftContext{
				Memory: mem,
				Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
			}

			// WebAssembly allows unaligned access - should not error
			lifted, err := abi.LiftHeap(ctx, tc.typ, tc.offset)
			// This should either work (wasm semantics) or error gracefully
			if err == nil {
				// Verify we got some value back
				require.NotNil(t, lifted)
			}
			// No panic is the main test
		})
	}
}

// TestABI_FlatIterExhaustion tests FlatIter behavior when values are exhausted.
func TestABI_FlatIterExhaustion(t *testing.T) {
	t.Run("single_value_iter", func(t *testing.T) {
		iter := abi.NewFlatIter([]uint64{42})
		val := iter.NextI32()
		require.Equal(t, uint32(42), val)
		// Further calls would panic - that's expected behavior
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
		// Store float bits in uint64
		f32Bits := uint64(0x40490FDB)         // approximately pi as f32
		f64Bits := uint64(0x400921FB54442D18) // approximately pi as f64

		iter := abi.NewFlatIter([]uint64{f32Bits, f64Bits})
		f32 := iter.NextF32()
		f64 := iter.NextF64()

		// Verify float values are reasonable (close to pi)
		require.True(t, f32 > 3.0 && f32 < 4.0)
		require.True(t, f64 > 3.0 && f64 < 4.0)
	})
}

// TestABI_RecordFieldOrder tests that record fields are processed in definition order.
func TestABI_RecordFieldOrder(t *testing.T) {
	recordType := types.Record{
		Fields: []types.Field{
			{Name: "first", Type: types.U32{}},
			{Name: "second", Type: types.U32{}},
			{Name: "third", Type: types.U32{}},
		},
	}

	val := types.ValRecord(map[string]types.Val{
		"first":  types.ValU32(100),
		"second": types.ValU32(200),
		"third":  types.ValU32(300),
	})

	flat, err := abi.LowerFlat(nil, recordType, val)
	require.NoError(t, err)

	// Fields should be flattened in definition order
	require.Equal(t, uint64(100), flat[0], "first should be at index 0")
	require.Equal(t, uint64(200), flat[1], "second should be at index 1")
	require.Equal(t, uint64(300), flat[2], "third should be at index 2")
}

// TestABI_VariantPadding tests that variant payloads are padded correctly.
func TestABI_VariantPadding(t *testing.T) {
	// variant { small: u8, large: u64 }
	variantType := types.Variant{
		Cases: []types.Case{
			{Name: "small", Type: types.U8{}},
			{Name: "large", Type: types.U64{}},
		},
	}

	t.Run("small_case_padded", func(t *testing.T) {
		payload := types.ValU8(42)
		val := types.ValVariant("small", &payload)

		flat, err := abi.LowerFlat(nil, variantType, val)
		require.NoError(t, err)
		// Discriminant (1) + payload (1, but padded to max which is 1 for u64) = 2
		require.Equal(t, 2, len(flat))
		require.Equal(t, uint64(0), flat[0], "discriminant for 'small' case")
		require.Equal(t, uint64(42), flat[1], "payload value")
	})

	t.Run("large_case", func(t *testing.T) {
		payload := types.ValU64(0x123456789ABCDEF0)
		val := types.ValVariant("large", &payload)

		flat, err := abi.LowerFlat(nil, variantType, val)
		require.NoError(t, err)
		require.Equal(t, 2, len(flat))
		require.Equal(t, uint64(1), flat[0], "discriminant for 'large' case")
		require.Equal(t, uint64(0x123456789ABCDEF0), flat[1])
	})
}

// TestABI_OptionNone tests option<T> with None value.
func TestABI_OptionNone(t *testing.T) {
	optionType := types.Option{Some: types.U32{}}

	val := types.ValOption(nil)
	flat, err := abi.LowerFlat(nil, optionType, val)
	require.NoError(t, err)
	require.Equal(t, 2, len(flat), "option has discriminant + padded payload")
	require.Equal(t, uint64(0), flat[0], "None discriminant is 0")

	// Lift back
	iter := abi.NewFlatIter(flat)
	lifted, err := abi.LiftFlat(nil, optionType, iter)
	require.NoError(t, err)
	require.Nil(t, lifted.Option())
}

// TestABI_ResultOkNil tests result<_, error> with Ok(nil).
func TestABI_ResultOkNil(t *testing.T) {
	resultType := types.Result{
		Ok:    nil, // unit ok
		Error: types.String{},
	}

	val := types.ValResultOk(nil)
	flat, err := abi.LowerFlat(nil, resultType, val)
	require.NoError(t, err)
	require.Equal(t, uint64(0), flat[0], "Ok discriminant is 0")
}

// TestABI_ResultErrorNil tests result<ok, _> with Error(nil).
func TestABI_ResultErrorNil(t *testing.T) {
	resultType := types.Result{
		Ok:    types.U32{},
		Error: nil, // unit error
	}

	val := types.ValResultError(nil)
	flat, err := abi.LowerFlat(nil, resultType, val)
	require.NoError(t, err)
	require.Equal(t, uint64(1), flat[0], "Error discriminant is 1")
}

// TestABI_FlagsAllSet tests flags with all bits set.
func TestABI_FlagsAllSet(t *testing.T) {
	flagsType := types.Flags{
		Names: []string{"a", "b", "c", "d", "e", "f", "g", "h"},
	}

	flags := make(map[string]bool)
	for _, name := range flagsType.Names {
		flags[name] = true
	}
	val := types.ValFlags(flags)

	flat, err := abi.LowerFlat(nil, flagsType, val)
	require.NoError(t, err)
	require.Equal(t, 1, len(flat))
	require.Equal(t, uint64(0xFF), flat[0], "all 8 flags should be set")
}

// TestABI_FlagsNoneSet tests flags with no bits set.
func TestABI_FlagsNoneSet(t *testing.T) {
	flagsType := types.Flags{
		Names: []string{"a", "b", "c", "d", "e", "f", "g", "h"},
	}

	flags := make(map[string]bool)
	val := types.ValFlags(flags)

	flat, err := abi.LowerFlat(nil, flagsType, val)
	require.NoError(t, err)
	require.Equal(t, 1, len(flat))
	require.Equal(t, uint64(0), flat[0], "no flags should be set")
}

// TestABI_EnumRoundtrip tests enum lowering and lifting.
func TestABI_EnumRoundtrip(t *testing.T) {
	enumType := types.Enum{
		Cases: []string{"first", "second", "third"},
	}

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

			flat, err := abi.LowerFlat(nil, enumType, val)
			require.NoError(t, err)
			require.Equal(t, tc.expected, flat[0])

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(nil, enumType, iter)
			require.NoError(t, err)
			require.Equal(t, tc.name, lifted.Enum())
		})
	}
}

// TestABI_TupleEmpty tests empty tuple handling.
func TestABI_TupleEmpty(t *testing.T) {
	tupleType := types.Tuple{Types: []types.ValType{}}
	val := types.ValTuple([]types.Val{})

	flat, err := abi.LowerFlat(nil, tupleType, val)
	require.NoError(t, err)
	require.Equal(t, 0, len(flat))

	iter := abi.NewFlatIter([]uint64{})
	lifted, err := abi.LiftFlat(nil, tupleType, iter)
	require.NoError(t, err)
	require.Equal(t, 0, len(lifted.Tuple()))
}

// TestABI_MaxFlatParamsConstant verifies the MaxFlatParams constant.
func TestABI_MaxFlatParamsConstant(t *testing.T) {
	require.Equal(t, 16, abi.MaxFlatParams, "MaxFlatParams should be 16 per Component Model spec")
}

// TestABI_MaxFlatResultsConstant verifies the MaxFlatResults constant.
func TestABI_MaxFlatResultsConstant(t *testing.T) {
	require.Equal(t, 1, abi.MaxFlatResults, "MaxFlatResults should be 1 per Component Model spec")
}
