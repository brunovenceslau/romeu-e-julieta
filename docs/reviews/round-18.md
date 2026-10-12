# Spec review round 18: the targeted re-audit of round 17

Reader: reviewers checking that every finding of round 18 was handled,
and maintainers tracing why the spec reads as it does. Type: reference
(history). The current state is [docs/spec.md](../spec.md); round 17 is
in [round-17.md](round-17.md); the lenses are defined in
[lenses.md](lenses.md).

Round 18 is step 6 of "How stage A runs, and when the spec counts as
validated" in lenses.md: the targeted re-audit of the round-17 write
pass, which applied the round-17 rows, the decisions of 2026-10-12 and
the exit rule of stage A decided that day. It is the first round graded
by that rule: a Required finding names its class, `contradiction`,
`behavior-gap` or `test-detail`, and only the first two block. lenses.md
is cited at the baseline commit.

The baseline is commit 0a32191, the head of pull request #36 (the
round-9 to round-17 write passes, branch `docs/review-round-9`), read
from a detached clone that nothing wrote to while the reviewers ran.
The delta read is 7155491..0a32191.

This page records the round before its write pass: every row below is
a proposal, with the status **open** until a write pass applies it. The
round raised blocking Required rows, so the session that ran it stopped
after recording it, as its brief asked, and added no session
checkpoint. Three rows would change text that a decision fixed, so they
are in "Decisions for the operator" below.

The reports are kept in `round-18/reports/<lens>.json`, as the
reviewers wrote them.

## How the round was run

Four reviewers, each reading only what the pass changed in its scope:
L6 and L13 on the salvage text (01 1.6, 04 4.1, 04 4.2, 04 4.4, 03 3.8,
08 8.5, J3, with I17 read against it) and the acceptance items added to
T008, T068 and T069; L0 on the amendment of lenses.md, the same
acceptance items and the write-pass record of round-17.md; L2 on the
token and maintainer text (the note on the maintainer block, 10 10.2
items g and h, index row 348, the plan's operator blocks), which the
pass touched. L0, L2 and L13 ran on `opus`, L6 on `sonnet`. Each lens
ran once, with one reader, in fresh contexts, read-only, in parallel.
The verdicts of the lenses that did not run carry forward: L3 from
round 15, the others from round 13.

The brief was delivered in two parts, as in rounds 14 to 17: part one
as the first message, part two (the lens's bullets and "Where to
confirm", the reconcile, the self-critique questions and the report)
as a second message to the same reviewer after its part-one notes came
back; L0's part two held no bullets. Both parts carried the grading
rule. The report fields are those of round 17, plus `class` on each
finding. No report is missing, and no reviewer wrote any other file.

The reports hold 32 findings: 0 Blocking, 8 Required, 24 Advisory; 24
new, 1 unresolved, 7 regression. The reviewers classed all 8 Required
findings as `contradiction` or `behavior-gap`; two of them, L6-r18-1
and L13-r18-3, are one defect and merge. The consolidation checked each
against the baseline text and kept every class. The result is 7
Required rows, every one of class `contradiction` or
`behavior-gap`, so all 7 block, and 14 Advisory rows. Nothing is
downgraded, dropped or parked, except the plan rows that the lenses
themselves marked `parked for B`.

### Per lens

Severities in this table are the reviewers'.

| Lens | Model | prior_checked | prior_resolved_as_stated | Findings (B / R / A) | new | unresolved | regression | listed / own |
|---|---|---|---|---|---|---|---|---|
| L0 | opus | 8 | 8 | 9 (0 / 0 / 9) | 9 | 0 | 0 | 0 / 9 |
| L2 | opus | 7 | 5 | 5 (0 / 2 / 3) | 2 | 1 | 2 | 0 / 5 |
| L6 | sonnet | 7 | 7 | 9 (0 / 3 / 6) | 9 | 0 | 0 | 0 / 9 |
| L13 | opus | 9 | 8 | 9 (0 / 3 / 6) | 4 | 0 | 5 | 0 / 9 |
| all | | 31 | 28 | 32 (0 / 8 / 24) | 24 | 1 | 7 | 0 / 32 |

The three round-17 findings not counted as resolved are L2-r17-2
(R17-14: read 2 was weakened, R18-02), L2-r17-4 (R17-15: a re-narrowing
is still no event, R18-A12) and L13-r17-9 (R17-09: the restart row
still says "killed, or failed", R18-A1).

Two of the seven blocking rows were written by the round-17 pass itself
(R18-01, R18-02, both on the maintainer text); two more are in text
the pass rewrote (R18-05 in J3 and T069, R18-07 in the R17-11
paragraph); three predate it (R18-03, R18-04, R18-06). Under the exit
rule, round 17 had one blocking row (R17-03) and round 18 has seven.

## Decisions for the operator

| Item | Question | Options | Recommendation |
|---|---|---|---|
| R18-03 | The details of `RJ-338` name "the first path, in byte order, whose entry changed" (round 15, R16-12), but the state keeps only the sha256 of the entries, so the rerun has nothing to compare a fresh entry with; and no page says which call reads the fresh values or carries the stored ones to romeu | (a) store, next to `fingerprint`, the sorted digest of each entry, and name the julieta output that carries the values (08 8.5 steps 2 and 4, 04 4.3, 03 3.11); (b) keep one digest and make the details name the kind of entry that changed (daemon heads, a worktree, ignored paths), amending the decided wording; both also correct 03 3.8's "step 2" to "steps 2 and 4" | (b): it adds no store and no field, keeps the fingerprint one value as decided, and the hint already tells the operator what to do without the path; (a) is the one to take if a report shows the kind is not enough |
| R18-06 | 04 4.1 says a fix hint begins with a command line or with `decide:`, and a test parses each; the fixed hint of `RJ-338`, decided on 2026-10-12 and amended for R17-06, begins with prose, and T008 now asserts it byte for byte. The hint also does not say that the `salvage.excludeIgnored` route ends with `romeu sync` exiting 2 with `removal-pending` (R18-A6) | (a) amend the fixed text to open with the command, as in "romeu rm <name>, once nothing writes in the sandbox any more; ..."; (b) 04 4.1 states that a hint may be one fixed text that a row of 04 4.4 quotes, and names `RJ-338` as such a hint | (b): the hint was chosen word for word twice, and a one-clause exception in the rule keeps both texts true; the `sync` sentence then waits for the first report that the exit 2 confused an operator |
| R18-07 | The paragraph decided as R17-11 option (a) states that writes made after a salvage's read and before its `sbx env rm` are not salvaged on a removal from `open`, or on a salvage restarted from `salvaging`, which includes a killed salvage that a changed fingerprint started; the index says a part of a security invariant is deferred only as a risk accepted by name in 05 5.4, and the 05 5.4 row says salvage proves that nothing the sandbox still holds is lost | (a) R17-11's option (b): a 05 5.4 row for that window, the kill route named, reopened by the first report of such a loss, and the existing row narrowed to what the sandbox held at the salvage read; (b) keep the paragraph and record the window as a stated limit, which leaves the principle of the index unmet | (a): it is the form the index asks for, and it adds no mechanism; it changes the list of accepted risks, which is why it is asked |

## Findings and resolutions

One row per cluster. The Required rows come first, then the Advisory
rows. Kind is the cluster's: `unresolved` when any member is, else
`regression` when any member is, else `new`. Sections are cited as the
spec cites itself.

### Required

| Cluster | Section | Lenses | Kind | Defect | Verification | Proposed resolution | Class | Status |
|---|---|---|---|---|---|---|---|---|
| **R18-01** | index (row 348); plan (Operator blocks, opening); the note (the entry rule) | L2 (L2-r18-1) | regression | Row 348 now says the narrowing is "applied at block O8 of the plan; the token stands until then", which ties the end of the operator's token to the block; the plan's opening, as R17-17 wrote it, ties it to the note's first narrowed entry, and the note says the newest entry's verdict decides. If the entry written at O8 is not narrowed, the texts cannot all hold. | holds as reported: the R17-20 edit of row 348 wrote "applied at block O8"; the plan's opening reads "until the note on the maintainer block records its first narrowed entry" | row 348: "applied by the note's first narrowed entry, planned at block O8; the token stands until that entry". Cost: one clause. Approval line `spec` | `contradiction` | A |
| **R18-02** | the note (read 2, the expected answer); ADR 0011 (decision 2); 04 4.1 (the `SSH_AUTH_SOCK` of sbx calls) | L2 (L2-r18-2) | regression | The R17-14 edit accepts "the setting names no socket" as an expected answer, but sbx forwards the `SSH_AUTH_SOCK` of the calling environment (inferred, not measured), so a host agent inherited by the shell, with the authentication key, passes read 2, and the verdict reads narrowed while ADR 0011, decision 2, "the forwarding of the GitHub authentication SSH key to the sandbox stops", does not hold. | holds as reported: R17-14 proposed "no socket is forwarded"; the pass wrote "the setting names no socket" | read 2 reads the effect: when no setting names a socket, it lists, on the host, the keys behind the `SSH_AUTH_SOCK` of the shell that runs sbx, and expects none, or the signing key alone. Cost: one sentence. Approval line `spec` | `contradiction` | A |
| **R18-03** | 03 3.8 (`salvage[].fingerprint`); 04 4.4 (`RJ-338`, the details); 08 8.5 (steps 2 and 4); 04 4.3; 03 3.11 | L6 (L6-r18-1), L13 (L13-r18-3) (high-signal) | new | The details name the first changed path, but only one sha256 is stored, so no old entry exists to compare with; no page names the call that reads a fresh fingerprint on a rerun with no salvage, or the output that carries the values to romeu; and 03 3.8 credits ignored paths to step 2, which reads only HEAD and status (step 4 reads ignored files). | holds as reported: 03 3.8 defines one plain sha256; 08 8.5 steps, the `julieta salvage` row of 04 4.3 and 03 3.11 do not mention the fingerprint; I17's own case says step 2 "does not see" ignored paths. It predates the round-17 pass | the item in "Decisions for the operator"; either option also writes "steps 2 and 4" in 03 3.8 | `behavior-gap` | A |
| **R18-04** | 01 1.6 (the two `removing` rows with the sandbox present); 04 4.2 (`rm`) | L6 (L6-r18-2) | new | The two rows split on a fresh fingerprint that is equal or differs, but the read needs the sandbox (the daemon heads, each worktree's status), and no row, exit or error covers a read that fails: a stopped sandbox, an unreachable daemon, a julieta that exits non-zero. | holds as reported: the only "changes nothing" rule in 01 1.6 covers `sbx ls --json`; the rows give no third outcome. It predates the pass, which made the rows explicit | one clause in 01 1.6: a fresh fingerprint that cannot be read exits 1 and changes nothing, as a failed `sbx ls --json` does, and the hint names the reason. Cost: one clause. Approval line `spec` | `behavior-gap` | A, T (T068) |
| **R18-05** | 04 4.4 (`RJ-338`); 08 8.5; 09 J3 (step 5); 01 1.6 (the `removing -> salvaging` row); plan T069 | L13 (L13-r18-2) | regression | These texts give a salvage that a changed fingerprint started two endings, `RJ-338` or `RJ-306` with exit 5, while 01 1.6 has more: a failed sandbox half writes `result: incomplete` and exits 1 with no error id, and a sandbox found absent goes to `closed-lost`. | holds as reported: the 01 1.6 row "the sandbox half fails ... \| open \| ...; exit 1" and the absent rows; J3 and T069 were written by the round-17 pass | "the `salvaging` rows decide how it ends": `RJ-338` when complete or accepted, `RJ-306` (exit 5) for a verified incomplete salvage, exit 1 for a failed sandbox half, `closed-lost` for an absent sandbox; 01 1.6 says "the incomplete rows"; T069 follows. Cost: sentence edits on four pages | `contradiction` | A |
| **R18-06** | 04 4.1 (Error ids, the fix-hint shape); 04 4.4 (`RJ-338`); plan T008 | L6 (L6-r18-3) | new | 04 4.1 says a fix hint begins with a command line or with `decide:`; the fixed hint of `RJ-338` begins with prose, and T008 asserts it byte for byte. | holds as reported: 04 4.1 reads "a fix hint begins with a command line the user can run ... or with `decide:` and the choice"; the hint opens "If nothing writes in the sandbox any more". The shape predates the pass (R16-03) | the item in "Decisions for the operator" | `contradiction` | A |
| **R18-07** | 01 1.6 (the paragraph of R17-11); 05 5.4 (the salvage row); index (Decide at the last responsible moment); I17 | L13 (L13-r18-1) | new | The paragraph states a part of I17 that no mechanism guards, the kill route past `RJ-338` among it, with no row in 05 5.4, while the index says such a part is deferred only as a risk accepted by name there. | holds as reported: index, "A part of one is deferred only as a risk accepted by name in 05 5.4, and its row says so"; the 05 5.4 row reads "Salvage proves that nothing the sandbox still holds is lost" | the item in "Decisions for the operator" | `contradiction` | A |

### Advisory

Advisory findings are applied by default (lenses.md, step 4), by a
sentence edit, a stated limit with its trigger, or an acceptance item.
The Defect cell keeps the reviewer's first sentence in short; the
report holds the rest.

| Finding | Section | Lenses | Kind | Defect | Proposed resolution | Status |
|---|---|---|---|---|---|---|
| **R18-A1** | 01 1.6 (the restart row, the recovery paragraph, the new paragraph); 04 4.4 | L13 (L13-r18-4), L6 (L6-r18-5, L6-r18-6) (high-signal) | regression | The restart row says "killed, or failed" where the new paragraph and `RJ-338` say "killed", and a failed sandbox half goes to `open`; the recovery paragraph calls the restart under a new salvage id "resumed", verifying refs created under an earlier id. | the restart row and the paragraphs use one word, "restarted under a new salvage id", and name the failures that leave `salvaging` (a host-side error after the sandbox half) or drop "failed" | A |
| **R18-A2** | 04 4.1; 04 4.2 (`rm`); 08 8.5; 01 1.6 (`salvaging -> removing`) | L13 (L13-r18-5) | regression | The phrase of R17-05 drops "fresh" on four pages, so it reads as a comparison of stored entries. | "a fresh fingerprint that differs from the newest one written with `removing`" | A |
| **R18-A3** | 03 3.8; 04 4.4 | L6 (L6-r18-4), L13 (L13-r18-7) (high-signal) | regression | "The greatest salvage id" relies on an order no page declares; only the generation id is declared a ULID. | "newest" is the last entry of `salvage[]`, or 03 3.8 declares the salvage id a ULID | A |
| **R18-A4** | 03 3.8; plan T068 | L13 (L13-r18-6), L6 (L6-r18-7) (high-signal) | new | Which of step 2's reads the fingerprint takes, for an accepted salvage that skipped a worktree as `worktree-changing`, is not said; any choice is safe. | 03 3.8: "the last read"; T068's `--accept-loss` test asserts it | A, T (T068) |
| **R18-A5** | 09 J3 (step 5) | L13 (L13-r18-8) | regression | The step joins its two branches with a "so" clause and says the rerun after `RJ-338` resumes, where the hint says the first rerun may exit 5 once more. | reorder the branches and cite the hint for the reruns | A |
| **R18-A6** | 04 4.4 (the hint) | L6 (L6-r18-8) | new | The hint's `salvage.excludeIgnored` route ends with `romeu sync` exiting 2 with `removal-pending` after the promotion, which the hint does not say. | part of R18-06 in "Decisions for the operator" | L |
| **R18-A7** | plan T008 | L0 (L0-r18-8), L13 (L13-r18-9) (high-signal) | new | The byte-equal test of the hint names no reference copy and does not say whether the code spans' backticks of the 04 4.4 quote are kept. | T008 names the reference (the parsed 04 4.4 cell, backticks stripped) | T (T008) |
| **R18-A8** | plan T068 | L13 (L13-r18-9), L0 (L0-r18-9) (high-signal) | new | The R17-02 item lacks the precondition the R17-01 item has (the fingerprint already differs when the rerun starts), and the kill point is a range of steps that 01 1.6's write order does not list. | add the precondition; name the write `removing -> salvaging` as a step of the fault-injection list | A, T (T068) |
| **R18-A9** | plan T069 and T008 (the Implements fields; parked for B); plan T069 (acceptance); 04 4.1 (the hint-parsing test, parked for B) | L0 (L0-r18-6, L0-r18-7), L6 (L6-r18-9) | new | T069 does not assert that `retire` moves nothing on an `RJ-338` or `RJ-306` exit, or that the move completes after the resume; the Implements fields of T069 and T008 omit the sections their new items test; the 04 4.1 hint-parsing test has no task. | T069 gains the no-move assertion; the Implements fields and the parsing test are parked for B | T (T069) |
| **R18-A10** | lenses.md (the stage table, steps 3, 4 and 6, the severity scale, the dated decision) | L0 (L0-r18-1, L0-r18-2, L0-r18-5) | new | An acceptance item that carries a `test-detail` finding is never judged in stage A, since findings on `plan.md` are parked for B; a class set lower than the reporting lens's has no route to the operator; "Required (fix before the next plan task)" does not fit `test-detail`; the dated decision says neither why nor whether it covers rows open before it. | step 6 says the re-audit judges those items as acceptance; step 4 sends a lowered class to the operator; the scale and the dated sentence get one clause each | A |
| **R18-A11** | lenses.md (the classes); 05 5.2 (I17, "write during the salvage"); 08 8.5 (step 2) | L0 (L0-r18-3) | new | The rule that the spec text is not grown for `test-detail` leaves I17's case worded so that one write reads as enough, which step 2 absorbs; the correct construction is only in T068. | a stated limit in lenses.md: a spec test cell may get a wording fix that changes no behavior, reopened by the plan review of stage B | L, A |
| **R18-A12** | the note (the entry rule, limit 5, the fourth part); 10 10.2 (item h) | L2 (L2-r18-5) | unresolved | A narrowed entry after a not narrowed one is no event, and item h's answers stay in the first narrowed entry even when a later or replaced token is the one tried. | limit 5 names the re-narrowing; limit 4 names item h among what a replaced token does not re-run | L |
| **R18-A13** | the note (the limits, closing line); plan O8 | L2 (L2-r18-3) | new | Limits 4 to 6 are reopened by the sitting of block O8, the sitting that writes the first entry and so starts them; after it they have no live trigger. | a trigger after O8: the first of a re-created development sandbox, a replaced token, a release-candidate tag, checkpoint C7 | A |
| **R18-A14** | the note (the entry rule); 05 5.4 (the token row); round-17.md (R17-02, the write pass) | L2 (L2-r18-4), L0 (L0-r18-4) | new | The note says the 05 5.4 token row describes both states, but the row is in the present tense and still says "applied at the next maintainer sitting"; separately, R17-02's phrase edit in I17 was dropped without a record. | the note says the 05 5.4 row describes the state before the narrowing and points to row 348 for the state after; the write pass of round 18 records that the I17 phrase of R17-02 went to T068 with the rest | A |

## Exit criteria of stage A

The five criteria of lenses.md, as amended on 2026-10-12, judged on
0a32191:

1. **No Blocking open; every Required of class `contradiction` or
   `behavior-gap` fixed or declined with its decision and date; every
   `test-detail` Required an acceptance item: not met.** 0 Blocking. 7
   Required rows are open, all of class `contradiction` (R18-01,
   R18-02, R18-05, R18-06, R18-07) or `behavior-gap` (R18-03, R18-04).
   R17-01 and R17-02, of class `test-detail`, are acceptance items of
   T068, citing their ids.
2. **Coverage: carried from round 8, as in rounds 9 to 17**
   ([round-8.md](round-8.md#coverage)).
3. **The open questions and deferred decisions of the index have a
   valid shape: carried from round 17.** The round-17 pass edited only
   row 348's cells, which keep their trigger; no row was added or
   removed. R18-A13 is about a trigger in the note, not a row of the
   index.
4. **The three measurements: carried from round 8**
   ([round-8.md](round-8.md#measurements)). This round ran no noise
   probe; its reports record an origin per finding (0 listed, 32 own).
   L0 raised nine findings, all Advisory and all its own; R18-A10 and
   R18-A11 are L0's alone, on the amended exit rule. The lens critiques
   add input for the lens rewrite before stage B.
5. **The targeted re-audit raises no new Required finding of class
   `contradiction` or `behavior-gap`: not met.** It raised 7.

Validated for T004 onward: no. 0 Blocking; 7 blocking Required open
(5 `contradiction`, 2 `behavior-gap`), 0 declined; 2 `test-detail`
Required of round 17 in T068's acceptance; coverage and the three
measurements carried from round 8; the index rows carried from round
17; the re-audit raised 7 blocking Required rows.

What is left before T004: the final write pass below and its one
review, and then nothing of this review.

The `.githooks/pre-push` row of 12 12.1 is edited by a separate pull
request, #41; pull request #36 does not edit that row, and no write
pass of this review touches it.

## Final write pass (2026-10-12)

The pass ran on branch `docs/review-round-9`, the branch of pull
request #36, at c14479f, in its worktree. Each row's status is the one
applied: A, M or L as in rounds 6 to 17, and T for a row moved to the
acceptance of a plan task, which the status names. No decision record
is edited, no accepted risk of 05 5.4 changes except as R18-07 decides
it, and nothing under `tools/**`, `.github/**` or `.githooks/**`
changed; the `.githooks/pre-push` row of 12 12.1, plan.md's language
bullet and the `tools/release` lines of 10 10.2 are not touched.

Decided on 2026-10-12, and applied here:

- this pass is the final pass of stage A: one writer and one reviewer,
  in fresh contexts, with at most three change exchanges between them,
  after which the review is finished; no lens round follows it, and the
  round-19 re-audit the exit criteria above named is not run. The exit
  rule of lenses.md stands: only `contradiction` and `behavior-gap`
  block, and `test-detail` goes to the implementing task's acceptance
  in plan.md;
- R18-03: its option (b). The details of `RJ-338` name the kind of
  entry that changed, not a path; the fingerprint stays one value;
- R18-06: its option (b). 04 4.1 states, in one clause, that the hint
  of `RJ-338` is the one fixed text a row of 4.4 quotes;
- R18-07: its option (a), which is option (b) of R17-11. A row of
  05 5.4 accepts the window, the kill route named, reopened by the
  first report of such a loss, and the existing salvage row is narrowed
  to what the sandbox held at the salvage read.

The pass then read the two regions that kept producing findings as
wholes, and made each say one thing once: the fingerprint is defined
in 03 3.8 alone, the ending of a salvage that a changed fingerprint
started in the `RJ-338` row of 04 4.4 alone, and the state of the token
in the note on the maintainer block alone; every other page cites the
home. The order of preference was: delete a concept, edit a sentence,
state a limit with what reopens it, move test detail to a task's
acceptance, and add mechanism last of all. One mechanism was added, the
read a rerun makes (R18-03, below).

The Required rows:

- R18-01 (A): row 348 of the index reads "applied by the first narrowed
  entry of the note on the maintainer block, planned at block O8 of the
  plan; the token stands until that entry".
- R18-02 (A): read 2 of the note says that whether sbx forwards the
  agent of the shell that starts it, when no setting names a socket, is
  inferred and not measured, so the read does not rely on the setting:
  the socket its second and third parts read is then the
  `SSH_AUTH_SOCK` of the shell that starts the sandbox, on the host,
  and the expected answer is the signing key alone or no agent, with
  every fingerprint absent from the authentication keys. ADR 0011 is
  not edited.
- R18-03 (A, option (b)): 03 3.8 reads "steps 2 and 4 of 08 8.5" and
  is named the one home of what a fingerprint is; for a worktree step 2
  read more than once, it takes the last read (R18-A4). The details
  name the kind of entry that changed as far as the last entry's
  manifest (03 3.11) tells it: a daemon head or a worktree's HEAD whose
  fresh value differs from the manifest's, and otherwise a worktree's
  status or an ignored path, which the one stored sha256 does not tell
  apart; that lump is the stated limit of option (b). The read a rerun
  makes is named: `julieta salvage --fingerprint` (04 4.3), which runs
  only the reads of steps 2 and 4, writes nothing, stops no agent and
  prints the values as JSON; 03 3.8 and 01 1.6 say the rerun runs it
  over `sbx env exec`, and 08 8.5 says what it runs; the daemon heads,
  which steps 2 and 4 do not read, romeu fetches itself as step 3 of
  its half does. `RJ-338` and the
  I17 case cite 03 3.8 for the kind, and the I17 case turns red on
  "naming no kind". The tasks that build `julieta salvage` (T061,
  T062) are outside this pass; the stage A to B delta on 04 4.3 and
  08 8.5 names them.
- R18-04 (A): 01 1.6 says a stopped sandbox is started for the read and
  stopped again, as for the sandbox half (04 4.2), and that a read
  that fails, because the start fails, the daemon is unreachable or
  julieta exits non-zero, exits 1 and changes nothing, as a failed
  `sbx ls --json` does, with the failed subprocess in the error's
  details (04 4.1); the `rm` row of 04 4.2 and `RJ-338` cite it. No new
  error id: the row keeps parity with the "sandbox half fails" row of
  01 1.6. The test is test detail and is an item of T068 (T (T068)).
- R18-05 (A): the `RJ-338` row lists the four endings once: complete or
  accepted, the entry and `fingerprint` written with `removing`, exit 5
  with `RJ-338` and no `sbx env rm`; verified incomplete, `RJ-306`, exit
  5, `open`; the sandbox half fails, exit 1, `open`, naming
  `--from-host` as the `salvage` row of 4.2 says; the sandbox absent,
  `closed-lost`. The `removing -> salvaging` row of 01 1.6 says "the
  `salvaging` rows decide how that salvage ends, as `RJ-338` lists
  them"; 08 8.5, J6 step 5 and T069 say "ends as `RJ-338` says". The
  row's "09 J3 (step 5)" is J6 step 5: J3 has no numbered steps, and
  the text the lens quoted is 09:196-199, in J6. The `RJ-338` row's
  list of citers reads "09 J6" for the same reason.
- R18-06 (A, option (b)): 04 4.1 reads "except the one hint that a row
  of 4.4 quotes as one fixed text, the hint of `RJ-338`, which a test
  holds byte for byte (the plan, T008) and the parsing test below
  skips", and the parsing test parses "every other fix hint". The hint
  text is not changed by a byte. R18-A6 is a stated limit in the
  `RJ-338` row: the hint does not say that the `romeu sync` of its
  second route exits 2 with `removal-pending` once it has promoted the
  edit (4.2, `sync` step 9), and the first report that the exit confused
  an operator reopens the hint's text.
- R18-07 (A, option (a)): a new row of "Risks of running romeu" in
  05 5.4 accepts that a write made inside the sandbox after a salvage's
  read and before its `sbx env rm` is not salvaged when the salvage ran
  from `open` or was restarted from `salvaging`, the salvage that a
  changed fingerprint started and a killed romeu left in `salvaging`
  among them; why accepted: the fingerprint guards only the rerun on
  `removing`, which has an earlier read, and a marker or a second read
  is mechanism the start does not need; mitigation: `--stop-agents`
  and the recheck of step 2; reopened by the first report of such a
  loss, as an issue labelled `deferred`. The existing row reads
  "Salvage proves that nothing the sandbox held at the salvage read is
  lost; the next row covers what is written after that read", and the
  create-only host ref row of the index reads the same phrase. The
  R17-11 paragraph of 01 1.6 cites the row, and `RJ-338` cites it for
  the restart. No Deferred decisions row is added: the decision names a
  05 5.4 row, and that row carries its own trigger.

The Advisory rows:

- R18-A1 (A): one cause, named by the state: `salvaging` is left by a
  romeu that ended, killed or failed, while it salvaged, whether or
  not it had written the salvage record. The restart row of 01 1.6 says so and adds "the refs the
  earlier salvage created stay"; the recovery paragraph says
  `salvaging` "is restarted by the next salvage under a new salvage id,
  and the refs the earlier one created stay", in place of "resumed ...
  verifies the refs already created"; the R17-11 paragraph and
  `RJ-338` use the same words.
- R18-A2 (A): 04 4.1, the `rm` row of 04 4.2, 08 8.5 and the
  `salvaging -> removing` row of 01 1.6 say "a fresh fingerprint
  (03 3.8) that differs from the newest one written with `removing`";
  the two `removing` rows, the recovery paragraph of 01 1.6, J6 step 5
  and `RJ-338` drop their own list of what a fingerprint reads, which
  omitted the ignored paths that 03 3.8 and the I17 cases include, and
  cite 03 3.8 instead.
- R18-A3 (A): 03 3.8 gains the field row `salvage[].id`, a ULID as the
  generation's id is, and says the newest entry is the last of
  `salvage[]`, which 01 1.6 appends in order; the fingerprint cell
  compares with "the one of the last entry" and no longer with "the
  greatest salvage id", and `RJ-338` cites 03 3.8 for it.
- R18-A4 (A, T (T068)): 03 3.8, "for a worktree that step 2 read more
  than once, the last read"; T068's `--accept-loss` item asserts that
  a rerun which finds the worktree as that read left it resumes.
- R18-A5 (A): J6 step 5 reads "salvages again under a new salvage id
  and ends as `fingerprint-unstable` (`RJ-338`, 04 4.4) says, its hint
  naming the reruns that then resume".
- R18-A6 (L): part of R18-06, above.
- R18-A7 (T (T008)): the reference is the `RJ-338` row's quoted text,
  read from the page by the test with its code-span backticks stripped,
  so the table's hint holds no backtick.
- R18-A8 (A, T (T068)): the write order of 01 1.6 opens with "the state
  `salvaging` with the new salvage id", which it had omitted; T068's
  R17-02 item gains "the fingerprint already differs when the rerun
  starts", and "kill the salvage" kills after each step of that order
  from the write of `salvaging` up to, and not including, the write of
  `removing`, one test per step.
- R18-A9 (T (T069); parked for B): T069 asserts that on an exit with
  `RJ-338` or `RJ-306` nothing is moved, and that the move of J7
  completes after the resume. The Implements fields of T069 and T008
  and the hint-parsing test's task stay parked for B, as the lenses
  marked them.
- R18-A10 (A): lenses.md: step 4 sends to the operator a Required
  finding whose class the consolidation sets lower than the reporting
  lens's; step 6 says the re-audit judges the acceptance items a write
  pass added for `test-detail` findings as acceptance, not parked for
  B; the scale reads "or, for `test-detail`, by that task's
  acceptance"; the dated sentence says the rule applies from the
  round-17 write pass on, its open rows included, and why.
- R18-A11 (L, A): lenses.md states the limit: a `test-detail` finding
  may carry a wording fix of the spec's own test cell when the case as
  written does not reach its stated result, a fix that changes no
  behavior, reopened by the plan review of stage B. Under it, the I17
  case "write during the salvage" reads "a writer keeps changing a
  tracked file through the repeat of 08 8.5 step 2", the construction
  T068 already held.
- R18-A12 (L): limit 5 of the note says a narrowed entry after a not
  narrowed one is no event, since the rows the first narrowed entry
  fired stay fired; limit 4 says the required-name try of item h is
  not run again for a replaced token, so its answers describe the
  token of the first narrowed entry. 10 10.2 item h is not edited.
- R18-A13 (A): limits 4 to 6 are reopened, after the sitting of block
  O8 has written the first entry, by the first of a re-created
  development sandbox, a replaced token, the first release-candidate
  tag or checkpoint C7. Block O8's Recommendation is not edited
  (parked for B, as the lens marked it).
- R18-A14 (A): the note says the index row describes the token before
  and after the narrowing, and the 05 5.4 token row the state before
  it, in its present words, pointing to the index row for the state
  after. The 05 5.4 token row itself is not edited, as the
  round-17 pass decided for R17-15 and R17-20 (limit 5 of the note).
  For the record R18-A14 asks: the phrase edit of R17-02 in I17
  ("naming any reason" to "naming a reason the salvage skipped") was
  not applied by the round-17 pass and is not applied here; the case
  is correct as written for the complete salvage it describes, since a
  complete salvage skipped no reason and `--accept-loss` naming any
  reason leaves it `RJ-338`, and the branch where `--accept-loss`
  names a skipped reason is T068's item (R17-02). Its status stands as
  T (T068).

Outside the rows, within the two regions: the `RJ-338` row's citers
read "09 J6" (R18-05); 03 3.8 and the index's create-only host ref row
read "held at the salvage read" (R18-07); 08 8.5 names what
`--fingerprint` runs (R18-03). Nothing else changed.

Under the closing decision, the review is finished when the reviewer
of this pass has no change left to ask for, within three exchanges.
That reviewer stands in for step 6 of lenses.md and for the fifth exit
criterion, the re-audit that raises nothing: its verdict is recorded in
the pull request, not in a round page. The exit criteria judged on
0a32191 above are not re-judged here; the pull request's reviewer reads
this pass against them.
