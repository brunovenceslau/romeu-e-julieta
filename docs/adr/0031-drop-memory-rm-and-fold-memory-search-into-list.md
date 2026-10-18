# 31. Drop memory rm and fold memory search into list

Date: 2026-10-18

## Status

Proposed

## Context

`memory rm` is a delete path in a product whose memory store otherwise
never deletes an entry
([08 8.1](../spec/08-memory-handoff-salvage.md#81-memory-store)), and
`memory search` duplicates `list` with a filter. Review round 8 raised
both as item SC10 of [round 8](../reviews/round-8.md#decisions-for-the-operator).

### Alternatives considered

- **Keep `rm` and `search`.** Not taken.
- **Drop `rm` and fold `search` into `list`.** Taken. `edit --status
  done` stays as the way to close an entry.

## Decision

Decided on 2026-10-08: `memory rm` is dropped, and `memory search`
folds into `list --query`.

## Consequences

- julieta never deletes a memory entry; backups of memory are the
  owner's.
- The command set has two commands fewer.
