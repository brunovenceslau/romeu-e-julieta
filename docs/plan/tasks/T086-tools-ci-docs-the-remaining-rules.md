---
id: T086
title: "`tools/ci docs`: the remaining rules"
milestone: M6
module: ci-release
fate: deferred
owns: []
needs:
  - T085
after: []
ask_first:
  - checks
sitting: none
---

# T086 - `tools/ci docs`: the remaining rules

Re-plan (2026-10-12): deferred to M6; release and docs. Reopened by: the
first adopter other than the maintainer; or the maintainer's
environments all on the new tools. The text below is the one written
before the re-plan; `needs` lists its earlier dependencies with a folded
id replaced by the id that absorbed it, and `owns` is written when the
task reopens.

- Implements: 10 10.2 (the `docs` step), ADR 0001 rules 9, 12, 13, 17
  and 20, S10.
- Operator: no.
- Ask-first, as forecast before the re-plan: `checks`.
- Acceptance:
  - A fixture fails for each remaining case of rule 9: one in a heading,
    one in front matter, one inside another link, a number with no
    record, a plural form (the first-mention matcher landed in T006).
  - A fixture fails for a heading that is missing, extra or out of order
    against `tools/ci/headings.yaml` (rules 12 and 20).
  - A fixture fails for a fenced block with no language, for a `romeu`
    or `julieta` command in an `sh` block that equals no entry of
    `e2e/scenarios/commands.yaml`, and for a guide page whose `journey:`
    is no heading of 09 (rule 13).
  - A fixture fails for an image and for a file under `docs/` with
    another extension (rule 17).
- Verify: `go test ./tools/ci/... -run '^TestDocs'`, which runs the
  fixture lines; `go run ./tools/ci docs`.
