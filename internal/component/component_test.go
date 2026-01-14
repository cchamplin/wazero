// internal/component/component_test.go

package component

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
	"github.com/tetratelabs/wazero/internal/wasm"
)

func TestComponentStructure(t *testing.T) {
	c := &Component{}

	// Verify all slice fields are nil by default
	require.Nil(t, c.CoreModules)
	require.Nil(t, c.Types)
	require.Nil(t, c.Canonicals)
	require.Nil(t, c.Exports)
}

func TestComponentWithCoreModule(t *testing.T) {
	m := &wasm.Module{}
	c := &Component{
		CoreModules: []*wasm.Module{m},
	}

	require.Equal(t, 1, len(c.CoreModules))
	require.Same(t, m, c.CoreModules[0])
}

func TestSortString(t *testing.T) {
	tests := []struct {
		sort     Sort
		expected string
	}{
		{SortFunc, "func"},
		{SortValue, "value"},
		{SortType, "type"},
		{SortComponent, "component"},
		{SortInstance, "instance"},
		{SortCoreSort, "core"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.expected, tt.sort.String())
	}
}

func TestCoreSortString(t *testing.T) {
	tests := []struct {
		sort     CoreSort
		expected string
	}{
		{CoreSortFunc, "func"},
		{CoreSortTable, "table"},
		{CoreSortMemory, "memory"},
		{CoreSortGlobal, "global"},
		{CoreSortType, "type"},
		{CoreSortModule, "module"},
		{CoreSortInstance, "instance"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.expected, tt.sort.String())
	}
}

func TestAliasKindString(t *testing.T) {
	tests := []struct {
		kind     AliasKind
		expected string
	}{
		{AliasKindExport, "export"},
		{AliasKindCoreExport, "core-export"},
		{AliasKindOuter, "outer"},
		{AliasKind(255), "unknown(255)"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.expected, tt.kind.String())
	}
}

func TestAlias_ExportAlias(t *testing.T) {
	a := Alias{
		Kind:        AliasKindExport,
		Sort:        SortFunc,
		InstanceIdx: 1,
		ExportName:  "my-func",
	}
	require.Equal(t, AliasKindExport, a.Kind)
	require.Equal(t, SortFunc, a.Sort)
	require.Equal(t, uint32(1), a.InstanceIdx)
	require.Equal(t, "my-func", a.ExportName)
}

func TestAlias_CoreExportAlias(t *testing.T) {
	a := Alias{
		Kind:        AliasKindCoreExport,
		CoreSort:    CoreSortMemory,
		InstanceIdx: 0,
		ExportName:  "memory",
	}
	require.Equal(t, AliasKindCoreExport, a.Kind)
	require.Equal(t, CoreSortMemory, a.CoreSort)
	require.Equal(t, uint32(0), a.InstanceIdx)
	require.Equal(t, "memory", a.ExportName)
}

func TestAlias_OuterAlias(t *testing.T) {
	a := Alias{
		Kind:       AliasKindOuter,
		Sort:       SortType,
		OuterCount: 1,
		OuterIndex: 5,
	}
	require.Equal(t, AliasKindOuter, a.Kind)
	require.Equal(t, SortType, a.Sort)
	require.Equal(t, uint32(1), a.OuterCount)
	require.Equal(t, uint32(5), a.OuterIndex)
}

func TestComponent_Aliases(t *testing.T) {
	c := &Component{
		Aliases: []Alias{
			{Kind: AliasKindExport, Sort: SortFunc, InstanceIdx: 0, ExportName: "test"},
		},
	}
	require.Equal(t, 1, len(c.Aliases))
	require.Equal(t, AliasKindExport, c.Aliases[0].Kind)
}

func TestImportExternDescKind(t *testing.T) {
	tests := []struct {
		kind     ImportExternDescKind
		expected string
	}{
		{ImportExternDescCoreModule, "core-module"},
		{ImportExternDescFunc, "func"},
		{ImportExternDescValue, "value"},
		{ImportExternDescType, "type"},
		{ImportExternDescComponent, "component"},
		{ImportExternDescInstance, "instance"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.expected, tt.kind.String())
	}
}

func TestImport_FuncImport(t *testing.T) {
	imp := Import{
		Name: "wasi:cli/environment@0.2.0",
		ExternDesc: ImportExternDesc{
			Kind:    ImportExternDescFunc,
			TypeIdx: 5,
		},
	}
	require.Equal(t, "wasi:cli/environment@0.2.0", imp.Name)
	require.Equal(t, ImportExternDescFunc, imp.ExternDesc.Kind)
	require.Equal(t, uint32(5), imp.ExternDesc.TypeIdx)
}
