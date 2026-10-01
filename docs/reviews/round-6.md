# Spec review round 6: the sweep of rounds 4 and 5

Reader: reviewers checking that every finding of the sweep was handled,
and maintainers tracing why the spec reads as it does. Type: reference
(history). The current state is [docs/spec.md](../spec.md); round 5 is
in [round-5.md](round-5.md).

Round 6 reviewed the spec as rounds 4 and 5 left it: the engineering
practices, the runtime ledger, and everything under them. It is the
sweep that round 4 and round 5 call the round-3 sweep, which is why
its finding ids start with `R3-`. Eight reviewers read the whole spec,
each through one lens: security, doubt, testability, fidelity to the
maintainer's record, craftsmanship, simplicity, a newcomer's reading,
and the documentation standard. Each finding then went to a second
reviewer briefed to refute it, and each High finding that survived to
a third. A completeness critic read last; no refuter checked its
thirteen findings, so this pass checked each against the spec text
before applying it.

98 findings survived and 29 were refuted. By severity: 3 High, 37
Medium, 58 Low. Where a refuter adjusted a fix, the adjusted fix is
the one applied. The fixes were applied in one write pass, with eleven
items the previous gate had left for it and with fifteen maintainer
decisions.

The principles the pass was judged by: prefer deleting, merging or
deferring with a trigger over adding machinery; where a fix would add
a concept, state the limit first; count concepts; a rule lives in one
place and the other places point to it; each deferred decision carries
the event that reopens it.

Status: **A** = applied; **M** = applied in a smaller or different
form than proposed, as stated; **L** = stated as a limit, no mechanism
added; **D** = deferred, with its trigger in the index; **R** =
rejected, with the reason; **O** = maintainer decision.

## Maintainer decisions

Decided by the maintainer on 2026-10-01. The fifteen items were put to
the maintainer as recommendations, in one block. Before the answer, at
the maintainer's request, a reviewer briefed to refute the block
checked each recommendation against the spec and corrected nine of
them. The maintainer accepted the corrected block, and answered its
questions in the maintainer's own words: memory that exists
will be imported when v1 is adopted; `handoff list` is in v1; the
response expectation is 30 days; where herdr's upstream is; ssh-agent
use joins what a personal kit may not declare; results measured on the
release candidate count for v1.0.0.

| Id | Decision | Accepted cost | Where |
|---|---|---|---|
| op-acceptance-after-release | `tools/ci acceptance` leaves the release workflow. The acceptance file is committed by the final plan task, after the release; a `command` item is run again by its argv, for the product's own file only. The release candidate of block B is a `v*` tag with a suffix, published as a prerelease; release notes count from the last tag that is not a prerelease; the build order names the two tags. Results measured on the candidate count for v1.0.0 | a release no longer verifies the acceptance file again; one more tag for the maintainer to push | 10 10.2, 10 10.5, 11 11.2, 12 12.2, 12 12.6 |
| op-probe-completeness | the requirement that every probe has a result moves from `tools/ci probes` to `tools/ci acceptance` | a probe nobody ran is noticed only at the final task | 11 11.4, 10 10.5 |
| op-merge-commits | merge commits only: the default-branch ruleset allows no other method, and the repository's squash and rebase settings are off, with the API's answer saved | none in code; `main` carries each pull request's commits | 10 10.2 (maintainer block), 12 12.9, ADR 0002 |
| op-host-clone | items a and b of the maintainer block run on the host, in a plain clone, after the maintainer has read the diff of step 1 | the step-1 review is an explicit step | 10 10.2 |
| op-ruleset-approvals | the default-branch ruleset dismisses an approval when a commit is pushed and requires approval of the most recent reviewable push | each push after an approval costs the maintainer one more review | 10 10.2, 12 12.4 |
| op-no-path | the `path` field leaves v1, with `--path`, the secret-pattern check, its list, its error slug and their test rows; one Deferred row | an install failure no longer names the file it failed on; ADR 0006 and I33 change | 13, 04 4.3, 04 4.4, 05, ADR 0006; index, Deferred decisions |
| op-mutate | three sentences say how `mutate` treats an absence invariant, a runner without a test level, and "fails"; no invariant is exempt | running e2e under mutants raises CI time | 05 5.2 |
| op-no-sandbox-source | `sync --from sandbox/<branch>` is deleted, with its open question and its probe | a spec that was never pushed cannot be tried; the deleted probe was the only one that measured idle auto-stop | 04 4.2, 05 5.1, I27, 09 J2, 11 11.1 |
| op-migration-set | `julieta memory import`, `julieta memory verify`, and `tools/ci acceptance --file` with the `repo` evidence kind stay in v1, because memory that exists will be imported when v1 is adopted | they trace to a named need, not to a criterion or a journey | 00 0.5 |
| op-handoff-list | `julieta handoff list` stays in v1 | as above | 00 0.5 |
| op-dora-late | `tools/ci dora` and its fixtures land with `ci-release`, not on day one; the `checks` surface's reason reads "the code of the gates and reports"; the form check of a `Fixes-release:` trailer is in `tools/ci pr` from the start | pull request size and the maintainer wait have no number until the tool lands; its first run reads the whole history | 10 10.2, 12 12.2, 12 12.9, 05 5.3, ADR 0004 |
| op-s8-limit | no timing field is added: the `julieta setup` budget of S8 is measured in the container e2e against local origins, so the network is outside it, and the spec says so | the julieta figure has no host confirmation | S8, 10 10.1, 11 11.2 |
| op-no-live-view | the optional live-view step, its open question and its probe are deleted | none; the v1.1 helper stays listed in 00 0.5 | 09 J4 |
| op-security-page | `SECURITY.md` lands with or before the `tools/ci docs` check; the channel is GitHub's private vulnerability reporting, switched on in the maintainer block; the expectation is an acknowledgement within 30 days, best effort, with no promised time for a fix | one more item in the maintainer block | 12 12.2, 12 12.8, 10 10.2 |
| op-herdr-pin | herdr's upstream is named; the pin is a version and a sha256 per architecture, taken by `tools/kitpin` from the release API's digest of a release marked immutable | the hash proves the binary has not changed since publication, not who built it: a residual-risk row | 06 6.4, 05 5.4 |
| op-personal-kits | the rule says what is refused: network, mount, volume, credential and ssh-agent capabilities. The other capability types are shown in the gate | a personal kit can still run an install step; the gate diff is the control | 01 1.4, 06 6.1, I28 |

Three of these decide between two findings whose adjusted fixes
disagreed. op-acceptance-after-release takes the fix of
R3-testability-01 over that of R3-doubt-09. op-probe-completeness
takes R3-testability-06 over R3-newcomer-01. op-dora-late keeps the
trailer's form check where R3-newcomer-06 put it, not where
R3-simplicity-01 moved it.

## Resolutions

| Finding | Status | Resolution |
|---|---|---|
| R3-doubt-01 (High) no way out of `salvaging` while the sandbox lives | A | one row: `salvaging` to `salvaging` when `salvage`, `rm`, `recreate` or `retire` starts the sandbox half again (01 1.6) |
| R3-newcomer-01 (High) `tools/ci probes` kept `all` red until block B | O | op-probe-completeness; and a golden with no recorded A5 hash is not flagged (11 11.4) |
| R3-testability-01 (High) S1 and S2 could not be in a tracked file; acceptance ran before the release existed; `command` evidence had no rule | O | op-acceptance-after-release |
| R3-craftsmanship-01 two shared packages depended on side-specific code | A | `salvage` takes the agent profile as an argument (08 8.5); "Depends on" is plan order, 10.7 is the import rule (12 12.2) |
| R3-craftsmanship-02 `ledger` was not a reserved repo dir | A | added to the reserved names (03 3.1) |
| R3-craftsmanship-03 flags after the positional argument do not parse with stdlib `flag` | A | flags precede positional arguments; nine synopses and J2 reordered (04, 09) |
| R3-craftsmanship-04 S8's budgets against the design | O | op-s8-limit |
| R3-craftsmanship-05 the merge method was fixed nowhere | O | op-merge-commits |
| R3-craftsmanship-06 `memstore`, shared, carried `Delete` inside the I16 scan | A | the store is two packages: shared readers, and julieta's `memstore/write` (08 8.1, 10 10.7) |
| R3-craftsmanship-07 no generation row for `salvage --from-host` | A | one row that changes no state (01 1.6) |
| R3-craftsmanship-08 the idempotency rule could not pass | A | gitsafe fetches with `--no-write-fetch-head`; the diff is content and mode; the appending commands are a named list (04 4.1, 10 10.1) |
| R3-craftsmanship-09 julieta's pin commands needed `oci`, a romeu-only package | A | `oci` is shared; `pin` is in `julieta-core` (10 10.7, 12 12.2) |
| R3-craftsmanship-11 `tools` named two things | M | `internal/tools` is `internal/mise`; no vocabulary rows for `entry` and `manifest` |
| R3-craftsmanship-13 four incoherences in the CLI contract | M | `adopt` lists exit 3; `--json` is one document or one object per line where the row says so. The other two parts were dropped by the refuter |
| R3-craftsmanship-14 the preflight "before invoking sbx" called sbx | A | "before any mutating sbx call"; `sync` checks gate 1 at its first sbx call, step 9; step 6 says an absent sandbox passes; `run` step 2 points at it (01 1.5, 04) |
| R3-critic-01 the leftover cleanup inside ingest broke its own race test | A | verified against 13 13.5 and 13 13.10. The cleanup runs in the command, under the lock, before the first ingest; the ingest function unlinks only its own temporary name; one test row added |
| R3-critic-02 a removed table member rejects events an older julieta wrote | L, D | verified against 13 13.2 and 13 13.7. Members are only added in v1; validating against the previous release's tables is deferred |
| R3-critic-03 `ledger` and `julieta-core` depend on each other | R | three refuted findings of the sweep made the same claim, and their refuters showed that 12.2 lists plan order: the ledger task wires the drain and the emit into julieta. The sentence that says so is the one of R3-craftsmanship-01 |
| R3-critic-04 the set of ids `invariants` iterates was undefined | A | verified against 05 5.2 and 10 10.2. `invariants` pairs the tags in the code; `acceptance` checks once that every row has both (05 5.2, 10 10.5) |
| R3-critic-05 no linter bounds dark code | A, D | verified: 12 12.9 and ADR 0002 claimed a bound a dead-code checker does not give for exported identifiers. The claim is deleted, coverage stays, a check is deferred |
| R3-critic-06 the live-view step pointed a host tool at an agent tree | O | op-no-live-view |
| R3-critic-07 `retire` had no next ingest | A | verified against 13 13.4 and J7. `retire` ingests once more, after the sandbox half and before the move |
| R3-critic-08 `path` named no repo | O | moot under op-no-path |
| R3-critic-09 draft or published was fixed nowhere | O | `tools/release publish` creates a published release, never a draft; the prerelease clause is op-acceptance-after-release |
| R3-critic-10 one repository, two URL spellings | A | verified against 03 3.1. URLs are compared after one normalization |
| R3-critic-11 `gh` was a dependency nobody listed | M | verified against J1 and 12 12.1. `gh` is a prerequisite in J1 and the README outline and is pinned in `mise.lock` for `tools/release verify`. No version number is written: the pin is the floor |
| R3-critic-12 one condition, two error ids | A | verified against 04 4.4, 13 13.4 and 13 13.5. `ledger-incomplete` is the one id an ingest ends with; `ledger-entry-mismatch` is a reason inside it and the id `doctor` reports |
| R3-critic-13 recorded help text against the em dash check | A | not verifiable here (no sbx); the fix costs one clause, so it is applied: the recorder replaces U+2014 (10 10.3) |
| R3-docs-1 a quotation that is not on the page | A | replaced by a paraphrase marked as ours (ADR 0002) |
| R3-docs-2 ADR 0004 did not say its tool is on an ask-first surface | O | op-dora-late; one sentence in ADR 0004's Consequences |
| R3-docs-3 edition names from sources nobody named | A | the edition names, the labels and the unquoted cadence are deleted (ADR 0003, 12 12.9) |
| R3-docs-5 the dark-code bound rested on a checker nobody switched on | A | with R3-critic-05 |
| R3-docs-6 two hand copies of which probe settles which question disagreed | M | the index's "Settled by" column is the one source; 11's Settles column and the result format no longer hold question ids |
| R3-docs-7 no module wrote `SECURITY.md` | O | op-security-page |
| R3-docs-8 a paraphrase under "DORA's definition" | A | the page's sentence, in quotes (ADR 0004, 12 12.10) |
| R3-docs-9 a citing habit with no rule | M | left to the maintainer as Q26; ADR 0005's first mention now carries the title |
| R3-doubt-02 a crash between create and record left an unknown sandbox | A | the generation is recorded before `sbx env run` and removed when sbx fails (01 1.6, 04) |
| R3-doubt-03 I24 counted three readers and the CLI had seven | M | the count and the list are deleted; `imports` fails a romeu-linked package that imports the writer; that no other code opens a memory path is review |
| R3-doubt-04 "before every sbx call" was not true of `status`, `adopt`, `doctor` | A | "of a P command"; `status` skips its julieta exec when drift or identity is not clean |
| R3-doubt-05 I2's byte comparison against the remote sbx writes | A | the `remote.sandbox-<name>.*` keys are sbx-owned and left out; the fake sbx writes the stanza (05, 01 1.2, 10 10.3) |
| R3-doubt-06 julieta's drain deleted files inside the scanned package | A | emit and drain are `ledger/emit`, linked by julieta only (10 10.7, 13 13.2) |
| R3-doubt-08 the fix marker depended on the merge method | M | with op-merge-commits: a deployment's commits are those of its pull requests, read from GitHub (12 12.10, ADR 0004) |
| R3-doubt-09 S1 and S2 in a tracked file | R | the maintainer took the fix of R3-testability-01 (op-acceptance-after-release) |
| R3-doubt-10 block C had no computed decision | M | with R3-newcomer-03 |
| R3-doubt-11, R3-simplicity-14 an overturned probe needed an ADR to supersede that did not exist | A | "adds an ADR that records the decision; it supersedes the default's ADR when one exists" (11 11.4) |
| R3-doubt-14 the `setup` no-op is a network round trip | O | op-s8-limit; the fetch no longer writes `FETCH_HEAD` |
| R3-doubt-15 view derivation against S8 | D | one more trigger on the rotation row |
| R3-doubt-16 no probe named the `sbx ls` fields | A | A4 records them (11 11.1) |
| R3-doubt-17 no record held the workspace files' hash | A | they are regenerated from state and not drift-checked (01 1.2) |
| R3-doubt-18 nothing said when the lock is released | A | held for the whole command, released before the final `exec` of `run`; one test (04 4.1, 10 10.1) |
| R3-doubt-19 `approve` did not apply live changes and said nothing | A | it prints that they apply on the next `run` or `sync`; J11 step 5 |
| R3-doubt-20 two rules for when a hook failure stops being shown | A | "since the last session" deleted (08 8.2) |
| R3-doubt-21 a project recreated under a retired name | L | one sentence in 13 13.5 |
| R3-doubt-22 the manifest exception list named a flag that does not exist | A | the list is deleted; the Reads column names the manifest |
| R3-doubt-24 the crash-safety sentence was false for `.romeu/bin` | A | reworded (01 1.6) |
| R3-fidelity-1 `path` broke "no free text" | O | op-no-path |
| R3-fidelity-2 where items a and b run | O | op-host-clone |
| R3-fidelity-5 a deferred maintainer decision had no row | A | one row: the maintainer's merge as a manual checkpoint (index; 12 12.10) |
| R3-fidelity-6 three ADRs attributed the writer's words to the maintainer | A | reworded (ADRs 0003, 0004, 0005). The op-practices row of round 4 is left as written |
| R3-newcomer-02 how the first release happens | O | op-acceptance-after-release |
| R3-newcomer-03 a third block nobody scheduled | M | the sandbox-side checks keep their ids and are part of block B: the harness runs them through `sbx env exec` in the B2 sandbox (11 11.2) |
| R3-newcomer-04 herdr was never defined | O | op-herdr-pin |
| R3-newcomer-05 julieta steps on the macOS runners | A | a journey that reaches julieta runs at the hybrid level, on the Linux runners (10 10.1, 10 10.3, 09, S7) |
| R3-newcomer-06 `dora` in the first plan task | O | op-dora-late |
| R3-newcomer-07 "contains" was undefined | M | one meaning: a deployment's commits are those of its changes; a commit outside a pull request counts in no metric (12 12.10) |
| R3-newcomer-09 three forms of the probe output path | A | one file per probe; `--out` names a directory (11) |
| R3-newcomer-11 forward references | M | "dekit" deleted (03 3.2); nothing else, as the refuter adjusted |
| R3-security-01 an approval survived a later push | O | op-ruleset-approvals |
| R3-security-02 a local `insteadOf` is not neutralized | L | stated in 05 5.1; I3 plants a rewrite to a refused protocol |
| R3-security-03 `id` crossed projects | A | `id` is off the cross-project allowlist; the low-rate channel is named in 05 5.4 |
| R3-security-04 the generators of gate files had no path | A | the generator that reads `ask-first.yaml` is a `tools/ci` subcommand (12 12.3) |
| R3-security-06 salvage against a hostile agent | L | one row in 05 5.4 |
| R3-security-07 "files and agent context only" | O | op-personal-kits |
| R3-security-08 no fsck before a secondary review checkout | A | `git fsck --strict` on the unbundled head (04, `romeu pull`; 05 5.4) |
| R3-security-09 the read-only host clone was not measured | A | one step in A3 (11 11.1) |
| R3-simplicity-01 the metrics tool on day one | O | op-dora-late |
| R3-simplicity-02 thirteen tools before the first package | A | day one has `tools/new adr`, `generated`, `sequences`, `pr` and the ask-first list; each other kind and generator lands with its module (10 10.2, 12 12.2, 12 12.3; index) |
| R3-simplicity-03 features that trace to nothing | O | op-migration-set, op-handoff-list |
| R3-simplicity-04 the sandbox spec source | O | op-no-sandbox-source |
| R3-simplicity-06 four lock platforms required everywhere | A | `--check` requires the two linux ones; a missing macOS entry is a warning (07 7.3, 04) |
| R3-simplicity-11 two bounds for one job | A | the count cap and `over-cap` are deleted; the listing bound stays (13) |
| R3-simplicity-15 a registry with one entry | A | the Swappable row is deleted (08 8.1) |
| R3-testability-02 S6 for absence invariants | O | op-mutate |
| R3-testability-04 the fake sbx could not express out-of-band state | M | a scenario names the recorded session it starts from; A1 records the settings read and A4 `sbx stop`, since the probe the fix named left the spec (10 10.3, 11 11.1) |
| R3-testability-05 append commands in the idempotency meta-test | A | with R3-craftsmanship-08 |
| R3-testability-06 `probes` red until the last probe | O | op-probe-completeness |
| R3-testability-07 scans at a granularity nothing can see | M | an admitted call site is a named function in one table under `tools/ci`, for I16, I31 and I32 (05 5.2). For I24 the smaller fix of R3-doubt-03 was taken |
| R3-testability-08 `--catalog` could report an unknown and exit 0 | A | it exits 1; a `ci-run` item names its repository (04, 10 10.5, S4) |
| R3-testability-09 block B steps had no kind | A | each is `kind: check`, with its predicate (11 11.2) |
| R3-testability-10 journey steps with no oracle | A | one legend sentence (09) |
| R3-testability-11 rows that need root | M | the device node leaves the unit rows and I12 and joins A3's plants |
| R3-testability-12 what a command name is | A | the full command path (13 13.2) |
| R3-testability-13 what a golden is, for `pr` | A | one sentence (10 10.4) |
| R3-testability-14 `--timings --json` had no schema | A | `run-timings.v1` with `romeuMs` and `sbxMs` (04, 02 2.1) |
| R3-testability-15 nothing required a guide per journey | M | the check is in `acceptance`, not in `docs`: in `docs` it would keep `all` red until the guides are written. The ADR half of S10 is marked review |
| R3-testability-16 what a download is, for `kits` | A | defined (10 10.2) |
| R3-testability-17 one hook order tested | A | the container e2e runs both (10 10.1, 09 J5, Q22) |

The eleven items the previous gate left for this pass:

| Item | Status | Resolution |
|---|---|---|
| G1 maintainer block, item d: a try that is not refused takes effect; who runs what; "before step 1 is pushed" | A | throwaway payloads and what the maintainer undoes; the maintainer runs each item and opens step 1's PR; the sitting is around step 1; the tries are repeated with the second account if Q25's fallback is taken (10 10.2) |
| G2 a merge of the default branch into a PR demanded approval lines for surfaces the PR never changed | A | changed paths are read from the merge base to the head (10 10.2, 12 12.4) |
| G3 ADR 0001 rule 13 fails closed on lines that name a binary as a file | L | one limit sentence: such lines go in a `text` block |
| G4 the third fix pass has no review record | A | below |
| G5 maintainer-block evidence is text no check reads; approval-line edge cases; the matcher for a repeated first segment | A | tagged review; a trailing carriage return, a whitespace-only quote and `**` over zero segments are defined (12 12.4); a match is any start position that leads to all segments (10 10.2) |
| G6 nits | M | the Never bullet names the identity; issues and review comments are in the limits; an identity that holds an entry is a stated cost; the step-1 branch is fetched by the maintainer; a push to a URL walks every reachable commit; the TTY tests include `hygiene add`. Two nits were not applied: "stale hand copies" named none, and what was meant by "variant ids in the J3 heading" could not be told |
| G7 the workflow grammar left expressions open | A | an expression is one context path (10 10.2) |
| G8 `generated` on a dirty tree and on new files; the `docs` row repeated it | A | a comparison before and after the run; the limit for an untracked file; the duplicate removed (10 10.2) |
| G9 Markdown lint and spell check have no oracle | D | one row: which tools, and their configuration on the `checks` globs |
| G10 the bypass actor and the sandbox's token | - | known and carried by Q25; nothing to apply |
| G11 the spec cites Proposed ADRs | - | known and stated in Q24; the statement was checked and is where it should be |

## The writer's own decisions

The maintainer has not ruled on these. The next review should look at
each:

- The rule for what may change between the release candidate and
  v1.0.0: only paths under `docs/` and Markdown files at the
  repository root, checked by `tools/ci acceptance` against each block
  B result's `commit`. The maintainer decided that there is a rule,
  not which. So that the rule can hold, a block B artifact is
  committed under `docs/probes/`.
- B1's Settles cell no longer says S1: S1 is about v1.0.0 and its
  evidence comes from that release.
- With `--file`, a `command` item is not run and is listed as
  recorded, not verified.
- The "both hosts" requirement of an arch-sensitive probe moved to
  `acceptance` with the completeness requirement.
- The index's "Settled by" column as the one source: question ids left
  11's Settles column, and the `settles` field left the probe result
  format.
- The guide-per-journey check in `acceptance` and not in `docs`.
- The sandbox-side checks as part of block B, with `block: "B"`.
- `memstore/write` and `ledger/emit` as the names of the two
  julieta-only packages.
- The prompt-injection row of 05 5.4 was deleted with `path`, so the
  ledger rows there are six.
- The 1.5 heading says "before every mutating sbx call", and its
  anchor changed with it.
- `sbx stop` recorded by A4, since the probe the fix named was
  deleted.
- The trigger of the deferred `path` row. The finding's own trigger
  was not in the input this pass had.
- Q26, for the citing habit: the finding left the choice to the
  maintainer, and an open maintainer decision lives in the Open
  questions table.
- A row that takes an open generation back to none when the creating
  `sbx env run` fails.
- ADR 0002 gained a sixth decision item for the merge method, since
  12.9 takes its trunk rules from that record.
- op-dora-late was read as: the tool and its fixtures land with
  `ci-release`, and `tools/ci pr` is whole from day one, the trailer's
  form check included.
- In ADR 0001, the example of a placeholder was replaced, because it
  named the deleted spec source. Its rule 11 still mentions a squash
  merge as the reason to check the PR title; with squash off the
  reason is weaker and the check stays.

## The third fix pass, recorded late

The commit that applied the previous gate's first-round findings to
ADR 0001 and the spec had no review record. This is that record,
written from the commit's message and its diff; the gate's finding ids
were not kept, so it has no row per finding.

What it added: the maintainer block of `ci-bootstrap`, with the
rulesets and the saved refusals; Q25; `owner` in `.github/ask-first.yaml`
and the reading of that file at base and at head; the forbidden-name
walk over paths and identities, and `hygiene add` reading from a
terminal; the fail-closed matcher of ADR 0001 rule 13 and its closed
list of fence languages; the payload fields of `tools/ci pr`, the
approval-line pattern, the glob dialect, the commit subject form, the
personal-path rule and the ids `sequences` reads.

Which of those are the maintainer's: two, and both are recorded in
[round 4](round-4.md#maintainer-decisions). The three residual risks
that the block rests on (op-residual-risks) and the fallback of Q25
(op-second-account). The rest was the writer's answer to the gate's
findings, and this round's sweep is the first review that read it.

## What the maintainer has to do

Nothing in this round is done until the maintainer acts on these, in
this order:

1. Sign off the six Proposed records, or decline one (Q24). It comes
   before the plan's first task.
2. Answer Q26.
3. The maintainer block of `ci-bootstrap`
   ([10 10.2](../spec/10-testing-style.md#release-and-bootstrap)):
   read the diff of step 1, then items a to d in one sitting, and item
   e after step 2.
4. Run probe block A on both hosts.
5. Push the release candidate tag after `ci-release`, run block B on
   both hosts, and push the v1.0.0 tag after `docs`
   ([12 12.2](../spec/12-engineering.md#122-development-commands-and-capability-map)).
6. Write what happens next in `SECURITY.md`
   ([12 12.8](../spec/12-engineering.md#128-contributor-docs-outlines)).

## Removed or deferred

Removed from the spec: the `path` field with its flag, the
secret-pattern check, its list and its error slug; the spec source
`--from sandbox/<branch>`; two open questions and their two probes
(the live view, the idle auto-stop); the optional live-view step of
J4; `tools/ci acceptance` as a step of `release.yml`; the count cap of
the spool and `over-cap`; the memory backend registry; the lint half
of the dark-code bound; the count and the list of memory readers in
I24; the manifest exception list and `--memory-dir`; block C as a
third block; question ids in 11's Settles column and the `settles`
field; the edition names of the XP practices; one residual-risk row.

Deferred, each with its trigger, in the index's
[Deferred decisions](../spec.md#deferred-decisions) table: the
Markdown linter, the spell checker and their configuration; a check
that bounds dark code; whether the maintainer's merge stays a manual
checkpoint; a `path` field on an event; removing a member from a
membership table. Two rows changed: rotation gained a trigger, and the
tuning row lost the pattern list and the count cap.

Added, counted as the index asks: two packages (`memstore/write`,
`ledger/emit`), each split from one that existed; the table of
admitted call sites, which replaces three lists of call sites; the
`run-timings.v1` document; the release candidate, a prerelease tag;
three rows of the generation machine; two residual-risk rows; `gh` as a
pinned tool; the `SECURITY.md` outline.

Questions changed: the two questions named under Removed left the
table; Q26 is new; Q22 says the test no longer waits for its probe.
