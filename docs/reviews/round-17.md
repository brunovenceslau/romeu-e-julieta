# Spec review round 17: the targeted re-audit of round 16

Reader: reviewers checking that every finding of round 17 was handled,
and maintainers tracing why the spec reads as it does. Type: reference
(history). The current state is [docs/spec.md](../spec.md); round 16 is
in [round-16.md](round-16.md); the lenses are defined in
[lenses.md](lenses.md).

Round 17 is step 6 of "How stage A runs, and when the spec counts as
validated" in lenses.md: the targeted re-audit of the round-16 write
pass, which applied the round-16 rows and the decision of 2026-10-12 on
R16-03. lenses.md is cited at the baseline commit.

The baseline is commit 54f674c, the head of pull request #36 (the
round-9 to round-16 write passes, branch `docs/review-round-9`), read
from a detached clone that nothing wrote to while the reviewers ran.
The delta read is 3c32d62..54f674c, over the index, the pages under
`spec/`, `plan.md` and the note on the maintainer block.

This page records the round before its write pass: every row below is
a proposal, with the status **open** until a write pass applies it and
records A, M or L as in rounds 6 to 16. The round raised Required rows,
so the session that ran it stopped after recording it, as its brief
asked; the write pass is the next session's. Two rows would change text
that a decision fixed, or the list of accepted risks, so they are in
"Decisions for the operator" below.

The reports are kept in `round-17/reports/<lens>.json`, as the
reviewers wrote them.

## How the round was run

Four reviewers, each reading only what the pass changed in its scope:
L6 and L13 on the salvage text (04 4.1, 04 4.2, 04 4.4, 08 8.5, 01 1.6,
03 3.8, I17 and T068, with 09 J3 read against it); L0 and L2 on the
note on the maintainer block, 10 10.2 items g and h, the plan's
operator blocks (the opening and block O8) and the token rows of the
index. L0, L2 and L13 ran on `opus`, L6 on `sonnet`. Each lens ran
once, with one reader, in fresh contexts, read-only, in parallel. The
verdicts of the lenses that did not run carry forward: L3 from round
15, the others from round 13.

The brief was delivered in two parts, as in rounds 14 to 16: part one
as the first message, part two (the lens's bullets and "Where to
confirm", the reconcile, the self-critique questions and the report)
as a second message to the same reviewer after its part-one notes came
back; L0's part two held no bullets. The report fields are those of
round 16. No report is missing. L6 wrote one helper script next to its
report in the scratch directory, outside every tree; it was deleted
after the round, and only the four reports are kept.

The reports hold 31 findings: 0 Blocking, 3 Required, 28 Advisory; 18
new, 4 unresolved, 9 regression. The consolidation merged them by
section and defect, kept every lens on its row, and checked each
Required finding against the baseline text. The result is 3 Required
rows and 17 Advisory rows; nothing is downgraded, dropped or parked.

### Per lens

Severities in this table are the reviewers'.

| Lens | Model | prior_checked | prior_resolved_as_stated | Findings (B / R / A) | new | unresolved | regression | listed / own |
|---|---|---|---|---|---|---|---|---|
| L0 | opus | 6 | 5 | 8 (0 / 0 / 8) | 4 | 1 | 3 | 0 / 8 |
| L2 | opus | 5 | 4 | 7 (0 / 0 / 7) | 4 | 2 | 1 | 0 / 7 |
| L6 | sonnet | 3 | 2 | 7 (0 / 2 / 5) | 6 | 0 | 1 | 0 / 7 |
| L13 | opus | 6 | 2 | 9 (0 / 1 / 8) | 4 | 1 | 4 | 1 / 8 |
| all | | 20 | 13 | 31 (0 / 3 / 28) | 18 | 4 | 9 | 1 / 30 |

The round-16 Required rows read as resolved in the text: R16-01 removed
every "keeps changing", and R16-02 sends an incomplete salvage that a
changed fingerprint started to `RJ-306` and `open` on every page the
pass rewrote. What is left of them is wording (R17-04, R17-05) and two
I17 cases that do not reach their branch (R17-01, R17-02). The seven
round-16 findings not counted as resolved are L0-r16-3 (R16-07: the
pull request that holds decision 1's answer is not named, R17-18),
L2-r16-3 (R16-11: its limit is stated only in round-16.md, R17-16),
L6-r16-3 (R16-12: no golden holds the hint and T068 has no `RJ-338`
bullet, R17-06 and R17-08), and L13-r16-1, L13-r16-2, L13-r16-5 and
L13-r16-6 (R17-05, R17-06, R17-07, R17-04).

## Decisions for the operator

| Item | Question | Options | Recommendation |
|---|---|---|---|
| R17-06 | The hint of `RJ-338`, decided on 2026-10-12 and amended for R16-03, promises that a rerun resumes "if nothing still writes that path", where the decision says "when nothing is writing": a second path still written, or a write after the step-2 read the fingerprint was taken from, makes that rerun exit 5 again. Its `excludeIgnored` route does nothing for a tracked or untracked path. And "the rerun resumes" next to "the first rerun after either may exit 5 once more" reads as a contradiction after a stop. | (a) amend the fixed text: "If nothing writes in the sandbox any more, rerun; the first rerun may exit 5 once more, and the one after it resumes at `sbx env rm`. If something still writes, stop it, or, for an ignored path, add it to `salvage.excludeIgnored` (gate 2); then rerun, with the same result. `--accept-loss` does not cover this error."; (b) keep the text and record the two gaps as a stated limit in 04 4.4 | (a): it states what the rule gives, matches the I17 `excludeIgnored` case, and scopes the second recovery to the paths it can help; it stays one fixed text, so R15-02 stays closed |
| R17-11 | A first `rm` from `open` salvages, writes `removing` and runs `sbx env rm` with no comparison, so a write to an ignored path between the salvage read and the removal is lost without a word, while the same write after a failed `sbx env rm` is caught by `RJ-338`. No page says why only the rerun is guarded. | (a) one sentence in the recovery paragraph of 01 1.6: the fingerprint guards only a rerun on `removing`, since a first removal has no earlier read to compare with, and writes after the salvage read of a first removal are not salvaged; (b) (a) and a row in the accepted risks of 05 5.4 | (a): it writes the exception down where the mechanism is, changes no accepted risk, and the row of (b) can follow at the first report of such a loss |

## Findings and resolutions

One row per cluster. The Required rows come first, then the Advisory
rows. Kind is the cluster's: `unresolved` when any member is, else
`regression` when any member is, else `new`. Sections are cited as the
spec cites itself.

### Required

| Cluster | Section | Lenses | Kind | Defect | Verification | Proposed resolution | Status |
|---|---|---|---|---|---|---|---|
| **R17-01** | 05 5.2 (I17, "write during the salvage"); 08 8.5 (step 2) | L6 (L6-r17-1, Required) | new | The new case says only that "a writer changes a tracked file while the rerun salvages", but step 2 of 08 8.5 repeats the capture once before it skips a worktree, so a single write is absorbed, the salvage completes and the rerun ends as `RJ-338`; and the case does not say that the fingerprint already differs when the rerun starts, without which no salvage starts and the rerun resumes at `sbx env rm`. | holds as reported: 08 8.5 step 2 reads "if either moved, the capture repeats once, then the worktree is skipped as `worktree-changing`"; the I17 case added by R16-02 names neither the precondition nor a writer that outlasts the repeat. T068 writes the I17 tests | the case reads: the fingerprint already differs when the rerun starts, and the writer changes the tracked file between every pair of reads of HEAD and status, through the repeat, so the worktree is skipped as `worktree-changing`; the rerun exits 5 with `RJ-306` and the state is `open` (returning `RJ-338` turns it red). Cost: one clause. Approval line `spec` | open |
| **R17-02** | 05 5.2 (I17); 04 4.4 (`RJ-338`); 08 8.5 | L6 (L6-r17-2, Required) | new | No case runs the branch of `RJ-338` where `--accept-loss` names the skipped reasons of the rerun's salvage; the one mention, "`--accept-loss` naming any reason does not change that", sits in a case whose salvage is complete, so no reason is skipped and the flag does nothing. A build that honours the flag and runs `sbx env rm`, a destructive path, stays green. | holds as reported: 04 4.4 and 08 8.5 give that branch (`removing`, exit 5, `RJ-338`, no `sbx env rm`); the only I17 text on it is in the complete-salvage case. T068 writes the I17 tests | a case: a writer keeps a tracked file changing through the repeat, and the rerun runs with `--accept-loss=worktree-changing`; it exits 5 with `RJ-338`, the state is `removing`, the new entry records the accepted reason, and `sbx env rm` is not reached (reaching it, or returning `RJ-306`, turns it red). In the complete-salvage case, "naming any reason" becomes "naming a reason the salvage skipped". Cost: one case, one phrase. Approval line `spec` | open |
| **R17-03** | 01 1.6 (the `removing` row); 05 5.2 (I17, the U case and the transition case) | L13 (L13-r17-1, Required) | regression | The table has no row from `removing` to `salvaging`: that transition, and the new branch to `open` of an incomplete salvage, sit only in the effect cell of a row whose To is `removing`, while I17 tests "every row, every illegal pair" of the table and names `removing -> salvaging` as a transition; the row's "exit 1" also covers both of its events, and a command that finds the sandbox present does not exit 1. | holds as reported: 01 1.6's `removing` row reads "`sbx env rm` fails, or the sandbox found present ... \| removing \| ... exit 1; ... otherwise `removing -> salvaging` with a new salvage id"; I17's U case reads "generation state-machine table (every row, every illegal pair)". The shape predates the round-16 pass, which rewrote that cell. T034 builds the machine from the table | split the row: `removing` \| `sbx env rm` fails \| `removing` \| exit 1; `removing` \| the sandbox found present, a fresh fingerprint equal to the newest written with `removing` \| `removing` \| resumes at `sbx env rm`; `removing` \| the sandbox found present, the fingerprint differs \| `salvaging` \| new salvage id; the `salvaging` rows decide the end (`RJ-338` when complete or accepted, `RJ-306` and `open` otherwise). Cost: two rows. Approval line `spec` | open |

### Advisory

Advisory findings are applied by default (lenses.md, step 4). The
Defect cell keeps the reviewer's first sentence in short; the report
holds the rest. The platform behavior R17-13 and R17-14 rest on (which
`github` secret a re-created sandbox takes, whether sbx forwards the
caller's socket, whether a deploy key appears on the account's SSH keys
page) is the reviewers' inference, not read from a primary source this
round.

| Finding | Section | Lenses | Kind | Defect | Proposed resolution | Status |
|---|---|---|---|---|---|---|
| **R17-04** | 09 J3 (step 5); 04 4.2 (the `rm` row) | L13 (L13-r17-3), L6 (L6-r17-5) (high-signal) | unresolved | J3 step 5 compares against "the one written with `removing`", without "newest", and, like the `rm` row of 04 4.2, says a changed fingerprint exits 5 with `fingerprint-unstable` with no branch for an incomplete salvage; the round-16 pass left both. | J3: "equals the newest one written with `removing`" and "otherwise salvages again and ends as `RJ-338` says"; the `rm` row cites 04 4.4 for how the salvage ends. Approval line `spec` | open |
| **R17-05** | 04 4.1; 04 4.2 (the `rm` row); 08 8.5 | L13 (L13-r17-4) | regression | R16-01 wrote three phrasings next to the one name R16-13 chose, and 08 8.5's "worktrees changed" leaves out the daemon heads and ignored paths the fingerprint covers. | each says "a fingerprint that differs from the newest one written with `removing`". Approval line `spec` | open |
| **R17-06** | 04 4.4 (the `RJ-338` hint); 03 3.8 | L13 (L13-r17-5), L6 (L6-r17-6) (high-signal) | regression | The applied hint narrows the decision and reads as a contradiction after a stop; no golden holds it. | the item in "Decisions for the operator"; and T008's acceptance holds the hint in a golden, or 04 4.4 says no test asserts it | open |
| **R17-07** | plan T068 (Acceptance) | L13 (L13-r17-2) | regression | The first bullet cites "the first `E:` case of I17" by its place, under a bullet that says the cases are cited by title. | the first `E:` case of I17 gets a title, "rm refuses", and T068 cites it by that title. Approval line `spec` | open |
| **R17-08** | 05 5.2 (I17); plan T068, T069; 04 4.4 | L6 (L6-r17-3) | new | `RJ-338` names `rm`, `recreate` and `retire`, but every fingerprint case is written for `rm`, and neither T068 nor T069 names `RJ-338`. | I17 says the fingerprint cases run for the three commands; T068 gains a bullet for `RJ-338` and `RJ-306` on a `removing` rerun, T069 one for `retire` | open |
| **R17-09** | 05 5.2 (I17, "kill the salvage"); 04 4.4 (the restart sentence); 01 1.6 (the `salvaging` restart row) | L6 (L6-r17-4), L13 (L13-r17-9) (high-signal) | new | The case names no kill point, and "interrupted" names two outcomes: an interrupted exec goes to `open`, while only a killed romeu leaves `salvaging`. | the case kills after `removing -> salvaging` is written and before `removing` is; RJ-338 and the restart row say "killed"; 01 1.6 says the distinction is the state `salvaging`, with no stored marker. Approval line `spec` | open |
| **R17-10** | 03 3.8 (`salvage[].fingerprint`); 04 4.4 | L6 (L6-r17-7), L13 (L13-r17-8) (high-signal) | new | The rewritten 03 3.8 sentence dangles ("ends as `RJ-338` ... says, which names"); "the first path whose entry changed" and "newest" have no defined order; 04 4.4 says twice that `--accept-loss` does not cover the error. | "... and ends as `RJ-338` says, whose details name the first path, in byte order, whose entry changed"; newest is the entry with the greatest salvage id; the second sentence in 04 4.4 goes. Approval line `spec` | open |
| **R17-11** | 01 1.6; 04 4.4; 08 8.5 (step 2) | L13 (L13-r17-6) | new | A first removal is not guarded against writes after its salvage read, a rerun is, and no page says why. | the item in "Decisions for the operator" | open |
| **R17-12** | 05 5.2 (I17, the `excludeIgnored` case); 04 4.2 (`sync`) | L13 (L13-r17-7) | new | The `excludeIgnored` recovery runs through `sync`, which promotes the edit and then exits 2 with `removal-pending`, which neither the hint nor the case says. | the case reads "after `romeu sync` promotes an edit of `salvage.excludeIgnored` (gate 2), which then exits 2 with `removal-pending`"; whether the hint says so is part of R17-06. Approval line `spec` | open |
| **R17-13** | the note on the maintainer block (the cadence); plan (O3, O6, O7, O8); 05 5.4 (the token row) | L2 (L2-r17-1), L0 (L0-r17-1) (high-signal) | unresolved | Later reads are due at sittings, not at the events that change the state (a re-created sandbox, `sbx secret set` again, a replaced token, the maintainer's host pushes), none is due after O7, no plan row carries them, and a read's merge try is not ordered against the candidate and the tag. | the note adds those events and a read at each release-candidate tag after O7; the read runs after the maintainer's pushes, before the candidate tag at O6 and after the `v1.0.0` tag at O7, with empty commits as payloads; O3, O6 and O7 name it; O8 names the sandbox-scoped secret and that no global `github` secret holds the operator's token. Approval line `spec` | open |
| **R17-14** | the note on the maintainer block (read 2, the verdict); 06 6.4; ADR 0011 (decision 2) | L2 (L2-r17-2), L0 (L0-r17-2) (high-signal) | regression | Read 2 expects one socket that holds the signing key, so forwarding nothing, which ADR 0011 allows, reads not narrowed, while a socket that holds the signing key and another key passes when that key is not on the account's page. | expected: every forwarded socket holds the signing key alone, as 06 6.4 states for romeu's sandboxes, or no socket is forwarded; each forwarded fingerprint is absent from the account's authentication keys. No ADR change. Approval line `spec` | open |
| **R17-15** | the note on the maintainer block (the entry rule); index (rows 279, 283, 348, 351, 353); 05 5.4 (the token row) | L2 (L2-r17-3, L2-r17-4), L0 (L0-r17-3, L0-r17-5) (high-signal) | regression | A later not narrowed entry widens the blast radius back with no decision asked; it "returns" two rows to a state they never left, since nothing writes their narrowed state; the trigger cells do not name it, a decided row has left the table, and a re-narrowing is no event. | the first narrowed entry, and a narrowed entry after a not narrowed one, edit the 05 5.4 token row and the index row to say the narrowing holds, with the date; a not narrowed entry after a narrowed one is an item for the operator at that sitting (re-narrow before it ends, or accept with a date and an ending event); the four trigger cells name "a later entry whose verdict differs from the one before it", and a row that left the table returns with its last decision. The accepted risk of the token row is unchanged. Approval line `spec` | open |
| **R17-16** | the note on the maintainer block (the limits); plan (O8, Recommendation) | L2 (L2-r17-5) | unresolved | R16-11's stated limit lives only in round-16.md; the note still lists two limits. | the note lists three: how the forwarding stops is the maintainer's choice, ADR 0011, decision 2, deciding only that it stops; an entry whose read 2 finds an authentication key or a second key reopens it. Approval line `spec` | open |
| **R17-17** | plan (Operator blocks, opening) | L2 (L2-r17-6), L0 (L0-r17-4) (high-signal) | regression | "Which the sandbox holds until block O8" ties the end of the token to the block and reads as if the tag ruleset binds the sandbox after it, which nothing measured. | "which the sandbox holds until the note's first narrowed entry; whether the tag ruleset binds the narrowed token is measured only by an entry made before `release.yml` lands" | open |
| **R17-18** | plan (O8: order, Pass, Unblocks, Also needs); 10 10.2 (item a); the note (read 3, the merge try); index (S2) | L0 (L0-r17-6), L2 (L2-r17-7) (high-signal) | unresolved | No pull request is named for decision 1's answer, O8's Unblocks omits S2, O8 sets no order between decision 1 and the entry, and the merge try, expected accepted once checks are green, cannot show a bypass. | O8 names the pull request that rewrites item a's "until that step" with the date and holds the answer under its Evidence (as ADR 0011, decision 1, words the home), adds S2 to Unblocks, and orders decision 1 before the entry; read 3's merge try is made once before the checks are green, expected refused | open |
| **R17-19** | 10 10.2 (item h); the note (the fourth part) | L0 (L0-r17-7) | new | Item h may run after a later not narrowed entry, and its answers go under the first narrowed entry's older date with no date of their own. | "run only while the newest entry's verdict is narrowed"; the fourth part opens with the date it was run. Approval line `spec` | open |
| **R17-20** | index (row 348); 05 5.4 (the token row); 10 10.2 (item g); plan (O8) | L0 (L0-r17-8) | new | Leftover seams: row 348 and the 05 5.4 token row point at "the entry of 10 10.2, item g" and say "at the next maintainer sitting"; item g speaks of the rows ADR 0011, decision 2, fires, which names two of the four; O8 asks for one throwaway pull request where two are used. | row 348 links the note's anchor and says "at block O8"; the 05 5.4 row is left for R17-15's edit, since it is an accepted risk; item g says "the two rows ADR 0011, decision 2, names and the two others with the same trigger"; O8 reads "two throwaway pull requests". Approval line `spec` | open |

## Exit criteria of stage A

The five criteria of lenses.md, judged on 54f674c:

1. **No Blocking open; every Required fixed or declined with its
   decision and date: not met.** 0 Blocking. 3 Required rows are open
   (R17-01 and R17-02, raised by L6, and R17-03, raised by L13), not
   fixed and not declined. R16-01 and R16-02 read as resolved in the
   text in every report that re-read them.
2. **Coverage: carried from round 8, as in rounds 9 to 16**
   ([round-8.md](round-8.md#coverage)).
3. **The open questions and deferred decisions of the index have a
   valid shape: met for 101 of 101 rows as measured.** 24 open
   questions and 77 deferred decisions (36, 11 and 30), the same count
   as round 16; every cell filled and no first cell repeated, measured
   by a script over the tables at 54f674c. R17-15 finds that a second
   event of four rows is defined only in the note; the rows keep a
   trigger, so their shape holds.
4. **The three measurements: carried from round 8**
   ([round-8.md](round-8.md#measurements)). This round ran no noise
   probe; its reports record an origin per finding (1 listed, 30 own).
   L0 raised eight findings; R17-19 and R17-20 are L0's alone, on the
   seams between item h, the note and the token rows. The lens
   critiques add input for the lens rewrite before stage B: L2 names an
   owner of a credential's life (where it is stored, when it expires,
   what a re-created sandbox inherits), and L6 a reader who checks which
   test covers each state-machine row for each command that shares it,
   since a test can be red for the wrong reason or never reach its
   branch.
5. **The targeted re-audit raises no new Required finding: not met.**
   It raised 3 Required rows: R17-01 and R17-02, of kind `new`, in the
   I17 text the round-16 pass wrote, and R17-03, of kind `regression`,
   in a table shape that predates the pass, in the cell it rewrote.

Validated for T004 onward: no. 0 Blocking; 3 Required open (R17-01 and
R17-02, raised by L6, and R17-03, raised by L13), 0 declined; coverage
and the three measurements carried from round 8; 101 of 101 index rows
with a valid shape as measured; the re-audit raised 3 Required rows.

What is left before T004:

- the answers to the two items in "Decisions for the operator";
- the write pass of this page's rows, with the `spec` approval line;
- a targeted re-audit of that pass, round 18, by L6 and L13 on the
  salvage text and L0 and L2 on the maintainer block, reading only the
  sections it changes.

The `.githooks/pre-push` row of 12 12.1 is edited by a separate pull
request, `fix/pre-push-toolchain-hint`; pull request #36 does not edit
that row, and no write pass of this review touches it.
