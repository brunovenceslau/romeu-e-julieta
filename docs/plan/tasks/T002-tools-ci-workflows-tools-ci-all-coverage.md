---
id: T002
title: "`tools/ci workflows`, `tools/ci all`, `coverage` and a green `ci.yml`"
milestone: M0
module: ci-bootstrap
fate: merged
owns: []
needs:
  - T001
after: []
ask_first: []
sitting: O1
merged: 2026-10-08
pr: 16
---

# T002 - `tools/ci workflows`, `tools/ci all`, `coverage` and a green `ci.yml`

Merged on 2026-10-08 in pull request #16. The text below is the one
written before the re-plan.

- Implements: 10 10.2 (step 2; the workflow grammar; Runners; the
  `coverage` step), S2, S9, S11.
- Operator: block O2 follows its green run.
- Ask-first, as forecast before the re-plan: none exists yet.
- Acceptance:
  - `tools/ci workflows` fails a fixture workflow for each construct
    outside the grammar of 10 10.2 (an `if`, a `shell`, an expression
    with an operator, a `uses` without a 40-hex SHA, a `run` line that
    is not `go run ./tools/ci` or `go run ./tools/release`, a file with
    another name ending), and fails a `ci.yml` without `edited`.
  - Hardening after the merge (decided on 2026-10-08): the
    grammar is made of allowlists (the four runner labels, `with` keys
    per action with literal values, `permissions` of `read` or `none`,
    no `github.token` and no `secrets.*`, `persist-credentials: false`
    on a checkout, a closed `strategy`, keys tagged `!!str`), and
    `tools/ci` runs `govulncheck`, `golangci-lint` and `go` by the
    path `mise which` resolves, at the version `mise.lock` locks, with
    a step timeout that names its limit, a 40-minute budget for the
    race run, and an architecture verdict (`uname -m` against the Go
    binary).
  - `go run ./tools/ci all` runs the steps of 10 10.2 that have code to
    check at this point: format and vet, lint, unit, hygiene, workflows,
    `govulncheck`, the race run with its cover profile, `coverage`,
    `reuse lint`. The lint row of this task moved to the change that
    adopted testify in tests ([ADR 0007, adopt testify assert and require in tests](../../adr/0007-adopt-testify-assert-and-require-in-tests.md)):
    that change added `.golangci.yml`, the fixture test of ADR 0001 rule
    14 and the lint step of `go run ./tools/ci fast`, so `all` only has
    to run the step that `fast` already runs. `ci.yml` runs `go run
    ./tools/ci setup`, which runs `mise trust`, before `go run ./tools/ci
    all`: the steps of `tools/ci` keep
    only the variables that say where things are, so neither `CI` nor
    `MISE_TRUSTED_CONFIG_PATHS` reaches `mise which`, and it sets
    neither `MISE_DATA_DIR` nor `XDG_DATA_HOME`, which `tools/ci`
    refuses.
  - `tools/ci coverage` enforces the two thresholds of S9 and reports
    the packages below them. It lands here because `tools/...` is its
    first input.
  - `ci.yml` passes on `ubuntu-26.04`, `ubuntu-26.04-arm`,
    `macos-26` and `macos-26-intel`; the first run verifies that the
    arm64 and Intel labels exist and that `uname -m` reports `x86_64`
    on `ubuntu-26.04`, `aarch64` on `ubuntu-26.04-arm`, `arm64` on
    `macos-26` and `x86_64` on `macos-26-intel` (10 10.2, Runners).
  - `tools/ci workflows` has a fixture that fails a `runs-on` with a
    `*-latest` label, so that "never `*-latest`" is a check and not a
    habit (10 10.2, Runners).
  - How mise and Go reach each runner is one `uses` step pinned to a
    40-hex commit SHA, with the mise version and its sha256 in `with`;
    Go and every tool of `mise.lock` come through that mise. No step
    downloads anything unpinned, and the grammar of 10 10.2 admits no
    `curl | sh` line. The row is in "Toolchain pins".
  - `tools/ci coverage` has a fixture cover profile per threshold of S9:
    a package under 80, a package of the 90 list under 90, and both
    passing cases.
- Verify: `go test ./tools/ci/... -run '^Test(Workflows|Coverage)'` runs
  the fixture tests named here; the run id of the green workflow, under
  the pull request's Evidence.
