// internal/component/instance_test.go

package component

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestInstanceStructure(t *testing.T) {
	c := &Component{}
	inst := &Instance{
		component: c,
	}

	require.Same(t, c, inst.Component())
	require.Nil(t, inst.ExportedFunction("nonexistent"))
}
