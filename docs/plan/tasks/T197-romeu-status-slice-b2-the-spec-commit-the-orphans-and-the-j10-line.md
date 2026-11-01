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
needs:
  - T048
after:
  - T185
ask_first:
  - formats
  - gates
sitting: none
---

# T197 - romeu status slice B2: the spec commit, the orphans and the J10 line

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

- Implements: REQ-0327 (04 4.2), REQ-0334 (04 4.2), REQ-0712 (08 8.5).
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
  - `status --json` carries the new members, the schema is regenerated,
    and T048's `TestStatus*` cases still pass unchanged.
- Verify: `go test -tags e2e ./e2e/... -run '^TestStatusB2' -v`, which
  names the cases this task adds.
