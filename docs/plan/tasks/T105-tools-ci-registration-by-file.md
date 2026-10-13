---
id: T105
title: "`tools/ci` registration by file"
milestone: M0
module: ci-bootstrap
fate: new
owns:
  - tools/ci/**
  - .github/ask-first.yaml
  - internal/procgroup/callsite_test.go
needs: []
after:
  - T121
  - T137
ask_first:
  - checks
  - approvals
  - ask-first
sitting: none
---

# T105 - `tools/ci` registration by file

Goal: Each check moves to `tools/ci/internal/<check>/` with its tests
and testdata; one `check_<check>.go` per check whose `init` registers
the name, the usage line, membership in `fast` and `all`, the order key,
whether the check runs code of the change or judges the tree, and the
generator outputs; the runner sorts by order key; `generate` iterates
the registry and emits `docs/reference/checks.md`, one line per check
(name, what it guards, `fast` or `all`, the requirement id of its row in
10 10.2, a link to the package doc), a generated output that every later
check task regenerates (rule 1 of `docs/plan.md`); `fast_test.go`,
`all_test.go` and `main_test.go` split per check. The `approvals` surface names
`tools/ci/askfirst.go`, so the glob follows the moved package in
`.github/ask-first.yaml`, the one edit this task makes there; T107 then
takes that file. It follows the registration-by-file rule of 12 12.2 as
amended on 2026-10-12 (ADR 0012); `after: T121` for the file overlap on
`tools/ci/main.go`, and it moves `reqs`, coverage half included, like
every check; `after: T137`, so the move switches `tools/ci` and
`tools/mutants` to `internal/procgroup` and removes
`tools/ci/procgroup/`, the smaller diff. The four five-parameter
functions (`whichPinned`, `scanLines`, `snapshot`, `sendPack`) are
reshaped in a structural pull request of their own when their files
move, never here. The lint split with its new lint set is T139 and the
two repository tests over the specification's names are T140, both
`after` this task, so this pull request is one frozen-behaviour move.

- Implements: REQ-0006 (index), REQ-0021 (index), REQ-0037 (00 0.4),
  REQ-0040 (01 1.1), REQ-0121 (02 2.1), REQ-0473 (05 5.2), REQ-0770 (10
  10.1), REQ-0821 (10 10.2), REQ-0855 (10 10.2), REQ-0877 (10 10.2),
  REQ-0880 (10 10.2), REQ-1040 (12 12.1), REQ-1051 (12 12.1), REQ-1052
  (12 12.1), REQ-1080 (12 12.2), REQ-1081 (12 12.2), REQ-1101 (12 12.2),
  REQ-1104 (12 12.3), REQ-1105 (12 12.3), REQ-1108 (12 12.3), REQ-1113
  (12 12.3), REQ-1117 (12 12.3), REQ-1184 (12 12.8), REQ-1187 (12 12.8),
  REQ-1188 (12 12.8), REQ-1229 (10 10.6).
- Operator: no
- Acceptance:
  - The step lists of `fast` and `all` taken before the move equal the
    lists taken after: a one-off comparison under the pull request's
    Evidence, not a committed file; the committed inventory is the
    generated `docs/reference/checks.md` (rule 1), one line per check
    with the requirement id of its row in 10 10.2, checks only and never
    a helper package, and a test fails a registered check whose id is
    not a row of the step table.
  - A test registers a fixture check by adding two paths and edits none;
    a duplicate check name panics at `init`, and a test asserts the
    registry's order and names.
  - Each registration says whether the check runs code of the change
    (`unit`, `race`) or judges the tree; the runner places every
    tree-judging check before the first code-running one whatever the
    order keys say, keeps `generated` before `unit` and `race`, and
    reads the commit once, before mise and before any step, for every
    check that judges the commit; `TestGeneratedRunsBeforeTheTests`,
    `TestCommitChecksReadBeforeTheSteps` and
    `TestCommitChecksReadBeforeMise` stay green, and a fixture check
    registered after `unit` with the tree-judging flag still runs before
    `unit`.
  - The behaviour of every check is unchanged: its tests move with it
    and pass; the hardening is frozen, and a further fix is a pending
    item of T136 unless a real escape is shown.
  - Every `tools/ci` path the specification cites is listed with its
    new path in the pull request body.
  - The packages `tools/mutants` and `tools/new` import stay importable
    from `tools/`: they keep their paths below `tools/ci/` or move
    together to `tools/internal/<name>/` in a pull request of their own;
    the shared fixture helpers of the `tools/ci` tests go to one package
    the check packages import, `tools/ci/internal/citest`; a moved test
    file takes the name of the source file it tests, or, for a
    repository property with no source file, a name that says the
    property.
  - `pinnedTools` moves beside `miseEnv` and `hookAllow` in the mise
    package; the tool resolver and the lint tests read it; its comment
    says `all` starts govulncheck by the path `mise which` resolves, and
    the comment of the ask-first test names the task file that holds
    T005's table.
  - After the move, `tools/ci`'s `main` exports nothing but what the
    dispatcher needs, and revive `exported` reads every check package.
  - `tools/ci/internal/askfirst/**` is on the `approvals` surface after
    the move (T107 forecasts it).
  - The switch to `internal/procgroup` removes the `tools/ci` and
    `tools/mutants` entries from the admitted list of
    `internal/procgroup/callsite_test.go`, which leaves
    `internal/procgroup` alone, and the scan passes on the tree with no
    exception for test files. The `_test.go` files under `tools/ci` and
    `tools/mutants` that import `os/exec`, 13 at commit `b1700c9`
    (`grep -rl '"os/exec"' --include='*_test.go' tools/`), switch with
    the code they test: a command a test runs goes through
    `internal/procgroup` under a class; a signal case that covers the
    shared handler rather than the tool, such as a signal sent to a
    helper binary's pid, is dropped when the tests of
    `internal/procgroup` already cover it, and moves there otherwise;
    and `tools/ci/procgroup/procgroup_unix_test.go` leaves with
    `tools/ci/procgroup/`. The scan stays a test in
    `internal/procgroup` until T039 moves it to `tools/ci callsites`
    (decided on 2026-10-13).
  - The timeout classes of the tools are added to the timeout table of
    `internal/procgroup` with the switch, each at the bound the tool
    has on the base: a command step of `tools/ci` (15 minutes), the
    race run of `all` (40 minutes) and a git query (2 minutes). The
    `go` commands of `tools/mutants` are bounded on the base only by the
    `-timeout` they pass to `go test`, the larger of one minute and ten
    times the baseline's time, which can exceed 15 minutes, so their
    class is no shorter than that `-timeout`.
  - Green on the four runners.
- Verify: `go run ./tools/ci all` exits 0; `go run ./tools/ci generated`
  exits 0 with `docs/reference/checks.md` present.
