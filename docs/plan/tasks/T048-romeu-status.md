---
id: T048
title: "`romeu status`"
milestone: M3
module: romeu-cli
fate: kept
owns:
  - internal/cmd/romeu/status/**
  - internal/state/read.go
  - internal/state/read_test.go
  - internal/state/store.go
  - internal/state/lock.go
  - internal/state/load.go
  - internal/state/lock_commands_test.go
  - internal/render/live.go
  - internal/render/drift.go
  - internal/render/live_test.go
  - internal/cmd/romeu/version/**
  - internal/cmd/romeu/all/import_status.go
  - e2e/status_test.go
  - e2e/version_test.go
  - internal/gitsafe/version.go
  - internal/gitsafe/version_test.go
  - internal/gitsafe/floor.go
needs:
  - T043
  - T103
after: []
ask_first:
  - gates
  - contracts
  - formats
  - gitsafe
sitting: none
---

# T048 - `romeu status`

Goal: `romeu status`, and the `versions` object of `romeu version
--json`.

Re-plan (2026-10-12): Kept, narrowed: drift, interrupted promotion,
stale toolchain, orphaned; julieta's fields join with T066. T103 stays
in `needs`: its e2e runs the `--dry-run` build. `status/sections.go`
holds the registered report sections (rule 14 of `docs/plan.md`), the
seam T066 and T080 plug into.

Re-plan (2026-10-22): the sweep of leftover temporary files in the state
directory (the dotfiles an interrupted atomic write leaves, such as
`.shop.json123`) is deferred. `status` ignores dotfiles in the state
directory and its `projects/` and `descriptors/` subdirectories.

Re-plan (2026-10-30): built in slices, each its own pull request.
Slice A: the report, its sections registry (`status/sections.go`, the
seam of T066 and T080), the generations and salvage records, the
sandbox state and `removal-pending` from `sbx ls`, the failed reads per
project, and `status-incomplete` (RJ-359, exit 1), a row 04 4.4 lacked
for a report printed with a failed read (REQ-0329); host state is read
through `state.OpenRead`, which makes, changes and locks nothing, so a
mode `doctor` reports stays as found. Slice B1 (no git): the synced
commit of the live render, drift, the interrupted promotion and
recreate needed, read once through `render.ReadLive` and `render.Drift`, which
open no FIFO and wait on none, with the root of the trees injected until slice D.
Slice B2 (git and the spec): the spec commit against the synced one,
and the orphaned project and repo dir with their unpushed and
salvage-only work. Slice C:
the toolchain acknowledgement (`approve.Changes`, reported and never
exit 3) and `sbx-untested`, once at the head of the report, since gate 1
is the machine's and not a project's, whatever REQ-0327 lists per
project. Slice D1: the command file over T169's `hostpaths`, which
runs the sbx floor check once before any other sbx call, as the driver
requires, `import_status.go` and the e2e cases `TestStatus*`. Slice D2:
the `versions` object of `romeu version --json` (which reads the
settings too), the J10 line and the schema `romeu-status.v1` (`formats`),
once the report is whole; and the text of REQ-0327, REQ-0329 and the
RJ-359 row amended for what slice C put at the head of the report (the
toolchain, its warnings and the failed reads outside a project's
block). The
command file reads the host settings through the shared host paths of
T169 and waits for it, so no third copy of the settings path exists;
the newest stamp's dirty and stash counts (03 3.10) wait for T174,
which writes the stamps; julieta's fields, `fetch-failed` and the
compatibility check before julieta's read are T066's
(`status/julieta_fields.go`). The orphan scan and `envDirs` of `sync`
sit in files T055 and T169 own, so slice B copies the small part it
needs and names who unifies it.

- Implements: REQ-0061 (01 1.4), REQ-0062 (01 1.4), REQ-0063 (01 1.4),
  REQ-0065 (01 1.4), REQ-0087 (01 1.6), REQ-0107 (01 1.6), REQ-0327 (04
  4.2), REQ-0328 (04 4.2), REQ-0329 (04 4.2), REQ-0334 (04 4.2),
  REQ-0519 (05 5.2), REQ-0585 (06 6.3), REQ-0712 (08 8.5), REQ-0933 (10
  10.3).
- Operator: no.
- Acceptance:
  - The report's sections register from their own files through
    `status/sections.go` (rule 14 of `docs/plan.md`), the seam T066
    and T080 add a file to.
  - On fixtures of host state, `status --json` reports, without taking
    the lock: drift, an interrupted promotion, a stale toolchain, an
    orphaned project and an orphaned repo dir, the sandbox state, spec
    vs synced commit, recreate needed, each generation with its state
    (`adopted, not gated` for an adopted one), the salvage records,
    unimported snapshots, `removal-pending` and `record-unreadable`, and
    per repo the `createdAt` of the last `snapshot/heads.json`,
    `disabled`, and the newest facts' dirty and stash counts; the fields
    julieta supplies land with T066.
  - The J10 step 1 line is a fixture: `absent; generation <G> open;
    snapshot <T>; refs/sandboxes last fetched <T2>`; for an orphaned
    repo dir the report lists its unpushed and salvage-only work (J13
    step 3).
  - `status` reports a live sbx version outside the tested window of 10
    10.3 as the warning `sbx-untested`, naming the version and the
    recorded range; until block A the window is the version the
    synthetic sessions were written for.
  - A read that fails (an sbx error, a julieta exec that fails or times
    out, an unreadable record) is printed in that project's block with
    its error id and the output goes on; `--json` lists such reads under
    `errors` per project; `status` shows `fetch-failed` from julieta's
    state; the command runs the compatibility check of 06 6.3 before its
    first julieta call and reports a failure and goes on; it exits 1
    when any read failed, and never 3.
  - `romeu version --json` gains the `versions` object of 04 4.1, read
    from host state and each project's live `render.json`, in its own
    file `internal/cmd/romeu/version/versions.go`.
  - The fields that come from julieta land with T066, in a file of its
    own under this package.
  - A romeu built with plain `go build`, which embeds no julieta, runs
    `status` as the embedded build does, and never stops with
    `julieta-not-embedded` (from T043, 2026-10-30; 04 4.2).
- Verify: `go test -tags e2e ./e2e/... -run '^TestStatus' -v`. The
  pattern names the `TestStatus*` functions this task adds, and the
  reviewer reads them in the `-v` output (10 10.8).
