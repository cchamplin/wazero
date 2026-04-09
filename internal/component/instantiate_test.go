// internal/component/instantiate_test.go
//
// Integration tests for instantiating and calling the add_s32 test
// component through the public API pipeline (CompileComponent +
// InstantiateComponent + ExportedFunction.Call).
//
// The original tests used the free-function Instantiate path, which
// does not wire ExportedFunc.impl. The restored tests use the full
// ComponentLinker.Instantiate path via wazero.Runtime, which is the
// production path.
package component_test

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/internal/component/testdata"
)

// TestInstantiate_AddS32 verifies the full pipeline for the add_s32
// component: compile, instantiate, get exported "add" function, call
// add(2,3) and assert the result is 5.
//
// Spec: definitions.py:1978-2040 (canon_lift), :2064-2130 (canon_lower)
// for the component function call path.
// Wasmtime parallel: runtime/component/instance.rs:710-833 (Instantiator
// pipeline); tests/all/component_model/func.rs add_s32 tests.
func TestInstantiate_AddS32(t *testing.T) {
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

// TestInstantiate_AddS32_EdgeCases verifies edge cases of the add_s32
// component: zero+zero, negatives, mixed, and boundary values near
// int32 min/max.
//
// Spec: definitions.py:1978-2040 (canon_lift) — the s32 type is
// lifted/lowered as a single i32 core value.
// Wasmtime parallel: tests/all/component_model/func.rs add tests with
// boundary values.
func TestInstantiate_AddS32_EdgeCases(t *testing.T) {
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

	tests := []struct {
		name     string
		a, b     int32
		expected int32
	}{
		{"zero plus zero", 0, 0, 0},
		{"positive plus positive", 2, 3, 5},
		{"negative plus negative", -2, -3, -5},
		{"positive plus negative", 5, -3, 2},
		{"max int32", 2147483646, 1, 2147483647},
		{"min int32", -2147483647, -1, -2147483648},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			results, err := addFunc.Call(ctx, tc.a, tc.b)
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
			if got != tc.expected {
				t.Errorf("expected %d, got %d", tc.expected, got)
			}
		})
	}
}
