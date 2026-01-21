# Phase 5: Alignment Validation Implementation

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add alignment assertion on memory load/store operations to catch misaligned accesses.

**Architecture:** Add validation in `LiftContext` read methods and `LowerContext` write methods to verify the pointer is properly aligned for the type being accessed. Misalignment should return an error (trap in spec terms).

**Tech Stack:** Go, wazero internal component APIs

**Gap References:** GAP-BOUNDS-1 from `docs/plans/spec-canon-lift-lower-gap-analysis.md`

**Priority:** P3 (low priority, can be done anytime)

---

## Spec Reference

From `debug-vendored/component-model/design/mvp/CanonicalABI.md`:

```python
# Line 1995 - In load function:
def load(cx, ptr, t):
    assert(ptr == align_to(ptr, alignment(t)))
    assert(ptr + elem_size(t) <= len(cx.opts.memory))
    # ... load value ...

# Line 2070 - In store function:
def store(cx, v, t, ptr):
    assert(ptr == align_to(ptr, alignment(t)))
    assert(ptr + elem_size(t) <= len(cx.opts.memory))
    # ... store value ...
```

The assertion `ptr == align_to(ptr, alignment(t))` verifies that the pointer is properly aligned for the type's alignment requirement.

---

## Task 5.1: Add Alignment Validation Helper

**Files:**
- Modify: `internal/component/abi/context.go`

**Step 1: Write the failing test**

Create file: `internal/component/conformance/alignment_test.go`

```go
package conformance

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component/abi"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestAlignment_IsAligned(t *testing.T) {
	tests := []struct {
		ptr     uint32
		align   uint32
		aligned bool
	}{
		{0, 1, true},
		{0, 4, true},
		{0, 8, true},
		{4, 4, true},
		{8, 4, true},
		{8, 8, true},
		{1, 1, true},
		{1, 2, false},
		{1, 4, false},
		{2, 4, false},
		{3, 4, false},
		{5, 4, false},
		{6, 4, false},
		{7, 4, false},
		{7, 8, false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("ptr=%d_align=%d", tt.ptr, tt.align), func(t *testing.T) {
			result := abi.IsAligned(tt.ptr, tt.align)
			require.Equal(t, tt.aligned, result)
		})
	}
}

func TestAlignment_ValidateAlignment(t *testing.T) {
	t.Run("aligned_no_error", func(t *testing.T) {
		err := abi.ValidateAlignment(4, 4)
		require.NoError(t, err)
	})

	t.Run("misaligned_error", func(t *testing.T) {
		err := abi.ValidateAlignment(5, 4)
		require.Error(t, err)
		require.Contains(t, err.Error(), "alignment")
	})
}
```

**Step 2: Run test to verify it fails**

```bash
go test -v ./internal/component/conformance/... -run TestAlignment
```

Expected: FAIL with "abi.IsAligned undefined"

**Step 3: Write minimal implementation**

Add to `internal/component/abi/context.go`:

```go
// IsAligned checks if ptr is aligned to the given alignment.
// alignment must be a power of 2.
func IsAligned(ptr, alignment uint32) bool {
	if alignment == 0 || alignment == 1 {
		return true
	}
	return ptr%alignment == 0
}

// ValidateAlignment returns an error if ptr is not properly aligned.
func ValidateAlignment(ptr, alignment uint32) error {
	if !IsAligned(ptr, alignment) {
		return fmt.Errorf("misaligned memory access: ptr=%d alignment=%d", ptr, alignment)
	}
	return nil
}
```

**Step 4: Run test to verify it passes**

```bash
go test -v ./internal/component/conformance/... -run TestAlignment
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/abi/context.go internal/component/conformance/alignment_test.go
git commit -m "feat(component): add alignment validation helpers

IsAligned checks if pointer is properly aligned.
ValidateAlignment returns error for misaligned access.

Addresses GAP-BOUNDS-1 from gap analysis."
```

---

## Task 5.2: Add Alignment Check to LiftContext Reads

**Files:**
- Modify: `internal/component/abi/context.go`
- Modify: `internal/component/conformance/alignment_test.go`

**Step 1: Write the failing test**

Add to `internal/component/conformance/alignment_test.go`:

```go
func TestLiftContext_ReadAlignmentCheck(t *testing.T) {
	mem := newMockMemory(1024)

	// Write some test data at aligned offset
	binary.LittleEndian.PutUint32(mem.data[4:], 0x12345678)
	binary.LittleEndian.PutUint64(mem.data[8:], 0xDEADBEEFCAFEBABE)

	t.Run("aligned_u32_succeeds", func(t *testing.T) {
		ctx := &abi.LiftContext{Memory: mem, Opts: &abi.Options{}}
		val, err := ctx.ReadU32Aligned(4)
		require.NoError(t, err)
		require.Equal(t, uint32(0x12345678), val)
	})

	t.Run("misaligned_u32_fails", func(t *testing.T) {
		ctx := &abi.LiftContext{Memory: mem, Opts: &abi.Options{}}
		_, err := ctx.ReadU32Aligned(5)
		require.Error(t, err)
		require.Contains(t, err.Error(), "alignment")
	})

	t.Run("aligned_u64_succeeds", func(t *testing.T) {
		ctx := &abi.LiftContext{Memory: mem, Opts: &abi.Options{}}
		val, err := ctx.ReadU64Aligned(8)
		require.NoError(t, err)
		require.Equal(t, uint64(0xDEADBEEFCAFEBABE), val)
	})

	t.Run("misaligned_u64_fails", func(t *testing.T) {
		ctx := &abi.LiftContext{Memory: mem, Opts: &abi.Options{}}
		_, err := ctx.ReadU64Aligned(4) // 4 is not 8-byte aligned
		require.Error(t, err)
		require.Contains(t, err.Error(), "alignment")
	})
}
```

**Step 2: Run test to verify it fails**

```bash
go test -v ./internal/component/conformance/... -run TestLiftContext_ReadAlignmentCheck
```

Expected: FAIL with "ctx.ReadU32Aligned undefined"

**Step 3: Write minimal implementation**

Add to `internal/component/abi/context.go`:

```go
// ReadU32Aligned reads a u32 from memory with alignment validation.
func (c *LiftContext) ReadU32Aligned(offset uint32) (uint32, error) {
	if err := ValidateAlignment(offset, 4); err != nil {
		return 0, err
	}
	return c.ReadU32(offset)
}

// ReadU64Aligned reads a u64 from memory with alignment validation.
func (c *LiftContext) ReadU64Aligned(offset uint32) (uint64, error) {
	if err := ValidateAlignment(offset, 8); err != nil {
		return 0, err
	}
	return c.ReadU64(offset)
}

// ReadU16Aligned reads a u16 from memory with alignment validation.
func (c *LiftContext) ReadU16Aligned(offset uint32) (uint16, error) {
	if err := ValidateAlignment(offset, 2); err != nil {
		return 0, err
	}
	return c.ReadU16(offset)
}

// ReadF32Aligned reads a f32 from memory with alignment validation.
func (c *LiftContext) ReadF32Aligned(offset uint32) (float32, error) {
	if err := ValidateAlignment(offset, 4); err != nil {
		return 0, err
	}
	return c.ReadF32(offset)
}

// ReadF64Aligned reads a f64 from memory with alignment validation.
func (c *LiftContext) ReadF64Aligned(offset uint32) (float64, error) {
	if err := ValidateAlignment(offset, 8); err != nil {
		return 0, err
	}
	return c.ReadF64(offset)
}
```

**Step 4: Run test to verify it passes**

```bash
go test -v ./internal/component/conformance/... -run TestLiftContext_ReadAlignmentCheck
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/abi/context.go internal/component/conformance/alignment_test.go
git commit -m "feat(component): add aligned read methods to LiftContext

ReadU32Aligned, ReadU64Aligned, etc. validate alignment before reading.
Per CanonicalABI.md line 1995: assert(ptr == align_to(ptr, alignment(t)))"
```

---

## Task 5.3: Add Alignment Check to LowerContext Writes

**Files:**
- Modify: `internal/component/abi/context.go`
- Modify: `internal/component/conformance/alignment_test.go`

**Step 1: Write the failing test**

Add to `internal/component/conformance/alignment_test.go`:

```go
func TestLowerContext_WriteAlignmentCheck(t *testing.T) {
	mem := newMockMemory(1024)

	t.Run("aligned_u32_succeeds", func(t *testing.T) {
		ctx := &abi.LowerContext{Memory: mem, Opts: &abi.Options{}}
		err := ctx.WriteU32Aligned(4, 0x12345678)
		require.NoError(t, err)

		// Verify write
		v, ok := mem.ReadUint32Le(4)
		require.True(t, ok)
		require.Equal(t, uint32(0x12345678), v)
	})

	t.Run("misaligned_u32_fails", func(t *testing.T) {
		ctx := &abi.LowerContext{Memory: mem, Opts: &abi.Options{}}
		err := ctx.WriteU32Aligned(5, 0x12345678)
		require.Error(t, err)
		require.Contains(t, err.Error(), "alignment")
	})

	t.Run("aligned_u64_succeeds", func(t *testing.T) {
		ctx := &abi.LowerContext{Memory: mem, Opts: &abi.Options{}}
		err := ctx.WriteU64Aligned(8, 0xDEADBEEFCAFEBABE)
		require.NoError(t, err)
	})

	t.Run("misaligned_u64_fails", func(t *testing.T) {
		ctx := &abi.LowerContext{Memory: mem, Opts: &abi.Options{}}
		err := ctx.WriteU64Aligned(4, 0xDEADBEEFCAFEBABE)
		require.Error(t, err)
		require.Contains(t, err.Error(), "alignment")
	})
}
```

**Step 2: Run test to verify it fails**

```bash
go test -v ./internal/component/conformance/... -run TestLowerContext_WriteAlignmentCheck
```

Expected: FAIL with "ctx.WriteU32Aligned undefined"

**Step 3: Write minimal implementation**

Add to `internal/component/abi/context.go`:

```go
// WriteU32Aligned writes a u32 to memory with alignment validation.
func (c *LowerContext) WriteU32Aligned(offset, value uint32) error {
	if err := ValidateAlignment(offset, 4); err != nil {
		return err
	}
	return c.WriteU32(offset, value)
}

// WriteU64Aligned writes a u64 to memory with alignment validation.
func (c *LowerContext) WriteU64Aligned(offset uint32, value uint64) error {
	if err := ValidateAlignment(offset, 8); err != nil {
		return err
	}
	return c.WriteU64(offset, value)
}

// WriteU16Aligned writes a u16 to memory with alignment validation.
func (c *LowerContext) WriteU16Aligned(offset uint32, value uint16) error {
	if err := ValidateAlignment(offset, 2); err != nil {
		return err
	}
	return c.WriteU16(offset, value)
}

// WriteF32Aligned writes a f32 to memory with alignment validation.
func (c *LowerContext) WriteF32Aligned(offset uint32, value float32) error {
	if err := ValidateAlignment(offset, 4); err != nil {
		return err
	}
	return c.WriteF32(offset, value)
}

// WriteF64Aligned writes a f64 to memory with alignment validation.
func (c *LowerContext) WriteF64Aligned(offset uint32, value float64) error {
	if err := ValidateAlignment(offset, 8); err != nil {
		return err
	}
	return c.WriteF64(offset, value)
}

// Helper methods for LowerContext that don't exist yet
func (c *LowerContext) WriteU32(offset, value uint32) error {
	if c.Memory == nil {
		return fmt.Errorf("no memory")
	}
	if !c.Memory.WriteUint32Le(offset, value) {
		return fmt.Errorf("memory write out of bounds: offset=%d", offset)
	}
	return nil
}

func (c *LowerContext) WriteU64(offset uint32, value uint64) error {
	if c.Memory == nil {
		return fmt.Errorf("no memory")
	}
	if !c.Memory.WriteUint64Le(offset, value) {
		return fmt.Errorf("memory write out of bounds: offset=%d", offset)
	}
	return nil
}

func (c *LowerContext) WriteU16(offset uint32, value uint16) error {
	if c.Memory == nil {
		return fmt.Errorf("no memory")
	}
	if !c.Memory.WriteUint16Le(offset, value) {
		return fmt.Errorf("memory write out of bounds: offset=%d", offset)
	}
	return nil
}

func (c *LowerContext) WriteF32(offset uint32, value float32) error {
	bits := math.Float32bits(value)
	return c.WriteU32(offset, bits)
}

func (c *LowerContext) WriteF64(offset uint32, value float64) error {
	bits := math.Float64bits(value)
	return c.WriteU64(offset, bits)
}
```

**Step 4: Run test to verify it passes**

```bash
go test -v ./internal/component/conformance/... -run TestLowerContext_WriteAlignmentCheck
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/abi/context.go internal/component/conformance/alignment_test.go
git commit -m "feat(component): add aligned write methods to LowerContext

WriteU32Aligned, WriteU64Aligned, etc. validate alignment before writing.
Per CanonicalABI.md line 2070: assert(ptr == align_to(ptr, alignment(t)))"
```

---

## Task 5.4: Add Comprehensive Conformance Tests

**Files:**
- Modify: `internal/component/conformance/alignment_test.go`

**Step 1: Add comprehensive tests**

```go
package conformance

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/tetratelabs/wazero/internal/component/abi"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestAlignment_IsAlignedEdgeCases(t *testing.T) {
	t.Run("zero_alignment", func(t *testing.T) {
		require.True(t, abi.IsAligned(5, 0), "0 alignment = no requirement")
	})

	t.Run("one_alignment", func(t *testing.T) {
		require.True(t, abi.IsAligned(5, 1), "1 alignment = always aligned")
	})

	t.Run("max_uint32_aligned", func(t *testing.T) {
		// 0xFFFFFFFC is 4-byte aligned
		require.True(t, abi.IsAligned(0xFFFFFFFC, 4))
		// 0xFFFFFFF8 is 8-byte aligned
		require.True(t, abi.IsAligned(0xFFFFFFF8, 8))
	})
}

func TestAlignment_AllTypes(t *testing.T) {
	mem := newMockMemory(1024)

	// Set up test data at various offsets
	mem.data[1] = 0x42                                             // u8 at 1
	binary.LittleEndian.PutUint16(mem.data[2:], 0x1234)            // u16 at 2
	binary.LittleEndian.PutUint32(mem.data[4:], 0x12345678)        // u32 at 4
	binary.LittleEndian.PutUint64(mem.data[8:], 0xDEADBEEFCAFEBABE) // u64 at 8

	liftCtx := &abi.LiftContext{Memory: mem, Opts: &abi.Options{}}

	t.Run("u8_any_alignment", func(t *testing.T) {
		// u8 has alignment 1, any offset works
		v, err := liftCtx.ReadU8(1)
		require.NoError(t, err)
		require.Equal(t, uint8(0x42), v)
	})

	t.Run("u16_aligned", func(t *testing.T) {
		v, err := liftCtx.ReadU16Aligned(2)
		require.NoError(t, err)
		require.Equal(t, uint16(0x1234), v)
	})

	t.Run("u16_misaligned", func(t *testing.T) {
		_, err := liftCtx.ReadU16Aligned(1)
		require.Error(t, err)
	})

	t.Run("u32_aligned", func(t *testing.T) {
		v, err := liftCtx.ReadU32Aligned(4)
		require.NoError(t, err)
		require.Equal(t, uint32(0x12345678), v)
	})

	t.Run("u32_misaligned", func(t *testing.T) {
		_, err := liftCtx.ReadU32Aligned(2)
		require.Error(t, err)
	})

	t.Run("u64_aligned", func(t *testing.T) {
		v, err := liftCtx.ReadU64Aligned(8)
		require.NoError(t, err)
		require.Equal(t, uint64(0xDEADBEEFCAFEBABE), v)
	})

	t.Run("u64_misaligned", func(t *testing.T) {
		_, err := liftCtx.ReadU64Aligned(4)
		require.Error(t, err)
	})
}

func TestAlignment_BoundaryAndAlignment(t *testing.T) {
	mem := newMockMemory(64)
	liftCtx := &abi.LiftContext{Memory: mem, Opts: &abi.Options{}}

	t.Run("read_at_boundary_aligned", func(t *testing.T) {
		// Write u32 at offset 60 (last 4 bytes, 4-byte aligned)
		binary.LittleEndian.PutUint32(mem.data[60:], 0xABCDEF01)
		v, err := liftCtx.ReadU32Aligned(60)
		require.NoError(t, err)
		require.Equal(t, uint32(0xABCDEF01), v)
	})

	t.Run("read_beyond_boundary_fails_before_alignment_check", func(t *testing.T) {
		// Offset 64 is out of bounds (mem size is 64)
		// Alignment check happens first, but bounds check should also fail
		_, err := liftCtx.ReadU32Aligned(64)
		require.Error(t, err)
	})
}

func TestAlignment_WriteRoundtrip(t *testing.T) {
	mem := newMockMemory(1024)
	liftCtx := &abi.LiftContext{Memory: mem, Opts: &abi.Options{}}
	lowerCtx := &abi.LowerContext{Memory: mem, Opts: &abi.Options{}}

	t.Run("u32_roundtrip", func(t *testing.T) {
		err := lowerCtx.WriteU32Aligned(100, 0x12345678)
		require.NoError(t, err)

		v, err := liftCtx.ReadU32Aligned(100)
		require.NoError(t, err)
		require.Equal(t, uint32(0x12345678), v)
	})

	t.Run("u64_roundtrip", func(t *testing.T) {
		err := lowerCtx.WriteU64Aligned(104, 0xDEADBEEFCAFEBABE)
		require.NoError(t, err)

		v, err := liftCtx.ReadU64Aligned(104)
		require.NoError(t, err)
		require.Equal(t, uint64(0xDEADBEEFCAFEBABE), v)
	})
}

func TestAlignment_PowersOfTwo(t *testing.T) {
	// Test that alignment works correctly for all powers of 2
	alignments := []uint32{1, 2, 4, 8, 16, 32, 64}

	for _, align := range alignments {
		t.Run(fmt.Sprintf("align_%d", align), func(t *testing.T) {
			// Test multiple of alignment
			require.True(t, abi.IsAligned(align, align))
			require.True(t, abi.IsAligned(align*2, align))
			require.True(t, abi.IsAligned(align*10, align))
			require.True(t, abi.IsAligned(0, align))

			// Test non-multiple (if align > 1)
			if align > 1 {
				require.False(t, abi.IsAligned(1, align))
				require.False(t, abi.IsAligned(align+1, align))
			}
		})
	}
}
```

**Step 2: Run all tests**

```bash
go test -v ./internal/component/conformance/... -run TestAlignment
```

Expected: All PASS

**Step 3: Commit**

```bash
git add internal/component/conformance/alignment_test.go
git commit -m "test(component): add comprehensive alignment validation tests

Tests cover:
- Edge cases (zero, one alignment)
- All numeric types
- Boundary conditions
- Write/read roundtrip
- Powers of two"
```

---

## Phase 5 Regression Check

**CRITICAL:** After completing all Task 5.x, run the calculator regression test:

```bash
go test -v ./internal/component/wasip2test/... -run "TestAdd|TestSubtract"
```

**Expected:** Both tests PASS

If tests fail:
1. Check that existing code uses non-aligned reads (ReadU32 vs ReadU32Aligned)
2. The aligned variants are opt-in; existing behavior unchanged
3. Verify no accidental usage of aligned methods in existing code

---

## Phase 5 Summary

After completing Phase 5, the codebase will have:

1. `IsAligned(ptr, alignment)` helper function
2. `ValidateAlignment(ptr, alignment)` error-returning helper
3. Aligned read methods on LiftContext: `ReadU16Aligned`, `ReadU32Aligned`, `ReadU64Aligned`, `ReadF32Aligned`, `ReadF64Aligned`
4. Aligned write methods on LowerContext: `WriteU16Aligned`, `WriteU32Aligned`, `WriteU64Aligned`, `WriteF32Aligned`, `WriteF64Aligned`
5. Comprehensive test coverage

**Note:** The aligned methods are additive. Existing code can continue using non-aligned versions. Future work can migrate load/store operations to use aligned versions as appropriate.

**Files Modified:**
- `internal/component/abi/context.go` (helpers and aligned methods)
- `internal/component/conformance/alignment_test.go` (new)

---

## Plan Complete

All five phases are now documented with detailed step-by-step tasks.

**Execution Order:**
1. Phase 1: may_leave Flag (P0, CRITICAL)
2. Phase 2: Reentrance Guard (P0, CRITICAL)
3. Phase 3: Subtask Management (P1, HIGH)
4. Phase 4: Parameter Spilling (P1, HIGH)
5. Phase 5: Alignment Validation (P3, LOW)

Return to [00-root-plan.md](./00-root-plan.md) for progress tracking.
