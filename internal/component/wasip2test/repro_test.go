// internal/component/wasip2test/repro_test.go
//
// Reproduction tests for component model instantiation bugs.
//
// These tests use a Go WASM component (built via wasip1 adapter) that has:
//   - A shared types interface with a record type
//   - An interface import with a function the guest calls
//   - An interface export (not world-level) returning the record type
//
// Bug 1 (func not found): canon-lifted functions are not registered in the
// parent Instance's componentFuncs map during component instantiation.
//
// Bug 2 (type not found): component-defined types (records, enums, etc.) used
// across interfaces are not tracked in the parent Instance's type space, so
// resolveFromParentScope fails when a nested core module references them.
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

// newReproInstance creates a wazero runtime, registers WASI + mock host-ops,
// compiles and instantiates the go-repro-plugin component.
func newReproInstance(t *testing.T) (*component.Instance, context.Context, func()) {
	t.Helper()

	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)

	wasmBytes, err := os.ReadFile("go-repro-plugin/component.wasm")
	if err != nil {
		rt.Close(ctx)
		t.Fatalf("ReadFile: %v", err)
	}

	compiled, err := rt.CompileComponent(ctx, wasmBytes)
	if err != nil {
		rt.Close(ctx)
		t.Fatalf("CompileComponent: %v", err)
	}

	linker := component.NewComponentLinker(rt)
	linker.SetRelaxedSemverMatching(true)

	// Register WASI P2 interfaces (needed for wasip1 adapter)
	wasiLinker := component.NewLinker()
	if err := wasip2.Instantiate(wasiLinker); err != nil {
		compiled.Close(ctx)
		rt.Close(ctx)
		t.Fatalf("wasip2.Instantiate: %v", err)
	}
	linker.MergeFrom(wasiLinker)

	// Register the types interface (no functions, just type definitions)
	hostLinker := component.NewLinker()
	err = hostLinker.DefineInstance("test:repro/types").
		SkipValidation().
		Build()
	if err != nil {
		compiled.Close(ctx)
		rt.Close(ctx)
		t.Fatalf("DefineInstance types: %v", err)
	}

	// Register the host-ops import
	err = hostLinker.DefineInstance("test:repro/host-ops").
		FuncNoType("get-value", func(ctx context.Context, args []component.Val) ([]component.Val, error) {
			return []component.Val{component.ValU32(42)}, nil
		}).
		SkipValidation().
		Build()
	if err != nil {
		compiled.Close(ctx)
		rt.Close(ctx)
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

	instance, err := linker.Instantiate(testCtx, compiled.(*component.CompiledComponent))
	if err != nil {
		compiled.Close(ctx)
		rt.Close(ctx)
		t.Fatalf("Instantiate: %v", err)
	}

	cleanup := func() {
		compiled.Close(ctx)
		rt.Close(ctx)
	}

	return instance, testCtx, cleanup
}

// TestRepro_InterfaceExportWithHostImport reproduces the "func N not found in
// parent" bug. Canon-lifted functions (exports) must be registered in the
// parent Instance's componentFuncs map so nested core modules can reference
// them via instantiation args.
func TestRepro_InterfaceExportWithHostImport(t *testing.T) {
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()

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

	// process() returns a process-result record {value: u32, ok: bool}.
	// The guest calls host-ops.get-value() (returns 42) and wraps it.
	rec := results[0].Record()
	if rec == nil {
		t.Fatalf("expected record, got kind=%v", results[0].Kind())
	}
	if got := rec["value"].U32(); got != 42 {
		t.Errorf("process().value = %d, want 42", got)
	}
	if got := rec["ok"].Bool(); !got {
		t.Errorf("process().ok = %v, want true", got)
	}
	t.Logf("process() = {value: %d, ok: %v}", rec["value"].U32(), rec["ok"].Bool())
}

// TestRepro_RecordTypeResolution reproduces the "type N not found in parent"
// bug. When a component defines record types in a shared types interface and
// uses them across import/export interfaces, those types must be tracked in
// the parent Instance's type space so resolveFromParentScope can find them
// when nested core modules reference them via instantiation args.
func TestRepro_RecordTypeResolution(t *testing.T) {
	// This test exercises the same instantiation path but focuses on the
	// type resolution aspect. If instantiation succeeds, the type was found.
	// The test deliberately calls the export to verify the record type is
	// correctly lifted back from the core module's linear memory.
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()

	handlerInst := instance.GetExportedInstance("test:repro/handler")
	if handlerInst == nil {
		t.Fatal("handler instance not found")
	}
	processFunc := handlerInst.ExportedFunction("process")
	if processFunc == nil {
		t.Fatal("process function not found")
	}

	results, err := processFunc.Call(testCtx)
	if err != nil {
		t.Fatalf("process() call failed: %v", err)
	}

	// Verify the record structure is correctly decoded
	rec := results[0].Record()
	if rec == nil {
		t.Fatalf("expected record type, got kind=%v", results[0].Kind())
	}

	// Verify both fields exist and have correct types
	valField, hasVal := rec["value"]
	okField, hasOk := rec["ok"]
	if !hasVal {
		t.Fatal("record missing 'value' field")
	}
	if !hasOk {
		t.Fatal("record missing 'ok' field")
	}

	t.Logf("Record type resolved correctly: {value: %d (kind=%v), ok: %v (kind=%v)}",
		valField.U32(), valField.Kind(), okField.Bool(), okField.Kind())
}
