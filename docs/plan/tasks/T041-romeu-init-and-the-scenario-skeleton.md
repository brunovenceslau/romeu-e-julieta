---
id: T041
title: "`romeu init` and the scenario skeleton"
milestone: M3
module: romeu-cli
fate: merged
owns:
  - internal/cmd/romeu/init/**
  - internal/cmd/romeu/all/import_init.go
  - e2e/scenarios/j1/**
  - e2e/scenarios/scenarios.go
  - e2e/internal/romeu/**
  - e2e/init_test.go
  - e2e/testdata/origins/**
needs:
  - T023
  - T033
  - T029
  - T103
after: []
ask_first: []
sitting: none
merged: 2026-10-23
pr: 259
---

# T041 - `romeu init` and the scenario skeleton

Merged on 2026-10-23 in pull request #259. The text below is the one
written when it was built.

Earlier title: `romeu init`.

Goal: `romeu init`, the scenario skeleton and the romeu e2e harness.

Re-plan (2026-10-23): the e2e runs the host-native `romeu`, built with
`go build`, since the `--dry-run` build of `tools/release` is a darwin
cross-build that the Linux runner cannot run (10 10.2, the e2e row).
`init` needs no embedded julieta. T103 stays in `needs`: the
`--dry-run` build stays the one that embeds julieta, and a later
command's case that needs it extends that build to a host-native one.

Re-plan (2026-10-12): Kept, changed. One directory per journey under
`e2e/scenarios/<journey>/`, each with its own `commands.yaml`; the e2e
harness `e2e/internal/romeu` (the temporary root, the TLS origins over
`git http-backend`, the fake sbx process, the settings writer) with its
own tests. A later command task owns its own scenario directory and e2e
file, so no task appends to another's.

- Implements: REQ-0136 (02 2.3), REQ-0150 (02 2.4), REQ-0204 (03 3.4),
  REQ-0205 (03 3.4), REQ-0262 (03 3.13), REQ-0281 (04 4.1), REQ-0309 (04
  4.2), REQ-0375 (04 4.2), REQ-0504 (05 5.2), REQ-0545 (05 5.3),
  REQ-0724 (09 (opening)), REQ-0725 (09 (opening)), REQ-0729 (09
  (opening)), REQ-0730 (09 (opening)), REQ-0735 (10 10.1), REQ-0931 (10
  10.3), REQ-1088 (12 12.2).
- Operator: no.
- Acceptance:
  - In the e2e with origins served over TLS (10 10.1), `init` writes
    host settings with absolute tool paths and the real root path, and
    clones the config repo to `<root>/<cfg>-env/<dir>`.
  - `ROMEU_SETTINGS` overrides the settings path; no other `ROMEU_*`
    variable changes a path; `XDG_CONFIG_HOME` and `XDG_STATE_HOME` move
    the settings and state paths; the root is read only from
    `settings.yaml`.
  - It refuses to overwrite existing settings.
  - It refuses a root inside a git repository, equal to `$HOME`, or
    holding host settings or state (I20).
  - The harness is `e2e/internal/romeu`; a later task extends it by
    adding a file of its own under that directory (rule 1 of
    `docs/plan.md`) and edits none of this task's.
  - `e2e/scenarios` starts here: the package,
    `e2e/scenarios/j1/commands.yaml` with the `romeu init` line, the J1
    scenario function up to its step 3, which reads that line, and the
    package's own test, which fails a table entry no function reads (ADR
    0001 rule 13); the completeness test runs per journey package over
    that journey's `commands.yaml`, and the definitions test reads every
    `e2e/scenarios/*/commands.yaml` as data, so a new journey adds a
    directory and edits no import list. A second test fails a command of
    the command definitions (12 12.3) that has no `commands.yaml` line
    and whose definition does not declare that no journey runs it (a
    field beside the side-effect fields of 12 12.3, read from the
    package that registers the command, so no shared exemption list
    exists; rule 5 of `docs/plan.md`), so a missing line is found at the
    task that adds the command and not at T094, after the candidate tag.
    Each later task that adds a host-facing command adds its journey's
    function and command lines (10 10.8). The package needs no `sbx`
    fact, so the tasks that add a julieta-side journey function wait for
    this one and not for block O3.
  - The scenario harness gives each step an [X] argv assertion against
    the fake sbx or an [H] state-record assertion, and each scenario
    declares its omitted steps (attach, a live daemon fetch); a test
    fails an [X] or [H] step with neither an assertion nor an entry in
    the omitted list (09, opening).
- Verify: `go test -tags e2e ./e2e/... -run '^TestInit' -v`. The pattern
  names the `TestInit*` functions this task adds, and the reviewer reads
  them in the `-v` output (10 10.8).
