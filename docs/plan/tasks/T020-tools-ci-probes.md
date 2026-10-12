---
id: T020
title: "`tools/ci probes`"
milestone: M6
module: probes
fate: deferred
owns: []
needs:
  - T014
after: []
ask_first:
  - checks
sitting: none
---

# T020 - `tools/ci probes`

Re-plan (2026-10-12): deferred to M6; it reads results the harness does
not write yet. Reopened by: a real-sbx behaviour the fake cannot model
breaks a daily run; or the first release candidate. The text below is
the one written before the re-plan; `needs` lists its earlier
dependencies with a folded id replaced by the id that absorbed it, and
`owns` is written when the task reopens.

- Implements: 11 11.4, 10 10.2 (the `probes` step), 10 10.4.
- Operator: no.
- Ask-first, as forecast before the re-plan: `checks`.
- Acceptance:
  - A fixture per row of the lifecycle table of 11 11.4: a result whose
    `decision` differs from the computed one fails; an overturned result
    fails until `resolvedBy` names a commit that descends from the
    result's `commit`, touches each `affects` path and adds an ADR.
  - A golden whose sha256 moved since its A5 result is flagged, and one
    with no recorded hash is not.
  - `probes --require-pass <ids>` exits non-zero when a named id has no
    result or a `verdict` other than `pass`; it needs, for each named
    id, a `pass` result from each of the two host architectures, and
    `--hosts 1` relaxes that for the rehearsal of block O5 and the B1
    run of block O7; a fixture with one host's result fails. It also
    takes `--dir <dir>`, which reads the results of a directory instead
    of `docs/probes/`, for a run whose output is not committed. A result
    or observation marked `rehearsal` never satisfies
    `--require-pass B1`, with or without `--hosts 1`. All three flags
    are this plan's additions: they are the pass criteria of blocks O3,
    O5, O6 and O7, one command whose exit status the maintainer reads.
- Verify: `go test ./tools/ci/... -run '^TestProbes'`, which runs the
  fixture rows; `go run ./tools/ci probes`.
