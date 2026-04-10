// internal/component/value_import_test.go

package component

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestValueImport asserts that ComponentLinker.DefineValue satisfies a
// component's ImportExternDescValue import and that the resolved value
// lands in the instance's value index space accessible via GetValue.
//
// Spec: definitions.py:256-273 (ComponentInstance) does not itself model
// the value import; the wiring is declared in Explainer.md as `(import
// ... (value ...))` and implemented by the host linker.
// Wasmtime parallel: wasmtime does not expose a public "value import"
// host API in the same shape; wazero's DefineValue / GetValue pair is a
// wazero-specific embedder affordance sitting above the canonical ABI.
// No counterpart (justified): canonical-abi run_tests.py exercises the
// lift/lower value paths only; the embedder-facing value-import wiring
// is a wazero/wasmtime host-layer concern outside run_tests.py scope.
func TestValueImport(t *testing.T) {
	// Component that imports a single value named "config/name".
	c := &Component{
		Imports: []Import{
			{
				Name: "config/name",
				ExternDesc: ImportExternDesc{
					Kind: ImportExternDescValue,
				},
			},
		},
	}

	compiled := &CompiledComponent{
		component: c,
	}

	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := NewComponentLinker(rt)

	// Define the value that satisfies the import. The key stored by
	// DefineValue is "config/name" (namespace+"/"+name), matching the
	// import name directly.
	require.NoError(t, linker.DefineValue("config", "name", types.ValString("TestApp")))

	ctx := context.Background()
	inst, err := linker.Instantiate(ctx, compiled)
	require.NoError(t, err)

	// populateValueImports (component_linker.go:755-768) appends each
	// resolved value-import's Val to inst.values in import-declaration
	// order. The single import above lands at index 0.
	val, err := inst.GetValue(0)
	require.NoError(t, err)
	require.Equal(t, "TestApp", val.StringVal())
}
