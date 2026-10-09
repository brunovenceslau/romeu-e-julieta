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
not applied, and the item is in "Decisions for the operator". The
write pass, and why its resolution of the three Required clusters
departs from the one proposed here, are in "Write pass (2026-10-09)"
below.

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
| **R11-01** | 05 5.3 (the `pr` package paragraph; the `approvals` globs); 12 12.4 (`approvalLine` in `tools/ci/askfirst.go`; "Where this specification writes `tools/ci pr`"); 12 12.2 (Development commands, the two PR checks lines); 02 2.1 (the `tools/ci/` line); 10 10.7 | L13 (L13-R11-1), L3 (L3-R11-1, L3-R11-2), L12 (L12-R11-1), L14 (L14-r11-1), L0 (L0-r11-2), L6 (L6-r11-2) (high-signal) | regression | The delta moved `pr` into the main package `tools/ci/pr` and left the texts that place its code as they were: 12 12.4 defines `approvalLine` in `tools/ci/askfirst.go`, a file of the main package `tools/ci`, which Go cannot import; the `approvals` globs name files, while `go list -deps` lists packages, and no text says how one is matched against the other or which fixture turns the test red; 02 2.1 lays `tools/ci` out as one dispatcher with one package per step and does not name `tools/ci/pr`; and the command block of 12 12.2 still gives `go run ./tools/ci pr <payload file>` and `go run ./tools/ci pr --title <t> --body <file>`. | holds: 12 12.4 reads "`approvalLine` in `tools/ci/askfirst.go`"; `.github/ask-first.yaml` and the copy in 05 5.3 glob `tools/ci/askfirst.go` and `tools/ci/denylist.yaml`; 02 2.1 reads as quoted; 12 12.2 lines 164 and 165 read as quoted, while 10 10.2 (the `run` step and pr job rows, the local-run paragraph) says `go run ./tools/ci/pr`; the alias sentence of 12 12.4 renames the prose name, not a command line | one home: the approval-line, grant and forbidden-name checks live under `tools/ci/pr/` (the main package, and packages below it if needed), and 12 12.4 defines `approvalLine` there; 05 5.3 says the `approvals` glob `tools/ci/askfirst.go` becomes `tools/ci/pr/**` in the tooling pull request of the task that builds `pr`, which R10-01 already names, with the file glob dropped in the same change; 05 5.3 says the test maps each package of `go list -deps` (with `-test`) to its directory, treats a path whose first element has no dot as the standard library, requires every other package's directory to match an `approvals` glob, and names a fixture package that imports a path outside the globs; 02 2.1 names `tools/ci/pr/` as the one main package outside the dispatcher; 12 12.2's two lines become `go run ./tools/ci/pr ...`; 10 10.7 names the closure test beside `tools/ci imports`, so the boundaries have one list (L3-R11-3 below) | M: the spec states the invariant and the decided job shape, and the mechanism claims of R10-01 leave it; each becomes an acceptance requirement of plan T005 (see Write pass); requirement (b) |
| **R11-02** | 05 5.3 ("whose import closure is the standard library and paths on `approvals`"); 12 12.1 (Go dependencies: `go.yaml.in/yaml/v3`, `github.com/BurntSushi/toml`); 12 12.4 (`pr` reads `.github/ask-first.yaml` at base and head, `docs/grants.yaml`, the head's `.github/workflows/`); 12 12.3; 10 10.2 (the forbidden-name check, `tools/ci/denylist.yaml`) | L2 (L2-r11-1), L13 (L13-R11-2), L0 (L0-r11-1) (high-signal) | new | `pr` decodes YAML (`.github/ask-first.yaml` at base and head, `docs/grants.yaml`, `tools/ci/denylist.yaml`, and the head's workflows for the name refusal), the standard library has no YAML decoder, and the one 12 12.1 names, `go.yaml.in/yaml/v3`, is a module of `go.mod`, on `dependencies` and outside the `approvals` globs; no text says how `pr` decodes them, and the same files are read with the library by the generator of 12 12.3 and by `tools/ci hygiene`, so two readings of the surface list could differ. The git runner has the same gap in a smaller form: `internal/gitsafe` is on the `gitsafe` surface, not on `approvals`. | holds: 12 12.1 lists the YAML and TOML modules as the complete v1 list; the `dependencies` globs hold `go.mod` and `go.sum`, and `dependencies` is grantable (12 12.4, A checkpoint grant); the `approvals` globs hold no decoder; the git flags `pr` passes are listed in 12 12.4, so `pr` can carry its own git calls without `internal/gitsafe` | 05 5.3 and 12 12.4 state the reader: one strict decoder of the YAML subset these files use (block mappings and sequences, plain and quoted scalars; no anchors, aliases, tags, merge keys or duplicate keys), under `tools/ci/pr/` on `approvals`, with a fuzz target in `fuzz.yml`; the generator of 12 12.3 and `hygiene` read `.github/ask-first.yaml` and `docs/grants.yaml` through it too, so the files have one reading, and a test fails a committed file the subset does not decode; `pr` runs git through its own calls with the flags of 12 12.4 and imports no `internal/` package; see D1 | M: the spec states the invariant and the decided job shape, and the mechanism claims of R10-01 leave it; each becomes an acceptance requirement of plan T005 (see Write pass); requirement (a); D1 is not needed |
| **R11-03** | 05 5.3 ("so code a checkpoint grant covers never runs in the job that judges the grants"; "The grantable inputs left under the job"); 12 12.4 (the `pr` job builds with the base's `go.mod`, `go.sum`, `mise.toml` and `mise.lock`); 10 10.2 (the mise-table and `go.mod`-directive rules); index, Deferred decisions (the row on the `pr` job's toolchain) | L2 (L2-r11-2) | new | 05 5.3 says the grantable inputs left under the job are a toolchain version and the `go` line, but `mise.toml` can hold an `[env]` table (for example `GOFLAGS=-toolexec=...`) and `go.mod` a `toolchain` or `godebug` directive; the rules that keep those files to versions are tests of 10 10.2 run by the head's `tools/ci`, on the grantable `checks` surface, and `pr` does not apply them. A pull request granted on `checks` and `dependencies` can loosen the rule and add the table in one diff; after its merge every `pr` job compiles under that table, and the Deferred row's trigger fires only then. | holds as a reading of the text: 10 10.2 lists the two rules among the tests of `tools/ci`; the grant list of 12 12.4 includes `checks` and `dependencies`; 12 12.4 builds `pr` with the base's mise files. That the mise action exports `[env]` to the go it starts is inferred from 10 10.2's own sentence on the mise shim, not measured | `pr`, from the base, applies the mise-table rule and the `go.mod`-directive rule of 10 10.2 to the head's `mise.toml`, `mise.lock` and `go.mod` when the diff touches them, each with a fixture, reading TOML through a strict subset reader under `tools/ci/pr/` (R11-02); 05 5.3 reads "the grantable inputs left under the job are the versions those files pin, held to that shape by `pr`"; the Deferred row keeps option (b) of round 10's D2 deferred, with its trigger unchanged; see D2 | M: the spec states the invariant and the decided job shape, and the mechanism claims of R10-01 leave it; each becomes an acceptance requirement of plan T005 (see Write pass); requirements (c) and (d); D2 is not needed |

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
| L2-r11-4 | 12 12.4 (neutralizing author text) | new | Neutralizing "a leading `::`" is narrower than what the runner reads (a `::` after leading whitespace, the older `##[` form). Rests on the runner's code as remembered, not measured. | `pr` prints each line of author text behind a fixed prefix (a `>` and a space), so no printed line starts with author bytes, and escapes `##[`; the fixture "pr escapes a workflow command in a PR body" gains a leading space, a leading U+00A0 and a `##[` case | A, with the runner's reading of a `::` after whitespace and of `##[` stated as remembered and not measured |
| L3-R11-3 | 05 5.3 (the `go list -deps` test); 10 10.7; 10 10.2 (imports row) | new | Import boundaries now have two enforcers, `tools/ci imports` (10 10.7) and the closure test of `tools/ci/pr`, and 10 10.7 and the imports row name only the first. | 10 10.7 gains a row for `tools/ci/pr` and the imports row of 10 10.2 cites the closure test, so the boundaries have one list (with R11-01) | M: the closure test left the spec with R11-01, so 10 10.7 and the imports row keep their one enforcer, and requirement (b) of T005 names the test |
| L4-r11-3 | J7 step 4 | new | "no ref whose work is kept nowhere else" is a judgement, and `git status` does not cover linked worktrees or stashes kept in the attic. | J7 step 4 names `git -C <clone> log --branches --refs=refs/romeu/salvage --not --remotes --oneline`, `git stash list` and `git worktree list` as the check | M: J7 step 4 names `git log --branches --glob='refs/romeu/salvage/*' --not --remotes --oneline`, since `git log` has no `--refs` option, with `git stash list` and `git worktree list` |
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
  Answered on 2026-10-09: none of the options in the spec; how `pr`
  reads its YAML and TOML is decided by the task that builds `pr`,
  under requirement (a) of plan T005 (see "Write pass (2026-10-09)").
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
  Answered on 2026-10-09: option (a), as requirement (c) of plan T005
  rather than as spec text, with (b) deferred and its trigger unchanged
  (see "Write pass (2026-10-09)").

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

What is left before T004, after the write pass below (D1 and D2 of
this page, and D5 and D6 of round 10, were answered on 2026-10-09):

- a targeted re-audit of the pass, round 12, by the lenses of the three
  Required clusters, L0, L2, L3, L6, L12, L13 and L14, reading only
  the sections it changes; the verdicts of the other lenses are carried
  forward.

## Write pass (2026-10-09)

The pass ran on branch `docs/review-round-9`, the branch of pull
request #36, at d133542, in one scratch clone, by one build node on
`opus`. Each row's status above is the one applied.

### Why R11-01 to R11-03 depart from the proposals

The proposals above would each have specified more of the internal
mechanism of `tools/ci pr`, a step that does not exist yet: a main
package and its layout (R11-01), a strict YAML-subset decoder shared
with the generator and `hygiene` (R11-02), and a TOML reader with two
more rules applied from the base (R11-03). Rounds 9, 10 and 11 each
specified more of that mechanism, and each found a new gap in what the
round before had written. Under
[ADR 0005, decide at the last responsible moment and record the trigger](../adr/0005-decide-at-the-last-responsible-moment-and-record-the-trigger.md),
the structure is decided by the task that builds `pr`, when its code
and tests can show it, and the spec keeps what is decided now: the
invariant and the requirements that make it hold. Decided on
2026-10-09.

- 05 5.3 and 12 12.4 state the invariant: no code that a pull request
  changes judges that pull request: `pr` runs from a workflow defined
  in the default branch, builds the default branch's commit, and reads
  the head only as data. The decided job shape stays in 12 12.4 and
  10 10.2: `pull_request_target`, `branches: [main]`, a checkout of
  `github.sha`, `contents: read`, no secret, the git flags, and the
  event that picks up a fixed `pr`.
- Removed from the spec, because not yet true or not yet decided: the
  main package `tools/ci/pr` and the `go list -deps` closure test of
  R10-01 (05 5.3, 12 12.4, the run-step and pr job rows of 10 10.2 and
  its local-run paragraph, which go back to `go run ./tools/ci pr`);
  "code a checkpoint grant covers never runs in the job that judges
  the grants" (05 5.3); and the reading of 05 5.3 and of the index row
  on the toolchain that every input of the job but the toolchain and
  the `go` line is covered, which said nothing of the YAML and TOML
  readers or of the mise and `go.mod` rules.
- Each removed claim is an acceptance requirement of plan T005, the
  task that builds `pr`, one bullet each:
  - (a) every input `pr` reads from the head is parsed by code under a
    path no checkpoint grant covers, its YAML and TOML readers included
    (R11-02);
  - (b) the code `pr` builds imports only the standard library and
    packages on `approvals`, held by a test, and `approvals` names
    those packages' directories (R11-01);
  - (c) `pr`, built from the default branch, applies the version-pin
    rules of `mise.toml` and `go.mod` to the head (R11-03);
  - (d) the mise toolchain and the `go` line are the remaining
    grantable inputs, each listed in 05 5.4.
- The index row on the `pr` job's toolchain says the same: those two
  inputs may change under a grant, the task lists them in 05 5.4, and
  every other input is held to the invariant by the task's acceptance;
  its trigger is unchanged.

The Advisory rows were applied as their status says. The parked items
P1, P3, P5 and P6 were not applied; T005 gained only the four
requirements above. Nothing under `tools/**`, `.github/**` or
`.githooks/**` changed.

One command line a row names was checked before it was written:
`git log` has no `--refs` option, so J7 step 4 uses `--glob`, which
matched a nested `refs/romeu/salvage/<generation>/<salvage-id>/...`
ref on a commit no branch holds, with git 2.53.0 in a scratch
repository on 2026-10-09.

## Session checkpoint (2026-10-09, end of session)

### Decided (2026-10-09)

- The open block of the round-8 checkpoint: every recommendation was
  accepted, with one change to G5. G5 option (a), the `approvals`
  surface, was widened in four steps, each after a must-fix security
  re-audit. The end state: `tools/ci pr` runs from a workflow defined
  in the default branch (`pull_request_target`, `branches: [main]`,
  `github.sha`, `contents: read`, no secret) and reads the head only as
  data. From round 11 on, the spec states this as an invariant, and the
  mechanism is acceptance requirements (a) to (d) of plan T005.
- PR #34 (DR3): option (a) and then S1 (the mise global config) were
  fixed. After four audit rounds, each of which found another
  caller-set mise variable (N1 `MISE_ENV_FILE`, N3 `MISE_CD`, N4
  `MISE_TRUSTED_CONFIG_PATHS`, plus `mise exec` trusting a parent
  `mise.toml`), the hook and every mise run of tools/ci were moved to an
  environment allowlist. That also closes N2 (`MISE_SYSTEM_CONFIG_FILE`).
- Rounds 10 and 11 ran.
- D2 to D6 of round 9 were all decided as recommended, and AR22 was
  closed as not needed (PR #33 measured the ruleset).
- D2, D4 and D6 (token narrowing, required checks without the admin
  bypass, direct-push refusal) were decided by delegation and recorded
  in [ADR 0011](../adr/0011-bind-the-administrator-to-the-required-checks-and-narrow-the-sandbox-s-github-token.md).
- The golangci-lint flake was fixed in PR #39 with
  `--allow-parallel-runners`, plus `-j 1` on request. The measured cost
  of `-j 1`: it does not fix the lock; a cold run takes about 14s
  against about 7s; it is kept to limit CPU use when runs overlap.
- Merges stay the maintainer's act. A pull request leaves draft when its
  gates are done and its hosted CI is green.

### Done

- PR #25 merged; ship gate rounds 1 to 3 plus five G5 security
  re-audits GO, CI green at 3f29a9c.
- PR #37 merged; ship gate GO, CI green at 3fda471.
- PR #36: round-9 write pass, ship gate GO in round 3 at dc76fec; then
  the decisions of 2026-10-09 and ADR 0010 (d458f83), the round-10 write
  pass (d133542), and the round-11 write pass plus ADR 0011 (6fd6e60),
  each re-audited by the next lens round. No ship gate has run yet on
  the commits after dc76fec.
- PR #39 at 439e406: ship gate GO in round 2.
- PR #34 at edda3e3: the allowlist redesign. Every repro (escape 2, S1,
  N1, N2, N3, N4) was blocked with a real push. A full ship gate and
  hosted CI were running at handoff, with no result recorded.

### Open for the maintainer

One block, each item with a recommendation:

- Merges, in the order #39, #34, #36, each once it is out of draft.
- Host and GitHub steps of ADR 0011, recommended in one sitting:
  - remove "Repository admin" from the bypass list of ruleset 24611273
    and save the API answer under Evidence;
  - create a fine-grained token for this repository only (Contents,
    Pull requests and Workflows read-write; Actions, Checks and Metadata
    read; no Administration) and set it with
    `sbx secret set github --sandbox <name>`;
  - stop forwarding the GitHub SSH auth key;
  - then re-run probe A16 and the four tries, the direct push included.
- A finding to know before that sitting: the current sandbox token is a
  classic token with scopes `gist, read:org, repo, workflow`, read on
  2026-10-09 from a response header, not on the host. It reaches every
  repository of the account, private ones included.

### Next steps

1. Round 12, the targeted re-audit of round 11 by L0, L2, L3, L6, L12,
   L13 and L14, on the head of PR #36. It was running at handoff;
   re-run it if its result is not recorded in a
   docs/reviews/round-12.md.
2. If round 12 raises no new Required finding, declare the spec
   validated for T004 onward in round-12.md, with the numbers.
   Otherwise, one more write pass and round.
3. A ship gate on PR #36's commits after dc76fec, then out of draft.
4. PR #34: finish the full ship gate on its head and get hosted CI green
   (the hosted mise action exports `MISE_TRUSTED_CONFIG_PATHS`,
   `MISE_YES` and `MISE_LOG_LEVEL`), then out of draft.
5. PR #39: hosted CI green, then out of draft. It overlaps PR #34 on
   tools/ci/fast.go, so whichever lands second rebases.
6. The R8-05-3 tooling PR (the `sbxdrv` surface and `skills/**` on
   `kits`), after #36 lands, because both edit the 05 5.3 fence.
7. The stage-B items in round-8/write-pass.md and the pending items of
   rounds 9 to 11.

### Pending items

- The mise `HOME`, `PATH` and Go cache pass-throughs of the allowlist (a
  poisoned module or build cache).
- Several agents sharing one machine run tools/ci at the same time, so
  any check that takes a machine-wide resource must be safe under that.
- tools/lenses: a script that runs a lens round (the briefs of rounds 9
  to 12 were generated by hand from lenses.md) and a well-formedness
  check of review reports.

Branch and PR state is not recorded here; the next session measures it
with `~/.sbx-kit/claude-home/bin/handoff_state.py`.
