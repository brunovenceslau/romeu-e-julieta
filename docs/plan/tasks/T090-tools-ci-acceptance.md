---
id: T090
title: "`tools/ci acceptance`"
milestone: M6
module: ci-release
fate: deferred
owns: []
needs:
  - T088
after: []
ask_first:
  - checks
sitting: none
---

# T090 - `tools/ci acceptance`

Re-plan (2026-10-12): deferred to M6; release only. Reopened by: the
first adopter other than the maintainer; or the maintainer's
environments all on the new tools. The text below is the one written
before the re-plan; `needs` lists its earlier dependencies with a folded
id replaced by the id that absorbed it, and `owns` is written when the
task reopens.

- Implements: 10 10.5, 10 10.2 (the subcommands outside `all`), 11 11.2,
  S6.
- Operator: no.
- Ask-first, as forecast before the re-plan: `checks`.
- Acceptance:
  - Against a recorded API fixture, each evidence kind is verified as 10
    10.5 says.
  - On the product's own file the four end-of-plan checks run: the S6
    tags per row of 05 5.2; a result per probe id of 11, from both hosts
    when arch-sensitive; a guide page per journey; block B results that
    count for v1.0.0. They join `all` only in the last task, T102, so
    `all` stays green while the plan is under way (10 10.5).
  - It fails while a `default-overturned` result has no `resolvedBy`,
    and when a block A result's `sbxVersion` or `upstream` version
    differs from the current floor or pin (11 11.4).
  - `acceptance --pre-tag` runs the four checks offline against the
    commit about to be tagged, else it exits non-zero; it is step 3 of
    the release checklist of 12 12.2, which block O7 runs.
  - `schemas/acceptance.v1.json` is generated.
- Verify: `go test ./tools/ci/... -run '^TestAcceptance'`, which lists
  the tests by name.
