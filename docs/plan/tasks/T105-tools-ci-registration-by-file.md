---
id: T105
title: "`tools/ci` registration by file"
milestone: M0
module: ci-bootstrap
fate: merged
owns:
  - tools/ci/**
  - tools/mutants/**
  - .github/ask-first.yaml
  - internal/procgroup/callsite_test.go
  - internal/procgroup/class.go
  - internal/procgroup/class_test.go
  - internal/procgroup/lookpath.go
  - internal/procgroup/lookpath_test.go
  - internal/procgroup/pipes.go
  - internal/procgroup/procgroup_test.go
needs: []
after:
  - T121
  - T137
ask_first:
  - checks
  - procgroup
  - approvals
  - ask-first
sitting: none
merged: 2026-10-16
pr: 63
---

# T105 - `tools/ci` registration by file

Merged on 2026-10-16 in pull request #63. The text below is the one
written at the re-plan.

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

Decided on 2026-10-13, when the task started:

- (a) The tests of `tools/ci` and `tools/mutants` stop importing
  `os/exec` through one export of `internal/procgroup`, `LookPath`, a
  documented wrapper over `exec.LookPath`. Their fixtures run through
  `procgroup.Run` and `procgroup.CombinedOutput` with an explicit
  environment, under a class of the table or a limit the test sets
  through `WithTestTimeout`; a signal test runs `Run` in a goroutine,
  the process it signals writes its own pid, the test signals that pid
  with `syscall.Kill` (never a pid at or below 1), and reads the exit
  from `procgroup.ExitError`. The lint test that matched build
  constraints moves from `go/build` to `go/build/constraint` with a
  file-name suffix matcher of its own, so no file under `tools/`
  imports `os/exec`, `os/signal` or `go/build`.
- (b) The classes of the tools are `tool-step` (15 minutes),
  `tool-race` (40 minutes), `tool-git` (2 minutes) and `tool-probe`
  (30 seconds: `uname`, `sysctl`, `mise which`, `mise --version`, `go
  list -find`); `go env` in `tools/mutants` takes the existing `mise`
  class, and the `go` build and test runs of `tools/mutants` take
  `unbounded`. The rows of 04 4.1, Starting values, come first, by a
  pull request that edits that table alone (REQ-1220).
- (c) `tools/mutants` no longer takes SIGHUP: both `main`s install
  `procgroup.Main`, which takes SIGINT and SIGTERM only, and a SIGHUP
  takes its default action, as 04 4.1, Signals, says of every signal
  the handler does not take.
- (d) The task ships as three sequential pull requests: the rows of
  (b) in 04 4.1, the switch to `internal/procgroup` with the removal of
  `tools/ci/procgroup/`, and then the registration by file.

Amended on 2026-11-01 (the release and upgrade amendments, ADR 0044):
REQ-1051 (12 12.1) gained git-cliff; T192 pins it and adds it to
`PinnedTools`.

- Implements: REQ-0006 (index), REQ-0021 (index), REQ-0037 (00 0.4),
  REQ-0040 (01 1.1), REQ-0121 (02 2.1), REQ-0307 (04 4.1), REQ-0473 (05
  5.2), REQ-0770 (10 10.1), REQ-0821 (10 10.2), REQ-0855 (10 10.2),
  REQ-0877 (10 10.2), REQ-0880 (10 10.2), REQ-1040 (12 12.1), REQ-1051
  (12 12.1), REQ-1052 (12 12.1), REQ-1080 (12 12.2), REQ-1081 (12 12.2),
  REQ-1101 (12 12.2), REQ-1104 (12 12.3), REQ-1105 (12 12.3), REQ-1108
  (12 12.3), REQ-1113 (12 12.3), REQ-1117 (12 12.3), REQ-1184 (12 12.8),
  REQ-1187 (12 12.8), REQ-1188 (12 12.8), REQ-1229 (10 10.6), REQ-1234
  (04 4.1).
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
    (`grep -rl '"os/exec"' --include='*_test.go' tools/`), stop doing
    so by a route this task decides when it starts and records in its
    pull request: an export from `internal/procgroup`, `LookPath`,
    which this task's `owns` gains (decided on 2026-10-13, item (a)
    above). `tools/ci/lintconfig_test.go`, the one file under `tools/`
    that imports `go/build` at `b1700c9`, moves to
    `go/build/constraint` (item (a) above).
    `tools/ci/procgroup/procgroup_unix_test.go` leaves with
    `tools/ci/procgroup/`. A tool under `tools/` that the admitted list
    never named, as `tools/new`, reaches git through `tools/ci/git`;
    the switch moves that package to `internal/procgroup`, so the
    route closes with it. The scan stays a test in
    `internal/procgroup` until T039 moves it to `tools/ci callsites`
    (decided on 2026-10-13).
  - Each subprocess of the tools gets a class of the timeout table of
    `internal/procgroup` with the switch (REQ-0300). Their bounds on
    the base, measured at commit `b1700c9`:
    - `tools/ci`: a command step of `fast` or `all`, 15 minutes
      (`stepTimeout`, `tools/ci/fast.go:30`, applied at line 411); the
      race step, 40 minutes (`raceTimeout`, `tools/ci/all.go:40`); a
      git command through `git.Repo.Run`, 2 minutes each
      (`git.Timeout`, `tools/ci/git/git.go:30`, applied at line 55);
      `uname -m` and `sysctl -n sysctl.proc_translated`
      (`tools/ci/all.go:262` and `278`), `mise which`
      (`tools/ci/fast.go:185`) and `mise --version`
      (`tools/ci/misefiles.go:394`), none.
    - `tools/mutants`: a git command through `git.Repo.Run`, 2 minutes
      each; `git pack-objects` piped into `git unpack-objects`
      (`tools/mutants/snapshot.go:151` and `160`), 2 minutes for the
      pair, under one deadline (`snapshot.go:149`); `go env GOROOT`
      and `go env GOVERSION` (`tools/mutants/runner.go:446` and `463`)
      and `go list -find` (`runner.go:387`), none; the build check, `go
      test -exec=true` (`runner.go:402`), none; the baseline `go test`
      (`runner.go:261`, which passes a timeout of 0), none from the
      tool, and go test's default `-timeout` of 10 minutes bounds its
      test binary only; a mutant's `go test` (`runner.go:339`), none
      from the tool, and its `-timeout`, the larger of one minute and
      ten times the baseline's time (`runner.go:282`), bounds the test
      binary, not the compile.
    The classes are a step class, a race class and a git class at
    those values, a probe class for the short probes (`uname`,
    `sysctl`, `mise which`, `mise --version`, `go env`, `go list
    -find`), and, for the `go` build and test runs of `tools/mutants`,
    either an unbounded class or a limit per call taken from the
    baseline. Their rows come by a pull request that edits the Starting
    values table of 04 4.1 alone (REQ-1220) and merges before this
    task's switch; the mutant runs take the unbounded class and the
    probe class 30 seconds (decided on 2026-10-13, item (b) above).
    This task then edits `internal/procgroup/class.go` and its test to
    add them.
  - After the switch, a test proves the second-signal window closed for
    `tools/ci`: a run of `tools/ci` that receives two SIGTERMs while a
    step's group runs exits 143, with the group gone and the second
    signal absorbed (REQ-1234).
  - Green on the four runners.
- Verify: `go run ./tools/ci all` exits 0; `go run ./tools/ci generated`
  exits 0 with `docs/reference/checks.md` present.
