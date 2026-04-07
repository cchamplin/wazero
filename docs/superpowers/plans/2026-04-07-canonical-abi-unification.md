# Canonical ABI Unification — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the orchestration system (prompts, scripts, templates, status files, Python harness, README) that will execute the three-loop canonical-ABI unification work described in `docs/superpowers/specs/2026-04-07-canonical-abi-unification-design.md`.

**Architecture:** This plan produces the *machinery* that drives the actual canonical-ABI work. The plan does NOT do the canonical-ABI work itself. After this plan completes, a separate session is started by feeding `prompts/loop1-abi-correctness.md` to a fresh Claude instance, and the iterative loops begin. The orchestration consists of: 3 loop prompts, 3 iteration scripts, 28 templates, 14 status files, 1 Python harness (+1 test file), 1 project README — all under `docs/superpowers/projects/2026-04-07-canonical-abi-unification/` (uncommitted, never `git add`'d).

**Tech Stack:** Markdown (prompts, scripts, templates, README), JSON (status files), Python 3 (spec_diff_driver.py — the only real source file in the project dir).

**Critical reading before executing this plan:**
- `docs/superpowers/specs/2026-04-07-canonical-abi-unification-design.md` (the design spec — every task references it)
- `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py` (skim sections; needed for harness work)
- `debug-vendored/component-model/design/mvp/CanonicalABI.md` (skim table of contents)

**Operational note:** The project directory `docs/superpowers/projects/2026-04-07-canonical-abi-unification/` is **uncommitted**. Files created by this plan are NOT `git add`'d. The plan produces no commits that touch this directory. The only commits this plan makes are commits that reference the spec doc itself (e.g., a final completion commit on the plan file).

---

## Phase 0 — Project directory bootstrap

### Task 1: Create the project directory tree

**Files:**
- Create directory: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/`
- Create directory: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/prompts/`
- Create directory: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/scripts/`
- Create directory: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/`
- Create directory: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/`
- Create directory: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/reconciliation/`
- Create directory: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/audit/`
- Create directory: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/harness/`

- [ ] **Step 1: Create the directory tree**

Run:
```bash
mkdir -p docs/superpowers/projects/2026-04-07-canonical-abi-unification/{prompts,scripts,templates,status/reconciliation,status/audit,harness}
```

- [ ] **Step 2: Verify the directory tree exists**

Run:
```bash
find docs/superpowers/projects/2026-04-07-canonical-abi-unification -type d | sort
```

Expected output (8 lines):
```
docs/superpowers/projects/2026-04-07-canonical-abi-unification
docs/superpowers/projects/2026-04-07-canonical-abi-unification/harness
docs/superpowers/projects/2026-04-07-canonical-abi-unification/prompts
docs/superpowers/projects/2026-04-07-canonical-abi-unification/scripts
docs/superpowers/projects/2026-04-07-canonical-abi-unification/status
docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/audit
docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/reconciliation
docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates
```

- [ ] **Step 3: Verify the project dir is gitignored or untracked**

Run:
```bash
git status docs/superpowers/projects/2026-04-07-canonical-abi-unification/ 2>&1
```

Expected: either "untracked files" listing the directory, OR the directory is silent because nothing in it is tracked yet. Either is acceptable. **Do not `git add` anything in this directory at any point during this plan.**

---

## Phase 1 — Status file schemas and initial state

All status files use a `schema_version: 1` field. When the parent agent reads a file, it asserts this matches expected. Files start in their initial empty state and are populated by enumeration templates run during the loops.

### Task 2: Write Loop 1 status files

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/loop1-functions.json`
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/loop1-existing-conformance-audit.json`
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/loop1-async-stubs.json`
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/loop1-publicapi-coverage.json`

- [ ] **Step 1: Write loop1-functions.json**

Content:
```json
{
  "schema_version": 1,
  "loop": 1,
  "started": null,
  "functions": {}
}
```

The `functions` object will be populated by `templates/enumerate-functions.md` at Loop 1 start. Each function entry will have the form:
```json
{
  "py_ref": "definitions.py:lift_flat (lines 142-201)",
  "spec_ref": "CanonicalABI.md §Flat Lifting",
  "gates": {
    "gate1": { "status": "pending" },
    "gate2": { "status": "pending" },
    "gate3": { "status": "pending" },
    "gate4": { "status": "pending" },
    "gate6": { "status": "pending" },
    "gate7": { "status": "pending" }
  },
  "review_chains": [],
  "iteration_status": "pending"
}
```

There is no `gate5`. See spec §5.7.

- [ ] **Step 2: Write loop1-existing-conformance-audit.json**

Content:
```json
{
  "schema_version": 1,
  "started": null,
  "files": {}
}
```

Populated at Loop 1 start by listing `internal/component/conformance/*_test.go`. Each file entry:
```json
{
  "audit_status": "pending",
  "audit_subagent_id": null,
  "review_chains": [],
  "header_added": false,
  "started": null,
  "completed": null,
  "findings_count": 0
}
```

- [ ] **Step 3: Write loop1-async-stubs.json**

Content:
```json
{
  "schema_version": 1,
  "started": null,
  "stubs": {}
}
```

Populated by `templates/enumerate-functions.md` (Task 24), which produces BOTH the sync function list (in `loop1-functions.json`) and the async stub list (in `loop1-async-stubs.json`) in a single subagent dispatch. Each stub entry:
```json
{
  "spec_ref": "CanonicalABI.md §Stream State",
  "missing_primitive": "canon stream.new",
  "stub_status": "pending",
  "review_chains": [],
  "started": null,
  "completed": null
}
```

- [ ] **Step 4: Write loop1-publicapi-coverage.json**

Content:
```json
{
  "schema_version": 1,
  "coverage": {}
}
```

Each coverage entry:
```json
{
  "fixture": "internal/component/testdata/add_s32.wasm",
  "test": "TestPublicAPI_LiftFlat"
}
```

A function with `fixture: null` or `test: null` is not Gate 6 pass.

- [ ] **Step 5: Verify all four files parse as JSON**

Run:
```bash
for f in docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/loop1-*.json; do
  python3 -c "import json,sys; json.load(open(sys.argv[1])); print(sys.argv[1], 'OK')" "$f"
done
```

Expected: 4 lines, each ending with `OK`.

### Task 3: Write Loop 2 status files

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/loop2-callsites.json`
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/loop2-baseline.json`
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/loop2-existing-wiring-audit.json`
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/loop2-wiring-tests.json`

- [ ] **Step 1: Write loop2-callsites.json**

Content:
```json
{
  "schema_version": 1,
  "loop": 2,
  "started": null,
  "callsites": {},
  "deletion_targets": []
}
```

`callsites` populated by `templates/enumerate-callsites.md` at Loop 2 start. Each callsite entry:
```json
{
  "file": "internal/component/instance.go",
  "function": "liftFieldFromMemory",
  "lines": "1242-1395",
  "calls_into": "liftFromMemory and helpers",
  "replaces_with": "abi.LiftHeap",
  "migration_status": "pending",
  "wiring_test": null
}
```

`deletion_targets` is an array of helper symbol names that will be grep-verified as having zero references at Loop 2 termination per spec §9.4 P-T2. Populated by the same enumeration template.

- [ ] **Step 2: Write loop2-baseline.json**

Content:
```json
{
  "schema_version": 1,
  "captured": null,
  "command": "go test ./...",
  "total_tests": 0,
  "passing": 0,
  "failing": 0,
  "failing_test_names": []
}
```

Captured by `templates/capture-loop2-baseline.md` at Loop 2 start. **Read-only after capture.**

- [ ] **Step 3: Write loop2-existing-wiring-audit.json**

Content:
```json
{
  "schema_version": 1,
  "started": null,
  "files": {}
}
```

Each entry mirrors the loop1-existing-conformance-audit format but for wiring-layer test files (`canon_lower_test.go`, `component_linker_test.go`, `instance_test.go`, files under `wasip2test/`).

- [ ] **Step 4: Write loop2-wiring-tests.json**

Content:
```json
{
  "schema_version": 1,
  "tests": {}
}
```

Each entry maps a test name to the callsite it covers:
```json
{
  "callsite": "instance.go:liftFieldFromMemory:1242",
  "fixture": "internal/component/testdata/echo_record.wasm",
  "added_in_commit": "<sha>"
}
```

- [ ] **Step 5: Verify all four files parse as JSON**

Run:
```bash
for f in docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/loop2-*.json; do
  python3 -c "import json,sys; json.load(open(sys.argv[1])); print(sys.argv[1], 'OK')" "$f"
done
```

Expected: 4 lines, each ending with `OK`.

### Task 4: Write Loop 3 status files

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/loop3-suppressed-errors.json`
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/loop3-baseline.json`

- [ ] **Step 1: Write loop3-suppressed-errors.json**

Content:
```json
{
  "schema_version": 1,
  "loop": 3,
  "started": null,
  "sites": {}
}
```

Each site entry:
```json
{
  "file": "imports/wasip2/sockets/tcp.go",
  "function": "tcpSocketStartBind",
  "lines": "101-105",
  "fix_status": "pending",
  "trap_test": null,
  "review_chains": []
}
```

- [ ] **Step 2: Write loop3-baseline.json**

Content:
```json
{
  "schema_version": 1,
  "captured": null,
  "command": "go test ./...",
  "total_tests": 0,
  "passing": 0,
  "failing": 0,
  "failing_test_names": []
}
```

Captured at Loop 3 start. Read-only after capture.

- [ ] **Step 3: Verify both files parse as JSON**

Run:
```bash
for f in docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/loop3-*.json; do
  python3 -c "import json,sys; json.load(open(sys.argv[1])); print(sys.argv[1], 'OK')" "$f"
done
```

Expected: 2 lines, each ending with `OK`.

### Task 5: Write shared status files

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/blockers.json`
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/iteration-log.json`
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/wasmtime-fixture-corpus.json`
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/project-state.json`

- [ ] **Step 1: Write blockers.json**

Content:
```json
{
  "schema_version": 1,
  "blockers": []
}
```

Each blocker entry (appended by subagents via `templates/file-blocker.md`):
```json
{
  "id": "blk-001",
  "filed": "2026-04-07T15:28:00Z",
  "filed_by_subagent_id": "agent_f55de",
  "loop": 1,
  "iteration_function": "LowerFlat",
  "kind": "definitions_py_vs_spec_disagreement",
  "summary": "...",
  "spec_quote_attempted": "...",
  "py_quote_attempted": "...",
  "resolution_status": "open",
  "resolved_by": null,
  "resolved_at": null,
  "resolution_notes": null
}
```

`resolution_status` is one of: `open`, `resolved`, `wontfix`.

- [ ] **Step 2: Write iteration-log.json**

Content:
```json
{
  "schema_version": 1,
  "iterations": []
}
```

Each iteration entry (appended by parent agent at iteration end):
```json
{
  "iteration_id": 1,
  "loop": 1,
  "function": "LiftFlat",
  "started": "2026-04-07T14:32:00Z",
  "completed": "2026-04-07T16:16:00Z",
  "gates_run": ["gate1", "gate2", "gate3", "gate4", "gate6", "gate7"],
  "review_chains": 5,
  "commits": ["abc1234", "def5678"],
  "blockers_filed": [],
  "outcome": "complete"
}
```

`outcome` is one of: `complete`, `halted_for_blocker`, `regression_reverted`.

- [ ] **Step 3: Write wasmtime-fixture-corpus.json**

Content:
```json
{
  "schema_version": 1,
  "wasmtime_version_required": null,
  "captured": null,
  "fixtures": []
}
```

Each fixture entry (populated by `templates/enumerate-wasmtime-fixtures.md` at Loop 1 start):
```json
{
  "path": "debug-vendored/wit-bindgen/tests/runtime/numbers/wasm.wasm",
  "exports_to_invoke": ["test-numbers"],
  "exercises_abi_functions": ["LiftFlat", "LowerFlat"],
  "skip_reason": null,
  "spec_section": null
}
```

`skip_reason` is null for fixtures that should run, or a string naming the missing primitive when out of scope (per spec §5.4 allowed skips).

- [ ] **Step 4: Write project-state.json**

Content:
```json
{
  "schema_version": 1,
  "loop1_complete": false,
  "loop1_completed_at": null,
  "loop1_completion_sha": null,
  "loop2_complete": false,
  "loop2_completed_at": null,
  "loop2_completion_sha": null,
  "loop3_complete": false,
  "loop3_completed_at": null,
  "loop3_completion_sha": null,
  "project_complete": false,
  "project_completed_at": null
}
```

Updated by parent agents at loop termination. `loop1_completion_sha` is the git SHA at the moment Loop 1 was declared complete; Loop 2's L2-T3 termination check uses this to verify `internal/component/abi/` is unchanged.

- [ ] **Step 5: Verify all four files parse as JSON**

Run:
```bash
for f in docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/{blockers,iteration-log,wasmtime-fixture-corpus,project-state}.json; do
  python3 -c "import json,sys; json.load(open(sys.argv[1])); print(sys.argv[1], 'OK')" "$f"
done
```

Expected: 4 lines, each ending with `OK`.

- [ ] **Step 6: Final status directory verification**

Run:
```bash
ls -1 docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/*.json | wc -l
```

Expected: `14` (4 loop1 + 4 loop2 + 2 loop3 + 4 shared).

---

## Phase 2 — Python differential harness

The Python harness `spec_diff_driver.py` is the only real source file in the project dir. It wraps `definitions.py` from `debug-vendored/component-model/design/mvp/canonical-abi/` and exposes a JSON-IO protocol over stdin/stdout that the Go differential test (Gate 3) uses.

### Task 6: Write spec_diff_driver.py with TDD

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/harness/spec_diff_driver.py`
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/harness/test_spec_diff_driver.py`

- [ ] **Step 1: Verify Python and definitions.py are available**

Run:
```bash
python3 --version && python3 -c "import sys; sys.path.insert(0, 'debug-vendored/component-model/design/mvp/canonical-abi'); import definitions; print('definitions OK', dir(definitions)[:5])"
```

Expected: a Python version line, then `definitions OK [...]` listing some attributes. If `definitions.py` has missing imports, install required Python deps via `pip3 install` for whatever it needs (the user said tooling can be installed in traditional ways).

- [ ] **Step 2: Write the failing test for the driver's `--list-sync-functions` mode**

Create `docs/superpowers/projects/2026-04-07-canonical-abi-unification/harness/test_spec_diff_driver.py`:
```python
"""Tests for spec_diff_driver.py — verify the JSON-IO protocol forwards
to definitions.py without altering inputs or losing precision in outputs."""

import json
import subprocess
import sys
from pathlib import Path

DRIVER = Path(__file__).parent / "spec_diff_driver.py"


def run_driver(stdin_json):
    """Invoke the driver as a subprocess; return parsed JSON output."""
    proc = subprocess.run(
        [sys.executable, str(DRIVER)],
        input=json.dumps(stdin_json),
        capture_output=True,
        text=True,
        check=False,
    )
    if proc.returncode != 0:
        raise AssertionError(
            f"driver exited {proc.returncode}\nstderr: {proc.stderr}"
        )
    return json.loads(proc.stdout)


def test_list_sync_functions():
    """The driver in --list-sync-functions mode returns the names of every
    synchronous canonical-ABI function defined in definitions.py."""
    proc = subprocess.run(
        [sys.executable, str(DRIVER), "--list-sync-functions"],
        capture_output=True,
        text=True,
        check=True,
    )
    funcs = json.loads(proc.stdout)
    assert isinstance(funcs, list)
    assert "lift_flat" in funcs
    assert "lower_flat" in funcs
    assert "lift_heap" in funcs or "load" in funcs
    assert "store" in funcs
    # Async functions must NOT appear in the sync list:
    assert "canon_stream_new" not in funcs
    assert "canon_thread_spawn_ref" not in funcs


def test_lift_flat_i32_roundtrip():
    """Lift a single i32 from a flat core value list. Verify the result
    matches definitions.py's canonical lift output."""
    response = run_driver({
        "op": "lift_flat",
        "type": {"kind": "S32"},
        "vals": [42],
        "opts": {"string_encoding": "utf8"},
    })
    assert response["trapped"] is False
    assert response["result"] == 42


def test_lower_flat_i32_roundtrip():
    """Lower a single i32 to a flat core value list."""
    response = run_driver({
        "op": "lower_flat",
        "type": {"kind": "S32"},
        "value": 42,
        "opts": {"string_encoding": "utf8"},
    })
    assert response["trapped"] is False
    assert response["result"] == [42]


def test_lift_flat_invalid_input_traps():
    """An out-of-range i8 value should trap."""
    response = run_driver({
        "op": "lift_flat",
        "type": {"kind": "S8"},
        "vals": [256],  # out of range for s8
        "opts": {"string_encoding": "utf8"},
    })
    assert response["trapped"] is True
    assert isinstance(response["reason"], str) and len(response["reason"]) > 0


def test_unknown_op_returns_error():
    """An unknown op key returns a structured error, not an exception."""
    response = run_driver({
        "op": "this_op_does_not_exist",
    })
    assert "error" in response
```

- [ ] **Step 3: Run the test to verify it fails because the driver does not exist yet**

Run:
```bash
cd docs/superpowers/projects/2026-04-07-canonical-abi-unification/harness && python3 -m pytest test_spec_diff_driver.py -v 2>&1 | head -40
```

Expected: every test fails (FileNotFoundError on the driver, or pytest collection error). This is the red state.

- [ ] **Step 4: Write a minimal `spec_diff_driver.py` that handles `--list-sync-functions` mode only**

Create `docs/superpowers/projects/2026-04-07-canonical-abi-unification/harness/spec_diff_driver.py`:
```python
#!/usr/bin/env python3
"""spec_diff_driver.py — JSON-IO wrapper around the canonical reference
implementation in debug-vendored/component-model/design/mvp/canonical-abi/definitions.py.

Usage:
    spec_diff_driver.py --list-sync-functions
        Print a JSON array of every synchronous canonical-ABI function name.

    spec_diff_driver.py
        Read a JSON object from stdin describing one canonical-ABI op to
        invoke. Write the result as a JSON object to stdout.

    Stdin schema (one of):
        {"op": "lift_flat", "type": <type-encoding>, "vals": [<core-vals>], "opts": {...}}
        {"op": "lower_flat", "type": <type-encoding>, "value": <component-val>, "opts": {...}}
        {"op": "lift_heap", "type": <type-encoding>, "offset": <int>, "memory": <hex-string>, "opts": {...}}
        {"op": "store", "type": <type-encoding>, "value": <component-val>, "offset": <int>, "memory_size": <int>, "opts": {...}}

    Stdout schema:
        {"trapped": false, "result": <value>}                            (success)
        {"trapped": true, "reason": "<message>"}                         (canonical-ABI trap)
        {"error": "<message>"}                                           (driver-level error)

The driver imports definitions.py from the vendored path. It does not
modify, monkey-patch, or wrap definitions.py functions — every call goes
straight through.

This driver is the Gate 3 differential oracle. It must forward inputs
without alteration and must serialize outputs without precision loss.
Per spec §5.3, every modification to this driver triggers the full
R1 → R3 → R5 review chain.
"""

import json
import sys
import os
from pathlib import Path

# Locate definitions.py relative to the wazero repo root.
# The harness lives at docs/superpowers/projects/<slug>/harness/spec_diff_driver.py.
# definitions.py lives at debug-vendored/component-model/design/mvp/canonical-abi/definitions.py.
HARNESS_DIR = Path(__file__).resolve().parent
REPO_ROOT = HARNESS_DIR.parents[4]
DEFINITIONS_DIR = REPO_ROOT / "debug-vendored" / "component-model" / "design" / "mvp" / "canonical-abi"

if not (DEFINITIONS_DIR / "definitions.py").exists():
    print(json.dumps({"error": f"definitions.py not found at {DEFINITIONS_DIR}"}), file=sys.stderr)
    sys.exit(2)

sys.path.insert(0, str(DEFINITIONS_DIR))
import definitions  # type: ignore
from definitions import *  # type: ignore  # noqa: F401,F403

# Force deterministic profile so the driver and Go side observe the same
# behavior across runs (run_tests.py does the same at line 6).
definitions.DETERMINISTIC_PROFILE = True


# Names of every synchronous canonical-ABI top-level function exported by
# definitions.py. This list MUST be kept in sync with definitions.py if
# the upstream spec adds or renames sync functions. Each name listed here
# must exist as a callable attribute on the `definitions` module — the
# --list-sync-functions verifier checks this on every invocation.
#
# Async functions (canon_stream_*, canon_future_*, canon_error_context_*,
# canon_thread_*, canon_waitable_set_*, canon_task_return, canon_task_cancel,
# canon_subtask_*, canon_context_*) are explicitly EXCLUDED.
SYNC_FUNCTIONS = [
    "lift_flat",
    "lower_flat",
    "load",
    "store",
    "lift_own",
    "lift_borrow",
    "lower_own",
    "lower_borrow",
    "flatten_functype",
    "flatten_type",
    "alignment",
    "elem_size",
    "size",
    "canon_lift",
    "canon_lower",
    "canon_resource_new",
    "canon_resource_drop",
    "canon_resource_rep",
]


def list_sync_functions():
    """Verify every name in SYNC_FUNCTIONS exists on the definitions module
    and return the list. Names not present in definitions.py are dropped
    from the returned list with a warning to stderr."""
    present = []
    for name in SYNC_FUNCTIONS:
        if hasattr(definitions, name):
            present.append(name)
        else:
            print(
                f"WARNING: SYNC_FUNCTIONS lists '{name}' but definitions.py "
                f"has no such attribute; dropping",
                file=sys.stderr,
            )
    return present


def main():
    if len(sys.argv) >= 2 and sys.argv[1] == "--list-sync-functions":
        print(json.dumps(list_sync_functions()))
        return 0

    # Stdin mode: read one JSON request, write one JSON response.
    try:
        req = json.loads(sys.stdin.read())
    except json.JSONDecodeError as e:
        print(json.dumps({"error": f"invalid JSON on stdin: {e}"}))
        return 1

    op = req.get("op")
    if op is None:
        print(json.dumps({"error": "missing 'op' field"}))
        return 1

    handler = OPS.get(op)
    if handler is None:
        print(json.dumps({"error": f"unknown op '{op}'"}))
        return 1

    try:
        response = handler(req)
    except definitions.Trap as t:
        response = {"trapped": True, "reason": str(t) or "trap"}
    except Exception as e:
        response = {"error": f"{type(e).__name__}: {e}"}

    print(json.dumps(response))
    return 0


# Op handlers — each takes the parsed request dict and returns a response dict.
# More handlers will be added as Gate 3 needs them; the initial driver only
# needs lift_flat and lower_flat to satisfy the smoke tests in Task 6.

def _decode_type(type_obj):
    """Convert a JSON type encoding to a definitions.py ValType instance.
    Initial implementation handles primitives only; composite types added
    later as Gate 3 needs them.

    Type encodings:
        {"kind": "Bool"|"S8"|"U8"|"S16"|"U16"|"S32"|"U32"|"S64"|"U64"|"F32"|"F64"|"Char"|"String"}
        {"kind": "List", "elem": <type>}
        {"kind": "Record", "fields": [{"name": str, "type": <type>}, ...]}
        ... (etc.)
    """
    kind = type_obj["kind"]
    if kind == "Bool":
        return definitions.BoolType()
    if kind == "S8":
        return definitions.S8Type()
    if kind == "U8":
        return definitions.U8Type()
    if kind == "S16":
        return definitions.S16Type()
    if kind == "U16":
        return definitions.U16Type()
    if kind == "S32":
        return definitions.S32Type()
    if kind == "U32":
        return definitions.U32Type()
    if kind == "S64":
        return definitions.S64Type()
    if kind == "U64":
        return definitions.U64Type()
    if kind == "F32":
        return definitions.F32Type()
    if kind == "F64":
        return definitions.F64Type()
    if kind == "Char":
        return definitions.CharType()
    if kind == "String":
        return definitions.StringType()
    raise NotImplementedError(f"type kind '{kind}' not yet implemented in driver")


def _make_opts(opts_obj):
    """Convert a JSON opts dict to a definitions.CanonicalOptions instance."""
    opts = definitions.CanonicalOptions()
    opts.memory = bytearray()  # default — handler can override
    opts.string_encoding = (opts_obj or {}).get("string_encoding", "utf8")
    opts.realloc = None
    opts.post_return = None
    opts.sync_task_return = False
    opts.async_ = False
    opts.callback = None
    return opts


def _make_cx(opts_obj):
    """Build a LiftLowerContext for primitive ops that don't need a real instance."""
    opts = _make_opts(opts_obj)
    inst = definitions.ComponentInstance(definitions.Store())
    return definitions.LiftLowerContext(opts, inst)


def op_lift_flat(req):
    t = _decode_type(req["type"])
    vals = req["vals"]
    cx = _make_cx(req.get("opts"))
    vi = definitions.CoreValueIter(vals)
    result = definitions.lift_flat(cx, vi, t)
    return {"trapped": False, "result": result}


def op_lower_flat(req):
    t = _decode_type(req["type"])
    val = req["value"]
    cx = _make_cx(req.get("opts"))
    result = definitions.lower_flat(cx, val, t)
    return {"trapped": False, "result": result}


OPS = {
    "lift_flat": op_lift_flat,
    "lower_flat": op_lower_flat,
}


if __name__ == "__main__":
    sys.exit(main())
```

- [ ] **Step 5: Run the tests to verify the smoke tests pass**

Run:
```bash
cd docs/superpowers/projects/2026-04-07-canonical-abi-unification/harness && python3 -m pytest test_spec_diff_driver.py -v 2>&1 | tail -30
```

Expected: at minimum `test_list_sync_functions`, `test_lift_flat_i32_roundtrip`, `test_lower_flat_i32_roundtrip`, `test_unknown_op_returns_error` pass. `test_lift_flat_invalid_input_traps` may pass or fail depending on whether `definitions.S8Type` validates range on lift — if it fails, the test is wrong, not the driver; revise the test to use a value that the spec actually traps on.

If `test_list_sync_functions` fails because some `SYNC_FUNCTIONS` names are not on `definitions`, read `definitions.py` and update `SYNC_FUNCTIONS` to match the actual function names. The driver's `list_sync_functions()` already handles this by warning to stderr and dropping missing names — but the test asserts specific names like `lift_flat` and `lower_flat` exist, so those at minimum must be present.

If a primitive type is named differently on `definitions` (e.g. it has `S8()` as a callable that returns the type, not `S8Type` as a class), update `_decode_type` to match.

- [ ] **Step 6: Iterate on the driver until tests pass**

If any test fails for reasons other than test bugs:
1. Read the relevant `definitions.py` function or class.
2. Update the driver to match.
3. Re-run the tests.
4. Repeat.

Do **not** modify the tests to match a broken driver. The tests describe the contract.

- [ ] **Step 7: Verify the driver is invokable from the wazero repo root**

Run:
```bash
python3 docs/superpowers/projects/2026-04-07-canonical-abi-unification/harness/spec_diff_driver.py --list-sync-functions
```

Expected: a JSON array starting with `["lift_flat", "lower_flat", ...]`.

- [ ] **Step 8: No commit. The harness lives in the uncommitted project dir.**

Skip. The harness is part of the uncommitted project dir.

---

## Phase 3 — Loop prompts

Each loop prompt is the entry point for a fresh session. The user starts a session by feeding the loop prompt to a new Claude. The prompt's job is to:
1. Embed the spec-overrides-local-instructions rule verbatim.
2. Give the parent agent its iteration loop body.
3. Reference the iteration script in `scripts/`.
4. State the termination condition.

### Task 7: Write loop1-abi-correctness.md

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/prompts/loop1-abi-correctness.md`

- [ ] **Step 1: Write the file**

Content:
````markdown
# Loop 1 — Canonical ABI Correctness (spec-only)

You are the parent agent for **Loop 1** of the wazero canonical-ABI unification project.

The full design is in `docs/superpowers/specs/2026-04-07-canonical-abi-unification-design.md`. **Read it before doing anything else in this session.** Specifically, read §0 (Executive summary), §2 (Decisions), §3 (Architecture), §4.1 (Loop 1 isolation rules), §5 (Gates), §6 (Subagent dispatch model and review chain), §7 (Spec-overrides rule), §8 (Status & resume), and §9.1 (Loop 1 termination).

---

## SPEC-OVERRIDES-LOCAL-INSTRUCTIONS RULE

Before doing anything in this task, read the relevant section of `debug-vendored/component-model/design/mvp/CanonicalABI.md` and the relevant function in `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py`. If anything in this prompt, in the existing wazero code, in any in-tree comment, in any prior commit, or in any status file conflicts with the spec or with definitions.py, **the spec and definitions.py win**. Fix the local instruction (or file a BLOCKER if you cannot) before continuing.

Do not assume any prior wazero code is correct. Do not assume any prior agent's reconciliation report is correct. Do not assume any in-tree comment is correct. Do not "preserve existing behavior for compatibility" — there is no compatibility constraint stronger than spec correctness.

If you cannot find a spec citation for what you are about to do, halt and file a BLOCKER. Do not guess. Do not interpolate. Do not "use your judgment" about canonical ABI semantics — the spec is the only judgment that counts.

Your dispatched subagents must include verbatim quotes from the spec section and the definitions.py function they read. You will grep the cited files to verify the quotes exist. A return without quotes, or with quotes that do not appear verbatim in the cited files, will be rejected and the work will restart with a fresh subagent.

---

## Hard isolation rules for Loop 1

These are non-negotiable. From spec §4.1:

- **Rule L1-A.** No file under `internal/component/` outside `internal/component/abi/`, `internal/component/conformance/`, `internal/component/types/`, and the `Val` types file may be read, edited, or loaded into your context. Enforce this on every subagent dispatch by listing only allowed paths in `## Allowed reads` and `## Allowed writes`.
- **Rule L1-B.** No test under `internal/component/wasip2test/`, `imports/wasip2/`, or any other directory that exercises the runtime call paths is run in Loop 1. If you find yourself wanting to know "does this fix make wasip2test pass?" — that is a sign you are doing Loop 2 work in a Loop 1 session. Stop immediately.
- **Rule L1-C.** Your only correctness oracles are: `definitions.py` (Gate 1, Gate 3), `run_tests.py` (Gate 2), `CanonicalABI.md` (every gate), the installed `wasmtime` CLI on standalone fixtures (Gate 4), and direct construction-then-call-into-`abi/` Go code (Gate 6).
- **Rule L1-D.** If a subagent reports a discrepancy between `abi/` and `definitions.py`, the resolution is **always** "fix `abi/` to match `definitions.py`" unless the subagent can produce a spec citation showing `definitions.py` itself diverges from `CanonicalABI.md`. In the second case, halt the loop and file a BLOCKER.
- **Rule L1-E.** Every Loop 1 commit lands a single verifiable, atomic improvement. Mixed commits are prohibited.

---

## Session start protocol

Before dispatching any subagent, perform these steps in order:

1. Read this entire prompt.
2. Read `docs/superpowers/specs/2026-04-07-canonical-abi-unification-design.md` in full.
3. Read `docs/superpowers/projects/2026-04-07-canonical-abi-unification/scripts/loop1-iteration.md` in full.
4. Run `git status` to confirm the working tree is clean. If there are uncommitted changes, halt and report them — uncommitted changes from a previous session indicate a crash mid-iteration that needs human inspection.
5. Run `git log --oneline -20` and read against `status/iteration-log.json`. If they disagree (a recent commit not in the log, or a logged commit not in git), halt and file a BLOCKER.
6. Read every status file under `docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/` and run a self-consistency check: confirm parseability, confirm `schema_version: 1`, confirm zero open Loop 1 blockers in `blockers.json`. If any check fails, halt and report.
7. Determine the next pending work item:
   - If `loop1-existing-conformance-audit.json` has files with `audit_status != "pass"`, the next work item is the first such file. Loop 1 cannot reconcile any function until the existing-conformance audit is complete.
   - Otherwise, if `loop1-functions.json` has functions with `iteration_status != "complete"`, the next work item is the first such function.
   - Otherwise, if `loop1-async-stubs.json` has stubs with `stub_status != "installed"`, the next work item is the first such stub.
   - Otherwise, run the Loop 1 termination check (spec §9.1). If all assertions pass, write `loop1_complete: true` to `status/project-state.json` and stop. Loop 1 is done.

8. If this is the very first Loop 1 session (every status file is empty), run the bootstrap protocol:
   a. Dispatch `templates/enumerate-functions.md` (Task 24 of the plan). This template populates BOTH `loop1-functions.json` (sync function entries) AND `loop1-async-stubs.json` (async stub entries). It is the single enumeration template for Loop 1's function-and-stub work.
   b. Populate `loop1-existing-conformance-audit.json` directly: list every `internal/component/conformance/*_test.go` file via `ls` and create one entry per file with `audit_status: "pending"`. No subagent template is needed for this — the parent agent does it itself because it is purely mechanical and has no spec content. Alternatively, if you prefer a template-driven path, dispatch a fresh subagent with the equivalent procedure inline; do NOT invent a "templates/enumerate-existing-conformance.md" file unless you also create it under templates/.
   c. Dispatch `templates/enumerate-wasmtime-fixtures.md` (Task 25) to populate `status/wasmtime-fixture-corpus.json`.
   After bootstrap, return to step 7.

---

## Iteration body

For each iteration, follow `scripts/loop1-iteration.md` exactly. The script specifies:
- Which gate runs next for the current work item.
- Which template to dispatch.
- What inputs to fill in.
- How to integrate the subagent's return.
- When to dispatch the review chain.
- When to commit.

Do not deviate from the script. If the script does not cover a situation, halt and file a BLOCKER asking for the script to be extended.

---

## Mandatory review chain after every code change

From spec §6.4. Every code-producing subagent dispatch is followed by:

1. **R1 — Spec compliance review.** Dispatch a fresh subagent using `templates/review-spec-compliance.md` with the diff as input. The reviewer is **not** the writer.
2. **R2 — Revision (if R1 returned FINDINGS).** Dispatch a fresh revision subagent using `templates/revise-after-review.md`. The reviser is **not** the writer and **not** the R1 reviewer. After revision, **restart at R1 with yet another fresh subagent.**
3. **R3 — Code quality review.** Dispatch a fresh subagent using `templates/review-code-quality.md`. The reviewer is **not** the writer and **not** the R1 reviewer.
4. **R4 — Revision (if R3 returned FINDINGS).** Dispatch a fresh revision subagent. After revision, **restart at R1**, not at R3.
5. **R5 — Commit.** Only after both R1 and R3 have returned `APPROVED` for the same diff state.

No batching. No grouping. No skipping. No deferring. No self-review. Restart-on-revision is total (any revision restarts at R1).

---

## Subagent return verification

When a subagent returns:

1. Read the structured return.
2. **Do not trust the subagent's "I did it."** Independently verify the artifacts on disk support the claim:
   - For a reconciliation report: read the file, check every row is classified.
   - For a test port: read the file, run `go test -v -run <name>`, check the output matches the subagent's claim.
   - For a code change: run `git diff --cached` (if staged) or `git diff` (if not), check it matches what the subagent described.
3. **Verify the subagent's spec/Python quotes are real.** For each verbatim quote in the return message, run `grep -F "<quote>" <cited-file>` and confirm at least one match. A quote that does not match is a protocol violation; reject the return and restart with a fresh subagent.
4. If verification passes, integrate (update status file, commit if applicable). If verification fails, file a BLOCKER naming the false claim and end the iteration.

---

## Commit messages

Every Loop 1 commit message must include footer lines:

```
Spec: CanonicalABI.md §<section>
Reference: definitions.py:<func> (lines <start>-<end>)
Loop1-Function: <func_name>
Loop1-Gate: <gate>
Writer: <writer_template_name>
R1-Reviewer: <r1_subagent_id> (approved)
R3-Reviewer: <r3_subagent_id> (approved)
Revisions: <count>
Subagents: <id-list>
```

A commit without these lines is a protocol violation. Reject and re-do.

---

## Termination

When the session-start protocol step 7 finds zero pending work items, run the Loop 1 termination check from spec §9.1. The check has nine assertions L1-T1 through L1-T9. Each is mechanical. Run them in order. If all pass, write to `status/project-state.json`:

```json
{
  "loop1_complete": true,
  "loop1_completed_at": "<current ISO timestamp>",
  "loop1_completion_sha": "<git rev-parse HEAD>"
}
```

Then commit `status/project-state.json` is **not** committed (it lives in the uncommitted project dir). Instead, surface to the user: "Loop 1 complete. project-state.json updated. Loop 2 may now start by feeding `prompts/loop2-runtime-migration.md` to a fresh Claude."

Stop accepting Loop 1 iterations. Loop 1 is sealed.

---

## What you do NOT do

- You do not write code yourself. Every code change is produced by a dispatched subagent.
- You do not read `definitions.py` or `CanonicalABI.md` yourself. Subagents read them and quote back.
- You do not skip the review chain.
- You do not batch iterations.
- You do not parallelize subagents within an iteration.
- You do not edit any file outside `internal/component/abi/`, `internal/component/conformance/`, `internal/component/types/`, and the project status dir.
- You do not run any test under `wasip2test/` or `imports/wasip2/`.
- You do not declare Loop 1 complete unilaterally. The termination check must pass first.

---

Begin by performing the session start protocol.
````

- [ ] **Step 2: Verify the file exists and is readable**

Run:
```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/prompts/loop1-abi-correctness.md
```

Expected: line count > 100.

### Task 8: Write loop2-runtime-migration.md

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/prompts/loop2-runtime-migration.md`

- [ ] **Step 1: Write the file**

Content (mirrors loop1 structurally; differences highlighted in comments below):
````markdown
# Loop 2 — Runtime Migration (migration-only)

You are the parent agent for **Loop 2** of the wazero canonical-ABI unification project.

The full design is in `docs/superpowers/specs/2026-04-07-canonical-abi-unification-design.md`. **Read it before doing anything else in this session.** Specifically, read §0, §3, §4.2 (Loop 2 isolation rules), §5 (Gates — for understanding what Loop 1 produced, not for running them), §6 (Subagent dispatch and review chain), §7 (Spec-overrides rule), §8 (Status & resume), and §9.2 (Loop 2 termination).

---

## SPEC-OVERRIDES-LOCAL-INSTRUCTIONS RULE

[Insert verbatim text from spec §7.5 — same as in loop1 prompt. Do not abbreviate. Do not summarize. Repeat the full text here.]

---

## Pre-flight check

Before doing anything, confirm Loop 1 is complete:

1. Read `status/project-state.json`. Confirm `loop1_complete: true` and `loop1_completion_sha` is non-null.
2. If Loop 1 is not complete, halt immediately and report: "Loop 2 cannot start; Loop 1 is not complete. Run `prompts/loop1-abi-correctness.md` first." Do not proceed.

---

## Hard isolation rules for Loop 2

From spec §4.2:

- **Rule L2-A.** No file under `internal/component/abi/` may be edited in this session. Read-only access is allowed (you need to know the abi/ API to call it). Any subagent that tries to write to `abi/` halts immediately and files a BLOCKER. A human reopens Loop 1 to address it under the full Gate 1–7 protocol, then Loop 2 resumes.
- **Rule L2-B.** Loop 2 commits replace one runtime call site at a time. After each replacement, run `go test ./...`. The failing-test count is read from `status/loop2-baseline.json` (captured at Loop 2 start). The new failing-test count is compared:
  - Strictly less → expected; the migration fixed a runtime bug. Commit.
  - Equal → expected; the call site was already producing correct behavior or the test doesn't yet cover it. Commit.
  - Strictly greater → regression. Revert. Investigate via `templates/diagnose-loop2-regression.md`. Do not commit. Do not edit `abi/`.
  - New failure that was passing before, even if total count is unchanged → regression. Same handling.
- **Rule L2-C.** Dead-helper deletion is its own commit, separate from the migration commit that orphaned the helper. The deletion commit verifies via grep that the symbol has zero references in the entire tree before deleting.
- **Rule L2-D.** Loop 2 may add **wiring tests** through `templates/add-wiring-test.md`. Wiring tests exercise migrated call sites through the public wazero API using real component fixtures. Loop 2 may also audit-and-fix existing wiring-layer tests via `templates/audit-existing-wiring-test.md`. Loop 2 may **not** add or modify any file under `internal/component/conformance/` (sealed at Loop 2 start). Wiring tests may not import `internal/component/abi` directly.

---

## Session start protocol

1. Read this entire prompt.
2. Read `docs/superpowers/specs/2026-04-07-canonical-abi-unification-design.md` in full.
3. Read `docs/superpowers/projects/2026-04-07-canonical-abi-unification/scripts/loop2-iteration.md` in full.
4. Run the Pre-flight check above.
5. `git status` clean check.
6. `git log --oneline -20` cross-check against `status/iteration-log.json`.
7. Read every status file. Confirm parseability, schema_version, zero open Loop 2 blockers.
8. If `status/loop2-callsites.json[callsites]` is empty, this is the first Loop 2 session. Run the bootstrap:
   - Dispatch `templates/enumerate-callsites.md` to enumerate every runtime call site that performs canonical-ABI work and populate `loop2-callsites.json[callsites]` and `loop2-callsites.json[deletion_targets]`.
   - Dispatch `templates/capture-loop2-baseline.md` to run `go test ./...` and capture the failure baseline to `loop2-baseline.json`.
   - Populate `loop2-existing-wiring-audit.json` directly: list `internal/component/wasip2test/*_test.go`, `internal/component/canon_lower_test.go`, `internal/component/component_linker_test.go`, `internal/component/instance_test.go` via `ls`/`find` and create one entry per file with `audit_status: "pending"`. The parent agent does this itself; no subagent template is needed.
9. Determine the next pending work item:
   - If `loop2-existing-wiring-audit.json` has files with `audit_status != "pass"`, the next work item is the first such file. Audit before migration.
   - Otherwise, if `loop2-callsites.json[callsites]` has callsites with `migration_status != "complete"`, the next work item is the first such callsite.
   - Otherwise, if any helper symbol in `loop2-callsites.json[deletion_targets]` still has references in the tree (verify via `templates/verify-grep-zero.md`), dispatch `templates/delete-dead-helper.md` for that symbol.
   - Otherwise, run the Loop 2 termination check (spec §9.2).

---

## Iteration body

For each iteration, follow `scripts/loop2-iteration.md` exactly.

---

## Mandatory review chain

[Same as Loop 1. Insert verbatim from spec §6.4. Do not abbreviate.]

---

## Subagent return verification

[Same as Loop 1. Insert verbatim.]

---

## Commit messages

Every Loop 2 commit must include footer lines:

```
Spec: CanonicalABI.md §<section>
Reference: definitions.py:<func> (lines <start>-<end>)
Migration: <callsite-id>   (or DeleteHelper: <symbol>)
Writer: <writer_template_name>
R1-Reviewer: <r1_subagent_id> (approved)
R3-Reviewer: <r3_subagent_id> (approved)
Revisions: <count>
Subagents: <id-list>
```

---

## Termination

Run the Loop 2 termination check from spec §9.2. Nine assertions L2-T1 through L2-T9. If all pass, update `status/project-state.json` with `loop2_complete: true`. Surface to user: "Loop 2 complete. Loop 3 may now start by feeding `prompts/loop3-wasip2-cleanup.md`."

---

## What you do NOT do

- You do not edit any file under `internal/component/abi/`. (Rule L2-A)
- You do not add or modify any file under `internal/component/conformance/`. (Rule L2-D)
- You do not commit without running `go test ./...`. (Rule L2-B)
- You do not reuse the wasip2test fixtures as proof of correctness — Loop 2 fixes wiring; Loop 3 fixes wasip2 itself.
- You do not parallelize migrations.

---

Begin by performing the Pre-flight check, then the session start protocol.
````

- [ ] **Step 2: Replace the bracketed `[Insert verbatim ...]` placeholders with actual verbatim text**

Open `docs/superpowers/projects/2026-04-07-canonical-abi-unification/prompts/loop2-runtime-migration.md` and replace each `[Insert verbatim ...]` block with the actual text copied from the corresponding section of `loop1-abi-correctness.md` (which already has the verbatim text). The three blocks to replace are:
- The SPEC-OVERRIDES-LOCAL-INSTRUCTIONS RULE block.
- The Mandatory review chain block.
- The Subagent return verification block.

After replacement, confirm there are zero `[Insert verbatim ...]` strings remaining via:
```bash
grep -c "Insert verbatim" docs/superpowers/projects/2026-04-07-canonical-abi-unification/prompts/loop2-runtime-migration.md
```
Expected: `0`.

- [ ] **Step 3: Verify the file exists**

Run:
```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/prompts/loop2-runtime-migration.md
```

Expected: line count > 100.

### Task 9: Write loop3-wasip2-cleanup.md

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/prompts/loop3-wasip2-cleanup.md`

- [ ] **Step 1: Write the file**

Use the same structure as `loop2-runtime-migration.md`, with these key changes:
- Title: `# Loop 3 — wasip2 Error-Suppression Cleanup`
- Pre-flight check: confirm `project-state.json` has both `loop1_complete: true` and `loop2_complete: true`. Halt if either is false.
- Hard isolation rules:
  - **Rule L3-A.** No file under `internal/component/` may be edited in this session. Read-only access is allowed if necessary to understand the trap path, but the only writes are to `imports/wasip2/`.
  - **Rule L3-B.** Same regression-gate as L2-B but compared to `loop3-baseline.json`.
  - **Rule L3-C.** Each suppression-site fix is its own commit. Bundling multiple fixes into one commit is prohibited.
  - **Rule L3-D.** Each fix must be accompanied by a wiring-style trap test that asserts the trap is delivered to the guest. Tests live in the appropriate `_test.go` file alongside the touched wasip2 file.
- Session start protocol:
  - Bootstrap: `templates/enumerate-wasip2-suppression-sites.md` (Task 33) populates `loop3-suppressed-errors.json[sites]`. To populate `loop3-baseline.json`, re-use `templates/capture-loop2-baseline.md` (Task 27), which despite its name takes a `{baseline_file}` input parameter and writes to whichever path the parent specifies. The parent passes `loop3-baseline.json` as the input. (The Task 27 template definition specifies the parameter — see Task 27.)
  - Next work item: first site with `fix_status != "complete"`.
- Iteration body: follow `scripts/loop3-iteration.md`.
- Commit messages: footer includes `SuppressionSite: <site-id>` instead of `Migration:`.
- Termination: spec §9.3 assertions L3-T1 through L3-T7.

Insert the SPEC-OVERRIDES-LOCAL-INSTRUCTIONS RULE block, the Mandatory review chain block, and the Subagent return verification block verbatim from `loop1-abi-correctness.md` — same prohibition on `[Insert verbatim ...]` placeholders.

- [ ] **Step 2: Verify zero placeholder strings**

Run:
```bash
grep -c "Insert verbatim" docs/superpowers/projects/2026-04-07-canonical-abi-unification/prompts/loop3-wasip2-cleanup.md
```

Expected: `0`.

- [ ] **Step 3: Verify the file exists**

Run:
```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/prompts/loop3-wasip2-cleanup.md
```

Expected: line count > 80.

---

## Phase 4 — Iteration scripts

Each iteration script is the parent agent's playbook for a single iteration of the loop. The parent reads the script at the start of every iteration. The script is **not skippable** — step 0 of every script is "re-read the spec section and `definitions.py` reference for the current work item, even if you read them in a prior iteration" per spec §7.2.

### Task 10: Write loop1-iteration.md

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/scripts/loop1-iteration.md`

- [ ] **Step 1: Write the file**

Content:
````markdown
# Loop 1 Iteration Script

This script is the parent agent's playbook for a single Loop 1 iteration. Read it at the start of every iteration. Do not skip steps. Do not batch.

---

## Step 0 — Re-read the spec for the current work item

Even if you read it in a prior iteration. The spec is the only judgment that counts. Per spec §7.2 this step is mandatory and not skippable.

1. Identify the current work item from the status file (function name, callsite, audit target, or async stub).
2. Identify the relevant `CanonicalABI.md` section. Read it in full.
3. Identify the relevant `definitions.py` function (or audit target). Read it in full, plus every helper it transitively calls.
4. State, in your session context, a one-paragraph summary of what the spec says about this work item.

---

## Step 1 — Determine the work item type

Loop 1 work items are of three kinds:

- **Existing-conformance audit.** A file in `internal/component/conformance/*_test.go` needs to be audited against `definitions.py`. Status file: `loop1-existing-conformance-audit.json`.
- **Function reconciliation.** A function in `internal/component/abi/` needs to pass all six gates (1, 2, 3, 4, 6, 7). Status file: `loop1-functions.json`.
- **Async stub installation.** An async entry point in `abi/` needs to be replaced with the explicit-error stub. Status file: `loop1-async-stubs.json`.

Existing-conformance audits run before any function reconciliation. Async stubs run after all function reconciliations.

---

## Step 2 — Dispatch the appropriate template

### For existing-conformance audit work items

1. Dispatch `templates/audit-existing-conformance-test.md` with placeholders `{file_path}` and `{spec_section}` (the section the file appears to test, determined by reading the file's package documentation).
2. Wait for the subagent to return.
3. Verify the return: read the audit report, check classifications, grep for the spec quote.
4. If the audit found zero changes needed, the subagent will have written the `// Audited <date> against definitions.py — assertions match canonical reference.` header to the file. **This is a code change.** Run the review chain (Step 4 below).
5. If the audit found changes needed, the subagent will have produced a list of findings but **not** edited the file. Dispatch `templates/fix-conformance-audit-finding.md` (one per finding, one at a time, no batching) with the finding as input. Each fix is a code change. Run the review chain after each fix.
6. After all findings are addressed, dispatch a fresh `audit-existing-conformance-test.md` to re-audit the file. Repeat until the re-audit returns zero findings, then the subagent writes the header.
7. Update `loop1-existing-conformance-audit.json[file].audit_status = "pass"`, write `header_added: true`, write `completed: <timestamp>`, append review chain entries.
8. Commit per the commit message template in `prompts/loop1-abi-correctness.md`.

### For function reconciliation work items

Run the gates in order. Each gate is its own subsection.

#### Gate 1 — Reconcile

1. Dispatch `templates/reconcile-function.md` with placeholders `{go_file}`, `{go_func}`, `{py_file: definitions.py}`, `{py_func}`, `{py_line_range}`, `{spec_section}`.
2. Verify the return: read the reconciliation report at `status/reconciliation/{go_func}.md`, check every row is classified, grep both spec and Python quotes.
3. If the report has zero unresolved `bug-in-go` rows, mark `gates.gate1.status = "pass"` and proceed to Gate 2.
4. If the report has `bug-in-go` rows, for each one (in order, no batching):
   a. Dispatch `templates/fix-reconciliation-finding.md` with the row as input.
   b. The subagent edits `internal/component/abi/<file>.go`. **This is a code change. Run the review chain (Step 4 below).**
   c. After commit, dispatch a fresh `reconcile-function.md` to re-reconcile from scratch.
   d. Repeat until re-reconciliation returns zero `bug-in-go` rows.
5. If the report has `bug-in-python` rows without spec quotes, file a BLOCKER via `templates/file-blocker.md` and end the iteration.

#### Gate 2 — Spec test

1. Determine which Python test function in `run_tests.py` is the relevant one for the function being reconciled. Look at `loop1-functions.json[function].py_ref` and the corresponding `run_tests.py` test function name.
2. Check whether `internal/component/conformance/spec_<test_name>_test.go` already exists.
3. If it does not exist, dispatch `templates/port-spec-test.md` with placeholders `{py_file: run_tests.py}`, `{py_func}`, `{py_line_range}`, `{go_dest_file}`. **The ported test file is a code change. Run the review chain (Step 4) before proceeding.**
4. Run `go test -v -run TestSpec<Name> ./internal/component/conformance/` and capture output.
5. If every subtest passes, mark `gates.gate2.status = "pass"` and proceed to Gate 3.
6. If any subtest fails, dispatch `templates/fix-reconciliation-finding.md` against the abi/ function with the failing input as a regression case. Each fix is a code change. Run the review chain. After commit, re-run the test. Loop until the test passes.

#### Gate 3 — Python differential

1. Check whether `internal/component/conformance/spec_diff_test.go` already exists.
2. If it does not exist, dispatch a one-off subagent (no specific template; use `templates/run-python-differential.md` adapted) to write the Go differential test file. **The new test file is a code change. Run the review chain.**
3. Run `go test -v -run TestSpecDiff ./internal/component/conformance/` (or the specific subtest exercising the function being reconciled) and capture output.
4. If every input matches between abi/ and the Python subprocess, mark `gates.gate3.status = "pass"` and proceed.
5. If a divergence is found, classify per Gate 1 rules and reopen Gate 1 with the divergent input as a new regression case.

#### Gate 4 — wasmtime conformance

1. Check whether `internal/component/conformance/spec_wasmtime_diff_test.go` already exists.
2. If it does not exist, dispatch a one-off subagent (use `templates/run-wasmtime-conformance.md` adapted) to write the Go wasmtime differential test file. **Code change. Review chain.**
3. Determine which fixtures in `wasmtime-fixture-corpus.json` exercise this function.
4. Run `go test -v -run TestSpecWasmtimeDiff ./internal/component/conformance/` for those fixtures.
5. If every fixture matches, mark `gates.gate4.status = "pass"`. Skipped fixtures (wasmtime not installed, async/stream out of scope, Loop 3 wasip2 territory) are allowed per spec §5.4.
6. If a divergence is found, dispatch `templates/diagnose-wasmtime-divergence.md` to classify it. Audit-only artifact; if it reopens Gate 1, follow Gate 1's protocol.

#### Gate 6 — Public-API exercise

1. Read `loop1-publicapi-coverage.json[<function>]`. If `fixture` and `test` are both non-null, run the test, confirm pass, mark `gates.gate6.status = "pass"`.
2. If they are null, dispatch `templates/build-public-api-exercise.md` with placeholders `{function}`, `{spec_section}`. The subagent picks (or builds) a fixture and writes the test. **Multi-file code change — Go test, possibly fixture source, possibly built .wasm — counts as one code change for the review chain.** Run the review chain.
3. Update `loop1-publicapi-coverage.json[<function>]` with the fixture path and test name.
4. Run the test. Confirm pass. Mark gate pass.

#### Gate 7 — Spec citation

1. Dispatch `templates/add-spec-citation.md` with placeholders `{go_func}`, `{spec_section}`, `{py_func}`, `{py_line_range}`. The subagent writes the doc comment. **Code change. Run the review chain.**
2. Mark `gates.gate7.status = "pass"`.
3. Mark `iteration_status = "complete"` for the function. Update `loop1-functions.json` with timestamps.

### For async stub installation work items

1. Dispatch `templates/install-async-stub.md` with placeholders `{go_file}`, `{go_func}`, `{spec_section}`, `{missing_primitive}`. The subagent replaces the function body with the explicit-error stub. **Code change. Review chain.**
2. Update `loop1-async-stubs.json[stub].stub_status = "installed"`.

---

## Step 3 — Independent verification

After every subagent return, before integrating:

1. Read the artifact the subagent claimed to produce.
2. For code changes, run `git diff` and confirm it matches the subagent's description.
3. For each verbatim quote in the subagent's return message, run `grep -F "<quote>" <cited-file>` and confirm at least one match.
4. If verification fails, file a BLOCKER and end the iteration.

---

## Step 4 — Mandatory review chain (for code changes only)

If the dispatched subagent produced a code change, run the review chain BEFORE committing or moving to the next gate.

1. **R1.** Dispatch `templates/review-spec-compliance.md` with the diff and the spec section as inputs. Wait for return.
2. If R1 returns FINDINGS:
   a. Dispatch `templates/revise-after-review.md` with the findings. The reviser is a fresh subagent, not the writer, not the R1 reviewer. Wait for return.
   b. After revision, **restart at R1** with another fresh subagent. Loop until R1 returns APPROVED.
3. **R3.** Dispatch `templates/review-code-quality.md` with the diff. Wait for return.
4. If R3 returns FINDINGS:
   a. Dispatch a fresh `revise-after-review.md` subagent (not the writer, not the R1 reviewer, not the R3 reviewer, not any prior reviser). Wait for return.
   b. After revision, **restart at R1** (not R3 — full restart). Loop until both R1 and R3 return APPROVED consecutively without intervening revisions.
5. **R5.** Commit. Construct the commit message per the format in `prompts/loop1-abi-correctness.md`. Append the chain entry to the appropriate `review_chains` array in the status file.

Hard constraints from spec §6.5:
- No self-review.
- No batching.
- No grouping.
- No skipping.
- No deferring.
- Restart-on-revision is total.

---

## Step 5 — Update status files and continue

1. Atomically update the status file (write to temp + rename).
2. Append iteration entry to `iteration-log.json`.
3. Return to Step 1 for the next gate or work item.

---

## Halt conditions

Halt the iteration immediately and end the session if:

- A subagent files a BLOCKER.
- A regression check (Loop 1 doesn't have one, but if a test that was passing starts failing during a fix loop, treat it as a regression) detects a regression.
- Independent verification fails (subagent claim does not match disk artifacts).
- A status file fails its self-consistency check.
- The user interrupts.

A halted iteration is **not** committed. The next session resumes by reading the most recently committed state.
````

- [ ] **Step 2: Verify**

Run:
```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/scripts/loop1-iteration.md
```
Expected: line count > 200.

### Task 11: Write loop2-iteration.md

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/scripts/loop2-iteration.md`

- [ ] **Step 1: Write the file**

Use the same structure as `loop1-iteration.md`, with these key differences:

- **Step 0** is the same: re-read the spec for the current work item.
- **Step 1** distinguishes work item types: existing-wiring audit, callsite migration, dead-helper deletion.
- **Step 2** dispatches:
  - For existing-wiring audit: `templates/audit-existing-wiring-test.md`, with the same audit-then-fix loop pattern as Loop 1's existing-conformance audit but in the wiring-test domain.
  - For callsite migration: `templates/migrate-callsite.md`, then the regression check from Rule L2-B (run `go test ./...`, compare to baseline). If the regression check fails, dispatch `templates/diagnose-loop2-regression.md` and revert. Then dispatch `templates/add-wiring-test.md` to add the wiring test for the migrated callsite. Each is a code change with its own review chain.
  - For dead-helper deletion: `templates/verify-grep-zero.md` first to confirm zero references, then `templates/delete-dead-helper.md` to perform the deletion. The deletion is a code change with its own review chain. The deletion commit is separate from the migration commit per Rule L2-C.
- **Step 3** independent verification is the same.
- **Step 4** review chain is the same.
- **Step 5** status update is the same; for migration commits the regression-check result is recorded in the iteration log.

Critically, this script must include the regression-check protocol from Rule L2-B as an explicit subsection. Pseudocode:

```
function check_regression(baseline_path):
    baseline = load_json(baseline_path)
    current = run_go_test_and_capture_failures()
    if current.failing > baseline.failing: return REGRESSION
    if current.passing_set < baseline.passing_set: return REGRESSION  # passing test now failing
    return OK
```

- [ ] **Step 2: Verify**

Run:
```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/scripts/loop2-iteration.md
```

Expected: line count > 150.

### Task 12: Write loop3-iteration.md

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/scripts/loop3-iteration.md`

- [ ] **Step 1: Write the file**

Same structure as `loop2-iteration.md`. Differences:
- Work item type: suppression-site fix.
- Template dispatch: `templates/fix-error-suppression.md` then `templates/add-wasip2-trap-test.md`. Each is a code change with its own review chain. Each suppression-site fix is its own commit per Rule L3-C.
- Regression check uses `loop3-baseline.json`.
- Status file: `loop3-suppressed-errors.json`.

- [ ] **Step 2: Verify**

Run:
```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/scripts/loop3-iteration.md
```

Expected: line count > 100.

---

## Phase 5 — Loop 1 templates

Each template is a markdown file under `templates/`. Every template has the canonical section structure from spec §6.2:

1. `# <Template Name>` — title
2. `## Spec-overrides-instructions warning` — the verbatim text from spec §7.5
3. `## First action` — read the spec/Python before doing anything
4. `## Inputs` — placeholders the parent fills in
5. `## Allowed reads` — exact files/globs the subagent may Read or Grep
6. `## Allowed writes` — exact files the subagent may Edit or Write
7. `## Procedure` — numbered steps
8. `## Halt conditions` — conditions that force a BLOCKER
9. `## Return format` — exact structure the parent expects
10. `## Self-check` — questions the subagent answers truthfully

Code-producing templates additionally end with `## After this template runs` naming the next required step (R1 spec compliance review).

The verbatim text for `## Spec-overrides-instructions warning` is from spec §7.5 and is reproduced here for copy-paste into every template:

```
## Spec-overrides-instructions warning

**SPEC-OVERRIDES-LOCAL-INSTRUCTIONS RULE.**

Before doing anything in this task, read the relevant section of `debug-vendored/component-model/design/mvp/CanonicalABI.md` and the relevant function in `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py`. If anything in this prompt, in the existing wazero code, in any in-tree comment, in any prior commit, or in any status file conflicts with the spec or with definitions.py, **the spec and definitions.py win**. Fix the local instruction (or file a BLOCKER if you cannot) before continuing.

Do not assume any prior wazero code is correct. Do not assume any prior agent's reconciliation report is correct. Do not assume any in-tree comment is correct. Do not "preserve existing behavior for compatibility" — there is no compatibility constraint stronger than spec correctness.

If you cannot find a spec citation for what you are about to do, halt and file a BLOCKER. Do not guess. Do not interpolate. Do not "use your judgment" about canonical ABI semantics — the spec is the only judgment that counts.

Your return message must include verbatim quotes from the spec section and the definitions.py function you read. The parent agent will grep the cited files to verify the quotes exist. A return without quotes, or with quotes that do not appear verbatim in the cited files, will be rejected and the work will restart with a fresh subagent.
```

This text is referenced as **THE WARNING TEXT** in subsequent template tasks. Every template includes it verbatim under its `## Spec-overrides-instructions warning` section.

### Task 13: Write templates/reconcile-function.md

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/reconcile-function.md`

- [ ] **Step 1: Write the file**

Content:
````markdown
# reconcile-function.md

You are a fresh subagent dispatched by the Loop 1 parent agent to perform Gate 1 line-by-line reconciliation of one Go function in `internal/component/abi/` against its corresponding Python function in `definitions.py`.

You produce an audit report only. You do **not** write code in this task. If you find bugs, you classify them; the parent will dispatch `fix-reconciliation-finding.md` to fix them in a separate subagent.

## Spec-overrides-instructions warning

**[Insert THE WARNING TEXT verbatim here.]**

## First action

Before reading anything else (including the inputs below):

1. Read `debug-vendored/component-model/design/mvp/CanonicalABI.md` §`{spec_section}` in full.
2. Read `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py` lines `{py_line_range}` (the function `{py_func}`) in full.
3. Read every Python helper function that `{py_func}` transitively calls, until you reach primitives or already-reconciled functions.
4. In your return message, quote one short passage (one sentence or two) from CanonicalABI.md and one short passage from definitions.py. The parent will grep both files to verify the quotes appear verbatim.

## Inputs

- `{go_file}`: e.g., `internal/component/abi/lift.go`
- `{go_func}`: e.g., `LiftFlat`
- `{py_file}`: always `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py`
- `{py_func}`: e.g., `lift_flat`
- `{py_line_range}`: e.g., `142-201`
- `{spec_section}`: e.g., `Flat Lifting`

## Allowed reads

- `internal/component/abi/*.go` (read-only — you need the Go function and any helpers it calls)
- `internal/component/abi/*_test.go` (read-only — for context on existing tests)
- `internal/component/types/**` (read-only — for type definitions)
- `internal/component/val.go` (read-only — for Val constructors)
- `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py` (read-only)
- `debug-vendored/component-model/design/mvp/CanonicalABI.md` (read-only)
- `docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/reconciliation/*.md` (read-only — to check whether helpers have already been reconciled)

You may NOT read any other file. If you need to read something else, file a BLOCKER asking for it to be added to the allowed list.

## Allowed writes

- `docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/reconciliation/{go_func}.md` (the reconciliation report — you Write this)

You may NOT write to any other file. In particular, you may NOT edit any `.go` file. If you find a bug that needs fixing, classify it in the report and let the parent dispatch `fix-reconciliation-finding.md`.

## Procedure

1. Perform the First action above.
2. Read the Go function `{go_func}` in `{go_file}`.
3. Read the Python function `{py_func}` in `definitions.py`.
4. Read the spec section `{spec_section}` in `CanonicalABI.md`.
5. Read every Python helper function transitively called by `{py_func}`. For each helper, check whether it is already reconciled by reading `status/reconciliation/<helper_name>.md`. If yes, you may rely on its reconciled state without re-deriving its content. If no, read it in full.
6. Walk the Python function statement by statement. For each statement (or contiguous logical group):
   a. Find the corresponding statement(s) in the Go function.
   b. Compare branch logic, ordering, constants, types, error handling.
   c. Classify the row.
7. For every classification:
   - **identical**: Go matches Python exactly. No further work.
   - **bug-in-go**: Go diverges from Python without justification. Record the divergence and the spec rule it violates. Cite the spec line.
   - **bug-in-python**: Python appears to diverge from CanonicalABI.md. Record the divergence and quote the spec line that contradicts the Python code. **If you cannot produce a spec quote that contradicts the Python, you must NOT classify as bug-in-python; either find one or classify as identical/intentional-deviation, or file a BLOCKER.**
   - **intentional-deviation**: Go diverges from Python, with a spec citation justifying the divergence (e.g., spec allows multiple correct implementations and Go picks one). You must quote the spec text justifying the choice.
8. Write the reconciliation report to `status/reconciliation/{go_func}.md` in the format below.
9. End the report with the line: `RECONCILED <YYYY-MM-DD> — function matches definitions.py and CanonicalABI.md.` if and only if every row is classified `identical` or `intentional-deviation`. If any row is `bug-in-go` or `bug-in-python`, end with `PENDING — see findings above.` instead.
10. Return the report content and the classification summary in your return message per the Return format below.

## Halt conditions

File a BLOCKER and halt if:
- The Python function references a spec construct (e.g., async, stream, future) that is out of scope per spec §2.2. (Async functions should not be reconciled by this template; this is a Loop 1 enumeration mistake. Halt with a BLOCKER asking the parent to install an async stub instead.)
- You find a `bug-in-python` candidate but cannot produce a spec quote contradicting the Python.
- The Go function references a Go type, constant, or helper you cannot find. (This indicates a stale function or a missing dependency that needs human inspection.)
- The spec section `{spec_section}` does not exist in the vendored `CanonicalABI.md`. (Loop 1 enumeration mistake.)
- You find that a helper called by `{py_func}` has its own bug-in-go that affects `{py_func}` but is not listed as a transitive dependency. Record the helper bug in your report and file a BLOCKER asking the parent to fix the helper first.

## Return format

Return a JSON object:

```json
{
  "subagent_id": "<your-id>",
  "status": "complete" | "blocker",
  "go_func": "{go_func}",
  "report_path": "status/reconciliation/{go_func}.md",
  "row_count": <int>,
  "classifications": {
    "identical": <int>,
    "bug-in-go": <int>,
    "bug-in-python": <int>,
    "intentional-deviation": <int>
  },
  "first_bug_in_go": null | {
    "row_index": <int>,
    "go_line": <int>,
    "py_line": <int>,
    "summary": "<one-line>",
    "spec_quote": "<verbatim spec text>"
  },
  "first_action_quotes": {
    "spec_quote": "<verbatim CanonicalABI.md text>",
    "spec_quote_source": "CanonicalABI.md §{spec_section}",
    "py_quote": "<verbatim definitions.py text>",
    "py_quote_source": "definitions.py:{py_func}"
  },
  "blocker_reason": null | "<reason if status=blocker>"
}
```

The reconciliation report at `status/reconciliation/{go_func}.md` has the format:

```markdown
# Reconciliation: {go_func}

- Go file: {go_file}
- Python file: definitions.py
- Python function: {py_func} (lines {py_line_range})
- Spec section: CanonicalABI.md §{spec_section}
- Reconciled by: <subagent-id>
- Reconciled at: <ISO timestamp>

## Side-by-side

| # | Python (line) | Go (line) | Classification | Notes |
|---|---|---|---|---|
| 1 | ... | ... | identical | |
| 2 | ... | ... | bug-in-go | go reads 4 bytes, py reads 1 — spec line N says 1 |
| ... | ... | ... | ... | ... |

## Spec citations referenced

- CanonicalABI.md §{spec_section}: "<verbatim quote>"
- (additional quotes for any classification that needed one)

RECONCILED <date> — function matches definitions.py and CanonicalABI.md.
```

## Self-check

Before returning, answer truthfully:

1. Did I read CanonicalABI.md §{spec_section} end-to-end? **(yes/no)**
2. Did I read definitions.py:{py_func} end-to-end, plus all transitive helpers? **(yes/no)**
3. Did I produce a row in the side-by-side table for every Python statement (no Python statement skipped)? **(yes/no)**
4. Did I classify every row? **(yes/no)**
5. For every `bug-in-python` row, do I have a verbatim spec quote that contradicts the Python? **(yes/no — if no, halt with BLOCKER)**
6. Did I include the verbatim spec and Python quotes in the return message? **(yes/no)**
7. Did the parent's grep verification of my quotes succeed? **(can't know — but the parent will reject your return if not)**

If any answer is "no", you must not declare the report complete. Either fix the issue or file a BLOCKER.

## After this template runs

This template does not produce a code change (it produces an audit artifact only). The parent does NOT run a review chain after this template. Instead:

- If `first_bug_in_go` is non-null, the parent will dispatch `templates/fix-reconciliation-finding.md` (which DOES produce a code change and DOES trigger the review chain).
- If `first_bug_in_go` is null and `bug-in-python` count is zero, Gate 1 is pass for this function and the parent moves to Gate 2.
````

- [ ] **Step 2: Replace `[Insert THE WARNING TEXT verbatim here.]` with the actual canonical text from spec §7.5**

Open the file and replace the placeholder with the verbatim text. Verify with:
```bash
grep -c "Insert THE WARNING TEXT" docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/reconcile-function.md
```
Expected: `0`.

- [ ] **Step 3: Verify the file**

Run:
```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/reconcile-function.md
```
Expected: line count > 100.

### Task 14: Write templates/fix-reconciliation-finding.md

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/fix-reconciliation-finding.md`

- [ ] **Step 1: Write the file**

Same canonical structure (10 sections plus `## After this template runs` since this template DOES produce a code change). Key content:

- Title: `# fix-reconciliation-finding.md`
- Inputs: `{finding_row_index}`, `{go_file}`, `{go_func}`, `{go_line}`, `{py_func}`, `{py_line}`, `{summary}`, `{spec_quote}`, `{report_path}`.
- Allowed reads: same as `reconcile-function.md` plus `{report_path}`.
- Allowed writes: `internal/component/abi/<file>.go` (only the file containing `{go_func}`).
- Procedure:
  1. Perform First action (read spec section, read python function, quote in return).
  2. Read the reconciliation report at `{report_path}` to get full context for the finding.
  3. Read the Go function in full.
  4. Read the corresponding Python function in full.
  5. Identify the **minimum** Go change that makes the Go behavior match the Python behavior at this row. The change must NOT touch unrelated parts of the Go function. Scope creep is prohibited.
  6. Apply the change via Edit.
  7. Run `go build ./internal/component/abi/...` to confirm the change compiles. If it does not, the change is wrong; fix and re-run.
  8. Run `go test ./internal/component/abi/...` to confirm no existing test is broken by the change. If a test fails, the change is wrong (the existing test already encoded the correct behavior — but wait, Loop 1 also audits existing tests; if the existing test encoded the wazero bug, the test will be fixed by the conformance audit). If the failing test was not yet audited, file a BLOCKER asking the parent to audit it before this fix proceeds.
- Halt conditions: build fails after fix; existing test fails after fix and the test has not been audited; the change you would need to make extends beyond the single function `{go_func}`.
- Return format: JSON with `subagent_id`, `status`, `diff_summary` (brief), `files_changed` (list), `build_passed` (bool), `tests_in_abi_passed` (bool), `first_action_quotes` (same form as reconcile).
- After this template runs: **CODE CHANGE — parent runs the full R1 → R3 → R5 review chain before committing.**

- [ ] **Step 2: Replace WARNING TEXT placeholder**

Verify with:
```bash
grep -c "Insert THE WARNING TEXT" docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/fix-reconciliation-finding.md
```
Expected: `0`.

- [ ] **Step 3: Verify**

Run:
```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/fix-reconciliation-finding.md
```
Expected: line count > 80.

### Task 15: Write templates/port-spec-test.md

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/port-spec-test.md`

- [ ] **Step 1: Write the file**

Canonical structure. Key content:

- Inputs: `{py_file: run_tests.py}`, `{py_func}`, `{py_line_range}`, `{go_dest_file}`.
- Allowed reads: `debug-vendored/component-model/design/mvp/canonical-abi/run_tests.py`, `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py`, `debug-vendored/component-model/design/mvp/CanonicalABI.md`, `internal/component/abi/*.go`, `internal/component/conformance/*.go`, `internal/component/types/**`, `internal/component/val.go`.
- Allowed writes: `{go_dest_file}` only.
- Procedure:
  1. First action: read run_tests.py:{py_func} in full, read CanonicalABI.md section relevant to the test, return verbatim quotes.
  2. Identify every `test(...)` call (and `test_string(...)`, `test_heap(...)`, `test_flatten(...)`, etc. as appropriate) in the Python function. Each call becomes one Go subtest.
  3. Identify the data tables (lists, dicts) the Python function iterates over. Each row becomes one entry in a Go table-driven test.
  4. Write the Go test file. Use the form:
     ```go
     // Spec port of run_tests.py:<py_func> (lines <start>-<end>).
     // See debug-vendored/component-model/design/mvp/canonical-abi/run_tests.py.

     package conformance

     import (
         "testing"
         "<wazero abi import>"
     )

     func TestSpec<Name>(t *testing.T) {
         tests := []struct {
             name string
             // fields mirroring the Python tuple/dict
         }{
             {name: "<row1>", ...},
             // ...
         }
         for _, tt := range tests {
             t.Run(tt.name, func(t *testing.T) {
                 // body mirroring the Python test() helper
             })
         }
     }
     ```
  5. Run `go build ./internal/component/conformance/...` to confirm it compiles.
  6. Run `go test -v -run TestSpec<Name> ./internal/component/conformance/` to capture initial pass/fail state. The test does not need to pass at this stage — failing subtests will be addressed by Gate 2's fix-reconciliation-finding loop. The point of this template is to PORT the test, not to make it pass.
- Halt conditions: cannot identify how the Python data tables map to Go (file BLOCKER); cannot find the corresponding wazero abi/ function being tested.
- Return format: JSON with `subagent_id`, `status`, `go_dest_file`, `subtest_count`, `compile_passed` (bool), `initial_pass_count` (int), `initial_fail_count` (int), `first_action_quotes`.
- After this template runs: **CODE CHANGE — review chain.**

- [ ] **Step 2: Replace WARNING TEXT placeholder**

- [ ] **Step 3: Verify**

```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/port-spec-test.md
```
Expected > 80.

### Task 16: Write templates/run-python-differential.md

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/run-python-differential.md`

- [ ] **Step 1: Write the file**

This template has TWO modes selected by the parent: (a) initial creation of the Go differential test file, (b) writing the Python harness ops needed by Gate 3 for a function whose ops are not yet implemented in `spec_diff_driver.py`.

- Inputs: `{mode}` (one of `create-go-test`, `extend-driver`), `{go_func}`, `{spec_section}`.
- Allowed reads: harness/, conformance/, abi/, run_tests.py, definitions.py.
- Allowed writes: `internal/component/conformance/spec_diff_test.go` (mode=create-go-test) or `docs/superpowers/projects/2026-04-07-canonical-abi-unification/harness/spec_diff_driver.py` (mode=extend-driver).
- Procedure (mode=create-go-test):
  1. First action.
  2. Write the Go differential test file. The test sets up a Python subprocess via `os/exec`, communicates over stdin/stdout, and asserts byte-for-byte equality between abi/ outputs and Python outputs for inputs sourced from the same data tables used by the spec_*_test.go ports.
  3. Compile and confirm the test framework starts the subprocess successfully.
- Procedure (mode=extend-driver):
  1. First action.
  2. Read the existing `spec_diff_driver.py`.
  3. Add the missing op handler (e.g., `op_lift_heap`, `op_store_record`) by mirroring the corresponding `definitions.py` function. The op handler must NOT alter inputs and must NOT lossy-serialize outputs.
  4. Add the new op to the `OPS` dict.
  5. Run the harness's pytest suite to confirm existing tests still pass.
- Both modes are CODE CHANGES → review chain after.
- Return format includes `mode`, `files_changed`, `compile_or_test_passed`, `first_action_quotes`.

- [ ] **Step 2: Replace WARNING TEXT placeholder**

- [ ] **Step 3: Verify**

```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/run-python-differential.md
```
Expected > 80.

### Task 17: Write templates/run-wasmtime-conformance.md

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/run-wasmtime-conformance.md`

- [ ] **Step 1: Write the file**

Two modes: (a) initial creation of the Go wasmtime differential test file, (b) running it for a specific fixture and reporting the result.

- Mode `create-go-test`: Write `internal/component/conformance/spec_wasmtime_diff_test.go`. The test enumerates fixtures from `wasmtime-fixture-corpus.json` (which lives in the project status dir, so the test reads it at runtime via a hardcoded path or build-time generation — pick one and document it). For each non-skipped fixture, the test runs `wasmtime` via `os/exec` and runs wazero in-process via the public API, captures observable outputs, asserts equivalence. If `wasmtime` is not on PATH, the entire test calls `t.Skip("wasmtime CLI not installed; install via curl https://wasmtime.dev/install.sh -sSf | bash")`.
- Mode `run-fixture`: Run a specific fixture and report the result. No code change in this mode.

- Allowed reads: conformance/, testdata/, debug-vendored/, abi/.
- Allowed writes: `internal/component/conformance/spec_wasmtime_diff_test.go` (mode=create-go-test only).
- Halt conditions: cannot find the corpus file; cannot find a normalizer for fixture-injected nondeterminism; the fixture's expected behavior per `definitions.py` cannot be determined.
- Return: JSON with mode-specific fields.

Mode `create-go-test` is a CODE CHANGE → review chain. Mode `run-fixture` is not.

- [ ] **Step 2: Replace WARNING TEXT placeholder**

- [ ] **Step 3: Verify**

```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/run-wasmtime-conformance.md
```
Expected > 80.

### Task 18: Write templates/diagnose-wasmtime-divergence.md

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/diagnose-wasmtime-divergence.md`

- [ ] **Step 1: Write the file**

Audit-only template. No code change.

- Inputs: `{fixture_path}`, `{wazero_output}`, `{wasmtime_output}`, `{spec_section}`.
- Allowed reads: harness/, conformance/, abi/, debug-vendored/, definitions.py.
- Allowed writes: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/wasmtime-divergence/<fixture-slug>-<timestamp>.md` (a divergence record).
- Procedure:
  1. First action.
  2. Read the spec section relevant to the fixture's exercise.
  3. Run the same input through `spec_diff_driver.py` (if applicable — the fixture may exercise high-level ops not yet supported by the driver).
  4. Compare three outputs: wazero, wasmtime, definitions.py.
  5. Decide which side is wrong:
     - If wazero ≠ definitions.py and wasmtime = definitions.py → wazero bug, reopen Gate 1.
     - If wazero = definitions.py and wasmtime ≠ definitions.py → wasmtime version mismatch or wasmtime bug. File a BLOCKER asking the user to verify the installed wasmtime version against the vendored spec version.
     - If wazero ≠ definitions.py and wasmtime ≠ definitions.py → both wrong. File a BLOCKER for human inspection.
     - If all three agree → the fixture's expected behavior is wrong. Audit the fixture and the corpus enumeration.
  6. Write the divergence record.
- Return format: JSON with verdict and reasoning.
- Audit artifact only — no review chain.

- [ ] **Step 2: Replace WARNING TEXT placeholder**

- [ ] **Step 3: Verify**

```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/diagnose-wasmtime-divergence.md
```
Expected > 60.

### Task 19: Write templates/build-public-api-exercise.md

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/build-public-api-exercise.md`

- [ ] **Step 1: Write the file**

This template builds a Gate 6 public-API exercise: a Go test that uses ONLY the wazero public API to load a real .wasm fixture and invoke an exported function whose canonical-ABI semantics exercise the target abi/ function.

- Inputs: `{abi_function}`, `{spec_section}`.
- Allowed reads: conformance/, testdata/, testdata/gen/, debug-vendored/wit-bindgen/tests/runtime/, debug-vendored/component-model/test/, abi/, the wazero public API surface (api/, wazero.go), definitions.py for understanding what input the function needs.
- Allowed writes: `internal/component/conformance/spec_publicapi_test.go`, `internal/component/testdata/<new>.wasm` (only when building a new fixture), `internal/component/testdata/gen/<new>_gen.go` (or `.rs`+`Cargo.toml` for Rust fixtures), `docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/loop1-publicapi-coverage.json`.
- Procedure:
  1. First action.
  2. Determine which existing fixture in `internal/component/testdata/` or `debug-vendored/wit-bindgen/tests/runtime/` exercises the target function. If one exists, use it. Otherwise, build a new minimal fixture.
  3. To build a new fixture, write the source (Go via TinyGo, or Rust via cargo-component) under `testdata/gen/`, build it to `.wasm`, commit both source and .wasm.
  4. Write the test in `spec_publicapi_test.go` (or extend the existing file). The test:
     - Constructs a `wazero.Runtime` via the public API.
     - Loads the fixture.
     - Instantiates the component via the public component API (`runtime.NewComponent` or whatever the current public surface exposes).
     - Invokes an exported function via `ComponentFunc.Call(...)`.
     - Asserts the return value matches the expected canonical-ABI behavior.
     - Imports ONLY the wazero public package (`github.com/tetratelabs/wazero` and friends). MUST NOT import `internal/component/abi`.
  5. Run `go test -v -run TestPublicAPI_<function> ./internal/component/conformance/` to confirm pass.
  6. Update `loop1-publicapi-coverage.json[<function>]` with the fixture path and test name.
- Halt conditions: no existing fixture exercises the function and a new fixture cannot be built (toolchain unavailable); the public API does not yet expose a path to invoke the function (would require API changes — file a BLOCKER).
- Return format: JSON with `subagent_id`, `status`, `function`, `fixture_path`, `fixture_built` (bool — true if newly built), `test_name`, `test_passed` (bool), `files_changed` (multi-file list), `first_action_quotes`.
- **CODE CHANGE — multi-file but counts as ONE chain unit per spec §5.5.**

- [ ] **Step 2: Replace WARNING TEXT placeholder**

- [ ] **Step 3: Verify**

```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/build-public-api-exercise.md
```
Expected > 100.

### Task 20: Write templates/add-spec-citation.md

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/add-spec-citation.md`

- [ ] **Step 1: Write the file**

- Inputs: `{go_file}`, `{go_func}`, `{spec_section}`, `{py_func}`, `{py_line_range}`.
- Allowed reads: abi/, debug-vendored/component-model/design/mvp/CanonicalABI.md, definitions.py.
- Allowed writes: `{go_file}` only.
- Procedure:
  1. First action.
  2. Read the Go function.
  3. Identify nontrivial branches (any `if`/`switch` case whose condition is not a simple type-switch over `ValType`).
  4. Add the doc comment in the form:
     ```go
     // {go_func} implements canon {spec_op} from CanonicalABI.md §{spec_section}.
     //
     // Reference implementation: definitions.py:{py_func} (lines {py_line_range}).
     // Reconciled <today>—see status/reconciliation/{go_func}.md.
     ```
  5. For each nontrivial branch, add an end-of-line or preceding-line comment `// Spec: §{spec_section}` or `// Spec: definitions.py:<line>`. Do NOT add comments to trivial type-switch cases — the type switch itself is the citation.
  6. Run `go build ./internal/component/abi/...` to confirm the file still compiles.
- Halt conditions: cannot determine which spec section to cite; the function appears to span multiple spec sections (file BLOCKER for clarification).
- Return format: JSON.
- **CODE CHANGE — review chain.**

- [ ] **Step 2: Replace WARNING TEXT placeholder**

- [ ] **Step 3: Verify**

```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/add-spec-citation.md
```
Expected > 60.

### Task 21: Write templates/audit-existing-conformance-test.md

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/audit-existing-conformance-test.md`

- [ ] **Step 1: Write the file**

- Inputs: `{file_path}`, `{spec_section}` (best-guess section the file appears to test).
- Allowed reads: conformance/, abi/, definitions.py, CanonicalABI.md.
- Allowed writes: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/audit/<file-basename>.md` (audit report) AND, if the audit comes back clean, the input file itself for the purpose of adding the `// Audited <date>` header.
- Procedure:
  1. First action.
  2. Read the entire test file.
  3. For each test assertion, identify the input, the asserted output, and the abi/ function being exercised.
  4. For each assertion, determine what `definitions.py` would return for the same input. Check whether the asserted output matches `definitions.py`'s behavior.
  5. Classify each assertion as:
     - `correct`: matches definitions.py.
     - `wazero-bug-encoded`: asserts the wazero bug, not the spec behavior. Needs a fix.
     - `unclear`: cannot determine which behavior is correct (file BLOCKER).
  6. Write the audit report.
  7. If every assertion is `correct`, edit the file to add the header comment at the top:
     ```go
     // Audited <YYYY-MM-DD> against definitions.py — assertions match canonical reference.
     ```
     (placed after the package declaration, before imports).
- Halt conditions: any assertion is `unclear` and the BLOCKER cannot be filed (which shouldn't happen — always file the BLOCKER).
- Return format: JSON with `subagent_id`, `status`, `file_path`, `assertion_count`, `correct_count`, `wazero_bug_encoded_count`, `unclear_count`, `header_added` (bool — only if all correct).
- If header was added: **CODE CHANGE — review chain.** If only an audit report was produced (no header): no review chain; the parent dispatches `templates/fix-conformance-audit-finding.md` for each `wazero-bug-encoded` assertion.

- [ ] **Step 2: Replace WARNING TEXT placeholder**

- [ ] **Step 3: Verify**

```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/audit-existing-conformance-test.md
```
Expected > 80.

### Task 22: Write templates/fix-conformance-audit-finding.md

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/fix-conformance-audit-finding.md`

- [ ] **Step 1: Write the file**

This template was implied by `audit-existing-conformance-test.md` but not previously enumerated. It fixes one `wazero-bug-encoded` assertion in an existing conformance test by replacing the assertion with the spec-correct expected value.

- Inputs: `{file_path}`, `{assertion_index}`, `{audit_report_path}`, `{spec_quote}`.
- Allowed reads: same as audit template plus the audit report.
- Allowed writes: `{file_path}` only.
- Procedure:
  1. First action.
  2. Read the audit report and identify the specific assertion to fix.
  3. Read the test file and locate the assertion.
  4. Determine the spec-correct expected value by reading `definitions.py`.
  5. Replace the assertion with the spec-correct expected value. Add a comment citing the spec quote.
  6. Run `go build ./internal/component/conformance/...` to confirm it compiles.
- **CODE CHANGE — review chain.**

- [ ] **Step 2: Replace WARNING TEXT placeholder**

- [ ] **Step 3: Verify**

```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/fix-conformance-audit-finding.md
```
Expected > 50.

### Task 23: Write templates/install-async-stub.md

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/install-async-stub.md`

- [ ] **Step 1: Write the file**

- Inputs: `{go_file}`, `{go_func}`, `{spec_section}`, `{missing_primitive}`.
- Allowed reads: abi/, CanonicalABI.md.
- Allowed writes: `{go_file}` only.
- Procedure:
  1. First action.
  2. Confirm the function is genuinely an async entry point (cross-check against `loop1-async-stubs.json`).
  3. Replace the function body with:
     ```go
     return fmt.Errorf("canonical ABI: %s requires %s, defined in CanonicalABI.md §%s, not implemented in wazero", "{go_func}", "{missing_primitive}", "{spec_section}")
     ```
     The exact form is per spec §3.2: "fmt.Errorf with explicit citation, no panics, no silent no-ops, no placeholder values."
  4. If the function returns multiple values, return zero values for the others alongside the error.
  5. Run `go build ./internal/component/abi/...` to confirm.
- Halt conditions: function signature does not return an error (file BLOCKER asking for the signature to be reconciled first); function is not actually async (Loop 1 enumeration mistake).
- **CODE CHANGE — review chain.**

- [ ] **Step 2: Replace WARNING TEXT placeholder**

- [ ] **Step 3: Verify**

```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/install-async-stub.md
```
Expected > 50.

### Task 24: Write templates/enumerate-functions.md

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/enumerate-functions.md`

- [ ] **Step 1: Write the file**

- Inputs: none (this is a one-shot bootstrap template).
- Allowed reads: definitions.py, CanonicalABI.md, abi/, harness/spec_diff_driver.py.
- Allowed writes: `status/loop1-functions.json`, `status/loop1-async-stubs.json`.
- Procedure:
  1. First action.
  2. Run `python3 docs/superpowers/projects/2026-04-07-canonical-abi-unification/harness/spec_diff_driver.py --list-sync-functions` to get the canonical sync function list.
  3. Read `internal/component/abi/*.go` and enumerate every public function in the package.
  4. For each Python sync function:
     - Find the corresponding Go function (by name match or by spec section). If absent, file a BLOCKER asking for the Go function to be added before reconciliation can begin.
     - Determine the `py_line_range` by grepping `definitions.py`.
     - Determine the `spec_section` by reading the function's location in `CanonicalABI.md`.
     - Add an entry to `loop1-functions.json[functions]` with `iteration_status: "pending"` and all gates pending.
  5. For each Go async function (functions whose names match patterns like `*Stream*`, `*Future*`, `*ErrorContext*`, `*Thread*`, `*Waitable*`, `*Subtask*`, `Task*`, etc., AND functions present in abi/ but absent from the sync list):
     - Determine the `missing_primitive` by reading the spec.
     - Add an entry to `loop1-async-stubs.json[stubs]` with `stub_status: "pending"`.
  6. Atomically write both files.
- Halt conditions: spec_diff_driver.py is not invokable; abi/ is empty; a Go function cannot be classified as either sync-reconcilable or async-stubable.
- **NOT a code change** (only writes status files, which are audit artifacts) → no review chain. The next bootstrap template runs immediately.

- [ ] **Step 2: Replace WARNING TEXT placeholder**

- [ ] **Step 3: Verify**

```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/enumerate-functions.md
```
Expected > 60.

### Task 25: Write templates/enumerate-wasmtime-fixtures.md

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/enumerate-wasmtime-fixtures.md`

- [ ] **Step 1: Write the file**

- Inputs: none.
- Allowed reads: debug-vendored/component-model/test/, debug-vendored/wit-bindgen/tests/runtime/, debug-vendored/wasmtime/tests/disas/component-model/, internal/component/wasip2test/.
- Allowed writes: `status/wasmtime-fixture-corpus.json`.
- Procedure:
  1. First action.
  2. Run `wasmtime --version` and capture the version. Write it to `wasmtime_version_required` in the corpus file (the version that was installed at corpus capture time).
  3. Walk the allowed read directories and enumerate every `.wasm` and `.wat` file.
  4. For each fixture, determine:
     - Which exports it has (use `wasm-tools component wit <fixture>` if available, or read accompanying README/test files).
     - Which abi/ functions it exercises (infer from the WIT types in the exports).
     - Whether it should be skipped per spec §5.4 allowed-skip categories (async/stream/future/thread/error-context, Loop 3 wasip2 territory).
  5. Write the corpus file.
- Halt conditions: wasmtime is not installed (file BLOCKER with the install command); a fixture's exports cannot be determined.
- **NOT a code change** → no review chain.

- [ ] **Step 2: Replace WARNING TEXT placeholder**

- [ ] **Step 3: Verify**

```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/enumerate-wasmtime-fixtures.md
```
Expected > 60.

---

## Phase 6 — Loop 2 templates

### Task 26: Write templates/enumerate-callsites.md

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/enumerate-callsites.md`

- [ ] **Step 1: Write the file**

- Inputs: none.
- Allowed reads: `internal/component/canon_lower.go`, `internal/component/component_linker.go`, `internal/component/instance.go`, `internal/component/resource_table.go`, `internal/component/abi/*.go` (for the API surface), the spec doc.
- Allowed writes: `status/loop2-callsites.json`.
- Procedure:
  1. First action.
  2. Read the three runtime files (canon_lower.go, component_linker.go, instance.go) and resource_table.go.
  3. Identify every function or method that performs canonical-ABI work directly. Use spec §3.3 as the starting list (it names every helper that should be deleted) and grep for additional helpers that match the same pattern.
  4. For each callsite, populate `callsites[<id>]` with file, function, line range, what abi/ function will replace it, and `migration_status: "pending"`.
  5. Populate `deletion_targets` with every helper symbol that will be deleted at the end of Loop 2.
  6. Cross-check against spec §3.3 and §9.4 P-T2 — every named symbol in those sections must appear in `deletion_targets`. If any are missing, file a BLOCKER. If any extra symbols are found, ADD them to the list (the spec accepted that the design-time list may be incomplete).
- **NOT a code change** → no review chain.

- [ ] **Step 2: Replace WARNING TEXT placeholder**

- [ ] **Step 3: Verify**

```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/enumerate-callsites.md
```
Expected > 60.

### Task 27: Write templates/capture-loop2-baseline.md (also used for loop3 baseline)

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/capture-loop2-baseline.md`

The file is named `capture-loop2-baseline.md` for historical reasons (Loop 2 was the first loop to need a baseline). It is parameterized so Loop 3 reuses it. Do NOT create a separate `capture-loop3-baseline.md` — the same template handles both loops via the `{baseline_file}` and `{loop_number}` inputs.

- [ ] **Step 1: Write the file**

- Inputs:
  - `{baseline_file}`: e.g., `status/loop2-baseline.json` or `status/loop3-baseline.json`. The parent specifies which loop's baseline is being captured.
  - `{loop_number}`: `2` or `3`.
- Allowed reads: full Go test suite (the subagent runs `go test`, not Read).
- Allowed writes: `{baseline_file}` only.
- Procedure:
  1. First action (read the spec sections relevant to Rule L2-B / L3-B regression checking — same protocol for both).
  2. Run `go test ./... 2>&1 | tee /tmp/loop{loop_number}-baseline.txt`. Capture stdout+stderr.
  3. Parse the output to count `total_tests`, `passing`, `failing`, and the list of failing test names.
  4. Write `{baseline_file}` with the results, an ISO timestamp, and `loop: {loop_number}`.
- Halt conditions: `go test ./...` does not complete (compile error somewhere — file BLOCKER asking for the compile error to be fixed first; that fix is itself a code change that must go through Loop 1 or be handled outside this project).
- **NOT a code change** → no review chain. (The baseline file is a status file, an audit artifact.)

- [ ] **Step 2: Replace WARNING TEXT placeholder**

- [ ] **Step 3: Verify**

```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/capture-loop2-baseline.md
```
Expected > 50.

### Task 28: Write templates/migrate-callsite.md

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/migrate-callsite.md`

- [ ] **Step 1: Write the file**

- Inputs: `{callsite_id}`, `{file}`, `{function}`, `{lines}`, `{calls_into}`, `{replaces_with}`.
- Allowed reads: `internal/component/canon_lower.go`, `internal/component/component_linker.go`, `internal/component/instance.go`, `internal/component/abi/*.go` (read-only — to know the API), `internal/component/types/**`, `internal/component/val.go`, the relevant `definitions.py` function and `CanonicalABI.md` section (for context, NOT for fixing abi/).
- Allowed writes: `{file}` only.
- Procedure:
  1. First action (read spec/python — context only; do not modify).
  2. Read the callsite function in full.
  3. Identify the exact call(s) to in-file lift/lower helpers within `{function}`. These are what will be replaced.
  4. Construct an `abi.LiftContext` (or `abi.LowerContext`) using the wazero `Instance` data available at the call site.
  5. Replace the in-file helper calls with the corresponding `abi/` calls.
  6. Run `go build ./internal/component/...` to confirm compile.
  7. Run `go test ./... 2>&1 | tee /tmp/loop2-migration-<callsite_id>.txt`. Parse failure count.
  8. Compare to `loop2-baseline.json`:
     - If failure count strictly less: success.
     - If equal: success (callsite was already producing correct behavior).
     - If strictly greater OR a previously-passing test now fails: REGRESSION.
  9. If REGRESSION:
     - Run `git diff` to confirm the change.
     - Run `git checkout -- {file}` to revert (do NOT commit the broken change).
     - Return status `regression` with the failing test names.
     - Do NOT proceed to Step 10.
  10. If success, return status `complete` with the migration diff summary.
- Halt conditions:
  - The replacement requires changes to `internal/component/abi/` (Rule L2-A violation — file BLOCKER, parent reopens Loop 1).
  - The replacement requires changes to a file outside `{file}` (file BLOCKER, parent re-evaluates the callsite enumeration).
  - Compilation fails after the change.
- Return format: JSON with `status`, `regression` (bool), `failing_tests_added` (list), `failing_tests_removed` (list), `compile_passed` (bool), `first_action_quotes`.
- **CODE CHANGE — review chain. The chain runs only if `regression=false`. If regression, the change is reverted and the parent dispatches `templates/diagnose-loop2-regression.md`.**

- [ ] **Step 2: Replace WARNING TEXT placeholder**

- [ ] **Step 3: Verify**

```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/migrate-callsite.md
```
Expected > 100.

### Task 29: Write templates/add-wiring-test.md

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/add-wiring-test.md`

- [ ] **Step 1: Write the file**

- Inputs: `{callsite_id}`, `{migrated_function}`, `{abi_function}`.
- Allowed reads: all internal/component/ files except internal/component/abi/, internal/component/wasip2test/, internal/component/testdata/, the wazero public API, debug-vendored/wit-bindgen/tests/runtime/.
- Allowed writes: `internal/component/canon_lower_test.go`, `internal/component/component_linker_test.go`, `internal/component/instance_test.go`, `internal/component/wasip2test/*_test.go` (whichever is appropriate to the migrated callsite). May also write to `internal/component/testdata/` and `internal/component/testdata/gen/` if a new fixture is needed (same multi-file rule as Gate 6 in Loop 1).
- Procedure:
  1. First action.
  2. Pick (or build) a real wasm fixture that exercises the migrated callsite via the public wazero API.
  3. Write a Go test in the appropriate `_test.go` file that loads the fixture, instantiates it, invokes the relevant export, and asserts observable wiring outcomes (return value, trap, memory bytes, resource handle).
  4. The test MUST use the public wazero API. MUST NOT import `internal/component/abi`. MUST NOT import any other internal package directly.
  5. Run the test. Confirm pass.
- Halt conditions: no fixture exercises the callsite and a new fixture cannot be built.
- **CODE CHANGE — review chain.**

- [ ] **Step 2: Replace WARNING TEXT placeholder**

- [ ] **Step 3: Verify**

```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/add-wiring-test.md
```
Expected > 80.

### Task 30: Write templates/audit-existing-wiring-test.md

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/audit-existing-wiring-test.md`

- [ ] **Step 1: Write the file**

Mirror of `audit-existing-conformance-test.md` but for wiring-layer tests under `internal/component/wasip2test/`, `internal/component/canon_lower_test.go`, `internal/component/component_linker_test.go`, `internal/component/instance_test.go`. Same audit-then-fix loop.

- Inputs: `{file_path}`.
- Allowed reads: all internal/component/ files except abi/ (read-only on canon_lower.go, component_linker.go, instance.go, resource_table.go), wasip2test/, definitions.py, CanonicalABI.md.
- Allowed writes: `status/audit/<file-basename>.md`, and the input file itself if all assertions audit clean (for the header).
- Procedure: same as audit-existing-conformance-test.md with the audit context shifted from canonical-ABI semantics (Loop 1) to wiring outcomes (Loop 2).

- [ ] **Step 2: Replace WARNING TEXT placeholder**

- [ ] **Step 3: Verify**

```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/audit-existing-wiring-test.md
```
Expected > 70.

### Task 31: Write templates/diagnose-loop2-regression.md

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/diagnose-loop2-regression.md`

- [ ] **Step 1: Write the file**

Audit-only template. Triggered when `migrate-callsite.md` returns `regression`.

- Inputs: `{callsite_id}`, `{failing_tests_added}` (list).
- Allowed reads: same as `migrate-callsite.md`.
- Allowed writes: `status/loop2-regressions/<callsite_id>-<timestamp>.md`.
- Procedure:
  1. First action.
  2. Read the diff that was reverted (it's in /tmp from `migrate-callsite.md`'s logs, or re-derive by reading the file at the previous commit).
  3. For each failing test, identify why it now fails. Categorize:
     - **abi/ context construction wrong**: the migration built an `abi.LiftContext`/`abi.LowerContext` with wrong fields. Recommend: re-do the migration with correct context construction.
     - **abi/ function bug**: abi/ produces wrong output for the input the test exercises. Recommend: REOPEN LOOP 1 (Rule L2-A — Loop 2 cannot fix abi/). File BLOCKER.
     - **test was relying on the runtime bug**: the test's expected value matches the buggy runtime, not the spec. Recommend: AUDIT the test via `audit-existing-wiring-test.md`.
     - **calling convention mismatch**: the migration called abi/ with the wrong signature. Recommend: re-do the migration with correct signature.
  4. Write the regression report.
- Audit only — no code change, no review chain.

- [ ] **Step 2: Replace WARNING TEXT placeholder**

- [ ] **Step 3: Verify**

```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/diagnose-loop2-regression.md
```
Expected > 60.

### Task 32: Write templates/delete-dead-helper.md

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/delete-dead-helper.md`

- [ ] **Step 1: Write the file**

- Inputs: `{symbol_name}`, `{file_path}` (where the symbol is defined).
- Allowed reads: every Go file in the tree (read-only — needs to grep for references).
- Allowed writes: `{file_path}` only.
- Procedure:
  1. First action.
  2. Run `templates/verify-grep-zero.md` (via the parent — this template is dispatched AFTER the parent has verified zero references). If the parent did not verify, this template halts with BLOCKER.
  3. Read `{file_path}` and locate the symbol definition.
  4. Delete the symbol definition. If the symbol is the only thing in the file, delete the file.
  5. Run `go build ./...` to confirm the deletion does not break anything. If it does, the verify-grep-zero check missed something — halt with BLOCKER.
- **CODE CHANGE — review chain.** The deletion is its own commit, separate from the migration commit that orphaned the symbol (per Rule L2-C).

- [ ] **Step 2: Replace WARNING TEXT placeholder**

- [ ] **Step 3: Verify**

```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/delete-dead-helper.md
```
Expected > 60.

---

## Phase 7 — Loop 3 templates

### Task 33: Write templates/enumerate-wasip2-suppression-sites.md

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/enumerate-wasip2-suppression-sites.md`

- [ ] **Step 1: Write the file**

- Inputs: none.
- Allowed reads: `imports/wasip2/sockets/tcp.go`, `imports/wasip2/sockets/udp.go`, `imports/wasip2/http/http.go`, `imports/wasip2/http/types.go`, `imports/wasip2/http/incoming.go`, `imports/wasip2/http/outgoing.go`, the spec doc.
- Allowed writes: `status/loop3-suppressed-errors.json`.
- Procedure:
  1. First action (read spec §1.4 for the suppression pattern).
  2. Grep each allowed-read file for the pattern: a call to a `get*` helper followed by `if err != nil { return ... }` returning a placeholder success value (`ValResultOk(...)`, `ValBool(false)`, `ValOption(nil)`, `ValOwn(0)`, `ValU8(...)`, `ValList([])`, etc.).
  3. For each match, record file, function, line range. Add to `loop3-suppressed-errors.json[sites]`.
  4. Cross-check the count against the audit's claim of ~77 sites. If significantly different, document the discrepancy in the return.
- **NOT a code change** → no review chain.

- [ ] **Step 2: Replace WARNING TEXT placeholder**

- [ ] **Step 3: Verify**

```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/enumerate-wasip2-suppression-sites.md
```
Expected > 60.

### Task 34: Write templates/fix-error-suppression.md

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/fix-error-suppression.md`

- [ ] **Step 1: Write the file**

- Inputs: `{site_id}`, `{file}`, `{function}`, `{lines}`.
- Allowed reads: imports/wasip2/, internal/component/component_linker.go (read-only — to confirm the panic-on-error path), the spec doc.
- Allowed writes: `{file}` only.
- Procedure:
  1. First action.
  2. Read the function around `{lines}`.
  3. Identify the exact `if err != nil { return placeholder }` block.
  4. Replace it with `if err != nil { return nil, fmt.Errorf("...") }` (the trap-emitting form). The error message should name the specific failure (invalid handle, missing field, etc.).
  5. Run `go build ./imports/wasip2/...` to confirm compile.
  6. Run `go test ./imports/wasip2/...` to capture pre-existing pass/fail state. The new trap behavior may break a test that was relying on the silent default — that test will be audited by `audit-existing-wiring-test.md` (or its Loop 3 equivalent).
- **CODE CHANGE — review chain.** Each fix is its own commit per Rule L3-C.

- [ ] **Step 2: Replace WARNING TEXT placeholder**

- [ ] **Step 3: Verify**

```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/fix-error-suppression.md
```
Expected > 70.

### Task 35: Write templates/add-wasip2-trap-test.md

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/add-wasip2-trap-test.md`

- [ ] **Step 1: Write the file**

- Inputs: `{site_id}`, `{fixed_function}`, `{file}`.
- Allowed reads: imports/wasip2/, internal/component/wasip2test/, internal/component/testdata/, the wazero public API.
- Allowed writes: the appropriate `_test.go` file for `{file}` (e.g., `imports/wasip2/sockets/tcp_test.go` for tcp.go fixes).
- Procedure:
  1. First action.
  2. Build (or reuse) a wasm fixture that calls `{fixed_function}` with an invalid handle.
  3. Write a Go test that loads the fixture via the public wazero API, instantiates, invokes, and asserts the trap is delivered (the call returns an error, or the wasm instance is in a trapped state).
  4. Run the test. Confirm pass.
- **CODE CHANGE — review chain.**

- [ ] **Step 2: Replace WARNING TEXT placeholder**

- [ ] **Step 3: Verify**

```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/add-wasip2-trap-test.md
```
Expected > 60.

---

## Phase 8 — Review chain templates

### Task 36: Write templates/review-spec-compliance.md (R1)

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/review-spec-compliance.md`

- [ ] **Step 1: Write the file**

This is the R1 reviewer template per spec §6.4. Key constraints:
- Reviewer is **fresh** — no prior conversation history.
- Reviewer is **not** the writer.
- Reviewer is **read-only** — Allowed writes is empty.
- Reviewer reads spec text BEFORE reading the diff.
- Reviewer returns either `APPROVED` (with verbatim spec quotes) or `FINDINGS` (with each finding linked to a spec quote).

Content:

````markdown
# review-spec-compliance.md (R1)

You are a fresh subagent dispatched to perform R1 (spec compliance review) on a code change produced by another subagent. You are NOT the writer. You have NO prior conversation history.

Your job is to read the spec section relevant to the change, read `definitions.py`, then read the diff line-by-line and determine whether the change correctly implements the spec.

You return either:
- `APPROVED` with verbatim spec quotes that support the implementation.
- `FINDINGS` with each finding linked to a spec quote showing what the correct behavior is.

## Spec-overrides-instructions warning

**[Insert THE WARNING TEXT verbatim here.]**

## First action

BEFORE reading the diff:
1. Read `debug-vendored/component-model/design/mvp/CanonicalABI.md` §`{spec_section}` in full.
2. Read `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py:{py_func}` in full.
3. In your return message, include verbatim quotes from both files.

You read the spec FIRST so you cannot be biased by what the diff "looks like."

## Inputs

- `{diff}`: the unified diff produced by the writing subagent. May span multiple files.
- `{writer_subagent_id}`: the ID of the writing subagent. You verify you are NOT this subagent. (Trivially true since you're freshly dispatched, but the parent records both IDs for auditability.)
- `{spec_section}`: e.g., `Flat Lifting`.
- `{py_func}`: e.g., `lift_flat`.
- `{relevant_go_function}`: the abi/ function the change affects, if applicable. May be null for changes to test files or status files.

## Allowed reads

- Every file referenced in the diff (read-only).
- `debug-vendored/component-model/design/mvp/CanonicalABI.md` (read-only).
- `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py` (read-only).
- `internal/component/abi/*.go` (read-only).
- `docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/reconciliation/*.md` (read-only — to check whether helpers used by the change are themselves reconciled).
- The spec doc `docs/superpowers/specs/2026-04-07-canonical-abi-unification-design.md` (read-only).

## Allowed writes

NONE. You are read-only. You produce a return message only.

## Procedure

1. Perform First action.
2. Read the diff in full.
3. For each changed line:
   a. Determine what spec rule the line is implementing.
   b. Find the corresponding `definitions.py` line.
   c. Find the corresponding `CanonicalABI.md` text.
   d. Determine whether the Go line correctly implements the spec.
4. If every line correctly implements the spec, return `APPROVED` with at least one verbatim spec quote per major spec assertion the diff implements (rule of thumb: one quote per non-trivial branch).
5. If any line does NOT correctly implement the spec, return `FINDINGS` with each finding as a row containing: file, line, what the line does, what the spec says, the verbatim spec quote contradicting the line, suggested fix.
6. Do NOT make code style judgments. That is R3's job. Your only concern is spec compliance.
7. Do NOT reject a change because the writing subagent skipped a code style convention. Forward style concerns to R3 by leaving them out of your findings.

## Halt conditions

File a BLOCKER if:
- The spec text and `definitions.py` disagree on the rule the diff is implementing. (Same as Loop 1's `bug-in-python` — you cannot decide unilaterally; the human resolves.)
- The diff implements a spec primitive that is not yet defined in `CanonicalABI.md`. (Async, stream, future — Loop 1 should have stubbed these.)
- You cannot identify which spec section the diff is implementing.

## Return format

```json
{
  "subagent_id": "<your-id>",
  "writer_subagent_id": "{writer_subagent_id}",
  "status": "APPROVED" | "FINDINGS" | "BLOCKER",
  "spec_section": "{spec_section}",
  "first_action_quotes": {
    "spec_quote": "<verbatim CanonicalABI.md text>",
    "spec_quote_source": "CanonicalABI.md §{spec_section}",
    "py_quote": "<verbatim definitions.py text>",
    "py_quote_source": "definitions.py:{py_func}"
  },
  "findings": [
    {
      "file": "internal/component/abi/lift.go",
      "line": 45,
      "current_behavior": "...",
      "spec_correct_behavior": "...",
      "spec_quote": "<verbatim>",
      "suggested_fix": "..."
    }
  ],
  "approval_quotes": [
    {
      "diff_line_range": "lift.go:42-50",
      "spec_quote": "<verbatim spec text>",
      "explanation": "this implements rule X correctly"
    }
  ],
  "blocker_reason": null
}
```

When `status: APPROVED`, `findings` is empty and `approval_quotes` has at least one entry.
When `status: FINDINGS`, `findings` has at least one entry.
When `status: BLOCKER`, `blocker_reason` is non-null and the parent files the BLOCKER.

## Self-check

1. Did I read CanonicalABI.md §{spec_section} BEFORE reading the diff? **(yes/no)**
2. Did I read definitions.py:{py_func} BEFORE reading the diff? **(yes/no)**
3. Did I include verbatim quotes from both files in `first_action_quotes`? **(yes/no)**
4. Did I check every changed line against the spec? **(yes/no)**
5. Did I avoid making code style judgments? **(yes/no — those are R3's job)**
6. If I returned APPROVED, do I have at least one approval quote per non-trivial branch? **(yes/no)**
7. If I returned FINDINGS, does every finding have a verbatim spec quote? **(yes/no)**

If any answer is "no", you must not return your current status. Either fix the issue or file a BLOCKER.

## After this template runs

This template is part of the review chain. After this template:
- If status=APPROVED: the parent dispatches `templates/review-code-quality.md` (R3).
- If status=FINDINGS: the parent dispatches `templates/revise-after-review.md` (R2 — a fresh reviser, not the writer, not you).
- If status=BLOCKER: the parent files the BLOCKER and ends the iteration.
````

- [ ] **Step 2: Replace WARNING TEXT placeholder**

- [ ] **Step 3: Verify**

```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/review-spec-compliance.md
```
Expected > 100.

### Task 37: Write templates/review-code-quality.md (R3)

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/review-code-quality.md`

- [ ] **Step 1: Write the file**

Mirror of `review-spec-compliance.md` with the following key differences:

- Title: `# review-code-quality.md (R3)`
- Purpose: review code style, naming, error handling, idiomatic Go usage, comment clarity, test naming, imports, adherence to wazero codebase conventions.
- Reviewer is **forbidden** from making spec-correctness judgments (those are R1's job). If a quality issue cannot be reviewed without judging spec correctness, file a BLOCKER.
- First action additionally requires reading 3-5 nearby files in the same package as style references (e.g., for a change to `internal/component/abi/lift.go`, read `internal/component/abi/lower.go` and `internal/component/abi/strings.go` and `internal/component/abi/context.go` for style consistency). Also read `CONTRIBUTING.md` if present.
- The First action also reads CanonicalABI.md and definitions.py because the user explicitly required this in spec §6.4 / §7.2: "The R3 code quality reviewer reads the spec before reading the diff (because quality cannot be judged without knowing what the code is supposed to do)." Do not skip this.
- Procedure focuses on: (1) idiomatic error handling vs. ad-hoc panics; (2) naming consistency with the surrounding package; (3) test naming and structure; (4) imports (no `internal/component/abi` imports in tests that are supposed to be public-API only); (5) comment clarity; (6) adherence to existing patterns.
- Return format mirrors R1's, with `findings` describing quality issues.
- Halt condition: a quality issue cannot be reviewed without judging spec correctness.

After this template:
- If status=APPROVED: parent commits.
- If status=FINDINGS: parent dispatches `revise-after-review.md`. After revision, **chain restarts at R1** (not R3).
- If status=BLOCKER: parent files the BLOCKER.

- [ ] **Step 2: Replace WARNING TEXT placeholder**

- [ ] **Step 3: Verify**

```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/review-code-quality.md
```
Expected > 100.

### Task 38: Write templates/revise-after-review.md

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/revise-after-review.md`

- [ ] **Step 1: Write the file**

This template is dispatched when either R1 or R3 returns FINDINGS. The reviser:
- Is NOT the writer.
- Is NOT the reviewer that produced the findings.
- Is NOT any prior reviser in the same chain.
- Has NO prior conversation history.

The reviser's job is to apply the findings as edits to the diff, producing a new diff that addresses every finding. Scope creep is prohibited — the reviser may NOT change anything not addressed by a finding.

Content:

````markdown
# revise-after-review.md

You are a fresh revision subagent dispatched after R1 or R3 returned FINDINGS on a code change. You are NOT the writer. You are NOT the reviewer that produced the findings. You are NOT any prior reviser in the same review chain.

You apply the findings as edits and produce a new diff. You may NOT change anything not addressed by a finding.

## Spec-overrides-instructions warning

**[Insert THE WARNING TEXT verbatim here.]**

## First action

Read `debug-vendored/component-model/design/mvp/CanonicalABI.md` §`{spec_section}` in full and `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py:{py_func}` in full BEFORE applying any finding. Quote both in your return message.

## Inputs

- `{findings}`: a JSON array of findings from R1 or R3. Each finding has file, line, problem, suggested fix, spec quote (R1) or style note (R3).
- `{review_phase}`: `R1` or `R3` — tells you whether you are revising after a spec compliance review or a code quality review.
- `{spec_section}`, `{py_func}`: same as the writer's inputs.
- `{previous_diff}`: the diff the reviewer found problems with.

## Allowed reads

- Every file in `{previous_diff}`.
- `debug-vendored/component-model/design/mvp/CanonicalABI.md`.
- `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py`.
- `internal/component/abi/*.go` (read-only).
- The relevant directories under `internal/component/` per the loop's isolation rules.

## Allowed writes

- Only the files listed in `{previous_diff}`. You may NOT add new files. You may NOT touch files not in the diff. (If a finding implies a new file or unrelated change, you cannot resolve it; halt with BLOCKER.)

## Procedure

1. First action.
2. Read every finding.
3. For each finding, in order (no batching):
   a. Read the relevant file at the relevant line.
   b. Apply the suggested fix (or your own fix that addresses the finding's spec quote, if you can justify it with another spec quote).
   c. If a finding's suggested fix conflicts with another finding (or with your own spec reading), halt with BLOCKER.
4. Run `go build ./internal/component/...` to confirm the revised diff compiles. (For non-Go diffs, the appropriate compile/lint check.)
5. Return the new diff summary in the return format.

## Halt conditions

- A finding's suggested fix would require changes outside the files in `{previous_diff}`.
- Two findings contradict each other.
- Applying a finding makes the diff fail to compile.
- A finding has no spec quote and no clear style reference (R3 findings should always have a style reference; R1 findings should always have a spec quote — if either is missing, the finding is malformed and you halt with BLOCKER asking the parent to re-dispatch the review).

## Return format

JSON with `subagent_id`, `status` (`complete`/`blocker`), `revised_files` (list), `findings_addressed` (count), `compile_passed`, `first_action_quotes`.

## Self-check

1. Did I read the spec and definitions.py before applying findings? **(yes/no)**
2. Did I address every finding? **(yes/no)**
3. Did I avoid making changes not addressed by findings? **(yes/no)**
4. Does the revised diff compile? **(yes/no)**

## After this template runs

The parent restarts the review chain at R1 with a fresh subagent. (Per spec §6.4 step R4: "After revision, the chain restarts at R1, not at R3.")
````

- [ ] **Step 2: Replace WARNING TEXT placeholder**

- [ ] **Step 3: Verify**

```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/revise-after-review.md
```
Expected > 80.

---

## Phase 9 — Shared support templates

### Task 39: Write templates/file-blocker.md

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/file-blocker.md`

- [ ] **Step 1: Write the file**

This template is dispatched when ANY subagent encounters a halt condition. It appends an entry to `status/blockers.json` with full context.

- Inputs: `{loop}` (1/2/3), `{iteration_function}` (or callsite/site id), `{kind}`, `{summary}`, `{spec_quote_attempted}`, `{py_quote_attempted}`.
- Allowed reads: `status/blockers.json` (to determine the next blocker id).
- Allowed writes: `status/blockers.json`.
- Procedure:
  1. Read existing blockers.json.
  2. Determine next id (e.g., `blk-001`, `blk-002`).
  3. Append the new blocker entry per the schema in spec §8.2.
  4. Atomically write the file.
- Audit only — no review chain.

- [ ] **Step 2: Replace WARNING TEXT placeholder**

- [ ] **Step 3: Verify**

```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/file-blocker.md
```
Expected > 50.

### Task 40: Write templates/verify-grep-zero.md

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/verify-grep-zero.md`

- [ ] **Step 1: Write the file**

- Inputs: `{symbol_name}`.
- Allowed reads: every Go file in the tree (the subagent runs grep).
- Allowed writes: none.
- Procedure:
  1. First action (in this case, just reading the spec section about deletion is sufficient).
  2. Run `grep -rn '\b{symbol_name}\b' --include='*.go' .` and capture all matches.
  3. Filter out matches that are: the symbol's own definition (in the file the parent named), comments mentioning the symbol, _test.go files that test the symbol's deletion (audit tests), the project status dir.
  4. If zero matches remain, return `verified: true`. If any matches remain, return `verified: false` with the list of matches.
- Audit only — no code change, no review chain.

- [ ] **Step 2: Replace WARNING TEXT placeholder**

- [ ] **Step 3: Verify**

```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/verify-grep-zero.md
```
Expected > 40.

---

## Phase 10 — README and final wiring

### Task 41: Write README.md for the project dir

**Files:**
- Create: `docs/superpowers/projects/2026-04-07-canonical-abi-unification/README.md`

- [ ] **Step 1: Write the file**

Content:
````markdown
# Canonical ABI Unification — Project Workspace

This directory contains the orchestration system that drives the wazero canonical-ABI unification work. **Nothing in this directory is committed to git.** The canonical artifacts (committed) are:

- Spec: `docs/superpowers/specs/2026-04-07-canonical-abi-unification-design.md`
- Plan: `docs/superpowers/plans/2026-04-07-canonical-abi-unification.md`

This README, the prompts, scripts, templates, status files, and harness are all uncommitted working artifacts. They drive multi-session iterative work but the work product itself (Go code in `internal/component/abi/`, tests in `internal/component/conformance/`, etc.) is committed normally.

---

## How to start a session

### First time (project bootstrap)

```bash
# 1. Confirm the project dir exists
ls docs/superpowers/projects/2026-04-07-canonical-abi-unification/

# 2. Confirm the Python harness works
python3 docs/superpowers/projects/2026-04-07-canonical-abi-unification/harness/spec_diff_driver.py --list-sync-functions

# 3. Confirm wasmtime is installed (optional but recommended)
wasmtime --version

# 4. Start a fresh Claude session and feed it the Loop 1 prompt:
#    The first prompt the agent should see is the contents of:
#    docs/superpowers/projects/2026-04-07-canonical-abi-unification/prompts/loop1-abi-correctness.md
#
#    Do this by either:
#    - Pasting the contents of loop1-abi-correctness.md as the first user message, or
#    - Telling the agent: "Read prompts/loop1-abi-correctness.md and follow it."
```

### Resuming a session

The orchestration is designed to resume cleanly from any point. Simply start a fresh Claude session and feed the same loop prompt. The parent agent's session-start protocol will read the status files and pick up where the last session left off.

---

## Loop progression

Loops run sequentially. Loop N cannot start until Loop N-1 is complete (verified by `status/project-state.json`).

1. **Loop 1 — `prompts/loop1-abi-correctness.md`**: reconcile every sync function in `internal/component/abi/` against `definitions.py`, port the synchronous test functions from `run_tests.py`, install async stubs, audit existing conformance tests.
2. **Loop 2 — `prompts/loop2-runtime-migration.md`**: migrate every runtime call site in `canon_lower.go`, `component_linker.go`, `instance.go` to call into `abi/`, delete every parallel implementation helper, audit existing wiring tests.
3. **Loop 3 — `prompts/loop3-wasip2-cleanup.md`**: convert every silent-default error site in `imports/wasip2/sockets/{tcp,udp}.go` and `imports/wasip2/http/http.go` to `(nil, error)` trap.

After Loop 3 completes, surface `status/project-state.json` to the user and ask for the manual confirmation gate (P-T9 in spec §9.4).

---

## Inspecting state

```bash
# Project completion
cat docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/project-state.json | jq

# Loop 1 progress
cat docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/loop1-functions.json | jq '.functions | to_entries | map({key: .key, status: .value.iteration_status})'

# Open blockers
cat docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/blockers.json | jq '.blockers | map(select(.resolution_status == "open"))'

# Iteration log (most recent 10)
cat docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/iteration-log.json | jq '.iterations[-10:]'

# Reconciliation reports
ls docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/reconciliation/
```

---

## Manual override

You may edit any status file by hand. The parent agent reads whatever JSON it finds, as long as it parses and `schema_version` matches. Use cases:

- Mark a blocker as resolved with notes.
- Demote a falsely-claimed pass back to pending.
- Remove a function from the queue if it turns out to be out of scope.
- Roll back a loop completion if you discover a missed bug.

The parent agent does not validate that a manual override is "correct." Trust is in the human.

---

## Files in this directory

- `prompts/` — three loop prompts, one entry point each
- `scripts/` — three iteration scripts, one playbook per loop
- `templates/` — 28 subagent templates for every kind of work
- `status/` — 14 JSON status files + reconciliation/ + audit/ subdirs
- `harness/spec_diff_driver.py` — the Python wrapper around `definitions.py` for Gate 3 differential testing

For full details of the design, read the spec: `docs/superpowers/specs/2026-04-07-canonical-abi-unification-design.md`.
````

- [ ] **Step 2: Verify**

```bash
wc -l docs/superpowers/projects/2026-04-07-canonical-abi-unification/README.md
```
Expected > 80.

### Task 42: Final structural verification

- [ ] **Step 1: Verify all expected files exist**

Run:
```bash
cd docs/superpowers/projects/2026-04-07-canonical-abi-unification && find . -type f | sort
```

Expected output (51 files):
```
./README.md
./harness/spec_diff_driver.py
./harness/test_spec_diff_driver.py
./prompts/loop1-abi-correctness.md
./prompts/loop2-runtime-migration.md
./prompts/loop3-wasip2-cleanup.md
./scripts/loop1-iteration.md
./scripts/loop2-iteration.md
./scripts/loop3-iteration.md
./status/blockers.json
./status/iteration-log.json
./status/loop1-async-stubs.json
./status/loop1-existing-conformance-audit.json
./status/loop1-functions.json
./status/loop1-publicapi-coverage.json
./status/loop2-baseline.json
./status/loop2-callsites.json
./status/loop2-existing-wiring-audit.json
./status/loop2-wiring-tests.json
./status/loop3-baseline.json
./status/loop3-suppressed-errors.json
./status/project-state.json
./status/wasmtime-fixture-corpus.json
./templates/add-spec-citation.md
./templates/add-wasip2-trap-test.md
./templates/add-wiring-test.md
./templates/audit-existing-conformance-test.md
./templates/audit-existing-wiring-test.md
./templates/build-public-api-exercise.md
./templates/capture-loop2-baseline.md
./templates/delete-dead-helper.md
./templates/diagnose-loop2-regression.md
./templates/diagnose-wasmtime-divergence.md
./templates/enumerate-callsites.md
./templates/enumerate-functions.md
./templates/enumerate-wasip2-suppression-sites.md
./templates/enumerate-wasmtime-fixtures.md
./templates/file-blocker.md
./templates/fix-conformance-audit-finding.md
./templates/fix-error-suppression.md
./templates/fix-reconciliation-finding.md
./templates/install-async-stub.md
./templates/migrate-callsite.md
./templates/port-spec-test.md
./templates/reconcile-function.md
./templates/review-code-quality.md
./templates/review-spec-compliance.md
./templates/revise-after-review.md
./templates/run-python-differential.md
./templates/run-wasmtime-conformance.md
./templates/verify-grep-zero.md
```

- [ ] **Step 2: Verify every template includes the canonical warning text**

Run:
```bash
for f in docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/*.md; do
  if ! grep -q "SPEC-OVERRIDES-LOCAL-INSTRUCTIONS RULE" "$f"; then
    echo "MISSING WARNING: $f"
  fi
done
```
Expected: no output (every template has the warning).

- [ ] **Step 3: Verify no template still has the placeholder string**

Run:
```bash
grep -rn "Insert THE WARNING TEXT" docs/superpowers/projects/2026-04-07-canonical-abi-unification/templates/
```
Expected: no output.

- [ ] **Step 4: Verify no prompt still has the placeholder string**

Run:
```bash
grep -rn "Insert verbatim" docs/superpowers/projects/2026-04-07-canonical-abi-unification/prompts/
```
Expected: no output.

- [ ] **Step 5: Verify all status JSON files parse**

Run:
```bash
for f in docs/superpowers/projects/2026-04-07-canonical-abi-unification/status/*.json; do
  python3 -c "import json,sys; d=json.load(open(sys.argv[1])); assert d.get('schema_version') == 1, f'bad schema_version in {sys.argv[1]}'; print(sys.argv[1], 'OK')" "$f"
done
```
Expected: 14 lines, each ending with `OK`.

- [ ] **Step 6: Verify the harness still works**

Run:
```bash
python3 docs/superpowers/projects/2026-04-07-canonical-abi-unification/harness/spec_diff_driver.py --list-sync-functions
```
Expected: a JSON array containing at minimum `["lift_flat", "lower_flat", ...]`.

### Task 43: Smoke test — start a Loop 1 session and verify it reaches first work item

This task is **manual** for the user. The plan does not automate it because the smoke test is the moment the orchestration is handed over to the user.

- [ ] **Step 1: Start a fresh Claude session**

Open a new Claude session. Either via the CLI (`claude`), the desktop app, or the IDE extension.

- [ ] **Step 2: Feed the Loop 1 prompt**

Tell the new session: "Read `docs/superpowers/projects/2026-04-07-canonical-abi-unification/prompts/loop1-abi-correctness.md` and follow it." Or paste the contents of the prompt directly.

- [ ] **Step 3: Verify the parent agent performs the session-start protocol correctly**

The parent should:
1. Read the spec doc.
2. Read the iteration script.
3. Run `git status` and confirm clean.
4. Read every status file.
5. Detect that the status files are empty (first session).
6. Run the bootstrap: dispatch `enumerate-functions.md`, `enumerate-wasmtime-fixtures.md`, etc.
7. After bootstrap, find the first pending work item.

If the parent fails any of these steps (skips reading the spec, skips status files, batches subagents, etc.), the orchestration has a bug. Halt the smoke test and fix the bug before continuing.

- [ ] **Step 4: Verify the first iteration completes through Gate 1**

Let the parent run one full iteration. Verify:
- A reconciliation report is written to `status/reconciliation/<func>.md`.
- The report has the expected structure.
- If any code change was produced, both R1 and R3 review chains ran.
- The commit message has the required footer.

If everything works, the orchestration is operational. Future sessions will simply restart the loop prompt.

- [ ] **Step 5: Document the smoke test outcome**

Append a note to the project README's bottom: a one-line "First smoke test passed YYYY-MM-DD" entry. (This is the only edit to the README after Task 41.)

---

## Plan completion

When Task 43 passes its smoke test, the orchestration system is built and operational. The actual canonical-ABI work is now handed off to the orchestration: the user starts sessions by feeding the loop prompts to fresh Claude instances, and the iterative work proceeds until project termination per spec §9.4.

This plan does not include the canonical-ABI work itself. That work is done BY the orchestration AFTER this plan completes.

---

## Self-review checklist (run after writing the plan; documented here for the executing agent)

1. **Spec coverage:** every section of the spec is covered by tasks above. Specifically:
   - Spec §3 (Architecture): the orchestration produces the abi/ engine via Loop 1; Tasks 13-25 build the templates that drive that.
   - Spec §4.1 / §4.2 (Loop isolation): Tasks 7-12 build the prompts and scripts that enforce isolation.
   - Spec §5 (Gates): Tasks 13-25 build templates for every gate.
   - Spec §6 (Subagent dispatch + review chain): Tasks 36-38 build the review chain templates; every other code-producing template explicitly calls the chain in its `## After this template runs` section.
   - Spec §7 (Spec-overrides rule): the verbatim text is in spec §7.5, referenced as "THE WARNING TEXT" throughout, and included verbatim in every template via the placeholder-replacement step.
   - Spec §8 (Status & resume): Tasks 2-5 build all 14 status files with their initial schemas.
   - Spec §9 (Termination): the loop prompts (Tasks 7-9) include the termination-check protocol referencing the spec sections.
   - Spec §10 (Deliverable inventory): the file structure section at the top of this plan matches.
   - Spec §11 (Out of scope): captured in async stub installation (Task 23) and the explicit error format.

2. **Placeholder scan:** the plan uses `[Insert THE WARNING TEXT verbatim here.]` and `[Insert verbatim ...]` as deliberate placeholders that the executing agent MUST replace before each task is complete. Each task that uses one has an explicit verification step (`grep -c "Insert..." ... | expected: 0`).

3. **Type/name consistency:** template names, status file names, JSON field names, and section names are consistent across all tasks. Every reference to a template by name uses the same filename.

If you find an issue while executing this plan, halt and fix the plan before proceeding. The plan is committed to git and is the source of truth.
