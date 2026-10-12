---
id: T091
title: "The v1 catalog table"
milestone: none
module: catalog
fate: folded
absorbed_by: T102
owns: []
needs: []
after: []
ask_first: []
sitting: none
---

# T091 - The v1 catalog table

Re-plan (2026-10-12): folded into T102, which carries its v1 table and
the handover sitting. This file keeps the id so that earlier citations
resolve; the text below is the one written before the re-plan, and T102
is where it is built.

- Implements: 03 3.5 (the closing paragraph), 07 7.6, S4.
- Operator: block O4 feeds it; block O5 confirms it.
- Ask-first, as forecast before the re-plan: `catalog`.
- Acceptance:
  - The output block O4 handed over, read and redacted by the maintainer
    first, is committed as a fixture under `internal/catalog/testdata/`;
    for the keys and hosts it lists, the catalog gains entries, each
    domain with an explicit `upload` flag, and a test asserts that every
    listed key and host has one.
  - `tools/ci catalog` passes.
  - That `julieta spec validate --catalog` then exits 0 against the
    reference config repo is the maintainer's first step of block O5.
- Verify: `go test ./internal/catalog/...`; `go run ./tools/ci catalog`.
