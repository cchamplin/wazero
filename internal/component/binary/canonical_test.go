// internal/component/binary/canonical_test.go

package binary

import (
	"bytes"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestDecodeCanonicalLift(t *testing.T) {
	// canon lift: 0x00 0x00 core:funcidx opts typeidx
	// Minimal: lift core function 0 as component function type 0
	input := []byte{
		0x00, // canon.lift
		0x00, // core sort (always 0x00 for funcs)
		0x00, // core:funcidx = 0
		0x00, // opts count = 0
		0x00, // typeidx = 0
	}

	r := bytes.NewReader(input)
	def, err := decodeCanonical(r)
	require.NoError(t, err)
	require.Equal(t, component.CanonKindLift, def.Kind)
	require.Equal(t, uint32(0), def.CoreFuncIdx)
	require.Equal(t, uint32(0), def.TypeIdx)
}

func TestDecodeCanonicalLower(t *testing.T) {
	// canon lower: 0x01 0x00 funcidx opts
	input := []byte{
		0x01, // canon.lower
		0x00, // always 0x00
		0x00, // funcidx = 0
		0x00, // opts count = 0
	}

	r := bytes.NewReader(input)
	def, err := decodeCanonical(r)
	require.NoError(t, err)
	require.Equal(t, component.CanonKindLower, def.Kind)
	require.Equal(t, uint32(0), def.FuncIdx)
}

func TestDecodeCanonicalLiftWithOptions(t *testing.T) {
	// canon lift with memory option
	input := []byte{
		0x00, // canon.lift
		0x00, // core sort
		0x00, // core:funcidx = 0
		0x01, // opts count = 1
		0x03, // memory option
		0x00, // memory index = 0
		0x00, // typeidx = 0
	}

	r := bytes.NewReader(input)
	def, err := decodeCanonical(r)
	require.NoError(t, err)
	require.NotNil(t, def.Options.MemoryIdx)
	require.Equal(t, uint32(0), *def.Options.MemoryIdx)
}
