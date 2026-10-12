---
id: T038
title: "`render`: kit materialization and the workspace files"
milestone: M3
module: render
fate: kept
owns:
  - internal/render/kits*.go
  - internal/render/bin*.go
  - internal/render/testdata/kits/**
needs:
  - T037
  - T024
  - T077
  - T009
after:
  - T037
ask_first: []
sitting: none
---

# T038 - `render`: kit materialization and the workspace files

Goal: `render`: kit materialization. Product kits from the embed seam,
personal kits through `WalkTree` and `os.Root`, the refusal rules, the
capability set into the widening set, the julieta binary and
`SHA256SUMS` into `.romeu/bin`.

Re-plan (2026-10-12): Kept. The I27 hostile trees and the E-level cases
of T044 ride here.

- Implements: 06 6.1 (Materialization, Capabilities, Personal kits), 01
  1.4, 02 2.3, 05 5.2 (I13, I27, I28).
- Operator: no.
- Acceptance:
  - A personal kit is read from the config commit's tree through `git
    fsck --strict` and `WalkTree`, and written through `os.Root` into
    the candidate only (I27).
  - A personal kit that declares a network, mount, volume, credential or
    ssh-agent capability is a sync error; another capability type is
    kept for the gate diff.
  - Each conformance fixture A11 and A12 recorded, one per capability
    type, parses to the recorded capability set, in `internal/render`
    for local kits and in `internal/oci` for the workload (I28).
  - Each workspace file has a golden and decodes to the key set
    {`folders`}; a memory directory is in neither, and a review checkout
    is only in `review.code-workspace` (I13).
  - Product kits come from an embed seam that tests fill with fixture
    kits; the seam reads the real kits T077 landed, and this task
    asserts the embed and materialization row of T077.
- Verify: `go test ./internal/render/...`.
