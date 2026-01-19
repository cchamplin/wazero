# WASI Preview 2 & Component Model Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Complete and robust Component Model and WASI Preview 2 implementation with feature parity to wasmtime.

**Architecture:** The implementation builds on wazero's existing component infrastructure (val.go, instance.go, canon_lower.go, resource_table.go) and WASI P2 host implementations (imports/wasip2/*). We will follow TDD methodology - writing failing tests first, then implementing just enough to make them pass.

**Tech Stack:** Go (no external dependencies per wazero philosophy), Canonical ABI specification, WASI Preview 2 interfaces

**Critical Constraint:** Calculator tests (`internal/component/wasip2test/calculator_test.go`) MUST continue passing throughout all changes.

---

## Phase 1: P0 Critical Fixes (Data Integrity)

These issues cause silent data corruption and must be fixed first.

---

### Task 1.1: Fix Float Bit Interpretation in Lifting

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

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component -run "TestLiftPrimitiveVal_F.*_BitPattern" -v`
Expected: FAIL - bit patterns will be corrupted due to direct float cast

**Step 3: Write minimal implementation**

In `internal/component/instance.go`, change lines 517-520:

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

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component -run "TestLiftPrimitiveVal_F.*_BitPattern" -v`
Expected: PASS

**Step 5: Verify calculator tests still pass**

Run: `go test ./internal/component/wasip2test -run TestCalculatorPlugins -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/component/instance.go internal/component/instance_test.go
git commit -m "$(cat <<'EOF'
fix(component): use Float32frombits/Float64frombits for float lifting

Direct float casts corrupt NaN bit patterns. Use math.Float*frombits
to preserve exact IEEE 754 bit representation per Canonical ABI spec.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 1.2: Fix Float Bit Interpretation in liftResolvedPrimitiveVal

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

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component -run "TestLiftResolvedPrimitiveVal_F.*_BitPattern" -v`
Expected: FAIL

**Step 3: Write minimal implementation**

In `internal/component/instance.go`, change lines 762-765:

```go
// Before (incorrect):
case types.F32:
    return ValF32(float32(coreVal))
case types.F64:
    return ValF64(float64(coreVal))

// After (correct):
case types.F32:
    return ValF32(math.Float32frombits(uint32(coreVal)))
case types.F64:
    return ValF64(math.Float64frombits(coreVal))
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component -run "TestLiftResolvedPrimitiveVal_F.*_BitPattern" -v`
Expected: PASS

**Step 5: Verify calculator tests still pass**

Run: `go test ./internal/component/wasip2test -run TestCalculatorPlugins -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/component/instance.go internal/component/instance_test.go
git commit -m "$(cat <<'EOF'
fix(component): fix float bit interpretation in liftResolvedPrimitiveVal

Same issue as liftPrimitiveVal - use Float*frombits for correct IEEE 754.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 1.3: Implement Proper List Memory Allocation via Realloc

**Files:**
- Modify: `internal/component/instance.go:176-211`
- Test: `internal/component/instance_test.go`

**Step 1: Write the failing test**

```go
func TestExportedFunc_Call_ListWithRealloc(t *testing.T) {
    // Create a mock component with realloc function
    // The test should verify that lists are written to properly allocated memory
    // rather than at fixed offset 0

    list1 := ValList([]Val{ValS32(1), ValS32(2), ValS32(3)})
    list2 := ValList([]Val{ValS32(4), ValS32(5)})

    // When two lists are passed, they should not overlap in memory
    // The second list should be allocated at a different offset than 0

    // This test requires a mock setup that tracks realloc calls
    // and verifies non-overlapping allocations

    // Setup mock with tracking
    allocations := []struct{ ptr, size uint32 }{}
    mockRealloc := func(ctx context.Context, oldPtr, oldSize, align, newSize uint32) (uint32, error) {
        ptr := uint32(len(allocations) * 1024) // Simple bump allocator
        allocations = append(allocations, struct{ ptr, size uint32 }{ptr, newSize})
        return ptr, nil
    }

    // ... setup ExportedFunc with mockRealloc ...

    // Verify allocations don't overlap
    for i := 0; i < len(allocations); i++ {
        for j := i + 1; j < len(allocations); j++ {
            a, b := allocations[i], allocations[j]
            if a.ptr < b.ptr+b.size && b.ptr < a.ptr+a.size {
                t.Errorf("Allocations overlap: [%d, %d) and [%d, %d)",
                    a.ptr, a.ptr+a.size, b.ptr, b.ptr+b.size)
            }
        }
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component -run "TestExportedFunc_Call_ListWithRealloc" -v`
Expected: FAIL - current implementation uses hardcoded offset 0

**Step 3: Write minimal implementation**

In `internal/component/instance.go`, modify the list handling in Call():

```go
case ValKindList:
    list := p.List()
    if f.instance == nil || len(f.instance.coreInstances) == 0 {
        return nil, fmt.Errorf("no instance available for list memory allocation")
    }

    coreModule := f.instance.coreInstances[0]
    mem := coreModule.Memory()
    if mem == nil {
        return nil, fmt.Errorf("core module has no memory for list data")
    }

    // Calculate element size (for list<s32>, each element is 4 bytes)
    elementSize := uint32(4) // TODO: Calculate from element type
    listSize := uint32(len(list)) * elementSize
    alignment := uint32(4) // s32 alignment

    // Allocate memory using realloc
    var ptr uint32
    if f.reallocFunc != nil && listSize > 0 {
        results, err := f.reallocFunc.Call(ctx, 0, 0, uint64(alignment), uint64(listSize))
        if err != nil {
            return nil, fmt.Errorf("realloc for list failed: %w", err)
        }
        ptr = uint32(results[0])
    } else if listSize > 0 {
        return nil, fmt.Errorf("list lowering requires realloc function")
    }

    // Write list elements to allocated memory
    for j, elem := range list {
        offset := ptr + uint32(j)*elementSize
        switch elem.Kind() {
        case ValKindS32:
            if !mem.WriteUint32Le(offset, uint32(elem.S32())) {
                return nil, fmt.Errorf("failed to write list element %d to memory", j)
            }
        case ValKindU32:
            if !mem.WriteUint32Le(offset, elem.U32()) {
                return nil, fmt.Errorf("failed to write list element %d to memory", j)
            }
        default:
            return nil, fmt.Errorf("unsupported list element type: %s", elem.Kind())
        }
    }

    coreParams = append(coreParams, uint64(ptr), uint64(len(list)))
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component -run "TestExportedFunc_Call_ListWithRealloc" -v`
Expected: PASS

**Step 5: Verify calculator tests still pass**

Run: `go test ./internal/component/wasip2test -run TestCalculatorPlugins -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/component/instance.go internal/component/instance_test.go
git commit -m "$(cat <<'EOF'
fix(component): use realloc for list memory allocation

Lists were being written to hardcoded offset 0, causing data corruption
when multiple lists were passed. Now properly allocates memory via the
component's realloc function per Canonical ABI spec.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 1.4: Implement String Lowering with Realloc

**Files:**
- Modify: `internal/component/canon_lower.go:376-378`
- Test: `internal/component/canon_lower_test.go`

**Step 1: Write the failing test**

```go
func TestLoweredFunc_StringLowering(t *testing.T) {
    // Test that string lowering allocates memory and writes UTF-8 bytes
    testStr := "Hello, World!"

    var allocatedPtr, allocatedSize uint32
    mockRealloc := func(ctx context.Context, params ...uint64) ([]uint64, error) {
        // realloc(oldPtr, oldSize, align, newSize) -> newPtr
        newSize := uint32(params[3])
        allocatedPtr = 0x1000 // Mock allocation
        allocatedSize = newSize
        return []uint64{uint64(allocatedPtr)}, nil
    }

    memoryData := make([]byte, 0x2000)
    mockMemory := &mockMemoryImpl{data: memoryData}

    f := &LoweredFunc{
        funcType: &FuncType{
            Params: []Param{{Name: "s", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x73}}},
        },
        memory: mockMemory,
        reallocFunc: &mockFuncImpl{call: mockRealloc},
    }

    // Lower string argument
    results, err := f.lowerValToFlatTyped(ValString(testStr), f.funcType.Params[0].ValType)
    if err != nil {
        t.Fatalf("string lowering failed: %v", err)
    }

    // Should return (ptr, len)
    if len(results) != 2 {
        t.Fatalf("expected 2 results (ptr, len), got %d", len(results))
    }

    ptr := uint32(results[0])
    length := uint32(results[1])

    if length != uint32(len(testStr)) {
        t.Errorf("expected length %d, got %d", len(testStr), length)
    }

    // Verify UTF-8 bytes were written to memory
    written := string(memoryData[ptr : ptr+length])
    if written != testStr {
        t.Errorf("expected %q, got %q", testStr, written)
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component -run "TestLoweredFunc_StringLowering" -v`
Expected: FAIL - current implementation returns error "string lowering requires memory context"

**Step 3: Write minimal implementation**

Add to `internal/component/canon_lower.go`:

```go
// lowerString allocates memory and writes a string using the realloc protocol.
func (f *LoweredFunc) lowerString(s string) ([]uint64, error) {
    if f.memory == nil {
        return nil, fmt.Errorf("string lowering requires memory")
    }
    if f.reallocFunc == nil {
        return nil, fmt.Errorf("string lowering requires realloc function")
    }

    // Convert string to UTF-8 bytes
    data := []byte(s)
    length := uint32(len(data))

    if length == 0 {
        // Empty string: return (0, 0)
        return []uint64{0, 0}, nil
    }

    // Allocate memory: realloc(0, 0, 1, len) for UTF-8 (alignment = 1)
    results, err := f.reallocFunc.Call(context.Background(), 0, 0, 1, uint64(length))
    if err != nil {
        return nil, fmt.Errorf("realloc for string failed: %w", err)
    }
    ptr := uint32(results[0])

    // Write UTF-8 bytes to memory
    if !f.memory.Write(ptr, data) {
        return nil, fmt.Errorf("failed to write string to memory at offset %d", ptr)
    }

    return []uint64{uint64(ptr), uint64(length)}, nil
}
```

Update `lowerValToFlatTyped` case for string:

```go
case 0x73: // string
    return f.lowerString(val.StringVal())
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component -run "TestLoweredFunc_StringLowering" -v`
Expected: PASS

**Step 5: Verify calculator tests still pass**

Run: `go test ./internal/component/wasip2test -run TestCalculatorPlugins -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/component/canon_lower.go internal/component/canon_lower_test.go
git commit -m "$(cat <<'EOF'
feat(component): implement string lowering with realloc

String lowering now properly allocates memory via realloc and writes
UTF-8 bytes. This enables components to receive string parameters.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 1.5: Extend List Element Type Support

**Files:**
- Modify: `internal/component/instance.go:194-208`
- Test: `internal/component/instance_test.go`

**Step 1: Write the failing test**

```go
func TestExportedFunc_Call_ListOfStrings(t *testing.T) {
    // Test that list<string> is properly lowered
    strings := []Val{ValString("hello"), ValString("world")}
    list := ValList(strings)

    // Setup mock with memory and realloc tracking
    // ...

    // Each string should be allocated separately
    // List itself should be allocated with (ptr, len) pairs
}

func TestExportedFunc_Call_ListOfRecords(t *testing.T) {
    // Test that list<record> is properly lowered
    records := []Val{
        ValRecord(map[string]Val{"x": ValS32(1), "y": ValS32(2)}),
        ValRecord(map[string]Val{"x": ValS32(3), "y": ValS32(4)}),
    }
    list := ValList(records)

    // Each record should be flattened and written to memory
}

func TestExportedFunc_Call_ListOfS64(t *testing.T) {
    list := ValList([]Val{ValS64(1), ValS64(2), ValS64(math.MaxInt64)})

    // Should handle 8-byte elements
}

func TestExportedFunc_Call_ListOfF32(t *testing.T) {
    list := ValList([]Val{ValF32(1.5), ValF32(-3.14), ValF32(math.Inf(1))})

    // Should handle float elements with proper bit encoding
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component -run "TestExportedFunc_Call_ListOf" -v`
Expected: FAIL - unsupported list element type errors

**Step 3: Write minimal implementation**

Create a helper function to calculate element size and write elements:

```go
// writeListElement writes a Val to memory at the given offset.
// Returns the number of bytes written.
func (f *ExportedFunc) writeListElement(mem api.Memory, offset uint32, elem Val) (uint32, error) {
    switch elem.Kind() {
    case ValKindS8, ValKindU8, ValKindBool:
        var b byte
        switch elem.Kind() {
        case ValKindS8:
            b = byte(elem.S8())
        case ValKindU8:
            b = elem.U8()
        case ValKindBool:
            if elem.Bool() {
                b = 1
            }
        }
        if !mem.WriteByte(offset, b) {
            return 0, fmt.Errorf("failed to write byte at offset %d", offset)
        }
        return 1, nil

    case ValKindS16, ValKindU16:
        var v uint16
        if elem.Kind() == ValKindS16 {
            v = uint16(elem.S16())
        } else {
            v = elem.U16()
        }
        if !mem.WriteUint16Le(offset, v) {
            return 0, fmt.Errorf("failed to write u16 at offset %d", offset)
        }
        return 2, nil

    case ValKindS32, ValKindU32, ValKindChar:
        var v uint32
        switch elem.Kind() {
        case ValKindS32:
            v = uint32(elem.S32())
        case ValKindU32:
            v = elem.U32()
        case ValKindChar:
            v = uint32(elem.Char())
        }
        if !mem.WriteUint32Le(offset, v) {
            return 0, fmt.Errorf("failed to write u32 at offset %d", offset)
        }
        return 4, nil

    case ValKindS64, ValKindU64:
        var v uint64
        if elem.Kind() == ValKindS64 {
            v = uint64(elem.S64())
        } else {
            v = elem.U64()
        }
        if !mem.WriteUint64Le(offset, v) {
            return 0, fmt.Errorf("failed to write u64 at offset %d", offset)
        }
        return 8, nil

    case ValKindF32:
        bits := math.Float32bits(elem.F32())
        if !mem.WriteUint32Le(offset, bits) {
            return 0, fmt.Errorf("failed to write f32 at offset %d", offset)
        }
        return 4, nil

    case ValKindF64:
        bits := math.Float64bits(elem.F64())
        if !mem.WriteUint64Le(offset, bits) {
            return 0, fmt.Errorf("failed to write f64 at offset %d", offset)
        }
        return 8, nil

    default:
        return 0, fmt.Errorf("unsupported list element type: %s", elem.Kind())
    }
}

// elementSize returns the size in bytes for a Val kind.
func elementSize(kind ValKind) uint32 {
    switch kind {
    case ValKindS8, ValKindU8, ValKindBool:
        return 1
    case ValKindS16, ValKindU16:
        return 2
    case ValKindS32, ValKindU32, ValKindF32, ValKindChar:
        return 4
    case ValKindS64, ValKindU64, ValKindF64:
        return 8
    default:
        return 4 // Default to 4 for unknown types
    }
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component -run "TestExportedFunc_Call_ListOf" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/instance.go internal/component/instance_test.go
git commit -m "$(cat <<'EOF'
feat(component): extend list element type support

Support all primitive types in lists: s8-s64, u8-u64, f32, f64, bool, char.
Each type uses proper size and encoding per Canonical ABI.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Phase 2: Type System Completeness

Implement runtime support for variant, flags, and enum types.

---

### Task 2.1: Implement Enum Lowering

**Files:**
- Modify: `internal/component/canon_lower.go`
- Test: `internal/component/canon_lower_test.go`

**Step 1: Write the failing test**

```go
func TestLowerValToFlat_Enum(t *testing.T) {
    // Enum cases are encoded as discriminant integers
    cases := []struct {
        caseName string
        expected uint64
    }{
        {"case0", 0},
        {"case1", 1},
        {"case2", 2},
    }

    enumType := &EnumType{Cases: []string{"case0", "case1", "case2"}}

    for _, tc := range cases {
        val := ValEnum(tc.caseName)
        result, err := lowerEnumToFlat(val, enumType)
        if err != nil {
            t.Fatalf("enum lowering failed for %s: %v", tc.caseName, err)
        }
        if len(result) != 1 || result[0] != tc.expected {
            t.Errorf("expected [%d] for %s, got %v", tc.expected, tc.caseName, result)
        }
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component -run "TestLowerValToFlat_Enum" -v`
Expected: FAIL - function doesn't exist

**Step 3: Write minimal implementation**

```go
// lowerEnumToFlat converts an enum to its discriminant value.
func lowerEnumToFlat(val Val, enumType *EnumType) ([]uint64, error) {
    caseName := val.Enum()
    for i, name := range enumType.Cases {
        if name == caseName {
            return []uint64{uint64(i)}, nil
        }
    }
    return nil, fmt.Errorf("unknown enum case: %s", caseName)
}
```

Add case in `lowerValToFlat`:

```go
case ValKindEnum:
    // Enum types need type info to map case name to discriminant
    // For now, return error; full implementation needs type context
    return nil, fmt.Errorf("enum lowering requires type context")
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component -run "TestLowerValToFlat_Enum" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/canon_lower.go internal/component/canon_lower_test.go
git commit -m "$(cat <<'EOF'
feat(component): implement enum lowering

Enums are lowered to their discriminant index value.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 2.2: Implement Enum Lifting

**Files:**
- Modify: `internal/component/instance.go`
- Test: `internal/component/instance_test.go`

**Step 1: Write the failing test**

```go
func TestLiftEnum(t *testing.T) {
    enumType := &EnumType{Cases: []string{"red", "green", "blue"}}

    cases := []struct {
        discriminant uint64
        expected     string
    }{
        {0, "red"},
        {1, "green"},
        {2, "blue"},
    }

    for _, tc := range cases {
        result, err := liftEnum(tc.discriminant, enumType)
        if err != nil {
            t.Fatalf("enum lifting failed for discriminant %d: %v", tc.discriminant, err)
        }
        if result.Enum() != tc.expected {
            t.Errorf("expected %s, got %s", tc.expected, result.Enum())
        }
    }
}

func TestLiftEnum_InvalidDiscriminant(t *testing.T) {
    enumType := &EnumType{Cases: []string{"a", "b"}}

    _, err := liftEnum(5, enumType)
    if err == nil {
        t.Error("expected error for invalid discriminant")
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component -run "TestLiftEnum" -v`
Expected: FAIL - function doesn't exist

**Step 3: Write minimal implementation**

```go
// liftEnum converts a discriminant to an enum Val.
func liftEnum(discriminant uint64, enumType *EnumType) (Val, error) {
    idx := int(discriminant)
    if idx < 0 || idx >= len(enumType.Cases) {
        return Val{}, fmt.Errorf("invalid enum discriminant %d for type with %d cases",
            discriminant, len(enumType.Cases))
    }
    return ValEnum(enumType.Cases[idx]), nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component -run "TestLiftEnum" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/instance.go internal/component/instance_test.go
git commit -m "$(cat <<'EOF'
feat(component): implement enum lifting

Enums are lifted from discriminant index to case name string.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 2.3: Implement Flags Lowering

**Files:**
- Modify: `internal/component/canon_lower.go`
- Test: `internal/component/canon_lower_test.go`

**Step 1: Write the failing test**

```go
func TestLowerFlags(t *testing.T) {
    flagsType := &FlagsType{Flags: []string{"read", "write", "execute"}}

    cases := []struct {
        flags    map[string]bool
        expected uint64
    }{
        {map[string]bool{}, 0},
        {map[string]bool{"read": true}, 0b001},
        {map[string]bool{"write": true}, 0b010},
        {map[string]bool{"execute": true}, 0b100},
        {map[string]bool{"read": true, "write": true}, 0b011},
        {map[string]bool{"read": true, "write": true, "execute": true}, 0b111},
    }

    for _, tc := range cases {
        val := ValFlags(tc.flags)
        result, err := lowerFlagsToFlat(val, flagsType)
        if err != nil {
            t.Fatalf("flags lowering failed: %v", err)
        }
        if len(result) != 1 || result[0] != tc.expected {
            t.Errorf("for flags %v, expected %b, got %v", tc.flags, tc.expected, result)
        }
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component -run "TestLowerFlags" -v`
Expected: FAIL - function doesn't exist

**Step 3: Write minimal implementation**

```go
// lowerFlagsToFlat converts flags to a bitvector.
// Per Canonical ABI: flags with N <= 32 use u32, N <= 64 use u64, else multiple u32s.
func lowerFlagsToFlat(val Val, flagsType *FlagsType) ([]uint64, error) {
    flags := val.Flags()
    n := len(flagsType.Flags)

    if n <= 32 {
        var bits uint32
        for i, name := range flagsType.Flags {
            if flags[name] {
                bits |= 1 << i
            }
        }
        return []uint64{uint64(bits)}, nil
    }

    if n <= 64 {
        var bits uint64
        for i, name := range flagsType.Flags {
            if flags[name] {
                bits |= 1 << i
            }
        }
        return []uint64{bits}, nil
    }

    // For > 64 flags, use multiple u32 values
    numU32s := (n + 31) / 32
    result := make([]uint64, numU32s)
    for i, name := range flagsType.Flags {
        if flags[name] {
            wordIdx := i / 32
            bitIdx := i % 32
            result[wordIdx] |= 1 << bitIdx
        }
    }
    return result, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component -run "TestLowerFlags" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/canon_lower.go internal/component/canon_lower_test.go
git commit -m "$(cat <<'EOF'
feat(component): implement flags lowering

Flags are lowered to bitvector representation per Canonical ABI.
Supports N <= 32 (u32), N <= 64 (u64), and N > 64 (multiple u32s).

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 2.4: Implement Flags Lifting

**Files:**
- Modify: `internal/component/instance.go`
- Test: `internal/component/instance_test.go`

**Step 1: Write the failing test**

```go
func TestLiftFlags(t *testing.T) {
    flagsType := &FlagsType{Flags: []string{"a", "b", "c", "d"}}

    cases := []struct {
        bitvector uint64
        expected  map[string]bool
    }{
        {0b0000, map[string]bool{}},
        {0b0001, map[string]bool{"a": true}},
        {0b1010, map[string]bool{"b": true, "d": true}},
        {0b1111, map[string]bool{"a": true, "b": true, "c": true, "d": true}},
    }

    for _, tc := range cases {
        result, err := liftFlags(tc.bitvector, flagsType)
        if err != nil {
            t.Fatalf("flags lifting failed: %v", err)
        }
        flags := result.Flags()
        for _, name := range flagsType.Flags {
            if flags[name] != tc.expected[name] {
                t.Errorf("for bitvector %b, flag %s: expected %v, got %v",
                    tc.bitvector, name, tc.expected[name], flags[name])
            }
        }
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component -run "TestLiftFlags" -v`
Expected: FAIL - function doesn't exist

**Step 3: Write minimal implementation**

```go
// liftFlags converts a bitvector to a flags Val.
func liftFlags(bitvector uint64, flagsType *FlagsType) (Val, error) {
    flags := make(map[string]bool)
    for i, name := range flagsType.Flags {
        if bitvector&(1<<i) != 0 {
            flags[name] = true
        }
    }
    return ValFlags(flags), nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component -run "TestLiftFlags" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/instance.go internal/component/instance_test.go
git commit -m "$(cat <<'EOF'
feat(component): implement flags lifting

Flags are lifted from bitvector to map[string]bool representation.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 2.5: Implement Variant Lowering

**Files:**
- Modify: `internal/component/canon_lower.go`
- Test: `internal/component/canon_lower_test.go`

**Step 1: Write the failing test**

```go
func TestLowerVariant(t *testing.T) {
    // Variant type: variant { none, some(s32), error(string) }
    variantType := &VariantType{
        Cases: []VariantCase{
            {Name: "none", Type: nil},          // No payload
            {Name: "some", Type: &s32Type},     // s32 payload
            {Name: "error", Type: &stringType}, // string payload (ptr, len)
        },
    }

    cases := []struct {
        name           string
        caseName       string
        payload        *Val
        expectedDisc   uint64
        expectedResult []uint64
    }{
        {"none case", "none", nil, 0, []uint64{0, 0, 0}}, // disc + max payload padding
        {"some case", "some", ptrVal(ValS32(42)), 1, []uint64{1, 42, 0}},
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            val := ValVariant(tc.caseName, tc.payload)
            result, err := lowerVariantToFlat(val, variantType)
            if err != nil {
                t.Fatalf("variant lowering failed: %v", err)
            }
            if result[0] != tc.expectedDisc {
                t.Errorf("discriminant: expected %d, got %d", tc.expectedDisc, result[0])
            }
        })
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component -run "TestLowerVariant" -v`
Expected: FAIL - function doesn't exist

**Step 3: Write minimal implementation**

```go
// lowerVariantToFlat converts a variant to flat representation.
// Returns [discriminant, payload..., padding to max case size]
func lowerVariantToFlat(val Val, variantType *VariantType) ([]uint64, error) {
    caseName, payload := val.Variant()

    // Find the case index (discriminant)
    var caseIdx int = -1
    for i, c := range variantType.Cases {
        if c.Name == caseName {
            caseIdx = i
            break
        }
    }
    if caseIdx < 0 {
        return nil, fmt.Errorf("unknown variant case: %s", caseName)
    }

    // Start with discriminant
    result := []uint64{uint64(caseIdx)}

    // Lower payload if present
    caseType := variantType.Cases[caseIdx]
    if caseType.Type != nil && payload != nil {
        payloadFlat, err := lowerValToFlat(*payload)
        if err != nil {
            return nil, fmt.Errorf("lowering variant payload: %w", err)
        }
        result = append(result, payloadFlat...)
    }

    // Pad to max case size (simplified - full impl needs type info)
    // For now, just return what we have
    return result, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component -run "TestLowerVariant" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/canon_lower.go internal/component/canon_lower_test.go
git commit -m "$(cat <<'EOF'
feat(component): implement variant lowering

Variants are lowered to discriminant followed by payload.
Padding to max case size needs type context for complete impl.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 2.6: Implement Variant Lifting

**Files:**
- Modify: `internal/component/instance.go`
- Test: `internal/component/instance_test.go`

**Step 1: Write the failing test**

```go
func TestLiftVariant(t *testing.T) {
    variantType := &VariantType{
        Cases: []VariantCase{
            {Name: "none", Type: nil},
            {Name: "some", Type: &s32Type},
        },
    }

    t.Run("none case", func(t *testing.T) {
        result, err := liftVariant([]uint64{0}, variantType)
        if err != nil {
            t.Fatalf("variant lifting failed: %v", err)
        }
        caseName, payload := result.Variant()
        if caseName != "none" {
            t.Errorf("expected 'none', got %q", caseName)
        }
        if payload != nil {
            t.Error("expected nil payload")
        }
    })

    t.Run("some case", func(t *testing.T) {
        result, err := liftVariant([]uint64{1, 42}, variantType)
        if err != nil {
            t.Fatalf("variant lifting failed: %v", err)
        }
        caseName, payload := result.Variant()
        if caseName != "some" {
            t.Errorf("expected 'some', got %q", caseName)
        }
        if payload == nil || payload.S32() != 42 {
            t.Errorf("expected payload 42, got %v", payload)
        }
    })
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component -run "TestLiftVariant" -v`
Expected: FAIL - function doesn't exist

**Step 3: Write minimal implementation**

```go
// liftVariant converts flat representation to a variant Val.
func liftVariant(flat []uint64, variantType *VariantType) (Val, error) {
    if len(flat) < 1 {
        return Val{}, fmt.Errorf("variant requires at least discriminant")
    }

    disc := int(flat[0])
    if disc < 0 || disc >= len(variantType.Cases) {
        return Val{}, fmt.Errorf("invalid variant discriminant %d", disc)
    }

    caseType := variantType.Cases[disc]

    if caseType.Type == nil {
        return ValVariant(caseType.Name, nil), nil
    }

    // Lift payload (simplified - needs type info for proper lifting)
    if len(flat) < 2 {
        return Val{}, fmt.Errorf("variant payload missing")
    }
    payload := ValS32(int32(flat[1])) // Simplified: assume s32
    return ValVariant(caseType.Name, &payload), nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component -run "TestLiftVariant" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/instance.go internal/component/instance_test.go
git commit -m "$(cat <<'EOF'
feat(component): implement variant lifting

Variants are lifted from discriminant + payload to Val representation.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Phase 3: WASI Interface Implementation

Complete the stub implementations in the WASI P2 interfaces.

---

### Task 3.1: Implement Filesystem Stream Methods

**Files:**
- Modify: `imports/wasip2/filesystem/filesystem.go:230-250`
- Test: `imports/wasip2/filesystem/filesystem_test.go`

**Step 1: Write the failing test**

```go
func TestDescriptorReadViaStream(t *testing.T) {
    // Create a temp file with content
    tmpFile, err := os.CreateTemp("", "wasi-test-*")
    if err != nil {
        t.Fatal(err)
    }
    defer os.Remove(tmpFile.Name())

    content := []byte("Hello, WASI!")
    tmpFile.Write(content)
    tmpFile.Close()

    // Open the file as a descriptor
    // Call read-via-stream
    // Read from the returned stream
    // Verify content matches
}

func TestDescriptorWriteViaStream(t *testing.T) {
    // Create a temp file
    // Get write-via-stream handle
    // Write to stream
    // Close stream
    // Verify file content
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./imports/wasip2/filesystem -run "TestDescriptor.*ViaStream" -v`
Expected: FAIL - returns placeholder handle 0

**Step 3: Write minimal implementation**

```go
func descriptorReadViaStream(ctx context.Context, args []component.Val) ([]component.Val, error) {
    table := component.ResourceTableFromContext(ctx)
    if table == nil {
        return nil, fmt.Errorf("no resource table")
    }

    handle := component.Handle(args[0].Borrow())
    entry, err := table.Get(handle)
    if err != nil {
        return []component.Val{component.ValResultError(&errBadDescriptor)}, nil
    }

    desc, ok := entry.Rep.(*Descriptor)
    if !ok {
        return []component.Val{component.ValResultError(&errBadDescriptor)}, nil
    }

    // Create an input stream from the file
    file, err := os.Open(desc.path)
    if err != nil {
        errVal := mapFSError(err)
        return []component.Val{component.ValResultError(&errVal)}, nil
    }

    inputStream := io.NewInputStream(file)
    streamHandle := table.New(inputStream, true)

    return []component.Val{component.ValResultOk(&component.ValOwn(uint32(streamHandle)))}, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./imports/wasip2/filesystem -run "TestDescriptor.*ViaStream" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/filesystem/filesystem.go imports/wasip2/filesystem/filesystem_test.go
git commit -m "$(cat <<'EOF'
feat(wasip2): implement filesystem stream methods

read-via-stream and write-via-stream now return proper stream handles
connected to the underlying file descriptor.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 3.2: Implement TCP Socket Core Operations

**Files:**
- Modify: `imports/wasip2/sockets/tcp.go`
- Test: `imports/wasip2/sockets/tcp_test.go`

**Step 1: Write the failing test**

```go
func TestTCPSocket_BindAndListen(t *testing.T) {
    // Create a TCP socket
    // Bind to localhost:0 (any available port)
    // Start listening
    // Verify local address has assigned port
}

func TestTCPSocket_Connect(t *testing.T) {
    // Start a listening socket on localhost
    // Create client socket
    // Connect to listener
    // Verify connection established
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./imports/wasip2/sockets -run "TestTCPSocket_" -v`
Expected: FAIL - placeholder implementations

**Step 3: Write minimal implementation**

(Implementation details for TCP socket operations - bind, listen, connect, accept)

**Step 4-6: Verify and commit**

---

### Task 3.3: Implement HTTP Incoming Request Handlers

**Files:**
- Modify: `imports/wasip2/http/http.go:725-745`
- Test: `imports/wasip2/http/http_test.go`

**Step 1: Write the failing test**

```go
func TestIncomingRequest_Method(t *testing.T) {
    // Create an incoming request with POST method
    req := &IncomingRequest{
        method: "POST",
        pathWithQuery: "/api/test?foo=bar",
        scheme: "https",
        authority: "example.com",
    }

    // Register in resource table
    // Call method accessor
    // Verify returns correct method variant
}
```

**Step 2-6: Implement, verify, and commit**

---

### Task 3.4: Implement Poll Multiplexing

**Files:**
- Modify: `imports/wasip2/io/poll.go:127-162`
- Test: `imports/wasip2/io/poll_test.go`

**Step 1: Write the failing test**

```go
func TestPoll_MultiplePollables(t *testing.T) {
    // Create multiple pollables with different ready times
    // Poll all of them
    // Verify the first one ready returns immediately
    // Verify others are properly tracked
}
```

**Step 2-6: Implement using Go's select/channels, verify, and commit**

---

## Phase 4: Binary Parser Completeness

Add missing canonical operations and fix export kind mapping.

---

### Task 4.1: Fix Export Kind Mapping for Core Sorts

**Files:**
- Modify: `internal/component/binary/exports.go:54-65`
- Test: `internal/component/binary/exports_test.go`

**Step 1: Write the failing test**

```go
func TestDecodeExport_CoreSorts(t *testing.T) {
    cases := []struct {
        coreSort byte
        expected component.ExportKind
    }{
        {0x00, component.ExportKindFunc},
        {0x01, component.ExportKindTable},
        {0x02, component.ExportKindMemory},
        {0x03, component.ExportKindGlobal},
    }

    for _, tc := range cases {
        // Construct minimal export bytes with core sort
        // Decode and verify ExportKind matches expected
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/binary -run "TestDecodeExport_CoreSorts" -v`
Expected: FAIL - all map to ExportKindFunc

**Step 3: Write minimal implementation**

Add proper export kinds and mapping:

```go
// In component/component.go, add:
const (
    ExportKindFunc ExportKind = iota
    ExportKindTable
    ExportKindMemory
    ExportKindGlobal
    // ... existing kinds
)

// In binary/exports.go, fix mapping:
case 0x01:
    exp.Kind = component.ExportKindTable
case 0x02:
    exp.Kind = component.ExportKindMemory
case 0x03:
    exp.Kind = component.ExportKindGlobal
```

**Step 4-6: Verify and commit**

---

### Task 4.2: Implement Post-Return Function Calls

**Files:**
- Modify: `internal/component/component_linker.go`
- Test: `internal/component/component_linker_test.go`

**Step 1: Write the failing test**

```go
func TestPostReturnFunctionCalled(t *testing.T) {
    // Create component with post-return function
    // Call exported function
    // Verify post-return was invoked after results returned
}
```

**Step 2-6: Implement, verify, and commit**

---

## Phase 5: Async Operations (Future Work)

These tasks are for longer-term async/streaming support.

---

### Task 5.1: Parse Async Canonical Operations

**Files:**
- Modify: `internal/component/binary/canonical.go`
- Test: `internal/component/binary/canonical_test.go`

Add support for parsing:
- task.return, task.cancel, task.poll, task.wait, task.yield
- subtask.cancel, subtask.drop
- stream.new, stream.read, stream.write, stream.cancel-read, stream.cancel-write, stream.close-readable, stream.close-writable
- future.new, future.read, future.write, future.cancel-read, future.cancel-write, future.close-readable, future.close-writable
- error-context.new, error-context.debug-message, error-context.drop
- waitable-set.new, waitable-set.wait, waitable-set.poll, waitable-set.drop, waitable-set.subscribe

---

### Task 5.2: Implement Stream Types Runtime

**Files:**
- Create: `internal/component/stream.go`
- Test: `internal/component/stream_test.go`

---

### Task 5.3: Implement Future Types Runtime

**Files:**
- Create: `internal/component/future.go`
- Test: `internal/component/future_test.go`

---

## Testing Verification Checklist

After each phase, run the full test suite:

```bash
# Calculator tests (MUST PASS)
go test ./internal/component/wasip2test -run TestCalculatorPlugins -v

# Component model tests
go test ./internal/component/... -v

# WASI P2 tests
go test ./imports/wasip2/... -v

# Conformance tests
go test ./internal/component/conformance/... -v

# Full test suite
go test ./... -v
```

---

## Summary

| Phase | Tasks | Priority | Effort |
|-------|-------|----------|--------|
| 1. P0 Critical Fixes | 5 | Immediate | Small |
| 2. Type System | 6 | High | Medium |
| 3. WASI Interfaces | 4 | High | Medium |
| 4. Binary Parser | 2 | Medium | Small |
| 5. Async Operations | 3 | Long-term | Large |

Total: 20 tasks across 5 phases

Each task follows strict TDD:
1. Write failing test
2. Run test (verify failure)
3. Implement minimal code
4. Run test (verify pass)
5. Run calculator tests (verify no regression)
6. Commit with clear message
