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
  - internal/cmd/romeu/sync/step2_fetch.go
needs:
  - T041
after:
  - T169
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
3. `sync` step 2 reads the marker and refuses a missing or malformed
   one with the rule slug `config-marker`, exit 2, through `usage`
   (a rule with no row of 04 4.4, REQ-1258, so no new error row), in
   place of T043's guess (the one dir under `<cfg>-env/` that holds a
   `.git`).

- Implements: none: a layout rule of 02 2.3 that this task writes.
- Operator: no.
- Acceptance:
  - After `romeu init`, `<cfg>-env/.romeu-config` exists at 0600 and
    holds the clone's dir; a second init refuses as it does today.
  - `sync` with two repo dirs under `<cfg>-env/` finds the config clone
    by the marker.
  - `sync` reads the marker without following a link, bounded in size,
    checks its value against the repo `dir` pattern of 03 3.1
    (REQ-0176: no `/`, no `..`, not reserved), and checks that the dir
    is a plain directory holding `.git`; a planted `../x`, a link and a
    marker naming an absent clone are each exit 2.
  - `init` writes the marker inside its undo path, so a failed init
    leaves no marker.
- Verify: `go test ./internal/cmd/romeu/init/ -run Marker -v` and
  `go test ./internal/cmd/romeu/sync/ -run Marker -v`.
