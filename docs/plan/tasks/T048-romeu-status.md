---
id: T048
title: "`romeu status`"
milestone: M3
module: romeu-cli
fate: kept
owns:
  - internal/cmd/romeu/status/**
  - internal/cmd/romeu/version/versions.go
  - internal/cmd/romeu/all/import_status.go
  - e2e/status_test.go
needs:
  - T043
  - T103
after: []
ask_first: []
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

- Implements: REQ-0061 (01 1.4), REQ-0062 (01 1.4), REQ-0063 (01 1.4),
  REQ-0065 (01 1.4), REQ-0087 (01 1.6), REQ-0107 (01 1.6), REQ-0327 (04
  4.2), REQ-0328 (04 4.2), REQ-0329 (04 4.2), REQ-0334 (04 4.2),
  REQ-0519 (05 5.2), REQ-0585 (06 6.3), REQ-0712 (08 8.5), REQ-0933 (10
  10.3).
- Operator: no.
- Acceptance:
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
- Verify: `go test -tags e2e ./e2e/... -run '^TestStatus' -v`. The
  pattern names the `TestStatus*` functions this task adds, and the
  reviewer reads them in the `-v` output (10 10.8).
