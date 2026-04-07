# Loop 3 — Real wasm via public API, migrate existing tests

> **Status:** blocked on Loop 2
>
> **Goal:** End-to-end validation through `github.com/tetratelabs/wazero` +
> `api` + `api/component` + `imports/wasip2`. No test in the validation
> suite imports `internal/component`. Real-world WIT-defined components
> from upstream sources (component-model spec WAST suite + wit-bindgen
> runtime tests) drive the canonical ABI through the public surface.
>
> **Total items:** 18 across 4 phases
>
> Phase 3.A and 3.B can be worked in parallel by different sessions.
> Phase 3.C must wait until Loop 2 is complete (because the migrated
> tests rely on the new wired public API path). Phase 3.D is the
> terminal sweep and runs last.

---

## Phase 3.A — Spec WAST suite (3 items)

### Item 1: Verify the existing spectest runner consumes upstream WAST format

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** -

**Files:**
- Read (no modification): `internal/component/spectest/spectest.go`,
  `internal/component/spectest/spectest_test.go`,
  `internal/component/spectest/resources_test.go`
- Read: `debug-vendored/component-model/test/wasm-tools/aliases.wast`
  (or any one upstream file as a probe)
- Create: `docs/plans/projects/abi-unification/loop-3-spectest-runner-decision.md`

**Spec authorities:**
- N/A — this is a verification item

**Description:**
The existing `internal/component/spectest/spectest.go` is a 133-line
WAST runner that uses `wasm-tools json-from-wast` to convert WAST to
JSON, then parses the JSON. Verify it works against an upstream file by:

1. Read the runner code in full. Note its API: how does a test invoke
   it? What does it produce?
2. Pick one upstream WAST file as a probe (e.g.,
   `debug-vendored/component-model/test/wasm-tools/aliases.wast`).
3. Run `wasm-tools json-from-wast` on the probe file from the command
   line to confirm it produces parseable output. (This is a
   verification, not a permanent change — the agent runs it as a one-
   off `Bash` command.)
4. Try invoking the existing runner against the probe file (write a
   throwaway test case in your scratch context — DO NOT commit it).
   Confirm it either works or identifies a specific gap.

Write `loop-3-spectest-runner-decision.md`:

```markdown
# Spectest Runner Compatibility Assessment

## Runner API
<paragraph describing how the runner is invoked>

## Probe file used
<file path>

## Result
- wasm-tools json-from-wast produced output: <yes|no>
- Existing runner consumed the file: <yes|no with details>

## Gaps identified (if any)
- <e.g. "the runner expects a 'meta' field that upstream files don't have">

## Recommended action for item 2
- <e.g. "no changes needed to runner; just copy files in item 2"
   OR
   "extend runner to handle X before copying files in item 2">
```

**Definition of done:**
- `loop-3-spectest-runner-decision.md` exists
- The decision document either confirms the runner works as-is or
  identifies a specific extension needed (in which case the extension
  becomes a sub-item before item 2 starts)

**Reviewer focus areas:**
- Spec compliance: N/A
- Code quality: confirm the assessment is honest — if the runner
  doesn't work, the document says so; the document does not gloss over
  gaps

---

### Item 2: Copy 69 spec WAST files into spectest/testdata/upstream/

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Depends on item 1

**Files:**
- Create: `internal/component/spectest/testdata/upstream/` directory
  tree mirroring `debug-vendored/component-model/test/`
- Create: 69 files inside (one per upstream `.wast` file)
- Create: `internal/component/spectest/testdata/upstream/SOURCE.md`
  recording the upstream git SHA at copy time

**Spec authorities:**
- The 69 source files themselves

**Description:**
Copy every `.wast` file from `debug-vendored/component-model/test/`
into `internal/component/spectest/testdata/upstream/`, preserving the
directory structure (`async/`, `names/`, `resources/`, `values/`,
`wasm-tools/`, `wasmtime/`).

**Each copied file gets a `;; SOURCE:` provenance comment at the top**,
formatted like this (WAST uses `;;` for line comments):

```
;; SOURCE: debug-vendored/component-model/test/wasm-tools/aliases.wast
;; UPSTREAM: github.com/WebAssembly/component-model
;; SHA: <git rev-parse HEAD of debug-vendored/component-model at copy time>
;; COPIED: 2026-04-XX (loop-3 item 2)
```

The original file content follows the comment. The provenance comment
is the only modification.

**No symlinks.** Verify by `Grep -l "symlink"` or by inspecting the
file types (`stat -c %F`).

Create `internal/component/spectest/testdata/upstream/SOURCE.md` with:

```markdown
# Upstream Component-Model WAST Test Suite

Copied from: `debug-vendored/component-model/test/`
Upstream: https://github.com/WebAssembly/component-model
Git SHA at copy time: <SHA>
Copy date: 2026-04-XX
File count: 69

Re-copy procedure:
1. Update debug-vendored/component-model to the desired SHA
2. Run: <a documented copy command>
3. Verify file count matches and re-run go test ./internal/component/spectest/...
```

The "documented copy command" can be a one-liner shell snippet (in
documentation form, NOT a shell script committed to the repo — the
project rule is "no shell scripts, agent driven development").

**Definition of done:**
- 69 files copied with directory structure preserved
- Each file has a `;; SOURCE:` provenance comment
- `SOURCE.md` exists with all fields filled
- No symlinks (verify with `find ... -type l` or equivalent)
- Reviewer subagent verifies a sample of 5 files match their source
  byte-for-byte (after stripping the provenance comment)

**Reviewer focus areas:**
- Spec compliance: N/A (no spec change)
- Code quality: confirm provenance headers are correctly formatted;
  confirm no symlinks; confirm file counts; confirm directory structure
  matches upstream

---

### Item 3: Wire spectest_test.go to discover and run every upstream file

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Depends on items 1 and 2. Async-touching files get documented `t.Skipf` per Loop 1's deferral.

**Files:**
- Modify: `internal/component/spectest/spectest_test.go` — add
  `TestUpstreamSpecWAST` function that walks
  `testdata/upstream/` and runs each file
- Create: `docs/plans/projects/abi-unification/loop-3-async-deferred.md`
  listing every upstream file that exercises async features

**Spec authorities:**
- `definitions.py` async sections (for identifying which features are
  async-touching)
- `debug-vendored/component-model/design/mvp/Async.md` (if it exists;
  otherwise async-relevant sections of `CanonicalABI.md`)

**Description:**
Add a single test function:

```go
func TestUpstreamSpecWAST(t *testing.T) {
    err := filepath.Walk("testdata/upstream", func(path string, info os.FileInfo, err error) error {
        if err != nil { return err }
        if info.IsDir() || !strings.HasSuffix(path, ".wast") { return nil }

        relPath, _ := filepath.Rel("testdata/upstream", path)
        t.Run(relPath, func(t *testing.T) {
            if isAsyncDeferred(relPath) {
                t.Skipf("async deferred: see loop-3-async-deferred.md")
            }
            runWastFile(t, path)
        })
        return nil
    })
    if err != nil { t.Fatal(err) }
}
```

(Exact API depends on what the existing `spectest.go` runner exposes;
verify with item 1's decision doc.)

`isAsyncDeferred` is a small function that returns true for any file in
`testdata/upstream/async/` AND any file the agent identifies as
async-touching after reading its content. The list lives in
`loop-3-async-deferred.md`:

```markdown
# Async-Deferred WAST Files

These files exercise async, stream, future, or thread features that are
out of scope for the canonical ABI unification project. They will be
addressed in the follow-up async project (docs/plans/abi-unification-async/).

| File | Reason |
|---|---|
| async/cancel-stream.wast | uses canon stream.* |
| async/drop-subtask.wast | uses canon task.* |
| ... |
```

The agent populates this list by reading each upstream file and
checking for canon names like `canon stream.read`, `canon task.return`,
`canon thread.yield`, `canon backpressure.set`, etc.

**Every other file must run.** If a file fails, that's either a real
canonical ABI bug (file a sub-item to fix it) or an upstream bug in
the test fixture (escalate to the user — do not silently skip).

**Definition of done:**
- `TestUpstreamSpecWAST` exists in `spectest_test.go` and discovers all
  copied files
- `loop-3-async-deferred.md` exists with one row per deferred file
- `go test ./internal/component/spectest/...` passes
- No `t.Skip` outside the documented async-deferred list (the runner's
  skip logic is centralised in `isAsyncDeferred`)

**Reviewer focus areas:**
- Spec compliance: confirm every async deferral has a citation to a
  spec section that documents the deferred feature (e.g., "uses canon
  stream.read per definitions.py:XXX")
- Code quality: confirm no per-file `t.Skip` calls outside the
  centralised `isAsyncDeferred` function; confirm the runner walks the
  full tree, not just a hardcoded subset

---

## Phase 3.B — wit-bindgen runtime suite (3 items)

### Item 4: Decide build pipeline for wit-bindgen test components

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Pre-built binaries are committed; build instructions are documented per case

**Files:**
- Read: `debug-vendored/wit-bindgen/tests/runtime/numbers/`,
  `debug-vendored/wit-bindgen/tests/runtime/records/`,
  `debug-vendored/wit-bindgen/tests/runtime/resources/` (3 representative
  case directories) to understand the layout
- Read: `debug-vendored/wit-bindgen/Cargo.toml` (root)
- Create: `docs/plans/projects/abi-unification/loop-3-wit-bindgen-decision.md`

**Spec authorities:**
- N/A — this is a tooling decision item

**Description:**
The 33 wit-bindgen runtime test cases each have:
- A `test.wit` defining the component interface
- Source files in multiple languages (`runner.rs`, `test.rs`,
  `runner.c`, `test.c`, `runner.go`, `test.go`, `runner.cpp`,
  `test.cpp`, etc.)
- Sometimes a `compose.wac` for composing multiple components

For wazero's purposes, we need the BUILT `.wasm` for each test (one
per case, the composed component). We do NOT need the source.

Decide:
1. **Which language's source do we build from?** Rust is most
   mature; pick Rust unless a case is Rust-incompatible. Document the
   choice in the decision doc.
2. **Where are the built `.wasm` files going to live in wazero?**
   Default: `internal/component/wasip2test/upstream/wit-bindgen/<case>/component.wasm`.
3. **How do we build them?** Document the exact `cargo` /
   `wasm-tools` / `wac` invocations.
4. **Do we commit the binaries?** Yes (per the design's
   single-source-of-truth principle for the wazero source tree).

Write `loop-3-wit-bindgen-decision.md`:

```markdown
# wit-bindgen Runtime Suite Build Decision

## Source language chosen
Rust (default). Exceptions: <list any cases that need a different language>

## Output location
internal/component/wasip2test/upstream/wit-bindgen/<case>/component.wasm

## Build commands (per case)
```bash
cd debug-vendored/wit-bindgen/tests/runtime/<case>
cargo build --target wasm32-unknown-unknown --release
wasm-tools component new \
  ../target/wasm32-unknown-unknown/release/<case>.wasm \
  -o component.wasm
```
(Adjust per case; document exceptions.)

## Cases that compose multiple components
<list cases that have compose.wac and how to compose them with `wac compose`>

## Cases that cannot be built (and why)
<list any cases that fail to build, with the failure mode>
```

**Definition of done:**
- `loop-3-wit-bindgen-decision.md` exists with all sections filled
- The agent has actually run the build commands on at least 3
  representative cases (numbers, records, resources) to verify they
  work, and documented the results
- Cases that cannot be built are escalated to the user — do not
  silently skip

**Reviewer focus areas:**
- Spec compliance: N/A
- Code quality: confirm the build instructions are exact (not
  paraphrased), reproducible, and reference the actual upstream layout

---

### Item 5: Copy 33 wit-bindgen runtime cases (test.wit + built component.wasm + BUILD.md per case)

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Depends on item 4

**Files:**
- Create: `internal/component/wasip2test/upstream/wit-bindgen/<case>/`
  for each of the 33 cases
- Create per case: `test.wit` (verbatim from upstream),
  `component.wasm` (built per item 4's decision), `BUILD.md` (recording
  build commands and upstream SHA)
- Create: `internal/component/wasip2test/upstream/wit-bindgen/SOURCE.md`

**Spec authorities:**
- N/A

**Description:**
For each of the 33 cases in `debug-vendored/wit-bindgen/tests/runtime/`:

1. Build the component per item 4's decision.
2. Create the case directory in
   `internal/component/wasip2test/upstream/wit-bindgen/<case>/`.
3. Copy `test.wit` verbatim. Add a `// SOURCE:` (or `; SOURCE:`,
   depending on whether WIT supports `//` comments) provenance comment.
4. Copy the built `component.wasm`.
5. Create `BUILD.md` with:

```markdown
# Build instructions for <case>

Source: debug-vendored/wit-bindgen/tests/runtime/<case>
Upstream: github.com/bytecodealliance/wit-bindgen
Git SHA: <SHA>

## To rebuild
\`\`\`
cd debug-vendored/wit-bindgen/tests/runtime/<case>
<exact commands from item 4>
cp <output>.wasm internal/component/wasip2test/upstream/wit-bindgen/<case>/component.wasm
\`\`\`
```

Create the top-level `SOURCE.md` listing all 33 cases and the upstream
SHA.

**No symlinks** anywhere. The `component.wasm` files are committed as
binary blobs (verify with `git ls-files --others --exclude-standard`
or equivalent before committing).

Cases that exercise async/stream/future features get a note in their
BUILD.md saying "exercises async features; will be skipped in
upstream_wit_bindgen_test.go per Loop 1's async deferral". The case is
still copied (so the test surface is documented) but is skipped in
item 6's test wiring.

**Definition of done:**
- 33 case directories exist with `test.wit`, `component.wasm`, `BUILD.md`
- Each `test.wit` has a provenance comment
- Each `component.wasm` is a real component (verify with
  `wasm-tools component wit <file>` or equivalent — should produce a
  valid WIT)
- `SOURCE.md` exists with all 33 cases listed
- No symlinks
- Async-touching cases are noted in their BUILD.md

**Reviewer focus areas:**
- Spec compliance: N/A
- Code quality: confirm a sample of 5 cases pass `wasm-tools` validation;
  confirm provenance headers; confirm no symlinks; confirm async
  cases are documented

---

### Item 6: Add upstream_wit_bindgen_test.go using only the public API

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Depends on item 5. This is the canonical example of a public-API-only test.

**Files:**
- Create: `internal/component/wasip2test/upstream_wit_bindgen_test.go`

**Spec authorities:**
- The WIT files of the 33 test cases (for understanding what each
  case exports/imports)

**Description:**
Add a single Go test file that walks
`internal/component/wasip2test/upstream/wit-bindgen/` and runs each
case as a subtest.

**The test file uses ONLY the public wazero API:**

```go
package wasip2test

import (
    "context"
    "os"
    "path/filepath"
    "strings"
    "testing"

    "github.com/tetratelabs/wazero"
    "github.com/tetratelabs/wazero/api"
    "github.com/tetratelabs/wazero/api/component"
    "github.com/tetratelabs/wazero/imports/wasip2"
)

func TestUpstreamWitBindgen(t *testing.T) {
    ctx := context.Background()
    rt := wazero.NewRuntime(ctx)
    defer rt.Close(ctx)

    cases, err := os.ReadDir("upstream/wit-bindgen")
    if err != nil { t.Fatal(err) }

    for _, c := range cases {
        if !c.IsDir() { continue }
        name := c.Name()

        t.Run(name, func(t *testing.T) {
            if isAsyncDeferred(name) {
                t.Skipf("async deferred: see loop-3-async-deferred.md")
            }

            wasmPath := filepath.Join("upstream/wit-bindgen", name, "component.wasm")
            wasm, err := os.ReadFile(wasmPath)
            if err != nil { t.Fatal(err) }

            cl := wazero.NewComponentLinker(rt)
            wasip2.MergeInto(cl)  // bring in WASI P2 host imports

            comp, err := rt.CompileComponent(ctx, wasm)
            if err != nil { t.Fatalf("compile: %v", err) }
            defer comp.Close(ctx)

            inst, err := rt.InstantiateComponent(ctx, comp, cl)
            if err != nil { t.Fatalf("instantiate: %v", err) }
            defer inst.Close(ctx)

            // For each case, invoke its expected entry point.
            // The case-specific dispatch lives in a small switch below.
            runCase(t, ctx, inst, name)
        })
    }
}

func runCase(t *testing.T, ctx context.Context, inst component.Instance, name string) {
    // For most cases, the entry point is "test" or "main" by convention.
    // For some (e.g. "many-arguments") it's case-specific.
    // The agent reads each test.wit to determine the expected entry point.
    switch name {
    case "numbers":
        f := inst.ExportedFunc("test:numbers/test.test-numbers")
        // ... etc
    case "records":
        f := inst.ExportedFunc("test:records/test.test-record")
        // ... etc
    // ... 33 cases
    }
}

func isAsyncDeferred(name string) bool {
    // Match against the list in loop-3-async-deferred.md
    return false  // populate per Item 3 + Item 5 deferral lists
}
```

(Exact API depends on what `component.Instance` exposes — read the
public API surface in `api/component/` to confirm before writing.)

**This file MUST NOT import `internal/component`.** The reviewer
verifies this with `Grep`.

**Definition of done:**
- `upstream_wit_bindgen_test.go` exists
- It imports only `github.com/tetratelabs/wazero`,
  `github.com/tetratelabs/wazero/api`,
  `github.com/tetratelabs/wazero/api/component`,
  `github.com/tetratelabs/wazero/imports/wasip2`, and the standard
  library
- All 33 cases are dispatched (or marked deferred via
  `isAsyncDeferred`)
- `go test -run TestUpstreamWitBindgen ./internal/component/wasip2test/...`
  passes for all non-deferred cases
- Reviewer verifies zero `internal/component` imports with Grep

**Reviewer focus areas:**
- Spec compliance: confirm each case's expected behavior matches its
  `test.wit` (the runner asserts the WIT-defined contract)
- Code quality: confirm zero `internal/component` imports; confirm
  the runner uses idiomatic public API; confirm no mocks, no
  shortcuts

---

## Phase 3.C — Migrate existing wasip2test files to public API (9 items)

> Each item in this phase modifies one existing test file. The pattern
> is the same for all 9: replace `internal/component` imports with
> public API imports, delete `testutil` imports, verify the test still
> passes after Loop 2's wiring is in place.

### Item 7: Migrate calculator_test.go to public API

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** -

**Files:**
- Modify: `internal/component/wasip2test/calculator_test.go`

**Spec authorities:**
- N/A — this is a test refactor, not a spec change

**Description:**
Read the current `calculator_test.go`. List every `internal/component`
import (and `internal/component/testutil` if present). For each, find
the public API equivalent in `github.com/tetratelabs/wazero`,
`github.com/tetratelabs/wazero/api`,
`github.com/tetratelabs/wazero/api/component`, or
`github.com/tetratelabs/wazero/imports/wasip2`. Replace.

The test's behavior must not change. It must still load the same
plugins and assert the same outputs. The only change is which API it
uses to set up the runtime.

Pattern (representative):

```go
// BEFORE:
import (
    "github.com/tetratelabs/wazero/internal/component"
    "github.com/tetratelabs/wazero/internal/component/testutil"
)

// ... test setup using internal types ...

// AFTER:
import (
    "github.com/tetratelabs/wazero"
    "github.com/tetratelabs/wazero/api/component"
    "github.com/tetratelabs/wazero/imports/wasip2"
)

// ... test setup using public API ...
```

**Definition of done:**
- `calculator_test.go` imports zero `internal/component` symbols
- `go test -run TestCalculatorPlugins ./internal/component/wasip2test/...`
  passes
- Reviewer verifies with Grep

**Reviewer focus areas:**
- Spec compliance: N/A
- Code quality: confirm zero `internal/component` imports; confirm the
  test's assertions are unchanged (the refactor must not weaken
  coverage)

---

### Item 8: Migrate composition_test.go to public API

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** -

**Files:**
- Modify: `internal/component/wasip2test/composition_test.go`

**Spec authorities:**
- N/A

**Description:**
Same pattern as item 7, applied to `composition_test.go`.

**Definition of done:**
- `composition_test.go` imports zero `internal/component` symbols
- `go test -run TestComposition ./internal/component/wasip2test/...`
  passes (or matching test name; verify with the actual file)

**Reviewer focus areas:**
Same as item 7.

---

### Item 9: Migrate converter_test.go to public API (and remove testutil import)

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Verify testutil dependency is removable

**Files:**
- Modify: `internal/component/wasip2test/converter_test.go`

**Spec authorities:**
- N/A

**Description:**
Same pattern as item 7. Additionally: this file imports
`internal/component/testutil`. Remove that import. If the test depends
on a function from `testutil`, either:
- Replace it with the public API equivalent, or
- Inline the helper into the test file (only if it has a single
  caller — items 7-15 are all single-caller migrations)

**Definition of done:**
- `converter_test.go` imports zero `internal/component` symbols
- Zero `internal/component/testutil` imports
- Test still passes

**Reviewer focus areas:**
Same as item 7, plus: confirm no helpers from testutil were
duplicated into multiple test files (if a helper is used by multiple
files, it should go into a shared helper at
`internal/component/wasip2test/helpers_test.go` — but only if it has
multiple consumers).

---

### Item 10: Migrate kv_store_test.go to public API (and remove testutil import)

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** -

**Files:**
- Modify: `internal/component/wasip2test/kv_store_test.go`

**Spec authorities:**
- N/A

**Description:**
Same pattern as item 9.

**Definition of done:**
- `kv_store_test.go` imports zero `internal/component` symbols
- Zero `testutil` imports
- Test still passes

**Reviewer focus areas:**
Same as item 9.

---

### Item 11: Migrate large_record_test.go to public API

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** -

**Files:**
- Modify: `internal/component/wasip2test/large_record_test.go`

**Spec authorities:**
- N/A

**Description:**
Same pattern as item 7.

**Definition of done:**
- `large_record_test.go` imports zero `internal/component` symbols
- Test still passes — this test exercises the retptr lifting path and
  is the canonical end-to-end test for items in Loop 1 phase 1.D
  (`canon_lift` retptr handling)

**Reviewer focus areas:**
Same as item 7.

---

### Item 12: Migrate linking_test.go to public API (and remove testutil import)

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** -

**Files:**
- Modify: `internal/component/wasip2test/linking_test.go`

**Spec authorities:**
- N/A

**Description:**
Same pattern as item 9.

**Definition of done:**
- `linking_test.go` imports zero `internal/component` symbols
- Zero `testutil` imports
- Test still passes

**Reviewer focus areas:**
Same as item 9.

---

### Item 13: Migrate nested_types_test.go to public API

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** -

**Files:**
- Modify: `internal/component/wasip2test/nested_types_test.go`

**Spec authorities:**
- N/A

**Description:**
Same pattern as item 7.

**Definition of done:**
- `nested_types_test.go` imports zero `internal/component` symbols
- Test still passes — this test exercises nested record/option/result
  shared types

**Reviewer focus areas:**
Same as item 7.

---

### Item 14: Migrate variant_types_test.go to public API

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** -

**Files:**
- Modify: `internal/component/wasip2test/variant_types_test.go`

**Spec authorities:**
- N/A

**Description:**
Same pattern as item 7.

**Definition of done:**
- `variant_types_test.go` imports zero `internal/component` symbols
- Test still passes — this test exercises variant/enum type resolution

**Reviewer focus areas:**
Same as item 7.

---

### Item 15: Migrate wasi_exercise_test.go to public API

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** -

**Files:**
- Modify: `internal/component/wasip2test/wasi_exercise_test.go`

**Spec authorities:**
- N/A

**Description:**
Same pattern as item 7. This is the most complex migration: the test
uses preopens, calls `test-fs-set-size`, `test-fs-metadata-hash`,
`test-fs-is-same-object` on both Rust and Go reactor components.
Verify all asserts still hold after Loop 2's wasip2 silent-default
fixes (items 8-11) — the test may have been depending on the silent
behavior.

**Definition of done:**
- `wasi_exercise_test.go` imports zero `internal/component` symbols
- Test still passes
- Any assertion that depended on Loop 2-fixed silent behavior is
  rewritten to expect the spec-correct behavior, with a comment
  citing the WIT method's spec-correct return type

**Reviewer focus areas:**
- Code quality: confirm the test is not silently weakened; confirm
  rewritten asserts cite WIT spec lines

---

## Phase 3.D — Test surface verification (3 items)

### Item 16: Public API audit subagent — confirm zero `internal/component` imports outside the allow-list

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** -

**Files:**
- Read (no modification): every `_test.go` file in the repo
- Create: `docs/plans/projects/abi-unification/loop-3-public-api-audit.md`

**Spec authorities:**
- N/A — verification item

**Description:**
Dispatch a subagent (or do it directly as the driver) that runs:

```
Grep -l "internal/component" --include="*_test.go" .
```

Expected: matches only in `internal/component/abi/`,
`internal/component/conformance/`, and `internal/component/spectest/`
(the allow-list). Any other match is a violation.

For each violation:
1. Determine if it's a Loop 3 phase 3.C migration that was missed
   (file a regression for that item; bounce it).
2. Determine if it's a new test added by Loop 3 that incorrectly
   imported internal types (file a regression; bounce that item).
3. Determine if it's a pre-existing test outside `wasip2test/` that
   was never on the migration list (escalate to the user — the
   migration list may need to grow).

Write `loop-3-public-api-audit.md`:

```markdown
# Public API Audit

## Allow-list (legitimate internal/component imports in test files)
- internal/component/abi/
- internal/component/conformance/
- internal/component/spectest/

## Audit results
- Files in allow-list: <count>
- Files outside allow-list: <count, expected 0>

## Violations (if any)
| File | Symbol imported | Source item | Action |
|---|---|---|---|
| ... |
```

**Definition of done:**
- Audit document exists
- Zero violations (or all violations are escalated and tracked)
- `Grep -l "internal/component" --include="*_test.go" .` returns only
  the allow-list

**Reviewer focus areas:**
- Spec compliance: N/A
- Code quality: confirm the audit was actually run (not inferred);
  confirm the document is honest

---

### Item 17: Coverage audit subagent — confirm every abi/ entry point is exercised by public-API tests

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** -

**Files:**
- Read: every test file under `internal/component/spectest/`,
  `internal/component/wasip2test/`,
  `internal/component/integration_public_api_test.go`,
  `internal/component/wasip2test/upstream/wit-bindgen/`
- Read: `internal/component/abi/lift.go`, `lower.go`, `flatten.go`,
  `strings.go`, `resource_lower.go`, `context.go` — to enumerate
  entry points
- Create: `docs/plans/projects/abi-unification/loop-3-coverage-matrix.md`

**Spec authorities:**
- The exported API of `internal/component/abi/`

**Description:**
Build a coverage matrix mapping each `abi/` entry point to at least
one public-API test that exercises it. The matrix is a Markdown table:

```markdown
| abi/ entry point | Type covered | Public API test that exercises it |
|---|---|---|
| LiftFlat (Bool) | bool | spectest/upstream/wasm-tools/types.wast TestUpstreamSpecWAST/types |
| LiftFlat (Record) | record | wasip2test/upstream/wit-bindgen/records TestUpstreamWitBindgen/records |
| LowerHeap (FixedSizeList) | fixed-length list | wasip2test/upstream/wit-bindgen/fixed-length-lists |
| CanonLift (retptr) | retptr param spill | wasip2test/large_record_test.go |
| CanonLift (post-return) | post-return | (need a test) |
| ... |
```

For every entry point:
- If a test exists, name it
- If no test exists, file a sub-item to add one

The coverage targets (each must be exercised):
- LiftFlat for every type (Bool, S8/U8, S16/U16, S32/U32, S64/U64,
  F32/F64, Char, String, List, FixedSizeList, Record, Tuple, Variant,
  Enum, Option, Result, Flags, Own, Borrow)
- LowerFlat for every type (same list)
- LiftHeap for every type
- LowerHeap for every type
- String encoding for UTF-8, UTF-16, Latin1+UTF16 (each)
- CanonLift retptr param spill (>16 flat params)
- CanonLift retptr result spill
- CanonLift post-return invocation
- CanonLift NaN canonicalization (lift)
- CanonLower NaN scrambling (store)
- Resource type mismatch trap
- Realloc failure trap
- Variant flat-join with mixed core types

**Definition of done:**
- `loop-3-coverage-matrix.md` exists
- Every coverage target is mapped to at least one test
- Any missing target has a follow-up item filed at the end of the
  Loop 3 backlog (or at the end of Loop 1 if the gap is in `abi/`
  itself, not in test coverage)

**Reviewer focus areas:**
- Spec compliance: confirm the coverage targets list matches the
  exported `abi/` API
- Code quality: confirm follow-up items are filed for any gaps;
  confirm the matrix does not silently omit a target

---

### Item 18: Final verifier — go test ./... is green, no skips outside async-deferred list

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** This is the final terminal item before Loop 3 closes

**Files:**
- Read: `loop-3-async-deferred.md`
- Run: `go test ./...` from repo root

**Spec authorities:**
- N/A

**Description:**
Run `go test ./...` from the repo root. Capture the output. Expected:
PASS, with all tests green except for the documented async-deferred
skips.

Run `Grep -rn "t\.Skip" --include="*_test.go" .` and confirm every
skip cites `loop-3-async-deferred.md` or otherwise points to the
deferred-async list. Any other skip is a violation.

Run a fresh `Grep` for `TestPublicAPIAddS32` and confirm it does not
contain `t.Skipf` (Loop 2 item 13 should have removed it; this is the
final cross-check).

**Definition of done:**
- `go test ./...` is green
- All `t.Skip` calls are accounted for in `loop-3-async-deferred.md`
- `TestPublicAPIAddS32` is not skipped

**Reviewer focus areas:**
- Spec compliance: N/A
- Code quality: confirm the test run was actually performed (capture
  the output in the item's `notes:` field); confirm the skip list
  matches the async-deferred list

---

## Loop 3 termination

When all 18 items are `status: done`, the driver runs
`templates/verify-loop-complete.md` with `{LOOP_NUMBER}=3`. The
verifier produces `loop-3-completion-report.md`. If `COMPLETE`, the
loop closes and the entire project is done. If `INCOMPLETE`, failing
checks become new items at the end of this backlog.
