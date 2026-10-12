# Spec review round 15: the targeted re-audit of round 14

Reader: reviewers checking that every finding of round 15 was handled,
and maintainers tracing why the spec reads as it does. Type: reference
(history). The current state is [docs/spec.md](../spec.md); round 14 is
in [round-14.md](round-14.md); the lenses are defined in
[lenses.md](lenses.md).

Round 15 is step 6 of "How stage A runs, and when the spec counts as
validated" in lenses.md: the targeted re-audit of the round-14 write
pass. Each lens read the sections that pass changed, checked the
round-14 findings it had raised against the row that resolved them and
the status the pass applied, and reported what was left, what regressed
and what the new text added. lenses.md is cited at the baseline commit.

The baseline is commit 85f6860, the head of pull request #36 (the
round-9 to round-14 write passes, branch `docs/review-round-9`), read
from a detached clone that nothing wrote to while the reviewers ran.
The delta read is 19863f5..85f6860, over the index, the pages under
`spec/`, `plan.md` and the note on the maintainer block.

No row changes a decision record, an accepted risk or the scope. One
decision was taken for R15-01 and R15-02, recorded under the Required
table; it replaces a rule of round 12, not a decision record.

Status, as in rounds 6 to 14: **A** = applied as proposed; **M** =
applied in a smaller or different form than proposed, as the write
pass states; **L** = stated as a limit, no mechanism added. The round
was recorded before its write pass, with each resolution a proposal;
each row's status is the one the pass applied, in "Write pass
(2026-10-12)" below.

The reports are kept in `round-15/reports/<lens>.json`, as the
reviewers wrote them.

## How the round was run

Five reviewers, one per lens that raised a round-14 finding: L0, L2,
L3, L6 and L13, each in a fresh context, read-only, in parallel on
85f6860. L0, L2 and L13 ran on `opus`, L3 and L6 on `sonnet`; L0 ran
on the strongest tier, as lenses.md asks, for the first time in a
re-audit. Each lens ran once, with one reader; a re-audit carries no
noise probe. The verdicts of L1, L4, L5, L7 to L12 and L14 to L16
carry forward from round 13.

The brief was delivered in two parts, as in round 14: part one as the
first message, part two (the lens's bullets and "Where to confirm",
the reconcile, the self-critique questions and the report) as a second
message to the same reviewer after its part-one notes came back; L0's
part two held no bullets. The report fields are those of round 14. No
report is missing. L2 wrote one scratch file of notes outside every
tree during part one and deleted it before it reported.

The reports hold 29 findings: 0 Blocking, 2 Required, 27 Advisory; 24
new, 4 unresolved, 1 regression. The consolidation merged them by
section and defect, kept every lens on its row, and checked each
Required finding against the baseline text. The result is 2 Required
rows and 15 Advisory rows; nothing is downgraded, dropped or parked.

### Per lens

Severities in this table are the reviewers'.

| Lens | Model | prior_checked | prior_resolved_as_stated | Findings (B / R / A) | new | unresolved | regression | listed / own |
|---|---|---|---|---|---|---|---|---|
| L0 | opus | 2 | 2 | 6 (0 / 0 / 6) | 6 | 0 | 0 | 0 / 6 |
| L2 | opus | 3 | 2 | 5 (0 / 0 / 5) | 4 | 1 | 0 | 1 / 4 |
| L3 | sonnet | 5 | 5 | 5 (0 / 0 / 5) | 2 | 2 | 1 | 0 / 5 |
| L6 | sonnet | 5 | 2 | 6 (0 / 1 / 5) | 6 | 0 | 0 | 0 / 6 |
| L13 | opus | 4 | 3 | 7 (0 / 1 / 6) | 6 | 1 | 0 | 3 / 4 |
| all | | 19 | 14 | 29 (0 / 2 / 27) | 24 | 4 | 1 | 4 / 25 |

The Required row of round 14, R14-01, reads as resolved in every
report that re-read it (L3's, L6's and L13's). The five round-14
findings not counted as resolved are L2-r14-1 (R14-02: the key check
is read in the sandbox, R15-04 below), L6-r14-2 and L13-r14-2 (R14-07:
the `excludeIgnored` and standalone-salvage cases, R15-12 and R15-13),
and L6-r14-4 and L6-r14-5 (R14-11, R14-12: resolved in part, R15-16 and
R15-17). L3 counts its five round-14 findings resolved as stated and
still kinds two of its findings `unresolved` (L3-r15-4 and L3-r15-5),
for residue the M or A form left.

## Findings and resolutions

One row per cluster. The Required rows come first, then the Advisory
rows. Kind is the cluster's: `unresolved` when any member is, else
`regression` when any member is, else `new`. Sections are cited as the
spec cites itself.

### Required

| Cluster | Section | Lenses | Kind | Defect | Verification | Resolution | Status |
|---|---|---|---|---|---|---|---|
| **R15-01** | 03 3.8 (`salvage[].cause`); 04 4.4 (`RJ-338`); 01 1.6 (the `salvaging` rows); 05 5.2 (I17) | L6 (L6-r15-1, Required), L13 (L13-r15-5, Advisory), L0 (L0-r15-6, Advisory) (high-signal: three lenses) | new | The row of `RJ-338` is defined twice, and one path through it is unstated. The R14-09 write keeps 03 3.8's own sentence, "the salvages in a row that a changed fingerprint started are the newest entries of the generation with `fingerprint-changed`", next to 04 4.4's new rule (entries newer than the newest `requested` one), and the I17 case R14-07 added says counting every `fingerprint-changed` entry is wrong. And a changed-fingerprint salvage interrupted before it ends leaves the generation `salvaging`; the next `rm` restarts it from `salvaging`, which the new closed set of 03 3.8 writes as `requested`, so an interruption restarts the count, and no text says that is the rule and no I17 case asserts the cause. | holds as reported: 03 3.8's `cause` cell lists `requested` for `rm`, `recreate` or `retire` "from `open` or `salvaging`" and still holds the older row sentence; 01 1.6 sends an interrupted salvage `salvaging -> salvaging` with a new salvage id; the fault-injection case of I17 asserts convergence, not the cause. Severity as L6 graded it: T008 builds the error table from 04 4.4, and the salvage tasks build the rerun from 03 3.8 and I17 | 03 3.8 drops its own row sentence and cites `RJ-338` for the row; 04 4.4 says, as a rule, that a salvage restarted from `salvaging` writes `requested` and so starts a new row (the cap guards liveness, and an interruption is the operator's own act); I17 gains the case: a changed-fingerprint salvage killed before it ends, then a rerun, which records `cause: requested` and does not exit 5 even after three earlier changed-fingerprint salvages (the removal that turns it red: carrying the cause of the interrupted salvage). Cost: one sentence in 03 3.8, one in 04 4.4, one E case. Approval line `spec` | M |
| **R15-02** | 04 4.4 (the `RJ-338` hint) | L13 (L13-r15-6, Required), L6 (L6-r15-6, Advisory) (high-signal) | new | The R14-01 rewrite makes the hint name the two recoveries only "when a process is still writing that path", a fact romeu cannot observe, so the text of the hint depends on something the product does not know. | holds as reported: 04 4.4 reads "the hint says that a rerun with nothing written since this salvage resumes at `sbx env rm`, and, when a process is still writing that path, names the two recoveries". T008 builds the error table and its hints from 04 4.4 | the hint becomes one fixed text: a rerun with nothing written since this salvage resumes at `sbx env rm`; if a process still writes that path, stop it or add an ignored path to `salvage.excludeIgnored` (gate 2), then rerun; the first rerun after either may salvage and exit 5 once more. Cost: one clause. Approval line `spec` | M |

Decided on 2026-10-12: `RJ-338` loses its count. Every salvage that a
changed fingerprint starts on a `removing` generation runs, records its
entry and `fingerprint`, and then exits 5 with `fingerprint-unstable`
instead of running `sbx env rm`; a rerun whose fresh fingerprint equals
the one written resumes at `sbx env rm`. The hint is one fixed text:
the first path whose entry changed; if something still writes it, stop
it or add an ignored path to `salvage.excludeIgnored` (gate 2), then
rerun; `--accept-loss` does not cover this error. This supersedes the
limit of three salvages in a row of round 12 (R12-01) and rounds 13 and
14 (R13-01, R14-01), and it closes R15-01 and R15-02 in place of their
proposed resolutions; the cost is one more rerun after a single benign
change.

### Advisory

Advisory findings are applied by default (lenses.md, step 4). The
Defect cell keeps the reviewer's first sentence in short; the report
holds the rest. The platform behavior R15-04, R15-07, R15-08 and R15-09
rest on is from the reviewers' reading, not measured, as the reports
say.

| Finding | Section | Lenses | Kind | Defect | Resolution | Status |
|---|---|---|---|---|---|---|
| **R15-03** | 10 10.2 (item g); the note on the maintainer block (The token narrowing); index (the four rows whose trigger is "the token is narrowed") | L0 (L0-r15-1), L2 (L2-r15-2), L3 (L3-r15-1), L6 (L6-r15-3), L13 (L13-r15-1) (high-signal: five lenses) | new | The verdict "narrowed" has two homes that disagree: the note requires the exact expected A16 read, while item g fails the token only on more than this repository, Administration, a GitHub key or a ruleset update not refused, so a token with an extra permission passes one and fails the other. Both call any entry "the event", one that records "not narrowed" included; the entry has no fixed format, nothing says which entry decides or what a later "not narrowed" entry does, and the "No entry yet" line is never removed. | the note holds the rule alone and item g cites it; an entry is a heading `### <date>: narrowed` or `### <date>: not narrowed`, and the event is the first entry whose verdict is narrowed; a later "not narrowed" entry returns the token rows of the index and 05 5.4 to the state before the narrowing, and the rows already fired stay fired; the first entry replaces the "No entry yet" line. Approval line `spec` | A |
| **R15-04** | the note on the maintainer block (The token narrowing, read 2); 10 10.2 (item g) | L2 (L2-r15-1), L0 (L0-r15-2), L13 (L13-r15-2) (high-signal: three lenses) | unresolved | The check that the GitHub authentication key is no longer forwarded is `ssh-add -l` run in the sandbox, which the note's own rule ("never only from what the sandbox printed") excludes, and not the host-side check R14-02's resolution promised; a sandbox shell can shape that output. | read 2 becomes a host read: the forwarding configuration the host gives the sandbox (the sbx setting or kit that forwards the agent) and the GitHub account's SSH keys page, each against a known list; the sandbox's `ssh-add -l` may be kept as a second read, not as the proof. Approval line `spec` | A |
| **R15-05** | 10 10.2 (maintainer block: the opening rule on Evidence, and the "To do" lead-in) | L3 (L3-r15-2), L0 (L0-r15-4), L13 (L13-r15-2) (high-signal: three lenses) | regression | The block's opening rule and the "To do" lead-in still say each item's answer goes under a pull request's Evidence, while items g and h save theirs in the note's entry. | both sentences name the exception: items g and h save in the dated entry of the note, since the narrowing has no pull request. Approval line `spec` | A |
| **R15-06** | plan (the maintainer blocks O1 to O7); 10 10.2 (items g and h) | L0 (L0-r15-5), L3 (L3-r15-3) (high-signal) | new | No maintainer block of the plan carries items g and h, so the narrowing decided on 2026-10-09 can reach v1.0.0 undone by omission. | an O row for items g and h, with what proceeds around it, before checkpoint C7; it is a row the plan keeps by hand, so it is applied with this pass and not parked for B | A |
| **R15-07** | the note on the maintainer block (The token narrowing, read 3, the tag try); index (the token-narrowing row) | L0 (L0-r15-3), L2 (L2-r15-4) (high-signal) | new | The tag try is repeated "for each later read", and `v0.0.0-try` has the prerelease form `release.yml` releases, so a read after `release.yml` lands could publish a stray prerelease (inferred); and the answer is "recorded either way" with nothing acting on it. | the tag try runs only before `release.yml` lands, and later reads drop it; an accepted answer adds a maintainer step to remove the administrator role's bypass of the tag ruleset, or is recorded against the deferred row on `release.yml`. Approval line `spec` | L |
| **R15-08** | 10 10.2 (item h, step 3) | L2 (L2-r15-3) | new | "so no head that carries a status under a required name can be reopened and merged" claims more than the steps give: the commit stays reachable through `refs/pull/<n>/head` and statuses attach to the SHA (inferred). | the sentence names the real guard: the head is one empty commit, so a merge of it changes nothing. Approval line `spec` | A |
| **R15-09** | the note on the maintainer block (The token narrowing); index (the token-narrowing row, after the narrowing) | L2 (L2-r15-5) | new | The after-state reads "limited to this repository" while nothing in the entry looks, from the host, for deploy keys, webhooks or collaborators the old token could have planted, a class ADR 0008 lists; the class stays an accepted risk, and the defect is that the after-state ignores it. | the entry adds a host read of this repository's deploy keys, webhooks and collaborators, each against a known list; the class's acceptance is not changed. Approval line `spec` | L |
| **R15-10** | 05 5.4 (the token row); index (the token-narrowing row) | L13 (L13-r15-7) | new | Both say the item g entry "measures" the narrowed token's full list of capabilities, but the entry reads the permissions and four tries, not branch deletion, pull request body edits or workflow pushes. | "measures" becomes "reads the permissions behind"; the list stays derived from ADR 0011, decision 2. Approval line `spec` | A |
| **R15-11** | 01 1.6 (the recovery paragraph); 09 J3 (step 5); 03 3.8 (`salvage[].fingerprint`) | L13 (L13-r15-3) | new | "up to the limit of `fingerprint-unstable`" (01 1.6, J3) and "the rule that ends such salvages in a row" (03 3.8) say salvages stop, while 04 4.4 says each later salvage runs and only the resume in that command is withheld; the cap now has three names. | each says "each such salvage exits 5 once the row reaches the count of `fingerprint-unstable`", the phrase 01 1.6's table and 08 8.5 use. Approval line `spec` | M |
| **R15-12** | 05 5.2 (I17, the `excludeIgnored` case) | L13 (L13-r15-4) | unresolved | The case stops the writer before the edit, so it cannot tell the edit from the stop, never tests the exclusion as a recovery on its own, and names no removal. | the writer keeps writing the one ignored path; the edit excludes it; the next rerun salvages and exits 5 once, the one after resumes; the removal is leaving `excludeIgnored` out of the fingerprint. Approval line `spec` | M |
| **R15-13** | 05 5.2 (I17, the standalone-salvage case) | L6 (L6-r15-2) | new | The later `rm` from `open` writes its own `requested` entry, so the case stays green when a standalone salvage does not end the row. | the case asserts the count right after the standalone salvage, from the state, before any `rm`. Approval line `spec` | M |
| **R15-14** | 03 3.8 (`salvage[].cause`) | L3 (L3-r15-4) | unresolved | A lost record from crash recovery carries `requested`, which reads as a user action, and the cell does not say a new value needs a format change. | the cell says `requested` means written by a command's own run, not asked for by a person, and that a new value is a change of the state format under its version rule. Approval line `spec` | M |
| **R15-15** | 05 5.2 (I17); plan (the task that writes the I17 tests) | L3 (L3-r15-5) | unresolved | Splitting the I17 cell into titled cases is recorded only in round-14.md, so the task that writes the tests does not see it. | the task's acceptance names the split | A |
| **R15-16** | plan T005 (b) | L6 (L6-r15-4) | new | The build-constraint fixture covers GOOS only; a GOARCH-only constraint is not shown, and listing two GOOS on one GOARCH would pass. | a second fixture whose outside import builds only for arm64, with its removal (listing one GOARCH) | L |
| **R15-17** | plan T005 (e) | L6 (L6-r15-5) | new | "a second reader inlined in either turns it red" names no mechanism that can see an inlined reader. | the test scans the two packages' imports and calls for the YAML decoder and fails on a decode of either file outside the one reader | L |

## Exit criteria of stage A

The five criteria of lenses.md, judged on 85f6860:

1. **No Blocking open; every Required fixed or declined with its
   decision and date: not met.** 0 Blocking. 2 Required rows are open
   (R15-01, R15-02), new, not fixed and not declined. R14-01 reads as
   resolved in every report that re-read it.
2. **Coverage: carried from round 8, as in rounds 9 to 14**
   ([round-8.md](round-8.md#coverage)).
3. **The open questions and deferred decisions of the index have a
   valid shape: met for 101 of 101 rows as measured.** 24 open
   questions and 77 deferred decisions (36, 11 and 30), the same count
   as round 14; every cell filled and no first cell repeated, measured
   by a script over the tables at 85f6860. R15-03 finds the event four
   rows cite is defined in two places; the rows keep a trigger, so
   their shape holds.
4. **The three measurements: carried from round 8**
   ([round-8.md](round-8.md#measurements)). This round ran no noise
   probe; its reports record an origin per finding (4 listed, 25 own).
   L0 raised six findings; each joined a row another lens raised, so no
   finding is L0's alone. The lens critiques add input for the lens
   rewrite before stage B: L0 names an operator viewpoint on the
   maintainer's own steps, L2 an incident responder after a compromise
   and asks that the observable that flips a security state be read
   from outside the boundary it claims to close, L6 that a case reach
   the branch it names, and L13 that a cite point rather than restate.
5. **The targeted re-audit raises no new Required finding: not met.**
   It raised 2 Required rows, both of kind `new`, both in text the
   round-14 pass wrote: R15-01 in 03 3.8 (the R14-09 write) and R15-02
   in 04 4.4 (the R14-01 write).

Validated for T004 onward: no. 0 Blocking; 2 Required open (R15-01,
raised by three lenses, and R15-02, raised by two), 0 declined;
coverage and the three measurements carried from round 8; 101 of 101
index rows with a valid shape as measured; the re-audit raised 2
Required rows.

What is left before T004:

- the write pass of this page's rows, with the `spec` approval line:
  done, in "Write pass (2026-10-12)" below, with the decision recorded
  under the Required table;
- a targeted re-audit of that pass, round 16, by the lenses whose
  findings drive it, reading only the sections it changes.

## Write pass (2026-10-12)

The pass ran on branch `docs/review-round-9`, the branch of pull
request #36, at be402ca, in its worktree. Each row's status above is
the one applied.

Under the decision of 2026-10-12:

- R15-01 (M): the count is gone, so neither the row's second
  definition nor the interrupted salvage has anything to restart.
  `RJ-338` in 04 4.4 is rewritten and stays the one home of the rule;
  01 1.6 (the `removing` row and the recovery paragraph), 03 3.8 (the
  `salvage[].fingerprint` cell), 08 8.5, 09 J3 and I17 cite it.
  `salvage[].cause` is removed from 03 3.8 and its example: the count
  was its one reader, measured by a search of the index, the pages
  under `spec/`, `plan.md` and the decision records.
- R15-02 (M): the hint is the one fixed text of the decision.
- R15-11 (M): 01 1.6, 09 J3, 03 3.8 and 08 8.5 now say each such
  salvage exits 5, which is the rule.
- R15-12 (M): the `excludeIgnored` case keeps its writer running: the
  rerun after the edit salvages and exits 5, the one after resumes,
  and leaving `salvage.excludeIgnored` out of the fingerprint turns it
  red.
- R15-13 and R15-14 (M): moot. The standalone-salvage case and the
  `cause` field went with the count.
- I17 loses the four cases that tested only the count (the writer on
  every rerun, the three-salvage reset, the two `requested` cases),
  rewrites the `excludeIgnored` case (R15-12) and gains "after a failed `sbx env rm`, a changed fingerprint makes the
  rerun salvage, record its entry and exit 5; the next rerun with an
  equal fingerprint resumes" (running `sbx env rm` after that salvage
  turns it red).

The maintainer block and the token rows:

- R15-03 (A): the note on the maintainer block is the one home of the
  reads, the expected answers and the verdict; item g of 10 10.2 cites
  it; an entry is `### <date>: narrowed` or `### <date>: not
  narrowed`; the first narrowed entry is the event; a later not
  narrowed entry returns the token rows to the earlier state, and the
  rows it fired stay fired.
- R15-04 (A): read 2 is made on the host, on the socket the host
  forwards; the sandbox's own `ssh-add -l` is a second read, not the
  proof.
- R15-05 (A): the block's opening rule and the "To do" lead-in name
  items g and h as the exception.
- R15-06 (A): the plan gains block O8, items g and h, after T002 and
  before checkpoint C7, with no task waiting on it; the plan now has
  eight blocks.
- R15-08 (A): item h names the empty head commit as the guard.
- R15-10 (A): "measures" becomes "reads" in 05 5.4 and the index.
- R15-15 (A): T068's acceptance makes each `E:` case of I17 a test
  with a title of its own.

Stated limits, each with what reopens it:

- R15-07 (L): the guard is kept, in the note: the tag try runs only
  while no `release.yml` exists. Nothing acts on an accepted tag try;
  the pull request that adds `release.yml` reopens it. The note states
  this limit.
- R15-09 (L): no read of the entry looks for deploy keys, webhooks or
  collaborators the operator's token could have added before the
  narrowing; the review of the token narrowing row of the index
  reopens it. The note states this limit.
- R15-16 (L): a GOARCH-only build constraint in T005 (b); T005's own
  review reopens it.
- R15-17 (L): a mechanism that sees an inlined second reader in T005
  (e); T005, when it takes one reader, reopens it.

The date of the first four tries, 2026-10-07 in the note and ADR 0008
and 2026-10-08 in 10 10.2, 05 5.4 and ADR 0011, is left as round 14
recorded it. No decision record is edited, and no accepted risk of 05
5.4 changes. Nothing under `tools/**`, `.github/**` or `.githooks/**`
changed.
