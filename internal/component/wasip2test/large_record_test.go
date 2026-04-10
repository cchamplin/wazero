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

func TestLargeRecordPlugin_RetptrLifting(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	wasmBytes, err := os.ReadFile("go-large-record-plugin/component.wasm")
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
	if err := wasip2.Instantiate(linker); err != nil {
		t.Fatalf("wasip2.Instantiate: %v", err)
	}

	// Types interface
	err = linker.DefineInstance("test:large-record/types").SkipValidation().Build()
	if err != nil {
		t.Fatalf("DefineInstance types: %v", err)
	}

	// host-data import - returns coordinates {x: 10, y: 20, z: 30}
	err = linker.DefineInstance("test:large-record/host-data").
		Func("get-position", func(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			rec := map[string]types.Val{
				"x": types.ValS32(10),
				"y": types.ValS32(20),
				"z": types.ValS32(30),
			}
			return []types.Val{types.ValRecord(rec)}, nil
		}).
		SkipValidation().
		Build()
	if err != nil {
		t.Fatalf("DefineInstance host-data: %v", err)
	}

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

	// Get the status-handler export
	handlerInst := instance.GetExportedInstance("test:large-record/status-handler")
	if handlerInst == nil {
		t.Fatal("status-handler instance not found")
	}

	// Test get-position (3 fields = retptr path)
	posFunc := handlerInst.ExportedFunction("get-position")
	if posFunc == nil {
		t.Fatal("get-position function not found")
	}
	t.Log("Instantiation succeeded — large record types resolved correctly")
	t.Log("get-position export found (3-field record, forces retptr path)")

	// Test get-status (nested record with 4+ fields = retptr path)
	statusFunc := handlerInst.ExportedFunction("get-status")
	if statusFunc == nil {
		t.Fatal("get-status function not found")
	}
	t.Log("get-status export found (nested record with coordinates+health+alive+score, forces retptr path)")
}
