# Session of 2026-10-09, after the write pass (migrated from docs/reviews/round-8.md)

The write pass of #25 ran, the ship gate found work for the next pass,
and the tooling pull requests were merged. The write pass is described
in `docs/reviews/round-8/write-pass.md` and the gate's findings are in
`docs/reviews/round-8/ship-gate-2026-10-09.md`.

- Decided on 2026-10-09:
  - Tooling pull requests: work went on autonomously for about three
    hours; the pull requests were to be stacked GitHub pull requests,
    checked for how they integrate, so that they can be approved
    together with `gh`; pushing and opening or editing the pull requests
    the work needs did not wait for an answer. No stack was needed:
    AR13 was folded into #25, and DR3 and DR7 were independent.
  - Stacked pull requests use `gh stack`, with the `--base` that gives
    the web UI its stack support, once there is a real dependency, after
    verifying the signing of its rebases and the retarget race. The
    amendment of the house payload rule that every pull request in a
    stack targets `main` is deferred, with the first real dependency as
    its trigger; then the `gh stack` research gets adversarial
    verification first, and the amendment, a payload pull request on an
    ask-first surface, follows from what it confirms. The spec holds the
    matching row in its Deferred decisions (docs/spec.md, stacked pull
    requests).
  - The new mechanism text of 06 6.4 (hooks call julieta by the absolute
    `.romeu/bin` path, passed as a kit arg) is accepted with the F12 fix
    that makes the kit arg reserved and quoted; page 13 is marked
    designed, not built, with the ledger risks in 13 13.11. R8-05-30,
    `init` with a non-empty root, was left unspecified (pending
    "init-with-a-non-empty-root-is-unspecified").
  - Ship gate F1, confirmed: the pull request that adds the `spec`
    surface carries three approval lines, on `ask-first`, `spec` and
    `decisions`.
  - Round 9 runs after the write pass.
  - F8: the weakened invariant is accepted; a protective command goes on
    past a failed gate 1, gate 2 or step 5 check, and 05 5.4 holds the
    row (R8-01-16).
  - AR13 (the `spec` surface) landed in #25 and not in a tooling pull
    request, because `TestSpecListIsTheCommittedList` requires the YAML
    fence in 05 5.3 to equal `.github/ask-first.yaml`.
  - Answered on the same day: DR3 takes option (a), `GOENV=off`,
    `GOWORK=off` and `GOFLAGS=-mod=readonly` (draft #34); the write
    pass design calls are accepted (`kits/pins.yaml`, exit code 6,
    RJ-301 renamed and RJ-302 to RJ-328); the `sbxdrv` surface and
    `skills/**` on `kits` go in a small tooling pull request after #25;
    the ADR 0007 half rides in ADR 0009 as decision 21; smoke step 2 is
    at C7 with T103 in phase 4 (F2); the token-narrowing trigger is
    reworded (F6); `docs/grants.yaml` sits on the `ask-first` surface
    (F7).
  - G5, an `approvals` surface: option (a), which amends AR14. Re-audits
    widened it: `pr` runs the base commit's code, `.github/workflows/**`
    joined `approvals`, `pr` runs in a base-defined job on
    `pull_request_target`, read-only and without secrets, pinned to the
    default branch with `branches: [main]`; the head is never built or
    run. A private repository is the trigger to revisit the
    unauthenticated head fetch. The mechanism is acceptance (a) to (d)
    of T005.
  - The CI flake of #27 (a runner DNS error during setup) is a pending
    tooling item, see pending
    "harden-the-setup-download-against-transient-failures".
  - AR22 stays with #33, which read that the default-branch ruleset no
    longer restricts updates.
- Merged: #22 (2026-10-08), #26 (ship gate GO, CI green) and #27 (`gh`
  2.102.0 locked in `mise.lock` and the `go.mod` equals `mise.toml` test;
  ship gate GO; one macos-intel leg re-run after a DNS error). Main then
  moved by #28 to #30 and #33.
- The write pass on the #25 branch: six sequential opus groups, each
  with `fast`, `all` and `hygiene` green, and ADR 0009 ("Drop the
  required review and amend the records that assumed it"). The security
  log research of DR5 was verified against github/docs at 9f65179: a
  personal account keeps 90 days and records ruleset changes, visibility
  and listed settings changes, and no merge, push, tag, branch deletion
  or release change (05 5.4).
- DR3 is on the branch `fix/pre-push-go-env`, draft #34; its gate found
  two Mediums (an empty `GOFLAGS` falls back to the `go env -w` file, an
  untracked `vendor/` makes `go run` default to `-mod=vendor`) and left
  S1 (a `GOFLAGS` in the `[env]` table of a global mise configuration).
- The ship gate of #25: NO-GO in round 1 (F1 to F23), one write pass,
  round 2 found G1 to G12, one more pass applied all but G5.
- Planned then: the targeted re-audit, the rewrite of the #25 body,
  round 9 (Done record 15), draft #34, the payload pull request for the
  stack rule (deferred above), the R8-05-3 surface pull request (T107,
  #75, Done record 9) and the stage-B items of `round-8/write-pass.md`
  (taken by the stage-B pass, Done record 21).
