---
id: T066
title: "`romeu run` and the version handshake"
milestone: M3
module: romeu-cli
fate: kept
owns:
  - internal/cmd/romeu/run/**
  - internal/cmd/romeu/all/import_run.go
  - internal/cmd/julieta/layout/**
  - internal/cmd/julieta/all/import_layout.go
  - internal/sbxdrv/julieta*.go
  - e2e/scenarios/j3/**
  - e2e/scenarios/j2/step4_*.go
  - e2e/hybrid_test.go
  - e2e/run_test.go
  - e2e/internal/romeu/run_helpers.go
  - internal/cmd/romeu/status/julieta_fields.go
needs:
  - T043
  - T046
  - T051
  - T054
  - T079
  - T103
  - T013
  - T135
after:
  - T048
ask_first:
  - sbxdrv
sitting: none
---

# T066 - `romeu run` and the version handshake

Earlier title: `romeu run`.

Goal: `romeu run`: the generation record, egress reconcile, the
compatibility check of 06 6.3, `julieta setup`, `sbx env run -d`, the
lock released before `exec`, J3, the hybrid test, the julieta client of
`sbxdrv` and the minimal `julieta layout up`.

Re-plan (2026-10-12): Kept, changed; T016 (the version handshake on a
synthetic session) folds in as its hybrid test, `e2e/hybrid_test.go`,
with its rows copied below; T030's two rows move to T026 with the A4
recording. A minimal `julieta layout up` lands here, so the argv of I1
holds byte for byte while `layout` (T056) is deferred; the alternative
weighed, asserting the fallback shell of 04 4.2 step 7 (`sbx env exec
-it <dir> -- /bin/bash -l`) until T056, was not taken, so that no task
reads the spec differently. The julieta client (manifest encoding
through `internal/manifest`, the `sbx env exec --env JULIETA_MANIFEST`
call, the compatibility check of 06 6.3 against `render.json`) is
`internal/sbxdrv/julieta*.go`, this task's own files in the package T029
owns (rule 1 of `docs/plan.md`; the `sbxdrv` forecast), with the
compatibility check in a non-command package the task names, which
`stop`, `pull`, `salvage` and `status` import, never the `run` package.
The manifest type and encoder come from T135; `setup` from T051. `after:
T048` for `status/julieta_fields.go`. The helper file it adds to the
harness of T041 is `e2e/internal/romeu/run_helpers.go`. No per-step
stderr lines: the progress-lines row of the index's Deferred decisions
holds them.

- Implements: REQ-0001 (index), REQ-0012 (index), REQ-0013 (index),
  REQ-0041 (01 1.1), REQ-0051 (01 1.2), REQ-0055 (01 1.3), REQ-0088 (01
  1.6), REQ-0089 (01 1.6), REQ-0101 (01 1.6), REQ-0104 (01 1.6),
  REQ-0130 (02 2.1), REQ-0144 (02 2.3), REQ-0151 (02 2.5), REQ-0171 (03
  (opening)), REQ-0202 (03 3.3), REQ-0215 (03 3.6), REQ-0216 (03 3.6),
  REQ-0225 (03 3.6), REQ-0227 (03 3.7), REQ-0233 (03 3.8), REQ-0270 (04
  4.1), REQ-0271 (04 4.1), REQ-0283 (04 4.1), REQ-0302 (04 4.1),
  REQ-0308 (04 4.2), REQ-0311 (04 4.2), REQ-0327 (04 4.2), REQ-0328 (04
  4.2), REQ-0340 (04 4.2), REQ-0346 (04 4.2), REQ-0348 (04 4.2),
  REQ-0349 (04 4.2), REQ-0350 (04 4.2), REQ-0351 (04 4.2), REQ-0352 (04
  4.2), REQ-0353 (04 4.2), REQ-0354 (04 4.2), REQ-0355 (04 4.2),
  REQ-0356 (04 4.2), REQ-0357 (04 4.2), REQ-0358 (04 4.2), REQ-0441 (04
  4.4), REQ-0482 (05 5.2), REQ-0488 (05 5.2), REQ-0495 (05 5.2),
  REQ-0508 (05 5.2), REQ-0519 (05 5.2), REQ-0550 (05 5.4), REQ-0551 (05
  5.4), REQ-0584 (06 6.3), REQ-0585 (06 6.3), REQ-0609 (07 7.1),
  REQ-0634 (07 7.5), REQ-0635 (07 7.5), REQ-0651 (08 8.1), REQ-0675 (08
  8.2), REQ-0722 (08 8.5), REQ-0728 (09 (opening)), REQ-0736 (10 10.1),
  REQ-0738 (10 10.1), REQ-0747 (10 10.1), REQ-1212 (01 1.6), REQ-1213
  (01 1.6).
- Operator: no.
- Acceptance:
  - Hybrid, on the Linux runners, the J3b scenario function passes: a
    generation is recorded before `sbx env run`, egress is reconciled,
    the compatibility check and `setup` run, and the final argv equals
    the one of I1 byte for byte, through the minimal `julieta layout
    up`.
  - A minimal `julieta layout up` lands here in
    `internal/cmd/julieta/layout/`: with no herdr layout it execs a
    login shell in the primary repo's dir, so the argv of I1 holds; T056
    replaces its body.
  - The 32 KiB manifest round-trip of I30 runs against the hand-written
    synthetic `--env` session of T029 and the manifest reader of T135,
    which this task is the first to hold together.
  - For each base SHA romeu puts in a manifest, the create-only ref
    `refs/romeu/base/<name>/<generation>/<dir>` exists, created with
    `CreateRef` (I6), before the manifest is passed; after a forced
    rewrite of `refs/romeu/origin/<dir>/*` and `git gc --prune=now`, a
    secondary's thin snapshot bundle still verifies and unbundles.
  - After a promotion, the next exec's manifest carries the new
    `specCommit`; no manifest of an earlier exec is reused.
  - The J3a variant: with a sandbox that exists, `run` starts or
    reattaches with `sbx env run -d` without re-provisioning, and
    `julieta setup` is a no-op when the locks are unchanged (09, J3;
    S8). The create and start calls are interactive, raw, their stdio
    the terminal's.
  - A stopped sandbox whose recreate digest differs, or whose rendered
    env file changed since its create, makes `run` exit 5 with
    `recreate-needed` (`RJ-305`) and start nothing (04 4.2 step 3, J3a);
    a failing `julieta setup` exits 1 without attaching and prints the
    escaped report; `--no-attach` ends after setup; `run` attaches when
    `julieta setup` reports a failed fetch.
  - A failed create removes the record when the sandbox is absent and
    keeps it when present; a timed-out `sbx env run -d --clone` keeps
    the open record and the next `run` closes it as lost or goes on. An
    absent sandbox with an open generation is preserved first (04 4.2,
    `run` step 2); what that preservation imports is asserted in T068,
    with `result: lost`; after preserving a lost generation, `run` exits
    1 with `RJ-335` naming `romeu run <name>`.
  - `egressApplied` is written before the `allow` calls, and the
    reconcile converges after a kill between two calls.
  - A memory directory that violates the layout allowlist makes `run`
    exit 2 (I24); a `stamps/` over its cap makes the command warn and
    not refuse.
  - A protocol mismatch, a `SHA256SUMS` mismatch and a writable bin
    mount each stop `run` with exit 2 and `julieta-protocol`,
    `julieta-digest` or `julieta-bin-writable` (I30, 06 6.3).
  - `run` warns `spec-moved: romeu sync <name>` when the last-fetched
    config ref is ahead, with no network call; a running sandbox whose
    recreate digest differs warns `recreate-needed` naming the changed
    fields.
  - A romeu built with plain `go build` exits 2 with
    `julieta-not-embedded` on `sync` and `run`, and `status`, `doctor`
    and `version` work.
  - A gate 1 or a gate 2 change makes `run` exit 3 before a mutating
    call (I7).
  - The julieta exec calls carry the julieta class of the table of 04
    4.1, through the seam of `internal/procgroup`, and a test proves
    `subprocess-timeout` (`RJ-323`) fires through it; a fuzz target over
    the decoder of julieta's `--json` output read over `sbx env exec`
    has its seeds committed (10 10.1, Fuzz).
  - romeu's share of S8 is measured by B5 on real hosts (11 11.2), and
    no CI test asserts a bound on it (S8).
  - The lock is released before the final `exec` (04 4.1).
  - `romeu status` gains julieta's warnings and recorded hook failures
    for a running sandbox, and says so when it skips that exec (04 4.2),
    in this task's own file under `internal/cmd/romeu/status/`, after
    T048, asserted in `e2e/run_test.go` against the sandbox `run`
    started; the sandbox `run` starts carries the `julieta-claude` hooks
    (T079), read by `status`'s `agent-hooks-missing` (J3b).
  - The J2 scenario function gains its step 4, `romeu run`, in this
    task's own step file under `e2e/scenarios/j2/`.
- Acceptance (from T016, folded):
  - At the hybrid level on both Linux runners, the compatibility check
    passes for a julieta of an accepted protocol, with the expected
    sha256, in a read-only directory; a protocol mismatch, a
    `SHA256SUMS` mismatch and a writable bin mount each fail it with
    exit 2.
  - The session the fake `sbx` replays is hand-written, lives in the
    test's own data and is named `synthetic`; it is not under
    `e2e/testdata/sbx/<version>/`, which holds block A recordings only
    (10 10.3). T026 replaces it with the A4 recording.
  - `internal/sbxdrv` gains the two argv builders the check needs, `sbx
    version` and `sbx env exec`. They are the first rows of the argv
    table that T019 completes.
  - The test supplies `render.json` and `SHA256SUMS` as fixtures;
    `render` writes them from T039 on.
  - The I4 and I25 checks of T029 cover the builders; the fake's exit 99
    already holds here.
- Verify: `go test -tags e2e ./e2e/... -run '^(TestRun|TestHybrid)' -v`.
  The pattern names the `TestRun*` and `TestHybrid*` functions this task
  adds, and the reviewer reads them in the `-v` output (10 10.8).
