---
id: T009
title: "Two binaries that print their version"
milestone: M1
module: julieta-core
fate: merged
owns:
  - cmd/romeu/main*.go
  - cmd/julieta/main*.go
  - internal/cmd/romeu/all/import_version.go
  - internal/cmd/julieta/all/import_version.go
  - internal/cmd/romeu/version/**
  - internal/cmd/julieta/version/**
  - tools/ci/internal/crossbuild/**
  - tools/ci/check_crossbuild.go
  - tools/new/command*.go
  - tools/new/error*.go
  - internal/cli/main.go
  - internal/cli/main_test.go
  - tools/ci/checks_test.go
  - tools/ci/all_test.go
  - .gitattributes
  - tools/new/kind_test.go
  - tools/new/main.go
needs:
  - T008
  - T105
  - T106
  - T137
after: []
ask_first:
  - checks
sitting: none
merged: 2026-10-18
pr: 138
---

# T009 - Two binaries that print their version

Merged on 2026-10-18 in pull request #138. The text below is the one
written at the re-plan.

Goal: `cmd/romeu` and `cmd/julieta` print their version; the cross-build
check as a `tools/ci` package; the `command` and `error` kinds of
`tools/new`.

Re-plan (2026-10-18): Changed. T009 also owns `internal/cli/main.go`
and its test: `cli.Main` reports nothing for an error that is the
cancellation of the command's context, so a command ended by SIGINT or
SIGTERM prints the one line "<binary>: interrupted" of `procgroup.Main`
and exits 130 or 143, rather than the error `internal` (`RJ-348`) first
(04 4.1, Signals). It also owns the files that pin or list what
T009 adds: `tools/ci/checks_test.go` and `tools/ci/all_test.go` (the
registered checks and the steps of `all`, which gain the cross-build
check), `.gitattributes` (the two `version` pages, each with `-text`),
and `tools/new/kind_test.go` and `tools/new/main.go` (the kinds of
`tools/new`, which gain `command` and `error`).

Re-plan (2026-10-17): Changed by ADR 0015. `cli.Main` runs a cobra
tree, so `main.go` calls `cli.Main(ctx, os.Args[1:], env)` under
`procgroup.Main`, and the `command` kind of `tools/new` writes a
definition that returns a `*cobra.Command`. The generator of the
per-command reference pages is T008's; the `version` pages appear here
because this task registers the first command.

Re-plan (2026-10-12): Kept. `version --json` reports the fields of 04
4.1 and whether its directory is writable; julieta builds for linux
amd64 and arm64 in CI. `cmd/<binary>/main.go` holds no command list:
each command package is linked in by its own
`internal/cmd/<binary>/all/import_<command>.go` (rule 1 of
`docs/plan.md`), and this task writes the two for `version`. Both
`tools/new` kinds land here, with their first input, so T008 waits for
M0's tooling no longer. The runner-class table and the zero-tests
failure are T013's.

- Implements: REQ-0112 (02 2.1), REQ-0113 (02 2.1), REQ-0115 (02 2.1),
  REQ-0267 (04 4.1), REQ-0268 (04 4.1), REQ-0274 (04 4.1), REQ-0281 (04
  4.1), REQ-0285 (04 4.1), REQ-0286 (04 4.1), REQ-0303 (04 4.1),
  REQ-0304 (04 4.1), REQ-0305 (04 4.1), REQ-0838 (10 10.2), REQ-0971 (10
  10.7), REQ-1038 (12 12.1), REQ-1039 (12 12.1), REQ-1109 (12 12.3),
  REQ-1124 (12 12.3), REQ-1234 (04 4.1), REQ-1235 (04 4.1).
- Operator: no.
- Acceptance:
  - `julieta-core` and the `version` command of `romeu-cli` land before
    their modules' place in the order of 12 12.2 (Q26): the cross-build,
    the import rule (T010) and the container e2e (T013) need two
    binaries, and they need nothing from the modules between.
  - `cmd/<binary>/main.go` is the composition root: it builds `Env` from
    the real clock, entropy, the ldflags version stamp and the process
    streams, installs the signal handler of `internal/procgroup`,
    imports `internal/cmd/<binary>/all` and calls `cli.Main`; it holds
    no other logic, and nothing else maps a signal.
  - `romeu version` and `julieta version [--json]` print the fields of
    04 4.1; `romeu version --json` carries the `versions` object of 04
    4.1 (sbx, git, macOS; per project the workload digest and the kit
    pins once a render exists), which joins with T048; the julieta
    `--json` form reports whether its own directory is writable.
  - `romeu version` and `julieta version` sent SIGINT exit 130.
  - The cross-build step builds `romeu` for darwin amd64 and arm64 and
    `julieta` for linux amd64 and arm64.
  - `go run ./tools/new command <binary> <command> [<subcommand>]`
    writes a stub, its help test and its definition with the
    side-effect fields to fill and a function that returns its
    `*cobra.Command` with `RunE` (ADR 0015), plus its import file; `go run ./tools/new error <package> <slug>` writes the next
    number into the package's `errors.go` (the `error` kind, 12 12.3);
    `docs/reference/romeu/version.md` and
    `docs/reference/julieta/version.md` are generated (12 12.3). A
    duplicate kind name of `tools/new` panics at `init`, and a test
    asserts the kinds' order and names.
- Verify: `go run ./tools/ci all` on the runners of `ci.yml`, and of
  `x64.yml` when it runs (10 10.2, Runners).
