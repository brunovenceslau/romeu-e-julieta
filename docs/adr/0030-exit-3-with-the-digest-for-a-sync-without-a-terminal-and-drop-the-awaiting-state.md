# 30. Exit 3 with the digest for a sync without a terminal and drop the awaiting state

Date: 2026-10-18

## Status

Proposed

## Context

A non-interactive `sync` had no named caller for the state in which a
candidate waits for approval, nor for `approve <name>`. Review round 8
raised this as item SC5 of [round 8](../reviews/round-8.md#decisions-for-the-operator). The decision supersedes, in part,
the candidate state machine of finding C2 of round 2.

### Alternatives considered

- **Keep the awaiting state and `approve <name>`.** Not taken: the
  non-TTY path has no named caller.
- **Exit 3 with the digest and keep no awaiting record.** Taken.

## Decision

Decided on 2026-10-08: a sync without a terminal exits 3 with the
digest and writes no awaiting record. The awaiting and superseded rows
of the state machine and `approve <name>` leave v1. One Deferred row
holds the approval of a candidate rendered without a terminal, reopened
by the first scripted sync.

## Consequences

- Approval needs a terminal in every path.
- The state machine of
  [01 1.6](../spec/01-system-model.md#16-state-machines) has fewer states.
- A scripted sync has no approval path until the Deferred row fires.
