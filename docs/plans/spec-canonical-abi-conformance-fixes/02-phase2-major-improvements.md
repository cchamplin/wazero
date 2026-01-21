# Phase 2: Major Improvements

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement major spec features and fix significant spec violations.

**Architecture:** Type system enhancements for fixed-length lists, validation for empty types, and optimization for borrow operations.

**Tech Stack:** Go, Component Model types

---

## Prerequisites

- Complete all Phase 1 tasks
- All Phase 1 tests passing

---

## Reference

- **Gap Analysis:** `docs/plans/canonical-abi-gap-analysis.md` (Sections 8-12)
- **Spec:** `debug-vendored/component-model/design/mvp/CanonicalABI.md`

---

## Task 2.1: Fixed-Length List Type Support

**Problem:** Spec supports fixed-length lists with optional length parameter. Current implementation only supports dynamic lists.

**Spec Reference:** CanonicalABI.md lines 1860-1875, 1946-1957

**Files:**
- Modify: `internal/component/types/composite.go`
- Test: `internal/component/types/composite_test.go`

### Step 1: Write the failing test

Create or add to `internal/component/types/composite_test.go`:

```go
package types

import (
	"testing"
)

func TestFixedLengthListAlignment(t *testing.T) {
	// Fixed-length list alignment = element alignment
	fixedList := List{Element: U32{}, Length: ptr(3)}
	if got := fixedList.Align(); got != 4 {
		t.Errorf("fixed list Align() = %d, want 4 (element alignment)", got)
	}

	// Dynamic list alignment = 4 (pointer alignment)
	dynamicList := List{Element: U8{}}
	if got := dynamicList.Align(); got != 4 {
		t.Errorf("dynamic list Align() = %d, want 4 (pointer alignment)", got)
	}
}

func TestFixedLengthListSize(t *testing.T) {
	// Fixed-length list size = length * element_size
	fixedList := List{Element: U32{}, Length: ptr(3)}
	if got := fixedList.Size(); got != 12 { // 3 * 4
		t.Errorf("fixed list Size() = %d, want 12", got)
	}

	// Dynamic list size = 8 (ptr + len)
	dynamicList := List{Element: U32{}}
	if got := dynamicList.Size(); got != 8 {
		t.Errorf("dynamic list Size() = %d, want 8", got)
	}
}

func TestFixedLengthListFlattenCount(t *testing.T) {
	// Fixed-length list flattens to length * element_flatten_count
	fixedList := List{Element: U32{}, Length: ptr(3)}
	if got := fixedList.FlattenCount(); got != 3 { // 3 * 1
		t.Errorf("fixed list FlattenCount() = %d, want 3", got)
	}

	// Dynamic list flattens to 2 (ptr, len)
	dynamicList := List{Element: U32{}}
	if got := dynamicList.FlattenCount(); got != 2 {
		t.Errorf("dynamic list FlattenCount() = %d, want 2", got)
	}
}

// Helper to create *uint32
func ptr(v uint32) *uint32 {
	return &v
}
```

### Step 2: Run test to verify it fails

```bash
go test -v ./internal/component/types/... -run "TestFixedLengthList"
```

Expected: FAIL - List type doesn't have Length field

### Step 3: Update List type to support fixed length

Modify `internal/component/types/composite.go`:

```go
// List represents a list type.
// If Length is nil, it's a dynamic list (ptr + len in memory).
// If Length is set, it's a fixed-length list (inline elements).
type List struct {
	Element ValType
	Length  *uint32 // nil for dynamic lists, set for fixed-length
}

func (List) valType() {}

// Size returns the size of the list in memory.
// Dynamic lists: 8 bytes (ptr: i32, len: i32)
// Fixed lists: length * element_size
func (l List) Size() uint32 {
	if l.Length != nil {
		return *l.Length * l.Element.Size()
	}
	return 8 // ptr + len
}

// Align returns the alignment of the list.
// Dynamic lists: 4 (pointer alignment)
// Fixed lists: element alignment
func (l List) Align() uint32 {
	if l.Length != nil {
		return l.Element.Align()
	}
	return 4
}

// FlattenCount returns the number of core wasm values when flattened.
// Dynamic lists: 2 (pointer and length)
// Fixed lists: length * element_flatten_count
func (l List) FlattenCount() int {
	if l.Length != nil {
		return int(*l.Length) * l.Element.FlattenCount()
	}
	return 2
}

// ElementSize returns the size of each element.
func (l List) ElementSize() uint32 { return l.Element.Size() }

// ElementAlign returns the alignment of each element.
func (l List) ElementAlign() uint32 { return l.Element.Align() }

// IsFixedLength returns true if this is a fixed-length list.
func (l List) IsFixedLength() bool { return l.Length != nil }
```

### Step 4: Run test to verify it passes

```bash
go test -v ./internal/component/types/... -run "TestFixedLengthList"
```

Expected: PASS

### Step 5: Run all type tests

```bash
go test -v ./internal/component/types/...
```

Expected: PASS

### Step 6: Commit

```bash
git add internal/component/types/composite.go internal/component/types/composite_test.go
git commit -m "feat(types): add fixed-length list support

Per Canonical ABI spec lines 1860-1875, lists can have optional fixed
length. Fixed-length lists:
- Align to element alignment (not pointer alignment)
- Size is length * element_size
- Flatten to length * element_flatten_count values"
```

---

## Task 2.2: Fixed-Length List Lifting

**Problem:** Fixed-length lists should be lifted inline (not from ptr+len), and should flatten to multiple values.

**Spec Reference:** CanonicalABI.md lines 2145-2161, 2935-2943

**Files:**
- Modify: `internal/component/abi/lift.go`
- Modify: `internal/component/abi/flatten.go`
- Test: `internal/component/abi/lift_test.go`

### Step 1: Write the failing test for fixed-length list lifting

Add to `internal/component/abi/lift_test.go`:

```go
func TestLiftHeapFixedLengthList(t *testing.T) {
	// Create memory with 3 u32 values inline
	mem := &testMemory{data: make([]byte, 20)}
	binary.LittleEndian.PutUint32(mem.data[0:], 10)
	binary.LittleEndian.PutUint32(mem.data[4:], 20)
	binary.LittleEndian.PutUint32(mem.data[8:], 30)

	ctx := &LiftContext{Memory: mem, Opts: &Options{}}
	length := uint32(3)
	listType := types.List{Element: types.U32{}, Length: &length}

	val, err := LiftHeap(ctx, listType, 0)
	if err != nil {
		t.Fatalf("LiftHeap failed: %v", err)
	}

	elems := val.List()
	if len(elems) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(elems))
	}
	expected := []uint32{10, 20, 30}
	for i, elem := range elems {
		if got := elem.U32(); got != expected[i] {
			t.Errorf("element[%d] = %d, want %d", i, got, expected[i])
		}
	}
}

func TestLiftFlatFixedLengthList(t *testing.T) {
	// Fixed-length list of 3 u32s should read 3 flat values
	length := uint32(3)
	listType := types.List{Element: types.U32{}, Length: &length}

	iter := NewFlatIter([]uint64{10, 20, 30})
	ctx := &LiftContext{Opts: &Options{}}

	val, err := LiftFlat(ctx, listType, iter)
	if err != nil {
		t.Fatalf("LiftFlat failed: %v", err)
	}

	elems := val.List()
	if len(elems) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(elems))
	}
	expected := []uint32{10, 20, 30}
	for i, elem := range elems {
		if got := elem.U32(); got != expected[i] {
			t.Errorf("element[%d] = %d, want %d", i, got, expected[i])
		}
	}
}
```

### Step 2: Run test to verify it fails

```bash
go test -v ./internal/component/abi/... -run "TestLiftHeapFixedLengthList|TestLiftFlatFixedLengthList"
```

Expected: FAIL - fixed-length lists not handled

### Step 3: Update flattenType for fixed-length lists

Modify `internal/component/abi/flatten.go`:

```go
	case types.List:
		if v.Length != nil {
			// Fixed-length list: flatten each element
			var result []api.ValueType
			elemFlat := flattenType(v.Element)
			for i := uint32(0); i < *v.Length; i++ {
				result = append(result, elemFlat...)
			}
			return result
		}
		return []api.ValueType{api.ValueTypeI32, api.ValueTypeI32} // ptr, len
```

### Step 4: Update LiftHeap for fixed-length lists

Modify the List case in `internal/component/abi/lift.go` LiftHeap function:

```go
	// List
	case types.List:
		if t.Length != nil {
			// Fixed-length list: elements are inline at offset
			length := *t.Length
			elemSize := t.Element.Size()
			elems := make([]component.Val, length)
			for i := uint32(0); i < length; i++ {
				elemOffset := offset + i*elemSize
				// Align element offset
				align := t.Element.Align()
				if elemOffset%align != 0 {
					elemOffset = alignTo(elemOffset, align)
				}
				elem, err := LiftHeap(ctx, t.Element, elemOffset)
				if err != nil {
					return component.Val{}, fmt.Errorf("lift fixed list element %d: %w", i, err)
				}
				elems[i] = elem
			}
			return component.ValList(elems), nil
		}

		// Dynamic list: read ptr and length
		ptr, err := ctx.ReadU32(offset)
		// ... rest of existing dynamic list code
```

### Step 5: Update LiftFlat for fixed-length lists

Modify the List case in `internal/component/abi/lift.go` LiftFlat function:

```go
	case types.List:
		t := typ.(types.List)

		if t.Length != nil {
			// Fixed-length list: lift each element from flat values
			length := *t.Length
			elems := make([]component.Val, length)
			for i := uint32(0); i < length; i++ {
				elem, err := LiftFlat(ctx, t.Element, iter)
				if err != nil {
					return component.Val{}, fmt.Errorf("lift fixed list element %d: %w", i, err)
				}
				elems[i] = elem
			}
			return component.ValList(elems), nil
		}

		// Dynamic list: read ptr and length
		ptr := iter.NextI32()
		length := iter.NextI32()
		// ... rest of existing dynamic list code
```

### Step 6: Run test to verify it passes

```bash
go test -v ./internal/component/abi/... -run "TestLiftHeapFixedLengthList|TestLiftFlatFixedLengthList"
```

Expected: PASS

### Step 7: Commit

```bash
git add internal/component/abi/lift.go internal/component/abi/flatten.go internal/component/abi/lift_test.go
git commit -m "feat(abi): implement fixed-length list lifting

Per Canonical ABI spec lines 2145-2161 and 2935-2943, fixed-length
lists are lifted:
- From heap: elements inline at offset
- From flat: elements as sequential flat values"
```

---

## Task 2.3: Fixed-Length List Lowering

**Problem:** Fixed-length lists should be lowered inline, not as ptr+len.

**Spec Reference:** CanonicalABI.md lines 2594-2614, 3054-3062

**Files:**
- Modify: `internal/component/abi/lower.go`
- Test: `internal/component/abi/lower_test.go`

### Step 1: Write the failing test

Add to `internal/component/abi/lower_test.go`:

```go
func TestLowerHeapFixedLengthList(t *testing.T) {
	mem := &testMemory{data: make([]byte, 20)}
	ctx := &LowerContext{Memory: mem, Opts: &Options{}}

	length := uint32(3)
	listType := types.List{Element: types.U32{}, Length: &length}

	elements := []component.Val{
		component.ValU32(10),
		component.ValU32(20),
		component.ValU32(30),
	}
	val := component.ValList(elements)

	err := LowerHeap(ctx, listType, val, 0)
	if err != nil {
		t.Fatalf("LowerHeap failed: %v", err)
	}

	// Verify elements written inline
	expected := []uint32{10, 20, 30}
	for i, exp := range expected {
		got := binary.LittleEndian.Uint32(mem.data[i*4:])
		if got != exp {
			t.Errorf("element[%d] = %d, want %d", i, got, exp)
		}
	}
}

func TestLowerFlatFixedLengthList(t *testing.T) {
	ctx := &LowerContext{Opts: &Options{}}

	length := uint32(3)
	listType := types.List{Element: types.U32{}, Length: &length}

	elements := []component.Val{
		component.ValU32(10),
		component.ValU32(20),
		component.ValU32(30),
	}
	val := component.ValList(elements)

	flat, err := LowerFlat(ctx, listType, val)
	if err != nil {
		t.Fatalf("LowerFlat failed: %v", err)
	}

	// Fixed list should flatten to 3 values, not ptr+len
	if len(flat) != 3 {
		t.Fatalf("expected 3 flat values, got %d", len(flat))
	}

	expected := []uint64{10, 20, 30}
	for i, exp := range expected {
		if flat[i] != exp {
			t.Errorf("flat[%d] = %d, want %d", i, flat[i], exp)
		}
	}
}
```

### Step 2: Run test to verify it fails

```bash
go test -v ./internal/component/abi/... -run "TestLowerHeapFixedLengthList|TestLowerFlatFixedLengthList"
```

Expected: FAIL - fixed-length lists not handled

### Step 3: Update LowerHeap for fixed-length lists

Modify the List case in `internal/component/abi/lower.go` LowerHeap function:

```go
	case types.List:
		t := typ.(types.List)
		elements := val.List()

		if t.Length != nil {
			// Fixed-length list: validate length and write elements inline
			if uint32(len(elements)) != *t.Length {
				return fmt.Errorf("fixed list length mismatch: got %d, expected %d", len(elements), *t.Length)
			}

			elemSize := t.Element.Size()
			for i, elem := range elements {
				elemOffset := offset + uint32(i)*elemSize
				if err := LowerHeap(ctx, t.Element, elem, elemOffset); err != nil {
					return fmt.Errorf("lower fixed list element %d: %w", i, err)
				}
			}
			return nil
		}

		// Dynamic list: existing code
		length := uint32(len(elements))
		// ... rest of existing dynamic list code
```

### Step 4: Update LowerFlat for fixed-length lists

Modify the List case in `internal/component/abi/lower.go` LowerFlat function:

```go
	case types.List:
		t := typ.(types.List)
		elements := val.List()

		if t.Length != nil {
			// Fixed-length list: validate length and lower each element
			if uint32(len(elements)) != *t.Length {
				return nil, fmt.Errorf("fixed list length mismatch: got %d, expected %d", len(elements), *t.Length)
			}

			var result []uint64
			for i, elem := range elements {
				flat, err := LowerFlat(ctx, t.Element, elem)
				if err != nil {
					return nil, fmt.Errorf("lower fixed list element %d: %w", i, err)
				}
				result = append(result, flat...)
			}
			return result, nil
		}

		// Dynamic list: existing code
		length := uint32(len(elements))
		// ... rest of existing dynamic list code
```

### Step 5: Run test to verify it passes

```bash
go test -v ./internal/component/abi/... -run "TestLowerHeapFixedLengthList|TestLowerFlatFixedLengthList"
```

Expected: PASS

### Step 6: Commit

```bash
git add internal/component/abi/lower.go internal/component/abi/lower_test.go
git commit -m "feat(abi): implement fixed-length list lowering

Per Canonical ABI spec lines 2594-2614 and 3054-3062, fixed-length
lists are lowered:
- To heap: elements written inline at offset
- To flat: elements as sequential flat values"
```

---

## Task 2.4: Empty Type Prohibition

**Problem:** Spec prohibits empty types (records with no fields). Current implementation allows them.

**Spec Reference:** CanonicalABI.md lines 1930-1932, `assert(s > 0)` in elem_size_record

**Files:**
- Modify: `internal/component/types/composite.go`
- Test: `internal/component/types/composite_test.go`

### Step 1: Write the test

Add to `internal/component/types/composite_test.go`:

```go
func TestEmptyRecordSize(t *testing.T) {
	// Per spec, empty records should have size > 0 or be prohibited
	// The spec says "Empty types, such as records with no fields, are not permitted"
	// However, for compatibility we might want to give them size 1

	emptyRecord := Record{Fields: []Field{}}

	// Option 1: Panic/error (strict spec compliance)
	// Option 2: Size 1 (lenient, for compatibility)

	// For now, we document current behavior
	size := emptyRecord.Size()
	t.Logf("Empty record size = %d", size)

	// The spec says size must be > 0, assert 1963
	if size == 0 {
		t.Log("WARNING: Empty record has size 0, spec says this is not permitted")
	}
}

func TestEmptyTupleSize(t *testing.T) {
	emptyTuple := Tuple{Types: []ValType{}}
	size := emptyTuple.Size()
	t.Logf("Empty tuple size = %d", size)

	if size == 0 {
		t.Log("WARNING: Empty tuple has size 0, spec says this is not permitted")
	}
}
```

### Step 2: Run test to observe current behavior

```bash
go test -v ./internal/component/types/... -run "TestEmptyRecordSize|TestEmptyTupleSize"
```

### Step 3: Update Record.Size() to return minimum 1

Modify `internal/component/types/composite.go`:

```go
func (r Record) Size() uint32 {
	if len(r.Fields) == 0 {
		// Per spec, empty types are not permitted.
		// Return 1 as minimum size for defensive programming.
		return 1
	}
	size := uint32(0)
	maxAlign := uint32(1)
	for _, f := range r.Fields {
		align := f.Type.Align()
		if align > maxAlign {
			maxAlign = align
		}
		// Align current offset
		size = alignTo(size, align)
		size += f.Type.Size()
	}
	// Pad to struct alignment
	return alignTo(size, maxAlign)
}
```

### Step 4: Update Tuple.Size() similarly

The Tuple.asRecord() will handle this since it delegates to Record.

### Step 5: Run test again

```bash
go test -v ./internal/component/types/... -run "TestEmptyRecordSize|TestEmptyTupleSize"
```

Expected: PASS with size = 1

### Step 6: Commit

```bash
git add internal/component/types/composite.go internal/component/types/composite_test.go
git commit -m "fix(types): ensure empty records have non-zero size

Per Canonical ABI spec lines 1930-1932, empty types are not permitted.
Return minimum size of 1 for empty records as defensive measure."
```

---

## Task 2.5: Borrow Optimization for Same Instance

**Problem:** When lowering a borrow to the same component that owns the resource, the spec says to return the rep directly instead of creating a handle.

**Spec Reference:** CanonicalABI.md lines 2677-2689

**Files:**
- Modify: `internal/component/abi/lower.go`
- Modify: `internal/component/abi/context.go`
- Test: `internal/component/abi/lower_test.go`

### Step 1: Document the optimization requirement

This optimization requires knowing the component instance that implements the resource type. The current `LowerContext` doesn't have this information.

Add to `internal/component/abi/context.go`:

```go
// LowerContext provides context for lowering operations.
type LowerContext struct {
	Memory        Memory
	Opts          *Options
	Realloc       func(oldPtr, oldSize, align, newSize uint32) (uint32, error)
	ResourceTable *component.ResourceTable
	CallContext   *component.CallContext
	// Instance is the component instance performing the lowering.
	// Used for borrow optimization when lowering to the resource's implementing instance.
	Instance      interface{} // TODO: Use proper ComponentInstance type when available
}
```

### Step 2: Add TODO comment in LowerBorrow

Modify `internal/component/abi/lower.go`:

```go
// LowerBorrow receives a borrowed resource into the component.
// Creates a borrowed handle in the table and tracks it in CallContext.
// This implements canon_lower for borrow<T> types.
//
// TODO: Per spec lines 2679-2680, when cx.inst is t.rt.impl (same instance),
// should return rep directly instead of creating handle. This optimization
// requires tracking the resource type's implementing instance.
func LowerBorrow(ctx *LowerContext, rep any) (uint32, error) {
	// ... existing implementation
```

### Step 3: Write a test documenting expected behavior

Add to `internal/component/abi/lower_test.go`:

```go
func TestLowerBorrowOptimization(t *testing.T) {
	// This test documents the expected optimization behavior.
	// Per spec lines 2679-2680, when lowering a borrow to the same
	// instance that implements the resource, return rep directly.

	table := component.NewResourceTable()
	ctx := &LowerContext{
		ResourceTable: table,
		// TODO: Set Instance to match resource's implementing instance
	}

	// Create a resource representation
	rep := uint32(42)

	// Lower as borrow
	idx, err := LowerBorrow(ctx, rep)
	if err != nil {
		t.Fatalf("LowerBorrow failed: %v", err)
	}

	t.Logf("LowerBorrow returned index %d", idx)

	// TODO: When optimization is implemented:
	// If ctx.Instance == resource's implementing instance, idx should equal rep
	// Otherwise, idx should be a new handle index
}
```

### Step 4: Commit

```bash
git add internal/component/abi/lower.go internal/component/abi/context.go internal/component/abi/lower_test.go
git commit -m "docs(abi): document borrow optimization requirement

Per Canonical ABI spec lines 2677-2689, when lowering a borrow to
the same component instance that implements the resource type,
the rep should be returned directly instead of creating a handle.

TODO: Implement when ComponentInstance tracking is available."
```

---

## Phase 2 Completion Checklist

After completing all tasks:

1. Run full type tests:
```bash
go test -v ./internal/component/types/...
```

2. Run full ABI tests:
```bash
go test -v ./internal/component/abi/...
```

3. Run regression tests:
```bash
CGO_ENABLED=0 go test -v ./internal/component/wasip2test/... -run "TestCalculatorPlugins"
```

4. Run full component tests:
```bash
go test -v ./internal/component/...
```

5. Update progress in `00-overview.md`

---

## Next Phase

Continue to [03-phase3-async-support.md](./03-phase3-async-support.md)
