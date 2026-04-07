# Component Runtime Package Restructure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Eliminate every `internal/component` import from `internal/component/abi/` by moving the dynamic `Val` type into `internal/component/types` and the runtime state (handles, tables, contexts, registries, trackers) into a new `internal/component/runtime` package, then rewriting every caller across the tree to use the new locations. The public API surface in `api/component/component.go` is preserved.

**Architecture:** Pure mechanical move + import rewrite. No behavior changes, no renames. Final dependency direction: `component/ → abi/ → runtime/ → types/`. The only new edge is `runtime → types` to satisfy `Subtask.result []Val`.

**Tech Stack:** Go 1.x, `git mv`, `gofmt`, `go build`, `go test`, `go vet`. Repo-wide find/replace via `grep` + `sed` is acceptable for the mechanical identifier swaps as long as each step is verified by build + tests before committing.

**Spec:** `docs/superpowers/specs/2026-04-07-component-runtime-package-restructure-design.md`

---

## Pre-flight: working directory and baseline

### Task 0: Baseline build and test

**Files:** none

- [ ] **Step 1: Confirm clean working tree**

Run: `git status`
Expected: only the spec and plan files are present as untracked / modified, no other dirty state.

- [ ] **Step 2: Capture baseline build**

Run: `go build ./... 2>&1 | tee /tmp/wazero-baseline-build.log`
Expected: exit 0, no errors.

- [ ] **Step 3: Capture baseline test result**

Run: `go test ./internal/component/... ./imports/wasip2/... ./api/component/... 2>&1 | tee /tmp/wazero-baseline-test.log`
Expected: exit 0. If any tests are flaky on the base branch, note them now — they are out of scope for this refactor.

- [ ] **Step 4: Capture baseline vet**

Run: `go vet ./... 2>&1 | tee /tmp/wazero-baseline-vet.log`
Expected: exit 0.

---

## Phase 1 — Move `Val` into `internal/component/types`

### Task 1: Move `val.go` to `internal/component/types/val.go`

**Files:**
- Move: `internal/component/val.go` → `internal/component/types/val.go`

- [ ] **Step 1: Verify no name collision exists**

Run: `grep -nE "^(type|func|var|const) Val(\b|[A-Z])" internal/component/types/types.go internal/component/types/composite.go internal/component/types/resource.go`
Expected: no matches. (`ValType` is the only existing `Val`-prefix symbol, and it does not collide with `Val`.)

- [ ] **Step 2: Move the file with git**

Run: `git mv internal/component/val.go internal/component/types/val.go`
Expected: file moved, git tracks the rename.

- [ ] **Step 3: Update the package declaration**

Edit `internal/component/types/val.go` line 3:
- Change `package component` → `package types`

The file at line 3 must read exactly:
```go
package types
```

Run: `head -5 internal/component/types/val.go`
Expected: shows the new package line.

### Task 2: Move `val_test.go` to `internal/component/types/val_test.go`

**Files:**
- Move: `internal/component/val_test.go` → `internal/component/types/val_test.go`

- [ ] **Step 1: Move with git**

Run: `git mv internal/component/val_test.go internal/component/types/val_test.go`

- [ ] **Step 2: Update the package declaration**

Edit `internal/component/types/val_test.go` line 1:
- Change `package component` → `package types`

Run: `head -3 internal/component/types/val_test.go`
Expected: shows `package types`.

- [ ] **Step 3: Resolve in-package identifier references**

The original test file used unqualified names (`Val`, `ValS32`, `ValKindBool`, etc.) because it was in `package component`. After the package change those names still work because everything from `val.go` is now in `package types`. But the test file may reference *other* `component` package symbols (e.g. `ResourceTable`, `Handle`).

Run: `grep -nE "\\b(ResourceTable|Handle|MakeHandle|HandleEntry|CallContext|NewCallContext|BorrowScope|NewBorrowScope|Subtask|NewSubtask|InstanceState|NewInstanceState|DestructorRegistry|NewDestructorRegistry|ReentranceTracker|NewReentranceTracker|ResourceTypeID|NewResourceTypeID|ResourceTypeInfo|NewResourceTypeInfo)\\b" internal/component/types/val_test.go`

Expected: NO matches. The original `val_test.go` is a pure value-type test (it only constructs and inspects `Val` objects), so it should not reference runtime state. If matches appear, those tests must be left in `internal/component/val_test.go` and only the relevant tests moved — but we expect a clean move.

If matches DO appear: revert step 1 (`git mv internal/component/types/val_test.go internal/component/val_test.go`), then split the file by hand: copy only the `Val`-only test functions into `internal/component/types/val_test.go` (as `package types`) and leave the rest behind.

### Task 3: Build the `types` package in isolation

**Files:** none

- [ ] **Step 1: Run a focused build**

Run: `go build ./internal/component/types/...`
Expected: exit 0, no errors.

If errors mention missing identifiers from the original `package component` (e.g. references to `Handle`, `ResourceTable`), revisit Task 2 step 3 and split the test file.

- [ ] **Step 2: Run focused tests for the types package**

Run: `go test ./internal/component/types/...`
Expected: PASS for all tests, including the moved val tests.

### Task 4: Commit Phase 1

**Files:** the two moved files.

- [ ] **Step 1: Stage and commit**

Note: `git mv` already stages both the deletion of the source path and
the addition of the destination path. The `git add` below is just to
catch any post-mv edits to the destination files (the package
declaration changes).

Run:
```bash
git add internal/component/types/val.go internal/component/types/val_test.go
git commit -m "$(cat <<'EOF'
refactor(component/types): move Val into types package

Move the dynamic Val type, ValKind enum, all constructors, and accessors
from internal/component/val.go into internal/component/types/val.go,
co-located with the static ValType definitions. The types package now
holds both the static type description (ValType) and the dynamic value
representation (Val) for component-model values.

This is the first step of the component/runtime restructure that
eliminates the wrong-direction component/abi → component/ import.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

Note: at this point the tree does NOT compile because callers in
`internal/component/`, `internal/component/abi/`, etc. still reference
`component.Val*`. That's expected — the broken-build window stays open
through Phase 4. We commit anyway because the rename itself is atomic and
reviewable. (If you prefer a green-build-only history, do Phase 1 + 2 + 3
in a single commit instead. Either approach is acceptable.)

---

## Phase 2 — Create `internal/component/runtime` package

### Task 5: Create the runtime directory and move resource_table files

**Files:**
- Create: `internal/component/runtime/` (directory)
- Move: `internal/component/resource_table.go` → `internal/component/runtime/resource_table.go`
- Move: `internal/component/resource_table_test.go` → `internal/component/runtime/resource_table_test.go`

- [ ] **Step 1: Create the runtime directory**

Run: `mkdir -p internal/component/runtime`
Expected: directory exists.

- [ ] **Step 2: Move resource_table files**

Run:
```bash
git mv internal/component/resource_table.go internal/component/runtime/resource_table.go
git mv internal/component/resource_table_test.go internal/component/runtime/resource_table_test.go
```

- [ ] **Step 3: Update package declarations**

Edit each moved file to change `package component` → `package runtime`.

For `internal/component/runtime/resource_table.go` line 3:
```go
package runtime
```

For `internal/component/runtime/resource_table_test.go` line 1:
```go
package runtime
```

Run: `head -5 internal/component/runtime/resource_table.go internal/component/runtime/resource_table_test.go | grep -E '^package'`
Expected: both show `package runtime`.

### Task 6: Move resource_type_id files

**Files:**
- Move: `internal/component/resource_type_id.go` → `internal/component/runtime/resource_type_id.go`
- Move: `internal/component/resource_type_id_test.go` → `internal/component/runtime/resource_type_id_test.go`

- [ ] **Step 1: Move with git**

Run:
```bash
git mv internal/component/resource_type_id.go internal/component/runtime/resource_type_id.go
git mv internal/component/resource_type_id_test.go internal/component/runtime/resource_type_id_test.go
```

- [ ] **Step 2: Update package declarations**

Edit both files: `package component` → `package runtime`.

Run: `head -5 internal/component/runtime/resource_type_id.go internal/component/runtime/resource_type_id_test.go | grep -E '^package'`
Expected: both show `package runtime`.

### Task 7: Move borrow_scope files

**Files:**
- Move: `internal/component/borrow_scope.go` → `internal/component/runtime/borrow_scope.go`
- Move: `internal/component/borrow_scope_test.go` → `internal/component/runtime/borrow_scope_test.go`

- [ ] **Step 1: Move with git**

Run:
```bash
git mv internal/component/borrow_scope.go internal/component/runtime/borrow_scope.go
git mv internal/component/borrow_scope_test.go internal/component/runtime/borrow_scope_test.go
```

- [ ] **Step 2: Update package declarations**

Edit both files: `package component` → `package runtime`.

### Task 8: Move call_context files

**Files:**
- Move: `internal/component/call_context.go` → `internal/component/runtime/call_context.go`
- Move: `internal/component/call_context_test.go` → `internal/component/runtime/call_context_test.go`

- [ ] **Step 1: Move with git**

Run:
```bash
git mv internal/component/call_context.go internal/component/runtime/call_context.go
git mv internal/component/call_context_test.go internal/component/runtime/call_context_test.go
```

- [ ] **Step 2: Update package declarations**

Edit both files: `package component` → `package runtime`.

### Task 9: Move subtask file and add types import

**Files:**
- Move: `internal/component/subtask.go` → `internal/component/runtime/subtask.go`

- [ ] **Step 1: Move with git**

Run: `git mv internal/component/subtask.go internal/component/runtime/subtask.go`

- [ ] **Step 2: Update package declaration and add types import**

Edit `internal/component/runtime/subtask.go`:

Change:
```go
package component

import "fmt"
```

To:
```go
package runtime

import (
	"fmt"

	"github.com/tetratelabs/wazero/internal/component/types"
)
```

- [ ] **Step 3: Update `[]Val` references to `[]types.Val`**

In `internal/component/runtime/subtask.go`, replace every `[]Val` with `[]types.Val`:

Original (line ~24):
```go
result      []Val // Stored result after resolve
```

New:
```go
result      []types.Val // Stored result after resolve
```

Original (line ~48):
```go
func (s *Subtask) DeliverResolve(result []Val) error {
```

New:
```go
func (s *Subtask) DeliverResolve(result []types.Val) error {
```

Original (line ~90):
```go
func (s *Subtask) Result() []Val {
```

New:
```go
func (s *Subtask) Result() []types.Val {
```

Run: `grep -nE '(\[\])?\bVal\b' internal/component/runtime/subtask.go`
Expected: no matches (every `Val` is now `types.Val`).

### Task 10: Move instance_state files

**Files:**
- Move: `internal/component/instance_state.go` → `internal/component/runtime/instance_state.go`
- Move: `internal/component/instance_state_test.go` → `internal/component/runtime/instance_state_test.go`

- [ ] **Step 1: Move with git**

Run:
```bash
git mv internal/component/instance_state.go internal/component/runtime/instance_state.go
git mv internal/component/instance_state_test.go internal/component/runtime/instance_state_test.go
```

- [ ] **Step 2: Update package declarations**

Edit both files: `package component` → `package runtime`.

### Task 11: Move destructor files

**Files:**
- Move: `internal/component/destructor.go` → `internal/component/runtime/destructor.go`
- Move: `internal/component/destructor_test.go` → `internal/component/runtime/destructor_test.go`

- [ ] **Step 1: Move with git**

Run:
```bash
git mv internal/component/destructor.go internal/component/runtime/destructor.go
git mv internal/component/destructor_test.go internal/component/runtime/destructor_test.go
```

- [ ] **Step 2: Update package declarations**

Edit both files: `package component` → `package runtime`.

### Task 12: Move reentrance files

**Files:**
- Move: `internal/component/reentrance.go` → `internal/component/runtime/reentrance.go`
- Move: `internal/component/reentrance_test.go` → `internal/component/runtime/reentrance_test.go`

- [ ] **Step 1: Move with git**

Run:
```bash
git mv internal/component/reentrance.go internal/component/runtime/reentrance.go
git mv internal/component/reentrance_test.go internal/component/runtime/reentrance_test.go
```

- [ ] **Step 2: Update package declarations**

Edit both files: `package component` → `package runtime`.

### Task 13: Verify the runtime package builds and tests in isolation

**Files:** none

- [ ] **Step 1: Build the runtime package alone**

Run: `go build ./internal/component/runtime/...`
Expected: exit 0, no errors.

If you see errors about missing identifiers (e.g. `undefined: Handle` from a *_test.go file inside `runtime/`), it means a test file needed something that lives outside `runtime`. The likely culprit is a test referencing a top-level `component.X` symbol that wasn't moved. Inspect, then either move that symbol or fix the test in place.

- [ ] **Step 2: Run runtime package tests**

Run: `go test ./internal/component/runtime/...`
Expected: all tests pass.

- [ ] **Step 3: Confirm runtime has only stdlib + types imports**

Run: `grep -hn '^\s*"' internal/component/runtime/*.go | grep -E 'tetratelabs/wazero' | sort -u`
Expected: only `"github.com/tetratelabs/wazero/internal/component/types"` (from `subtask.go`). No other internal wazero imports.

### Task 14: Commit Phase 2

- [ ] **Step 1: Stage and commit**

Note: `git mv` already staged the source-side deletions. We only need to
stage the destination directory (which catches the package-declaration
edits and the new types-import edit in subtask.go).

Run:
```bash
git add internal/component/runtime/
git commit -m "$(cat <<'EOF'
refactor(component/runtime): create runtime package for canonical-ABI state

Move resource tables, handles, type IDs, borrow scopes, call contexts,
subtasks, instance state, destructor registry, and reentrance tracker
into a new internal/component/runtime package. These are the runtime-state
primitives the canonical ABI executes against.

Subtask gains an explicit dependency on internal/component/types so that
its result []types.Val field stays statically typed. This is the only
edge from runtime into another internal/component/* package, and it
points downward (runtime → types).

Top-level internal/component/ and its callers do not yet compile after
this commit; they are rewritten in subsequent commits.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 3 — Rewrite `internal/component/abi/` (the goal commit)

This phase eliminates every `internal/component` import from `abi/`. The
replacements are mechanical: `component.Val*` → `types.Val*`,
`component.ResourceTable` → `runtime.ResourceTable`, etc.

### Task 15: Rewrite `abi/context.go`

**Files:**
- Modify: `internal/component/abi/context.go`

- [ ] **Step 1: Update imports**

Replace:
```go
import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/tetratelabs/wazero/internal/component"
)
```

With:
```go
import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/tetratelabs/wazero/internal/component/runtime"
)
```

- [ ] **Step 2: Replace identifiers**

Replace every `component.ResourceTable` with `runtime.ResourceTable`.
Replace every `component.BorrowScope` with `runtime.BorrowScope`.
Replace every `component.CallContext` with `runtime.CallContext`.
Replace every `component.Subtask` with `runtime.Subtask`.

The struct field updates (around lines 75-77 and 164-172):

Original `LiftContext`:
```go
type LiftContext struct {
	Memory        Memory
	Opts          *Options
	ResourceTable *component.ResourceTable
	BorrowScope   *component.BorrowScope
}
```

New:
```go
type LiftContext struct {
	Memory        Memory
	Opts          *Options
	ResourceTable *runtime.ResourceTable
	BorrowScope   *runtime.BorrowScope
}
```

Original `LowerContext`:
```go
type LowerContext struct {
	Memory        Memory
	Opts          *Options
	Realloc       func(oldPtr, oldSize, align, newSize uint32) (uint32, error)
	ResourceTable *component.ResourceTable
	CallContext   *component.CallContext
	Instance interface{}
	Subtask *component.Subtask
}
```

New:
```go
type LowerContext struct {
	Memory        Memory
	Opts          *Options
	Realloc       func(oldPtr, oldSize, align, newSize uint32) (uint32, error)
	ResourceTable *runtime.ResourceTable
	CallContext   *runtime.CallContext
	Instance interface{}
	Subtask *runtime.Subtask
}
```

The `BorrowScope()` method (around line 176):
```go
func (c *LowerContext) BorrowScope() *component.BorrowScope {
```

Becomes:
```go
func (c *LowerContext) BorrowScope() *runtime.BorrowScope {
```

- [ ] **Step 3: Verify no `component.` references remain in this file**

Run: `grep -n 'component\.' internal/component/abi/context.go`
Expected: no output.

### Task 16: Rewrite `abi/lift.go`

**Files:**
- Modify: `internal/component/abi/lift.go`

- [ ] **Step 1: Update imports**

Replace:
```go
import (
	"fmt"
	"math"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/types"
)
```

With:
```go
import (
	"fmt"
	"math"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
)
```

- [ ] **Step 2: Replace `component.Val*` identifiers with `types.Val*`**

Use sed for the bulk replacement:
```bash
sed -i \
  -e 's/component\.Val{/types.Val{/g' \
  -e 's/component\.ValBool/types.ValBool/g' \
  -e 's/component\.ValS8/types.ValS8/g' \
  -e 's/component\.ValU8/types.ValU8/g' \
  -e 's/component\.ValS16/types.ValS16/g' \
  -e 's/component\.ValU16/types.ValU16/g' \
  -e 's/component\.ValS32/types.ValS32/g' \
  -e 's/component\.ValU32/types.ValU32/g' \
  -e 's/component\.ValS64/types.ValS64/g' \
  -e 's/component\.ValU64/types.ValU64/g' \
  -e 's/component\.ValF32/types.ValF32/g' \
  -e 's/component\.ValF64/types.ValF64/g' \
  -e 's/component\.ValChar/types.ValChar/g' \
  -e 's/component\.ValString/types.ValString/g' \
  -e 's/component\.ValRecord/types.ValRecord/g' \
  -e 's/component\.ValVariant/types.ValVariant/g' \
  -e 's/component\.ValOption/types.ValOption/g' \
  -e 's/component\.ValList/types.ValList/g' \
  -e 's/component\.ValTuple/types.ValTuple/g' \
  -e 's/component\.ValResultOk/types.ValResultOk/g' \
  -e 's/component\.ValResultError/types.ValResultError/g' \
  -e 's/component\.ValFlags/types.ValFlags/g' \
  -e 's/component\.ValEnum/types.ValEnum/g' \
  -e 's/component\.ValOwn/types.ValOwn/g' \
  -e 's/component\.ValBorrow/types.ValBorrow/g' \
  -e 's/component\.ValKindBool/types.ValKindBool/g' \
  -e 's/component\.ValKindS8/types.ValKindS8/g' \
  -e 's/component\.ValKindU8/types.ValKindU8/g' \
  -e 's/component\.ValKindS16/types.ValKindS16/g' \
  -e 's/component\.ValKindU16/types.ValKindU16/g' \
  -e 's/component\.ValKindS32/types.ValKindS32/g' \
  -e 's/component\.ValKindU32/types.ValKindU32/g' \
  -e 's/component\.ValKindS64/types.ValKindS64/g' \
  -e 's/component\.ValKindU64/types.ValKindU64/g' \
  -e 's/component\.ValKindF32/types.ValKindF32/g' \
  -e 's/component\.ValKindF64/types.ValKindF64/g' \
  -e 's/component\.ValKindChar/types.ValKindChar/g' \
  -e 's/component\.ValKindString/types.ValKindString/g' \
  -e 's/component\.ValKindList/types.ValKindList/g' \
  -e 's/component\.ValKindRecord/types.ValKindRecord/g' \
  -e 's/component\.ValKindTuple/types.ValKindTuple/g' \
  -e 's/component\.ValKindVariant/types.ValKindVariant/g' \
  -e 's/component\.ValKindOption/types.ValKindOption/g' \
  -e 's/component\.ValKindResult/types.ValKindResult/g' \
  -e 's/component\.ValKindFlags/types.ValKindFlags/g' \
  -e 's/component\.ValKindEnum/types.ValKindEnum/g' \
  -e 's/component\.ValKindOwn/types.ValKindOwn/g' \
  -e 's/component\.ValKindBorrow/types.ValKindBorrow/g' \
  -e 's/component\.Val\b/types.Val/g' \
  -e 's/component\.ValKind\b/types.ValKind/g' \
  internal/component/abi/lift.go
```

- [ ] **Step 3: Replace runtime identifiers**

```bash
sed -i \
  -e 's/component\.ResourceTable/runtime.ResourceTable/g' \
  -e 's/component\.NewResourceTable/runtime.NewResourceTable/g' \
  -e 's/component\.Handle\b/runtime.Handle/g' \
  -e 's/component\.MakeHandle/runtime.MakeHandle/g' \
  -e 's/component\.HandleEntry/runtime.HandleEntry/g' \
  -e 's/component\.CallContext/runtime.CallContext/g' \
  -e 's/component\.NewCallContext/runtime.NewCallContext/g' \
  -e 's/component\.BorrowScope/runtime.BorrowScope/g' \
  -e 's/component\.NewBorrowScope/runtime.NewBorrowScope/g' \
  -e 's/component\.Subtask\b/runtime.Subtask/g' \
  -e 's/component\.NewSubtask/runtime.NewSubtask/g' \
  -e 's/component\.SubtaskState/runtime.SubtaskState/g' \
  -e 's/component\.ResourceTypeID/runtime.ResourceTypeID/g' \
  -e 's/component\.NewResourceTypeID/runtime.NewResourceTypeID/g' \
  -e 's/component\.InvalidResourceTypeID/runtime.InvalidResourceTypeID/g' \
  -e 's/component\.ResourceTypeInfo/runtime.ResourceTypeInfo/g' \
  -e 's/component\.NewResourceTypeInfo/runtime.NewResourceTypeInfo/g' \
  -e 's/component\.InstanceState/runtime.InstanceState/g' \
  -e 's/component\.NewInstanceState/runtime.NewInstanceState/g' \
  -e 's/component\.DestructorRegistry/runtime.DestructorRegistry/g' \
  -e 's/component\.NewDestructorRegistry/runtime.NewDestructorRegistry/g' \
  -e 's/component\.DestructorFunc/runtime.DestructorFunc/g' \
  -e 's/component\.ReentranceTracker/runtime.ReentranceTracker/g' \
  -e 's/component\.NewReentranceTracker/runtime.NewReentranceTracker/g' \
  -e 's/component\.TrapHandler/runtime.TrapHandler/g' \
  -e 's/component\.CrossInstanceDestructor/runtime.CrossInstanceDestructor/g' \
  -e 's/component\.Destroyable/runtime.Destroyable/g' \
  -e 's/component\.MaxTableLength/runtime.MaxTableLength/g' \
  -e 's/component\.ErrInvalidHandle/runtime.ErrInvalidHandle/g' \
  -e 's/component\.ErrHandleNotOwned/runtime.ErrHandleNotOwned/g' \
  -e 's/component\.ErrResourceInUse/runtime.ErrResourceInUse/g' \
  -e 's/component\.ErrNoBorrowsToDecrement/runtime.ErrNoBorrowsToDecrement/g' \
  -e 's/component\.ErrResourceTypeMismatch/runtime.ErrResourceTypeMismatch/g' \
  -e 's/component\.ErrMayNotLeave/runtime.ErrMayNotLeave/g' \
  -e 's/component\.ErrReentrance/runtime.ErrReentrance/g' \
  -e 's/component\.ErrTableFull/runtime.ErrTableFull/g' \
  -e 's/component\.ErrOutstandingBorrows/runtime.ErrOutstandingBorrows/g' \
  internal/component/abi/lift.go
```

- [ ] **Step 4: Verify no `component.` references remain in this file**

Run: `grep -n '\bcomponent\.' internal/component/abi/lift.go`
Expected: no output.

### Task 17: Rewrite `abi/lower.go`

**Files:**
- Modify: `internal/component/abi/lower.go`

- [ ] **Step 1: Update imports**

Same import surgery as Task 16 step 1: replace `internal/component` with `internal/component/runtime`. Keep `internal/component/types` (already imported).

- [ ] **Step 2: Run the same Val sed replacement as Task 16 step 2**

Apply the entire `sed -i ... internal/component/abi/lower.go` block from Task 16 step 2 (substituting `lower.go` for `lift.go`).

- [ ] **Step 3: Run the same runtime sed replacement as Task 16 step 3**

Apply the entire `sed -i ... internal/component/abi/lower.go` block from Task 16 step 3.

- [ ] **Step 4: Verify**

Run: `grep -n '\bcomponent\.' internal/component/abi/lower.go`
Expected: no output.

### Task 18: Rewrite `abi/flatten.go`

**Files:**
- Modify: `internal/component/abi/flatten.go`

- [ ] **Step 1: Inspect current imports**

Run: `head -10 internal/component/abi/flatten.go`

`flatten.go` already imports only `api` and `internal/component/types` — it does not import `internal/component`. Confirm by:

Run: `grep -n 'internal/component"' internal/component/abi/flatten.go`
Expected: no output.

If there are no matches, this file is already clean — skip the rest of this task.

- [ ] **Step 2: If references exist, apply both sed replacements as in Tasks 16-17**

(Only run if step 1 found `component` references.)

### Task 19: Rewrite `abi/strings.go`

**Files:**
- Modify: `internal/component/abi/strings.go`

- [ ] **Step 1: Inspect current imports**

Run: `head -10 internal/component/abi/strings.go`

`strings.go` does not currently import `internal/component`. Confirm:

Run: `grep -n 'internal/component"' internal/component/abi/strings.go`
Expected: no output.

If clean, skip the rest.

- [ ] **Step 2: If references exist, apply sed replacements**

(Only if step 1 found references.)

### Task 20: Rewrite `abi/resource_lower.go`

**Files:**
- Modify: `internal/component/abi/resource_lower.go`

- [ ] **Step 1: Update imports**

Replace:
```go
import (
	"github.com/tetratelabs/wazero/internal/component"
)
```

With:
```go
import (
	"github.com/tetratelabs/wazero/internal/component/runtime"
)
```

- [ ] **Step 2: Apply both sed replacements**

Apply the Val sed (Task 16 step 2) and runtime sed (Task 16 step 3) to `internal/component/abi/resource_lower.go`.

- [ ] **Step 3: Verify**

Run: `grep -n '\bcomponent\.' internal/component/abi/resource_lower.go`
Expected: no output.

### Task 21: Rewrite `abi/context_test.go`

**Files:**
- Modify: `internal/component/abi/context_test.go`

- [ ] **Step 1: Update imports**

Replace:
```go
import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)
```

With:
```go
import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/testing/require"
)
```

- [ ] **Step 2: Apply both sed replacements**

Apply the Val sed (Task 16 step 2) and runtime sed (Task 16 step 3) to `internal/component/abi/context_test.go`.

- [ ] **Step 3: Verify**

Run: `grep -n '\bcomponent\.' internal/component/abi/context_test.go`
Expected: no output.

### Task 22: Rewrite `abi/lift_test.go`

**Files:**
- Modify: `internal/component/abi/lift_test.go`

- [ ] **Step 1: Update imports**

Replace:
```go
import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)
```

With:
```go
import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)
```

- [ ] **Step 2: Apply both sed replacements**

Apply the Val sed (Task 16 step 2) and runtime sed (Task 16 step 3) to `internal/component/abi/lift_test.go`.

- [ ] **Step 3: Verify**

Run: `grep -n '\bcomponent\.' internal/component/abi/lift_test.go`
Expected: no output.

### Task 23: Rewrite `abi/lower_test.go`

**Files:**
- Modify: `internal/component/abi/lower_test.go`

- [ ] **Step 1: Update imports**

Same surgery as Task 22: replace `internal/component` with `internal/component/runtime`.

- [ ] **Step 2: Apply both sed replacements**

Apply the Val sed (Task 16 step 2) and runtime sed (Task 16 step 3) to `internal/component/abi/lower_test.go`.

- [ ] **Step 3: Verify**

Run: `grep -n '\bcomponent\.' internal/component/abi/lower_test.go`
Expected: no output.

### Task 24: Rewrite `abi/resource_lower_test.go`

**Files:**
- Modify: `internal/component/abi/resource_lower_test.go`

- [ ] **Step 1: Update imports**

Same surgery: replace `internal/component` with `internal/component/runtime`.

- [ ] **Step 2: Apply both sed replacements**

Apply the Val sed and runtime sed to `internal/component/abi/resource_lower_test.go`.

- [ ] **Step 3: Verify**

Run: `grep -n '\bcomponent\.' internal/component/abi/resource_lower_test.go`
Expected: no output.

### Task 25: Rewrite `abi/flatten_test.go`

**Files:**
- Modify: `internal/component/abi/flatten_test.go`

- [ ] **Step 1: Inspect**

Run: `grep -n 'internal/component"' internal/component/abi/flatten_test.go`

If empty, this file is already clean (it only imports `internal/component/types`). Skip the rest of this task.

- [ ] **Step 2: If references exist, apply both sed replacements**

### Task 26: Rewrite `abi/strings_test.go`

**Files:**
- Modify: `internal/component/abi/strings_test.go`

- [ ] **Step 1: Inspect**

Run: `grep -n 'internal/component' internal/component/abi/strings_test.go`

If empty, this file is already clean. Skip the rest.

- [ ] **Step 2: If references exist, apply both sed replacements**

### Task 27: Verify abi/ is fully decoupled

**Files:** none

- [ ] **Step 1: The acceptance grep**

Run: `grep -rn '"github.com/tetratelabs/wazero/internal/component"' internal/component/abi/`
Expected: NO output. This is the goal of the entire restructure.

If there are any matches, fix them before proceeding.

- [ ] **Step 2: Verify abi/ has no `component.` qualified references**

Run: `grep -rn '\bcomponent\.' internal/component/abi/`
Expected: no output (the only places `component` could appear are in import paths, which we already grepped, or in identifiers — both should be gone).

- [ ] **Step 3: Build abi/ in isolation**

Run: `go build ./internal/component/abi/...`
Expected: exit 0. This will succeed even though the rest of the tree is broken because `abi/` only depends on `types/`, `runtime/`, and `api`.

- [ ] **Step 4: Run abi/ tests**

Run: `go test ./internal/component/abi/...`
Expected: all tests pass.

### Task 28: Commit Phase 3 (the goal commit)

- [ ] **Step 1: Stage and commit**

Run:
```bash
git add internal/component/abi/
git commit -m "$(cat <<'EOF'
refactor(component/abi): drop wrong-direction internal/component import

Rewrite all twelve abi/*.go files (production and tests) to import
internal/component/runtime and internal/component/types instead of
internal/component. Replace identifiers mechanically:

  component.Val*           → types.Val*
  component.ValKind*       → types.ValKind*
  component.ResourceTable  → runtime.ResourceTable
  component.Handle         → runtime.Handle
  component.CallContext    → runtime.CallContext
  component.BorrowScope    → runtime.BorrowScope
  component.Subtask        → runtime.Subtask
  component.ResourceTypeInfo → runtime.ResourceTypeInfo
  ... and so on for the full list of moved symbols.

The abi/ package now depends only on internal/component/runtime,
internal/component/types, and the public api package. The dependency
direction is correct: component/ → abi/ → runtime/ → types/.

Acceptance check:
  $ grep -rn '"github.com/tetratelabs/wazero/internal/component"' internal/component/abi/
  (no output)

Top-level internal/component/ and downstream callers still do not
compile after this commit; they are rewritten in subsequent commits.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 4 — Rewrite `internal/component/` (top-level package)

The top-level `internal/component/` package previously used the moved symbols
as in-package identifiers (no qualifier). After the move, it must import
`internal/component/types` and `internal/component/runtime` and use qualified
references everywhere.

### Task 29: Add helper script for top-level qualifier rewrite

**Files:**
- Create: `/tmp/wazero-rewrite-component.sh` (throwaway helper)

- [ ] **Step 1: Write the helper script**

Create `/tmp/wazero-rewrite-component.sh`:

```bash
#!/usr/bin/env bash
# Rewrite a single Go file inside internal/component/ to use qualified
# references for symbols moved to types/ and runtime/.
#
# Usage: /tmp/wazero-rewrite-component.sh <file>
#
# Idempotent: safe to run multiple times.

set -euo pipefail
f="$1"

# Replace bare in-package identifiers with qualified references.
# Use word boundaries and avoid double-qualifying things already qualified.
sed -i -E \
  -e 's/\b(types|runtime)\.(Val|ValKind|ValBool|ValS8|ValU8|ValS16|ValU16|ValS32|ValU32|ValS64|ValU64|ValF32|ValF64|ValChar|ValString|ValRecord|ValVariant|ValOption|ValList|ValTuple|ValResultOk|ValResultError|ValFlags|ValEnum|ValOwn|ValBorrow|ValKindBool|ValKindS8|ValKindU8|ValKindS16|ValKindU16|ValKindS32|ValKindU32|ValKindS64|ValKindU64|ValKindF32|ValKindF64|ValKindChar|ValKindString|ValKindList|ValKindRecord|ValKindTuple|ValKindVariant|ValKindOption|ValKindResult|ValKindFlags|ValKindEnum|ValKindOwn|ValKindBorrow)\b/__SAFE_\1_\2__/g' \
  "$f"

# Now rewrite bare names → types.X
for sym in Val ValKind ValBool ValS8 ValU8 ValS16 ValU16 ValS32 ValU32 ValS64 ValU64 ValF32 ValF64 ValChar ValString ValRecord ValVariant ValOption ValList ValTuple ValResultOk ValResultError ValFlags ValEnum ValOwn ValBorrow ValKindBool ValKindS8 ValKindU8 ValKindS16 ValKindU16 ValKindS32 ValKindU32 ValKindS64 ValKindU64 ValKindF32 ValKindF64 ValKindChar ValKindString ValKindList ValKindRecord ValKindTuple ValKindVariant ValKindOption ValKindResult ValKindFlags ValKindEnum ValKindOwn ValKindBorrow; do
  sed -i -E "s/\b${sym}\b/types.${sym}/g" "$f"
done

# Bare names → runtime.X
for sym in ResourceTable NewResourceTable Handle MakeHandle HandleEntry Destroyable MaxTableLength CallContext NewCallContext BorrowScope NewBorrowScope Subtask SubtaskState SubtaskStatePending SubtaskStateResolved SubtaskStateFinishing SubtaskStateDone NewSubtask ResourceTypeID NewResourceTypeID InvalidResourceTypeID ResourceTypeInfo NewResourceTypeInfo InstanceState NewInstanceState DestructorRegistry NewDestructorRegistry DestructorFunc ReentranceTracker NewReentranceTracker TrapHandler CrossInstanceDestructor ErrInvalidHandle ErrHandleNotOwned ErrResourceInUse ErrNoBorrowsToDecrement ErrResourceTypeMismatch ErrMayNotLeave ErrReentrance ErrTableFull ErrOutstandingBorrows; do
  sed -i -E "s/\b${sym}\b/runtime.${sym}/g" "$f"
done

# Restore the safe-tagged identifiers (which were already qualified before).
sed -i -E 's/__SAFE_(types|runtime)_([A-Za-z0-9]+)__/\1.\2/g' "$f"

# Restore over-qualified entries that are not actually moved symbols. For
# example, MaxTableLength happens to be a unique name; but if a struct or
# field shares a name with a moved symbol, this script will mis-rewrite it.
# Each file MUST be inspected after running, and any false positives
# reverted by hand.
```

Make it executable:
```bash
chmod +x /tmp/wazero-rewrite-component.sh
```

> **WARNING for the executor:** This script is a starting point for the
> bulk rewrite. Because it rewrites bare identifiers, it WILL produce false
> positives if a struct field, local variable, or other symbol happens to
> share a name with a moved symbol (e.g. a struct field named `Handle`).
> After running it on a file, ALWAYS:
>
> 1. Run `gofmt -w <file>` and `go vet ./internal/component/...`
> 2. Look for compile errors that mention `types.X` or `runtime.X` where
>    `X` was originally a struct field, parameter, or local var
> 3. Manually revert false positives
>
> An alternative is to do each file by hand using targeted Edit calls. For
> small files (`context.go`, `compiled.go`, `instantiate.go`, etc.) the
> manual approach is faster. For the giant files (`instance.go`,
> `component_linker.go`, `canon_lower.go`) the script saves time but
> requires careful review.

### Task 30: Rewrite `internal/component/context.go`

**Files:**
- Modify: `internal/component/context.go`

- [ ] **Step 1: Add the runtime import**

Original imports:
```go
import (
	"context"
	"io"
)
```

New imports:
```go
import (
	"context"
	"io"

	"github.com/tetratelabs/wazero/internal/component/runtime"
)
```

- [ ] **Step 2: Replace `*ResourceTable` with `*runtime.ResourceTable`**

Original `WithResourceTable` and `ResourceTableFromContext`:
```go
func WithResourceTable(ctx context.Context, table *ResourceTable) context.Context {
	return context.WithValue(ctx, resourceTableContextKey, table)
}

func ResourceTableFromContext(ctx context.Context) *ResourceTable {
	table, _ := ctx.Value(resourceTableContextKey).(*ResourceTable)
	return table
}
```

New:
```go
func WithResourceTable(ctx context.Context, table *runtime.ResourceTable) context.Context {
	return context.WithValue(ctx, resourceTableContextKey, table)
}

func ResourceTableFromContext(ctx context.Context) *runtime.ResourceTable {
	table, _ := ctx.Value(resourceTableContextKey).(*runtime.ResourceTable)
	return table
}
```

- [ ] **Step 3: Verify**

Run: `grep -nE '\bResourceTable\b' internal/component/context.go`
Expected: every match is `runtime.ResourceTable`.

### Task 31: Rewrite `internal/component/context_test.go`

**Files:**
- Modify: `internal/component/context_test.go`

- [ ] **Step 1: Inspect current contents**

Read: `internal/component/context_test.go`

- [ ] **Step 2: Add the runtime import and qualify**

Add `"github.com/tetratelabs/wazero/internal/component/runtime"` to the import block.

Replace any bare `ResourceTable`, `NewResourceTable`, etc. with their `runtime.X` qualified form.

- [ ] **Step 3: Verify**

Run: `grep -nE '\b(ResourceTable|NewResourceTable|Handle|CallContext)\b' internal/component/context_test.go`
Expected: every match is qualified.

### Task 32: Rewrite `internal/component/canon_lower.go`

**Files:**
- Modify: `internal/component/canon_lower.go`

- [ ] **Step 1: Read the current imports**

Read: `internal/component/canon_lower.go` (offset 1, limit 30)

- [ ] **Step 2: Add types and runtime imports**

Update the import block to include:
```go
"github.com/tetratelabs/wazero/internal/component/runtime"
"github.com/tetratelabs/wazero/internal/component/types"
```

- [ ] **Step 3: Run the helper script**

Run: `/tmp/wazero-rewrite-component.sh internal/component/canon_lower.go`

- [ ] **Step 4: Format and inspect**

Run: `gofmt -w internal/component/canon_lower.go`
Run: `go vet ./internal/component/`
Expected: errors that pinpoint over-qualified identifiers (e.g. a parameter named `Subtask` of type `runtime.Subtask` that became `runtime.runtime.Subtask`).

- [ ] **Step 5: Manually fix any false positives**

For each `vet` error, open the file at the indicated line and revert the over-qualification by hand. Use the `Edit` tool with enough context to make the change unique.

- [ ] **Step 6: Verify build of canon_lower.go alone**

Run: `go build ./internal/component/`
Expected: progress (other files in the package may still fail, but the file you just edited shouldn't be the cause).

### Task 33: Rewrite `internal/component/canon_lower_test.go`

**Files:**
- Modify: `internal/component/canon_lower_test.go`

- [ ] **Step 1: Add types and runtime imports**

- [ ] **Step 2: Run the helper script**

Run: `/tmp/wazero-rewrite-component.sh internal/component/canon_lower_test.go`

- [ ] **Step 3: Format and fix false positives**

Run: `gofmt -w internal/component/canon_lower_test.go`

Manually fix anything that mis-rewrote.

### Task 34: Rewrite `internal/component/compiled.go` and its test

**Files:**
- Modify: `internal/component/compiled.go`
- Modify: `internal/component/compiled_test.go`

- [ ] **Step 1: For each file, add types/runtime imports as needed**

Inspect imports first:
```bash
head -10 internal/component/compiled.go internal/component/compiled_test.go
```

- [ ] **Step 2: Run the helper script on each**

```bash
/tmp/wazero-rewrite-component.sh internal/component/compiled.go
/tmp/wazero-rewrite-component.sh internal/component/compiled_test.go
gofmt -w internal/component/compiled.go internal/component/compiled_test.go
```

- [ ] **Step 3: Manually fix false positives surfaced by `go vet ./internal/component/`**

### Task 35: Rewrite `internal/component/component.go`

**Files:**
- Modify: `internal/component/component.go`

- [ ] **Step 1: Add types/runtime imports**

- [ ] **Step 2: Run helper script and gofmt**

```bash
/tmp/wazero-rewrite-component.sh internal/component/component.go
gofmt -w internal/component/component.go
```

- [ ] **Step 3: Manually fix false positives**

This is a large file (882 lines). Take particular care: it defines `TypeDef`, `RecordTypeDef`, etc., which may have field names that collide with moved symbols. Inspect every change near struct definitions.

### Task 36: Rewrite `internal/component/component_linker.go`

**Files:**
- Modify: `internal/component/component_linker.go`

- [ ] **Step 1: Add imports**

- [ ] **Step 2: Run helper script + gofmt**

```bash
/tmp/wazero-rewrite-component.sh internal/component/component_linker.go
gofmt -w internal/component/component_linker.go
```

- [ ] **Step 3: Manually fix false positives**

This is the largest file (3809 lines). Budget extra time. Run `go vet ./internal/component/` repeatedly and fix in batches.

### Task 37: Rewrite `internal/component/instance.go`

**Files:**
- Modify: `internal/component/instance.go`

- [ ] **Step 1: Add imports**

- [ ] **Step 2: Run helper script + gofmt**

```bash
/tmp/wazero-rewrite-component.sh internal/component/instance.go
gofmt -w internal/component/instance.go
```

- [ ] **Step 3: Manually fix false positives**

Second-largest file (2908 lines). Take care.

### Task 38: Rewrite remaining top-level production files

**Files:**
- Modify: `internal/component/instantiate.go`
- Modify: `internal/component/linker.go`
- Modify: `internal/component/linker_api.go`
- Modify: `internal/component/nested_component.go`
- Modify: `internal/component/type_checker.go`
- Modify: `internal/component/type_resolver.go`
- Modify: `internal/component/index_space.go`
- Modify: `internal/component/import_name.go`
- Modify: `internal/component/outer_alias.go`
- Modify: `internal/component/semver.go`

- [ ] **Step 1: For each file, run the helper script and gofmt**

```bash
for f in internal/component/instantiate.go internal/component/linker.go internal/component/linker_api.go internal/component/nested_component.go internal/component/type_checker.go internal/component/type_resolver.go internal/component/index_space.go internal/component/import_name.go internal/component/outer_alias.go internal/component/semver.go; do
  /tmp/wazero-rewrite-component.sh "$f"
  gofmt -w "$f"
done
```

- [ ] **Step 2: For each file, add the missing import block entries**

After the script runs, files that gained `types.X` or `runtime.X` references need their import block updated. The script does NOT add imports; it only rewrites identifiers. Open each file, look at the new qualified references, and add to the imports:

```go
"github.com/tetratelabs/wazero/internal/component/runtime"
"github.com/tetratelabs/wazero/internal/component/types"
```

(Only add the ones actually used. `goimports` would do this automatically; if you have it installed, run `goimports -w internal/component/*.go`.)

- [ ] **Step 3: Build incrementally**

Run: `go build ./internal/component/`
Expected: any remaining errors are false positives from the script. Fix them.

### Task 39: Rewrite top-level test files

**Files:**
- Modify: `internal/component/component_test.go`
- Modify: `internal/component/component_linker_test.go`
- Modify: `internal/component/instance_test.go`
- Modify: `internal/component/instantiate_test.go`
- Modify: `internal/component/linker_test.go`
- Modify: `internal/component/linker_api_test.go`
- Modify: `internal/component/composite_test.go`
- Modify: `internal/component/edge_case_test.go`
- Modify: `internal/component/import_name_test.go`
- Modify: `internal/component/index_space_test.go`
- Modify: `internal/component/integration_public_api_test.go`
- Modify: `internal/component/integration_records_test.go`
- Modify: `internal/component/integration_test.go`
- Modify: `internal/component/nested_component_test.go`
- Modify: `internal/component/outer_alias_test.go`
- Modify: `internal/component/semver_test.go`
- Modify: `internal/component/start_function_test.go`
- Modify: `internal/component/type_checker_test.go`
- Modify: `internal/component/type_resolver_test.go`
- Modify: `internal/component/value_import_test.go`

- [ ] **Step 1: For each file, run the helper script and gofmt**

```bash
for f in internal/component/*_test.go; do
  /tmp/wazero-rewrite-component.sh "$f"
  gofmt -w "$f"
done
```

(This rewrites the top-level test files in one batch. The conformance/, wasip2test/, and binary/ subdirectories are handled in later tasks.)

- [ ] **Step 2: Add imports as needed**

For each file that now references `types.X` or `runtime.X`, add to its import block:
```go
"github.com/tetratelabs/wazero/internal/component/runtime"
"github.com/tetratelabs/wazero/internal/component/types"
```

If you have `goimports`, run it:
```bash
goimports -w internal/component/*_test.go
```

- [ ] **Step 3: Build the top-level package**

Run: `go build ./internal/component/`
Expected: success. If errors remain, fix them.

- [ ] **Step 4: Run the top-level tests**

Run: `go test ./internal/component/`
Expected: PASS for everything that passed in baseline.

### Task 40: Commit Phase 4

- [ ] **Step 1: Stage and commit**

Run:
```bash
git add internal/component/*.go
git commit -m "$(cat <<'EOF'
refactor(component): qualify Val and runtime references in top-level package

After moving Val into internal/component/types and runtime state into
internal/component/runtime, every top-level component/*.go file now
imports both packages and uses qualified references (types.Val,
runtime.ResourceTable, runtime.CallContext, runtime.Subtask, etc.)
in place of the previously bare identifiers.

context.go's WithResourceTable and ResourceTableFromContext signatures
now take *runtime.ResourceTable.

No behavior changes. The top-level component package compiles and its
tests pass. Downstream packages (conformance/, wasip2test/, binary/,
imports/wasip2/) and the public api/component/ alias file are rewritten
in subsequent commits.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 5 — Rewrite downstream test packages

### Task 41: Rewrite `internal/component/conformance/`

**Files:** ~30 files in `internal/component/conformance/`

- [ ] **Step 1: Identify files that need changes**

Run: `grep -lE 'component\.(Val|ValKind|ResourceTable|NewResourceTable|Handle|MakeHandle|HandleEntry|CallContext|NewCallContext|BorrowScope|NewBorrowScope|Subtask|NewSubtask|ResourceTypeID|NewResourceTypeID|ResourceTypeInfo|NewResourceTypeInfo|InstanceState|NewInstanceState|DestructorRegistry|NewDestructorRegistry|ReentranceTracker|NewReentranceTracker|TrapHandler|Destroyable|Err(InvalidHandle|HandleNotOwned|ResourceInUse|NoBorrowsToDecrement|ResourceTypeMismatch|MayNotLeave|Reentrance|TableFull|OutstandingBorrows))\b' internal/component/conformance/*.go`

- [ ] **Step 2: For each file in step 1, run sed replacements**

For each file `f`:

```bash
sed -i -E \
  -e 's/component\.(Val[A-Za-z0-9]*)/types.\1/g' \
  -e 's/component\.(ResourceTable|NewResourceTable|Handle|MakeHandle|HandleEntry|CallContext|NewCallContext|BorrowScope|NewBorrowScope|Subtask|SubtaskState|NewSubtask|ResourceTypeID|NewResourceTypeID|InvalidResourceTypeID|ResourceTypeInfo|NewResourceTypeInfo|InstanceState|NewInstanceState|DestructorRegistry|NewDestructorRegistry|DestructorFunc|ReentranceTracker|NewReentranceTracker|TrapHandler|CrossInstanceDestructor|Destroyable|MaxTableLength|ErrInvalidHandle|ErrHandleNotOwned|ErrResourceInUse|ErrNoBorrowsToDecrement|ErrResourceTypeMismatch|ErrMayNotLeave|ErrReentrance|ErrTableFull|ErrOutstandingBorrows)/runtime.\1/g' \
  "$f"
```

(Apply the above to each file individually so you can review them.)

- [ ] **Step 3: Add imports**

For each modified file, ensure the import block contains:
```go
"github.com/tetratelabs/wazero/internal/component/runtime"  // if runtime.X is referenced
"github.com/tetratelabs/wazero/internal/component/types"     // if types.X is referenced
```

If you have `goimports`:
```bash
goimports -w internal/component/conformance/*.go
```

- [ ] **Step 4: Build and test**

Run: `go build ./internal/component/conformance/...`
Run: `go test ./internal/component/conformance/...`
Expected: PASS (matching baseline).

### Task 42: Rewrite `internal/component/wasip2test/`

**Files:** ~12 files in `internal/component/wasip2test/`

- [ ] **Step 1: Identify files needing changes**

Same grep as Task 41 but against `internal/component/wasip2test/*.go`.

- [ ] **Step 2: Apply the same sed transformations**

Same sed pipeline as Task 41 step 2.

- [ ] **Step 3: Add imports**

Same as Task 41 step 3.

- [ ] **Step 4: Build and test**

Run: `go build ./internal/component/wasip2test/...`
Run: `go test ./internal/component/wasip2test/...`
Expected: PASS (matching baseline).

### Task 43: Verify `internal/component/binary/` requires no changes

**Files:** none

- [ ] **Step 1: Confirm no moved-symbol references**

Run: `grep -rnE 'component\.(Val|ValKind|ResourceTable|NewResourceTable|Handle|MakeHandle|HandleEntry|CallContext|NewCallContext|BorrowScope|NewBorrowScope|Subtask|NewSubtask|ResourceTypeID|NewResourceTypeID|ResourceTypeInfo|NewResourceTypeInfo|InstanceState|NewInstanceState|DestructorRegistry|NewDestructorRegistry|ReentranceTracker|NewReentranceTracker)\b' internal/component/binary/`
Expected: no output. Confirmed in spec exploration; this package only references `TypeDef`/`FuncType` symbols which stay in `component/`.

- [ ] **Step 2: Build and test the binary package**

Run: `go build ./internal/component/binary/...`
Run: `go test ./internal/component/binary/...`
Expected: PASS.

### Task 44: Commit Phase 5

- [ ] **Step 1: Stage and commit**

Run:
```bash
git add internal/component/conformance/ internal/component/wasip2test/
git commit -m "$(cat <<'EOF'
refactor(component): qualify Val and runtime references in test packages

Update internal/component/conformance/*.go and
internal/component/wasip2test/*.go to use the new package locations:
component.Val* → types.Val*, component.ResourceTable etc. → runtime.X.

internal/component/binary/ requires no changes — verified that it only
references TypeDef/FuncType symbols which remain in the top-level
component package.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 6 — Rewrite `imports/wasip2/`

### Task 45: Rewrite `imports/wasip2/` files

**Files:** all `.go` files under `imports/wasip2/` that reference moved symbols.

- [ ] **Step 1: Identify files**

Run: `grep -rlE 'component\.(Val|ValKind|ResourceTable|NewResourceTable|Handle|MakeHandle|CallContext|NewCallContext|BorrowScope|Subtask|ResourceTypeID|ResourceTypeInfo|InstanceState|DestructorRegistry|ReentranceTracker)\b' imports/wasip2`

Expected list (from baseline exploration):
- `imports/wasip2/cli/cli.go`, `cli_test.go`
- `imports/wasip2/clocks/monotonic.go`, `monotonic_test.go`, `wall.go`
- `imports/wasip2/filesystem/filesystem.go`, `filesystem_test.go`, `preopens.go`
- `imports/wasip2/http/http.go`, `http_test.go`, `incoming.go`, `outgoing.go`
- `imports/wasip2/io/error.go`, `poll.go`, `poll_test.go`, `streams.go`, `streams_test.go`
- `imports/wasip2/random/insecure.go`, `random.go`
- `imports/wasip2/sockets/network.go`, `sockets_test.go`, `tcp.go`, `tcp_test.go`, `types.go`, `udp.go`
- `imports/wasip2/wasip2_integration_test.go`

- [ ] **Step 2: Apply the same sed transformations**

For each file:

```bash
sed -i -E \
  -e 's/component\.(Val[A-Za-z0-9]*)/types.\1/g' \
  -e 's/component\.(ResourceTable|NewResourceTable|Handle|MakeHandle|HandleEntry|CallContext|NewCallContext|BorrowScope|NewBorrowScope|Subtask|SubtaskState|NewSubtask|ResourceTypeID|NewResourceTypeID|InvalidResourceTypeID|ResourceTypeInfo|NewResourceTypeInfo|InstanceState|NewInstanceState|DestructorRegistry|NewDestructorRegistry|DestructorFunc|ReentranceTracker|NewReentranceTracker|TrapHandler|CrossInstanceDestructor|Destroyable|MaxTableLength|ErrInvalidHandle|ErrHandleNotOwned|ErrResourceInUse|ErrNoBorrowsToDecrement|ErrResourceTypeMismatch|ErrMayNotLeave|ErrReentrance|ErrTableFull|ErrOutstandingBorrows)/runtime.\1/g' \
  "$f"
```

- [ ] **Step 3: Add imports**

For each modified file, ensure the import block contains the new packages used. If you have `goimports`:
```bash
goimports -w imports/wasip2/**/*.go
```

Otherwise add manually:
```go
"github.com/tetratelabs/wazero/internal/component/runtime"
"github.com/tetratelabs/wazero/internal/component/types"
```

- [ ] **Step 4: Verify the existing `internal/component` import is still needed**

Some files in `imports/wasip2/` may still legitimately need `internal/component` for `Component`, `HostFunc`, `FuncType`, `TypeDef`, etc. (which stay in the top-level package). Don't remove that import unless `grep -n 'component\.' <file>` shows zero remaining references.

- [ ] **Step 5: Build and test**

Run: `go build ./imports/wasip2/...`
Run: `go test ./imports/wasip2/...`
Expected: PASS.

### Task 46: Commit Phase 6

- [ ] **Step 1: Stage and commit**

Run:
```bash
git add imports/wasip2/
git commit -m "$(cat <<'EOF'
refactor(imports/wasip2): qualify Val and runtime references

Update imports/wasip2/**/*.go to reference the new locations of Val
(internal/component/types) and runtime state types
(internal/component/runtime). Files retain their dependency on
internal/component for the high-level orchestration types
(Component, HostFunc, FuncType, TypeDef) that stay in that package.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 7 — Update public API aliases

### Task 47: Rewrite `api/component/component.go`

**Files:**
- Modify: `api/component/component.go`

- [ ] **Step 1: Add new package imports**

Current:
```go
import (
	"context"

	internalcomponent "github.com/tetratelabs/wazero/internal/component"
)
```

New:
```go
import (
	"context"

	internalcomponent "github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
)
```

- [ ] **Step 2: Update Val type aliases**

Replace:
```go
type Val = internalcomponent.Val
type ValKind = internalcomponent.ValKind
```

With:
```go
type Val = types.Val
type ValKind = types.ValKind
```

- [ ] **Step 3: Update ValKind constant aliases**

Replace each `internalcomponent.ValKind*` with `types.ValKind*`. The block becomes:

```go
const (
	ValKindBool    = types.ValKindBool
	ValKindS8      = types.ValKindS8
	ValKindU8      = types.ValKindU8
	ValKindS16     = types.ValKindS16
	ValKindU16     = types.ValKindU16
	ValKindS32     = types.ValKindS32
	ValKindU32     = types.ValKindU32
	ValKindS64     = types.ValKindS64
	ValKindU64     = types.ValKindU64
	ValKindF32     = types.ValKindF32
	ValKindF64     = types.ValKindF64
	ValKindChar    = types.ValKindChar
	ValKindString  = types.ValKindString
	ValKindList    = types.ValKindList
	ValKindRecord  = types.ValKindRecord
	ValKindTuple   = types.ValKindTuple
	ValKindVariant = types.ValKindVariant
	ValKindEnum    = types.ValKindEnum
	ValKindOption  = types.ValKindOption
	ValKindResult  = types.ValKindResult
	ValKindFlags   = types.ValKindFlags
	ValKindOwn     = types.ValKindOwn
	ValKindBorrow  = types.ValKindBorrow
)
```

- [ ] **Step 4: Update Val constructor var aliases**

Replace each `internalcomponent.Val*` constructor reference with `types.Val*`:

```go
var (
	ValBool        = types.ValBool
	ValS8          = types.ValS8
	ValU8          = types.ValU8
	ValS16         = types.ValS16
	ValU16         = types.ValU16
	ValS32         = types.ValS32
	ValU32         = types.ValU32
	ValS64         = types.ValS64
	ValU64         = types.ValU64
	ValF32         = types.ValF32
	ValF64         = types.ValF64
	ValChar        = types.ValChar
	ValString      = types.ValString
	ValRecord      = types.ValRecord
	ValList        = types.ValList
	ValTuple       = types.ValTuple
	ValVariant     = types.ValVariant
	ValEnum        = types.ValEnum
	ValOption      = types.ValOption
	ValResultOk    = types.ValResultOk
	ValResultError = types.ValResultError
	ValFlags       = types.ValFlags
	ValOwn         = types.ValOwn
	ValBorrow     = types.ValBorrow
)
```

- [ ] **Step 5: Update ResourceTable aliases**

Replace:
```go
type ResourceTable = internalcomponent.ResourceTable
var NewResourceTable = internalcomponent.NewResourceTable
```

With:
```go
type ResourceTable = runtime.ResourceTable
var NewResourceTable = runtime.NewResourceTable
```

- [ ] **Step 6: Verify WithResourceTable signature still works**

The current `WithResourceTable`:
```go
func WithResourceTable(ctx context.Context, table *ResourceTable) context.Context {
	return internalcomponent.WithResourceTable(ctx, table)
}
```

After step 5, `ResourceTable` is `runtime.ResourceTable`, so the parameter type matches. Internally, `internalcomponent.WithResourceTable` (which we updated in Task 30) takes `*runtime.ResourceTable`. The signatures align — no further change needed.

- [ ] **Step 7: Verify HostFunc still routes through internalcomponent**

`HostFunc` is `internalcomponent.HostFunc` and stays that way because `HostFunc` is defined in `component.go` (top-level), not in any of the moved files.

- [ ] **Step 8: Build and test**

Run: `go build ./api/component/`
Run: `go test ./api/component/`
Expected: PASS.

### Task 48: Verify whole-tree build, test, and vet

**Files:** none

- [ ] **Step 1: Full build**

Run: `go build ./...`
Expected: exit 0, no errors anywhere in the tree.

- [ ] **Step 2: Full test (component subset + imports)**

Run: `go test ./internal/component/... ./imports/wasip2/... ./api/component/...`
Expected: same pass/fail set as the baseline captured in Task 0. Compare against `/tmp/wazero-baseline-test.log` if needed.

- [ ] **Step 3: Vet**

Run: `go vet ./...`
Expected: exit 0.

- [ ] **Step 4: The acceptance grep**

Run: `grep -rn '"github.com/tetratelabs/wazero/internal/component"' internal/component/abi/`
Expected: NO output.

- [ ] **Step 5: Whole-tree test (optional but recommended)**

Run: `go test ./...`
Expected: same pass/fail as baseline.

### Task 49: Commit Phase 7

- [ ] **Step 1: Stage and commit**

Run:
```bash
git add api/component/component.go
git commit -m "$(cat <<'EOF'
refactor(api/component): point public aliases at types and runtime packages

Update api/component/component.go so that:
  type Val      = types.Val
  type ValKind  = types.ValKind
  type ResourceTable = runtime.ResourceTable
  var  ValS32 = types.ValS32         (and all sibling Val constructors)
  var  ValKindS32 = types.ValKindS32 (and all sibling ValKind constants)
  var  NewResourceTable = runtime.NewResourceTable

The exported names in the api/component package are unchanged in name,
kind, and effective type signature. External users continue to use
component.Val, component.ResourceTable, component.WithResourceTable,
etc., with no source changes required on their side.

This completes the component runtime restructure. abi/ no longer imports
internal/component:

  $ grep -rn '"github.com/tetratelabs/wazero/internal/component"' internal/component/abi/
  (empty)

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

### Task 50: Final verification and cleanup

**Files:** none

- [ ] **Step 1: Re-run the acceptance grep**

Run: `grep -rn '"github.com/tetratelabs/wazero/internal/component"' internal/component/abi/`
Expected: empty.

- [ ] **Step 2: Re-run the build**

Run: `go build ./...`
Expected: exit 0.

- [ ] **Step 3: Re-run all tests**

Run: `go test ./...`
Expected: same pass/fail as baseline.

- [ ] **Step 4: Re-run vet**

Run: `go vet ./...`
Expected: exit 0.

- [ ] **Step 5: Confirm public API surface unchanged**

Run: `grep -E '^(type|var|func|const) [A-Z]' api/component/component.go | sort > /tmp/api-after.txt`

Compare against the same grep on the base branch (run via `git stash` + `git checkout main` if needed) — every public name must appear on both sides. Restore the working tree afterward.

- [ ] **Step 6: Delete the throwaway helper script**

Run: `rm /tmp/wazero-rewrite-component.sh`

- [ ] **Step 7: Confirm git status is clean**

Run: `git status`
Expected: only the new commits, working tree clean.

---

## Definition of done

All five conditions from the spec must hold:

1. `grep -rn '"github.com/tetratelabs/wazero/internal/component"' internal/component/abi/` returns zero matches.
2. `go build ./...` passes.
3. `go test ./...` passes (same set as baseline).
4. `go vet ./...` passes.
5. Public symbols in `api/component/component.go` are unchanged in name, kind, and effective type signature.
