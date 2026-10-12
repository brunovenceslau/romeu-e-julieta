---
id: T040
title: "`gate`"
milestone: M3
module: gate
fate: kept
owns:
  - internal/gate/**
needs:
  - T021
  - T033
  - T039
  - T007
after: []
ask_first:
  - gates
sitting: none
---

# T040 - `gate`

Goal: `gate`.

Re-plan (2026-10-12): Kept as written before the re-plan.

- Implements: 01 1.4 (gate 2), 12 12.3 (the struct-tag row), 05 5.2 (I7,
  I29).
- Operator: no.
- Acceptance:
  - The widening set and the recreate digest field lists are generated
    from the struct tags; the generated flip table shows that each
    `gate:"widening"` field flips the digest and each field on the
    untagged list of T021 does not (I7); a field on neither fails the
    generator, so the table is never green by omission.
  - The diff is not truncated: a 10 000-line kit file change appears in
    full, the summary is repeated above the prompt, and a binary file is
    shown as size and sha256, flagged (I29). The diff text has a golden
    under `internal/gate/testdata/` (10 10.1).
  - The prompt needs a terminal; a `/dev/ptmx` test drives `y` and `N`
    (10 10.1).
- Verify: `go test ./internal/gate/...`.
