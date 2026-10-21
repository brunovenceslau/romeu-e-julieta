# One race run, test tags and parallel CI jobs

What is at stake: medium. The macOS job of `ci.yml` gates every pull
request and took 13 to 25 minutes in the runs of 2026-10-20, against 9
on Linux; a slow gate is the bottleneck of the development loop.

Scope: this repository.

Measured on 2026-10-20 (run 39471007600, a push to main):

| step | Linux | macOS |
|---|---|---|
| race, `go test -race -count=1 -coverprofile ./...` | 4m14 | 9m17 |
| unit, `go test -count=1 ./internal/... ./tools/...` | 1m25 | 4m22 |
| lint, 4 targets | about 1m15 | about 1m40 |
| fix, 4 targets | about 57s | about 75s |
| mise install | 37s | 58s |

Locally, on 10 cores with a warm build cache: `go run ./tools/ci fast`
took 50s (unit 40s), and `go test -race -count=1 ./...` 232s.

Approved for planning on 2026-10-20:

1. One test run, always with `-race`: `unit` leaves `fast` and `all`
   (`tools/ci/internal/unit/unit.go:21-27`, REQ-0824), since `race`
   (`tools/ci/internal/race/race.go:32`, REQ-0833) runs every test it
   runs.
2. Filters are build tags, so `go test -race ./...` tests everything and
   `go test -race -tags <tag> ./...` everything of that tag; no
   directory list selects packages.
3. Parallel jobs, each calling a `tools/ci` subcommand, with `all` the
   union locally: on Linux a static job, the race with coverage, and
   e2e; on macOS the race alone, since lint and fix already cover the
   darwin targets from Linux.
4. The Go idiom findings of the audit of 2026-10-20 are fixed as
   defects: `-count=1` kept only where a test runs git or another
   subprocess against the real tree (the test cache tracks in-process
   reads); the e2e step's hand-built argv (`-json -timeout=14m`) and its
   `ROMEU_E2E_INFRA_FILE` side channel
   (`e2e/internal/container/container.go:56`); the `host` tag, named in
   REQ-0832, REQ-1254 and three tag lists, that no file uses; 28
   `context.Background()` in tests where `t.Context()` fits; two copies
   of `ModuleRoot`; the hand-parsed cover profile
   (`tools/ci/internal/coverage`), where `go tool cover -func` needs no
   dependency.

Measured on 2026-10-21 (10 cores, linux/arm64, Go 1.27.3, warm build
cache, a second builder compiling on the same machine): the race run
took 226 s, 47 s of it building the test binaries. Two packages held
most of the rest, for reasons of their tests:
`tools/release/internal/build` (94 s; gzip under the race
instrumentation) and `tools/ci/internal/drift` (92 s; a child built with
`-race` sleeps 1 s before it exits, 4.2 s with
`GORACE=atexit_sleep_ms=0`). No other package took more than 19 s. The
table is in T155.

Planned on 2026-10-21 as three tasks:

- T155, the race run's slow tests and the 28 `context.Background()` of
  the tests.
- T156, after T155 and T150: one list of tags, the `-count=1` reasons,
  one argv for the race and e2e steps, one `ModuleRoot`, the cover
  profile.
- T157, after T155 and T156: `unit` leaves `fast` and `all`, the jobs
  `static`, `race` and `e2e` with `all` their union, and one aggregator
  per host that keeps the required check names; a skipped job is a
  success for GitHub, so each aggregator needs one `if` that fails it
  unless every needed job succeeded, which the workflow grammar admits
  for that form only.

The build order is T155, T150, T156, T157.

Open questions, each in its task: whether the e2e job keeps
`./e2e/...` (T156), and whether `fast` runs the race once it fits the
budget (T157).

Reopen: closed when T157 merges; this note then gets its Done record.
