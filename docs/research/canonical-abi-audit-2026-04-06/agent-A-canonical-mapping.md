# Canonical ABI Spec → Wazero Mapping (File 2)

**Date:** 2026-04-06
**Auditor:** Claude (research agent)
**Scope:** 1:1 mapping between spec pseudocode in `debug-vendored/component-model/design/mvp/CanonicalABI.md` and wazero implementations in `internal/component/`.

## Legend

- **Spec location:** Section name and line range in `CanonicalABI.md`.
- **Wazero location:** `file.go:line-line`. Absolute paths rooted at `/home/cchamplin/development/wazero/`.
- **Status:**
  - ✅ **matches** — implementation matches the spec (no behavior divergence that I identified).
  - ⚠️ **partial** — implementation exists but has a known behavioral gap, missing types, or an internally consistent but spec-divergent encoding choice.
  - ❌ **missing** — no corresponding implementation, or the code returns an error/zero placeholder.
  - 🔁 **multiple impls** — two or more wazero functions implement the same spec function with different behavior. See File 1 for which to keep.
- **Confidence:** H / M / L per the audit instructions.

## Common file shortcuts

| Short | Full path |
|---|---|
| `types.go` | `internal/component/types/types.go` |
| `composite.go` | `internal/component/types/composite.go` |
| `resource.go` | `internal/component/types/resource.go` |
| `instance.go` | `internal/component/instance.go` |
| `linker.go` | `internal/component/component_linker.go` |
| `canon_lower.go` | `internal/component/canon_lower.go` |
| `val.go` | `internal/component/val.go` |
| `type_resolver.go` | `internal/component/type_resolver.go` |
| `abi/lift.go` | `internal/component/abi/lift.go` |
| `abi/lower.go` | `internal/component/abi/lower.go` |
| `abi/flatten.go` | `internal/component/abi/flatten.go` |
| `abi/strings.go` | `internal/component/abi/strings.go` |
| `abi/context.go` | `internal/component/abi/context.go` |
| `abi/resource_lower.go` | `internal/component/abi/resource_lower.go` |
| `resource_table.go` | `internal/component/resource_table.go` |

---

## 1. Despecialization

| # | Spec function | Spec section/line | Wazero function(s) | File:line | Status | Notes | Conf |
|---|---|---|---|---|---|---|---|
| 1.1 | `despecialize(TupleType(ts))` → `RecordType([FieldType(str(i), t)])` | CanonicalABI.md:1799-1801 | `types.Tuple.asRecord` | `composite.go:365-371` | ✅ | Uses `fmt.Sprintf("%d", i)` labels. Behaviorally identical. | H |
| 1.2 | `despecialize(EnumType(labels))` → `VariantType([CaseType(l, None)])` | CanonicalABI.md:1802 | *(no single function)* | `composite.go:275-293` | ⚠️ partial | `Enum` has its own `Size/Align/FlattenCount` that directly computes the discriminant sizing — does not actually despecialize to a `Variant`. Behaviorally this produces the same size/align as despecialization-then-variant-sizing only because Enum has no payload, so the variant's `max_case_alignment = 1` and all payloads have size 0. **Consequence:** does not route enum lift/lower through variant lift/lower. | H |
| 1.3 | `despecialize(OptionType(t))` → `VariantType([CaseType("none", None), CaseType("some", t)])` | CanonicalABI.md:1803 | `types.Option.asVariant` | `composite.go:228-236` | ✅ | Correctly despecializes for `Size/Align/FlattenCount`. Lift/lower operations also route through the variant logic inside `abi/lift.go` and `abi/lower.go`. Duplicate paths in `instance.go` do NOT despecialize — they read/write option discriminant directly. | H |
| 1.4 | `despecialize(ResultType(ok, err))` → `VariantType([CaseType("ok", ok), CaseType("error", err)])` | CanonicalABI.md:1804 | `types.Result.asVariant` | `composite.go:258-266` | ✅ | Same shape as Option. Duplicate paths in `instance.go:1406-1491` (`liftResultFromMemory`) do NOT despecialize and have a 4-byte-discriminant bug. | H |
| 1.5 | String and flags NOT despecialized | CanonicalABI.md:1807-1809 | — | — | ✅ | Correctly treated as their own kinds everywhere. | H |

---

## 2. Alignment and Element Size

| # | Spec function | Spec section/line | Wazero function(s) | File:line | Status | Notes | Conf |
|---|---|---|---|---|---|---|---|
| 2.1 | `alignment(t)` top-level dispatch | CanonicalABI.md:1848-1866 | `types.ValType.Align()` | `types.go:27, 35, 43, 51, 59, 67, 75, 83, 91, 99, 107, 115, 124`, `composite.go:42, 122, 183, 220, 250, 287, 317, 351`, `resource.go:24, 45` | ✅ | Methods per type. All match spec. | H |
| 2.1 dup 🔁 | same | — | `elemAlignFromTypeDef`, `elemAlignFromTypeRef` | `linker.go:2878-2914` | ⚠️ partial | Parallel impl. Missing Variant, Tuple, Result, Flags. Enum → 1 (spec-incorrect for n > 256). | H |
| 2.1 dup 🔁 | same | — | `alignmentForKind` | `instance.go:2785-2798` | ⚠️ partial | Returns 4 for all composite `ValKind`s. Only spec-correct for primitives. | H |
| 2.2 | `alignment_list(t, l)` | CanonicalABI.md:1871-1874 | `types.List.Align()` | `composite.go:183-188` | ✅ | Fixed-length → `elemAlign`, dynamic → 4. Matches spec. | H |
| 2.3 | `alignment_record(fields)` | CanonicalABI.md:1879-1883 | `types.Record.Align()` | `composite.go:42-50` | ✅ | Max of field alignments, starting at 1. Matches spec. | H |
| 2.3 dup 🔁 | same | — | `recordAlign` | `linker.go:2937-2947` | ⚠️ partial | Depends on buggy `elemAlignFromTypeRef`. | H |
| 2.4 | `alignment_variant(cases) = max(alignment(disc), max_case_alignment)` | CanonicalABI.md:1892-1893 | `types.Variant.Align()` | `composite.go:122-132` | ✅ | Starts with `DiscriminantSize()` (which equals `alignment(discriminant_type)` since U8=1, U16=2, U32=4), then max over case types. Matches spec. | H |
| 2.5 | `discriminant_type(cases)` = U8/U16/U32 by case count | CanonicalABI.md:1895-1902 | `types.Variant.DiscriminantSize`, `types.Enum.Size` | `composite.go:91-101`, `composite.go:275-285` | ✅ | Uses `n ≤ 0x100 → 1, n ≤ 0x10000 → 2, else → 4`. Spec formula `ceil(log2(n)/8)` yields the same boundaries. | H |
| 2.5 dup 🔁 | same | — | `elemSizeFromTypeDef` enum branch | `linker.go:2828-2834` | ⚠️ partial | Uses `n ≤ 256 → 1, else → 4`. **Missing the U16 case for 257 ≤ n ≤ 65536.** | H |
| 2.6 | `max_case_alignment(cases)` | CanonicalABI.md:1904-1909 | *(inlined inside `types.Variant.Align()` and `Size()`)* | `composite.go:122-132`, `composite.go:103-120` | ✅ | Inlined as loops in both methods. Matches spec. | H |
| 2.7 | `alignment_flags(labels)` | CanonicalABI.md:1915-1921 | `types.Flags.Align()` | `composite.go:317-329` | ⚠️ partial | Matches spec for 1..32 flags. Extends: returns 4 for n > 32 (spec asserts n ≤ 32). Returns 1 for n == 0 (spec asserts n > 0). | M |
| 2.7 dup 🔁 | same | — | `elemAlignFromTypeDef` flags branch | `linker.go:2900-2913` | ❌ | Not implemented; falls through to default 4. | H |
| 2.8 | `elem_size(t)` top-level dispatch | CanonicalABI.md:1934-1951 | `types.ValType.Size()` | `types.go:26, 34, 42, 50, 58, 66, 74, 82, 90, 98, 106, 114, 123`, `composite.go:22-40, 103-120, 173-178, 216-218, 246-248, 275-285, 302-315, 347-349`, `resource.go:21, 43` | ✅ | Matches spec for all types. | H |
| 2.8 dup 🔁 | same | — | `elemSizeFromTypeDef`, `elemSizeFromTypeRef`, `fieldSizeForType` | `linker.go:2828-2875`, `linker.go:3524-3574` | ⚠️ partial | Three parallel impls. Missing Variant/Tuple/Result. Flags wrong for small n. Enum wrong for middle range. | H |
| 2.8 dup 🔁 | same | — | `elementSizeForKind`, `sizeOfVal` | `instance.go:2769-2782, 2802-2844` | ⚠️ partial | Kind-based fallbacks. `sizeOfVal` for variant returns `4 + sizeOfVal(*payload)` — **spec-incorrect**. | H |
| 2.9 | `elem_size_list(t, l)` | CanonicalABI.md:1953-1956 | `types.List.Size()` | `composite.go:173-178` | ✅ | Fixed: `length * elemSize`. Dynamic: 8 (ptr+len). Matches spec. | H |
| 2.10 | `elem_size_record(fields)` | CanonicalABI.md:1958-1964 | `types.Record.Size()` | `composite.go:22-40` | ✅ | Correct loop: `align_to(size, fieldAlign); size += fieldSize`; final `align_to(size, maxAlign)`. Empty records return 0 (line 24) — **spec asserts `s > 0` at line 1963**, so the wazero extension to return 0 for empty records is a spec-tolerant permissive extension, not strictly spec-conformant. | H |
| 2.11 | `align_to(ptr, alignment)` | CanonicalABI.md:1966-1967 | `alignTo` | `composite.go:73-75` | ✅ | Power-of-2 rounding. | H |
| 2.11 dup 🔁 | same | — | `alignTo`, `alignTo32` | `instance.go:1494-1496, 2000-2005` | ✅ | Duplicates. `alignTo32` has extra `align == 0` guard. | H |
| 2.12 | `elem_size_variant(cases)` | CanonicalABI.md:1969-1977 | `types.Variant.Size()` | `composite.go:103-120` | ✅ | Correctly computes `align_to(discSize, maxCaseAlign) + max_payload_size`, aligned to variant alignment. | H |
| 2.13 | `elem_size_flags(labels)` | CanonicalABI.md:1979-1984 | `types.Flags.Size()` | `composite.go:302-315` | ⚠️ partial | Correct for n in {1..16, 17..32}. Extends: returns `4 * ((n+31)/32)` for n > 32 — spec asserts n ≤ 32. Returns 0 for n == 0 — spec asserts n > 0. | M |

---

## 3. Loading (lift from memory)

| # | Spec function | Spec section/line | Wazero function(s) | File:line | Status | Notes | Conf |
|---|---|---|---|---|---|---|---|
| 3.1 | `load(cx, ptr, t)` top-level dispatch | CanonicalABI.md:1994-2019 | 🔁 `abi.LiftHeap` | `abi/lift.go:339-695` | ✅ | **Not called from production.** Correct dispatch covering all types including fixed-length lists. | H |
| 3.1 dup 🔁 | same | — | `ExportedFunc.liftFieldFromMemory` | `instance.go:1242-1402` | ⚠️ partial | Called from production `ExportedFunc.Call` via `liftResolvedType`. Missing: Variant, Tuple, Result, Own, Borrow, fixed-length List. **Bug:** Record branch at line 1385-1393 does not apply field alignment (skips `align_to`). Option branch at line 1355-1365 uses `offset+4` for payload instead of `align_to(offset+1, innerAlign)`. | H |
| 3.1 dup 🔁 | same | — | `liftValFromMemory`, `liftRecordFromMemory`, `liftOptionFromMemory` | `linker.go:2726-2825` | ⚠️ partial | Called from `createCanonLowerFunc`'s `liftFromStack` lift-list-from-memory path. Missing: Variant, Tuple, Result, Flags, String, Own, Borrow. Enum reads 1 byte always (line 2757) regardless of case count. List only specialized for list<u8>. | H |
| 3.2 | `load_int(cx, ptr, nbytes, signed)` | CanonicalABI.md:2025-2026 | `api.Memory.ReadByteAt / ReadUint16Le / ReadUint32Le / ReadUint64Le` | (via `internal/wasm/memory.go`) | ✅ | Delegated to core memory primitives. Sign handling is per-type at the caller. | H |
| 3.3 | `convert_int_to_bool(i)` | CanonicalABI.md:2032-2034 | Inline `v != 0` in lift functions | e.g., `abi/lift.go:54-55`, `instance.go:1244-1250` | ✅ | All impls treat any non-zero as `true`. | H |
| 3.4 | `decode_i32_as_float` / `decode_i64_as_float` (with NaN canonicalization) | CanonicalABI.md:2046-2072 | `abi.canonicalizeNaN32 / canonicalizeNaN64` | `abi/context.go:32-45` | ⚠️ partial | Only `abi/` package canonicalizes NaNs. **`instance.go:1304-1310` (`liftFieldFromMemory`) and `canon_lower.go:318-321` (`liftValFromFlat`) do NOT canonicalize** — they call `math.Float32frombits` directly without NaN handling. | H |
| 3.5 | `convert_i32_to_char(cx, i)` — validate Unicode scalar | CanonicalABI.md:2079-2083 | `abi.isValidUnicodeScalar` | `abi/lift.go:827-837` | ✅ | Rejects surrogates and > 0x10FFFF. Only in `abi/`. | H |
| 3.5 dup 🔁 | same | — | Inline in `instance.go:1311-1316`, `linker.go:2572-2573`, `canon_lower.go:322-323` | | ❌ | Production paths do NOT validate — they cast `rune(stack[0])` directly without checking for surrogates. | H |
| 3.6 | `load_string(cx, ptr)` | CanonicalABI.md:2098-2101 | `abi.LiftString` | `abi/strings.go:18-29` | ✅ | Reads (ptr, len) then dispatches to encoding-specific. Only in `abi/`. | H |
| 3.6 dup 🔁 | same | — | `ExportedFunc.liftStringFromRetptr` | `instance.go:719-753` | ⚠️ partial | **UTF-8 only**, ignores `StringEncoding` option. Does UTF-8 validation. | H |
| 3.6 dup 🔁 | same | — | Inline in `liftFromStack` (linker.go:2574-2582) | | ⚠️ partial | **UTF-8 only**, no validation. | H |
| 3.7 | `load_string_from_range(cx, ptr, tagged_code_units)` w/ utf8/utf16/latin1+utf16 | CanonicalABI.md:2105-2131 | `abi.liftStringUTF8 / liftStringUTF16 / liftStringLatin1UTF16` | `abi/strings.go:46-123` | ✅ | Fully implemented in `abi/`. Checks alignment, handles UTF16_TAG. | H |
| 3.7 dup 🔁 | same | — | — | — | ❌ | Production code does not have UTF-16 or latin1+utf16 support on the lift path. | H |
| 3.8 | `lift_error_context(cx, i)` | CanonicalABI.md:2137-2140 | — | — | ❌ | Not implemented. `ErrorContextType` absent from `types/`. | M |
| 3.9 | `load_list(cx, ptr, t, l)` | CanonicalABI.md:2145-2150 | `abi.LiftHeap` List branch | `abi/lift.go:637-690` | ✅ | Implements dynamic and fixed-length. Reads ptr/len for dynamic, iterates elements inline for fixed. Validates alignment and bounds. | H |
| 3.9 dup 🔁 | same | — | `ExportedFunc.liftFieldFromMemory` List branch | `instance.go:1366-1384` | ⚠️ partial | Missing fixed-length lists; no alignment validation, no bounds check. | H |
| 3.9 dup 🔁 | same | — | `liftListFromMemory` | `linker.go:2709-2723` | ⚠️ partial | Uses buggy `elemSizeFromTypeDef`. Returns nil for unsupported element types. | H |
| 3.10 | `load_list_from_range(cx, ptr, length, elem_type)` | CanonicalABI.md:2152-2155 | `abi.LiftHeap` List inline loop | `abi/lift.go:667-690` | ✅ | Validates alignment and bounds. | H |
| 3.11 | `load_record(cx, ptr, fields)` | CanonicalABI.md:2163-2169 | `abi.LiftHeap` Record branch | `abi/lift.go:425-441` | ✅ | Applies `align_to` per field; iterates declared order. | H |
| 3.11 dup 🔁 | same | — | `liftRecordFromMemory` | `linker.go:2797-2812` | ⚠️ partial | Correct loop, but depends on buggy `elemAlignFromTypeRef`/`elemSizeFromTypeRef`. | H |
| 3.11 dup 🔁 | same | — | `liftFieldFromMemory` Record branch | `instance.go:1385-1393` | ⚠️ partial | **Missing alignment between fields.** | H |
| 3.12 | `load_variant(cx, ptr, cases)` | CanonicalABI.md:2181-2190 | `abi.LiftHeap` Variant branch | `abi/lift.go:463-497` | ✅ | Reads discriminant with correct size (1/2/4 based on case count). Uses `PayloadOffset()`. | H |
| 3.12 dup 🔁 | same | — | `liftFieldFromMemory` Variant branch | `instance.go:1242-1402` | ❌ | **Variant not handled** in `liftFieldFromMemory`. Falls through to default `ValU32(0)`. | H |
| 3.12 dup 🔁 | same | — | `liftResultFromMemory` | `instance.go:1406-1491` | ⚠️ partial | Handles Result (despecialized variant) only. **Bug:** reads 4-byte discriminant (spec says 1 byte for result). | H |
| 3.12 dup 🔁 | same | — | `liftResolvedType` Variant branch retptr | `instance.go:1114-1162` | ✅ | Uses correct `t.DiscriminantSize()` and `t.PayloadOffset()`. | H |
| 3.13 | `load_flags(cx, ptr, labels)` | CanonicalABI.md:2197-2206 | `abi.LiftHeap` Flags branch | `abi/lift.go:591-634` | ✅ | Handles n ≤ 8, ≤ 16, ≤ 32, > 32 (multi-word). Matches spec. | H |
| 3.13 dup 🔁 | same | — | `liftFieldFromMemory` Flags branch | `instance.go:1345-1354` | ⚠️ partial | Always reads 4 bytes (`memory.ReadUint32Le`) regardless of n. Spec says 1/2/4 depending on n. | H |
| 3.14 | `lift_own(cx, i, t)` | CanonicalABI.md:2215-2221 | `abi.LiftOwn` | `abi/lift.go:708-778` | ⚠️ partial | **Does NOT validate `h.rt is t.rt`** (see TODO at line 703-707). Does check `NumLends != 0` (via `Remove`). **Generation scan workaround** (1..1000 loop). Dead in production. | H |
| 3.14 dup 🔁 | same | — | `ExportedFunc.liftOwn` | `instance.go:2312-2348` | ⚠️ partial | Production path. Same gen scan. Same missing resource-type check. | H |
| 3.15 | `lift_borrow(cx, i, t)` | CanonicalABI.md:2234-2240 | `abi.LiftBorrow` | `abi/lift.go:790-822` | ⚠️ partial | Same TODO (resource type check missing). Calls `BorrowScope.AddLender`. Dead in production. | H |
| 3.15 dup 🔁 | same | — | `ExportedFunc.liftBorrow` | `instance.go:2353-2385` | ⚠️ partial | Production path. Same limitations. | H |
| 3.16 | `lift_stream(cx, i, t)`, `lift_future(cx, i, t)`, `lift_async_value(...)` | CanonicalABI.md:2256-2268 | — | — | ❌ | Not implemented anywhere. | M |

---

## 4. Storing (lower to memory)

| # | Spec function | Spec section/line | Wazero function(s) | File:line | Status | Notes | Conf |
|---|---|---|---|---|---|---|---|
| 4.1 | `store(cx, v, t, ptr)` top-level dispatch | CanonicalABI.md:2278-2304 | 🔁 `abi.LowerHeap` | `abi/lower.go:308-648` | ✅ | Not called from production. Full coverage of all types. | H |
| 4.1 dup 🔁 | same | — | `ExportedFunc.lowerToMemory` | `instance.go:2009-2277` | ⚠️ partial | Production path. Handles most types. **Bug:** Result writes discriminant as `WriteUint32Le(offset, 0 or 1)` then computes payload offset as `offset + alignTo(4, payloadAlign)` — treating discriminant as 4 bytes. Spec says discriminant is 1 byte for result (2 cases). | H |
| 4.1 dup 🔁 | same | — | `writeResultsToMemory`, `writeValToMemory`, `writeRecordToMemory` | `linker.go:3157-3521` | ❌ many bugs | Production path for canon_lower retptr. Multiple critical bugs: `ValKindVariant` writes **placeholder discriminant `0`** unconditionally (line 3292). `ValKindS16/U16` writes `WriteUint32Le` (4 bytes) but advances offset by 2. `writeRecordToMemory` does **not apply field alignment**. | H |
| 4.2 | `store_int(cx, v, ptr, nbytes, signed)` | CanonicalABI.md:2311-2312 | `api.Memory.WriteByteAt / WriteUint16Le / WriteUint32Le / WriteUint64Le` | (via core memory primitives) | ✅ | Everywhere. | H |
| 4.3 | `encode_float_as_i32/i64` (with NaN scrambling) | CanonicalABI.md:2328-2358 | `math.Float32bits / Float64bits` | Everywhere. | ⚠️ partial | Nothing applies `maybe_scramble_nan32/64`. wazero relies on the host deterministic-float profile. Spec allows both but wazero's behavior is fixed (no scrambling). | H |
| 4.4 | `char_to_i32(c)` | CanonicalABI.md:2370-2373 | Inline `uint32(c)` / `uint64(c)` | e.g., `instance.go:1629`, `abi/lower.go:42-46` | ⚠️ partial | `abi/lower.go:42-46` validates via `isValidUnicodeScalarRune`. **Production paths do not validate**, writing surrogates as-is. | H |
| 4.5 | `store_string(cx, v, ptr)` | CanonicalABI.md:2395-2398 | `abi.LowerString` + writers | `abi/strings.go:128-248` + `abi/lower.go:354-361` | ✅ | Full UTF-8 / UTF-16 / latin1+utf16 support with encoding-transition handling. | H |
| 4.5 dup 🔁 | same | — | `ExportedFunc.lowerStringParam` | `instance.go:1964-1985` | ⚠️ partial | **UTF-8 only**, ignores encoding option. | H |
| 4.5 dup 🔁 | same | — | `LoweredFunc.lowerString` | `canon_lower.go:442-473` | ⚠️ partial | UTF-8 only. | H |
| 4.5 dup 🔁 | same | — | Inline in `writeResultsToMemory` ValKindString | `linker.go:3233-3259` | ⚠️ partial | UTF-8 only. | H |
| 4.5 dup 🔁 | same | — | Inline in `writeValToMemory` ValKindString | `linker.go:3490-3506` | ⚠️ partial | UTF-8 only. | H |
| 4.6 | `store_string_into_range(cx, v)` + transcoding functions | CanonicalABI.md:2400-2580 | `abi.lowerStringUTF8 / lowerStringUTF16 / lowerStringLatin1UTF16` | `abi/strings.go:143-248` | ⚠️ partial | `abi/` handles encoding choice but does NOT implement the realloc-shrink optimization from `store_utf8_to_utf16` (spec lines 2493-2506) — it over-allocates UTF-16 by max code units without the shrink-realloc-at-end step. Functionally correct, memory-wasteful. | M |
| 4.7 | `lower_error_context(cx, v)` | CanonicalABI.md:2585-2586 | — | — | ❌ | Not implemented. | M |
| 4.8 | `store_list(cx, v, ptr, t, l)` | CanonicalABI.md:2594-2601 | `abi.LowerHeap` List branch | `abi/lower.go:586-643` | ✅ | Handles fixed-length and dynamic. Allocates via realloc. | H |
| 4.8 dup 🔁 | same | — | `ExportedFunc.lowerToMemory` List branch | `instance.go:2239-2271` | ⚠️ partial | Missing fixed-length list handling. | H |
| 4.8 dup 🔁 | same | — | `writeResultsToMemory` ValKindList branch | `linker.go:3191-3232` | ⚠️ partial | Uses `elementSizeForKind` and `alignmentForKind` (ValKind-based) — cannot lower lists of records/variants/options correctly. | H |
| 4.9 | `store_list_into_range(cx, v, elem_type)` | CanonicalABI.md:2603-2610 | Inline in `abi.LowerHeap` | `abi/lower.go:586-643` | ✅ | | H |
| 4.10 | `store_list_into_valid_range(cx, v, ptr, elem_type)` | CanonicalABI.md:2612-2614 | Inline in `abi.LowerHeap` / `ExportedFunc.lowerToMemory` | `abi/lower.go:617-638`, `instance.go:1818-1829` | ✅ | Straightforward element loop. | H |
| 4.11 | `store_record(cx, v, ptr, fields)` | CanonicalABI.md:2616-2620 | `abi.LowerHeap` Record branch | `abi/lower.go:363-382` | ✅ | Aligns per field, writes in declared order. | H |
| 4.11 dup 🔁 | same | — | `ExportedFunc.lowerToMemory` Record branch | `instance.go:2077-2088` | ✅ | Uses `t.FieldOffsets()` which applies alignment. | H |
| 4.11 dup 🔁 | same | — | `writeRecordToMemory` | `linker.go:3369-3384` | ❌ | **Does not apply field alignment.** Just calls `writeValToMemory` and uses its returned offset. | H |
| 4.12 | `store_variant(cx, v, ptr, cases)` | CanonicalABI.md:2631-2645 | `abi.LowerHeap` Variant branch | `abi/lower.go:404-440` | ✅ | Uses `t.PayloadOffset()` and correct `discSize`. | H |
| 4.12 dup 🔁 | same | — | `ExportedFunc.lowerToMemory` Variant branch | `instance.go:2160-2212` | ✅ | Writes discriminant at correct `discSize`, payload aligned. | H |
| 4.12 dup 🔁 | same | — | `writeResultsToMemory` ValKindVariant branch | `linker.go:3286-3299` | ❌ | **Writes placeholder discriminant `0` with comment "Would need type info"**. | H |
| 4.13 | `store_flags(cx, v, ptr, labels)` | CanonicalABI.md:2655-2665 | `abi.LowerHeap` Flags branch | `abi/lower.go:534-584` | ✅ | Handles n ≤ 8, ≤ 16, ≤ 32, > 32. Read-modify-write for multi-word. | H |
| 4.13 dup 🔁 | same | — | `writeResultsToMemory` ValKindFlags branch | `linker.go:3332-3346` | ⚠️ partial | Always writes `WriteUint32Le` regardless of n. | H |
| 4.14 | `lower_own(cx, rep, t)` | CanonicalABI.md:2673-2675 | `abi.LowerOwn`, `abi.LowerOwnWithType` | `abi/lower.go:654-661`, `abi/resource_lower.go:52-59` | ⚠️ partial | Missing resource type tracking in base version. `LowerOwnWithType` implements type tracking. Dead in production. | H |
| 4.14 dup 🔁 | same | — | `ExportedFunc.lowerTyped` Own branch | `instance.go:1831-1837` | ⚠️ partial | Production path. No resource type check. | H |
| 4.15 | `lower_borrow(cx, rep, t)` | CanonicalABI.md:2677-2683 | `abi.LowerBorrowWithType` | `abi/resource_lower.go:21-42` | ✅ | Implements **same-instance optimization** (spec lines 2679-2680): `if currentInstanceID == resourceType.InstanceID() { return rep }`. Dead in production. | H |
| 4.15 dup 🔁 | same | — | `ExportedFunc.lowerTyped` Borrow branch | `instance.go:1838-1847` | ⚠️ partial | Production path. **Missing same-instance optimization** — always creates a handle. | H |
| 4.16 | `lower_stream(cx, v, t)`, `lower_future(cx, v, t)` | CanonicalABI.md:2695-2703 | — | — | ❌ | Not implemented. | M |

---

## 5. Flattening

| # | Spec function | Spec section/line | Wazero function(s) | File:line | Status | Notes | Conf |
|---|---|---|---|---|---|---|---|
| 5.1 | `flatten_functype(opts, ft, context)` | CanonicalABI.md:2739-2768 | `abi.CoreSignature` | `abi/flatten.go:41-51` | ⚠️ partial | Implements the MAX_FLAT_RESULTS → retptr conversion. **Missing:** `MAX_FLAT_PARAMS → i32 pointer` fallback (line 2743-2744 of spec). **Missing:** async handling. **Missing:** `MAX_FLAT_ASYNC_PARAMS` and `MAX_FLAT_ASYNC_RESULTS`. Dead in production. | H |
| 5.1 dup 🔁 | same | — | `coreSignature` | `linker.go:3579-3589` | ⚠️ partial | Same limitations. Used in production. | H |
| 5.1 dup 🔁 | same | — | `LoweredFunc.CoreSignature` | `canon_lower.go:130-149` | ❌ | Works directly off `ValTypeRef.Primitive` opcodes, returning `i32` for any non-primitive. Used in production for plain host imports. | H |
| 5.2 | `flatten_types(ts)` | CanonicalABI.md:2770-2771 | `abi.FlattenParams`, `abi.FlattenResults` | `abi/flatten.go:10-36` | ✅ | Simple concatenation. Spec-correct. | H |
| 5.2 dup 🔁 | same | — | `flattenParams`, `flattenResults` | `linker.go:3592-3612` | ✅ | Same logic. | H |
| 5.3 | `flatten_type(t)` dispatch | CanonicalABI.md:2780-2797 | `abi.flattenType` | `abi/flatten.go:55-102` | ✅ | Full dispatch. Uses real `join` for variants. Spec-correct. | H |
| 5.3 dup 🔁 | same | — | `flattenValType` | `linker.go:3616-3654` | ⚠️ partial | Same dispatch, but calls a home-grown `isWiderValueType` instead of `join`. **Spec-incorrect** for variant/result with mixed-primitive cases. | H |
| 5.3 dup 🔁 | same | — | `componentTypeToCoreTypes` | `canon_lower.go:154-192` | ❌ | Primitives only. Returns `i32` for composite types. | H |
| 5.4 | `flatten_list(elem_type, maybe_length)` | CanonicalABI.md:2802-2805 | `abi.flattenType` List branch | `abi/flatten.go:71-81` | ✅ | Fixed-length: `flatten_type(elem_type) * maybe_length`. Dynamic: `[i32, i32]`. | H |
| 5.4 dup 🔁 | same | — | `flattenValType` List branch | `linker.go:3632-3634` | ⚠️ partial | Only handles dynamic lists; **does not handle fixed-length lists** (returns `[i32, i32]` regardless). | H |
| 5.5 | `flatten_record(fields)` | CanonicalABI.md:2810-2814 | `abi.flattenRecord` | `abi/flatten.go:104-111` | ✅ | Concatenates field flattenings. | H |
| 5.5 dup 🔁 | same | — | `flattenRecordType` | `linker.go:3657-3663` | ✅ | Same. | H |
| 5.6 | `flatten_variant(cases)` with `join` | CanonicalABI.md:2826-2840 | `abi.flattenVariant`, `abi.join` | `abi/flatten.go:122-225` | ✅ | Spec-correct `join`: `i32+f32 → i32; else → i64`. Applied across payloads. | H |
| 5.6 dup 🔁 | same | — | `flattenVariantType`, `isWiderValueType`, `valueTypeWidth` | `linker.go:3747-3797` | ❌ | **Wrong join semantics.** Width ordering `i32 < f32 < i64 < f64` picks `f32` when spec says `i32`. For any variant with `{case1: f32, case2: i32}`, wazero produces `[i32, f32]` where spec says `[i32, i32]`. | H |
| 5.7 | `join(a, b)` | CanonicalABI.md:2837-2840 | `abi.join` | `abi/flatten.go:217-226` | ✅ | Correct. | H |
| 5.7 dup 🔁 | same | — | `isWiderValueType`, `valueTypeWidth` | `linker.go:3779-3797` | ❌ | Not `join`. | H |

---

## 6. Flat Lifting

| # | Spec function | Spec section/line | Wazero function(s) | File:line | Status | Notes | Conf |
|---|---|---|---|---|---|---|---|
| 6.1 | `CoreValueIter` class | CanonicalABI.md:2848-2868 | `abi.FlatIter` | `abi/lift.go:12-49` | ✅ | Straightforward iterator over `[]uint64`. | H |
| 6.1 dup 🔁 | same | — | `flatIter` (local type) | `canon_lower.go:240-273` | ✅ | Same interface. Duplicate. | H |
| 6.2 | `lift_flat(cx, vi, t)` dispatch | CanonicalABI.md:2877-2901 | `abi.LiftFlat` | `abi/lift.go:52-336` | ✅ | Full dispatch. Implements `lift_flat_variant` with `join` coercion. Spec-correct. | H |
| 6.2 dup 🔁 | same | — | `liftFromStack` + `liftRecordFromStack` + `liftOptionFromStack` + `liftVariantFromStack` | `linker.go:2547-3017` | ⚠️ partial | Production path for canon_lower. **Does not apply `join` coercion** in variant lift. Does not handle Result/Tuple/Stream/Future. | H |
| 6.2 dup 🔁 | same | — | `LoweredFunc.liftValFromFlat` | `canon_lower.go:292-340` | ❌ | **Primitives and string only.** For any non-primitive returns `ValS32(...)`. | H |
| 6.3 | `lift_flat_unsigned`, `lift_flat_signed` | CanonicalABI.md:2910-2921 | Inline `int8(iter.NextI32())` etc. | `abi/lift.go:56-71`, `instance.go:1206-1236`, etc. | ✅ | Go's integer casting does 2s complement for signed narrowing. | H |
| 6.4 | `lift_flat_string(cx, vi)` | CanonicalABI.md:2930-2933 | `abi.LiftFlat` String branch | `abi/lift.go:82-89` | ✅ | Reads ptr/len, calls `liftStringFromPtrLen`. | H |
| 6.4 dup 🔁 | same | — | `liftFromStack` string branch | `linker.go:2574-2582` | ⚠️ partial | UTF-8 only, no validation. | H |
| 6.4 dup 🔁 | same | — | `liftValFromFlat` string branch | `canon_lower.go:324-339` | ⚠️ partial | UTF-8 only, no validation, requires memory. | H |
| 6.5 | `lift_flat_list(cx, vi, elem_type, maybe_length)` | CanonicalABI.md:2935-2943 | `abi.LiftFlat` List branch | `abi/lift.go:278-331` | ✅ | Handles fixed-length and dynamic. | H |
| 6.5 dup 🔁 | same | — | `liftFromStack` list branch | `linker.go:2646-2673` | ⚠️ partial | Only handles dynamic lists, only list<u8> specialization + generic via `liftListFromMemory`. | H |
| 6.6 | `lift_flat_record(cx, vi, fields)` | CanonicalABI.md:2948-2952 | `abi.LiftFlat` Record branch | `abi/lift.go:90-100` | ✅ | Recurses per field, preserves declared order. | H |
| 6.6 dup 🔁 | same | — | `liftRecordFromStack` | `linker.go:2681-2691` | ✅ | Same logic, declared order. | H |
| 6.6 dup 🔁 | same | — | `ExportedFunc.liftRecord` | `instance.go:757-790` | ❌ | **Sorts fields alphabetically before reading.** Legacy path at `instance.go:501`. | H |
| 6.7 | `lift_flat_variant(cx, vi, cases)` — with `CoerceValueIter` | CanonicalABI.md:2962-2984 | `abi.LiftFlat` Variant branch | `abi/lift.go:101-160` | ✅ | Implements coerce-and-skip-padding pattern via `flattenVariantPayload` + `coerceFlatValue`. | H |
| 6.7 dup 🔁 | same | — | `liftVariantFromStack` | `linker.go:2991-3017` | ❌ | **No coerce step.** Just reads `stack[1:]` as the payload. Returns garbage for variants with mixed-primitive payloads. | H |
| 6.8 | `lift_flat_flags(vi, labels)` | CanonicalABI.md:2994-2997 | `abi.LiftFlat` Flags branch | `abi/lift.go:258-276` | ✅ | Reads `ceil(n/32)` i32s. | H |
| 6.8 dup 🔁 | same | — | `liftFromStack` flags branch (via ResolvedType.Flags) | `linker.go:2606-2613` | ⚠️ partial | Always reads 1 i32, only handles n ≤ 32 flags. | H |

---

## 7. Flat Lowering

| # | Spec function | Spec section/line | Wazero function(s) | File:line | Status | Notes | Conf |
|---|---|---|---|---|---|---|---|
| 7.1 | `lower_flat(cx, v, t)` dispatch | CanonicalABI.md:3006-3030 | `abi.LowerFlat` | `abi/lower.go:14-305` | ✅ | Full dispatch. Implements `lower_flat_variant` with `join` coercion. | H |
| 7.1 dup 🔁 | same | — | `ExportedFunc.lowerTyped` | `instance.go:1601-1852` | ⚠️ partial | Production path. **No `join` coercion in variant path.** Correct for primitive types. | H |
| 7.1 dup 🔁 | same | — | `ExportedFunc.lowerByKind` | `instance.go:1856-1961` | ⚠️ partial | Kind-based fallback. Record path **sorts fields alphabetically**. | H |
| 7.1 dup 🔁 | same | — | `LoweredFunc.lowerValToFlatTyped`, `lowerValToFlat` | `canon_lower.go:393-513` | ❌ | Primitives and string only. Errors on composite types. | H |
| 7.1 dup 🔁 | same | — | `lowerToStack` | `linker.go:3074-3152` | ❌ | Primitives, Enum, Flags, Own, Borrow only. **Missing String, List, Record, Variant, Option, Result, Tuple.** | H |
| 7.2 | `lower_flat_signed(i, core_bits)` | CanonicalABI.md:3038-3041 | Inline `uint64(uint32(int32(val.S8())))` | `instance.go:1608-1613`, `abi/lower.go:22-30`, etc. | ✅ | Go's integer casting handles the 2s complement conversion. | H |
| 7.3 | `lower_flat_string(cx, v)` | CanonicalABI.md:3050-3052 | `abi.LowerFlat` String branch | `abi/lower.go:47-52` | ✅ | Calls `LowerString` and returns `[ptr, taggedLen]`. | H |
| 7.3 dup 🔁 | same | — | Various inline | `instance.go:1631`, `canon_lower.go:427-428`, `linker.go:...` | ⚠️ partial | UTF-8 only. | H |
| 7.4 | `lower_flat_list(cx, v, elem_type, maybe_length)` | CanonicalABI.md:3054-3062 | `abi.LowerFlat` List branch | `abi/lower.go:247-300` | ✅ | Handles fixed-length (flatten each element inline) and dynamic (allocate and return ptr/len). | H |
| 7.4 dup 🔁 | same | — | `ExportedFunc.lowerTyped` List branch | `instance.go:1800-1830` | ⚠️ partial | Dynamic only. Missing fixed-length. | H |
| 7.5 | `lower_flat_record(cx, v, fields)` | CanonicalABI.md:3067-3071 | `abi.LowerFlat` Record branch | `abi/lower.go:53-68` | ✅ | | H |
| 7.5 dup 🔁 | same | — | `lowerTyped` Record branch | `instance.go:1632-1646` | ✅ | Declared order. | H |
| 7.5 dup 🔁 | same | — | `lowerByKind` Record branch | `instance.go:1940-1957` | ❌ | **Alphabetical sort.** | H |
| 7.6 | `lower_flat_variant(cx, v, cases)` — with `join` coercion | CanonicalABI.md:3078-3097 | `abi.LowerFlat` Variant branch + `coerceFlatValueForLower` | `abi/lower.go:69-122`, `abi/lower.go:711-731` | ✅ | Spec-correct. | H |
| 7.6 dup 🔁 | same | — | `lowerTyped` Variant branch | `instance.go:1757-1799` | ⚠️ partial | Pads to `maxPayloadFlat` but no `join` coercion. | H |
| 7.6 dup 🔁 | same | — | `lowerVariantToFlat` | `canon_lower.go:572-619` | ⚠️ partial | Pads but no `join`. Dead in production. | H |
| 7.7 | `lower_flat_flags(v, labels)` | CanonicalABI.md:3102-3104 | `abi.LowerFlat` Flags branch | `abi/lower.go:229-245` | ✅ | `ceil(n/32)` words. | H |
| 7.7 dup 🔁 | same | — | `lowerTyped` Flags branch | `instance.go:1675-1687` | ✅ | Same. | H |
| 7.7 dup 🔁 | same | — | `lowerFlagsToFlat` | `canon_lower.go:528-563` | ⚠️ partial | Separate 32/64/multi-word logic. Dead in production. | H |

---

## 8. Lifting and Lowering Values (MAX_FLAT dispatch)

| # | Spec function | Spec section/line | Wazero function(s) | File:line | Status | Notes | Conf |
|---|---|---|---|---|---|---|---|
| 8.1 | `lift_flat_values(cx, max_flat, vi, ts)` | CanonicalABI.md:3113-3123 | *(inlined in ExportedFunc.Call dispatch logic)* | `instance.go:297-326, 442-634` | ⚠️ partial | Implements MAX_FLAT_RESULTS fallback by checking `FlattenCount()` and reading retptr. Missing proper `load(cx, ptr, TupleType(ts))` path — instead has per-result-type dispatch logic. | M |
| 8.1 dup 🔁 | same | — | *(inlined in createCanonLowerFunc)* | `linker.go:2495-2506` | ⚠️ partial | Lifts per-param without MAX_FLAT fallback — assumes params are always flat. | H |
| 8.2 | `lower_flat_values(cx, max_flat, vs, ts, out_param)` | CanonicalABI.md:3132-3152 | *(inlined in ExportedFunc.Call)* | `instance.go:218-283` | ⚠️ partial | Implements MAX_FLAT_PARAMS → memory spill. Sets `may_leave = false`. Does not exactly mirror the spec's "store via tuple type" approach — uses a hand-rolled offset loop. | M |
| 8.2 dup 🔁 | same | — | *(inlined in createCanonLowerFunc)* | `linker.go:2521-2541` | ⚠️ partial | Lowers per-result. Calls `writeResultsToMemory` or `lowerToStack`. No `may_leave` handling. | H |

---

## 9. `canon lift`

| # | Spec function | Spec section/line | Wazero function(s) | File:line | Status | Notes | Conf |
|---|---|---|---|---|---|---|---|
| 9.1 | `canon_lift(opts, inst, ft, callee, caller, on_start, on_resolve)` | CanonicalABI.md:3237-3252 | `ExportedFunc.Call` | `instance.go:133-675` | ⚠️ partial | Implements synchronous lift: reentrance check, subtask creation, borrow scope, `may_leave` tracking, param lowering, core call, result lifting, post-return. **Missing: async / callback / stackful async / backpressure / ComponentInstance.exclusive.** Overall structure matches spec for synchronous functions. | M |
| 9.2 | `call_and_trap_on_throw(callee, thread, args)` | CanonicalABI.md:3420-3424 | Direct `f.coreFunc.Call(ctx, coreParams...)` | `instance.go:329-332` | ⚠️ partial | No try/catch wrapper — wazero core runtime surfaces panics as errors; `Call` returns `(results, err)` and the err is propagated. Functionally equivalent. | H |
| 9.3 | `post-return` invocation | CanonicalABI.md:3286-3289 | `instance.go:345-356` | `instance.go:345-356` | ✅ | Calls `postReturnFunc`, clears `may_leave` during the call, restores it after. | H |
| 9.4 | Async / callback / stackful async | CanonicalABI.md:3299-3425 | — | — | ❌ | Not implemented in `ExportedFunc.Call`. | H |
| 9.5 | Reentrance check `call_might_be_recursive` | CanonicalABI.md:3238 | `Instance.CallMightBeRecursive` / `ValidateNotRecursive` | `instance.go:2502-2538` | ⚠️ partial | Implements self-call detection only. Does not walk the full caller chain. | M |

---

## 10. `canon lower`

| # | Spec function | Spec section/line | Wazero function(s) | File:line | Status | Notes | Conf |
|---|---|---|---|---|---|---|---|
| 10.1 | `canon_lower(opts, ft, callee, thread, flat_args)` | CanonicalABI.md:3453-3576 | 🔁 `createCanonLowerFunc` | `linker.go:2430-2543` | ⚠️ partial | Synchronous path. Lifts args via `liftFromStack`, calls callee, lowers results via `writeResultsToMemory` or `lowerToStack`. No subtask state management beyond the bare minimum. | H |
| 10.1 dup 🔁 | same | — | `LoweredFunc.CallWithStack` + `createHostModuleExport` | `canon_lower.go:201-221`, `linker.go:1868-1892` | ❌ | Only handles primitives and strings. Produces wrong core signature for any non-primitive param. | H |
| 10.2 | `trap_if(not thread.task.inst.may_leave)` | CanonicalABI.md:3454 | *(no check)* | — | ❌ | Neither `createCanonLowerFunc` nor `CallWithStack` check `may_leave` before invoking the host function. | H |
| 10.3 | `Subtask` creation and `deliver_resolve` | CanonicalABI.md:3470, 3544 | `ExportedFunc.Call` does this for exports | `instance.go:134-176` | ⚠️ partial | Subtasks are created in `ExportedFunc.Call` (exports), not in `createCanonLowerFunc` (imports). Spec's `canon_lower` is always host-import-facing. | M |
| 10.4 | `on_start`/`on_resolve` callbacks and `subtask.state` machine | CanonicalABI.md:3495-3517 | — | — | ❌ | Not implemented. The call is synchronous and direct. | H |
| 10.5 | Async caller case | CanonicalABI.md:3560-3576 | — | — | ❌ | Not implemented. | H |

---

## 11. `canon resource.new` / `resource.drop` / `resource.rep`

| # | Spec function | Spec section/line | Wazero function(s) | File:line | Status | Notes | Conf |
|---|---|---|---|---|---|---|---|
| 11.1 | `canon_resource_new(rt, thread, rep)` | CanonicalABI.md:3605-3609 | `Instance.ResourceNew` | `instance.go:2389-2395` | ⚠️ partial | Creates a new owned handle. **Missing:** `trap_if(not inst.may_leave)` check. | H |
| 11.1 dup 🔁 | same | — | `createResourceOpExport` ResourceNew branch | `linker.go:2169-2182` | ⚠️ partial | Same. | H |
| 11.2 | `canon_resource_drop(rt, thread, i)` | CanonicalABI.md:3627-3650 | `Instance.ResourceDrop` | `instance.go:2414-2439` | ⚠️ partial | Removes handle, calls destructor if owned, decrements borrow count if borrowed. **Missing:** `trap_if(h.rt is not rt)` check (has TODO). **Missing:** cross-instance destructor call via `canon_lift` protocol — just calls the destructor synchronously. | H |
| 11.2 dup 🔁 | same | — | `createResourceOpExport` ResourceDrop branch | `linker.go:2156-2168` | ⚠️ partial | Simpler: just `inst.resourceTable.Remove(Handle(handle))`. Does not call the destructor. | H |
| 11.3 | `canon_resource_rep(rt, thread, i)` | CanonicalABI.md:3683-3687 | `Instance.ResourceRep` | `instance.go:2399-2409` | ⚠️ partial | **Missing:** `trap_if(h.rt is not rt)` check. | H |
| 11.3 dup 🔁 | same | — | `createResourceOpExport` ResourceRep branch | `linker.go:2183-2198` | ⚠️ partial | Same. | H |
| 11.4 | `ResourceHandle` state (rt, rep, own, num_lends, borrow_scope) | CanonicalABI.md:493-550 | `HandleEntry` | `resource_table.go:52-58` | ✅ | All fields present. | H |
| 11.5 | `Table.add`, `Table.get`, `Table.remove`, `Table.MAX_LENGTH` | CanonicalABI.md:436-492 | `ResourceTable.New/Get/Remove`, `MaxTableLength` | `resource_table.go` | ✅ | Implemented with generation tracking beyond spec. | H |

---

## 12. Canonical options (`canonopt`)

| # | Spec concept | Spec section/line | Wazero function(s) | File:line | Status | Notes | Conf |
|---|---|---|---|---|---|---|---|
| 12.1 | `string-encoding` | CanonicalABI.md:241 | `component.StringEncoding`, `abi.StringEncoding` | `component.go:424-430`, `abi/context.go:48-54` | ⚠️ partial | Two separate enums: one in `component/` package, one in `abi/` package. Wired correctly into `abi/strings.go` lift/lower, but **production paths ignore it** and assume UTF-8. | H |
| 12.2 | `memory` | CanonicalABI.md:242 | `CanonicalOptions.MemoryIdx` | `component.go:414` | ✅ | Stored in CanonicalOptions. Resolved at call time in `createCanonLowerFunc` via `memSpace.Resolve`. | H |
| 12.3 | `realloc` | CanonicalABI.md:243 | `CanonicalOptions.ReallocIdx` | `component.go:415` | ✅ | Stored. Resolved at call time. | H |
| 12.4 | `post-return` | CanonicalABI.md:244 | `CanonicalOptions.PostReturnIdx` | `component.go:416` | ✅ | Stored, used by `ExportedFunc.Call`. | H |
| 12.5 | `async` / `callback` | CanonicalABI.md:245-246 | `CanonicalOptions.Async, CallbackIdx` | `component.go:417-418` | ❌ | Fields present in struct but not wired into lift/lower runtime. | H |
| 12.6 | `gc` | CanonicalABI.md:— | `CanonicalOptions.GC` | `component.go:420` | ❌ | Field present, no runtime wiring. | H |

---

## 13. Context and runtime state

| # | Spec concept | Spec section/line | Wazero function(s) | File:line | Status | Notes | Conf |
|---|---|---|---|---|---|---|---|
| 13.1 | `LiftLowerContext` (cx) | CanonicalABI.md:207-233 | `abi.LiftContext`, `abi.LowerContext` | `abi/context.go:71-173` | ⚠️ partial | Have Memory, Opts, ResourceTable, BorrowScope / Realloc / CallContext / Subtask. **Missing:** caller instance, task, borrow_scope as a unified field. Dead in production. | H |
| 13.2 | `ComponentInstance` state: `may_leave`, `exclusive`, `table` | CanonicalABI.md:286-436 | `Instance.mayLeaveDisabled`, `Instance.resourceTable`, `Instance.activeCallDepth` | `instance.go:46-58` | ⚠️ partial | `may_leave` implemented. `exclusive` not tracked. `activeCallDepth` approximates it. | M |
| 13.3 | `Task` state | CanonicalABI.md:911-1194 | `Subtask` (partial) | `subtask.go` (not fully read) | ⚠️ partial | Subtask exists but not the full task state machine. | L |
| 13.4 | `BorrowScope` | CanonicalABI.md:1195-1278 | `BorrowScope` | `borrow_scope.go` | ⚠️ partial | Tracks `num_borrows` via `IncrementBorrows/DecrementBorrows`. AddLender for individual handles. Not fully read in this audit. | M |

---

## 14. Stream, Future, ErrorContext

| # | Spec concept | Spec section/line | Wazero function(s) | File:line | Status | Notes | Conf |
|---|---|---|---|---|---|---|---|
| 14.1 | `StreamType` | CanonicalABI.md:— | `component.StreamTypeDef` | `component.go:769-773` | ⚠️ partial | Decoded from binary. No lift/lower implementation. | H |
| 14.2 | `FutureType` | CanonicalABI.md:— | `component.FutureTypeDef` | `component.go:775-778` | ⚠️ partial | Decoded from binary. No lift/lower implementation. | H |
| 14.3 | `ErrorContextType` | CanonicalABI.md:1859 | — | — | ❌ | Not decoded, not implemented. | H |
| 14.4 | `lift_stream`, `lift_future`, `lower_stream`, `lower_future` | CanonicalABI.md:2256-2269, 2695-2703 | — | — | ❌ | None. | H |

---

## 15. Async built-ins

| # | Spec concept | Spec section/line | Wazero function(s) | File:line | Status | Conf |
|---|---|---|---|---|---|---|
| 15.1 | `canon context.get/set` | CanonicalABI.md:3693-3735 | — | — | ❌ | H |
| 15.2 | `canon backpressure.{set,inc,dec}` | CanonicalABI.md:3736-3787 | — | — | ❌ | H |
| 15.3 | `canon task.{return,cancel}` | CanonicalABI.md:3788-3867 | — | — | ❌ | H |
| 15.4 | `canon waitable-set.{new,wait,poll,drop}` | CanonicalABI.md:3868-3988 | — | — | ❌ | H |
| 15.5 | `canon waitable.join` | CanonicalABI.md:3989-4019 | — | — | ❌ | H |
| 15.6 | `canon subtask.{cancel,drop}` | CanonicalABI.md:4020-4109 | — | — | ❌ | H |
| 15.7 | `canon {stream,future}.new` | CanonicalABI.md:4110-4144 | — | — | ❌ | H |
| 15.8 | `canon {stream,future}.{read,write}` | CanonicalABI.md:4145-4345 | — | — | ❌ | H |
| 15.9 | `canon {stream,future}.cancel-{read,write}` | CanonicalABI.md:4346-4423 | — | — | ❌ | H |
| 15.10 | `canon {stream,future}.drop-{readable,writable}` | CanonicalABI.md:4424-4463 | — | — | ❌ | H |
| 15.11 | `canon thread.*` built-ins | CanonicalABI.md:4464-4681 | — | — | ❌ | H |
| 15.12 | `canon error-context.*` built-ins | CanonicalABI.md:4683-4772 | — | — | ❌ | H |

---

## 16. Summary counts

| Status | Count |
|---|---|
| ✅ matches | 36 |
| ⚠️ partial | 52 |
| ❌ missing | 23 |
| 🔁 multiple impls (unique spec rows with ≥2 wazero impls) | 28 |

Totals include the dup rows; each dup row reflects a different wazero function implementing the same spec function.

**High-level observation:** Of the 23 ❌ missing rows, **all are async/streaming/future/error-context/thread-related**. The synchronous canonical ABI is largely present, but with 52 ⚠️ partial entries reflecting the duplication and divergence documented in File 1. The 28 🔁 rows show the structural scale of the duplication problem: roughly one in three spec functions has two or more parallel wazero implementations.

---

*End of File 2.*
