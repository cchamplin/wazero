# Phase 1: P0 Critical Fixes (Data Integrity)

These issues cause silent data corruption and must be fixed first.

---

## Task 1.1: Fix Float Bit Interpretation in Lifting

**Status:** COMPLETED (commit `f10c036d`)

**Files:**
- Modify: `internal/component/instance.go:517-520`
- Test: `internal/component/instance_test.go`

**Step 1: Write the failing test**

```go
func TestLiftPrimitiveVal_F32_BitPattern(t *testing.T) {
    f := &ExportedFunc{}

    // Test NaN bit pattern (0x7FC00000 is a common quiet NaN)
    nanBits := uint64(0x7FC00000)
    result := f.liftPrimitiveVal(nanBits, ValTypeRef{IsPrimitive: true, Primitive: 0x76})

    resultBits := math.Float32bits(result.F32())
    if resultBits != uint32(nanBits) {
        t.Errorf("F32 bit pattern corrupted: expected 0x%08x, got 0x%08x", nanBits, resultBits)
    }
}

func TestLiftPrimitiveVal_F64_BitPattern(t *testing.T) {
    f := &ExportedFunc{}

    // Test NaN bit pattern
    nanBits := uint64(0x7FF8000000000000)
    result := f.liftPrimitiveVal(nanBits, ValTypeRef{IsPrimitive: true, Primitive: 0x75})

    resultBits := math.Float64bits(result.F64())
    if resultBits != nanBits {
        t.Errorf("F64 bit pattern corrupted: expected 0x%016x, got 0x%016x", nanBits, resultBits)
    }
}
```

**Step 2:** Run: `go test ./internal/component -run "TestLiftPrimitiveVal_F.*_BitPattern" -v`

**Step 3: Implementation**

```go
// Before (incorrect):
case 0x76: // f32
    return ValF32(float32(coreVal))
case 0x75: // f64
    return ValF64(float64(coreVal))

// After (correct):
case 0x76: // f32
    return ValF32(math.Float32frombits(uint32(coreVal)))
case 0x75: // f64
    return ValF64(math.Float64frombits(coreVal))
```

---

## Task 1.2: Fix Float Bit Interpretation in liftResolvedPrimitiveVal

**Status:** COMPLETED (commit `a1809c8f`)

**Files:**
- Modify: `internal/component/instance.go:762-765`
- Test: `internal/component/instance_test.go`

**Step 1: Write the failing test**

```go
func TestLiftResolvedPrimitiveVal_F32_BitPattern(t *testing.T) {
    f := &ExportedFunc{}

    // Test infinity bit pattern
    infBits := uint64(0x7F800000)
    result := f.liftResolvedPrimitiveVal(infBits, types.F32{})

    resultBits := math.Float32bits(result.F32())
    if resultBits != uint32(infBits) {
        t.Errorf("F32 bit pattern corrupted: expected 0x%08x, got 0x%08x", infBits, resultBits)
    }
}

func TestLiftResolvedPrimitiveVal_F64_BitPattern(t *testing.T) {
    f := &ExportedFunc{}

    // Test negative infinity
    negInfBits := uint64(0xFFF0000000000000)
    result := f.liftResolvedPrimitiveVal(negInfBits, types.F64{})

    resultBits := math.Float64bits(result.F64())
    if resultBits != negInfBits {
        t.Errorf("F64 bit pattern corrupted: expected 0x%016x, got 0x%016x", negInfBits, resultBits)
    }
}
```

**Step 2:** Run: `go test ./internal/component -run "TestLiftResolvedPrimitiveVal_F.*_BitPattern" -v`

**Step 3: Implementation**

```go
case types.F32:
    return ValF32(math.Float32frombits(uint32(coreVal)))
case types.F64:
    return ValF64(math.Float64frombits(coreVal))
```

---

## Task 1.3: Implement Proper List Memory Allocation via Realloc

**Status:** COMPLETED (commit `b14874b0`)

**Files:**
- Modify: `internal/component/instance.go:176-211`
- Modify: `internal/component/instantiate.go` (wire up reallocFunc)
- Test: `internal/component/instance_test.go`

**Key Changes:**
- List lowering now uses realloc to allocate memory instead of hardcoded offset 0
- Updated instantiate.go to properly wire up reallocFunc from canonical options
- Added tests for multiple lists, empty lists, and missing realloc error

**Additional commit:** `dc296d21` - Renamed ReadByte/WriteByte to ReadByteAt/WriteByteAt (go vet fix)

---

## Task 1.4: Implement String Lowering with Realloc

**Status:** COMPLETED (commit `f0961aad`)

**Files:**
- Modify: `internal/component/canon_lower.go:376-378`
- Test: `internal/component/canon_lower_test.go`

**Implementation:**

```go
// lowerString allocates memory and writes a string using the realloc protocol.
func (f *LoweredFunc) lowerString(s string) ([]uint64, error) {
    if f.memory == nil {
        return nil, fmt.Errorf("string lowering requires memory")
    }
    if f.reallocFunc == nil {
        return nil, fmt.Errorf("string lowering requires realloc function")
    }

    data := []byte(s)
    length := uint32(len(data))

    if length == 0 {
        return []uint64{0, 0}, nil
    }

    // Allocate memory: realloc(0, 0, 1, len) for UTF-8 (alignment = 1)
    results, err := f.reallocFunc.Call(context.Background(), 0, 0, 1, uint64(length))
    if err != nil {
        return nil, fmt.Errorf("realloc for string failed: %w", err)
    }
    ptr := uint32(results[0])

    if !f.memory.Write(ptr, data) {
        return nil, fmt.Errorf("failed to write string to memory at offset %d", ptr)
    }

    return []uint64{uint64(ptr), uint64(length)}, nil
}
```

---

## Task 1.5: Extend List Element Type Support

**Status:** COMPLETED (commit `a1a4289e`)

**Files:**
- Modify: `internal/component/instance.go:194-208`
- Test: `internal/component/instance_test.go`

**Key Changes:**
- Added `elementSizeForKind(kind ValKind) uint32` helper
- Added `alignmentForKind(kind ValKind) uint32` helper
- Added `writeListElement(mem api.Memory, offset uint32, elem Val) error` helper
- Supports all primitive types: s8-s64, u8-u64, f32, f64, bool, char
- 10 new tests covering each primitive type
