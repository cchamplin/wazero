package binary

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/leb128"
)

// decodeValueSection decodes the value section (section 12) of a component binary.
// Per spec: value ::= t:<valtype> len:<core:u32> v:<val(t)>
// where len is the byte length of the encoded value v.
func decodeValueSection(c *component.Component, r *bytes.Reader) error {
	count, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("read value count: %w", err)
	}

	c.Values = make([]component.ValueDef, count)
	for i := uint32(0); i < count; i++ {
		valType, err := decodeValType(r)
		if err != nil {
			return fmt.Errorf("read value %d type: %w", i, err)
		}

		// Per spec: read the length prefix (len:<core:u32>)
		valLen, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return fmt.Errorf("read value %d length: %w", i, err)
		}

		// Read the value bytes according to the length
		data := make([]byte, valLen)
		if valLen > 0 {
			if _, err := io.ReadFull(r, data); err != nil {
				return fmt.Errorf("read value %d data: %w", i, err)
			}
		}

		c.Values[i] = component.ValueDef{
			Type: valType,
			Data: data,
		}
	}

	return nil
}

// decodeValue decodes a single value of the given type from raw bytes.
// This is used to interpret the Data field of ValueDef based on its Type.
// For primitive types, this returns the interpreted Go value.
// For composite types, this may return structured data.
//
// The val(T) grammar from the spec:
//   - val(bool):   0x00 => false, 0x01 => true
//   - val(u8):     v:<core:byte> => v
//   - val(s8):     v:<core:byte> => v' (signed interpretation)
//   - val(s16):    v:<core:s16> (little-endian 2 bytes)
//   - val(u16):    v:<core:u16> (little-endian 2 bytes)
//   - val(s32):    v:<core:s32> (little-endian 4 bytes)
//   - val(u32):    v:<core:u32> (little-endian 4 bytes)
//   - val(s64):    v:<core:s64> (little-endian 8 bytes)
//   - val(u64):    v:<core:u64> (little-endian 8 bytes)
//   - val(f32):    v:<core:f32> (little-endian 4 bytes IEEE 754)
//   - val(f64):    v:<core:f64> (little-endian 8 bytes IEEE 754)
//   - val(char):   b*:<core:byte>* (UTF-8 encoded character)
//   - val(string): v:<core:name> (length-prefixed UTF-8 string)
func decodeValue(valType component.ValTypeRef, data []byte) (interface{}, error) {
	if !valType.IsPrimitive {
		// For non-primitive types (type index references, handles),
		// the data contains the raw bytes which need type-specific interpretation
		return data, nil
	}

	r := bytes.NewReader(data)

	switch valType.Primitive {
	case byte(PrimValTypeBool): // 0x7f
		if len(data) != 1 {
			return nil, fmt.Errorf("bool value requires 1 byte, got %d", len(data))
		}
		return data[0] != 0, nil

	case byte(PrimValTypeS8): // 0x7e
		if len(data) != 1 {
			return nil, fmt.Errorf("s8 value requires 1 byte, got %d", len(data))
		}
		return int8(data[0]), nil

	case byte(PrimValTypeU8): // 0x7d
		if len(data) != 1 {
			return nil, fmt.Errorf("u8 value requires 1 byte, got %d", len(data))
		}
		return data[0], nil

	case byte(PrimValTypeS16): // 0x7c
		if len(data) != 2 {
			return nil, fmt.Errorf("s16 value requires 2 bytes, got %d", len(data))
		}
		return int16(binary.LittleEndian.Uint16(data)), nil

	case byte(PrimValTypeU16): // 0x7b
		if len(data) != 2 {
			return nil, fmt.Errorf("u16 value requires 2 bytes, got %d", len(data))
		}
		return binary.LittleEndian.Uint16(data), nil

	case byte(PrimValTypeS32): // 0x7a
		if len(data) != 4 {
			return nil, fmt.Errorf("s32 value requires 4 bytes, got %d", len(data))
		}
		return int32(binary.LittleEndian.Uint32(data)), nil

	case byte(PrimValTypeU32): // 0x79
		if len(data) != 4 {
			return nil, fmt.Errorf("u32 value requires 4 bytes, got %d", len(data))
		}
		return binary.LittleEndian.Uint32(data), nil

	case byte(PrimValTypeS64): // 0x78
		if len(data) != 8 {
			return nil, fmt.Errorf("s64 value requires 8 bytes, got %d", len(data))
		}
		return int64(binary.LittleEndian.Uint64(data)), nil

	case byte(PrimValTypeU64): // 0x77
		if len(data) != 8 {
			return nil, fmt.Errorf("u64 value requires 8 bytes, got %d", len(data))
		}
		return binary.LittleEndian.Uint64(data), nil

	case byte(PrimValTypeF32): // 0x76
		if len(data) != 4 {
			return nil, fmt.Errorf("f32 value requires 4 bytes, got %d", len(data))
		}
		bits := binary.LittleEndian.Uint32(data)
		return float32FromBits(bits), nil

	case byte(PrimValTypeF64): // 0x75
		if len(data) != 8 {
			return nil, fmt.Errorf("f64 value requires 8 bytes, got %d", len(data))
		}
		bits := binary.LittleEndian.Uint64(data)
		return float64FromBits(bits), nil

	case byte(PrimValTypeChar): // 0x74
		// Char is encoded as UTF-8 bytes
		if len(data) == 0 || len(data) > 4 {
			return nil, fmt.Errorf("char value requires 1-4 UTF-8 bytes, got %d", len(data))
		}
		runes := []rune(string(data))
		if len(runes) != 1 {
			return nil, fmt.Errorf("char value must decode to single unicode scalar, got %d runes", len(runes))
		}
		return runes[0], nil

	case byte(PrimValTypeString): // 0x73
		// String is encoded as core:name (length-prefixed UTF-8)
		// The data should be the raw UTF-8 bytes (length already accounted for in the outer len field)
		// Per spec, val(string) ::= v:<core:name> where core:name is length + UTF-8 bytes
		// However, since we already have the length-bounded data, we read the inner length + string
		if len(data) == 0 {
			return "", nil
		}
		strLen, bytesRead, err := leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("decode string length: %w", err)
		}
		n := int(bytesRead)
		if int(strLen) > len(data)-n {
			return nil, fmt.Errorf("string length %d exceeds available data %d", strLen, len(data)-n)
		}
		return string(data[n : n+int(strLen)]), nil

	case byte(PrimValTypeErrorContext): // 0x64
		// Error context is implementation-specific
		return data, nil

	default:
		return nil, fmt.Errorf("unknown primitive type 0x%02x", valType.Primitive)
	}
}

// float32FromBits converts uint32 bits to float32
func float32FromBits(bits uint32) float32 {
	return math.Float32frombits(bits)
}

// float64FromBits converts uint64 bits to float64
func float64FromBits(bits uint64) float64 {
	return math.Float64frombits(bits)
}

// Helper constants for canonical NaN representations per spec
const (
	// Canonical NaN for f32: 0x00 0x00 0xC0 0x7F (little-endian)
	canonicalNaN32Bits uint32 = 0x7FC00000
	// Canonical NaN for f64: 0x00 0x00 0x00 0x00 0x00 0x00 0xF8 0x7F (little-endian)
	canonicalNaN64Bits uint64 = 0x7FF8000000000000
)
