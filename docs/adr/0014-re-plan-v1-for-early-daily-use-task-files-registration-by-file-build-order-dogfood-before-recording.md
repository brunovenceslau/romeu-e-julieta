# 14. Re-plan v1 for early daily use: task files, registration by file, build order, dogfood before recording

Date: 2026-10-17

## Status

Proposed

Supersedes in part [ADR 0005, decide at the last responsible moment and record the trigger](0005-decide-at-the-last-responsible-moment-and-record-the-trigger.md) (rule 1)

## Context

The plan was re-planned on 2026-10-12, at the end of stage A of the
specification's review, from an earlier plan of 103 tasks in ten
phases. The re-plan set six requirements, daily use first among them,
and its reasons are in
[How this plan was made](../plan.md#how-this-plan-was-made). It opened
a block of decisions,
[Decisions of the re-plan](../plan.md#decisions-of-the-re-plan), whose
answers came on 2026-10-12, 2026-10-13 and 2026-10-17. The amendments
it carried into the specification were applied by T122 under
[ADR 0012, adopt requirement ids in the specification and amend the documentation standard for the plan's shape](0012-adopt-requirement-ids-in-the-specification-and-amend-the-documentation-standard-for-the-plan-s-shape.md).

Until now the answers lived only in that block. A few of them replace
earlier decisions, and one of those is a rule of a decision record: the
re-plan keeps parts of the merge gate deferred, which rule 1 of
ADR 0005 says are never deferred. 05 5.4 accepted that collision by
name on 2026-10-07, and no record had amended the rule. This record
states the practices the re-plan adopts, the decision each one replaces
and the event that reopens it.

### Alternatives considered

- For the shape of the plan, a plan document plus a separate checklist
  file, the convention of the planning skill in use: declined because
  a second checklist is a second source of the same list
  ([Where the task checklist lives](../plan.md#where-the-task-checklist-lives)).
- For the registries, the specification as it was written: sorted
  generated tables with regenerate-on-merge (item 1).
- For the build order, block A as one two-host sitting before M3's
  lane B, about 2.5 hours on the critical path (item 3); and M4 before
  the slots of sitting M3, which delays M3's value by M4's tasks
  (item 7).
- For the fake's sessions, a recorder seam in `sbxdrv` with the
  redaction package landed early, declined for the irreversible-harm
  reason of rule 10 (item 4).
- For rule 1 of ADR 0005, leaving it as written and the collision
  accepted only in 05 5.4: declined because a reader of the record
  would find a rule the repository does not follow.

## Decision

1. **The plan's shape: task files.** The plan is `docs/plan.md` plus
   one file per task under `docs/plan/tasks/`; the task files are the
   checklist, and `tools/ci plan` checks them in `fast` (item 2 of
   Decisions of the re-plan, decided on 2026-10-12). It replaces the
   answer of 2026-10-01 to plan question 1, `docs/plan.md` as one page
   with its ticks inline. Reopened by: the first task file whose
   `merged` disagrees with the merged pull requests (the row "deriving
   `merged` from merged pull requests" of
   [Decisions this plan defers](../plan.md#decisions-this-plan-defers)).
2. **Registration by file.** Each `tools/ci` step and each command
   registers itself from its own file's `init` (item 1, decided on
   2026-10-12; T105 split the packages), and the reference pages are
   generated from the registries (item 5 and 12 12.3). It replaces
   sorted generated dispatcher tables with regenerate-on-merge, the
   specification as written before 2026-10-12. Reopened by: the first
   check or command present in the tree that its registry does not
   list at run time (the row "generated dispatcher tables" of
   Decisions this plan defers, added with this record).
3. **Build order for early daily use.** julieta's daily commands and
   the kits come before `sync`; block A is the last word, in M6; the
   smoke steps are per-project slots; render and run come before
   salvage (items 3 and 7, decided on 2026-10-12). It replaces the
   order of the earlier plan of ten phases. Reopened by: the first
   uncommitted change lost to a recreate, which moves T068 ahead of any
   remaining M3 task (the first row of
   [Accepted risks](../plan.md#accepted-risks)).
4. **Dogfood before recording.** The sbx sessions the fake replays are
   written by hand and marked `synthetic: true` until the probe harness
   records them, and no recorder is built before the harness (item 4,
   decided on 2026-10-12). It replaces recordings from block A as the
   fake's source before the harness. Reopened by: a real-sbx behaviour
   the fake cannot model that breaks a daily run; or the first release
   candidate (the row of the recorder and the probe harness in
   Decisions this plan defers).
5. **Merge-gate parts deferred by name.** Rule 1 of ADR 0005, as this
   record amends it, reads: a security invariant and anything a later
   step depends on are never deferred; a part of a merge gate is
   deferred only when
   [05 5.4](../spec/05-security.md#risks-of-how-this-repository-is-developed)
   accepts the risk by name and a row of
   [Deferred decisions](../spec.md#in-how-this-repository-is-run)
   holds its trigger. ADR 0005's own text is unchanged; its Status
   points here. The parts so deferred are the six that 05 5.4 names: a
   check that reads the rulesets again, a signed approval record,
   running the change's tests in a separate job, `tools/ci pr`,
   `tools/ci sequences`, and `tools/ci invariants` with `mutate`. The
   list is closed: deferring a further part needs a record that amends
   this decision. For `invariants` and `mutate`, only the automated
   tag pairing and mutation run are deferred; each invariant's guard
   and its tests land with the code they guard. Until each part lands,
   the maintainer reads its inputs at merge. Reopened by: the trigger
   of each part's row in Deferred decisions.
6. **The rest of the block.** Every other answer stands as
   [Decisions of the re-plan](../plan.md#decisions-of-the-re-plan)
   records it, and this record does not copy them. Items 8, 9 and 11,
   the ask-first surfaces, were decided on 2026-10-17 and applied by
   T107. Items 10, 16 and 20 are deferred decisions, each with the
   sitting that decides it (M0, M0 and M2), so no item of the block is
   left open.
7. **After this record**, a change to `docs/plan.md` is a re-plan pull
   request of its own (rule 1 of
   [How the work is done](../plan.md#how-the-work-is-done)).

## Consequences

- ADR 0005 gains its `Superseded in part by` line for rule 1. Its other
  rules stand: a deferral still names what holds until then and an
  observable event.
- Deferring a merge-gate part is no longer a silent exception: it needs
  a risk row in 05 5.4, a trigger row and a record, and the
  maintainer's reading at merge is the gate until the trigger fires.
  The cost is that a mistake those checks would catch reaches `main`
  when the maintainer misses it.
- The practices in decisions 1 to 4 are reopened by events, not by a
  later review round; a pull request that meets one of those events
  names this record.
