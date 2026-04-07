# Spec Authorities

**Every agent that touches this project — implementation, reviewer, or
verifier — MUST read the relevant files below BEFORE writing or evaluating
any code.**

Your training data is out of date. The files in `debug-vendored/` are the
source of truth.

## Universal rules

1. **Spec wins.** If your training data, prior commit history, or local code
   disagrees with the spec text, the spec text is correct and the code is
   wrong. Update the code. If a tracking-file item description disagrees
   with the spec, escalate to the user — do not silently follow the wrong
   instruction.

2. **No symlinks.** Anything you need from `debug-vendored/` is read into
   your context with the Read tool, or copied into the wazero source tree
   with a `// SOURCE:` provenance header recording the upstream path and
   the upstream git SHA at copy time. **No symlinks anywhere.**

3. **Cite the file:line you are implementing** in commit messages and in
   code comments where the implementation is non-obvious. Reviewers must
   also cite file:line — a finding without a citation is not a valid
   finding.

4. **If the spec is ambiguous on a question**, consult wasmtime AND check
   that the chosen interpretation is consistent across all wasmtime call
   sites. If still ambiguous, escalate to the user — do not invent.

5. **Do not infer behavior from training data.** If you find yourself
   thinking "I know how the canonical ABI works", stop and re-read
   `definitions.py` for that specific algorithm. The spec evolves; what
   you remember may be obsolete or wrong.

6. **No mocks. No stubs. No noops. No placeholder success values.** If a
   test needs a real component, build one or copy one from
   `debug-vendored/`. If a host import does not yet implement a method,
   it traps — it does not return success.

7. **No new helpers without consumers.** Every function you write must be
   called by something other than its own tests. If you find yourself
   creating a "utility" with no production caller, delete it.

8. **No `// TODO`. No `// FIXME`. No `// fallback`. No `// hack`.** If you
   would write one, the work is incomplete; do the work or escalate.

## Required reading by topic

### Synchronous canonical ABI

These define lift/lower semantics, alignment, size, flatten, retptr,
post-return, NaN canonicalization, string encoding, resource handles, and
type validation.

- `debug-vendored/component-model/design/mvp/CanonicalABI.md` — the
  human-readable spec text. Section headers map to algorithm names.
- `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py`
  — the executable Python reference implementation. The spec text in
  `CanonicalABI.md` is auto-extracted from this file by `diff.py`, so the
  Python is the actual source of truth.
- `debug-vendored/component-model/design/mvp/canonical-abi/run_tests.py`
  — the test suite that the Python reference passes. Loop 1 ports the
  ~272 data-driven cases to Go.

### Wasmtime cross-reference

When the spec is ambiguous, when an algorithm is described abstractly, or
when you need to confirm an implementation choice fits the canonical
pattern, consult wasmtime. Wasmtime's component runtime is the most mature
production implementation of the canonical ABI.

- `debug-vendored/wasmtime/crates/environ/src/component/types.rs` — the
  `ComponentTypes` struct, `InterfaceType` enum, `TypeRecord`, `TypeFunc`,
  `CanonicalAbiInfo` cache. This is the single type representation.
- `debug-vendored/wasmtime/crates/environ/src/component/types_builder.rs`
  — the builder that interns parser output into `ComponentTypes`.
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/values.rs`
  — `Val::lift` matches on `InterfaceType` directly. The dispatch model
  Loop 1.D adopts.
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/func/typed.rs`
  — typed Lift/Lower traits, `call_raw` retptr handling.
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/func/options.rs`
  — `LiftContext`/`LowerContext` carry `&ComponentTypes`.
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/matching.rs`
  — `InstanceType<'a>` view struct that joins compile-time types with
  per-instance resource types.
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/resources/ty.rs`
  — `ResourceType` enum.

### Go ecosystem precedent

For idiomatic Go patterns when the spec leaves implementation details to
the runtime.

- `debug-vendored/go-modules/wit/codec.go` — allocate-then-fill JSON
  decoder; the pattern Loop 1.A item 6 adopts for the binary parser.
- `debug-vendored/go-modules/wit/typedef.go` — the `*TypeDef` graph with
  pointer references.
- `debug-vendored/go-modules/wit/abi.go` — the `ABI` interface
  (`Size`/`Align`/`Flat`).
- `internal/wasm/module.go` — wazero's own core-wasm decoder; one type
  representation, populated during decode.

### WASI WIT schemas (for Loop 2 phase 2.E trap rule)

When fixing the silent-default sites, the agent must read the WIT method
definition to determine whether the spec-correct response is `result.err`
(if the return type has an error union) or a trap (if it does not).

- `debug-vendored/WASI/proposals/sockets/`
- `debug-vendored/WASI/proposals/http/`
- `debug-vendored/WASI/proposals/filesystem/`
- `debug-vendored/WASI/proposals/clocks/`
- `debug-vendored/WASI/proposals/random/`
- `debug-vendored/WASI/proposals/cli/`
- `debug-vendored/WASI/proposals/io/`

### Component-model spec WAST suite (for Loop 3 phase 3.A)

- `debug-vendored/component-model/test/` — 69 `.wast` files copied into
  `internal/component/spectest/testdata/upstream/` in Loop 3.

### wit-bindgen runtime suite (for Loop 3 phase 3.B)

- `debug-vendored/wit-bindgen/tests/runtime/` — 33 per-type test cases
  copied into `internal/component/wasip2test/upstream/wit-bindgen/<case>/`
  in Loop 3.

## How to cite

When you cite a spec line, use this format:

```
spec: definitions.py:1197 (load() dispatching to lift_own)
spec: CanonicalABI.md "Loading" section
wasmtime: values.rs:115 (InterfaceType::Own dispatch in Val::lift)
go-precedent: wit/codec.go:140 (allocate-then-fill TypeDef pattern)
```

Reviewers will reject any finding that does not include a citation.

## Escalation

If you encounter any of the following, **stop and escalate to the user**
rather than guessing:

- Spec text contradicts wasmtime
- Two spec sections contradict each other
- The tracking-file item description contradicts the spec
- An implementation choice has more than one defensible answer and the
  spec does not pick
- A test fixture in `debug-vendored/` appears corrupt or unrunnable

Do not invent. Do not pick "the closest one". Escalate.
