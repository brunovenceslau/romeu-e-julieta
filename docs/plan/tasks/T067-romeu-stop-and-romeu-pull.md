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

Narrowed on 2026-11-08: a stopped sandbox whose recreate digests
differ is now started with `sbx env exec <dir> -- true`, and refused
only for an adopted generation or, by `run` and `pull`, when the spec
removed a kit or a secrets binding since the create (04 4.2, REQ-1387,
REQ-1405, REQ-1406). T217 builds it; where this task's text below says
such a sandbox is refused, T217's text wins.

Goal: `romeu stop` and `romeu pull`.

Re-plan (2026-10-12): Kept as written before the re-plan; its e2e files
are `e2e/stop_test.go` and `e2e/pull_test.go` (rule 1 of
`docs/plan.md`).

Amended on 2026-11-07 (the sbx 0.51 and kit schema m.12 amendments,
as decided on 2026-11-07): on a sandbox whose live recreate digest
differs from the open generation's `createdWith`, or whose generation
was adopted, `stop` warns `recreate-needed` before its `sbx stop`, saying
that once stopped the sandbox is refused by `run`, `salvage`, `rm`,
`recreate`, `retire` and `pull` until the digests match or T210 lifts the
refusal, and that `romeu recreate <name>` works while it runs (REQ-0313).
`pull` makes the refusal of REQ-1387 right after the preflight and
before any exec, for any repo, before the running check of REQ-0360, and
re-reads `sbx ls --json` before each of its execs on such a generation
(REQ-1389). A failed `sbx-env-base` or `sbx-global-secret` check of the
preflight stops a `pull` whose execs would start a stopped sandbox, the
secondary's case with equal digests, with the hint to remove the file or
the secret (REQ-1390), though `pull` otherwise goes on past a failed
step 5. `stop` and `pull` use the hint function of REQ-1377 that T203 builds in
`internal/preflight`. The
preflight's mounts check (step 6, T203) is printed by `stop` and `pull`,
which go on (REQ-0078). Deferred, not in the specification: a TTY
confirmation before the `sbx stop` of such a sandbox; trigger: the first
report of a sandbox stopped into the refusal by mistake, which opens a
spec pull request first.

- Implements: REQ-0048 (01 1.2), REQ-0050 (01 1.2), REQ-0053 (01 1.3),
  REQ-0058 (01 1.3), REQ-0143 (02 2.3), REQ-0157 (02 2.6), REQ-0271 (04
  4.1), REQ-0279 (04 4.1), REQ-0313 (04 4.2), REQ-0314 (04 4.2),
  REQ-0332 (04 4.2), REQ-0359 (04 4.2), REQ-0360 (04 4.2), REQ-0361 (04
  4.2), REQ-0362 (04 4.2), REQ-0363 (04 4.2), REQ-0445 (04 4.4),
  REQ-0468 (05 5.1), REQ-0483 (05 5.2), REQ-0486 (05 5.2), REQ-0503 (05
  5.2), REQ-0519 (05 5.2), REQ-0546 (05 5.4), REQ-0585 (06 6.3),
  REQ-0687 (08 8.4), REQ-0781 (10 10.1), REQ-1387 (04 4.2), REQ-1389
  (05 5.4), REQ-1390 (05 5.4).
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
    julieta call and exits 2 with that check's id on a failure; an `sbx
    env exec` that exits 125 in the check is `sbx-unknown` (`RJ-308`,
    exit 1; REQ-1366), not the check's id.
  - On a running sandbox whose recreate digests differ, and on one whose
    generation was adopted, `stop` prints `recreate-needed` with the
    text of REQ-0313 before the `sbx stop` the fake records, and exits 0;
    with equal digests it prints no such warning.
  - `pull` on a stopped sandbox whose recreate digests differ, and on a
    stopped adopted one, exits 5 with `recreate-needed` (`RJ-305`) and the
    stopped hint of REQ-1377, for the primary and for a secondary, before
    any exec and before the `sandbox-stopped` check: the fake records no
    `sbx env exec` and no starting call, and no ref or record changes.
  - On such a generation, a sandbox the fake reports stopped between two
    execs of `pull` makes it exit 5 with `recreate-needed` before the
    next exec (REQ-1389).
  - With the preflight's mounts check failing, `stop` and `pull` print
    `mounts-mismatch` and go on (REQ-0078).
  - `pull` of a secondary on a stopped sandbox with equal digests and
    `~/.sbxenv.yaml` present under the test's home exits non-zero with
    `sbx-env-base` and its hint before any exec, and the fake records no
    call that starts the sandbox; the same holds for a failed
    `sbx-global-secret` check (REQ-1390).
  - A unit table of stale state covers I19: a daemon URL read before a
    port change is not used. In the e2e the fake `sbx` changes the port
    between calls and reports stopped; the fetch uses the new URL or is
    not attempted.
  - A head mismatch with `julieta status` exits 1 and updates nothing.
  - `pull` on a stopped primary exits 2 with `sandbox-stopped`
    (`RJ-347`) and the hint `romeu run --no-attach <name>`; a secondary
    needs no running sandbox, except on a generation whose recreate
    digests differ or that was adopted, which the refusal above stops
    first.
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
