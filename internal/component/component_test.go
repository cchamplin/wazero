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
