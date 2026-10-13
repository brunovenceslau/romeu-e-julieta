---
id: T032
title: "`oci`"
milestone: M3
module: oci
fate: kept
owns:
  - internal/oci/**
needs:
  - T021
  - T113
after: []
ask_first: []
sitting: none
---

# T032 - `oci`

Goal: `oci`: the guarded client, the strict descriptor grammar, the
normalized capability set, with the sitting M2 descriptor as the
conformance fixture.

Re-plan (2026-10-12): Kept as written before the re-plan.

- Implements: REQ-0064 (01 1.4), REQ-0068 (01 1.4), REQ-0074 (01 1.5),
  REQ-0083 (01 1.6), REQ-0102 (01 1.6), REQ-0160 (03 (opening)),
  REQ-0168 (03 (opening)), REQ-0170 (03 (opening)), REQ-0210 (03 3.4),
  REQ-0341 (04 4.2), REQ-0433 (04 4.4), REQ-0513 (05 5.2), REQ-0514 (05
  5.2), REQ-0515 (05 5.2), REQ-0516 (05 5.2), REQ-0517 (05 5.2),
  REQ-0569 (06 6.1), REQ-0570 (06 6.1), REQ-0571 (06 6.1), REQ-0573 (06
  6.1), REQ-0576 (06 6.2), REQ-0760 (10 10.1), REQ-0793 (10 10.1),
  REQ-1050 (12 12.1).
- Operator: no.
- Acceptance:
  - The `oci` table of I28 is refused row by row: a digest mismatch at
    each hop, an HTTP URL, an HTTPS-to-HTTP redirect, an oversize body,
    a credential helper in a fake Docker config.
  - The strict grammar refuses an anchor, an alias, a merge key, an
    include and an unknown key or capability type; a fuzz target covers
    the descriptor grammar (10 10.1).
  - The registry client returns `upstream-shape` (`RJ-310`) naming the
    registry on a manifest or annotation missing a field it reads, and
    on a manifest of unknown shape; a manifest with a duplicate member
    name is refused, and an unknown member in a registry response is
    accepted (03, opening), one fixture each.
  - `render` parses local kits through `oci`'s grammar, and a test fails
    a second descriptor parser outside `internal/oci` (06 6.1, I28).
  - The conformance fixtures of A11 and A12 are parsed in T038, which
    waits for them.
- Verify: `go test ./internal/oci/...`.
