---
id: T211
title: "The step 7 switch to the attached sbx env run, once A15 records it"
milestone: M6
module: romeu-cli
fate: deferred
owns: []
needs:
  - T066
  - T026
after: []
ask_first: []
sitting: none
---

# T211 - The step 7 switch to the attached sbx env run, once A15 records it

Goal: `run` step 7 execs into the attached `sbx env run <dir>`, which
runs the env file's `entrypoint` and `cmd` (`julieta layout up`), in place of the `sbx
env exec -it` of `layout up`, for a generation whose recreate digests
are equal and that romeu created.

Added on 2026-11-07 (scope this repository), as decided on 2026-11-07
(decision O, V2.20, and N2): deferred from the start. Reopened by:
A15's record (T026) that the attached call on a running sandbox whose
env file equals the running one and carries the `entrypoint` and `cmd`
of 03 3.3 attaches, runs them and provisions nothing, with and without
`--no-input` (04 4.2, REQ-0358; 03 3.3, REQ-1370; that shape named on
2026-11-07, after a `cmd` with no `entrypoint` failed by hand on sbx
v0.51.0 and only `sbx env plan` accepted the two keys), together with a recorded env-file digest (item 1). `owns` is written when the task reopens; the files it will
name are `run`'s step 7 file under `internal/cmd/romeu/run/`, the
request form of `internal/sbxdrv/attach*.go` (T066), `e2e/run_test.go`
and the I1 check of `e2e/hybrid_test.go`. T202's builder of the
attached call lands dark until then.

1. The switch: the typed attach request of REQ-1384 takes the `run`
   form for a generation whose digests are equal and that romeu
   created, and only when the live env file equals the one the
   generation was created or last started with, which needs the
   env-file digest of the in-place apply's scope (T210, REQ-1395);
   until that digest is recorded the task stays deferred, since equal
   recreate digests do not mean an equal env file (RC-9, amended on
   2026-11-07); the drift check of `sbxenv.yaml` (01 1.5, step 3) runs
   again right before the attach, since sbx reads the env file after
   steps 5 and 6 ran code in the sandbox (REQ-0358).
2. The exception (N2): a generation whose digests differ, or that was
   adopted, keeps the `sbx env exec -it` form, which makes no `sbx env
   run` with the changed file (REQ-1376, REQ-1378; I1's E case).
3. `--no-input` on the attached call, as A15 records it (REQ-1360).
4. The hybrid test: the fake matches the attached call's argv and
   forwards the env file's `entrypoint` and `cmd` into the julieta container (10 10.1,
   REQ-0738; 10 10.3, REQ-0927).

- Implements: REQ-0358 (04 4.2), REQ-0927 (10 10.3).
- Operator: yes, the approval line for `internal/sbxdrv/**` (ask-first
  `sbxdrv`) when the task reopens and edits `attach*.go` (RP-16).
- Acceptance:
  - For a generation with equal digests that romeu created whose env
    file digest differs from the recorded one, the final argv stays the
    `sbx env exec -it` of `layout up`.
  - For a generation with equal digests that romeu created, and an
    equal env file, the final argv is the attached `sbx env run <dir>`, with `--no-input` exactly
    when A15 recorded that it attaches under it; the drift check runs
    again right before the exec and a drifted `sbxenv.yaml` exits 4
    without exec'ing.
  - For a generation whose digests differ and for an adopted one, the
    final argv stays the `sbx env exec -it` of `layout up` (I1).
  - The hybrid test runs `layout up` through the attached call's
    `entrypoint` and `cmd`.
- Verify: `go test -tags e2e ./e2e/... -run '^(TestRunAttached|TestHybridAttached)' -v`.
  The pattern names the functions this task adds, and the reviewer
  reads them in the `-v` output (10 10.8).
