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
		0x00,                // simple name (no version)
		0x03, 'a', 'd', 'd', // name length=3, "add"
		0x01,                // sort = func (0x01)
		0x00,                // index = 0
		0x00,                // no extern desc
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

func TestDecodeExportMultiple(t *testing.T) {
	data := buildComponentWithSection(SectionIDExport, []byte{
		0x02,                 // count = 2
		// First export
		0x00,                 // simple name
		0x03, 'f', 'o', 'o', // name "foo"
		0x01,                 // func sort
		0x00,                 // index
		0x00,                 // no extern desc
		// Second export
		0x00,                 // simple name
		0x03, 'b', 'a', 'r', // name "bar"
		0x01,                 // func sort
		0x01,                 // index
		0x00,                 // no extern desc
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(c.Exports) != 2 {
		t.Fatalf("expected 2 exports, got %d", len(c.Exports))
	}

	if c.Exports[0].Name != "foo" {
		t.Errorf("expected first export name 'foo', got '%s'", c.Exports[0].Name)
	}

	if c.Exports[1].Name != "bar" {
		t.Errorf("expected second export name 'bar', got '%s'", c.Exports[1].Name)
	}
}

func TestDecodeExport_CoreSorts(t *testing.T) {
	cases := []struct {
		name     string
		coreSort byte
		expected component.ExportKind
	}{
		{"core func", 0x00, component.ExportKindFunc},
		{"core table", 0x01, component.ExportKindTable},
		{"core memory", 0x02, component.ExportKindMemory},
		{"core global", 0x03, component.ExportKindGlobal},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Construct minimal export bytes with core sort
			// Export format: exportname' sortidx [externdesc?]
			// For core sorts: sort=0x00 followed by core sort byte
			input := []byte{
				0x00,                   // simple name (no version)
				0x04, 't', 'e', 's', 't', // name length=4, "test"
				0x00,       // sort = core (0x00)
				tc.coreSort, // core sort byte
				0x00,       // index = 0
				0x00,       // no extern desc
			}

			r := bytes.NewReader(input)
			exp, err := decodeExport(r)
			require.NoError(t, err)
			require.Equal(t, "test", exp.Name)
			require.Equal(t, tc.expected, exp.Kind, "expected ExportKind %d for core sort 0x%02x, got %d", tc.expected, tc.coreSort, exp.Kind)
			require.Equal(t, uint32(0), exp.Idx)
		})
	}
}
