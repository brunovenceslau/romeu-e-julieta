# Stage B: the trace pass from the specification to the plan

Reader: reviewers checking how the specification gained requirement
ids and how the plan was traced to them, and maintainers tracing why
the spec and the plan read as they do. Type: reference (history). The
current state is [docs/spec.md](../spec.md) and
[docs/plan.md](../plan.md); stage A ended with
[round-18.md](round-18.md); the lenses are defined in
[lenses.md](lenses.md).

This pass is not a numbered round. It is the first pass of stage B in
the sense of "From stage A to stage B" in lenses.md: the plan held
against the specification as stage A corrected it. Its artifacts are
kept in [stage-b/](stage-b/), listed in "What is kept" below.

## What the pass did and why

Pull request #36 applied review rounds 9 to 18 and changed the
specification in every chapter, about 1300 lines. The plan only
received a patch per round, and the re-plan of pull request #43 was
driven by six ordering requirements. Nothing traced the specification
to the tasks, so no one could say which obligation a task builds, or
which obligation no task builds.

The pass closes that gap with one mechanism and three changes:

- Stable requirement ids, `REQ-nnnn`, one per independently checkable
  obligation of the specification. 12 12.11 states the rule; ADR 0012
  records the decision and is Proposed.
- An allocator and a check in one subcommand, `tools/ci reqs`:
  `go run ./tools/ci reqs allocate` writes the ids, and
  `go run ./tools/ci reqs` checks them in `fast` and `all`. The check
  fails on an active id that no task cites, and on a task that cites
  an unknown or retired id.
- A write pass that leaves the specification pristine: the findings
  of the trace and of the reviews applied, and the ids placed. It is
  this branch, `docs/spec-req` (pull request #45).
- The re-plan updated against the ids, on the plan branch, stacked on
  this one, for pull request #43.

## How the pass was run

One workflow of 30 agents ran the pass, with 0 agent errors. Every
agent was read-only except the two writers and the fixer. The
workflow script is kept as [stage-b/workflow.js](stage-b/workflow.js).

| Phase | Agents | Model | What it did |
|---|---|---|---|
| Trace | 8 | opus | six slices of the specification (trace-A to trace-F), the reverse trace from tasks to the specification (trace-R) and the priorities of the plan (trace-P, lenses L9, L11 and L12) |
| Craft | 3 | fable | coherence (L13 and L3), prose (L8 and ADR 0001) and testability (L6 and L11) of the specification |
| Verify 1 to 3 | 3 | fable | adversarial verifiers of the trace and craft findings, defaulting to refuted when the evidence does not hold |
| Skills | 6 | fable | Go and domain specialists: the Go of the specification, of the plan and of the code, the runtime, the supply chain, and the migration of existing code |
| Verify 4 and 5 | 2 | fable | adversarial verifiers of the skill findings |
| Synthesize | 1 | fable | one specification change list and one plan change list |
| Write | 2 | fable | the specification branch with the tool, then the plan branch |
| Review | 4 | fable | completeness, consistency, the plan against the stage-B criteria, and a fresh read of the specification through L13, L3, L8 and L6 |
| Fix | 1 | fable | one write pass over both branches |

The consolidator's record is its second pass, kept as
[stage-b/reports/synth2.json](stage-b/reports/synth2.json); it read the
first pass, made before the skill phase was added, as one of its
inputs. Across all passes the verifiers returned 512 verdict rows; of
the 166 rows of verify 4 and 5, 93 were confirmed, 64 modified and 9
refuted, and the refuted ones were dropped. The consolidation produced
349 specification edits, 1193 requirement ids after the splits (1178
candidates placed, 118 excluded), 18 new tasks (T121 to T138), edits to
90 existing tasks, a decision on each of the 45 pending items it read,
and 16 decisions for the operator (OD-1 to OD-16).

Every agent that wrote, changed, tested or reviewed Go loaded the
cc-skills-golang skills, starting with golang-how-to and every skill it
recommends, used gopls for navigation, and reported the skills it
loaded. That rule lives in the sandbox payload, not in this repository;
each report names its skills.

For this pass only, craft findings were applied as well as
contradictions and behavior gaps (decided on 2026-10-12). Stage A's
exit rule, in lenses.md, blocks on the latter two classes only.

### The review and the fix pass

The four reviewers read the specification branch at 8cc30a8 and the
plan branch, each against the frozen baselines. Their reports are in
[stage-b/reports/](stage-b/reports/). None returned GO: one Blocking
rendering defect (ids placed before list markers) and a set of
Required findings on placement, `owns` overlaps, the critical path and
the wording of 12 12.11.

The fixer applied 75 findings and declined 2, recorded in
[stage-b/reports/fix.json](stage-b/reports/fix.json):

- completeness-8, the shape of the pull request: no repository file
  carries it, since the change list was a scratch document of the
  write pass and the shape is the orchestrator's call. Its
  recommendation, one pull request with the checks, spec, decisions
  and ask-first lines, was followed.
- RR-21, a split of T068: measured after RR-1, T068 cites 117 ids and
  only 14 name recreate. A split would move 14 ids, keep T068 above
  100 and duplicate the shared acceptance, so T068 stays whole.

The fixer added REQ-1203 to REQ-1233 and retired REQ-0033, REQ-0036,
REQ-0139, REQ-0213, REQ-0560, REQ-0561, REQ-0562 and REQ-1195, and
added T139 and T140 to the plan.

### What is kept

- [stage-b/workflow.js](stage-b/workflow.js): the workflow script.
- [stage-b/inventory.py](stage-b/inventory.py) and
  [stage-b/inventory-summary.json](stage-b/inventory-summary.json): the
  phase-0 inventory of spec sections and tasks, and its summary.
- [stage-b/pending-items.md](stage-b/pending-items.md): the ids of the
  pending items the trace read. Their text stays in the repository's
  reminder store, because it holds session context that does not
  belong in a public record.
- [stage-b/tool-contract.md](stage-b/tool-contract.md): the contract of
  `tools/ci reqs` the consolidator handed to the writer.
- [stage-b/reports/](stage-b/reports/): the consolidator, the four
  reviewers and the fixer.

Sandbox paths in the copies read `<scratch>`, `<session>`,
`<worktrees>`, `<src>`, `<home>` and `<tmp>`. Not kept: the change
lists of the consolidator (spec-changes.md and plan-changes.md, about
1.1 MB together), the requirement map and the merged verdicts, each
over 300 KB, and the reports of the other agents. The commits of this
branch and of the plan branch hold every change those lists describe;
the plan's decisions on the pending items are in docs/plan.md,
"Decisions of the re-plan", on the plan branch.

## Session checkpoint (2026-10-13)

The stage-B trace pass ran as one workflow, its output landed on this
branch, and the tool's focused gate ran two rounds.

### Decided

- Decided on 2026-10-12: requirement ids are `REQ-nnnn` from one global
  counter. An id is never reused, a retired id is recorded in the
  table of 12 12.11, and ADRs carry no ids. Ids are allocated only by
  `tools/ci reqs allocate`, never by hand. Tasks cite them in
  Implements. The check fails on an active id that no task cites and
  on a task that cites an unknown or retired id. Citing ids in code and
  tests is deferred to the first implementation task that needs it.
- Decided on 2026-10-12: for this pass only, craft findings were
  applied as well as contradictions and behavior gaps, reversing the
  stage-A rule for this pass.
- Decided on 2026-10-12: every agent that writes, changes, tests or
  reviews Go loads the cc-skills-golang skills, starting with
  golang-how-to, uses gopls for navigation, and reports the skills it
  loaded. The rule lives in the sandbox payload; here it is the method
  of the pass.
- Decided on 2026-10-12: the sixteen decisions of the re-plan's block,
  OD-1 to OD-16, take their recommended options. They are recorded on
  the plan branch, pull request #43, in docs/plan.md, "Decisions of the
  re-plan". OD-14 and OD-7 wait for a confirmation (items 2 and 3
  below).
- Decided on 2026-10-12, pull request #44, the gopls pin: the CI cost
  of building gopls is accepted, the split of the pinned tools is
  deferred, and the approval line uses the bare form until T005.
- Decided on 2026-10-12, pull request #45: the workflow's own chain
  (find, verify, write, four reviews, fix) stands in for the full ship
  gate on the documents, and the Go tool gets a focused gate, a
  security auditor and a test engineer, which may run after the pull
  request opens.
- Declined on 2026-10-13: the fix pass's suggestion to change T037's
  "the sitting that opens M3" to "sitting M2", because it contradicts
  OD-15.

### Done

- The workflow's output on this branch: the write pass and the ids,
  ac488e2; the tool, 8cc30a8; the fix pass, b8439cb and 0e44a9e.
  `go run ./tools/ci all` exited 0.
- Gate round 1 at 0e44a9e: security auditor GO, with 3 Low and 2 Info;
  test engineer NO-GO, with 4 missing tests and 5 cheap ones.
- Fixes at 0590ae8 (CommonMark fences and backticks counted in one
  pass) and 8229928 (12 12.11 states that the check cannot see a
  deleted highest id).
- Gate round 2 at 8229928: both GO, with remaining test gaps and an
  Info on the wording of 12 12.11.
- Fixes at bf98aea (tests pin the fence, citation and quoting edges)
  and 7db9744 (12 12.11 says a deletion of the highest ids passes).
- Gate round 3, the targeted re-audit of the tests, was pending when
  this page was written.

### Open for the operator

One block, a recommendation per item.

1. Update pull request #43. Its branch, docs/replan, is replaced by a
   force push of the plan rebased on docs/spec-req; the current plan
   sits on branch docs/replan-v2. Recommended: yes.
2. OD-14, the open-items cut-over. The block's answer chose T119 (M5);
   the stage-B review moved it to T111 (M1), because a second store
   left open until M5 would hold items the count never sees.
   Recommended: T111.
3. OD-7, the accepted risks. Accept every risk row of the plan, or only
   those linked to the five rows of 05 5.4? The plan branch accepts
   only the linked ones today. Recommended: all, which is what OD-7
   said.
4. Items 8 to 11 of the plan's decision block (the narrowing of the
   checks and contracts surfaces, the grants, the surfaces) stay open
   and are needed before T107 or sitting M0. Recommended: decide them
   at sitting M0.
5. `internal/egress/**` is on no ask-first surface. This predates pull
   request #45; the security audit found it, and it is recorded as a
   pending item. Recommended: add it to `catalog` in its own pull
   request.
6. Merges: #45 first, then #43, stacked on it, plus #44. Recommended:
   that order.
7. Pending items for the repository's reminder store:
   - the deletion limit of `tools/ci reqs` (a deletion of the highest
     ids passes the check), with its reopening trigger;
   - closing the spec sweep item of 2026-10-01, per OD-16;
   - the measurement of OD-9 at sitting M0.

   Recommended: record all three.
8. The count of final handoffs in the index row on the sandbox's
   GitHub access lists checkpoints with
   `grep -h '^## Session checkpoint (' docs/reviews/round-*.md`, so the
   checkpoint on this page is not counted. Recommended: widen the
   pattern to `docs/reviews/*.md` in the docs pass after #45.

### Next steps

1. Get the answers to items 1 to 3 above.
2. Rebase docs/replan onto docs/spec-req, apply the answers to OD-14
   and OD-7, force-push docs/replan, update the body of #43 (stacked
   on #45, with its approval lines), and run both names gates.
3. CI green on #45 and #43.
4. The operator merges.
5. The sandbox-payload and external-repository handovers are tracked
   outside this repository.

Branch and pull request state is not recorded here; the next session
measures it with `~/.sbx-kit/claude-home/bin/handoff_state.py`.
