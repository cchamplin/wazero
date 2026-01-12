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
