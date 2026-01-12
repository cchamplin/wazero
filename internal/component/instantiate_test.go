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

func TestInstantiate_AddS32_EdgeCases(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	c, err := binary.DecodeComponent(testdata.AddS32Component)
	require.NoError(t, err)

	inst, err := component.Instantiate(ctx, rt, c)
	require.NoError(t, err)

	add := inst.ExportedFunction("add")
	require.NotNil(t, add)

	tests := []struct {
		name     string
		a, b     int32
		expected int32
	}{
		{"zero plus zero", 0, 0, 0},
		{"positive plus positive", 2, 3, 5},
		{"negative plus negative", -2, -3, -5},
		{"positive plus negative", 5, -3, 2},
		{"max int32", 2147483646, 1, 2147483647},
		{"min int32", -2147483647, -1, -2147483648},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			results, err := add.Call(ctx, component.ValS32(tc.a), component.ValS32(tc.b))
			require.NoError(t, err)
			require.Equal(t, 1, len(results))
			require.Equal(t, tc.expected, results[0].S32())
		})
	}
}
