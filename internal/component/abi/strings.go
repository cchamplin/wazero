package abi

import (
	"fmt"
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
// Placeholder for Task 62.
func liftStringUTF16(ctx *LiftContext, ptr, codeUnits uint32) (string, error) {
	return "", fmt.Errorf("UTF-16 string encoding not yet implemented")
}

// liftStringLatin1UTF16 lifts a Latin1+UTF16 encoded string from memory.
// Placeholder for Task 63.
func liftStringLatin1UTF16(ctx *LiftContext, ptr, taggedLen uint32) (string, error) {
	return "", fmt.Errorf("Latin1+UTF16 string encoding not yet implemented")
}
