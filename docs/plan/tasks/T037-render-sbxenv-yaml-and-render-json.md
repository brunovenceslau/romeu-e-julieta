---
id: T037
title: "`render`: `sbxenv.yaml` and `render.json`"
milestone: M3
module: render
fate: kept
owns:
  - internal/render/render*.go
  - internal/render/env*.go
  - internal/render/json*.go
  - internal/render/workspace*.go
  - internal/render/types*.go
  - internal/render/testdata/env/**
  - internal/shquote/**
  - tools/ci/internal/golden/**
  - tools/ci/check_golden.go
needs:
  - T021
  - T032
  - T036
  - T105
after: []
ask_first:
  - checks
sitting: none
---

# T037 - `render`: `sbxenv.yaml` and `render.json`

Goal: `render`: `sbxenv.yaml`, `render.json`, the workspace files, the
types, the goldens, `shquote`, `tools/ci golden`.

Re-plan (2026-10-12): Kept. `render` is one package in three file sets
(T037, T038, T039), each owning its own files and testdata, ordered by
`after`. Read differently until M6, in the first row below: the
goldens are written by this task and confirmed by the maintainer's
daily use from sitting M3 (rule 10 and gap 23 in `docs/plan.md`); A5
hashes them when it runs (T019, M6), and only from then on does a
difference move a recorded hash. Until then a difference is a reviewed
`-update`.

- Implements: 03 3.3, 03 3.7, 10 10.4, 10 10.2 (`golden`), 05 5.2 (I8,
  I11).
- Operator: no.
- Acceptance:
  - The renderer produces, byte for byte, the goldens T019 wrote and A5
    probed: the shape of 03 3.3, the fixed mount `./.romeu/bin`, no
    `x-*` key, no `lifecycle` hook. A difference is not fixed by
    `-update`: it moves a hash A5 recorded, `tools/ci probes` flags it,
    and A5 runs again on both hosts.
  - The secret command is built by `internal/shquote` (table-tested) in
    the `/usr/bin/env -i` form; no literal value is rendered (I8).
  - `tools/ci imports` rejects `text/template` in `internal/render`
    (I11); `tools/ci golden` fails on `-update` in CI and lists the
    changed goldens.
  - `schemas/render.v1.json` is generated, and `render.json` has a
    golden under `internal/render/testdata/` (10 10.1).
- Verify: `go test ./internal/render/...`; `go run ./tools/ci golden`.
