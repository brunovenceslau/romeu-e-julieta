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

- Implements: 01 1.4 (the normalized capability set), 06 6.1
  (Capabilities), 05 5.2 (I28).
- Operator: no.
- Acceptance:
  - The `oci` table of I28 is refused row by row: a digest mismatch at
    each hop, an HTTP URL, an HTTPS-to-HTTP redirect, an oversize body,
    a credential helper in a fake Docker config.
  - The strict grammar refuses an anchor, an alias, a merge key, an
    include and an unknown key or capability type; a fuzz target covers
    the descriptor grammar (10 10.1).
  - The conformance fixtures of A11 and A12 are parsed in T038, which
    waits for them.
- Verify: `go test ./internal/oci/...`.
