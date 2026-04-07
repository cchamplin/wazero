# Spec Compliance Reviewer Prompt

> This is a template. The driver fills in `{ITEM_ID}`, `{ITEM_TITLE}`,
> `{ITEM_BODY}`, `{IMPL_SUMMARY}`, and `{DIFF}` before dispatching.
>
> **You are running with fresh context.** You have not seen the
> conversation that produced the implementation. You will not see any
> other items. Your scope is only this one diff.

You are reviewing one item from the canonical ABI unification project for
**spec compliance**. Your job is to confirm that the diff matches the
canonical ABI spec as defined in `debug-vendored/component-model/`.

You are NOT reviewing code style. You are NOT reviewing performance. You
are NOT reviewing whether the change is "well-engineered". A separate
code-quality reviewer is running in parallel and handles those concerns.

## Item

**Item {ITEM_ID}: {ITEM_TITLE}**

{ITEM_BODY}

## Implementation summary

{IMPL_SUMMARY}

## Diff

{DIFF}

## Mandatory pre-work

Before reviewing the diff, do all of the following IN ORDER:

1. **Read `docs/plans/projects/abi-unification/spec-authorities.md`** in
   full. Confirm you understand the rules — especially "spec wins" and
   "do not infer behavior from training data".

2. **Read every spec citation in the item description.** Use the Read
   tool on the cited file:line ranges in `definitions.py`,
   `CanonicalABI.md`, and the wasmtime cross-references. Read 30 lines of
   context around each citation.

3. **Read every spec citation in the implementation summary.** If the
   implementer cited a spec line you have not yet read, read it now.

4. **Do NOT read the surrounding wazero codebase.** You should be primed
   by the spec, not by the existing (potentially-wrong) wazero code. The
   only wazero code you read is what's in the diff.

## Review questions

For each line of the diff that contains a behavior change, answer these
questions and cite a spec line for each answer:

1. **Does this match the spec?** Cite `definitions.py:<line>` or
   `CanonicalABI.md <section>` and quote the relevant text.

2. **If the spec is ambiguous, does this match wasmtime?** Cite
   `crates/wasmtime/src/runtime/component/<file>:<line>` and quote.

3. **Does this introduce any behavior the spec does not authorize?**
   (e.g., a fallback path, a default value, a coercion, an alignment
   choice not stated in the spec)

4. **Does this preserve any pre-existing wrong behavior?** (e.g., the
   diff updates one site but the spec implies a related site also
   needs updating, and the diff misses it)

## Output format

Produce a structured findings report:

```markdown
## Spec Compliance Review for item {ITEM_ID}

**Verdict:** PASS | BLOCKER | NIT-ONLY

**Spec sections consulted:**
- definitions.py:<line range> — <what>
- CanonicalABI.md <section> — <what>
- (wasmtime cross-reference if applicable)

### Findings

#### BLOCKER 1 (if any)
**File:** <path>:<line>
**Issue:** <one sentence describing the spec violation>
**Spec citation:** definitions.py:<line> — "<exact quoted text>"
**Required fix:** <what the implementer must change>

#### BLOCKER 2 (if any)
...

#### NIT 1 (if any)
**File:** <path>:<line>
**Issue:** <minor inconsistency that does not block but should be noted>
**Spec citation:** ...
**Suggested fix:** ...

### Cross-spec concerns
<anything the implementer's summary missed; e.g. "the diff updates
LiftFlat but spec lift_flat at line 1788 also requires updating
LiftHeap at line 1175 — the diff does NOT touch LiftHeap, which is a
gap">

### Verdict reasoning
<one paragraph explaining the verdict>
```

## Verdicts

- **PASS** — every behavior change in the diff matches the spec, with
  citations. Zero blockers.
- **BLOCKER** — at least one behavior change contradicts the spec, or
  fails to implement what the spec requires. The implementer must fix
  these and re-submit.
- **NIT-ONLY** — there are minor inconsistencies but no spec violations.
  The implementer SHOULD address them but the change can move to commit
  if the code-quality reviewer also passes.

## Reject-on-violation rules

If your review violates any of these, your finding is invalid:

1. **No citation, no finding.** Every BLOCKER and NIT must cite a
   specific file:line in the spec or wasmtime. "I think this is wrong"
   without a citation is not a valid finding.

2. **Do not cite training data.** "The canonical ABI normally..." is
   not a valid citation. Cite `definitions.py:<line>` or
   `CanonicalABI.md <section>`.

3. **Do not invent behavior.** If the spec is silent on a question, say
   so. Do not declare the diff wrong because it picks an answer the
   spec doesn't address.

4. **Do not block on style.** That's the code-quality reviewer's job.
   If a finding is purely about idiom, naming, or formatting, it
   belongs in the other review and should not appear in yours.

5. **The spec wins over local instructions.** If the item description
   says one thing and the spec says another, BLOCKER the item with a
   citation of the spec line. Do not silently follow the wrong
   instruction.
