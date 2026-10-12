---
id: T025
title: "`signing`"
milestone: M2
module: signing
fate: kept
owns:
  - internal/signing/**
needs:
  - T008
after: []
ask_first:
  - signing
sitting: none
---

# T025 - `signing`

Goal: The ssh-agent identity listing, dark until T080.

Re-plan (2026-10-12): Kept as written before the re-plan.

- Implements: 06 6.4 (the signing rule's check), 12 12.1.
- Operator: no.
- Acceptance:
  - Against a test agent socket, the identity listing returns the keys;
    a socket that is unset, unreachable, or holds a number of keys other
    than one, or one that differs from the given key, is reported as
    such.
  - No dependency and no subprocess is used.
  - The code is **dark** until T080.
- Verify: `go test ./internal/signing/...`.
