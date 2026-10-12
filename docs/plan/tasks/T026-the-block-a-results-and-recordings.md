---
id: T026
title: "The block A results and recordings"
milestone: M6
module: probes
fate: deferred
owns: []
needs: []
after: []
ask_first: []
sitting: O3
---

# T026 - The block A results and recordings

Re-plan (2026-10-12): deferred to M6; block A results and recordings
arrive with the harness; the facts the code needs first come from daily
use and from sitting M2, each marked with its source. Reopened by: a
real-sbx behaviour the fake cannot model breaks a daily run; or the
first release candidate. The text below is the one written before the
re-plan; `needs` lists its earlier dependencies with a folded id
replaced by the id that absorbed it, and `owns` is written when the task
reopens.

- Implements: 11 11.1, 11 11.3, 11 11.4, 10 10.3, 10 10.2 ("Forbidden
  names").
- Operator: block O3 produces it; the maintainer opens it.
- Ask-first, as forecast before the re-plan: none for results alone;
  each surface a resolution touches.
- Acceptance:
  - The pull request holds the `probe-result.v1` files under
    `docs/probes/`, from both hosts, and the recordings under
    `e2e/testdata/sbx/<version>/`, and nothing else.
  - It reaches `main` like any change, not by a direct push, because a
    direct push is public before any check reads it: a branch in the
    maintainer's plain host clone with the hook enabled, `go run
    ./tools/ci fast`, the diff of everything staged read by the
    maintainer before the push, then a pull request, so the hook,
    `hygiene`, the pushed range and `pr` read host data before it is
    public.
  - A results-only branch holds no code, so the hook runs `main`'s own
    `tools/ci` on the host. When a resolution commit joins the branch
    (below), the maintainer reads that diff in the plain clone before
    checking the branch out, as item c of block O1 requires of code that
    runs on the host (10 10.2).
  - `tools/ci probes` passes. A result with `decision:
    default-overturned` keeps that step red until it is resolved (11
    11.4), so the commit that applies it, its ADR and the `resolvedBy`
    commit join this pull request; that is the one case where it holds
    more than results.
  - Pass, for block O3: `go run ./tools/ci probes --require-pass
    A1,A2,A3,A4,A5,A6,A9,A10,A11,A12,A13,A14` exits 0 on the branch with
    both hosts' results; a `fail` or `inconclusive` verdict does not
    merge.
  - While the branch holds results alone, `git diff --name-only
    origin/main...HEAD` lists only paths under `docs/probes/` and
    `e2e/testdata/sbx/`.
  - The sha256 of each `sbxenv.yaml` golden is in the A5 result, one per
    file under `internal/render/testdata/`, as T019 defines A5.
- Verify: `go run ./tools/ci fast`; `go run ./tools/ci probes
  --require-pass A1,A2,A3,A4,A5,A6,A9,A10,A11,A12,A13,A14`.
