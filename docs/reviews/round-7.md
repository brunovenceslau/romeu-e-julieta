# Spec review round 7: the re-audit of round 6

Reader: reviewers checking that every finding of the re-audit was
handled, and maintainers tracing why the spec reads as it does. Type:
reference (history). The current state is [docs/spec.md](../spec.md);
round 6 is in [round-6.md](round-6.md).

Round 7 reviewed the pass that round 6 records. Two reviewers read it,
and this page calls them the re-audit: reviewer A read for correctness
of the plan and the documents, reviewer B for security. They raised
five findings that must be fixed (M1 to M5) and fourteen smaller ones
(A1 to A5, B1 to B8, B-info). The fixes were applied in one write pass.

Every item below is a finding of the re-audit. None is a maintainer
decision, and the maintainer has not ruled on any of them.

The principle the pass was judged by: prefer deleting, merging or
stating a limit over adding machinery. Each fix is a few sentences.

Status, as in round 6: **A** = applied; **M** = applied in a smaller or
different form than proposed, as stated; **L** = stated as a limit, no
mechanism added.

## Resolutions

| Finding | Status | Resolution |
|---|---|---|
| M1 (reviewer A, Required) the `README.md` skeleton landed after the candidate, so rule 12 of ADR 0001 had no page to check between `ci-release` and `docs`, and its heading list would have changed `tools/ci/headings.yaml` after the candidate | A | the `ci-release` row lands the `README.md` skeleton with `SECURITY.md`; the `docs` row says "README text" (12 12.2) |
| M2 (reviewer A, Required) S1's `command` evidence could not be run again from the repository root: the archive is a release asset, not a tracked file | A | S1's evidence is the asset list read through the GitHub API and a `ci-run` of `release.yml` on the tagged commit, whose `tools/release verify` step runs the attestation check (index, S1) |
| M3 (reviewer B, High) the refusals of the token test were read inside the sandbox, where the agent controls the shell and the tools | M | a refusal is proved on the host: four checks from the maintainer's own session, for the second account's tries too. The sentence that says when a try took effect was reworded so that it has a verb (10 10.2; 05 5.4; Q25) |
| M4 (reviewer B, Medium) the rulesets were created after the agent had held a token for the whole of step 1 | A | the ruleset item is first in the sitting, before the sandbox that writes step 1 receives its token, or at the latest before the denylist item; the items are lettered again, a to e; one clause covers a repository with no default branch yet, which nobody tested against GitHub (10 10.2, 12 12.2) |
| M5 (reviewer B, Medium) the candidate rule compared the commit the harness ran from, and claimed that nothing built had changed | A, L | B1's result records the candidate's tag and commit as two observations, and `acceptance` compares that commit to the v1.0.0 tag; the claim is replaced by the limit that the rule compares sources; the build order gains B1 on v1.0.0, on one host (11 11.2, 10 10.5, 12 12.2) |
| A1 no row left `salvaging` on `salvage --from-host` | A | one row, `salvaging` to `salvaging`, as the open row (01 1.6) |
| A2 the `docs` row listed the acceptance evidence, which is committed after the v1.0.0 tag | A | it left the `docs` row; the last step of the build order is `tools/ci acceptance`, which commits `docs/acceptance.json` (12 12.2) |
| A3 B3's test output under `docs/probes/` would have had an extension rule 17 of ADR 0001 refuses | M | the artifact is `docs/probes/B3-<host>-test.json`, one JSON object per line. The name the finding gave is the name of B3's result file, so the artifact has a suffix. Rule 17 is unchanged, and the example artifact of 11.3 is under `e2e/`, outside the rule (11 11.2) |
| A4 the migration row attributed the verification mechanism to the maintainer | A | reworded as proposed (00 0.5) |
| A5 "each release holds the whole history up to itself" was not true of a prerelease | A | one clause (12 12.10) |
| B1 `hygiene` allowed other files under `.githooks/` | A | "`.githooks/` holds exactly `pre-push`, tracked with mode 100755" (10 10.2) |
| B2 gate 1 at step 9 of `sync` let a stale toolchain promote first | A | gate 1 is checked at step 1; step 9 starts at `sbx ls --json`. This reverses the placement the R3-craftsmanship-14 fix of round 6 chose (04 4.2) |
| B3 B3's test output was committed without redaction | A | written through the recorder's redaction before it is hashed (11 11.2) |
| B4 recordings of block B under `e2e/testdata/` would have failed the docs-only rule | A | block B records nothing there; its outputs are under `docs/probes/` (11 11.2) |
| B5 the throwaway pull request and its branch were left behind, and nothing said who may update the default branch | A | the ruleset item restricts updates to the bypass actor, and says that this GitHub behaviour is not tested here; the tries item closes the pull request and deletes its branch (10 10.2) |
| B6 what a download line may hold, for `kits` | A | no `\|`, `;`, `&&` or `$(` on a download line, and the file `-c` reads is a kit file (10 10.2) |
| B7 a failed create that left a half-built sandbox lost its record | A | after a non-zero create romeu reads `sbx ls --json`: present keeps the open record, absent removes it, and both exit 1 (01 1.6, 04 4.2) |
| B8 `tools/ci acceptance` ran the argv of a tracked file on no ask-first surface | A | `docs/acceptance.json` is on the `checks` globs (05 5.3) |
| B-info herdr: who checks that the digest matches the bytes | A | `tools/kitpin` takes the digest without downloading, so the build's `sha256sum -c` is that check, and it fails closed (06 6.4) |

The re-audit then confirmed the five fixes that had to be made. It
closed three and left two partial, each a gap inside the fix. Both are
the re-audit's findings too:

| Finding | Status | Resolution |
|---|---|---|
| M5, confirmation (Medium) results of an older candidate passed `acceptance` beside a newer B1 of the same host | A | every block B result, the sandbox-side checks included, carries the candidate's tag and commit, copied by the harness from the release B1 installed in the same run; results of one host that name different candidates fail. No field is added. The run of B1 on v1.0.0 is not a block B result of a candidate, and the comparison does not read it (11 11.2, 10 10.5) |
| M2, confirmation (Medium) S1's asset list had no evidence kind | L | no kind is added: S1's evidence is the `ci-run` of `release.yml`, whose `success` conclusion is the evidence that the assets exist, and the asset list itself is **[review]** under the acceptance PR's Evidence (index, S1) |

The two rows replace what the M5 and M2 rows above say about B1's
result alone and about the asset list read through the API.

Two notes on what the fixes touch elsewhere:

- M1 adds to round 6's count of what was added: the `README.md`
  skeleton joins the `SECURITY.md` outline in `ci-release`. Round 6 is
  left as written.
- M2 changes one unit test: the S1 case against the recorded API
  fixture gains the `release.yml` run.

## The writer's own decisions

The maintainer has not ruled on these. The next review should look at
each:

- The name of B3's artifact, `docs/probes/B3-<host>-test.json` (A3).
- The candidate's tag and commit are two observations of a result,
  so the `probe-result.v1` format gains no field (M5).
- The run of B1 on v1.0.0 writes outside the repository, so that it
  does not replace the candidate's B1 result, and its output is
  **[review]** text under the acceptance PR's Evidence (M5).
- The "not tested" notes of M4 and B5 are written in the ruleset item
  itself, not as a try of Q25's fallback.
- Two sentences outside the passages the findings name follow M3: the
  residual-risk row of 05 5.4 says "proved on the host", and Q25 says
  "a try that took effect".
- ADR 0001's Consequences list the `checks` globs as the maintainer
  decided them, without `docs/acceptance.json`. That sentence is left
  as written, because the new glob is a finding of the re-audit and
  not a decision of the maintainer (B8).

## What the maintainer has to do

Nothing in this round is done until the maintainer acts on these, in
this order. The list replaces the one of round 6:

1. Sign off the six Proposed records, or decline one (Q24). It comes
   before the plan's first task.
2. Answer Q26.
3. The maintainer block of `ci-bootstrap`
   ([10 10.2](../spec/10-testing-style.md#release-and-bootstrap)), in
   its new order. Item a, the rulesets and the two repository
   settings, comes before the sandbox that writes step 1 receives its
   token. Then read the diff of step 1, and items b and c on the host.
   Then item d: the four tries from inside the sandbox, the four
   checks from the host that prove each refusal, and the throwaway
   pull request closed with its branch deleted. Item e comes after
   step 2.
4. Run probe block A on both hosts.
5. Push the release candidate tag after `ci-release`, run block B on
   both hosts, and push the v1.0.0 tag after `docs`
   ([12 12.2](../spec/12-engineering.md#122-development-commands-and-capability-map)).
6. Run B1 once more, on v1.0.0 and on one host, before the acceptance
   commit ([11 11.2](../spec/11-host-probes.md#112-block-b---acceptance-on-real-hosts-last)).
7. Write what happens next in `SECURITY.md`
   ([12 12.8](../spec/12-engineering.md#128-contributor-docs-outlines)).

## Removed or added

Removed: the sentence that a candidate result counts "when nothing
that is built changed in between"; the gate 1 check at step 9 of
`sync`; "acceptance evidence" in the `docs` row.

Added, counted as the index asks: two rows of the generation machine;
one maintainer step, B1 on v1.0.0; two observations in each block B
result;
one glob on the `checks` surface; the `README.md` skeleton in
`ci-release`. No package, command, file format field or check
subcommand is new.
