---
id: T031
title: "`catalog`"
milestone: M3
module: catalog
fate: kept
owns:
  - internal/catalog/**
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
file per tool family under `internal/catalog/egress/<family>.yaml`,
beside the package that embeds them with `//go:embed egress/*.yaml` (a
pattern cannot leave its package directory, measured at the pinned go),
so two families never edit one file. The v1 table and the handover
sitting of T091 fold into the acceptance of T102. The `catalog` surface
is `internal/catalog/**`, code and data (T107). It stays in M3, lane A:
T053 lands `lock --check` without the catalog lookup, which T036 adds
after T053 (decision of the stage B pass), so M2 does not wait for this
task.

- Implements: REQ-0009 (index), REQ-0040 (01 1.1), REQ-0128 (02 2.1),
  REQ-0129 (02 2.1), REQ-0188 (03 3.1), REQ-0211 (03 3.5), REQ-0212 (03
  3.5), REQ-0617 (07 7.3), REQ-0620 (07 7.3), REQ-0626 (07 7.4),
  REQ-0637 (07 7.5), REQ-0638 (07 7.6), REQ-0640 (07 7.6), REQ-0641 (07
  7.6), REQ-0842 (10 10.2).
- Operator: no.
- Acceptance:
  - `internal/catalog/egress/*.yaml` is embedded by glob and decoded
    strictly, one file per tool family; the `meta` and `base` sets live
    in `internal/catalog/egress/base.yaml`; `schemas/catalog.v1.json` is
    generated.
  - Two family files declaring one domain with different entries fail
    decoding.
  - `tools/ci catalog` fails a domain used anywhere without an explicit
    `upload` entry, and a malformed key.
  - The entries are the examples of 03 3.5 and the v1 entries derived
    from the reference locks read in a sandbox; the handover that
    confirms them is T102's acceptance.
- Verify: `go test ./tools/ci/... -run '^TestCatalog'`, which runs the
  fixture lines; `go run ./tools/ci catalog`.
