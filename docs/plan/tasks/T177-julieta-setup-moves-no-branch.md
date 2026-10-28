---
id: T177
title: "julieta setup moves no branch"
milestone: M2
module: julieta-core
fate: new
owns:
  - internal/cmd/julieta/setup/repos.go
  - internal/cmd/julieta/setup/state.go
  - internal/cmd/julieta/handoff/show.go
  - internal/cmd/julieta/status/section_warnings.go
needs:
  - T051
after: []
ask_first: []
sitting: none
---

# T177 - julieta setup moves no branch

Goal: `julieta setup` fetches origin in every repo and moves no branch,
so a checkout stays at the code and locks it was cloned at and `setup`
can run at every boot (T179) without changing the code under a session.
Decided on 2026-10-28 with the specification amended first: REQ-0399
(04 4.3), the warnings list of 04 4.1 and the SessionStart list of 08
8.2 lose `default-branch-moved`.

1. `setup` drops the fast-forward of the local default branch and the
   warning `default-branch-moved`: its report, the state it records, the
   one-time SessionStart line of `handoff show --hook` and the warnings
   section of `julieta status`. A state file written before this task
   that still carries the warning is read and the member ignored
   (REQ-0276 rows are read by slug; T162's unknown-member rule).
2. The fetch, its `fetch-failed` warning and every other step stay as
   they are. `internal/gitsafe` keeps its fast-forward function for its
   other callers, if any; when none is left, its removal is a gitsafe
   task (ask-first `gitsafe`), not this one.

- Implements: REQ-0399 (04 4.3).
- Operator: no.
- Acceptance:
  - A clean `main` checked out and behind origin is fetched and stays at
    its commit after `setup`; the report and `--json` carry no
    `default-branch-moved`; a detached HEAD and a feature branch are
    untouched as before.
  - A state file of before this task with a `default-branch-moved`
    member loads, and neither `status` nor SessionStart prints it.
  - `grep -rn default-branch-moved internal e2e docs/reference` finds
    nothing but the reader's test of the old state file;
    `go run ./tools/ci reqs` passes.
- Verify: `go test -race ./internal/cmd/julieta/setup/... ./internal/cmd/julieta/handoff/... ./internal/cmd/julieta/status/...`,
  `go test ./e2e -run 'JulietaSetup|JulietaStatus|JulietaHandoff'` and
  `go run ./tools/ci fast`.
