// internal/component/binary/valtype_test.go

package binary

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestPrimValTypeOpcodes(t *testing.T) {
	tests := []struct {
		name     string
		opcode   byte
		expected PrimValType
	}{
		{"bool", 0x7f, PrimValTypeBool},
		{"s8", 0x7e, PrimValTypeS8},
		{"u8", 0x7d, PrimValTypeU8},
		{"s16", 0x7c, PrimValTypeS16},
		{"u16", 0x7b, PrimValTypeU16},
		{"s32", 0x7a, PrimValTypeS32},
		{"u32", 0x79, PrimValTypeU32},
		{"s64", 0x78, PrimValTypeS64},
		{"u64", 0x77, PrimValTypeU64},
		{"f32", 0x76, PrimValTypeF32},
		{"f64", 0x75, PrimValTypeF64},
		{"char", 0x74, PrimValTypeChar},
		{"string", 0x73, PrimValTypeString},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.opcode, byte(tc.expected))
		})
	}
}

func TestPrimValTypeName(t *testing.T) {
	require.Equal(t, "bool", PrimValTypeBool.String())
	require.Equal(t, "s32", PrimValTypeS32.String())
	require.Equal(t, "string", PrimValTypeString.String())
	require.Equal(t, "unknown(0x50)", PrimValType(0x50).String())
}

func TestIsPrimValType(t *testing.T) {
	// All primitive valtypes in range 0x73-0x7f should return true
	for b := byte(0x73); b <= 0x7f; b++ {
		require.True(t, IsPrimValType(b), "expected 0x%02x to be a prim valtype", b)
	}

	// Values outside the range should return false
	require.False(t, IsPrimValType(0x72))
	require.False(t, IsPrimValType(0x80))
	require.False(t, IsPrimValType(0x00))
	require.False(t, IsPrimValType(0xff))
}

func TestCompositeTypeOpcodes(t *testing.T) {
	tests := []struct {
		opcode byte
		name   string
	}{
		{0x72, "record"},
		{0x71, "variant"},
		{0x70, "list"},
		{0x6f, "tuple"},
		{0x6e, "flags"},
		{0x6d, "enum"},
		{0x6c, "option"},
		{0x6b, "result"},
	}

	for _, tc := range tests {
		require.True(t, IsCompositeTypeOpcode(tc.opcode),
			"opcode 0x%02x should be composite type %s", tc.opcode, tc.name)
	}
}

func TestNonCompositeTypeOpcodes(t *testing.T) {
	// Primitive opcodes should not be composite
	for b := byte(0x73); b <= 0x7f; b++ {
		require.False(t, IsCompositeTypeOpcode(b),
			"opcode 0x%02x should not be composite type", b)
	}

	// Other opcodes outside the range should not be composite
	require.False(t, IsCompositeTypeOpcode(0x6a))
	require.False(t, IsCompositeTypeOpcode(0x40)) // function type opcode
}
