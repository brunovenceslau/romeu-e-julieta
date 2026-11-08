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
  - internal/sbxdrv/attach*.go
  - tools/ci/internal/callsites/**
  - internal/state/records*.go
  - internal/state/generation*.go
  - internal/state/apply*.go
  - internal/cmd/romeu/all/memory_push*.go
needs:
  - T043
  - T046
  - T051
  - T054
  - T079
  - T103
  - T013
  - T135
  - T186
  - T200
  - T202
  - T190
  - T201
  - T203
after:
  - T048
ask_first:
  - sbxdrv
  - formats
  - gates
  - procgroup
  - checks
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

Amended on 2026-11-06 (the sbx 0.51 and kit schema m.12 amendments):
steps 5 and 6 invoke `/opt/julieta/bin/julieta-linux-<GOARCH>`, the
target of the `.romeu/bin` mount (02 2.5, T200), and step 7 execs into
the attached `sbx env run <dir>`, whose `entrypoint` and `cmd` run
`julieta layout up` over the manifest step 6 cached (03 3.3, REQ-0358; the builder is
T202's), so the argv of I1 is that call, the `sbx env exec -it` of
`layout up` until A15 records it; step 3 hands a changed env file to
sbx with `sbx env run -d <dir>`, stopped or running, after `julieta
snapshot --all` on a running sandbox, keeps the generation and moves
its `createdWith` on exit 0, and exits 1 with `recreate-needed`
otherwise (REQ-0352, REQ-0353, amended on 2026-11-07); the generation record gains
`sbxVersion` from `sbx inspect` after the create (REQ-1359); the
minimal `layout up` of this task, run as `entrypoint` and `cmd` before any manifest is
cached, prints that none is cached and that `romeu run --no-attach
<name>` writes one, and execs the login shell (REQ-0418, 04 4.3); the
hybrid test mounts `.romeu/bin` at `/opt/julieta/bin` (REQ-0738). The
sentence on the `sbx env exec -it` of `layout up` in the re-plan
paragraph above is history (the `entrypoint` and `cmd` of step 7
amended on 2026-11-07).

Amended on 2026-11-07 (the closed gate, as decided on 2026-11-07):
the in-place apply waits behind a closed gate, a Deferred row of the
index that T210's hand measurement opens, so this task builds the gate
and no apply. The sentence above on step 3 handing a changed env file
to sbx, the snapshot before it and the move of `createdWith` are
history. Step 3 now acts on the sandbox's state when the recreate
digests differ or the generation was adopted (REQ-1376, REQ-1378): a
running sandbox gets no `sbx env run`, `run` warns `recreate-needed`
with the hint of REQ-1377 and goes on as for equal digests, unless
preflight step 6 refused it first with `mounts-mismatch` (a repo
removed or a mount target changed since the create, REQ-1354); a
stopped one is not started, and `run` exits 5 with `recreate-needed`
(`RJ-305`) and the stopped hint. The hint and the refusal are the
shared function of T203 in `internal/preflight`, which `status`, `sync`
step 9, `pull` and the salvage commands use too; `run` imports it.
Which manifest steps 5 and 6 pass on such a generation, and what a
repo added, a repo removed, a mount target changed or a kit changed
does on a running sandbox, wait for the open question on mount-affecting changes on a running sandbox (docs/spec.md): `julieta setup` checks every
memory dir of the manifest (REQ-0412), so a repo added since the create
cannot attach over the live render's manifest as it stands (RP-2).
Before each `sbx env exec` of steps 5 to 7 on such a generation `run`
reads `sbx ls --json` again and exits 5 with `recreate-needed` if the
sandbox stopped, and right before it releases the lock it computes the
live recreate digest again and compares it with the one step 3 read,
not with `createdWith` (REQ-1389, amended on 2026-11-07).
Before the attach, and as the last step under `--no-attach`, `run`
makes the memory commit and push of 08 8.1 itself, through the step
that `internal/cmd/romeu/all/memory_push.go` exposes, then releases the
lock and execs (REQ-1380). The exec is one named function of `sbxdrv`,
in `internal/sbxdrv/attach*.go`, the one place besides `upgrade`'s swap
step that the call-site table admits `syscall.Exec` (REQ-1384, decided
on 2026-11-07): the table is under the `procgroup` and `checks`
surfaces, which this task forecasts. Step 7 stays the `sbx env exec
-it` of `layout up` for every generation; the switch to the attached
`sbx env run <dir>` is the deferred T211. After a create, the
generation's `sbxVersion` is written from `sbx inspect` through a row
of 01 1.6 (REQ-1379), so this task owns the generation record and its
machine in `internal/state` (the `gates` surface) and regenerates
`schemas/state-project.v1.json` (the `formats` approval line). Needs
T203 (was `after`): `run`'s tests assert the step 6 mounts result and
the shared hint. Needs T201 (was `after`): the sandbox `run` creates
carries the kit of T201, and T200 no longer removes `julietaBin`
(T212 does). Needs T190: step 4 applies the domain set T190 derives.
`RJ-305` exits 5 (REQ-0273, refused to protect data; decided on
2026-11-07).

- Implements: REQ-0001 (index), REQ-0012 (index), REQ-0013 (index),
  REQ-0041 (01 1.1), REQ-0051 (01 1.2), REQ-0055 (01 1.3), REQ-0088 (01
  1.6), REQ-0089 (01 1.6), REQ-0101 (01 1.6), REQ-0104 (01 1.6),
  REQ-0130 (02 2.1), REQ-0144 (02 2.3), REQ-0151 (02 2.5), REQ-0171 (03
  (opening)), REQ-0202 (03 3.3), REQ-0215 (03 3.6), REQ-0216 (03 3.6),
  REQ-0225 (03 3.6), REQ-0227 (03 3.7), REQ-0233 (03 3.8), REQ-1359 (03
  3.8), REQ-0418 (04 4.3), REQ-1366 (06 6.3), REQ-0270 (04
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
  (01 1.6), REQ-1376 (04 4.2), REQ-1378 (04 4.2),
  REQ-1379 (01 1.6), REQ-1380 (04 4.2), REQ-1384 (04 4.1), REQ-1389 (05
  5.4), REQ-1321 (04 4.1), REQ-1336 (05 5.2).
- Operator: yes, the approval lines for `internal/state/**` (ask-first
  `gates`), `schemas/**` (ask-first `formats`, the regenerated
  `state-project.v1.json`), `internal/sbxdrv/**` (ask-first `sbxdrv`)
  and `tools/ci/internal/callsites/**` (ask-first `procgroup` and
  `checks`). Its build waits for the maintainer's answer to the open question on mount-affecting changes on a running sandbox (docs/spec.md),
  which decides step 3's running branch for a repo added or removed, a
  mount target changed or a kit changed, and the manifest of steps 5
  and 6 (amended on 2026-11-07).
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
  - The J3a variant: with a sandbox that exists and equal digests,
    `run` starts a stopped one with `sbx env run -d` and makes no sbx
    call for a running one, then reattaches at step 7, and `julieta
    setup` is a no-op when the locks are unchanged (09, J3; S8). The
    create and the start are interactive, raw, their stdio the
    terminal's.
  - The closed gate (REQ-1376, amended on 2026-11-07): a running
    sandbox whose recreate digests differ gets no `sbx env run` and no
    snapshot exec; `run` prints the `recreate-needed` warning with the
    hint `romeu recreate <name>` from T203's function, runs steps 4 to
    6 and attaches through the `sbx env exec -it` of step 7, and the
    generation record is byte for byte unchanged. A stopped sandbox
    whose digests differ gets no sbx call that starts it: `run` exits 5
    with `recreate-needed` (`RJ-305`), the stopped hint of REQ-1377 in
    the details, and the record unchanged. The fake records no `sbx env
    run` in either case.
  - An adopted generation (`createdWith` null, REQ-1378): on a running
    sandbox `run` warns `recreate-needed` with the adopted message of
    REQ-1377 and attaches through `sbx env exec -it`; on a stopped one
    it exits 5 with `recreate-needed` and the stopped adopted message;
    the fake records no `sbx env run` with romeu's env file.
  - On a running sandbox whose render removed a repo, or changed a
    mount target, since the create, `run` stops at preflight step 6
    with `mounts-mismatch` (`RJ-368`, exit 2) before any mutating call
    and does not attach (REQ-1354, decided on 2026-11-07). On a
    stopped sandbox with equal digests whose listing showed no mounts,
    `run` calls T203's comparison right after its `sbx env run -d`
    start and, on a failure, stops the sandbox and exits 2 (REQ-1382,
    REQ-1393).
  - The cases of a repo added and of a kit changed on a running
    sandbox are written when the open question on mount-affecting changes on a running sandbox (docs/spec.md) is answered (amended on 2026-11-07).
  - Before each `sbx env exec` of steps 5 to 7 on a generation whose
    digests differ or that was adopted, `run` reads `sbx ls --json`
    again; a fake that reports the sandbox stopped at that read makes
    `run` exit 5 with `recreate-needed` and make no further sbx call. A
    secret's argv in the host settings changed after step 3 (by the
    test, between step 3 and the release of the lock) moves the live
    recreate digest away from the one step 3 read, and `run` exits 5
    with `recreate-needed` before the exec; with nothing changed, a
    running sandbox whose digests differ from `createdWith` still
    attaches (REQ-1389).
  - A failing `julieta setup` exits 1 without attaching and prints the
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
    config ref is ahead, with no network call.
  - A romeu built with plain `go build` exits 2 with
    `julieta-not-embedded` on `sync` and `run`, and `status`, `doctor`
    and `version` work. `run` reads julieta from the package of T186.
  - A gate 1 or a gate 2 change makes `run` exit 3 before a mutating
    call (I7).
  - The julieta exec calls carry the julieta class of the table of 04
    4.1, through the seam of `internal/procgroup`, and a test proves
    `subprocess-timeout` (`RJ-323`) fires through it; a fuzz target over
    the decoder of julieta's `--json` output read over `sbx env exec`
    has its seeds committed (10 10.1, Fuzz).
  - romeu's share of S8 is measured by B5 on real hosts (11 11.2), and
    no CI test asserts a bound on it (S8).
  - An attached `run` and a `run --no-attach` each commit and push
    memory before the lock is released, and the final `exec` follows
    the release (04 4.1, REQ-1380); the memory warnings print before
    the attach.
  - The attach goes through the one function of
    `internal/sbxdrv/attach*.go` (REQ-1384, RX-7): it takes a typed
    request (the project dir, the manifest, and the form, `exec -it`
    now and `run` for T211) and builds argv and environment itself; a
    unit test shows a caller cannot add an argv element or an
    environment variable; argv[0] is the absolute sbx path of the host
    settings; the gate 1 comparison of REQ-0062 and the floor check run
    against that same path right before `syscall.Exec`, and a changed
    sbx between preflight and exec exits 3 without exec'ing; the argv
    it builds passes `sbxdrv`'s I4 and I25 guards (05 5.2) before the
    exec, and a request whose built argv would carry a refused flag or
    a secret value form is rejected with no exec (RC-7); the
    scrubbed environment of 04 4.1 is the one passed; signal
    dispositions are reset to default and the mask cleared before the
    exec, and no descriptor but 0 to 2 survives it.
  - The call-site table admits `syscall.Exec` in that function and in
    `upgrade`'s swap step only, and the scan fails a fixture that calls
    it anywhere else; the calls REQ-1321 admits nowhere (`os.StartProcess`,
    `syscall.ForkExec`, `syscall.StartProcess`,
    `golang.org/x/sys/unix.Exec`, a raw `SYS_EXECVE` or `SYS_EXECVEAT`
    syscall) each have a failing fixture (RX-8).
  - The final argv is byte for byte the `sbx env exec -it <dir> --env
    JULIETA_MANIFEST=... -- /opt/julieta/bin/julieta-linux-<GOARCH>
    layout up` of I1 for a generation with equal digests, one whose
    digests differ and an adopted one (I1's E cases, RX-6).
  - After a create, the generation record carries `sbxVersion` as `sbx
    inspect` reported it (03 3.8), read through T202's parser and
    written through the new row of 01 1.6 (REQ-1379); a shape without
    the field, or a read that fails, leaves the member absent and
    prints the warning `sbx-version-unread` (04 4.1, Warnings).
  - The manifest steps 5 and 6 pass carries the in-sandbox targets of
    T200 in `repos[].memory` and `sharedMemory` (03 3.6, REQ-0221).
  - The minimal `layout up`, run with no manifest cached and none in
    its environment, prints the no-manifest line naming `romeu run
    --no-attach <name>` and execs the login shell; with a cached
    manifest it opens the shell in the primary repo's dir as before.
  - The hybrid test mounts `.romeu/bin` at `/opt/julieta/bin` and the
    julieta calls of steps 5 and 6 name that path; a fake `env exec`
    that exits 125 makes the compatibility check report `sbx-unknown`
    and no julieta failure (06 6.3, REQ-1366).
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
    (10 10.3). T026 replaces it with the A4 recording. Amended on
    2026-11-07: the handshake session is
    `e2e/testdata/sbx/<floor>/handshake.jsonl`, marked `synthetic: true`
    (REQ-0924, T202); the sentence above on the test's own data is
    history.
  - The compatibility check uses `sbxdrv`'s merged `sbx version` and
    `sbx env exec` builders (T029, T202); this task adds no builder
    (amended on 2026-11-07).
  - The test supplies `render.json` and `SHA256SUMS` as fixtures;
    `render` writes them from T039 on.
  - The I4 and I25 checks of T029 cover the builders; the fake's exit 99
    already holds here.
- Verify: `go test -tags e2e ./e2e/... -run '^(TestRun|TestHybrid)' -v
  && go test ./internal/sbxdrv/... ./internal/state/...
  ./tools/ci/internal/callsites/... -run
  '^(TestAttach|TestGenerationSbxVersion|TestCallsitesExec)' -v`. The
  patterns name the functions this task adds, and the reviewer reads
  them in the `-v` output (10 10.8).
