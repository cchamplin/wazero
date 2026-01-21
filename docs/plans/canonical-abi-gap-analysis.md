# Canonical ABI Gap Analysis: Type Lifting and Lowering

**Date:** 2026-01-20
**Scope:** internal/component/abi/ vs Component Model Specification
**Spec Reference:** debug-vendored/component-model/design/mvp/CanonicalABI.md (Sections: "Supporting definitions" through "Flat Lowering")

## Executive Summary

This document provides a comprehensive defect and gap analysis comparing wazero's Canonical ABI implementation against the official Component Model specification. The analysis covers type despecialization, alignment/size calculations, loading/storing operations, and flat lifting/lowering.

**Overall Assessment:** The implementation is largely functional for core use cases but has several specification deviations ranging from missing functionality to edge case handling issues.

---

## Critical Gaps (Must Fix)

### 1. Float NaN Canonicalization - MISSING

**Spec Reference:** CanonicalABI.md lines 2037-2073

**Specification Requirement:**
```python
CANONICAL_FLOAT32_NAN = 0x7fc00000
CANONICAL_FLOAT64_NAN = 0x7ff8000000000000

def canonicalize_nan32(f):
  if math.isnan(f):
    f = core_f32_reinterpret_i32(CANONICAL_FLOAT32_NAN)
  return f
```

**Current Implementation:** `lift.go:349-358` and `lower.go:316-321`
- Floats are read/written directly without NaN canonicalization
- No check for `math.IsNaN(f)` or bit pattern normalization

**Impact:** Non-conformant behavior when NaN values are lifted; different NaN bit patterns may produce different results across implementations.

**Fix Required:**
```go
func canonicalizeNaN32(f float32) float32 {
    if math.IsNaN(float64(f)) {
        return math.Float32frombits(0x7fc00000)
    }
    return f
}
```

---

### 2. Fixed-Length Lists - NOT IMPLEMENTED

**Spec Reference:** CanonicalABI.md lines 1860-1875, 1946-1957, 2799-2806

**Specification Requirement:**
```python
@dataclass
class ListType(ValType):
  t: ValType
  l: Optional[int] = None  # Fixed length

def alignment_list(elem_type, maybe_length):
  if maybe_length is not None:
    return alignment(elem_type)
  return 4

def flatten_list(elem_type, maybe_length):
  if maybe_length is not None:
    return flatten_type(elem_type) * maybe_length
  return ['i32', 'i32']
```

**Current Implementation:** `types/composite.go:159-179`
- `List` struct only has `Element ValType`, no length field
- All lists treated as dynamic (ptr + len)

**Impact:** Fixed-length lists cannot be properly flattened into registers, causing potential ABI incompatibility.

**Fix Required:** Add optional `Length *uint32` field to `List` type and update Size(), Align(), and FlattenCount() accordingly.

---

### 3. String Alignment Validation - MISSING

**Spec Reference:** CanonicalABI.md lines 2124-2125

**Specification Requirement:**
```python
trap_if(ptr != align_to(ptr, alignment))
# where alignment = 1 for UTF-8, 2 for UTF-16
```

**Current Implementation:** `strings.go` does not validate alignment of string pointers.

**Impact:** Misaligned string access could cause undefined behavior on strict architectures.

**Fix Required:** Add alignment check in `liftStringUTF16` and `liftStringLatin1UTF16`:
```go
if ptr%2 != 0 {
    return "", fmt.Errorf("UTF-16 string pointer not 2-byte aligned: %d", ptr)
}
```

---

### 4. List Element Pointer Alignment Validation - MISSING

**Spec Reference:** CanonicalABI.md lines 2153-2154

**Specification Requirement:**
```python
trap_if(ptr != align_to(ptr, alignment(elem_type)))
```

**Current Implementation:** `lift.go:269-272` validates bounds but not alignment.

**Impact:** Misaligned list element access could cause undefined behavior.

**Fix Required:** Add alignment check before list iteration.

---

### 5. Variant Flat Lifting/Lowering Type Coercion - INCORRECT

**Spec Reference:** CanonicalABI.md lines 2825-2841, 2962-2989, 3077-3098

**Specification Requirement:**
The spec uses a `join` function to determine the widest compatible type:
```python
def join(a, b):
  if a == b: return a
  if (a == 'i32' and b == 'f32') or (a == 'f32' and b == 'i32'): return 'i32'
  return 'i64'
```

And requires coercion during lift/lower:
```python
# Lifting: decode types as needed
case ('i32', 'f32') : return decode_i32_as_float(x)
case ('i64', 'i32') : return wrap_i64_to_i32(x)
case ('i64', 'f32') : return decode_i32_as_float(wrap_i64_to_i32(x))
case ('i64', 'f64') : return decode_i64_as_float(x)

# Lowering: encode types as needed
case ('f32', 'i32') : payload[i] = encode_float_as_i32(fv)
case ('i32', 'i64') : payload[i] = fv
case ('f32', 'i64') : payload[i] = encode_float_as_i32(fv)
case ('f64', 'i64') : payload[i] = encode_float_as_i64(fv)
```

**Current Implementation:** `flatten.go:116-151` uses `isWiderType` for type selection but:
- `lift.go:129-132` skips padding as i64 without proper coercion
- `lower.go:111-114` pads with zeros without type coercion

**Impact:** Incorrect values when variant payloads have mixed types (e.g., one case with f32, another with i32).

---

### 6. Resource Type Validation - MISSING

**Spec Reference:** CanonicalABI.md lines 2216-2221, 2234-2241

**Specification Requirement:**
```python
def lift_own(cx, i, t):
  h = cx.inst.table.remove(i)
  trap_if(not isinstance(h, ResourceHandle))
  trap_if(h.rt is not t.rt)  # Validate resource type matches!
  trap_if(h.num_lends != 0)
  trap_if(not h.own)
  return h.rep
```

**Current Implementation:** `lift.go:624-698`
- Does not validate that handle's resource type matches expected type
- Missing `h.rt is not t.rt` check

**Impact:** Type confusion vulnerabilities possible if wrong handle index passed.

---

### 7. Borrow Optimization for Same Instance - MISSING

**Spec Reference:** CanonicalABI.md lines 2677-2689

**Specification Requirement:**
```python
def lower_borrow(cx, rep, t):
  assert(isinstance(cx.borrow_scope, Task))
  if cx.inst is t.rt.impl:  # Optimization!
    return rep
  h = ResourceHandle(t.rt, rep, own = False, borrow_scope = cx.borrow_scope)
  h.borrow_scope.num_borrows += 1
  return cx.inst.table.add(h)
```

**Current Implementation:** `lower.go:623-636` always creates a new handle.

**Impact:** Unnecessary overhead when borrowing resources from the same component.

---

## Major Gaps (Should Fix)

### 8. ErrorContext Type - NOT IMPLEMENTED

**Spec Reference:** CanonicalABI.md lines 1859, 1945, 2011, 2295

**Specification:**
- `alignment(ErrorContextType()) = 4`
- `elem_size(ErrorContextType()) = 4`
- Load: `lift_error_context(cx, load_int(cx, ptr, 4))`
- Store: `store_int(cx, lower_error_context(cx, v), ptr, 4)`

**Current Implementation:** No `ErrorContextType` in types package.

---

### 9. Stream and Future Types - NOT IMPLEMENTED

**Spec Reference:** CanonicalABI.md lines 1865, 1951

**Specification:**
- `alignment(StreamType() | FutureType()) = 4`
- `elem_size(StreamType() | FutureType()) = 4`

**Current Implementation:** No `StreamType` or `FutureType` in types package.

**Note:** These are part of async support, which may be deferred.

---

### 10. String Encoding Hint Preservation - MISSING

**Spec Reference:** CanonicalABI.md lines 2090-2132

**Specification Requirement:**
```python
String = tuple[str, str, int]  # (content, encoding, tagged_code_units)

def load_string(cx, ptr) -> String:
  ...
  return (s, cx.opts.string_encoding, tagged_code_units)
```

The encoding hint helps `store_string` make better allocation decisions.

**Current Implementation:** `LiftString` returns only `string`, losing the encoding hint.

**Impact:** Suboptimal memory allocation when strings are lifted and then lowered again.

---

### 11. Empty Types Not Prohibited - SPEC VIOLATION

**Spec Reference:** CanonicalABI.md lines 1930-1932

**Specification Requirement:**
```
Empty types, such as records with no fields, are not permitted, to avoid
complications in source languages.
```

And in elem_size_record: `assert(s > 0)`

**Current Implementation:** `types/composite.go:22-39`
```go
func (r Record) Size() uint32 {
    if len(r.Fields) == 0 {
        return 0  // Allows empty records!
    }
```

**Impact:** Potential issues with empty record handling.

---

### 12. Async Function Flattening - NOT IMPLEMENTED

**Spec Reference:** CanonicalABI.md lines 2736-2768

**Specification:**
```python
MAX_FLAT_ASYNC_PARAMS = 4

def flatten_functype(opts, ft, context):
  ...
  if opts.async_:
    # Different handling for async functions
```

**Current Implementation:** Only `MAX_FLAT_PARAMS` and `MAX_FLAT_RESULTS` defined.

---

## Minor Gaps (Nice to Fix)

### 13. Discriminant Type Boundary Condition

**Spec Reference:** CanonicalABI.md lines 1896-1903

**Specification:**
```python
def discriminant_type(cases):
  n = len(cases)
  assert(0 < n < (1 << 32))
  match math.ceil(math.log2(n)/8):
    case 0: return U8Type()  # n=1
    case 1: return U8Type()  # n=2..256
    case 2: return U16Type()
    case 3: return U32Type()
```

For n=1: `math.ceil(math.log2(1)/8) = 0` → U8Type
For n=256: `math.ceil(math.log2(256)/8) = 1` → U8Type
For n=257: `math.ceil(math.log2(257)/8) = 2` → U16Type

**Current Implementation:** `types/composite.go:89-100`
```go
case n <= 0x100: // 256
    return 1
case n <= 0x10000: // 65536
    return 2
```

This uses `n <= 256` which is equivalent to spec's case 0 and 1 combined.

**Analysis:** Current implementation is actually CORRECT since:
- n=1 to n=256 → 1 byte (spec: case 0 and case 1 both return U8Type)
- n=257 to n=65536 → 2 bytes (spec: case 2)

---

### 14. Flags with > 32 Labels

**Spec Reference:** CanonicalABI.md lines 1917-1921, 1979-1984

**Specification:**
```python
def alignment_flags(labels):
  n = len(labels)
  assert(0 < n <= 32)  # Limited to 32!
```

But `elem_size_flags` and flattening support multiple i32s.

**Current Implementation:** Supports n > 32 with multiple u32s, which is more permissive than spec's alignment assertion.

**Analysis:** The spec assertion `assert(0 < n <= 32)` appears to be a limit on individual flags types, but the implementation correctly handles larger counts. This may be a spec limitation being relaxed.

---

### 15. Signed Integer Lowering Representation

**Spec Reference:** CanonicalABI.md lines 3037-3041

**Specification:**
```python
def lower_flat_signed(i, core_bits):
  if i < 0:
    i += (1 << core_bits)
  return [i]
```

**Current Implementation:** `lower.go:21-29`
```go
case types.S8:
    return []uint64{uint64(uint32(int32(val.S8())))}, nil
```

**Analysis:** Both achieve 2's complement conversion. Current is correct but uses different method.

---

## Implementation Plan

### Phase 1: Critical Fixes (High Priority)

1. **Float NaN Canonicalization**
   - Add `canonicalizeNaN32` and `canonicalizeNaN64` functions
   - Update `LiftHeap` for F32/F64
   - Update `LiftFlat` for F32/F64
   - Files: `lift.go`, potentially `context.go`

2. **Alignment Validation**
   - Add string pointer alignment check in `strings.go`
   - Add list element alignment check in `lift.go`
   - Files: `strings.go`, `lift.go`

3. **Variant Type Coercion**
   - Implement proper `join` semantics in `flatten.go`
   - Add type coercion in `LiftFlat` for variants
   - Add type coercion in `LowerFlat` for variants
   - Files: `flatten.go`, `lift.go`, `lower.go`

4. **Resource Type Validation**
   - Add resource type field to handle lookup
   - Validate type match in `LiftOwn` and `LiftBorrow`
   - Files: `lift.go`, `lower.go`

### Phase 2: Major Improvements

5. **Fixed-Length Lists**
   - Add `Length *uint32` to `List` type
   - Update `Size()`, `Align()`, `FlattenCount()`
   - Update lifting/lowering logic
   - Files: `types/composite.go`, `lift.go`, `lower.go`, `flatten.go`

6. **Empty Type Prohibition**
   - Add validation in Record, Tuple construction
   - Files: `types/composite.go`

7. **Borrow Optimization**
   - Add same-instance check in `LowerBorrow`
   - Files: `lower.go`

### Phase 3: Async Support (Deferred)

8. **ErrorContext, Stream, Future Types**
   - Add new types to `types/` package
   - Implement lift/lower operations
   - Add async function flattening

---

## Conformance Test Additions

### Test Categories Needed:

1. **NaN Canonicalization Tests**
   - Lift various NaN bit patterns
   - Verify canonical pattern output

2. **Alignment Validation Tests**
   - Misaligned string pointers (should trap)
   - Misaligned list pointers (should trap)

3. **Variant Coercion Tests**
   - Variant with mixed f32/i32 cases
   - Variant with i32/i64 cases
   - Verify correct value reconstruction

4. **Fixed-Length List Tests**
   - Fixed-length list in flat representation
   - Fixed-length list alignment

5. **Resource Type Validation Tests**
   - Pass wrong handle type (should trap)

6. **Edge Case Tests**
   - Empty records (should error/trap)
   - Unicode edge cases in strings
   - Maximum size lists

---

## Regression Requirements

**CRITICAL:** All changes MUST ensure the following tests continue to pass:
- `internal/component/wasip2test/calculator_test.go` - `add` plugin tests
- `internal/component/wasip2test/calculator_test.go` - `subtract` plugin tests

Run verification:
```bash
go test -v ./internal/component/wasip2test/... -run "TestCalculator"
```

---

## References

- Primary Spec: `debug-vendored/component-model/design/mvp/CanonicalABI.md`
- Python Reference: `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py`
- Wasmtime Reference: `debug-vendored/wasmtime/crates/component-util/src/lib.rs`
- Wasmtime Values: `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/values.rs`
