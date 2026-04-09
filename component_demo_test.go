// component_demo_test.go
package wazero

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/testdata"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestInstantiateAndCallLiftedFunc asserts a component that exports a
// single function `add(s32, s32) -> s32` implemented in core wasm can
// be instantiated and called through the lifted entry point.
//
// Spec: definitions.py:1978-2040 canon_lift full flow.
// Canonical test: run_tests.py test_pairs (primitive round-trips).
// Wasmtime parallel: runtime/component/func.rs Func::call (232-706).
//
// This is the Task C8-c end-to-end integration: real component binary
// (testdata.AddS32Component) -> CompileComponent -> ComponentLinker.
// Instantiate -> ExportedFunction("add").Call(7, 35) == 42.
func TestInstantiateAndCallLiftedFunc(t *testing.T) {
	ctx := context.Background()

	rt := NewRuntime(ctx)
	defer rt.Close(ctx)

	compiled, err := rt.CompileComponent(ctx, testdata.AddS32Component)
	require.NoError(t, err)
	defer compiled.Close(ctx)

	cc, ok := compiled.(*component.CompiledComponent)
	require.True(t, ok, "expected *component.CompiledComponent")

	linker := component.NewComponentLinker(rt)

	instance, err := linker.Instantiate(ctx, cc)
	require.NoError(t, err)
	require.NotNil(t, instance)

	addFunc := instance.ExportedFunction("add")
	require.NotNil(t, addFunc, "expected exported function \"add\"")

	results, err := addFunc.Call(ctx, types.ValS32(7), types.ValS32(35))
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, int32(42), results[0].S32())
}
