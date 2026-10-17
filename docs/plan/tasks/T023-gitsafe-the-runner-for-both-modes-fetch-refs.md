---
id: T023
title: "`gitsafe`: the runner for both modes, fetch, refs and bundles"
milestone: M1
module: gitsafe
fate: kept
owns:
  - internal/gitsafe/**
needs:
  - T137
after: []
ask_first:
  - gitsafe
  - contracts
sitting: none
---

# T023 - `gitsafe`: the runner for both modes, fetch, refs and bundles

Earlier title: `gitsafe`: the hardened runner, fetch and refs.

Goal: `gitsafe`, the part every caller needs: the hardened runner, its
environment and flags in both modes of 05 5.1 (sandbox mode adds the
closed network pass-through list of REQ-0470, with a test that pins it),
the allowlisted ref namespaces, a `Clone` verb in both modes (host:
`romeu init`'s config clone, T041; sandbox: the secondary clones of
`julieta setup`, REQ-0398), fetch and fast-forward, create-only refs,
bundle creation
(`git bundle create` under the hardened flags, for `julieta snapshot`,
T060) and the floor file.

Re-plan (2026-10-12): Kept, split by verb. The runner serves both modes:
julieta's sandbox calls and romeu's host clone (T041, which needs this
task), so the host-mode rows below (`caFile`, the destination prefixes,
the host flag list) land here. Bundle creation lands here too, since
`julieta snapshot` (T060, M1) calls it. Only four verbs are split out:
unbundle, verify, fsck and the tree walk, T024 (M3), which owns
`internal/gitsafe/unbundle*.go`, `verify*.go`, `fsck*.go` and `walk*.go`
under this task's directory, so the two file sets stay disjoint. The
floor file holds the initial floor 2.45.4 of 05 5.1 until T027 fills the
per-series list from A1. Every git call runs through
`internal/procgroup` (T137). Acceptance kept: the I3 plant list, the
neutralized-key list test, nothing ambient passes (bar the sandbox
network list), `git://` and
`file://` refused.

- Implements: REQ-0017 (index), REQ-0053 (01 1.3), REQ-0143 (02 2.3),
  REQ-0146 (02 2.3), REQ-0208 (03 3.4), REQ-0222 (03 3.6), REQ-0296 (04
  4.1), REQ-0297 (04 4.1), REQ-0302 (04 4.1), REQ-0364 (04 4.2),
  REQ-0396 (04 4.3), REQ-0442 (04 4.4), REQ-0458 (05 5.1), REQ-0459 (05
  5.1), REQ-0460 (05 5.1), REQ-0461 (05 5.1), REQ-0462 (05 5.1),
  REQ-0463 (05 5.1), REQ-0464 (05 5.1), REQ-0467 (05 5.1), REQ-0469 (05
  5.1), REQ-0470 (05 5.1), REQ-0471 (05 5.1), REQ-0476 (05 5.2),
  REQ-0479 (05 5.2), REQ-0483 (05 5.2), REQ-0484 (05 5.2), REQ-0486 (05
  5.2), REQ-0487 (05 5.2), REQ-0497 (05 5.2), REQ-0502 (05 5.2),
  REQ-0694 (08 8.5), REQ-0789 (10 10.1), REQ-0793 (10 10.1), REQ-0957
  (10 10.6), REQ-1057 (12 12.1).
- Operator: no.
- Acceptance:
  - A test asserts the environment and the flags of 05 5.1 on each
    invocation, and that nothing ambient passes through (I3), except in
    sandbox mode the closed network pass-through list of REQ-0470
    (`HTTPS_PROXY`, `HTTP_PROXY`, `NO_PROXY`, `https_proxy`,
    `http_proxy`, `no_proxy`, `SSL_CERT_FILE`) and never a `GIT_*`
    variable; one test pins that list exactly and shows host mode
    passes none of it.
  - `Clone` runs in both modes under the hardened flags with an
    explicit URL: a host-mode clone of a config repo (REQ-0309, T041)
    and a sandbox-mode clone through the proxy environment (REQ-0398)
    both succeed in a test, and a refused URL is refused in each mode.
    The sandbox-mode test clones through a local HTTPS proxy, which
    alone reaches the server's host name, from a server whose
    certificate only a test CA trusts; the CA reaches git through the
    sandbox's global config (`http.sslCAInfo`), since git's libcurl
    may ignore `SSL_CERT_FILE`, and the same clone fails without it.
  - One test holds the plant list of I3, the host git keys `doctor`
    warns on and gitsafe's neutralized keys equal, all three read from
    one Go slice in `internal/gitsafe` (10 10.1, Git keys); T049's row
    reads that slice. This task ships the slice (`NeutralizedKeys`) and
    pins it to the list of 05 5.1; the three-way test lands with the
    first of T049 and the I3 plant list (deferred; trigger: T049
    starts).
  - The keys that make git run a program from a host clone's local
    config, `core.askPass` and `core.alternateRefsCommand` among them,
    run nothing under host mode, each shown against a positive control
    with plain git (05 5.1, REQ-0463).
  - A `git://` URL whose host is not `127.0.0.1`, a `file://` URL and a
    plain path are refused, with a `caFile` set too.
  - `-c credential.helper=<value>` is passed only when
    `tools.gitCredentialHelper` is set; a `gitHosts[].caFile` becomes
    `-c http.sslCAInfo=<path>` for that host only.
  - A fetch argv carries `--no-tags --no-prune --no-recurse-submodules
    --no-write-fetch-head` and an explicit URL with refspecs, never a
    remote name.
  - A local `url.<base>.insteadOf` rewriting to a refused protocol, or
    to `git://127.0.0.1` for a URL that was not, is refused; one to
    another HTTPS host is not (05 5.1).
  - The refspec validator table refuses a destination outside the
    allowlisted prefixes (I5); `CreateRef` fails on an existing ref
    (I6); a fuzz target with committed seeds covers the ref-name checks
    (10 10.1).
  - `floor.go` holds the initial floor 2.45.4; T027 fills the per-series
    list from A1. The floor check before gate 1 exits 2 with `git-floor`
    (`RJ-344`).
  - Every git call runs through `internal/procgroup` with the git class
    of 04 4.1 Starting values, and a test proves `subprocess-timeout`
    (`RJ-323`) fires through the table's seam.
  - `BundleCreate` writes a bundle of named refs under the hardened
    flags, excluding the objects reachable from a given base, and a test
    reads it back with plain `git bundle verify`.
- Verify: `go test ./internal/gitsafe/...`.
