# TestMutantsSecondSignal failed once under the full race run

What is at stake: low. A flaky test in the pre-push hook costs a
retried push; if it hides a real race in the signal handling of
`tools/mutants`, an interrupted mutation run could leave a mutant
applied.

Scope: this repository.

On 2026-10-25 the pre-push hook of a docs-only branch failed in `race`
with `--- FAIL: TestMutantsSecondSignal (0.72s)` in `tools/mutants`; the
same tree had passed `go run ./tools/ci all` minutes before, and
`go test -race -count=8 -run TestMutantsSecondSignal ./tools/mutants/`
then passed. The failure message was not captured.

Next time it fails: keep the whole output of the step, then run it
under load (`-count=50` beside a full `go run ./tools/ci race`) to
reproduce, and fix the test or the code; do not retry it away.

Reopen: the test fails again.
