# 23. Defer the runtime ledger out of v1

Date: 2026-10-18

## Status

Proposed

## Context

The runtime ledger was designed as an add-only record of events inside
sandboxes, ingested on the host
([ADR 0006, record runtime events in an add-only ledger ingested on the host](0006-record-runtime-events-in-an-add-only-ledger-ingested-on-the-host.md)).
Review round 8 found (item SC1 of [round 8](../reviews/round-8.md#decisions-for-the-operator)) that the ledger traces to
no success criterion, journey or scope row; that ADR 0006 has no "not in
v1" alternative; and that the build order puts it before `romeu run`.
It reverses the round 5 decision to accept the design as a whole
(op-ledger and the eleven op-ledger-* decisions).

The cost of waiting, by the rule of
[ADR 0005, decide at the last responsible moment and record the trigger](0005-decide-at-the-last-responsible-moment-and-record-the-trigger.md):
events before the ledger exists are not recorded, and its two mounts
cost one recreate per project later
([13](../spec/13-runtime-ledger.md)).

### Alternatives considered

- **Keep the ledger in v1**, with a scope row that names the need and
  the rejected alternative written into ADR 0006. Not taken on
  2026-10-08.
- **Defer it**, as one Deferred row. Taken.

## Decision

Decided on 2026-10-08: the runtime ledger leaves v1. The page of the
ledger (13) stays, designed and not built. The Deferred row says that, until the
ledger exists, julieta state and each command's stderr are the record,
and that the row is reopened by the first lesson that names an
in-sandbox failure julieta state could not reconstruct, or by a second
machine. The mounts, the ingest steps, the ledger rows of `doctor`, the
ledger invariants and the plan tasks that build it leave v1. The same
decision made three other items moot: the ledger's inherited risks and
starting values, a retired name reused in the ledger, and owner removal
from the ledger (AR7, AR19 and AR20).

## Consequences

- ADR 0006 stays Accepted as the design of the deferred ledger.
- The `ledger` surface of the ask-first list stays listed and matches
  nothing until the row fires.
- The two mounts cost one recreate per project when the ledger lands.
- Events before that are not recorded.
