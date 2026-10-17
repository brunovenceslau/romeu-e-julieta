# Tests bound by wall-clock time

A test that asserts an upper bound in seconds fails on a slow or loaded
runner without a bug. TestReqsReads was replaced by a byte count in #64.
Left as they are:

- tools/ci/signal_unix_test.go, TestMainSecondSignalAbsorbed: failed
  once on ubuntu-26.04-arm during #64's first CI run, while it passed
  300 runs of 300 locally. The cause is not measured yet; read the
  failing log before changing it.
- internal/procgroup/procgroup_test.go, the subtest "a leader that exits
  0 with a holder outside the group succeeds after WaitDelay": asserts
  `took < WaitDelay+3s`. The same file bounds other subtests by
  WaitDelay and by 6 s. These are on the procgroup ask-first surface.

The lower bounds (at least WaitDelay) prove behavior and stay. For each
upper bound, decide whether an event (the holder still alive, the pipe
closed) can replace the clock, without weakening what it proves.

Reopen: any of these tests fails in CI again, or the next change to
internal/procgroup or tools/ci/signal*.
