---
id: T067
title: "`romeu stop` and `romeu pull`"
milestone: M4
module: romeu-cli
fate: kept
owns:
  - internal/cmd/romeu/stop/**
  - internal/cmd/romeu/pull/**
  - cmd/romeu/import_stop.go
  - cmd/romeu/import_pull.go
  - e2e/scenarios/j4/**
needs:
  - T066
  - T060
after: []
ask_first: []
sitting: none
---

# T067 - `romeu stop` and `romeu pull`

Goal: `romeu stop` and `romeu pull`.

Re-plan (2026-10-12): Kept as written before the re-plan.

- Implements: 04 4.2 (`stop`; "How `romeu pull` updates a review
  checkout"), 09 (J4), 05 5.2 (I2, I5, I19).
- Operator: no.
- Acceptance:
  - `stop` runs `julieta snapshot --all`, then the recorded `sbx stop`
    argv.
  - A unit table of stale state covers I19: a daemon URL read before a
    port change is not used. In the e2e the fake `sbx` changes the port
    between calls and reports stopped; the fetch uses the new URL or is
    not attempted.
  - A head mismatch with `julieta status` exits 1 and updates nothing.
  - After `pull`, `review/<dir>/` is a standalone repository with the
    host clone as alternate and the hardening keys in its local config,
    detached at the sandbox's current branch head, and the path and the
    commit are printed (J4 step 3); the secondary's
    `snapshot/heads.bundle` is unbundled into
    `refs/romeu/snapshots/<name>/<dir>/*` and `git fsck --strict` runs
    on the unbundled head (04 4.2).
  - After `pull`, `refs/heads`, `refs/tags`, `refs/remotes` and `HEAD`
    are unchanged (I5); the review repo's local config equals the fixed
    set (I2).
  - `pull` succeeds while a `run` of the project is attached (10 10.1).
    The J4 scenario function lands here.
- Verify: `go test -tags e2e ./e2e/... -run '^(TestStop|TestPull)' -v`.
  The pattern names the `TestStop*` and `TestPull*` functions this task
  adds, and the reviewer reads them in the `-v` output (10 10.8).
