---
id: T006
title: "Contributor page skeletons, the lessons file, `tools/ci lessons` and the first rules of `tools/ci docs`"
milestone: M6
module: ci-bootstrap
fate: deferred
owns: []
needs:
  - T005
after: []
ask_first:
  - checks
sitting: none
---

# T006 - Contributor page skeletons, the lessons file, `tools/ci lessons` and the first rules of `tools/ci docs`

Re-plan (2026-10-12): deferred to M6; checks before the documents exist.
Reopened by: the first outside contributor; or the first release
candidate. The issue forms of 12 12.7 are T124's. The text below is the
one written before the re-plan; `needs` lists its earlier dependencies
with a folded id replaced by the id that absorbed it, and `owns` is
written when the task reopens.

- Implements: REQ-0023 (index), REQ-0123 (02 2.1), REQ-0850 (10 10.2),
  REQ-0853 (10 10.2), REQ-1083 (12 12.2), REQ-1084 (12 12.2), REQ-1085
  (12 12.2), REQ-1114 (12 12.3), REQ-1119 (12 12.3), REQ-1125 (12 12.3),
  REQ-1166 (12 12.5), REQ-1181 (12 12.7), REQ-1184 (12 12.8), REQ-1185
  (12 12.8).
- Operator: no.
- Ask-first, as forecast before the re-plan: `checks`.
- Acceptance:
  - `ARCHITECTURE.md` and `CONTRIBUTING.md` exist with front matter (ADR
    0001 rule 1) and the headings of their outlines.
  - `docs/lessons/0001-<slug>.md` exists, the first lesson, what the
    bootstrap taught, written by `go run ./tools/new lesson`, with the
    generated index `docs/lessons/README.md` (12 12.7). The kind, the
    generator row and the check land here because this lesson is their
    first input (12 12.2).
  - `tools/ci lessons` fails a lesson that names no existing subcommand
    or test and gives no "no check possible" reason.
  - `tools/ci docs` lands here with its first inputs, by the rule
    "Middleware lands with its first input": a fixture Markdown file
    fails for a missing or invalid front matter key (rule 1) and for a
    relative link or anchor that does not resolve (rule 5); text in a
    code span is not a link. The rule 9 matcher lands here too and runs
    over the spec and the records: a first mention of a record that is
    not its link form fails, and the spec passes it. `docs/spec.md` and
    the pages under `docs/spec/` move to front matter in this change
    (ADR 0001, Consequences). The rest of the step lands in T085 and
    T086.
  - `docs/notes/*.md` pass rule 1, and the first ADR mentions of the
    task files take the link form or the rule 9 matcher skips
    `docs/plan/tasks/` (ADR 0012 exempts the task files from rule 1:
    they have their own schema, checked by `tools/ci plan`).
- Verify: `go test ./tools/ci/... -run '^Test(Lessons|Docs)'`, which
  runs the fixture lines; `go run ./tools/ci lessons`; `go run
  ./tools/ci docs`; `go run ./tools/ci generated`; `go test
  ./tools/new/...`.
