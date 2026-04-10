// internal/component/start_function_test.go
//
// Tests for ComponentLinker.executeStartFunction.
//
// Spec: Explainer.md start function (lines 2436-2476).
// Wasmtime parallel: runtime/component/instance.rs start function execution.
package component

import (
	"context"
	"errors"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/internal/component/types"
)

func TestStartFunction_NoStart(t *testing.T) {
	// Spec: Explainer.md start function.
	// Wasmtime parallel: runtime/component/instance.rs start function execution.
	//
	// When c.Start is nil, executeStartFunction should return nil immediately.
	c := &Component{}
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	l := NewComponentLinker(rt)
	inst := NewInstance(c, 0, nil)

	err := l.executeStartFunction(context.Background(), inst, c)
	if err != nil {
		t.Fatalf("expected no error for nil Start, got: %v", err)
	}
}

func TestStartFunction_ExecutesOnce(t *testing.T) {
	// Spec: Explainer.md start function.
	// Wasmtime parallel: runtime/component/instance.rs start function execution.
	//
	// A start function is called with the specified args consumed from the
	// value index space, and its results are appended back.
	ft := types.TypeFunc{}
	c := &Component{
		Types: &types.ComponentTypes{
			Funcs: []types.TypeFunc{ft},
		},
		Start: &StartDef{
			FuncIdx:     0,
			ArgValueIdx: []uint32{0},
			ResultCount: 1,
		},
	}
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	l := NewComponentLinker(rt)
	inst := NewInstance(c, 0, nil)

	// Pre-populate value index space with an argument.
	inst.AddValue(types.ValS32(42))

	var called bool
	var gotArgs []types.Val
	inst.componentFuncs[0] = ComponentFunc{
		Type: &c.Types.Funcs[0],
		Impl: func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			called = true
			gotArgs = args
			return []types.Val{types.ValString("hello")}, nil
		},
	}

	err := l.executeStartFunction(context.Background(), inst, c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected start function to be called")
	}
	if len(gotArgs) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(gotArgs))
	}
	if gotArgs[0].S32() != 42 {
		t.Fatalf("expected arg value 42, got %v", gotArgs[0].S32())
	}

	// The result should be appended to the value index space.
	// Index 0 was the argument, index 1 should be the result.
	resultVal, err := inst.GetValue(1)
	if err != nil {
		t.Fatalf("failed to get result value: %v", err)
	}
	if resultVal.StringVal() != "hello" {
		t.Fatalf("expected result 'hello', got %v", resultVal.StringVal())
	}
}

func TestStartFunction_FuncNotFound(t *testing.T) {
	// Spec: Explainer.md start function.
	// Wasmtime parallel: runtime/component/instance.rs start function execution.
	//
	// If the function index does not exist in componentFuncs, return an error.
	c := &Component{
		Start: &StartDef{
			FuncIdx:     99,
			ArgValueIdx: nil,
			ResultCount: 0,
		},
	}
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	l := NewComponentLinker(rt)
	inst := NewInstance(c, 0, nil)

	err := l.executeStartFunction(context.Background(), inst, c)
	if err == nil {
		t.Fatal("expected error for missing function, got nil")
	}
	if !containsString(err.Error(), "component function") {
		t.Fatalf("expected error about component function, got: %v", err)
	}
}

func TestStartFunction_NilImpl(t *testing.T) {
	// Spec: Explainer.md start function.
	// Wasmtime parallel: runtime/component/instance.rs start function execution.
	//
	// If the function exists but Impl is nil, return an error.
	ft := types.TypeFunc{}
	c := &Component{
		Types: &types.ComponentTypes{
			Funcs: []types.TypeFunc{ft},
		},
		Start: &StartDef{
			FuncIdx:     0,
			ArgValueIdx: nil,
			ResultCount: 0,
		},
	}
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	l := NewComponentLinker(rt)
	inst := NewInstance(c, 0, nil)
	inst.componentFuncs[0] = ComponentFunc{
		Type: &c.Types.Funcs[0],
		Impl: nil,
	}

	err := l.executeStartFunction(context.Background(), inst, c)
	if err == nil {
		t.Fatal("expected error for nil Impl, got nil")
	}
	if !containsString(err.Error(), "nil") {
		t.Fatalf("expected error mentioning nil, got: %v", err)
	}
}

func TestStartFunction_ConsumeArgs(t *testing.T) {
	// Spec: Explainer.md start function.
	// Wasmtime parallel: runtime/component/instance.rs start function execution.
	//
	// After calling the start function, the argument values should be consumed
	// and cannot be consumed again.
	ft := types.TypeFunc{}
	c := &Component{
		Types: &types.ComponentTypes{
			Funcs: []types.TypeFunc{ft},
		},
		Start: &StartDef{
			FuncIdx:     0,
			ArgValueIdx: []uint32{0, 1},
			ResultCount: 0,
		},
	}
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	l := NewComponentLinker(rt)
	inst := NewInstance(c, 0, nil)

	inst.AddValue(types.ValS32(10))
	inst.AddValue(types.ValS32(20))

	inst.componentFuncs[0] = ComponentFunc{
		Type: &c.Types.Funcs[0],
		Impl: func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			return nil, nil
		},
	}

	err := l.executeStartFunction(context.Background(), inst, c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both values should now be consumed.
	if !inst.IsValueConsumed(0) {
		t.Fatal("expected value 0 to be consumed")
	}
	if !inst.IsValueConsumed(1) {
		t.Fatal("expected value 1 to be consumed")
	}

	// Trying to consume again should fail.
	_, err = inst.ConsumeValue(0)
	if err == nil {
		t.Fatal("expected error when consuming already-consumed value")
	}
}

func TestStartFunction_ValueAlreadyConsumed(t *testing.T) {
	// Spec: Explainer.md start function.
	// Wasmtime parallel: runtime/component/instance.rs start function execution.
	//
	// If a value has already been consumed, the start function should fail.
	ft := types.TypeFunc{}
	c := &Component{
		Types: &types.ComponentTypes{
			Funcs: []types.TypeFunc{ft},
		},
		Start: &StartDef{
			FuncIdx:     0,
			ArgValueIdx: []uint32{0},
			ResultCount: 0,
		},
	}
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	l := NewComponentLinker(rt)
	inst := NewInstance(c, 0, nil)

	inst.AddValue(types.ValS32(42))
	// Pre-consume the value.
	inst.ConsumeValue(0)

	inst.componentFuncs[0] = ComponentFunc{
		Type: &c.Types.Funcs[0],
		Impl: func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			return nil, nil
		},
	}

	err := l.executeStartFunction(context.Background(), inst, c)
	if err == nil {
		t.Fatal("expected error for already-consumed value, got nil")
	}
}

func TestStartFunction_ValueIndexOutOfRange(t *testing.T) {
	// Spec: Explainer.md start function.
	// Wasmtime parallel: runtime/component/instance.rs start function execution.
	//
	// If a value index is out of range, return an error.
	ft := types.TypeFunc{}
	c := &Component{
		Types: &types.ComponentTypes{
			Funcs: []types.TypeFunc{ft},
		},
		Start: &StartDef{
			FuncIdx:     0,
			ArgValueIdx: []uint32{99},
			ResultCount: 0,
		},
	}
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	l := NewComponentLinker(rt)
	inst := NewInstance(c, 0, nil)

	inst.componentFuncs[0] = ComponentFunc{
		Type: &c.Types.Funcs[0],
		Impl: func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			return nil, nil
		},
	}

	err := l.executeStartFunction(context.Background(), inst, c)
	if err == nil {
		t.Fatal("expected error for out-of-range value index, got nil")
	}
}

func TestStartFunction_ResultCountMismatch(t *testing.T) {
	// Spec: Explainer.md start function.
	// Wasmtime parallel: runtime/component/instance.rs start function execution.
	//
	// If the function returns a different number of results than expected,
	// return an error.
	ft := types.TypeFunc{}
	c := &Component{
		Types: &types.ComponentTypes{
			Funcs: []types.TypeFunc{ft},
		},
		Start: &StartDef{
			FuncIdx:     0,
			ArgValueIdx: nil,
			ResultCount: 2,
		},
	}
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	l := NewComponentLinker(rt)
	inst := NewInstance(c, 0, nil)

	inst.componentFuncs[0] = ComponentFunc{
		Type: &c.Types.Funcs[0],
		Impl: func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			// Return only 1 result when 2 are expected.
			return []types.Val{types.ValS32(1)}, nil
		},
	}

	err := l.executeStartFunction(context.Background(), inst, c)
	if err == nil {
		t.Fatal("expected error for result count mismatch, got nil")
	}
	if !containsString(err.Error(), "result") {
		t.Fatalf("expected error about results, got: %v", err)
	}
}

func TestStartFunction_MultipleResults(t *testing.T) {
	// Spec: Explainer.md start function.
	// Wasmtime parallel: runtime/component/instance.rs start function execution.
	//
	// Multiple results are appended to the value index space in order.
	ft := types.TypeFunc{}
	c := &Component{
		Types: &types.ComponentTypes{
			Funcs: []types.TypeFunc{ft},
		},
		Start: &StartDef{
			FuncIdx:     0,
			ArgValueIdx: nil,
			ResultCount: 2,
		},
	}
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	l := NewComponentLinker(rt)
	inst := NewInstance(c, 0, nil)

	inst.componentFuncs[0] = ComponentFunc{
		Type: &c.Types.Funcs[0],
		Impl: func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			return []types.Val{types.ValS32(10), types.ValString("world")}, nil
		},
	}

	err := l.executeStartFunction(context.Background(), inst, c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Results should be at index 0 and 1 (no prior values).
	v0, err := inst.GetValue(0)
	if err != nil {
		t.Fatalf("failed to get result 0: %v", err)
	}
	if v0.S32() != 10 {
		t.Fatalf("expected result 0 = 10, got %v", v0.S32())
	}

	v1, err := inst.GetValue(1)
	if err != nil {
		t.Fatalf("failed to get result 1: %v", err)
	}
	if v1.StringVal() != "world" {
		t.Fatalf("expected result 1 = 'world', got %v", v1.StringVal())
	}
}

func TestStartFunction_ReturnsError(t *testing.T) {
	// Spec: Explainer.md start function.
	// Wasmtime parallel: runtime/component/instance.rs start function execution.
	//
	// If the start function itself returns an error, propagate it.
	ft := types.TypeFunc{}
	c := &Component{
		Types: &types.ComponentTypes{
			Funcs: []types.TypeFunc{ft},
		},
		Start: &StartDef{
			FuncIdx:     0,
			ArgValueIdx: nil,
			ResultCount: 0,
		},
	}
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	l := NewComponentLinker(rt)
	inst := NewInstance(c, 0, nil)

	startErr := errors.New("start function failed")
	inst.componentFuncs[0] = ComponentFunc{
		Type: &c.Types.Funcs[0],
		Impl: func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			return nil, startErr
		},
	}

	err := l.executeStartFunction(context.Background(), inst, c)
	if err == nil {
		t.Fatal("expected error from start function, got nil")
	}
	if !errors.Is(err, startErr) {
		t.Fatalf("expected wrapped startErr, got: %v", err)
	}
}

// containsString is a helper to check if a string contains a substring.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
