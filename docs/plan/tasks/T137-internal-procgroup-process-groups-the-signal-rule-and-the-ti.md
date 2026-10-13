---
id: T137
title: "`internal/procgroup`: process groups, the signal rule and the timeout table"
milestone: M0
module: procgroup
fate: new
owns:
  - internal/procgroup/**
needs: []
after: []
ask_first:
  - procgroup
sitting: none
---

# T137 - `internal/procgroup`: process groups, the signal rule and the timeout table

Goal: `tools/ci/procgroup` moves to `internal/procgroup`, the one
package that starts a subprocess for both binaries, `tools/ci` and
`tools/mutants` (04 4.1, Subprocesses): a process group per child, the
kill of the group on a timeout, a cancellation or a signal, with a
bounded drain of the pipes; the one signal handler both `main`s install
(04 4.1, Signals); and the timeout table of 04 4.1 Starting values as
one Go table keyed by class with a seam only tests set. `tools/ci` and
`tools/mutants` keep importing `tools/ci/procgroup` until T105, which
comes `after` this task, switches them to `internal/procgroup` and
removes `tools/ci/procgroup/`, under its own `tools/ci/**`; until
then the two copies are the dark code of 12 12.9. 10 10.7's row and 12
12.2's `procgroup` row are T122's text; T010's `imports` check reads
them. T009, T023, T029 and T053 need this task.

Re-plan (2026-10-12, stage B): New, an M0 task: both binaries take their
signal rule and their subprocess runner from this package at their first
version, so daily use does not wait for it. Where two proposed timeout
tables differed, the longer value is the starting value (a first daily
use suffers a timeout more than a wait).

- Implements: REQ-0274 (04 4.1), REQ-0298 (04 4.1), REQ-0299 (04 4.1),
  REQ-0301 (04 4.1), REQ-0302 (04 4.1), REQ-0303 (04 4.1), REQ-0304 (04
  4.1), REQ-0305 (04 4.1), REQ-0307 (04 4.1), REQ-0500 (05 5.2),
  REQ-0958 (10 10.6), REQ-0969 (10 10.7), REQ-1086 (12 12.2), REQ-1220
  (04 4.1), REQ-1234 (04 4.1), REQ-1235 (04 4.1).
- Operator: no
- Acceptance:
  - A command starts in its own process group; a cancellation or a
    timeout kills the group and the drain after the kill is bounded; the
    case "a grandchild holding the pipe is dead after the step returns"
    (69725ae0, item 1: a leader that exits while a grandchild holds the
    output pipe) passes on Linux and macOS.
  - The call-site scan fails an `os/exec` call outside
    `internal/procgroup`, beside the I16 scan of T039, with a fixture
    package that calls `exec.Command`; `tools/ci`, `tools/mutants` and
    both binaries import `procgroup` and nothing else imports `os/exec`.
  - The handler: a helper binary run from the test receives SIGTERM
    while a child of its own group sleeps; it exits 143, the child is
    gone and one line names the interruption (the case "a signal ends
    the group and exits 143"); a command that had already failed with 2
    exits 143 when the signal lands before it returns; a command that
    finished keeps 0. The handler kills every tracked group
    synchronously before it steps aside and refuses new starts; a
    second signal before it steps aside is absorbed and the status
    stays 128 plus the first signal's number (the case "a second signal
    before the handler steps aside is absorbed"); after it steps aside
    a signal takes its default action (the case "a signal after the
    handler steps aside takes its default action").
  - The timeout table is one Go table keyed by class; a test shortens
    one class through the seam and proves `subprocess-timeout`
    (`RJ-323`) fires with details that name the class and its limit; the
    salvage class has no timeout.
  - `procgroup.KillGroup` refuses a pid at or below 1 (70e30de0, item
    4).
  - The step lists of `fast` and `all` before and after the move are
    equal (the one-off comparison of T105's form, under Evidence).
- Verify: `go test ./internal/procgroup/... ./tools/...`; `go run
  ./tools/ci all` exits 0.
