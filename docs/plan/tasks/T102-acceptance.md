---
id: T102
title: "Acceptance"
milestone: M6
module: ci-release
fate: deferred
owns: []
needs:
  - T094
  - T095
  - T101
  - T093
after: []
ask_first:
  - checks
sitting: O7
---

# T102 - Acceptance

Re-plan (2026-10-12): deferred to M6; release only; T091's v1 catalog
table folds into its acceptance. Reopened by: the first release
candidate. The text below is the one written before the re-plan; `needs`
lists its earlier dependencies with a folded id replaced by the id that
absorbed it, and `owns` is written when the task reopens.

- Implements: index ("Success criteria"), 10 10.5, 10 10.2 (`links`,
  `acceptance`), 11 11.2, 12 12.2 (step 6 of the release checklist).
- Operator: block O7 comes first.
- Ask-first, as forecast before the re-plan: `checks`
  (`docs/acceptance.json`).
- Acceptance:
  - `go run ./tools/ci links` ran once, and a dead link it finds is
    fixed or reopens the deferred row about a scheduled check.
  - `docs/acceptance.json` has one entry per success criterion, and the
    release and CI-run items name the release and the run of the v1.0.0
    commit.
  - The result files of the B1 and B2 runs on v1.0.0, written by the
    harness through its redaction, are under the pull request's
    Evidence, and no raw run output is; `go run ./tools/ci probes
    --require-pass B1,B2 --hosts 1 --dir <that directory>` exited 0 on
    them in block O7. **[review]**
  - The four end-of-plan checks of 10 10.5 join `all` in this pull
    request.
- Verify: `go run ./tools/ci acceptance` exits 0.
