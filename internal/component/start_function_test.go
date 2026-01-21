// internal/component/start_function_test.go
package component

import (
	"context"
	"testing"
)

func TestExecuteStartFunction_Basic(t *testing.T) {
	// Create instance with a component function
	c := &Component{
		Start: &StartDef{
			FuncIdx:     0,
			ArgValueIdx: []uint32{0}, // Use value at index 0
			ResultCount: 1,
		},
	}

	inst := &Instance{
		component:      c,
		componentFuncs: make(map[uint32]ComponentFunc),
	}

	// Add a value to be passed to start function
	inst.AddValue(ValString("World"))

	// Add the start function that prepends "Hello, "
	inst.componentFuncs[0] = ComponentFunc{
		Impl: func(ctx context.Context, args []Val) ([]Val, error) {
			name := args[0].StringVal()
			return []Val{ValString("Hello, " + name)}, nil
		},
	}

	// Create a mock linker
	l := &ComponentLinker{}

	// Execute start function
	err := l.executeStartFunction(context.Background(), inst, c)
	if err != nil {
		t.Fatalf("executeStartFunction failed: %v", err)
	}

	// Input value should be consumed
	if !inst.IsValueConsumed(0) {
		t.Error("input value should be consumed")
	}

	// Result should be in value index space at index 1
	result, err := inst.GetValue(1)
	if err != nil {
		t.Fatalf("GetValue failed: %v", err)
	}
	if result.StringVal() != "Hello, World" {
		t.Errorf("expected 'Hello, World', got '%s'", result.StringVal())
	}
}

func TestExecuteStartFunction_NoStart(t *testing.T) {
	c := &Component{
		Start: nil, // No start function
	}
	inst := &Instance{component: c}
	l := &ComponentLinker{}

	// Should succeed with no-op
	err := l.executeStartFunction(context.Background(), inst, c)
	if err != nil {
		t.Errorf("no start function should not error: %v", err)
	}
}

func TestExecuteStartFunction_ValueAlreadyConsumed(t *testing.T) {
	c := &Component{
		Start: &StartDef{
			FuncIdx:     0,
			ArgValueIdx: []uint32{0},
			ResultCount: 0,
		},
	}

	inst := &Instance{
		component:      c,
		componentFuncs: make(map[uint32]ComponentFunc),
	}

	// Add and immediately consume the value
	inst.AddValue(ValS32(42))
	_, _ = inst.ConsumeValue(0)

	inst.componentFuncs[0] = ComponentFunc{
		Impl: func(ctx context.Context, args []Val) ([]Val, error) {
			return nil, nil
		},
	}

	l := &ComponentLinker{}

	// Should fail because value already consumed
	err := l.executeStartFunction(context.Background(), inst, c)
	if err == nil {
		t.Error("should fail when value already consumed")
	}
}

func TestExecuteStartFunction_MultipleArgs(t *testing.T) {
	c := &Component{
		Start: &StartDef{
			FuncIdx:     0,
			ArgValueIdx: []uint32{0, 1}, // Use values at indices 0 and 1
			ResultCount: 1,
		},
	}

	inst := &Instance{
		component:      c,
		componentFuncs: make(map[uint32]ComponentFunc),
	}

	// Add values to be passed to start function
	inst.AddValue(ValS32(10))
	inst.AddValue(ValS32(20))

	// Add the start function that adds two numbers
	inst.componentFuncs[0] = ComponentFunc{
		Impl: func(ctx context.Context, args []Val) ([]Val, error) {
			a := args[0].S32()
			b := args[1].S32()
			return []Val{ValS32(a + b)}, nil
		},
	}

	l := &ComponentLinker{}

	err := l.executeStartFunction(context.Background(), inst, c)
	if err != nil {
		t.Fatalf("executeStartFunction failed: %v", err)
	}

	// Both input values should be consumed
	if !inst.IsValueConsumed(0) {
		t.Error("value 0 should be consumed")
	}
	if !inst.IsValueConsumed(1) {
		t.Error("value 1 should be consumed")
	}

	// Result should be in value index space at index 2
	result, err := inst.GetValue(2)
	if err != nil {
		t.Fatalf("GetValue failed: %v", err)
	}
	if result.S32() != 30 {
		t.Errorf("expected 30, got %d", result.S32())
	}
}

func TestExecuteStartFunction_FunctionNotFound(t *testing.T) {
	c := &Component{
		Start: &StartDef{
			FuncIdx:     99, // Non-existent function
			ArgValueIdx: []uint32{},
			ResultCount: 0,
		},
	}

	inst := &Instance{
		component:      c,
		componentFuncs: make(map[uint32]ComponentFunc),
	}

	l := &ComponentLinker{}

	err := l.executeStartFunction(context.Background(), inst, c)
	if err == nil {
		t.Error("should fail when function not found")
	}
}

func TestExecuteStartFunction_ResultCountMismatch(t *testing.T) {
	c := &Component{
		Start: &StartDef{
			FuncIdx:     0,
			ArgValueIdx: []uint32{},
			ResultCount: 2, // Expect 2 results
		},
	}

	inst := &Instance{
		component:      c,
		componentFuncs: make(map[uint32]ComponentFunc),
	}

	// Function returns only 1 result
	inst.componentFuncs[0] = ComponentFunc{
		Impl: func(ctx context.Context, args []Val) ([]Val, error) {
			return []Val{ValS32(42)}, nil
		},
	}

	l := &ComponentLinker{}

	err := l.executeStartFunction(context.Background(), inst, c)
	if err == nil {
		t.Error("should fail when result count mismatches")
	}
}
