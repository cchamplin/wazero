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

	inst, err := component.Instantiate(ctx, component.NewRuntimeInstantiator(rt), c)
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

	inst, err := component.Instantiate(ctx, component.NewRuntimeInstantiator(rt), c)
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

func TestOptionRoundtrip(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	c, err := binary.DecodeComponent(testdata.OptionRoundtripComponent)
	require.NoError(t, err)

	inst, err := component.Instantiate(ctx, component.NewRuntimeInstantiator(rt), c)
	require.NoError(t, err)

	echo := inst.ExportedFunction("echo")
	require.NotNil(t, echo)

	t.Run("None case", func(t *testing.T) {
		// Call with None (nil payload)
		input := component.ValOption(nil)

		results, err := echo.Call(ctx, input)
		require.NoError(t, err)
		require.Equal(t, 1, len(results))

		// Expect None back
		opt := results[0].Option()
		require.Nil(t, opt, "expected None (nil payload)")
	})

	t.Run("Some case", func(t *testing.T) {
		// Call with Some(42)
		val := component.ValS32(42)
		input := component.ValOption(&val)

		results, err := echo.Call(ctx, input)
		require.NoError(t, err)
		require.Equal(t, 1, len(results))

		// Expect Some(42) back
		opt := results[0].Option()
		require.NotNil(t, opt, "expected Some (non-nil payload)")
		require.Equal(t, int32(42), opt.S32())
	})
}

func TestListSum(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	c, err := binary.DecodeComponent(testdata.ListSumComponent)
	require.NoError(t, err)

	inst, err := component.Instantiate(ctx, component.NewRuntimeInstantiator(rt), c)
	require.NoError(t, err)

	sum := inst.ExportedFunction("sum")
	require.NotNil(t, sum)

	tests := []struct {
		name     string
		input    []int32
		expected int32
	}{
		{
			name:     "empty list",
			input:    []int32{},
			expected: 0,
		},
		{
			name:     "single element",
			input:    []int32{42},
			expected: 42,
		},
		{
			name:     "multiple elements",
			input:    []int32{1, 2, 3, 4, 5},
			expected: 15,
		},
		{
			name:     "negative values",
			input:    []int32{-10, 5, -3, 8},
			expected: 0,
		},
		{
			name:     "large values",
			input:    []int32{1000000, 2000000, 3000000},
			expected: 6000000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Convert input to list of Val
			elements := make([]component.Val, len(tc.input))
			for i, v := range tc.input {
				elements[i] = component.ValS32(v)
			}
			input := component.ValList(elements)

			results, err := sum.Call(ctx, input)
			require.NoError(t, err)
			require.Equal(t, 1, len(results))
			require.Equal(t, tc.expected, results[0].S32())
		})
	}
}

func TestResultDivide(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	c, err := binary.DecodeComponent(testdata.ResultDivideComponent)
	require.NoError(t, err)

	inst, err := component.Instantiate(ctx, component.NewRuntimeInstantiator(rt), c)
	require.NoError(t, err)

	divide := inst.ExportedFunction("divide")
	require.NotNil(t, divide)

	t.Run("Ok case", func(t *testing.T) {
		// divide(10, 2) should return Ok(5)
		results, err := divide.Call(ctx, component.ValS32(10), component.ValS32(2))
		require.NoError(t, err)
		require.Equal(t, 1, len(results))

		isOk, okVal, errVal := results[0].Result()
		require.True(t, isOk, "expected Ok result")
		require.NotNil(t, okVal)
		require.Nil(t, errVal)
		require.Equal(t, int32(5), okVal.S32())
	})

	t.Run("Error case", func(t *testing.T) {
		// divide(10, 0) should return Error(1) (division by zero)
		results, err := divide.Call(ctx, component.ValS32(10), component.ValS32(0))
		require.NoError(t, err)
		require.Equal(t, 1, len(results))

		isOk, okVal, errVal := results[0].Result()
		require.False(t, isOk, "expected Error result")
		require.Nil(t, okVal)
		require.NotNil(t, errVal)
		require.Equal(t, int32(1), errVal.S32())
	})

	t.Run("Additional cases", func(t *testing.T) {
		tests := []struct {
			name     string
			a        int32
			b        int32
			expectOk bool
			expected int32 // expected ok value or error code
		}{
			{"positive division", 100, 4, true, 25},
			{"negative dividend", -10, 2, true, -5},
			{"negative divisor", 10, -2, true, -5},
			{"both negative", -10, -2, true, 5},
			{"zero dividend", 0, 5, true, 0},
			{"division by zero", 1, 0, false, 1},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				results, err := divide.Call(ctx, component.ValS32(tc.a), component.ValS32(tc.b))
				require.NoError(t, err)
				require.Equal(t, 1, len(results))

				isOk, okVal, errVal := results[0].Result()
				require.Equal(t, tc.expectOk, isOk)

				if tc.expectOk {
					require.NotNil(t, okVal)
					require.Nil(t, errVal)
					require.Equal(t, tc.expected, okVal.S32())
				} else {
					require.Nil(t, okVal)
					require.NotNil(t, errVal)
					require.Equal(t, tc.expected, errVal.S32())
				}
			})
		}
	})
}
