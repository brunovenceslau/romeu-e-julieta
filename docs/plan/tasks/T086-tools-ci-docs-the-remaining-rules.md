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

- Implements: REQ-0015 (index), REQ-0730 (09 (opening)), REQ-0769 (10
  10.1), REQ-0850 (10 10.2), REQ-1080 (12 12.2), REQ-1087 (12 12.2),
  REQ-1108 (12 12.3), REQ-1166 (12 12.5), REQ-1184 (12 12.8), REQ-1187
  (12 12.8), REQ-1188 (12 12.8).
- Operator: no.
- Ask-first, as forecast before the re-plan: `checks`.
- Acceptance:
  - A fixture fails for each remaining case of rule 9: one in a heading,
    one in front matter, one inside another link, a number with no
    record, a plural form (the first-mention matcher landed in T006).
  - A fixture fails for a heading that is missing, extra or out of order
    against `tools/ci/headings.yaml` (rules 12 and 20).
  - A fixture fails for a fenced block with no language, for a `romeu`
    or `julieta` command in an `sh` block that equals no entry of the
    page's journey's `commands.yaml`, or J1 step 2's verification entry,
    and for a guide page whose `journey:` is no heading of 09 (rule 13).
  - A fixture fails `ARCHITECTURE.md` missing an invariant id of 05 5.2
    or a module id of 12 12.2, and an error id whose recovered-in cell
    (12 12.8) is empty or names an anchor that does not resolve, a
    fixture each.
  - A fixture fails for an image and for a file under `docs/` with
    another extension (rule 17); a fixture passes a `.excalidraw` file
    under `docs/notes/` and another fails one elsewhere under `docs/`
    (ADR 0013, decision 1).
- Verify: `go test ./tools/ci/... -run '^TestDocs'`, which runs the
  fixture lines; `go run ./tools/ci docs`.
