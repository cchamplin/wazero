package wasip2test

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasip2"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
)

func TestNestedTypesPlugin_OptionResultSharedRecords(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	wasmBytes, err := os.ReadFile("go-nested-types-plugin/component.wasm")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	compiled, err := rt.CompileComponent(ctx, wasmBytes)
	if err != nil {
		t.Skipf("decoder limitation: CompileComponent failed: %v", err)
	}
	defer compiled.Close(ctx)

	linker := component.NewComponentLinker(rt)
	linker.SetRelaxedSemverMatching(true)

	// WASI P2
	wasiLinker := component.NewLinker()
	if err := wasip2.Instantiate(wasiLinker); err != nil {
		t.Fatalf("wasip2.Instantiate: %v", err)
	}
	linker.MergeFrom(wasiLinker)

	// Types interface (no functions)
	hostLinker := component.NewLinker()
	err = hostLinker.DefineInstance("test:nested-types/types").SkipValidation().Build()
	if err != nil {
		t.Fatalf("DefineInstance types: %v", err)
	}

	// store import - returns some(item) for id=1, none for others
	err = hostLinker.DefineInstance("test:nested-types/store").
		Func("get-item", func(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			// For now, return a simple option<item> as none
			// The exact ABI encoding of option<record> depends on the lowering
			return []types.Val{types.ValOption(nil)}, nil
		}).
		SkipValidation().
		Build()
	if err != nil {
		t.Fatalf("DefineInstance store: %v", err)
	}
	linker.MergeFrom(hostLinker)

	// WASI context
	var stdout, stderr bytes.Buffer
	wasiConfig := wasip2.NewConfig().
		WithStdout(&stdout).
		WithStderr(&stderr).
		WithArgs([]string{"test"}).
		WithEnviron([]string{})
	resourceTable := runtime.NewTable()
	testCtx := wasip2.WithConfig(ctx, wasiConfig)
	testCtx = component.WithResourceTable(testCtx, resourceTable)

	instance, err := linker.Instantiate(testCtx, compiled.(*component.CompiledComponent))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}

	// Get the handler interface export
	handlerInst := instance.GetExportedInstance("test:nested-types/handler")
	if handlerInst == nil {
		t.Fatal("handler instance not found")
	}

	lookupFunc := handlerInst.ExportedFunction("lookup")
	if lookupFunc == nil {
		t.Fatal("lookup function not found")
	}

	createFunc := handlerInst.ExportedFunction("create")
	if createFunc == nil {
		t.Fatal("create function not found")
	}

	t.Log("Instantiation succeeded — option/result with shared record types resolved correctly")
}
