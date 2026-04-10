// internal/component/integration_records_test.go
//
// Integration tests for record types through the public API. These exercise
// the full pipeline: wazero.Runtime.CompileComponent, InstantiateComponent,
// and ExportedFunction.Call with record-valued parameters and results.
package component_test

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/internal/component/testdata"
)

// TestPublicAPIRecordEcho verifies round-tripping a record {x: s32, y: s32}
// through the echo_record component via the public API.
//
// Spec: Explainer.md record type definition; definitions.py
// flatten/lift/lower for record types at :1978-2040 (canon_lift).
// Wasmtime parallel: tests/all/component_model/func.rs echo_record.
func TestPublicAPIRecordEcho(t *testing.T) {
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
	// Input: {x: 7, y: 11} -> Output: {x: 14, y: 22}
	results, err := echoFunc.Call(ctx, map[string]any{"x": int32(7), "y": int32(11)})
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
	x := rec["x"].(int32)
	y := rec["y"].(int32)
	if x != 14 {
		t.Errorf("expected x=14, got %d", x)
	}
	if y != 22 {
		t.Errorf("expected y=22, got %d", y)
	}
}

// TestPublicAPIRecordWithDifferentValues verifies edge cases (zeros,
// negatives, mixed, boundary values) for the echo_record component.
//
// Spec: Explainer.md record type definition; definitions.py record
// flatten/lift/lower.
func TestPublicAPIRecordWithDifferentValues(t *testing.T) {
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
		{"positive", 100, 200, 200, 400},
		{"zero", 0, 0, 0, 0},
		{"negative", -50, -25, -100, -50},
		{"mixed", 10, -10, 20, -20},
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
