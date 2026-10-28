---
id: T061
title: "`julieta salvage`, the sandbox half"
milestone: M4
module: salvage
fate: kept
owns:
  - internal/salvage/capture*.go
  - internal/salvage/fingerprint*.go
  - internal/salvage/manifest*.go
  - internal/salvage/doc.go
  - internal/salvage/testdata/**
  - internal/cmd/julieta/salvage/**
  - internal/cmd/julieta/all/import_salvage.go
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

Re-plan (2026-10-12): Kept. `owns` names its files by glob, so rule 1 of
`docs/plan.md` holds for T060 (`snapshot*.go`), this task and T062
(`verify*.go`, `import*.go`) in one package. `internal/salvage` holds
the capture, the fingerprint and the manifest writer and imports neither
`agent/*` nor `memstore/write`; `cmd/julieta` passes the agent profile
and a lock-wait function from `memstore/write`, and the romeu half
(T062) stays in the same package. `agent-not-matched` follows the
open-session rule of julieta state: a session is open when the state
records a SessionStart with no later SessionEnd (08 8.2, 08 8.5). Two
alternatives were weighed and not taken: reading the pane process of
`julieta pane run`, which needs `layout` (deferred), and reading the
agent's open files, which no measurement on the pinned agent supports
yet.

- Implements: REQ-0103 (01 1.6), REQ-0157 (02 2.6), REQ-0223 (03 3.6),
  REQ-0236 (03 3.8), REQ-0246 (03 3.11), REQ-0248 (03 3.11), REQ-0401
  (04 4.3), REQ-0417 (04 4.3), REQ-0501 (05 5.2), REQ-0502 (05 5.2),
  REQ-0521 (05 5.2), REQ-0522 (05 5.2), REQ-0523 (05 5.2), REQ-0524 (05
  5.2), REQ-0525 (05 5.2), REQ-0526 (05 5.2), REQ-0527 (05 5.2),
  REQ-0528 (05 5.2), REQ-0529 (05 5.2), REQ-0530 (05 5.2), REQ-0531 (05
  5.2), REQ-0532 (05 5.2), REQ-0533 (05 5.2), REQ-0534 (05 5.2),
  REQ-0552 (05 5.4), REQ-0694 (08 8.5), REQ-0695 (08 8.5), REQ-0696 (08
  8.5), REQ-0697 (08 8.5), REQ-0698 (08 8.5), REQ-0699 (08 8.5),
  REQ-0700 (08 8.5), REQ-0701 (08 8.5), REQ-0703 (08 8.5), REQ-0704 (08
  8.5), REQ-0705 (08 8.5), REQ-0706 (08 8.5), REQ-0707 (08 8.5),
  REQ-0708 (08 8.5), REQ-0709 (08 8.5), REQ-0710 (08 8.5), REQ-0739 (10
  10.1), REQ-0742 (10 10.1), REQ-0744 (10 10.1), REQ-0745 (10 10.1),
  REQ-0746 (10 10.1).
- Operator: no.
- Acceptance:
  - In the container, each salvage completeness case of I17's Test cell
    (05 5.2, the one list) that the sandbox half reaches, cited by its
    title, and the three cases 10 10.1 adds: a fixture process with the
    agent's name that ignores SIGTERM is killed after the wait and the
    manifest's `agentStop` records it; untracked files count against the
    salvage cap; a nested repository gets the dirt and stash capture.
  - A rebase in progress is skipped as `in-progress-operation`, and a
    full disk in step 2 as `disk-full`.
  - Salvage commits carry author `julieta-salvage` and leave the
    worktree and index unchanged; a salvage dir without `manifest.json`
    is ignored by verification and kept.
  - Salvage commits carry no signature with `commit.gpgsign=true` set
    (I18).
  - After `--stop-agents`, salvage waits on the store lock of every
    manifest memory dir; a stamp written while it waits is in
    the dir and is not counted for completeness; while it waits more
    than 10 s on a store lock it prints the lock's path on stderr and in
    `--json` progress; the wait has no bound.
  - A missing required profile path is the skipped item
    `agent-path-missing`; a missing optional path is no item.
  - The two `agent-not-matched` cases: a salvage with no agent process
    and a recorded SessionStart without SessionEnd skips
    `agent-not-matched`; the same with a later SessionEnd records zero
    and skips nothing.
  - Salvage dirs are 0700 and payload files 0600 (08 8.5).
  - `salvage` does not import `agent/*` and takes the profile as an
    argument; `schemas/salvage.v1.json` is generated.
  - `--fingerprint` writes nothing, stops no agent, and prints the
    fingerprint object of 04 4.3 and nothing else; the full salvage's
    `--json` document carries the same object (round 18, exchange 1,
    finding 1 and A4).
  - The package takes the clock and the ULID entropy source as injected
    values, every ULID comes from `canon.NewULID`, and its tests fix
    both (10 10.4).
- Verify: `go test -tags e2e ./e2e/... -run '^TestJulietaSalvage' -v`.
  The pattern names the `TestJulietaSalvage*` functions this task adds,
  and the reviewer reads them in the `-v` output (10 10.8).
