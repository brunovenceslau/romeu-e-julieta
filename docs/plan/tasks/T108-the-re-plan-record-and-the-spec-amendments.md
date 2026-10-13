---
id: T108
title: "The re-plan record"
milestone: M0
module: docs
fate: new
owns:
  - docs/adr/0013-*.md
  - docs/adr/0001-*.md
  - docs/adr/0005-*.md
  - docs/plan.md
needs: []
after:
  - T104
  - T105
  - T107
  - T122
ask_first:
  - decisions
sitting: none
---

# T108 - The re-plan record

Earlier title: The re-plan record and the spec amendments.

Goal: One decision record, "re-plan v1 for early daily use: task files,
registration by file, build order, dogfood before recording", numbered
by `tools/new adr` (0013, after the stage B pass's 0012). It records the
answers of the open block of `docs/plan.md` and supersedes in part ADR
0005 rule 1 (the deferrals of merge-gate parts, as 05 5.4 now accepts
them by name). Every spec amendment the re-plan carried was applied on
2026-10-12 (ADR 0012; T122), so this task edits no spec page; it owns
`docs/plan.md`, since after it a change to that page is a re-plan pull
request of its own (rule 1). The record is on `decisions`, and
`docs/adr/README.md` is regenerated.

- Implements: REQ-0015 (index), REQ-0040 (01 1.1), REQ-1083 (12 12.2),
  REQ-1090 (12 12.2), REQ-1101 (12 12.2), REQ-1105 (12 12.3), REQ-1167
  (12 12.5).
- Operator: the answers of the open block, recorded here.
- Acceptance:
  - Every anchor the plan and the decision records cite still resolves
    (a one-off script, run outside the tree).
  - Each record the new record supersedes in part gains a `Superseded in
    part by` line under its status word, and the new record holds the
    matching `Supersedes in part` lines.
  - Each answer of the open block is recorded in `docs/plan.md` as
    decided on <date>, and the open block is removed.
  - The record cites, for each practice it adopts, the earlier decision
    it supersedes and the trigger that reopens it.
- Verify: `go run ./tools/ci generated` exits 0 after `go generate
  ./...`; `go run ./tools/ci fast` exits 0.
