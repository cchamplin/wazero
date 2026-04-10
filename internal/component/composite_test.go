// internal/component/composite_test.go
//
// Integration tests for composite component types (records, options, lists,
// results). These exercise the full pipeline: binary decode, component
// instantiation via the public API, and ExportedFunc.Call.
package component_test

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/internal/component/testdata"
	"github.com/tetratelabs/wazero/internal/component/types"
)

// valOption wraps a types.Val as Some for passing through the public API.
// The anyToVal converter handles types.Val directly.
func valOption(v types.Val) types.Val {
	return types.ValOption(&v)
}

// TestEchoRecord verifies round-tripping a record {x: s32, y: s32} through
// the echo_record component's exported "echo" function.
//
// Spec: Explainer.md record type definition; definitions.py
// flatten/lift/lower for record types.
// Wasmtime parallel: tests/all/component_model/func.rs echo_record tests.
func TestEchoRecord(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	compiled, err := rt.CompileComponent(ctx, testdata.EchoRecordComponent)
	if err != nil {
		t.Fatalf("CompileComponent: %v", err)
	}
	defer compiled.Close(ctx)

	instance, err := rt.InstantiateComponent(ctx, compiled)
	if err != nil {
		t.Fatalf("InstantiateComponent: %v", err)
	}
	defer instance.Close(ctx)

	echoFunc := instance.ExportedFunction("echo")
	if echoFunc == nil {
		t.Fatal("expected 'echo' function export")
	}

	// The echo_record core module doubles x and y.
	// Input: {x: 3, y: 4} -> Output: {x: 6, y: 8}
	results, err := echoFunc.Call(ctx, map[string]any{"x": int32(3), "y": int32(4)})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	rec, ok := results[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any result, got %T", results[0])
	}
	x, ok := rec["x"].(int32)
	if !ok {
		t.Fatalf("expected int32 for x, got %T", rec["x"])
	}
	y, ok := rec["y"].(int32)
	if !ok {
		t.Fatalf("expected int32 for y, got %T", rec["y"])
	}
	if x != 6 {
		t.Errorf("expected x=6, got %d", x)
	}
	if y != 8 {
		t.Errorf("expected y=8, got %d", y)
	}
}

// TestEchoRecord_EdgeCases verifies edge cases (zero, negative, mixed) for
// the echo_record component.
//
// Spec: Explainer.md record type definition; definitions.py
// flatten/lift/lower for record types.
func TestEchoRecord_EdgeCases(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	compiled, err := rt.CompileComponent(ctx, testdata.EchoRecordComponent)
	if err != nil {
		t.Fatalf("CompileComponent: %v", err)
	}
	defer compiled.Close(ctx)

	instance, err := rt.InstantiateComponent(ctx, compiled)
	if err != nil {
		t.Fatalf("InstantiateComponent: %v", err)
	}
	defer instance.Close(ctx)

	echoFunc := instance.ExportedFunction("echo")
	if echoFunc == nil {
		t.Fatal("expected 'echo' function export")
	}

	tests := []struct {
		name  string
		x, y  int32
		wantX int32
		wantY int32
	}{
		{"zeros", 0, 0, 0, 0},
		{"negatives", -5, -3, -10, -6},
		{"mixed", -1, 1, -2, 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			results, err := echoFunc.Call(ctx, map[string]any{"x": tc.x, "y": tc.y})
			if err != nil {
				t.Fatalf("Call: %v", err)
			}
			if len(results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(results))
			}
			rec := results[0].(map[string]any)
			gotX := rec["x"].(int32)
			gotY := rec["y"].(int32)
			if gotX != tc.wantX {
				t.Errorf("x: expected %d, got %d", tc.wantX, gotX)
			}
			if gotY != tc.wantY {
				t.Errorf("y: expected %d, got %d", tc.wantY, gotY)
			}
		})
	}
}

// TestOptionRoundtrip verifies round-tripping option<s32> through the
// option_roundtrip component.
//
// Spec: Explainer.md option type; definitions.py option lift/lower.
// Wasmtime parallel: tests/all/component_model/func.rs option tests.
func TestOptionRoundtrip(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	compiled, err := rt.CompileComponent(ctx, testdata.OptionRoundtripComponent)
	if err != nil {
		t.Fatalf("CompileComponent: %v", err)
	}
	defer compiled.Close(ctx)

	instance, err := rt.InstantiateComponent(ctx, compiled)
	if err != nil {
		t.Fatalf("InstantiateComponent: %v", err)
	}
	defer instance.Close(ctx)

	echoFunc := instance.ExportedFunction("echo")
	if echoFunc == nil {
		t.Fatal("expected 'echo' function export")
	}

	// Pass Some(42) as an option<s32> using types.Val.
	results, err := echoFunc.Call(ctx, valOption(types.ValS32(42)))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	// valToAny converts Some(42) to the unwrapped value: int32(42).
	// None would be returned as nil.
	got, ok := results[0].(int32)
	if !ok {
		t.Fatalf("expected int32 result (unwrapped Some), got %T: %v", results[0], results[0])
	}
	if got != 42 {
		t.Errorf("expected 42, got %d", got)
	}
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
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	compiled, err := rt.CompileComponent(ctx, testdata.ResultDivideComponent)
	if err != nil {
		t.Fatalf("CompileComponent: %v", err)
	}
	defer compiled.Close(ctx)

	instance, err := rt.InstantiateComponent(ctx, compiled)
	if err != nil {
		t.Fatalf("InstantiateComponent: %v", err)
	}
	defer instance.Close(ctx)

	divideFunc := instance.ExportedFunction("divide")
	if divideFunc == nil {
		t.Fatal("expected 'divide' function export")
	}

	// Successful division: 10 / 3 = 3 -> Ok(3)
	results, err := divideFunc.Call(ctx, int32(10), int32(3))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	// valToAny converts result to map[string]any{"ok": bool, "value"/"error": any}
	resultMap, ok := results[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any result, got %T: %v", results[0], results[0])
	}
	isOk, _ := resultMap["ok"].(bool)
	if !isOk {
		t.Fatalf("expected Ok result, got Error: %v", resultMap)
	}
	value, _ := resultMap["value"].(int32)
	if value != 3 {
		t.Errorf("expected Ok(3), got Ok(%d)", value)
	}
}
