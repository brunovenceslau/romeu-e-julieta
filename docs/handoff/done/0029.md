# Session of 2026-10-17, sixteenth part: the second-signal test

- The stack #103 (#101, #102, #104), T129 and its plan and spec fix
  (Done record 28), merged on 2026-10-17.
- The first CI run of #104 failed on macos-26 in
  TestMainSecondSignalAbsorbed of tools/ci: the second SIGTERM ended
  the process. The diff of #104 was docs only; a run after a rebase
  passed.
- The cause: the test's hook (`procgroup.SetBeforeStepAside`) sent the
  second signal to its own process and returned, and the handler then
  called `signal.Stop`. The test assumed a signal a process sends
  itself is delivered before kill returns; when it arrives after the
  Stop, its default action ends the process. Reproduced on linux/arm64
  by sending it from a goroutine: 3 runs of 20 failed with the messages
  of the CI run.
- The fix, approved for the procgroup and checks surfaces on
  2026-10-17: the hook gets the handler's channel and waits, bounded at
  10 s, until the second signal is on it, in tools/ci and in the
  "absorb" helper of internal/procgroup. With the goroutine send kept,
  40 runs of 40 passed. Product code is unchanged: the hook is nil
  outside tests, and the call-site scan still keeps it to test files.
- Gate: code-reviewer and security-auditor, nothing Required; their
  Optional items (the bound on the wait, the comments) applied.
