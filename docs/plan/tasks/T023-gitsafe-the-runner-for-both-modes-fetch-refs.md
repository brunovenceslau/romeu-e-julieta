---
id: T023
title: "`gitsafe`: the runner for both modes, fetch, refs and bundles"
milestone: M1
module: gitsafe
fate: kept
owns:
  - internal/gitsafe/**
needs: []
after: []
ask_first:
  - gitsafe
sitting: none
---

# T023 - `gitsafe`: the runner for both modes, fetch, refs and bundles

Earlier title: `gitsafe`: the hardened runner, fetch and refs.

Goal: `gitsafe`, the part every caller needs: the hardened runner, its
environment and flags in both modes of 05 5.1, the allowlisted ref
namespaces, fetch and fast-forward, create-only refs, bundle creation
(`git bundle create` under the hardened flags, for `julieta snapshot`,
T060) and the floor file.

Re-plan (2026-10-12): Kept, split by verb. The runner serves both
modes: julieta's sandbox calls and romeu's host clone (T041, which
needs this task), so the host-mode rows below (`caFile`, the
destination prefixes, the host flag list) land here. Bundle creation
lands here too, since `julieta snapshot` (T060, M1) calls it. Only four
verbs are split out: unbundle, verify, fsck and the tree walk, T024
(M3), which owns `internal/gitsafe/unbundle*.go`, `verify*.go`,
`fsck*.go` and `walk*.go` under this task's directory, so the two file
sets stay disjoint. The floor file is written
here from the git floor read on the host at sitting M2, as a fixture
with `source: sitting M2`, until T027 re-measures it. Acceptance kept:
the I3 plant list, the neutralized-key list test, nothing ambient
passes, `git://` and `file://` refused.

- Implements: 05 5.1, 05 5.2 (I3, I5, I6), 02 2.3 (the ref namespaces).
- Operator: no.
- Acceptance:
  - A test asserts the environment and the flags of 05 5.1 on each
    invocation, and that nothing ambient passes through (I3).
  - A `git://` URL whose host is not `127.0.0.1`, a `file://` URL and a
    plain path are refused, with a `caFile` set too.
  - The refspec validator table refuses a destination outside the
    allowlisted prefixes (I5); `CreateRef` fails on an existing ref
    (I6); a fuzz target with committed seeds covers the ref-name checks
    (10 10.1).
  - `floor.go` holds the initial floor 2.45.4; T027 fills the per-series
    list from A1.
  - `BundleCreate` writes a bundle of named refs under the hardened
    flags, excluding the objects reachable from a given base, and a
    test reads it back with plain `git bundle verify`.
- Verify: `go test ./internal/gitsafe/...`.
