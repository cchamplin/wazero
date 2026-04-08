# Canonical-ABI Session 1 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Date:** 2026-04-08
**Status:** Ready for execution
**Scope reference:** `docs/superpowers/specs/2026-04-08-canonical-abi-session1-design.md` (commit `13049148`, 2201 lines)
**Branch:** `feat/wasip2-complete-implementation`
**HEAD at planning time:** `13049148`
**Previous session plan:** `docs/superpowers/plans/2026-04-07-canonical-abi-type-unification-session0-plan.md` (format reference only)

**Goal:** Wire `abi/` into the production `ComponentLinker.Instantiate` pipeline, rebuild `component.Instance` on top of `*runtime.ComponentInstance`, bind component-local resource declarations to runtime pointer-identity `*ResourceType`, fix four latent correctness gaps in `abi/lift.go`, expose `Component.TypeDefs []TypeDef` as the single decoder→linker index, change `DefineFunc` to require a typed `*types.TypeFunc`, and restore 223 skipped tests + 29 conformance stubs to spec-correct state with upstream citations on every restored test.

**Architecture:** Six ordered checkpoints A → F, each with a bash success criterion. Every numbered task follows test-first TDD where applicable (failing test → run → fail → implement → run → pass → review), otherwise the housekeeping pattern (implement → build → review). Every task dispatches `superpowers:code-reviewer` plus a spec-compliance review subagent with the Session 1 amended checklist (Decision 9).

**Tech Stack:** Go 1.22+, wazero `internal/component/...`, canonical-ABI reference `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py` + `run_tests.py`, wasmtime reference `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/...` + `debug-vendored/wasmtime/crates/wasmtime/src/runtime/vm/component/...`.

---

## Source of Truth

**Single authoritative design document:**
`docs/superpowers/specs/2026-04-08-canonical-abi-session1-design.md`

Every code shape, struct field, function signature, dispatch arm, deletion target, and checkpoint criterion in this plan is anchored to a section or Decision in that design doc. The design doc text takes precedence over any text in this plan if they conflict. If such a conflict is discovered during execution, stop and resolve it from the design doc.

**Forbidden sources during execution.** Executor agents MUST NOT consult any other wazero design, spec, or followup document in the `docs/` tree except these two:
1. `docs/superpowers/specs/2026-04-08-canonical-abi-session1-design.md` (the authoritative Session 1 design).
2. `docs/plans/2026-04-07-canonical-abi-unification-session0-followup.md` (background only — its internal contradictions have been resolved in the Session 1 design).

The prior Session 0 design doc (`2026-04-07-canonical-abi-type-unification-design.md`) is **not** authoritative for Session 1. Decisions 1-9 from Session 0 are final and MUST NOT be re-litigated. The Session 1 design doc references Session 0 decisions inline where needed; no out-of-band reading is required.

**Spec authorities cited from the design doc (consulted for test expectations and production code shapes):**

| File | Role |
|---|---|
| `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py` | canonical-ABI reference implementation. Authoritative for step-by-step semantics of every lift/lower/canon operation. |
| `debug-vendored/component-model/design/mvp/canonical-abi/run_tests.py` | canonical-ABI reference test harness. Authoritative for observable behavior of canon_* functions. |
| `debug-vendored/component-model/design/mvp/CanonicalABI.md` | spec prose. Authoritative for rationale and invariants. |
| `debug-vendored/component-model/design/mvp/Explainer.md` | component-model explainer. Instance width subtyping at `:920-982`. |
| `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/func.rs` | wasmtime `Func::call` / `call_impl` / `call_raw` exported call flow (lines 232-706), post-return (737-837). |
| `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/func/options.rs` | wasmtime `LiftContext` / `LowerContext` analog. |
| `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/instance.rs` | wasmtime `Instantiator` at line 710, `Instantiator::new` at 743, `extract_resource` at 920-930. |
| `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/matching.rs` | wasmtime import type matching. `matching.rs:51` bails on `None` actual; `:162` recursive instance walk. |
| `debug-vendored/wasmtime/crates/wasmtime/src/runtime/vm/component.rs` | wasmtime `ComponentInstance` at 93-159 (aggregating-map — cross-reference only; wazero adopts single-layer per Session 0 Decision 6). |
| `debug-vendored/wasmtime/crates/wasmtime/src/runtime/vm/component/resources.rs` | `resource_lift_own` at 275-279, `resource_lift_borrow` at 291-297, `resource_new` at 218-221, `enter_call`/`exit_call` borrow scope at 324-346. |
| `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/resources/ty.rs` | `ResourceType::guest` at 68-79. |
| `debug-vendored/wasmtime/crates/wasmtime/src/runtime/vm/component/concurrent_disabled.rs` | `may_enter` check at :159. |
| `debug-vendored/wasmtime/tests/all/component_model/` | wasmtime's component-model integration tests. Secondary to canonical sources; valuable for real-world scenarios. |

**Key spec line anchors** (re-verified at planning time against the vendored `definitions.py`):

| Spec anchor | File:line | What it defines |
|---|---|---|
| `class ComponentInstance` | `definitions.py:256-273` | spec instance shape with `may_leave`, `table`, `parent` |
| `call_might_be_recursive` | `definitions.py:290-299` | reflexive-ancestors reentrance check |
| `class Table` | `definitions.py:303-315` | unified handle table |
| `class ResourceHandle` | `definitions.py:337-349` | `rep: int` (u32 invariant) |
| `class ResourceType` | `definitions.py:351-361` | nominal identity |
| `lift_own` | `definitions.py:1333-1339` | `:1337 trap_if(h.num_lends != 0)`, `:1338 trap_if(not h.own)`, `:1339 return h.rep` |
| `lift_borrow` | `definitions.py:1341-1347` | `:1346 add_lender`, `:1347 return h.rep` |
| `lift_flat_values` | `definitions.py:1943-1952` | aggregate boundary decision (retptr vs flat) |
| `lower_flat_values` | `definitions.py:1954-1974` | `:1955 may_leave=False`, `:1973 may_leave=True` |
| `canon_lift` | `definitions.py:1978-2040` | `:1979 trap_if(call_might_be_recursive)`, post_return may_leave toggle |
| `canon_lower` | `definitions.py:2064-2130` | `:2065 trap_if(not may_leave)`, Subtask + borrow scope |
| `canon_resource_new` | `definitions.py:2134-2138` | may_leave check + table.add(own) |
| `canon_resource_drop` | `definitions.py:2142-2165` | type check, lends check, destructor dispatch |
| `canon_resource_rep` | `definitions.py:2169-2173` | type check, return h.rep |
| `deliver_resolve` (borrow scope) | `definitions.py:738-742` | `num_lends` cleanup |

Every task-level spec citation below comes from this anchor table. Line numbers were verified against the vendored `definitions.py` at HEAD `13049148` during planning. Re-grep before each edit if drift is suspected.

---

## Hard Constraints (carried from design doc)

1. **No parallel paths.** Edit existing files in place. Build may break between tasks inside a checkpoint; it MUST be green at the end of each checkpoint.
2. **No placeholders.** No `// TODO`, `// FIXME`, `// XXX`, `panic("not implemented")`, or `t.Skip("session 1 work")`. If a behavior is explicitly deferred per the design doc, the code traps with a precise error that cites the deferral.
3. **Correctness over preservation.** Where the design overrides existing wazero code, the plan implements the design. The pre-Session-0 `component_linker.go` is reference material, not template code.
4. **Session 0 decisions are final.** The 9 numbered Session 0 Design Decisions are not re-litigated.
5. **Upstream authority on tests.** Every restored test has an upstream citation block per the Test Restoration Methodology (design Decision 8). Tests asserting behavior contradicting spec/canonical/wasmtime are reworked or deleted.
6. **No subagent destructive git operations.** Corrective subagents MUST NOT run `git stash`, `git reset`, or `git checkout` of paths. Per-task working-tree integrity is verified after every dispatch.
7. **Per-task dual review.** Every task dispatches both `superpowers:code-reviewer` AND a spec-compliance review subagent. No substituting inline verification for a subagent dispatch.
8. **Line numbers drift.** Every `file.go:N` reference in the design doc was captured 2026-04-07. Re-grep before each edit.

---

## Preconditions

- **Branch:** `feat/wasip2-complete-implementation`
- **Working directory:** `/home/cchamplin/development/wazero`
- **HEAD at plan authoring:** `13049148` (`docs: revise Session 1 design from 10-chunk subagent review`)
- **Build-green check:**
  ```bash
  cd /home/cchamplin/development/wazero && go build ./... 2>&1 | head -40
  ```
  Expected: empty (the repo compiles) except in Session 0 broken-test spots.
- **Design doc present:**
  ```bash
  test -f docs/superpowers/specs/2026-04-08-canonical-abi-session1-design.md && echo OK
  ```
  Expected: `OK`.
- **Vendored references present:**
  ```bash
  test -f debug-vendored/component-model/design/mvp/canonical-abi/definitions.py && \
    test -f debug-vendored/component-model/design/mvp/canonical-abi/run_tests.py && \
    test -d debug-vendored/wasmtime/crates/wasmtime/src/runtime/component && echo OK
  ```
  Expected: `OK`.
- **No prior Session 1 work committed beyond design doc.** Confirm via:
  ```bash
  git log --oneline feat/wasip2-complete-implementation ^main | grep -i "session 1" | head
  ```
  Expected: `docs: revise Session 1 design from 10-chunk subagent review` and `docs: add canonical-ABI Session 1 design` and no implementation commits.

---

## Roadmap and Checkpoints

Six checkpoints A → F plus Final. Each checkpoint has a bash success criterion that any executor agent can run to verify the gate passed. Between checkpoints the plan dispatches per-checkpoint reviewers and fixes correctives before proceeding.

| Checkpoint | Scope | Success criterion |
|---|---|---|
| **A — `Component.TypeDefs` + decoder exposure** | Add `TypeDef` fields (`ResourceDtor*`), change `TypeDef.Func` to `types.FuncTypeIdx`, add `Component.TypeDefs []TypeDef`, populate in decoder, delete private decoder maps, restore 18 decoder tests in `binary/component_type_test.go` + `binary/instance_type_test.go`, plus one `component_test.go` decoder-produced-ComponentTypes test. | `go build ./internal/component/binary/... ./internal/component/... 2>&1` empty AND `go test ./internal/component/binary/... -count=1` green. |
| **B — `component.Instance` embeds `*runtime.ComponentInstance`** | Delete duplicated fields (`table`, `destructors`, `callContext`, `mayLeaveDisabled`, `activeCallDepth`), rewrite delegators with the `IsMayLeave` semantic fix and the transitive `CallMightBeRecursive` check via `ReentranceTracker`, update every call site in `internal/component/` + `imports/wasip2/`. | `go build ./internal/component/... ./imports/wasip2/... 2>&1` empty AND accessor tests in `instance_test.go` pass. |
| **C — `Instantiate` top-level + canon.lift/lower/resource wiring + primitive/composite/string/abi_edge conformance** | Delete Instantiate + coreSignature stubs, rebuild the 14-step Instantiate pipeline, wire canon.lift/lower/resource host module exports, add `abi.LowerParams` / `LiftParams` / `LowerResults` / `LiftResults`, change `DefineFunc` signature to require `*types.TypeFunc`, migrate every `imports/wasip2/` call site, restore linker tests + primitive/composite/string/abi_edge/post_return/flat_abi conformance tests, audit `primitives_test.go` + `may_leave_test.go` for missing citations. | `go test ./internal/component/conformance/... -run 'Primitives\|Composites\|Strings\|ABIEdge\|FlatABI\|PostReturn\|Linker' -count=1` green AND `go test ./internal/component/ -run 'Linker' -count=1` green. |
| **D — Nested components + resolveExportTypeAlias + integration tests** | Delete `nested_component.go` panic stub, rebuild `resolveExportTypeAlias`, `instantiateNestedComponent`, `wireNestedComponentExports`, `createInlineInstanceModule`, audit `reentrance_test.go` citations, restore `nested_component_test.go` (21 tests), `integration_test.go` (19 tests), `start_function_test.go` (9 tests), `component_linker_test.go` (8 tests), plus `conformance/nested_test.go` + `nested_instantiation_test.go` + `nesting_depth_test.go`. | `go test ./internal/component/ -run 'Nested\|Integration\|StartFunction\|ComponentLinker' -count=1` green AND `go test ./internal/component/conformance/ -run 'Nested\|NestingDepth\|Reentrance' -count=1` green. |
| **E — Resource type binding + 4 lift.go fixes + resource conformance** | Implement `bindResourceTypes`, add `Table.GetByIndex`, change `ResourceHandleEntry.Rep` from `any` to `uint32`, migrate `imports/wasip2/io/*` (and http/filesystem/sockets/clocks/cli) to per-module u32 registries, add `runtime.ResourceType.HostDestructor`, add `BorrowScope.ReleaseBorrow`, fix 4 lift.go gaps (Own, NumLends, GetByIndex, Rep-u32), rewrite `Instance.ResourceNew/Rep/Drop`, add `byteMemory` helper, restore 11 abi bounds-check tests, restore `conformance/resources_test.go` + `destructor_test.go` + `resource_generation_test.go` + `concurrent_access_test.go`, restore resource-related cases in `instance_test.go`. | `go test ./internal/component/conformance/ -run 'Resources\|Destructor\|ResourceGeneration\|ConcurrentAccess' -count=1` green AND `go test ./internal/component/abi/ -count=1` green AND `go test ./internal/component/ -run 'CanonResource\|InstanceResource' -count=1` green. |
| **F — All 223 tests + 29 conformance stubs green; type_checker fixes** | Fix `type_checker.go::checkFuncDefinition` + `checkInstanceDefinition`. Restore remaining `instance_test.go` (lift/lower), `type_checker_test.go` (17 tests), `edge_case_test.go`, `value_import_test.go`, `composite_test.go`, `instantiate_test.go`, `integration_public_api_test.go`, `integration_records_test.go`, remaining `conformance/*.go` stubs (error_messages, instance_types, memory_bounds, realloc_failure, type_edge_cases, utf_validation, `wasi_*` world tests). | `grep -rln 'session 1 work' internal/ api/ imports/` empty AND `grep -rln 'DeferredToSession1' internal/component/conformance/` empty AND `go test ./... -count=1` green except `conformance/subtask_test.go` (`t.Skip("later work: async lift/lower")`). |
| **Final** | Full repo suite, Session 2 followup note. | `go vet ./... && go test ./... -count=1` green AND `docs/plans/2026-04-08-canonical-abi-session1-followup.md` written. |

### Between-checkpoint reviews

Between each checkpoint A → F, the plan runs:
1. `superpowers:code-reviewer` over the checkpoint's task group (with explicit scope of the files touched).
2. Spec-compliance review subagent with the Session 1 amended checklist (Decision 9): citation coverage, citation verification, assertion cross-check, deduplication check, `run_tests.py` coverage audit.

Correctives identified by either reviewer land as inline follow-up tasks before the next checkpoint begins. Each checkpoint ends with a working-tree integrity verification:

```bash
git status --porcelain | head -20 && git diff --stat HEAD
```

Expected: only files from the checkpoint's manifest are touched; no stashes or resets occurred.

---

## Session 1 Spec-Review Checklist (amended from Session 0)

Every task's spec-compliance review subagent MUST perform these steps in order. This checklist is reproduced here (rather than referenced) so every executor agent has it at hand.

1. **Citation coverage.** Run the V4 grep script from the design doc on files touched by the task. Fail if any new `func Test...` lacks a citation comment block in the 10 lines above it.
2. **Citation verification.** For each citation in each restored test, open the cited file at the cited line number and read the surrounding context. Write a one-sentence confirmation in the review report: "Confirmed: definitions.py:1338 contains `trap_if(not h.own)` as cited in TestExportedFuncCall_OwnArgument."
3. **Assertion cross-check.** For each `require.*` / `assert.*` call in each restored test, cross-reference the assertion against the cited upstream and record whether it matches observable behavior. Flag any contradictions for rework-or-delete.
4. **Deduplication check.** Search the repo for any other test asserting the same behavior; if found, flag for consolidation.
5. **`run_tests.py` coverage audit.** For the category of behavior the test targets, list every `run_tests.py` case in that category and cross-check whether the restoration covers each case. Flag any missing cases.
6. **No-TODO scan.** Run `grep -n 'TODO\|FIXME\|XXX' <touched-files>`. Must return empty.
7. **No-stub scan.** Run `grep -n 'panic("compile-fix\|panic("not implemented\|session 1 work\|DeferredToSession1' <touched-files>`. Must return empty for production code and for restored tests.
8. **Spec-deferral audit.** Every `return nil, fmt.Errorf(...session 2...)` trap must cite its spec line and the design doc section that explicitly defers the behavior to Session 2.

---

# Tasks

The tasks below are organized by checkpoint. Within a checkpoint, tasks run in the numbered order. Each task follows the test-first TDD structure where applicable: **Step N.1 failing test** → **Step N.2 confirm failure** → **Step N.3 implementation** → **Step N.4 confirm pass** → **Step N.5 reviewers**. Tasks that are pure housekeeping (file manifest, deletions, signature migrations) use **Step N.1 implementation** → **Step N.2 build** → **Step N.3 reviewers**.

---

## Checkpoint A — `Component.TypeDefs` + decoder exposure

**Scope:** Add `TypeDef` fields, change `TypeDef.Func` from `*types.TypeFunc` to `types.FuncTypeIdx`, add `Component.TypeDefs []TypeDef`, populate in the binary decoder, delete `decodeContext.funcTypeIdx` + `decodeContext.resourceDefs` private maps, restore 18 decoder tests.

**Design references:** Decision 5 (design lines 382-448), Decoder → Linker Indirection section (lines 1229-1248), File Manifest (lines 1908-1912), Verification V5 + V9.

**Exit criterion (Checkpoint A gate):**
```bash
cd /home/cchamplin/development/wazero && \
  go build ./internal/component/binary/... ./internal/component/... 2>&1 | head -20 && \
  go test ./internal/component/binary/... -count=1 2>&1 | tail -20
```
Expected: build output empty, all 18 previously-skipped binary decoder tests pass.

---

### Task A1: Add `TypeDef` resource destructor fields + change `TypeDef.Func` to `types.FuncTypeIdx`

**Design reference:** Decision 5 option A (design lines 440-446); Resource Identity section (lines 1192-1210).
**Spec citation:** Spec `definitions.py:351-361` (ResourceType with `dtor`, `dtor_async`, `dtor_callback`). Wasmtime parallel: `runtime/component/instance.rs:912-931` (Instantiator::resource reads destructor metadata).
**Files modified:**
- `internal/component/component.go` — `TypeDef` struct fields change.
- Every caller of the old `TypeDef.Func *types.TypeFunc` in `internal/component/`.

- [ ] **Step A1.1: Write the failing test**

Create `internal/component/component_typedef_test.go` with:

```go
package component

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component/types"
)

// TestTypeDefFuncStoresIndex asserts TypeDef.Func is a types.FuncTypeIdx
// (not a *types.TypeFunc pointer), per Session 1 Decision 5 option A.
//
// Spec: definitions.py:88-101 (FuncType shape — function types are
// structural and interned in the canonical bag).
// Wasmtime parallel: crates/environ/src/component/types.rs (canonical bag
// uses indices for cross-type references to avoid dangling pointers).
func TestTypeDefFuncStoresIndex(t *testing.T) {
	td := TypeDef{
		Kind: TypeDefKindFunc,
		Func: types.FuncTypeIdx(5),
	}
	if td.Func != types.FuncTypeIdx(5) {
		t.Fatalf("TypeDef.Func = %v, want 5", td.Func)
	}
}

// TestTypeDefResourceDtorFields asserts TypeDef carries resource
// destructor metadata so bindResourceTypes can wire Dtor without a
// second pass over decoder state. Design lines 1192-1210.
//
// Spec: definitions.py:351-361 (ResourceType {dtor, dtor_async, dtor_callback}).
func TestTypeDefResourceDtorFields(t *testing.T) {
	dtorIdx := uint32(7)
	callbackIdx := uint32(9)
	td := TypeDef{
		Kind:                 TypeDefKindResource,
		Resource:             types.ResourceTableIdx(2),
		ResourceDtor:         &dtorIdx,
		ResourceDtorAsync:    true,
		ResourceDtorCallback: &callbackIdx,
	}
	if td.ResourceDtor == nil || *td.ResourceDtor != 7 {
		t.Fatalf("ResourceDtor = %v, want 7", td.ResourceDtor)
	}
	if !td.ResourceDtorAsync {
		t.Fatalf("ResourceDtorAsync = false, want true")
	}
	if td.ResourceDtorCallback == nil || *td.ResourceDtorCallback != 9 {
		t.Fatalf("ResourceDtorCallback = %v, want 9", td.ResourceDtorCallback)
	}
}
```

- [ ] **Step A1.2: Run the test to confirm it fails**

```bash
cd /home/cchamplin/development/wazero && \
  go test ./internal/component/ -run 'TestTypeDef(FuncStoresIndex|ResourceDtorFields)' -count=1 2>&1 | tail -20
```

Expected failure modes (one or both):
- `td.Func undefined (type TypeDef has no field or method Func)` (if the struct currently uses pointer) OR `cannot use types.FuncTypeIdx(5) (untyped int constant) as *types.TypeFunc`
- `unknown field ResourceDtor in struct literal of type TypeDef`
- `unknown field ResourceDtorAsync in struct literal of type TypeDef`
- `unknown field ResourceDtorCallback in struct literal of type TypeDef`

- [ ] **Step A1.3: Implement the production change**

Read `internal/component/component.go` around line 126 to see current `TypeDef` definition. Edit the struct to:

```go
// TypeDef is one entry per type-section slot in the binary, populated by
// the decoder. Every Kind-specific field is populated only when Kind
// matches; other fields are zero.
//
// Session 1 Decision 5 option A: TypeDef.Func is a types.FuncTypeIdx
// (not a *types.TypeFunc) so it remains stable across canonical bag
// growth. Callers that need the *types.TypeFunc do:
//
//     &c.Types.Funcs[td.Func]
//
// after the bag is finalized.
type TypeDef struct {
	Kind TypeDefKind

	// Func is the function-type index into Component.Types.Funcs when
	// Kind == TypeDefKindFunc.
	Func types.FuncTypeIdx

	// Resource is the resource-table index when Kind == TypeDefKindResource.
	Resource types.ResourceTableIdx

	// ValType is the value-type reference when Kind == TypeDefKindDefined.
	ValType types.ValType

	// Instance is the instance-type declaration when Kind == TypeDefKindInstance.
	Instance *InstanceTypeDef

	// Component is the component-type declaration when Kind == TypeDefKindComponent.
	Component *ComponentTypeDef

	// ResourceDtor, ResourceDtorAsync, ResourceDtorCallback carry the
	// destructor metadata the decoder extracts for TypeDefKindResource
	// slots. bindResourceTypes reads these at Instantiate time to
	// populate runtime.ResourceType fields. Spec: definitions.py:351-361.
	ResourceDtor         *uint32
	ResourceDtorAsync    bool
	ResourceDtorCallback *uint32
}
```

Callers of the old `TypeDef.Func *types.TypeFunc` field inside `internal/component/` must update to dereference via the canonical bag. Run:

```bash
grep -n 'TypeDef{' internal/component/*.go | grep -v _test.go
grep -rn '\.Func\b' internal/component/ | grep -i 'typedef\|td\.\|.\.Func\s*=' | grep -v _test.go
```

For every call site that assigns or reads `typedef.Func`, change:
- `td.Func = someTypeFuncPointer` → `td.Func = someFuncTypeIdx`
- `&c.Types.Funcs[td.Func]` (if caller needs a pointer) or `c.Types.Funcs[td.Func]` (if caller needs a value).
- Add a helper method if many callers need the pointer form:
  ```go
  // FuncType resolves TypeDef.Func to its canonical *TypeFunc.
  // Kind must be TypeDefKindFunc.
  func (td *TypeDef) FuncType(c *Component) *types.TypeFunc {
      return &c.Types.Funcs[td.Func]
  }
  ```

- [ ] **Step A1.4: Run the test to confirm it passes + build is green**

```bash
cd /home/cchamplin/development/wazero && \
  go build ./internal/component/... 2>&1 | head -40 && \
  go test ./internal/component/ -run 'TestTypeDef(FuncStoresIndex|ResourceDtorFields)' -count=1 2>&1 | tail -10
```

Expected:
- Build empty.
- Both tests PASS.

- [ ] **Step A1.5: Run per-task reviewers**

Dispatch `superpowers:code-reviewer` with scope `internal/component/component.go + internal/component/component_typedef_test.go + any caller updates`. Dispatch spec-compliance reviewer with the Session 1 amended checklist. Apply correctives before proceeding.

---

### Task A2: Add `Component.TypeDefs []TypeDef` field

**Design reference:** Decision 5 (design lines 382-411).
**Spec citation:** Not directly a spec field — it is an engineering convenience for decoder → linker type indirection. No upstream counterpart; justified by the decoder → linker contract described in the design doc at lines 1229-1248.
**Files modified:** `internal/component/component.go`.

- [ ] **Step A2.1: Write the failing test**

Append to `internal/component/component_typedef_test.go`:

```go
// TestComponentTypeDefsField asserts Component.TypeDefs exists as an
// accessible []TypeDef slice — the single source of truth for type-section
// slot → canonical-bag index resolution.
//
// No counterpart (justified): this is a wazero engineering convenience to
// carry per-slot type kind through the decoder → linker boundary. The
// spec's type section is a linear slot sequence; wazero models it as a
// slice alongside the canonical *types.ComponentTypes bag.
func TestComponentTypeDefsField(t *testing.T) {
	c := &Component{
		TypeDefs: []TypeDef{
			{Kind: TypeDefKindFunc, Func: types.FuncTypeIdx(0)},
			{Kind: TypeDefKindResource, Resource: types.ResourceTableIdx(0)},
		},
	}
	if len(c.TypeDefs) != 2 {
		t.Fatalf("len(c.TypeDefs) = %d, want 2", len(c.TypeDefs))
	}
	if c.TypeDefs[0].Kind != TypeDefKindFunc {
		t.Fatalf("TypeDefs[0].Kind = %v, want TypeDefKindFunc", c.TypeDefs[0].Kind)
	}
	if c.TypeDefs[1].Kind != TypeDefKindResource {
		t.Fatalf("TypeDefs[1].Kind = %v, want TypeDefKindResource", c.TypeDefs[1].Kind)
	}
}
```

- [ ] **Step A2.2: Run the test to confirm it fails**

```bash
go test ./internal/component/ -run TestComponentTypeDefsField -count=1 2>&1 | tail -10
```

Expected: `unknown field TypeDefs in struct literal of type Component`.

- [ ] **Step A2.3: Implement**

Edit `internal/component/component.go`. Locate the `Component` struct and append `TypeDefs []TypeDef` with a documentation comment:

```go
// TypeDefs is one entry per type-section slot in the binary, in the
// order the slots were decoded. Every caller that previously used
// CanonicalDef.TypeIdx / ImportExternDesc.TypeIdx / Export.TypeIdx /
// InstanceExport.TypeIdx resolves the raw type-section index through
// this slice: `slot := c.TypeDefs[canon.TypeIdx]` and switches on
// slot.Kind. The private decoder maps `funcTypeIdx` and `resourceDefs`
// are deleted in Task A4; Component.TypeDefs is the single source of
// truth.
//
// Session 1 design: Decision 5 (lines 382-448).
TypeDefs []TypeDef
```

- [ ] **Step A2.4: Run the test to confirm it passes**

```bash
go test ./internal/component/ -run TestComponentTypeDefsField -count=1 2>&1 | tail -5
```

Expected: PASS.

- [ ] **Step A2.5: Run per-task reviewers**

Dispatch both reviewers. Apply correctives.

---

### Task A3: Populate `Component.TypeDefs` in the binary decoder

**Design reference:** Decision 5 (design lines 412-447); Decoder → Linker Indirection (lines 1229-1248).
**Spec citation:** No direct spec line — this is decoder plumbing mapping each type-section slot to its canonical bag counterpart. Justified by `definitions.py` having a linear type-index space that the decoder must surface to the linker.
**Files modified:** `internal/component/binary/decoder.go`.

- [ ] **Step A3.1: Write the failing test**

Before writing the test, locate the one currently-skipped decoder test in `component_test.go`:

```bash
grep -n "session 1 work" internal/component/component_test.go
```

Expected: one hit for a test that validates `c.TypeDefs` is populated by the decoder.

Restore that test using the Test Restoration Methodology (Step 2 — git, Step 5 — citation block). Begin with:

```bash
git show 98b3bbc3:internal/component/component_test.go > /tmp/old_component_test.go
```

Then locate the test function in the old file whose body was skipped and port it. Example minimal restoration (adjust function name to match the existing skipped function in `component_test.go`):

```go
// TestDecoderPopulatesTypeDefs asserts the binary decoder produces a
// Component.TypeDefs slice in declaration order, with each entry's Kind
// + kind-specific field set to the decoder's canonical bag index.
//
// Spec: definitions.py section "Type Section" (type section slots are
// numbered in declaration order; no separate anchor needed beyond the
// general type system shape).
// No counterpart (justified): this is a wazero decoder contract, not a
// canonical-ABI observable — run_tests.py does not exercise the decoder.
func TestDecoderPopulatesTypeDefs(t *testing.T) {
	// Build a tiny component binary with: 1 funcType, 1 resource decl.
	// Use the binary-builder helpers already present in component_test.go
	// to assemble a binary. Decode it and assert:
	//   c.TypeDefs[0].Kind == TypeDefKindFunc
	//   c.TypeDefs[0].Func == FuncTypeIdx(0)
	//   c.TypeDefs[1].Kind == TypeDefKindResource
	//   c.TypeDefs[1].Resource == ResourceTableIdx(0)
	//   c.TypeDefs[1].ResourceDtor != nil (if the encoded binary sets a dtor)
	// (Test body uses the same binary-assembly pattern as other tests in
	// component_test.go — read the existing tests to get the pattern.)
}
```

Fill in the body using the binary assembly helpers visible in the rest of `component_test.go`.

- [ ] **Step A3.2: Run the test to confirm it fails**

```bash
go test ./internal/component/ -run TestDecoderPopulatesTypeDefs -count=1 2>&1 | tail -20
```

Expected: test panics or fails with `len(c.TypeDefs) = 0, want 2` — the decoder has not yet been modified to populate `TypeDefs`.

- [ ] **Step A3.3: Implement**

Read `internal/component/binary/decoder.go` around the `decodeTypeSection` call sites:

```bash
grep -n 'decodeTypeSection\|funcTypeIdx\|resourceDefs\|appendOther\|appendResource' internal/component/binary/decoder.go | head -40
```

Locate every `dc.funcTypeIdx[slot] = ...` and `dc.resourceDefs[slot] = ...` site and add (alongside, for now — the old maps are deleted in Task A4):

```go
// Append to Component.TypeDefs in the same slot order.
dc.c.TypeDefs = append(dc.c.TypeDefs, component.TypeDef{
	Kind: component.TypeDefKindFunc,
	Func: ftIdx,   // types.FuncTypeIdx
})
```

For each type-section opcode handled by `decodeTypeSection` or `decodeTypeDef`, emit exactly one `TypeDef` append per slot:

| Type opcode | `TypeDef.Kind` | kind-specific field |
|---|---|---|
| `TypeOpFuncSync` / `TypeOpFuncAsync` | `TypeDefKindFunc` | `Func: ftIdx` |
| `TypeOpResourceSync` / `TypeOpResourceAsync` | `TypeDefKindResource` | `Resource: resourceDef.ResourceTableIdx`, plus `ResourceDtor`, `ResourceDtorAsync`, `ResourceDtorCallback` from `resourceDef` |
| `TypeOpInstance` | `TypeDefKindInstance` | `Instance: &itd` |
| `TypeOpComponent` | `TypeDefKindComponent` | `Component: &ctd` |
| Any ValType opcode (`ValTypeOpBool`, `ValTypeOpRecord`, etc.) | `TypeDefKindDefined` | `ValType: decodedValType` |

Verify each append runs exactly once per slot by adding a local counter assertion: after `decodeTypeSection` returns, `len(dc.c.TypeDefs)` equals the number of decoded slots.

- [ ] **Step A3.4: Run the test to confirm it passes**

```bash
go build ./internal/component/binary/... ./internal/component/... 2>&1 | head -20 && \
  go test ./internal/component/ -run TestDecoderPopulatesTypeDefs -count=1 2>&1 | tail -5
```

Expected: build empty, test PASS.

- [ ] **Step A3.5: Run per-task reviewers**

Dispatch both reviewers. Confirm the reviewer walks `decodeTypeSection` for every opcode case and verifies a single `TypeDefs` append per case.

---

### Task A4: Delete `decodeContext.funcTypeIdx` + `decodeContext.resourceDefs` private maps

**Design reference:** Decision 5 (design lines 446-448); Decoder → Linker Indirection (lines 1229-1248).
**Spec citation:** None — housekeeping deletion driven by Decision 5's "single source of truth" rule.
**Files modified:** `internal/component/binary/decoder.go`, any callers of the two private maps.

- [ ] **Step A4.1: Audit current usage**

```bash
grep -rn 'funcTypeIdx\|resourceDefs\b' internal/component/binary/ 2>&1
```

Record every call site. Each call site will migrate to `dc.c.TypeDefs[slot]`.

- [ ] **Step A4.2: Migrate call sites**

For each hit, replace the map lookup with an index into `TypeDefs`:

- `dc.funcTypeIdx[slot]` → `dc.c.TypeDefs[slot].Func` (asserting `Kind == TypeDefKindFunc` where defensive).
- `dc.resourceDefs[slot]` → look up from `dc.c.TypeDefs[slot]` (with Kind assertion).

If any site needs the old pointer-form `*ResourceTypeDef` and the new `TypeDef` doesn't carry that exact shape, reconstruct the needed data from `TypeDef.Resource` + `TypeDef.ResourceDtor*` fields. Do NOT re-add the private map.

- [ ] **Step A4.3: Delete the struct fields and their initializers**

Remove `funcTypeIdx map[uint32]types.FuncTypeIdx` and `resourceDefs map[uint32]*ResourceTypeDef` from `decodeContext`. Remove their `make()` calls in the `decodeContext` constructor.

- [ ] **Step A4.4: Verify V5**

```bash
grep -rn 'funcTypeIdx\|resourceDefs' internal/component/binary/ 2>&1
```

Expected: empty (V5 passes).

- [ ] **Step A4.5: Build + run decoder tests**

```bash
go build ./internal/component/binary/... 2>&1 | head -20 && \
  go test ./internal/component/binary/... -count=1 2>&1 | tail -20
```

Expected: build empty, all existing (non-skipped) decoder tests still pass.

- [ ] **Step A4.6: Run per-task reviewers**

Dispatch both reviewers. Key spec-reviewer item: confirm no silent fallbacks were introduced during the migration (e.g., a map-get with a `comma-ok` that now reads past `len(TypeDefs)`).

---

### Task A5: Restore 8 `binary/component_type_test.go` tests

**Design reference:** Test Restoration Methodology (design lines 1580-1669); File Manifest (lines 1953).
**Spec citation:** `definitions.py` Type Section decoding (implicit — these tests validate the decoder's `*types.ComponentTypes` production).
**Files modified:** `internal/component/binary/component_type_test.go`.

- [ ] **Step A5.1: Locate the 8 skipped tests**

```bash
grep -n 't\.Skip.*session 1 work' internal/component/binary/component_type_test.go
```

Expected: 8 function-level skips. Record each function name.

- [ ] **Step A5.2: Pull pre-Session-0 bodies**

```bash
git show 98b3bbc3:internal/component/binary/component_type_test.go > /tmp/old_component_type_test.go
wc -l /tmp/old_component_type_test.go
```

- [ ] **Step A5.3: For each function, port the body with citation block**

For each of the 8 functions, perform these sub-steps (do NOT batch — port one at a time to keep the review surface small):

1. Read the old body from `/tmp/old_component_type_test.go`.
2. Perform the Step 1 deduplication grep from the methodology:
   ```bash
   grep -rn 'TestComponentType' internal/component/binary/ | head -30
   ```
3. Rewrite the body against the new types (`*types.ComponentTypes`, `Component.TypeDefs`). Mechanical translations:
   - `c.Types[idx]` (old `[]TypeDef` slice) → `c.TypeDefs[idx]`
   - `c.Types[idx].FuncType` → `c.TypeDefs[idx].FuncType(c)` (if the helper was added in Task A1)
   - `&FuncType{Params: [...]}` → `builder.InternFuncType(...)` using the canonical bag builder.
4. Add the citation block directly above the function. Format:
   ```go
   // TestXxx asserts the decoder produces the expected ComponentTypes bag
   // shape for the given binary input.
   //
   // Spec: definitions.py (Type Section decoding).
   // No counterpart (justified): canonical-abi run_tests.py does not
   // exercise the binary decoder; wazero's decoder is a wazero-specific
   // engineering artifact mapping the component binary format to the
   // canonical bag representation.
   func TestXxx(t *testing.T) { ... }
   ```
5. Delete the `t.Skip(session1SkipReason)` call.
6. Run the single test:
   ```bash
   go test ./internal/component/binary/ -run TestXxx -count=1 2>&1 | tail -10
   ```
7. If failing, diagnose: is the assertion wrong (fix the test) or is the production code wrong (drop back to Task A3 and add the missing decoder logic)? Do NOT leave the test skipped.

- [ ] **Step A5.4: Run the full file**

```bash
go test ./internal/component/binary/ -run TestComponentType -count=1 2>&1 | tail -20
```

Expected: all 8 restored tests PASS.

- [ ] **Step A5.5: Run per-task reviewers**

Dispatch both reviewers with scope `internal/component/binary/component_type_test.go`. Spec reviewer MUST apply V4 grep to the file and confirm every restored function has a citation block.

---

### Task A6: Restore 10 `binary/instance_type_test.go` tests

**Design reference:** Test Restoration Methodology; File Manifest (lines 1954).
**Spec citation:** `definitions.py` Instance Type section (implicit; instance type decoding is a wazero-specific decoder behavior).
**Files modified:** `internal/component/binary/instance_type_test.go`.

- [ ] **Step A6.1: Locate the 10 skipped tests**

```bash
grep -n 't\.Skip.*session 1 work' internal/component/binary/instance_type_test.go
```

Expected: 10 hits.

- [ ] **Step A6.2: Port pre-Session-0 bodies**

Apply the same sub-steps as Task A5.3 for each of the 10 functions. Key mechanical translations for instance-type tests:

- `InstanceTypeDef` struct literals with the new post-Session-0 declaration shape (see `internal/component/component.go` for the current `InstanceTypeDef` definition).
- `ExportKindFunc` / `ExportKindInstance` enum references should match the current shape; verify via `grep -n 'ExportKind' internal/component/`.
- `TypeIdx` references resolve via `c.TypeDefs[typeIdx]`.

Each restored function gets a citation block:
```go
// TestInstanceTypeXxx asserts the decoder produces the expected
// InstanceTypeDef shape for the given binary input.
//
// Spec: Binary.md "Instance Type" section (see debug-vendored/component-model/design/mvp/Binary.md).
// No counterpart (justified): wazero decoder test; run_tests.py does not cover decoder behavior.
```

- [ ] **Step A6.3: Run the file**

```bash
go test ./internal/component/binary/ -run TestInstanceType -count=1 2>&1 | tail -20
```

Expected: all 10 restored tests PASS.

- [ ] **Step A6.4: Run per-task reviewers**

Dispatch both reviewers.

---

### Task A7: Checkpoint A verification

**Goal:** Confirm the Checkpoint A gate passes end-to-end.

- [ ] **Step A7.1: Build check**

```bash
cd /home/cchamplin/development/wazero && \
  go build ./internal/component/binary/... ./internal/component/... 2>&1 | head -40
```

Expected: empty.

- [ ] **Step A7.2: Test check**

```bash
go test ./internal/component/binary/... -count=1 2>&1 | tail -20
```

Expected: all tests in the `binary` package pass, including the 18 restored decoder tests.

- [ ] **Step A7.3: V5 verification**

```bash
grep -rn 'funcTypeIdx\|resourceDefs' internal/component/binary/
```

Expected: empty.

- [ ] **Step A7.4: V9 verification**

```bash
grep -n 'TypeDefs \[\]TypeDef' internal/component/component.go && \
  grep -n 'c\.TypeDefs = append' internal/component/binary/decoder.go
```

Expected: both return hits.

- [ ] **Step A7.5: Working-tree integrity**

```bash
git status --porcelain | head -30
```

Expected: only files from the Checkpoint A manifest (`component.go`, `component_typedef_test.go`, `component_test.go`, `binary/decoder.go`, `binary/component_type_test.go`, `binary/instance_type_test.go`) are modified or added.

- [ ] **Step A7.6: Dispatch checkpoint review**

Dispatch `superpowers:code-reviewer` over the full Checkpoint A scope. Dispatch spec-compliance reviewer with Checkpoint A citation audit (V4 grep on the restored test files). Apply correctives before Checkpoint B.

---

## Checkpoint B — `component.Instance` embeds `*runtime.ComponentInstance`

**Scope:** Delete duplicated runtime state from `component.Instance`. Rewrite every method that read/wrote the deleted fields as one-liner delegators into `rt`. Fix the `IsMayLeave` semantic bug (the spec's `may_leave` is a standalone boolean, not coupled to `enterCount`). Use `runtime.ReentranceTracker` for the transitive `CallMightBeRecursive` check. Update every call site inside `internal/component/` and `imports/wasip2/`.

**Design references:** Decision 3 (lines 185-286); Instance Layering — Concrete Shape (lines 726-827).

**Exit criterion (Checkpoint B gate):**
```bash
cd /home/cchamplin/development/wazero && \
  go build ./internal/component/... ./imports/wasip2/... 2>&1 | head -40 && \
  go test ./internal/component/runtime/... -count=1 2>&1 | tail -20
```
Expected: build empty, runtime package tests (including new tests for `IsMayLeave` fix and `CallMightBeRecursive` transitive walk) pass.

---

### Task B1: Fix `ComponentInstance.IsMayLeave` semantic bug

**Design reference:** Decision 3 IsMayLeave semantic fix (design lines 254-263).
**Spec citation:** `definitions.py:260, 270, 1955, 1973, 2065, 2135, 2143` — `may_leave` is a standalone boolean field on ComponentInstance with no coupling to `enter_count`. The `enter_count` is a separate field used for reentrance tracking.

**Files modified:** `internal/component/runtime/component_instance.go`.

- [ ] **Step B1.1: Write the failing test**

Create `internal/component/runtime/component_instance_may_leave_test.go` (or append to the existing `component_instance_test.go`) with:

```go
package runtime

import "testing"

// TestIsMayLeaveIsStandaloneBoolean asserts the spec's may_leave flag is
// independent of enterCount. The prior wazero implementation ANDed the two,
// which caused canon.lower and canon.resource.new/drop to trap while on
// the call stack even though the spec permits them.
//
// Spec: definitions.py:260 (class ComponentInstance: may_leave: bool).
// Spec: definitions.py:1955 (lower_flat_values sets may_leave=False).
// Spec: definitions.py:1973 (lower_flat_values restores may_leave=True).
// Spec: definitions.py:2065 (canon_lower: trap_if(not caller_task.inst.may_leave)).
// Wasmtime parallel: runtime/vm/component/concurrent_disabled.rs:159 may_enter().
func TestIsMayLeaveIsStandaloneBoolean(t *testing.T) {
	inst := NewComponentInstance(0, nil)

	// Fresh instance: MayLeave defaults true, enterCount = 0, IsMayLeave() = true.
	if !inst.IsMayLeave() {
		t.Fatalf("fresh instance IsMayLeave() = false, want true")
	}

	// Enter the instance. enterCount = 1, MayLeave still true.
	// Under the spec, IsMayLeave() must still be true because the two
	// fields are orthogonal.
	inst.Enter()
	if !inst.IsMayLeave() {
		t.Fatalf("entered instance IsMayLeave() = false, want true (enterCount is orthogonal to may_leave)")
	}

	// Now explicitly set MayLeave = false. IsMayLeave() must be false.
	inst.MayLeave = false
	if inst.IsMayLeave() {
		t.Fatalf("MayLeave=false IsMayLeave() = true, want false")
	}

	// Restore MayLeave = true. IsMayLeave() must be true regardless of
	// enterCount.
	inst.MayLeave = true
	if !inst.IsMayLeave() {
		t.Fatalf("restored MayLeave IsMayLeave() = false, want true")
	}

	// Leave. enterCount = 0, MayLeave still true, IsMayLeave() still true.
	inst.Leave()
	if !inst.IsMayLeave() {
		t.Fatalf("left instance IsMayLeave() = false, want true")
	}
}
```

- [ ] **Step B1.2: Run the test to confirm it fails**

```bash
go test ./internal/component/runtime/ -run TestIsMayLeaveIsStandaloneBoolean -count=1 2>&1 | tail -20
```

Expected: test fails at the `inst.Enter(); if !inst.IsMayLeave()` assertion — current implementation returns `MayLeave && enterCount == 0`, so after Enter enterCount=1, IsMayLeave returns false.

- [ ] **Step B1.3: Implement the fix**

Edit `internal/component/runtime/component_instance.go` at the existing `IsMayLeave` method (around line 100):

```go
// IsMayLeave reports the spec's may_leave flag. The flag is toggled
// by lower_flat_values (definitions.py:1955 / :1973) and checked by
// canon.lower and canon.resource.* (definitions.py:2065, :2135, :2143).
// It is ORTHOGONAL to enterCount — the spec has two independent
// fields, and wazero must not couple them.
//
// Session 1 fix: the prior body `return c.MayLeave && c.enterCount == 0`
// was a wazero divergence that conflated reentrance with can-leave
// semantics. The two are now fully independent.
func (c *ComponentInstance) IsMayLeave() bool {
	return c.MayLeave
}
```

- [ ] **Step B1.4: Run the test to confirm it passes**

```bash
go test ./internal/component/runtime/ -run TestIsMayLeaveIsStandaloneBoolean -count=1 2>&1 | tail -5
```

Expected: PASS.

Then verify nothing else in the package broke:

```bash
go test ./internal/component/runtime/ -count=1 2>&1 | tail -20
```

Expected: all runtime tests pass. If a pre-existing test was asserting the buggy behavior (`enterCount == 0` coupling), update its assertion to match the new spec-correct semantics and add a citation comment explaining the Session 1 fix.

- [ ] **Step B1.5: Run per-task reviewers**

Dispatch both reviewers.

---

### Task B2: Add `ReentranceTracker.CallMightBeRecursive(calleeID)` method

**Design reference:** Decision 3 `CallMightBeRecursive` transitive ancestor check (design lines 265-286).
**Spec citation:** `definitions.py:290-299` — `call_might_be_recursive(caller, callee_inst)` uses `reflexive_ancestors()` overlap. Not direct `caller == inst` equality.

**Files modified:** `internal/component/runtime/reentrance.go`, `internal/component/runtime/reentrance_test.go`.

- [ ] **Step B2.1: Inspect existing ReentranceTracker**

```bash
grep -n 'type ReentranceTracker\|func.*ReentranceTracker\|func NewReentranceTracker' internal/component/runtime/reentrance.go
```

Determine the current shape (fields + methods). The design assumes `EnterInstance(id uint32)` / `LeaveInstance(id uint32)` exist; verify.

- [ ] **Step B2.2: Write the failing test**

Append to `internal/component/runtime/reentrance_test.go`:

```go
// TestReentranceTrackerCallMightBeRecursive asserts the tracker's
// spec-correct transitive recursive-call detection.
//
// Spec: definitions.py:290-299 call_might_be_recursive:
//   def call_might_be_recursive(caller, callee_inst):
//     if caller is None:
//       return False
//     return caller.task.inst in callee_inst.reflexive_ancestors() \
//         or callee_inst in caller.task.inst.reflexive_ancestors()
//
// The tracker implements this by maintaining a set of active instance
// IDs on the current call stack and consulting it.
func TestReentranceTrackerCallMightBeRecursive(t *testing.T) {
	rt := NewReentranceTracker()

	// No active calls: no recursion possible.
	if rt.CallMightBeRecursive(5) {
		t.Fatalf("empty tracker CallMightBeRecursive(5) = true, want false")
	}

	// Enter instance 5. Calling 5 is now recursive.
	rt.EnterInstance(5)
	if !rt.CallMightBeRecursive(5) {
		t.Fatalf("after Enter(5): CallMightBeRecursive(5) = false, want true")
	}
	// Calling a different instance (7) from inside 5 is not recursive.
	if rt.CallMightBeRecursive(7) {
		t.Fatalf("after Enter(5): CallMightBeRecursive(7) = true, want false")
	}

	// Enter nested instance 7. Calling either is now recursive.
	rt.EnterInstance(7)
	if !rt.CallMightBeRecursive(5) || !rt.CallMightBeRecursive(7) {
		t.Fatalf("nested 5→7: CallMightBeRecursive should be true for both")
	}

	// Leave 7. Back to just 5 on the stack.
	rt.LeaveInstance(7)
	if !rt.CallMightBeRecursive(5) {
		t.Fatalf("after Leave(7): CallMightBeRecursive(5) = false, want true")
	}
	if rt.CallMightBeRecursive(7) {
		t.Fatalf("after Leave(7): CallMightBeRecursive(7) = true, want false")
	}

	// Leave 5. Stack empty.
	rt.LeaveInstance(5)
	if rt.CallMightBeRecursive(5) {
		t.Fatalf("after Leave(5): CallMightBeRecursive(5) = true, want false")
	}
}
```

- [ ] **Step B2.3: Run the test to confirm it fails**

```bash
go test ./internal/component/runtime/ -run TestReentranceTrackerCallMightBeRecursive -count=1 2>&1 | tail -20
```

Expected: failure — either the method doesn't exist (`rt.CallMightBeRecursive undefined`) or returns incorrect values.

- [ ] **Step B2.4: Implement**

Open `internal/component/runtime/reentrance.go`. Add the method (and supporting state if needed):

```go
// CallMightBeRecursive reports whether calling callee would be recursive
// given the currently-active instance stack. An instance is considered
// active between EnterInstance(id) and LeaveInstance(id). A call is
// recursive if callee is already on the active stack.
//
// Spec: definitions.py:290-299 call_might_be_recursive. The spec uses
// reflexive_ancestors() overlap between caller and callee; wazero's
// tracker models this by maintaining the per-instance active set on
// the shared tracker used by all instances on a call stack.
func (r *ReentranceTracker) CallMightBeRecursive(calleeID uint32) bool {
	if r == nil {
		return false
	}
	return r.isActive(calleeID)
}
```

If `isActive(id uint32) bool` does not already exist, add it: consult the existing `EnterInstance` / `LeaveInstance` to see how the active set is stored. Typical shape:

```go
type ReentranceTracker struct {
	active map[uint32]int  // id → nested enter count
}

func NewReentranceTracker() *ReentranceTracker {
	return &ReentranceTracker{active: make(map[uint32]int)}
}

func (r *ReentranceTracker) EnterInstance(id uint32) { r.active[id]++ }
func (r *ReentranceTracker) LeaveInstance(id uint32) {
	if r.active[id] > 1 {
		r.active[id]--
	} else {
		delete(r.active, id)
	}
}
func (r *ReentranceTracker) isActive(id uint32) bool {
	return r.active[id] > 0
}
```

(Adapt to the existing shape; do not replace a working implementation with a rewrite.)

- [ ] **Step B2.5: Run the test to confirm it passes**

```bash
go test ./internal/component/runtime/ -run TestReentranceTrackerCallMightBeRecursive -count=1 2>&1 | tail -5
```

Expected: PASS.

- [ ] **Step B2.6: Run per-task reviewers**

Dispatch both reviewers.

---

### Task B3: Rewrite `component.Instance` struct to embed `*runtime.ComponentInstance`

**Design reference:** Decision 3 (design lines 185-253); Instance Layering After (lines 760-827).
**Spec citation:** `definitions.py:256-273` ComponentInstance shape. Wasmtime parallel: `runtime/component/instance.rs:710-833` Instantiator uses a single runtime instance struct.
**Files modified:** `internal/component/instance.go` (struct definition + `newInstance` constructor + all delegator methods).

- [ ] **Step B3.1: Write failing tests for the new shape**

Append to `internal/component/instance_test.go` (or create `internal/component/instance_embedding_test.go` — confirm the file does not already have the tests via grep first):

```go
// TestInstanceEmbedsRuntimeComponentInstance asserts Instance carries
// a *runtime.ComponentInstance and delegates spec-level state.
//
// Spec: definitions.py:256-273 class ComponentInstance.
// Wasmtime parallel: runtime/component/instance.rs:710-743 (Instantiator).
func TestInstanceEmbedsRuntimeComponentInstance(t *testing.T) {
	c := &Component{}
	inst := newInstance(c, 0, nil)
	if inst.Runtime() == nil {
		t.Fatalf("inst.Runtime() = nil, want non-nil *runtime.ComponentInstance")
	}
	// Spec: definitions.py:260 may_leave defaults true.
	if !inst.MayLeave() {
		t.Fatalf("fresh inst.MayLeave() = false, want true")
	}
	if inst.ActiveCallDepth() != 0 {
		t.Fatalf("fresh inst.ActiveCallDepth() = %d, want 0", inst.ActiveCallDepth())
	}
}

// TestInstanceMayLeaveDelegatesToRuntime asserts MayLeave/SetMayLeave
// read/write runtime state directly, not a duplicate wrapper field.
//
// Spec: definitions.py:260 may_leave field.
func TestInstanceMayLeaveDelegatesToRuntime(t *testing.T) {
	inst := newInstance(&Component{}, 0, nil)
	inst.SetMayLeave(false)
	if inst.Runtime().MayLeave {
		t.Fatalf("SetMayLeave(false): rt.MayLeave = true, want false (wrapper must write through)")
	}
	inst.SetMayLeave(true)
	if !inst.Runtime().MayLeave {
		t.Fatalf("SetMayLeave(true): rt.MayLeave = false, want true")
	}
}

// TestInstanceCallMightBeRecursiveUsesReentranceTracker asserts the
// wrapper's CallMightBeRecursive uses the runtime ReentranceTracker,
// not direct caller == i equality.
//
// Spec: definitions.py:290-299 call_might_be_recursive.
func TestInstanceCallMightBeRecursiveUsesReentranceTracker(t *testing.T) {
	a := newInstance(&Component{}, 1, nil)
	b := newInstance(&Component{}, 2, nil)

	// Neither has entered: nothing is recursive.
	if a.CallMightBeRecursive(b) {
		t.Fatalf("before any Enter: CallMightBeRecursive(b) = true, want false")
	}

	// a.Enter() activates instance id 1. Calling a (callee=a) is now recursive.
	a.EnterCall()
	defer a.ExitCall()
	if !a.CallMightBeRecursive(a) {
		t.Fatalf("after a.Enter: a.CallMightBeRecursive(a) = false, want true")
	}
}
```

- [ ] **Step B3.2: Run the tests to confirm they fail**

```bash
go test ./internal/component/ -run 'TestInstance(EmbedsRuntimeComponentInstance|MayLeaveDelegatesToRuntime|CallMightBeRecursiveUsesReentranceTracker)' -count=1 2>&1 | tail -30
```

Expected failures: `inst.Runtime undefined`, `inst.Runtime().MayLeave` (because current field shape hides the runtime instance), or semantic mismatches.

- [ ] **Step B3.3: Delete duplicated fields + rewrite struct**

Read `internal/component/instance.go` lines 50-90 to see the current `Instance` struct. Rewrite to match Decision 3:

```go
// Instance is the linker/compile-time wrapper around a running component
// instantiation. Per-instance runtime state matching the canonical-abi
// spec's ComponentInstance (definitions.py:256-273) lives on the
// embedded *runtime.ComponentInstance. Wrapper-level state (core module
// instances, component-level exports, linker-time index spaces) stays
// on this struct because runtime/ cannot import component/ without an
// import cycle.
//
// Session 1 design: Decision 3 (design lines 185-253).
type Instance struct {
	// rt is the per-instance runtime state. One-to-one with this Instance
	// and non-nil after newInstance.
	rt *runtime.ComponentInstance

	// Linker-time state.
	component      *Component
	coreInstances  []api.Module
	exports        map[string]*ExportedFunc
	componentFuncs map[uint32]ComponentFunc

	// Value index space for start function support.
	values         []types.Val
	valuesConsumed []bool

	// Wrapper-layer instance tree. rt.Parent holds the runtime-layer
	// back-pointer; parent / children hold *component.Instance wrapper
	// pointers so linker code can navigate without going through rt.
	parent            *Instance
	children          []*Instance
	instanceSpace     []*Instance
	typeSpace         []*TypeDef
	componentSpace    []*Component
	exportedInstances map[string]*Instance
}
```

**Deleted fields:** `table`, `destructors`, `callContext`, `mayLeaveDisabled`, `activeCallDepth`.

- [ ] **Step B3.4: Rewrite `newInstance` constructor**

Locate the existing constructor. Rewrite:

```go
func newInstance(c *Component, id uint32, parent *Instance) *Instance {
	var parentRT *runtime.ComponentInstance
	if parent != nil {
		parentRT = parent.rt
	}
	return &Instance{
		component:      c,
		rt:             runtime.NewComponentInstance(id, parentRT),
		coreInstances:  make([]api.Module, 0),
		exports:        make(map[string]*ExportedFunc),
		componentFuncs: make(map[uint32]ComponentFunc),
		parent:         parent,
	}
}
```

- [ ] **Step B3.5: Add/update delegator methods**

Add (or update in-place if any already exist):

```go
// Runtime returns the embedded *runtime.ComponentInstance.
func (i *Instance) Runtime() *runtime.ComponentInstance { return i.rt }

// MayLeave reports the spec may_leave flag. Spec: definitions.py:260.
func (i *Instance) MayLeave() bool { return i.rt.IsMayLeave() }

// SetMayLeave writes the spec may_leave flag. Spec: definitions.py:1955, :1973.
func (i *Instance) SetMayLeave(allowed bool) { i.rt.MayLeave = allowed }

// ActiveCallDepth returns the current reentrance nesting count.
func (i *Instance) ActiveCallDepth() int { return i.rt.EnterCount() }

// EnterCall increments the call-depth counter and registers the instance
// on the ReentranceTracker so CallMightBeRecursive can detect recursive
// re-entries. Spec: definitions.py:290-299.
func (i *Instance) EnterCall() {
	i.rt.Enter()
	i.rt.Reentrance.EnterInstance(i.rt.ID)
}

// ExitCall is the inverse of EnterCall.
func (i *Instance) ExitCall() {
	i.rt.Reentrance.LeaveInstance(i.rt.ID)
	i.rt.Leave()
}

// Table returns the per-instance runtime handle table.
func (i *Instance) Table() *runtime.Table { return i.rt.Table }

// Parent returns the wrapper-layer parent, paired with rt.Parent at
// construction time.
func (i *Instance) Parent() *Instance { return i.parent }

// CallMightBeRecursive reports whether calling i from caller would be
// recursive given the active ReentranceTracker state. Spec:
// definitions.py:290-299 call_might_be_recursive.
func (i *Instance) CallMightBeRecursive(caller *Instance) bool {
	if i == nil || i.rt == nil {
		return false
	}
	// Check whether i is currently active on the shared ReentranceTracker.
	if i.rt.Reentrance.CallMightBeRecursive(i.rt.ID) {
		return true
	}
	// Also consult the caller's tracker in case they are on disjoint
	// trackers (cross-instance call chains). In Session 1's local-only
	// model this is typically the same tracker via the shared runtime.
	if caller != nil && caller.rt != nil && caller.rt.Reentrance != nil {
		return caller.rt.Reentrance.CallMightBeRecursive(i.rt.ID)
	}
	return false
}

// ValidateMayLeave traps if the instance cannot currently leave.
// Spec: definitions.py:2065, :2135, :2143.
func (i *Instance) ValidateMayLeave() error {
	if !i.rt.IsMayLeave() {
		return errMayNotLeave
	}
	return nil
}
```

**Deleted methods:** `SetCallContext`, `CallContext()` (CallContext is per-call state allocated in canon.lift / canon.lower closures, not stored on Instance). `SetDestructor` delegates to `rt.Destructors.Register` — if `runtime.ComponentInstance` doesn't yet have a `Destructors.Register`, either add it or route the wrapper's `SetDestructor` directly to `rt.Destructors` as the per-instance destructor registry.

- [ ] **Step B3.6: Define `errMayNotLeave` sentinel**

If the sentinel doesn't already exist in the package, add it (near the top of `instance.go`):

```go
var errMayNotLeave = errors.New("component instance cannot leave (may_leave=false)")
```

Add `"errors"` to imports if missing.

- [ ] **Step B3.7: Build + run tests**

```bash
go build ./internal/component/... 2>&1 | head -40 && \
  go test ./internal/component/ -run 'TestInstance(EmbedsRuntimeComponentInstance|MayLeaveDelegatesToRuntime|CallMightBeRecursiveUsesReentranceTracker)' -count=1 2>&1 | tail -10
```

Expected: build may show errors in OTHER files (call sites that used the deleted fields) — those are fixed in Task B4. The three new tests in this task must PASS in isolation.

Run isolated:
```bash
go test ./internal/component/ -run 'TestInstance(EmbedsRuntimeComponentInstance|MayLeaveDelegatesToRuntime|CallMightBeRecursiveUsesReentranceTracker)' -count=1 -run-only 2>&1
```

If compilation is blocked by call-site errors in other files, proceed to Task B4 first and revisit these tests at Checkpoint B verification.

- [ ] **Step B3.8: Run per-task reviewers**

Dispatch both reviewers with scope `internal/component/instance.go`. Note in the review request: "This task intentionally leaves call-site errors in other files — Task B4 fixes them."

---

### Task B4: Migrate every call site to the new embedded shape

**Design reference:** Decision 3 delegated methods (design lines 233-246). Instance Layering construction (lines 789-807).
**Spec citation:** N/A (mechanical migration).
**Files modified:** every file in `internal/component/` that references the deleted fields, plus every file in `imports/wasip2/` that accesses `instance.table` / `instance.destructors` / `instance.mayLeaveDisabled` / `instance.activeCallDepth` / `instance.callContext`.

- [ ] **Step B4.1: Enumerate call sites**

```bash
grep -rn 'inst\.table\|inst\.destructors\|inst\.callContext\|inst\.mayLeaveDisabled\|inst\.activeCallDepth' internal/component/ imports/wasip2/ 2>&1
grep -rn 'i\.table\b\|i\.destructors\b\|i\.callContext\b\|i\.mayLeaveDisabled\b\|i\.activeCallDepth\b' internal/component/ imports/wasip2/ 2>&1
```

Record every hit. Each becomes a mechanical rewrite:

| Before | After |
|---|---|
| `i.table` | `i.rt.Table` or `i.Table()` |
| `i.destructors[idx]` | `i.rt.Destructors.Get(idx)` (or the equivalent Destructor registry API) |
| `i.destructors[idx] = dtor` | `i.rt.Destructors.Register(idx, dtor)` (or SetDestructor method if the registry exposes that) |
| `i.callContext` | allocate a fresh `runtime.NewCallContext()` at the call site (CallContext is per-call, not per-instance) |
| `i.mayLeaveDisabled` | `!i.MayLeave()` |
| `i.mayLeaveDisabled = true` | `i.SetMayLeave(false)` |
| `i.mayLeaveDisabled = false` | `i.SetMayLeave(true)` |
| `i.activeCallDepth` | `i.ActiveCallDepth()` |
| `atomic.AddInt32(&i.activeCallDepth, 1)` | `i.EnterCall()` |
| `atomic.AddInt32(&i.activeCallDepth, -1)` | `i.ExitCall()` |

- [ ] **Step B4.2: Audit the `runtime.Destructors` API**

```bash
grep -n 'type.*Destructor\|func.*DestructorRegistry\|Destructors\|destructors' internal/component/runtime/destructor.go
```

Confirm the Destructor registry has `Register` + `Get` (or equivalent). If not, the wrapper's `SetDestructor(idx, dtor)` can store into `i.rt.Destructors` directly via whatever API exists. Do NOT add new APIs to `runtime.DestructorRegistry` in this task — instead, adapt the wrapper to the existing API.

- [ ] **Step B4.3: Rewrite each call site**

For each hit from B4.1, edit in place. Run the build after every file or two to catch regressions early:

```bash
go build ./internal/component/... ./imports/wasip2/... 2>&1 | head -20
```

If a call site accesses `i.callContext` and expects it as a per-instance field, replace with a fresh `runtime.NewCallContext()` local variable at the call site. If the caller needs to pass the context across multiple lifts/lowers within one outer call, route the context through the function parameters, not through the Instance.

- [ ] **Step B4.4: Delete the `ExportedFunc.Call` panic stub and leave a precise Session-1-rebuilt trap**

At `internal/component/instance.go:156` the current panic stub is `ExportedFunc.Call`. Checkpoint C is where the body is rebuilt around `abi.LiftParams` / `LowerResults`. For Checkpoint B, the body is allowed to remain a precise error that cites Checkpoint C:

```go
// Call invokes the exported function with the given arguments.
//
// Checkpoint B: delegators in place; the full body is rebuilt in Checkpoint C
// Task C5 against abi.LiftParams / abi.LowerResults. Until then Call returns
// a precise error rather than panicking.
func (f *ExportedFunc) Call(ctx context.Context, params ...types.Val) ([]types.Val, error) {
	return nil, fmt.Errorf("ExportedFunc.Call: rebuild in progress (Session 1 Checkpoint C Task C5)")
}
```

Do NOT leave `panic(...)`. The explicit error is a compile-green placeholder that gets deleted in Task C5.

Same pattern for the `ResourceNew` / `ResourceRep` / `ResourceDrop` panic stubs at `instance.go:185/193/202`, BUT with the Session 1 spec-correct signatures (so `createResourceOpExport` in Task C8 compiles cleanly against them). The bodies are precise placeholder errors; the full spec-correct bodies land in Task E5.

```go
// ResourceNew is canon.resource.new — spec definitions.py:2134-2138.
// Session 1 Task B4: signature in place; body is a placeholder that
// returns a precise error. Task E5 wires the full spec-correct body
// against Table.NewResourceHandle after Tasks E1 (GetByIndex) + E2
// (Rep uint32) + E3 (BorrowScope.ReleaseBorrow) land.
func (i *Instance) ResourceNew(resourceIdx types.ResourceIdx, rep uint32) (uint32, error) {
	_, _ = resourceIdx, rep
	return 0, fmt.Errorf("Instance.ResourceNew: body rebuild in progress (Session 1 Checkpoint E Task E5)")
}

// ResourceRep is canon.resource.rep — spec definitions.py:2169-2173.
func (i *Instance) ResourceRep(resourceIdx types.ResourceIdx, handleIdx uint32) (uint32, error) {
	_, _ = resourceIdx, handleIdx
	return 0, fmt.Errorf("Instance.ResourceRep: body rebuild in progress (Session 1 Checkpoint E Task E5)")
}

// ResourceDrop is canon.resource.drop — spec definitions.py:2142-2165.
func (i *Instance) ResourceDrop(resourceIdx types.ResourceIdx, handleIdx uint32) error {
	_, _ = resourceIdx, handleIdx
	return fmt.Errorf("Instance.ResourceDrop: body rebuild in progress (Session 1 Checkpoint E Task E5)")
}
```

**Note:** these are transitional compile-green placeholders for Checkpoints B-D only. They are deleted/replaced in Tasks C5 (ExportedFunc.Call) and E5 (Resource*). `grep -n 'rebuild in progress' internal/component/instance.go` must return empty at Checkpoint F.

Also delete any pre-existing wasip2 or test call sites that passed the old `(rep any)` signature and update to the new `(resourceIdx types.ResourceIdx, rep uint32)` shape. The wasip2 modules call `NewResourceHandle` directly on the runtime Table, not through `Instance.ResourceNew`, so this change should be contained to internal/component/ callers.

- [ ] **Step B4.5: Build the component + wasip2 packages**

```bash
go build ./internal/component/... ./imports/wasip2/... 2>&1 | head -40
```

Expected: empty. Iterate with per-file fixes until green.

- [ ] **Step B4.6: Run the accessor tests added in Task B3**

```bash
go test ./internal/component/ -run 'TestInstance(EmbedsRuntimeComponentInstance|MayLeaveDelegatesToRuntime|CallMightBeRecursiveUsesReentranceTracker)' -count=1 2>&1 | tail -10
```

Expected: PASS.

- [ ] **Step B4.7: Run per-task reviewers**

Dispatch both reviewers with scope "every file edited in Task B4". Spec reviewer checklist item: "confirm no call site silently ignores the semantic change where `mayLeaveDisabled = true` used to mean the inverse of the spec's `may_leave`."

---

### Task B5: Checkpoint B verification

- [ ] **Step B5.1: Build check**

```bash
cd /home/cchamplin/development/wazero && \
  go build ./internal/component/... ./imports/wasip2/... 2>&1 | head -40
```

Expected: empty.

- [ ] **Step B5.2: Runtime tests green**

```bash
go test ./internal/component/runtime/... -count=1 2>&1 | tail -20
```

Expected: all tests in the `runtime` package pass, including the new `TestIsMayLeaveIsStandaloneBoolean` and `TestReentranceTrackerCallMightBeRecursive`.

- [ ] **Step B5.3: Component accessor tests green**

```bash
go test ./internal/component/ -run 'TestInstance(EmbedsRuntimeComponentInstance|MayLeaveDelegatesToRuntime|CallMightBeRecursiveUsesReentranceTracker)' -count=1 2>&1 | tail -10
```

Expected: PASS.

- [ ] **Step B5.4: V6 verification**

```bash
grep -n 'table.*runtime\.Table\|mayLeaveDisabled\|activeCallDepth' internal/component/instance.go
```

Expected: empty (duplicated fields deleted). The `i.rt.Table` references are fine — V6 forbids only the DUPLICATED `i.table` field declaration.

- [ ] **Step B5.5: Working-tree integrity**

```bash
git status --porcelain | head -30
```

Expected: only files from Checkpoint B manifest touched.

- [ ] **Step B5.6: Dispatch checkpoint review**

Dispatch both reviewers over Checkpoint B scope. Apply correctives before Checkpoint C.

---

## Checkpoint C — `Instantiate` + canon.lift/lower/resource + primitive conformance

**Scope:** Delete the `component_linker.go::Instantiate` and `coreSignature` panic stubs. Rebuild the 14-step `Instantiate` pipeline. Wire canon.lift / canon.lower / canon.resource.* host module exports. Add `abi.LowerParams` / `LiftParams` / `LowerResults` / `LiftResults` helpers. Change `ComponentLinker.DefineFunc` signature to require `*types.TypeFunc`. Migrate every `imports/wasip2/` call site to the new `DefineFunc` signature. Restore primitive / composite / string / abi_edge / flat_abi / post_return conformance tests. Restore the `linker_test.go` and `linker_api_test.go` suites. Audit `primitives_test.go` and `may_leave_test.go` for missing citation blocks.

**Design references:** Decision 1 (lines 129-140), Decision 6 (lines 449-475), Decision 7 (lines 477-667), Instantiate Pipeline section (lines 829-1137), Canon Resource Ops (lines 1249-1259), Test Restoration Methodology (lines 1580-1669), File Manifest (lines 1924-1932).

**Exit criterion (Checkpoint C gate):**
```bash
cd /home/cchamplin/development/wazero && \
  go build ./... 2>&1 | head -40 && \
  go test ./internal/component/conformance/ -run 'Primitives|Composites|Strings|ABIEdge|FlatABI|PostReturn|Linker' -count=1 2>&1 | tail -20 && \
  go test ./internal/component/ -run 'Linker' -count=1 2>&1 | tail -20
```
Expected: build empty; all named conformance and linker tests pass.

---

### Task C1: Add `abi.LowerParams` aggregate helper (failing test first)

**Design reference:** Instantiate pipeline canon.lift wiring (design lines 981-1037); File Manifest abi helpers (lines 1924-1928).
**Spec citation:** `definitions.py:1954-1974` `lower_flat_values` — the aggregate boundary decision (flat path vs retptr via realloc) with `may_leave = False` toggle at `:1955` and restore at `:1973`. `definitions.py:1943-1952` is the inverse `lift_flat_values`.
**Files modified:** `internal/component/abi/lower.go` (add `LowerParams`), `internal/component/abi/lower_params_test.go` (new file).

- [ ] **Step C1.1: Write the failing test**

Create `internal/component/abi/lower_params_test.go`:

```go
package abi

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/wazerotest"
)

// TestLowerParamsFlatPath asserts LowerParams takes the flat path when
// the flattened parameter count is <= maxFlat. The may_leave toggle is
// applied by the caller (buildCanonLiftFunc); LowerParams does not
// touch ctx.Instance.MayLeave directly per the clarified design.
//
// Spec: definitions.py:1954-1974 lower_flat_values.
// Canonical test: run_tests.py test_flatten cases exercising
// small-parameter signatures.
func TestLowerParamsFlatPath(t *testing.T) {
	bag := types.NewComponentTypesBuilder().Finish()
	inst := runtime.NewComponentInstance(0, nil)
	mem := wazerotest.NewMemory(1) // 1 page is fine for this test
	ctx := &LowerContext{
		Memory:   mem,
		Types:    bag,
		Instance: inst,
	}
	// Two i32 params → 2 flat values → under maxFlat=16.
	paramTypes := []types.ValType{types.S32, types.S32}
	args := []types.Val{types.ValS32(42), types.ValS32(-7)}
	flat, err := LowerParams(ctx, paramTypes, args, MaxFlatParams)
	if err != nil {
		t.Fatalf("LowerParams: %v", err)
	}
	if len(flat) != 2 {
		t.Fatalf("flat len = %d, want 2", len(flat))
	}
	if int32(flat[0]) != 42 || int32(flat[1]) != -7 {
		t.Fatalf("flat = %v, want [42, -7]", flat)
	}
}

```

(Only the flat path test is written in Task C1. The retptr path test is added in Task C2 alongside `LiftParams` / `LiftResults` / `LowerResults` so the retptr wiring lands as one coherent change.)

- [ ] **Step C1.2: Run the test to confirm it fails**

```bash
go test ./internal/component/abi/ -run TestLowerParamsFlatPath -count=1 2>&1 | tail -10
```

Expected: `LowerParams undefined` or `MaxFlatParams undefined`.

- [ ] **Step C1.3: Implement**

Read the current `internal/component/abi/lower.go` to understand the per-value lowering primitives (`LowerFlat`, `LowerHeap`, etc.). Add `LowerParams` at the end of the file:

```go
// MaxFlatParams is the spec's MAX_FLAT_PARAMS from definitions.py.
// Component-model spec: 16 flat values. Canonical: canonical-abi/definitions.py.
const MaxFlatParams = 16

// MaxFlatResults is the spec's MAX_FLAT_RESULTS from definitions.py.
// Canonical: canonical-abi/definitions.py. Must match the value the
// component-model spec uses for the canonical flat ABI single-result
// threshold (1).
const MaxFlatResults = 1

// LowerParams lowers a slice of component-level values into the flat ABI
// representation the core wasm callee expects. Implements the aggregate
// boundary decision from lower_flat_values at definitions.py:1954-1974:
//
//   - flatten the parameter types into []FlatType
//   - if len(flatTypes) > maxFlat:
//       - realloc a tuple buffer
//       - store each arg into the buffer via lowerHeap
//       - return [ptr] as the single flat value (retptr path)
//   - else:
//       - call LowerFlat for each arg, concatenating into the flat slice
//
// The caller (buildCanonLiftFunc / createCanonLowerFunc) is responsible
// for toggling ctx.Instance.MayLeave around this call per spec :1955 and
// :1973. Doing the toggle here would couple the helper to the instance
// state model and preclude sharing it with test harnesses that use a
// detached runtime.
//
// Spec: definitions.py:1954-1974 lower_flat_values.
// Wasmtime parallel: runtime/component/func.rs Func::call_raw aggregate
// lowering (the lower_args path at lines ~512+).
func LowerParams(ctx *LowerContext, paramTypes []types.ValType, args []types.Val, maxFlat int) ([]uint64, error) {
	if len(paramTypes) != len(args) {
		return nil, fmt.Errorf("lower params: arg count %d != param count %d", len(args), len(paramTypes))
	}
	flatTypes := FlattenParams(ctx.Types, paramTypes)
	if len(flatTypes) > maxFlat {
		// Retptr path. Compute tuple alignment + size from paramTypes,
		// allocate via ctx.Realloc, store each arg into the buffer via
		// per-value LowerHeap.
		tupleAlign := tupleAlignment(ctx.Types, paramTypes)
		tupleSize := tupleSize(ctx.Types, paramTypes)
		ptr, err := ctx.Realloc(0, 0, tupleAlign, tupleSize)
		if err != nil {
			return nil, fmt.Errorf("lower params: realloc: %w", err)
		}
		offset := ptr
		for i, pt := range paramTypes {
			offset = alignTo(offset, typeAlignment(ctx.Types, pt))
			if err := LowerHeap(ctx, args[i], pt, offset); err != nil {
				return nil, fmt.Errorf("lower params[%d]: %w", i, err)
			}
			offset += typeSize(ctx.Types, pt)
		}
		return []uint64{uint64(ptr)}, nil
	}
	// Flat path: lower each arg and concatenate.
	flat := make([]uint64, 0, len(flatTypes))
	for i, pt := range paramTypes {
		lowered, err := LowerFlat(ctx, args[i], pt)
		if err != nil {
			return nil, fmt.Errorf("lower params[%d]: %w", i, err)
		}
		flat = append(flat, lowered...)
	}
	return flat, nil
}
```

Note: `tupleAlignment`, `tupleSize`, `typeAlignment`, `typeSize` already exist in `abi/` (they are used by `LowerHeap` dispatch). If they are named differently, adapt the calls. `ctx.Realloc` is assumed to be a `Realloc func(oldPtr, oldSize, align, newSize uint32) (uint32, error)` field on `LowerContext`; if the actual field name/shape differs, adapt. Do NOT invent new API — consult the current `abi/context.go` to see the existing realloc adapter.

- [ ] **Step C1.4: Run the test to confirm it passes**

```bash
go test ./internal/component/abi/ -run TestLowerParamsFlatPath -count=1 2>&1 | tail -10
```

Expected: PASS.

- [ ] **Step C1.5: Run per-task reviewers**

Dispatch both reviewers.

---

### Task C2: Complete `LowerParams` retptr test + add `LiftParams`, `LiftResults`, `LowerResults`

**Design reference:** canon.lift + canon.lower wiring (design lines 915-1132).
**Spec citation:** `definitions.py:1954-1974` `lower_flat_values` (lower path), `:1943-1952` `lift_flat_values` (lift path).
**Files modified:** `internal/component/abi/lower.go` (add `LowerResults`), `internal/component/abi/lift.go` (add `LiftParams`, `LiftResults`), `internal/component/abi/lower_params_test.go`, `internal/component/abi/lift_params_test.go` (new).

- [ ] **Step C2.1: Add the retptr test**

Open `internal/component/abi/lower_params_test.go` and append `TestLowerParamsRetptrPath`:

```go
// TestLowerParamsRetptrPath ... (same citation block as C1.1)
func TestLowerParamsRetptrPath(t *testing.T) {
	bag := types.NewComponentTypesBuilder().Finish()
	inst := runtime.NewComponentInstance(0, nil)
	mem := wazerotest.NewMemory(1)
	// Wire a fake realloc that returns a fixed pointer and tracks calls.
	var reallocCalls int
	var lastPtr uint32 = 1024
	realloc := func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
		reallocCalls++
		return lastPtr, nil
	}
	ctx := &LowerContext{
		Memory:   mem,
		Realloc:  realloc,
		Types:    bag,
		Instance: inst,
	}
	// 3 i32 params with maxFlat=2 → retptr path.
	paramTypes := []types.ValType{types.S32, types.S32, types.S32}
	args := []types.Val{types.ValS32(1), types.ValS32(2), types.ValS32(3)}
	flat, err := LowerParams(ctx, paramTypes, args, 2)
	if err != nil {
		t.Fatalf("LowerParams retptr: %v", err)
	}
	if len(flat) != 1 {
		t.Fatalf("retptr flat len = %d, want 1", len(flat))
	}
	if uint32(flat[0]) != lastPtr {
		t.Fatalf("retptr flat[0] = %d, want %d", flat[0], lastPtr)
	}
	if reallocCalls != 1 {
		t.Fatalf("reallocCalls = %d, want 1", reallocCalls)
	}
	// Verify each arg was written to memory at ptr + i*4.
	for i := 0; i < 3; i++ {
		v, ok := mem.ReadUint32Le(lastPtr + uint32(i*4))
		if !ok {
			t.Fatalf("memory read at %d failed", lastPtr+uint32(i*4))
		}
		if int32(v) != int32(i+1) {
			t.Fatalf("memory[%d] = %d, want %d", i, int32(v), i+1)
		}
	}
}
```

- [ ] **Step C2.2: Write failing tests for `LiftParams`, `LiftResults`, `LowerResults`**

Create `internal/component/abi/lift_params_test.go`:

```go
package abi

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/wazerotest"
)

// TestLiftParamsFlatPath — spec :1943-1952 lift_flat_values flat branch.
func TestLiftParamsFlatPath(t *testing.T) {
	bag := types.NewComponentTypesBuilder().Finish()
	inst := runtime.NewComponentInstance(0, nil)
	mem := wazerotest.NewMemory(1)
	ctx := &LiftContext{
		Memory:      mem,
		Types:       bag,
		Instance:    inst,
		BorrowScope: runtime.NewBorrowScope(inst.Table),
	}
	paramTypes := []types.ValType{types.S32, types.S32}
	flat := []uint64{42, 0xFFFFFFF9} // 42, -7
	vals, err := LiftParams(ctx, paramTypes, flat, MaxFlatParams)
	if err != nil {
		t.Fatalf("LiftParams: %v", err)
	}
	if len(vals) != 2 {
		t.Fatalf("vals len = %d, want 2", len(vals))
	}
	if vals[0].S32() != 42 || vals[1].S32() != -7 {
		t.Fatalf("vals = %v, want [42, -7]", vals)
	}
}

// TestLiftResultsFlatPath — spec :1943-1952 lift_flat_values flat branch
// applied to results. Mirrors LiftParams shape but for MaxFlatResults.
func TestLiftResultsFlatPath(t *testing.T) {
	bag := types.NewComponentTypesBuilder().Finish()
	inst := runtime.NewComponentInstance(0, nil)
	mem := wazerotest.NewMemory(1)
	ctx := &LiftContext{
		Memory:      mem,
		Types:       bag,
		Instance:    inst,
		BorrowScope: runtime.NewBorrowScope(inst.Table),
	}
	// Single i32 result — fits in MaxFlatResults = 1.
	resultTypes := []types.ValType{types.S32}
	flat := []uint64{1234}
	vals, err := LiftResults(ctx, resultTypes, flat, MaxFlatResults)
	if err != nil {
		t.Fatalf("LiftResults: %v", err)
	}
	if len(vals) != 1 || vals[0].S32() != 1234 {
		t.Fatalf("vals = %v, want [1234]", vals)
	}
}

// TestLowerResultsFlatPath — spec :1954-1974 lower_flat_values flat branch
// applied to results. Writes results directly to the output stack slice.
func TestLowerResultsFlatPath(t *testing.T) {
	bag := types.NewComponentTypesBuilder().Finish()
	inst := runtime.NewComponentInstance(0, nil)
	mem := wazerotest.NewMemory(1)
	ctx := &LowerContext{
		Memory:   mem,
		Types:    bag,
		Instance: inst,
	}
	resultTypes := []types.ValType{types.S32}
	results := []types.Val{types.ValS32(999)}
	stack := make([]uint64, 1)
	if err := LowerResults(ctx, resultTypes, results, stack, false, MaxFlatResults); err != nil {
		t.Fatalf("LowerResults: %v", err)
	}
	if int32(stack[0]) != 999 {
		t.Fatalf("stack[0] = %d, want 999", int32(stack[0]))
	}
}
```

- [ ] **Step C2.3: Run tests to confirm all fail**

```bash
go test ./internal/component/abi/ -run 'TestLowerParamsRetptrPath|TestLiftParamsFlatPath|TestLiftResultsFlatPath|TestLowerResultsFlatPath' -count=1 2>&1 | tail -30
```

Expected: `LiftParams undefined`, `LiftResults undefined`, `LowerResults undefined`, plus the retptr test failing its assertions.

- [ ] **Step C2.4: Implement `LiftParams`, `LiftResults`, `LowerResults`**

In `internal/component/abi/lift.go`:

```go
// LiftParams lifts flat ABI values into a slice of component-level Vals.
// Implements the aggregate boundary from lift_flat_values at
// definitions.py:1943-1952:
//
//   - flatten paramTypes
//   - if len(flatTypes) > maxFlat:
//       - flat[0] is a retptr; read each param from memory at increasing
//         offsets using LiftHeap
//   - else:
//       - consume flat in order via LiftFlat for each param
//
// Spec: definitions.py:1943-1952 lift_flat_values.
func LiftParams(ctx *LiftContext, paramTypes []types.ValType, flat []uint64, maxFlat int) ([]types.Val, error) {
	flatTypes := FlattenParams(ctx.Types, paramTypes)
	if len(flatTypes) > maxFlat {
		if len(flat) == 0 {
			return nil, fmt.Errorf("lift params: retptr path but flat is empty")
		}
		ptr := uint32(flat[0])
		// Bounds + alignment check (spec :1947-1948).
		tupleAlign := tupleAlignment(ctx.Types, paramTypes)
		tupleSize := tupleSize(ctx.Types, paramTypes)
		if ptr != alignTo(ptr, tupleAlign) {
			return nil, fmt.Errorf("lift params: retptr %d not aligned to %d", ptr, tupleAlign)
		}
		if memSize(ctx.Memory) < uint64(ptr)+uint64(tupleSize) {
			return nil, fmt.Errorf("lift params: retptr + tuple size out of memory bounds")
		}
		vals := make([]types.Val, 0, len(paramTypes))
		offset := ptr
		for _, pt := range paramTypes {
			offset = alignTo(offset, typeAlignment(ctx.Types, pt))
			v, err := LiftHeap(ctx, pt, offset)
			if err != nil {
				return nil, fmt.Errorf("lift params: %w", err)
			}
			vals = append(vals, v)
			offset += typeSize(ctx.Types, pt)
		}
		return vals, nil
	}
	// Flat path: consume flat iter per param.
	vals := make([]types.Val, 0, len(paramTypes))
	iter := newFlatIter(flat)
	for _, pt := range paramTypes {
		v, err := LiftFlat(ctx, iter, pt)
		if err != nil {
			return nil, fmt.Errorf("lift params: %w", err)
		}
		vals = append(vals, v)
	}
	return vals, nil
}

// LiftResults lifts flat ABI result values into component Vals. Mirrors
// LiftParams with the MAX_FLAT_RESULTS threshold (single-result cap).
// Spec: definitions.py:1943-1952 lift_flat_values (used for result lifting
// in canon.lift return-path at canon_lift :1997).
func LiftResults(ctx *LiftContext, resultTypes []types.ValType, flat []uint64, maxFlat int) ([]types.Val, error) {
	flatTypes := FlattenResults(ctx.Types, resultTypes)
	if len(flatTypes) > maxFlat {
		if len(flat) == 0 {
			return nil, fmt.Errorf("lift results: retptr path but flat is empty")
		}
		ptr := uint32(flat[0])
		tupleAlign := tupleAlignment(ctx.Types, resultTypes)
		tupleSize := tupleSize(ctx.Types, resultTypes)
		if ptr != alignTo(ptr, tupleAlign) {
			return nil, fmt.Errorf("lift results: retptr %d not aligned", ptr)
		}
		if memSize(ctx.Memory) < uint64(ptr)+uint64(tupleSize) {
			return nil, fmt.Errorf("lift results: retptr + tuple size out of memory bounds")
		}
		vals := make([]types.Val, 0, len(resultTypes))
		offset := ptr
		for _, rt := range resultTypes {
			offset = alignTo(offset, typeAlignment(ctx.Types, rt))
			v, err := LiftHeap(ctx, rt, offset)
			if err != nil {
				return nil, fmt.Errorf("lift results: %w", err)
			}
			vals = append(vals, v)
			offset += typeSize(ctx.Types, rt)
		}
		return vals, nil
	}
	vals := make([]types.Val, 0, len(resultTypes))
	iter := newFlatIter(flat)
	for _, rt := range resultTypes {
		v, err := LiftFlat(ctx, iter, rt)
		if err != nil {
			return nil, fmt.Errorf("lift results: %w", err)
		}
		vals = append(vals, v)
	}
	return vals, nil
}
```

(`newFlatIter(flat)` + `memSize(mem)` may need adapter helpers if they don't already exist. Check the current `abi/` package for equivalents — `LiftFlat` signature uses something already. Adapt the variable names to match.)

In `internal/component/abi/lower.go` add `LowerResults`:

```go
// LowerResults lowers component Vals into flat ABI result values. If the
// flat width exceeds maxFlat, the caller has already provided a retptr
// (needsRetptr = true) in stack[len(params)] (or the slot designated by
// the caller) and LowerResults writes the tuple into memory at that ptr;
// otherwise it writes directly into stack[0..flatResultWidth].
//
// Spec: definitions.py:1954-1974 lower_flat_values applied to the result
// path in canon_lower (:2113 adjacent).
func LowerResults(ctx *LowerContext, resultTypes []types.ValType, results []types.Val, stack []uint64, needsRetptr bool, maxFlat int) error {
	if len(results) != len(resultTypes) {
		return fmt.Errorf("lower results: result count %d != type count %d", len(results), len(resultTypes))
	}
	if needsRetptr {
		// stack[len(params)] holds the retptr. Caller convention: the
		// last element of stack is the retptr for a core function with
		// `(params...) -> (retptr: i32)` signature. Session 1 adopts
		// the convention "stack[len(stack)-1] is the retptr when
		// needsRetptr is true".
		if len(stack) == 0 {
			return fmt.Errorf("lower results: needsRetptr=true but stack is empty")
		}
		ptr := uint32(stack[len(stack)-1])
		offset := ptr
		for i, rt := range resultTypes {
			offset = alignTo(offset, typeAlignment(ctx.Types, rt))
			if err := LowerHeap(ctx, results[i], rt, offset); err != nil {
				return fmt.Errorf("lower results[%d]: %w", i, err)
			}
			offset += typeSize(ctx.Types, rt)
		}
		return nil
	}
	// Flat path: lower each result and write into stack in order.
	idx := 0
	for i, rt := range resultTypes {
		lowered, err := LowerFlat(ctx, results[i], rt)
		if err != nil {
			return fmt.Errorf("lower results[%d]: %w", i, err)
		}
		for _, v := range lowered {
			if idx >= len(stack) {
				return fmt.Errorf("lower results: flat overflow stack (idx=%d, len=%d)", idx, len(stack))
			}
			stack[idx] = v
			idx++
		}
	}
	return nil
}
```

- [ ] **Step C2.5: Run tests to confirm they pass**

```bash
go test ./internal/component/abi/ -run 'TestLowerParamsRetptrPath|TestLiftParamsFlatPath|TestLiftResultsFlatPath|TestLowerResultsFlatPath' -count=1 2>&1 | tail -10
```

Expected: all four PASS.

- [ ] **Step C2.6: Run per-task reviewers**

Dispatch both reviewers. Spec reviewer MUST verify the `needsRetptr` convention ("stack[-1] is the retptr") matches wasmtime's `Func::call_raw` convention — this is a wazero engineering choice that must be consistent with the `canon.lower` core wasm function signature produced by `abi.CoreSignature`.

---

### Task C3: Change `ComponentLinker.DefineFunc` signature to require `*types.TypeFunc`

**Design reference:** Decision 6 (design lines 449-475); File Manifest (lines 1930-1932).
**Spec citation:** Wasmtime parallel `runtime/component/matching.rs:51` — "function implementation is missing" on `None` actual. Session 1 enforces typed host functions at registration time.
**Files modified:** `internal/component/component_linker.go`, `internal/component/linker.go`, every call site in `imports/wasip2/`, plus test fixtures that use the old signature.

- [ ] **Step C3.1: Audit current call sites**

```bash
grep -rn '\.DefineFunc(' internal/component/ imports/wasip2/ 2>&1 | head -50
grep -rn 'FuncNoType' internal/component/ imports/wasip2/ 2>&1 | head -30
```

Record every hit. The `ComponentLinker.DefineFunc` signature changes; the `Linker.DefineFunc` (if distinct) may already have a typed variant per the Checkpoint B audit. The `InstanceBuilder.FuncNoType` escape hatch is removed or gated behind a typed wrapper.

- [ ] **Step C3.2: Write failing test**

Create `internal/component/component_linker_definefunc_test.go`:

```go
package component

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/internal/component/types"
)

// TestComponentLinkerDefineFuncRequiresTypeFunc asserts ComponentLinker.DefineFunc
// rejects nil *types.TypeFunc at registration time.
//
// Spec: wasmtime matching.rs:51 (every actual must be typed) — wazero
// Session 1 Decision 6 enforces this at registration rather than at match.
func TestComponentLinkerDefineFuncRequiresTypeFunc(t *testing.T) {
	l := NewComponentLinker()
	fn := HostFunc(func(ctx context.Context, args []types.Val) ([]types.Val, error) {
		return nil, nil
	})
	// Nil type must be rejected.
	if err := l.DefineFunc("ns", "f", nil, fn); err == nil {
		t.Fatalf("DefineFunc(nil) = nil error, want rejection")
	}
	// Typed registration must succeed.
	builder := types.NewComponentTypesBuilder()
	ft := &types.TypeFunc{
		Params:  builder.InternTuple(nil),
		Results: builder.InternTuple(nil),
	}
	if err := l.DefineFunc("ns", "f", ft, fn); err != nil {
		t.Fatalf("DefineFunc(typed) = %v, want nil", err)
	}
}
```

- [ ] **Step C3.3: Run the test to confirm it fails**

```bash
go test ./internal/component/ -run TestComponentLinkerDefineFuncRequiresTypeFunc -count=1 2>&1 | tail -10
```

Expected: compile error `too many arguments in call to l.DefineFunc` OR `cannot use nil as HostFunc` depending on current signature order.

- [ ] **Step C3.4: Change the signature**

Edit `internal/component/component_linker.go`:

```go
// DefineFunc adds a host function definition with a required type.
// Spec: wasmtime matching.rs:51 (every host function must be typed).
// Session 1 Decision 6: the prior signature took an untyped HostFunc
// and relied on runtime type matching at import resolution time; that
// opens a gap where typos in import names surface as opaque trap
// messages. The typed signature rejects missing types at registration.
func (l *ComponentLinker) DefineFunc(namespace, name string, typ *types.TypeFunc, fn HostFunc) error {
	if typ == nil {
		return fmt.Errorf("DefineFunc: type is nil (every host function must declare a *types.TypeFunc)")
	}
	// Existing body, but store typ on the FuncDef.
	fd := &FuncDef{
		Type:     typ,
		Callback: fn,
	}
	key := namespace
	if name != "" {
		key = namespace + "#" + name
	}
	l.definitions[key] = fd
	return nil
}
```

(Adapt the body to whatever existing `definitions` map shape is currently in use.)

Apply the same signature change to `ComponentInstanceBuilder.Func` (line 112 area):

```go
func (b *ComponentInstanceBuilder) Func(name string, typ *types.TypeFunc, fn HostFunc) *ComponentInstanceBuilder {
	if typ == nil {
		b.err = fmt.Errorf("ComponentInstanceBuilder.Func %q: type is nil", name)
		return b
	}
	b.exports[name] = &FuncDef{Type: typ, Callback: fn}
	return b
}
```

- [ ] **Step C3.5: Migrate all `imports/wasip2/` call sites**

```bash
grep -rln '\.Func(' imports/wasip2/ 2>&1
```

For each file, open it and find every `InstanceBuilder.Func(name, fn)` or `ComponentInstanceBuilder.Func(name, fn)` call. Add a `*types.TypeFunc` argument constructed via a per-module builder. Example pattern (wasip2/io/streams.go):

```go
// Top of file — shared builder per host module.
var ioBuilder = types.NewComponentTypesBuilder()

// Somewhere at init/build time — construct the TypeFunc for each export.
var (
	inputStreamReadType = &types.TypeFunc{
		Params: ioBuilder.InternTuple([]types.ValType{
			{Kind: types.TypeKindOwn, Index: inputStreamResourceTableIdx},
			types.U64, // len
		}),
		Results: ioBuilder.InternTuple([]types.ValType{
			ioBuilder.InternResult(
				ioBuilder.InternList(types.U8),   // ok
				streamErrorType,                  // err
			),
		}),
	}
	// ... one per export ...
)

// At Func() call time:
builder.Func("read", inputStreamReadType, readImpl)
```

(The builder API shapes are from Session 0 — adapt to the actual builder method names.)

Every `imports/wasip2/` file that currently calls `.Func(name, fn)` must supply a typed `*types.TypeFunc`. Do NOT add a `FuncNoType` escape hatch; the whole point of Decision 6 is to eliminate untyped host functions.

- [ ] **Step C3.6: Build the full tree**

```bash
go build ./... 2>&1 | head -40
```

Expected: empty. Iterate until green.

- [ ] **Step C3.7: Run the test**

```bash
go test ./internal/component/ -run TestComponentLinkerDefineFuncRequiresTypeFunc -count=1 2>&1 | tail -10
```

Expected: PASS.

- [ ] **Step C3.8: Run per-task reviewers**

Dispatch both reviewers. Scope: `internal/component/component_linker.go` + `internal/component/linker.go` + every file in `imports/wasip2/` touched. Spec reviewer checklist item: "every wasip2 module constructs its `*types.TypeFunc` values via a single per-module ComponentTypesBuilder to avoid duplicate interned entries across modules."

---

### Task C4: Delete `coreSignature` panic stub; route callers to `abi.CoreSignature`

**Design reference:** Design line 1877 — "delete coreSignature entirely; callers route to abi.CoreSignature directly".
**Spec citation:** `definitions.py:flatten_functype` is the spec's core signature computation. `abi.CoreSignature` implements it (Session 0 Task 15).
**Files modified:** `internal/component/component_linker.go` (delete lines ~170-184).

- [ ] **Step C4.1: Audit callers of the stub**

```bash
grep -rn 'coreSignature(' internal/component/ | head
```

- [ ] **Step C4.2: For each caller, route directly to `abi.CoreSignature`**

```bash
grep -n 'func CoreSignature' internal/component/abi/*.go
```

Confirm the `abi.CoreSignature` entry point exists with a compatible signature. If the caller expected `(params, results []api.ValueType, needsRetptr bool)` the `abi.CoreSignature` signature should already produce that shape; adapt the call.

- [ ] **Step C4.3: Delete the stub**

Open `internal/component/component_linker.go` around line 177 and delete the `coreSignature` function entirely (including its comment block).

- [ ] **Step C4.4: Build**

```bash
go build ./internal/component/... 2>&1 | head -20
```

Expected: empty.

- [ ] **Step C4.5: Run per-task reviewers**

Dispatch both reviewers. Spec reviewer item: "confirm `abi.CoreSignature` implements `flatten_functype` from `definitions.py` without wazero-specific divergences; if divergences exist they must be pinned to a spec comment at the callsite."

---

### Task C5: Rebuild `Instantiate` — Stage 1: skeleton + resource binding + index spaces

**Design reference:** Decision 1 (design lines 129-140); Instantiate Pipeline (lines 829-913); Resource Identity section (lines 1138-1227).
**Spec citation:** `definitions.py:256-273` ComponentInstance shape. Wasmtime parallel: `runtime/component/instance.rs:710-931` Instantiator. `:912-931` `Instantiator::resource` → `bindResourceTypes`.
**Files modified:** `internal/component/component_linker.go`, `internal/component/instance.go` (resolve any method that needs to exist for `Instantiate` to compile).

- [ ] **Step C5.1: Write a failing minimal-Instantiate test**

Create `internal/component/instantiate_skeleton_test.go`:

```go
package component

import (
	"context"
	"testing"
)

// TestInstantiateSkeleton asserts ComponentLinker.Instantiate returns a
// non-nil Instance with a populated *runtime.ComponentInstance for a
// trivial component (no imports, no core modules, no resources).
//
// Spec: definitions.py:256-273 ComponentInstance shape.
// Wasmtime parallel: runtime/component/instance.rs:743 Instantiator::new.
func TestInstantiateSkeleton(t *testing.T) {
	// Construct a tiny compiled component with no imports and no core modules.
	// Use the binary-assembly helpers from the rest of the internal/component
	// tests to build a compiled.Internal() that has:
	//   - Types.ResourceTables: empty
	//   - Imports: empty
	//   - CoreModules: empty
	//   - NestedComponents: empty
	compiled := buildEmptyCompiledComponent(t)

	l := NewComponentLinker()
	inst, err := l.Instantiate(context.Background(), compiled)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if inst == nil {
		t.Fatalf("Instantiate returned nil instance")
	}
	if inst.Runtime() == nil {
		t.Fatalf("inst.Runtime() = nil; expected populated *runtime.ComponentInstance")
	}
}

// buildEmptyCompiledComponent constructs the minimal valid CompiledComponent
// for instantiation skeleton tests. See the existing decoder tests in
// binary/ for binary-assembly patterns.
func buildEmptyCompiledComponent(t *testing.T) *CompiledComponent {
	t.Helper()
	// Minimal binary: just the component-model header + empty sections.
	// The binary assembly helpers are defined in binary/ tests; reuse them
	// via a small in-package helper that packages them up.
	// (Implementation: assemble a 4-byte magic + 4-byte version + no sections,
	//  feed through Decode + NewCompiledComponent.)
	panic("implement buildEmptyCompiledComponent using binary assembly helpers from binary/ tests")
}
```

- [ ] **Step C5.2: Run the test to confirm it fails**

```bash
go test ./internal/component/ -run TestInstantiateSkeleton -count=1 2>&1 | tail -20
```

Expected: either the helper panics (if it's still unimplemented) or `Instantiate` returns the current `panic("compile-fix stub...")`.

Implement `buildEmptyCompiledComponent` using the actual binary assembly helpers — grep `internal/component/binary/` for an existing "build minimal component binary" helper, copy its pattern.

- [ ] **Step C5.3: Delete the `Instantiate` panic stub and implement the skeleton body**

Open `internal/component/component_linker.go` around line 146. Replace the panic stub with:

```go
// Instantiate creates a component instance from a compiled component.
// Spec: definitions.py:256-273 ComponentInstance + canon.lift/lower
// closure creation at :1978-2040 and :2064-2130.
// Wasmtime parallel: runtime/component/instance.rs:710-833 Instantiator.
//
// Session 1 rebuild: this is the 14-step pipeline from the design doc
// (design lines 829-913). Each helper method gets its own task in the
// implementation plan. This skeleton wires steps 1-3 and returns the
// fully-constructed (but not yet export-wired) Instance.
func (l *ComponentLinker) Instantiate(ctx context.Context, compiled *CompiledComponent) (*Instance, error) {
	if compiled == nil {
		return nil, fmt.Errorf("Instantiate: compiled is nil")
	}
	c := compiled.Internal()
	if c == nil {
		return nil, fmt.Errorf("Instantiate: compiled.Internal() is nil")
	}

	// Step 1 — Allocate instance + runtime.ComponentInstance.
	inst := newInstance(c, l.nextInstanceID(), nil)

	// Step 2 — Bind resource type declarations to runtime identities.
	// Matches wasmtime Instantiator::resource at instance.rs:912-931.
	// Session 1 Decision 2.
	if err := l.bindResourceTypes(inst, c); err != nil {
		return nil, fmt.Errorf("Instantiate: bind resource types: %w", err)
	}

	// Step 3 — Build index spaces from aliases (funcSpace, memSpace).
	// This is pre-existing logic; retarget to the new types.
	funcSpace := NewCoreFuncIndexSpace()
	memSpace := NewCoreMemoryIndexSpace()
	l.buildCoreIndexSpaces(c, funcSpace, memSpace)

	// Steps 4-14 land in subsequent tasks (C6..C11). For now,
	// Instantiate returns after step 3 and is extended task-by-task.
	_ = funcSpace
	_ = memSpace
	return inst, nil
}

// nextInstanceID returns the next monotonic instance ID for this linker.
// Session 1: simple counter on the linker; Session 2 may widen to a
// store-wide ID namespace when cross-instance resource lookup lands.
func (l *ComponentLinker) nextInstanceID() uint32 {
	l.instanceCounter++
	return l.instanceCounter
}

// bindResourceTypes walks c.Types.ResourceTables and mints one
// *runtime.ResourceType per declared resource, storing it in
// inst.rt.ResourceTypes in declaration order. Matches wasmtime
// Instantiator::resource at instance.rs:912-931.
//
// Spec: definitions.py:351-361 ResourceType {dtor, dtor_async, dtor_callback}.
// Wasmtime parallel: runtime/component/resources/ty.rs:68-79 ResourceType::guest.
func (l *ComponentLinker) bindResourceTypes(inst *Instance, c *Component) error {
	for rtIdx, table := range c.Types.ResourceTables {
		if table.Concrete {
			// Already concrete (cross-component imported resource). Session 1
			// does not overwrite; Session 2 handles cross-component matching.
			continue
		}
		// Locate destructor metadata for this declaration via c.TypeDefs.
		var dtor *uint32
		var dtorAsync bool
		var dtorCallback *uint32
		for _, td := range c.TypeDefs {
			if td.Kind == TypeDefKindResource && td.Resource == types.ResourceTableIdx(rtIdx) {
				dtor = td.ResourceDtor
				dtorAsync = td.ResourceDtorAsync
				dtorCallback = td.ResourceDtorCallback
				break
			}
		}
		rt := &runtime.ResourceType{
			Impl:         inst.rt,
			Dtor:         dtor,
			DtorAsync:    dtorAsync,
			DtorCallback: dtorCallback,
			// HostDestructor is nil for guest-declared resources. Host
			// resources (wasip2/io/*, etc.) set HostDestructor on their
			// own *ResourceType singletons constructed in package-init,
			// not here.
		}
		inst.rt.ResourceTypes = append(inst.rt.ResourceTypes, rt)
	}
	return nil
}

// buildCoreIndexSpaces is a stub for Task C5; the full body lands with
// the rest of the pipeline in Tasks C6-C11.
func (l *ComponentLinker) buildCoreIndexSpaces(c *Component, funcSpace *CoreFuncIndexSpace, memSpace *CoreMemoryIndexSpace) {
	// Step 3 is populated in Task C6.
}
```

**Note on `instanceCounter`:** add a `instanceCounter uint32` field to `ComponentLinker` if it doesn't already exist. Initialize to 0.

**Note on `CoreFuncIndexSpace` / `CoreMemoryIndexSpace`:** these are existing types in the package; if they are not, define them in this task with empty bodies and populate in C6.

- [ ] **Step C5.4: Run the skeleton test to confirm it passes**

```bash
go build ./internal/component/... 2>&1 | head -20 && \
  go test ./internal/component/ -run TestInstantiateSkeleton -count=1 2>&1 | tail -10
```

Expected: build empty, skeleton test PASS.

- [ ] **Step C5.5: Run per-task reviewers**

Dispatch both reviewers. Spec reviewer item: "confirm bindResourceTypes walks c.Types.ResourceTables in declaration order and that the `continue` for `table.Concrete = true` is spec-correct for Session 1 scope."

---

### Task C6: Rebuild `Instantiate` — Stage 2: imports + type checking + value imports + type space

**Design reference:** Instantiate Pipeline steps 4-8 (design lines 855-875).
**Spec citation:** `definitions.py` import-matching via spec type equivalence rules. Wasmtime parallel: `runtime/component/matching.rs` for import type-matching.
**Files modified:** `internal/component/component_linker.go` (extend Instantiate body and add helpers).

- [ ] **Step C6.1: Write failing test**

Add to `internal/component/instantiate_skeleton_test.go`:

```go
// TestInstantiateWithTypedImport asserts Instantiate resolves a typed
// import from the linker registry and type-checks it.
//
// Spec: definitions.py import type matching (component-model import
// subtyping rules).
// Wasmtime parallel: matching.rs:51 function import matching.
func TestInstantiateWithTypedImport(t *testing.T) {
	compiled := buildComponentWithOneTypedImport(t, "ns", "f")
	l := NewComponentLinker()
	// Register the import with a matching type.
	builder := types.NewComponentTypesBuilder()
	ft := &types.TypeFunc{
		Params:  builder.InternTuple([]types.ValType{types.S32}),
		Results: builder.InternTuple([]types.ValType{types.S32}),
	}
	l.DefineFunc("ns", "f", ft, func(ctx context.Context, args []types.Val) ([]types.Val, error) {
		return args, nil
	})
	inst, err := l.Instantiate(context.Background(), compiled)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if inst == nil {
		t.Fatalf("Instantiate returned nil")
	}
}

func buildComponentWithOneTypedImport(t *testing.T, ns, name string) *CompiledComponent {
	t.Helper()
	// Assemble a minimal binary with one imported function `ns#f: (i32) -> i32`.
	// Follow the same binary-assembly pattern as buildEmptyCompiledComponent.
	panic("implement buildComponentWithOneTypedImport")
}
```

- [ ] **Step C6.2: Run the test to confirm it fails**

```bash
go test ./internal/component/ -run TestInstantiateWithTypedImport -count=1 2>&1 | tail -10
```

Expected: either the helper panics or the `Instantiate` body does not yet resolve imports.

- [ ] **Step C6.3: Implement steps 4-8**

Extend the `Instantiate` body in `component_linker.go`:

```go
// Step 4 — Resolve and type-check imports.
tc := NewTypeChecker(c)
resolvedImports := make(map[string]Definition)
instanceToImport := make(map[uint32]string)
if err := l.resolveAndCheckImports(c, tc, resolvedImports, instanceToImport); err != nil {
	return nil, fmt.Errorf("Instantiate: resolve imports: %w", err)
}

// Step 5 — Populate value index space from value imports.
l.populateValueImports(inst, c, resolvedImports)

// Step 6 — Align instance index space with instance imports.
l.alignInstanceImports(inst, c)

// Step 7 — Build component function index space.
l.buildComponentFuncs(inst, c, resolvedImports, instanceToImport)

// Step 8 — Build type index space for nested instantiation arg resolution.
l.buildTypeSpace(inst, c)
```

Helper bodies (add below `Instantiate`):

```go
// resolveAndCheckImports walks c.Imports, resolves each from the
// linker's definitions, and type-checks the resolved definition against
// the expected extern-desc type.
//
// Spec: component-model import type matching (Explainer.md :920-982).
// Wasmtime parallel: runtime/component/matching.rs:51-162.
func (l *ComponentLinker) resolveAndCheckImports(
	c *Component,
	tc *TypeChecker,
	resolvedImports map[string]Definition,
	instanceToImport map[uint32]string,
) error {
	for _, imp := range c.Imports {
		def, err := l.MatchImport(imp.Name)
		if err != nil {
			return fmt.Errorf("import %q: %w", imp.Name, err)
		}
		if err := tc.Check(&imp.Desc, def); err != nil {
			return fmt.Errorf("import %q: type check: %w", imp.Name, err)
		}
		resolvedImports[imp.Name] = def
		if imp.Desc.Kind == ImportExternDescInstance {
			// Track which instance-space slot this import occupies for
			// nested wiring.
			instanceToImport[imp.InstanceIdx] = imp.Name
		}
	}
	return nil
}

// populateValueImports fills inst.values with value imports (constants).
func (l *ComponentLinker) populateValueImports(inst *Instance, c *Component, resolvedImports map[string]Definition) {
	for _, imp := range c.Imports {
		if imp.Desc.Kind != ImportExternDescValue {
			continue
		}
		vd, ok := resolvedImports[imp.Name].(*ValueDef)
		if !ok {
			continue
		}
		for int(imp.ValueIdx) >= len(inst.values) {
			inst.values = append(inst.values, types.Val{})
			inst.valuesConsumed = append(inst.valuesConsumed, false)
		}
		inst.values[imp.ValueIdx] = vd.Value
	}
}

// alignInstanceImports ensures inst.instanceSpace has a slot for every
// instance import (populated from resolvedImports via a later wiring step).
func (l *ComponentLinker) alignInstanceImports(inst *Instance, c *Component) {
	for _, imp := range c.Imports {
		if imp.Desc.Kind != ImportExternDescInstance {
			continue
		}
		for int(imp.InstanceIdx) >= len(inst.instanceSpace) {
			inst.instanceSpace = append(inst.instanceSpace, nil)
		}
	}
}

// buildComponentFuncs populates inst.componentFuncs from canon.lift
// declarations + aliases + resolved function imports.
func (l *ComponentLinker) buildComponentFuncs(
	inst *Instance,
	c *Component,
	resolvedImports map[string]Definition,
	instanceToImport map[uint32]string,
) {
	// Walk c.Canonical entries (canon.lift declarations).
	for _, canon := range c.Canonical {
		if canon.Kind != CanonKindLift {
			continue
		}
		// Resolve canon.TypeIdx → *types.TypeFunc via c.TypeDefs.
		td := c.TypeDefs[canon.TypeIdx]
		if td.Kind != TypeDefKindFunc {
			continue
		}
		funcType := td.FuncType(c)
		inst.componentFuncs[canon.FuncIdx] = ComponentFunc{
			Type: funcType,
			Impl: nil, // filled by wireExports after core modules instantiate
		}
	}

	// Walk function imports and populate componentFuncs for those slots.
	for _, imp := range c.Imports {
		if imp.Desc.Kind != ImportExternDescFunc {
			continue
		}
		fd, ok := resolvedImports[imp.Name].(*FuncDef)
		if !ok || fd == nil {
			continue
		}
		inst.componentFuncs[imp.FuncIdx] = ComponentFunc{
			Type: fd.Type,
			Impl: fd,
		}
	}
}

// buildTypeSpace populates inst.typeSpace in declaration order from
// c.TypeDefs. Nested instantiations read from this slice when they
// resolve type arguments from the parent scope.
func (l *ComponentLinker) buildTypeSpace(inst *Instance, c *Component) {
	inst.typeSpace = make([]*TypeDef, len(c.TypeDefs))
	for i := range c.TypeDefs {
		inst.typeSpace[i] = &c.TypeDefs[i]
	}
}
```

- [ ] **Step C6.4: Run tests**

```bash
go build ./internal/component/... 2>&1 | head -20 && \
  go test ./internal/component/ -run 'TestInstantiate(Skeleton|WithTypedImport)' -count=1 2>&1 | tail -10
```

Expected: both pass.

- [ ] **Step C6.5: Run per-task reviewers**

Dispatch both reviewers.

---

### Task C7: Rebuild `Instantiate` — Stage 3: nested components + canon maps

**Design reference:** Instantiate Pipeline steps 9-11 (design lines 876-889); Nested component instantiation (lines 1134-1137).
**Spec citation:** Component-model nested instantiation semantics (Explainer.md `:1020+` component instantiation).
**Files modified:** `internal/component/component_linker.go`, `internal/component/nested_component.go` (add helper stub; full body in Checkpoint D).

- [ ] **Step C7.1: Extend `Instantiate` body with steps 9-11**

Append to the `Instantiate` body after Step 8:

```go
// Step 9 — Process nested component instances.
componentInstDefs, err := l.processNestedInstances(ctx, inst, c)
if err != nil {
	return nil, fmt.Errorf("Instantiate: nested instances: %w", err)
}

// Step 10 — Build canon lower / canon resource info maps from CanonicalDef.
canonLowers, canonResources := l.buildCanonMaps(c)

// Step 11 — Build function alias map for inline instance resolution.
funcAliases := l.buildFuncAliases(c)

_ = componentInstDefs
_ = canonLowers
_ = canonResources
_ = funcAliases
```

Helper bodies:

```go
// processNestedInstances handles nested component instantiation. The
// full restoration of instantiateNestedComponent lives in Checkpoint D.
// Session 1 Stage 3 returns the placeholder empty map and traps at the
// first nested instance if one exists.
func (l *ComponentLinker) processNestedInstances(ctx context.Context, inst *Instance, c *Component) (map[uint32]*InstanceDef, error) {
	componentInstDefs := make(map[uint32]*InstanceDef)
	for _, ci := range c.ComponentInstances {
		// The full nested-instantiation body is restored in Checkpoint D.
		// Until then Session 1 handles trivially-aliasing cases but
		// traps on real nested instantiation.
		if ci.Kind == ComponentInstanceKindAlias {
			continue
		}
		return nil, fmt.Errorf(
			"nested component instantiation: rebuild in progress (Session 1 Checkpoint D Task D2). ComponentInstance kind %v at index %d", ci.Kind, ci.Index)
	}
	return componentInstDefs, nil
}

// buildCanonMaps indexes canon.lower / canon.resource.* declarations by
// the core function slot they occupy. The returned maps are consumed
// during core module instantiation to wire host module exports.
func (l *ComponentLinker) buildCanonMaps(c *Component) (map[uint32]canonLowerInfo, map[uint32]canonResourceInfo) {
	lowers := make(map[uint32]canonLowerInfo)
	resources := make(map[uint32]canonResourceInfo)
	for _, canon := range c.Canonical {
		switch canon.Kind {
		case CanonKindLower:
			lowers[canon.CoreFuncIdx] = canonLowerInfo{
				funcIdx:  canon.FuncIdx,
				typeIdx:  canon.TypeIdx,
				options:  canon.Options,
			}
		case CanonKindResourceNew, CanonKindResourceDrop, CanonKindResourceRep:
			resources[canon.CoreFuncIdx] = canonResourceInfo{
				kind:    canon.Kind,
				typeIdx: canon.TypeIdx,
			}
		}
	}
	return lowers, resources
}

// buildFuncAliases indexes function aliases by their alias FuncIdx.
func (l *ComponentLinker) buildFuncAliases(c *Component) map[uint32]*Alias {
	aliases := make(map[uint32]*Alias)
	for i := range c.Aliases {
		a := &c.Aliases[i]
		if a.Kind == AliasKindCoreFunc || a.Kind == AliasKindFunc {
			aliases[a.Index] = a
		}
	}
	return aliases
}

type canonLowerInfo struct {
	funcIdx uint32
	typeIdx uint32
	options CanonicalOptions
}

type canonResourceInfo struct {
	kind    CanonKind
	typeIdx uint32
}
```

Adapt field names to match the actual `CanonicalDef` / `Canonical` shape in the current codebase. Grep `grep -n 'type CanonicalDef\|Canonical \[\]' internal/component/component.go` to verify.

- [ ] **Step C7.2: Run the skeleton + typed-import tests**

```bash
go test ./internal/component/ -run 'TestInstantiate(Skeleton|WithTypedImport)' -count=1 2>&1 | tail -10
```

Expected: both still pass.

- [ ] **Step C7.3: Run per-task reviewers**

Dispatch both reviewers.

---

### Task C8: Rebuild `Instantiate` — Stage 4: core module instantiation + host module exports

**Design reference:** Instantiate Pipeline steps 12-14 (design lines 890-909); canon.lift wiring (lines 915-1021); canon.lower wiring (lines 1040-1132); Canon Resource Ops (lines 1249-1259).
**Spec citation:** `definitions.py:1978-2040` canon_lift full flow. `:2064-2130` canon_lower full flow. `:2134-2173` canon_resource_*.
**Files modified:** `internal/component/component_linker.go` (extend Instantiate + add `buildCanonLiftFunc`, `createCanonLowerFunc`, `createResourceOpExport`, `instantiateCoreModules`, `wireExports`).

This is the largest task in Checkpoint C. Break into sub-steps.

- [ ] **Step C8.1: Write failing end-to-end integration test**

Create `internal/component/instantiate_end_to_end_test.go`:

```go
package component

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/internal/component/types"
)

// TestInstantiateAndCallLiftedFunc asserts a component that exports a
// single function `add(s32, s32) -> s32` implemented in core wasm can be
// instantiated and called through the lifted entry point.
//
// Spec: definitions.py:1978-2040 canon_lift full flow.
// Canonical test: run_tests.py test_pairs (primitive round-trips).
// Wasmtime parallel: runtime/component/func.rs Func::call (call flow :232-706).
func TestInstantiateAndCallLiftedFunc(t *testing.T) {
	compiled := buildComponentWithAddExport(t)
	l := NewComponentLinker()
	inst, err := l.Instantiate(context.Background(), compiled)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	addFn, ok := inst.exports["add"]
	if !ok {
		t.Fatalf("missing export add")
	}
	results, err := addFn.Call(context.Background(), types.ValS32(7), types.ValS32(35))
	if err != nil {
		t.Fatalf("add.Call: %v", err)
	}
	if len(results) != 1 || results[0].S32() != 42 {
		t.Fatalf("add(7, 35) = %v, want 42", results)
	}
}

func buildComponentWithAddExport(t *testing.T) *CompiledComponent {
	t.Helper()
	// Assemble a minimal component that:
	//   - Imports no imports
	//   - Contains a core module exporting `(func (param i32 i32) (result i32))`
	//     that returns the sum of its two params
	//   - Has a canon.lift declaration wrapping the core function as a
	//     component function with type `(s32, s32) -> s32`
	//   - Exports the lifted function as "add"
	panic("implement buildComponentWithAddExport using core-wasm assembly helpers from binary/ tests")
}
```

- [ ] **Step C8.2: Run the test to confirm it fails**

```bash
go test ./internal/component/ -run TestInstantiateAndCallLiftedFunc -count=1 2>&1 | tail -20
```

Expected: either helper panics, or `Instantiate` returns without wiring exports.

- [ ] **Step C8.3: Implement `instantiateCoreModules`**

Append to `Instantiate` body after Step 11:

```go
// Step 12 — Instantiate core modules with wired host module exports.
if err := l.instantiateCoreModules(
	ctx, inst, c, compiled.CompiledModules(),
	resolvedImports, canonLowers, canonResources, funcAliases,
); err != nil {
	return nil, fmt.Errorf("Instantiate: core modules: %w", err)
}

// Step 13 — Execute start function if declared.
if err := l.executeStartFunction(ctx, inst, c); err != nil {
	return nil, fmt.Errorf("Instantiate: start function: %w", err)
}

// Step 14 — Wire exports (ExportedFunc per exported function, Instance per exported instance).
if err := l.wireExports(inst, c, componentInstDefs, funcSpace, memSpace); err != nil {
	return nil, fmt.Errorf("Instantiate: wire exports: %w", err)
}

return inst, nil
```

Add helper bodies:

```go
// instantiateCoreModules walks c.CoreInstances, constructs a host module
// per core module providing the canon.lower + canon.resource.* + alias
// exports needed by the core module's imports, and instantiates the
// core module via the wazero runtime.
//
// Spec: definitions.py:2064-2130 canon_lower (for the host-module
// canon.lower exports wired here) + :2134-2173 canon_resource_*.
// Wasmtime parallel: runtime/component/instance.rs:800-900 core module
// instantiation and host-function wiring.
func (l *ComponentLinker) instantiateCoreModules(
	ctx context.Context,
	inst *Instance,
	c *Component,
	compiledModules []api.CompiledModule,
	resolvedImports map[string]Definition,
	canonLowers map[uint32]canonLowerInfo,
	canonResources map[uint32]canonResourceInfo,
	funcAliases map[uint32]*Alias,
) error {
	for ciIdx, ci := range c.CoreInstances {
		if ci.Kind != CoreInstanceKindInline {
			continue
		}
		cm := compiledModules[ci.ModuleIdx]
		// Build the host module providing canon.lower / canon.resource.* /
		// aliased exports for this core module's imports.
		hostModule := l.buildCoreHostModule(inst, c, ci, canonLowers, canonResources, funcAliases, resolvedImports)
		// Register the host module with the runtime, then instantiate the core module.
		mod, err := l.runtime.InstantiateModuleWithHost(ctx, cm, hostModule)
		if err != nil {
			return fmt.Errorf("core instance %d: %w", ciIdx, err)
		}
		for int(ciIdx) >= len(inst.coreInstances) {
			inst.coreInstances = append(inst.coreInstances, nil)
		}
		inst.coreInstances[ciIdx] = mod
	}
	return nil
}
```

Note: `l.runtime` and `InstantiateModuleWithHost` are placeholder names — adapt to the actual wazero core runtime API. Grep `grep -n 'InstantiateModule\|NewHostModule\|HostModuleBuilder' internal/component/component_linker.go` to find the current pattern.

- [ ] **Step C8.4: Implement `buildCanonLiftFunc` closure**

Add to `component_linker.go`:

```go
// buildCanonLiftFunc creates the closure that implements a canon.lift
// component function. Matches spec canon_lift at definitions.py:1978-2040
// for synchronous calls.
//
// Step-by-step spec mapping:
//   :1979  trap_if(call_might_be_recursive(caller, inst))
//   :1989  args = on_start()
//   :1990  flat_args = lower_flat_values(cx, MAX_FLAT_PARAMS, args, ft.param_types())
//            (:1955/:1973 toggle may_leave around the call)
//   :1995  flat_results = call_and_trap_on_throw(callee, thread, flat_args)
//   :1997  result = lift_flat_values(cx, MAX_FLAT_RESULTS, iter(flat_results), ft.result_type())
//   :2000-2002  may_leave = False around post_return, then True again
//   :738-742  deliver_resolve — close borrow scope, undo lends
func (l *ComponentLinker) buildCanonLiftFunc(
	inst *Instance,
	canon *CanonicalDef,
	coreFunc api.Function,
	funcType *types.TypeFunc,
	memory api.Memory,
	realloc api.Function,
	postReturn api.Function,
) func(goCtx context.Context, caller *Instance, args []types.Val) ([]types.Val, error) {
	return func(goCtx context.Context, caller *Instance, args []types.Val) ([]types.Val, error) {
		// :1979 trap_if(call_might_be_recursive).
		if caller != nil && inst.CallMightBeRecursive(caller) {
			return nil, errReentrance
		}
		inst.rt.Reentrance.EnterInstance(inst.rt.ID)
		defer inst.rt.Reentrance.LeaveInstance(inst.rt.ID)

		opts := buildAbiOptions(canon, memory, realloc)
		liftCtx := &abi.LiftContext{
			Memory:      memory,
			Opts:        opts,
			Types:       inst.component.Types,
			Instance:    inst.rt,
			BorrowScope: runtime.NewBorrowScope(inst.rt.Table),
		}
		lowerCtx := &abi.LowerContext{
			Memory:      memory,
			Opts:        opts,
			Realloc:     reallocAdapter(realloc),
			Types:       inst.component.Types,
			Instance:    inst.rt,
			CallContext: runtime.NewCallContext(),
		}

		paramTypes := unpackTupleElems(inst.component.Types, funcType.Params)
		resultTypes := unpackTupleElems(inst.component.Types, funcType.Results)

		// :1955/:1973 may_leave toggle around the aggregate lower.
		inst.rt.MayLeave = false
		flatArgs, err := abi.LowerParams(lowerCtx, paramTypes, args, abi.MaxFlatParams)
		inst.rt.MayLeave = true
		if err != nil {
			return nil, fmt.Errorf("canon.lift: lower params: %w", err)
		}

		// :1995 invoke the core callee.
		flatResults, err := coreFunc.Call(goCtx, flatArgs...)
		if err != nil {
			return nil, fmt.Errorf("canon.lift: core call: %w", err)
		}

		// :1997 lift results.
		results, err := abi.LiftResults(liftCtx, resultTypes, flatResults, abi.MaxFlatResults)
		if err != nil {
			return nil, fmt.Errorf("canon.lift: lift results: %w", err)
		}

		// :2000-2002 post_return with may_leave toggle.
		if postReturn != nil {
			inst.rt.MayLeave = false
			_, perr := postReturn.Call(goCtx, flatResults...)
			inst.rt.MayLeave = true
			if perr != nil {
				return nil, fmt.Errorf("canon.lift: post_return: %w", perr)
			}
		}

		// :738-742 deliver_resolve → close borrow scope.
		if err := liftCtx.BorrowScope.Release(); err != nil {
			return nil, fmt.Errorf("canon.lift: borrow scope: %w", err)
		}

		return results, nil
	}
}

// errReentrance is the sentinel for spec :1979.
var errReentrance = errors.New("canon.lift: call would be recursive (call_might_be_recursive)")
```

**Helper references:** `buildAbiOptions`, `reallocAdapter`, `unpackTupleElems` are new small helpers. Add them next to `buildCanonLiftFunc`:

```go
func buildAbiOptions(canon *CanonicalDef, memory api.Memory, realloc api.Function) abi.Options {
	return abi.Options{
		StringEncoding: canon.Options.StringEncoding,
		// ... other option fields from canon.Options ...
	}
}

func reallocAdapter(realloc api.Function) func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
	if realloc == nil {
		return nil
	}
	return func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
		res, err := realloc.Call(context.Background(), uint64(oldPtr), uint64(oldSize), uint64(align), uint64(newSize))
		if err != nil {
			return 0, err
		}
		return uint32(res[0]), nil
	}
}

func unpackTupleElems(bag *types.ComponentTypes, v types.ValType) []types.ValType {
	if v.Kind != types.TypeKindTuple {
		return nil
	}
	return bag.Tuples[v.Index].Types
}
```

Adapt `abi.Options` / `bag.Tuples[].Types` to the actual field names in the current codebase.

- [ ] **Step C8.5: Implement `createCanonLowerFunc` closure**

```go
// createCanonLowerFunc produces the api.GoModuleFunc implementing a
// canon.lower core wasm function. Matches spec canon_lower at
// definitions.py:2064-2130.
//
// :2065  trap_if(not caller_task.inst.may_leave)
// :2068-2070  create Subtask + LiftLowerContext
// :2089  lift args from the flat iterator
// :2095  invoke callee (component function)
// :2113  deliver_resolve → close borrow scope
func (l *ComponentLinker) createCanonLowerFunc(
	inst *Instance,
	c *Component,
	info canonLowerInfo,
	compFunc ComponentFunc,
	paramTypes []types.ValType,
	resultTypes []types.ValType,
	needsRetptr bool,
) api.GoModuleFunc {
	return api.GoModuleFunc(func(goCtx context.Context, mod api.Module, stack []uint64) {
		memory := mod.Memory()
		realloc := resolveReallocFunction(mod, info.options)

		// :2065 trap_if(not may_leave).
		if !inst.rt.IsMayLeave() {
			panic(errMayNotLeave)
		}

		opts := buildAbiOptionsFromCanonOptions(info.options, memory, realloc)
		borrowScope := runtime.NewBorrowScope(inst.rt.Table)
		liftCtx := &abi.LiftContext{
			Memory:      memory,
			Opts:        opts,
			Types:       c.Types,
			Instance:    inst.rt,
			BorrowScope: borrowScope,
		}
		lowerCtx := &abi.LowerContext{
			Memory:      memory,
			Opts:        opts,
			Realloc:     reallocAdapter(realloc),
			Types:       c.Types,
			Instance:    inst.rt,
			CallContext: runtime.NewCallContext(),
		}

		// :2089 lift args from flat iterator.
		args, err := abi.LiftParams(liftCtx, paramTypes, stack, abi.MaxFlatParams)
		if err != nil {
			panic(fmt.Errorf("canon.lower: lift params: %w", err))
		}

		// :2095 invoke callee.
		var results []types.Val
		if compFunc.Impl != nil {
			// Host-provided function.
			results, err = compFunc.Impl.(*FuncDef).Callback(goCtx, args)
		} else {
			// Lifted guest function.
			// Session 1 wires host-imported functions; cross-component
			// canon.lower of a guest function is Checkpoint D / Session 2.
			err = fmt.Errorf("canon.lower: guest-function callee (cross-component) deferred to Session 2")
		}
		if err != nil {
			panic(fmt.Errorf("canon.lower: callee: %w", err))
		}

		// Lower results with may_leave toggle.
		inst.rt.MayLeave = false
		lerr := abi.LowerResults(lowerCtx, resultTypes, results, stack, needsRetptr, abi.MaxFlatResults)
		inst.rt.MayLeave = true
		if lerr != nil {
			panic(fmt.Errorf("canon.lower: lower results: %w", lerr))
		}

		// :2113 deliver_resolve.
		if err := borrowScope.Release(); err != nil {
			panic(fmt.Errorf("canon.lower: borrow scope: %w", err))
		}
	})
}
```

- [ ] **Step C8.6: Implement `createResourceOpExport`**

```go
// createResourceOpExport constructs the host module export implementing
// canon.resource.new / canon.resource.drop / canon.resource.rep for a
// resource declaration. The returned export has the core wasm signatures
// unchanged from the pre-Session-0 shape:
//
//   canon resource.new $T : (i32 rep) -> i32 handle
//   canon resource.drop $T : (i32 handle) -> ()
//   canon resource.rep $T : (i32 handle) -> i32 rep
//
// Spec: definitions.py:2134-2138 canon_resource_new, :2142-2165
// canon_resource_drop, :2169-2173 canon_resource_rep.
func (l *ComponentLinker) createResourceOpExport(
	inst *Instance,
	name string,
	info canonResourceInfo,
) *HostModuleExport {
	td := inst.component.TypeDefs[info.typeIdx]
	if td.Kind != TypeDefKindResource {
		return nil
	}
	// Resolve the ResourceTableIdx → ResourceIdx slot in rt.ResourceTypes.
	// Session 1 convention: ResourceTables are walked in declaration order
	// by bindResourceTypes, so rtTableIdx IS the slot index into
	// rt.ResourceTypes (design Resource Identity section, lines 1212-1218).
	resourceIdx := types.ResourceIdx(td.Resource)

	switch info.kind {
	case CanonKindResourceNew:
		return &HostModuleExport{
			Name:        name,
			ParamTypes:  []api.ValueType{api.ValueTypeI32},
			ResultTypes: []api.ValueType{api.ValueTypeI32},
			Func: api.GoModuleFunc(func(goCtx context.Context, mod api.Module, stack []uint64) {
				rep := uint32(stack[0])
				h, err := inst.ResourceNew(resourceIdx, rep)
				if err != nil {
					panic(err)
				}
				stack[0] = uint64(h)
			}),
		}
	case CanonKindResourceDrop:
		return &HostModuleExport{
			Name:        name,
			ParamTypes:  []api.ValueType{api.ValueTypeI32},
			ResultTypes: nil,
			Func: api.GoModuleFunc(func(goCtx context.Context, mod api.Module, stack []uint64) {
				handleIdx := uint32(stack[0])
				if err := inst.ResourceDrop(resourceIdx, handleIdx); err != nil {
					panic(err)
				}
			}),
		}
	case CanonKindResourceRep:
		return &HostModuleExport{
			Name:        name,
			ParamTypes:  []api.ValueType{api.ValueTypeI32},
			ResultTypes: []api.ValueType{api.ValueTypeI32},
			Func: api.GoModuleFunc(func(goCtx context.Context, mod api.Module, stack []uint64) {
				handleIdx := uint32(stack[0])
				rep, err := inst.ResourceRep(resourceIdx, handleIdx)
				if err != nil {
					panic(err)
				}
				stack[0] = uint64(rep)
			}),
		}
	}
	return nil
}
```

**Note:** This references `inst.ResourceNew` / `ResourceRep` / `ResourceDrop` with the Session 1 spec-matching signatures. Those bodies are rewritten in Task E5. Until then (Checkpoints C + D), the stub bodies from Task B4 return precise "rebuild in progress" errors. This is acceptable because the integration test in Checkpoint C (`TestInstantiateAndCallLiftedFunc`) uses a component with NO resources — the `createResourceOpExport` code path is compiled but not executed.

- [ ] **Step C8.7: Implement `wireExports`, `executeStartFunction`, `buildCoreHostModule`**

Add helpers:

```go
// wireExports populates inst.exports with an ExportedFunc per exported
// component function. Each ExportedFunc wraps a canon.lift closure
// built via buildCanonLiftFunc.
func (l *ComponentLinker) wireExports(
	inst *Instance,
	c *Component,
	componentInstDefs map[uint32]*InstanceDef,
	funcSpace *CoreFuncIndexSpace,
	memSpace *CoreMemoryIndexSpace,
) error {
	for _, exp := range c.Exports {
		switch exp.Kind {
		case ExportKindFunc:
			// Look up the component function.
			cf, ok := inst.componentFuncs[exp.Index]
			if !ok {
				return fmt.Errorf("export %q: function index %d not wired", exp.Name, exp.Index)
			}
			// If this function was produced by canon.lift of a core function,
			// the canon.lift closure is already attached to cf.Impl by the
			// core-module instantiation step. Otherwise the function is a
			// re-export of an imported host function.
			inst.exports[exp.Name] = &ExportedFunc{
				name:     exp.Name,
				funcType: cf.Type,
				impl:     cf.Impl,
			}
		case ExportKindInstance:
			// Construct a sub-Instance wrapper for the exported instance.
			// Full body in Checkpoint D.
			return fmt.Errorf("instance export %q: Checkpoint D wiring", exp.Name)
		}
	}
	return nil
}

// executeStartFunction invokes the component's start function if declared.
// Spec: component-model start function calls the declared function once
// after instantiation, consuming values from the value index space.
func (l *ComponentLinker) executeStartFunction(ctx context.Context, inst *Instance, c *Component) error {
	if c.Start == nil {
		return nil
	}
	// Full start function execution lands in Checkpoint D (nested components
	// + start function tests). For Session 1 Checkpoint C, components
	// without a start function still pass; components with one get the
	// Checkpoint D body.
	return nil
}

// buildCoreHostModule constructs the per-core-instance host module
// providing canon.lower / canon.resource.* / alias exports. The host
// module is scoped to one core instance.
func (l *ComponentLinker) buildCoreHostModule(
	inst *Instance,
	c *Component,
	ci CoreInstance,
	canonLowers map[uint32]canonLowerInfo,
	canonResources map[uint32]canonResourceInfo,
	funcAliases map[uint32]*Alias,
	resolvedImports map[string]Definition,
) *HostModule {
	mod := NewHostModule()
	// Walk the core module's imports and resolve each to a host export.
	// (Full body: read the pre-Session-0 buildCoreHostModule for the
	//  import-walk pattern; rewrite against new types.)
	// ... body lives in the next sub-step because it's lengthy ...
	return mod
}
```

- [ ] **Step C8.8: Finish `buildCoreHostModule` by wiring canon.lower + canon.resource exports**

Fill in the body to walk the core module's imports (accessible via `cm.ImportedFunctions()` or similar wazero runtime API — grep the current codebase to find the pattern) and for each import, find the matching `canonLowerInfo` / `canonResourceInfo` in the maps and call `createCanonLowerFunc` or `createResourceOpExport` to obtain the host function.

The exact walk depends on how wazero currently associates core import names with `CanonicalDef` entries — this association is pre-existing in the repo and must be preserved. Grep:

```bash
grep -n 'canon.*lower\|canon.*resource' internal/component/ | grep -v _test.go | head -20
```

- [ ] **Step C8.9: Run the end-to-end test**

```bash
go build ./internal/component/... 2>&1 | head -40 && \
  go test ./internal/component/ -run 'TestInstantiateAndCallLiftedFunc' -count=1 2>&1 | tail -20
```

Expected: PASS (the `add(s32, s32) -> s32` component instantiates, and the lifted export runs the core function successfully).

If the test fails, diagnose:
- `buildComponentWithAddExport` helper producing a malformed binary? Compare against the decoder tests in Checkpoint A.
- Host module wiring missing the canon.lift closure attachment? Add it.
- `LowerParams` / `LiftResults` bug? Re-run the unit tests from C1/C2.

- [ ] **Step C8.10: Run per-task reviewers**

Dispatch both reviewers. Spec reviewer MUST verify the may_leave toggle is applied in exactly the two sites the spec prescribes (`:1955/:1973` and `:2000/:2002`) and not elsewhere.

---

### Task C9: Restore `linker_test.go` (34 tests)

**Design reference:** File Manifest (design line 1940).
**Spec citation:** Component-model host-function linking semantics; wasmtime `linker.rs` parallel.
**Files modified:** `internal/component/linker_test.go`.

- [ ] **Step C9.1: Locate skipped tests**

```bash
grep -n 't\.Skip.*session 1 work' internal/component/linker_test.go
```

Expected: 34 hits.

- [ ] **Step C9.2: Pull pre-Session-0 body**

```bash
git show 98b3bbc3:internal/component/linker_test.go > /tmp/old_linker_test.go
wc -l /tmp/old_linker_test.go
```

- [ ] **Step C9.3: Restore one test at a time using the Test Restoration Methodology**

For each of the 34 functions, follow the 8-step methodology:

1. **Dedup check:** `grep -rn '<test name>' internal/component/ | head`
2. **Old body:** from `/tmp/old_linker_test.go`
3. **Upstream source:** wasmtime `runtime/component/linker.rs` for linker behavior; `definitions.py` does not cover linker specifics (linker is host-facing, not core spec).
4. **Port canonical cases:** N/A for linker (no run_tests.py counterpart; wasmtime test files under `wasmtime/tests/all/component_model/` may cover specific linker behaviors — consult on a per-test basis).
5. **Citation block:** example format:
   ```go
   // TestLinkerDefineFunc asserts Linker.DefineFunc stores a typed host
   // function and MatchImport resolves it.
   //
   // Wasmtime parallel: runtime/component/linker.rs Linker::define.
   // Wasmtime test: wasmtime/tests/all/component_model/import.rs (host import patterns).
   // No counterpart (justified): canonical-abi run_tests.py does not
   // exercise the host linker; linker semantics are a wazero/wasmtime
   // embedder-facing layer outside the canonical ABI scope.
   ```
6. **Validate assertions:** each `require.Equal` / `require.Error` must match the wasmtime linker behavior. Reword or delete any that contradict.
7. **Run the single test:**
   ```bash
   go test ./internal/component/ -run TestLinkerXxx -count=1 2>&1 | tail -10
   ```
8. **Delete the `t.Skip`** and verify.

Commit after every 5-10 tests. Each commit message lists the tests restored and their upstream source.

- [ ] **Step C9.4: Run the full file**

```bash
go test ./internal/component/ -run TestLinker -count=1 2>&1 | tail -30
```

Expected: all 34 restored tests pass plus any pre-existing passes.

- [ ] **Step C9.5: Run per-task reviewers**

Dispatch both reviewers. V4 grep MUST pass.

---

### Task C10: Restore `linker_api_test.go` (8 tests)

**Design reference:** File Manifest (line 1939).
**Files modified:** `internal/component/linker_api_test.go`.

- [ ] **Step C10.1: Apply the same 8-step methodology as Task C9 for each of 8 tests.**

Citation format:
```go
// Wasmtime parallel: runtime/component/linker.rs Linker public API surface.
// No counterpart (justified): host linker API is outside canonical-abi scope.
```

- [ ] **Step C10.2: Run the file + reviewers**

```bash
go test ./internal/component/ -run TestLinkerAPI -count=1 2>&1 | tail -10
```

Dispatch both reviewers.

---

### Task C11: Restore `conformance/primitives_test.go` citations (audit-only)

**Design reference:** File Manifest audit-only (design lines 1990-2000).
**Spec citation:** `run_tests.py::test_pairs`, `test_nan32`, `test_nan64`.
**Files modified:** `internal/component/conformance/primitives_test.go`.

- [ ] **Step C11.1: Read the current file**

```bash
wc -l internal/component/conformance/primitives_test.go
grep -c '^func Test' internal/component/conformance/primitives_test.go
```

- [ ] **Step C11.2: For each `func Test...` function, add a citation block**

Walk every top-level `func TestXxx(t *testing.T)` in the file. For each, determine whether it maps to a `run_tests.py` case:

```bash
grep -n 'def test_pairs\|def test_nan32\|def test_nan64' debug-vendored/component-model/design/mvp/canonical-abi/run_tests.py
```

For each wazero test that maps to a canonical-abi test, add the citation:

```go
// TestPrimitiveS32RoundTrip asserts lift/lower of s32 values round-trips
// through the canonical ABI.
//
// Canonical test: run_tests.py:NNN (test_pairs s32 cases).
// Spec: definitions.py:1065-1171 scalar ABI formulas (elem_size/alignment
// for numeric types).
func TestPrimitiveS32RoundTrip(t *testing.T) { ... }
```

For each wazero test without a direct canonical counterpart, add:

```go
// No counterpart (justified): wazero-specific edge case exercising the
// lift/lower dispatch for an internal invariant not covered by run_tests.py.
```

- [ ] **Step C11.3: Extend with missing `run_tests.py` cases**

After citation coverage, walk `run_tests.py::test_pairs` / `test_nan32` / `test_nan64` and identify any cases not yet covered in `primitives_test.go`. Add table-driven cases:

```go
// Added from run_tests.py:NNN test_pairs — cases missing in the audit.
```

- [ ] **Step C11.4: Run the file**

```bash
go test ./internal/component/conformance/ -run Primitive -count=1 2>&1 | tail -10
```

Expected: PASS.

- [ ] **Step C11.5: V4 grep on the file**

```bash
python3 -c "
import re
with open('internal/component/conformance/primitives_test.go') as f:
    lines = f.readlines()
cite = re.compile(r'(Spec:|definitions\.py:|run_tests\.py|Wasmtime:|No counterpart \(justified\):)')
pattern = re.compile(r'^func (Test\w+)\(t \*testing\.T\)')
bad = []
for i, line in enumerate(lines):
    m = pattern.match(line)
    if not m:
        continue
    found = False
    for j in range(i-1, max(i-16, -1), -1):
        if lines[j].strip().startswith('//') and cite.search(lines[j]):
            found = True
            break
        if not lines[j].strip().startswith('//') and lines[j].strip() != '':
            break
    if not found:
        bad.append(f'{i+1}: {m.group(1)}')
if bad:
    print('\n'.join(bad))
    exit(1)
"
```

Expected: exit 0.

- [ ] **Step C11.6: Run per-task reviewers**

Dispatch both reviewers.

---

### Task C12: Restore `conformance/may_leave_test.go` citations (audit-only)

**Design reference:** File Manifest audit-only (design line 1997).
**Spec citation:** `definitions.py:260 class ComponentInstance.may_leave`, `:1955 lower_flat_values may_leave=False`, `:1973 may_leave=True`, `:2065 canon_lower trap_if(not may_leave)`, `:2135, :2143 canon_resource_* may_leave`.
**Files modified:** `internal/component/conformance/may_leave_test.go`.

- [ ] **Step C12.1: Add citation blocks**

For every `func TestXxx` in `may_leave_test.go`, add a citation block referencing the relevant `definitions.py` line from the list above.

- [ ] **Step C12.2: Cross-check assertions against spec**

For each test, read the cited `definitions.py` line and verify the assertion matches. If the test asserts "IsMayLeave() returns false during an active call" (the pre-Session-0 buggy behavior), the test must be rewritten to match the Session 1 Task B1 spec-correct semantics, OR deleted if the test was validating the bug.

- [ ] **Step C12.3: Run the file**

```bash
go test ./internal/component/conformance/ -run MayLeave -count=1 2>&1 | tail -10
```

Expected: PASS.

- [ ] **Step C12.4: V4 grep + reviewers**

Run V4 grep. Dispatch both reviewers.

---

### Task C13: Restore `conformance/composites_test.go` (1 test + new body)

**Design reference:** File Manifest (line 1961).
**Spec citation:** `run_tests.py::test_heap`, `test_flatten` — composite type heap/flat round-trips.
**Files modified:** `internal/component/conformance/composites_test.go`.

- [ ] **Step C13.1: Delete the `TestCompositesDeferredToSession1` stub**

```bash
grep -n 'TestCompositesDeferredToSession1' internal/component/conformance/composites_test.go
```

Delete the function body and replace with a real multi-case suite.

- [ ] **Step C13.2: Pull pre-Session-0 body**

```bash
git show 98b3bbc3:internal/component/conformance/composites_test.go > /tmp/old_composites_test.go
wc -l /tmp/old_composites_test.go
```

- [ ] **Step C13.3: Port cases from `run_tests.py::test_heap`**

Walk `run_tests.py` for `test_heap` cases (record, variant, list, tuple, flags, enum, option, result). For each composite ABI case, port into `composites_test.go` with a citation block:

```go
// TestCompositeRecordRoundTrip exercises record lift/lower.
//
// Canonical test: run_tests.py:NNN test_heap record cases.
// Spec: definitions.py:1065-1171 record elem_size/alignment formulas.
```

- [ ] **Step C13.4: Run the file**

```bash
go test ./internal/component/conformance/ -run Composites -count=1 2>&1 | tail -20
```

Expected: PASS.

- [ ] **Step C13.5: V4 grep + reviewers**

---

### Task C14: Restore `conformance/strings_test.go` (1 test + new body)

**Design reference:** File Manifest (line 1962).
**Spec citation:** `run_tests.py` string cases (spec string encodings: utf-8, utf-16, latin-1+utf-16).
**Files modified:** `internal/component/conformance/strings_test.go`.

- [ ] **Step C14.1 through C14.5: Follow the same pattern as Task C13**

Pull old body, port canonical cases with citations, run, V4 grep, reviewers.

For strings, the citations reference:
- `definitions.py:1078-1127` (or wherever string elem_size + encoding rules live — verify via `grep -n 'string\|encoding' definitions.py`).
- `run_tests.py` string encoding round-trip cases.

---

### Task C15: Restore `conformance/flat_abi_test.go` (1 test + new body)

**Design reference:** File Manifest (line 1963).
**Spec citation:** `run_tests.py::test_flatten` (7 cases).
**Files modified:** `internal/component/conformance/flat_abi_test.go`.

- [ ] **Step C15.1 through C15.5: Same pattern**

Port all 7 `test_flatten` cases with citations. Verify MAX_FLAT_PARAMS + MAX_FLAT_RESULTS boundary behavior.

---

### Task C16: Restore `conformance/abi_edge_cases_test.go` (1 test + new body)

**Design reference:** File Manifest (line 1960).
**Spec citation:** `run_tests.py::test_roundtrips` (end-to-end lift/lower round-trips across primitive + composite types).
**Files modified:** `internal/component/conformance/abi_edge_cases_test.go`.

- [ ] **Step C16.1 through C16.5: Same pattern**

---

### Task C17: Restore `conformance/linker_test.go` (1 test + new body)

**Design reference:** File Manifest (line 1969).
**Spec citation:** None direct — linker behavior is wazero/wasmtime embedder layer.
**Files modified:** `internal/component/conformance/linker_test.go`.

- [ ] **Step C17.1 through C17.5: Same pattern**

Citations use `Wasmtime parallel: runtime/component/linker.rs` or `No counterpart (justified):` for each case.

---

### Task C18: Restore `conformance/post_return_test.go` (1 test + new body)

**Design reference:** File Manifest (line 1973).
**Spec citation:** `definitions.py:2000-2002` (may_leave toggle around post_return).
**Files modified:** `internal/component/conformance/post_return_test.go`.

- [ ] **Step C18.1 through C18.5: Same pattern**

---

### Task C19: Restore `value_import_test.go`

**Design reference:** File Manifest (line 1952).
**Files modified:** `internal/component/value_import_test.go`.

- [ ] **Step C19.1: Restore the 1 skipped test**

Per Test Restoration Methodology.

---

### Task C20: Checkpoint C verification

- [ ] **Step C20.1: Build check**

```bash
go build ./... 2>&1 | head -40
```

Expected: empty.

- [ ] **Step C20.2: Targeted test check**

```bash
go test ./internal/component/conformance/ -run 'Primitives|Composites|Strings|ABIEdge|FlatABI|PostReturn|Linker|MayLeave' -count=1 2>&1 | tail -30 && \
  go test ./internal/component/ -run 'Linker|Instantiate' -count=1 2>&1 | tail -30 && \
  go test ./internal/component/abi/ -count=1 2>&1 | tail -20
```

Expected: all green.

- [ ] **Step C20.3: V4 grep on all Checkpoint C files**

Run the V4 python script on every file touched in C9-C18. Fail if any restored test lacks a citation.

- [ ] **Step C20.4: Working-tree integrity**

```bash
git status --porcelain | head -40
```

- [ ] **Step C20.5: Dispatch checkpoint review**

Dispatch both reviewers over Checkpoint C scope. Apply correctives.

---

## Checkpoint D — Nested components + `resolveExportTypeAlias` + integration tests

**Scope:** Delete the `nested_component.go:171` panic stub. Rebuild `resolveExportTypeAlias`, `instantiateNestedComponent`, `wireNestedComponentExports`, `createInlineInstanceModule`. Restore `nested_component_test.go` (21 tests), `integration_test.go` (19 tests), `start_function_test.go` (9 tests), `component_linker_test.go` (8 tests). Restore `conformance/nested_test.go`, `nested_instantiation_test.go`, `nesting_depth_test.go`. Audit `conformance/reentrance_test.go` for citations.

**Design references:** Decision 1 (nested wiring), Nested component instantiation (design lines 1134-1137), File Manifest (lines 1878 nested_component.go, 1941-1944).

**Exit criterion (Checkpoint D gate):**
```bash
cd /home/cchamplin/development/wazero && \
  go build ./... 2>&1 | head -20 && \
  go test ./internal/component/ -run 'Nested|Integration|StartFunction|ComponentLinker' -count=1 2>&1 | tail -30 && \
  go test ./internal/component/conformance/ -run 'Nested|NestingDepth|Reentrance' -count=1 2>&1 | tail -30
```
Expected: build empty, all targeted tests pass.

---

### Task D1: Implement `resolveExportTypeAlias`

**Design reference:** Decoder → Linker Indirection (design lines 1246-1247); nested_component.go panic stub (line 1878).
**Spec citation:** Component-model type alias semantics (Explainer.md export type alias at `:600+`). Spec `definitions.py` does not cover linker-level type aliases; wazero-specific nested component bookkeeping.
**Files modified:** `internal/component/nested_component.go`.

- [ ] **Step D1.1: Write failing test**

Create `internal/component/nested_component_resolve_alias_test.go`:

```go
package component

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component/types"
)

// TestResolveExportTypeAlias asserts resolveExportTypeAlias walks parent.TypeDefs
// and an alias's source-instance exports to find the referenced TypeDef.
//
// No counterpart (justified): type alias resolution is a wazero linker-layer
// concern not covered by canonical-abi spec; equivalent logic in wasmtime
// lives in runtime/component/types.rs alias resolution.
func TestResolveExportTypeAlias(t *testing.T) {
	parent := &Instance{
		component: &Component{
			TypeDefs: []TypeDef{
				{Kind: TypeDefKindFunc, Func: types.FuncTypeIdx(0)},
				{Kind: TypeDefKindResource, Resource: types.ResourceTableIdx(0)},
			},
		},
	}
	// Alias pointing at slot 1 (the resource declaration).
	alias := &Alias{
		Kind:  AliasKindType,
		Index: 1,
	}
	l := NewComponentLinker()
	td := l.resolveExportTypeAlias(parent, parent.component, alias)
	if td == nil {
		t.Fatalf("resolveExportTypeAlias returned nil")
	}
	if td.Kind != TypeDefKindResource {
		t.Fatalf("td.Kind = %v, want TypeDefKindResource", td.Kind)
	}
}
```

- [ ] **Step D1.2: Run to confirm fail**

```bash
go test ./internal/component/ -run TestResolveExportTypeAlias -count=1 2>&1 | tail -10
```

Expected: panic with `compile-fix stub` (the current body).

- [ ] **Step D1.3: Implement**

Edit `internal/component/nested_component.go` and replace the panic body:

```go
// resolveExportTypeAlias resolves a type export alias by tracing through
// the source instance's type definitions to find the actual TypeDef for
// the exported type.
//
// The alias carries a SourceKind (parent / local / export) and an Index.
// For AliasKindType (local parent-scope type reference), Index is the
// parent component's type-section slot, resolved via c.TypeDefs.
//
// Spec: not covered by canonical-abi (linker layer). Wasmtime parallel:
// runtime/component/types.rs resolves type aliases via the instantiator's
// type context at nested instantiation time.
func (l *ComponentLinker) resolveExportTypeAlias(parent *Instance, c *Component, alias *Alias) *TypeDef {
	if alias == nil {
		return nil
	}
	switch alias.Kind {
	case AliasKindType:
		// Local parent-scope type reference.
		if int(alias.Index) >= len(c.TypeDefs) {
			return nil
		}
		return &c.TypeDefs[alias.Index]
	case AliasKindExport:
		// Export alias: alias.Index is the source instance-space slot;
		// alias.ExportName is the export to read.
		if parent == nil || int(alias.Index) >= len(parent.instanceSpace) {
			return nil
		}
		srcInst := parent.instanceSpace[alias.Index]
		if srcInst == nil {
			return nil
		}
		// Look up the export by name in the source instance's type space.
		// The source instance's component has its own TypeDefs; walk them.
		srcC := srcInst.component
		for i := range srcC.TypeDefs {
			// Match by name via the source component's exports.
			// (Adapt to actual export-name → type-def lookup.)
			_ = i
		}
		return nil
	}
	return nil
}
```

(The `AliasKindExport` branch depends on how Aliases are shaped in the current codebase — grep `grep -n 'type Alias struct\|AliasKind' internal/component/component.go` to verify field names. Adapt the body.)

- [ ] **Step D1.4: Run the test to confirm PASS**

```bash
go test ./internal/component/ -run TestResolveExportTypeAlias -count=1 2>&1 | tail -10
```

Expected: PASS.

- [ ] **Step D1.5: Run per-task reviewers**

Dispatch both reviewers.

---

### Task D2: Rebuild `instantiateNestedComponent`

**Design reference:** Nested component instantiation (design lines 1134-1137).
**Spec citation:** Component-model nested instantiation (Explainer.md `:1020+`). Wasmtime parallel: `runtime/component/instance.rs` nested component handling.
**Files modified:** `internal/component/nested_component.go`, `internal/component/component_linker.go` (remove the Session 1 Checkpoint C placeholder trap in `processNestedInstances`).

- [ ] **Step D2.1: Write failing test**

Create `internal/component/nested_instantiation_test.go` (only if the file doesn't already exist with the same name from the skipped-tests manifest — grep first):

```bash
grep -n 'TestNestedInstantiation\|nested_instantiation' internal/component/*.go
```

If an existing test is skipped, restore it per the Test Restoration Methodology. Otherwise create:

```go
// TestNestedInstantiateSimple asserts a component can instantiate a
// nested component inline and wire its exports into the parent.
//
// Spec: Explainer.md :1020+ component instantiation.
// Wasmtime parallel: runtime/component/instance.rs Instantiator walks
// ComponentInstances and recursively calls instantiate.
func TestNestedInstantiateSimple(t *testing.T) {
	compiled := buildParentWithNestedComponentExportingAdd(t)
	l := NewComponentLinker()
	inst, err := l.Instantiate(context.Background(), compiled)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	// Parent re-exports nested.add as "add"; call it.
	addFn, ok := inst.exports["add"]
	if !ok {
		t.Fatalf("missing export add")
	}
	results, err := addFn.Call(context.Background(), types.ValS32(10), types.ValS32(32))
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if results[0].S32() != 42 {
		t.Fatalf("add = %d, want 42", results[0].S32())
	}
}
```

- [ ] **Step D2.2: Run to confirm fail**

```bash
go test ./internal/component/ -run TestNestedInstantiateSimple -count=1 2>&1 | tail -20
```

Expected: trap from `processNestedInstances` ("nested component instantiation: rebuild in progress").

- [ ] **Step D2.3: Implement `instantiateNestedComponent`**

Add to `internal/component/nested_component.go`:

```go
// instantiateNestedComponent instantiates a nested component inline,
// resolving its imports from the parent's scope and returning the
// resulting Instance.
//
// Spec: Explainer.md component instantiation :1020+.
// Wasmtime parallel: runtime/component/instance.rs Instantiator recursion.
func (l *ComponentLinker) instantiateNestedComponent(
	ctx context.Context,
	parent *Instance,
	compInst *ComponentInstance,
) (*Instance, error) {
	// Look up the nested component by its componentSpace index.
	if int(compInst.Index) >= len(parent.componentSpace) {
		return nil, fmt.Errorf("nested instantiate: component index %d out of range", compInst.Index)
	}
	nestedComp := parent.componentSpace[compInst.Index]
	if nestedComp == nil {
		return nil, fmt.Errorf("nested instantiate: nil component at index %d", compInst.Index)
	}

	// Allocate a child instance with parent = parent.
	child := newInstance(nestedComp, l.nextInstanceID(), parent)
	parent.children = append(parent.children, child)

	// Resolve imports from the parent's scope using compInst.Imports as
	// the map from nested import name to parent-scope source.
	childImports := make(map[string]Definition)
	for _, imp := range compInst.Imports {
		src, err := l.resolveFromParentScope(parent, imp)
		if err != nil {
			return nil, fmt.Errorf("nested import %q: %w", imp.Name, err)
		}
		childImports[imp.Name] = src
	}

	// Run the nested instantiate pipeline against the child. Session 1
	// shares the same helpers as the top-level Instantiate body; the
	// difference is that imports come from childImports instead of the
	// linker's global definitions.
	if err := l.bindResourceTypes(child, nestedComp); err != nil {
		return nil, fmt.Errorf("nested bind resources: %w", err)
	}
	// ... remaining steps (index spaces, type checking with the childImports,
	//     core module instantiation, exports wiring) ...
	// The full sequence mirrors Instantiate steps 3-14 but reads imports
	// from childImports. Extract the shared body into a helper
	// (`instantiateCommon`) if desired, or inline the steps here.

	return child, nil
}

// resolveFromParentScope resolves a nested component's import name from
// the parent instance's index spaces (component funcs, instance space,
// type space, component space) following the component-model scoping rules.
func (l *ComponentLinker) resolveFromParentScope(parent *Instance, imp *ImportExtern) (Definition, error) {
	// Walk parent.componentFuncs / parent.instanceSpace / parent.typeSpace
	// based on imp.Desc.Kind. Rewrite from the pre-Session-0 body if needed;
	// verify against the existing shape.
	switch imp.Desc.Kind {
	case ImportExternDescFunc:
		// Look up in parent.componentFuncs.
		// ...
	case ImportExternDescInstance:
		// Look up in parent.instanceSpace.
		// ...
	case ImportExternDescType:
		// Look up in parent.typeSpace.
		// ...
	}
	return nil, fmt.Errorf("resolve from parent scope: kind %v not handled", imp.Desc.Kind)
}
```

Update `component_linker.go::processNestedInstances` to call `instantiateNestedComponent`:

```go
func (l *ComponentLinker) processNestedInstances(ctx context.Context, inst *Instance, c *Component) (map[uint32]*InstanceDef, error) {
	componentInstDefs := make(map[uint32]*InstanceDef)
	for i := range c.ComponentInstances {
		ci := &c.ComponentInstances[i]
		switch ci.Kind {
		case ComponentInstanceKindAlias:
			continue
		case ComponentInstanceKindInline:
			child, err := l.instantiateNestedComponent(ctx, inst, ci)
			if err != nil {
				return nil, err
			}
			for int(ci.Index) >= len(inst.instanceSpace) {
				inst.instanceSpace = append(inst.instanceSpace, nil)
			}
			inst.instanceSpace[ci.Index] = child
		}
	}
	return componentInstDefs, nil
}
```

Adapt field names to match the actual `ComponentInstance` struct in the codebase.

- [ ] **Step D2.4: Run the test**

```bash
go test ./internal/component/ -run TestNestedInstantiateSimple -count=1 2>&1 | tail -20
```

Expected: PASS.

- [ ] **Step D2.5: Run per-task reviewers**

Dispatch both reviewers.

---

### Task D3: Rebuild `wireNestedComponentExports` + `createInlineInstanceModule`

**Design reference:** Nested component instantiation (design lines 1134-1137).
**Files modified:** `internal/component/nested_component.go`.

- [ ] **Step D3.1: Write a failing test exercising nested export → parent re-export**

Extend `TestNestedInstantiateSimple` or add a parallel test where the nested component exports a resource and the parent re-exports it. Verify the re-export's `*ResourceType` pointer identity is preserved (same `*runtime.ResourceType` visible from both parent and nested).

- [ ] **Step D3.2: Run to confirm fail**

- [ ] **Step D3.3: Implement**

```go
// wireNestedComponentExports walks a nested instance's component-level
// exports and makes each accessible under the parent's export name
// mapping per the parent's export declarations.
func (l *ComponentLinker) wireNestedComponentExports(parent *Instance, nested *Instance, parentExport *Export) error {
	// For a func export: look up nested.exports[exportName] and copy into
	// parent.componentFuncs at the parent-declared slot.
	// For an instance export: copy the nested instance reference into
	// parent.instanceSpace at the parent-declared slot + propagate the
	// nested's exports into parent.exports[parentExport.Name + "/" + subname].
	// For a type export: copy the TypeDef via resolveExportTypeAlias.
	// ... body ...
	return nil
}

// createInlineInstanceModule constructs a wazero host module from a
// component-level instance export declaration (used when a nested component
// needs to import an instance that the parent provides inline).
func (l *ComponentLinker) createInlineInstanceModule(
	parent *Instance,
	decl *InstanceTypeDef,
	exports map[string]Definition,
) (*HostModule, error) {
	mod := NewHostModule()
	// Walk decl.Declarations; for each InstanceDeclKindExport whose export
	// is a func, register a host function that proxies to the matching
	// entry in `exports`.
	// ... body ...
	return mod, nil
}
```

- [ ] **Step D3.4: Run the test + reviewers**

---

### Task D4: Extend `executeStartFunction` to the full body

**Design reference:** Instantiate pipeline step 13 (design lines 898-900).
**Spec citation:** Component-model start function semantics (Explainer.md start function).
**Files modified:** `internal/component/component_linker.go`.

- [ ] **Step D4.1: Write failing test**

Restore `internal/component/start_function_test.go` tests. Begin with the simplest skipped case and add a citation:

```go
// TestStartFunctionExecutesOnce asserts the declared start function
// runs exactly once during Instantiate.
//
// Wasmtime parallel: runtime/component/instance.rs start function execution.
// No canonical counterpart (justified): start function is declared by
// the component binary, not by canonical-abi; see Explainer.md "start".
```

- [ ] **Step D4.2: Implement**

Replace the Checkpoint C stub body in `executeStartFunction`:

```go
func (l *ComponentLinker) executeStartFunction(ctx context.Context, inst *Instance, c *Component) error {
	if c.Start == nil {
		return nil
	}
	startFunc, ok := inst.componentFuncs[c.Start.FuncIdx]
	if !ok {
		return fmt.Errorf("start function: component function %d not found", c.Start.FuncIdx)
	}
	// Resolve the start's argument values from the value index space.
	args := make([]types.Val, 0, len(c.Start.Args))
	for _, vIdx := range c.Start.Args {
		if int(vIdx) >= len(inst.values) || inst.valuesConsumed[vIdx] {
			return fmt.Errorf("start function: value index %d invalid or already consumed", vIdx)
		}
		args = append(args, inst.values[vIdx])
		inst.valuesConsumed[vIdx] = true
	}
	// Invoke.
	if startFunc.Impl == nil {
		return fmt.Errorf("start function: function %d not wired", c.Start.FuncIdx)
	}
	// Wrap in the same canon.lift-style closure pattern as other calls.
	// ... or use a direct function invocation if startFunc.Impl is a
	// HostFunc with a Callback ...
	results, err := invokeComponentFunc(ctx, inst, startFunc, args)
	if err != nil {
		return fmt.Errorf("start function: %w", err)
	}
	// Populate result values into the value index space.
	for i, r := range results {
		resultIdx := c.Start.ResultIdx + uint32(i)
		for int(resultIdx) >= len(inst.values) {
			inst.values = append(inst.values, types.Val{})
			inst.valuesConsumed = append(inst.valuesConsumed, false)
		}
		inst.values[resultIdx] = r
	}
	return nil
}

// invokeComponentFunc is the dispatch helper used by executeStartFunction
// (and by wireExports for regular-export invocation).
func invokeComponentFunc(ctx context.Context, inst *Instance, cf ComponentFunc, args []types.Val) ([]types.Val, error) {
	switch impl := cf.Impl.(type) {
	case *FuncDef:
		return impl.Callback(ctx, args)
	default:
		return nil, fmt.Errorf("invokeComponentFunc: unsupported impl type %T", cf.Impl)
	}
}
```

Adapt to actual field names.

- [ ] **Step D4.3: Run the test + reviewers**

---

### Task D5: Restore `nested_component_test.go` (21 tests)

**Design reference:** File Manifest (line 1941).
**Files modified:** `internal/component/nested_component_test.go`.

- [ ] **Step D5.1: Locate skipped tests**

```bash
grep -c 't\.Skip.*session 1 work' internal/component/nested_component_test.go
```

Expected: 21.

- [ ] **Step D5.2: Restore per Test Restoration Methodology**

For each of the 21 tests, pull pre-Session-0 body, dedup-check, add citation block, rewrite against new types, run, delete skip, commit.

Citation format:
```go
// Wasmtime parallel: runtime/component/instance.rs nested instantiation.
// No direct canonical counterpart (justified): nested component
// instantiation is wazero linker-layer logic; canonical-abi does not
// exercise linker structure.
```

- [ ] **Step D5.3: Run + V4 grep + reviewers**

---

### Task D6: Restore `integration_test.go` (19 tests)

**Design reference:** File Manifest (line 1942).
**Files modified:** `internal/component/integration_test.go`.

- [ ] **Step D6.1-D6.3: Apply same methodology**

These tests exercise end-to-end flows through the linker. Citations typically reference:
- `run_tests.py::test_pairs` / `test_heap` when the integration exercises lift/lower.
- Wasmtime `tests/all/component_model/func.rs` for full-instantiation scenarios.

---

### Task D7: Restore `start_function_test.go` (9 tests)

**Design reference:** File Manifest (line 1943).
**Files modified:** `internal/component/start_function_test.go`.

- [ ] **Step D7.1-D7.3: Apply methodology**

Citations reference Explainer.md start function semantics.

---

### Task D8: Restore `component_linker_test.go` (8 tests)

**Design reference:** File Manifest (line 1944).
**Files modified:** `internal/component/component_linker_test.go`.

- [ ] **Step D8.1-D8.3: Apply methodology**

---

### Task D9: Restore `conformance/nested_test.go`

**Design reference:** File Manifest (line 1971).
**Spec citation:** Component-model nested component semantics.
**Files modified:** `internal/component/conformance/nested_test.go`.

- [ ] **Step D9.1: Replace deferred stub + port from pre-Session-0 body**

Apply methodology.

---

### Task D10: Restore `conformance/nested_instantiation_test.go`

**Design reference:** File Manifest (line 1970).
**Files modified:** `internal/component/conformance/nested_instantiation_test.go`.

- [ ] **Step D10.1: Restore**

---

### Task D11: Restore `conformance/nesting_depth_test.go`

**Design reference:** File Manifest (line 1972).
**Files modified:** `internal/component/conformance/nesting_depth_test.go`.

- [ ] **Step D11.1: Restore**

Cite wazero-specific engineering invariant (nesting depth limit) with `No counterpart (justified):`.

---

### Task D12: Audit `conformance/reentrance_test.go` citations

**Design reference:** File Manifest audit-only (design line 1998).
**Spec citation:** `definitions.py:290-299` call_might_be_recursive, `:1979` trap_if(call_might_be_recursive). `run_tests.py::test_reentrance`.
**Files modified:** `internal/component/conformance/reentrance_test.go`.

- [ ] **Step D12.1: Add citation blocks**

For each `func TestXxx` in the file, add a citation block referencing the appropriate `definitions.py` line + `run_tests.py::test_reentrance` case.

- [ ] **Step D12.2: Cross-check assertions against spec**

In particular, verify the tests assert the transitive ancestor check from `:290-299`, not the direct `caller == inst` check. If any test relies on the old direct check, update to exercise the Session 1 Task B2 transitive semantic.

- [ ] **Step D12.3: Extend with missing `test_reentrance` cases**

- [ ] **Step D12.4: Run + V4 grep + reviewers**

---

### Task D13: Checkpoint D verification

- [ ] **Step D13.1: Build + test**

```bash
go build ./... 2>&1 | head -20 && \
  go test ./internal/component/ -run 'Nested|Integration|StartFunction|ComponentLinker' -count=1 2>&1 | tail -30 && \
  go test ./internal/component/conformance/ -run 'Nested|NestingDepth|Reentrance' -count=1 2>&1 | tail -30
```

- [ ] **Step D13.2: V4 grep over all Checkpoint D files**

- [ ] **Step D13.3: Working-tree integrity**

- [ ] **Step D13.4: Dispatch checkpoint review**

---

## Checkpoint E — Resource type binding + 4 lift.go fixes + resource conformance

**Scope:** Implement `Table.GetByIndex`. Change `ResourceHandleEntry.Rep` from `any` to `uint32`. Migrate `imports/wasip2/io/*` + `http` + `filesystem` + `sockets` + `clocks` + `cli` to per-module u32 registries. Add `runtime.ResourceType.HostDestructor`. Add `BorrowScope.ReleaseBorrow`. Fix 4 lift.go gaps (Own, NumLends, GetByIndex, Rep-u32). Rewrite `Instance.ResourceNew`/`Rep`/`Drop` to the spec-matching signatures. Add `byteMemory` test helper. Restore 11 `abi/` bounds-check tests. Restore `conformance/resources_test.go`, `destructor_test.go`, `resource_generation_test.go`, `concurrent_access_test.go`. Restore resource-related cases in `instance_test.go`.

**Design references:** Decision 2 (design lines 141-184), Decision 4 (lines 287-380), Decision 7 (lines 477-667), Resource Identity (lines 1138-1227), lift.go Gap Fixes (lines 1261-1425), Canon Resource Ops (lines 1249-1259), bounds-check harness (lines 1690-1845), File Manifest (lines 1914-1923), Verification V7, V8.

**Exit criterion (Checkpoint E gate):**
```bash
cd /home/cchamplin/development/wazero && \
  go build ./... 2>&1 | head -20 && \
  go test ./internal/component/abi/ -count=1 2>&1 | tail -20 && \
  go test ./internal/component/conformance/ -run 'Resources|Destructor|ResourceGeneration|ConcurrentAccess' -count=1 2>&1 | tail -30 && \
  go test ./internal/component/ -run 'CanonResource|InstanceResource|ResourceNew|ResourceDrop|ResourceRep' -count=1 2>&1 | tail -20
```

---

### Task E1: Add `Table.GetByIndex`

**Design reference:** Decision 4 Gap 3 (design lines 295-320); Gap 3 code block (lines 1397-1425).
**Spec citation:** `definitions.py:303-315` Table (the spec's Table is index-keyed; wazero's generation-tagged Handle bridging is a wazero impl detail that must produce the same observable behavior).
**Files modified:** `internal/component/runtime/table.go`, `internal/component/runtime/table_test.go`.

- [ ] **Step E1.1: Write failing test**

Append to `internal/component/runtime/table_test.go`:

```go
// TestTableGetByIndexGenerationBridging asserts GetByIndex looks up an
// entry by slot index (the low 32 bits of a Handle) and returns the
// full generation-tagged Handle plus the entry — bridging the 32-bit
// Wasm-side handle space to the 64-bit runtime Handle space.
//
// Spec: definitions.py:303-315 class Table (index-keyed; the generation
// bridging is a wazero implementation detail that must preserve the
// spec's index-keyed observable behavior).
// Wasmtime parallel: runtime/vm/component/resources.rs handle index
// lookup uses raw index + generation.
func TestTableGetByIndexGenerationBridging(t *testing.T) {
	tbl := NewTable()
	rt := &ResourceType{}
	h1, err := tbl.NewResourceHandle(42, true, rt)
	if err != nil {
		t.Fatalf("NewResourceHandle: %v", err)
	}
	// h1 is a 64-bit tagged handle; its low 32 bits are the slot index.
	idx := h1.Index()

	// GetByIndex(idx) must return (h1, entry, nil) even though the
	// caller only knows the 32-bit index.
	h, entry, err := tbl.GetByIndex(idx)
	if err != nil {
		t.Fatalf("GetByIndex: %v", err)
	}
	if h != h1 {
		t.Fatalf("GetByIndex handle = %v, want %v", h, h1)
	}
	if entry == nil {
		t.Fatalf("GetByIndex entry = nil")
	}

	// Remove h1, allocate a new handle at the same slot (generation increments).
	if _, err := tbl.Remove(h1); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	h2, err := tbl.NewResourceHandle(99, true, rt)
	if err != nil {
		t.Fatalf("NewResourceHandle 2: %v", err)
	}
	// h2 should reuse the slot (same Index) with incremented generation.
	if h2.Index() != idx {
		t.Fatalf("h2.Index = %d, want %d (slot reuse)", h2.Index(), idx)
	}
	if h2 == h1 {
		t.Fatalf("h2 == h1 (generation did not increment)")
	}

	// GetByIndex(idx) must now return h2 (with the new generation), not h1.
	h, _, err = tbl.GetByIndex(idx)
	if err != nil {
		t.Fatalf("GetByIndex after reuse: %v", err)
	}
	if h != h2 {
		t.Fatalf("GetByIndex handle = %v, want %v (new generation)", h, h2)
	}
	if h == h1 {
		t.Fatalf("GetByIndex returned stale handle %v", h1)
	}
}

// TestTableGetByIndexFreeSlot asserts GetByIndex returns ErrInvalidHandle
// for a slot that's currently free.
func TestTableGetByIndexFreeSlot(t *testing.T) {
	tbl := NewTable()
	rt := &ResourceType{}
	h, _ := tbl.NewResourceHandle(1, true, rt)
	tbl.Remove(h)
	_, _, err := tbl.GetByIndex(h.Index())
	if err == nil {
		t.Fatalf("GetByIndex(freed slot) returned nil error")
	}
}

// TestTableGetByIndexOutOfRange asserts GetByIndex returns ErrInvalidHandle
// for an index past the end of the entries slice.
func TestTableGetByIndexOutOfRange(t *testing.T) {
	tbl := NewTable()
	_, _, err := tbl.GetByIndex(9999)
	if err == nil {
		t.Fatalf("GetByIndex(9999) returned nil error")
	}
}
```

**Note:** `Rep: 42` assumes `Rep` is still `any` at this point. After Task E2 it will be `uint32(42)`. Tests may need minor tweaks after E2.

- [ ] **Step E1.2: Run to confirm fail**

```bash
go test ./internal/component/runtime/ -run TestTableGetByIndex -count=1 2>&1 | tail -20
```

Expected: `tbl.GetByIndex undefined`.

- [ ] **Step E1.3: Implement**

Open `internal/component/runtime/table.go`. Add after the existing `Get` method:

```go
// GetByIndex looks up an entry by slot index (the low 32 bits of a
// Handle), returning the current generation-tagged Handle alongside the
// entry. Used by the canonical ABI lift path (abi/lift.go) to resolve a
// Wasm-side u32 handle index to the runtime's 64-bit generation-tagged
// handle.
//
// Spec: definitions.py:303-315 class Table (index-keyed). The generation
// bridging is a wazero implementation detail that must preserve the
// spec's observable "slot index → entry" lookup semantics.
//
// Returns ErrInvalidHandle if the slot index is out of range or the
// slot is currently free.
func (t *Table) GetByIndex(idx uint32) (Handle, TableEntry, error) {
	if idx >= uint32(len(t.entries)) {
		return 0, nil, ErrInvalidHandle
	}
	slot := &t.entries[idx]
	if slot.state == entryFree || slot.entry == nil {
		return 0, nil, ErrInvalidHandle
	}
	h := MakeHandle(idx, slot.generation)
	return h, slot.entry, nil
}
```

- [ ] **Step E1.4: Run the tests to PASS**

```bash
go test ./internal/component/runtime/ -run TestTableGetByIndex -count=1 2>&1 | tail -10
```

Expected: PASS.

- [ ] **Step E1.5: Run per-task reviewers**

---

### Task E2: Change `ResourceHandleEntry.Rep` from `any` to `uint32`

**Design reference:** Decision 4 Gap 4 (design lines 322-380).
**Spec citation:** `definitions.py:337-349` `class ResourceHandle.rep: int` (u32 invariant). Wasmtime: `runtime/component/instance.rs:383-387` `resource_new32`.
**Files modified:** `internal/component/runtime/table.go`, all callers of `ResourceHandleEntry.Rep`.

- [ ] **Step E2.1: Audit callers**

```bash
grep -rn '\.Rep\b' internal/component/runtime/ internal/component/abi/ internal/component/ imports/wasip2/ 2>&1 | grep -v '\.Rep\s*[=:]' | head -40
grep -rn 'ResourceHandleEntry' internal/component/ imports/wasip2/ 2>&1 | head -40
grep -rn 'Rep:\s' internal/component/ imports/wasip2/ 2>&1 | head -40
```

Record every site that reads or writes `Rep`.

- [ ] **Step E2.2: Write failing test**

Append to `internal/component/runtime/table_test.go`:

```go
// TestResourceHandleEntryRepIsUint32 asserts Rep is typed uint32 per
// spec definitions.py:337-349 ResourceHandle.rep.
//
// Spec: definitions.py:337-349 class ResourceHandle.rep.
// Wasmtime parallel: runtime/component/instance.rs:383-387 resource_new32
// + vm/component/resources.rs rep: u32 throughout.
func TestResourceHandleEntryRepIsUint32(t *testing.T) {
	entry := &ResourceHandleEntry{
		RT:  &ResourceType{},
		Rep: uint32(42),
	}
	if entry.Rep != uint32(42) {
		t.Fatalf("entry.Rep = %v, want 42", entry.Rep)
	}
	// Compile-time check: any attempt to store a non-uint32 will fail.
	// var _ uint32 = entry.Rep  // (enforced at compile time by the field type)
}
```

- [ ] **Step E2.3: Run to confirm fail (or compile failure)**

```bash
go test ./internal/component/runtime/ -run TestResourceHandleEntryRepIsUint32 -count=1 2>&1 | tail -10
```

Expected: either compile failure `cannot use uint32(42) as any` (if the field is still `any`) or the test passes if you're running before the migration.

- [ ] **Step E2.4: Change the field type + add `HostDestructor`**

Edit `internal/component/runtime/table.go`:

```go
// ResourceHandleEntry is the Table entry type for resource handles.
// Session 1 Gap 4: Rep is uint32 per spec definitions.py:337-349.
// Wasmtime parallel: runtime/vm/component/resources.rs rep: u32.
type ResourceHandleEntry struct {
	RT          *ResourceType // Resource type (pointer identity)
	Rep         uint32        // Spec: definitions.py:337-349 rep: int (u32 invariant)
	Own         bool          // True if owning handle
	NumLends    uint32        // Outstanding borrow count
	BorrowScope *BorrowScope  // Borrow scope for borrow handles
}
```

Also change `BorrowScope any` → `*BorrowScope` (it is always a `*BorrowScope` in practice; the `any` was a Session 0 placeholder).

Edit `NewResourceHandle` to take `uint32`:

```go
func (t *Table) NewResourceHandle(rep uint32, own bool, rt *ResourceType) (Handle, error) {
	entry := &ResourceHandleEntry{RT: rt, Rep: rep, Own: own}
	return t.add(entry)
}
```

Similarly for `NewWithMayLeaveCheck`:

```go
func (t *Table) NewWithMayLeaveCheck(rep uint32, own bool, rt *ResourceType, inst *ComponentInstance) (Handle, error) {
	if inst != nil && !inst.IsMayLeave() {
		return 0, ErrMayNotLeave
	}
	return t.NewResourceHandle(rep, own, rt)
}
```

Add the `HostDestructor` field to `ResourceType` (task E3 will use it):

```go
// ResourceType identifies a nominal resource type. Pointer identity is
// used for type comparisons (spec definitions.py:351-361 class ResourceType).
type ResourceType struct {
	Impl         *ComponentInstance // Defining component instance
	Dtor         *uint32            // Core function index of guest-side destructor
	DtorAsync    bool
	DtorCallback *uint32

	// HostDestructor is set by host modules (imports/wasip2/*) for their
	// own resource types. When non-nil, Table.Remove dispatches to this
	// closure instead of looking up Dtor as a guest core function. The
	// rep is passed as the argument; the closure is responsible for
	// looking up any host-side state via its per-module registry and
	// invoking any Destroyable semantics.
	//
	// Spec: definitions.py:351-361 (dtor closure varies by instance). In
	// wazero, guest-declared resources use Dtor (core function); host-
	// declared resources use HostDestructor (Go closure).
	HostDestructor func(rep uint32) error
}
```

- [ ] **Step E2.5: Migrate every caller**

For each call site from Step E2.1:

- `tbl.NewResourceHandle(someGoPointer, true, rt)` → `tbl.NewResourceHandle(registerXxx(someGoPointer), true, rt)` (see Task E4 for the per-module registry pattern).
- `entry.Rep` read sites: drop the type assertion `.(uint32)` if any; the value is already `uint32`.

For the `imports/wasip2/` call sites, wire the migration to a per-module registry (Task E4) — but for this task, at minimum make the code compile by passing `uint32(0)` placeholders where the Go pointer used to go. The placeholder sites are resolved in Task E4.

Alternative: mark this task as "builds" but defer the semantic wiring to E4; the plan then runs tests for lift.go gaps in E5 after both E2 and E4 complete. In practice, Tasks E2 + E4 are coupled; run them together in a single task if the migration is small enough.

- [ ] **Step E2.6: Build**

```bash
go build ./... 2>&1 | head -40
```

Expected: empty. Iterate until green.

- [ ] **Step E2.7: Run the test**

```bash
go test ./internal/component/runtime/ -run TestResourceHandleEntryRepIsUint32 -count=1 2>&1 | tail -5
```

Expected: PASS.

- [ ] **Step E2.8: Run per-task reviewers**

---

### Task E3: Add `BorrowScope.ReleaseBorrow` + `invokeLocalDestructor` helper

**Design reference:** Decision 7 borrow branch (design lines 603-621); File Manifest (lines 1921-1922).
**Spec citation:** `definitions.py:2163-2164` canon_resource_drop borrow branch decrements `h.borrow_scope.num_borrows`. `definitions.py:738-742` deliver_resolve.
**Files modified:** `internal/component/runtime/borrow_scope.go`, `internal/component/runtime/destructor.go` (or wherever destructor invocation helpers live), `internal/component/instance.go` (add `invokeLocalDestructor`).

- [ ] **Step E3.1: Write failing test**

Append to `internal/component/runtime/borrow_scope_test.go`:

```go
// TestBorrowScopeReleaseBorrow asserts the scope's release operation is
// the symmetric inverse of AddLender: it decrements NumLends on the
// handle's entry and removes the handle from the lender set.
//
// Spec: definitions.py:2163-2164 canon_resource_drop borrow branch.
// Spec: definitions.py:738-742 deliver_resolve (scope closure).
func TestBorrowScopeReleaseBorrow(t *testing.T) {
	tbl := NewTable()
	rt := &ResourceType{}
	h, _ := tbl.NewResourceHandle(1, true, rt)
	scope := NewBorrowScope(tbl)
	if err := tbl.IncrementLends(h); err != nil {
		t.Fatalf("IncrementLends: %v", err)
	}
	if err := scope.AddLender(h); err != nil {
		t.Fatalf("AddLender: %v", err)
	}
	if !scope.HasOutstandingBorrows() {
		t.Fatalf("HasOutstandingBorrows = false, want true")
	}
	if err := scope.ReleaseBorrow(h); err != nil {
		t.Fatalf("ReleaseBorrow: %v", err)
	}
	if scope.HasOutstandingBorrows() {
		t.Fatalf("HasOutstandingBorrows after release = true, want false")
	}
	entry, _ := tbl.GetResourceHandle(h)
	if entry.NumLends != 0 {
		t.Fatalf("entry.NumLends = %d, want 0", entry.NumLends)
	}
}
```

- [ ] **Step E3.2: Run to confirm fail**

```bash
go test ./internal/component/runtime/ -run TestBorrowScopeReleaseBorrow -count=1 2>&1 | tail -10
```

Expected: `scope.ReleaseBorrow undefined`.

- [ ] **Step E3.3: Implement `ReleaseBorrow`**

Edit `internal/component/runtime/borrow_scope.go`. Add:

```go
// ReleaseBorrow is the symmetric inverse of AddLender. It decrements the
// handle's NumLends count via the table and removes the handle from the
// scope's lender set.
//
// Spec: definitions.py:2163-2164 canon_resource_drop borrow branch
// decrements h.borrow_scope.num_borrows when a borrow handle is dropped
// while its scope is still open.
func (s *BorrowScope) ReleaseBorrow(h Handle) error {
	// Decrement lends on the handle.
	if err := s.table.DecrementLends(h); err != nil {
		return err
	}
	// Remove from the lender set.
	for i, lh := range s.lenders {
		if lh == h {
			s.lenders = append(s.lenders[:i], s.lenders[i+1:]...)
			return nil
		}
	}
	return nil
}
```

The `DecrementLends` method on `Table` may not exist yet; if not, add it alongside `IncrementLends`:

```go
// DecrementLends decrements NumLends on a resource handle. Returns
// ErrInvalidHandle if the handle is invalid or NumLends is already zero.
func (t *Table) DecrementLends(h Handle) error {
	entry, err := t.GetResourceHandle(h)
	if err != nil {
		return err
	}
	if entry.NumLends == 0 {
		return fmt.Errorf("%w: NumLends already zero", ErrInvalidHandle)
	}
	entry.NumLends--
	return nil
}
```

- [ ] **Step E3.4: Run to PASS**

```bash
go test ./internal/component/runtime/ -run TestBorrowScopeReleaseBorrow -count=1 2>&1 | tail -5
```

- [ ] **Step E3.5: Add `invokeLocalDestructor` in `instance.go`**

Add helper:

```go
// invokeLocalDestructor invokes a resource destructor on the defining
// instance. For guest-declared resources, rt.Dtor is a core function
// index into the defining instance's core module; the function is
// called with the rep as its single i32 argument. For host-declared
// resources, rt.HostDestructor is a Go closure that is invoked directly.
//
// Spec: definitions.py:2151 rt.dtor(h.rep).
func invokeLocalDestructor(inst *Instance, rt *runtime.ResourceType, rep uint32) error {
	if rt.HostDestructor != nil {
		return rt.HostDestructor(rep)
	}
	if rt.Dtor == nil {
		return nil
	}
	// Look up the core function at index *rt.Dtor in the defining instance's
	// core module(s). Session 1 assumes resource destructors live in core
	// instance 0 (the single core module for a simple component); multi-core
	// components resolve via the alias map.
	if len(inst.coreInstances) == 0 {
		return fmt.Errorf("invokeLocalDestructor: instance has no core modules")
	}
	core := inst.coreInstances[0]
	fn := core.ExportedFunctionByIndex(*rt.Dtor)
	if fn == nil {
		return fmt.Errorf("invokeLocalDestructor: function %d not found", *rt.Dtor)
	}
	_, err := fn.Call(context.Background(), uint64(rep))
	return err
}
```

Adapt `ExportedFunctionByIndex` / core-function lookup to the actual wazero runtime API.

- [ ] **Step E3.6: Run per-task reviewers**

---

### Task E4: Migrate `imports/wasip2/io/*` (and http/filesystem/sockets/clocks/cli) to per-module u32 registries

**Design reference:** Decision 4 Gap 4 host-managed resources migration (design lines 326-380).
**Spec citation:** `definitions.py:337-349` + wasmtime `vm/component/host_tables.rs` pattern.
**Files modified:** `imports/wasip2/io/streams.go`, `imports/wasip2/io/poll.go`, `imports/wasip2/http/*.go`, `imports/wasip2/filesystem/*.go`, `imports/wasip2/sockets/*.go`, `imports/wasip2/clocks/*.go`, `imports/wasip2/cli/*.go`.

- [ ] **Step E4.1: Audit per-module resource sites**

```bash
grep -rn 'NewResourceHandle' imports/wasip2/ 2>&1
```

For each file, identify the resource types being minted and the Go object type stored as the rep.

- [ ] **Step E4.2: Add the registry pattern per module**

For each module, add the registry helpers (example for `imports/wasip2/io/streams.go`):

```go
// inputStreamRegistry holds the per-module InputStream state indexed by
// u32 id. The id is the Rep stored in the runtime.Table and flows
// through canon.resource.rep as a spec-conformant u32.
//
// Spec: definitions.py:337-349 ResourceHandle.rep. Wasmtime parallel:
// runtime/vm/component/host_tables.rs per-type host state keyed by index.
var (
	inputStreamRegistryMu sync.Mutex
	inputStreamRegistry   []*InputStream
	inputStreamFreelist   []uint32
)

func registerInputStream(s *InputStream) uint32 {
	inputStreamRegistryMu.Lock()
	defer inputStreamRegistryMu.Unlock()
	if n := len(inputStreamFreelist); n > 0 {
		id := inputStreamFreelist[n-1]
		inputStreamFreelist = inputStreamFreelist[:n-1]
		inputStreamRegistry[id] = s
		return id
	}
	id := uint32(len(inputStreamRegistry))
	inputStreamRegistry = append(inputStreamRegistry, s)
	return id
}

func getInputStream(id uint32) *InputStream {
	inputStreamRegistryMu.Lock()
	defer inputStreamRegistryMu.Unlock()
	if int(id) >= len(inputStreamRegistry) {
		return nil
	}
	return inputStreamRegistry[id]
}

func unregisterInputStream(id uint32) {
	inputStreamRegistryMu.Lock()
	defer inputStreamRegistryMu.Unlock()
	if int(id) < len(inputStreamRegistry) {
		inputStreamRegistry[id] = nil
		inputStreamFreelist = append(inputStreamFreelist, id)
	}
}
```

- [ ] **Step E4.3: Wire `HostDestructor` on the per-module `*ResourceType`**

For each resource type singleton, set `HostDestructor`:

```go
var inputStreamResourceType = &runtime.ResourceType{
	HostDestructor: func(rep uint32) error {
		if s := getInputStream(rep); s != nil {
			// Preserve existing Destroyable semantics.
			if d, ok := interface{}(s).(interface{ Destroy() error }); ok {
				if err := d.Destroy(); err != nil {
					return err
				}
			}
			unregisterInputStream(rep)
		}
		return nil
	},
}
```

- [ ] **Step E4.4: Update every `NewResourceHandle` site in the module**

```go
// Before:
// handle, err := table.NewResourceHandle(stream, true, inputStreamResourceType)

// After:
id := registerInputStream(stream)
handle, err := table.NewResourceHandle(id, true, inputStreamResourceType)
if err != nil {
	unregisterInputStream(id)  // rollback on error
	return 0, err
}
```

- [ ] **Step E4.5: Apply the same pattern to every other wasip2 module with resources**

- `imports/wasip2/io/poll.go` — `Pollable` registry
- `imports/wasip2/http/outgoing.go` + `incoming.go` — request/response/body registries
- `imports/wasip2/filesystem/filesystem.go` + `preopens.go` — descriptor, directory-entry-stream registries
- `imports/wasip2/sockets/tcp.go` + `udp.go` + `network.go` — socket registries
- `imports/wasip2/clocks/monotonic.go` — (resources, if any)
- `imports/wasip2/cli/cli.go` — (resources, if any)

For each, mirror the pattern from E4.2.

- [ ] **Step E4.6: Build the full tree**

```bash
go build ./... 2>&1 | head -40
```

Expected: empty.

- [ ] **Step E4.7: Run existing wasip2 tests**

```bash
go test ./imports/wasip2/... -count=1 2>&1 | tail -30
```

Expected: passes except for any tests that were asserting `Rep` as a Go pointer (those must be updated to assert the registry pattern).

- [ ] **Step E4.8: Run per-task reviewers**

Dispatch both reviewers. Spec reviewer must verify that every registry is thread-safe (the wasip2 tables are accessed concurrently if multiple instances share a host module).

---

### Task E5: Rewrite `Instance.ResourceNew` / `ResourceRep` / `ResourceDrop` with spec-correct signatures

**Design reference:** Decision 7 (design lines 477-621).
**Spec citation:** `definitions.py:2134-2138` `canon_resource_new`, `:2142-2165` `canon_resource_drop`, `:2169-2173` `canon_resource_rep`.
**Files modified:** `internal/component/instance.go`.

- [ ] **Step E5.1: Write failing tests**

Create `internal/component/instance_resource_ops_test.go`:

```go
package component

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
)

// TestInstanceResourceNewSpecSignature asserts ResourceNew's signature
// matches spec canon_resource_new(rt, thread, rep).
//
// Spec: definitions.py:2134-2138 canon_resource_new:
//   def canon_resource_new(rt, thread, rep):
//     trap_if(not thread.task.inst.may_leave)
//     i = thread.task.inst.table.add(ResourceHandle(rt, rep, own=True))
//     return [i]
func TestInstanceResourceNewSpecSignature(t *testing.T) {
	inst := newInstance(&Component{}, 1, nil)
	rt := &runtime.ResourceType{Impl: inst.rt}
	inst.rt.ResourceTypes = append(inst.rt.ResourceTypes, rt)

	h, err := inst.ResourceNew(types.ResourceIdx(0), uint32(42))
	if err != nil {
		t.Fatalf("ResourceNew: %v", err)
	}
	if h == 0 {
		t.Fatalf("ResourceNew returned 0")
	}
}

// TestInstanceResourceNewTrapMayLeave asserts the may_leave trap.
// Spec: definitions.py:2135 trap_if(not may_leave).
func TestInstanceResourceNewTrapMayLeave(t *testing.T) {
	inst := newInstance(&Component{}, 1, nil)
	rt := &runtime.ResourceType{Impl: inst.rt}
	inst.rt.ResourceTypes = append(inst.rt.ResourceTypes, rt)
	inst.rt.MayLeave = false
	_, err := inst.ResourceNew(types.ResourceIdx(0), uint32(42))
	if err == nil {
		t.Fatalf("ResourceNew with may_leave=false returned nil error")
	}
}

// TestInstanceResourceRepSpecSignature asserts ResourceRep returns the
// rep as uint32 and validates the type.
//
// Spec: definitions.py:2169-2173 canon_resource_rep:
//   def canon_resource_rep(rt, thread, i):
//     h = thread.task.inst.table.get(i)
//     trap_if(not isinstance(h, ResourceHandle))
//     trap_if(h.rt is not rt)
//     return [h.rep]
func TestInstanceResourceRepSpecSignature(t *testing.T) {
	inst := newInstance(&Component{}, 1, nil)
	rt := &runtime.ResourceType{Impl: inst.rt}
	inst.rt.ResourceTypes = append(inst.rt.ResourceTypes, rt)
	h, _ := inst.ResourceNew(types.ResourceIdx(0), uint32(42))
	rep, err := inst.ResourceRep(types.ResourceIdx(0), h)
	if err != nil {
		t.Fatalf("ResourceRep: %v", err)
	}
	if rep != 42 {
		t.Fatalf("ResourceRep = %d, want 42", rep)
	}
}

// TestInstanceResourceDropSpecSignature asserts ResourceDrop removes
// the handle, validates type, and dispatches to the destructor.
//
// Spec: definitions.py:2142-2165 canon_resource_drop.
func TestInstanceResourceDropSpecSignature(t *testing.T) {
	inst := newInstance(&Component{}, 1, nil)
	var destructorCalls int
	rt := &runtime.ResourceType{
		Impl:         inst.rt,
		HostDestructor: func(rep uint32) error {
			destructorCalls++
			if rep != 42 {
				t.Errorf("destructor rep = %d, want 42", rep)
			}
			return nil
		},
	}
	inst.rt.ResourceTypes = append(inst.rt.ResourceTypes, rt)

	h, _ := inst.ResourceNew(types.ResourceIdx(0), uint32(42))
	if err := inst.ResourceDrop(types.ResourceIdx(0), h); err != nil {
		t.Fatalf("ResourceDrop: %v", err)
	}
	if destructorCalls != 1 {
		t.Fatalf("destructorCalls = %d, want 1", destructorCalls)
	}
	// After drop, ResourceRep should fail.
	if _, err := inst.ResourceRep(types.ResourceIdx(0), h); err == nil {
		t.Fatalf("ResourceRep after drop returned nil error")
	}
}

// TestInstanceResourceDropTypeMismatch asserts the type mismatch trap.
// Spec: definitions.py:2147 trap_if(h.rt is not rt).
func TestInstanceResourceDropTypeMismatch(t *testing.T) {
	inst := newInstance(&Component{}, 1, nil)
	rtA := &runtime.ResourceType{Impl: inst.rt}
	rtB := &runtime.ResourceType{Impl: inst.rt}
	inst.rt.ResourceTypes = append(inst.rt.ResourceTypes, rtA, rtB)
	h, _ := inst.ResourceNew(types.ResourceIdx(0), uint32(1))
	err := inst.ResourceDrop(types.ResourceIdx(1), h)
	if err == nil {
		t.Fatalf("ResourceDrop with wrong type returned nil error")
	}
}

// TestInstanceResourceDropLendsTrap asserts own handles with outstanding
// lends trap on drop.
// Spec: definitions.py:2148 trap_if(h.num_lends != 0).
func TestInstanceResourceDropLendsTrap(t *testing.T) {
	inst := newInstance(&Component{}, 1, nil)
	rt := &runtime.ResourceType{Impl: inst.rt}
	inst.rt.ResourceTypes = append(inst.rt.ResourceTypes, rt)
	h, _ := inst.ResourceNew(types.ResourceIdx(0), uint32(1))
	if err := inst.rt.Table.IncrementLends(runtime.Handle(h)); err != nil {
		// Use GetByIndex to get the full handle.
		full, _, _ := inst.rt.Table.GetByIndex(h)
		_ = full
		inst.rt.Table.IncrementLends(full)
	}
	err := inst.ResourceDrop(types.ResourceIdx(0), h)
	if err == nil {
		t.Fatalf("ResourceDrop with outstanding lends returned nil error")
	}
}
```

- [ ] **Step E5.2: Run to confirm fail**

```bash
go test ./internal/component/ -run 'TestInstanceResource' -count=1 2>&1 | tail -30
```

Expected: signature mismatch (wrong number of arguments or wrong types).

- [ ] **Step E5.3: Replace the placeholder errors from Task B4 with spec-correct bodies**

Edit `internal/component/instance.go`. Replace the three methods:

```go
// ResourceNew is canon.resource.new — spec definitions.py:2134-2138.
// Mints a new own-handle of the given resource type and returns the
// Wasm-side handle index. Traps if !may_leave. resourceIdx is the
// declaration index in the component's type section; it resolves via
// i.rt.ResourceTypes[resourceIdx] to the pointer-identity *ResourceType.
func (i *Instance) ResourceNew(resourceIdx types.ResourceIdx, rep uint32) (uint32, error) {
	if !i.rt.IsMayLeave() {
		return 0, errMayNotLeave
	}
	if int(resourceIdx) >= len(i.rt.ResourceTypes) {
		return 0, fmt.Errorf("resource.new: resource declaration %d not defined", resourceIdx)
	}
	rt := i.rt.ResourceTypes[resourceIdx]
	if rt == nil {
		return 0, fmt.Errorf("resource.new: resource type %d not concrete", resourceIdx)
	}
	h, err := i.rt.Table.NewResourceHandle(rep, true, rt)
	if err != nil {
		return 0, err
	}
	return h.Index(), nil
}

// ResourceRep is canon.resource.rep — spec definitions.py:2169-2173.
func (i *Instance) ResourceRep(resourceIdx types.ResourceIdx, handleIdx uint32) (uint32, error) {
	if int(resourceIdx) >= len(i.rt.ResourceTypes) {
		return 0, fmt.Errorf("resource.rep: resource declaration %d not defined", resourceIdx)
	}
	rt := i.rt.ResourceTypes[resourceIdx]
	if rt == nil {
		return 0, fmt.Errorf("resource.rep: resource type %d not concrete", resourceIdx)
	}
	_, entry, err := i.rt.Table.GetByIndex(handleIdx)
	if err != nil {
		return 0, err
	}
	resEntry, ok := entry.(*runtime.ResourceHandleEntry)
	if !ok {
		return 0, runtime.ErrInvalidHandle
	}
	// Spec :2172 — trap_if(h.rt is not rt).
	if resEntry.RT != rt {
		return 0, fmt.Errorf("resource.rep: type mismatch")
	}
	// Spec :2173 — return h.rep.
	return resEntry.Rep, nil
}

// ResourceDrop is canon.resource.drop — spec definitions.py:2142-2165.
func (i *Instance) ResourceDrop(resourceIdx types.ResourceIdx, handleIdx uint32) error {
	if !i.rt.IsMayLeave() {
		return errMayNotLeave
	}
	if int(resourceIdx) >= len(i.rt.ResourceTypes) {
		return fmt.Errorf("resource.drop: resource declaration %d not defined", resourceIdx)
	}
	rt := i.rt.ResourceTypes[resourceIdx]
	if rt == nil {
		return fmt.Errorf("resource.drop: resource type %d not concrete", resourceIdx)
	}
	h, entry, err := i.rt.Table.GetByIndex(handleIdx)
	if err != nil {
		return err
	}
	resEntry, ok := entry.(*runtime.ResourceHandleEntry)
	if !ok {
		return runtime.ErrInvalidHandle
	}
	// Spec :2147 — trap_if(h.rt is not rt).
	if resEntry.RT != rt {
		return fmt.Errorf("resource.drop: type mismatch")
	}
	// Spec :2148 — trap_if(h.num_lends != 0) on own handles.
	if resEntry.Own && resEntry.NumLends != 0 {
		return fmt.Errorf("resource.drop: own handle has %d outstanding lends", resEntry.NumLends)
	}
	// Remove from table.
	if _, err := i.rt.Table.Remove(h); err != nil {
		return err
	}
	if resEntry.Own {
		// Spec :2149-2161 — own branch: invoke destructor.
		if rt.HasDestructor() || rt.HostDestructor != nil {
			if rt.Impl != i.rt {
				// Spec :2154-2160 — cross-instance destructor dispatch via
				// canon_lift → canon_lower. Session 2 work.
				return fmt.Errorf("resource.drop: cross-instance destructor invocation (session 2 wiring)")
			}
			if err := invokeLocalDestructor(i, rt, resEntry.Rep); err != nil {
				return fmt.Errorf("resource.drop: destructor: %w", err)
			}
		}
	} else {
		// Spec :2163-2164 — borrow branch.
		if resEntry.BorrowScope != nil {
			if err := resEntry.BorrowScope.ReleaseBorrow(h); err != nil {
				return fmt.Errorf("resource.drop: borrow release: %w", err)
			}
		}
	}
	return nil
}
```

- [ ] **Step E5.4: Run tests**

```bash
go build ./... 2>&1 | head -20 && \
  go test ./internal/component/ -run TestInstanceResource -count=1 2>&1 | tail -30
```

Expected: all PASS.

- [ ] **Step E5.5: Run per-task reviewers**

---

### Task E6: Fix 4 lift.go gaps in `liftOwnHandle` + `liftBorrowHandle`

**Design reference:** Decision 4 (design lines 287-380); lift.go Gap Fixes (lines 1261-1395).
**Spec citation:** `definitions.py:1333-1339` lift_own, `:1341-1347` lift_borrow.
**Files modified:** `internal/component/abi/lift.go`.

- [ ] **Step E6.1: Write failing tests**

Create `internal/component/abi/lift_handle_gaps_test.go`:

```go
package abi

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
)

// TestLiftOwnHandleGap1TrapNotOwn asserts Gap 1: lift_own must trap if
// the handle is a borrow, not an own.
//
// Spec: definitions.py:1338 trap_if(not h.own).
// Wasmtime parallel: runtime/vm/component/resources.rs:275-279 resource_lift_own
// checks h.own in its internal handle state.
func TestLiftOwnHandleGap1TrapNotOwn(t *testing.T) {
	// ... construct LiftContext, mint a borrow handle, call liftOwnHandle, assert trap ...
}

// TestLiftOwnHandleGap2TrapNumLends asserts Gap 2: lift_own must trap
// if the own handle has outstanding lends.
//
// Spec: definitions.py:1337 trap_if(h.num_lends != 0).
func TestLiftOwnHandleGap2TrapNumLends(t *testing.T) {
	// ... mint an own handle, IncrementLends on it, call liftOwnHandle, assert trap ...
}

// TestLiftOwnHandleGap3GenerationBridging asserts Gap 3: lift_own uses
// Table.GetByIndex so recycled slots (generation > 0) are reachable.
//
// Spec: definitions.py:1334 h = cx.inst.table.remove(i) — lookup-by-index
// semantics; wazero's generation bridging must preserve this.
func TestLiftOwnHandleGap3GenerationBridging(t *testing.T) {
	// ... mint two handles at the same slot (Remove then NewResourceHandle),
	//     call liftOwnHandle on the second handle using its index, assert success ...
}

// TestLiftOwnHandleGap4ReturnsRep asserts Gap 4: lift_own returns the
// stored rep (uint32) via types.ValOwn.
//
// Spec: definitions.py:1339 return h.rep.
func TestLiftOwnHandleGap4ReturnsRep(t *testing.T) {
	// ... mint an own handle with rep=999, call liftOwnHandle, assert result.Own() == 999 ...
}

// TestLiftBorrowHandleGap3GenerationBridging asserts Gap 3 for borrow.
func TestLiftBorrowHandleGap3GenerationBridging(t *testing.T) { /* ... */ }

// TestLiftBorrowHandleGap4ReturnsRep asserts Gap 4 for borrow.
// Spec: definitions.py:1347 return h.rep.
func TestLiftBorrowHandleGap4ReturnsRep(t *testing.T) { /* ... */ }
```

Fill in each test body using the `runtime` + `abi` packages. Each test constructs a `LiftContext` with the appropriate `Instance.ResourceTypes` population to satisfy `LookupResourceType`.

- [ ] **Step E6.2: Run to confirm fail**

```bash
go test ./internal/component/abi/ -run 'TestLiftOwnHandleGap|TestLiftBorrowHandleGap' -count=1 2>&1 | tail -30
```

Expected: each test fails for its corresponding gap (Gap 1: borrow accepted, Gap 2: NumLends ignored, Gap 3: generation mismatch, Gap 4: wrong returned value).

- [ ] **Step E6.3: Rewrite `liftOwnHandle`**

Edit `internal/component/abi/lift.go`. Replace the body (around lines 638-669) with the Decision 4 rewrite:

```go
// liftOwnHandle implements TypeKindOwn lift per definitions.py:1333-1339.
//
// Spec:
//   def lift_own(cx, i, t):
//     h = cx.inst.table.remove(i)        # :1334
//     trap_if(not isinstance(h, ResourceHandle))
//     trap_if(h.rt is not t.rt)          # :1336
//     trap_if(h.num_lends != 0)          # :1337
//     trap_if(not h.own)                 # :1338
//     return h.rep                       # :1339
//
// Wasmtime parallel: runtime/vm/component/resources.rs:275-279 resource_lift_own.
func liftOwnHandle(ctx *LiftContext, typ types.ValType, handleIdx uint32) (types.Val, error) {
	if ctx == nil || ctx.Instance == nil {
		return types.Val{}, fmt.Errorf("lift own: no component instance available")
	}
	if ctx.Types == nil {
		return types.Val{}, fmt.Errorf("lift own: no component types available")
	}
	if int(typ.Index) >= len(ctx.Types.ResourceTables) {
		return types.Val{}, fmt.Errorf("lift own: resource table index %d out of range", typ.Index)
	}
	rt := ctx.Types.ResourceTables[typ.Index]
	expectedRT := ctx.Instance.LookupResourceType(rt.Instance, rt.Resource)
	if expectedRT == nil {
		return types.Val{}, fmt.Errorf(
			"lift own: no resource type for instance %d declaration %d "+
				"(cross-instance resolution: session 2 wiring)",
			rt.Instance, rt.Resource)
	}

	// Gap 3: GetByIndex bridges Wasm-side u32 to runtime 64-bit Handle.
	h, entry, err := ctx.Instance.Table.GetByIndex(handleIdx)
	if err != nil {
		return types.Val{}, fmt.Errorf("lift own: %w", err)
	}
	resEntry, ok := entry.(*runtime.ResourceHandleEntry)
	if !ok {
		return types.Val{}, fmt.Errorf("lift own: handle %d is not a resource handle", handleIdx)
	}
	// Spec :1336 — trap_if(h.rt is not t.rt).
	if resEntry.RT != expectedRT {
		return types.Val{}, fmt.Errorf("lift own: resource type mismatch")
	}
	// Spec :1337 — Gap 2 — trap_if(h.num_lends != 0).
	if resEntry.NumLends != 0 {
		return types.Val{}, fmt.Errorf("lift own: handle has %d outstanding lends", resEntry.NumLends)
	}
	// Spec :1338 — Gap 1 — trap_if(not h.own).
	if !resEntry.Own {
		return types.Val{}, fmt.Errorf("lift own: handle %d is a borrow, not an own", handleIdx)
	}
	// All checks passed — remove and return rep.
	if _, err := ctx.Instance.Table.Remove(h); err != nil {
		return types.Val{}, fmt.Errorf("lift own: %w", err)
	}
	// Spec :1339 — return h.rep. Gap 4 — Rep is now uint32.
	return types.ValOwn(resEntry.Rep), nil
}
```

- [ ] **Step E6.4: Rewrite `liftBorrowHandle`**

Replace the body (around lines 674-709) with:

```go
// liftBorrowHandle implements TypeKindBorrow lift per definitions.py:1341-1347.
//
// Spec:
//   def lift_borrow(cx, i, t):
//     assert(isinstance(cx.borrow_scope, Subtask))  # :1342
//     h = cx.inst.table.get(i)                      # :1343
//     trap_if(not isinstance(h, ResourceHandle))
//     trap_if(h.rt is not t.rt)                     # :1345
//     cx.borrow_scope.add_lender(h)                 # :1346
//     return h.rep                                  # :1347
//
// Wasmtime parallel: runtime/vm/component/resources.rs:291-297 resource_lift_borrow.
func liftBorrowHandle(ctx *LiftContext, typ types.ValType, handleIdx uint32) (types.Val, error) {
	if ctx == nil || ctx.Instance == nil {
		return types.Val{}, fmt.Errorf("lift borrow: no component instance available")
	}
	if ctx.Types == nil {
		return types.Val{}, fmt.Errorf("lift borrow: no component types available")
	}
	// Spec :1342 — assert(isinstance(cx.borrow_scope, Subtask)).
	if ctx.BorrowScope == nil {
		return types.Val{}, fmt.Errorf("lift borrow: no borrow scope active (caller must construct one)")
	}
	if int(typ.Index) >= len(ctx.Types.ResourceTables) {
		return types.Val{}, fmt.Errorf("lift borrow: resource table index %d out of range", typ.Index)
	}
	rt := ctx.Types.ResourceTables[typ.Index]
	expectedRT := ctx.Instance.LookupResourceType(rt.Instance, rt.Resource)
	if expectedRT == nil {
		return types.Val{}, fmt.Errorf(
			"lift borrow: no resource type for instance %d declaration %d "+
				"(cross-instance resolution: session 2 wiring)",
			rt.Instance, rt.Resource)
	}

	// Gap 3: GetByIndex.
	h, entry, err := ctx.Instance.Table.GetByIndex(handleIdx)
	if err != nil {
		return types.Val{}, fmt.Errorf("lift borrow: %w", err)
	}
	resEntry, ok := entry.(*runtime.ResourceHandleEntry)
	if !ok {
		return types.Val{}, fmt.Errorf("lift borrow: handle %d is not a resource handle", handleIdx)
	}
	// Spec :1345 — trap_if(h.rt is not t.rt).
	if resEntry.RT != expectedRT {
		return types.Val{}, fmt.Errorf("lift borrow: resource type mismatch")
	}
	// Spec :1346 — cx.borrow_scope.add_lender(h).
	if err := ctx.Instance.Table.IncrementLends(h); err != nil {
		return types.Val{}, fmt.Errorf("lift borrow: %w", err)
	}
	if err := ctx.BorrowScope.AddLender(h); err != nil {
		return types.Val{}, fmt.Errorf("lift borrow: %w", err)
	}
	// Spec :1347 — return h.rep. Gap 4 — Rep is now uint32.
	return types.ValBorrow(resEntry.Rep), nil
}
```

- [ ] **Step E6.5: Run tests**

```bash
go test ./internal/component/abi/ -run 'TestLiftOwnHandleGap|TestLiftBorrowHandleGap' -count=1 2>&1 | tail -30
```

Expected: all PASS.

- [ ] **Step E6.6: Update `lower.go` `lowerOwnHandleFlat` / `lowerBorrowHandleFlat` to use `GetByIndex`**

Find the lower counterparts:

```bash
grep -n 'lowerOwnHandle\|lowerBorrowHandle\|Handle(handleIdx)' internal/component/abi/lower.go
```

For each site that constructs `runtime.Handle(handleIdx)` from a Wasm-side u32, migrate to `Table.GetByIndex`. This preserves symmetry between lift and lower in the generation-tag handling.

- [ ] **Step E6.7: Run per-task reviewers**

---

### Task E7: Add `byteMemory` test helper + restore 11 abi bounds-check tests

**Design reference:** bounds-check test harness (design lines 1690-1845).
**Spec citation:** `api.Memory` wazero interface invariant; test harness for sub-page memories.
**Files modified:** `internal/component/abi/memory_test_helper.go` (new), `internal/component/abi/context_test.go`, `internal/component/abi/strings_test.go`.

- [ ] **Step E7.1: Create `byteMemory`**

Create `internal/component/abi/memory_test_helper.go` with the full body from design lines 1716-1840:

```go
package abi

import (
	"encoding/binary"
	"math"

	"github.com/tetratelabs/wazero/api"
)

// byteMemory is a direct []byte-backed api.Memory implementation for
// bounds-check tests that need to construct memories smaller than a
// Wasm page (64 KiB). Every method implements the wazero api.Memory
// contract: methods that read or write out of range return (zero, false)
// or (false) per the interface's "or returns false if out of range" rule.
//
// No counterpart (justified): this is a wazero test-harness invariant
// (api.Memory bounds semantics) not covered by canonical-abi spec.
type byteMemory struct {
	data []byte
}

func newByteMemory(size uint32) *byteMemory {
	return &byteMemory{data: make([]byte, size)}
}

func (m *byteMemory) Definition() api.MemoryDefinition { return nil }
func (m *byteMemory) Size() uint32                    { return uint32(len(m.data)) }
func (m *byteMemory) Grow(deltaPages uint32) (previousPages uint32, ok bool) {
	return 0, false
}

func (m *byteMemory) inRange(offset, length uint32) bool {
	return uint64(offset)+uint64(length) <= uint64(len(m.data))
}

func (m *byteMemory) ReadByte(offset uint32) (byte, bool) {
	if !m.inRange(offset, 1) {
		return 0, false
	}
	return m.data[offset], true
}

func (m *byteMemory) ReadUint16Le(offset uint32) (uint16, bool) {
	if !m.inRange(offset, 2) {
		return 0, false
	}
	return binary.LittleEndian.Uint16(m.data[offset:]), true
}

func (m *byteMemory) ReadUint32Le(offset uint32) (uint32, bool) {
	if !m.inRange(offset, 4) {
		return 0, false
	}
	return binary.LittleEndian.Uint32(m.data[offset:]), true
}

func (m *byteMemory) ReadFloat32Le(offset uint32) (float32, bool) {
	v, ok := m.ReadUint32Le(offset)
	if !ok {
		return 0, false
	}
	return math.Float32frombits(v), true
}

func (m *byteMemory) ReadUint64Le(offset uint32) (uint64, bool) {
	if !m.inRange(offset, 8) {
		return 0, false
	}
	return binary.LittleEndian.Uint64(m.data[offset:]), true
}

func (m *byteMemory) ReadFloat64Le(offset uint32) (float64, bool) {
	v, ok := m.ReadUint64Le(offset)
	if !ok {
		return 0, false
	}
	return math.Float64frombits(v), true
}

func (m *byteMemory) Read(offset, byteCount uint32) ([]byte, bool) {
	if !m.inRange(offset, byteCount) {
		return nil, false
	}
	return m.data[offset : offset+byteCount], true
}

func (m *byteMemory) WriteByte(offset uint32, v byte) bool {
	if !m.inRange(offset, 1) {
		return false
	}
	m.data[offset] = v
	return true
}

func (m *byteMemory) WriteUint16Le(offset uint32, v uint16) bool {
	if !m.inRange(offset, 2) {
		return false
	}
	binary.LittleEndian.PutUint16(m.data[offset:], v)
	return true
}

func (m *byteMemory) WriteUint32Le(offset uint32, v uint32) bool {
	if !m.inRange(offset, 4) {
		return false
	}
	binary.LittleEndian.PutUint32(m.data[offset:], v)
	return true
}

func (m *byteMemory) WriteFloat32Le(offset uint32, v float32) bool {
	return m.WriteUint32Le(offset, math.Float32bits(v))
}

func (m *byteMemory) WriteUint64Le(offset uint32, v uint64) bool {
	if !m.inRange(offset, 8) {
		return false
	}
	binary.LittleEndian.PutUint64(m.data[offset:], v)
	return true
}

func (m *byteMemory) WriteFloat64Le(offset uint32, v float64) bool {
	return m.WriteUint64Le(offset, math.Float64bits(v))
}

func (m *byteMemory) Write(offset uint32, v []byte) bool {
	if !m.inRange(offset, uint32(len(v))) {
		return false
	}
	copy(m.data[offset:], v)
	return true
}

func (m *byteMemory) WriteString(offset uint32, v string) bool {
	if !m.inRange(offset, uint32(len(v))) {
		return false
	}
	copy(m.data[offset:], v)
	return true
}
```

**Verify** every method on `api.Memory` is implemented. Run:

```bash
grep -n 'func (.*Memory)' /home/cchamplin/development/wazero/api/wasm.go 2>&1 | head -30
```

(or wherever `api.Memory` is declared — likely `api/memory.go` or `api/wasm.go`). For every method in the interface, ensure `byteMemory` has an implementation.

- [ ] **Step E7.2: Build**

```bash
go build ./internal/component/abi/... 2>&1 | head -10
```

Expected: empty. If any interface method is missing, the compiler will list them.

- [ ] **Step E7.3: Restore 9 context_test.go bounds-check tests**

```bash
grep -n 't\.Skip.*session 1 work' internal/component/abi/context_test.go
```

Expected: 9 hits (the `2` hit count from Checkpoint B was for partial patterns; recount).

Actually the manifest says 9. Verify the count matches. For each skipped test, restore the body against `newByteMemory(N)` where N is a small sub-page size that triggers bounds errors on specific reads.

Citation format:
```go
// TestContextBoundsCheckXxx asserts the LiftContext produces a precise
// bounds error when a lift operation would read past the end of memory.
//
// Spec: definitions.py:1947-1948 trap_if(ptr + elem_size(type) > len(memory)).
// Wasmtime parallel: runtime/component/func/options.rs bounds checks on
// LiftContext / LowerContext.
```

- [ ] **Step E7.4: Restore 4 strings_test.go tests**

```bash
grep -n 't\.Skip.*session 1 work' internal/component/abi/strings_test.go
```

Expected: 4 hits (or whatever the manifest says — verify).

Each test uses `newByteMemory(N)` to trigger a utf-8 / utf-16 bounds error.

- [ ] **Step E7.5: Run the abi package tests**

```bash
go test ./internal/component/abi/ -count=1 2>&1 | tail -30
```

Expected: all tests pass, including the 11 restored bounds-check tests.

- [ ] **Step E7.6: Run per-task reviewers**

---

### Task E8: Restore `conformance/resources_test.go`

**Design reference:** File Manifest (line 1976); test_handles enumeration (design line 1680).
**Spec citation:** `run_tests.py::test_handles` (~30 cases). `definitions.py:1333-1347, 2134-2173` resource lifecycle.
**Files modified:** `internal/component/conformance/resources_test.go`.

- [ ] **Step E8.1: Replace the deferred stub**

Delete `TestResourcesDeferredToSession1` body and construct a real multi-case suite.

- [ ] **Step E8.2: Port `run_tests.py::test_handles` cases**

Open `debug-vendored/component-model/design/mvp/canonical-abi/run_tests.py` and locate `def test_handles()`. Walk every case (expect ~30) and port each into Go as a table-driven sub-test:

```go
// TestResources ports the canonical-abi test_handles cases.
//
// Canonical test: run_tests.py test_handles (see debug-vendored/component-model/.../run_tests.py).
// Spec: definitions.py:1333-1347 lift_own/lift_borrow; :2134-2173 canon_resource_*.
// Wasmtime parallel: runtime/component/func.rs handle passing; runtime/vm/component/resources.rs.
func TestResources(t *testing.T) {
	cases := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			// run_tests.py test_handles: new + rep + drop round-trip.
			name: "NewRepDropRoundTrip",
			fn: func(t *testing.T) {
				// ... construct Instance, call ResourceNew → ResourceRep → ResourceDrop ...
			},
		},
		{
			// run_tests.py test_handles: own-handle lifts correctly.
			name: "LiftOwnHandle",
			fn: func(t *testing.T) { /* ... */ },
		},
		// ... continue for every case in test_handles ...
	}
	for _, tc := range cases {
		t.Run(tc.name, tc.fn)
	}
}
```

Each case must cite its `run_tests.py` line range in a comment above.

- [ ] **Step E8.3: Run**

```bash
go test ./internal/component/conformance/ -run Resources -count=1 2>&1 | tail -30
```

Expected: PASS.

- [ ] **Step E8.4: V4 grep + reviewers**

---

### Task E9: Restore `conformance/destructor_test.go`

**Design reference:** File Manifest (line 1965).
**Spec citation:** `definitions.py:2149-2161` destructor dispatch in canon_resource_drop.
**Files modified:** `internal/component/conformance/destructor_test.go`.

- [ ] **Step E9.1: Restore with local-instance cases**

Per design scope (Checkpoint E: "destructor_test.go local-instance cases only; cross-instance destructor cases get per-case session-2 skips"):

- Local destructor invocation cases are restored with full bodies.
- Cross-instance destructor cases keep a `t.Skip("Session 2: cross-instance destructor dispatch")` with a precise skip text that is NOT `"session 1 work"` (so V2 passes).

Each test gets a citation block:
```go
// TestDestructorLocalInvocation asserts a resource declared and dropped
// in the same instance invokes its registered destructor.
//
// Spec: definitions.py:2149-2161 canon_resource_drop destructor branch.
// Canonical test: run_tests.py test_handles destructor cases.
// Wasmtime parallel: runtime/component/resources/ty.rs dtor invocation.
```

- [ ] **Step E9.2: Run + reviewers**

---

### Task E10: Restore `conformance/resource_generation_test.go`

**Design reference:** File Manifest (line 1975).
**Spec citation:** `definitions.py:303-315` table entry generation semantics (wazero's generation bridging is a wazero impl detail; tests validate the bridging preserves spec observable behavior).
**Files modified:** `internal/component/conformance/resource_generation_test.go`.

- [ ] **Step E10.1 through E10.3: Restore with citations**

---

### Task E11: Restore `conformance/concurrent_access_test.go`

**Design reference:** File Manifest (line 1964).
**Spec citation:** Resource lifetime invariants under concurrent access; wazero engineering invariant.
**Files modified:** `internal/component/conformance/concurrent_access_test.go`.

- [ ] **Step E11.1 through E11.3: Restore with citations**

Most cases will be `No counterpart (justified):` (wazero-specific concurrency invariants).

---

### Task E12: Restore resource-related cases in `instance_test.go`

**Design reference:** File Manifest (design line 1938 — "resource tests in checkpoint E").
**Files modified:** `internal/component/instance_test.go`.

- [ ] **Step E12.1: Identify resource-related skipped tests**

```bash
grep -n 'session 1 work\|TestInstance.*Resource\|TestExported.*Resource' internal/component/instance_test.go
```

- [ ] **Step E12.2: Restore each per methodology**

Citations reference `definitions.py:2134-2173` canon_resource_*.

---

### Task E13: Checkpoint E verification

- [ ] **Step E13.1: Build check**

```bash
go build ./... 2>&1 | head -20
```

- [ ] **Step E13.2: V7 verification**

```bash
grep -n 'resEntry.Own' internal/component/abi/lift.go
grep -n 'resEntry.NumLends' internal/component/abi/lift.go
grep -n 'GetByIndex' internal/component/abi/lift.go
grep -n 'func .*Table. GetByIndex' internal/component/runtime/table.go
grep -n 'Rep\s*uint32' internal/component/runtime/table.go
grep -rn '\.Rep\.(uint32)' internal/component/
grep -rn 'HostDestructor' internal/component/runtime/ imports/wasip2/
```

Expected per design V7 — Own check present, NumLends check present, GetByIndex present in both lift.go and table.go, Rep is uint32, no `.Rep.(uint32)` assertions anywhere, HostDestructor set in wasip2.

- [ ] **Step E13.3: V8 verification (Instance resource op signatures)**

```bash
grep -n 'func (i \*Instance) ResourceNew' internal/component/instance.go
grep -n 'func (i \*Instance) ResourceRep' internal/component/instance.go
grep -n 'func (i \*Instance) ResourceDrop' internal/component/instance.go
```

Expected: `ResourceNew(resourceIdx types.ResourceIdx, rep uint32) (uint32, error)`, `ResourceRep(resourceIdx types.ResourceIdx, handleIdx uint32) (uint32, error)`, `ResourceDrop(resourceIdx types.ResourceIdx, handleIdx uint32) error`.

- [ ] **Step E13.4: V10 verification**

```bash
grep -n 'bindResourceTypes' internal/component/component_linker.go
```

Expected: function definition + call site.

- [ ] **Step E13.5: Targeted test check**

```bash
go test ./internal/component/abi/ -count=1 2>&1 | tail -20 && \
  go test ./internal/component/conformance/ -run 'Resources|Destructor|ResourceGeneration|ConcurrentAccess' -count=1 2>&1 | tail -30 && \
  go test ./internal/component/ -run 'CanonResource|InstanceResource|ResourceNew|ResourceDrop|ResourceRep' -count=1 2>&1 | tail -20
```

- [ ] **Step E13.6: Working-tree integrity + dispatch checkpoint review**

---

## Checkpoint F — Remaining tests + `type_checker.go` fixes + all 223 skipped tests green

**Scope:** Fix `type_checker.go::checkFuncType`/`checkFuncDefinition`/`checkInstanceDefinition` per Decision 6. Restore remaining `instance_test.go` (non-resource lift/lower), `type_checker_test.go` (17 tests), `edge_case_test.go`, `composite_test.go`, `instantiate_test.go`, `integration_public_api_test.go`, `integration_records_test.go`. Restore remaining conformance stubs: `error_messages_test.go`, `instance_types_test.go`, `memory_bounds_test.go`, `realloc_failure_test.go`, `type_edge_cases_test.go`, `utf_validation_test.go`, `wasi_cli_test.go`, `wasi_clocks_test.go`, `wasi_error_handling_test.go`, `wasi_filesystem_test.go`, `wasi_http_test.go`, `wasi_poll_test.go`, `wasi_random_test.go`, `wasi_resource_lifecycle_test.go`, `wasi_sockets_test.go`, `wasi_streams_test.go`.

**Design references:** Decision 6 (lines 449-475); Type Checker Scope section (lines 1429-1578); File Manifest (lines 1935-1989).

**Exit criterion (Checkpoint F gate):**
```bash
cd /home/cchamplin/development/wazero && \
  grep -rln 'session 1 work' internal/ api/ imports/ && \
  grep -rln 'DeferredToSession1' internal/component/conformance/ && \
  go test ./... -count=1 2>&1 | tail -40
```
Expected: first two greps return empty. `go test ./...` passes except `conformance/subtask_test.go` which has `t.Skip("later work: async lift/lower")`.

---

### Task F1: Fix `type_checker.go::checkFuncType` + `checkFuncDefinition` + `checkInstanceDefinition`

**Design reference:** Decision 6 (design lines 449-475); Type Checker Scope — Session 1 (lines 1429-1578).
**Spec citation:** `definitions.py:88-101` FuncType param_types() / result_type() strip names. Wasmtime parallel: `runtime/component/matching.rs:51, :162`.
**Files modified:** `internal/component/type_checker.go`.

- [ ] **Step F1.1: Write failing tests**

Create `internal/component/type_checker_session1_test.go`:

```go
package component

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component/types"
)

// TestCheckFuncTypeIdentityOnly asserts checkFuncType compares via
// identity on (Async, Params, Results) and ignores ParamNames.
//
// Spec: definitions.py:88-101 FuncType.param_types() strips names.
// Wasmtime parallel: matching.rs InterfaceType::Tuple comparison ignores names.
func TestCheckFuncTypeIdentityOnly(t *testing.T) {
	builder := types.NewComponentTypesBuilder()
	paramsTuple := builder.InternTuple([]types.ValType{types.S32, types.S32})
	resultsTuple := builder.InternTuple([]types.ValType{types.S32})

	expected := &types.TypeFunc{
		Async:      false,
		Params:     paramsTuple,
		Results:    resultsTuple,
		ParamNames: []string{"a", "b"},
	}
	actual := &types.TypeFunc{
		Async:      false,
		Params:     paramsTuple,
		Results:    resultsTuple,
		ParamNames: []string{"x", "y"}, // different names, same types
	}

	c := &Component{Types: builder.Finish()}
	tc := NewTypeChecker(c)
	if err := tc.checkFuncType(expected, actual); err != nil {
		t.Fatalf("checkFuncType (different names, same types) = %v, want nil", err)
	}
}

// TestCheckFuncDefinitionRequiresTypedActual asserts host FuncDef must
// carry a non-nil Type — Session 1 Decision 6 rejects untyped host functions.
//
// Spec: wasmtime matching.rs:51 bails on None actual.
func TestCheckFuncDefinitionRequiresTypedActual(t *testing.T) {
	builder := types.NewComponentTypesBuilder()
	paramsTuple := builder.InternTuple([]types.ValType{})
	resultsTuple := builder.InternTuple([]types.ValType{})
	c := &Component{
		Types: builder.Finish(),
		TypeDefs: []TypeDef{
			{Kind: TypeDefKindFunc, Func: types.FuncTypeIdx(0)},
		},
	}
	// Need to populate c.Types.Funcs with the matching TypeFunc.
	// (Adapt to actual builder API.)
	tc := NewTypeChecker(c)
	expected := &ImportExternDesc{Kind: ImportExternDescFunc, TypeIdx: 0}

	// Untyped actual — must reject.
	fdUntyped := &FuncDef{Type: nil}
	if err := tc.checkFuncDefinition(expected, fdUntyped); err == nil {
		t.Fatalf("checkFuncDefinition(untyped) returned nil, want error")
	}

	// Typed actual with matching type — must accept.
	fdTyped := &FuncDef{Type: &types.TypeFunc{
		Params:  paramsTuple,
		Results: resultsTuple,
	}}
	if err := tc.checkFuncDefinition(expected, fdTyped); err != nil {
		t.Fatalf("checkFuncDefinition(typed, matching) = %v, want nil", err)
	}
}

// TestCheckInstanceDefinitionRecursivelyTypeChecks asserts instance
// type matching walks each declared export and recursively type-checks.
//
// Spec: Explainer.md :920-982 instance subtyping.
// Wasmtime parallel: matching.rs:162 self.definition recursive walk.
func TestCheckInstanceDefinitionRecursivelyTypeChecks(t *testing.T) {
	builder := types.NewComponentTypesBuilder()
	innerParams := builder.InternTuple([]types.ValType{types.S32})
	innerResults := builder.InternTuple([]types.ValType{types.S32})
	// Build an InstanceTypeDef that declares one exported func `f: (s32)->(s32)`.
	innerFT := &types.TypeFunc{Params: innerParams, Results: innerResults}
	c := &Component{
		Types: builder.Finish(),
		TypeDefs: []TypeDef{
			{Kind: TypeDefKindFunc, Func: types.FuncTypeIdx(0)}, // the inner func type (slot 0)
			{Kind: TypeDefKindInstance, Instance: &InstanceTypeDef{
				Declarations: []InstanceDecl{
					{
						Kind: InstanceDeclKindExport,
						Export: &InstanceExport{
							Name:    "f",
							Kind:    ExportKindFunc,
							TypeIdx: ptrUint32(0),
						},
					},
				},
			}},
		},
	}
	tc := NewTypeChecker(c)
	expected := &ImportExternDesc{Kind: ImportExternDescInstance, TypeIdx: 1}

	// Matching actual: instance with export `f` of the correct type.
	matching := &InstanceDef{Exports: map[string]Definition{
		"f": &FuncDef{Type: innerFT},
	}}
	if err := tc.checkInstanceDefinition(expected, matching); err != nil {
		t.Fatalf("checkInstanceDefinition(matching) = %v, want nil", err)
	}

	// Mismatching actual: export `f` has wrong type (different tuple).
	wrongParams := builder.InternTuple([]types.ValType{types.S64})
	mismatching := &InstanceDef{Exports: map[string]Definition{
		"f": &FuncDef{Type: &types.TypeFunc{Params: wrongParams, Results: innerResults}},
	}}
	if err := tc.checkInstanceDefinition(expected, mismatching); err == nil {
		t.Fatalf("checkInstanceDefinition(mismatching) = nil, want error")
	}

	// Missing export actual: no `f` at all.
	missing := &InstanceDef{Exports: map[string]Definition{}}
	if err := tc.checkInstanceDefinition(expected, missing); err == nil {
		t.Fatalf("checkInstanceDefinition(missing) = nil, want error")
	}
}

func ptrUint32(v uint32) *uint32 { return &v }
```

- [ ] **Step F1.2: Run to confirm fail**

```bash
go test ./internal/component/ -run 'TestCheckFuncType|TestCheckFuncDefinition|TestCheckInstanceDefinition' -count=1 2>&1 | tail -20
```

Expected: failures reflecting the current buggy behavior.

- [ ] **Step F1.3: Rewrite `checkFuncType`**

Edit `internal/component/type_checker.go` at the existing `checkFuncType` method (around line 42):

```go
// checkFuncType compares two *types.TypeFunc via identity on the three
// spec-relevant fields: Async, Params, Results. ParamNames are metadata
// and not part of type equivalence.
//
// Spec: definitions.py:88-101 FuncType.param_types() / result_type()
// strip names.
// Wasmtime parallel: matching.rs InterfaceType::Tuple comparison.
//
// Cross-bag structural walk (where expected and actual come from different
// *types.ComponentTypes bags) stays deferred to Session 2.
func (tc *TypeChecker) checkFuncType(expected, actual *types.TypeFunc) error {
	if expected == nil || actual == nil {
		if expected != actual {
			return fmt.Errorf("function type mismatch: one side is nil")
		}
		return nil
	}
	if expected.Async != actual.Async {
		return fmt.Errorf("function async mismatch: expected %v, got %v", expected.Async, actual.Async)
	}
	if expected.Params != actual.Params {
		return fmt.Errorf("function params mismatch")
	}
	if expected.Results != actual.Results {
		return fmt.Errorf("function results mismatch")
	}
	return nil
}
```

- [ ] **Step F1.4: Rewrite `checkFuncDefinition`**

```go
// checkFuncDefinition asserts the actual host FuncDef has a non-nil Type
// and matches the expected import's declared type.
//
// Spec: wasmtime matching.rs:51 — untyped actual is rejected at match time.
func (tc *TypeChecker) checkFuncDefinition(expected *ImportExternDesc, actual Definition) error {
	fd, ok := actual.(*FuncDef)
	if !ok {
		return fmt.Errorf("import: expected function, got %T", actual)
	}
	if fd.Type == nil {
		return fmt.Errorf("import: host function has no type (DefineFunc must be called with a *types.TypeFunc)")
	}
	// Resolve the expected type via c.TypeDefs (Decision 5).
	if int(expected.TypeIdx) >= len(tc.component.TypeDefs) {
		return fmt.Errorf("import: type index %d out of range", expected.TypeIdx)
	}
	expectedTd := &tc.component.TypeDefs[expected.TypeIdx]
	if expectedTd.Kind != TypeDefKindFunc {
		return fmt.Errorf("import: type %d is not a function type", expected.TypeIdx)
	}
	expectedFT := &tc.component.Types.Funcs[expectedTd.Func]
	return tc.checkFuncType(expectedFT, fd.Type)
}
```

- [ ] **Step F1.5: Rewrite `checkInstanceDefinition`**

Use the full body from the design doc at lines 1510-1575. Adapt field names to match the actual `InstanceTypeDef.Declarations` / `Export.Kind` / `ExportKindFunc` constants.

- [ ] **Step F1.6: Run tests**

```bash
go test ./internal/component/ -run 'TestCheckFuncType|TestCheckFuncDefinition|TestCheckInstanceDefinition' -count=1 2>&1 | tail -10
```

Expected: all PASS.

- [ ] **Step F1.7: V11 verification**

```bash
grep -n '_ = expected' internal/component/type_checker.go
```

Expected: empty.

- [ ] **Step F1.8: Run per-task reviewers**

---

### Task F2: Restore `type_checker_test.go` (17 tests)

**Design reference:** File Manifest (line 1949).
**Files modified:** `internal/component/type_checker_test.go`.

- [ ] **Step F2.1: Apply methodology per test**

Citations reference `definitions.py:88-101` (FuncType equality), Explainer.md `:920-982` (instance subtyping), wasmtime `matching.rs:51, :162`.

---

### Task F3: Restore remaining `instance_test.go` (non-resource) lift/lower tests

**Design reference:** File Manifest (line 1938 — "lift/lower tests in F").
**Files modified:** `internal/component/instance_test.go`.

- [ ] **Step F3.1: Identify remaining skipped tests**

```bash
grep -n 'session 1 work' internal/component/instance_test.go
```

Expected: the remainder of the 57 total after Checkpoint B (accessor) and Checkpoint E (resource) partials.

- [ ] **Step F3.2: Restore each per methodology**

Citations reference `definitions.py:1065-1171` scalar ABI formulas, `run_tests.py::test_pairs`, `test_heap`.

---

### Task F4: Restore `edge_case_test.go` (1), `component_test.go` (any remaining), `composite_test.go` (5), `instantiate_test.go` (2), `integration_records_test.go` (2), `integration_public_api_test.go` (7)

**Design reference:** File Manifest lines 1945-1952.
**Files modified:** Each named test file.

- [ ] **Step F4.1 through F4.6: Restore each file per methodology**

One sub-step per file. Each sub-step: grep the skipped tests, pull pre-Session-0 body, restore with citation block, run, delete skip, commit.

---

### Task F5: Restore `conformance/error_messages_test.go`

**Design reference:** File Manifest (line 1966).
**Spec citation:** Error message invariants are wazero-specific; most cases `No counterpart (justified):`.
**Files modified:** `internal/component/conformance/error_messages_test.go`.

- [ ] **Step F5.1: Replace stub + restore**

---

### Task F6: Restore `conformance/instance_types_test.go`

**Design reference:** File Manifest (line 1967).
**Files modified:** `internal/component/conformance/instance_types_test.go`.

- [ ] **Step F6.1: Restore with citations referencing Explainer.md instance subtyping.**

---

### Task F7: Restore `conformance/memory_bounds_test.go`

**Design reference:** File Manifest (line 1969).
**Spec citation:** `definitions.py:1947-1948` bounds trap invariants.
**Files modified:** `internal/component/conformance/memory_bounds_test.go`.

- [ ] **Step F7.1: Restore**

---

### Task F8: Restore `conformance/realloc_failure_test.go`

**Design reference:** File Manifest (line 1974).
**Spec citation:** `definitions.py` realloc error propagation; wasmtime `func.rs` realloc adapter.
**Files modified:** `internal/component/conformance/realloc_failure_test.go`.

- [ ] **Step F8.1: Restore**

---

### Task F9: Restore `conformance/type_edge_cases_test.go`

**Design reference:** File Manifest (line 1977).
**Files modified:** `internal/component/conformance/type_edge_cases_test.go`.

- [ ] **Step F9.1: Restore**

---

### Task F10: Restore `conformance/utf_validation_test.go`

**Design reference:** File Manifest (line 1978).
**Spec citation:** `definitions.py` utf validation in string encoding.
**Files modified:** `internal/component/conformance/utf_validation_test.go`.

- [ ] **Step F10.1: Restore**

---

### Task F11: Restore WASI world conformance stubs (`wasi_cli_test.go`, `wasi_clocks_test.go`, `wasi_error_handling_test.go`, `wasi_filesystem_test.go`, `wasi_http_test.go`, `wasi_poll_test.go`, `wasi_random_test.go`, `wasi_resource_lifecycle_test.go`, `wasi_sockets_test.go`, `wasi_streams_test.go`)

**Design reference:** File Manifest (lines 1979-1988).
**Spec citation:** WASI preview-2 world definitions (see `debug-vendored/component-model/design/mvp/WASI.md` and `imports/wasip2/` WIT manifests).
**Files modified:** Each `conformance/wasi_*.go` file.

- [ ] **Step F11.1 through F11.10: One task per WASI world file**

Each task:
1. Replace `TestWASIXxxDeferredToSession1` stub with real multi-case suite.
2. Reference the WASI world's WIT file for each assertion (`No counterpart (justified): WASI world invariant`).
3. For resource-carrying WASI types (streams, pollables, http bodies), citations reference `run_tests.py::test_handles` where applicable.
4. Run + V4 grep + reviewers per file.

These tasks are the largest volume in Checkpoint F. Allow for 2-3 review iterations per file given the citation density.

---

### Task F12: Final V2/V3 verification

- [ ] **Step F12.1: V2 grep**

```bash
grep -rln 'session 1 work' internal/ api/ imports/
```

Expected: empty.

- [ ] **Step F12.2: V3 grep**

```bash
grep -rln 'DeferredToSession1' internal/component/conformance/
```

Expected: empty.

- [ ] **Step F12.3: V1 grep**

```bash
grep -rn 'panic("compile-fix' internal/component/
```

Expected: empty.

- [ ] **Step F12.4: Full repo test**

```bash
go test ./... -count=1 2>&1 | tail -50
```

Expected: all pass except `conformance/subtask_test.go` (`t.Skip("later work: async lift/lower")`).

- [ ] **Step F12.5: Working-tree integrity + dispatch checkpoint review**

Dispatch both reviewers over Checkpoint F scope. Apply correctives.

---

## Final — Session 2 followup note + full-suite green

### Task FINAL1: `go vet` + full suite

- [ ] **Step FINAL1.1: Vet**

```bash
go vet ./... 2>&1 | head -20
```

Expected: empty.

- [ ] **Step FINAL1.2: Full test suite**

```bash
go test ./... -count=1 -timeout 15m 2>&1 | tail -60
```

Expected: all pass except `conformance/subtask_test.go` (deferred-to-Later). No new skips introduced.

- [ ] **Step FINAL1.3: V4 grep across all restored test files**

Run the V4 python script over every file in `internal/component/` and `internal/component/conformance/` and verify every `func Test...` has a citation block.

---

### Task FINAL2: Write Session 2 followup note

**Design reference:** Followup Note — Session 2 Scope (design lines 2024-2041).
**Files modified:** `docs/plans/2026-04-08-canonical-abi-session1-followup.md` (new).

- [ ] **Step FINAL2.1: Write the followup**

Create `docs/plans/2026-04-08-canonical-abi-session1-followup.md` with:

```markdown
# Canonical-ABI Session 1 — Followup Note

**Date:** 2026-04-08 (end of Session 1 execution)
**Status:** Session 1 complete; Session 2 scope documented.
**Precursor:** `docs/superpowers/specs/2026-04-08-canonical-abi-session1-design.md`
**Implementation plan:** `docs/superpowers/plans/2026-04-08-canonical-abi-session1-plan.md`

## Session 1 end-state

- Zero panic stubs in `internal/component/instance.go`, `component_linker.go`, `nested_component.go`.
- All 223 tests previously marked `t.Skip("session 1 work")` are restored with spec citations.
- All 29 `TestXxxDeferredToSession1` conformance stubs replaced with multi-case suites citing `run_tests.py` where applicable.
- Four latent correctness gaps in `abi/lift.go` fixed per spec (Decision 4).
- `component.Instance` embeds `*runtime.ComponentInstance`; no duplicated runtime state.
- `ComponentLinker.Instantiate` rebuilt end-to-end with the 14-step pipeline.
- `Component.TypeDefs` is the single decoder → linker type-index source of truth.
- `DefineFunc` requires a typed `*types.TypeFunc` at registration time.
- Local-only Concrete promotion: each instantiation mints fresh `*runtime.ResourceType` per resource declaration.
- `runtime.ResourceHandleEntry.Rep` is `uint32` per spec.
- `runtime.ResourceType.HostDestructor` dispatches host-side resources in wasip2 modules.
- `BorrowScope.ReleaseBorrow` implements canon_resource_drop borrow branch.
- `byteMemory` test helper unblocks 11 abi bounds-check tests.

## Session 2 scope

### Cross-instance resource type resolution
- Extend `runtime.ComponentInstance.LookupResourceType` beyond the Parent chain via a linker-maintained or store-wide registry.
- Location: `internal/component/runtime/component_instance.go` + new `runtime.InstanceRegistry`.
- Removes the `"cross-instance resolution: session 2 wiring"` traps in `abi/lift.go::liftOwnHandle/liftBorrowHandle` and `instance.go::ResourceDrop`.

### Cross-component structural type checking
- Extend `type_checker.go::checkFuncType` beyond same-bag identity.
- New file: `internal/component/types/typecheck.go` with a recursive structural walk over `*types.ComponentTypes` entries.
- Handles host type bag ≠ component type bag scenarios.

### `TypeResourceTable.Concrete` promotion
- Wire the `Concrete` bit for cross-component matching, or document that it stays in Abstract state for local-only instantiation.

### Cross-instance destructor invocation
- Wire the `canon_lift` → `canon_lower` cross-call that invokes a destructor in a foreign instance (`definitions.py:2154-2160`).
- Removes the `"cross-instance destructor invocation: session 2 wiring"` trap in `Instance.ResourceDrop`.

## Later — Async lift/lower (no session scheduled)

- Stream / future / error-context / subtask support.
- `conformance/subtask_test.go` stays deferred.
- `abi/lift.go:306, 626` and `abi/lower.go:282, 597` stream/future/error-context trap arms stay as trap arms.

## Open items for Session 2 discovery

- Verify the Instantiator ID namespace (currently linker-local counter) is sufficient for cross-instance resource lookup, or migrate to a store-wide ID space.
- Audit any `t.Skip` added during Session 1 for "Session 2: cross-instance ..." and ensure they cite the spec line precisely.
- Verify no new wazero divergence from the spec has accumulated during Session 1 test restoration (the spec-compliance reviewers should catch these, but a final pass is prudent).
```

- [ ] **Step FINAL2.2: Commit the followup**

---

### Task FINAL3: Final self-review + commit

- [ ] **Step FINAL3.1: Self-review the plan execution**

Walk every checkpoint and verify:
- No `t.Skip("session 1 work")` remains.
- No `TestXxxDeferredToSession1` remains.
- No `panic("compile-fix` remains.
- No `// TODO`, `// FIXME`, `// XXX` remains.
- Every restored test has a citation block.
- Every `session 2` trap has a spec line citation.
- V1-V12 all pass.

- [ ] **Step FINAL3.2: Commit the Session 1 plan completion**

The plan file itself was committed at plan-authoring time. The final commit marks the end of Session 1 execution:

```bash
git log --oneline feat/wasip2-complete-implementation ^main | head -30
```

Expected: a linear history of per-task commits from Task A1 through Task FINAL2.

- [ ] **Step FINAL3.3: Dispatch the final whole-session review**

Dispatch `superpowers:code-reviewer` with scope "the entire Session 1 branch delta". Dispatch spec-compliance reviewer with the Session 1 checklist applied to the whole branch. Report any outstanding correctives.

---

## Open Questions

**None.** Every task maps to a design decision or section in `docs/superpowers/specs/2026-04-08-canonical-abi-session1-design.md`. Every task has a spec/canonical/wasmtime citation. Every deferral to Session 2 or Later carries a spec line reference. Per the self-review in the plan-authoring session:

- **Coverage:** every Checkpoint A-F scope item in the design's Work Order + Checkpoint Gates table (design lines 1849-1858) has at least one task in the corresponding checkpoint section above.
- **TDD sequencing:** every non-housekeeping task writes a failing test before the production change, runs to confirm failure, implements, runs to confirm pass.
- **Per-task reviewers:** every task ends with a reviewer-dispatch step invoking both `superpowers:code-reviewer` and a spec-compliance reviewer with the Session 1 amended checklist.
- **No deferred stubs:** no task says "stub X, leave Y for later." Every deferral is an explicit spec-cited trap with a precise error message.
- **No circular dependencies:** Task B4 (call-site migration) depends on B3 (struct rewrite); Task C5 onwards depend on A-B; Task E5 (Instance.ResourceNew) depends on E1 (GetByIndex) + E2 (Rep-uint32) + E3 (BorrowScope.ReleaseBorrow) + E4 (host registries). Task C8 depends on C1-C7. No cycles.
- **`byteMemory` matches the full design spec** (Task E7 reproduces the design doc body at lines 1716-1840 verbatim).
- **`abi.LowerParams` / `LiftParams` / `LiftResults` / `LowerResults`** have dedicated tasks (C1, C2) with their own TDD tests.
- **`runtime.ResourceType.HostDestructor` + wasip2 registry migration** has dedicated tasks E2, E3, E4.
- **Gaps 1-4 in `lift.go`** have a single dedicated task (E6) with one failing test per gap.
- **`CallMightBeRecursive` transitive check** has a dedicated task (B2).
- **`IsMayLeave` semantic fix** has a dedicated task (B1).
- **`DefineFunc` signature change** has a dedicated task (C3) with call-site migration across `imports/wasip2/`.

If the executor encounters an ambiguity or missing detail during execution, they MUST stop and resolve it from the design doc (`docs/superpowers/specs/2026-04-08-canonical-abi-session1-design.md`), NOT from wazero's current broken implementation or from external spec documents beyond the Source-of-Truth list at the top of this plan.
