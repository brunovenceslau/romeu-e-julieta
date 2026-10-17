# Tight wall-clock bounds left in the tests

The stack of 2026-10-17 replaced the two bounds that failed in CI: the
drain of internal/procgroup is now ended by idle pipes and its escaped
holder tests prove the bound by the holder being alive, and
TestMainSecondSignalAbsorbed sends its second signal from inside the
handler. These upper bounds of 5 s or less remain, none of which has
failed yet:

- internal/procgroup/procgroup_test.go: `limit/2` (line 136); `less than
  WaitDelay` after a cancel or a group kill (lines 233, 266, 279); the
  1 s of "a cancellation with a stdin that never ends" (line 253).
- internal/procgroup/errors_test.go:31, 5 s.
- tools/ci/internal/check/step_test.go:149, 5 s.

The two "escape-chatty" subtests of internal/procgroup/procgroup_test.go
failed once on macos-26 for another reason, a holder that died after the
run instead of a bound; fixed on 2026-10-17 (Done record 27).

Bounds of 10 s or more only guard against a hang and stay. For each
bound above, an event (a process still alive, a pipe closed) may
replace the clock without weakening what it proves.

Reopen: any of these tests fails in CI, or the next change to the file
that holds it.
