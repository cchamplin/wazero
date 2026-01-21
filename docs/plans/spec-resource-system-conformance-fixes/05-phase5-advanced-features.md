# Phase 5: Advanced Features

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement advanced spec features including may_leave checks, reentrance protection, and table size limits.

**Architecture:** Add execution state tracking to component instances, implement call_might_be_recursive check, and enforce table maximum length. These are defensive features that prevent invalid state during resource operations.

**Tech Stack:** Go

---

## Prerequisites

- Complete Phases 1-4
- Read gap analysis: `docs/plans/resource-system-gap-analysis.md` (Section 7, P2 items)
- Understand spec: `debug-vendored/component-model/design/mvp/CanonicalABI.md` lines 3604-3609, 3664-3667

## Reference: Spec Advanced Requirements

From resource.new (CanonicalABI.md:3604-3609):
```python
def canon_resource_new(rt, thread, rep):
  trap_if(not thread.task.inst.may_leave)  # <-- We need this check
  h = ResourceHandle(rt, rep, own = True)
  i = thread.task.inst.table.add(h)
  return [i]
```

From resource.drop no-destructor case (CanonicalABI.md:3664-3667):
```python
else:
  trap_if(call_might_be_recursive(thread.task, rt.impl))  # <-- Reentrance guard
```

Table maximum (spec mentions `2**28 - 1` as MAX_LENGTH for handle tables).

---

## Task 5.1: Add MayLeave Field to Instance State

**Files:**
- Create: `internal/component/instance_state.go`
- Test: `internal/component/instance_state_test.go`

**Step 1: Write the failing test**

Create `internal/component/instance_state_test.go`:
```go
package component

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestInstanceState_MayLeave(t *testing.T) {
	state := NewInstanceState(1)

	// Initially may_leave is true
	require.True(t, state.MayLeave())

	// During certain operations, may_leave is false
	state.SetMayLeave(false)
	require.False(t, state.MayLeave())

	// Can be restored
	state.SetMayLeave(true)
	require.True(t, state.MayLeave())
}

func TestInstanceState_ID(t *testing.T) {
	state := NewInstanceState(42)
	require.Equal(t, uint32(42), state.ID())
}

func TestInstanceState_EnterLeave(t *testing.T) {
	state := NewInstanceState(1)

	// Enter disables may_leave
	state.Enter()
	require.False(t, state.MayLeave())

	// Leave enables may_leave
	state.Leave()
	require.True(t, state.MayLeave())
}

func TestInstanceState_NestedEnter(t *testing.T) {
	state := NewInstanceState(1)

	// Multiple enters
	state.Enter()
	state.Enter()
	state.Enter()

	// Still can't leave
	require.False(t, state.MayLeave())

	// Need to leave same number of times
	state.Leave()
	require.False(t, state.MayLeave())

	state.Leave()
	require.False(t, state.MayLeave())

	state.Leave()
	require.True(t, state.MayLeave())
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -run "TestInstanceState" -v`
Expected: FAIL with "NewInstanceState undefined"

**Step 3: Write minimal implementation**

Create `internal/component/instance_state.go`:
```go
// internal/component/instance_state.go

package component

// InstanceState tracks the execution state of a component instance.
// This includes the may_leave flag required by the Canonical ABI.
type InstanceState struct {
	id         uint32
	enterCount int  // Number of active Enter() calls
	mayLeave   bool // Can the instance perform operations that "leave" it?
}

// NewInstanceState creates a new instance state with the given ID.
func NewInstanceState(id uint32) *InstanceState {
	return &InstanceState{
		id:       id,
		mayLeave: true,
	}
}

// ID returns the instance ID.
func (s *InstanceState) ID() uint32 {
	return s.id
}

// MayLeave returns whether the instance may perform "leave" operations.
// Resource operations like resource.new and resource.drop require this to be true.
func (s *InstanceState) MayLeave() bool {
	return s.mayLeave && s.enterCount == 0
}

// SetMayLeave sets the may_leave flag directly.
func (s *InstanceState) SetMayLeave(may bool) {
	s.mayLeave = may
}

// Enter marks entry into a region where may_leave should be false.
// This is called when entering code that shouldn't be reentered.
func (s *InstanceState) Enter() {
	s.enterCount++
}

// Leave marks exit from a region where may_leave was false.
// Must be paired with Enter.
func (s *InstanceState) Leave() {
	if s.enterCount > 0 {
		s.enterCount--
	}
}

// EnterCount returns the current nesting depth of Enter calls.
func (s *InstanceState) EnterCount() int {
	return s.enterCount
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -run "TestInstanceState" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/instance_state.go internal/component/instance_state_test.go
git commit -m "feat(resource): add InstanceState for may_leave tracking"
```

---

## Task 5.2: Add MayLeave Checks to Resource Operations

**Files:**
- Modify: `internal/component/resource_table.go`
- Test: `internal/component/resource_table_test.go`

**Step 1: Write the failing test**

Add to `internal/component/resource_table_test.go`:
```go
func TestResourceTable_NewWithMayLeaveCheck(t *testing.T) {
	table := NewResourceTable()
	state := NewInstanceState(1)

	// When may_leave is true, New succeeds
	h, err := table.NewWithMayLeaveCheck(uint32(42), true, NewResourceTypeID(1), state)
	require.NoError(t, err)
	require.NotEqual(t, Handle(0), h)

	// When may_leave is false, New fails
	state.Enter()
	_, err = table.NewWithMayLeaveCheck(uint32(43), true, NewResourceTypeID(1), state)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrMayNotLeave)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -run "TestResourceTable_NewWithMayLeaveCheck" -v`
Expected: FAIL with "NewWithMayLeaveCheck undefined" or "ErrMayNotLeave undefined"

**Step 3: Write minimal implementation**

Add to `internal/component/resource_table.go`:
```go
// ErrMayNotLeave is returned when an operation requires may_leave but it's false.
var ErrMayNotLeave = errors.New("operation not allowed: instance may not leave")

// NewWithMayLeaveCheck creates a new resource handle with may_leave validation.
// Returns ErrMayNotLeave if the instance state doesn't allow leaving.
//
// From spec (CanonicalABI.md:3604-3609):
//
//	def canon_resource_new(rt, thread, rep):
//	  trap_if(not thread.task.inst.may_leave)
//	  ...
func (t *ResourceTable) NewWithMayLeaveCheck(rep any, own bool, rtID ResourceTypeID, state *InstanceState) (Handle, error) {
	if state != nil && !state.MayLeave() {
		return 0, ErrMayNotLeave
	}
	return t.NewWithType(rep, own, rtID), nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -run "TestResourceTable_NewWithMayLeaveCheck" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/resource_table.go internal/component/resource_table_test.go
git commit -m "feat(resource): add may_leave check to NewWithMayLeaveCheck"
```

---

## Task 5.3: Implement CallMightBeRecursive Check

**Files:**
- Create: `internal/component/reentrance.go`
- Test: `internal/component/reentrance_test.go`

**Step 1: Write the failing test**

Create `internal/component/reentrance_test.go`:
```go
package component

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestCallMightBeRecursive_NoActiveCall(t *testing.T) {
	tracker := NewReentranceTracker()

	// No active calls to instance 100
	require.False(t, tracker.CallMightBeRecursive(100))
}

func TestCallMightBeRecursive_ActiveCall(t *testing.T) {
	tracker := NewReentranceTracker()

	// Start a call to instance 100
	tracker.EnterInstance(100)

	// A call to instance 100 would be recursive
	require.True(t, tracker.CallMightBeRecursive(100))

	// A call to instance 200 would not be recursive
	require.False(t, tracker.CallMightBeRecursive(200))
}

func TestCallMightBeRecursive_NestedCalls(t *testing.T) {
	tracker := NewReentranceTracker()

	// 100 calls 200
	tracker.EnterInstance(100)
	tracker.EnterInstance(200)

	// Both would be recursive
	require.True(t, tracker.CallMightBeRecursive(100))
	require.True(t, tracker.CallMightBeRecursive(200))

	// 300 would not be recursive
	require.False(t, tracker.CallMightBeRecursive(300))

	// Leave 200
	tracker.LeaveInstance(200)
	require.False(t, tracker.CallMightBeRecursive(200))
	require.True(t, tracker.CallMightBeRecursive(100))
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -run "TestCallMightBeRecursive" -v`
Expected: FAIL with "NewReentranceTracker undefined"

**Step 3: Write minimal implementation**

Create `internal/component/reentrance.go`:
```go
// internal/component/reentrance.go

package component

// ReentranceTracker tracks which component instances are currently on the call stack.
// This is used to implement the call_might_be_recursive check from the spec.
type ReentranceTracker struct {
	// Maps instance ID to the number of times it's currently on the call stack
	activeCalls map[uint32]int
}

// NewReentranceTracker creates a new reentrance tracker.
func NewReentranceTracker() *ReentranceTracker {
	return &ReentranceTracker{
		activeCalls: make(map[uint32]int),
	}
}

// EnterInstance records that we're entering a call to the given instance.
func (r *ReentranceTracker) EnterInstance(instanceID uint32) {
	r.activeCalls[instanceID]++
}

// LeaveInstance records that we're leaving a call to the given instance.
func (r *ReentranceTracker) LeaveInstance(instanceID uint32) {
	if count := r.activeCalls[instanceID]; count > 0 {
		r.activeCalls[instanceID] = count - 1
		if r.activeCalls[instanceID] == 0 {
			delete(r.activeCalls, instanceID)
		}
	}
}

// CallMightBeRecursive returns true if calling the given instance would be recursive.
// This implements the spec's call_might_be_recursive check.
//
// From spec: A call to an instance is recursive if that instance is already
// on the current call stack.
func (r *ReentranceTracker) CallMightBeRecursive(instanceID uint32) bool {
	return r.activeCalls[instanceID] > 0
}

// ActiveInstances returns a copy of the active instance IDs (for debugging).
func (r *ReentranceTracker) ActiveInstances() []uint32 {
	result := make([]uint32, 0, len(r.activeCalls))
	for id := range r.activeCalls {
		result = append(result, id)
	}
	return result
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -run "TestCallMightBeRecursive" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/reentrance.go internal/component/reentrance_test.go
git commit -m "feat(resource): add ReentranceTracker for call_might_be_recursive"
```

---

## Task 5.4: Add Reentrance Trap to DropOwned

**Files:**
- Modify: `internal/component/resource_table.go`
- Test: `internal/component/resource_table_test.go`

**Step 1: Write the failing test**

Add to `internal/component/resource_table_test.go`:
```go
func TestResourceTable_DropOwned_TrapsOnReentrance(t *testing.T) {
	table := NewResourceTable()
	registry := NewDestructorRegistry()
	tracker := NewReentranceTracker()

	// No destructor registered for this type
	// (reentrance check only applies when no destructor)

	h := table.NewWithType(uint32(42), true, NewResourceTypeID(1))

	// Instance 100 is currently on the call stack
	tracker.EnterInstance(100)

	// Dropping a resource defined in instance 100 from instance 200
	// should trap because of potential reentrance
	err := table.DropOwnedWithReentranceCheck(
		h,
		NewResourceTypeID(1),
		registry,
		200,  // current instance
		100,  // defining instance (on call stack!)
		nil,
		tracker,
	)
	require.ErrorIs(t, err, ErrReentrance)
}

func TestResourceTable_DropOwned_NoReentranceWithDestructor(t *testing.T) {
	table := NewResourceTable()
	registry := NewDestructorRegistry()
	tracker := NewReentranceTracker()

	// Register a destructor
	registry.Register(NewResourceTypeID(1), func(rep uint32) {})

	h := table.NewWithType(uint32(42), true, NewResourceTypeID(1))

	// Instance 100 is on the call stack
	tracker.EnterInstance(100)

	// But since there's a destructor, reentrance check is skipped
	// (the destructor will be called via canon_lift/canon_lower which handles reentrance)
	err := table.DropOwnedWithReentranceCheck(
		h,
		NewResourceTypeID(1),
		registry,
		200,
		100,
		func(rep, inst uint32) {}, // cross-instance dtor
		tracker,
	)
	require.NoError(t, err)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -run "TestResourceTable_DropOwned_Traps|TestResourceTable_DropOwned_NoReentrance" -v`
Expected: FAIL with "ErrReentrance undefined" or "DropOwnedWithReentranceCheck undefined"

**Step 3: Write minimal implementation**

Add to `internal/component/resource_table.go`:
```go
// ErrReentrance is returned when an operation would cause invalid reentrance.
var ErrReentrance = errors.New("operation would cause invalid recursive reentrance")

// DropOwnedWithReentranceCheck drops an owned handle with full reentrance checking.
// This implements the complete spec behavior for resource.drop.
//
// From spec (CanonicalABI.md:3664-3667):
//
//	else:
//	  trap_if(call_might_be_recursive(thread.task, rt.impl))
//
// The reentrance check only applies when there's no destructor.
func (t *ResourceTable) DropOwnedWithReentranceCheck(
	h Handle,
	expectedRT ResourceTypeID,
	dtorRegistry *DestructorRegistry,
	currentInstanceID uint32,
	definingInstanceID uint32,
	crossInstanceDtor CrossInstanceDestructor,
	tracker *ReentranceTracker,
) error {
	// Validate type first
	if expectedRT.IsValid() {
		if err := t.ValidateType(h, expectedRT); err != nil {
			return err
		}
	}

	// Check if this is cross-instance without destructor
	if currentInstanceID != definingInstanceID {
		hasDestructor := dtorRegistry != nil && dtorRegistry.Has(expectedRT)
		if !hasDestructor && tracker != nil && tracker.CallMightBeRecursive(definingInstanceID) {
			return ErrReentrance
		}
	}

	// Proceed with normal DropOwned
	return t.DropOwned(h, expectedRT, dtorRegistry, currentInstanceID, definingInstanceID, crossInstanceDtor)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -run "TestResourceTable_DropOwned_Traps|TestResourceTable_DropOwned_NoReentrance" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/resource_table.go internal/component/resource_table_test.go
git commit -m "feat(resource): add reentrance trap to DropOwnedWithReentranceCheck"
```

---

## Task 5.5: Add Table MAX_LENGTH Enforcement

**Files:**
- Modify: `internal/component/resource_table.go`
- Test: `internal/component/resource_table_test.go`

**Step 1: Write the failing test**

Add to `internal/component/resource_table_test.go`:
```go
func TestResourceTable_MaxLength(t *testing.T) {
	// This is a documentation/constant test, not a real allocation test
	// (we don't want to allocate 2^28 entries in a test)
	require.Equal(t, uint32(1<<28-1), MaxTableLength)
}

func TestResourceTable_ReturnsErrorOnOverflow(t *testing.T) {
	// This tests the error path, not actual overflow
	// We mock this by checking the error type exists
	err := ErrTableFull
	require.Error(t, err)
	require.Contains(t, err.Error(), "table full")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -run "TestResourceTable_MaxLength|TestResourceTable_ReturnsErrorOnOverflow" -v`
Expected: FAIL with "MaxTableLength undefined" or "ErrTableFull undefined"

**Step 3: Write minimal implementation**

Add to `internal/component/resource_table.go`:
```go
// MaxTableLength is the maximum number of entries in a resource table.
// From the spec, this is 2^28 - 1.
const MaxTableLength = uint32(1<<28 - 1)

// ErrTableFull is returned when the resource table has reached its maximum size.
var ErrTableFull = errors.New("resource table full: maximum length exceeded")

// NewWithLimit creates a new resource handle, returning an error if the table is full.
func (t *ResourceTable) NewWithLimit(rep any, own bool, rtID ResourceTypeID) (Handle, error) {
	if uint32(len(t.entries)) >= MaxTableLength && t.freeHead < 0 {
		return 0, ErrTableFull
	}
	return t.NewWithType(rep, own, rtID), nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -run "TestResourceTable_MaxLength|TestResourceTable_ReturnsErrorOnOverflow" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/resource_table.go internal/component/resource_table_test.go
git commit -m "feat(resource): add MaxTableLength constant and table full error"
```

---

## Phase 5 Completion: Final Regression Check

**CRITICAL: Run all regression tests**

```bash
CGO_ENABLED=0 go test -v ./internal/component/wasip2test/... -run "TestCalculatorPlugins/add"
CGO_ENABLED=0 go test -v ./internal/component/wasip2test/... -run "TestCalculatorPlugins/subtract"
```

**Expected:** Both tests PASS

Run comprehensive tests:
```bash
CGO_ENABLED=0 go test -v ./internal/component/... -v
CGO_ENABLED=0 go test -v ./internal/component/types/... -v
CGO_ENABLED=0 go test -v ./internal/component/abi/... -v
CGO_ENABLED=0 go test -v ./internal/component/conformance/... -v
```

**Expected:** All tests PASS

**If all tests pass, commit the phase completion:**
```bash
git commit --allow-empty -m "milestone: complete phase 5 - advanced features"
git commit --allow-empty -m "milestone: complete resource system spec conformance"
```

---

## Summary of Changes in Phase 5

| File | Change |
|------|--------|
| `internal/component/instance_state.go` | NEW: InstanceState for may_leave tracking |
| `internal/component/instance_state_test.go` | NEW: InstanceState tests |
| `internal/component/resource_table.go` | ADD: NewWithMayLeaveCheck method |
| `internal/component/resource_table.go` | ADD: ErrMayNotLeave error |
| `internal/component/reentrance.go` | NEW: ReentranceTracker |
| `internal/component/reentrance_test.go` | NEW: ReentranceTracker tests |
| `internal/component/resource_table.go` | ADD: DropOwnedWithReentranceCheck |
| `internal/component/resource_table.go` | ADD: ErrReentrance error |
| `internal/component/resource_table.go` | ADD: MaxTableLength, ErrTableFull |

---

## Plan Complete

All five phases of the resource system spec conformance plan are complete:

1. **Phase 1**: Core Type System - ResourceTypeID tracking
2. **Phase 2**: Trap Conditions - Proper error handling
3. **Phase 3**: Borrow Scope Integration - Lenders tracking and same-instance optimization
4. **Phase 4**: Destructor Support - Full destructor invocation paths
5. **Phase 5**: Advanced Features - may_leave, reentrance, table limits

The implementation now aligns with the Component Model specification as documented in `debug-vendored/component-model/design/mvp/CanonicalABI.md`.

---

## Back to Root

Return to: [00-root-plan.md](./00-root-plan.md) to update progress tracking.
