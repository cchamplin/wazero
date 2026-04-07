// Package conformance contains conformance tests for the Component Model implementation.
// Type system edge case tests verify handling of boundary conditions in type definitions.
package conformance

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component/abi"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestType_DeeplyNestedRecord tests that deeply nested records don't cause stack overflow.
// Creates a 100-level nested record structure and verifies operations don't panic.
func TestType_DeeplyNestedRecord(t *testing.T) {
	const depth = 100

	// Build deeply nested record type: record { inner: record { inner: ... { value: u32 } } }
	var innerType types.ValType = types.U32{}

	for i := 0; i < depth; i++ {
		innerType = types.Record{
			Fields: []types.Field{
				{Name: "inner", Type: innerType},
			},
		}
	}

	// Build the corresponding value
	innerVal := types.ValU32(42)
	for i := 0; i < depth; i++ {
		innerVal = types.ValRecord(map[string]types.Val{
			"inner": innerVal,
		})
	}

	// Test that Size/Align/FlattenCount don't overflow or panic
	outerRecord := innerType.(types.Record)
	t.Run("type_properties", func(t *testing.T) {
		// Should compute without stack overflow
		size := outerRecord.Size()
		align := outerRecord.Align()
		flatCount := outerRecord.FlattenCount()

		// The innermost type is u32 (size 4, align 4, flatten 1)
		// Each wrapper doesn't add padding (record of single field)
		require.Equal(t, uint32(4), size)
		require.Equal(t, uint32(4), align)
		require.Equal(t, 1, flatCount)
	})

	t.Run("lower_flat", func(t *testing.T) {
		// Should not stack overflow
		flat, err := abi.LowerFlat(nil, innerType, innerVal)
		require.NoError(t, err)
		require.Equal(t, 1, len(flat))
		require.Equal(t, uint64(42), flat[0])
	})

	t.Run("lift_flat", func(t *testing.T) {
		iter := abi.NewFlatIter([]uint64{42})
		lifted, err := abi.LiftFlat(nil, innerType, iter)
		require.NoError(t, err)
		require.Equal(t, types.ValKindRecord, lifted.Kind())

		// Navigate to innermost value
		current := lifted
		for i := 0; i < depth; i++ {
			innerV, ok := current.RecordField("inner")
			require.True(t, ok, "should have inner field at depth %d", i)
			current = innerV
		}
		require.Equal(t, uint32(42), current.U32())
	})
}

// TestType_ZeroSizeRecord tests that records containing only empty records have size 0.
func TestType_ZeroSizeRecord(t *testing.T) {
	// record { a: record{}, b: record{} }
	emptyInner := types.Record{Fields: []types.Field{}}
	outerRecord := types.Record{
		Fields: []types.Field{
			{Name: "a", Type: emptyInner},
			{Name: "b", Type: emptyInner},
		},
	}

	t.Run("type_properties", func(t *testing.T) {
		require.Equal(t, uint32(0), outerRecord.Size(), "record of empty records should have size 0")
		require.Equal(t, uint32(1), outerRecord.Align(), "record of empty records should have align 1")
		require.Equal(t, 0, outerRecord.FlattenCount(), "record of empty records should have FlattenCount 0")
	})

	t.Run("roundtrip", func(t *testing.T) {
		val := types.ValRecord(map[string]types.Val{
			"a": types.ValRecord(map[string]types.Val{}),
			"b": types.ValRecord(map[string]types.Val{}),
		})

		flat, err := abi.LowerFlat(nil, outerRecord, val)
		require.NoError(t, err)
		require.Equal(t, 0, len(flat))

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, outerRecord, iter)
		require.NoError(t, err)
		require.Equal(t, types.ValKindRecord, lifted.Kind())
	})
}

// TestType_MaxTypeIndex tests that large type indices don't cause panics.
// This simulates referencing types by large index values.
func TestType_MaxTypeIndex(t *testing.T) {
	// Test Own and Borrow with large resource indices
	t.Run("own_large_index", func(t *testing.T) {
		own := types.Own{ResourceIdx: 0xFFFF}
		require.Equal(t, uint32(4), own.Size())
		require.Equal(t, uint32(4), own.Align())
		require.Equal(t, 1, own.FlattenCount())
	})

	t.Run("borrow_large_index", func(t *testing.T) {
		borrow := types.Borrow{ResourceIdx: 0xFFFF}
		require.Equal(t, uint32(4), borrow.Size())
		require.Equal(t, uint32(4), borrow.Align())
		require.Equal(t, 1, borrow.FlattenCount())
	})

	t.Run("own_max_u32_index", func(t *testing.T) {
		own := types.Own{ResourceIdx: 0xFFFFFFFF}
		// Should not panic
		_ = own.Size()
		_ = own.Align()
		_ = own.FlattenCount()
	})
}

// TestType_SingleCaseVariant tests that a variant with only 1 case still has a discriminant.
func TestType_SingleCaseVariant(t *testing.T) {
	// variant { only-case: u32 }
	singleVariant := types.Variant{
		Cases: []types.Case{
			{Name: "only-case", Type: types.U32{}},
		},
	}

	t.Run("type_properties", func(t *testing.T) {
		// Even with 1 case, discriminant is still 1 byte (for extensibility)
		require.Equal(t, uint32(1), singleVariant.DiscriminantSize(), "single-case variant should have 1-byte discriminant")
		// Layout: 1 byte disc + 3 bytes padding + 4 bytes u32 = 8 bytes
		require.Equal(t, uint32(8), singleVariant.Size())
		require.Equal(t, uint32(4), singleVariant.Align())
		require.Equal(t, 2, singleVariant.FlattenCount(), "discriminant + payload")
	})

	t.Run("roundtrip", func(t *testing.T) {
		payload := types.ValU32(123)
		val := types.ValVariant("only-case", &payload)

		flat, err := abi.LowerFlat(nil, singleVariant, val)
		require.NoError(t, err)
		require.Equal(t, 2, len(flat))
		require.Equal(t, uint64(0), flat[0], "discriminant should be 0")
		require.Equal(t, uint64(123), flat[1])

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, singleVariant, iter)
		require.NoError(t, err)
		caseName, payloadPtr := lifted.Variant()
		require.Equal(t, "only-case", caseName)
		require.NotNil(t, payloadPtr)
		require.Equal(t, uint32(123), payloadPtr.U32())
	})
}

// TestType_VariantMaxDiscriminant tests discriminant sizing for large variant case counts.
// Per the spec:
// - 1-256 cases: 1 byte (u8) discriminant
// - 257-65536 cases: 2 bytes (u16) discriminant
// - 65537+ cases: 4 bytes (u32) discriminant
func TestType_VariantMaxDiscriminant(t *testing.T) {
	t.Run("256_cases_u8_discriminant", func(t *testing.T) {
		cases := make([]types.Case, 256)
		for i := range cases {
			cases[i] = types.Case{Name: "case", Type: nil}
		}
		v := types.Variant{Cases: cases}
		require.Equal(t, uint32(1), v.DiscriminantSize(), "256 cases should use u8 discriminant")
	})

	t.Run("257_cases_u16_discriminant", func(t *testing.T) {
		cases := make([]types.Case, 257)
		for i := range cases {
			cases[i] = types.Case{Name: "case", Type: nil}
		}
		v := types.Variant{Cases: cases}
		require.Equal(t, uint32(2), v.DiscriminantSize(), "257 cases should use u16 discriminant")
	})

	t.Run("500_cases_u16_discriminant", func(t *testing.T) {
		cases := make([]types.Case, 500)
		for i := range cases {
			cases[i] = types.Case{Name: "case", Type: nil}
		}
		v := types.Variant{Cases: cases}
		require.Equal(t, uint32(2), v.DiscriminantSize(), "500 cases should use u16 discriminant")
	})

	t.Run("65536_cases_u16_discriminant", func(t *testing.T) {
		cases := make([]types.Case, 65536)
		for i := range cases {
			cases[i] = types.Case{Name: "case", Type: nil}
		}
		v := types.Variant{Cases: cases}
		require.Equal(t, uint32(2), v.DiscriminantSize(), "65536 cases should use u16 discriminant")
	})

	t.Run("65537_cases_u32_discriminant", func(t *testing.T) {
		cases := make([]types.Case, 65537)
		for i := range cases {
			cases[i] = types.Case{Name: "case", Type: nil}
		}
		v := types.Variant{Cases: cases}
		require.Equal(t, uint32(4), v.DiscriminantSize(), "65537 cases should use u32 discriminant")
	})
}

// TestType_FlagsMaxCount tests flags type sizing for large flag counts.
// Per the spec:
// - 0 flags: size 0
// - 1-8 flags: 1 byte
// - 9-16 flags: 2 bytes
// - 17-32 flags: 4 bytes
// - 33-64 flags: 8 bytes (2 u32s)
func TestType_FlagsMaxCount(t *testing.T) {
	t.Run("0_flags", func(t *testing.T) {
		f := types.Flags{Names: []string{}}
		require.Equal(t, uint32(0), f.Size())
		require.Equal(t, uint32(1), f.Align())
		require.Equal(t, 0, f.FlattenCount())
	})

	t.Run("8_flags_1_byte", func(t *testing.T) {
		names := make([]string, 8)
		for i := range names {
			names[i] = "flag"
		}
		f := types.Flags{Names: names}
		require.Equal(t, uint32(1), f.Size())
		require.Equal(t, uint32(1), f.Align())
		require.Equal(t, 1, f.FlattenCount())
	})

	t.Run("16_flags_2_bytes", func(t *testing.T) {
		names := make([]string, 16)
		for i := range names {
			names[i] = "flag"
		}
		f := types.Flags{Names: names}
		require.Equal(t, uint32(2), f.Size())
		require.Equal(t, uint32(2), f.Align())
		require.Equal(t, 1, f.FlattenCount())
	})

	t.Run("32_flags_4_bytes", func(t *testing.T) {
		names := make([]string, 32)
		for i := range names {
			names[i] = "flag"
		}
		f := types.Flags{Names: names}
		require.Equal(t, uint32(4), f.Size())
		require.Equal(t, uint32(4), f.Align())
		require.Equal(t, 1, f.FlattenCount())
	})

	t.Run("33_flags_8_bytes", func(t *testing.T) {
		names := make([]string, 33)
		for i := range names {
			names[i] = "flag"
		}
		f := types.Flags{Names: names}
		require.Equal(t, uint32(8), f.Size())
		require.Equal(t, uint32(4), f.Align())
		require.Equal(t, 2, f.FlattenCount())
	})

	t.Run("64_flags_8_bytes", func(t *testing.T) {
		names := make([]string, 64)
		for i := range names {
			names[i] = "flag"
		}
		f := types.Flags{Names: names}
		require.Equal(t, uint32(8), f.Size(), "64 flags should require 8 bytes")
		require.Equal(t, uint32(4), f.Align())
		require.Equal(t, 2, f.FlattenCount(), "64 flags need 2 i32s")
	})

	t.Run("65_flags_12_bytes", func(t *testing.T) {
		names := make([]string, 65)
		for i := range names {
			names[i] = "flag"
		}
		f := types.Flags{Names: names}
		require.Equal(t, uint32(12), f.Size(), "65 flags should require 12 bytes (3 u32s)")
		require.Equal(t, uint32(4), f.Align())
		require.Equal(t, 3, f.FlattenCount(), "65 flags need 3 i32s")
	})
}

// TestType_ListOfEmptyRecord tests list<record{}> handling.
// Elements have size 0, but the list itself should still work.
func TestType_ListOfEmptyRecord(t *testing.T) {
	emptyRecord := types.Record{Fields: []types.Field{}}
	listType := types.List{Element: emptyRecord}

	t.Run("type_properties", func(t *testing.T) {
		// List is always ptr+len = 8 bytes
		require.Equal(t, uint32(8), listType.Size())
		require.Equal(t, uint32(4), listType.Align())
		require.Equal(t, 2, listType.FlattenCount())

		// But element size is 0
		require.Equal(t, uint32(0), listType.ElementSize())
		require.Equal(t, uint32(1), listType.ElementAlign())
	})

	t.Run("empty_list", func(t *testing.T) {
		val := types.ValList([]types.Val{})

		flat, err := abi.LowerFlat(nil, listType, val)
		require.NoError(t, err)
		require.Equal(t, 2, len(flat))
		require.Equal(t, uint64(0), flat[0], "empty list ptr should be 0")
		require.Equal(t, uint64(0), flat[1], "empty list len should be 0")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, listType, iter)
		require.NoError(t, err)
		require.Equal(t, 0, len(lifted.List()))
	})
}

// TestType_OptionOfEmptyRecord tests option<record{}> handling.
func TestType_OptionOfEmptyRecord(t *testing.T) {
	emptyRecord := types.Record{Fields: []types.Field{}}
	optionType := types.Option{Some: emptyRecord}

	t.Run("type_properties", func(t *testing.T) {
		// Option is a variant with 2 cases (none, some)
		// Discriminant is 1 byte, payload is empty record (size 0)
		// So total size is 1 byte with align 1
		require.Equal(t, uint32(1), optionType.Size())
		require.Equal(t, uint32(1), optionType.Align())
		// FlattenCount: discriminant (1) + max payload flatten (0) = 1
		require.Equal(t, 1, optionType.FlattenCount())
	})

	t.Run("none_case", func(t *testing.T) {
		val := types.ValOption(nil)

		flat, err := abi.LowerFlat(nil, optionType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(0), flat[0], "None discriminant should be 0")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, optionType, iter)
		require.NoError(t, err)
		require.Nil(t, lifted.Option())
	})

	t.Run("some_case", func(t *testing.T) {
		emptyVal := types.ValRecord(map[string]types.Val{})
		val := types.ValOption(&emptyVal)

		flat, err := abi.LowerFlat(nil, optionType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(1), flat[0], "Some discriminant should be 1")

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(nil, optionType, iter)
		require.NoError(t, err)
		require.NotNil(t, lifted.Option())
	})
}

// TestType_TupleAlignment tests tuple alignment calculations.
func TestType_TupleAlignment(t *testing.T) {
	t.Run("tuple_u8_u64_alignment", func(t *testing.T) {
		// tuple<u8, u64> should have align 8 due to u64
		tupleType := types.Tuple{
			Types: []types.ValType{types.U8{}, types.U64{}},
		}
		require.Equal(t, uint32(8), tupleType.Align())
		// Layout: u8 at 0, padding to 8, u64 at 8, total = 16
		require.Equal(t, uint32(16), tupleType.Size())
	})

	t.Run("tuple_u64_u8_alignment", func(t *testing.T) {
		// tuple<u64, u8> should have align 8
		tupleType := types.Tuple{
			Types: []types.ValType{types.U64{}, types.U8{}},
		}
		require.Equal(t, uint32(8), tupleType.Align())
		// Layout: u64 at 0, u8 at 8, total = 9, rounded up to 16
		require.Equal(t, uint32(16), tupleType.Size())
	})
}

// TestType_RecordFieldOffsets tests that field offsets are computed correctly.
func TestType_RecordFieldOffsets(t *testing.T) {
	t.Run("simple_offsets", func(t *testing.T) {
		// record { a: u8, b: u32, c: u8 }
		recordType := types.Record{
			Fields: []types.Field{
				{Name: "a", Type: types.U8{}},
				{Name: "b", Type: types.U32{}},
				{Name: "c", Type: types.U8{}},
			},
		}
		offsets := recordType.FieldOffsets()
		require.Equal(t, 3, len(offsets))
		require.Equal(t, uint32(0), offsets[0], "a at offset 0")
		require.Equal(t, uint32(4), offsets[1], "b at offset 4 (after padding)")
		require.Equal(t, uint32(8), offsets[2], "c at offset 8")
	})

	t.Run("no_padding_needed", func(t *testing.T) {
		// record { a: u32, b: u32, c: u32 }
		recordType := types.Record{
			Fields: []types.Field{
				{Name: "a", Type: types.U32{}},
				{Name: "b", Type: types.U32{}},
				{Name: "c", Type: types.U32{}},
			},
		}
		offsets := recordType.FieldOffsets()
		require.Equal(t, uint32(0), offsets[0])
		require.Equal(t, uint32(4), offsets[1])
		require.Equal(t, uint32(8), offsets[2])
	})
}

// TestType_EnumDiscriminantSize tests enum discriminant sizing.
func TestType_EnumDiscriminantSize(t *testing.T) {
	t.Run("256_cases_u8", func(t *testing.T) {
		cases := make([]string, 256)
		for i := range cases {
			cases[i] = "case"
		}
		e := types.Enum{Cases: cases}
		require.Equal(t, uint32(1), e.Size())
		require.Equal(t, uint32(1), e.Align())
	})

	t.Run("257_cases_u16", func(t *testing.T) {
		cases := make([]string, 257)
		for i := range cases {
			cases[i] = "case"
		}
		e := types.Enum{Cases: cases}
		require.Equal(t, uint32(2), e.Size())
		require.Equal(t, uint32(2), e.Align())
	})

	t.Run("65537_cases_u32", func(t *testing.T) {
		cases := make([]string, 65537)
		for i := range cases {
			cases[i] = "case"
		}
		e := types.Enum{Cases: cases}
		require.Equal(t, uint32(4), e.Size())
		require.Equal(t, uint32(4), e.Align())
	})
}

// TestType_ResultLayout tests result type layout calculations.
func TestType_ResultLayout(t *testing.T) {
	t.Run("result_u32_string", func(t *testing.T) {
		resultType := types.Result{
			Ok:    types.U32{},
			Error: types.String{},
		}
		// Discriminant 1 byte, max payload is string (8 bytes, align 4)
		// Payload offset = align(1, 4) = 4
		// Total = 4 + 8 = 12
		require.Equal(t, uint32(12), resultType.Size())
		require.Equal(t, uint32(4), resultType.Align())
	})

	t.Run("result_unit_unit", func(t *testing.T) {
		resultType := types.Result{
			Ok:    nil,
			Error: nil,
		}
		// Just discriminant
		require.Equal(t, uint32(1), resultType.Size())
		require.Equal(t, uint32(1), resultType.Align())
	})
}

// TestType_VariantPayloadOffset tests variant payload offset calculations.
func TestType_VariantPayloadOffset(t *testing.T) {
	t.Run("variant_with_u64_payload", func(t *testing.T) {
		v := types.Variant{
			Cases: []types.Case{
				{Name: "a", Type: types.U8{}},
				{Name: "b", Type: types.U64{}},
			},
		}
		// Discriminant size = 1 (2 cases)
		// Max payload align = 8 (u64)
		// Payload offset = align(1, 8) = 8
		require.Equal(t, uint32(8), v.PayloadOffset())
	})

	t.Run("variant_with_u8_only", func(t *testing.T) {
		v := types.Variant{
			Cases: []types.Case{
				{Name: "a", Type: types.U8{}},
				{Name: "b", Type: types.U8{}},
			},
		}
		// Discriminant size = 1, max payload align = 1
		// Payload offset = align(1, 1) = 1
		require.Equal(t, uint32(1), v.PayloadOffset())
	})
}
