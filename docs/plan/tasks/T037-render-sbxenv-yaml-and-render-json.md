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
  - internal/render/testdata/workspace/**
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
`needs`. The goldens are written by this task and confirmed by the
maintainer's daily use from sitting M3 (rule 10 and gap 23 in
`docs/plan.md`); A5 hashes them when it runs (T019, M6), and only from
then on does a difference move a recorded hash. The I13 row and its
golden are this task's, with the workspace files. Whether the candidate
and `render.json` types become a slice of their own, so that T037, T038
and T039 land in parallel, is decided at the sitting that opens M3
(`docs/plan.md`, item 20 of Decisions of the re-plan).

- Implements: REQ-0018 (index), REQ-0041 (01 1.1), REQ-0042 (01 1.1),
  REQ-0045 (01 1.2), REQ-0048 (01 1.2), REQ-0054 (01 1.3), REQ-0132 (02
  2.1), REQ-0140 (02 2.3), REQ-0142 (02 2.3), REQ-0156 (02 2.6),
  REQ-0198 (03 3.3), REQ-0199 (03 3.3), REQ-0200 (03 3.3), REQ-0201 (03
  3.3), REQ-0202 (03 3.3), REQ-0208 (03 3.4), REQ-0256 (03 3.12),
  REQ-0489 (05 5.2), REQ-0490 (05 5.2), REQ-0491 (05 5.2), REQ-0494 (05
  5.2), REQ-0567 (06 6.1), REQ-0583 (06 6.3), REQ-0633 (07 7.5),
  REQ-0644 (08 8.1), REQ-0732 (10 10.1), REQ-0758 (10 10.1), REQ-0835
  (10 10.2), REQ-0839 (10 10.2), REQ-0940 (10 10.4), REQ-0941 (10 10.4),
  REQ-0959 (10 10.6), REQ-0980 (11 (opening)).
- Operator: no.
- Acceptance:
  - The renderer produces, byte for byte, the goldens T019 wrote and A5
    probed: the shape of 03 3.3, the fixed mount `./.romeu/bin`, no
    `x-*` key, no `lifecycle` hook. Render's own test keeps the raw
    golden; a difference whose normalized form (`internal/canon`) is
    unchanged is a reviewed `-update`, and one that changes the
    normalized hash A5 recorded is flagged by `tools/ci probes` and
    needs A5 again (03 3.3); until A5 runs, the goldens are confirmed by
    daily use.
  - The secret command is built by `internal/shquote` (table-tested) in
    the `/usr/bin/env -i` form; no literal value is rendered (I8).
  - Each workspace file has a golden under
    `internal/render/testdata/workspace/` and decodes to the key set
    {`folders`}; a memory directory is in neither, and a review checkout
    is only in `review.code-workspace` (I13).
  - `tools/ci imports` rejects `text/template` in `internal/render`
    (I11); `tools/ci golden` fails on `-update` in CI and lists the
    changed goldens.
  - `schemas/render.v1.json` is generated, and `render.json` has a
    golden under `internal/render/testdata/` (10 10.1).
  - `render/types.go` holds the `Candidate` layout, the `render.json`
    type and the materializer seam T038 and T039 build on (rule 14 of
    `docs/plan.md`).
- Verify: `go test ./internal/render/...`; `go run ./tools/ci golden`.
