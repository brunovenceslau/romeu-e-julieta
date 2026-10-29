---
id: T177
title: "julieta setup moves no branch"
milestone: M2
module: julieta-core
fate: new
owns:
  - internal/cmd/julieta/setup/repos.go
  - internal/cmd/julieta/setup/state.go
  - internal/cmd/julieta/setup/setup.go
  - internal/cmd/julieta/setup/report.go
  - internal/cmd/julieta/setup/*_test.go
  - internal/cmd/julieta/setup/testdata/**
  - internal/cmd/julieta/handoff/show*.go
  - internal/cmd/julieta/status/section_warnings.go
  - internal/cmd/julieta/status/status_test.go
  - e2e/julieta_setup_test.go
  - e2e/julieta_handoff_test.go
  - e2e/julieta_status_test.go
  - docs/spec/03-formats.md
  - docs/spec/04-cli.md
  - docs/spec/08-memory-handoff-salvage.md
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
Decided on 2026-10-28 with REQ-0399 (04 4.3) amended first; this
task's pull request removes `default-branch-moved` from the warnings
list of 04 4.1 and the SessionStart list of 08 8.2, in the same diff as
the code, since `TestWarningSlugs` holds the list to the code.

1. `setup` drops the fast-forward of the local default branch and the
   warning `default-branch-moved`: its report, the state it records, the
   one-time SessionStart line of `handoff show --hook` and the warnings
   section of `julieta status`. A state file written before this task
   whose warnings list still holds the slug `default-branch-moved` is
   read with that entry dropped, and the next write leaves it out.
2. The fetch, its `fetch-failed` warning and every other step stay as
   they are. `internal/gitsafe` keeps its fast-forward function for its
   other callers, if any; when none is left, its removal is a gitsafe
   task (ask-first `gitsafe`), not this one.

Re-plan (2026-10-28): no longer after T172. T172 waits on T174 and T175,
and both wait on T039 through T043 and T167, while this task needs only
T051; the file overlap (`internal/cmd/julieta/handoff/show*.go` and
`internal/cmd/julieta/setup/*_test.go`) is now ordered the other way, T172
after T178, so this task removes the one-time line from `handoff show`
before T172 removes the command.

Re-plan (2026-10-28): the `--json` report keeps `julieta-setup/v1` the
way protobuf retires a field (REQ-1310, added to the opening of 03 by this task):
`fastForward`, `from` and `to` stay, deprecated and always empty, and
the new member `branchState` carries the state of the default branch.
03 joins `owns`; no open task owns it.

- Implements: REQ-0399 (04 4.3), REQ-1310 (03 (opening)).
- Operator: no.
- Acceptance:
  - A clean `main` checked out and behind origin is fetched and stays at
    its commit after `setup`; the report and `--json` carry no
    `default-branch-moved`; a detached HEAD and a feature branch are
    untouched as before.
  - A state file of before this task whose warnings list holds the slug
    `default-branch-moved` loads, neither `status` nor SessionStart
    prints it, and the next `setup` writes the file without it.
  - The slug is gone from the warnings list of 04 4.1 and the
    SessionStart list of 08 8.2, with the lag sentence under REQ-0276
    reduced to `repo-setup-failed`; `TestWarningSlugs` passes.
  - `grep -rn default-branch-moved internal e2e docs/reference` finds
    nothing but the reader's test of the old state file;
    `go run ./tools/ci reqs` passes.
- Verify: `go test -race ./internal/cmd/julieta/setup/... ./internal/cmd/julieta/handoff/... ./internal/cmd/julieta/status/...`,
  `go test ./e2e -run 'JulietaSetup|JulietaStatus|JulietaHandoff'` and
  `go run ./tools/ci fast`.
