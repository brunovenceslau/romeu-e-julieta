# Session of 2026-10-17, fourteenth part: the chatty holder test

- #97, the record of T007, T017 and T023 (Done record 26), merged on
  2026-10-17. Its first CI run failed on macos-26 in
  `TestRunKillsTheGroup`, "a holder outside the group that never goes
  idle makes a run with no deadline an error at the cap": `no such
  process` on the check that the holder is still alive. A re-run of the
  job passed.
- The cause: the "chatter" holder of `internal/procgroup/helper_test.go`
  exited on its first failed write, and a write to a broken standard
  output kills a Go program by SIGPIPE anyway. Once the run returned and
  closed the pipe, the holder died within 10 ms, racing the check.
  Reproduced on linux/arm64 with a 200 ms sleep before the check: both
  "escape-chatty" subtests failed the same way.
- The fix, approved for the procgroup surface on 2026-10-17: the
  chatter ignores SIGPIPE and, once a write fails, blocks until
  `killAtEnd` or `helperLifetime` ends it. With the same sleep both
  subtests pass. Only test files change.
- Gate: code-reviewer and security-auditor, nothing Required; their
  Optional items (the holder blocks instead of retrying, and a
  clearer assert message) applied.
- Decided on 2026-10-17: in a public repository, a CI job that failed
  for a cause outside the diff may be re-run without asking. The rule
  goes to the house kit.
