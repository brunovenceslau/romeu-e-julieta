---
id: T197
title: "romeu status slice B2: the spec commit, the orphans and the J10 line"
milestone: M4
module: romeu-cli
fate: new
owns:
  - internal/cmd/romeu/status/spec_commit*.go
  - internal/cmd/romeu/status/orphans*.go
  - internal/cmd/romeu/status/lost_generation*.go
  - e2e/status_b2_test.go
  - internal/cmd/romeu/status/mounts*.go
  - internal/cmd/romeu/status/sbx_version*.go
  - internal/cmd/romeu/status/section_tree.go
  - internal/cmd/romeu/status/section_tree_test.go
needs:
  - T048
  - T066
  - T202
  - T203
after:
  - T185
  - T174
ask_first:
  - formats
  - gates
sitting: none
---

# T197 - romeu status slice B2: the spec commit, the orphans and the J10 line

Narrowed on 2026-11-08: a stopped sandbox whose recreate digests
differ is now started with `sbx env exec <dir> -- true`, and refused
only for an adopted generation or, by `run` and `pull`, when the spec
removed a kit or a secrets binding since the create or the generation
lacks the lists that tell it (04 4.2, REQ-1387, REQ-1405, REQ-1406).
The hint of REQ-1377 is `romeu recreate <name>` for every generation
romeu created, running or stopped; the hint to undo the change and run
`romeu sync`, and the advice to start a sandbox before a sync or an
upgrade, are gone. T217 builds the start; where this task's text below
says otherwise, the spec and T217 win.

Goal: the slice B2 of `romeu status` (T048): the spec commit against the
synced one, the orphaned project and the orphaned repo dir with their
unpushed and salvage-only work (J13 step 3), and the J10 step 1 line of
a lost generation.

Added on 2026-11-01 (decided on 2026-10-31, scope this repository): T048
closes with its slice D2, so it can read merged and T066 can start; B2
moves here, after M3, with the J10 line, which reads git and the salvage
records. Each section registers from its own file through
`status/sections.go` (rule 14 of `docs/plan.md`), so this task edits no
file of T048 and comes after it for the shared directory, and after
T185, whose marker `sync` finds the config clone by. The schema
`romeu-status.v1` is closed (03, REQ-0163), so the members these
sections add carry the `formats` approval line; the orphan scan and
`envDirs` of `sync` sit in files T055 and T169 own, so this slice copies
the small part it needs and names who unifies it, as T048 says.

Amended on 2026-11-06 (the sbx 0.51 and kit schema m.12 amendments):
per project, `status` also prints the mounts sbx lists for the sandbox
against the render's and the ones it lists that romeu did not render
(01 1.5 step 6, T203), and the sbx version that created the open
generation beside the live `sbx version` (REQ-1359, 03 3.8), "unknown"
for a generation without the member, the kits removed from the set
since the generation's create (05 5.4, REQ-1375), and, where the recorded `sbx ls
--json` shape lists no mounts, that the mounts were not compared;
acceptance: a fixture with one extra mount, one whose shape lists no
mounts, and one generation with `sbxVersion` and one without, each line
asserted in `e2e/status_b2_test.go`.

Amended on 2026-11-07 (as decided on 2026-11-07, decisions G and H):
the line of the kits removed since the generation's create (REQ-1375)
is not built: a removed kit leaves residue only through an in-place
apply, which waits behind the gate of `run` step 3, and the line moves
with that apply's scope under the Deferred row that T210 opens; the
sentence above that names it is history. The `recreate-needed`
finding of `section_tree.go`, which T048 merged with the hint `romeu
recreate <name>`, now takes its hint from the shared function of T203
by the sandbox's state (REQ-1377): `romeu recreate <name>` for a
running sandbox, the stopped hint for a stopped one; and an adopted
generation, which the merged code skips, gets the finding too, with
the adopted message (REQ-1378). `status` prints every host bind the
listing holds (REQ-1381), and an adopted generation's mounts with no
comparison (REQ-1383). The edges follow the data (V2.19): `needs` T066,
which writes `sbxVersion`, and T203, whose hint and mounts result this
slice prints; T203 leaves `after`.

- Implements: REQ-0327 (04 4.2), REQ-0334 (04 4.2), REQ-1359 (03 3.8),
  REQ-0712 (08 8.5), REQ-1377 (04 4.1), REQ-1378 (04 4.2), REQ-1381 (01
  1.5), REQ-1383 (01 1.5).
- Operator: no.
- Acceptance:
  - Per project, the spec commit against the synced one: equal, ahead,
    behind and unrelated, each a fixture of host state and config
    clone, read without the lock.
  - An orphaned project (in the tree, absent from the spec) and an
    orphaned repo dir (J13) are listed with their unpushed commits and
    salvage-only refs counted from the clone; `sync` and `status` agree
    on the word `orphaned` (REQ-0334).
  - The J10 step 1 line is a fixture: `absent; generation <G> open;
    snapshot <T>; refs/sandboxes last fetched <T2>`.
  - Per repo, from host state and without the lock: the unimported
    snapshots, the `createdAt` of the newest `snapshot/heads.json`
    (`internal/salvage/snapshot.go`), `disabled`, and the newest stamp's
    dirty and stash counts (03 3.10), which T174 writes; moved from
    T048's acceptance with slice B2 (re-plan of 2026-10-31).
  - The `recreate-needed` finding names `romeu recreate <name>` for a
    running sandbox whose digests differ, the stopped hint of REQ-1377
    for a stopped one, and the adopted messages for an adopted
    generation, running and stopped, each a fixture (amended on
    2026-11-07).
  - A listing with a host bind off the closed list prints that bind; an
    adopted generation prints its mounts and says they were not
    compared; a generation with `sbxVersion` and one without print the
    creating version and "unknown".
  - `status --json` carries the new members, the schema is regenerated,
    and T048's `TestStatus*` cases still pass, the `recreate-needed`
    case updated to the new hint.
- Verify: `go test -tags e2e ./e2e/... -run '^TestStatusB2' -v && go test
  ./internal/cmd/romeu/status/... -run '^TestSectionTreeRecreateNeeded' -v`,
  which name the cases this task adds.
