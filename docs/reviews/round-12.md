# Spec review round 12: the targeted re-audit of round 11

Reader: reviewers checking that every finding of round 12 was handled,
and maintainers tracing why the spec reads as it does. Type: reference
(history). The current state is [docs/spec.md](../spec.md); round 11 is
in [round-11.md](round-11.md); the lenses are defined in
[lenses.md](lenses.md).

Round 12 is step 6 of "How stage A runs, and when the spec counts as
validated" in lenses.md: the targeted re-audit of the round-11 write
pass. Each lens read the sections that pass changed, checked the
round-11 findings it had raised against the new text, and reported what
was left, what regressed and what the new text added. lenses.md is
cited at the baseline commit.

The baseline is commit 6fd6e60, the head of pull request #36 (the
round-9, round-10 and round-11 write passes, branch
`docs/review-round-9`), read from a detached clone that nothing wrote
to while the reviewers ran. It is not a commit of the default branch,
as step 1 of lenses.md asks for a full round: the write passes are
validated before they merge, so the re-audit reads the pull request's
head. The delta read is d133542..6fd6e60.

Every item below is a finding of a reviewer or a proposal of the
consolidation. Where a row needs the operator, the item is in
"Decisions for the operator" below with a recommendation, and the
decision is recorded next to it, with its date, when it comes.

Status, as in rounds 6 to 11: **A** = applied as proposed; **M** =
applied in a smaller or different form than proposed, as its row
states; **L** = stated as a limit, no mechanism added; **operator** =
not applied, and the item is in "Decisions for the operator". The
statuses below are the consolidation's proposals; the write pass
records the ones it applies.

The reports are kept in `round-12/reports/<lens>.json`, as the
reviewers wrote them.

## How the round was run

Seven reviewers, one per lens that raised a round-11 finding the write
pass resolved: L0, L2, L3, L6, L12, L13 and L14, each in a fresh
context, read-only, in parallel on 6fd6e60. L2 and L13 ran on `opus`,
the rest on `sonnet`. Each lens ran once, with one reader: a re-audit
carries no noise probe, since the probe measures a lens set on a full
read, and round 8 recorded it. The verdicts of L1, L4, L5, L7, L8, L9,
L10, L11, L15 and L16 carry forward from round 11. Each reviewer wrote
one JSON report with `prior_checked`, `prior_resolved_as_stated` and
its findings, each of kind `unresolved` (a round-11 finding the pass
did not resolve as its row states), `regression` (the pass broke what
held) or `new` (a defect in text the pass wrote or a seam it opened).
No report is missing.

The reports hold 17 findings: 0 Blocking, 3 Required, 14 Advisory; 16
new, 1 unresolved, 0 regression. One consolidator merged them by
section and defect, kept every lens on its row, and checked each
Required finding against the baseline text. Of the 3 Required findings,
1 holds as reported, 1 holds in a narrower form, and 1 was downgraded
to Advisory; none was dropped. The result is 2 Required rows, 1
downgraded row, 7 Advisory rows (one of them joining three lenses) and
2 items parked for stage B.

### Per lens

`prior_checked` is the number of round-11 findings the lens re-read
against the new text, and `prior_resolved_as_stated` how many of them
read as their round-11 row states. Findings parked for stage B in round
11 (P1, P3, P5, P6) were not re-read. Severities in this table are the
reviewers'; the consolidation's are in the findings below.

| Lens | Model | prior_checked | prior_resolved_as_stated | Findings (B / R / A) | new | unresolved | regression |
|---|---|---|---|---|---|---|---|
| L0 | sonnet | 2 | 2 | 2 (0 / 0 / 2) | 2 | 0 | 0 |
| L2 | opus | 4 | 4 | 4 (0 / 2 / 2) | 4 | 0 | 0 |
| L3 | sonnet | 3 | 3 | 2 (0 / 0 / 2) | 2 | 0 | 0 |
| L6 | sonnet | 2 | 1 | 3 (0 / 0 / 3) | 2 | 1 | 0 |
| L12 | sonnet | 2 | 2 | 1 (0 / 0 / 1) | 1 | 0 | 0 |
| L13 | opus | 4 | 4 | 5 (0 / 1 / 4) | 5 | 0 | 0 |
| L14 | sonnet | 1 | 1 | 0 (0 / 0 / 0) | 0 | 0 | 0 |
| all | | 18 | 17 | 17 (0 / 3 / 14) | 16 | 1 | 0 |

The one prior finding reported as not resolved is L6-r11-2, a member
of R11-01. Its round-11 row reads "M: ... requirement (b)", and
requirement (b) of plan T005 is in the text: the code `pr` builds
imports only the standard library and packages on `approvals`, held by
a test. The consolidation reads L6-r11-2 as resolved as its row states;
what L6 asks for beyond it (a red fixture, `-test`, the standard-library
rule) is new text for that requirement, kept in R12-06. L14 reports no
finding.

## Findings and resolutions

One row per cluster. The Required rows come first, then the downgraded
row, then the Advisory rows. Kind is the cluster's: `unresolved` when
any member is, else `regression` when any member is, else `new`.
Sections are cited as the spec cites itself.

### Required

| Cluster | Section | Lenses | Kind | Defect | Verification | Resolution | Status |
|---|---|---|---|---|---|---|---|
| **R12-01** | 03 3.8 (`salvage[].fingerprint`); 04 4.1 (Exit codes; Error ids); 04 4.4; 04 4.2 (the `rm`, `recreate` and `retire` rows); 08 8.5 (skipped reasons and their recovery); 01 1.6; 05 5.2 (I17) | L13 (L13-R12-1) | new | The R11-06 write put a behavior rule in a format cell: "the third salvage in a row that a changed fingerprint starts for one generation exits 1, naming that path as `worktree-changing`". The slug is a julieta skipped reason (03 3.11, 08 8.5), which makes a salvage incomplete and leads to exit 5, accepted by `--accept-loss=worktree-changing`; here it is a romeu exit 1 with no `RJ-<nnn>` row in 04 4.4, against 04 4.1's rule that every error has an id, a slug and an exit code in one table. The rule is absent from the `rm` row, 08 8.5, 01 1.6 and I17, and no salvage record field says why a salvage started, so "three in a row" cannot be counted across separate `rm` processes. | holds: 03 3.8 reads as quoted; 03 3.11 lists `worktree-changing` among `skipped[]` reasons; 08 8.5's recovery table maps it to "let the process or the operation finish, then rerun"; 04 4.1 gives exit 5 to "refused to protect data (incomplete salvage, ...)" and exit 1 to "operation failed"; 04 4.4 has `RJ-306` `salvage-incomplete` (exit 5) and no row for this case; the `salvage[]` fields of 03 3.8 are `manifestSha256`, `fingerprint` and `result` with `reasons[]` and `acceptedLoss[]`, none of which records why a salvage started; I17's E list holds no case for the third salvage. Severity: T008 builds the error table from 04 4.4 and the salvage tasks build the rerun from 03 3.8, so the gap reaches the next task that implements the cited sections | one home and one name: 04 4.4 gains an error row with its own slug, for example `fingerprint-unstable`, exit 5 (refused to protect data, 04 4.1), raised by `rm`, `recreate` and `retire`, whose hint names the path and the two recoveries (stop the writer, or add an ignored path to `salvage.excludeIgnored`) and says `--accept-loss` does not cover it, since it is not a skipped reason; 03 3.8 gains a `salvage[].cause` field (`first` or `fingerprint-changed`) from which the count is read, and the fingerprint cell links the rule instead of stating it; the `rm` row of 04 4.2 and 08 8.5 step 4 cite it; I17 gains a case: a third salvage in a row started by a changed fingerprint exits 5 with that id, and a matching fingerprint after two resumes at `sbx env rm` | A (proposed); approval line `spec` (01 1.6 and 05 5.2 are on the `spec` surface) |
| **R12-02** | plan T005 (acceptance requirement (c)); 05 5.3 (the invariant of the `pr` job); index, Deferred decisions (the row on the `pr` job's toolchain); 10 10.2 (Hygiene: a tracked `go.work`, `go.work.sum` or `vendor/`) | L2 (L2-r12-1) | new | Requirement (c) has `pr` apply the version-pin rules of `mise.toml` and `go.mod` to the head, and the toolchain row says every other input is held to the invariant by T005's acceptance. The go command that builds `pr` also reads `go.work` and `go.work.sum` from the checkout, and what refuses them is the hygiene rule of 10 10.2, run by the head's `tools/ci` on the grantable `checks` surface. A pull request granted on `checks` can relax that rule and add a `go.work` with a `godebug` or `use` directive in one diff; after the merge every `pr` job builds under it. | holds in a narrower form than reported. The mise half does not hold: the top-level `env` of the workflow holds the mise environment of 10 10.2 (`MISE_OVERRIDE_CONFIG_FILENAMES=mise.toml`, `MISE_OVERRIDE_TOOL_VERSIONS_FILENAMES=none`, `MISE_AUTO_ENV=false`), which "reaches every mise run of the workflow, the mise action's and the one of the mise shim that starts go", so another mise config path or a `.tool-versions` is not read; that file is under `.github/workflows/**`, on `approvals`, which no grant covers (05 5.3). `GOTOOLCHAIN=local` in the same table makes a `toolchain` line of `go.work` inert. The `go.work` half holds: `GOWORK=off` and `GOFLAGS=-mod=readonly` are set only in the environment `tools/ci` builds for its own steps (10 10.2, the command steps of `fast`), not for the `go run` that builds `pr`; `go.work` and `go.work.sum` are on no surface's globs (05 5.3), and the rule that refuses them is on `checks` | requirement (c) of T005 also covers every file the go command reads from the checkout beyond `go.mod`: `pr`, built from the default branch, refuses a head that adds a tracked `go.work`, `go.work.sum` or `vendor/` (the hygiene rule of 10 10.2, applied from the base), with a fixture each; or the workflow's mise environment gains `GOWORK=off`, which `setup` already requires the table to hold. The consolidation recommends the first: it keeps the rule where the other version-pin rules are applied and adds no variable to the workflow's one `env` table. The toolchain row keeps its wording | M (proposed): the `go.work`, `go.work.sum` and `vendor/` half only; the mise half does not hold |

### Downgraded to Advisory

| Cluster | Section | Lenses | Kind | Defect | Why downgraded | Resolution | Status |
|---|---|---|---|---|---|---|---|
| **R12-03** | ADR 0011 (decision 1 and its declined alternative; decision 2); 05 5.4 (the approval-line row; the token row); 12 12.4 (the name refusal); 10 10.2 (maintainer block, items d and e) | L2 (L2-r12-2, Required) | new | A required check is matched by name, and the narrowed token keeps Contents and Workflows write: a workflow on `push`, pushed on a side branch, runs at once with the `statuses: write` it asks for and can post a success under a required check's name on another pull request's head SHA; the current token can post a commit status directly. Neither ADR 0011 nor 05 5.4 names this path. | 05 5.4 already accepts, in the approval-line row, that with one account and one token the approval line "is a record and a guard against mistakes, not a barrier against an agent", and the reviewer's scenario (a `pr` check red over a missing approval line) is reached more simply by the agent writing the line, which that row covers; lenses.md asks a lens not to report what the spec lists as an accepted risk unless the acceptance is the defect. What is left is that the row speaks of the line and not of the required checks themselves, and that the platform behavior the path rests on is not measured | 05 5.4 states that the required checks, like the approval line, guard against mistakes and not against an agent that holds a token with Contents and Workflows write, since a workflow on any branch can post a check or status under a required name; and item d of the maintainer block gains a fifth try after the narrowing, on a throwaway pull request: post a status, and a check from a side-branch workflow, under a required name, and record which one the ruleset counts. ADR 0011 is not edited after acceptance; see D1 | operator (D1) |

### Advisory

Advisory findings are applied by default (lenses.md, step 4). The
Defect cell keeps the reviewer's first sentence in short; the report
holds the rest.

| Finding | Section | Lenses | Kind | Defect | Resolution | Status |
|---|---|---|---|---|---|---|
| **R12-04** | index (S2 Evidence; Deferred decisions, the local-gates row); 05 5.4 (the approval-line row); 10 10.2 (maintainer block, item a); ADR 0011 (decision 1; Consequences) | L0 (L0-r12-1), L2 (L2-r12-4), L13 (L13-R12-2) (high-signal) | new | ADR 0011 decision 1 applies only after a maintainer step still pending (round 11, "Open for the maintainer", item 1), and its Consequences say the spec says so where it names the bypass. S2 says in the present tense that "the ruleset requires the checks of every merge, the administrator's included", and the local-gates row that the ruleset "binds the administrator", while 05 5.4 still names a bypass actor for pull requests. The removed Deferred row was the one index line that tracked the bypass. | the state is written once, in item a of the maintainer block: until the maintainer step of ADR 0011, decision 1, removes the bypass actor, the administrator role bypasses the required checks on a pull request merge, reopened by the saved API answer that shows no bypass actor; S2 and the local-gates row say "once that step is applied" and link item a; until then the run, not the ruleset, is S2's evidence | A |
| **R12-05** | 12 12.4 (`approvalLine` in `tools/ci/askfirst.go`); 05 5.3 (the `approvals` globs); 02 2.1 (the `tools/ci/` line); plan T005 (requirements (a) and (b)); 12 12.3 (the generator) | L0 (L0-r12-2), L13 (L13-R12-5) (high-signal) | new | The write pass left where the code of `pr` lives to T005, but 12 12.4 still places `approvalLine` in `tools/ci/askfirst.go`, a file of the main package `tools/ci` that a separate package cannot import, and the `approvals` glob names that file; requirements (a) and (b) use two names for one set ("a path no checkpoint grant covers", "packages on `approvals`"); and no requirement holds the reading of `.github/ask-first.yaml` by `pr` equal to the generator's of 12 12.3, the core of L13-R11-2. | 12 12.4's deferral sentence adds that the file holding `approvalLine` is decided by the same task, which moves the glob with it; (a) and (b) both say "paths on `approvals`"; T005 gains one requirement: every committed `.github/ask-first.yaml` and `docs/grants.yaml` gives `pr` and the generator of 12 12.3 the same surface set, held by a test, or both use one reader | A |
| **R12-06** | plan T005 (the four requirements under "The invariant of 05 5.3 holds"); 05 5.3; 12 12.4 (the name refusal, the head pr-job event, the printing of author text, the hostile git fixture) | L3 (L3-R12-1), L6 (L6-r12-1, L6-r12-2) | unresolved | The four requirements are unlettered bullets that the round-11 page cites as (a) to (d); the closure requirement names no comparison rule and no fixture that turns it red (L6-r12-1, reported as `unresolved` of L6-r11-2); and the fixtures 12 12.4 requires (a head job whose check name equals the pr job's, a head version of the pr job's file naming another event, the escape cases, the hostile fixture repository) are not acceptance lines of T005. | letter the four bullets (a) to (d); (b) adds the comparison (each package of `go list -deps -test` mapped to its directory and matched to an `approvals` glob, a path whose first element has no dot read as the standard library) and a fixture package importing a path outside the globs, which turns the test red; T005 gains one acceptance line per fixture 12 12.4 names, each with the removal that turns it red | A |
| L2-r12-3 | 12 12.4 (the name refusal) | L2 | new | The refusal compares the written check name; a job `name` that holds a `${{ }}` expression (a literal or a matrix value) posts a check name the comparison does not see. | `pr` refuses a head job outside the pr job's file whose `name` holds `${{`; fixtures for a literal expression and a matrix value | A |
| L13-R12-3 | index (Deferred decisions, the token-narrowing row); ADR 0011 (Consequences) | L13 | new | The token-narrowing row's first cell still names the narrowing, which ADR 0011 decided; what stays open, a later review of the sandbox's GitHub access, no cell names, so the row fails the "still open" part of exit criterion 3. | the first cell reads what is still open: reviewing the development sandbox's GitHub access again after the narrowing (a narrower permission set, a GitHub App; ADR 0008, ADR 0011); its "Until then", events and count stay | A |
| L13-R12-4 | index (Deferred decisions: the operator signature over `checksums.txt`; the create-only host ref per fetched origin head); ADR 0011 (decision 2) | L13 | new | Decision 2 lists two rows that the narrowing fires; two more rows name the same event as "the token narrowing of ADR 0008". One event, two names, and an incomplete list. | the two trigger cells read "the token is narrowed (the token narrowing row above)", as the other two do; ADR 0011 is not edited | A |
| L6-r12-3 | 10 10.2 (the `gh` floor test); 12 12.1 | L6 | new | The test that the `gh` pinned in `mise.toml` is not below the floor of 12 12.1 names no fixture pair and does not say what happens when 12 12.1 holds no parseable floor. | the test names its fixtures (a pin below the floor fails, a pin at the floor passes) and fails when 12 12.1 holds no parseable floor; the task that builds it is parked for stage B (P6) | M (proposed): the spec half; the owning task joins P6 |

## Parked for stage B

Findings on `plan.md` alone, kept for the plan review. Both join a
round-11 item of the same name.

- **P1, the pr job's task.** L12 (L12-R12-1): T005's Ask-first field
  lists `checks` only, while requirement (b) edits the `approvals`
  globs in `.github/ask-first.yaml`, so the task touches the
  `approvals` and `ask-first` surfaces too, and 12 12.2 counts an
  ask-first surface as overlapping every in-flight pull request. L3
  (L3-R12-2): 12 12.4 says the pull request of the task that builds
  `pr` updates the spelling `tools/ci pr` where it appears, and T005
  has no acceptance line for that update.
- **P6, the `gh` floor in the plan.** L6 (L6-r12-3): no task owns the
  test that holds the `gh` pin at or above the floor.

## Decisions for the operator

Everything the consolidation cannot decide, in one block, with a
recommendation; the recommendation is not a decision.

- **D1 (R12-03), what the required checks are worth against a token
  holder.** The finding asks to state, or close, a path the ruleset
  leaves: a status or check posted under a required name by a workflow
  on a side branch. Options: (a) state it in 05 5.4: the required
  checks guard against mistakes, not against an agent holding a token
  with Contents and Workflows write, as the approval-line row already
  says of the line, and add the fifth try to item d of the maintainer
  block after the narrowing; (b) as (a), and also remove Workflows
  write from the narrowed token, so a workflow change goes through the
  maintainer's host, at the cost of every workflow change leaving the
  sandbox's reach, which supersedes part of ADR 0011 decision 2 in a
  new record; (c) leave the text as it is. Recommendation: (a): it
  writes down what the accepted row already implies, measures the
  platform behavior the path rests on, and leaves (b) to be decided
  with that measurement. Approval line: `spec`.

## Exit criteria of stage A

The five criteria of lenses.md, "How stage A runs, and when the spec
counts as validated", judged on 6fd6e60:

1. **No Blocking open; every Required fixed or declined with its
   decision and date: not met.** 0 Blocking. 2 Required rows are open
   (R12-01, R12-02), both new, none fixed and none declined. The 3
   Required clusters of round 11 read as resolved in every report that
   re-read them.
2. **Coverage: carried from round 8, as in rounds 9 to 11.** A
   re-audit reads only the changed sections, and its reports carry no
   complete coverage table (one report, L2, has one), so the round-8
   measurement stands: every page with a finding or an explicit
   "nothing for this lens" from at least two lenses; the criteria and
   journeys unproven beyond that ([round-8.md](round-8.md#coverage)).
3. **The open questions and deferred decisions of the index have a
   valid shape: met for 100 of 101 rows as measured.** 101 rows: 24
   open questions and 77 deferred decisions (36 in the product, 11
   with the runtime ledger, 30 in how this repository is run, one fewer
   than round 11: the row "required status checks without the
   administrator bypass" left with ADR 0011). Every row has all its
   cells filled, and no two rows share a first cell, measured by a
   script over the tables at 6fd6e60. One row fails "still open": the
   token-narrowing row's first cell names a decided item (L13-R12-3).
   Duplicates in meaning were not measured beyond the reviewers'
   reading; two rows name the narrowing event in another form
   (L13-R12-4). That the toolchain row's "Until then" holds depends on
   R12-02.
4. **The three measurements: carried from round 8.** The `listed` to
   `own` ratio per lens, the clusters only L0 raised and the overlap of
   the L2 and L13 noise-probe pairs are in
   [round-8.md](round-8.md#measurements). This round ran no noise probe
   and its reports record no origin per finding. L0 raised two
   findings, and each joined a cluster other lenses raised (R12-04,
   R12-05), so no finding is L0's alone. L2's lens critique repeats the
   round-11 request for a viewpoint on what the CI platform trusts;
   R12-02 and R12-03 sit there. That is input for the lens rewrite
   before stage B.
5. **The targeted re-audit raises no new Required finding: not met.**
   It raised 2 Required rows, both of kind `new`: R12-01 from the text
   the R11-06 Advisory row wrote in 03 3.8, and R12-02 from the scope
   of requirement (c) of T005 (R11-03). Of the 3 Required findings
   reported, 1 holds, 1 holds narrowed and 1 was downgraded. The count
   fell from 3 new Required clusters in round 11 to 2.

Validated for T004 onward: no. 0 Blocking; 2 Required open (2 new), 0
declined; coverage and the three measurements carried from round 8;
100 of 101 index rows with a valid shape as measured; the re-audit
raised 2 Required rows.

Neither Required row cites the `sequences` step of 10 10.2, the section
T004 implements; the criteria are judged for the spec as a whole, so
the answer stays no until they hold.

What is left before T004:

- D1 of this page, answered;
- the write pass of this page's rows, with the `spec` approval line;
- a targeted re-audit of that pass, round 13, by the lenses whose
  findings drive it, L0, L2, L3, L6, L13, reading only the sections it
  changes; the verdicts of the other lenses are carried forward.
