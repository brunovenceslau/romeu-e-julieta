---
id: T095
title: "`README.md`, `ARCHITECTURE.md` and `CONTRIBUTING.md`"
milestone: M6
module: docs
fate: deferred
owns: []
needs:
  - T092
  - T094
after: []
ask_first: []
sitting: none
---

# T095 - `README.md`, `ARCHITECTURE.md` and `CONTRIBUTING.md`

Re-plan (2026-10-12): deferred to M6; release and docs. Reopened by: the
first adopter other than the maintainer; or the maintainer's
environments all on the new tools. The text below is the one written
before the re-plan; `needs` lists its earlier dependencies with a folded
id replaced by the id that absorbed it, and `owns` is written when the
task reopens.

- Implements: 12 12.8, S10, ADR 0001 rules 5, 12, 18 and 19.
- Operator: no.
- Ask-first, as forecast before the re-plan: none expected.
- Acceptance:
  - The three pages have the content of their outlines in 12 12.8, under
    headings that `tools/ci/headings.yaml` already fixes; the README's
    links to the guide pages resolve (rule 5), which is why this task
    follows T094.
  - The review report of rule 18 scores each at 16 of 20 or more with no
    zero. **[review]**
- Verify: `go run ./tools/ci docs`.
