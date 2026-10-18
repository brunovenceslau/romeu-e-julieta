# 34. Add no user-facing success criterion in v1

Date: 2026-10-18

## Status

Proposed

## Context

Three clusters of round 8 took two positions on one scope question:
whether v1 has a success criterion that a stranger can meet (S12). One
proposed S12, recorded counts of operator commands and prompts and wall
clock on both block B hosts. Two declined it, one by stating in 00 0.2
that acceptance runs only with the maintainer's config repo and
deferring the stranger-run criterion, one by adding the journey J14,
stopping the use of romeu and julieta, under S7. A success criterion is
scope, which only the operator adds or declines. Review round 8 raised
this as item SC16 of [round 8](../reviews/round-8.md#decisions-for-the-operator).

### Alternatives considered

- **Add S12**, with a bound set now. Not taken: a bound set from a
  measurement is the principle the proposals were judged by.
- **Add S12 in a smaller form**, as recorded counts and a review
  comparison, with no invented bound. Not taken as a criterion; its
  counts became fields of a result.
- **Add no S12.** Taken.

## Decision

Decided on 2026-10-08: v1 has no S12. The counts become fields of the
B3 result and are compared with 00 0.2 in the review of the acceptance
pull request. 00 0.2 states the limit, and a Deferred row names a
user-facing success criterion as the decision that the first B3
measurement or the first adopter issue reopens. J14 lands under S7.

## Consequences

- v1 is accepted with the maintainer's config repo only.
- A stranger's first day is measured by B3 but binds nothing.
- The Deferred row reopens the question on a measurement or an issue.
