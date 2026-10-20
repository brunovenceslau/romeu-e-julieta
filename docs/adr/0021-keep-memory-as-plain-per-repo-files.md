# 21. Keep memory as plain per-repo files

Date: 2026-10-18

## Status

Accepted

## Context

julieta stores what a session learns, as memory entries and handoffs,
and the specification describes the store in
[08 8.1](../spec/08-memory-handoff-salvage.md#81-memory-store). The
`julieta-claude` kit disables the agent's built-in memory, so that the
agent consumes julieta's memory instead of remembering to read it
([08 8.1](../spec/08-memory-handoff-salvage.md#81-memory-store)). A
memory server is a non-goal of v1 ([00 0.3](../spec/00-scope.md#03-non-goals-v1)).
The layout allowlist, the caps and the refusal on violation came with
review round 1 (D8 of [round 1](../reviews/round-1.md)).

This record was written on 2026-10-18, because the specification cites
the decision as authority.

### Alternatives considered

- **An external memory server.** The specification gives the reason: it
  has no per-project authorization, and its data directory must not be
  mounted over virtiofs.
- **Mounting the agent's own memory directory from the host.** A
  non-goal of v1.

## Decision

Memory is a directory of plain markdown files with YAML front matter,
one directory per repo per project, mounted into that project's sandbox
only. A layout allowlist and size caps are enforced, and romeu reads
the store only through readers that stay inside it. julieta never
deletes an entry. The mechanism is in
[08 8.1](../spec/08-memory-handoff-salvage.md#81-memory-store).

## Consequences

- Isolation holds by construction, and a server per repo stays possible
  later because the backend is behind an interface.
- Memory is machine-local in v1 (Q14), and backups of it are the
  owner's.
- The store is plain files, so no agent-specific field exists in it.
- The specification and the reviews record no alternative beyond those above.
