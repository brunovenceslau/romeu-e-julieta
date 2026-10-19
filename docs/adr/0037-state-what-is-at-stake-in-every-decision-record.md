# 37. State what is at stake in every decision record

Date: 2026-10-19

## Status

Accepted

Supersedes in part [1. Adopt a documentation standard with checkable rules and a voice](0001-adopt-a-documentation-standard-with-checkable-rules-and-a-voice.md) (rule 9)

## Context

The forbidden-names rule was recorded without what was at stake, and
later sessions sized their work by the strongest word they found
instead ([ADR 0036, keep the forbidden names as a convenience list in the config repository](0036-keep-the-forbidden-names-as-a-convenience-list-in-the-config-repository.md)). A record
in the layout of rule 6 of
[ADR 0001, adopt a documentation standard with checkable rules and a voice](0001-adopt-a-documentation-standard-with-checkable-rules-and-a-voice.md)
has a Context section, but nothing
asks it to say what happens when the decision turns out wrong, and the
decision notes of `docs/handoff/decisions/` have no context section at
all. A reader can then tell what was decided and when, and not how much
care it deserves.

What is at stake: effort spent out of proportion to the risk, as with
the names list, or too little care where the risk is high.

### Alternatives considered

- **Leave it to the author.** The state before this record; the names
  rule shows how it fails.
- **A checked field.** Not taken now: a `tools/ci` rule on the
  `checks` surface for a judgment a reviewer reads better. It is
  reopened if a record merges without the paragraph.
- **A review rule: every decision states its stakes.** Taken.

## Decision

Decided on 2026-10-19: rule 9 of ADR 0001 gains one sentence. **[review]**
Every decision record states what is at stake in its Context: what
happens if the decision is wrong or fails, and how hard that is to
undo, in plain words and with no stronger word than the facts carry.
The same holds for a decision note under `docs/handoff/decisions/`,
which says it in its first paragraph. A rule restated elsewhere (the
spec, a working rule, a resume prompt) links the record that holds its
stakes rather than restating its weight.

## Consequences

- `docs/handoff/about.md` names the rule for decision notes.
- A record written before this one is not rewritten; its stakes are
  added when it is next amended.
