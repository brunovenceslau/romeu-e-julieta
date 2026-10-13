---
id: T067
title: "`romeu stop` and `romeu pull`"
milestone: M4
module: romeu-cli
fate: kept
owns:
  - internal/cmd/romeu/stop/**
  - internal/cmd/romeu/pull/**
  - internal/cmd/romeu/all/import_stop.go
  - internal/cmd/romeu/all/import_pull.go
  - e2e/scenarios/j4/**
  - e2e/stop_test.go
  - e2e/pull_test.go
needs:
  - T066
  - T060
after: []
ask_first: []
sitting: none
---

# T067 - `romeu stop` and `romeu pull`

Goal: `romeu stop` and `romeu pull`.

Re-plan (2026-10-12): Kept as written before the re-plan; its e2e files
are `e2e/stop_test.go` and `e2e/pull_test.go` (rule 1 of
`docs/plan.md`).

- Implements: REQ-0048 (01 1.2), REQ-0050 (01 1.2), REQ-0053 (01 1.3),
  REQ-0058 (01 1.3), REQ-0143 (02 2.3), REQ-0157 (02 2.6), REQ-0271 (04
  4.1), REQ-0279 (04 4.1), REQ-0313 (04 4.2), REQ-0314 (04 4.2),
  REQ-0332 (04 4.2), REQ-0359 (04 4.2), REQ-0360 (04 4.2), REQ-0361 (04
  4.2), REQ-0362 (04 4.2), REQ-0363 (04 4.2), REQ-0445 (04 4.4),
  REQ-0468 (05 5.1), REQ-0483 (05 5.2), REQ-0486 (05 5.2), REQ-0503 (05
  5.2), REQ-0519 (05 5.2), REQ-0546 (05 5.4), REQ-0585 (06 6.3),
  REQ-0687 (08 8.4), REQ-0781 (10 10.1).
- Operator: no.
- Acceptance:
  - `stop` runs `julieta snapshot --all`, then the recorded `sbx stop`
    argv. A failed `julieta snapshot` is reported, `sbx stop` still
    runs, and `stop` exits 1.
  - `stop` on a sandbox `sbx ls --json` shows stopped runs no exec and
    no `sbx stop`, prints `already stopped` and exits 0; T073's
    idempotency row for `stop` runs the second `stop` against a running
    fixture.
  - The command runs the compatibility check of 06 6.3 before its first
    julieta call and exits 2 with that check's id on a failure.
  - A unit table of stale state covers I19: a daemon URL read before a
    port change is not used. In the e2e the fake `sbx` changes the port
    between calls and reports stopped; the fetch uses the new URL or is
    not attempted.
  - A head mismatch with `julieta status` exits 1 and updates nothing.
  - `pull` on a stopped primary exits 2 with `sandbox-stopped`
    (`RJ-347`) and the hint `romeu run --no-attach <name>`; a secondary
    needs no running sandbox.
  - After `pull`, `review/<dir>/` is a standalone repository with the
    host clone as alternate and the hardening keys in its local config,
    detached at the sandbox's current branch head, and the path and the
    commit are printed (J4 step 3); `review.code-workspace` lists the
    new review checkout; the secondary's `snapshot/heads.bundle` is
    unbundled into `refs/romeu/snapshots/<name>/<dir>/*` and `git fsck
    --strict` runs on the unbundled head (04 4.2); a fuzz target over
    the `snapshot/heads.json` decoder has its seeds committed (10 10.1,
    Fuzz).
  - After `pull`, `refs/heads`, `refs/tags`, `refs/remotes` and `HEAD`
    are unchanged (I5); the review repo's local config equals the fixed
    set (I2).
  - `pull` succeeds while a `run` of the project is attached (10 10.1).
    The J4 scenario function lands here.
- Verify: `go test -tags e2e ./e2e/... -run '^(TestStop|TestPull)' -v`.
  The pattern names the `TestStop*` and `TestPull*` functions this task
  adds, and the reviewer reads them in the `-v` output (10 10.8).
