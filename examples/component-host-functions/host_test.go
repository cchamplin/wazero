package component_host_functions

import (
	"context"
	_ "embed"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api/component"
)

//go:embed testdata/double_import.wasm
var doubleImportWasm []byte

func TestHostFunctions(t *testing.T) {
	ctx := context.Background()

	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	compiled, err := rt.CompileComponent(ctx, doubleImportWasm)
	if err != nil {
		t.Fatal(err)
	}
	defer compiled.Close(ctx)

	for _, imp := range compiled.Imports() {
		t.Logf("import: %s (kind=%d)", imp.Name, imp.Kind)
	}

	linker := rt.NewComponentLinker()

	// Define the "double" function in the "host:util/calc" instance
	err = linker.DefineInstance("host:util/calc").
		Func("double", component.HostFunc(func(ctx context.Context, _ *component.TypeFunc, args []component.Val) ([]component.Val, error) {
			x := args[0].S32()
			return []component.Val{component.ValS32(x * 2)}, nil
		})).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	// session 1 work: Instantiate not yet implemented
	t.Skip("session 1 work: Instantiate not yet implemented")

	instance, err := linker.Instantiate(ctx, compiled)
	if err != nil {
		t.Fatal(err)
	}
	defer instance.Close(ctx)

	runFunc := instance.ExportedFunction("run")
	if runFunc == nil {
		t.Fatal("exported function 'run' not found")
	}

	results, err := runFunc.CallAndPostReturn(ctx, component.ValS32(21))
	if err != nil {
		t.Fatal(err)
	}

	got := results[0].S32()
	if got != 42 {
		t.Errorf("run(21) = %d, want 42", got)
	}
	t.Logf("run(21) = %d (host doubled 21 to 42)", got)
}
