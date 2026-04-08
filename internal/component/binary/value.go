// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package binary

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/leb128"
)

// decodeValueSection decodes the value section (section 12) of a component binary.
// Per spec: value ::= t:<valtype> len:<core:u32> v:<val(t)>
// where len is the byte length of the encoded value v.
func decodeValueSection(dc *decodeContext, r *bytes.Reader) error {
	count, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("read value count: %w", err)
	}

	c := dc.c
	c.Values = make([]component.ValueDef, count)
	for i := uint32(0); i < count; i++ {
		valType, err := decodeValType(r, dc.scope, dc.builder)
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
// For primitive types this returns the interpreted Go value. Composite
// types (records, variants, lists, etc.) are returned as raw bytes in
// Session 0; the caller consults *types.ComponentTypes to interpret the
// shape at lift/lower time.
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
func decodeValue(valType types.ValType, data []byte) (interface{}, error) {
	switch valType.Kind {
	case types.TypeKindBool:
		if len(data) != 1 {
			return nil, fmt.Errorf("bool value requires 1 byte, got %d", len(data))
		}
		return data[0] != 0, nil

	case types.TypeKindS8:
		if len(data) != 1 {
			return nil, fmt.Errorf("s8 value requires 1 byte, got %d", len(data))
		}
		return int8(data[0]), nil

	case types.TypeKindU8:
		if len(data) != 1 {
			return nil, fmt.Errorf("u8 value requires 1 byte, got %d", len(data))
		}
		return data[0], nil

	case types.TypeKindS16:
		if len(data) != 2 {
			return nil, fmt.Errorf("s16 value requires 2 bytes, got %d", len(data))
		}
		return int16(binary.LittleEndian.Uint16(data)), nil

	case types.TypeKindU16:
		if len(data) != 2 {
			return nil, fmt.Errorf("u16 value requires 2 bytes, got %d", len(data))
		}
		return binary.LittleEndian.Uint16(data), nil

	case types.TypeKindS32:
		if len(data) != 4 {
			return nil, fmt.Errorf("s32 value requires 4 bytes, got %d", len(data))
		}
		return int32(binary.LittleEndian.Uint32(data)), nil

	case types.TypeKindU32:
		if len(data) != 4 {
			return nil, fmt.Errorf("u32 value requires 4 bytes, got %d", len(data))
		}
		return binary.LittleEndian.Uint32(data), nil

	case types.TypeKindS64:
		if len(data) != 8 {
			return nil, fmt.Errorf("s64 value requires 8 bytes, got %d", len(data))
		}
		return int64(binary.LittleEndian.Uint64(data)), nil

	case types.TypeKindU64:
		if len(data) != 8 {
			return nil, fmt.Errorf("u64 value requires 8 bytes, got %d", len(data))
		}
		return binary.LittleEndian.Uint64(data), nil

	case types.TypeKindF32:
		if len(data) != 4 {
			return nil, fmt.Errorf("f32 value requires 4 bytes, got %d", len(data))
		}
		bits := binary.LittleEndian.Uint32(data)
		return float32FromBits(bits), nil

	case types.TypeKindF64:
		if len(data) != 8 {
			return nil, fmt.Errorf("f64 value requires 8 bytes, got %d", len(data))
		}
		bits := binary.LittleEndian.Uint64(data)
		return float64FromBits(bits), nil

	case types.TypeKindChar:
		if len(data) == 0 || len(data) > 4 {
			return nil, fmt.Errorf("char value requires 1-4 UTF-8 bytes, got %d", len(data))
		}
		runes := []rune(string(data))
		if len(runes) != 1 {
			return nil, fmt.Errorf("char value must decode to single unicode scalar, got %d runes", len(runes))
		}
		return runes[0], nil

	case types.TypeKindString:
		// String is encoded as core:name (length-prefixed UTF-8).
		if len(data) == 0 {
			return "", nil
		}
		rr := bytes.NewReader(data)
		strLen, bytesRead, err := leb128.DecodeUint32(rr)
		if err != nil {
			return nil, fmt.Errorf("decode string length: %w", err)
		}
		n := int(bytesRead)
		if int(strLen) > len(data)-n {
			return nil, fmt.Errorf("string length %d exceeds available data %d", strLen, len(data)-n)
		}
		return string(data[n : n+int(strLen)]), nil

	default:
		// For non-primitive types (composites, handles, error-context)
		// the data is returned verbatim; the caller interprets it using
		// the component's *types.ComponentTypes table.
		return data, nil
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
