# 32. Merge a green pull request at the next sitting, with one merge per business day

Date: 2026-10-18

## Status

Proposed

## Context

The rule against stacked pull requests and the maintainer's merge of
each pull request make the calendar path of the plan equal to the depth
of its graph, counted in sittings. Review round 8 raised this as item
SC14 of [round 8](../reviews/round-8.md#decisions-for-the-operator). The cadence and the number N commit the operator's
time, so the operator decides them.

### Alternatives considered

- **Leave the cadence open.** The state before the decision.
- **Fix a cadence and N.** Taken.

## Decision

Decided on 2026-10-08: consecutive dependent tasks of one chain by one
agent may ship as one pull request, and the trigger of the Deferred row
on automated merge gates counts the first checkpoint with more than N
pull requests waiting, with N = 3. A green pull request with its approval
lines is merged at the operator's next sitting, with at least one merge
per business day. The rules are in
[12 12.9](../spec/12-engineering.md#129-delivery-practices).

## Consequences

- The merge stays a convention, since the token can merge too (05 5.4).
- The operator commits time: one merge per business day at least.
