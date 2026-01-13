// internal/component/composite_test.go

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

func TestEchoRecord(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	c, err := binary.DecodeComponent(testdata.EchoRecordComponent)
	require.NoError(t, err)

	inst, err := component.Instantiate(ctx, rt, c)
	require.NoError(t, err)

	echo := inst.ExportedFunction("echo")
	require.NotNil(t, echo)

	// Call with record { x: 10, y: 20 }
	input := component.ValRecord(map[string]component.Val{
		"x": component.ValS32(10),
		"y": component.ValS32(20),
	})

	results, err := echo.Call(ctx, input)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))

	// Expect { x: 20, y: 40 } (values doubled)
	rec := results[0].Record()
	require.Equal(t, int32(20), rec["x"].S32())
	require.Equal(t, int32(40), rec["y"].S32())
}

func TestEchoRecord_EdgeCases(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	c, err := binary.DecodeComponent(testdata.EchoRecordComponent)
	require.NoError(t, err)

	inst, err := component.Instantiate(ctx, rt, c)
	require.NoError(t, err)

	echo := inst.ExportedFunction("echo")
	require.NotNil(t, echo)

	tests := []struct {
		name     string
		inputX   int32
		inputY   int32
		expectX  int32
		expectY  int32
	}{
		{
			name:    "zero values",
			inputX:  0,
			inputY:  0,
			expectX: 0,
			expectY: 0,
		},
		{
			name:    "negative values",
			inputX:  -10,
			inputY:  -20,
			expectX: -20,
			expectY: -40,
		},
		{
			name:    "mixed values",
			inputX:  100,
			inputY:  -50,
			expectX: 200,
			expectY: -100,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := component.ValRecord(map[string]component.Val{
				"x": component.ValS32(tc.inputX),
				"y": component.ValS32(tc.inputY),
			})

			results, err := echo.Call(ctx, input)
			require.NoError(t, err)
			require.Equal(t, 1, len(results))

			rec := results[0].Record()
			require.Equal(t, tc.expectX, rec["x"].S32())
			require.Equal(t, tc.expectY, rec["y"].S32())
		})
	}
}
