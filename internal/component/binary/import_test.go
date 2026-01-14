// internal/component/binary/import_test.go

package binary

import (
	"bytes"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestDecodeImportName_Plain(t *testing.T) {
	// 0x00 prefix = plain name without version suffix
	data := []byte{
		0x00,                           // plain name
		0x08,                           // length
		't', 'e', 's', 't', '-', 'a', 'p', 'i',
	}

	name, err := decodeImportName(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, "test-api", name)
}

func TestDecodeImportName_WithVersion(t *testing.T) {
	// 0x01 prefix = name with version suffix
	data := []byte{
		0x01,                           // with version suffix
		0x0a,                           // length = 10
		't', 'e', 's', 't', '@', '1', '.', '2', '.', '3',
	}

	name, err := decodeImportName(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, "test@1.2.3", name)
}

func TestDecodeExternDesc_Func(t *testing.T) {
	// 0x01 = func, then type index
	data := []byte{0x01, 0x05}

	desc, err := decodeExternDesc(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, component.ImportExternDescFunc, desc.Kind)
	require.Equal(t, uint32(5), desc.TypeIdx)
}

func TestDecodeExternDesc_CoreModule(t *testing.T) {
	// 0x00 0x11 = core module, then core type index
	data := []byte{0x00, 0x11, 0x02}

	desc, err := decodeExternDesc(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, component.ImportExternDescCoreModule, desc.Kind)
	require.Equal(t, uint32(2), desc.CoreTypeIdx)
}

func TestDecodeExternDesc_Instance(t *testing.T) {
	// 0x05 = instance, then type index
	data := []byte{0x05, 0x03}

	desc, err := decodeExternDesc(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, component.ImportExternDescInstance, desc.Kind)
	require.Equal(t, uint32(3), desc.TypeIdx)
}

func TestDecodeExternDesc_Component(t *testing.T) {
	// 0x04 = component, then type index
	data := []byte{0x04, 0x07}

	desc, err := decodeExternDesc(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, component.ImportExternDescComponent, desc.Kind)
	require.Equal(t, uint32(7), desc.TypeIdx)
}

func TestDecodeImportSection(t *testing.T) {
	// Section with 2 imports
	data := []byte{
		0x02,                                // count: 2
		// Import 1: func
		0x00, 0x04, 't', 'e', 's', 't',      // name (plain)
		0x01, 0x00,                          // func type 0
		// Import 2: instance
		0x00, 0x05, 'o', 't', 'h', 'e', 'r', // name (plain)
		0x05, 0x01,                          // instance type 1
	}

	c := &component.Component{}
	err := decodeImportSection(c, bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, 2, len(c.Imports))
	require.Equal(t, "test", c.Imports[0].Name)
	require.Equal(t, component.ImportExternDescFunc, c.Imports[0].ExternDesc.Kind)
	require.Equal(t, "other", c.Imports[1].Name)
	require.Equal(t, component.ImportExternDescInstance, c.Imports[1].ExternDesc.Kind)
}
