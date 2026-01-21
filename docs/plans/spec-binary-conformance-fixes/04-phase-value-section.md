# Phase 4: Value Section Completion

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Complete the value section (section 12) decoder to handle all value types per spec.

**Architecture:** Extend `decodeValueSection` to handle float, char, string, and composite value encodings per the `val(T)` grammar in Binary.md.

**Tech Stack:** Go

**Gap Analysis Reference:** Section 8 - Value Definitions

---

## Context

The value section (🪙 gated) contains constant values that can be passed as arguments to the start function. Current implementation only handles a few integer primitives.

Missing:
- Float values (f32, f64)
- Char values
- String values
- Composite values (record, variant, list, tuple, flags, enum, option, result)

---

## Reference Files

- **Spec:** `debug-vendored/component-model/design/mvp/Binary.md` (lines 432-465, val(T) grammar)
- **Current impl:** `internal/component/binary/value.go`

---

## Task 4.1: Add Float Value Decoding (f32, f64)

**Files:**
- Modify: `internal/component/binary/value.go`

**Step 1: Read current decodeValueSection**

Understand the current structure and how primitive types are handled.

**Step 2: Add f32 decoding**

F32 values are encoded as 4 little-endian bytes:

```go
case byte(PrimValTypeF32): // 0x76
    data := make([]byte, 4)
    if _, err := io.ReadFull(r, data); err != nil {
        return fmt.Errorf("read f32 value %d: %w", i, err)
    }
    c.Values[i] = component.ValueDef{
        Type: valType,
        Data: data,
    }
```

**Step 3: Add f64 decoding**

F64 values are encoded as 8 little-endian bytes:

```go
case byte(PrimValTypeF64): // 0x75
    data := make([]byte, 8)
    if _, err := io.ReadFull(r, data); err != nil {
        return fmt.Errorf("read f64 value %d: %w", i, err)
    }
    c.Values[i] = component.ValueDef{
        Type: valType,
        Data: data,
    }
```

**Step 4: Run build to verify**

```bash
CGO_ENABLED=0 go build ./internal/component/...
```
Expected: Build succeeds

---

## Task 4.2: Add Char Value Decoding

**Files:**
- Modify: `internal/component/binary/value.go`

**Step 1: Add char decoding**

Char values are encoded as a single Unicode scalar value (LEB128 u32):

```go
case byte(PrimValTypeChar): // 0x74
    charVal, n, err := leb128.DecodeUint32(r)
    if err != nil {
        return fmt.Errorf("read char value %d: %w", i, err)
    }
    // Store as 4 bytes (u32)
    data := make([]byte, 4)
    data[0] = byte(charVal)
    data[1] = byte(charVal >> 8)
    data[2] = byte(charVal >> 16)
    data[3] = byte(charVal >> 24)
    _ = n // bytes read
    c.Values[i] = component.ValueDef{
        Type: valType,
        Data: data,
    }
```

**Step 2: Run build to verify**

```bash
CGO_ENABLED=0 go build ./internal/component/...
```
Expected: Build succeeds

---

## Task 4.3: Add String Value Decoding

**Files:**
- Modify: `internal/component/binary/value.go`

**Step 1: Add string decoding**

String values are encoded as length-prefixed UTF-8 bytes:

```go
case byte(PrimValTypeString): // 0x73
    strLen, _, err := leb128.DecodeUint32(r)
    if err != nil {
        return fmt.Errorf("read string length for value %d: %w", i, err)
    }
    data := make([]byte, strLen)
    if strLen > 0 {
        if _, err := io.ReadFull(r, data); err != nil {
            return fmt.Errorf("read string value %d: %w", i, err)
        }
    }
    c.Values[i] = component.ValueDef{
        Type: valType,
        Data: data,
    }
```

**Step 2: Run build to verify**

```bash
CGO_ENABLED=0 go build ./internal/component/...
```
Expected: Build succeeds

---

## Task 4.4: Add Composite Value Decoding

**Files:**
- Modify: `internal/component/binary/value.go`
- Modify: `internal/component/component.go`

**Step 1: Update ValueDef to support composite values**

In `internal/component/component.go`:

```go
// ValueDef represents a component value.
type ValueDef struct {
    Type     ValTypeRef
    Data     []byte         // Raw bytes for primitives
    Children []ValueDef     // NEW: For composite types (record, tuple, list, etc.)
    Index    *uint32        // NEW: For enum/flags (variant case index)
}
```

**Step 2: Create helper function for recursive value decoding**

In `internal/component/binary/value.go`:

```go
// decodeValue decodes a single value based on its type.
// This handles the val(T) grammar from the spec.
func decodeValue(r *bytes.Reader, valType component.ValTypeRef, c *component.Component) (component.ValueDef, error) {
    val := component.ValueDef{Type: valType}

    if valType.IsPrimitive {
        return decodePrimitiveValue(r, valType)
    }

    // For type references, look up the actual type
    if valType.TypeIdx >= uint32(len(c.Types)) {
        return val, fmt.Errorf("type index %d out of bounds", valType.TypeIdx)
    }

    typeDef := c.Types[valType.TypeIdx]

    switch typeDef.Kind {
    case component.TypeDefKindDefined:
        if typeDef.Record != nil {
            return decodeRecordValue(r, typeDef.Record, c)
        }
        if typeDef.Variant != nil {
            return decodeVariantValue(r, typeDef.Variant, c)
        }
        if typeDef.List != nil {
            return decodeListValue(r, typeDef.List, c)
        }
        if typeDef.Tuple != nil {
            return decodeTupleValue(r, typeDef.Tuple, c)
        }
        if typeDef.Flags != nil {
            return decodeFlagsValue(r, typeDef.Flags)
        }
        if typeDef.Enum != nil {
            return decodeEnumValue(r, typeDef.Enum)
        }
        if typeDef.Option != nil {
            return decodeOptionValue(r, typeDef.Option, c)
        }
        if typeDef.Result != nil {
            return decodeResultValue(r, typeDef.Result, c)
        }
    }

    return val, fmt.Errorf("unsupported value type kind: %d", typeDef.Kind)
}
```

**Step 3: Add primitive value decoder**

```go
func decodePrimitiveValue(r *bytes.Reader, valType component.ValTypeRef) (component.ValueDef, error) {
    val := component.ValueDef{Type: valType}

    switch PrimValType(valType.Primitive) {
    case PrimValTypeBool:
        b, err := r.ReadByte()
        if err != nil {
            return val, err
        }
        val.Data = []byte{b}

    case PrimValTypeS8, PrimValTypeU8:
        b, err := r.ReadByte()
        if err != nil {
            return val, err
        }
        val.Data = []byte{b}

    case PrimValTypeS16, PrimValTypeU16:
        data := make([]byte, 2)
        if _, err := io.ReadFull(r, data); err != nil {
            return val, err
        }
        val.Data = data

    case PrimValTypeS32, PrimValTypeU32:
        v, _, err := leb128.DecodeUint32(r)
        if err != nil {
            return val, err
        }
        val.Data = make([]byte, 4)
        binary.LittleEndian.PutUint32(val.Data, v)

    case PrimValTypeS64, PrimValTypeU64:
        v, _, err := leb128.DecodeUint64(r)
        if err != nil {
            return val, err
        }
        val.Data = make([]byte, 8)
        binary.LittleEndian.PutUint64(val.Data, v)

    case PrimValTypeF32:
        val.Data = make([]byte, 4)
        if _, err := io.ReadFull(r, val.Data); err != nil {
            return val, err
        }

    case PrimValTypeF64:
        val.Data = make([]byte, 8)
        if _, err := io.ReadFull(r, val.Data); err != nil {
            return val, err
        }

    case PrimValTypeChar:
        v, _, err := leb128.DecodeUint32(r)
        if err != nil {
            return val, err
        }
        val.Data = make([]byte, 4)
        binary.LittleEndian.PutUint32(val.Data, v)

    case PrimValTypeString:
        strLen, _, err := leb128.DecodeUint32(r)
        if err != nil {
            return val, err
        }
        val.Data = make([]byte, strLen)
        if strLen > 0 {
            if _, err := io.ReadFull(r, val.Data); err != nil {
                return val, err
            }
        }

    default:
        return val, fmt.Errorf("unknown primitive type: 0x%02x", valType.Primitive)
    }

    return val, nil
}
```

**Step 4: Add composite value decoders**

```go
func decodeRecordValue(r *bytes.Reader, record *component.RecordTypeDef, c *component.Component) (component.ValueDef, error) {
    val := component.ValueDef{}
    val.Children = make([]component.ValueDef, len(record.Fields))
    for i, field := range record.Fields {
        child, err := decodeValue(r, field.ValType, c)
        if err != nil {
            return val, fmt.Errorf("decode record field %d (%s): %w", i, field.Name, err)
        }
        val.Children[i] = child
    }
    return val, nil
}

func decodeVariantValue(r *bytes.Reader, variant *component.VariantTypeDef, c *component.Component) (component.ValueDef, error) {
    val := component.ValueDef{}

    // Read case index
    caseIdx, _, err := leb128.DecodeUint32(r)
    if err != nil {
        return val, fmt.Errorf("decode variant case index: %w", err)
    }
    val.Index = &caseIdx

    if int(caseIdx) >= len(variant.Cases) {
        return val, fmt.Errorf("variant case index %d out of bounds", caseIdx)
    }

    // Decode payload if present
    caseType := variant.Cases[caseIdx]
    if caseType.ValType != nil {
        child, err := decodeValue(r, *caseType.ValType, c)
        if err != nil {
            return val, fmt.Errorf("decode variant payload: %w", err)
        }
        val.Children = []component.ValueDef{child}
    }

    return val, nil
}

func decodeListValue(r *bytes.Reader, list *component.ListTypeDef, c *component.Component) (component.ValueDef, error) {
    val := component.ValueDef{}

    length, _, err := leb128.DecodeUint32(r)
    if err != nil {
        return val, fmt.Errorf("decode list length: %w", err)
    }

    val.Children = make([]component.ValueDef, length)
    for i := uint32(0); i < length; i++ {
        child, err := decodeValue(r, list.ElementType, c)
        if err != nil {
            return val, fmt.Errorf("decode list element %d: %w", i, err)
        }
        val.Children[i] = child
    }

    return val, nil
}

func decodeTupleValue(r *bytes.Reader, tuple *component.TupleTypeDef, c *component.Component) (component.ValueDef, error) {
    val := component.ValueDef{}
    val.Children = make([]component.ValueDef, len(tuple.Types))
    for i, elemType := range tuple.Types {
        child, err := decodeValue(r, elemType, c)
        if err != nil {
            return val, fmt.Errorf("decode tuple element %d: %w", i, err)
        }
        val.Children[i] = child
    }
    return val, nil
}

func decodeFlagsValue(r *bytes.Reader, flags *component.FlagsTypeDef) (component.ValueDef, error) {
    val := component.ValueDef{}

    // Flags are encoded as ceil(len(flags.Names) / 32) u32 values
    numU32s := (len(flags.Names) + 31) / 32
    val.Data = make([]byte, numU32s*4)
    if _, err := io.ReadFull(r, val.Data); err != nil {
        return val, fmt.Errorf("decode flags: %w", err)
    }

    return val, nil
}

func decodeEnumValue(r *bytes.Reader, enum *component.EnumTypeDef) (component.ValueDef, error) {
    val := component.ValueDef{}

    caseIdx, _, err := leb128.DecodeUint32(r)
    if err != nil {
        return val, fmt.Errorf("decode enum case: %w", err)
    }
    val.Index = &caseIdx

    return val, nil
}

func decodeOptionValue(r *bytes.Reader, option *component.OptionTypeDef, c *component.Component) (component.ValueDef, error) {
    val := component.ValueDef{}

    discriminant, err := r.ReadByte()
    if err != nil {
        return val, fmt.Errorf("decode option discriminant: %w", err)
    }

    idx := uint32(discriminant)
    val.Index = &idx

    if discriminant == 0x01 {
        // Some case - decode inner value
        child, err := decodeValue(r, option.InnerType, c)
        if err != nil {
            return val, fmt.Errorf("decode option inner: %w", err)
        }
        val.Children = []component.ValueDef{child}
    }

    return val, nil
}

func decodeResultValue(r *bytes.Reader, result *component.ResultTypeDef, c *component.Component) (component.ValueDef, error) {
    val := component.ValueDef{}

    discriminant, err := r.ReadByte()
    if err != nil {
        return val, fmt.Errorf("decode result discriminant: %w", err)
    }

    idx := uint32(discriminant)
    val.Index = &idx

    if discriminant == 0x00 && result.OkType != nil {
        // Ok case with payload
        child, err := decodeValue(r, *result.OkType, c)
        if err != nil {
            return val, fmt.Errorf("decode result ok: %w", err)
        }
        val.Children = []component.ValueDef{child}
    } else if discriminant == 0x01 && result.ErrType != nil {
        // Err case with payload
        child, err := decodeValue(r, *result.ErrType, c)
        if err != nil {
            return val, fmt.Errorf("decode result err: %w", err)
        }
        val.Children = []component.ValueDef{child}
    }

    return val, nil
}
```

**Step 5: Update decodeValueSection to use new decoder**

```go
func decodeValueSection(c *component.Component, r *bytes.Reader) error {
    count, _, err := leb128.DecodeUint32(r)
    if err != nil {
        return fmt.Errorf("read value count: %w", err)
    }

    c.Values = make([]component.ValueDef, count)
    for i := uint32(0); i < count; i++ {
        valType, err := decodeValType(r)
        if err != nil {
            return fmt.Errorf("read value %d type: %w", i, err)
        }

        val, err := decodeValue(r, valType, c)
        if err != nil {
            return fmt.Errorf("decode value %d: %w", i, err)
        }
        val.Type = valType
        c.Values[i] = val
    }

    return nil
}
```

**Step 6: Add import for encoding/binary**

```go
import (
    "bytes"
    "encoding/binary"
    "fmt"
    "io"

    "github.com/tetratelabs/wazero/internal/component"
    "github.com/tetratelabs/wazero/internal/leb128"
)
```

**Step 7: Run build to verify**

```bash
CGO_ENABLED=0 go build ./internal/component/...
```
Expected: Build succeeds

---

## Task 4.5: Add Value Section Tests

**Files:**
- Create: `internal/component/binary/value_test.go`

**Step 1: Create test file**

```go
package binary

import (
    "bytes"
    "testing"

    "github.com/tetratelabs/wazero/internal/component"
)

func TestDecodePrimitiveValue(t *testing.T) {
    tests := []struct {
        name     string
        input    []byte
        valType  component.ValTypeRef
        expected []byte
    }{
        {
            name:    "bool true",
            input:   []byte{0x01},
            valType: component.ValTypeRef{IsPrimitive: true, Primitive: 0x7f},
            expected: []byte{0x01},
        },
        {
            name:    "u8",
            input:   []byte{0x42},
            valType: component.ValTypeRef{IsPrimitive: true, Primitive: 0x7d},
            expected: []byte{0x42},
        },
        {
            name:    "string hello",
            input:   append([]byte{0x05}, []byte("hello")...),
            valType: component.ValTypeRef{IsPrimitive: true, Primitive: 0x73},
            expected: []byte("hello"),
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            r := bytes.NewReader(tt.input)
            got, err := decodePrimitiveValue(r, tt.valType)
            if err != nil {
                t.Fatalf("unexpected error: %v", err)
            }
            if !bytes.Equal(got.Data, tt.expected) {
                t.Errorf("Data: got %v, want %v", got.Data, tt.expected)
            }
        })
    }
}
```

**Step 2: Run tests**

```bash
CGO_ENABLED=0 go test ./internal/component/binary/... -run TestDecodePrimitiveValue -v
```
Expected: All tests pass

---

## Task 4.6: Run Regression Tests and Commit

**Step 1: Run calculator regression tests**

```bash
CGO_ENABLED=0 go test ./internal/component/wasip2test/... -run "TestCalculatorPlugins/(add|subtract)" -v
```
Expected: Both add and subtract pass

**Step 2: Commit changes**

```bash
git add internal/component/component.go internal/component/binary/value.go internal/component/binary/value_test.go
git commit -m "feat(component): complete value section decoding per spec

Implement full val(T) grammar for value section (section 12):
- Float values (f32, f64)
- Char values (unicode scalar)
- String values (length-prefixed UTF-8)
- Composite values: record, variant, list, tuple, flags, enum, option, result

Add Children and Index fields to ValueDef for composite types.

Ref: docs/plans/component-model-binary-parser-gap-analysis.md Section 8
Ref: debug-vendored/component-model/design/mvp/Binary.md lines 432-465

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Verification Checklist

- [ ] Float values (f32, f64) decoded correctly
- [ ] Char values decoded as u32
- [ ] String values decoded with length prefix
- [ ] Record values decode all fields
- [ ] Variant values decode case index and optional payload
- [ ] List values decode length and elements
- [ ] Tuple values decode all elements
- [ ] Flags values decode as packed u32s
- [ ] Enum values decode case index
- [ ] Option values decode discriminant and optional inner
- [ ] Result values decode discriminant and optional ok/err
- [ ] Tests cover primitive and composite values
- [ ] Calculator add/subtract tests pass
- [ ] Changes committed
