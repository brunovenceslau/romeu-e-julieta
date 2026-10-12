---
id: T003
title: "The ask-first list, `tools/new adr` and `tools/ci generated`"
milestone: M0
module: ci-bootstrap
fate: merged
owns: []
needs:
  - T002
after: []
ask_first: []
sitting: O2
merged: 2026-10-08
pr: 19
---

# T003 - The ask-first list, `tools/new adr` and `tools/ci generated`

Merged on 2026-10-08 in pull request #19. The text below is the one
written before the re-plan.

- Implements: 05 5.3, 12 12.3, 10 10.2 (step 3; the `generated` step).
- Operator: no.
- Ask-first, as forecast before the re-plan: none exists before this
  task.
- Acceptance:
  - `.github/ask-first.yaml` holds the surfaces of 05 5.3 and lists
    itself.
  - `go generate ./...` writes two of the three consumers of that file
    (12 12.3), `.github/CODEOWNERS` and `docs/reference/ask-first.md`,
    and, from the ADR titles and statuses, `docs/adr/README.md`. The
    third consumer of the ask-first file, the `mutate` trigger, lands
    with `mutate` in T007.
  - `tools/ci generated` fails a changed, a new and a removed generated
    file, comparing content before and after the run (10 10.2).
  - `go run ./tools/new adr <title>` writes the next number, the date
    from the injected clock and the filename from `slug(title)`, status
    Proposed (12 12.3; ADR 0001 rule 6).
  - `.github/pull_request_template.md` has the five sections of 12 12.7.
- Verify: `go run ./tools/ci generated`; `go test ./tools/new/...`.
