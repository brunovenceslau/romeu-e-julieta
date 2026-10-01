# Spec round 4: engineering practices and maintainer decisions

Reader: reviewers checking what entered the spec after round 3 and
why, and maintainers tracing a decision. Type: reference (history).
The current state is [docs/spec.md](../spec.md); round 3 is in
[round-3.md](round-3.md).

Round 4 is not a review. It records one write pass that folded the
maintainer's engineering practices into the spec, and the maintainer
decisions that pass rests on. The round-3 sweep reviews its result. It
is a new file because a review round is history and is not rewritten
(ADR 0001, Consequences); one stale cell of round 3 was corrected in
the same pass, as listed below.

The inputs were research notes on trunk-based development, Extreme
Programming and the DORA metrics, and on DORA and Google Cloud's
report "The ROI of AI-assisted Software Development" (v. 2026.1). Each
claim in them had been checked against its source by reviewers briefed
to refute it. A claim that was neither confirmed nor corrected there
did not enter the spec, and a number entered only with its page or
URL. The published sources are cited in ADRs 0002 to 0005 and in
[00 0.6](../spec/00-scope.md#06-teams-adopting-ai-assisted-development).

## Maintainer decisions

Decided on 2026-10-01, except the first two, which the maintainer
stated on 2026-09-30. The others were put to the maintainer as
recommendations, in one block, and the maintainer accepted the block.

| Id | Decision | Where |
|---|---|---|
| op-practices | the project works with XP and so with trunk-based development, and applies DORA metrics; each adopted practice is recorded in an ADR that cites its source and says why it fits here | 12 12.9, 12 12.10, ADRs 0002 to 0004 |
| op-last-moment | what the start needs is built as soon as it is needed; a decision that can wait for more context and more use waits, and is written down | index, Principles; ADR 0005 |
| op-pairing-rule | a throughput metric is never shown without the change fail rate and the rework rate beside it | 12 12.10 |
| op-backfill | a backfill of the repository's GitHub history is the baseline | 12 12.10 |
| op-maintainer-wait | the wait from a pull request being ready to its merge is measured from GitHub timestamps, by the metrics tool; merging stays the maintainer's | 12 12.10 |
| op-pr-size | pull request size is reported by the metrics tool | 12 12.10 |
| op-gate-run | a record of each gate run is not specified now: it is an open item for the runtime ledger, and its join key is unresolved | index, Deferred decisions |
| op-finding-class | a finding-class field is deferred with a trigger | index, Deferred decisions |
| op-no-roi | scenario analysis and a return-on-investment percentage are dropped | 00 0.6; ADR 0004 |
| op-residual-risks | three residual risks are accepted for v1: with one account the code-owner review is not independent and the maintainer's merge is the only control; no check reads the rulesets again after they are created; a `v*` tag's release depends on the tag ruleset | 05 5.4 |
| op-second-account | a second account without bypass rights, for agents, is the accepted fallback of Q25, to be confirmed when the token test runs | Q25, 05 5.4 |

## What the pass added

| Added | Where |
|---|---|
| the principle "Decide at the last responsible moment" | index, Principles |
| trunk-based development and six XP practices as rules, each with its check or a review tag | 12 12.9 |
| `tools/ci dora`: inputs, the `dora.v1` output, the five DORA metrics mapped to a CLI released from tags, pull request size, the maintainer wait | 12 12.10 |
| two clauses of `tools/ci pr` (the base branch; the form of a `Fixes-release:` trailer) and one of `tools/ci sequences` (each deferral row has three cells) | 10 10.2, 12 12.4 |
| who else the product is for, the problems the report attributes to them, and what the spec does and does not answer | 00 0.6 |
| ADRs 0002 to 0005, Proposed | `docs/adr/`; Q24 covers them |

Why four records: a record holds one decision (12 12.5), and these are
four that can be reversed separately. Stacking could replace the
branching rule without touching the XP practices; DORA could redefine
a metric without touching either; and the principle of deferring
stands with or without the other three.

The writer's own decisions, which the maintainer has not ruled on and
the round-3 sweep should look at: the mapping of the five metrics to
releases, the `Fixes-release:` trailer as the failure marker, the
definition of a rework deployment, `dora.json` as a release asset and
no tracked metrics file, and `tools/ci dora` as a subcommand of
`tools/ci` and so on the `checks` surface.

## Removed or deferred

Deferred, each with its trigger, in the index's
[Deferred decisions](../spec.md#deferred-decisions) table: stacked
pull requests, a runtime feature-flag mechanism, targets for the
delivery metrics, a check on lessons that cite a metric, a record of
gate runs, a finding-class field, and the inputs of a
return-on-investment model for a team.

Dropped: scenario analysis; a return-on-investment percentage;
performance levels as targets.

Corrected in the same pass, from the earlier gate's leftover items:
12 12.4 named two of the four commit readings of the forbidden-name
check; 05 5.3 cited ADR 0001 by number at its first mention; the Never
boundary said "a user name" for a local user name; the op-declined row
of round 3 named a location that no longer held the text; 10 10.2 said
that `fast` without arguments checks the tracked files, where it runs
the `In fast` steps.

Question changed: Q24 now covers the five Proposed records. Q25 states
the accepted fallback.
