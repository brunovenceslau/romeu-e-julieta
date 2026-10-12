---
id: T057
title: "`memstore/write` and the memory commands"
milestone: none
module: memstore
fate: folded
absorbed_by: T035
owns: []
needs: []
after: []
ask_first: []
sitting: none
---

# T057 - `memstore/write` and the memory commands

Re-plan (2026-10-12): folded into T035, which carries its acceptance
rows. This file keeps the id so that earlier citations resolve; the text
below is the one written before the re-plan, and T035 is where it is
built.

- Implements: 08 8.1, 03 3.9, 04 4.3 (`memory add|list|show|edit`,
  `memory check`, `setup`).
- Operator: no.
- Ask-first, as forecast before the re-plan: none expected.
- Acceptance:
  - julieta stamps `id`, `created` and `updated`; a supplied id or
    timestamp is an error.
  - Two julieta processes writing one store lose no entry (10 10.1,
    Concurrency).
  - `list --query <text>` filters, and `edit --status done` closes an
    entry; there is no `memory rm` and no `memory search` (04 4.3).
  - `memory check` exits 1 on an unmounted or unwritable directory, and
    6 (`check-findings`) on one that breaks the layout allowlist; `setup` gains `memory check` and the agent memory dir
    check (04 4.3; 08 8.1).
- Verify: `go test -tags e2e ./e2e/...`.
