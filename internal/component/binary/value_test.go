// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package binary

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/tetratelabs/wazero/internal/component/types"
)

func TestDecodeValueSection(t *testing.T) {
	// Per spec: value ::= t:<valtype> len:<core:u32> v:<val(t)>
	// For s32: val(s32) is 4 bytes little-endian.
	data := buildComponentWithSection(SectionIDValue, []byte{
		0x01,                   // count = 1
		0x7a,                   // s32 type
		0x04,                   // len = 4 bytes
		0x2a, 0x00, 0x00, 0x00, // value = 42 (little-endian)
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(c.Values) != 1 {
		t.Fatalf("expected 1 value, got %d", len(c.Values))
	}
	if c.Values[0].Type != types.S32 {
		t.Errorf("expected s32 type, got %v", c.Values[0].Type)
	}
	if len(c.Values[0].Data) != 4 {
		t.Errorf("expected 4 bytes of data, got %d", len(c.Values[0].Data))
	}
}

func TestDecodeValueSectionAllPrimitives(t *testing.T) {
	tests := []struct {
		name     string
		typeCode byte
		length   byte
		data     []byte
		want     types.ValType
	}{
		{"bool_false", byte(PrimValTypeBool), 1, []byte{0x00}, types.Bool},
		{"bool_true", byte(PrimValTypeBool), 1, []byte{0x01}, types.Bool},
		{"u8", byte(PrimValTypeU8), 1, []byte{0x42}, types.U8},
		{"s8", byte(PrimValTypeS8), 1, []byte{0xff}, types.S8},
		{"u16", byte(PrimValTypeU16), 2, []byte{0x34, 0x12}, types.U16},
		{"s16", byte(PrimValTypeS16), 2, []byte{0xff, 0xff}, types.S16},
		{"u32", byte(PrimValTypeU32), 4, []byte{0x78, 0x56, 0x34, 0x12}, types.U32},
		{"s32", byte(PrimValTypeS32), 4, []byte{0xff, 0xff, 0xff, 0xff}, types.S32},
		{"u64", byte(PrimValTypeU64), 8, []byte{0xef, 0xcd, 0xab, 0x90, 0x78, 0x56, 0x34, 0x12}, types.U64},
		{"s64", byte(PrimValTypeS64), 8, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, types.S64},
		{"f32", byte(PrimValTypeF32), 4, []byte{0x00, 0x00, 0x80, 0x3f}, types.F32},
		{"f64", byte(PrimValTypeF64), 8, []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xf0, 0x3f}, types.F64},
		{"char_ascii", byte(PrimValTypeChar), 1, []byte{0x41}, types.Char},
		{"char_utf8", byte(PrimValTypeChar), 3, []byte{0xe2, 0x9c, 0x93}, types.Char},
		{"string_empty", byte(PrimValTypeString), 1, []byte{0x00}, types.String_},
		{"string_hello", byte(PrimValTypeString), 6, []byte{0x05, 'h', 'e', 'l', 'l', 'o'}, types.String_},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sectionData := []byte{0x01, tt.typeCode, tt.length}
			sectionData = append(sectionData, tt.data...)

			data := buildComponentWithSection(SectionIDValue, sectionData)

			c, err := DecodeComponent(data)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(c.Values) != 1 {
				t.Fatalf("expected 1 value, got %d", len(c.Values))
			}
			if c.Values[0].Type != tt.want {
				t.Errorf("expected type %v, got %v", tt.want, c.Values[0].Type)
			}
			if len(c.Values[0].Data) != int(tt.length) {
				t.Errorf("expected %d bytes of data, got %d", tt.length, len(c.Values[0].Data))
			}
		})
	}
}

func TestDecodeValue(t *testing.T) {
	tests := []struct {
		name     string
		valType  types.ValType
		data     []byte
		expected interface{}
	}{
		{"bool_false", types.Bool, []byte{0x00}, false},
		{"bool_true", types.Bool, []byte{0x01}, true},
		{"u8", types.U8, []byte{0x42}, uint8(0x42)},
		{"s8_positive", types.S8, []byte{0x7f}, int8(127)},
		{"s8_negative", types.S8, []byte{0xff}, int8(-1)},
		{"u16", types.U16, []byte{0x34, 0x12}, uint16(0x1234)},
		{"s16_negative", types.S16, []byte{0xff, 0xff}, int16(-1)},
		{"u32", types.U32, []byte{0x78, 0x56, 0x34, 0x12}, uint32(0x12345678)},
		{"s32_negative", types.S32, []byte{0xff, 0xff, 0xff, 0xff}, int32(-1)},
		{"u64", types.U64, []byte{0xef, 0xcd, 0xab, 0x90, 0x78, 0x56, 0x34, 0x12}, uint64(0x1234567890abcdef)},
		{"s64_negative", types.S64, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, int64(-1)},
		{"f32_one", types.F32, []byte{0x00, 0x00, 0x80, 0x3f}, float32(1.0)},
		{"f64_one", types.F64, []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xf0, 0x3f}, float64(1.0)},
		{"char_A", types.Char, []byte{0x41}, rune('A')},
		{"char_checkmark", types.Char, []byte{0xe2, 0x9c, 0x93}, rune('\u2713')},
		{"string_empty", types.String_, []byte{0x00}, ""},
		{"string_hello", types.String_, []byte{0x05, 'h', 'e', 'l', 'l', 'o'}, "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := decodeValue(tt.valType, tt.data)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result != tt.expected {
				t.Errorf("expected %v (%T), got %v (%T)", tt.expected, tt.expected, result, result)
			}
		})
	}
}

func TestDecodeValueF32NaN(t *testing.T) {
	data := []byte{0x00, 0x00, 0xc0, 0x7f}
	result, err := decodeValue(types.F32, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.(float32)
	if !ok {
		t.Fatalf("expected float32, got %T", result)
	}
	if !math.IsNaN(float64(f)) {
		t.Errorf("expected NaN, got %v", f)
	}
}

func TestDecodeValueF64NaN(t *testing.T) {
	data := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xf8, 0x7f}
	result, err := decodeValue(types.F64, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.(float64)
	if !ok {
		t.Fatalf("expected float64, got %T", result)
	}
	if !math.IsNaN(f) {
		t.Errorf("expected NaN, got %v", f)
	}
}

func TestDecodeValueMultipleValues(t *testing.T) {
	sectionData := []byte{
		0x03, // count = 3

		// Value 0: bool true
		byte(PrimValTypeBool), // type
		0x01,                  // len = 1
		0x01,                  // true

		// Value 1: u32 = 0x12345678
		byte(PrimValTypeU32),   // type
		0x04,                   // len = 4
		0x78, 0x56, 0x34, 0x12, // value (little-endian)

		// Value 2: string "hi"
		byte(PrimValTypeString), // type
		0x03,                    // len = 3 (1 for length prefix + 2 for "hi")
		0x02, 'h', 'i',          // length-prefixed string
	}

	data := buildComponentWithSection(SectionIDValue, sectionData)

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(c.Values) != 3 {
		t.Fatalf("expected 3 values, got %d", len(c.Values))
	}

	if c.Values[0].Type != types.Bool {
		t.Errorf("value 0: expected bool, got %v", c.Values[0].Type)
	}
	v0, err := decodeValue(c.Values[0].Type, c.Values[0].Data)
	if err != nil {
		t.Fatalf("value 0: decode error: %v", err)
	}
	if v0 != true {
		t.Errorf("value 0: expected true, got %v", v0)
	}

	if c.Values[1].Type != types.U32 {
		t.Errorf("value 1: expected u32, got %v", c.Values[1].Type)
	}
	v1, err := decodeValue(c.Values[1].Type, c.Values[1].Data)
	if err != nil {
		t.Fatalf("value 1: decode error: %v", err)
	}
	if v1 != uint32(0x12345678) {
		t.Errorf("value 1: expected 0x12345678, got %v", v1)
	}

	if c.Values[2].Type != types.String_ {
		t.Errorf("value 2: expected string, got %v", c.Values[2].Type)
	}
	v2, err := decodeValue(c.Values[2].Type, c.Values[2].Data)
	if err != nil {
		t.Fatalf("value 2: decode error: %v", err)
	}
	if v2 != "hi" {
		t.Errorf("value 2: expected 'hi', got %v", v2)
	}
}

func TestDecodeValueEmptySection(t *testing.T) {
	sectionData := []byte{0x00} // count = 0
	data := buildComponentWithSection(SectionIDValue, sectionData)

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(c.Values) != 0 {
		t.Errorf("expected 0 values, got %d", len(c.Values))
	}
}

// Test helper to build f32 bytes from float32
func f32ToBytes(f float32) []byte {
	bits := math.Float32bits(f)
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, bits)
	return b
}

// Test helper to build f64 bytes from float64
func f64ToBytes(f float64) []byte {
	bits := math.Float64bits(f)
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, bits)
	return b
}
