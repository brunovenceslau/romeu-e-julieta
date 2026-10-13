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

- Implements: REQ-0199 (03 3.3), REQ-0565 (06 6.1), REQ-0579 (06 6.2),
  REQ-0846 (10 10.2), REQ-0943 (10 10.4), REQ-0979 (11 (opening)),
  REQ-1026 (11 11.3), REQ-1027 (11 11.3), REQ-1028 (11 11.4), REQ-1031
  (11 11.4), REQ-1032 (11 11.4), REQ-1036 (11 11.4), REQ-1069 (12 12.1),
  REQ-1168 (12 12.5), REQ-1172 (12 12.5).
- Operator: no.
- Ask-first, as forecast before the re-plan: `checks`.
- Acceptance:
  - A fixture per row of the lifecycle table of 11 11.4: a result whose
    `decision` differs from the computed one fails; an overturned result
    passes; when `resolvedBy` is set, the check fails unless that commit
    descends from the result's `commit`, touches every `affects` path
    and adds an ADR that supersedes the default's.
  - A committed block A result whose `host.upstream` key differs from
    that pin's current value fails, one fixture per key, each key
    compared on its own.
  - A `docs/guide/` page that names a probe id its `affects` does not
    list fails.
  - Each `affects` anchor of a committed result resolves to a heading of
    the spec, by the resolver of `tools/ci docs`; a fixture with a
    renamed heading fails.
  - A golden whose sha256 moved since its A5 result is flagged, and one
    with no recorded hash prints a notice and passes.
  - `probes --require-pass <ids>` exits non-zero when a named id has no
    result or a `verdict` other than `pass`; it needs, for each named
    id, a `pass` result from each of the two host architectures, and
    `--hosts 1` relaxes that for the rehearsal of block O5 and the B1
    run of block O7; a fixture with one host's result fails. It also
    takes `--dir <dir>`, which reads the results of a directory instead
    of `docs/probes/`, for a run whose output is not committed. A result
    or observation marked `rehearsal` never satisfies `--require-pass
    B1`, with or without `--hosts 1`. All three flags are this plan's
    additions: they are the pass criteria of blocks O3, O5, O6 and O7,
    one command whose exit status the maintainer reads.
- Verify: `go test ./tools/ci/... -run '^TestProbes'`, which runs the
  fixture rows; `go run ./tools/ci probes`.
