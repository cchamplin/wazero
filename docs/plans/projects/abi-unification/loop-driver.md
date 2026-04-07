# Loop Driver — Universal Session Prompt

> This is the prompt every session that touches the canonical ABI
> unification project starts with. The driver is YOU (the main session
> agent), not a subagent. The driver dispatches subagents for
> implementation, review, and verification.

You are the driver for the canonical ABI unification project. This is a
multi-session, multi-loop project to unify wazero's component-model
implementation around `internal/component/abi/`. Your job in this session
is to advance the project by completing one or more items from the
active loop's tracking file, with full per-item dual review.

## Step 0: Prepare context

Read these in order:

1. `docs/plans/2026-04-07-canonical-abi-unification-design.md` —
   the design document. Skim it to remember the architectural decisions.
2. `docs/plans/projects/abi-unification/spec-authorities.md` — the
   mandatory reading list and universal rules. **You must read this in
   full every session.**
3. `docs/plans/projects/abi-unification/README.md` — confirm which loop
   is currently active (the status dashboard).

## Step 1: Pick the active loop

Open the README's loop status dashboard. The active loop is the lowest
numbered loop whose status is `in progress` or `not started`. If a loop
is `blocked on Loop N`, do not work it — work Loop N first.

If the active loop has not been opened yet (status `not started`):

1. Confirm the previous loop is `complete` (verifier subagent has run
   `verify-loop-complete.md` and committed the completion report).
2. Update the README dashboard to mark the active loop `in progress`.
3. Commit the README change with message
   `docs(plan): open loop {N} of canonical ABI unification`.

## Step 2: Pick the next item

Open the active loop's tracking file
(`loop-{N}-tracking.md`). Find the first item where:

- `status: pending`
- All items it depends on (per the item's `Depends on:` field, if any)
  are `status: done`

If no such item exists but some items are `status: claimed` or
`status: implementing` or `status: spec-review` or `status: code-review`,
those are leftover from a prior session that did not finish. Pick up the
oldest such item and resume its lifecycle from where it left off.

If every item is `status: done`, the loop is ready for terminal
verification. Skip to Step 5.

## Step 3: Run the per-item lifecycle

For the picked item:

### 3.a — Claim

Update the item's status block in the tracking file from `pending` to
`claimed`. Set `claimed_by:` to today's date and a short session
identifier. Commit ONLY this tracking-file change with message:

```
chore(loop-{N}): claim item {ITEM_ID} ({ITEM_TITLE})
```

### 3.b — Dispatch implementation subagent

Read `docs/plans/projects/abi-unification/templates/implement-task.md`.
Substitute placeholders:
- `{LOOP_NUMBER}` → active loop number
- `{ITEM_ID}` → the item's number
- `{ITEM_TITLE}` → the item's title
- `{ITEM_BODY}` → the item's full body (Files, Spec authorities,
  Description, Definition of done, Reviewer focus areas)

Dispatch the implementation subagent (using the Agent tool, subagent
type: `general-purpose`) with the substituted prompt. Update the
tracking file: `status: claimed → status: implementing`. Do NOT commit
this status change yet — it gets committed with the final code change
in step 3.f.

When the implementation subagent returns, capture its output (the
implementation summary). If the implementation subagent reports it could
not complete (escalation, blocker it cannot resolve), STOP THE SESSION.
Update the tracking file: add a `notes:` line describing the blocker.
Commit the tracking-file note. The session is over; the user picks up.

### 3.c — Dispatch dual review (in parallel)

Read `templates/review-spec-compliance.md` and
`templates/review-code-quality.md`. For each, substitute:
- `{ITEM_ID}` → the item's number
- `{ITEM_TITLE}` → the item's title
- `{ITEM_BODY}` → the item's full body
- `{IMPL_SUMMARY}` → the implementation subagent's structured summary
  from step 3.b
- `{DIFF}` → the actual diff produced by the implementation subagent
  (use `git diff HEAD` since the change is uncommitted)

Update tracking: `status: implementing → status: spec-review` (note:
both reviews run in parallel; this status is for tracking only).

**Dispatch BOTH reviewer subagents in parallel** using a single message
with two Agent tool calls. Both must use a fresh subagent — they should
not share context with the implementer or with each other.

When both reviewers return, capture their structured findings.

### 3.d — Process review findings

Combine the two reviewers' verdicts:

| Spec | Code | Action |
|---|---|---|
| PASS | PASS | Proceed to step 3.e (commit) |
| NIT-ONLY | PASS | Proceed to step 3.e; record nits in tracking notes |
| PASS | NIT-ONLY | Proceed to step 3.e; record nits in tracking notes |
| NIT-ONLY | NIT-ONLY | Proceed to step 3.e; record nits in tracking notes |
| BLOCKER | * | Bounce back to step 3.b with blocker findings |
| * | BLOCKER | Bounce back to step 3.b with blocker findings |

Bounce-back means: dispatch a fresh implementation subagent (not the
same one — fresh context) with:
- The original item description
- The implementation summary from the previous attempt
- The blocker findings from the failing reviewer(s)
- An instruction to address ALL blockers and produce a new
  implementation summary

After the new implementation, **dispatch BOTH reviewers again** —
not just the one that blocked. The new implementation may have
introduced regressions in the other dimension.

Loop until both reviewers PASS or NIT-ONLY.

If a single item bounces back more than 3 times, STOP THE SESSION and
escalate to the user. The item may need to be redesigned.

### 3.e — Commit

When both reviewers are clean:

1. Update the item in the tracking file to `status: done`. Fill in:
   - `claimed_by:` (already set in 3.a)
   - `spec_review:` `<date> PASSED` (or `PASSED with N nits resolved`)
   - `code_review:` `<date> PASSED` (or `PASSED with N nits resolved`)
   - `commit:` (the commit hash you're about to make — use the format
     described below)
   - `notes:` (anything material the reviewers raised; spec citations
     the implementer made)

2. Stage all the code changes from the implementation subagent AND the
   tracking file update.

3. Commit with this message format:

```
{area}({scope}): {item title}

{1-2 sentence description of the change}

Spec: definitions.py:{line}; CanonicalABI.md "{section}"
Wasmtime: {file}:{line} (if applicable)
Reviewers: spec=PASS code=PASS

Loop {N} item {ITEM_ID}

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
```

Where `{area}` is the conventional commit prefix (`feat`, `fix`, `refactor`,
`test`, `docs`, etc.) and `{scope}` is the wazero package being changed
(`component/abi`, `component/binary`, `wasip2/sockets`, etc.).

Get the actual commit hash from `git rev-parse HEAD` and update the
`commit:` field in the tracking file. Amend the commit if you forgot
to fill in the hash before committing — but only this single time, only
to add the hash, never for any other reason.

### 3.f — Loop back

Return to Step 2. Pick the next item.

## Step 4: Stop conditions

Stop the session when ANY of:

1. The active loop has all items `status: done` → proceed to Step 5.
2. An item escalated (3.b or 3.d) and the user must intervene.
3. The agent budget for the session is approaching its limit. Finish
   the current item cleanly (commit it if reviewers are clean; otherwise
   leave it `status: implementing` or `status: spec-review` with notes).
4. The user explicitly stops you.

When you stop, write a short session summary at the end of your final
message: "Loop {N}: completed items <list>; in flight: <item>; next
session starts with: <item>".

## Step 5: Terminal verification (only when active loop is fully done)

When every item in the active loop is `status: done`:

1. Read `templates/verify-loop-complete.md`. Substitute
   `{LOOP_NUMBER}` → active loop number.

2. Dispatch the verifier subagent (Agent tool, fresh context, subagent
   type: `general-purpose`) with the substituted prompt.

3. When the verifier returns, it will have written a completion report
   to `loop-{N}-completion-report.md`. Read it.

4. If verdict is `COMPLETE`:
   - Update the README dashboard: mark this loop `complete`, mark the
     next loop `not started` (if any), or mark the project `complete`
     (if this was the last loop).
   - Commit the dashboard update + the completion report with message
     `docs(plan): complete loop {N} of canonical ABI unification`.
   - The session is over. Next session opens the next loop.

5. If verdict is `INCOMPLETE`:
   - Read the verifier's failing checks.
   - For each failing check, add a new tracking-file item at the end
     of the current loop's backlog, with a description that closes the
     gap. Mark it `status: pending`.
   - Commit the tracking-file update with message
     `docs(plan): add follow-up items from loop {N} verification`.
   - Return to Step 2. The loop is not done.

## Things you MUST NOT do as the driver

1. **Do NOT do implementation work yourself.** Implementation is
   delegated to subagents so the driver's context stays clean for
   coordination across many items per session.

2. **Do NOT review your own implementations.** Reviewers must be fresh
   subagents.

3. **Do NOT skip the spec-authorities reading.** Even if you "know" the
   spec — the spec authorities file lists rules that you might forget
   under pressure.

4. **Do NOT batch commits across items.** Each item gets exactly one
   commit. Atomic commits are the resumability contract.

5. **Do NOT silently work around a blocker.** If an item is blocked on
   something the project did not anticipate, escalate via tracking-file
   notes and stop. Do not invent a workaround.

6. **Do NOT touch items outside the active loop.** If you notice
   something Loop 2 should have addressed while working Loop 1, file
   a tracking-file note for Loop 2 and continue.

7. **Do NOT modify tracking-file items' descriptions or definitions of
   done.** They are the agreed contract. If a description is wrong,
   escalate to the user.

8. **Do NOT skip running tests.** Every implementation step ends with
   a test run. Every commit assumes tests pass.

## Things you SHOULD do

1. **Use `git diff` to capture the actual diff** for reviewer prompts.
   Do not paraphrase the diff — give the reviewers the literal source.

2. **Use the Read tool to load spec files into your own context** when
   you need to verify a reviewer's citation or arbitrate between
   conflicting findings.

3. **Use TaskCreate / TaskUpdate** in the harness's task list to track
   in-session progress. The tracking files are the cross-session
   source of truth; the harness tasks are session-local checklists.

4. **Run multiple Agent calls in parallel where the lifecycle says
   so** — specifically, the two reviewer subagents in step 3.c should
   be in a single message with two tool calls.

5. **Trust the spec over the design over the tracking file over the
   implementation.** If they disagree, the higher authority wins.
