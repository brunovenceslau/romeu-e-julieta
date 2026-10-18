# 35. Show lessons to the owner with a pager that runs nothing

Date: 2026-10-18

## Status

Proposed

## Context

`romeu handoff` shows only handoffs, so the owner has no way to read
the lessons that agents write into memory. Review round 8 raised this
as item AR10 of [round 8](../reviews/round-8.md#decisions-for-the-operator). The smaller form loosens finding D8 of
round 1 (read memory only through `romeu handoff`, no host extraction)
for one case, and the operator decides that by name.

### Alternatives considered

- **Leave lessons unreadable by romeu.** The state before the decision.
- **Add `romeu memory list|show`.** Not taken now; a Deferred row holds
  it.
- **A pager that shows bytes and runs nothing, never a workspace
  folder.** Taken.

## Decision

Decided on 2026-10-08: the owner reads lessons through a pager that
shows bytes and runs nothing. `romeu memory list|show` is a Deferred
row. The rule of D8 is loosened for the byte-only pager, by name.

## Consequences

- The host still extracts nothing and runs nothing from memory.
- Memory stays a steering channel labelled as agent-written text
  (item AR11).
