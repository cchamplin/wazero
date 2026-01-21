# Phase 1: Core Type System

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add ResourceType tracking to HandleEntry so all subsequent type validation can be implemented.

**Architecture:** Introduce a ResourceTypeID type that uniquely identifies resource types within a component instance. Store this ID in each HandleEntry. This is the foundation for all spec-required type checks.

**Tech Stack:** Go

---

## Prerequisites

- Read gap analysis: `docs/plans/resource-system-gap-analysis.md` (Section 1: Resource Handle Types Analysis)
- Understand spec: `debug-vendored/component-model/design/mvp/CanonicalABI.md` lines 497-549

## Reference: Spec ResourceHandle

From the spec (CanonicalABI.md:497-511):
```python
class ResourceHandle:
  rt: ResourceType           # <-- We need to add this
  rep: int
  own: bool
  borrow_scope: Optional[Task]
  num_lends: int
```

---

## Task 1.1: Define ResourceTypeID Type

**Files:**
- Create: `internal/component/resource_type_id.go`
- Test: `internal/component/resource_type_id_test.go`

**Step 1: Write the failing test**

Create `internal/component/resource_type_id_test.go`:
```go
package component

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestResourceTypeID_Equality(t *testing.T) {
	id1 := NewResourceTypeID(1)
	id2 := NewResourceTypeID(1)
	id3 := NewResourceTypeID(2)

	require.True(t, id1 == id2, "same index should be equal")
	require.False(t, id1 == id3, "different index should not be equal")
}

func TestResourceTypeID_Index(t *testing.T) {
	id := NewResourceTypeID(42)
	require.Equal(t, uint32(42), id.Index())
}

func TestResourceTypeID_IsValid(t *testing.T) {
	valid := NewResourceTypeID(1)
	invalid := InvalidResourceTypeID()

	require.True(t, valid.IsValid())
	require.False(t, invalid.IsValid())
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -run "TestResourceTypeID" -v`
Expected: FAIL with "undefined: NewResourceTypeID"

**Step 3: Write minimal implementation**

Create `internal/component/resource_type_id.go`:
```go
// internal/component/resource_type_id.go

package component

// ResourceTypeID uniquely identifies a resource type within a component instance.
// This corresponds to the 'rt' field in the spec's ResourceHandle.
// A value of 0 is reserved as invalid/unset.
type ResourceTypeID uint32

// NewResourceTypeID creates a ResourceTypeID from a type index.
// Type indices start at 1; 0 is reserved for invalid.
func NewResourceTypeID(typeIndex uint32) ResourceTypeID {
	return ResourceTypeID(typeIndex + 1)
}

// InvalidResourceTypeID returns an invalid ResourceTypeID (zero value).
func InvalidResourceTypeID() ResourceTypeID {
	return ResourceTypeID(0)
}

// Index returns the underlying type index.
// Returns the original index passed to NewResourceTypeID.
func (id ResourceTypeID) Index() uint32 {
	return uint32(id) - 1
}

// IsValid returns true if this is a valid resource type ID.
func (id ResourceTypeID) IsValid() bool {
	return id != 0
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -run "TestResourceTypeID" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/resource_type_id.go internal/component/resource_type_id_test.go
git commit -m "feat(resource): add ResourceTypeID type for resource type tracking"
```

---

## Task 1.2: Add RT Field to HandleEntry

**Files:**
- Modify: `internal/component/resource_table.go:32-38`
- Test: `internal/component/resource_table_test.go`

**Step 1: Write the failing test**

Add to `internal/component/resource_table_test.go`:
```go
func TestHandleEntry_HasResourceTypeID(t *testing.T) {
	table := NewResourceTable()

	// Create a handle with a specific resource type
	rtID := NewResourceTypeID(5)
	h := table.NewWithType(uint32(100), true, rtID)

	entry, err := table.Get(h)
	require.NoError(t, err)
	require.Equal(t, rtID, entry.RT)
	require.Equal(t, uint32(100), entry.Rep.(uint32))
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -run "TestHandleEntry_HasResourceTypeID" -v`
Expected: FAIL with "entry.RT undefined" or "NewWithType undefined"

**Step 3: Write minimal implementation**

Modify `internal/component/resource_table.go`:

First, update HandleEntry struct (lines 32-38):
```go
// HandleEntry represents an active resource in the table.
type HandleEntry struct {
	RT          ResourceTypeID // Resource type this handle belongs to
	Rep         any            // The resource representation value
	Own         bool           // True if this is an owning handle
	NumLends    uint32         // Number of active borrows from this handle
	BorrowScope any            // The scope that created this borrow (for borrowed handles)
}
```

Then, add the NewWithType method after line 96:
```go
// NewWithType creates a new resource handle with a specific resource type.
func (t *ResourceTable) NewWithType(rep any, own bool, rtID ResourceTypeID) Handle {
	var idx uint32
	var gen uint32

	if t.freeHead >= 0 {
		// Reuse a free slot
		idx = uint32(t.freeHead)
		entry := &t.entries[idx]
		t.freeHead = entry.nextFree
		gen = entry.generation + 1
		entry.state = entryOccupied
		entry.generation = gen
		entry.entry = HandleEntry{RT: rtID, Rep: rep, Own: own}
		entry.nextFree = -1
	} else {
		// Allocate new slot
		idx = uint32(len(t.entries))
		gen = 0
		t.entries = append(t.entries, tableEntry{
			state:      entryOccupied,
			generation: gen,
			entry:      HandleEntry{RT: rtID, Rep: rep, Own: own},
			nextFree:   -1,
		})
	}

	return MakeHandle(idx, gen)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -run "TestHandleEntry_HasResourceTypeID" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/resource_table.go internal/component/resource_table_test.go
git commit -m "feat(resource): add RT field to HandleEntry and NewWithType method"
```

---

## Task 1.3: Update Existing New() to Use InvalidResourceTypeID

**Files:**
- Modify: `internal/component/resource_table.go:69-96`

**Step 1: Write the failing test**

Add to `internal/component/resource_table_test.go`:
```go
func TestResourceTable_New_HasInvalidTypeByDefault(t *testing.T) {
	table := NewResourceTable()

	// Old API should still work, RT will be invalid
	h := table.New("test-rep", true)

	entry, err := table.Get(h)
	require.NoError(t, err)
	require.False(t, entry.RT.IsValid(), "legacy New() should have invalid RT")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -run "TestResourceTable_New_HasInvalidTypeByDefault" -v`
Expected: May FAIL if RT field isn't initialized (zero value should work, but verify)

**Step 3: Write minimal implementation**

Modify `internal/component/resource_table.go` New() method to explicitly set RT:

```go
// New creates a new resource handle and returns it.
// Note: This creates a handle with an invalid ResourceTypeID.
// Use NewWithType for type-tracked handles.
func (t *ResourceTable) New(rep any, own bool) Handle {
	return t.NewWithType(rep, own, InvalidResourceTypeID())
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -run "TestResourceTable_New_HasInvalidTypeByDefault" -v`
Expected: PASS

Also run all existing resource table tests to ensure backward compatibility:
Run: `go test ./internal/component/... -run "TestResource" -v`
Expected: All PASS

**Step 5: Commit**

```bash
git add internal/component/resource_table.go internal/component/resource_table_test.go
git commit -m "refactor(resource): update New() to delegate to NewWithType with invalid RT"
```

---

## Task 1.4: Update CreateResourceNewFunc to Accept and Store Type

**Files:**
- Modify: `internal/component/resource_table.go:215-220`

**Step 1: Write the failing test**

Add to `internal/component/resource_table_test.go`:
```go
func TestCreateResourceNewFunc_StoresResourceType(t *testing.T) {
	table := NewResourceTable()

	// Create the resource.new function for type index 3
	newFunc := table.CreateResourceNewFuncWithType(3)

	// Call it to create a resource with rep=42
	handleIdx := newFunc(42)

	// Verify the handle has the correct type
	entry, err := table.Get(Handle(handleIdx))
	require.NoError(t, err)
	require.True(t, entry.RT.IsValid())
	require.Equal(t, uint32(3), entry.RT.Index())
	require.Equal(t, uint32(42), entry.Rep.(uint32))
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -run "TestCreateResourceNewFunc_StoresResourceType" -v`
Expected: FAIL with "CreateResourceNewFuncWithType undefined"

**Step 3: Write minimal implementation**

Add to `internal/component/resource_table.go` after CreateResourceNewFunc:
```go
// CreateResourceNewFuncWithType creates a core function for resource.new
// that stores the resource type ID with each created handle.
// The resourceTypeIdx is the index from the component's type section.
func (t *ResourceTable) CreateResourceNewFuncWithType(resourceTypeIdx uint32) func(rep uint32) uint32 {
	rtID := NewResourceTypeID(resourceTypeIdx)
	return func(rep uint32) uint32 {
		handle := t.NewWithType(rep, true, rtID)
		return uint32(handle)
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -run "TestCreateResourceNewFunc_StoresResourceType" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/resource_table.go internal/component/resource_table_test.go
git commit -m "feat(resource): add CreateResourceNewFuncWithType with type tracking"
```

---

## Task 1.5: Add Type Accessor and Validation Helper Methods

**Files:**
- Modify: `internal/component/resource_table.go`

**Step 1: Write the failing test**

Add to `internal/component/resource_table_test.go`:
```go
func TestResourceTable_GetType(t *testing.T) {
	table := NewResourceTable()
	rtID := NewResourceTypeID(7)
	h := table.NewWithType(uint32(100), true, rtID)

	// GetType should return the resource type
	gotType, err := table.GetType(h)
	require.NoError(t, err)
	require.Equal(t, rtID, gotType)
}

func TestResourceTable_GetType_InvalidHandle(t *testing.T) {
	table := NewResourceTable()

	invalidHandle := MakeHandle(999, 0)
	_, err := table.GetType(invalidHandle)
	require.ErrorIs(t, err, ErrInvalidHandle)
}

func TestResourceTable_ValidateType(t *testing.T) {
	table := NewResourceTable()
	rtID := NewResourceTypeID(7)
	wrongID := NewResourceTypeID(8)
	h := table.NewWithType(uint32(100), true, rtID)

	// Correct type should pass
	require.NoError(t, table.ValidateType(h, rtID))

	// Wrong type should fail
	err := table.ValidateType(h, wrongID)
	require.Error(t, err)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -run "TestResourceTable_GetType|TestResourceTable_ValidateType" -v`
Expected: FAIL with "GetType undefined" or "ValidateType undefined"

**Step 3: Write minimal implementation**

Add to `internal/component/resource_table.go`:
```go
// ErrResourceTypeMismatch is returned when a handle's type doesn't match expected.
var ErrResourceTypeMismatch = errors.New("resource type mismatch")

// GetType returns the ResourceTypeID for a handle.
func (t *ResourceTable) GetType(h Handle) (ResourceTypeID, error) {
	entry, err := t.Get(h)
	if err != nil {
		return InvalidResourceTypeID(), err
	}
	return entry.RT, nil
}

// ValidateType checks that a handle has the expected resource type.
// Returns ErrResourceTypeMismatch if types don't match.
func (t *ResourceTable) ValidateType(h Handle, expected ResourceTypeID) error {
	actual, err := t.GetType(h)
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("%w: expected type %d, got %d", ErrResourceTypeMismatch, expected.Index(), actual.Index())
	}
	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/... -run "TestResourceTable_GetType|TestResourceTable_ValidateType" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/resource_table.go internal/component/resource_table_test.go
git commit -m "feat(resource): add GetType and ValidateType methods"
```

---

## Phase 1 Completion: Regression Check

**CRITICAL: Run regression tests before proceeding to Phase 2**

```bash
CGO_ENABLED=0 go test -v ./internal/component/wasip2test/... -run "TestCalculatorPlugins/add"
CGO_ENABLED=0 go test -v ./internal/component/wasip2test/... -run "TestCalculatorPlugins/subtract"
```

**Expected:** Both tests PASS

Also run full resource conformance tests:
```bash
CGO_ENABLED=0 go test -v ./internal/component/conformance/... -run "Resource"
```

**Expected:** All tests PASS

**If all tests pass, commit the phase completion:**
```bash
git commit --allow-empty -m "milestone: complete phase 1 - core type system"
```

---

## Summary of Changes in Phase 1

| File | Change |
|------|--------|
| `internal/component/resource_type_id.go` | NEW: ResourceTypeID type |
| `internal/component/resource_type_id_test.go` | NEW: ResourceTypeID tests |
| `internal/component/resource_table.go` | ADD: RT field to HandleEntry |
| `internal/component/resource_table.go` | ADD: NewWithType method |
| `internal/component/resource_table.go` | ADD: CreateResourceNewFuncWithType |
| `internal/component/resource_table.go` | ADD: GetType, ValidateType methods |
| `internal/component/resource_table.go` | ADD: ErrResourceTypeMismatch |
| `internal/component/resource_table_test.go` | ADD: Type tracking tests |

---

## Next Phase

Proceed to: [Phase 2: Trap Conditions](./02-phase2-trap-conditions.md)
