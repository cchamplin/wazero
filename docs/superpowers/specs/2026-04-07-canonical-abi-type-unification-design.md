# Canonical-ABI Type Unification — Design

**Date:** 2026-04-07
**Status:** Design approved, ready for implementation planning
**Scope:** Session 0 only (type representation + abi consumer + decoder). Session 1+ are documented as followups.

## Summary

wazero's component-model side currently has six or more parallel representations of component-model value types, four converters that bridge them, and two complete lift/lower implementations — one correct but test-only (`internal/component/abi/`), one in-use but broken (`internal/component/instance.go` + `internal/component/canon_lower.go`). This design collapses everything to a single canonical type representation that matches the canonical-ABI reference and wasmtime, delivered in a sequence of sessions that reach a clean end state without introducing any new parallel paths during the transition.

This document covers Session 0: replace the type representation in place, rewrite the `abi/` package to consume the new shape, rewrite the binary decoder to produce it directly, and delete every conflicting parallel. Session 1 (wire `abi/` into the production call sites, delete `instance.go`'s broken lift/lower bodies) and Session 2 (resource `Concrete` promotion + cross-component type checking) are scoped separately and referenced in the followup note at the end of Session 0.

## Goals

- Exactly one canonical component-model type representation in the repository.
- The binary decoder produces the canonical representation directly — zero converter functions.
- The representation is correct against the canonical-ABI spec and structurally matches wasmtime.
- `canon_lower.go` and its private type universes are deleted entirely.
- `ComponentTypes` is frozen post-decode via a builder type that panics on post-`Finish` mutation.
- Structural resource layer lands in Session 0 (`TypeResourceTable` in `types/`). The nominal layer is upgraded from the existing composite `runtime.ResourceTypeInfo` to a pointer-identity `*runtime.ResourceType` struct, matching the spec's Python `is` check (`definitions.py:1345`) and fixing an existing bug where `runtime.ResourceTable.ValidateType` only compared `ResourceTypeID` (ignoring instance identity).
- `abi/` dispatch switches gain `TypeKindOwn` and `TypeKindBorrow` arms, closing the current gap. Standalone resource-lift/lower helpers (`resource_lower.go`, `LowerOwn`, `LowerBorrow`) are deleted.
- Runtime state is restructured to match the spec's `ComponentInstance` model (`definitions.py:256-273`): a single-layer `runtime.ComponentInstance` struct with optional `parent` pointer for sub-instance nesting, a unified `Table` field holding heterogeneous handle kinds, and `may_leave` directly on the struct. Fixes a second existing bug where wazero's split `ResourceTable` would produce index-collision between resource and future stream/future/subtask handles once async lands.
- `component.FuncType` and `component.NamedValType` consolidate into `types.TypeFunc` — value-type metadata moves to the types package.
- Async value types (`stream`, `future`, `error-context`) are recognized by the decoder and trap at lift/lower with a precise error message.

## Non-Goals (explicitly deferred)

- **Wiring `abi/` into the production call sites** in `internal/component/instance.go` — Session 1.
- **Deleting the broken lift/lower bodies** in `instance.go` — Session 1.
- **Resource `Abstract` → `Concrete` promotion** at instantiation time (populating `runtime.ComponentInstance.ResourceTypes` and minting per-instance `*ResourceType` pointer identities) — Session 2.
- **`typeChecker` walk** for cross-component import type-matching — Session 2.
- **Async lift/lower implementation** (`stream<T>`, `future<T>`, `error-context` as real values) — deferred, no session scheduled.
- **WIT-binding codegen** for typed/monomorphized call sites — not on any session.
- **Backwards compatibility** with existing public API shape in `api/component`. wazero's component-model side is pre-production and pre-use.
- **Renaming `component.ParsedComponentInstance`** — already renamed before this session from the earlier conflicting `component.ComponentInstance`; no further rename needed.

## Spec Authorities

All spec citations in this design reference files already vendored in the repo. These sources win over any contrary wazero comment, doc, or test.

- `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py` — the canonical-ABI reference implementation. Type class definitions at lines 103-180. Resource identity checks at 1336, 1345, 2147. ABI metadata dispatch throughout.
- `debug-vendored/component-model/design/mvp/CanonicalABI.md` — the spec prose. Resource identity section at 531-549. Lift/lower semantics throughout.
- `debug-vendored/component-model/design/mvp/Explainer.md:600` — confirms value types cannot recurse: *"none of these types are recursive"*.
- `debug-vendored/wasmtime/crates/environ/src/component/types.rs` — `InterfaceType` enum at 576-604, `CanonicalAbiInfo` at 608+, `TypeResourceTable` variants at 1125-1147.
- `debug-vendored/wasmtime/crates/environ/src/component/types_builder.rs` — `ComponentTypesBuilder` with intern maps at 38-124.
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/types.rs` — `Handle<T>` / `TypeChecker` pattern at 39-303.
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/values.rs` — `Val` discriminated union at 67-93, dynamic lift/lower dispatch at 97-346.

## Background: Current State

### Parallel type representations that exist today

1. **`internal/component/binary.TypeDef`** — produced by the binary decoder, carries composite content via pointer fields (`*RecordTypeDef`, `*VariantTypeDef`, …).
2. **`internal/component.TypeDef`** (at `component.go:213`) — produced by linking, a near-copy of `binary.TypeDef` with additional fields (`SourceLocalTypes`, `Handle`).
3. **`internal/component.ValTypeRef`** (at `component.go:357-377`) — a four-variant union (primitive, type-index, own, borrow) referenced from function params and composite type fields.
4. **`internal/component/types.ValType`** — a Go interface with scalar struct types (`Bool`, `S8`, …) and composite struct types (`Record`, `Variant`, …) that satisfy it. Consumed by `internal/component/abi/`.
5. **`internal/component/canon_lower.go` private types** — `EnumType`, `FlagsType`, `VariantType`, `VariantCaseForLower`, `PayloadType`, `PrimitiveType`. An entirely separate type universe used only by the broken production lift/lower in `canon_lower.go`.
6. **Ad-hoc `switch typ.(type) { ... }` dispatch sites** in `instance.go` that construct `types.ValType` literals inline — effectively a sixth shape of sorts, treating the interface as a closed set.

### Converters that bridge them

Four explicit converters, plus a fifth (`TypeResolver`) that is its own 150-line fallback system:

- `internal/component/component_linker.go:701` — `resolveValTypeRef(ref ValTypeRef, localTypes map[uint32]*TypeDef) ValTypeRef`
- `internal/component/component_linker.go:723` — `resolveToValType(nvt NamedValType, resolver *TypeResolver) types.ValType`
- `internal/component/component_linker.go:749` — `typeDefToValType(td *TypeDef, localTypes map[uint32]*TypeDef) types.ValType`
- `internal/component/component_linker.go:834` — `valTypeRefToValType(ref ValTypeRef, localTypes map[uint32]*TypeDef) types.ValType`
- `internal/component/type_resolver.go` — the `TypeResolver` type and its `resolveDefinedType` + 8 child resolvers at lines 172-303.

These converters have known divergences. Most notably, `valTypeRefToValType` at `component_linker.go:883` silently returns `types.U32{}` as a "fallback" when it cannot resolve a type index — a silent corruption, not an error.

### Lift/lower implementations

- **`internal/component/abi/`** — structurally correct, consumes `types.ValType` via interface type-switch, exercised only by `internal/component/conformance/` tests (12 files). Not wired into any production code path.
- **`internal/component/instance.go` + `internal/component/canon_lower.go`** — the production lift/lower path. Consumes `types.ValType` plus the private canon_lower.go type shapes plus ad-hoc logic. Used by real component invocations but known to be broken on non-trivial inputs.

## Design Decisions

### Decision 1: Approach — wasmtime-style indexed types table

Adopt the shape wasmtime uses: a discriminated union (`ValType { Kind, Index }`) where composite kinds carry a typed index into a per-component `ComponentTypes` table that holds the actual type content. Three language-neutral forces drive this choice:

1. **Interning / dedup.** Real WIT worlds produce many structurally-identical `tuple<u32, u32>`, `option<string>`, `result<_, error>` references. Interning produces one table entry per distinct structure. Inline trees cannot dedup without identity-hashing subtrees.

2. **Cross-instance type aliasing.** `outer` aliases pull a type from one scope into another. With indices, aliasing is "copy the index into the new binding" — both sides reference the same table entry automatically. With inline trees this is deep-clone-or-Arc.

3. **Resource identity is nominally two-level.** `definitions.py:1336, 1345, 2147` use Python `is` (object identity), not `==`. CanonicalABI.md:531-549 confirms: "resource type equality is not defined structurally." Two instantiations of the same component produce distinct resource identities; handles minted by one must trap if presented to the other. The spec forces a two-level representation.

### Decision 2: No recursion in value types

`Explainer.md:600`: *"none of these types are recursive."* The canonical-abi reference enforces `MAX_TYPE_DEPTH = 100`. This means the value-type DAG is finite, buildable bottom-up in a single decoder pass, and requires no cycle breaking, weak references, or two-phase construction. Resources *are* nominal (identity-referenced via `ResourceIdx`) so they don't count as structural recursion.

### Decision 3: Rust-isms that do not cross the language boundary

| wasmtime pattern | Why it exists | Go equivalent |
|---|---|---|
| `PrimaryMap<TypeFooIndex, TypeFoo>` | Strongly-typed bounds-checked append-only slice with newtype indices | `[]TypeFoo` indexed by a named `uint32` type — matches `internal/wasm/module.go:37`'s existing `TypeSection []FunctionType` / `Index` idiom |
| `Arc<ComponentTypes>` + `Arc::ptr_eq` | Cheap clone with pointer-identity fast path | `*ComponentTypes` + native Go `==` on pointers |
| `Lift`/`Lower` traits + monomorphized typed API | Zero-cost generic specialization | Not replicated. Dynamic API only; future WIT codegen is the escape hatch |
| `#[repr(transparent)] u32` newtypes at indexing sites | Compile-time kind safety at the `[]` operator | Not fully replicable in Go. Constructor methods + tests catch misuse |

### Decision 4: Two separate enums for type kinds and value kinds

wasmtime has two completely distinct enum types:

- `InterfaceType` (type-side, `environ/src/component/types.rs:576-604`) — variants carry `TypeRecordIndex`, `TypeVariantIndex`, etc.
- `Val` (value-side, `runtime/component/values.rs:67-93`) — variants carry inline value data (`Vec<(String, Val)>`, etc.).

wazero follows the same split:

- **`TypeKind`** (in `types/types.go`) — the type-side discriminator. Full set: `TypeKindBool`, …, `TypeKindFuture`, `TypeKindErrorContext`.
- **`ValKind`** (in `types/val.go`, already exists) — the value-side discriminator. Existing constants `ValKindBool`, …, `ValKindBorrow` stay unchanged. Three new constants (`ValKindStream`, `ValKindFuture`, `ValKindErrorContext`) added for symmetry with wasmtime, with no constructors yet.

Full prefixes prevent accidental confusion. `TypeKindBool` and `ValKindBool` are distinct types with distinct semantics — one says "I am a bool-valued type," the other says "I am a bool value."

### Decision 5: Resource identity — structural + pointer-identity nominal layer in Session 0

Both layers of the spec's resource identity model land in Session 0:

**Structural layer** (in `types/`):

- `types.TypeResourceTable` struct inside `ComponentTypes.ResourceTables` — bare uint32-typed fields (`Concrete`, `Resource`, `Instance`, `AbstractIdx`). No pointers to anything outside `types/`. No import cycle.
- `types.ResourceIdx`, `types.RuntimeComponentInstanceIdx` named uint32 types in `types/`.
- `Own`/`Borrow` ValType encoding (`ValType{Kind: TypeKindOwn, Index: uint32(resourceTableIdx)}`).
- Integration of Own/Borrow into the `abi/` lift/lower dispatch switches (they are currently missing — see Decision 7).

**Nominal layer** (in `runtime/`):

- New `runtime.ResourceType` struct with **pointer identity**. Compared via pointer equality (`*ResourceType == *ResourceType`), matching the spec's Python `is` check at `definitions.py:1345` (`trap_if(h.rt is not t.rt)`) and at `:2147`.
- Field shape: `runtime.ResourceType{Impl *ComponentInstance, Dtor *uint32, DtorAsync bool, DtorCallback *uint32}`. `Impl` is the defining component instance for the resource type (spec field name: `impl`, at `definitions.py:351-361`).
- The existing `runtime.ResourceTypeInfo` composite (at `runtime/resource_type_id.go:35-66`) is deleted. It is replaced by direct `*ResourceType` references.
- The existing `runtime.ResourceTypeID` (a uint32 alias at `runtime/resource_type_id.go:8`) is also deleted. Handle entries in the unified `Table` (see Decision 6) hold `*ResourceType` directly rather than an opaque typeID.

**Bug fix from the re-verification:**

`runtime.ResourceTable.ValidateType` at `resource_table.go:391-400` currently compares only `ResourceTypeID` (`actual != expected`), ignoring instance identity. This accepts handles from one component instance's type as valid for a different component instance's type when both happen to have the same type-section index. Session 0 fixes this by changing the validation to pointer-identity comparison on `*ResourceType`, so a handle minted by instance A is rejected when presented to a function expecting instance B's resource type even if both have type index 0.

**What does NOT land in Session 0** (deferred to Session 2):

- Linker plumbing that promotes `Abstract` `TypeResourceTable` entries to `Concrete` at instantiation time. Session 0 leaves `ComponentTypes.ResourceTables` containing only `Abstract` entries.
- Population of `runtime.ComponentInstance.ResourceTypes` at runtime (the `[]*ResourceType` pool that the `Concrete` promotion would fill in). The field exists (see Decision 6) but is empty at end of Session 0.
- Actual resource handle lift/lower succeeding end-to-end on a real component. The code path exists, is spec-correct, and traps precisely when the per-instance state is missing. Tests that exercise full cross-instance resource handles are Session 2 work.

At end of Session 0, the resource lift/lower dispatch arms exist, compile, and trap with a precise "no resource type for instance N declaration M (resource concrete promotion not yet wired — session 2)" error when `ctx.Instance.LookupResourceType` returns nil. This is consistent with "pre-production" — the current test suite does not exercise cross-instance resource-identity end-to-end. When Session 2 populates the state, the dispatch arms begin working without further changes to the type representation.

### Decision 6: Runtime `ComponentInstance` follows the spec's single-layer model

The canonical-ABI reference at `definitions.py:256-273` defines `ComponentInstance` as a single self-contained struct:

```python
class ComponentInstance:
  store: Store
  parent: Optional[ComponentInstance]     # sub-instance hierarchy via parent pointer
  table: Table                            # ONE unified handle table per instance
  may_leave: bool                         # ON the instance, not at a VM level
  backpressure: int
  exclusive: bool
  num_waiting_to_enter: int
```

Each instantiated component — top-level or nested — is a separate `ComponentInstance` with its own table and its own `may_leave` flag. The sub-instance hierarchy is tracked via the `parent` pointer, not a map of sub-instance states.

This is simpler than wasmtime's aggregating-map approach (`wasmtime/runtime/vm/component.rs:93-159`, which has an `instance_states: PrimaryMap<RuntimeComponentInstanceIndex, InstanceState>` + resource_types pool). Wasmtime's model is a Rust-specific optimization to share one top-level allocation; the spec's model is structurally simpler and a better fit for Go.

**wazero adopts the spec's single-layer model.** The existing `runtime.InstanceState` struct (at `runtime/instance_state.go`) is **merged into** the new `runtime.ComponentInstance`:

```go
// runtime.ComponentInstance — one per instantiated component (top-level
// or nested). Matches the spec's ComponentInstance at definitions.py:256-273.
//
// For nested instantiation, Parent points to the parent instance; for
// top-level instances, Parent is nil. Each instance owns its own Table
// and its own MayLeave flag.
type ComponentInstance struct {
    ID     uint32
    Parent *ComponentInstance // nil for top-level instances

    // Table is the unified handle table for this instance. Holds all
    // handle kinds (resources, and eventually subtasks/streams/futures/
    // error-contexts) — see Table type below. One table per instance;
    // handle indices are unique within this instance across all kinds.
    Table *Table

    // MayLeave tracks whether this instance can perform operations that
    // "leave" it. Set to false during canon.task.enter, restored after.
    // Spec: definitions.py:260, 270, 1955, 1973.
    MayLeave bool

    // enterCount tracks the nesting depth of the instance (the existing
    // enterCount from wazero's InstanceState, preserved semantically).
    // Accessed via Enter()/Leave()/EnterCount() methods.
    enterCount int

    // ResourceTypes is the nominal resource type identity pool for
    // resource declarations DEFINED BY this instance. Indexed by
    // types.ResourceIdx. Populated at instantiation time (Session 2 —
    // empty in Session 0).
    //
    // Each entry is a *ResourceType with pointer identity. Two handles
    // are of "the same resource type" iff their *ResourceType pointers
    // are equal — matching the spec's `is` check at definitions.py:1345.
    ResourceTypes []*ResourceType

    // Destructors is this instance's destructor registry.
    Destructors *DestructorRegistry

    // Reentrance tracks call-site reentrance for this instance.
    Reentrance *ReentranceTracker
}
```

**wazero's existing `runtime.InstanceState` is deleted** as a separate struct. Its fields (`id`, `enterCount`, `mayLeave`) become fields on `runtime.ComponentInstance` (renamed `id → ID`, `mayLeave → MayLeave`, `enterCount` kept lowercase as it was). Its methods (`MayLeave()`, `SetMayLeave`, `Enter()`, `Leave()`, `EnterCount()`) move to `ComponentInstance` unchanged.

**Name collision check:** There is no collision with `component.ParsedComponentInstance` (the parse-time instantiation-expression type at `component.go:705`, already renamed). The simple name `ComponentInstance` is free in the `runtime` package.

**`abi/LiftContext` and `abi/LowerContext` carry `Instance *runtime.ComponentInstance`** (the current calling instance — per the spec, each call is performed by a specific instance). Resource lookups inside dispatch navigate: `ctx.Instance.Table` → fetch the entry → type-check against the expected `*ResourceType` from `ctx.Instance.ResourceTypes[rt.Resource]` or a cross-instance resolution when the resource is owned by a different instance.

Cross-instance resource handling walks instance references via the `Parent` pointer or via a top-level store-wide lookup — the exact mechanism for finding a foreign instance's `*ResourceType` when lifting a handle from a different instance is wired in Session 2 alongside the `Concrete` promotion work.

### Decision 7: `component.FuncType` and `component.NamedValType` consolidate into `types.TypeFunc`, and Own/Borrow integrate into dispatch

`component.FuncType` at `component.go:344-347` is a thin wrapper over `[]NamedValType` (params) and `[]NamedValType` (results). `component.NamedValType` at `component.go:350-355` is a `(name, ValTypeRef)` pair with two now-defunct helper fields (`ResolvedType`, `LocalTypes`) that exist only to support the old cross-instance-alias resolution machinery being deleted in this session.

Neither carries component-runtime-level state. Both are value-type metadata and belong in `types/`. Session 0 deletes both from `component/` and consolidates them into `types.TypeFunc` (already defined in the core types section). Every caller that takes `*component.FuncType` updates to `*types.TypeFunc` (or the `types.FuncTypeIdx` index form where appropriate).

Missing Own/Borrow dispatch arms in current abi/: `abi/lift.go:52-351` and `abi/lower.go:13-319` do not have `case types.Own:`/`case types.Borrow:` arms at all. They fall through to the `default: "unsupported flat lift for type: %T"` error. The existing `LowerOwn`/`LowerBorrow`/`LowerOwnWithType`/`LowerBorrowWithType` functions in `lower.go:683-712` and `resource_lower.go:21-59` are standalone helpers that are not integrated into dispatch.

Session 0 fixes this by adding `case types.TypeKindOwn:` and `case types.TypeKindBorrow:` arms to `LiftFlat`, `LiftHeap`, `LowerFlat`, and `LowerHeap`. The arm bodies inline the resource-lookup logic (including the same-instance optimization from `CanonicalABI.md:2677-2683` currently in `LowerBorrowWithType`). The standalone helpers in `resource_lower.go` and `lower.go:683-712` are deleted — one dispatch path, no parallel helpers.

### Decision 8: Async types are stub-and-trap

`stream<T>`, `future<T>`, and `error-context` are recognized by the decoder, get proper structural entries in `ComponentTypes.Streams` / `.Futures` / `.ErrorContextTables`, and have correct ABI metadata (size 4, align 4, flatten 1 per `definitions.py:1074, 1080, 1132, 1138, 1713, 1719`). But the `abi/` lift/lower dispatch traps on them with a precise error:

```go
case TypeKindStream, TypeKindFuture, TypeKindErrorContext:
    return Val{}, fmt.Errorf("component-model async types not yet supported: kind=%s", typ.Kind)
```

The per-instance table identity layering (analogous to `TypeResourceTable.Concrete`/`Abstract`) is deferred to the async lift/lower work.

### Decision 9: Delete `canon_lower.go` entirely in Session 0

`canon_lower.go` defines its own private type universe that is inconsistent with `types.ValType`, houses a broken lift/lower implementation, and will be superseded entirely by `abi/` in Session 1. Leaving half of it alive during the Session 0 → Session 1 transition produces compile-fixups for code that will be deleted. Deleting the whole file in Session 0 removes that waste.

Consequence: end of Session 0 leaves `internal/component/instance.go`'s lift/lower bodies compile-broken at every call site that previously routed through `canon_lower.go`. Those bodies are slated for deletion in Session 1, so the compile-fixup minimizes to "make `instance.go` compile, tolerate logically-wrong bodies, document the deficiencies in the followup note, skip the end-to-end tests."

## Core Type Representation

### Package and file layout

All type definitions live in `internal/component/types/` (existing package, reshaped in place). No new packages. The file layout is:

- `types/types.go` — `TypeKind` enum, `ValType` struct, `ComponentTypes` struct, primitive constants, public accessors
- `types/composite.go` — `TypeRecord`, `TypeVariant`, `TypeList`, `TypeTuple`, `TypeFlags`, `TypeEnum`, `TypeOption`, `TypeResult`, `TypeStream`, `TypeFuture`, `TypeErrorContextTable`, `TypeFunc`, `RecordField`, `VariantCase`
- `types/resource.go` — `TypeResourceTable`, `ResourceType`, `ResourceIdx`, `RuntimeComponentInstanceIdx`
- `types/abi_info.go` — **new file** — `CanonicalABIInfo`, `DiscriminantInfo`, scalar ABI lookup table
- `types/builder.go` — **new file** — `ComponentTypesBuilder` with `Intern...` methods and `Finish`
- `types/val.go` — existing file, minimal changes (add three `ValKind*` constants, update `String()` method)

### Core types

```go
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
    TypeKindList        // Index -> ComponentTypes.Lists (dynamic length)
    TypeKindFixedList   // Index -> ComponentTypes.FixedLists (fixed length, distinct type)
    TypeKindRecord      // Index -> ComponentTypes.Records
    TypeKindTuple       // Index -> ComponentTypes.Tuples
    TypeKindVariant     // Index -> ComponentTypes.Variants
    TypeKindEnum        // Index -> ComponentTypes.Enums
    TypeKindOption      // Index -> ComponentTypes.Options
    TypeKindResult      // Index -> ComponentTypes.Results
    TypeKindFlags       // Index -> ComponentTypes.Flags
    TypeKindOwn         // Index -> ComponentTypes.ResourceTables
    TypeKindBorrow      // Index -> ComponentTypes.ResourceTables
    TypeKindStream      // Index -> ComponentTypes.Streams (lift/lower traps)
    TypeKindFuture      // Index -> ComponentTypes.Futures (lift/lower traps)
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

// Zero ValType is distinguishable from a legitimate TypeKindBool value
// only by context. The builder never returns a zero ValType; scalar
// constants are exposed as named variables (see below).
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
    String_ = ValType{Kind: TypeKindString} // String_ because `string` is a reserved word concern
)
```

### ComponentTypes

```go
// ComponentTypes is the per-top-level-component immutable type bag.
// Built by ComponentTypesBuilder during binary decode, frozen at Finish,
// and threaded through all subsequent lift/lower / validation / linking.
// One pointer identity per compiled component drives the fast-path
// type-equality short-circuit during cross-component type checking
// (added in Session 2).
type ComponentTypes struct {
    Records            []TypeRecord
    Variants           []TypeVariant
    Lists              []TypeList              // dynamic-length lists only
    FixedLists         []TypeFixedLengthList   // fixed-length lists are a distinct
                                                // type per spec (`ListType(t, l)` with
                                                // `l != None`) and wasmtime
                                                // (`TypeFixedLengthListIndex`)
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

### Composite structs

All composite structs carry precomputed ABI metadata. The scalar kinds get their ABI from a constant table keyed by `TypeKind`. Precomputation happens during interning so the lift/lower hot path never recomputes.

```go
// RecordField is one field of a record type. Order is significant
// (spec-defined); names are unique within the record.
type RecordField struct {
    Name string
    Type ValType
}

type TypeRecord struct {
    Fields []RecordField
    ABI    CanonicalABIInfo
}

type VariantCase struct {
    Name       string
    Payload    ValType // zero-valued iff HasPayload == false
    HasPayload bool
}

type TypeVariant struct {
    Cases []VariantCase
    ABI   CanonicalABIInfo
    Disc  DiscriminantInfo
}

// TypeList is a dynamic-length list. Memory layout is (ptr: i32, len: i32).
// Fixed-length lists are a distinct type — see TypeFixedLengthList.
type TypeList struct {
    Element ValType
    ABI     CanonicalABIInfo
}

// TypeFixedLengthList is a list with a compile-time-known length. Memory
// layout is `length` elements stored inline, not via ptr+len indirection.
// Distinct from TypeList because spec and wasmtime treat them as distinct
// types with distinct identities: a function expecting `list<u32>` cannot
// accept a `list<u32, 5>` and vice versa.
//
// Spec: definitions.py:122-125 — `ListType(t, l)` with `l: Optional[int]`
// where l != None produces the fixed-length variant.
// Wasmtime: environ/src/component/types.rs separates `TypeListIndex`
// (lists) from `TypeFixedLengthListIndex` (fixed-length lists) as
// distinct PrimaryMap keys and distinct `InterfaceType` variants.
type TypeFixedLengthList struct {
    Element ValType
    Length  uint32 // > 0 per spec
    ABI     CanonicalABIInfo
}

type TypeTuple struct {
    Types []ValType
    ABI   CanonicalABIInfo
}

type TypeFlags struct {
    Names []string
    ABI   CanonicalABIInfo
}

type TypeEnum struct {
    Names []string
    ABI   CanonicalABIInfo
    Disc  DiscriminantInfo
}

type TypeOption struct {
    Element ValType
    ABI     CanonicalABIInfo
    Disc    DiscriminantInfo
}

type TypeResult struct {
    OK     ValType
    Err    ValType
    HasOK  bool
    HasErr bool
    ABI    CanonicalABIInfo
    Disc   DiscriminantInfo
}

type TypeStream struct {
    Element    ValType
    HasElement bool
}

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
// are each a Tuple (by ValType of TypeKindTuple). One mechanism for
// "ordered list of types," reused.
type TypeFunc struct {
    Async      bool
    ParamNames []string
    Params     ValType // TypeKindTuple
    Results    ValType // TypeKindTuple
}
```

### ABI metadata types

```go
// CanonicalABIInfo carries precomputed size / alignment / flatten data
// for a type in both 32-bit and 64-bit memory modes. Mirrors wasmtime's
// CanonicalAbiInfo (environ/src/component/types.rs:608+).
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
        return scalarABI[v.Kind] // i32 handle shape
    }
    panic(fmt.Sprintf("ABI: unknown TypeKind %d", v.Kind))
}

// scalarABI is a package-level constant table for types whose ABI is
// not dependent on their content. Indexed by TypeKind.
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
    TypeKindString:       {Size32: 8, Align32: 4, Size64: 16, Align64: 8, FlattenCount: 2}, // ptr+len
    TypeKindOwn:          {Size32: 4, Align32: 4, Size64: 4, Align64: 4, FlattenCount: 1},
    TypeKindBorrow:       {Size32: 4, Align32: 4, Size64: 4, Align64: 4, FlattenCount: 1},
    TypeKindStream:       {Size32: 4, Align32: 4, Size64: 4, Align64: 4, FlattenCount: 1},
    TypeKindFuture:       {Size32: 4, Align32: 4, Size64: 4, Align64: 4, FlattenCount: 1},
    TypeKindErrorContext: {Size32: 4, Align32: 4, Size64: 4, Align64: 4, FlattenCount: 1},
}
```

## Resource Identity

Spec authority: `definitions.py:256-273` (ComponentInstance shape), `:303-315` (Table), `:351-361` (ResourceType), `:1336, 1345, 2147` (Python `is` check), `CanonicalABI.md:531-549`.

### Structural layer — `types/resource.go`

```go
package types

// ResourceIdx names a resource *declaration* — a `(type $r (resource ...))`
// site in the binary. Unique within a single component's type section.
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
//   - Concrete: bound to a specific runtime component instance.
//     Resolves at call time via runtime.ComponentInstance.ResourceTypes
//     (possibly walking to a parent or across instances) to the nominal
//     *runtime.ResourceType for validity checking.
//   - Abstract: lives only inside a not-yet-instantiated component or
//     instance type declaration. Cannot be lifted/lowered at runtime;
//     lift/lower traps if reached at call time.
//
// At end of Session 0 ALL entries are Abstract — Concrete promotion
// at instantiation time is Session 2 work.
//
// Spec: CanonicalABI.md:531-549
type TypeResourceTable struct {
    Concrete bool

    // Concrete fields (Concrete == true)
    Resource ResourceIdx                  // which nominal declaration
    Instance RuntimeComponentInstanceIdx  // which instance defines it

    // Abstract fields (Concrete == false)
    AbstractIdx uint32
}
```

### Nominal layer — `runtime/resource_type.go`

New file. Pointer-identity `*ResourceType` struct replacing the existing composite `ResourceTypeInfo`:

```go
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
// component produce TWO distinct *ResourceType objects — a handle
// minted by the first instance is rejected when presented to a function
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

**Deletions from the existing wazero runtime:**

- `runtime.ResourceTypeID` (at `runtime/resource_type_id.go:8`) — deleted. Replaced by direct `*ResourceType` references in handle entries and validation calls.
- `runtime.ResourceTypeInfo` (at `runtime/resource_type_id.go:35-66`) — deleted. Its `typeID + instanceID` composite is not spec-correct because wazero's `ValidateType` only compares `typeID`, ignoring `instanceID`, producing false positives for cross-instance resource identity checks.
- `runtime.NewResourceTypeID`, `runtime.InvalidResourceTypeID`, `runtime.NewResourceTypeInfo` helpers — deleted.
- `runtime/resource_type_id.go` — entire file deleted.
- `runtime/resource_type_id_test.go` — entire file deleted.

**Bug fix:** `runtime.ResourceTable.ValidateType(h Handle, expected ResourceTypeID)` at `resource_table.go:391-400` currently compares only `ResourceTypeID`, ignoring instance identity. This accepts handles from one component instance's type as valid for a different component instance's type when both happen to have the same type-section index. Session 0 replaces this method with a version that takes `*ResourceType` and compares pointers:

```go
// NEW signature (replaces existing ValidateType):
func (t *Table) ValidateType(h Handle, expected *ResourceType) error {
    entry, err := t.Get(h)
    if err != nil {
        return err
    }
    resEntry, ok := entry.(*ResourceHandleEntry)
    if !ok {
        return ErrInvalidHandle  // handle is not a resource handle
    }
    if resEntry.RT != expected {  // POINTER equality — spec's `is` check
        return fmt.Errorf("%w: wrong resource type", ErrResourceTypeMismatch)
    }
    return nil
}
```

### Host-managed resource types

Host code that creates resource handles outside the component-model linker — most importantly the wasi:io / wasi:filesystem / wasi:sockets / etc. host modules under `imports/wasip2/` — does not have a guest-defined component instance to bind its resources to. Each host-managed resource kind needs a `*runtime.ResourceType` for handle tagging and validation, but its `Impl` is `nil` (no defining component instance) and its destructor uses Go's `Destroyable` interface (already in the existing `runtime/resource_table.go:32-34`) rather than a guest core function index.

**Wasmtime parity.** Wasmtime's `ResourceType` (`crates/wasmtime/src/runtime/component/resources/ty.rs:23-142`) is a tagged union with five variants: `Host(TypeId)`, `HostDynamic(u32)`, `Guest{store, instance, id}`, `Uninstantiated`, `Abstract`. The `Host(TypeId)` variant uses Rust's compile-time-stable `TypeId::of::<T>()` so `ResourceType::host::<InputStream>()` is the same value at every call site. The `Guest` variant identifies an instantiation by `(store, *const ComponentInstance as usize, DefinedResourceIndex)`. Two `ResourceType`s are equal iff their tagged union content is equal. Identity propagates through the inner fields.

**Go translation.** Wazero uses pointer identity on a single `*runtime.ResourceType` struct in place of wasmtime's tagged union. The semantic equivalent of `ResourceType::host::<T>()` is a **package-level singleton `*runtime.ResourceType`** with `Impl: nil`. The semantic equivalent of `ResourceType::guest(...)` is one `*runtime.ResourceType` per `(ResourceIdx, ComponentInstance)` allocated at instantiation time (Session 2 work). Pointer comparison gives the same observable behavior as wasmtime's structural-tagged comparison because the inner fields ensure each value is unique.

**Pattern for `imports/wasip2/io/streams.go` (and analogous host modules):**

```go
package io

import "github.com/tetratelabs/wazero/internal/component/runtime"

// Host-managed resource type singletons. One *ResourceType per host
// resource kind that this module exposes to guests. Impl is nil because
// these resources are host-owned, not bound to any guest component
// instance. Destruction is handled via the Destroyable interface on the
// stream's Rep value, not via the guest-side Dtor field.
//
// Equivalent to wasmtime's ResourceType::host::<InputStream>(),
// ResourceType::host::<OutputStream>(), etc. at
// crates/wasmtime/src/runtime/component/resources/ty.rs:44.
var (
    inputStreamResourceType  = &runtime.ResourceType{}
    outputStreamResourceType = &runtime.ResourceType{}
    pollableResourceType     = &runtime.ResourceType{}
    errorResourceType        = &runtime.ResourceType{}
)

// Minting an input-stream handle (replaces the old table.New(rep, true)):
func mintInputStream(table *runtime.Table, stream *InputStream) uint32 {
    handle := table.NewResourceHandle(stream, true /* own */, inputStreamResourceType)
    return uint32(handle)
}

// Retrieving an input-stream handle (replaces the old entry.Rep direct
// access; type assertion required because Table.Get returns the generic
// runtime.TableEntry interface):
func getInputStream(table *runtime.Table, handle uint32) (*InputStream, error) {
    entry, err := table.Get(runtime.Handle(handle))
    if err != nil {
        return nil, err
    }
    resEntry, ok := entry.(*runtime.ResourceHandleEntry)
    if !ok {
        return nil, fmt.Errorf("handle %d is not a resource handle", handle)
    }
    // Optional: verify the host resource type matches what this getter expects.
    // resEntry.RT == inputStreamResourceType prevents an output-stream handle
    // from being silently treated as an input-stream handle.
    if resEntry.RT != inputStreamResourceType {
        return nil, fmt.Errorf("handle %d is not an input-stream", handle)
    }
    stream, ok := resEntry.Rep.(*InputStream)
    if !ok {
        return nil, fmt.Errorf("handle %d rep is not *InputStream (got %T)", handle, resEntry.Rep)
    }
    return stream, nil
}
```

**Why pointer identity is correct here:**

- Two distinct host modules that each declare a singleton `*ResourceType` for their own "stream" concept produce two distinct pointers, and a handle minted by one module is rejected by the other's `ValidateType`. This matches the spec's `is` check at `definitions.py:1345`.
- The same module across different `runtime.ComponentInstance`s reuses the same singleton pointer, so handles round-trip correctly within a process.
- Across processes / restarts there is no state to preserve; pointer identity is process-local, which matches the lifetime of the resource handles themselves (handles do not survive process restart either).

**Destructors for host resources** flow through wazero's existing `runtime.Destroyable` interface. When `Table.Delete(handle)` is called on an owned handle, the table checks whether the entry's `Rep` implements `Destroyable` and, if so, calls `Rep.Destroy()`. This is preserved across the Session 0 rename of `ResourceTable → Table`. Host modules whose resource types need cleanup (e.g., `InputStream` closes its underlying `io.Reader` if it implements `io.Closer`) implement `Destroy()` on their Rep type, exactly as `imports/wasip2/io/streams.go:137` and `:268` already do for `InputStream` and `OutputStream`. The `*runtime.ResourceType` struct's `Dtor *uint32` field stays `nil` for host resources because there is no guest core function index to call — the destructor is Go code reached via the interface.

**Single-table-per-instance still holds.** Host-minted resources go into the same `runtime.Table` as guest-minted resources because the spec at `definitions.py:259` mandates one `Table` per `ComponentInstance` and the wasip2 host modules write into the calling instance's table so the guest can subsequently read the handle back. Wasmtime separately maintains a `host_table: &mut HandleTable` on its `LiftContext` for some cross-instance host bookkeeping, but that is a wasmtime optimization, not a spec requirement; wazero's design follows the spec literally.

### Unified handle table — `runtime/table.go`

The spec at `definitions.py:303-315` defines `class Table` with a single `array: list[any]` holding handles of **any** kind. `cx.inst.table.add(h)` is called for resource handles (`:1643, 1651`), streams (`:1656`), futures (`:1661`), error-contexts (`:1583`), and subtasks (`:2121`) — all into the same table. Handle indices are unique across all kinds within an instance.

wazero's existing `runtime.ResourceTable` (at `runtime/resource_table.go`) holds only resource handles. This is **not spec-correct** — it allows a resource handle and a future stream handle to coexist at the same index in different tables, whereas the spec mandates unified index space. This is a latent bug that will manifest when direct stream/future/subtask handles are introduced (beyond the resource-wrapped approach used by wasi 0.2).

Session 0 replaces `runtime.ResourceTable` with a unified `runtime.Table`:

```go
package runtime

// Table is the unified per-instance handle table. Holds heterogeneous
// handle kinds: resource handles, subtask handles, stream endpoint
// handles, future endpoint handles, error-context handles. Each entry
// is typed via a TableEntry interface; callers check the dynamic type
// via type assertion at retrieval.
//
// Spec: definitions.py:303-315 (class Table), with references at
// :1583, 1643, 1651, 1656, 1661, 2121 showing unified usage.
//
// One Table per runtime.ComponentInstance. Handle indices are unique
// within a single Table across all handle kinds.
type Table struct {
    entries  []tableEntry
    freeHead int32
}

// TableEntry is the interface implemented by everything stored in a
// Table. The type assertion at retrieval distinguishes kinds.
type TableEntry interface {
    tableEntry()
}

// ResourceHandleEntry is the Table entry type for resource handles.
// Replaces the old HandleEntry{RT ResourceTypeID, Rep any, Own bool,
// NumLends uint32, BorrowScope any} struct.
type ResourceHandleEntry struct {
    RT          *ResourceType // pointer identity for spec's `is` check
    Rep         any
    Own         bool
    NumLends    uint32
    BorrowScope any
}

func (*ResourceHandleEntry) tableEntry() {}

// Future Session 2+ entry types (not implemented in Session 0):
// - SubtaskEntry  for async subtasks (definitions.py:2121)
// - StreamEntry   for ReadableStreamEnd (definitions.py:1656)
// - FutureEntry   for ReadableFutureEnd (definitions.py:1661)
// - ErrorContextEntry for error-context values (definitions.py:1583)
//
// All future kinds live in the same Table, sharing index space with
// resource handles. Adding them is a matter of defining new entry
// types and extending dispatch arms — no structural changes to Table.
```

**Modifications to existing files:**

- `runtime/resource_table.go` — heavily rewritten:
  - `type ResourceTable struct { ... }` renamed to `type Table struct { ... }` (file also renamed: `runtime/resource_table.go` → `runtime/table.go`). Existing entries/freeHead machinery preserved; the type that each entry holds becomes the new `TableEntry` interface.
  - `type HandleEntry struct { RT ResourceTypeID; ... }` replaced by `type ResourceHandleEntry struct { RT *ResourceType; ... }` satisfying `TableEntry`.
  - `NewResourceTable` renamed to `NewTable`.
  - `NewWithType(rep any, own bool, rtID ResourceTypeID)` replaced by a `NewResourceHandle(rep any, own bool, rt *ResourceType)` method that constructs and inserts a `ResourceHandleEntry`.
  - `ValidateType` signature changed from `(h Handle, expected ResourceTypeID) error` to `(h Handle, expected *ResourceType) error`, body changed to pointer comparison and type assertion.
  - `GetType(h Handle) (ResourceTypeID, error)` replaced by `GetResourceType(h Handle) (*ResourceType, error)` that fetches the entry, type-asserts to `*ResourceHandleEntry`, returns its `RT` field.
  - `CreateResourceNewFunc`, `CreateResourceDropFunc`, `CreateResourceRepFunc` and their `WithType`/`WithTrap`/`WithContext` variants rewritten to take `*ResourceType` arguments instead of `resourceTypeIdx uint32`.

- `runtime/resource_table_test.go` — 1240-line test file updated to use the new `Table` / `*ResourceType` signatures. Mechanical but voluminous.

### Own / Borrow encoding and dispatch

`Own` and `Borrow` have no struct representation. They are encoded as ValType values:

```go
vt := ValType{Kind: TypeKindOwn, Index: uint32(rtIdx)}
```

Pseudocode for the lift-side `TypeKindOwn` dispatch arm:

```go
case types.TypeKindOwn:
    rt := ctx.Types.ResourceTables[typ.Index]
    if !rt.Concrete {
        return types.Val{}, fmt.Errorf(
            "cannot lift abstract resource at runtime (type %d)", typ.Index)
    }

    // Resolve the nominal *ResourceType. The resource was defined by
    // some instance (rt.Instance). Find that instance and fetch its
    // ResourceTypes[rt.Resource]. For same-instance use, this is just
    // ctx.Instance.ResourceTypes[rt.Resource]. For cross-instance use,
    // the runtime walks the instance tree (via Parent pointers or a
    // top-level instance map maintained by the linker).
    //
    // In Session 0, ctx.Instance.ResourceTypes is empty (Concrete
    // promotion is Session 2 work), so this traps for any real handle.
    expectedRT := ctx.Instance.LookupResourceType(rt.Instance, rt.Resource)
    if expectedRT == nil {
        return types.Val{}, fmt.Errorf(
            "no resource type for instance %d declaration %d "+
            "(resource concrete promotion not yet wired — session 2)",
            rt.Instance, rt.Resource)
    }

    handleIdx := iter.NextI32()
    h := runtime.Handle(handleIdx)

    // Validate via pointer-identity comparison — spec's `is` check.
    if err := ctx.Instance.Table.ValidateType(h, expectedRT); err != nil {
        return types.Val{}, err
    }

    // For own<>: transfer ownership via Table.Remove (decrements the
    // table entry, returns the rep).
    entry, err := ctx.Instance.Table.RemoveResourceHandle(h)
    if err != nil {
        return types.Val{}, err
    }
    // ... convert entry.Rep into the Val representation
    return types.ValOwn(handleIdx), nil
```

The `TypeKindBorrow` arm is analogous but calls `IncrementLends` instead of removing the entry, and tracks the borrow in the per-call borrow scope (`ctx.BorrowScope` on `LiftContext`).

Same-instance borrow optimization from `CanonicalABI.md:2677-2683` folds into the `TypeKindBorrow` lower arm. The current calling instance is `ctx.Instance` itself (per the spec, each call is performed by a specific instance). Comparison is pointer-identity on `*ComponentInstance`:

```go
case types.TypeKindBorrow:
    rt := ctx.Types.ResourceTables[typ.Index]
    expectedRT := ctx.Instance.LookupResourceType(rt.Instance, rt.Resource)
    if expectedRT == nil { /* trap */ }

    // Same-instance optimization: if the current calling instance is
    // the one that defined the resource, return rep directly without
    // creating a handle. This matches cx.inst is t.rt.impl at
    // definitions.py:1647.
    if ctx.Instance == expectedRT.Impl {
        // ... return rep directly, no handle allocation
    }

    // Cross-instance: allocate a borrow handle in the caller's Table.
    // ... create ResourceHandleEntry, call ctx.Instance.Table.Add(...)
```

## Runtime Instance — `runtime.ComponentInstance`

Single-layer struct matching the spec's `ComponentInstance` at `definitions.py:256-273`. Each instantiated component — top-level or nested — is its own self-contained `ComponentInstance` with its own `Table`, its own `MayLeave` flag, and its own `ResourceTypes` pool. Nesting is tracked via an optional `Parent` pointer.

### File: `internal/component/runtime/component_instance.go` (new)

```go
package runtime

import "github.com/tetratelabs/wazero/internal/component/types"

// ComponentInstance is the runtime state for one instantiated component
// (top-level or nested). Matches the spec's ComponentInstance at
// definitions.py:256-273.
//
// One ComponentInstance per instantiation. For nested instantiation,
// Parent points to the parent instance. For top-level instances, Parent
// is nil. Each instance owns its own Table, its own MayLeave flag, and
// its own ResourceTypes pool.
type ComponentInstance struct {
    // ID is a monotonically-assigned runtime instance identifier.
    ID uint32

    // Parent is the parent component instance for nested instantiation,
    // or nil for top-level instances. Matches spec field `parent`
    // (definitions.py:258).
    Parent *ComponentInstance

    // Table is the unified handle table for this instance. Holds
    // resource handles today; streams, futures, error-contexts, and
    // subtasks share this table when async lands. Handle indices are
    // unique across all handle kinds within this instance.
    // Matches spec field `table` (definitions.py:259, class Table
    // at :303-315).
    Table *Table

    // MayLeave is the may_leave flag. Set to false during canon.task.enter
    // and restored after canon.task.exit. Operations like canon.resource.new
    // trap if !MayLeave. Matches spec field `may_leave`
    // (definitions.py:260, 270, 1955, 1973, 2065, 2135, 2143).
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

// IsMayLeave returns whether the instance may leave — both MayLeave
// is true and enterCount is zero. Matches wazero's existing MayLeave()
// semantics.
func (c *ComponentInstance) IsMayLeave() bool {
    return c.MayLeave && c.enterCount == 0
}

// LookupResourceType resolves a (RuntimeComponentInstanceIdx, ResourceIdx)
// pair from a TypeResourceTable entry to the nominal *ResourceType.
// Walks the instance tree to find the defining instance, then returns
// the ResourceTypes[ResourceIdx] entry.
//
// Returns nil if the target instance is not found or the resource
// type slot is not yet populated (Session 0 state).
func (c *ComponentInstance) LookupResourceType(
    instanceIdx types.RuntimeComponentInstanceIdx,
    resourceIdx types.ResourceIdx,
) *ResourceType {
    // Find the instance with matching ID by walking the tree.
    // Implementation detail: for Session 0 the tree is a single-node
    // linear hierarchy (a single top-level instance), so this walk
    // always returns self or nil. Session 2 extends this to real
    // tree navigation once nested instantiation is wired.
    target := c.findInstance(uint32(instanceIdx))
    if target == nil {
        return nil
    }
    if int(resourceIdx) >= len(target.ResourceTypes) {
        return nil
    }
    return target.ResourceTypes[resourceIdx]
}

// findInstance walks the instance tree to find the instance with the
// given ID. Walks parents first, then siblings if any are tracked.
func (c *ComponentInstance) findInstance(id uint32) *ComponentInstance {
    for inst := c; inst != nil; inst = inst.Parent {
        if inst.ID == id {
            return inst
        }
    }
    return nil
}
```

### Deleted file: `internal/component/runtime/instance_state.go`

Wazero's existing `runtime.InstanceState` (`runtime/instance_state.go`, ~60 lines) is **deleted**. Its fields (`id`, `enterCount`, `mayLeave`) move to `runtime.ComponentInstance` (renamed `id → ID`, `mayLeave → MayLeave`, `enterCount` kept lowercase). Its methods (`ID()`, `MayLeave()`, `SetMayLeave`, `Enter()`, `Leave()`, `EnterCount()`) move to `ComponentInstance` — with `MayLeave()` renamed to `IsMayLeave()` to avoid colliding with the new exported field.

`runtime/instance_state_test.go` — updated or merged into a new `component_instance_test.go`.

Every caller that used `*runtime.InstanceState` — including `runtime/resource_table.go:133` (`NewWithMayLeaveCheck`), and anywhere else `InstanceState` is passed around — updates to `*runtime.ComponentInstance`.

### Dependency direction

- `runtime/` already imports `types/` via `runtime/subtask.go:8`. The new `types.RuntimeComponentInstanceIdx` / `types.ResourceIdx` references in `LookupResourceType` extend this existing edge. No new packages imported.
- `runtime.ComponentInstance` holds no references to `component/` or `abi/`.
- `abi/` already imports `runtime/`; carrying `Instance *runtime.ComponentInstance` on `LiftContext`/`LowerContext` adds no new import.
- `types/` does not gain a dependency on `runtime/`.

## Builder

```go
package types

// ComponentTypesBuilder assembles a *ComponentTypes during decoding.
// After Finish() the builder is consumed; further Intern* calls panic.
// Go equivalent of Rust's "consumed self" idiom. The returned
// *ComponentTypes is safe for concurrent reads.
type ComponentTypesBuilder struct {
    ct       ComponentTypes
    finished bool

    // Intern maps per kind. Keys are hashes of the structural content.
    // Hash collisions resolved by scanning the bucket's slice and doing
    // a per-kind structural equality check. Maps are dropped in Finish
    // so the returned *ComponentTypes is cheap to retain.
    recordIntern        map[uint64][]uint32 // hash -> candidate Record indices
    variantIntern       map[uint64][]uint32
    listIntern          map[uint64][]uint32
    tupleIntern         map[uint64][]uint32
    flagsIntern         map[uint64][]uint32
    enumIntern          map[uint64][]uint32
    optionIntern        map[uint64][]uint32
    resultIntern        map[uint64][]uint32
    streamIntern        map[uint64][]uint32
    futureIntern        map[uint64][]uint32
    errCtxIntern        map[uint64][]uint32
    resourceTableIntern map[uint64][]uint32
    funcIntern          map[uint64][]uint32
}

func NewComponentTypesBuilder() *ComponentTypesBuilder {
    return &ComponentTypesBuilder{
        recordIntern:        map[uint64][]uint32{},
        // ... etc.
    }
}

// Intern methods. Each computes a hash of the structural content,
// scans the bucket for an existing match, and either returns the
// existing ValType or appends a new entry (with precomputed ABI) and
// returns the new ValType.
//
// Precondition: the type arguments must already have been interned —
// the builder is strictly bottom-up. Callers never pass uninterned
// composite content.
func (b *ComponentTypesBuilder) InternRecord(fields []RecordField) ValType
func (b *ComponentTypesBuilder) InternVariant(cases []VariantCase) ValType
func (b *ComponentTypesBuilder) InternList(elem ValType) ValType
func (b *ComponentTypesBuilder) InternFixedLengthList(elem ValType, length uint32) ValType
func (b *ComponentTypesBuilder) InternTuple(elems []ValType) ValType
func (b *ComponentTypesBuilder) InternFlags(names []string) ValType
func (b *ComponentTypesBuilder) InternEnum(names []string) ValType
func (b *ComponentTypesBuilder) InternOption(elem ValType) ValType
func (b *ComponentTypesBuilder) InternResult(okType, errType ValType, hasOk, hasErr bool) ValType
func (b *ComponentTypesBuilder) InternStream(elem ValType, hasElem bool) ValType
func (b *ComponentTypesBuilder) InternFuture(elem ValType, hasElem bool) ValType
func (b *ComponentTypesBuilder) InternErrorContextTable() ValType

// InternAbstractResource creates a new Abstract TypeResourceTable entry
// and returns the index. Used by the decoder when processing a
// `(type $r (resource ...))` declaration — Concrete promotion happens
// later at instantiation time (Session 2).
func (b *ComponentTypesBuilder) InternAbstractResource() ResourceTableIdx

// InternOwnHandle / InternBorrowHandle take a ResourceTableIdx and
// return the matching ValType. These are trivial wrappers but exist
// so the decoder can use consistent Intern* naming throughout.
func (b *ComponentTypesBuilder) InternOwnHandle(rtIdx ResourceTableIdx) ValType {
    return ValType{Kind: TypeKindOwn, Index: uint32(rtIdx)}
}
func (b *ComponentTypesBuilder) InternBorrowHandle(rtIdx ResourceTableIdx) ValType {
    return ValType{Kind: TypeKindBorrow, Index: uint32(rtIdx)}
}

// InternFunc interns a function type. paramNames is preserved order;
// params and results must each be a TypeKindTuple ValType produced by
// a prior InternTuple call (or equivalent).
func (b *ComponentTypesBuilder) InternFunc(async bool, paramNames []string, params, results ValType) FuncTypeIdx

// Finish freezes the builder and returns the immutable *ComponentTypes.
// After Finish, further Intern* calls panic with "builder already
// finished". The intern maps are nilled out so the returned
// *ComponentTypes carries only the slices.
func (b *ComponentTypesBuilder) Finish() *ComponentTypes
```

### Intern keys per kind (exhaustive)

Each `Intern<Kind>` method hashes the kind's structurally-significant fields into a `uint64` bucket key, then scans the bucket for a structural match. The fields below are the complete set for each kind. Any addition to a composite struct that introduces a new structurally-significant field must be reflected here.

- **`InternRecord(fields []RecordField) ValType`**
  - Key includes: `len(fields)`, then for each field in order: `field.Name` (full string), `field.Type.Kind` (uint8), `field.Type.Index` (uint32).
  - Two records with the same field set in different *order* are distinct (order matters in the spec).

- **`InternVariant(cases []VariantCase) ValType`**
  - Key includes: `len(cases)`, then for each case in order: `case.Name`, `case.HasPayload` (bool), `case.Payload.Kind`, `case.Payload.Index`.

- **`InternList(elem ValType) ValType`** (dynamic lists)
  - Key includes: `elem.Kind`, `elem.Index`.
  - `list<u32>` and `list<u32>` collapse to one entry.

- **`InternFixedLengthList(elem ValType, length uint32) ValType`** (fixed-length lists)
  - Key includes: `elem.Kind`, `elem.Index`, `length`.
  - `list<u32, 5>` and `list<u32, 7>` are distinct. `list<u32>` (dynamic) and `list<u32, 5>` (fixed) are distinct because they live in different intern maps (and produce different `TypeKind` values).
  - Distinct from dynamic lists because spec and wasmtime treat fixed-length lists as a distinct type kind with distinct structural identity.

- **`InternTuple(elems []ValType) ValType`**
  - Key includes: `len(elems)`, then for each elem in order: `elem.Kind`, `elem.Index`.

- **`InternFlags(names []string) ValType`**
  - Key includes: `len(names)`, then each name in order.

- **`InternEnum(names []string) ValType`**
  - Key includes: `len(names)`, then each name in order.

- **`InternOption(elem ValType) ValType`**
  - Key includes: `elem.Kind`, `elem.Index`.

- **`InternResult(okType, errType ValType, hasOk, hasErr bool) ValType`**
  - Key includes: `hasOk`, `hasErr`, then conditionally `okType.Kind`/`okType.Index` (only if `hasOk`) and `errType.Kind`/`errType.Index` (only if `hasErr`).
  - `result<_, _>`, `result<u32, _>`, `result<_, error>`, `result<u32, error>` are four distinct interned entries.

- **`InternStream(elem ValType, hasElem bool) ValType`**
  - Key includes: `hasElem`, then conditionally `elem.Kind`/`elem.Index`.
  - `stream<>` and `stream<u32>` are distinct.

- **`InternFuture(elem ValType, hasElem bool) ValType`**
  - Key includes: `hasElem`, then conditionally `elem.Kind`/`elem.Index`.

- **`InternErrorContextTable() ValType`**
  - No key — there is only one `TypeErrorContextTable` per component in Session 0 (the empty struct). First call appends the entry and returns its index; subsequent calls return the same index. Session 2 may expand this with per-instance table identity.

- **`InternAbstractResource() ResourceTableIdx`**
  - **No interning** — every call returns a fresh index. Abstract resource declarations are distinct by construction (each `(type $r (resource ...))` in the binary is a new nominal declaration, even if structurally identical). The intern map for resource tables is only used in Session 2 when Concrete entries with identical `(Resource, Instance)` pairs can dedup.

- **`InternFunc(async bool, paramNames []string, params, results ValType) FuncTypeIdx`**
  - Key includes: `async`, `len(paramNames)`, each name in order, `params.Kind`, `params.Index`, `results.Kind`, `results.Index`.
  - Two function types with the same signature but different parameter *names* are distinct (names are part of the component-level signature).

The planning agent implements exactly these keys. A test per kind verifies that structurally-distinct-but-nearly-identical types do not collapse into one entry (and that structurally-identical types do).

## Decoder Flow

### Scope-local index tracking

The binary format assigns type-section indices that are scope-local: a top-level component, a nested instance type, and a nested component type each have their own type index spaces. Aliases (`outer`, `export`) pull a type from one scope into another, consuming an index in the destination scope.

Decoder maintains a stack of scope-local tagged entries:

```go
// typeScope tracks scope-local type indices during decode. Each scope
// is a flat []scopeEntry — binary scope-local index N corresponds to
// scope.entries[N]. Aliases append to the slice; definitions append AND
// call through to the builder.
//
// parent chains up the scope hierarchy so `outer` aliases can resolve
// across scopes.
//
// The scopeEntry and scopeEntryKind types are defined in the "Resource
// declarations" subsection below — entries are tagged so that resource
// declarations (which are not value types) are distinguishable from
// value types that happen to produce ValType values. See that section
// for the full scopeEntry definition and the rejection rules.
type typeScope struct {
    entries []scopeEntry
    parent  *typeScope
}
```

### decodeValType

```go
// New signature and behavior. Replaces the current
// internal/component/binary/types.go:138 decodeValType.
func decodeValType(
    r *bytes.Reader,
    scope *typeScope,
    b *types.ComponentTypesBuilder,
) (types.ValType, error) {
    opcode, err := r.ReadByte()
    if err != nil {
        return types.ValType{}, err
    }

    // Primitive: direct mapping, no builder interaction.
    if IsPrimValType(opcode) {
        return primitiveOpcodeToValType(opcode), nil
    }

    // own<R>
    if opcode == ValTypeOpcodeOwn {
        resIdx, _, err := leb128.DecodeUint32(r)
        if err != nil {
            return types.ValType{}, err
        }
        if int(resIdx) >= len(scope.entries) {
            return types.ValType{}, fmt.Errorf("own<> type index %d out of range", resIdx)
        }
        entry := scope.entries[resIdx]
        if entry.kind != scopeEntryResource {
            return types.ValType{}, fmt.Errorf("own<> references type index %d which is not a resource declaration", resIdx)
        }
        return b.InternOwnHandle(entry.resource), nil
    }

    // borrow<R>
    if opcode == ValTypeOpcodeBorrow {
        resIdx, _, err := leb128.DecodeUint32(r)
        if err != nil {
            return types.ValType{}, err
        }
        if int(resIdx) >= len(scope.entries) {
            return types.ValType{}, fmt.Errorf("borrow<> type index %d out of range", resIdx)
        }
        entry := scope.entries[resIdx]
        if entry.kind != scopeEntryResource {
            return types.ValType{}, fmt.Errorf("borrow<> references type index %d which is not a resource declaration", resIdx)
        }
        return b.InternBorrowHandle(entry.resource), nil
    }

    // Type index reference: look up the scope entry and expect a value type.
    r.UnreadByte()
    idx, _, err := leb128.DecodeUint32(r)
    if err != nil {
        return types.ValType{}, err
    }
    if int(idx) >= len(scope.entries) {
        return types.ValType{}, fmt.Errorf("type index %d out of range", idx)
    }
    entry := scope.entries[idx]
    if entry.kind != scopeEntryValType {
        return types.ValType{}, fmt.Errorf("type index %d refers to a non-value-type declaration (kind %d)", idx, entry.kind)
    }
    return entry.valType, nil
}
```

### decodeDefinedType

```go
// Replaces binary/types.go:302 decodeDefinedType. Returns a ValType
// instead of *TypeDef.
func decodeDefinedType(
    r *bytes.Reader,
    scope *typeScope,
    b *types.ComponentTypesBuilder,
) (types.ValType, error) {
    opcode, err := r.ReadByte()
    if err != nil {
        return types.ValType{}, err
    }
    switch opcode {
    case ValTypeOpcodeRecord:
        return decodeRecord(r, scope, b)
    case ValTypeOpcodeVariant:
        return decodeVariant(r, scope, b)
    case ValTypeOpcodeList:
        return decodeList(r, scope, b)
    case ValTypeOpcodeFixedSizeList: // 0x67
        return decodeFixedLengthList(r, scope, b)
    case ValTypeOpcodeTuple:
        return decodeTuple(r, scope, b)
    case ValTypeOpcodeFlags:
        return decodeFlags(r, b)
    case ValTypeOpcodeEnum:
        return decodeEnum(r, b)
    case ValTypeOpcodeOption:
        return decodeOption(r, scope, b)
    case ValTypeOpcodeResult:
        return decodeResult(r, scope, b)
    case ValTypeOpcodeStream: // 0x66
        return decodeStream(r, scope, b)
    case ValTypeOpcodeFuture: // 0x65
        return decodeFuture(r, scope, b)
    // Note: error-context primitive is 0x64, handled in IsPrimValType
    default:
        return types.ValType{}, fmt.Errorf("unknown defined type opcode: 0x%02x", opcode)
    }
}
```

Each `decode<Kind>` helper reads the binary payload, recursively calls `decodeValType` for child types (which are already interned), and calls the matching builder `Intern` method. The `types.ValType` result is appended to the current scope's `types` slice.

### Resource declarations

```go
// Called from the type-section dispatch when opcode is 0x3f (sync
// resource) or 0x3e (async resource). Creates an Abstract resource
// table entry and records it in the current scope as a resource-kind
// entry (not a value type). See the scope entry shape below.
func decodeResourceDecl(
    r *bytes.Reader,
    scope *typeScope,
    b *types.ComponentTypesBuilder,
    isAsync bool,
) error {
    // Parse destructor index, async callback, etc. from the binary.
    // (decodeResourceTypeDefWithAsync in the current binary/types.go:759
    // has the LEB128 parsing logic that survives this rewrite.)

    // Create an Abstract TypeResourceTable entry in ComponentTypes.
    rtIdx := b.InternAbstractResource()

    // Record in the scope as a resource-kind entry. The binary's next
    // type-section index consumes this slot; any subsequent own<N> or
    // borrow<N> reference that reads scope.entries[N] will find a
    // scopeEntryResource entry and unwrap the ResourceTableIdx.
    scope.entries = append(scope.entries, scopeEntry{
        kind:     scopeEntryResource,
        resource: rtIdx,
    })
    return nil
}
```

A resource declaration is not a value type and must not be reachable from lift/lower as one. The scope-local index space is a single flat slice in the binary format — type sections can mix declarations of different kinds (value types, resource declarations, and Session 2 instance/component types). Each scope entry is tagged with its kind:

```go
// In internal/component/binary/types.go (or binary/decoder.go).
type scopeEntryKind uint8

const (
    scopeEntryValType scopeEntryKind = iota // value type (record, variant, list, etc.)
    scopeEntryResource                       // resource declaration
    // Session 2: scopeEntryInstance, scopeEntryComponent for instance/component types
)

type scopeEntry struct {
    kind     scopeEntryKind
    valType  types.ValType          // valid iff kind == scopeEntryValType
    resource types.ResourceTableIdx // valid iff kind == scopeEntryResource
}
```

Lookup rules:

- `decodeValType` seeing an index reference (LEB128 uint): look up `scope.entries[idx]`, require `kind == scopeEntryValType`, return `entry.valType`. If the entry is a resource declaration, return a decode error.
- `decodeValType` seeing the own<> opcode (0x69) or borrow<> opcode (0x68): read the LEB128 uint, look up `scope.entries[idx]`, require `kind == scopeEntryResource`. Call `b.InternOwnHandle(entry.resource)` or `b.InternBorrowHandle(entry.resource)` to produce the ValType. If the entry is a value type, return a decode error.

This rejects ill-formed input ("record field of type own<5>, where 5 is itself a record") at decode time with a precise error rather than producing a confusing ValType that might reach lift/lower.

## abi/ Consumer Flow

### LiftContext and LowerContext

Current shapes at `abi/context.go:66-71, 154-167`:

```go
// CURRENT (will be replaced)
type LiftContext struct {
    Memory        api.Memory
    Opts          *Options
    ResourceTable *runtime.ResourceTable  // single table, wrong model
    BorrowScope   *runtime.BorrowScope
}

type LowerContext struct {
    Memory        api.Memory
    Opts          *Options
    Realloc       func(oldPtr, oldSize, align, newSize uint32) (uint32, error)
    ResourceTable *runtime.ResourceTable
    CallContext   *runtime.CallContext
    Instance      interface{} // TODO at line 163 — no typed ComponentInstance existed
    Subtask       *runtime.Subtask
}
```

New shapes (Session 0), matching wasmtime's single-instance-reference pattern for per-instance state while keeping per-call state as separate context fields:

```go
// NEW
type LiftContext struct {
    Memory      api.Memory
    Opts        *Options
    Types       *types.ComponentTypes       // per-component type bag
    Instance    *runtime.ComponentInstance  // the calling instance
    BorrowScope *runtime.BorrowScope         // per-call borrow scope
}

type LowerContext struct {
    Memory      api.Memory
    Opts        *Options
    Realloc     func(oldPtr, oldSize, align, newSize uint32) (uint32, error)
    Types       *types.ComponentTypes
    Instance    *runtime.ComponentInstance  // the calling instance
    CallContext *runtime.CallContext         // per-call context
}
```

**Field semantics:**

- `Instance *runtime.ComponentInstance` is the component instance that is **performing** this lift/lower call. Per the spec, each call is associated with a specific instance — the one whose `may_leave` flag gates the operation, whose `table` holds new handle allocations, and whose identity is compared against resource-type `Impl` for the same-instance borrow optimization. For nested instantiation, navigating to a parent instance or a sibling uses `ctx.Instance.Parent` or `ctx.Instance.LookupResourceType(...)`.
- `BorrowScope` (on `LiftContext`) and `CallContext` (on `LowerContext`) are **per-call** state, unchanged from the current shape.
- The existing `LowerContext.Subtask *runtime.Subtask` field (`context.go:166`) is **not carried in sync `LowerContext`**. Subtasks are async machinery; async lift/lower is deferred to a later session (Decision 8). The `LowerContext.BorrowScope() *runtime.BorrowScope` helper method at `context.go:170-175` that derived a borrow scope from the Subtask is deleted along with the Subtask field.
- The existing `LowerContext.ResourceTable *runtime.ResourceTable` and `LiftContext.ResourceTable *runtime.ResourceTable` fields are **deleted**. The per-instance unified handle table lives on `ctx.Instance.Table` directly.
- No `CurrentSubInstance` field. The spec's model is single-layer per instance; the "current sub-instance" IS `ctx.Instance`, directly. Cross-instance operations use `ctx.Instance.LookupResourceType` to resolve a resource type from its defining instance (which may be `ctx.Instance` itself, a parent, or a foreign instance found via a store-level lookup in Session 2+).

### Dispatch shape

Current (`abi/lift.go:52-351`): type-switch on the interface form.

```go
func LiftFlat(ctx *LiftContext, typ types.ValType, iter *FlatIter) (types.Val, error) {
    switch typ.(type) {
    case types.Bool:
        return types.ValBool(iter.NextI32() != 0), nil
    case types.Record:
        t := typ.(types.Record)
        ...
    // NOTE: no case for Own or Borrow — currently a gap, falls through to default
    }
}
```

New: kind-switch with per-kind reads from `ctx.Types`, and full Own/Borrow coverage:

```go
func LiftFlat(ctx *LiftContext, typ types.ValType, iter *FlatIter) (types.Val, error) {
    switch typ.Kind {
    case types.TypeKindBool:
        return types.ValBool(iter.NextI32() != 0), nil
    case types.TypeKindS8:
        return types.ValS8(int8(iter.NextI32())), nil
    // ... primitives

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
        // ... discriminant read, case selection, payload lift

    case types.TypeKindList:
        list := &ctx.Types.Lists[typ.Index]
        // Dynamic list: read ptr+len from flat, then lift elements from memory
        _ = list

    case types.TypeKindFixedList:
        fl := &ctx.Types.FixedLists[typ.Index]
        // Fixed-length list: lift `fl.Length` elements inline from flat
        _ = fl

    case types.TypeKindOwn:
        // Read TypeResourceTable entry, check Concrete, resolve the
        // expected *runtime.ResourceType via
        // ctx.Instance.LookupResourceType(rt.Instance, rt.Resource),
        // validate the handle against that *ResourceType by pointer
        // identity via ctx.Instance.Table.ValidateType, then transfer
        // ownership via RemoveResourceHandle. Session 0 traps when
        // ResourceTypes is empty (Session 2 wires Concrete promotion).
        // See the Resource Identity section for the pseudocode.

    case types.TypeKindBorrow:
        // Same structure as KindOwn, but calls IncrementLends instead
        // of RemoveResourceHandle and tracks the borrow in
        // ctx.Instance.Reentrance / ctx.BorrowScope. Includes the
        // same-instance optimization (CanonicalABI.md:2677-2683) on
        // the lower side by pointer-comparing
        // ctx.Instance == expectedRT.Impl.

    case types.TypeKindStream, types.TypeKindFuture, types.TypeKindErrorContext:
        return types.Val{}, fmt.Errorf(
            "component-model async types not yet supported: kind=%s", typ.Kind)
    // ... other kinds

    default:
        return types.Val{}, fmt.Errorf("LiftFlat: unknown TypeKind %d", typ.Kind)
    }
}
```

Same shape for `LiftHeap`, `LowerFlat`, `LowerHeap`, and the `flatten.go` helpers. All read composite content via `ctx.Types.<slice>[typ.Index]` and all four entry points now have full Own/Borrow coverage.

### Standalone resource helpers go away

`internal/component/abi/resource_lower.go` (`LowerBorrowWithType`, `LowerOwnWithType`) and `internal/component/abi/lower.go:683-712` (`LowerOwn`, `LowerBorrow`) are **deleted**. Their logic is inlined into the `TypeKindOwn` and `TypeKindBorrow` dispatch arms. This removes a parallel helper layer and ensures the dispatch switches are the single source of resource lift/lower.

The test file `internal/component/abi/resource_lower_test.go` is also deleted. Its test coverage is moved into new tests on the dispatch switches.

### Flatten / CoreSignature

`abi/flatten.go` functions (`FlattenParams`, `FlattenResults`, `flattenType`, `CoreSignature`) gain a `ct *types.ComponentTypes` parameter. Their bodies dispatch on `TypeKind` the same way. Note that `flatten.go:82` already handles `case types.Own, types.Borrow: return []api.ValueType{api.ValueTypeI32}` correctly — the flattening side is already consistent; only the lift/lower dispatch has the gap.

## Work Order (Session 0)

Each numbered step is independently reviewable. Steps with shared dependencies run in parallel if practical.

1. **types/types.go** — Add `TypeKind` enum, `ValType` struct, `ComponentTypes` struct, scalar constants. Simultaneously delete the old `ValType` interface and the scalar struct types (`Bool`, `S8`, …, `String`) — they collide on the `ValType` name, so they must go together. Build breaks from here until the end of the session.
2. **types/composite.go** — Replace inline composite structs with the table-entry structs (`TypeRecord`, `TypeVariant`, `TypeList`, `TypeTuple`, `TypeFlags`, `TypeEnum`, `TypeOption`, `TypeResult`, `TypeStream`, `TypeFuture`, `TypeErrorContextTable`, `TypeFunc`) and helper structs (`RecordField`, `VariantCase`). Depends on step 1. Parallel-safe with step 3.
3. **types/resource.go** — Delete the old `Own` / `Borrow` struct types. Delete the existing limited `ResourceType` struct at `types/resource.go:61-80` (the one with the `InstanceID uint32` field — this is only a structural layer; the new nominal-identity `ResourceType` lives in `runtime/`, see step 7a). Add `TypeResourceTable`, `ResourceIdx`, `RuntimeComponentInstanceIdx`, `ResourceTableIdx`. Depends on step 1. Parallel-safe with step 2.
4. **types/abi_info.go** (new file) — Create `CanonicalABIInfo`, `DiscriminantInfo`, the `scalarABI` table, and `(ValType).ABI` accessor. Depends on steps 1-3.
5. **types/builder.go** (new file) — Create `ComponentTypesBuilder` with all `Intern*` methods and `Finish`. Include intern-key functions per kind. Depends on steps 1-4.
6. **types/val.go** — Add `ValKindStream`, `ValKindFuture`, `ValKindErrorContext` constants. Update `ValKind.String()`. No constructor changes. Independent, parallel-safe with 1-5.
7. **runtime/resource_type.go** (new file) — Create `runtime.ResourceType` struct with pointer-identity semantics: `{Impl *ComponentInstance, Dtor *uint32, DtorAsync bool, DtorCallback *uint32}`. Add `HasDestructor()` method. Imports `types/` transitively via the package already. Decision 5. Depends on none (leaf struct definition). Parallel-safe with 1-6.

7a. **runtime/table.go** (renamed from `runtime/resource_table.go`, heavily rewritten) — Rename the existing `ResourceTable` type to `Table`. Rename the existing `HandleEntry` to `ResourceHandleEntry` and change `RT ResourceTypeID` field to `RT *ResourceType`. Introduce `TableEntry` interface that `ResourceHandleEntry` (and future Stream/Future/Subtask/ErrorContext entries) implements. Rewrite `NewWithType` → `NewResourceHandle(rep any, own bool, rt *ResourceType)`. Rewrite `ValidateType(h Handle, expected *ResourceType) error` to do pointer comparison (fixing the existing cross-instance-identity bug). Rewrite `GetType` → `GetResourceType(h Handle) (*ResourceType, error)` via type assertion. Rewrite `CreateResourceNewFunc`, `CreateResourceDropFunc`, `CreateResourceRepFunc` and their WithType/WithTrap/WithContext variants to take `*ResourceType` arguments. Decision 5, Decision 6. Depends on step 7.

7b. **runtime/resource_table_test.go** — 1240-line test file updated to use new `Table`/`*ResourceType` signatures. Mechanical but voluminous. Depends on step 7a.

7c. **runtime/component_instance.go** (new file) — Create single-layer `runtime.ComponentInstance` struct matching the spec at `definitions.py:256-273`: `{ID uint32, Parent *ComponentInstance, Table *Table, MayLeave bool, enterCount int, ResourceTypes []*ResourceType, Destructors *DestructorRegistry, Reentrance *ReentranceTracker}`. Add methods `NewComponentInstance(id, parent)`, `Enter()`, `Leave()`, `EnterCount()`, `IsMayLeave()`, and `LookupResourceType(instanceIdx, resourceIdx) *ResourceType` for cross-instance type resolution via `Parent` walking. Imports `types/` for `RuntimeComponentInstanceIdx` / `ResourceIdx`. Decision 6. Depends on steps 7, 7a.

7d. **runtime/instance_state.go** — **deleted entirely**. Its fields and methods are merged into `runtime.ComponentInstance` in step 7c. `runtime/instance_state_test.go` is updated/merged into a new `component_instance_test.go`. Depends on step 7c.

7e. **runtime/resource_type_id.go** — **deleted entirely** (file). `ResourceTypeID`, `NewResourceTypeID`, `InvalidResourceTypeID`, `ResourceTypeInfo`, `NewResourceTypeInfo` helpers all go away. Every caller updates to direct `*ResourceType` usage. `runtime/resource_type_id_test.go` is also deleted. Depends on steps 7, 7a.
8. **types/*_test.go** — Rewrite tests for the new shape. Add new tests for interning dedup, ABI precomputation, builder freeze enforcement. Depends on steps 1-6.
9. **internal/component/component.go** — Replace `Component.Types []TypeDef` with `Component.Types *types.ComponentTypes`. Delete `Component.TypeIdxToStoredIdx`. Collapse `TypeDef` composite pointer fields to a single `types.ValType`. **Delete `FuncType` struct entirely** (Decision 7) — call sites switch to `*types.TypeFunc`. **Delete `NamedValType` struct entirely** (Decision 7) — the `(name, type)` pairing is carried by `types.TypeFunc.ParamNames` + the params/results tuple ValTypes. Delete `ValTypeRef` entirely. Delete `RecordTypeDef`, `VariantTypeDef`, `ListTypeDef`, `TupleTypeDef`, `FlagsTypeDef`, `EnumTypeDef`, `OptionTypeDef`, `ResultTypeDef`, `StreamTypeDef`, `FutureTypeDef`, `FixedSizeListTypeDef` structs. **Do NOT rename `ParsedComponentInstance`** — already renamed to that name before this session. Depends on steps 1-5.
10. **internal/component/binary/** — Rewrite `decodeValType`, `decodeDefinedType`, type-section-dispatch, and `decode<Kind>TypeDef` functions to produce `types.ValType` via the builder. Delete `binary.TypeDef`'s composite fields. Update all call sites that previously produced `component.FuncType` or `component.NamedValType` to produce `types.TypeFunc` via `ComponentTypesBuilder.InternFunc`. Add `typeScope` machinery for scope-local index tracking. Resolve the resource-declaration-vs-valtype representation question. Depends on steps 5, 9.
11. **internal/component/abi/context.go** — Rewrite `LiftContext` and `LowerContext` structs. Drop `ResourceTable` and `Subtask` fields. Drop the `BorrowScope()` helper method that derived from Subtask. Drop the `interface{} Instance` TODO. Add `Types *types.ComponentTypes` and `Instance *runtime.ComponentInstance` fields. Keep `BorrowScope *runtime.BorrowScope` on `LiftContext` and `CallContext *runtime.CallContext` on `LowerContext` as per-call state. The `ReadU8`/`ReadU16`/... memory helpers are unchanged. Depends on steps 1-7e.
12. **internal/component/abi/lift.go, lower.go, flatten.go** — Rewrite dispatch to `switch typ.Kind` and read composite content via `ctx.Types.<slice>[typ.Index]`. Add `case types.TypeKindOwn:` and `case types.TypeKindBorrow:` arms with inlined resource-lookup logic:
    - Read `TypeResourceTable` entry from `ctx.Types.ResourceTables[typ.Index]`.
    - If `!rt.Concrete`, trap.
    - Resolve `expectedRT := ctx.Instance.LookupResourceType(rt.Instance, rt.Resource)`. If nil, trap with "no resource type for instance N declaration M (session 2 wiring)".
    - Validate handle via `ctx.Instance.Table.ValidateType(handle, expectedRT)` — pointer-identity comparison against `*ResourceType`.
    - For `Own`: `RemoveResourceHandle` to transfer ownership; return `ValOwn(handle)`.
    - For `Borrow`: `IncrementLends`; track in per-call borrow scope; apply same-instance optimization (`ctx.Instance == expectedRT.Impl` → return rep directly, no handle allocation, per `CanonicalABI.md:2677-2683`).

    Add trap arms for `TypeKindStream`/`TypeKindFuture`/`TypeKindErrorContext`. Update `FlattenParams`, `FlattenResults`, `flattenType`, `CoreSignature` to take `*types.ComponentTypes` and dispatch on `TypeKind`. Depends on steps 1-6, 11.

12a. **External runtime symbol callers — V5b corrective sweep.** Run the audit command from the V5b verification section to find every caller of the renamed/deleted runtime symbols across `api/`, `imports/`, and `internal/` (not just `internal/component/`). For each hit, apply the per-symbol fix from the V5b table:

    - **`api/component/component.go`** — update the `type ResourceTable = runtime.ResourceTable` alias and `var NewResourceTable = runtime.NewResourceTable` to point at the renamed `runtime.Table` and `runtime.NewTable`. Update `WithResourceTable` and `ResourceTableFromContext` accessor signatures and bodies. Pre-production status (Non-Goals) permits public surface change.

    - **`imports/wasip2/io/{streams,error,poll}.go`** — at the top of `streams.go`, declare per-kind `*runtime.ResourceType{}` singletons (`inputStreamResourceType`, `outputStreamResourceType`, `pollableResourceType`, `errorResourceType`) per the "Host-managed resource types" pattern in the Resource Identity section. Replace `table.New(rep, true)` calls with `table.NewResourceHandle(rep, true, <kind>ResourceType)`. Replace `entry.Rep` accesses with `entry.(*runtime.ResourceHandleEntry).Rep` (type assertion required because `Table.Get` returns the generic `runtime.TableEntry` interface). Existing `Destroy()` methods on `*InputStream`/`*OutputStream`/`*Error` are unchanged — host destructors continue to flow through the `runtime.Destroyable` interface.

    - **Other `imports/wasip2/{filesystem,sockets,http,cli,clocks,...}` host modules** — apply the same pattern to any flagged hits. Each module declares its own per-kind `*runtime.ResourceType` singletons.

    - **`internal/component/wasip2test/*.go`** — update test fixtures that call `runtime.NewResourceTypeID()` (e.g., `kv_store_test.go`) to declare a fresh `*runtime.ResourceType` for the test's resource kind. If a test exercises behavior that the new runtime API does not yet support in Session 0 (e.g., needs Session 2's Concrete promotion plumbing to be meaningful), `t.Skip("session 1 work: <reason>")` and reference the followup note rather than leaving a broken build.

    Depends on steps 7-7e (the runtime/ refactor must be done first; the new types must exist before external callers can be updated to use them). Must complete before step 13 (the wholesale deletes); otherwise external callers reference symbols that no longer exist and `go build ./api/... ./imports/... ./internal/...` fails. Verification: `go build ./api/... ./imports/... ./internal/...` returns clean.

13. **Delete wholesale:**
    - `internal/component/type_resolver.go` (entire file)
    - `internal/component/type_resolver_test.go` (entire file)
    - `internal/component/canon_lower.go` (entire file)
    - `internal/component/canon_lower_test.go` (entire file)
    - `internal/component/abi/resource_lower.go` (entire file — logic inlined into dispatch arms in step 12)
    - `internal/component/abi/resource_lower_test.go` (entire file — coverage moved into `lower_test.go`)
    - `internal/component/runtime/resource_type_id.go` (entire file — `ResourceTypeID` and `ResourceTypeInfo` deleted in step 7e; callers use `*runtime.ResourceType` directly)
    - `internal/component/runtime/resource_type_id_test.go` (entire file)
    - `internal/component/runtime/instance_state.go` (entire file — fields and methods merged into `runtime.ComponentInstance` in steps 7c/7d)
    - `internal/component/runtime/instance_state_test.go` (tests migrated to the new `component_instance_test.go`)
    - `internal/component/abi/lower.go` symbols `LowerOwn` (line 683) and `LowerBorrow` (line 699) — inlined into dispatch, not whole file
    - `internal/component/component_linker.go` lines 701-884: `resolveValTypeRef`, `resolveToValType`, `typeDefToValType`, `valTypeRefToValType`
    Depends on steps 7e, 9-12, **and 12a** (external callers must be migrated off the runtime symbols being deleted before this step removes them).
14. **internal/component/abi/*_test.go** — Update tests to build types via the builder, pass `*types.ComponentTypes` + `*runtime.ComponentInstance` through the context. Port Own/Borrow dispatch tests from the deleted `resource_lower_test.go`. Depends on steps 5, 7, 11-12.
15. **Compile-fix** broken call sites in `internal/component/component_linker.go` and `internal/component/instance.go`. Bodies may be logically wrong but must compile. Mechanical `switch typ.(type)` → `switch typ.Kind`. Every reference to `component.FuncType` or `component.NamedValType` updates to `*types.TypeFunc` / the tuple-based params/results pattern. Document deficiencies in followup note. Depends on step 13.
16. **internal/component/conformance/*_test.go** — Update the 12 test files to build types via `ComponentTypesBuilder` and pass `*types.ComponentTypes` + `*runtime.ComponentInstance` through `LiftContext`. Depends on steps 5, 7, 11-12.
17. **Write followup note** at `docs/plans/2026-04-07-canonical-abi-unification-session0-followup.md`. Depends on all prior steps so the list is accurate.
18. **Run full test suite.** Confirm: `types/`, `runtime/`, `binary/`, `abi/`, `conformance/` green; `api/component/`, `imports/wasip2/...`, `internal/component/wasip2test/` build-green and test-green except for documented `t.Skip("session 1 work: ...")` calls; `component/` top-level compile-green with documented test skips. Run `go build ./api/... ./imports/... ./internal/...` and `go vet ./api/... ./imports/... ./internal/...` as the build-completeness gate. Depends on step 17.

**Parallelism:** Steps 2, 3, 6, 7 run in parallel after step 1 (step 7 is independent of steps 1-6 entirely). Step 12 depends on step 11 but once step 11 is done, step 12 can run in parallel with steps 9-10. Step 12a (V5b external caller corrective sweep) depends on the runtime refactor (steps 7-7e) and must complete before step 13's wholesale deletes; it can run in parallel with the abi/ rewrite (steps 11-12).

## Testing Strategy

### Tests rewritten for the new shape

- `internal/component/types/types_test.go` — primitive tests, equality tests on the value-type `ValType`
- `internal/component/types/composite_test.go` — build composite types via the builder and assert on `ComponentTypes` slice contents
- `internal/component/types/resource_test.go` — `TypeResourceTable` construction (Concrete and Abstract variants), index-type round-trips. Pointer-equality tests are deferred to Session 2 along with any `ResourceType` struct decision.
- `internal/component/types/val_test.go` — `ValKind.String()` coverage for new constants
- `internal/component/binary/types_test.go` — decoder output assertions against `*ComponentTypes`
- `internal/component/binary/decoder_test.go` — same
- `internal/component/abi/lift_test.go`, `lower_test.go`, `flatten_test.go`, `canonical_test.go`, `canonical_options_test.go` — build types via builder, pass `*ComponentTypes` through context
- `internal/component/conformance/*_test.go` (12 files: `primitives_test.go`, `composites_test.go`, `strings_test.go`, `abi_edge_cases_test.go`, `type_edge_cases_test.go`, `concurrent_access_test.go`, `nesting_depth_test.go`, `realloc_failure_test.go`, `instance_types_test.go`, `error_messages_test.go`, `utf_validation_test.go`, `memory_bounds_test.go`) — same migration pattern

### Tests deleted wholesale

- `internal/component/canon_lower_test.go` — entire file
- `internal/component/type_resolver_test.go` — entire file
- `component_linker_test.go` — verified 2026-04-07 that this file has **zero direct references** to `resolveToValType`, `typeDefToValType`, `valTypeRefToValType`, `resolveValTypeRef`, or `TypeResolver`. The converters are exercised indirectly through the linker flow. No test deletions needed here; the tests that happen to exercise the deleted converters indirectly will need their expectations updated as part of the compile-fix step (15), and tests that exercise end-to-end lift/lower through the linker get the `t.Skip("session 1 work")` treatment like `instance_test.go`.

### New tests added in Session 0

- **`internal/component/types/builder_test.go`** (new file):
  - Interning dedup: two identical `tuple<u32, u32>` produce one entry
  - Interning differentiates: `list<u32>` (dynamic) vs `list<u32, 5>` (fixed) are distinct
  - Post-`Finish` intern calls panic
  - Concurrent reads of a finished `*ComponentTypes` are safe (race detector)
  - Exhaustive intern-key coverage for every kind
- **`internal/component/types/abi_info_test.go`** (new file):
  - `CanonicalABIInfo` precomputation matches spec values for every composite at multiple nesting levels
  - Scalar ABI table matches spec literals
- **`internal/component/binary/scope_test.go`** (new file):
  - A type defined in an inner instance type, exported, then aliased outside, resolves to the same `ValType`
  - Outer alias across two nesting levels
- **`internal/component/abi/types_context_test.go`** (new file):
  - `LiftContext.Types` threading: a lift operation reads composite content from the context's type table correctly
  - `LowerContext.Types` same

### Tests intentionally left broken at end of Session 0 (captured in followup note)

- `instance_test.go` lift/lower tests — bodies being deleted in Session 1; skip with `t.Skip("session 1 work: canonical-abi-unification-session0-followup.md")`
- Any test that constructs `Component` with populated lift/lower end-to-end — same skip reason
- `component_linker_test.go` tests that exercise the full instantiation path through the broken lift/lower

## Build State at End of Session 0

| Package | Build | Tests |
|---|---|---|
| `internal/component/types/` | green | green |
| `internal/component/binary/` | green | green |
| `internal/component/abi/` | green | green |
| `internal/component/conformance/` | green | green |
| `internal/component/` (top-level) | **compile-green** | **test-red**, documented skips |
| Repo-wide `go build ./...` | **green** | — |
| Repo-wide `go test ./...` | — | documented skips, no compile errors |

## Followup Note Format

The followup note lives at `docs/plans/2026-04-07-canonical-abi-unification-session0-followup.md` and is written as step 15 of the work order. Format:

```markdown
# Canonical ABI Unification — Session 0 Followup Note

This note captures intentionally-broken or intentionally-deferred work
from Session 0. Each item lists the exact file:line and the scope of
the follow-up work.

## Session 1 — Wire abi/ into production

### Broken in-place at end of Session 0 (compile-only, logically wrong):

- `internal/component/instance.go:<line range>` — lift/lower method
  bodies. Currently: <list each function>. Session 1 action: DELETE
  these and replace call sites with direct calls into
  `abi.LiftFlat`/`LiftHeap`/`LowerFlat`/`LowerHeap`.

- `internal/component/component_linker.go:<line range>` — flatten
  helpers. Currently: <list>. Session 1 action: delete and replace with
  `abi.FlattenParams`/`abi.FlattenResults`/`abi.CoreSignature`.

### Skipped tests (`t.Skip("session 1 work: ...")`):

- `internal/component/instance_test.go` — <list by test name>
- ...

### Session 1 acceptance criteria:

- Zero references to `instance.go`'s old lift/lower bodies.
- `canon_lower.go` stays deleted.
- All tests currently marked `t.Skip("session 1 work")` pass without
  the skip.

## Session 2 — Resource Concrete promotion + cross-component type checking

### Deferred from Session 0:

- Linker plumbing that promotes `TypeResourceTable.Concrete = false`
  entries to `true` at instantiation time. For each resource declaration
  in the component, the linker mints a fresh `*runtime.ResourceType`
  (pointer identity) and stores it in
  `runtime.ComponentInstance.ResourceTypes[ResourceIdx]`. Location:
  `internal/component/component_linker.go` instantiation path.

- `typeChecker` struct and `equivalent` walk for cross-component import
  type-matching. New file: `internal/component/types/typecheck.go`.

- Cross-instance resource type resolution. When lift/lower encounters
  an own/borrow of a resource defined by a *different* instance, the
  runtime must walk from the current `ComponentInstance` to find the
  defining instance and fetch its `*ResourceType`. For nested
  instantiation this goes through the `Parent` pointer; for sibling
  instances a top-level store/linker-level lookup table is needed.
  Session 0's `LookupResourceType` method handles the single-instance
  case only.

### Session 2 acceptance criteria:

- Instantiating two copies of the same component produces two distinct
  `*runtime.ResourceType` pointers. A handle minted by one instance
  traps if presented to the other instance's function expecting the
  same-typed resource (pointer inequality → `ErrResourceTypeMismatch`).
- Cross-component type-import matching works for at least one realistic
  WIT world.

## Later — Async lift/lower (no session scheduled)

### Stub-and-trap sites that need real implementations:

- `internal/component/abi/lift.go` — `case TypeKindStream,
  TypeKindFuture, TypeKindErrorContext` trap arms.
- `internal/component/abi/lower.go` — same.
- `internal/component/types/composite.go` — `TypeStream`, `TypeFuture`,
  `TypeErrorContextTable` need per-instance table identity layering
  analogous to `TypeResourceTable`.
- `internal/component/types/val.go` — constructors for stream, future,
  error-context values.
```

## Consolidated Change Manifest

### Deleted whole files

- `internal/component/type_resolver.go`
- `internal/component/type_resolver_test.go`
- `internal/component/canon_lower.go`
- `internal/component/canon_lower_test.go`
- `internal/component/abi/resource_lower.go` (logic inlined into Own/Borrow dispatch arms)
- `internal/component/abi/resource_lower_test.go` (coverage moves into `lower_test.go`)

### Deleted symbols (files remain)

**`internal/component/types/types.go`:**
- `ValType` interface (replaced by value-type struct)
- `Bool`, `S8`, `U8`, `S16`, `U16`, `S32`, `U32`, `S64`, `U64`, `F32`, `F64`, `Char`, `String` scalar structs and their `valType()`/`Size()`/`Align()`/`FlattenCount()` methods

**`internal/component/types/composite.go`:**
- `Record`, `Variant`, `List`, `Tuple`, `Flags`, `Enum`, `Option`, `Result`, `Stream`, `Future`, `ErrorContext` struct types and all their methods (replaced by table-entry struct types)
- Spec-comment blocks that reference the old interface design

**`internal/component/types/resource.go`:**
- `Own` struct and its methods (the inline-handle representation)
- `Borrow` struct and its methods (the inline-handle representation)
- The existing limited `ResourceType` struct at `resource.go:61-75` (with `InstanceID uint32` field). The new nominal-identity `ResourceType` lives in `runtime/` (see runtime deletions and additions below).
- `TODO: ResourceType *ResourceType` stale comments at `resource.go:14-15, 31-32`
- `HasDestructor` method — moves to the new `runtime.ResourceType`

**`internal/component/runtime/` — deletions:**
- `runtime/resource_type_id.go` (entire file) — `ResourceTypeID`, `NewResourceTypeID`, `InvalidResourceTypeID`, `ResourceTypeInfo`, `NewResourceTypeInfo`. Replaced by `runtime.ResourceType` pointer identity.
- `runtime/resource_type_id_test.go` (entire file)
- `runtime/instance_state.go` (entire file) — `InstanceState` struct, constructor, methods (`ID`, `MayLeave`, `SetMayLeave`, `Enter`, `Leave`, `EnterCount`). Merged into `runtime.ComponentInstance`.
- `runtime/instance_state_test.go` — tests moved into `component_instance_test.go`

**`internal/component/component.go`:**
- `Component.TypeIdxToStoredIdx` field (line 45)
- `TypeDef.SourceLocalTypes` field (lines 266-270)
- `TypeDef.Handle *ValTypeRef` field (lines 262-264)
- `TypeDef.Record`, `.Option`, `.List`, `.Result`, `.Variant`, `.Tuple`, `.Flags`, `.Enum`, `.Stream`, `.Future`, `.FixedSizeList` fields (collapsed to single `ValType types.ValType`)
- `TypeDef.Resource interface{}` field (replaced by `Resource types.ResourceTableIdx`)
- `FuncType` struct (lines ~344-347) — Decision 7, replaced by `types.TypeFunc`
- `NamedValType` struct (lines ~350-355) — Decision 7, absorbed into `types.TypeFunc.ParamNames` + params/results tuple pattern; has no replacement as a standalone type
- `ValTypeRef` struct (lines 357-377)
- `RecordTypeDef`, `VariantTypeDef`, `ListTypeDef`, `TupleTypeDef`, `FlagsTypeDef`, `EnumTypeDef`, `OptionTypeDef`, `ResultTypeDef` structs (declared elsewhere in the file)
- `StreamTypeDef` (line 770)
- `FutureTypeDef` (line 776)
- `FixedSizeListTypeDef` (line 781)

**`internal/component/component_linker.go`:**
- `resolveValTypeRef` function (line 701)
- `resolveToValType` function (line 723)
- `typeDefToValType` function (line 749)
- `valTypeRefToValType` function (line 834)

**`internal/component/binary/types.go`:**
- `RecordTypeDef`, `RecordField` (file-local), `VariantTypeDef`, `VariantCase`, `ListTypeDef`, `TupleTypeDef`, `FlagsTypeDef`, `EnumTypeDef`, `OptionTypeDef`, `ResultTypeDef`, `ResourceTypeDef` structs
- `decodeRecordTypeDef`, `decodeVariantTypeDef`, `decodeListTypeDef`, `decodeTupleTypeDef`, `decodeFlagsTypeDef`, `decodeEnumTypeDef`, `decodeOptionTypeDef`, `decodeResultTypeDef` functions (replaced by inline interning at call sites)
- `decodeStreamTypeDef` (lines 666-699), `decodeFutureTypeDef` (lines 701-721), `decodeFixedSizeListTypeDef` (lines 723-745)
- `decodeResourceTypeDef`, `decodeResourceTypeDefWithAsync` (replaced by code that interns into `ComponentTypes.ResourceTables` as Abstract)
- `TypeDef.Record`, `.Variant`, `.List`, `.Tuple`, `.Flags`, `.Enum`, `.Option`, `.Result`, `.Resource` fields on `binary.TypeDef`

**`internal/component/abi/lower.go`:**
- `LowerOwn` standalone function (lines 683-690) — logic inlined into `LowerFlat` case `TypeKindOwn`
- `LowerBorrow` standalone function (lines 699-712) — logic inlined into `LowerFlat` case `TypeKindBorrow`

**`internal/component/abi/context.go`:**
- `LiftContext.ResourceTable *runtime.ResourceTable` field (line 69) — deleted; resource access now goes through `ctx.Instance.Table` (the unified handle table on the calling instance)
- `LowerContext.ResourceTable *runtime.ResourceTable` field (line 158) — deleted; same reason
- `LowerContext.Instance interface{}` field (line 163) — replaced by typed `*runtime.ComponentInstance`
- `LowerContext.Subtask *runtime.Subtask` field (line 166) — deleted; async is deferred, no sync call needs subtask state
- `LowerContext.BorrowScope() *runtime.BorrowScope` helper method (lines 170-175) — deleted; it derived from Subtask which is gone. Direct-field access is used instead where needed.
- `LiftContext.BorrowScope *runtime.BorrowScope` field (line 70) — **kept**, per-call borrow scope
- `LowerContext.CallContext *runtime.CallContext` field (line 159) — **kept**, per-call context

### Created — new files

- `internal/component/types/abi_info.go` — `CanonicalABIInfo`, `DiscriminantInfo`, `scalarABI` table, `(ValType).ABI` accessor
- `internal/component/types/builder.go` — `ComponentTypesBuilder` with intern methods and `Finish`
- `internal/component/runtime/resource_type.go` — `runtime.ResourceType` pointer-identity struct (Decision 5). Fields: `Impl *ComponentInstance`, `Dtor *uint32`, `DtorAsync bool`, `DtorCallback *uint32`. Method: `HasDestructor()`.
- `internal/component/runtime/component_instance.go` — single-layer `runtime.ComponentInstance` struct (Decision 6) matching the spec's ComponentInstance at `definitions.py:256-273`. Fields: `ID`, `Parent *ComponentInstance`, `Table *Table`, `MayLeave bool`, `enterCount int`, `ResourceTypes []*ResourceType`, `Destructors *DestructorRegistry`, `Reentrance *ReentranceTracker`. Methods: `NewComponentInstance(id, parent)`, `Enter`, `Leave`, `EnterCount`, `IsMayLeave`, `LookupResourceType`.
- `internal/component/types/builder_test.go` — interning, freeze enforcement, race safety
- `internal/component/types/abi_info_test.go` — ABI precomputation correctness
- `internal/component/runtime/resource_type_test.go` — pointer-identity equality, `HasDestructor`
- `internal/component/runtime/component_instance_test.go` — construction, Enter/Leave, IsMayLeave, LookupResourceType tree walking (replaces the deleted `instance_state_test.go`)
- `internal/component/binary/scope_test.go` — scope-local index / alias resolution
- `internal/component/abi/types_context_test.go` — context type-table and instance threading
- `docs/plans/2026-04-07-canonical-abi-unification-session0-followup.md` — the followup note

### Renamed files

- `internal/component/runtime/resource_table.go` → `internal/component/runtime/table.go` — type renamed `ResourceTable` → `Table`, entry type reshaped to `TableEntry` interface with `ResourceHandleEntry` implementation; `ValidateType` reshaped to take `*ResourceType` with pointer comparison. Heavy rewrite of the 659-line file.
- `internal/component/runtime/resource_table_test.go` → `internal/component/runtime/table_test.go` — 1240-line mechanical migration to the new API.

### Created — new symbols in existing files

**`internal/component/types/types.go`:**
- `TypeKind` enum (all constants)
- `ValType` struct
- `ComponentTypes` struct
- Named scalar ValType constants (`Bool`, `S8`, …, `String_`)
- `(ValType).IsZero()` method

**`internal/component/types/composite.go`:**
- `TypeRecord`, `RecordField`, `TypeVariant`, `VariantCase`
- `TypeList` (dynamic lists only)
- `TypeFixedLengthList` (distinct struct and type kind for fixed-length lists — wasmtime parity)
- `TypeTuple`, `TypeFlags`, `TypeEnum`, `TypeOption`, `TypeResult`
- `TypeStream`, `TypeFuture`, `TypeErrorContextTable`
- `TypeFunc`

**`internal/component/types/resource.go`:**
- `ResourceIdx`, `RuntimeComponentInstanceIdx`, `ResourceTableIdx` named types
- `TypeResourceTable` struct (bare uint32-typed fields; no references to other packages)
- The nominal-identity `ResourceType` struct lives in `runtime/resource_type.go` (see "Created — new files" above), not in `types/`, because it references `*runtime.ComponentInstance` which would create an import cycle if placed in `types/`.

**`internal/component/types/val.go`:**
- `ValKindStream`, `ValKindFuture`, `ValKindErrorContext` constants
- `ValKind.String()` cases for the three new constants

**`internal/component/component.go`:**
- New collapsed `TypeDef` struct shape: `{Kind TypeDefKind; Func *types.TypeFunc; Resource types.ResourceTableIdx; ValType types.ValType; Instance *InstanceTypeDef; Component *ComponentTypeDef}` — `FuncType` now lives in `types/`, `NamedValType` is gone entirely. (No `ComponentInstance.ResourceTypes` field — that was based on my earlier incorrect model; the per-instance resource tables live on `runtime.ComponentInstance`, not on the parse-time `ParsedComponentInstance` at `component.go:705`.)

**`internal/component/runtime/component_instance.go`** (new file):
- `ComponentInstance` single-layer struct matching spec `definitions.py:256-273`
- Fields: `ID uint32`, `Parent *ComponentInstance`, `Table *Table`, `MayLeave bool`, `enterCount int`, `ResourceTypes []*ResourceType`, `Destructors *DestructorRegistry`, `Reentrance *ReentranceTracker`
- Constructor: `NewComponentInstance(id uint32, parent *ComponentInstance) *ComponentInstance`
- Methods: `Enter()`, `Leave()`, `EnterCount() int`, `IsMayLeave() bool`, `LookupResourceType(instanceIdx types.RuntimeComponentInstanceIdx, resourceIdx types.ResourceIdx) *ResourceType`, internal `findInstance(id uint32) *ComponentInstance`

**`internal/component/runtime/resource_type.go`** (new file):
- `ResourceType` struct with pointer-identity semantics
- Fields: `Impl *ComponentInstance`, `Dtor *uint32`, `DtorAsync bool`, `DtorCallback *uint32`
- Method: `HasDestructor() bool`

**`internal/component/abi/context.go`:**
- `LiftContext.Types *types.ComponentTypes` field (new)
- `LiftContext.Instance *runtime.ComponentInstance` field (new — the calling instance; replaces the deleted `ResourceTable` field)
- `LiftContext.BorrowScope *runtime.BorrowScope` field (kept from current shape as per-call state)
- `LowerContext.Types *types.ComponentTypes` field (new)
- `LowerContext.Instance *runtime.ComponentInstance` field (new — the calling instance; replaces the deleted `interface{}` TODO, `ResourceTable`, and `Subtask` fields)
- `LowerContext.CallContext *runtime.CallContext` field (kept from current shape as per-call state)

**`internal/component/abi/lift.go`:**
- `TypeKindOwn` dispatch arm in `LiftFlat` and `LiftHeap` (new — closes the current gap)
- `TypeKindBorrow` dispatch arm in `LiftFlat` and `LiftHeap` (new — closes the current gap)
- Trap arms for `TypeKindStream`, `TypeKindFuture`, `TypeKindErrorContext` in `LiftFlat` and `LiftHeap`

**`internal/component/abi/lower.go`:**
- `TypeKindOwn` dispatch arm in `LowerFlat` and `LowerHeap` (new — folds in the deleted `LowerOwn` + `LowerOwnWithType` logic)
- `TypeKindBorrow` dispatch arm in `LowerFlat` and `LowerHeap` (new — folds in the deleted `LowerBorrow` + `LowerBorrowWithType` logic, including the `CanonicalABI.md:2677-2683` same-instance optimization)
- Trap arms for `TypeKindStream`, `TypeKindFuture`, `TypeKindErrorContext` in `LowerFlat` and `LowerHeap`

**`internal/component/binary/types.go`:**
- `typeScope` struct or equivalent scope-tracking machinery
- New top-level `decodeTypeSection` function (replaces whatever the existing section-walk is)

### Modified — behavioral changes

- `internal/component/component.go:14` (`Component` struct): `Types` field type changes from `[]TypeDef` to `*types.ComponentTypes`
- `internal/component/component.go:213` (`TypeDef` struct): collapses as described in Work Order step 9
- `internal/component/binary/types.go:138` (`decodeValType`): return type changes from `component.ValTypeRef` to `types.ValType`, signature gains `*typeScope` and `*types.ComponentTypesBuilder` parameters
- `internal/component/binary/types.go:302` (`decodeDefinedType`): return type changes from `*TypeDef` to `types.ValType`
- `internal/component/abi/context.go:66-71` (`LiftContext`): struct body replaced per Decision 6 — drop `ResourceTable`; keep `BorrowScope` (per-call); add `Types *types.ComponentTypes` and `Instance *runtime.ComponentInstance`
- `internal/component/abi/context.go:154-175` (`LowerContext` + `BorrowScope()` helper): struct body replaced — drop `ResourceTable`, `Subtask`, and the `BorrowScope()` helper method; keep `CallContext` (per-call); replace `interface{} Instance` with typed `*runtime.ComponentInstance`; add `Types *types.ComponentTypes`
- `internal/component/abi/lift.go:52` (`LiftFlat`): body rewritten to dispatch on `typ.Kind`; **new** `TypeKindOwn` and `TypeKindBorrow` arms (closes current gap); async trap arms
- `internal/component/abi/lift.go:354` (`LiftHeap`): same
- `internal/component/abi/lower.go:13` (`LowerFlat`): same; the Own/Borrow arms absorb the logic of the deleted standalone `LowerOwn`/`LowerBorrow` helpers and the deleted `resource_lower.go` `LowerOwnWithType`/`LowerBorrowWithType` functions
- `internal/component/abi/lower.go:322` (`LowerHeap`): same
- `internal/component/abi/flatten.go:10-60` (`FlattenParams`, `FlattenResults`, `CoreSignature`, `flattenType`): gain `*types.ComponentTypes` parameter and kind-switch dispatch. `flattenType` already handles `Own`/`Borrow` at line 82 — that logic moves to `case types.TypeKindOwn, types.TypeKindBorrow:` in the new dispatch form
- `internal/component/component_linker.go:3580-3617` (`coreSignature`, `flattenParams`, `flattenResults`, `flattenValType`): compile-fix to new shape; these are already marked for deletion in Session 1
- `internal/component/instance.go` lift/lower sites at lines 794, 1205, 1242, 1518, 1527, 1546, 1601, 2009 (`liftResolvedType`, `liftResolvedPrimitiveVal`, `liftFieldFromMemory`, `lowerParam`, `lowerTyped`, `lowerToMemory`, `typeCanCoerce`, `typeMatchesKind`): compile-fix only, bodies slated for Session 1 deletion
- `internal/component/types/val.go`: `ValKind.String()` at lines 301-352 extended for new constants
- All call sites in `internal/component/`, `internal/component/binary/`, `internal/component/abi/`, and their tests that previously used `*component.FuncType` or `component.NamedValType` — mechanical rename to `*types.TypeFunc` (Decision 7). Planning agent must `grep -rn "component\.FuncType\|component\.NamedValType" internal/` and update each hit.
- `api/component/component.go:114` (`type ResourceTable = runtime.ResourceTable`) — RHS updated to `runtime.Table`. Public alias name `ResourceTable` may be preserved or renamed; the underlying type is the renamed `runtime.Table`. Pre-production, no backwards-compat constraint.
- `api/component/component.go:117` (`var NewResourceTable = runtime.NewResourceTable`) — RHS updated to `runtime.NewTable`.
- `api/component/component.go:122-124` (`WithResourceTable(ctx, table *ResourceTable) context.Context`) — signature/body updated to use the renamed type.
- `api/component/component.go` — any other re-export or accessor that touches a runtime symbol caught by the V5b audit must be updated in the same pass.
- `imports/wasip2/io/streams.go:318, 369` (`table.New(rep, true)` calls inside `lastOperationFailedError` and `createPollableHandle`) — replaced by `table.NewResourceHandle(rep, true, errorResourceType)` and `table.NewResourceHandle(rep, true, pollableResourceType)` respectively. Requires declaring per-kind host resource type singletons at the top of the package per the "Host-managed resource types" section above.
- `imports/wasip2/io/streams.go:338, 355` (`entry.Rep.(*InputStream)` and `entry.Rep.(*OutputStream)`) — replaced by `entry.(*runtime.ResourceHandleEntry).Rep.(*InputStream)` and `entry.(*runtime.ResourceHandleEntry).Rep.(*OutputStream)`. Type-assertion required because `Table.Get` now returns the generic `runtime.TableEntry` interface.
- `imports/wasip2/io/error.go:148, 151` and `imports/wasip2/io/poll.go:147` — same `entry.Rep` → `entry.(*runtime.ResourceHandleEntry).Rep` migration.
- `imports/wasip2/io/streams.go` (top of file): declare package-level `inputStreamResourceType`, `outputStreamResourceType`, `pollableResourceType`, `errorResourceType` as `*runtime.ResourceType{}` singletons per the "Host-managed resource types" pattern. The `Destroy()` methods on `*InputStream` (line 137), `*OutputStream` (line 268), and `*Error` (`error.go:117`) are unchanged — host destructors flow through the existing `Destroyable` interface.
- `imports/wasip2/io/{streams,error,poll}.go` plus any other `imports/wasip2/{filesystem,sockets,http,cli,clocks,...}/*.go` flagged by the audit command — same pattern applied per host module. Each host module declares its own per-kind `*runtime.ResourceType` singletons.
- `internal/component/wasip2test/kv_store_test.go` — `runtime.NewResourceTypeID()` call replaced by a fresh `*runtime.ResourceType` declared in the test fixture. If the test exercises behavior that the new runtime API genuinely no longer supports in Session 0, the test may instead `t.Skip("session 1 work: ...")` and reference the followup note.

## Implementation Verification Checklist

These are checks the implementation must perform or the tests must assert. Not open questions — concrete verifications against the spec and the committed decisions above.

### V1 — ABI values match spec authorities and wasmtime

For every kind, the precomputed `CanonicalABIInfo` values match both the canonical-ABI spec (`debug-vendored/component-model/design/mvp/canonical-abi/definitions.py`) and wasmtime's reference implementation (`debug-vendored/wasmtime/crates/environ/src/component/types.rs`). Verified 2026-04-07:

**Scalars** (hardcoded in `scalarABI` table, see `types/abi_info.go`):

| Type | Size32 | Align32 | Size64 | Align64 | Flatten | Spec cite | Wasmtime cite |
|---|---|---|---|---|---|---|---|
| bool | 1 | 1 | 1 | 1 | 1 | definitions.py:1065, 1123, 1705 | types.rs:667 `SCALAR1` |
| s8/u8 | 1 | 1 | 1 | 1 | 1 | :1066, 1124, 1706-7 | `SCALAR1` |
| s16/u16 | 2 | 2 | 2 | 2 | 1 | :1067, 1125, 1706-7 | `SCALAR2` |
| s32/u32 | 4 | 4 | 4 | 4 | 1 | :1068, 1126, 1706-7 | `SCALAR4` |
| s64/u64 | 8 | 8 | 8 | 8 | 1 | :1069, 1127, 1708 | `SCALAR8` |
| f32 | 4 | 4 | 4 | 4 | 1 | :1070, 1128, 1709 | `SCALAR4` |
| f64 | 8 | 8 | 8 | 8 | 1 | :1071, 1129, 1710 | `SCALAR8` |
| char | 4 | 4 | 4 | 4 | 1 | :1072, 1130, 1711 | `SCALAR4` |
| string | 8 | 4 | 16 | 8 | 2 | :1073, 1131, 1712 (memory32) | types.rs:678-684 `POINTER_PAIR` (memory64 values) |
| own / borrow | 4 | 4 | 4 | 4 | 1 | :1079, 1137, 1718 | `SCALAR4` |
| stream / future | 4 | 4 | 4 | 4 | 1 | :1080, 1138, 1719 | `SCALAR4` |
| error-context | 4 | 4 | 4 | 4 | 1 | :1074, 1132, 1713 | `SCALAR4` |

**Record / Tuple** — precomputed at intern time:
- `size32` = aligned sum of field `size32` values, finally aligned to `align32` of the record. Spec: `elem_size_record` at `:1145-1151`. Wasmtime: `record_static` at `types.rs:705-723` (with `next_field32` helper at `:727-738`).
- `align32` = max of field `align32` values, default 1 if no fields. Spec: `alignment_record` at `:1087-1091`.
- `size64`/`align64`: analogous using field-64 values.
- `flatten` = sum of field flatten counts. Spec: `flatten_record` at `:1726-1730`.
- Tuples are positional records; same formulas.

**Variant** — precomputed at intern time:
- Discriminant type determined by case count: `discriminant_type` at `definitions.py:1096-1103`. n ≤ 256 → U8 (size/align 1); n ≤ 65536 → U16 (2/2); otherwise U32 (4/4).
- `size32` = `align_to(align_to(discSize32, maxCaseAlign32) + maxCaseSize32, variantAlign32)`. Spec: `elem_size_variant` at `:1156-1164`. Wasmtime: `variant` at `types.rs:772-815`.
- `align32` = max(alignment of discriminant type, max of case alignments). Spec: `alignment_variant` at `:1093-1094`, `max_case_alignment` at `:1105-1110`.
- `flatten` = 1 (discriminant) + max-joined case payload flatten. Spec: `flatten_variant` at `:1732-1741`, `join` at `:1743-1746`.
- **Option**: syntactic sugar for `variant {none, some(T)}`. Same formulas.
- **Result**: syntactic sugar for `variant {ok(T), error(E)}`. Same formulas.
- **Enum**: discriminant only, no payloads. `size`/`align` equal to the discriminant type's `size`/`align`, `flatten = 1`.

**List (dynamic)**: pointer-pair. Spec: `:1075, 1133, 1714` → `['i32', 'i32']`. Wasmtime: `POINTER_PAIR` at `types.rs:678-684`.
- size32=8, align32=4, size64=16, align64=8, flatten=2.

**List (fixed-length)**: inline elements. Spec: `alignment_list` at `:1082-1085`, `elem_size_list` at `:1140-1143`, `flatten_list` at `:1721-1723`.
- size = `length * elem.Size`, align = `elem.Align`, flatten = `length * elem.Flatten`. Both memory modes.

**Flags**: size/align depends on label count. Spec: `alignment_flags` at `:1112-1117`, `elem_size_flags` at `:1166-1171`.
- n ≤ 8: size 1, align 1, flatten 1.
- n ≤ 16: size 2, align 2, flatten 1.
- n ≤ 32: size 4, align 4, flatten 1.
- n > 32: size `4 * ceil(n/32)`, align 4, flatten `ceil(n/32)`. This is a **divergence from the literal spec**, which asserts `0 < n <= 32` at `definitions.py:1114`. Wasmtime supports the >32 case via `FlagsSize::Size4Plus(n)` at `types.rs:756-770`; my design matches wasmtime.

**Three divergences from the literal canonical-ABI spec that wasmtime and this design both make:**

1. **Empty records are permitted with size 0.** `definitions.py:1150` asserts `s > 0`, which would reject empty records at the spec level. Wasmtime's `record_static` at `types.rs:705-723` does not execute the field loop for empty fields and returns `CanonicalAbiInfo::ZERO` (size 0, align 1, flatten 0). wazero's current `types/composite.go:22-39` also returns size 0. This design preserves the wasmtime/wazero permissive behavior. The conformance test `TestCompositeRecordEmpty` at `conformance/composites_test.go:18` asserts size 0 and is preserved.

2. **Flags with more than 32 labels are permitted.** `definitions.py:1114` asserts `0 < n <= 32`. Wasmtime's `CanonicalAbiInfo::flags` at `types.rs:756-770` has `FlagsSize::Size4Plus(n)` covering `n > 16` (and by extension >32) via multi-i32 encoding. This design matches wasmtime's support for arbitrary flag counts.

3. **Memory64 sizes and alignments.** `definitions.py` has only memory32 formulas (no memory-size parameter on `alignment()` or `elem_size()`). Wasmtime's `CanonicalAbiInfo` carries both `size32/align32` and `size64/align64` variants — identical for scalars, but `POINTER_PAIR` doubles (8/4 → 16/8) for memory64 strings and dynamic lists. This design matches wasmtime by carrying both pairs on `CanonicalABIInfo`.

Tests in `types/abi_info_test.go` validate every formula against hand-computed spec-derived values for scalar kinds and against representative composite cases (nested records containing variants containing tuples). Memory32 and memory64 both. Each test has a comment citing the spec line and/or wasmtime line that anchors its expected value.

### V2 — Intern dedup and distinctness

For each `Intern<Kind>` method, a test constructs multiple structurally-identical inputs and verifies they collapse to one entry (returning the same index). A companion test constructs structurally-distinct-but-nearly-identical inputs (e.g., `list<u32>` vs `list<u32, 5>`, `record {a: u32, b: u32}` vs `record {b: u32, a: u32}` — same fields different order, `result<u32, _>` vs `result<_, u32>`) and verifies they do NOT collapse. See "Intern keys per kind" section above for the exhaustive rules.

### V3 — Decoder produces exactly one canonical shape

After the binary decoder runs, grep across `internal/component/` must find:
- Zero references to `component.ValTypeRef`
- Zero references to `component.RecordTypeDef`, `component.VariantTypeDef`, `component.ListTypeDef`, `component.TupleTypeDef`, `component.FlagsTypeDef`, `component.EnumTypeDef`, `component.OptionTypeDef`, `component.ResultTypeDef`, `component.StreamTypeDef`, `component.FutureTypeDef`, `component.FixedSizeListTypeDef`
- Zero references to `component.FuncType` or `component.NamedValType`
- Zero references to `binary.TypeDef.Record`/`.Variant`/`.List`/`.Tuple`/`.Flags`/`.Enum`/`.Option`/`.Result`/`.Resource` fields (the `binary.TypeDef` struct still exists for `Func` and related wrappers, but its composite content fields are gone)
- Zero references to `component.TypeIdxToStoredIdx`
- Zero references to `resolveToValType`, `typeDefToValType`, `valTypeRefToValType`, `resolveValTypeRef`
- Zero references to `*TypeResolver` (file deleted)

The final grep for `func .*ValType\b` in `internal/component/` returns only methods on the new `types.ValType` value struct plus the builder interning functions.

### V4 — Conformance test migration pattern

The 12 files in `internal/component/conformance/` follow a uniform construction pattern: struct literal → abi call → roundtrip assert. The migration replaces:

- **Scalar literals**: `types.S32{}` → `types.S32` (the named scalar constant)
- **Composite literals**: `types.Record{Fields: [...]}` → `b := types.NewComponentTypesBuilder(); recT := b.InternRecord([]types.RecordField{...})`, then `ct := b.Finish()` used as `ctx.Types`
- **ABI property access**: `emptyRecord.Size()` → `recT.ABI(ct).Size32` (or an equivalent helper)
- **Nil context for primitives**: `abi.LowerFlat(nil, ...)` stays valid for scalar-only tests because the scalar dispatch arms do not read from `ctx.Types`. Composite tests must construct a `*LiftContext`/`*LowerContext` with `Types: ct` and a nil `Instance` (instance is only needed for resource arms, which are not exercised by primitive/composite tests).

A shared test helper file `internal/component/conformance/helpers_test.go` (new) carries the builder/context construction so each test file is a minimal 3-4 line invocation.

### V5 — `FuncType`/`NamedValType` caller audit (complete list)

Grep for `component\.FuncType|component\.NamedValType|\bFuncType\b|\bNamedValType\b` across `internal/component/` returned 21 files on 2026-04-07. Of those, 3 are files that will be deleted wholesale (`canon_lower.go`, `canon_lower_test.go`, `type_resolver_test.go`), leaving **18 files** that need mechanical rename to `*types.TypeFunc`:

- `internal/component/component.go` (the struct definitions — delete FuncType + NamedValType)
- `internal/component/component_linker.go`, `component_linker_test.go`
- `internal/component/linker.go`, `linker_test.go`, `linker_api_test.go`
- `internal/component/instance.go`, `instance_test.go`
- `internal/component/instantiate.go`
- `internal/component/type_checker.go`, `type_checker_test.go`
- `internal/component/binary/types.go` (producer side)
- `internal/component/nested_component_test.go`
- `internal/component/outer_alias_test.go`
- `internal/component/integration_test.go`
- `internal/component/edge_case_test.go`
- `internal/component/conformance/linker_test.go`
- `internal/component/conformance/nested_test.go`

Each file's updates are mechanical: a `NamedValType{Name: "x", ValType: someValTypeRef}` becomes a contribution to the `ParamNames + InternTuple` pattern. `*FuncType` in method signatures becomes `*types.TypeFunc`.

### V5b — Repo-wide runtime-symbol caller audit (deleted/renamed runtime symbols)

V5 was originally scoped only to FuncType/NamedValType. The runtime-side renames (Decision 5, Decision 6) also have callers outside `internal/component/` that must be migrated. **Audit command (run before any code changes in the runtime refactor):**

```
grep -rn "runtime\.\(ResourceTable\|NewResourceTable\|HandleEntry\|ResourceTypeID\|NewResourceTypeID\|InvalidResourceTypeID\|ResourceTypeInfo\|NewResourceTypeInfo\)\|table\.New(" api/ imports/ internal/
```

Symbols that the audit catches and the corresponding fix per call site:

| Old symbol | New form | Fix pattern |
|---|---|---|
| `runtime.ResourceTable` (type) | `runtime.Table` | mechanical type rename |
| `*runtime.ResourceTable` | `*runtime.Table` | mechanical type rename |
| `runtime.NewResourceTable` | `runtime.NewTable` | mechanical func rename |
| `runtime.HandleEntry` | `runtime.ResourceHandleEntry` (via `runtime.TableEntry` interface) | type assertion required at retrieval |
| `entry.Rep` (where `entry` is from `Table.Get`) | `entry.(*runtime.ResourceHandleEntry).Rep` | type assertion to the concrete entry kind |
| `entry.RT` (handle entry's resource type tag) | `entry.(*runtime.ResourceHandleEntry).RT` (now `*ResourceType`) | type assertion + pointer-identity comparison |
| `runtime.ResourceTypeID` | `*runtime.ResourceType` | pointer-identity replaces uint32 alias |
| `runtime.NewResourceTypeID(idx)` | declare or look up a `*runtime.ResourceType` and use it directly | no-op constructor; pointer comes from a singleton or instance pool |
| `runtime.InvalidResourceTypeID()` | `nil` (`*ResourceType`) or omit the field | no separate "invalid" sentinel; `nil` pointer is the unset case |
| `runtime.ResourceTypeInfo` (composite struct) | `*runtime.ResourceType` | pointer replaces composite |
| `runtime.NewResourceTypeInfo(idx, instID)` | declare a `*runtime.ResourceType` per kind | no composite constructor needed |
| `(*Table).New(rep, own)` (the no-type-info constructor) | `(*Table).NewResourceHandle(rep, own, *ResourceType)` | a `*ResourceType` must be passed; see "Host-managed resource types" below |
| `(*Table).NewWithType(rep, own, rtID)` | `(*Table).NewResourceHandle(rep, own, *ResourceType)` | same |
| `(*Table).GetType(h)` returning `ResourceTypeID` | `(*Table).GetResourceType(h)` returning `*ResourceType, error` | type assertion happens inside the helper |
| `(*Table).ValidateType(h, rtID)` | `(*Table).ValidateType(h, *ResourceType)` | pointer comparison body; spec's `is` check |
| Public re-exports in `api/component/component.go` (e.g. `type ResourceTable = runtime.ResourceTable`, `var NewResourceTable = runtime.NewResourceTable`, `WithResourceTable`, `ResourceTableFromContext`) | Update the right-hand side to the renamed types/funcs. The public alias name `ResourceTable` may be preserved on the api side (it is just an alias and external Go consumers reference it by the api/component name); the underlying type is `runtime.Table`. | mechanical RHS rename plus signature alignment |

**Files known affected at design time (2026-04-08 audit):**

- `api/component/component.go` — `type ResourceTable = runtime.ResourceTable`, `var NewResourceTable = runtime.NewResourceTable`, `WithResourceTable`, `ResourceTableFromContext` accessors
- `imports/wasip2/io/streams.go` — `table.New(streamPtr, true)` calls at lines 318, 369; `entry.Rep` accesses at lines 338, 355
- `imports/wasip2/io/error.go` — `entry.Rep` accesses at lines 148, 151
- `imports/wasip2/io/poll.go` — `entry.Rep` access at line 147
- `internal/component/wasip2test/kv_store_test.go` — `runtime.NewResourceTypeID()` call

**Other `imports/wasip2/` modules** (filesystem, sockets, http, cli, clocks) and the broader imports tree may have additional callers; the audit command above must be run to produce a complete list at implementation time. The pattern for each is identical: declare host resource type singletons per kind, replace `New(rep, own)` with `NewResourceHandle(rep, own, hostResourceType)`, type-assert at retrieval.

**Non-Goals reminder:** the design's Non-Goals section says backwards compatibility with `api/component`'s public API is not a goal (`api/component` is pre-production and pre-use). The plan SHOULD update `api/component` to the new types; it should NOT add transitional shims that preserve the old runtime API alongside the new one. No parallel paths.

### V6 — ValTypeOpcode constants are all present

Verified in `internal/component/binary/valtype.go`:

- Composite (lines 74-82): `ValTypeOpcodeRecord = 0x72`, `ValTypeOpcodeVariant = 0x71`, `ValTypeOpcodeList = 0x70`, `ValTypeOpcodeTuple = 0x6f`, `ValTypeOpcodeFlags = 0x6e`, `ValTypeOpcodeEnum = 0x6d`, `ValTypeOpcodeOption = 0x6b`, `ValTypeOpcodeResult = 0x6a`
- Handles (lines 87-88): `ValTypeOpcodeBorrow = 0x68`, `ValTypeOpcodeOwn = 0x69`
- Async (lines 94-96): `ValTypeOpcodeFuture = 0x65`, `ValTypeOpcodeStream = 0x66`, `ValTypeOpcodeFixedSizeList = 0x67`
- Primitive (line 26): `PrimValTypeErrorContext = 0x64` (handled via `IsPrimValType` which accepts `0x64` as a special case at line 68)

All 14 opcodes the design references exist. No additions needed. The design's reference to `ValTypeOpcodeFixedList` is a typo — the real name is `ValTypeOpcodeFixedSizeList`. The planning agent uses `ValTypeOpcodeFixedSizeList`.

### V7 — `runtime.ComponentInstance` matches the spec's single-layer model

Session 0 introduces `runtime.ComponentInstance` following the spec at `definitions.py:256-273`:

```go
type ComponentInstance struct {
    ID             uint32
    Parent         *ComponentInstance       // nil for top-level
    Table          *Table                    // unified handle table
    MayLeave       bool                      // spec may_leave flag
    enterCount     int                       // from wazero's existing InstanceState
    ResourceTypes  []*ResourceType           // per-instance nominal type pool
    Destructors    *DestructorRegistry
    Reentrance     *ReentranceTracker
}
```

**Spec-correctness properties verified:**

1. **`MayLeave` is per-instance, not at a VM level** — matches spec field at `definitions.py:260`. wazero's existing `runtime.InstanceState.mayLeave` field is merged in (renamed `MayLeave`).

2. **`Table` is unified** per `definitions.py:259, 303-315`. Holds heterogeneous handle kinds (resources today; streams/futures/subtasks/error-contexts when async lands) with unique index space across all kinds. **Fixes the latent bug** where wazero's split `ResourceTable` would allow index collisions between resource and async handles.

3. **`ResourceTypes []*ResourceType` with pointer identity** matches the spec's Python `is` check at `:1345`. Each `*ResourceType` is a unique pointer; comparing two of them is equivalent to the spec's identity check. **Fixes the existing bug** in `runtime.ResourceTable.ValidateType` where only `ResourceTypeID` was compared, causing cross-instance type-index collisions to be silently accepted.

4. **Nesting via `Parent` pointer** matches spec field at `:258`. Simpler than wasmtime's aggregating-map approach and a better fit for Go's idioms.

**Session 0 state:** ComponentInstance structure exists and is wired through `LiftContext`/`LowerContext`. `ResourceTypes` is empty at end of Session 0 (Concrete promotion is Session 2 work); resource lift/lower traps with a precise error when the lookup returns nil. Every other instance operation (Enter/Leave, MayLeave checks, Table add/get for resource handles minted internally) works end-to-end.

**Session 0 context shapes** (committed — no open questions):

```go
type LiftContext struct {
    Memory      api.Memory
    Opts        *Options
    Types       *types.ComponentTypes
    Instance    *runtime.ComponentInstance  // the calling instance
    BorrowScope *runtime.BorrowScope         // per-call borrow scope
}

type LowerContext struct {
    Memory      api.Memory
    Opts        *Options
    Realloc     func(oldPtr, oldSize, align, newSize uint32) (uint32, error)
    Types       *types.ComponentTypes
    Instance    *runtime.ComponentInstance
    CallContext *runtime.CallContext
}
```

`LowerContext.Subtask *runtime.Subtask` from the current shape is not carried in the sync shape. Async-era state is added when async lift/lower is wired.

### V8 — Line numbers drift during implementation

The line numbers in this design were captured 2026-04-07 from the current branch. The planning agent must re-grep before each edit — line numbers shift as other edits land. Every `file.go:N` reference is a snapshot, not a guarantee.


## Out of Scope

- Wiring `abi/` into `instance.go` production call sites (Session 1)
- Deleting `instance.go` lift/lower bodies (Session 1)
- Resource `Abstract` → `Concrete` linker promotion (Session 2)
- `typeChecker` cross-component matching (Session 2)
- Async lift/lower implementation (no session)
- WIT-binding codegen typed path (no session)
- Any change to `api/component` public surface beyond what's forced by the internal type changes (public API reshape is a separate concern and can wait until the internal state stabilizes)
