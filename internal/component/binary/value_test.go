// internal/component/binary/value_test.go
package binary

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
)

func TestDecodeValueSection(t *testing.T) {
	// Per spec: value ::= t:<valtype> len:<core:u32> v:<val(t)>
	// For s32: val(s32) is 4 bytes little-endian
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

	if !c.Values[0].Type.IsPrimitive {
		t.Error("expected primitive type")
	}

	if c.Values[0].Type.Primitive != byte(PrimValTypeS32) {
		t.Errorf("expected s32 type (0x7a), got 0x%02x", c.Values[0].Type.Primitive)
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
	}{
		{"bool_false", byte(PrimValTypeBool), 1, []byte{0x00}},
		{"bool_true", byte(PrimValTypeBool), 1, []byte{0x01}},
		{"u8", byte(PrimValTypeU8), 1, []byte{0x42}},
		{"s8", byte(PrimValTypeS8), 1, []byte{0xff}}, // -1
		{"u16", byte(PrimValTypeU16), 2, []byte{0x34, 0x12}},
		{"s16", byte(PrimValTypeS16), 2, []byte{0xff, 0xff}}, // -1
		{"u32", byte(PrimValTypeU32), 4, []byte{0x78, 0x56, 0x34, 0x12}},
		{"s32", byte(PrimValTypeS32), 4, []byte{0xff, 0xff, 0xff, 0xff}}, // -1
		{"u64", byte(PrimValTypeU64), 8, []byte{0xef, 0xcd, 0xab, 0x90, 0x78, 0x56, 0x34, 0x12}},
		{"s64", byte(PrimValTypeS64), 8, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}}, // -1
		{"f32", byte(PrimValTypeF32), 4, []byte{0x00, 0x00, 0x80, 0x3f}},                          // 1.0
		{"f64", byte(PrimValTypeF64), 8, []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xf0, 0x3f}},  // 1.0
		{"char_ascii", byte(PrimValTypeChar), 1, []byte{0x41}},                                    // 'A'
		{"char_utf8", byte(PrimValTypeChar), 3, []byte{0xe2, 0x9c, 0x93}},                         // U+2713 (checkmark)
		{"string_empty", byte(PrimValTypeString), 1, []byte{0x00}},                               // empty string (length prefix only)
		{"string_hello", byte(PrimValTypeString), 6, []byte{0x05, 'h', 'e', 'l', 'l', 'o'}},      // "hello"
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

			if !c.Values[0].Type.IsPrimitive {
				t.Error("expected primitive type")
			}

			if c.Values[0].Type.Primitive != tt.typeCode {
				t.Errorf("expected type 0x%02x, got 0x%02x", tt.typeCode, c.Values[0].Type.Primitive)
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
		valType  component.ValTypeRef
		data     []byte
		expected interface{}
	}{
		{
			name:     "bool_false",
			valType:  component.ValTypeRef{IsPrimitive: true, Primitive: byte(PrimValTypeBool)},
			data:     []byte{0x00},
			expected: false,
		},
		{
			name:     "bool_true",
			valType:  component.ValTypeRef{IsPrimitive: true, Primitive: byte(PrimValTypeBool)},
			data:     []byte{0x01},
			expected: true,
		},
		{
			name:     "u8",
			valType:  component.ValTypeRef{IsPrimitive: true, Primitive: byte(PrimValTypeU8)},
			data:     []byte{0x42},
			expected: uint8(0x42),
		},
		{
			name:     "s8_positive",
			valType:  component.ValTypeRef{IsPrimitive: true, Primitive: byte(PrimValTypeS8)},
			data:     []byte{0x7f},
			expected: int8(127),
		},
		{
			name:     "s8_negative",
			valType:  component.ValTypeRef{IsPrimitive: true, Primitive: byte(PrimValTypeS8)},
			data:     []byte{0xff},
			expected: int8(-1),
		},
		{
			name:     "u16",
			valType:  component.ValTypeRef{IsPrimitive: true, Primitive: byte(PrimValTypeU16)},
			data:     []byte{0x34, 0x12},
			expected: uint16(0x1234),
		},
		{
			name:     "s16_negative",
			valType:  component.ValTypeRef{IsPrimitive: true, Primitive: byte(PrimValTypeS16)},
			data:     []byte{0xff, 0xff},
			expected: int16(-1),
		},
		{
			name:     "u32",
			valType:  component.ValTypeRef{IsPrimitive: true, Primitive: byte(PrimValTypeU32)},
			data:     []byte{0x78, 0x56, 0x34, 0x12},
			expected: uint32(0x12345678),
		},
		{
			name:     "s32_negative",
			valType:  component.ValTypeRef{IsPrimitive: true, Primitive: byte(PrimValTypeS32)},
			data:     []byte{0xff, 0xff, 0xff, 0xff},
			expected: int32(-1),
		},
		{
			name:     "u64",
			valType:  component.ValTypeRef{IsPrimitive: true, Primitive: byte(PrimValTypeU64)},
			data:     []byte{0xef, 0xcd, 0xab, 0x90, 0x78, 0x56, 0x34, 0x12},
			expected: uint64(0x1234567890abcdef),
		},
		{
			name:     "s64_negative",
			valType:  component.ValTypeRef{IsPrimitive: true, Primitive: byte(PrimValTypeS64)},
			data:     []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
			expected: int64(-1),
		},
		{
			name:     "f32_one",
			valType:  component.ValTypeRef{IsPrimitive: true, Primitive: byte(PrimValTypeF32)},
			data:     []byte{0x00, 0x00, 0x80, 0x3f},
			expected: float32(1.0),
		},
		{
			name:     "f64_one",
			valType:  component.ValTypeRef{IsPrimitive: true, Primitive: byte(PrimValTypeF64)},
			data:     []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xf0, 0x3f},
			expected: float64(1.0),
		},
		{
			name:     "char_A",
			valType:  component.ValTypeRef{IsPrimitive: true, Primitive: byte(PrimValTypeChar)},
			data:     []byte{0x41},
			expected: rune('A'),
		},
		{
			name:     "char_checkmark",
			valType:  component.ValTypeRef{IsPrimitive: true, Primitive: byte(PrimValTypeChar)},
			data:     []byte{0xe2, 0x9c, 0x93}, // U+2713
			expected: rune('\u2713'),
		},
		{
			name:     "string_empty",
			valType:  component.ValTypeRef{IsPrimitive: true, Primitive: byte(PrimValTypeString)},
			data:     []byte{0x00},
			expected: "",
		},
		{
			name:     "string_hello",
			valType:  component.ValTypeRef{IsPrimitive: true, Primitive: byte(PrimValTypeString)},
			data:     []byte{0x05, 'h', 'e', 'l', 'l', 'o'},
			expected: "hello",
		},
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
	// Per spec: canonical NaN for f32 is 0x00 0x00 0xC0 0x7F (little-endian)
	data := []byte{0x00, 0x00, 0xc0, 0x7f}
	valType := component.ValTypeRef{IsPrimitive: true, Primitive: byte(PrimValTypeF32)}

	result, err := decodeValue(valType, data)
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
	// Per spec: canonical NaN for f64 is 0x00 0x00 0x00 0x00 0x00 0x00 0xF8 0x7F (little-endian)
	data := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xf8, 0x7f}
	valType := component.ValTypeRef{IsPrimitive: true, Primitive: byte(PrimValTypeF64)}

	result, err := decodeValue(valType, data)
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
	// Test decoding multiple values in a section
	sectionData := []byte{
		0x03, // count = 3

		// Value 0: bool true
		byte(PrimValTypeBool), // type
		0x01,                  // len = 1
		0x01,                  // true

		// Value 1: u32 = 0x12345678
		byte(PrimValTypeU32),       // type
		0x04,                       // len = 4
		0x78, 0x56, 0x34, 0x12,     // value (little-endian)

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

	// Check value 0: bool
	if c.Values[0].Type.Primitive != byte(PrimValTypeBool) {
		t.Errorf("value 0: expected bool type, got 0x%02x", c.Values[0].Type.Primitive)
	}
	v0, err := decodeValue(c.Values[0].Type, c.Values[0].Data)
	if err != nil {
		t.Fatalf("value 0: decode error: %v", err)
	}
	if v0 != true {
		t.Errorf("value 0: expected true, got %v", v0)
	}

	// Check value 1: u32
	if c.Values[1].Type.Primitive != byte(PrimValTypeU32) {
		t.Errorf("value 1: expected u32 type, got 0x%02x", c.Values[1].Type.Primitive)
	}
	v1, err := decodeValue(c.Values[1].Type, c.Values[1].Data)
	if err != nil {
		t.Fatalf("value 1: decode error: %v", err)
	}
	if v1 != uint32(0x12345678) {
		t.Errorf("value 1: expected 0x12345678, got %v", v1)
	}

	// Check value 2: string
	if c.Values[2].Type.Primitive != byte(PrimValTypeString) {
		t.Errorf("value 2: expected string type, got 0x%02x", c.Values[2].Type.Primitive)
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
	// Test empty value section
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

func TestDecodeValueTypeIndexRef(t *testing.T) {
	// Test value with type index reference (non-primitive)
	// The type index is encoded as a u32 in the valtype position
	sectionData := []byte{
		0x01, // count = 1
		0x05, // type index = 5 (not a primitive, so IsPrimitive should be false)
		0x04, // len = 4
		0x01, 0x02, 0x03, 0x04, // some data
	}

	data := buildComponentWithSection(SectionIDValue, sectionData)

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(c.Values) != 1 {
		t.Fatalf("expected 1 value, got %d", len(c.Values))
	}

	if c.Values[0].Type.IsPrimitive {
		t.Error("expected non-primitive type (type index reference)")
	}

	if c.Values[0].Type.TypeIdx != 5 {
		t.Errorf("expected type index 5, got %d", c.Values[0].Type.TypeIdx)
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
