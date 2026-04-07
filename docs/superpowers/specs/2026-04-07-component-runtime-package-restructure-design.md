# Component Runtime Package Restructure — Design

**Date:** 2026-04-07
**Branch:** `feat/wasip2-complete-implementation`
**Status:** Approved for implementation planning

## Problem

`internal/component/abi/` currently imports `internal/component`. This is the
wrong direction: `abi/` is supposed to be a leaf-level primitive that the
top-level `component/` package builds on, not the other way around. Twelve
files in `abi/` (production and test) reference `component.Val*`,
`component.ResourceTable`, `component.Handle`, `component.CallContext`,
`component.NewSubtask`, `component.BorrowScope`, `component.NewResourceTypeInfo`,
and related identifiers.

The fix is to move the symbols `abi/` needs out of `internal/component/` and
into lower layers, so that `abi/` depends only on packages strictly beneath it.

## Goal

End state: `grep -rn '"github.com/tetratelabs/wazero/internal/component"' internal/component/abi/`
returns zero matches, while `go build ./...`, `go test ./...`, and `go vet ./...`
all pass and the public API surface in `api/component/component.go` is
unchanged in name, kind, and effective signature.

## Final package layout

```
internal/component/types/      static ValType + dynamic Val (paired)
internal/component/runtime/    NEW: handles, tables, contexts, registries, trackers
internal/component/abi/        lift/lower (depends only on types + runtime)
internal/component/            Component, Instance, Linker, canon_lower, ... (top-level)
```

## Dependency direction

Strict downward-only:

```
component/  →  abi  →  runtime  →  types  →  (stdlib)
                          ↘             ↗
                           (no upward edges)
```

`runtime → types` exists solely because `Subtask.result` is `[]types.Val`.
`types/` itself depends on nothing inside `internal/component/`. No cycles.

## Section 1 — `component/types` contents

### Already present (untouched)
- `types.go` — `ValType` interface + primitive types (Bool, S8…U64, F32, F64, Char, String)
- `composite.go` — Record, Variant, Tuple, Option, Result, Enum, Flags, List
- `resource.go` — Own, Borrow, ResourceType
- existing `*_test.go` files

### Moved IN from `internal/component/val.go`
The entire file (~348 lines) becomes `internal/component/types/val.go`:
- `Val` struct, `ValKind` type, all `ValKind*` constants
- Constructors: `ValBool`, `ValS8`…`ValU64`, `ValF32`, `ValF64`, `ValChar`,
  `ValString`, `ValRecord`, `ValVariant`, `ValOption`, `ValList`, `ValTuple`,
  `ValResultOk`, `ValResultError`, `ValFlags`, `ValEnum`, `ValOwn`, `ValBorrow`
- Accessors: `Bool()`, `S32()`, `Record()`, `Variant()`, `Option()`, `List()`,
  `Tuple()`, `Result()`, `Flags()`, `Enum()`, `Own()`, `Borrow()`,
  `StringVal()`, etc.
- Internal helpers: `variantVal`, `resultVal`
- `ValKind.String()`

`val_test.go` moves alongside (becomes `internal/component/types/val_test.go`).

### Naming
All identifiers keep their current names. `component.Val` becomes `types.Val`,
`component.ValS32` becomes `types.ValS32`, etc. Verified there is no name
collision in `types/` today (no symbol named `Val` in `types.go`,
`composite.go`, or `resource.go`).

## Section 2 — `component/runtime` contents

A new package created from these files (moved verbatim with `git mv`, only the
`package` declaration changes from `package component` to `package runtime`):

| Source path | Destination path |
|---|---|
| `internal/component/resource_table.go` (659 lines) | `runtime/resource_table.go` |
| `internal/component/resource_table_test.go` (1240 lines) | `runtime/resource_table_test.go` |
| `internal/component/resource_type_id.go` (66 lines) | `runtime/resource_type_id.go` |
| `internal/component/resource_type_id_test.go` (44 lines) | `runtime/resource_type_id_test.go` |
| `internal/component/borrow_scope.go` (51 lines) | `runtime/borrow_scope.go` |
| `internal/component/borrow_scope_test.go` (54 lines) | `runtime/borrow_scope_test.go` |
| `internal/component/call_context.go` (90 lines) | `runtime/call_context.go` |
| `internal/component/call_context_test.go` (122 lines) | `runtime/call_context_test.go` |
| `internal/component/subtask.go` (109 lines) | `runtime/subtask.go` |
| `internal/component/instance_state.go` (60 lines) | `runtime/instance_state.go` |
| `internal/component/instance_state_test.go` (61 lines) | `runtime/instance_state_test.go` |
| `internal/component/destructor.go` (41 lines) | `runtime/destructor.go` |
| `internal/component/destructor_test.go` (58 lines) | `runtime/destructor_test.go` |
| `internal/component/reentrance.go` (55 lines) | `runtime/reentrance.go` |
| `internal/component/reentrance_test.go` (47 lines) | `runtime/reentrance_test.go` |

### Symbols exposed by `runtime/`
- Resource handles: `Handle`, `MakeHandle`, `HandleEntry`, `Destroyable`,
  `MaxTableLength`
- Tables: `ResourceTable`, `NewResourceTable`
- Errors: `ErrInvalidHandle`, `ErrHandleNotOwned`, `ErrResourceInUse`,
  `ErrNoBorrowsToDecrement`, `ErrResourceTypeMismatch`, `ErrMayNotLeave`,
  `ErrReentrance`, `ErrTableFull`, `ErrOutstandingBorrows`
- Trap/destructor wiring: `TrapHandler`, `CrossInstanceDestructor`,
  `DestructorFunc`, `DestructorRegistry`, `NewDestructorRegistry`
- Resource type identification: `ResourceTypeID`, `NewResourceTypeID`,
  `InvalidResourceTypeID`, `ResourceTypeInfo`, `NewResourceTypeInfo`
- Borrow tracking: `BorrowScope`, `NewBorrowScope`, `CallContext`,
  `NewCallContext`
- Subtask machinery: `Subtask`, `SubtaskState`, `SubtaskStatePending`,
  `SubtaskStateResolved`, `SubtaskStateFinishing`, `SubtaskStateDone`,
  `NewSubtask`
- Instance state: `InstanceState`, `NewInstanceState`
- Reentrance tracking: `ReentranceTracker`, `NewReentranceTracker`

### `runtime → types` edge
`Subtask.result` is `[]Val`. After the move, `runtime/subtask.go` imports
`internal/component/types` and uses `[]types.Val`. This is the only edge from
`runtime` into another `internal/component/*` package, and it points downward.
`types/` has no dependency on `runtime/` and no other `internal/component/*`
import.

## Section 3 — What stays in `internal/component/` (top-level)

The top-level `internal/component/` package remains as the orchestration
layer. After the move it depends on `types`, `runtime`, and `abi`.

### Files that stay (unchanged in location)
- `component.go` — `Component`, `TypeDef`, `RecordTypeDef`, `VariantTypeDef`,
  `FuncType`, `HostFunc`, etc.
- `component_linker.go` (3809 lines), `linker.go`, `linker_api.go`
- `component_linker_test.go`, `linker_test.go`, `linker_api_test.go`
- `instance.go` (2908 lines), `instance_test.go` (2327 lines)
- `canon_lower.go` (619 lines) — `EnumType`, `FlagsType`, `VariantType`,
  `LoweredFunc`, `CanonicalOptions`. This is the spec-execution glue layered
  on top of `abi/`'s lift/lower primitives. The name overlap with
  `abi/lower.go` is intentional and orthogonal — not addressed here.
- `canon_lower_test.go`
- `instantiate.go`, `instantiate_test.go`
- `compiled.go`, `compiled_test.go`
- `nested_component.go`, `nested_component_test.go`
- `index_space.go`, `index_space_test.go`
- `import_name.go`, `import_name_test.go`
- `outer_alias.go`, `outer_alias_test.go`
- `semver.go`, `semver_test.go`
- `type_checker.go`, `type_checker_test.go`
- `type_resolver.go`, `type_resolver_test.go`
- `context.go`, `context_test.go` — context helpers; will need updating since
  `*ResourceTable` now lives in `runtime`
- Integration and edge-case tests that stay in place:
  `composite_test.go`, `component_test.go`, `edge_case_test.go`,
  `integration_public_api_test.go`, `integration_records_test.go`,
  `integration_test.go`, `start_function_test.go`,
  `value_import_test.go`
- `doc.go`

### Files that LEAVE `internal/component/`
- `val.go`, `val_test.go` → `internal/component/types/`
- 15 files listed in Section 2 → `internal/component/runtime/`

## Section 4 — Import rewrites required across the tree

Every file that currently uses `component.X` for one of the moved symbols
must be updated to `types.X` or `runtime.X`.

1. **`internal/component/abi/*.go` (12 files) — the goal commit.**
   Drop `"github.com/tetratelabs/wazero/internal/component"` entirely. Add
   `"github.com/tetratelabs/wazero/internal/component/runtime"` to every
   file that needs it, and add `"github.com/tetratelabs/wazero/internal/component/types"`
   to any file that newly references `types.Val*` (some `abi/` files such as
   `lift.go`, `lower.go`, `flatten.go`, and a few of their tests already
   import `types`; others such as `context.go`, `resource_lower.go`, and
   `strings.go` do not yet and will need it added). Replace identifiers:
   `component.Val*` → `types.Val*`, `component.ResourceTable` →
   `runtime.ResourceTable`, `component.Handle` → `runtime.Handle`,
   `component.MakeHandle` → `runtime.MakeHandle`, `component.CallContext` →
   `runtime.CallContext`, `component.NewCallContext` → `runtime.NewCallContext`,
   `component.NewResourceTable` → `runtime.NewResourceTable`,
   `component.BorrowScope` → `runtime.BorrowScope`, `component.NewBorrowScope`
   → `runtime.NewBorrowScope`, `component.NewResourceTypeInfo` →
   `runtime.NewResourceTypeInfo`, `component.ResourceTypeInfo` →
   `runtime.ResourceTypeInfo`, `component.NewSubtask` → `runtime.NewSubtask`,
   `component.Subtask` → `runtime.Subtask`. Heavy churn in
   `lower_test.go` (1855 lines) and `lift_test.go` (2794 lines), but the
   replacement is mechanical.

2. **`internal/component/*.go` (top-level) and their tests.** The top-level
   package previously used these as in-package identifiers. After the move,
   it imports `internal/component/types` and `internal/component/runtime` and
   uses qualified references. Largest files affected: `instance.go`,
   `component_linker.go`, `canon_lower.go`. `context.go` updates its
   `*ResourceTable` references to `*runtime.ResourceTable`. Test files in
   this directory updated in lockstep.

3. **`internal/component/conformance/*.go` (~15 files).** Replace
   `component.Val*` / `component.ResourceTable` / etc. with `types.*` /
   `runtime.*` references. Some files already import
   `internal/component/types`; they gain `internal/component/runtime` if
   needed.

4. **`internal/component/wasip2test/*.go`.** Update imports as needed.

5. **`internal/component/binary/*.go`.** This package depends on
   `internal/component` for `TypeDef` / `FuncType` symbols which stay in
   `component/`. Likely no rewrites required, but verified during execution.

6. **`imports/wasip2/**/*.go`.** Files that use `component.NewResourceTable`,
   `component.Val*`, etc. switch to `runtime.NewResourceTable` / `types.Val*`.

7. **`api/component/component.go` — public type aliases.** Update the
   right-hand sides only:
   ```go
   type Val      = types.Val
   type ValKind  = types.ValKind
   type ResourceTable = runtime.ResourceTable
   var  ValS32 = types.ValS32
   var  NewResourceTable = runtime.NewResourceTable
   func WithResourceTable(ctx, table) ... { return component.WithResourceTable(ctx, table) }
   ```
   Context helpers stay routed through `internal/component` because
   `context.go` lives there. The exported names (`component.Val`,
   `component.ValS32`, `component.ResourceTable`, `component.WithResourceTable`,
   etc.) are preserved exactly.

8. **`runtime.go` (top-level wazero).** Uses high-level types only; no
   change expected, verified by build.

### Public API invariant
Every type alias and var in `api/component/component.go` keeps the same
exported name and the same effective type / function signature. External
callers see no change. Verified by `go build ./...` and by running tests in
`api/component/` and any integration test that touches `component.Val*` or
`component.ResourceTable`.

## Section 5 — Execution order

Each step is a separate commit so the history is bisectable. Mid-step build
failures are acceptable inside a step but each commit is described to
minimize the broken-build window.

1. **Create `internal/component/runtime/`** and `git mv` the 15 files from
   Section 2. Edit each `package component` declaration to `package runtime`.
   Build will fail until step 3.

2. **Move `val.go` and `val_test.go`** into `internal/component/types/` with
   `git mv`. Update `package component` → `package types` in each. Verify no
   name collision with existing `types/` symbols.

3. **Add the `runtime → types` edge** in `runtime/subtask.go`: import
   `internal/component/types` and change `[]Val` to `[]types.Val`. After
   this commit, `types/` and `runtime/` both compile cleanly in isolation
   (`go build ./internal/component/types/... ./internal/component/runtime/...`).

4. **Rewrite `internal/component/abi/*.go`** to drop the
   `internal/component` import and use `types.*` / `runtime.*` qualified
   references. Includes all `abi/*_test.go` files. After this commit:
   `grep -rn '"github.com/tetratelabs/wazero/internal/component"' internal/component/abi/`
   returns zero matches.

5. **Rewrite `internal/component/*.go` (top-level)** to use `types` and
   `runtime` packages explicitly. Test files in this directory updated in
   lockstep.

6. **Rewrite `internal/component/conformance/`,
   `internal/component/wasip2test/`, `internal/component/binary/`** as
   needed.

7. **Rewrite `imports/wasip2/**/*.go`** as needed.

8. **Update `api/component/component.go`** type aliases and var assignments
   to point at `types` and `runtime`.

9. **Build, test, and vet verification:**
   - `go build ./...`
   - `go test ./internal/component/... ./imports/wasip2/... ./api/component/...`
   - `go vet ./...`
   - Final goal grep: empty.

## Section 6 — Risks and mitigations

- **Hidden cyclic dependency.** `BorrowScope → ResourceTable`,
  `Subtask → BorrowScope`, `Subtask → []Val` are all internal to `runtime/`
  except the `Val` reference, which becomes `types.Val` (downward).
  Mitigation: verified by Go compiler in step 3.

- **Name collision in `types/` between `Val` and existing symbols.** Verified
  by reading `types.go`, `composite.go`, `resource.go` — no `Val` symbol
  exists today. Safe.

- **Public surface drift in `api/component/component.go`.** Mitigated by not
  renaming any public-facing symbol; type aliases (`type Val = types.Val`)
  preserve identity at the type level so users' code continues to compile;
  variable aliases (`var ValS32 = types.ValS32`) preserve calling convention.
  Verified by tests in `api/component/` and any wazero example/integration
  test that touches `component.Val*` or `component.ResourceTable`.

- **In-progress ABI unification work (loop-1).** That effort unifies the
  static type representation across `binary`, `component`, and `canon_lower`.
  It is orthogonal to this dependency-direction fix. Potential merge conflict
  in `canon_lower.go`. Mitigation: do this restructure as a single focused
  branch and rebase if needed.

- **Enormous diff in `abi/*_test.go`.** The find/replace is mechanical.
  Mitigation: split into per-symbol logical commits if review burden is too
  high; otherwise do as one commit since the change is uniform.

- **Forgotten caller in `imports/wasip2/...`.** Caught by `go build ./...` in
  step 9.

- **Unchanged external public API but secretly broken alias semantics.**
  Caught by `api/component/`'s tests + `runtime.go`'s tests.

## Out of scope

- No renaming of any symbol.
- No splitting of the giant files (`instance.go` 2908 lines,
  `component_linker.go` 3809 lines).
- No behavior changes — pure mechanical move + import rewrite.
- No changes to existing `types/` definitions for static `ValType`.
- Not removing `canon_lower.go` from `internal/component/` despite the name
  overlap with `abi/lower.go`.

## Definition of done

1. `grep -rn '"github.com/tetratelabs/wazero/internal/component"' internal/component/abi/`
   returns zero matches.
2. `go build ./...` passes.
3. `go test ./...` passes (or at least the same tests that pass on the base
   branch continue to pass — pre-existing flaky tests are not in scope).
4. `go vet ./...` passes.
5. Public symbols in `api/component/component.go` are unchanged in name,
   kind (type alias vs var vs func), and effective type signature.
