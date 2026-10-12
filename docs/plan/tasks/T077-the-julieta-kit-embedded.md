---
id: T077
title: "The `julieta` kit, embedded"
milestone: M2
module: kits
fate: kept
owns:
  - kits/julieta/**
  - kits/os-base/**
  - e2e/kit_julieta_test.go
needs:
  - T076
  - T012
  - T013
after: []
ask_first:
  - kits
sitting: none
---

# T077 - The `julieta` kit, embedded

Earlier title: The `julieta` and `os-base` kits, embedded.

Goal: The `julieta` kit: mise and herdr at the pins, the install-phase
domains, the env lines.

Re-plan (2026-10-12): Kept. `os-base` lands only if sitting M2 shows a
missing package. herdr is downloaded and verified as 06 6.4 says while
`layout` (T056) stays deferred. Acceptance added: the kit builds and
starts in a daily sandbox (sitting M2); the install steps run once in the
container e2e, whose harness (T013) this task needs and whose cases
live in `e2e/kit_julieta_test.go` (rule 1). Read differently until M6,
in the rows below: "the
descriptor file name and the capability names A11 recorded" are the
ones read by hand at sitting M2 (T113), fixtures with `source: sitting
M2`; "the missing set A12 measured" is the set sitting M2 shows, and
`os-base` lands only if it is not empty; A11 and A12 re-measure both
in M6. The embed and materialization row below is asserted by T038 and
T043 (M3) when the embed seam and the `sync` sessions land; at this
task the kit is built and started by hand at sitting M2 (T113) and
used by the wave 1 sandboxes (T115).

- Implements: 06 6.4, 02 2.1 (Embedding), 07 7.2, S3.
- Operator: no.
- Acceptance:
  - The two kits use the descriptor file name and the capability names
    A11 recorded.
  - `julieta` installs mise and herdr at the pins of the pin file and
    declares the two install-phase domains of 06 6.4; the `os-base`
    package list is the missing set A12 measured.
  - romeu embeds `kits/*`, and `sync` materializes them into
    content-addressed directories; the e2e reuses the `sync` sessions of
    T043.
  - The install steps of the `julieta` kit run once in the container e2e
    as a smoke test, under the network rule of 10 10.1 and Q27; a
    kit build itself is what B2 measures (10 10.3, Cannot model).
  - That they build and start is measured by B2 (S3).
- Verify: `go run ./tools/ci kits`; `go test -tags e2e ./e2e/... -run
  '^TestKitJulieta' -v`. The pattern names the `TestKitJulieta*`
  functions this task adds, and the reviewer reads them in the `-v`
  output (10 10.8).
