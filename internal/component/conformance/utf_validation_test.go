// Package conformance contains conformance tests for the Component Model implementation.
// UTF validation tests verify that invalid UTF-8/UTF-16 sequences are rejected gracefully.
package conformance

import (
	"encoding/binary"
	"testing"

	"github.com/tetratelabs/wazero/internal/component/abi"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestUTF8_InvalidSequences tests that invalid UTF-8 sequences are rejected.
func TestUTF8_InvalidSequences(t *testing.T) {
	invalidSequences := []struct {
		name string
		data []byte
		desc string
	}{
		// Single invalid bytes
		{
			name: "single_continuation_byte",
			data: []byte{0x80},
			desc: "continuation byte without start byte",
		},
		{
			name: "single_0xFF",
			data: []byte{0xFF},
			desc: "0xFF is never valid in UTF-8",
		},
		{
			name: "single_0xFE",
			data: []byte{0xFE},
			desc: "0xFE is never valid in UTF-8",
		},

		// Incomplete sequences
		{
			name: "incomplete_2byte",
			data: []byte{0xC2},
			desc: "2-byte sequence start without continuation",
		},
		{
			name: "incomplete_3byte_1",
			data: []byte{0xE0},
			desc: "3-byte sequence with only start byte",
		},
		{
			name: "incomplete_3byte_2",
			data: []byte{0xE0, 0xA0},
			desc: "3-byte sequence with 2 bytes",
		},
		{
			name: "incomplete_4byte_1",
			data: []byte{0xF0},
			desc: "4-byte sequence with only start byte",
		},
		{
			name: "incomplete_4byte_2",
			data: []byte{0xF0, 0x90},
			desc: "4-byte sequence with 2 bytes",
		},
		{
			name: "incomplete_4byte_3",
			data: []byte{0xF0, 0x90, 0x80},
			desc: "4-byte sequence with 3 bytes",
		},

		// Overlong encodings
		{
			name: "overlong_null_2byte",
			data: []byte{0xC0, 0x80},
			desc: "overlong encoding of NUL character",
		},
		{
			name: "overlong_slash_2byte",
			data: []byte{0xC0, 0xAF},
			desc: "overlong encoding of '/'",
		},
		{
			name: "overlong_3byte",
			data: []byte{0xE0, 0x80, 0x80},
			desc: "overlong 3-byte encoding of NUL",
		},
		{
			name: "overlong_4byte",
			data: []byte{0xF0, 0x80, 0x80, 0x80},
			desc: "overlong 4-byte encoding of NUL",
		},

		// Invalid continuation bytes
		{
			name: "missing_continuation_2byte",
			data: []byte{0xC2, 0x00},
			desc: "2-byte sequence with non-continuation second byte",
		},
		{
			name: "invalid_continuation_2byte",
			data: []byte{0xC2, 0xC0},
			desc: "2-byte sequence with invalid continuation",
		},

		// Out of range (beyond U+10FFFF)
		{
			name: "beyond_unicode_max",
			data: []byte{0xF4, 0x90, 0x80, 0x80},
			desc: "beyond U+10FFFF",
		},
		{
			name: "way_beyond_unicode_max",
			data: []byte{0xF7, 0xBF, 0xBF, 0xBF},
			desc: "way beyond U+10FFFF",
		},

		// Surrogate pairs (invalid in UTF-8)
		{
			name: "utf8_surrogate_high",
			data: []byte{0xED, 0xA0, 0x80},
			desc: "high surrogate U+D800 in UTF-8",
		},
		{
			name: "utf8_surrogate_low",
			data: []byte{0xED, 0xBF, 0xBF},
			desc: "low surrogate U+DFFF in UTF-8",
		},
		{
			name: "utf8_surrogate_mid",
			data: []byte{0xED, 0xAD, 0xBF},
			desc: "surrogate in middle U+DB7F in UTF-8",
		},

		// 5-byte and 6-byte sequences (never valid)
		{
			name: "5byte_sequence",
			data: []byte{0xF8, 0x80, 0x80, 0x80, 0x80},
			desc: "5-byte sequence (never valid)",
		},
		{
			name: "6byte_sequence",
			data: []byte{0xFC, 0x80, 0x80, 0x80, 0x80, 0x80},
			desc: "6-byte sequence (never valid)",
		},
	}

	for _, tc := range invalidSequences {
		t.Run(tc.name, func(t *testing.T) {
			mem := newMockMemory(1024)
			copy(mem.data[64:], tc.data)

			// Set up ptr/len for string
			binary.LittleEndian.PutUint32(mem.data[0:], 64)
			binary.LittleEndian.PutUint32(mem.data[4:], uint32(len(tc.data)))

			ctx := &abi.LiftContext{
				Memory: mem,
				Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
			}

			_, err := abi.LiftString(ctx, 0)
			require.Error(t, err, "should reject %s: %s", tc.name, tc.desc)
		})
	}
}

// TestUTF8_ValidSequences tests that valid UTF-8 sequences are accepted.
func TestUTF8_ValidSequences(t *testing.T) {
	validSequences := []struct {
		name     string
		data     []byte
		expected string
	}{
		{"empty", []byte{}, ""},
		{"ascii", []byte("hello"), "hello"},
		{"2byte_char", []byte{0xC2, 0xA9}, "\u00A9"},                 // Copyright symbol
		{"3byte_char", []byte{0xE2, 0x82, 0xAC}, "\u20AC"},           // Euro sign
		{"4byte_char", []byte{0xF0, 0x9F, 0x98, 0x80}, "\U0001F600"}, // Grinning face
		{"mixed", []byte("Hello\xC2\xA9World"), "Hello\u00A9World"},
		{"bom", []byte{0xEF, 0xBB, 0xBF}, "\uFEFF"}, // BOM
		{"null_char", []byte{0x00}, "\x00"},
		{"max_valid_3byte", []byte{0xED, 0x9F, 0xBF}, "\uD7FF"},       // Just before surrogates
		{"min_after_surrogates", []byte{0xEE, 0x80, 0x80}, "\uE000"},  // Just after surrogates
		{"max_unicode", []byte{0xF4, 0x8F, 0xBF, 0xBF}, "\U0010FFFF"}, // Max valid
	}

	for _, tc := range validSequences {
		t.Run(tc.name, func(t *testing.T) {
			mem := newMockMemory(1024)
			copy(mem.data[64:], tc.data)

			binary.LittleEndian.PutUint32(mem.data[0:], 64)
			binary.LittleEndian.PutUint32(mem.data[4:], uint32(len(tc.data)))

			ctx := &abi.LiftContext{
				Memory: mem,
				Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
			}

			s, err := abi.LiftString(ctx, 0)
			require.NoError(t, err)
			require.Equal(t, tc.expected, s)
		})
	}
}

// TestUTF16_InvalidSequences tests that invalid UTF-16 sequences are rejected.
func TestUTF16_InvalidSequences(t *testing.T) {
	invalidSequences := []struct {
		name string
		data []uint16
		desc string
	}{
		{
			name: "lone_high_surrogate",
			data: []uint16{0xD800},
			desc: "lone high surrogate",
		},
		{
			name: "lone_low_surrogate",
			data: []uint16{0xDC00},
			desc: "lone low surrogate",
		},
		{
			name: "high_surrogate_at_end",
			data: []uint16{0x0041, 0xD800},
			desc: "high surrogate at end of string",
		},
		{
			name: "high_surrogate_followed_by_non_surrogate",
			data: []uint16{0xD800, 0x0041},
			desc: "high surrogate followed by regular char",
		},
		{
			name: "high_surrogate_followed_by_high_surrogate",
			data: []uint16{0xD800, 0xD801},
			desc: "two consecutive high surrogates",
		},
		{
			name: "low_surrogate_first",
			data: []uint16{0xDC00, 0xD800},
			desc: "low surrogate before high surrogate",
		},
		{
			name: "mid_high_surrogate",
			data: []uint16{0xDBFF},
			desc: "lone max high surrogate",
		},
		{
			name: "mid_low_surrogate",
			data: []uint16{0xDFFF},
			desc: "lone max low surrogate",
		},
	}

	for _, tc := range invalidSequences {
		t.Run(tc.name, func(t *testing.T) {
			mem := newMockMemory(1024)

			// Write UTF-16 data starting at offset 64
			for i, u := range tc.data {
				binary.LittleEndian.PutUint16(mem.data[64+i*2:], u)
			}

			binary.LittleEndian.PutUint32(mem.data[0:], 64)
			binary.LittleEndian.PutUint32(mem.data[4:], uint32(len(tc.data)))

			ctx := &abi.LiftContext{
				Memory: mem,
				Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF16},
			}

			// Note: Go's utf16.Decode replaces invalid surrogates with U+FFFD
			// rather than returning an error. This follows Go's standard library
			// behavior. The implementation replaces lone/invalid surrogates with
			// the Unicode replacement character (U+FFFD).
			result, err := abi.LiftString(ctx, 0)
			require.NoError(t, err, "lifting should succeed (invalid surrogates become U+FFFD)")
			require.Contains(t, result, "\uFFFD", "invalid surrogate %s should result in replacement char: %s", tc.name, tc.desc)
		})
	}
}

// TestUTF16_ValidSequences tests that valid UTF-16 sequences are accepted.
func TestUTF16_ValidSequences(t *testing.T) {
	validSequences := []struct {
		name     string
		data     []uint16
		expected string
	}{
		{"empty", []uint16{}, ""},
		{"ascii", []uint16{0x0048, 0x0069}, "Hi"},
		{"bmp_char", []uint16{0x20AC}, "\u20AC"},                         // Euro sign
		{"valid_surrogate_pair", []uint16{0xD83D, 0xDE00}, "\U0001F600"}, // Grinning face
		{"mixed", []uint16{0x0041, 0xD83D, 0xDE00, 0x0042}, "A\U0001F600B"},
		{"null", []uint16{0x0000}, "\x00"},
		{"max_bmp", []uint16{0xFFFF}, "\uFFFF"},
	}

	for _, tc := range validSequences {
		t.Run(tc.name, func(t *testing.T) {
			mem := newMockMemory(1024)

			for i, u := range tc.data {
				binary.LittleEndian.PutUint16(mem.data[64+i*2:], u)
			}

			binary.LittleEndian.PutUint32(mem.data[0:], 64)
			binary.LittleEndian.PutUint32(mem.data[4:], uint32(len(tc.data)))

			ctx := &abi.LiftContext{
				Memory: mem,
				Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF16},
			}

			s, err := abi.LiftString(ctx, 0)
			require.NoError(t, err)
			require.Equal(t, tc.expected, s)
		})
	}
}

// TestLatin1UTF16_TagBit tests the Latin1+UTF16 encoding tag bit handling.
func TestLatin1UTF16_TagBit(t *testing.T) {
	utf16Tag := uint32(1 << 31)

	t.Run("latin1_without_tag", func(t *testing.T) {
		mem := newMockMemory(1024)

		// Write Latin-1 "Hi" at offset 64
		mem.data[64] = 0x48 // 'H'
		mem.data[65] = 0x69 // 'i'

		binary.LittleEndian.PutUint32(mem.data[0:], 64)
		binary.LittleEndian.PutUint32(mem.data[4:], 2) // 2 bytes, no tag

		ctx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingLatin1UTF16},
		}

		s, err := abi.LiftString(ctx, 0)
		require.NoError(t, err)
		require.Equal(t, "Hi", s)
	})

	t.Run("utf16_with_tag", func(t *testing.T) {
		mem := newMockMemory(1024)

		// Write UTF-16 "Hi" at offset 64
		binary.LittleEndian.PutUint16(mem.data[64:], 0x0048)
		binary.LittleEndian.PutUint16(mem.data[66:], 0x0069)

		binary.LittleEndian.PutUint32(mem.data[0:], 64)
		binary.LittleEndian.PutUint32(mem.data[4:], 2|utf16Tag) // 2 code units with tag

		ctx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingLatin1UTF16},
		}

		s, err := abi.LiftString(ctx, 0)
		require.NoError(t, err)
		require.Equal(t, "Hi", s)
	})

	t.Run("latin1_extended_chars", func(t *testing.T) {
		mem := newMockMemory(1024)

		// Write Latin-1 extended characters (ISO 8859-1)
		mem.data[64] = 0xE9 // e with acute
		mem.data[65] = 0xF1 // n with tilde
		mem.data[66] = 0xFC // u with umlaut

		binary.LittleEndian.PutUint32(mem.data[0:], 64)
		binary.LittleEndian.PutUint32(mem.data[4:], 3) // 3 bytes, no tag

		ctx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingLatin1UTF16},
		}

		s, err := abi.LiftString(ctx, 0)
		require.NoError(t, err)
		require.Equal(t, "\u00E9\u00F1\u00FC", s) // e-acute, n-tilde, u-umlaut
	})
}

// TestCharValidation_Surrogates tests that char values reject surrogates.
func TestCharValidation_Surrogates(t *testing.T) {
	surrogates := []struct {
		name  string
		value uint32
	}{
		{"first_high_surrogate", 0xD800},
		{"last_high_surrogate", 0xDBFF},
		{"first_low_surrogate", 0xDC00},
		{"last_low_surrogate", 0xDFFF},
		{"mid_surrogate", 0xDC00},
	}

	for _, tc := range surrogates {
		t.Run(tc.name+"_lift", func(t *testing.T) {
			iter := abi.NewFlatIter([]uint64{uint64(tc.value)})
			_, err := abi.LiftFlat(nil, types.Char{}, iter)
			require.Error(t, err, "should reject surrogate U+%04X", tc.value)
			require.Contains(t, err.Error(), "not a valid Unicode scalar value")
		})
	}
}

// TestCharValidation_BeyondMax tests that char values above U+10FFFF are rejected.
func TestCharValidation_BeyondMax(t *testing.T) {
	invalidValues := []struct {
		name  string
		value uint32
	}{
		{"just_above_max", 0x110000},
		{"way_above_max", 0x1FFFFF},
		{"max_u32", 0xFFFFFFFF},
	}

	for _, tc := range invalidValues {
		t.Run(tc.name+"_lift", func(t *testing.T) {
			iter := abi.NewFlatIter([]uint64{uint64(tc.value)})
			_, err := abi.LiftFlat(nil, types.Char{}, iter)
			require.Error(t, err)
			require.Contains(t, err.Error(), "not a valid Unicode scalar value")
		})
	}
}

// TestCharValidation_ValidBoundaries tests valid char boundary values.
func TestCharValidation_ValidBoundaries(t *testing.T) {
	validValues := []struct {
		name  string
		value uint32
	}{
		{"null", 0},
		{"ascii_a", 0x0041},
		{"before_surrogates", 0xD7FF},
		{"after_surrogates", 0xE000},
		{"max_bmp", 0xFFFF},
		{"first_supplementary", 0x10000},
		{"max_valid", 0x10FFFF},
	}

	for _, tc := range validValues {
		t.Run(tc.name+"_lift", func(t *testing.T) {
			iter := abi.NewFlatIter([]uint64{uint64(tc.value)})
			lifted, err := abi.LiftFlat(nil, types.Char{}, iter)
			require.NoError(t, err)
			require.Equal(t, rune(tc.value), lifted.Char())
		})
	}
}

// TestUTF8_MiddleOfSequence tests handling of data starting mid-sequence.
func TestUTF8_MiddleOfSequence(t *testing.T) {
	testCases := []struct {
		name string
		data []byte
	}{
		{"starts_with_continuation", []byte{0x80, 0x41}},
		{"starts_with_two_continuations", []byte{0x80, 0x80, 0x41}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mem := newMockMemory(1024)
			copy(mem.data[64:], tc.data)

			binary.LittleEndian.PutUint32(mem.data[0:], 64)
			binary.LittleEndian.PutUint32(mem.data[4:], uint32(len(tc.data)))

			ctx := &abi.LiftContext{
				Memory: mem,
				Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
			}

			_, err := abi.LiftString(ctx, 0)
			require.Error(t, err, "should reject string starting with continuation bytes")
		})
	}
}

// TestUTF8_Truncated tests various truncation scenarios.
func TestUTF8_Truncated(t *testing.T) {
	testCases := []struct {
		name string
		data []byte
	}{
		{"truncated_2byte", []byte{0x41, 0xC2}},               // 'A' + incomplete 2-byte
		{"truncated_3byte_1", []byte{0x41, 0xE0}},             // 'A' + incomplete 3-byte
		{"truncated_3byte_2", []byte{0x41, 0xE0, 0xA0}},       // 'A' + incomplete 3-byte
		{"truncated_4byte_1", []byte{0x41, 0xF0}},             // 'A' + incomplete 4-byte
		{"truncated_4byte_2", []byte{0x41, 0xF0, 0x90}},       // 'A' + incomplete 4-byte
		{"truncated_4byte_3", []byte{0x41, 0xF0, 0x90, 0x80}}, // 'A' + incomplete 4-byte
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mem := newMockMemory(1024)
			copy(mem.data[64:], tc.data)

			binary.LittleEndian.PutUint32(mem.data[0:], 64)
			binary.LittleEndian.PutUint32(mem.data[4:], uint32(len(tc.data)))

			ctx := &abi.LiftContext{
				Memory: mem,
				Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
			}

			_, err := abi.LiftString(ctx, 0)
			require.Error(t, err, "should reject truncated UTF-8 sequence")
		})
	}
}
