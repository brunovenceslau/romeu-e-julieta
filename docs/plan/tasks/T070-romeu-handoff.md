---
id: T070
title: "`romeu handoff`"
milestone: M4
module: romeu-cli
fate: kept
owns:
  - internal/cmd/romeu/handoff/**
  - cmd/romeu/import_handoff.go
  - e2e/handoff_test.go
needs:
  - T059
  - T041
after: []
ask_first: []
sitting: none
---

# T070 - `romeu handoff`

Goal: `romeu handoff`.

Re-plan (2026-10-12): Kept as written before the re-plan.

- Implements: 04 4.2 (`handoff`), 08 8.3 (Host), 05 5.2 (I24, I29).
- Operator: no.
- Acceptance:
  - It reads through the handoff reader and the `os.Root` readers of
    `memstore` (I24), and prints the latest narrative and the newest
    facts.
  - A body and a branch name with ESC sequences print escaped (I29).
- Verify: `go test -tags e2e ./e2e/... -run '^TestHandoff' -v`. The
  pattern names the `TestHandoff*` functions this task adds, and the
  reviewer reads them in the `-v` output (10 10.8).
