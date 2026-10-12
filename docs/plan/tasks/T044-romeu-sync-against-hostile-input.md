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

Re-plan (2026-10-12): deferred to M6; the I-level hostile cases ride in
T038 (the hostile trees of I27) and T043 (the hostile input of `sync`);
the breadth waits for a consumer. Reopened by: a hostile-input case
outside the I-level set that a daily run or a review asks for; or the
first release candidate. The text below is the one written before the
re-plan; `needs` lists its earlier dependencies with a folded id
replaced by the id that absorbed it, and `owns` is written when the task
reopens.

- Implements: 04 4.2 (`sync` steps 5 and 6), 05 5.1, 05 5.2 (I12, I24,
  I27, I28).
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
  the four runners. The pattern names the `TestSyncHostile*` functions
  this task adds, and the reviewer reads them in the `-v` output (10
  10.8).
