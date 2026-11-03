---
id: T049
title: "`romeu doctor`"
milestone: M3
module: romeu-cli
fate: merged
owns:
  - internal/cmd/romeu/doctor/**
  - internal/cmd/romeu/all/import_doctor.go
  - e2e/doctor_test.go
  - internal/preflight/step5.go
  - internal/preflight/step5_test.go
  - tools/ci/internal/schema/romeu.go
needs:
  - T046
  - T029
  - T042
  - T103
  - T169
after: []
ask_first:
  - contracts
  - checks
  - formats
  - gates
sitting: none
merged: 2026-10-30
pr: 358
---

# T049 - `romeu doctor`

Goal: `romeu doctor`, one file per row.

Re-plan (2026-10-12): Kept, narrowed to the rows below, by slug:
`sbx-floor`, `git-floor`, `sbx-untested`, `toolchain-approval-required`,
`host-git-config`, `sbx-global-secret`, `sbx-broad-allow`,
`sbx-remember-commands`, `sbx-allowed-sources`, `root-unsafe`,
`root-indexed`, `home-symlink`, `auto-trust`, `vscode-trust`,
`settings-mode`, `tree-mode`, `memory-layout`, `removal-pending`,
`record-unreadable` and `generation-sandbox` (open, salvaging,
removing), plus the two I22 `upstream-shape` rows and the doctor
sentinel case. Every other slug of the table of 04 4.2, the fix-hint
paragraph and `doctor --json` per check are T134's. "The A9 recording"
reads as T046 does. T103 stays in `needs` for its e2e. `doctor/rows.go`
holds the registered rows (rule 14 of `docs/plan.md`); T080 and T134 add
theirs by one file each.

Re-plan (2026-10-22): the sweep of leftover temporary files in the state
directory is deferred to this task: `doctor` lists the leftover temporary
files (the dotfiles an interrupted atomic write leaves) for the operator
to clean up, and deletes none.

Re-plan (2026-10-30, scope this repository): `secret-argv` is built
here, since the sentinel case runs the secret argv;
`secret-unknown-project` stays T134's. The exit 1 of an incomplete
report is the error `doctor-incomplete` (`RJ-361`; `RJ-360` is
`fetch-failed` of T055), a row added to 04 4.4 as `status-incomplete`
was for T048, and declared in `doctor/errors.go` (ask-first
`contracts`). The secret argv run joins the admitted call sites of
`tools/ci/internal/callsites/table.go` by one line (ask-first `checks`).
The leftover temporary files are a member of the report, `leftovers`, a
list of paths, and no check: no slug of 04 4.1 names them.

Re-plan (2026-10-30, decided by the operator, scope this repository):
(A) the rows `host-git-config`, `sbx-remember-commands` and
`sbx-allowed-sources` move to T191 with their acceptance and requirement
ids: the first waits for a gitsafe verb that reads the host global git
config, the other two for an sbx settings read in `sbxdrv`, whose argv
probe A1 records (11 11.1). (B) `romeu doctor --json` has its schema
here, `schemas/romeu-doctor.v1.json` (format `romeu-doctor.v1`,
ask-first `formats`), registered by `tools/ci/internal/schema/romeu.go`
(ask-first `checks`); T134 no longer owns the schema nor the
`secret-argv` row. (C) `secret-argv` stays here. (D) `vscode-trust` is a
warning, not a pass, while the trusted folders of VS Code are not
checked. `preflight.HostFindings` returns each finding with the slug of
its check (`internal/preflight/step5.go`, which joins `owns`), so doctor
places no finding by guess, and a finding with no row in doctor is
reported as an error, never dropped.

Re-plan (2026-10-30, scope this repository): rows built in part, each
with the trigger that reopens it: `root-indexed` does not check that the
root is excluded from Time Machine (trigger: `tmutil` admitted as a call
site); `record-unreadable` reads the project records and
`toolchain.json`, not the descriptors (trigger: a state listing of
descriptors); `vscode-trust` does not read the trusted folders of VS
Code (trigger: a first SQLite reader, or T134).

Amended on 2026-11-01 (the release and upgrade amendments, ADR 0044):
REQ-0330 and the row table of REQ-0369 (04 4.2) gained the `gh-floor`
row, which runs `gh --version`; T195 adds it through `doctor/rows.go`.

- Implements: REQ-0056 (01 1.3), REQ-0057 (01 1.3), REQ-0059 (01 1.4),
  REQ-0062 (01 1.4), REQ-0136 (02 2.3), REQ-0141 (02 2.3), REQ-0148 (02
  2.4), REQ-0207 (03 3.4), REQ-0209 (03 3.4), REQ-0330 (04 4.2),
  REQ-0331 (04 4.2), REQ-0364 (04 4.2), REQ-0365 (04 4.2), REQ-0366 (04
  4.2), REQ-0367 (04 4.2), REQ-0368 (04 4.2), REQ-0369 (04 4.2),
  REQ-0371 (04 4.2), REQ-0372 (04 4.2), REQ-0375 (04 4.2), REQ-0376 (04
  4.2), REQ-0377 (04 4.2), REQ-0378 (04 4.2), REQ-0379 (04 4.2),
  REQ-0380 (04 4.2), REQ-0381 (04 4.2), REQ-0382 (04 4.2), REQ-0383 (04
  4.2), REQ-0385 (04 4.2), REQ-0386 (04 4.2), REQ-0387 (04 4.2),
  REQ-0388 (04 4.2), REQ-0389 (04 4.2), REQ-0390 (04 4.2), REQ-0391 (04
  4.2), REQ-0392 (04 4.2), REQ-0393 (04 4.2), REQ-0394 (04 4.2),
  REQ-0395 (04 4.2), REQ-0435 (04 4.4), REQ-0471 (05 5.1), REQ-0497 (05
  5.2), REQ-0504 (05 5.2), REQ-0506 (05 5.2), REQ-0507 (05 5.2),
  REQ-0508 (05 5.2), REQ-0510 (05 5.2), REQ-0546 (05 5.4), REQ-0555 (05
  5.4), REQ-0557 (05 5.4), REQ-0559 (05 5.4), REQ-0712 (08 8.5),
  REQ-0713 (08 8.5), REQ-0751 (10 10.1), REQ-0762 (10 10.1), REQ-0763
  (10 10.1), REQ-0789 (10 10.1), REQ-0933 (10 10.3).
- Operator: no.
- Acceptance:
  - Each row above has a fixture with `HOME` in a temp dir, and its test
    asserts the row's slug in `romeu doctor --json`, not only the exit
    status (10 10.1): the `sbx` and git floors, the toolchain record
    (the check function of T042, called from here), the root rules
    (I20), the file modes, the secret bindings and the tree rows. The
    host global git config, `env.rememberHostCommands` and
    `kit.allowedSources` are T191's (2026-10-30).
  - The rows marked **pre** are the functions of T046, called from here
    (I22, I23). The two I22 rows fail, and `doctor` exits 6 with
    `check-findings`, on each synthesized fixture of T046, and pass on
    the A9 recording, with the same files. On the two shape fixtures of
    T046, `doctor` reports `upstream-shape` on both I22 rows and exits
    1.
  - `doctor` reports a live sbx version outside the tested window of 10
    10.3 as the warning `sbx-untested`, naming the version and the
    recorded range; until block A the window is the version the
    synthetic sessions were written for.
  - The test titled "doctor warns root-indexed when the marker is
    missing" passes (04 4.2).
  - A trusted root or disabled workspace trust fails (I14).
  - The case "doctor never shows a secret argv's output": a fixture
    whose secret argv prints a sentinel on stdout and on stderr and
    exits 0, and one that prints it and exits 1; the sentinel appears in
    no stdout, stderr or `--json` output of `doctor` and in no file
    under the temp `HOME` and `XDG_*` directories after the run;
    positive control: the guard's stub prints it (I25). The secret argv
    runs through `internal/procgroup` with its class and with exactly
    `HOME` and `PATH=/usr/bin:/bin` (03 3.4).
  - Exit 1 when any check could not run, every failed check still
    printed; 6 when every check ran and any failed; `--json` carries one
    object per check with `result` in `pass`, `warn`, `fail` or `error`
    and the error id for `error`.
  - A romeu built with plain `go build`, which embeds no julieta, runs
    `doctor` as the embedded build does, and never stops with
    `julieta-not-embedded` (from T043, 2026-10-30; 04 4.2).
- Verify: `go test ./internal/... -run '^TestDoctor'`, which lists the
  row tests by name; `go test -tags e2e ./e2e/... -run '^TestDoctor' -v`
  for the A9 rows. The pattern names the `TestDoctor*` functions this
  task adds, and the reviewer reads them in the `-v` output (10 10.8).
