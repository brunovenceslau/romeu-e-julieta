---
id: T195
title: "tools.gh: the host setting, its init resolution and the gh-floor row of doctor"
milestone: M3
module: romeu-cli
fate: new
owns:
  - internal/spec/host*.go
  - internal/cmd/romeu/init/init.go
  - internal/cmd/romeu/doctor/gh_floor*.go
  - internal/cmd/romeu/doctor/errors.go
  - tools/ci/internal/callsites/**
needs:
  - T049
  - T169
after:
  - T185
ask_first:
  - formats
  - gates
  - contracts
  - procgroup
  - checks
sitting: none
---

# T195 - tools.gh: the host setting, its init resolution and the gh-floor row of doctor

Goal: the optional host setting `tools.gh` (03 3.4), which `romeu init`
records when `gh` is on `PATH`, and the `doctor` row `gh-floor`, which
runs `gh --version` and holds it to the floor of 12 12.1.

Added on 2026-11-01 (decided on 2026-10-31 and 2026-11-01, scope this
repository): `romeu upgrade` (T196) verifies the attestation through
`gh`, the one program romeu starts besides `sbx` and `git` (REQ-0296,
amended). The setting enters through `init`, as `sbx` and `git` do,
and is unset when `gh` is absent; `doctor` warns then and fails on a
`gh` that is not executable or is below the floor. `gh --version` is an
admitted call site of I16 (the `gh-floor` row of `doctor`), so the table
of `tools/ci/internal/callsites` gains the row with its `procgroup`
approval line. The floor is one constant in code, and a test compares it
with the number 12 12.1 states, as the `tools/ci` test of REQ-0911 does
for the pin. The validation rule of `tools.gh` (REQ-1319) lands in
`internal/spec/host*.go`; if it has to go into `rules.go`, the `gates`
line carries it. `init.go` changes by the one line of its tools loop,
after T185, which owns the file.

- Implements: REQ-1319 (03 3.1), REQ-1320 (04 4.1), REQ-1322 (04 4.2),
  REQ-1331 (04 4.2), REQ-1058 (12 12.1).
- Operator: no.
- Acceptance:
  - `HostSettings` gains `Tools.Gh`, optional, validated as an absolute
    clean path of letters, digits and `._/+-` (REQ-1319); an invalid
    value is a usage error, exit 2, naming the rule; the schema
    `host-settings.v1` is regenerated and the pull request carries the
    `formats` line.
  - `romeu init` records the `gh` found on `PATH` as an absolute path
    when it meets the rule of REQ-1319 (the path form and the file
    rule of the credential helper, `gitsafe.CheckCredentialHelperFile`)
    and goes on with `tools.gh` unset when none is found or the found
    one fails the rule; a test for each, the `init` fixtures of T041
    unchanged otherwise.
  - The row `gh_floor.go` registers through `doctor/rows.go` (rule 14 of
    `docs/plan.md`): unset, `warn` with a hint naming `romeu upgrade`
    and the J1 step 2 install; set and not an absolute path to an
    executable file, `fail`; `gh --version` (run through `procgroup`
    with the environment of REQ-0296, its first line parsed as `gh
    version <semver>`) below 2.68.0, `fail` naming both numbers; at or
    above, `pass`; a `gh` that exits non-zero or prints a shape the
    parser does not read, `error` (`doctor-incomplete`); a fixture per
    result with a `gh` helper re-executed from the test (10 10.1).
  - `go run ./tools/ci callsites` admits the row and fails a fixture
    package that starts `gh` elsewhere (REQ-0500).
- Verify: `go test ./internal/spec/ -run '^TestHostSettingsGh' -v`; `go
  test ./internal/cmd/romeu/init/ -run '^TestInitGh' -v`; `go test
  ./internal/cmd/romeu/doctor/ -run '^TestDoctorGhFloor' -v`; `go run
  ./tools/ci callsites`.
