# Spec review round 3

Reader: reviewers checking that every round-3 finding was handled, and
maintainers tracing why the spec reads as it does. Type: reference
(history). The current state is [docs/spec.md](../spec.md); round 2 is
in [round-2.md](round-2.md).

Round 3 looked at one change: the commit that applied a review of
[ADR 0001, the documentation standard](../adr/0001-adopt-a-documentation-standard-with-checkable-rules-and-a-voice.md)
to the record and to the spec, the commit just before this round's
fixes. That commit also added
mechanisms no earlier round had seen, so round 3 reviewed those as new
design. It ran in three passes: a re-audit of the commit, a review
under the maintainer's principles in which each finding went to a
second reviewer briefed to refute it, and a completeness critic whose
findings no refuter checked. The fixes were applied in one write pass
on top of that commit.

The principles the round judged by: decide at the last responsible
moment, and record what is left out with the event that reopens it;
count concepts, one rule with one source; a rule that depends on
someone remembering is a defect; each merge gate also runs locally from
the same code; learn from a source and cite it, never copy it; open
maintainer decisions live in one table.

Status: **A** = applied; **M** = applied in a smaller or different form
than proposed, as stated; **D** = declined or deferred, with the
reason; **O** = maintainer decision.

## What the reviewed commit had added

| Mechanism | Outcome in round 3 |
|---|---|
| `docs/adr/**` as a candidate ask-first surface | adopted (O); rule 8 now rests on it |
| `tools/ci workflows`, the local-parity check | kept (O); its grammar is now closed and written out |
| the definition of equivalent local gates | kept (O); the "not covered" escape removed, the run record deferred |
| the `sequences` clause "a cited ADR is Accepted" | kept (O); the spec says it fails that clause today |
| the hashed denylist entry | kept (O); extended to segments so that separator forms match |
| restoring the spec text if ADR 0001 is declined | kept as prose (O); no check |
| `links.yml`, a scheduled external-link workflow | removed and deferred (O) |
| a generated spell-check word list under `docs/spelling/` | removed and deferred |
| `e2e/scenarios/commands.yaml` and its matcher (rule 13) | kept; the matcher is now specified, and the rejected alternative is recorded |

## Maintainer decisions

Decided on 2026-10-01. Each was put to the maintainer as a
recommendation, in one block, and the maintainer accepted the block.

| Id | Decision | Where |
|---|---|---|
| op-host-suite | ADR 0001 rule 13 keeps the host-suite alternative: a guide command may be exercised by the host suite and not in CI. Why: CI is offline and has no real sbx | ADR 0001 rule 13 and its conflicts table |
| op-plain-hash | denylist entries use a plain, unkeyed hash, knowing that it only keeps the plaintext out of a search and hides nothing from someone who guesses the name. A keyed hash was the alternative; its key would be a CI secret that a local run does not have, so the gate would stop being the same check locally | 10 10.2 |
| op-upstream-paths | a registry or repository path of the upstream sandbox product is not a forbidden name, so a slash is not a separator between the halves of an entry; the denylist and the scanner used before `tools/ci` exists agree on that separator rule. The scanner reads more: it also folds Unicode compatibility forms and joins wrapped lines, which the denylist matcher does not (10 10.2, the limits) | 10 10.2 |
| op-adr-ask-first | `docs/adr/**` and `.adr-dir` are an ask-first surface | 05 5.3, ADR 0001 rule 8 |
| op-checks-ask-first | the code of the gates is an ask-first surface too: `tools/ci/**`, the hooks, the denylist file, the linter configuration. Why: the workflows are thin shells, so the check code is the gate | 05 5.3 |
| op-workflows | `tools/ci workflows` stays as the mechanism that keeps hosted and local runs the same | 10 10.2 |
| op-equivalence | local gates are equivalent when the same subcommands run at the same commit on each OS and architecture of the runner list; a release still needs the hosted runner, for the attestation | 10 10.2 |
| op-accepted-only | the spec may cite only Accepted ADRs; the spec fails that check until ADR 0001 is signed off | 10 10.2, Q24 |
| op-entry-form | a denylist entry is a length and a sha256, matched against each substring of that length, and applies to the PR title, body, branch name and commit messages | 10 10.2 |
| op-declined | if ADR 0001 is declined, the PR that marks it Rejected restores the spec text; prose only, no check | Q24 |
| op-links | the scheduled external-links workflow leaves v1 and is deferred; `tools/ci links` stays, run by hand | index, Deferred decisions |

One reconciliation is the writer's, not the maintainer's: op-entry-form
and op-upstream-paths together need separator forms to match while a
slash does not. An entry therefore became an ordered list of segments,
each a length and a sha256, matched as op-entry-form says, with the
separator rule between them (10 10.2, Forbidden names).

## Resolutions

Ids are the reviewers': `N` and `#` from the re-audit, `A-` from the
principles review, `C-` from the critic.

| Finding | Status | Resolution |
|---|---|---|
| #6, #8, N1 an agent could promote or rewrite an ADR: the sign-off quote is typed by the PR author, and only a Status change triggered the check | A | `docs/adr/**` and `.adr-dir` are the `decisions` surface, so any ADR diff needs the approval line and a code-owner review; the check code is the `checks` surface; 05 5.4 records what the line does not prove |
| N2 the approval-quote form was defined nowhere | A | one form, in 12 12.4; rule 8 and 05 5.3 point to it |
| N3 a placeholder could not match `sandbox/<branch>`; journey ids J3a and J3b | A | a placeholder matches one or more non-whitespace characters inside a word; the table is keyed by scenario id, a journey or one of its variants |
| N4, #24, A-determinism-1, C-4 the denylist read the HEAD tree and PR text only, missed separator forms, and was first applied to messages and ref names after the push | A | one definition and one function; three surfaces (tracked files, the pushed range in the pre-push hook, the PR); limits listed |
| N5 the `workflows` check looked at `run:` and `uses:` only, and `release.yml` as described broke it | A | a closed key grammar; `release.yml` is described as `tools/release` subcommands around one `uses` step |
| N6, A-simplicity-6, A-determinism-5 (b) "equivalent" allowed a platform named as not covered; the evidence record had no kind | M | no equivalence without a run on each platform; the record format is deferred, with the constraint that `tools/ci` writes it |
| N7, A-simplicity-9 "every gate" overstated; the policy was restated in 12.4 | A | "every merge gate"; 12.4 keeps one sentence and the link |
| N8 the em dash ban was justified by a circular cite | A | the conflicts row cites the index Boundaries and gives the reason |
| N9 a command after `cd x &&` or in another fence escaped rule 13 | M | lines are split at shell operators; a block in another language stays a stated limit |
| N10 hygiene had no caller until the hook landed; the PR title was outside rules 4 and 11; the denylist had no path; `tools/ci fuzz` was undescribed | A | hygiene, `fast` and the hook land in one commit; title added to both rules; `tools/ci/denylist.yaml`; `fuzz` described in 10.2 |
| N11, A-fidelity-3 10.2 recorded a decision with a rejected alternative, worded as decided before the maintainer had answered; the index status went stale at acceptance | A | op-plain-hash, recorded here; 10.2 states the current state; the status of ADR 0001 lives in Q24 only |
| the spec fails its own `sequences` clause and did not say so | A | said in Q24 and in ADR 0001 Consequences |
| #28, A-determinism-10 (b) title and filename had no shared rule | A | one `slug` function, used by `tools/new adr` and `tools/ci sequences` |
| A-determinism-10 (a) no transition table for ADR statuses | D | refuted in review: each status change is already the maintainer's, through rule 8 |
| #30 ADR 0001 cited by number only at a file's first mention | A | the first mention in 10 and in 12 is the titled link |
| A-fidelity-1 the upstream workload image path matched the interim scanner | O | op-upstream-paths |
| A-determinism-2 no writer for denylist entries | A | `tools/ci hygiene add` |
| A-determinism-3 the gate code was not ask-first | O | op-checks-ask-first |
| A-determinism-4 the inputs of `tools/ci pr` were not in one payload | A | payload file for title, body, ref and SHAs; git for commits and paths; no network call |
| A-determinism-5 (a) nothing tied `tools/ci` to the locked tool versions | A | the tools start through `mise exec` |
| A-determinism-6 "no scenario function runs" could not be tested for host-only journeys | A | "reads", tested with a recording runner |
| A-determinism-7 nothing tested the linter configuration | A | a fixture with one violation per checker |
| A-determinism-8 hand-kept copies of the generated list and the `fast` list | A | both copies deleted; generating the table is deferred |
| A-determinism-9, A-simplicity-3 a scheduled link workflow; no workflow carried the mutation schedule | O, A | op-links; `fuzz.yml` calls `tools/ci mutate` |
| A-fidelity-2 the status line said round 2 for content no round had reviewed | M | this file; the status line points here. The reviewed commit's own message is not reworded by this pass |
| A-fidelity-4 new maintainer decisions sat outside the Open questions table | M | the two real candidates were decided (op-plain-hash, op-equivalence); the one still open is Q24 |
| A-fidelity-5 the record said adr-tools writes `Supersedes` | A | the conflicts row states the tool's spelling, cites the source, and records the divergence |
| A-simplicity-1 rule 13 had no recorded alternative | A | an entry in Alternatives considered |
| A-simplicity-2 a generated spell-check list before a spell checker is chosen | A | removed; deferred with its trigger |
| A-simplicity-8 docs versioning was deferred without a trigger | A | a row in Deferred decisions |

## Removed or deferred

Removed from the spec: `links.yml`; `docs/spelling/` and its generator;
the hand-kept list in the `generated` row of 10.2 and the Enforces list
of the pre-push row in 12.4; the per-PR evidence tuple for local gates;
the quota policy restated in 12.4; the rule 8 trigger on the Status
section, which the `decisions` surface replaces.

Deferred, each with its trigger, in the index's
[Deferred decisions](../spec.md#deferred-decisions) table: the
scheduled external-link workflow, the generated spell-check list, docs
versioning per release, generating the 10.2 step table, and the record
of a local-gate run.

New open question: Q24 (is ADR 0001 accepted).
