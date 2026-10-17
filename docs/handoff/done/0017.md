# Session of 2026-10-09, end of session (migrated from docs/reviews/round-11.md)

- Decided on 2026-10-09:
  - Every recommendation of the open block of the round-8 checkpoint was
    accepted, with one change to G5: the `approvals` surface was widened
    in four steps, each after a must-fix security re-audit. The end
    state is that `tools/ci pr` runs from a workflow defined in the
    default branch (`pull_request_target`, `branches: [main]`,
    `github.sha`, `contents: read`, no secret) and reads the head only
    as data. From round 11 on the spec states it as an invariant and the
    mechanism is acceptance (a) to (d) of T005.
  - #34 (DR3): option (a), then S1, were fixed. After four audit rounds,
    each of which found another caller-set mise variable (N1
    `MISE_ENV_FILE`, N3 `MISE_CD`, N4 `MISE_TRUSTED_CONFIG_PATHS`, and
    `mise exec` trusting a parent `mise.toml`), the hook and every mise
    run of `tools/ci` moved to an environment allowlist, which also
    closed N2 (`MISE_SYSTEM_CONFIG_FILE`).
  - D2 to D6 of round 9 were decided as recommended, and AR22 was closed
    (#33 measured the ruleset). D2, D4 and D6 (token narrowing, required
    checks without the administrator bypass, direct-push refusal) were
    decided by delegation and recorded in
    [ADR 0011](../../adr/0011-keep-the-administrator-s-pull-request-bypass-and-give-the-sandbox-a-github-app-token.md).
  - The golangci-lint flake was fixed in #39 with
    `--allow-parallel-runners`, plus `-j 1` on request. Measured: `-j 1`
    does not fix the lock; a cold run takes about 14 s against about 7 s;
    it is kept to limit CPU use when runs overlap (docs/spec/10-testing-style.md).
  - A pull request leaves draft when its gates are done and its hosted CI
    is green.
- Rounds 10 and 11 ran.
- #25 merged (ship gate rounds 1 to 3 plus five G5 security re-audits GO,
  CI green at 3f29a9c). #37 merged (gate GO, CI green at 3fda471).
- #36: the round-9 write pass (gate GO in round 3 at dc76fec), then the
  decisions of 2026-10-09 and ADR 0010 (d458f83), the round-10 write
  pass (d133542) and the round-11 write pass plus ADR 0011 (6fd6e60), each
  re-audited by the next lens round. No ship gate had run on the commits
  after dc76fec.
- #39 at 439e406: ship gate GO in round 2. #34 at edda3e3: the allowlist
  redesign; every repro (escape 2, S1, N1, N2, N3, N4) was blocked with a
  real push; a full ship gate and hosted CI were running at handoff.
- A finding of the day: the sandbox token was a classic token with scopes
  `gist, read:org, repo, workflow`, read from a response header, that
  reaches every repository of the account, private ones included. ADR
  0011 replaces it with a GitHub App installation token.
- The host and GitHub steps listed then (remove the administrator from
  the bypass list, a fine-grained token, stop forwarding the SSH key,
  re-run probe A16 and the tries): two were superseded by ADR 0011 as
  decided again on 2026-10-12 (Done record 20), since the administrator
  stays in the bypass list and an App token replaces the classic one in
  place of a fine-grained token. Stopping the forwarded GitHub SSH key
  and re-running probe A16 and the tries of item d after the narrowing
  are still held by ADR 0011 decision 2 and plan block O8.
- Pending items listed then: the mise `HOME`, `PATH` and Go cache
  pass-throughs of the allowlist and the shared-machine runs of
  `tools/ci` (pending
  "the-allowlist-pass-throughs-and-tools-ci-on-a-shared-machine"), and
  the `tools/lenses` script and report check (pending
  "the-lens-page-tooling-and-the-atam-row").
- Planned then: round 12, a ship gate on #36 after dc76fec, the gates
  and CI of #34 and #39, the R8-05-3 pull request after #36 (T107, #75),
  and the stage-B items. #39, #34 and #36 merged since.
