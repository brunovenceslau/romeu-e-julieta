---
id: T031
title: "`catalog`"
milestone: M3
module: catalog
fate: merged
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
merged: 2026-10-22
pr: 247
---

# T031 - `catalog`

Merged on 2026-10-22 in pull request #247. The text below is the one
written when it was built.

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

Re-plan (2026-10-22): The entries of the fourth acceptance row are
half confirmed. Measured in this repository's `mise.lock`: the backends
`core:go`, `aqua:cli/cli`, `aqua:golangci/golangci-lint` and the two
`go:` tools, and the lock hosts `github.com`, `api.github.com` and
`dl.google.com`. Not measured: the domains of the `go:` tools
(`proxy.golang.org`, `sum.golang.org`, from the way the mise go backend
builds from source) and the other examples of 03 3.5, which are taken as
written. The reference config repo's locks were not read in a sandbox.
Deferred to T102, whose handover confirms every entry from those locks;
the trigger is that handover.

Re-plan (2026-10-22): `internal/state` (T033, PR #246) stores the
catalog in its records as `map[string]any` until this package's type
exists. Replacing that map with the catalog type is this task's or a
follow-up's. Nothing may recompute a digest from the stored map: a
large integer goes through `float64` and changes.

Re-plan (2026-10-22, maintainer decision): `tuf-repo-cdn.sigstore.dev`
(`upload: false`) joins the `meta` set in `base.yaml`. mise runs with
`MISE_PARANOID=1` (T053) and verifies the GitHub artifact attestations
of the aqua tools through sigstore's TUF repository, so every sandbox
that installs an attested aqua tool needs it.

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
