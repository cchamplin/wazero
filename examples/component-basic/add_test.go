package component_basic

import (
	"context"
	_ "embed"
	"testing"

	"github.com/tetratelabs/wazero"
)

// addWasm is a component that exports: add(a: s32, b: s32) -> s32
//
//go:embed testdata/add_s32.wasm
var addWasm []byte

// TestComponentBasic demonstrates the simplest component model usage:
// compile a component, instantiate it, and call an exported function.
func TestComponentBasic(t *testing.T) {
	ctx := context.Background()

	// Create a wazero runtime
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	// Compile the component binary
	compiled, err := rt.CompileComponent(ctx, addWasm)
	if err != nil {
		t.Fatal(err)
	}
	defer compiled.Close(ctx)

	// Inspect exports
	for _, exp := range compiled.Exports() {
		t.Logf("export: %s (kind=%d)", exp.Name, exp.Kind)
	}

	// Instantiate with no imports (convenience method)
	instance, err := rt.InstantiateComponent(ctx, compiled)
	if err != nil {
		t.Fatal(err)
	}
	defer instance.Close(ctx)

	// Get the exported "add" function
	addFunc := instance.ExportedFunction("add")
	if addFunc == nil {
		t.Fatal("exported function 'add' not found")
	}

	// Call add(2, 3) - pass Go primitives directly
	results, err := addFunc.Call(ctx, int32(2), int32(3))
	if err != nil {
		t.Fatal(err)
	}

	// Results are returned as Go native types
	got := results[0].(int32)
	if got != 5 {
		t.Errorf("add(2, 3) = %d, want 5", got)
	}
	t.Logf("add(2, 3) = %d", got)
}
