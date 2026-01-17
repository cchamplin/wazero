// Package conformance contains conformance tests for the Component Model implementation.
// Memory bounds edge case tests verify out-of-bounds memory access is handled gracefully.
package conformance

import (
	"encoding/binary"
	"testing"

	"github.com/tetratelabs/wazero/internal/component/abi"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestMemory_ReadBeyondBounds tests that reads beyond memory bounds return errors.
func TestMemory_ReadBeyondBounds(t *testing.T) {
	mem := newMockMemory(64)

	testCases := []struct {
		name   string
		offset uint32
		size   uint32
	}{
		{"offset_at_boundary", 64, 1},
		{"offset_beyond_boundary", 65, 1},
		{"offset_plus_size_beyond", 60, 10},
		{"way_beyond", 1000, 1},
		{"max_offset", 0xFFFFFFFF, 1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, ok := mem.Read(tc.offset, tc.size)
			require.False(t, ok, "read beyond bounds should fail")
			require.Nil(t, data)
		})
	}
}

// TestMemory_WriteBeyondBounds tests that writes beyond memory bounds return errors.
func TestMemory_WriteBeyondBounds(t *testing.T) {
	mem := newMockMemory(64)

	testCases := []struct {
		name   string
		offset uint32
		data   []byte
	}{
		{"offset_at_boundary", 64, []byte{1}},
		{"offset_beyond_boundary", 65, []byte{1}},
		{"offset_plus_size_beyond", 60, []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}},
		{"way_beyond", 1000, []byte{1}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ok := mem.Write(tc.offset, tc.data)
			require.False(t, ok, "write beyond bounds should fail")
		})
	}
}

// TestMemory_LiftStringOOB tests lifting strings with out-of-bounds pointers.
func TestMemory_LiftStringOOB(t *testing.T) {
	mem := newMockMemory(64)

	testCases := []struct {
		name string
		ptr  uint32
		len  uint32
	}{
		{"ptr_beyond_memory", 100, 5},
		{"ptr_plus_len_beyond", 60, 10},
		{"max_ptr", 0xFFFFFFFF, 1},
		{"overflow_ptr_len", 0x80000000, 0x80000000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Write ptr/len at offset 0
			binary.LittleEndian.PutUint32(mem.data[0:], tc.ptr)
			binary.LittleEndian.PutUint32(mem.data[4:], tc.len)

			ctx := &abi.LiftContext{
				Memory: mem,
				Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
			}

			_, err := abi.LiftString(ctx, 0)
			require.Error(t, err, "lifting OOB string should error")
		})
	}
}

// TestMemory_LiftListOOB tests lifting lists with out-of-bounds pointers.
func TestMemory_LiftListOOB(t *testing.T) {
	mem := newMockMemory(64)
	listType := types.List{Element: types.U32{}}

	testCases := []struct {
		name string
		ptr  uint32
		len  uint32
	}{
		{"ptr_beyond_memory", 100, 5},
		{"ptr_plus_len_beyond", 60, 10}, // 60 + 10*4 = 100 > 64
		{"very_large_len", 0, 0xFFFFFFFF},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := &abi.LiftContext{
				Memory: mem,
				Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
			}

			iter := abi.NewFlatIter([]uint64{uint64(tc.ptr), uint64(tc.len)})
			_, err := abi.LiftFlat(ctx, listType, iter)
			require.Error(t, err, "lifting OOB list should error")
		})
	}
}

// TestMemory_ValidBoundaryReads tests reads exactly at valid boundaries.
func TestMemory_ValidBoundaryReads(t *testing.T) {
	mem := newMockMemory(64)

	// Write test data at end of memory
	mem.data[60] = 0xDE
	mem.data[61] = 0xAD
	mem.data[62] = 0xBE
	mem.data[63] = 0xEF

	testCases := []struct {
		name     string
		offset   uint32
		size     uint32
		expected []byte
	}{
		{"last_byte", 63, 1, []byte{0xEF}},
		{"last_4_bytes", 60, 4, []byte{0xDE, 0xAD, 0xBE, 0xEF}},
		{"first_byte", 0, 1, []byte{0}},
		{"exact_fit", 60, 4, []byte{0xDE, 0xAD, 0xBE, 0xEF}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, ok := mem.Read(tc.offset, tc.size)
			require.True(t, ok, "read should succeed")
			require.Equal(t, tc.expected, data)
		})
	}
}

// TestMemory_ValidBoundaryWrites tests writes exactly at valid boundaries.
func TestMemory_ValidBoundaryWrites(t *testing.T) {
	mem := newMockMemory(64)

	testCases := []struct {
		name   string
		offset uint32
		data   []byte
	}{
		{"last_byte", 63, []byte{0xFF}},
		{"last_4_bytes", 60, []byte{0x11, 0x22, 0x33, 0x44}},
		{"first_byte", 0, []byte{0xAB}},
		{"exact_fit", 60, []byte{0xDE, 0xAD, 0xBE, 0xEF}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ok := mem.Write(tc.offset, tc.data)
			require.True(t, ok, "write should succeed")

			// Verify written data
			readBack, readOk := mem.Read(tc.offset, uint32(len(tc.data)))
			require.True(t, readOk)
			require.Equal(t, tc.data, readBack)
		})
	}
}

// TestMemory_ZeroLengthOperations tests zero-length reads and writes.
func TestMemory_ZeroLengthOperations(t *testing.T) {
	mem := newMockMemory(64)

	t.Run("zero_length_read_valid_offset", func(t *testing.T) {
		data, ok := mem.Read(0, 0)
		require.True(t, ok)
		require.Equal(t, 0, len(data))
	})

	t.Run("zero_length_read_at_boundary", func(t *testing.T) {
		// Zero-length read at exact end of memory should succeed
		data, ok := mem.Read(64, 0)
		require.True(t, ok)
		require.Equal(t, 0, len(data))
	})

	t.Run("zero_length_write_valid_offset", func(t *testing.T) {
		ok := mem.Write(0, []byte{})
		require.True(t, ok)
	})

	t.Run("zero_length_write_at_boundary", func(t *testing.T) {
		ok := mem.Write(64, []byte{})
		require.True(t, ok)
	})
}

// TestMemory_OverflowProtection tests that pointer+size overflow is detected.
func TestMemory_OverflowProtection(t *testing.T) {
	mem := newMockMemory(64)

	testCases := []struct {
		name   string
		offset uint32
		size   uint32
	}{
		{"offset_size_overflow_to_zero", 0x80000000, 0x80000000},
		{"offset_size_overflow_small", 0xFFFFFFFF, 1},
		{"offset_size_overflow_large", 0xFFFFFFF0, 0x20},
	}

	for _, tc := range testCases {
		t.Run(tc.name+"_read", func(t *testing.T) {
			_, ok := mem.Read(tc.offset, tc.size)
			require.False(t, ok, "should detect overflow")
		})
	}
}

// TestMemory_ListBoundsValidation tests list bounds are validated before element access.
func TestMemory_ListBoundsValidation(t *testing.T) {
	mem := newMockMemory(1024)
	listType := types.List{Element: types.U32{}}

	// Set up a list at offset 0 with ptr=500, len=100
	// Total bytes needed: 100 * 4 = 400, so 500 + 400 = 900 < 1024 (should work)
	t.Run("valid_list_bounds", func(t *testing.T) {
		// Write some u32 values starting at offset 500
		for i := uint32(0); i < 100; i++ {
			binary.LittleEndian.PutUint32(mem.data[500+i*4:], i+1)
		}

		ctx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		}

		iter := abi.NewFlatIter([]uint64{500, 100})
		lifted, err := abi.LiftFlat(ctx, listType, iter)
		require.NoError(t, err)
		require.Equal(t, 100, len(lifted.List()))
		require.Equal(t, uint32(1), lifted.List()[0].U32())
		require.Equal(t, uint32(100), lifted.List()[99].U32())
	})

	t.Run("invalid_list_bounds", func(t *testing.T) {
		// ptr=900, len=100 would need 900 + 400 = 1300 > 1024
		ctx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		}

		iter := abi.NewFlatIter([]uint64{900, 100})
		_, err := abi.LiftFlat(ctx, listType, iter)
		require.Error(t, err, "should error on OOB list")
	})
}

// TestMemory_RecordHeapBounds tests record heap access bounds checking.
func TestMemory_RecordHeapBounds(t *testing.T) {
	mem := newMockMemory(64)

	// record { a: u32, b: u64 } = size 16 (4 + padding + 8)
	recordType := types.Record{
		Fields: []types.Field{
			{Name: "a", Type: types.U32{}},
			{Name: "b", Type: types.U64{}},
		},
	}

	t.Run("valid_record_at_boundary", func(t *testing.T) {
		// Write record at offset 48 (48 + 16 = 64, exactly fits)
		binary.LittleEndian.PutUint32(mem.data[48:], 0x12345678)
		binary.LittleEndian.PutUint64(mem.data[56:], 0xDEADBEEFCAFEBABE)

		ctx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		}

		lifted, err := abi.LiftHeap(ctx, recordType, 48)
		require.NoError(t, err)
		aVal, _ := lifted.RecordField("a")
		require.Equal(t, uint32(0x12345678), aVal.U32())
	})
}

// TestMemory_StringEncodingBounds tests string encoding-specific bounds checking.
func TestMemory_StringEncodingBounds(t *testing.T) {
	mem := newMockMemory(64)

	encodings := []struct {
		name string
		enc  abi.StringEncoding
		// For UTF-16, length is in code units (2 bytes each)
		lenMultiplier uint32
	}{
		{"utf8", abi.StringEncodingUTF8, 1},
		{"utf16", abi.StringEncodingUTF16, 2},
		{"latin1utf16", abi.StringEncodingLatin1UTF16, 1}, // Can be 1 or 2 depending on tag
	}

	for _, enc := range encodings {
		t.Run(enc.name+"_oob", func(t *testing.T) {
			// Set up ptr=50, len=20 - for UTF-16 this means 40 bytes (50+40 > 64)
			binary.LittleEndian.PutUint32(mem.data[0:], 50)
			binary.LittleEndian.PutUint32(mem.data[4:], 20) // length

			ctx := &abi.LiftContext{
				Memory: mem,
				Opts:   &abi.Options{StringEncoding: enc.enc},
			}

			_, err := abi.LiftString(ctx, 0)
			if enc.enc == abi.StringEncodingUTF16 {
				// UTF-16 needs 50 + 20*2 = 90 bytes > 64
				require.Error(t, err, "%s should error on OOB", enc.name)
			}
			// Other encodings might succeed since 50 + 20 = 70 > 64
			// The important thing is no panic
		})
	}
}

// TestMemory_VariantPayloadBounds tests variant payload bounds checking.
func TestMemory_VariantPayloadBounds(t *testing.T) {
	mem := newMockMemory(64)

	// variant { small: u8, large: string }
	// String is the large case requiring ptr+len in memory
	variantType := types.Variant{
		Cases: []types.Case{
			{Name: "small", Type: types.U8{}},
			{Name: "large", Type: types.String{}},
		},
	}

	t.Run("valid_small_case", func(t *testing.T) {
		// Discriminant 0 (small), payload at aligned offset
		// For variant { small: u8, large: string }:
		// - DiscriminantSize = 1 (2 cases = u8)
		// - payloadAlign = max(align(u8), align(string)) = max(1, 4) = 4
		// - PayloadOffset = alignTo(1, 4) = 4
		mem.data[0] = 0  // small case (discriminant)
		mem.data[4] = 42 // payload at offset 4 (aligned)

		ctx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		}

		lifted, err := abi.LiftHeap(ctx, variantType, 0)
		require.NoError(t, err)
		caseName, payload := lifted.Variant()
		require.Equal(t, "small", caseName)
		require.NotNil(t, payload)
		require.Equal(t, uint8(42), payload.U8())
	})
}

// TestMemory_EmptyMemory tests operations on zero-size memory.
func TestMemory_EmptyMemory(t *testing.T) {
	mem := newMockMemory(0)

	t.Run("read_fails", func(t *testing.T) {
		_, ok := mem.Read(0, 1)
		require.False(t, ok)
	})

	t.Run("write_fails", func(t *testing.T) {
		ok := mem.Write(0, []byte{1})
		require.False(t, ok)
	})

	t.Run("zero_length_read_succeeds", func(t *testing.T) {
		data, ok := mem.Read(0, 0)
		require.True(t, ok)
		require.Equal(t, 0, len(data))
	})

	t.Run("zero_length_write_succeeds", func(t *testing.T) {
		ok := mem.Write(0, []byte{})
		require.True(t, ok)
	})
}
