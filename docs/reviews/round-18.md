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

The round was recorded before its write pass, with each resolution a
proposal; each row's status is the one the final write pass applied,
in "Final write pass (2026-10-12)" below. The round raised blocking
Required rows, so the session that ran it stopped after recording it,
as its brief asked; the session checkpoint at the end of this page was
added later. Three rows would change text that a decision fixed, so they
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
| R18-03 | The details of `RJ-338` name "the first path, in byte order, whose entry changed" (round 16, R16-12), but the state keeps only the sha256 of the entries, so the rerun has nothing to compare a fresh entry with; and no page says which call reads the fresh values or carries the stored ones to romeu | (a) store, next to `fingerprint`, the sorted digest of each entry, and name the julieta output that carries the values (08 8.5 steps 2 and 4, 04 4.3, 03 3.11); (b) keep one digest and make the details name the kind of entry that changed (daemon heads, a worktree, ignored paths), amending the decided wording; both also correct 03 3.8's "step 2" to "steps 2 and 4" | (b): it adds no store and no field, keeps the fingerprint one value as decided, and the hint already tells the operator what to do without the path; (a) is the one to take if a report shows the kind is not enough |
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

### Exchange 1

The reviewer of the pass read d349b2c and returned four blocking
findings and five advisory ones; this exchange applies them. Statuses
as above.

- Finding 1, `behavior-gap` (A): nothing said how the full salvage
  carried the fingerprint's sandbox-side values to romeu at the
  `removing` write. The `julieta salvage` row of 04 4.3 says its
  `--json` document carries the fingerprint object, which romeu digests
  at that write, and that `--fingerprint` prints that object and
  nothing else; 03 3.8 says the sandbox-side values are taken from that
  object. T061 gains the acceptance item (A4).
- Finding 2, `contradiction` (A): limit 5 of the note reads "a narrowed
  entry that follows a not narrowed one after the first narrowed entry
  is no event", so the first narrowed entry stays the event.
- Finding 3, `contradiction` (A): the index gains a row under "In the
  product" for the marker or second read the new 05 5.4 row defers, a
  part of I17 accepted there, with the same trigger; the 05 5.4 row's
  mitigation cell names the invariant and the row, and its trigger
  reopens both rows.
- Finding 4, `contradiction` (A): the carve-out of 04 4.1 says the
  parsing test and the flag sentence of that cell skip the `RJ-338`
  hint, which names `--accept-loss` only to deny it. The hint text is
  unchanged.
- A1 (A): the `RJ-338` limit reads "when the sandbox is running".
- A2 (A): the restart row and the recovery sentence of 01 1.6 read
  "and wrote no later state".
- A3 (A): the `RJ-308` cell covers the rerun whose fresh fingerprint
  cannot be read, and 01 1.6 names `sbx-unknown` for it; no new id.
- A4 (A): T061, above.
- A5 (L, not applied): the daemon heads stay read by the hardened fetch
  of step 3 of the host half. gitsafe (05 5.1) runs fetches with
  explicit refspecs into allowlisted namespaces and names no
  `ls-remote` mode, so a read without a write would add a gitsafe mode,
  more than one clause; the fetch writes only romeu's own allowlisted
  refs on the host, nothing in the sandbox, and brings nothing when the
  heads are unchanged. Reopened by the first finding that the fetch
  changes a comparison.

### Exchange 2

This exchange applies the decisions of 2026-10-12 on
[ADR 0011](../adr/0011-keep-the-administrator-s-pull-request-bypass-and-give-the-sandbox-a-github-app-token.md),
item 1 of the Next steps below. It is not a reviewer's round: no
finding drove it. Statuses as above.

- ADR 0011 (A): Proposed, with the title "Keep the administrator's
  pull-request bypass and give the sandbox a GitHub App token" and its
  file renamed to match. Decision 1 keeps the administrator role in the
  bypass list of the default-branch ruleset, in `pull_request` mode,
  for the two reasons decided, and is reopened by the first
  measurement in which a merge with the sandbox's token lands over a
  red required check. Decision 2 replaces the classic token with a
  GitHub App installation token delivered as a global sbx dynamic
  secret; isolation per repository is deferred, reopened by a
  repository that a sandbox must not reach. Decision 3 is unchanged.
  Context cites the docs.github.com pages and the "Not verified" list
  of the facts read for the GitHub App route below; Consequences lists
  the App checks that come before Accepted. The `Superseded in part by`
  lines of ADR 0008 and ADR 0010 are removed, since a Proposed record
  supersedes nothing yet; the pull request that accepts ADR 0011 adds
  them back. The index is regenerated, and the links of rounds 9 to 11
  point at the new file name with their text unchanged.
- The current state (A): no spec text rests on ADR 0011 for what holds
  today. S2's evidence and the local-gates row of the index say that
  the administrator role bypasses the required checks on a pull
  request merge, and the local-gates row names decision 1 as the
  proposed record that keeps it; item a of 10 10.2 says the same, and
  item e no longer records under decision 1. The approval row of
  05 5.4 drops "until the maintainer step ... removes it".
- The token (A): the token narrowing row of the index, the token rows
  of 05 5.4, item g of 10 10.2, the note on the maintainer block and
  probe A16 name the App installation token as the proposed narrowing,
  applied at block O8 once ADR 0011 is Accepted; the classic token
  stands until the first narrowed entry. The index row adds the trigger
  "a repository that a development sandbox must not reach" and moves
  the date of its last review to 2026-10-12. Read 1 of the note expects
  an App installation token on selected repositories with the same
  permissions as before. Item h of 10 10.2 names no record and is not
  edited.
- Block O8 of the plan (A): it runs once ADR 0011 is Accepted, sets
  the global dynamic secret, and no longer removes the bypass or saves
  a ruleset answer; S2 leaves its Unblocks cell.

### Exchange 3

A fresh reviewer read d349b2c..8730ce9 and returned two blocking
findings and one advisory one; this exchange applies them. It is the
last exchange the final pass of stage A allows. Statuses as above.

- F1, `contradiction` (L): ADR 0010, Accepted, cites the Deferred
  decisions row of the required checks, which the commit that accepted
  ADR 0011 removed; with ADR 0011 Proposed, that removal rested on a
  Proposed record. The row is restored under "In how this repository
  is run": the administrator role bypasses the required checks on a
  pull request merge, as read on 2026-10-09, and ADR 0011, proposed,
  takes option (b) of ADR 0010, decision 1; reopened by the acceptance
  of ADR 0011, whose pull request removes the row again, as its
  Consequences now say.
- F2, `contradiction` (A): ADR 0011 drops its two `Supersedes in part`
  lines, so the links match in both directions (ADR 0001, rule 7); the
  pull request that accepts it adds them back with the reverse lines.
- A1 (A): decision 1 of ADR 0011 says that the App token does not hold
  the administrator role once decision 2 is applied, and that until
  then the classic token holds it.

Outcome of stage A: finished on 2026-10-12 with exchange 3, the last
one the closing decision allows, and not by a re-check that raised
nothing; no reviewer read 949df51. Validated for T004 onward: yes, by
that decision. 0 Blocking; the 7 blocking Required rows of round 18
applied (R18-01 to R18-07), 0 declined; 6 blocking findings of the
exchanges applied (4 in exchange 1, 2 in exchange 3), 0 declined; the
`test-detail` rows of rounds 17 and 18 are acceptance items of T008,
T068 and T069; coverage and the three measurements carried from round
8; the index rows carried from round 17, with the rows the exchanges
edited or restored.

### Ship gate round 1

The ship gate read the branch at 556e3f0, after main was merged in,
and returned NO-GO with 19 findings: code-reviewer on opus,
security-auditor on opus, test-engineer on sonnet. One write pass
applies all 19. MF marks a must-fix finding.

- F1 (MF, A): the hint of `RJ-337` opens with `decide: ` in 03 and in
  the 4.4 row, as the hint rule of 04 4.1 asks; no golden held the old
  text.
- F2 (MF, A): the write order of 01 1.6 reads "(create-only, under the
  salvage id's prefix)", since R18-A1 removed the rerun's verification.
- F3 (MF, A): the session checkpoint below: Done gains exchanges 2 and
  3, the merge of main and this gate; Next steps 1 to 4 are marked
  done; dc76fec reads c2ff9b3; its opening no longer says the sections
  above are left as written.
- F4 (MF, A): the outcome line of stage A above, with its numbers.
- F5 (MF, A): decision 2 of ADR 0011 and block O8 keep the host helper
  and the App's private key outside every path mounted into any
  sandbox, and not writable from any sandbox; the App checks gain a
  read from inside a sandbox that finds neither readable.
- F6 (A): decision 2 names the four rows whose trigger is "the token is
  narrowed", two of them among them.
- F7 (A): the trigger of the token narrowing row, and the 05 5.4 row
  that cites it, exclude the App's bot.
- F8 (L): decision 1 states that no standing read looks for a merge
  over a red required check.
- F9 (A, T (T059)): the three goldens of the gap line of 08 8.2.
- F10 (A, T (T103)): the fuzz-run fixtures of 10 10.2 and a
  `workflow_dispatch` case.
- F11 (A, T (T088)): the `gh` floor test of 10 10.2, with its three
  cases.
- F12 (A, T (T005)): an empty walk fails, and a local run with
  `--title` and `--body` reads no network.
- F13 (A, T (T068)): the `--accept-loss` branch of `RJ-338` is plan
  detail of T068, titled "accept-loss names the skipped reasons".
- F14 (A): T068 says how the I17 tests are titled; the hook row of
  12 12.1 says its tests hold the hook's line, and that no test
  compares the prose list with `hookAllow`; block O8 says no command
  reads its answers.
- F15 (A): ADR 0011 counts three final handoffs when D2 was recorded
  and four at review.
- F16 (A): one note in the checkpoint below, in place of edits to
  rounds 9, 11 and 12.
- F17 (A): the same note covers the old title of ADR 0011 in rounds 9
  and 10, whose links point at the renamed file.
- F18 (A): block O6 and T092 run B1 to B6, as 11 11.2 does.
- F19 (A): the openings of rounds 17 and 18 say where the statuses
  are, and round 18's says a checkpoint was added later; R18-03 cites
  round 16, R16-12; the token narrowing row reads "decided on
  2026-10-09; its route was decided on 2026-10-12".

## Session checkpoint (2026-10-12)

Rounds 14 to 18, the final write pass of stage A with its three
exchanges, and the first round of the ship gate were run on this
branch, and the sandbox's GitHub route was read for ADR 0011.

### Decided

- Decided on 2026-10-11: after its owning session ended, a new session
  adopted PR #36.
- R15-01 and R15-02, decided on 2026-10-12: `RJ-338` has no count.
  Every salvage that a changed fingerprint starts records its entry and
  exits 5; a rerun with an equal fingerprint resumes.
- R16-03, decided on 2026-10-12: the fixed hint is amended with two
  clauses. A plain rerun resumes when nothing writes; the first rerun
  after a recovery may exit 5 once more.
- Decided on 2026-10-12: the exit rule of stage A in `lenses.md`. Only
  a contradiction or a behavior gap blocks; a test-plan detail becomes
  an acceptance item of the implementing task.
- Decided on 2026-10-12: R17-06 takes option (a), R17-11 takes
  option (a), and R17-15 and R17-20 are stated limits.
- Decided on 2026-10-12: stage A ends with one final pass, made of a
  writer and a fresh reviewer, with at most 3 change exchanges and no
  round 19. R18-03, R18-06 and R18-07 take their recommended options.
- Deferred on 2026-10-12: three process rules.
  - A convergence-guard tool.
  - Fixing by deletion first.
  - Carrying both, with the three classes, to the house payload.

  Trigger: the first production use of romeu-e-julieta, or a prose
  review that passes 3 rounds without converging.
- Decided on 2026-10-12: a pause. Work already in flight finishes;
  nothing new starts until the spec review and then a plan review end.
  The plan review runs in the same final-pass format. T004 is asked
  first after that.
- Decided on 2026-10-12: ADR 0011 becomes Proposed in this PR.
  - Decision 1 keeps the administrator in the default-branch ruleset's
    bypass list, in `pull_request` bypass mode. The reasons: GitHub
    Actions minutes run out on private repositories, and some changes
    cannot fail CI.
  - Decision 2 replaces the sandbox's classic token with a GitHub App
    installation token, delivered as an sbx dynamic secret (`sbx secret
    set github --command <absolute host helper path> --refresh 55m`).
    The secret is global; per-repository isolation is deferred and
    reopened by a repository that a sandbox must not reach.
  - ADR 0011 becomes Accepted in a later PR, after the App checks.
- Decided on 2026-10-12, for the period while the operator is away:
  - commits and pushes to PR branches need no confirmation, and both
    forbidden-names gates still run before each push;
  - a must-fix finding the orchestrator would decline is decided after
    consulting a fable subagent, and is listed in the PR body as an
    unticked `- [ ] Review: <decision> - <reason>` line;
  - CI is fixed until green, never by weakening a check.

### Done

- Round 14: record 19863f5; write pass 85f6860 (`all`, `hygiene` and
  `generated` all 0).
- Round 15: record be402ca; write pass 591a1aa (`all`, `hygiene` and
  `generated` all 0).
- Round 16: record 3c32d62; write pass 54f674c (all 0).
- Round 17: record 7155491; write pass 0a32191 (all 0).
- Round 18: record c14479f.
- Final write pass: d349b2c (`all`, `hygiene` and `generated` 0). The
  fresh reviewer's verdict on d349b2c was CHANGES, with 4 blocking
  findings and 5 advisory. Exchange 1, 04d48a7 (`all`, `hygiene` and
  `generated` 0), applied all 4 blocking findings and A1 to A4; A5 is a
  stated limit.
- Exchange 2, 8730ce9, and exchange 3, 949df51 (`all`, `hygiene`,
  `generated` and the forbidden-names script 0 for each). Stage A is
  finished.
- main merged in as 556e3f0, with `tools/ci all` exiting 0.
- Ship gate round 1 at 556e3f0: NO-GO, 19 findings, applied by the
  write pass recorded in "Ship gate round 1" above.
- A note on older records: dc76fec, cited in rounds 9, 11 and 12, was
  rewritten as c2ff9b3 by the rebase onto main; rounds 9 and 10 name
  ADR 0011 by its title at the time, "Bind the administrator to the
  required checks and narrow the sandbox's GitHub token", and their
  links point at the renamed file.
- Before every push of this branch, both names gates (`tools/ci
  hygiene` and the forbidden-names script of the sandbox kit
  repository) exited 0.

### Open for the operator

One block, a recommendation per item.

1. The GitHub App checks, after #36 lands. Create the App once in the
   web UI, install it on a throwaway repository, then run the checks
   listed in Next steps item 8. Recommended: yes, in one sitting.
2. The merge of #36, once its ship gate is GO and CI is green.
   Recommended: merge.
3. Review the "Decided while the operator was away" checklist in the PR
   bodies. Recommended: tick each line or reverse it.
4. The merge of the plan-review PR after #36. Recommended: merge once
   it is gated.

### Next steps

1. Done (8730ce9). Exchange 2: ADR 0011 to Proposed, as decided above. The
   token-narrowing note, 10 10.2 items g and h, the 05 5.4 token rows,
   the index rows and plan block O8 follow it. No spec text rests on a
   Proposed record for the current state.
2. Done (949df51). The fresh reviewer re-checks d349b2c..head. At most one more
   exchange (the third). Then stage A is finished.
3. Done (556e3f0). Merge main into the branch with a signed merge
   commit, then run `tools/ci all`.
4. Round 1 done, NO-GO, applied above. The ship gate on the commits
   after c2ff9b3: code-reviewer opus, security-auditor opus,
   test-engineer sonnet.
5. The PR body:
   - the sections Why, What changed, Evidence, Middleware and Lessons;
   - the plain 12.4 approval lines for `spec` and `decisions`;
   - an Accepted risks block;
   - the away-decisions checklist.

   Both names gates run on the body text.
6. Push, CI green on all four legs, then leave draft.
7. The plan review as a PR stacked on #36:
   - a branch from #36's head, a PR that targets main, and a body that
     says "stacked on #36, review only the last N commits";
   - a writer and a fresh reviewer, at most 3 exchanges;
   - its own ship gate, CI green, then ready.
8. Write the brief for a session of the sandbox kit repository: a host
   helper that mints the App installation token, then measurements on
   a throwaway PR. Measure each of these:
   - a signed commit pushed with the token keeps Verified;
   - a PR and a comment made by `gh` with the token show the author as
     the App bot;
   - the agent's merge with a red required check is refused;
   - a ruleset update is refused;
   - the operator's bypass merge works;
   - the push starts the workflow;
   - from inside a sandbox, neither the host helper nor the App's
     private key is readable;
   - which `gh` commands fail with an installation token.
9. After #36 lands, the docs pass:
   - exit codes 130/143 and their precedence;
   - the generated, `.adr-dir` and title rules;
   - the ADR 0008 offline wording;
   - the conditional sentence of 12.1;
   - the two stale hook descriptions at `docs/plan.md:93-94` and
     `docs/spec/10-testing-style.md:543-545`, which PR #41 made stale.
     Leave `docs/plan.md:392` alone.

   Then ask before T004.

### Facts read for the GitHub App route

Read on 2026-10-12 from docs.github.com.

- A fine-grained personal access token is created only in the web UI;
  no API creates one:
  <https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens>
- A GitHub App of a personal account can be installed on selected
  repositories:
  <https://docs.github.com/en/apps/creating-github-apps/about-creating-github-apps/about-creating-github-apps>
- `POST /app/installations/{installation_id}/access_tokens` takes
  `repositories` and `permissions`. Its token expires one hour after
  creation:
  <https://docs.github.com/en/rest/apps/apps#create-an-installation-access-token-for-an-app>
- The installation token serves git over HTTPS when the App has the
  Contents permission:
  <https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/authenticating-as-a-github-app-installation>
- Ruleset bypass modes are `always`, `pull_request` and `exempt`. Actor
  types include Integration (an App), RepositoryRole and User:
  <https://docs.github.com/en/rest/repos/rules>
- GitHub Pro includes 3,000 Actions minutes per month for private
  repositories; public repositories on standard runners are free; usage
  is blocked when the quota runs out only if no payment method is on
  file:
  <https://docs.github.com/en/billing/managing-billing-for-github-actions/about-billing-for-github-actions>
- `sbx secret set` stores a dynamic secret with `--command` and
  `--refresh` (default 55m). Its scopes are global or `--sandbox`, with
  no per-repository scope (read from the sbx help text).
- Not verified: the `[bot]` attribution of App-made PRs and comments;
  `gh` with an installation token in `GH_TOKEN`; how bypass applies to
  a PAT whose owner is in the bypass list.

Branch and PR state is not recorded here: the next session measures it with ~/.sbx-kit/claude-home/bin/handoff_state.py.
