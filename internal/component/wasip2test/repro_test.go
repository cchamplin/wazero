// internal/component/wasip2test/repro_test.go
//
// Minimal reproduction of the "func N not found in parent" bug.
//
// The bug occurs when a Go WASM component (built via wasip1 adapter) has:
//   - An interface import with at least one function the guest calls
//   - An interface export (not world-level)
//
// During Instantiate, resolveFromParentScope fails because canon-lifted
// functions are not registered in the parent's componentFuncs map.
package wasip2test

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasip2"
	"github.com/tetratelabs/wazero/internal/component"
)

func TestRepro_InterfaceExportWithHostImport(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	// Load the pre-built component
	wasmBytes, err := os.ReadFile("go-repro-plugin/component.wasm")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// Compile the component
	compiled, err := rt.CompileComponent(ctx, wasmBytes)
	if err != nil {
		t.Fatalf("CompileComponent: %v", err)
	}
	defer compiled.Close(ctx)

	// Create component linker with runtime integration
	linker := component.NewComponentLinker(rt)
	linker.SetRelaxedSemverMatching(true)

	// Register WASI P2 interfaces (needed for wasip1 adapter)
	wasiLinker := component.NewLinker()
	if err := wasip2.Instantiate(wasiLinker); err != nil {
		t.Fatalf("wasip2.Instantiate: %v", err)
	}
	linker.MergeFrom(wasiLinker)

	// Register the host-ops import with SkipValidation
	hostLinker := component.NewLinker()
	err = hostLinker.DefineInstance("test:repro/host-ops").
		FuncNoType("get-value", func(ctx context.Context, args []component.Val) ([]component.Val, error) {
			return []component.Val{component.ValU32(42)}, nil
		}).
		SkipValidation().
		Build()
	if err != nil {
		t.Fatalf("DefineInstance host-ops: %v", err)
	}
	linker.MergeFrom(hostLinker)

	// Set up WASI context
	var stdout, stderr bytes.Buffer
	wasiConfig := wasip2.NewConfig().
		WithStdout(&stdout).
		WithStderr(&stderr).
		WithArgs([]string{"test"}).
		WithEnviron([]string{})
	resourceTable := component.NewResourceTable()
	testCtx := wasip2.WithConfig(ctx, wasiConfig)
	testCtx = component.WithResourceTable(testCtx, resourceTable)

	// THIS IS WHERE THE BUG MANIFESTS:
	// Instantiate fails with "func N not found in parent" because
	// canon-lifted functions are not added to the parent Instance's
	// componentFuncs map during component instantiation.
	instance, err := linker.Instantiate(testCtx, compiled.(*component.CompiledComponent))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}

	// If we get past instantiation, verify the export works
	handlerInst := instance.GetExportedInstance("test:repro/handler")
	if handlerInst == nil {
		t.Fatal("handler instance not found in component exports")
	}
	processFunc := handlerInst.ExportedFunction("process")
	if processFunc == nil {
		t.Fatal("process function not found")
	}

	results, err := processFunc.Call(testCtx)
	if err != nil {
		t.Fatalf("process() call failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// The guest calls host-ops.get-value() which returns 42
	got := results[0].U32()
	if got != 42 {
		t.Errorf("process() = %d, want 42", got)
	}
	t.Logf("process() = %d", got)
}
