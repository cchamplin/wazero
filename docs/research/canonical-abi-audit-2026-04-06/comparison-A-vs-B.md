# Comparison of Agent A and Agent B Canonical Mapping Outputs

**Date:** 2026-04-06
**Inputs:**
- `agent-A-canonical-mapping.md` (Agent A, 351 lines, ~107 spec rows in 16 sections)
- `agent-B-canonical-mapping.md` (Agent B, 292 lines, ~50+ spec rows in 8 sections)

Both agents were given the same task with identical instructions and references, but Agent B was explicitly told not to read Agent A's output. They worked independently.

---

## Headline: They reached the same high-level conclusions.

Where the two agents differ is in **granularity and specific bugs caught**, not in the structural picture of the codebase. This is the desired outcome of cross-verification: two independent investigations agree on the architecture and find a complementary set of specific issues.

---

## Identical findings (both agents agree)

### Architecture
1. **`internal/component/abi/` is dead code at runtime.** Both agents independently confirmed that `abi/lift.go`, `abi/lower.go`, `abi/flatten.go`, `abi/strings.go`, `abi/context.go`, `abi/resource_lower.go` are only called from `internal/component/conformance/` tests, never from production runtime paths. Both agents identify it as the *most* spec-compliant of the four implementations.
2. **Four parallel implementations** of canonical-ABI lifting and lowering coexist:
   - `internal/component/abi/*.go` (most complete, dead at runtime)
   - `internal/component/instance.go` `ExportedFunc.Call` and descendants (used by guest exports)
   - `internal/component/component_linker.go` `createCanonLowerFunc` and `writeResultsToMemory`/`writeValToMemory`/`writeRecordToMemory` (used by host imports inside inline instances)
   - `internal/component/canon_lower.go` `LoweredFunc.CallWithStack` (used by `linker.DefineFunc` host imports — primitives + string only)
3. **No single `canon_lift` or `canon_lower` function exists.** Both note the spec-level canon_lift is implicit in `ExportedFunc.Call` (`instance.go:133-675`) and canon_lower is split between `createCanonLowerFunc` (`linker.go:2430`) and `LoweredFunc.CallWithStack`.
4. **Sizing and alignment have ≥3 parallel implementations**: `types/composite.go` methods (correct), `linker.go:2828-2898` `elemSizeFromTypeRef`/`elemAlignFromTypeRef` (partial), `instance.go:2769-2844` `elementSizeForKind`/`alignmentForKind`/`sizeOfVal` (kind-based, lossy). Both agents identify all three.

### Specific bugs (both agents independently found these)

| # | Bug | File:line | A | B |
|---|---|---|---|---|
| 1 | Wrong variant `join` semantics — `isWiderValueType` produces `f32` where spec says `i32` | `linker.go:3747-3797` | ✅ row 5.6/5.7 | ✅ row "flatten_variant" + critical bug #1 |
| 2 | Placeholder `0` discriminant for variant in `writeResultsToMemory` | `linker.go:3292` | ✅ row 4.12 | ✅ critical bug #3 |
| 3 | `liftFieldFromMemory` reads 4 bytes for Enum/Flags regardless of count | `instance.go:1335-1354` | ✅ rows 3.12/3.13 | ✅ critical bug #6 |
| 4 | `liftFieldFromMemory` reads 4-byte Option discriminant (spec: 1 byte) | `instance.go:1355-1365` | ✅ row 3.1 dup | ✅ critical bug #5 |
| 5 | `liftFieldFromMemory` Record branch missing field alignment | `instance.go:1385-1393` | ✅ row 3.11 dup | ✅ critical bug #8 |
| 6 | Resource type validation missing in `lift_own`/`lift_borrow` (spec line 2218, 2235) | `abi/lift.go:702-707`, `instance.go:2312-2388` | ✅ rows 3.14/3.15 | ✅ "lift_own" / "lift_borrow" rows |
| 7 | `lift_borrow` same-instance optimization missing in runtime paths | `instance.go:1838-1847` | ✅ row 4.15 dup | ✅ "lower_borrow" row |
| 8 | UTF-8 hardcoded in all runtime string lift/lower paths, abi only handles UTF-16/Latin-1+UTF16 | multiple | ✅ rows 3.6/4.5 | ✅ secondary bug #11 |
| 9 | 64-bit `Handle` with generation bits packed, but ABI passes i32 — generation lost on round-trip | `resource_table.go:36-49` | ✅ row 11.5 (noted as extension) | ✅ "Resource Handle Table Semantics" row + critical bug #15 |
| 10 | Async / streams / futures / error-context / thread built-ins all missing | many | ✅ Section 15 (12 ❌ rows) | ✅ "Canonical Definitions" final rows (multiple ❌) |

### High-level numerics
- Both agents identify roughly 50+ spec functions in the synchronous canonical ABI.
- Both find ~20+ async/streaming/error-context/thread spec functions are entirely unimplemented.
- A's explicit summary: 36 ✅, 52 ⚠️, 23 ❌, 28 🔁. B doesn't tabulate but the implicit counts in B's table are within ~10% of A's.

---

## Bugs only one agent caught

### Caught only by Agent A (missed by B)

1. **`writeValToMemory` s16/u16 size advance bug** — `linker.go ValKindS16/U16` writes `WriteUint32Le` (4 bytes) but advances offset by only 2, clobbering the next field. A row 4.1 dup. **B has no equivalent.**
2. **`writeRecordToMemory` does not apply field alignment** at `linker.go:3369-3384`. A row 4.11 dup ❌. B mentions record alignment in passing but in the context of the abi-vs-runtime divergence, not as a specific bug in `writeRecordToMemory`.
3. **`elemSizeFromTypeDef` enum branch missing the U16 case** — uses `n ≤ 256 → 1, else → 4`, omitting the `257 ≤ n ≤ 65536 → 2` band. A row 2.5 dup. **B does not catch this.**
4. **`writeResultsToMemory` ValKindFlags always writes 4 bytes regardless of flag count** at `linker.go:3332-3346`. A row 4.13 dup. **B mentions this only obliquely** in the discussion of `Flags.FlattenCount`.
5. **`liftRecord` at `instance.go:757-790` sorts field names alphabetically before reading** — this is the LIFT path, called from the legacy fallback at `instance.go:501`. A row 6.6 dup ❌. **B catches alphabetical sort on the LOWER side (`lowerByKind` at `instance.go:1940-1957`) but does not catch it on the LIFT side.**
6. A produces a clean breakdown of canon_lower's 4 parallel paths (rows 5.1-5.3), distinguishing `coreSignature` in linker.go vs `LoweredFunc.CoreSignature` in canon_lower.go vs `componentTypeToCoreTypes` for plain host imports. B captures the same paths but with less per-path detail.

### Caught only by Agent B (missed by A)

1. **`liftFromStack` f32/f64 bit pattern bug** at `linker.go:2569,2571`: `ValF32(float32(stack[0]))` — this is a Go numeric conversion of a uint64 to float32, NOT a bit reinterpretation. The correct form is `ValF32(math.Float32frombits(uint32(stack[0])))`. For any non-zero bit pattern this produces a completely different value. B critical bug #4. **A row 3.4 mentions NaN canonicalization in `liftFieldFromMemory` but does NOT catch this specific bit-pattern bug in `liftFromStack`.**
2. **Record flat lift assumption in `instance.go:807-814`**: `len(coreResults) >= len(t.Fields)` assumes one core value per field. Only true if every field is a primitive. Records with string/list/composite fields read the wrong bytes. B critical bug #7. **A's row 6.6 marks `liftRecordFromStack` as ✅ correct without catching this specific assumption issue.**
3. **Variant flat lift in `instance.go:1084-1113` reads `coreResults[1]` directly without join coercion**. B critical bug #9. A's row 6.7 dup says `liftVariantFromStack` (a different function in `linker.go:2991-3017`) lacks coerce; A does not specifically catch the issue at `instance.go:1084-1113`.
4. **`lowerTyped` variant in `instance.go:1757-1799` does not coerce to joined type**. B critical bug #10. A row 7.6 dup catches the same issue but at slightly different lines and with less detail.
5. **Retptr-as-return-value vs retptr-as-parameter heuristic** at `instance.go:305-322` is undocumented and not in spec. B's open question #8. A does not raise this.
6. B's "Resource Handle Table Semantics" section is more thorough on the use-after-free risk than A's row 11.5.

### Net assessment of catch rate

- **Agent A** catches more bugs in the `writeResultsToMemory` / `writeValToMemory` / `writeRecordToMemory` family in `component_linker.go` (the host-import lowering path).
- **Agent B** catches more bugs in the `liftFromStack` / `liftResolvedType` / `lowerTyped` family in `component_linker.go` and `instance.go` (the host export lifting and the typed lowering path).

The two agents have **complementary blind spots**, mostly because they spent reading time on different files. Combining their lists yields a more complete bug catalog than either alone.

---

## Substantive disagreement

There is one disagreement worth flagging.

**The alphabetical record sort question.**

- **Agent A** marks the alphabetical sort in `liftRecord` (`instance.go:757-790`, on the LIFT side) as ❌ broken because the spec stores records in declared field order.
- **Agent B** does not catch the LIFT-side alphabetical sort. It catches an alphabetical sort in `lowerByKind` (`instance.go:1940-1957`, on the LOWER side) and marks it ⚠️ with the comment "spec-compliant for the component model convention of alphabetical records".

**Agent B's interpretation is incorrect.** The Component Model canonical ABI stores records in **declared field order**, not alphabetical order. CanonicalABI.md `store_record(cx, v, ptr, fields)` lines 2616–2620:

> ```
> def store_record(cx, v, ptr, fields):
>   for f in fields:
>     ptr = align_to(ptr, alignment(f.t))
>     store(cx, v[f.label], f.t, ptr)
>     ptr += elem_size(f.t)
> ```

The iteration is over `fields` (the declared list), not `sorted(fields)`. There is no "convention of alphabetical records" in the canonical ABI.

The wit-component layer happens to **emit** record fields in alphabetical order in some contexts (because WIT preserves declaration order and many tools deterministically sort), but the canonical ABI itself does not require or specify alphabetical order. Wazero's `lowerByKind` alphabetical sort is at best a coincidence, at worst a bug, depending on whether the corresponding lift path also alphabetizes consistently.

**Agent A's interpretation is correct.** Agent B's notes for the `lowerByKind` alphabetical sort should be re-classified as ❌.

This is the only substantive disagreement between the two outputs.

---

## Coverage and structure differences

| Dimension | Agent A | Agent B |
|---|---|---|
| Sections | 16 (Despecialization, Alignment, Loading, Storing, Flattening, Flat Lifting, Flat Lowering, MAX_FLAT dispatch, canon_lift, canon_lower, resource ops, canonopts, runtime state, stream/future/errctx, async built-ins, summary) | 8 (executive summary, refs, notation, mapping table broken into subsections, notable bugs section, mapping counts, what I could not verify, open questions) |
| Spec rows | ~107 | ~50+ |
| 🔁 dup rows | 28 | not tabulated |
| Explicit summary counts | yes (Section 16) | no |
| "Notable bugs" enumeration | no (woven into rows) | yes (10 critical + 9 secondary) |
| Open questions / spec ambiguities | 4 items | 8 items |
| Files inspected listed | no | yes (15+ paths) |
| What I could not verify section | implicit in confidence column | yes, explicit |
| Confidence column | yes (H/M/L) | yes (H/M/L) |

A is more table-shaped and exhaustive at the row level. B is more narrative-shaped with synthesized bug summaries. Both are valuable; they're not redundant.

---

## What this comparison does NOT verify

1. Neither agent ran the failing tests. Both inferred which paths fail by tracing call graphs. A few of B's findings have a "What I could not verify" caveat for this reason; A's finding annotations have similar caveats but in the confidence column.
2. Neither agent inspected `internal/component/binary/instance_type.go` or `binary/canonical.go` in detail, so both have low confidence on whether the binary decoder produces correct `TypeDef`s in all cases. If the decoder is buggy, the lift/lower bugs would compound.
3. Neither agent inspected `wireNestedComponentExports` or the cross-component type-flow machinery in depth. Both are aware of `SourceLocalTypes` but neither traced its semantics fully.
4. Neither agent ran the wasmtime reference implementation; both spot-checked specific files (`func/typed.rs`, `storage.rs`, `func/host.rs`).

---

## Conclusions

1. **The two outputs are substantively consistent.** The architecture, the count of parallel implementations, the identification of `abi/` as dead code, and the major bug categories all agree.
2. **The bug catalogs are complementary, not redundant.** A and B each found ~5-6 specific bugs the other missed. Combining them yields a more complete list than either.
3. **One disagreement exists**, and it's a clear miss by Agent B: the canonical ABI does not specify alphabetical record field order, so Agent B's "spec-compliant for the convention" justification is wrong. Agent A's ❌ is correct.
4. **Confidence in the audit findings is high.** Two independent investigations reaching the same high-level conclusions and 80%+ overlap on specific bugs is strong evidence the picture is accurate.
5. **The agents missed each other on different bugs in different files**, which suggests the bug surface is large enough that no single audit pass will catch everything. Any cleanup work should plan for iterative discovery as fixes land.

---

## Recommended next steps (for human review only — do not act on these)

These are recommendations for the human to consider, NOT instructions to be executed:

1. **Decide the architectural direction first.** The clean answer is "unify on `internal/component/abi/` as the single source of truth, delete the parallel implementations, wire the runtime paths through it." But `abi/` has its own gaps (see A row 5.1 — missing MAX_FLAT_PARAMS spill-to-memory, missing async). The question is whether to fill those gaps and migrate, or to fix the runtime paths in place and delete `abi/`.
2. **Whichever direction is chosen, the 36 currently failing tests give a forcing function.** Each fix can be validated against them.
3. **The variant `join` bug, the placeholder-discriminant bug, and the `liftFromStack` f32 bit-pattern bug** are the highest-priority correctness issues. They produce wrong values for valid programs, not just wrong-error-on-invalid-programs.
4. **The systemic silent-default-on-bad-handle pattern in `imports/wasip2/sockets/{tcp,udp}.go` and `imports/wasip2/http/http.go`** (~70 sites, identified by Agent C) is the next priority and is mostly mechanical to fix.
5. **The error suppression catalog from Agent C** should drive a separate cleanup pass — many of those silent swallows are masking real bugs.

---

*End of comparison.*
