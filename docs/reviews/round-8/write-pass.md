# Write pass of review round 8, 2026-10-09

How the write pass on PR #25 ran, what each group applied, the calls the
groups made, and what is left for later. The Status column of
`docs/reviews/round-8.md` stays the record of each cluster; this note
records the pass.

## How it ran

Six build nodes (opus) ran one after another, never at the same time, each
in the same scratch clone of branch `docs/review-round-8`. A group read the
carry-over of the groups before it, applied the clusters of its pages whose
Status is A, M or L in the form the Status column states, applied the
decisions of 2026-10-08 that land on its pages, skipped the
clusters parked for stage B, and re-read the clusters that cite pages 01,
05, 10, 12 and 13 against the text PRs #23 and #24 had changed. No group
edited `.github/workflows/**`, `tools/**`, `.githooks/**`, `go.mod` or
`mise.*`; the one ask-first edit, `.github/ask-first.yaml`, is the `spec`
surface described under group 4. Each group committed signed and ran
`go run ./tools/ci fast`, `go run ./tools/ci all` and
`go run ./tools/ci hygiene`; all three were green for every commit.

| Group | Pages | Subject of its commit |
|---|---|---|
| 1 | index, 00, 09, 11 | docs(spec): apply the round-8 resolutions to the index, 00, 09 and 11 |
| 2 | 01, 02, 03 | docs(spec): apply the round-8 resolutions to 01, 02 and 03 |
| 3 | 04, 06, 07 | docs(spec): apply the round-8 resolutions to 04, 06 and 07 |
| 4 | 05, 08, 13 and the `spec` surface | docs(spec): apply the round-8 resolutions to 05, 08 and 13 |
| 5 | 10, 12 | docs(spec): apply the round-8 resolutions to 10, 11 and 12 |
| 6 | docs/adr, plan | docs(adr): drop the required review and amend the records that assumed it; docs(spec): apply the round-8 record decisions to 05, 12, the index and the plan |

## Applied, by group

The commit bodies list every cluster id; this table only gives the shape.

- Group 1. Index clusters R8-index-1 to 48 (the parts landing on the index
  and 09), R8-00-4 to 15 (minus the ones owned elsewhere), R8-09-1 to 11,
  R8-11-2 to 16; decisions SC1, SC2, SC3, SC6, SC7, SC11, SC15,
  SC16, SC17, DR2, DR4, DR5, AR12. The delivery-metrics analysis of 00 0.6
  moved to `docs/notes/dora-roi-report.md`.
- Group 2. R8-01 (about 30 clusters), R8-02 (about 20), R8-03 (about 30),
  plus the inbound R8-00-13, R8-09-6, R8-09-1, R8-index-27 and 45;
  decisions SC1, SC2, SC3, SC5, SC8, AR1, AR2, AR3, AR13. SC4 was
  kept, as decided, so nothing of it was applied.
- Group 3. R8-04 (about 35), R8-06 (about 20), R8-07 (10) plus the inbound
  items from groups 1 and 2; decisions SC1, SC2, SC3, SC5, SC7,
  SC10, AR15, AR16, AR18, AR23. Page 04 gained a general "Error ids"
  section (4.4) and the sequence RJ-302 to RJ-328.
- Group 4. R8-05 (page 05 rewritten around one invariants table and a 5.4
  split into the risks of running romeu and of how this repository is
  developed), R8-08 (all), R8-13 (page marked designed, not built); AR1
  to AR12, AR15, AR17, AR18, AR21, AR22, DR2, DR5 record parts. The `spec`
  surface (AR13) was added to `.github/ask-first.yaml` and the 05 5.3
  fence together, because `TestSpecListIsTheCommittedList` requires the two
  copies to be equal; CODEOWNERS and `docs/reference/ask-first.md` were
  regenerated.
- Group 5. R8-10 (about 48), R8-12 (about 38), the 11 carry items; the
  delivery-metrics design and the dated maintainer-block results moved,
  word for word, to `docs/notes/delivery-metrics-design.md` and
  `docs/notes/maintainer-block-of-ci-bootstrap.md`.
- Group 6. ADR 0009, "Drop the required review and amend the records that
  assumed it" (decision DR1, with the riders accepted on 2026-10-08), and
  the "Superseded in part by" line on ADRs 0001, 0002, 0003, 0005, 0007 and
  0008; the plan items of SC1, SC2, SC3, SC5, SC10, SC12, SC13, SC14,
  SC15, the questions Q26 to Q31 and the lessons path; the documented
  security-log coverage (DR5) in 05 5.4 and the index token row.

## Design calls made by the groups

These go beyond the cluster text; the ship gate checked them (answers in
`ship-gate-2026-10-09.md`).

- The pin file is named `kits/pins.yaml` (group 2; grounded by R8-02-9,
  R8-06-25, R8-12-34, which ask for "the pin file").
- A new exit code 6 means "the check ran and found something" (doctor,
  `lock --check`, `pin check`, `spec validate --catalog`); RJ-301 got the
  slug `project-approval-required` and the new ids run RJ-302 to RJ-328
  (group 3). Exit codes and error ids are the `contracts` surface once code
  exists.
- 06 6.4 says the hooks call julieta by the absolute `.romeu/bin` path,
  passed as a kit argument (group 4; the path is grounded by R8-08-5, the
  "kit arg" form is the group's, see F12).
- Page 13 is marked designed, not built, with the ledger risks in a new
  13 13.11 (group 4).
- AR13 landed in this PR rather than a tooling PR (orchestrator decision,
  because the two copies of the surface list must be equal in one commit).
- DR3 is described as pending, in PR #34, in ADR 0009 and the plan,
  never as closed.

## Resolved by main, or not applicable

- R8-00-11 (00 0.2 pointer, J1 agent login): already present on the branch.
- R8-01-31, R8-03-27: moot, the quoted message and import/verify left with
  SC5 and SC2.
- R8-03-30: the ledger field is moot; the reason left with SC1.
- R8-01-15, R8-02-16, R8-02-19, R8-03-21: land on 06 and 12 only.
- R8-01-7: applied without the header-equality Deferred row, which
  R8-03-3 drops.
- R8-04-24 keeps only the dispatcher's stage line and R8-04-36 drops the
  caFile part, as the regression check amended them; R8-03-13 keeps caFile.
- R8-05-30 (init with a non-empty root): not applied, unspecified.
- R8-05-3: only the `spec` surface was applied; `sbxdrv` and `skills/**`
  need their own approval line (see the leftovers).
- R8-05-35: the ADR 0007 Consequences half was not applied by the
  groups; it landed on 2026-10-09 as decision 21 of ADR 0009, under the
  extended `decisions` approval line.
- R8-plan-3 is a finding on the index and ADR 0007, routed to DR3.
- 33 of the 34 plan clusters are parked for stage B and carry no change.

## Left for stage B, tooling PRs or later

Collected from the groups' carry-over notes; one line each. The list was
not re-measured against the final tree, so a line may already be resolved:
check it before acting. The ship gate's findings F1 to F23 are separate and
are in `ship-gate-2026-10-09.md`.

### Stage B (plan delta and re-planning)

- Module ids changed by 12 12.2 (`cli`, `kitpin`) are not re-planned in
  T008, T012, T052, the phase table or "pieces that land early" (group 6 report).
- T020 still tests the old 11 11.4 lifecycle fixtures (group 6 report).
- J14 and I34 have no traceability rows in the plan (R8-06-12 for I34).
- Resolved on 2026-10-09 by the ship-gate fixes (F2): smoke step 2 at C6
  needed `tools/release build --dry-run`, which T088 landed in phase 8;
  the new T103 lands it in phase 4 and step 2 moved to C7 (R8-11-6).
- Blocks O3 and O6 name no `--require-pass` for A15 and A16, and no plan
  task defines either probe (ship gate F15).
- Plan tasks for the new surfaces: `approve <name>` leaves, memory
  import/verify, ledger commands, `--timings` (SC1, SC2, SC3, SC5).
- The plan line citing `RJ-<exit><nn>`, the line citing 06 6.6 and the
  line citing 04 4.4 for `event add` (group 3 report).
- The index page table still says 06 holds the "publishing path"
  (R8-index-4).
- ADR 0008 narrowing trigger "the first issue or PR from an account other
  than the operator's" and its scope line, in DR1's record (AR4, AR6).
- The index Deferred row for DR3 is not added; the hook change is
  pending, in PR #34 (DR3), in the form decided on 2026-10-09.
- `tools/ci pr` reads checkpoint grants from `docs/grants.yaml` at the
  base commit (12 12.4; ship gate F7, decided on 2026-10-09). The file
  lands with that change, on the `ask-first` surface that already lists
  it; nothing reads it before then.
- `tools/ci pr` and 12 12.4 aligned with the house approval checklist:
  one `> [!IMPORTANT]` alert whose `> - [ ] Approval: ...` lines are
  written unticked and ticked as the grant (surface `checks`).
- With the task that builds `pr` (ship gate round 2, G10): a
  `grant: checkpoint` field per surface in `.github/ask-first.yaml`, so
  the list of surfaces a checkpoint grant may cover is data, not prose.
- With the task that builds `pr` (G11): the approval line written as an
  ABNF, and a grant tied to a diff and given an expiry.
- With the task that builds `pr` (G12): the tests TestGrantsFile and
  TestPRApprovalLine, a test that every non-glob path of the ask-first
  list exists, and a `grantsPath` constant beside `askFirstPath`; and a
  test that `pr` refuses a checkpoint grant for the `approvals` surface
  (G5, decided on 2026-10-09); a test that a pull request that changes
  `tools/ci` is judged by the base commit's `tools/ci`; and the
  security-auditor's Low of the G5 re-audit, that nothing in code holds
  the grantable set (G10 closes it).

### Tooling pull requests (each ask-first, with its own approval line)

- `sbxdrv` surface (`internal/sbxdrv/**`, `e2e/fakesbx/**`) and `skills/**`
  on `kits`; the `ledger` surface stays although the ledger is deferred
  and is a candidate to drop (R8-05-3).
- Pre-push hook environment: `GOWORK=off` and `GOFLAGS=-mod=readonly`
  added to the `GOENV=off` and `GOTOOLCHAIN=local` the hook has set
  since PR #30 (DR3, the form decided on 2026-10-09); pending, in PR #34.
- Setup env `GOPROXY` and `GOSUMDB` (R8-10-38).
- `tools/ci` checks: one 00 0.5 row per command and every Deferred table;
  probes and acceptance rules of 11 11.3 and 11.4 (group 1 report).
- The never-tracked list in `tools/ci/denylist.yaml` replacing the
  hard-coded PROLOGUE.md (R8-10-34).
- Release grammar row in `tools/ci workflows` (R8-10-1).
- The generated ask-first page sentence on outside contributors
  (R8-12-18).
- `tools/ci` checks that the ADR 0001 rule 9 matcher runs over the spec
  (R8-05-37).
- `tools/ci setup` hardened against a transient DNS failure while it
  downloads modules (seen once on macos-intel, on `proxy.golang.org`),
  by a bounded retry and never by skipping the step (surface `checks`).
- `pinnedLintTools` nested-worktree fix, pending until the first task that
  edits `tools/ci/fast_test.go`.

### Later, by decision or trigger

- The 04 4.4 id list gives way to the table of 4.1 when T008 lands
  (R8-04-38; ship gate F13).
- The `ledger` surface is a candidate to drop while the ledger is deferred
  (SC1; see the tooling list).

## gh-stack research (not adversarially verified)

A cheap research node read the `github/gh-stack` repository and the GitHub
documentation on stacked pull requests on 2026-10-09. Nothing below was
checked by a fresh context briefed to refute it, so none of it may enter a
rule, an ADR or the spec until it is.

- Latest release v0.2.0 (tag d4ab7ab); pin with
  `gh extension install github/gh-stack --pin v0.2.0`.
- Bases: the bottom pull request targets the trunk, every other one targets
  the branch below it. `gh stack submit` creates or updates the pull
  requests and fixes their bases. GitHub also has native stacks, with a
  banner in the web UI; rules and CODEOWNERS are evaluated against the
  stack base.
- Merge: merging the top pull request lands the whole stack, and GitHub
  retargets the next unmerged pull request to the stack base. The order of
  that retarget against branch auto-delete is undocumented, so the closure
  race is unverified.
- Signing: `gh stack` runs plain `git rebase` and `cherry-pick` and follows
  the local signing configuration (unverified); the documentation disagrees
  on whether stack rebases done in the GitHub UI are signed.
- Rulesets: merges go through the merge API with a `merge_method`; a
  merge-only repository works with `--merge-method merge`.

Orchestrator decision: no stack was needed this session (AR13 folded into
#25; DR3 and DR7 were independent of it). `gh stack` is used at the first
real dependency between pull requests, after verifying the signing and the
retarget race adversarially.
