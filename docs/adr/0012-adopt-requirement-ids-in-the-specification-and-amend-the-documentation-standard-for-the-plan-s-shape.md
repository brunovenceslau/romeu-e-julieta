# 12. Adopt requirement ids in the specification and amend the documentation standard for the plan's shape

Date: 2026-10-12

## Status

Proposed

Supersedes in part [ADR 0001, adopt a documentation standard with checkable rules and a voice](0001-adopt-a-documentation-standard-with-checkable-rules-and-a-voice.md) (rules 1, 5, 13 and 15)
Supersedes in part [ADR 0004, measure delivery with the five DORA metrics computed by a tool](0004-measure-delivery-with-the-five-dora-metrics-computed-by-a-tool.md) (Consequences)

## Context

The specification review's write pass of PR #36 changed about 1300 lines
across every chapter, and the plan received per-round patches and then
a re-plan driven by ordering requirements, never by a trace from each
obligation to the task that builds it. The plan review of 2026-10-12
(stage B of the lenses page) traced every obligation and found the
drift this record closes: obligations with no task, tasks citing
sections a round had rewritten, and rules of the documentation
standard that the plan's shape (one file per task, one `commands.yaml`
per journey, registration by file) had outgrown.

## Decision

1. Each independently checkable obligation of the specification carries
   a stable requirement id, `REQ-<nnnn>`, placed, allocated, retired and
   checked as [12 12.11](../spec/12-engineering.md#1211-requirement-ids)
   states: `tools/ci reqs allocate` numbers the placeholders
   in document order, `tools/ci reqs` checks the ids and, over the task
   files, their coverage; a retired id is recorded, never deleted or
   reused. Error ids (`RJ-<nnn>`) stay what they are. The plan's task
   files cite requirements by id. Citing requirement ids from code and
   tests is deferred to the first implementation task that needs it.
2. For the write pass of this review, craft findings (precision, voice,
   quantifiers, redundancy, one authoritative list, standalone
   readability) were applied alongside the contradictions and behavior
   gaps, reversing for that pass alone the stage-A rule that only the
   last two block.
3. ADR 0001, rule 1: the task files under `docs/plan/tasks/` are exempt
   from the front-matter rule; they have their own schema, checked by
   `tools/ci plan`.
4. ADR 0001, rule 5, first sentence: **[check]** Markdown lint, link
   check and spell check, external URLs excepted.
5. ADR 0001, rule 13: the commands a guide page documents have one
   committed source per journey, `e2e/scenarios/<journey>/commands.yaml`,
   and the scenario package's test reads all of them; the CI suite runs
   the journeys the opening of [09](../spec/09-journeys.md) lists (J2,
   J3b, J6, J7, J10 and J11, J6 at the hybrid level), and rule 13 cites
   that opening instead of repeating the list.
6. ADR 0001, rule 15: every package the two binaries import lives under
   `internal/` ([10 10.7](../spec/10-testing-style.md#107-module-boundaries-enforced-by-toolsci-imports)),
   so a doc comment alone cannot show a call contract to an outside
   reader, and Example tests are written where that matters.
7. ADR 0004, Consequences: a change to `tools/ci dora` needs the
   approval line of the `checks` surface; no review is required
   ([ADR 0009](0009-drop-the-required-review-and-amend-the-records-that-assumed-it.md), decision 1).
8. The deferred row of the index on generating the step table of 10 10.2
   is closed: `docs/reference/checks.md`, generated from the check
   registry, with the `generated` test that holds each registered check
   to its row, is the test that row named as its event.

## Consequences

- Every spec page carries ids after the write pass; a pull request that
  changes an obligation updates the task files that cite it, and
  `tools/ci reqs` turns red on a stale citation, so a one-line conflict
  in an in-flight worktree is found at its rebase.
- The stage-B criteria of `docs/reviews/lenses.md` read traceability
  through `tools/ci reqs` instead of a search of `plan.md`.
- ADR 0001 and ADR 0004 gain their `Superseded in part by` lines.
