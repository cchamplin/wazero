# Phase 2: Type System Completeness

Implement runtime support for variant, flags, and enum types.

---

## Task 2.1: Implement Enum Lowering

**Status:** COMPLETED (commit `80eab6de`)

**Files:**
- Modify: `internal/component/canon_lower.go`
- Test: `internal/component/canon_lower_test.go`

**Implementation:**

```go
// EnumType represents an enum type for lowering
type EnumType struct {
    Cases []string
}

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

---

## Task 2.2: Implement Enum Lifting

**Status:** COMPLETED (commit `8b7e7c10`)

**Files:**
- Modify: `internal/component/instance.go`
- Test: `internal/component/instance_test.go`

**Implementation:**

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

---

## Task 2.3: Implement Flags Lowering

**Status:** COMPLETED (commit `67af3de2`)

**Files:**
- Modify: `internal/component/canon_lower.go`
- Test: `internal/component/canon_lower_test.go`

**Implementation:**

```go
// FlagsType represents a flags type for lowering
type FlagsType struct {
    Flags []string
}

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

---

## Task 2.4: Implement Flags Lifting

**Status:** COMPLETED (commit `b6bf2536`)

**Files:**
- Modify: `internal/component/instance.go`
- Test: `internal/component/instance_test.go`

**Implementation:**

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

---

## Task 2.5: Implement Variant Lowering

**Status:** COMPLETED (commit `1e3aab1a`)

**Files:**
- Modify: `internal/component/canon_lower.go`
- Test: `internal/component/canon_lower_test.go`

**New Types:**
- `VariantType` - variant type with cases
- `VariantCaseForLower` - case with name and optional payload type
- `PayloadType` interface - provides `FlattenCount()`
- `PrimitiveType` - concrete implementation

**Implementation:**

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
    // ... (handles payload and padding to max case size)

    return result, nil
}
```

---

## Task 2.6: Implement Variant Lifting

**Status:** COMPLETED (commit `0cb1bbc9`)

**Files:**
- Modify: `internal/component/instance.go`
- Test: `internal/component/instance_test.go`

**Implementation:**

```go
// liftVariant converts flat representation to a variant Val.
func liftVariant(flat []uint64, variantType *VariantType) (Val, error) {
    if len(flat) < 1 {
        return Val{}, fmt.Errorf("variant requires at least discriminant")
    }

    disc := int(flat[0])
    if disc < 0 || disc >= len(variantType.Cases) {
        return Val{}, fmt.Errorf("invalid variant discriminant %d for type with %d cases",
            disc, len(variantType.Cases))
    }

    variantCase := variantType.Cases[disc]

    if variantCase.Type == nil {
        return ValVariant(variantCase.Name, nil), nil
    }

    // Lift payload using liftVariantPayload helper
    // ... (handles all primitive payload types)

    return ValVariant(variantCase.Name, &payload), nil
}
```

**Helper:** `liftVariantPayload(flatVal uint64, payloadType PayloadType) (Val, error)` - supports all primitive types with proper bit conversion for floats.
