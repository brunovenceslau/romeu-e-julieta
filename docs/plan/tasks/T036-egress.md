---
id: T036
title: "`egress`"
milestone: M3
module: egress
fate: kept
owns:
  - internal/egress/**
needs:
  - T031
  - T024
  - T053
after: []
ask_first: []
sitting: none
---

# T036 - `egress`

Goal: `egress`: the split, `egress.extra`, `egress.tools`, kit-declared
domains, upload flags, derivation from locks with the TOML parser.

Re-plan (2026-10-12): Kept. The TOML parser enters `go.mod` with its
first importer, the mise driver (T053), which this task needs (rule 7
of `docs/plan.md`).

- Implements: 07 7.5 (derivation), 07 7.3, 05 5.2 (I9, I10).
- Operator: no.
- Acceptance:
  - The eight steps of 07 7.5 have a table test: a `mise.toml` without a
    lock is an error; an unknown `backend:tool` without a spec entry is
    exit 2 naming the tool; a lock host the catalog did not produce is
    gated.
  - A domain with `upload: true` or with no `domains` entry is gated,
    for catalog, base and kit-declared domains (I10).
  - The lock is read with `git cat-file` at `egressCommit`, and no mise
    process runs (07 7.1; I9).
- Verify: `go test ./internal/egress/...`.
