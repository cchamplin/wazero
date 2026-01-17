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

func TestDecodeExportWithExternDesc(t *testing.T) {
	data := buildComponentWithSection(SectionIDExport, []byte{
		0x01,                   // count = 1
		0x00,                   // simple name
		0x04, 't', 'e', 's', 't',
		0x01,                   // func sort
		0x00,                   // index
		0x01,                   // has extern desc
		0x01,                   // func type
		0x05,                   // type index
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.Exports[0].TypeIdx == nil {
		t.Fatal("expected type index")
	}

	if *c.Exports[0].TypeIdx != 5 {
		t.Errorf("expected type index 5, got %d", *c.Exports[0].TypeIdx)
	}
}
