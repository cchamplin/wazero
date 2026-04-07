# Implementation Subagent Prompt

> This is a template. The driver fills in `{ITEM_ID}`, `{ITEM_TITLE}`,
> `{ITEM_BODY}`, `{LOOP_NUMBER}`, and any prior reviewer blockers before
> dispatching.

You are implementing one item from the canonical ABI unification project.
Your scope is **only this item**. Do not touch other items. Do not refactor
unrelated code.

## Item

**Loop {LOOP_NUMBER}, item {ITEM_ID}: {ITEM_TITLE}**

{ITEM_BODY}

## Mandatory pre-work

Before writing any code, do all of the following IN ORDER:

1. **Read the spec authorities.** Use the Read tool to load every file
   listed in `docs/plans/projects/abi-unification/spec-authorities.md`
   that is relevant to this item. The item's `spec authorities` field
   tells you which specific files and line ranges to read.

2. **Read the cited spec lines.** Open `definitions.py` (and
   `CanonicalABI.md` and any wasmtime cross-references) at the lines the
   item cites. Read the surrounding 30 lines for context. If your
   training-data understanding contradicts the spec text, the spec wins.

3. **Read the existing wazero code you will be modifying.** Use the Read
   tool on every file in the item's `Files: Modify` list. Understand the
   current shape before changing it.

4. **Re-read this prompt.** Confirm you understand:
   - The exact files you will create/modify
   - The exact behavior change required
   - The definition of done
   - That you may NOT introduce mocks, stubs, helpers without consumers,
     `// TODO` comments, error suppression, or `t.Skip` calls

## Implementation discipline

**Red/green TDD where applicable.** For items that add new behavior:

1. Write the failing test first. Run it. Confirm it fails for the right
   reason (not for an unrelated compile error).
2. Write the minimal implementation that makes the test pass.
3. Run the test. Confirm it passes.
4. Run the broader test suite for the package. Confirm nothing else
   regressed.

**For deletion items**, the discipline is different:

1. Run `Grep` for the symbol you're deleting. List every reference,
   including in test files.
2. For each reference, decide: migrate to the replacement, or delete the
   referencing code if it only existed for the deleted symbol.
3. Delete the symbol and all references in one coherent commit.
4. Run the broader test suite. Confirm nothing regressed.

**For wiring items** (Loop 2 phase 2.B/2.C), the discipline is:

1. Read the new entry point (`abi.CanonLift` or `abi.CanonLower`) and
   confirm its signature.
2. Replace the old function body with a thin shim. Delete the old
   helpers it called (in the same commit if they have no other callers).
3. Run the integration tests for the affected path. Confirm they pass —
   or, if they assert wrong-spec behavior, rewrite them with a comment
   citing the spec line that says they were wrong.

## Constraints (REJECT-ON-VIOLATION)

These are hard constraints. If you write code that violates any of
them, the reviewer will block the change and you will redo the work.

- **No mocks, stubs, fakes, or noops.** If a test needs a component,
  build a real one or copy one from `debug-vendored/`. If a host import
  does not yet implement a method, it traps.
- **No helpers without consumers.** Every new function must be called
  by something other than its own tests.
- **No `// TODO`, `// FIXME`, `// fallback`, `// hack` comments.** If
  you would write one, the work is incomplete.
- **No `t.Skip` or `t.SkipNow` introduced.** Skips are only allowed for
  documented async deferrals (Loop 3 phase 3.A async-touching WAST
  files), and those must cite `loop-3-async-deferred.md`.
- **No error suppression.** No `if err != nil { return defaultValue }`
  in production code. Errors propagate or trap.
- **No new `internal/component` imports in `_test.go` files** outside
  the allow-list (`abi/`, `conformance/`, `spectest/`). Loop 3 phase 3.C
  removes existing violations; new tests must use only the public API.
- **No symlinks** anywhere in the wazero source tree. Copy files with
  `// SOURCE:` headers instead.
- **Spec compliance over local convention.** If wazero's existing code
  disagrees with the spec, the spec wins and the existing code is wrong.

## Output

When you finish (and only when the tests pass and you have nothing left
to do), produce a structured summary:

```markdown
## Implementation summary for Loop {LOOP_NUMBER} item {ITEM_ID}

**Files changed:**
- Created: <list>
- Modified: <list with brief change description per file>
- Deleted: <list>

**Spec citations:**
- definitions.py:<line> — <what it covers>
- CanonicalABI.md <section> — <what it covers>
- (wasmtime cross-reference if any)

**Tests added:**
- <test name> — <what it asserts>
- (or: "no new tests; this is a deletion item")

**Test results:**
- `go test ./<package>` — <PASS|FAIL with summary>
- `go test ./...` — (only if the item explicitly requires it; otherwise skip)

**Cross-cutting concerns the reviewers should look at:**
- <e.g. "I had to change a struct field name; check no callers were missed">
- (or: "none")

**Open questions:**
- <e.g. "spec is silent on whether X applies when Y; I picked X based on
   wasmtime's behavior at values.rs:230">
- (or: "none")
```

The driver will then dispatch the two reviewer subagents in parallel. If
either posts BLOCKER, you will receive their findings and must address
them before re-submitting.

## Stop conditions

You stop when:
- The change is one coherent commit-ready diff
- All required tests are written and passing
- The summary above is produced
- You have NOT yet committed the change (the driver commits after dual
  review passes)

You do NOT:
- Speculatively touch other items
- Refactor unrelated code that you happen to notice is wrong (file a
  follow-up note in the summary instead)
- Pick the next item from the tracking file (the driver does that)
