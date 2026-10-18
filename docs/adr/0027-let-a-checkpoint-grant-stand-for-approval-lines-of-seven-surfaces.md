# 27. Let a checkpoint grant stand for approval lines of seven surfaces

Date: 2026-10-18

## Status

Proposed

## Context

Most pull requests carry an approval line in the maintainer's words.
Review round 8 raised the volume as item AR14 of [round 8](../reviews/round-8.md#decisions-for-the-operator). It adds a
second form to the one that round 3 decided (N2). The consolidation
proposed a grant for a list of surfaces; the decision removed one of
them.

### Alternatives considered

- **Per-pull-request words on every surface**, as before. Kept for the
  surfaces not granted.
- **A grant that includes `kits`.** The consolidation's first
  proposal. Not taken: `kits` carries pins, so it stays per pull
  request.

## Decision

Decided on 2026-10-08: a sitting grant covers the approval lines of the
`contracts`, `checks`, `dependencies`, `catalog`, `ledger`, `digests`
and `termsafe` surfaces, for a list of tasks. Every other surface keeps
an approval line in the maintainer's words on each pull request.
Decided on 2026-10-09 (ship gate): the grants are kept in a file on the
`ask-first` surface, and the pull request check reads that file at the
base commit, not the plan, which is on no surface. Amended on 2026-10-09
(ship gate round 2): a new `approvals` surface holds the code and the
denylist of the ask-first check, and no grant covers it. The file and
its form are in
[12 12.4](../spec/12-engineering.md#124-middleware-before-and-after-every-change).

## Consequences

- A grant is itself an ask-first change, in its own pull request.
- The check proves that the approval was recorded, not who gave it
  ([12 12.4](../spec/12-engineering.md#124-middleware-before-and-after-every-change)).
