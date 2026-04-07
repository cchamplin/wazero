# Loop 1 — Unify type representation, then make `abi/` correct

> **Status:** not started
>
> **Goal:** wazero has ONE type representation for component-model
> types. The binary parser populates `internal/component/types.ValType`
> directly. The four converters are gone. `abi/` correctly implements
> the synchronous canonical ABI as defined by `definitions.py` and
> `CanonicalABI.md`, with full Python test parity plus wazero
> supplemental tests.
>
> **Total items:** 35 across 6 phases
>
> Phases must be worked in order. Phase 1.A is a high-risk surgical
> change that leaves `go test ./...` broken until Loop 2 wires things
> in — this is expected and tracked in
> `loop-1-unification-status.md`. Phase 1.B and 1.C build the test
> infrastructure and red tests. Phase 1.D fills the gaps to make
> them green. Phase 1.E adds wazero supplemental tests. Phase 1.F is
> the terminal verification.

---

## Phase 1.A — Type unification (~11 items)

> Phase 1.A collapses two parallel type hierarchies into one. Read the
> design's "Architectural decisions" section before starting any item
> in this phase. The reference architecture is wasmtime's
> `ComponentTypes` and `go.bytecodealliance.org/wit`'s allocate-then-fill
> decoder.

### Item 1: Add `FixedSizeList` to `types.ValType` (or confirm existing `List{Length *uint32}` is sufficient)

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** -

**Files:**
- Read: `internal/component/types/composite.go`,
  `internal/component/types/types.go`
- Read: `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py`
  lines 351-361 (`ListType` definition)
- Read: `debug-vendored/wasmtime/crates/environ/src/component/types.rs`
  `TypeList` definition
- Modify: `internal/component/types/composite.go` (add `FixedSizeList`
  case if needed, OR document that `List{Length *uint32}` already
  covers it)
- Modify: `internal/component/types/composite_test.go` — add tests for
  the chosen shape

**Spec authorities:**
- `definitions.py:351-361` — `ListType(t, l=None)` where `l` is the
  optional fixed length
- `crates/wasmtime/src/runtime/component/values.rs` — wasmtime's
  treatment of fixed-length lists in lift/lower

**Description:**
The Python spec represents fixed-length lists as `ListType(t, l=N)`
with the fixed length as a constructor argument. Wazero already has
`types.List{Element ValType, Length *uint32}` which is structurally
equivalent.

Decide:
- **Option A:** Confirm `List{Length *uint32}` is sufficient. Document
  the decision in code comments. Update `composite_test.go` to
  exercise both `Length == nil` (dynamic) and `Length != nil` (fixed)
  cases.
- **Option B:** If wasmtime uses a separate `FixedLengthList` type
  in its dispatch (not just an optional length on `List`), add
  `types.FixedSizeList` as a distinct case for parity. Update lift/lower
  dispatch in Loop 1.D to handle it.

Read both spec sources before deciding. Cite the spec line in the
commit message.

**Definition of done:**
- The decision is documented in code comments and in the commit
  message
- If Option A: tests cover both nil and non-nil `Length` cases; this
  item is essentially a no-code change that just verifies and
  documents the existing shape
- If Option B: the new type case is added with tests
- The decision is consistent with how `definitions.py` and wasmtime
  represent the type
- `go test ./internal/component/types/...` passes

**Reviewer focus areas:**
- Spec compliance: confirm the chosen shape matches at least one of
  the spec sources (cite line); confirm the decision is justified
  (not just "looks similar")
- Code quality: confirm the decision is documented in code, not just
  in the commit; confirm tests cover both shapes

---

### Item 2: Add `Stream`, `Future`, `ErrorContext` types to `types.ValType` as recognised cases that trap on lift/lower

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Async runtime is OUT OF SCOPE for this project; these types exist only so the parser can produce complete output and lift/lower can trap with a clear message

**Files:**
- Modify: `internal/component/types/composite.go` — add `Stream`,
  `Future`, `ErrorContext` as new `ValType` cases
- Modify: `internal/component/types/types_test.go` — add tests
  asserting the new types exist and have correct `Align`/`Size`/
  `FlattenCount` per spec
- Modify: `internal/component/abi/lift.go` — add a `case types.Stream`,
  `case types.Future`, `case types.ErrorContext` in each of the four
  dispatch functions (`LiftFlat`, `LiftHeap`, `LowerFlat`, `LowerHeap`)
  that returns an error like `fmt.Errorf("async type %T not yet
  supported in synchronous canonical ABI", t)`
- Modify: `internal/component/abi/lift_test.go`,
  `internal/component/abi/lower_test.go` — add tests asserting the
  trap

**Spec authorities:**
- `definitions.py` — `StreamType`, `FutureType`, `ErrorContextType`
  definitions (search the file)
- `definitions.py` — `lift_flat`/`lower_flat` cases for these types
  (the spec Python implements them; we document them as deferred)
- `CanonicalABI.md` "Streams and Futures" section (if it exists)

**Description:**
Add the three async-related value types to wazero's type representation
as recognised cases. They are not implemented; they trap on
lift/lower. This documents the surface and lets the binary parser
produce complete type output without crashing.

The three new types:

```go
// Stream is the type of a wasi-async stream<T>. Out of scope for the
// canonical ABI unification project; lift/lower of streams traps with
// "async not yet supported". See
// docs/plans/2026-04-07-canonical-abi-unification-design.md "Out of scope".
type Stream struct {
    Element ValType  // may be nil for stream<>
}

func (Stream) valType() {}
func (Stream) Align() uint32       { return 4 }  // per spec definitions.py:XXX
func (Stream) Size() uint32        { return 4 }  // 32-bit handle
func (Stream) FlattenCount() int   { return 1 }
```

(Same shape for `Future` and `ErrorContext`. Read `definitions.py` for
the exact `Align`/`Size`/`FlattenCount` values — do not invent them.)

In `abi/lift.go` and `abi/lower.go`, add the trap cases:

```go
case types.Stream:
    return nil, fmt.Errorf("stream<T> lift not yet supported (async deferred to follow-up project)")
case types.Future:
    return nil, fmt.Errorf("future<T> lift not yet supported (async deferred to follow-up project)")
case types.ErrorContext:
    return nil, fmt.Errorf("error-context lift not yet supported (async deferred to follow-up project)")
```

**Definition of done:**
- `Stream`, `Future`, `ErrorContext` types exist in
  `internal/component/types/composite.go`
- Each has `Align`, `Size`, `FlattenCount` methods that match
  `definitions.py`
- All four dispatch functions in `abi/` trap on these types with a
  clear "async not yet supported" message
- Tests assert the existence of the types and the trap behavior
- `go test ./internal/component/types/...` and
  `go test ./internal/component/abi/...` pass

**Reviewer focus areas:**
- Spec compliance: confirm `Align`/`Size`/`FlattenCount` values match
  `definitions.py` (cite line); confirm the trap message correctly
  identifies these as async features
- Code quality: confirm the trap message is informative; confirm no
  TODO comments; confirm tests assert the trap, not just the type's
  existence

---

### Item 3: Change `types.Own{ResourceIdx}` → `types.Own{Resource *ResourceType}` (and same for `types.Borrow`)

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Removes the existing TODO comments at internal/component/types/resource.go lines 14-16 and 35-38

**Files:**
- Modify: `internal/component/types/resource.go` — change `Own` and
  `Borrow` struct field; remove the TODO comments
- Modify: every existing reference to `types.Own{ResourceIdx: ...}`
  and `types.Borrow{ResourceIdx: ...}` in the codebase — migrate to
  `types.Own{Resource: ...}` (use Grep first to find them all)
- Modify: tests

**Spec authorities:**
- `definitions.py:1641` — `lower_own(cx, rep, t)` where `t` carries
  the full `OwnType(rt)` with `rt: ResourceType`
- `crates/wasmtime/src/runtime/component/values.rs:115` — wasmtime
  takes `InterfaceType::Own(idx)` where `idx` resolves to a real
  `TypeResourceTable` joined to `ResourceType`
- `crates/wasmtime/src/runtime/component/types.rs` (the runtime type
  module, not environ) — `Handle<T>` API exposes resource types as
  pointers

**Description:**
Currently `types.Own{ResourceIdx uint32}` and `types.Borrow{ResourceIdx
uint32}` carry only an index. The acknowledged TODO in
`internal/component/types/resource.go` says this is wrong: per spec,
the type carries a direct reference to the resource type, not just
its index.

Change the shape to:

```go
// Own is the type of own<T> for some resource T. The Resource field
// points to the resource type metadata (destructor, source instance,
// type identity).
type Own struct {
    Resource *ResourceType
}

// Borrow is the type of borrow<T> for some resource T.
type Borrow struct {
    Resource *ResourceType
}
```

Where `*ResourceType` is the existing struct in
`internal/component/types/resource.go`.

The `ResourceIdx uint32` field is removed. Every existing reference
to it (use Grep) is migrated to use `Resource.Index` if the index is
genuinely needed (e.g. for serialization), or replaced with a direct
`Resource` pointer if the consumer just needs the type metadata.

The TODO comments at lines 14-16 and 35-38 in `resource.go` are
deleted (the work is done).

**Definition of done:**
- `types.Own` and `types.Borrow` carry `Resource *ResourceType`
- TODO comments at the cited lines are gone
- Every caller of the old shape is migrated (Grep returns zero for
  `Own{ResourceIdx`, `Borrow{ResourceIdx`)
- `go test ./internal/component/types/...` passes
- `go test ./internal/component/abi/...` passes (the existing
  standalone `LiftOwn`/`LowerOwn` may need a signature update; that
  update is part of this item)

**Reviewer focus areas:**
- Spec compliance: confirm the new shape matches `definitions.py`
  (cite line); confirm `*ResourceType` is the right struct (compare
  to wasmtime's `ResourceType` enum)
- Code quality: confirm zero references to the old field name remain;
  confirm the TODO comments are deleted (not just commented out);
  confirm no callers were missed

---

### Item 4: Add `Refines` field to `types.Variant.Case`

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** The binary parser already reads this field; the runtime form drops it

**Files:**
- Read: `internal/component/binary/types.go` — confirm
  `VariantCase.Refines` exists at the binary layer
- Modify: `internal/component/types/composite.go` — add
  `Refines *string` (or whatever the spec shape is) to `types.Case`
- Modify: every caller that constructs a `types.Case` to thread the
  refines value through

**Spec authorities:**
- `definitions.py` — `CaseType` definition (search for `class CaseType`)
- `CanonicalABI.md` — variant section discussing refinement
- `internal/component/binary/types.go` — the existing
  `VariantCase.Refines` field shape

**Description:**
The audit found that `internal/component/binary/types.go` has a
`VariantCase.Refines` field that the binary parser populates, but
`types.Case` (in the runtime form) drops it. This is information loss
during the converter step that's being eliminated.

Add `Refines` to `types.Case`. Check the spec for the type — it's
either `*string` (case label) or an index. Use whatever the binary
parser produces.

**Definition of done:**
- `types.Case` has a `Refines` field
- The shape matches `definitions.py` `CaseType`
- Existing `Variant` lift/lower in `abi/` does not use `Refines` (it
  doesn't need to; refinement is for static type-checking, not
  runtime lift/lower) but the field is preserved
- `go test ./internal/component/types/...` passes

**Reviewer focus areas:**
- Spec compliance: cite `definitions.py` for the field shape; confirm
  this matches what the binary parser reads
- Code quality: confirm the field is exported (public) consistent
  with other `types.Case` fields; confirm doc comment

---

### Item 5: Add `CanonicalAbiInfo` cache to each composite (`Record`, `Variant`, `List`, `Tuple`, `Flags`, `Enum`, `Option`, `Result`)

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Wasmtime precomputes these per spec definitions.py alignment/size/flatten functions

**Files:**
- Modify: `internal/component/types/composite.go` — add a
  `CanonicalAbiInfo` field (or pre-compute and inline) on each
  composite struct
- Modify: `internal/component/types/composite_test.go` — add tests
  asserting the cached values match the spec for representative
  composites
- Modify: callers that compute `Align`/`Size`/`FlattenCount` to use
  the cached value

**Spec authorities:**
- `definitions.py` `alignment(t)`, `elem_size(t)`, `flatten_type(t)`
  functions
- `crates/wasmtime/src/runtime/environ/component/types.rs` —
  `CanonicalAbiInfo {size32, align32, size64, align64, flat_count}`

**Description:**
Wasmtime stores precomputed `CanonicalAbiInfo` on each composite
(record, variant, list, tuple, flags, enum, option, result) so that
lift/lower never recomputes alignment, size, or flatten count. Wazero
currently recomputes them on every call.

Add the cache:

```go
// CanonicalAbiInfo holds precomputed canonical ABI metadata for a
// composite type. It is computed once when the type is constructed
// (in the binary parser) and consulted by every subsequent lift/lower
// operation. Spec authority: definitions.py alignment(t), elem_size(t),
// flatten_type(t).
type CanonicalAbiInfo struct {
    Size32     uint32
    Align32    uint32
    Size64     uint32  // for memory64; can be deferred or set equal to Size32 if memory64 is not in scope
    Align64    uint32
    FlatCount  int     // number of core values in the flat representation
}
```

Add this as a field on each composite struct. Compute it in the
binary parser (Item 6) — for now, add a constructor or `init()` style
method that populates the cache from the existing `Align`/`Size`/
`FlattenCount` methods.

(Memory64 is out of scope for this project per the design; either
defer the `Size64`/`Align64` fields and document why, or set them
equal to `Size32`/`Align32` and document.)

The existing `Align()`, `Size()`, `FlattenCount()` methods become thin
accessors that read from the cache.

**Definition of done:**
- Every composite has a `CanonicalAbiInfo` field (or equivalent)
- The cache is populated when the type is constructed
- `Align`/`Size`/`FlattenCount` methods read from the cache
- Tests confirm the cached values for representative composites
  (record-of-primitives, variant-with-mixed-cases, list-of-string,
  flags-of-many-bits) match `definitions.py`
- `go test ./internal/component/types/...` passes

**Reviewer focus areas:**
- Spec compliance: confirm the cached values match `definitions.py`
  for at least 3 representative composites (cite the lines)
- Code quality: confirm the cache is computed once and not on every
  call; confirm `Align`/`Size`/`FlattenCount` are now thin accessors

---

### Item 6: Refactor binary parser to populate `types.ValType` directly via allocate-then-fill

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** This is the largest single item in the project. Reference: go.bytecodealliance.org/wit/codec.go and internal/wasm/module.go.

**Files:**
- Read: `debug-vendored/go-modules/wit/codec.go` (the canonical
  allocate-then-fill pattern)
- Read: `internal/wasm/module.go` (wazero's own one-representation
  precedent for core wasm)
- Read: `internal/component/binary/decoder.go` and other binary parser
  files
- Modify: `internal/component/binary/decoder.go` (and any companion
  parser files) to populate `types.ValType` directly during decode
- Modify: `internal/component/binary/decoder_test.go` — adjust tests
  to expect `types.ValType` output instead of `binary.TypeDef`

**Spec authorities:**
- `debug-vendored/go-modules/wit/codec.go` lines around `getTypeDef`
  and `decodeTypeDef` — the allocate-then-fill pattern
- `internal/wasm/module.go` lines around `Module.TypeSection`
  population
- The component-model binary format spec at
  `debug-vendored/component-model/design/mvp/Binary.md` (if it
  exists; otherwise the relevant section of the WebAssembly spec)

**Description:**
Currently the binary parser produces `binary.TypeDef` (a wide tagged
struct with `*RecordTypeDef`, `*VariantTypeDef`, ...). Then a
converter step (the four functions deleted in items 8-9) translates
`binary.TypeDef` into `types.ValType`.

Eliminate the intermediate form. The parser produces `types.ValType`
directly via allocate-then-fill:

1. **First pass:** walk the binary type section. For each type, allocate
   a `*types.ValType` slot in a slice indexed by type-section index.
   Do not fill it yet — the slots are zero-value.

2. **Second pass:** walk the type section again. For each type, read
   its tag and contents from the binary, then fill the corresponding
   slot. References to other types (e.g. record field type indices)
   become pointers into the slice.

3. **Validation:** after filling, walk the slice and confirm every
   slot is non-zero and structurally valid.

This pattern handles recursion naturally: `record { foo: list<self> }`
fills the list's element pointer with a pointer to the record's own
slot, which is allocated but not yet filled at the time the inner
list is parsed.

The reference is `wit/codec.go::getTypeDef` which returns a `*TypeDef`
that may be partially filled at the time it is returned. Read this
file in full before starting.

When this item is complete, `binary.TypeDef` and the parallel
`binary.{Record,Variant,List,...}TypeDef` structs are still in place
(for now) — they're deleted in item 7. This item is just the parser
refactor.

**Definition of done:**
- The binary parser produces `[]types.ValType` (or equivalent
  `*types.ValType` slice) populated via allocate-then-fill
- The output matches the existing `binary.TypeDef` content for every
  test case in `decoder_test.go` (the converter is now in the parser
  itself)
- Recursion (e.g. record fields referring to outer record) works
- `go test ./internal/component/binary/...` passes
- A new test exercises a recursive type (e.g.
  `type recursive = list<recursive>` if syntactically valid)

**Reviewer focus areas:**
- Spec compliance: confirm the parser handles every type case the
  spec defines; cite the binary format spec
- Code quality: confirm the allocate-then-fill pattern is implemented
  correctly (no half-filled slots leaking out); confirm the
  reference (`wit/codec.go`) was actually consulted; confirm no
  duplicate logic between this and the (about-to-be-deleted) converter

---

### Item 7: Delete `binary.TypeDef` and the parallel `binary.{Record,Variant,List,Option,Result,Tuple,Flags,Enum}TypeDef` structs

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Depends on item 6

**Files:**
- Modify: `internal/component/binary/types.go` — delete the structs
- Modify: any caller that referenced the deleted structs (use Grep
  first)
- Modify: `internal/component/binary/types_test.go` — delete tests of
  the deleted structs

**Spec authorities:**
- N/A — this is a deletion item

**Description:**
After item 6 the parser produces `types.ValType` directly. The
intermediate `binary.TypeDef` and its `*RecordTypeDef`,
`*VariantTypeDef`, `*ListTypeDef`, `*OptionTypeDef`, `*ResultTypeDef`,
`*TupleTypeDef`, `*FlagsTypeDef`, `*EnumTypeDef` structs have no
remaining production callers. Delete them.

Use Grep to find every reference. Migrate each to the new
`types.ValType` shape if the caller is in production code; delete if
the caller was a test of the deleted struct.

**Definition of done:**
- `binary.TypeDef` and the parallel structs are deleted
- Grep returns zero for each deleted name
- `go test ./internal/component/binary/...` passes

**Reviewer focus areas:**
- Spec compliance: N/A
- Code quality: confirm zero references remain; confirm no caller was
  missed

---

### Item 8: Delete the three duplicate type converters in `component_linker.go`

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Depends on items 6 and 7. Confirmed by audit research that these have zero external production callers.

**Files:**
- Modify: `internal/component/component_linker.go` — delete
  `resolveToValType` (around line 722), `typeDefToValType` (around
  748), `valTypeRefToValType` (around 833)
- Modify: any caller (the audit found 4 callers in
  `component_linker.go` itself at lines 2120, 2125, 2308, 2313 — Grep
  to confirm)
- Modify: `internal/component/component_linker_test.go` — delete
  tests of these functions

**Spec authorities:**
- N/A — deletion item

**Description:**
The audit confirmed:
- `typeDefToValType` and `valTypeRefToValType` have zero external
  production callers; they only call each other recursively from
  `resolveToValType`
- `resolveToValType` has 4 callers, all in `component_linker.go`
  (lines 2120, 2125, 2308, 2313)
- `valTypeRefToValType` has an actively dangerous bug at line 882:
  returns `types.U32{}` on lookup failure

After items 6 and 7, the binary parser produces `types.ValType`
directly. The 4 callers of `resolveToValType` should now be able to
read the type from the parser's output without conversion.

Migrate the 4 callers to use the parser-produced types directly.
Delete the three converter functions. Delete their tests.

**Definition of done:**
- `resolveToValType`, `typeDefToValType`, `valTypeRefToValType`
  deleted
- 4 callers migrated to use the parser-produced types
- Grep returns zero for each deleted name
- `go test ./internal/component/...` passes (or shows only the
  expected pre-existing failures from the rest of phase 1.A)

**Reviewer focus areas:**
- Spec compliance: confirm the migrated callers receive structurally
  identical types to what the converter produced (compare via debug
  print or test fixture)
- Code quality: confirm zero references; confirm tests for migrated
  callers still pass

---

### Item 9: Delete `(*TypeResolver).resolveDefinedType` and its per-shape helpers

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Depends on item 8. The 3 production callers in instance.go:198/301/440 need to migrate.

**Files:**
- Modify: `internal/component/type_resolver.go` — delete
  `resolveDefinedType`, `resolveRecord`, `resolveVariant`, `resolveList`,
  `resolveOption`, `resolveResult`, `resolveTuple`, `resolveFlags`,
  `resolveEnum`. Decide whether the rest of `TypeResolver` (e.g. lookup
  helpers) should remain or also be deleted.
- Modify: `internal/component/instance.go` lines 198, 301, 440 —
  migrate to use parser-produced types
- Modify: `internal/component/type_resolver_test.go` — delete tests of
  the deleted methods

**Spec authorities:**
- N/A — deletion item

**Description:**
After items 6, 7, 8 the only remaining type converter is
`TypeResolver.resolveDefinedType` and its per-shape helpers. The 3
production callers in `instance.go` (lines 198, 301, 440) need to
migrate to read parser-produced types directly.

If `TypeResolver` has any remaining responsibilities besides
type conversion (e.g. resolving named imports across instance scopes),
preserve those and rename the struct (e.g. to `TypeLookup`). If
type conversion was its only purpose, delete the struct entirely.

**Definition of done:**
- `resolveDefinedType` and per-shape helpers deleted
- 3 production callers migrated
- `TypeResolver` either deleted or trimmed to its non-converter role
- `go test ./internal/component/...` passes (or shows expected
  pre-existing failures)

**Reviewer focus areas:**
- Spec compliance: N/A
- Code quality: confirm the migration of `instance.go:198/301/440` is
  semantically equivalent (same types come out); confirm no orphan
  helpers in `TypeResolver`

---

### Item 10: Run full test suite, document expected failures in `loop-1-unification-status.md`

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Phase 1.A is intentionally disruptive; this item captures the broken state for Loop 2 to fix.

**Files:**
- Run: `go test ./...` from repo root
- Create: `docs/plans/projects/abi-unification/loop-1-unification-status.md`

**Spec authorities:**
- N/A — status capture item

**Description:**
After items 1-9 the type representation is unified but the production
lift/lower paths in `instance.go`, `component_linker.go`, and
`canon_lower.go` still exist with their bugs. They may still compile
(if items 8-9 successfully migrated their type-converter callers) but
their lift/lower bugs are unchanged.

Run `go test ./...`. Capture every failing test with its failure
message. Categorise:

1. **Failures expected to be fixed by Loop 2** — broken because
   production paths still call the old buggy lift/lower code. Loop 2
   wires `abi/` in and these failures resolve.
2. **Failures caused by phase 1.A bugs** — meaning items 1-9
   introduced regressions. These are blockers for phase 1.A; bounce
   the responsible item.
3. **Failures unrelated to this project** — pre-existing bugs that
   neither Loop 1 nor Loop 2 will fix. Document them as
   out-of-scope.

Write `loop-1-unification-status.md`:

```markdown
# Loop 1 Phase 1.A Status

After items 1-9, the type representation is unified. The production
lift/lower paths in instance.go, component_linker.go, canon_lower.go
still exist with their pre-existing bugs and have not yet been wired
to abi/. This document captures the test status as expected.

## Test results
go test ./... output: <captured>

## Categorised failures
### Expected to be fixed by Loop 2 (production wiring)
- TestX in pkgY — currently fails because <reason>; will pass after
  Loop 2 item N

### Phase 1.A regressions (must be fixed before Phase 1.B starts)
- (none, hopefully)

### Out of scope (pre-existing, neither loop will fix)
- TestZ in pkgW — pre-existing failure unrelated to this project
```

If category 2 has any entries, the responsible Phase 1.A item is
bounced.

**Definition of done:**
- `loop-1-unification-status.md` exists with all three categories
  filled
- Category 2 (phase 1.A regressions) is empty (any entries trigger
  bounce of the responsible item)
- `go test ./internal/component/types/...` and
  `go test ./internal/component/binary/...` and
  `go test ./internal/component/abi/...` are individually green
  (these packages should be standalone after phase 1.A)

**Reviewer focus areas:**
- Spec compliance: N/A
- Code quality: confirm the test run was actually performed (capture
  output); confirm the categorisation is honest

---

### Item 11: Reviewer subagent verifies new `types.ValType` shape matches spec for every type category

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Independent verification of phase 1.A's correctness before phase 1.B starts

**Files:**
- Read: `internal/component/types/composite.go`,
  `internal/component/types/types.go`,
  `internal/component/types/resource.go`
- Read: `definitions.py`
- Create: `docs/plans/projects/abi-unification/loop-1-phase-1a-spec-review.md`

**Spec authorities:**
- `definitions.py` — every value type definition

**Description:**
Dispatch a fresh subagent (using
`templates/review-spec-compliance.md`) with the scope set to "the
shape of `internal/component/types.ValType` after phase 1.A". The
subagent reads `definitions.py` and verifies that every value type
the spec defines has a corresponding case in wazero's
`types.ValType`, with structurally matching fields.

Coverage to verify:
- Bool, S8/U8, S16/U16, S32/U32, S64/U64, F32/F64, Char, String —
  primitives
- List (dynamic and fixed-length per item 1)
- Record (with Field name + type)
- Tuple
- Variant (with Case name + type + refines per item 4)
- Option, Result, Enum, Flags
- Own (with `*ResourceType` per item 3)
- Borrow (with `*ResourceType` per item 3)
- Stream, Future, ErrorContext (added per item 2)

The subagent's report goes in `loop-1-phase-1a-spec-review.md`.
Verdict must be `PASS` before phase 1.B starts.

**Definition of done:**
- Subagent dispatched
- `loop-1-phase-1a-spec-review.md` exists with the subagent's findings
- Verdict is `PASS` (any blocker becomes a sub-item; bounce phase
  1.A's relevant item)

**Reviewer focus areas:**
- This IS the spec review

---

## Phase 1.B — Test infrastructure (3 items)

### Item 12: Create `internal/component/conformance/canonical_abi/` subdir + doc.go + python_reference.go

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** -

**Files:**
- Create: `internal/component/conformance/canonical_abi/doc.go`
- Create: `internal/component/conformance/canonical_abi/python_reference.go`

**Spec authorities:**
- `debug-vendored/component-model/design/mvp/canonical-abi/run_tests.py`

**Description:**
Create the conformance subdir for the Python test ports. The doc.go
file documents the package's purpose and the source-of-truth invariant:

```go
// Package canonical_abi contains direct Go ports of the canonical-ABI
// Python reference test suite at
// debug-vendored/component-model/design/mvp/canonical-abi/run_tests.py.
//
// Source-of-truth invariant: every test in this package corresponds
// 1:1 with a row or assertion in run_tests.py. Each Go subtest carries
// a comment citing the source line in run_tests.py. To modify a test:
//
// 1. Update run_tests.py upstream (preferably propose a PR to the
//    component-model repo)
// 2. Update the SHA in python_reference.go
// 3. Update the corresponding Go test
//
// Wazero supplemental tests (cases not in run_tests.py) live in
// supplemental_test.go and are clearly marked.
package canonical_abi
```

The python_reference.go file holds spec constants and source-line
references:

```go
package canonical_abi

// PythonReferenceSHA records the upstream commit of run_tests.py at
// the time these tests were ported. Update when re-syncing.
const PythonReferenceSHA = "<sha>"

// Spec constants from definitions.py
const (
    MaxFlatParams        = 16     // definitions.py:XXX
    MaxFlatResults       = 1      // definitions.py:XXX
    CanonicalFloat32NaN  = 0x7FC00000  // definitions.py:XXX
    CanonicalFloat64NaN  = 0x7FF8000000000000  // definitions.py:XXX
    Utf16Tag             = 1 << 31  // definitions.py:XXX
    DeterministicProfile = true   // definitions.py:XXX
)

// SourceLine records the run_tests.py line that a Go test ports.
type SourceLine struct {
    Line int
    Description string
}
```

**Definition of done:**
- `internal/component/conformance/canonical_abi/` directory exists
- `doc.go` documents the source-of-truth invariant
- `python_reference.go` exists with all spec constants from
  `definitions.py`, each citing the line
- `go build ./internal/component/conformance/canonical_abi/...`
  succeeds (no test code yet)

**Reviewer focus areas:**
- Spec compliance: confirm every constant value in
  `python_reference.go` matches `definitions.py` (cite line for each)
- Code quality: confirm doc comments; confirm package name follows
  Go conventions

---

### Item 13: Define table-driven helpers matching run_tests.py's helpers

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Depends on item 12

**Files:**
- Create: `internal/component/conformance/canonical_abi/helpers_test.go`

**Spec authorities:**
- `run_tests.py` lines for `test()` (105), `test_pairs()` (180),
  `test_heap()` (284), `test_flatten()` (372), `test_nan32`/`test_nan64`
  (203/217), `test_string()` (253), `test_roundtrip()` (399),
  `test_reentrance()` (2765)

**Description:**
The Python test suite uses helper functions to dispatch each
parametric test category. Port each as a Go helper that takes a row
struct and runs the assertions.

Example for `test()`:

```go
// runTest is the Go equivalent of run_tests.py's test() helper at
// line 105. It asserts (a) lift_flat(vi, t) equals expected value,
// and (b) lower-then-re-lift through a fresh heap is stable.
func runTest(t *testing.T, valType types.ValType, flatVals []core.Value, expected component.Val) {
    t.Helper()
    // ... port the Python logic
}

// runTestPairs is the Go equivalent of run_tests.py's test_pairs()
// at line 180. Each row is (input, expected) for primitive coercion.
func runTestPairs[T any](t *testing.T, valType types.ValType, pairs []struct{ In, Out T }) {
    t.Helper()
    // ...
}

// ... and so on for runTestHeap, runTestFlatten, runTestNan,
// runTestStringEncoding, runTestRoundtrip, runTestReentrance
```

The exact signatures depend on the Go types after phase 1.A. Read
`internal/component/types/composite.go` and `core.Value` (or whatever
the Go equivalent of Python's `int`/`float` flat values is) before
writing.

**Definition of done:**
- `helpers_test.go` exists with one helper per Python helper
- Each helper has a doc comment citing the Python line
- Helpers compile and pass a smoke test (write one trivial test row
  for each helper to confirm the helper executes correctly)
- `go test ./internal/component/conformance/canonical_abi/...` passes
  the smoke tests

**Reviewer focus areas:**
- Spec compliance: confirm each helper's behavior matches the Python
  helper's behavior (cite Python line; describe the assertion shape)
- Code quality: confirm idiomatic Go (use generics where Python uses
  duck typing); confirm `t.Helper()` calls; confirm doc comments

---

### Item 14: Verify spec constants in python_reference.go are complete

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Cross-check against definitions.py before phase 1.C ports start

**Files:**
- Read: `definitions.py` from top to bottom
- Modify: `python_reference.go` if any constant is missing

**Spec authorities:**
- `definitions.py` — every module-level constant

**Description:**
Walk `definitions.py` from top to bottom. For every module-level
constant, verify it has a corresponding Go constant in
`python_reference.go`. The set should include at least:

- `MAX_FLAT_PARAMS`, `MAX_FLAT_RESULTS`
- `CANONICAL_FLOAT32_NAN`, `CANONICAL_FLOAT64_NAN`
- `UTF16_TAG`
- `DETERMINISTIC_PROFILE`
- Any `EventCode.*` enum values (deferred to async loop, but
  documented as such)
- Any `Subtask.State.*` enum values (deferred)
- Any `CopyResult.*` enum values (deferred)
- Any `CallbackCode.*` enum values (deferred)
- Any string-encoding tags

For deferred (async-related) constants, add a comment explaining
they're deferred but document the value:

```go
// Subtask.State enum values from definitions.py:XXX. These are used
// only by async tests, which are deferred to the follow-up async
// project. Defined here for completeness so that the constants are
// available if the project is reopened.
```

**Definition of done:**
- Every module-level constant in `definitions.py` has a corresponding
  Go constant
- Each citation matches the actual `definitions.py` line
- Deferred constants are documented as such
- `go build ./internal/component/conformance/canonical_abi/...`
  succeeds

**Reviewer focus areas:**
- Spec compliance: walk `definitions.py` independently and confirm
  no constant is missing
- Code quality: confirm citations are correct (read the cited line)

---

## Phase 1.C — Port Python tests, expected to fail (9 items)

> Each item in this phase ports one category from `run_tests.py`. Tests
> are expected to FAIL until phase 1.D fills the corresponding gap in
> `abi/`. Use `t.Run` for each row so failures are reported per-row.
>
> Ports must be **direct** (the same input produces the same expected
> output). Do not "improve" the test; the spec wins.

### Item 15: Port primitive coercion tests (test_pairs, ~55 cases) → primitives_test.go

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Depends on item 13

**Files:**
- Create: `internal/component/conformance/canonical_abi/primitives_test.go`

**Spec authorities:**
- `run_tests.py:180-202` (`test_pairs` definition and the ~14 calls
  with their ~55 expanded sub-cases)

**Description:**
Port every `test_pairs(...)` call from `run_tests.py`. Each Python row
becomes a Go row in a table-driven test:

```go
// TestPrimitiveCoercion ports test_pairs from run_tests.py:180-202.
// Each row is one (input, expected) pair from the Python suite.
func TestPrimitiveCoercion(t *testing.T) {
    t.Run("S8", func(t *testing.T) {
        // run_tests.py:187: test_pairs(S8Type(), [(127,127),(128,-128),(255,-1),...])
        runTestPairs(t, types.S8{}, []struct{ In, Out int8 }{
            {127, 127},
            {-128, -128},  // 128 → -128 by 8-bit modular arithmetic
            {-1, -1},      // 255 → -1
            // ... port every Python row
        })
    })

    // ... one t.Run per primitive type covered by test_pairs
}
```

For Python's arbitrary-precision int → Go's fixed-size int:
- `(1<<32)-1` → `uint32(0xFFFFFFFF)`
- `(1<<63)-1` → `int64(0x7FFFFFFFFFFFFFFF)`

For Python's float → Go's float (use bit patterns where needed):
- `(3.14, 3.14)` → `{In: 3.14, Out: 3.14}` works for f64
- For f32, use `math.Float32bits` if comparing against a specific
  bit pattern

**Definition of done:**
- Every `test_pairs` row in `run_tests.py` is ported
- Each Go subtest cites the source line
- Tests run via `go test -run TestPrimitiveCoercion
  ./internal/component/conformance/canonical_abi/...`
- Tests are EXPECTED TO FAIL at this stage (phase 1.D fills the gaps)
- The test file compiles cleanly

**Reviewer focus areas:**
- Spec compliance: count the rows in the Python source and confirm
  the same count in the Go port; spot-check 3-5 rows for value
  correctness
- Code quality: confirm citations; confirm idiomatic table-driven
  pattern; confirm no rows were silently skipped

---

### Item 16: Port lift/lower roundtrip composites tests (test, 23 cases) → composites_test.go

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** -

**Files:**
- Create: `internal/component/conformance/canonical_abi/composites_test.go`

**Spec authorities:**
- `run_tests.py:105-179` (`test` helper and the 23 direct invocations
  for record/tuple/list/flags/variant/option/result)

**Description:**
Port every direct `test(...)` call from `run_tests.py:105-179`. Each
becomes a row in a table-driven test for the relevant composite type.

```go
// TestComposites ports test() invocations from run_tests.py:105-179.
func TestComposites(t *testing.T) {
    t.Run("RecordOfPrimitives", func(t *testing.T) {
        // run_tests.py:145: test(RecordType([FieldType('x',U8Type()),...]),
        //                          [1,2,3], {'x':1,'y':2,'z':3})
        runTest(t, types.Record{
            Fields: []types.Field{
                {Name: "x", Type: types.U8{}},
                {Name: "y", Type: types.U8{}},
                {Name: "z", Type: types.U8{}},
            },
        }, []core.Value{1, 2, 3}, component.ValRecord(map[string]component.Val{
            "x": component.ValU8(1),
            "y": component.ValU8(2),
            "z": component.ValU8(3),
        }))
    })

    // ... 22 more t.Run blocks
}
```

(Exact `component.Val` and `core.Value` constructors depend on the
public API after phase 1.A. Read the actual types before writing.)

**Definition of done:**
- All 23 `test()` invocations are ported as table rows or t.Run
  blocks
- Each cites the source line
- Tests are EXPECTED TO FAIL at this stage
- The test file compiles cleanly

**Reviewer focus areas:**
- Spec compliance: count Python rows vs Go rows; spot-check 3-5
- Code quality: confirm citations; confirm consistent pattern with
  item 15

---

### Item 17: Port heap layout tests (test_heap, 31 cases) → heap_test.go

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** -

**Files:**
- Create: `internal/component/conformance/canonical_abi/heap_test.go`

**Spec authorities:**
- `run_tests.py:284-371` (`test_heap` definition and 31 invocations)

**Description:**
Port every `test_heap(...)` call. These are byte-level golden tests:
each row specifies `(type, expected value, ptrLenArgs, memoryBytes)`.

```go
// TestHeapLayout ports test_heap from run_tests.py:284-371.
func TestHeapLayout(t *testing.T) {
    t.Run("ListU16", func(t *testing.T) {
        // run_tests.py:295: test_heap(ListType(U16Type()), [1,2,3], [0,3], [1,0,2,0,3,0])
        runTestHeap(t, types.List{Element: types.U16{}},
            component.ValList(...{1,2,3}),
            []core.Value{0, 3},  // ptr=0, len=3
            []byte{1, 0, 2, 0, 3, 0},
        )
    })
    // ... 30 more
}
```

**Definition of done:**
- All 31 `test_heap` rows are ported
- Each cites the source line
- The test file compiles cleanly
- Tests are EXPECTED TO FAIL at this stage

**Reviewer focus areas:**
- Spec compliance: spot-check 3-5 rows for byte-level correctness
- Code quality: confirm consistent pattern

---

### Item 18: Port flatten signatures tests (test_flatten, 8 cases) → flatten_test.go

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** -

**Files:**
- Create: `internal/component/conformance/canonical_abi/flatten_test.go`

**Spec authorities:**
- `run_tests.py:372-398` (`test_flatten` definition and 8 invocations)

**Description:**
Port every `test_flatten(...)` call. These assert that
`flatten_functype` produces the expected core function signature.

```go
// TestFlattenFuncType ports test_flatten from run_tests.py:372-398.
func TestFlattenFuncType(t *testing.T) {
    t.Run("MixedPrimitives", func(t *testing.T) {
        // run_tests.py:389: test_flatten(FuncType([U8,F32,F64],[]),
        //                                ['i32','f32','f64'], [])
        runTestFlatten(t,
            types.FuncType{Params: []types.ValType{types.U8{}, types.F32{}, types.F64{}}},
            []string{"i32", "f32", "f64"},
            []string{},
        )
    })
    // ... 7 more
}
```

**Definition of done:**
- All 8 `test_flatten` rows are ported
- Each cites the source line
- The test file compiles cleanly
- Tests are EXPECTED TO FAIL at this stage

**Reviewer focus areas:**
Same as items 15-17.

---

### Item 19: Port NaN canonicalization tests (test_nan32/test_nan64, 14 cases) → nan_test.go

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** -

**Files:**
- Create: `internal/component/conformance/canonical_abi/nan_test.go`

**Spec authorities:**
- `run_tests.py:203-252` (`test_nan32`/`test_nan64` definitions and
  14 invocations)

**Description:**
Port every NaN canonicalization test. These assert that lifted NaN
bit patterns match the canonical NaN constant.

```go
// TestCanonicalNaN ports test_nan32/test_nan64 from run_tests.py:203-252.
func TestCanonicalNaN(t *testing.T) {
    t.Run("F32_QuietNaN", func(t *testing.T) {
        // run_tests.py:231: test_nan32(0x7fc00000, CANONICAL_FLOAT32_NAN)
        runTestNan32(t, 0x7fc00000, CanonicalFloat32NaN)
    })
    // ... 13 more
}
```

**Definition of done:**
- All 14 NaN test rows are ported
- Each cites the source line
- Tests use bit-level comparisons (`math.Float32bits`/`Float64bits`),
  NOT `==`
- The test file compiles cleanly
- Tests are EXPECTED TO FAIL at this stage

**Reviewer focus areas:**
- Spec compliance: confirm bit values match `definitions.py`
- Code quality: confirm bit-level comparison (Go `float32 ==` is
  unsafe for NaN)

---

### Item 20: Port string encoding matrix tests (135 cases) → string_encoding_test.go

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Largest single test file by case count

**Files:**
- Create: `internal/component/conformance/canonical_abi/string_encoding_test.go`

**Spec authorities:**
- `run_tests.py:253-283` (`test_string` definition and the
  3×3×15 nested loop)

**Description:**
Port the string encoding matrix. Python has a triple loop:

```python
for src in [Latin1Utf16, Utf8, Utf16]:
    for dst in [Latin1Utf16, Utf8, Utf16]:
        for s in fun_strings:
            test_string(s, src, dst)
```

Where `fun_strings` is a list of 15 sample strings (ASCII, BMP, emoji,
surrogate pairs, etc.).

Port the 15 strings as a Go `var funStrings = []string{...}` (with
each Python string preserved exactly, including emoji as Go string
literals).

Port the loop:

```go
func TestStringEncodingMatrix(t *testing.T) {
    encodings := []StringEncoding{Latin1Utf16, Utf8, Utf16}
    for _, src := range encodings {
        for _, dst := range encodings {
            for i, s := range funStrings {
                t.Run(fmt.Sprintf("%s_to_%s_%d", src, dst, i), func(t *testing.T) {
                    runTestStringEncoding(t, s, src, dst)
                })
            }
        }
    }
}
```

**Definition of done:**
- The matrix runs 3×3×15 = 135 subtests
- The 15 strings exactly match the Python `fun_strings` list (no
  truncation, no escape changes)
- Each subtest cites the Python loop
- The test file compiles cleanly
- Tests are EXPECTED TO FAIL at this stage

**Reviewer focus areas:**
- Spec compliance: spot-check 5 strings against the Python source
  (read the file)
- Code quality: confirm subtests are individually addressable

---

### Item 21: Port test_roundtrips (6 cases) → canon_lift_lower_test.go

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** -

**Files:**
- Create: `internal/component/conformance/canonical_abi/canon_lift_lower_test.go`

**Spec authorities:**
- `run_tests.py:399-440` (`test_roundtrips` definition and 6
  invocations)

**Description:**
Port `test_roundtrips`. These are end-to-end canon_lift roundtrips
through a fake component instance, exercising the full
flatten → invoke → re-lift path.

```go
// TestRoundtrips ports test_roundtrips from run_tests.py:399-440.
func TestRoundtrips(t *testing.T) {
    t.Run("ListString", func(t *testing.T) {
        // run_tests.py:XXX: test_roundtrip(ListType(StringType()), [mk_str("hello there")])
        runTestRoundtrip(t, types.List{Element: types.String{}},
            []component.Val{mkStr("hello there")},
        )
    })
    // ... 5 more
}
```

**Definition of done:**
- All 6 roundtrip rows are ported
- Each cites the source line
- The test file compiles cleanly
- Tests are EXPECTED TO FAIL at this stage (depends on phase 1.D
  items 25-26 for `CanonLift`/`CanonLower`)

**Reviewer focus areas:**
Same as items 15-20.

---

### Item 22: Port test_handles resource scenario (~50 asserts) → handles_test.go

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Most complex single port

**Files:**
- Create: `internal/component/conformance/canonical_abi/handles_test.go`

**Spec authorities:**
- `run_tests.py:441-551` (`test_handles` scenario)

**Description:**
Port `test_handles`. This is a single scenario test (~50 asserts)
exercising:
- `canon_resource_new`
- `canon_resource_drop`
- `canon_resource_rep`
- Lift/lower of own/borrow handles
- Resource table state transitions
- Borrow lifetime tracking

The test is scenario-style, not table-driven. Port it as a single Go
test function with t.Run blocks for each asserted state transition:

```go
// TestHandles ports test_handles from run_tests.py:441-551.
func TestHandles(t *testing.T) {
    // Set up resource table, instance, etc.

    t.Run("NewHandle", func(t *testing.T) {
        // run_tests.py:XXX: handle = canon_resource_new(rt, 42)
        // assert(table.get(handle).rep == 42)
    })

    t.Run("DropHandle", func(t *testing.T) {
        // ...
    })

    // ... per asserted state
}
```

**Definition of done:**
- Every assert in `test_handles` is ported as either a t.Run block or
  a check inside one
- Each ported assert cites the source line
- The test file compiles cleanly
- Tests are EXPECTED TO FAIL at this stage

**Reviewer focus areas:**
- Spec compliance: count asserts in Python vs Go; confirm none were
  silently skipped
- Code quality: confirm scenario state is set up cleanly; confirm
  t.Run blocks are independently addressable

---

### Item 23: Port test_reentrance (~12 asserts) → reentrance_test.go

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Pure boolean test on call_might_be_recursive

**Files:**
- Create: `internal/component/conformance/canonical_abi/reentrance_test.go`

**Spec authorities:**
- `run_tests.py:2765-2831` (`test_reentrance` scenario)

**Description:**
Port `test_reentrance`. ~12 boolean asserts on
`call_might_be_recursive(task, inst)` for various task chains. The
test has no I/O, no memory — just chain-construction and assertions.

This test should pass relatively easily since `call_might_be_recursive`
is a pure function over task graph data.

**Definition of done:**
- All ~12 asserts ported as table rows or t.Run blocks
- Each cites the source line
- The test file compiles cleanly
- Tests pass IF `call_might_be_recursive` is implemented in `abi/`
  (which it should be — verify before completing this item; if it's
  missing, that becomes a sub-item under phase 1.D)

**Reviewer focus areas:**
Same as items 15-22.

---

## Phase 1.D — Fill `abi/` gaps to make tests green (8 items)

> The Python tests ported in phase 1.C are expected to fail. Each item
> in phase 1.D fills one gap and makes one or more failing tests pass.
> Use red/green TDD: confirm the test fails, write the fix, confirm
> the test passes.

### Item 24: Add `types.Own`/`types.Borrow` cases to `LiftFlat`/`LowerFlat`/`LiftHeap`/`LowerHeap` dispatch

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Depends on Loop 1 phase 1.A item 3 (Own carries *ResourceType)

**Files:**
- Modify: `internal/component/abi/lift.go` — add `case types.Own:` and
  `case types.Borrow:` to `LiftFlat` and `LiftHeap`
- Modify: `internal/component/abi/lower.go` — add the same to
  `LowerFlat` and `LowerHeap`
- Modify: `internal/component/abi/lift_test.go`,
  `internal/component/abi/lower_test.go` — add tests for the
  integrated dispatch (record-of-own, list-of-borrow, etc.)

**Spec authorities:**
- `definitions.py:1197-1198` — `load(cx, ptr, t)` `case OwnType()`,
  `case BorrowType()`
- `definitions.py:1387-1388` — `store(cx, v, t, ptr)` symmetric
- `definitions.py:1792-1793` — `lift_flat()` Own/Borrow case
- `definitions.py:1886-1887` — `lower_flat()` Own/Borrow case
- `definitions.py:1333-1364` — `lift_own`/`lift_borrow` definitions
  (resource type validation at lines 2218-2219, 2237-2238)
- `crates/wasmtime/src/runtime/component/values.rs:115` — wasmtime's
  matched case

**Description:**
Currently `abi/`'s four dispatch functions do not handle `types.Own`
or `types.Borrow` — they fall through to the default error branch.
The standalone `LiftOwn`/`LiftBorrow` helpers are the only way to
lift a handle, and they're not called from inside the dispatch.

Add the integrated dispatch:

```go
// In LiftFlat:
case types.Own:
    handleIdx := iter.NextI32()
    return liftOwnInternal(cx, uint32(handleIdx), t.Resource)
case types.Borrow:
    handleIdx := iter.NextI32()
    return liftBorrowInternal(cx, uint32(handleIdx), t.Resource)

// Similar for LiftHeap (read 4 bytes from memory), LowerFlat,
// LowerHeap.
```

`liftOwnInternal` is the spec-correct lift_own that:
1. Looks up the handle in the resource table
2. **Traps** if `handle.rt is not t.Resource` per spec lines
   2218-2219, 2237-2238
3. Returns the rep

(The standalone `LiftOwn`/`LiftBorrow` exports remain in this item;
they're deleted in Loop 2 phase 2.D item 6 once production code uses
the integrated dispatch.)

**Definition of done:**
- All four dispatch functions handle `types.Own` and `types.Borrow`
- Resource type mismatch produces a trap (returned error) with a
  clear message
- Tests added for: composite-of-own (record with own field),
  composite-of-borrow, type mismatch trap, borrow lifetime tracking
- Tests from phase 1.C item 22 (handles) start passing for cases
  that exercise the integrated dispatch
- `go test ./internal/component/abi/...` passes

**Reviewer focus areas:**
- Spec compliance: confirm dispatch matches `definitions.py:1197/1792`;
  confirm trap matches `definitions.py:2218`
- Code quality: confirm no duplication with the existing standalones;
  confirm idiomatic Go; confirm no error suppression

---

### Item 25: Add `CanonLift` entry point with retptr param spill and result spill

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Top-level entry point that does not exist today

**Files:**
- Modify: `internal/component/abi/lift.go` (or create
  `internal/component/abi/canon_lift.go`) — add `CanonLift` function
- Modify: `internal/component/abi/lift_test.go` — add tests for
  param spill (>16 flat) and result spill (retptr)

**Spec authorities:**
- `definitions.py:3237+` — `canon_lift` definition
- `definitions.py:3113` — `lift_flat_values` (param spill)
- `definitions.py:3132` — `lower_flat_values` (result spill)
- `crates/wasmtime/src/runtime/component/func/typed.rs::call_raw` —
  wasmtime's retptr handling

**Description:**
Today `abi/` has `LiftFlat`, `LiftHeap`, `LowerFlat`, `LowerHeap` —
leaf operations. There is no top-level `CanonLift`/`CanonLower` that
handles flatten + spill + invoke + result-handling. Production code
has to reimplement the dispatcher externally; that's part of why the
parallel implementations exist.

Add `CanonLift`:

```go
// CanonLift implements the canon_lift abstract operation per
// definitions.py:3237. It lifts the wasm core stack values into
// component-level Vals, handling parameter spill (when len(flat) >
// MaxFlatParams) and result retptr.
//
// opts: canonical options (encoding, memory, realloc, post-return)
// ft: the component-level function type
// args: the wasm core stack as []core.Value
// callee: the inner host or guest function to invoke (returns
//   component Vals as its result)
//
// Returns the component-level result Vals, or an error.
func CanonLift(opts *Options, ft *types.FuncType, args []core.Value,
                callee func([]component.Val) ([]component.Val, error)) (
                []component.Val, error) {
    // 1. Determine if params need spill: flatten ft.Params,
    //    if len(flat) > MaxFlatParams, args[0] is a retptr to
    //    a packed memory image of the params.
    // 2. Lift each param via LiftFlat or LiftHeap.
    // 3. Invoke callee with the lifted Vals.
    // 4. Handle result: if flatten ft.Results > MaxFlatResults,
    //    write to a caller-provided retptr; otherwise pack into
    //    return values.
    // 5. Invoke post-return callback if configured.
}
```

(Exact signature depends on existing `abi/` API. Read the current
`Options` struct and `core.Value` shape before writing.)

The retptr handling references wasmtime's `call_raw` for the exact
spill semantics — read it.

Post-return invocation is item 27.

**Definition of done:**
- `CanonLift` exists and matches the spec
- Param spill works for >16 flat params (test with 17, 32, 100)
- Result spill works for >1 flat result (test with 2 results, 16
  results)
- Tests from phase 1.C items 21 (test_roundtrips) start passing
- `go test ./internal/component/abi/...` passes

**Reviewer focus areas:**
- Spec compliance: confirm the implementation matches `canon_lift` at
  `definitions.py:3237`; cite each step's source
- Code quality: confirm idiomatic Go; confirm error wrapping; confirm
  no helpers without consumers; confirm retptr handling matches
  wasmtime's `call_raw`

---

### Item 26: Add `CanonLower` entry point (mirror of CanonLift)

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** -

**Files:**
- Modify: `internal/component/abi/lower.go` (or create
  `internal/component/abi/canon_lower.go`) — add `CanonLower`
- Modify: `internal/component/abi/lower_test.go` — add tests

**Spec authorities:**
- `definitions.py:3453+` — `canon_lower` definition
- `crates/wasmtime/src/runtime/component/func/typed.rs::call_raw`

**Description:**
Mirror of item 25 for the lowering direction. Used by host imports:
the host receives lifted component Vals, the host computes a result,
the result is lowered into the wasm core stack.

```go
// CanonLower implements the canon_lower abstract operation per
// definitions.py:3453. It lowers component-level Vals into the wasm
// core stack, handling parameter and result spill via retptr.
func CanonLower(opts *Options, ft *types.FuncType, args []component.Val,
                stack []core.Value) error {
    // 1. Lower each param via LowerFlat or LowerHeap.
    // 2. If params overflow MaxFlatParams, spill to memory at a
    //    realloc-allocated buffer; pass the buffer ptr as the
    //    retptr arg.
    // 3. Invoke the wasm-side callee (this typically happens via
    //    the existing api.Function.Call machinery).
    // 4. Result is in the stack; the caller deals with it.
}
```

**Definition of done:**
- `CanonLower` exists and matches the spec
- Param spill works
- Result handling matches the spec
- Tests pass
- `go test ./internal/component/abi/...` passes

**Reviewer focus areas:**
Same as item 25.

---

### Item 27: Add post-return invocation in `CanonLift` per spec definitions.py:3197+

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Synchronous post-return only; async-flavored post-return is out of scope

**Files:**
- Modify: `internal/component/abi/lift.go` (or `canon_lift.go`) —
  add post-return invocation to `CanonLift`
- Modify: `internal/component/abi/lift_test.go` — add a test that
  configures a post-return callback and asserts it's invoked

**Spec authorities:**
- `definitions.py:3197+` — post-return semantics
- `CanonicalABI.md` "Post-return" section

**Description:**
After `CanonLift` collects the result Vals, it must invoke the
post-return callback (if configured in `Options.PostReturnIdx`). The
callback receives the same retptr (or flat result values) the
exporter wrote, and is responsible for freeing any guest-owned
list/string backing storage.

```go
// In CanonLift, after step 4 (result handling):
if opts.PostReturnIdx != nil {
    // Invoke the post-return callback with the same flat result
    // values that the exporter returned. The post-return callback
    // is a wasm function on the same instance.
    err := invokePostReturn(opts, results)
    if err != nil { return nil, fmt.Errorf("post-return: %w", err) }
}
```

The exact mechanism for invoking the post-return callback depends on
how `Options` carries the callback reference. Read `abi/context.go`
to confirm.

**Definition of done:**
- `CanonLift` invokes the post-return callback when configured
- A test confirms the callback is invoked once per call, with the
  correct arguments
- `go test ./internal/component/abi/...` passes

**Reviewer focus areas:**
- Spec compliance: confirm the post-return semantics match
  `definitions.py:3197+` (cite); confirm ownership transfer
- Code quality: confirm error wrapping; confirm no leaks if
  post-return fails

---

### Item 28: Add lower-side list size overflow check (`length * elemSize`)

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Lift side already has the check; lower side does not

**Files:**
- Modify: `internal/component/abi/lower.go` — add overflow check in
  `LowerFlat` and `LowerHeap` for list types
- Modify: `internal/component/abi/lower_test.go` — add a test that
  triggers the overflow

**Spec authorities:**
- `definitions.py` `lower_list` (search the file) — confirms the
  check
- Existing wazero check on the lift side at
  `internal/component/abi/lift.go` lines 316-320, 676-680 (reference
  for the pattern)

**Description:**
The lift side of `abi/` checks for `length * elemSize` overflow when
reading a list from memory and traps. The lower side does not. Add
the same check on the lower side: when writing a list of `length *
elemSize` bytes, if the multiplication overflows uint32, trap.

```go
// In LowerFlat / LowerHeap for list types:
totalSize := uint64(length) * uint64(elemSize)
if totalSize > math.MaxUint32 {
    return fmt.Errorf("list size overflow: length=%d elemSize=%d", length, elemSize)
}
```

**Definition of done:**
- Lower side traps on `length * elemSize` overflow
- A test triggers the overflow with a large length and asserts the
  trap
- `go test ./internal/component/abi/...` passes

**Reviewer focus areas:**
- Spec compliance: cite the spec for the overflow rule (or, if the
  spec is silent, cite wasmtime's equivalent check)
- Code quality: confirm the check uses uint64 for the multiplication
  to detect overflow

---

### Item 29: Add NaN scrambling on store under `DETERMINISTIC_PROFILE=false`, OR pin to `true` and document

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Per spec definitions.py, NaN canonicalization on lift is required; NaN scrambling on store is profile-dependent

**Files:**
- Modify: `internal/component/abi/lower.go` — add NaN scrambling on
  f32/f64 store, gated by the deterministic profile
- Modify: `internal/component/abi/lower_test.go` — add tests for both
  modes
- OR: Modify: `internal/component/abi/lower.go` to document that
  wazero pins `DeterministicProfile = true` and skips scrambling

**Spec authorities:**
- `definitions.py:2329` — `maybe_scramble_nan32`/`maybe_scramble_nan64`
- `definitions.py` (search for `DETERMINISTIC_PROFILE`)

**Description:**
The spec defines two profiles:
- `DETERMINISTIC_PROFILE=true`: NaN values are canonicalized on lift
  AND store
- `DETERMINISTIC_PROFILE=false`: NaN values are canonicalized on lift
  but on store, the implementation MAY scramble the bit pattern of
  any NaN to discourage code that depends on specific NaN bit
  representations

Wazero already canonicalizes on lift. For store, decide:
- **Option A:** Implement scrambling (random bit flips per call)
  gated by a build-time or runtime flag
- **Option B:** Pin to `DETERMINISTIC_PROFILE=true` (no scrambling),
  document the choice in code, and confirm with the spec authority
  that this is permitted

Wasmtime's behavior is the tiebreaker — read its NaN handling.

**Definition of done:**
- NaN scrambling is either implemented or explicitly skipped with a
  documented spec citation
- Tests confirm the chosen behavior
- `go test ./internal/component/abi/...` passes

**Reviewer focus areas:**
- Spec compliance: confirm the chosen profile is permitted by the
  spec; cite the line
- Code quality: confirm the decision is documented in code

---

### Item 30: Add canonical-options pre-flight validation

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Spec definitions.py canonopt validation rules

**Files:**
- Modify: `internal/component/abi/context.go` (or wherever `Options`
  is defined) — add a validate method
- Modify: `internal/component/abi/lift.go` and `lower.go` to call
  validate at the start of `CanonLift`/`CanonLower`
- Modify: `internal/component/abi/context_test.go` — add tests for
  validation failures

**Spec authorities:**
- `definitions.py` `canonopt` validation (search for `validate` near
  the canon_lift/canon_lower definitions)

**Description:**
The spec defines validation rules for canonical options:
- If a function signature contains strings or lists, `realloc` must
  be configured
- `memory` must be configured if any non-flat type is involved
- The string encoding must be a valid value
- `post_return` must reference a valid function on the same instance

Add a `func (o *Options) Validate(ft *types.FuncType) error` that
runs these checks. Call it at the start of `CanonLift` and
`CanonLower` so errors surface before any lift/lower work begins.

**Definition of done:**
- `Options.Validate` method exists and checks all spec-mandated
  conditions
- `CanonLift` and `CanonLower` call validate at entry
- Tests confirm validation traps for: missing realloc when strings
  needed, missing memory, invalid encoding
- `go test ./internal/component/abi/...` passes

**Reviewer focus areas:**
- Spec compliance: confirm every spec-mandated check is implemented;
  cite line
- Code quality: confirm validation runs ONCE per call, not on every
  type lift; confirm clear error messages

---

### Item 31: Add `FixedSizeList` lift/lower dispatch (or confirm List handles it)

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Depends on item 1's decision (separate type vs Length pointer)

**Files:**
- Modify: `internal/component/abi/lift.go` and
  `internal/component/abi/lower.go` if a separate `FixedSizeList`
  case is needed (per item 1)
- Modify: tests

**Spec authorities:**
- `definitions.py` for fixed-length list lift/lower

**Description:**
If item 1 chose Option A (use existing `List{Length *uint32}`),
verify that `LiftFlat` and friends correctly handle `Length != nil`
(fixed-length list). Add tests if missing.

If item 1 chose Option B (separate `FixedSizeList` type), add the new
type case to all four dispatch functions and tests.

**Definition of done:**
- Fixed-length lists lift and lower correctly
- Tests cover at least: fixed list of u8 (length 4), fixed list of
  string (length 2), fixed list of record (length 3)
- `go test ./internal/component/abi/...` passes

**Reviewer focus areas:**
- Spec compliance: cite `definitions.py` for fixed-length list
  semantics
- Code quality: confirm consistency with item 1's decision

---

## Phase 1.E — Wazero supplemental tests (2 items)

### Item 32: Add wazero-specific supplemental tests

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Each new case must cite the spec and not contradict any Python-ported case

**Files:**
- Create: `internal/component/conformance/canonical_abi/supplemental_test.go`

**Spec authorities:**
- Each test must cite the spec section it exercises

**Description:**
Add Go-specific test cases that the Python suite does not cover but
that are valuable for Go correctness:

1. **Float bit-exact equality**: confirm that lift+lower of
   `math.Float32frombits(0x4048f5c3)` round-trips to the same bit
   pattern. Cite `definitions.py` flat lift/lower for f32.
2. **Byte-slice aliasing across realloc**: lower a list, force a
   realloc by lowering a larger list, confirm the original buffer
   is not aliased.
3. **Deeply-nested record alignment**: a record with 10 levels of
   nesting, each level a record with mixed-alignment fields. Confirm
   alignment is correctly applied per spec `definitions.py:1607`
   `store_record`.
4. **List-of-string-of-record roundtrip**: composite stress test.
5. **Retptr boundary**: test functions with exactly 16 and 17 flat
   params; 16 should not spill, 17 should. Cite `definitions.py`
   `MAX_FLAT_PARAMS`.
6. **FixedSizeList edge cases**: zero-length, length-1, length-equal-
   to-MaxFlatParams.
7. Any other case the implementer thinks of, with citations.

Each test in `supplemental_test.go` must:
- Have a comment citing the spec section it exercises
- Not contradict any Python-ported test (if a contradiction is
  discovered, the spec wins; rewrite the supplemental test or drop it)
- Be marked clearly as "wazero-specific, not from Python suite"

**Definition of done:**
- `supplemental_test.go` exists with at least the 6 listed test
  categories plus any additions the implementer chooses
- Each test cites the spec
- `go test ./internal/component/conformance/canonical_abi/...` passes
- No supplemental test contradicts a Python-ported test

**Reviewer focus areas:**
- Spec compliance: confirm citations are accurate; confirm no
  contradiction with Python ports
- Code quality: confirm tests are valuable (not duplicates of Python
  ports); confirm citations

---

### Item 33: Reviewer subagent confirms supplemental tests do not contradict spec

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** -

**Files:**
- Read: `internal/component/conformance/canonical_abi/supplemental_test.go`
- Create: `docs/plans/projects/abi-unification/loop-1-supplemental-spec-review.md`

**Spec authorities:**
- All `definitions.py` and `CanonicalABI.md`

**Description:**
Dispatch a fresh spec-compliance reviewer subagent (using
`templates/review-spec-compliance.md`) with the scope set to
`supplemental_test.go`. The subagent re-reads each supplemental test
against the spec and confirms:
- Each test cites a real spec line
- The cited line actually says what the test asserts
- No supplemental test contradicts a Python-ported test
- Where a test exercises behavior the spec leaves underspecified, the
  test is marked accordingly with a comment

**Definition of done:**
- Subagent dispatched
- Findings recorded in `loop-1-supplemental-spec-review.md`
- Verdict is `PASS`

**Reviewer focus areas:**
- This IS the spec review

---

## Phase 1.F — Termination (2 items)

### Item 34: Verifier confirms 100% of ported Python tests pass, no skips

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** -

**Files:**
- Run: `go test -v ./internal/component/conformance/canonical_abi/...`
- Run: `go test ./internal/component/abi/...`

**Spec authorities:**
- N/A — verification

**Description:**
Run the conformance tests and the abi unit tests. Confirm:
- `go test -v ./internal/component/conformance/canonical_abi/...`
  passes 100%
- No `t.Skip` calls in `conformance/canonical_abi/`
- No `t.SkipNow` calls
- The test count is at least 272 (matching the design's count of
  ports plus supplementals)

If anything fails or skips, the responsible phase 1.C or 1.D item is
bounced.

**Definition of done:**
- Both test runs pass
- Zero skips
- Test count meets the floor

**Reviewer focus areas:**
- Code quality: confirm the verification was actually run (capture
  output)

---

### Item 35: Spec-coverage verifier produces loop-1-spec-coverage-report.md

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** -

**Files:**
- Read: `debug-vendored/component-model/design/mvp/CanonicalABI.md`
  end to end
- Read: `internal/component/abi/` source files
- Create: `docs/plans/projects/abi-unification/loop-1-spec-coverage-report.md`

**Spec authorities:**
- `CanonicalABI.md` — every section

**Description:**
Dispatch a fresh subagent to walk `CanonicalABI.md` section by
section. For each section, the subagent records whether wazero's
`abi/` package implements the algorithm:
- `implemented` — abi/ has a corresponding function; cite the file:line
- `deferred` — async-related; documented in the design's "Out of
  scope" section
- `N/A` — section is descriptive only, not an algorithm

Output:

```markdown
# Loop 1 Spec Coverage Report

## Summary
Total sections: <count>
Implemented: <count>
Deferred (async): <count>
N/A: <count>

## Section by section

### CanonicalABI.md "Loading"
Status: implemented
abi/ location: lift.go LiftHeap and friends

### CanonicalABI.md "Loading variants"
Status: implemented
abi/ location: lift.go variant case in LiftHeap

### CanonicalABI.md "Streams"
Status: deferred (async)
Reason: Loop 1 phase 1.A item 2 added trap; full impl in
docs/plans/abi-unification-async/

### ... (every section)
```

If any section is `implemented` but the cited code does not match the
spec, file a sub-item to fix it. If any section is `deferred` but is
not actually async-related, fix the design and the deferral.

**Definition of done:**
- `loop-1-spec-coverage-report.md` exists with every CanonicalABI.md
  section listed
- No section is left blank or marked "TBD"
- Any `implemented` claim has a real file:line cite
- Any `deferred` claim cites the design

**Reviewer focus areas:**
- Spec compliance: confirm the section count matches the actual
  CanonicalABI.md
- Code quality: confirm the report is complete (no missing sections)

---

## Loop 1 termination

When all 35 items are `status: done`, the driver runs
`templates/verify-loop-complete.md` with `{LOOP_NUMBER}=1`. The
verifier produces `loop-1-completion-report.md`. If `COMPLETE`, the
loop closes and Loop 2 opens. If `INCOMPLETE`, failing checks become
new items at the end of this backlog.
