# Phase 2: Reentrance Guard Implementation

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement the `call_might_be_recursive` guard that prevents recursive calls into the same component instance.

**Architecture:** Add call tracking fields to `Instance` to detect when a call from the same component would cause reentrance. The guard traps at the entry of canon_lift if the caller is the same instance that's being called and there's already an active call.

**Tech Stack:** Go, wazero internal component APIs

**Gap References:** GAP-LIFT-1, GAP-STACK-1 from `docs/plans/spec-canon-lift-lower-gap-analysis.md`

---

## Spec Reference

From `debug-vendored/component-model/design/mvp/CanonicalABI.md`:

```python
# Lines 3237-3238 - At start of canon_lift:
def canon_lift(opts, inst, ft, callee, caller, on_start, on_resolve):
    trap_if(call_might_be_recursive(caller, inst))
    # ...

# The call_might_be_recursive function checks if:
# 1. The caller is the same component instance as the callee
# 2. There's already an active call in progress
```

From wasmtime reference (`debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/func/host.rs`):
- Uses `reentrance_flag` on component instances
- Traps if flag is already set when entering

---

## Task 2.1: Add Call Tracking Fields to Instance

**Files:**
- Modify: `internal/component/instance.go:17-35`

**Step 1: Write the failing test**

Create file: `internal/component/conformance/reentrance_test.go`

```go
package conformance

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestInstance_CallDepthTracking(t *testing.T) {
	inst := &component.Instance{}

	require.Equal(t, 0, inst.ActiveCallDepth(), "should start at 0")

	inst.EnterCall()
	require.Equal(t, 1, inst.ActiveCallDepth())

	inst.EnterCall()
	require.Equal(t, 2, inst.ActiveCallDepth())

	inst.ExitCall()
	require.Equal(t, 1, inst.ActiveCallDepth())

	inst.ExitCall()
	require.Equal(t, 0, inst.ActiveCallDepth())
}
```

**Step 2: Run test to verify it fails**

```bash
go test -v ./internal/component/conformance/... -run TestInstance_CallDepthTracking
```

Expected: FAIL with "inst.ActiveCallDepth undefined"

**Step 3: Write minimal implementation**

Add to `internal/component/instance.go`:

```go
// Instance represents an instantiated component.
type Instance struct {
	component     *Component
	coreInstances []api.Module
	exports       map[string]*ExportedFunc

	componentFuncs map[uint32]ComponentFunc

	// Resource management fields
	resourceTable *ResourceTable
	destructors   map[uint32]func(any)
	callContext   *CallContext

	// mayLeaveDisabled tracks whether the component cannot call out.
	mayLeaveDisabled bool

	// activeCallDepth tracks the number of active calls into this instance.
	// Used by call_might_be_recursive to detect reentrance.
	activeCallDepth int32
}

// ActiveCallDepth returns the number of active calls into this instance.
func (i *Instance) ActiveCallDepth() int {
	if i == nil {
		return 0
	}
	return int(i.activeCallDepth)
}

// EnterCall increments the active call depth.
// Called at the start of canon_lift.
func (i *Instance) EnterCall() {
	if i != nil {
		i.activeCallDepth++
	}
}

// ExitCall decrements the active call depth.
// Called at the end of canon_lift (including post-return).
func (i *Instance) ExitCall() {
	if i != nil && i.activeCallDepth > 0 {
		i.activeCallDepth--
	}
}
```

**Step 4: Run test to verify it passes**

```bash
go test -v ./internal/component/conformance/... -run TestInstance_CallDepthTracking
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/instance.go internal/component/conformance/reentrance_test.go
git commit -m "feat(component): add call depth tracking to Instance

Adds activeCallDepth field and EnterCall/ExitCall methods.
This is the foundation for call_might_be_recursive detection.

Addresses GAP-LIFT-1, GAP-STACK-1 from gap analysis."
```

---

## Task 2.2: Implement callMightBeRecursive Check

**Files:**
- Modify: `internal/component/instance.go`
- Modify: `internal/component/conformance/reentrance_test.go`

**Step 1: Write the failing test**

Add to `internal/component/conformance/reentrance_test.go`:

```go
func TestInstance_CallMightBeRecursive(t *testing.T) {
	callee := &component.Instance{}
	caller := &component.Instance{}
	otherCaller := &component.Instance{}

	t.Run("different_instances_no_reentrance", func(t *testing.T) {
		// Caller and callee are different - never recursive
		callee.EnterCall()
		recursive := callee.CallMightBeRecursive(caller)
		require.False(t, recursive, "different instances cannot be recursive")
		callee.ExitCall()
	})

	t.Run("same_instance_no_active_call", func(t *testing.T) {
		// Same instance but no active call - not recursive
		recursive := callee.CallMightBeRecursive(callee)
		require.False(t, recursive, "no active call means not recursive")
	})

	t.Run("same_instance_with_active_call", func(t *testing.T) {
		// Same instance with active call - RECURSIVE
		callee.EnterCall()
		recursive := callee.CallMightBeRecursive(callee)
		require.True(t, recursive, "same instance with active call is recursive")
		callee.ExitCall()
	})

	t.Run("nil_caller", func(t *testing.T) {
		// Nil caller (host call) - never recursive
		recursive := callee.CallMightBeRecursive(nil)
		require.False(t, recursive, "nil caller means host call, not recursive")
	})
}
```

**Step 2: Run test to verify it fails**

```bash
go test -v ./internal/component/conformance/... -run TestInstance_CallMightBeRecursive
```

Expected: FAIL with "inst.CallMightBeRecursive undefined"

**Step 3: Write minimal implementation**

Add to `internal/component/instance.go`:

```go
// CallMightBeRecursive checks if a call from caller into this instance might
// cause recursive reentrance. Returns true if:
// 1. caller is the same instance as this instance (self-call)
// 2. There's already an active call in this instance
//
// Per CanonicalABI.md, canon_lift must trap if this returns true.
func (i *Instance) CallMightBeRecursive(caller *Instance) bool {
	if i == nil || caller == nil {
		// Nil callee or nil caller (host) - no reentrance concern
		return false
	}

	// Check if this is a self-call with an active call already in progress
	if caller == i && i.activeCallDepth > 0 {
		return true
	}

	return false
}
```

**Step 4: Run test to verify it passes**

```bash
go test -v ./internal/component/conformance/... -run TestInstance_CallMightBeRecursive
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/instance.go internal/component/conformance/reentrance_test.go
git commit -m "feat(component): implement CallMightBeRecursive check

Detects when a call from the same component instance would cause
recursive reentrance. This check must trap per CanonicalABI.md line 3238."
```

---

## Task 2.3: Add Guard at canon_lift Entry

**Files:**
- Modify: `internal/component/instance.go` (Call method)
- Modify: `internal/component/conformance/reentrance_test.go`

**Step 1: Write the failing test**

Add to `internal/component/conformance/reentrance_test.go`:

```go
func TestInstance_ValidateNotRecursive(t *testing.T) {
	inst := &component.Instance{}

	t.Run("no_active_call_passes", func(t *testing.T) {
		err := inst.ValidateNotRecursive(inst)
		require.NoError(t, err)
	})

	t.Run("active_call_from_same_instance_fails", func(t *testing.T) {
		inst.EnterCall()
		defer inst.ExitCall()

		err := inst.ValidateNotRecursive(inst)
		require.Error(t, err)
		require.Contains(t, err.Error(), "recursive")
	})

	t.Run("active_call_from_different_instance_passes", func(t *testing.T) {
		other := &component.Instance{}
		inst.EnterCall()
		defer inst.ExitCall()

		err := inst.ValidateNotRecursive(other)
		require.NoError(t, err)
	})

	t.Run("host_call_always_passes", func(t *testing.T) {
		inst.EnterCall()
		defer inst.ExitCall()

		err := inst.ValidateNotRecursive(nil)
		require.NoError(t, err)
	})
}
```

**Step 2: Run test to verify it fails**

```bash
go test -v ./internal/component/conformance/... -run TestInstance_ValidateNotRecursive
```

Expected: FAIL with "inst.ValidateNotRecursive undefined"

**Step 3: Write minimal implementation**

Add to `internal/component/instance.go`:

```go
// ValidateNotRecursive checks if a call from caller would be recursive reentrance.
// Returns an error that should cause a trap if reentrance is detected.
// This implements the trap_if(call_might_be_recursive(caller, inst)) check.
func (i *Instance) ValidateNotRecursive(caller *Instance) error {
	if i.CallMightBeRecursive(caller) {
		return fmt.Errorf("trap: recursive call into same component instance")
	}
	return nil
}
```

**Step 4: Run test to verify it passes**

```bash
go test -v ./internal/component/conformance/... -run TestInstance_ValidateNotRecursive
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/instance.go internal/component/conformance/reentrance_test.go
git commit -m "feat(component): add ValidateNotRecursive check

Returns error when call_might_be_recursive would return true.
This error should be treated as a trap."
```

---

## Task 2.4: Integrate Reentrance Check into Call Path

**Files:**
- Modify: `internal/component/instance.go` (Call method)

**Step 1: Identify integration point**

The reentrance check must happen at the very start of `ExportedFunc.Call`, before any lowering. We also need to track the caller context.

**Step 2: Add caller tracking to ExportedFunc**

The challenge is that `ExportedFunc.Call` doesn't know who the caller is. In the current implementation, calls come from either:
1. The host (Go code) - caller is nil/external
2. Another component function - caller is that component's instance

For now, we'll assume host calls (caller = nil) and add the reentrance check for self-calls only.

Modify `internal/component/instance.go` at the start of the `Call` method:

```go
// Call invokes the exported function with the given arguments.
func (f *ExportedFunc) Call(ctx context.Context, params ...Val) ([]Val, error) {
	// === REENTRANCE CHECK ===
	// For now, we check if this instance has an active call and reject self-calls.
	// In a full implementation, we'd track the caller instance through the context.
	// Host calls (caller=nil) are always allowed.
	//
	// Note: This is a simplified check. The full spec checks if the caller
	// is the same component instance, which requires caller tracking.
	if f.instance != nil {
		// Get caller from context if available
		caller := GetCallerInstance(ctx)
		if err := f.instance.ValidateNotRecursive(caller); err != nil {
			return nil, err
		}

		// Track this call
		f.instance.EnterCall()
		defer f.instance.ExitCall()
	}

	// ... rest of existing Call implementation ...
}
```

Add helper function:

```go
// callerInstanceKey is the context key for the caller instance.
type callerInstanceKey struct{}

// GetCallerInstance retrieves the caller instance from context.
// Returns nil if called from host (no caller in context).
func GetCallerInstance(ctx context.Context) *Instance {
	if caller, ok := ctx.Value(callerInstanceKey{}).(*Instance); ok {
		return caller
	}
	return nil
}

// WithCallerInstance returns a context with the caller instance set.
// Used when a component calls another component.
func WithCallerInstance(ctx context.Context, caller *Instance) context.Context {
	return context.WithValue(ctx, callerInstanceKey{}, caller)
}
```

**Step 3: Run existing tests**

```bash
go test -v ./internal/component/... -short
```

Expected: PASS (existing tests are host calls with nil caller)

**Step 4: Commit**

```bash
git add internal/component/instance.go
git commit -m "feat(component): integrate reentrance check into Call path

- Check ValidateNotRecursive at Call entry
- Track call depth with EnterCall/ExitCall
- Add context helpers for caller instance tracking

This implements the trap_if(call_might_be_recursive(...)) from
CanonicalABI.md line 3238."
```

---

## Task 2.5: Add Comprehensive Conformance Tests

**Files:**
- Modify: `internal/component/conformance/reentrance_test.go`

**Step 1: Add comprehensive tests**

```go
package conformance

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestReentrance_CallDepthTracking(t *testing.T) {
	inst := &component.Instance{}

	require.Equal(t, 0, inst.ActiveCallDepth())

	inst.EnterCall()
	require.Equal(t, 1, inst.ActiveCallDepth())

	inst.EnterCall()
	require.Equal(t, 2, inst.ActiveCallDepth())

	inst.ExitCall()
	require.Equal(t, 1, inst.ActiveCallDepth())

	inst.ExitCall()
	require.Equal(t, 0, inst.ActiveCallDepth())
}

func TestReentrance_CallMightBeRecursive(t *testing.T) {
	callee := &component.Instance{}

	t.Run("different_instances_not_recursive", func(t *testing.T) {
		caller := &component.Instance{}
		callee.EnterCall()
		defer callee.ExitCall()

		require.False(t, callee.CallMightBeRecursive(caller))
	})

	t.Run("same_instance_no_call_not_recursive", func(t *testing.T) {
		require.False(t, callee.CallMightBeRecursive(callee))
	})

	t.Run("same_instance_active_call_is_recursive", func(t *testing.T) {
		callee.EnterCall()
		defer callee.ExitCall()

		require.True(t, callee.CallMightBeRecursive(callee))
	})

	t.Run("nil_caller_not_recursive", func(t *testing.T) {
		callee.EnterCall()
		defer callee.ExitCall()

		require.False(t, callee.CallMightBeRecursive(nil))
	})

	t.Run("nil_callee_not_recursive", func(t *testing.T) {
		var nilInst *component.Instance
		require.False(t, nilInst.CallMightBeRecursive(callee))
	})
}

func TestReentrance_ValidateNotRecursive(t *testing.T) {
	inst := &component.Instance{}

	t.Run("host_call_allowed", func(t *testing.T) {
		inst.EnterCall()
		defer inst.ExitCall()

		err := inst.ValidateNotRecursive(nil)
		require.NoError(t, err, "host calls always allowed")
	})

	t.Run("cross_component_call_allowed", func(t *testing.T) {
		other := &component.Instance{}
		inst.EnterCall()
		defer inst.ExitCall()

		err := inst.ValidateNotRecursive(other)
		require.NoError(t, err, "calls from different component allowed")
	})

	t.Run("self_call_no_active_allowed", func(t *testing.T) {
		err := inst.ValidateNotRecursive(inst)
		require.NoError(t, err, "first self-call allowed")
	})

	t.Run("self_call_active_trapped", func(t *testing.T) {
		inst.EnterCall()
		defer inst.ExitCall()

		err := inst.ValidateNotRecursive(inst)
		require.Error(t, err, "recursive self-call trapped")
		require.Contains(t, err.Error(), "recursive")
	})
}

func TestReentrance_CallerInstanceContext(t *testing.T) {
	ctx := context.Background()

	t.Run("no_caller_returns_nil", func(t *testing.T) {
		caller := component.GetCallerInstance(ctx)
		require.Nil(t, caller)
	})

	t.Run("with_caller_returns_instance", func(t *testing.T) {
		inst := &component.Instance{}
		ctxWithCaller := component.WithCallerInstance(ctx, inst)

		caller := component.GetCallerInstance(ctxWithCaller)
		require.Same(t, inst, caller)
	})

	t.Run("nested_contexts", func(t *testing.T) {
		inst1 := &component.Instance{}
		inst2 := &component.Instance{}

		ctx1 := component.WithCallerInstance(ctx, inst1)
		ctx2 := component.WithCallerInstance(ctx1, inst2)

		// Most recent caller wins
		require.Same(t, inst2, component.GetCallerInstance(ctx2))
		// Original context still has inst1
		require.Same(t, inst1, component.GetCallerInstance(ctx1))
	})
}

func TestReentrance_DeepCallStack(t *testing.T) {
	inst := &component.Instance{}

	// Simulate deep call stack
	depth := 100
	for i := 0; i < depth; i++ {
		inst.EnterCall()
	}
	require.Equal(t, depth, inst.ActiveCallDepth())

	for i := 0; i < depth; i++ {
		inst.ExitCall()
	}
	require.Equal(t, 0, inst.ActiveCallDepth())
}

func TestReentrance_ExitCallNeverNegative(t *testing.T) {
	inst := &component.Instance{}

	// Exit without enter should not go negative
	inst.ExitCall()
	inst.ExitCall()
	inst.ExitCall()

	require.Equal(t, 0, inst.ActiveCallDepth(), "call depth should never go negative")
}
```

**Step 2: Run all tests**

```bash
go test -v ./internal/component/conformance/... -run TestReentrance
```

Expected: All PASS

**Step 3: Commit**

```bash
git add internal/component/conformance/reentrance_test.go
git commit -m "test(component): add comprehensive reentrance conformance tests

Tests cover:
- Call depth tracking
- call_might_be_recursive logic
- ValidateNotRecursive behavior
- Caller instance context
- Deep call stacks
- Edge cases"
```

---

## Phase 2 Regression Check

**CRITICAL:** After completing all Task 2.x, run the calculator regression test:

```bash
go test -v ./internal/component/wasip2test/... -run "TestAdd|TestSubtract"
```

**Expected:** Both tests PASS

If tests fail:
1. Check that `EnterCall`/`ExitCall` are balanced with `defer`
2. Verify `GetCallerInstance` returns nil for host calls (existing tests)
3. Ensure `ValidateNotRecursive(nil)` returns no error

---

## Phase 2 Summary

After completing Phase 2, the codebase will have:

1. `activeCallDepth` field on `Instance`
2. `ActiveCallDepth()` getter
3. `EnterCall()` / `ExitCall()` methods
4. `CallMightBeRecursive(caller)` detection
5. `ValidateNotRecursive(caller)` validation
6. `GetCallerInstance(ctx)` / `WithCallerInstance(ctx, inst)` context helpers
7. Reentrance check at `Call` entry with `EnterCall`/`ExitCall` tracking
8. Comprehensive test coverage

**Files Modified:**
- `internal/component/instance.go`
- `internal/component/conformance/reentrance_test.go` (new)

**Next Phase:** [03-subtask-management.md](./03-subtask-management.md)
