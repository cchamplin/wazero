# Phase 2: Start Function Support

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Execute component start functions during instantiation, managing the value index space per Component Model spec.

**Architecture:** Add value index space to Instance, process value imports, execute start function after core modules but before exports, track value consumption.

**Tech Stack:** Go

**Parent Plan:** [00-root.md](./00-root.md)

**Prerequisite:** Phase 1 (Type Checking) must be complete.

**Gap Analysis Reference:** [Section 7: Start Function Handling](../2026-01-20-instantiation-linking-gap-analysis.md#7-start-function-handling)

---

## Spec References

Read these before starting:
- `debug-vendored/component-model/design/mvp/Binary.md` lines 360-379 (Start definition)
- `debug-vendored/component-model/design/mvp/Explainer.md` lines 2436-2476 (Start semantics)

Key requirements:
- Start function called during instantiation
- Results appended to value index space
- Each value consumed exactly once (tracked via consumed flag)

---

## Task 1: Add Value Index Space to Instance

**Files:**
- Modify: `internal/component/instance.go`
- Test: `internal/component/instance_test.go`

### Step 1: Write the failing test for value index space

```go
// internal/component/instance_test.go
package component

import (
	"testing"
)

func TestInstance_ValueIndexSpace(t *testing.T) {
	inst := &Instance{}

	// Should be able to add values
	idx := inst.AddValue(ValS32(42))
	if idx != 0 {
		t.Errorf("first value should be index 0, got %d", idx)
	}

	// Should be able to get values
	val, err := inst.GetValue(0)
	if err != nil {
		t.Errorf("GetValue failed: %v", err)
	}
	if val.S32() != 42 {
		t.Errorf("expected 42, got %d", val.S32())
	}

	// Values should not be consumed initially
	if inst.IsValueConsumed(0) {
		t.Error("value should not be consumed initially")
	}
}

func TestInstance_ConsumeValue(t *testing.T) {
	inst := &Instance{}
	inst.AddValue(ValS32(42))

	// Consume the value
	val, err := inst.ConsumeValue(0)
	if err != nil {
		t.Errorf("ConsumeValue failed: %v", err)
	}
	if val.S32() != 42 {
		t.Errorf("expected 42, got %d", val.S32())
	}

	// Should be marked consumed
	if !inst.IsValueConsumed(0) {
		t.Error("value should be consumed after ConsumeValue")
	}

	// Consuming again should fail
	_, err = inst.ConsumeValue(0)
	if err == nil {
		t.Error("consuming same value twice should fail")
	}
}
```

### Step 2: Run test to verify it fails

Run: `CGO_ENABLED=0 go test ./internal/component/... -run TestInstance_Value -v`

Expected: FAIL with "undefined: inst.AddValue"

### Step 3: Write minimal implementation

```go
// Add to internal/component/instance.go

// In the Instance struct, add these fields:
//
// // Value index space for start function support
// values         []Val
// valuesConsumed []bool

// AddValue adds a value to the instance's value index space.
// Returns the index of the added value.
func (i *Instance) AddValue(v Val) uint32 {
	if i.values == nil {
		i.values = make([]Val, 0)
		i.valuesConsumed = make([]bool, 0)
	}
	idx := uint32(len(i.values))
	i.values = append(i.values, v)
	i.valuesConsumed = append(i.valuesConsumed, false)
	return idx
}

// GetValue retrieves a value from the value index space.
func (i *Instance) GetValue(idx uint32) (Val, error) {
	if idx >= uint32(len(i.values)) {
		return Val{}, fmt.Errorf("value index %d out of range (have %d)", idx, len(i.values))
	}
	return i.values[idx], nil
}

// ConsumeValue retrieves and marks a value as consumed.
// Returns error if value already consumed or index out of range.
func (i *Instance) ConsumeValue(idx uint32) (Val, error) {
	if idx >= uint32(len(i.values)) {
		return Val{}, fmt.Errorf("value index %d out of range (have %d)", idx, len(i.values))
	}
	if i.valuesConsumed[idx] {
		return Val{}, fmt.Errorf("value %d already consumed", idx)
	}
	i.valuesConsumed[idx] = true
	return i.values[idx], nil
}

// IsValueConsumed returns whether a value has been consumed.
func (i *Instance) IsValueConsumed(idx uint32) bool {
	if idx >= uint32(len(i.valuesConsumed)) {
		return false
	}
	return i.valuesConsumed[idx]
}
```

### Step 4: Run test to verify it passes

Run: `CGO_ENABLED=0 go test ./internal/component/... -run TestInstance_Value -v`

Expected: PASS

### Step 5: Commit

```bash
git add internal/component/instance.go internal/component/instance_test.go
git commit -m "feat(component): add value index space to Instance

Values can be added, retrieved, and consumed exactly once.
Supports start function result storage per spec.

Refs: docs/plans/spec-instantiation-linking-conformance-fixes"
```

---

## Task 2: Add Start Definition to Component

**Files:**
- Modify: `internal/component/component.go`
- Test: `internal/component/component_test.go`

### Step 1: Write the failing test for Start structure

```go
// Add to internal/component/component_test.go (create if needed)
package component

import (
	"testing"
)

func TestComponent_StartDef(t *testing.T) {
	start := &StartDef{
		FuncIdx:     5,
		Args:        []uint32{0, 1},
		ResultCount: 2,
	}

	if start.FuncIdx != 5 {
		t.Errorf("expected FuncIdx 5, got %d", start.FuncIdx)
	}
	if len(start.Args) != 2 {
		t.Errorf("expected 2 args, got %d", len(start.Args))
	}
	if start.ResultCount != 2 {
		t.Errorf("expected 2 results, got %d", start.ResultCount)
	}
}
```

### Step 2: Run test to verify it fails

Run: `CGO_ENABLED=0 go test ./internal/component/... -run TestComponent_StartDef -v`

Expected: FAIL with "undefined: StartDef"

### Step 3: Write minimal implementation

```go
// Add to internal/component/component.go

// StartDef defines a component's start function.
// The start function is called during instantiation.
type StartDef struct {
	FuncIdx     uint32   // Index of the function to call
	Args        []uint32 // Value indices to pass as arguments
	ResultCount uint32   // Number of result values to expect
}

// In the Component struct, add:
// Start *StartDef // Optional start function
```

### Step 4: Run test to verify it passes

Run: `CGO_ENABLED=0 go test ./internal/component/... -run TestComponent_StartDef -v`

Expected: PASS

### Step 5: Commit

```bash
git add internal/component/component.go internal/component/component_test.go
git commit -m "feat(component): add StartDef structure

Defines the start function index, arguments, and result count.
Start field added to Component struct.

Refs: docs/plans/spec-instantiation-linking-conformance-fixes"
```

---

## Task 3: Implement Start Function Execution

**Files:**
- Modify: `internal/component/component_linker.go`
- Test: `internal/component/start_function_test.go`

### Step 1: Write the failing test for start function execution

```go
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
			Args:        []uint32{0}, // Use value at index 0
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
			Args:        []uint32{0},
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
```

### Step 2: Run test to verify it fails

Run: `CGO_ENABLED=0 go test ./internal/component/... -run TestExecuteStartFunction -v`

Expected: FAIL with "undefined: l.executeStartFunction"

### Step 3: Write minimal implementation

```go
// Add to internal/component/component_linker.go

// executeStartFunction executes the component's start function if defined.
// Called after core module instantiation, before wiring exports.
func (l *ComponentLinker) executeStartFunction(ctx context.Context, inst *Instance, c *Component) error {
	if c.Start == nil {
		return nil // No start function
	}

	// Get the start function
	startFunc, ok := inst.componentFuncs[c.Start.FuncIdx]
	if !ok {
		return fmt.Errorf("start function %d not found", c.Start.FuncIdx)
	}
	if startFunc.Impl == nil {
		return fmt.Errorf("start function %d has no implementation", c.Start.FuncIdx)
	}

	// Gather value arguments and mark as consumed
	args := make([]Val, len(c.Start.Args))
	for i, argIdx := range c.Start.Args {
		val, err := inst.ConsumeValue(argIdx)
		if err != nil {
			return fmt.Errorf("start arg %d: %w", i, err)
		}
		args[i] = val
	}

	// Call start function
	results, err := startFunc.Impl(ctx, args)
	if err != nil {
		return fmt.Errorf("start function failed: %w", err)
	}

	// Verify result count matches declaration
	if uint32(len(results)) != c.Start.ResultCount {
		return fmt.Errorf("start function returned %d values, expected %d",
			len(results), c.Start.ResultCount)
	}

	// Append results to value index space
	for _, r := range results {
		inst.AddValue(r)
	}

	return nil
}
```

### Step 4: Run test to verify it passes

Run: `CGO_ENABLED=0 go test ./internal/component/... -run TestExecuteStartFunction -v`

Expected: PASS

### Step 5: Commit

```bash
git add internal/component/component_linker.go internal/component/start_function_test.go
git commit -m "feat(component): implement start function execution

Start function is called with consumed values.
Results are appended to value index space.
Proper error handling for missing functions and consumed values.

Refs: docs/plans/spec-instantiation-linking-conformance-fixes"
```

---

## Task 4: Integrate Start Function into Instantiate

**Files:**
- Modify: `internal/component/component_linker.go`

### Step 1: Identify integration point

The start function should be called:
- AFTER core module instantiation
- AFTER component functions are built
- BEFORE exports are wired

Find this comment in `component_linker.go`:
```go
// Step 3: Wire exports
```

### Step 2: Add start function execution

Insert before "Step 3: Wire exports":

```go
// Step 2.5: Execute start function if defined
if err := l.executeStartFunction(ctx, inst, c); err != nil {
	return nil, fmt.Errorf("start function: %w", err)
}
```

### Step 3: Run tests to verify no regression

Run: `CGO_ENABLED=0 go test ./internal/component/... -v`

Expected: All PASS

### Step 4: Commit

```bash
git add internal/component/component_linker.go
git commit -m "feat(component): integrate start function into Instantiate

Start function now called during component instantiation.
Executed after core modules, before export wiring.

Refs: docs/plans/spec-instantiation-linking-conformance-fixes"
```

---

## Task 5: Process Value Imports

**Files:**
- Modify: `internal/component/component_linker.go`
- Test: `internal/component/value_import_test.go`

### Step 1: Write the failing test for value imports

```go
// internal/component/value_import_test.go
package component

import (
	"context"
	"testing"
)

func TestValueImport(t *testing.T) {
	// Component that imports a value
	c := &Component{
		Imports: []Import{
			{
				Name: "config/name",
				ExternDesc: ImportExternDesc{
					Kind: ImportExternDescValue,
				},
			},
		},
	}

	compiled := &CompiledComponent{
		component: c,
	}

	linker := NewComponentLinker(nil)

	// Define the value
	linker.DefineValue("config", "name", ValString("TestApp"))

	ctx := context.Background()
	inst, err := linker.Instantiate(ctx, compiled)
	if err != nil {
		t.Fatalf("Instantiate failed: %v", err)
	}

	// Value should be in value index space
	val, err := inst.GetValue(0)
	if err != nil {
		t.Fatalf("GetValue failed: %v", err)
	}
	if val.StringVal() != "TestApp" {
		t.Errorf("expected 'TestApp', got '%s'", val.StringVal())
	}
}
```

### Step 2: Run test to verify it fails

Run: `CGO_ENABLED=0 go test ./internal/component/... -run TestValueImport -v`

Expected: FAIL with "undefined: linker.DefineValue"

### Step 3: Write minimal implementation

```go
// Add to internal/component/component_linker.go

// DefineValue adds a value definition for value imports.
func (l *ComponentLinker) DefineValue(namespace, name string, value Val) error {
	key := namespace + "/" + name
	if _, exists := l.definitions[key]; exists {
		return fmt.Errorf("definition already exists: %s", key)
	}
	l.definitions[key] = &ValueDef{Value: value}
	return nil
}

// In Instantiate, after resolving imports, add value imports to value index space:
// (Add this after the existing import resolution loop)

// Process value imports to populate value index space
for _, imp := range c.Imports {
	if imp.ExternDesc.Kind == ImportExternDescValue {
		def := resolvedImports[imp.Name]
		valueDef, ok := def.(*ValueDef)
		if !ok {
			return nil, fmt.Errorf("import %q: expected value, got %T", imp.Name, def)
		}
		inst.AddValue(valueDef.Value)
	}
}
```

### Step 4: Run test to verify it passes

Run: `CGO_ENABLED=0 go test ./internal/component/... -run TestValueImport -v`

Expected: PASS

### Step 5: Commit

```bash
git add internal/component/component_linker.go internal/component/value_import_test.go
git commit -m "feat(component): support value imports

Values are added to the instance's value index space.
DefineValue method added to ComponentLinker.

Refs: docs/plans/spec-instantiation-linking-conformance-fixes"
```

---

## Task 6: Run Phase 2 Regression Tests

**Files:** None (verification only)

### Step 1: Run all start function tests

Run: `CGO_ENABLED=0 go test ./internal/component/... -run "StartFunction|ValueImport|Instance_Value" -v`

Expected: All PASS

### Step 2: Run calculator regression tests

Run: `CGO_ENABLED=0 go test ./internal/component/wasip2test/... -run TestCalculatorPlugins/add -v`

Expected: PASS

Run: `CGO_ENABLED=0 go test ./internal/component/wasip2test/... -run TestCalculatorPlugins/subtract -v`

Expected: PASS

### Step 3: Update progress tracker

Edit `docs/plans/spec-instantiation-linking-conformance-fixes/00-root.md`:
- Mark Phase 2 status as `[x] Complete`
- Mark Phase 2 regression as `[x] Verified`

### Step 4: Commit

```bash
git add docs/plans/spec-instantiation-linking-conformance-fixes/00-root.md
git commit -m "docs: mark Phase 2 (Start Function) complete

All start function tests pass.
Calculator add/subtract regression tests pass.

Refs: docs/plans/spec-instantiation-linking-conformance-fixes"
```

---

## Phase 2 Complete

**Summary of changes:**
- Added value index space to Instance (AddValue, GetValue, ConsumeValue)
- Added StartDef structure to Component
- Implemented executeStartFunction in ComponentLinker
- Integrated start function execution into Instantiate
- Added DefineValue for value imports

**Next steps:**
- Proceed to Phase 3 (Nested Components) or Phase 4 (Export Instance API)
- Phase 3 enables nested component instantiation
- Phase 4 requires Phase 3
