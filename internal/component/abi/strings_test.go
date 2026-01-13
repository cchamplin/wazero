package abi

import (
	"encoding/binary"
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

func TestLiftStringUTF16_NotImplemented(t *testing.T) {
	data := make([]byte, 16)
	binary.LittleEndian.PutUint32(data[0:], 8)
	binary.LittleEndian.PutUint32(data[4:], 2)

	ctx := &LiftContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingUTF16},
	}

	_, err := LiftString(ctx, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not yet implemented")
}

func TestLiftStringLatin1UTF16_NotImplemented(t *testing.T) {
	data := make([]byte, 16)
	binary.LittleEndian.PutUint32(data[0:], 8)
	binary.LittleEndian.PutUint32(data[4:], 2)

	ctx := &LiftContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingLatin1UTF16},
	}

	_, err := LiftString(ctx, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not yet implemented")
}
