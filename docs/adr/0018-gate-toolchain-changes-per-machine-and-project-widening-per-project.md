# 18. Gate toolchain changes per machine and project widening per project

Date: 2026-10-18

## Status

Proposed

## Context

romeu changes what a sandbox can reach and what runs on the host, and
sbx already asks for approval of its own host commands. The question is
where romeu itself asks the operator, and how often. The specification
states that sbx updates itself, and that romeu or sbx can be replaced by
a package manager run or by another process or person on the machine; a
catalog or julieta change that arrives that way can widen egress for
every project
([01 1.4](../spec/01-system-model.md#14-the-gates)).

Both gates are in the first specification commit of 2026-09-30.
Gate 1 comes from the finding D3 of [round 1](../reviews/round-1.md),
the per-machine toolchain acknowledgement, and the findings N4 and N13
of [round 2](../reviews/round-2.md) shaped what it prints and how
`status` reports it. Gate 2 comes from the findings H1 and H2 of
[round 1](../reviews/round-1.md): capabilities in a widening set, and
candidates promoted only on approval. Gate 1 was kept after a question
in round 8, item SC8 of [round 8](../reviews/round-8.md#decisions-for-the-operator),
decided on 2026-10-08. This record
was written on 2026-10-18, because the specification cites the decision
as authority.

### Alternatives considered

- **Gate every project on every toolchain change.** The specification
  says gate 1 "keeps an sbx or romeu upgrade from turning into a gate
  on every project".
- **Fold gate 1 into `sync`.** Considered in round 8 and not taken: it
  would weaken a security gate, and it would remove the findings that
  gate 1 answers.
- **Rely on sbx's own approval alone.** sbx re-asks when a host command
  changes, and the specification adds that romeu "gates only what sbx
  cannot see", which is why two romeu-level gates exist.

## Decision

There are two romeu-level approvals, both at a terminal. Gate 1 is the
toolchain acknowledgement, once per machine and per change.
Gate 2 is the widening of a project, once per project. What each gate
records and which fields are gated are in the specification. romeu does not bypass sbx's
own approval of host commands. Approvals are per machine by design.

## Consequences

- An upgrade is acknowledged once on a machine, and a project is then
  asked only about its own widening.
- Gate 1 guards a machine where the operator upgrades against a binary
  or a catalog replaced behind the operator's back.
- Both approvals need a terminal. Round 8 decided that a sync without
  one exits 3 with the digest (item SC5,
  [ADR 0030, exit 3 with the digest for a sync without a terminal and drop the awaiting state](0030-exit-3-with-the-digest-for-a-sync-without-a-terminal-and-drop-the-awaiting-state.md)).
- Which fields are gated is declared once in the spec types, and the
  specification holds the details.
- No review file or note weighs the number of gates against other
  numbers; this record names only the alternatives that the
  specification and round 8 state.
