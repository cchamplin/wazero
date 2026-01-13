package abi

import (
	"encoding/binary"
	"fmt"
	"math"
	"unicode/utf16"
	"unicode/utf8"
)

// utf16Tag is the tag bit used in Latin1+UTF16 encoding to indicate UTF-16.
// This is used in Task 63 for liftStringLatin1UTF16.
const utf16Tag = uint32(1 << 31)

// LiftString lifts a string from memory at the given offset.
// The offset points to a (ptr, len) pair in memory.
// NOTE: Integration with LiftHeap/LiftFlat for types.String is handled in Task 65.
func LiftString(ctx *LiftContext, offset uint32) (string, error) {
	ptr := ctx.ReadU32(offset)
	taggedLen := ctx.ReadU32(offset + 4)

	switch ctx.Opts.StringEncoding {
	case StringEncodingUTF8:
		return liftStringUTF8(ctx, ptr, taggedLen)
	case StringEncodingUTF16:
		return liftStringUTF16(ctx, ptr, taggedLen)
	case StringEncodingLatin1UTF16:
		return liftStringLatin1UTF16(ctx, ptr, taggedLen)
	default:
		return "", fmt.Errorf("unknown string encoding: %d", ctx.Opts.StringEncoding)
	}
}

// liftStringUTF8 lifts a UTF-8 encoded string from memory.
// The byteLen parameter is the byte length of the string.
func liftStringUTF8(ctx *LiftContext, ptr, byteLen uint32) (string, error) {
	if byteLen == 0 {
		return "", nil
	}
	data, ok := ctx.Memory.Read(ptr, byteLen)
	if !ok {
		return "", fmt.Errorf("failed to read string bytes at %d len %d", ptr, byteLen)
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("invalid UTF-8 string at ptr=%d len=%d", ptr, byteLen)
	}
	return string(data), nil
}

// liftStringUTF16 lifts a UTF-16 encoded string from memory.
// The codeUnits parameter is the number of UTF-16 code units (not bytes).
// Each code unit is 2 bytes, stored in little-endian order.
func liftStringUTF16(ctx *LiftContext, ptr, codeUnits uint32) (string, error) {
	if codeUnits == 0 {
		return "", nil
	}
	// Check for potential overflow in byte length calculation
	if codeUnits > math.MaxUint32/2 {
		return "", fmt.Errorf("UTF-16 string too large: %d code units", codeUnits)
	}
	byteLen := codeUnits * 2
	data, ok := ctx.Memory.Read(ptr, byteLen)
	if !ok {
		return "", fmt.Errorf("failed to read UTF-16 string at ptr=%d len=%d", ptr, byteLen)
	}

	// Decode UTF-16 LE
	u16 := make([]uint16, codeUnits)
	for i := uint32(0); i < codeUnits; i++ {
		u16[i] = binary.LittleEndian.Uint16(data[i*2:])
	}

	return string(utf16.Decode(u16)), nil
}

// liftStringLatin1UTF16 lifts a Latin1+UTF16 encoded string from memory.
// If the tag bit (bit 31) is set, it's UTF-16 encoded; otherwise it's Latin-1.
// Latin-1 is a subset of Unicode where each byte is a code point 0-255.
func liftStringLatin1UTF16(ctx *LiftContext, ptr, taggedLen uint32) (string, error) {
	if taggedLen&utf16Tag != 0 {
		// UTF-16 encoded (tag bit set)
		codeUnits := taggedLen &^ utf16Tag
		return liftStringUTF16(ctx, ptr, codeUnits)
	}
	// Latin-1 encoded (each byte is a code point)
	if taggedLen == 0 {
		return "", nil
	}
	data, ok := ctx.Memory.Read(ptr, taggedLen)
	if !ok {
		return "", fmt.Errorf("failed to read Latin-1 string at ptr=%d len=%d", ptr, taggedLen)
	}
	// Latin-1 is a subset of Unicode (code points 0-255), direct conversion
	runes := make([]rune, taggedLen)
	for i, b := range data {
		runes[i] = rune(b)
	}
	return string(runes), nil
}
