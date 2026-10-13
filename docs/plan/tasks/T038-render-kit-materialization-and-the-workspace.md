---
id: T038
title: "`render`: kit materialization and `.romeu/bin`"
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
after: []
ask_first: []
sitting: none
---

# T038 - `render`: kit materialization and `.romeu/bin`

Earlier title: `render`: kit materialization and the workspace files.

Goal: `render`: kit materialization. Product kits from the embed seam,
personal kits through `WalkTree` and `os.Root`, the refusal rules, the
capability set into the widening set, the julieta binary and
`SHA256SUMS` into `.romeu/bin`.

Re-plan (2026-10-12): Kept. The unit table of the hostile trees is
T024's; the E-level cases of T044 ride in T043. The I13 row and its
golden are T037's. The I28 row reads the sitting M2 fixtures of T113
until A11 and A12 re-measure them.

- Implements: REQ-0043 (01 1.1), REQ-0044 (01 1.2), REQ-0048 (01 1.2),
  REQ-0056 (01 1.3), REQ-0068 (01 1.4), REQ-0070 (01 1.4), REQ-0132 (02
  2.1), REQ-0137 (02 2.3), REQ-0341 (04 4.2), REQ-0363 (04 4.2),
  REQ-0465 (05 5.1), REQ-0495 (05 5.2), REQ-0496 (05 5.2), REQ-0512 (05
  5.2), REQ-0513 (05 5.2), REQ-0514 (05 5.2), REQ-0515 (05 5.2),
  REQ-0516 (05 5.2), REQ-0517 (05 5.2), REQ-0567 (06 6.1), REQ-0568 (06
  6.1), REQ-0569 (06 6.1), REQ-0572 (06 6.1), REQ-0574 (06 6.1),
  REQ-0581 (06 6.3), REQ-0582 (06 6.3), REQ-0596 (06 6.4), REQ-0644 (08
  8.1), REQ-0732 (10 10.1), REQ-0737 (10 10.1).
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
    for local kits and in `internal/oci` for the workload (I28); until
    block A runs, the fixtures are the facts of sitting M2 (T113),
    marked `source: sitting M2`.
  - romeu renders `julietaBin` as the absolute path of the project's
    `.romeu/bin` julieta, quoted through `internal/shquote`, into each
    kit that declares it (06 6.4).
  - On promotion `.romeu/bin/` holds `julieta-linux-<romeu's GOARCH>`
    and `SHA256SUMS` from the embed seam, and `render.json` records the
    binary's sha256 (06 6.3 step 1).
  - Product kits come from an embed seam that tests fill with fixture
    kits; the seam reads the product kit directories through
    `kits/embed.go`, the one `embed.FS` (T077), never through a second
    embed site, and this task asserts the embed and materialization row
    of T077.
- Verify: `go test ./internal/render/...`.
