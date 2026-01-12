// internal/component/instantiate_test.go

package component_test

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/binary"
	"github.com/tetratelabs/wazero/internal/component/testdata"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestInstantiate_AddS32(t *testing.T) {
	ctx := context.Background()

	// Create wazero runtime
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	// Parse the component
	c, err := binary.DecodeComponent(testdata.AddS32Component)
	require.NoError(t, err)

	// Instantiate the component
	inst, err := component.Instantiate(ctx, rt, c)
	require.NoError(t, err)
	require.NotNil(t, inst)

	// Get the "add" export
	add := inst.ExportedFunction("add")
	require.NotNil(t, add, "expected 'add' export")

	// Call add(2, 3) and expect 5
	results, err := add.Call(ctx, component.ValS32(2), component.ValS32(3))
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, int32(5), results[0].S32())
}
