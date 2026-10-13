---
id: T070
title: "`romeu handoff`"
milestone: M4
module: romeu-cli
fate: kept
owns:
  - internal/cmd/romeu/handoff/**
  - internal/cmd/romeu/all/import_handoff.go
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

- Implements: REQ-0333 (04 4.2), REQ-0518 (05 5.2), REQ-0652 (08 8.1),
  REQ-0680 (08 8.3), REQ-0684 (08 8.3).
- Operator: no.
- Acceptance:
  - It reads through the handoff reader and the `os.Root` readers of
    `memstore` (I24), and prints the latest narrative and the newest
    facts.
  - The print opens with the fixed line that labels what follows as text
    an agent wrote, before the narrative and before the facts (05 5.4).
  - A body and a branch name with ESC sequences print escaped (I29).
- Verify: `go test -tags e2e ./e2e/... -run '^TestHandoff' -v`. The
  pattern names the `TestHandoff*` functions this task adds, and the
  reviewer reads them in the `-v` output (10 10.8).
