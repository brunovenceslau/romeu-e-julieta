# One e2e failure on the T035 branch with no cause kept

What is at stake: small; a flaky test of the container e2e fails a pull
request for a reason outside its diff, and REQ-0799 says a flaky test is
fixed, never skipped or re-run away.

Scope: this repository.

On 2026-10-20 one local run of `go run ./tools/ci all` on
`feat/t035-memstore` failed its `e2e` step with only the summary line
kept. After the concurrent adds of `TestJulietaMemory` were made a step
whose stderr the test reads, the CI run of #204 on Ubuntu failed the
same test and named the cause: `fatal error: newosproc`, the Go runtime
unable to start a thread. The 40 adds all started at once, and the
container runs with `--pids-limit=256`, which counts threads. The fix
runs them two at a time, as the acceptance row of T035 says, and keeps
the limit.

Reopen: the `e2e` step fails again on `TestJulietaMemory`; otherwise
this note goes with the next Done record.
