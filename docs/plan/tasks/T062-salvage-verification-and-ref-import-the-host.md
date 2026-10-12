---
id: T062
title: "Salvage verification and ref import, the host half"
milestone: M4
module: salvage
fate: kept
owns:
  - internal/salvage/verify*.go
  - internal/salvage/import*.go
needs:
  - T061
  - T033
  - T035
  - T024
after: []
ask_first: []
sitting: none
---

# T062 - Salvage verification and ref import, the host half

Goal: Salvage verification and ref import, the host half, dark.

Re-plan (2026-10-12): Kept as written before the re-plan.

- Implements: 08 8.5 ("Host half"), 05 5.2 (I6, I17).
- Operator: no.
- Acceptance:
  - The manifest is read through `os.Root`; completeness is recomputed
    and the manifest's own `complete` ignored (I17).
  - Refs are created only under `refs/romeu/salvage/...` with
    `CreateRef` (I6).
  - The package is **dark** until T068.
- Verify: `go test ./internal/salvage/...`.
