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
  - T007
after:
  - T123
ask_first:
  - gates
sitting: none
---

# T040 - `gate`

Goal: `gate`.

Re-plan (2026-10-12): Kept as written before the re-plan; `after: T123`,
since the record of the two gates is a review order (12 12.5).

- Implements: REQ-0055 (01 1.3), REQ-0059 (01 1.4), REQ-0066 (01 1.4),
  REQ-0067 (01 1.4), REQ-0071 (01 1.4), REQ-0191 (03 3.2), REQ-0192 (03
  3.2), REQ-0272 (04 4.1), REQ-0488 (05 5.2), REQ-0518 (05 5.2),
  REQ-0575 (06 6.1), REQ-0638 (07 7.6), REQ-0732 (10 10.1), REQ-0754 (10
  10.1), REQ-0790 (10 10.1), REQ-1050 (12 12.1), REQ-1106 (12 12.3).
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
  - The golden's fixture changes one item of each class (a capability, a
    gated domain, a secret argv, the workload repository, a content-only
    kit file); the golden shows the summary first, in that order and
    marked, then content-only kit changes with file and line counts, the
    kit file list with modes, the workload repository highlighted and
    the host-settings argv behind each secret name, and the summary
    repeated above the prompt (01 1.4).
  - The prompt needs a terminal: it is written to stderr and the TTY
    condition is stdin; a `/dev/ptmx` test drives `y` and `N` on stdin
    and reads stderr (10 10.1).
  - The record of the two gates is merged (12 12.5).
- Verify: `go test ./internal/gate/...`.
