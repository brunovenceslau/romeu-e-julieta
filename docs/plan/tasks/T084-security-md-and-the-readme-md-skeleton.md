---
id: T084
title: "`SECURITY.md`"
milestone: M6
module: ci-release
fate: deferred
owns: []
needs: []
after:
  - T118
ask_first:
  - checks
sitting: none
---

# T084 - `SECURITY.md`

Earlier title: `SECURITY.md` and the `README.md` skeleton.

Re-plan (2026-10-12): deferred to M6; release and docs. Narrowed to
`SECURITY.md` and its heading list in `tools/ci/headings.yaml`; T118
owns the README list and this task comes after it. Reopened by: the
first adopter other than the maintainer; or the maintainer's
environments all on the new tools. The text below is the one written
before the re-plan; `owns` is written when the task reopens.

- Implements: REQ-0015 (index), REQ-1087 (12 12.2), REQ-1183 (12 12.8),
  REQ-1186 (12 12.8).
- Operator: question 12 is answered in this pull request's review.
- Ask-first, as forecast before the re-plan: `checks`
  (`tools/ci/headings.yaml`).
- Acceptance:
  - `SECURITY.md` has the three headings of rule 20, the channel and the
    response expectation of 12 12.8, and under it the maintainer's own
    text for what happens next, which 12 12.8 says is written with the
    page (question 12); its supported versions section names the tested
    sbx window of 10 10.3.
  - Its heading list is in `tools/ci/headings.yaml`, beside the README
    list of T118 (the nine headings of ADR 0001 rule 12 as ADR 0009
    decision 6 amends it), which does not change after the candidate (11
    11.2).
- Verify: `go test ./tools/ci/... -run '^TestHeadings'`, a read test of
  the file that T086 replaces with the rule 20 check.
