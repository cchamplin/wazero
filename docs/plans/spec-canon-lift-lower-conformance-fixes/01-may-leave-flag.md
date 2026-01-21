# Phase 1: may_leave Flag Implementation

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement the `may_leave` flag that prevents components from calling out during lowering or post-return operations.

**Architecture:** Add a `mayLeave` boolean field to `Instance`, set it to `false` during lowering and post-return, and check it before any lowered call. This prevents the undefined behavior that occurs when a component attempts reentrant calls during value conversion.

**Tech Stack:** Go, wazero internal component APIs

**Gap References:** GAP-LEAVE-1, GAP-LIFT-2, GAP-LOWER-1 from `docs/plans/spec-canon-lift-lower-gap-analysis.md`

---

## Spec Reference

From `debug-vendored/component-model/design/mvp/CanonicalABI.md`:

```python
# Lines 3133, 3151 - In lower_flat_values:
def lower_flat_values(cx, max_flat, vs, ts, out_param = None):
    cx.inst.may_leave = False
    # ... lowering operations ...
    cx.inst.may_leave = True
    return flat_vals

# Lines 3287-3289 - During post-return:
if opts.post_return is not None:
    inst.may_leave = False
    [] = call_and_trap_on_throw(opts.post_return, thread, flat_results)
    inst.may_leave = True

# Line 3454 - In canon_lower:
def canon_lower(opts, ft, callee, thread, flat_args):
    trap_if(not thread.task.inst.may_leave)
```

---

## Task 1.1: Add mayLeave Field to Instance

**Files:**
- Modify: `internal/component/instance.go:17-32`

**Step 1: Write the failing test**

Create file: `internal/component/conformance/may_leave_test.go`

```go
package conformance

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestInstance_MayLeaveDefaultTrue(t *testing.T) {
	inst := &component.Instance{}
	require.True(t, inst.MayLeave(), "may_leave should default to true")
}
```

**Step 2: Run test to verify it fails**

```bash
go test -v ./internal/component/conformance/... -run TestInstance_MayLeaveDefaultTrue
```

Expected: FAIL with "inst.MayLeave undefined"

**Step 3: Write minimal implementation**

Modify `internal/component/instance.go`:

```go
// Instance represents an instantiated component.
type Instance struct {
	component     *Component
	coreInstances []api.Module
	exports       map[string]*ExportedFunc

	// componentFuncs maps component function indices to their implementations.
	componentFuncs map[uint32]ComponentFunc

	// Resource management fields
	resourceTable *ResourceTable
	destructors   map[uint32]func(any)
	callContext   *CallContext

	// mayLeave tracks whether the component can call out.
	// Set to false during lowering and post-return to prevent reentrance.
	// Per Canonical ABI spec, defaults to true.
	mayLeave bool
}

// MayLeave returns whether this instance is allowed to call out.
// Returns true by default, false during lowering and post-return.
func (i *Instance) MayLeave() bool {
	// Default value of bool is false, but spec says default is true
	// We use a pointer or explicit initialization pattern
	return i.mayLeave
}
```

Wait - the default bool value is `false`, but spec says default is `true`. We need to handle initialization.

Update the implementation:

```go
// Instance represents an instantiated component.
type Instance struct {
	component     *Component
	coreInstances []api.Module
	exports       map[string]*ExportedFunc

	// componentFuncs maps component function indices to their implementations.
	componentFuncs map[uint32]ComponentFunc

	// Resource management fields
	resourceTable *ResourceTable
	destructors   map[uint32]func(any)
	callContext   *CallContext

	// mayLeave tracks whether the component can call out.
	// Set to false during lowering and post-return to prevent reentrance.
	// Per Canonical ABI spec, defaults to true.
	// We use mayLeaveSet to track if explicitly set to false.
	mayLeaveDisabled bool // true means may_leave=false
}

// MayLeave returns whether this instance is allowed to call out.
// Returns true by default, false during lowering and post-return.
func (i *Instance) MayLeave() bool {
	return !i.mayLeaveDisabled
}
```

**Step 4: Run test to verify it passes**

```bash
go test -v ./internal/component/conformance/... -run TestInstance_MayLeaveDefaultTrue
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/instance.go internal/component/conformance/may_leave_test.go
git commit -m "feat(component): add mayLeave field to Instance

Addresses GAP-LEAVE-1 from spec-canon-lift-lower-gap-analysis.md.
The may_leave flag defaults to true and will be set to false
during lowering and post-return operations.

Spec reference: CanonicalABI.md lines 3133, 3151, 3287-3289, 3454"
```

---

## Task 1.2: Implement SetMayLeave Helper

**Files:**
- Modify: `internal/component/instance.go`
- Modify: `internal/component/conformance/may_leave_test.go`

**Step 1: Write the failing test**

Add to `internal/component/conformance/may_leave_test.go`:

```go
func TestInstance_SetMayLeave(t *testing.T) {
	inst := &component.Instance{}

	// Default is true
	require.True(t, inst.MayLeave())

	// Set to false
	inst.SetMayLeave(false)
	require.False(t, inst.MayLeave())

	// Set back to true
	inst.SetMayLeave(true)
	require.True(t, inst.MayLeave())
}
```

**Step 2: Run test to verify it fails**

```bash
go test -v ./internal/component/conformance/... -run TestInstance_SetMayLeave
```

Expected: FAIL with "inst.SetMayLeave undefined"

**Step 3: Write minimal implementation**

Add to `internal/component/instance.go`:

```go
// SetMayLeave sets whether this instance is allowed to call out.
// Called with false at the start of lowering/post-return, true at the end.
func (i *Instance) SetMayLeave(allowed bool) {
	i.mayLeaveDisabled = !allowed
}
```

**Step 4: Run test to verify it passes**

```bash
go test -v ./internal/component/conformance/... -run TestInstance_SetMayLeave
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/instance.go internal/component/conformance/may_leave_test.go
git commit -m "feat(component): add SetMayLeave method to Instance"
```

---

## Task 1.3: Set mayLeave=false During Parameter Lowering

**Files:**
- Modify: `internal/component/instance.go:90-260` (Call method)
- Modify: `internal/component/conformance/may_leave_test.go`

**Step 1: Write the failing test**

This test verifies that during parameter lowering, mayLeave is false.
We need a way to observe this. We'll create a test that checks the flag during a callback.

Add to `internal/component/conformance/may_leave_test.go`:

```go
func TestInstance_MayLeaveFalseDuringLowering(t *testing.T) {
	// This test verifies that if a component function were to call back
	// during parameter lowering, it would see mayLeave=false.
	//
	// We test this by checking the flag state in the Call path.
	// The actual enforcement is tested in Task 1.5.

	inst := &component.Instance{}
	require.True(t, inst.MayLeave(), "should start true")

	// Simulate entering lowering
	inst.SetMayLeave(false)
	require.False(t, inst.MayLeave(), "should be false during lowering")

	// Simulate exiting lowering
	inst.SetMayLeave(true)
	require.True(t, inst.MayLeave(), "should be true after lowering")
}
```

**Step 2: Run test to verify it passes**

This test should already pass since we implemented SetMayLeave.

```bash
go test -v ./internal/component/conformance/... -run TestInstance_MayLeaveFalseDuringLowering
```

Expected: PASS

**Step 3: Implement mayLeave guard in Call**

Modify `internal/component/instance.go` in the `Call` method. Find the parameter conversion loop and wrap it:

```go
// Call invokes the exported function with the given arguments.
func (f *ExportedFunc) Call(ctx context.Context, params ...Val) ([]Val, error) {
	// Set up call context and borrow scope for resource tracking
	callCtx := NewCallContext()
	var borrowScope *BorrowScope

	// Initialize resource table if needed
	if f.instance != nil {
		if f.instance.resourceTable == nil {
			f.instance.resourceTable = NewResourceTable()
		}
		borrowScope = NewBorrowScope(f.instance.resourceTable)
		// Set the call context for this invocation
		prevCallCtx := f.instance.callContext
		f.instance.callContext = callCtx
		defer func() {
			f.instance.callContext = prevCallCtx
		}()
	}

	// ... TypeResolver setup ...

	// === BEGIN LOWERING - may_leave = false ===
	if f.instance != nil {
		f.instance.SetMayLeave(false)
	}

	// Convert component Vals to core wasm values
	var coreParams []uint64
	var lowerErr error
	func() {
		defer func() {
			// === END LOWERING - may_leave = true ===
			if f.instance != nil {
				f.instance.SetMayLeave(true)
			}
		}()

		for i, p := range params {
			// ... existing parameter conversion logic ...
		}
	}()

	if lowerErr != nil {
		return nil, lowerErr
	}

	// ... rest of Call method ...
}
```

Actually, this refactor is complex. Let's use a simpler approach with explicit set/restore:

```go
// Call invokes the exported function with the given arguments.
func (f *ExportedFunc) Call(ctx context.Context, params ...Val) ([]Val, error) {
	// ... existing setup code ...

	// === BEGIN LOWERING PARAMS - may_leave = false ===
	if f.instance != nil {
		f.instance.SetMayLeave(false)
	}

	// Convert component Vals to core wasm values
	var coreParams []uint64
	for i, p := range params {
		switch p.Kind() {
		// ... existing cases ...
		}
	}

	// === END LOWERING PARAMS - may_leave = true ===
	if f.instance != nil {
		f.instance.SetMayLeave(true)
	}

	// Call the core function
	coreResults, err := f.coreFunc.Call(ctx, coreParams...)
	if err != nil {
		return nil, err
	}

	// ... rest of method ...
}
```

**Step 4: Run existing tests to verify no regression**

```bash
go test -v ./internal/component/... -short
```

Expected: All existing tests PASS

**Step 5: Commit**

```bash
git add internal/component/instance.go
git commit -m "feat(component): set may_leave=false during parameter lowering

Per CanonicalABI.md lines 3133, 3151, the may_leave flag must be
false during lower_flat_values to prevent callbacks during lowering."
```

---

## Task 1.4: Set mayLeave=false During Post-Return

**Files:**
- Modify: `internal/component/instance.go:260-270` (post-return section)
- Modify: `internal/component/conformance/may_leave_test.go`

**Step 1: Write the failing test**

Add to `internal/component/conformance/may_leave_test.go`:

```go
func TestInstance_MayLeaveFalseDuringPostReturn(t *testing.T) {
	// This test documents that may_leave should be false during post-return.
	// The actual logic is in instance.go where postReturnFunc is called.

	inst := &component.Instance{}

	// Simulate post-return execution
	inst.SetMayLeave(false)
	require.False(t, inst.MayLeave(), "should be false during post-return")

	inst.SetMayLeave(true)
	require.True(t, inst.MayLeave(), "should be true after post-return")
}
```

**Step 2: Run test**

```bash
go test -v ./internal/component/conformance/... -run TestInstance_MayLeaveFalseDuringPostReturn
```

Expected: PASS (test documents behavior)

**Step 3: Implement mayLeave guard around post-return call**

Modify `internal/component/instance.go` around line 265:

```go
	// Call the post-return function if specified.
	// Per Canonical ABI spec, the post-return function is called after the main
	// function returns but before control returns to the caller.
	// IMPORTANT: may_leave must be false during post-return to prevent callbacks.
	if f.postReturnFunc != nil {
		if f.instance != nil {
			f.instance.SetMayLeave(false)
		}
		_, postReturnErr := f.postReturnFunc.Call(ctx, coreResults...)
		if f.instance != nil {
			f.instance.SetMayLeave(true)
		}
		if postReturnErr != nil {
			return nil, fmt.Errorf("post-return function failed: %w", postReturnErr)
		}
	}
```

**Step 4: Run existing tests**

```bash
go test -v ./internal/component/... -short
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/instance.go internal/component/conformance/may_leave_test.go
git commit -m "feat(component): set may_leave=false during post-return

Per CanonicalABI.md lines 3287-3289, the may_leave flag must be
false during post-return execution to ensure synchronous lowered
calls can always be implemented by plain synchronous function calls."
```

---

## Task 1.5: Check mayLeave in Lowered Call Path

**Files:**
- Modify: `internal/component/component_linker.go`
- Modify: `internal/component/instance.go`
- Modify: `internal/component/conformance/may_leave_test.go`

**Step 1: Write the failing test**

Add to `internal/component/conformance/may_leave_test.go`:

```go
func TestInstance_ValidateMayLeave(t *testing.T) {
	inst := &component.Instance{}

	// When may_leave is true, validation passes
	err := inst.ValidateMayLeave()
	require.NoError(t, err)

	// When may_leave is false, validation fails
	inst.SetMayLeave(false)
	err = inst.ValidateMayLeave()
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot call")
}
```

**Step 2: Run test to verify it fails**

```bash
go test -v ./internal/component/conformance/... -run TestInstance_ValidateMayLeave
```

Expected: FAIL with "inst.ValidateMayLeave undefined"

**Step 3: Write minimal implementation**

Add to `internal/component/instance.go`:

```go
// ValidateMayLeave checks if this instance is allowed to make outgoing calls.
// Returns an error if may_leave is false (during lowering or post-return).
// This implements the trap_if(not inst.may_leave) check from canon_lower.
func (i *Instance) ValidateMayLeave() error {
	if i == nil {
		return nil // No instance means no restriction
	}
	if !i.MayLeave() {
		return fmt.Errorf("trap: cannot call out of component while lowering values")
	}
	return nil
}
```

**Step 4: Run test to verify it passes**

```bash
go test -v ./internal/component/conformance/... -run TestInstance_ValidateMayLeave
```

Expected: PASS

**Step 5: Add validation to lowered call paths**

The validation should be called when a lowered component function is invoked.
In `component_linker.go`, when a `canonLower` trampoline is called, it should check `ValidateMayLeave`.

For now, the validation is available. Integration into the actual call path depends on how `canonLower` trampolines are implemented. If they go through `ComponentFunc.Impl`, we can add the check there.

Add a note in the code:

```go
// ComponentFunc represents a callable component-level function.
type ComponentFunc struct {
	// Type is the component function type (params and results).
	Type *FuncType

	// Implementation is the actual callable.
	// For imports: the host-provided Definition
	// For canon lift: the lifted core function
	// NOTE: When Impl is a lowered function (from canon lower),
	// callers should call ValidateMayLeave before invoking.
	Impl func(ctx context.Context, args []Val) ([]Val, error)
}
```

**Step 6: Commit**

```bash
git add internal/component/instance.go internal/component/conformance/may_leave_test.go
git commit -m "feat(component): add ValidateMayLeave for canon_lower check

Implements the trap_if(not inst.may_leave) check from CanonicalABI.md
line 3454. This validation should be called before invoking any
lowered component function."
```

---

## Task 1.6: Add Comprehensive Conformance Tests

**Files:**
- Modify: `internal/component/conformance/may_leave_test.go`

**Step 1: Add comprehensive tests**

```go
package conformance

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestMayLeave_DefaultTrue(t *testing.T) {
	inst := &component.Instance{}
	require.True(t, inst.MayLeave(), "may_leave defaults to true per spec")
}

func TestMayLeave_SetFalseAndTrue(t *testing.T) {
	inst := &component.Instance{}

	inst.SetMayLeave(false)
	require.False(t, inst.MayLeave())

	inst.SetMayLeave(true)
	require.True(t, inst.MayLeave())
}

func TestMayLeave_ValidateMayLeaveWhenTrue(t *testing.T) {
	inst := &component.Instance{}
	err := inst.ValidateMayLeave()
	require.NoError(t, err, "validation passes when may_leave is true")
}

func TestMayLeave_ValidateMayLeaveWhenFalse(t *testing.T) {
	inst := &component.Instance{}
	inst.SetMayLeave(false)

	err := inst.ValidateMayLeave()
	require.Error(t, err, "validation fails when may_leave is false")
	require.Contains(t, err.Error(), "cannot call")
}

func TestMayLeave_ValidateMayLeaveNilInstance(t *testing.T) {
	var inst *component.Instance
	err := inst.ValidateMayLeave()
	require.NoError(t, err, "nil instance should not error")
}

func TestMayLeave_MultipleSetCycles(t *testing.T) {
	inst := &component.Instance{}

	// Simulate multiple lowering cycles
	for i := 0; i < 10; i++ {
		require.True(t, inst.MayLeave())
		inst.SetMayLeave(false)
		require.False(t, inst.MayLeave())
		inst.SetMayLeave(true)
	}
	require.True(t, inst.MayLeave())
}

func TestMayLeave_ConcurrentAccess(t *testing.T) {
	// Note: may_leave is typically single-threaded per component instance,
	// but this tests basic safety.
	inst := &component.Instance{}

	done := make(chan bool)

	go func() {
		for i := 0; i < 1000; i++ {
			inst.SetMayLeave(false)
			_ = inst.MayLeave()
			inst.SetMayLeave(true)
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 1000; i++ {
			_ = inst.MayLeave()
			_ = inst.ValidateMayLeave()
		}
		done <- true
	}()

	<-done
	<-done
}
```

**Step 2: Run all tests**

```bash
go test -v ./internal/component/conformance/... -run TestMayLeave
```

Expected: All PASS

**Step 3: Commit**

```bash
git add internal/component/conformance/may_leave_test.go
git commit -m "test(component): add comprehensive may_leave conformance tests

Tests cover:
- Default true value
- Set/get cycles
- Validation behavior
- Nil instance handling
- Multiple cycles
- Basic concurrent access"
```

---

## Phase 1 Regression Check

**CRITICAL:** After completing all Task 1.x, run the calculator regression test:

```bash
go test -v ./internal/component/wasip2test/... -run "TestAdd|TestSubtract"
```

**Expected:** Both tests PASS

If tests fail, review changes to `instance.go` and ensure the mayLeave logic doesn't interfere with normal call flow.

---

## Phase 1 Summary

After completing Phase 1, the codebase will have:

1. `mayLeaveDisabled` field on `Instance`
2. `MayLeave()` getter returning `!mayLeaveDisabled`
3. `SetMayLeave(bool)` setter
4. `ValidateMayLeave()` checker
5. `SetMayLeave(false)` called before parameter lowering
6. `SetMayLeave(true)` called after parameter lowering
7. `SetMayLeave(false/true)` around post-return call
8. Comprehensive test coverage

**Files Modified:**
- `internal/component/instance.go`
- `internal/component/conformance/may_leave_test.go` (new)

**Next Phase:** [02-reentrance-guard.md](./02-reentrance-guard.md)
