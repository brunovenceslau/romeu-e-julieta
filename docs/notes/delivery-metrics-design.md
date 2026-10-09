# The design of the delivery metrics tool

Reader: whoever builds `tools/ci dora` when its deferred decision
reopens. Type: reference.

This note keeps the design of `tools/ci dora`, which review round 8
deferred out of v1: the tool is a row of the index's
[Deferred decisions](../spec.md#in-how-this-repository-is-run), and
[12 12.10](../spec/12-engineering.md#1210-delivery-metrics) says what
holds until it lands. It is not part of the specification. The text
below is the section as the specification held it before that round,
word for word; the points the round raised for when the tool lands
follow it.

`go run ./tools/ci dora` computes this repository's delivery metrics
from GitHub data. Nobody computes or types them. The decision, the
sources and the rejected alternatives are in
[ADR 0004, measure delivery with the five DORA metrics computed by a tool](../adr/0004-measure-delivery-with-the-five-dora-metrics-computed-by-a-tool.md).
The names and definitions are those of DORA, a research program run by
Google Cloud ([DORA's software delivery metrics](https://dora.dev/guides/dora-metrics/)).
The mapping of each one to a CLI released from tags is our decision:
no DORA text covers that case.

The tool measures this repository. It is not part of `romeu` or
`julieta` and gives a user's team nothing
([00 0.6](../spec/00-scope.md#06-teams-adopting-ai-assisted-development)).
It does not read the runtime ledger: each input below is GitHub data
or git history, and the ledger owns no metric
([13 13.9](../spec/13-runtime-ledger.md#139-consumers)).

**Inputs.**

| Input | From |
|---|---|
| the repository, `<owner>/<name>` | `--repo`, or `GITHUB_REPOSITORY` |
| releases: tag, `published_at`, `draft`, `prerelease` | the GitHub REST API |
| pull requests merged into `main`: `number`, `created_at`, `merged_at`, `merge_commit_sha`, `additions`, `deletions`, `changed_files`, `draft`; each one's commits, with their messages; its `ready_for_review` timeline events | the GitHub REST API |
| which release first contains a merge commit | git, in the checkout the tool runs in, with tags fetched |
| the time of the run | the injected clock (10 10.4) |

A token is read from `GITHUB_TOKEN` when it is set; the repository is
public, so the tool also runs without one. It makes read requests
only, except with `--attach` (below). Its unit tests run it against a
recorded API fixture and a fixture repository, offline (10 10.1).

**Terms.**

- A **deployment** is a published GitHub release of this repository
  whose tag matches `v*`, with `draft` and `prerelease` false.
- A **change** is a pull request merged into `main`. It belongs to the
  first deployment whose tag contains its merge commit. A deployment's
  commits, below, are the commits of its changes, as the pull request
  lists them; that is the one meaning of "a deployment has a commit".
- A **fix marker** is a commit message trailer, a whole line of the
  form `Fixes-release: v<major>.<minor>.<patch>`. It says that the
  commit repairs a problem in that release that needed immediate
  intervention. The author of the fix writes it, and whether the
  problem qualified is **[review]**. `tools/ci pr` fails a commit
  whose message has a line that starts with `Fixes-release:` and does
  not match `^Fixes-release: v[0-9]+\.[0-9]+\.[0-9]+$`. A marker that
  names a tag with no deployment is listed in the output under
  `unmatched` and counts in no metric.
- A deployment is **failed** when a later deployment has a commit
  with a fix marker that names it.
- A period is a calendar month in UTC. A deployment counts in the
  month it was published, a change in the month it merged.
- A median over an even number of values is the lower of the two
  middle values, so every figure is a whole number.

**The five metrics.**

| Metric (DORA's name) | DORA's definition | Computed here as |
|---|---|---|
| change lead time | "The amount of time it takes for a change to go from committed to version control to deployed in production." | per change: from the earliest committer date among the pull request's commits to the `published_at` of its deployment. Reported: the median, in seconds, over the changes of the period's deployments |
| deployment frequency | "The number of deployments over a given period or the time between deployments." | the number of deployments in the period |
| failed deployment recovery time | "The time it takes to recover from a deployment that fails and requires immediate intervention." | per failed deployment: from its `published_at` to the `published_at` of the first later deployment that has a commit whose fix marker names it. Reported: the median, in seconds |
| change fail rate | "The ratio of deployments that require immediate intervention following a deployment." | failed deployments over deployments, both as whole numbers |
| deployment rework rate | "The ratio of deployments that are unplanned but happen as a result of an incident in production." | rework deployments over deployments. A rework deployment has a commit with a fix marker and no commit of type `feat` (12.4) |

DORA groups the first three as throughput and the last two as
instability.

**Two more numbers**, which are not DORA's:

| Number | Computed as |
|---|---|
| pull request size | per change: `additions + deletions`, and `changed_files`. Reported: the median of each over the period's changes |
| maintainer wait | per change: from the moment it was ready to `merged_at`. Ready is `created_at`, or the time of the last `ready_for_review` event for a pull request opened as a draft. Reported: the median, in seconds; and `pushedAfterReady`, the number of changes with a commit whose committer date is later than that moment |

The wait means something because of a rule that already exists: a
pull request is opened after `tools/ci all` passed
([index](../spec.md#boundaries-for-everyone-who-changes-this-repo)).
A pull request that received a push after it was ready was not ready,
and the count says how often. The wait is measured for one decision
that is not taken yet: whether the maintainer's merge stays a manual
checkpoint, a [deferred decision](../spec.md#deferred-decisions) that
the first measured wait reopens.

**Output.** JSON in the format `dora.v1`, on stdout or in the file
`--out` names: `schema`, `repo`, `asOf`, `head` (the commit of `main`
the run read), `periods[]`, `deployments[]` (tag, `publishedAt`, the
numbers of its changes, `failed`, `rework`, `recoveredBy`) and
`unmatched[]`. A period object has `period` (`YYYY-MM`),
`deployments`, `changeLeadTimeSeconds`,
`failedDeploymentRecoverySeconds`, `failedDeployments`,
`reworkDeployments`, `changes`, `prSizeLines`, `prSizeFiles`,
`maintainerWaitSeconds` and `pushedAfterReady`. `--text` prints one
line per period instead. The same input gives the same bytes.

Four rules about what the output says:

1. **Throughput is never shown without instability.** A period is one
   object and one text line, written whole from one type, and no flag
   selects a metric. A unit test renders a fixture and fails when an
   output that has a lead time or a deployment count for a period
   lacks that period's failed and rework counts.
2. **Empty is not zero.** A period with no deployment has `null` for
   the two times, and the text line says `n/a`. Until the first
   release the five metrics are empty, and pull request size and the
   maintainer wait are all the tool reports.
3. **Every run is a backfill.** The tool has no incremental mode: it
   reads the history from the first commit each time. Its first output
   is therefore the baseline, and it holds every period before any
   change one might want to judge.
4. **No target.** The tool prints no performance level and compares
   with no other team. Targets are a
   [deferred decision](../spec.md#deferred-decisions).

Stated limits: a committer date is set by the machine that commits
and is rewritten by a rebase, so lead time is understated for a
rebased pull request, and `pushedAfterReady` is overstated for one;
`published_at` and `merged_at` are GitHub's. A failure nobody marks is
not counted. A commit that reaches `main` outside a pull request,
which no pull request carried, belongs to no
change and counts in no metric.

**Where the numbers live.** In two places, and in no tracked file:

- Each release carries them. `release.yml` runs
  `go run ./tools/ci dora --attach "$GITHUB_REF_NAME"` as its last step, after
  `tools/release publish`; the tool computes the output and uploads it
  to that release as the asset `dora.json`, replacing one that is
  there. So each release holds the whole history up to itself, written
  by the workflow; a prerelease's `dora.json` holds the history up to
  the last deployment before it. `dora.json` is a report, not a build output: it is
  not in `checksums.txt` and has no attestation (S1 lists the build
  outputs).
- Anyone can run the tool at any time and read stdout. Before the
  first release this is the only place.

**How they feed the lessons loop.** A lesson (12.7) that says a change
made delivery better or worse cites two periods of the tool's output,
one from before the change, and names where they came from: a
release's `dora.json`, or a run's `asOf` and `head`. It quotes the
instability figures with any throughput figure, as the tool does. A
lesson that cites no period attributes nothing. **[review]**; the
check on `tools/ci lessons` that would enforce it is a
[deferred decision](../spec.md#deferred-decisions). What the numbers
do not decide: they do not gate a merge, and nothing fails when one
gets worse. They are where a lesson starts.

## Points for when the tool lands

Review round 8 raised these against the design above; they apply when
the tool is built:

- The number "maintainer wait" is renamed "ready-to-merge wait", and
  its field `readyToMergeWaitSeconds`: it covers CI, review and merges
  by any account, not only the maintainer's.
- The ready-to-merge wait mixes the operator's own pull requests and
  outside ones; splitting them waits for the first outside pull
  request.
- A rate-limit or authentication error exits with a status of its own;
  a failed `--attach` leaves the release published and the run red, and
  running the step again attaches the file. The tool paginates and
  sends the API version header.
- A published release never changes (10 10.2), so `dora.json` is
  written before the release is published and is never replaced on an
  earlier release.
