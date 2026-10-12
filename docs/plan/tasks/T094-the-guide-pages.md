---
id: T094
title: "The guide pages"
milestone: M6
module: docs
fate: deferred
owns: []
needs:
  - T092
after: []
ask_first: []
sitting: none
---

# T094 - The guide pages

Re-plan (2026-10-12): deferred to M6; release and docs. Reopened by: the
first adopter other than the maintainer; or the maintainer's
environments all on the new tools. The text below is the one written
before the re-plan; `needs` lists its earlier dependencies with a folded
id replaced by the id that absorbed it, and `owns` is written when the
task reopens.

- Implements: 12 12.8 (`docs/guide/*`), 09, S10, ADR 0001 rules 1, 2 and
  13.
- Operator: no.
- Ask-first, as forecast before the re-plan: none (`docs/guide/` is on
  no surface).
- Acceptance:
  - Each `## J<n>` heading of 09 is named by the `journey:` of a page.
  - Each `romeu` or `julieta` command in an `sh` block equals an entry
    of `e2e/scenarios/commands.yaml`, which this task does not change:
    the completeness test of T041 kept that file whole as the commands
    landed.
  - The J5 page describes the hook order C5 recorded (Q22).
  - Each page gives the expected output and the recovery from each
    expected error id. **[review]**: no check reads which error ids a
    page names.
- Verify: `go run ./tools/ci docs`.
