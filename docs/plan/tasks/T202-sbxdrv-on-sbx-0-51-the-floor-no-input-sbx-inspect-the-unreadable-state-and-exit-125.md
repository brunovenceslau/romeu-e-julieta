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
  - internal/sbxdrv/versions*.go
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

1. The floor (REQ-1056): `>= 0.51.0`, which gate 1 and `doctor` name;
   it lands with a recording set at 0.51.0 under `e2e/testdata/sbx/`,
   hand-written and marked `synthetic: true` until A4 records it (10
   10.3), since the floor is never above the lowest recorded version
   (`TestFloorIsTheLowestRecordedVersion`) and the spec sentence of 12
   12.1 moves to `>= v0.51.0` in the same pull request
   (`TestFloorIsTheSpecs`).
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

- Implements: REQ-0102 (01 1.6), REQ-1360 (04 4.1), REQ-1366 (06 6.3),
  REQ-0997 (11 11.1), REQ-1056 (12 12.1), REQ-0933 (10 10.3).
- Operator: yes, the approval line for `internal/sbxdrv/**` and
  `e2e/fakesbx/**` (ask-first `sbxdrv`).
- Acceptance:
  - The builder table asserts `--no-input` on every non-interactive
    shape and its absence on `sbx env run -d [--clone]` and the attached
    `sbx env run <dir>`; the I4 and I25 checks of T029 still hold.
  - A fake session where a non-interactive call prompts under
    `--no-input` ends with sbx's error in the details, not a hang:
    asserted with the fake's prompt failure mode.
  - `sbx version` below 0.51.0 fails gate 1 and the preflight with the
    floor named; 0.51.0 passes; the tested-window warning
    `sbx-untested` names the window.
  - `sbx inspect` parses the creating-version field of the synthetic
    session, and a shape without it is `upstream-shape`.
  - `sbx ls --json` with the mount fields parses to a mount list with
    source, target and the missing marker; a sandbox in an unknown or
    unreadable state is reported present with `upstream-shape` and the
    daemon-restart hint, never absent.
  - The call layer classifies a fake `env exec` that exits 125 as
    `sbx-unknown` (`RJ-308`); that the compatibility check of 06 6.3
    reports it is T066's acceptance.
- Verify: `go test ./internal/sbxdrv/... ./e2e/fakesbx/... -run
  '^(TestNoInput|TestInspect|TestLsMounts|TestLsUnreadable|TestEnvRunAttached|TestExit125)'
  -v`. The pattern names the functions this task adds, and the reviewer
  reads them in the `-v` output (10 10.8).
