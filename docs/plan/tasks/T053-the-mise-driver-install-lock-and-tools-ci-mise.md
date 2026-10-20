---
id: T053
title: "The mise driver: `install`, `lock` and `tools/ci mise`"
milestone: M2
module: julieta-core
fate: kept
owns:
  - internal/mise/**
  - internal/cmd/julieta/install/**
  - internal/cmd/julieta/lock/**
  - internal/cmd/julieta/setup/install.go
  - internal/cmd/julieta/setup/steps.go
  - internal/cmd/julieta/all/import_install.go
  - internal/cmd/julieta/all/import_lock.go
  - e2e/julieta_install_test.go
  - e2e/julieta_lock_test.go
  - internal/hooks/actions/lock_check.go
  - tools/ci/internal/mise/**
  - tools/ci/check_mise.go
  - go.mod
  - go.sum
needs:
  - T051
  - T012
  - T054
  - T105
  - T137
after:
  - T008
  - T035
  - T139
  - T141
  - T145
  - T146
ask_first:
  - checks
  - dependencies
sitting: none
---

# T053 - The mise driver: `install`, `lock` and `tools/ci mise`

Goal: The mise driver: `install`, `lock`, `lock --check`, the mise
environment of 04 4.3, `tools/ci mise`, and the `pre-commit` action
file.

Re-plan (2026-10-12): Kept. The TOML parser of 12 12.1 enters `go.mod`
here, with its first importer (rule 7 of `docs/plan.md`); `egress`
(T036) comes after this task and adds the catalog lookup of `lock
--check`, so this task does not need the catalog (T031, M3). `after:
T035`: both extend `internal/cmd/julieta/setup/steps.go` by one line
(rule 14). The J8 scenario function moves to T075 with the other
remaining scenario functions; the lock-change commit on the fixture
origin is asserted here by the container e2e instead. `mise` runs
through `internal/procgroup` (T137).

- Implements: REQ-0013 (index), REQ-0169 (03 (opening)), REQ-0211 (03
  3.5), REQ-0297 (04 4.1), REQ-0302 (04 4.1), REQ-0400 (04 4.3),
  REQ-0403 (04 4.3), REQ-0404 (04 4.3), REQ-0405 (04 4.3), REQ-0406 (04
  4.3), REQ-0407 (04 4.3), REQ-0426 (04 4.3), REQ-0435 (04 4.4),
  REQ-0438 (04 4.4), REQ-0439 (04 4.4), REQ-0612 (07 7.2), REQ-0613 (07
  7.3), REQ-0614 (07 7.3), REQ-0615 (07 7.3), REQ-0616 (07 7.3),
  REQ-0617 (07 7.3), REQ-0618 (07 7.3), REQ-0619 (07 7.3), REQ-0620 (07
  7.3), REQ-0621 (07 7.3), REQ-0625 (07 7.4), REQ-0637 (07 7.5),
  REQ-0740 (10 10.1), REQ-0791 (10 10.1), REQ-0845 (10 10.2), REQ-0960
  (10 10.6), REQ-1045 (12 12.1), REQ-1049 (12 12.1).
- Operator: no.
- Acceptance:
  - `install` runs with the environment of 04 4.3 and skips when the
    `lock-set` digest equals the last success; `--force` overrides; a
    `mise.toml` without `mise.lock` is `lock-missing` (`RJ-340`).
    `setup` gains `install` (04 4.3) in this task's own file under the
    `setup` package of T051.
  - `lock` runs `mise lock --platform` with `linux-x64`, `linux-arm64`
    and the platforms of the repo's `lockfile_platforms`, and rewrites
    `mise.lock`; `lock --check` writes nothing and exits 6
    (`check-findings`) on a tool without a lock entry or a lockable
    entry without both linux platforms; it warns `lock-no-checksum` for
    an entry without a checksum, naming the fix of 07 7.5 step 2; `lock
    --check` lands without the catalog lookup, and the
    `lock-unknown-tool` warning is T036's.
  - `install` warns `version-only-tool` once per version-only tool (07
    7.3); the version-only backend list is read again at a mise pin bump
    and named in that pull request's Evidence.
  - A download blocked by egress fails with `install-blocked` (`RJ-341`)
    naming the tool, the host, the sbx command that approves it once for
    this sandbox, that the approval is lost at recreate, and the spec
    edit plus gate 2 that fixes it, and says whether the host is in the
    manifest's egress set (04 4.3).
  - `mise` runs through `internal/procgroup` with the mise class of the
    table of 04 4.1, through the seam of that package; a mise that hangs
    is killed with its group and `install` exits 1 with
    `subprocess-timeout` (`RJ-323`) naming the tool.
  - The `mise.lock` decoder is `github.com/BurntSushi/toml`; a duplicate
    key is refused, with a fixture; a fuzz target over the `mise.lock`
    parser has its seeds committed (10 10.1, Fuzz).
  - In the container e2e, at the pinned mise, `install` makes no
    self-update request to the versions host, which is still read for
    version lists, and a configuration path outside
    `MISE_TRUSTED_CONFIG_PATHS` is refused (10 10.1; 04 4.3).
  - `tools/ci mise` runs that logic on this repository's lock and the
    examples'.
  - `pre-commit` runs `lock --check` when a mise file is staged (the
    action file `internal/hooks/actions/lock_check.go`).
  - The `julieta setup` no-op is at most 3 s, median of 5, in the
    container e2e against local origins (S8). The bound is S8's own, so
    a run over it is a defect to fix, never a flake to retry (10 10.1).
- Verify: `go test -tags e2e ./e2e/... -run
  '^(TestJulietaInstall|TestJulietaLock)' -v`; `go run ./tools/ci mise`.
  The pattern names the `TestJulietaInstall*` and `TestJulietaLock*`
  functions this task adds, and the reviewer reads them in the `-v`
  output (10 10.8).
