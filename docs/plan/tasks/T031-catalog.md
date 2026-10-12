---
id: T031
title: "`catalog`"
milestone: M3
module: catalog
fate: kept
owns:
  - internal/catalog/**
  - catalog/egress/**
  - tools/ci/internal/catalog/**
  - tools/ci/check_catalog.go
needs:
  - T021
  - T105
after:
  - T107
ask_first:
  - catalog
  - checks
sitting: none
---

# T031 - `catalog`

Goal: `catalog`: the entries, the lookups and `tools/ci catalog`.

Re-plan (2026-10-12): Kept, changed. The entries are the allowlists of
the maintainer's projects and the reference config repo's stack, one
file per tool family under `catalog/egress/`, embedded by glob, so two
families never edit one file. The v1 table and the handover sitting of
T091 fold into the acceptance of T102. `catalog/egress/**` replaces
`catalog/egress.yaml` on the `catalog` surface (T107).

- Implements: 03 3.5, 07 7.6, 10 10.2 (the `catalog` step).
- Operator: no.
- Acceptance:
  - `catalog/egress/*.yaml` is embedded by glob and decoded strictly,
    one file per tool family; `schemas/catalog.v1.json` is generated.
  - `tools/ci catalog` fails a domain used anywhere without an explicit
    `upload` entry, and a malformed key.
  - The entries are the examples of 03 3.5 and the v1 entries derived
    from the reference locks read in a sandbox (amendment 10 in
    `docs/plan.md`); the handover that confirms them is T102's
    acceptance.
- Verify: `go test ./tools/ci/... -run '^TestCatalog'`, which runs the
  fixture lines; `go run ./tools/ci catalog`.
