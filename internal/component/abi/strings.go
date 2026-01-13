package abi

import (
	"encoding/binary"
	"fmt"
	"math"
	"unicode/utf16"
	"unicode/utf8"
)

// utf16Tag is the tag bit used in Latin1+UTF16 encoding to indicate UTF-16.
// When this bit is set in the length field, the string is UTF-16 encoded;
// otherwise it's Latin-1 encoded.
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

// LowerString lowers a string to memory.
// Returns (ptr, taggedLen, err) where ptr points to the allocated string data.
// NOTE: Integration with LowerFlat/LowerHeap for types.String is handled in Task 65.
func LowerString(ctx *LowerContext, s string) (ptr, taggedLen uint32, err error) {
	switch ctx.Opts.StringEncoding {
	case StringEncodingUTF8:
		return lowerStringUTF8(ctx, s)
	case StringEncodingUTF16:
		return lowerStringUTF16(ctx, s)
	case StringEncodingLatin1UTF16:
		return lowerStringLatin1UTF16(ctx, s)
	default:
		return 0, 0, fmt.Errorf("unknown string encoding: %d", ctx.Opts.StringEncoding)
	}
}

// lowerStringUTF8 lowers a string to memory using UTF-8 encoding.
// Returns (ptr, byteLen, err) where byteLen is the byte length of the string.
func lowerStringUTF8(ctx *LowerContext, s string) (uint32, uint32, error) {
	data := []byte(s)
	if len(data) > math.MaxUint32 {
		return 0, 0, fmt.Errorf("UTF-8 string too large: %d bytes", len(data))
	}

	byteLen := uint32(len(data))
	if byteLen == 0 {
		return 0, 0, nil
	}

	ptr, err := ctx.Realloc(0, 0, 1, byteLen)
	if err != nil {
		return 0, 0, fmt.Errorf("realloc for string: %w", err)
	}

	if !ctx.Memory.Write(ptr, data) {
		return 0, 0, fmt.Errorf("failed to write string at %d", ptr)
	}

	return ptr, byteLen, nil
}

// lowerStringUTF16 lowers a string to memory using UTF-16 LE encoding.
// Returns (ptr, codeUnits, err) where codeUnits is the number of UTF-16 code units.
func lowerStringUTF16(ctx *LowerContext, s string) (uint32, uint32, error) {
	if len(s) == 0 {
		return 0, 0, nil
	}

	runes := []rune(s)
	u16 := utf16.Encode(runes)

	// Check for overflow in code units
	if len(u16) > math.MaxUint32/2 {
		return 0, 0, fmt.Errorf("UTF-16 string too large: %d code units", len(u16))
	}

	byteLen := uint32(len(u16) * 2)

	ptr, err := ctx.Realloc(0, 0, 2, byteLen)
	if err != nil {
		return 0, 0, fmt.Errorf("realloc for UTF-16 string: %w", err)
	}

	data := make([]byte, byteLen)
	for i, u := range u16 {
		binary.LittleEndian.PutUint16(data[i*2:], u)
	}

	if !ctx.Memory.Write(ptr, data) {
		return 0, 0, fmt.Errorf("failed to write UTF-16 string at %d", ptr)
	}

	return ptr, uint32(len(u16)), nil
}

// lowerStringLatin1UTF16 lowers a string to memory using Latin-1 or UTF-16 encoding.
// Tries Latin-1 first (all code points <= 0xFF), falls back to UTF-16 with tag bit if needed.
// Returns (ptr, taggedLen, err) where taggedLen has the UTF-16 tag bit set if UTF-16 was used.
func lowerStringLatin1UTF16(ctx *LowerContext, s string) (uint32, uint32, error) {
	if len(s) == 0 {
		return 0, 0, nil
	}

	// Try Latin-1 first (all code points <= 0xFF)
	canLatin1 := true
	for _, r := range s {
		if r > 0xFF {
			canLatin1 = false
			break
		}
	}

	if canLatin1 {
		// Store as Latin-1
		// Use utf8.RuneCountInString to avoid intermediate slice allocation
		runeCount := utf8.RuneCountInString(s)
		if runeCount > math.MaxUint32 {
			return 0, 0, fmt.Errorf("Latin-1 string too large: %d characters", runeCount)
		}
		data := make([]byte, runeCount)
		idx := 0
		for _, r := range s {
			data[idx] = byte(r)
			idx++
		}
		ptr, err := ctx.Realloc(0, 0, 1, uint32(len(data)))
		if err != nil {
			return 0, 0, fmt.Errorf("realloc for Latin-1 string: %w", err)
		}
		if !ctx.Memory.Write(ptr, data) {
			return 0, 0, fmt.Errorf("failed to write Latin-1 string at %d", ptr)
		}
		return ptr, uint32(len(data)), nil
	}

	// Fall back to UTF-16
	ptr, codeUnits, err := lowerStringUTF16(ctx, s)
	if err != nil {
		return 0, 0, err
	}
	return ptr, codeUnits | utf16Tag, nil
}
