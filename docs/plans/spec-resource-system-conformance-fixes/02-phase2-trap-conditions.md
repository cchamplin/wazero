# Phase 2: Trap Conditions

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix incorrect error handling in resource operations to trap (panic/error) as required by the specification.

**Architecture:** Replace silent error returns with explicit traps. The spec requires trapping on invalid handles and type mismatches. In Go, this means returning errors that callers must handle (or panicking for truly unrecoverable situations).

**Tech Stack:** Go

---

## Prerequisites

- Complete Phase 1 (Core Type System)
- Read gap analysis: `docs/plans/resource-system-gap-analysis.md` (Sections 3.2 and 3.3)
- Understand spec traps: `debug-vendored/component-model/design/mvp/CanonicalABI.md` lines 3626-3687

## Reference: Spec Trap Conditions

From resource.drop (CanonicalABI.md:3626-3633):
```python
def canon_resource_drop(rt, thread, i):
  trap_if(not thread.task.inst.may_leave)
  inst = thread.task.inst
  h = inst.table.remove(i)
  trap_if(not isinstance(h, ResourceHandle))  # <-- Must trap, not ignore
  trap_if(h.rt is not rt)                      # <-- Must validate type
  trap_if(h.num_lends != 0)
  ...
```

From resource.rep (CanonicalABI.md:3682-3687):
```python
def canon_resource_rep(rt, thread, i):
  h = thread.task.inst.table.get(i)
  trap_if(not isinstance(h, ResourceHandle))  # <-- Must trap, not return 0
  trap_if(h.rt is not rt)                      # <-- Must validate type
  return [h.rep]
```

---

## Task 2.1: Fix CreateResourceDropFunc to Return Error on Invalid Handle

**Files:**
- Modify: `internal/component/resource_table.go:225-241`
- Test: `internal/component/resource_table_test.go`

**Current (DEFECTIVE) Implementation:**
```go
func (t *ResourceTable) CreateResourceDropFunc(resourceTypeIdx uint32, destructor func(rep uint32)) func(handle uint32) {
    return func(handle uint32) {
        entry, err := t.Remove(Handle(handle))
        if err != nil {
            return // Silently ignore invalid handles per spec  <-- WRONG!
        }
        ...
    }
}
```

**Step 1: Write the failing test**

Add to `internal/component/resource_table_test.go`:
```go
func TestCreateResourceDropFunc_TrapsOnInvalidHandle(t *testing.T) {
	table := NewResourceTable()

	// Create drop function for type 1
	var trapCalled bool
	var trapErr error
	dropFunc := table.CreateResourceDropFuncWithTrap(1, nil, func(err error) {
		trapCalled = true
		trapErr = err
	})

	// Try to drop an invalid handle
	dropFunc(999)

	require.True(t, trapCalled, "should trap on invalid handle")
	require.ErrorIs(t, trapErr, ErrInvalidHandle)
}

func TestCreateResourceDropFunc_TrapsOnTypeMismatch(t *testing.T) {
	table := NewResourceTable()

	// Create a handle of type 1
	h := table.NewWithType(uint32(42), true, NewResourceTypeID(1))

	// Create drop function for type 2 (different type)
	var trapCalled bool
	var trapErr error
	dropFunc := table.CreateResourceDropFuncWithTrap(2, nil, func(err error) {
		trapCalled = true
		trapErr = err
	})

	// Try to drop with wrong type
	dropFunc(uint32(h))

	require.True(t, trapCalled, "should trap on type mismatch")
	require.ErrorIs(t, trapErr, ErrResourceTypeMismatch)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -run "TestCreateResourceDropFunc_Traps" -v`
Expected: FAIL with "CreateResourceDropFuncWithTrap undefined"

**Step 3: Write minimal implementation**

Add to `internal/component/resource_table.go`:
```go
// TrapHandler is a function called when a resource operation should trap.
// In production, this typically panics or records the error for the runtime.
type TrapHandler func(err error)

// CreateResourceDropFuncWithTrap creates a core function for resource.drop
// that calls the trap handler on errors instead of silently ignoring them.
// This is the spec-compliant version that properly validates types.
func (t *ResourceTable) CreateResourceDropFuncWithTrap(resourceTypeIdx uint32, destructor func(rep uint32), trap TrapHandler) func(handle uint32) {
	expectedRT := NewResourceTypeID(resourceTypeIdx)
	return func(handle uint32) {
		h := Handle(handle)

		// Validate type before removal (spec: trap_if(h.rt is not rt))
		if expectedRT.IsValid() {
			if err := t.ValidateType(h, expectedRT); err != nil {
				trap(err)
				return
			}
		}

		entry, err := t.Remove(h)
		if err != nil {
			// Spec: trap_if(not isinstance(h, ResourceHandle))
			trap(err)
			return
		}

		// Call destructor for owned resources
		if destructor != nil && entry.Own && entry.Rep != nil {
			switch rep := entry.Rep.(type) {
			case uint32:
				destructor(rep)
			case int:
				destructor(uint32(rep))
			}
		}
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -run "TestCreateResourceDropFunc_Traps" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/resource_table.go internal/component/resource_table_test.go
git commit -m "fix(resource): add CreateResourceDropFuncWithTrap that traps on errors"
```

---

## Task 2.2: Fix CreateResourceRepFunc to Return Error on Invalid Handle

**Files:**
- Modify: `internal/component/resource_table.go:246-254`
- Test: `internal/component/resource_table_test.go`

**Current (DEFECTIVE) Implementation:**
```go
func (t *ResourceTable) CreateResourceRepFunc(resourceTypeIdx uint32) func(handle uint32) uint32 {
    return func(handle uint32) uint32 {
        rep, err := t.Rep(Handle(handle))
        if err != nil {
            return 0 // Return 0 for invalid handles  <-- WRONG!
        }
        return rep
    }
}
```

**Step 1: Write the failing test**

Add to `internal/component/resource_table_test.go`:
```go
func TestCreateResourceRepFunc_TrapsOnInvalidHandle(t *testing.T) {
	table := NewResourceTable()

	var trapCalled bool
	var trapErr error
	repFunc := table.CreateResourceRepFuncWithTrap(1, func(err error) {
		trapCalled = true
		trapErr = err
	})

	// Try to get rep of invalid handle
	_ = repFunc(999)

	require.True(t, trapCalled, "should trap on invalid handle")
	require.ErrorIs(t, trapErr, ErrInvalidHandle)
}

func TestCreateResourceRepFunc_TrapsOnTypeMismatch(t *testing.T) {
	table := NewResourceTable()

	// Create a handle of type 1
	h := table.NewWithType(uint32(42), true, NewResourceTypeID(1))

	var trapCalled bool
	var trapErr error
	// Create rep function for type 2 (different type)
	repFunc := table.CreateResourceRepFuncWithTrap(2, func(err error) {
		trapCalled = true
		trapErr = err
	})

	// Try to get rep with wrong type
	_ = repFunc(uint32(h))

	require.True(t, trapCalled, "should trap on type mismatch")
	require.ErrorIs(t, trapErr, ErrResourceTypeMismatch)
}

func TestCreateResourceRepFunc_ReturnsRepOnSuccess(t *testing.T) {
	table := NewResourceTable()

	h := table.NewWithType(uint32(42), true, NewResourceTypeID(1))

	var trapCalled bool
	repFunc := table.CreateResourceRepFuncWithTrap(1, func(err error) {
		trapCalled = true
	})

	rep := repFunc(uint32(h))

	require.False(t, trapCalled, "should not trap on valid handle")
	require.Equal(t, uint32(42), rep)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -run "TestCreateResourceRepFunc_Traps|TestCreateResourceRepFunc_Returns" -v`
Expected: FAIL with "CreateResourceRepFuncWithTrap undefined"

**Step 3: Write minimal implementation**

Add to `internal/component/resource_table.go`:
```go
// CreateResourceRepFuncWithTrap creates a core function for resource.rep
// that calls the trap handler on errors instead of returning 0.
// This is the spec-compliant version that properly validates types.
func (t *ResourceTable) CreateResourceRepFuncWithTrap(resourceTypeIdx uint32, trap TrapHandler) func(handle uint32) uint32 {
	expectedRT := NewResourceTypeID(resourceTypeIdx)
	return func(handle uint32) uint32 {
		h := Handle(handle)

		// Validate type (spec: trap_if(h.rt is not rt))
		if expectedRT.IsValid() {
			if err := t.ValidateType(h, expectedRT); err != nil {
				trap(err)
				return 0
			}
		}

		rep, err := t.Rep(h)
		if err != nil {
			// Spec: trap_if(not isinstance(h, ResourceHandle))
			trap(err)
			return 0
		}
		return rep
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -run "TestCreateResourceRepFunc_Traps|TestCreateResourceRepFunc_Returns" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/resource_table.go internal/component/resource_table_test.go
git commit -m "fix(resource): add CreateResourceRepFuncWithTrap that traps on errors"
```

---

## Task 2.3: Add RemoveWithType Method for Type-Validated Removal

**Files:**
- Modify: `internal/component/resource_table.go`
- Test: `internal/component/resource_table_test.go`

**Step 1: Write the failing test**

Add to `internal/component/resource_table_test.go`:
```go
func TestResourceTable_RemoveWithType_Success(t *testing.T) {
	table := NewResourceTable()
	rtID := NewResourceTypeID(5)
	h := table.NewWithType(uint32(100), true, rtID)

	entry, err := table.RemoveWithType(h, rtID)
	require.NoError(t, err)
	require.Equal(t, uint32(100), entry.Rep.(uint32))
}

func TestResourceTable_RemoveWithType_TypeMismatch(t *testing.T) {
	table := NewResourceTable()
	rtID := NewResourceTypeID(5)
	wrongID := NewResourceTypeID(6)
	h := table.NewWithType(uint32(100), true, rtID)

	_, err := table.RemoveWithType(h, wrongID)
	require.ErrorIs(t, err, ErrResourceTypeMismatch)

	// Handle should still be valid (not removed on type error)
	entry, err := table.Get(h)
	require.NoError(t, err)
	require.Equal(t, uint32(100), entry.Rep.(uint32))
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -run "TestResourceTable_RemoveWithType" -v`
Expected: FAIL with "RemoveWithType undefined"

**Step 3: Write minimal implementation**

Add to `internal/component/resource_table.go`:
```go
// RemoveWithType removes a handle from the table after validating its type.
// Returns ErrResourceTypeMismatch if types don't match.
// The handle is NOT removed if type validation fails.
func (t *ResourceTable) RemoveWithType(h Handle, expectedRT ResourceTypeID) (*HandleEntry, error) {
	// Validate type first (before removal)
	if expectedRT.IsValid() {
		if err := t.ValidateType(h, expectedRT); err != nil {
			return nil, err
		}
	}

	return t.Remove(h)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -run "TestResourceTable_RemoveWithType" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/resource_table.go internal/component/resource_table_test.go
git commit -m "feat(resource): add RemoveWithType for type-validated removal"
```

---

## Task 2.4: Add GetWithType Method for Type-Validated Access

**Files:**
- Modify: `internal/component/resource_table.go`
- Test: `internal/component/resource_table_test.go`

**Step 1: Write the failing test**

Add to `internal/component/resource_table_test.go`:
```go
func TestResourceTable_GetWithType_Success(t *testing.T) {
	table := NewResourceTable()
	rtID := NewResourceTypeID(5)
	h := table.NewWithType(uint32(100), true, rtID)

	entry, err := table.GetWithType(h, rtID)
	require.NoError(t, err)
	require.Equal(t, uint32(100), entry.Rep.(uint32))
}

func TestResourceTable_GetWithType_TypeMismatch(t *testing.T) {
	table := NewResourceTable()
	rtID := NewResourceTypeID(5)
	wrongID := NewResourceTypeID(6)
	h := table.NewWithType(uint32(100), true, rtID)

	_, err := table.GetWithType(h, wrongID)
	require.ErrorIs(t, err, ErrResourceTypeMismatch)
}

func TestResourceTable_GetWithType_InvalidHandle(t *testing.T) {
	table := NewResourceTable()
	invalidH := MakeHandle(999, 0)

	_, err := table.GetWithType(invalidH, NewResourceTypeID(1))
	require.ErrorIs(t, err, ErrInvalidHandle)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -run "TestResourceTable_GetWithType" -v`
Expected: FAIL with "GetWithType undefined"

**Step 3: Write minimal implementation**

Add to `internal/component/resource_table.go`:
```go
// GetWithType retrieves an entry after validating its type.
// Returns ErrResourceTypeMismatch if types don't match.
func (t *ResourceTable) GetWithType(h Handle, expectedRT ResourceTypeID) (*HandleEntry, error) {
	entry, err := t.Get(h)
	if err != nil {
		return nil, err
	}

	if expectedRT.IsValid() && entry.RT != expectedRT {
		return nil, fmt.Errorf("%w: expected type %d, got %d", ErrResourceTypeMismatch, expectedRT.Index(), entry.RT.Index())
	}

	return entry, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -run "TestResourceTable_GetWithType" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/resource_table.go internal/component/resource_table_test.go
git commit -m "feat(resource): add GetWithType for type-validated access"
```

---

## Task 2.5: Add RepWithType Method for Type-Validated Rep Access

**Files:**
- Modify: `internal/component/resource_table.go`
- Test: `internal/component/resource_table_test.go`

**Step 1: Write the failing test**

Add to `internal/component/resource_table_test.go`:
```go
func TestResourceTable_RepWithType_Success(t *testing.T) {
	table := NewResourceTable()
	rtID := NewResourceTypeID(5)
	h := table.NewWithType(uint32(100), true, rtID)

	rep, err := table.RepWithType(h, rtID)
	require.NoError(t, err)
	require.Equal(t, uint32(100), rep)
}

func TestResourceTable_RepWithType_TypeMismatch(t *testing.T) {
	table := NewResourceTable()
	rtID := NewResourceTypeID(5)
	wrongID := NewResourceTypeID(6)
	h := table.NewWithType(uint32(100), true, rtID)

	_, err := table.RepWithType(h, wrongID)
	require.ErrorIs(t, err, ErrResourceTypeMismatch)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -run "TestResourceTable_RepWithType" -v`
Expected: FAIL with "RepWithType undefined"

**Step 3: Write minimal implementation**

Add to `internal/component/resource_table.go`:
```go
// RepWithType returns the representation value after validating the handle's type.
// Returns ErrResourceTypeMismatch if types don't match.
func (t *ResourceTable) RepWithType(h Handle, expectedRT ResourceTypeID) (uint32, error) {
	entry, err := t.GetWithType(h, expectedRT)
	if err != nil {
		return 0, err
	}

	switch v := entry.Rep.(type) {
	case uint32:
		return v, nil
	case int:
		return uint32(v), nil
	default:
		return 0, fmt.Errorf("resource rep is not a uint32: %T", entry.Rep)
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -run "TestResourceTable_RepWithType" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/resource_table.go internal/component/resource_table_test.go
git commit -m "feat(resource): add RepWithType for type-validated rep access"
```

---

## Phase 2 Completion: Regression Check

**CRITICAL: Run regression tests before proceeding to Phase 3**

```bash
CGO_ENABLED=0 go test -v ./internal/component/wasip2test/... -run "TestCalculatorPlugins/add"
CGO_ENABLED=0 go test -v ./internal/component/wasip2test/... -run "TestCalculatorPlugins/subtract"
```

**Expected:** Both tests PASS

Also run full resource tests:
```bash
CGO_ENABLED=0 go test -v ./internal/component/... -run "TestResource" -v
CGO_ENABLED=0 go test -v ./internal/component/conformance/... -run "Resource"
```

**Expected:** All tests PASS

**If all tests pass, commit the phase completion:**
```bash
git commit --allow-empty -m "milestone: complete phase 2 - trap conditions"
```

---

## Summary of Changes in Phase 2

| File | Change |
|------|--------|
| `internal/component/resource_table.go` | ADD: TrapHandler type |
| `internal/component/resource_table.go` | ADD: CreateResourceDropFuncWithTrap |
| `internal/component/resource_table.go` | ADD: CreateResourceRepFuncWithTrap |
| `internal/component/resource_table.go` | ADD: RemoveWithType method |
| `internal/component/resource_table.go` | ADD: GetWithType method |
| `internal/component/resource_table.go` | ADD: RepWithType method |
| `internal/component/resource_table_test.go` | ADD: Trap condition tests |

---

## Next Phase

Proceed to: [Phase 3: Borrow Scope Integration](./03-phase3-borrow-scope-integration.md)
