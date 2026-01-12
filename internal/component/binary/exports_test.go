// internal/component/binary/exports_test.go

package binary

import (
	"bytes"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestDecodeExport(t *testing.T) {
	// Export format: exportname' sortidx [externdesc?]
	// exportname': 0x00 len name (simple, no version suffix)
	// sortidx: sort u32

	// Export function index 0 with name "add"
	input := []byte{
		0x00,             // simple name (no version)
		0x03, 'a', 'd', 'd', // name length=3, "add"
		0x01,             // sort = func (0x01)
		0x00,             // index = 0
		// No optional externdesc
	}

	r := bytes.NewReader(input)
	exp, err := decodeExport(r)
	require.NoError(t, err)
	require.Equal(t, "add", exp.Name)
	require.Equal(t, component.ExportKindFunc, exp.Kind)
	require.Equal(t, uint32(0), exp.Idx)
}
