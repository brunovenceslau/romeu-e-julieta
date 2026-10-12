---
id: T107
title: "One source for the ask-first list"
milestone: M0
module: ci-bootstrap
fate: new
owns:
  - .github/ask-first.yaml
  - tools/ci/internal/askfirst/**
  - docs/spec/05-security.md
  - docs/grants.yaml
needs:
  - T105
after: []
ask_first:
  - ask-first
  - spec
  - checks
sitting: none
---

# T107 - One source for the ask-first list

Goal: 05 5.3 links `.github/ask-first.yaml` instead of copying it;
`docs/reference/ask-first.md` leaves the generated set; `CODEOWNERS`
stays (both are generated outputs under rule 1 of `docs/plan.md`, so
neither is in `owns`); the `sbxdrv` surface and `skills/**` under
`kits` are added (the
pending R8-05-3 change); `catalog/egress/**` replaces
`catalog/egress.yaml`; the two narrowings of the open decision block
land here if accepted. This task edits 05 and the code only: the row
of 12 12.3 that lists the generated page is removed by T108, which owns
that page and comes after this task (amendment 8 of `docs/plan.md`).
It also creates `docs/grants.yaml`, empty, on the `ask-first` surface
(rule 13); each grant is a later one-line pull request.

- Implements: 05 5.3; 12 12.3; 12 12.4.
- Operator: no
- Acceptance:
  - `tools/ci generated` passes with one consumer fewer.
  - The 05 5.3 test asserts the link, not the copy.
  - `.github/ask-first.yaml` lists `sbxdrv`, `skills/**` under `kits`,
    and `catalog/egress/**` under `catalog`; the `ledger` surface stays
    while its row is deferred.
- Verify: `go run ./tools/ci generated` and `go run ./tools/ci fast`
  exit 0.
