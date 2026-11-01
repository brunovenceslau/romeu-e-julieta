---
id: T185
title: "init marks the config clone and sync finds it by the marker"
milestone: M3
module: romeu-cli
fate: new
owns:
  - internal/cmd/romeu/init/marker*.go
  - internal/cmd/romeu/init/init.go
  - docs/spec/02-layouts.md
  - docs/spec/04-cli.md
  - internal/cmd/romeu/sync/step2_fetch.go
needs:
  - T041
after:
  - T169
  - T178
ask_first: []
sitting: none
---

# T185 - init marks the config clone and sync finds it by the marker

Goal: a marker that names the config repo's clone, so `romeu sync` does
not guess it. Today step 2 of `sync` (T043) takes the one dir under
`<cfg>-env/` that holds a `.git` and refuses none or several, which
breaks once the config project has a second repo. Decided on
2026-10-29, scope this repository: a dotfile marker is acceptable.

1. `romeu init` writes `<cfg>-env/.romeu-config`, outside the clone so
   `git status` in it does not show the marker, holding the clone's dir
   name, mode 0600, create only.
2. 02 2.3 lists the marker in the host tree.
3. `sync` step 2 reads the marker and refuses a malformed
   one with the rule slug `config-marker`, exit 2, through `usage`
   (a rule with no row of 04 4.4, REQ-1258, so no new error row), in
   place of T043's guess (the one dir under `<cfg>-env/` that holds a
   `.git`).
4. A config project made before this task has no marker: when it is
   missing and exactly one dir under `<cfg>-env/` holds a `.git`, step
   2 takes that dir, as T043 does, and warns `config-marker-missing`
   (a slug added to 04 4.1), whose hint names the marker to write;
   with none or several such dirs it refuses as T043 does. Decided on
   2026-10-29, scope this repository: an existing project keeps
   working with no step of the operator, and the guess fails closed.

Built on 2026-11-01 (scope this repository): the marker's name is a
constant of each of the two command packages (`MarkerName` of `init`,
`markerName` of `sync`), since a command package imports no other (10
10.7); `sync` holds the marker's line to the rule `repo-dir` of 03 3.1
read from `spec.Rules`, so the two cannot drift; `init` refuses a
marker that is already there by the slug `config-marker`, as it refuses
a clone that exists (checked before the clone, so none is made and
removed for it, and again by the create-only write), and writes the
marker after the clone and before the settings, so a failure of either
leaves none: the removal is a step of `made.undo`, the one deletion
site of `init` the call-site table of I16 admits, so the table of
`tools/ci/internal/callsites`, which T195 owns, is not touched; `sync`
opens the marker without following a link and judges the open file, so
nothing swapped between a check and the read is read; the tests of `sync`
live in `step2_fetch_test.go`, the test file of the one file this task
owns there, and the warning slug joins the list of 04 4.1, which the
`cli.Warn` test reads; neither spec page is on an ask-first surface.
The e2e of J1 step 3 (`e2e/init_test.go`, T041's) counted the entries
of `<cfg>-env/` after `init`; it reads the marker now, the one edit
outside `owns` the marker forced.

- Implements: none: a layout rule of 02 2.3 that this task writes.
- Operator: no.
- Acceptance:
  - After `romeu init`, `<cfg>-env/.romeu-config` exists at 0600 and
    holds the clone's dir; a second init refuses as it does today.
  - `sync` with two repo dirs under `<cfg>-env/` finds the config clone
    by the marker.
  - With no marker and one clone, `sync` goes on and warns
    `config-marker-missing`; with no marker and two clones it refuses.
  - `sync` reads the marker without following a link, bounded in size,
    checks its value against the repo `dir` pattern of 03 3.1
    (REQ-0176: no `/`, no `..`, not reserved), and checks that the dir
    is a plain directory holding `.git`; a planted `../x`, a link and a
    marker naming an absent clone are each exit 2.
  - `init` writes the marker inside its undo path, so a failed init
    leaves no marker.
- Verify: `go test ./internal/cmd/romeu/init/ -run Marker -v` and
  `go test ./internal/cmd/romeu/sync/ -run Marker -v`.
