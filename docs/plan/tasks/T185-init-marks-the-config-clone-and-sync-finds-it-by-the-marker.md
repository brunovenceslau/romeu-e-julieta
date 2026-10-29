---
id: T185
title: "init marks the config clone and sync finds it by the marker"
milestone: M3
module: romeu-cli
fate: new
owns:
  - internal/cmd/romeu/init/marker*.go
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
   one with a rule slug, exit 2, in place of T043's guess.

- Implements: none: a layout rule of 02 2.3 that this task writes.
- Operator: no.
- Acceptance:
  - After `romeu init`, `<cfg>-env/.romeu-config` exists at 0600 and
    holds the clone's dir; a second init refuses as it does today.
  - `sync` with two repo dirs under `<cfg>-env/` finds the config clone
    by the marker.
- Verify: `go test ./internal/cmd/romeu/init/ -run Marker -v`.
