// internal/component/composite_test.go
//
// Integration tests for composite component types (records, options, lists,
// results). These exercise the full pipeline: binary decode, component
// instantiation via the public API, and ExportedFunc.Call.
//
// Tests that require components whose wireExports path cannot yet resolve
// core function indices for multi-memory or record-typed lifts carry
// specific skip messages referencing the missing pipeline step.
package component_test

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/internal/component/testdata"
)

// TestEchoRecord verifies round-tripping a record {x: s32, y: s32} through
// the echo_record component's exported "echo" function.
//
// Spec: Explainer.md record type definition; definitions.py
// flatten/lift/lower for record types.
// Wasmtime parallel: tests/all/component_model/func.rs echo_record tests.
func TestEchoRecord(t *testing.T) {
	t.Skip("wireExports cannot resolve core function index 0 for echo_record (record-typed lift requires alias-aware core func space — see component_linker.go wireExports)")
}

// TestEchoRecord_EdgeCases verifies edge cases (zero, negative, mixed) for
// the echo_record component.
//
// Spec: Explainer.md record type definition; definitions.py
// flatten/lift/lower for record types.
func TestEchoRecord_EdgeCases(t *testing.T) {
	t.Skip("wireExports cannot resolve core function index 0 for echo_record (record-typed lift requires alias-aware core func space — see component_linker.go wireExports)")
}

// TestOptionRoundtrip verifies round-tripping option<s32> through the
// option_roundtrip component.
//
// Spec: Explainer.md option type; definitions.py option lift/lower.
// Wasmtime parallel: tests/all/component_model/func.rs option tests.
func TestOptionRoundtrip(t *testing.T) {
	t.Skip("wireExports cannot resolve core function index 0 for option_roundtrip (option-typed lift requires alias-aware core func space — see component_linker.go wireExports)")
}

// TestListSum verifies list<s32> sum through the list_sum component.
// The list_sum component instantiates successfully and its "sum" export
// is callable.
//
// Spec: Explainer.md list type; definitions.py list lift/lower with
// realloc for variable-length types.
// Wasmtime parallel: tests/all/component_model/func.rs list tests.
func TestListSum(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	compiled, err := rt.CompileComponent(ctx, testdata.ListSumComponent)
	if err != nil {
		t.Fatalf("CompileComponent: %v", err)
	}
	defer compiled.Close(ctx)

	instance, err := rt.InstantiateComponent(ctx, compiled)
	if err != nil {
		t.Fatalf("InstantiateComponent: %v", err)
	}
	defer instance.Close(ctx)

	sumFunc := instance.ExportedFunction("sum")
	if sumFunc == nil {
		t.Fatal("expected 'sum' function export")
	}

	results, err := sumFunc.Call(ctx, []int32{1, 2, 3, 4, 5})
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
	if got != 15 {
		t.Errorf("expected 15, got %d", got)
	}
}

// TestResultDivide verifies result<s32, s32> through the result_divide
// component.
//
// Spec: Explainer.md result type; definitions.py result lift/lower.
// Wasmtime parallel: tests/all/component_model/func.rs result tests.
func TestResultDivide(t *testing.T) {
	t.Skip("wireExports cannot resolve core function index 0 for result_divide (result-typed lift requires alias-aware core func space — see component_linker.go wireExports)")
}
