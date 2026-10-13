---
id: T094
title: "The guide pages"
milestone: M6
module: docs
fate: deferred
owns: []
needs:
  - T075
after: []
ask_first: []
sitting: none
---

# T094 - The guide pages

Re-plan (2026-10-12): deferred to M6; release and docs, before the
candidate: it needs T075, which completes the scenario functions the
pages cite. Reopened by: the first adopter other than the maintainer; or
the maintainer's environments all on the new tools. The text below is
the one written before the re-plan; `owns` is written when the task
reopens.

- Implements: REQ-0015 (index), REQ-0719 (08 8.5), REQ-0723 (09
  (opening)), REQ-0730 (09 (opening)), REQ-0909 (10 10.2), REQ-1080 (12
  12.2), REQ-1088 (12 12.2), REQ-1108 (12 12.3), REQ-1184 (12 12.8),
  REQ-1187 (12 12.8), REQ-1188 (12 12.8).
- Operator: no.
- Ask-first, as forecast before the re-plan: none (`docs/guide/` is on
  no surface).
- Acceptance:
  - Each `## J<n>` heading of 09 is named by the `journey:` of a page,
    J14's among them.
  - Each `romeu` or `julieta` command in an `sh` block equals an entry
    of the journey's `commands.yaml`, or J1 step 2's verification entry,
    which this task does not change: the completeness test of T041 kept
    those files whole as the commands landed.
  - `docs/guide/recovery.md` has one section per id whose recovered-in
    cell names it; each anchor a cell names resolves; the J1 page says
    what to check by hand for an editor other than VS Code; the J14 page
    lists the paths and rules the operator removes.
  - The J5 page's sentence on the hook order C5 recorded (Q22) is T093's
    post-block-B edit.
  - Each page gives the expected output and the recovery from each
    expected error id. **[review]**: no check reads which error ids a
    page names.
- Verify: `go run ./tools/ci docs`.
