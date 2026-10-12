# Spec review round 14: the targeted re-audit of round 13

Reader: reviewers checking that every finding of round 14 was handled,
and maintainers tracing why the spec reads as it does. Type: reference
(history). The current state is [docs/spec.md](../spec.md); round 13 is
in [round-13.md](round-13.md); the lenses are defined in
[lenses.md](lenses.md).

Round 14 is step 6 of "How stage A runs, and when the spec counts as
validated" in lenses.md: the targeted re-audit of the round-13 write
pass. Each lens read the sections that pass changed, checked the
round-13 findings it had raised against the row that resolved them,
and reported what was left, what regressed and what the new text
added. lenses.md is cited at the baseline commit.

The baseline is commit c64c468, the head of pull request #36 (the
round-9 to round-13 write passes, branch `docs/review-round-9`): the
round-13 write pass, f611db2, merged with the default branch at
2e775c7. It was read from a detached clone that nothing wrote to while
the reviewers ran. The delta read is 4c2b11d..f611db2. The merge also
brought the text of pull request #34 into 10 10.2, 12 12.1 and 12 12.2
(the environment of the pre-push hook and of each mise command); that
text was gated in its own pull request and is not this round's delta,
and each reviewer was asked only whether it contradicts the round-13
text. None found that it does.

No row changes a decision record, an accepted risk or the scope, so
this page has no "Decisions for the operator" block. R14-02, Advisory
in every report, was applied as must-fix, since it touches the
security-model text of 05 5.4 and the token rows of the index (decided
on 2026-10-11).

Status, as in rounds 6 to 13: **A** = applied as proposed; **M** =
applied in a smaller or different form than proposed, as its row
states; **L** = stated as a limit, no mechanism added. The round was
recorded before its write pass, with each row's resolution a proposal;
each row's status is the one the pass applied, and the pass is in
"Write pass (2026-10-11)" below.

The reports are kept in `round-14/reports/<lens>.json`, as the
reviewers wrote them.

## How the round was run

Five reviewers, one per lens that raised a round-13 finding the write
pass resolved: L0, L2, L3, L6 and L13, each in a fresh context,
read-only, in parallel on c64c468. L2 and L13 ran on `opus`, the rest
on `sonnet`; lenses.md names the strongest tier for L0, and as in
rounds 12 and 13 the re-audit ran it on `sonnet`. Each lens ran once,
with one reader; a re-audit carries no noise probe. The verdicts of
L1, L4, L5, L7 to L12 and L14 to L16 carry forward from round 13, L12's
clean verdict of that round included.

The brief was delivered in two parts, as "The brief, in order" in
lenses.md says: part one (the role and its definition of success, two
incidents of the reviewer's own, the read of the delta with notes, a
postmortem and what is missing) as the first message, and part two
(the lens's bullets and "Where to confirm", the reconcile, the two
self-critique questions and the report) as a second message to the
same reviewer, sent after its part-one notes came back. L0's part two
held no bullets. Each reviewer wrote one JSON report with
`prior_checked`, `prior_resolved_as_stated`, a note per prior finding
and its findings, each of kind `unresolved`, `regression` or `new` as
in round 13, and, unlike round 13, an origin (`listed` or `own`). No
report is missing.

The reports hold 19 findings: 0 Blocking, 1 Required, 18 Advisory; 18
new, 1 unresolved, 0 regression. The consolidation merged them by
section and defect, kept every lens on its row, and checked the
Required finding against the baseline text. At the cluster level all
13 rows are `new`: the one member a reviewer kinds `unresolved` joins
R14-01, whose gap is in text the round-13 pass left, not in a promise
of its rows. The result is 1 Required
row, raised by two lenses, and 11 Advisory rows; nothing is
downgraded, dropped or parked.

### Per lens

`prior_checked` is the number of round-13 findings the lens re-read
against the new text, and `prior_resolved_as_stated` how many of them
read as their round-13 row states. Severities in this table are the
reviewers'.

| Lens | Model | prior_checked | prior_resolved_as_stated | Findings (B / R / A) | new | unresolved | regression | listed / own |
|---|---|---|---|---|---|---|---|---|
| L0 | sonnet | 1 | 1 | 2 (0 / 0 / 2) | 2 | 0 | 0 | 0 / 2 |
| L2 | opus | 4 | 4 | 3 (0 / 0 / 3) | 3 | 0 | 0 | 2 / 1 |
| L3 | sonnet | 2 | 2 | 5 (0 / 0 / 5) | 4 | 1 | 0 | 1 / 4 |
| L6 | sonnet | 3 | 3 | 5 (0 / 1 / 4) | 5 | 0 | 0 | 0 / 5 |
| L13 | opus | 3 | 3 | 4 (0 / 0 / 4) | 4 | 0 | 0 | 2 / 2 |
| all | | 13 | 13 | 19 (0 / 1 / 18) | 18 | 1 | 0 | 5 / 14 |

Every round-13 finding the five lenses re-read reads as its row
states, the Required row R13-01 included (in L0's, L3's, L6's and
L13's reports). L3 reports L3-r14-2 as `unresolved` although it counts
L3-R13-1 as resolved as stated: the R13-01 row did not promise a reset
of the count, which L3-R13-1 offered as an alternative, and L3 raises
the reset again. The consolidation reads it as the same defect as
L6-r14-1, and kinds the cluster `new`.

## Findings and resolutions

One row per cluster. The Required row comes first, then the Advisory
rows. Kind is the cluster's: `unresolved` when any member is, else
`regression` when any member is, else `new`. Sections are cited as the
spec cites itself.

### Required

| Cluster | Section | Lenses | Kind | Defect | Verification | Resolution | Status |
|---|---|---|---|---|---|---|---|
| **R14-01** | 04 4.4 (`RJ-338` `fingerprint-unstable`); 03 3.8 (`salvage[].cause`); 05 5.2 (I17) | L6 (L6-r14-1, Required), L3 (L3-r14-2, Advisory, which L3 kinds `unresolved`) (high-signal: two lenses) | new | No text says whether a rerun that resumes at `sbx env rm`, and whose `sbx env rm` then fails again, ends the row of salvages that a changed fingerprint started. The count reads the newest `fingerprint-changed` entries of the generation, a resume writes no entry, and a `removing` generation admits no `requested` salvage (a standalone `salvage` there exits 2 with `removal-pending`, `RJ-336`), so after three such salvages spread over days of a failing `sbx env rm`, one later edit makes the first new salvage exit 5 with a hint about a writer that is not running. I17 stops before that case, so a test passes whether the row resets or not. | holds as reported, in part: the text gives one reading, not two. 03 3.8 says the salvages in a row "are the newest entries of the generation with `fingerprint-changed`, which `fingerprint-unstable` counts", 04 4.4 counts "from `salvage[].cause` as 03 3.8 reads it", and a resume writes no salvage entry, so the row never resets within a `removing` generation, as the R13-01 row's own verification noted at 4c2b11d ("a count that grows past 3 and never resets"). What is missing is the rule said as a rule, a hint that is true in this case, and the case in I17 that turns red when the counting changes. Severity as L6 graded it: T008 builds the error table from 04 4.4 and the salvage tasks build the rerun from 03 3.8 and I17, so the gap reaches the next task that implements the cited sections | recommended: keep the reading the text gives, and say it. 04 4.4 states that the row is the generation's `fingerprint-changed` entries since its newest `requested` entry, that a resume at `sbx env rm` writes no entry and so does not end it, and that once the row holds three, every later salvage a changed fingerprint starts on that generation exits 5 once, after which a rerun whose fresh fingerprint equals the one written resumes; the hint says so in place of implying a writer is running. I17 gains the case: three changed-fingerprint salvages, a rerun that resumes and whose `sbx env rm` fails, one edit, a rerun that salvages, records its entry and exits 5, then a rerun that resumes (the removal that turns it red: resetting the count on a resume). Cost: one sentence in 04 4.4 and one E case; no change to the state format. The other option, a row that a resume ends, needs a recorded resume in the state format (a new field under the version rule of 03), so it adds a concept for a pause that costs one extra rerun. Approval line `spec` | A |

### Advisory

Advisory findings are applied by default (lenses.md, step 4). The
Defect cell keeps the reviewer's first sentence in short; the report
holds the rest. The platform behavior L2-r14-2 and L2-r14-3 rest on is
from the reviewer's reading, not measured, as the reports say.

| Finding | Section | Lenses | Kind | Defect | Resolution | Status |
|---|---|---|---|---|---|---|
| **R14-02** | 10 10.2 (maintainer block, item d); index (Deferred decisions: the four rows whose trigger is "the token is narrowed"); 05 5.4 (the token row); ADR 0011, decision 2; 11 11.1 (A16) | L0 (L0-r14-1, L0-r14-2), L2 (L2-r14-1), L3 (L3-r14-1), L6 (L6-r14-3), L13 (L13-r14-4) (high-signal: five lenses) | new | The event "the token is narrowed" is recorded only by the four tries repeated with the narrowed token, and no text says which answer shows the narrowing: the narrowed token can still merge and push, so answers equal to the ones of 2026-10-08 would be saved as the record. The four tries are listed only in the note on the maintainer block, which item d does not link at the tries; ADR 0011, decision 2, names probe A16 next to the tries, and A16 is the read that shows "limited to this repository"; nothing records that the GitHub authentication SSH key is no longer forwarded; and the answers go under the Evidence of a pull request although the narrowing is a host step with no pull request of its own. | item d names the record of the narrowing: the host read of probe A16 (the token's type, its one repository and its permissions, no Administration), a host-side check that the GitHub authentication key is no longer forwarded into the sandbox, and the four tries repeated, each with its expected answer (a ruleset update refused; a merge through the API still accepted), run or confirmed by the maintainer from the host; answers equal to the 2026-10-08 ones mean not narrowed. The record is a dated entry in the note on the maintainer block, and item d links the note's list of the four tries. The four rows cite that record in place of restating it. Approval line `spec` | A |
| **R14-03** | index (the token-narrowing row, "Until then" after the narrowing); 05 5.4 (the token row) | L2 (L2-r14-3) | new | The list of what the narrowed token can still do is stated as fact, and it leaves out deleting branches and tags (Contents write, the reason the create-only host-ref row exists), creating a `v*` tag if the tag ruleset's bypass applies to the narrowed token (not measured), and editing pull request bodies, approval lines included. | both cells say the list is what ADR 0011, decision 2, grants, until the record of R14-02 measures it, and add the three missing capabilities, the tag bypass marked as unmeasured. Approval line `spec` | M |
| **R14-04** | 10 10.2 (item d, the required-name try, step 3); 05 5.4 (the approval-line row) | L2 (L2-r14-2) | new | Step 3 deletes the side branch and its workflow but not the throwaway pull request's head branch, so the status and check posted under required names stay on that head commit and a reopened pull request could merge with no job run. | step 3 also deletes the throwaway pull request's head branch, the head is an empty commit, and the maintainer confirms the cleanup from the host. Approval line `spec` | A |
| **R14-05** | 10 10.2 (maintainer block: item d, and the "To do" list); 05 5.4 (the token row); index (the token-narrowing row) | L3 (L3-r14-4) | new | The required-name try and the repeat of the four tries, both still to do, sit in item d under "Items a to d ran ... What they set up, and holds now", away from the "To do" list a maintainer reads; the try the token row says "is not made" has no name next to the named required-name try; and one index row says "operator" and "maintainer" for one person. | item d keeps what ran and holds; the two pending tries move to "To do" as their own items; the token row names the unmade try ("the ruleset-disable try"). The two names run through the whole spec, not this delta, and are left to the round that next reviews the vocabulary of 01 1.7. Approval line `spec` | M |
| **R14-06** | 04 4.4 (the `RJ-338` hint); 03 3.8 (`salvage[].fingerprint`) | L13 (L13-r14-1) | new | The hint says the first rerun after a recovery salvages once more "always so after an edit of `salvage.excludeIgnored`", but 03 3.8 fingerprints the ignored paths minus the list, not the list, so an edit that excludes no present ignored path leaves the fingerprint unchanged. | "when the edit excludes an ignored path that is present" in place of "always"; the round-13 verification note that repeats "always" stays as history. Approval line `spec` | A |
| **R14-07** | 05 5.2 (I17: the `excludeIgnored` case; the cause of the first salvage) | L13 (L13-r14-2), L6 (L6-r14-2) (high-signal) | new | The `excludeIgnored` case does not say what the writer does (it holds only when the writer writes nothing but ignored paths the edit excludes) nor which exit 5 it follows; and no case shows that the first salvage writes `cause: requested`, or that a `requested` entry ends the row. | the case names its writer (one that writes only an ignored path the edit then excludes) and follows the third salvage's exit; one assertion reads `cause: requested` on the first salvage of a generation; a case with a standalone salvage between changed-fingerprint salvages, on an `open` generation, shows the row restarting (with R14-01, which says where a row starts). Approval line `spec` | A |
| **R14-08** | 09 J3 (step 5); 01 1.6 (the recovery paragraph after the generation table) | L13 (L13-r14-3) | new | Both say a rerun on a `removing` generation salvages again when the fingerprint changed, with no cap and no cite of `RJ-338`, so a reader of the journey meets an exit 5 it does not explain. | each cites `RJ-338` ([04 4.4](../spec/04-cli.md#44-error-ids)) in one clause, as 08 8.5 does. Approval line `spec` | A |
| **R14-09** | 03 3.8 (`salvage[].cause`); 01 1.6 (the `closed-lost` rows) | L3 (L3-r14-3) | new | `requested` is defined as any salvage a changed fingerprint did not start, so the lost record `run` writes carries `requested` although nobody requested it, and a future value would fall under it silently. | the cell lists the closed set of values and which writer writes each: `requested` for `salvage`, `rm`, `recreate` and `retire` from `open` or `salvaging`, `fingerprint-changed` as now, and the value of a lost record (one of the two, or a third value, decided by the write pass from 01 1.6). Approval line `spec` | A |
| **R14-10** | 05 5.2 (I17); index (the token-narrowing row) | L3 (L3-r14-5) | new | The I17 test cell now holds about twenty `E:` clauses in one table cell, which cannot be cited by case title; and the token-narrowing row narrates its own history ("is no longer one ... (round 13, R13-03)") in a normative cell. | the history sentence moves to this page and the row keeps the rule; splitting I17's cell into titled cases is left to the task that writes the I17 tests, recorded here | M |
| **R14-11** | plan T005 (requirement (b)) | L6 (L6-r14-4) | new | `go list -deps -test` reads one GOOS and GOARCH, so an import behind a build tag for another target is not seen, while 10 10.2 runs the linter once per lint target for that reason; and the dotless-module fixture names no removal that turns it red. | (b) runs the listing once per lint target of 10 10.2, from the same list; the dotless fixture names its removal (reading the standard library by the dot rule instead of `Standard`) | A |
| **R14-12** | plan T005 (requirement (e)) | L6 (L6-r14-5) | new | The fixture pair covers different globs under the same ids, not different ids, though (e) asks for the same surface set; and the one-reader branch names no removal that turns its test red. | a third fixture with one id more in one file fails; the one-reader branch names its removal (a second, inlined reader) | A |

Out of scope, recorded for the round that touches it: L2 notes that
the attestation verification of 10 10.2, S1 and J1 pins the workflow
path and the commit but no ref, so a `release.yml` pushed on a side
branch plus a bypassed `v*` tag could, by its reading of gh's flags,
produce an attestation that passes. It is outside this delta and not
read from gh's source; it belongs to the round that next touches the
verification contract, or to the pull request that adds `release.yml`
(the deferred row of the index on `release.yml`).

## Exit criteria of stage A

The five criteria of lenses.md, "How stage A runs, and when the spec
counts as validated", judged on c64c468:

1. **No Blocking open; every Required fixed or declined with its
   decision and date: not met.** 0 Blocking. 1 Required row is open
   (R14-01), not fixed and not declined. The Required row of round 13,
   R13-01, reads as resolved in every report that re-read it.
2. **Coverage: carried from round 8, as in rounds 9 to 13.** A
   re-audit reads only the changed sections, and its reports carry no
   complete coverage table for the whole set, so the round-8
   measurement stands ([round-8.md](round-8.md#coverage)).
3. **The open questions and deferred decisions of the index have a
   valid shape: met for 101 of 101 rows as measured.** 101 rows: 24
   open questions and 77 deferred decisions (36 in the product, 11
   with the runtime ledger, 30 in how this repository is run), the same
   count as round 13. Every row has all its cells filled, and no two
   rows share a first cell, measured by a script over the tables at
   c64c468. R14-02 finds that the event four rows cite is not yet
   observable as written; the rows keep a trigger, so their shape
   holds. Duplicates in meaning were not measured beyond the reviewers'
   reading.
4. **The three measurements: carried from round 8.** The `listed` to
   `own` ratio per lens, the clusters only L0 raised and the overlap of
   the noise-probe pairs are in
   [round-8.md](round-8.md#measurements). This round ran no noise
   probe; its reports record an origin per finding (5 listed, 14 own,
   in the table above). L0 raised two findings, and both joined R14-02,
   which four other lenses raised, so no finding is L0's alone.
5. **The targeted re-audit raises no new Required finding: not met.**
   It raised 1 Required row, R14-01, of kind `new`: the reset of the
   count, which L3-R13-1 offered in round 13 as an alternative and the
   R13-01 row neither adopted nor declined. The count of Required clusters fell
   from 3 in round 11 and 2 in round 12 to 1 in round 13 and 1 here.

Validated for T004 onward: no. 0 Blocking; 1 Required open (R14-01,
raised by two lenses), 0 declined; coverage and the three measurements
carried from round 8; 101 of 101 index rows with a valid shape as
measured; the re-audit raised 1 Required row.

R14-01 does not cite the `sequences` step of 10 10.2, the section T004
implements; the criteria are judged for the spec as a whole, so the
answer stays no until they hold.

What is left before T004:

- the write pass of this page's rows, with the `spec` approval line:
  done, in "Write pass (2026-10-11)" below; no row needed a decision,
  R14-01's recommendation included, since it states the reading the
  text already gives;
- a targeted re-audit of that pass, round 15, by the lenses whose
  findings drive it: L2, L3, L6 and L13, and L0 as the control,
  reading only the sections it changes.

## Write pass (2026-10-11)

The pass ran on branch `docs/review-round-9`, the branch of pull
request #36, at 19863f5, in its worktree. Each row's status above is
the one applied. Rows applied in a smaller or different form:

- R14-03 (M): the full list of what the narrowed token can still do,
  with the tag bypass marked as not measured, is in the token
  narrowing row of the index; the token row of 05 5.4 keeps its own
  sentence and points to that list, so the text of the accepted risk
  stays as it stands.
- R14-05 (M): the two pending tries left item d and are items g and h
  of the "To do" list of 10 10.2; the unmade try is named the
  ruleset-disable try in the token row of 05 5.4. The two names
  "operator" and "maintainer" were not touched.
- R14-10 (M): the history sentence left the token narrowing row and is
  kept here: the sitting that applies the narrowing is no longer an
  event of that row, since it only ended the state before it (round
  13, R13-03). Splitting the I17 cell into titled cases is left to the
  task that writes the I17 tests, as the row proposed.

Each claim a row writes was checked against 19863f5 before it was
written:

- R14-01: a resume at `sbx env rm` writes no salvage entry (01 1.6, the
  write order); `salvage` is a **P** command (04 4.2), so on a
  `removing` generation with its sandbox present it exits 2 with
  `RJ-336`; a generation goes back to `open` from `salvaging` when the
  sandbox half fails (01 1.6), so a `fingerprint-changed` entry can
  precede a later `requested` one, which the new I17 case uses.
- R14-02: ADR 0011, decision 2, says the rows fire when the narrowing
  is applied and that probe A16 and the four tries run again after it;
  the dated entry of item g is the record of that application, so no
  decision of the record changes. A16 (11 11.1) reads a fine-grained
  token's repository list and permissions by hand from GitHub's token
  settings page. The expected answers are derived from the permissions
  decision 2 grants; the tag try is left unpredicted, as R14-03 says.
- R14-04: the original procedure in the note on the maintainer block
  had the maintainer check what took effect from their own session and
  delete the throwaway branch; item h now does both.
- R14-06: 03 3.8 fingerprints "ignored paths, minus
  `salvage.excludeIgnored`, each by path, size and modification time".
- R14-09: the lost records are written by the `closed-lost` rows of
  01 1.6 and by `salvage --from-host`; both are now `requested`.
- R14-11: 10 10.2 names one list of lint targets that the tests read
  too.

Found while writing and not changed, for the next round that touches
it: the note on the maintainer block dates the first four tries
2026-10-07, as ADR 0008 does, while 10 10.2, 05 5.4 and ADR 0011 date
them 2026-10-08. The new table in the note names them as "the answer
in item d" and carries no date.

ADR 0011 is not edited, and no accepted risk of 05 5.4 changes. Nothing
under `tools/**`, `.github/**` or `.githooks/**` changed.
