---
id: T084
title: "`SECURITY.md` and the `README.md` skeleton"
milestone: M6
module: ci-release
fate: deferred
owns: []
needs:
  - T006
after: []
ask_first:
  - checks
sitting: none
---

# T084 - `SECURITY.md` and the `README.md` skeleton

Re-plan (2026-10-12): deferred to M6; release and docs. Reopened by: the
first adopter other than the maintainer; or the maintainer's
environments all on the new tools. The text below is the one written
before the re-plan; `needs` lists its earlier dependencies with a folded
id replaced by the id that absorbed it, and `owns` is written when the
task reopens.

- Implements: 12 12.8, 12 12.2 (the `ci-release` row), ADR 0001 rules 12
  and 20.
- Operator: question 12 is answered in this pull request's review.
- Ask-first, as forecast before the re-plan: `checks`
  (`tools/ci/headings.yaml`).
- Acceptance:
  - `SECURITY.md` has the three headings of rule 20, the channel and the
    response expectation of 12 12.8, and under it the maintainer's own
    text for what happens next, which 12 12.8 says is written with the
    page (question 12).
  - `README.md` has the heading order of rule 12, with the strings fixed
    here.
  - Both heading lists are in `tools/ci/headings.yaml`, which does not
    change after the candidate (11 11.2).
- Verify: `go test ./tools/ci/... -run '^TestHeadings'`, a read test of
  the two files that T086 replaces with the rule 12 and rule 20 checks.
