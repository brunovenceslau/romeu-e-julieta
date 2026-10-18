# 17. Clone a secondary repo inside the sandbox with julieta

Date: 2026-10-18

## Status

Proposed

## Context

A project may hold several repos, and one of them is primary. The
question is how a secondary repo reaches the sandbox so that the agent
can edit it. sbx can mount a host directory as an additional workspace,
and the agent then writes into a tree that host tools open: its
`.git/config`, its hooks and its `.vscode/tasks.json`.

The decision was taken in review round 1 and kept in round 2, where it
is listed as Q3 ([round 2](../reviews/round-2.md)). The specification
that holds it was added on 2026-09-30. It rests on sbx's documented
behavior at the time (additional workspaces are direct mounts) and
waits for no probe. This record was written on 2026-10-18, because the
specification cites the decision as authority
([02 2.6](../spec/02-layouts.md#26-multi-repo-projects-the-secondary-repo-decision)).

### Alternatives considered

- **An sbx additional workspace, a direct read-write mount.** The
  agent can edit, and the bytes are already on the host. Rejected: a
  high risk of host execution, since the agent writes files that host
  tools open.
- **A direct read-only mount.** No risk from that tree. Rejected as the
  default: it is not useful for work, because the agent cannot edit.
- **One sandbox per repo.** No host risk, and the agent can edit.
  Rejected: it breaks "a project may hold several repos".

## Decision

A secondary repo is a clone made inside the sandbox by julieta from its
origin. Nothing the agent writes there is mounted on the host, except
memory data. Unpushed work reaches the host only through the snapshot
and salvage bundles that are kept in the memory directory, and romeu
verifies and imports them. The host keeps an operator clone of each
secondary repo, used for navigation and for deriving egress.

## Consequences

- The sandbox needs the project's git credential to clone private
  repos and to push. Work reaches GitHub by push as usual.
- Work that was never pushed reaches the host only through snapshot and
  salvage, so the safety of that work depends on those two paths
  ([08](../spec/08-memory-handoff-salvage.md)).
- The mechanism, the credential and the probe that re-confirms the
  decision are in
  [02 2.6](../spec/02-layouts.md#26-multi-repo-projects-the-secondary-repo-decision).
- The reasons for choosing this clone over the three options are the
  rows of the table in that section; no other reasoning is written in
  the repository.
