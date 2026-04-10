// internal/component/binary/valtype.go

package binary

import "fmt"

// PrimValType represents primitive value type opcodes in the component binary format.
// These are encoded as negative SLEB128 values starting at 0x7f.
// See: https://github.com/WebAssembly/component-model/blob/main/design/mvp/Binary.md
type PrimValType byte

const (
	PrimValTypeBool         PrimValType = 0x7f
	PrimValTypeS8           PrimValType = 0x7e
	PrimValTypeU8           PrimValType = 0x7d
	PrimValTypeS16          PrimValType = 0x7c
	PrimValTypeU16          PrimValType = 0x7b
	PrimValTypeS32          PrimValType = 0x7a
	PrimValTypeU32          PrimValType = 0x79
	PrimValTypeS64          PrimValType = 0x78
	PrimValTypeU64          PrimValType = 0x77
	PrimValTypeF32          PrimValType = 0x76
	PrimValTypeF64          PrimValType = 0x75
	PrimValTypeChar         PrimValType = 0x74
	PrimValTypeString       PrimValType = 0x73
	PrimValTypeErrorContext PrimValType = 0x64
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
	case PrimValTypeErrorContext:
		return "error-context"
	default:
		return fmt.Sprintf("unknown(0x%02x)", byte(p))
	}
}

// IsPrimValType returns true if the byte is a valid primitive valtype opcode.
// Primitive valtypes are in the range 0x73-0x7f (negative SLEB128), plus 0x64 (error-context).
func IsPrimValType(b byte) bool {
	return (b >= 0x73 && b <= 0x7f) || b == 0x64
}

// Composite type opcodes
// See: https://github.com/WebAssembly/component-model/blob/main/design/mvp/Binary.md
const (
	ValTypeOpcodeRecord  byte = 0x72
	ValTypeOpcodeVariant byte = 0x71
	ValTypeOpcodeList    byte = 0x70
	ValTypeOpcodeTuple   byte = 0x6f
	ValTypeOpcodeFlags   byte = 0x6e
	ValTypeOpcodeEnum    byte = 0x6d
	ValTypeOpcodeOption  byte = 0x6b
	ValTypeOpcodeResult  byte = 0x6a
)

// Handle type opcodes
// See: https://github.com/WebAssembly/component-model/blob/main/design/mvp/Binary.md
const (
	ValTypeOpcodeBorrow byte = 0x68 // borrow<T> handle type
	ValTypeOpcodeOwn    byte = 0x69 // own<T> handle type
)

// Async type opcodes
// See: https://github.com/WebAssembly/component-model/blob/main/design/mvp/Binary.md
const (
	ValTypeOpcodeMap            byte = 0x63 // map<K, V> type (component_model_map proposal)
	ValTypeOpcodeFuture         byte = 0x65 // future<T> type
	ValTypeOpcodeStream         byte = 0x66 // stream<T, E> type
	ValTypeOpcodeFixedSizeList  byte = 0x67 // list<T, N> fixed-size list type
)

// IsCompositeTypeOpcode returns true if the opcode is a composite type.
func IsCompositeTypeOpcode(opcode byte) bool {
	return (opcode >= 0x6a && opcode <= 0x72) || opcode == ValTypeOpcodeMap
}
