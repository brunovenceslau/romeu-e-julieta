# 29. Keep romeu pull, the review checkout and the workspace files in v1

Date: 2026-10-18

## Status

Proposed

## Context

`romeu pull`, the review checkout and the workspace files are a
host-side review path. Review round 8 raised them as item SC4 of
[round 8](../reviews/round-8.md#decisions-for-the-operator): the start does not need them, since the pull request is the
review ([ADR 0008, let the sandbox act as the maintainer on GitHub](0008-let-the-sandbox-act-as-the-maintainer-on-github.md) makes it so), and `dev.code-workspace` is opened by no
journey. Removing them would also remove the hardening that rounds 1
and 6 built on `pull` (findings L1, D6, the I19 test and R3-security-08)
and two rows of 05 5.4.

### Alternatives considered

- **Defer `romeu pull`, the review checkout and `review.code-workspace`**
  as one Deferred row, delete `dev.code-workspace`, and remove the two
  05 5.4 rows. This was the recommendation of the consolidation, and it
  was not taken.
- **Keep them in v1.** Taken.

## Decision

Decided on 2026-10-08, against the recommendation: `romeu pull`, the
review checkout and the workspace files stay in v1, with the hardening
that rounds 1 and 6 built on them. The deferral, the deletion of
`dev.code-workspace` and the removal of the two 05 5.4 rows are not
taken.

## Consequences

- The commands, their tests and the hardening stay specified and built.
- The reasons for keeping them are not written in round 8; the
  recorded decision is the choice and what it does not take.
- The one-sentence statement of the daily setup (edit and debug inside
  the sandbox; host clones are for reading in Restricted Mode) applies
  now, as the consolidation proposed.
