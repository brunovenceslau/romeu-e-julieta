---
id: T092
title: "The block B results"
milestone: M6
module: probes
fate: deferred
owns: []
needs: []
after: []
ask_first: []
sitting: O6
---

# T092 - The block B results

Re-plan (2026-10-12): deferred to M6; release only. Reopened by: a
real-sbx behaviour the fake cannot model breaks a daily run; or the
first release candidate. The text below is the one written before the
re-plan; `needs` lists its earlier dependencies with a folded id
replaced by the id that absorbed it, and `owns` is written when the task
reopens.

- Implements: 11 11.2, 11 11.3, 11 11.4, 10 10.2 ("Forbidden names").
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
  - `tools/ci probes` passes. A result with `decision:
    default-overturned` keeps that step red until it is resolved (11
    11.4): when every `affects` path is under `docs/` or a root Markdown
    file, the commit that applies it, its ADR and the `resolvedBy`
    commit join this pull request, as in T026; a result that needs a
    change outside those paths (a kit, a catalog entry) is fixed there,
    and blocks O5 and O6 run again on a new candidate (11 11.2).
  - Pass, for block O6: `go run ./tools/ci probes --require-pass
    B1,B2,B3,B4,B5,B6,C2,C3,C4,C5,C6` exits 0 on the branch with both
    hosts' results; a `fail` or `inconclusive` verdict does not merge.
- Verify: `go run ./tools/ci fast`; `go run ./tools/ci probes
  --require-pass B1,B2,B3,B4,B5,B6,C2,C3,C4,C5,C6`.
