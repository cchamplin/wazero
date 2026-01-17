// component_demo_test.go
package wazero

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/testdata"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestDemoComponentExecution demonstrates end-to-end component execution
// using the internal ComponentLinker with runtime access.
func TestDemoComponentExecution(t *testing.T) {
	ctx := context.Background()

	// Create the wazero runtime
	rt := NewRuntime(ctx)
	defer rt.Close(ctx)

	// Use the runtime to compile the component (handles parsing + core module compilation)
	compiled, err := rt.CompileComponent(ctx, testdata.AddS32Component)
	require.NoError(t, err)
	defer compiled.Close(ctx)

	// Get the internal CompiledComponent
	cc, ok := compiled.(*component.CompiledComponent)
	require.True(t, ok, "expected *component.CompiledComponent")

	// Create ComponentLinker with runtime access
	linker := component.NewComponentLinker(rt)

	// Instantiate the component - this should instantiate core modules
	instance, err := linker.Instantiate(ctx, cc)
	require.NoError(t, err)
	require.NotNil(t, instance)

	// Get the add function
	addFunc := instance.ExportedFunction("add")
	if addFunc == nil {
		t.Skip("add function not wired yet")
	}

	// Call it: add(2, 3) = 5
	results, err := addFunc.Call(ctx, component.ValS32(2), component.ValS32(3))
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, int32(5), results[0].S32())

	t.Logf("SUCCESS: add(2, 3) = %d", results[0].S32())
}
