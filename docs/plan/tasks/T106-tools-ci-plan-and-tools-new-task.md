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
file (rule 4 of `docs/plan.md`): `adr` moves to `tools/new/adr*.go` with
an `init`, `task` is `tools/new/task*.go`, and the `switch` in
`tools/new/main.go` goes, so `command` and `error` (T009) and `kit`
(T076) add files and edit none. `plan` is a check of the commit: it
reads `docs/plan/tasks/*.md` from the tree the commit checks read before
mise and the steps, as `hygiene`, `workflows` and `reqs` do.

- Implements: REQ-0826 (10 10.2), REQ-1089 (12 12.2), REQ-1101 (12
  12.2), REQ-1105 (12 12.3), REQ-1118 (12 12.3), REQ-1120 (12 12.3),
  REQ-1126 (12 12.3).
- Operator: no
- Acceptance:
  - Fixtures with a duplicate id, an unknown `needs` or `after`, an id
    that appears in both `needs` and `after`, a cycle, a pair of open
    tasks that no edge orders, directly or through other tasks, and that
    own one path (a path in the tree, or a literal path, matched by
    both, a generated output of rule 1 left out), a `merged:` without
    `pr:` and a `pr:` without `merged:` each fail; the tree passes; a
    deferred task's edges are checked like any other.
  - An absorbing task whose Implements is not a superset of each
    absorbed task's fails.
  - The fixture "an untracked task file that satisfies a `needs` the
    committed tree lacks fails in a pre-push run".
  - `tools/new task <slug>` writes one file with the next free id and
    edits no other file; the rebase-and-rerun rule of 12 12.3 covers a
    collision.
  - A test registers a fixture kind of `tools/new` by adding one file
    and editing none; a duplicate kind name panics at `init`; `go run
    ./tools/new adr` behaves as before.
  - Every Implements entry is a live requirement id with the right
    reading aid, checked by `tools/ci reqs`; `plan` reads the field's
    shape only: each entry an id with its reading aid in one of the
    forms the plan writes (`<page> <n>.<m>`, `<page> (opening)`, or
    `index` for the index page), or the `none` form of a task with no
    obligation; an Implements line of more than 40 ids is a finding,
    the mirror of the citation cap of 12 12.11.
  - A task's `ask_first` forecast is checked against its `owns`: a
    forecast that misses a surface of `.github/ask-first.yaml` whose
    globs match a path in `owns` fails, with a fixture.
  - `go run ./tools/ci plan --critical-path` prints the longest chain
    over `needs` and `after`, by sitting, and `--ready` the open tasks
    whose `needs` and `after` are merged, each one id per line, with a
    fixture graph test titled "critical path counts after edges".
- Verify: `go run ./tools/ci plan` exits 0; `go test ./tools/ci/...`
  runs the fixtures.
