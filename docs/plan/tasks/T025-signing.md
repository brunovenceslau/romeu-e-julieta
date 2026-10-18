---
id: T025
title: "`signing`"
milestone: M2
module: signing
fate: merged
owns:
  - internal/signing/**
needs:
  - T008
after: []
ask_first:
  - signing
sitting: none
merged: 2026-10-18
pr: 140
---

# T025 - `signing`

Merged on 2026-10-18 in pull request #140. The text below is the one
written at the re-plan.

Goal: The ssh-agent identity listing, dark until T080.

Re-plan (2026-10-12): Kept as written before the re-plan.

- Implements: REQ-0520 (05 5.2), REQ-0601 (06 6.4), REQ-1050 (12 12.1).
- Operator: no.
- Acceptance:
  - Against a test agent socket, the identity listing returns the keys;
    a socket that is unset, unreachable, or holds a number of keys other
    than one, or one that differs from the given key, is reported as
    such.
  - No dependency and no subprocess is used.
  - The code is **dark** until T080.
- Verify: `go test ./internal/signing/...`.
