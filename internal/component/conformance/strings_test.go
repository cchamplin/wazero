// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package conformance: string conformance tests (UTF-8/UTF-16/Latin-1
// encoding roundtrips, boundary cases, unpaired surrogate rejection,
// type properties). Ported from wasmtime's
// tests/all/component_model/strings.rs STRINGS constant and cross-
// checked against the canonical-ABI spec's load_string /
// store_string family in definitions.py.
package conformance

import (
	"encoding/binary"
	"errors"
	"testing"
	"unicode/utf16"

	"github.com/tetratelabs/wazero/experimental/wazerotest"
	"github.com/tetratelabs/wazero/internal/component/abi"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// testStrings is the shared set of string fixtures for the encoding
// roundtrip tests. Ported from wasmtime's STRINGS constant at
// tests/all/component_model/strings.rs.
var testStrings = []struct {
	name string
	s    string
}{
	{"empty", ""},
	{"single_ascii", "x"},
	{"ascii", "hello world"},
	{"long_ascii", "hello this is a particularly long string yes it is it keeps going"},
	{"latin1_extended", "\u00E0 \u00E1 \u00E2 \u00E3 \u00E4 \u00E5 \u00E6 \u00E7 \u00E8 \u00E9 \u00EA \u00EB"},
	{"greek", "\u03B1\u03B2\u03B3\u03B4\u03B5\u03B6\u03B7\u03B8"},
	{"greek_spaced", "\u039E \u039F \u03A0 \u03A1 \u03A3 \u03A4 \u03A5 \u03A6 \u03A7 \u03A8 \u03A9 \u03AA \u03AB \u03AC \u03AD \u03AE"},
	{"fullwidth", "\uFF33\uFF34\uFF35\uFF36\uFF37\uFF38\uFF39\uFF3A"},
	{"fullwidth_space", "\u3000"},
	{"supplementary", "\U00010000"},
	{"mixed_emoji", "hello \U0001F600 world"},
	{"latin1_prefix_unicode_suffix", "pr\u00E9fix\u00E9"},
	{"mixed", "\u00E0 ascii \uFF36\uFF37\uFF38\uFF39\uFF3A"},
	{"extended_latin", "\u00CB\u00CC\u00CD\u00CE\u00CF\u00D0\u00D1\u00D2"},
}

// newStringLowerContext constructs a LowerContext around a fresh
// wazerotest.NewMemory with a bump-pointer realloc starting at startPtr.
func newStringLowerContext(startPtr uint32, enc abi.StringEncoding) *abi.LowerContext {
	mem := wazerotest.NewMemory(wazerotest.PageSize)
	alloc := startPtr
	return &abi.LowerContext{
		Memory: mem,
		Opts:   &abi.Options{StringEncoding: enc},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			// Align up to the requested alignment.
			if align > 1 {
				alloc = (alloc + align - 1) &^ (align - 1)
			}
			result := alloc
			alloc += newSize
			return result, nil
		},
	}
}

// newStringLiftContextFrom constructs a LiftContext around a shared
// *wazerotest.Memory produced by a companion LowerContext.
func newStringLiftContextFrom(lc *abi.LowerContext, enc abi.StringEncoding) *abi.LiftContext {
	return &abi.LiftContext{
		Memory: lc.Memory,
		Opts:   &abi.Options{StringEncoding: enc},
	}
}

// TestStringsUTF8Roundtrip asserts that every fixture roundtrips
// cleanly through LowerString → LiftFlat under the UTF-8 encoding.
//
// Spec: definitions.py:1437 store_string + :1252-1278 load_string
// (UTF-8 branch at :1254-1257 alignment=1, byte_length=tagged_code_units).
// Canonical test: run_tests.py exercises the same fixtures implicitly
// through store_string_copy at definitions.py:1478.
func TestStringsUTF8Roundtrip(t *testing.T) {
	for _, tc := range testStrings {
		t.Run(tc.name, func(t *testing.T) {
			lowerCtx := newStringLowerContext(256, abi.StringEncodingUTF8)

			ptr, length, err := abi.LowerString(lowerCtx, tc.s)
			require.NoError(t, err)

			liftCtx := newStringLiftContextFrom(lowerCtx, abi.StringEncodingUTF8)

			iter := abi.NewFlatIter([]uint64{uint64(ptr), uint64(length)})
			lifted, err := abi.LiftFlat(liftCtx, types.String_, iter)
			require.NoError(t, err)
			require.Equal(t, types.ValKindString, lifted.Kind())
			require.Equal(t, tc.s, lifted.StringVal())
		})
	}
}

// TestStringsUTF16Roundtrip asserts that every fixture roundtrips
// cleanly through LowerString → LiftFlat under the UTF-16 encoding.
//
// Spec: definitions.py:1258-1261 load_string UTF-16 branch
// (alignment=2, byte_length = 2 * tagged_code_units). Spec:
// definitions.py:1462-1466 store_string UTF-16 branch.
func TestStringsUTF16Roundtrip(t *testing.T) {
	for _, tc := range testStrings {
		t.Run(tc.name, func(t *testing.T) {
			lowerCtx := newStringLowerContext(256, abi.StringEncodingUTF16)

			ptr, codeUnits, err := abi.LowerString(lowerCtx, tc.s)
			require.NoError(t, err)

			liftCtx := newStringLiftContextFrom(lowerCtx, abi.StringEncodingUTF16)

			iter := abi.NewFlatIter([]uint64{uint64(ptr), uint64(codeUnits)})
			lifted, err := abi.LiftFlat(liftCtx, types.String_, iter)
			require.NoError(t, err)
			require.Equal(t, types.ValKindString, lifted.Kind())
			require.Equal(t, tc.s, lifted.StringVal())
		})
	}
}

// TestStringsLatin1UTF16Roundtrip asserts that every fixture
// roundtrips cleanly through LowerString → LiftFlat under the
// Latin-1+UTF-16 encoding — both Latin-1-compatible and non-Latin-1
// strings fall through the same path.
//
// Spec: definitions.py:1262-1269 load_string Latin1+UTF16 branch
// (UTF16_TAG check). Spec: definitions.py:1467-1474 store_string
// Latin1+UTF16 branch.
func TestStringsLatin1UTF16Roundtrip(t *testing.T) {
	for _, tc := range testStrings {
		t.Run(tc.name, func(t *testing.T) {
			lowerCtx := newStringLowerContext(256, abi.StringEncodingLatin1UTF16)

			ptr, taggedLen, err := abi.LowerString(lowerCtx, tc.s)
			require.NoError(t, err)

			liftCtx := newStringLiftContextFrom(lowerCtx, abi.StringEncodingLatin1UTF16)

			iter := abi.NewFlatIter([]uint64{uint64(ptr), uint64(taggedLen)})
			lifted, err := abi.LiftFlat(liftCtx, types.String_, iter)
			require.NoError(t, err)
			require.Equal(t, types.ValKindString, lifted.Kind())
			require.Equal(t, tc.s, lifted.StringVal())
		})
	}
}

// TestStringsLatin1UTF16Compression asserts that Latin-1-compatible
// strings are stored as Latin-1 (one byte per char, UTF16_TAG clear),
// and non-Latin-1 strings fall back to UTF-16 with UTF16_TAG set.
//
// Spec: definitions.py:1250 UTF16_TAG = 1 << 31. Spec:
// definitions.py:1531 store_string_to_latin1_or_utf16 selects the
// compact encoding when all code points fit in Latin-1.
func TestStringsLatin1UTF16Compression(t *testing.T) {
	const utf16Tag = uint32(1 << 31)

	latin1Only := []string{
		"",
		"hello",
		"\u00E0\u00E1\u00E2\u00E3",
		"\u00CB\u00CC\u00CD\u00CE",
	}

	for _, s := range latin1Only {
		t.Run("latin1_compression", func(t *testing.T) {
			lowerCtx := newStringLowerContext(64, abi.StringEncodingLatin1UTF16)

			_, taggedLen, err := abi.LowerString(lowerCtx, s)
			require.NoError(t, err)

			if s != "" {
				require.Equal(t, uint32(0), taggedLen&utf16Tag)
			}
		})
	}

	nonLatin1 := []string{
		"\u03B1\u03B2\u03B3",
		"\u4E2D\u6587",
		"hello \U0001F600",
	}

	for _, s := range nonLatin1 {
		t.Run("utf16_fallback", func(t *testing.T) {
			lowerCtx := newStringLowerContext(64, abi.StringEncodingLatin1UTF16)

			_, taggedLen, err := abi.LowerString(lowerCtx, s)
			require.NoError(t, err)

			require.NotEqual(t, uint32(0), taggedLen&utf16Tag)
		})
	}
}

// TestStringsPtrOutOfBounds asserts that LiftString fails when the
// string pointer + length exceeds memory. The canonical-ABI spec
// traps at definitions.py:1272
// trap_if(ptr + byte_length > len(cx.opts.memory)).
//
// Spec: definitions.py:1272 trap on ptr + byte_length overflow.
// Wasmtime parallel: tests/all/component_model/strings.rs
// ptr_out_of_bounds test exercises the same gate.
func TestStringsPtrOutOfBounds(t *testing.T) {
	// wazerotest.NewMemory rounds to page size, so we use offsets
	// that exceed the single-page boundary to force OOB.
	const memSize = wazerotest.PageSize
	const beyondPage = uint32(memSize + 1000)

	t.Run("ptr_beyond_memory", func(t *testing.T) {
		mem := wazerotest.NewMemory(memSize)
		// Write (ptr=beyondPage, len=5) at offset 0.
		binary.LittleEndian.PutUint32(mem.Bytes[0:], beyondPage)
		binary.LittleEndian.PutUint32(mem.Bytes[4:], 5)

		liftCtx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		}

		_, err := abi.LiftString(liftCtx, 0)
		require.Error(t, err)
	})

	t.Run("ptr_plus_len_beyond_memory", func(t *testing.T) {
		mem := wazerotest.NewMemory(memSize)
		// ptr=memSize-4, len=10 → ptr+len > memSize.
		binary.LittleEndian.PutUint32(mem.Bytes[0:], memSize-4)
		binary.LittleEndian.PutUint32(mem.Bytes[4:], 10)

		liftCtx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		}

		_, err := abi.LiftString(liftCtx, 0)
		require.Error(t, err)
	})

	t.Run("zero_length_valid_ptr", func(t *testing.T) {
		mem := wazerotest.NewMemory(memSize)
		// Zero-length strings are valid regardless of ptr; the
		// UTF-8 branch short-circuits on byte_length == 0 at
		// definitions.py:1256 / liftStringUTF8 early-return.
		binary.LittleEndian.PutUint32(mem.Bytes[0:], beyondPage)
		binary.LittleEndian.PutUint32(mem.Bytes[4:], 0)

		liftCtx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		}

		s, err := abi.LiftString(liftCtx, 0)
		require.NoError(t, err)
		require.Equal(t, "", s)
	})

	t.Run("utf16_ptr_beyond_memory", func(t *testing.T) {
		mem := wazerotest.NewMemory(memSize)
		// Ensure ptr is 2-byte aligned to exercise the byte-length
		// overflow rather than the alignment trap.
		binary.LittleEndian.PutUint32(mem.Bytes[0:], beyondPage&^1)
		binary.LittleEndian.PutUint32(mem.Bytes[4:], 5)

		liftCtx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF16},
		}

		_, err := abi.LiftString(liftCtx, 0)
		require.Error(t, err)
	})
}

// TestStringsPtrOverflow asserts that ptr + len overflow (both for
// UTF-8 and UTF-16 code-unit * 2) is caught before the memory read.
// The canonical-ABI spec handles this via the trap_if at
// definitions.py:1272 and, for UTF-16, the implicit code-units * 2
// computation at :1260.
//
// Spec: definitions.py:1272 trap_if ptr + byte_length > len(memory).
// Wasmtime parallel: tests/all/component_model/strings.rs
// ptr_overflow test.
func TestStringsPtrOverflow(t *testing.T) {
	t.Run("utf8_overflow", func(t *testing.T) {
		mem := wazerotest.NewMemory(wazerotest.PageSize)
		binary.LittleEndian.PutUint32(mem.Bytes[0:], 1<<31)
		binary.LittleEndian.PutUint32(mem.Bytes[4:], 1<<31)

		liftCtx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		}

		_, err := abi.LiftString(liftCtx, 0)
		require.Error(t, err)
	})

	t.Run("utf16_code_units_overflow", func(t *testing.T) {
		mem := wazerotest.NewMemory(wazerotest.PageSize)
		binary.LittleEndian.PutUint32(mem.Bytes[0:], 0)
		binary.LittleEndian.PutUint32(mem.Bytes[4:], 0x80000000)

		liftCtx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF16},
		}

		_, err := abi.LiftString(liftCtx, 0)
		require.Error(t, err)
	})
}

// TestStringsEmptyHandling asserts that empty strings go through the
// fast-path: LowerString returns (0, 0) without calling realloc, and
// LiftFlat with (0, 0) returns "". This mirrors the spec's implicit
// no-op behaviour for zero-length strings at store_string_copy
// (definitions.py:1478) — dst_byte_length == 0 produces an empty
// encoded buffer, though wazero optimises by short-circuiting before
// realloc.
//
// Spec: definitions.py:1478 store_string_copy (dst_byte_length = 0
// when src_code_units = 0).
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
			mem := wazerotest.NewMemory(wazerotest.PageSize)
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
			require.False(t, reallocCalled)
		})

		t.Run(enc.name+"_empty_lift", func(t *testing.T) {
			mem := wazerotest.NewMemory(wazerotest.PageSize)

			liftCtx := &abi.LiftContext{
				Memory: mem,
				Opts:   &abi.Options{StringEncoding: enc.enc},
			}

			iter := abi.NewFlatIter([]uint64{0, 0})
			lifted, err := abi.LiftFlat(liftCtx, types.String_, iter)
			require.NoError(t, err)
			require.Equal(t, "", lifted.StringVal())
		})
	}
}

// TestStringsInvalidUTF8 asserts invalid UTF-8 sequences are
// rejected during lift via utf8.Valid; canonical ABI's load_string
// traps on the UnicodeError at definitions.py:1274-1276 `try ...
// except UnicodeError: trap()`.
//
// Spec: definitions.py:1274-1276 decode-or-trap on UnicodeError.
func TestStringsInvalidUTF8(t *testing.T) {
	invalidSequences := []struct {
		name string
		data []byte
	}{
		{"single_ff", []byte{0xFF}},
		{"incomplete_2byte", []byte{0xC2}},
		{"incomplete_3byte", []byte{0xE0, 0x80}},
		{"invalid_continuation", []byte{0x80}},
		{"overlong_null", []byte{0xC0, 0x80}},
		{"invalid_f5", []byte{0xF5, 0x80, 0x80, 0x80}},
	}

	for _, tc := range invalidSequences {
		t.Run(tc.name, func(t *testing.T) {
			mem := wazerotest.NewMemory(wazerotest.PageSize)
			copy(mem.Bytes[16:], tc.data)

			binary.LittleEndian.PutUint32(mem.Bytes[0:], 16)
			binary.LittleEndian.PutUint32(mem.Bytes[4:], uint32(len(tc.data)))

			liftCtx := &abi.LiftContext{
				Memory: mem,
				Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
			}

			_, err := abi.LiftString(liftCtx, 0)
			require.Error(t, err)
		})
	}
}

// TestStringsUTF16Surrogates asserts that valid UTF-16 surrogate
// pairs are decoded correctly. The spec defers to Python's
// 'utf-16-le' codec at definitions.py:1261, which handles surrogate
// pairs natively; wazero uses Go's utf16.Decode for the same
// behaviour.
//
// Spec: definitions.py:1258-1278 UTF-16 decode (load_string Latin1+
// UTF16 / UTF16 branches invoke Python's utf-16-le codec which
// handles surrogate pairs).
func TestStringsUTF16Surrogates(t *testing.T) {
	t.Run("valid_surrogate_pair", func(t *testing.T) {
		mem := wazerotest.NewMemory(wazerotest.PageSize)

		// U+10000 encoded as surrogate pair: D800 DC00.
		binary.LittleEndian.PutUint16(mem.Bytes[16:], 0xD800)
		binary.LittleEndian.PutUint16(mem.Bytes[18:], 0xDC00)

		binary.LittleEndian.PutUint32(mem.Bytes[0:], 16)
		binary.LittleEndian.PutUint32(mem.Bytes[4:], 2)

		liftCtx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF16},
		}

		s, err := abi.LiftString(liftCtx, 0)
		require.NoError(t, err)
		require.Equal(t, "\U00010000", s)
	})

	t.Run("emoji_surrogate_pair", func(t *testing.T) {
		mem := wazerotest.NewMemory(wazerotest.PageSize)

		// U+1F600 (grinning face) surrogate pair: D83D DE00.
		binary.LittleEndian.PutUint16(mem.Bytes[16:], 0xD83D)
		binary.LittleEndian.PutUint16(mem.Bytes[18:], 0xDE00)

		binary.LittleEndian.PutUint32(mem.Bytes[0:], 16)
		binary.LittleEndian.PutUint32(mem.Bytes[4:], 2)

		liftCtx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF16},
		}

		s, err := abi.LiftString(liftCtx, 0)
		require.NoError(t, err)
		require.Equal(t, "\U0001F600", s)
	})
}

// TestStringsCrossEncodingConversion roundtrips every fixture
// through every (src_encoding, dst_encoding) pair. This exercises
// the full 3×3 matrix of store_string_into_range branches at
// definitions.py:1442-1474.
//
// Spec: definitions.py:1442-1474 store_string_into_range dispatch
// table across utf8 / utf16 / latin1+utf16 source and destination
// encodings.
func TestStringsCrossEncodingConversion(t *testing.T) {
	encodings := []abi.StringEncoding{
		abi.StringEncodingUTF8,
		abi.StringEncodingUTF16,
		abi.StringEncodingLatin1UTF16,
	}

	for _, srcEnc := range encodings {
		for _, dstEnc := range encodings {
			srcEnc, dstEnc := srcEnc, dstEnc
			testName := encName(srcEnc) + "_to_" + encName(dstEnc)
			t.Run(testName, func(t *testing.T) {
				for _, tc := range testStrings {
					// Lower with source encoding.
					lowerCtx := newStringLowerContext(512, srcEnc)
					ptr, taggedLen, err := abi.LowerString(lowerCtx, tc.s)
					require.NoError(t, err)

					// Lift with source encoding.
					liftCtx := newStringLiftContextFrom(lowerCtx, srcEnc)
					iter := abi.NewFlatIter([]uint64{uint64(ptr), uint64(taggedLen)})
					lifted, err := abi.LiftFlat(liftCtx, types.String_, iter)
					require.NoError(t, err)
					require.Equal(t, tc.s, lifted.StringVal())

					// Re-lower with destination encoding (fresh memory).
					lowerCtx2 := newStringLowerContext(512, dstEnc)
					ptr2, taggedLen2, err := abi.LowerString(lowerCtx2, lifted.StringVal())
					require.NoError(t, err)

					// Lift with destination encoding.
					liftCtx2 := newStringLiftContextFrom(lowerCtx2, dstEnc)
					iter2 := abi.NewFlatIter([]uint64{uint64(ptr2), uint64(taggedLen2)})
					lifted2, err := abi.LiftFlat(liftCtx2, types.String_, iter2)
					require.NoError(t, err)
					require.Equal(t, tc.s, lifted2.StringVal())
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

// TestStringsTypeProperties asserts the canonical-ABI metadata for
// the string ValType: size 8 (ptr+len), alignment 4, flatten count 2.
// Matches wasmtime's types.rs scalar ABI table and the spec's
// alignment/size-of functions in definitions.py.
//
// Spec: definitions.py:1078-1127 alignment / size_of for string
// (alignment 4 in memory32, size 8 = ptr32 + len32). Spec:
// definitions.py:1706-1708 flatten_type for string (ptr32 + len32 = 2).
func TestStringsTypeProperties(t *testing.T) {
	ct := &types.ComponentTypes{}
	abiInfo := types.String_.ABI(ct)
	require.Equal(t, uint32(8), abiInfo.Size32)
	require.Equal(t, uint32(4), abiInfo.Align32)
	require.Equal(t, int32(2), abiInfo.FlattenCount)
}

// TestStringsLowerFlatRoundtrip asserts LowerFlat/LiftFlat round-trip
// the string ValType through exactly 2 flat values (ptr, len) for
// every fixture under UTF-8.
//
// Spec: definitions.py:1706-1708 flatten_type maps string to
// (i32, i32). Spec: definitions.py:1820-1830 lower_flat_string
// writes (ptr, len) to the flat iterator.
func TestStringsLowerFlatRoundtrip(t *testing.T) {
	for _, tc := range testStrings {
		t.Run(tc.name, func(t *testing.T) {
			ctx := newStringLowerContext(256, abi.StringEncodingUTF8)

			val := types.ValString(tc.s)
			flat, err := abi.LowerFlat(ctx, types.String_, val)
			require.NoError(t, err)
			require.Equal(t, 2, len(flat))

			liftCtx := newStringLiftContextFrom(ctx, abi.StringEncodingUTF8)
			iter := abi.NewFlatIter(flat)
			lifted, err := abi.LiftFlat(liftCtx, types.String_, iter)
			require.NoError(t, err)
			require.Equal(t, tc.s, lifted.StringVal())
		})
	}
}

// TestStringsReallocError asserts that a realloc failure inside
// LowerString is propagated to the caller. The spec's store_string
// family calls realloc at definitions.py:1481 and propagates any
// trap; wazero returns a wrapped Go error.
//
// Spec: definitions.py:1481 ptr = cx.opts.realloc(...) (any failure
// here traps in Python, is returned as an error in Go).
func TestStringsReallocError(t *testing.T) {
	mem := wazerotest.NewMemory(wazerotest.PageSize)
	ctx := &abi.LowerContext{
		Memory: mem,
		Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			return 0, errors.New("allocation failed")
		},
	}

	_, _, err := abi.LowerString(ctx, "test")
	require.Error(t, err)
	require.Contains(t, err.Error(), "realloc")
}

// TestStringsWriteError asserts that a memory write failure inside
// LowerString is propagated to the caller. The spec's
// store_string_copy writes to memory at definitions.py:1486; a trap
// there in Python becomes a returned error in Go.
//
// Spec: definitions.py:1483 trap_if(ptr + dst_byte_length >
// len(cx.opts.memory)).
func TestStringsWriteError(t *testing.T) {
	mem := wazerotest.NewMemory(wazerotest.PageSize)
	ctx := &abi.LowerContext{
		Memory: mem,
		Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			// Return a pointer beyond the single page so the
			// subsequent Write() fails.
			return wazerotest.PageSize + 1000, nil
		},
	}

	_, _, err := abi.LowerString(ctx, "test")
	require.Error(t, err)
}

// TestStringsUnknownEncoding asserts that an unknown StringEncoding
// is rejected with an "unknown string encoding" error. The spec's
// match statement at definitions.py:1253 has no default branch, so
// an unrecognised encoding is a programmer error; wazero converts it
// to a returned Go error.
//
// Spec: definitions.py:1253 match cx.opts.string_encoding (exhaustive
// over the three canonical encodings).
func TestStringsUnknownEncoding(t *testing.T) {
	mem := wazerotest.NewMemory(wazerotest.PageSize)

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

// TestStringsUTF16ByteLength asserts that the length returned by
// LowerString for UTF-16 is in code units (not bytes), matching the
// spec's src_code_units variable.
//
// Spec: definitions.py:1258-1261 load_string UTF-16 branch uses
// tagged_code_units (the code-unit count, *not* the byte count) and
// multiplies by 2 to derive byte_length.
func TestStringsUTF16ByteLength(t *testing.T) {
	testCases := []struct {
		name      string
		s         string
		codeUnits int
	}{
		{"empty", "", 0},
		{"ascii", "hello", 5},
		{"bmp_only", "\u4E2D\u6587", 2},
		{"with_surrogate", "\U0001F600", 2},
		{"mixed", "a\U0001F600b", 4},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := newStringLowerContext(64, abi.StringEncodingUTF16)

			_, codeUnits, err := abi.LowerString(ctx, tc.s)
			require.NoError(t, err)
			require.Equal(t, uint32(tc.codeUnits), codeUnits)

			if tc.s != "" {
				runes := []rune(tc.s)
				u16 := utf16.Encode(runes)
				require.Equal(t, len(u16), tc.codeUnits)
			}
		})
	}
}

// TestStringsLatin1UTF16TagBit asserts the UTF16_TAG (bit 31) flag
// discriminates between Latin-1 and UTF-16 payloads in the Latin1+
// UTF16 encoding. The tag is set by the lift/lower path exactly when
// the stored data is UTF-16.
//
// Spec: definitions.py:1250 UTF16_TAG = 1 << 31. Spec:
// definitions.py:1264-1269 load_string selects UTF-16 branch when
// tagged_code_units & UTF16_TAG is set.
func TestStringsLatin1UTF16TagBit(t *testing.T) {
	const utf16Tag = uint32(1 << 31)

	t.Run("lift_with_tag", func(t *testing.T) {
		mem := wazerotest.NewMemory(wazerotest.PageSize)

		binary.LittleEndian.PutUint16(mem.Bytes[16:], 0x0041) // 'A'
		binary.LittleEndian.PutUint16(mem.Bytes[18:], 0x0042) // 'B'

		binary.LittleEndian.PutUint32(mem.Bytes[0:], 16)
		binary.LittleEndian.PutUint32(mem.Bytes[4:], 2|utf16Tag)

		ctx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingLatin1UTF16},
		}

		s, err := abi.LiftString(ctx, 0)
		require.NoError(t, err)
		require.Equal(t, "AB", s)
	})

	t.Run("lift_without_tag", func(t *testing.T) {
		mem := wazerotest.NewMemory(wazerotest.PageSize)

		// Latin-1 "AB" at offset 16. Per the current
		// liftStringLatin1UTF16 implementation the ptr must be
		// 2-byte aligned (spec line 2116), which offset 16 is.
		mem.Bytes[16] = 0x41
		mem.Bytes[17] = 0x42

		binary.LittleEndian.PutUint32(mem.Bytes[0:], 16)
		binary.LittleEndian.PutUint32(mem.Bytes[4:], 2)

		ctx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingLatin1UTF16},
		}

		s, err := abi.LiftString(ctx, 0)
		require.NoError(t, err)
		require.Equal(t, "AB", s)
	})

	t.Run("latin1_extended_chars", func(t *testing.T) {
		mem := wazerotest.NewMemory(wazerotest.PageSize)

		mem.Bytes[16] = 0xE0
		mem.Bytes[17] = 0xE1
		mem.Bytes[18] = 0xE2
		mem.Bytes[19] = 0xE3

		binary.LittleEndian.PutUint32(mem.Bytes[0:], 16)
		binary.LittleEndian.PutUint32(mem.Bytes[4:], 4)

		ctx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingLatin1UTF16},
		}

		s, err := abi.LiftString(ctx, 0)
		require.NoError(t, err)
		require.Equal(t, "\u00E0\u00E1\u00E2\u00E3", s)
	})
}

// TestStringsHeapOperations asserts that LiftHeap / LowerHeap for
// the string ValType exercise the memory path (as opposed to the
// flat-path LiftFlat/LowerFlat). This is the canonical-ABI store /
// load pair for strings when the parent context is itself a heap
// store (e.g. a record field or list element).
//
// Spec: definitions.py:1245-1248 load_string (heap path reads
// ptr, len from memory offset). Spec: definitions.py:1437-1440
// store_string (heap path writes ptr, len to memory offset).
func TestStringsHeapOperations(t *testing.T) {
	t.Run("lift_heap", func(t *testing.T) {
		mem := wazerotest.NewMemory(wazerotest.PageSize)
		copy(mem.Bytes[16:], "hello")

		binary.LittleEndian.PutUint32(mem.Bytes[0:], 16)
		binary.LittleEndian.PutUint32(mem.Bytes[4:], 5)

		ctx := &abi.LiftContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		}

		val, err := abi.LiftHeap(ctx, types.String_, 0)
		require.NoError(t, err)
		require.Equal(t, types.ValKindString, val.Kind())
		require.Equal(t, "hello", val.StringVal())
	})

	t.Run("lower_heap", func(t *testing.T) {
		mem := wazerotest.NewMemory(wazerotest.PageSize)
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
		err := abi.LowerHeap(ctx, types.String_, val, 0)
		require.NoError(t, err)

		// Verify ptr/len were written at offset 0.
		ptr := binary.LittleEndian.Uint32(mem.Bytes[0:])
		length := binary.LittleEndian.Uint32(mem.Bytes[4:])
		require.Equal(t, uint32(64), ptr)
		require.Equal(t, uint32(5), length)

		require.Equal(t, "world", string(mem.Bytes[64:69]))
	})
}
