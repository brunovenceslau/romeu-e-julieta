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

Open:

- Which packages hold the 232s of the race run, measured before the
  pre-push hook runs the race (about 4m30 instead of 50s).
- The required checks of `main` are "all on ubuntu-26.04-arm" and "all
  on macos-26"; new job names need the operator to change that rule at
  merge.
- The surfaces: `checks` (`tools/ci`), `dependencies` and `approvals`
  (`.github/workflows/**`), and the 10 10.2 rows.

Reopen: when the task is planned, after the `gitsafe` question of
now.md.
