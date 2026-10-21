---
id: T155
title: "The race run's slow tests and t.Context"
milestone: M1
module: ci-bootstrap
fate: new
owns:
  - tools/ci/internal/drift/drift_test.go
  - tools/ci/internal/drift/generate_test.go
  - tools/release/internal/build/*_test.go
  - internal/procgroup/helper_test.go
  - internal/procgroup/procgroup_test.go
  - internal/procgroup/signal_test.go
  - internal/cmd/julieta/setup/setup_test.go
  - internal/cmd/julieta/setup/helper_test.go
  - internal/cmd/julieta/status/helper_test.go
  - internal/cmd/julieta/version/version_test.go
  - internal/cmd/romeu/version/version_test.go
  - internal/gitsafe/helper_test.go
  - tools/ci/signal_unix_test.go
  - tools/ci/main_test.go
  - tools/ci/git/gittest/gittest_test.go
  - tools/mutants/main_test.go
  - tools/mutants/signal_test.go
  - internal/racechild/**
needs: []
after: []
ask_first:
  - checks
  - procgroup
  - release
  - gitsafe
sitting: none
---

# T155 - The race run's slow tests and t.Context

Goal: the first part of "One race run, test tags and parallel CI jobs"
(01m5drzqsp6zg34xzmywckwb7b), approved for planning on 2026-10-20:
the tests that hold the time of the race run, and the 28
`context.Background()` of the tests that the Go idiom audit of
2026-10-20 found. Tests only; no code of a binary or a check changes.

The race run of 10 10.2, measured on 2026-10-21 on the clean tree at
`a0fea855` (10 cores, linux/arm64, Go 1.27.3, a warm build cache, a
second builder compiling on the same machine), took 226 s; a run that
builds every test binary and runs no test (`-run '^$'`) took 47 s. Two
packages hold most of the rest, and neither for its own work:

| package | race, in the whole run | race, alone | no race, alone |
|---|---|---|---|
| `tools/release/internal/build` | 94.1 s | 116 s | 7.8 s |
| `tools/ci/internal/drift` | 91.5 s | 92 s | 2.7 s |
| `internal/procgroup` | 18.3 s | | |
| `tools/mutants` | 16.6 s | | |
| `tools/release` | 12.5 s | | |
| `internal/gitsafe` | 10.5 s | | |
| each other package | 6.5 s or less | | |

The sum of the 74 package times is 347 s. The unit run
(`go test -count=1 ./internal/... ./tools/...`) took 92 s on the same
machine, `tools/mutants` its slowest package at 49.5 s.

1. `drift`: its tests start the test binary itself as the fake `go
   generate` (`fsOps`, `drift_test.go:70`), 64 times in
   `TestGeneratedRequiresTrackedRegularOutputs` alone. A program built
   with the race detector sleeps before it exits for `atexit_sleep_ms`,
   1000 ms by default (the `GORACE` options of the race detector), and
   the child's environment, built from nothing, does not set it: each
   case takes 1.07 s, while the test's own CPU is 1 s in all (a CPU and
   a syscall profile of the test, 2026-10-21). With
   `GORACE=atexit_sleep_ms=0` in the child's environment the package
   took 4.2 s under the race (measured with that one line, then
   reverted). The other tests that start the test binary as a child
   (`os.Args[0]` or `os.Executable`: `internal/procgroup`,
   `internal/cmd/julieta/setup`, the two `version` commands,
   `tools/ci/signal_unix_test.go`, `tools/mutants/signal_test.go`) pay
   the same second per child.
2. `tools/release/internal/build`: its tests build the release
   archives, and the gzip of the archives runs under the race
   detector's instrumentation: `TestOutput` takes 9.1 s with the race
   and 0.64 s without, 97 percent of its profile in `compress/flate` and
   the race runtime. Eight of its tests take 9 s or more each under
   the race, seven of them calling `Build` and the eighth,
   `TestBuildIsReproducible` (19.2 s), building twice to compare.

So the numbers do not argue for a slow tag or for sharding: the floor
of the race run is the build of the test binaries, then the slowest
package, and both slow packages lose their time to the test harness,
not to the code under test. Sharding and GOCACHE stay deferred
("Shard the macOS race run and cache GOCACHE, deferred",
01m5drzqtpe5w9q37kv3g04xnp).

- Implements: none: the change is to tests; REQ-0833 is the row whose
  run gets shorter.
- Operator: no.
- Acceptance:
  - Every test that starts the test binary, or another program built
    with `-race`, as a child gives the child `GORACE=atexit_sleep_ms=0`,
    through one shared helper, `internal/racechild`, a new package under
    the root `internal/` that every package above may import (no open
    task owns that path, measured on 2026-10-21), or one helper per
    package where the imports check refuses that import; the helper's
    doc names the option and its default, and the child's environment
    is otherwise as it was.
  - The archive tests keep what they check, the bytes of a reproducible
    archive among them, and stop paying the instrumented gzip once per
    test: the archive is built once per package run and shared by the
    tests that read it, or the payload under test is smaller, or both;
    the builder picks and says why in the pull request.
  - Measured on one machine, before and after, in the pull request:
    `go test -race -count=1 ./tools/ci/internal/drift` under 10 s,
    `go test -race -count=1 ./tools/release/internal/build` under 30 s,
    and the whole `go test -race -count=1 ./...`, with its slowest five
    packages.
  - Each of the 28 `context.Background()` of a `_test.go` file (one in
    each `helper_test.go` of `setup`, `status` and `gitsafe`, two in
    `gittest_test.go`, seven in `drift`, one in `tools/ci/main_test.go`,
    fifteen in `tools/mutants/main_test.go`) becomes `t.Context()` where
    a `*testing.T` is at hand and the context ends with the test; one
    that stays, such as a helper's context that a cleanup cancels after
    `t.Context()` is done, carries a comment that says why.
  - No assertion is removed or weakened, no test is skipped (REQ-0799),
    and `go run ./tools/ci all` passes.
- Verify: the timings above, then `go test -race -count=1 ./...` and
  `go run ./tools/ci all`.

Decisions taken while building (2026-10-21), each with what reopens
it:

- No `internal/racechild`. The imports check refuses it twice, measured
  with a probe package imported from `tools/mutants`: no row of the
  table of 10 10.7 names it (`imports unnamed`, REQ-0972), and `tools/**`
  imports below `internal/` only `procgroup` and the `all` packages
  (`imports tools`, REQ-0971). The acceptance's fallback holds: each of
  the four packages that starts the test binary as a child
  (`internal/procgroup`, `tools/ci`, `tools/ci/internal/drift`,
  `tools/mutants`) has a `raceEnv` constant in its helper test file,
  its doc naming `GORACE`, `atexit_sleep_ms` and the 1000 ms default.
  The child's environment is otherwise as it was: the entry is added,
  nothing is removed. Reopened by a row in the table for a test-support
  package that `tools/**` may import.
- The `setup` and the two `version` tests start no child: they call
  `os.Executable` to read the path and the hash of the running test
  binary. The 1 s of each of those packages is the test binary's own
  `atexit_sleep_ms` sleep at its exit, which no test can set.
- The archive tests: both approaches, as far as each keeps what a test
  checks. A smaller payload is out for the tests that read the
  archive: the 3.8 MB of a romeu with the julieta in it is the Go
  runtime twice, and the build contract takes no `-s -w`. So
  `fixture_test.go` builds the fixture twice at once, once per package
  run (`shared`): a dry run whose gate counts its requests, and a
  release behind a passing gate, both of one commit. `TestDryRunBuild`,
  `TestBuildIsReproducible` (the two builds of one commit, a dry run
  and a release, byte for byte), the dry run of
  `TestDryRunBuildMakesNoNetworkRead`, the embedded half of
  `TestEmbeddingDoesNotPassSilently` and `TestReleaseBuildWithTheTagAtHead`
  read them. `TestDirtyTree` and `TestOutput` build the fixture without
  the consumer of the julieta (half the payload), which neither reads.
  The tests that cannot share a build keep their own: the symlink test,
  which sets `TMPDIR`, and the two builds of the fixture without the
  consumer. Every test that does not set the environment is parallel,
  so a package run holds seven builds, not twelve, and they overlap.
  Reopened by a build option that picks the gzip level, or a smaller
  fixture binary.
- Two `context.Background()` stay in `tools/mutants/main_test.go`,
  each with its reason in a comment: `pinnedGo`, shared by every test
  and made by the first to ask, and `fixture.runArgs`, which has no
  `*testing.T` and a caller in `isolation_test.go`, which the task does
  not own. The three helpers that serve git (`setup`, `status`,
  `gitsafe`) take `context.WithCancel(t.Context())`: the cleanup still
  cancels, and the test's end does it first.
- `internal/gitsafe/reach_test.go` holds four more
  `context.Background()`, which the audit's 28 do not list and this task
  does not own.
