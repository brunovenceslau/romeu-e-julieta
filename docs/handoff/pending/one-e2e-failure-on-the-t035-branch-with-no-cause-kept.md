# One e2e failure on the T035 branch with no cause kept

What is at stake: small; if a test of the container e2e is flaky, a
later run fails a pull request for a reason outside its diff, and REQ-0799
says a flaky test is fixed, never skipped or re-run away.

Scope: this repository.

On 2026-10-20 one run of `go run ./tools/ci all` on `feat/t035-memstore`
failed its `e2e` step (`FAIL e2e (4.9s)`) right after the builder's pass
ended; only the summary line was kept. Thirteen later runs of the step
passed, and the julieta e2e tests passed 24 times on their own. No race
was found in T035's tests. One gap was found and closed: the 40
concurrent `memory add` processes of `TestJulietaMemory` now form a step
whose exit and stderr the test reads, so a failure there names itself.

Reopen: the `e2e` step fails again, locally or in CI; keep the whole
output of `go test -json` this time.
