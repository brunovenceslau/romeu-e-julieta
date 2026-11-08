---
id: T202
title: "sbxdrv on sbx 0.51: the floor, --no-input, sbx inspect, the unreadable state and exit 125"
milestone: M3
module: sbxdrv
fate: new
owns:
  - internal/sbxdrv/noinput*.go
  - internal/sbxdrv/inspect*.go
  - internal/sbxdrv/ls*.go
  - internal/sbxdrv/list*.go
  - internal/sbxdrv/envrun*.go
  - internal/sbxdrv/envexec*.go
  - internal/sbxdrv/call*.go
  - internal/sbxdrv/builders_test.go
  - e2e/fakesbx/**
  - e2e/testdata/sbx/**
needs:
  - T029
after:
  - T191
ask_first:
  - sbxdrv
sitting: none
---

# T202 - sbxdrv on sbx 0.51: the floor, --no-input, sbx inspect, the unreadable state and exit 125

Goal: the sbx driver speaks sbx 0.51: the version floor is 0.51.0,
every argv shape that accepts it carries `--no-input`, `sbx inspect`
reads the version a sandbox was created with, the attached `sbx env run
<dir>` of `run` step 7 is a builder, the mount fields of `sbx ls --json`
and its unreadable state are parsed, and exit 125 of `sbx env exec` is
`sbx-unknown`.

Added on 2026-11-06 (scope this repository), with the specification
amended first: the sbx 0.51 and kit schema m.12 amendments of 01 1.6,
03 3.8, 04 4.1, 06 6.3, 11 11.4 and 12 12.1. The facts the fake needs
are hand-written sessions marked `synthetic: true` (rule 10 of
`docs/plan.md`) until A4 records them (T019, T026).

1. The floor (REQ-1056): raised to 0.51.0 on 2026-11-07 with the
   session set moved to `e2e/testdata/sbx/0.51.0/`, still marked
   `synthetic: true`, by the pull request that amended the spec; this
   task brings the sessions to the shapes of 0.51 where A4 or the
   maintainer's host shows they differ.
2. `--no-input` (REQ-1360): every builder but the two interactive calls
   of 04 4.1 Output carries it where the recorded shape accepts it; a
   prompt under it ends the call with sbx's error in the details; the
   I4 guard still refuses the flags that approve a plan.
3. `sbx inspect` (REQ-1359, 04 4.1 item 3): a read, 30 s class, whose
   creating-version field romeu records in the generation (T066 writes
   it; this task parses it).
4. `sbx ls --json` (REQ-0102): the mount fields (source, target, the
   missing marker) are parsed here and compared by T203 (REQ-1354); a
   sandbox in a state romeu does not know, the unreadable state among
   them, is present and `upstream-shape`, never absent. The parse of
   `sbx inspect` is this task's; the record it feeds is T066's
   (REQ-1359).
5. The attached `sbx env run <dir>` (REQ-0358, REQ-0997): the builder
   of `run` step 7, raw and interactive as the create and the start
   are.
6. Exit 125 of `sbx env exec` (REQ-1366): `sbx-unknown` (`RJ-308`), no
   julieta failure, in the julieta client of T066 through this task's
   call layer.

Amended on 2026-11-07 (as decided on 2026-11-07, decisions G and N):
item 1's floor is merged (the pull request that amended the spec raised
it and carried the sessions), so REQ-1056 and REQ-0933 leave the
Implements line; the sessions' julieta placeholder, `defaultFixture`'s
julieta path and the builder table become
`/opt/julieta/bin/julieta-linux-<GOARCH>`, and a session this task adds
or changes keeps the carried header of REQ-0937 (the version it was
checked on) or says it was checked on 0.51. Item 2 follows REQ-1360 as
amended: each argv shape has a no-input bit in one per-shape table in
`call*.go`, off until A4 records that the shape accepts the flag and
that a prompt under it ends the call non-zero and changes nothing; a
shape A4 records as `not-provoked` keeps it off, a shape whose prompt
under the flag ends in a default never carries it, and the attached
`sbx env run <dir>` goes without it until A15 records that it attaches
under the flag; so at this task's merge no shape carries it (V2.5,
V2.46). The attached `sbx env run <dir>` of item 5 lands dark: `run`
attaches by `sbx env exec -it` until the deferred T211. The doc comment
of `EnvStart` in `envrun.go` is rewritten to the gated text: `run`
never hands it a changed env file while the in-place apply waits behind
the gate of run step 3 (04 4.2, REQ-1376), and sbx's own "started and
re-attached without re-provisioning" is why. Item 6 maps exit 125 to
`sbx-unknown` only for the status A4 records for an `env exec` sbx
cannot run, the synthetic session's until then; 126 and 127 stay
julieta's (REQ-1366, amended on 2026-11-07). The shapes A4 cannot
provoke are named in its result, not here (REQ-0924, T026). The title
keeps "the floor" so the file name stays the one other tasks cite.

- Implements: REQ-0102 (01 1.6), REQ-1360 (04 4.1), REQ-1366 (06 6.3),
  REQ-0997 (11 11.1).
- Operator: yes, the approval line for `internal/sbxdrv/**` and
  `e2e/fakesbx/**` (ask-first `sbxdrv`), and the maintainer's sbx 0.51
  output of `sbx inspect <name>` (JSON) and `sbx ls --json` for a
  sandbox with additional workspaces, redacted, pasted under Evidence as
  the source of the synthetic sessions' shapes and field names.
- Acceptance:
  - Each builder has a no-input bit in the per-shape table; the
    builder table asserts the flag absent on every shape today, present
    for a shape whose session marks it accepted, and never on a shape
    whose session records a default under it, nor on `sbx env run -d
    [--clone]`, nor on the attached `sbx env run <dir>` until a session
    records A15's result; the I4 and I25 checks of T029 still hold
    (amended on 2026-11-07).
  - A session entry with sbx's prompt text on stderr and a non-zero
    exit when `--no-input` is present ends the call with sbx's error in
    the details, not a hang.
  - I4's E case (REQ-0485): on the fake, the attached `sbx env run
    --no-input <dir>` built with the bit forced on, over an env file
    whose plan widens, exits non-zero and the fake records no apply.
  - `sbx inspect` parses the creating-version field of the synthetic
    session, and a shape without it is `upstream-shape`.
  - `sbx ls --json` with the mount fields parses to a mount list with
    source, target and the missing marker; a sandbox in an unknown or
    unreadable state is reported present with `upstream-shape` and the
    daemon-restart hint, never absent.
  - The call layer classifies a fake `env exec` that exits 125 as
    `sbx-unknown` (`RJ-308`) and one that exits 126 or 127 as the
    command's own failure; that the compatibility check of 06 6.3
    reports it is T066's acceptance.
  - The doc comment of `EnvStart` states the gated text, and no comment
    of `sbxdrv` says `run` hands sbx a changed env file.
- Verify: `go test ./internal/sbxdrv/... ./e2e/fakesbx/... -run
  '^(TestNoInput|TestInspect|TestLsMounts|TestLsUnreadable|TestEnvRunAttached|TestExit125|TestI4WideningNoInput)'
  -v`. The pattern names the functions this task adds, and the reviewer
  reads them in the `-v` output (10 10.8).
