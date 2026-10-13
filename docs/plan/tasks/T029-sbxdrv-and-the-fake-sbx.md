---
id: T029
title: "`sbxdrv` and the fake `sbx`"
milestone: M3
module: sbxdrv
fate: kept
owns:
  - internal/sbxdrv/**
  - e2e/fakesbx/**
  - e2e/testdata/sbx/**
needs:
  - T021
  - T008
  - T137
after:
  - T107
ask_first:
  - sbxdrv
sitting: none
---

# T029 - `sbxdrv` and the fake `sbx`

Earlier title: `sbxdrv`.

Goal: `sbxdrv` and the fake `sbx`: argv builders one file each and
parsers from the pinned sbx's help text; the I4 and I25 scans; absolute
path, scrubbed environment, timeout; the sessions under
`e2e/testdata/sbx/<version>/`; the fake replays recordings only,
stateful, exit 99 on `--auto-approve` and on a value-form secret.

Re-plan (2026-10-12): Kept, changed; T015 (the fake `sbx`) is folded in:
one seam, one owner, and its rows are copied below. The sessions are
written by hand from the pinned sbx's help text and from what daily use
showed, each marked `synthetic: true`; no recorder lands before the
harness (T018, M6). The `sbxdrv` forecast holds once T107 adds the
surface. T019 derives its table from the list of sessions when it lands
(gap 24 in `docs/plan.md`). Every sbx call runs through
`internal/procgroup` (T137). `owns` keeps `internal/sbxdrv/**`; T066
adds its own `internal/sbxdrv/julieta*.go` files under rule 1 of
`docs/plan.md`.

- Implements: REQ-0017 (index), REQ-0027 (index), REQ-0054 (01 1.3),
  REQ-0064 (01 1.4), REQ-0074 (01 1.5), REQ-0083 (01 1.6), REQ-0102 (01
  1.6), REQ-0120 (02 2.1), REQ-0170 (03 (opening)), REQ-0263 (03 3.13),
  REQ-0264 (03 3.13), REQ-0265 (03 3.13), REQ-0266 (03 3.13), REQ-0270
  (04 4.1), REQ-0287 (04 4.1), REQ-0288 (04 4.1), REQ-0296 (04 4.1),
  REQ-0300 (04 4.1), REQ-0364 (04 4.2), REQ-0441 (04 4.4), REQ-0485 (05
  5.2), REQ-0509 (05 5.2), REQ-0544 (05 5.3), REQ-0759 (10 10.1),
  REQ-0760 (10 10.1), REQ-0766 (10 10.1), REQ-0927 (10 10.3), REQ-0928
  (10 10.3), REQ-0929 (10 10.3), REQ-0930 (10 10.3), REQ-0934 (10 10.3),
  REQ-0938 (10 10.3), REQ-0957 (10 10.6), REQ-0996 (11 11.1), REQ-0997
  (11 11.1), REQ-0999 (11 11.1), REQ-1056 (12 12.1).
- Operator: no.
- Acceptance:
  - Each argv builder produces a shape present in a hand-written session
    marked `synthetic: true` under `e2e/testdata/sbx/<version>/`; each
    such session has a builder. Every builder carries its timeout class
    of 04 4.1 Starting values and whether the call is interactive (stdio
    inherited, raw: `sbx env run -d [--clone]`) or captured.
  - A scan finds no builder that emits `--auto-approve` (I4); the
    builder table rejects a value form of `sbx secret set` (I25).
  - The parsers run over the sessions of the `sbx` version they were
    written for. The placeholder types are listed here; the round-trip
    of each through the parsers is asserted when the recorder (T018)
    lands, and T018's own test uses a stand-in parser until then.
  - Each parser, run over a session with a read field missing or
    mistyped, a truncated document or an unknown top-level shape,
    returns `upstream-shape` (`RJ-310`, exit 2) naming sbx and the
    recorded versions, never an empty result, and the test asserts no
    state record changed; an `sbx ls --json` with a renamed sandbox list
    is never read as an empty list (01 1.6); a session where `sbx ls
    --json` exits non-zero gives `sbx-unknown` (`RJ-308`, exit 1). The
    upstream shapes decode with `encoding/json/v2` without
    `RejectUnknownMembers`; a document with a duplicate member name is
    refused, with a fixture.
  - A failed sbx call returns details with the escaped argv, the exit
    status, the first 4 KiB of stderr with the home path, user name,
    host name and root path replaced, the numbered step of 04 4.2 and
    the romeu, julieta and sbx versions, by a table test in
    `internal/sbxdrv`.
  - The hand-written sessions include one failure session per failure
    row of 01 1.6 (an `sbx ls` error, an `sbx env exec` timeout, an `sbx
    env run` timeout, a sandbox that does not start), each marked
    `synthetic: true`.
  - On create the fake writes the recorded `remote.sandbox-<name>`
    stanza into the primary clone's config (10 10.3, Stateful replay; 01
    1.2).
  - The sbx floor check before gate 1 exits 2 with `sbx-floor`
    (`RJ-343`).
  - `sbx` runs by absolute path with the scrubbed environment of 04 4.1
    and the sbx timeouts of its table, through `internal/procgroup` (10
    10.6); a test proves `subprocess-timeout` (`RJ-323`) fires through
    the table's seam.
  - `sbxdrv/env.go` holds a `register` of environment contributions, the
    seam T080's `env_signing.go` plugs into (rule 14 of `docs/plan.md`).
- Acceptance (from T015, folded):
  - The map names no owner for `e2e/fakesbx`; see [Questions for the
    maintainer](../../plan.md#questions-for-the-maintainer).
  - On recordings of its own test data: an argv shape absent from the
    recording fails the test with the argv (Replay only); a call out of
    the recorded order fails (Stateful replay); for `env exec`, the argv
    before `--` is matched and the command after it is forwarded.
  - It exits 99 on `--auto-approve` (I4) and on a value form of `sbx
    secret set` (I25), whatever the recording holds.
  - The placeholder normalizer has a unit test (10 10.1).
- Verify: `go test ./internal/sbxdrv/...`; `go test ./e2e/fakesbx/...`.
