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

- Implements: REQ-0007 (index), REQ-0014 (index), REQ-0016 (index),
  REQ-0021 (index), REQ-0022 (index), REQ-0263 (03 3.13), REQ-0264 (03
  3.13), REQ-0265 (03 3.13), REQ-0266 (03 3.13), REQ-0542 (05 5.3),
  REQ-0731 (10 (opening)), REQ-0768 (10 10.1), REQ-0800 (10 10.2),
  REQ-0801 (10 10.2), REQ-0802 (10 10.2), REQ-0803 (10 10.2), REQ-0804
  (10 10.2), REQ-0806 (10 10.2), REQ-0807 (10 10.2), REQ-0808 (10 10.2),
  REQ-0809 (10 10.2), REQ-0810 (10 10.2), REQ-0811 (10 10.2), REQ-0813
  (10 10.2), REQ-0814 (10 10.2), REQ-0820 (10 10.2), REQ-0823 (10 10.2),
  REQ-0828 (10 10.2), REQ-0831 (10 10.2), REQ-0833
  (10 10.2), REQ-0834 (10 10.2), REQ-0840 (10 10.2), REQ-0841 (10 10.2),
  REQ-0847 (10 10.2), REQ-0848 (10 10.2), REQ-0849 (10 10.2), REQ-0851
  (10 10.2), REQ-0856 (10 10.2), REQ-0857 (10 10.2), REQ-0859 (10 10.2),
  REQ-0860 (10 10.2), REQ-0861 (10 10.2), REQ-0862 (10 10.2), REQ-0863
  (10 10.2), REQ-0864 (10 10.2), REQ-0865 (10 10.2), REQ-0868 (10 10.2),
  REQ-0869 (10 10.2), REQ-0876 (10 10.2), REQ-0878 (10 10.2), REQ-0879
  (10 10.2), REQ-0881 (10 10.2), REQ-0883 (10 10.2), REQ-0898 (10 10.2),
  REQ-0905 (10 10.2), REQ-0908 (10 10.2), REQ-0910 (10 10.2), REQ-0914
  (10 10.2), REQ-0915 (10 10.2), REQ-0916 (10 10.2), REQ-0917 (10 10.2),
  REQ-0918 (10 10.2), REQ-0919 (10 10.2), REQ-0920 (10 10.2), REQ-0921
  (10 10.2), REQ-0922 (10 10.2), REQ-0962 (10 10.6), REQ-0973 (10 10.8),
  REQ-0974 (10 10.8), REQ-0975 (10 10.8), REQ-1037 (12 12.1), REQ-1040
  (12 12.1), REQ-1051 (12 12.1), REQ-1053 (12 12.1), REQ-1054 (12 12.1),
  REQ-1055 (12 12.1), REQ-1061 (12 12.1), REQ-1062 (12 12.1), REQ-1063
  (12 12.1), REQ-1065 (12 12.1), REQ-1066 (12 12.1), REQ-1071 (12 12.1),
  REQ-1083 (12 12.2), REQ-1129 (12 12.4), REQ-1140 (12 12.4), REQ-1141
  (12 12.4), REQ-1143 (12 12.4), REQ-1190 (12 12.9), REQ-1191 (12 12.9),
  REQ-1192 (12 12.9), REQ-1224 (10 10.2), REQ-1225 (10 10.2), REQ-1230
  (10 10.6), REQ-1231 (10 10.6), REQ-1241 (10 10.2).
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
    check at this point: format and vet, lint, hygiene, workflows,
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

Re-plan (2026-10-21): changed by T156. The row of REQ-0833 states why the race run keeps `-count=1` (a test lists the tree through `git ls-files`, a subprocess the test cache does not record); the race and e2e argv come from `race.Argv`.

Re-plan (2026-10-21): changed by T157. REQ-0824 (the `unit` step) is retired: the race run, REQ-0833, is the one run of the tests, in `fast` and in `all`. `ci.yml` runs the jobs `static`, `race` and `e2e` with two aggregators in place of the one job that ran `all`, and the grammar admits the aggregator's `if` and its step `exit 1` (REQ-0800, REQ-0803, REQ-0806, REQ-0809); REQ-0856 and REQ-0860 name the race step in place of `unit`.
