// Package conformance contains conformance tests for the Component Model implementation.
// Error message quality tests verify that error messages contain useful debugging information.
package conformance

import (
	"encoding/binary"
	"errors"
	"strings"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/abi"
	compbinary "github.com/tetratelabs/wazero/internal/component/binary"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestErrorMessages_Binary tests that binary decoding errors are informative.
func TestErrorMessages_Binary(t *testing.T) {
	t.Run("invalid_magic", func(t *testing.T) {
		data := []byte{0x00, 0x00, 0x00, 0x00, 0x0d, 0x00, 0x01, 0x00}
		_, err := compbinary.DecodeComponent(data)
		require.Error(t, err)
		errStr := err.Error()
		require.True(t, strings.Contains(strings.ToLower(errStr), "magic"),
			"error should mention 'magic': %s", errStr)
	})

	t.Run("invalid_version", func(t *testing.T) {
		data := []byte{0x00, 0x61, 0x73, 0x6d, 0x00, 0x00, 0x01, 0x00}
		_, err := compbinary.DecodeComponent(data)
		require.Error(t, err)
		errStr := err.Error()
		require.True(t, strings.Contains(strings.ToLower(errStr), "version"),
			"error should mention 'version': %s", errStr)
	})

	t.Run("not_component", func(t *testing.T) {
		data := []byte{0x00, 0x61, 0x73, 0x6d, 0x0d, 0x00, 0x00, 0x00}
		_, err := compbinary.DecodeComponent(data)
		require.Error(t, err)
		errStr := strings.ToLower(err.Error())
		require.True(t, strings.Contains(errStr, "component") || strings.Contains(errStr, "layer") || strings.Contains(errStr, "module"),
			"error should indicate not a component: %s", err.Error())
	})
}

// TestErrorMessages_CharValidation tests char validation error messages.
func TestErrorMessages_CharValidation(t *testing.T) {
	t.Run("surrogate_value", func(t *testing.T) {
		iter := abi.NewFlatIter([]uint64{0xD800})
		_, err := abi.LiftFlat(nil, types.Char{}, iter)
		require.Error(t, err)
		errStr := err.Error()
		require.True(t, strings.Contains(errStr, "Unicode scalar") ||
			strings.Contains(errStr, "not a valid"),
			"error should explain why char is invalid: %s", errStr)
		require.True(t, strings.Contains(errStr, "D800") ||
			strings.Contains(errStr, "d800"),
			"error should include the invalid value: %s", errStr)
	})

	t.Run("above_max_unicode", func(t *testing.T) {
		iter := abi.NewFlatIter([]uint64{0x110000})
		_, err := abi.LiftFlat(nil, types.Char{}, iter)
		require.Error(t, err)
		errStr := err.Error()
		require.True(t, strings.Contains(errStr, "not a valid"),
			"error should indicate value is invalid: %s", errStr)
	})
}

// TestErrorMessages_VariantDiscriminant tests variant discriminant errors.
func TestErrorMessages_VariantDiscriminant(t *testing.T) {
	variantType := types.Variant{
		Cases: []types.Case{
			{Name: "a", Type: types.U32{}},
			{Name: "b", Type: types.U32{}},
		},
	}

	t.Run("invalid_discriminant", func(t *testing.T) {
		// Discriminant 10 is out of range for a 2-case variant
		iter := abi.NewFlatIter([]uint64{10, 0})
		_, err := abi.LiftFlat(nil, variantType, iter)
		require.Error(t, err)
		errStr := err.Error()
		require.True(t, strings.Contains(errStr, "discriminant"),
			"error should mention 'discriminant': %s", errStr)
		require.True(t, strings.Contains(errStr, "10"),
			"error should include the invalid value: %s", errStr)
	})
}

// TestErrorMessages_EnumDiscriminant tests enum discriminant errors.
func TestErrorMessages_EnumDiscriminant(t *testing.T) {
	enumType := types.Enum{Cases: []string{"first", "second", "third"}}

	t.Run("invalid_discriminant", func(t *testing.T) {
		iter := abi.NewFlatIter([]uint64{100})
		_, err := abi.LiftFlat(nil, enumType, iter)
		require.Error(t, err)
		errStr := err.Error()
		require.True(t, strings.Contains(errStr, "discriminant"),
			"error should mention 'discriminant': %s", errStr)
		require.True(t, strings.Contains(errStr, "100"),
			"error should include the invalid value: %s", errStr)
	})
}

// TestErrorMessages_UnknownCase tests unknown variant/enum case errors.
func TestErrorMessages_UnknownCase(t *testing.T) {
	t.Run("unknown_variant_case", func(t *testing.T) {
		variantType := types.Variant{
			Cases: []types.Case{
				{Name: "known", Type: types.U32{}},
			},
		}
		payload := types.ValU32(0)
		val := types.ValVariant("unknown_case", &payload)

		_, err := abi.LowerFlat(nil, variantType, val)
		require.Error(t, err)
		errStr := err.Error()
		require.True(t, strings.Contains(errStr, "unknown") ||
			strings.Contains(errStr, "case"),
			"error should mention unknown case: %s", errStr)
		require.True(t, strings.Contains(errStr, "unknown_case"),
			"error should include the invalid case name: %s", errStr)
	})

	t.Run("unknown_enum_case", func(t *testing.T) {
		enumType := types.Enum{Cases: []string{"a", "b", "c"}}
		val := types.ValEnum("nonexistent")

		_, err := abi.LowerFlat(nil, enumType, val)
		require.Error(t, err)
		errStr := err.Error()
		require.True(t, strings.Contains(errStr, "unknown") ||
			strings.Contains(errStr, "case") ||
			strings.Contains(errStr, "enum"),
			"error should mention unknown case: %s", errStr)
		require.True(t, strings.Contains(errStr, "nonexistent"),
			"error should include the invalid case name: %s", errStr)
	})
}

// TestErrorMessages_RecordField tests record field errors.
func TestErrorMessages_RecordField(t *testing.T) {
	recordType := types.Record{
		Fields: []types.Field{
			{Name: "required_field", Type: types.U32{}},
		},
	}

	t.Run("missing_field", func(t *testing.T) {
		val := types.ValRecord(map[string]types.Val{
			"wrong_field": types.ValU32(42),
		})

		_, err := abi.LowerFlat(nil, recordType, val)
		require.Error(t, err)
		errStr := err.Error()
		require.True(t, strings.Contains(errStr, "field") ||
			strings.Contains(errStr, "missing"),
			"error should mention field issue: %s", errStr)
		require.True(t, strings.Contains(errStr, "required_field"),
			"error should include the missing field name: %s", errStr)
	})
}

// TestErrorMessages_TupleLength tests tuple length mismatch errors.
func TestErrorMessages_TupleLength(t *testing.T) {
	tupleType := types.Tuple{
		Types: []types.ValType{types.U32{}, types.U32{}, types.U32{}},
	}

	t.Run("too_few_elements", func(t *testing.T) {
		val := types.ValTuple([]types.Val{
			types.ValU32(1),
		})

		_, err := abi.LowerFlat(nil, tupleType, val)
		require.Error(t, err)
		errStr := err.Error()
		require.True(t, strings.Contains(errStr, "element") ||
			strings.Contains(errStr, "tuple"),
			"error should mention tuple elements: %s", errStr)
	})

	t.Run("too_many_elements", func(t *testing.T) {
		val := types.ValTuple([]types.Val{
			types.ValU32(1), types.ValU32(2), types.ValU32(3),
			types.ValU32(4), types.ValU32(5),
		})

		_, err := abi.LowerFlat(nil, tupleType, val)
		require.Error(t, err)
		errStr := err.Error()
		require.True(t, strings.Contains(errStr, "element") ||
			strings.Contains(errStr, "tuple") ||
			strings.Contains(errStr, "expected"),
			"error should mention element count issue: %s", errStr)
	})
}

// TestErrorMessages_StringDecoding tests string decoding error messages.
func TestErrorMessages_StringDecoding(t *testing.T) {
	t.Run("out_of_bounds_pointer", func(t *testing.T) {
		mem := newMockMemory(64)
		binary.LittleEndian.PutUint32(mem.data[0:], 1000)
		binary.LittleEndian.PutUint32(mem.data[4:], 10)

		ctx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		}

		_, err := abi.LiftString(ctx, 0)
		require.Error(t, err)
		errStr := strings.ToLower(err.Error())
		require.True(t, strings.Contains(errStr, "read") ||
			strings.Contains(errStr, "memory") ||
			strings.Contains(errStr, "bound"),
			"error should mention memory access issue: %s", err.Error())
	})

	t.Run("invalid_utf8", func(t *testing.T) {
		mem := newMockMemory(64)
		// Invalid UTF-8 sequence
		copy(mem.data[16:], []byte{0xFF})
		binary.LittleEndian.PutUint32(mem.data[0:], 16)
		binary.LittleEndian.PutUint32(mem.data[4:], 1)

		ctx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		}

		_, err := abi.LiftString(ctx, 0)
		require.Error(t, err)
		// Error should indicate UTF-8 problem
	})
}

// TestErrorMessages_ReallocFailure tests realloc failure error messages.
func TestErrorMessages_ReallocFailure(t *testing.T) {
	t.Run("string_realloc_failure", func(t *testing.T) {
		ctx := &abi.LowerContext{
			Memory: newMockMemory(1024),
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
			Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
				return 0, errors.New("custom allocation error")
			},
		}

		_, _, err := abi.LowerString(ctx, "test")
		require.Error(t, err)
		errStr := err.Error()
		require.True(t, strings.Contains(errStr, "realloc") ||
			strings.Contains(errStr, "alloc"),
			"error should mention realloc: %s", errStr)
	})

	t.Run("list_realloc_failure", func(t *testing.T) {
		ctx := &abi.LowerContext{
			Memory: newMockMemory(1024),
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
			Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
				return 0, errors.New("allocation failed")
			},
		}

		listType := types.List{Element: types.U32{}}
		val := types.ValList([]types.Val{types.ValU32(1)})

		_, err := abi.LowerFlat(ctx, listType, val)
		require.Error(t, err)
		errStr := err.Error()
		require.True(t, strings.Contains(errStr, "realloc"),
			"error should mention realloc: %s", errStr)
	})
}

// TestErrorMessages_ResourceTable tests resource table error messages.
func TestErrorMessages_ResourceTable(t *testing.T) {
	t.Run("invalid_handle", func(t *testing.T) {
		table := runtime.NewResourceTable()
		h := runtime.MakeHandle(100, 0)

		_, err := table.Get(h)
		require.Error(t, err)
		errStr := strings.ToLower(err.Error())
		require.True(t, strings.Contains(errStr, "invalid") ||
			strings.Contains(errStr, "handle"),
			"error should mention invalid handle: %s", err.Error())
	})

	t.Run("generation_mismatch", func(t *testing.T) {
		table := runtime.NewResourceTable()
		h1 := table.New("resource", true)
		table.Remove(h1)
		table.New("new_resource", true) // Reuses slot with new generation

		_, err := table.Get(h1)
		require.Error(t, err)
		errStr := strings.ToLower(err.Error())
		require.True(t, strings.Contains(errStr, "generation"),
			"error should mention generation: %s", err.Error())
	})

	t.Run("resource_in_use", func(t *testing.T) {
		table := runtime.NewResourceTable()
		h := table.New("resource", true)
		table.IncrementLends(h)

		_, err := table.Remove(h)
		require.Error(t, err)
		errStr := strings.ToLower(err.Error())
		require.True(t, strings.Contains(errStr, "borrow") ||
			strings.Contains(errStr, "use") ||
			strings.Contains(errStr, "lend"),
			"error should mention active borrows: %s", err.Error())
	})
}

// TestErrorMessages_ListBounds tests list bounds error messages.
func TestErrorMessages_ListBounds(t *testing.T) {
	mem := newMockMemory(64)
	listType := types.List{Element: types.U32{}}

	t.Run("out_of_bounds_list", func(t *testing.T) {
		ctx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		}

		// List pointing beyond memory
		iter := abi.NewFlatIter([]uint64{100, 10})
		_, err := abi.LiftFlat(ctx, listType, iter)
		require.Error(t, err)
		errStr := strings.ToLower(err.Error())
		require.True(t, strings.Contains(errStr, "bound") ||
			strings.Contains(errStr, "memory") ||
			strings.Contains(errStr, "exceed"),
			"error should mention bounds issue: %s", err.Error())
	})
}

// TestErrorMessages_WriteFailure tests memory write failure error messages.
func TestErrorMessages_WriteFailure(t *testing.T) {
	smallMem := newMockMemory(16)

	ctx := &abi.LowerContext{
		Memory: smallMem,
		Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			return 100, nil // Return pointer beyond memory
		},
	}

	_, _, err := abi.LowerString(ctx, "test")
	require.Error(t, err)
	errStr := strings.ToLower(err.Error())
	require.True(t, strings.Contains(errStr, "write") ||
		strings.Contains(errStr, "failed"),
		"error should mention write failure: %s", err.Error())
}

// TestErrorMessages_UnknownEncoding tests unknown string encoding errors.
func TestErrorMessages_UnknownEncoding(t *testing.T) {
	ctx := &abi.LowerContext{
		Memory: newMockMemory(64),
		Opts:   &abi.Options{StringEncoding: abi.StringEncoding(99)},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			return 0, nil
		},
	}

	_, _, err := abi.LowerString(ctx, "test")
	require.Error(t, err)
	errStr := strings.ToLower(err.Error())
	require.True(t, strings.Contains(errStr, "encoding") ||
		strings.Contains(errStr, "unknown"),
		"error should mention unknown encoding: %s", err.Error())
}

// TestErrorMessages_ContextRequired tests context requirement error messages.
func TestErrorMessages_ContextRequired(t *testing.T) {
	t.Run("list_without_memory_context", func(t *testing.T) {
		listType := types.List{Element: types.U32{}}
		// Non-empty list without memory context
		iter := abi.NewFlatIter([]uint64{100, 5})
		_, err := abi.LiftFlat(nil, listType, iter)
		require.Error(t, err)
		errStr := strings.ToLower(err.Error())
		require.True(t, strings.Contains(errStr, "memory") ||
			strings.Contains(errStr, "context") ||
			strings.Contains(errStr, "required"),
			"error should mention context requirement: %s", err.Error())
	})

	t.Run("list_lower_without_realloc", func(t *testing.T) {
		listType := types.List{Element: types.U32{}}
		val := types.ValList([]types.Val{types.ValU32(1)})

		// No realloc function provided
		ctx := &abi.LowerContext{
			Memory:  newMockMemory(1024),
			Opts:    &abi.Options{StringEncoding: abi.StringEncodingUTF8},
			Realloc: nil,
		}

		_, err := abi.LowerFlat(ctx, listType, val)
		require.Error(t, err)
		errStr := strings.ToLower(err.Error())
		require.True(t, strings.Contains(errStr, "realloc"),
			"error should mention realloc requirement: %s", err.Error())
	})
}

// TestErrorMessages_NestedErrors tests that nested errors provide context.
func TestErrorMessages_NestedErrors(t *testing.T) {
	t.Run("record_field_error", func(t *testing.T) {
		recordType := types.Record{
			Fields: []types.Field{
				{Name: "ch", Type: types.Char{}},
			},
		}

		// Invalid char value in a record field
		iter := abi.NewFlatIter([]uint64{0xD800})
		_, err := abi.LiftFlat(nil, recordType, iter)
		require.Error(t, err)
		errStr := err.Error()
		// Error should indicate both the field name and the underlying issue
		require.True(t, strings.Contains(errStr, "ch") ||
			strings.Contains(errStr, "field"),
			"error should mention the field: %s", errStr)
	})

	t.Run("tuple_element_error", func(t *testing.T) {
		tupleType := types.Tuple{
			Types: []types.ValType{types.U32{}, types.Char{}},
		}

		// Invalid char in second element
		iter := abi.NewFlatIter([]uint64{42, 0xD800})
		_, err := abi.LiftFlat(nil, tupleType, iter)
		require.Error(t, err)
		errStr := err.Error()
		require.True(t, strings.Contains(errStr, "element") ||
			strings.Contains(errStr, "tuple") ||
			strings.Contains(errStr, "1"),
			"error should mention tuple element: %s", errStr)
	})

	t.Run("variant_payload_error", func(t *testing.T) {
		variantType := types.Variant{
			Cases: []types.Case{
				{Name: "char_case", Type: types.Char{}},
			},
		}

		// Invalid char in variant payload
		iter := abi.NewFlatIter([]uint64{0, 0xD800})
		_, err := abi.LiftFlat(nil, variantType, iter)
		require.Error(t, err)
		errStr := err.Error()
		require.True(t, strings.Contains(errStr, "variant") ||
			strings.Contains(errStr, "payload"),
			"error should mention variant payload: %s", errStr)
	})
}
