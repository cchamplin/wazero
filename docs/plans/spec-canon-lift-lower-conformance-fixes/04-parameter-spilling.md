# Phase 4: Parameter Spilling Implementation

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement parameter spilling for functions with more than MAX_FLAT_PARAMS (16) flattened parameters.

**Architecture:** When a function has more than 16 flat parameters, instead of passing them directly, allocate memory via realloc, store the parameters there, and pass a single i32 pointer. On the lift side, read parameters from memory instead of from the flat args.

**Tech Stack:** Go, wazero internal component APIs, Canonical ABI memory layout

**Gap References:** GAP-CALL-1, GAP-LIFT-3 from `docs/plans/spec-canon-lift-lower-gap-analysis.md`

**Prerequisites:** Phase 3 (Subtask) should be completed for proper allocation tracking.

---

## Spec Reference

From `debug-vendored/component-model/design/mvp/CanonicalABI.md`:

```python
# Lines 2735-2744 - MAX_FLAT_PARAMS constant and flattening:
MAX_FLAT_PARAMS = 16
MAX_FLAT_RESULTS = 1

def flatten_functype(opts, ft, context):
    flat_params = flatten_types(ft.param_types())
    if len(flat_params) > MAX_FLAT_PARAMS:
        flat_params = ['i32']  # Pointer to params in memory

# Lines 2950-2960 - Lowering params when spilled:
def lower_flat_values(cx, max_flat, vs, ts, out_param = None):
    flat_vals = []
    for i, (v, t) in enumerate(zip(vs, ts)):
        flat_vals += lower_flat(cx, v, t)

    if len(flat_vals) > max_flat:
        tuple_type = TupleType(ts)
        ptr = cx.opts.realloc(0, 0, alignment(tuple_type), elem_size(tuple_type))
        store(cx, Tuple(vs), tuple_type, ptr)
        flat_vals = [ptr]

    return flat_vals
```

---

## Task 4.1: Define MAX_FLAT_PARAMS Constant

**Files:**
- Modify: `internal/component/abi/flatten.go`

**Step 1: Write the failing test**

Create file: `internal/component/conformance/param_spilling_test.go`

```go
package conformance

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component/abi"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestMaxFlatParams_Constant(t *testing.T) {
	require.Equal(t, 16, abi.MaxFlatParams, "MAX_FLAT_PARAMS should be 16 per spec")
}
```

**Step 2: Run test to verify it fails**

```bash
go test -v ./internal/component/conformance/... -run TestMaxFlatParams_Constant
```

Expected: FAIL with "abi.MaxFlatParams undefined"

**Step 3: Write minimal implementation**

Add to `internal/component/abi/flatten.go`:

```go
// MaxFlatParams is the maximum number of flattened parameters that can be
// passed directly. Beyond this, params are passed via a memory pointer.
// Per Canonical ABI spec, this is 16.
const MaxFlatParams = 16

// MaxFlatResults is the maximum number of flattened results that can be
// returned directly. Beyond this, results are returned via a memory pointer.
// Per Canonical ABI spec, this is 1.
const MaxFlatResults = 1
```

Note: `MaxFlatResults` may already exist in `component_linker.go` - consolidate if needed.

**Step 4: Run test to verify it passes**

```bash
go test -v ./internal/component/conformance/... -run TestMaxFlatParams_Constant
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/abi/flatten.go internal/component/conformance/param_spilling_test.go
git commit -m "feat(component): define MaxFlatParams constant

MaxFlatParams = 16 per CanonicalABI.md line 2735.
Functions with more flattened params use memory pointer instead.

Addresses GAP-LIFT-3 from gap analysis."
```

---

## Task 4.2: Detect Parameter Spilling Condition

**Files:**
- Modify: `internal/component/abi/flatten.go`
- Modify: `internal/component/conformance/param_spilling_test.go`

**Step 1: Write the failing test**

Add to `internal/component/conformance/param_spilling_test.go`:

```go
func TestFlattenParams_NeedsSpilling(t *testing.T) {
	t.Run("under_16_no_spilling", func(t *testing.T) {
		// 4 params of u32 = 4 flat values
		paramTypes := []types.ValType{
			types.U32{}, types.U32{}, types.U32{}, types.U32{},
		}
		flat, needsSpilling := abi.FlattenParamsWithSpilling(paramTypes)
		require.False(t, needsSpilling)
		require.Equal(t, 4, len(flat))
	})

	t.Run("exactly_16_no_spilling", func(t *testing.T) {
		// 16 u32 params = 16 flat values (exactly at limit)
		paramTypes := make([]types.ValType, 16)
		for i := range paramTypes {
			paramTypes[i] = types.U32{}
		}
		flat, needsSpilling := abi.FlattenParamsWithSpilling(paramTypes)
		require.False(t, needsSpilling)
		require.Equal(t, 16, len(flat))
	})

	t.Run("over_16_needs_spilling", func(t *testing.T) {
		// 17 u32 params = 17 flat values (over limit)
		paramTypes := make([]types.ValType, 17)
		for i := range paramTypes {
			paramTypes[i] = types.U32{}
		}
		flat, needsSpilling := abi.FlattenParamsWithSpilling(paramTypes)
		require.True(t, needsSpilling)
		require.Equal(t, 1, len(flat), "should return single i32 pointer")
	})

	t.Run("record_causes_spilling", func(t *testing.T) {
		// Single record with 17 fields = 17 flat values
		fields := make([]types.Field, 17)
		for i := range fields {
			fields[i] = types.Field{Name: fmt.Sprintf("f%d", i), Type: types.U32{}}
		}
		paramTypes := []types.ValType{types.Record{Fields: fields}}
		flat, needsSpilling := abi.FlattenParamsWithSpilling(paramTypes)
		require.True(t, needsSpilling)
	})
}
```

**Step 2: Run test to verify it fails**

```bash
go test -v ./internal/component/conformance/... -run TestFlattenParams_NeedsSpilling
```

Expected: FAIL with "abi.FlattenParamsWithSpilling undefined"

**Step 3: Write minimal implementation**

Add to `internal/component/abi/flatten.go`:

```go
// FlattenParamsWithSpilling flattens parameter types and detects if spilling is needed.
// Returns the flat types and whether a memory pointer should be used instead.
// If needsSpilling is true, the returned flat types will be [i32] for the pointer.
func FlattenParamsWithSpilling(paramTypes []types.ValType) ([]api.ValueType, bool) {
	flat := FlattenParams(paramTypes)

	if len(flat) > MaxFlatParams {
		// Params must be passed via memory pointer
		return []api.ValueType{api.ValueTypeI32}, true
	}

	return flat, false
}
```

**Step 4: Run test to verify it passes**

```bash
go test -v ./internal/component/conformance/... -run TestFlattenParams_NeedsSpilling
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/abi/flatten.go internal/component/conformance/param_spilling_test.go
git commit -m "feat(component): detect parameter spilling condition

FlattenParamsWithSpilling returns whether params exceed MAX_FLAT_PARAMS.
When true, params should be passed via single i32 memory pointer."
```

---

## Task 4.3: Calculate Tuple Memory Layout

**Files:**
- Modify: `internal/component/abi/flatten.go` or create `internal/component/abi/layout.go`
- Modify: `internal/component/conformance/param_spilling_test.go`

**Step 1: Write the failing test**

Add to `internal/component/conformance/param_spilling_test.go`:

```go
func TestTupleLayout_SizeAndAlignment(t *testing.T) {
	t.Run("single_u32", func(t *testing.T) {
		tupleTypes := []types.ValType{types.U32{}}
		size, align := abi.TupleLayout(tupleTypes)
		require.Equal(t, uint32(4), size)
		require.Equal(t, uint32(4), align)
	})

	t.Run("u32_u64", func(t *testing.T) {
		tupleTypes := []types.ValType{types.U32{}, types.U64{}}
		size, align := abi.TupleLayout(tupleTypes)
		// u32 at 0-3, padding 4-7, u64 at 8-15 = size 16, align 8
		require.Equal(t, uint32(16), size)
		require.Equal(t, uint32(8), align)
	})

	t.Run("u8_u32_u8", func(t *testing.T) {
		tupleTypes := []types.ValType{types.U8{}, types.U32{}, types.U8{}}
		size, align := abi.TupleLayout(tupleTypes)
		// u8 at 0, padding 1-3, u32 at 4-7, u8 at 8 = size 9, padded to 12 (align 4)
		require.Equal(t, uint32(12), size)
		require.Equal(t, uint32(4), align)
	})

	t.Run("17_u32s", func(t *testing.T) {
		tupleTypes := make([]types.ValType, 17)
		for i := range tupleTypes {
			tupleTypes[i] = types.U32{}
		}
		size, align := abi.TupleLayout(tupleTypes)
		require.Equal(t, uint32(68), size, "17 * 4 = 68")
		require.Equal(t, uint32(4), align)
	})
}
```

**Step 2: Run test to verify it fails**

```bash
go test -v ./internal/component/conformance/... -run TestTupleLayout
```

Expected: FAIL with "abi.TupleLayout undefined"

**Step 3: Write minimal implementation**

Add to `internal/component/abi/flatten.go` (or new file `layout.go`):

```go
// TupleLayout calculates the size and alignment for a tuple of types.
// This follows the Canonical ABI layout rules for storing tuples in memory.
func TupleLayout(elemTypes []types.ValType) (size, alignment uint32) {
	if len(elemTypes) == 0 {
		return 0, 1
	}

	var offset uint32
	var maxAlign uint32 = 1

	for _, t := range elemTypes {
		elemSize := SizeOf(t)
		elemAlign := AlignOf(t)

		// Update max alignment
		if elemAlign > maxAlign {
			maxAlign = elemAlign
		}

		// Align offset for this element
		offset = alignTo(offset, elemAlign)

		// Add element size
		offset += elemSize
	}

	// Final size is padded to tuple alignment
	size = alignTo(offset, maxAlign)
	alignment = maxAlign

	return size, alignment
}

// SizeOf returns the byte size of a type per Canonical ABI.
func SizeOf(t types.ValType) uint32 {
	switch t.(type) {
	case types.Bool, types.S8, types.U8:
		return 1
	case types.S16, types.U16:
		return 2
	case types.S32, types.U32, types.F32, types.Char:
		return 4
	case types.S64, types.U64, types.F64:
		return 8
	case types.String, types.List:
		return 8 // ptr (4) + len (4)
	case types.Own, types.Borrow:
		return 4 // handle index
	case types.Record:
		rec := t.(types.Record)
		size, _ := TupleLayout(recordToTypes(rec))
		return size
	case types.Tuple:
		tup := t.(types.Tuple)
		size, _ := TupleLayout(tup.Types)
		return size
	case types.Variant:
		// Discriminant + max payload
		v := t.(types.Variant)
		return variantSize(v)
	case types.Option:
		// Discriminant (i32) + payload
		o := t.(types.Option)
		if o.Some == nil {
			return 4
		}
		return 4 + alignTo(SizeOf(o.Some), 4)
	case types.Result:
		r := t.(types.Result)
		return resultSize(r)
	case types.Enum:
		return 4 // discriminant
	case types.Flags:
		f := t.(types.Flags)
		return uint32((len(f.Names) + 31) / 32 * 4)
	default:
		return 4
	}
}

// AlignOf returns the alignment of a type per Canonical ABI.
func AlignOf(t types.ValType) uint32 {
	switch t.(type) {
	case types.Bool, types.S8, types.U8:
		return 1
	case types.S16, types.U16:
		return 2
	case types.S32, types.U32, types.F32, types.Char:
		return 4
	case types.S64, types.U64, types.F64:
		return 8
	case types.String, types.List:
		return 4 // alignment of ptr
	case types.Own, types.Borrow:
		return 4
	case types.Record:
		rec := t.(types.Record)
		_, align := TupleLayout(recordToTypes(rec))
		return align
	case types.Tuple:
		tup := t.(types.Tuple)
		_, align := TupleLayout(tup.Types)
		return align
	case types.Variant:
		v := t.(types.Variant)
		return variantAlign(v)
	case types.Option:
		o := t.(types.Option)
		if o.Some == nil {
			return 4
		}
		payloadAlign := AlignOf(o.Some)
		if payloadAlign < 4 {
			return 4
		}
		return payloadAlign
	case types.Result:
		r := t.(types.Result)
		return resultAlign(r)
	case types.Enum, types.Flags:
		return 4
	default:
		return 4
	}
}

// alignTo rounds up offset to the given alignment.
func alignTo(offset, align uint32) uint32 {
	return (offset + align - 1) &^ (align - 1)
}

func recordToTypes(r types.Record) []types.ValType {
	result := make([]types.ValType, len(r.Fields))
	for i, f := range r.Fields {
		result[i] = f.Type
	}
	return result
}

func variantSize(v types.Variant) uint32 {
	discSize := uint32(4) // Always i32 for simplicity
	var maxPayload uint32
	for _, c := range v.Cases {
		if c.Type != nil {
			if s := SizeOf(c.Type); s > maxPayload {
				maxPayload = s
			}
		}
	}
	return discSize + alignTo(maxPayload, variantAlign(v))
}

func variantAlign(v types.Variant) uint32 {
	align := uint32(4) // discriminant
	for _, c := range v.Cases {
		if c.Type != nil {
			if a := AlignOf(c.Type); a > align {
				align = a
			}
		}
	}
	return align
}

func resultSize(r types.Result) uint32 {
	discSize := uint32(4)
	var maxPayload uint32
	if r.Ok != nil {
		maxPayload = SizeOf(r.Ok)
	}
	if r.Error != nil {
		if s := SizeOf(r.Error); s > maxPayload {
			maxPayload = s
		}
	}
	return discSize + alignTo(maxPayload, resultAlign(r))
}

func resultAlign(r types.Result) uint32 {
	align := uint32(4)
	if r.Ok != nil {
		if a := AlignOf(r.Ok); a > align {
			align = a
		}
	}
	if r.Error != nil {
		if a := AlignOf(r.Error); a > align {
			align = a
		}
	}
	return align
}
```

**Step 4: Run test to verify it passes**

```bash
go test -v ./internal/component/conformance/... -run TestTupleLayout
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/abi/flatten.go internal/component/conformance/param_spilling_test.go
git commit -m "feat(component): implement TupleLayout for memory layout calculation

TupleLayout calculates size and alignment for storing params in memory.
SizeOf and AlignOf implement Canonical ABI type sizing rules."
```

---

## Task 4.4: Implement Parameter Store to Memory

**Files:**
- Modify: `internal/component/abi/lower.go`
- Modify: `internal/component/conformance/param_spilling_test.go`

**Step 1: Write the failing test**

Add to `internal/component/conformance/param_spilling_test.go`:

```go
func TestStoreParamsToMemory(t *testing.T) {
	mem := newMockMemory(4096)
	allocPtr := uint32(256)

	ctx := &abi.LowerContext{
		Memory: mem,
		Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			result := allocPtr
			allocPtr += newSize
			return result, nil
		},
	}

	t.Run("store_simple_params", func(t *testing.T) {
		allocPtr = 256
		paramTypes := []types.ValType{types.U32{}, types.U32{}, types.U32{}}
		params := []component.Val{
			component.ValU32(0x11111111),
			component.ValU32(0x22222222),
			component.ValU32(0x33333333),
		}

		ptr, err := abi.StoreParamsToMemory(ctx, paramTypes, params)
		require.NoError(t, err)
		require.Equal(t, uint32(256), ptr)

		// Verify memory contents
		v1, ok := mem.ReadUint32Le(256)
		require.True(t, ok)
		require.Equal(t, uint32(0x11111111), v1)

		v2, ok := mem.ReadUint32Le(260)
		require.True(t, ok)
		require.Equal(t, uint32(0x22222222), v2)

		v3, ok := mem.ReadUint32Le(264)
		require.True(t, ok)
		require.Equal(t, uint32(0x33333333), v3)
	})
}
```

**Step 2: Run test to verify it fails**

```bash
go test -v ./internal/component/conformance/... -run TestStoreParamsToMemory
```

Expected: FAIL with "abi.StoreParamsToMemory undefined"

**Step 3: Write minimal implementation**

Add to `internal/component/abi/lower.go`:

```go
// StoreParamsToMemory allocates memory and stores spilled parameters.
// This is used when flattened params exceed MAX_FLAT_PARAMS.
// Returns the pointer to the stored parameters.
func StoreParamsToMemory(ctx *LowerContext, paramTypes []types.ValType, params []Val) (uint32, error) {
	if ctx.Realloc == nil {
		return 0, fmt.Errorf("StoreParamsToMemory requires realloc")
	}

	// Calculate tuple layout
	size, align := TupleLayout(paramTypes)

	// Allocate memory
	ptr, err := ctx.Realloc(0, 0, align, size)
	if err != nil {
		return 0, fmt.Errorf("realloc for params: %w", err)
	}

	// Store each parameter at its offset
	offset := ptr
	for i, t := range paramTypes {
		elemAlign := AlignOf(t)
		offset = alignTo(offset, elemAlign)

		if err := LowerHeap(ctx, t, params[i], offset); err != nil {
			return 0, fmt.Errorf("store param %d: %w", i, err)
		}

		offset += SizeOf(t)
	}

	return ptr, nil
}
```

**Step 4: Run test to verify it passes**

```bash
go test -v ./internal/component/conformance/... -run TestStoreParamsToMemory
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/abi/lower.go internal/component/conformance/param_spilling_test.go
git commit -m "feat(component): implement StoreParamsToMemory for spilled params

Allocates memory via realloc and stores parameters according to
Canonical ABI tuple layout rules."
```

---

## Task 4.5: Integrate Spilling into Call Path

**Files:**
- Modify: `internal/component/instance.go`

**Step 1: Identify integration point**

In `ExportedFunc.Call`, after flattening parameters, check if spilling is needed:

```go
func (f *ExportedFunc) Call(ctx context.Context, params ...Val) ([]Val, error) {
	// ... existing setup ...

	// Flatten and check for spilling
	flatTypes := abi.FlattenParams(paramTypes)
	needsSpilling := len(flatTypes) > abi.MaxFlatParams

	var coreParams []uint64

	if needsSpilling {
		// Allocate and store params in memory
		ptr, err := abi.StoreParamsToMemory(lowerCtx, paramTypes, params)
		if err != nil {
			return nil, fmt.Errorf("spill params: %w", err)
		}
		coreParams = []uint64{uint64(ptr)}
	} else {
		// Flatten params normally
		for _, p := range params {
			// ... existing flattening ...
		}
	}

	// ... call core function ...
}
```

**Step 2: Add parameter type resolution**

The implementation needs to know the component-level parameter types. This should come from `f.funcType`.

```go
func (f *ExportedFunc) Call(ctx context.Context, params ...Val) ([]Val, error) {
	// Get param types for spilling check
	var paramTypes []types.ValType
	if f.funcType != nil {
		paramTypes = make([]types.ValType, len(f.funcType.Params))
		resolver := NewTypeResolver(f.component)
		for i, p := range f.funcType.Params {
			vt, err := resolver.ResolveValType(p.ValType)
			if err == nil {
				paramTypes[i] = vt
			} else {
				// Fallback to inferring from Val
				paramTypes[i] = inferTypeFromVal(params[i])
			}
		}
	}

	// Check for spilling
	if paramTypes != nil {
		flatTypes := abi.FlattenParams(paramTypes)
		if len(flatTypes) > abi.MaxFlatParams {
			// Create lower context
			lowerCtx := &abi.LowerContext{
				Memory:  f.memory,
				Opts:    &abi.Options{StringEncoding: abi.StringEncodingUTF8},
				Realloc: f.reallocWrapper(),
			}

			ptr, err := abi.StoreParamsToMemory(lowerCtx, paramTypes, params)
			if err != nil {
				return nil, fmt.Errorf("spill params: %w", err)
			}
			coreParams = []uint64{uint64(ptr)}
			// Skip normal param conversion
			goto callCore
		}
	}

	// ... existing param conversion ...

callCore:
	coreResults, err := f.coreFunc.Call(ctx, coreParams...)
	// ...
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
git commit -m "feat(component): integrate parameter spilling into Call path

When params exceed MAX_FLAT_PARAMS (16), allocate memory and pass
pointer instead of flat values."
```

---

## Task 4.6: Add Comprehensive Conformance Tests

**Files:**
- Modify: `internal/component/conformance/param_spilling_test.go`

**Step 1: Add comprehensive tests**

```go
package conformance

import (
	"fmt"
	"testing"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/abi"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestParamSpilling_MaxFlatParams(t *testing.T) {
	require.Equal(t, 16, abi.MaxFlatParams)
}

func TestParamSpilling_FlattenWithSpilling(t *testing.T) {
	tests := []struct {
		name         string
		paramTypes   []types.ValType
		wantSpilling bool
		wantFlatLen  int
	}{
		{
			name:         "empty",
			paramTypes:   nil,
			wantSpilling: false,
			wantFlatLen:  0,
		},
		{
			name:         "one_u32",
			paramTypes:   []types.ValType{types.U32{}},
			wantSpilling: false,
			wantFlatLen:  1,
		},
		{
			name:         "16_u32s_no_spill",
			paramTypes:   makeU32Types(16),
			wantSpilling: false,
			wantFlatLen:  16,
		},
		{
			name:         "17_u32s_spill",
			paramTypes:   makeU32Types(17),
			wantSpilling: true,
			wantFlatLen:  1,
		},
		{
			name:         "8_strings_spill",
			paramTypes:   makeStringTypes(8),
			wantSpilling: false, // 8 * 2 = 16 flat
			wantFlatLen:  16,
		},
		{
			name:         "9_strings_spill",
			paramTypes:   makeStringTypes(9),
			wantSpilling: true, // 9 * 2 = 18 > 16
			wantFlatLen:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flat, needsSpilling := abi.FlattenParamsWithSpilling(tt.paramTypes)
			require.Equal(t, tt.wantSpilling, needsSpilling)
			require.Equal(t, tt.wantFlatLen, len(flat))
		})
	}
}

func TestParamSpilling_TupleLayout(t *testing.T) {
	tests := []struct {
		name      string
		types     []types.ValType
		wantSize  uint32
		wantAlign uint32
	}{
		{"empty", nil, 0, 1},
		{"single_u8", []types.ValType{types.U8{}}, 1, 1},
		{"single_u32", []types.ValType{types.U32{}}, 4, 4},
		{"single_u64", []types.ValType{types.U64{}}, 8, 8},
		{"u32_u32", []types.ValType{types.U32{}, types.U32{}}, 8, 4},
		{"u8_u64", []types.ValType{types.U8{}, types.U64{}}, 16, 8}, // pad to 8
		{"17_u32s", makeU32Types(17), 68, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size, align := abi.TupleLayout(tt.types)
			require.Equal(t, tt.wantSize, size)
			require.Equal(t, tt.wantAlign, align)
		})
	}
}

func TestParamSpilling_StoreAndLoad(t *testing.T) {
	mem := newMockMemory(4096)
	allocPtr := uint32(256)

	ctx := &abi.LowerContext{
		Memory: mem,
		Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			result := allocPtr
			allocPtr = alignTo(allocPtr+newSize, 8)
			return result, nil
		},
	}

	t.Run("store_17_u32s", func(t *testing.T) {
		allocPtr = 256
		paramTypes := makeU32Types(17)
		params := make([]component.Val, 17)
		for i := range params {
			params[i] = component.ValU32(uint32(i + 1))
		}

		ptr, err := abi.StoreParamsToMemory(ctx, paramTypes, params)
		require.NoError(t, err)
		require.Equal(t, uint32(256), ptr)

		// Verify each value
		for i := 0; i < 17; i++ {
			v, ok := mem.ReadUint32Le(256 + uint32(i)*4)
			require.True(t, ok)
			require.Equal(t, uint32(i+1), v)
		}
	})
}

func TestParamSpilling_NoReallocFails(t *testing.T) {
	mem := newMockMemory(4096)
	ctx := &abi.LowerContext{
		Memory:  mem,
		Opts:    &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		Realloc: nil, // No realloc
	}

	paramTypes := makeU32Types(17)
	params := make([]component.Val, 17)
	for i := range params {
		params[i] = component.ValU32(uint32(i))
	}

	_, err := abi.StoreParamsToMemory(ctx, paramTypes, params)
	require.Error(t, err)
	require.Contains(t, err.Error(), "realloc")
}

// Helper functions

func makeU32Types(n int) []types.ValType {
	result := make([]types.ValType, n)
	for i := range result {
		result[i] = types.U32{}
	}
	return result
}

func makeStringTypes(n int) []types.ValType {
	result := make([]types.ValType, n)
	for i := range result {
		result[i] = types.String{}
	}
	return result
}

func alignTo(offset, align uint32) uint32 {
	return (offset + align - 1) &^ (align - 1)
}
```

**Step 2: Run all tests**

```bash
go test -v ./internal/component/conformance/... -run TestParamSpilling
```

Expected: All PASS

**Step 3: Commit**

```bash
git add internal/component/conformance/param_spilling_test.go
git commit -m "test(component): add comprehensive parameter spilling tests

Tests cover:
- MaxFlatParams constant
- Spilling detection
- Tuple layout calculation
- Store and load round-trip
- Error cases"
```

---

## Phase 4 Regression Check

**CRITICAL:** After completing all Task 4.x, run the calculator regression test:

```bash
go test -v ./internal/component/wasip2test/... -run "TestAdd|TestSubtract"
```

**Expected:** Both tests PASS

If tests fail:
1. Check that param spilling only triggers for >16 flat params
2. Verify existing tests don't hit spilling path
3. Ensure type resolution doesn't break existing behavior

---

## Phase 4 Summary

After completing Phase 4, the codebase will have:

1. `MaxFlatParams = 16` constant
2. `FlattenParamsWithSpilling` detection function
3. `TupleLayout` size/alignment calculation
4. `SizeOf` and `AlignOf` for all types
5. `StoreParamsToMemory` allocation and storage
6. Integration in `ExportedFunc.Call`
7. Comprehensive test coverage

**Files Modified:**
- `internal/component/abi/flatten.go` (constants, layout functions)
- `internal/component/abi/lower.go` (StoreParamsToMemory)
- `internal/component/instance.go` (Call integration)
- `internal/component/conformance/param_spilling_test.go` (new)

**Next Phase:** [05-alignment-validation.md](./05-alignment-validation.md)
