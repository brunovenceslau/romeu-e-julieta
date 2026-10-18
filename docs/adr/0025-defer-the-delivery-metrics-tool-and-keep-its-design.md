# 25. Defer the delivery metrics tool and keep its design

Date: 2026-10-18

## Status

Proposed

## Context

A tool that computes the five delivery metrics was designed for this
repository
([ADR 0004, measure delivery with the five DORA metrics computed by a tool](0004-measure-delivery-with-the-five-dora-metrics-computed-by-a-tool.md)).
Review round 8 found (item SC12 of [round 8](../reviews/round-8.md#decisions-for-the-operator)) a five-metric tool for a
repository with one contributor and no release, and that the record of
the design itself says nothing is lost by waiting. The decision
supersedes op-dora-late (round 6) and leaves the round 4 decisions
op-practices, op-pairing-rule, op-backfill, op-maintainer-wait and
op-pr-size without their mechanism until the trigger.

### Alternatives considered

- **Keep the tool in v1.** Not taken.
- **Defer the tool**, keep the design and keep the one check whose data
  cannot be filled in later. Taken.

## Decision

Decided on 2026-10-08: the delivery metrics tool is deferred, as a
Deferred row. Until it lands, the three rows of the delivery practices
that cite a number are judged by review, from `gh pr list --json` read
by hand, and no number gates a merge. The check of the fix-marker
trailer stays in the pull request check, because its data cannot be
filled in later. The row is reopened by the third release that is not a
prerelease, or by the first lesson that wants a delivery number. The
decision also settled the delivery-metrics part of the question about
release immutability (DR6) and the metrics part of finding R8-12-38.

## Consequences

- ADR 0004 stays Accepted as the design of the deferred tool.
- The section on delivery metrics in the specification shrinks to one
  paragraph
  ([12 12.10](../spec/12-engineering.md#1210-delivery-metrics)).
- The plan loses the task that builds the tool and one step of the
  release workflow.
