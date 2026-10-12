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
one seam, one owner, and its acceptance rows apply as written there. The
sessions are written by hand from the pinned sbx's help text and from
what daily use showed, each marked `synthetic: true`; no recorder lands
before the harness (T018, M6). The `sbxdrv` forecast holds once T107
adds the surface. Read differently until M6, in the rows below: "a
block A recording" is a hand-written session marked `synthetic: true`
under `e2e/testdata/sbx/<version>/`; "the table of T019" is the list of
those sessions, from which T019 derives its table when it lands (gap 24
in `docs/plan.md`); "each recorded `sbx` version" is the one the
sessions were written for; and the placeholder round-trip of T018 is
asserted when T018 lands, with a stand-in parser until then, as the
row says.

- Implements: 10 10.3 (Parsers, Known argv), 04 4.1 (Subprocesses), 05
  5.2 (I4, I25), 12 12.1 (the version floor).
- Operator: no.
- Acceptance:
  - Each argv builder produces a shape present in a block A recording;
    the table of T019 has a builder per row.
  - A scan finds no builder that emits `--auto-approve` (I4); the
    builder table rejects a value form of `sbx secret set` (I25).
  - The parsers run over each recorded `sbx` version. Each typed
    placeholder of the recorder (T018) round-trips through them, and
    the placeholder types are listed here; T018's own test uses a
    stand-in parser until then.
  - `sbx` runs by absolute path with the scrubbed environment of 04 4.1
    and a timeout (10 10.6).
- Verify: `go test ./internal/sbxdrv/...`; `go test ./e2e/fakesbx/...`.
