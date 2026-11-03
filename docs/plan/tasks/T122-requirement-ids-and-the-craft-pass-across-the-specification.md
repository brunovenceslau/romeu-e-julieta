---
id: T122
title: "Requirement ids and the craft pass across the specification"
milestone: M0
module: docs
fate: merged
owns:
  - docs/spec.md
  - docs/spec/**
  - docs/adr/0012-*.md
  - docs/adr/0001-*.md
  - docs/adr/0004-*.md
  - docs/adr/0006-*.md
  - docs/adr/0011-*.md
  - docs/notes/maintainer-block-of-ci-bootstrap.md
  - docs/reviews/lenses.md
  - docs/reviews/round-18.md
  - .github/ask-first.yaml
  - .github/CODEOWNERS
  - docs/reference/ask-first.md
  - docs/notes/delivery-metrics-design.md
  - docs/notes/dora-roi-report.md
needs:
  - T121
after: []
ask_first:
  - spec
  - decisions
  - ask-first
sitting: none
merged: 2026-10-13
pr: 45
---

# T122 - Requirement ids and the craft pass across the specification

Merged on 2026-10-13 in pull request #45. The text below is the one
written by the re-plan.

Goal: The write pass of the stage B review of the specification: every
edit of its change set, one `REQ-new` placeholder per obligation placed,
then `go run ./tools/ci reqs allocate` once; ADR 0012, the record that
amends the earlier records for the plan's shape; the review log and
front matter of the note on the maintainer block; the stage B criteria
and the five plan lenses L17 to L21 of `docs/reviews/lenses.md`. The
amendments the re-plan carried for T108 are applied here, so T108 keeps
the record only. `owns` names the one note it edits (the bridge note
`docs/notes/julieta-by-hand.md` is T104's) and
`docs/reviews/round-18.md` for one line of its Done list. The `catalog`
surface glob becomes `internal/catalog/**` here, in
`.github/ask-first.yaml` and its two generated outputs, since the
`askfirst` check holds 05 5.3 and the yaml equal and the spec row
cannot move alone; T107 keeps the rest of item 11 of Decisions of the
re-plan.
The two notes under `docs/notes/` are edited for the heading renames
the pass made.

Re-plan (2026-10-12, stage B): New. The specification ends this pass
with a stable id on each obligation a test, a check, a generator, a
probe or a task's acceptance can verify, so the plan can cite it by id
and a check can hold both directions.

Amended on 2026-11-01 (the release and upgrade amendments, ADR 0044):
REQ-1051 (12 12.1) gained git-cliff; T192 pins it.

- Implements: REQ-0024 (index), REQ-1040 (12 12.1), REQ-1051 (12 12.1),
  REQ-1101 (12 12.2), REQ-1105 (12 12.3), REQ-1108 (12 12.3), REQ-1148
  (12 12.4), REQ-1159 (12 12.4), REQ-1198 (12 12.11).
- Operator: the operator decisions the pass raised, answered before the
  pull request opens; the measurement of the sandbox hosts of 10 10.8 if
  a sandbox run is possible.
- Acceptance:
  - `go run ./tools/ci reqs` exits 0 after `allocate`; no `REQ-new` is
    left; the id count equals the obligations the pass numbered: 1233
    at this pass, 8 of them retired in the table of 12 12.11 (so 1225
    live), measured with `grep -oh '\`REQ-[0-9]\{4\}\`' docs/spec.md
    docs/spec/*.md | sort -u | wc -l`.
  - Every anchor the change set cites resolves, and every citer of a
    renamed heading (03 3.5, 11 11.1) resolves (a one-off link check run
    outside the tree, as T108's acceptance has it).
  - Each citation of ADR 0011 carries the word `proposed` in its
    sentence; no `T<nnn>`, `O<n>` or `checkpoint C<n>` token is left in
    `docs/spec.md` or `docs/spec/` (`grep -nE
    '\bT[0-9]{3}\b|\bO[0-9]\b|checkpoint C[0-9]'` prints nothing).
  - `grep -h '^## Session checkpoint (' docs/reviews/round-*.md` and the
    two review entries moved to the note agree.
  - The five lenses L17 to L21 are in `lenses.md` under "The lenses",
    each between 280 and 400 words, and the stage B row names them.
  - Before the pull request opens, the writer reads the `generated` row
    of 10 10.2, the `.adr-dir` mention and the ADR title rule against
    `tools/ci` at 1e63d6a and fixes a sentence that differs (round 18,
    next step 9).
  - The Done list of `docs/reviews/round-18.md` names ship gate round 2
    (f31b3f9, NO-GO, three findings, applied at eedda13).
- Verify: `go run ./tools/ci fast` exits 0; `go run ./tools/ci reqs`
  exits 0.
