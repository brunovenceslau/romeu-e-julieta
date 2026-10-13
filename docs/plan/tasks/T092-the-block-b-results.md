---
id: T092
title: "The block B results"
milestone: M6
module: probes
fate: deferred
owns: []
needs:
  - T082
  - T088
  - T090
after: []
ask_first: []
sitting: O6
---

# T092 - The block B results

Re-plan (2026-10-12): deferred to M6; release only. Reopened by: a
real-sbx behaviour the fake cannot model breaks a daily run; or the
first release candidate. The text below is the one written before the
re-plan; `needs` names the tasks whose code it reads (the definitions of
T082, the release of T088, the check of T090), and `owns` is written
when the task reopens.

- Implements: REQ-0005 (index), REQ-0006 (index), REQ-0008 (index),
  REQ-0010 (index), REQ-0012 (index), REQ-0750 (10 10.1), REQ-1000 (11
  11.2), REQ-1001 (11 11.2), REQ-1003 (11 11.2), REQ-1004 (11 11.2),
  REQ-1005 (11 11.2), REQ-1006 (11 11.2), REQ-1010 (11 11.2), REQ-1028
  (11 11.4), REQ-1094 (12 12.2).
- Operator: block O6 produces it; the maintainer opens it.
- Ask-first, as forecast before the re-plan: none for results alone.
- Acceptance:
  - The pull request holds the results and artifacts under
    `docs/probes/`, from both hosts, and nothing under `e2e/testdata/`
    (11 11.2).
  - It reaches `main` through a branch in a plain host clone, the hook,
    `go run ./tools/ci fast`, the staged diff read before the push, and
    a pull request, for the reasons T026 gives, the read of a resolution
    commit's diff before checkout included.
  - `tools/ci probes` passes. An overturned result lands with the
    results; its fix is its own pull request, and `tools/ci acceptance`
    (T090) fails until `resolvedBy` is set (11 11.4); a result that
    needs a change outside `docs/` and the root Markdown files (a kit, a
    catalog entry) is fixed there, and blocks O5 and O6 run again on a
    new candidate (11 11.2).
  - Pass, for block O6: `go run ./tools/ci probes --require-pass
    B1,B2,B3,B4,B5,B6,C2,C3,C4,C5,C6` exits 0 on the branch with both
    hosts' results; a `fail` or `inconclusive` verdict does not merge.
- Verify: `go run ./tools/ci fast`; `go run ./tools/ci probes
  --require-pass B1,B2,B3,B4,B5,B6,C2,C3,C4,C5,C6`.
