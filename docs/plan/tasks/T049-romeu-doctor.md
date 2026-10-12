---
id: T049
title: "`romeu doctor`"
milestone: M3
module: romeu-cli
fate: kept
owns:
  - internal/cmd/romeu/doctor/**
  - cmd/romeu/import_doctor.go
  - e2e/doctor_test.go
needs:
  - T046
  - T029
  - T042
  - T103
after: []
ask_first: []
sitting: none
---

# T049 - `romeu doctor`

Goal: `romeu doctor`, one file per row.

Re-plan (2026-10-12): Kept, narrowed to the rows J1 step 4 needs:
floors, root rules, `rememberHostCommands`, `kit.allowedSources`, the
global secret and broad-rule rows, memory layout. The breadth is
deferred with its trigger.

- Implements: 04 4.2 ("Checks run by `romeu doctor`"), 05 5.2 (I14, I20,
  I22, I23).
- Operator: no.
- Acceptance:
  - Each row of the doctor table that is not marked **pre** and that
    this phase can build has a fixture with `HOME` in a temp dir: the
    `sbx` and git floors, the toolchain record (the check function of
    T042, called from here), the host global git
    config, `env.rememberHostCommands`, `kit.allowedSources`, the root
    rules (I20), the file modes, the secret bindings and the tree rows.
  - The rows marked **pre** are the functions of T046, called from here
    (I22, I23). The two I22 rows fail, and `doctor` exits 6 with
    `check-findings`, on each synthesized fixture of T046, and pass on the A9 recording, with the
    same files.
  - A trusted root or disabled workspace trust fails (I14).
- Verify: `go test ./internal/... -run '^TestDoctor'`, which lists the
  row tests by name; `go test -tags e2e ./e2e/... -run '^TestDoctor' -v`
  for the A9 rows. The pattern names the `TestDoctor*` functions this
  task adds, and the reviewer reads them in the `-v` output (10 10.8).
