// internal/component/binary/alias_test.go

package binary

import (
	"bytes"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestDecodeAlias_ExportAlias(t *testing.T) {
	// sort=func(0x01), target=export(0x00), instance=0, name="test"
	data := []byte{
		0x01,                     // sort: func
		0x00,                     // target: export
		0x00,                     // instance index
		0x04, 't', 'e', 's', 't', // name: "test"
	}

	alias, err := decodeAlias(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, component.AliasKindExport, alias.Kind)
	require.Equal(t, component.SortFunc, alias.Sort)
	require.Equal(t, uint32(0), alias.InstanceIdx)
	require.Equal(t, "test", alias.ExportName)
}

func TestDecodeAlias_CoreExportAlias(t *testing.T) {
	// sort=core(0x00)+memory(0x02), target=core-export(0x01), instance=1, name="mem"
	data := []byte{
		0x00,                // sort: core
		0x02,                // core sort: memory
		0x01,                // target: core export
		0x01,                // instance index
		0x03, 'm', 'e', 'm', // name: "mem"
	}

	alias, err := decodeAlias(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, component.AliasKindCoreExport, alias.Kind)
	require.Equal(t, component.CoreSortMemory, alias.CoreSort)
	require.Equal(t, uint32(1), alias.InstanceIdx)
	require.Equal(t, "mem", alias.ExportName)
}

func TestDecodeAlias_OuterAlias(t *testing.T) {
	// sort=type(0x03), target=outer(0x02), count=1, index=5
	data := []byte{
		0x03, // sort: type
		0x02, // target: outer
		0x01, // outer count
		0x05, // outer index
	}

	alias, err := decodeAlias(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, component.AliasKindOuter, alias.Kind)
	require.Equal(t, component.SortType, alias.Sort)
	require.Equal(t, uint32(1), alias.OuterCount)
	require.Equal(t, uint32(5), alias.OuterIndex)
}

func TestDecodeAliasSection(t *testing.T) {
	// Section with 2 aliases
	data := []byte{
		0x02, // count: 2
		// Alias 1: export func from instance 0
		0x01, 0x00, 0x00,
		0x04, 't', 'e', 's', 't',
		// Alias 2: outer type from depth 1, index 2
		0x03, 0x02, 0x01, 0x02,
	}

	c := &component.Component{}
	err := decodeAliasSection(c, bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, 2, len(c.Aliases))
	require.Equal(t, component.AliasKindExport, c.Aliases[0].Kind)
	require.Equal(t, component.AliasKindOuter, c.Aliases[1].Kind)
}
