# Component Model Phase 2: Complete Type System

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Parent Plan:** [2026-01-12-component-model-implementation.md](./2026-01-12-component-model-implementation.md)
**Design Doc:** [2026-01-12-component-model-design.md](./2026-01-12-component-model-design.md)
**Previous Phase:** [Phase 1: Binary Parser & Primitives](./2026-01-12-component-model-phase1-binary-parser.md)
**Status:** NOT STARTED
**Tasks:** 31-70

---

## Overview

This phase completes the component type system by implementing all WIT composite types and their Canonical ABI lift/lower operations.

**Goal:** Support all WIT types: record, variant, list, option, result, flags, enum, tuple with full Canonical ABI round-trip.

**Prerequisites:**
- Phase 1 complete (binary parser, primitive types, basic instantiation)
- `add(s32, s32) -> s32` working end-to-end

---

## Phase 2 Milestones

| Milestone | Description | Tasks | Success Criteria |
|-----------|-------------|-------|------------------|
| 2.1 | Composite type definitions | 31-38 | Record, variant, list, option, result, flags, enum, tuple types compile with correct Size/Align/FlattenCount |
| 2.2 | Binary parsing for composites | 39-45 | Decoder reads composite type opcodes 0x6b-0x72 |
| 2.3 | Val type extensions | 46-48 | Val constructors and accessors for all composite types |
| 2.4 | Flat ABI lift/lower | 49-54 | Composite types that fit in registers work |
| 2.5 | Heap ABI lift/lower | 55-60 | Large types via memory pointers work |
| 2.6 | String encodings | 61-65 | UTF-8, UTF-16, Latin1+UTF16 all work |
| 2.7 | Integration tests | 66-70 | Round-trip all composite types through real components |

---

## Canonical ABI Reference

### Constants

```go
const MaxFlatParams = 16   // Maximum flattened parameter values
const MaxFlatResults = 1   // Maximum flattened result values (sync)
```

### Memory Layout Rules

1. **Records**: Fields laid out in declaration order, each aligned to its requirement
2. **Variants**: Discriminant (1/2/4 bytes) + padding + max payload size
3. **Lists**: Variable-length as `(ptr: i32, len: i32)`, fixed-length inline
4. **Flags**: Packed bits into 1/2/4+ bytes based on flag count
5. **Strings**: `(ptr: i32, tagged_len: i32)` where bit 31 indicates UTF-16

### Discriminant Size Formula

```
cases <= 256:   1 byte (u8)
cases <= 65536: 2 bytes (u16)
cases > 65536:  4 bytes (u32)
```

### Flat Type Mapping

| Component Type | Flat Types |
|----------------|------------|
| bool, u8, s8, u16, s16, u32, s32, char | `[i32]` |
| u64, s64 | `[i64]` |
| f32 | `[f32]` |
| f64 | `[f64]` |
| string | `[i32, i32]` |
| list<T> (variable) | `[i32, i32]` |
| record | concatenated field flattening |
| variant | `[i32] + joined(case payloads)` |
| flags | `[i32]` |
| own<T>, borrow<T> | `[i32]` |

---

## Tasks

### Task 31: Define Record Type

**Files:**
- Create: `internal/component/types/composite.go`
- Create: `internal/component/types/composite_test.go`

**Step 1: Write failing test for Record type**

```go
// internal/component/types/composite_test.go

package types

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestRecordType(t *testing.T) {
	// Record { a: u32, b: u64 }
	// Layout: u32 at offset 0, padding 4, u64 at offset 8
	// Size: 16, Align: 8
	r := Record{
		Fields: []Field{
			{Name: "a", Type: U32{}},
			{Name: "b", Type: U64{}},
		},
	}

	require.Equal(t, uint32(16), r.Size())
	require.Equal(t, uint32(8), r.Align())
	require.Equal(t, 2, r.FlattenCount()) // Both fields fit flat
}

func TestRecordOffset(t *testing.T) {
	r := Record{
		Fields: []Field{
			{Name: "a", Type: U32{}},
			{Name: "b", Type: U64{}},
		},
	}

	offsets := r.FieldOffsets()
	require.Equal(t, uint32(0), offsets[0])  // a at 0
	require.Equal(t, uint32(8), offsets[1])  // b at 8 (aligned)
}

func TestRecordEmpty(t *testing.T) {
	r := Record{Fields: []Field{}}
	require.Equal(t, uint32(0), r.Size())
	require.Equal(t, uint32(1), r.Align())
	require.Equal(t, 0, r.FlattenCount())
}

func TestRecordComplex(t *testing.T) {
	// Record { a: u8, b: u32, c: u16 }
	// Offset 0: a (1 byte)
	// Offset 1-3: padding
	// Offset 4: b (4 bytes)
	// Offset 8: c (2 bytes)
	// Offset 10-11: padding to align 4
	// Size: 12, Align: 4
	r := Record{
		Fields: []Field{
			{Name: "a", Type: U8{}},
			{Name: "b", Type: U32{}},
			{Name: "c", Type: U16{}},
		},
	}

	require.Equal(t, uint32(12), r.Size())
	require.Equal(t, uint32(4), r.Align())

	offsets := r.FieldOffsets()
	require.Equal(t, uint32(0), offsets[0])
	require.Equal(t, uint32(4), offsets[1])
	require.Equal(t, uint32(8), offsets[2])
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/types/... -v -run TestRecord`
Expected: FAIL with "undefined: Record"

**Step 3: Implement Record type**

```go
// internal/component/types/composite.go

package types

// Field represents a named field in a record.
type Field struct {
	Name string
	Type ValType
}

// Record represents a record (struct) type with named fields.
type Record struct {
	Fields []Field
}

func (Record) valType() {}

func (r Record) Size() uint32 {
	if len(r.Fields) == 0 {
		return 0
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

func (r Record) Align() uint32 {
	maxAlign := uint32(1)
	for _, f := range r.Fields {
		if a := f.Type.Align(); a > maxAlign {
			maxAlign = a
		}
	}
	return maxAlign
}

func (r Record) FlattenCount() int {
	count := 0
	for _, f := range r.Fields {
		count += f.Type.FlattenCount()
	}
	return count
}

// FieldOffsets returns the byte offset of each field in memory.
func (r Record) FieldOffsets() []uint32 {
	offsets := make([]uint32, len(r.Fields))
	offset := uint32(0)
	for i, f := range r.Fields {
		offset = alignTo(offset, f.Type.Align())
		offsets[i] = offset
		offset += f.Type.Size()
	}
	return offsets
}

// alignTo rounds offset up to the given alignment.
func alignTo(offset, align uint32) uint32 {
	return (offset + align - 1) &^ (align - 1)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/types/... -v -run TestRecord`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/types/composite.go internal/component/types/composite_test.go
git commit -m "feat(component): add Record composite type with memory layout

- Add Field and Record types
- Implement Size(), Align(), FlattenCount() per Canonical ABI spec
- Add FieldOffsets() for memory layout calculations
- Add alignTo() helper function"
```

---

### Task 32: Define Variant Type

**Files:**
- Modify: `internal/component/types/composite.go`
- Modify: `internal/component/types/composite_test.go`

**Step 1: Write failing test**

```go
func TestVariantType(t *testing.T) {
	// variant { none, some(u32) }
	// Discriminant: 1 byte (2 cases < 256)
	// Payload: max(0, 4) = 4 bytes
	// Layout: disc(1) + padding(3) + payload(4) = 8, Align: 4
	v := Variant{
		Cases: []Case{
			{Name: "none", Type: nil},
			{Name: "some", Type: U32{}},
		},
	}

	require.Equal(t, uint32(8), v.Size())
	require.Equal(t, uint32(4), v.Align())
	require.Equal(t, 2, v.FlattenCount()) // disc + payload
}

func TestVariantDiscriminantSize(t *testing.T) {
	tests := []struct {
		numCases int
		discSize uint32
	}{
		{2, 1},
		{255, 1},
		{256, 1},
		{257, 2},
		{65536, 2},
		{65537, 4},
	}

	for _, tc := range tests {
		cases := make([]Case, tc.numCases)
		for i := range cases {
			cases[i] = Case{Name: fmt.Sprintf("case%d", i), Type: nil}
		}
		v := Variant{Cases: cases}
		require.Equal(t, tc.discSize, v.DiscriminantSize(),
			"numCases=%d", tc.numCases)
	}
}

func TestVariantWithU64Payload(t *testing.T) {
	// variant { a, b(u64) }
	// Discriminant: 1 byte
	// Padding: 7 bytes (align to 8 for u64)
	// Payload: 8 bytes
	// Size: 16, Align: 8
	v := Variant{
		Cases: []Case{
			{Name: "a", Type: nil},
			{Name: "b", Type: U64{}},
		},
	}

	require.Equal(t, uint32(16), v.Size())
	require.Equal(t, uint32(8), v.Align())
}

func TestVariantFlatten(t *testing.T) {
	// variant { a(u32), b(f32) }
	// Flat: [i32 (disc), i32 (joined payload)]
	// Note: u32 and f32 both flatten to single value, join produces i32
	v := Variant{
		Cases: []Case{
			{Name: "a", Type: U32{}},
			{Name: "b", Type: F32{}},
		},
	}

	require.Equal(t, 2, v.FlattenCount())
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/types/... -v -run TestVariant`
Expected: FAIL with "undefined: Variant"

**Step 3: Implement Variant type**

```go
// Case represents a case in a variant type.
type Case struct {
	Name string
	Type ValType // nil for cases with no payload
}

// Variant represents a discriminated union type.
type Variant struct {
	Cases []Case
}

func (Variant) valType() {}

// DiscriminantSize returns the size of the discriminant in bytes.
func (v Variant) DiscriminantSize() uint32 {
	n := len(v.Cases)
	switch {
	case n <= 0x100: // 256
		return 1
	case n <= 0x10000: // 65536
		return 2
	default:
		return 4
	}
}

func (v Variant) Size() uint32 {
	discSize := v.DiscriminantSize()
	payloadSize := uint32(0)
	payloadAlign := uint32(1)
	for _, c := range v.Cases {
		if c.Type != nil {
			if s := c.Type.Size(); s > payloadSize {
				payloadSize = s
			}
			if a := c.Type.Align(); a > payloadAlign {
				payloadAlign = a
			}
		}
	}
	// discriminant + padding + payload, aligned to variant alignment
	offset := alignTo(discSize, payloadAlign)
	return alignTo(offset+payloadSize, v.Align())
}

func (v Variant) Align() uint32 {
	align := v.DiscriminantSize()
	for _, c := range v.Cases {
		if c.Type != nil {
			if a := c.Type.Align(); a > align {
				align = a
			}
		}
	}
	return align
}

func (v Variant) FlattenCount() int {
	// discriminant + max payload flattening
	maxPayload := 0
	for _, c := range v.Cases {
		if c.Type != nil {
			if n := c.Type.FlattenCount(); n > maxPayload {
				maxPayload = n
			}
		}
	}
	return 1 + maxPayload
}

// PayloadOffset returns the byte offset where payload data starts.
func (v Variant) PayloadOffset() uint32 {
	payloadAlign := uint32(1)
	for _, c := range v.Cases {
		if c.Type != nil {
			if a := c.Type.Align(); a > payloadAlign {
				payloadAlign = a
			}
		}
	}
	return alignTo(v.DiscriminantSize(), payloadAlign)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/types/... -v -run TestVariant`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/types/composite.go internal/component/types/composite_test.go
git commit -m "feat(component): add Variant composite type

- Add Case and Variant types
- Implement discriminant size calculation per spec
- Implement Size(), Align(), FlattenCount(), PayloadOffset()
- Handle payload alignment and padding correctly"
```

---

### Task 33: Define List Type

**Files:**
- Modify: `internal/component/types/composite.go`
- Modify: `internal/component/types/composite_test.go`

**Step 1: Write failing test**

```go
func TestListType(t *testing.T) {
	// list<u32> (variable length)
	// In memory: (ptr: i32, len: i32)
	// Size: 8, Align: 4, Flatten: 2
	l := List{Element: U32{}}

	require.Equal(t, uint32(8), l.Size())
	require.Equal(t, uint32(4), l.Align())
	require.Equal(t, 2, l.FlattenCount())
}

func TestListTypeWithComplexElement(t *testing.T) {
	// list<u64>
	// Still stored as ptr+len regardless of element type
	l := List{Element: U64{}}

	require.Equal(t, uint32(8), l.Size())
	require.Equal(t, uint32(4), l.Align())
	require.Equal(t, 2, l.FlattenCount())
}

func TestListElementSize(t *testing.T) {
	l := List{Element: U64{}}
	require.Equal(t, uint32(8), l.ElementSize())
	require.Equal(t, uint32(8), l.ElementAlign())
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/types/... -v -run TestList`
Expected: FAIL with "undefined: List"

**Step 3: Implement List type**

```go
// List represents a variable-length list type.
type List struct {
	Element ValType
}

func (List) valType() {}

// Size returns the size of the list in memory (pointer + length).
func (List) Size() uint32 { return 8 } // ptr: i32, len: i32

// Align returns the alignment of the list (i32 alignment).
func (List) Align() uint32 { return 4 }

// FlattenCount returns 2 (pointer and length).
func (List) FlattenCount() int { return 2 }

// ElementSize returns the size of each element.
func (l List) ElementSize() uint32 { return l.Element.Size() }

// ElementAlign returns the alignment of each element.
func (l List) ElementAlign() uint32 { return l.Element.Align() }
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/types/... -v -run TestList`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/types/composite.go internal/component/types/composite_test.go
git commit -m "feat(component): add List composite type

- Variable-length lists stored as ptr+len (8 bytes)
- Add ElementSize() and ElementAlign() helpers"
```

---

### Task 34: Define Option Type

**Files:**
- Modify: `internal/component/types/composite.go`
- Modify: `internal/component/types/composite_test.go`

**Step 1: Write failing test**

```go
func TestOptionType(t *testing.T) {
	// option<u32> is variant { none, some(u32) }
	o := Option{Some: U32{}}

	// Same layout as variant with 2 cases
	// disc(1) + padding(3) + payload(4) = 8, align 4
	require.Equal(t, uint32(8), o.Size())
	require.Equal(t, uint32(4), o.Align())
	require.Equal(t, 2, o.FlattenCount()) // disc + payload
}

func TestOptionNone(t *testing.T) {
	// option<string> where Some is a string (8 bytes, align 4)
	o := Option{Some: String{}}

	// disc(1) + padding(3) + payload(8) = 12, align 4
	require.Equal(t, uint32(12), o.Size())
	require.Equal(t, uint32(4), o.Align())
}

func TestOptionU64(t *testing.T) {
	// option<u64>
	// disc(1) + padding(7) + payload(8) = 16, align 8
	o := Option{Some: U64{}}

	require.Equal(t, uint32(16), o.Size())
	require.Equal(t, uint32(8), o.Align())
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/types/... -v -run TestOption`
Expected: FAIL with "undefined: Option"

**Step 3: Implement Option type**

```go
// Option represents an optional value type (sugar for variant { none, some(T) }).
type Option struct {
	Some ValType
}

func (Option) valType() {}

func (o Option) Size() uint32 {
	return o.asVariant().Size()
}

func (o Option) Align() uint32 {
	return o.asVariant().Align()
}

func (o Option) FlattenCount() int {
	return o.asVariant().FlattenCount()
}

// asVariant returns the equivalent Variant representation.
func (o Option) asVariant() Variant {
	return Variant{
		Cases: []Case{
			{Name: "none", Type: nil},
			{Name: "some", Type: o.Some},
		},
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/types/... -v -run TestOption`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/types/composite.go internal/component/types/composite_test.go
git commit -m "feat(component): add Option composite type

- Option desugars to variant { none, some(T) }
- Delegates Size/Align/FlattenCount to asVariant()"
```

---

### Task 35: Define Result Type

**Files:**
- Modify: `internal/component/types/composite.go`
- Modify: `internal/component/types/composite_test.go`

**Step 1: Write failing test**

```go
func TestResultType(t *testing.T) {
	// result<u32, string>
	// variant { ok(u32), error(string) }
	// Discriminant: 1 byte
	// Payload: max(4, 8) = 8 bytes, align max(4, 4) = 4
	// Size: disc(1) + padding(3) + payload(8) = 12, align 4
	r := Result{Ok: U32{}, Error: String{}}

	require.Equal(t, uint32(12), r.Size())
	require.Equal(t, uint32(4), r.Align())
	require.Equal(t, 3, r.FlattenCount()) // disc + max(1, 2)
}

func TestResultOkOnly(t *testing.T) {
	// result<u64, _> (no error payload)
	r := Result{Ok: U64{}, Error: nil}

	// disc(1) + padding(7) + payload(8) = 16, align 8
	require.Equal(t, uint32(16), r.Size())
	require.Equal(t, uint32(8), r.Align())
}

func TestResultErrorOnly(t *testing.T) {
	// result<_, string> (no ok payload)
	r := Result{Ok: nil, Error: String{}}

	// disc(1) + padding(3) + payload(8) = 12, align 4
	require.Equal(t, uint32(12), r.Size())
	require.Equal(t, uint32(4), r.Align())
}

func TestResultUnit(t *testing.T) {
	// result (no payloads)
	r := Result{Ok: nil, Error: nil}

	// Just discriminant: 1 byte, align 1
	require.Equal(t, uint32(1), r.Size())
	require.Equal(t, uint32(1), r.Align())
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/types/... -v -run TestResult`
Expected: FAIL with "undefined: Result"

**Step 3: Implement Result type**

```go
// Result represents a result type (sugar for variant { ok(T), error(E) }).
type Result struct {
	Ok    ValType // nil for result<_, E>
	Error ValType // nil for result<T, _>
}

func (Result) valType() {}

func (r Result) Size() uint32 {
	return r.asVariant().Size()
}

func (r Result) Align() uint32 {
	return r.asVariant().Align()
}

func (r Result) FlattenCount() int {
	return r.asVariant().FlattenCount()
}

// asVariant returns the equivalent Variant representation.
func (r Result) asVariant() Variant {
	return Variant{
		Cases: []Case{
			{Name: "ok", Type: r.Ok},
			{Name: "error", Type: r.Error},
		},
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/types/... -v -run TestResult`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/types/composite.go internal/component/types/composite_test.go
git commit -m "feat(component): add Result composite type

- Result desugars to variant { ok(T), error(E) }
- Handles nil Ok/Error for partial results"
```

---

### Task 36: Define Flags Type

**Files:**
- Modify: `internal/component/types/composite.go`
- Modify: `internal/component/types/composite_test.go`

**Step 1: Write failing test**

```go
func TestFlagsType(t *testing.T) {
	// flags { read, write, execute }
	// 3 flags fits in u8
	f := Flags{Names: []string{"read", "write", "execute"}}

	require.Equal(t, uint32(1), f.Size())
	require.Equal(t, uint32(1), f.Align())
	require.Equal(t, 1, f.FlattenCount())
}

func TestFlagsType8(t *testing.T) {
	// 8 flags still fits in u8
	names := make([]string, 8)
	for i := range names {
		names[i] = fmt.Sprintf("flag%d", i)
	}
	f := Flags{Names: names}

	require.Equal(t, uint32(1), f.Size())
	require.Equal(t, uint32(1), f.Align())
}

func TestFlagsType9(t *testing.T) {
	// 9 flags needs u16
	names := make([]string, 9)
	for i := range names {
		names[i] = fmt.Sprintf("flag%d", i)
	}
	f := Flags{Names: names}

	require.Equal(t, uint32(2), f.Size())
	require.Equal(t, uint32(2), f.Align())
}

func TestFlagsType16(t *testing.T) {
	// 16 flags fits in u16
	names := make([]string, 16)
	for i := range names {
		names[i] = fmt.Sprintf("flag%d", i)
	}
	f := Flags{Names: names}

	require.Equal(t, uint32(2), f.Size())
	require.Equal(t, uint32(2), f.Align())
}

func TestFlagsType17(t *testing.T) {
	// 17 flags needs u32
	names := make([]string, 17)
	for i := range names {
		names[i] = fmt.Sprintf("flag%d", i)
	}
	f := Flags{Names: names}

	require.Equal(t, uint32(4), f.Size())
	require.Equal(t, uint32(4), f.Align())
}

func TestFlagsType33(t *testing.T) {
	// 33 flags needs 2 x u32 = 8 bytes
	names := make([]string, 33)
	for i := range names {
		names[i] = fmt.Sprintf("flag%d", i)
	}
	f := Flags{Names: names}

	require.Equal(t, uint32(8), f.Size())
	require.Equal(t, uint32(4), f.Align())
	require.Equal(t, 2, f.FlattenCount()) // 2 i32s
}

func TestFlagsEmpty(t *testing.T) {
	f := Flags{Names: []string{}}
	require.Equal(t, uint32(0), f.Size())
	require.Equal(t, uint32(1), f.Align())
	require.Equal(t, 0, f.FlattenCount())
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/types/... -v -run TestFlags`
Expected: FAIL with "undefined: Flags"

**Step 3: Implement Flags type**

```go
// Flags represents a flags (bitfield) type.
type Flags struct {
	Names []string
}

func (Flags) valType() {}

func (f Flags) Size() uint32 {
	n := len(f.Names)
	switch {
	case n == 0:
		return 0
	case n <= 8:
		return 1
	case n <= 16:
		return 2
	default:
		// Round up to multiple of 32 bits
		return 4 * uint32((n+31)/32)
	}
}

func (f Flags) Align() uint32 {
	n := len(f.Names)
	switch {
	case n == 0:
		return 1
	case n <= 8:
		return 1
	case n <= 16:
		return 2
	default:
		return 4
	}
}

func (f Flags) FlattenCount() int {
	n := len(f.Names)
	if n == 0 {
		return 0
	}
	// Number of i32s needed
	return (n + 31) / 32
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/types/... -v -run TestFlags`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/types/composite.go internal/component/types/composite_test.go
git commit -m "feat(component): add Flags composite type

- Pack flags into u8/u16/u32 based on count
- Support > 32 flags with multiple u32s
- Handle empty flags case"
```

---

### Task 37: Define Enum Type

**Files:**
- Modify: `internal/component/types/composite.go`
- Modify: `internal/component/types/composite_test.go`

**Step 1: Write failing test**

```go
func TestEnumType(t *testing.T) {
	// enum { red, green, blue }
	e := Enum{Cases: []string{"red", "green", "blue"}}

	require.Equal(t, uint32(1), e.Size())  // 3 cases fits in u8
	require.Equal(t, uint32(1), e.Align())
	require.Equal(t, 1, e.FlattenCount())
}

func TestEnumType256(t *testing.T) {
	// 256 cases still fits in u8
	cases := make([]string, 256)
	for i := range cases {
		cases[i] = fmt.Sprintf("case%d", i)
	}
	e := Enum{Cases: cases}

	require.Equal(t, uint32(1), e.Size())
	require.Equal(t, uint32(1), e.Align())
}

func TestEnumType257(t *testing.T) {
	// 257 cases needs u16
	cases := make([]string, 257)
	for i := range cases {
		cases[i] = fmt.Sprintf("case%d", i)
	}
	e := Enum{Cases: cases}

	require.Equal(t, uint32(2), e.Size())
	require.Equal(t, uint32(2), e.Align())
}

func TestEnumType65537(t *testing.T) {
	// 65537 cases needs u32
	cases := make([]string, 65537)
	for i := range cases {
		cases[i] = fmt.Sprintf("case%d", i)
	}
	e := Enum{Cases: cases}

	require.Equal(t, uint32(4), e.Size())
	require.Equal(t, uint32(4), e.Align())
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/types/... -v -run TestEnum`
Expected: FAIL with "undefined: Enum"

**Step 3: Implement Enum type**

```go
// Enum represents an enumeration type (discriminant-only variant).
type Enum struct {
	Cases []string
}

func (Enum) valType() {}

func (e Enum) Size() uint32 {
	n := len(e.Cases)
	switch {
	case n <= 0x100: // 256
		return 1
	case n <= 0x10000: // 65536
		return 2
	default:
		return 4
	}
}

func (e Enum) Align() uint32 {
	return e.Size()
}

func (Enum) FlattenCount() int {
	return 1
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/types/... -v -run TestEnum`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/types/composite.go internal/component/types/composite_test.go
git commit -m "feat(component): add Enum composite type

- Enum is discriminant-only variant
- Size based on case count (u8/u16/u32)"
```

---

### Task 38: Define Tuple Type

**Files:**
- Modify: `internal/component/types/composite.go`
- Modify: `internal/component/types/composite_test.go`

**Step 1: Write failing test**

```go
func TestTupleType(t *testing.T) {
	// tuple<u32, u64>
	// Same layout as record with positional fields
	tup := Tuple{Types: []ValType{U32{}, U64{}}}

	require.Equal(t, uint32(16), tup.Size())
	require.Equal(t, uint32(8), tup.Align())
	require.Equal(t, 2, tup.FlattenCount())
}

func TestTupleEmpty(t *testing.T) {
	tup := Tuple{Types: []ValType{}}
	require.Equal(t, uint32(0), tup.Size())
	require.Equal(t, uint32(1), tup.Align())
	require.Equal(t, 0, tup.FlattenCount())
}

func TestTupleSingle(t *testing.T) {
	tup := Tuple{Types: []ValType{U32{}}}
	require.Equal(t, uint32(4), tup.Size())
	require.Equal(t, uint32(4), tup.Align())
	require.Equal(t, 1, tup.FlattenCount())
}

func TestTupleComplex(t *testing.T) {
	// tuple<u8, u32, u16>
	// Same as record { 0: u8, 1: u32, 2: u16 }
	tup := Tuple{Types: []ValType{U8{}, U32{}, U16{}}}

	require.Equal(t, uint32(12), tup.Size())
	require.Equal(t, uint32(4), tup.Align())
}

func TestTupleOffsets(t *testing.T) {
	tup := Tuple{Types: []ValType{U8{}, U32{}, U16{}}}
	offsets := tup.ElementOffsets()

	require.Equal(t, uint32(0), offsets[0])  // u8 at 0
	require.Equal(t, uint32(4), offsets[1])  // u32 at 4 (aligned)
	require.Equal(t, uint32(8), offsets[2])  // u16 at 8
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/types/... -v -run TestTuple`
Expected: FAIL with "undefined: Tuple"

**Step 3: Implement Tuple type**

```go
// Tuple represents a tuple type (positional record).
type Tuple struct {
	Types []ValType
}

func (Tuple) valType() {}

func (t Tuple) Size() uint32 {
	return t.asRecord().Size()
}

func (t Tuple) Align() uint32 {
	return t.asRecord().Align()
}

func (t Tuple) FlattenCount() int {
	return t.asRecord().FlattenCount()
}

// ElementOffsets returns the byte offset of each element in memory.
func (t Tuple) ElementOffsets() []uint32 {
	return t.asRecord().FieldOffsets()
}

// asRecord returns the equivalent Record representation.
func (t Tuple) asRecord() Record {
	fields := make([]Field, len(t.Types))
	for i, typ := range t.Types {
		fields[i] = Field{Name: fmt.Sprintf("%d", i), Type: typ}
	}
	return Record{Fields: fields}
}
```

**Step 4: Add import for fmt**

```go
// At top of composite.go
import "fmt"
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/component/types/... -v -run TestTuple`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/component/types/composite.go internal/component/types/composite_test.go
git commit -m "feat(component): add Tuple composite type

- Tuple desugars to Record with numeric field names
- Add ElementOffsets() helper"
```

---

### Task 39: Add Composite Type Binary Opcodes

**Files:**
- Modify: `internal/component/binary/valtype.go`
- Modify: `internal/component/binary/valtype_test.go`

**Step 1: Write failing test**

```go
func TestCompositeTypeOpcodes(t *testing.T) {
	tests := []struct {
		opcode byte
		name   string
	}{
		{0x72, "record"},
		{0x71, "variant"},
		{0x70, "list"},
		{0x6f, "tuple"},
		{0x6e, "flags"},
		{0x6d, "enum"},
		{0x6b, "option"},
		{0x6a, "result"},
	}

	for _, tc := range tests {
		require.True(t, IsCompositeTypeOpcode(tc.opcode),
			"opcode 0x%02x should be composite type %s", tc.opcode, tc.name)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/binary/... -v -run TestCompositeTypeOpcodes`
Expected: FAIL with "undefined: IsCompositeTypeOpcode"

**Step 3: Add opcode constants and helper**

```go
// internal/component/binary/valtype.go

// Composite type opcodes
const (
	ValTypeOpcodeRecord  byte = 0x72
	ValTypeOpcodeVariant byte = 0x71
	ValTypeOpcodeList    byte = 0x70
	ValTypeOpcodeTuple   byte = 0x6f
	ValTypeOpcodeFlags   byte = 0x6e
	ValTypeOpcodeEnum    byte = 0x6d
	ValTypeOpcodeOption  byte = 0x6b
	ValTypeOpcodeResult  byte = 0x6a
)

// IsCompositeTypeOpcode returns true if the opcode is a composite type.
func IsCompositeTypeOpcode(opcode byte) bool {
	return opcode >= 0x6a && opcode <= 0x72
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/component/binary/... -v -run TestCompositeTypeOpcodes`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/binary/valtype.go internal/component/binary/valtype_test.go
git commit -m "feat(component): add composite type binary opcodes

- Add constants for opcodes 0x6b-0x72
- Add IsCompositeTypeOpcode() helper"
```

---

### Task 40: Parse Record Type Definition

**Files:**
- Modify: `internal/component/binary/types.go`
- Create: `internal/component/binary/types_composite_test.go`

**Step 1: Write failing test**

```go
// internal/component/binary/types_composite_test.go

package binary

import (
	"bytes"
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestDecodeRecordType(t *testing.T) {
	// Record with 2 fields: (a: s32, b: u64)
	// Format: 0x72 <field_count> (<name> <type>)*
	data := []byte{
		0x72,             // record opcode
		0x02,             // 2 fields
		0x01, 'a',        // field name "a"
		0x7a,             // s32
		0x01, 'b',        // field name "b"
		0x77,             // u64
	}

	r := bytes.NewReader(data)
	typeDef, err := decodeDefinedType(r)
	require.NoError(t, err)
	require.Equal(t, TypeDefKindRecord, typeDef.Kind)
	require.NotNil(t, typeDef.Record)
	require.Len(t, typeDef.Record.Fields, 2)
	require.Equal(t, "a", typeDef.Record.Fields[0].Name)
	require.Equal(t, "b", typeDef.Record.Fields[1].Name)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/binary/... -v -run TestDecodeRecordType`
Expected: FAIL

**Step 3: Update TypeDef structure**

```go
// internal/component/binary/types.go

// TypeDefKind identifies the kind of type definition.
type TypeDefKind uint8

const (
	TypeDefKindFunc TypeDefKind = iota
	TypeDefKindRecord
	TypeDefKindVariant
	TypeDefKindList
	TypeDefKindTuple
	TypeDefKindFlags
	TypeDefKindEnum
	TypeDefKindOption
	TypeDefKindResult
)

// TypeDef represents a component type definition.
type TypeDef struct {
	Kind    TypeDefKind
	Func    *FuncType
	Record  *RecordTypeDef
	Variant *VariantTypeDef
	List    *ListTypeDef
	Tuple   *TupleTypeDef
	Flags   *FlagsTypeDef
	Enum    *EnumTypeDef
	Option  *OptionTypeDef
	Result  *ResultTypeDef
}

// RecordTypeDef represents a record type definition.
type RecordTypeDef struct {
	Fields []RecordField
}

// RecordField represents a field in a record type.
type RecordField struct {
	Name string
	Type ValTypeRef
}
```

**Step 4: Implement record parsing**

```go
func decodeDefinedType(r *bytes.Reader) (TypeDef, error) {
	opcode, err := r.ReadByte()
	if err != nil {
		return TypeDef{}, fmt.Errorf("read type opcode: %w", err)
	}

	switch opcode {
	case 0x40: // func type
		return decodeFuncTypeDef(r)
	case ValTypeOpcodeRecord:
		return decodeRecordTypeDef(r)
	// ... other cases added in subsequent tasks
	default:
		return TypeDef{}, fmt.Errorf("unknown type opcode: 0x%02x", opcode)
	}
}

func decodeRecordTypeDef(r *bytes.Reader) (TypeDef, error) {
	fieldCount, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return TypeDef{}, fmt.Errorf("read record field count: %w", err)
	}

	fields := make([]RecordField, fieldCount)
	for i := uint32(0); i < fieldCount; i++ {
		name, err := decodeName(r)
		if err != nil {
			return TypeDef{}, fmt.Errorf("read field %d name: %w", i, err)
		}
		valType, err := decodeValTypeRef(r)
		if err != nil {
			return TypeDef{}, fmt.Errorf("read field %d type: %w", i, err)
		}
		fields[i] = RecordField{Name: name, Type: valType}
	}

	return TypeDef{
		Kind:   TypeDefKindRecord,
		Record: &RecordTypeDef{Fields: fields},
	}, nil
}
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/component/binary/... -v -run TestDecodeRecordType`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/component/binary/types.go internal/component/binary/types_composite_test.go
git commit -m "feat(component): parse record type definitions

- Add RecordTypeDef and RecordField types
- Implement decodeRecordTypeDef()
- Update decodeDefinedType() to handle 0x72 opcode"
```

---

### Task 41: Parse Variant Type Definition

**Files:**
- Modify: `internal/component/binary/types.go`
- Modify: `internal/component/binary/types_composite_test.go`

**Step 1: Write failing test**

```go
func TestDecodeVariantType(t *testing.T) {
	// Variant with 2 cases: { none, some(s32) }
	// Format: 0x71 <case_count> (<name> <refines>? <type>?)*
	data := []byte{
		0x71,             // variant opcode
		0x02,             // 2 cases
		0x04, 'n', 'o', 'n', 'e',  // case name "none"
		0x00,             // no refines
		// no type (discriminant only)
		0x04, 's', 'o', 'm', 'e',  // case name "some"
		0x00,             // no refines
		0x01,             // has type
		0x7a,             // s32
	}

	r := bytes.NewReader(data)
	typeDef, err := decodeDefinedType(r)
	require.NoError(t, err)
	require.Equal(t, TypeDefKindVariant, typeDef.Kind)
	require.NotNil(t, typeDef.Variant)
	require.Len(t, typeDef.Variant.Cases, 2)
	require.Equal(t, "none", typeDef.Variant.Cases[0].Name)
	require.Nil(t, typeDef.Variant.Cases[0].Type)
	require.Equal(t, "some", typeDef.Variant.Cases[1].Name)
	require.NotNil(t, typeDef.Variant.Cases[1].Type)
}
```

**Step 2: Implement variant parsing**

```go
// VariantTypeDef represents a variant type definition.
type VariantTypeDef struct {
	Cases []VariantCase
}

// VariantCase represents a case in a variant type.
type VariantCase struct {
	Name    string
	Refines *uint32     // Optional index into cases
	Type    *ValTypeRef // nil for discriminant-only cases
}

func decodeVariantTypeDef(r *bytes.Reader) (TypeDef, error) {
	caseCount, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return TypeDef{}, fmt.Errorf("read variant case count: %w", err)
	}

	cases := make([]VariantCase, caseCount)
	for i := uint32(0); i < caseCount; i++ {
		name, err := decodeName(r)
		if err != nil {
			return TypeDef{}, fmt.Errorf("read case %d name: %w", i, err)
		}

		// Read optional refines
		var refines *uint32
		refinesFlag, err := r.ReadByte()
		if err != nil {
			return TypeDef{}, fmt.Errorf("read case %d refines flag: %w", i, err)
		}
		if refinesFlag == 0x01 {
			idx, _, err := leb128.DecodeUint32(r)
			if err != nil {
				return TypeDef{}, fmt.Errorf("read case %d refines index: %w", i, err)
			}
			refines = &idx
		}

		// Read optional type
		var valType *ValTypeRef
		typeFlag, err := r.ReadByte()
		if err != nil {
			return TypeDef{}, fmt.Errorf("read case %d type flag: %w", i, err)
		}
		if typeFlag == 0x01 {
			vt, err := decodeValTypeRef(r)
			if err != nil {
				return TypeDef{}, fmt.Errorf("read case %d type: %w", i, err)
			}
			valType = &vt
		}

		cases[i] = VariantCase{Name: name, Refines: refines, Type: valType}
	}

	return TypeDef{
		Kind:    TypeDefKindVariant,
		Variant: &VariantTypeDef{Cases: cases},
	}, nil
}
```

**Step 3: Add to switch in decodeDefinedType**

```go
case ValTypeOpcodeVariant:
	return decodeVariantTypeDef(r)
```

**Step 4: Run test and commit**

---

### Task 42: Parse List Type Definition

**Files:**
- Modify: `internal/component/binary/types.go`
- Modify: `internal/component/binary/types_composite_test.go`

**Step 1: Write failing test**

```go
func TestDecodeListType(t *testing.T) {
	// list<s32>
	// Format: 0x70 <element_type>
	data := []byte{
		0x70,             // list opcode
		0x7a,             // s32 element type
	}

	r := bytes.NewReader(data)
	typeDef, err := decodeDefinedType(r)
	require.NoError(t, err)
	require.Equal(t, TypeDefKindList, typeDef.Kind)
	require.NotNil(t, typeDef.List)
}
```

**Step 2: Implement list parsing**

```go
// ListTypeDef represents a list type definition.
type ListTypeDef struct {
	Element ValTypeRef
}

func decodeListTypeDef(r *bytes.Reader) (TypeDef, error) {
	elemType, err := decodeValTypeRef(r)
	if err != nil {
		return TypeDef{}, fmt.Errorf("read list element type: %w", err)
	}

	return TypeDef{
		Kind: TypeDefKindList,
		List: &ListTypeDef{Element: elemType},
	}, nil
}
```

**Step 3: Add to switch and commit**

---

### Task 43: Parse Tuple, Flags, Enum Type Definitions

**Files:**
- Modify: `internal/component/binary/types.go`
- Modify: `internal/component/binary/types_composite_test.go`

**Step 1: Write failing tests**

```go
func TestDecodeTupleType(t *testing.T) {
	// tuple<s32, u64>
	// Format: 0x6f <count> <type>*
	data := []byte{
		0x6f,             // tuple opcode
		0x02,             // 2 elements
		0x7a,             // s32
		0x77,             // u64
	}

	r := bytes.NewReader(data)
	typeDef, err := decodeDefinedType(r)
	require.NoError(t, err)
	require.Equal(t, TypeDefKindTuple, typeDef.Kind)
	require.Len(t, typeDef.Tuple.Types, 2)
}

func TestDecodeFlagsType(t *testing.T) {
	// flags { read, write }
	// Format: 0x6e <count> <name>*
	data := []byte{
		0x6e,             // flags opcode
		0x02,             // 2 flags
		0x04, 'r', 'e', 'a', 'd',
		0x05, 'w', 'r', 'i', 't', 'e',
	}

	r := bytes.NewReader(data)
	typeDef, err := decodeDefinedType(r)
	require.NoError(t, err)
	require.Equal(t, TypeDefKindFlags, typeDef.Kind)
	require.Equal(t, []string{"read", "write"}, typeDef.Flags.Names)
}

func TestDecodeEnumType(t *testing.T) {
	// enum { red, green, blue }
	// Format: 0x6d <count> <name>*
	data := []byte{
		0x6d,             // enum opcode
		0x03,             // 3 cases
		0x03, 'r', 'e', 'd',
		0x05, 'g', 'r', 'e', 'e', 'n',
		0x04, 'b', 'l', 'u', 'e',
	}

	r := bytes.NewReader(data)
	typeDef, err := decodeDefinedType(r)
	require.NoError(t, err)
	require.Equal(t, TypeDefKindEnum, typeDef.Kind)
	require.Equal(t, []string{"red", "green", "blue"}, typeDef.Enum.Cases)
}
```

**Step 2: Implement all three**

```go
// TupleTypeDef represents a tuple type definition.
type TupleTypeDef struct {
	Types []ValTypeRef
}

// FlagsTypeDef represents a flags type definition.
type FlagsTypeDef struct {
	Names []string
}

// EnumTypeDef represents an enum type definition.
type EnumTypeDef struct {
	Cases []string
}

func decodeTupleTypeDef(r *bytes.Reader) (TypeDef, error) {
	count, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return TypeDef{}, fmt.Errorf("read tuple type count: %w", err)
	}

	types := make([]ValTypeRef, count)
	for i := uint32(0); i < count; i++ {
		vt, err := decodeValTypeRef(r)
		if err != nil {
			return TypeDef{}, fmt.Errorf("read tuple type %d: %w", i, err)
		}
		types[i] = vt
	}

	return TypeDef{
		Kind:  TypeDefKindTuple,
		Tuple: &TupleTypeDef{Types: types},
	}, nil
}

func decodeFlagsTypeDef(r *bytes.Reader) (TypeDef, error) {
	count, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return TypeDef{}, fmt.Errorf("read flags count: %w", err)
	}

	names := make([]string, count)
	for i := uint32(0); i < count; i++ {
		name, err := decodeName(r)
		if err != nil {
			return TypeDef{}, fmt.Errorf("read flag %d name: %w", i, err)
		}
		names[i] = name
	}

	return TypeDef{
		Kind:  TypeDefKindFlags,
		Flags: &FlagsTypeDef{Names: names},
	}, nil
}

func decodeEnumTypeDef(r *bytes.Reader) (TypeDef, error) {
	count, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return TypeDef{}, fmt.Errorf("read enum case count: %w", err)
	}

	cases := make([]string, count)
	for i := uint32(0); i < count; i++ {
		name, err := decodeName(r)
		if err != nil {
			return TypeDef{}, fmt.Errorf("read enum case %d: %w", i, err)
		}
		cases[i] = name
	}

	return TypeDef{
		Kind: TypeDefKindEnum,
		Enum: &EnumTypeDef{Cases: cases},
	}, nil
}
```

**Step 3: Add to switch and commit**

---

### Task 44: Parse Option Type Definition

**Files:**
- Modify: `internal/component/binary/types.go`
- Modify: `internal/component/binary/types_composite_test.go`

**Step 1: Write failing test**

```go
func TestDecodeOptionType(t *testing.T) {
	// option<s32>
	// Format: 0x6b <type>
	data := []byte{
		0x6b,             // option opcode
		0x7a,             // s32
	}

	r := bytes.NewReader(data)
	typeDef, err := decodeDefinedType(r)
	require.NoError(t, err)
	require.Equal(t, TypeDefKindOption, typeDef.Kind)
	require.NotNil(t, typeDef.Option)
}
```

**Step 2: Implement**

```go
// OptionTypeDef represents an option type definition.
type OptionTypeDef struct {
	Some ValTypeRef
}

func decodeOptionTypeDef(r *bytes.Reader) (TypeDef, error) {
	someType, err := decodeValTypeRef(r)
	if err != nil {
		return TypeDef{}, fmt.Errorf("read option some type: %w", err)
	}

	return TypeDef{
		Kind:   TypeDefKindOption,
		Option: &OptionTypeDef{Some: someType},
	}, nil
}
```

**Step 3: Add to switch and commit**

---

### Task 45: Parse Result Type Definition

**Files:**
- Modify: `internal/component/binary/types.go`
- Modify: `internal/component/binary/types_composite_test.go`

**Step 1: Write failing test**

```go
func TestDecodeResultType(t *testing.T) {
	// result<s32, string>
	// Format: 0x6a <ok_type>? <error_type>?
	data := []byte{
		0x6a,             // result opcode
		0x01,             // has ok type
		0x7a,             // s32
		0x01,             // has error type
		0x73,             // string
	}

	r := bytes.NewReader(data)
	typeDef, err := decodeDefinedType(r)
	require.NoError(t, err)
	require.Equal(t, TypeDefKindResult, typeDef.Kind)
	require.NotNil(t, typeDef.Result.Ok)
	require.NotNil(t, typeDef.Result.Error)
}

func TestDecodeResultTypeOkOnly(t *testing.T) {
	// result<s32, _>
	data := []byte{
		0x6a,             // result opcode
		0x01,             // has ok type
		0x7a,             // s32
		0x00,             // no error type
	}

	r := bytes.NewReader(data)
	typeDef, err := decodeDefinedType(r)
	require.NoError(t, err)
	require.NotNil(t, typeDef.Result.Ok)
	require.Nil(t, typeDef.Result.Error)
}

func TestDecodeResultTypeUnit(t *testing.T) {
	// result (no ok, no error)
	data := []byte{
		0x6a,             // result opcode
		0x00,             // no ok type
		0x00,             // no error type
	}

	r := bytes.NewReader(data)
	typeDef, err := decodeDefinedType(r)
	require.NoError(t, err)
	require.Nil(t, typeDef.Result.Ok)
	require.Nil(t, typeDef.Result.Error)
}
```

**Step 2: Implement**

```go
// ResultTypeDef represents a result type definition.
type ResultTypeDef struct {
	Ok    *ValTypeRef // nil for result<_, E>
	Error *ValTypeRef // nil for result<T, _>
}

func decodeResultTypeDef(r *bytes.Reader) (TypeDef, error) {
	var okType, errType *ValTypeRef

	// Read optional ok type
	okFlag, err := r.ReadByte()
	if err != nil {
		return TypeDef{}, fmt.Errorf("read result ok flag: %w", err)
	}
	if okFlag == 0x01 {
		vt, err := decodeValTypeRef(r)
		if err != nil {
			return TypeDef{}, fmt.Errorf("read result ok type: %w", err)
		}
		okType = &vt
	}

	// Read optional error type
	errFlag, err := r.ReadByte()
	if err != nil {
		return TypeDef{}, fmt.Errorf("read result error flag: %w", err)
	}
	if errFlag == 0x01 {
		vt, err := decodeValTypeRef(r)
		if err != nil {
			return TypeDef{}, fmt.Errorf("read result error type: %w", err)
		}
		errType = &vt
	}

	return TypeDef{
		Kind:   TypeDefKindResult,
		Result: &ResultTypeDef{Ok: okType, Error: errType},
	}, nil
}
```

**Step 3: Add to switch and commit**

```bash
git add internal/component/binary/
git commit -m "feat(component): parse all composite type definitions

- Add parsing for record, variant, list, tuple, flags, enum, option, result
- Complete type section parsing for all WIT types"
```

---

### Task 46: Extend Val for Record

**Files:**
- Modify: `internal/component/val.go`
- Modify: `internal/component/val_test.go`

**Step 1: Write failing test**

```go
func TestValRecord(t *testing.T) {
	// Create a record value { a: 42, b: "hello" }
	fields := map[string]Val{
		"a": ValS32(42),
		"b": ValString("hello"),
	}
	v := ValRecord(fields)

	require.Equal(t, ValKindRecord, v.Kind())

	got := v.Record()
	require.Equal(t, int32(42), got["a"].S32())
	require.Equal(t, "hello", got["b"].StringVal())
}

func TestValRecordField(t *testing.T) {
	fields := map[string]Val{
		"x": ValF64(3.14),
	}
	v := ValRecord(fields)

	// Access single field
	x, ok := v.RecordField("x")
	require.True(t, ok)
	require.Equal(t, 3.14, x.F64())

	_, ok = v.RecordField("missing")
	require.False(t, ok)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/component/... -v -run TestValRecord`
Expected: FAIL

**Step 3: Implement**

```go
// In val.go

// ValRecord creates a record value from field name to value map.
func ValRecord(fields map[string]Val) Val {
	return Val{kind: ValKindRecord, v: fields}
}

// Record returns the value as a record (map of field name to value).
func (v Val) Record() map[string]Val {
	if v.kind != ValKindRecord {
		panic("Val is not a record")
	}
	return v.v.(map[string]Val)
}

// RecordField returns a specific field from a record value.
func (v Val) RecordField(name string) (Val, bool) {
	r := v.Record()
	val, ok := r[name]
	return val, ok
}
```

**Step 4: Run test and commit**

---

### Task 47: Extend Val for Variant and Option

**Files:**
- Modify: `internal/component/val.go`
- Modify: `internal/component/val_test.go`

**Step 1: Write failing test**

```go
func TestValVariant(t *testing.T) {
	// Create variant { some: 42 }
	payload := ValS32(42)
	v := ValVariant("some", &payload)

	require.Equal(t, ValKindVariant, v.Kind())

	caseName, casePayload := v.Variant()
	require.Equal(t, "some", caseName)
	require.NotNil(t, casePayload)
	require.Equal(t, int32(42), casePayload.S32())
}

func TestValVariantNoPayload(t *testing.T) {
	// Create variant { none }
	v := ValVariant("none", nil)

	caseName, casePayload := v.Variant()
	require.Equal(t, "none", caseName)
	require.Nil(t, casePayload)
}

func TestValOption(t *testing.T) {
	// Some(42)
	payload := ValS32(42)
	v := ValOption(&payload)

	require.Equal(t, ValKindOption, v.Kind())

	opt := v.Option()
	require.NotNil(t, opt)
	require.Equal(t, int32(42), opt.S32())
}

func TestValOptionNone(t *testing.T) {
	// None
	v := ValOption(nil)

	opt := v.Option()
	require.Nil(t, opt)
}
```

**Step 2: Implement**

```go
// variantVal holds a variant's case name and optional payload.
type variantVal struct {
	caseName string
	payload  *Val
}

// ValVariant creates a variant value with the given case and optional payload.
func ValVariant(caseName string, payload *Val) Val {
	return Val{kind: ValKindVariant, v: variantVal{caseName: caseName, payload: payload}}
}

// Variant returns the variant's case name and optional payload.
func (v Val) Variant() (string, *Val) {
	if v.kind != ValKindVariant {
		panic("Val is not a variant")
	}
	vv := v.v.(variantVal)
	return vv.caseName, vv.payload
}

// ValOption creates an option value (Some or None).
func ValOption(payload *Val) Val {
	return Val{kind: ValKindOption, v: payload}
}

// Option returns the option's payload (nil for None).
func (v Val) Option() *Val {
	if v.kind != ValKindOption {
		panic("Val is not an option")
	}
	return v.v.(*Val)
}
```

**Step 3: Run test and commit**

---

### Task 48: Extend Val for List, Tuple, Result, Flags, Enum

**Files:**
- Modify: `internal/component/val.go`
- Modify: `internal/component/val_test.go`

**Step 1: Write failing tests**

```go
func TestValList(t *testing.T) {
	elements := []Val{ValS32(1), ValS32(2), ValS32(3)}
	v := ValList(elements)

	require.Equal(t, ValKindList, v.Kind())
	require.Equal(t, elements, v.List())
}

func TestValTuple(t *testing.T) {
	elements := []Val{ValS32(1), ValString("hello")}
	v := ValTuple(elements)

	require.Equal(t, ValKindTuple, v.Kind())
	require.Equal(t, elements, v.Tuple())
}

func TestValResult(t *testing.T) {
	// Ok(42)
	okVal := ValS32(42)
	v := ValResultOk(&okVal)

	require.Equal(t, ValKindResult, v.Kind())
	isOk, ok, err := v.Result()
	require.True(t, isOk)
	require.NotNil(t, ok)
	require.Equal(t, int32(42), ok.S32())
	require.Nil(t, err)
}

func TestValResultError(t *testing.T) {
	// Error("oops")
	errVal := ValString("oops")
	v := ValResultError(&errVal)

	isOk, ok, err := v.Result()
	require.False(t, isOk)
	require.Nil(t, ok)
	require.NotNil(t, err)
	require.Equal(t, "oops", err.StringVal())
}

func TestValFlags(t *testing.T) {
	flags := map[string]bool{"read": true, "write": false, "execute": true}
	v := ValFlags(flags)

	require.Equal(t, ValKindFlags, v.Kind())
	got := v.Flags()
	require.True(t, got["read"])
	require.False(t, got["write"])
	require.True(t, got["execute"])
}

func TestValEnum(t *testing.T) {
	v := ValEnum("green")

	require.Equal(t, ValKindEnum, v.Kind())
	require.Equal(t, "green", v.Enum())
}
```

**Step 2: Implement all**

```go
// ValList creates a list value.
func ValList(elements []Val) Val {
	return Val{kind: ValKindList, v: elements}
}

// List returns the value as a list.
func (v Val) List() []Val {
	if v.kind != ValKindList {
		panic("Val is not a list")
	}
	return v.v.([]Val)
}

// ValTuple creates a tuple value.
func ValTuple(elements []Val) Val {
	return Val{kind: ValKindTuple, v: elements}
}

// Tuple returns the value as a tuple.
func (v Val) Tuple() []Val {
	if v.kind != ValKindTuple {
		panic("Val is not a tuple")
	}
	return v.v.([]Val)
}

// resultVal holds a result's ok/error state.
type resultVal struct {
	isOk bool
	ok   *Val
	err  *Val
}

// ValResultOk creates a result value with an Ok payload.
func ValResultOk(ok *Val) Val {
	return Val{kind: ValKindResult, v: resultVal{isOk: true, ok: ok}}
}

// ValResultError creates a result value with an Error payload.
func ValResultError(err *Val) Val {
	return Val{kind: ValKindResult, v: resultVal{isOk: false, err: err}}
}

// Result returns the result's state and payloads.
func (v Val) Result() (isOk bool, ok *Val, err *Val) {
	if v.kind != ValKindResult {
		panic("Val is not a result")
	}
	rv := v.v.(resultVal)
	return rv.isOk, rv.ok, rv.err
}

// ValFlags creates a flags value.
func ValFlags(flags map[string]bool) Val {
	return Val{kind: ValKindFlags, v: flags}
}

// Flags returns the value as flags.
func (v Val) Flags() map[string]bool {
	if v.kind != ValKindFlags {
		panic("Val is not flags")
	}
	return v.v.(map[string]bool)
}

// ValEnum creates an enum value.
func ValEnum(caseName string) Val {
	return Val{kind: ValKindEnum, v: caseName}
}

// Enum returns the value as an enum case name.
func (v Val) Enum() string {
	if v.kind != ValKindEnum {
		panic("Val is not an enum")
	}
	return v.v.(string)
}
```

**Step 3: Run tests and commit**

```bash
git add internal/component/val.go internal/component/val_test.go
git commit -m "feat(component): add Val constructors for all composite types

- Add ValRecord, ValVariant, ValOption, ValList, ValTuple
- Add ValResultOk, ValResultError, ValFlags, ValEnum
- Add corresponding accessor methods"
```

---

### Task 49: Create ABI Package with LiftContext

**Files:**
- Create: `internal/component/abi/context.go`
- Create: `internal/component/abi/context_test.go`

**Step 1: Write failing test**

```go
// internal/component/abi/context_test.go

package abi

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestLiftContext(t *testing.T) {
	// Create a mock memory with some data
	data := make([]byte, 64)
	// Write u32 at offset 8
	binary.LittleEndian.PutUint32(data[8:], 42)

	ctx := &LiftContext{
		Memory: &mockMemory{data: data},
		Opts: &Options{
			StringEncoding: StringEncodingUTF8,
		},
	}

	// Read u32 from offset 8
	val := ctx.ReadU32(8)
	require.Equal(t, uint32(42), val)
}

type mockMemory struct {
	data []byte
}

func (m *mockMemory) Read(offset, size uint32) ([]byte, bool) {
	if int(offset+size) > len(m.data) {
		return nil, false
	}
	return m.data[offset : offset+size], true
}
```

**Step 2: Implement**

```go
// internal/component/abi/context.go

package abi

import (
	"encoding/binary"
)

// StringEncoding specifies the string encoding for Canonical ABI.
type StringEncoding uint8

const (
	StringEncodingUTF8 StringEncoding = iota
	StringEncodingUTF16
	StringEncodingLatin1UTF16
)

// Options holds Canonical ABI options from canonical definitions.
type Options struct {
	StringEncoding StringEncoding
	MemoryIdx      uint32
	ReallocIdx     *uint32
	PostReturnIdx  *uint32
}

// Memory interface for reading/writing linear memory.
type Memory interface {
	Read(offset, size uint32) ([]byte, bool)
	Write(offset uint32, data []byte) bool
	Size() uint32
}

// LiftContext provides context for lifting operations.
type LiftContext struct {
	Memory Memory
	Opts   *Options
}

// ReadU8 reads a u8 from memory at the given offset.
func (c *LiftContext) ReadU8(offset uint32) uint8 {
	data, _ := c.Memory.Read(offset, 1)
	return data[0]
}

// ReadU16 reads a u16 from memory at the given offset.
func (c *LiftContext) ReadU16(offset uint32) uint16 {
	data, _ := c.Memory.Read(offset, 2)
	return binary.LittleEndian.Uint16(data)
}

// ReadU32 reads a u32 from memory at the given offset.
func (c *LiftContext) ReadU32(offset uint32) uint32 {
	data, _ := c.Memory.Read(offset, 4)
	return binary.LittleEndian.Uint32(data)
}

// ReadU64 reads a u64 from memory at the given offset.
func (c *LiftContext) ReadU64(offset uint32) uint64 {
	data, _ := c.Memory.Read(offset, 8)
	return binary.LittleEndian.Uint64(data)
}

// ReadF32 reads a f32 from memory at the given offset.
func (c *LiftContext) ReadF32(offset uint32) float32 {
	bits := c.ReadU32(offset)
	return math.Float32frombits(bits)
}

// ReadF64 reads a f64 from memory at the given offset.
func (c *LiftContext) ReadF64(offset uint32) float64 {
	bits := c.ReadU64(offset)
	return math.Float64frombits(bits)
}
```

**Step 3: Run test and commit**

---

### Task 50: Implement Flat Lift for Primitives

**Files:**
- Create: `internal/component/abi/lift.go`
- Create: `internal/component/abi/lift_test.go`

**Step 1: Write failing test**

```go
// internal/component/abi/lift_test.go

package abi

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestLiftFlatS32(t *testing.T) {
	iter := &FlatIter{values: []uint64{42}}
	val, err := LiftFlat(nil, types.S32{}, iter)
	require.NoError(t, err)
	require.Equal(t, int32(42), val.S32())
}

func TestLiftFlatU64(t *testing.T) {
	iter := &FlatIter{values: []uint64{0xDEADBEEF12345678}}
	val, err := LiftFlat(nil, types.U64{}, iter)
	require.NoError(t, err)
	require.Equal(t, uint64(0xDEADBEEF12345678), val.U64())
}

func TestLiftFlatF32(t *testing.T) {
	bits := math.Float32bits(3.14)
	iter := &FlatIter{values: []uint64{uint64(bits)}}
	val, err := LiftFlat(nil, types.F32{}, iter)
	require.NoError(t, err)
	require.InDelta(t, float32(3.14), val.F32(), 0.001)
}

func TestLiftFlatBool(t *testing.T) {
	iter := &FlatIter{values: []uint64{1}}
	val, err := LiftFlat(nil, types.Bool{}, iter)
	require.NoError(t, err)
	require.True(t, val.Bool())
}
```

**Step 2: Implement**

```go
// internal/component/abi/lift.go

package abi

import (
	"fmt"
	"math"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/types"
)

// FlatIter iterates over flattened core wasm values.
type FlatIter struct {
	values []uint64
	pos    int
}

// NewFlatIter creates a new flat value iterator.
func NewFlatIter(values []uint64) *FlatIter {
	return &FlatIter{values: values}
}

// Next returns the next value as i32.
func (f *FlatIter) NextI32() uint32 {
	v := f.values[f.pos]
	f.pos++
	return uint32(v)
}

// NextI64 returns the next value as i64.
func (f *FlatIter) NextI64() uint64 {
	v := f.values[f.pos]
	f.pos++
	return v
}

// NextF32 returns the next value as f32.
func (f *FlatIter) NextF32() float32 {
	v := f.values[f.pos]
	f.pos++
	return math.Float32frombits(uint32(v))
}

// NextF64 returns the next value as f64.
func (f *FlatIter) NextF64() float64 {
	v := f.values[f.pos]
	f.pos++
	return math.Float64frombits(v)
}

// LiftFlat lifts a flat representation to a component Val.
func LiftFlat(ctx *LiftContext, typ types.ValType, iter *FlatIter) (component.Val, error) {
	switch typ.(type) {
	case types.Bool:
		return component.ValBool(iter.NextI32() != 0), nil
	case types.S8:
		return component.ValS8(int8(iter.NextI32())), nil
	case types.U8:
		return component.ValU8(uint8(iter.NextI32())), nil
	case types.S16:
		return component.ValS16(int16(iter.NextI32())), nil
	case types.U16:
		return component.ValU16(uint16(iter.NextI32())), nil
	case types.S32:
		return component.ValS32(int32(iter.NextI32())), nil
	case types.U32:
		return component.ValU32(iter.NextI32()), nil
	case types.S64:
		return component.ValS64(int64(iter.NextI64())), nil
	case types.U64:
		return component.ValU64(iter.NextI64()), nil
	case types.F32:
		return component.ValF32(iter.NextF32()), nil
	case types.F64:
		return component.ValF64(iter.NextF64()), nil
	case types.Char:
		return component.ValChar(rune(iter.NextI32())), nil
	default:
		return component.Val{}, fmt.Errorf("unsupported flat lift for type: %T", typ)
	}
}
```

**Step 3: Run test and commit**

---

### Task 51: Implement Flat Lower for Primitives

**Files:**
- Create: `internal/component/abi/lower.go`
- Create: `internal/component/abi/lower_test.go`

**Step 1: Write failing test**

```go
// internal/component/abi/lower_test.go

package abi

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestLowerFlatS32(t *testing.T) {
	val := component.ValS32(-42)
	flat, err := LowerFlat(nil, types.S32{}, val)
	require.NoError(t, err)
	require.Equal(t, []uint64{uint64(int32(-42))}, flat)
}

func TestLowerFlatU64(t *testing.T) {
	val := component.ValU64(0xDEADBEEF12345678)
	flat, err := LowerFlat(nil, types.U64{}, val)
	require.NoError(t, err)
	require.Equal(t, []uint64{0xDEADBEEF12345678}, flat)
}

func TestLowerFlatBool(t *testing.T) {
	val := component.ValBool(true)
	flat, err := LowerFlat(nil, types.Bool{}, val)
	require.NoError(t, err)
	require.Equal(t, []uint64{1}, flat)
}
```

**Step 2: Implement**

```go
// internal/component/abi/lower.go

package abi

import (
	"fmt"
	"math"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/types"
)

// LowerContext provides context for lowering operations.
type LowerContext struct {
	Memory  Memory
	Opts    *Options
	Realloc func(oldPtr, oldSize, align, newSize uint32) (uint32, error)
}

// LowerFlat lowers a component Val to flat core wasm values.
func LowerFlat(ctx *LowerContext, typ types.ValType, val component.Val) ([]uint64, error) {
	switch typ.(type) {
	case types.Bool:
		if val.Bool() {
			return []uint64{1}, nil
		}
		return []uint64{0}, nil
	case types.S8:
		return []uint64{uint64(uint32(int32(val.S8())))}, nil
	case types.U8:
		return []uint64{uint64(val.U8())}, nil
	case types.S16:
		return []uint64{uint64(uint32(int32(val.S16())))}, nil
	case types.U16:
		return []uint64{uint64(val.U16())}, nil
	case types.S32:
		return []uint64{uint64(uint32(val.S32()))}, nil
	case types.U32:
		return []uint64{uint64(val.U32())}, nil
	case types.S64:
		return []uint64{uint64(val.S64())}, nil
	case types.U64:
		return []uint64{val.U64()}, nil
	case types.F32:
		return []uint64{uint64(math.Float32bits(val.F32()))}, nil
	case types.F64:
		return []uint64{math.Float64bits(val.F64())}, nil
	case types.Char:
		return []uint64{uint64(val.Char())}, nil
	default:
		return nil, fmt.Errorf("unsupported flat lower for type: %T", typ)
	}
}
```

**Step 3: Run test and commit**

---

### Task 52: Implement Flat Lift/Lower for Record

**Files:**
- Modify: `internal/component/abi/lift.go`
- Modify: `internal/component/abi/lower.go`
- Modify: `internal/component/abi/lift_test.go`
- Modify: `internal/component/abi/lower_test.go`

**Step 1: Write failing test**

```go
func TestLiftFlatRecord(t *testing.T) {
	// Record { a: s32, b: u64 }
	// Flat: [i32, i64]
	iter := &FlatIter{values: []uint64{42, 100}}
	recType := types.Record{
		Fields: []types.Field{
			{Name: "a", Type: types.S32{}},
			{Name: "b", Type: types.U64{}},
		},
	}

	val, err := LiftFlat(nil, recType, iter)
	require.NoError(t, err)
	require.Equal(t, component.ValKindRecord, val.Kind())

	rec := val.Record()
	require.Equal(t, int32(42), rec["a"].S32())
	require.Equal(t, uint64(100), rec["b"].U64())
}

func TestLowerFlatRecord(t *testing.T) {
	recType := types.Record{
		Fields: []types.Field{
			{Name: "a", Type: types.S32{}},
			{Name: "b", Type: types.U64{}},
		},
	}
	val := component.ValRecord(map[string]component.Val{
		"a": component.ValS32(42),
		"b": component.ValU64(100),
	})

	flat, err := LowerFlat(nil, recType, val)
	require.NoError(t, err)
	require.Equal(t, []uint64{42, 100}, flat)
}
```

**Step 2: Implement in lift.go and lower.go**

```go
// In lift.go, add to LiftFlat switch:
case types.Record:
	fields := make(map[string]component.Val)
	for _, f := range t.Fields {
		fieldVal, err := LiftFlat(ctx, f.Type, iter)
		if err != nil {
			return component.Val{}, fmt.Errorf("lift record field %s: %w", f.Name, err)
		}
		fields[f.Name] = fieldVal
	}
	return component.ValRecord(fields), nil

// In lower.go, add to LowerFlat switch:
case types.Record:
	rec := val.Record()
	var result []uint64
	for _, f := range t.Fields {
		fieldVal, ok := rec[f.Name]
		if !ok {
			return nil, fmt.Errorf("missing record field: %s", f.Name)
		}
		flat, err := LowerFlat(ctx, f.Type, fieldVal)
		if err != nil {
			return nil, fmt.Errorf("lower record field %s: %w", f.Name, err)
		}
		result = append(result, flat...)
	}
	return result, nil
```

**Step 3: Run tests and commit**

---

### Task 53: Implement Flat Lift/Lower for Variant

**Files:**
- Modify: `internal/component/abi/lift.go`
- Modify: `internal/component/abi/lower.go`

**Step 1: Write failing test**

```go
func TestLiftFlatVariant(t *testing.T) {
	// variant { none, some(s32) }
	// Flat for some(42): [i32(case=1), i32(payload=42)]
	iter := &FlatIter{values: []uint64{1, 42}}
	varType := types.Variant{
		Cases: []types.Case{
			{Name: "none", Type: nil},
			{Name: "some", Type: types.S32{}},
		},
	}

	val, err := LiftFlat(nil, varType, iter)
	require.NoError(t, err)

	caseName, payload := val.Variant()
	require.Equal(t, "some", caseName)
	require.NotNil(t, payload)
	require.Equal(t, int32(42), payload.S32())
}

func TestLiftFlatVariantNoPayload(t *testing.T) {
	// variant { none, some(s32) }
	// Flat for none: [i32(case=0), i32(padding=0)]
	iter := &FlatIter{values: []uint64{0, 0}}
	varType := types.Variant{
		Cases: []types.Case{
			{Name: "none", Type: nil},
			{Name: "some", Type: types.S32{}},
		},
	}

	val, err := LiftFlat(nil, varType, iter)
	require.NoError(t, err)

	caseName, payload := val.Variant()
	require.Equal(t, "none", caseName)
	require.Nil(t, payload)
}
```

**Step 2: Implement**

```go
// In lift.go, add to LiftFlat switch:
case types.Variant:
	disc := iter.NextI32()
	if int(disc) >= len(t.Cases) {
		return component.Val{}, fmt.Errorf("invalid variant discriminant: %d", disc)
	}
	c := t.Cases[disc]

	// Calculate max payload flatten count for padding
	maxPayloadFlat := 0
	for _, vc := range t.Cases {
		if vc.Type != nil {
			if n := vc.Type.FlattenCount(); n > maxPayloadFlat {
				maxPayloadFlat = n
			}
		}
	}

	var payload *component.Val
	payloadConsumed := 0
	if c.Type != nil {
		p, err := LiftFlat(ctx, c.Type, iter)
		if err != nil {
			return component.Val{}, fmt.Errorf("lift variant payload: %w", err)
		}
		payload = &p
		payloadConsumed = c.Type.FlattenCount()
	}

	// Skip remaining padding values
	for i := payloadConsumed; i < maxPayloadFlat; i++ {
		iter.NextI64() // Consume as i64 (largest type)
	}

	return component.ValVariant(c.Name, payload), nil
```

**Step 3: Run tests and commit**

---

### Task 54: Implement Flat Lift/Lower for Remaining Composites

**Files:**
- Modify: `internal/component/abi/lift.go`
- Modify: `internal/component/abi/lower.go`

This task implements flat lift/lower for:
- List (only ptr+len, actual elements via heap)
- Tuple (like record)
- Option (like variant)
- Result (like variant)
- Flags (packed bits)
- Enum (single discriminant)

**Step 1: Write tests for each**

**Step 2: Implement each in the switch statement**

**Step 3: Run tests and commit**

```bash
git commit -m "feat(component): implement flat ABI lift/lower for all composite types"
```

---

### Task 55: Implement Heap Lift for Record

**Files:**
- Modify: `internal/component/abi/lift.go`

**Step 1: Write failing test**

```go
func TestLiftHeapRecord(t *testing.T) {
	// Record { a: u8, b: u32, c: u16 } at offset 16
	// Layout: u8@0, padding@1-3, u32@4, u16@8
	data := make([]byte, 32)
	data[16] = 0x42        // a = 0x42
	binary.LittleEndian.PutUint32(data[20:], 0xDEADBEEF) // b
	binary.LittleEndian.PutUint16(data[24:], 0x1234)     // c

	ctx := &LiftContext{Memory: &mockMemory{data: data}}
	recType := types.Record{
		Fields: []types.Field{
			{Name: "a", Type: types.U8{}},
			{Name: "b", Type: types.U32{}},
			{Name: "c", Type: types.U16{}},
		},
	}

	val, err := LiftHeap(ctx, recType, 16)
	require.NoError(t, err)

	rec := val.Record()
	require.Equal(t, uint8(0x42), rec["a"].U8())
	require.Equal(t, uint32(0xDEADBEEF), rec["b"].U32())
	require.Equal(t, uint16(0x1234), rec["c"].U16())
}
```

**Step 2: Implement LiftHeap function**

```go
// LiftHeap lifts a value from heap memory at the given offset.
func LiftHeap(ctx *LiftContext, typ types.ValType, offset uint32) (component.Val, error) {
	switch t := typ.(type) {
	// Primitives
	case types.Bool:
		return component.ValBool(ctx.ReadU8(offset) != 0), nil
	case types.U8:
		return component.ValU8(ctx.ReadU8(offset)), nil
	case types.S8:
		return component.ValS8(int8(ctx.ReadU8(offset))), nil
	case types.U16:
		return component.ValU16(ctx.ReadU16(offset)), nil
	case types.S16:
		return component.ValS16(int16(ctx.ReadU16(offset))), nil
	case types.U32:
		return component.ValU32(ctx.ReadU32(offset)), nil
	case types.S32:
		return component.ValS32(int32(ctx.ReadU32(offset))), nil
	case types.U64:
		return component.ValU64(ctx.ReadU64(offset)), nil
	case types.S64:
		return component.ValS64(int64(ctx.ReadU64(offset))), nil
	case types.F32:
		return component.ValF32(ctx.ReadF32(offset)), nil
	case types.F64:
		return component.ValF64(ctx.ReadF64(offset)), nil
	case types.Char:
		return component.ValChar(rune(ctx.ReadU32(offset))), nil

	// Record
	case types.Record:
		fields := make(map[string]component.Val)
		fieldOffsets := t.FieldOffsets()
		for i, f := range t.Fields {
			fieldVal, err := LiftHeap(ctx, f.Type, offset+fieldOffsets[i])
			if err != nil {
				return component.Val{}, fmt.Errorf("lift record field %s: %w", f.Name, err)
			}
			fields[f.Name] = fieldVal
		}
		return component.ValRecord(fields), nil

	default:
		return component.Val{}, fmt.Errorf("unsupported heap lift for type: %T", typ)
	}
}
```

**Step 3: Run test and commit**

---

### Task 56-60: Implement Heap Lift/Lower for Remaining Composites

These tasks follow the same pattern as Task 55, implementing heap operations for:
- **Task 56:** Variant heap lift/lower
- **Task 57:** List heap lift/lower (with realloc for lower)
- **Task 58:** Option, Result (delegate to variant)
- **Task 59:** Flags, Enum heap lift/lower
- **Task 60:** Tuple heap lift/lower

Each task:
1. Write failing test with known memory layout
2. Implement the case in LiftHeap/LowerHeap switch
3. Run test and commit

---

### Task 61: Implement String UTF-8 Encoding

**Files:**
- Create: `internal/component/abi/strings.go`
- Create: `internal/component/abi/strings_test.go`

**Step 1: Write failing test**

```go
// internal/component/abi/strings_test.go

package abi

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestLiftStringUTF8(t *testing.T) {
	// "hello" in UTF-8 at offset 16
	data := make([]byte, 32)
	copy(data[16:], "hello")
	// String stored as (ptr=16, len=5)
	binary.LittleEndian.PutUint32(data[0:], 16) // ptr
	binary.LittleEndian.PutUint32(data[4:], 5)  // len

	ctx := &LiftContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingUTF8},
	}

	val, err := LiftString(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, "hello", val)
}

func TestLiftStringUTF8_Unicode(t *testing.T) {
	// "日本語" (9 bytes in UTF-8)
	data := make([]byte, 32)
	copy(data[16:], "日本語")
	binary.LittleEndian.PutUint32(data[0:], 16)
	binary.LittleEndian.PutUint32(data[4:], 9)

	ctx := &LiftContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingUTF8},
	}

	val, err := LiftString(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, "日本語", val)
}
```

**Step 2: Implement**

```go
// internal/component/abi/strings.go

package abi

import (
	"encoding/binary"
	"fmt"
	"unicode/utf16"
	"unicode/utf8"
)

const UTF16Tag = uint32(1 << 31)

// LiftString lifts a string from memory.
func LiftString(ctx *LiftContext, offset uint32) (string, error) {
	ptr := ctx.ReadU32(offset)
	taggedLen := ctx.ReadU32(offset + 4)

	switch ctx.Opts.StringEncoding {
	case StringEncodingUTF8:
		return liftStringUTF8(ctx, ptr, taggedLen)
	case StringEncodingUTF16:
		return liftStringUTF16(ctx, ptr, taggedLen)
	case StringEncodingLatin1UTF16:
		return liftStringLatin1UTF16(ctx, ptr, taggedLen)
	default:
		return "", fmt.Errorf("unknown string encoding: %d", ctx.Opts.StringEncoding)
	}
}

func liftStringUTF8(ctx *LiftContext, ptr, byteLen uint32) (string, error) {
	data, ok := ctx.Memory.Read(ptr, byteLen)
	if !ok {
		return "", fmt.Errorf("failed to read string bytes at %d len %d", ptr, byteLen)
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("invalid UTF-8 string")
	}
	return string(data), nil
}

func liftStringUTF16(ctx *LiftContext, ptr, codeUnits uint32) (string, error) {
	byteLen := codeUnits * 2
	data, ok := ctx.Memory.Read(ptr, byteLen)
	if !ok {
		return "", fmt.Errorf("failed to read UTF-16 string at %d len %d", ptr, byteLen)
	}

	// Decode UTF-16 LE
	u16 := make([]uint16, codeUnits)
	for i := uint32(0); i < codeUnits; i++ {
		u16[i] = binary.LittleEndian.Uint16(data[i*2:])
	}

	return string(utf16.Decode(u16)), nil
}

func liftStringLatin1UTF16(ctx *LiftContext, ptr, taggedLen uint32) (string, error) {
	if taggedLen&UTF16Tag != 0 {
		// UTF-16 encoded
		codeUnits := taggedLen &^ UTF16Tag
		return liftStringUTF16(ctx, ptr, codeUnits)
	}
	// Latin-1 encoded (each byte is a code point)
	data, ok := ctx.Memory.Read(ptr, taggedLen)
	if !ok {
		return "", fmt.Errorf("failed to read Latin-1 string at %d len %d", ptr, taggedLen)
	}
	// Latin-1 is a subset of Unicode, direct conversion
	runes := make([]rune, taggedLen)
	for i, b := range data {
		runes[i] = rune(b)
	}
	return string(runes), nil
}
```

**Step 3: Run test and commit**

---

### Task 62-63: Implement String UTF-16 and Latin1+UTF16 Encoding

Follow the same pattern as Task 61 for:
- **Task 62:** UTF-16 encoding tests and edge cases
- **Task 63:** Latin1+UTF16 encoding with tag bit handling

---

### Task 64: Implement String Lowering

**Files:**
- Modify: `internal/component/abi/strings.go`

**Step 1: Write failing test**

```go
func TestLowerStringUTF8(t *testing.T) {
	data := make([]byte, 64)
	allocPtr := uint32(16)

	ctx := &LowerContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			return allocPtr, nil
		},
	}

	ptr, codeUnits, err := LowerString(ctx, "hello")
	require.NoError(t, err)
	require.Equal(t, uint32(16), ptr)
	require.Equal(t, uint32(5), codeUnits)
	require.Equal(t, "hello", string(data[16:21]))
}
```

**Step 2: Implement**

```go
// LowerString lowers a string to memory.
func LowerString(ctx *LowerContext, s string) (ptr, taggedLen uint32, err error) {
	switch ctx.Opts.StringEncoding {
	case StringEncodingUTF8:
		return lowerStringUTF8(ctx, s)
	case StringEncodingUTF16:
		return lowerStringUTF16(ctx, s)
	case StringEncodingLatin1UTF16:
		return lowerStringLatin1UTF16(ctx, s)
	default:
		return 0, 0, fmt.Errorf("unknown string encoding: %d", ctx.Opts.StringEncoding)
	}
}

func lowerStringUTF8(ctx *LowerContext, s string) (uint32, uint32, error) {
	data := []byte(s)
	byteLen := uint32(len(data))

	ptr, err := ctx.Realloc(0, 0, 1, byteLen)
	if err != nil {
		return 0, 0, fmt.Errorf("realloc for string: %w", err)
	}

	if !ctx.Memory.Write(ptr, data) {
		return 0, 0, fmt.Errorf("failed to write string at %d", ptr)
	}

	return ptr, byteLen, nil
}

func lowerStringUTF16(ctx *LowerContext, s string) (uint32, uint32, error) {
	runes := []rune(s)
	u16 := utf16.Encode(runes)
	byteLen := uint32(len(u16) * 2)

	ptr, err := ctx.Realloc(0, 0, 2, byteLen)
	if err != nil {
		return 0, 0, fmt.Errorf("realloc for UTF-16 string: %w", err)
	}

	data := make([]byte, byteLen)
	for i, u := range u16 {
		binary.LittleEndian.PutUint16(data[i*2:], u)
	}

	if !ctx.Memory.Write(ptr, data) {
		return 0, 0, fmt.Errorf("failed to write UTF-16 string at %d", ptr)
	}

	return ptr, uint32(len(u16)), nil
}

func lowerStringLatin1UTF16(ctx *LowerContext, s string) (uint32, uint32, error) {
	// Try Latin-1 first
	canLatin1 := true
	for _, r := range s {
		if r > 0xFF {
			canLatin1 = false
			break
		}
	}

	if canLatin1 {
		// Store as Latin-1
		data := make([]byte, len(s))
		for i, r := range s {
			data[i] = byte(r)
		}
		ptr, err := ctx.Realloc(0, 0, 1, uint32(len(data)))
		if err != nil {
			return 0, 0, err
		}
		if !ctx.Memory.Write(ptr, data) {
			return 0, 0, fmt.Errorf("failed to write Latin-1 string")
		}
		return ptr, uint32(len(data)), nil
	}

	// Fall back to UTF-16
	ptr, codeUnits, err := lowerStringUTF16(ctx, s)
	if err != nil {
		return 0, 0, err
	}
	return ptr, codeUnits | UTF16Tag, nil
}
```

**Step 3: Run test and commit**

---

### Task 65: String Type Integration

**Files:**
- Modify: `internal/component/abi/lift.go`
- Modify: `internal/component/abi/lower.go`

Integrate string lift/lower into the main LiftFlat/LiftHeap and LowerFlat/LowerHeap functions.

```go
// In lift.go LiftFlat:
case types.String:
	ptr := iter.NextI32()
	taggedLen := iter.NextI32()
	s, err := liftStringFromPtrLen(ctx, ptr, taggedLen)
	if err != nil {
		return component.Val{}, err
	}
	return component.ValString(s), nil

// In lift.go LiftHeap:
case types.String:
	s, err := LiftString(ctx, offset)
	if err != nil {
		return component.Val{}, err
	}
	return component.ValString(s), nil
```

---

### Task 66: Create echo_record Test Component

**Files:**
- Create: `internal/component/testdata/composites/echo_record.wit`
- Create: `internal/component/testdata/composites/src/echo_record.rs`
- Build: `internal/component/testdata/composites/echo_record.wasm`

**Step 1: Create WIT interface**

```wit
// internal/component/testdata/composites/echo_record.wit

package test:composites;

interface types {
    record point {
        x: s32,
        y: s32,
    }
}

world echo-record {
    use types.{point};
    export echo: func(p: point) -> point;
}
```

**Step 2: Create Rust implementation**

```rust
// internal/component/testdata/composites/src/echo_record.rs

wit_bindgen::generate!({
    world: "echo-record",
});

struct Component;

impl Guest for Component {
    fn echo(p: Point) -> Point {
        Point { x: p.x * 2, y: p.y * 2 }
    }
}

export!(Component);
```

**Step 3: Build with cargo-component**

```bash
cd internal/component/testdata/composites
cargo component build --release
cp target/wasm32-wasip1/release/echo_record.wasm .
```

**Step 4: Commit**

```bash
git add internal/component/testdata/composites/
git commit -m "test(component): add echo_record test component"
```

---

### Task 67: Test Record Round-Trip

**Files:**
- Create: `internal/component/composite_test.go`

**Step 1: Write integration test**

```go
// internal/component/composite_test.go

package component_test

import (
	"context"
	"embed"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

//go:embed testdata/composites/*.wasm
var compositesFS embed.FS

func TestEchoRecord(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	binary, err := compositesFS.ReadFile("testdata/composites/echo_record.wasm")
	require.NoError(t, err)

	comp, err := component.DecodeComponent(binary)
	require.NoError(t, err)

	inst, err := component.Instantiate(ctx, rt, comp)
	require.NoError(t, err)

	echo := inst.ExportedFunction("echo")
	require.NotNil(t, echo)

	// Call with record { x: 10, y: 20 }
	input := component.ValRecord(map[string]component.Val{
		"x": component.ValS32(10),
		"y": component.ValS32(20),
	})

	results, err := echo.Call(ctx, input)
	require.NoError(t, err)
	require.Len(t, results, 1)

	// Expect { x: 20, y: 40 }
	rec := results[0].Record()
	require.Equal(t, int32(20), rec["x"].S32())
	require.Equal(t, int32(40), rec["y"].S32())
}
```

**Step 2: Run test**

Run: `go test ./internal/component/... -v -run TestEchoRecord`

**Step 3: Commit**

---

### Task 68-69: Create and Test Additional Composite Components

**Task 68:** Create and test `option_roundtrip.wasm`
- `func(option<s32>) -> option<s32>`
- Test Some and None cases

**Task 69:** Create and test `list_sum.wasm`
- `func(list<s32>) -> s32`
- Test empty list, single element, multiple elements

---

### Task 70: Create and Test Result Component

**Files:**
- Create: `internal/component/testdata/composites/result_test.wit`
- Create: `internal/component/testdata/composites/src/result_test.rs`
- Build and test

**Step 1: Create WIT**

```wit
// result_test.wit

package test:composites;

world result-test {
    export divide: func(a: s32, b: s32) -> result<s32, string>;
}
```

**Step 2: Create Rust implementation**

```rust
wit_bindgen::generate!({
    world: "result-test",
});

struct Component;

impl Guest for Component {
    fn divide(a: i32, b: i32) -> Result<i32, String> {
        if b == 0 {
            Err("division by zero".to_string())
        } else {
            Ok(a / b)
        }
    }
}

export!(Component);
```

**Step 3: Write integration test**

```go
func TestResultComponent(t *testing.T) {
	// ... setup ...

	divide := inst.ExportedFunction("divide")

	// Test Ok case
	results, err := divide.Call(ctx, component.ValS32(10), component.ValS32(2))
	require.NoError(t, err)
	isOk, ok, errVal := results[0].Result()
	require.True(t, isOk)
	require.Equal(t, int32(5), ok.S32())

	// Test Error case
	results, err = divide.Call(ctx, component.ValS32(10), component.ValS32(0))
	require.NoError(t, err)
	isOk, ok, errVal = results[0].Result()
	require.False(t, isOk)
	require.Equal(t, "division by zero", errVal.StringVal())
}
```

**Step 4: Commit**

```bash
git add internal/component/
git commit -m "test(component): complete Phase 2 integration tests

- Add echo_record, option_roundtrip, list_sum, result_test components
- Verify round-trip for all composite types"
```

---

## Running Tests

```bash
# Run all Phase 2 tests
go test ./internal/component/types/... -v
go test ./internal/component/abi/... -v
go test ./internal/component/... -v -run "Test.*Record\|Test.*Variant\|Test.*List\|Test.*Option\|Test.*Result\|Test.*Flags\|Test.*Enum\|Test.*Tuple"

# Run specific composite type tests
go test ./internal/component/types/... -v -run TestRecord
go test ./internal/component/abi/... -v -run TestLiftFlat

# Run integration tests
go test ./internal/component/... -v -run "TestEcho\|TestOption\|TestList\|TestResult"

# Run with race detector
go test ./internal/component/... -race -v

# Run benchmarks
go test ./internal/component/abi/... -bench=. -benchmem
```

---

## Phase 2 Completion Criteria

- [ ] All 8 composite types defined with correct Size/Align/FlattenCount
- [ ] Binary parser handles all composite type opcodes (0x6b-0x72)
- [ ] Val has constructors and accessors for all composite types
- [ ] Flat ABI lift/lower works for types fitting in registers
- [ ] Heap ABI lift/lower works for types via memory
- [ ] All 3 string encodings work (UTF-8, UTF-16, Latin1+UTF16)
- [ ] Integration tests pass for record, variant, list, option, result
- [ ] All tests pass with race detector

---

## References

- [Canonical ABI Specification](https://github.com/WebAssembly/component-model/blob/main/design/mvp/CanonicalABI.md)
- [Canonical ABI Python Implementation](https://github.com/WebAssembly/component-model/blob/main/design/mvp/canonical-abi/definitions.py)
- [WIT Types Reference](https://component-model.bytecodealliance.org/design/wit.html#built-in-types)
- [Component Model Binary Format - Types](https://github.com/WebAssembly/component-model/blob/main/design/mvp/Binary.md#type-definitions)
