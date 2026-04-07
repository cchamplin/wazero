# Loop 3 — Public API expansion, real wasm fixtures, test migration

> **Status:** blocked on Loop 2
>
> **Goal:** Expose the minimum public API surface needed to drive the
> canonical ABI through wazero's public types (Phase 3.0), then bring
> in real-world WIT-defined components from upstream sources (Phase 3.A
> spec WAST + Phase 3.B wit-bindgen runtime), then migrate the
> existing wasip2test files to use only the public API (Phase 3.C),
> then verify (Phase 3.D).
>
> **Total items:** 27 across 5 phases (was 18; Phase 3.0 adds 5 items
> and Phase 3.C drops kv_store from migration scope per the public
> API parity research with wasmtime)
>
> The 4 public API additions in Phase 3.0 are direct parallels of
> wasmtime types (`LinkerInstance`, `ResourceTableFromContext`,
> `func_new` dynamic, `ResourceTable.WithDestructor`) and only expose
> existing internal symbols; they are not invented. See the design
> document's wasmtime-parity research findings.
>
> Phase 3.0 must complete before Phase 3.C starts. Phases 3.A and 3.B
> can be worked in parallel after Phase 3.0 (they don't depend on
> public API). Phase 3.C depends on 3.0. Phase 3.D is the terminal
> sweep and runs last.

---

## Phase 3.0 — Public API expansion (5 items, mandatory before Phase 3.C)

> Per wasmtime parity research, four small public API additions plus
> one runner extension are needed before the existing tests can be
> migrated. Each addition has a direct wasmtime analogue and only
> exposes an internal symbol that already exists.

### Item 0.1: Expose `api/component.ResourceTableFromContext`

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Wraps existing internal/component.ResourceTableFromContext. Used by 6 of 9 migration tests.

**Files:**
- Read: `internal/component/call_context.go` (or wherever
  `ResourceTableFromContext` is defined; verify with Grep)
- Modify: `api/component/component.go` — add a thin alias or wrapper
- Read: `examples/component-wasip2/wasip2_test.go` for the canonical
  public-API style

**Spec authorities:**
- N/A — public API design choice
- Wasmtime parallel: `wasmtime::component::ResourceTable` accessor
  pattern via store

**Description:**
The internal symbol `internal/component.ResourceTableFromContext(ctx) *ResourceTable`
exists today and is used by every dynamic host function in the 9
migration tests. Expose it from the public `api/component` package
as a one-line wrapper:

```go
// In api/component/component.go:
// ResourceTableFromContext returns the resource table that the host
// function should use for resource handle lookups. The table is
// installed via WithResourceTable on a context passed to a host
// function.
func ResourceTableFromContext(ctx context.Context) *ResourceTable {
    return internal.ResourceTableFromContext(ctx)
}
```

(`internal` here means `internal/component`; the actual import
alias depends on the existing `api/component/component.go` style.)

**Definition of done:**
- `api/component.ResourceTableFromContext` exists
- A doc comment explains its use
- A test confirms `WithResourceTable` + `ResourceTableFromContext`
  round-trips through a host function call
- `go test ./api/component/...` passes

**Reviewer focus areas:**
- Code quality: confirm zero new behavior — pure re-export; confirm
  the doc comment is clear; confirm no `internal/` types leak through
  the signature

---

### Item 0.2: Add a public basic component sub-linker (parallel to wasmtime's `LinkerInstance`)

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Used by all 9 migration tests. Wraps existing internal/component.Linker (the basic one, NOT ComponentLinker).

**Files:**
- Read: `internal/component/linker.go` — the existing basic `Linker`
  type, its `DefineFunc` / `Build` / etc. methods
- Read: `internal/component/component_linker.go` — the existing
  `ComponentLinker.MergeFrom(*Linker)` method
- Read: `imports/wasip2/wasip2.go:71` — `MergeInto(linker api.ComponentLinker) error`
  (the type-assertion-based current pattern)
- Modify: `api/component.go` — add the new interface
- Modify: `runtime.go` — add a constructor method if needed
- Modify: `internal/component/component_linker.go` — add a public
  `Merge` method that takes the new interface

**Spec authorities:**
- N/A — public API design
- Wasmtime parallel: `wasmtime::component::LinkerInstance` (per-instance
  scoped linker that can register dynamic host functions and resources)
  + `Linker::merge_with` semantics

**Description:**
The 9 migration tests construct `internal/component.NewLinker()` (the
basic single-instance linker, NOT the `ComponentLinker`), register
host functions on it, then use `internal/component.ComponentLinker.MergeFrom(*Linker)`
to fold it into the main `ComponentLinker` for instantiation. This
pattern is also what `imports/wasip2.MergeInto` does internally
(via a runtime type-assertion on `*ComponentLinkerWrapper`).

Expose the same shape publicly:

```go
// In api/component.go:

// ComponentSubLinker is a basic, instance-scoped component linker.
// It registers dynamic host functions and resources for a single
// component instance. To use a sub-linker, build it (DefineFunc,
// DefineResource, etc.), then merge it into a ComponentLinker via
// ComponentLinker.Merge.
//
// Parallel to wasmtime::component::LinkerInstance.
type ComponentSubLinker interface {
    // DefineFunc registers a dynamic host function under the given
    // namespace and name. The function receives Vals and returns
    // Vals; type information is checked at call time.
    DefineFunc(ns, name string, fn HostFunc) error

    // DefineResource registers a resource type with a destructor.
    DefineResource(ns, name string, dtor ResourceDtor) error

    // SetRelaxedSemverMatching enables version-mismatch tolerance.
    SetRelaxedSemverMatching(enabled bool)

    internalapi.WazeroOnly
}

// In wazero.Runtime:
//   NewComponentSubLinker() api.ComponentSubLinker

// On api.ComponentLinker:
//   Merge(other ComponentSubLinker) error
```

The implementation is a thin wrapper around `internal/component.Linker`
(the basic linker) and `ComponentLinker.MergeFrom`. Internal types
already exist; this item just exposes them via the public interface.

**Definition of done:**
- `api.ComponentSubLinker` interface exists
- `wazero.Runtime.NewComponentSubLinker()` exists
- `api.ComponentLinker.Merge(api.ComponentSubLinker) error` exists
- Tests demonstrate constructing a sub-linker, registering a host
  function and a resource, then merging into a ComponentLinker
- `imports/wasip2.MergeInto` is updated to use the new public Merge
  (eliminating the runtime type-assertion)
- `go test ./api/component/...` and `go test ./imports/wasip2/...`
  pass

**Reviewer focus areas:**
- Spec compliance: confirm the API mirrors wasmtime's
  `LinkerInstance` for the operations the migration tests use
- Code quality: confirm interface uses `internalapi.WazeroOnly`;
  confirm no `internal/` types in signatures; confirm
  `imports/wasip2.MergeInto` no longer needs the type-assert hack

---

### Item 0.3: Add `ResourceTable.WithDestructor` (rename of internal `CreateResourceDropFunc`)

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Used by kv_store_test.go's TestResourceLifecycle_TableWithDestructor. Loop 2 item 7 deletes the silent variant; this item exposes the trap-emitting variant publicly.

**Files:**
- Read: `internal/component/resource_table.go` — find the
  trap-emitting destructor wiring (the one that survives Loop 2 item 7)
- Modify: `api/component/component.go` — add the public method

**Spec authorities:**
- `definitions.py:1641` — `lower_own(cx, rep, t)` (drop semantics)
- Wasmtime parallel: `ResourceTable::push` returns a `Resource<T>`;
  the destructor is wired via `LinkerInstance::resource(name, ty, dtor)`

**Description:**
After Loop 2 item 7, the silent `CreateResourceDropFunc` variant is
gone and only the trap-emitting variant remains. Expose it publicly
under a clearer name:

```go
// On api/component.ResourceTable (which aliases the internal type):
func (t *ResourceTable) WithDestructor(typeIdx uint32, dtor func(uint32) error) func(uint32) error
```

The function returns a destructor closure that can be passed to
`ComponentLinker.DefineResource` or to a sub-linker's
`DefineResource`. Calling the returned closure on a handle removes
it from the table and invokes `dtor`. If the handle is invalid or
the resource is borrowed, it returns an error (which becomes a trap
in wasm).

**Definition of done:**
- `ResourceTable.WithDestructor` exists publicly
- A test demonstrates the full flow: create table, push handle, get
  destructor, call it, observe destructor invoked
- `go test ./api/component/...` passes

**Reviewer focus areas:**
- Spec compliance: confirm trap-on-borrowed and trap-on-invalid
  handle behavior matches `definitions.py` `lower_own`
- Code quality: confirm the new method doesn't expose internal types

---

### Item 0.4: Confirm `api.ComponentInstanceBuilder.Func` accepts dynamic host functions; expose if needed

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Used by composition_test.go (handler interface forwarding). Internal: FuncNoType.

**Files:**
- Read: `api/component.go` — current
  `ComponentInstanceBuilder.Func(name, fn)` signature
- Read: `internal/component/linker.go` — `InstanceBuilder.FuncNoType`
- Modify: `api/component.go` — confirm `Func` already accepts a
  type-optional `HostFunc`, OR add `FuncDynamic(name, fn)` if
  needed

**Spec authorities:**
- N/A — API ergonomics
- Wasmtime parallel: `LinkerInstance::func_new(name, ty, fn)` for
  dynamic; `LinkerInstance::func_wrap(name, fn)` for typed

**Description:**
The composition test forwards an incoming request from one component
to a host function that doesn't have static type info available
(it's serving as a generic handler). Today this is done via internal
`InstanceBuilder.FuncNoType`. Verify the public
`api.ComponentInstanceBuilder.Func` already supports this case (the
existing signature accepts a `HostFunc` with `(ctx, []Val) ([]Val, error)`,
which is dynamic). If yes, this item is documentation only.

If the public `Func` requires static type info that the dynamic case
can't provide, add `FuncDynamic(name string, fn HostFunc) error` to
`api.ComponentInstanceBuilder`.

**Definition of done:**
- The public sub-linker / ComponentInstanceBuilder API has a way to
  register a dynamic host function with no static type
- A test confirms it (composition-style forwarding)
- `go test ./api/component/...` passes

**Reviewer focus areas:**
- Spec compliance: confirm the dynamic-call shape matches what the
  canonical ABI runtime can validate at call time
- Code quality: confirm minimum addition

---

### Item 0.5: Extend `internal/component/spectest/` runner to handle `assert_trap`, `assert_return`, `invoke`, `register`

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** The existing runner is parse-only — its execution helpers in resources_test.go t.Skipf assert_trap/assert_return/invoke/register. Without this extension, Phase 3.A becomes a no-op (most upstream WAST files use these directives).

**Files:**
- Read: `internal/component/spectest/spectest.go` (133 lines, the
  parse-only runner)
- Read: `internal/component/spectest/resources_test.go` — find the
  existing `runModuleTest` and `runAssertInvalidTest` private helpers
  AND the t.Skipf calls for unsupported directives
- Modify: `internal/component/spectest/spectest.go` — add execution
  helpers for `invoke`, `assert_return`, `assert_trap`, `register`
  directives. Use the public `wazero.Runtime` API (since the runner
  must drive a real component instance)
- Modify: `internal/component/spectest/spectest_test.go` — update
  the existing two-fixture test to exercise the new directives if
  the fixtures use them

**Spec authorities:**
- WAST format: `debug-vendored/component-model/test/` example files
- `wasm-tools` `json-from-wast` output format (already used by the
  parse-only runner)

**Description:**
The existing runner converts a WAST file to JSON via
`wasm-tools json-from-wast`, then parses each command. It already
handles:
- `module` — instantiate a component (via `runModuleTest`)
- `assert_invalid` — instantiate and confirm error (via `runAssertInvalidTest`)

It currently SKIPS:
- `invoke` — call an exported function
- `assert_return` — call and confirm result
- `assert_trap` — call and confirm trap
- `register` — name an instantiation for cross-module reference

Extend the runner to actually execute these. Each handler builds on
the existing module instantiation: `invoke` looks up the named export
and calls it; `assert_return` does the same and compares the result
to the expected `Value` array; `assert_trap` expects the call to
return an error matching the expected text; `register` stores the
named instance in a map for later cross-references.

The runner uses **only the public API** (`wazero.Runtime`,
`api.ComponentLinker`, `api.Component`, `api.ComponentFunc`) for
instantiation and calls. This is consistent with Loop 3's "no
internal/component imports in tests" goal — note that
`spectest_test.go` is currently in the L3-1 allow-list as a
legitimate exception.

**Definition of done:**
- The runner handles `invoke`, `assert_return`, `assert_trap`,
  `register` directives correctly
- The two existing fixtures (`resources.wast`, `simple.wast`) still
  pass
- A new test exercises each new directive against a small fixture
  (committed to `testdata/`)
- `go test ./internal/component/spectest/...` passes

**Reviewer focus areas:**
- Spec compliance: confirm the directive handlers match the WAST
  format spec; confirm `assert_trap` doesn't accept any error message
  but verifies the expected trap reason
- Code quality: confirm the runner uses public API only; confirm no
  silent skips remain in the new directive paths

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
- Read: `debug-vendored/component-model/test/wasm-tools/alias.wast`
  (verified file name; the original plan said `aliases.wast` which
  does not exist)
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
The 32 wit-bindgen runtime test cases each have:
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

### Item 5: Copy 32 wit-bindgen runtime cases (test.wit + built component.wasm + BUILD.md per case)

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Depends on item 4

**Files:**
- Create: `internal/component/wasip2test/upstream/wit-bindgen/<case>/`
  for each of the 32 cases
- Create per case: `test.wit` (verbatim from upstream),
  `component.wasm` (built per item 4's decision), `BUILD.md` (recording
  build commands and upstream SHA)
- Create: `internal/component/wasip2test/upstream/wit-bindgen/SOURCE.md`

**Spec authorities:**
- N/A

**Description:**
For each of the 32 cases in `debug-vendored/wit-bindgen/tests/runtime/`:

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

Create the top-level `SOURCE.md` listing all 32 cases and the upstream
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
- 32 case directories exist with `test.wit`, `component.wasm`, `BUILD.md`
- Each `test.wit` has a provenance comment
- Each `component.wasm` is a real component (verify with
  `wasm-tools component wit <file>` or equivalent — should produce a
  valid WIT)
- `SOURCE.md` exists with all 32 cases listed
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
- The WIT files of the 32 test cases (for understanding what each
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
    "testing"

    "github.com/tetratelabs/wazero"
    "github.com/tetratelabs/wazero/api"
    "github.com/tetratelabs/wazero/imports/wasip2"
)

func TestUpstreamWitBindgen(t *testing.T) {
    ctx := context.Background()
    rt := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig())
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

            // Verified API: rt.NewComponentLinker() takes NO args
            // (runtime.go:166)
            cl := rt.NewComponentLinker()
            cl.SetRelaxedSemverMatching(true)  // wasi-p2 uses 0.2.x

            // Verified API: wasip2.MergeInto returns error
            // (imports/wasip2/wasip2.go:71)
            if err := wasip2.MergeInto(cl); err != nil { t.Fatal(err) }

            comp, err := rt.CompileComponent(ctx, wasm)
            if err != nil { t.Fatalf("compile: %v", err) }
            defer comp.Close(ctx)

            // Verified API: linker.Instantiate, NOT
            // rt.InstantiateComponent (which is the no-imports
            // shortcut at runtime.go:182 with 2 args)
            inst, err := cl.Instantiate(ctx, comp)
            if err != nil { t.Fatalf("instantiate: %v", err) }
            defer inst.Close(ctx)

            runCase(t, ctx, inst, name)
        })
    }
}

// runCase parameter type is api.Component (api/component.go:72), NOT
// the nonexistent component.Instance.
func runCase(t *testing.T, ctx context.Context, inst api.Component, name string) {
    switch name {
    case "numbers":
        // Verified API: ExportedFunction (api/component.go:75), NOT
        // ExportedFunc (which is the internal struct type).
        f := inst.ExportedFunction("test:numbers/test.test-numbers")
        if f == nil { t.Fatal("export not found") }
        // Verified API: Call(ctx, ...any) ([]any, error)
        // (api/component.go:102). Tests type-assert results.
        results, err := f.Call(ctx, int32(1), float32(2.0))
        if err != nil { t.Fatal(err) }
        _ = results  // assert per case
    // ... 32 cases
    }
}

func isAsyncDeferred(name string) bool {
    // Match against the list in loop-3-async-deferred.md
    return false  // populate per Item 3 + Item 5 deferral lists
}
```

**Verified public API** (every symbol cross-checked against current
wazero source via the audit):
- `rt.NewComponentLinker()` — no args (runtime.go:166)
- `rt.CompileComponent(ctx, wasm)` — 2 args
- `cl.Instantiate(ctx, comp)` — preferred over rt.InstantiateComponent
- `wasip2.MergeInto(linker) error` — returns error
- `inst.ExportedFunction(name)` — NOT `ExportedFunc`
- `f.Call(ctx, ...any) ([]any, error)` — type-assert results
- `cl.SetRelaxedSemverMatching(bool)` — required for wasi-p2 versioning

**This file MUST NOT import `internal/component`.** The reviewer
verifies this with `Grep`.

**Definition of done:**
- `upstream_wit_bindgen_test.go` exists
- It imports only `github.com/tetratelabs/wazero`,
  `github.com/tetratelabs/wazero/api`,
  `github.com/tetratelabs/wazero/imports/wasip2`, and the standard
  library (and `github.com/tetratelabs/wazero/api/component` only if
  the test needs `Val` constructors directly — most cases can use
  Go primitives via `Call(ctx, ...any)`)
- All 32 cases are dispatched (or marked deferred via
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

## Phase 3.C — Migrate existing wasip2test files to public API (10 items)

> Each item in this phase modifies one existing test file. **All
> migration items depend on Phase 3.0** for the public API exposures
> they use. The migration pattern is:
>
> 1. Remove `internal/component` and `internal/component/testutil`
>    imports
> 2. Use `rt.NewComponentLinker()` (verified, no args) and
>    `rt.NewComponentSubLinker()` (Phase 3.0 item 0.2)
> 3. Use `cl.Instantiate(ctx, comp)` instead of
>    `rt.InstantiateComponent` or `*component.CompiledComponent` casts
> 4. Use `inst.ExportedFunction(name)` (NOT `ExportedFunc` which is
>    the internal struct)
> 5. Use `f.Call(ctx, ...any) ([]any, error)` and type-assert results
> 6. Use `apicomponent.ResourceTableFromContext(ctx)` (Phase 3.0
>    item 0.1) inside dynamic host functions
> 7. Use `apicomponent.ResourceTable.WithDestructor(...)` (Phase 3.0
>    item 0.3) instead of internal `CreateResourceDropFunc`
> 8. For files using `testutil.BuildComponentFromWAT`: read pre-built
>    fixtures committed in item 10.5
> 9. Use `wasip2.MergeInto(cl)` and check the returned error
> 10. Call `cl.SetRelaxedSemverMatching(true)` before merging WASI P2
>
> Items in this phase can be worked in parallel by different sessions
> after Phase 3.0 is complete.

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
- **notes:** Heaviest migration target (~750 lines, most internal symbols). Depends on Phase 3.0 items 0.1, 0.2, 0.4 (basic sub-linker, ResourceTableFromContext, FuncDynamic). Test names start with TestServiceMiddlewareComposition_*, NOT TestComposition.

**Files:**
- Modify: `internal/component/wasip2test/composition_test.go`

**Spec authorities:**
- N/A — test refactor

**Description:**
Apply the migration pattern from Phase 3.C preamble. This file is the
heaviest internal-API user (~750 lines). Specific challenges:
- Forwards an `ExportedFunc` from one component to another via a
  handler interface — use the new sub-linker's `FuncDynamic` (Phase
  3.0 item 0.4) to register the forwarding host function
- Creates `Handle` types for resource references — use
  `apicomponent.ResourceTableFromContext` to access the table inside
  the host function
- Defines two parallel basic linkers and merges them — use the new
  `rt.NewComponentSubLinker()` and `cl.Merge(sub)` (Phase 3.0 item 0.2)
- Test names: `TestServiceMiddlewareComposition_LoadComponents`,
  `TestServiceMiddlewareComposition_CompileComponents`,
  `TestServiceMiddlewareComposition_InstantiateService`,
  `TestServiceMiddlewareComposition_FullComposition`,
  `TestServiceMiddlewareComposition_ErrorHandling` (5 tests, verified
  by audit)

**Definition of done:**
- `composition_test.go` imports zero `internal/component` symbols
- `go test -run TestServiceMiddlewareComposition ./internal/component/wasip2test/...`
  passes for all 5 tests

**Reviewer focus areas:**
- Spec compliance: confirm the migrated handler-forwarding pattern
  uses the public sub-linker correctly
- Code quality: confirm zero internal imports; confirm the test's
  assertions are unchanged

---

### Item 9: Migrate converter_test.go to public API (and remove testutil import)

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Depends on item 10.5 (pre-built WAT fixtures) and Phase 3.0. Audit found 5 BuildComponentFromWAT call sites in this file.

**Files:**
- Modify: `internal/component/wasip2test/converter_test.go`

**Spec authorities:**
- N/A — test refactor

**Description:**
Migration steps:
1. Replace `internal/component` imports with public API:
   `wazero`, `api`, `api/component`, `imports/wasip2`
2. Replace `component.NewLinker()` → `rt.NewComponentSubLinker()`
   (Phase 3.0 item 0.2)
3. Replace `component.NewComponentLinker(rt)` → `rt.NewComponentLinker()`
4. Replace 5 `testutil.BuildComponentFromWAT(wat)` calls with
   `os.ReadFile("upstream/wat-fixtures/<name>/component.wasm")` —
   the fixtures are pre-built in item 10.5
5. Replace `*component.CompiledComponent` cast: use the public
   `cl.Instantiate(ctx, comp)` flow
6. Use `apicomponent.ValS32(...)` (already public in
   `api/component/component.go`)
7. Remove `internal/component/testutil` import entirely

**Definition of done:**
- `converter_test.go` imports zero `internal/component` or
  `internal/component/testutil` symbols
- All 5 BuildComponentFromWAT call sites read pre-built fixtures
  via `os.ReadFile`
- Test still passes

**Reviewer focus areas:**
- Code quality: confirm zero `internal/component` imports; confirm
  the test's assertions are unchanged; confirm fixtures are
  pre-built (item 10.5), not built at test time

---

### Item 10: kv_store_test.go — partial migration; TestResourceLifecycle_LinkerDefinition stays internal

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Per wasmtime parity research: TestResourceLifecycle_LinkerDefinition is white-box wiring verification (asserts on *InstanceDef.Exports["store"].(*ResourceDef).Destructor(42)). It tests linker internals, not the API. It STAYS internal. The other tests in kv_store_test.go migrate.

**Files:**
- Modify: `internal/component/wasip2test/kv_store_test.go`

**Spec authorities:**
- N/A — test refactor

**Description:**
`kv_store_test.go` contains multiple tests. Per the wasmtime parity
research, **`TestResourceLifecycle_LinkerDefinition` stays internal**
because it asserts on internal linker structures (`*InstanceDef`,
`*ResourceDef.Destructor`) — this is white-box wiring verification,
not API usage. Wasmtime doesn't expose linker definition introspection
publicly either.

For all OTHER tests in `kv_store_test.go`:
- Migrate from `component.NewLinker` → `rt.NewComponentSubLinker()`
  (Phase 3.0 item 0.2)
- Migrate from `component.NewComponentLinker(rt)` → `rt.NewComponentLinker()`
- Migrate from `component.ResourceTableFromContext` →
  `apicomponent.ResourceTableFromContext` (Phase 3.0 item 0.1)
- Migrate from `ResourceTable.CreateResourceDropFunc` →
  `ResourceTable.WithDestructor` (Phase 3.0 item 0.3)
- Replace `*component.CompiledComponent` casts: use `cl.Instantiate(ctx, comp)`
  on the public interface
- For tests using `testutil.BuildComponentFromWAT`: pre-build the WAT
  fixtures and commit them (see new item 10.5 below)

The file ends up with `TestResourceLifecycle_LinkerDefinition` (and
potentially other white-box tests it contains) in the internal-allow-list,
plus the rest using only public API. The audit allow-list (item 16)
documents the named exception.

**Definition of done:**
- All non-white-box tests in `kv_store_test.go` use only public API
- `TestResourceLifecycle_LinkerDefinition` and any other white-box
  tests are documented in the L3-1 allow-list as named exceptions
- The file may have a `// internal: TestResourceLifecycle_LinkerDefinition`
  marker comment if needed
- Test still passes

**Reviewer focus areas:**
- Code quality: confirm only the documented white-box tests retain
  `internal/component` imports; confirm migrated tests use the new
  public API additions from Phase 3.0; confirm the named exception
  is documented

---

### Item 10.5: Pre-build WAT fixtures used by converter, kv_store, linking tests

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Per user decision: pre-build all WAT to .wasm and commit binaries. testutil.BuildComponentFromWAT (which shells out to wasm-tools at runtime) cannot be used by public-API tests because it's an internal helper.

**Files:**
- Read: `internal/component/wasip2test/converter_test.go`,
  `kv_store_test.go`, `linking_test.go` — find every
  `testutil.BuildComponentFromWAT(...)` call site (the audit found 5
  in converter, multiple in kv_store, 8 in linking)
- Read: `internal/component/testutil/builder.go` — understand the
  current build pipeline
- Create: `internal/component/wasip2test/upstream/wat-fixtures/<name>/component.wasm`
  — one directory per WAT fixture, with the built `.wasm` and a
  `BUILD.md` recording the original WAT source (verbatim) and the
  build command used

**Spec authorities:**
- N/A — fixture pre-build

**Description:**
The migration target tests use `testutil.BuildComponentFromWAT(wat)`
which calls `wasm-tools parse` via `os/exec` at test time. This is
not viable for public-API tests because `testutil` is in
`internal/component/testutil`.

Pre-build all WAT fixtures used by these tests:

1. For each `testutil.BuildComponentFromWAT(wat)` call site, extract
   the WAT source string from the test file.
2. Save the WAT source to
   `internal/component/wasip2test/upstream/wat-fixtures/<descriptive-name>/source.wat`.
3. Run `wasm-tools parse <source.wat> -o component.wasm` to produce
   the binary.
4. Commit both files and a `BUILD.md` documenting the source and
   build command.
5. Items 9, 10, 12 (converter, kv_store, linking migrations) read
   the pre-built `.wasm` via `os.ReadFile` instead of calling
   `BuildComponentFromWAT`.

**Definition of done:**
- All WAT fixtures used by the 3 migration target tests are
  pre-built and committed
- Each fixture has source.wat, component.wasm, BUILD.md
- Migration items 9, 10, 12 reference the new fixture paths
- `wasm-tools` is documented as the build prerequisite (one-time
  rebuild only, not at test time)

**Reviewer focus areas:**
- Code quality: confirm every BuildComponentFromWAT call site has a
  matching pre-built fixture; confirm BUILD.md is honest about the
  build command; confirm no symlinks

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

### Item 16: Public API audit subagent — confirm zero `internal/component` imports in migration-target files outside the allow-list

- **status:** pending
- **claimed_by:** -
- **spec_review:** -
- **code_review:** -
- **commit:** -
- **notes:** Allow-list is broader than the original plan. Many existing test packages legitimately import internal/component because they ARE the internal tests. The audit only verifies migration targets stay clean.

**Files:**
- Read (no modification): every `_test.go` file in the migration
  scope (see allow-list below)
- Create: `docs/plans/projects/abi-unification/loop-3-public-api-audit.md`

**Spec authorities:**
- N/A — verification item

**Description:**
Dispatch a subagent that walks `**/*_test.go` files and verifies each
file's imports. The check must use **content-based grep on imports**,
NOT path-based grep (a file's path containing "internal/component" is
not a violation; only its imports matter).

**Allow-list (legitimately import internal/component):**

These packages own internal types, so their tests must import them:
1. `internal/component/*_test.go` — the internal package's own tests
2. `internal/component/abi/*_test.go` — abi package tests
3. `internal/component/binary/*_test.go` — binary parser tests
4. `internal/component/conformance/*_test.go` — conformance tests
5. `internal/component/spectest/*_test.go` — spec runner tests
6. `internal/component/types/*_test.go`, `runtime/*_test.go` — type
   system tests (post-Loop-1-item-9.7)

**Migration target list (must NOT import internal/component):**

These files are in scope for the Loop 3 phase 3.C migration:
- `internal/component/wasip2test/calculator_test.go`
- `internal/component/wasip2test/composition_test.go`
- `internal/component/wasip2test/converter_test.go`
- `internal/component/wasip2test/large_record_test.go`
- `internal/component/wasip2test/linking_test.go`
- `internal/component/wasip2test/nested_types_test.go`
- `internal/component/wasip2test/variant_types_test.go`
- `internal/component/wasip2test/wasi_exercise_test.go`
- `internal/component/wasip2test/upstream_wit_bindgen_test.go` (new
  per item 6)
- Any new test files added under `internal/component/wasip2test/`
  during this loop

**Named exceptions** (in the migration target directory but with
documented reason to keep internal access):
- `internal/component/wasip2test/kv_store_test.go` — its
  `TestResourceLifecycle_LinkerDefinition` is a white-box wiring
  test that asserts on `*InstanceDef.Exports["store"].(*ResourceDef).Destructor(42)`.
  This is testing linker internals, not exercising the API. Keep
  internal-only. (This test was the audit-identified white-box
  exception.)
- `internal/component/wasip2test/bench_test.go` — uses public API
  only per the audit; verify and skip
- `internal/component/wasip2test/repro_test.go` — uses public API
  only per the audit; verify and skip
- `internal/component/wasip2test/loader_test.go` — package-internal
  loader test; uses no `internal/component` types directly per the
  audit; verify and skip
- `internal/component/integration_public_api_test.go` — currently
  imports `internal/component` for `HostFunc`/`Val`. After Loop 2
  item 13 unskips `TestPublicAPIAddS32`, this file should be
  reviewed: either also migrate it (sub-item) or document as
  internal-test exception (since it's in `internal/component/`
  itself, the allow-list rule covers it).

For each violation in a migration-target file:
1. Determine if it's a Loop 3 phase 3.C migration that was incomplete
   (file a regression; bounce that item).
2. Determine if a new symbol leaked through that should be exposed
   publicly (escalate; may need a new Phase 3.0 item).
3. Determine if the import is unavoidable for legitimate test reasons
   (add to the named-exceptions list above with documentation).

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
contain any "not wired yet" `t.Skipf` (Loop 2 item 13 should have
removed those at lines 89, 110, 121). The line-70 `t.Skipf("test
component not available: %v", err)` is a legitimate fixture-existence
guard and may remain unless `add_s32.wasm` is committed.

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
