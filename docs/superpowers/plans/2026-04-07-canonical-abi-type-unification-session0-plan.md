# Canonical-ABI Type Unification — Session 0 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace wazero's six parallel component-model type representations with a single canonical, wasmtime-style indexed `ComponentTypes` table; rewrite the binary decoder to produce it directly; rewrite the `abi/` lift/lower package to consume it; restructure runtime instance state to match the canonical-ABI spec's single-layer `ComponentInstance` model; and delete every conflicting parallel path. End-state at end of Session 0 is compile-green repo-wide with documented test skips in `internal/component/`.

**Architecture:** Adopts wasmtime's `ComponentTypes` indexed-table approach (Decision 1 in design doc). Composite types live in per-kind slices on `*types.ComponentTypes`; `types.ValType` is a value-type 8-byte `{Kind TypeKind; Index uint32}` struct comparable with `==`. Resource identity is two-layer: structural `types.TypeResourceTable` in the type table + nominal pointer-identity `*runtime.ResourceType` (Decision 5). Runtime state is a single-layer `runtime.ComponentInstance` matching `definitions.py:256-273`, holding a unified `*runtime.Table` for all handle kinds (Decision 6). The `abi/` lift/lower dispatch becomes a `switch typ.Kind` against the `TypeKind` enum, with new `TypeKindOwn`/`TypeKindBorrow` arms (Decision 7) and trap arms for `TypeKindStream`/`TypeKindFuture`/`TypeKindErrorContext` (Decision 8). `canon_lower.go`, `type_resolver.go`, `abi/resource_lower.go`, `runtime/instance_state.go`, and `runtime/resource_type_id.go` are deleted entirely (Decision 9).

**Tech Stack:** Go 1.22+, wazero `internal/component/` packages, canonical-ABI reference implementation at `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py`, wasmtime reference at `debug-vendored/wasmtime/crates/environ/src/component/types.rs` and `types_builder.rs`.

---

## Source of Truth

**Design document (single authoritative reference):**
`docs/superpowers/specs/2026-04-07-canonical-abi-type-unification-design.md`

Every code shape, struct field, function signature, intern-key rule, dispatch arm, deletion target, and verification check in this plan is anchored to a section in that design doc. The design doc text takes precedence over any text in this plan if they conflict; if such a conflict is discovered during execution, stop and resolve it from the design doc.

**Spec authorities cited from the design doc (do not re-verify, only consult for test expectations):**
- `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py` — Python canonical-ABI reference; class definitions at 103-180; ABI formulas at 1065-1171; Table at 303-315; ComponentInstance at 256-273; ResourceType at 351-361; pointer-identity `is` checks at 1336, 1345, 2147; lift/lower at 1700-1746
- `debug-vendored/component-model/design/mvp/CanonicalABI.md` — spec prose; resource identity 531-549; same-instance borrow optimization 2677-2683
- `debug-vendored/component-model/design/mvp/Explainer.md:600` — "none of these types are recursive"
- `debug-vendored/wasmtime/crates/environ/src/component/types.rs` — `InterfaceType` 576-604, `CanonicalAbiInfo` 608+, `record_static` 705-723, `variant` 772-815, `FlagsSize::Size4Plus` 756-770, `POINTER_PAIR` 678-684, `TypeResourceTable` 1125-1147, `SCALAR1`/`SCALAR2`/`SCALAR4`/`SCALAR8` 667+
- `debug-vendored/wasmtime/crates/environ/src/component/types_builder.rs` — `ComponentTypesBuilder` and intern maps at 38-124
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/values.rs` — `Val` 67-93, dynamic dispatch 97-346
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/func/options.rs` — wasmtime `LiftContext`/`LowerContext`
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/vm/component.rs` — wasmtime `ComponentInstance` at 93-159 (the aggregating-map approach wazero is NOT adopting; cited for cross-reference only)
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/resources.rs` — wasmtime pointer-identity `ResourceType`

**Hard constraints (from the task brief and design doc):**
1. **No parallel paths.** Edit existing files in place. Build will break between steps; that is acceptable.
2. **No new placeholders.** No `// TODO`, no silent fallbacks, no "return default on error". If a function depends on Session 2 plumbing, it traps with a precise error.
3. **No other-doc contamination.** The single source of truth is the design doc named above. Do not read any other design doc.
4. **Decisions are final.** The 9 numbered Design Decisions are not re-litigated.
5. **Correctness over preservation.** Where the design overrides existing wazero code, the plan implements the design.

**V8 — Line numbers drift.** Every `file.go:N` reference in the design doc was captured 2026-04-07. Re-grep before each edit.

---

## Preconditions

- Branch: `feat/wasip2-complete-implementation`
- Working directory: `/home/cchamplin/development/wazero`
- Repo-wide `go build ./...` is currently green.
- Repo-wide `go test ./...` currently has known failures in the broken `instance.go` lift/lower paths; these are expected.
- Design doc `docs/superpowers/specs/2026-04-07-canonical-abi-type-unification-design.md` is committed.
- The earlier rename of `component.ComponentInstance` → `component.ParsedComponentInstance` is already in place; do not re-rename.
- No prior session of this work has produced code; the plan starts from a clean slate against the current branch state.

---

## Roadmap and Checkpoints

The 18 work-order steps run in roughly this order with these per-package checkpoint gates:

| Checkpoint | After step | Success criterion |
|---|---|---|
| **A — `types/` package green** | Step 8 | `go build ./internal/component/types/...` and `go test ./internal/component/types/...` both pass |
| **B — `runtime/` package green** | Step 11 | `go build ./internal/component/runtime/...` and `go test ./internal/component/runtime/...` both pass |
| **C — `binary/` package green** | Step 13 | `go build ./internal/component/binary/...` and `go test ./internal/component/binary/...` both pass |
| **D — `abi/` package green** | Step 15 | `go build ./internal/component/abi/...` and `go test ./internal/component/abi/...` both pass |
| **E — Repo compiles** | Step 17 | `go build ./...` is green |
| **F — Conformance green** | Step 18 | `go test ./internal/component/conformance/...` is green |
| **Final** | Step 20 | `go vet ./...`, `go test ./...` end-state matches "Build State at End of Session 0" table in design doc |

Between checkpoints the build is intentionally broken. Within a checkpoint group, tasks may interleave file edits without running tests; verification runs at the gate.

The work-order numbering in the design doc has 18 steps; this plan splits some of them for TDD-friendly granularity. The mapping is documented in each task header (`Design step N`).

---

# Tasks

## Task 1: Add `TypeKind` enum and value `ValType` struct (delete old interface)

**Design step:** 1 (work order)
**Goal:** Replace the `ValType` interface and scalar struct types in `types/types.go` with the new `TypeKind` enum and value-type `ValType` struct, plus named scalar constants. Build will break starting here for everything that imports `types.ValType` as an interface or constructs `types.Bool{}`-style literals.
**Design citations:** Decision 1; Decision 4; Core Type Representation section (design doc lines 247-336)

**Files:**
- Modify (rewrite contents): `internal/component/types/types.go`
- Create: `internal/component/types/types_test.go` (replaces existing — rewritten for the new shape)

**Note:** Existing `types_test.go` content is rewritten in place. The old interface-based tests are superseded.

- [ ] **Step 1.1: Write the failing TypeKind constants test**

Replace the contents of `internal/component/types/types_test.go` with:

```go
package types

import "testing"

func TestTypeKindConstants(t *testing.T) {
	// Confirm the discriminator order matches the design doc.
	// Spec: definitions.py:103-180 (canonical type list).
	cases := []struct {
		k    TypeKind
		want uint8
	}{
		{TypeKindBool, 0},
		{TypeKindS8, 1},
		{TypeKindU8, 2},
		{TypeKindString, 12},
		{TypeKindList, 13},
		{TypeKindFixedList, 14},
		{TypeKindRecord, 15},
		{TypeKindOwn, 22},
		{TypeKindBorrow, 23},
		{TypeKindStream, 24},
		{TypeKindFuture, 25},
		{TypeKindErrorContext, 26},
	}
	for _, c := range cases {
		if uint8(c.k) != c.want {
			t.Errorf("TypeKind(%v) = %d, want %d", c.k, uint8(c.k), c.want)
		}
	}
}

func TestValTypeIsZero(t *testing.T) {
	var z ValType
	if !z.IsZero() {
		t.Errorf("zero ValType.IsZero() = false, want true")
	}
	if Bool.IsZero() {
		t.Errorf("Bool.IsZero() = true, want false")
	}
	if (ValType{Kind: TypeKindRecord, Index: 5}).IsZero() {
		t.Errorf("non-zero ValType.IsZero() = true, want false")
	}
}

func TestNamedScalarConstants(t *testing.T) {
	cases := []struct {
		name string
		v    ValType
		kind TypeKind
	}{
		{"Bool", Bool, TypeKindBool},
		{"S8", S8, TypeKindS8},
		{"U8", U8, TypeKindU8},
		{"S16", S16, TypeKindS16},
		{"U16", U16, TypeKindU16},
		{"S32", S32, TypeKindS32},
		{"U32", U32, TypeKindU32},
		{"S64", S64, TypeKindS64},
		{"U64", U64, TypeKindU64},
		{"F32", F32, TypeKindF32},
		{"F64", F64, TypeKindF64},
		{"Char", Char, TypeKindChar},
		{"String_", String_, TypeKindString},
	}
	for _, c := range cases {
		if c.v.Kind != c.kind {
			t.Errorf("%s.Kind = %v, want %v", c.name, c.v.Kind, c.kind)
		}
		if c.v.Index != 0 {
			t.Errorf("%s.Index = %d, want 0", c.name, c.v.Index)
		}
	}
}

func TestValTypeComparable(t *testing.T) {
	// ValType is a value-type struct and must be usable as a map key.
	m := map[ValType]string{
		Bool:                                "bool",
		U32:                                 "u32",
		{Kind: TypeKindRecord, Index: 5}:    "record5",
		{Kind: TypeKindRecord, Index: 6}:    "record6",
	}
	if m[Bool] != "bool" {
		t.Errorf("map lookup of Bool failed")
	}
	if m[ValType{Kind: TypeKindRecord, Index: 5}] != "record5" {
		t.Errorf("map lookup of record5 failed")
	}
}
```

- [ ] **Step 1.2: Run the test to confirm it fails**

```bash
go test ./internal/component/types/... -run 'TestTypeKindConstants|TestValTypeIsZero|TestNamedScalarConstants|TestValTypeComparable'
```

Expected: compile error in `types.go` (`undefined: TypeKind`, etc.).

- [ ] **Step 1.3: Replace the contents of `types/types.go` with the new core types**

Overwrite `internal/component/types/types.go` with exactly the code from the design doc's "Core types" subsection (design doc lines 262-336) plus the package documentation. The full body is:

```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

// TypeKind discriminates the variants of ValType. For scalar kinds the
// Index field of ValType is unused. For composite kinds Index points
// into the corresponding slice on *ComponentTypes.
//
// Spec: debug-vendored/component-model/design/mvp/canonical-abi/definitions.py:103-180
type TypeKind uint8

const (
	TypeKindBool TypeKind = iota
	TypeKindS8
	TypeKindU8
	TypeKindS16
	TypeKindU16
	TypeKindS32
	TypeKindU32
	TypeKindS64
	TypeKindU64
	TypeKindF32
	TypeKindF64
	TypeKindChar
	TypeKindString
	TypeKindList         // Index -> ComponentTypes.Lists (dynamic length)
	TypeKindFixedList    // Index -> ComponentTypes.FixedLists (fixed length, distinct type)
	TypeKindRecord       // Index -> ComponentTypes.Records
	TypeKindTuple        // Index -> ComponentTypes.Tuples
	TypeKindVariant      // Index -> ComponentTypes.Variants
	TypeKindEnum         // Index -> ComponentTypes.Enums
	TypeKindOption       // Index -> ComponentTypes.Options
	TypeKindResult       // Index -> ComponentTypes.Results
	TypeKindFlags        // Index -> ComponentTypes.Flags
	TypeKindOwn          // Index -> ComponentTypes.ResourceTables
	TypeKindBorrow       // Index -> ComponentTypes.ResourceTables
	TypeKindStream       // Index -> ComponentTypes.Streams (lift/lower traps)
	TypeKindFuture       // Index -> ComponentTypes.Futures (lift/lower traps)
	TypeKindErrorContext // Index -> ComponentTypes.ErrorContextTables (lift/lower traps)
)

// ValType identifies a single component-model value type. 8 bytes total.
// Comparable with ==, usable as a map key, copyable by value. Pass by
// value through lift/lower dispatch.
//
// For scalar kinds (TypeKindBool through TypeKindString), Index is zero
// and ignored. For composite kinds Index is the offset into the matching
// ComponentTypes slice.
type ValType struct {
	Kind  TypeKind
	Index uint32
}

// IsZero reports whether v is the zero ValType. Zero is distinguishable
// from a legitimate TypeKindBool value only by context; the builder
// never returns a zero ValType.
func (v ValType) IsZero() bool { return v == ValType{} }

// Named scalar constants. These are the only non-composite ValType
// values that can be constructed without a builder.
var (
	Bool    = ValType{Kind: TypeKindBool}
	S8      = ValType{Kind: TypeKindS8}
	U8      = ValType{Kind: TypeKindU8}
	S16     = ValType{Kind: TypeKindS16}
	U16     = ValType{Kind: TypeKindU16}
	S32     = ValType{Kind: TypeKindS32}
	U32     = ValType{Kind: TypeKindU32}
	S64     = ValType{Kind: TypeKindS64}
	U64     = ValType{Kind: TypeKindU64}
	F32     = ValType{Kind: TypeKindF32}
	F64     = ValType{Kind: TypeKindF64}
	Char    = ValType{Kind: TypeKindChar}
	String_ = ValType{Kind: TypeKindString}
)

// ComponentTypes is the per-top-level-component immutable type bag.
// Built by ComponentTypesBuilder during binary decode, frozen at Finish,
// and threaded through all subsequent lift/lower / validation / linking.
// One pointer identity per compiled component drives the fast-path
// type-equality short-circuit during cross-component type checking
// (added in Session 2).
type ComponentTypes struct {
	Records            []TypeRecord
	Variants           []TypeVariant
	Lists              []TypeList            // dynamic-length lists only
	FixedLists         []TypeFixedLengthList // fixed-length lists are a distinct type
	Tuples             []TypeTuple
	Flags              []TypeFlags
	Enums              []TypeEnum
	Options            []TypeOption
	Results            []TypeResult
	ResourceTables     []TypeResourceTable
	Streams            []TypeStream
	Futures            []TypeFuture
	ErrorContextTables []TypeErrorContextTable
	Funcs              []TypeFunc
}
```

This deletes the old `ValType` interface and the 13 scalar struct types (`Bool`, `S8`, ..., `String`) along with their methods. The build will be broken from this point until Step 8.

- [ ] **Step 1.4: Run the test to confirm it passes (in the `types/` package alone)**

```bash
go test ./internal/component/types/... -run 'TestTypeKindConstants|TestValTypeIsZero|TestNamedScalarConstants|TestValTypeComparable' 2>&1 | head -50
```

Expected: tests fail to compile because `composite.go`, `resource.go`, `val.go`, and `composite_test.go` reference the deleted symbols. Note the failure but proceed — those files are rewritten in Tasks 2-6.

- [ ] **Step 1.5: Commit**

```bash
git add internal/component/types/types.go internal/component/types/types_test.go
git commit -m "$(cat <<'EOF'
types: introduce TypeKind enum and value ValType struct

Replace the ValType interface and the 13 scalar struct types with a
TypeKind discriminator and an 8-byte value-type ValType{Kind, Index}.
Add ComponentTypes type-bag struct (slices populated by composite
types in following commits). Add named scalar constants. Build is
intentionally broken from this commit until the canonical-ABI
unification series completes.

Design: docs/superpowers/specs/2026-04-07-canonical-abi-type-unification-design.md

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Replace composite types with table-entry structs

**Design step:** 2 (work order)
**Goal:** Rewrite `types/composite.go` to define `TypeRecord`, `TypeVariant`, `TypeList`, `TypeFixedLengthList`, `TypeTuple`, `TypeFlags`, `TypeEnum`, `TypeOption`, `TypeResult`, `TypeStream`, `TypeFuture`, `TypeErrorContextTable`, `TypeFunc`, plus helper structs `RecordField` and `VariantCase`. Each composite carries a `CanonicalABIInfo` field for precomputed ABI metadata. The old `Record`/`Variant`/etc. struct types and their methods are deleted.
**Design citations:** Composite structs subsection (design doc lines 369-477); Decision 1, Decision 7

**Files:**
- Modify (full rewrite): `internal/component/types/composite.go`

**Note:** This task does NOT add tests yet — the new structs cannot be exercised without the builder (Task 5) and the ABI helper (Task 4). Composite tests are written in Task 8.

- [ ] **Step 2.1: Replace `types/composite.go` contents**

Overwrite `internal/component/types/composite.go` with:

```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

// RecordField is one field of a record type. Order is significant
// (spec-defined); names are unique within the record.
type RecordField struct {
	Name string
	Type ValType
}

// TypeRecord is a record (struct) type with named, ordered fields.
// Spec: definitions.py:118-121 (RecordType).
type TypeRecord struct {
	Fields []RecordField
	ABI    CanonicalABIInfo
}

// VariantCase is one case of a variant type. HasPayload distinguishes
// the unit case from the payload case (Payload is zero-valued iff
// HasPayload is false).
type VariantCase struct {
	Name       string
	Payload    ValType
	HasPayload bool
}

// TypeVariant is a discriminated-union variant type.
// Spec: definitions.py:128-132 (VariantType).
type TypeVariant struct {
	Cases []VariantCase
	ABI   CanonicalABIInfo
	Disc  DiscriminantInfo
}

// TypeList is a dynamic-length list. Memory layout is (ptr: i32, len: i32).
// Fixed-length lists are a distinct type — see TypeFixedLengthList.
// Spec: definitions.py:122-125 (ListType with l == None).
type TypeList struct {
	Element ValType
	ABI     CanonicalABIInfo
}

// TypeFixedLengthList is a list with a compile-time-known length. Memory
// layout is `length` elements stored inline, not via ptr+len indirection.
// Distinct from TypeList because spec and wasmtime treat them as distinct
// types: a function expecting `list<u32>` cannot accept a `list<u32, 5>`
// and vice versa.
//
// Spec: definitions.py:122-125 — `ListType(t, l)` with `l != None`.
// Wasmtime: environ/src/component/types.rs uses TypeListIndex (lists)
// and TypeFixedLengthListIndex (fixed-length lists) as distinct keys.
type TypeFixedLengthList struct {
	Element ValType
	Length  uint32 // > 0 per spec
	ABI     CanonicalABIInfo
}

// TypeTuple is a positional record (anonymous struct).
// Spec: definitions.py:126-127 (TupleType).
type TypeTuple struct {
	Types []ValType
	ABI   CanonicalABIInfo
}

// TypeFlags is a set of named boolean flags packed into i32 words.
// Spec: definitions.py:166-168 (FlagsType).
type TypeFlags struct {
	Names []string
	ABI   CanonicalABIInfo
}

// TypeEnum is a discriminant-only variant (no payloads).
// Spec: definitions.py:163-165 (EnumType).
type TypeEnum struct {
	Names []string
	ABI   CanonicalABIInfo
	Disc  DiscriminantInfo
}

// TypeOption is syntactic sugar for variant{none, some(T)}.
// Spec: definitions.py:160-162 (OptionType).
type TypeOption struct {
	Element ValType
	ABI     CanonicalABIInfo
	Disc    DiscriminantInfo
}

// TypeResult is syntactic sugar for variant{ok(T), error(E)}.
// Spec: definitions.py:155-159 (ResultType).
type TypeResult struct {
	OK     ValType
	Err    ValType
	HasOK  bool
	HasErr bool
	ABI    CanonicalABIInfo
	Disc   DiscriminantInfo
}

// TypeStream is an async stream-of-element type. Lift/lower traps
// in Session 0; per-instance table identity is added when async lands.
type TypeStream struct {
	Element    ValType
	HasElement bool
}

// TypeFuture is an async future-of-element type. Lift/lower traps
// in Session 0; per-instance table identity is added when async lands.
type TypeFuture struct {
	Element    ValType
	HasElement bool
}

// TypeErrorContextTable is intentionally empty for Session 0. The
// per-instance table identity layering analogous to TypeResourceTable
// is added when async lift/lower lands.
type TypeErrorContextTable struct{}

// TypeFunc represents a component function type. Matches the wasmtime
// shape (environ/src/component/types.rs:557-566): params and results
// are each a Tuple (a ValType of TypeKindTuple). One mechanism for
// "ordered list of types," reused.
type TypeFunc struct {
	Async      bool
	ParamNames []string
	Params     ValType // TypeKindTuple
	Results    ValType // TypeKindTuple
}
```

This deletes `Field`, `Record`, `Case`, `Variant`, `List`, `Tuple`, `Flags`, `Enum`, `Option`, `Result`, `Stream`, `Future`, `ErrorContext`, the old `alignTo` helper, and every `valType()` / `Size()` / `Align()` / `FlattenCount()` method on those types. (The `alignTo` helper will be reintroduced in Task 4 inside `abi_info.go` if needed for ABI computation.)

- [ ] **Step 2.2: Verify the file parses (cannot test yet)**

```bash
gofmt -e internal/component/types/composite.go
```

Expected: empty output (file parses cleanly). Compile errors elsewhere are expected and will cascade through Task 8.

- [ ] **Step 2.3: Commit**

```bash
git add internal/component/types/composite.go
git commit -m "$(cat <<'EOF'
types: replace inline composite structs with table-entry shapes

Each composite (TypeRecord, TypeVariant, TypeList, TypeFixedLengthList,
TypeTuple, TypeFlags, TypeEnum, TypeOption, TypeResult, TypeStream,
TypeFuture, TypeErrorContextTable, TypeFunc) is a struct with the
structural fields plus a CanonicalABIInfo precomputed field. Helpers
RecordField and VariantCase replace Field/Case. The old interface
methods are gone; ABI is read via the table accessor added in a
following commit.

TypeFixedLengthList is a distinct kind from TypeList, matching wasmtime
and spec definitions.py:122-125.

Design: docs/superpowers/specs/2026-04-07-canonical-abi-type-unification-design.md

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Replace `types/resource.go` with structural-only TypeResourceTable

**Design step:** 3 (work order)
**Goal:** Delete `Own`, `Borrow`, and the existing limited `ResourceType` struct from `types/resource.go`. Add `ResourceIdx`, `RuntimeComponentInstanceIdx`, `ResourceTableIdx` named uint32 types and the structural `TypeResourceTable` struct. The nominal-identity `*runtime.ResourceType` lives in `runtime/resource_type.go` (Task 7), not here, to avoid an import cycle.
**Design citations:** Decision 5; Resource Identity / Structural layer subsection (design doc lines 561-604)

**Files:**
- Modify (full rewrite): `internal/component/types/resource.go`
- Modify (full rewrite): `internal/component/types/resource_test.go`

- [ ] **Step 3.1: Write the failing structural-resource test**

Replace the contents of `internal/component/types/resource_test.go` with:

```go
package types

import "testing"

func TestResourceIdxRoundTrip(t *testing.T) {
	var r ResourceIdx = 5
	if uint32(r) != 5 {
		t.Errorf("ResourceIdx round-trip = %d, want 5", uint32(r))
	}
}

func TestRuntimeComponentInstanceIdxRoundTrip(t *testing.T) {
	var i RuntimeComponentInstanceIdx = 7
	if uint32(i) != 7 {
		t.Errorf("RuntimeComponentInstanceIdx round-trip = %d, want 7", uint32(i))
	}
}

func TestResourceTableIdxRoundTrip(t *testing.T) {
	var idx ResourceTableIdx = 11
	if uint32(idx) != 11 {
		t.Errorf("ResourceTableIdx round-trip = %d, want 11", uint32(idx))
	}
}

func TestTypeResourceTableConcrete(t *testing.T) {
	rt := TypeResourceTable{
		Concrete: true,
		Resource: 3,
		Instance: 1,
	}
	if !rt.Concrete {
		t.Errorf("Concrete = false, want true")
	}
	if rt.Resource != 3 {
		t.Errorf("Resource = %d, want 3", rt.Resource)
	}
	if rt.Instance != 1 {
		t.Errorf("Instance = %d, want 1", rt.Instance)
	}
}

func TestTypeResourceTableAbstract(t *testing.T) {
	rt := TypeResourceTable{
		Concrete:    false,
		AbstractIdx: 42,
	}
	if rt.Concrete {
		t.Errorf("Concrete = true, want false")
	}
	if rt.AbstractIdx != 42 {
		t.Errorf("AbstractIdx = %d, want 42", rt.AbstractIdx)
	}
}

func TestOwnBorrowValType(t *testing.T) {
	// Own and Borrow are encoded as ValType values, not separate structs.
	own := ValType{Kind: TypeKindOwn, Index: 5}
	borrow := ValType{Kind: TypeKindBorrow, Index: 5}
	if own.Kind != TypeKindOwn {
		t.Errorf("own.Kind = %v, want TypeKindOwn", own.Kind)
	}
	if borrow.Kind != TypeKindBorrow {
		t.Errorf("borrow.Kind = %v, want TypeKindBorrow", borrow.Kind)
	}
	if own == borrow {
		t.Errorf("own and borrow at same index should be distinct ValTypes")
	}
}
```

- [ ] **Step 3.2: Run the test to confirm it fails**

```bash
go test ./internal/component/types/... -run 'TestResourceIdx|TestTypeResourceTable|TestOwnBorrowValType' 2>&1 | head -30
```

Expected: compile errors for `ResourceIdx`, `RuntimeComponentInstanceIdx`, `ResourceTableIdx`, `TypeResourceTable`.

- [ ] **Step 3.3: Replace `types/resource.go`**

Overwrite `internal/component/types/resource.go` with:

```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

// ResourceIdx names a resource *declaration* — a `(type $r (resource ...))`
// site in a component's binary. Unique within a single component's type
// section. The runtime nominal layer maps (RuntimeComponentInstanceIdx,
// ResourceIdx) → *runtime.ResourceType for the spec's `is` check at
// definitions.py:1345.
type ResourceIdx uint32

// RuntimeComponentInstanceIdx names an instantiated component instance
// at runtime, assigned monotonically at instantiation time.
type RuntimeComponentInstanceIdx uint32

// ResourceTableIdx is the index of a TypeResourceTable entry in
// ComponentTypes.ResourceTables. TypeKindOwn / TypeKindBorrow ValTypes
// carry this as their Index field.
type ResourceTableIdx uint32

// TypeResourceTable is the structural layer in ComponentTypes.ResourceTables.
// Two variants:
//
//   - Concrete: bound to a specific runtime component instance. Resolves
//     at call time via runtime.ComponentInstance.ResourceTypes (possibly
//     walking to a parent or across instances) to the nominal
//     *runtime.ResourceType for validity checking.
//   - Abstract: lives only inside a not-yet-instantiated component or
//     instance type declaration. Cannot be lifted/lowered at runtime;
//     lift/lower traps if reached at call time.
//
// At end of Session 0 ALL entries are Abstract — Concrete promotion at
// instantiation time is Session 2 work.
//
// Spec: CanonicalABI.md:531-549.
type TypeResourceTable struct {
	Concrete bool

	// Concrete fields (Concrete == true)
	Resource ResourceIdx                 // which nominal declaration
	Instance RuntimeComponentInstanceIdx // which instance defines it

	// Abstract fields (Concrete == false)
	AbstractIdx uint32
}
```

This deletes the existing `Own`, `Borrow`, the old composite `ResourceType` struct, and any related methods.

- [ ] **Step 3.4: Run the test to confirm it passes**

```bash
go test ./internal/component/types/... -run 'TestResourceIdx|TestTypeResourceTable|TestOwnBorrowValType' 2>&1 | head -40
```

Expected: tests still fail because `composite_test.go` and `val_test.go` reference deleted symbols. Note that the new tests are syntactically valid in `resource_test.go` (will pass once the rest of the package compiles in Task 8).

- [ ] **Step 3.5: Commit**

```bash
git add internal/component/types/resource.go internal/component/types/resource_test.go
git commit -m "$(cat <<'EOF'
types: replace resource.go with structural TypeResourceTable

Delete the old Own, Borrow, and composite ResourceType structs from
types/. Introduce ResourceIdx, RuntimeComponentInstanceIdx,
ResourceTableIdx named types and the TypeResourceTable struct
covering both Concrete (Session 2) and Abstract (Session 0) variants.

The nominal-identity *ResourceType lives in runtime/ to avoid an
import cycle; types/ holds only the structural layer.

Design: docs/superpowers/specs/2026-04-07-canonical-abi-type-unification-design.md

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Add `types/abi_info.go` with `CanonicalABIInfo`, `DiscriminantInfo`, scalar table, and ABI accessor

**Design step:** 4 (work order)
**Goal:** Create `types/abi_info.go` containing `CanonicalABIInfo`, `DiscriminantInfo`, the package-level `scalarABI` table, the `(ValType).ABI` accessor, and any helpers required to compute composite ABI values during interning (`alignTo`, `discriminantSize`, `recordABI`, `variantABI`, `listABI`, `fixedListABI`, `tupleABI`, `flagsABI`, `enumABI`, `optionABI`, `resultABI`).
**Design citations:** ABI metadata types subsection (design doc lines 482-555); V1 verification table (design doc lines 1840-1898); Decision 1

**Files:**
- Create: `internal/component/types/abi_info.go`
- Create: `internal/component/types/abi_info_test.go`

- [ ] **Step 4.1: Write the failing scalar ABI test**

Create `internal/component/types/abi_info_test.go` with:

```go
package types

import "testing"

// TestScalarABI verifies the scalar ABI table against canonical-ABI spec
// values. Citations are inline at each entry. Spec authority:
// debug-vendored/component-model/design/mvp/canonical-abi/definitions.py
// at lines 1065-1138 and 1705-1719.
func TestScalarABI(t *testing.T) {
	cases := []struct {
		v        ValType
		size32   uint32
		align32  uint32
		size64   uint32
		align64  uint32
		flatten  int32
		specCite string
	}{
		{Bool, 1, 1, 1, 1, 1, "definitions.py:1065,1123,1705"},
		{S8, 1, 1, 1, 1, 1, "definitions.py:1066,1124,1706"},
		{U8, 1, 1, 1, 1, 1, "definitions.py:1066,1124,1706"},
		{S16, 2, 2, 2, 2, 1, "definitions.py:1067,1125,1706"},
		{U16, 2, 2, 2, 2, 1, "definitions.py:1067,1125,1706"},
		{S32, 4, 4, 4, 4, 1, "definitions.py:1068,1126,1706"},
		{U32, 4, 4, 4, 4, 1, "definitions.py:1068,1126,1706"},
		{S64, 8, 8, 8, 8, 1, "definitions.py:1069,1127,1708"},
		{U64, 8, 8, 8, 8, 1, "definitions.py:1069,1127,1708"},
		{F32, 4, 4, 4, 4, 1, "definitions.py:1070,1128,1709"},
		{F64, 8, 8, 8, 8, 1, "definitions.py:1071,1129,1710"},
		{Char, 4, 4, 4, 4, 1, "definitions.py:1072,1130,1711"},
		// Strings: memory32 = 8/4 (ptr+len i32), memory64 = 16/8.
		// Spec: definitions.py:1073,1131,1712 (memory32 only).
		// Wasmtime: types.rs:678-684 POINTER_PAIR (memory64 doubles).
		// This is divergence (3) from the literal spec — see design doc.
		{String_, 8, 4, 16, 8, 2, "definitions.py:1073,1131,1712"},
	}
	for _, c := range cases {
		ct := &ComponentTypes{} // scalar ABIs do not need a populated ct
		got := c.v.ABI(ct)
		if got.Size32 != c.size32 || got.Align32 != c.align32 ||
			got.Size64 != c.size64 || got.Align64 != c.align64 ||
			got.FlattenCount != c.flatten {
			t.Errorf("%v.ABI = {%d/%d/%d/%d/flat=%d}, want {%d/%d/%d/%d/flat=%d} (%s)",
				c.v.Kind, got.Size32, got.Align32, got.Size64, got.Align64, got.FlattenCount,
				c.size32, c.align32, c.size64, c.align64, c.flatten, c.specCite)
		}
	}
}

func TestScalarABIHandles(t *testing.T) {
	// own/borrow/stream/future/error-context all encode as i32 handles.
	// Spec: definitions.py:1079,1080,1132,1137,1138,1713,1718,1719.
	// All have size 4, align 4, flatten 1.
	ct := &ComponentTypes{}
	for _, k := range []TypeKind{
		TypeKindOwn, TypeKindBorrow,
		TypeKindStream, TypeKindFuture, TypeKindErrorContext,
	} {
		v := ValType{Kind: k}
		got := v.ABI(ct)
		if got.Size32 != 4 || got.Align32 != 4 || got.FlattenCount != 1 {
			t.Errorf("kind %v ABI = {%d/%d/flat=%d}, want {4/4/1}",
				k, got.Size32, got.Align32, got.FlattenCount)
		}
	}
}

func TestRecordABIEmpty(t *testing.T) {
	// Empty records have size 0. Divergence (1) from the literal spec
	// (definitions.py:1150 asserts s > 0); wasmtime's record_static at
	// types.rs:705-723 returns CanonicalAbiInfo::ZERO. Both wazero and
	// this design preserve the permissive behavior.
	abi := computeRecordABI(nil, &ComponentTypes{})
	if abi.Size32 != 0 || abi.Align32 != 1 || abi.FlattenCount != 0 {
		t.Errorf("empty record ABI = {%d/%d/flat=%d}, want {0/1/0}",
			abi.Size32, abi.Align32, abi.FlattenCount)
	}
}

func TestRecordABISimple(t *testing.T) {
	// record { a: u32, b: u32 } -> size 8, align 4, flatten 2.
	// Spec: alignment_record at definitions.py:1087-1091, elem_size_record
	// at :1145-1151, flatten_record at :1726-1730.
	abi := computeRecordABI([]RecordField{
		{Name: "a", Type: U32},
		{Name: "b", Type: U32},
	}, &ComponentTypes{})
	if abi.Size32 != 8 || abi.Align32 != 4 || abi.FlattenCount != 2 {
		t.Errorf("record{a:u32,b:u32} ABI = {%d/%d/flat=%d}, want {8/4/2}",
			abi.Size32, abi.Align32, abi.FlattenCount)
	}
}

func TestVariantDiscriminantSize(t *testing.T) {
	// Spec: discriminant_type at definitions.py:1096-1103.
	// n <= 256 -> 1 byte, n <= 65536 -> 2 bytes, otherwise 4 bytes.
	cases := []struct {
		n    int
		want uint8
	}{
		{1, 1},
		{2, 1},
		{256, 1},
		{257, 2},
		{65536, 2},
		{65537, 4},
	}
	for _, c := range cases {
		got := discriminantSize(c.n)
		if got != c.want {
			t.Errorf("discriminantSize(%d) = %d, want %d", c.n, got, c.want)
		}
	}
}

func TestFlagsABISmall(t *testing.T) {
	// Spec: alignment_flags at definitions.py:1112-1117, elem_size_flags
	// at :1166-1171.
	// n <= 8: size 1, align 1, flatten 1.
	cases := []struct {
		n        int
		size     uint32
		align    uint32
		flatten  int32
	}{
		{1, 1, 1, 1},
		{8, 1, 1, 1},
		{9, 2, 2, 1},
		{16, 2, 2, 1},
		{17, 4, 4, 1},
		{32, 4, 4, 1},
		// Divergence (2) from literal spec: flags > 32 are permitted via
		// multi-i32 encoding, matching wasmtime's FlagsSize::Size4Plus(n)
		// at types.rs:756-770.
		{33, 8, 4, 2},
		{64, 8, 4, 2},
		{65, 12, 4, 3},
	}
	for _, c := range cases {
		names := make([]string, c.n)
		abi := computeFlagsABI(names)
		if abi.Size32 != c.size || abi.Align32 != c.align || abi.FlattenCount != c.flatten {
			t.Errorf("flags(n=%d) ABI = {%d/%d/flat=%d}, want {%d/%d/%d}",
				c.n, abi.Size32, abi.Align32, abi.FlattenCount,
				c.size, c.align, c.flatten)
		}
	}
}

func TestListDynamicABI(t *testing.T) {
	// Dynamic list: pointer-pair. memory32=8/4, memory64=16/8, flatten=2.
	// Spec: definitions.py:1075,1133,1714. Wasmtime POINTER_PAIR at
	// types.rs:678-684.
	abi := computeListABI(U32, &ComponentTypes{})
	if abi.Size32 != 8 || abi.Align32 != 4 || abi.Size64 != 16 || abi.Align64 != 8 || abi.FlattenCount != 2 {
		t.Errorf("list<u32> ABI = {%d/%d/%d/%d/flat=%d}, want {8/4/16/8/2}",
			abi.Size32, abi.Align32, abi.Size64, abi.Align64, abi.FlattenCount)
	}
}

func TestFixedListABI(t *testing.T) {
	// Fixed-length list: inline elements. size = length * elem.size,
	// align = elem.align, flatten = length * elem.flatten.
	// Spec: alignment_list at :1082-1085, elem_size_list at :1140-1143,
	// flatten_list at :1721-1723.
	abi := computeFixedListABI(U32, 5, &ComponentTypes{})
	if abi.Size32 != 20 || abi.Align32 != 4 || abi.FlattenCount != 5 {
		t.Errorf("list<u32, 5> ABI = {%d/%d/flat=%d}, want {20/4/5}",
			abi.Size32, abi.Align32, abi.FlattenCount)
	}
}
```

- [ ] **Step 4.2: Run the test to confirm it fails**

```bash
go test ./internal/component/types/... -run 'TestScalarABI|TestScalarABIHandles|TestRecordABI|TestVariantDiscriminantSize|TestFlagsABI|TestListDynamicABI|TestFixedListABI' 2>&1 | head -40
```

Expected: compile errors (`undefined: computeRecordABI`, `undefined: ABI`, etc.).

- [ ] **Step 4.3: Create `types/abi_info.go`**

Create `internal/component/types/abi_info.go` with:

```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import "fmt"

// CanonicalABIInfo carries precomputed size / alignment / flatten data
// for a type in both 32-bit and 64-bit memory modes. Mirrors wasmtime's
// CanonicalAbiInfo at debug-vendored/wasmtime/crates/environ/src/component/types.rs:608+.
//
// Computed once during interning and stored on the composite struct.
// Lift/lower never recomputes.
type CanonicalABIInfo struct {
	Size32, Align32 uint32
	Size64, Align64 uint32
	FlattenCount    int32 // -1 if the type is not representable in flat form
}

// DiscriminantInfo carries derived sizing and offsets for variant-shaped
// types (Variant, Enum, Option, Result). Computed during interning.
type DiscriminantInfo struct {
	DiscSize      uint8  // 1, 2, or 4 bytes
	PayloadOffset uint32 // byte offset of the payload in the discriminated layout
}

// scalarABI is a package-level constant table for types whose ABI is
// not dependent on their content. Indexed by TypeKind. Spec citations
// in the test for each entry.
var scalarABI = [...]CanonicalABIInfo{
	TypeKindBool:         {Size32: 1, Align32: 1, Size64: 1, Align64: 1, FlattenCount: 1},
	TypeKindS8:           {Size32: 1, Align32: 1, Size64: 1, Align64: 1, FlattenCount: 1},
	TypeKindU8:           {Size32: 1, Align32: 1, Size64: 1, Align64: 1, FlattenCount: 1},
	TypeKindS16:          {Size32: 2, Align32: 2, Size64: 2, Align64: 2, FlattenCount: 1},
	TypeKindU16:          {Size32: 2, Align32: 2, Size64: 2, Align64: 2, FlattenCount: 1},
	TypeKindS32:          {Size32: 4, Align32: 4, Size64: 4, Align64: 4, FlattenCount: 1},
	TypeKindU32:          {Size32: 4, Align32: 4, Size64: 4, Align64: 4, FlattenCount: 1},
	TypeKindS64:          {Size32: 8, Align32: 8, Size64: 8, Align64: 8, FlattenCount: 1},
	TypeKindU64:          {Size32: 8, Align32: 8, Size64: 8, Align64: 8, FlattenCount: 1},
	TypeKindF32:          {Size32: 4, Align32: 4, Size64: 4, Align64: 4, FlattenCount: 1},
	TypeKindF64:          {Size32: 8, Align32: 8, Size64: 8, Align64: 8, FlattenCount: 1},
	TypeKindChar:         {Size32: 4, Align32: 4, Size64: 4, Align64: 4, FlattenCount: 1},
	TypeKindString:       {Size32: 8, Align32: 4, Size64: 16, Align64: 8, FlattenCount: 2}, // ptr+len; memory64 doubles
	TypeKindOwn:          {Size32: 4, Align32: 4, Size64: 4, Align64: 4, FlattenCount: 1},
	TypeKindBorrow:       {Size32: 4, Align32: 4, Size64: 4, Align64: 4, FlattenCount: 1},
	TypeKindStream:       {Size32: 4, Align32: 4, Size64: 4, Align64: 4, FlattenCount: 1},
	TypeKindFuture:       {Size32: 4, Align32: 4, Size64: 4, Align64: 4, FlattenCount: 1},
	TypeKindErrorContext: {Size32: 4, Align32: 4, Size64: 4, Align64: 4, FlattenCount: 1},
}

// ABI returns the canonical-ABI size/align/flatten info for a given
// ValType. Scalar kinds read from a package-level constant table;
// composite kinds dereference into *ComponentTypes.
func (v ValType) ABI(ct *ComponentTypes) CanonicalABIInfo {
	if v.Kind <= TypeKindString {
		return scalarABI[v.Kind]
	}
	switch v.Kind {
	case TypeKindRecord:
		return ct.Records[v.Index].ABI
	case TypeKindVariant:
		return ct.Variants[v.Index].ABI
	case TypeKindList:
		return ct.Lists[v.Index].ABI
	case TypeKindFixedList:
		return ct.FixedLists[v.Index].ABI
	case TypeKindTuple:
		return ct.Tuples[v.Index].ABI
	case TypeKindFlags:
		return ct.Flags[v.Index].ABI
	case TypeKindEnum:
		return ct.Enums[v.Index].ABI
	case TypeKindOption:
		return ct.Options[v.Index].ABI
	case TypeKindResult:
		return ct.Results[v.Index].ABI
	case TypeKindOwn, TypeKindBorrow, TypeKindStream, TypeKindFuture, TypeKindErrorContext:
		return scalarABI[v.Kind]
	}
	panic(fmt.Sprintf("ABI: unknown TypeKind %d", v.Kind))
}

// alignTo rounds offset up to the given alignment. align must be a
// power of two.
func alignTo(offset, align uint32) uint32 {
	return (offset + align - 1) &^ (align - 1)
}

// computeRecordABI computes the canonical ABI info for a record with
// the given (already-interned) field types.
//
// Spec: alignment_record at definitions.py:1087-1091, elem_size_record
// at :1145-1151, flatten_record at :1726-1730. Wasmtime: record_static
// at types.rs:705-723.
//
// Empty records yield size 0, align 1 — divergence (1) from the literal
// spec which asserts s > 0 at definitions.py:1150. Both wasmtime and
// this design permit empty records.
func computeRecordABI(fields []RecordField, ct *ComponentTypes) CanonicalABIInfo {
	if len(fields) == 0 {
		return CanonicalABIInfo{Size32: 0, Align32: 1, Size64: 0, Align64: 1, FlattenCount: 0}
	}
	var size32, align32 uint32 = 0, 1
	var size64, align64 uint32 = 0, 1
	var flatten int32
	for _, f := range fields {
		fa := f.Type.ABI(ct)
		if fa.Align32 > align32 {
			align32 = fa.Align32
		}
		if fa.Align64 > align64 {
			align64 = fa.Align64
		}
		size32 = alignTo(size32, fa.Align32) + fa.Size32
		size64 = alignTo(size64, fa.Align64) + fa.Size64
		flatten += fa.FlattenCount
	}
	size32 = alignTo(size32, align32)
	size64 = alignTo(size64, align64)
	return CanonicalABIInfo{
		Size32: size32, Align32: align32,
		Size64: size64, Align64: align64,
		FlattenCount: flatten,
	}
}

// computeTupleABI is record ABI for positional types.
func computeTupleABI(elems []ValType, ct *ComponentTypes) CanonicalABIInfo {
	fs := make([]RecordField, len(elems))
	for i, e := range elems {
		fs[i] = RecordField{Type: e}
	}
	return computeRecordABI(fs, ct)
}

// discriminantSize returns the byte size of the discriminant for a
// variant with n cases. Spec: discriminant_type at definitions.py:1096-1103.
func discriminantSize(n int) uint8 {
	switch {
	case n <= 256:
		return 1
	case n <= 65536:
		return 2
	default:
		return 4
	}
}

// computeVariantABI computes ABI for a variant with the given
// (already-interned) cases.
//
// Spec: alignment_variant at definitions.py:1093-1094, elem_size_variant
// at :1156-1164, flatten_variant at :1732-1741, max_case_alignment at
// :1105-1110, join at :1743-1746.
func computeVariantABI(cases []VariantCase, ct *ComponentTypes) (CanonicalABIInfo, DiscriminantInfo) {
	disc := discriminantSize(len(cases))
	discA := uint32(disc)

	var maxCaseAlign32, maxCaseSize32 uint32 = 1, 0
	var maxCaseAlign64, maxCaseSize64 uint32 = 1, 0
	var maxCaseFlatten int32
	for _, c := range cases {
		if !c.HasPayload {
			continue
		}
		pa := c.Payload.ABI(ct)
		if pa.Align32 > maxCaseAlign32 {
			maxCaseAlign32 = pa.Align32
		}
		if pa.Align64 > maxCaseAlign64 {
			maxCaseAlign64 = pa.Align64
		}
		if pa.Size32 > maxCaseSize32 {
			maxCaseSize32 = pa.Size32
		}
		if pa.Size64 > maxCaseSize64 {
			maxCaseSize64 = pa.Size64
		}
		if pa.FlattenCount > maxCaseFlatten {
			maxCaseFlatten = pa.FlattenCount
		}
	}

	align32 := discA
	if maxCaseAlign32 > align32 {
		align32 = maxCaseAlign32
	}
	align64 := discA
	if maxCaseAlign64 > align64 {
		align64 = maxCaseAlign64
	}

	payloadOffset32 := alignTo(discA, maxCaseAlign32)
	size32 := alignTo(payloadOffset32+maxCaseSize32, align32)
	payloadOffset64 := alignTo(discA, maxCaseAlign64)
	size64 := alignTo(payloadOffset64+maxCaseSize64, align64)

	abi := CanonicalABIInfo{
		Size32: size32, Align32: align32,
		Size64: size64, Align64: align64,
		FlattenCount: 1 + maxCaseFlatten,
	}
	return abi, DiscriminantInfo{
		DiscSize:      disc,
		PayloadOffset: payloadOffset32,
	}
}

// computeListABI is the dynamic-list pointer-pair ABI.
// Spec: definitions.py:1075,1133,1714.
func computeListABI(_ ValType, _ *ComponentTypes) CanonicalABIInfo {
	return CanonicalABIInfo{
		Size32: 8, Align32: 4,
		Size64: 16, Align64: 8,
		FlattenCount: 2,
	}
}

// computeFixedListABI is the inline fixed-length-list ABI.
// Spec: alignment_list at :1082-1085, elem_size_list at :1140-1143,
// flatten_list at :1721-1723.
func computeFixedListABI(elem ValType, length uint32, ct *ComponentTypes) CanonicalABIInfo {
	ea := elem.ABI(ct)
	return CanonicalABIInfo{
		Size32: ea.Size32 * length, Align32: ea.Align32,
		Size64: ea.Size64 * length, Align64: ea.Align64,
		FlattenCount: ea.FlattenCount * int32(length),
	}
}

// computeFlagsABI is the flags-with-N-labels ABI.
// Spec: alignment_flags at :1112-1117, elem_size_flags at :1166-1171.
// Divergence (2): n > 32 is permitted via multi-i32 encoding, matching
// wasmtime's FlagsSize::Size4Plus(n).
func computeFlagsABI(names []string) CanonicalABIInfo {
	n := len(names)
	switch {
	case n <= 0:
		return CanonicalABIInfo{Size32: 0, Align32: 1, Size64: 0, Align64: 1, FlattenCount: 0}
	case n <= 8:
		return CanonicalABIInfo{Size32: 1, Align32: 1, Size64: 1, Align64: 1, FlattenCount: 1}
	case n <= 16:
		return CanonicalABIInfo{Size32: 2, Align32: 2, Size64: 2, Align64: 2, FlattenCount: 1}
	case n <= 32:
		return CanonicalABIInfo{Size32: 4, Align32: 4, Size64: 4, Align64: 4, FlattenCount: 1}
	default:
		words := uint32((n + 31) / 32)
		return CanonicalABIInfo{
			Size32: 4 * words, Align32: 4,
			Size64: 4 * words, Align64: 4,
			FlattenCount: int32(words),
		}
	}
}

// computeEnumABI is enum (discriminant only, no payloads).
func computeEnumABI(names []string) (CanonicalABIInfo, DiscriminantInfo) {
	disc := discriminantSize(len(names))
	d := uint32(disc)
	return CanonicalABIInfo{
			Size32: d, Align32: d,
			Size64: d, Align64: d,
			FlattenCount: 1,
		}, DiscriminantInfo{
			DiscSize:      disc,
			PayloadOffset: d,
		}
}

// computeOptionABI is sugar for variant{none, some(T)}.
func computeOptionABI(elem ValType, ct *ComponentTypes) (CanonicalABIInfo, DiscriminantInfo) {
	return computeVariantABI([]VariantCase{
		{Name: "none", HasPayload: false},
		{Name: "some", Payload: elem, HasPayload: true},
	}, ct)
}

// computeResultABI is sugar for variant{ok(OK), err(Err)} with the
// payloads conditionally present.
func computeResultABI(okT, errT ValType, hasOK, hasErr bool) func(ct *ComponentTypes) (CanonicalABIInfo, DiscriminantInfo) {
	return func(ct *ComponentTypes) (CanonicalABIInfo, DiscriminantInfo) {
		cases := []VariantCase{
			{Name: "ok", Payload: okT, HasPayload: hasOK},
			{Name: "err", Payload: errT, HasPayload: hasErr},
		}
		return computeVariantABI(cases, ct)
	}
}
```

- [ ] **Step 4.4: Run the test to confirm it passes**

```bash
go test ./internal/component/types/... -run 'TestScalarABI|TestScalarABIHandles|TestRecordABI|TestVariantDiscriminantSize|TestFlagsABI|TestListDynamicABI|TestFixedListABI' 2>&1 | head -60
```

Expected: tests still fail at the package level because `composite_test.go`, `val_test.go`, and `val.go` reference deleted/moved symbols. Note any failures specific to `abi_info_test.go` itself and fix them; the cascading errors will resolve in Tasks 5-8.

- [ ] **Step 4.5: Commit**

```bash
git add internal/component/types/abi_info.go internal/component/types/abi_info_test.go
git commit -m "$(cat <<'EOF'
types: add CanonicalABIInfo, DiscriminantInfo, scalar table, computers

New file types/abi_info.go provides CanonicalABIInfo, DiscriminantInfo,
the scalarABI constant table, the (ValType).ABI accessor, and per-kind
compute helpers used by the builder during interning. ABI values match
canonical-ABI spec and wasmtime; the three documented divergences
(empty records, flags > 32 labels, memory64 sizes) match wasmtime.

Tests cover the scalar table, record/variant discriminant sizing,
flags small and >32, dynamic and fixed-length lists.

Design: docs/superpowers/specs/2026-04-07-canonical-abi-type-unification-design.md

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Add `types/builder.go` with `ComponentTypesBuilder` and intern methods

**Design step:** 5 (work order)
**Goal:** Create `types/builder.go` containing `ComponentTypesBuilder`, all `Intern<Kind>` methods with intern-key hashing per the design's "Intern keys per kind (exhaustive)" subsection, `InternFunc` for function types, `InternAbstractResource`, `InternOwnHandle`/`InternBorrowHandle`, and `Finish`. Each `Intern*` call computes a structural hash, scans the bucket for an existing match, and either returns the existing index or appends a new entry with precomputed ABI.
**Design citations:** Builder section (design doc lines 977-1116); Intern keys per kind (design doc lines 1063-1116); V2 verification (design doc lines 1899-1902)

**Files:**
- Create: `internal/component/types/builder.go`
- Create: `internal/component/types/builder_test.go`

**Note:** `FuncTypeIdx` named type lives in `builder.go` since it is the return type of `InternFunc`.

- [ ] **Step 5.1: Write the failing builder dedup test**

Create `internal/component/types/builder_test.go` with:

```go
package types

import "testing"

func TestBuilderInternRecordDedup(t *testing.T) {
	b := NewComponentTypesBuilder()
	a := b.InternRecord([]RecordField{
		{Name: "a", Type: U32},
		{Name: "b", Type: U32},
	})
	c := b.InternRecord([]RecordField{
		{Name: "a", Type: U32},
		{Name: "b", Type: U32},
	})
	if a != c {
		t.Errorf("structurally identical records produced different ValTypes: %v vs %v", a, c)
	}
}

func TestBuilderInternRecordDistinct(t *testing.T) {
	b := NewComponentTypesBuilder()
	// Different field names → distinct
	a := b.InternRecord([]RecordField{{Name: "a", Type: U32}})
	c := b.InternRecord([]RecordField{{Name: "b", Type: U32}})
	if a == c {
		t.Errorf("differently-named records collapsed: %v == %v", a, c)
	}
	// Different field order → distinct
	d := b.InternRecord([]RecordField{
		{Name: "a", Type: U32},
		{Name: "b", Type: U32},
	})
	e := b.InternRecord([]RecordField{
		{Name: "b", Type: U32},
		{Name: "a", Type: U32},
	})
	if d == e {
		t.Errorf("reordered-field records collapsed: %v == %v", d, e)
	}
}

func TestBuilderInternListVsFixedList(t *testing.T) {
	b := NewComponentTypesBuilder()
	dynList := b.InternList(U32)
	fixedList5 := b.InternFixedLengthList(U32, 5)
	fixedList7 := b.InternFixedLengthList(U32, 7)
	if dynList.Kind != TypeKindList {
		t.Errorf("dynList.Kind = %v, want TypeKindList", dynList.Kind)
	}
	if fixedList5.Kind != TypeKindFixedList {
		t.Errorf("fixedList5.Kind = %v, want TypeKindFixedList", fixedList5.Kind)
	}
	if dynList == fixedList5 {
		t.Errorf("dynamic and fixed-length list collapsed: %v == %v", dynList, fixedList5)
	}
	if fixedList5 == fixedList7 {
		t.Errorf("fixed lists with different lengths collapsed: %v == %v", fixedList5, fixedList7)
	}
	dynList2 := b.InternList(U32)
	if dynList != dynList2 {
		t.Errorf("dynamic list dedup failed: %v vs %v", dynList, dynList2)
	}
	fixedList5b := b.InternFixedLengthList(U32, 5)
	if fixedList5 != fixedList5b {
		t.Errorf("fixed-length list dedup failed: %v vs %v", fixedList5, fixedList5b)
	}
}

func TestBuilderInternResultDistinct(t *testing.T) {
	b := NewComponentTypesBuilder()
	rA := b.InternResult(U32, ValType{}, true, false)
	rB := b.InternResult(ValType{}, U32, false, true)
	rC := b.InternResult(U32, U32, true, true)
	rD := b.InternResult(ValType{}, ValType{}, false, false)
	if rA == rB || rA == rC || rA == rD || rB == rC || rB == rD || rC == rD {
		t.Errorf("results with different has-flags collapsed: %v %v %v %v", rA, rB, rC, rD)
	}
}

func TestBuilderInternTupleDedup(t *testing.T) {
	b := NewComponentTypesBuilder()
	a := b.InternTuple([]ValType{U32, U32})
	c := b.InternTuple([]ValType{U32, U32})
	if a != c {
		t.Errorf("identical tuples not deduped: %v vs %v", a, c)
	}
}

func TestBuilderInternFlagsDedup(t *testing.T) {
	b := NewComponentTypesBuilder()
	a := b.InternFlags([]string{"x", "y"})
	c := b.InternFlags([]string{"x", "y"})
	if a != c {
		t.Errorf("identical flags not deduped: %v vs %v", a, c)
	}
	d := b.InternFlags([]string{"y", "x"})
	if a == d {
		t.Errorf("reordered flags collapsed: %v == %v", a, d)
	}
}

func TestBuilderInternAbstractResourceDoesNotDedup(t *testing.T) {
	b := NewComponentTypesBuilder()
	a := b.InternAbstractResource()
	c := b.InternAbstractResource()
	// Each call returns a fresh ResourceTableIdx — abstract resource
	// declarations are distinct by construction.
	if a == c {
		t.Errorf("InternAbstractResource collapsed two calls: %v == %v", a, c)
	}
}

func TestBuilderFinishFreezesBuilder(t *testing.T) {
	b := NewComponentTypesBuilder()
	b.InternRecord([]RecordField{{Name: "a", Type: U32}})
	ct := b.Finish()
	if ct == nil {
		t.Fatal("Finish returned nil")
	}
	if len(ct.Records) != 1 {
		t.Errorf("len(ct.Records) = %d, want 1", len(ct.Records))
	}
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("post-Finish InternRecord did not panic")
		}
	}()
	b.InternRecord([]RecordField{{Name: "b", Type: U32}})
}

func TestBuilderInternFunc(t *testing.T) {
	b := NewComponentTypesBuilder()
	params := b.InternTuple([]ValType{U32, S32})
	results := b.InternTuple([]ValType{Bool})
	idx := b.InternFunc(false, []string{"a", "b"}, params, results)
	idx2 := b.InternFunc(false, []string{"a", "b"}, params, results)
	if idx != idx2 {
		t.Errorf("InternFunc dedup failed: %v vs %v", idx, idx2)
	}
	// Different parameter names → distinct
	idx3 := b.InternFunc(false, []string{"x", "y"}, params, results)
	if idx == idx3 {
		t.Errorf("InternFunc collapsed differently-named params: %v == %v", idx, idx3)
	}
}

func TestBuilderRecordABIPrecomputed(t *testing.T) {
	// Verify that Intern* populates the ABI field correctly.
	b := NewComponentTypesBuilder()
	r := b.InternRecord([]RecordField{
		{Name: "a", Type: U32},
		{Name: "b", Type: U32},
	})
	ct := b.Finish()
	abi := ct.Records[r.Index].ABI
	if abi.Size32 != 8 || abi.Align32 != 4 || abi.FlattenCount != 2 {
		t.Errorf("interned record ABI = {%d/%d/%d}, want {8/4/2}",
			abi.Size32, abi.Align32, abi.FlattenCount)
	}
}
```

- [ ] **Step 5.2: Run the test to confirm it fails**

```bash
go test ./internal/component/types/... -run 'TestBuilder' 2>&1 | head -40
```

Expected: compile errors (`undefined: NewComponentTypesBuilder`, `undefined: FuncTypeIdx`).

- [ ] **Step 5.3: Create `types/builder.go`**

Create `internal/component/types/builder.go` with the body below. Note the use of FNV-1a-style hashing inline (via `hash/fnv`) for bucket keys, with structural equality checks on bucket scan to handle collisions.

```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import (
	"encoding/binary"
	"hash/fnv"
)

// FuncTypeIdx is the index of a TypeFunc in ComponentTypes.Funcs.
type FuncTypeIdx uint32

// ComponentTypesBuilder assembles a *ComponentTypes during decoding.
// After Finish() the builder is consumed; further Intern* calls panic.
// Go equivalent of Rust's "consumed self" idiom. The returned
// *ComponentTypes is safe for concurrent reads.
//
// Spec / wasmtime parity: ComponentTypesBuilder mirrors wasmtime's
// types_builder.rs:38-124. Each Intern method computes a structural
// hash, scans the bucket for an existing entry, and either returns
// the existing index or appends a new entry with precomputed ABI.
type ComponentTypesBuilder struct {
	ct       ComponentTypes
	finished bool

	recordIntern        map[uint64][]uint32
	variantIntern       map[uint64][]uint32
	listIntern          map[uint64][]uint32
	fixedListIntern     map[uint64][]uint32
	tupleIntern         map[uint64][]uint32
	flagsIntern         map[uint64][]uint32
	enumIntern          map[uint64][]uint32
	optionIntern        map[uint64][]uint32
	resultIntern        map[uint64][]uint32
	streamIntern        map[uint64][]uint32
	futureIntern        map[uint64][]uint32
	errCtxIntern        map[uint64][]uint32
	funcIntern          map[uint64][]uint32
}

// NewComponentTypesBuilder creates an empty builder.
func NewComponentTypesBuilder() *ComponentTypesBuilder {
	return &ComponentTypesBuilder{
		recordIntern:    map[uint64][]uint32{},
		variantIntern:   map[uint64][]uint32{},
		listIntern:      map[uint64][]uint32{},
		fixedListIntern: map[uint64][]uint32{},
		tupleIntern:     map[uint64][]uint32{},
		flagsIntern:     map[uint64][]uint32{},
		enumIntern:      map[uint64][]uint32{},
		optionIntern:    map[uint64][]uint32{},
		resultIntern:    map[uint64][]uint32{},
		streamIntern:    map[uint64][]uint32{},
		futureIntern:    map[uint64][]uint32{},
		errCtxIntern:    map[uint64][]uint32{},
		funcIntern:      map[uint64][]uint32{},
	}
}

func (b *ComponentTypesBuilder) panicIfFinished() {
	if b.finished {
		panic("ComponentTypesBuilder: Intern* called after Finish")
	}
}

// --- hashing helpers ---

func hashU32(h *fnv.Hash64, v uint32) {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], v)
	(*h).Write(buf[:])
}

func hashU8(h *fnv.Hash64, v uint8) {
	(*h).Write([]byte{v})
}

func hashString(h *fnv.Hash64, s string) {
	hashU32(h, uint32(len(s)))
	(*h).Write([]byte(s))
}

func hashValType(h *fnv.Hash64, v ValType) {
	hashU8(h, uint8(v.Kind))
	hashU32(h, v.Index)
}

func newHash() *fnv.Hash64 {
	h := fnv.New64a()
	return &h
}

// --- record ---

func (b *ComponentTypesBuilder) InternRecord(fields []RecordField) ValType {
	b.panicIfFinished()
	h := newHash()
	hashU32(h, uint32(len(fields)))
	for _, f := range fields {
		hashString(h, f.Name)
		hashValType(h, f.Type)
	}
	key := (*h).Sum64()
	for _, idx := range b.recordIntern[key] {
		if recordsEqual(b.ct.Records[idx].Fields, fields) {
			return ValType{Kind: TypeKindRecord, Index: idx}
		}
	}
	abi := computeRecordABI(fields, &b.ct)
	idx := uint32(len(b.ct.Records))
	b.ct.Records = append(b.ct.Records, TypeRecord{Fields: append([]RecordField(nil), fields...), ABI: abi})
	b.recordIntern[key] = append(b.recordIntern[key], idx)
	return ValType{Kind: TypeKindRecord, Index: idx}
}

func recordsEqual(a, b []RecordField) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].Type != b[i].Type {
			return false
		}
	}
	return true
}

// --- variant ---

func (b *ComponentTypesBuilder) InternVariant(cases []VariantCase) ValType {
	b.panicIfFinished()
	h := newHash()
	hashU32(h, uint32(len(cases)))
	for _, c := range cases {
		hashString(h, c.Name)
		if c.HasPayload {
			hashU8(h, 1)
			hashValType(h, c.Payload)
		} else {
			hashU8(h, 0)
		}
	}
	key := (*h).Sum64()
	for _, idx := range b.variantIntern[key] {
		if variantsEqual(b.ct.Variants[idx].Cases, cases) {
			return ValType{Kind: TypeKindVariant, Index: idx}
		}
	}
	abi, disc := computeVariantABI(cases, &b.ct)
	idx := uint32(len(b.ct.Variants))
	b.ct.Variants = append(b.ct.Variants, TypeVariant{
		Cases: append([]VariantCase(nil), cases...),
		ABI:   abi,
		Disc:  disc,
	})
	b.variantIntern[key] = append(b.variantIntern[key], idx)
	return ValType{Kind: TypeKindVariant, Index: idx}
}

func variantsEqual(a, b []VariantCase) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].HasPayload != b[i].HasPayload {
			return false
		}
		if a[i].HasPayload && a[i].Payload != b[i].Payload {
			return false
		}
	}
	return true
}

// --- list ---

func (b *ComponentTypesBuilder) InternList(elem ValType) ValType {
	b.panicIfFinished()
	h := newHash()
	hashValType(h, elem)
	key := (*h).Sum64()
	for _, idx := range b.listIntern[key] {
		if b.ct.Lists[idx].Element == elem {
			return ValType{Kind: TypeKindList, Index: idx}
		}
	}
	abi := computeListABI(elem, &b.ct)
	idx := uint32(len(b.ct.Lists))
	b.ct.Lists = append(b.ct.Lists, TypeList{Element: elem, ABI: abi})
	b.listIntern[key] = append(b.listIntern[key], idx)
	return ValType{Kind: TypeKindList, Index: idx}
}

// --- fixed-length list ---

func (b *ComponentTypesBuilder) InternFixedLengthList(elem ValType, length uint32) ValType {
	b.panicIfFinished()
	h := newHash()
	hashValType(h, elem)
	hashU32(h, length)
	key := (*h).Sum64()
	for _, idx := range b.fixedListIntern[key] {
		fl := &b.ct.FixedLists[idx]
		if fl.Element == elem && fl.Length == length {
			return ValType{Kind: TypeKindFixedList, Index: idx}
		}
	}
	abi := computeFixedListABI(elem, length, &b.ct)
	idx := uint32(len(b.ct.FixedLists))
	b.ct.FixedLists = append(b.ct.FixedLists, TypeFixedLengthList{
		Element: elem, Length: length, ABI: abi,
	})
	b.fixedListIntern[key] = append(b.fixedListIntern[key], idx)
	return ValType{Kind: TypeKindFixedList, Index: idx}
}

// --- tuple ---

func (b *ComponentTypesBuilder) InternTuple(elems []ValType) ValType {
	b.panicIfFinished()
	h := newHash()
	hashU32(h, uint32(len(elems)))
	for _, e := range elems {
		hashValType(h, e)
	}
	key := (*h).Sum64()
	for _, idx := range b.tupleIntern[key] {
		if valTypesEqual(b.ct.Tuples[idx].Types, elems) {
			return ValType{Kind: TypeKindTuple, Index: idx}
		}
	}
	abi := computeTupleABI(elems, &b.ct)
	idx := uint32(len(b.ct.Tuples))
	b.ct.Tuples = append(b.ct.Tuples, TypeTuple{
		Types: append([]ValType(nil), elems...),
		ABI:   abi,
	})
	b.tupleIntern[key] = append(b.tupleIntern[key], idx)
	return ValType{Kind: TypeKindTuple, Index: idx}
}

func valTypesEqual(a, b []ValType) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// --- flags ---

func (b *ComponentTypesBuilder) InternFlags(names []string) ValType {
	b.panicIfFinished()
	h := newHash()
	hashU32(h, uint32(len(names)))
	for _, n := range names {
		hashString(h, n)
	}
	key := (*h).Sum64()
	for _, idx := range b.flagsIntern[key] {
		if stringsEqual(b.ct.Flags[idx].Names, names) {
			return ValType{Kind: TypeKindFlags, Index: idx}
		}
	}
	abi := computeFlagsABI(names)
	idx := uint32(len(b.ct.Flags))
	b.ct.Flags = append(b.ct.Flags, TypeFlags{
		Names: append([]string(nil), names...),
		ABI:   abi,
	})
	b.flagsIntern[key] = append(b.flagsIntern[key], idx)
	return ValType{Kind: TypeKindFlags, Index: idx}
}

func stringsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// --- enum ---

func (b *ComponentTypesBuilder) InternEnum(names []string) ValType {
	b.panicIfFinished()
	h := newHash()
	hashU32(h, uint32(len(names)))
	for _, n := range names {
		hashString(h, n)
	}
	key := (*h).Sum64()
	for _, idx := range b.enumIntern[key] {
		if stringsEqual(b.ct.Enums[idx].Names, names) {
			return ValType{Kind: TypeKindEnum, Index: idx}
		}
	}
	abi, disc := computeEnumABI(names)
	idx := uint32(len(b.ct.Enums))
	b.ct.Enums = append(b.ct.Enums, TypeEnum{
		Names: append([]string(nil), names...),
		ABI:   abi,
		Disc:  disc,
	})
	b.enumIntern[key] = append(b.enumIntern[key], idx)
	return ValType{Kind: TypeKindEnum, Index: idx}
}

// --- option ---

func (b *ComponentTypesBuilder) InternOption(elem ValType) ValType {
	b.panicIfFinished()
	h := newHash()
	hashValType(h, elem)
	key := (*h).Sum64()
	for _, idx := range b.optionIntern[key] {
		if b.ct.Options[idx].Element == elem {
			return ValType{Kind: TypeKindOption, Index: idx}
		}
	}
	abi, disc := computeOptionABI(elem, &b.ct)
	idx := uint32(len(b.ct.Options))
	b.ct.Options = append(b.ct.Options, TypeOption{
		Element: elem, ABI: abi, Disc: disc,
	})
	b.optionIntern[key] = append(b.optionIntern[key], idx)
	return ValType{Kind: TypeKindOption, Index: idx}
}

// --- result ---

func (b *ComponentTypesBuilder) InternResult(okType, errType ValType, hasOk, hasErr bool) ValType {
	b.panicIfFinished()
	h := newHash()
	if hasOk {
		hashU8(h, 1)
		hashValType(h, okType)
	} else {
		hashU8(h, 0)
	}
	if hasErr {
		hashU8(h, 1)
		hashValType(h, errType)
	} else {
		hashU8(h, 0)
	}
	key := (*h).Sum64()
	for _, idx := range b.resultIntern[key] {
		r := &b.ct.Results[idx]
		if r.HasOK == hasOk && r.HasErr == hasErr {
			if (!hasOk || r.OK == okType) && (!hasErr || r.Err == errType) {
				return ValType{Kind: TypeKindResult, Index: idx}
			}
		}
	}
	abi, disc := computeResultABI(okType, errType, hasOk, hasErr)(&b.ct)
	idx := uint32(len(b.ct.Results))
	b.ct.Results = append(b.ct.Results, TypeResult{
		OK: okType, Err: errType, HasOK: hasOk, HasErr: hasErr,
		ABI: abi, Disc: disc,
	})
	b.resultIntern[key] = append(b.resultIntern[key], idx)
	return ValType{Kind: TypeKindResult, Index: idx}
}

// --- stream / future / error-context ---

func (b *ComponentTypesBuilder) InternStream(elem ValType, hasElem bool) ValType {
	b.panicIfFinished()
	h := newHash()
	if hasElem {
		hashU8(h, 1)
		hashValType(h, elem)
	} else {
		hashU8(h, 0)
	}
	key := (*h).Sum64()
	for _, idx := range b.streamIntern[key] {
		s := &b.ct.Streams[idx]
		if s.HasElement == hasElem && (!hasElem || s.Element == elem) {
			return ValType{Kind: TypeKindStream, Index: idx}
		}
	}
	idx := uint32(len(b.ct.Streams))
	b.ct.Streams = append(b.ct.Streams, TypeStream{Element: elem, HasElement: hasElem})
	b.streamIntern[key] = append(b.streamIntern[key], idx)
	return ValType{Kind: TypeKindStream, Index: idx}
}

func (b *ComponentTypesBuilder) InternFuture(elem ValType, hasElem bool) ValType {
	b.panicIfFinished()
	h := newHash()
	if hasElem {
		hashU8(h, 1)
		hashValType(h, elem)
	} else {
		hashU8(h, 0)
	}
	key := (*h).Sum64()
	for _, idx := range b.futureIntern[key] {
		f := &b.ct.Futures[idx]
		if f.HasElement == hasElem && (!hasElem || f.Element == elem) {
			return ValType{Kind: TypeKindFuture, Index: idx}
		}
	}
	idx := uint32(len(b.ct.Futures))
	b.ct.Futures = append(b.ct.Futures, TypeFuture{Element: elem, HasElement: hasElem})
	b.futureIntern[key] = append(b.futureIntern[key], idx)
	return ValType{Kind: TypeKindFuture, Index: idx}
}

func (b *ComponentTypesBuilder) InternErrorContextTable() ValType {
	b.panicIfFinished()
	// Single canonical entry — no key.
	if len(b.ct.ErrorContextTables) == 0 {
		b.ct.ErrorContextTables = append(b.ct.ErrorContextTables, TypeErrorContextTable{})
	}
	return ValType{Kind: TypeKindErrorContext, Index: 0}
}

// --- resource handles ---

// InternAbstractResource creates a new Abstract TypeResourceTable entry
// and returns the index. Each call returns a fresh index — abstract
// resource declarations are distinct by construction. Concrete promotion
// at instantiation time is Session 2 work.
func (b *ComponentTypesBuilder) InternAbstractResource() ResourceTableIdx {
	b.panicIfFinished()
	idx := uint32(len(b.ct.ResourceTables))
	b.ct.ResourceTables = append(b.ct.ResourceTables, TypeResourceTable{
		Concrete:    false,
		AbstractIdx: idx,
	})
	return ResourceTableIdx(idx)
}

func (b *ComponentTypesBuilder) InternOwnHandle(rtIdx ResourceTableIdx) ValType {
	b.panicIfFinished()
	return ValType{Kind: TypeKindOwn, Index: uint32(rtIdx)}
}

func (b *ComponentTypesBuilder) InternBorrowHandle(rtIdx ResourceTableIdx) ValType {
	b.panicIfFinished()
	return ValType{Kind: TypeKindBorrow, Index: uint32(rtIdx)}
}

// --- function types ---

func (b *ComponentTypesBuilder) InternFunc(async bool, paramNames []string, params, results ValType) FuncTypeIdx {
	b.panicIfFinished()
	h := newHash()
	if async {
		hashU8(h, 1)
	} else {
		hashU8(h, 0)
	}
	hashU32(h, uint32(len(paramNames)))
	for _, n := range paramNames {
		hashString(h, n)
	}
	hashValType(h, params)
	hashValType(h, results)
	key := (*h).Sum64()
	for _, idx := range b.funcIntern[key] {
		f := &b.ct.Funcs[idx]
		if f.Async == async && stringsEqual(f.ParamNames, paramNames) &&
			f.Params == params && f.Results == results {
			return FuncTypeIdx(idx)
		}
	}
	idx := uint32(len(b.ct.Funcs))
	b.ct.Funcs = append(b.ct.Funcs, TypeFunc{
		Async:      async,
		ParamNames: append([]string(nil), paramNames...),
		Params:     params,
		Results:    results,
	})
	b.funcIntern[key] = append(b.funcIntern[key], idx)
	return FuncTypeIdx(idx)
}

// --- Finish ---

// Finish freezes the builder and returns the immutable *ComponentTypes.
// After Finish, further Intern* calls panic. The intern maps are nilled
// out so the returned *ComponentTypes carries only the slices.
func (b *ComponentTypesBuilder) Finish() *ComponentTypes {
	b.panicIfFinished()
	b.finished = true
	out := b.ct
	// Drop intern maps so the returned ComponentTypes is cheap to retain.
	b.recordIntern = nil
	b.variantIntern = nil
	b.listIntern = nil
	b.fixedListIntern = nil
	b.tupleIntern = nil
	b.flagsIntern = nil
	b.enumIntern = nil
	b.optionIntern = nil
	b.resultIntern = nil
	b.streamIntern = nil
	b.futureIntern = nil
	b.errCtxIntern = nil
	b.funcIntern = nil
	return &out
}
```

- [ ] **Step 5.4: Run the test to confirm it passes**

```bash
go test ./internal/component/types/... -run 'TestBuilder' 2>&1 | head -60
```

Expected: builder tests pass; package-wide tests still fail because `composite_test.go`/`val.go` reference deleted symbols. That's resolved in Tasks 6-8.

- [ ] **Step 5.5: Commit**

```bash
git add internal/component/types/builder.go internal/component/types/builder_test.go
git commit -m "$(cat <<'EOF'
types: add ComponentTypesBuilder with intern methods

New file types/builder.go provides ComponentTypesBuilder, the FuncTypeIdx
named type, all Intern<Kind> methods (Record/Variant/List/FixedLengthList/
Tuple/Flags/Enum/Option/Result/Stream/Future/ErrorContextTable/Func), the
abstract-resource-only InternAbstractResource, and Finish.

Each Intern* call hashes the structurally significant fields, scans the
hash bucket for an existing entry, and either returns the existing
ValType or appends a new entry with precomputed ABI metadata. Tests
cover dedup, distinctness for nearly-identical inputs (different field
order, different fixed-length, different has-flags), abstract-resource
non-deduping behavior, and the post-Finish freeze.

Design: docs/superpowers/specs/2026-04-07-canonical-abi-type-unification-design.md

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: Update `types/val.go` with new ValKind constants

**Design step:** 6 (work order)
**Goal:** Add `ValKindStream`, `ValKindFuture`, `ValKindErrorContext` constants to `types/val.go` and update `ValKind.String()` to handle them. No constructors are added (the values cannot be constructed yet — async lift/lower is deferred). Existing `ValKind*` constants and constructors are preserved unchanged.
**Design citations:** Decision 4; Core Type Representation file layout (design doc lines 251-258); Created — new symbols (design doc lines 1778-1781)

**Files:**
- Modify: `internal/component/types/val.go`
- Modify: `internal/component/types/val_test.go`

- [ ] **Step 6.1: Read the current `val.go` to find the `ValKind` constant block and `String()` method**

```bash
grep -n 'ValKind\|String()' internal/component/types/val.go | head -40
```

Note the exact line numbers for the constant block and the `String()` method.

- [ ] **Step 6.2: Write the failing test for new ValKind constants**

Append to `internal/component/types/val_test.go` (or add a new test function):

```go
func TestNewValKindConstants(t *testing.T) {
	cases := []struct {
		k    ValKind
		want string
	}{
		{ValKindStream, "stream"},
		{ValKindFuture, "future"},
		{ValKindErrorContext, "error-context"},
	}
	for _, c := range cases {
		if got := c.k.String(); got != c.want {
			t.Errorf("ValKind(%d).String() = %q, want %q", c.k, got, c.want)
		}
	}
}
```

- [ ] **Step 6.3: Run the test to confirm it fails**

```bash
go test ./internal/component/types/... -run TestNewValKindConstants 2>&1 | head -20
```

Expected: undefined `ValKindStream`/`ValKindFuture`/`ValKindErrorContext`.

- [ ] **Step 6.4: Add the new constants and update `String()`**

Locate the `ValKind` `const (...)` block in `val.go` (the existing block ends with `ValKindBorrow`). Append three new entries to the iota sequence:

```go
const (
	ValKindBool ValKind = iota
	// ... existing constants ...
	ValKindBorrow
	ValKindStream
	ValKindFuture
	ValKindErrorContext
)
```

Locate the `String()` method on `ValKind`. Add three new cases:

```go
case ValKindStream:
	return "stream"
case ValKindFuture:
	return "future"
case ValKindErrorContext:
	return "error-context"
```

- [ ] **Step 6.5: Run the test to confirm it passes**

```bash
go test ./internal/component/types/... -run TestNewValKindConstants 2>&1 | head -20
```

Expected: PASS for `TestNewValKindConstants`. Other failures in `val_test.go` may persist if any test references the deleted scalar types — note them but proceed; Task 8 fixes them.

- [ ] **Step 6.6: Commit**

```bash
git add internal/component/types/val.go internal/component/types/val_test.go
git commit -m "$(cat <<'EOF'
types: add ValKindStream/Future/ErrorContext for async parity

Three new ValKind constants are added for symmetry with wasmtime's
Val variants. No constructors yet — async lift/lower is deferred to
a later session and these kinds are reachable only through the new
TypeKind dispatch arms (which trap in Session 0).

Design: docs/superpowers/specs/2026-04-07-canonical-abi-type-unification-design.md

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: Add `runtime/resource_type.go` with pointer-identity `*ResourceType`

**Design step:** 7 (work order)
**Goal:** Create `internal/component/runtime/resource_type.go` with the `ResourceType` struct (pointer identity, fields `Impl *ComponentInstance`, `Dtor *uint32`, `DtorAsync bool`, `DtorCallback *uint32`) and `HasDestructor()` method. Decision 5. This task does NOT yet delete `runtime/resource_type_id.go` — that happens in Task 11 after all callers move off it.
**Design citations:** Decision 5; Resource Identity / Nominal layer subsection (design doc lines 606-673); Created — new files (design doc line 1741)

**Files:**
- Create: `internal/component/runtime/resource_type.go`
- Create: `internal/component/runtime/resource_type_test.go`

**Note:** `ComponentInstance` is forward-referenced — Task 9 creates it. Until then, this file references a name that does not exist. We use a partial-build approach: define `ResourceType` with an exported field whose type is the soon-to-exist `*ComponentInstance`. This forces Task 9 to land before `runtime/` package compiles. The test file is gated to verify field shapes only.

- [ ] **Step 7.1: Write the failing pointer-identity test**

Create `internal/component/runtime/resource_type_test.go` with:

```go
package runtime

import "testing"

func TestResourceTypePointerIdentity(t *testing.T) {
	// Two distinct ResourceType pointers compare unequal even if their
	// fields are identical. Spec: definitions.py:1336, 1345 (Python `is`).
	rA := &ResourceType{}
	rB := &ResourceType{}
	if rA == rB {
		t.Errorf("two distinct *ResourceType pointers are equal")
	}
	rAAlias := rA
	if rA != rAAlias {
		t.Errorf("aliased *ResourceType pointers compare unequal")
	}
}

func TestResourceTypeHasDestructor(t *testing.T) {
	rt := &ResourceType{}
	if rt.HasDestructor() {
		t.Errorf("HasDestructor() = true on default, want false")
	}
	dtorIdx := uint32(7)
	rt.Dtor = &dtorIdx
	if !rt.HasDestructor() {
		t.Errorf("HasDestructor() = false after setting Dtor, want true")
	}
}
```

- [ ] **Step 7.2: Run the test to confirm it fails**

```bash
go test ./internal/component/runtime/... -run TestResourceType 2>&1 | head -20
```

Expected: undefined `ResourceType`.

- [ ] **Step 7.3: Create `runtime/resource_type.go`**

Create `internal/component/runtime/resource_type.go` with:

```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

// ResourceType is the runtime nominal-identity layer for resource types.
// Equality is POINTER EQUALITY — two *ResourceType values refer to the
// same resource type iff they are literally the same pointer. This
// directly matches the spec's `is` check at definitions.py:1345
// (`trap_if(h.rt is not t.rt)`) and at :2147.
//
// One distinct *ResourceType exists per (ResourceIdx, ComponentInstance)
// pair at runtime, allocated when the instance is instantiated and its
// resource declarations are bound. Two instantiations of the same
// component produce TWO distinct *ResourceType objects — a handle minted
// by the first instance is rejected when presented to a function
// expecting the second instance's type.
//
// Spec: definitions.py:351-361 (Python ResourceType class), :1345, :2147.
type ResourceType struct {
	// Impl is the component instance that defines this resource type.
	// Spec field name: impl (definitions.py:352).
	Impl *ComponentInstance

	// Dtor is the core function index of the destructor in the defining
	// instance, or nil if no destructor was declared.
	Dtor *uint32

	// DtorAsync indicates an async destructor (resource opcode 0x3e).
	DtorAsync bool

	// DtorCallback is the callback function index for async destructors.
	DtorCallback *uint32
}

// HasDestructor reports whether this resource type has a destructor.
func (r *ResourceType) HasDestructor() bool { return r.Dtor != nil }
```

This file references `*ComponentInstance`, which is created in Task 9. The package will not compile cleanly until then — but the build is intentionally broken in this stretch.

- [ ] **Step 7.4: Verify the file at least parses**

```bash
gofmt -e internal/component/runtime/resource_type.go
```

Expected: empty output. Actual `go test` will fail until Task 9.

- [ ] **Step 7.5: Commit**

```bash
git add internal/component/runtime/resource_type.go internal/component/runtime/resource_type_test.go
git commit -m "$(cat <<'EOF'
runtime: add pointer-identity *ResourceType

New file runtime/resource_type.go provides the nominal-identity
ResourceType struct. Equality is pointer equality, matching the spec's
Python `is` check at definitions.py:1345.

Field shape: {Impl *ComponentInstance, Dtor *uint32, DtorAsync bool,
DtorCallback *uint32}. The Impl back-pointer is the calling-instance
pivot for the same-instance borrow optimization (CanonicalABI.md:
2677-2683) and for cross-instance type resolution.

Build is intentionally broken until ComponentInstance lands.

Design: docs/superpowers/specs/2026-04-07-canonical-abi-type-unification-design.md

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: Rewrite `types/composite_test.go` and verify Checkpoint A

**Design step:** 8 (work order)
**Goal:** Rewrite or delete the existing `types/composite_test.go` to use the new builder-based API. Add `types_context_test.go`-style coverage for the new ABI accessor. Verify Checkpoint A: `go build ./internal/component/types/...` and `go test ./internal/component/types/...` are both green.
**Design citations:** Tests rewritten for the new shape (design doc lines 1530-1537); V2 (design doc lines 1899-1902); Testing Strategy / New tests added in Session 0

**Files:**
- Modify (full rewrite): `internal/component/types/composite_test.go`

- [ ] **Step 8.1: Replace `composite_test.go` with new builder-based tests**

Overwrite `internal/component/types/composite_test.go` with:

```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import "testing"

// TestComposite_RecordRoundTrip builds a record via the builder, finishes
// the bag, and verifies field shape and ABI.
func TestComposite_RecordRoundTrip(t *testing.T) {
	b := NewComponentTypesBuilder()
	r := b.InternRecord([]RecordField{
		{Name: "x", Type: U32},
		{Name: "y", Type: U32},
	})
	ct := b.Finish()
	if r.Kind != TypeKindRecord {
		t.Fatalf("Kind = %v, want TypeKindRecord", r.Kind)
	}
	rec := ct.Records[r.Index]
	if len(rec.Fields) != 2 {
		t.Fatalf("len(Fields) = %d, want 2", len(rec.Fields))
	}
	if rec.Fields[0].Name != "x" || rec.Fields[1].Name != "y" {
		t.Errorf("field names = [%q,%q], want [x,y]", rec.Fields[0].Name, rec.Fields[1].Name)
	}
	if rec.ABI.Size32 != 8 || rec.ABI.Align32 != 4 {
		t.Errorf("record ABI = {size32=%d,align32=%d}, want {8,4}", rec.ABI.Size32, rec.ABI.Align32)
	}
}

// TestComposite_VariantRoundTrip builds a variant with mixed payload/no-payload
// cases and verifies the discriminant info and ABI.
func TestComposite_VariantRoundTrip(t *testing.T) {
	b := NewComponentTypesBuilder()
	v := b.InternVariant([]VariantCase{
		{Name: "none", HasPayload: false},
		{Name: "some", Payload: U32, HasPayload: true},
	})
	ct := b.Finish()
	if v.Kind != TypeKindVariant {
		t.Fatalf("Kind = %v, want TypeKindVariant", v.Kind)
	}
	variant := ct.Variants[v.Index]
	if variant.Disc.DiscSize != 1 {
		t.Errorf("Disc.DiscSize = %d, want 1", variant.Disc.DiscSize)
	}
	// align(disc=1 to payload-align=4) = 4 → payload offset = 4
	if variant.Disc.PayloadOffset != 4 {
		t.Errorf("Disc.PayloadOffset = %d, want 4", variant.Disc.PayloadOffset)
	}
}

// TestComposite_ListRoundTrip exercises the dynamic list path.
func TestComposite_ListRoundTrip(t *testing.T) {
	b := NewComponentTypesBuilder()
	l := b.InternList(U32)
	ct := b.Finish()
	if l.Kind != TypeKindList {
		t.Fatalf("Kind = %v, want TypeKindList", l.Kind)
	}
	if ct.Lists[l.Index].Element != U32 {
		t.Errorf("Element = %v, want U32", ct.Lists[l.Index].Element)
	}
}

// TestComposite_FixedListRoundTrip exercises the fixed-length list path
// and verifies that fixed lists with different lengths are distinct.
func TestComposite_FixedListRoundTrip(t *testing.T) {
	b := NewComponentTypesBuilder()
	a := b.InternFixedLengthList(U32, 3)
	c := b.InternFixedLengthList(U32, 4)
	ct := b.Finish()
	if a.Kind != TypeKindFixedList || c.Kind != TypeKindFixedList {
		t.Fatalf("kinds = (%v, %v), want both TypeKindFixedList", a.Kind, c.Kind)
	}
	if a == c {
		t.Errorf("fixed lists with different lengths collapsed: %v == %v", a, c)
	}
	if ct.FixedLists[a.Index].Length != 3 || ct.FixedLists[c.Index].Length != 4 {
		t.Errorf("lengths = (%d, %d), want (3, 4)", ct.FixedLists[a.Index].Length, ct.FixedLists[c.Index].Length)
	}
}

// TestComposite_TupleRoundTrip exercises tuples.
func TestComposite_TupleRoundTrip(t *testing.T) {
	b := NewComponentTypesBuilder()
	tup := b.InternTuple([]ValType{U32, S32, Bool})
	ct := b.Finish()
	if tup.Kind != TypeKindTuple {
		t.Fatalf("Kind = %v, want TypeKindTuple", tup.Kind)
	}
	if len(ct.Tuples[tup.Index].Types) != 3 {
		t.Errorf("len(Types) = %d, want 3", len(ct.Tuples[tup.Index].Types))
	}
}

// TestComposite_OptionResultEnumFlags exercises the remaining composites.
func TestComposite_OptionResultEnumFlags(t *testing.T) {
	b := NewComponentTypesBuilder()
	opt := b.InternOption(U32)
	res := b.InternResult(U32, U32, true, true)
	en := b.InternEnum([]string{"red", "green", "blue"})
	fl := b.InternFlags([]string{"r", "w", "x"})
	ct := b.Finish()
	if opt.Kind != TypeKindOption {
		t.Errorf("opt.Kind = %v, want TypeKindOption", opt.Kind)
	}
	if res.Kind != TypeKindResult {
		t.Errorf("res.Kind = %v, want TypeKindResult", res.Kind)
	}
	if en.Kind != TypeKindEnum {
		t.Errorf("en.Kind = %v, want TypeKindEnum", en.Kind)
	}
	if fl.Kind != TypeKindFlags {
		t.Errorf("fl.Kind = %v, want TypeKindFlags", fl.Kind)
	}
	if len(ct.Enums[en.Index].Names) != 3 {
		t.Errorf("len(enum.Names) = %d, want 3", len(ct.Enums[en.Index].Names))
	}
	if len(ct.Flags[fl.Index].Names) != 3 {
		t.Errorf("len(flags.Names) = %d, want 3", len(ct.Flags[fl.Index].Names))
	}
}

// TestComposite_AsyncTypes exercises stream and future tables.
func TestComposite_AsyncTypes(t *testing.T) {
	b := NewComponentTypesBuilder()
	s := b.InternStream(U32, true)
	f := b.InternFuture(U32, true)
	ec := b.InternErrorContextTable()
	ct := b.Finish()
	if s.Kind != TypeKindStream {
		t.Errorf("s.Kind = %v, want TypeKindStream", s.Kind)
	}
	if f.Kind != TypeKindFuture {
		t.Errorf("f.Kind = %v, want TypeKindFuture", f.Kind)
	}
	if ec.Kind != TypeKindErrorContext {
		t.Errorf("ec.Kind = %v, want TypeKindErrorContext", ec.Kind)
	}
	if !ct.Streams[s.Index].HasElement || ct.Streams[s.Index].Element != U32 {
		t.Errorf("stream payload = %v/%v, want true/U32", ct.Streams[s.Index].HasElement, ct.Streams[s.Index].Element)
	}
	if len(ct.ErrorContextTables) != 1 {
		t.Errorf("len(ErrorContextTables) = %d, want 1", len(ct.ErrorContextTables))
	}
}
```

- [ ] **Step 8.2: Verify Checkpoint A — `types/` package green**

```bash
go build ./internal/component/types/... && go test ./internal/component/types/... 2>&1 | tail -30
```

Expected: PASS for all tests in `types/`. If there are compile errors, they are in the new builder/composite/abi files — fix them before commit. The rest of the repo is still broken.

- [ ] **Step 8.3: Commit**

```bash
git add internal/component/types/composite_test.go
git commit -m "$(cat <<'EOF'
types: rewrite composite tests for the new builder API

Replace the interface-based composite_test.go with builder-based
construction and *ComponentTypes-rooted assertions. Coverage spans
records, variants (with discriminant info), dynamic and fixed-length
lists, tuples, option/result/enum/flags, and the async type tables.

Checkpoint A reached: go build/test ./internal/component/types/... is
green. The rest of the repo remains intentionally broken until later
tasks rewrite consumers.

Design: docs/superpowers/specs/2026-04-07-canonical-abi-type-unification-design.md

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 9: Add `runtime/component_instance.go` with single-layer `ComponentInstance`

**Design step:** 7c (work order)
**Goal:** Create `runtime/component_instance.go` with the spec-shaped `ComponentInstance` struct (matching `definitions.py:256-273`) plus its methods (`NewComponentInstance`, `Enter`, `Leave`, `EnterCount`, `IsMayLeave`, `LookupResourceType`, internal `findInstance`). The struct holds `Table *Table`, but `Table` does not yet exist (Task 10 renames `ResourceTable` → `Table`). The forward reference is intentional; the package compiles after Task 10.
**Design citations:** Decision 6; Runtime Instance section (design doc lines 820-958); Created — new files (design doc lines 1742, 1785-1789); V7 (design doc lines 1958-1985)

**Files:**
- Create: `internal/component/runtime/component_instance.go`
- Create: `internal/component/runtime/component_instance_test.go`

- [ ] **Step 9.1: Write the failing ComponentInstance test**

Create `internal/component/runtime/component_instance_test.go` with:

```go
package runtime

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component/types"
)

func TestComponentInstance_NewDefaults(t *testing.T) {
	c := NewComponentInstance(7, nil)
	if c.ID != 7 {
		t.Errorf("ID = %d, want 7", c.ID)
	}
	if c.Parent != nil {
		t.Errorf("Parent = %v, want nil", c.Parent)
	}
	if c.Table == nil {
		t.Errorf("Table = nil, want non-nil")
	}
	if !c.MayLeave {
		t.Errorf("MayLeave = false, want true (definitions.py:270)")
	}
	if c.EnterCount() != 0 {
		t.Errorf("EnterCount = %d, want 0", c.EnterCount())
	}
}

func TestComponentInstance_EnterLeave(t *testing.T) {
	c := NewComponentInstance(0, nil)
	c.Enter()
	c.Enter()
	if c.EnterCount() != 2 {
		t.Errorf("EnterCount = %d, want 2", c.EnterCount())
	}
	c.Leave()
	if c.EnterCount() != 1 {
		t.Errorf("EnterCount = %d, want 1", c.EnterCount())
	}
	c.Leave()
	c.Leave() // extra leave clamps at 0
	if c.EnterCount() != 0 {
		t.Errorf("EnterCount = %d, want 0", c.EnterCount())
	}
}

func TestComponentInstance_IsMayLeave(t *testing.T) {
	c := NewComponentInstance(0, nil)
	if !c.IsMayLeave() {
		t.Errorf("IsMayLeave on fresh instance = false, want true")
	}
	c.Enter()
	if c.IsMayLeave() {
		t.Errorf("IsMayLeave during enter = true, want false")
	}
	c.Leave()
	c.MayLeave = false
	if c.IsMayLeave() {
		t.Errorf("IsMayLeave with MayLeave=false = true, want false")
	}
}

func TestComponentInstance_LookupResourceTypeEmpty(t *testing.T) {
	c := NewComponentInstance(0, nil)
	got := c.LookupResourceType(types.RuntimeComponentInstanceIdx(0), types.ResourceIdx(0))
	if got != nil {
		t.Errorf("LookupResourceType on empty instance = %v, want nil", got)
	}
}

func TestComponentInstance_LookupResourceTypeWalksParents(t *testing.T) {
	parent := NewComponentInstance(1, nil)
	rt := &ResourceType{}
	parent.ResourceTypes = []*ResourceType{rt}

	child := NewComponentInstance(2, parent)
	// Lookup of a resource owned by the parent should walk up.
	got := child.LookupResourceType(types.RuntimeComponentInstanceIdx(1), types.ResourceIdx(0))
	if got != rt {
		t.Errorf("LookupResourceType walked-up = %v, want %v", got, rt)
	}
	// Lookup of a non-existent instance ID returns nil.
	missing := child.LookupResourceType(types.RuntimeComponentInstanceIdx(99), types.ResourceIdx(0))
	if missing != nil {
		t.Errorf("LookupResourceType missing = %v, want nil", missing)
	}
}
```

- [ ] **Step 9.2: Run the test to confirm it fails**

```bash
go test ./internal/component/runtime/... -run TestComponentInstance 2>&1 | head -20
```

Expected: undefined `NewComponentInstance`.

- [ ] **Step 9.3: Create `runtime/component_instance.go`**

Create `internal/component/runtime/component_instance.go` with:

```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import "github.com/tetratelabs/wazero/internal/component/types"

// ComponentInstance is the runtime state for one instantiated component
// (top-level or nested). Matches the spec's ComponentInstance at
// debug-vendored/component-model/design/mvp/canonical-abi/definitions.py:256-273.
//
// One ComponentInstance per instantiation. For nested instantiation,
// Parent points to the parent instance. For top-level instances, Parent
// is nil. Each instance owns its own Table, its own MayLeave flag, and
// its own ResourceTypes pool.
type ComponentInstance struct {
	// ID is a monotonically-assigned runtime instance identifier.
	ID uint32

	// Parent is the parent component instance for nested instantiation,
	// or nil for top-level instances. Spec field: parent
	// (definitions.py:258).
	Parent *ComponentInstance

	// Table is the unified handle table for this instance. Holds
	// resource handles today; streams, futures, error-contexts, and
	// subtasks share this table when async lands. Handle indices are
	// unique across all handle kinds within this instance.
	// Spec: definitions.py:259, class Table at :303-315.
	Table *Table

	// MayLeave is the may_leave flag. Set to false during canon.task.enter
	// and restored after canon.task.exit. Operations like canon.resource.new
	// trap if !MayLeave. Spec: definitions.py:260, 270, 1955, 1973, 2065,
	// 2135, 2143.
	MayLeave bool

	// enterCount tracks Enter()/Leave() nesting for compatibility with
	// wazero's existing enter/leave tracking. Accessed via methods.
	enterCount int

	// ResourceTypes is the nominal resource type identity pool for
	// resource declarations DEFINED by this instance. Indexed by
	// types.ResourceIdx (the resource's position in the component's
	// type section).
	//
	// Each entry is a *ResourceType with POINTER identity. Two handles
	// are the same resource type iff their *ResourceType pointers are
	// equal — matching the spec's `h.rt is t.rt` check at
	// definitions.py:1345.
	//
	// Populated at instantiation time (Session 2). Empty in Session 0;
	// all TypeResourceTable entries are Abstract and resource lift/lower
	// traps before reaching this pool.
	ResourceTypes []*ResourceType

	// Destructors is this instance's destructor registry.
	Destructors *DestructorRegistry

	// Reentrance tracks call-site reentrance for this instance.
	Reentrance *ReentranceTracker
}

// NewComponentInstance creates a new instance with the given ID and
// optional parent. MayLeave starts true per spec definitions.py:270.
func NewComponentInstance(id uint32, parent *ComponentInstance) *ComponentInstance {
	return &ComponentInstance{
		ID:          id,
		Parent:      parent,
		Table:       NewTable(),
		MayLeave:    true,
		Destructors: NewDestructorRegistry(),
		Reentrance:  NewReentranceTracker(),
	}
}

// Enter marks entry into a region; may_leave does not change but
// enterCount increments. Matches wazero's existing InstanceState.Enter.
func (c *ComponentInstance) Enter() { c.enterCount++ }

// Leave decrements enterCount. Paired with Enter.
func (c *ComponentInstance) Leave() {
	if c.enterCount > 0 {
		c.enterCount--
	}
}

// EnterCount returns the current nesting depth.
func (c *ComponentInstance) EnterCount() int { return c.enterCount }

// IsMayLeave returns whether the instance may leave — both MayLeave is
// true and enterCount is zero. Matches wazero's existing MayLeave()
// semantics on the deleted InstanceState.
func (c *ComponentInstance) IsMayLeave() bool {
	return c.MayLeave && c.enterCount == 0
}

// LookupResourceType resolves a (RuntimeComponentInstanceIdx, ResourceIdx)
// pair from a TypeResourceTable entry to the nominal *ResourceType.
// Walks the instance tree to find the defining instance, then returns
// the ResourceTypes[ResourceIdx] entry.
//
// Returns nil if the target instance is not found or the resource type
// slot is not yet populated (Session 0 state — Concrete promotion is
// Session 2 work).
func (c *ComponentInstance) LookupResourceType(
	instanceIdx types.RuntimeComponentInstanceIdx,
	resourceIdx types.ResourceIdx,
) *ResourceType {
	target := c.findInstance(uint32(instanceIdx))
	if target == nil {
		return nil
	}
	if int(resourceIdx) >= len(target.ResourceTypes) {
		return nil
	}
	return target.ResourceTypes[resourceIdx]
}

// findInstance walks the parent chain to find the instance with the
// given ID. Returns nil if not found.
func (c *ComponentInstance) findInstance(id uint32) *ComponentInstance {
	for inst := c; inst != nil; inst = inst.Parent {
		if inst.ID == id {
			return inst
		}
	}
	return nil
}
```

Note: this file references `NewTable()`, `*Table`, `NewDestructorRegistry`, and `NewReentranceTracker`. `NewDestructorRegistry` and `NewReentranceTracker` already exist; `NewTable` and `*Table` are introduced in Task 10.

- [ ] **Step 9.4: Verify the file at least parses**

```bash
gofmt -e internal/component/runtime/component_instance.go
```

Expected: empty output. Actual `go test` will fail until Task 10.

- [ ] **Step 9.5: Commit**

```bash
git add internal/component/runtime/component_instance.go internal/component/runtime/component_instance_test.go
git commit -m "$(cat <<'EOF'
runtime: add single-layer ComponentInstance matching the spec

New file runtime/component_instance.go provides a self-contained
ComponentInstance struct matching definitions.py:256-273:
- ID, Parent, Table, MayLeave, enterCount
- ResourceTypes []*ResourceType for the nominal-identity pool
- Destructors and Reentrance carried per-instance

Methods: NewComponentInstance, Enter, Leave, EnterCount, IsMayLeave,
LookupResourceType (with parent-chain walking).

Build is intentionally broken until Table lands in the next commit.

Design: docs/superpowers/specs/2026-04-07-canonical-abi-type-unification-design.md

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 10: Rename `runtime/resource_table.go` → `runtime/table.go`, rewrite for `*ResourceType`

**Design step:** 7a, 7b (work order)
**Goal:** Rename `runtime/resource_table.go` to `runtime/table.go`. Rename `ResourceTable` type to `Table`. Replace `HandleEntry struct { RT ResourceTypeID; ... }` with `ResourceHandleEntry struct { RT *ResourceType; ... }` satisfying a new `TableEntry` interface. Rewrite `ValidateType`, `GetType`, `NewWithType`, `CreateResourceNewFunc`/`CreateResourceDropFunc`/`CreateResourceRepFunc` and their `WithType`/`WithTrap`/`WithContext` variants to take `*ResourceType` instead of `ResourceTypeID`. Migrate `runtime/resource_table_test.go` to `runtime/table_test.go` with the new signatures.
**Design citations:** Decision 5 (bug fix); Unified handle table subsection (design doc lines 675-744); Renamed files (design doc lines 1751-1754); Bug-fix prose (design doc lines 134-136, 655-672)

**Files:**
- Rename: `internal/component/runtime/resource_table.go` → `internal/component/runtime/table.go`
- Modify: `internal/component/runtime/table.go` (heavy rewrite)
- Rename: `internal/component/runtime/resource_table_test.go` → `internal/component/runtime/table_test.go`
- Modify: `internal/component/runtime/table_test.go` (mechanical migration)

**Note:** This task subsumes design step 7a. The accompanying step 7b (test migration) is integrated rather than a separate task because the tests must change in lockstep with the type signatures.

- [ ] **Step 10.1: Read `runtime/resource_table.go` to understand existing structure**

```bash
go doc -all ./internal/component/runtime/ 2>&1 | grep -A2 'func.*ResourceTable\|type ResourceTable\|type HandleEntry' | head -80
```

Note the existing type signatures so the rewrite preserves the unrelated methods (entries/freeHead machinery, NumLends bookkeeping, BorrowScope tracking) and only changes the `*ResourceType`-typed signatures.

- [ ] **Step 10.2: Rename the source files**

```bash
git mv internal/component/runtime/resource_table.go internal/component/runtime/table.go
git mv internal/component/runtime/resource_table_test.go internal/component/runtime/table_test.go
```

- [ ] **Step 10.3: Rewrite `runtime/table.go`**

Inside the new `runtime/table.go`:

1. Rename the type `ResourceTable` → `Table`. Use `replace_all` semantics or a careful sed-equivalent edit; verify zero remaining `ResourceTable` references in the file.
2. Add the `TableEntry` interface near the top:

```go
// TableEntry is the interface implemented by everything stored in a
// Table. The dynamic type is checked via type assertion at retrieval.
//
// Spec: definitions.py:303-315 (class Table). The unified table holds
// heterogeneous handle kinds — resources today; subtasks/streams/futures/
// error-contexts when async lands.
type TableEntry interface {
	tableEntry()
}
```

3. Replace the existing `HandleEntry` struct (the RT-bearing handle entry) with:

```go
// ResourceHandleEntry is the Table entry type for resource handles.
// Replaces the old HandleEntry{RT ResourceTypeID, ...} struct. RT is
// now a *ResourceType with pointer identity, fixing the cross-instance
// type-index collision bug in the deleted ValidateType.
type ResourceHandleEntry struct {
	RT          *ResourceType
	Rep         any
	Own         bool
	NumLends    uint32
	BorrowScope any
}

func (*ResourceHandleEntry) tableEntry() {}
```

4. Change the `entries []` field in `Table` from `HandleEntry` to `tableEntry` (unexported alias) — or keep it as `[]TableEntry` if there is no legacy reason to alias. The simplest approach: replace all internal `HandleEntry` references with `*ResourceHandleEntry` and change `entries []HandleEntry` → `entries []TableEntry`.

5. Rename `NewResourceTable` → `NewTable`. Update its body to construct `[]TableEntry` (or whatever the chosen storage type is).

6. Replace `NewWithType(rep any, own bool, rtID ResourceTypeID)` with:

```go
// NewResourceHandle inserts a resource handle into the table and returns
// its index. The RT is a *ResourceType pointer for spec-correct identity
// comparisons.
func (t *Table) NewResourceHandle(rep any, own bool, rt *ResourceType) (Handle, error) {
	entry := &ResourceHandleEntry{RT: rt, Rep: rep, Own: own}
	return t.add(entry)
}
```

(Where `t.add(entry TableEntry) (Handle, error)` is the internal append-to-entries helper. If the existing code has `add(entry HandleEntry)` semantics, generalize it to take `TableEntry` and return the handle.)

7. Replace `ValidateType(h Handle, expected ResourceTypeID) error` with:

```go
// ValidateType verifies that the handle h refers to a resource entry
// whose runtime type is the same nominal type as expected. Comparison
// is POINTER equality on *ResourceType — the spec's `is` check at
// definitions.py:1345.
//
// Bug fix: the old ValidateType compared only ResourceTypeID, ignoring
// instance identity. This silently accepted cross-instance handles
// when both happened to share a type-section index.
func (t *Table) ValidateType(h Handle, expected *ResourceType) error {
	entry, err := t.Get(h)
	if err != nil {
		return err
	}
	resEntry, ok := entry.(*ResourceHandleEntry)
	if !ok {
		return ErrInvalidHandle
	}
	if resEntry.RT != expected {
		return fmt.Errorf("%w: wrong resource type", ErrResourceTypeMismatch)
	}
	return nil
}
```

8. Replace `GetType(h Handle) (ResourceTypeID, error)` with:

```go
// GetResourceType returns the nominal type of the resource handle h.
// Returns an error if h is not a resource handle.
func (t *Table) GetResourceType(h Handle) (*ResourceType, error) {
	entry, err := t.Get(h)
	if err != nil {
		return nil, err
	}
	resEntry, ok := entry.(*ResourceHandleEntry)
	if !ok {
		return nil, ErrInvalidHandle
	}
	return resEntry.RT, nil
}
```

9. The `Get(h Handle) (TableEntry, error)` method already returns the typed entry; if it currently returns `HandleEntry`, change its return type to `TableEntry`.

10. Update `CreateResourceNewFunc`, `CreateResourceDropFunc`, `CreateResourceRepFunc` and their `WithType`/`WithTrap`/`WithContext` variants. These currently take `resourceTypeIdx uint32` (or `ResourceTypeID`); change every signature to take `rt *ResourceType` instead. The bodies that previously did `actual != expected` ResourceTypeID compares now do `entry.RT != rt`.

11. Update `NewWithMayLeaveCheck` (if present) — its `*InstanceState` parameter becomes `*ComponentInstance` (forward-referenced via Task 9).

12. Add `Add(entry TableEntry) (Handle, error)` as the public unified-entry add path if not already exposed (it is the natural symmetry of `Get`).

13. The error sentinels `ErrInvalidHandle` and `ErrResourceTypeMismatch` already exist in the file or in `runtime/`; verify and reuse.

- [ ] **Step 10.4: Rewrite `runtime/table_test.go`**

The 1240-line test file uses the old API. Mechanical migration:

- Replace every `NewResourceTable()` → `NewTable()`.
- Replace every `*ResourceTable` → `*Table`.
- Replace every `HandleEntry{RT: ...}` → `&ResourceHandleEntry{RT: ...}`.
- Replace every `ResourceTypeID` parameter or field with `*ResourceType`. Construction sites previously like `NewResourceTypeID(0)` become `&ResourceType{}` (a fresh pointer per type).
- Replace every `tab.NewWithType(rep, own, rtID)` → `tab.NewResourceHandle(rep, own, rt)`.
- Replace every `tab.ValidateType(h, expected)` where `expected` is `ResourceTypeID` with `tab.ValidateType(h, expectedRT)` where `expectedRT` is `*ResourceType`.
- Replace every `tab.GetType(h)` → `tab.GetResourceType(h)`.
- For tests that previously asserted `actual.Index() == expected.Index()`, replace with `actual == expectedPtr` (pointer comparison).
- For tests that exercised the cross-instance bug (if any are present), update them to assert that two distinct `&ResourceType{}` instances do NOT validate against each other even with the same conceptual type-index. If no such tests exist, ADD one:

```go
func TestTable_ValidateType_PointerIdentity(t *testing.T) {
	// Bug fix: two ResourceTypes with the same conceptual type index
	// must NOT validate against each other if their pointers differ.
	// Spec: definitions.py:1345 — `is` check, not value equality.
	tab := NewTable()
	rtA := &ResourceType{}
	rtB := &ResourceType{}
	h, err := tab.NewResourceHandle("rep", true, rtA)
	if err != nil {
		t.Fatalf("NewResourceHandle: %v", err)
	}
	if err := tab.ValidateType(h, rtA); err != nil {
		t.Errorf("ValidateType against same RT: %v, want nil", err)
	}
	if err := tab.ValidateType(h, rtB); err == nil {
		t.Errorf("ValidateType against different RT: nil, want error")
	}
}
```

- [ ] **Step 10.5: Verify Checkpoint B partial — runtime package compiles**

```bash
go build ./internal/component/runtime/... 2>&1 | head -40
```

Expected: most errors gone. Remaining errors (if any): references to `ResourceTypeID` or `ResourceTypeInfo` from other files in `runtime/` (e.g., `subtask.go`, `borrow_scope.go`, `instance_state.go`). Note them — they are addressed in Task 11.

- [ ] **Step 10.6: Run the new and migrated tests**

```bash
go test ./internal/component/runtime/... -run 'TestTable|TestComponentInstance|TestResourceType' 2>&1 | tail -40
```

Expected: tests fail to compile because `instance_state.go` and `resource_type_id.go` still exist and reference old types. That's resolved in Task 11. Note any failures specific to `table_test.go`.

- [ ] **Step 10.7: Commit**

```bash
git add internal/component/runtime/table.go internal/component/runtime/table_test.go
git commit -m "$(cat <<'EOF'
runtime: rename ResourceTable to Table, switch to *ResourceType identity

Rename runtime/resource_table.go -> runtime/table.go and the type
ResourceTable -> Table. Introduce a TableEntry interface; the
existing handle entries become ResourceHandleEntry holding a
*ResourceType (pointer identity) instead of a ResourceTypeID.

ValidateType, GetResourceType, NewResourceHandle, and the
CreateResourceNewFunc family take *ResourceType throughout. The
table can now hold heterogeneous handle kinds (resources today;
subtasks/streams/futures/error-contexts when async lands), matching
the spec's class Table at definitions.py:303-315.

Bug fix: the old ValidateType compared only ResourceTypeID, ignoring
instance identity, which silently accepted cross-instance handles
sharing a type-section index. ValidateType now compares pointers,
matching the spec's `h.rt is t.rt` check at definitions.py:1345.

Design: docs/superpowers/specs/2026-04-07-canonical-abi-type-unification-design.md

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 11: Delete `runtime/instance_state.go` and `runtime/resource_type_id.go`; rewire callers

**Design step:** 7d, 7e (work order)
**Goal:** Delete `runtime/instance_state.go` (its fields/methods are now on `ComponentInstance`), `runtime/instance_state_test.go` (tests migrated to `component_instance_test.go` in Task 9), `runtime/resource_type_id.go`, and `runtime/resource_type_id_test.go`. Update every caller in `runtime/` (`subtask.go`, `borrow_scope.go`, `destructor.go`, `reentrance.go`, etc.) to use `*ComponentInstance` instead of `*InstanceState` and `*ResourceType` instead of `ResourceTypeID`/`ResourceTypeInfo`. Verify Checkpoint B: `go build ./internal/component/runtime/...` and `go test ./internal/component/runtime/...` are green.
**Design citations:** Decision 5; Decision 6; Deleted whole files (design doc lines 1665-1672); runtime deletions (design doc lines 1690-1696)

**Files:**
- Delete: `internal/component/runtime/instance_state.go`
- Delete: `internal/component/runtime/instance_state_test.go`
- Delete: `internal/component/runtime/resource_type_id.go`
- Delete: `internal/component/runtime/resource_type_id_test.go`
- Modify (small): `internal/component/runtime/subtask.go`, `borrow_scope.go`, `destructor.go`, `reentrance.go`, `call_context.go` — wherever `*InstanceState`, `ResourceTypeID`, or `ResourceTypeInfo` appears

- [ ] **Step 11.1: Find all references to symbols being deleted**

```bash
```

Use Grep tool with pattern `InstanceState|ResourceTypeID|ResourceTypeInfo|NewResourceTypeID|InvalidResourceTypeID|NewInstanceState` over `internal/component/runtime/`. Record the file:line list.

- [ ] **Step 11.2: Delete the four files**

```bash
git rm internal/component/runtime/instance_state.go internal/component/runtime/instance_state_test.go internal/component/runtime/resource_type_id.go internal/component/runtime/resource_type_id_test.go
```

- [ ] **Step 11.3: Update each caller to use `*ComponentInstance` / `*ResourceType`**

For each file in the list from Step 11.1 (excluding the now-deleted ones):
- Replace `*InstanceState` parameter type with `*ComponentInstance`.
- Replace `state.MayLeave()` calls with `inst.IsMayLeave()`.
- Replace `state.Enter()` / `state.Leave()` / `state.EnterCount()` / `state.ID()` with the equivalent ComponentInstance method (note: `ID()` becomes the field access `inst.ID`).
- Replace any `state.SetMayLeave(b)` with `inst.MayLeave = b`.
- Replace `ResourceTypeID` with `*ResourceType`.
- Replace `NewResourceTypeID(idx)` with construction of a fresh `&ResourceType{}` only if the call site is genuinely allocating a new identity; in other cases, the caller should be receiving a `*ResourceType` from somewhere else.
- Replace `ResourceTypeInfo` with `*ResourceType` (the composite is gone).

- [ ] **Step 11.4: Verify Checkpoint B — `runtime/` package green**

```bash
go build ./internal/component/runtime/... && go test ./internal/component/runtime/... 2>&1 | tail -40
```

Expected: PASS for all tests in `runtime/`. Compile errors elsewhere (binary/, abi/, component/) persist; that is by design.

- [ ] **Step 11.5: Commit**

```bash
git add -A internal/component/runtime/
git commit -m "$(cat <<'EOF'
runtime: delete InstanceState and ResourceTypeID, rewire callers

Delete runtime/instance_state.go, runtime/instance_state_test.go,
runtime/resource_type_id.go, runtime/resource_type_id_test.go. Their
responsibilities are absorbed into the new ComponentInstance and
*ResourceType pointer-identity types.

Every caller in runtime/ updates: *InstanceState -> *ComponentInstance,
ResourceTypeID -> *ResourceType, ResourceTypeInfo -> *ResourceType.

Checkpoint B reached: runtime/ package builds and tests pass.

Design: docs/superpowers/specs/2026-04-07-canonical-abi-type-unification-design.md

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 12: Strip `component.go` of deleted struct types

**Design step:** 9 (work order)
**Goal:** In `internal/component/component.go`: change `Component.Types []TypeDef` to `Component.Types *types.ComponentTypes`. Delete `Component.TypeIdxToStoredIdx`. Collapse `TypeDef` composite pointer fields. Delete `FuncType`, `NamedValType`, `ValTypeRef`, and the per-kind `*TypeDef` structs (`RecordTypeDef`, `VariantTypeDef`, `ListTypeDef`, `TupleTypeDef`, `FlagsTypeDef`, `EnumTypeDef`, `OptionTypeDef`, `ResultTypeDef`, `StreamTypeDef`, `FutureTypeDef`, `FixedSizeListTypeDef`).
**Design citations:** Decision 7; Deleted symbols (design doc lines 1697-1709); Created — new symbols in existing files (design doc line 1782-1783)

**Files:**
- Modify (large structural rewrite): `internal/component/component.go`

- [ ] **Step 12.1: Find every reference to symbols being deleted, by file**

Use Grep with pattern `\bFuncType\b|\bNamedValType\b|\bValTypeRef\b|\bRecordTypeDef\b|\bVariantTypeDef\b|\bListTypeDef\b|\bTupleTypeDef\b|\bFlagsTypeDef\b|\bEnumTypeDef\b|\bOptionTypeDef\b|\bResultTypeDef\b|\bStreamTypeDef\b|\bFutureTypeDef\b|\bFixedSizeListTypeDef\b|TypeIdxToStoredIdx` over `internal/component/`. Record the file list. The design doc V5 caller audit lists 18 files; this confirms the actual scope.

- [ ] **Step 12.2: Rewrite the affected sections of `component.go`**

In `internal/component/component.go`:

1. Change the `Component` struct field from `Types []TypeDef` to `Types *types.ComponentTypes` (add `import "github.com/tetratelabs/wazero/internal/component/types"` if not present).
2. Delete the `TypeIdxToStoredIdx` field.
3. Rewrite the `TypeDef` struct (currently around lines 213-271) to:

```go
// TypeDef is a component-level type-section entry. The composite content
// has been hoisted into the canonical ComponentTypes table; TypeDef now
// carries only the kind discriminator plus references into that table.
type TypeDef struct {
	Kind TypeDefKind

	// Func is the function type when Kind == TypeDefKindFunc.
	Func *types.TypeFunc

	// Resource is the resource-table index when Kind == TypeDefKindResource.
	// Refers into Component.Types.ResourceTables.
	Resource types.ResourceTableIdx

	// ValType is the value-type reference when Kind == TypeDefKindDefined.
	// Refers into Component.Types via the ValType.Index field.
	ValType types.ValType

	// Instance and Component remain as before (sub-component / sub-instance
	// type declarations).
	Instance  *InstanceTypeDef
	Component *ComponentTypeDef
}
```

4. Delete the `FuncType` struct (currently around lines 342-347).
5. Delete the `NamedValType` struct (currently around lines 349-355).
6. Delete the `ValTypeRef` struct (currently around lines 357-377).
7. Delete the per-kind composite TypeDef structs (`RecordTypeDef`, `VariantTypeDef`, `ListTypeDef`, `TupleTypeDef`, `FlagsTypeDef`, `EnumTypeDef`, `OptionTypeDef`, `ResultTypeDef`, `StreamTypeDef`, `FutureTypeDef`, `FixedSizeListTypeDef`) wherever they appear in this file.
8. Note: this task does NOT touch other files (`linker.go`, `instance.go`, `binary/`, etc.). Those are addressed in Tasks 13-19. The build will be heavily broken from this point.

- [ ] **Step 12.3: Verify the file at least parses**

```bash
gofmt -e internal/component/component.go 2>&1 | head -10
```

Expected: empty output. Cascading errors throughout `internal/component/` are expected.

- [ ] **Step 12.4: Commit**

```bash
git add internal/component/component.go
git commit -m "$(cat <<'EOF'
component: hoist type representation into types.ComponentTypes

Component.Types is now *types.ComponentTypes (one canonical type bag
per top-level component) instead of []TypeDef. Delete TypeIdxToStoredIdx
and the per-kind composite TypeDef structs. Collapse TypeDef to a thin
discriminator carrying *types.TypeFunc / types.ResourceTableIdx /
types.ValType / *InstanceTypeDef / *ComponentTypeDef.

Delete FuncType, NamedValType, ValTypeRef from component/. Function
type metadata now lives on types.TypeFunc; the param-name list is
the ParamNames field, params and results are tuple-shaped ValTypes.

Build is intentionally broken — consumers (binary, linker, instance,
abi, conformance) are rewritten in following commits.

Design: docs/superpowers/specs/2026-04-07-canonical-abi-type-unification-design.md

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 13: Rewrite `binary/` decoder to produce `types.ValType` via the builder

**Design step:** 10 (work order)
**Goal:** Rewrite `internal/component/binary/types.go` so the decoder produces `types.ValType` directly, threading a `*types.ComponentTypesBuilder` through every decode helper. Add a `typeScope` struct for scope-local index tracking with `scopeEntry` discriminating value-type entries from resource-declaration entries. Delete the per-kind composite `TypeDef` structs and their decoders (`decodeRecordTypeDef`, `decodeVariantTypeDef`, etc.); their bodies become inline interning calls in a new `decodeDefinedType` function. Verify Checkpoint C: `go build ./internal/component/binary/...` and `go test ./internal/component/binary/...` are green.
**Design citations:** Decoder Flow section (design doc lines 1118-1318); Decision 9 (resource declarations); V6 (design doc lines 1947-1956)

**Files:**
- Modify (heavy rewrite): `internal/component/binary/types.go`
- Create: `internal/component/binary/scope_test.go`
- Modify: `internal/component/binary/types_test.go`, `internal/component/binary/types_composite_test.go`, `internal/component/binary/types_async_test.go`, `internal/component/binary/decoder_test.go` — adapt expectations to the new shape

- [ ] **Step 13.1: Read the current `types.go` to understand the dispatch**

```bash
```

Use Grep with pattern `func decode[A-Z]\w*` over `internal/component/binary/types.go` with output_mode "content" -n true. Note all the existing decoder functions — every per-kind decoder is being deleted or rewritten.

- [ ] **Step 13.2: Write the failing scope-tracking test**

Create `internal/component/binary/scope_test.go` with:

```go
package binary

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component/types"
)

func TestTypeScope_AppendAndLookupValType(t *testing.T) {
	scope := newTypeScope(nil)
	vt := types.U32
	scope.appendValType(vt)
	if len(scope.entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(scope.entries))
	}
	got := scope.entries[0]
	if got.kind != scopeEntryValType {
		t.Errorf("kind = %v, want scopeEntryValType", got.kind)
	}
	if got.valType != vt {
		t.Errorf("valType = %v, want %v", got.valType, vt)
	}
}

func TestTypeScope_AppendAndLookupResource(t *testing.T) {
	scope := newTypeScope(nil)
	scope.appendResource(types.ResourceTableIdx(7))
	got := scope.entries[0]
	if got.kind != scopeEntryResource {
		t.Errorf("kind = %v, want scopeEntryResource", got.kind)
	}
	if got.resource != types.ResourceTableIdx(7) {
		t.Errorf("resource = %d, want 7", got.resource)
	}
}

func TestTypeScope_ParentChain(t *testing.T) {
	parent := newTypeScope(nil)
	parent.appendValType(types.U32)
	child := newTypeScope(parent)
	if child.parent != parent {
		t.Errorf("child.parent != parent")
	}
	// Outer alias resolution by index N walks the parent chain.
	got := child.parent.entries[0]
	if got.valType != types.U32 {
		t.Errorf("parent[0].valType = %v, want U32", got.valType)
	}
}
```

- [ ] **Step 13.3: Run the failing scope test**

```bash
go test ./internal/component/binary/... -run TestTypeScope 2>&1 | head -10
```

Expected: undefined `newTypeScope`, `scopeEntryValType`, etc.

- [ ] **Step 13.4: Add scope-tracking machinery to `binary/types.go`**

At an appropriate location in `internal/component/binary/types.go`:

```go
// scopeEntryKind discriminates between value-type and resource-declaration
// entries in the scope-local type slice. The binary format uses a single
// flat type-section index space for both kinds; the discriminator catches
// ill-formed inputs at decode time (e.g., own<5> where 5 is a record).
type scopeEntryKind uint8

const (
	scopeEntryValType  scopeEntryKind = iota // value type (record, variant, list, ...)
	scopeEntryResource                       // resource declaration
)

// scopeEntry is one entry in a typeScope, tagged by kind.
type scopeEntry struct {
	kind     scopeEntryKind
	valType  types.ValType          // valid iff kind == scopeEntryValType
	resource types.ResourceTableIdx // valid iff kind == scopeEntryResource
}

// typeScope tracks scope-local type indices during decode. Each scope is
// a flat []scopeEntry — binary scope-local index N corresponds to
// scope.entries[N]. parent chains up the scope hierarchy so `outer`
// aliases resolve across nested scopes.
type typeScope struct {
	entries []scopeEntry
	parent  *typeScope
}

func newTypeScope(parent *typeScope) *typeScope {
	return &typeScope{parent: parent}
}

func (s *typeScope) appendValType(vt types.ValType) {
	s.entries = append(s.entries, scopeEntry{
		kind:    scopeEntryValType,
		valType: vt,
	})
}

func (s *typeScope) appendResource(rtIdx types.ResourceTableIdx) {
	s.entries = append(s.entries, scopeEntry{
		kind:     scopeEntryResource,
		resource: rtIdx,
	})
}
```

- [ ] **Step 13.5: Run the scope test**

```bash
go test ./internal/component/binary/... -run TestTypeScope 2>&1 | tail -20
```

Expected: PASS for `TestTypeScope*`. Other failures persist (cascade from upstream changes).

- [ ] **Step 13.6: Rewrite `decodeValType` and `decodeDefinedType`**

In `internal/component/binary/types.go`:

1. Replace the existing `decodeValType` (currently returning `*ValTypeRef` or similar) with a function whose signature is:

```go
func decodeValType(
	r *bytes.Reader,
	scope *typeScope,
	b *types.ComponentTypesBuilder,
) (types.ValType, error)
```

The body is exactly as in the design doc's "decodeValType" subsection (design doc lines 1147-1213), with one correction noted in V6: use the correct opcode constant name `ValTypeOpcodeFixedSizeList` (not `ValTypeOpcodeFixedList`).

2. Replace the existing `decodeDefinedType` with a function whose signature is:

```go
func decodeDefinedType(
	r *bytes.Reader,
	scope *typeScope,
	b *types.ComponentTypesBuilder,
) (types.ValType, error)
```

Body as in the design doc's "decodeDefinedType" subsection (design doc lines 1217-1257). Each `case ValTypeOpcode<Kind>` calls a new helper `decode<Kind>(r, scope, b)` that reads the binary payload, recursively calls `decodeValType` for child types, and calls the matching `b.Intern<Kind>` method, returning the resulting `types.ValType`.

3. Add the per-kind helpers `decodeRecord`, `decodeVariant`, `decodeList`, `decodeFixedLengthList`, `decodeTuple`, `decodeFlags`, `decodeEnum`, `decodeOption`, `decodeResult`, `decodeStream`, `decodeFuture`. Each is a small function that reads the LEB128 fields, recursively decodes child ValTypes via `decodeValType`, and calls `b.Intern<Kind>(...)`. Use the existing LEB128 reading helpers in the package.

4. Replace `decodeResourceTypeDef` and `decodeResourceTypeDefWithAsync` with a single `decodeResourceDecl(r, scope, b, isAsync)` function as in the design doc's "Resource declarations" subsection (design doc lines 1262-1290). Its result is appended to the scope as a `scopeEntryResource`, NOT as a value type.

5. Delete the per-kind composite `TypeDef` structs (`RecordTypeDef`, `VariantTypeDef`, ...) defined in `binary/types.go`. Delete `binary.TypeDef.Record`, `.Variant`, `.List`, `.Tuple`, `.Flags`, `.Enum`, `.Option`, `.Result`, `.Resource` fields.

6. Update the top-level type-section decoder (`decodeTypeSection` or whatever the current entry point is) to:
   - Construct a `*types.ComponentTypesBuilder` at the start of decoding
   - Construct a `*typeScope` for the top-level scope
   - For each type-section entry, dispatch on the leading byte (defined-type vs func-type vs resource-decl vs instance/component-type) and call the appropriate decoder
   - Append the result to the scope (value types via `appendValType`, resources via `appendResource`)
   - At the end, call `b.Finish()` and store the resulting `*types.ComponentTypes` on the `Component` struct

7. Update `decodeFuncType` to call `b.InternFunc(async, paramNames, paramsTuple, resultsTuple)` and store the resulting `FuncTypeIdx` (or look it up via the builder). Producer side: every reference to `component.FuncType` and `component.NamedValType` in `binary/` becomes a call into the builder.

- [ ] **Step 13.7: Update `binary/` test files for the new shape**

In each of `binary/types_test.go`, `binary/types_composite_test.go`, `binary/types_async_test.go`, `binary/decoder_test.go`:

- Replace assertions on `*RecordTypeDef`/`*VariantTypeDef`/etc. with assertions on `ct.Records[idx].Fields` / `ct.Variants[idx].Cases` / etc. via the resulting `*types.ComponentTypes`.
- Replace `ValTypeRef` literal construction with the equivalent `types.ValType` value.
- Where tests previously asserted that decoding a particular bytes sequence yielded a `*TypeDef` with specific composite field shape, change them to assert that decoding produced a specific `types.ValType` indexing into a specific row of the resulting `*types.ComponentTypes`.

Add scope-resolution tests to `binary/scope_test.go`:

```go
func TestScope_OuterAliasResolution(t *testing.T) {
	// A type defined in a parent scope, aliased into a child scope by
	// outer alias, resolves to the same ValType.
	parent := newTypeScope(nil)
	parent.appendValType(types.U32)
	child := newTypeScope(parent)
	// Simulate an outer alias copying parent[0] into child.
	child.appendValType(parent.entries[0].valType)
	if child.entries[0].valType != types.U32 {
		t.Errorf("aliased valType = %v, want U32", child.entries[0].valType)
	}
}
```

- [ ] **Step 13.8: Verify Checkpoint C — `binary/` package green**

```bash
go build ./internal/component/binary/... && go test ./internal/component/binary/... 2>&1 | tail -40
```

Expected: PASS. If cascading errors persist from `component.go` (e.g., Component struct field access), fix them inline. The `binary/` package depends only on `types/`, `runtime/`, and the part of `component/` that holds the `Component` and `TypeDef` structs (rewritten in Task 12).

- [ ] **Step 13.9: Commit**

```bash
git add internal/component/binary/
git commit -m "$(cat <<'EOF'
binary: rewrite type-section decoder to produce types.ValType

The binary decoder now constructs a *types.ComponentTypesBuilder per
top-level component and walks the type section calling the matching
Intern<Kind> method for each defined type. Returns types.ValType
directly throughout — no intermediate ValTypeRef or per-kind
*TypeDef structs.

Add typeScope / scopeEntry machinery for scope-local type indices,
tagging entries as either value types or resource declarations.
Catches ill-formed input (own<N> where N is a record, type index N
where N is a resource declaration) at decode time.

Replace decodeResourceTypeDef with decodeResourceDecl that interns
an Abstract TypeResourceTable entry and appends it to the scope as
a scopeEntryResource. Concrete promotion is Session 2 work.

Delete the per-kind composite TypeDef structs (RecordTypeDef,
VariantTypeDef, ...) and their decoder helpers. Test files migrate
their assertions to *ComponentTypes lookups.

Checkpoint C reached: binary/ package builds and tests pass.

Design: docs/superpowers/specs/2026-04-07-canonical-abi-type-unification-design.md

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 14: Rewrite `abi/context.go` LiftContext / LowerContext

**Design step:** 11 (work order)
**Goal:** Rewrite `LiftContext` and `LowerContext` in `internal/component/abi/context.go`. Drop `ResourceTable`, `Subtask`, `Instance interface{}`, and the `BorrowScope()` helper method. Add `Types *types.ComponentTypes` and `Instance *runtime.ComponentInstance`. Keep `BorrowScope *runtime.BorrowScope` on `LiftContext` and `CallContext *runtime.CallContext` on `LowerContext`. Update `context_test.go` to construct contexts with the new shape.
**Design citations:** Decision 6; abi/ Consumer Flow / LiftContext and LowerContext (design doc lines 1319-1374); V7 (design doc lines 1987-2008); Modified — behavioral changes (design doc lines 1824-1825)

**Files:**
- Modify: `internal/component/abi/context.go`
- Modify: `internal/component/abi/context_test.go`

- [ ] **Step 14.1: Write the failing context test**

In `internal/component/abi/context_test.go`, add:

```go
func TestLiftContext_NewShape(t *testing.T) {
	ct := &types.ComponentTypes{}
	inst := &runtime.ComponentInstance{}
	ctx := &LiftContext{
		Types:    ct,
		Instance: inst,
	}
	if ctx.Types != ct {
		t.Errorf("LiftContext.Types not set")
	}
	if ctx.Instance != inst {
		t.Errorf("LiftContext.Instance not set")
	}
}

func TestLowerContext_NewShape(t *testing.T) {
	ct := &types.ComponentTypes{}
	inst := &runtime.ComponentInstance{}
	ctx := &LowerContext{
		Types:    ct,
		Instance: inst,
	}
	if ctx.Types != ct {
		t.Errorf("LowerContext.Types not set")
	}
	if ctx.Instance != inst {
		t.Errorf("LowerContext.Instance not set")
	}
}
```

Add the import `"github.com/tetratelabs/wazero/internal/component/types"` to the test file if missing.

- [ ] **Step 14.2: Run to confirm it fails**

```bash
go test ./internal/component/abi/... -run TestLiftContext_NewShape 2>&1 | head -10
```

Expected: undefined `Types`, `Instance`, or compile errors.

- [ ] **Step 14.3: Rewrite the context structs in `abi/context.go`**

Replace the `LiftContext` definition (currently at `context.go:65-71`) with:

```go
// LiftContext provides context for lifting operations. Per-call state
// (BorrowScope) lives directly on the context; per-component (Types)
// and per-instance (Instance) state are pointer references.
//
// Spec: each lift call is performed by a specific instance — the one
// whose may_leave gates the operation, whose table holds new handle
// allocations, and whose identity is compared against resource type
// Impl for the same-instance borrow optimization.
type LiftContext struct {
	Memory      api.Memory
	Opts        *Options
	Types       *types.ComponentTypes
	Instance    *runtime.ComponentInstance
	BorrowScope *runtime.BorrowScope
}
```

Replace the `LowerContext` definition (currently at `context.go:151-167`) and the `BorrowScope()` helper method (currently at `context.go:170-175`) with:

```go
// LowerContext provides context for lowering operations.
//
// Spec: each lower call is performed by a specific instance, just like
// lift. CallContext carries per-call state.
type LowerContext struct {
	Memory      api.Memory
	Opts        *Options
	Realloc     func(oldPtr, oldSize, align, newSize uint32) (uint32, error)
	Types       *types.ComponentTypes
	Instance    *runtime.ComponentInstance
	CallContext *runtime.CallContext
}
```

Delete the `BorrowScope()` helper method on `LowerContext` — it derived from the now-deleted `Subtask` field. Direct field access is used instead.

Add the import `"github.com/tetratelabs/wazero/internal/component/types"` to `context.go` if not present.

- [ ] **Step 14.4: Run the context test**

```bash
go test ./internal/component/abi/... -run 'TestLiftContext_NewShape|TestLowerContext_NewShape' 2>&1 | tail -20
```

Expected: PASS for the two new tests. Other tests in `abi/` will fail because lift/lower bodies still reference the old shapes — that's resolved in Task 15.

- [ ] **Step 14.5: Commit**

```bash
git add internal/component/abi/context.go internal/component/abi/context_test.go
git commit -m "$(cat <<'EOF'
abi: rewrite LiftContext/LowerContext for the new shape

LiftContext and LowerContext now carry:
- Types *types.ComponentTypes (per-component type bag)
- Instance *runtime.ComponentInstance (the calling instance)
- BorrowScope (lift) / CallContext (lower) as per-call state

Drop ResourceTable (use Instance.Table instead), Subtask (async-only),
the interface{} Instance TODO, and the BorrowScope() helper method
that derived from Subtask.

Design: docs/superpowers/specs/2026-04-07-canonical-abi-type-unification-design.md

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 15: Rewrite `abi/lift.go`, `lower.go`, `flatten.go` for kind-switch dispatch

**Design step:** 12 (work order)
**Goal:** Rewrite the dispatch in `LiftFlat`, `LiftHeap`, `LowerFlat`, `LowerHeap`, `FlattenParams`, `FlattenResults`, `flattenType`, `CoreSignature` to switch on `typ.Kind` and read composite content via `ctx.Types.<slice>[typ.Index]`. Add `case TypeKindOwn:` and `case TypeKindBorrow:` arms that inline the resource lookup, validation, and same-instance optimization. Add trap arms for `TypeKindStream`/`TypeKindFuture`/`TypeKindErrorContext`.
**Design citations:** Decision 7; Decision 8; abi/ Consumer Flow / Dispatch shape (design doc lines 1377-1458); Modified — behavioral changes (design doc lines 1826-1830)

**Files:**
- Modify (large rewrite): `internal/component/abi/lift.go`
- Modify (large rewrite): `internal/component/abi/lower.go`
- Modify: `internal/component/abi/flatten.go`

- [ ] **Step 15.1: Read existing `abi/lift.go` to find the dispatch sites**

```bash
```

Use Grep with pattern `func Lift[A-Z]\w*\(|case types\.` over `internal/component/abi/lift.go` with output_mode "content" -n true. Note `LiftFlat` (around line 52) and `LiftHeap` (around line 354).

- [ ] **Step 15.2: Rewrite `LiftFlat`'s dispatch to switch on `typ.Kind`**

In `internal/component/abi/lift.go`, replace `func LiftFlat(...)`'s body. The new body's outline:

```go
func LiftFlat(ctx *LiftContext, typ types.ValType, iter *FlatIter) (types.Val, error) {
	switch typ.Kind {
	case types.TypeKindBool:
		return types.ValBool(iter.NextI32() != 0), nil
	case types.TypeKindS8:
		return types.ValS8(int8(iter.NextI32())), nil
	case types.TypeKindU8:
		return types.ValU8(uint8(iter.NextI32())), nil
	case types.TypeKindS16:
		return types.ValS16(int16(iter.NextI32())), nil
	case types.TypeKindU16:
		return types.ValU16(uint16(iter.NextI32())), nil
	case types.TypeKindS32:
		return types.ValS32(iter.NextI32()), nil
	case types.TypeKindU32:
		return types.ValU32(uint32(iter.NextI32())), nil
	case types.TypeKindS64:
		return types.ValS64(iter.NextI64()), nil
	case types.TypeKindU64:
		return types.ValU64(uint64(iter.NextI64())), nil
	case types.TypeKindF32:
		return types.ValF32(canonicalizeNaN32(iter.NextF32())), nil
	case types.TypeKindF64:
		return types.ValF64(canonicalizeNaN64(iter.NextF64())), nil
	case types.TypeKindChar:
		// existing char-from-i32 logic
	case types.TypeKindString:
		// existing string-from-flat logic via memory and Realloc
	case types.TypeKindRecord:
		rec := &ctx.Types.Records[typ.Index]
		fields := make(map[string]types.Val, len(rec.Fields))
		for _, f := range rec.Fields {
			fv, err := LiftFlat(ctx, f.Type, iter)
			if err != nil {
				return types.Val{}, fmt.Errorf("lift record field %s: %w", f.Name, err)
			}
			fields[f.Name] = fv
		}
		return types.ValRecord(fields), nil
	case types.TypeKindVariant:
		variant := &ctx.Types.Variants[typ.Index]
		// Lift discriminant from flat (size determined by variant.Disc.DiscSize),
		// select case, lift payload if HasPayload.
	case types.TypeKindList:
		list := &ctx.Types.Lists[typ.Index]
		// Lift ptr+len from flat, then lift elements from memory at ptr.
		_ = list
	case types.TypeKindFixedList:
		fl := &ctx.Types.FixedLists[typ.Index]
		// Lift fl.Length elements inline from flat (no ptr+len indirection).
		_ = fl
	case types.TypeKindTuple:
		tup := &ctx.Types.Tuples[typ.Index]
		out := make([]types.Val, len(tup.Types))
		for i, t := range tup.Types {
			v, err := LiftFlat(ctx, t, iter)
			if err != nil {
				return types.Val{}, fmt.Errorf("lift tuple element %d: %w", i, err)
			}
			out[i] = v
		}
		return types.ValTuple(out), nil
	case types.TypeKindFlags:
		fl := &ctx.Types.Flags[typ.Index]
		// Lift packed flag bits from flat (1, 2, or words depending on count).
		_ = fl
	case types.TypeKindEnum:
		en := &ctx.Types.Enums[typ.Index]
		// Lift discriminant, select case name.
		_ = en
	case types.TypeKindOption:
		opt := &ctx.Types.Options[typ.Index]
		// Lift discriminant, optionally lift payload.
		_ = opt
	case types.TypeKindResult:
		res := &ctx.Types.Results[typ.Index]
		// Lift discriminant; lift OK or Err payload depending on selected case.
		_ = res
	case types.TypeKindOwn:
		return liftOwnHandle(ctx, typ, iter)
	case types.TypeKindBorrow:
		return liftBorrowHandle(ctx, typ, iter)
	case types.TypeKindStream, types.TypeKindFuture, types.TypeKindErrorContext:
		return types.Val{}, fmt.Errorf(
			"component-model async types not yet supported: kind=%v", typ.Kind)
	default:
		return types.Val{}, fmt.Errorf("LiftFlat: unknown TypeKind %d", typ.Kind)
	}
	// Unreachable but required by control flow.
	return types.Val{}, fmt.Errorf("LiftFlat: unhandled TypeKind %d", typ.Kind)
}
```

For each composite case, port the existing logic from the interface-based body — the algorithms are unchanged; only the field access changes (`rec := typ.(types.Record); rec.Fields[i]` becomes `rec := &ctx.Types.Records[typ.Index]; rec.Fields[i]`).

Add the helper functions at file scope:

```go
// liftOwnHandle implements the TypeKindOwn lift arm.
//
// Spec: definitions.py:1336-1347 (lift_own).
// In Session 0, ctx.Instance.ResourceTypes is empty (Concrete promotion
// is Session 2 work) and this traps for any real handle. The dispatch
// arm exists, compiles, and traps with a precise error.
func liftOwnHandle(ctx *LiftContext, typ types.ValType, iter *FlatIter) (types.Val, error) {
	rt := ctx.Types.ResourceTables[typ.Index]
	if !rt.Concrete {
		return types.Val{}, fmt.Errorf(
			"cannot lift abstract resource at runtime (type %d)", typ.Index)
	}
	expectedRT := ctx.Instance.LookupResourceType(rt.Instance, rt.Resource)
	if expectedRT == nil {
		return types.Val{}, fmt.Errorf(
			"no resource type for instance %d declaration %d "+
				"(resource concrete promotion not yet wired — session 2)",
			rt.Instance, rt.Resource)
	}
	handleIdx := iter.NextI32()
	h := runtime.Handle(handleIdx)
	if err := ctx.Instance.Table.ValidateType(h, expectedRT); err != nil {
		return types.Val{}, err
	}
	// For own<>: transfer ownership via the table.
	if _, err := ctx.Instance.Table.RemoveResourceHandle(h); err != nil {
		return types.Val{}, err
	}
	return types.ValOwn(uint32(handleIdx)), nil
}

// liftBorrowHandle implements the TypeKindBorrow lift arm.
//
// Spec: definitions.py:1338-1347 (lift_borrow).
func liftBorrowHandle(ctx *LiftContext, typ types.ValType, iter *FlatIter) (types.Val, error) {
	rt := ctx.Types.ResourceTables[typ.Index]
	if !rt.Concrete {
		return types.Val{}, fmt.Errorf(
			"cannot lift abstract resource at runtime (type %d)", typ.Index)
	}
	expectedRT := ctx.Instance.LookupResourceType(rt.Instance, rt.Resource)
	if expectedRT == nil {
		return types.Val{}, fmt.Errorf(
			"no resource type for instance %d declaration %d "+
				"(resource concrete promotion not yet wired — session 2)",
			rt.Instance, rt.Resource)
	}
	handleIdx := iter.NextI32()
	h := runtime.Handle(handleIdx)
	if err := ctx.Instance.Table.ValidateType(h, expectedRT); err != nil {
		return types.Val{}, err
	}
	if err := ctx.Instance.Table.IncrementLends(h); err != nil {
		return types.Val{}, err
	}
	if ctx.BorrowScope != nil {
		ctx.BorrowScope.Add(h)
	}
	return types.ValBorrow(uint32(handleIdx)), nil
}
```

(If the names of helper methods like `RemoveResourceHandle`, `IncrementLends`, `Add` differ from what currently exists in `runtime/table.go` after Task 10, use the actual names — re-grep before edit.)

- [ ] **Step 15.3: Rewrite `LiftHeap` analogously**

`LiftHeap` (around line 354) lifts from a memory offset rather than a flat iterator. Same dispatch shape: switch on `typ.Kind`, read composite content via `ctx.Types.<slice>[typ.Index]`, recurse for nested types, add the same `TypeKindOwn`/`TypeKindBorrow` arms (sharing the same helper functions where the only difference is how the i32 is read — adapt as needed).

- [ ] **Step 15.4: Rewrite `LowerFlat` and `LowerHeap` analogously**

In `internal/component/abi/lower.go`, replace `LowerFlat` (around line 13) and `LowerHeap` (around line 322) with kind-switch dispatch. The Own/Borrow arms inline the previously-standalone `LowerOwn`/`LowerBorrow`/`LowerOwnWithType`/`LowerBorrowWithType` logic. Specifically, for `TypeKindBorrow`, fold in the same-instance optimization from `CanonicalABI.md:2677-2683`:

```go
case types.TypeKindBorrow:
	rt := ctx.Types.ResourceTables[typ.Index]
	if !rt.Concrete {
		return fmt.Errorf("cannot lower abstract resource at runtime (type %d)", typ.Index)
	}
	expectedRT := ctx.Instance.LookupResourceType(rt.Instance, rt.Resource)
	if expectedRT == nil {
		return fmt.Errorf(
			"no resource type for instance %d declaration %d "+
				"(resource concrete promotion not yet wired — session 2)",
			rt.Instance, rt.Resource)
	}
	// Same-instance optimization (CanonicalABI.md:2677-2683): if the
	// calling instance is the one that defined the resource, return
	// rep directly without allocating a new handle. Comparison is
	// pointer identity on *runtime.ComponentInstance.
	if ctx.Instance == expectedRT.Impl {
		// Return the rep directly via the chosen flat encoding.
		// (Implementation detail: the rep is stored in the Val's Borrow
		// field; the lower path emits it as i32.)
	}
	// Cross-instance: allocate a borrow handle in the caller's table.
	h, err := ctx.Instance.Table.NewResourceHandle(rep, false /*own*/, expectedRT)
	if err != nil {
		return err
	}
	flat.PushI32(int32(h))
	return nil
```

Add trap arms for `TypeKindStream`, `TypeKindFuture`, `TypeKindErrorContext` in both `LowerFlat` and `LowerHeap`.

Delete the now-orphaned `LowerOwn` (currently around line 683) and `LowerBorrow` (currently around line 699) functions from `lower.go` — the dispatch arms above subsume them.

- [ ] **Step 15.5: Rewrite `flatten.go` to take `*types.ComponentTypes`**

In `internal/component/abi/flatten.go`:

- Add `ct *types.ComponentTypes` as a parameter to `FlattenParams`, `FlattenResults`, `flattenType`, `CoreSignature`.
- Convert the dispatch in `flattenType` from `switch typ.(type)` to `switch typ.Kind`.
- The Own/Borrow case (currently at line 82, returning `[]api.ValueType{api.ValueTypeI32}`) becomes `case types.TypeKindOwn, types.TypeKindBorrow: return []api.ValueType{api.ValueTypeI32}`.
- Add `case types.TypeKindStream, types.TypeKindFuture, types.TypeKindErrorContext: return []api.ValueType{api.ValueTypeI32}` (matches the scalar ABI in Task 4).

- [ ] **Step 15.6: Update `abi/lift_test.go`, `lower_test.go`, `flatten_test.go` for the new shape**

Each test that previously constructed types via `types.Record{Fields: ...}` literals now uses:

```go
b := types.NewComponentTypesBuilder()
recT := b.InternRecord([]types.RecordField{{Name: "a", Type: types.U32}})
ct := b.Finish()
ctx := &LiftContext{Types: ct, /* ... */}
LiftFlat(ctx, recT, iter)
```

Tests for primitive scalars do not need a `Types` parameter (scalar ABI is constant-table).

Add a test for the new TypeKindOwn dispatch arm:

```go
func TestLiftFlat_OwnArm_TrapsWhenNoResourceType(t *testing.T) {
	// Session 0: ResourceTypes pool is empty, so any own<> lift traps
	// with the documented Session 2 wiring error.
	b := types.NewComponentTypesBuilder()
	rtIdx := b.InternAbstractResource()
	ownT := b.InternOwnHandle(rtIdx)
	ct := b.Finish()
	ctx := &LiftContext{
		Types:    ct,
		Instance: runtime.NewComponentInstance(0, nil),
	}
	iter := NewFlatIter([]uint64{42 /* fake handle */})
	_, err := LiftFlat(ctx, ownT, iter)
	if err == nil {
		t.Errorf("expected lift to trap, got nil error")
	}
}
```

(Adapt `NewFlatIter` to whatever the actual constructor is in `abi/`.)

Delete the references to `abi.LowerOwn` and `abi.LowerBorrow` from `lower_test.go` — they are now dispatched through `LowerFlat`.

- [ ] **Step 15.7: Verify Checkpoint D — `abi/` package green**

```bash
go build ./internal/component/abi/... && go test ./internal/component/abi/... 2>&1 | tail -40
```

Expected: PASS. If not, the failures are likely in tests still constructing the old types.* literal shapes — fix in place.

- [ ] **Step 15.8: Commit**

```bash
git add internal/component/abi/lift.go internal/component/abi/lower.go internal/component/abi/flatten.go internal/component/abi/lift_test.go internal/component/abi/lower_test.go internal/component/abi/flatten_test.go
git commit -m "$(cat <<'EOF'
abi: switch lift/lower dispatch to TypeKind, add Own/Borrow arms

LiftFlat, LiftHeap, LowerFlat, LowerHeap now switch on typ.Kind and
read composite content via ctx.Types.<slice>[typ.Index]. Add the
previously-missing TypeKindOwn and TypeKindBorrow arms with full
inlined resource-lookup logic, including the same-instance borrow
optimization from CanonicalABI.md:2677-2683.

In Session 0, ResourceTypes is empty (Concrete promotion is Session 2)
so the resource arms trap with a precise error. The dispatch path is
in place and spec-correct; only the data needed to make it succeed
end-to-end is deferred.

Trap arms for TypeKindStream/Future/ErrorContext provide a clear
"async types not yet supported" error rather than falling through to
the unknown-kind default.

Delete LowerOwn/LowerBorrow standalone helpers — folded into dispatch.

flatten.go gains a *types.ComponentTypes parameter and dispatches on
TypeKind with the i32 handle-shape arm covering Own/Borrow/Stream/
Future/ErrorContext.

Checkpoint D reached: abi/ package builds and tests pass.

Design: docs/superpowers/specs/2026-04-07-canonical-abi-type-unification-design.md

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 16: Delete wholesale-removed files (canon_lower, type_resolver, resource_lower)

**Design step:** 13 (work order)
**Goal:** Delete `internal/component/type_resolver.go`, `internal/component/type_resolver_test.go`, `internal/component/canon_lower.go`, `internal/component/canon_lower_test.go`, `internal/component/abi/resource_lower.go`, `internal/component/abi/resource_lower_test.go`. These have already been logically replaced by the dispatch arms in Task 15 and the builder-based decoder in Task 13.
**Design citations:** Decision 9; Deleted whole files (design doc lines 1665-1672)

**Files:**
- Delete: `internal/component/type_resolver.go`
- Delete: `internal/component/type_resolver_test.go`
- Delete: `internal/component/canon_lower.go`
- Delete: `internal/component/canon_lower_test.go`
- Delete: `internal/component/abi/resource_lower.go`
- Delete: `internal/component/abi/resource_lower_test.go`

- [ ] **Step 16.1: Delete the six files**

```bash
git rm internal/component/type_resolver.go internal/component/type_resolver_test.go internal/component/canon_lower.go internal/component/canon_lower_test.go internal/component/abi/resource_lower.go internal/component/abi/resource_lower_test.go
```

- [ ] **Step 16.2: Verify no remaining references to the deleted symbols**

Use Grep with pattern `\bTypeResolver\b|\bcanonLower\b|\bLowerOwnWithType\b|\bLowerBorrowWithType\b|\bresolveDefinedType\b` over `internal/component/`. Expected: zero matches except in the followup note (added in Task 19) and any callers in `instance.go` / `component_linker.go` that still need compile-fix in Task 17.

- [ ] **Step 16.3: Verify `abi/` still builds (canon_lower deletion doesn't affect abi/, but resource_lower deletion does)**

```bash
go build ./internal/component/abi/... 2>&1 | head -20
```

Expected: green. If not, a stray reference to deleted resource-lower symbols persists in `abi/`; remove it.

- [ ] **Step 16.4: Commit**

```bash
git add -A internal/component/
git commit -m "$(cat <<'EOF'
component, abi: delete canon_lower, type_resolver, resource_lower

Delete the six wholesale-replaced files:
- internal/component/type_resolver.go (+ test)
- internal/component/canon_lower.go (+ test) — its private type universe
  and broken lift/lower bodies are replaced by the new abi/ dispatch
- internal/component/abi/resource_lower.go (+ test) — folded into the
  TypeKindOwn/TypeKindBorrow dispatch arms in lift.go and lower.go

Design: docs/superpowers/specs/2026-04-07-canonical-abi-type-unification-design.md

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 17: Compile-fix `component_linker.go` and `instance.go`

**Design step:** 15 (work order)
**Goal:** Make `internal/component/component_linker.go` and `internal/component/instance.go` compile against the new `types.ValType` / `types.TypeFunc` / `runtime.ComponentInstance` shapes. The bodies may be logically wrong; only the compile-green requirement matters. Specifically: delete `resolveValTypeRef`, `resolveToValType`, `typeDefToValType`, `valTypeRefToValType` (currently around lines 701-884 in `component_linker.go`); compile-fix the lift/lower call sites in `instance.go` (currently around lines 794, 1205, 1242, 1518, 1527, 1546, 1601, 2009); update every `*FuncType` reference to `*types.TypeFunc`. The followup note will document the broken bodies for Session 1 deletion.
**Design citations:** Decision 9; Work Order step 15 (design doc line 1519); Modified — behavioral changes (design doc lines 1831-1832); V5 caller audit (design doc lines 1927-1945)

**Files:**
- Modify (large compile-fix): `internal/component/component_linker.go`
- Modify (large compile-fix): `internal/component/instance.go`
- Modify (mechanical rename): every other file in V5's audit list — `linker.go`, `linker_test.go`, `linker_api_test.go`, `instance_test.go`, `instantiate.go`, `type_checker.go`, `type_checker_test.go`, `nested_component_test.go`, `outer_alias_test.go`, `integration_test.go`, `edge_case_test.go`, `conformance/linker_test.go`, `conformance/nested_test.go`

- [ ] **Step 17.1: Find every remaining caller of `*FuncType`, `NamedValType`, `ValTypeRef`**

Use Grep with pattern `\*?\bFuncType\b|\bNamedValType\b|\bValTypeRef\b` over `internal/component/` (excluding the deleted files). Record the file list. Expect ~17 files.

- [ ] **Step 17.2: Mechanical rename — `*FuncType` → `*types.TypeFunc`**

For each file in the list, perform the mechanical rename. Where `NamedValType{Name: "x", ValType: vtRef}` constructions appear, they become contributions to a `(paramNames []string, params types.ValType)` pair: collect names into a separate slice, collect types into a tuple ValType via `b.InternTuple(...)` if a builder is in scope, otherwise restructure the call site to receive the pair separately.

For host-defined function types (declared in tests or linker call sites), the construction pattern becomes:

```go
b := types.NewComponentTypesBuilder()
params := b.InternTuple([]types.ValType{types.U32, types.S32})
results := b.InternTuple([]types.ValType{types.Bool})
funcIdx := b.InternFunc(false, []string{"a", "b"}, params, results)
ct := b.Finish()
ft := &ct.Funcs[funcIdx]
```

(In tests this is verbose; consider a `helpers_test.go` wrapper. The conformance helpers come in Task 18.)

- [ ] **Step 17.3: Delete the four resolver functions in `component_linker.go`**

Delete `resolveValTypeRef`, `resolveToValType`, `typeDefToValType`, `valTypeRefToValType` and any helper functions only used by them. Re-grep before edit to find the current line numbers.

- [ ] **Step 17.4: Compile-fix the `flatten*` helpers in `component_linker.go`**

`coreSignature`, `flattenParams`, `flattenResults`, `flattenValType` (currently around lines 3580-3617) are slated for deletion in Session 1. For Session 0 they only need to compile. Replace their bodies with calls into `abi.FlattenParams(...)`, `abi.FlattenResults(...)`, `abi.CoreSignature(...)` if straightforward, otherwise replace with stub bodies that return reasonable defaults. The Session 1 followup note records that these stubs are wrong and must be deleted.

If a stub body is needed, do NOT use a silent fallback. Use `panic("compile-fix stub: see Session 1 followup note")` so any runtime call traps loudly.

- [ ] **Step 17.5: Compile-fix `instance.go`'s lift/lower sites**

For each line in the design doc's list (`794, 1205, 1242, 1518, 1527, 1546, 1601, 2009`):

- Re-grep for the actual function name at that location (line numbers drift).
- Replace the body with a `panic("instance.go lift/lower path scheduled for Session 1 deletion — see followup note")` if it cannot be cleanly compile-fixed.
- DO NOT introduce silent fallbacks. The followup note documents which functions must be replaced in Session 1.

- [ ] **Step 17.6: Verify Checkpoint E — repo compiles**

```bash
go build ./... 2>&1 | tail -40
```

Expected: green. If errors remain, they fall into one of these categories:
- A caller still references a deleted symbol (`FuncType`, `NamedValType`, `ValTypeRef`, `RecordTypeDef`, etc.) — mechanically rename or restructure.
- A test in `internal/component/` (top-level) constructs `Component` with the old shape — change it to use the new shape or `t.Skip` per Task 19.
- An import cycle has been introduced — verify the dependency direction (`types/` → no deps; `runtime/` → `types/`; `binary/` → `types/` + `runtime/`; `abi/` → `types/` + `runtime/`; `component/` → all of the above).

- [ ] **Step 17.7: Commit**

```bash
git add internal/component/
git commit -m "$(cat <<'EOF'
component: compile-fix linker and instance for the new type shape

Delete resolveValTypeRef, resolveToValType, typeDefToValType,
valTypeRefToValType from component_linker.go. Mechanically rename
*FuncType -> *types.TypeFunc and NamedValType usage -> ParamNames +
tuple-shaped Params/Results pattern across the V5 caller audit (17
files).

instance.go's lift/lower bodies and component_linker.go's flatten
helpers are compile-fixed with panic stubs. They remain logically
broken — the followup note documents which functions Session 1 must
delete and replace with direct abi/ calls.

Checkpoint E reached: go build ./... is green.

Design: docs/superpowers/specs/2026-04-07-canonical-abi-type-unification-design.md

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 18: Migrate the 12 conformance test files

**Design step:** 16 (work order)
**Goal:** Update the 12 files in `internal/component/conformance/` to build types via `ComponentTypesBuilder` and pass `*types.ComponentTypes` + `*runtime.ComponentInstance` through `LiftContext` / `LowerContext`. Add a `helpers_test.go` for shared builder/context construction. Verify Checkpoint F: `go test ./internal/component/conformance/...` is green.
**Design citations:** Tests rewritten for the new shape (design doc line 1537); V4 conformance test migration pattern (design doc lines 1916-1925); New tests added in Session 0 (design doc line 1925)

**Files:**
- Modify: `internal/component/conformance/primitives_test.go`, `composites_test.go`, `strings_test.go`, `abi_edge_cases_test.go`, `type_edge_cases_test.go`, `concurrent_access_test.go`, `nesting_depth_test.go`, `realloc_failure_test.go`, `instance_types_test.go`, `error_messages_test.go`, `utf_validation_test.go`, `memory_bounds_test.go`
- Create: `internal/component/conformance/helpers_test.go`

- [ ] **Step 18.1: Create the shared helper file**

Create `internal/component/conformance/helpers_test.go` with:

```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conformance

import (
	"github.com/tetratelabs/wazero/internal/component/abi"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
)

// newBuilder returns a fresh ComponentTypesBuilder. Test helper to
// keep call sites short.
func newBuilder() *types.ComponentTypesBuilder {
	return types.NewComponentTypesBuilder()
}

// newLiftContext constructs a LiftContext with the given type bag and
// a fresh top-level ComponentInstance. Suitable for tests that do not
// exercise resource handles (Session 0 default).
func newLiftContext(ct *types.ComponentTypes) *abi.LiftContext {
	return &abi.LiftContext{
		Types:    ct,
		Instance: runtime.NewComponentInstance(0, nil),
	}
}

// newLowerContext constructs a LowerContext with the given type bag
// and a fresh top-level ComponentInstance.
func newLowerContext(ct *types.ComponentTypes) *abi.LowerContext {
	return &abi.LowerContext{
		Types:    ct,
		Instance: runtime.NewComponentInstance(0, nil),
	}
}
```

- [ ] **Step 18.2: Migrate each of the 12 conformance test files**

For each file:
1. Replace primitive scalar literals — `types.S32{}` becomes `types.S32`; `types.U32{}` becomes `types.U32`; etc.
2. Replace composite literals:
   - `types.Record{Fields: [...]}` → `b := newBuilder(); recT := b.InternRecord([]types.RecordField{...})`
   - `types.Variant{Cases: [...]}` → `b.InternVariant(...)`
   - `types.List{Element: ...}` → `b.InternList(...)`
   - `types.Tuple{Types: ...}` → `b.InternTuple(...)`
   - `types.Option{Some: ...}` → `b.InternOption(...)`
   - `types.Result{Ok: ..., Error: ...}` → `b.InternResult(okT, errT, hasOk, hasErr)`
   - `types.Flags{Names: ...}` → `b.InternFlags(...)`
   - `types.Enum{Cases: ...}` → `b.InternEnum(...)`
3. After all `Intern*` calls, call `ct := b.Finish()`. Construct contexts via `newLiftContext(ct)` / `newLowerContext(ct)`.
4. Replace ABI property access — `emptyRecord.Size()` becomes `ct.Records[recT.Index].ABI.Size32`. Or use the accessor: `recT.ABI(ct).Size32`.
5. For empty-record divergence (1) testing in `composites_test.go`, the current `TestCompositeRecordEmpty` expecting size 0 is preserved as-is — the new builder produces the same value.
6. Lift/lower calls now pass the constructed context: `abi.LiftFlat(ctx, recT, iter)`, `abi.LowerFlat(ctx, recT, val)`.

Migration is mechanical; do one file at a time and run that file's tests after each migration.

- [ ] **Step 18.3: Verify Checkpoint F — `conformance/` package green**

```bash
go test ./internal/component/conformance/... 2>&1 | tail -30
```

Expected: PASS. If failures, they are likely cosmetic (test expectations against the new ABI values) — fix in place. The new builder's ABI computation is deterministic and verified against the spec in Task 4.

- [ ] **Step 18.4: Commit**

```bash
git add internal/component/conformance/
git commit -m "$(cat <<'EOF'
conformance: migrate 12 test files to the builder-based API

Add helpers_test.go with newBuilder/newLiftContext/newLowerContext
shared constructors. Each of the 12 conformance files (primitives,
composites, strings, abi_edge_cases, type_edge_cases, concurrent_
access, nesting_depth, realloc_failure, instance_types, error_
messages, utf_validation, memory_bounds) now constructs types via
ComponentTypesBuilder and passes *types.ComponentTypes through the
LiftContext/LowerContext.

Scalar literals types.S32{} -> types.S32 named constant; composite
literals types.Record{...} -> b.InternRecord(...) -> ct.Records[idx].

Checkpoint F reached: conformance/ package builds and tests pass.

Design: docs/superpowers/specs/2026-04-07-canonical-abi-type-unification-design.md

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 19: Skip end-to-end tests in top-level `component/` and write the followup note

**Design step:** 17 (work order)
**Goal:** Add `t.Skip("session 1 work: see docs/plans/2026-04-07-canonical-abi-unification-session0-followup.md")` to every top-level `internal/component/*_test.go` test that exercises end-to-end lift/lower through the broken `instance.go` path. Write the followup note at `docs/plans/2026-04-07-canonical-abi-unification-session0-followup.md` covering the deferred work for Sessions 1 and 2 plus the async stub-and-trap sites.
**Design citations:** Tests intentionally left broken (design doc lines 1563-1567); Followup Note Format section (design doc lines 1581-1660)

**Files:**
- Modify: `internal/component/instance_test.go` (add `t.Skip` to lift/lower-exercising tests)
- Modify: any other top-level `internal/component/*_test.go` that exercises the broken bodies
- Create: `docs/plans/2026-04-07-canonical-abi-unification-session0-followup.md`

- [ ] **Step 19.1: Identify failing tests in `internal/component/`**

```bash
go test ./internal/component/ 2>&1 | grep -E '(FAIL|PASS|panic)' | head -30
```

Expected output: a list of failing tests and panics from the compile-fix stubs introduced in Task 17.

- [ ] **Step 19.2: Add `t.Skip` to each failing test**

For each failing test, add at the top of the test function:

```go
t.Skip("session 1 work: see docs/plans/2026-04-07-canonical-abi-unification-session0-followup.md")
```

The skip must be the very first statement in the test body so the test does not even reach the broken code.

- [ ] **Step 19.3: Re-run `internal/component/` tests**

```bash
go test ./internal/component/ 2>&1 | tail -20
```

Expected: PASS or SKIP throughout. Note any remaining failures and address them — either by skipping or by fixing the test if it does not actually exercise the broken path.

- [ ] **Step 19.4: Write the followup note**

Create `docs/plans/2026-04-07-canonical-abi-unification-session0-followup.md` with the content templated by the design doc's Followup Note Format section (design doc lines 1583-1660). The note has three sections:

1. **Session 1 — Wire abi/ into production**
   - List each broken-in-place file:line in `instance.go` (with the actual current line numbers from a fresh grep) with the function name and the planned Session 1 action ("delete this body and call into abi.LiftFlat/LowerFlat").
   - List each broken stub in `component_linker.go` with the same format.
   - List the skipped tests by file and test name.
   - Session 1 acceptance criteria as in the design doc.

2. **Session 2 — Resource Concrete promotion + cross-component type checking**
   - Linker plumbing for Concrete promotion at `component_linker.go` instantiation path.
   - typeChecker walk in a new `internal/component/types/typecheck.go`.
   - Cross-instance resource type resolution beyond the single-instance `LookupResourceType` path.
   - Session 2 acceptance criteria as in the design doc.

3. **Later — Async lift/lower (no session scheduled)**
   - List the trap arms in `lift.go` / `lower.go` for `TypeKindStream`/`TypeKindFuture`/`TypeKindErrorContext`.
   - List the per-instance table identity layering in `composite.go` for `TypeStream`/`TypeFuture`/`TypeErrorContextTable`.
   - List the missing `val.go` constructors for stream/future/error-context values.

- [ ] **Step 19.5: Verify the note exists and is well-formed**

```bash
```

Use Read tool on `docs/plans/2026-04-07-canonical-abi-unification-session0-followup.md` to verify all three sections are present, every cited file path exists, and every cited line number was captured by a fresh grep.

- [ ] **Step 19.6: Commit**

```bash
git add internal/component/instance_test.go internal/component/*_test.go docs/plans/2026-04-07-canonical-abi-unification-session0-followup.md
git commit -m "$(cat <<'EOF'
component, docs: skip Session 1 tests, write followup note

Tests in internal/component/ that exercise end-to-end lift/lower
through the broken instance.go path are marked t.Skip with a
reference to the followup note. The note documents:

- Session 1: which instance.go and component_linker.go bodies must
  be deleted and replaced with direct abi.LiftFlat/LowerFlat calls
- Session 2: resource Concrete promotion at instantiation, the
  typeChecker cross-component matching walk, and cross-instance
  resource type resolution
- Later: async lift/lower stub-and-trap sites that need real
  implementations when stream<T>/future<T>/error-context land

Design: docs/superpowers/specs/2026-04-07-canonical-abi-type-unification-design.md

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 20: Final verification — repo-wide build and test

**Design step:** 18 (work order)
**Goal:** Verify that the end-of-Session-0 build state matches the table in the design doc's "Build State at End of Session 0" section: `types/`, `binary/`, `abi/`, `conformance/` all green; top-level `internal/component/` compile-green with documented test skips; repo-wide `go build ./...` green.
**Design citations:** Build State at End of Session 0 (design doc lines 1569-1579); V3 (design doc lines 1903-1914)

- [ ] **Step 20.1: Repo-wide build**

```bash
go build ./... 2>&1 | tail -10
```

Expected: empty output (clean build). If errors remain, they are not part of Session 0 scope — investigate before proceeding.

- [ ] **Step 20.2: Repo-wide test**

```bash
go test ./... 2>&1 | tail -50
```

Expected:
- `internal/component/types/...` — PASS
- `internal/component/runtime/...` — PASS
- `internal/component/binary/...` — PASS
- `internal/component/abi/...` — PASS
- `internal/component/conformance/...` — PASS
- `internal/component/` (top-level) — PASS or SKIP throughout (no FAIL)
- All other packages — unaffected by Session 0; should match the pre-session state

- [ ] **Step 20.3: V3 verification — grep for deleted-symbol references**

Run each of these greps and verify ZERO matches under `internal/component/` (excluding the followup note in `docs/`):

```bash
```

- Grep for `\bcomponent\.ValTypeRef\b` — expect zero matches
- Grep for `\bcomponent\.RecordTypeDef\b|\bcomponent\.VariantTypeDef\b|\bcomponent\.ListTypeDef\b|\bcomponent\.TupleTypeDef\b|\bcomponent\.FlagsTypeDef\b|\bcomponent\.EnumTypeDef\b|\bcomponent\.OptionTypeDef\b|\bcomponent\.ResultTypeDef\b|\bcomponent\.StreamTypeDef\b|\bcomponent\.FutureTypeDef\b|\bcomponent\.FixedSizeListTypeDef\b` — expect zero matches
- Grep for `\bcomponent\.FuncType\b|\bcomponent\.NamedValType\b` — expect zero matches
- Grep for `\bbinary\.TypeDef\.(Record|Variant|List|Tuple|Flags|Enum|Option|Result|Resource)\b` — expect zero matches
- Grep for `\bcomponent\.TypeIdxToStoredIdx\b` — expect zero matches
- Grep for `\bresolveToValType\b|\btypeDefToValType\b|\bvalTypeRefToValType\b|\bresolveValTypeRef\b` — expect zero matches
- Grep for `\bTypeResolver\b` — expect zero matches (file deleted)
- Grep for `\bResourceTypeID\b|\bResourceTypeInfo\b|\bNewResourceTypeID\b|\bInvalidResourceTypeID\b|\bNewResourceTypeInfo\b` — expect zero matches
- Grep for `\bInstanceState\b|\bNewInstanceState\b` — expect zero matches except in deleted-file references in the followup note

- [ ] **Step 20.4: V6 verification — `ValTypeOpcode` constants**

Verify the 14 opcode constants the design references all exist in `internal/component/binary/valtype.go`:

```bash
```

Use Grep with pattern `ValTypeOpcode(Record|Variant|List|Tuple|Flags|Enum|Option|Result|Borrow|Own|Future|Stream|FixedSizeList)|PrimValTypeErrorContext` over `internal/component/binary/valtype.go` with output_mode "content" -n true. Expected: 14 distinct constants present.

- [ ] **Step 20.5: V7 verification — `runtime.ComponentInstance` shape matches the spec**

Verify by reading `internal/component/runtime/component_instance.go` that the struct has exactly the eight fields from V7 (`ID`, `Parent`, `Table`, `MayLeave`, `enterCount`, `ResourceTypes`, `Destructors`, `Reentrance`) and matches the Decision 6 description.

- [ ] **Step 20.6: Run `go vet ./...`**

```bash
go vet ./... 2>&1 | tail -20
```

Expected: no warnings. Fix any introduced lint issues in place.

- [ ] **Step 20.7: Final commit (if anything was modified during verification)**

If any step in this task required a small fix, commit with:

```bash
git add -A
git commit -m "$(cat <<'EOF'
session 0: final verification cleanup

Address final verification fixes from V3/V6/V7 grep audits and go vet.

Design: docs/superpowers/specs/2026-04-07-canonical-abi-type-unification-design.md

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

# Rollout / Checkpoint Strategy

The 20 tasks group into six checkpoint zones with independent reviewability:

| Zone | Tasks | Reviewer focus | Gate |
|---|---|---|---|
| **Z1 — types/ skeleton** | 1, 2, 3, 4, 5, 6, 8 | Are the types/ definitions, ABI computers, builder intern keys, and tests faithful to the design doc's Core Type Representation, ABI metadata, and Builder sections? Do the divergence (1)/(2)/(3) tests cite their spec lines? | Checkpoint A: `go build ./internal/component/types/...` and `go test ./internal/component/types/...` both green |
| **Z2 — runtime/ skeleton** | 7, 9, 10, 11 | Does `runtime.ComponentInstance` match the spec's `definitions.py:256-273` shape? Does `Table` use `*ResourceType` pointer identity? Are the deletions (`InstanceState`, `ResourceTypeID`, `ResourceTypeInfo`) complete? Is the `ValidateType` bug fix in place? | Checkpoint B: `go build ./internal/component/runtime/...` and `go test ./internal/component/runtime/...` both green |
| **Z3 — component / binary** | 12, 13 | Does `Component.Types` reference `*types.ComponentTypes`? Are `FuncType`, `NamedValType`, `ValTypeRef`, and the per-kind composite TypeDef structs deleted? Does the binary decoder produce `types.ValType` directly with scope-tracked resource declarations? | Checkpoint C: `go build ./internal/component/binary/...` and `go test ./internal/component/binary/...` both green |
| **Z4 — abi/ rewrite** | 14, 15, 16 | Do `LiftContext`/`LowerContext` carry `Types` and `Instance`? Do the dispatch switches use `typ.Kind`? Are the `TypeKindOwn`/`TypeKindBorrow` arms present and trap with the precise Session 2 wiring error? Are `canon_lower`, `type_resolver`, `resource_lower` deleted? | Checkpoint D: `go build ./internal/component/abi/...` and `go test ./internal/component/abi/...` both green |
| **Z5 — repo compile** | 17 | Are `instance.go` and `component_linker.go` compile-fixed without silent fallbacks? Do panic stubs cover broken-by-design bodies? Has every `*FuncType` reference been mechanically renamed to `*types.TypeFunc`? | Checkpoint E: `go build ./...` green |
| **Z6 — conformance + finish** | 18, 19, 20 | Are the 12 conformance files migrated? Are skipped tests documented in the followup note with file paths and line numbers? Does the followup note cover all three deferred sections (Session 1, Session 2, Later)? Do the V3 grep audits return zero matches? | Checkpoints F + Final: `go test ./...` matches the design doc's "Build State at End of Session 0" table |

**Batching guidance for the executing agent:**

- Z1 tasks 1-6 can be batched into a single dispatch (they all touch `types/` and the build is intentionally broken until Task 8 closes the package). Task 8 must be its own dispatch because it is the first end-to-end check.
- Z2 tasks 7, 9, 10, 11 should be sequenced — Task 7 (resource_type.go) and Task 9 (component_instance.go) introduce forward references that Task 10 (table.go) and Task 11 (deletions) close. Run them in order, not parallel.
- Z3 tasks 12 and 13 must be sequenced (Task 12's `component.go` rewrite is consumed by Task 13's binary rewrite).
- Z4 tasks 14, 15, 16 must be sequenced (Task 14's context rewrite is consumed by Task 15's dispatch rewrite, which makes Task 16's deletions safe).
- Z5 Task 17 is large and benefits from a fresh dispatch and careful review.
- Z6 tasks 18, 19, 20 should each be a separate dispatch with review between them.

**When to escalate to the user:**

- If a grep at any V3 audit step returns matches, stop and report.
- If any task introduces a silent fallback (a `// TODO` comment, a return-default-on-error, a `// fallback`, etc.) instead of a precise trap, stop and report.
- If a deleted file is found to be referenced by code outside `internal/component/`, stop and report — the public API of `internal/component/` may have leaked further than the design doc anticipated.
- If any composite ABI value computed by the new builder disagrees with the equivalent value previously computed by the deleted interface methods on a non-divergent case (i.e., not one of the three documented divergences), stop and report.

# Completion Criteria

Session 0 is complete when all of the following hold:

1. **Build state matches the design doc table:**
   - `go build ./...` green
   - `go test ./internal/component/types/...` green
   - `go test ./internal/component/runtime/...` green
   - `go test ./internal/component/binary/...` green
   - `go test ./internal/component/abi/...` green
   - `go test ./internal/component/conformance/...` green
   - `go test ./internal/component/` PASS or SKIP throughout (no FAIL)
   - Repo-wide `go test ./...` end-state has documented skips and no compile errors

2. **Deletions complete:** the eight wholesale-deleted files (`type_resolver.go`, `type_resolver_test.go`, `canon_lower.go`, `canon_lower_test.go`, `abi/resource_lower.go`, `abi/resource_lower_test.go`, `runtime/resource_type_id.go`, `runtime/resource_type_id_test.go`, `runtime/instance_state.go`, `runtime/instance_state_test.go`) are gone. The two renames (`runtime/resource_table.go` → `runtime/table.go`, `runtime/resource_table_test.go` → `runtime/table_test.go`) are committed via `git mv`.

3. **V3 grep audits return zero matches** for every deleted symbol (see Task 20.3).

4. **V1 ABI assertions pass:** every scalar in `scalarABI`, every composite in the `abi_info_test.go` and `builder_test.go` test bodies, has its expected value cited from the spec or wasmtime authority.

5. **V2 dedup/distinctness passes:** every `Intern<Kind>` method has a paired pass-fail test for structurally identical and structurally distinct inputs.

6. **V7 ComponentInstance shape matches the spec** at `definitions.py:256-273`. The struct has the eight fields, the `Table` is the unified `*Table`, the `MayLeave` is per-instance, the `ResourceTypes` pool exists (empty in Session 0).

7. **Followup note written** at `docs/plans/2026-04-07-canonical-abi-unification-session0-followup.md` covering Session 1, Session 2, and the async stub-and-trap sites with current file paths and line numbers.

8. **No silent fallbacks introduced.** Search for `TODO`, `FIXME`, `fallback`, `// removed`, and any pattern matching "return default value on error" in files modified during Session 0; expect zero matches that originate in this session.

9. **No parallel paths introduced.** There is exactly one canonical type representation (`types.ValType` + `*types.ComponentTypes`), exactly one decoder path (`binary/`), exactly one lift/lower path (`abi/`).

10. **The design doc decisions are honored.** A spot-check by the reviewer of any of Decisions 1-9 against the corresponding code change must show fidelity. Where the design doc says "delete X," X is gone. Where it says "the dispatch arm traps with this error," the trap message is verbatim or semantically equivalent.

When all ten criteria hold, Session 0 is complete and Session 1 (described in the followup note) is unblocked.
