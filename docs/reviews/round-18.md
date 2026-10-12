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
| **R18-01** | index (row 348); plan (Operator blocks, opening); the note (the entry rule) | L2 (L2-r18-1) | regression | Row 348 now says the narrowing is "applied at block O8 of the plan; the token stands until then", which ties the end of the operator's token to the block; the plan's opening, as R17-17 wrote it, ties it to the note's first narrowed entry, and the note says the newest entry's verdict decides. If the entry written at O8 is not narrowed, the texts cannot all hold. | holds as reported: the R17-20 edit of row 348 wrote "applied at block O8"; the plan's opening reads "until the note on the maintainer block records its first narrowed entry" | row 348: "applied by the note's first narrowed entry, planned at block O8; the token stands until that entry". Cost: one clause. Approval line `spec` | `contradiction` | open |
| **R18-02** | the note (read 2, the expected answer); ADR 0011 (decision 2); 04 4.1 (the `SSH_AUTH_SOCK` of sbx calls) | L2 (L2-r18-2) | regression | The R17-14 edit accepts "the setting names no socket" as an expected answer, but sbx forwards the `SSH_AUTH_SOCK` of the calling environment (inferred, not measured), so a host agent inherited by the shell, with the authentication key, passes read 2, and the verdict reads narrowed while ADR 0011, decision 2, "the forwarding of the GitHub authentication SSH key to the sandbox stops", does not hold. | holds as reported: R17-14 proposed "no socket is forwarded"; the pass wrote "the setting names no socket" | read 2 reads the effect: when no setting names a socket, it lists, on the host, the keys behind the `SSH_AUTH_SOCK` of the shell that runs sbx, and expects none, or the signing key alone. Cost: one sentence. Approval line `spec` | `contradiction` | open |
| **R18-03** | 03 3.8 (`salvage[].fingerprint`); 04 4.4 (`RJ-338`, the details); 08 8.5 (steps 2 and 4); 04 4.3; 03 3.11 | L6 (L6-r18-1), L13 (L13-r18-3) (high-signal) | new | The details name the first changed path, but only one sha256 is stored, so no old entry exists to compare with; no page names the call that reads a fresh fingerprint on a rerun with no salvage, or the output that carries the values to romeu; and 03 3.8 credits ignored paths to step 2, which reads only HEAD and status (step 4 reads ignored files). | holds as reported: 03 3.8 defines one plain sha256; 08 8.5 steps, the `julieta salvage` row of 04 4.3 and 03 3.11 do not mention the fingerprint; I17's own case says step 2 "does not see" ignored paths. It predates the round-17 pass | the item in "Decisions for the operator"; either option also writes "steps 2 and 4" in 03 3.8 | `behavior-gap` | open |
| **R18-04** | 01 1.6 (the two `removing` rows with the sandbox present); 04 4.2 (`rm`) | L6 (L6-r18-2) | new | The two rows split on a fresh fingerprint that is equal or differs, but the read needs the sandbox (the daemon heads, each worktree's status), and no row, exit or error covers a read that fails: a stopped sandbox, an unreachable daemon, a julieta that exits non-zero. | holds as reported: the only "changes nothing" rule in 01 1.6 covers `sbx ls --json`; the rows give no third outcome. It predates the pass, which made the rows explicit | one clause in 01 1.6: a fresh fingerprint that cannot be read exits 1 and changes nothing, as a failed `sbx ls --json` does, and the hint names the reason. Cost: one clause. Approval line `spec` | `behavior-gap` | open |
| **R18-05** | 04 4.4 (`RJ-338`); 08 8.5; 09 J3 (step 5); 01 1.6 (the `removing -> salvaging` row); plan T069 | L13 (L13-r18-2) | regression | These texts give a salvage that a changed fingerprint started two endings, `RJ-338` or `RJ-306` with exit 5, while 01 1.6 has more: a failed sandbox half writes `result: incomplete` and exits 1 with no error id, and a sandbox found absent goes to `closed-lost`. | holds as reported: the 01 1.6 row "the sandbox half fails ... \| open \| ...; exit 1" and the absent rows; J3 and T069 were written by the round-17 pass | "the `salvaging` rows decide how it ends": `RJ-338` when complete or accepted, `RJ-306` (exit 5) for a verified incomplete salvage, exit 1 for a failed sandbox half, `closed-lost` for an absent sandbox; 01 1.6 says "the incomplete rows"; T069 follows. Cost: sentence edits on four pages | `contradiction` | open |
| **R18-06** | 04 4.1 (Error ids, the fix-hint shape); 04 4.4 (`RJ-338`); plan T008 | L6 (L6-r18-3) | new | 04 4.1 says a fix hint begins with a command line or with `decide:`; the fixed hint of `RJ-338` begins with prose, and T008 asserts it byte for byte. | holds as reported: 04 4.1 reads "a fix hint begins with a command line the user can run ... or with `decide:` and the choice"; the hint opens "If nothing writes in the sandbox any more". The shape predates the pass (R16-03) | the item in "Decisions for the operator" | `contradiction` | open |
| **R18-07** | 01 1.6 (the paragraph of R17-11); 05 5.4 (the salvage row); index (Decide at the last responsible moment); I17 | L13 (L13-r18-1) | new | The paragraph states a part of I17 that no mechanism guards, the kill route past `RJ-338` among it, with no row in 05 5.4, while the index says such a part is deferred only as a risk accepted by name there. | holds as reported: index, "A part of one is deferred only as a risk accepted by name in 05 5.4, and its row says so"; the 05 5.4 row reads "Salvage proves that nothing the sandbox still holds is lost" | the item in "Decisions for the operator" | `contradiction` | open |

### Advisory

Advisory findings are applied by default (lenses.md, step 4), by a
sentence edit, a stated limit with its trigger, or an acceptance item.
The Defect cell keeps the reviewer's first sentence in short; the
report holds the rest.

| Finding | Section | Lenses | Kind | Defect | Proposed resolution | Status |
|---|---|---|---|---|---|---|
| **R18-A1** | 01 1.6 (the restart row, the recovery paragraph, the new paragraph); 04 4.4 | L13 (L13-r18-4), L6 (L6-r18-5, L6-r18-6) (high-signal) | regression | The restart row says "killed, or failed" where the new paragraph and `RJ-338` say "killed", and a failed sandbox half goes to `open`; the recovery paragraph calls the restart under a new salvage id "resumed", verifying refs created under an earlier id. | the restart row and the paragraphs use one word, "restarted under a new salvage id", and name the failures that leave `salvaging` (a host-side error after the sandbox half) or drop "failed" | open |
| **R18-A2** | 04 4.1; 04 4.2 (`rm`); 08 8.5; 01 1.6 (`salvaging -> removing`) | L13 (L13-r18-5) | regression | The phrase of R17-05 drops "fresh" on four pages, so it reads as a comparison of stored entries. | "a fresh fingerprint that differs from the newest one written with `removing`" | open |
| **R18-A3** | 03 3.8; 04 4.4 | L6 (L6-r18-4), L13 (L13-r18-7) (high-signal) | regression | "The greatest salvage id" relies on an order no page declares; only the generation id is declared a ULID. | "newest" is the last entry of `salvage[]`, or 03 3.8 declares the salvage id a ULID | open |
| **R18-A4** | 03 3.8; plan T068 | L13 (L13-r18-6), L6 (L6-r18-7) (high-signal) | new | Which of step 2's reads the fingerprint takes, for an accepted salvage that skipped a worktree as `worktree-changing`, is not said; any choice is safe. | 03 3.8: "the last read"; T068's `--accept-loss` test asserts it | open |
| **R18-A5** | 09 J3 (step 5) | L13 (L13-r18-8) | regression | The step joins its two branches with a "so" clause and says the rerun after `RJ-338` resumes, where the hint says the first rerun may exit 5 once more. | reorder the branches and cite the hint for the reruns | open |
| **R18-A6** | 04 4.4 (the hint) | L6 (L6-r18-8) | new | The hint's `salvage.excludeIgnored` route ends with `romeu sync` exiting 2 with `removal-pending` after the promotion, which the hint does not say. | part of R18-06 in "Decisions for the operator" | open |
| **R18-A7** | plan T008 | L0 (L0-r18-8), L13 (L13-r18-9) (high-signal) | new | The byte-equal test of the hint names no reference copy and does not say whether the code spans' backticks of the 04 4.4 quote are kept. | T008 names the reference (the parsed 04 4.4 cell, backticks stripped) | open |
| **R18-A8** | plan T068 | L13 (L13-r18-9), L0 (L0-r18-9) (high-signal) | new | The R17-02 item lacks the precondition the R17-01 item has (the fingerprint already differs when the rerun starts), and the kill point is a range of steps that 01 1.6's write order does not list. | add the precondition; name the write `removing -> salvaging` as a step of the fault-injection list | open |
| **R18-A9** | plan T069 and T008 (the Implements fields; parked for B); plan T069 (acceptance); 04 4.1 (the hint-parsing test, parked for B) | L0 (L0-r18-6, L0-r18-7), L6 (L6-r18-9) | new | T069 does not assert that `retire` moves nothing on an `RJ-338` or `RJ-306` exit, or that the move completes after the resume; the Implements fields of T069 and T008 omit the sections their new items test; the 04 4.1 hint-parsing test has no task. | T069 gains the no-move assertion; the Implements fields and the parsing test are parked for B | open |
| **R18-A10** | lenses.md (the stage table, steps 3, 4 and 6, the severity scale, the dated decision) | L0 (L0-r18-1, L0-r18-2, L0-r18-5) | new | An acceptance item that carries a `test-detail` finding is never judged in stage A, since findings on `plan.md` are parked for B; a class set lower than the reporting lens's has no route to the operator; "Required (fix before the next plan task)" does not fit `test-detail`; the dated decision says neither why nor whether it covers rows open before it. | step 6 says the re-audit judges those items as acceptance; step 4 sends a lowered class to the operator; the scale and the dated sentence get one clause each | open |
| **R18-A11** | lenses.md (the classes); 05 5.2 (I17, "write during the salvage"); 08 8.5 (step 2) | L0 (L0-r18-3) | new | The rule that the spec text is not grown for `test-detail` leaves I17's case worded so that one write reads as enough, which step 2 absorbs; the correct construction is only in T068. | a stated limit in lenses.md: a spec test cell may get a wording fix that changes no behavior, reopened by the plan review of stage B | open |
| **R18-A12** | the note (the entry rule, limit 5, the fourth part); 10 10.2 (item h) | L2 (L2-r18-5) | unresolved | A narrowed entry after a not narrowed one is no event, and item h's answers stay in the first narrowed entry even when a later or replaced token is the one tried. | limit 5 names the re-narrowing; limit 4 names item h among what a replaced token does not re-run | open |
| **R18-A13** | the note (the limits, closing line); plan O8 | L2 (L2-r18-3) | new | Limits 4 to 6 are reopened by the sitting of block O8, the sitting that writes the first entry and so starts them; after it they have no live trigger. | a trigger after O8: the first of a re-created development sandbox, a replaced token, a release-candidate tag, checkpoint C7 | open |
| **R18-A14** | the note (the entry rule); 05 5.4 (the token row); round-17.md (R17-02, the write pass) | L2 (L2-r18-4), L0 (L0-r18-4) | new | The note says the 05 5.4 token row describes both states, but the row is in the present tense and still says "applied at the next maintainer sitting"; separately, R17-02's phrase edit in I17 was dropped without a record. | the note says the 05 5.4 row describes the state before the narrowing and points to row 348 for the state after; the write pass of round 18 records that the I17 phrase of R17-02 went to T068 with the rest | open |

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

What is left before T004:

- the answers to the three items in "Decisions for the operator";
- the write pass of this page's rows, with the `spec` approval line;
- a targeted re-audit of that pass, round 19, by L6 and L13 on the
  salvage text, L2 on the maintainer text, and L0 on lenses.md and the
  plan items, reading only the sections it changes.

The `.githooks/pre-push` row of 12 12.1 is edited by a separate pull
request, #41; pull request #36 does not edit that row, and no write
pass of this review touches it.
