---
id: T070
title: "`romeu memory list|show`"
milestone: M4
module: romeu-cli
fate: kept
owns:
  - internal/cmd/romeu/memory/**
  - internal/cmd/romeu/all/import_memory.go
  - e2e/romeu_memory_test.go
needs:
  - T174
  - T041
after: []
ask_first: []
sitting: none
---

# T070 - `romeu memory list|show`

Earlier title: `romeu handoff`.

Goal: the host read of a project's memory, `romeu memory list [--repo
<dir>|--shared] [--json] <name>` and `romeu memory show [--json] <name>
<id>...`.

Re-plan (2026-10-28): `romeu handoff` leaves with ADR 0042, and this
task becomes the host read path of the one store: `list` prints the
index of 08 8.2 over every memory dir of the project, the open handoff
first with the gap line of 08 8.3, and takes the filters of `julieta
memory list` (T171); `show <id>...` prints entries in full. Every line
is escaped (I29) and the output opens with the fixed line that labels
what follows as text an agent wrote (05 5.4). Both read through
`memstore`'s `os.Root` readers and the reader of T174 (I24) and write
nothing. The deferred row `romeu memory list|show` of the index left
the table with this re-plan.

- Implements: REQ-0333 (04 4.2), REQ-0518 (05 5.2), REQ-0652 (08 8.1),
  REQ-0680 (08 8.3), REQ-0684 (08 8.3).
- Operator: no.
- Acceptance:
  - `list <name>` prints the fixed label line, then the open handoff's
    index line and the gap line when the newest stamp differs, then one
    line per open entry over the project's memory dirs, `shared` for
    the shared dir, with `--tag`, `--since`, `--limit`, `--sort`,
    `--status` and `--query` as `julieta memory list` has them; `show
    <name> <id> <id>` prints both in order and exits 1 after the known
    ones when one is unknown.
  - A body, a title and a branch name with ESC sequences print escaped
    (I29); nothing is written under the memory dirs (compared byte for
    byte before and after).
  - `tools/ci imports` still fails a romeu-linked package that imports
    `memstore/write` (I24).
- Verify: `go test -tags e2e ./e2e/... -run '^TestRomeuMemory' -v`. The
  pattern names the `TestRomeuMemory*` functions this task adds, and
  the reviewer reads them in the `-v` output (10 10.8).
