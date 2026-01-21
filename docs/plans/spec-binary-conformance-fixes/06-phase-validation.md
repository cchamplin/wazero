# Phase 6: Validation Layer

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add validation rules to catch malformed components during parsing.

**Architecture:** Create a validation pass that runs after decoding to check constraints from the spec. Errors are returned from the parser rather than silently accepting invalid components.

**Tech Stack:** Go

**Gap Analysis Reference:** Section 11 - Validation Gaps

---

## Context

The current parser performs minimal validation. The spec defines several constraints that should be enforced:

1. Type element counts (record/variant/tuple/flags/enum must have ≥1 element)
2. Borrow types not allowed in results
3. Outer alias sort restrictions
4. Unique names in type definitions
5. Resource destructor type constraints
6. Fixed-size list length must be >0

---

## Reference Files

- **Spec:** `debug-vendored/component-model/design/mvp/Binary.md` (validation rules throughout)
- **wasmparser validation:** `debug-vendored/wasm-tools/crates/wasmparser/src/validator/component/`
- **Current impl:** `internal/component/binary/decoder.go`

---

## Task 6.1: Add Type Element Count Validation

**Files:**
- Modify: `internal/component/binary/types.go`

**Step 1: Add validation in record type decoder**

In `decodeRecordTypeDef`:

```go
func decodeRecordTypeDef(r *bytes.Reader) (*RecordTypeDef, error) {
    count, _, err := leb128.DecodeUint32(r)
    if err != nil {
        return nil, fmt.Errorf("read field count: %w", err)
    }

    // VALIDATION: Record must have at least 1 field
    if count == 0 {
        return nil, fmt.Errorf("record type must have at least 1 field")
    }

    // ... rest of decoding
}
```

**Step 2: Add validation in variant type decoder**

```go
func decodeVariantTypeDef(r *bytes.Reader) (*VariantTypeDef, error) {
    count, _, err := leb128.DecodeUint32(r)
    if err != nil {
        return nil, fmt.Errorf("read case count: %w", err)
    }

    // VALIDATION: Variant must have at least 1 case
    if count == 0 {
        return nil, fmt.Errorf("variant type must have at least 1 case")
    }

    // ... rest of decoding
}
```

**Step 3: Add validation in tuple type decoder**

```go
func decodeTupleTypeDef(r *bytes.Reader) (*TupleTypeDef, error) {
    count, _, err := leb128.DecodeUint32(r)
    if err != nil {
        return nil, fmt.Errorf("read element count: %w", err)
    }

    // VALIDATION: Tuple must have at least 1 element
    if count == 0 {
        return nil, fmt.Errorf("tuple type must have at least 1 element")
    }

    // ... rest of decoding
}
```

**Step 4: Add validation in flags type decoder**

```go
func decodeFlagsTypeDef(r *bytes.Reader) (*FlagsTypeDef, error) {
    count, _, err := leb128.DecodeUint32(r)
    if err != nil {
        return nil, fmt.Errorf("read flag count: %w", err)
    }

    // VALIDATION: Flags must have at least 1 flag
    if count == 0 {
        return nil, fmt.Errorf("flags type must have at least 1 flag")
    }

    // ... rest of decoding
}
```

**Step 5: Add validation in enum type decoder**

```go
func decodeEnumTypeDef(r *bytes.Reader) (*EnumTypeDef, error) {
    count, _, err := leb128.DecodeUint32(r)
    if err != nil {
        return nil, fmt.Errorf("read case count: %w", err)
    }

    // VALIDATION: Enum must have at least 1 case
    if count == 0 {
        return nil, fmt.Errorf("enum type must have at least 1 case")
    }

    // ... rest of decoding
}
```

**Step 6: Add validation in list type decoder**

```go
func decodeListTypeDef(r *bytes.Reader) (*ListTypeDef, error) {
    // List doesn't require count validation - it's the element type only
    // But fixed-size list (0x67) must have length > 0
}

func decodeFixedSizeListTypeDef(r *bytes.Reader) (*FixedSizeListTypeDef, error) {
    elemType, err := decodeValType(r)
    if err != nil {
        return nil, fmt.Errorf("decode element type: %w", err)
    }

    length, _, err := leb128.DecodeUint32(r)
    if err != nil {
        return nil, fmt.Errorf("read length: %w", err)
    }

    // VALIDATION: Fixed-size list must have length > 0
    if length == 0 {
        return nil, fmt.Errorf("fixed-size list must have length > 0")
    }

    return &FixedSizeListTypeDef{
        ElementType: elemType,
        Size:        length,
    }, nil
}
```

**Step 7: Run build to verify**

```bash
CGO_ENABLED=0 go build ./internal/component/...
```

---

## Task 6.2: Add Borrow-in-Results Validation

**Files:**
- Modify: `internal/component/binary/types.go`

**Step 1: Create helper to check for borrow types**

```go
// containsBorrow returns true if the value type contains a borrow handle.
func containsBorrow(valType component.ValTypeRef) bool {
    return valType.IsBorrow
}

// containsBorrowInResults checks if any result type contains a borrow.
func containsBorrowInResults(results []component.NamedValType) error {
    for i, result := range results {
        if containsBorrow(result.ValType) {
            return fmt.Errorf("result %d (%s) contains borrow type, which is not allowed", i, result.Name)
        }
    }
    return nil
}
```

**Step 2: Add validation in function type decoder**

In `decodeFuncType`, after parsing results:

```go
func decodeFuncType(r *bytes.Reader) (*component.FuncType, error) {
    // ... decode params and results ...

    // VALIDATION: Results cannot contain borrow types
    if err := containsBorrowInResults(funcType.Results); err != nil {
        return nil, fmt.Errorf("invalid function type: %w", err)
    }

    return funcType, nil
}
```

**Step 3: Run build to verify**

```bash
CGO_ENABLED=0 go build ./internal/component/...
```

---

## Task 6.3: Add Outer Alias Sort Validation

**Files:**
- Modify: `internal/component/binary/alias.go`

**Step 1: Add validation in outer alias decoding**

In `decodeAlias`, for outer alias case:

```go
case component.AliasKindOuter:
    alias.OuterCount, _, err = leb128.DecodeUint32(r)
    if err != nil {
        return alias, fmt.Errorf("read outer count: %w", err)
    }
    alias.OuterIndex, _, err = leb128.DecodeUint32(r)
    if err != nil {
        return alias, fmt.Errorf("read outer index: %w", err)
    }

    // VALIDATION: Outer alias sort must be type, module, or component
    switch alias.Sort {
    case component.SortType:
        // Valid: can outer-alias types
    case component.SortComponent:
        // Valid: can outer-alias components
    case component.SortCoreSort:
        if alias.CoreSort == component.CoreSortModule {
            // Valid: can outer-alias modules
        } else if alias.CoreSort == component.CoreSortType {
            // Valid: can outer-alias core types
        } else {
            return alias, fmt.Errorf("outer alias with core sort %s not allowed (only module and type permitted)", alias.CoreSort)
        }
    default:
        return alias, fmt.Errorf("outer alias with sort %s not allowed (only type, component, and core module/type permitted)", alias.Sort)
    }
```

**Step 2: Run build to verify**

```bash
CGO_ENABLED=0 go build ./internal/component/...
```

---

## Task 6.4: Add Unique Name Validation

**Files:**
- Modify: `internal/component/binary/types.go`

**Step 1: Create helper function for unique name checking**

```go
// checkUniqueNames returns an error if any names are duplicated.
func checkUniqueNames(names []string, context string) error {
    seen := make(map[string]bool)
    for i, name := range names {
        if seen[name] {
            return fmt.Errorf("duplicate %s name at index %d: %q", context, i, name)
        }
        seen[name] = true
    }
    return nil
}
```

**Step 2: Add validation in record type decoder**

```go
func decodeRecordTypeDef(r *bytes.Reader) (*RecordTypeDef, error) {
    // ... decode fields ...

    // VALIDATION: Field names must be unique
    names := make([]string, len(record.Fields))
    for i, field := range record.Fields {
        names[i] = field.Name
    }
    if err := checkUniqueNames(names, "record field"); err != nil {
        return nil, err
    }

    return record, nil
}
```

**Step 3: Add validation in variant type decoder**

```go
func decodeVariantTypeDef(r *bytes.Reader) (*VariantTypeDef, error) {
    // ... decode cases ...

    // VALIDATION: Case names must be unique
    names := make([]string, len(variant.Cases))
    for i, c := range variant.Cases {
        names[i] = c.Name
    }
    if err := checkUniqueNames(names, "variant case"); err != nil {
        return nil, err
    }

    return variant, nil
}
```

**Step 4: Add validation in flags type decoder**

```go
func decodeFlagsTypeDef(r *bytes.Reader) (*FlagsTypeDef, error) {
    // ... decode flags ...

    // VALIDATION: Flag names must be unique
    if err := checkUniqueNames(flags.Names, "flag"); err != nil {
        return nil, err
    }

    return flags, nil
}
```

**Step 5: Add validation in enum type decoder**

```go
func decodeEnumTypeDef(r *bytes.Reader) (*EnumTypeDef, error) {
    // ... decode cases ...

    // VALIDATION: Enum case names must be unique
    if err := checkUniqueNames(enum.Names, "enum case"); err != nil {
        return nil, err
    }

    return enum, nil
}
```

**Step 6: Run build to verify**

```bash
CGO_ENABLED=0 go build ./internal/component/...
```

---

## Task 6.5: Add Validation Tests

**Files:**
- Create: `internal/component/binary/validation_test.go`

**Step 1: Create test file**

```go
package binary

import (
    "bytes"
    "strings"
    "testing"
)

func TestTypeElementCountValidation(t *testing.T) {
    t.Run("empty record rejected", func(t *testing.T) {
        // Empty record: count = 0
        input := []byte{0x00}
        r := bytes.NewReader(input)

        _, err := decodeRecordTypeDef(r)
        if err == nil {
            t.Fatal("expected error for empty record")
        }
        if !strings.Contains(err.Error(), "at least 1 field") {
            t.Errorf("unexpected error: %v", err)
        }
    })

    t.Run("empty variant rejected", func(t *testing.T) {
        input := []byte{0x00}
        r := bytes.NewReader(input)

        _, err := decodeVariantTypeDef(r)
        if err == nil {
            t.Fatal("expected error for empty variant")
        }
        if !strings.Contains(err.Error(), "at least 1 case") {
            t.Errorf("unexpected error: %v", err)
        }
    })

    t.Run("empty tuple rejected", func(t *testing.T) {
        input := []byte{0x00}
        r := bytes.NewReader(input)

        _, err := decodeTupleTypeDef(r)
        if err == nil {
            t.Fatal("expected error for empty tuple")
        }
        if !strings.Contains(err.Error(), "at least 1 element") {
            t.Errorf("unexpected error: %v", err)
        }
    })

    t.Run("empty flags rejected", func(t *testing.T) {
        input := []byte{0x00}
        r := bytes.NewReader(input)

        _, err := decodeFlagsTypeDef(r)
        if err == nil {
            t.Fatal("expected error for empty flags")
        }
        if !strings.Contains(err.Error(), "at least 1 flag") {
            t.Errorf("unexpected error: %v", err)
        }
    })

    t.Run("empty enum rejected", func(t *testing.T) {
        input := []byte{0x00}
        r := bytes.NewReader(input)

        _, err := decodeEnumTypeDef(r)
        if err == nil {
            t.Fatal("expected error for empty enum")
        }
        if !strings.Contains(err.Error(), "at least 1 case") {
            t.Errorf("unexpected error: %v", err)
        }
    })

    t.Run("fixed-size list with zero length rejected", func(t *testing.T) {
        // Element type (u8 = 0x7d), length = 0
        input := []byte{0x7d, 0x00}
        r := bytes.NewReader(input)

        _, err := decodeFixedSizeListTypeDef(r)
        if err == nil {
            t.Fatal("expected error for zero-length fixed-size list")
        }
        if !strings.Contains(err.Error(), "length > 0") {
            t.Errorf("unexpected error: %v", err)
        }
    })
}

func TestUniqueNameValidation(t *testing.T) {
    t.Run("duplicate record field names rejected", func(t *testing.T) {
        // Two fields both named "foo"
        // count=2, name1="foo", type1=u8, name2="foo", type2=u8
        input := []byte{
            0x02,                   // count = 2
            0x03, 'f', 'o', 'o',    // name = "foo"
            0x7d,                   // type = u8
            0x03, 'f', 'o', 'o',    // name = "foo" (duplicate)
            0x7d,                   // type = u8
        }
        r := bytes.NewReader(input)

        _, err := decodeRecordTypeDef(r)
        if err == nil {
            t.Fatal("expected error for duplicate field names")
        }
        if !strings.Contains(err.Error(), "duplicate") {
            t.Errorf("unexpected error: %v", err)
        }
    })
}

func TestOuterAliasSortValidation(t *testing.T) {
    // Test that invalid outer alias sorts are rejected
    // This would require creating a full alias encoding
}
```

**Step 2: Run tests**

```bash
CGO_ENABLED=0 go test ./internal/component/binary/... -run "TestTypeElementCountValidation|TestUniqueNameValidation" -v
```
Expected: All tests pass

---

## Task 6.6: Run Regression Tests and Commit

**Step 1: Run calculator regression tests**

```bash
CGO_ENABLED=0 go test ./internal/component/wasip2test/... -run "TestCalculatorPlugins/(add|subtract)" -v
```
Expected: Both add and subtract pass

**Step 2: Run all component binary tests**

```bash
CGO_ENABLED=0 go test ./internal/component/binary/... -v
```
Expected: All tests pass

**Step 3: Commit changes**

```bash
git add internal/component/binary/types.go internal/component/binary/alias.go internal/component/binary/validation_test.go
git commit -m "feat(component): add validation for type constraints and names

Add validation rules per spec:
- Record/variant/tuple/flags/enum must have ≥1 element
- Fixed-size list must have length > 0
- Results cannot contain borrow types
- Outer alias sort restricted to type/module/component
- Unique names required in records/variants/flags/enums

Ref: docs/plans/component-model-binary-parser-gap-analysis.md Section 11

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Verification Checklist

- [ ] Empty record/variant/tuple/flags/enum rejected
- [ ] Fixed-size list with length 0 rejected
- [ ] Function results with borrow types rejected
- [ ] Invalid outer alias sorts rejected
- [ ] Duplicate names in records/variants/flags/enums rejected
- [ ] Tests cover all validation rules
- [ ] Calculator add/subtract tests pass
- [ ] All component binary tests pass
- [ ] Changes committed

---

## Future Validation (Out of Scope)

These validations require deeper analysis and are deferred:

1. Resource destructor type must be [i32] -> []
2. Stream/future element types cannot contain borrow
3. Validation of instantiate args match imports
4. Type index bounds checking
5. Cross-reference validation between sections
