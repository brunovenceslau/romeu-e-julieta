# A data race in the hooks run test under load

What is at stake: small. The race step of `all` can fail on a change that
does not touch the code, as it did once on 2026-10-20 on a docs-only
commit (5081c41d), so a red run can be read as noise and re-run instead
of fixed.

Scope: this repository.

Seen on 2026-10-20: `go run ./tools/ci all` failed `race` with "race
detected during execution of test" in
`TestRunEndsWithTheStatusOfTheStageThatFailed`
(`internal/cmd/julieta/hooks/run_test.go:72`). Both racing writes are
`strings.(*Reader).WriteTo` from the stdin copy goroutine of
`procgroup.(*pipes).in` (`internal/procgroup/pipes.go:76-77`), started by
two `procgroup.Run` calls through `internal/hooks.Run`
(`internal/hooks/hooks.go:178`).

Cause, read from the code and not reproduced: the test gives one
`strings.NewReader` to every `julieta hooks run` of a sandbox
(`run_test.go:40`), and the test runs two. `procgroup.Run` does not wait
for its stdin copy (`pipes.go:60-62`), so the copy of the first run can
still read the reader when the second run starts. In production stdin is
an `*os.File`, which goes to the child with no copy (`pipes.go:67`).
`go test -race -count=100 ./internal/cmd/julieta/hooks/` passed; the
failure needed the load of the whole race run.

Fix, proposed: a fresh reader for each run in the test helper, or
`procgroup.Run` waits for its stdin copy once the leader has exited, if a
reused reader is a supported input (the second is a change to
`internal/procgroup`, a security surface of ADR 0038's list).

Reopen: when the race step fails on this test again, or when a task
touches `internal/procgroup` or the hooks tests.
