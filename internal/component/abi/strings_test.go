package abi

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestLiftStringUTF8(t *testing.T) {
	// "hello" in UTF-8 at offset 16
	data := make([]byte, 32)
	copy(data[16:], "hello")
	// String stored as (ptr=16, len=5)
	binary.LittleEndian.PutUint32(data[0:], 16) // ptr
	binary.LittleEndian.PutUint32(data[4:], 5)  // len

	ctx := &LiftContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingUTF8},
	}

	val, err := LiftString(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, "hello", val)
}

func TestLiftStringUTF8_Unicode(t *testing.T) {
	// "日本語" (9 bytes in UTF-8)
	data := make([]byte, 32)
	copy(data[16:], "日本語")
	binary.LittleEndian.PutUint32(data[0:], 16)
	binary.LittleEndian.PutUint32(data[4:], 9)

	ctx := &LiftContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingUTF8},
	}

	val, err := LiftString(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, "日本語", val)
}

func TestLiftStringUTF8_Empty(t *testing.T) {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint32(data[0:], 0) // ptr doesn't matter for empty
	binary.LittleEndian.PutUint32(data[4:], 0) // len=0

	ctx := &LiftContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingUTF8},
	}

	val, err := LiftString(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, "", val)
}

func TestLiftStringUTF8_InvalidUTF8(t *testing.T) {
	data := make([]byte, 16)
	data[8] = 0xFF // Invalid UTF-8 byte
	data[9] = 0xFE // Invalid UTF-8 byte
	binary.LittleEndian.PutUint32(data[0:], 8)
	binary.LittleEndian.PutUint32(data[4:], 2)

	ctx := &LiftContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingUTF8},
	}

	_, err := LiftString(ctx, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid UTF-8")
}

func TestLiftStringUTF8_BoundsCheck(t *testing.T) {
	// Small memory with string pointing beyond bounds
	data := make([]byte, 16)
	binary.LittleEndian.PutUint32(data[0:], 100) // ptr = 100 (beyond memory)
	binary.LittleEndian.PutUint32(data[4:], 10)  // len = 10

	ctx := &LiftContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingUTF8},
	}

	_, err := LiftString(ctx, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to read string bytes")
}

func TestLiftStringUTF8_AtOffset(t *testing.T) {
	// Test lifting string from a non-zero offset
	// String "test" at offset 24, ptr/len at offset 16
	data := make([]byte, 48)
	copy(data[24:], "test")
	binary.LittleEndian.PutUint32(data[16:], 24) // ptr at offset 16
	binary.LittleEndian.PutUint32(data[20:], 4)  // len at offset 20

	ctx := &LiftContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingUTF8},
	}

	val, err := LiftString(ctx, 16)
	require.NoError(t, err)
	require.Equal(t, "test", val)
}

func TestLiftStringUTF8_Emoji(t *testing.T) {
	// Test emoji (4-byte UTF-8 characters)
	emoji := "Hello 🌍🎉"
	data := make([]byte, 64)
	copy(data[16:], emoji)
	binary.LittleEndian.PutUint32(data[0:], 16)
	binary.LittleEndian.PutUint32(data[4:], uint32(len(emoji)))

	ctx := &LiftContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingUTF8},
	}

	val, err := LiftString(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, emoji, val)
}

// Task 62: UTF-16 tests

func TestLiftStringUTF16(t *testing.T) {
	// "hello" in UTF-16 LE
	data := make([]byte, 32)
	// UTF-16 LE for "hello": h=0x0068, e=0x0065, l=0x006C, l=0x006C, o=0x006F
	binary.LittleEndian.PutUint16(data[16:], 0x0068) // h
	binary.LittleEndian.PutUint16(data[18:], 0x0065) // e
	binary.LittleEndian.PutUint16(data[20:], 0x006C) // l
	binary.LittleEndian.PutUint16(data[22:], 0x006C) // l
	binary.LittleEndian.PutUint16(data[24:], 0x006F) // o
	binary.LittleEndian.PutUint32(data[0:], 16)      // ptr
	binary.LittleEndian.PutUint32(data[4:], 5)       // 5 code units

	ctx := &LiftContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingUTF16},
	}

	val, err := LiftString(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, "hello", val)
}

func TestLiftStringUTF16_Empty(t *testing.T) {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint32(data[0:], 0)
	binary.LittleEndian.PutUint32(data[4:], 0) // 0 code units

	ctx := &LiftContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingUTF16},
	}

	val, err := LiftString(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, "", val)
}

func TestLiftStringUTF16_SurrogatePair(t *testing.T) {
	// "😀" (U+1F600) requires surrogate pair: 0xD83D 0xDE00
	data := make([]byte, 16)
	binary.LittleEndian.PutUint16(data[8:], 0xD83D)  // high surrogate
	binary.LittleEndian.PutUint16(data[10:], 0xDE00) // low surrogate
	binary.LittleEndian.PutUint32(data[0:], 8)       // ptr
	binary.LittleEndian.PutUint32(data[4:], 2)       // 2 code units

	ctx := &LiftContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingUTF16},
	}

	val, err := LiftString(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, "😀", val)
}

func TestLiftStringUTF16_BoundsCheck(t *testing.T) {
	// Small memory with string pointing beyond bounds
	data := make([]byte, 16)
	binary.LittleEndian.PutUint32(data[0:], 100) // ptr = 100 (beyond memory)
	binary.LittleEndian.PutUint32(data[4:], 10)  // 10 code units

	ctx := &LiftContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingUTF16},
	}

	_, err := LiftString(ctx, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to read UTF-16 string")
}

// Task 63: Latin1+UTF16 tests

func TestLiftStringLatin1UTF16_Latin1(t *testing.T) {
	// "hello" in Latin-1 (same as ASCII for these characters)
	data := make([]byte, 16)
	copy(data[8:], "hello")
	binary.LittleEndian.PutUint32(data[0:], 8)
	binary.LittleEndian.PutUint32(data[4:], 5) // No tag bit = Latin-1

	ctx := &LiftContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingLatin1UTF16},
	}

	val, err := LiftString(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, "hello", val)
}

func TestLiftStringLatin1UTF16_Latin1Extended(t *testing.T) {
	// "café" with é = 0xE9 in Latin-1
	data := make([]byte, 16)
	copy(data[8:], "caf")
	data[11] = 0xE9 // é in Latin-1
	binary.LittleEndian.PutUint32(data[0:], 8)
	binary.LittleEndian.PutUint32(data[4:], 4) // No tag bit = Latin-1

	ctx := &LiftContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingLatin1UTF16},
	}

	val, err := LiftString(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, "café", val)
}

func TestLiftStringLatin1UTF16_UTF16Tagged(t *testing.T) {
	// "hello" in UTF-16 with tag bit set
	data := make([]byte, 32)
	binary.LittleEndian.PutUint16(data[16:], 0x0068)
	binary.LittleEndian.PutUint16(data[18:], 0x0065)
	binary.LittleEndian.PutUint16(data[20:], 0x006C)
	binary.LittleEndian.PutUint16(data[22:], 0x006C)
	binary.LittleEndian.PutUint16(data[24:], 0x006F)
	binary.LittleEndian.PutUint32(data[0:], 16) // ptr
	// 5 code units with tag bit set (0x80000000 | 5 = 0x80000005)
	binary.LittleEndian.PutUint32(data[4:], 0x80000005)

	ctx := &LiftContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingLatin1UTF16},
	}

	val, err := LiftString(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, "hello", val)
}

func TestLiftStringLatin1UTF16_Empty(t *testing.T) {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint32(data[0:], 0)
	binary.LittleEndian.PutUint32(data[4:], 0) // 0 length = empty

	ctx := &LiftContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingLatin1UTF16},
	}

	val, err := LiftString(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, "", val)
}

func TestLiftStringLatin1UTF16_Latin1BoundsCheck(t *testing.T) {
	// Small memory with string pointing beyond bounds
	data := make([]byte, 16)
	binary.LittleEndian.PutUint32(data[0:], 100) // ptr = 100 (beyond memory)
	binary.LittleEndian.PutUint32(data[4:], 10)  // No tag bit = Latin-1

	ctx := &LiftContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingLatin1UTF16},
	}

	_, err := LiftString(ctx, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to read Latin-1 string")
}

func TestLiftStringUTF16_UnpairedSurrogate(t *testing.T) {
	// Unpaired high surrogate 0xD83D without following low surrogate
	// Go's utf16.Decode replaces with U+FFFD (replacement character)
	data := make([]byte, 16)
	binary.LittleEndian.PutUint16(data[8:], 0xD83D)  // high surrogate
	binary.LittleEndian.PutUint16(data[10:], 0x0041) // 'A' (not a low surrogate)
	binary.LittleEndian.PutUint32(data[0:], 8)
	binary.LittleEndian.PutUint32(data[4:], 2)

	ctx := &LiftContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingUTF16},
	}

	val, err := LiftString(ctx, 0)
	require.NoError(t, err)
	// Go's utf16.Decode returns replacement char (U+FFFD) for unpaired surrogates
	require.Contains(t, val, "\uFFFD")
}

func TestLiftStringLatin1UTF16_UTF16SurrogatePair(t *testing.T) {
	// Emoji in UTF-16 with tag bit set
	data := make([]byte, 16)
	binary.LittleEndian.PutUint16(data[8:], 0xD83D)  // high surrogate
	binary.LittleEndian.PutUint16(data[10:], 0xDE00) // low surrogate
	binary.LittleEndian.PutUint32(data[0:], 8)
	binary.LittleEndian.PutUint32(data[4:], 0x80000002) // 2 code units with tag

	ctx := &LiftContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingLatin1UTF16},
	}

	val, err := LiftString(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, "\U0001F600", val)
}

// Task 64: String Lowering Tests

func TestLowerStringUTF8(t *testing.T) {
	data := make([]byte, 64)
	allocPtr := uint32(16)

	ctx := &LowerContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			return allocPtr, nil
		},
	}

	ptr, length, err := LowerString(ctx, "hello")
	require.NoError(t, err)
	require.Equal(t, uint32(16), ptr)
	require.Equal(t, uint32(5), length)
	require.Equal(t, "hello", string(data[16:21]))
}

func TestLowerStringUTF8_Empty(t *testing.T) {
	data := make([]byte, 64)
	ctx := &LowerContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			t.Fatal("Realloc should not be called for empty string")
			return 0, nil
		},
	}

	ptr, length, err := LowerString(ctx, "")
	require.NoError(t, err)
	require.Equal(t, uint32(0), ptr)
	require.Equal(t, uint32(0), length)
}

func TestLowerStringUTF8_Unicode(t *testing.T) {
	data := make([]byte, 64)
	allocPtr := uint32(16)

	ctx := &LowerContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			return allocPtr, nil
		},
	}

	ptr, length, err := LowerString(ctx, "日本語")
	require.NoError(t, err)
	require.Equal(t, uint32(16), ptr)
	require.Equal(t, uint32(9), length) // 3 chars * 3 bytes each
	require.Equal(t, "日本語", string(data[16:25]))
}

func TestLowerStringUTF16(t *testing.T) {
	data := make([]byte, 64)
	allocPtr := uint32(16)

	ctx := &LowerContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingUTF16},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			return allocPtr, nil
		},
	}

	ptr, codeUnits, err := LowerString(ctx, "hello")
	require.NoError(t, err)
	require.Equal(t, uint32(16), ptr)
	require.Equal(t, uint32(5), codeUnits)
	// Verify UTF-16 LE encoding
	require.Equal(t, uint16(0x0068), binary.LittleEndian.Uint16(data[16:])) // h
	require.Equal(t, uint16(0x0065), binary.LittleEndian.Uint16(data[18:])) // e
}

func TestLowerStringUTF16_Empty(t *testing.T) {
	data := make([]byte, 64)
	ctx := &LowerContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingUTF16},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			t.Fatal("Realloc should not be called for empty string")
			return 0, nil
		},
	}

	ptr, codeUnits, err := LowerString(ctx, "")
	require.NoError(t, err)
	require.Equal(t, uint32(0), ptr)
	require.Equal(t, uint32(0), codeUnits)
}

func TestLowerStringUTF16_SurrogatePair(t *testing.T) {
	data := make([]byte, 64)
	allocPtr := uint32(16)

	ctx := &LowerContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingUTF16},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			return allocPtr, nil
		},
	}

	// Emoji requires surrogate pair in UTF-16
	ptr, codeUnits, err := LowerString(ctx, "\U0001F600")
	require.NoError(t, err)
	require.Equal(t, uint32(16), ptr)
	require.Equal(t, uint32(2), codeUnits) // surrogate pair = 2 code units
	require.Equal(t, uint16(0xD83D), binary.LittleEndian.Uint16(data[16:]))
	require.Equal(t, uint16(0xDE00), binary.LittleEndian.Uint16(data[18:]))
}

func TestLowerStringLatin1UTF16_Latin1(t *testing.T) {
	data := make([]byte, 64)
	allocPtr := uint32(16)

	ctx := &LowerContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingLatin1UTF16},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			return allocPtr, nil
		},
	}

	ptr, length, err := LowerString(ctx, "cafe")
	require.NoError(t, err)
	require.Equal(t, uint32(16), ptr)
	require.Equal(t, uint32(4), length) // No tag bit = Latin-1
	require.Equal(t, byte('c'), data[16])
	require.Equal(t, byte('a'), data[17])
	require.Equal(t, byte('f'), data[18])
	require.Equal(t, byte('e'), data[19])
}

func TestLowerStringLatin1UTF16_Latin1Extended(t *testing.T) {
	data := make([]byte, 64)
	allocPtr := uint32(16)

	ctx := &LowerContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingLatin1UTF16},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			return allocPtr, nil
		},
	}

	// Note: "cafe" with accent must be represented correctly
	// In Go, "cafe" is different from "cafe" with the e-acute character
	// We'll test with explicit Latin-1 range characters
	ptr, length, err := LowerString(ctx, "caf\u00E9") // e-acute in Latin-1 range
	require.NoError(t, err)
	require.Equal(t, uint32(16), ptr)
	require.Equal(t, uint32(4), length) // No tag bit = Latin-1
	require.Equal(t, byte('c'), data[16])
	require.Equal(t, byte('a'), data[17])
	require.Equal(t, byte('f'), data[18])
	require.Equal(t, byte(0xE9), data[19]) // e-acute in Latin-1
}

func TestLowerStringLatin1UTF16_Empty(t *testing.T) {
	data := make([]byte, 64)
	ctx := &LowerContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingLatin1UTF16},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			t.Fatal("Realloc should not be called for empty string")
			return 0, nil
		},
	}

	ptr, length, err := LowerString(ctx, "")
	require.NoError(t, err)
	require.Equal(t, uint32(0), ptr)
	require.Equal(t, uint32(0), length)
}

func TestLowerStringLatin1UTF16_FallbackUTF16(t *testing.T) {
	data := make([]byte, 64)
	allocPtr := uint32(16)

	ctx := &LowerContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingLatin1UTF16},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			return allocPtr, nil
		},
	}

	// Japanese character cannot be encoded in Latin-1
	ptr, taggedLen, err := LowerString(ctx, "\u65E5") // U+65E5 = Japanese "day"
	require.NoError(t, err)
	require.Equal(t, uint32(16), ptr)
	// Tag bit should be set, 1 code unit
	require.Equal(t, uint32(0x80000001), taggedLen)
}

func TestLowerStringUTF8_ReallocError(t *testing.T) {
	data := make([]byte, 64)
	ctx := &LowerContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			return 0, fmt.Errorf("out of memory")
		},
	}

	_, _, err := LowerString(ctx, "hello")
	require.Error(t, err)
	require.Contains(t, err.Error(), "realloc")
}

func TestLowerStringUTF8_WriteError(t *testing.T) {
	// Create a small memory that will fail on write
	data := make([]byte, 10)
	ctx := &LowerContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			return 100, nil // Return pointer beyond memory bounds
		},
	}

	_, _, err := LowerString(ctx, "hello")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to write")
}

func TestLowerStringUnknownEncoding(t *testing.T) {
	data := make([]byte, 64)
	ctx := &LowerContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncoding(99)}, // Invalid encoding
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			return 0, nil
		},
	}

	_, _, err := LowerString(ctx, "hello")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown string encoding")
}

// Task 1.2: String Alignment Validation Tests

func TestLiftStringUTF16AlignmentValidation(t *testing.T) {
	// Create memory with UTF-16 string at misaligned offset
	mem := &mockMemory{data: make([]byte, 100)}

	// Write valid UTF-16 "Hi" at offset 1 (misaligned for 2-byte alignment)
	mem.data[1] = 'H'
	mem.data[2] = 0
	mem.data[3] = 'i'
	mem.data[4] = 0

	ctx := &LiftContext{
		Memory: mem,
		Opts:   &Options{StringEncoding: StringEncodingUTF16},
	}

	// ptr=1 is misaligned for UTF-16 (requires 2-byte alignment)
	_, err := liftStringUTF16(ctx, 1, 2)
	if err == nil {
		t.Error("expected error for misaligned UTF-16 string pointer, got nil")
	}
	if err != nil {
		require.Contains(t, err.Error(), "align")
	}
}

func TestLiftStringLatin1UTF16AlignmentValidation(t *testing.T) {
	mem := &mockMemory{data: make([]byte, 100)}

	// Write UTF-16 data at offset 1 with UTF16_TAG set
	mem.data[1] = 'H'
	mem.data[2] = 0
	mem.data[3] = 'i'
	mem.data[4] = 0

	ctx := &LiftContext{
		Memory: mem,
		Opts:   &Options{StringEncoding: StringEncodingLatin1UTF16},
	}

	// taggedLen with UTF16_TAG set, ptr=1 misaligned
	taggedLen := uint32(2) | utf16Tag
	_, err := liftStringLatin1UTF16(ctx, 1, taggedLen)
	if err == nil {
		t.Error("expected error for misaligned Latin1+UTF16 string pointer, got nil")
	}
	if err != nil {
		require.Contains(t, err.Error(), "align")
	}
}

func TestLiftStringLatin1AlignmentValidation(t *testing.T) {
	// Per spec line 2116, Latin1+UTF16 always requires 2-byte alignment
	// even for Latin-1 strings (without UTF16_TAG set)
	mem := &mockMemory{data: make([]byte, 100)}

	// Write Latin-1 data at offset 1 (misaligned)
	mem.data[1] = 'H'
	mem.data[2] = 'i'

	ctx := &LiftContext{
		Memory: mem,
		Opts:   &Options{StringEncoding: StringEncodingLatin1UTF16},
	}

	// taggedLen without UTF16_TAG - pure Latin-1, but still requires 2-byte alignment per spec
	taggedLen := uint32(2) // No utf16Tag
	_, err := liftStringLatin1UTF16(ctx, 1, taggedLen)
	if err == nil {
		t.Error("expected error for misaligned Latin-1 string pointer, got nil")
	}
	if err != nil {
		require.Contains(t, err.Error(), "align")
	}
}
