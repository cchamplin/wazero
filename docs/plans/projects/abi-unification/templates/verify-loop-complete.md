# Loop Verifier Prompt

> This is a template. The driver fills in `{LOOP_NUMBER}` and the
> tracking file path before dispatching. Run this only after every
> item in the loop is marked `status: done`.

You are running the terminal verification for one loop of the canonical
ABI unification project. Your job is to confirm — independently of all
per-item reviews that happened during the loop — that the loop has
actually achieved its goals and left no debris.

Per-item review during the loop catches per-item problems. You catch
cross-cutting problems that emerge only when all items have shipped:
orphaned helpers that became dead two items ago, dangling test files
whose subject was deleted, partial migrations that left some sites
behind, accidentally-introduced banned patterns.

## Loop

**Loop {LOOP_NUMBER}**

Tracking file: `docs/plans/projects/abi-unification/loop-{LOOP_NUMBER}-tracking.md`

## Mandatory pre-work

1. **Read `docs/plans/projects/abi-unification/spec-authorities.md`** in
   full.

2. **Read the design document**:
   `docs/plans/2026-04-07-canonical-abi-unification-design.md`. Pay
   special attention to:
   - The "Termination criteria for the project" section (the 10
     conditions)
   - The loop-specific goal at the start of each loop's section

3. **Read the loop's tracking file** in full. Confirm every item has
   `status: done` with both `spec_review: PASSED` and
   `code_review: PASSED`. If any item is not done, abort and report
   `LOOP NOT READY FOR VERIFICATION`.

## Verification checks

Run each check in order. Report each as `PASS`, `FAIL`, or `N/A FOR
THIS LOOP` with evidence.

### Universal checks (all loops)

**Check 1: Tracking file completeness.**
Every item in the tracking file is `status: done`. No items in any
other state. Run a Grep for `status: pending` and `status: claimed`
and `status: implementing` and `status: spec-review` and
`status: code-review` against the tracking file. Expected: zero
matches.

**Check 2: Test suite green.**
Run `go test ./...` from the repo root. Expected: PASS. Capture the
summary line.

**Check 3: No new banned comments.**
For commits made during this loop, run:

```
git log --since='<loop start commit>' --pretty=format:%H | xargs -I{} git show {} -- '*.go' | grep -E '^\+.*// (TODO|FIXME|HACK|fallback)'
```

Expected: zero matches. List any matches.

**Check 4: No new t.Skip.**
For commits made during this loop, run:

```
git log --since='<loop start commit>' --pretty=format:%H | xargs -I{} git show {} -- '*_test.go' | grep '^\+.*t\.Skip'
```

Expected: zero matches outside the documented async deferral allow-list
(if Loop 3, check `loop-3-async-deferred.md` for the allow-list).

**Check 5: No helpers without consumers.**
For every new public function added during this loop, Grep for callers.
Expected: every new function has at least one caller other than its
own test file. List any orphans.

### Loop 1 specific checks

**Check L1-1: Type representation is unified.**
- `internal/component/binary/types.go` no longer defines `TypeDef`,
  `RecordTypeDef`, `VariantTypeDef`, `ListTypeDef`, `OptionTypeDef`,
  `ResultTypeDef`, `TupleTypeDef`, `FlagsTypeDef`, `EnumTypeDef`. Run
  Grep for each name and confirm zero matches in production source.

**Check L1-2: The four converters are gone.**
Run Grep for `resolveToValType`, `typeDefToValType`,
`valTypeRefToValType`, `(*TypeResolver).resolveDefinedType`. Expected:
zero matches in production source. (The reduced TypeResolver may still
exist as a lookup helper; if so, confirm it has no Defined-type
conversion logic.)

**Check L1-3: Own/Borrow carry `*ResourceType`.**
Read `internal/component/types/resource.go` and confirm
`types.Own.Resource` and `types.Borrow.Resource` are `*ResourceType`,
not `uint32`. Confirm the prior TODO comments at lines 14-16 and 35-38
are gone (the work is done).

**Check L1-4: FixedSizeList, Stream, Future, ErrorContext exist.**
Read `internal/component/types/composite.go` (or wherever the value
types live) and confirm all four cases exist. Stream/Future/ErrorContext
must trap on lift/lower with a clear "async not yet supported" message.

**Check L1-5: Python tests are ported and green.**
Run `go test ./internal/component/conformance/canonical_abi/...`.
Expected: PASS. Confirm the test count is at least 272 subtests
(matching the design's count).

**Check L1-6: Spec coverage report exists.**
`docs/plans/projects/abi-unification/loop-1-spec-coverage-report.md`
exists. Read it. Confirm every CanonicalABI.md section is marked
`implemented`, `deferred`, or `N/A`, with no blanks.

### Loop 2 specific checks

**Check L2-1: abi/ is the only lift/lower implementation.**
Run Grep for `liftRecord`, `liftResolvedType`, `liftFromStack`,
`flattenVariantType`, `isWiderValueType`, `writeValToMemory`,
`writeRecordToMemory`, `writeResultsToMemory`, `createCanonLowerFunc`,
`LoweredFunc.CallWithStack` body. Expected: zero matches in production
source. (The functions may be replaced by thin shims that call
`abi.CanonLift`/`abi.CanonLower`; the helpers themselves are gone.)

**Check L2-2: Standalone resource helpers are gone.**
Run Grep for `LiftOwn`, `LiftBorrow`, `LowerOwn`, `LowerBorrow` as
exported function names in `internal/component/abi/`. Expected: zero
matches as standalone functions (they exist only as inner cases of the
unified dispatch).

**Check L2-3: Silent-default error suppression is gone.**
Read `imports/wasip2/sockets/tcp.go`, `udp.go`,
`imports/wasip2/http/http.go`. Run Grep for the pattern
`if err != nil .*\n.*Fallback|return Val(Bool|U16|Own|Result)` (or
equivalent). Expected: zero matches. Then read each function and
confirm error paths trap or return `result.err(...)`.

**Check L2-4: TestPublicAPIAddS32 is no longer skipped.**
Read `internal/component/integration_public_api_test.go` and
confirm `TestPublicAPIAddS32` does not contain `t.Skipf`.

**Check L2-5: Previously-broken tests pass.**
Run `go test -run 'TestCalculatorPlugins|TestHostImport|TestPublicAPI|TestProperty|TestWasiExercise' ./...`
Expected: PASS.

**Check L2-6: Deletion report exists.**
`docs/plans/projects/abi-unification/loop-2-deletion-report.md` exists
and lists every removed file/function with line counts.

### Loop 3 specific checks

**Check L3-1: No `internal/component` imports in test files outside
the allow-list.**
Run Grep for `internal/component` in `**/*_test.go`. Expected: matches
only in `internal/component/abi/`, `internal/component/conformance/`,
and `internal/component/spectest/`. Any other match is a violation.

**Check L3-2: Spec WAST suite is wired.**
`internal/component/spectest/testdata/upstream/` exists with at least
65 `.wast` files (69 minus the documented async deferrals). Run
`go test ./internal/component/spectest/...`. Expected: PASS with
either matches or documented skips.

**Check L3-3: wit-bindgen runtime suite is wired.**
`internal/component/wasip2test/upstream/wit-bindgen/` exists with at
least 30 case directories. `upstream_wit_bindgen_test.go` exists.
Run `go test -run TestUpstreamWitBindgen ./internal/component/wasip2test/...`.
Expected: PASS.

**Check L3-4: Existing wasip2test files use only public API.**
Read `calculator_test.go`, `composition_test.go`, `converter_test.go`,
`kv_store_test.go`, `large_record_test.go`, `linking_test.go`,
`nested_types_test.go`, `variant_types_test.go`, `wasi_exercise_test.go`.
Run Grep on each for `internal/component`. Expected: zero matches.
(`internal/component/testutil` is also banned.)

**Check L3-5: Coverage matrix exists and is complete.**
`docs/plans/projects/abi-unification/loop-3-coverage-matrix.md` exists.
Read it. Confirm every `abi/` entry point is exercised by at least one
test that uses only the public API.

## Output

Produce a structured loop completion report and write it to
`docs/plans/projects/abi-unification/loop-{LOOP_NUMBER}-completion-report.md`:

```markdown
# Loop {LOOP_NUMBER} Completion Report

**Date:** YYYY-MM-DD
**Verifier:** loop-verifier subagent
**Verdict:** COMPLETE | INCOMPLETE

## Universal checks
- Check 1 (tracking file completeness): PASS|FAIL — <evidence>
- Check 2 (test suite green): PASS|FAIL — <go test output summary>
- Check 3 (no new banned comments): PASS|FAIL — <list any>
- Check 4 (no new t.Skip): PASS|FAIL — <list any>
- Check 5 (no helpers without consumers): PASS|FAIL — <list any>

## Loop-specific checks
- Check L{N}-1: PASS|FAIL — <evidence>
- ...

## Findings
<any FAIL items expanded with what is wrong and what must be fixed>

## Verdict reasoning
<one paragraph>
```

If verdict is `COMPLETE`, the loop is officially done and the next loop
opens. If `INCOMPLETE`, the report is committed and the failing checks
become new tracking-file items at the end of the current loop's
backlog.

## Reject-on-violation rules

1. **Run the actual commands.** Do not infer "this probably passes". If
   you do not actually run `go test ./...` and capture the output, your
   report is invalid.

2. **Cite line numbers.** When you find a banned pattern, cite the
   exact file:line.

3. **Do not be lenient.** A failing check is a failing check. If you
   are tempted to mark a FAIL as "PASS with notes", that's a FAIL.

4. **Do not skip checks.** If a check is genuinely N/A for the loop
   (e.g. Loop 1-specific checks during a Loop 2 verification), mark
   it `N/A FOR THIS LOOP`. Do not omit it from the report.
