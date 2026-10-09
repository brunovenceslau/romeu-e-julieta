# 2. Develop on the trunk with short-lived branches

Date: 2026-10-01

## Status

Accepted

Superseded in part by [9. Drop the required review and amend the records that assumed it](0009-drop-the-required-review-and-amend-the-records-that-assumed-it.md)

## Context

Romeo and Juliet came from two houses that kept to themselves for so
long that nobody remembered how to merge them. We have one house, the
default branch, and several writers: one maintainer and the agents
that work in parallel sandboxes. An agent produces a branch in an
hour, so branches that wait pile up faster here than in a team of
people. The spec needs a branching rule before the first line of code,
because the checks, the release workflow and the metrics all assume
one.

The maintainer's practice is trunk-based development. This record
says what the sources decided, what it cost them, and what we take.

| Source | Their decision | Their why | What it cost them |
|---|---|---|---|
| Paul Hammant, [trunkbaseddevelopment.com](https://trunkbaseddevelopment.com/) | "A source-control branching model, where developers collaborate on code in a single branch called 'trunk' and resist any pressure to create other long-lived development branches by employing documented techniques." | the same page names the long-lived branch as the thing to resist | the "documented techniques" are the price: the two below |
| the same site, [short-lived feature branches](https://trunkbaseddevelopment.com/short-lived-feature-branches/) | "the branch should only last a couple of days" | "Any longer than two days, and there is a risk of the branch becoming a long-lived feature branch (the antithesis of trunk-based development)." | - |
| DORA, [trunk-based development](https://dora.dev/capabilities/trunk-based-development/) | "branches in trunk-based development typically last no more than a few hours"; "Have three or fewer active branches in the application's code repository."; "Merge branches to trunk at least once a day." | the page counts how often branches and forks are merged to trunk and asks for at least once a day; that wording is our paraphrase, not a sentence of the page | - |
| Martin Fowler, [FeatureToggle](https://martinfowler.com/bliki/FeatureToggle.html), 29 October 2010 | hide unfinished work on the mainline instead of branching | "How do you use Continuous Integration to keep everyone working on the mainline without revealing a half-implemented feature on your releases?" | see the next row |
| Pete Hodgson, [Feature Toggles (aka Feature Flags)](https://martinfowler.com/articles/feature-toggles.html), 09 October 2017 | release toggles: "feature flags used to enable trunk-based development for teams practicing Continuous Delivery" | the same sentence | "Release Toggles allow incomplete and un-tested codepaths to be shipped to production as latent code which may never be turned on." |
| Martin Fowler, [BranchByAbstraction](https://martinfowler.com/bliki/BranchByAbstraction.html), 7 January 2014; steps at [trunkbaseddevelopment.com](https://trunkbaseddevelopment.com/branch-by-abstraction/) | "a technique for making a large-scale change to a software system in gradual way that allows you to release the system regularly while the change is still in-progress" | the same sentence | the steps add an abstraction, a second implementation, a switch, and two removals; that extra work is our reading of the steps, not a sentence of theirs |
| trunkbaseddevelopment.com, [release from trunk](https://trunkbaseddevelopment.com/release-from-trunk/); DORA, [continuous delivery](https://dora.dev/capabilities/continuous-delivery/) | "Teams with a very high release cadence do not need (and cannot use) release branches at all. They have to release from the trunk." | DORA asks "Is our software in a deployable state throughout its lifecycle?" | - |

The two sources give different ceilings for a branch's life: a couple
of days on one, a few hours and a merge at least once a day on the other. We state
both and pick neither as a gate.

Does their constraint hold for us? Their constraint is many writers on
one codebase. Ours is the same with a twist: the writers are agents,
and the one reviewer who merges is a person. That is our reading of
our own setup; no source says it. It makes short branches more
important here, since a long branch is a long review, and the review
is where our time goes.

### Alternatives considered

- **Long-lived feature branches, merged when a feature is done.** This
  is what the sources define trunk-based development against. With
  agents writing, it also turns each merge into a review of days of
  generated code. Rejected.
- **Stacked pull requests, each based on the one below.** No source we
  verified says stacks are compatible with trunk-based development or
  that they are not. The spec has no stacking today, and v1 does not
  need it in order to start. Deferred, with its trigger, in the spec's
  [Deferred decisions](../spec.md#deferred-decisions); if it comes
  back, a new record supersedes this one.
- **Release branches.** A release here is a tag that `release.yml`
  builds (10 10.2). Nothing needs a branch that lives beside the
  trunk. Rejected.
- **A runtime feature-flag mechanism in the CLIs from day one.** A
  flag a user can turn on is a new surface: a setting, its validation,
  its help text, its tests. Unfinished work in a CLI can stay
  unreachable without one. Deferred, with its trigger, in the same
  table.
- **A limit on branch age or on open branches, enforced by a check.**
  The sources disagree on the number, and we have no measurement of
  our own yet. A limit we cannot defend would be copied, not learned.
  Rejected for v1; the metrics of
  [ADR 0004, measure delivery with the five DORA metrics computed by a tool](0004-measure-delivery-with-the-five-dora-metrics-computed-by-a-tool.md)
  measure the wait first.

## Decision

We develop on the trunk. The rules, with their checks, are in
[12 12.9](../spec/12-engineering.md#129-delivery-practices):

1. The trunk is the default branch, `main`. Every pull request targets
   it. `tools/ci pr` fails a pull request with another base.
2. A branch carries one pull request and lives until that pull request
   merges. v1 defines no stack of pull requests.
3. `main` is releasable at every merge: a merge needs the full check
   list to pass (10 10.2), and a release is a tag on a commit that
   passed it.
4. Work that is not finished merges dark, in small pieces, instead of
   waiting on a branch: code that no command reaches yet, with its
   tests. Replacing something that is in use goes through branch by
   abstraction, at an interface the code already has.
5. Batches are small. We measure pull request size and set no limit.
6. A merge is a merge commit. Squash and rebase merges are switched
   off, so the commits of a pull request reach `main` as they were
   written. The maintainer decided the method.

Why this fits us: the checks run from one tool at one commit (10
10.2), so "releasable" is a thing a machine says about `main`, not a
hope; and the reviewer is one person, so the smallest reviewable piece
is the cheapest one.

## Consequences

- `tools/ci pr` reads two more fields of the pull request payload and
  gains one clause (rule 1).
- Rules 2, 4 and 5 are review judgments. Nothing fails a branch that
  lived a week or a pull request of two thousand lines; the delivery
  metrics show both, and the review is where they are refused.
- Dark code is the cost Hodgson names: it ships and may never be
  turned on. Ours is counted by coverage (S9): a dark path is tested
  or it does not merge. No linter bounds it. A dark package's entry
  points are exported identifiers, which a dead-code checker treats as
  used. A check that bounds how long a package stays dark is deferred
  with its trigger.
- Merge commits keep each pull request's commits on `main`, which the
  fix marker and the release notes read
  ([ADR 0004, measure delivery with the five DORA metrics computed by a tool](0004-measure-delivery-with-the-five-dora-metrics-computed-by-a-tool.md)).
  The cost: the commits inside a merge were gated together, as that
  merge, and not one by one, so "releasable" is a statement about
  merges.
- With no stacking, a change that depends on an unmerged pull request
  waits for it. With one reviewer that wait is real, and ADR 0004
  measures it.
