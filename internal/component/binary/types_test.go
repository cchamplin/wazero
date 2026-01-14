// internal/component/binary/types_test.go

package binary

import (
	"bytes"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestDecodeValType_Primitives(t *testing.T) {
	tests := []struct {
		name     string
		input    byte
		expected component.ValTypeRef
	}{
		{"bool", 0x7f, component.ValTypeRef{IsPrimitive: true, Primitive: 0x7f}},
		{"s8", 0x7e, component.ValTypeRef{IsPrimitive: true, Primitive: 0x7e}},
		{"u8", 0x7d, component.ValTypeRef{IsPrimitive: true, Primitive: 0x7d}},
		{"s16", 0x7c, component.ValTypeRef{IsPrimitive: true, Primitive: 0x7c}},
		{"u16", 0x7b, component.ValTypeRef{IsPrimitive: true, Primitive: 0x7b}},
		{"s32", 0x7a, component.ValTypeRef{IsPrimitive: true, Primitive: 0x7a}},
		{"u32", 0x79, component.ValTypeRef{IsPrimitive: true, Primitive: 0x79}},
		{"s64", 0x78, component.ValTypeRef{IsPrimitive: true, Primitive: 0x78}},
		{"u64", 0x77, component.ValTypeRef{IsPrimitive: true, Primitive: 0x77}},
		{"f32", 0x76, component.ValTypeRef{IsPrimitive: true, Primitive: 0x76}},
		{"f64", 0x75, component.ValTypeRef{IsPrimitive: true, Primitive: 0x75}},
		{"char", 0x74, component.ValTypeRef{IsPrimitive: true, Primitive: 0x74}},
		{"string", 0x73, component.ValTypeRef{IsPrimitive: true, Primitive: 0x73}},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			r := bytes.NewReader([]byte{tc.input})
			result, err := decodeValType(r)
			require.NoError(t, err)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestDecodeValType_TypeIndex(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected component.ValTypeRef
	}{
		{"index 0", []byte{0x00}, component.ValTypeRef{IsPrimitive: false, TypeIdx: 0}},
		{"index 5", []byte{0x05}, component.ValTypeRef{IsPrimitive: false, TypeIdx: 5}},
		{"index 127", []byte{0x7f & 0x72}, component.ValTypeRef{IsPrimitive: false, TypeIdx: 0x72}}, // 0x72 is below primitive range
		// Multi-byte LEB128 type indices
		{"index 128", []byte{0x80, 0x01}, component.ValTypeRef{IsPrimitive: false, TypeIdx: 128}},            // 128 = 0x80 0x01 in LEB128
		{"index 255", []byte{0xff, 0x01}, component.ValTypeRef{IsPrimitive: false, TypeIdx: 255}},            // 255 = 0xff 0x01 in LEB128
		{"index 256", []byte{0x80, 0x02}, component.ValTypeRef{IsPrimitive: false, TypeIdx: 256}},            // 256 = 0x80 0x02 in LEB128
		{"index 16383", []byte{0xff, 0x7f}, component.ValTypeRef{IsPrimitive: false, TypeIdx: 16383}},        // 16383 = 0xff 0x7f in LEB128
		{"index 16384", []byte{0x80, 0x80, 0x01}, component.ValTypeRef{IsPrimitive: false, TypeIdx: 16384}},  // 16384 = 0x80 0x80 0x01 in LEB128
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			r := bytes.NewReader(tc.input)
			result, err := decodeValType(r)
			require.NoError(t, err)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestDecodeFuncType(t *testing.T) {
	// Encode: 0x40 (sync func) + params + results
	// params: vec(labelvaltype) = count + (name + valtype)*
	// results: 0x00 valtype (single result) or 0x01 0x00 (no results)

	// Function: (param "a" s32) (param "b" s32) -> s32
	input := []byte{
		0x40,      // sync functype
		0x02,      // 2 params
		0x01, 'a', // param name "a"
		0x7a,      // s32
		0x01, 'b', // param name "b"
		0x7a,      // s32
		0x00,      // single result
		0x7a,      // s32
	}

	r := bytes.NewReader(input)
	ft, err := decodeFuncType(r)
	require.NoError(t, err)
	require.NotNil(t, ft)
	require.Equal(t, 2, len(ft.Params))
	require.Equal(t, "a", ft.Params[0].Name)
	require.Equal(t, byte(0x7a), ft.Params[0].ValType.Primitive)
	require.True(t, ft.Params[0].ValType.IsPrimitive)
	require.Equal(t, "b", ft.Params[1].Name)
	require.Equal(t, byte(0x7a), ft.Params[1].ValType.Primitive)
	require.True(t, ft.Params[1].ValType.IsPrimitive)
	require.Equal(t, 1, len(ft.Results))
	require.Equal(t, byte(0x7a), ft.Results[0].ValType.Primitive)
}

func TestDecodeFuncType_NoParams(t *testing.T) {
	// Function: () -> string
	input := []byte{
		0x40, // sync functype
		0x00, // 0 params
		0x00, // single result
		0x73, // string
	}

	r := bytes.NewReader(input)
	ft, err := decodeFuncType(r)
	require.NoError(t, err)
	require.NotNil(t, ft)
	require.Equal(t, 0, len(ft.Params))
	require.Equal(t, 1, len(ft.Results))
	require.Equal(t, byte(0x73), ft.Results[0].ValType.Primitive)
}

func TestDecodeFuncType_NoResults(t *testing.T) {
	// Function: (param "x" u32) -> ()
	input := []byte{
		0x40,      // sync functype
		0x01,      // 1 param
		0x01, 'x', // param name "x"
		0x79,      // u32
		0x01,      // named results (vec)
		0x00,      // 0 results
	}

	r := bytes.NewReader(input)
	ft, err := decodeFuncType(r)
	require.NoError(t, err)
	require.NotNil(t, ft)
	require.Equal(t, 1, len(ft.Params))
	require.Equal(t, "x", ft.Params[0].Name)
	require.Equal(t, 0, len(ft.Results))
}

func TestDecodeFuncType_NamedResults(t *testing.T) {
	// Function: () -> (result "ok" bool) (result "err" string)
	input := []byte{
		0x40,       // sync functype
		0x00,       // 0 params
		0x01,       // named results (vec)
		0x02,       // 2 results
		0x02, 'o', 'k', // result name "ok"
		0x7f, // bool
		0x03, 'e', 'r', 'r', // result name "err"
		0x73, // string
	}

	r := bytes.NewReader(input)
	ft, err := decodeFuncType(r)
	require.NoError(t, err)
	require.NotNil(t, ft)
	require.Equal(t, 0, len(ft.Params))
	require.Equal(t, 2, len(ft.Results))
	require.Equal(t, "ok", ft.Results[0].Name)
	require.Equal(t, byte(0x7f), ft.Results[0].ValType.Primitive)
	require.Equal(t, "err", ft.Results[1].Name)
	require.Equal(t, byte(0x73), ft.Results[1].ValType.Primitive)
}

func TestDecodeFuncType_AsyncFunc(t *testing.T) {
	// Async function: async () -> s32
	input := []byte{
		0x43, // async functype
		0x00, // 0 params
		0x00, // single result
		0x7a, // s32
	}

	r := bytes.NewReader(input)
	ft, err := decodeFuncType(r)
	require.NoError(t, err)
	require.NotNil(t, ft)
	require.Equal(t, 0, len(ft.Params))
	require.Equal(t, 1, len(ft.Results))
}

func TestDecodeFuncType_InvalidOpcode(t *testing.T) {
	input := []byte{
		0x41, // component type opcode - not a function type
		0x00,
	}

	r := bytes.NewReader(input)
	_, err := decodeFuncType(r)
	require.Error(t, err)
}

func TestDecodeName(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{"empty", []byte{0x00}, ""},
		{"single char", []byte{0x01, 'a'}, "a"},
		{"multiple chars", []byte{0x05, 'h', 'e', 'l', 'l', 'o'}, "hello"},
		{"utf8", []byte{0x04, 0xc3, 0xa9, 0x6c, 0x69}, "\xc3\xa9li"}, // UTF-8 encoding of "eli" (e with acute accent)
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			r := bytes.NewReader(tc.input)
			result, err := decodeName(r)
			require.NoError(t, err)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestDecodeResourceType(t *testing.T) {
	// 0x7f = rep type i32, 0x00 = no destructor
	data := []byte{0x7f, 0x00}
	r := bytes.NewReader(data)

	typeDef, err := decodeResourceTypeDef(r)
	require.NoError(t, err)
	require.Nil(t, typeDef.Destructor)
}

func TestDecodeResourceType_WithDestructor(t *testing.T) {
	// 0x7f = rep type i32, 0x01 = has destructor, 0x05 = destructor at func index 5
	data := []byte{0x7f, 0x01, 0x05}
	r := bytes.NewReader(data)

	typeDef, err := decodeResourceTypeDef(r)
	require.NoError(t, err)
	require.NotNil(t, typeDef.Destructor)
	require.Equal(t, uint32(5), *typeDef.Destructor)
}

func TestDecodeResourceType_WithLargeDestructorIndex(t *testing.T) {
	// 0x7f = rep type i32, 0x01 = has destructor, 0x80 0x01 = destructor at func index 128
	data := []byte{0x7f, 0x01, 0x80, 0x01}
	r := bytes.NewReader(data)

	typeDef, err := decodeResourceTypeDef(r)
	require.NoError(t, err)
	require.NotNil(t, typeDef.Destructor)
	require.Equal(t, uint32(128), *typeDef.Destructor)
}

func TestDecodeResourceType_InvalidRepType(t *testing.T) {
	// 0x7e = s8 (not i32), should fail
	data := []byte{0x7e, 0x00}
	r := bytes.NewReader(data)

	_, err := decodeResourceTypeDef(r)
	require.Error(t, err)
}

func TestDecodeResourceType_InvalidDestructorFlag(t *testing.T) {
	// 0x7f = rep type i32, 0x02 = invalid flag (not 0x00 or 0x01)
	data := []byte{0x7f, 0x02}
	r := bytes.NewReader(data)

	_, err := decodeResourceTypeDef(r)
	require.Error(t, err)
}

func TestDecodeOwnType(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected component.ValTypeRef
	}{
		{
			name:  "own<resource_type_3>",
			input: []byte{0x69, 0x03}, // own<T> with type index 3
			expected: component.ValTypeRef{
				IsPrimitive: false,
				IsOwn:       true,
				TypeIdx:     3,
			},
		},
		{
			name:  "own<resource_type_0>",
			input: []byte{0x69, 0x00}, // own<T> with type index 0
			expected: component.ValTypeRef{
				IsPrimitive: false,
				IsOwn:       true,
				TypeIdx:     0,
			},
		},
		{
			name:  "own<resource_type_128>",
			input: []byte{0x69, 0x80, 0x01}, // own<T> with type index 128 (multi-byte LEB128)
			expected: component.ValTypeRef{
				IsPrimitive: false,
				IsOwn:       true,
				TypeIdx:     128,
			},
		},
		{
			name:  "own<resource_type_255>",
			input: []byte{0x69, 0xff, 0x01}, // own<T> with type index 255 (multi-byte LEB128)
			expected: component.ValTypeRef{
				IsPrimitive: false,
				IsOwn:       true,
				TypeIdx:     255,
			},
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			r := bytes.NewReader(tc.input)
			result, err := decodeValType(r)
			require.NoError(t, err)
			require.Equal(t, tc.expected.IsPrimitive, result.IsPrimitive, "IsPrimitive mismatch")
			require.Equal(t, tc.expected.IsOwn, result.IsOwn, "IsOwn mismatch")
			require.Equal(t, tc.expected.TypeIdx, result.TypeIdx, "TypeIdx mismatch")
		})
	}
}

func TestDecodeOwnType_Error(t *testing.T) {
	// Missing type index after 0x69 opcode
	data := []byte{0x69}
	r := bytes.NewReader(data)

	_, err := decodeValType(r)
	require.Error(t, err)
}

func TestDecodeBorrowType(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected component.ValTypeRef
	}{
		{
			name:  "borrow<resource_type_7>",
			input: []byte{0x68, 0x07}, // borrow<T> with type index 7
			expected: component.ValTypeRef{
				IsPrimitive: false,
				IsBorrow:    true,
				TypeIdx:     7,
			},
		},
		{
			name:  "borrow<resource_type_0>",
			input: []byte{0x68, 0x00}, // borrow<T> with type index 0
			expected: component.ValTypeRef{
				IsPrimitive: false,
				IsBorrow:    true,
				TypeIdx:     0,
			},
		},
		{
			name:  "borrow<resource_type_128>",
			input: []byte{0x68, 0x80, 0x01}, // borrow<T> with type index 128 (multi-byte LEB128)
			expected: component.ValTypeRef{
				IsPrimitive: false,
				IsBorrow:    true,
				TypeIdx:     128,
			},
		},
		{
			name:  "borrow<resource_type_255>",
			input: []byte{0x68, 0xff, 0x01}, // borrow<T> with type index 255 (multi-byte LEB128)
			expected: component.ValTypeRef{
				IsPrimitive: false,
				IsBorrow:    true,
				TypeIdx:     255,
			},
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			r := bytes.NewReader(tc.input)
			result, err := decodeValType(r)
			require.NoError(t, err)
			require.Equal(t, tc.expected.IsPrimitive, result.IsPrimitive, "IsPrimitive mismatch")
			require.Equal(t, tc.expected.IsBorrow, result.IsBorrow, "IsBorrow mismatch")
			require.Equal(t, tc.expected.TypeIdx, result.TypeIdx, "TypeIdx mismatch")
		})
	}
}

func TestDecodeBorrowType_Error(t *testing.T) {
	// Missing type index after 0x68 opcode
	data := []byte{0x68}
	r := bytes.NewReader(data)

	_, err := decodeValType(r)
	require.Error(t, err)
}
