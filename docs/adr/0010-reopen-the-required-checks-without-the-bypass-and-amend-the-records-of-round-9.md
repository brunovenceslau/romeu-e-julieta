# 10. Reopen the required checks without the bypass and amend the records of round 9

Date: 2026-10-09

## Status

Accepted

Supersedes in part [1. Adopt a documentation standard with checkable rules and a voice](0001-adopt-a-documentation-standard-with-checkable-rules-and-a-voice.md)

Supersedes in part [6. Record runtime events in an add-only ledger ingested on the host](0006-record-runtime-events-in-an-add-only-ledger-ingested-on-the-host.md)

Supersedes in part [8. Let the sandbox act as the maintainer on GitHub](0008-let-the-sandbox-act-as-the-maintainer-on-github.md)

Supersedes in part [9. Drop the required review and amend the records that assumed it](0009-drop-the-required-review-and-amend-the-records-that-assumed-it.md)

## Context

Review round 9, the targeted re-audit of the round-8 write pass, sent
five items to the operator
([round 9](../reviews/round-9.md#decisions-for-the-operator)). Two of
them land in decision records, and both were decided on 2026-10-09,
with the recommendation of the round page in each case.

The first is a trigger that fired. Deferred item 4 of
[ADR 0008, let the sandbox act as the maintainer on GitHub](0008-let-the-sandbox-act-as-the-maintainer-on-github.md),
"Required checks without the bypass", is reopened by item e of the
maintainer block, and the matching row of the spec's
[Deferred decisions](../spec.md#deferred-decisions) by "the jobs of the
green run are added as required checks". As read through the GitHub
API on 2026-10-09
([the note on the maintainer block](../notes/maintainer-block-of-ci-bootstrap.md)),
the default-branch ruleset has a required-status-checks rule that lists
the four `all on ...` checks, and the administrator role, which the
development sandbox's token holds, is still its bypass actor. Item e
has therefore happened, and the decision it reopens has not been taken.

The second is four texts in accepted records that the spec has since
corrected, which round 9 found (L0-r9-4, L2-r9-5, L13-R9-4,
L13-R9-9). An accepted record is not rewritten
([12 12.5](../spec/12-engineering.md#125-decisions-and-history)), so
they change here.

### Alternatives considered

- **Rewrite both triggers to a later event**, such as the token
  narrowing. It keeps the decision deferred, and it would hide that the
  event the record named has happened. Declined on 2026-10-09.
- **One record per amended text.** Four records for one review round;
  one record that names each part keeps the cause in one place, as
  [ADR 0009, drop the required review and amend the records that assumed it](0009-drop-the-required-review-and-amend-the-records-that-assumed-it.md)
  did for round 8.

## Decision

### Reopened

1. **Required checks without the administrator bypass is reopened.**
   Its trigger fired, as read on 2026-10-09. The options are open:
   (a) the required checks bind the administrator role too, so no merge,
   the operator's included, lands without them; (b) the bypass stays,
   with the reason recorded and a new trigger. What each costs the
   operator's own merges, and what each is worth while one account
   acts for everyone
   ([05 5.4](../spec/05-security.md#risks-of-how-this-repository-is-developed)),
   is weighed in that item. The next step is one item in the next
   decision block, with the ruleset's API answer of item e under its
   Evidence; until it is decided, the bypass stands, as the Deferred
   row states. This replaces deferred item 4 of ADR 0008.

### Amended texts

2. **ADR 0006, its "v1".** The runtime ledger is deferred out of v1
   (the Deferred decisions row "the runtime ledger"). Where ADR 0006
   says "v1", "a v1 event" or "Rejected for v1", it means the first
   version that builds the ledger; the record stays the ledger's design
   (L0-r9-4).
3. **ADR 0009, the pre-push hook bullet of its Consequences.** Until PR
   #34 lands, the guard against an untracked `go.work` or `vendor/` and
   a caller's `GOFLAGS` is the steps' environment of
   [10 10.2](../spec/10-testing-style.md#102-ci), built from nothing
   with `GOWORK=off` and `GOFLAGS=-mod=readonly`; what stays open is the
   build of `tools/ci` itself, which the hook start of PR #34 closes.
   This replaces "the pull request's diff review is the only guard"
   (L2-r9-5).
4. **ADR 0001, its "Where lessons live" row.** Lessons live one file
   each under `docs/lessons/`
   ([12 12.7](../spec/12-engineering.md#127-text-standard-and-lessons)),
   not in `docs/lessons.md` (L13-R9-4).
5. **ADR 0009, decision 5.** `tools/ci all` reads the network only where
   [10 10.1](../spec/10-testing-style.md#101-test-levels) lists, which is
   the one list of the network reads of the tests and the steps
   (L13-R9-9).
6. **ADR 0009, decision 18, the mark of a final handoff.** A final
   handoff is a level-2 heading `## Session checkpoint (<date>)` in a
   `docs/reviews/round-<n>.md`, and later a `--final` handoff file once
   julieta runs in the development sandbox; the spec's token-narrowing
   row holds the command that counts them and the count (R9-24).

## Consequences

- ADRs 0001, 0006, 0008 and 0009 gain a `Superseded in part by` line
  and stay Accepted; their text is not edited.
- The Deferred decisions row of the required checks says it is
  reopened by this record, and the decision waits for its item.
- Nothing changes in the rulesets or in any check: this record moves a
  decision from deferred to open and corrects four texts.
