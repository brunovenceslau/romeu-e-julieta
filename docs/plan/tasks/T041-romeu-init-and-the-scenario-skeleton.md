---
id: T041
title: "`romeu init` and the scenario skeleton"
milestone: M3
module: romeu-cli
fate: kept
owns:
  - internal/cmd/romeu/init/**
  - cmd/romeu/import_init.go
  - e2e/scenarios/j1/**
  - e2e/scenarios/scenarios.go
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
---

# T041 - `romeu init` and the scenario skeleton

Earlier title: `romeu init`.

Goal: `romeu init` and the scenario skeleton.

Re-plan (2026-10-12): Kept, changed. One directory per journey under
`e2e/scenarios/<journey>/`, each with its own `commands.yaml`; the
completeness test reads them all; the e2e harness with the fake sbx and
`git http-backend` origins. A later command task owns its own scenario
directory and e2e file, so no task appends to another's.

- Implements: 04 4.2 (`init`), 09 (J1, step 3), 03 3.4, 05 5.2 (I20).
- Operator: no.
- Acceptance:
  - In the e2e with origins served over TLS (10 10.1), `init` writes
    host settings with absolute tool paths and the real root path, and
    clones the config repo to `<root>/<cfg>-env/<dir>`.
  - It refuses to overwrite existing settings.
  - It refuses a root inside a git repository, equal to `$HOME`, or
    holding host settings or state (I20).
  - `e2e/scenarios` starts here: the package, `commands.yaml` with the
    `romeu init` line, the J1 scenario function up to its step 3, which
    reads that line, and the package's own test, which fails a table
    entry no function reads (ADR 0001 rule 13). A second test fails a
    command of the command definitions (12 12.3) that has no
    `commands.yaml` line and whose definition does not declare that no
    journey runs it (a field beside the side-effect fields of 12 12.3,
    read from the package that registers the command, so no shared
    exemption list exists; rule 5 of `docs/plan.md`), so a missing line
    is found at the task that adds the command and not at T094, after
    the candidate tag. Each later task that adds a
    host-facing command adds its journey's function and command lines
    (10 10.8). The package needs no `sbx` fact, so the tasks that add a
    julieta-side journey function wait for this one and not for block
    O3.
- Verify: `go test -tags e2e ./e2e/... -run '^TestInit' -v`. The pattern
  names the `TestInit*` functions this task adds, and the reviewer reads
  them in the `-v` output (10 10.8).
