---
id: T009
title: "Two binaries that print their version"
milestone: M1
module: julieta-core
fate: kept
owns:
  - cmd/romeu/main*.go
  - cmd/julieta/main*.go
  - cmd/romeu/import_version.go
  - cmd/julieta/import_version.go
  - internal/cmd/romeu/version/**
  - internal/cmd/julieta/version/**
  - tools/ci/internal/crossbuild/**
  - tools/ci/check_crossbuild.go
needs:
  - T008
  - T105
after: []
ask_first:
  - checks
sitting: none
---

# T009 - Two binaries that print their version

Goal: `cmd/romeu` and `cmd/julieta` print their version; the cross-build
check as a `tools/ci` package.

Re-plan (2026-10-12): Kept. `version --json` reports the fields of 04
4.1 and whether its directory is writable; julieta builds for linux
amd64 and arm64 in CI. `cmd/<binary>/main.go` holds no command list:
each command package is linked in by its own
`cmd/<binary>/import_<command>.go` (rule 1 of `docs/plan.md`), and this
task writes the two for `version`.

- Implements: 04 4.1 (Parsing, Version), 04 4.2, 04 4.3, 12 12.3 (the
  `command` kind, command definitions), 10 10.2 (cross-build).
- Operator: no.
- Acceptance:
  - `julieta-core` and the `version` command of `romeu-cli` land before
    their modules' place in the order of 12 12.2 (Q26): the cross-build,
    the import rule (T010) and the container e2e (T013) need two
    binaries, and they need nothing from the modules between.
  - `romeu version` and `julieta version [--json]` print the fields of
    04 4.1; the julieta `--json` form reports whether its own directory
    is writable.
  - The cross-build step builds `romeu` for darwin amd64 and arm64 and
    `julieta` for linux amd64 and arm64.
  - `go run ./tools/new command` writes a stub, its help test and its
    reference entry; `docs/reference/` gains one generated page per
    command (12 12.3).
- Verify: `go run ./tools/ci all` on the four runners.
