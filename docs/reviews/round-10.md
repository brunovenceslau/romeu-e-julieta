# Spec review round 10: the targeted re-audit of round 9

Reader: reviewers checking that every finding of round 10 was handled,
and maintainers tracing why the spec reads as it does. Type: reference
(history). The current state is [docs/spec.md](../spec.md); round 9 is
in [round-9.md](round-9.md); the lenses are defined in
[lenses.md](lenses.md).

Round 10 is step 6 of "How stage A runs, and when the spec counts as
validated" in lenses.md: the targeted re-audit of the round-9 write
pass. Each lens read the sections that pass changed, checked the
round-9 findings it had raised against the new text, and reported what
was left, what regressed and what the new text added. lenses.md is
cited at the baseline commit.

The baseline is commit d458f83, the head of pull request #36 (the
round-9 write pass, branch `docs/review-round-9`, with the decisions of
2026-10-09 applied), read from a detached clone that nothing wrote to
while the reviewers ran. It is not a commit of the default branch, as
step 1 of lenses.md asks for a full round: the write pass is validated
before it merges, so the re-audit reads the pull request's head.

Every item below is a finding of a reviewer or a proposal of the
consolidation. Where a row needs the operator, the item is in
"Decisions for the operator" below with a recommendation, and the
decision is recorded next to it, with its date, when it comes.

Status, as in rounds 6 to 9: **A** = applied as proposed; **M** =
applied in a smaller or different form than proposed, as its row
states; **L** = stated as a limit, no mechanism added; **operator** =
not applied, and the item is in "Decisions for the operator". A row
that also names an item there was applied as that item recommends, and
the item records the answer when it comes. The write pass is described
in "Write pass (2026-10-09)" below.

The reports are kept in `round-10/reports/<lens>.json`, as the
reviewers wrote them.

## How the round was run

Seventeen reviewers, one per lens, L0 to L16, each in a fresh context,
read-only, in parallel on d458f83. L2 and L13 ran on `opus`, the rest
on `sonnet`. Each lens ran once, with one reader: a re-audit carries no
noise probe, since the probe measures a lens set on a full read, and
round 8 recorded it. Every lens ran because every lens had findings
that drove the round-9 delta. Each wrote one JSON report with
`prior_checked`, `prior_resolved_as_stated` and its findings, each of
kind `unresolved` (a round-9 finding the pass did not resolve as its
row states), `regression` (the pass broke what held) or `new` (a defect
in text the pass wrote or a seam it opened). No report is missing.

The reports hold 38 findings: 0 Blocking, 14 Required, 24 Advisory; 36
new, 1 unresolved, 1 regression. One consolidator merged them by
section and defect, kept every lens on its row, and checked each
Required finding against the baseline text. Of the 14 Required
findings, 6 hold as Required (6 clusters, four of them joined by
Advisory findings of other lenses), 3 were downgraded to Advisory, and
5 are findings on `plan.md` alone, parked for stage B as lenses.md
says; none was dropped. The result is 6 Required clusters, 17 Advisory
rows (3 of them downgraded) and 4 clusters parked for stage B.

### Per lens

`prior_checked` is the number of round-9 findings the lens re-read
against the new text, and `prior_resolved_as_stated` how many of them
read as their round-9 row states. Severities in this table are the
reviewers'; the consolidation's are in the findings below.

| Lens | Model | prior_checked | prior_resolved_as_stated | Findings (B / R / A) | new | unresolved | regression |
|---|---|---|---|---|---|---|---|
| L0 | sonnet | 4 | 4 | 1 (0 / 0 / 1) | 1 | 0 | 0 |
| L1 | sonnet | 3 | 3 | 2 (0 / 0 / 2) | 2 | 0 | 0 |
| L2 | opus | 6 | 6 | 4 (0 / 3 / 1) | 4 | 0 | 0 |
| L3 | sonnet | 6 | 6 | 3 (0 / 1 / 2) | 3 | 0 | 0 |
| L4 | sonnet | 4 | 4 | 2 (0 / 1 / 1) | 2 | 0 | 0 |
| L5 | sonnet | 7 | 7 | 2 (0 / 0 / 2) | 2 | 0 | 0 |
| L6 | sonnet | 7 | 7 | 4 (0 / 2 / 2) | 3 | 0 | 1 |
| L7 | sonnet | 5 | 5 | 0 (0 / 0 / 0) | 0 | 0 | 0 |
| L8 | sonnet | 8 | 8 | 1 (0 / 0 / 1) | 1 | 0 | 0 |
| L9 | sonnet | 4 | 4 | 3 (0 / 2 / 1) | 3 | 0 | 0 |
| L10 | sonnet | 4 | 4 | 2 (0 / 0 / 2) | 2 | 0 | 0 |
| L11 | sonnet | 2 | 2 | 1 (0 / 0 / 1) | 1 | 0 | 0 |
| L12 | sonnet | 4 | 4 | 3 (0 / 2 / 1) | 3 | 0 | 0 |
| L13 | opus | 12 | 12 | 7 (0 / 1 / 6) | 7 | 0 | 0 |
| L14 | sonnet | 2 | 1 | 1 (0 / 1 / 0) | 0 | 1 | 0 |
| L15 | sonnet | 7 | 7 | 2 (0 / 1 / 1) | 2 | 0 | 0 |
| L16 | sonnet | 4 | 4 | 0 (0 / 0 / 0) | 0 | 0 | 0 |
| all | | 89 | 88 | 38 (0 / 14 / 24) | 36 | 1 | 1 |

The one prior finding not resolved as stated is L14-r9-1 (R9-26): its
04 4.1 half holds, and 05 still states the old claim (R10-05). L6
counts its 7 priors as resolved and reports one regression, L6-r10-1:
the spec half of R9-13 holds, and the plan's require-pass lists were
not extended (P4).

## Findings and resolutions

One row per cluster. The Required rows come first, then the three
downgraded rows, then the Advisory rows in lens order. Kind is the
cluster's: `unresolved` when any member is, else `regression` when any
member is, else `new`. Sections are cited as the spec cites itself.

### Required

| Cluster | Section | Lenses | Kind | Defect | Verification | Resolution | Status |
|---|---|---|---|---|---|---|---|
| **R10-01** | 05 5.3 (the `approvals` surface and its entry); 12 12.4 (the `pr` job; A checkpoint grant); 02 2.1 (`tools/ci`) | L2 (L2-r10-1) | new | 05 5.3 says the `approvals` surface holds the check of approval lines and grants, "which no checkpoint grant covers", but its globs name `tools/ci/askfirst.go` alone, while the `pr` job builds the whole `tools/ci` main package, its step packages and the modules of `go.mod`, all on `checks` or `dependencies`, which 12 12.4 lets a checkpoint grant cover. | holds: the `approvals` entry of 05 5.3 and `.github/ask-first.yaml` globs `tools/ci/askfirst.go`, `tools/ci/denylist.yaml` and `.github/workflows/**`; `tools/ci` is one main package of many files; the grant list of 12 12.4 names `checks` and `dependencies` | 05 5.3 and 12 12.4 state the rule: `pr` is built from a main package of its own whose import closure is the standard library and paths on `approvals`; a `tools/ci` test fails when `go list -deps` of that package holds a path outside the `approvals` globs; the globs gain that package in the tooling pull request of the task that builds `pr` (an `ask-first` change); and 05 5.3 names what stays grantable under the job (the toolchain pinned by `mise.toml` and `mise.lock`), see D2 | A (spec text; the glob with the task that builds `pr`): the package is named `tools/ci/pr`, and the `go` line of `go.mod` is named beside the toolchain as a grantable input; D2 |
| **R10-02** | 05 5.4 (the row "Under a `checks` grant ..."); 12 12.4 (the `pr` job) | L2 (L2-r10-2), L11 (L11-r10-1) (high-signal) | new | The mitigation "re-run the `pr` job of each open pull request when `pr` changes" cannot pick up a fix: by GitHub's documentation on re-running workflows, a re-run keeps the `GITHUB_SHA` of the original event, which under `pull_request_target` is the default branch's commit the job checks out; and the step is left to memory, not to a mechanism. | holds as inferred from GitHub's documentation, not measured: 05 5.4 says "re-run", and 12 12.4 has the job check out `github.sha` | 05 5.4 says a new event runs the fixed `pr` (an `edited` event from a change to the body, which the job listens to, or a close and reopen), and that a re-run of an old run judges with the old code; 12 12.4 carries the same sentence; an automatic new run per open pull request when the `pr` build inputs change is a row of Deferred decisions, reopened by the first pull request merged on a stale `pr` result | A; D1. GitHub's documentation, read on 2026-10-09, confirms that a re-run keeps the original `GITHUB_SHA`; the row's risk and why it is accepted are unchanged |
| **R10-03** | 10 10.2 (the `pr` paragraph: "`pr` itself makes no network call"); 12 12.4 (the `pr` job); 10 10.1 (Rules, the one list of network reads) | L13 (L13-R10-1) | new | 10 10.2 says `pr` makes no network call, while 12 12.4 has every git call of `pr` on head objects pass its flags, `fetch` included, and the grammar leaves no other step to fetch `head.sha`; 10 10.1, the one list of the network reads of the steps, does not list that fetch. | holds: the three texts read as stated; the pr job row of 10 10.2 admits one checkout, of `github.sha`, and `run` steps that are `go run ./tools/ci` only | option (a) of the finding: `pr` fetches `head.sha`, unauthenticated, only in the pr job and only when the object is absent; 10 10.2 says `pr` reads the network only for that fetch, and a local run on a saved payload or with `--title` and `--body` reads none; 10 10.1 lists the pr job's head fetch | A |
| **R10-04** | 01 1.6 (the `removing` row; Recovery); 03 3.8 (`salvage[]`); 03 3.11; 08 8.5; J6 step 5 | L4 (L4-r10-1) | new | The resume rule compares the daemon heads and each worktree's HEAD and status with "the salvage record", but the record of 03 3.8 holds no heads, HEAD or status; the manifest of 03 3.11 holds HEAD and a `dirty` flag only, and 08 8.5 lets the owner delete the salvage dir once the result is recorded. | holds: 03 3.8 lists `id`, `manifestSha256`, `result`, `reasons` and `refsPrefix`; 03 3.11 has `dirty` as a flag; 08 8.5 reads as stated | the state `salvage[]` entry of 03 3.8 gains a fingerprint, written with the state `removing`: the daemon heads, each worktree's HEAD and a digest of its full status (untracked and ignored paths with file digests), under the version rule of 03 (opening); the rerun resumes at `sbx env rm` only when a fresh fingerprint equals it, else `removing -> salvaging`; 08 8.5 says the owner may delete a salvage dir only once its generation has left `removing`; I17 gains the case of an already-dirty worktree edited again after a failed `sbx env rm` | A |
| **R10-05** | 05 5.4 ("Safe to paste"); 04 4.1 (Error ids) | L14 (L14-r10-1) | unresolved (R9-26) | 04 4.1 now limits the paste claim to the argv, status, step and versions and says a token in a remote URL or an sbx body is not removed, while 05 "Safe to paste" still says the details of an error hold no secret value and can be pasted into an issue. | holds: 05 "Safe to paste" reads as stated; 04 4.1 carries the narrower claim | 05 "Safe to paste" keeps one sentence and links to 04 4.1, which owns the claim: the argv, status, step and versions can be pasted, and the stderr is looked over first; `romeu version --json` stays in the sentence | A |
| **R10-06** | 11 11.2 (B2); 06 6.4 (the signing key paragraph); J1 step 5; 05 5.4 (Verified signature row); 08 8.1 (skill rule) | L6 (L6-r10-2), L0 (L0-r10-1), L1 (L1-r10-1), L5 (L5-r10-2), L10 (L10-r10-2) (high-signal) | new | B2 is `kind: check` and its predicate includes that a commit "asks for confirmation through the installed askpass", which no harness observes; and the texts disagree on whether an askpass is required (J1 step 5: install one first; 06 6.4 and 05 5.4: confirm-on-use where one is installed), with no page saying what a signing use does without one, or what the agent sees and does when a confirmation is pending, refused or unavailable. | holds: B2 and the predicate rule of 11 11.2 read as stated; J1 step 5, 06 6.4 and the 05 5.4 row read as quoted | B2 names its observation: the harness points `SSH_ASKPASS` at a wrapper that records each call and answers, and the result records the call and the signed commit; on each host it also records the outcome of a signing use with no askpass installed; 06 6.4 and J1 step 5 agree that `ssh-add -c` needs a program `SSH_ASKPASS` can name, installed first, and point at B2 for the outcome without one; the julieta skill rule of 08 8.1 says a signing failure is reported to the operator and never worked around by unsetting `commit.gpgsign`. Within decision D3 of round 9 (2026-10-09); the 05 5.4 row changes only its pointer | A |

### Downgraded to Advisory

| Cluster | Section | Lenses | Kind | Defect | Why downgraded | Resolution | Status |
|---|---|---|---|---|---|---|---|
| **R10-07** | 10 10.2 (maintainer block, item e; pr job row); ADR 0010 decision 1 | L2 (L2-r10-3, Required) | new | Item e says the `pr` job joins the required checks "the same way", but which commit a `pull_request_target` check attaches to is unmeasured, and a head's own `pull_request` workflow can post a check of the same name. | the name collision needs an agent that adds such a job on purpose, which the approval-line row of 05 5.4 already accepts ("a record and a guard against mistakes, not a barrier against an agent"); the attachment question is evidence for an item that is open (ADR 0010 decision 1) and for a maintainer step not yet due | item e says the step records the check run's `head_sha` and whether a ruleset that requires it blocks and then allows a merge, under the Evidence of the task that builds `pr`; `pr`, from the default branch, refuses a head whose workflows give a job outside the pr job's file the pr job's name, with a fixture; if the check cannot be required, that is recorded under the ADR 0010 item | M: the measurement and the name refusal are stated; the attachment question joins the next decision block's ADR 0010 item |
| **R10-08** | J1 steps 1 and 2; 10 10.2 (Verification contract); 12 12.1 | L15 (L15-R10-1, Required) | new | The operator's `gh attestation verify` now passes `--source-digest` and `--deny-self-hosted-runners`, while J1 and 12 12.1 ask only for a `gh` with the `attestation` command, and no floor or probe records the flags. | `gh` refuses an unknown flag and exits non-zero, so an older `gh` fails visibly and never verifies less; what is missing is a prerequisite line, not a weaker check | 12 12.1 and J1 step 1 state a `gh` floor, the first version with both flags; B1 records the `gh` version; 10 10.2 says the pinned `gh` fixes the development sandbox's flags and the floor fixes the operator's | A: the floor is `gh` 2.68.0, the first tag of `cli/cli` whose `gh attestation verify` has both flags (read on 2026-10-09 at v2.67.0 and v2.68.0) |
| **R10-09** | 11 11.2 (opening) | L6 (L6-r10-1, Required, its spec half) | regression (L6-r9-3) | 11 11.2 sizes the sitting as "about 90 minutes per host for B1 to B5", with B6 added in round 9. | the spec half is a time estimate; the gating omission (B6 outside the require-pass lists of O6 and T092) is plan text, parked for B as P4. Round 9's write pass noted the estimate for this round | 11 11.2 gives the estimate for B1 to B6 | A |

### Advisory

Advisory findings are applied by default (lenses.md, step 4). The
Defect cell keeps the reviewer's first sentence in short; the report
holds the rest.

| Finding | Section | Kind | Defect | Resolution | Status |
|---|---|---|---|---|---|
| L1-r10-2 | J1 step 2 | new | First-install verification takes three pasted commands with three placeholders, the attestation being the one most likely skipped. | the `commands.yaml` entry gives one pasteable block with the tag as its one variable; J1 says skipping it is the user's choice and what it costs | A |
| L2-r10-4 | 12 12.4 (the `pr` job); 10 10.2 (pr job row) | new | `pr` prints author-controlled text (title, body lines, commit subjects) in a job where the runner reads lines starting with `::` as workflow commands. | 12 12.4 says `pr` neutralizes a leading `::` and control characters in author text before printing it; fixture "pr escapes a workflow command in a PR body" | A |
| L3-R10-3 | 05 5.3 (`approvals`); 12 12.4 | new | The id `approvals` reads as the generic word, and one path sits on up to three surfaces with no vocabulary entry. | the id stays, since renaming a surface is a change on the `ask-first` surface for a reading cost; 05 5.3 says in one sentence that each path on `approvals` is also on `checks` or `dependencies` and needs a line for each | M: the overlap sentence only; no rename |
| L4-r10-2 | J7 step 4 | new | The attic may be deleted when a report nobody kept shows nothing to keep, or when `git status` and `git for-each-ref` look clean, which shows no unpushed commit. | J7 step 4 names `git -C <clone> log --branches --not --remotes --oneline` and the list of `refs/romeu/salvage/` refs as the check | A |
| L5-r10-1 | 08 8.3 (Resume); 08 8.2 | new | The gap line says "the session ended without /handoff" also when /handoff ran and a commit followed before /clear. | the line names what is measured ("the newest facts differ from the latest narrative: <n> new commits since <time>"); golden for handoff, commit, /clear. This refines R9-10 | A: the line lists what changed (new commits, a moved branch, a changed dirty count) |
| L6-r10-3 | 10 10.2 (Release step 1); 10 10.1 (fuzz); S6 | new | The release refusal reads the latest scheduled `fuzz.yml` run, with no stated schedule and no word on whether a manual run counts. | 10 10.1 names the schedule; one sentence beside step 1 says whether a `workflow_dispatch` run counts | A: daily on `main`; a `workflow_dispatch` run does not count |
| L6-r10-4 | 10 10.2 (the `pr` step); 12 12.4 | new | Nothing says the pr job fails when the walk from base to head finds no commit, or how its first real run is observed. | the `pr` step fails when the walk finds zero commits (spec half); the Evidence lines of the task that builds `pr` are parked for B (P1) | A (spec half) |
| L8-R10-1, L13-R10-2 | ADR 0010 (Context; Decision; Consequences) | new | ADR 0010 says it corrects four texts, and its Decision lists five (items 2 to 6); the Context leaves out round 9's D2 among the items that land in a record. (high-signal) | before pull request #36 merges: "five texts" in the Context and the Consequences, "three of them land in decision records", and round 9's D2 (R9-24) named beside the others; approval line `decisions`, which the pull request already carries | A; D3: ADR 0010 is still only in pull request #36, so it was corrected in place |
| L10-r10-1 | 03 (opening, Versions); 04 4.4 | new | A config repo whose `project.v1` or `host-settings.v1` is older than the window of two versions has no error or hint. | one row in 04 4.4, `format-unsupported`, exit 2, with the hint "apply the edit in the release notes of each skipped release"; 03 cites it | A: `RJ-337` |
| L13-R10-3 | index, Deferred decisions (token narrowing; required checks); ADR 0010 decision 1 | new | Two Deferred rows whose triggers fired are handled two ways: one names the record that reopens it and the decision-block item, the other says the review is due and names neither. | both rows say "fired" and name what reopens them and where the decision waits; for the token row, the next decision block's item and the record half in the next decision record | A; D4 |
| L13-R10-4 | 04 4.2 (doctor table); 04 4.4 (RJ-334, RJ-336); 01 1.7 | new | `doctor` is said to report `removal-pending` and `record-unreadable`, and the doctor table has neither row; its `generation-sandbox` row also matches a `removing` generation. | two doctor rows, `removal-pending` (fail; hint `romeu rm <name>`) and `record-unreadable` (fail; hint as RJ-334); `generation-sandbox` excludes a `removing` generation | A |
| L13-R10-5 | 04 4.2 (doctor preamble); 01 1.5; 05 5.4 | new | The new sentence "a **pre** check stops a mutating command with exit 2" contradicts 01 1.5, where the protective commands print a failed step 5 and go on. | the sentence excepts the protective commands of 01 1.5, which print it and go on | A |
| L13-R10-6 | 05 5.4 (approval-line row; token row); 10 10.2 (maintainer block, items a and d) | new | 05 5.4 states the direct-push refusal as measured on 2026-10-08, before the 2026-10-09 ruleset change, and no text names the rule that refuses it now. | 05 5.4 and item d say the try predates the change, and name the pull-request rule as the one expected to refuse it, read and not tried; a direct-push try joins the next sitting | operator: D5; the rows stay as they are until the re-measure of D6 |
| L15-R10-2 | 12 12.4 (the `pr` job, git flags) | new | The git flag rules rest on one git version and the runner's unpinned git, with no test that fails when a flag changes meaning. | 12 12.4 says the job uses the runner image's git, dates the 2.53.0 reading, and a `pr` test runs each listed git call against a hostile repository (submodule, textconv, external diff, hooks) | A |

## Parked for stage B

Findings on `plan.md` alone, kept for the plan review. Five of them
were Required for their lenses; they are not counted as open Required
findings of the spec, as lenses.md says for a finding on the plan.
P1 is the first item stage B reads, because it is the one place where
the G5 fix has no owner yet.

- **P1, the pr job's task and the ask-first forecasts.** L3 (L3-R10-1,
  Required), L9 (L9-R10-2, Required), L12 (L12-R10-2, Required), L13
  (L13-R10-7), L12 (L12-R10-3), and the plan half of L6-r10-4
  (high-signal). No task writes the base-defined `pull_request_target`
  workflow of 10 10.2 and 12 12.4, its grammar fixtures or its git
  rules; T002 still tests the `edited` type on `ci.yml`; T005's
  Ask-first names `checks` only, and no task forecasts `approvals` for
  the workflows and the denylist; the re-trigger of R10-02 has no task.
- **P2, the `sbxdrv` surface tooling pull request.** L12 (L12-R10-1,
  Required), L3 (L3-R10-2), L9 (L9-R10-3). T015 and T078 depend on a
  pull request with no task id, place in the phase table or critical
  path; the plan's list of tooling pull requests stops at round 8.
- **P3, T090 and the pre-tag run.** L9 (L9-R10-1, Required). T090 still
  says `--pre-tag` runs four checks, where 10 10.5 now lists six, and
  no task builds or tests `acceptance --tags` (12 12.2 step 7).
- **P4, B6 in the require-pass lists.** L6 (L6-r10-1, Required, its
  plan half). O6 and T092 require B1 to B5 and C2 to C6, without B6.

## Decisions for the operator

Everything the consolidation cannot decide, in one block: a change to
an accepted risk, a decision record, or a path outside `docs/`. Each
item names the finding, the options and the consolidation's
recommendation; the recommendation is not a decision. The write pass
applied D1 to D4 as recommended, and each records the answer, with its
date, when it comes; D5 and D6 wait for it.

- **D1 (R10-02), the mitigation of the `checks` grant row of 05 5.4.**
  Options: (a) the row says a new event (a body edit, or a close and
  reopen) runs the fixed `pr`, and a re-run does not, as a manual step,
  with an automatic re-trigger as a Deferred row reopened by the first
  pull request merged on a stale `pr` result; (b) build the automatic
  re-trigger now, a push-to-main job with `pull-requests: write`, kept
  out of the pr job; (c) leave the row, which keeps a mitigation that
  does not act. Recommendation: (a): it states what works today at no
  new permission, and the trigger names the event that would justify
  (b). Approval line: `spec`.
- **D2 (R10-01), what judges a change and who can change it.** Options:
  (a) `pr` gets its own main package whose closure is the standard
  library and paths on `approvals`, held by a `go list -deps` test, with
  the toolchain pinned by `mise.toml` and `mise.lock` named in 05 5.3
  as the one grantable input under the job; (b) the same, and
  `mise.toml` and `mise.lock` also leave the grantable list, so a
  toolchain bump always carries a line in the maintainer's words;
  (c) put all of `tools/ci` on `approvals`, which ends grants for the
  gates' code. Recommendation: (a) now, with (b) as a Deferred row
  reopened by the first toolchain bump under a grant; the glob change in
  `.github/ask-first.yaml` lands with the task that builds `pr`.
  Approval lines: `spec` now; `ask-first` and `approvals` with that
  task.
  Answered on 2026-10-09: option (a), then narrowed by round 11
  (R11-01 to R11-03): the spec keeps the invariant of the `pr` job, and
  the package, its closure test and the inputs left under the job are
  acceptance requirements of plan T005
  ([round 11](round-11.md#write-pass-2026-10-09)).
- **D3 (L8-R10-1, L13-R10-2), the count in ADR 0010.** The record is in
  pull request #36 and not yet merged. Options: (a) correct the count
  and name round 9's D2 before the merge; (b) merge as is and correct it in the
  next record. Recommendation: (a), since 12 12.5 does not rewrite an
  accepted record once merged. Approval line: `decisions`, already on
  the pull request. The write pass edited ADR 0010 in place, since it
  is in pull request #36 alone and not yet a record of the default
  branch.
- **D4 (L13-R10-3), the two fired Deferred rows.** Options: (a) both
  stay in the table, each saying "fired", the record that reopens it,
  and where the decision waits; the token-narrowing review becomes an
  item of the next decision block, and its record half goes to the
  next decision record; (b) both move to Open questions until decided.
  Recommendation: (a): the table's preamble keeps a row until its
  decision is made. Approval lines: `spec`, and `decisions` for the
  record half.
  Answered on 2026-10-09: option (a), and both decisions were taken
  that day in [ADR 0011, bind the administrator to the required checks and narrow the sandbox's GitHub token](../adr/0011-bind-the-administrator-to-the-required-checks-and-narrow-the-sandbox-s-github-token.md): the required-checks row leaves the table
  (decision 1), and the token-narrowing row stays, reviewed, until the
  narrowing is applied (decision 2).
- **D5 (L13-R10-6), the direct-push refusal in 05 5.4.** Options: (a)
  the row says the 2026-10-08 try predates the 2026-10-09 change and
  names the pull-request rule as the expected refusal, read and not
  tried, and a direct-push try joins the next maintainer sitting; (b)
  try it now and record the result. Recommendation: (a), with the try
  at the sitting the ADR 0010 item already needs. Approval line:
  `spec`. Not applied in the write pass: the 05 5.4 row and item d of
  the maintainer block stay as they are until the measurement of D6.
  Answered on 2026-10-09: option (a), with the try repeated with the
  four tries after the token narrowing (ADR 0011, decision 3).
- **D6 (D5's measurement), the direct-push refusal under the current
  ruleset.** The refusal in 05 5.4 and in item d was tried on
  2026-10-08, before the 2026-10-09 ruleset change, and has not been
  tried since. Options: (a) re-measure it: a direct push to the
  default branch with the development sandbox's token, its answer saved
  under Evidence, and the row and item d worded from that answer;
  (b) state it as read from the rules and not tried, as D5 (a) says.
  Recommendation: re-measure, with the operator present. Approval
  line: `spec`, for the rewording that follows.
  Answered on 2026-10-09: option (b), read from the rules and not
  tried, as D5 (a) says; the try is repeated with the four tries after
  the token narrowing (ADR 0011, decision 3).

## Exit criteria of stage A

The five criteria of lenses.md, "How stage A runs, and when the spec
counts as validated", judged on d458f83:

1. **No Blocking open; every Required fixed or declined with its
   decision and date: not met.** 0 Blocking. 6 Required clusters are
   open (5 new, 1 unresolved from round 9), none fixed and none
   declined.
2. **Coverage: carried from round 8, as in round 9.** A re-audit reads
   only the changed sections, and its reports carry no complete
   coverage table (one report, L2, has one), so the round-8 measurement
   stands: every page with a finding or an explicit "nothing for this
   lens" from at least two lenses; the criteria and journeys unproven
   beyond that ([round-8.md](round-8.md#coverage)).
3. **The open questions and deferred decisions of the index have a
   valid shape: met as measured.** 100 rows: 24 open questions and 76
   deferred decisions (36 in the product, 11 with the runtime ledger,
   29 in how this repository is run). Every row has all its cells
   filled, measured by a script over the tables. The token-narrowing
   row now names its record, its mark and its count (R9-24). Two rows
   have a fired trigger and stay open until their decision, as the
   preamble says; that they say so in two forms is L13-R10-3
   (Advisory, D4). Duplicates were not measured beyond the reviewers'
   reading.
4. **The three measurements: carried from round 8.** The `listed` to
   `own` ratio per lens, the clusters only L0 raised and the overlap of
   the L2 and L13 noise-probe pairs are in
   [round-8.md](round-8.md#measurements). This round ran no noise probe
   and records no origin per finding; L0 raised one finding, which
   L1, L5, L6 and L10 raised too (R10-06), so no finding is L0's alone.
5. **The targeted re-audit raises no new Required finding: not met.**
   It raised 5 new Required clusters (R10-01 to R10-04 and R10-06),
   from 13 Required findings of kind `new` or `regression` in the
   reports; 3 of those were downgraded and 5 parked for stage B.

Validated for T004 onward: no. 0 Blocking; 6 Required open (5 new, 1
unresolved), 0 declined; coverage and the three measurements carried
from round 8; 100 of 100 index rows with a valid shape as measured; the
re-audit raised 5 new Required clusters.

None of the six clusters cites the `sequences` step of 10 10.2, the
section T004 implements; the criteria are judged for the spec as a
whole, so the answer stays no until they hold.

What is left before T004, after the write pass below:

- the operator's answers to D1 to D6;
- a targeted re-audit of the pass, round 11, by the lenses whose
  findings drove it (L0, L1, L2, L4, L5, L6, L10, L11, L13, L14 and
  L15, with L3, L8 and L9 for their Advisory rows), reading only the
  sections it changed; the clean verdicts of L7 and L16 are carried
  forward.

## Write pass (2026-10-09)

The pass ran on branch `docs/review-round-9`, the branch of pull
request #36, at d458f83, in one scratch clone, by one build node on
`opus`. Each Required cluster was checked against the head before it
was applied, and each held as its Verification cell states. Pull
request #36 already carries the approval lines this delta needs:
`spec` (the index, 01 and 05) and `decisions` (ADR 0010).

Resolved: 6 Required clusters, all A; 3 downgraded clusters, 2 A and
1 M; 14 Advisory rows, 12 A, 1 M and 1 operator (L13-R10-6, D5). D1 to
D4 were applied as recommended. Nothing under `tools/**`, `.github/**`
or `.githooks/**` changed; where a resolution describes a check or a
glob that does not exist yet, the spec states it for the task that
builds it.

Three choices the rows left open were made in the pass, each stated in
its row: the main package of `pr` is `tools/ci/pr`, and the pr job's
`run` line is `go run ./tools/ci/pr` (R10-01); the `go` line of
`go.mod` is named beside the toolchain as a grantable input under the
job, because it sets the language version the package is compiled at
(R10-01); and `fuzz.yml` runs daily, with a `workflow_dispatch` run not
counted by release step 1 (L6-r10-3). The `gh` floor of R10-08 is
2.68.0, read on 2026-10-09 in `pkg/cmd/attestation/verify/verify.go`
of `cli/cli`: v2.67.0 has `--deny-self-hosted-runners` and not
`--source-digest`, v2.68.0 has both. The fact R10-02 rests on was
read on 2026-10-09 in GitHub's documentation: a re-run uses the
`GITHUB_SHA` and `GITHUB_REF` of the original event
([re-running workflows and jobs](https://docs.github.com/en/actions/how-tos/manage-workflow-runs/re-run-workflows-and-jobs)),
and under `pull_request_target` `GITHUB_SHA` is the last commit on the
default branch, with `edited` and `reopened` among the event's types
([events that trigger workflows](https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows)).

## Session checkpoint (2026-10-09)

Branch and pull request state is not recorded here; the next session
measures it with `handoff_state.py`.

### Done

- Round 10 ran: seventeen lenses on d458f83, then the consolidation.
- Its write pass was applied on branch `docs/review-round-9`, the
  branch of pull request #36, as described above.

### Open for the operator

- **D5 and D6**, the re-measure of the direct-push refusal under the
  current ruleset, with the operator present.
- **D1 to D4**, the answers to the items the write pass applied as
  recommended.
- **N2 of pull request #34**, a system-wide mise config that reaches
  the hook's `tools/ci`.
- **The stage-B items** P1 to P4 of this page, with those of
  round-8.md and round-9.md.

### Next steps

- Round 11, the targeted re-audit of this delta, by the lenses listed
  under "What is left before T004".
