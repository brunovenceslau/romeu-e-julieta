# fast does not run coverage, and macOS runs only in CI

Since T157 the local `go run ./tools/ci fast` runs the race but no
coverage; coverage (with its 80% floor) runs in `race` and `all`. The
license step also runs only in `all`.

A test can pass on Linux and fail on the macOS race job: on 2026-10-22 a
test expected the e2e step's go test on darwin, where its class does not
run it; on 2026-10-19 TMPDIR was under a symbolic link.

Vet for darwin too (`GOOS=darwin`) and gate platform-dependent
expectations on the runner-class table. Before a pull request, run
`go run ./tools/ci all` on a clean tree.
