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
  - .github/pull_request_template.md
needs:
  - T105
after:
  - T122
ask_first:
  - ask-first
  - spec
  - checks
  - approvals
sitting: none
---

# T107 - One source for the ask-first list

Goal: 05 5.3 links `.github/ask-first.yaml` instead of copying it;
`docs/reference/ask-first.md` leaves the generated set; `CODEOWNERS`
stays (both are generated outputs under rule 1 of `docs/plan.md`, so
neither is in `owns`); the `sbxdrv` surface and `skills/**` under `kits`
are added (the pending R8-05-3 change); the `catalog` surface becomes
`internal/catalog/**` (code and data, one surface); the two narrowings
of the open decision block land here if accepted. This task edits 05 and
the code only: the row of 12 12.3 that listed the generated page was
removed on 2026-10-12 (ADR 0012; T122). It also creates
`docs/grants.yaml`, `version: 1` with `grants: []`, on the `ask-first`
surface (rule 13); each grant is a later one-line pull request. It adds
the three rows of 05 5.4 that accept by name the deferred merge-gate
parts (`pr`, `sequences`, `invariants` and `mutate`) with the plan's
triggers, the I34-by-hand row and the approval-line row amended for the
grants; `docs/plan.md` links them. `after: T122`, so each later spec
edit carries the requirement ids. The surface question of
`tools/mutants` (`checks` or none) joins the open-block items it
carries.

- Implements: REQ-0025 (index), REQ-0122 (02 2.1), REQ-0125 (02 2.1),
  REQ-0263 (03 3.13), REQ-0264 (03 3.13), REQ-0265 (03 3.13), REQ-0266
  (03 3.13), REQ-0535 (05 5.3), REQ-0538 (05 5.3), REQ-0544 (05 5.3),
  REQ-1040 (12 12.1), REQ-1101 (12 12.2), REQ-1108 (12 12.3), REQ-1111
  (12 12.3), REQ-1116 (12 12.3), REQ-1148 (12 12.4), REQ-1159 (12 12.4),
  REQ-1160 (12 12.4), REQ-1161 (12 12.4).
- Operator: open-block items 8, 9 and 11 (the two narrowings and the
  surfaces), answered before this task.
- Acceptance:
  - `tools/ci generated` passes with one consumer fewer.
  - The 05 5.3 test asserts the link, not the copy; 05 5.3 names the
    approvals path that `.github/ask-first.yaml` holds after T105.
  - `.github/ask-first.yaml` lists `sbxdrv`, `skills/**` under `kits`,
    and `internal/catalog/**` under `catalog`; the `ledger` surface
    stays while its row is deferred; the `contracts` glob becomes
    `internal/*/errors.go`, `internal/cmd/*/*/errors.go` and
    `internal/spec/versions.go`, with the narrowing of open-block item
    9.
  - The created `docs/grants.yaml` is `version: 1` with `grants: []`.
  - The PR template's surfaces pointer reads `.github/ask-first.yaml`.
- Verify: `go run ./tools/ci generated` and `go run ./tools/ci fast`
  exit 0.
