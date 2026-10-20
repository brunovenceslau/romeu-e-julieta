# 39. State the scope of every decision record

Date: 2026-10-20

## Status

Proposed

Supersedes in part [1. Adopt a documentation standard with checkable rules and a voice](0001-adopt-a-documentation-standard-with-checkable-rules-and-a-voice.md) (rule 9)

## Context

A rule decided in this repository can be meant for one session, for
this repository, for every repository and sandbox the maintainer works
in, or for every adopter of romeu and julieta. Nothing asks a record to
say which, so the scope is whatever place the rule happened to be
written. On 2026-10-20 the rule to search for a maintained library
before writing one was first written as a record of this repository
only, although it was meant for every repository; it reached the sandbox
payload only because the question was asked in the session.

Each scope has its own home, and two of them move:

| Scope | Home today | Home once romeu and julieta replace the interim sandboxes |
|---|---|---|
| this session | none; it ends with the session | the same |
| this repository | its decision records, `docs/handoff/decisions/`, its checks | the same |
| every repository and sandbox of the maintainer | the sandbox payload (`kits/claude-home` of the payload's source repository), which is interim | the personal agent context of 06 (REQ-0608): a personal kit in the config repository |
| every adopter of the product | the product kits and `skills/` of this repository | the same |

What is at stake: a rule written in a narrower home than its scope does
not hold where it was meant to, and nothing shows it; a builder in
another repository keeps the old behavior, and the gap is found only
when the same failure repeats. When the interim payload is retired, a
rule that lives only there is lost unless it was named for the move.
Writing the scope down costs one line per record and is undone by
editing it.

Scope: every repository and sandbox of the maintainer; its later home
is named in the pending note of Consequences.

### Alternatives considered

- **Leave the scope to the author.** The state before this record; the
  case of 2026-10-20 shows how it fails.
- **A checked field.** Not taken now, for the reason
  [ADR 0037, state what is at stake in every decision record](0037-state-what-is-at-stake-in-every-decision-record.md)
  gives for its own paragraph: a reviewer reads a judgment better than
  a rule on the `checks` surface. Reopened if a record merges without
  its scope.
- **A review rule: every record states its scope and its homes.** Taken.

## Decision

Decided on 2026-10-20: rule 9 of ADR 0001 gains one more sentence.
**[review]** Every decision record and every decision note under
`docs/handoff/decisions/` states its scope next to what is at stake:
one of the four scopes of the table above. A scope wider than this
repository also names the home outside it where the rule is written,
or a pending note that carries it there; a rule whose home is the
interim payload names its later home too, so the move finds it.

## Consequences

- `docs/handoff/about.md` names the rule for decision notes.
- A record written before this one is not rewritten; its scope is
  added when it is next amended.
- The pending note "rules-to-move-when-the-interim-payload-is-retired"
  lists the payload's rules that come from this repository's records,
  with their later home, so the move does not depend on memory.
