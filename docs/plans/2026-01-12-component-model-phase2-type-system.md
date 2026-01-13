# Component Model Phase 2: Complete Type System

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Parent Plan:** [2026-01-12-component-model-implementation.md](./2026-01-12-component-model-implementation.md)
**Design Doc:** [2026-01-12-component-model-design.md](./2026-01-12-component-model-design.md)
**Previous Phase:** [Phase 1: Binary Parser & Primitives](./2026-01-12-component-model-phase1-binary-parser.md)
**Status:** NOT STARTED
**Estimated Tasks:** 31-70

---

## Overview

This phase completes the component type system by implementing all WIT composite types and their Canonical ABI lift/lower operations.

**Goal:** Support all WIT types: record, variant, list, option, result, flags, enum, tuple with full Canonical ABI round-trip.

**Prerequisites:**
- Phase 1 complete (binary parser, primitive types, basic instantiation)
- `add(s32, s32) -> s32` working end-to-end

---

## Phase 2 Milestones

| Milestone | Description | Success Criteria |
|-----------|-------------|------------------|
| 2.1 | Composite type definitions | Record, variant, list, option, result, flags, enum, tuple types compile |
| 2.2 | Memory layout calculations | `Size()`, `Align()`, `FlattenCount()` correct for all composite types |
| 2.3 | Flat ABI lift/lower | Composite types that fit in registers work |
| 2.4 | Heap ABI lift/lower | Large types via memory pointers work |
| 2.5 | String encodings | UTF-8, UTF-16, Latin1+UTF16 all work |
| 2.6 | Integration tests | Round-trip all composite types through real components |

---

## Type System Reference

From the design doc, these composite types need implementation:

```go
// internal/component/types/composite.go

type Record struct {
    Fields []Field  // Named fields in order
}

type Variant struct {
    Cases []Case    // Discriminant + optional payload
}

type List struct {
    Element ValType  // Element type
}

type Option struct {
    Some ValType     // None represented as discriminant 0
}

type Result struct {
    Ok    ValType    // May be nil (no payload)
    Error ValType    // May be nil (no payload)
}

type Flags struct {
    Names []string   // Flag names, packed into u8/u16/u32
}

type Enum struct {
    Cases []string   // Discriminant-only variant
}

type Tuple struct {
    Types []ValType  // Positional types
}
```

---

## Canonical ABI Reference

From the spec, key constants and layout rules:

```go
const MaxFlatParams = 16
const MaxFlatResults = 1

// Memory layout follows C struct packing rules:
// - Each field aligned to its alignment requirement
// - Struct size padded to largest field alignment
// - Variant discriminant size based on case count

// String encoding options:
// - UTF-8: direct encoding
// - UTF-16: 2 bytes per code unit
// - Latin1+UTF16: Latin1 if all chars <= 0xFF, else UTF-16
```

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
```

**Step 2: Implement Record type**

```go
// internal/component/types/composite.go

package types

// Field represents a named field in a record.
type Field struct {
	Name string
	Type ValType
}

// Record represents a record (struct) type.
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

func alignTo(offset, align uint32) uint32 {
	return (offset + align - 1) &^ (align - 1)
}
```

**Step 3: Run test and verify**

Run: `go test ./internal/component/types/... -v -run TestRecord`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/component/types/
git commit -m "feat(component): add Record composite type with memory layout"
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
	// Size: 1 + 3 padding + 4 = 8, Align: 4
	v := Variant{
		Cases: []Case{
			{Name: "none", Type: nil},
			{Name: "some", Type: U32{}},
		},
	}

	require.Equal(t, uint32(8), v.Size())
	require.Equal(t, uint32(4), v.Align())
}
```

**Step 2: Implement Variant type**

```go
// Case represents a case in a variant.
type Case struct {
	Name string
	Type ValType // nil for cases with no payload
}

// Variant represents a discriminated union type.
type Variant struct {
	Cases []Case
}

func (Variant) valType() {}

func (v Variant) discriminantSize() uint32 {
	n := len(v.Cases)
	switch {
	case n <= 0x100:
		return 1
	case n <= 0x10000:
		return 2
	default:
		return 4
	}
}

func (v Variant) Size() uint32 {
	discSize := v.discriminantSize()
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
	// discriminant + padding + payload
	offset := alignTo(discSize, payloadAlign)
	return alignTo(offset+payloadSize, v.Align())
}

func (v Variant) Align() uint32 {
	align := v.discriminantSize()
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
	// discriminant + max payload
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
```

**Step 3: Run test and commit**

---

### Task 33: Define List Type

**Step 1: Write failing test**

```go
func TestListType(t *testing.T) {
	// list<u32>
	// In memory: (ptr: i32, len: i32)
	// Size: 8, Align: 4, Flatten: 2
	l := List{Element: U32{}}

	require.Equal(t, uint32(8), l.Size())
	require.Equal(t, uint32(4), l.Align())
	require.Equal(t, 2, l.FlattenCount())
}
```

**Step 2: Implement**

```go
// List represents a list (dynamic array) type.
type List struct {
	Element ValType
}

func (List) valType() {}
func (List) Size() uint32     { return 8 }  // ptr + len
func (List) Align() uint32    { return 4 }  // i32 alignment
func (List) FlattenCount() int { return 2 } // ptr, len
```

---

### Task 34: Define Option Type

**Step 1: Write failing test**

```go
func TestOptionType(t *testing.T) {
	// option<u32> is variant { none, some(u32) }
	o := Option{Some: U32{}}

	// Same layout as variant with 2 cases
	require.Equal(t, uint32(8), o.Size())
	require.Equal(t, uint32(4), o.Align())
}
```

**Step 2: Implement**

```go
// Option represents an optional value type.
type Option struct {
	Some ValType
}

func (Option) valType() {}

func (o Option) Size() uint32 {
	// Option is variant { none, some(T) }
	v := o.toVariant()
	return v.Size()
}

func (o Option) Align() uint32 {
	v := o.toVariant()
	return v.Align()
}

func (o Option) FlattenCount() int {
	v := o.toVariant()
	return v.FlattenCount()
}

func (o Option) toVariant() Variant {
	return Variant{
		Cases: []Case{
			{Name: "none", Type: nil},
			{Name: "some", Type: o.Some},
		},
	}
}
```

---

### Task 35: Define Result Type

**Step 1: Write failing test**

```go
func TestResultType(t *testing.T) {
	// result<u32, string>
	r := Result{Ok: U32{}, Error: String{}}

	// variant { ok(u32), error(string) }
	// Discriminant: 1 byte
	// Payload: max(4, 8) = 8
	require.Equal(t, uint32(12), r.Size())
	require.Equal(t, uint32(4), r.Align())
}
```

**Step 2: Implement**

```go
// Result represents a result type (ok or error).
type Result struct {
	Ok    ValType // nil for result<_, E>
	Error ValType // nil for result<T, _>
}

func (Result) valType() {}

func (r Result) toVariant() Variant {
	cases := []Case{
		{Name: "ok", Type: r.Ok},
		{Name: "error", Type: r.Error},
	}
	return Variant{Cases: cases}
}

func (r Result) Size() uint32     { return r.toVariant().Size() }
func (r Result) Align() uint32    { return r.toVariant().Align() }
func (r Result) FlattenCount() int { return r.toVariant().FlattenCount() }
```

---

### Task 36: Define Flags Type

**Step 1: Write failing test**

```go
func TestFlagsType(t *testing.T) {
	// flags { read, write, execute }
	// 3 flags fits in u8
	f := Flags{Names: []string{"read", "write", "execute"}}

	require.Equal(t, uint32(1), f.Size())
	require.Equal(t, uint32(1), f.Align())
}

func TestFlagsType_Large(t *testing.T) {
	// 9 flags needs u16
	names := make([]string, 9)
	for i := range names {
		names[i] = fmt.Sprintf("flag%d", i)
	}
	f := Flags{Names: names}

	require.Equal(t, uint32(2), f.Size())
	require.Equal(t, uint32(2), f.Align())
}
```

**Step 2: Implement**

```go
// Flags represents a flags (bitfield) type.
type Flags struct {
	Names []string
}

func (Flags) valType() {}

func (f Flags) Size() uint32 {
	n := len(f.Names)
	switch {
	case n <= 8:
		return 1
	case n <= 16:
		return 2
	default:
		return 4 * uint32((n+31)/32)
	}
}

func (f Flags) Align() uint32 {
	n := len(f.Names)
	switch {
	case n <= 8:
		return 1
	case n <= 16:
		return 2
	default:
		return 4
	}
}

func (f Flags) FlattenCount() int {
	return int((f.Size() + 3) / 4) // Round up to i32s
}
```

---

### Task 37: Define Enum Type

**Step 1: Write failing test**

```go
func TestEnumType(t *testing.T) {
	// enum { red, green, blue }
	e := Enum{Cases: []string{"red", "green", "blue"}}

	require.Equal(t, uint32(1), e.Size())  // 3 cases fits in u8
	require.Equal(t, uint32(1), e.Align())
	require.Equal(t, 1, e.FlattenCount())
}
```

**Step 2: Implement**

```go
// Enum represents an enumeration type.
type Enum struct {
	Cases []string
}

func (Enum) valType() {}

func (e Enum) Size() uint32 {
	n := len(e.Cases)
	switch {
	case n <= 0x100:
		return 1
	case n <= 0x10000:
		return 2
	default:
		return 4
	}
}

func (e Enum) Align() uint32 { return e.Size() }
func (Enum) FlattenCount() int { return 1 }
```

---

### Task 38: Define Tuple Type

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
```

**Step 2: Implement**

```go
// Tuple represents a tuple type.
type Tuple struct {
	Types []ValType
}

func (Tuple) valType() {}

func (t Tuple) toRecord() Record {
	fields := make([]Field, len(t.Types))
	for i, typ := range t.Types {
		fields[i] = Field{Name: fmt.Sprintf("%d", i), Type: typ}
	}
	return Record{Fields: fields}
}

func (t Tuple) Size() uint32     { return t.toRecord().Size() }
func (t Tuple) Align() uint32    { return t.toRecord().Align() }
func (t Tuple) FlattenCount() int { return t.toRecord().FlattenCount() }
```

---

### Task 39-45: Parse Composite Types in Binary

These tasks add binary parsing for composite type opcodes:

| Opcode | Type |
|--------|------|
| 0x72 | record |
| 0x71 | variant |
| 0x70 | list |
| 0x6f | tuple |
| 0x6e | flags |
| 0x6d | enum |
| 0x6c | option |
| 0x6b | result |

Each task follows the same pattern as Task 12 (primitive opcodes).

---

### Task 46-55: Implement Lift/Lower for Composite Types

These tasks implement `Lift()` and `Lower()` for each composite type:

**Pattern:**
1. Write failing test with round-trip assertion
2. Implement flat representation (if FlattenCount <= MaxFlatParams)
3. Implement heap representation (pointer to linear memory)
4. Test edge cases (empty records, deeply nested variants, etc.)

---

### Task 56-60: Implement String Encoding Support

**Files:**
- Create: `internal/component/abi/strings.go`
- Create: `internal/component/abi/strings_test.go`

Key functions:
```go
func EncodeUTF8(s string) []byte
func DecodeUTF8(data []byte) string
func EncodeUTF16(s string) []byte
func DecodeUTF16(data []byte) string
func EncodeLatin1UTF16(s string) ([]byte, bool) // bool indicates UTF-16
```

---

### Task 61-70: Integration Tests for Composite Types

Test components to build with cargo-component:

```
internal/component/testdata/
├── composites/
│   ├── echo_record.wasm      # func(record) -> record
│   ├── option_roundtrip.wasm # func(option<u32>) -> option<u32>
│   ├── result_ok_err.wasm    # func(bool) -> result<u32, string>
│   ├── list_sum.wasm         # func(list<s32>) -> s32
│   └── composites.wit
```

---

## Running Tests

```bash
# Run all Phase 2 tests
go test ./internal/component/types/... -v
go test ./internal/component/abi/... -v

# Run specific composite type tests
go test ./internal/component/types/... -v -run TestRecord
go test ./internal/component/types/... -v -run TestVariant

# Run with race detector
go test ./internal/component/... -race -v
```

---

## References

- [Canonical ABI Specification](https://github.com/WebAssembly/component-model/blob/main/design/mvp/CanonicalABI.md)
- [WIT Types Reference](https://component-model.bytecodealliance.org/design/wit.html#built-in-types)
- [Component Model Binary Format - Types](https://github.com/WebAssembly/component-model/blob/main/design/mvp/Binary.md#type-definitions)
