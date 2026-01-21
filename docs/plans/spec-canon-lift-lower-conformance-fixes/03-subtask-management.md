# Phase 3: Subtask Management Implementation

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement the `Subtask` struct for proper borrow scope tracking during canon_lower calls.

**Architecture:** Create a `Subtask` type that tracks borrow lifetimes for a single lowered call. Each canon_lower creates a Subtask that owns a borrow scope. When the Subtask completes, borrows are released. This ensures borrowed handles are properly tracked across component boundaries.

**Tech Stack:** Go, wazero internal component APIs

**Gap References:** GAP-LOWER-2, GAP-CTX-2 from `docs/plans/spec-canon-lift-lower-gap-analysis.md`

**Prerequisites:** Phase 1 (may_leave flag) should be completed first.

---

## Spec Reference

From `debug-vendored/component-model/design/mvp/CanonicalABI.md`:

```python
# Lines 3468-3471 - Subtask creation in canon_lower:
def canon_lower(opts, ft, callee, thread, flat_args):
    trap_if(not thread.task.inst.may_leave)
    subtask = Subtask()
    cx = LiftLowerContext(opts, thread.task.inst, subtask)
    # ... perform the call ...
    subtask.deliver_resolve(result)
    subtask.finish()

# Subtask class tracks:
# - borrow_scope: Scope for tracking borrowed handles during the call
# - state: pending/resolved/finishing/done
```

---

## Task 3.1: Define Subtask Struct

**Files:**
- Create: `internal/component/subtask.go`

**Step 1: Write the failing test**

Create file: `internal/component/conformance/subtask_test.go`

```go
package conformance

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestSubtask_Creation(t *testing.T) {
	rt := component.NewResourceTable()
	subtask := component.NewSubtask(rt)

	require.NotNil(t, subtask)
	require.NotNil(t, subtask.BorrowScope())
	require.Equal(t, component.SubtaskStatePending, subtask.State())
}
```

**Step 2: Run test to verify it fails**

```bash
go test -v ./internal/component/conformance/... -run TestSubtask_Creation
```

Expected: FAIL with "component.NewSubtask undefined"

**Step 3: Write minimal implementation**

Create file `internal/component/subtask.go`:

```go
package component

// SubtaskState represents the state of a subtask.
type SubtaskState int

const (
	// SubtaskStatePending is the initial state before the call completes.
	SubtaskStatePending SubtaskState = iota
	// SubtaskStateResolved is set when the call has returned a value.
	SubtaskStateResolved
	// SubtaskStateFinishing is set during cleanup.
	SubtaskStateFinishing
	// SubtaskStateDone is set when the subtask is fully complete.
	SubtaskStateDone
)

// Subtask tracks the lifetime of a single canon_lower call.
// It owns a borrow scope for tracking borrowed handles during the call.
type Subtask struct {
	borrowScope *BorrowScope
	state       SubtaskState
	result      []Val // Stored result after resolve
}

// NewSubtask creates a new Subtask with its own borrow scope.
// The borrow scope is used to track borrowed handles during the call.
func NewSubtask(resourceTable *ResourceTable) *Subtask {
	return &Subtask{
		borrowScope: NewBorrowScope(resourceTable),
		state:       SubtaskStatePending,
	}
}

// BorrowScope returns the borrow scope for this subtask.
func (s *Subtask) BorrowScope() *BorrowScope {
	return s.borrowScope
}

// State returns the current state of this subtask.
func (s *Subtask) State() SubtaskState {
	return s.state
}
```

**Step 4: Run test to verify it passes**

```bash
go test -v ./internal/component/conformance/... -run TestSubtask_Creation
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/subtask.go internal/component/conformance/subtask_test.go
git commit -m "feat(component): add Subtask struct for canon_lower tracking

Subtask owns a borrow scope and tracks call state.
Per CanonicalABI.md, each canon_lower creates a Subtask.

Addresses GAP-LOWER-2 from gap analysis."
```

---

## Task 3.2: Implement Subtask State Transitions

**Files:**
- Modify: `internal/component/subtask.go`
- Modify: `internal/component/conformance/subtask_test.go`

**Step 1: Write the failing test**

Add to `internal/component/conformance/subtask_test.go`:

```go
func TestSubtask_StateTransitions(t *testing.T) {
	rt := component.NewResourceTable()
	subtask := component.NewSubtask(rt)

	t.Run("pending_to_resolved", func(t *testing.T) {
		result := []component.Val{component.ValU32(42)}
		err := subtask.DeliverResolve(result)
		require.NoError(t, err)
		require.Equal(t, component.SubtaskStateResolved, subtask.State())
	})

	t.Run("resolved_to_finishing", func(t *testing.T) {
		err := subtask.StartFinish()
		require.NoError(t, err)
		require.Equal(t, component.SubtaskStateFinishing, subtask.State())
	})

	t.Run("finishing_to_done", func(t *testing.T) {
		err := subtask.Finish()
		require.NoError(t, err)
		require.Equal(t, component.SubtaskStateDone, subtask.State())
	})
}

func TestSubtask_InvalidTransitions(t *testing.T) {
	rt := component.NewResourceTable()

	t.Run("cannot_resolve_twice", func(t *testing.T) {
		subtask := component.NewSubtask(rt)
		_ = subtask.DeliverResolve(nil)

		err := subtask.DeliverResolve(nil)
		require.Error(t, err, "should not resolve twice")
	})

	t.Run("cannot_finish_before_resolve", func(t *testing.T) {
		subtask := component.NewSubtask(rt)

		err := subtask.Finish()
		require.Error(t, err, "should not finish before resolve")
	})
}
```

**Step 2: Run test to verify it fails**

```bash
go test -v ./internal/component/conformance/... -run TestSubtask_State
```

Expected: FAIL with "subtask.DeliverResolve undefined"

**Step 3: Write minimal implementation**

Add to `internal/component/subtask.go`:

```go
import "fmt"

// DeliverResolve transitions the subtask from pending to resolved with a result.
// This is called when the lowered function returns.
func (s *Subtask) DeliverResolve(result []Val) error {
	if s.state != SubtaskStatePending {
		return fmt.Errorf("subtask: cannot resolve in state %d", s.state)
	}
	s.result = result
	s.state = SubtaskStateResolved
	return nil
}

// StartFinish transitions from resolved to finishing.
// This begins the cleanup phase.
func (s *Subtask) StartFinish() error {
	if s.state != SubtaskStateResolved {
		return fmt.Errorf("subtask: cannot start finish in state %d", s.state)
	}
	s.state = SubtaskStateFinishing
	return nil
}

// Finish transitions from finishing to done.
// This completes the subtask and releases the borrow scope.
func (s *Subtask) Finish() error {
	if s.state == SubtaskStatePending {
		return fmt.Errorf("subtask: cannot finish before resolve")
	}
	if s.state == SubtaskStateDone {
		return fmt.Errorf("subtask: already done")
	}

	// Release borrows
	if s.borrowScope != nil {
		if err := s.borrowScope.Release(); err != nil {
			return fmt.Errorf("subtask: release borrows: %w", err)
		}
	}

	s.state = SubtaskStateDone
	return nil
}

// Result returns the stored result after resolution.
// Returns nil if not yet resolved.
func (s *Subtask) Result() []Val {
	return s.result
}
```

**Step 4: Run test to verify it passes**

```bash
go test -v ./internal/component/conformance/... -run TestSubtask_State
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/subtask.go internal/component/conformance/subtask_test.go
git commit -m "feat(component): implement Subtask state transitions

State machine: pending -> resolved -> finishing -> done
- DeliverResolve stores result and transitions to resolved
- StartFinish begins cleanup phase
- Finish releases borrows and completes subtask"
```

---

## Task 3.3: Track Lends in Subtask

**Files:**
- Modify: `internal/component/subtask.go`
- Modify: `internal/component/conformance/subtask_test.go`

**Step 1: Write the failing test**

Add to `internal/component/conformance/subtask_test.go`:

```go
func TestSubtask_LendTracking(t *testing.T) {
	rt := component.NewResourceTable()

	// Create a resource to borrow
	handle := rt.New("test-resource", true)

	subtask := component.NewSubtask(rt)

	t.Run("track_lend", func(t *testing.T) {
		err := subtask.TrackLend(handle)
		require.NoError(t, err)
		require.Equal(t, 1, subtask.LendCount())
	})

	t.Run("multiple_lends", func(t *testing.T) {
		handle2 := rt.New("test-resource-2", true)
		err := subtask.TrackLend(handle2)
		require.NoError(t, err)
		require.Equal(t, 2, subtask.LendCount())
	})

	t.Run("finish_releases_lends", func(t *testing.T) {
		subtask.DeliverResolve(nil)
		subtask.StartFinish()
		err := subtask.Finish()
		require.NoError(t, err)
		require.Equal(t, component.SubtaskStateDone, subtask.State())
	})
}
```

**Step 2: Run test to verify it fails**

```bash
go test -v ./internal/component/conformance/... -run TestSubtask_LendTracking
```

Expected: FAIL with "subtask.TrackLend undefined"

**Step 3: Write minimal implementation**

Add to `internal/component/subtask.go`:

```go
// TrackLend records that a handle has been lent (borrowed) during this subtask.
// The lend will be released when the subtask finishes.
func (s *Subtask) TrackLend(handle Handle) error {
	if s.borrowScope == nil {
		return fmt.Errorf("subtask: no borrow scope")
	}
	return s.borrowScope.AddLender(handle)
}

// LendCount returns the number of active lends in this subtask.
func (s *Subtask) LendCount() int {
	if s.borrowScope == nil {
		return 0
	}
	return s.borrowScope.LendCount()
}
```

Also need to add `LendCount()` to `BorrowScope` if not present. Check `internal/component/resources.go`:

```go
// LendCount returns the number of active lends in this scope.
func (b *BorrowScope) LendCount() int {
	return len(b.lenders)
}
```

**Step 4: Run test to verify it passes**

```bash
go test -v ./internal/component/conformance/... -run TestSubtask_LendTracking
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/subtask.go internal/component/resources.go internal/component/conformance/subtask_test.go
git commit -m "feat(component): add lend tracking to Subtask

TrackLend records borrowed handles during a call.
LendCount exposes the number of active borrows.
Finish releases all tracked lends."
```

---

## Task 3.4: Add Subtask to LowerContext

**Files:**
- Modify: `internal/component/abi/context.go`
- Modify: `internal/component/abi/context_test.go`

**Step 1: Write the failing test**

Add to `internal/component/abi/context_test.go`:

```go
func TestLowerContext_WithSubtask(t *testing.T) {
	mem := newMockMemory(1024)
	rt := component.NewResourceTable()
	subtask := component.NewSubtask(rt)

	ctx := &LowerContext{
		Memory:  mem,
		Opts:    &Options{StringEncoding: StringEncodingUTF8},
		Subtask: subtask,
	}

	require.Same(t, subtask, ctx.Subtask)
	require.Same(t, subtask.BorrowScope(), ctx.BorrowScope())
}
```

**Step 2: Run test to verify it fails**

```bash
go test -v ./internal/component/abi/... -run TestLowerContext_WithSubtask
```

Expected: FAIL (Subtask field may not exist or BorrowScope method)

**Step 3: Write minimal implementation**

Modify `internal/component/abi/context.go`:

```go
import "github.com/tetratelabs/wazero/internal/component"

// LowerContext provides context for lowering component values to core wasm.
type LowerContext struct {
	Memory  Memory
	Opts    *Options
	Realloc func(oldPtr, oldSize, align, newSize uint32) (uint32, error)

	// Subtask is the subtask tracking this lowered call.
	// Used for borrow tracking during the call.
	Subtask *component.Subtask
}

// BorrowScope returns the borrow scope from the subtask, or nil if no subtask.
func (c *LowerContext) BorrowScope() *component.BorrowScope {
	if c.Subtask == nil {
		return nil
	}
	return c.Subtask.BorrowScope()
}
```

**Step 4: Run test to verify it passes**

```bash
go test -v ./internal/component/abi/... -run TestLowerContext_WithSubtask
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/abi/context.go internal/component/abi/context_test.go
git commit -m "feat(component): add Subtask field to LowerContext

LowerContext now has optional Subtask for borrow tracking.
BorrowScope() helper provides access to the subtask's scope.

Addresses GAP-CTX-2 from gap analysis."
```

---

## Task 3.5: Create Subtask in Lowered Call Path

**Files:**
- Modify: `internal/component/instance.go`

**Step 1: Identify integration point**

In `ExportedFunc.Call`, we already set up a `BorrowScope`. We should create a `Subtask` instead and use its borrow scope.

**Step 2: Modify Call to use Subtask**

This is an architectural change. For now, document where Subtask should be created.

In a canon_lower trampoline (when a component function calls an import):

```go
// When a lowered function is called:
func callLoweredFunction(ctx context.Context, inst *Instance, compFunc ComponentFunc, args []Val) ([]Val, error) {
	// Create subtask for this call
	subtask := NewSubtask(inst.resourceTable)

	// Create lower context with subtask
	lowerCtx := &abi.LowerContext{
		Memory:  inst.memory,
		Opts:    compFunc.Options,
		Subtask: subtask,
	}

	// Lower arguments, call implementation, lift results
	// ...

	// Mark call complete
	subtask.DeliverResolve(results)
	subtask.StartFinish()
	subtask.Finish()

	return results, nil
}
```

For the current implementation where `ExportedFunc.Call` is called from host:

```go
// In ExportedFunc.Call, replace BorrowScope with Subtask:
func (f *ExportedFunc) Call(ctx context.Context, params ...Val) ([]Val, error) {
	// ... validation ...

	var subtask *Subtask
	if f.instance != nil {
		if f.instance.resourceTable == nil {
			f.instance.resourceTable = NewResourceTable()
		}
		subtask = NewSubtask(f.instance.resourceTable)
		// Use subtask's borrow scope instead of creating one directly
		borrowScope = subtask.BorrowScope()
		// ...
	}

	// ... perform call ...

	// Complete subtask
	if subtask != nil {
		subtask.DeliverResolve(results)
		subtask.StartFinish()
		if err := subtask.Finish(); err != nil {
			return nil, fmt.Errorf("subtask finish: %w", err)
		}
	}

	return results, nil
}
```

**Step 3: Run existing tests**

```bash
go test -v ./internal/component/... -short
```

Expected: PASS

**Step 4: Commit**

```bash
git add internal/component/instance.go
git commit -m "feat(component): use Subtask for borrow tracking in Call

Replace direct BorrowScope with Subtask in ExportedFunc.Call.
Subtask manages the full lifecycle including state transitions."
```

---

## Task 3.6: Add Comprehensive Conformance Tests

**Files:**
- Modify: `internal/component/conformance/subtask_test.go`

**Step 1: Add comprehensive tests**

```go
package conformance

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestSubtask_FullLifecycle(t *testing.T) {
	rt := component.NewResourceTable()

	t.Run("simple_call", func(t *testing.T) {
		subtask := component.NewSubtask(rt)

		// Pending state
		require.Equal(t, component.SubtaskStatePending, subtask.State())

		// Call completes
		result := []component.Val{component.ValU32(42)}
		err := subtask.DeliverResolve(result)
		require.NoError(t, err)
		require.Equal(t, component.SubtaskStateResolved, subtask.State())
		require.Equal(t, result, subtask.Result())

		// Cleanup
		err = subtask.StartFinish()
		require.NoError(t, err)
		require.Equal(t, component.SubtaskStateFinishing, subtask.State())

		err = subtask.Finish()
		require.NoError(t, err)
		require.Equal(t, component.SubtaskStateDone, subtask.State())
	})

	t.Run("call_with_borrows", func(t *testing.T) {
		subtask := component.NewSubtask(rt)

		// Create resources and track lends
		h1 := rt.New("resource-1", true)
		h2 := rt.New("resource-2", true)

		err := subtask.TrackLend(h1)
		require.NoError(t, err)
		err = subtask.TrackLend(h2)
		require.NoError(t, err)

		require.Equal(t, 2, subtask.LendCount())

		// Complete call
		subtask.DeliverResolve(nil)
		subtask.StartFinish()
		err = subtask.Finish()
		require.NoError(t, err)

		// Lends should be released
		require.Equal(t, component.SubtaskStateDone, subtask.State())
	})
}

func TestSubtask_NilResult(t *testing.T) {
	rt := component.NewResourceTable()
	subtask := component.NewSubtask(rt)

	err := subtask.DeliverResolve(nil)
	require.NoError(t, err)
	require.Nil(t, subtask.Result())
}

func TestSubtask_EmptyResult(t *testing.T) {
	rt := component.NewResourceTable()
	subtask := component.NewSubtask(rt)

	err := subtask.DeliverResolve([]component.Val{})
	require.NoError(t, err)
	require.Equal(t, 0, len(subtask.Result()))
}

func TestSubtask_MultipleResults(t *testing.T) {
	rt := component.NewResourceTable()
	subtask := component.NewSubtask(rt)

	results := []component.Val{
		component.ValU32(1),
		component.ValU32(2),
		component.ValU32(3),
	}
	err := subtask.DeliverResolve(results)
	require.NoError(t, err)
	require.Equal(t, 3, len(subtask.Result()))
}

func TestSubtask_StateErrors(t *testing.T) {
	rt := component.NewResourceTable()

	t.Run("double_resolve", func(t *testing.T) {
		subtask := component.NewSubtask(rt)
		_ = subtask.DeliverResolve(nil)

		err := subtask.DeliverResolve(nil)
		require.Error(t, err)
	})

	t.Run("finish_without_resolve", func(t *testing.T) {
		subtask := component.NewSubtask(rt)

		err := subtask.Finish()
		require.Error(t, err)
	})

	t.Run("start_finish_twice", func(t *testing.T) {
		subtask := component.NewSubtask(rt)
		_ = subtask.DeliverResolve(nil)
		_ = subtask.StartFinish()

		err := subtask.StartFinish()
		require.Error(t, err)
	})

	t.Run("double_finish", func(t *testing.T) {
		subtask := component.NewSubtask(rt)
		_ = subtask.DeliverResolve(nil)
		_ = subtask.StartFinish()
		_ = subtask.Finish()

		err := subtask.Finish()
		require.Error(t, err)
	})
}

func TestSubtask_NilResourceTable(t *testing.T) {
	// Subtask with nil resource table should still work
	subtask := component.NewSubtask(nil)
	require.NotNil(t, subtask)

	// But borrow scope will be nil
	// This is acceptable for calls that don't involve resources
}
```

**Step 2: Run all tests**

```bash
go test -v ./internal/component/conformance/... -run TestSubtask
```

Expected: All PASS

**Step 3: Commit**

```bash
git add internal/component/conformance/subtask_test.go
git commit -m "test(component): add comprehensive Subtask conformance tests

Tests cover:
- Full lifecycle (pending -> resolved -> finishing -> done)
- Borrow tracking through lends
- Result handling (nil, empty, multiple)
- State transition errors
- Edge cases"
```

---

## Phase 3 Regression Check

**CRITICAL:** After completing all Task 3.x, run the calculator regression test:

```bash
go test -v ./internal/component/wasip2test/... -run "TestAdd|TestSubtract"
```

**Expected:** Both tests PASS

If tests fail:
1. Check that Subtask state transitions don't break existing BorrowScope usage
2. Verify backward compatibility with existing Call implementation
3. Ensure Subtask.Finish() properly releases borrows

---

## Phase 3 Summary

After completing Phase 3, the codebase will have:

1. `Subtask` struct with state machine
2. `SubtaskState` enum (Pending, Resolved, Finishing, Done)
3. `NewSubtask(rt)` constructor
4. State transition methods: `DeliverResolve`, `StartFinish`, `Finish`
5. `TrackLend` / `LendCount` for borrow tracking
6. `Subtask` field in `LowerContext`
7. `ExportedFunc.Call` using Subtask
8. Comprehensive test coverage

**Files Modified:**
- `internal/component/subtask.go` (new)
- `internal/component/resources.go` (LendCount)
- `internal/component/abi/context.go` (Subtask field)
- `internal/component/instance.go` (use Subtask in Call)
- `internal/component/conformance/subtask_test.go` (new)

**Next Phase:** [04-parameter-spilling.md](./04-parameter-spilling.md)
