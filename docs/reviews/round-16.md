# Spec review round 16: the targeted re-audit of round 15

Reader: reviewers checking that every finding of round 16 was handled,
and maintainers tracing why the spec reads as it does. Type: reference
(history). The current state is [docs/spec.md](../spec.md); round 15 is
in [round-15.md](round-15.md); the lenses are defined in
[lenses.md](lenses.md).

Round 16 is step 6 of "How stage A runs, and when the spec counts as
validated" in lenses.md: the targeted re-audit of the round-15 write
pass, which applied the decision of 2026-10-12 on `RJ-338` and the
round-15 rows. lenses.md is cited at the baseline commit.

The baseline is commit 591a1aa, the head of pull request #36 (the
round-9 to round-15 write passes, branch `docs/review-round-9`), read
from a detached clone that nothing wrote to while the reviewers ran.
The delta read is be402ca..591a1aa, over the index, the pages under
`spec/`, `plan.md` and the note on the maintainer block.

Status, as in rounds 6 to 15: **A** = applied as proposed; **M** =
applied in a smaller or different form than proposed, as the write
pass states; **L** = stated as a limit, no mechanism added. The round
was recorded before its write pass, with each resolution a proposal;
each row's status is the one the pass applied, in "Write pass
(2026-10-12)" below. One row, R16-03, would change text that the
decision of 2026-10-12 fixed, so it went to "Decisions for the
operator" below, and its decision is recorded there.

The reports are kept in `round-16/reports/<lens>.json`, as the
reviewers wrote them.

## How the round was run

Four reviewers, each reading only what the pass changed in its scope:
L6 and L13 on 04 4.4, 03 3.8, 01 1.6, 08 8.5, 09 J3, I17 and the new
line of T068; L0 and L2 on the note on the maintainer block, 10 10.2
(the opening rule of the maintainer block, the "To do" lead-in, items g
and h), the token rows of the index and of 05 5.4, and block O8 of the
plan. L0, L2 and L13 ran on `opus`, L6 on `sonnet`. L3 did not run:
its round-15 findings were all resolved by the pass or stated as
limits, and the scope of this round was set to the four lenses above.
Each lens ran once, with one reader, in fresh contexts, read-only, in
parallel. The verdicts of the lenses that did not run carry forward:
L3 from round 15, the others from round 13.

The brief was delivered in two parts, as in rounds 14 and 15. No
report is missing. The reports hold 20 findings: 0 Blocking, 2
Required, 18 Advisory; 14 new, 4 unresolved, 2 regression. The
consolidation merged them by section and defect, kept every lens on
its row, and checked each Required finding against the baseline text.
The result is 2 Required rows and 11 Advisory rows; nothing is
downgraded, dropped or parked.

### Per lens

Severities in this table are the reviewers'. Round-15 findings outside
a lens's scope this round count as checked and not resolved in L6's
report (L6-r15-3 to L6-r15-5), and as not re-read in L13's.

| Lens | Model | prior_checked | prior_resolved_as_stated | Findings (B / R / A) | new | unresolved | regression | listed / own |
|---|---|---|---|---|---|---|---|---|
| L0 | opus | 6 | 5 | 6 (0 / 0 / 6) | 5 | 1 | 0 | 0 / 6 |
| L2 | opus | 5 | 4 | 5 (0 / 0 / 5) | 4 | 1 | 0 | 0 / 5 |
| L6 | sonnet | 5 | 3 | 3 (0 / 1 / 2) | 2 | 1 | 0 | 0 / 3 |
| L13 | opus | 4 | 4 | 6 (0 / 1 / 5) | 3 | 1 | 2 | 2 / 4 |
| all | | 20 | 16 | 20 (0 / 2 / 18) | 14 | 4 | 2 | 2 / 18 |

The decision of 2026-10-12 reads as stated once and consistently in
the pages the pass rewrote, in L6's and L13's reports, and no text of
the old count or of `salvage[].cause` is left, by their searches of the
spec, the plan, the note and the decision records. R15-04 is the one
round-15 row that L0 and L2 both read as resolved only in part: the
pass wrote the host read of the forwarded socket and not the read of
the forwarding setting or of the account's SSH keys page that its row
proposed. Its status in round-15.md is corrected from A to M.

## Decisions for the operator

| Item | Question | Options | Recommendation |
|---|---|---|---|
| R16-03 | The hint of `RJ-338` is the one fixed text decided on 2026-10-12. L13 finds it no longer says that a plain rerun resumes when nothing is writing, nor that the first rerun after a recovery may exit 5 once more, as the text before the decision did. | (a) amend the decided text with one clause: "a rerun with nothing written since this salvage resumes at `sbx env rm`; the first rerun after a recovery may salvage and exit 5 once more"; (b) keep the decided text as it is | (a): both clauses follow from the rule, and the second is exactly what the I17 `excludeIgnored` case asserts, so the hint then matches its own test |

Decided on 2026-10-12: R16-03 takes option (a). The fixed hint of
`RJ-338`, decided earlier the same day, is amended with two clauses: a
plain rerun resumes at `sbx env rm` when nothing is writing; and the
first rerun after a recovery (stopping the process, or editing
`salvage.excludeIgnored`) may exit 5 once more. The hint stays one
fixed text, so R15-02 stays closed.

## Findings and resolutions

One row per cluster. The Required rows come first, then the Advisory
rows. Kind is the cluster's: `unresolved` when any member is, else
`regression` when any member is, else `new`. Sections are cited as the
spec cites itself.

### Required

| Cluster | Section | Lenses | Kind | Defect | Verification | Proposed resolution | Status |
|---|---|---|---|---|---|---|---|
| **R16-01** | 04 4.1 (the exit-code row); 04 4.2 (the `rm` row); 08 8.5 (the paragraph after the recovery table) | L13 (L13-r16-1, Required) | regression | Three sentences the round-15 pass did not rewrite still describe `RJ-338` as a fingerprint that "keeps changing", the rule under the old count, while 04 4.4 now exits 5 on the first changed fingerprint. A search for the old count's words could not find them, since they never name a count. | holds as reported: 04 4.1 reads "a fingerprint that keeps changing during a removal"; the `rm` row of 04 4.2 reads "A rerun on a `removing` generation whose fingerprint keeps changing exits 5 with `fingerprint-unstable` (4.4)"; 08 8.5 reads "on a `removing` generation whose worktrees keep changing is not a skipped reason". T008 writes the exit codes from 04 4.1, and T068 implements 04 4.2 and 08 8.5 | each says "changed": "a fingerprint that changed after a removal began" (04 4.1), "whose fingerprint changed since the last salvage exits 5" (04 4.2), "whose worktrees changed since the last salvage" (08 8.5). The write pass searches the spec for "keeps changing" and "keep changing" and records the result. Cost: three phrases. Approval line `spec` | A |
| **R16-02** | 04 4.4 (`RJ-338`, `RJ-306`); 08 8.5 (step 2, the recheck; the paragraph after the recovery table); 01 1.6 (the `salvaging -> removing` row; the `salvaging` restart row); 05 5.2 (I17) | L6 (L6-r16-1, Required), L13 (L13-r16-3, L13-r16-4, Advisory), L6 (L6-r16-2, Advisory) (high-signal: two lenses) | unresolved | No text says which error ends a rerun's salvage on a `removing` generation when a worktree changes during that salvage: the recheck of 08 8.5 step 2 skips it as `worktree-changing`, which makes the salvage incomplete (`RJ-306`, back to `open`, and `--accept-loss=worktree-changing` accepted), while 08 8.5 says changing worktrees on a `removing` generation are "not a skipped reason" and `RJ-338` says the salvage "ends at `removing`". The `salvaging -> removing` row of 01 1.6 says "before `sbx env rm`" for every command, and a changed-fingerprint salvage killed mid-run restarts from `salvaging` and goes on to `sbx env rm`, which no text states. The I17 writer cases assert "exits 5", not the error id or the state. | holds in part: the table of 01 1.6 already gives one path, since an incomplete salvage without `--accept-loss` takes `salvaging -> open` with exit 5 and only a complete or accepted one writes `removing`; so the text decides the case, but 08 8.5's sentence and `RJ-338`'s "ends at `removing`" read against it, and no test tells `RJ-306` from `RJ-338`. The interaction predates the round-15 pass (L6 finds the 08 8.5 sentence in ed64a03; L13 finds the same rows at c64c468 and 85f6860); the pass removed `cause`, the one stored marker of the path. Severity as L6 graded it: T068 and T008 | `RJ-338` applies to a salvage a changed fingerprint started that completes, or whose skipped reasons `--accept-loss` names; one that is incomplete without them follows the incomplete rows of 01 1.6, `RJ-306`, back to `open`, as any salvage does. 08 8.5's sentence says so in place of "not a skipped reason". The `salvaging -> removing` row of 01 1.6 cites `RJ-338` for a salvage a rerun on a `removing` generation started. A salvage restarted from `salvaging` is the command's own salvage and goes on to `sbx env rm`, said once in 04 4.4. I17: the writer cases assert `RJ-338` and the state `removing`, and the resumed rerun writes no new entry; one case writes during the salvage and asserts `RJ-306` and `open` (returning `RJ-338` turns it red); one kills a changed-fingerprint salvage and asserts that the rerun goes on to `sbx env rm`. Cost: three sentences and three cases. Approval line `spec` | A |

### Advisory

Advisory findings are applied by default (lenses.md, step 4). The
Defect cell keeps the reviewer's first sentence in short; the report
holds the rest. That GitHub lets one key be registered as both a
signing and an authentication key, and that a host keychain reload can
add a key back to an agent, are the reviewers' inference, not read from
a primary source this round.

| Finding | Section | Lenses | Kind | Defect | Proposed resolution | Status |
|---|---|---|---|---|---|---|
| **R16-03** | 04 4.4 (the `RJ-338` hint) | L13 (L13-r16-2) | regression | The fixed hint no longer says that a plain rerun resumes when nothing is writing, nor that the first rerun after a recovery may exit 5 once more. | the item in "Decisions for the operator"; applied only as decided | A |
| **R16-04** | the note on the maintainer block (The token narrowing, read 2); 10 10.2 (item g); 06 6.4 | L2 (L2-r16-1), L0 (L0-r16-2) (high-signal) | unresolved | Read 2 checks the keys of the forwarded socket by their role in the agent, not by how the GitHub account registers them, and neither the forwarding setting nor the account's SSH keys page is read, as R15-04 proposed; a signing key also registered as an authentication key would pass. | read 2 adds the forwarding setting that names the socket, and the account's SSH keys page, where the signing key's fingerprint is listed only under signing keys, the rule 06 6.4 states for romeu's sandboxes, cited. Approval line `spec` | A |
| **R16-05** | 10 10.2 (item h); the note on the maintainer block (the entry format); plan (block O8, Pass) | L0 (L0-r16-1), L2 (L2-r16-5) (high-signal) | new | Item h saves its answers "under the entry of item g", but an entry is a heading and the three reads, with no place for them; which entry is meant once there are several is unsaid; whether a not narrowed verdict stops item h is unsaid; O8's pass reads only the verdict. | the entry format gains an optional fourth part, the required-name try, in the first narrowed entry; item h runs only after that entry; O8's pass names both. Approval line `spec` | A |
| **R16-06** | the note on the maintainer block (the entry rule); index (rows whose trigger is "the token is narrowed") | L0 (L0-r16-4), L2 (L2-r16-2) (high-signal) | new | The return rule names "the token rows of the index and of 05 5.4" without a list; "the rows it fired stay fired" has two possible referents; two entries of one date have no order; and a not narrowed entry after a narrowed one reopens nothing decided on the narrowed premise. | the rule names the rows (the token narrowing row of the index and the token row of 05 5.4); "the rows the first narrowed entry fired"; an entry heading carries a time when two share a date; a not narrowed entry after a narrowed one is an event of the four rows. Approval line `spec` | A |
| **R16-07** | plan (block O8); 10 10.2 (item a); index (S2) | L0 (L0-r16-3) | new | Decision 1 of ADR 0011, removing the administrator's bypass, which S2's evidence waits on, appears only in O8's Recommendation column, in no block's "What the maintainer does", with no pass and no home for its evidence. | O8's "What the maintainer does" names decision 1, its pass is the API answer that shows the default-branch ruleset with no bypass actor (item a), and that answer goes in the note's entry. Approval line `spec` | M |
| **R16-08** | the note on the maintainer block (read 3, the tag try); index (the token-narrowing row); plan (O8, T088) | L0 (L0-r16-5) | new | "A later read leaves it out" can mean a read after `release.yml` exists or any read after the first, and if O8 runs after T088 the tag try is never made while the index row says the entry reads it. | "a read made once `release.yml` exists leaves it out"; the index row says the tag answer is read only when the entry ran before `release.yml`. Approval line `spec` | A |
| **R16-09** | index (the four rows whose trigger is "the token is narrowed"); plan (the opening of the operator blocks) | L0 (L0-r16-6) | new | The four rows link to 10 10.2 item g rather than to the note's anchor; ADR 0011 says the rows fire when the narrowing is applied while the spec fires them at the first narrowed entry; the plan's opening still says the sandbox holds the operator's token. | the rows link to the note's anchor; item g says the first narrowed entry is the record that the narrowing is applied, as ADR 0011, decision 2, words it, so the two agree; the plan's sentence reads "until block O8". Approval line `spec` | A |
| **R16-10** | index (the token-narrowing row, after the narrowing); the note on the maintainer block (the two limits) | L2 (L2-r16-4) | new | The R15-09 limit is stated only in the note, and the row that reopens it still says "limited to this repository" with no qualifier. | the row adds one clause: keys, webhooks and collaborators added before the narrowing are not read (the note, the two limits). Approval line `spec` | A |
| **R16-11** | the note on the maintainer block ("one for each later read"; read 2); plan (block O8) | L2 (L2-r16-3) | new | The narrowed state is read once with no cadence for later reads, and O8 does not say how the forwarding stops, while the maintainer's own pushes in O3 and O6 use the host's agent. | O8 forwards a dedicated signing socket, as 06 6.4 requires for romeu's sandboxes, in place of the host's agent; a later read is due at each sitting of O3, O6 and O7. Approval line `spec` | M |
| **R16-12** | 04 4.4 (`RJ-338`); 05 5.2 (I17); plan T068 | L6 (L6-r16-3), L13 (L13-r16-5) (high-signal) | new | No test is named for the details naming the first changed path; the hint is described by clauses, not quoted; T068's line that makes each `E:` case a titled test has no check; and T068's first acceptance bullet copies part of I17's exit-5 list, missing two cases. | I17 asserts the details' first path; 04 4.4 quotes the hint once decided (R16-03); T068's first bullet cites I17 in place of the copy; the titled-test check is left to T068's review, stated as a limit | M |
| **R16-13** | 04 4.4; 01 1.6; 03 3.8 (`salvage[].fingerprint`); 05 5.2 (I17) | L13 (L13-r16-6) | new | The stored fingerprint has three names ("the state's newest salvage entry", "the state's salvage entry", "the one written with `removing`"), and four I17 cases say "the resumed `rm` salvage again" for a command that salvages and does not resume. | one name, "the `fingerprint` written with `removing`", in the four places; the four cases say "the rerun of `rm` salvages again". Approval line `spec` | M |

## Exit criteria of stage A

The five criteria of lenses.md, judged on 591a1aa:

1. **No Blocking open; every Required fixed or declined with its
   decision and date: not met.** 0 Blocking. 2 Required rows are open
   (R16-01, a regression, and R16-02, unresolved), not fixed and not
   declined. R15-01 and R15-02 read as closed by the decision of
   2026-10-12 in every report that re-read them.
2. **Coverage: carried from round 8, as in rounds 9 to 15**
   ([round-8.md](round-8.md#coverage)).
3. **The open questions and deferred decisions of the index have a
   valid shape: met for 101 of 101 rows as measured.** 24 open
   questions and 77 deferred decisions (36, 11 and 30), the same count
   as round 15; every cell filled and no first cell repeated, measured
   by a script over the tables at 591a1aa.
4. **The three measurements: carried from round 8**
   ([round-8.md](round-8.md#measurements)). This round ran no noise
   probe; its reports record an origin per finding (2 listed, 18 own).
   L0 raised six findings; R16-07, R16-08 and R16-09 are L0's alone,
   all on the plan's operator blocks and their seams with the note,
   which no lens in this round's scope owns.
5. **The targeted re-audit raises no new Required finding: not met.**
   It raised 2 Required rows: R16-01, of kind `regression`, in three
   sentences the round-15 pass left with the old rule's words, and
   R16-02, of kind `unresolved`, an interaction that predates the
   pass and that the removal of `cause` left without a stored marker.

Validated for T004 onward: no. 0 Blocking; 2 Required open (R16-01,
raised by L13, and R16-02, raised by two lenses), 0 declined; coverage
and the three measurements carried from round 8; 101 of 101 index rows
with a valid shape as measured; the re-audit raised 2 Required rows.

What is left before T004:

- the answer to the one item in "Decisions for the operator": given
  on 2026-10-12, recorded under that table;
- the write pass of this page's rows, with the `spec` approval line:
  done, in "Write pass (2026-10-12)" below;
- a targeted re-audit of that pass, round 17, by L6 and L13 on the
  salvage text and L0 and L2 on the maintainer block, reading only the
  sections it changes.

## Write pass (2026-10-12)

The pass ran on branch `docs/review-round-9`, the branch of pull
request #36, at 3c32d62, in its worktree. Each row's status above is
the one applied. No decision record is edited, no accepted risk of 05
5.4 changes, and nothing under `tools/**`, `.github/**` or
`.githooks/**` changed; the `.githooks/pre-push` row of 12 12.1 is not
touched.

The salvage text:

- R16-01 (A): 04 4.1 reads "a fingerprint that changed after a removal
  began", the `rm` row of 04 4.2 "whose fingerprint changed since the
  last salvage", and 08 8.5 "whose worktrees changed since the last
  salvage". The search for the old count's phrasings, over the index,
  the pages under `spec/`, `plan.md`, the notes and the decision
  records:

  ```
  grep -rnEi "keeps? changing|kept changing|keeps? moving|changing again|in a row|changes? repeatedly|still changing" \
    docs/spec.md docs/spec/ docs/plan.md docs/notes/ docs/adr/
  ```

  It finds three lines, and each stays: `RJ-338` in 04 4.4, "in place
  of a limit of three such salvages in a row", which records the
  decision; I17 in 05 5.2, "a writer that keeps changing one ignored
  path", which describes the test's writer and not the rule; and 10
  10.7, "placed in a row", a row of a table. No sentence states the old
  count.
- R16-02 (A): `RJ-338` in 04 4.4 applies only to a salvage that
  completes, or whose skipped reasons `--accept-loss` names; an
  incomplete one follows the incomplete rows of 01 1.6, `RJ-306`, back
  to `open`. 08 8.5's paragraph and the `salvaging -> removing` row of
  01 1.6 say so. `RJ-338` states once that a salvage restarted from
  `salvaging` is the command's own salvage and goes on to `sbx env rm`.
  Two more sentences that read against the rule are qualified in the
  same way: the `removing` row of 01 1.6 ("ends at `removing`") and its
  recovery paragraph, and the `salvage[].fingerprint` cell of 03 3.8,
  which now cites `RJ-338` for how the salvage ends. I17: the
  `RJ-338` case asserts the id, the state `removing` after the exit and
  that the resumed rerun writes no new entry; the `excludeIgnored` case
  asserts `RJ-338` and `removing`, and says its writer is on an ignored
  path, which the recheck of 08 8.5 step 2 does not see; two cases are
  added, "write during the salvage" (a tracked file, so the recheck
  skips the worktree: `RJ-306` and `open`, red when it returns
  `RJ-338`) and "kill the salvage" (the rerun reaches `sbx env rm`, red
  when it exits 5 with `RJ-338`).
- R16-03 (A): as decided. 04 4.4 quotes the hint: "If nothing still
  writes that path, rerun, and the rerun resumes at `sbx env rm`. If
  something still writes it, stop it or add an ignored path to
  `salvage.excludeIgnored` (gate 2), then rerun; the first rerun after
  either may exit 5 once more. `--accept-loss` does not cover this
  error." The details name the path, so the hint's "that path" is the
  first path whose entry changed, as the decision of round 15 words it.
- R16-12 (M): I17's `RJ-338` case asserts that the details name the
  changed path (printing no path turns it red); 04 4.4 quotes the
  hint; T068's first acceptance bullet cites the first `E:` case of
  I17 in place of its partial copy. Stated limit: no check ties the
  `E:` cases of I17 to test titles; T068's review reopens it.
- R16-13 (M): the one name is "the newest `fingerprint` written with
  `removing`", in 04 4.4, 01 1.6 (the `removing` row and the recovery
  paragraph), 03 3.8 and I17, since after one `RJ-338` exit two
  entries carry a `fingerprint` written with `removing`; "the rerun of
  `rm`" replaces "the resumed `rm`" in five I17 cases, the four the row
  names and the commit case of the same form.

The maintainer block and the token rows:

- R16-04 (A): read 2 has three parts: the host's forwarding setting
  that names the socket, the keys of that socket, and the account's SSH
  keys page, where the signing key's fingerprint is listed only under
  signing keys, citing 06 6.4. The verdict sentence reads the three
  parts.
- R16-05 (A): the first narrowed entry has a fourth part, the answers
  of item h; item h runs only once the note holds a narrowed entry and
  saves there; O8's pass names both.
- R16-06 (A): the note names the rows a later not narrowed entry
  returns (the token narrowing row of the index and the token row of 05
  5.4), says "the rows the first narrowed entry fired", gives a heading
  a time when two entries share a date, and makes a not narrowed entry
  after a narrowed one an event of the four rows. The note is the one
  home of that rule; the four triggers link to it (R16-09).
- R16-07 (M): O8's "What the maintainer does" names decision 1 of ADR
  0011 (item a), and its pass is the API answer of the default-branch
  ruleset with no bypass actor. The answer stays under the Evidence of
  the pull request that records it, as ADR 0011, decision 1, says,
  and not in the note's entry as the row proposed, since that would
  change the decision record.
- R16-08 (A): "a read made once `release.yml` exists leaves it out and
  records the tag answer as not measured"; the index row says the tag
  answer is measured only by an entry made before `release.yml` lands.
- R16-09 (A): the four rows link to the note's anchor; item g says the
  first narrowed entry is the record that the narrowing is applied, the
  moment ADR 0011, decision 2, fires the rows; the plan's opening reads
  "which the sandbox holds until block O8".
- R16-10 (A): the token narrowing row adds that deploy keys, webhooks
  and collaborators added before the narrowing are not read, citing
  the note's two limits.
- R16-11 (M): the cadence is applied: a later read is due at each
  sitting of blocks O3, O6 and O7, in the note. The dedicated signing
  socket is a recommendation in O8, citing 06 6.4, not a requirement,
  since ADR 0011, decision 2, decides that the forwarding stops and not
  how. Stated limit: how the forwarding stops is the maintainer's
  choice; an entry whose read 2 finds an authentication key reopens it.
