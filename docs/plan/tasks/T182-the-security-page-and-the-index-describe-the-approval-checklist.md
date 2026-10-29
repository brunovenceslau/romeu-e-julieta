---
id: T182
title: "the security page and the index describe the approval checklist"
milestone: M2
module: ci-bootstrap
fate: new
owns:
  - docs/spec/05-security.md
  - docs/spec.md
needs:
  - T180
after:
  - T167
ask_first:
  - spec
sitting: none
---

# T182 - the security page and the index describe the approval checklist

Goal: 05 and the index of the specification describe the approval line
as T180 leaves it in 12 12.4: a checklist line inside one
`> [!IMPORTANT]` alert, whose tick by the maintainer is the approval,
and a grant line that needs no tick. Decided on 2026-10-29, scope this
repository: a small task now, rather than leaving the old wording to
T005 (M6), so 05 and 12 12.4 do not state two rules.

1. The row of 05 5.4 that starts "The approval line in a PR body is
   typed by the PR's author" says that the author writes the line and
   the maintainer ticks it, and that anyone who can edit the body can
   tick a box, so the check proves the approval was recorded, not who
   gave it.
2. The index row on the signed approval record (`docs/spec.md`, the
   Deferred decisions table) says the same of the approval line.
3. Each other sentence of 05 that names the approval line is read and
   aligned with 12 12.4; the rows of 05 5.4 on `tools/ci pr` landing
   stay with T181 and T005, and REQ-0541 with T005.
4. After T167, which owns 05 too; ADR 0001 rule 8 points to 12 12.4
   and is not rewritten.

- Implements: none: a wording change that keeps 05 and the index in
  line with REQ-1144 and REQ-1162 of 12 12.4, which T180 implements.
- Operator: no.
- Acceptance:
  - `grep -n "typed by the PR's author" docs/spec/05-security.md
    docs/spec.md` finds nothing.
  - `go run ./tools/ci hygiene` and `go run ./tools/ci reqs` pass.
- Verify: `go run ./tools/ci fast`.
