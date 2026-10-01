# 3. Adopt six Extreme Programming practices and review as pairing

Date: 2026-10-01

## Status

Proposed

## Context

The maintainer works in the Extreme Programming (XP) tradition and
asked for its practices to be written into the spec: test-first,
simple design, continuous refactoring, continuous integration, small
releases, collective ownership, and the loop of an agent and a
reviewer as the analogue of pairing.

The source is Kent Beck's book, in two editions that name the
practices differently. We say plainly what we could check:

- Kent Beck with Cynthia Andres, *Extreme Programming Explained:
  Embrace Change*, 2nd ed., Addison-Wesley, 2004 (ISBN 0-321-27865-8),
  and the first edition of the same book. Both are cited as
  bibliography only. We have not checked a copy of either, so this
  record quotes no sentence, gives no page and no chapter, and the
  second-edition practice names below come from secondary
  descriptions of the book.
- One mapping between the editions has a primary source we read:
  Martin Fowler, [Yagni](https://martinfowler.com/bliki/Yagni.html),
  26 May 2015: "Yagni is a way to refer to the XP practice of Simple
  Design (from the first edition of The White Book, the second edition
  refers to the related notion of 'incremental design')."
- For continuous integration we read Martin Fowler,
  [Continuous Integration](https://martinfowler.com/articles/continuousIntegration.html),
  18 January 2024: "Continuous Integration is a software development
  practice where each member of a team merges their changes into a
  codebase together with their colleagues' changes at least daily.
  Each of these integrations is verified by an automated build
  (including test) to detect integration errors as quickly as
  possible."

| Practice the maintainer named | Edition and name | Their decision, as far as we could check |
|---|---|---|
| test-first | 2nd ed., Test-First Programming (primary practice) | the test is written before the code it tests |
| simple design | 1st ed., Simple Design; 2nd ed., Incremental Design (primary practice); the one first-edition name we could check, on Fowler's page | design for today's need and grow it (Fowler's page, above) |
| continuous refactoring | 2nd ed., Incremental Design (primary practice), which is where the secondary descriptions place it | improve the design of existing code as part of each change |
| continuous integration | 2nd ed., Continuous Integration (primary practice) | a cadence: integrate and test every couple of hours. It is not a rule about when to release. Fowler's definition says at least daily, each integration verified by an automated build |
| small releases | 2nd ed., Incremental Deployment and Daily Deployment (corollary practices) | release often, in small steps |
| collective ownership | 2nd ed., Shared Code (corollary practice) | anyone on the team may change any code |

Their why, and what it cost them: we cannot state either in the
book's words without a copy, and we will not invent them. What we can
say is ours. These practices lean on each other: refactoring without
tests is a gamble, and collective ownership without integration is a
merge conflict. A team that adopts one alone pays for the others it
skipped.

Does their constraint hold for us? XP was written for a team of
people in one room. We are one maintainer and agents that share no
room and no memory between sessions. That changes which practices
need a machine behind them. An agent does not get tired of writing
the test first, and it does forget, in a new session, why the code is
shaped as it is. So here the tests, the checks and the written
record carry what a team's shared memory carried there. This
paragraph is our reasoning; no source says it.

### Alternatives considered

- **Adopt the second edition whole.** It names more practices than
  the maintainer listed, several about a team that sits together. We
  cannot check the list against a copy, and practices about a shared
  room do not describe us. Rejected; we adopt six and say which.
- **Name no method; keep only the checks.** The checks say what; they
  do not say why a contributor should write the test first when no
  check can see the order. A named practice with its source gives the
  review judgment something to stand on. Rejected.
- **Pair programming as the book has it.** Two people at one machine
  is not something agents and one maintainer can do. We keep what we
  think the practice is for, a second reader before the code lands,
  and say that this reading is ours. See the decision.
- **Enforce test-first with a check on commit order.** A check that a
  test file's commit comes before the code's would be satisfied by
  an empty test, and squashing erases the order. Rejected; the order
  is a review judgment, and what a machine can prove (a guard has a
  test that fails without it) is already checked (05 5.2).

## Decision

We adopt six XP practices. Each rule, with its check or its review
tag, is in
[12 12.9](../spec/12-engineering.md#129-delivery-practices):

1. **Test-first.** The test is written before the code, and a bug fix
   starts with a test that fails for the bug.
2. **Simple, incremental design.** Build what the task in hand needs.
   The spec's principle "Simple and explicit" is how we count it, and
   [ADR 0005, decide at the last responsible moment and record the trigger](0005-decide-at-the-last-responsible-moment-and-record-the-trigger.md)
   is how we defer the rest.
3. **Continuous refactoring.** Each change leaves the code it touches
   better, in the same pull request, with the tests green before and
   after.
4. **Continuous integration.** Work is integrated into `main` and
   tested often: a pull request is small, runs the full check list,
   and merges. Continuous integration is this cadence. It is not the
   rule "do not release until tests pass", though we have that rule
   too (10 10.2).
5. **Small releases.** A release is one tag, built by one workflow,
   and we would rather cut two small ones than a large one.
6. **Collective ownership.** No file belongs to a person or to the
   agent that wrote it. Anyone may change any code through a pull
   request.

And one analogy, which is ours and is cited to no one: **the loop of
an agent and a reviewer stands in for pairing.** The agent that wrote
a change is not its only reader. A reviewer, another agent or a
person, reads it before it merges, and the default-branch ruleset
requires one approving review. We do not claim the book describes
this.

Why this fits us: our first principle puts the mechanical part of
each practice in a check and leaves the judgment to a reviewer, and
these six split cleanly along that line.

## Consequences

- No new check. Rules 1, 2, 3 and 6 are review judgments supported
  by checks that already exist: the invariant and mutation checks
  for tests that bite, coverage (S9), the Middleware line of the
  pull request body, and the generators that keep one source per
  rule. Rule 4 rests on
  [ADR 0002, develop on the trunk with short-lived branches](0002-develop-on-the-trunk-with-short-lived-branches.md)
  and the merge ruleset; rule 5 on
  `release.yml`.
- Collective ownership meets `.github/CODEOWNERS`, which names the
  maintainer for each ask-first path. That file is an approval gate,
  not a claim that the path is one person's code: anyone may propose
  the change, and the maintainer must approve it. We keep both and
  say so here, because the word "owner" invites the confusion.
- The review that stands in for pairing is worth what the reviewer is
  worth. With one GitHub account it is not independent, which the
  spec records as a residual risk (05 5.4).
- If someone checks a copy of the book and a practice name or an
  edition above is wrong, the correction is a new record that
  supersedes this one.
- This record stays Proposed until the maintainer signs it off. What
  holds until then is Q24 in the spec's
  [Open questions](../spec.md#open-questions).
