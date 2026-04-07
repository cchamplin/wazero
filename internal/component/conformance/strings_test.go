// Package conformance contains conformance tests for the Component Model implementation.
// String tests ported from wasmtime's tests/all/component_model/strings.rs
package conformance

import (
	"encoding/binary"
	"errors"
	"testing"
	"unicode/utf16"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/abi"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// mockMemory implements abi.Memory for testing.
type mockMemory struct {
	data []byte
}

func (m *mockMemory) Read(offset, size uint32) ([]byte, bool) {
	// Check for overflow in offset+size calculation
	end := uint64(offset) + uint64(size)
	if end > uint64(len(m.data)) {
		return nil, false
	}
	return m.data[offset : offset+size], true
}

func (m *mockMemory) Write(offset uint32, data []byte) bool {
	// Check for overflow in offset+len(data) calculation
	end := uint64(offset) + uint64(len(data))
	if end > uint64(len(m.data)) {
		return false
	}
	copy(m.data[offset:], data)
	return true
}

func (m *mockMemory) Size() uint32 {
	return uint32(len(m.data))
}

// newMockMemory creates a new mock memory with the given size.
func newMockMemory(size int) *mockMemory {
	return &mockMemory{data: make([]byte, size)}
}

// testStrings are the strings to test, ported from wasmtime's strings.rs STRINGS constant.
var testStrings = []struct {
	name string
	s    string
}{
	// Empty string
	{"empty", ""},
	// Single ASCII character
	{"single_ascii", "x"},
	// ASCII string (hello world variant)
	{"ascii", "hello world"},
	// Long ASCII string
	{"long_ascii", "hello this is a particularly long string yes it is it keeps going"},
	// Latin-1 extended characters (0xE0-0xEB range)
	{"latin1_extended", "\u00E0 \u00E1 \u00E2 \u00E3 \u00E4 \u00E5 \u00E6 \u00E7 \u00E8 \u00E9 \u00EA \u00EB"},
	// Greek letters (require 2 bytes in UTF-8, but fit in BMP)
	{"greek", "\u03B1\u03B2\u03B3\u03B4\u03B5\u03B6\u03B7\u03B8"},
	// Greek letters with spaces (from wasmtime)
	{"greek_spaced", "\u039E \u039F \u03A0 \u03A1 \u03A3 \u03A4 \u03A5 \u03A6 \u03A7 \u03A8 \u03A9 \u03AA \u03AB \u03AC \u03AD \u03AE"},
	// Full-width characters (require 3 bytes in UTF-8)
	{"fullwidth", "\uFF33\uFF34\uFF35\uFF36\uFF37\uFF38\uFF39\uFF3A"},
	// Full-width space
	{"fullwidth_space", "\u3000"},
	// 4-byte UTF-8 / Supplementary plane character (emoji and beyond)
	{"supplementary", "\U00010000"},
	// Mixed content with emoji
	{"mixed_emoji", "hello \U0001F600 world"},
	// Latin-1 compatible prefix with non-Latin-1 suffix
	{"latin1_prefix_unicode_suffix", "pr\u00E9fix\u00E9"},
	// More mixed content (from wasmtime)
	{"mixed", "\u00E0 ascii \uFF36\uFF37\uFF38\uFF39\uFF3A"},
	// Extended Latin (fits in Latin-1 for Latin1+UTF16 encoding)
	{"extended_latin", "\u00CB\u00CC\u00CD\u00CE\u00CF\u00D0\u00D1\u00D2"},
}

// TestStringsUTF8Roundtrip tests string lower/lift roundtrip using UTF-8 encoding.
func TestStringsUTF8Roundtrip(t *testing.T) {
	for _, tc := range testStrings {
		t.Run(tc.name, func(t *testing.T) {
			mem := newMockMemory(4096)
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

			// Lower the string
			ptr, length, err := abi.LowerString(lowerCtx, tc.s)
			require.NoError(t, err)

			// Lift it back
			liftCtx := &abi.LiftContext{
				Memory: mem,
				Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
			}

			// Use LiftFlat with ptr/len
			iter := abi.NewFlatIter([]uint64{uint64(ptr), uint64(length)})
			lifted, err := abi.LiftFlat(liftCtx, types.String{}, iter)
			require.NoError(t, err)
			require.Equal(t, types.ValKindString, lifted.Kind())
			require.Equal(t, tc.s, lifted.StringVal())
		})
	}
}

// TestStringsUTF16Roundtrip tests string lower/lift roundtrip using UTF-16 encoding.
func TestStringsUTF16Roundtrip(t *testing.T) {
	for _, tc := range testStrings {
		t.Run(tc.name, func(t *testing.T) {
			mem := newMockMemory(4096)
			allocPtr := uint32(256)

			lowerCtx := &abi.LowerContext{
				Memory: mem,
				Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF16},
				Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
					result := allocPtr
					allocPtr += newSize
					return result, nil
				},
			}

			// Lower the string
			ptr, codeUnits, err := abi.LowerString(lowerCtx, tc.s)
			require.NoError(t, err)

			// Lift it back
			liftCtx := &abi.LiftContext{
				Memory: mem,
				Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF16},
			}

			// Use LiftFlat with ptr/codeUnits
			iter := abi.NewFlatIter([]uint64{uint64(ptr), uint64(codeUnits)})
			lifted, err := abi.LiftFlat(liftCtx, types.String{}, iter)
			require.NoError(t, err)
			require.Equal(t, types.ValKindString, lifted.Kind())
			require.Equal(t, tc.s, lifted.StringVal())
		})
	}
}

// TestStringsLatin1UTF16Roundtrip tests string lower/lift roundtrip using Latin1+UTF16 encoding.
func TestStringsLatin1UTF16Roundtrip(t *testing.T) {
	for _, tc := range testStrings {
		t.Run(tc.name, func(t *testing.T) {
			mem := newMockMemory(4096)
			allocPtr := uint32(256)

			lowerCtx := &abi.LowerContext{
				Memory: mem,
				Opts:   &abi.Options{StringEncoding: abi.StringEncodingLatin1UTF16},
				Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
					result := allocPtr
					allocPtr += newSize
					return result, nil
				},
			}

			// Lower the string
			ptr, taggedLen, err := abi.LowerString(lowerCtx, tc.s)
			require.NoError(t, err)

			// Lift it back
			liftCtx := &abi.LiftContext{
				Memory: mem,
				Opts:   &abi.Options{StringEncoding: abi.StringEncodingLatin1UTF16},
			}

			// Use LiftFlat with ptr/taggedLen
			iter := abi.NewFlatIter([]uint64{uint64(ptr), uint64(taggedLen)})
			lifted, err := abi.LiftFlat(liftCtx, types.String{}, iter)
			require.NoError(t, err)
			require.Equal(t, types.ValKindString, lifted.Kind())
			require.Equal(t, tc.s, lifted.StringVal())
		})
	}
}

// TestStringsLatin1UTF16Compression tests that Latin-1 compatible strings are stored
// as Latin-1 (1 byte per char) rather than UTF-16 (2 bytes per char).
func TestStringsLatin1UTF16Compression(t *testing.T) {
	latin1Only := []string{
		"",
		"hello",
		"\u00E0\u00E1\u00E2\u00E3", // Latin-1 extended chars
		"\u00CB\u00CC\u00CD\u00CE", // More Latin-1
	}

	for _, s := range latin1Only {
		t.Run("latin1_compression", func(t *testing.T) {
			mem := newMockMemory(1024)
			allocPtr := uint32(64)

			lowerCtx := &abi.LowerContext{
				Memory: mem,
				Opts:   &abi.Options{StringEncoding: abi.StringEncodingLatin1UTF16},
				Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
					result := allocPtr
					allocPtr += newSize
					return result, nil
				},
			}

			_, taggedLen, err := abi.LowerString(lowerCtx, s)
			require.NoError(t, err)

			// Verify the UTF-16 tag bit is NOT set (bit 31)
			utf16Tag := uint32(1 << 31)
			if s != "" {
				require.Equal(t, uint32(0), taggedLen&utf16Tag, "Latin-1 string should not have UTF-16 tag")
			}
		})
	}

	// Non-Latin-1 strings should use UTF-16
	nonLatin1 := []string{
		"\u03B1\u03B2\u03B3", // Greek
		"\u4E2D\u6587",       // Chinese
		"hello \U0001F600",   // Emoji
	}

	for _, s := range nonLatin1 {
		t.Run("utf16_fallback", func(t *testing.T) {
			mem := newMockMemory(1024)
			allocPtr := uint32(64)

			lowerCtx := &abi.LowerContext{
				Memory: mem,
				Opts:   &abi.Options{StringEncoding: abi.StringEncodingLatin1UTF16},
				Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
					result := allocPtr
					allocPtr += newSize
					return result, nil
				},
			}

			_, taggedLen, err := abi.LowerString(lowerCtx, s)
			require.NoError(t, err)

			// Verify the UTF-16 tag bit IS set (bit 31)
			utf16Tag := uint32(1 << 31)
			require.NotEqual(t, uint32(0), taggedLen&utf16Tag, "Non-Latin-1 string should have UTF-16 tag")
		})
	}
}

// TestStringsPtrOutOfBounds tests that lifting fails when the string pointer is out of bounds.
// Ported from wasmtime's ptr_out_of_bounds test.
func TestStringsPtrOutOfBounds(t *testing.T) {
	t.Run("ptr_beyond_memory", func(t *testing.T) {
		mem := newMockMemory(64)

		// Set up ptr/len pointing beyond memory
		binary.LittleEndian.PutUint32(mem.data[0:], 100) // ptr = 100 (beyond 64-byte memory)
		binary.LittleEndian.PutUint32(mem.data[4:], 5)   // len = 5

		liftCtx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		}

		_, err := abi.LiftString(liftCtx, 0)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to read")
	})

	t.Run("ptr_plus_len_beyond_memory", func(t *testing.T) {
		mem := newMockMemory(64)

		// ptr is valid but ptr + len goes beyond
		binary.LittleEndian.PutUint32(mem.data[0:], 60) // ptr = 60
		binary.LittleEndian.PutUint32(mem.data[4:], 10) // len = 10 (60+10 > 64)

		liftCtx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		}

		_, err := abi.LiftString(liftCtx, 0)
		require.Error(t, err)
	})

	t.Run("zero_length_valid_ptr", func(t *testing.T) {
		mem := newMockMemory(64)

		// Zero length should work even with invalid-ish ptr
		binary.LittleEndian.PutUint32(mem.data[0:], 100) // ptr = 100 (doesn't matter)
		binary.LittleEndian.PutUint32(mem.data[4:], 0)   // len = 0

		liftCtx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		}

		s, err := abi.LiftString(liftCtx, 0)
		require.NoError(t, err)
		require.Equal(t, "", s)
	})

	// UTF-16 variant
	t.Run("utf16_ptr_beyond_memory", func(t *testing.T) {
		mem := newMockMemory(64)

		binary.LittleEndian.PutUint32(mem.data[0:], 100) // ptr = 100
		binary.LittleEndian.PutUint32(mem.data[4:], 5)   // 5 code units = 10 bytes

		liftCtx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF16},
		}

		_, err := abi.LiftString(liftCtx, 0)
		require.Error(t, err)
	})
}

// TestStringsPtrOverflow tests that ptr+len overflow is handled correctly.
// Ported from wasmtime's ptr_overflow test.
func TestStringsPtrOverflow(t *testing.T) {
	t.Run("utf8_overflow", func(t *testing.T) {
		mem := newMockMemory(64)

		// Use values that would overflow when added
		binary.LittleEndian.PutUint32(mem.data[0:], 1<<31) // ptr = 2^31
		binary.LittleEndian.PutUint32(mem.data[4:], 1<<31) // len = 2^31

		liftCtx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		}

		_, err := abi.LiftString(liftCtx, 0)
		require.Error(t, err)
	})

	t.Run("utf16_code_units_overflow", func(t *testing.T) {
		mem := newMockMemory(64)

		// UTF-16 code units * 2 could overflow
		binary.LittleEndian.PutUint32(mem.data[0:], 0)
		binary.LittleEndian.PutUint32(mem.data[4:], 0x80000000) // Would be 4GB in bytes

		liftCtx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF16},
		}

		_, err := abi.LiftString(liftCtx, 0)
		require.Error(t, err)
	})
}

// TestStringsEmptyHandling tests various empty string scenarios.
func TestStringsEmptyHandling(t *testing.T) {
	encodings := []struct {
		name string
		enc  abi.StringEncoding
	}{
		{"utf8", abi.StringEncodingUTF8},
		{"utf16", abi.StringEncodingUTF16},
		{"latin1utf16", abi.StringEncodingLatin1UTF16},
	}

	for _, enc := range encodings {
		t.Run(enc.name+"_empty_lower", func(t *testing.T) {
			mem := newMockMemory(64)
			reallocCalled := false

			lowerCtx := &abi.LowerContext{
				Memory: mem,
				Opts:   &abi.Options{StringEncoding: enc.enc},
				Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
					reallocCalled = true
					return 0, nil
				},
			}

			ptr, length, err := abi.LowerString(lowerCtx, "")
			require.NoError(t, err)
			require.Equal(t, uint32(0), ptr)
			require.Equal(t, uint32(0), length)
			require.False(t, reallocCalled, "Realloc should not be called for empty string")
		})

		t.Run(enc.name+"_empty_lift", func(t *testing.T) {
			mem := newMockMemory(64)

			liftCtx := &abi.LiftContext{
				Memory: mem,
				Opts:   &abi.Options{StringEncoding: enc.enc},
			}

			iter := abi.NewFlatIter([]uint64{0, 0})
			lifted, err := abi.LiftFlat(liftCtx, types.String{}, iter)
			require.NoError(t, err)
			require.Equal(t, "", lifted.StringVal())
		})
	}
}

// TestStringsInvalidUTF8 tests that invalid UTF-8 sequences are rejected during lift.
func TestStringsInvalidUTF8(t *testing.T) {
	invalidSequences := []struct {
		name string
		data []byte
	}{
		{"single_ff", []byte{0xFF}},
		{"incomplete_2byte", []byte{0xC2}},             // Start of 2-byte sequence without continuation
		{"incomplete_3byte", []byte{0xE0, 0x80}},       // Start of 3-byte without full continuation
		{"invalid_continuation", []byte{0x80}},         // Continuation byte without starter
		{"overlong_null", []byte{0xC0, 0x80}},          // Overlong encoding of NUL
		{"invalid_f5", []byte{0xF5, 0x80, 0x80, 0x80}}, // Beyond valid Unicode
	}

	for _, tc := range invalidSequences {
		t.Run(tc.name, func(t *testing.T) {
			mem := newMockMemory(64)
			copy(mem.data[16:], tc.data)

			binary.LittleEndian.PutUint32(mem.data[0:], 16)
			binary.LittleEndian.PutUint32(mem.data[4:], uint32(len(tc.data)))

			liftCtx := &abi.LiftContext{
				Memory: mem,
				Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
			}

			_, err := abi.LiftString(liftCtx, 0)
			require.Error(t, err, "Should reject invalid UTF-8 sequence: %s", tc.name)
		})
	}
}

// TestStringsUTF16Surrogates tests UTF-16 surrogate pair handling.
func TestStringsUTF16Surrogates(t *testing.T) {
	t.Run("valid_surrogate_pair", func(t *testing.T) {
		mem := newMockMemory(64)

		// U+10000 encoded as surrogate pair: D800 DC00
		binary.LittleEndian.PutUint16(mem.data[16:], 0xD800)
		binary.LittleEndian.PutUint16(mem.data[18:], 0xDC00)

		binary.LittleEndian.PutUint32(mem.data[0:], 16)
		binary.LittleEndian.PutUint32(mem.data[4:], 2) // 2 code units

		liftCtx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF16},
		}

		s, err := abi.LiftString(liftCtx, 0)
		require.NoError(t, err)
		require.Equal(t, "\U00010000", s)
	})

	t.Run("emoji_surrogate_pair", func(t *testing.T) {
		mem := newMockMemory(64)

		// U+1F600 (grinning face) encoded as surrogate pair: D83D DE00
		binary.LittleEndian.PutUint16(mem.data[16:], 0xD83D)
		binary.LittleEndian.PutUint16(mem.data[18:], 0xDE00)

		binary.LittleEndian.PutUint32(mem.data[0:], 16)
		binary.LittleEndian.PutUint32(mem.data[4:], 2)

		liftCtx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF16},
		}

		s, err := abi.LiftString(liftCtx, 0)
		require.NoError(t, err)
		require.Equal(t, "\U0001F600", s)
	})
}

// TestStringsCrossEncodingConversion tests converting strings between different encodings.
func TestStringsCrossEncodingConversion(t *testing.T) {
	encodings := []abi.StringEncoding{
		abi.StringEncodingUTF8,
		abi.StringEncodingUTF16,
		abi.StringEncodingLatin1UTF16,
	}

	for _, srcEnc := range encodings {
		for _, dstEnc := range encodings {
			testName := encName(srcEnc) + "_to_" + encName(dstEnc)
			t.Run(testName, func(t *testing.T) {
				for _, tc := range testStrings {
					mem := newMockMemory(8192)
					allocPtr := uint32(512)

					// Lower with source encoding
					lowerCtx := &abi.LowerContext{
						Memory: mem,
						Opts:   &abi.Options{StringEncoding: srcEnc},
						Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
							result := allocPtr
							allocPtr += newSize
							return result, nil
						},
					}

					ptr, taggedLen, err := abi.LowerString(lowerCtx, tc.s)
					require.NoError(t, err)

					// Lift with same encoding (to verify lowered data is correct)
					liftCtx := &abi.LiftContext{
						Memory: mem,
						Opts:   &abi.Options{StringEncoding: srcEnc},
					}

					iter := abi.NewFlatIter([]uint64{uint64(ptr), uint64(taggedLen)})
					lifted, err := abi.LiftFlat(liftCtx, types.String{}, iter)
					require.NoError(t, err)
					require.Equal(t, tc.s, lifted.StringVal())

					// Now lower with destination encoding
					allocPtr = uint32(4096)
					lowerCtx2 := &abi.LowerContext{
						Memory: mem,
						Opts:   &abi.Options{StringEncoding: dstEnc},
						Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
							result := allocPtr
							allocPtr += newSize
							return result, nil
						},
					}

					ptr2, taggedLen2, err := abi.LowerString(lowerCtx2, lifted.StringVal())
					require.NoError(t, err)

					// Lift with destination encoding
					liftCtx2 := &abi.LiftContext{
						Memory: mem,
						Opts:   &abi.Options{StringEncoding: dstEnc},
					}

					iter2 := abi.NewFlatIter([]uint64{uint64(ptr2), uint64(taggedLen2)})
					lifted2, err := abi.LiftFlat(liftCtx2, types.String{}, iter2)
					require.NoError(t, err)
					require.Equal(t, tc.s, lifted2.StringVal(), "Cross-encoding roundtrip failed for %q", tc.s)
				}
			})
		}
	}
}

func encName(enc abi.StringEncoding) string {
	switch enc {
	case abi.StringEncodingUTF8:
		return "utf8"
	case abi.StringEncodingUTF16:
		return "utf16"
	case abi.StringEncodingLatin1UTF16:
		return "latin1utf16"
	default:
		return "unknown"
	}
}

// TestStringsTypeProperties verifies string type has correct ABI properties.
func TestStringsTypeProperties(t *testing.T) {
	strType := types.String{}
	require.Equal(t, uint32(8), strType.Size(), "String size should be 8 (ptr + len)")
	require.Equal(t, uint32(4), strType.Align(), "String alignment should be 4")
	require.Equal(t, 2, strType.FlattenCount(), "String should flatten to 2 values (ptr, len)")
}

// TestStringsLowerFlatRoundtrip tests LowerFlat/LiftFlat for strings.
func TestStringsLowerFlatRoundtrip(t *testing.T) {
	for _, tc := range testStrings {
		t.Run(tc.name, func(t *testing.T) {
			mem := newMockMemory(4096)
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

			val := types.ValString(tc.s)
			flat, err := abi.LowerFlat(ctx, types.String{}, val)
			require.NoError(t, err)
			require.Equal(t, 2, len(flat), "String should flatten to 2 values")

			// Lift back
			liftCtx := &abi.LiftContext{
				Memory: mem,
				Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
			}

			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(liftCtx, types.String{}, iter)
			require.NoError(t, err)
			require.Equal(t, tc.s, lifted.StringVal())
		})
	}
}

// TestStringsReallocError tests that realloc errors are properly propagated.
func TestStringsReallocError(t *testing.T) {
	mem := newMockMemory(64)
	ctx := &abi.LowerContext{
		Memory: mem,
		Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			return 0, errors.New("allocation failed") // Simulate allocation failure
		},
	}

	_, _, err := abi.LowerString(ctx, "test")
	require.Error(t, err)
	require.Contains(t, err.Error(), "realloc")
}

// TestStringsWriteError tests that memory write errors are properly propagated.
func TestStringsWriteError(t *testing.T) {
	mem := newMockMemory(16) // Small memory
	ctx := &abi.LowerContext{
		Memory: mem,
		Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			return 100, nil // Return pointer beyond memory
		},
	}

	_, _, err := abi.LowerString(ctx, "test")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to write")
}

// TestStringsUnknownEncoding tests that unknown encoding returns an error.
func TestStringsUnknownEncoding(t *testing.T) {
	mem := newMockMemory(64)

	t.Run("lower", func(t *testing.T) {
		ctx := &abi.LowerContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncoding(99)},
			Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
				return 0, nil
			},
		}

		_, _, err := abi.LowerString(ctx, "test")
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown string encoding")
	})
}

// TestStringsUTF16ByteLength tests that UTF-16 length is in code units, not bytes.
func TestStringsUTF16ByteLength(t *testing.T) {
	testCases := []struct {
		name      string
		s         string
		codeUnits int
	}{
		{"empty", "", 0},
		{"ascii", "hello", 5},
		{"bmp_only", "\u4E2D\u6587", 2},     // Chinese chars, 2 BMP code points
		{"with_surrogate", "\U0001F600", 2}, // Emoji requires surrogate pair
		{"mixed", "a\U0001F600b", 4},        // a + surrogate pair + b
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mem := newMockMemory(1024)
			allocPtr := uint32(64)

			ctx := &abi.LowerContext{
				Memory: mem,
				Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF16},
				Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
					result := allocPtr
					allocPtr += newSize
					return result, nil
				},
			}

			_, codeUnits, err := abi.LowerString(ctx, tc.s)
			require.NoError(t, err)
			require.Equal(t, uint32(tc.codeUnits), codeUnits, "Code unit count mismatch for %q", tc.s)

			// Verify against Go's utf16.Encode
			if tc.s != "" {
				runes := []rune(tc.s)
				u16 := utf16.Encode(runes)
				require.Equal(t, len(u16), tc.codeUnits, "Code unit count should match utf16.Encode")
			}
		})
	}
}

// TestStringsLatin1UTF16TagBit tests the tag bit behavior in Latin1+UTF16 encoding.
func TestStringsLatin1UTF16TagBit(t *testing.T) {
	utf16Tag := uint32(1 << 31)

	t.Run("lift_with_tag", func(t *testing.T) {
		mem := newMockMemory(64)

		// Write UTF-16 "AB" at offset 16
		binary.LittleEndian.PutUint16(mem.data[16:], 0x0041) // 'A'
		binary.LittleEndian.PutUint16(mem.data[18:], 0x0042) // 'B'

		binary.LittleEndian.PutUint32(mem.data[0:], 16)
		binary.LittleEndian.PutUint32(mem.data[4:], 2|utf16Tag) // 2 code units with tag

		ctx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingLatin1UTF16},
		}

		s, err := abi.LiftString(ctx, 0)
		require.NoError(t, err)
		require.Equal(t, "AB", s)
	})

	t.Run("lift_without_tag", func(t *testing.T) {
		mem := newMockMemory(64)

		// Write Latin-1 "AB" at offset 16
		mem.data[16] = 0x41 // 'A'
		mem.data[17] = 0x42 // 'B'

		binary.LittleEndian.PutUint32(mem.data[0:], 16)
		binary.LittleEndian.PutUint32(mem.data[4:], 2) // 2 bytes, no tag

		ctx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingLatin1UTF16},
		}

		s, err := abi.LiftString(ctx, 0)
		require.NoError(t, err)
		require.Equal(t, "AB", s)
	})

	t.Run("latin1_extended_chars", func(t *testing.T) {
		mem := newMockMemory(64)

		// Write Latin-1 extended characters (0xE0-0xE3 = a-grave to a-tilde)
		mem.data[16] = 0xE0
		mem.data[17] = 0xE1
		mem.data[18] = 0xE2
		mem.data[19] = 0xE3

		binary.LittleEndian.PutUint32(mem.data[0:], 16)
		binary.LittleEndian.PutUint32(mem.data[4:], 4) // 4 bytes, no tag

		ctx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingLatin1UTF16},
		}

		s, err := abi.LiftString(ctx, 0)
		require.NoError(t, err)
		require.Equal(t, "\u00E0\u00E1\u00E2\u00E3", s)
	})
}

// TestStringsHeapOperations tests LiftHeap/LowerHeap for strings.
func TestStringsHeapOperations(t *testing.T) {
	t.Run("lift_heap", func(t *testing.T) {
		mem := newMockMemory(64)
		copy(mem.data[16:], "hello")

		// Write ptr/len at offset 0
		binary.LittleEndian.PutUint32(mem.data[0:], 16) // ptr
		binary.LittleEndian.PutUint32(mem.data[4:], 5)  // len

		ctx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		}

		val, err := abi.LiftHeap(ctx, types.String{}, 0)
		require.NoError(t, err)
		require.Equal(t, types.ValKindString, val.Kind())
		require.Equal(t, "hello", val.StringVal())
	})

	t.Run("lower_heap", func(t *testing.T) {
		mem := newMockMemory(128)
		allocPtr := uint32(64)

		ctx := &abi.LowerContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
			Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
				result := allocPtr
				allocPtr += newSize
				return result, nil
			},
		}

		val := types.ValString("world")
		err := abi.LowerHeap(ctx, types.String{}, val, 0)
		require.NoError(t, err)

		// Verify ptr/len were written at offset 0
		ptr := binary.LittleEndian.Uint32(mem.data[0:])
		length := binary.LittleEndian.Uint32(mem.data[4:])
		require.Equal(t, uint32(64), ptr)
		require.Equal(t, uint32(5), length)

		// Verify string data was written
		require.Equal(t, "world", string(mem.data[64:69]))
	})
}
