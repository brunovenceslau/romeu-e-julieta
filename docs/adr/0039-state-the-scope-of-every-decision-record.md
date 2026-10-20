# 39. State the scope of every decision record

Date: 2026-10-20

## Status

Proposed

It becomes Accepted in a later pull request, which adds the
`Supersedes in part` line for rule 9 of
[ADR 0001, adopt a documentation standard with checkable rules and a voice](0001-adopt-a-documentation-standard-with-checkable-rules-and-a-voice.md)
here and the matching `Superseded in part by` line there.

## Context

A rule decided in this repository can be meant for one session, for
this repository, for every repository and sandbox the maintainer works
in, or for every adopter of romeu and julieta. Nothing asks a record to
say which, so the scope is whatever place the rule happened to be
written. On 2026-10-20 the rule of
[ADR 0038, search for a maintained library before writing one](0038-search-for-a-maintained-library-before-writing-one.md)
was first written as a record of this repository only, although it was meant for every repository; it reached the sandbox
payload only because the question was asked in the session.

Each scope has its own home, and two of them move:

| Scope | Home today | Home once romeu and julieta replace the interim sandboxes |
|---|---|---|
| this session | none; it ends with the session | the same |
| this repository | its decision records, `docs/handoff/decisions/`, its checks | the same |
| every repository and sandbox of the maintainer | the sandbox payload (`kits/claude-home` of the payload's source repository), which is interim | the personal agent context of 06 (REQ-0608): a personal kit in the config repository |
| one other repository or machine | that repository's records, or that machine's settings | the same |
| every adopter of the product | this repository's product parts: the spec, the product kits, `skills/`, the catalog | the same |

What is at stake: a rule written in a narrower home than its scope does
not hold where it was meant to, and nothing may show it; a builder in
another repository keeps the old behavior until the same failure
repeats. When the interim payload is retired, a rule that lives only
there can be lost unless it was named for the move.
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

Proposed on 2026-10-20: rule 9 of ADR 0001 gains one more sentence.
**[review]** Every decision record states its scope in its Context,
after what is at stake, and a decision note under
`docs/handoff/decisions/` in its first paragraph, after the stakes: a
line `Scope:` naming one of the scopes of the table above, the widest
the rule is meant to hold in. A scope other than this repository also
names the home where the rule is written, or a pending note that
carries it there; a rule whose home is the interim payload names its
later home too, so the move finds it. A rule that fits two scopes names
both homes.

The same rule goes to the "How to work" section of the sandbox payload,
so it holds in every repository; the pending note of Consequences
tracks it.

## Consequences

- `docs/handoff/about.md` names the rule for decision notes.
- A record written before this one is not rewritten; its scope is
  added when it is next amended.
- The pending note "rules-to-move-when-the-interim-payload-is-retired"
  lists the rules this repository's records mean for the payload, with
  whether each was sent and its later home, so the move does not depend
  on memory.
