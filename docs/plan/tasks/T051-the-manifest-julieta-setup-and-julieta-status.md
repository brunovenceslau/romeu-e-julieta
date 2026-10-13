---
id: T051
title: "`julieta setup` and `julieta status`"
milestone: M1
module: julieta-core
fate: kept
owns:
  - internal/cmd/julieta/setup/**
  - internal/cmd/julieta/status/**
  - internal/cmd/julieta/all/import_setup.go
  - internal/cmd/julieta/all/import_status.go
  - e2e/julieta_setup_test.go
  - e2e/julieta_status_test.go
  - docs/spec/03-formats.md
needs:
  - T135
  - T009
  - T023
  - T013
after:
  - T122

ask_first:
  - spec
sitting: none
---

# T051 - `julieta setup` and `julieta status`

Earlier title: The manifest, `julieta setup` and `julieta status`.

Goal: `julieta setup` and `julieta status`.

Re-plan (2026-10-12): Kept, changed. The manifest type, its strict
decoder with the N and N-1 window, its encoder, the size rule and the
first schema are T135's, which this task needs; `tools/schemagen` and
`tools/ci schema` are T125's, and so are `go.mod` and `go.sum`. `setup`
links the binary on `PATH`, installs the global `core.hooksPath` to the
dispatcher directory, fast-forwards clean default branches and runs
`memory check`, whose file lands with T035. `install` and `skills-stale`
join with T053 and T079. A hand-written manifest validates like a
rendered one (see `docs/notes/julieta-by-hand.md`). It owns
`docs/spec/03-formats.md` for the one edit the specification assigns it:
the manifest table of 03 3.6 that its generated page replaces by a link
(rule 6 of `docs/plan.md`), whose requirement ids are retired with
`Replaced by: <the generated page>`. `setup/steps.go` holds the ordered
setup steps and `status/sections.go` the report sections, the seams
T035, T053, T054 and T079 extend by one line (rule 14).

- Implements: REQ-0006 (index), REQ-0016 (index), REQ-0035 (00 0.4),
  REQ-0037 (00 0.4), REQ-0049 (01 1.2), REQ-0051 (01 1.2), REQ-0152 (02
  2.5), REQ-0153 (02 2.5), REQ-0154 (02 2.5), REQ-0155 (02 2.5),
  REQ-0156 (02 2.6), REQ-0161 (03 (opening)), REQ-0162 (03 (opening)),
  REQ-0163 (03 (opening)), REQ-0164 (03 (opening)), REQ-0165 (03
  (opening)), REQ-0166 (03 (opening)), REQ-0167 (03 (opening)), REQ-0172
  (03 (opening)), REQ-0173 (03 (opening)), REQ-0396 (04 4.3), REQ-0397
  (04 4.3), REQ-0398 (04 4.3), REQ-0399 (04 4.3), REQ-0401 (04 4.3),
  REQ-0402 (04 4.3), REQ-0424 (04 4.3), REQ-0425 (04 4.3), REQ-0586 (06
  6.3), REQ-0587 (06 6.3), REQ-0636 (07 7.5), REQ-0665 (08 8.2),
  REQ-0707 (08 8.5), REQ-0733 (10 10.1), REQ-0734 (10 10.1), REQ-0740
  (10 10.1), REQ-0747 (10 10.1), REQ-0836 (10 10.2), REQ-1044 (12 12.1),
  REQ-1049 (12 12.1), REQ-1050 (12 12.1), REQ-1107 (12 12.3).
- Operator: no.
- Acceptance:
  - In the container e2e, a manifest of protocol N or N-1 is accepted
    and another is refused with `manifest-protocol` (`RJ-342`), through
    the decoder of T135.
  - `setup` links `$HOME/.local/bin/julieta` to the running binary,
    clones a missing secondary repo, and fast-forwards a default branch
    that is checked out and clean, reporting the others; a fast-forward
    of a default branch records `default-branch-moved` with `<repo>
    <old>..<new>` in julieta state, and `julieta status` shows it.
  - A fetch that fails is the warning `fetch-failed`, naming the repo
    and the git error; `setup` exits 0, and `status` and the next
    SessionStart show it; `setup` exits 1 only when the sandbox is left
    unusable (the PATH link, the dispatcher, a secondary clone, `memory
    check`).
  - A command whose Reads cell names the manifest exits 2 without one
    (04 4.3).
  - `status --json` reports the per-repo fields of 04 4.3 and the
    manifest's applied egress set (07 7.5); the `--json` outputs are
    decoded in tests with the strict decoder, a duplicate and an unknown
    member each refused, one fixture each.
  - `tools/schemagen`'s command kind (T125) writes
    `schemas/<binary>-<command>.v1.json` for a definition that declares
    `--json`; this task adds the first two output types,
    `julieta-setup.v1` and `julieta-status.v1`.
  - The other steps of `setup` in 04 4.3 land with their modules, and
    each task says so: the dispatcher in T054, `install` in T053,
    `memory check` and the agent memory dir check in T035, the required
    agent-profile paths in T079.
  - The package takes the clock and the version stamp as injected
    values, and its tests fix both (10 10.4).
- Verify: `go test -tags e2e ./e2e/... -run
  '^(TestJulietaSetup|TestJulietaStatus)' -v`. The pattern names the
  `TestJulietaSetup*` and `TestJulietaStatus*` functions this task adds,
  and the reviewer reads them in the `-v` output (10 10.8).
