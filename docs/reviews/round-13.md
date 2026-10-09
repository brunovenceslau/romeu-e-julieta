# Spec review round 13: the targeted re-audit of round 12

Reader: reviewers checking that every finding of round 13 was handled,
and maintainers tracing why the spec reads as it does. Type: reference
(history). The current state is [docs/spec.md](../spec.md); round 12 is
in [round-12.md](round-12.md); the lenses are defined in
[lenses.md](lenses.md).

Round 13 is step 6 of "How stage A runs, and when the spec counts as
validated" in lenses.md: the targeted re-audit of the round-12 write
pass. Each lens read the sections that pass changed, checked the
round-12 findings it had raised against the new text, and reported what
was left, what regressed and what the new text added. lenses.md is
cited at the baseline commit.

The baseline is commit 4c2b11d, the head of pull request #36 (the
round-9 to round-12 write passes, branch `docs/review-round-9`), read
from a detached clone that nothing wrote to while the reviewers ran. It
is not a commit of the default branch, as step 1 of lenses.md asks for
a full round: the write passes are validated before they merge, so the
re-audit reads the pull request's head. The delta read is
4f1bd1b..4c2b11d.

Every item below is a finding of a reviewer or a proposal of the
consolidation. No item needs the operator, so this page has no
"Decisions for the operator" block.

Status, as in rounds 6 to 12: **A** = applied as proposed; **M** =
applied in a smaller or different form than proposed, as its row
states; **L** = stated as a limit, no mechanism added. In this draft
each status is the one proposed; the write pass records the one it
applies.

The reports are kept in `round-13/reports/<lens>.json`, as the
reviewers wrote them.

## How the round was run

Six reviewers, one per lens that raised a round-12 finding the write
pass resolved: L0, L2, L3, L6, L12 and L13, each in a fresh context,
read-only, in parallel on 4c2b11d. L2 and L13 ran on `opus`, the rest
on `sonnet`. Each lens ran once, with one reader: a re-audit carries no
noise probe, since the probe measures a lens set on a full read, and
round 8 recorded it. The verdicts of L1, L4, L5, L7 to L11 and L14 to
L16 carry forward from round 12. Each reviewer wrote one JSON report
with `prior_checked`, `prior_resolved_as_stated` and its findings, each
of kind `unresolved` (a round-12 finding the pass did not resolve as
its row states), `regression` (the pass broke what held) or `new` (a
defect in text the pass wrote or a seam it opened). No report is
missing.

The reports hold 13 findings: 0 Blocking, 1 Required, 12 Advisory; 13
new, 0 unresolved, 0 regression. One consolidator merged them by
section and defect, kept every lens on its row, and checked the
Required finding against the baseline text. It holds as reported. The
result is 1 Required row, raised by four lenses, and 6 Advisory rows;
nothing is downgraded, dropped or parked.

### Per lens

`prior_checked` is the number of round-12 findings the lens re-read
against the new text, and `prior_resolved_as_stated` how many of them
read as their round-12 row states. Findings parked for stage B in round
12 (P1, P6) were not re-read as results. Severities in this table are
the reviewers'; the consolidation's are in the findings below.

| Lens | Model | prior_checked | prior_resolved_as_stated | Findings (B / R / A) | new | unresolved | regression |
|---|---|---|---|---|---|---|---|
| L0 | sonnet | 2 | 2 | 1 (0 / 0 / 1) | 1 | 0 | 0 |
| L2 | opus | 4 | 4 | 4 (0 / 0 / 4) | 4 | 0 | 0 |
| L3 | sonnet | 2 | 1 | 2 (0 / 0 / 2) | 2 | 0 | 0 |
| L6 | sonnet | 3 | 3 | 3 (0 / 0 / 3) | 3 | 0 | 0 |
| L12 | sonnet | 0 | 0 | 0 (0 / 0 / 0) | 0 | 0 | 0 |
| L13 | opus | 5 | 5 | 3 (0 / 1 / 2) | 3 | 0 | 0 |
| all | | 16 | 15 | 13 (0 / 1 / 12) | 13 | 0 | 0 |

The one prior finding not counted as resolved is L3-R12-2, which round
12 parked in P1 for stage B; L3 reports it as parked, not as
`unresolved`, and the consolidation reads it the same way. L12's only
round-12 finding is in P1 too, so it re-read none, and it reports no
finding. The two Required clusters of round 12 read as resolved in
every report that re-read them: R12-01 in L13's, R12-02 in L2's.

## Findings and resolutions

One row per cluster. The Required row comes first, then the Advisory
rows. Kind is the cluster's: `unresolved` when any member is, else
`regression` when any member is, else `new`. Sections are cited as the
spec cites itself.

### Required

| Cluster | Section | Lenses | Kind | Defect | Verification | Resolution | Status |
|---|---|---|---|---|---|---|---|
| **R13-01** | 04 4.4 (`RJ-338` `fingerprint-unstable`); 03 3.8 (`salvage[].fingerprint`, `salvage[].cause`); 01 1.6 (the `removing` row); 08 8.5; 05 5.2 (I17) | L13 (L13-R13-1, Required), L0 (L0-r13-1), L3 (L3-R13-1), L6 (L6-r13-1) (high-signal: four lenses) | new | The R12-01 write states the rule as "the third in a row" in 04 4.4, 01 1.6, 08 8.5 and I17, and as "the limit on such salvages in a row" in 03 3.8, and no text says what a fourth changed-fingerprint salvage does. Under the literal reading the fourth goes on to `sbx env rm`, so the guard is a one-time pause; under the cap reading the recovery the hint names, "then rerun", can take two reruns, and always does after an edit of `salvage.excludeIgnored`, which is an input of the fingerprint. I17 stops at the third rerun, so a test passes both readings. | holds as reported: 04 4.4 fires `RJ-338` "when the salvage a changed fingerprint started is the third in a row"; 01 1.6 ends the `removing` row with "the third such salvage in a row ends at `removing` with exit 5"; 08 8.5 says "the third salvage in a row that a changed fingerprint starts exits 5"; 03 3.8 calls it "the limit on such salvages in a row" and reads the count from "the newest entries of the generation with `fingerprint-changed`", a count that grows past 3 and never resets within a `removing` generation; the fingerprint of 03 3.8 holds "ignored paths, minus `salvage.excludeIgnored`", so an edit of that list changes it; I17's case ends at "once the writer stops the next rerun resumes at `sbx env rm`". Severity: T008 builds the error table from 04 4.4 and the salvage tasks build the rerun from 03 3.8, so the gap reaches the next task that implements the cited sections, as R12-01 did; three lenses graded it Advisory and L2 found no host-safety path that depends on it, since each salvage completes before `sbx env rm`, and the scale's Required is about the task that implements the section, not about harm | one home: `RJ-338` in 04 4.4 fires on "the third or a later salvage in a row that a changed fingerprint started", counted as 03 3.8 already reads it; the hint adds that the first rerun after a recovery salvages once more and exits 5 again when the fresh fingerprint still differs (always so after an edit of `salvage.excludeIgnored`), and that the rerun after it resumes at `sbx env rm`; 03 3.8 links `RJ-338` in place of "the limit"; 01 1.6, 08 8.5 and the `rm` row of 04 4.2 cite 04 4.4 without restating the number; I17 gains a fourth rerun with the writer still running, which salvages, records its entry and exits 5 again (the removal that turns it red: comparing the count with equality), and a rerun after an `excludeIgnored` edit, which exits 5 once and then resumes. Approval line `spec` | proposed A |

### Advisory

Advisory findings are applied by default (lenses.md, step 4). The
Defect cell keeps the reviewer's first sentence in short; the report
holds the rest. Each was checked against the quoted text at 4c2b11d;
the platform and toolchain behavior L2-r13-1 and L2-r13-2 rest on is
from the reviewer's reading, not measured.

| Finding | Section | Lenses | Kind | Defect | Resolution | Status |
|---|---|---|---|---|---|---|
| **R13-02** | 10 10.2 (maintainer block, item d, the fifth try); 05 5.4 (the approval-line row; the token row) | L2 (L2-r13-1), L13 (L13-R13-2) | new | Two defects in one sentence. The fifth try posts a commit status "with the token" after the narrowing, but the narrowed token of ADR 0011, decision 2, holds no Commit statuses permission, so the try would record the token's refusal, not the ruleset's answer, and never tries the side-branch status path 05 5.4 accepts; it also leaves the throwaway pull request and side branch in place. And 05 5.4 names two different tries "the fifth try": the approval-line row's, which runs, and the token row's, which "is not made". | item d's try becomes three steps: (1) from a workflow pushed on a side branch, with `statuses: write` and `checks: write`, post a commit status and a check run under a required check's name on the throwaway pull request's head, record which the ruleset counts, and save the API answers under Evidence; (2) post a status directly with the narrowed token and record whether the token refuses it; (3) close the pull request and delete the side branch and its workflow. The try is named by what it does, "the required-name try of item d", in 10 10.2 and the approval-line row of 05 5.4, and the token row reads "the try that would have measured it is not made". This refines how D1 of round 12, option (a), is carried out and changes no decision. Approval line `spec` | proposed A |
| **R13-03** | index (Deferred decisions, In how this repository is run: the token-narrowing row; the rows whose trigger is "the token is narrowed": the `checksums.txt` signature, the create-only host ref, the rulesets check, the signed approval record); 05 5.4 (the token row) | L3 (L3-R13-2), L2 (L2-r13-4) (high-signal) | new | The reshaped token-narrowing row's "Until then" cell describes only the token before the narrowing, and its trigger still carries events that applied only before it; four rows fire on "the token is narrowed", an event no evidence item records; and no text lists what the narrowed token can still do (merge through the API, push workflows, and through them post a check under a required name). | the event becomes observable: the four tries of item d repeated with the narrowed token, saved under Evidence, and the four rows cite that; the "Until then" cell holds both states, before the narrowing (the operator's own token) and after it (the narrowed token of ADR 0011, decision 2, which can merge, push branches and workflows, and so post a check under a required name, 05 5.4, the approval-line row); the triggers that applied only before the narrowing are dropped, or kept with the reason they still apply. Approval line `spec` | proposed A |
| **R13-04** | plan T005 (requirement (c)); 10 10.2 (Hygiene: a tracked `go.work`, `go.work.sum` or `vendor/`) | L2 (L2-r13-3), L6 (L6-r13-3) (high-signal) | new | Requirement (c) refuses a head that "adds" a tracked `go.work`, `go.work.sum` or `vendor/`, while the hygiene rule it applies from the base refuses one that is tracked at all, so a head that edits a `go.work` already tracked on the base passes; and its fixtures name no removal that turns each red. | (c) reads "holds" for "adds", as the hygiene rule does; a fixture for an edited `go.work` already tracked on the base joins the one per file, and each fixture names the removal (the path pattern of its file) that turns it red | proposed A |
| L2-r13-2 | plan T005 (requirement (b)); 05 5.3 (the invariant of the `pr` job) | L2 | new | The test reads a package path whose first element has no dot as the standard library, the rule R12-06 wrote; a module path needs no dot under a local `replace`, so a repository package under such a path skips the `approvals` check, while `go list` reports the fact in each package's `Standard` field. | (b) reads the standard library from `go list -deps -test`'s `Standard` field and matches every other package to an `approvals` glob by its `Dir`; a fixture with a dotless module path under a local `replace` turns the test red. This replaces the dot rule of R12-06 | proposed A |
| L6-r13-2 | plan T005 (requirement (e)); 05 5.3 | L6 | new | Requirement (e), one surface set from every committed `ask-first.yaml` and `grants.yaml` for `pr` and the generator, is "held by a test, or both use one reader", and unlike (b) and the 12 12.4 fixtures it names no fixture and no removal that turns it red. | (e) names the fixture pair for the test (two files with the same ids and different globs fail; identical files pass) and the removal of the glob comparison that turns the first red; when T005 takes one reader instead, the test asserts that both call it | proposed A |
| L13-R13-3 | 03 3.8 (`salvage[].cause`); 04 4.2 (`romeu salvage`, and `rm`, `recreate` and `retire` through it) | L13 | new | The value `first` covers every salvage a changed fingerprint did not start, a second standalone `romeu salvage` of a generation and a salvage started again from `salvaging` included, so the name says what the rule does not. | before the state format lands, `first` is renamed `requested` and defined as any salvage a changed fingerprint did not start; the 03 3.8 example follows. Approval line `spec` | proposed A |

## Exit criteria of stage A

The five criteria of lenses.md, "How stage A runs, and when the spec
counts as validated", judged on 4c2b11d:

1. **No Blocking open; every Required fixed or declined with its
   decision and date: not met.** 0 Blocking. 1 Required row is open
   (R13-01), new, not fixed and not declined. The 2 Required clusters
   of round 12 (R12-01, R12-02) read as resolved in every report that
   re-read them.
2. **Coverage: carried from round 8, as in rounds 9 to 12.** A
   re-audit reads only the changed sections, and its reports carry no
   complete coverage table for the whole set (L2 and L13 have one), so
   the round-8 measurement stands: every page with a finding or an
   explicit "nothing for this lens" from at least two lenses; the
   criteria and journeys unproven beyond that
   ([round-8.md](round-8.md#coverage)).
3. **The open questions and deferred decisions of the index have a
   valid shape: met for 101 of 101 rows as measured.** 101 rows: 24
   open questions and 77 deferred decisions (36 in the product, 11
   with the runtime ledger, 30 in how this repository is run), the same
   count as round 12. Every row has all its cells filled, and no two
   rows share a first cell, measured by a script over the tables at
   4c2b11d. The token-narrowing row's first cell now names what is
   still open, the later review of the sandbox's GitHub access, which
   L13 confirms (L13-R12-3). Its "Until then" cell holds at 4c2b11d,
   since the narrowing is not applied yet; R13-03 makes it hold after
   the narrowing too and makes the event of four rows observable.
   Duplicates in meaning were not measured beyond the reviewers'
   reading.
4. **The three measurements: carried from round 8.** The `listed` to
   `own` ratio per lens, the clusters only L0 raised and the overlap of
   the L2 and L13 noise-probe pairs are in
   [round-8.md](round-8.md#measurements). This round ran no noise probe
   and its reports record no origin per finding. L0 raised one finding,
   and it joined R13-01, which three other lenses raised, so no finding
   is L0's alone. L2's lens critique repeats the request of rounds 11
   and 12 for a viewpoint on what the CI platform trusts, and adds that
   a lens should ask whether the actor in a measurement holds the same
   permissions as the actor in the accepted risk (R13-02 is that
   case). That is input for the lens rewrite before stage B.
5. **The targeted re-audit raises no new Required finding: not met.**
   It raised 1 Required row, of kind `new`: R13-01, from the text the
   R12-01 row wrote in 04 4.4 and 03 3.8. The 1 Required finding
   reported holds; none was downgraded or dropped. The count fell from
   3 new Required clusters in round 11 and 2 in round 12 to 1.

Validated for T004 onward: no. 0 Blocking; 1 Required open (1 new), 0
declined; coverage and the three measurements carried from round 8;
101 of 101 index rows with a valid shape as measured; the re-audit
raised 1 Required row (R13-01, raised by four lenses).

R13-01 does not cite the `sequences` step of 10 10.2, the section T004
implements; the criteria are judged for the spec as a whole, so the
answer stays no until they hold.

What is left before T004:

- the write pass of this page's rows, with the `spec` approval line;
  no row needs a decision;
- a targeted re-audit of that pass, round 14, by the lenses that raised
  round-13 findings, L0, L2, L3, L6 and L13, reading only the sections
  it changes; the verdicts of the other lenses, L12's clean verdict of
  this round included, are carried forward.
