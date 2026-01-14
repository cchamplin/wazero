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
