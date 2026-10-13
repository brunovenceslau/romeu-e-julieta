---
id: T045
title: "`romeu sync` without a terminal"
milestone: none
module: romeu-cli
fate: folded
absorbed_by: T043
owns: []
needs: []
after: []
ask_first: []
sitting: none
---

# T045 - `romeu sync` without a terminal

Re-plan (2026-10-12): folded into T043; its rows are copied into T043's
Acceptance. This file keeps the id so that earlier citations resolve;
the text below is the one written before the re-plan, and T043 is where
it is built.

- Implements: 04 4.2 (`sync`), 01 1.6 ("Candidate"), 09 (J11, step 5),
  05 5.2 (I7).
- Operator: no.
- Ask-first, as forecast before the re-plan: `contracts`, `gates`.
- Acceptance:
  - Without a terminal, a `sync` whose widening digest differs deletes
    the candidate dir, prints the digest and exits 3; it records no
    awaiting candidate (01 1.6). Approving a candidate rendered without
    a terminal is a deferred decision (index).
  - `romeu approve` takes `--toolchain` only; `approve <name>` is an
    unknown command (04 4.2).
- Verify: `go test -tags e2e ./e2e/...`.
