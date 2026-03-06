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
//
// Bug 3 (signature mismatch): canon lower generates i32 for u64 params instead
// of i64, causing core module instantiation to fail with signature mismatch.
package wasip2test

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/tetratelabs/wazero"
	apicomponent "github.com/tetratelabs/wazero/api/component"
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
		FuncNoType("get-random-len", func(ctx context.Context, args []component.Val) ([]component.Val, error) {
			// Echo back the u64 input
			return []component.Val{component.ValU64(args[0].U64())}, nil
		}).
		SkipValidation().
		Build()
	if err != nil {
		compiled.Close(ctx)
		rt.Close(ctx)
		t.Fatalf("DefineInstance host-ops: %v", err)
	}
	// Register the host-rng import (separate interface, same function name as wasi:random)
	err = hostLinker.DefineInstance("test:repro/host-rng").
		FuncNoType("get-random-bytes", func(ctx context.Context, args []component.Val) ([]component.Val, error) {
			return []component.Val{component.ValList(nil)}, nil
		}).
		SkipValidation().
		Build()
	if err != nil {
		compiled.Close(ctx)
		rt.Close(ctx)
		t.Fatalf("DefineInstance host-rng: %v", err)
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

// TestRepro_U64CanonLowerSignature reproduces the canon lower signature mismatch
// for u64 parameters. When a host function takes u64 args, canon lower must
// produce a core function with i64 params. The bug generates i32 instead,
// causing: "signature mismatch: i64i32_v != i32i32_v" during core module
// instantiation.
func TestRepro_U64CanonLowerSignature(t *testing.T) {
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()

	handlerInst := instance.GetExportedInstance("test:repro/handler")
	if handlerInst == nil {
		t.Fatal("handler instance not found")
	}
	processRandomFunc := handlerInst.ExportedFunction("process-random")
	if processRandomFunc == nil {
		t.Fatal("process-random function not found")
	}

	// Call process-random with a u64 value
	// The guest forwards to host-ops.get-random-len which echoes it back
	input := uint64(123456789)
	results, err := processRandomFunc.Call(testCtx, component.ValU64(input))
	if err != nil {
		t.Fatalf("process-random(%d) call failed: %v", input, err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	got := results[0].U64()
	if got != input {
		t.Errorf("process-random(%d) = %d, want %d", input, got, input)
	}
	t.Logf("process-random(%d) = %d", input, got)
}

// TestRepro_PublicAPISignatureMismatch reproduces the WASI random signature
// mismatch that occurs when using the public API (rt.NewComponentLinker() +
// wasip2.MergeInto + linker.DefineInstance via api.ComponentInstanceBuilder).
//
// The wasip1 adapter's core module expects get-random-bytes with signature
// (i64, i32) -> void but canon lower produces (i32, i32) -> void.
// This matches the director integration test setup.
func TestRepro_PublicAPISignatureMismatch(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	wasmBytes, err := os.ReadFile("go-repro-plugin/component.wasm")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	compiled, err := rt.CompileComponent(ctx, wasmBytes)
	if err != nil {
		t.Fatalf("CompileComponent: %v", err)
	}
	defer compiled.Close(ctx)

	// Use the PUBLIC API path — same as director integration test
	linker := rt.NewComponentLinker()
	linker.SetRelaxedSemverMatching(true)

	// Register WASI P2 via public API (MergeInto, not Instantiate+merge)
	if err := wasip2.MergeInto(linker); err != nil {
		t.Fatalf("wasip2.MergeInto: %v", err)
	}

	// Register host imports via public API DefineInstance
	err = linker.DefineInstance("test:repro/types").SkipValidation().Build()
	if err != nil {
		t.Fatalf("DefineInstance types: %v", err)
	}

	err = linker.DefineInstance("test:repro/host-ops").SkipValidation().
		Func("get-value", apicomponent.HostFunc(func(ctx context.Context, args []apicomponent.Val) ([]apicomponent.Val, error) {
			return []apicomponent.Val{apicomponent.ValU32(42)}, nil
		})).
		Func("get-random-len", apicomponent.HostFunc(func(ctx context.Context, args []apicomponent.Val) ([]apicomponent.Val, error) {
			return []apicomponent.Val{apicomponent.ValU64(args[0].U64())}, nil
		})).
		Build()
	if err != nil {
		t.Fatalf("DefineInstance host-ops: %v", err)
	}

	err = linker.DefineInstance("test:repro/host-rng").SkipValidation().
		Func("get-random-bytes", apicomponent.HostFunc(func(ctx context.Context, args []apicomponent.Val) ([]apicomponent.Val, error) {
			return []apicomponent.Val{apicomponent.ValList(nil)}, nil
		})).
		Build()
	if err != nil {
		t.Fatalf("DefineInstance host-rng: %v", err)
	}

	// Set up WASI context
	var stdout, stderr bytes.Buffer
	wasiConfig := wasip2.NewConfig().
		WithStdout(&stdout).
		WithStderr(&stderr).
		WithArgs([]string{"test"}).
		WithEnviron([]string{})
	resourceTable := apicomponent.NewResourceTable()
	testCtx := wasip2.WithConfig(ctx, wasiConfig)
	testCtx = apicomponent.WithResourceTable(testCtx, resourceTable)

	// THIS IS WHERE THE BUG MANIFESTS when using public API:
	// instantiate core instance N: import func[wasi:random/random@0.2.6.get-random-bytes]:
	// signature mismatch: i64i32_v != i32i32_v
	instance, err := linker.Instantiate(testCtx, compiled)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	defer instance.Close(ctx)

	// If instantiation succeeds, verify exports work
	handlerInst := instance.ExportedInstance("test:repro/handler")
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
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	t.Logf("process() returned: %v", results[0])
}
