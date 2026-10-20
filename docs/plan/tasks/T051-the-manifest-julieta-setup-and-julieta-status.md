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
  - tools/ci/internal/schema/julieta.go
needs:
  - T135
  - T009
  - T023
  - T013
  - T143
after:
  - T122

ask_first: []
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
rendered one (see `docs/notes/julieta-by-hand.md`). `setup/steps.go` holds the ordered
setup steps and `status/sections.go` the report sections, the seams
T035, T053, T054 and T079 extend by one line (rule 14).

Re-plan (2026-10-20): Changed, before building. `gitsafe` had no verb
that reads a working tree (HEAD, clean, branch heads, ahead and behind,
stashes, worktrees, the repo-local `core.hooksPath`), so those verbs are
T143's, which this task needs. T125 shipped `schemagen.Register` by type
and no command kind, so this task registers its two output types in
`tools/ci/internal/schema/julieta.go`, as `manifest.go` does for the
manifest; each output type carries a `schema` field, and the types sit
where `tools/ci` may import them (`tools/ci/internal/imports/rows.go`).
The edit of 03 3.6 leaves this task: REQ-0173 replaces a table by a
link to a generated page under `docs/reference/`, and no generator
writes one for the manifest yet (`schemas/manifest.v1.json` holds the
types, not the meaning of each field), so the edit and REQ-0173 move to
T021, whose field-table generator writes that page. The task no longer touches the
`spec` surface. The
protocol window is N = 1 with no protocol below it, so the container e2e
shows N accepted and 0 and N+1 refused, and N-1 stays with T135's unit
test of `acceptsProtocol` until a second protocol exists.

- Implements: REQ-0006 (index), REQ-0016 (index), REQ-0035 (00 0.4),
  REQ-0037 (00 0.4), REQ-0049 (01 1.2), REQ-0051 (01 1.2), REQ-0152 (02
  2.5), REQ-0153 (02 2.5), REQ-0154 (02 2.5), REQ-0155 (02 2.5),
  REQ-0156 (02 2.6), REQ-0161 (03 (opening)), REQ-0162 (03 (opening)),
  REQ-0163 (03 (opening)), REQ-0164 (03 (opening)), REQ-0165 (03
  (opening)), REQ-0166 (03 (opening)), REQ-0167 (03 (opening)), REQ-0172
  (03 (opening)), REQ-0396 (04 4.3), REQ-0397
  (04 4.3), REQ-0398 (04 4.3), REQ-0399 (04 4.3), REQ-0401 (04 4.3),
  REQ-0402 (04 4.3), REQ-0424 (04 4.3), REQ-0425 (04 4.3), REQ-0586 (06
  6.3), REQ-0587 (06 6.3), REQ-0636 (07 7.5), REQ-0665 (08 8.2),
  REQ-0707 (08 8.5), REQ-0733 (10 10.1), REQ-0734 (10 10.1), REQ-0740
  (10 10.1), REQ-0747 (10 10.1), REQ-0836 (10 10.2), REQ-1044 (12 12.1),
  REQ-1049 (12 12.1), REQ-1050 (12 12.1), REQ-1107 (12 12.3).
- Operator: no.
- Acceptance:
  - In the container e2e, a manifest of protocol N is accepted and a
    manifest of protocol 0 and one of N+1 are each refused with
    `manifest-protocol` (`RJ-342`), through the decoder of T135; N-1 is
    covered by T135's unit test while N is the first protocol.
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
  - `tools/ci/internal/schema/julieta.go` registers the first two
    output types, `julieta-setup.v1` and `julieta-status.v1`, with
    `schemagen.Register` (T125), and `tools/ci generate` writes
    `schemas/julieta-setup.v1.json` and `schemas/julieta-status.v1.json`.
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
