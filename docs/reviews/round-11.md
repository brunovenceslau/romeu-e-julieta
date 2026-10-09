# Spec review round 11: the targeted re-audit of round 10

Reader: reviewers checking that every finding of round 11 was handled,
and maintainers tracing why the spec reads as it does. Type: reference
(history). The current state is [docs/spec.md](../spec.md); round 10 is
in [round-10.md](round-10.md); the lenses are defined in
[lenses.md](lenses.md).

Round 11 is step 6 of "How stage A runs, and when the spec counts as
validated" in lenses.md: the targeted re-audit of the round-10 write
pass. Each lens read the sections that pass changed, checked the
round-10 findings it had raised against the new text, and reported what
was left, what regressed and what the new text added. lenses.md is
cited at the baseline commit.

The baseline is commit d133542, the head of pull request #36 (the
round-9 and round-10 write passes, branch `docs/review-round-9`), read
from a detached clone that nothing wrote to while the reviewers ran. It
is not a commit of the default branch, as step 1 of lenses.md asks for
a full round: the write passes are validated before they merge, so the
re-audit reads the pull request's head. The delta read is
d458f83..d133542.

Every item below is a finding of a reviewer or a proposal of the
consolidation. Where a row needs the operator, the item is in
"Decisions for the operator" below with a recommendation, and the
decision is recorded next to it, with its date, when it comes.

Status, as in rounds 6 to 10: **A** = applied as proposed; **M** =
applied in a smaller or different form than proposed, as its row
states; **L** = stated as a limit, no mechanism added; **operator** =
not applied, and the item is in "Decisions for the operator". In this
draft the status is the one proposed; the write pass confirms it.

The reports are kept in `round-11/reports/<lens>.json`, as the
reviewers wrote them.

## How the round was run

Seventeen reviewers, one per lens, L0 to L16, each in a fresh context,
read-only, in parallel on d133542. L2 and L13 ran on `opus`, the rest
on `sonnet`. Each lens ran once, with one reader: a re-audit carries no
noise probe, since the probe measures a lens set on a full read, and
round 8 recorded it. Each wrote one JSON report with `prior_checked`,
`prior_resolved_as_stated` and its findings, each of kind `unresolved`
(a round-10 finding the pass did not resolve as its row states),
`regression` (the pass broke what held) or `new` (a defect in text the
pass wrote or a seam it opened). No report is missing.

The reports hold 28 findings: 0 Blocking, 11 Required, 17 Advisory; 26
new, 0 unresolved, 2 regression. One consolidator merged them by
section and defect, kept every lens on its row, and checked each
Required finding against the baseline text. Of the 11 Required
findings, 8 hold as Required (3 clusters, two of them joined by
Advisory findings of other lenses), 2 were downgraded to Advisory, and
1 is a finding on `plan.md` alone, parked for stage B as lenses.md
says; none was dropped. The result is 3 Required clusters, 11 Advisory
rows (2 of them downgraded) and 5 findings parked for stage B.

All three Required clusters come from one resolution of round 10,
R10-01: `pr` built from a main package of its own, `tools/ci/pr`, whose
import closure is the standard library and paths on `approvals`. The
rule holds as a goal; the texts around it do not yet let it be built.

### Per lens

`prior_checked` is the number of round-10 findings the lens re-read
against the new text, and `prior_resolved_as_stated` how many of them
read as their round-10 row states. Findings parked for stage B in round
10 (P1 to P4) were not re-read, so L9 and L12 check none. Severities in
this table are the reviewers'; the consolidation's are in the findings
below.

| Lens | Model | prior_checked | prior_resolved_as_stated | Findings (B / R / A) | new | unresolved | regression |
|---|---|---|---|---|---|---|---|
| L0 | sonnet | 1 | 1 | 2 (0 / 1 / 1) | 2 | 0 | 0 |
| L1 | sonnet | 2 | 2 | 1 (0 / 0 / 1) | 1 | 0 | 0 |
| L2 | opus | 4 | 4 | 4 (0 / 2 / 2) | 4 | 0 | 0 |
| L3 | sonnet | 1 | 1 | 3 (0 / 1 / 2) | 2 | 0 | 1 |
| L4 | sonnet | 2 | 2 | 3 (0 / 1 / 2) | 3 | 0 | 0 |
| L5 | sonnet | 2 | 2 | 0 (0 / 0 / 0) | 0 | 0 | 0 |
| L6 | sonnet | 4 | 4 | 2 (0 / 1 / 1) | 2 | 0 | 0 |
| L7 | sonnet | 0 | 0 | 0 (0 / 0 / 0) | 0 | 0 | 0 |
| L8 | sonnet | 1 | 1 | 0 (0 / 0 / 0) | 0 | 0 | 0 |
| L9 | sonnet | 0 | 0 | 4 (0 / 1 / 3) | 4 | 0 | 0 |
| L10 | sonnet | 2 | 2 | 0 (0 / 0 / 0) | 0 | 0 | 0 |
| L11 | sonnet | 1 | 1 | 0 (0 / 0 / 0) | 0 | 0 | 0 |
| L12 | sonnet | 0 | 0 | 2 (0 / 1 / 1) | 2 | 0 | 0 |
| L13 | opus | 5 | 5 | 4 (0 / 2 / 2) | 4 | 0 | 0 |
| L14 | sonnet | 1 | 1 | 1 (0 / 1 / 0) | 1 | 0 | 0 |
| L15 | sonnet | 2 | 2 | 2 (0 / 0 / 2) | 1 | 0 | 1 |
| L16 | sonnet | 0 | 0 | 0 (0 / 0 / 0) | 0 | 0 | 0 |
| all | | 28 | 28 | 28 (0 / 11 / 17) | 26 | 0 | 2 |

Every prior finding a lens re-read resolves as its round-10 row states.
The two regressions are L3-R11-2 (the command block of 12 12.2 still
runs `pr` as a subcommand of `tools/ci`) and L15-R11-1 (the README
prerequisite of 12 12.8 still names no `gh` floor). L5, L7, L8, L10,
L11 and L16 report no finding.

## Findings and resolutions

One row per cluster. The Required rows come first, then the two
downgraded rows, then the Advisory rows in lens order. Kind is the
cluster's: `unresolved` when any member is, else `regression` when any
member is, else `new`. Sections are cited as the spec cites itself.
Two reports cite the command block as 12 12.1 or 12 12.4; it is in
12 12.2, and the rows say so.

### Required

| Cluster | Section | Lenses | Kind | Defect | Verification | Resolution | Status |
|---|---|---|---|---|---|---|---|
| **R11-01** | 05 5.3 (the `pr` package paragraph; the `approvals` globs); 12 12.4 (`approvalLine` in `tools/ci/askfirst.go`; "Where this specification writes `tools/ci pr`"); 12 12.2 (Development commands, the two PR checks lines); 02 2.1 (the `tools/ci/` line); 10 10.7 | L13 (L13-R11-1), L3 (L3-R11-1, L3-R11-2), L12 (L12-R11-1), L14 (L14-r11-1), L0 (L0-r11-2), L6 (L6-r11-2) (high-signal) | regression | The delta moved `pr` into the main package `tools/ci/pr` and left the texts that place its code as they were: 12 12.4 defines `approvalLine` in `tools/ci/askfirst.go`, a file of the main package `tools/ci`, which Go cannot import; the `approvals` globs name files, while `go list -deps` lists packages, and no text says how one is matched against the other or which fixture turns the test red; 02 2.1 lays `tools/ci` out as one dispatcher with one package per step and does not name `tools/ci/pr`; and the command block of 12 12.2 still gives `go run ./tools/ci pr <payload file>` and `go run ./tools/ci pr --title <t> --body <file>`. | holds: 12 12.4 reads "`approvalLine` in `tools/ci/askfirst.go`"; `.github/ask-first.yaml` and the copy in 05 5.3 glob `tools/ci/askfirst.go` and `tools/ci/denylist.yaml`; 02 2.1 reads as quoted; 12 12.2 lines 164 and 165 read as quoted, while 10 10.2 (the `run` step and pr job rows, the local-run paragraph) says `go run ./tools/ci/pr`; the alias sentence of 12 12.4 renames the prose name, not a command line | one home: the approval-line, grant and forbidden-name checks live under `tools/ci/pr/` (the main package, and packages below it if needed), and 12 12.4 defines `approvalLine` there; 05 5.3 says the `approvals` glob `tools/ci/askfirst.go` becomes `tools/ci/pr/**` in the tooling pull request of the task that builds `pr`, which R10-01 already names, with the file glob dropped in the same change; 05 5.3 says the test maps each package of `go list -deps` (with `-test`) to its directory, treats a path whose first element has no dot as the standard library, requires every other package's directory to match an `approvals` glob, and names a fixture package that imports a path outside the globs; 02 2.1 names `tools/ci/pr/` as the one main package outside the dispatcher; 12 12.2's two lines become `go run ./tools/ci/pr ...`; 10 10.7 names the closure test beside `tools/ci imports`, so the boundaries have one list (L3-R11-3 below) | A (spec text; the glob with the task that builds `pr`) |
| **R11-02** | 05 5.3 ("whose import closure is the standard library and paths on `approvals`"); 12 12.1 (Go dependencies: `go.yaml.in/yaml/v3`, `github.com/BurntSushi/toml`); 12 12.4 (`pr` reads `.github/ask-first.yaml` at base and head, `docs/grants.yaml`, the head's `.github/workflows/`); 12 12.3; 10 10.2 (the forbidden-name check, `tools/ci/denylist.yaml`) | L2 (L2-r11-1), L13 (L13-R11-2), L0 (L0-r11-1) (high-signal) | new | `pr` decodes YAML (`.github/ask-first.yaml` at base and head, `docs/grants.yaml`, `tools/ci/denylist.yaml`, and the head's workflows for the name refusal), the standard library has no YAML decoder, and the one 12 12.1 names, `go.yaml.in/yaml/v3`, is a module of `go.mod`, on `dependencies` and outside the `approvals` globs; no text says how `pr` decodes them, and the same files are read with the library by the generator of 12 12.3 and by `tools/ci hygiene`, so two readings of the surface list could differ. The git runner has the same gap in a smaller form: `internal/gitsafe` is on the `gitsafe` surface, not on `approvals`. | holds: 12 12.1 lists the YAML and TOML modules as the complete v1 list; the `dependencies` globs hold `go.mod` and `go.sum`, and `dependencies` is grantable (12 12.4, A checkpoint grant); the `approvals` globs hold no decoder; the git flags `pr` passes are listed in 12 12.4, so `pr` can carry its own git calls without `internal/gitsafe` | 05 5.3 and 12 12.4 state the reader: one strict decoder of the YAML subset these files use (block mappings and sequences, plain and quoted scalars; no anchors, aliases, tags, merge keys or duplicate keys), under `tools/ci/pr/` on `approvals`, with a fuzz target in `fuzz.yml`; the generator of 12 12.3 and `hygiene` read `.github/ask-first.yaml` and `docs/grants.yaml` through it too, so the files have one reading, and a test fails a committed file the subset does not decode; `pr` runs git through its own calls with the flags of 12 12.4 and imports no `internal/` package; see D1 | A, after D1 |
| **R11-03** | 05 5.3 ("so code a checkpoint grant covers never runs in the job that judges the grants"; "The grantable inputs left under the job"); 12 12.4 (the `pr` job builds with the base's `go.mod`, `go.sum`, `mise.toml` and `mise.lock`); 10 10.2 (the mise-table and `go.mod`-directive rules); index, Deferred decisions (the row on the `pr` job's toolchain) | L2 (L2-r11-2) | new | 05 5.3 says the grantable inputs left under the job are a toolchain version and the `go` line, but `mise.toml` can hold an `[env]` table (for example `GOFLAGS=-toolexec=...`) and `go.mod` a `toolchain` or `godebug` directive; the rules that keep those files to versions are tests of 10 10.2 run by the head's `tools/ci`, on the grantable `checks` surface, and `pr` does not apply them. A pull request granted on `checks` and `dependencies` can loosen the rule and add the table in one diff; after its merge every `pr` job compiles under that table, and the Deferred row's trigger fires only then. | holds as a reading of the text: 10 10.2 lists the two rules among the tests of `tools/ci`; the grant list of 12 12.4 includes `checks` and `dependencies`; 12 12.4 builds `pr` with the base's mise files. That the mise action exports `[env]` to the go it starts is inferred from 10 10.2's own sentence on the mise shim, not measured | `pr`, from the base, applies the mise-table rule and the `go.mod`-directive rule of 10 10.2 to the head's `mise.toml`, `mise.lock` and `go.mod` when the diff touches them, each with a fixture, reading TOML through a strict subset reader under `tools/ci/pr/` (R11-02); 05 5.3 reads "the grantable inputs left under the job are the versions those files pin, held to that shape by `pr`"; the Deferred row keeps option (b) of round 10's D2 deferred, with its trigger unchanged; see D2 | A, after D2 |

### Downgraded to Advisory

| Cluster | Section | Lenses | Kind | Defect | Why downgraded | Resolution | Status |
|---|---|---|---|---|---|---|---|
| **R11-04** | 03 3.8 (`salvage[].fingerprint`); 01 1.6 (the `salvaging -> removing` row; Recovery); 08 8.5 step 2 | L4 (L4-r11-1, Required) | new | The fingerprint is "written in the same whole-file write as the state `removing`", and no text says its values are those step 2 of 08 8.5 captured and read again; read fresh at that write, a change made between the capture and the write is in neither the bundle nor the fingerprint, and a rerun after a failed `sbx env rm` resumes over it. | `rm`, `recreate` and `retire` always stop the agents (04 4.2, the `romeu salvage` row: `julieta salvage --stop-agents`), so the reviewer's scenario of an agent committing does not arise; a change by another process in that window is lost the same way by a first `sbx env rm` that succeeds, which I17 accepts by design (the recheck of step 2 is where other processes are caught). Taking the fingerprint from the step-2 read narrows the window on a rerun only | 03 3.8 says the fingerprint is built from the HEAD, status and daemon-head values that step 2 of 08 8.5 and the host verification read, not read again at the `removing` write | A |
| **R11-05** | 05 5.4 (I17, the E list); 01 1.6; 03 3.8 | L6 (L6-r11-1, Required) | new | The reviewer reads I17 as naming only the equal-fingerprint case, so that a resume without the comparison leaves every test green. | does not hold as stated: I17 already names `removing -> salvaging` with a new salvage id when the fingerprint differs, an uncommitted edit after a failed `sbx env rm` that makes the resumed `rm` salvage again, and an already-dirty worktree edited again; an unconditional resume fails those cases. What is left is narrower: no case moves a daemon head or changes an ignored file between the failed `sbx env rm` and the rerun | I17 gains the two cases: a daemon head moved, and an ignored file changed (within the scope R11-06 settles), each making the resumed `rm` salvage again | M: the two missing cases only |

### Advisory

Advisory findings are applied by default (lenses.md, step 4). The
Defect cell keeps the reviewer's first sentence in short; the report
holds the rest. Three Advisory findings joined R11-01 (L0-r11-2,
L3-R11-2, L6-r11-2) and are not repeated here.

| Finding | Section | Kind | Defect | Resolution | Status |
|---|---|---|---|---|---|
| R11-06: L1-r11-1, L4-r11-2 | 03 3.8 (`salvage[].fingerprint`); 01 1.6; J7 step 5; 08 8.5 steps 2 and 4 | new | The fingerprint digests every ignored path with its file's digest: a large ignored tree makes a rerun slow with no output, and a process that keeps writing an ignored file (a log, a cache) changes it on every rerun, so the generation salvages again each time and never resumes at `sbx env rm`. L4-r11-2 says salvage captures no ignored file; step 4 of 08 8.5 does capture them, up to the cap, so the row keeps the cost and churn halves. | 03 3.8 says the fingerprint digests tracked changes and untracked paths with their files, and ignored paths, minus `salvage.excludeIgnored`, by path, size and modification time; a rerun that salvages again prints the first path whose entry changed; the third consecutive salvage of one `rm` exits 1 with `worktree-changing` naming that path | A |
| L2-r11-3 | 12 12.4 (the name refusal); 10 10.2 (pr job row) | new | The name refusal exempts the pr job's own file and compares only a job's `name`, so a head that adds `pull_request` to that file, or a job with no `name` whose id equals the pr job's name, posts a check of the required name. | the refusal compares each head job's check name (its `name`, else its id) with the pr job's name, and `pr` refuses a head whose version of the pr job's file names an event other than `pull_request_target`; a fixture for each. The accepted row of 05 5.4 on approval lines stays as it is | A |
| L2-r11-4 | 12 12.4 (neutralizing author text) | new | Neutralizing "a leading `::`" is narrower than what the runner reads (a `::` after leading whitespace, the older `##[` form). Rests on the runner's code as remembered, not measured. | `pr` prints each line of author text behind a fixed prefix (a `>` and a space), so no printed line starts with author bytes, and escapes `##[`; the fixture "pr escapes a workflow command in a PR body" gains a leading space, a leading U+00A0 and a `##[` case | A |
| L3-R11-3 | 05 5.3 (the `go list -deps` test); 10 10.7; 10 10.2 (imports row) | new | Import boundaries now have two enforcers, `tools/ci imports` (10 10.7) and the closure test of `tools/ci/pr`, and 10 10.7 and the imports row name only the first. | 10 10.7 gains a row for `tools/ci/pr` and the imports row of 10 10.2 cites the closure test, so the boundaries have one list (with R11-01) | A |
| L4-r11-3 | J7 step 4 | new | "no ref whose work is kept nowhere else" is a judgement, and `git status` does not cover linked worktrees or stashes kept in the attic. | J7 step 4 names `git -C <clone> log --branches --refs=refs/romeu/salvage --not --remotes --oneline`, `git stash list` and `git worktree list` as the check | A |
| L13-R11-3 | 05 5.4 (the "Verified" signature row); 06 6.4; J1 step 5 | new | 06 6.4 and J1 step 5 make the askpass program a requirement, while the 05 5.4 row still says `ssh-add -c` "gives confirm-on-use where an askpass program is installed (macOS ships none)". | the row reads "which gives confirm-on-use through the askpass program J1 step 5 installs (macOS ships none)" | A |
| L13-R11-4 | 04 4.2 (doctor table, `generation-sandbox`); 01 1.7 | new | "a sandbox a project names with no open generation, a generation in `removing` excluded" can be read as removing the case from the check or as not counting a `removing` generation, which makes the row match what `removal-pending` owns. | the row reads "a sandbox a project names with no generation in `open`, `salvaging` or `removing`" | A |
| L15-R11-1 | 12 12.8 (the `README.md` row, prerequisites) | regression | The README prerequisite still says "a `gh` with the `attestation` command", while J1 step 1 and 12 12.1 require the floor, 2.68.0. | the prerequisite reads "a `gh` at or above the floor of 12 12.1" | A |
| L15-R11-2 | 10 10.2 (Verification contract); 12 12.1 | new | The `gh` pinned in `mise.toml` fixes the development sandbox's flags and 12 12.1 sets the operator's floor, and nothing holds the pin at or above the floor. | a `tools/ci` test fails when the `gh` pin of `mise.toml` is below the floor, which 12 12.1 states once and the test reads; a fixture | A |

## Parked for stage B

Findings on `plan.md` alone, kept for the plan review. One of them was
Required for its lens; it is not counted as an open Required finding of
the spec, as lenses.md says for a finding on the plan. P1 and P3 join
the round-10 items of the same name; P5 and P6 are new.

- **P1, the pr job's task.** L9 (L9-R11-1, Required): T005 still names
  the step `tools/ci pr`, its Verify line runs `go run ./tools/ci pr
  <payload file>`, and no acceptance line holds the closure test.
  L12 (L12-R11-2): no text orders the glob change of
  `.github/ask-first.yaml` before the first file under `tools/ci/pr/`;
  the spec half holds, since 05 5.3 puts the glob in the tooling pull
  request of the task that builds `pr`.
- **P5, the removing resume in T068.** L9 (L9-R11-2): T068 has no
  acceptance line for a rerun on a `removing` generation, equal or
  changed fingerprint.
- **P3, the pre-tag fuzz run.** L9 (L9-R11-3): T087 and O7 do not say
  that only a scheduled `fuzz.yml` run counts and that it runs daily.
- **P6, the `gh` floor in the plan.** L9 (L9-R11-4): no task records
  the `gh` version in B1 or fails `tools/release verify` below the
  floor.

## Decisions for the operator

Everything the consolidation cannot decide, in one block. Each item
names the finding, the options and the consolidation's recommendation;
the recommendation is not a decision. D5 and D6 of round 10 still wait
for their answer and are not repeated.

- **D1 (R11-02), how `pr` reads its YAML.** Options: (a) a strict
  YAML-subset decoder under `tools/ci/pr/`, on `approvals`, fuzzed,
  which the generator of 12 12.3 and `hygiene` use for the same files;
  (b) the decoder module's source copied under a path on `approvals`,
  which the closure test admits by that path, and which 12 12.1's
  dependency list and the hygiene rule on `vendor/` then have to name;
  (c) admit `go.yaml.in/yaml/v3` into the closure and name `go.sum`'s
  line for it beside the toolchain as a grantable input, which widens
  what round 10's D2 accepted. Recommendation: (a): the files use a
  small subset, one reading serves every reader, and no third-party
  code or new grantable input enters the job that judges grants.
  Approval line: `spec`; `approvals` and `ask-first` with the task that
  builds `pr`.
- **D2 (R11-03), what holds the shape of the mise files and `go.mod`
  under the `pr` job.** Options: (a) `pr`, from the base, applies the
  mise-table and `go.mod`-directive rules to the head when the diff
  touches those files; (b) pull the Deferred row forward: `mise.toml`,
  `mise.lock`, `go.mod` and `go.sum` go on `approvals`, so any change to
  them carries a line in the maintainer's words, and the row leaves the
  table; (c) state the residual as an accepted risk in 05 5.4.
  Recommendation: (a): it makes the sentence of 05 5.3 true at the cost
  of two checks and a TOML subset reader, keeps grants for version
  bumps, and leaves (b) deferred with its trigger. Approval line:
  `spec`.

## Exit criteria of stage A

The five criteria of lenses.md, "How stage A runs, and when the spec
counts as validated", judged on d133542:

1. **No Blocking open; every Required fixed or declined with its
   decision and date: not met.** 0 Blocking. 3 Required clusters are
   open (2 new, 1 regression), none fixed and none declined. The 6
   Required clusters of round 10 read as resolved in every report that
   re-read them.
2. **Coverage: carried from round 8, as in rounds 9 and 10.** A
   re-audit reads only the changed sections, and its reports carry no
   complete coverage table (one report, L2, has one), so the round-8
   measurement stands: every page with a finding or an explicit
   "nothing for this lens" from at least two lenses; the criteria and
   journeys unproven beyond that ([round-8.md](round-8.md#coverage)).
3. **The open questions and deferred decisions of the index have a
   valid shape: met as measured.** 102 rows: 24 open questions and 78
   deferred decisions (36 in the product, 11 with the runtime ledger,
   31 in how this repository is run, two more than round 10: the `pr`
   job's toolchain and the automatic new `pr` run). Every row has all
   its cells filled, and no two rows share a first cell, measured by a
   script over the tables. The two rows whose trigger fired both say
   "fired", in one form (L13-R10-3). Duplicates in meaning were not
   measured beyond the reviewers' reading. That the toolchain row's
   "Until then" holds depends on R11-03.
4. **The three measurements: carried from round 8.** The `listed` to
   `own` ratio per lens, the clusters only L0 raised and the overlap of
   the L2 and L13 noise-probe pairs are in
   [round-8.md](round-8.md#measurements). This round ran no noise probe
   and its reports record no origin per finding. L0 raised two
   findings, and each joined a cluster other lenses raised (R11-01,
   R11-02), so no finding is L0's alone. L2's lens critique asks for a
   viewpoint on what the CI platform and its supply chain trust (the
   toolchain, config files, decoders and runner image under the `pr`
   job); all three Required clusters sit there. That is input for the
   lens rewrite before stage B.
5. **The targeted re-audit raises no new Required finding: not met.**
   It raised 3 Required clusters (R11-01 to R11-03), from 8 Required
   findings of kind `new` that hold, with 1 member of kind `regression`
   (Advisory) in R11-01; of the 11 Required findings reported, 2 were
   downgraded and 1 parked for stage B. The count fell from 5 new
   Required clusters in round 10 to 3, all from one round-10
   resolution (R10-01).

Validated for T004 onward: no. 0 Blocking; 3 Required open (2 new, 1
regression), 0 declined; coverage and the three measurements carried
from round 8; 102 of 102 index rows with a valid shape as measured; the
re-audit raised 3 Required clusters, all from R10-01.

None of the three clusters cites the `sequences` step of 10 10.2, the
section T004 implements; the criteria are judged for the spec as a
whole, so the answer stays no until they hold.

What is left before T004:

- the operator's answers to D1 and D2 of this page, and to D5 and D6
  of round 10;
- one write pass for the rows above;
- a targeted re-audit of that pass, round 12, by the lenses whose
  findings drove it (L0, L2, L3, L12, L13 and L14 for the Required
  clusters, with L1, L4, L6 and L15 for their Advisory rows), reading
  only the sections it changes; the clean verdicts of L5, L7, L8, L10,
  L11 and L16 are carried forward.
