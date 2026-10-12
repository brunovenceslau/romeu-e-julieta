---
id: T106
title: "`tools/ci plan` and `tools/new task`"
milestone: M0
module: ci-release
fate: new
owns:
  - tools/ci/internal/plan/**
  - tools/ci/check_plan.go
  - tools/new/**
needs:
  - T104
  - T105
after: []
ask_first:
  - checks
sitting: none
---

# T106 - `tools/ci plan` and `tools/new task`

Goal: The check, in `fast`, that validates the task files; the scaffold
that writes one with the next free id; `tools/new` kinds registered by
file (rule 4 of `docs/plan.md`): `adr` moves to `tools/new/adr*.go`
with an `init`, `task` is `tools/new/task*.go`, and the `switch` in
`tools/new/main.go` goes, so `command` (T008) and `kit` (T076) add
files and edit none.

- Implements: 10 10.2 (the plan part of the `sequences` row, until that
  check lands); 12 12.2 (Re-planning); 12 12.3 (the `tools/new` kinds).
- Operator: no
- Acceptance:
  - Fixtures with a duplicate id, an unknown `needs` or `after`, a
    cycle, a pair of open tasks that no edge orders, directly or
    through other tasks, and that own one path (a path in the tree, or
    a literal path, matched by both, a generated output of rule 1 left
    out), a `merged:` without `pr:` and a `pr:` without `merged:` each
    fail; the tree passes.
  - `tools/new task <slug>` writes one file with the next free id and
    edits no other file; the rebase-and-rerun rule of 12 12.3 covers a
    collision.
  - A test registers a fixture kind of `tools/new` by adding one file
    and editing none; `go run ./tools/new adr` behaves as before.
  - Every `Implements` entry cites an existing section, as the
    `sequences` row of 10 10.2 asks.
- Verify: `go run ./tools/ci plan` exits 0; `go test ./tools/ci/...`
  runs the fixtures.
