---
id: T095
title: "`README.md`, `ARCHITECTURE.md` and `CONTRIBUTING.md`"
milestone: M6
module: docs
fate: deferred
owns: []
needs:
  - T094
after:
  - T118
ask_first: []
sitting: none
---

# T095 - `README.md`, `ARCHITECTURE.md` and `CONTRIBUTING.md`

Re-plan (2026-10-12): deferred to M6; release and docs; after T118,
which writes the README's install and host setup. Reopened by: the first
adopter other than the maintainer; or the maintainer's environments all
on the new tools. The text below is the one written before the re-plan;
`owns` is written when the task reopens.

Amended on 2026-11-01 (the release and upgrade amendments, ADR 0044):
REQ-0909 (10 10.2) gained `--source-ref refs/heads/main`, and REQ-1058
(12 12.1) names `romeu upgrade` beside J1 as what needs the `gh` floor;
the README's entry comes from J1's `commands.yaml` (T088).

- Implements: REQ-0015 (index), REQ-0909 (10 10.2), REQ-1058 (12 12.1),
  REQ-1087 (12 12.2), REQ-1088 (12 12.2), REQ-1182 (12 12.8), REQ-1183
  (12 12.8), REQ-1184 (12 12.8), REQ-1185 (12 12.8), REQ-1223 (10 10.2).
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
