// internal/component/integration_public_api_test.go
//
// Integration tests for the public wazero component API surface:
// Runtime.CompileComponent, Runtime.InstantiateComponent,
// Runtime.NewComponentLinker, Component.ExportedFunction,
// Component.ExportedInstance, and ComponentFunc.Call.
//
// These use the external test package to exercise the exported API.
package component_test

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/component/testdata"
	"github.com/tetratelabs/wazero/internal/component/types"
)

// TestPublicAPICompileComponent verifies that CompileComponent parses
// the add_s32 test binary and returns a CompiledComponent with the
// expected "add" function export.
//
// Spec: Binary.md component binary format.
// Wasmtime parallel: crates/wasmtime/src/runtime/component.rs
// Component::new (compile path).
func TestPublicAPICompileComponent(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	compiled, err := rt.CompileComponent(ctx, testdata.AddS32Component)
	if err != nil {
		t.Fatalf("CompileComponent: %v", err)
	}
	defer compiled.Close(ctx)

	exports := compiled.Exports()
	if len(exports) == 0 {
		t.Fatal("expected at least one export")
	}

	foundAdd := false
	for _, exp := range exports {
		if exp.Name == "add" && exp.Kind == api.ComponentExportKindFunc {
			foundAdd = true
			break
		}
	}
	if !foundAdd {
		t.Error("expected 'add' function in exports")
	}
}

// TestPublicAPIAddS32 exercises the full pipeline: compile add_s32,
// instantiate, get exported "add", call add(2,3) and assert 5.
//
// Spec: definitions.py:1978-2040 (canon_lift) and :2064-2130
// (canon_lower) for the component function call path.
// Wasmtime parallel: tests/all/component_model/func.rs add tests.
func TestPublicAPIAddS32(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	compiled, err := rt.CompileComponent(ctx, testdata.AddS32Component)
	if err != nil {
		t.Fatalf("CompileComponent: %v", err)
	}
	defer compiled.Close(ctx)

	instance, err := rt.InstantiateComponent(ctx, compiled)
	if err != nil {
		t.Fatalf("InstantiateComponent: %v", err)
	}
	defer instance.Close(ctx)

	addFunc := instance.ExportedFunction("add")
	if addFunc == nil {
		t.Fatal("expected 'add' function export")
	}

	results, err := addFunc.Call(ctx, int32(2), int32(3))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	got, ok := results[0].(int32)
	if !ok {
		t.Fatalf("expected int32, got %T", results[0])
	}
	if got != 5 {
		t.Errorf("expected 5, got %d", got)
	}
}

// TestPublicAPICompileComponentError verifies that CompileComponent
// returns an error for invalid (non-component) binary.
//
// Spec: Binary.md component magic bytes (0x00 0x61 0x73 0x6d +
// layer 0x0a 0x00).
func TestPublicAPICompileComponentError(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	_, err := rt.CompileComponent(ctx, []byte{0x00, 0x01, 0x02, 0x03})
	if err == nil {
		t.Error("expected error for invalid binary")
	}
}

// TestPublicAPIComponentLinker verifies the NewComponentLinker builder
// API: DefineFunc (with the 3-arg HostFunc signature under the wasmtime
// func_new model) and DefineInstance with chained Func + Build.
//
// Spec: wasmtime LinkerInstance::func_new (linker.rs:665-675) —
// the host has no type to declare; fnType is supplied at call time.
// Wasmtime parallel: crates/wasmtime/src/runtime/component/linker.rs
// LinkerInstance::func_new, LinkerInstance::instance.
func TestPublicAPIComponentLinker(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	linker := rt.NewComponentLinker()
	if linker == nil {
		t.Fatal("expected non-nil linker")
	}

	// DefineFunc with the 3-arg HostFunc signature (ctx, *TypeFunc, []Val).
	err := linker.DefineFunc("test:api@1.0.0", "greet",
		func(_ context.Context, _ *types.TypeFunc, _ []types.Val) ([]types.Val, error) {
			return []types.Val{types.ValString("Hello!")}, nil
		})
	if err != nil {
		t.Fatalf("DefineFunc: %v", err)
	}

	// DefineInstance with a function export.
	err = linker.DefineInstance("test:env@1.0.0").
		Func("get-value",
			func(_ context.Context, _ *types.TypeFunc, _ []types.Val) ([]types.Val, error) {
				return []types.Val{types.ValS32(42)}, nil
			}).
		Build()
	if err != nil {
		t.Fatalf("DefineInstance: %v", err)
	}
}

// TestPublicAPIComponentLinkerInstantiate verifies instantiation of
// add_s32 through a ComponentLinker (instead of the convenience
// rt.InstantiateComponent wrapper) and exercises the full call path.
//
// Spec: definitions.py:256-273 ComponentInstance; canon_lift/lower.
// Wasmtime parallel: crates/wasmtime/src/runtime/component/linker.rs:274-284.
func TestPublicAPIComponentLinkerInstantiate(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	compiled, err := rt.CompileComponent(ctx, testdata.AddS32Component)
	if err != nil {
		t.Fatalf("CompileComponent: %v", err)
	}
	defer compiled.Close(ctx)

	linker := rt.NewComponentLinker()
	instance, err := linker.Instantiate(ctx, compiled)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	defer instance.Close(ctx)

	addFunc := instance.ExportedFunction("add")
	if addFunc == nil {
		t.Fatal("expected 'add' function export")
	}

	results, err := addFunc.Call(ctx, int32(10), int32(20))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	got, ok := results[0].(int32)
	if !ok {
		t.Fatalf("expected int32, got %T", results[0])
	}
	if got != 30 {
		t.Errorf("expected 30, got %d", got)
	}
}

// TestPublicAPIExportedInstanceNil verifies that ExportedInstance
// returns nil for a non-existent nested instance name.
//
// Spec: Explainer.md instance export lookup.
func TestPublicAPIExportedInstanceNil(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	compiled, err := rt.CompileComponent(ctx, testdata.AddS32Component)
	if err != nil {
		t.Fatalf("CompileComponent: %v", err)
	}
	defer compiled.Close(ctx)

	instance, err := rt.InstantiateComponent(ctx, compiled)
	if err != nil {
		t.Fatalf("InstantiateComponent: %v", err)
	}
	defer instance.Close(ctx)

	nestedInstance := instance.ExportedInstance("nonexistent")
	if nestedInstance != nil {
		t.Error("expected nil for non-existent nested instance")
	}
}

// TestPublicAPIExportedFunctionNil verifies that ExportedFunction
// returns nil for a non-existent function name.
//
// Spec: Explainer.md export lookup.
func TestPublicAPIExportedFunctionNil(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	compiled, err := rt.CompileComponent(ctx, testdata.AddS32Component)
	if err != nil {
		t.Fatalf("CompileComponent: %v", err)
	}
	defer compiled.Close(ctx)

	instance, err := rt.InstantiateComponent(ctx, compiled)
	if err != nil {
		t.Fatalf("InstantiateComponent: %v", err)
	}
	defer instance.Close(ctx)

	fn := instance.ExportedFunction("nonexistent")
	if fn != nil {
		t.Error("expected nil for non-existent function")
	}
}
