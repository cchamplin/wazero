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

> Phase 1.A collapses **three** parallel type hierarchies into one. Read
> the design's "Architectural decisions" section before starting any
> item in this phase. The reference architecture is wasmtime's
> `ComponentTypes` and `go.bytecodealliance.org/wit`'s allocate-then-fill
> decoder.
>
> The three hierarchies are:
> 1. `internal/component/binary/types.go` — `binary.TypeDef`,
>    `RecordTypeDef`, `VariantTypeDef`, etc. Used only inside
>    `internal/component/binary/` as a parser scratchpad.
> 2. `internal/component/component.go` — `component.TypeDef`,
>    `component.RecordTypeDef`, `component.VariantTypeDef`,
>    `component.StreamTypeDef`, `component.FutureTypeDef`,
>    `component.FixedSizeListTypeDef`. The actual decoder output used
>    by `instance.go`, `TypeResolver`, and the four converters in
>    `component_linker.go`. (Note: there is no `ErrorContextTypeDef`
>    yet — it must be added.)
> 3. `internal/component/canon_lower.go` — `EnumType`, `FlagsType`,
>    `VariantType`, `VariantCaseForLower`, `PrimitiveType`, plus the
>    `PayloadType` interface (`canon_lower.go:15-62`). A third
>    representation used only by the dead `LoweredFunc.CallWithStack`
>    family.
>
> All three are deleted and replaced with `internal/component/types.ValType`.
> Phase 1.A also deletes the existing `CanonLower` constructor in
> `canon_lower.go` (item 9.5) because it returns the dead `*LoweredFunc`
> and would collide with the new `abi.CanonLower` from item 26.

### Item 1: Verify `types.List{Length *uint32}` covers fixed-size lists; add tests for both shapes

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Pre-decided: `definitions.py:122-125` `ListType(t, l=None)` matches wazero's existing `List{Element ValType, Length *uint32}` exactly. No new type needed.

**Files:**
- Read: `internal/component/types/composite.go` (existing `List` struct)
- Read: `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py:122-125`
- Modify: `internal/component/types/composite.go` — add a doc comment
  on `List` documenting that `Length != nil` represents the spec's
  `ListType(t, l=N)` fixed-length form
- Modify: `internal/component/types/composite_test.go` — add tests
  exercising both `Length == nil` (dynamic) and `Length != nil` (fixed)
  cases; assert `Size`/`Align`/`FlattenCount` values for each

**Spec authorities:**
- `definitions.py:122-125` — `ListType(t, l=None)` (the canonical form)
- `crates/wasmtime/src/runtime/component/values.rs` — wasmtime's
  `InterfaceType::List` dispatch, for cross-reference

**Description:**
Wazero already has `types.List{Element ValType, Length *uint32}` which
is structurally identical to the spec's `ListType(t, l=None)`. No new
type case is needed. This item just adds the documentation comment
and the tests that demonstrate both shapes work.

Loop 1 phase 1.D item 31 verifies the dispatch in `abi/lift.go` and
`abi/lower.go` handles both `Length == nil` and `Length != nil` correctly.

**Definition of done:**
- `types.List` has a doc comment citing `definitions.py:122-125` and
  explaining that `Length != nil` is the fixed-length form
- `composite_test.go` has tests for both shapes
- `go test ./internal/component/types/...` passes

**Reviewer focus areas:**
- Spec compliance: confirm the doc comment cites the right spec line
- Code quality: confirm the test covers both shapes

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
  asserting the new types exist and have `Align`/`Size`/`FlattenCount`
  of 4/4/1 (all three are i32 handles, same shape as Own/Borrow)
- Modify: `internal/component/abi/lift.go` — add `case types.Stream`,
  `case types.Future`, `case types.ErrorContext` in each of the four
  dispatch functions (`LiftFlat`, `LiftHeap`, `LowerFlat`, `LowerHeap`)
  that returns an error like `fmt.Errorf("async type %T not yet
  supported in synchronous canonical ABI", t)`
- Modify: `internal/component/abi/lift_test.go`,
  `internal/component/abi/lower_test.go` — add tests asserting the
  trap

**Spec authorities:**
- `definitions.py` — search for `class StreamType`, `class FutureType`,
  `class ErrorContextType` (the runtime treats all three as i32
  handles, same shape as Own/Borrow)

Note: `internal/component/component.go` already has
`StreamTypeDef` and `FutureTypeDef`, but does NOT have
`ErrorContextTypeDef` — that one must also be added in this item to
complete parity. The parser refactor in item 6 will populate the
`types` versions from the `component.*TypeDef` structs.

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
func (Stream) Align() uint32       { return 4 }  // i32 handle
func (Stream) Size() uint32        { return 4 }
func (Stream) FlattenCount() int   { return 1 }
```

(Same shape for `Future{Element ValType}` and `ErrorContext{}`. The
4/4/1 values are correct because all three flatten to a single i32
handle per spec.)

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
- **notes:** Removes the existing TODO comments at internal/component/types/resource.go lines 10-15 and 32-37

**Files:**
- Modify: `internal/component/types/resource.go` — change `Own` and
  `Borrow` struct field; remove the TODO comments at lines 10-15 and
  32-37 (verified line numbers)
- Modify: every existing reference to `types.Own{ResourceIdx: ...}`
  and `types.Borrow{ResourceIdx: ...}` in the codebase — migrate to
  `types.Own{Resource: ...}` (use Grep first to find them all)
- Modify: tests

**Spec authorities:**
- `definitions.py:1333-1339` — `lift_own(cx, i, t)` (verified line);
  `t.rt` is the `ResourceType`
- `definitions.py:1341-1347` — `lift_borrow(cx, i, t)`
- `definitions.py:1641` — `lower_own(cx, rep, t)`
- `definitions.py:1645` — `lower_borrow(cx, rep, t)`
- `crates/wasmtime/src/runtime/component/values.rs:115` — wasmtime
  takes `InterfaceType::Own(idx)` where `idx` resolves to a real
  `TypeResourceTable` joined to `ResourceType`

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
to it (use Grep) is migrated to use the `Resource *ResourceType`
pointer directly. **Note:** `types.ResourceType` does NOT currently
have an `Index` field (only `InstanceID`, `Destructor`, `DtorAsync`,
`DtorCallback`). If a caller genuinely needs an index for
serialization, that becomes a sub-decision in this item — either add
`Index uint32` to `ResourceType` (matching the deleted `ResourceIdx`),
or look up the index via the resource type table. Default: add
`Index uint32` to `ResourceType` since the parser already produces it.

The TODO comments at lines 10-15 and 32-37 in `resource.go` are
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

### Item 4: Preserve `Refines *uint32` field on `types.Case` (binary format defines it; runtime currently ignores it but the field belongs in the type)

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Restored after verification. The binary format defines refines; .wast test fixtures use it; wazero's parser already reads it. Delete-from-runtime-form would silently drop information the binary spec chose to encode.

**Files:**
- Read: `internal/component/binary/types.go:90` — confirm
  `VariantCase.Refines *uint32` exists at the binary layer (verified)
- Read: `internal/component/binary/types_composite_test.go:132`
  `TestDecodeVariantType_WithRefines` — confirms the parser already
  reads the field
- Read: `debug-vendored/component-model/test/wasm-tools/definedtypes.wast`
  lines 25, 26, 49, 56, 64, 73, 81 — confirm refines syntax in
  upstream fixtures with validation rules
- Modify: `internal/component/types/composite.go` — add `Refines *uint32`
  to `types.Case` (the type-system representation)
- Modify: every caller that constructs a `types.Case` to thread the
  refines value through (use Grep to find them)

**Spec authorities and verification trail:**
- `internal/component/binary/types.go:90` — wazero's binary parser
  reads `VariantCase.Refines *uint32`
- `debug-vendored/component-model/test/wasm-tools/definedtypes.wast`
  — upstream wasm-tools test fixtures use `(refines $x)` syntax with
  validation enforcement ("variant case cannot refine itself",
  "variant case can only refine a previously defined case")
- `definitions.py:141-143` `CaseType` does NOT have a refines field
  — confirms the canonical ABI **runtime** ignores refinement (it's
  not used during lift/lower)
- `Explainer.md`/`Binary.md`/`WIT.md`/`CanonicalABI.md` — none
  formally describe refines in their current form. The field is a
  binary-format artifact that pre-dates the current design docs and
  remains in wasm-tools.
- Wasmtime: zero references in `crates/wasmtime/src/`. Wasmtime
  drops the field on the floor.

**Description:**
The binary format encodes a `refines` slot per variant case. Wazero's
binary parser already reads it. The canonical ABI runtime
(`definitions.py`) does not use it for lift/lower. The current
design docs (Explainer/Binary/WIT/CanonicalABI) don't describe it.
Wasmtime drops it on the floor.

Two valid choices for wazero:
- **Drop on the floor** like wasmtime — simpler, but silently
  discards information the binary format chose to encode and breaks
  any future spec/tool that adds refines validation
- **Preserve on `types.Case`** — keeps the field round-trippable,
  costs one optional field per variant case, lets future code (e.g.
  a future spec validator, or the type checker) consume it

**Decision:** Preserve. The binary format defines it, upstream test
fixtures use it, and the cost of carrying an optional `*uint32`
through the type representation is negligible. Wazero should not
silently drop spec-encoded information.

This is a "preserved information" item, not a "runtime uses it" item.
The reviewer should NOT block on "no consumer uses Refines" — the
consumer is the binary format itself, which is round-tripped through
the type system. If a future spec validator or type checker is added,
it will read this field.

**Definition of done:**
- `types.Case` has `Refines *uint32` matching `binary.VariantCase.Refines`
- The parser refactor in item 6 populates this field
- A test confirms a variant defined with refines round-trips through
  the parser into a populated `types.Case.Refines` field
- `go test ./internal/component/binary/...` and
  `go test ./internal/component/types/...` pass

**Reviewer focus areas:**
- Spec compliance: confirm the field type is `*uint32` matching the
  binary format
- Code quality: confirm the field is exported with a doc comment
  explaining that it's a binary-format-preserved field that the
  current canonical ABI runtime does not consume but is preserved
  per the principle of not silently dropping spec-encoded
  information; confirm it does NOT count as "dead code" because
  the round-trip IS the consumer

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
// (in the binary parser, item 6) and consulted by every subsequent
// lift/lower operation. Spec authority: definitions.py alignment(t),
// elem_size(t), flatten_type(t). Wasmtime: TypeRecord.abi etc.
//
// Memory64 fields are intentionally omitted — wazero does not support
// memory64 in component-model contexts at this time.
type CanonicalAbiInfo struct {
    Size       uint32 // 32-bit memory size
    Align      uint32 // 32-bit alignment
    FlatCount  int    // number of core values in the flat representation
}
```

Add this as a field on each composite struct. **Item 6's parser
refactor MUST populate this cache when constructing each composite.**
Item 6 is updated to reference this requirement explicitly.

The existing `Align()`, `Size()`, `FlattenCount()` methods become thin
accessors that read from the cache.

**Demonstrated consumer (required to avoid dead code):** Update
`internal/component/abi/flatten.go` to read `FlatCount` from the
cache instead of recomputing via `flattenType` recursion. This
demonstrates the optimization is real.

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
- Read: every file under `internal/component/binary/` that constructs
  type values. Specifically: `internal/component/binary/decoder.go`,
  `internal/component/binary/binary.go`, `internal/component/binary/types.go`,
  `internal/component/binary/valtype.go`, `internal/component/binary/component_type.go`,
  `internal/component/binary/instance_type.go`, `internal/component/binary/core_type.go`.
  (Use `Grep` for `TypeDef{` and `RecordTypeDef{` etc. inside `binary/`
  to find every construction site.)
- Modify: every file in the above list to populate `types.ValType` 
  directly during decode (see specific guidance below)
- Modify: `internal/component/binary/decoder_test.go` and any
  companion `*_test.go` files in `binary/` — adjust to expect
  `types.ValType` output

**Spec authorities:**
- `debug-vendored/go-modules/wit/codec.go` lines around `getTypeDef`
  and `decodeTypeDef` — the allocate-then-fill pattern
- `internal/wasm/module.go` lines around `Module.TypeSection`
  population
- The component-model binary format spec at
  `debug-vendored/component-model/design/mvp/Binary.md` (if it
  exists; otherwise the relevant section of the WebAssembly spec)

**Description:**
Wazero currently has THREE parallel type hierarchies (see phase 1.A
preamble): `binary.*TypeDef` (parser scratchpad inside `binary/`),
`component.*TypeDef` (decoder output used by `instance.go`,
`TypeResolver`, and the four converters), and `canon_lower.*` types
(the dead `LoweredFunc` family). The converters in `component_linker.go`
(items 8-9) translate `component.TypeDef` into `types.ValType`. Items
6-9 collapse all three hierarchies into `types.ValType`.

This item refactors the parser to produce `types.ValType` directly via
allocate-then-fill. After this item lands, the parser still produces
`component.TypeDef` for compatibility (callers of `instance.go`, etc.,
still use it), but the `component.TypeDef` will ALSO carry a populated
`*types.ValType` field. Items 8-9 then migrate the consumers to read
from the `*types.ValType` directly, after which `component.TypeDef`
itself can be deleted.

(The cleaner alternative — replace `component.TypeDef` entirely in
this item — is too disruptive for one commit. Two-phase migration
keeps the build green between items 6 and 9.)

**Algorithm (allocate-then-fill):**

1. **First pass:** walk the binary type section. For each type, allocate
   a `*types.ValType` slot in a slice indexed by type-section index.
   Slots are zero-value pointers.

2. **Second pass:** walk the type section again. For each type, read
   its tag and contents from the binary, then construct the appropriate
   `types.ValType` (e.g., `types.Record{Fields: ...}`) and assign it to
   the slot. References to other types (e.g. record field type indices)
   become pointers into the slice. **Compute and populate
   `CanonicalAbiInfo` (from item 5) at the same time** so lift/lower
   never recomputes layout.

3. **Validation:** after filling, walk the slice and confirm every
   slot is non-nil and structurally valid.

**Note on variant-case refines field:** The binary format's variant
case has an optional `refines` field. Per item 4, the parser
refactor MUST populate `types.Case.Refines` from the binary
representation. The canonical ABI runtime does not consume the
field, but preserving it round-trips information the binary format
chose to encode (and which upstream wasm-tools fixtures use with
validation rules). See item 4 for the full rationale.

This pattern handles recursion naturally: `record { foo: list<self> }`
fills the list's element pointer with a pointer to the record's own
slot, which is allocated but not yet filled at the time the inner
list is parsed.

The reference is `debug-vendored/go-modules/wit/codec.go::getTypeDef`
which returns a `*TypeDef` that may be partially filled at the time
it is returned. Read this file in full before starting.

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

### Item 7: Delete the three parallel type hierarchies (binary, component, canon_lower)

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Depends on items 6, 8, 9. This item completes the type unification by deleting the now-orphaned hierarchies.

**Files:**
- Modify: `internal/component/binary/types.go` — delete `TypeDef`,
  `RecordTypeDef`, `VariantTypeDef`, `ListTypeDef`, `OptionTypeDef`,
  `ResultTypeDef`, `TupleTypeDef`, `FlagsTypeDef`, `EnumTypeDef`,
  `VariantCase` (the binary scratchpads)
- Modify: `internal/component/component.go` — delete `TypeDef`,
  `RecordTypeDef`, `VariantTypeDef`, `ListTypeDef`, `OptionTypeDef`,
  `ResultTypeDef`, `TupleTypeDef`, `FlagsTypeDef`, `EnumTypeDef`,
  `StreamTypeDef`, `FutureTypeDef`, `FixedSizeListTypeDef`,
  `NamedValType`, `ValTypeRef` (or whatever the local names are; use
  Grep to verify)
- Modify: `internal/component/canon_lower.go` — delete the local
  `EnumType`, `FlagsType`, `VariantType`, `VariantCaseForLower`,
  `PrimitiveType`, `PayloadType` interface (lines 15-62 verified by
  audit)
- Modify: any caller that referenced any deleted struct (Grep first)
- Modify: `internal/component/binary/types_test.go`,
  `internal/component/component_test.go` — delete tests of deleted structs

**Spec authorities:**
- N/A — this is a deletion item

**Description:**
After items 6, 8, and 9 the parser produces `types.ValType` directly
and all production consumers (instance.go, component_linker.go) read
from it. The three intermediate hierarchies have no production
callers. Delete them all in this item.

Use Grep to find every reference to each deleted name. Migrate any
remaining caller to `types.ValType` if it's production code; delete
the caller if it was a test of the deleted struct.

**Definition of done:**
- All three hierarchies are deleted (binary scratchpads, component
  decoder output, canon_lower locals)
- Grep returns zero for each deleted name
- `go test ./internal/component/binary/...` passes
- `go test ./internal/component/...` may have other failures from the
  lift/lower paths still being broken — that's expected and tracked
  in item 10

**Reviewer focus areas:**
- Spec compliance: N/A
- Code quality: confirm zero references remain for every deleted
  name; confirm no caller was missed; confirm the canon_lower.go
  local types were also deleted (audit found this hierarchy was
  forgotten in the original plan)

---

### Item 8: Delete the three duplicate type converters in `component_linker.go`

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Depends on item 6. The three converters are mutually recursive within component_linker.go; only resolveToValType has external callers.

**Files:**
- Modify: `internal/component/component_linker.go` — delete
  `resolveToValType` (line 722, verified), `typeDefToValType` (line
  748, verified), `valTypeRefToValType` (line 833, verified)
- Modify: the 4 callers of `resolveToValType` inside the same file
  (verified at lines 2120, 2125, 2308, 2313 by audit) — migrate to
  the parser-produced `types.ValType` per item 6
- Modify: `internal/component/component_linker_test.go` — delete
  tests of these functions

**Spec authorities:**
- N/A — deletion item

**Description:**
The three converters are mutually recursive within
`component_linker.go`: `resolveToValType` calls `typeDefToValType`
which calls `valTypeRefToValType` which calls back into
`typeDefToValType`. The only external entry point is `resolveToValType`,
which has 4 callers in `component_linker.go` itself (lines 2120, 2125,
2308, 2313). After items 6 and 7, the parser populates `types.ValType`
directly; the 4 callers can read from there without conversion.

`valTypeRefToValType` has an actively dangerous bug at line 882:
returns `types.U32{}` on lookup failure (silently turning a record
into an i32). This bug disappears with the deletion.

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

### Item 9: Delete `TypeResolver` (resolveDefinedType + per-shape helpers + ResolveValType)

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Depends on item 8. The 3 production callers in instance.go:198/301/440 call the public ResolveValType method (not the private resolveDefinedType directly).

**Files:**
- Modify: `internal/component/type_resolver.go` — delete the entire
  `TypeResolver` struct: public `ResolveValType` method, private
  `resolveDefinedType`, `resolveRecord`, `resolveVariant`,
  `resolveList`, `resolveOption`, `resolveResult`, `resolveTuple`,
  `resolveFlags`, `resolveEnum`, plus the cache and `withLocalTypes`
  helpers. Pre-decided: TypeResolver's only responsibility is type
  conversion; once that's gone, the struct has no remaining purpose.
- Modify: `internal/component/instance.go` lines 198, 301, 440 —
  these currently call `resolver.ResolveValType(typeRef)`. Migrate to
  read parser-produced types directly (via the populated
  `types.ValType` slot from item 6).
- Modify: `internal/component/type_resolver_test.go` — delete the file
  entirely (the struct is gone)

**Spec authorities:**
- N/A — deletion item

**Description:**
After items 6, 7, 8 the only remaining type converter is
`TypeResolver`. Its public entry point is `ResolveValType` (called
from instance.go:198, 301, 440). All three callers iterate over
`*NamedValType` (already populated by the parser) and currently call
`ResolveValType` to translate the embedded `ValTypeRef` into a
`types.ValType`. After item 6, the parser populates `types.ValType`
directly on the `NamedValType` (or wherever the canonical home is),
so the callers just read it.

`TypeResolver` has no responsibilities beyond type conversion: its
fields are a cache, an instance ref, and a localTypes ref. Once
`ResolveValType` is deleted, the struct is empty. Delete it entirely.

**Definition of done:**
- `TypeResolver` struct and all methods deleted
- `type_resolver.go` file removed (or reduced to a doc-only stub if
  the package needs the file for build hygiene; default: remove)
- `type_resolver_test.go` removed
- 3 production callers in instance.go migrated to read parser-produced
  types directly
- `go test ./internal/component/...` passes (or shows expected
  pre-existing failures)

**Reviewer focus areas:**
- Spec compliance: N/A
- Code quality: confirm the migration of `instance.go:198/301/440` is
  semantically equivalent (same types come out); confirm zero
  references to `TypeResolver` or `ResolveValType` remain repo-wide

---

### Item 9.5: Delete existing `CanonLower` and `LoweredFunc` family from canon_lower.go

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Resolves the name collision before item 26 adds the new abi.CanonLower. The existing CanonLower at canon_lower.go:98 returns *LoweredFunc which is the dead third lift/lower path.

**Files:**
- Modify: `internal/component/canon_lower.go` — delete the existing
  `CanonLower(callback HostFunc, funcType *FuncType, options *CanonicalOptions) *LoweredFunc`
  constructor at line 98 (verified). Delete `LoweredFunc` struct at
  line 67. Delete every private helper inside `canon_lower.go` that
  was only used by `LoweredFunc.CallWithStack`: `liftArguments`,
  `liftArgumentsTyped`, `liftValFromFlat`, `liftString`, `lowerResults`,
  `lowerResultsTyped`, `lowerValToFlatTyped`, `lowerString`,
  `lowerValToFlat`, `lowerEnumToFlat`, `lowerFlagsToFlat`,
  `lowerVariantToFlat`, the `flatIter` type and its methods.
- Modify: `internal/component/canon_lower_test.go` — delete the file
  entirely OR delete every test that references the deleted symbols
  (the file probably becomes empty)
- Modify: any caller of `CanonLower` or `LoweredFunc` outside
  `canon_lower.go` (Grep first; expected: zero, since this is dead
  code per the audit)

**Spec authorities:**
- N/A — deletion item

**Description:**
The audit confirmed that `internal/component/canon_lower.go` defines
its own `CanonLower` constructor (line 98) returning `*LoweredFunc`
(line 67), plus a third type hierarchy (`EnumType`/`FlagsType`/
`VariantType`/`PrimitiveType`/`PayloadType` interface — already
deleted by item 7) and a private lift/lower helper family that's
called only by `LoweredFunc.CallWithStack`. The whole file is part
of the dead canonical lower path that Loop 2 wires through `abi/`.

**This item must run before item 26** because item 26 adds a new
`CanonLower` in `abi/`. If both exist, Go has no name collision (they
are in different packages — `internal/component` vs.
`internal/component/abi`) but the existence of two functions with
identical names and totally different semantics is confusing and the
old one is dead code anyway.

**`LoweredFunc.CallWithStack` is NOT deleted in this item.** That's
deleted/rewritten by Loop 2 item 2. This item only deletes the
constructor, the lift/lower helpers, and the local type hierarchy.

**Pre-condition check:** before deleting `LoweredFunc`, run Grep to
confirm `canon_lower.go` itself is the only file that constructs
`*LoweredFunc`. If any other file constructs it, escalate — Loop 2
item 2's "wire LoweredFunc.CallWithStack" plan needs the struct to
still exist.

**Resolution:** keep `LoweredFunc` struct AND its `CallWithStack`
method (so Loop 2 item 2 can rewrite the body). Delete only the
helper functions and the `CanonLower` constructor. Item 26 of Loop 1
adds `abi.CanonLower` — different package, no collision.

**Definition of done:**
- `CanonLower` constructor in canon_lower.go is deleted
- The 14 private helper functions listed above are deleted
- `LoweredFunc` struct stays (Loop 2 item 2 needs it)
- `LoweredFunc.CallWithStack` stays (Loop 2 item 2 rewrites it)
- Grep returns zero for each deleted helper name
- `go test ./internal/component/...` may fail (expected — production
  paths still call into this code; Loop 2 wires them) but the deletion
  is complete

**Reviewer focus areas:**
- Spec compliance: N/A
- Code quality: confirm `LoweredFunc` and `CallWithStack` are NOT
  deleted (they're needed by Loop 2 item 2); confirm the deleted
  helpers had zero callers outside the deleted constructor

---

### Item 9.7: Resolve circular dependency between abi/ and component/ via shared package extraction

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** **CRITICAL ARCHITECTURAL FIX.** Today abi/lift.go, abi/lower.go, abi/context.go, abi/resource_lower.go all import `internal/component`. Loop 2 must make `internal/component` import `abi/`. This creates a circular dependency. Must be resolved before Loop 2 starts.

**Files:**
- Read: `internal/component/abi/lift.go:8`, `lower.go:9`, `context.go:8`,
  `resource_lower.go` — verify the four files importing `internal/component`
- Read: `internal/component/{val.go,resource_table.go,borrow_scope.go,call_context.go,subtask.go}` —
  source of `component.Val`, `component.ResourceTable`,
  `component.BorrowScope`, `component.CallContext`, `component.Subtask`
- Read: `debug-vendored/wasmtime/crates/wasmtime/src/runtime/vm/component/resources.rs`
  — wasmtime's low-level vm-resources separation reference
- Create: a new sub-package — pre-decided default name
  `internal/component/runtime/` — that holds the value types and
  resource state shared between `abi/` and the parent `component/`
- Modify: every `abi/*.go` (lift, lower, context, resource_lower) —
  change `import "github.com/tetratelabs/wazero/internal/component"`
  to `import "github.com/tetratelabs/wazero/internal/component/runtime"`,
  rename `component.Val` → `runtime.Val`, etc.
- Modify: `internal/component/{val.go,resource_table.go,borrow_scope.go,
  call_context.go,subtask.go}` — move the type definitions to the new
  `runtime/` sub-package, then re-export type aliases from the parent
  package for backward compatibility within `internal/component`
  (e.g. `type Val = runtime.Val`)
- Modify: every other `internal/component/*.go` file that uses these
  types — they continue to compile via the type aliases (no source
  changes needed except imports if any)

**Spec authorities:**
- N/A — this is a Go package boundary fix, not a spec change. Wasmtime
  reference is `vm/component/resources.rs` (low-level resource state)
  vs `runtime/component/values.rs` (Val + lift/lower) vs
  `runtime/component/func.rs` (orchestration). Three layers, with the
  arrows pointing UP only.

**Description:**
The current package layout:

```
internal/component/        (Val, ResourceTable, BorrowScope, CallContext,
                            Subtask, instance.go, linker.go, ...)
       |
       v   imports
internal/component/abi/    (lift.go imports component.Val etc.)
internal/component/types/  (ValType, Record, Variant, ...)
```

After Loop 2, the wiring direction must reverse for lift/lower:
`instance.go::ExportedFunc.Call` will call `abi.CanonLift`/
`abi.CanonLower`. This requires `internal/component` to import `abi/`,
which is impossible while `abi/` imports `internal/component`.

The fix is wasmtime's three-layer pattern. Move the shared
runtime-value types into a new sub-package `internal/component/runtime/`:

```
internal/component/runtime/   (Val, ResourceTable, BorrowScope,
                                CallContext, Subtask)
       ^                ^
       |                |
internal/component/abi/  internal/component/types/
       ^                ^
       |                |
internal/component/      (instance.go, linker.go — orchestration)
```

`abi/` imports `runtime/` and `types/`, never the parent.
`internal/component/` imports `abi/`, `runtime/`, `types/`.
No cycles.

**Migration plan within this item:**

1. Create `internal/component/runtime/` directory.
2. For each of `Val`, `ResourceTable`, `BorrowScope`, `CallContext`,
   `Subtask`: identify the file in `internal/component/` that defines
   it (likely `val.go`, `resource_table.go`, etc.), MOVE the type
   definition (and its methods, constructors, helpers) into a
   correspondingly-named file under `internal/component/runtime/`.
3. In `internal/component/`, replace each moved type with a one-line
   alias: `type Val = runtime.Val`, `type ResourceTable = runtime.ResourceTable`,
   etc. This keeps the rest of `internal/component/` (instance.go,
   linker.go, etc.) compiling without source changes.
4. In each `abi/*.go` file that previously imported
   `internal/component`, change the import path to
   `internal/component/runtime` and rename `component.X` → `runtime.X`
   throughout.
5. Run `go build ./internal/component/...` and confirm everything
   compiles.
6. Run `go test ./internal/component/abi/...` and confirm the abi
   tests still pass (they should be unaffected by the move).
7. Run `go test ./internal/component/runtime/...` (the moved tests).

**This item must complete before Loop 2 starts.** It is in phase 1.A
because it's foundational to the package architecture; everything in
phase 1.D (new abi/ entry points) and Loop 2 (wiring) depends on it.

**Definition of done:**
- `internal/component/runtime/` exists with `Val`, `ResourceTable`,
  `BorrowScope`, `CallContext`, `Subtask` and their methods
- `internal/component/` retains compatibility type aliases
- `abi/*.go` imports `internal/component/runtime` (NOT
  `internal/component`)
- Grep for `\"github.com/tetratelabs/wazero/internal/component\"` in
  `internal/component/abi/*.go` returns ZERO matches
- `go build ./internal/component/...` succeeds
- `go test ./internal/component/abi/...` passes
- `go test ./internal/component/runtime/...` passes
- All other tests still build (other packages use the type aliases)

**Reviewer focus areas:**
- Spec compliance: confirm the package layering matches wasmtime's
  three-layer pattern (cite `vm/component/resources.rs` and
  `runtime/component/func/options.rs`)
- Code quality: confirm zero `internal/component"` imports in
  `abi/*.go`; confirm the type aliases are minimal (one line each);
  confirm no functionality moved that shouldn't have (e.g.,
  instance.go-specific orchestration must NOT move into runtime/)

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

### Item 10.5: Verify the binary parser computes flatten counts correctly for retptr-triggering types

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Anti-bug-hiding check. The instance.go retptr synthesis hack at 335-338 may have been masking a parser bug where FlattenCount returned 1 for types that should return >1 (causing the retptr branch to never trigger). Loop 2 deletes the hack; this item verifies the parser produces correct flatten counts so the retptr branch DOES trigger when needed.

**Files:**
- Read: `internal/component/binary/` parser files (the ones modified
  in item 6)
- Read: `definitions.py` `flatten_type` function (search for `def flatten_type`)
- Read: `crates/wasmtime/src/runtime/component/values.rs` and
  `crates/wasmtime-environ/src/component/types.rs` —
  `CanonicalAbiInfo.flat_count` computation
- Add: `internal/component/conformance/canonical_abi/parser_flatten_test.go`
  — table-driven test that constructs every value type via the parser
  AND directly, then asserts both produce the same `FlattenCount`

**Spec authorities:**
- `definitions.py` `flatten_type(t)` and `flatten_record/variant/option/result/tuple/flags`
  helpers — the canonical computation
- `crates/wasmtime/src/runtime/environ/component/types.rs` —
  `CanonicalAbiInfo.flat_count` (precomputed at parse time)

**Description:**
The audit found an unverified concern: if the binary parser's
`FlattenCount` is wrong for any composite type (e.g., always returns
1 because of a bug), the retptr branch in lifters and lowers would
never trigger — and the bug would be masked by the synthesis hack at
instance.go:335-338. Loop 2 item 5 deletes that hack; if the parser
bug exists, deleting the hack will expose it as test failures in
Loop 2 (good — the bug surfaces). But it's better to verify and
fix the parser before Loop 2 starts.

This item:
1. Walks every concrete `types.ValType` case (Bool, S8/U8, ..., String,
   List, FixedSizeList, Record, Tuple, Variant, Enum, Option, Result,
   Flags, Own, Borrow, Stream, Future, ErrorContext)
2. For each, constructs the type two ways: (a) via the binary parser
   on a hand-crafted minimal binary that defines the type, (b) directly
   in Go by allocating `types.X{...}`
3. Computes `FlattenCount` (and `Size`, `Align`) for both
4. Asserts they match
5. Additionally: builds a record-of-3-i32 (which should have flatten
   count 3, triggering the retptr branch on the lower side and the
   memory-read branch on the lift side) and confirms both methods
   agree on count=3
6. Asserts against `definitions.py` directly: for each type, manually
   compute `flatten_type` per the spec algorithm and compare

If any mismatch is found, fix it in the binary parser (item 6 may
need a follow-up commit).

**Definition of done:**
- `parser_flatten_test.go` exists with at least one test row per
  concrete value type
- All rows pass
- A specific row exercises a record with 3 i32 fields and confirms
  flatten count is 3 (not 1 — the bug-masking case)
- `go test ./internal/component/conformance/canonical_abi/...` passes
  for the new test file

**Reviewer focus areas:**
- Spec compliance: confirm every flatten count matches `definitions.py`
  `flatten_type` (cite the specific case)
- Code quality: confirm the test exhaustively covers all types; confirm
  no synthesis hacks were re-introduced

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
references. **Note:** `internal/component/abi/context.go:13-29`
already defines `MaxFlatParams`, `MaxFlatResults`,
`CanonicalFloat32NaN`, `CanonicalFloat64NaN`. The conformance package
does NOT redefine them — it imports from `abi/`. Only constants that
are not already in `abi/` are defined here:

```go
package canonical_abi

import "github.com/tetratelabs/wazero/internal/component/abi"

// PythonReferenceSHA records the upstream commit of run_tests.py at
// the time these tests were ported. Update when re-syncing.
const PythonReferenceSHA = "<sha>"

// Constants imported from internal/component/abi (single source of truth):
//   abi.MaxFlatParams      = 16    (definitions.py:142, MAX_FLAT_PARAMS)
//   abi.MaxFlatResults     = 1     (definitions.py:143, MAX_FLAT_RESULTS)
//   abi.CanonicalFloat32NaN = 0x7FC00000   (definitions.py canonical NaN)
//   abi.CanonicalFloat64NaN = 0x7FF8000000000000

// Constants only defined in the conformance package (not yet in abi/):
const (
    Utf16Tag             = 1 << 31     // definitions.py UTF16_TAG
    // DeterministicProfile is the spec default per definitions.py:1209
    // (DETERMINISTIC_PROFILE = False). Wazero pins it to true for
    // testability — see item 29 for the rationale and citation.
    DeterministicProfile = true
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
struct and runs the assertions. Concrete signatures (using the
post-item-9.7 `runtime` package and the actual wazero flat
representation `[]uint64`):

```go
// runTest ports run_tests.py:105 (test() helper). It asserts
// (a) lift_flat(vi, t) equals expected value, and (b) lower-then-
// re-lift through a fresh heap is stable.
func runTest(t *testing.T, valType types.ValType, flatVals []uint64, expected runtime.Val) {
    t.Helper()
    // 1. Build a LiftContext with a fresh memory + canonical options
    // 2. Lift via abi.LiftValues (item 25)
    // 3. Assert equal to expected
    // 4. Round-trip: lower + re-lift, assert stable
}

// runTestPairs ports run_tests.py:180 (test_pairs() helper). Each
// row is (input, expected) for primitive coercion.
func runTestPairs[In, Out any](t *testing.T, valType types.ValType, pairs []struct{ In In; Out Out }) {
    t.Helper()
    // For each row: lift_flat then assert equality
}

// runTestHeap ports run_tests.py:284 (test_heap()).
// (t, expected, ptrLenArgs, memoryBytes) — byte-level golden test.
func runTestHeap(t *testing.T, valType types.ValType, expected runtime.Val, ptrLen []uint64, memBytes []byte) {
    t.Helper()
}

// runTestFlatten ports run_tests.py:372 (test_flatten()).
func runTestFlatten(t *testing.T, ft types.FuncType, expectedParams []string, expectedResults []string) {
    t.Helper()
}

// runTestNan32 ports run_tests.py:203 (test_nan32()).
func runTestNan32(t *testing.T, inputBits, expectedBits uint32) {
    t.Helper()
}

// runTestNan64 ports run_tests.py:217 (test_nan64()).
func runTestNan64(t *testing.T, inputBits, expectedBits uint64) {
    t.Helper()
}

// runTestStringEncoding ports run_tests.py:253 (test_string()).
func runTestStringEncoding(t *testing.T, s string, src, dst abi.StringEncoding) {
    t.Helper()
}

// runTestRoundtrip ports run_tests.py:399 (test_roundtrip()).
func runTestRoundtrip(t *testing.T, valType types.ValType, args []runtime.Val) {
    t.Helper()
}

// runTestReentrance ports run_tests.py:2765 (test_reentrance()).
// Uses internal/component.ReentranceTracker.CallMightBeRecursive,
// not abi/.
func runTestReentrance(t *testing.T, ...) { t.Helper() }
```

(Read `abi/context.go` for the actual `LiftContext`/`LowerContext`
shapes, `abi/strings.go` for `StringEncoding`, and
`internal/component/types/composite.go` for `types.FuncType` (after
item 6 populates it). All helpers MUST use the post-item-9.7
`runtime` package, not the parent `component` package.)

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

### Item 15: Port primitive coercion tests (test_pairs, 58 cases) → primitives_test.go

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Depends on item 13

**Files:**
- Create: `internal/component/conformance/canonical_abi/primitives_test.go`

**Spec authorities:**
- `run_tests.py:180-202` (`test_pairs` definition and the 14 calls
  with their 58 expanded sub-cases — verified by counting:
  Bool 4 + U8 7 + S8 7 + U16 7 + S16 7 + U32 3 + S32 3 + U64 3 +
  S64 3 + F32 1 + F64 1 + Char 5 + Char 4 + Enum 3 = 58)

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

### Item 16: Port lift/lower roundtrip composites tests (test, 20 cases) → composites_test.go

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** -

**Files:**
- Create: `internal/component/conformance/canonical_abi/composites_test.go`

**Spec authorities:**
- `run_tests.py:105-179` (`test` helper and the 20 direct invocations
  for record/tuple/list/flags/variant/option/result — verified count
  by grep)

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
- All 20 `test()` invocations are ported as table rows or t.Run
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

### Item 22: Port test_handles resource scenario (~36 asserts) → handles_test.go

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Most complex single port

**Files:**
- Create: `internal/component/conformance/canonical_abi/handles_test.go`

**Spec authorities:**
- `run_tests.py:441-551` (`test_handles` scenario; ~36 asserts
  verified by counting `assert(` lines in this range)

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

### Item 23: Port test_reentrance (13 asserts) → reentrance_test.go

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Pure boolean test on call_might_be_recursive

**Files:**
- Create: `internal/component/conformance/canonical_abi/reentrance_test.go`

**Spec authorities:**
- `run_tests.py:2765-2802` (`test_reentrance` scenario; 13 asserts
  verified by count)

**Description:**
Port `test_reentrance`. 13 boolean asserts on
`call_might_be_recursive(task, inst)` for various task chains. The
test has no I/O, no memory — just chain-construction and assertions.

The function `CallMightBeRecursive` is implemented as a method of
`ReentranceTracker` at `internal/component/reentrance.go:44` (NOT in
`abi/`). The Go test imports it from there.

**Definition of done:**
- All 13 asserts ported as table rows or t.Run blocks
- Each cites the source line
- The test file compiles cleanly
- Tests pass — `CallMightBeRecursive` already exists at
  `internal/component/reentrance.go:44`

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
- **notes:** Depends on Loop 1 phase 1.A item 3 (Own carries *ResourceType) and item 9.7 (package boundary). Extends the EXISTING LiftOwn/LiftBorrow helpers to take *ResourceType, then folds them into the dispatch.

**Files:**
- Modify: `internal/component/abi/lift.go` — extend the existing
  `LiftOwn`/`LiftBorrow` (lines 708, 790) to take a `*types.ResourceType`
  parameter for spec-correct trap-on-mismatch; then add `case types.Own:`
  and `case types.Borrow:` to `LiftFlat` and `LiftHeap` that call them
- Modify: `internal/component/abi/lower.go` — same for
  `LowerOwn`/`LowerBorrow` (lines 654, 670) and `LowerFlat`/`LowerHeap`
- Modify: `internal/component/abi/lift_test.go`,
  `internal/component/abi/lower_test.go` — add tests for the
  integrated dispatch (record-of-own, list-of-borrow, etc.) AND
  for the new ResourceType validation trap
- Modify: callers of the existing `LiftOwn`/`LiftBorrow`/`LowerOwn`/
  `LowerBorrow` (use Grep) — pass the new `*ResourceType` argument

**Spec authorities:**
- `definitions.py:1197-1198` — `load(cx, ptr, t)` `case OwnType()`,
  `case BorrowType()` (verified)
- `definitions.py:1387-1388` — `store(cx, v, t, ptr)` symmetric
- `definitions.py:1792-1793` — `lift_flat()` Own/Borrow case
- `definitions.py:1886-1887` — `lower_flat()` Own/Borrow case
- `definitions.py:1333-1339` — `lift_own(cx, i, t)` definition; the
  trap `trap_if(h.rt is not t.rt)` is at line 1336 (verified)
- `definitions.py:1341-1347` — `lift_borrow(cx, i, t)`; the trap is at
  line 1345 (verified)
- `crates/wasmtime/src/runtime/component/values.rs:115` — wasmtime's
  matched case

**Description:**
Currently `abi/`'s four dispatch functions do not handle `types.Own`
or `types.Borrow` — they fall through to the default error branch.
The standalone `LiftOwn`/`LiftBorrow` helpers (at `lift.go:708, 790`)
already exist but: (a) they don't take a `*ResourceType` so they
can't validate the trap at `definitions.py:1336`, and (b) they're
not called from inside the dispatch.

This item:
1. **Extends the existing helpers** to take `*types.ResourceType`
   (matching `definitions.py:1333-1347`):

```go
// LiftOwn lifts an own<T> handle, validating the resource type per
// definitions.py:1336. Pre-item-9.7 this took only handleIdx; now it
// takes the resource type for trap-on-mismatch.
func (cx *LiftContext) LiftOwn(handleIdx uint32, t *runtime.ResourceType) (runtime.Val, error) {
    h, ok := cx.ResourceTable.Get(handleIdx)
    if !ok {
        return runtime.Val{}, fmt.Errorf("invalid handle %d", handleIdx)
    }
    if h.ResourceType != t {  // definitions.py:1336 trap_if(h.rt is not t.rt)
        return runtime.Val{}, fmt.Errorf("handle resource type mismatch")
    }
    if h.NumLends != 0 {  // definitions.py:1337
        return runtime.Val{}, fmt.Errorf("cannot lift_own a handle with active borrows")
    }
    if !h.Owned {  // definitions.py:1338
        return runtime.Val{}, fmt.Errorf("cannot lift_own a borrowed handle")
    }
    cx.ResourceTable.Remove(handleIdx)  // definitions.py:1334 (table.remove(i))
    return runtime.ValOwn(h.Rep), nil
}
```

2. **Adds the integrated dispatch** in the four type switches:

```go
// In LiftFlat:
case types.Own:
    handleIdx := iter.NextI32()
    return cx.LiftOwn(uint32(handleIdx), t.Resource)
case types.Borrow:
    handleIdx := iter.NextI32()
    return cx.LiftBorrow(uint32(handleIdx), t.Resource)

// Similar for LiftHeap (read 4 bytes from memory), LowerFlat,
// LowerHeap.
```

The standalone `LiftOwn`/`LiftBorrow`/`LowerOwn`/`LowerBorrow` now
exist in their EXTENDED form (with `*ResourceType`) and are
internal-only. Loop 2 phase 2.D item 6 deletes them as standalones —
their callers will be the dispatch cases above, which is fine
because they're simply being inlined.

**Note:** Existing callers of `LiftOwn`/`LiftBorrow` etc. (find via
Grep) need their call sites updated to pass the new `*ResourceType`
parameter. Since `*ResourceType` is now on `types.Own.Resource` (per
item 3), most callers can pass `t.Resource`.

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
  confirm the trap matches `definitions.py:1336` (`trap_if(h.rt is not t.rt)`)
- Code quality: confirm the existing helpers were extended (not
  duplicated); confirm idiomatic Go; confirm no error suppression;
  confirm `runtime.Val`/`runtime.ResourceType` (post-item-9.7) not
  `component.Val`/`component.ResourceType`

---

### Item 25: Add `CanonLift`/`CanonLower` lift-only entry points (pure math, no lifecycle)

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Depends on item 9.7 (package boundary). NOT a full canon_lift wrapper — that lives in instance.go per the wasmtime layering. abi/ stays pure math.

**Files:**
- Modify: `internal/component/abi/lift.go` (or create
  `internal/component/abi/canon_lift.go`) — add lift entry points
- Modify: `internal/component/abi/lift_test.go` — add tests for
  param spill (>16 flat) and retptr result spill

**Spec authorities:**
- `definitions.py:1978-2063` — `canon_lift(opts, inst, ft, callee, ...)`
  (verified line; the spec function combines lift+invoke+lower in one
  Python function — wazero splits this between `abi/` math and
  `instance.go` lifecycle per the wasmtime layering)
- `definitions.py:1943` — `lift_flat_values(cx, max_flat, vi, ts)`
  (param spill helper; this is the pure-math part wazero needs)
- `definitions.py:1954` — `lower_flat_values(cx, max_flat, vs, ts, out_param)`
- `crates/wasmtime/src/runtime/component/values.rs:97-218` —
  `Val::lift` pure-math reference (NOT `func.rs::call_raw` which is
  the lifecycle wrapper; that's item 5 of Loop 2)
- `crates/wasmtime/src/runtime/component/func/typed.rs` — `Lift`/`Lower`
  trait reference

**Architectural decision (per wasmtime layering research):**
`abi/` is pure math, mirroring wasmtime's `values.rs`. It does NOT
know about subtasks, borrow scope, may_leave, may_enter, post_return,
reentrance, or enter_call/exit_call. Those live in
`instance.go::ExportedFunc.Call` (the wasmtime `func.rs::call_raw`
analogue) — that's Loop 2 item 5. abi/ provides the math primitives
that the wrapper calls.

**Description:**
Today `abi/` has `LiftFlat`, `LiftHeap`, `LowerFlat`, `LowerHeap` as
leaf operations on individual values. What's missing are the
**multi-value spill helpers** that handle param spill (when
`len(flat) > MaxFlatParams`) and result spill (retptr) for an entire
parameter or result list at once. These are direct ports of
`lift_flat_values` and `lower_flat_values` from `definitions.py`.

Add two pure-math entry points (signatures use the post-item-9.7
`runtime` package, not the parent `component`):

```go
// LiftValues implements lift_flat_values from definitions.py:1943.
// It lifts a list of parameter or result values from the wasm flat
// representation, spilling to memory via the retptr if the flat
// representation would exceed maxFlat.
//
// cx: lift context (memory, options, resource tables — no lifecycle)
// maxFlat: MaxFlatParams (16) for params, MaxFlatResults (1) for results
// flat: the wasm core stack values (post-item-9.7: []uint64, the
//   actual wazero core stack representation)
// types: the component-level types of each value to lift
//
// Returns the lifted runtime.Val list, or an error. Does NOT touch
// borrow scope, may_leave, post_return, or any lifecycle state.
func LiftValues(cx *LiftContext, maxFlat int, flat []uint64, types []types.ValType) ([]runtime.Val, error)

// LowerValues implements lower_flat_values from definitions.py:1954.
// Mirror of LiftValues for the lower direction.
func LowerValues(cx *LowerContext, maxFlat int, vs []runtime.Val, types []types.ValType, outParam *uint32) ([]uint64, error)
```

(Exact signature confirmed against the current `abi/` API: `LiftContext`/
`LowerContext` already exist in `abi/context.go`; `[]uint64` is the
actual wazero flat representation per `abi/lift.go:14` `FlatIter`;
`runtime.Val` is the post-item-9.7 import path for what is currently
`component.Val`.)

The retptr handling matches `lift_flat_values`/`lower_flat_values` in
`definitions.py:1943-1977`: if the flat representation would exceed
`maxFlat`, the actual values are read from / written to memory at
the address held in the retptr slot, and the flat slot just holds
the pointer.

Post-return is NOT part of these entry points — it's a lifecycle
concern, handled by `instance.go::ExportedFunc.Call` after `LiftValues`
returns the result Vals (Loop 2 item 5).

**Rename note:** the design and prior plan revisions called this
`CanonLift`/`CanonLower` after the spec's `canon_lift`/`canon_lower`
functions. Those spec functions are wrappers that include the lifecycle
work (subtask creation, post_return, etc.). To avoid implying that the
wazero `abi/` versions also do lifecycle, the wazero functions are
named `LiftValues`/`LowerValues` after `lift_flat_values`/
`lower_flat_values` — which is exactly what they implement.

**Definition of done:**
- `LiftValues` exists and matches `definitions.py:1943` `lift_flat_values`
- `LowerValues` exists and matches `definitions.py:1954` `lower_flat_values`
- Param spill works for >16 flat params (test with 17, 32, 100)
- Result spill works for >1 flat result (test with 2 results, 16
  results)
- Tests from phase 1.C items 21 (test_roundtrips) start passing for
  the math portions (the lifecycle-dependent portions still need
  Loop 2 item 5)
- `go test ./internal/component/abi/...` passes
- Neither function references subtask, borrow scope, may_leave,
  post_return, or any other lifecycle state — verified by Grep

**Reviewer focus areas:**
- Spec compliance: confirm the implementations match
  `definitions.py:1943-1977` line by line; cite each step
- Code quality: confirm pure-math (no lifecycle); confirm idiomatic
  Go; confirm error wrapping; confirm `runtime.Val` (post-item-9.7)
  not `component.Val`

---

### Item 26: Verify `LiftValues`/`LowerValues` integration with existing leaf operations

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Item 25 added LiftValues and LowerValues together. This item is the integration verification — confirms both functions correctly compose with existing LiftFlat/LiftHeap/LowerFlat/LowerHeap leaf operations and the FlatIter abstraction.

**Files:**
- Read: `internal/component/abi/flatten.go` — existing
  `FlattenParams`, `FlattenResults`, `CoreSignature`, `flattenType`,
  `FlatIter` (verify how the leaf operations are currently composed)
- Modify (only if integration tests reveal a gap):
  `internal/component/abi/lift.go`, `lower.go`, `flatten.go`
- Add: `internal/component/abi/canon_values_test.go` — integration
  tests that exercise `LiftValues`/`LowerValues` against composite
  type lists with mixed flat/heap-spilled values

**Spec authorities:**
- `definitions.py:1943-1977` — `lift_flat_values`/`lower_flat_values`
- `crates/wasmtime/src/runtime/component/values.rs:97-218` — wasmtime
  reference for how leaf operations compose into multi-value lifts

**Description:**
Item 25 added the multi-value spill helpers `LiftValues` and
`LowerValues`. They internally call the leaf operations (`LiftFlat`,
`LiftHeap`, `LowerFlat`, `LowerHeap`) for each individual value.
This item is the integration verification — write tests that exercise
the composition under conditions the leaf-operation tests don't cover:

1. **Boundary at 16/17 flat params:** call with exactly 16 flat
   params (no spill), then exactly 17 (spill). Confirm the spill
   pointer is read/written correctly and the resulting Vals match.
2. **Mixed compositions:** lift/lower a parameter list of
   `[i32, string, list<u8>, record{f1: u8, f2: i64}]`. Verify each
   leaf operation is called in order with the right offsets.
3. **Result spill:** lower a result list with 2 results (must spill
   since `MaxFlatResults=1`). Verify the retptr is allocated via
   realloc and the heap layout is correct.
4. **Round-trip:** for each composite type defined in
   `composite_test.go`, do `LowerValues` then `LiftValues` and
   confirm the result equals the input.

If any test reveals a bug in the leaf operations or in the
`LiftValues`/`LowerValues` composition, fix it in this item.

**Definition of done:**
- `canon_values_test.go` exists with the four test categories above
- All tests pass
- Round-trip property holds for every composite type
- `go test ./internal/component/abi/...` passes

**Reviewer focus areas:**
- Spec compliance: confirm round-trip semantics match
  `definitions.py` (lower then lift is identity for spec-supported
  types)
- Code quality: confirm tests use real composite types (not
  hand-rolled mocks); confirm coverage of all flat/heap branches

---

### Item 27: Document post-return contract in `Options`; do NOT invoke from abi/

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Per the wasmtime layering decision: post-return invocation is a lifecycle concern that lives in instance.go (Loop 2 item 5), NOT in abi/. abi/ stays pure math. This item only documents the contract.

**Files:**
- Modify: `internal/component/abi/context.go` — update the doc comment
  on `Options.PostReturnIdx` to document that post-return invocation
  is the CALLER's responsibility (i.e., `instance.go::ExportedFunc.Call`),
  not abi/'s
- Modify: `internal/component/abi/lift_test.go` — add a test
  confirming `LiftValues` does NOT invoke post-return (negative
  assertion: configure a post-return callback that increments a
  counter, call `LiftValues`, assert counter is still zero)

**Spec authorities:**
- `definitions.py:1978-2063` — `canon_lift` Python function. The
  Python version invokes post-return inline because Python conflates
  math and lifecycle in one function. Wazero splits them per the
  wasmtime layering: post-return invocation lives in the
  `func.rs::call_raw` analogue (Loop 2 item 5), not in `values.rs`
  (abi/'s analogue).
- `crates/wasmtime/src/runtime/component/func.rs::Func::post_return_impl`
  — the wasmtime analogue. It is in `func.rs`, NOT in `values.rs`.
  Confirms that post-return is a wrapper/lifecycle concern.

**Description:**
The original plan had `CanonLift` invoke post-return. Per the
wasmtime layering research, post-return is a lifecycle concern that
belongs in the `func.rs::call_raw` analogue — for wazero, that's
`instance.go::ExportedFunc.Call` (Loop 2 item 5).

This item is therefore a documentation-only change in `abi/`:
1. Update `Options.PostReturnIdx`'s doc comment to make it explicit
   that this field is read by the CALLER (i.e., the wrapper in
   `instance.go`), not by `abi/` itself.
2. Add a negative test that confirms `LiftValues` does NOT invoke
   post-return — to catch regressions if a future change adds the
   invocation back into abi/.

The actual post-return invocation logic is in Loop 2 item 5 (the
instance.go orchestration shim). Loop 2 item 5's description has been
updated to include post-return as one of the lifecycle steps it owns.

**Definition of done:**
- `Options.PostReturnIdx` doc comment makes clear that post-return
  invocation is the caller's responsibility
- A negative test confirms `LiftValues` does NOT invoke post-return
- `go test ./internal/component/abi/...` passes
- `Grep "PostReturnIdx" internal/component/abi/` shows the field is
  read only by callers, not invoked from inside `abi/`

**Reviewer focus areas:**
- Spec compliance: confirm post-return is not invoked from `abi/`
  (cite the wasmtime layering: `func.rs::post_return_impl`, NOT
  `values.rs`)
- Code quality: confirm the doc comment is clear; confirm the
  negative test would catch a regression

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
- `definitions.py:1594-1601` — `store_list_into_range`. The trap is
  at `definitions.py:1596`: `trap_if(byte_length >= (1 << 32))`.
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

### Item 29: Implement spec-correct NaN canonicalization on lift AND maybe_scramble hook on lower; pin DETERMINISTIC_PROFILE=true

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Restored after verification. The spec defines canonicalize_nan on lift and maybe_scramble_nan on lower, both called from lift_flat/lower_flat at definitions.py:1783-1784/1877-1878. Under DETERMINISTIC_PROFILE=true, maybe_scramble_nan is a no-op pass-through, but the call site is part of the spec.

**Files:**
- Read: `internal/component/abi/context.go:32-44` — existing
  `canonicalizeNaN32`/`canonicalizeNaN64` functions
- Read: `internal/component/abi/lift.go:73-75` — existing canonicalize
  call from lift path
- Modify: `internal/component/abi/context.go` — add
  `maybeScrambleNaN32`/`maybeScrambleNaN64` matching
  `definitions.py:1395-1414` (under DETERMINISTIC_PROFILE=true these
  are no-op pass-throughs; the function exists for spec literal
  correspondence)
- Modify: `internal/component/abi/lower.go` — call
  `maybeScrambleNaN32`/`maybeScrambleNaN64` in `LowerFlat`/`LowerHeap`
  for f32/f64 cases, matching `definitions.py:1877-1878`
- Modify: `internal/component/abi/lower_test.go` — add tests for the
  call sites; under deterministic profile assert that input bit
  patterns pass through unchanged (functional no-op verification)
- Modify: `internal/component/abi/context.go` — add a doc comment on
  the deterministic profile choice

**Spec authorities (verified):**
- `definitions.py:1209` — `DETERMINISTIC_PROFILE = False  # or True`
  (spec leaves it to the implementation)
- `definitions.py:1213-1217` — `def canonicalize_nan32(f)` (lift side,
  always-on, not profile-dependent)
- `definitions.py:1219-1223` — `def canonicalize_nan64(f)`
- `definitions.py:1226-1229` — used in `core_f32_reinterpret_i32`
  and `core_f64_reinterpret_i64`
- `definitions.py:1395-1402` — `def maybe_scramble_nan32(f)`:
  under `DETERMINISTIC_PROFILE` returns `f` unchanged; otherwise
  scrambles
- `definitions.py:1404-1411` — `def maybe_scramble_nan64(f)`
- `definitions.py:1421` — `core_i32_reinterpret_f32(maybe_scramble_nan32(f))`
- `definitions.py:1424` — `core_i64_reinterpret_f64(maybe_scramble_nan64(f))`
- `definitions.py:1783-1784` — `lift_flat` for `F32Type()`/`F64Type()`
  calls `canonicalize_nan32/64(vi.next('f32'/'f64'))`
- `definitions.py:1877-1878` — `lower_flat` for `F32Type()`/`F64Type()`
  returns `[maybe_scramble_nan32/64(v)]`

**Wasmtime cross-reference:**
- `crates/wasmtime/src/runtime/component/values.rs` — does NOT call
  canonicalize_nan or maybe_scramble in the component lift/lower path
  (verified by grep returning zero hits in
  `crates/wasmtime/src/runtime/component/`)
- `crates/wasmtime/src/runtime/wave.rs:27,32` `canonicalize_nan32/64`
  — these exist but are in the WAVE serialization (text display
  format), not the canonical ABI runtime
- **Wasmtime drops the spec hook on the floor.** This is a permissible
  spec deviation under DETERMINISTIC_PROFILE=true (the no-op profile)
  because the OUTPUT is identical. Wazero's choice is the opposite:
  call the spec function explicitly so the implementation literally
  corresponds to the spec text, even though the output matches.

**Description:**
The spec defines NaN canonicalization in two places:
1. **Lift side (always on):** `lift_flat` for f32/f64 calls
   `canonicalize_nan32/64`. Wazero already does this at
   `lift.go:73-75`.
2. **Lower side (profile-dependent):** `lower_flat` for f32/f64
   wraps the value in `maybe_scramble_nan32/64`. Under
   `DETERMINISTIC_PROFILE=true`, this is a no-op pass-through.

This item:
1. Adds `maybeScrambleNaN32/64` to `abi/context.go` matching the
   spec signature (under deterministic profile, returns input
   unchanged; the function exists so the lower path can call it
   literally per spec).
2. Adds the call site in `LowerFlat`/`LowerHeap` for f32/f64.
3. Pins `DETERMINISTIC_PROFILE=true` (decision documented in code
   with spec line citation).
4. Adds tests for both lift canonicalization (already covered by
   the Python NaN port in item 19) and lower no-op pass-through.

Why bother adding a no-op call site? Spec literal correspondence.
The spec text at `definitions.py:1877-1878` says the lower path
calls `maybe_scramble_nan*`. Wazero's implementation should mirror
this. The cost is a single function call per f32/f64 lower; under
the deterministic profile it's a return-value pass-through with
no observable behavior change. The benefit is that future spec
changes (e.g., a new profile, a runtime debugging mode that scrambles
to flush out bad code) can be enabled by flipping the profile flag,
without re-finding every f32/f64 lower site.

**Definition of done:**
- `maybeScrambleNaN32` and `maybeScrambleNaN64` exist in
  `abi/context.go` matching `definitions.py:1395-1414`
- `LowerFlat`/`LowerHeap` for f32/f64 call them
- A test confirms NaN bit patterns pass through unchanged under
  the deterministic profile (functional no-op verification)
- A test confirms canonicalization on lift still works (regression
  test for the existing behavior)
- A doc comment in `context.go` explains the deterministic profile
  choice and cites `definitions.py:1209`
- `go test ./internal/component/abi/...` passes

**Reviewer focus areas:**
- Spec compliance: confirm the lower-side call site matches
  `definitions.py:1877-1878` literally; confirm `maybeScrambleNaN32`
  matches `definitions.py:1395-1402` (returns input under
  deterministic profile)
- Code quality: confirm the no-op nature is documented; confirm the
  call sites are NOT optimized away (the function call should exist
  in source even if compiler inlines it); confirm doc comments cite
  spec lines

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
- The test count is at least 285 (= 58 + 20 + 31 + 8 + 14 + 135 + 6
  + 36 + 13 ported subtests; wazero supplemental tests from item 32
  are additional). The 285 floor catches under-implementation; the
  per-item Definition of done catches off-by-one regressions.

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
