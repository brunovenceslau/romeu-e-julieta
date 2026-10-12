---
id: T061
title: "`julieta salvage`, the sandbox half"
milestone: M4
module: salvage
fate: kept
owns:
  - internal/salvage/**
  - internal/cmd/julieta/salvage/**
  - cmd/julieta/import_salvage.go
  - e2e/julieta_salvage_test.go
needs:
  - T060
  - T059
  - T079
after: []
ask_first: []
sitting: none
---

# T061 - `julieta salvage`, the sandbox half

Goal: `julieta salvage`, the sandbox half: the complete salvage of 08
8.5, the fingerprint, the agent profile paths, I17's completeness cases.

Re-plan (2026-10-12): Kept. `snapshot*.go` belongs to T060 and
`verify*.go`, `import*.go` to T062.

- Implements: 08 8.5 ("Sandbox half"), 03 3.11, 04 4.3 (`salvage`), 05
  5.2 (I18), 10 10.7.
- Operator: no.
- Acceptance:
  - In the container, the salvage completeness cases of 10 10.1: dirt in
    a linked worktree, a stash, a detached HEAD, a nested repository, an
    ignored file over the cap listed as `over-cap`, a transcript listed
    as `transcript-excluded`.
  - Salvage commits carry no signature with `commit.gpgsign=true` set
    (I18).
  - `salvage` does not import `agent/*` and takes the profile as an
    argument; `schemas/salvage.v1.json` is generated.
  - `--fingerprint` writes nothing, stops no agent, and prints the
    fingerprint object of 04 4.3 and nothing else; the full salvage's
    `--json` document carries the same object (round 18, exchange 1,
    finding 1 and A4).
- Verify: `go test -tags e2e ./e2e/... -run '^TestJulietaSalvage' -v`.
  The pattern names the `TestJulietaSalvage*` functions this task adds,
  and the reviewer reads them in the `-v` output (10 10.8).
