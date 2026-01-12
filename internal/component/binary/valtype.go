// internal/component/binary/valtype.go

package binary

import "fmt"

// PrimValType represents primitive value type opcodes in the component binary format.
// These are encoded as negative SLEB128 values starting at 0x7f.
// See: https://github.com/WebAssembly/component-model/blob/main/design/mvp/Binary.md
type PrimValType byte

const (
	PrimValTypeBool   PrimValType = 0x7f
	PrimValTypeS8     PrimValType = 0x7e
	PrimValTypeU8     PrimValType = 0x7d
	PrimValTypeS16    PrimValType = 0x7c
	PrimValTypeU16    PrimValType = 0x7b
	PrimValTypeS32    PrimValType = 0x7a
	PrimValTypeU32    PrimValType = 0x79
	PrimValTypeS64    PrimValType = 0x78
	PrimValTypeU64    PrimValType = 0x77
	PrimValTypeF32    PrimValType = 0x76
	PrimValTypeF64    PrimValType = 0x75
	PrimValTypeChar   PrimValType = 0x74
	PrimValTypeString PrimValType = 0x73
)

// String returns a human-readable name for the primitive value type.
func (p PrimValType) String() string {
	switch p {
	case PrimValTypeBool:
		return "bool"
	case PrimValTypeS8:
		return "s8"
	case PrimValTypeU8:
		return "u8"
	case PrimValTypeS16:
		return "s16"
	case PrimValTypeU16:
		return "u16"
	case PrimValTypeS32:
		return "s32"
	case PrimValTypeU32:
		return "u32"
	case PrimValTypeS64:
		return "s64"
	case PrimValTypeU64:
		return "u64"
	case PrimValTypeF32:
		return "f32"
	case PrimValTypeF64:
		return "f64"
	case PrimValTypeChar:
		return "char"
	case PrimValTypeString:
		return "string"
	default:
		return fmt.Sprintf("unknown(0x%02x)", byte(p))
	}
}

// IsPrimValType returns true if the byte is a valid primitive valtype opcode.
// Primitive valtypes are in the range 0x73-0x7f (negative SLEB128).
func IsPrimValType(b byte) bool {
	return b >= 0x73 && b <= 0x7f
}
