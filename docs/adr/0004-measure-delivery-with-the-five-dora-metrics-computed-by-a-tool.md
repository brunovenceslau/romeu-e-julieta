# 4. Measure delivery with the five DORA metrics computed by a tool

Date: 2026-10-01

## Status

Accepted

## Context

The maintainer's workflow uses DORA metrics, and asked for them in the
spec. The condition is ours, put to the maintainer: a tool computes
them from GitHub data, and nobody computes them by hand. It follows
from our first principle, which is the maintainer's
([index](../spec.md#deterministic-where-it-can-be-the-model-where-it-adds-value)):
a number someone types is a number someone can round.

DORA is a research program run by Google Cloud. Google sells AI and
consulting; we label that and do not discount the work for it.

| Source | Their decision | Their why | What it cost them |
|---|---|---|---|
| DORA, [DORA's software delivery metrics](https://dora.dev/guides/dora-metrics/) | a five-metric model. Throughput: change lead time, deployment frequency, failed deployment recovery time. Instability: change fail rate, deployment rework rate | the same page: "The goal is to improve your team's performance over time, not to compete against other teams or organizations." | see the next row |
| DORA, [a history of DORA's metrics](https://dora.dev/insights/dora-metrics-history/) | in 2023 the recovery metric "historically known as 'mean time to recover (MTTR)' or 'time to restore service' was renamed and redefined as failed deployment recovery time"; in 2024 "they introduced a fifth metric: deployment rework rate" | - | the names and the definitions moved twice in two years. Anyone who wrote the old names into a tool has a stale tool; that reading is ours |
| DORA and Google Cloud, "The ROI of AI-assisted Software Development", v. 2026.1 (2026), linked from [dora.dev](https://dora.dev/ai/roi/report/), p.45 | "Stability gauge: Track change failure rate and rework to ensure that increased velocity isn't coming at the cost of stability" | its Figure 4 (p.20), from the DORA 2025 survey, which the report cites and we did not read: AI adoption is associated with software delivery throughput at +0.029 (89% credible interval -0.001 to +0.059) and with instability at +0.097 (+0.067 to +0.129) | the report's own calculator leaves the rework rate out (p.27) |
| the same report, p.4 | "it is difficult to measure the direct impact of AI without an understanding of 'before.' Do you have a baseline?" | the authors argue it; they present no data for it | - |

The definitions we use are DORA's, quoted from the guide:

- "Change lead time: The amount of time it takes for a change to go
  from committed to version control to deployed in production."
- "Deployment frequency: The number of deployments over a given
  period or the time between deployments."
- "Failed deployment recovery time: The time it takes to recover from
  a deployment that fails and requires immediate intervention."
- "Change fail rate: The ratio of deployments that require immediate
  intervention following a deployment."
- "Deployment rework rate: The ratio of deployments that are unplanned
  but happen as a result of an incident in production."

Does their constraint hold for us? Their model is about services
deployed to production. We ship two command-line programs from tags.
No DORA text covers that case, so the mapping from "deployment" to
what we do is our decision, written below as ours. Their warning
about throughput holds for us more than for most: agents raise
throughput by themselves, and what tells us whether that is good is
the instability beside it.

### Alternatives considered

- **Compute the numbers by hand, in a lesson or a review.** That
  breaks the first principle. Rejected.
- **Adopt an existing tool.** We looked at four, each measured through
  the GitHub API on 2026-09-30:
  [Apache DevLake](https://github.com/apache/devlake), a platform with
  its own storage;
  [DeveloperMetrics/deployment-frequency](https://github.com/DeveloperMetrics/deployment-frequency),
  a GitHub Action in PowerShell that computes one of the five;
  [middleware](https://github.com/middlewarehq/middleware), a
  TypeScript application; and
  [ephess/dora-metrics-calculator](https://github.com/ephess/dora-metrics-calculator),
  in Python. None knows our mapping of a tagged release to a
  deployment, and each adds a runtime or a service to a repository
  whose checks are one Go program that also runs offline in tests.
  Rejected; we learn the definitions and write a subcommand.
- **Count a merge to `main` as the deployment.** A merge reaches no
  user. A user gets a change when a release is published. Rejected.
- **Use DORA's performance levels as targets.** The guide itself says
  the goal is to improve over time, not to compete. We have no
  baseline yet, and a target without one is a number borrowed from
  someone else's team. Rejected for v1; deferred with its trigger.
- **A committed metrics file under `docs/`.** Someone would have to
  remember to refresh it, and no offline check can prove that a
  committed file equals what GitHub holds. Rejected; the release
  workflow attaches the file to each release instead.
- **The four-metric model, or the name "time to restore".** Both are
  retired by the source. Rejected.

## Decision

`go run ./tools/ci dora` computes the delivery metrics from GitHub
data and from the repository's git history. The tool, its inputs, its
output format and the definitions are in
[12 12.10](../spec/12-engineering.md#1210-delivery-metrics). The
decisions that are ours:

1. **A deployment is a published GitHub release of this repository
   whose tag matches `v*`**, drafts and prereleases excluded. That is
   what `release.yml` produces for a tag with no suffix; a tag with
   one, a release candidate, is published as a prerelease and is not a
   deployment.
2. **A change is a pull request merged into `main`.** Its lead time
   runs from its earliest commit to the publication of the first
   deployment that contains it. We report the median.
3. **A failed deployment is one that a later commit names** in a
   `Fixes-release: v<x.y.z>` trailer. The commits of a deployment are
   those of its pull requests, read from GitHub, so the result does
   not depend on how a merge was made. Whether a problem needed
   immediate intervention is a judgment, made by whoever writes the
   fix and whoever reviews it; the tool only counts the trailers.
4. **Recovery is the publication of the first later deployment that
   has such a commit**, because a user of a command-line program
   recovers by installing the release that fixes it.
5. **A rework deployment is one that has such a commit and no
   `feat` commit**: a release cut to repair, not one that was planned
   and also carries a repair.
6. **A throughput figure is never shown without the change fail rate
   and the rework rate beside it.** The tool has no way to print one
   without the others.
7. **The first run is the baseline.** The tool always reads the whole
   history, so its first output is a backfill, and no effect is
   attributed to a change without a period from before that change.
8. **Two numbers that are not DORA's ride along**: pull request size,
   and the maintainer's wait from a pull request being ready to its
   merge. Both come from GitHub's own fields.
9. **No target and no performance level in v1.**

Why this fits us: one more subcommand of the tool that already holds
every check, tested against a recorded API fixture like `acceptance`
(10 10.5), and a workflow step that needs nobody to remember it.

## Consequences

- `tools/ci` gains the `dora` subcommand, outside `all` because it
  needs the network, and `tools/ci pr` gains a shape check for the
  trailer. `release.yml` gains one step. The tool lives under
  `tools/ci/**`, an ask-first surface
  ([05 5.3](../spec/05-security.md#53-ask-first-surfaces)), so each
  change to it needs an approval line and a code-owner review, though
  it is a report and gates nothing. It lands with the release tooling
  and not on day one: every run reads the whole history, so nothing
  is lost by the wait.
- A commit that reaches `main` outside a pull request belongs to no
  change and counts in no metric.
- Before the first release there is no deployment, so the five
  metrics are empty and the baseline is pull request size and the
  maintainer's wait. We say "empty", never zero.
- The trailer is typed by a person or an agent. A failure nobody
  marks is a failure the change fail rate does not see. The review of
  a `fix` pull request is the guard, and it is a judgment.
- Commit times are set by the machine that commits and are rewritten
  by a rebase, so lead time is understated for a rebased pull request.
  Publication and merge times are GitHub's.
- DORA has redefined its metrics before. When it does again, the
  definitions here change by a new record that supersedes this one,
  and the output format gets a new version.
- The tool measures this repository. It is not a feature of `romeu`
  or `julieta`, and nothing here gives a user's team its own numbers;
  [00 0.6](../spec/00-scope.md#06-teams-adopting-ai-assisted-development)
  records that gap.
- Deferred, each with its trigger, in the spec's
  [Deferred decisions](../spec.md#deferred-decisions): targets, a
  check on lessons that cite a metric, a record of gate runs, a
  finding-class field, and the inputs a team would need for a return
  on investment model. We compute no return on investment and run no
  scenario analysis.
