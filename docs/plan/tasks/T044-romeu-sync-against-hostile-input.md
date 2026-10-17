---
id: T044
title: "`romeu sync` against hostile input"
milestone: M6
module: romeu-cli
fate: deferred
owns: []
needs:
  - T043
  - T103
after: []
ask_first: []
sitting: none
---

# T044 - `romeu sync` against hostile input

Re-plan (2026-10-12): deferred to M6; the four I-level rows below ride
in T043 (the memory directory, the hostile kit on the macOS runners, the
hostile bundle, the I28 flip), and the unit table of the hostile trees
is T024's; the breadth waits for a consumer. Reopened by: a
hostile-input case outside the I-level set that a daily run or a review
asks for; or the first release candidate. The text below is the one
written before the re-plan; `needs` lists its earlier dependencies with
a folded id replaced by the id that absorbed it, and `owns` is written
when the task reopens.

- Implements: REQ-0340 (04 4.2), REQ-0495 (05 5.2), REQ-0512 (05 5.2).
- Operator: no.
- Ask-first, as forecast before the re-plan: none expected.
- Acceptance:
  - A memory directory with a symlink, a FIFO or a `..` name makes
    `sync` exit 2 and is not followed (I12, I24).
  - A personal kit with each hostile entry of I27 fails `sync` before a
    file is written outside `.romeu/candidates/`, on the macOS runners
    too.
  - A hostile tree delivered by a bundle is refused as `--from <sha>`,
    and refused by the walk when it is made reachable (I27).
  - Adding a capability to a kit flips gate 2 (I28).
- Verify: `go test -tags e2e ./e2e/... -run '^TestSyncHostile' -v` on
  the runners of `ci.yml`, and of `x64.yml` when it runs. The pattern names the `TestSyncHostile*` functions
  this task adds, and the reviewer reads them in the `-v` output (10
  10.8).
