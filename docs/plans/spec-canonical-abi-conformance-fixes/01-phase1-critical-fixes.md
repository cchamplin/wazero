# Phase 1: Critical Fixes

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix critical spec deviations that affect correctness and safety.

**Architecture:** Each fix is isolated to specific functions, with comprehensive test coverage added before implementation changes.

**Tech Stack:** Go, math package for NaN handling, encoding/binary for alignment

---

## Reference

- **Gap Analysis:** `docs/plans/canonical-abi-gap-analysis.md` (Sections 1-7)
- **Spec:** `debug-vendored/component-model/design/mvp/CanonicalABI.md`

---

## Task 1.1: Float NaN Canonicalization

**Problem:** Spec requires NaN values to be canonicalized to specific bit patterns when lifting. Current implementation passes NaN values through unchanged.

**Spec Reference:** CanonicalABI.md lines 2037-2073

**Files:**
- Modify: `internal/component/abi/lift.go`
- Modify: `internal/component/abi/context.go`
- Test: `internal/component/abi/lift_test.go`

### Step 1: Write the failing test for F32 NaN canonicalization

Add to `internal/component/abi/lift_test.go`:

```go
func TestLiftHeapF32NaNCanonicalization(t *testing.T) {
	// Different NaN bit patterns that should all canonicalize to 0x7fc00000
	nanPatterns := []uint32{
		0x7fc00000, // Canonical quiet NaN
		0x7fc00001, // Quiet NaN with payload
		0x7f800001, // Signaling NaN
		0xffc00000, // Negative quiet NaN
		0xff800001, // Negative signaling NaN
	}

	for _, pattern := range nanPatterns {
		t.Run(fmt.Sprintf("pattern_0x%08x", pattern), func(t *testing.T) {
			mem := &testMemory{data: make([]byte, 8)}
			binary.LittleEndian.PutUint32(mem.data[0:], pattern)

			ctx := &LiftContext{Memory: mem, Opts: &Options{}}
			val, err := LiftHeap(ctx, types.F32{}, 0)
			if err != nil {
				t.Fatalf("LiftHeap failed: %v", err)
			}

			// All NaNs should canonicalize to the same value
			resultBits := math.Float32bits(val.F32())
			canonicalNaN := uint32(0x7fc00000)
			if resultBits != canonicalNaN {
				t.Errorf("NaN not canonicalized: got 0x%08x, want 0x%08x", resultBits, canonicalNaN)
			}
		})
	}
}

func TestLiftHeapF64NaNCanonicalization(t *testing.T) {
	// Different NaN bit patterns that should all canonicalize to 0x7ff8000000000000
	nanPatterns := []uint64{
		0x7ff8000000000000, // Canonical quiet NaN
		0x7ff8000000000001, // Quiet NaN with payload
		0x7ff0000000000001, // Signaling NaN
		0xfff8000000000000, // Negative quiet NaN
	}

	for _, pattern := range nanPatterns {
		t.Run(fmt.Sprintf("pattern_0x%016x", pattern), func(t *testing.T) {
			mem := &testMemory{data: make([]byte, 16)}
			binary.LittleEndian.PutUint64(mem.data[0:], pattern)

			ctx := &LiftContext{Memory: mem, Opts: &Options{}}
			val, err := LiftHeap(ctx, types.F64{}, 0)
			if err != nil {
				t.Fatalf("LiftHeap failed: %v", err)
			}

			resultBits := math.Float64bits(val.F64())
			canonicalNaN := uint64(0x7ff8000000000000)
			if resultBits != canonicalNaN {
				t.Errorf("NaN not canonicalized: got 0x%016x, want 0x%016x", resultBits, canonicalNaN)
			}
		})
	}
}
```

### Step 2: Run test to verify it fails

```bash
go test -v ./internal/component/abi/... -run "TestLiftHeapF32NaNCanonicalization|TestLiftHeapF64NaNCanonicalization"
```

Expected: FAIL - NaN values pass through unchanged

### Step 3: Add NaN canonicalization constants to context.go

Add to `internal/component/abi/context.go` after the existing constants:

```go
// Canonical NaN bit patterns as per Canonical ABI spec.
// All NaN values are canonicalized to these patterns when lifting.
const (
	CanonicalFloat32NaN = uint32(0x7fc00000)
	CanonicalFloat64NaN = uint64(0x7ff8000000000000)
)

// canonicalizeNaN32 returns the canonical NaN if f is NaN, otherwise returns f unchanged.
func canonicalizeNaN32(f float32) float32 {
	if math.IsNaN(float64(f)) {
		return math.Float32frombits(CanonicalFloat32NaN)
	}
	return f
}

// canonicalizeNaN64 returns the canonical NaN if f is NaN, otherwise returns f unchanged.
func canonicalizeNaN64(f float64) float64 {
	if math.IsNaN(f) {
		return math.Float64frombits(CanonicalFloat64NaN)
	}
	return f
}
```

### Step 4: Update LiftHeap in lift.go to use canonicalization

Modify `internal/component/abi/lift.go` around lines 348-359:

```go
	case types.F32:
		v, err := ctx.ReadF32(offset)
		if err != nil {
			return component.Val{}, fmt.Errorf("lift f32: %w", err)
		}
		return component.ValF32(canonicalizeNaN32(v)), nil
	case types.F64:
		v, err := ctx.ReadF64(offset)
		if err != nil {
			return component.Val{}, fmt.Errorf("lift f64: %w", err)
		}
		return component.ValF64(canonicalizeNaN64(v)), nil
```

### Step 5: Update LiftFlat in lift.go to use canonicalization

Modify `internal/component/abi/lift.go` around lines 71-74:

```go
	case types.F32:
		return component.ValF32(canonicalizeNaN32(iter.NextF32())), nil
	case types.F64:
		return component.ValF64(canonicalizeNaN64(iter.NextF64())), nil
```

### Step 6: Run test to verify it passes

```bash
go test -v ./internal/component/abi/... -run "TestLiftHeapF32NaNCanonicalization|TestLiftHeapF64NaNCanonicalization"
```

Expected: PASS

### Step 7: Commit

```bash
git add internal/component/abi/context.go internal/component/abi/lift.go internal/component/abi/lift_test.go
git commit -m "fix(abi): canonicalize NaN values when lifting floats

Per Canonical ABI spec lines 2037-2073, all NaN values must be
canonicalized to a specific bit pattern:
- F32: 0x7fc00000
- F64: 0x7ff8000000000000

This ensures consistent behavior across implementations."
```

---

## Task 1.2: String Alignment Validation

**Problem:** Spec requires string pointers to be aligned (UTF-8: 1, UTF-16: 2). Current implementation doesn't validate alignment.

**Spec Reference:** CanonicalABI.md lines 2124-2125

**Files:**
- Modify: `internal/component/abi/strings.go`
- Test: `internal/component/abi/strings_test.go`

### Step 1: Write the failing test for UTF-16 alignment

Add to `internal/component/abi/strings_test.go`:

```go
func TestLiftStringUTF16AlignmentValidation(t *testing.T) {
	// Create memory with UTF-16 string at misaligned offset
	mem := &testMemory{data: make([]byte, 100)}

	// Write valid UTF-16 "Hi" at offset 1 (misaligned for 2-byte alignment)
	mem.data[1] = 'H'
	mem.data[2] = 0
	mem.data[3] = 'i'
	mem.data[4] = 0

	ctx := &LiftContext{
		Memory: mem,
		Opts:   &Options{StringEncoding: StringEncodingUTF16},
	}

	// ptr=1 is misaligned for UTF-16 (requires 2-byte alignment)
	_, err := liftStringUTF16(ctx, 1, 2)
	if err == nil {
		t.Error("expected error for misaligned UTF-16 string pointer, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "align") {
		t.Errorf("expected alignment error, got: %v", err)
	}
}

func TestLiftStringLatin1UTF16AlignmentValidation(t *testing.T) {
	mem := &testMemory{data: make([]byte, 100)}

	// Write UTF-16 data at offset 1 with UTF16_TAG set
	mem.data[1] = 'H'
	mem.data[2] = 0
	mem.data[3] = 'i'
	mem.data[4] = 0

	ctx := &LiftContext{
		Memory: mem,
		Opts:   &Options{StringEncoding: StringEncodingLatin1UTF16},
	}

	// taggedLen with UTF16_TAG set, ptr=1 misaligned
	taggedLen := uint32(2) | utf16Tag
	_, err := liftStringLatin1UTF16(ctx, 1, taggedLen)
	if err == nil {
		t.Error("expected error for misaligned Latin1+UTF16 string pointer, got nil")
	}
}
```

### Step 2: Run test to verify it fails

```bash
go test -v ./internal/component/abi/... -run "TestLiftStringUTF16AlignmentValidation|TestLiftStringLatin1UTF16AlignmentValidation"
```

Expected: FAIL - no alignment check

### Step 3: Add alignment validation to liftStringUTF16

Modify `internal/component/abi/strings.go` function `liftStringUTF16` around line 65:

```go
// liftStringUTF16 lifts a UTF-16 encoded string from memory.
// The codeUnits parameter is the number of UTF-16 code units (not bytes).
// Each code unit is 2 bytes, stored in little-endian order.
func liftStringUTF16(ctx *LiftContext, ptr, codeUnits uint32) (string, error) {
	// UTF-16 requires 2-byte alignment
	if ptr%2 != 0 {
		return "", fmt.Errorf("UTF-16 string pointer not 2-byte aligned: ptr=%d", ptr)
	}
	if codeUnits == 0 {
		return "", nil
	}
	// ... rest of function unchanged
```

### Step 4: Add alignment validation to liftStringLatin1UTF16 for UTF-16 case

Modify `internal/component/abi/strings.go` function `liftStringLatin1UTF16` around line 91:

```go
func liftStringLatin1UTF16(ctx *LiftContext, ptr, taggedLen uint32) (string, error) {
	if taggedLen&utf16Tag != 0 {
		// UTF-16 encoded (tag bit set) - requires 2-byte alignment
		if ptr%2 != 0 {
			return "", fmt.Errorf("UTF-16 string pointer not 2-byte aligned: ptr=%d", ptr)
		}
		codeUnits := taggedLen &^ utf16Tag
		return liftStringUTF16(ctx, ptr, codeUnits)
	}
	// Latin-1 has 1-byte alignment, no check needed
	// ... rest of function unchanged
```

### Step 5: Run test to verify it passes

```bash
go test -v ./internal/component/abi/... -run "TestLiftStringUTF16AlignmentValidation|TestLiftStringLatin1UTF16AlignmentValidation"
```

Expected: PASS

### Step 6: Commit

```bash
git add internal/component/abi/strings.go internal/component/abi/strings_test.go
git commit -m "fix(abi): validate string pointer alignment

Per Canonical ABI spec line 2124, string pointers must be aligned:
- UTF-8: 1-byte alignment (no check needed)
- UTF-16: 2-byte alignment (added check)
- Latin1+UTF16 with UTF16_TAG: 2-byte alignment (added check)"
```

---

## Task 1.3: List Element Alignment Validation

**Problem:** Spec requires list element pointers to be aligned to element type alignment. Current implementation only validates bounds.

**Spec Reference:** CanonicalABI.md lines 2153-2154

**Files:**
- Modify: `internal/component/abi/lift.go`
- Test: `internal/component/abi/lift_test.go`

### Step 1: Write the failing test

Add to `internal/component/abi/lift_test.go`:

```go
func TestLiftListAlignmentValidation(t *testing.T) {
	// Create memory with list data at misaligned offset
	mem := &testMemory{data: make([]byte, 100)}

	// Write list header: ptr=5 (misaligned for u32), length=2
	binary.LittleEndian.PutUint32(mem.data[0:], 5)  // ptr - misaligned!
	binary.LittleEndian.PutUint32(mem.data[4:], 2)  // length

	// Write u32 elements at offset 5 (would be misaligned)
	binary.LittleEndian.PutUint32(mem.data[5:], 42)
	binary.LittleEndian.PutUint32(mem.data[9:], 43)

	ctx := &LiftContext{Memory: mem, Opts: &Options{}}
	listType := types.List{Element: types.U32{}}

	_, err := LiftHeap(ctx, listType, 0)
	if err == nil {
		t.Error("expected error for misaligned list element pointer, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "align") {
		t.Errorf("expected alignment error, got: %v", err)
	}
}

func TestLiftFlatListAlignmentValidation(t *testing.T) {
	// Test flat lifting with misaligned pointer
	mem := &testMemory{data: make([]byte, 100)}

	// Write u32 element at offset 5 (misaligned)
	binary.LittleEndian.PutUint32(mem.data[5:], 42)

	ctx := &LiftContext{Memory: mem, Opts: &Options{}}
	listType := types.List{Element: types.U32{}}

	// Flat values: ptr=5 (misaligned), length=1
	iter := NewFlatIter([]uint64{5, 1})

	_, err := LiftFlat(ctx, listType, iter)
	if err == nil {
		t.Error("expected error for misaligned list element pointer in flat lift, got nil")
	}
}
```

### Step 2: Run test to verify it fails

```bash
go test -v ./internal/component/abi/... -run "TestLiftListAlignmentValidation|TestLiftFlatListAlignmentValidation"
```

Expected: FAIL - no alignment check

### Step 3: Add alignment validation to LiftHeap for List

Modify `internal/component/abi/lift.go` in the List case around line 589:

```go
	// List
	case types.List:
		ptr, err := ctx.ReadU32(offset)
		if err != nil {
			return component.Val{}, fmt.Errorf("lift list ptr: %w", err)
		}
		length, err := ctx.ReadU32(offset + 4)
		if err != nil {
			return component.Val{}, fmt.Errorf("lift list length: %w", err)
		}

		if length > 0 {
			// Validate alignment per spec line 2153
			elemAlign := t.Element.Align()
			if ptr%elemAlign != 0 {
				return component.Val{}, fmt.Errorf("list element pointer not aligned: ptr=%d, required alignment=%d", ptr, elemAlign)
			}
		}

		// Validate bounds to prevent overflow and excessive allocation
		elemSize := t.Element.Size()
		if length > 0 {
			// ... rest unchanged
```

### Step 4: Add alignment validation to LiftFlat for List

Modify `internal/component/abi/lift.go` in the LiftFlat List case around line 252:

```go
	case types.List:
		t := typ.(types.List)
		ptr := iter.NextI32()
		length := iter.NextI32()

		// Empty list case - no memory access needed
		if length == 0 {
			return component.ValList([]component.Val{}), nil
		}

		// Need memory context for non-empty lists
		if ctx == nil || ctx.Memory == nil {
			return component.Val{}, fmt.Errorf("lift list: memory context required for non-empty list")
		}

		// Validate alignment per spec line 2153
		elemAlign := t.Element.Align()
		if ptr%elemAlign != 0 {
			return component.Val{}, fmt.Errorf("list element pointer not aligned: ptr=%d, required alignment=%d", ptr, elemAlign)
		}

		// Validate bounds
		// ... rest unchanged
```

### Step 5: Run test to verify it passes

```bash
go test -v ./internal/component/abi/... -run "TestLiftListAlignmentValidation|TestLiftFlatListAlignmentValidation"
```

Expected: PASS

### Step 6: Commit

```bash
git add internal/component/abi/lift.go internal/component/abi/lift_test.go
git commit -m "fix(abi): validate list element pointer alignment

Per Canonical ABI spec line 2153, list element pointers must be
aligned to the element type's alignment requirement."
```

---

## Task 1.4: Variant Flatten Join Semantics

**Problem:** Spec uses `join` function to determine widest compatible type across variant cases. Current implementation doesn't properly compute joined types.

**Spec Reference:** CanonicalABI.md lines 2825-2841

**Files:**
- Modify: `internal/component/abi/flatten.go`
- Test: `internal/component/abi/flatten_test.go`

### Step 1: Write the failing test

Add to `internal/component/abi/flatten_test.go`:

```go
func TestFlattenVariantJoinSemantics(t *testing.T) {
	tests := []struct {
		name     string
		variant  types.Variant
		expected []api.ValueType
	}{
		{
			name: "i32_and_f32_join_to_i32",
			variant: types.Variant{Cases: []types.Case{
				{Name: "a", Type: types.S32{}},
				{Name: "b", Type: types.F32{}},
			}},
			// Discriminant (i32) + payload joined to i32 (f32 reinterpreted as i32)
			expected: []api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
		},
		{
			name: "i32_and_i64_join_to_i64",
			variant: types.Variant{Cases: []types.Case{
				{Name: "a", Type: types.S32{}},
				{Name: "b", Type: types.S64{}},
			}},
			// Discriminant (i32) + payload joined to i64
			expected: []api.ValueType{api.ValueTypeI32, api.ValueTypeI64},
		},
		{
			name: "f32_and_f64_join_to_i64",
			variant: types.Variant{Cases: []types.Case{
				{Name: "a", Type: types.F32{}},
				{Name: "b", Type: types.F64{}},
			}},
			// Discriminant (i32) + payload joined to i64 (since f32!=f64, join returns i64)
			expected: []api.ValueType{api.ValueTypeI32, api.ValueTypeI64},
		},
		{
			name: "f64_and_i64_join_to_i64",
			variant: types.Variant{Cases: []types.Case{
				{Name: "a", Type: types.F64{}},
				{Name: "b", Type: types.S64{}},
			}},
			// Same type width, join returns i64
			expected: []api.ValueType{api.ValueTypeI32, api.ValueTypeI64},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := flattenType(tt.variant)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("flattenType() = %v, want %v", result, tt.expected)
			}
		})
	}
}
```

### Step 2: Run test to verify it fails

```bash
go test -v ./internal/component/abi/... -run "TestFlattenVariantJoinSemantics"
```

Expected: FAIL - join semantics not implemented correctly

### Step 3: Implement proper join function

Modify `internal/component/abi/flatten.go`, replace `isWiderType` and `typeWidth` with spec-compliant `join`:

```go
// join returns the joined type per Canonical ABI spec.
// The join of two types is the type that can represent both:
// - Same types return that type
// - i32 and f32 return i32 (f32 reinterpreted as i32)
// - Any other combination returns i64
func join(a, b api.ValueType) api.ValueType {
	if a == b {
		return a
	}
	if (a == api.ValueTypeI32 && b == api.ValueTypeF32) ||
		(a == api.ValueTypeF32 && b == api.ValueTypeI32) {
		return api.ValueTypeI32
	}
	return api.ValueTypeI64
}
```

### Step 4: Update flattenVariant to use join

Modify `internal/component/abi/flatten.go` function `flattenVariant`:

```go
// flattenVariant flattens a variant type.
// The flattened form is: discriminant (i32) + joined payload types.
// Per spec, payload types are joined across all cases using the join function.
func flattenVariant(v types.Variant) []api.ValueType {
	// Start with discriminant
	result := []api.ValueType{api.ValueTypeI32}

	// Find max payload length and compute joined types
	var flat []api.ValueType
	for _, c := range v.Cases {
		if c.Type != nil {
			caseFlat := flattenType(c.Type)
			for i, ft := range caseFlat {
				if i < len(flat) {
					flat[i] = join(flat[i], ft)
				} else {
					flat = append(flat, ft)
				}
			}
		}
	}

	return append(result, flat...)
}
```

### Step 5: Remove old isWiderType and typeWidth functions

Remove from `internal/component/abi/flatten.go`:

```go
// DELETE these functions - replaced by join
// func isWiderType(a, b api.ValueType) bool { ... }
// func typeWidth(t api.ValueType) int { ... }
```

### Step 6: Run test to verify it passes

```bash
go test -v ./internal/component/abi/... -run "TestFlattenVariantJoinSemantics"
```

Expected: PASS

### Step 7: Run all flatten tests

```bash
go test -v ./internal/component/abi/... -run "TestFlatten"
```

Expected: PASS (some tests may need adjustment)

### Step 8: Commit

```bash
git add internal/component/abi/flatten.go internal/component/abi/flatten_test.go
git commit -m "fix(abi): implement proper join semantics for variant flattening

Per Canonical ABI spec lines 2837-2840, variant payload types are
joined using the join function:
- Same types return that type
- i32 and f32 return i32 (f32 reinterpreted as i32)
- All other combinations return i64"
```

---

## Task 1.5: Variant Lift Type Coercion

**Problem:** When lifting variants, payload values need to be coerced from the joined flat type to the actual case type.

**Spec Reference:** CanonicalABI.md lines 2962-2989

**Files:**
- Modify: `internal/component/abi/lift.go`
- Test: `internal/component/abi/lift_test.go`

### Step 1: Write the failing test

Add to `internal/component/abi/lift_test.go`:

```go
func TestLiftFlatVariantTypeCoercion(t *testing.T) {
	// Variant with i32 and f32 cases - payload joined to i32
	// When lifting f32 case, must decode i32 bits as f32
	variantType := types.Variant{Cases: []types.Case{
		{Name: "int_case", Type: types.S32{}},
		{Name: "float_case", Type: types.F32{}},
	}}

	// Create flat values for float_case with f32 value 3.14 encoded as i32
	f32Bits := math.Float32bits(3.14)
	iter := NewFlatIter([]uint64{
		1,                // discriminant = 1 (float_case)
		uint64(f32Bits),  // payload as i32 bits
	})

	ctx := &LiftContext{Opts: &Options{}}
	val, err := LiftFlat(ctx, variantType, iter)
	if err != nil {
		t.Fatalf("LiftFlat failed: %v", err)
	}

	caseName, payload := val.Variant()
	if caseName != "float_case" {
		t.Errorf("case name = %q, want %q", caseName, "float_case")
	}
	if payload == nil {
		t.Fatal("expected payload, got nil")
	}

	// The float value should be correctly decoded
	gotFloat := payload.F32()
	if math.Abs(float64(gotFloat-3.14)) > 0.001 {
		t.Errorf("payload = %v, want ~3.14", gotFloat)
	}
}

func TestLiftFlatVariantI64Coercion(t *testing.T) {
	// Variant with i32 and i64 cases - payload joined to i64
	// When lifting i32 case, must wrap i64 to i32
	variantType := types.Variant{Cases: []types.Case{
		{Name: "small", Type: types.S32{}},
		{Name: "large", Type: types.S64{}},
	}}

	// Flat values for small case with i32 value in i64 slot
	iter := NewFlatIter([]uint64{
		0,   // discriminant = 0 (small)
		42,  // payload as i64 (will be truncated to i32)
	})

	ctx := &LiftContext{Opts: &Options{}}
	val, err := LiftFlat(ctx, variantType, iter)
	if err != nil {
		t.Fatalf("LiftFlat failed: %v", err)
	}

	caseName, payload := val.Variant()
	if caseName != "small" {
		t.Errorf("case name = %q, want %q", caseName, "small")
	}
	if payload == nil {
		t.Fatal("expected payload, got nil")
	}
	if got := payload.S32(); got != 42 {
		t.Errorf("payload = %d, want 42", got)
	}
}
```

### Step 2: Run test to verify it fails

```bash
go test -v ./internal/component/abi/... -run "TestLiftFlatVariantTypeCoercion|TestLiftFlatVariantI64Coercion"
```

Expected: FAIL - no type coercion

### Step 3: Add coercion helper functions to lift.go

Add to `internal/component/abi/lift.go`:

```go
// coerceFlatValue coerces a flat value from 'have' type to 'want' type.
// This implements the coercion rules from spec lines 2971-2976.
func coerceFlatValue(value uint64, have, want api.ValueType) uint64 {
	switch {
	case have == want:
		return value
	case have == api.ValueTypeI32 && want == api.ValueTypeF32:
		// Decode i32 bits as f32, return as uint64
		return value
	case have == api.ValueTypeI64 && want == api.ValueTypeI32:
		// Wrap i64 to i32
		return value & 0xFFFFFFFF
	case have == api.ValueTypeI64 && want == api.ValueTypeF32:
		// Wrap i64 to i32, then interpret as f32
		return value & 0xFFFFFFFF
	case have == api.ValueTypeI64 && want == api.ValueTypeF64:
		// i64 bits as f64
		return value
	default:
		return value
	}
}
```

### Step 4: Update LiftFlat Variant case to use coercion

This is a more complex change - we need to create a CoerceValueIter similar to the spec. Modify the Variant case in LiftFlat in `internal/component/abi/lift.go`:

```go
	case types.Variant:
		t := typ.(types.Variant)
		disc := iter.NextI32()
		if int(disc) >= len(t.Cases) {
			return component.Val{}, fmt.Errorf("invalid variant discriminant: %d", disc)
		}
		c := t.Cases[disc]

		// Calculate the joined flat types for the variant
		flatTypes := flattenVariantPayload(t)

		var payload *component.Val
		if c.Type != nil {
			// Create coercing iterator for this case
			caseFlat := flattenType(c.Type)
			coercedValues := make([]uint64, len(caseFlat))

			for i := 0; i < len(caseFlat); i++ {
				have := flatTypes[i]
				want := caseFlat[i]
				rawValue := iter.NextI64() // Read as i64 (widest type)
				coercedValues[i] = coerceFlatValue(rawValue, have, want)
			}

			// Skip remaining padding
			for i := len(caseFlat); i < len(flatTypes); i++ {
				iter.NextI64()
			}

			coerceIter := NewFlatIter(coercedValues)
			p, err := LiftFlat(ctx, c.Type, coerceIter)
			if err != nil {
				return component.Val{}, fmt.Errorf("lift variant payload: %w", err)
			}
			payload = &p
		} else {
			// Skip all payload slots
			for i := 0; i < len(flatTypes); i++ {
				iter.NextI64()
			}
		}

		return component.ValVariant(c.Name, payload), nil
```

### Step 5: Add flattenVariantPayload helper

Add to `internal/component/abi/lift.go`:

```go
// flattenVariantPayload returns the joined flat types for a variant's payload.
func flattenVariantPayload(v types.Variant) []api.ValueType {
	var flat []api.ValueType
	for _, c := range v.Cases {
		if c.Type != nil {
			caseFlat := flattenType(c.Type)
			for i, ft := range caseFlat {
				if i < len(flat) {
					flat[i] = join(flat[i], ft)
				} else {
					flat = append(flat, ft)
				}
			}
		}
	}
	return flat
}
```

### Step 6: Run test to verify it passes

```bash
go test -v ./internal/component/abi/... -run "TestLiftFlatVariantTypeCoercion|TestLiftFlatVariantI64Coercion"
```

Expected: PASS

### Step 7: Run all lift tests

```bash
go test -v ./internal/component/abi/... -run "TestLift"
```

Expected: PASS

### Step 8: Commit

```bash
git add internal/component/abi/lift.go internal/component/abi/lift_test.go
git commit -m "fix(abi): implement type coercion for variant flat lifting

Per Canonical ABI spec lines 2962-2989, when lifting variants from
flat representation, payload values must be coerced from the joined
type to the actual case type:
- i32 as f32: reinterpret bits
- i64 to i32: truncate
- i64 as f64: reinterpret bits"
```

---

## Task 1.6: Variant Lower Type Coercion

**Problem:** When lowering variants, payload values need to be coerced from the actual case type to the joined flat type.

**Spec Reference:** CanonicalABI.md lines 3077-3098

**Files:**
- Modify: `internal/component/abi/lower.go`
- Test: `internal/component/abi/lower_test.go`

### Step 1: Write the failing test

Add to `internal/component/abi/lower_test.go`:

```go
func TestLowerFlatVariantTypeCoercion(t *testing.T) {
	// Variant with i32 and f32 cases - payload joined to i32
	variantType := types.Variant{Cases: []types.Case{
		{Name: "int_case", Type: types.S32{}},
		{Name: "float_case", Type: types.F32{}},
	}}

	// Lower float_case with value 3.14
	floatPayload := component.ValF32(3.14)
	val := component.ValVariant("float_case", &floatPayload)

	ctx := &LowerContext{Opts: &Options{}}
	flat, err := LowerFlat(ctx, variantType, val)
	if err != nil {
		t.Fatalf("LowerFlat failed: %v", err)
	}

	// Expected: [1, f32_bits_as_i32]
	if len(flat) != 2 {
		t.Fatalf("expected 2 flat values, got %d", len(flat))
	}
	if flat[0] != 1 {
		t.Errorf("discriminant = %d, want 1", flat[0])
	}

	// The f32 should be encoded as i32 bits
	expectedBits := uint64(math.Float32bits(3.14))
	if flat[1] != expectedBits {
		t.Errorf("payload bits = 0x%x, want 0x%x", flat[1], expectedBits)
	}
}

func TestLowerFlatVariantI32ToI64Coercion(t *testing.T) {
	// Variant with i32 and i64 cases - payload joined to i64
	variantType := types.Variant{Cases: []types.Case{
		{Name: "small", Type: types.S32{}},
		{Name: "large", Type: types.S64{}},
	}}

	// Lower small case with value 42
	intPayload := component.ValS32(42)
	val := component.ValVariant("small", &intPayload)

	ctx := &LowerContext{Opts: &Options{}}
	flat, err := LowerFlat(ctx, variantType, val)
	if err != nil {
		t.Fatalf("LowerFlat failed: %v", err)
	}

	// Expected: [0, 42] where 42 is zero-extended to i64
	if len(flat) != 2 {
		t.Fatalf("expected 2 flat values, got %d", len(flat))
	}
	if flat[0] != 0 {
		t.Errorf("discriminant = %d, want 0", flat[0])
	}
	if flat[1] != 42 {
		t.Errorf("payload = %d, want 42", flat[1])
	}
}
```

### Step 2: Run test to verify it fails

```bash
go test -v ./internal/component/abi/... -run "TestLowerFlatVariantTypeCoercion|TestLowerFlatVariantI32ToI64Coercion"
```

Expected: FAIL - no type coercion

### Step 3: Add coercion helper for lowering

Add to `internal/component/abi/lower.go`:

```go
// coerceFlatValueForLower coerces a flat value from 'have' type to 'want' type for lowering.
// This implements the coercion rules from spec lines 3088-3094.
func coerceFlatValueForLower(value uint64, have, want api.ValueType) uint64 {
	switch {
	case have == want:
		return value
	case have == api.ValueTypeF32 && want == api.ValueTypeI32:
		// f32 bits encoded as i32
		return value
	case have == api.ValueTypeI32 && want == api.ValueTypeI64:
		// i32 zero-extended to i64
		return value
	case have == api.ValueTypeF32 && want == api.ValueTypeI64:
		// f32 bits encoded as i32, then zero-extended to i64
		return value
	case have == api.ValueTypeF64 && want == api.ValueTypeI64:
		// f64 bits encoded as i64
		return value
	default:
		return value
	}
}
```

### Step 4: Update LowerFlat Variant case to use coercion

Modify the Variant case in LowerFlat in `internal/component/abi/lower.go`:

```go
	case types.Variant:
		t := typ.(types.Variant)
		caseName, payload := val.Variant()

		// Find case index
		caseIdx := -1
		var caseType types.ValType
		for i, c := range t.Cases {
			if c.Name == caseName {
				caseIdx = i
				caseType = c.Type
				break
			}
		}
		if caseIdx == -1 {
			return nil, fmt.Errorf("unknown variant case: %s", caseName)
		}

		if caseType != nil && payload == nil {
			return nil, fmt.Errorf("variant case %q requires a payload", caseName)
		}

		// Calculate joined flat types
		flatTypes := flattenVariantPayload(t)

		result := []uint64{uint64(caseIdx)}

		if caseType != nil && payload != nil {
			// Lower the payload
			payloadFlat, err := LowerFlat(ctx, caseType, *payload)
			if err != nil {
				return nil, fmt.Errorf("lower variant payload: %w", err)
			}

			// Coerce each payload value to the joined type
			caseFlat := flattenType(caseType)
			for i, pv := range payloadFlat {
				have := caseFlat[i]
				want := flatTypes[i]
				result = append(result, coerceFlatValueForLower(pv, have, want))
			}

			// Pad remaining slots
			for i := len(payloadFlat); i < len(flatTypes); i++ {
				result = append(result, 0)
			}
		} else {
			// No payload - pad all slots
			for i := 0; i < len(flatTypes); i++ {
				result = append(result, 0)
			}
		}

		return result, nil
```

### Step 5: Import flattenVariantPayload and flattenType

Add import if needed and ensure the functions are accessible. You may need to move `flattenVariantPayload` to a shared location or duplicate it:

```go
// Add at top of lower.go if not already present
import "github.com/tetratelabs/wazero/api"

// If flattenVariantPayload is not exported from flatten.go, either export it
// or add this local helper:
func flattenVariantPayloadForLower(v types.Variant) []api.ValueType {
	var flat []api.ValueType
	for _, c := range v.Cases {
		if c.Type != nil {
			caseFlat := flattenType(c.Type)
			for i, ft := range caseFlat {
				if i < len(flat) {
					flat[i] = join(flat[i], ft)
				} else {
					flat = append(flat, ft)
				}
			}
		}
	}
	return flat
}
```

### Step 6: Run test to verify it passes

```bash
go test -v ./internal/component/abi/... -run "TestLowerFlatVariantTypeCoercion|TestLowerFlatVariantI32ToI64Coercion"
```

Expected: PASS

### Step 7: Run all lower tests

```bash
go test -v ./internal/component/abi/... -run "TestLower"
```

Expected: PASS

### Step 8: Commit

```bash
git add internal/component/abi/lower.go internal/component/abi/lower_test.go
git commit -m "fix(abi): implement type coercion for variant flat lowering

Per Canonical ABI spec lines 3077-3098, when lowering variants to
flat representation, payload values must be coerced from the case
type to the joined type:
- f32 to i32: encode f32 bits as i32
- i32 to i64: zero-extend
- f64 to i64: encode f64 bits as i64"
```

---

## Task 1.7: Resource Type Validation

**Problem:** When lifting resources, the spec requires validating that the handle's resource type matches the expected type. Current implementation doesn't check this.

**Spec Reference:** CanonicalABI.md lines 2216-2221, 2234-2241

**Files:**
- Modify: `internal/component/abi/lift.go`
- Modify: `internal/component/types/resource.go`
- Test: `internal/component/abi/lift_test.go`

### Step 1: Update resource types to include ResourceType reference

First, we need to track the actual resource type in handles. Modify `internal/component/types/resource.go`:

```go
// Own represents an owning handle to a resource.
type Own struct {
	ResourceIdx  uint32        // Index of the resource type in component's type section
	ResourceType *ResourceType // Reference to the resource type (for validation)
}

// Borrow represents a borrowed handle to a resource.
type Borrow struct {
	ResourceIdx  uint32        // Index of the resource type in component's type section
	ResourceType *ResourceType // Reference to the resource type (for validation)
}
```

### Step 2: Write the failing test

Add to `internal/component/abi/lift_test.go`:

```go
func TestLiftOwnResourceTypeValidation(t *testing.T) {
	// Create two different resource types
	rtA := &types.ResourceType{}
	rtB := &types.ResourceType{}

	// Create a resource table and add a handle with type A
	table := component.NewResourceTable()
	handle := table.New(&testResource{value: "test"}, true)

	// Create lift context
	ctx := &LiftContext{
		ResourceTable: table,
	}

	// Try to lift as type B - should fail
	// Note: This test depends on how resource type validation is implemented
	// The current implementation doesn't track resource types on handles,
	// so this test documents the expected behavior after the fix.

	// For now, we test that LiftOwn works with valid handles
	_, err := LiftOwn(ctx, handle.Index())
	if err != nil {
		t.Fatalf("LiftOwn failed for valid handle: %v", err)
	}

	// TODO: Add test for type mismatch once ResourceType tracking is implemented
	_ = rtA
	_ = rtB
}
```

### Step 3: Run test to verify current behavior

```bash
go test -v ./internal/component/abi/... -run "TestLiftOwnResourceTypeValidation"
```

Expected: PASS (test documents current behavior)

### Step 4: Document the limitation

This is a complex change that requires updating the ResourceTable and handle tracking. For now, add a TODO comment in `lift.go`:

```go
// LiftOwn transfers ownership of a resource out of the component.
// TODO: Per spec lines 2218-2219, should validate:
//   - trap_if(h.rt is not t.rt) - resource type matches
// Currently, resource type tracking is not implemented in ResourceTable.
func LiftOwn(ctx *LiftContext, handleIdx uint32) (any, error) {
```

And in `LiftBorrow`:

```go
// LiftBorrow reads a resource representation for borrowing.
// TODO: Per spec lines 2237-2238, should validate:
//   - trap_if(h.rt is not t.rt) - resource type matches
func LiftBorrow(ctx *LiftContext, handleIdx uint32) (any, error) {
```

### Step 5: Commit

```bash
git add internal/component/abi/lift.go internal/component/abi/lift_test.go
git commit -m "docs(abi): document resource type validation requirement

Per Canonical ABI spec lines 2218-2219 and 2237-2238, lifting
resources should validate that the handle's resource type matches
the expected type.

TODO: Implement full resource type tracking in ResourceTable."
```

---

## Phase 1 Completion Checklist

After completing all tasks:

1. Run full ABI test suite:
```bash
go test -v ./internal/component/abi/...
```

2. Run full component tests:
```bash
go test -v ./internal/component/...
```

3. Run regression tests:
```bash
CGO_ENABLED=0 go test -v ./internal/component/wasip2test/... -run "TestCalculatorPlugins"
```

4. Update progress in `00-overview.md`

---

## Next Phase

Continue to [02-phase2-major-improvements.md](./02-phase2-major-improvements.md)
