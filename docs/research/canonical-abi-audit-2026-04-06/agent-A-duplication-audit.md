# Canonical ABI Duplication Audit (File 1)

**Date:** 2026-04-06
**Auditor:** Claude (research agent)
**Scope:** `internal/component/` in `wazero`, branch `feat/wasip2-complete-implementation`
**Status:** Research / read-only. **Do not act on the recommendations in this file without further human review.**

> All citations use absolute line numbers in current `main` branch sources. Spec citations reference `debug-vendored/component-model/design/mvp/CanonicalABI.md`.

---

## 1. Executive Summary

The wazero component model runtime contains **at least four parallel implementations** of canonical-ABI lifting/lowering logic, living in three different Go files, operating on three different type representations. None of the four agree with each other, and only one of them (the unused `internal/component/abi` package) comes close to matching the spec.

The four implementations are:

| # | Location | Value type abstraction used | Status in production |
|---|---|---|---|
| **A** | `internal/component/abi/` (`lift.go`, `lower.go`, `flatten.go`, `strings.go`, `context.go`, `resource_lower.go`) | `types.ValType` (clean Go interface) | **Not called by any production code path.** Only referenced in `internal/component/conformance/` tests. Confidence: **high**. |
| **B** | `internal/component/instance.go` — `ExportedFunc.lowerTyped`, `lowerByKind`, `lowerToMemory`, `liftResolvedType`, `liftFieldFromMemory`, `liftResultFromMemory`, `liftRecord`, `liftPrimitiveVal`, `liftStringFromRetptr`, `liftResolvedPrimitiveVal` | `types.ValType` (when resolver succeeds) + `ValTypeRef` (legacy fallback) | Called from `ExportedFunc.Call` for **all exported (guest→host) calls**. |
| **C** | `internal/component/component_linker.go` — `liftFromStack`, `liftRecordFromStack`, `liftOptionFromStack`, `liftVariantFromStack`, `liftListFromMemory`, `liftValFromMemory`, `liftRecordFromMemory`, `liftOptionFromMemory`, `lowerToStack`, `writeResultsToMemory`, `writeRecordToMemory`, `writeValToMemory`, `elemSizeFromTypeRef`, `elemSizeFromTypeDef`, `elemAlignFromTypeRef`, `elemAlignFromTypeDef`, `recordSize`, `recordAlign`, `optionSize`, `optionAlign`, `fieldSizeForType`, `flatSlotCount`, `flattenValType`, `flattenRecordType`, `flattenTupleType`, `flattenOptionType`, `flattenResultType`, `flattenFlagsType`, `flattenVariantType` | `ValTypeRef` + `*TypeDef` + `localTypes map[uint32]*TypeDef` — the legacy three-level triple | Called from `createCanonLowerFunc` for **host-function dispatch when the component function has canonical options (canon lower inside an inline instance)**. |
| **D** | `internal/component/canon_lower.go` — `LoweredFunc`, `liftValFromFlat`, `liftString`, `lowerValToFlatTyped`, `lowerValToFlat`, `lowerString`, plus the helper types `EnumType`, `FlagsType`, `VariantType`, `VariantCaseForLower`, `PayloadType`, `PrimitiveType` | Custom local types (`EnumType`, `FlagsType`, `VariantType`) **separate from everything else**. Also uses `ValTypeRef.Primitive` opcodes. | Called from `createHostModuleExport` (component_linker.go:1876) for **plain FuncDef host imports**. Only handles primitives + strings. |

Additionally, there are **ad-hoc fragments**: `instance.go:2673-2766` defines `liftEnum`, `liftFlags`, `liftVariant`, `liftVariantPayload` that are only referenced from tests in `instance_test.go:1692-1839`; they are dead production code.

Four observations frame the rest of this report:

1. **Data flows through the "wrong" implementation depending on wiring.** A host function registered via `linker.DefineFunc("wasi:...", ...)` goes through impl (D) and cannot handle records, variants, or lists. A host function exported by an inline instance whose canonical options reference a memory and realloc goes through impl (C). An exported guest function called from Go via the public API goes through impl (B). The clean impl (A) is in the repo but never reached.
2. **The `Val` representation is the same everywhere** (`internal/component/val.go`), but the *type context* each impl uses to decide how to lower/lift a `Val` is different. B uses `types.ValType` + `ValTypeRef`; C uses `ValTypeRef` + `*TypeDef` + `localTypes`; D uses opcode bytes from `ValTypeRef.Primitive` + its own custom type structs.
3. **Variant and record despecialization differs between impls.** Impl (A) and the cleaner parts of impl (B) despecialize tuples→records and options/results→variants correctly via `types.Option.asVariant()`, `types.Result.asVariant()`, `types.Tuple.asRecord()`. Impl (C)'s sizing functions (`recordSize`, `optionSize`) do *not* implement full variant sizing at all and don't touch tuples/enums, which means any variant in a retptr-returned result written by `writeResultsToMemory` is laid out by essentially stub code (see §4 for the specific bugs).
4. **The "crossed wiring" that produces the 36 failing tests.** `ExportedFunc.Call` (impl B) lowers the caller's arguments for the guest via `lowerTyped`/`lowerToMemory` and lifts the guest's response via `liftResolvedType`/`liftFieldFromMemory`. On the guest's side, when the guest calls an imported host function, depending on how that import was wired, the lift/lower path runs through impl (C) or impl (D) — often with a subtly different alignment or flattening rule. Values written by one impl and read by another produce garbage whenever the two disagree.

---

## 2. Inventory: Every Function That Performs Canonical-ABI Work

This section lists every function in `internal/component/` that I identified as touching canonical-ABI lift/lower/sizing/alignment/memory layout. Grouping is by the spec operation they implement.

### 2.1 Sizing, alignment, and element size

**Spec:** `alignment(t)`, `elem_size(t)`, `align_to`, `discriminant_type`, `max_case_alignment` (CanonicalABI.md:1842-1985).

| Function | File:line | Input | Implementation |
|---|---|---|---|
| `Bool.Size / Align / FlattenCount` through `Char.Size / Align / FlattenCount` | `internal/component/types/types.go:22-116` | nothing (methods on primitive `ValType` structs) | Hard-coded matches to spec alignments (1/2/4/8). Confidence: **high, spec-correct**. |
| `String.Size / Align / FlattenCount` | `types/types.go:118-125` | `types.String` | Returns 8 / 4 / 2. **⚠ Spec says alignment is 4 — correct. Elem_size is 8 — correct.** Confidence: **high**. |
| `Record.Size / Align / FlattenCount / FieldOffsets` | `types/composite.go:22-70` | `types.Record` | Implements `elem_size_record` and `alignment_record` with the standard "align field, add size, final pad to record alignment" loop. **Spec-correct**. Confidence: **high**. |
| `Variant.DiscriminantSize / Size / Align / FlattenCount / PayloadOffset` | `types/composite.go:91-158` | `types.Variant` | Implements `discriminant_type` → `DiscriminantSize()` (returns 1/2/4 based on case-count boundaries 256/65536). Size uses `align_to(disc_size, max_case_alignment) + max_payload_size`, aligned to variant alignment. Matches spec exactly. Confidence: **high**. |
| `List.Size / Align / FlattenCount / ElementSize / ElementAlign / IsFixedLength` | `types/composite.go:173-207` | `types.List` | Correct for both fixed-length and dynamic lists. **Spec-correct**. Confidence: **high**. |
| `Option.Size / Align / FlattenCount / asVariant` | `types/composite.go:216-236` | `types.Option` | Despecializes to `Variant{none, some(T)}` and delegates. Spec-correct despecialization. Confidence: **high**. |
| `Result.Size / Align / FlattenCount / asVariant` | `types/composite.go:246-266` | `types.Result` | Despecializes to `Variant{ok(T), error(E)}` and delegates. Spec-correct. Confidence: **high**. |
| `Enum.Size / Align / FlattenCount` | `types/composite.go:275-293` | `types.Enum` | `Size = 1/2/4` based on case count. `Align = Size`. Spec-correct because `alignment(discriminant_type)` equals its size for U8/U16/U32. Confidence: **high**. |
| `Flags.Size / Align / FlattenCount` | `types/composite.go:302-338` | `types.Flags` | `Size`: n≤8 → 1, n≤16 → 2, else `4*((n+31)/32)`. **Bug vs spec:** spec says "`assert(0 < n <= 32)`" (CanonicalABI.md:1917) — wazero allows >32 flags. For `n == 0`, wazero returns 0 but spec asserts n > 0. `Align`: n≤8→1, n≤16→2, else 4 — spec-correct for n≤32. For n>32 returns 4 which is an extrapolation. Confidence: **medium** on whether this extrapolation is safe. |
| `Tuple.Size / Align / FlattenCount / ElementOffsets / asRecord` | `types/composite.go:347-371` | `types.Tuple` | Despecializes to a `Record` with `fmt.Sprintf("%d", i)` labels, delegates. Spec-correct. Confidence: **high**. |
| `Own.Size / Align / FlattenCount` | `types/resource.go:22-27` | `types.Own` | 4 / 4 / 1. Spec-correct. Confidence: **high**. |
| `Borrow.Size / Align / FlattenCount` | `types/resource.go:43-49` | `types.Borrow` | 4 / 4 / 1. Spec-correct. Confidence: **high**. |
| `alignTo` | `types/composite.go:73-75` | `offset, align uint32` | Power-of-2 round-up `(offset + align - 1) &^ (align - 1)`. Spec-correct for power-of-2 alignments (which is all canonical-ABI alignments). Confidence: **high**. |
| `alignTo` | `instance.go:1494-1496` | `offset, align uint32` | Same formula. **Duplicate of `types.alignTo`.** Confidence: **high**. |
| `alignTo32` | `instance.go:2000-2005` | `offset, align uint32` | Same formula, except it returns `offset` unchanged when `align == 0`. **Triple duplicate.** Confidence: **high**. |
| `elemSizeFromTypeRef` | `component_linker.go:2852-2875` | `ValTypeRef, *TypeDef` | Parallel `elem_size` over the `ValTypeRef` + `*TypeDef` representation. **Does not handle `Variant`, `Tuple`, `Result`, `Enum`, `Flags`, `Stream`, `Future`, `String` beyond a 4-byte placeholder, or fixed-length `List`.** Line 2873 falls through to `4` for unrecognized types. Confidence: **high** that this is incomplete. |
| `elemSizeFromTypeDef` | `component_linker.go:2828-2849` | `*TypeDef` | Parallel `elem_size`. Has special cases only for `Enum` (1 or 4, missing the U16 case boundary), `Record`, `Option`, `List` (8), `Flags` (`(n+31)/32*4`, which disagrees with spec's 1/2/4 rules for n≤16), and falls through to 4. **Spec-incorrect for Enum>256, Flags≤16, Variant, Tuple, Result.** Confidence: **high**. |
| `elemAlignFromTypeRef` | `component_linker.go:2878-2897` | `ValTypeRef, *TypeDef` | Similar coverage gap. Returns 4 for strings (ptr-align is 4, correct). 8 for `s64/u64/f64`. 4 for everything else. **Does not walk record/variant/option.** Confidence: **high** that this underestimates alignment for records containing 64-bit fields where `fieldAlign` is queried via this function's own recursive call through `recordAlign`. |
| `elemAlignFromTypeDef` | `component_linker.go:2900-2914` | `*TypeDef` | Enum → 1 (**spec-incorrect**, should be 1/2/4 matching size). Record → recursive. Option → recursive. List → 4 (ptr-align). Falls through to 4. **Spec-incorrect for Enum (alignment should match discriminant size), Variant, Flags, Tuple, Result.** Confidence: **high**. |
| `recordSize` | `component_linker.go:2917-2934` | `*RecordTypeDef, localTypes` | Correct loop: align, add, final-pad to max alignment. Matches spec. Confidence: **high** (the loop is right, but it depends on the buggy `elemSizeFromTypeRef`/`elemAlignFromTypeRef`). |
| `recordAlign` | `component_linker.go:2937-2947` | `*RecordTypeDef, localTypes` | Max of field alignments. Correct loop, but depends on the buggy `elemAlignFromTypeRef`. Confidence: **high** on correctness of the aggregate, **medium** on whether it produces the right result for records containing variants/tuples. |
| `optionSize` | `component_linker.go:2950-2963` | `*OptionTypeDef, localTypes` | Hand-rolled variant sizing: `payloadOffset := align_to(1, innerAlign); total := payloadOffset + innerSize; align_to(total, innerAlign or 1)`. Conceptually matches spec's despecialization of option to variant{none,some(T)}, but hand-coded. **Bug:** the final alignment uses `innerAlign` but spec says it should be `max(alignment(disc=u8)=1, innerAlign)` which is just `max(1, innerAlign) = innerAlign` for innerAlign ≥ 1. OK this is fine for innerAlign ≥ 1. Confidence: **medium**. |
| `optionAlign` | `component_linker.go:2966-2973` | `*OptionTypeDef, localTypes` | `max(1, innerAlign)`. Conceptually correct. Confidence: **high**. |
| `fieldSizeForType` | `component_linker.go:3524-3574` | `ValTypeRef, localTypes` | **Fourth parallel** size computation used only by `writeValToMemory` for option-skip and list-element sizing. Covers primitives, `Enum`, `Flags`, `List`, `Option`, `Record`. **Missing Variant, Tuple, Result, Stream, Future.** Flags returns 4 unconditionally. Confidence: **high** this is spec-incorrect for small flags. |
| `flatSlotCount` | `component_linker.go:3020-3070` | `ValTypeRef, localTypes` | Parallel `flatten_type` (counting only, not producing types). Handles Enum, Flags, List, Option, Record, Variant. **Variant case:** returns `1 + maxPayload` but does not use the `join` rule from spec; it just counts slots. **Missing Tuple, Result.** Confidence: **high**. |
| `elementSizeForKind` | `instance.go:2769-2782` | `ValKind` | Size by `ValKind` (not type). Used when no type information is available. **Ignores ValKindString / ValKindList / ValKindRecord / ValKindVariant / ValKindOption / ValKindResult / ValKindTuple / ValKindFlags / ValKindEnum / ValKindOwn / ValKindBorrow** — returns 4 for any of those. Confidence: **high**: this is a known fallback that produces incorrect element sizes for any non-primitive list element. |
| `alignmentForKind` | `instance.go:2785-2798` | `ValKind` | Parallels `elementSizeForKind`. Same coverage gaps. Confidence: **high**. |
| `sizeOfVal` | `instance.go:2802-2844` | `Val` (runtime value) | **Fifth parallel** size computation, this time based on the actual `Val` instance's shape. Used by `writeResultsToMemory` (component_linker.go:3272, 3283, 3298, 3314) for post-discriminant offset arithmetic. Known incorrect: for `ValKindVariant`, returns `4 + sizeOfVal(*payload)` which ignores the max-case-size rule. Confidence: **high** this is spec-incorrect; the size of a variant is not the size of the particular case you happen to be writing. |

**Group A of duplicates: sizing and alignment of composite types.**
- `types.Record.Size()` at `composite.go:22-40` and `recordSize()` at `component_linker.go:2917-2934` both compute record size. The former is spec-correct; the latter depends on buggy underlying `elemSizeFromTypeRef`/`elemAlignFromTypeRef`.
- `types.Option.Size()` at `composite.go:216-218` (via `asVariant`) and `optionSize()` at `component_linker.go:2950-2963` both compute option size.
- `types.Variant.Size()` at `composite.go:103-120` has no counterpart in `component_linker.go` — **`component_linker.go` simply does not know how to size variants in retptr returns**, which is why `writeResultsToMemory:3286-3299` writes a placeholder discriminant of `0` and then advances by `sizeOfVal(*payload)`.
- `types.Flags.Size()` at `composite.go:302-315` and `fieldSizeForType` (flags case at `component_linker.go:3557-3559`, returning 4 unconditionally) disagree on small flags.
- `types.Enum.Size()` at `composite.go:275-285` and `elemSizeFromTypeDef` (enum case at `component_linker.go:2829-2833`, returning 1 for ≤256 cases and 4 otherwise) disagree for `257 ≤ n ≤ 65536` (spec says 2, wazero says 4 in the linker path).

### 2.2 Lifting (memory → host value)

**Spec:** `load(cx, ptr, t)` and its sub-functions `load_int`, `load_string`, `load_record`, `load_variant`, `load_list`, `load_flags`, `lift_own`, `lift_borrow` (CanonicalABI.md:1987-2269).

| Function | File:line | Input | Representation | Called from |
|---|---|---|---|---|
| `LiftHeap` | `abi/lift.go:339-695` | `*LiftContext, types.ValType, offset` | `types.ValType` | Only `internal/component/conformance/*_test.go`. **Dead code in production.** Confidence: **high**. |
| `LiftFlat` | `abi/lift.go:52-336` | `*LiftContext, types.ValType, *FlatIter` | `types.ValType` | Same — only conformance tests. |
| `LiftString` | `abi/strings.go:18-29` | `*LiftContext, offset` | `StringEncoding` from `Opts` | Same — only conformance tests. |
| `liftStringFromPtrLen` / `liftStringUTF8` / `liftStringUTF16` / `liftStringLatin1UTF16` | `abi/strings.go:33-123` | `*LiftContext, ptr, len` | `StringEncoding` | Same. |
| `LiftOwn` / `LiftBorrow` | `abi/lift.go:708-822` | `*LiftContext, handleIdx` | Resource table | Same. |
| `ExportedFunc.liftResolvedType` | `instance.go:794-1202` | `types.ValType, coreResults []uint64, *Subtask, *CallContext` | `types.ValType` **partial**: implements `Record`, `Option`, `Result`, `String`, `Enum`, `Flags` (including a retptr path at line 1005-1015), `Tuple`, `Variant`, `List`. **Missing:** Own/Borrow lifting (handled separately in `ExportedFunc.Call` at lines 362-436), fixed-length list handling. **Bug:** the `Enum` branch at lines 970-989 handles only the "flat" case — it reads `int(coreResults[0])` directly and does not handle the retptr case or the case where the discriminant is smaller than 4 bytes (u8 or u16). Similarly the `Variant` branch at lines 1084-1162 has both a flat and a retptr case, but the flat case at line 1089-1113 only recognizes `flatCount <= 1 or len(coreResults) >= flatCount` which is a different condition from the spec's "flat results exceed MAX_FLAT_RESULTS". | Called from `ExportedFunc.Call` at `instance.go:442` whenever a result type can be resolved to a `types.ValType` and it's the guest returning to the host. |
| `ExportedFunc.liftFieldFromMemory` | `instance.go:1242-1402` | `offset uint32, types.ValType` | `types.ValType` | Called from `liftResolvedType`. Implements all primitives, `String`, `Enum`, `Flags`, `Option`, `List`, `Record`. **Missing: Variant, Tuple, Result, Own, Borrow, fixed-length List.** **Bug at line 1388-1392:** `Record` case iterates fields and accumulates `totalSize += size` without `alignTo` — contradicts spec's `load_record`. **Bug at line 1360-1365:** `Option` case uses `4 + innerSize` for the returned size (because `disc` is hard-coded as a u32) but the discriminant should only be 1 byte. The payload offset is computed as `offset+4` instead of `offset+align_to(1, innerAlign)`. Confidence: **high**. |
| `ExportedFunc.liftResultFromMemory` | `instance.go:1406-1491` | `types.Result, retptr uint32` | `types.Result` | Called from `liftResolvedType` for the retptr case of Result. **Bug at line 1411-1412:** reads a 4-byte u32 as discriminant regardless of `discriminant_type(cases)`. For option/result the spec uses u8 as the discriminant. The code then computes `payloadOffset := alignTo(retptr+4, maxAlign)` using a **+4 hard-coded offset** instead of +1 for the 1-byte discriminant. This over-aligns payload offset by 3 bytes when `maxAlign >= 4`, so the payload is written correctly relative to this offset (the lowering side does the same), but **it does not match the canonical ABI layout**, so any guest that reads the same memory using a spec-compliant reader gets garbage. Confidence: **high**. |
| `ExportedFunc.liftStringFromRetptr` | `instance.go:719-753` | `retptr uint32` | hard-coded UTF-8 | Called from `ExportedFunc.Call` for primitive-string results. **Only UTF-8 — ignores canonical `StringEncoding` option.** Confidence: **high**. |
| `ExportedFunc.liftRecord` | `instance.go:757-790` | `*RecordTypeDef, coreResults []uint64` | `*RecordTypeDef` (the `TypeDef` variant) | Legacy path called from `instance.go:501` when `liftResolvedType` errors. **Bug at line 765:** `sort.Strings(fieldNames)` reorders the fields alphabetically before reading — the spec uses declared field order (`load_record` iterates the fields in order). Confidence: **high**. |
| `ExportedFunc.liftPrimitiveVal` | `instance.go:679-713` | `uint64 coreVal, ValTypeRef` | `ValTypeRef` (opcode bytes) | Called from various places in `instance.go`'s legacy path. Primitives only. Confidence: **high**. |
| `ExportedFunc.liftResolvedPrimitiveVal` | `instance.go:1205-1238` | `uint64 coreVal, types.ValType` | `types.ValType` | Only primitives. Used by `liftResolvedType`. Confidence: **high**. |
| `ExportedFunc.liftOwn` | `instance.go:2312-2348` | `handleIdx uint32, *BorrowScope` | handle table | Called for own results. Does a generation scan loop at line 2324 (`for gen := 1; gen < 1000`) — this is a workaround for the component ABI passing index-only `u32` and wazero's table using generations. Functionally works but is O(1000) per lookup. Confidence: **high** (works, but slow). |
| `ExportedFunc.liftBorrow` | `instance.go:2353-2385` | `handleIdx uint32, *BorrowScope` | handle table | Same generation-scan workaround. Confidence: **high**. |
| `liftFromStack` | `component_linker.go:2547-2678` | `stack []uint64, ValTypeRef, *TypeDef, localTypes, memory` | `ValTypeRef + *TypeDef + localTypes` | Called from `createCanonLowerFunc:2502` — this is the **host-function dispatch path** for canon-lower host functions called from the guest. Handles primitives, String (UTF-8 only), Own, Borrow, Enum, Flags, Record, Option, Variant, List (only list<u8> specially; other list element types go through `liftListFromMemory` which returns `nil` for unsized elements). **Missing: Result, Tuple, Stream, Future, fixed-length List.** Confidence: **high**. |
| `liftRecordFromStack` | `component_linker.go:2681-2691` | same | same | Delegated to by `liftFromStack`. Recurses via `liftFromStack`. Preserves declared field order. Confidence: **high**. |
| `liftOptionFromStack` | `component_linker.go:2694-2706` | same | same | Delegated to by `liftFromStack`. **Bug:** computes `innerSlots := flatSlotCount(...)` and returns `1 + innerSlots`, but the discriminant is 1 slot (i32), and then the inner type's flattened slots follow — this is the correct total consume count, but the payload reading at line 2703 uses `stack[1:]` regardless of slot offsets, which is only correct because the flat representation of an option is always `i32(disc), ...payload_flat`. Note also that **non-Some options still consume payload slots** in the return value (`1 + innerSlots`) but the slot *values* in the core wasm stack are undefined when disc=0. This appears correct. Confidence: **medium**. |
| `liftVariantFromStack` | `component_linker.go:2991-3017` | same | same | Called from `liftFromStack`. Uses `flatSlotCount` to compute `maxPayloadSlots`. **Bug:** does not apply the `join` coercion between actual case payload slots and the joined flat types (spec's `lift_flat_variant` at CanonicalABI.md:2962-2989). Reads `stack[1:]` directly as the case's payload. For variants whose cases have different flat types — e.g., one case with `f32` and another with `i32` — this produces garbage. Confidence: **high**. |
| `liftListFromMemory` | `component_linker.go:2709-2723` | `ptr, length, ValTypeRef, *TypeDef, localTypes, memory` | same | Uses `elemSizeFromTypeDef` which does not handle variant/tuple/result/fixed-length list. Confidence: **high**. |
| `liftValFromMemory` | `component_linker.go:2726-2794` | `offset uint32, ValTypeRef, *TypeDef, localTypes, memory` | same | Handles primitives, Enum (bad: only reads a byte even for larger enums — at line 2757, `b, _ := memory.ReadByteAt(offset)` — always 1 byte regardless of case count), Record, Option, List (list<u8> special-case only, everything else returns nil). **Missing: Variant, Tuple, Result, Flags, String, Own, Borrow.** Confidence: **high**. |
| `liftRecordFromMemory` | `component_linker.go:2797-2812` | `offset, *RecordTypeDef, localTypes, memory` | `*RecordTypeDef + localTypes` | Applies `alignTo` between fields. Correct loop, subject to the bugs in the underlying `elemSizeFromTypeRef` / `elemAlignFromTypeRef`. Confidence: **high** on the loop; **medium** on the final result. |
| `liftOptionFromMemory` | `component_linker.go:2815-2825` | `offset, *OptionTypeDef, localTypes, memory` | same | **Bug:** reads `offset+1+innerAlign-1 &^ (innerAlign-1)` which is `align_to(offset+1, innerAlign)` — this is off from `align_to(1, innerAlign) + offset` by an unsigned wraparound if `offset+1 < innerAlign`. In practice this works out the same for power-of-2 innerAlign. Confidence: **medium**. |
| `LoweredFunc.liftValFromFlat` | `canon_lower.go:292-340` | `*flatIter, ValTypeRef` | `ValTypeRef` (opcode bytes) | Called from `LoweredFunc.liftArgumentsTyped` at `canon_lower.go:276` — the **plain host import path**. **Only handles primitives + string.** Line 294-297 short-circuits non-primitives to `ValS32(...)`. Confidence: **high**. |
| `LoweredFunc.liftString` | `canon_lower.go:343-352` | `ptr, length uint32` | (UTF-8 only) | Called from `liftValFromFlat`. Does no UTF-8 validation. Confidence: **high**. |
| `liftEnum` | `instance.go:2673-2680` | `discriminant uint64, *EnumType` | `*EnumType` (custom from canon_lower.go) | **Only called from `instance_test.go:1692`.** Dead production code. Confidence: **high**. |
| `liftFlags` | `instance.go:2683-2691` | `bitvector uint64, *FlagsType` | `*FlagsType` (custom) | **Only called from `instance_test.go:1727`.** Dead production code. |
| `liftVariant` | `instance.go:2696-2726` | `flat []uint64, *VariantType` | `*VariantType` (custom) | **Only called from `instance_test.go:1754-1839`.** Dead production code. |
| `liftVariantPayload` | `instance.go:2729-2766` | `flatVal uint64, PayloadType` | `PayloadType` interface (custom) | Only called from `liftVariant`. Dead production code. |

### 2.3 Lowering (host value → memory)

**Spec:** `store(cx, v, t, ptr)` and its sub-functions `store_int`, `store_string`, `store_record`, `store_variant`, `store_list`, `store_flags`, `lower_own`, `lower_borrow` (CanonicalABI.md:2272-2705).

| Function | File:line | Input | Representation | Called from |
|---|---|---|---|---|
| `LowerFlat` | `abi/lower.go:14-305` | `*LowerContext, types.ValType, Val` | `types.ValType` | **Dead in production.** Only conformance tests. |
| `LowerHeap` | `abi/lower.go:308-648` | `*LowerContext, types.ValType, Val, offset` | `types.ValType` | Same. |
| `LowerString` / `lowerStringUTF8` / `lowerStringUTF16` / `lowerStringLatin1UTF16` | `abi/strings.go:128-248` | `*LowerContext, string` | `StringEncoding` | Same. |
| `LowerOwn` / `LowerBorrow` | `abi/lower.go:654-683` | `*LowerContext, rep any` | Resource table | Same. |
| `LowerOwnWithType` / `LowerBorrowWithType` | `abi/resource_lower.go:21-59` | `*ResourceTable, ResourceTypeInfo, rep, currentInstanceID` | Resource table with type-tracking | Same. |
| `ExportedFunc.lowerParam` | `instance.go:1518-1523` | `ctx, Val, types.ValType, *CallContext` | `types.ValType` or fallback | Dispatcher for B: chooses `lowerTyped` or `lowerByKind`. Confidence: **high**. |
| `ExportedFunc.lowerTyped` | `instance.go:1601-1852` | `ctx, Val, types.ValType, *CallContext` | `types.ValType` | Handles all of `Bool`..`Char`, `String`, `Record`, `Tuple`, `Enum`, `Flags`, `Option`, `Result`, `Variant`, `List`, `Own`, `Borrow`. **Bugs:** (1) `Variant` at line 1787-1798 pads with zeros to `maxPayload` but does not apply `join` coercion on the written values — spec's `lower_flat_variant` at CanonicalABI.md:3078-3097 says values must be bit-reinterpreted between `f32`↔`i32` and extended between `i32`↔`i64`. (2) `Result` at line 1724-1756 does similar padding without `join` coercion. (3) Does not handle fixed-length lists. Confidence: **high**. |
| `ExportedFunc.lowerByKind` | `instance.go:1856-1961` | `ctx, Val, *CallContext` | `ValKind` | Kind-based fallback when no type info is available. **Bug at line 1917-1918:** uses `elementSizeForKind` / `alignmentForKind` which return 4 for composite kinds, so a `list<record<...>>` lowered via this path gets wrong layout. **Bug at line 1941-1957:** `ValKindRecord` path sorts field names alphabetically (`sort.Strings(fieldNames)`), which disagrees with the declared order used by every other impl. Confidence: **high**. |
| `ExportedFunc.lowerToMemory` | `instance.go:2009-2277` | `ctx, Val, types.ValType, offset` | `types.ValType` | Called when params exceed `MAX_FLAT_PARAMS` and must spill to memory, and from `lowerTyped` for list element writes. Handles all primitive and composite types. **Bug at line 2213-2238:** `Result` writes the discriminant via `WriteUint32Le(offset, 0 or 1)` but then computes `payloadOffset := offset + alignTo(4, payloadAlign)` — this **assumes the discriminant is 4 bytes** (because it wrote u32). Spec says result discriminant is `u8` since there are 2 cases. Impl B is internally consistent (lift reads u32 on line 1412, lower writes u32 on line 2217), but this breaks interop with any spec-compliant reader or with impls A/C/D. Confidence: **high**. **Bug at line 2190-2204:** `Variant` writes discriminant with `discSize` = `t.DiscriminantSize()` (spec-correct), but then `payloadOffset := offset + alignTo(discSize, payloadAlign)` — correct. However, the `Variant` path at line 2160-2212 **does not apply the join coercion** on the payload values, and it writes the payload at its natural type (e.g., an `f32` payload is written as 4 bytes via `WriteFloat32Le`) instead of at the joined flat-type width. For variants like `variant { a(f64), b(i32) }` where both cases share offset=8 in memory, that's fine; but for the flat-args (not memory) path, this is wrong. In memory the padding is determined by `max_case_alignment`, not by the join. Confidence: **medium**. |
| `ExportedFunc.lowerStringParam` | `instance.go:1964-1985` | `ctx, string` | hard-coded UTF-8 | Ignores `StringEncoding` option. Confidence: **high**. |
| `ExportedFunc.allocate` | `instance.go:1988-1997` | `ctx, size, align` | realloc | Trivial wrapper. Confidence: **high**. |
| `LoweredFunc.lowerResults` / `lowerResultsTyped` / `lowerValToFlatTyped` / `lowerString` | `canon_lower.go:355-473` | `[]Val, ValTypeRef` | `ValTypeRef` opcode bytes | Called from `CallWithStack` on the plain host import path. Handles only primitives + string. Any compound type falls through to `lowerValToFlat` which errors for anything non-primitive. Confidence: **high**. |
| `lowerValToFlat` | `canon_lower.go:477-513` | `Val` | `ValKind` | Errors for compound kinds. Confidence: **high**. |
| `lowerEnumToFlat` / `lowerFlagsToFlat` / `lowerVariantToFlat` | `canon_lower.go:516-619` | `Val, *EnumType` etc. | custom `EnumType/FlagsType/VariantType` | **Not called from production code**, only from tests and other custom-type code in the same file. The `lowerVariantToFlat` function at line 572-619 pads to `maxPayloadFlat` but does no `join` coercion. Confidence: **high** that these are dead in production. |
| `writeResultsToMemory` | `component_linker.go:3157-3366` | `ctx, memory, realloc, retptr, []Val, *FuncType` | `*FuncType` + `ValKind` | Called from `createCanonLowerFunc:2524` — this is the retptr-write path for host functions called from the guest. Handles all `ValKind` values. **Multiple bugs:** (1) Line 3263-3285 writes `Result` discriminant as a single byte but advances offset by 4 (and passes subsequent `[]Val{*okVal}` into a recursive call with no type context, so the payload is lowered kind-based — hope for the best). (2) Line 3286-3299 writes `Variant` discriminant as an i32 placeholder `0` with comment `"// Placeholder discriminant"` — **this literally writes zero as the discriminant for every variant** because the function has no access to the variant type info for this result. (3) Line 3175-3179: `ValKindF32` calls `result.U32()` and `ValKindF64` calls `result.U64()` — these **panic at runtime** because the underlying value is `float32`/`float64` stored via `any` and `v.v.(uint32)` / `v.v.(uint64)` fails type assertion. Any host function returning an `f32` or `f64` via retptr panics. Same bugs recur in `writeValToMemory` at lines 3414-3419. (4) Line 3188-3190 writes `Own/Borrow` handles via `result.U32()`; this happens to work because `ValOwn`/`ValBorrow` store the handle as `uint32` even though `Kind != ValKindU32`. Confidence: **high**. |
| `writeRecordToMemory` | `component_linker.go:3369-3384` | `ctx, memory, realloc, offset, map[string]Val, *RecordTypeDef, localTypes` | `*RecordTypeDef + localTypes` | Loops over `recordDef.Fields` in declared order. **Does not apply field alignment** — just calls `writeValToMemory` and lets it return `offset + size`. This means records with mixed-alignment fields are packed instead of aligned. Confidence: **high** this is spec-incorrect. |
| `writeValToMemory` | `component_linker.go:3387-3521` | `ctx, memory, realloc, offset, Val, ValTypeRef, localTypes` | `ValTypeRef + localTypes` | Handles primitives and strings OK. **Bug at line 3402-3407:** `ValKindS16/U16` writes via `WriteUint32Le` (4 bytes!) but returns `offset + 2`. The next field starts 2 bytes later, but the written data is 4 bytes, so this clobbers the next 2 bytes. **Bug at line 3420-3436:** `ValKindEnum` writes a 4-byte discriminant regardless of enum case count. **Bug at line 3437-3459:** `ValKindOption` writes discriminant `0` then advances `offset += 4` (not 1 as the spec says — because the function writes a byte and then adds 4 "for alignment"; but the payload offset should be `align_to(offset+1, innerAlign)`, not `offset+4`). **Bug at line 3441-3449 (None case):** computes `innerSize := fieldSizeForType(ValTypeRef{}, localTypes)` with an **empty ValTypeRef**, which resolves to `4` — so a `none` option of `s64` writes 1 byte discriminant + 3 bytes pad + 4 bytes zero, totalling 8 bytes, but the full option size is 16 bytes (s64 payload is 8-byte aligned). Confidence: **high**. |
| `ExportedFunc.lowerString / lowerStringParam` | `canon_lower.go:442-473`, `instance.go:1964-1985` | `string` | hard-coded UTF-8 | Two independent UTF-8-only string lowerings. Confidence: **high**. |
| `lowerToStack` | `component_linker.go:3074-3152` | `[]uint64, Val, *TypeDef` | `*TypeDef` | Called from `createCanonLowerFunc:2538` on the non-retptr path. Handles only primitives, Enum, Flags, Own, Borrow. **Missing: String, List, Record, Variant, Option, Result, Tuple** — returns 1 and does nothing for those. Confidence: **high**. |

### 2.4 Flattening

**Spec:** `flatten_functype`, `flatten_type`, `flatten_list`, `flatten_record`, `flatten_variant`, `join` (CanonicalABI.md:2707-2841).

| Function | File:line | Representation | Notes |
|---|---|---|---|
| `abi.FlattenParams` / `abi.FlattenResults` / `abi.CoreSignature` / `abi.flattenType` / `abi.flattenRecord` / `abi.flattenTuple` / `abi.flattenVariant` / `abi.flattenOption` / `abi.flattenResult` / `abi.flattenFlags` / `abi.join` | `abi/flatten.go:10-226` | `types.ValType` | **Spec-correct**: uses the real `join` function and applies it to variant payloads. **Dead in production.** Confidence: **high**. |
| `componentTypeToCoreTypes` | `canon_lower.go:154-192` | `ValTypeRef` | Primitives only. Returns `i32` for any non-primitive. Used by `LoweredFunc.CoreSignature`. Confidence: **high**. |
| `coreSignature` | `component_linker.go:3579-3589` | `[]types.ValType` | Wrapper around `flattenParams` / `flattenResults`. Confidence: **high**. |
| `flattenParams` / `flattenResults` / `flattenValType` / `flattenRecordType` / `flattenTupleType` / `flattenOptionType` / `flattenResultType` / `flattenFlagsType` / `flattenVariantType` / `isWiderValueType` / `valueTypeWidth` | `component_linker.go:3592-3797` | `types.ValType` | **Parallel to the `abi/` package.** Duplicate implementation. **Bug in `flattenResultType` at 3690-3727:** uses a home-grown `isWiderValueType` (order: `i32 < f32 < i64 < f64`) instead of the spec's `join` (which returns `i32` for `i32`+`f32` and `i64` for every other mismatch). For result types where `Ok` is `i32` and `Err` is `f32`, spec says `i32`, wazero says `f32`. **Bug in `flattenVariantType` at 3747-3775:** same issue. Confidence: **high**. |
| `flatSlotCount` | `component_linker.go:3020-3070` | `ValTypeRef + localTypes` | Flattened count only. Parallel to `types.ValType.FlattenCount()`. Missing Tuple/Result. Confidence: **high**. |
| `types.X.FlattenCount()` methods | `types/types.go`, `types/composite.go` | `types.ValType` | Spec-correct counts. `Variant.FlattenCount()` at `composite.go:134-145` returns `1 + maxPayload` (correct), but it doesn't describe the *types* — only the count. Confidence: **high**. |

**Group B of duplicates: flattening.**
- `abi.flattenType` at `abi/flatten.go:55-102` vs. `flattenValType` at `component_linker.go:3616-3654`. Same inputs, same outputs. The abi version is correct (uses `join`); the linker version is incorrect (uses a width ordering that is not the spec's `join`).
- `abi.CoreSignature` at `abi/flatten.go:41-51` vs. `coreSignature` at `component_linker.go:3579-3589`.

### 2.5 `canon lift` and `canon lower`

**Spec:** `canon_lift`, `canon_lower` (CanonicalABI.md:3197-3590).

| Function | File:line | Input | Notes |
|---|---|---|---|
| `ExportedFunc.Call` | `instance.go:133-675` | `ctx, ...Val` | **This is the de-facto `canon lift` wrapper for exported guest functions.** It (1) computes a subtask and borrow scope, (2) sets `may_leave = false`, (3) resolves param types, (4) picks between flat lowering and memory spill based on `MAX_FLAT_PARAMS`, (5) allocates a retptr if needed, (6) calls the core function, (7) lifts results via `liftResolvedType` or the legacy path, (8) deliver-resolves the subtask. **Huge function** (540 lines), mixing all concerns. Roughly matches the spec's `canon_lift` → `lower_flat_values(args)` → `callee(flat_args)` → `lift_flat_values(results)` flow but does so inline with special cases for each result type shape. Confidence: **high** that it is the primary call path; **medium** that it handles all spec corner cases correctly. |
| `createCanonLowerFunc` | `component_linker.go:2430-2543` | host-side `ComponentFunc`, `canonLowerInfo`, options | **The `canon lower` wrapper for host imports inside inline instances.** Corresponds to the spec's `canon_lower`. Lifts args via `liftFromStack`, calls the host func, lowers results via `lowerToStack` or `writeResultsToMemory`. Does NOT implement the `may_leave` / `Subtask` state machine. Confidence: **high**. |
| `createHostModuleExport` / `LoweredFunc.CallWithStack` | `component_linker.go:1868-1891`, `canon_lower.go:201-221` | host `FuncDef` | **The other `canon lower` wrapper**, for plain host imports defined via `linker.DefineFunc`. Uses the separate `LoweredFunc` / `liftValFromFlat` / `lowerValToFlatTyped` / `lowerString` stack. **Only handles primitives and strings.** Confidence: **high**. |

**Group C of duplicates: canon lower wrapping.**
- `LoweredFunc.CallWithStack` (canon_lower.go:201) and `createCanonLowerFunc` (component_linker.go:2430) both implement the spec's `canon_lower` but for different import shapes.

### 2.6 Resource handles

**Spec:** `canon_resource_new`, `canon_resource_drop`, `canon_resource_rep`, `lift_own`, `lift_borrow`, `lower_own`, `lower_borrow` (CanonicalABI.md:3590-3691).

| Function | File:line | Notes |
|---|---|---|
| `ResourceTable.New` / `NewWithType` / `Get` / `Remove` / `Rep` | `resource_table.go` | Central table implementation. **Generation-tracked** (handle is a u64 = gen<<32|idx). Confidence: **high**. |
| `Instance.ResourceNew` / `ResourceRep` / `ResourceDrop` | `instance.go:2389-2439` | Direct table wrappers. Confidence: **high**. |
| `createResourceOpExport` | `component_linker.go:2149-2201` | Wires `resource.drop/new/rep` core-level functions. Confidence: **high**. |
| `ExportedFunc.liftOwn` / `liftBorrow` | `instance.go:2312-2385` | Guest-returned-own/borrow path used by `ExportedFunc.Call`. **Generation-scan workaround** (loops 1..1000). Confidence: **high**. |
| `abi.LiftOwn` / `LiftBorrow` / `LowerOwn` / `LowerBorrow` | `abi/lift.go:708-822`, `abi/lower.go:654-683` | Same generation-scan workaround. Dead code path. Confidence: **high**. |
| `LowerOwnWithType` / `LowerBorrowWithType` | `abi/resource_lower.go:21-59` | Implements the same-instance optimization from spec `lower_borrow` (CanonicalABI.md:2679-2683). Dead code path. Confidence: **high**. |

**Duplication group D: handle lift/lower.** Two impls (`instance.go` and `abi/`) with near-identical code — the `abi/` version never runs in production.

---

## 3. Parallel Type Representations and Conversion Paths

There are **three distinct representations** of component value types in the runtime:

1. **`TypeDef`** (`component.go:213-271`) — the decoder output. A discriminated-union struct with pointers for each kind (Record, Variant, List, etc.). Primitives are stored in a `Handle *ValTypeRef` field when the TypeDef is a primitive alias. This is the representation produced by the binary parser.

2. **`ValTypeRef`** (`component.go:359-377`) — a lightweight reference: `IsPrimitive` + `Primitive byte` opcode for primitives; `TypeIdx uint32` for non-primitives; `IsOwn/IsBorrow` flags with `TypeIdx` pointing at the resource type. This is what gets stored inside fields of `RecordTypeDef`, `VariantCase`, etc.

3. **`types.ValType`** (`types/types.go:7-20`) — a clean Go interface with `Size()`, `Align()`, `FlattenCount()` methods. Implemented by `types.Bool`, `types.U32`, `types.Record`, `types.Variant`, `types.List`, `types.Option`, `types.Result`, `types.Tuple`, `types.Enum`, `types.Flags`, `types.Own`, `types.Borrow`.

### Conversion paths

- `TypeResolver.ResolveValType(ValTypeRef) (types.ValType, error)` at `type_resolver.go:37-51` converts a `ValTypeRef` into a `types.ValType`. **This is the intended conversion**. It recursively resolves record/variant/option/result fields. It also has a `withLocalTypes` variant for cross-instance-type scope resolution.
- `resolveToValType(NamedValType, *TypeResolver) types.ValType` at `component_linker.go:722-746` is a second path used by the linker for param/result type resolution. Internally delegates to `TypeResolver.ResolveValType`.
- `typeDefToValType(*TypeDef, localTypes) types.ValType` at `component_linker.go:748-831` is a **third conversion path**, operating directly on `TypeDef` instead of `ValTypeRef`. This duplicates the resolver's dispatch logic.
- `valTypeRefToValType(ValTypeRef, localTypes) types.ValType` at `component_linker.go:833-903` is a **fourth conversion path** that combines the primitive-byte dispatch of `TypeResolver.resolvePrimitive` with a `localTypes` lookup.

**Crossed wiring example:** When the instance processes a record field that references another type, `TypeResolver.resolveTypeIdx` at `type_resolver.go:99-170` first checks `localTypes`, then the component's `TypeIdxToStoredIdx`, then the instance's `typeSpace`, and finally falls back to a direct index into `component.Types`. The same lookup logic is NOT applied in `component_linker.go`'s `resolveInnerType` at line 2978-2988, which only does a `localTypes[ref.TypeIdx]` lookup and returns nil if not found. If the same record type is reachable from both paths with different `localTypes` scopes, it may size differently in the lifting direction (via resolver) vs the lowering direction (via linker).

### "Crossed wiring": places where representations are converted and converted back

- **At the canon-lower host boundary** (`createCanonLowerFunc:2497-2506`), arguments are lifted from the core stack using `ValTypeRef` + `*TypeDef` (`liftFromStack`, impl C), but before that the linker already has the `ResolvedType` in the `NamedValType.ResolvedType` field. The code uses `paramDef.ResolvedType` (a `*TypeDef`) plus `localTypes` instead of the already-resolved `types.ValType`. The `types.ValType` is never carried through.
- **In `ExportedFunc.Call`** (`instance.go:133-675`), the code first resolves *all* params to `types.ValType` (line 195-202) using the resolver, then passes them to `lowerTyped`. On the lifting side, it calls `resolver.ResolveValType` *again* for the result type (line 440) before dispatching to `liftResolvedType`. This is fine in isolation but wasteful — and it re-exposes a subtle bug: the resolver cache lives on the `TypeResolver` instance, not on the instance, so every call creates a new resolver and re-does the work.
- **`writeResultsToMemory`** (`component_linker.go:3157`) receives a `*FuncType` (legacy, ValTypeRef-based) and a list of `Val`s, and internally falls back to `ValKind`-based dispatch instead of resolving the type for each result. The result type information is carried only indirectly via `funcType.Results[i].ResolvedType` — which is read only in the `ValKindEnum`/`ValKindFlags`/`ValKindRecord` cases (lines 3318, 3334, 3350). For `ValKindVariant` and `ValKindResult` the resolved type is completely ignored, which is why the discriminant is hard-coded to `0` in the variant case.
- **`createHostModuleExport`** (`component_linker.go:1876`) uses `CanonLower(funcDef.Callback, funcDef.Type, nil)` and relies on `LoweredFunc.CoreSignature()` (`canon_lower.go:130-149`) which in turn calls `componentTypeToCoreTypes` on each param. This ignores `types.ValType` entirely and works directly off `ValTypeRef.Primitive` opcodes, returning `i32` for anything non-primitive. As a result, **any host import registered via `linker.DefineFunc` that takes a record, variant, list, or option has the wrong core signature** — it gets `i32` where the spec says the flattened sequence. When the guest calls such an import, the core-wasm types don't match, and the wasm runtime either rejects the call at link time or passes garbage.

---

## 4. Intended Single Source of Truth

### What the code documents

- `types/types.go:1-20` is the cleanest: a documented `ValType` interface with `Size()`, `Align()`, `FlattenCount()` as the three operations every spec-needed sizing/alignment/flattening function reduces to. Its package comment is BSD-licensed header from "The Go Authors", which is unusual in this repo — it suggests it was imported from elsewhere and the rest of the codebase has not caught up.
- `abi/` package doc at `abi/context.go:11-22` says: "Flat ABI limits as defined by the Component Model specification. These determine when values are passed in registers (flat) vs. memory (heap)." The package is organized around `LiftContext` / `LowerContext` / `LiftFlat` / `LiftHeap` / `LowerFlat` / `LowerHeap` which mirrors wasmtime's `Lift`/`Lower` trait split (see `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/func/typed.rs:794` and `889`). **The `abi/` package is clearly the intended single source of truth.** However, there is no README or design doc saying so, and it is not wired into production.

### What the majority of the code does

The majority of actual runtime code paths (`instance.go`, `component_linker.go`, `canon_lower.go`) do **not** use `abi/`. They reimplement sizing/alignment/lifting/lowering inline. The de-facto source of truth is a three-way split:

- Exported guest→host calls: `instance.go` functions (impl B).
- Canon-lower host imports with canonical options: `component_linker.go` functions (impl C).
- Plain `linker.DefineFunc` host imports: `canon_lower.go` functions (impl D).

### Proposed design intent (based on what the majority *should* do)

Based on the structure of `abi/` and the shape of `types.ValType`, the clearly intended architecture is:

1. Decode binary → `TypeDef` (in `internal/component/binary/`).
2. At link time, resolve once: `ValTypeRef` + `localTypes` → `types.ValType` via `TypeResolver`. Cache per-instance.
3. All runtime lifting/lowering operates on `types.ValType` only, through a single `abi` package.
4. The call paths (`ExportedFunc.Call`, `createCanonLowerFunc`, `createHostModuleExport`) all delegate to the same `abi.LiftFlat` / `abi.LiftHeap` / `abi.LowerFlat` / `abi.LowerHeap`.

This is **exactly** how wasmtime is structured: `Lift::linear_lift_from_flat` / `linear_lift_from_memory` / `Lower::linear_lower_to_flat` / `linear_lower_to_memory` (see `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/func/typed.rs:889` and `814`).

**If this design intent is documented anywhere in-repo, I did not find it.** There is no README in `internal/component/`, no comment in `abi/context.go` or `lift.go` that says "this is the one implementation", and no deprecation comment on `instance.go`'s `liftResolvedType` or `component_linker.go`'s `writeResultsToMemory`. Confidence: **high** that the single-source-of-truth intent is undocumented.

---

## 5. Duplicate Groups and Recommendations

> **Do not act on these recommendations without further human review.** These are my suggestions based on spec alignment and internal consistency, not on test coverage or integration points I may have missed.

### Group 1: `alignTo`

- `types/composite.go:73-75` (`alignTo`)
- `instance.go:1494-1496` (`alignTo`)
- `instance.go:2000-2005` (`alignTo32`, differs in that it handles `align == 0`)

All three are identical to within the `align == 0` edge case. **Keep `types/composite.go`'s exported `AlignTo`** (or export the internal one) and delete the rest. **Rationale:** `align == 0` should never happen for a valid type — if it does it's a bug.

### Group 2: Sizing of records/options/variants/enums/flags

**Keep:** `types/composite.go` (Record, Variant, Option, Result, Tuple, Enum, Flags). **Spec-correct, high confidence.**

**Delete:**
- `component_linker.go:2828-2973` (`elemSizeFromTypeDef`, `elemSizeFromTypeRef`, `elemAlignFromTypeRef`, `elemAlignFromTypeDef`, `recordSize`, `recordAlign`, `optionSize`, `optionAlign`)
- `component_linker.go:3524-3574` (`fieldSizeForType`)
- `instance.go:2769-2844` (`elementSizeForKind`, `alignmentForKind`, `sizeOfVal`) — these are used as fallbacks when no type info is available. If the intent is that type info is *always* available, remove them; otherwise replace with functions that take a `types.ValType`.

**Rationale:** `component_linker.go`'s parallel implementation does not handle Variant, Tuple, or Result sizing at all. Its Enum sizing is wrong for `257 ≤ n ≤ 65536`. Its Flags sizing is wrong for `n ≤ 16`. `sizeOfVal` is spec-nonsense (size of a variant is not the size of one case).

### Group 3: Flattening

**Keep:** `abi/flatten.go`. Uses the real `join` function on variant payloads. Spec-correct. High confidence.

**Delete:**
- `component_linker.go:3592-3797` (`coreSignature`, `flattenParams`, `flattenResults`, `flattenValType`, `flattenRecordType`, `flattenTupleType`, `flattenOptionType`, `flattenResultType`, `flattenFlagsType`, `flattenVariantType`, `isWiderValueType`, `valueTypeWidth`)
- `canon_lower.go:154-192` (`componentTypeToCoreTypes`)

**Rationale:** `component_linker.go`'s `join`-equivalent (`isWiderValueType`) uses a width ordering `i32 < f32 < i64 < f64` that is **not the spec's `join`**. Spec's join: `join(a,b) = a if a==b; i32 if {i32,f32}; i64 otherwise` — this means `join(f32, i32) = i32`, not `f32`. The linker's version says `f32` is wider than `i32` and picks `f32`, which is wrong. This produces an incorrect core signature for result types with mismatched ok/err primitives, and for variants with mixed-primitive cases.

### Group 4: Lifting from flat (core stack) to Val

**Keep:** `abi/lift.go` `LiftFlat`. Spec-correct variant handling with real `join`-based coercion loop (`coerceFlatValue` at `lift.go:867-888`). High confidence.

**Delete:**
- `canon_lower.go:276-352` (`liftArgumentsTyped`, `liftValFromFlat`, `liftString`)
- `component_linker.go:2547-3017` (`liftFromStack`, `liftRecordFromStack`, `liftOptionFromStack`, `liftVariantFromStack`, `flatSlotCount` — note that `flatSlotCount` has a use that the `types.ValType.FlattenCount()` method already covers)
- `instance.go:679-713` (`liftPrimitiveVal`) and `instance.go:1205-1238` (`liftResolvedPrimitiveVal`)

**Rationale:** These reimplementations are all missing the variant payload coercion (`join` bit-reinterpretation) and don't handle fixed-length lists. The abi/ implementation also misses fixed-length lists inside variants (see `abi/lift.go:278-293` which checks `t.Length != nil` at the top but does not handle that inside a variant case — minor), but is closer to spec.

### Group 5: Lifting from memory (heap) to Val

**Keep:** `abi/lift.go` `LiftHeap`. Spec-correct alignment. High confidence.

**Delete:**
- `instance.go:1242-1491` (`liftFieldFromMemory`, `liftResultFromMemory`) — has alignment bugs (line 1388-1392 for records; line 1411-1412 for result discriminant size).
- `instance.go:792-1202` (`liftResolvedType`) — large function that mixes "retptr vs flat" dispatch logic with the lifting itself. The dispatch logic (deciding whether core results encode a retptr or flat values) should stay in `ExportedFunc.Call`, but the actual lifting should delegate to `abi.LiftFlat` / `abi.LiftHeap`.
- `component_linker.go:2709-2825` (`liftListFromMemory`, `liftValFromMemory`, `liftRecordFromMemory`, `liftOptionFromMemory`)
- `instance.go:719-753` (`liftStringFromRetptr`) — only handles UTF-8.
- `instance.go:757-790` (`liftRecord`) — **sorts fields alphabetically**, violates spec.

**Rationale:** The abi/ version applies spec-correct alignment in record loops (`lift.go:427-441`), reads discriminants with the correct size based on case count, and handles all composite types.

### Group 6: Lowering Val to flat (core stack)

**Keep:** `abi/lower.go` `LowerFlat`. Uses spec-correct `join` coercion (`lower.go:704-731`).

**Delete:**
- `canon_lower.go:355-619` (`lowerResults`, `lowerResultsTyped`, `lowerValToFlatTyped`, `lowerString`, `lowerValToFlat`, `lowerEnumToFlat`, `lowerFlagsToFlat`, `lowerVariantToFlat`) — all of these. Also delete the custom `EnumType`, `FlagsType`, `VariantType`, `VariantCaseForLower`, `PayloadType`, `PrimitiveType` type declarations at `canon_lower.go:13-62`.
- `instance.go:1516-1961` (`lowerParam`, `typeCanCoerce`, `typeMatchesKind`, `lowerTyped`, `lowerByKind`, `lowerStringParam`, `allocate`) — all of these.
- `component_linker.go:3074-3152` (`lowerToStack`).

**Rationale:** `lowerTyped`/`lowerByKind` has the alphabetical-field-name bug (line 1947), and `lowerToStack` only handles primitives + enum + flags + handles. `canon_lower.go`'s stack is entirely independent and only handles primitives + strings.

### Group 7: Lowering Val to memory (heap)

**Keep:** `abi/lower.go` `LowerHeap`.

**Delete:**
- `instance.go:2009-2277` (`lowerToMemory`) — has the 4-byte result discriminant bug (line 2217) which makes it internally consistent but spec-incompatible.
- `component_linker.go:3157-3521` (`writeResultsToMemory`, `writeRecordToMemory`, `writeValToMemory`) — has the literal `// Placeholder discriminant` bug at line 3292, the 4-byte u32 write for s16/u16 bug at line 3403-3407, the zero-ValTypeRef fieldSizeForType bug at 3443-3449.

**Rationale:** The abi/ version writes discriminants at spec-correct sizes (1 for variants with ≤256 cases) and correctly computes payload offsets.

### Group 8: `canon_lower` wrapper

**Keep (rewired):** One unified function that wraps the delegation to `abi.LiftFlat` + host call + `abi.LowerFlat` / `abi.LowerHeap`. Both `createCanonLowerFunc` (component_linker.go:2430) and `createHostModuleExport` (component_linker.go:1868) should route through this.

**Delete:** The bifurcation between `LoweredFunc.CallWithStack` and `createCanonLowerFunc`. Replace with a single wrapper.

**Rationale:** The two wrappers implement the same spec function (`canon_lower`) for different import registration paths, producing two sets of bugs that diverge over time. The failing `TestHostImport_*` tests are in this zone — they register a host function via `linker.DefineFunc` with a record or option param, hit `createHostModuleExport` → `CanonLower` → `liftValFromFlat`, and get `ValS32(...)` back for a record type because `liftValFromFlat` doesn't know how to handle non-primitives.

### Group 9: Resource handle lift/lower

**Keep:** `abi/resource_lower.go` (`LowerOwnWithType`, `LowerBorrowWithType`) — these implement the same-instance optimization from spec lines 2679-2683.

**Delete:** `abi/lift.go:708-822` (`LiftOwn`, `LiftBorrow`) and `instance.go:2312-2385` (`ExportedFunc.liftOwn`, `liftBorrow`). Both do the same generation-scan workaround. Replace with a direct `ResourceTable` lookup that receives a full `Handle` (including generation) from the call site — this requires the call site to track the generation, which in turn requires the `liftFromStack` / `LiftFlat` paths to be given generation-tracking context.

**Rationale:** The generation-scan workaround is O(1000) per lookup and will eventually break if handle indices are reused more than 999 times.

### Group 10: Dead test-only code

**Delete:**
- `instance.go:2673-2766` (`liftEnum`, `liftFlags`, `liftVariant`, `liftVariantPayload`) — only referenced from `instance_test.go`.
- The custom types in `canon_lower.go:13-62` (`EnumType`, `FlagsType`, `VariantType`, `VariantCaseForLower`, `PayloadType`, `PrimitiveType`) — only used by the dead `lowerEnumToFlat`, `lowerFlagsToFlat`, `lowerVariantToFlat` and by the dead `liftVariantPayload`.

**Rationale:** Clean-up.

---

## 6. Confidence Summary

- **High confidence** on: the existence of four parallel implementations; the fact that `abi/` is never called from production; the identities of the duplicate function groups; the spec deviation in `flattenResultType` / `flattenVariantType` / `isWiderValueType`; the alphabetical-field-sort bug in `liftRecord` and `lowerByKind`; the "placeholder discriminant" bug in `writeResultsToMemory`; the s16/u16 4-byte write bug in `writeValToMemory`; the UTF-8-only assumption in the production string lift/lower.

- **Medium confidence** on: the correctness claims about each alternative impl under the retptr/flat corner cases (I did not trace every code path end-to-end); whether the option size wraparound in `liftOptionFromMemory` is actually triggered; whether `lowerTyped`'s Variant padding is wrong in practice for memory layout (vs. flat args, where it is clearly wrong).

- **Low confidence** on: whether my duplicate-deletion recommendations will pass the test suite as-is, and whether any of the "dead" conformance-test-only abi functions have subtle differences from the production impls that the tests have been implicitly relying on. The recommendation is to do this under test coverage with careful diffing of behavior before removal.

---

## 7. Specific Failing-Test-to-Impl Map

Cross-referencing the 36 failing tests in `internal/component/wasip2test/`:

- **`TestRepro_StringParameterSupport`** / `TestEcho_*` / `TestEcho_Enum` / `TestEcho_Flags` / `TestEcho_Variant` (`repro_test.go:247-487`) — These call `getHandlerFunc(...).Call(...)`, which routes through `ComponentFuncWrapper.Call` → `ExportedFunc.Call` → **impl B** (`instance.go`). Primitives and strings should work in impl B; if they're failing it's because of a subtle regression in `liftResolvedType` or `lowerTyped`.

- **`TestHostImport_*`** (`repro_test.go:635-1046`) — These register a host function via `linker.DefineInstance(...).Func(name, fn).Build()`, then the guest calls that function. The call path is `guest wasm` → host module export → `createHostModuleExport` → **impl D** (`canon_lower.go`). Any test that takes a record, variant, option, or list as a host-function argument will fail because impl D only handles primitives + strings.

- **`TestPublicAPI_*`** (`repro_test.go:1047-...`) — These call `getHandlerFunc(...).Call(...)` with records/options/variants/lists/strings. Route through **impl B**. Failures here are in `lowerTyped`/`lowerToMemory`/`liftResolvedType`/`liftFieldFromMemory`.

- **`TestCalculatorPlugins/multi`** / **`TestWasiExercise_Go`** / **`TestWasiExercise_Rust`** — These instantiate a WASI-P2 component that imports wasip2 host functions. The wasip2 host functions are registered in `imports/wasip2/`. **I did not read the wasip2 import registration code in this audit.** If wasip2 uses `linker.DefineFunc` directly, then the wasip2 import path for any function with non-primitive types hits impl D and fails.

- **`TestProperty_*`** — I didn't enumerate these but they likely exercise a mix of paths.

**Hypothesis (medium confidence):** The 36 failing tests are primarily in impls B and D. The parallel implementation that was reverted probably provided a consistent D (replacing `canon_lower.go`'s broken record/variant handling) that the tests had come to depend on. Without it, tests that depend on host functions taking non-primitive types fail because impl D doesn't know how to lift them.

---

*End of File 1.*
